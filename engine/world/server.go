package world

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	opcodePong          uint16 = uint16(protocol.OpcodeSMSG_PONG)
	opcodeAuthChallenge uint16 = uint16(protocol.OpcodeSMSG_AUTH_CHALLENGE)
	opcodeAuthSession   uint32 = uint32(protocol.OpcodeCMSG_AUTH_SESSION)
	opcodeAuthResponse  uint16 = uint16(protocol.OpcodeSMSG_AUTH_RESPONSE)
	authOK              byte   = 12
	authReject          byte   = 14
	authUnknownAccount  byte   = 21
	authFailed          byte   = 13
	authBanned          byte   = 28
	loginServerNotFound byte   = 26
)

// overspeedPingWindow mirrors the fixed 27 second threshold in
// WorldSocket::HandlePing before the over-speed ping counter resets.
const overspeedPingWindow = 27 * time.Second

type Server struct {
	AuthStore         *database.Store
	CharactersStore   *database.Store
	WorldStore        *database.Store
	Logger            *slog.Logger
	RealmID           uint32
	Config            config.Config
	Features          *Features
	Data              *wotlk.Store
	sessionsMu        sync.RWMutex
	sessions          map[*session]struct{}
	objectsMu         sync.RWMutex
	inventoryMu       sync.Mutex
	hiddenGameObjects map[uint64]struct{}
	creatureAuras     map[uint64]map[uint32]struct{}
	channelsMu        sync.RWMutex
	channels          map[string]*worldChannel
	groupsMu          sync.RWMutex
	groups            map[uint64]*groupState // groupID -> groupState
	motionMu          sync.Mutex
	creatureMotion    map[uint64]*creatureMotion
	creatureRespawns  map[uint32]creatureRespawn
	lootMu            sync.Mutex
	creatureLoot      map[uint64]*activeLootState
}

type session struct {
	server             *Server
	conn               net.Conn
	authSeed           [4]byte
	crypt              *crypto.AuthCrypt
	authed             bool
	accountID          uint32
	accountName        string
	security           uint8
	accountExpansion   uint8
	gmChat             bool
	twoSideChat        bool
	legitimate         map[uint64]struct{}
	mounts             *MountState
	playerGUID         uint64
	playerLoaded       bool
	player             *playerState
	logoutAt           time.Time
	writeMu            sync.Mutex
	selection          uint64
	auras              map[uint32]struct{}
	scale              float32
	emoteState         uint32
	playerLocked       bool
	rooted             bool
	attackTarget       uint64
	lastSwing          time.Time
	logoutHook         bool
	gossip             *gossipMenuState
	gossipClosed       bool
	channels           map[string]struct{}
	tutorials          [8]uint32
	tutorialsInDB      bool
	activeLoot         *activeLootState
	trade              *playerTradeState
	guildInvitedID     uint32
	groupID            uint64 // GUID of the group this player is in (0 = no group)
	pendingGroupLeader uint64 // GUID of the player who invited us (0 = no invite pending)
	lastStreamX        float32
	lastStreamY        float32
	lastStreamZ        float32
	latency            atomic.Uint32
	lastPing           time.Time
	overSpeedPings     uint32
}

type account struct {
	ID          uint32
	SessionKey  []byte
	LastIP      string
	Locked      bool
	LockCountry string
	OS          string
	Security    uint8
	Expansion   uint8
}

func NewServer(stores *database.Set, logger *slog.Logger, realmID uint32, settings ...config.Config) *Server {
	c := config.Default()
	if len(settings) != 0 {
		c = settings[0]
	}
	server := &Server{AuthStore: stores.Auth, CharactersStore: stores.Characters, WorldStore: stores.World, Logger: logger, RealmID: realmID, Config: c, Features: NewFeatures(c, stores, logger), Data: wotlk.NewStore(filepath.Join(c.GameDataDir, "dbc")), sessions: make(map[*session]struct{}), hiddenGameObjects: make(map[uint64]struct{}), creatureAuras: make(map[uint64]map[uint32]struct{}), channels: make(map[string]*worldChannel), groups: make(map[uint64]*groupState), creatureMotion: make(map[uint64]*creatureMotion), creatureRespawns: make(map[uint32]creatureRespawn), creatureLoot: make(map[uint64]*activeLootState)}
	server.Features.LFG.SetDungeonValidator(func(id uint32) bool {
		dungeon, found, err := server.Data.LFGDungeon(id)
		return err == nil && found && wotlk.IsSupportedLFGType(dungeon.TypeID)
	})
	server.Features.Scripts.SetPlayerProvider(server.luaPlayers)
	return server
}

func (s *Server) Initialize(ctx context.Context) error {
	if err := s.Features.Initialize(ctx); err != nil {
		return err
	}
	go s.runWorldTick(ctx)
	return nil
}

func (s *Server) runWorldTick(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateActiveCreatures(ctx)
			s.updatePlayerCombat(ctx)
			s.processCreatureRespawns(ctx, time.Now())
		}
	}
}

func (s *Server) updatePlayerCombat(ctx context.Context) {
	s.sessionsMu.RLock()
	var combatSessions []*session
	for sess := range s.sessions {
		if sess.playerLoaded && sess.player != nil && sess.attackTarget != 0 && sess.player.Health > 0 {
			combatSessions = append(combatSessions, sess)
		}
	}
	s.sessionsMu.RUnlock()

	now := time.Now()
	for _, sess := range combatSessions {
		target, ok := sess.getCombatTarget(ctx, sess.attackTarget)
		if !ok || target.Health == 0 {
			_ = sess.sendAttackStop(sess.attackTarget, target.Health == 0)
			sess.attackTarget = 0
			continue
		}
		if target.Map != sess.player.Map {
			_ = sess.sendAttackStop(sess.attackTarget, false)
			sess.attackTarget = 0
			continue
		}
		if distance3D(sess.player.X, sess.player.Y, sess.player.Z, target.X, target.Y, target.Z) <= meleeAttackRange+2.0 {
			if now.Sub(sess.lastSwing) >= 2*time.Second {
				sess.lastSwing = now
				sess.executeMeleeSwing(ctx, target)
			}
		}
	}
}

func (s *Server) Handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	closed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-closed:
		}
	}()
	defer close(closed)
	state := &session{server: s, conn: conn, legitimate: make(map[uint64]struct{}), auras: make(map[uint32]struct{}), channels: make(map[string]struct{}), scale: 1}
	s.addSession(state)
	defer s.removeSession(state)
	defer state.logout()
	if _, err := rand.Read(state.authSeed[:]); err != nil {
		return
	}
	var extra [32]byte
	if _, err := rand.Read(extra[:]); err != nil {
		return
	}
	challenge := protocol.NewBuffer(40)
	challenge.WriteU32(1)
	challenge.Write(state.authSeed[:])
	challenge.Write(extra[:])
	if err := state.write(opcodeAuthChallenge, challenge.Bytes(), false); err != nil {
		return
	}
	for {
		if state.logoutAt.IsZero() {
			_ = conn.SetReadDeadline(time.Time{})
		} else {
			_ = conn.SetReadDeadline(state.logoutAt)
		}
		header, payload, err := protocol.ReadClientFrame(conn, state.decrypt)
		if err != nil {
			state.debug("world connection closed", "account", state.accountName, "error", err)
			if !state.logoutAt.IsZero() && isReadTimeout(err) {
				if logoutErr := state.completeLogout(ctx); logoutErr != nil {
					state.debug("player logout failed", "account", state.accountName, "error", logoutErr)
				}
				state.logoutAt = time.Time{}
				continue
			}
			return
		}
		state.debug("world packet received", "account", state.accountName, "opcode", opcodeName(header.Opcode), "size", len(payload))
		if !state.logoutAt.IsZero() && !time.Now().Before(state.logoutAt) {
			if logoutErr := state.completeLogout(ctx); logoutErr != nil {
				state.debug("player logout failed", "account", state.accountName, "error", logoutErr)
			}
			return
		}
		switch header.Opcode {
		case opcodeAuthSession:
			if state.authed || !state.handleAuthSession(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PING):
			if !state.authed || !state.handlePing(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TIME_SYNC_RESP):
			if !state.authed || !state.handleTimeSyncResponse(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_ENUM):
			if !state.authed || !state.handleCharEnum(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CREATURE_QUERY):
			if !state.authed || !state.handleCreatureQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GAMEOBJECT_QUERY):
			if !state.authed || !state.handleGameObjectQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ITEM_QUERY_SINGLE):
			if !state.authed || !state.handleItemQuerySingle(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUTOEQUIP_ITEM):
			if !state.authed || !state.handleAutoEquipItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUTOEQUIP_ITEM_SLOT):
			if !state.authed || !state.handleAutoEquipItemSlot(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SPLIT_ITEM):
			if !state.authed || !state.handleSplitItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUTOSTORE_BAG_ITEM):
			if !state.authed || !state.handleAutoStoreBagItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SWAP_INV_ITEM):
			if !state.authed || !state.handleSwapInvItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SWAP_ITEM):
			if !state.authed || !state.handleSwapItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DESTROYITEM):
			if !state.authed || !state.handleDestroyItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LIST_INVENTORY):
			if !state.authed || !state.handleListInventory(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BUY_ITEM):
			if !state.authed || !state.handleBuyItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BUY_ITEM_IN_SLOT):
			if !state.authed || !state.handleBuyItemInSlot(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SELL_ITEM):
			if !state.authed || !state.handleSellItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TRAINER_LIST):
			if !state.authed || !state.handleTrainerList(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TRAINER_BUY_SPELL):
			if !state.authed || !state.handleTrainerBuySpell(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOOT):
			if !state.authed || !state.handleLoot(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOOT_MONEY):
			if !state.authed || !state.handleLootMoney(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUTOSTORE_LOOT_ITEM):
			if !state.authed || !state.handleAutostoreLootItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOOT_RELEASE):
			if !state.authed || !state.handleLootRelease(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TAXINODE_STATUS_QUERY):
			if !state.authed || !state.handleTaxiNodeStatusQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TAXIQUERYAVAILABLENODES):
			if !state.authed || !state.handleTaxiQueryAvailableNodes(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ACTIVATETAXI), uint32(protocol.OpcodeCMSG_ACTIVATETAXIEXPRESS):
			if !state.authed || !state.handleActivateTaxi(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GET_MAIL_LIST):
			if !state.authed || !state.handleGetMailList(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SEND_MAIL):
			if !state.authed || !state.handleSendMail(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MAIL_TAKE_MONEY):
			if !state.authed || !state.handleMailTakeMoney(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MAIL_TAKE_ITEM):
			if !state.authed || !state.handleMailTakeItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MAIL_DELETE):
			if !state.authed || !state.handleMailDelete(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MAIL_MARK_AS_READ):
			if !state.authed || !state.handleMailMarkAsRead(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_QUERY_NEXT_MAIL_TIME):
			if !state.authed || !state.handleQueryNextMailTime(ctx) {
				return
			}
		case uint32(protocol.OpcodeMSG_AUCTION_HELLO):
			if !state.authed || !state.handleAuctionHello(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_LIST_ITEMS):
			if !state.authed || !state.handleAuctionListItems(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_SELL_ITEM):
			if !state.authed || !state.handleAuctionSellItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_PLACE_BID):
			if !state.authed || !state.handleAuctionPlaceBid(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_LIST_OWNER_ITEMS):
			if !state.authed || !state.handleAuctionListOwnerItems(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_LIST_BIDDER_ITEMS):
			if !state.authed || !state.handleAuctionListBidderItems(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_REMOVE_ITEM):
			if !state.authed || !state.handleAuctionRemoveItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_INITIATE_TRADE):
			if !state.authed || !state.handleInitiateTrade(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BEGIN_TRADE):
			if !state.authed || !state.handleBeginTrade(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_TRADE_GOLD):
			if !state.authed || !state.handleSetTradeGold(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_TRADE_ITEM):
			if !state.authed || !state.handleSetTradeItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CLEAR_TRADE_ITEM):
			if !state.authed || !state.handleClearTradeItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ACCEPT_TRADE):
			if !state.authed || !state.handleAcceptTrade(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_UNACCEPT_TRADE):
			if !state.authed || !state.handleUnacceptTrade(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_TRADE), uint32(protocol.OpcodeCMSG_IGNORE_TRADE), uint32(protocol.OpcodeCMSG_BUSY_TRADE):
			if !state.authed || !state.handleCancelTrade(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_QUERY):
			if !state.authed || !state.handleGuildQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_ROSTER):
			if !state.authed || !state.handleGuildRoster(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_INVITE):
			if !state.authed || !state.handleGuildInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_ACCEPT):
			if !state.authed || !state.handleGuildAccept(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_DECLINE):
			if !state.authed || !state.handleGuildDecline(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_LEAVE):
			if !state.authed || !state.handleGuildLeave(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_MOTD):
			if !state.authed || !state.handleGuildMotd(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANK_QUERY_TAB):
			if !state.authed || !state.handleGuildBankQueryTab(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CONTACT_LIST):
			if !state.authed || !state.handleContactList(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ADD_FRIEND):
			if !state.authed || !state.handleAddFriend(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DEL_FRIEND):
			if !state.authed || !state.handleDelFriend(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ADD_IGNORE):
			if !state.authed || !state.handleAddIgnore(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DEL_IGNORE):
			if !state.authed || !state.handleDelIgnore(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_CONTACT_NOTES):
			if !state.authed || !state.handleSetContactNotes(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_INVITE):
			if !state.authed || !state.handleGroupInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_ACCEPT):
			if !state.authed || !state.handleGroupAccept(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_DECLINE):
			if !state.authed || !state.handleGroupDecline(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_UNINVITE):
			if !state.authed || !state.handleGroupUninvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_UNINVITE_GUID):
			if !state.authed || !state.handleGroupUninviteGUID(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_SET_LEADER):
			if !state.authed || !state.handleGroupSetLeader(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_DISBAND):
			if !state.authed || !state.handleGroupDisband(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOOT_METHOD):
			if !state.authed || !state.handleLootMethod(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_MINIMAP_PING):
			if !state.authed || !state.handleMinimapPing(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_RAID_TARGET_UPDATE):
			if !state.authed || !state.handleRaidTargetUpdate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_RAID_CONVERT):
			if !state.authed || !state.handleGroupRaidConvert(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_PARTY_ASSIGNMENT):
			if !state.authed || !state.handlePartyAssignment(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_RAID_READY_CHECK):
			if !state.authed || !state.handleReadyCheck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_RANDOM_ROLL):
			if !state.authed || !state.handleRandomRoll(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ATTACK_SWING):
			if !state.authed || !state.handleAttackSwing(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ATTACK_STOP):
			if !state.authed || !state.handleAttackStop() {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_SHEATHED):
			if !state.authed || !state.handleSetSheathed(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CAST_SPELL):
			if !state.authed || !state.handleCastSpell(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_CAST):
			if !state.authed || !state.handleCancelCast(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_CHANNELLING):
			if !state.authed || !state.handleCancelChanneling(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_AURA):
			if !state.authed || !state.handleCancelAura(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GOSSIP_HELLO):
			if !state.authed || !state.handleGossipHello(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_NPC_TEXT_QUERY):
			if !state.authed || !state.handleNpcTextQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_JOIN_CHANNEL):
			if !state.authed || !state.handleJoinChannel(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LEAVE_CHANNEL):
			if !state.authed || !state.handleLeaveChannel(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_LIST), uint32(protocol.OpcodeCMSG_CHANNEL_DISPLAY_LIST):
			if !state.authed || !state.handleChannelList(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GOSSIP_SELECT_OPTION):
			if !state.authed || !state.handleGossipSelectOption(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_STATUS_QUERY):
			if !state.authed || !state.handleQuestgiverStatusQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_HELLO):
			if !state.authed || !state.handleQuestgiverHello(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_QUERY_QUEST):
			if !state.authed || !state.handleQuestgiverQueryQuest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUEST_QUERY):
			if !state.authed || !state.handleQuestQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_ACCEPT_QUEST):
			if !state.authed || !state.handleQuestgiverAcceptQuest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_COMPLETE_QUEST):
			if !state.authed || !state.handleQuestgiverCompleteQuest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_REQUEST_REWARD):
			if !state.authed || !state.handleQuestgiverRequestReward(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_CHOOSE_REWARD):
			if !state.authed || !state.handleQuestgiverChooseReward(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_QUEST_AUTOLAUNCH):
			if !state.authed {
				return
			}
			state.debug("quest autolaunch received", "account", state.accountName, "size", len(payload))
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_CANCEL):
			if !state.authed || !state.handleQuestgiverCancel() {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTLOG_REMOVE_QUEST):
			if !state.authed || !state.handleQuestLogRemoveQuest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_NAME_QUERY):
			if !state.authed || !state.handleNameQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUERY_TIME):
			if !state.authed || !state.handleQueryTime() {
				return
			}
		case uint32(protocol.OpcodeCMSG_PLAYED_TIME):
			if !state.authed || !state.handlePlayedTime(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ZONEUPDATE):
			if !state.authed || !state.handleZoneUpdate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_ACCOUNT_DATA):
			if !state.authed || !state.handleRequestAccountData(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_UPDATE_ACCOUNT_DATA):
			if !state.authed || !state.handleUpdateAccountData(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_ACTIONBAR_TOGGLES):
			if !state.authed || !state.handleSetActionBarToggles(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_ACTION_BUTTON):
			if !state.authed || !state.handleSetActionButton(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_WORLD_STATE_UI_TIMER_UPDATE):
			if !state.authed || !state.handleWorldStateUITimer() {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_RAID_INFO):
			if !state.authed || !state.handleRequestRaidInfo() {
				return
			}
		case uint32(protocol.OpcodeCMSG_READY_FOR_ACCOUNT_DATA_TIMES):
			if !state.authed || !state.handleReadyForAccountDataTimes() {
				return
			}
		case uint32(protocol.OpcodeCMSG_REALM_SPLIT):
			if !state.authed || !state.handleRealmSplit(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_CREATE):
			if !state.authed || !state.handleCharCreate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_DELETE):
			if !state.authed || !state.handleCharDelete(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PLAYER_LOGIN):
			if !state.authed || !state.handlePlayerLogin(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_OPENING_CINEMATIC):
			if !state.authed || !state.handleOpeningCinematic() {
				return
			}
		case uint32(protocol.OpcodeCMSG_NEXT_CINEMATIC_CAMERA):
			if !state.authed || !state.handleNextCinematicCamera() {
				return
			}
		case uint32(protocol.OpcodeCMSG_COMPLETE_CINEMATIC):
			if !state.authed || !state.handleCompleteCinematic(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_FACTION_ATWAR):
			if !state.authed || !state.handleSetFactionAtWar(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_STANDSTATECHANGE):
			if !state.authed || !state.handleStandStateChange(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_EMOTE):
			if !state.authed || !state.handleEmote(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TEXT_EMOTE):
			if !state.authed || !state.handleTextEmote(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_MOVE_WORLDPORT_ACK):
			if state.authed && state.player != nil {
				state.sendPlayerUpdate()
				if update, _, err := state.server.buildNearbyCreatureUpdates(ctx, *state.player); err == nil && update != nil {
					_ = state.write(update.Opcode, update.Payload.Bytes(), true)
				}
				if update, _, err := state.server.buildNearbyGameObjectUpdates(ctx, *state.player); err == nil && update != nil {
					_ = state.write(update.Opcode, update.Payload.Bytes(), true)
				}
				_ = state.write(uint16(protocol.OpcodeSMSG_TIME_SYNC_REQ), buildTimeSyncRequest(0), true)
			}
		case uint32(protocol.OpcodeMSG_MOVE_TELEPORT_ACK), uint32(protocol.OpcodeCMSG_MOVE_SET_CAN_FLY_ACK),
			uint32(protocol.OpcodeCMSG_FORCE_RUN_SPEED_CHANGE_ACK), uint32(protocol.OpcodeCMSG_FORCE_RUN_BACK_SPEED_CHANGE_ACK),
			uint32(protocol.OpcodeCMSG_FORCE_SWIM_SPEED_CHANGE_ACK), uint32(protocol.OpcodeCMSG_FORCE_SWIM_BACK_SPEED_CHANGE_ACK),
			uint32(protocol.OpcodeCMSG_FORCE_WALK_SPEED_CHANGE_ACK), uint32(protocol.OpcodeCMSG_FORCE_FLIGHT_SPEED_CHANGE_ACK),
			uint32(protocol.OpcodeCMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE_ACK):
			// Movement acknowledged by client
		case uint32(protocol.OpcodeCMSG_MESSAGECHAT):
			if !state.authed || !state.handleMessageChat(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_SELECTION):
			if !state.authed || !state.handleSetSelection(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_ACTIVE_MOVER):
			if !state.authed || !state.handleSetActiveMover(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_JOIN):
			if !state.authed || !state.handleLFGJoin(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_LEAVE):
			if !state.authed || !state.handleLFGLeave() {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_GET_STATUS):
			if !state.authed || !state.handleLFGGetStatus() {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOGOUT_REQUEST):
			if !state.authed || !state.handleLogoutRequest(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOGOUT_CANCEL):
			if !state.authed || !state.handleLogoutCancel() {
				return
			}
		case uint32(protocol.OpcodeCMSG_TUTORIAL_FLAG):
			if !state.authed || !state.handleTutorialFlag(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TUTORIAL_CLEAR):
			if !state.authed || !state.handleTutorialClear(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TUTORIAL_RESET):
			if !state.authed || !state.handleTutorialReset(ctx) {
				return
			}
		case uint32(protocol.OpcodeMSG_MOVE_START_FORWARD), uint32(protocol.OpcodeMSG_MOVE_START_BACKWARD), uint32(protocol.OpcodeMSG_MOVE_STOP), uint32(protocol.OpcodeMSG_MOVE_START_STRAFE_LEFT), uint32(protocol.OpcodeMSG_MOVE_START_STRAFE_RIGHT), uint32(protocol.OpcodeMSG_MOVE_STOP_STRAFE), uint32(protocol.OpcodeMSG_MOVE_JUMP), uint32(protocol.OpcodeMSG_MOVE_START_TURN_LEFT), uint32(protocol.OpcodeMSG_MOVE_START_TURN_RIGHT), uint32(protocol.OpcodeMSG_MOVE_STOP_TURN), uint32(protocol.OpcodeMSG_MOVE_START_PITCH_UP), uint32(protocol.OpcodeMSG_MOVE_START_PITCH_DOWN), uint32(protocol.OpcodeMSG_MOVE_STOP_PITCH), uint32(protocol.OpcodeMSG_MOVE_SET_RUN_MODE), uint32(protocol.OpcodeMSG_MOVE_SET_WALK_MODE), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), uint32(protocol.OpcodeMSG_MOVE_START_SWIM), uint32(protocol.OpcodeMSG_MOVE_STOP_SWIM), uint32(protocol.OpcodeMSG_MOVE_ROOT), uint32(protocol.OpcodeMSG_MOVE_UNROOT), uint32(protocol.OpcodeMSG_MOVE_HEARTBEAT), uint32(protocol.OpcodeMSG_MOVE_HOVER), uint32(protocol.OpcodeMSG_MOVE_SET_FACING), uint32(protocol.OpcodeMSG_MOVE_SET_PITCH):
			if !state.authed || !state.handleMovement(ctx, header.Opcode, payload) {
				return
			}
		default:
			if !state.authed {
				return
			}
			state.debug("world packet ignored", "account", state.accountName, "opcode", opcodeName(header.Opcode), "size", len(payload))
		}
	}
}

func opcodeName(opcode uint32) string {
	if name, ok := protocol.OpcodeNames[protocol.Opcode(opcode)]; ok {
		return name
	}
	return fmt.Sprintf("0x%03X", opcode)
}

func (s *session) handleReadyForAccountDataTimes() bool {
	if err := s.write(uint16(protocol.OpcodeSMSG_ACCOUNT_DATA_TIMES), buildAccountDataTimes(time.Now(), globalAccountDataMask), true); err != nil {
		s.debug("global account data times failed", "account", s.accountName, "error", err)
		return false
	}
	return true
}

func (s *session) handleRealmSplit(payload []byte) bool {
	response, err := buildRealmSplit(payload)
	if err != nil {
		s.debug("realm split rejected", "account", s.accountName, "error", err)
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_REALM_SPLIT), response, true); err != nil {
		s.debug("realm split response failed", "account", s.accountName, "error", err)
		return false
	}
	return true
}

func (s *session) handleAuthSession(ctx context.Context, payload []byte) bool {
	b := protocol.NewReader(payload)
	build, err := b.ReadU32()
	if err != nil {
		return false
	}
	if _, err = b.ReadU32(); err != nil {
		return false
	}
	accountName, err := b.ReadCString()
	if err != nil {
		return false
	}
	debugAccount := accountName
	if _, err = b.ReadU32(); err != nil {
		return false
	}
	localChallenge, err := b.Read(4)
	if err != nil {
		return false
	}
	if _, err = b.ReadU32(); err != nil {
		return false
	}
	if _, err = b.ReadU32(); err != nil {
		return false
	}
	realmID, err := b.ReadU32()
	if err != nil {
		return false
	}
	if _, err = b.ReadU64(); err != nil {
		return false
	}
	digestBytes, err := b.Read(sha1.Size)
	if err != nil {
		return false
	}
	account, err := loadAccount(ctx, s.server.AuthStore, accountName, s.server.RealmID)
	if err != nil || account == nil {
		s.debug("world authentication rejected", "account", debugAccount, "reason", "unknown account")
		_ = s.write(opcodeAuthResponse, []byte{authUnknownAccount}, false)
		return false
	}
	if realmID != s.server.RealmID {
		s.debug("world authentication rejected", "account", debugAccount, "reason", "realm mismatch", "realm", realmID)
		_ = s.write(opcodeAuthResponse, []byte{loginServerNotFound}, false)
		return false
	}
	if banned, err := accountBanned(ctx, s.server.AuthStore, account.ID); err != nil || banned {
		s.debug("world authentication rejected", "account", debugAccount, "reason", "account ban")
		_ = s.write(opcodeAuthResponse, []byte{authBanned}, false)
		return false
	}
	if account.Locked && account.LastIP != remoteAddress(s.conn) {
		s.debug("world authentication rejected", "account", debugAccount, "reason", "ip lock")
		_ = s.write(opcodeAuthResponse, []byte{authFailed}, false)
		return false
	}
	if len(account.SessionKey) != crypto.SRP6SessionKeyLength {
		s.debug("world authentication rejected", "account", debugAccount, "reason", "invalid session key length")
		_ = s.write(opcodeAuthResponse, []byte{authFailed}, false)
		return false
	}
	h := sha1.New()
	_, _ = h.Write([]byte(accountName))
	_, _ = h.Write(make([]byte, 4))
	_, _ = h.Write(localChallenge)
	_, _ = h.Write(s.authSeed[:])
	_, _ = h.Write(account.SessionKey)
	if subtle.ConstantTimeCompare(h.Sum(nil), digestBytes) != 1 {
		s.debug("world authentication rejected", "account", debugAccount, "reason", "invalid session digest")
		_ = s.write(opcodeAuthResponse, []byte{authFailed}, false)
		return false
	}
	if _, err := s.server.AuthStore.ExecStatement(ctx, "LOGIN_UPD_ACCOUNT_ONLINE", account.ID); err != nil {
		return false
	}
	if _, err := s.server.AuthStore.DB.ExecContext(ctx, "UPDATE account SET last_ip = ? WHERE id = ?", remoteAddress(s.conn), account.ID); err != nil {
		return false
	}
	s.crypt, err = crypto.NewAuthCrypt(account.SessionKey)
	if err != nil {
		return false
	}
	s.authed = true
	s.accountID = account.ID
	s.accountName = accountName
	s.security = account.Security
	s.gmChat = false
	if s.twoSideChat, err = accountHasPermission(ctx, s.server.AuthStore.DB, account.ID, s.server.RealmID, account.Security, permissionTwoSideInteractionChat); err != nil {
		s.twoSideChat = false
		s.debug("RBAC permission lookup failed", "account", accountName, "permission", permissionTwoSideInteractionChat, "error", err)
	}
	s.accountExpansion = account.Expansion
	if s.accountExpansion == 0 && s.server.Config.Expansion > 0 {
		s.accountExpansion = uint8(s.server.Config.Expansion)
	}
	s.debug("world authentication accepted", "account", accountName, "build", build, "expansion", s.accountExpansion, "gm_chat", s.gmChat, "two_side_chat", s.twoSideChat, "remote", remoteAddress(s.conn))
	s.loadTutorials(ctx)

	authBuf := protocol.NewBuffer(12)
	authBuf.WriteU8(authOK)
	authBuf.WriteU32(0)                 // BillingTimeRemaining
	authBuf.WriteU8(0)                  // BillingPlanFlags
	authBuf.WriteU32(0)                 // BillingTimeRested
	authBuf.WriteU8(s.accountExpansion) // 0 Vanilla, 1 TBC, 2 WotLK
	authBuf.WriteU32(0)                 // Queue position
	authBuf.WriteU8(0)                  // Free character migration bool
	return s.write(opcodeAuthResponse, authBuf.Bytes(), true) == nil
}

func (s *session) loadTutorials(ctx context.Context) {
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return
	}
	row := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT tut0, tut1, tut2, tut3, tut4, tut5, tut6, tut7 FROM account_tutorial WHERE accountId = ?", s.accountID)
	var tut0, tut1, tut2, tut3, tut4, tut5, tut6, tut7 uint32
	if err := row.Scan(&tut0, &tut1, &tut2, &tut3, &tut4, &tut5, &tut6, &tut7); err == nil {
		s.tutorials = [8]uint32{tut0, tut1, tut2, tut3, tut4, tut5, tut6, tut7}
		s.tutorialsInDB = true
	}
}

func (s *session) saveTutorials(ctx context.Context) {
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return
	}
	if s.tutorialsInDB {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE account_tutorial SET tut0 = ?, tut1 = ?, tut2 = ?, tut3 = ?, tut4 = ?, tut5 = ?, tut6 = ?, tut7 = ? WHERE accountId = ?",
			s.tutorials[0], s.tutorials[1], s.tutorials[2], s.tutorials[3], s.tutorials[4], s.tutorials[5], s.tutorials[6], s.tutorials[7], s.accountID)
	} else {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "INSERT INTO account_tutorial(tut0, tut1, tut2, tut3, tut4, tut5, tut6, tut7, accountId) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			s.tutorials[0], s.tutorials[1], s.tutorials[2], s.tutorials[3], s.tutorials[4], s.tutorials[5], s.tutorials[6], s.tutorials[7], s.accountID)
		s.tutorialsInDB = true
	}
}

func (s *session) handleTutorialFlag(ctx context.Context, payload []byte) bool {
	b := protocol.NewReader(payload)
	data, err := b.ReadU32()
	if err != nil {
		return false
	}
	index := data / 32
	if index < 8 {
		s.tutorials[index] |= 1 << (data % 32)
		s.saveTutorials(ctx)
	}
	return true
}

func (s *session) handleTutorialClear(ctx context.Context) bool {
	for i := range s.tutorials {
		s.tutorials[i] = 0xFFFFFFFF
	}
	s.saveTutorials(ctx)
	return true
}

func (s *session) handleTutorialReset(ctx context.Context) bool {
	for i := range s.tutorials {
		s.tutorials[i] = 0
	}
	s.saveTutorials(ctx)
	return true
}

func (s *session) handlePing(ctx context.Context, payload []byte) bool {
	b := protocol.NewReader(payload)
	ping, err := b.ReadU32()
	if err != nil {
		return false
	}
	latency, err := b.ReadU32()
	if err != nil {
		return false
	}
	// Reference: WorldSocket::HandlePing — over-speed ping protection with a 27 second window,
	// latency tracking on the session, and a SMSG_PONG echo of the ping counter.
	now := time.Now()
	if s.lastPing.IsZero() {
		s.lastPing = now
	} else {
		diff := now.Sub(s.lastPing)
		s.lastPing = now
		if diff < overspeedPingWindow {
			s.overSpeedPings++
			if s.server != nil && s.server.Config.MaxOverSpeedPings != 0 && s.overSpeedPings > s.server.Config.MaxOverSpeedPings {
				if s.server.AuthStore == nil {
					return false
				}
				skip, permErr := accountHasPermission(ctx, s.server.AuthStore.DB, s.accountID, s.server.RealmID, s.security, permissionSkipCheckOverSpeedPing)
				if permErr != nil {
					s.debug("over-speed ping permission lookup failed", "account", s.accountName, "error", permErr)
					return false
				}
				if !skip {
					s.debug("session kicked for over-speed pings", "account", s.accountName)
					return false
				}
			}
		} else {
			s.overSpeedPings = 0
		}
	}
	s.latency.Store(latency)
	response := protocol.NewBuffer(4)
	response.WriteU32(ping)
	return s.write(opcodePong, response.Bytes(), true) == nil
}

func (s *session) decrypt(data []byte) error {
	if s.crypt == nil {
		return nil
	}
	return s.crypt.DecryptRecv(data)
}

func (s *session) write(opcode uint16, payload []byte, encrypt bool) error {
	if s == nil || s.conn == nil {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	frame, headerSize, err := protocol.EncodeServerFrame(opcode, payload)
	if err != nil {
		return err
	}
	if encrypt && s.crypt != nil {
		if err := s.crypt.EncryptSend(frame[:headerSize]); err != nil {
			return err
		}
	}
	for len(frame) > 0 {
		n, err := s.conn.Write(frame)
		if err != nil {
			return err
		}
		frame = frame[n:]
	}
	s.debug("world packet sent", "account", s.accountName, "opcode", opcodeName(uint32(opcode)), "size", len(payload))
	return nil
}

func loadAccount(ctx context.Context, store *database.Store, username string, realmID uint32) (*account, error) {
	var result account
	var locked int64
	var expansion sql.NullInt64
	query := "SELECT id, session_key_auth, last_ip, locked, lock_country, os, expansion FROM account WHERE username = ? LIMIT 1"
	if store.Backend == database.BackendSQLite {
		query = "SELECT id, session_key_auth, last_ip, locked, lock_country, os, expansion FROM account WHERE UPPER(username) = UPPER(?) LIMIT 1"
	}
	err := store.DB.QueryRowContext(ctx, query, username).Scan(&result.ID, &result.SessionKey, &result.LastIP, &locked, &result.LockCountry, &result.OS, &expansion)
	if err != nil {
		fallbackQuery := "SELECT id, session_key_auth, last_ip, locked, lock_country, os FROM account WHERE username = ? LIMIT 1"
		if store.Backend == database.BackendSQLite {
			fallbackQuery = "SELECT id, session_key_auth, last_ip, locked, lock_country, os FROM account WHERE UPPER(username) = UPPER(?) LIMIT 1"
		}
		err = store.DB.QueryRowContext(ctx, fallbackQuery, username).Scan(&result.ID, &result.SessionKey, &result.LastIP, &locked, &result.LockCountry, &result.OS)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	result.Locked = locked != 0
	if expansion.Valid && expansion.Int64 > 0 {
		result.Expansion = uint8(expansion.Int64)
	} else {
		result.Expansion = 2 // WotLK default
	}
	var security int64
	err = store.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(SecurityLevel), 0) FROM account_access WHERE AccountID = ? AND RealmID IN (-1, ?)", result.ID, realmID).Scan(&security)
	if err == nil {
		result.Security = uint8(security)
	} else if !strings.Contains(strings.ToLower(err.Error()), "no such table") && !strings.Contains(strings.ToLower(err.Error()), "doesn't exist") && !strings.Contains(strings.ToLower(err.Error()), "unknown table") {
		return nil, err
	}
	return &result, nil
}

func accountBanned(ctx context.Context, store *database.Store, id uint32) (bool, error) {
	var count int64
	err := store.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM account_banned WHERE id = ? AND active = 1 AND (unbandate > ? OR unbandate = bandate)", id, timeNow()).Scan(&count)
	return count != 0, err
}

func timeNow() int64 { return time.Now().Unix() }

func remoteAddress(conn net.Conn) string {
	address := conn.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return strings.TrimSpace(address)
}

func (s *Server) debug(message string, args ...any) {
	if s.Logger != nil {
		s.Logger.Debug(message, args...)
	}
}

func (s *session) debug(message string, args ...any) {
	if s != nil && s.server != nil {
		s.server.debug(message, args...)
	}
}

func (s *session) logout() {
	ctx := context.Background()
	if s.server != nil {
		s.server.removeSessionChannels(s)
	}
	if s.playerLoaded {
		s.triggerLogout(ctx)
		if err := s.savePlayerPosition(ctx); err != nil {
			s.debug("player position save failed", "account", s.accountName, "guid", s.playerGUID, "error", err)
		}
		_, _ = s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_ACCOUNT_ONLINE", s.accountID)
	}
	if s.accountID != 0 {
		_, _ = s.server.AuthStore.DB.ExecContext(ctx, "UPDATE account SET online = 0 WHERE id = ?", s.accountID)
	}
}
