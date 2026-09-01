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
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	opcodePing          uint32 = uint32(protocol.OpcodeCMSG_PING)
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
	hiddenGameObjects map[uint64]struct{}
	creatureAuras     map[uint64]map[uint32]struct{}
	channelsMu        sync.RWMutex
	channels          map[string]*worldChannel
}

type session struct {
	server       *Server
	conn         net.Conn
	authSeed     [4]byte
	crypt        *crypto.AuthCrypt
	authed       bool
	accountID    uint32
	accountName  string
	security     uint8
	legitimate   map[uint64]struct{}
	mounts       *MountState
	playerGUID   uint64
	playerLoaded bool
	player       *playerState
	logoutAt     time.Time
	writeMu      sync.Mutex
	selection    uint64
	auras        map[uint32]struct{}
	scale        float32
	emoteState   uint32
	playerLocked bool
	rooted       bool
	attackTarget uint64
	logoutHook   bool
	gossip       *gossipMenuState
	gossipClosed bool
	channels     map[string]struct{}
}

type account struct {
	ID          uint32
	SessionKey  []byte
	LastIP      string
	Locked      bool
	LockCountry string
	OS          string
	Security    uint8
}

func NewServer(stores *database.Set, logger *slog.Logger, realmID uint32, settings ...config.Config) *Server {
	c := config.Default()
	if len(settings) != 0 {
		c = settings[0]
	}
	server := &Server{AuthStore: stores.Auth, CharactersStore: stores.Characters, WorldStore: stores.World, Logger: logger, RealmID: realmID, Config: c, Features: NewFeatures(c, stores, logger), Data: wotlk.NewStore(filepath.Join(c.GameDataDir, "dbc")), sessions: make(map[*session]struct{}), hiddenGameObjects: make(map[uint64]struct{}), creatureAuras: make(map[uint64]map[uint32]struct{}), channels: make(map[string]*worldChannel)}
	server.Features.LFG.SetDungeonValidator(func(id uint32) bool {
		dungeon, found, err := server.Data.LFGDungeon(id)
		return err == nil && found && wotlk.IsSupportedLFGType(dungeon.TypeID)
	})
	server.Features.Scripts.SetPlayerProvider(server.luaPlayers)
	return server
}

func (s *Server) Initialize(ctx context.Context) error {
	return s.Features.Initialize(ctx)
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
		case opcodePing:
			if !state.authed || !state.handlePing(payload) {
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
		case uint32(protocol.OpcodeCMSG_ATTACK_SWING):
			if !state.authed || !state.handleAttackSwing(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_ATTACK_STOP):
			if !state.authed || !state.handleAttackStop() {
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
		case uint32(protocol.OpcodeCMSG_QUESTGIVER_CANCEL):
			if !state.authed || !state.handleQuestgiverCancel() {
				return
			}
		case uint32(protocol.OpcodeCMSG_NAME_QUERY):
			if !state.authed || !state.handleNameQuery(ctx, payload) {
				return
			}
		case uint32(protocol.OpcodeCMSG_GUILD_QUERY):
			if !state.authed || !state.handleGuildQuery(ctx, payload) {
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
	s.debug("world authentication accepted", "account", accountName, "build", build, "remote", remoteAddress(s.conn))
	return s.write(opcodeAuthResponse, []byte{authOK}, true) == nil
}

func (s *session) handlePing(payload []byte) bool {
	b := protocol.NewReader(payload)
	ping, err := b.ReadU32()
	if err != nil {
		return false
	}
	if _, err := b.ReadU32(); err != nil {
		return false
	}
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
	query := "SELECT id, session_key_auth, last_ip, locked, lock_country, os FROM account WHERE username = ? LIMIT 1"
	if store.Backend == database.BackendSQLite {
		query = "SELECT id, session_key_auth, last_ip, locked, lock_country, os FROM account WHERE UPPER(username) = UPPER(?) LIMIT 1"
	}
	err := store.DB.QueryRowContext(ctx, query, username).Scan(&result.ID, &result.SessionKey, &result.LastIP, &locked, &result.LockCountry, &result.OS)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result.Locked = locked != 0
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
	s.server.debug(message, args...)
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
