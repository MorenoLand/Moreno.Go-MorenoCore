//go:build ignore

package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestAuctionHouseListingSellingAndBidding(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, displayid INTEGER)",
		"CREATE TABLE auctionhouse (id INTEGER PRIMARY KEY, houseid INTEGER, itemguid INTEGER, item_template INTEGER, itemCount INTEGER, itemowner INTEGER, buyoutprice INTEGER, time INTEGER, buyguid INTEGER, lastbid INTEGER, startbid INTEGER, deposit INTEGER)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER, PRIMARY KEY (mail_id, item_guid))",
		"INSERT INTO characters VALUES (1, 'Seller', 10000, '')",
		"INSERT INTO characters VALUES (2, 'Buyer', 50000, '')",
		"INSERT INTO item_template VALUES (1001, 'Iron Sword', 50)",
		"INSERT INTO item_instance VALUES (501, 1001, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 501)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sessSeller := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Seller", Money: 10000}}
	sessBuyer := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Buyer", Money: 50000}}

	ctx := context.Background()

	// 1. Seller lists item 501 on auction
	sellBuf := protocol.NewBuffer(64)
	sellBuf.WriteU64(0)     // auctioneer
	sellBuf.WriteU32(1)     // itemsCount
	sellBuf.WriteU64(501)   // itemGUID
	sellBuf.WriteU32(1)     // count
	sellBuf.WriteU32(1000)  // bid (10 silver)
	sellBuf.WriteU32(5000)  // buyout (50 silver)
	sellBuf.WriteU32(1440)  // duration 24h
	if !sessSeller.handleAuctionSellItem(ctx, sellBuf.Bytes()) {
		t.Fatal("handleAuctionSellItem failed")
	}
	if sessSeller.player.Money != 10000-100 { // 100 deposit deducted
		t.Fatalf("expected seller money 9900, got %d", sessSeller.player.Money)
	}

	// 2. Buyer searches auction items
	listBuf := protocol.NewBuffer(64)
	listBuf.WriteU64(0)
	listBuf.WriteU32(0) // listFrom
	listBuf.WriteCString("Iron")
	if !sessBuyer.handleAuctionListItems(ctx, listBuf.Bytes()) {
		t.Fatal("handleAuctionListItems failed")
	}

	// 3. Buyer buys out auction 1
	bidBuf := protocol.NewBuffer(32)
	bidBuf.WriteU64(0)
	bidBuf.WriteU32(1)    // auction ID
	bidBuf.WriteU32(5000) // price (buyout)
	if !sessBuyer.handleAuctionPlaceBid(ctx, bidBuf.Bytes()) {
		t.Fatal("handleAuctionPlaceBid failed")
	}
	if sessBuyer.player.Money != 50000-5000 {
		t.Fatalf("expected buyer money 45000, got %d", sessBuyer.player.Money)
	}

	// 4. Verify auction is cleared from auctionhouse
	var count int64
	_ = db.QueryRow("SELECT COUNT(*) FROM auctionhouse").Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 active auctions after buyout, got %d", count)
	}
}

