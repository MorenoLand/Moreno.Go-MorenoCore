package world

import (
	"context"
	"database/sql"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSendNewMailNotificationForUnreadDueMail(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE mail (receiver INTEGER, deliver_time INTEGER, expire_time INTEGER, checked INTEGER)"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if _, err := db.Exec("INSERT INTO mail VALUES (9, ?, ?, 0), (9, ?, ?, 1), (9, ?, ?, 0)", now-1, now+3600, now-1, now+3600, now+3600, now+7200); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	sess := &session{server: &Server{CharactersStore: store}, conn: serverConn, playerGUID: 9}
	done := make(chan struct{})
	go func() {
		sess.sendNewMailNotification(context.Background())
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_RECEIVED_MAIL) || len(payload) != 4 || binary.LittleEndian.Uint32(payload) != 0 {
		t.Fatalf("opcode=%x payload=%x", opcode, payload)
	}
}

func TestSendNewMailNotificationSkipsReadAndFutureMail(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE mail (receiver INTEGER, deliver_time INTEGER, expire_time INTEGER, checked INTEGER)"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if _, err := db.Exec("INSERT INTO mail VALUES (9, ?, ?, 1), (9, ?, ?, 0)", now-1, now+3600, now+3600, now+7200); err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	sess := &session{server: &Server{CharactersStore: store}, conn: serverConn, playerGUID: 9}
	done := make(chan struct{})
	go func() {
		sess.sendNewMailNotification(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("mail notification blocked without a due unread message")
	}
}

func TestLoadAndSendPersistentAura(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE character_aura (
		guid INTEGER, casterGuid INTEGER, itemGuid INTEGER, spell INTEGER, effectMask INTEGER,
		stackCount INTEGER, amount0 INTEGER, maxDuration INTEGER, remainTime INTEGER, remainCharges INTEGER
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO character_aura VALUES (9, 9, 0, 123, 1, 2, 50, 60000, 30000, 0)"); err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	sess := &session{server: &Server{CharactersStore: store}, conn: serverConn, playerGUID: 9}
	state := &playerState{GUID: 9, Level: 20}
	if err := sess.loadPlayerAuras(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	aura, ok := sess.activeAuras[123]
	if !ok || aura.RemainingMs != 30000 || aura.DurationMs != 60000 || aura.Amount != 50 || aura.StackCount != 2 {
		t.Fatalf("aura=%+v present=%v", aura, ok)
	}
	go sess.sendLoadedAuras()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_AURA_UPDATE) || len(payload) == 0 {
		t.Fatalf("opcode=%x payload=%x", opcode, payload)
	}
	sess.clearActiveAuras()
}

func TestLoadGhostAuraRestoresGhostFlag(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE character_aura (
		guid INTEGER, casterGuid INTEGER, itemGuid INTEGER, spell INTEGER, effectMask INTEGER,
		stackCount INTEGER, amount0 INTEGER, maxDuration INTEGER, remainTime INTEGER, remainCharges INTEGER
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO character_aura VALUES (9, 9, 0, 8326, 1, 1, 0, -1, -1, 0)"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	sess := &session{server: &Server{CharactersStore: store}, playerGUID: 9}
	state := &playerState{GUID: 9, Level: 20}
	if err := sess.loadPlayerAuras(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if state.PlayerFlags&playerFlagGhost == 0 {
		t.Fatal("ghost aura did not restore player ghost flag")
	}
}
