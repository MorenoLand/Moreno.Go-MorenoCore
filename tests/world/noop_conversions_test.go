//go:build ignore

package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestItemRepairAndWrapAndSocket(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE item_instance (
			guid INTEGER PRIMARY KEY,
			itemEntry INTEGER NOT NULL,
			owner_guid INTEGER NOT NULL,
			creatorGuid INTEGER NOT NULL DEFAULT 0,
			count INTEGER NOT NULL DEFAULT 1,
			duration INTEGER NOT NULL DEFAULT 0,
			charges TEXT NOT NULL DEFAULT '',
			flags INTEGER NOT NULL DEFAULT 0,
			enchantments TEXT NOT NULL DEFAULT '',
			randomPropertyId INTEGER NOT NULL DEFAULT 0,
			durability INTEGER NOT NULL DEFAULT 0,
			playedTime INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE character_inventory (
			guid INTEGER NOT NULL,
			bag INTEGER NOT NULL,
			slot INTEGER NOT NULL,
			item INTEGER PRIMARY KEY
		)`,
		`CREATE TABLE character_gifts (
			guid INTEGER NOT NULL,
			item_guid INTEGER PRIMARY KEY,
			entry INTEGER NOT NULL,
			flags INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE characters (
			guid INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			money INTEGER NOT NULL DEFAULT 0
		)`,
		"INSERT INTO characters (guid, name, money) VALUES (1, 'Hero', 50000)",
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, durability) VALUES (10, 1001, 1, 10)",
		"INSERT INTO character_inventory (guid, bag, slot, item) VALUES (1, 0, 1, 10)",
		// Wrapper paper (entry 5042)
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, durability) VALUES (20, 5042, 1, 0)",
		"INSERT INTO character_inventory (guid, bag, slot, item) VALUES (1, 0, 23, 20)",
		// Item to wrap (entry 25)
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, durability) VALUES (21, 25, 1, 0)",
		"INSERT INTO character_inventory (guid, bag, slot, item) VALUES (1, 0, 24, 21)",
		// Gem to socket
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, durability) VALUES (30, 7001, 1, 0)",
		"INSERT INTO character_inventory (guid, bag, slot, item) VALUES (1, 0, 25, 30)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	wdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()
	wdb.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE item_template (
			entry INTEGER PRIMARY KEY,
			MaxDurability INTEGER NOT NULL DEFAULT 0
		)`,
		"INSERT INTO item_template (entry, MaxDurability) VALUES (1001, 50)",
	} {
		if _, err := wdb.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	cStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	wStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: wdb}
	srv := &Server{CharactersStore: cStore, WorldStore: wStore, Config: config.Default()}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Name: "Hero", Money: 50000},
	}
	ctx := context.Background()

	// Drain frames in background
	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	// 1. Repair item 10 (durability 10 -> 50, cost = 40 * 10 = 400 copper)
	rBuf := protocol.NewBuffer(17)
	rBuf.WriteU64(100) // npcGUID
	rBuf.WriteU64(10)  // itemGUID
	rBuf.WriteU8(0)   // guildBank
	if !sess.handleRepairItem(ctx, rBuf.Bytes()) {
		t.Fatal("handleRepairItem failed")
	}

	var d uint32
	_ = db.QueryRow("SELECT durability FROM item_instance WHERE guid = 10").Scan(&d)
	if d != 50 {
		t.Fatalf("expected repaired durability 50, got %d", d)
	}
	if sess.player.Money != 49600 {
		t.Fatalf("expected player money 49600, got %d", sess.player.Money)
	}

	// 2. Wrap item 21 with wrapper 20
	wrapBuf := protocol.NewBuffer(4)
	wrapBuf.WriteU8(0)  // giftBag
	wrapBuf.WriteU8(23) // giftSlot
	wrapBuf.WriteU8(0)  // itemBag
	wrapBuf.WriteU8(24) // itemSlot
	if !sess.handleWrapItem(ctx, wrapBuf.Bytes()) {
		t.Fatal("handleWrapItem failed")
	}

	var giftEntry uint32
	_ = db.QueryRow("SELECT entry FROM character_gifts WHERE item_guid = 21").Scan(&giftEntry)
	if giftEntry != 25 {
		t.Fatalf("expected character_gifts record for original entry 25, got %d", giftEntry)
	}

	// 3. Socket gem 30 into item 10
	sockBuf := protocol.NewBuffer(32)
	sockBuf.WriteU64(10) // itemGUID
	sockBuf.WriteU64(30) // gem 1
	sockBuf.WriteU64(0)
	sockBuf.WriteU64(0)
	if !sess.handleSocketGems(ctx, sockBuf.Bytes()) {
		t.Fatal("handleSocketGems failed")
	}

	var gemCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM item_instance WHERE guid = 30").Scan(&gemCount)
	if gemCount != 0 {
		t.Fatalf("expected socketed gem 30 consumed, count=%d", gemCount)
	}
}

func TestLootOptOutAndTaxiBenchmark(t *testing.T) {
	sess := &session{
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Name: "Hero"},
	}
	ctx := context.Background()

	// 1. Opt out of loot
	optBuf := protocol.NewBuffer(4)
	optBuf.WriteU32(1)
	if !sess.handleOptOutOfLoot(ctx, optBuf.Bytes()) {
		t.Fatal("handleOptOutOfLoot failed")
	}
	if !sess.player.PassOnGroupLoot {
		t.Fatal("expected PassOnGroupLoot true")
	}

	optBuf2 := protocol.NewBuffer(4)
	optBuf2.WriteU32(0)
	if !sess.handleOptOutOfLoot(ctx, optBuf2.Bytes()) {
		t.Fatal("handleOptOutOfLoot failed")
	}
	if sess.player.PassOnGroupLoot {
		t.Fatal("expected PassOnGroupLoot false")
	}

	// 2. Taxi benchmark mode
	benchBuf := protocol.NewBuffer(1)
	benchBuf.WriteU8(1)
	if !sess.handleSetTaxiBenchmarkMode(ctx, benchBuf.Bytes()) {
		t.Fatal("handleSetTaxiBenchmarkMode failed")
	}
	const playerFlagTaxiBenchmark uint32 = 0x04000000
	if sess.player.PlayerFlags&playerFlagTaxiBenchmark == 0 {
		t.Fatal("expected playerFlagTaxiBenchmark flag set")
	}

	benchBuf2 := protocol.NewBuffer(1)
	benchBuf2.WriteU8(0)
	if !sess.handleSetTaxiBenchmarkMode(ctx, benchBuf2.Bytes()) {
		t.Fatal("handleSetTaxiBenchmarkMode failed")
	}
	if sess.player.PlayerFlags&playerFlagTaxiBenchmark != 0 {
		t.Fatal("expected playerFlagTaxiBenchmark flag cleared")
	}
}

func TestSummonResponseAndSplineDone(t *testing.T) {
	srv := &Server{Config: config.Default()}
	srv.sessions = make(map[*session]struct{})

	summoner := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   99,
		player:       &playerState{GUID: 99, Name: "Warlock", Map: 571, X: 100, Y: 200, Z: 300, Orientation: 1.5},
	}
	srv.sessions[summoner] = struct{}{}

	sess := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Name: "Hero", Map: 0, X: 0, Y: 0, Z: 0},
	}
	srv.sessions[sess] = struct{}{}

	ctx := context.Background()

	// 1. Agree to summon
	sumBuf := protocol.NewBuffer(9)
	sumBuf.WriteU64(99)
	sumBuf.WriteU8(1) // agree
	if !sess.handleSummonResponse(ctx, sumBuf.Bytes()) {
		t.Fatal("handleSummonResponse failed")
	}
	if sess.player.Map != 571 || sess.player.X != 100 || sess.player.Y != 200 || sess.player.Z != 300 {
		t.Fatalf("expected player teleported to summoner, got map=%d x=%f y=%f z=%f", sess.player.Map, sess.player.X, sess.player.Y, sess.player.Z)
	}

	// 2. Request vehicle exit
	if !sess.handleRequestVehicleExit(ctx, nil) {
		t.Fatal("handleRequestVehicleExit failed")
	}
}
