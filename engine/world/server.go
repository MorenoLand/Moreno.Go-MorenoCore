package world

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"io"
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
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
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
	spiritWaveMu      sync.Mutex
	lastSpiritWave    time.Time
	spiritReviveQueue map[uint64]uint64 // playerGUID -> spiritGuideGUID
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
	auraSlots          map[uint32]uint8
	scale              float32
	emoteState         uint32
	playerLocked       bool
	rooted             bool
	attackTarget       uint64
	duelPartner        uint64
	lastSwing          time.Time
	lastRegenTick      time.Time
	lastCastTime       time.Time
	logoutHook         bool
	gossip             *gossipMenuState
	gossipClosed       bool
	channels           map[string]struct{}
	tutorials          [8]uint32
	tutorialsInDB      bool
	activeLoot         *activeLootState
	trade              *playerTradeState
	guildInvitedID     uint32
	guildInviterGUID   uint64
	groupID            uint64 // GUID of the group this player is in (0 = no group)
	pendingGroupLeader uint64 // GUID of the player who invited us (0 = no invite pending)
	lastStreamX        float32
	lastStreamY        float32
	lastStreamZ        float32
	latency            atomic.Uint32
	lastPing           time.Time
	overSpeedPings     uint32
	deathExpireTime    int64
	deathTimer         time.Time
	resurrection       *resurrectionData
	inFlight           bool
	buyback            []buybackEntry
	arenaTeamInvited   uint32
	bgQueues           [2]bgQueueEntry
	afkReporters       map[uint64]struct{}
	targetGlyphSlot    uint8
}

type bgQueueEntry struct {
	Active     bool
	BgTypeID   uint32
	InstanceID uint32
	JoinTime   time.Time
	Status     uint32
}

type buybackEntry struct {
	ItemEntry uint32
	Count     uint32
	Price     uint32
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
			now := time.Now()
			s.updateActiveCreatures(ctx)
			s.updatePlayerCombat(ctx)
			s.updatePlayerRegeneration(ctx, now)
			s.processCreatureRespawns(ctx, now)
			s.updatePlayerDeathTimers(ctx, now)
			s.updateSpiritHealerResurrectWaves(ctx, now)
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
		swingSpeed := 2 * time.Second
		if sess.player.AttackTime > 0 {
			swingSpeed = time.Duration(sess.player.AttackTime) * time.Millisecond
		}
		if distance3D(sess.player.X, sess.player.Y, sess.player.Z, target.X, target.Y, target.Z) <= meleeAttackRange+2.0 {
			if now.Sub(sess.lastSwing) >= swingSpeed {
				sess.lastSwing = now
				sess.executeMeleeSwing(ctx, target)
			}
		}
	}
}

func (s *Server) updatePlayerRegeneration(ctx context.Context, now time.Time) {
	s.sessionsMu.RLock()
	var activeSessions []*session
	for sess := range s.sessions {
		if sess.playerLoaded && sess.player != nil && sess.player.Health > 0 {
			activeSessions = append(activeSessions, sess)
		}
	}
	s.sessionsMu.RUnlock()

	for _, sess := range activeSessions {
		if sess.lastRegenTick.IsZero() {
			sess.lastRegenTick = now
			continue
		}
		if now.Sub(sess.lastRegenTick) < 2*time.Second {
			continue
		}
		sess.lastRegenTick = now

		p := sess.player
		if p == nil {
			continue
		}

		inCombat := sess.attackTarget != 0
		fields := make(map[int]uint32)
		changed := false

		// 1. Health regeneration (out of combat)
		if !inCombat && p.Health < p.MaxHealth {
			spirit := p.Stats[4]
			gain := uint32(max(1, int(spirit)/2))
			if gain < uint32(p.MaxHealth/25) {
				gain = uint32(p.MaxHealth / 25)
			}
			if gain < 2 {
				gain = 2
			}
			if p.Health+gain >= p.MaxHealth {
				p.Health = p.MaxHealth
			} else {
				p.Health += gain
			}
			fields[unitFieldHealth] = p.Health
			changed = true
		}

		// 2. Power regeneration
		switch p.Class {
		case 1: // Warrior: rage decays out of combat
			if !inCombat && p.Powers[1] > 0 {
				if p.Powers[1] > 20 {
					p.Powers[1] -= 20
				} else {
					p.Powers[1] = 0
				}
				fields[unitFieldPower1+1] = p.Powers[1]
				changed = true
			}
		case 4: // Rogue: energy regenerates 20 per 2s tick
			if p.Powers[3] < 100 {
				if p.Powers[3]+20 >= 100 {
					p.Powers[3] = 100
				} else {
					p.Powers[3] += 20
				}
				fields[unitFieldPower1+3] = p.Powers[3]
				changed = true
			}
		case 6: // Death Knight: runic power decays out of combat
			if !inCombat && p.Powers[6] > 0 {
				if p.Powers[6] > 30 {
					p.Powers[6] -= 30
				} else {
					p.Powers[6] = 0
				}
				fields[unitFieldPower1+6] = p.Powers[6]
				changed = true
			}
		default: // Mana classes (Paladin, Hunter, Priest, Shaman, Mage, Warlock, Druid)
			if p.Powers[0] < p.MaxPowers[0] {
				// Outside 5-second rule
				if now.Sub(sess.lastCastTime) >= 5*time.Second {
					spirit := p.Stats[4]
					intellect := p.Stats[3]
					gain := uint32(max(5, int(spirit)/2+int(intellect)/10))
					if p.Powers[0]+gain >= p.MaxPowers[0] {
						p.Powers[0] = p.MaxPowers[0]
					} else {
						p.Powers[0] += gain
					}
					fields[unitFieldPower1] = p.Powers[0]
					changed = true
				}
			}
		}

		if changed && len(fields) > 0 {
			if packet, err := s.buildPlayerValuesUpdate(sess.playerGUID, fields); err == nil && packet != nil {
				_ = sess.write(packet.Opcode, packet.Payload.Bytes(), true)
				s.broadcastToNearby(packet.Opcode, packet.Payload.Bytes(), sess)
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
	state := &session{server: s, conn: conn, legitimate: make(map[uint64]struct{}), auras: make(map[uint32]struct{}), auraSlots: make(map[uint32]uint8), channels: make(map[string]struct{}), scale: 1}
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
		case uint32(protocol.OpcodeCMSG_KEEP_ALIVE):
			if !state.authed || !state.handleKeepAlive() {
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
		case uint32(protocol.OpcodeCMSG_CANCEL_MOUNT_AURA):
			if !state.authed || !state.handleCancelMountAura(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_GROWTH_AURA):
			if !state.authed || !state.handleCancelGrowthAura(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_AUTO_REPEAT_SPELL):
			if !state.authed || !state.handleCancelAutoRepeatSpell(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CANCEL_TEMP_ENCHANTMENT):
			if !state.authed || !state.handleCancelTempEnchantment(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CORPSE_MAP_POSITION_QUERY):
			if !state.authed || !state.handleCorpseMapPositionQuery(payload) {
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
			if !state.authed || !state.handleRequestRaidInfo(ctx) {
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
		case uint32(protocol.OpcodeCMSG_PLAYER_LOGOUT):
			if !state.authed || !state.handlePlayerLogout() {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOGOUT_CANCEL):
			if !state.authed || !state.handleLogoutCancel() {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_WATCHED_FACTION):
			if !state.authed || !state.handleSetWatchedFaction(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_FACTION_INACTIVE):
			if !state.authed || !state.handleSetFactionInactive(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REPOP_REQUEST):
			if !state.authed || !state.handleRepopRequest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_RECLAIM_CORPSE):
			if !state.authed || !state.handleReclaimCorpse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_RESURRECT_RESPONSE):
			if !state.authed || !state.handleResurrectResponse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SELF_RES):
			if !state.authed || !state.handleSelfRes(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_HEARTH_AND_RESURRECT):
			if !state.authed || !state.handleHearthAndResurrect(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AREA_SPIRIT_HEALER_QUERY):
			if !state.authed || !state.handleAreaSpiritHealerQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AREA_SPIRIT_HEALER_QUEUE):
			if !state.authed || !state.handleAreaSpiritHealerQueue(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_WHO):
			if !state.authed || !state.handleWho(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_WHOIS):
			if !state.authed || !state.handleWhoIs(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_INSPECT):
			if !state.authed || !state.handleInspect(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAT_IGNORED):
			if !state.authed || !state.handleChatIgnored(payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LEARN_TALENT):
			if !state.authed || !state.handleLearnTalent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LEARN_PREVIEW_TALENTS):
			if !state.authed || !state.handleLearnPreviewTalents(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_UNLEARN_SKILL):
			if !state.authed || !state.handleUnlearnSkill(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ITEM_NAME_QUERY):
			if !state.authed || !state.handleItemNameQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ITEM_TEXT_QUERY):
			if !state.authed || !state.handleItemTextQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ITEM_REFUND_INFO):
			if !state.authed || !state.handleItemRefundInfo(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ITEM_REFUND):
			if !state.authed || !state.handleItemRefund(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_USE_ITEM):
			if !state.authed || !state.handleUseItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_EQUIPMENT_SET_SAVE):
			if !state.authed || !state.handleEquipmentSetSave(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_EQUIPMENT_SET_USE):
			if !state.authed || !state.handleEquipmentSetUse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DELETEEQUIPMENT_SET):
			if !state.authed || !state.handleEquipmentSetDelete(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GAMEOBJ_USE):
			if !state.authed || !state.handleGameObjectUse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GAMEOBJ_REPORT_USE):
			if !state.authed || !state.handleGameObjectReportUse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMTICKET_SYSTEMSTATUS):
			if !state.authed || !state.handleGMTicketSystemStatus(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMTICKET_GETTICKET):
			if !state.authed || !state.handleGMTicketGetTicket(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMTICKET_CREATE):
			if !state.authed || !state.handleGMTicketCreate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMTICKET_UPDATETEXT):
			if !state.authed || !state.handleGMTicketUpdate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMTICKET_DELETETICKET):
			if !state.authed || !state.handleGMTicketDelete(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMRESPONSE_RESOLVE):
			if !state.authed || !state.handleGMResponseResolve(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMSURVEY_SUBMIT):
			if !state.authed || !state.handleGMSurveySubmit(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GM_REPORT_LAG):
			if !state.authed || !state.handleGMReportLag(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BANKER_ACTIVATE):
			if !state.authed || !state.handleBankerActivate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BUY_BANK_SLOT):
			if !state.authed || !state.handleBuyBankSlot(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUTOBANK_ITEM):
			if !state.authed || !state.handleAutoBankItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUTOSTORE_BANK_ITEM):
			if !state.authed || !state.handleAutoStoreBankItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AREATRIGGER):
			if !state.authed || !state.handleAreaTrigger(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ALTER_APPEARANCE):
			if !state.authed || !state.handleAlterAppearance(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BINDER_ACTIVATE):
			if !state.authed || !state.handleBinderActivate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BUYBACK_ITEM):
			if !state.authed || !state.handleBuybackItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BUY_STABLE_SLOT):
			if !state.authed || !state.handleBuyStableSlot(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BUG):
			if !state.authed || !state.handleBug(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_AUCTION_LIST_PENDING_SALES):
			if !state.authed || !state.handleAuctionListPendingSales(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ACCEPT_LEVEL_GRANT):
			if !state.authed || !state.handleAcceptLevelGrant(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_QUERY):
			if !state.authed || !state.handleArenaTeamQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_ROSTER):
			if !state.authed || !state.handleArenaTeamRoster(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_INVITE):
			if !state.authed || !state.handleArenaTeamInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_ACCEPT):
			if !state.authed || !state.handleArenaTeamAccept(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_DECLINE):
			if !state.authed || !state.handleArenaTeamDecline(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_LEAVE):
			if !state.authed || !state.handleArenaTeamLeave(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_REMOVE):
			if !state.authed || !state.handleArenaTeamRemove(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_DISBAND):
			if !state.authed || !state.handleArenaTeamDisband(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ARENA_TEAM_LEADER):
			if !state.authed || !state.handleArenaTeamLeader(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEMASTER_HELLO):
			if !state.authed || !state.handleBattlemasterHello(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEFIELD_LIST):
			if !state.authed || !state.handleBattlefieldList(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEMASTER_JOIN):
			if !state.authed || !state.handleBattlemasterJoin(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEMASTER_JOIN_ARENA):
			if !state.authed || !state.handleBattlemasterJoinArena(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEFIELD_PORT):
			if !state.authed || !state.handleBattlefieldPort(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEFIELD_STATUS):
			if !state.authed || !state.handleBattlefieldStatus(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEFIELD_MGR_ENTRY_INVITE_RESPONSE):
			if !state.authed || !state.handleBfEntryInviteResponse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEFIELD_MGR_QUEUE_INVITE_RESPONSE):
			if !state.authed || !state.handleBfQueueInviteResponse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_BATTLEFIELD_MGR_EXIT_REQUEST):
			if !state.authed || !state.handleBfQueueExitRequest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_GET_CALENDAR):
			if !state.authed || !state.handleCalendarGetCalendar(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_GET_NUM_PENDING):
			if !state.authed || !state.handleCalendarGetNumPending(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_GET_EVENT):
			if !state.authed || !state.handleCalendarGetEvent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_GUILD_FILTER):
			if !state.authed || !state.handleCalendarGuildFilter(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_ARENA_TEAM):
			if !state.authed || !state.handleCalendarArenaTeam(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_ADD_EVENT):
			if !state.authed || !state.handleCalendarAddEvent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_UPDATE_EVENT):
			if !state.authed || !state.handleCalendarUpdateEvent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_REMOVE_EVENT):
			if !state.authed || !state.handleCalendarRemoveEvent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_COPY_EVENT):
			if !state.authed || !state.handleCalendarCopyEvent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_EVENT_INVITE):
			if !state.authed || !state.handleCalendarEventInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_EVENT_SIGNUP):
			if !state.authed || !state.handleCalendarEventSignup(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_EVENT_RSVP):
			if !state.authed || !state.handleCalendarEventRSVP(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_EVENT_REMOVE_INVITE):
			if !state.authed || !state.handleCalendarEventRemoveInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_EVENT_STATUS):
			if !state.authed || !state.handleCalendarEventStatus(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_EVENT_MODERATOR_STATUS):
			if !state.authed || !state.handleCalendarEventModeratorStatus(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CALENDAR_COMPLAIN):
			if !state.authed || !state.handleCalendarComplain(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_PASSWORD):
			if !state.authed || !state.handleChannelPassword(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_SET_OWNER):
			if !state.authed || !state.handleChannelSetOwner(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_OWNER):
			if !state.authed || !state.handleChannelOwner(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_MODERATOR):
			if !state.authed || !state.handleChannelModerator(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_UNMODERATOR):
			if !state.authed || !state.handleChannelUnmoderator(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_MUTE):
			if !state.authed || !state.handleChannelMute(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_UNMUTE):
			if !state.authed || !state.handleChannelUnmute(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_INVITE):
			if !state.authed || !state.handleChannelInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_KICK):
			if !state.authed || !state.handleChannelKick(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_BAN):
			if !state.authed || !state.handleChannelBan(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_UNBAN):
			if !state.authed || !state.handleChannelUnban(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_ANNOUNCEMENTS):
			if !state.authed || !state.handleChannelAnnouncements(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANNEL_VOICE_ON):
			if !state.authed || !state.handleChannelVoiceOn(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DECLINE_CHANNEL_INVITE):
			if !state.authed || !state.handleDeclineChannelInvite(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_RENAME):
			if !state.authed || !state.handleCharRename(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_CUSTOMIZE):
			if !state.authed || !state.handleCharCustomize(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_RACE_CHANGE):
			if !state.authed || !state.handleCharRaceChange(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHAR_FACTION_CHANGE):
			if !state.authed || !state.handleCharFactionChange(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_COMPLETE_MOVIE):
			if !state.authed || !state.handleCompleteMovie(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_COMPLAIN):
			if !state.authed || !state.handleComplain(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_CREATE):
			if !state.authed || !state.handleGuildCreate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_INFO):
			if !state.authed || !state.handleGuildInfo(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_PROMOTE):
			if !state.authed || !state.handleGuildPromote(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_DEMOTE):
			if !state.authed || !state.handleGuildDemote(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_LEADER):
			if !state.authed || !state.handleGuildLeader(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_REMOVE):
			if !state.authed || !state.handleGuildRemove(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_DISBAND):
			if !state.authed || !state.handleGuildDisband(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_ADD_RANK):
			if !state.authed || !state.handleGuildAddRank(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_DEL_RANK):
			if !state.authed || !state.handleGuildDelRank(ctx) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_RANK):
			if !state.authed || !state.handleGuildRank(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_SET_PUBLIC_NOTE):
			if !state.authed || !state.handleGuildSetPublicNote(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_SET_OFFICER_NOTE):
			if !state.authed || !state.handleGuildSetOfficerNote(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_INFO_TEXT):
			if !state.authed || !state.handleGuildInfoText(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANKER_ACTIVATE):
			if !state.authed || !state.handleGuildBankerActivate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANK_SWAP_ITEMS):
			if !state.authed || !state.handleGuildBankSwapItems(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANK_BUY_TAB):
			if !state.authed || !state.handleGuildBankBuyTab(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANK_UPDATE_TAB):
			if !state.authed || !state.handleGuildBankUpdateTab(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANK_DEPOSIT_MONEY):
			if !state.authed || !state.handleGuildBankDepositMoney(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_BANK_WITHDRAW_MONEY):
			if !state.authed || !state.handleGuildBankWithdrawMoney(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_GUILD_BANK_LOG_QUERY):
			if !state.authed || !state.handleGuildBankLogQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_GUILD_BANK_MONEY_WITHDRAWN):
			if !state.authed || !state.handleGuildBankMoneyWithdrawn(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_QUERY_GUILD_BANK_TEXT):
			if !state.authed || !state.handleQueryGuildBankText(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_GUILD_BANK_TEXT):
			if !state.authed || !state.handleSetGuildBankText(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DUEL_ACCEPTED):
			if !state.authed || !state.handleDuelAccepted(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DUEL_CANCELLED):
			if !state.authed || !state.handleDuelCancelled(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_FAR_SIGHT):
			if !state.authed || !state.handleFarSight(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_FORCE_MOVE_ROOT_ACK):
			if !state.authed || !state.handleForceMoveRootAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_FORCE_MOVE_UNROOT_ACK):
			if !state.authed || !state.handleForceMoveUnrootAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_FORCE_TURN_RATE_CHANGE_ACK):
			if !state.authed || !state.handleForceTurnRateChangeAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GET_CHANNEL_MEMBER_COUNT):
			if !state.authed || !state.handleGetChannelMemberCount(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GET_MIRRORIMAGE_DATA):
			if !state.authed || !state.handleGetMirrorImageData(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GMTICKETSYSTEM_TOGGLE):
			if !state.authed || !state.handleGmTicketSystemToggle(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GRANT_LEVEL):
			if !state.authed || !state.handleGrantLevel(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_ASSISTANT_LEADER):
			if !state.authed || !state.handleGroupAssistantLeader(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GROUP_CHANGE_SUB_GROUP):
			if !state.authed || !state.handleGroupChangeSubGroup(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ENABLETAXI):
			if !state.authed || !state.handleEnableTaxi(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DISMISS_CRITTER):
			if !state.authed || !state.handleDismissCritter(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE):
			if !state.authed || !state.handleChangeSeatsOnControlledVehicle(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_CONTROLLER_EJECT_PASSENGER):
			if !state.authed || !state.handleControllerEjectPassenger(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_DISMISS_CONTROLLED_VEHICLE):
			if !state.authed || !state.handleDismissControlledVehicle(ctx, payload) {
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

		// Petitions & Guild Tabards
		case uint32(protocol.OpcodeCMSG_PETITION_BUY):
			if !state.authed || !state.handlePetitionBuy(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PETITION_SHOW_SIGNATURES):
			if !state.authed || !state.handlePetitionShowSignatures(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PETITION_QUERY):
			if !state.authed || !state.handlePetitionQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PETITION_SIGN):
			if !state.authed || !state.handlePetitionSign(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TURN_IN_PETITION):
			if !state.authed || !state.handleTurnInPetition(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_OFFER_PETITION):
			if !state.authed || !state.handleOfferPetition(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PETITION_SHOWLIST):
			if !state.authed || !state.handlePetitionShowList(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_PETITION_DECLINE):
			if !state.authed || !state.handlePetitionDecline(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_PETITION_RENAME):
			if !state.authed || !state.handlePetitionRename(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_TABARDVENDOR_ACTIVATE):
			if !state.authed || !state.handleTabardVendorActivate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_SAVE_GUILD_EMBLEM):
			if !state.authed || !state.handleSaveGuildEmblem(ctx, payload) {
				return
			}

		// Movement ACKs, Summons & Animations
		case uint32(protocol.OpcodeCMSG_MOVE_FEATHER_FALL_ACK):
			if !state.authed || !state.handleMoveFeatherFallAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_HOVER_ACK):
			if !state.authed || !state.handleMoveHoverAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_WATER_WALK_ACK):
			if !state.authed || !state.handleMoveWaterWalkAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_KNOCK_BACK_ACK):
			if !state.authed || !state.handleMoveKnockBackAck(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_NOT_ACTIVE_MOVER):
			if !state.authed || !state.handleMoveNotActiveMover(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_FALL_RESET):
			if !state.authed || !state.handleMoveFallReset(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_SPLINE_DONE):
			if !state.authed || !state.handleMoveSplineDone(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_CHNG_TRANSPORT):
			if !state.authed || !state.handleMoveChngTransport(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_SET_FLY):
			if !state.authed || !state.handleMoveSetFly(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOVE_TIME_SKIPPED):
			if !state.authed || !state.handleMoveTimeSkipped(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SUMMON_RESPONSE):
			if !state.authed || !state.handleSummonResponse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MOUNTSPECIAL_ANIM):
			if !state.authed || !state.handleMountSpecialAnim(ctx, payload) {
				return
			}

		// Vehicle Passengers & Seats
		case uint32(protocol.OpcodeCMSG_PLAYER_VEHICLE_ENTER):
			if !state.authed || !state.handlePlayerVehicleEnter(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_VEHICLE_EXIT):
			if !state.authed || !state.handleRequestVehicleExit(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_VEHICLE_NEXT_SEAT):
			if !state.authed || !state.handleRequestVehicleNextSeat(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_VEHICLE_PREV_SEAT):
			if !state.authed || !state.handleRequestVehiclePrevSeat(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_VEHICLE_SWITCH_SEAT):
			if !state.authed || !state.handleRequestVehicleSwitchSeat(ctx, payload) {
				return
			}

		// Items & Page Text
		case uint32(protocol.OpcodeCMSG_OPEN_ITEM):
			if !state.authed || !state.handleOpenItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_READ_ITEM):
			if !state.authed || !state.handleReadItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PAGE_TEXT_QUERY):
			if !state.authed || !state.handlePageTextQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_WRAP_ITEM):
			if !state.authed || !state.handleWrapItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REPAIR_ITEM):
			if !state.authed || !state.handleRepairItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SOCKET_GEMS):
			if !state.authed || !state.handleSocketGems(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_AMMO):
			if !state.authed || !state.handleSetAmmo(ctx, payload) {
				return
			}

		// Character Display, Titles & PvP
		case uint32(protocol.OpcodeCMSG_SHOWING_CLOAK):
			if !state.authed || !state.handleShowingCloak(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SHOWING_HELM):
			if !state.authed || !state.handleShowingHelm(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_TITLE):
			if !state.authed || !state.handleSetTitle(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_TOGGLE_PVP):
			if !state.authed || !state.handleTogglePvP(ctx, payload) {
				return
			}

		// Instances & Difficulty
		case uint32(protocol.OpcodeCMSG_RESET_INSTANCES):
			if !state.authed || !state.handleResetInstances(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_SET_DUNGEON_DIFFICULTY):
			if !state.authed || !state.handleSetDungeonDifficulty(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_SET_RAID_DIFFICULTY):
			if !state.authed || !state.handleSetRaidDifficulty(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_INSTANCE_LOCK_RESPONSE):
			if !state.authed || !state.handleInstanceLockResponse(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_SAVED_INSTANCE_EXTEND):
			if !state.authed || !state.handleSetSavedInstanceExtend(ctx, payload) {
				return
			}

		// Guild Permissions, Event Log & Inspect
		case uint32(protocol.OpcodeMSG_GUILD_EVENT_LOG_QUERY):
			if !state.authed || !state.handleGuildEventLogQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_GUILD_PERMISSIONS):
			if !state.authed || !state.handleGuildPermissions(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_INSPECT_ARENA_TEAMS):
			if !state.authed || !state.handleInspectArenaTeams(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_INSPECT_HONOR_STATS):
			if !state.authed || !state.handleInspectHonorStats(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_PVP_LOG_DATA):
			if !state.authed || !state.handlePvpLogData(ctx, payload) {
				return
			}

		// Spirit Healer & Corpse
		case uint32(protocol.OpcodeCMSG_SPIRIT_HEALER_ACTIVATE):
			if !state.authed || !state.handleSpiritHealerActivate(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_CORPSE_QUERY):
			if !state.authed || !state.handleCorpseQuery(ctx, payload) {
				return
			}

		// Spells & Talents
		case uint32(protocol.OpcodeCMSG_TOTEM_DESTROYED):
			if !state.authed || !state.handleTotemDestroyed(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SPELLCLICK):
			if !state.authed || !state.handleSpellClick(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_TALENT_WIPE_CONFIRM):
			if !state.authed || !state.handleTalentWipeConfirm(ctx, payload) {
				return
			}

		// Quests & Inspect Achievements
		case uint32(protocol.OpcodeCMSG_QUEST_CONFIRM_ACCEPT):
			if !state.authed || !state.handleQuestConfirmAccept(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUEST_POI_QUERY):
			if !state.authed || !state.handleQuestPoiQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUERY_QUESTS_COMPLETED):
			if !state.authed || !state.handleQueryQuestsCompleted(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTLOG_SWAP_QUEST):
			if !state.authed || !state.handleQuestlogSwapQuest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PUSHQUESTTOPARTY):
			if !state.authed || !state.handlePushQuestToParty(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_QUEST_PUSH_RESULT):
			if !state.authed || !state.handleQuestPushResult(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_STATUS_MULTIPLE_QUERY):
			if !state.authed || !state.handleQuestgiverStatusMultipleQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_QUERY_INSPECT_ACHIEVEMENTS):
			if !state.authed || !state.handleQueryInspectAchievements(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_RAID_READY_CHECK_FINISHED):
			if !state.authed || !state.handleRaidReadyCheckFinished(ctx, payload) {
				return
			}

		// Pets & Pet Stabling
		case uint32(protocol.OpcodeCMSG_PET_ABANDON):
			if !state.authed || !state.handlePetAbandon(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_ACTION):
			if !state.authed || !state.handlePetAction(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_CANCEL_AURA):
			if !state.authed || !state.handlePetCancelAura(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_CAST_SPELL):
			if !state.authed || !state.handlePetCastSpell(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_LEARN_TALENT):
			if !state.authed || !state.handlePetLearnTalent(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_NAME_QUERY):
			if !state.authed || !state.handlePetNameQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_RENAME):
			if !state.authed || !state.handlePetRename(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_SET_ACTION):
			if !state.authed || !state.handlePetSetAction(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_SPELL_AUTOCAST):
			if !state.authed || !state.handlePetSpellAutocast(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_PET_STOP_ATTACK):
			if !state.authed || !state.handlePetStopAttack(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REQUEST_PET_INFO):
			if !state.authed || !state.handleRequestPetInfo(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_STABLE_PET):
			if !state.authed || !state.handleStablePet(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_STABLE_REVIVE_PET):
			if !state.authed || !state.handleStableRevivePet(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_STABLE_SWAP_PET):
			if !state.authed || !state.handleStableSwapPet(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_UNSTABLE_PET):
			if !state.authed || !state.handleUnstablePet(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_LIST_STABLED_PETS):
			if !state.authed || !state.handleListStabledPets(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LEARN_PREVIEW_TALENTS_PET):
			if !state.authed || !state.handleLearnPreviewTalentsPet(ctx, payload) {
				return
			}

		// LFG / Dungeon Finder
		case uint32(protocol.OpcodeCMSG_LFD_PARTY_LOCK_INFO_REQUEST):
			if !state.authed || !state.handleLfdPartyLockInfoRequest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFD_PLAYER_LOCK_INFO_REQUEST):
			if !state.authed || !state.handleLfdPlayerLockInfoRequest(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_PROPOSAL_RESULT):
			if !state.authed || !state.handleLfgProposalResult(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_SET_BOOT_VOTE):
			if !state.authed || !state.handleLfgSetBootVote(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_SET_ROLES):
			if !state.authed || !state.handleLfgSetRoles(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LFG_TELEPORT):
			if !state.authed || !state.handleLfgTeleport(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SEARCH_LFG_JOIN):
			if !state.authed || !state.handleSearchLfgJoin(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SEARCH_LFG_LEAVE):
			if !state.authed || !state.handleSearchLfgLeave(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_LFG_COMMENT):
			if !state.authed || !state.handleSetLfgComment(ctx, payload) {
				return
			}

		// Loot
		case uint32(protocol.OpcodeCMSG_LOOT_MASTER_GIVE):
			if !state.authed || !state.handleLootMasterGive(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_LOOT_ROLL):
			if !state.authed || !state.handleLootRoll(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_OPT_OUT_OF_LOOT):
			if !state.authed || !state.handleOptOutOfLoot(ctx, payload) {
				return
			}

		// Mail
		case uint32(protocol.OpcodeCMSG_MAIL_CREATE_TEXT_ITEM):
			if !state.authed || !state.handleMailCreateTextItem(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_MAIL_RETURN_TO_SENDER):
			if !state.authed || !state.handleMailReturnToSender(ctx, payload) {
				return
			}

		// Battleground & PvP
		case uint32(protocol.OpcodeCMSG_LEAVE_BATTLEFIELD):
			if !state.authed || !state.handleLeaveBattlefield(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_REPORT_PVP_AFK):
			if !state.authed || !state.handleReportPvPAfk(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeMSG_BATTLEGROUND_PLAYER_POSITIONS):
			if !state.authed || !state.handleBattlegroundPlayerPositions(ctx, payload) {
				return
			}

		// Spells & Glyphs
		case uint32(protocol.OpcodeCMSG_REMOVE_GLYPH):
			if !state.authed || !state.handleRemoveGlyph(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_UPDATE_MISSILE_TRAJECTORY):
			if !state.authed || !state.handleUpdateMissileTrajectory(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_UPDATE_PROJECTILE_POSITION):
			if !state.authed || !state.handleUpdateProjectilePosition(ctx, payload) {
				return
			}

		// Group
		case uint32(protocol.OpcodeCMSG_REQUEST_PARTY_MEMBER_STATS):
			if !state.authed || !state.handleRequestPartyMemberStats(ctx, payload) {
				return
			}

		// Voice & Channels
		case uint32(protocol.OpcodeCMSG_SET_ACTIVE_VOICE_CHANNEL):
			if !state.authed || !state.handleSetActiveVoiceChannel(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_VOICE_SESSION_ENABLE):
			if !state.authed || !state.handleVoiceSessionEnable(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_SET_CHANNEL_WATCH):
			if !state.authed || !state.handleSetChannelWatch(ctx, payload) {
				return
			}

		// Commands & Admin
		case uint32(protocol.OpcodeCMSG_SET_FACTION_CHEAT):
			if !state.authed || !state.handleSetFactionCheat(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_WORLD_TELEPORT):
			if !state.authed || !state.handleWorldTeleport(ctx, payload) {
				return
			}

		// Character Declined Names
		case uint32(protocol.OpcodeCMSG_SET_PLAYER_DECLINED_NAMES):
			if !state.authed || !state.handleSetPlayerDeclinedNames(ctx, payload) {
				return
			}

		// Taxi
		case uint32(protocol.OpcodeCMSG_SET_TAXI_BENCHMARK_MODE):
			if !state.authed || !state.handleSetTaxiBenchmarkMode(ctx, payload) {
				return
			}

		// Warden
		case uint32(protocol.OpcodeCMSG_WARDEN_DATA):
			if !state.authed || !state.handleWardenData(ctx, payload) {
				return
			}

		case uint32(protocol.OpcodeMSG_MOVE_START_FORWARD), uint32(protocol.OpcodeMSG_MOVE_START_BACKWARD), uint32(protocol.OpcodeMSG_MOVE_STOP), uint32(protocol.OpcodeMSG_MOVE_START_STRAFE_LEFT), uint32(protocol.OpcodeMSG_MOVE_START_STRAFE_RIGHT), uint32(protocol.OpcodeMSG_MOVE_STOP_STRAFE), uint32(protocol.OpcodeMSG_MOVE_JUMP), uint32(protocol.OpcodeMSG_MOVE_START_TURN_LEFT), uint32(protocol.OpcodeMSG_MOVE_START_TURN_RIGHT), uint32(protocol.OpcodeMSG_MOVE_STOP_TURN), uint32(protocol.OpcodeMSG_MOVE_START_PITCH_UP), uint32(protocol.OpcodeMSG_MOVE_START_PITCH_DOWN), uint32(protocol.OpcodeMSG_MOVE_STOP_PITCH), uint32(protocol.OpcodeMSG_MOVE_SET_RUN_MODE), uint32(protocol.OpcodeMSG_MOVE_SET_WALK_MODE), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), uint32(protocol.OpcodeMSG_MOVE_START_SWIM), uint32(protocol.OpcodeMSG_MOVE_STOP_SWIM), uint32(protocol.OpcodeMSG_MOVE_ROOT), uint32(protocol.OpcodeMSG_MOVE_UNROOT), uint32(protocol.OpcodeMSG_MOVE_HEARTBEAT), uint32(protocol.OpcodeMSG_MOVE_HOVER), uint32(protocol.OpcodeMSG_MOVE_SET_FACING), uint32(protocol.OpcodeMSG_MOVE_SET_PITCH), uint32(protocol.OpcodeMSG_MOVE_START_ASCEND), uint32(protocol.OpcodeMSG_MOVE_START_DESCEND), uint32(protocol.OpcodeMSG_MOVE_STOP_ASCEND), uint32(protocol.OpcodeMSG_MOVE_GRAVITY_CHNG):
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
	if s.server.Config.Expansion > 0 && s.accountExpansion > uint8(s.server.Config.Expansion) {
		s.accountExpansion = uint8(s.server.Config.Expansion)
	}
	if s.accountExpansion == 0 && s.server.Config.Expansion > 0 {
		s.accountExpansion = uint8(s.server.Config.Expansion)
	}
	if s.accountExpansion == 0 {
		s.accountExpansion = 2 // default to WotLK
	}
	s.debug("world authentication accepted", "account", accountName, "build", build, "expansion", s.accountExpansion, "gm_chat", s.gmChat, "two_side_chat", s.twoSideChat, "remote", remoteAddress(s.conn))
	s.loadTutorials(ctx)

	// SMSG_AUTH_RESPONSE: 11-byte short form for AUTH_OK (TrinityCore AuthHandler.cpp)
	authBuf := protocol.NewBuffer(11)
	authBuf.WriteU8(authOK)
	authBuf.WriteU32(0)                 // BillingTimeRemaining
	authBuf.WriteU8(0)                  // BillingPlanFlags
	authBuf.WriteU32(0)                 // BillingTimeRested
	authBuf.WriteU8(s.accountExpansion) // 0 Vanilla, 1 TBC, 2 WotLK
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

// handleKeepAlive mirrors WorldSocket::ReadDataHandler case CMSG_KEEP_ALIVE (WorldSocket.cpp:348).
// An empty client heartbeat packet resetting the session activity timeout.
func (s *session) handleKeepAlive() bool {
	return true
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
		if s.server != nil {
			s.server.broadcastFriendStatus(s.playerGUID, friendsResultOffline, 0, 0, 0)
		}
	}
	if s.accountID != 0 {
		_, _ = s.server.AuthStore.DB.ExecContext(ctx, "UPDATE account SET online = 0 WHERE id = ?", s.accountID)
	}
}

type Features struct {
	Config  config.Config
	LFG     *LFGManager
	NPCBots *NPCBotManager
	Scripts *scripting.Runtime
}

func NewFeatures(c config.Config, stores *database.Set, logger *slog.Logger) *Features {
	return &Features{Config: c, LFG: NewLFGManager(c.SoloLFGEnable), NPCBots: NewNPCBotManager(stores.Characters, stores.World, c.NPCBots), Scripts: scripting.NewRuntime(scripting.Config{Enabled: c.LuaEnabled, ScriptPath: c.LuaScriptPath, CoreExpansion: c.Expansion, AuthDatabase: stores.Auth.DB, CharacterDB: stores.Characters.DB, WorldDatabase: stores.World.DB, Logger: logger})}
}

func (f *Features) Initialize(ctx context.Context) error {
	if err := f.NPCBots.Initialize(ctx); err != nil {
		return err
	}
	return f.Scripts.Load(ctx)
}

func (f *Features) OnPlayerLogin() {
	f.LFG.OnLogin()
}

func (s *session) handleNameQuery(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		s.debug("name query rejected", "account", s.accountName, "error", err)
		return false
	}
	packet := protocol.NewBuffer(32)
	packet.WritePackedGUID(guid)
	var name string
	var race, gender, class int64
	if s.player != nil && s.player.GUID == guid {
		name = s.player.Name
		race = int64(s.player.Race)
		gender = int64(s.player.Gender)
		class = int64(s.player.Class)
	} else if online := s.server.findSessionByGUID(guid); online != nil && online.player != nil {
		name = online.player.Name
		race = int64(online.player.Race)
		gender = int64(online.player.Gender)
		class = int64(online.player.Class)
	} else {
		err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT name, race, gender, class FROM characters WHERE guid = ? AND (deleteInfos_Name IS NULL OR deleteInfos_Name = '')", guid).Scan(&name, &race, &gender, &class)
		if errors.Is(err, sql.ErrNoRows) {
			packet.WriteU8(1)
			return s.write(uint16(protocol.OpcodeSMSG_NAME_QUERY_RESPONSE), packet.Bytes(), true) == nil
		} else if err != nil {
			s.debug("name query failed", "account", s.accountName, "guid", guid, "error", err)
			return false
		}
	}
	packet.WriteU8(0)
	packet.WriteCString(name)
	packet.WriteU8(0)
	packet.WriteU8(uint8(race))
	packet.WriteU8(uint8(gender))
	packet.WriteU8(uint8(class))
	packet.WriteU8(0)
	return s.write(uint16(protocol.OpcodeSMSG_NAME_QUERY_RESPONSE), packet.Bytes(), true) == nil
}

func (s *session) handleQueryTime() bool {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	packet := protocol.NewBuffer(8)
	packet.WriteU32(uint32(now.Unix()))
	packet.WriteU32(uint32(next.Sub(now).Seconds()))
	return s.write(uint16(protocol.OpcodeSMSG_QUERY_TIME_RESPONSE), packet.Bytes(), true) == nil
}

func (s *session) handlePlayedTime(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	trigger, err := reader.ReadU8()
	if err != nil {
		return false
	}
	var total, level int64
	err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT totaltime, leveltime FROM characters WHERE guid = ? AND account = ?", s.playerGUID, s.accountID).Scan(&total, &level)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.debug("played time query failed", "account", s.accountName, "error", err)
		return false
	}
	packet := protocol.NewBuffer(9)
	packet.WriteU32(uint32(total))
	packet.WriteU32(uint32(level))
	packet.WriteU8(trigger)
	return s.write(uint16(protocol.OpcodeSMSG_PLAYED_TIME), packet.Bytes(), true) == nil
}

func (s *session) handleZoneUpdate(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	zone, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	s.updateLocalChannels(zone)
	s.streamNearbyObjects(ctx)
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_ZONE", zone, s.playerGUID); err != nil {
		s.debug("zone update failed", "account", s.accountName, "zone", zone, "error", err)
		return false
	}
	return true
}

func (s *session) handleSetActionBarToggles(payload []byte) bool {
	reader := protocol.NewReader(payload)
	toggles, err := reader.ReadU8()
	if err != nil {
		return false
	}
	if s.player != nil {
		s.player.ActionBars = uint32(toggles)
	}
	return true
}

func (s *session) handleSetActionButton(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	button, err := reader.ReadU8()
	if err != nil {
		return false
	}
	data, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if button >= 144 || !s.playerLoaded || s.player == nil {
		return true
	}
	s.player.Actions[button] = data
	spec := int64(s.player.ActiveTalentGroup)
	if data == 0 {
		_, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_DEL_CHAR_ACTION_BY_BUTTON_SPEC", s.playerGUID, button, spec)
		if err != nil && s.server.CharactersStore.DB != nil {
			_, err = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_action WHERE guid = ? AND button = ? AND spec = ?", s.playerGUID, button, spec)
		}
	} else {
		var action, kind int64
		action = int64(data & 0x00FFFFFF)
		kind = int64(data >> 24)
		result, updateErr := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_CHAR_ACTION", action, kind, s.playerGUID, button, spec)
		err = updateErr
		if err == nil && result != nil {
			var affected int64
			affected, err = result.RowsAffected()
			if err == nil && affected == 0 {
				_, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_INS_CHAR_ACTION", s.playerGUID, spec, button, action, kind)
			}
		} else if s.server.CharactersStore.DB != nil {
			res, dErr := s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_action SET action = ?, type = ? WHERE guid = ? AND button = ? AND spec = ?", action, kind, s.playerGUID, button, spec)
			if dErr == nil {
				if aff, _ := res.RowsAffected(); aff == 0 {
					_, err = s.server.CharactersStore.DB.ExecContext(ctx, "INSERT INTO character_action (guid, spec, button, action, type) VALUES (?, ?, ?, ?, ?)", s.playerGUID, spec, button, action, kind)
				}
			} else {
				err = dErr
			}
		}
	}
	if err != nil {
		s.debug("action button update failed", "account", s.accountName, "button", button, "error", err)
		return false
	}
	return true
}

func (s *session) handleUpdateAccountData(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	typeID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	timestamp, err := reader.ReadU32()
	if err != nil {
		return false
	}
	decompressedSize, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if typeID >= 8 || decompressedSize > 0xFFFF {
		return true
	}
	compressed, err := reader.Read(reader.Remaining())
	if err != nil {
		return false
	}
	data, err := decompressAccountData(compressed, decompressedSize)
	if err != nil {
		s.debug("account data decompression failed", "account", s.accountName, "type", typeID, "error", err)
		return true
	}
	var result sql.Result
	if globalAccountDataMask&(1<<typeID) != 0 {
		result, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_REP_ACCOUNT_DATA", s.accountID, typeID, timestamp, data)
	} else if s.playerLoaded {
		result, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_REP_PLAYER_ACCOUNT_DATA", s.playerGUID, typeID, timestamp, data)
	}
	_ = result
	if err != nil {
		s.debug("account data update failed", "account", s.accountName, "type", typeID, "error", err)
		return false
	}
	response := protocol.NewBuffer(8)
	response.WriteU32(typeID)
	response.WriteU32(0)
	return s.write(uint16(protocol.OpcodeSMSG_UPDATE_ACCOUNT_DATA_COMPLETE), response.Bytes(), true) == nil
}

func (s *session) handleRequestAccountData(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	typeID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if typeID >= 8 {
		return true
	}
	var timestamp int64
	var data []byte
	if globalAccountDataMask&(1<<typeID) != 0 {
		err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT time, data FROM account_data WHERE accountId = ? AND type = ?", s.accountID, typeID).Scan(&timestamp, &data)
	} else if s.playerLoaded {
		err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT time, data FROM character_account_data WHERE guid = ? AND type = ?", s.playerGUID, typeID).Scan(&timestamp, &data)
	} else {
		err = sql.ErrNoRows
	}
	if errors.Is(err, sql.ErrNoRows) {
		timestamp = 0
		data = nil
	} else if err != nil {
		s.debug("account data request failed", "account", s.accountName, "type", typeID, "error", err)
		return false
	}
	compressed, err := compressAccountData(data)
	if err != nil {
		return false
	}
	packet := protocol.NewBuffer(24 + len(compressed))
	if s.playerLoaded {
		packet.WriteU64(s.playerGUID)
	} else {
		packet.WriteU64(0)
	}
	packet.WriteU32(typeID)
	packet.WriteU32(uint32(timestamp))
	packet.WriteU32(uint32(len(data)))
	packet.Write(compressed)
	return s.write(uint16(protocol.OpcodeSMSG_UPDATE_ACCOUNT_DATA), packet.Bytes(), true) == nil
}

func (s *session) handleWorldStateUITimer() bool {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(uint32(time.Now().Unix()))
	return s.write(uint16(protocol.OpcodeSMSG_WORLD_STATE_UI_TIMER_UPDATE), packet.Bytes(), true) == nil
}

func (s *session) handleRequestRaidInfo(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		packet := protocol.NewBuffer(4)
		packet.WriteU32(0)
		return s.write(uint16(protocol.OpcodeSMSG_RAID_INSTANCE_INFO), packet.Bytes(), true) == nil
	}

	type raidLock struct {
		mapID      uint32
		difficulty uint32
		instanceID uint64
		expired    uint8
		extended   uint8
		resetTime  uint32
	}
	var locks []raidLock

	now := time.Now().Unix()
	rows, err := cdb.QueryContext(ctx, `SELECT i.map, i.difficulty, i.id, COALESCE(ci.extendState, 0), i.resettime
		FROM character_instance ci
		JOIN instance i ON i.id = ci.instance
		WHERE ci.guid = ? AND ci.permanent = 1`, s.playerGUID)
	if err == nil {
		for rows.Next() {
			var mapID, diff, instID, extendState, resetTime int64
			if err := rows.Scan(&mapID, &diff, &instID, &extendState, &resetTime); err == nil {
				rem := int64(0)
				if resetTime > now {
					rem = resetTime - now
				}
				expired := uint8(0)
				if rem == 0 && extendState != 2 { // 2 = EXTEND_STATE_EXTENDED
					expired = 1
				}
				extended := uint8(0)
				if extendState == 2 {
					extended = 1
				}
				locks = append(locks, raidLock{
					mapID:      uint32(mapID),
					difficulty: uint32(diff),
					instanceID: uint64(instID),
					expired:    expired,
					extended:   extended,
					resetTime:  uint32(rem),
				})
			}
		}
		rows.Close()
	}

	packet := protocol.NewBuffer(4 + len(locks)*22)
	packet.WriteU32(uint32(len(locks)))
	for _, l := range locks {
		packet.WriteU32(l.mapID)
		packet.WriteU32(l.difficulty)
		packet.WriteU64(l.instanceID)
		packet.WriteU8(1 - l.expired) // 1 = not expired, 0 = expired (Player.cpp:19231)
		packet.WriteU8(l.extended)
		packet.WriteU32(l.resetTime)
	}
	return s.write(uint16(protocol.OpcodeSMSG_RAID_INSTANCE_INFO), packet.Bytes(), true) == nil
}

func decompressAccountData(compressed []byte, expected uint32) ([]byte, error) {
	if expected == 0 {
		return nil, nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, int64(expected)+1))
	if err != nil {
		return nil, err
	}
	if uint32(len(data)) != expected {
		return nil, fmt.Errorf("decompressed size %d does not match %d", len(data), expected)
	}
	return data, nil
}

func compressAccountData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

// handleWardenData processes CMSG_WARDEN_DATA (0x2E7).
// Reference: WorldSession::HandleWardenDataOpcode (WardenHandler.cpp:25).
func (s *session) handleWardenData(ctx context.Context, payload []byte) bool {
	return true
}
