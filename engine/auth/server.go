package auth

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.TrinityGo/engine/crypto"
	"github.com/MorenoLand/Moreno.TrinityGo/engine/database"
	"github.com/MorenoLand/Moreno.TrinityGo/pkg/protocol"
)

const (
	logonChallenge        byte   = 0x00
	logonProof            byte   = 0x01
	reconnectChallenge    byte   = 0x02
	reconnectProof        byte   = 0x03
	realmList             byte   = 0x10
	wowSuccess            byte   = 0x00
	wowBanned             byte   = 0x03
	wowUnknownAccount     byte   = 0x04
	wowDBBusy             byte   = 0x08
	wowVersionInvalid     byte   = 0x09
	wowSuspended          byte   = 0x0c
	wowLockedEnforced     byte   = 0x10
	wowUnlockableLock     byte   = 0x19
	preBCMaxBuild         uint32 = 6141
	realmFlagOffline      uint32 = 0x02
	realmFlagSpecifyBuild uint32 = 0x04
)

var versionChallenge = [16]byte{0xBA, 0xA3, 0x1E, 0x99, 0xA0, 0x0B, 0x21, 0x57, 0xFC, 0x37, 0x3F, 0xB3, 0x69, 0xCD, 0xD2, 0xF1}

type Server struct {
	Store   *database.Store
	Logger  *slog.Logger
	RealmID uint32
}

type account struct {
	ID           uint32
	Login        string
	Locked       bool
	LockCountry  string
	LastIP       string
	FailedLogins uint32
	Security     uint8
	Banned       bool
	PermanentBan bool
	TotpSecret   []byte
	Salt         [crypto.SRP6SaltLength]byte
	Verifier     [crypto.SRP6VerifierLength]byte
}

type buildInfo struct {
	Build  uint32
	Major  uint8
	Minor  uint8
	Bugfix uint8
}

type realm struct {
	ID                   uint32
	Name                 string
	Address              string
	Port                 uint16
	Icon                 uint8
	Flags                uint32
	Timezone             uint8
	AllowedSecurityLevel uint8
	Population           float32
	Build                uint32
}

type session struct {
	server     *Server
	conn       net.Conn
	status     byte
	account    account
	srp        *crypto.SRP6
	sessionKey [crypto.SRP6SessionKeyLength]byte
	build      uint32
	postBC     bool
	login      string
	remoteIP   string
	os         string
	locale     uint8
}

const (
	statusChallenge byte = iota
	statusProof
	statusAuthed
)

func NewServer(store *database.Store, logger *slog.Logger, realmID uint32) *Server {
	return &Server{Store: store, Logger: logger, RealmID: realmID}
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
	remoteIP := remoteAddress(conn)
	state := &session{server: s, conn: conn, status: statusChallenge, remoteIP: remoteIP}
	for {
		cmd := []byte{0}
		if _, err := io.ReadFull(conn, cmd); err != nil {
			return
		}
		var err error
		switch cmd[0] {
		case logonChallenge:
			err = state.handleLogonChallenge(ctx)
		case logonProof:
			err = state.handleLogonProof(ctx)
		case realmList:
			err = state.handleRealmList(ctx)
		case reconnectChallenge, reconnectProof:
			return
		default:
			return
		}
		if err != nil {
			if s.Logger != nil {
				s.Logger.Debug("authentication session closed", "error", err)
			}
			return
		}
	}
}

func (s *session) handleLogonChallenge(ctx context.Context) error {
	if s.status != statusChallenge {
		return errors.New("unexpected logon challenge")
	}
	prefix := make([]byte, 3)
	if _, err := io.ReadFull(s.conn, prefix); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint16(prefix[1:])
	if size < 30 || size > 46 {
		return errors.New("invalid logon challenge size")
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(s.conn, body); err != nil {
		return err
	}
	if int(body[29])+30 != int(size) {
		return errors.New("invalid logon challenge login length")
	}
	s.build = uint32(binary.LittleEndian.Uint16(body[7:9]))
	s.postBC = s.build > preBCMaxBuild
	s.login = string(body[30 : 30+body[29]])
	s.os = reverseCode(string(body[13:17]))
	s.locale = localeID(reverseCode(string(body[17:21])))
	loaded, err := loadAccount(ctx, s.server.Store, s.login, s.remoteIP)
	if err != nil {
		return err
	}
	if loaded == nil {
		return writePacket(s.conn, []byte{logonChallenge, 0, wowUnknownAccount})
	}
	s.account = *loaded
	if s.account.Locked && s.account.LastIP != s.remoteIP {
		return writePacket(s.conn, []byte{logonChallenge, 0, wowLockedEnforced})
	}
	if s.account.Banned {
		result := wowSuspended
		if s.account.PermanentBan {
			result = wowBanned
		}
		return writePacket(s.conn, []byte{logonChallenge, 0, result})
	}
	info, err := loadBuildInfo(ctx, s.server.Store, s.build)
	if err != nil {
		return err
	}
	if info == nil {
		return writePacket(s.conn, []byte{logonChallenge, 0, wowVersionInvalid})
	}
	s.srp, err = crypto.NewSRP6(s.account.Login, s.account.Salt, s.account.Verifier)
	if err != nil {
		return err
	}
	packet := protocol.NewBuffer(96)
	packet.WriteU8(logonChallenge)
	packet.WriteU8(0)
	packet.WriteU8(wowSuccess)
	packet.Write(s.srp.B[:])
	packet.WriteU8(1)
	packet.Write(crypto.SRP6GeneratorBytes())
	packet.WriteU8(crypto.SRP6EphemeralLength)
	modulus := crypto.SRP6ModulusBytes()
	packet.Write(modulus[:])
	packet.Write(s.account.Salt[:])
	packet.Write(versionChallenge[:])
	packet.WriteU8(0)
	s.status = statusProof
	return writePacket(s.conn, packet.Bytes())
}

func (s *session) handleLogonProof(ctx context.Context) error {
	if s.status != statusProof || s.srp == nil {
		return errors.New("unexpected logon proof")
	}
	data := make([]byte, 74)
	if _, err := io.ReadFull(s.conn, data); err != nil {
		return err
	}
	var A [crypto.SRP6EphemeralLength]byte
	var clientM [20]byte
	copy(A[:], data[:32])
	copy(clientM[:], data[32:52])
	key, ok, err := s.srp.VerifyChallengeResponse(A, clientM)
	if err != nil {
		return err
	}
	if !ok {
		_ = writePacket(s.conn, []byte{logonProof, wowUnknownAccount, 0, 0})
		return errors.New("invalid SRP6 proof")
	}
	s.sessionKey = key
	if err := updateAuthenticatedAccount(ctx, s.server.Store, s.account.Login, s.sessionKey[:], s.remoteIP, s.locale, s.os); err != nil {
		return err
	}
	m2 := crypto.SessionVerifier(A, clientM, s.sessionKey)
	packet := protocol.NewBuffer(32)
	packet.WriteU8(logonProof)
	packet.WriteU8(wowSuccess)
	packet.Write(m2[:])
	packet.WriteU32(0x00800000)
	packet.WriteU32(0)
	packet.WriteU16(0)
	if err := writePacket(s.conn, packet.Bytes()); err != nil {
		return err
	}
	s.status = statusAuthed
	return nil
}

func (s *session) handleRealmList(ctx context.Context) error {
	if s.status != statusAuthed {
		return errors.New("unexpected realm list request")
	}
	padding := make([]byte, 4)
	if _, err := io.ReadFull(s.conn, padding); err != nil {
		return err
	}
	realms, err := loadRealms(ctx, s.server.Store)
	if err != nil {
		return err
	}
	counts, err := loadCharacterCounts(ctx, s.server.Store, s.account.ID)
	if err != nil {
		return err
	}
	builds, err := loadBuilds(ctx, s.server.Store)
	if err != nil {
		return err
	}
	payload := protocol.NewBuffer(256)
	count := 0
	for _, r := range realms {
		build, exists := builds[r.Build]
		compatible := (s.postBC && r.Build == s.build) || (!s.postBC && r.Build <= preBCMaxBuild)
		if !compatible && !exists {
			continue
		}
		flags := r.Flags
		if !compatible {
			flags |= realmFlagOffline | realmFlagSpecifyBuild
		}
		if !exists {
			flags &^= realmFlagSpecifyBuild
		}
		name := r.Name
		if !s.postBC && flags&realmFlagSpecifyBuild != 0 {
			name = fmt.Sprintf("%s (%d.%d.%d)", name, build.Major, build.Minor, build.Bugfix)
		}
		lock := uint8(0)
		if r.AllowedSecurityLevel > s.account.Security {
			lock = 1
		}
		address := net.JoinHostPort(r.Address, strconv.Itoa(int(r.Port)))
		payload.WriteU8(r.Icon)
		if s.postBC {
			payload.WriteU8(lock)
		}
		payload.WriteU8(uint8(flags))
		payload.WriteCString(name)
		payload.WriteCString(address)
		payload.WriteF32(r.Population)
		payload.WriteU8(uint8(counts[r.ID]))
		payload.WriteU8(r.Timezone)
		if s.postBC {
			payload.WriteU8(uint8(r.ID))
		} else {
			payload.WriteU8(0)
		}
		if s.postBC && flags&realmFlagSpecifyBuild != 0 {
			payload.WriteU8(build.Major)
			payload.WriteU8(build.Minor)
			payload.WriteU8(build.Bugfix)
			payload.WriteU16(uint16(build.Build))
		}
		count++
	}
	if s.postBC {
		payload.WriteU8(0x10)
		payload.WriteU8(0)
	} else {
		payload.WriteU8(0)
		payload.WriteU8(2)
	}
	header := protocol.NewBuffer(payload.Len() + 8)
	header.WriteU8(realmList)
	header.WriteU16(uint16(payload.Len() + 6))
	header.WriteU32(0)
	if s.postBC {
		header.WriteU16(uint16(count))
	} else {
		header.WriteU32(uint32(count))
	}
	header.Write(payload.Bytes())
	return writePacket(s.conn, header.Bytes())
}

func loadAccount(ctx context.Context, store *database.Store, login, remoteIP string) (*account, error) {
	row, err := store.QueryRowStatement(ctx, "LOGIN_SEL_LOGONCHALLENGE", login)
	if err != nil {
		return nil, err
	}
	var result account
	var locked, failed, banned, permanent, security uint64
	var totp []byte
	var salt, verifier []byte
	if err := row.Scan(&result.ID, &result.Login, &locked, &result.LockCountry, &result.LastIP, &failed, &banned, &permanent, &security, &totp, &salt, &verifier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	result.Locked = locked != 0
	result.FailedLogins = uint32(failed)
	result.Banned = banned != 0
	result.PermanentBan = permanent != 0
	result.Security = uint8(security)
	result.TotpSecret = totp
	if len(salt) != crypto.SRP6SaltLength || len(verifier) != crypto.SRP6VerifierLength {
		return nil, errors.New("account SRP6 data has invalid length")
	}
	copy(result.Salt[:], salt)
	copy(result.Verifier[:], verifier)
	if result.Locked && result.LastIP != remoteIP {
		return &result, nil
	}
	return &result, nil
}

func accountBanned(ctx context.Context, store *database.Store, id uint32) (bool, error) {
	var count int64
	err := store.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM account_banned WHERE id = ? AND active = 1 AND (unbandate > ? OR unbandate = bandate)", id, time.Now().Unix()).Scan(&count)
	return count != 0, err
}

func updateAuthenticatedAccount(ctx context.Context, store *database.Store, login string, sessionKey []byte, ip string, locale uint8, osName string) error {
	_, err := store.ExecStatement(ctx, "LOGIN_UPD_LOGONPROOF", sessionKey, ip, locale, osName, login)
	return err
}

func loadBuildInfo(ctx context.Context, store *database.Store, build uint32) (*buildInfo, error) {
	var result buildInfo
	err := store.DB.QueryRowContext(ctx, "SELECT build, majorVersion, minorVersion, bugfixVersion FROM build_info WHERE build = ? LIMIT 1", build).Scan(&result.Build, &result.Major, &result.Minor, &result.Bugfix)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &result, err
}

func loadBuilds(ctx context.Context, store *database.Store) (map[uint32]buildInfo, error) {
	rows, err := store.DB.QueryContext(ctx, "SELECT build, majorVersion, minorVersion, bugfixVersion FROM build_info")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uint32]buildInfo)
	for rows.Next() {
		var info buildInfo
		if err := rows.Scan(&info.Build, &info.Major, &info.Minor, &info.Bugfix); err != nil {
			return nil, err
		}
		result[info.Build] = info
	}
	return result, rows.Err()
}

func loadRealms(ctx context.Context, store *database.Store) ([]realm, error) {
	rows, err := store.DB.QueryContext(ctx, "SELECT id, name, address, port, icon, flag, timezone, allowedSecurityLevel, population, gamebuild FROM realmlist ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]realm, 0)
	for rows.Next() {
		var r realm
		var population sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Name, &r.Address, &r.Port, &r.Icon, &r.Flags, &r.Timezone, &r.AllowedSecurityLevel, &population, &r.Build); err != nil {
			return nil, err
		}
		if population.Valid {
			r.Population = float32(population.Float64)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func loadCharacterCounts(ctx context.Context, store *database.Store, accountID uint32) (map[uint32]int64, error) {
	rows, err := store.DB.QueryContext(ctx, "SELECT realmid, COUNT(*) FROM realmcharacters WHERE acctid = ? GROUP BY realmid", accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uint32]int64)
	for rows.Next() {
		var realmID uint32
		var count int64
		if err := rows.Scan(&realmID, &count); err != nil {
			return nil, err
		}
		result[realmID] = count
	}
	return result, rows.Err()
}

func writePacket(conn net.Conn, data []byte) error {
	for len(data) != 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func reverseCode(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return strings.TrimRight(string(runes), "\x00")
}

func localeID(value string) uint8 {
	for i, code := range []string{"enUS", "koKR", "frFR", "deDE", "zhCN", "zhTW", "esES", "esMX", "ruRU", "ptBR", "itIT"} {
		if value == code {
			return uint8(i)
		}
	}
	return 0
}

func remoteAddress(conn net.Conn) string {
	address := conn.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return strings.TrimSpace(address)
}
