package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestItemAutoEquipAndSwap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, InventoryType INTEGER)",
		"INSERT INTO characters VALUES (1, '')",
		"INSERT INTO item_template VALUES (1001, 1)", // Head
		"INSERT INTO item_instance VALUES (500, 1001, 1)",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 500)", // in backpack slot 23
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	// Auto equip item from backpack bag 0 slot 23
	if !sess.handleAutoEquipItem(context.Background(), []byte{0, 23}) {
		t.Fatal("handleAutoEquipItem failed")
	}

	// Verify it was moved to slot 0 (Head)
	var bag, slot int
	err = db.QueryRow("SELECT bag, slot FROM character_inventory WHERE item = 500").Scan(&bag, &slot)
	if err != nil {
		t.Fatal(err)
	}
	if bag != 0 || slot != 0 {
		t.Fatalf("expected bag 0 slot 0, got bag %d slot %d", bag, slot)
	}

	// Swap item from slot 0 to slot 23
	if !sess.handleSwapInvItem(context.Background(), []byte{0, 23}) {
		t.Fatal("handleSwapInvItem failed")
	}
	err = db.QueryRow("SELECT bag, slot FROM character_inventory WHERE item = 500").Scan(&bag, &slot)
	if err != nil {
		t.Fatal(err)
	}
	if bag != 0 || slot != 23 {
		t.Fatalf("expected bag 0 slot 23, got bag %d slot %d", bag, slot)
	}
}

func TestSplitItemStack(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, count INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, InventoryType INTEGER, ContainerSlots INTEGER)",
		"INSERT INTO characters VALUES (1, '')",
		"INSERT INTO item_template VALUES (1001, 0, 0)",
		"INSERT INTO item_instance VALUES (500, 1001, 1, 20)",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 500)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	payload := []byte{0, 23, 0, 24, 5, 0, 0, 0, 0}
	if !sess.handleSplitItem(context.Background(), payload) {
		t.Fatal("handleSplitItem failed")
	}
	var sourceCount int
	if err := db.QueryRow("SELECT count FROM item_instance WHERE guid = 500").Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if sourceCount != 15 {
		t.Fatalf("source count=%d", sourceCount)
	}
	var splitCount int
	if err := db.QueryRow("SELECT count FROM item_instance WHERE guid <> 500").Scan(&splitCount); err != nil {
		t.Fatal(err)
	}
	if splitCount != 5 {
		t.Fatalf("split count=%d", splitCount)
	}
}

func TestAutoStoreItemIntoEquippedBag(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, count INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, InventoryType INTEGER, ContainerSlots INTEGER)",
		"INSERT INTO characters VALUES (1, '')",
		"INSERT INTO item_template VALUES (5001, 18, 4)",
		"INSERT INTO item_template VALUES (1001, 0, 0)",
		"INSERT INTO item_instance VALUES (200, 5001, 1, 1)",
		"INSERT INTO item_instance VALUES (500, 1001, 1, 1)",
		"INSERT INTO character_inventory VALUES (1, 0, 19, 200)",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 500)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	if !sess.handleAutoStoreBagItem(context.Background(), []byte{0, 23, 19}) {
		t.Fatal("handleAutoStoreBagItem failed")
	}
	var bag, slot int
	if err := db.QueryRow("SELECT bag, slot FROM character_inventory WHERE item = 500").Scan(&bag, &slot); err != nil {
		t.Fatal(err)
	}
	if bag != 200 || slot != 0 {
		t.Fatalf("item location=%d/%d", bag, slot)
	}
}
