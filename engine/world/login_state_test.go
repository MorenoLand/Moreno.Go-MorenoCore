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
