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
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
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
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
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

func TestMailValidationSelfAndTeamAndCapacity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, race INTEGER, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER, PRIMARY KEY (mail_id, item_guid))",
		"INSERT INTO characters VALUES (1, 'Alice', 1, 5000, '')",  // Race 1 = Human (Alliance)
		"INSERT INTO characters VALUES (2, 'Thrall', 2, 5000, '')", // Race 2 = Orc (Horde)
		"INSERT INTO characters VALUES (3, 'Bob', 1, 5000, '')",    // Race 1 = Human (Alliance)
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sessAlice := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Alice", Race: 1, Money: 5000}}
	srv.sessions[sessAlice] = struct{}{}

	ctx := context.Background()

	buildMailPayload := func(target string) []byte {
		buf := protocol.NewBuffer(128)
		buf.WriteU64(0)
		buf.WriteCString(target)
		buf.WriteCString("Subject")
		buf.WriteCString("Body")
		buf.WriteU32(41)
		buf.WriteU32(0)
		buf.WriteU8(0) // 0 attachments
		buf.WriteU32(100)
		buf.WriteU32(0)
		return buf.Bytes()
	}

	// 1. Cannot send mail to self
	sessAlice.handleSendMail(ctx, buildMailPayload("Alice"))
	if sessAlice.player.Money != 5000 {
		t.Fatalf("expected money unchanged after self-mail attempt, got %d", sessAlice.player.Money)
	}

	// 2. Cannot send mail to opposing faction (Orc)
	sessAlice.handleSendMail(ctx, buildMailPayload("Thrall"))
	if sessAlice.player.Money != 5000 {
		t.Fatalf("expected money unchanged after opposing faction attempt, got %d", sessAlice.player.Money)
	}

	// 3. Cannot send mail to nonexistent character
	sessAlice.handleSendMail(ctx, buildMailPayload("GhostPlayer"))
	if sessAlice.player.Money != 5000 {
		t.Fatalf("expected money unchanged after nonexistent character attempt, got %d", sessAlice.player.Money)
	}

	// 4. Fill Bob's mailbox to 100 mails
	for i := 1; i <= 100; i++ {
		_, err := db.Exec("INSERT INTO mail VALUES (?, 0, 41, 0, 1, 3, 'Spam', '', 0, 9999999999, 0, 0, 0, 0)", 1000+i)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Alice tries to send to Bob whose mailbox is full (capacity 100)
	sessAlice.handleSendMail(ctx, buildMailPayload("Bob"))
	if sessAlice.player.Money != 5000 {
		t.Fatalf("expected money unchanged after sending to full mailbox, got %d", sessAlice.player.Money)
	}
}

func TestMailReturnToSender(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, race INTEGER, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER, PRIMARY KEY (mail_id, item_guid))",
		"INSERT INTO characters VALUES (1, 'Alice', 1, 5000, '')",
		"INSERT INTO characters VALUES (2, 'Bob', 1, 5000, '')",
		"INSERT INTO item_instance VALUES (301, 12345, 2, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO mail VALUES (10, 0, 41, 0, 1, 2, 'Package for Bob', 'Here you go', 1, 9999999999, 0, 0, 0, 0)",
		"INSERT INTO mail_items VALUES (10, 301, 12345, 2)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sessBob := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Bob", Race: 1, Money: 5000}}
	srv.sessions[sessBob] = struct{}{}

	ctx := context.Background()

	returnBuf := protocol.NewBuffer(12)
	returnBuf.WriteU64(0)
	returnBuf.WriteU32(10) // mailID 10

	if !sessBob.handleMailReturnToSender(ctx, returnBuf.Bytes()) {
		t.Fatal("handleMailReturnToSender failed")
	}

	// Verify mail table: sender is now Bob (2), receiver is Alice (1), checked bit 2 set
	var sender, receiver, checked int64
	err = db.QueryRow("SELECT sender, receiver, checked FROM mail WHERE id = 10").Scan(&sender, &receiver, &checked)
	if err != nil {
		t.Fatalf("failed to query returned mail: %v", err)
	}
	if sender != 2 || receiver != 1 {
		t.Fatalf("expected sender 2 and receiver 1, got sender %d, receiver %d", sender, receiver)
	}
	if checked&2 == 0 {
		t.Fatalf("expected MAIL_CHECK_MASK_RETURNED (2) set on checked, got %d", checked)
	}

	// Verify item ownership returned to Alice (1)
	var itemOwner int64
	err = db.QueryRow("SELECT owner_guid FROM item_instance WHERE guid = 301").Scan(&itemOwner)
	if err != nil || itemOwner != 1 {
		t.Fatalf("expected item 301 owner_guid to be 1, got %d (err: %v)", itemOwner, err)
	}

	var mailItemReceiver int64
	err = db.QueryRow("SELECT receiver FROM mail_items WHERE mail_id = 10 AND item_guid = 301").Scan(&mailItemReceiver)
	if err != nil || mailItemReceiver != 1 {
		t.Fatalf("expected mail_items receiver to be 1, got %d (err: %v)", mailItemReceiver, err)
	}
}

func TestMailCreateTextItem(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, race INTEGER, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER, PRIMARY KEY (mail_id, item_guid))",
		"INSERT INTO characters VALUES (1, 'Alice', 1, 5000, '')",
		"INSERT INTO mail VALUES (20, 0, 41, 0, 2, 1, 'Letter Subject', 'Secret Meeting Notes', 0, 9999999999, 0, 0, 0, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sessAlice := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Alice", Race: 1, Money: 5000}}
	srv.sessions[sessAlice] = struct{}{}

	ctx := context.Background()

	createBuf := protocol.NewBuffer(12)
	createBuf.WriteU64(0)
	createBuf.WriteU32(20) // mailID 20

	if !sessAlice.handleMailCreateTextItem(ctx, createBuf.Bytes()) {
		t.Fatal("handleMailCreateTextItem failed")
	}

	// Verify letter item created in inventory
	var invSlot, itemGUID int64
	err = db.QueryRow("SELECT slot, item FROM character_inventory WHERE guid = 1 AND bag = 0").Scan(&invSlot, &itemGUID)
	if err != nil {
		t.Fatalf("failed to query inventory for created text item: %v", err)
	}
	if invSlot < 23 || invSlot > 38 {
		t.Fatalf("expected backpack slot 23..38, got %d", invSlot)
	}

	// Verify item text
	var itemEntry int64
	var text string
	err = db.QueryRow("SELECT itemEntry, text FROM item_instance WHERE guid = ?", itemGUID).Scan(&itemEntry, &text)
	if err != nil || itemEntry != 8383 || text != "Secret Meeting Notes" {
		t.Fatalf("expected item 8383 with text 'Secret Meeting Notes', got itemEntry=%d, text=%q, err=%v", itemEntry, text, err)
	}

	// Verify mail checked bit 4 (MAIL_CHECK_MASK_COPIED) set
	var checked int64
	err = db.QueryRow("SELECT checked FROM mail WHERE id = 20").Scan(&checked)
	if err != nil || (checked&4) == 0 {
		t.Fatalf("expected checked mask to contain bit 4 (COPIED), got %d (err: %v)", checked, err)
	}
}
