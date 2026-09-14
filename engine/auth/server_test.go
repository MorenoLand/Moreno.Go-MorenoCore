package auth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestLogonAndRealmList(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT NOT NULL, locked INTEGER NOT NULL, lock_country TEXT NOT NULL, last_ip TEXT NOT NULL, failed_logins INTEGER NOT NULL, salt BLOB NOT NULL, verifier BLOB NOT NULL, totp_secret BLOB, session_key_auth BLOB, last_login TEXT, online INTEGER, locale INTEGER, os TEXT)",
		"CREATE TABLE account_access (AccountID INTEGER NOT NULL, SecurityLevel INTEGER NOT NULL, RealmID INTEGER NOT NULL)",
		"CREATE TABLE account_banned (id INTEGER NOT NULL, bandate INTEGER NOT NULL, unbandate INTEGER NOT NULL, active INTEGER NOT NULL)",
		"CREATE TABLE build_info (build INTEGER PRIMARY KEY, majorVersion INTEGER, minorVersion INTEGER, bugfixVersion INTEGER)",
		"CREATE TABLE realmlist (id INTEGER PRIMARY KEY, name TEXT, address TEXT, port INTEGER, icon INTEGER, flag INTEGER, timezone INTEGER, allowedSecurityLevel INTEGER, population REAL, gamebuild INTEGER)",
		"CREATE TABLE realmcharacters (realmid INTEGER, acctid INTEGER)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	var salt [crypto.SRP6SaltLength]byte
	_, verifier, err := crypto.MakeRegistrationDataWithReader("TEST", "PASSWORD", bytes.NewReader(salt[:]))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO account (id, username, locked, lock_country, last_ip, failed_logins, salt, verifier) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", 7, "TEST", 0, "00", "127.0.0.1", 0, salt[:], verifier[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO account_access (AccountID, SecurityLevel, RealmID) VALUES (7, 0, -1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO build_info (build, majorVersion, minorVersion, bugfixVersion) VALUES (12340, 3, 3, 5)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO realmlist (id, name, address, port, icon, flag, timezone, allowedSecurityLevel, population, gamebuild) VALUES (1, 'Test Realm', '203.0.113.10', 8085, 0, 0, 0, 0, 0, 12340)"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: db}
	server := NewServer(store, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})), 1)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.Handle(context.Background(), serverConn)
	challenge := buildChallenge(" TEST ")
	if _, err := clientConn.Write(challenge); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 119)
	if _, err := io.ReadFull(clientConn, response); err != nil {
		t.Fatal(err)
	}
	if response[0] != logonChallenge || response[2] != wowSuccess {
		t.Fatalf("challenge response: %x", response[:3])
	}
	var B [crypto.SRP6EphemeralLength]byte
	var returnedSalt [crypto.SRP6SaltLength]byte
	copy(B[:], response[3:35])
	copy(returnedSalt[:], response[70:102])
	A, M, _, err := crypto.MakeClientProof("TEST", "PASSWORD", returnedSalt, B, bytes.Repeat([]byte{0x27}, crypto.SRP6EphemeralLength))
	if err != nil {
		t.Fatal(err)
	}
	proof := make([]byte, 75)
	proof[0] = logonProof
	copy(proof[1:33], A[:])
	copy(proof[33:53], M[:])
	if _, err := clientConn.Write(proof); err != nil {
		t.Fatal(err)
	}
	proofResponse := make([]byte, 32)
	if _, err := io.ReadFull(clientConn, proofResponse); err != nil {
		t.Fatal(err)
	}
	if proofResponse[0] != logonProof || proofResponse[1] != wowSuccess {
		t.Fatalf("proof response: %x", proofResponse)
	}
	if _, err := clientConn.Write([]byte{realmList, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	realmHeader := make([]byte, 3)
	if _, err := io.ReadFull(clientConn, realmHeader); err != nil {
		t.Fatal(err)
	}
	realmSize := binary.LittleEndian.Uint16(realmHeader[1:])
	realmBody := make([]byte, realmSize)
	if _, err := io.ReadFull(clientConn, realmBody); err != nil {
		t.Fatal(err)
	}
	if realmHeader[0] != realmList || len(realmBody) < 8 {
		t.Fatalf("realm response: %x %x", realmHeader, realmBody)
	}
	if !bytes.Contains(realmBody, []byte("127.0.0.1:8085")) || bytes.Contains(realmBody, []byte("203.0.113.10:8085")) {
		t.Fatalf("realm address: %x", realmBody)
	}
}

func TestFailedLoginProtectionCountsAndBansAccount(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT, failed_logins INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE account_banned (id INTEGER, bandate INTEGER, unbandate INTEGER, bannedby TEXT, banreason TEXT, active INTEGER)",
		"CREATE TABLE ip_banned (ip TEXT, bandate INTEGER, unbandate INTEGER, bannedby TEXT, banreason TEXT)",
		"INSERT INTO account (id, username, failed_logins) VALUES (7, 'TEST', 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: db}
	settings := config.Default()
	settings.WrongPassMaxCount = 2
	settings.WrongPassBanTime = 120
	settings.WrongPassBanType = true
	server := NewServer(store, slog.New(slog.NewTextHandler(io.Discard, nil)), 1, settings)
	sess := &session{server: server, account: account{ID: 7, Login: "TEST"}, remoteIP: "127.0.0.1"}
	sess.recordFailedLogin(context.Background())
	sess.recordFailedLogin(context.Background())
	var failed int
	if err := db.QueryRow("SELECT failed_logins FROM account WHERE id = 7").Scan(&failed); err != nil || failed != 2 {
		t.Fatalf("failed logins=%d err=%v", failed, err)
	}
	var bans int
	if err := db.QueryRow("SELECT COUNT(*) FROM account_banned WHERE id = 7 AND active = 1").Scan(&bans); err != nil || bans != 1 {
		t.Fatalf("account bans=%d err=%v", bans, err)
	}
}

func TestBannedIPIsRejectedBeforeAuthenticationChallenge(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE ip_banned (ip TEXT, bandate INTEGER, unbandate INTEGER, bannedby TEXT, banreason TEXT); INSERT INTO ip_banned VALUES ('pipe', 100, 100, 'test', 'test')"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: db}
	server := NewServer(store, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	go server.Handle(context.Background(), serverConn)
	packet := make([]byte, 3)
	if _, err := io.ReadFull(clientConn, packet); err != nil {
		t.Fatal(err)
	}
	if packet[0] != logonChallenge || packet[2] != wowBanned {
		t.Fatalf("banned IP response=%x", packet)
	}
}

func TestReconnectChallengeAndProof(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key := bytes.Repeat([]byte{0x42}, crypto.SRP6SessionKeyLength)
	for _, statement := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT NOT NULL, locked INTEGER NOT NULL, lock_country TEXT NOT NULL, last_ip TEXT NOT NULL, failed_logins INTEGER NOT NULL, session_key_auth BLOB, expansion INTEGER NOT NULL DEFAULT 2, mutetime INTEGER NOT NULL DEFAULT 0, locale INTEGER NOT NULL DEFAULT 0, recruiter INTEGER NOT NULL DEFAULT 0, os TEXT NOT NULL DEFAULT '', last_login TEXT)",
		"CREATE TABLE account_access (AccountID INTEGER, SecurityLevel INTEGER, RealmID INTEGER)",
		"CREATE TABLE account_banned (id INTEGER, bandate INTEGER, unbandate INTEGER, active INTEGER)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO account (id, username, locked, lock_country, last_ip, failed_logins, session_key_auth) VALUES (7, 'TEST', 0, '00', '127.0.0.1', 0, ?)", key); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: db}
	server := NewServer(store, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	sess := &session{server: server, conn: serverConn, status: statusChallenge, remoteIP: "127.0.0.1"}
	challenge := buildChallenge("TEST")
	challenge[0] = reconnectChallenge
	done := make(chan error, 1)
	go func() { done <- sess.handleReconnectChallenge(context.Background()) }()
	if _, err := clientConn.Write(challenge[1:]); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 34)
	if _, err := io.ReadFull(clientConn, response); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if response[0] != reconnectChallenge || response[1] != wowSuccess {
		t.Fatalf("reconnect challenge response=%x", response[:2])
	}
	proofSeed := bytes.Repeat([]byte{0x11}, 16)
	h := sha1.New()
	_, _ = h.Write([]byte("TEST"))
	_, _ = h.Write(proofSeed)
	_, _ = h.Write(response[2:18])
	_, _ = h.Write(key)
	proof := make([]byte, 57)
	copy(proof[:16], proofSeed)
	copy(proof[16:36], h.Sum(nil))
	done = make(chan error, 1)
	go func() { done <- sess.handleReconnectProof(context.Background()) }()
	if _, err := clientConn.Write(proof); err != nil {
		t.Fatal(err)
	}
	proofResponse := make([]byte, 4)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(clientConn, proofResponse); err != nil {
		if proofErr := <-done; proofErr != nil {
			t.Fatal(proofErr)
		}
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(proofResponse, []byte{reconnectProof, wowSuccess, 0, 0}) || sess.status != statusAuthed {
		t.Fatalf("reconnect proof response=%x status=%d", proofResponse, sess.status)
	}
}

func TestReconnectProofRejectsInvalidChecksum(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	sess := &session{server: &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, conn: serverConn, status: statusReconnectProof, account: account{Login: "TEST"}, sessionKey: [crypto.SRP6SessionKeyLength]byte{0x42}, reconnectProof: [16]byte{0x11}}
	proof := make([]byte, 57)
	done := make(chan error, 1)
	go func() { done <- sess.handleReconnectProof(context.Background()) }()
	if _, err := clientConn.Write(proof); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || err.Error() != "invalid reconnect proof" {
		t.Fatalf("invalid reconnect proof error=%v", err)
	}
}

func buildChallenge(login string) []byte {
	var b bytes.Buffer
	b.WriteByte(logonChallenge)
	b.WriteByte(0)
	b.Write([]byte{0, 0})
	b.Write([]byte{'W', 'o', 'W', 0})
	b.Write([]byte{3, 3, 5})
	var build [2]byte
	binary.LittleEndian.PutUint16(build[:], 12340)
	b.Write(build[:])
	b.Write([]byte{'x', '8', '6', 0})
	b.Write([]byte{'W', 'i', 'n', 0})
	b.Write([]byte{'e', 'n', 'U', 'S'})
	b.Write(make([]byte, 8))
	b.WriteByte(byte(len(login)))
	b.WriteString(login)
	binary.LittleEndian.PutUint16(b.Bytes()[2:4], uint16(30+len(login)))
	return b.Bytes()
}
