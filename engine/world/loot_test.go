package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestLootingMoneyAndItems(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, played_time INTEGER, text TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER)",
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER, map INTEGER, position_x REAL, position_y REAL, position_z REAL, curhealth INTEGER)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, minGold INTEGER, maxGold INTEGER)",
		"CREATE TABLE creature_loot_template (Entry INTEGER, Item INTEGER, Chance REAL, QuestRequired INTEGER, LootMode INTEGER, GroupId INTEGER, MinCount INTEGER, MaxCount INTEGER)",
		"INSERT INTO characters VALUES (1, 100, '')",
		"INSERT INTO creature VALUES (1, 303, 0, 0, 0, 0, 0)",
		"INSERT INTO creature_template VALUES (303, 50, 50)",
		"INSERT INTO item_template VALUES (7001, 200)",
		"INSERT INTO creature_loot_template VALUES (303, 7001, 100.0, 0, 1, 0, 1, 1)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 100}}

	// 1. Send CMSG_LOOT
	targetGUID := creatureWorldGUID(1, 303)
	lootBuf := protocol.NewBuffer(8)
	lootBuf.WriteU64(targetGUID)
	if !sess.handleLoot(context.Background(), lootBuf.Bytes()) {
		t.Fatal("handleLoot failed")
	}
	if sess.activeLoot == nil || sess.activeLoot.Money != 50 {
		t.Fatalf("expected 50 copper in loot, got %v", sess.activeLoot)
	}

	// 2. Loot money
	if !sess.handleLootMoney(context.Background()) {
		t.Fatal("handleLootMoney failed")
	}
	if sess.player.Money != 150 {
		t.Fatalf("expected 150 money after loot, got %d", sess.player.Money)
	}

	// 3. Loot item from slot 0
	itemBuf := protocol.NewBuffer(1)
	itemBuf.WriteU8(0)
	if !sess.handleAutostoreLootItem(context.Background(), itemBuf.Bytes()) {
		t.Fatal("handleAutostoreLootItem failed")
	}
	var storedItemEntry int64
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 1").Scan(&storedItemEntry)
	if err != nil || storedItemEntry != 7001 {
		t.Fatalf("expected item 7001 stored in inventory, err=%v entry=%d", err, storedItemEntry)
	}

	// 4. Release loot
	releaseBuf := protocol.NewBuffer(8)
	releaseBuf.WriteU64(targetGUID)
	if !sess.handleLootRelease(releaseBuf.Bytes()) {
		t.Fatal("handleLootRelease failed")
	}
	if sess.activeLoot != nil {
		t.Fatal("expected activeLoot to be cleared")
	}
	if len(srv.creatureLoot) != 0 {
		t.Fatal("expected creature loot state to be cleared")
	}
}
