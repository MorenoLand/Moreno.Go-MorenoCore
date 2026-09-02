package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestVendorBuyingAndSelling(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER, BuyPrice INTEGER, SellPrice INTEGER, MaxDurability INTEGER, BuyCount INTEGER)",
		"CREATE TABLE npc_vendor (entry INTEGER, slot INTEGER, item INTEGER, maxcount INTEGER, incrtime INTEGER, ExtendedCost INTEGER)",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO item_template VALUES (5001, 100, 50, 10, 100, 1)",
		"INSERT INTO npc_vendor VALUES (101, 1, 5001, 0, 0, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 1000}}

	// Vendor list
	vendorGUID := creatureWorldGUID(1, 101)
	payload := protocol.NewBuffer(8)
	payload.WriteU64(vendorGUID)
	if !sess.handleListInventory(context.Background(), payload.Bytes()) {
		t.Fatal("handleListInventory failed")
	}

	// Buy 2 items
	buyBuf := protocol.NewBuffer(20)
	buyBuf.WriteU64(vendorGUID)
	buyBuf.WriteU32(5001) // itemEntry
	buyBuf.WriteU32(1)    // slot
	buyBuf.WriteU32(2)    // count
	if !sess.handleBuyItem(context.Background(), buyBuf.Bytes()) {
		t.Fatal("handleBuyItem failed")
	}
	if sess.player.Money != 900 {
		t.Fatalf("expected 900 money after buy, got %d", sess.player.Money)
	}

	// Verify item was stored in character_inventory
	var itemGUID int64
	err = db.QueryRow("SELECT item FROM character_inventory WHERE guid = 1 AND slot = 23").Scan(&itemGUID)
	if err != nil {
		t.Fatal(err)
	}
	if itemGUID == 0 {
		t.Fatal("expected item to be stored in slot 23")
	}

	// Sell the item back
	sellBuf := protocol.NewBuffer(17)
	sellBuf.WriteU64(vendorGUID)
	sellBuf.WriteU64(uint64(itemGUID))
	sellBuf.WriteU8(2) // count
	if !sess.handleSellItem(context.Background(), sellBuf.Bytes()) {
		t.Fatal("handleSellItem failed")
	}
	if sess.player.Money != 920 { // 900 + (2 * 10)
		t.Fatalf("expected 920 money after sell, got %d", sess.player.Money)
	}
}

func TestVendorInfiniteStockUsesZeroWireCount(t *testing.T) {
	if got := vendorStockValue(0); got != 0 {
		t.Fatalf("infinite stock=%d", got)
	}
	if got := vendorStockValue(7); got != 7 {
		t.Fatalf("finite stock=%d", got)
	}
}

func TestVendorListEncodesUnlimitedStockAsZero(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE npc_vendor (entry INTEGER, slot INTEGER, item INTEGER, maxcount INTEGER, incrtime INTEGER, ExtendedCost INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER, BuyPrice INTEGER, MaxDurability INTEGER, BuyCount INTEGER)",
		"INSERT INTO npc_vendor VALUES (101, 1, 5001, 0, 0, 0)",
		"INSERT INTO item_template VALUES (5001, 100, 50, 100, 1)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	sess := &session{conn: serverConn, server: &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}}, playerLoaded: true, player: &playerState{GUID: 1}}
	done := make(chan bool, 1)
	go func() { done <- sess.sendVendorList(context.Background(), creatureWorldGUID(1, 101)) }()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !<-done || opcode != uint16(protocol.OpcodeSMSG_LIST_INVENTORY) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	if _, err := reader.ReadU64(); err != nil {
		t.Fatal(err)
	}
	if count, err := reader.ReadU8(); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	for i := 0; i < 3; i++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if stock, err := reader.ReadU32(); err != nil || stock != 0 {
		t.Fatalf("unlimited stock=%d err=%v", stock, err)
	}
}

func TestBuyItemSendsItemCreate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER, BuyPrice INTEGER, SellPrice INTEGER, MaxDurability INTEGER, BuyCount INTEGER)",
		"CREATE TABLE npc_vendor (entry INTEGER, slot INTEGER, item INTEGER, maxcount INTEGER, incrtime INTEGER, ExtendedCost INTEGER)",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO item_template VALUES (5001, 100, 50, 10, 100, 1)",
		"INSERT INTO npc_vendor VALUES (101, 1, 5001, 0, 0, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{conn: serverConn, server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 1000}}

	// Buy item
	vendorGUID := creatureWorldGUID(1, 101)
	buyBuf := protocol.NewBuffer(20)
	buyBuf.WriteU64(vendorGUID)
	buyBuf.WriteU32(5001) // itemEntry
	buyBuf.WriteU32(1)    // slot
	buyBuf.WriteU32(1)    // count

	go func() {
		sess.handleBuyItem(context.Background(), buyBuf.Bytes())
	}()

	// 1. Read SMSG_BUY_ITEM
	op1, _, err := readServerFrame(clientConn, nil)
	if err != nil || op1 != uint16(protocol.OpcodeSMSG_BUY_ITEM) {
		t.Fatalf("expected SMSG_BUY_ITEM, got op=%x err=%v", op1, err)
	}

	// 2. Read SMSG_UPDATE_OBJECT (item create block)
	op2, _, err := readServerFrame(clientConn, nil)
	if err != nil || (op2 != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op2 != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT)) {
		t.Fatalf("expected update object for item creation, got op=%x err=%v", op2, err)
	}
}


