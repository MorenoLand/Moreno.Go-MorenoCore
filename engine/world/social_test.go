package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestSocialContactListAndFriendBroadcast(t *testing.T) {
	charDB, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer charDB.Close()

	for _, stmt := range []string{
		"CREATE TABLE character_social (guid INTEGER, friend INTEGER, flags INTEGER, note TEXT, PRIMARY KEY (guid, friend, flags))",
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT)",
	} {
		if _, err := charDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{
		CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: charDB},
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config:          config.Default(),
		sessions:        make(map[*session]struct{}),
	}

	// Player 1 has Player 2 as friend (flags = 1)
	if _, err := charDB.Exec("INSERT INTO character_social (guid, friend, flags, note) VALUES (1, 2, 1, 'Best Buddy')"); err != nil {
		t.Fatal(err)
	}
	if _, err := charDB.Exec("INSERT INTO characters (guid, name) VALUES (1, 'Alice'), (2, 'Bob')"); err != nil {
		t.Fatal(err)
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess1 := &session{
		server:       server,
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Name: "Alice", Level: 80, Race: 1, Class: 1},
	}
	server.sessions[sess1] = struct{}{}

	// 1. Player 1 requests contact list (flags 0x7)
	reqBuf := protocol.NewBuffer(4)
	reqBuf.WriteU32(0x7)
	done := make(chan struct{})
	go func() {
		if !sess1.handleContactList(context.Background(), reqBuf.Bytes()) {
			t.Error("handleContactList returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if op != uint16(protocol.OpcodeSMSG_CONTACT_LIST) {
		t.Fatalf("expected SMSG_CONTACT_LIST, got %x", op)
	}
	r := protocol.NewReader(payload)
	listFlags, _ := r.ReadU32()
	count, _ := r.ReadU32()
	if listFlags != 7 || count != 1 {
		t.Fatalf("unexpected listFlags=%d count=%d", listFlags, count)
	}
	fGUID, _ := r.ReadU64()
	fFlags, _ := r.ReadU32()
	fNote, _ := r.ReadCString()
	fStatus, _ := r.ReadU8()
	if fGUID != 2 || fFlags != 1 || fNote != "Best Buddy" || fStatus != friendStatusOffline {
		t.Fatalf("unexpected contact entry: guid=%d flags=%d note=%q status=%d", fGUID, fFlags, fNote, fStatus)
	}

	// 2. Player 2 logs in -> server broadcasts friendStatusOnline
	done2 := make(chan struct{})
	go func() {
		server.broadcastFriendStatus(2, friendsResultOnline, 14, 80, 2)
		close(done2)
	}()

	op2, payload2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done2
	if op2 != uint16(protocol.OpcodeSMSG_FRIEND_STATUS) {
		t.Fatalf("expected SMSG_FRIEND_STATUS, got %x", op2)
	}
	r2 := protocol.NewReader(payload2)
	res, _ := r2.ReadU8()
	p2GUID, _ := r2.ReadU64()
	stat, _ := r2.ReadU8()
	zone, _ := r2.ReadU32()
	lvl, _ := r2.ReadU32()
	cls, _ := r2.ReadU32()
	if res != friendsResultOnline || p2GUID != 2 || stat != friendStatusOnline || zone != 14 || lvl != 80 || cls != 2 {
		t.Fatalf("unexpected friend online broadcast: res=%d guid=%d stat=%d zone=%d lvl=%d cls=%d", res, p2GUID, stat, zone, lvl, cls)
	}

	// 3. Player 2 logs out -> server broadcasts friendStatusOffline
	done3 := make(chan struct{})
	go func() {
		server.broadcastFriendStatus(2, friendsResultOffline, 0, 0, 0)
		close(done3)
	}()

	op3, payload3, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done3
	if op3 != uint16(protocol.OpcodeSMSG_FRIEND_STATUS) {
		t.Fatalf("expected SMSG_FRIEND_STATUS, got %x", op3)
	}
	r3 := protocol.NewReader(payload3)
	res3, _ := r3.ReadU8()
	p3GUID, _ := r3.ReadU64()
	if res3 != friendsResultOffline || p3GUID != 2 {
		t.Fatalf("unexpected friend offline broadcast: res=%d guid=%d", res3, p3GUID)
	}
}
