package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestMailSendingAndReceiving(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, played_time INTEGER, text TEXT)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER, PRIMARY KEY (mail_id, item_guid))",
		"INSERT INTO characters VALUES (1, 'Alice', 10000, '')",
		"INSERT INTO characters VALUES (2, 'Bob', 500, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sessSender := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Alice", Money: 10000}}
	sessReceiver := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Bob", Money: 500}}
	srv.sessions[sessSender] = struct{}{}
	srv.sessions[sessReceiver] = struct{}{}

	ctx := context.Background()

	// 1. Send mail from Alice to Bob with 200 copper
	sendBuf := protocol.NewBuffer(128)
	sendBuf.WriteU64(0)           // mailbox
	sendBuf.WriteCString("Bob")   // target
	sendBuf.WriteCString("Hello") // subject
	sendBuf.WriteCString("World") // body
	sendBuf.WriteU32(41)          // stationery
	sendBuf.WriteU32(0)           // package
	sendBuf.WriteU8(0)            // attachments count
	sendBuf.WriteU32(200)         // money
	sendBuf.WriteU32(0)           // COD
	if !sessSender.handleSendMail(ctx, sendBuf.Bytes()) {
		t.Fatal("handleSendMail failed")
	}
	if sessSender.player.Money != 10000-230 { // 200 money + 30 postage
		t.Fatalf("expected sender money %d, got %d", 10000-230, sessSender.player.Money)
	}

	// 2. Bob reads mail list
	getBuf := protocol.NewBuffer(8)
	getBuf.WriteU64(0)
	if !sessReceiver.handleGetMailList(ctx, getBuf.Bytes()) {
		t.Fatal("handleGetMailList failed")
	}

	// 3. Bob takes money from mail 1
	takeMoneyBuf := protocol.NewBuffer(12)
	takeMoneyBuf.WriteU64(0)
	takeMoneyBuf.WriteU32(1) // mail ID
	if !sessReceiver.handleMailTakeMoney(ctx, takeMoneyBuf.Bytes()) {
		t.Fatal("handleMailTakeMoney failed")
	}
	if sessReceiver.player.Money != 500+200 {
		t.Fatalf("expected receiver money 700, got %d", sessReceiver.player.Money)
	}

	// 4. Bob deletes mail 1
	delBuf := protocol.NewBuffer(12)
	delBuf.WriteU64(0)
	delBuf.WriteU32(1)
	if !sessReceiver.handleMailDelete(ctx, delBuf.Bytes()) {
		t.Fatal("handleMailDelete failed")
	}
	var remainingMails int64
	_ = db.QueryRow("SELECT COUNT(*) FROM mail WHERE receiver = 2").Scan(&remainingMails)
	if remainingMails != 0 {
		t.Fatalf("expected 0 remaining mails, got %d", remainingMails)
	}
}

