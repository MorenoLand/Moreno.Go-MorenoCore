package world

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore5/pkg/protocol"
)

func TestAuthSessionAndPing(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT NOT NULL, session_key_auth BLOB, last_ip TEXT, locked INTEGER, lock_country TEXT, os TEXT)",
		"CREATE TABLE account_banned (id INTEGER NOT NULL, bandate INTEGER NOT NULL, unbandate INTEGER NOT NULL, active INTEGER NOT NULL)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	key := bytes.Repeat([]byte{0x42}, crypto.SRP6SessionKeyLength)
	if _, err := db.Exec("INSERT INTO account (id, username, session_key_auth, last_ip, locked, lock_country, os) VALUES (7, 'TEST', ?, '127.0.0.1', 0, '00', 'Win')", key); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := NewServer(store, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.Handle(context.Background(), serverConn)
	challengeHeader := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, challengeHeader); err != nil {
		t.Fatal(err)
	}
	challengeSize := int(binary.BigEndian.Uint16(challengeHeader[:2])) - 2
	challengePayload := make([]byte, challengeSize)
	if _, err := io.ReadFull(clientConn, challengePayload); err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(challengeHeader[2:]) != opcodeAuthChallenge || len(challengePayload) != 40 {
		t.Fatalf("challenge header=%x payload=%d", challengeHeader, len(challengePayload))
	}
	authSeed := challengePayload[4:8]
	localChallenge := []byte{1, 2, 3, 4}
	h := sha1.New()
	_, _ = h.Write([]byte("TEST"))
	_, _ = h.Write(make([]byte, 4))
	_, _ = h.Write(localChallenge)
	_, _ = h.Write(authSeed)
	_, _ = h.Write(key)
	payload := protocol.NewBuffer(96)
	payload.WriteU32(12340)
	payload.WriteU32(0)
	payload.WriteCString("TEST")
	payload.WriteU32(0)
	payload.Write(localChallenge)
	payload.WriteU32(0)
	payload.WriteU32(0)
	payload.WriteU32(1)
	payload.WriteU64(0)
	payload.Write(h.Sum(nil))
	if err := writeClientFrame(clientConn, opcodeAuthSession, payload.Bytes(), nil); err != nil {
		t.Fatal(err)
	}
	clientCrypt, err := crypto.NewClientAuthCrypt(key)
	if err != nil {
		t.Fatal(err)
	}
	responseOpcode, responsePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if responseOpcode != opcodeAuthResponse || !bytes.Equal(responsePayload, []byte{authOK}) {
		t.Fatalf("auth response opcode=%x payload=%x", responseOpcode, responsePayload)
	}
	ping := protocol.NewBuffer(8)
	ping.WriteU32(123)
	ping.WriteU32(45)
	if err := writeClientFrame(clientConn, opcodePing, ping.Bytes(), clientCrypt); err != nil {
		t.Fatal(err)
	}
	pongOpcode, pongPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if pongOpcode != opcodePong || !bytes.Equal(pongPayload, []byte{123, 0, 0, 0}) {
		t.Fatalf("pong opcode=%x payload=%x", pongOpcode, pongPayload)
	}
}

func writeClientFrame(w io.Writer, opcode uint32, payload []byte, crypt interface{ EncryptSend([]byte) error }) error {
	size := len(payload) + 4
	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[:2], uint16(size))
	binary.LittleEndian.PutUint32(header[2:], opcode)
	if crypt != nil {
		if err := crypt.EncryptSend(header); err != nil {
			return err
		}
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readServerFrame(r io.Reader, crypt interface{ DecryptRecv([]byte) error }) (uint16, []byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	if err := crypt.DecryptRecv(header); err != nil {
		return 0, nil, err
	}
	size := int(binary.BigEndian.Uint16(header[:2]))
	payload := make([]byte, size-2)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return binary.LittleEndian.Uint16(header[2:]), payload, nil
}
