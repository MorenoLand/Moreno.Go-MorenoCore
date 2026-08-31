package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/database"
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
	if _, err := db.Exec("INSERT INTO realmlist (id, name, address, port, icon, flag, timezone, allowedSecurityLevel, population, gamebuild) VALUES (1, 'Test Realm', '127.0.0.1', 8085, 0, 0, 0, 0, 0, 12340)"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: db}
	server := NewServer(store, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})), 1)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.Handle(context.Background(), serverConn)
	challenge := buildChallenge("TEST")
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
