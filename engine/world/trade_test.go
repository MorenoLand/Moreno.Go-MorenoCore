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
		"INSERT INTO item_instance VALUES (101, 5555, 1, 0, 1, 0, '', 0, '3789 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0', 0, 100, 0, '')",
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
	if sess1.trade.Items[0].EnchantID != 3789 {
		t.Fatalf("expected EnchantID 3789, got %d", sess1.trade.Items[0].EnchantID)
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

func TestTradeValidations(t *testing.T) {
	srv := &Server{sessions: make(map[*session]struct{})}
	sess1 := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Player1", Health: 100, MaxHealth: 100, Race: 1, Map: 0, X: 0, Y: 0, Z: 0}}
	sess2 := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Player2", Health: 100, MaxHealth: 100, Race: 1, Map: 0, X: 0, Y: 0, Z: 0}}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	ctx := context.Background()

	// 1. Non-existent target -> tradeStatusNoTarget (6)
	initBuf := protocol.NewBuffer(8)
	initBuf.WriteU64(999)
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	if sess1.trade != nil {
		t.Fatal("expected trade to not start with invalid target")
	}

	// 2. Target dead -> tradeStatusTargetDead (18)
	sess2.player.Health = 0
	initBuf = protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	if sess1.trade != nil {
		t.Fatal("expected trade to not start with dead target")
	}
	sess2.player.Health = 100

	// 3. Target too far (> 11.11 yards) -> tradeStatusTargetTooFar (10)
	sess2.player.X = 20.0
	initBuf = protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	if sess1.trade != nil {
		t.Fatal("expected trade to not start with distant target")
	}
	sess2.player.X = 0.0

	// 4. Target opposite faction -> tradeStatusWrongFaction (11)
	// Race 1 = Human (Alliance, team 0), Race 2 = Orc (Horde, team 1)
	sess2.player.Race = 2
	initBuf = protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	if sess1.trade != nil {
		t.Fatal("expected trade to not start with opposite faction")
	}
	sess2.player.Race = 1

	// 5. Success initiation
	initBuf = protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	if !sess1.handleInitiateTrade(ctx, initBuf.Bytes()) || sess1.trade == nil || sess2.trade == nil {
		t.Fatal("expected successful trade initiation")
	}
}

func TestTradeNonTradedSlotAndCancel(t *testing.T) {
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
		"INSERT INTO characters VALUES (1, 'Player1', 1000, '')",
		"INSERT INTO characters VALUES (2, 'Player2', 1000, '')",
		// item 201 in slot 23, item 202 in slot 24
		"INSERT INTO item_instance VALUES (201, 7001, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO item_instance VALUES (202, 7002, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 201)",
		"INSERT INTO character_inventory VALUES (1, 0, 24, 202)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sess1 := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Player1", Health: 100, MaxHealth: 100, Race: 1, Money: 1000}}
	sess2 := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Player2", Health: 100, MaxHealth: 100, Race: 1, Money: 1000}}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	ctx := context.Background()

	// Initiate & open trade
	initBuf := protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	sess2.handleBeginTrade(ctx)

	// Player 1 puts item 201 into traded slot 0, item 202 into non-traded slot 6
	itemBuf0 := protocol.NewBuffer(3)
	itemBuf0.WriteU8(0)  // slot 0 (traded)
	itemBuf0.WriteU8(0)  // bag 0
	itemBuf0.WriteU8(23) // inv slot 23
	sess1.handleSetTradeItem(ctx, itemBuf0.Bytes())

	itemBuf6 := protocol.NewBuffer(3)
	itemBuf6.WriteU8(6)  // slot 6 (non-traded!)
	itemBuf6.WriteU8(0)  // bag 0
	itemBuf6.WriteU8(24) // inv slot 24
	sess1.handleSetTradeItem(ctx, itemBuf6.Bytes())

	// Test unaccept: player 1 accepts, then modifies trade (or unaccepts)
	sess1.handleAcceptTrade(ctx)
	if !sess1.trade.Accepted {
		t.Fatal("expected sess1 accepted")
	}
	sess1.handleUnacceptTrade(ctx)
	if sess1.trade.Accepted {
		t.Fatal("expected sess1 unaccepted")
	}

	// Re-accept both
	sess1.handleAcceptTrade(ctx)
	sess2.handleAcceptTrade(ctx)

	// Item 201 (slot 0) should be transferred to player 2
	var owner201 int64
	_ = db.QueryRow("SELECT owner_guid FROM item_instance WHERE guid = 201").Scan(&owner201)
	if owner201 != 2 {
		t.Fatalf("expected item 201 transferred to player 2, got %d", owner201)
	}

	// Item 202 (slot 6, non-traded) MUST NOT be transferred, must stay with player 1!
	var owner202 int64
	_ = db.QueryRow("SELECT owner_guid FROM item_instance WHERE guid = 202").Scan(&owner202)
	if owner202 != 1 {
		t.Fatalf("expected item 202 in non-traded slot 6 to stay with player 1, got %d", owner202)
	}

	// Now test handleCancelTrade
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	if sess1.trade == nil || sess2.trade == nil {
		t.Fatal("expected new trade initiated")
	}
	sess1.handleCancelTrade(ctx)
	if sess1.trade != nil || sess2.trade != nil {
		t.Fatal("expected trade states cleared after cancel")
	}
}

func TestTradeParitySoulboundDistanceBags(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, ContainerSlots INTEGER)",
		"INSERT INTO characters VALUES (1, 'Player1', 1000, '')",
		"INSERT INTO characters VALUES (2, 'Player2', 1000, '')",
		// Soulbound item (flags = 1) in slot 23 for player 1
		"INSERT INTO item_instance VALUES (301, 8001, 1, 0, 1, 0, '', 1, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 301)",
		// Normal item (flags = 0) in slot 24 for player 1
		"INSERT INTO item_instance VALUES (302, 8002, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 24, 302)",
		// Equipped bag (guid 500, entry 9001 with 4 slots) in slot 19 for player 2
		"INSERT INTO item_template VALUES (9001, 4)",
		"INSERT INTO item_instance VALUES (500, 9001, 2, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (2, 0, 19, 500)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	// Fill Player 2's backpack (slots 23..38) completely
	for sl := 23; sl <= 38; sl++ {
		dummyGUID := 600 + sl
		_, err := db.Exec("INSERT INTO item_instance VALUES (?, 9999, 2, 0, 1, 0, '', 0, '', 0, 100, 0, '')", dummyGUID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("INSERT INTO character_inventory VALUES (2, 0, ?, ?)", sl, dummyGUID)
		if err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sess1 := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Player1", Health: 100, MaxHealth: 100, Race: 1, Map: 0, X: 0, Y: 0, Z: 0, Money: 1000}}
	sess2 := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Player2", Health: 100, MaxHealth: 100, Race: 1, Map: 0, X: 0, Y: 0, Z: 0, Money: 1000}}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	ctx := context.Background()

	// 1. Initiate trade
	initBuf := protocol.NewBuffer(8)
	initBuf.WriteU64(2)
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	sess2.handleBeginTrade(ctx)

	// 2. Try placing soulbound item (301) into traded slot 0 -> should be rejected!
	itemBufSoulbound := protocol.NewBuffer(3)
	itemBufSoulbound.WriteU8(0)  // slot 0
	itemBufSoulbound.WriteU8(0)  // bag 0
	itemBufSoulbound.WriteU8(23) // inv slot 23 (item 301, soulbound)
	sess1.handleSetTradeItem(ctx, itemBufSoulbound.Bytes())
	if sess1.trade.Items[0].ItemGUID != 0 {
		t.Fatalf("expected soulbound item not placed in slot 0, got %d", sess1.trade.Items[0].ItemGUID)
	}

	// 3. Placing soulbound item into non-traded slot 6 -> should be allowed!
	itemBufSoulbound6 := protocol.NewBuffer(3)
	itemBufSoulbound6.WriteU8(6)  // slot 6
	itemBufSoulbound6.WriteU8(0)  // bag 0
	itemBufSoulbound6.WriteU8(23) // inv slot 23
	sess1.handleSetTradeItem(ctx, itemBufSoulbound6.Bytes())
	if sess1.trade.Items[6].ItemGUID != 301 {
		t.Fatalf("expected soulbound item allowed in non-traded slot 6, got %d", sess1.trade.Items[6].ItemGUID)
	}

	// 4. Test distance check on accept: if partner is > 11.11 yards away
	sess2.player.X = 15.0 // partner moved away
	sess1.handleAcceptTrade(ctx)
	if sess1.trade != nil || sess2.trade != nil {
		t.Fatal("expected trade canceled due to distance on accept")
	}
	sess2.player.X = 0.0 // reset position

	// Re-initiate trade
	sess1.handleInitiateTrade(ctx, initBuf.Bytes())
	sess2.handleBeginTrade(ctx)

	// 5. Test trade into equipped bag:
	// Player 1 offers normal tradeable item 302
	itemBufNormal := protocol.NewBuffer(3)
	itemBufNormal.WriteU8(0)  // slot 0
	itemBufNormal.WriteU8(0)  // bag 0
	itemBufNormal.WriteU8(24) // inv slot 24 (item 302)
	sess1.handleSetTradeItem(ctx, itemBufNormal.Bytes())

	// Both accept
	sess1.handleAcceptTrade(ctx)
	sess2.handleAcceptTrade(ctx)

	// Item 302 should now belong to player 2, placed in player 2's equipped bag (bag = 500, slot = 0)
	var owner302, bag302, slot302 int64
	err = db.QueryRow("SELECT owner_guid FROM item_instance WHERE guid = 302").Scan(&owner302)
	if err != nil || owner302 != 2 {
		t.Fatalf("expected item 302 transferred to player 2, err=%v, owner=%d", err, owner302)
	}
	err = db.QueryRow("SELECT bag, slot FROM character_inventory WHERE item = 302").Scan(&bag302, &slot302)
	if err != nil || bag302 != 500 || slot302 != 0 {
		t.Fatalf("expected item 302 in bag 500 slot 0, got bag=%d, slot=%d, err=%v", bag302, slot302, err)
	}
}

