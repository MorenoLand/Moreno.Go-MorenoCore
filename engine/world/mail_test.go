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

func TestMailTakeItemWithCODEnforcement(t *testing.T) {
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
		"INSERT INTO characters VALUES (2, 'Bob', 50, '')", // starts with 50 copper (insufficient for 150 COD)
		"INSERT INTO item_instance VALUES (501, 9999, 2, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		// Mail 1 from Alice (1) to Bob (2) with COD = 150 and item 501
		"INSERT INTO mail VALUES (1, 0, 41, 0, 1, 2, 'Sword for Sale', '', 1, 9999999999, 0, 0, 150, 4)",
		"INSERT INTO mail_items VALUES (1, 501, 9999, 2)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sessBob := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Bob", Money: 50}}
	srv.sessions[sessBob] = struct{}{}

	ctx := context.Background()

	// 1. Bob tries to take item with only 50 money (COD is 150) -> should fail
	takeBuf := protocol.NewBuffer(16)
	takeBuf.WriteU64(0)
	takeBuf.WriteU32(1)   // mail ID 1
	takeBuf.WriteU32(501) // item GUID 501
	_ = sessBob.handleMailTakeItem(ctx, takeBuf.Bytes())

	// Verify Bob did not get the item and lost no money
	if sessBob.player.Money != 50 {
		t.Fatalf("expected Bob money to remain 50, got %d", sessBob.player.Money)
	}
	var invCount int64
	_ = db.QueryRow("SELECT COUNT(*) FROM character_inventory WHERE guid = 2").Scan(&invCount)
	if invCount != 0 {
		t.Fatalf("expected 0 inventory items, got %d", invCount)
	}

	// 2. Give Bob 500 money and try again
	sessBob.player.Money = 500
	_, _ = db.Exec("UPDATE characters SET money = 500 WHERE guid = 2")

	if !sessBob.handleMailTakeItem(ctx, takeBuf.Bytes()) {
		t.Fatal("handleMailTakeItem failed when Bob had enough money")
	}

	// Verify Bob money deducted by 150
	if sessBob.player.Money != 500-150 {
		t.Fatalf("expected Bob money 350, got %d", sessBob.player.Money)
	}

	// Verify item is now in Bob's inventory
	var bagSlot int64
	err = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 2 AND item = 501").Scan(&bagSlot)
	if err != nil || bagSlot < 23 || bagSlot > 38 {
		t.Fatalf("item not found in inventory or invalid slot: %v, slot=%d", err, bagSlot)
	}

	// Verify Alice received COD payment mail with 150 copper
	var paymentMoney int64
	var paymentSubject string
	err = db.QueryRow("SELECT money, subject FROM mail WHERE receiver = 1").Scan(&paymentMoney, &paymentSubject)
	if err != nil {
		t.Fatalf("Alice COD payment mail not found: %v", err)
	}
	if paymentMoney != 150 {
		t.Fatalf("expected COD payment 150 copper, got %d", paymentMoney)
	}
}
