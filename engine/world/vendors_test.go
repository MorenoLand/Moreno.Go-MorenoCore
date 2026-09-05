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

func TestVendorBuybackFullParityAndFields(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER, BuyPrice INTEGER, SellPrice INTEGER, MaxDurability INTEGER, BuyCount INTEGER, ContainerSlots INTEGER)",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO item_template VALUES (5001, 100, 50, 25, 100, 1, 0)",
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, count, durability) VALUES (500, 5001, 1, 1, 100)",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 500)",
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
	sess := &session{
		conn:         serverConn,
		server:       srv,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:  1,
			Money: 1000,
		},
	}

	vendorGUID := creatureWorldGUID(1, 101)

	type serverPkt struct {
		opcode uint16
		data   []byte
	}
	receivedPkts := make(chan serverPkt, 64)
	go func() {
		for {
			op, data, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedPkts <- serverPkt{opcode: op, data: data}
		}
	}()

	// 1. Sell item (guid 500) to vendor
	sellBuf := protocol.NewBuffer(17)
	sellBuf.WriteU64(vendorGUID)
	sellBuf.WriteU64(500)
	sellBuf.WriteU8(1)

	done := make(chan bool, 1)
	go func() {
		done <- sess.handleSellItem(context.Background(), sellBuf.Bytes())
	}()

	// Read SMSG_SELL_ITEM
	pkt1 := <-receivedPkts
	if pkt1.opcode != uint16(protocol.OpcodeSMSG_SELL_ITEM) {
		t.Fatalf("expected SMSG_SELL_ITEM (0x1A1), got op=%x", pkt1.opcode)
	}

	// Read SMSG_UPDATE_OBJECT from sendInventoryItems
	pkt2 := <-receivedPkts
	if pkt2.opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && pkt2.opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("expected update object, got op=%x", pkt2.opcode)
	}

	// Read SMSG_UPDATE_OBJECT from sendPlayerUpdate
	pkt3 := <-receivedPkts
	if pkt3.opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && pkt3.opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("expected player update object, got op=%x", pkt3.opcode)
	}
	<-done

	// Verify player earned 25 copper: 1000 + 25 = 1025
	if sess.player.Money != 1025 {
		t.Fatalf("expected 1025 money after sell, got %d", sess.player.Money)
	}

	// Verify buyback slot 0 is populated
	bb0 := sess.buyback[0]
	if bb0 == nil {
		t.Fatal("expected buyback[0] to be populated")
	}
	expectedFullGUID := uint64(500) | (uint64(0x4000) << 48)
	if bb0.ItemGUID != expectedFullGUID {
		t.Fatalf("expected buyback GUID %x, got %x", expectedFullGUID, bb0.ItemGUID)
	}
	if bb0.ItemEntry != 5001 || bb0.Count != 1 || bb0.Price != 25 {
		t.Fatalf("unexpected buyback data: %+v", bb0)
	}
	if bb0.Timestamp == 0 {
		t.Fatal("expected non-zero buyback timestamp")
	}

	// Verify character_inventory slot 23 is now empty
	var invCount int
	_ = db.QueryRow("SELECT COUNT(1) FROM character_inventory WHERE guid = 1 AND slot = 23").Scan(&invCount)
	if invCount != 0 {
		t.Fatalf("expected slot 23 cleared in character_inventory")
	}

	// 2. Buyback item using retail slot 74 (BUYBACK_SLOT_START)
	buybackBuf := protocol.NewBuffer(12)
	buybackBuf.WriteU64(vendorGUID)
	buybackBuf.WriteU32(74) // slot 74

	buyDone := make(chan bool, 1)
	go func() {
		buyDone <- sess.handleBuybackItem(context.Background(), buybackBuf.Bytes())
	}()

	// Read SMSG_BUY_ITEM (skipping any destroy/despawn of the old buyback object)
	var pktBuy serverPkt
	for {
		pkt := <-receivedPkts
		if pkt.opcode == uint16(protocol.OpcodeSMSG_BUY_ITEM) {
			pktBuy = pkt
			break
		}
	}
	if pktBuy.opcode != uint16(protocol.OpcodeSMSG_BUY_ITEM) {
		t.Fatalf("expected SMSG_BUY_ITEM (0x1A2), got op=%x", pktBuy.opcode)
	}

	<-buyDone

	// Verify money deducted back to 1000
	if sess.player.Money != 1000 {
		t.Fatalf("expected 1000 money after buyback, got %d", sess.player.Money)
	}

	// Verify buyback slot 0 is now empty
	if sess.buyback[0] != nil {
		t.Fatalf("expected buyback[0] nil after buyback, got %+v", sess.buyback[0])
	}

	// Verify item is restored to character_inventory
	var restoredEntry int
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 1 AND ci.bag = 0 AND ci.slot = 23").Scan(&restoredEntry)
	if err != nil || restoredEntry != 5001 {
		t.Fatalf("expected item 5001 restored to inventory, got err=%v entry=%d", err, restoredEntry)
	}
}
