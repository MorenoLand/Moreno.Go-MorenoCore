package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestPlayerToPlayerTrade(t *testing.T) {
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
		"INSERT INTO characters VALUES (1, 'Player1', 5000, '')",
		"INSERT INTO characters VALUES (2, 'Player2', 2000, '')",
		"INSERT INTO item_instance VALUES (101, 5555, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 101)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sess1 := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Player1", Money: 5000}}
	sess2 := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Player2", Money: 2000}}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	ctx := context.Background()

	// 1. Initiate trade
	initBuf := protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	if !sess1.handleInitiateTrade(ctx, initBuf.Bytes()) {
		t.Fatal("handleInitiateTrade failed")
	}

	// 2. Begin trade window
	if !sess1.handleBeginTrade(ctx) {
		t.Fatal("handleBeginTrade failed")
	}

	// 3. Player 1 offers 1000 gold and item in slot 23
	goldBuf := protocol.NewBuffer(4)
	goldBuf.WriteU32(1000)
	if !sess1.handleSetTradeGold(ctx, goldBuf.Bytes()) {
		t.Fatal("handleSetTradeGold failed")
	}
	itemBuf := protocol.NewBuffer(3)
	itemBuf.WriteU8(0)  // tradeSlot 0
	itemBuf.WriteU8(0)  // bag 0
	itemBuf.WriteU8(23) // slot 23
	if !sess1.handleSetTradeItem(ctx, itemBuf.Bytes()) {
		t.Fatal("handleSetTradeItem failed")
	}

	// 4. Player 2 offers 500 gold
	goldBuf2 := protocol.NewBuffer(4)
	goldBuf2.WriteU32(500)
	if !sess2.handleSetTradeGold(ctx, goldBuf2.Bytes()) {
		t.Fatal("handleSetTradeGold 2 failed")
	}

	// 5. Both accept trade
	if !sess1.handleAcceptTrade(ctx) {
		t.Fatal("handleAcceptTrade 1 failed")
	}
	if !sess2.handleAcceptTrade(ctx) {
		t.Fatal("handleAcceptTrade 2 failed")
	}

	// 6. Verify trade executed: Player 1 has 5000 - 1000 + 500 = 4500, Player 2 has 2000 - 500 + 1000 = 2500
	if sess1.player.Money != 4500 || sess2.player.Money != 2500 {
		t.Fatalf("money mismatch: p1=%d p2=%d", sess1.player.Money, sess2.player.Money)
	}

	// 7. Verify item 101 ownership moved to Player 2
	var owner int64
	_ = db.QueryRow("SELECT owner_guid FROM item_instance WHERE guid = 101").Scan(&owner)
	if owner != 2 {
		t.Fatalf("expected item 101 owner to be 2, got %d", owner)
	}
}
