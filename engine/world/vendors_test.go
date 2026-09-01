package world

import (
	"context"
	"database/sql"
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
