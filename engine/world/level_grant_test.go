package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSlice13Handlers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, level INTEGER, homebind_map INTEGER, homebind_zone INTEGER, homebind_x REAL, homebind_y REAL, homebind_z REAL)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE character_pet (id INTEGER PRIMARY KEY, owner INTEGER, slot INTEGER)",
		"CREATE TABLE bugreport (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT, content TEXT)",
		"INSERT INTO characters VALUES (1, 10000, 10, 0, 0, 0, 0, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store, Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{
		GUID:  1,
		Money: 10000,
		Level: 10,
		Map:   1,
		Zone:  12,
		X:     10.5,
		Y:     20.5,
		Z:     30.5,
	}}
	ctx := context.Background()

	// 1. CMSG_BINDER_ACTIVATE
	bindBuf := protocol.NewBuffer(8)
	bindBuf.WriteU64(999)
	if !sess.handleBinderActivate(ctx, bindBuf.Bytes()) {
		t.Fatal("handleBinderActivate failed")
	}
	if sess.player.HomebindMap != 1 || sess.player.HomebindZone != 12 || sess.player.HomebindX != 10.5 {
		t.Fatalf("homebind mismatch: %+v", sess.player)
	}
	var hbMap int
	_ = db.QueryRowContext(ctx, "SELECT homebind_map FROM characters WHERE guid = 1").Scan(&hbMap)
	if hbMap != 1 {
		t.Fatalf("expected DB homebind_map 1, got %d", hbMap)
	}

	// 2. CMSG_BUY_STABLE_SLOT
	stableBuf := protocol.NewBuffer(8)
	stableBuf.WriteU64(888)
	if !sess.handleBuyStableSlot(ctx, stableBuf.Bytes()) {
		t.Fatal("handleBuyStableSlot failed")
	}
	if sess.player.Money != 9500 { // 10000 - 500 = 9500
		t.Fatalf("expected 9500 money, got %d", sess.player.Money)
	}

	// 3. CMSG_BUYBACK_ITEM
	sess.buyback = []buybackEntry{
		{ItemEntry: 4567, Count: 2, Price: 200},
	}
	buybackBuf := protocol.NewBuffer(12)
	buybackBuf.WriteU64(777)
	buybackBuf.WriteU32(0) // slot 0
	if !sess.handleBuybackItem(ctx, buybackBuf.Bytes()) {
		t.Fatal("handleBuybackItem failed")
	}
	if sess.player.Money != 9300 { // 9500 - 200 = 9300
		t.Fatalf("expected 9300 money, got %d", sess.player.Money)
	}
	if len(sess.buyback) != 0 {
		t.Fatalf("expected buyback empty, got %d", len(sess.buyback))
	}
	var itemEntry int
	_ = db.QueryRowContext(ctx, "SELECT itemEntry FROM item_instance WHERE owner_guid = 1").Scan(&itemEntry)
	if itemEntry != 4567 {
		t.Fatalf("expected item 4567 created, got %d", itemEntry)
	}

	// 4. CMSG_BUG
	bugBuf := protocol.NewBuffer(64)
	bugBuf.WriteU32(0) // bug
	bugBuf.WriteU32(4) // content len
	bugBuf.WriteString("test")
	bugBuf.WriteU32(5) // type len
	bugBuf.WriteString("spell")
	if !sess.handleBug(ctx, bugBuf.Bytes()) {
		t.Fatal("handleBug failed")
	}
	var bugType, bugContent string
	_ = db.QueryRowContext(ctx, "SELECT type, content FROM bugreport LIMIT 1").Scan(&bugType, &bugContent)
	if bugType != "spell" || bugContent != "test" {
		t.Fatalf("expected bugreport (spell, test), got (%s, %s)", bugType, bugContent)
	}

	// 5. CMSG_AUCTION_LIST_PENDING_SALES
	ahBuf := protocol.NewBuffer(8)
	ahBuf.WriteU64(666)
	if !sess.handleAuctionListPendingSales(ctx, ahBuf.Bytes()) {
		t.Fatal("handleAuctionListPendingSales failed")
	}

	// 6. CMSG_ACCEPT_LEVEL_GRANT
	grantBuf := protocol.NewBuffer(8)
	grantBuf.WritePackedGUID(555)
	if !sess.handleAcceptLevelGrant(ctx, grantBuf.Bytes()) {
		t.Fatal("handleAcceptLevelGrant failed")
	}
	if sess.player.Level != 11 {
		t.Fatalf("expected level 11, got %d", sess.player.Level)
	}
}
