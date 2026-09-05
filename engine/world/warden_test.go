package world

import (
	"bytes"
	"context"
	"crypto/rc4"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSessionKeyGeneratorParity(t *testing.T) {
	dummyKey := bytes.Repeat([]byte{0x42}, crypto.SRP6SessionKeyLength)
	gen := newSessionKeyGenerator(dummyKey)

	var inKey [16]byte
	var outKey [16]byte
	gen.Generate(inKey[:])
	gen.Generate(outKey[:])

	expectedIn := "834d977c3f301ed70df9ec13e1375e61"
	expectedOut := "84cfc7b3ff362c83ea2631a8818a7c1c"

	if hex.EncodeToString(inKey[:]) != expectedIn {
		t.Fatalf("SessionKeyGenerator inputKey mismatch: got %x, want %s", inKey, expectedIn)
	}
	if hex.EncodeToString(outKey[:]) != expectedOut {
		t.Fatalf("SessionKeyGenerator outputKey mismatch: got %x, want %s", outKey, expectedOut)
	}
}

func TestWardenChecksum(t *testing.T) {
	data := []byte("Hello, Warden!")
	cs := buildWardenChecksum(data)
	expected := uint32(0x93450444)
	if cs != expected {
		t.Fatalf("buildWardenChecksum mismatch: got 0x%08X, want 0x%08X", cs, expected)
	}
	if !isValidWardenChecksum(expected, data) {
		t.Fatal("isValidWardenChecksum returned false for valid checksum")
	}
	if isValidWardenChecksum(expected^1, data) {
		t.Fatal("isValidWardenChecksum returned true for corrupted checksum")
	}
}

func TestWardenCheckMgrLoadingAndOverrides(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE warden_checks (id INTEGER PRIMARY KEY, type INTEGER, data BLOB, result BLOB, address INTEGER, length INTEGER, str TEXT, comment TEXT)`,
		`CREATE TABLE warden_action (wardenId INTEGER PRIMARY KEY, action INTEGER)`,
		`INSERT INTO warden_checks (id, type, data, result, address, length, str, comment) VALUES (1, 243, NULL, X'010203', 0x00401000, 3, '', 'MemCheck1')`,
		`INSERT INTO warden_checks (id, type, data, result, address, length, str, comment) VALUES (2, 178, X'AABBCC', NULL, 0x00402000, 10, '', 'PageCheckA1')`,
		`INSERT INTO warden_checks (id, type, data, result, address, length, str, comment) VALUES (3, 139, NULL, NULL, 0, 0, 'return IsHackLoaded()', 'LuaCheck1')`,
		`INSERT INTO warden_action (wardenId, action) VALUES (3, 2)`, // Ban
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	mgr := newWardenCheckMgr()
	if err := mgr.loadChecks(context.Background(), db, uint32(WardenActionLog)); err != nil {
		t.Fatalf("loadChecks failed: %v", err)
	}
	if err := mgr.loadOverrides(context.Background(), db); err != nil {
		t.Fatalf("loadOverrides failed: %v", err)
	}

	chk1, ok := mgr.getCheckData(1)
	if !ok || chk1.Type != MemCheck || chk1.Action != WardenActionLog {
		t.Fatalf("chk1 mismatch: ok=%v, type=%v, action=%v", ok, chk1.Type, chk1.Action)
	}
	res1, ok := mgr.getCheckResult(1)
	if !ok || !bytes.Equal(res1, []byte{1, 2, 3}) {
		t.Fatalf("res1 mismatch: ok=%v, got=%v", ok, res1)
	}

	chk3, ok := mgr.getCheckData(3)
	if !ok || chk3.Type != LuaEvalCheck || chk3.Action != WardenActionBan {
		t.Fatalf("chk3 mismatch: ok=%v, type=%v, action=%v", ok, chk3.Type, chk3.Action)
	}
	if string(chk3.IdStr[:]) != "0003" {
		t.Fatalf("chk3 IdStr mismatch: got %q, want '0003'", string(chk3.IdStr[:]))
	}

	availInject := mgr.getAvailableChecks(InjectCheckCategory)
	if len(availInject) != 1 || availInject[0] != 2 {
		t.Fatalf("availInject mismatch: %v", availInject)
	}
	availLua := mgr.getAvailableChecks(LuaCheckCategory)
	if len(availLua) != 1 || availLua[0] != 3 {
		t.Fatalf("availLua mismatch: %v", availLua)
	}
	availMod := mgr.getAvailableChecks(ModdedCheckCategory)
	if len(availMod) != 1 || availMod[0] != 1 {
		t.Fatalf("availMod mismatch: %v", availMod)
	}
}

func setupWardenServer(t *testing.T) (*Server, *session, net.Conn, *rc4.Cipher, *rc4.Cipher) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT, session_key_auth BLOB, last_ip TEXT, locked INTEGER, lock_country TEXT, os TEXT, online INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE account_banned (id INTEGER, bandate INTEGER, unbandate INTEGER, bannedby TEXT, banreason TEXT, active INTEGER, PRIMARY KEY (id, bandate))`,
		`CREATE TABLE characters (guid INTEGER PRIMARY KEY, account INTEGER, name TEXT, race INTEGER, class INTEGER, gender INTEGER, skin INTEGER, face INTEGER, hairStyle INTEGER, hairColor INTEGER, facialStyle INTEGER, level INTEGER, zone INTEGER, map INTEGER, position_x REAL, position_y REAL, position_z REAL, orientation REAL, playerFlags INTEGER, at_login INTEGER, equipmentCache TEXT)`,
		`CREATE TABLE warden_checks (id INTEGER PRIMARY KEY, type INTEGER, data BLOB, result BLOB, address INTEGER, length INTEGER, str TEXT, comment TEXT)`,
		`CREATE TABLE warden_action (wardenId INTEGER PRIMARY KEY, action INTEGER)`,
		`INSERT INTO warden_checks (id, type, data, result, address, length, str, comment) VALUES (1, 243, NULL, X'112233', 0x00401000, 3, '', 'MemCheck1')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Default()
	cfg.WardenEnabled = true
	cfg.WardenClientCheckHoldOff = 1
	cfg.WardenClientCheckFailAction = 1 // Kick
	cfg.WardenNumClientModChecks = 1
	cfg.WardenNumInjectionChecks = 0
	cfg.WardenNumLuaSandboxChecks = 0

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	stores := &database.Set{Auth: store, Characters: store, World: store}
	srv := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1, cfg)
	srv.loadWardenChecks(context.Background())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var serverConn net.Conn
	var clientConn net.Conn
	var connErr error
	ch := make(chan struct{})
	go func() {
		serverConn, connErr = ln.Accept()
		close(ch)
	}()
	clientConn, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	<-ch
	if connErr != nil {
		t.Fatal(connErr)
	}

	sessionKey := bytes.Repeat([]byte{0x42}, crypto.SRP6SessionKeyLength)
	sess := &session{
		server:      srv,
		conn:        serverConn,
		accountID:   1,
		accountName: "TEST_USER",
		authed:      false,
	}
	srv.sessionsMu.Lock()
	srv.sessions[sess] = struct{}{}
	srv.sessionsMu.Unlock()

	// Compute client-side ciphers
	gen := newSessionKeyGenerator(sessionKey)
	var clientInKey [16]byte  // receives server's outputKey
	var clientOutKey [16]byte // sends with server's inputKey
	gen.Generate(clientOutKey[:])
	gen.Generate(clientInKey[:])

	clientDecrypt, err := rc4.NewCipher(clientInKey[:])
	if err != nil {
		t.Fatal(err)
	}
	clientEncrypt, err := rc4.NewCipher(clientOutKey[:])
	if err != nil {
		t.Fatal(err)
	}

	w, err := newWardenSession(sess, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	sess.warden = w

	return srv, sess, clientConn, clientDecrypt, clientEncrypt
}

func readWardenPacket(t *testing.T, clientConn net.Conn, clientDecrypt *rc4.Cipher) (uint8, []byte) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, header); err != nil {
		t.Fatalf("failed to read packet header: %v", err)
	}
	size := int(binary.BigEndian.Uint16(header[:2])) - 2
	payload := make([]byte, size)
	if _, err := io.ReadFull(clientConn, payload); err != nil {
		t.Fatalf("failed to read packet payload: %v", err)
	}
	clientDecrypt.XORKeyStream(payload, payload)
	return payload[0], payload[1:]
}

func sendWardenPacket(t *testing.T, clientConn net.Conn, clientEncrypt *rc4.Cipher, sess *session, payload []byte) {
	encrypted := make([]byte, len(payload))
	copy(encrypted, payload)
	clientEncrypt.XORKeyStream(encrypted, encrypted)
	// Dispatch into sess.handleWardenData in background so synchronous net.Pipe writes do not deadlock the read loop
	go func() {
		_ = sess.handleWardenData(context.Background(), encrypted)
	}()
}

func TestWardenFullHandshakeAndChecks(t *testing.T) {
	_, sess, clientConn, clientDecrypt, clientEncrypt := setupWardenServer(t)
	defer clientConn.Close()

	// 1. Initial packet sent by newWardenSession is WARDEN_SMSG_MODULE_USE (0x00)
	op, body := readWardenPacket(t, clientConn, clientDecrypt)
	if op != wardenSmsgModuleUse {
		t.Fatalf("expected WARDEN_SMSG_MODULE_USE (0), got %d", op)
	}
	if len(body) != 16+16+4 {
		t.Fatalf("unexpected body len for MODULE_USE: %d", len(body))
	}

	// 2. Client sends WARDEN_CMSG_MODULE_MISSING (0x00)
	sendWardenPacket(t, clientConn, clientEncrypt, sess, []byte{wardenCmsgModuleMissing})

	// 3. Server streams chunks with WARDEN_SMSG_MODULE_CACHE (0x01)
	totalBytes := 0
	for {
		op, chunk := readWardenPacket(t, clientConn, clientDecrypt)
		if op != wardenSmsgModuleCache {
			t.Fatalf("expected WARDEN_SMSG_MODULE_CACHE, got %d", op)
		}
		dataSize := int(binary.LittleEndian.Uint16(chunk[:2]))
		burst := chunk[2:]
		if len(burst) != dataSize {
			t.Fatalf("burst len %d != dataSize %d", len(burst), dataSize)
		}
		totalBytes += dataSize
		if totalBytes >= 18756 {
			break
		}
	}
	if totalBytes != 18756 {
		t.Fatalf("expected 18756 total module bytes, got %d", totalBytes)
	}

	// 4. Client sends WARDEN_CMSG_MODULE_OK (0x01)
	sendWardenPacket(t, clientConn, clientEncrypt, sess, []byte{wardenCmsgModuleOk})

	// 5. Server sends WARDEN_SMSG_HASH_REQUEST (0x05)
	op, seedBody := readWardenPacket(t, clientConn, clientDecrypt)
	if op != wardenSmsgHashRequest {
		t.Fatalf("expected WARDEN_SMSG_HASH_REQUEST, got %d", op)
	}
	if !bytes.Equal(seedBody, wardenWinSeed[:]) {
		t.Fatalf("seed mismatch: got %x, want %x", seedBody, wardenWinSeed)
	}

	// 6. Client sends WARDEN_CMSG_HASH_RESULT (0x04) with wardenWinClientKeySeedHash
	hashResultPkt := append([]byte{wardenCmsgHashResult}, wardenWinClientKeySeedHash[:]...)
	sendWardenPacket(t, clientConn, clientEncrypt, sess, hashResultPkt)

	// Client re-keys after sending hash result
	var err error
	clientDecrypt, err = rc4.NewCipher(wardenWinServerKeySeed[:])
	if err != nil {
		t.Fatal(err)
	}
	clientEncrypt, err = rc4.NewCipher(wardenWinClientKeySeed[:])
	if err != nil {
		t.Fatal(err)
	}

	// 7. Server sends WARDEN_SMSG_MODULE_INITIALIZE (0x03)
	op, initBody := readWardenPacket(t, clientConn, clientDecrypt)
	if op != wardenSmsgModuleInitialize {
		t.Fatalf("expected WARDEN_SMSG_MODULE_INITIALIZE, got %d", op)
	}
	if len(initBody) != 56 { // 57 - 1 (opcode already stripped)
		t.Fatalf("unexpected init body size: %d", len(initBody))
	}

	// 8. Trigger check request
	sess.warden.forceChecks([]uint16{1})
	if err := sess.warden.requestChecks(); err != nil {
		t.Fatalf("requestChecks failed: %v", err)
	}

	op, chkReq := readWardenPacket(t, clientConn, clientDecrypt)
	if op != wardenSmsgCheatChecksRequest {
		t.Fatalf("expected WARDEN_SMSG_CHEAT_CHECKS_REQUEST, got %d", op)
	}
	if len(chkReq) == 0 {
		t.Fatal("empty checks request body")
	}

	// 9. Client constructs valid WARDEN_CMSG_CHEAT_CHECKS_RESULT
	// Timing check result (1) + client ticks (uint32) + MemCheck result (0) + expected bytes (3 bytes: 0x11, 0x22, 0x33)
	buf := protocol.NewBuffer(32)
	buf.WriteU8(1)                   // timing result
	buf.WriteU32(12345)               // client ticks
	buf.WriteU8(0)                   // mem check result (0 = ok)
	buf.Write([]byte{0x11, 0x22, 0x33}) // mem check bytes

	unencryptedData := buf.Bytes()
	cs := buildWardenChecksum(unencryptedData)

	resPkt := protocol.NewBuffer(1 + 2 + 4 + len(unencryptedData))
	resPkt.WriteU8(wardenCmsgCheatChecksResult)
	resPkt.WriteU16(uint16(len(unencryptedData)))
	resPkt.WriteU32(cs)
	resPkt.Write(unencryptedData)

	sendWardenPacket(t, clientConn, clientEncrypt, sess, resPkt.Bytes())

	deadline := time.Now().Add(2 * time.Second)
	for sess.warden.dataSent && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if sess.warden.dataSent {
		t.Fatal("dataSent should be false after valid check result received")
	}
}

func TestWardenLuaCheckResponseIntercept(t *testing.T) {
	_, sess, clientConn, _, _ := setupWardenServer(t)
	defer clientConn.Close()

	sess.playerLoaded = true
	sess.player = &playerState{Health: 100}

	// Message with Warden token "_TW\t0001" (check ID 1 is not Lua check, so bogus response)
	pkt := protocol.NewBuffer(32)
	pkt.WriteU32(0) // chatSay
	pkt.WriteU32(0) // langUniversal
	pkt.WriteCString("_TW\t0001")

	// handleMessageChat should return true (handled/intercepted)
	if !sess.handleMessageChat(context.Background(), pkt.Bytes()) {
		t.Fatal("handleMessageChat should return true for intercepted Warden Lua token")
	}
}

func TestWardenClientResponseTimeout(t *testing.T) {
	_, sess, clientConn, clientDecrypt, _ := setupWardenServer(t)
	defer clientConn.Close()

	// Drain initial WARDEN_SMSG_MODULE_USE packet sent on session init
	readWardenPacket(t, clientConn, clientDecrypt)

	sess.warden.initialized = true
	sess.warden.dataSent = true
	sess.server.Config.WardenClientResponseDelay = 5   // 5 seconds
	sess.server.Config.WardenClientCheckFailAction = 1 // Kick

	// Update with 6 seconds diff -> clientResponseTimer becomes 6s
	sess.warden.update(6 * time.Second)
	// Second tick -> clientResponseTimer (6s) > maxDelay (5s) -> kicks
	sess.warden.update(100 * time.Millisecond)

	// Conn should be closed
	oneByte := make([]byte, 1)
	_ = clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err := clientConn.Read(oneByte)
	if err == nil {
		t.Fatal("expected connection to be closed after timeout kick")
	}
}
