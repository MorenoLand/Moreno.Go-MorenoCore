package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

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
	sellBuf.WriteU64(0)    // auctioneer
	sellBuf.WriteU32(1)    // itemsCount
	sellBuf.WriteU64(501)  // itemGUID
	sellBuf.WriteU32(1)    // count
	sellBuf.WriteU32(1000) // bid (10 silver)
	sellBuf.WriteU32(5000) // buyout (50 silver)
	sellBuf.WriteU32(1440) // duration 24h
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

func TestAuctionCancellationRefundsActiveBidder(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, money INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, played_time INTEGER, text TEXT)",
		"CREATE TABLE auctionhouse (id INTEGER PRIMARY KEY, houseid INTEGER, itemguid INTEGER, item_template INTEGER, itemCount INTEGER, itemowner INTEGER, buyoutprice INTEGER, time INTEGER, buyguid INTEGER, lastbid INTEGER, startbid INTEGER, deposit INTEGER)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER)",
		"INSERT INTO characters VALUES (1, 'Seller', 50000)",
		"INSERT INTO characters VALUES (2, 'Bidder', 40000)",
		"INSERT INTO item_instance VALUES (201, 1234, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		// Active auction 1 owned by 1, with bidder 2 and bid 2500
		"INSERT INTO auctionhouse VALUES (1, 1, 201, 1234, 1, 1, 10000, 9999999999, 2, 2500, 1000, 100)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, sessions: make(map[*session]struct{})}
	sessSeller := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 50000}}
	sessBidder := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Money: 40000}}
	srv.sessions[sessSeller] = struct{}{}
	srv.sessions[sessBidder] = struct{}{}

	ctx := context.Background()

	// Seller cancels auction 1
	cancelBuf := protocol.NewBuffer(16)
	cancelBuf.WriteU64(0)
	cancelBuf.WriteU32(1) // auction ID 1
	if !sessSeller.handleAuctionRemoveItem(ctx, cancelBuf.Bytes()) {
		t.Fatal("handleAuctionRemoveItem failed")
	}

	// Verify auction is deleted
	var count int64
	_ = db.QueryRow("SELECT COUNT(*) FROM auctionhouse WHERE id = 1").Scan(&count)
	if count != 0 {
		t.Fatalf("expected auction 1 deleted, got count %d", count)
	}

	// Verify bidder 2 received refund mail with money = 2500
	var refundMoney int64
	var refundSubject string
	err = db.QueryRow("SELECT money, subject FROM mail WHERE receiver = 2").Scan(&refundMoney, &refundSubject)
	if err != nil {
		t.Fatalf("bidder refund mail not found: %v", err)
	}
	if refundMoney != 2500 {
		t.Fatalf("expected refund money 2500, got %d", refundMoney)
	}
	if refundSubject != "1234:0:4:1:1" {
		t.Fatalf("unexpected subject: %s (expected 1234:0:4:1:1)", refundSubject)
	}

	// Verify seller 1 received item mail
	var itemReceiver int64
	err = db.QueryRow("SELECT receiver FROM mail_items WHERE item_guid = 201").Scan(&itemReceiver)
	if err != nil || itemReceiver != 1 {
		t.Fatalf("seller item return mail not found or receiver mismatch: %v (receiver=%d)", err, itemReceiver)
	}
}

func TestAuctionHousePendingSalesParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, money INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, played_time INTEGER, text TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, displayid INTEGER)",
		"CREATE TABLE auctionhouse (id INTEGER PRIMARY KEY, houseid INTEGER, itemguid INTEGER, item_template INTEGER, itemCount INTEGER, itemowner INTEGER, buyoutprice INTEGER, time INTEGER, buyguid INTEGER, lastbid INTEGER, startbid INTEGER, deposit INTEGER)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER)",
		"INSERT INTO characters VALUES (10, 'ArtisanSeller', 10000)",
		"INSERT INTO characters VALUES (20, 'RichBuyer', 99000)",
		"INSERT INTO item_template VALUES (777, 'Arcanite Reaper', 250)",
		"INSERT INTO item_instance VALUES (999, 777, 10, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (10, 0, 23, 999)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, WorldStore: charStore, sessions: make(map[*session]struct{})}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sessSeller := &session{conn: serverConn, server: srv, playerGUID: 10, playerLoaded: true, player: &playerState{GUID: 10, Name: "ArtisanSeller", Money: 10000}}
	sessBuyer := &session{server: srv, playerGUID: 20, playerLoaded: true, player: &playerState{GUID: 20, Name: "RichBuyer", Money: 99000}}
	srv.sessions[sessSeller] = struct{}{}
	srv.sessions[sessBuyer] = struct{}{}

	ctx := context.Background()

	// Drain frames in background to prevent pipe deadlock
	type packetData struct {
		opcode  uint16
		payload []byte
	}
	pktChan := make(chan packetData, 10)
	go func() {
		for {
			header := make([]byte, 4)
			if _, err := clientConn.Read(header); err != nil {
				return
			}
			sz := int(header[0])<<8 | int(header[1])
			op := uint16(header[2]) | uint16(header[3])<<8
			payload := make([]byte, sz-2)
			if _, err := clientConn.Read(payload); err != nil {
				return
			}
			pktChan <- packetData{opcode: op, payload: payload}
		}
	}()

	// 1. Seller lists item
	sellBuf := protocol.NewBuffer(64)
	sellBuf.WriteU64(0)
	sellBuf.WriteU32(1)
	sellBuf.WriteU64(999)
	sellBuf.WriteU32(1)
	sellBuf.WriteU32(10000)
	sellBuf.WriteU32(50000)
	sellBuf.WriteU32(1440)
	if !sessSeller.handleAuctionSellItem(ctx, sellBuf.Bytes()) {
		t.Fatal("handleAuctionSellItem failed")
	}

	// 2. Buyer buys out item
	bidBuf := protocol.NewBuffer(32)
	bidBuf.WriteU64(0)
	bidBuf.WriteU32(1)
	bidBuf.WriteU32(50000)
	if !sessBuyer.handleAuctionPlaceBid(ctx, bidBuf.Bytes()) {
		t.Fatal("handleAuctionPlaceBid failed")
	}

	// 3. Verify delayed seller profit mail and immediate invoice mail exist
	var profitMailDeliverTime int64
	// TC Consignment cut: 50000 * 5% = 2500 cut. Profit: 50000 + 100 deposit - 2500 = 47600
	err = db.QueryRow("SELECT deliver_time FROM mail WHERE receiver = 10 AND money = 47600").Scan(&profitMailDeliverTime)
	if err != nil {
		t.Fatalf("expected delayed profit mail with 47600 copper: %v", err)
	}
	if profitMailDeliverTime <= time.Now().Unix() {
		t.Fatalf("expected profit mail deliver_time in the future, got %d", profitMailDeliverTime)
	}

	// 4. Seller queries pending sales
	qBuf := protocol.NewBuffer(8)
	qBuf.WriteU64(0)
	if !sessSeller.handleAuctionListPendingSales(ctx, qBuf.Bytes()) {
		t.Fatal("handleAuctionListPendingSales failed")
	}

	var foundPkt bool
	for !foundPkt {
		select {
		case pkt := <-pktChan:
			if pkt.opcode == uint16(protocol.OpcodeSMSG_AUCTION_LIST_PENDING_SALES) {
				foundPkt = true
				r := protocol.NewReader(pkt.payload)
				count, err := r.ReadU32()
				if err != nil || count != 1 {
					t.Fatalf("expected count 1 pending sale, got %d, err %v", count, err)
				}
				itemName, err := r.ReadCString()
				if err != nil || itemName == "" {
					t.Fatalf("expected non-empty itemName, got %q, err %v", itemName, err)
				}
				buyerName, err := r.ReadCString()
				if err != nil || buyerName != "RichBuyer" {
					t.Fatalf("expected buyerName 'RichBuyer', got %q, err %v", buyerName, err)
				}
				bid, err := r.ReadU32()
				if err != nil || bid != 47600 {
					t.Fatalf("expected bid 47600, got %d, err %v", bid, err)
				}
				buyout, err := r.ReadU32()
				if err != nil || buyout != 47600 {
					t.Fatalf("expected buyout 47600, got %d, err %v", buyout, err)
				}
				timeLeft, err := r.ReadF32()
				if err != nil || timeLeft <= 0 {
					t.Fatalf("expected timeLeft > 0, got %f, err %v", timeLeft, err)
				}
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for SMSG_AUCTION_LIST_PENDING_SALES")
		}
	}
}

func TestAuctionHouseTrinityParity(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, displayid INTEGER, class INTEGER, subclass INTEGER, InventoryType INTEGER, Quality INTEGER, RequiredLevel INTEGER, SellPrice INTEGER)",
		"CREATE TABLE auctionhouse (id INTEGER PRIMARY KEY, houseid INTEGER, itemguid INTEGER, item_template INTEGER, itemCount INTEGER, itemowner INTEGER, buyoutprice INTEGER, time INTEGER, buyguid INTEGER, lastbid INTEGER, startbid INTEGER, deposit INTEGER)",
		"CREATE TABLE mail (id INTEGER PRIMARY KEY, messageType INTEGER, stationery INTEGER, mailTemplateId INTEGER, sender INTEGER, receiver INTEGER, subject TEXT, body TEXT, has_items INTEGER, expire_time INTEGER, deliver_time INTEGER, money INTEGER, cod INTEGER, checked INTEGER)",
		"CREATE TABLE mail_items (mail_id INTEGER, item_guid INTEGER, item_template INTEGER, receiver INTEGER, PRIMARY KEY (mail_id, item_guid))",
		"INSERT INTO characters VALUES (1, 'Seller', 100000, '')",
		"INSERT INTO characters VALUES (2, 'BidderA', 200000, '')",
		"INSERT INTO characters VALUES (3, 'BidderB', 300000, '')",
		"INSERT INTO item_template VALUES (101, 'Broadsword', 10, 2, 7, 13, 2, 20, 2000)", // SellPrice 2000 (20s)
		"INSERT INTO item_template VALUES (102, 'Dagger', 11, 2, 15, 13, 1, 10, 0)",       // SellPrice 0 (min deposit 100)
		"INSERT INTO item_instance VALUES (501, 101, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO item_instance VALUES (502, 102, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 501)",
		"INSERT INTO character_inventory VALUES (1, 0, 24, 502)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, WorldStore: charStore, sessions: make(map[*session]struct{})}
	sessSeller := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Seller", Level: 60, Money: 100000}}
	sessBidderA := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "BidderA", Level: 60, Money: 200000}}
	sessBidderB := &session{server: srv, playerGUID: 3, playerLoaded: true, player: &playerState{GUID: 3, Name: "BidderB", Level: 60, Money: 300000}}
	srv.sessions[sessSeller] = struct{}{}
	srv.sessions[sessBidderA] = struct{}{}
	srv.sessions[sessBidderB] = struct{}{}

	ctx := context.Background()

	// 1. Seller lists Broadsword (SellPrice 2000, 24h = timeHr 2)
	// Expected deposit: 2000 * 5% * 2 = 200 copper
	sellBuf := protocol.NewBuffer(64)
	sellBuf.WriteU64(0)
	sellBuf.WriteU32(1)
	sellBuf.WriteU64(501)
	sellBuf.WriteU32(1)
	sellBuf.WriteU32(1000)  // startbid
	sellBuf.WriteU32(10000) // buyout
	sellBuf.WriteU32(1440)  // 24 hours
	if !sessSeller.handleAuctionSellItem(ctx, sellBuf.Bytes()) {
		t.Fatal("handleAuctionSellItem for Broadsword failed")
	}
	if sessSeller.player.Money != 100000-200 {
		t.Fatalf("expected seller money 99800 after 200 copper deposit, got %d", sessSeller.player.Money)
	}

	// 2. Seller tries to bid on own auction -> must fail
	bidBufOwn := protocol.NewBuffer(32)
	bidBufOwn.WriteU64(0)
	bidBufOwn.WriteU32(1)
	bidBufOwn.WriteU32(2000)
	// Calling handleAuctionPlaceBid by seller should not deduct money or succeed
	moneyBefore := sessSeller.player.Money
	_ = sessSeller.handleAuctionPlaceBid(ctx, bidBufOwn.Bytes())
	if sessSeller.player.Money != moneyBefore {
		t.Fatal("seller should not be able to bid on own auction")
	}

	// 3. BidderA bids below minimum increment (startbid is 1000, tries 900)
	bidBufLow := protocol.NewBuffer(32)
	bidBufLow.WriteU64(0)
	bidBufLow.WriteU32(1)
	bidBufLow.WriteU32(900)
	_ = sessBidderA.handleAuctionPlaceBid(ctx, bidBufLow.Bytes())
	if sessBidderA.player.Money != 200000 {
		t.Fatal("bid below startbid should be rejected")
	}

	// 4. BidderA places valid bid of 1000
	bidBufValid := protocol.NewBuffer(32)
	bidBufValid.WriteU64(0)
	bidBufValid.WriteU32(1)
	bidBufValid.WriteU32(1000)
	if !sessBidderA.handleAuctionPlaceBid(ctx, bidBufValid.Bytes()) {
		t.Fatal("handleAuctionPlaceBid for BidderA failed")
	}
	if sessBidderA.player.Money != 200000-1000 {
		t.Fatalf("expected BidderA money 199000, got %d", sessBidderA.player.Money)
	}

	// 5. BidderB tries to outbid with 1020 (below 5% increment of 1000, which is min 1050)
	bidBufUnderInc := protocol.NewBuffer(32)
	bidBufUnderInc.WriteU64(0)
	bidBufUnderInc.WriteU32(1)
	bidBufUnderInc.WriteU32(1020)
	_ = sessBidderB.handleAuctionPlaceBid(ctx, bidBufUnderInc.Bytes())
	if sessBidderB.player.Money != 300000 {
		t.Fatal("bid below minimum 5% increment should be rejected")
	}

	// 6. BidderB places valid outbid of 1200 -> BidderA should be refunded 1000 via mail
	bidBufOutbid := protocol.NewBuffer(32)
	bidBufOutbid.WriteU64(0)
	bidBufOutbid.WriteU32(1)
	bidBufOutbid.WriteU32(1200)
	if !sessBidderB.handleAuctionPlaceBid(ctx, bidBufOutbid.Bytes()) {
		t.Fatal("handleAuctionPlaceBid for BidderB failed")
	}
	if sessBidderB.player.Money != 300000-1200 {
		t.Fatalf("expected BidderB money 298800, got %d", sessBidderB.player.Money)
	}

	// Verify BidderA outbid refund mail exists
	var refundMoney int64
	var refundSubj string
	err = db.QueryRow("SELECT money, subject FROM mail WHERE receiver = 2").Scan(&refundMoney, &refundSubj)
	if err != nil || refundMoney != 1000 {
		t.Fatalf("expected 1000 copper refund mail for BidderA, got %d, err %v", refundMoney, err)
	}
	if refundSubj != "101:0:0:1:1" { // action 0 = AUCTION_OUTBIDDED
		t.Fatalf("expected subject '101:0:0:1:1', got %q", refundSubj)
	}

	// 7. Seller cancels auction with active bid (1200 copper bid)
	// TC Consignment cut: 1200 * 5% = 60 copper deducted from seller!
	// BidderB is refunded 1200 via mail
	sellerMoneyBeforeCancel := sessSeller.player.Money
	cancelBuf := protocol.NewBuffer(16)
	cancelBuf.WriteU64(0)
	cancelBuf.WriteU32(1)
	if !sessSeller.handleAuctionRemoveItem(ctx, cancelBuf.Bytes()) {
		t.Fatal("handleAuctionRemoveItem failed")
	}
	if sessSeller.player.Money != sellerMoneyBeforeCancel-60 {
		t.Fatalf("expected seller to pay 60 copper auction cut on cancel, money %d (was %d)", sessSeller.player.Money, sellerMoneyBeforeCancel)
	}

	// Verify BidderB refund mail exists with action 4 (AUCTION_CANCELLED_TO_BIDDER)
	var bidderBRefund int64
	var bidderBSubj string
	err = db.QueryRow("SELECT money, subject FROM mail WHERE receiver = 3").Scan(&bidderBRefund, &bidderBSubj)
	if err != nil || bidderBRefund != 1200 {
		t.Fatalf("expected BidderB refund 1200, got %d, err %v", bidderBRefund, err)
	}
	if bidderBSubj != "101:0:4:1:1" {
		t.Fatalf("expected subject '101:0:4:1:1', got %q", bidderBSubj)
	}

	// 8. Test expireAuctions routine
	// Setup expired auction 2 without bidder
	now := time.Now().Unix()
	_, _ = db.Exec("INSERT INTO auctionhouse VALUES (2, 1, 502, 102, 1, 1, 5000, ?, 0, 0, 500, 100)", now-10)
	sessSeller.expireAuctions(ctx)

	// Verify auction 2 deleted and item 502 returned to seller via mail with AUCTION_EXPIRED (3)
	var expCount int64
	_ = db.QueryRow("SELECT COUNT(*) FROM auctionhouse WHERE id = 2").Scan(&expCount)
	if expCount != 0 {
		t.Fatal("expired auction 2 should be deleted")
	}
	var expSubj string
	err = db.QueryRow("SELECT m.subject FROM mail AS m JOIN mail_items AS mi ON mi.mail_id = m.id WHERE mi.item_guid = 502").Scan(&expSubj)
	if err != nil || expSubj != "102:0:3:2:1" {
		t.Fatalf("expected expired return mail subject '102:0:3:2:1', got %q, err %v", expSubj, err)
	}
}

