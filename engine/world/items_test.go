package world

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
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

func TestItemNameQuery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, InventoryType INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO item_template VALUES (1001, 'Iron Helmet', 1)"); err != nil {
		t.Fatal(err)
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: store}
	sess := &session{server: srv, conn: serverConn, authed: true, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	payload := protocol.NewBuffer(12)
	payload.WriteU32(1001)
	payload.WriteU64(0)

	done := make(chan struct{})
	go func() {
		if !sess.handleItemNameQuery(context.Background(), payload.Bytes()) {
			t.Error("handleItemNameQuery returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_ITEM_NAME_QUERY_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	id, err := r.ReadU32()
	if err != nil || id != 1001 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	name, err := r.ReadCString()
	if err != nil || name != "Iron Helmet" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	invType, err := r.ReadU32()
	if err != nil || invType != 1 {
		t.Fatalf("invType=%d err=%v", invType, err)
	}
}

func TestItemTextQuery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemTextId INTEGER)",
		"CREATE TABLE item_text (id INTEGER PRIMARY KEY, text TEXT)",
		"INSERT INTO item_instance VALUES (500, 42)",
		"INSERT INTO item_text VALUES (42, 'Secret Orders')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store}
	sess := &session{server: srv, conn: serverConn, authed: true, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	// Item with text
	payload := protocol.NewBuffer(8)
	payload.WriteU64(500)

	done := make(chan struct{})
	go func() {
		if !sess.handleItemTextQuery(context.Background(), payload.Bytes()) {
			t.Error("handleItemTextQuery returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_ITEM_TEXT_QUERY_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	status, err := r.ReadU8()
	if err != nil || status != 0 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	guid, err := r.ReadU64()
	if err != nil || guid != 500 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
	text, err := r.ReadCString()
	if err != nil || text != "Secret Orders" {
		t.Fatalf("text=%q err=%v", text, err)
	}

	// Item with no text (not found)
	payloadBad := protocol.NewBuffer(8)
	payloadBad.WriteU64(9999)

	done2 := make(chan struct{})
	go func() {
		if !sess.handleItemTextQuery(context.Background(), payloadBad.Bytes()) {
			t.Error("handleItemTextQuery returned false")
		}
		close(done2)
	}()

	opcode2, data2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done2
	if opcode2 != uint16(protocol.OpcodeSMSG_ITEM_TEXT_QUERY_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode2)
	}
	r2 := protocol.NewReader(data2)
	status2, err := r2.ReadU8()
	if err != nil || status2 != 1 {
		t.Fatalf("expected status 1, got %d", status2)
	}
}

func TestItemRefundInfoAndRefund(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, BuyPrice INTEGER)",
		"INSERT INTO item_template VALUES (5001, 2500)",
		"INSERT INTO item_instance VALUES (600, 5001)",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 600)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "test", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, conn: serverConn, authed: true, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 1000}}

	// 1. Refund Info
	infoPayload := protocol.NewBuffer(8)
	infoPayload.WriteU64(600)

	done := make(chan struct{})
	go func() {
		if !sess.handleItemRefundInfo(context.Background(), infoPayload.Bytes()) {
			t.Error("handleItemRefundInfo returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, infoData, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_ITEM_REFUND_INFO_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	ir := protocol.NewReader(infoData)
	iguid, _ := ir.ReadU64()
	cost, _ := ir.ReadU32()
	if iguid != 600 || cost != 2500 {
		t.Fatalf("unexpected refund info: guid=%d cost=%d", iguid, cost)
	}

	// 2. Execute Refund
	refundPayload := protocol.NewBuffer(8)
	refundPayload.WriteU64(600)

	done2 := make(chan struct{})
	go func() {
		if !sess.handleItemRefund(context.Background(), refundPayload.Bytes()) {
			t.Error("handleItemRefund returned false")
		}
		close(done2)
	}()

	var refundData []byte
	for {
		op, data, rerr := readServerFrame(clientConn, nil)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if op == uint16(protocol.OpcodeSMSG_ITEM_REFUND_RESULT) {
			refundData = data
			break
		}
	}
	<-done2
	rr := protocol.NewReader(refundData)
	rguid, _ := rr.ReadU64()
	res, _ := rr.ReadU32()
	if rguid != 600 || res != 0 {
		t.Fatalf("unexpected refund result: guid=%d res=%d", rguid, res)
	}

	// Check money was refunded
	if sess.player.Money != 3500 {
		t.Fatalf("expected 3500 money, got %d", sess.player.Money)
	}

	// 3. Refund non-existent item returns error
	badRefundPayload := protocol.NewBuffer(8)
	badRefundPayload.WriteU64(999)

	done3 := make(chan struct{})
	go func() {
		if !sess.handleItemRefund(context.Background(), badRefundPayload.Bytes()) {
			t.Error("handleItemRefund returned false")
		}
		close(done3)
	}()

	opcode3, badData, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done3
	if opcode3 != uint16(protocol.OpcodeSMSG_ITEM_REFUND_RESULT) {
		t.Fatalf("unexpected opcode %x", opcode3)
	}
	br := protocol.NewReader(badData)
	bguid, _ := br.ReadU64()
	bres, _ := br.ReadU32()
	if bguid != 999 || bres != 10 {
		t.Fatalf("expected error result 10, got %d for guid %d", bres, bguid)
	}
}

func TestItemUseCleanHandling(t *testing.T) {
	sess := &session{playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	buf := protocol.NewBuffer(32)
	buf.WriteU8(0)    // bag
	buf.WriteU8(0)    // slot
	buf.WriteU8(1)    // castCount
	buf.WriteU32(0)   // spellID
	buf.WriteU64(500) // itemGUID
	buf.WriteU32(0)   // glyphIndex
	buf.WriteU8(0)    // castFlags
	buf.WriteU32(0)   // target flags

	if !sess.handleUseItem(context.Background(), buf.Bytes()) {
		t.Fatal("handleUseItem should return true")
	}
}

func TestUseItemValidationAndConsumption(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, class INTEGER)",
		"INSERT INTO characters VALUES (1, '')",
		"INSERT INTO item_template VALUES (1001, 0)",             // Consumable (class 0)
		"INSERT INTO item_instance VALUES (500, 1001, 2)",        // 2 potions
		"INSERT INTO character_inventory VALUES (1, 0, 23, 500)", // in backpack slot 23
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
	sess := &session{server: srv, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Health: 100}}

	// Drain frames in background
	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	// 1. Use consumable item from slot 23
	buf := protocol.NewBuffer(32)
	buf.WriteU8(0)    // bag 0
	buf.WriteU8(23)   // slot 23
	buf.WriteU8(1)    // castCount
	buf.WriteU32(0)   // spellID
	buf.WriteU64(500) // itemGUID
	buf.WriteU32(0)   // glyphIndex
	buf.WriteU8(0)    // castFlags
	buf.WriteU32(0)   // target flags

	if !sess.handleUseItem(context.Background(), buf.Bytes()) {
		t.Fatal("handleUseItem failed")
	}

	// Verify count was decremented to 1
	var count int
	_ = db.QueryRow("SELECT count FROM item_instance WHERE guid = 500").Scan(&count)
	if count != 1 {
		t.Fatalf("expected count 1 after use, got %d", count)
	}

	// 2. Dead player cannot use item
	sess.player.Health = 0
	deadBuf := protocol.NewBuffer(32)
	deadBuf.WriteU8(0)
	deadBuf.WriteU8(23)
	deadBuf.WriteU8(1)
	deadBuf.WriteU32(0)
	deadBuf.WriteU64(500)
	deadBuf.WriteU32(0)
	deadBuf.WriteU8(0)
	deadBuf.WriteU32(0)

	if !sess.handleUseItem(context.Background(), deadBuf.Bytes()) {
		t.Fatal("handleUseItem should return true even when dead")
	}
	// Count remains 1
	_ = db.QueryRow("SELECT count FROM item_instance WHERE guid = 500").Scan(&count)
	if count != 1 {
		t.Fatalf("dead player used item, count changed to %d", count)
	}
}

func TestSyncEquipmentCachePreservesEnchantments(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY(guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, enchantments TEXT)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	// Equipped item at mainhand (slot 15) with enchant 3789 (Berserking)
	_, _ = db.Exec("INSERT INTO characters (guid, equipmentCache) VALUES (1, '')")
	_, _ = db.Exec("INSERT INTO character_inventory (guid, bag, slot, item) VALUES (1, 0, 15, 100)")
	_, _ = db.Exec("INSERT INTO item_instance (guid, itemEntry, enchantments) VALUES (100, 49623, '3789 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0')")

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	sess.syncEquipmentCache(context.Background())

	fields := strings.Fields(sess.player.Equipment)
	if len(fields) < 32 {
		t.Fatalf("expected at least 32 equipment fields, got %d", len(fields))
	}
	// Slot 15: mainhand -> index 15*2 = 30 (entry), index 15*2+1 = 31 (enchant)
	if fields[30] != "49623" {
		t.Fatalf("expected mainhand item 49623, got %s", fields[30])
	}
	if fields[31] != "3789" {
		t.Fatalf("expected mainhand enchant 3789, got %s", fields[31])
	}

	// Verify DB was updated
	var dbCache string
	_ = db.QueryRow("SELECT equipmentCache FROM characters WHERE guid = 1").Scan(&dbCache)
	if !strings.Contains(dbCache, "49623 3789") {
		t.Fatalf("expected db cache to contain '49623 3789', got %s", dbCache)
	}
}

func TestInventoryTwoItemSwapAndEquipParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY(guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER, enchantments TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, InventoryType INTEGER, ContainerSlots INTEGER, BuyPrice INTEGER)",
		"INSERT INTO characters VALUES (1, '')",
		// Item entries:
		"INSERT INTO item_template VALUES (10, 'Ring 1', 11, 0, 100)",
		"INSERT INTO item_template VALUES (11, 'Ring 2', 11, 0, 100)",
		"INSERT INTO item_template VALUES (20, '1H Sword', 13, 0, 200)",
		"INSERT INTO item_template VALUES (21, 'Shield', 14, 0, 150)",
		"INSERT INTO item_template VALUES (22, '2H Axe', 17, 0, 300)",
		"INSERT INTO item_template VALUES (30, 'Small Bag', 18, 6, 50)",
		"INSERT INTO item_template VALUES (40, 'Linen Cloth', 0, 0, 10)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, WorldStore: charStore}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	// 1. Two-item swap in backpack: both slots occupied
	// Slot 23 has item 1001, Slot 24 has item 1002
	_, _ = db.Exec("INSERT INTO item_instance VALUES (1001, 40, 1, ''), (1002, 40, 1, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 23, 1001), (1, 0, 24, 1002)")

	if !sess.handleSwapInvItem(context.Background(), []byte{24, 23}) {
		t.Fatal("handleSwapInvItem failed for occupied slots")
	}
	var s23, s24 int64
	_ = db.QueryRow("SELECT item FROM character_inventory WHERE guid = 1 AND bag = 0 AND slot = 23").Scan(&s23)
	_ = db.QueryRow("SELECT item FROM character_inventory WHERE guid = 1 AND bag = 0 AND slot = 24").Scan(&s24)
	if s23 != 1002 || s24 != 1001 {
		t.Fatalf("expected slots swapped (23->1002, 24->1001), got 23->%d, 24->%d", s23, s24)
	}

	// 2. Equipped bag logic:
	// Insert bag item 2001 (entry 30, invType 18) into slot 19
	// Put item 3001 inside bag 2001 at slot 0
	_, _ = db.Exec("INSERT INTO item_instance VALUES (2001, 30, 1, ''), (3001, 40, 5, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 19, 2001), (1, 2001, 0, 3001)")

	// Attempting to move non-empty bag from slot 19 to slot 25 should fail
	if !sess.handleSwapInvItem(context.Background(), []byte{25, 19}) {
		t.Fatal("expected handleSwapInvItem to return true even on error")
	}
	var bagSlot int
	_ = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 1 AND item = 2001").Scan(&bagSlot)
	if bagSlot != 19 {
		t.Fatalf("non-empty bag should NOT move from slot 19, but got slot %d", bagSlot)
	}

	// Move item 3001 out of bag 19 into backpack slot 25 using handleSwapItem
	if !sess.handleSwapItem(context.Background(), []byte{0, 25, 19, 0}) {
		t.Fatal("handleSwapItem moving out of bag failed")
	}
	var itemBag, itemSlot int
	_ = db.QueryRow("SELECT bag, slot FROM character_inventory WHERE guid = 1 AND item = 3001").Scan(&itemBag, &itemSlot)
	if itemBag != 0 || itemSlot != 25 {
		t.Fatalf("expected item 3001 to be at bag 0 slot 25, got bag %d slot %d", itemBag, itemSlot)
	}

	// Now bag 2001 is empty: move bag 2001 from slot 19 to slot 26
	if !sess.handleSwapInvItem(context.Background(), []byte{26, 19}) {
		t.Fatal("handleSwapInvItem failed for empty bag")
	}
	_ = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 1 AND item = 2001").Scan(&bagSlot)
	if bagSlot != 26 {
		t.Fatalf("expected empty bag to move to slot 26, got %d", bagSlot)
	}

	// 3. Ring auto-equip to secondary slot:
	// Put Ring 1 (item 4001, entry 10) in slot 10 (Finger1)
	// Put Ring 2 (item 4002, entry 11) in backpack slot 27
	_, _ = db.Exec("INSERT INTO item_instance VALUES (4001, 10, 1, ''), (4002, 11, 1, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 10, 4001), (1, 0, 27, 4002)")

	if !sess.handleAutoEquipItem(context.Background(), []byte{0, 27}) {
		t.Fatal("handleAutoEquipItem for ring failed")
	}
	var r1Slot, r2Slot int
	_ = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 1 AND item = 4001").Scan(&r1Slot)
	_ = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 1 AND item = 4002").Scan(&r2Slot)
	if r1Slot != 10 || r2Slot != 11 {
		t.Fatalf("expected Ring 1 at 10 and Ring 2 at 11, got r1=%d, r2=%d", r1Slot, r2Slot)
	}

	// 4. 2-Handed weapon equipping with offhand occupied:
	// Slot 15: 1H Sword (item 5001)
	// Slot 16: Shield (item 5002)
	// Slot 28: 2H Axe (item 5003)
	_, _ = db.Exec("INSERT INTO item_instance VALUES (5001, 20, 1, ''), (5002, 21, 1, ''), (5003, 22, 1, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 15, 5001), (1, 0, 16, 5002), (1, 0, 28, 5003)")

	if !sess.handleAutoEquipItem(context.Background(), []byte{0, 28}) {
		t.Fatal("handleAutoEquipItem for 2H weapon failed")
	}
	var mhItem, ohItem, shieldBag, shieldSlot int64
	_ = db.QueryRow("SELECT COALESCE(item, 0) FROM character_inventory WHERE guid = 1 AND bag = 0 AND slot = 15").Scan(&mhItem)
	_ = db.QueryRow("SELECT COALESCE(item, 0) FROM character_inventory WHERE guid = 1 AND bag = 0 AND slot = 16").Scan(&ohItem)
	_ = db.QueryRow("SELECT bag, slot FROM character_inventory WHERE guid = 1 AND item = 5002").Scan(&shieldBag, &shieldSlot)

	if mhItem != 5003 {
		t.Fatalf("expected 2H Axe (5003) at slot 15, got %d", mhItem)
	}
	if ohItem != 0 {
		t.Fatalf("expected slot 16 (offhand) to be empty after 2H equip, got item %d", ohItem)
	}
	if shieldBag != 0 || shieldSlot < 23 || shieldSlot > 38 {
		t.Fatalf("expected shield (5002) to be unequipped to backpack, got bag %d slot %d", shieldBag, shieldSlot)
	}

	// 5. High GUID masking test for handleAutoEquipItemSlot:
	// Pass raw 64-bit GUID: 0x4000000000000000 | 5001 (1H Sword currently in backpack)
	var swordSlot int
	_ = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 1 AND item = 5001").Scan(&swordSlot)
	rawGuid := uint64(0x4000000000000000) | uint64(5001)
	slotBuf := protocol.NewBuffer(9)
	slotBuf.WriteU64(rawGuid)
	slotBuf.WriteU8(15) // equip to mainhand

	if !sess.handleAutoEquipItemSlot(context.Background(), slotBuf.Bytes()) {
		t.Fatal("handleAutoEquipItemSlot with high GUID failed")
	}
	_ = db.QueryRow("SELECT slot FROM character_inventory WHERE guid = 1 AND item = 5001").Scan(&swordSlot)
	if swordSlot != 15 {
		t.Fatalf("expected 1H sword (5001) equipped to slot 15 via high GUID, got slot %d", swordSlot)
	}
}

func TestHandleSocketGemsParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY(guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER, enchantments TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, InventoryType INTEGER, ContainerSlots INTEGER, BuyPrice INTEGER, socketColor_1 INTEGER, socketColor_2 INTEGER, socketColor_3 INTEGER, socketBonus INTEGER, GemProperties INTEGER)",
		"INSERT INTO characters VALUES (1, '')",
		// Target helm with 3 sockets (Red=2, Yellow=4, Blue=8, Bonus=3333)
		"INSERT INTO item_template VALUES (100, 'Socketed Helm', 1, 0, 100, 2, 4, 8, 3333, 0)",
		// 3 gems
		"INSERT INTO item_template VALUES (201, 'Red Gem', 0, 0, 50, 0, 0, 0, 0, 3001)",
		"INSERT INTO item_template VALUES (202, 'Yellow Gem', 0, 0, 50, 0, 0, 0, 0, 3002)",
		"INSERT INTO item_template VALUES (203, 'Blue Gem', 0, 0, 50, 0, 0, 0, 0, 3003)",
		// Target item in slot 0 (head)
		"INSERT INTO item_instance VALUES (500, 100, 1, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 0, 500)",
		// Gems in backpack (slots 23, 24, 25)
		"INSERT INTO item_instance VALUES (501, 201, 1, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 501)",
		"INSERT INTO item_instance VALUES (502, 202, 1, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 24, 502)",
		"INSERT INTO item_instance VALUES (503, 203, 1, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 25, 503)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, WorldStore: charStore}
	sess := &session{server: srv, conn: s, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	pChan := make(chan []byte, 20)
	opChan := make(chan uint16, 20)
	go func() {
		for {
			op, data, err := readServerFrame(c, nil)
			if err != nil {
				return
			}
			opChan <- op
			pChan <- data
		}
	}()

	// Build CMSG_SOCKET_GEMS: target item 500, gems [501, 502, 503]
	sockBuf := protocol.NewBuffer(32)
	sockBuf.WriteU64(500)
	sockBuf.WriteU64(501)
	sockBuf.WriteU64(502)
	sockBuf.WriteU64(503)

	if !sess.handleSocketGems(context.Background(), sockBuf.Bytes()) {
		t.Fatal("handleSocketGems failed")
	}

	// 1. Verify SMSG_SOCKET_GEMS_RESULT
	var op uint16
	var data []byte
	select {
	case op = <-opChan:
		data = <-pChan
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_SOCKET_GEMS_RESULT")
	}

	if op != uint16(protocol.OpcodeSMSG_SOCKET_GEMS_RESULT) {
		t.Fatalf("expected SMSG_SOCKET_GEMS_RESULT (0x50B), got 0x%04X", op)
	}

	r := protocol.NewReader(data)
	resGUID, _ := r.ReadU64()
	e1, _ := r.ReadU32()
	e2, _ := r.ReadU32()
	e3, _ := r.ReadU32()
	bonus, _ := r.ReadU32()

	if resGUID != 500 || e1 != 3001 || e2 != 3002 || e3 != 3003 || bonus != 3333 {
		t.Fatalf("unexpected SMSG_SOCKET_GEMS_RESULT: guid=%d e1=%d e2=%d e3=%d bonus=%d",
			resGUID, e1, e2, e3, bonus)
	}

	// 2. Verify DB item_instance enchantments string
	var encStr string
	if err := db.QueryRow("SELECT enchantments FROM item_instance WHERE guid = 500").Scan(&encStr); err != nil {
		t.Fatal(err)
	}
	encFields := strings.Fields(encStr)
	if len(encFields) != 36 {
		t.Fatalf("expected 36 enchantment fields, got %d: %s", len(encFields), encStr)
	}
	if encFields[6] != "3001" || encFields[9] != "3002" || encFields[12] != "3003" || encFields[15] != "3333" {
		t.Fatalf("unexpected enchantments in DB: %s", encStr)
	}

	// 3. Verify gems were deleted
	var count int
	_ = db.QueryRow("SELECT count(*) FROM item_instance WHERE guid IN (501, 502, 503)").Scan(&count)
	if count != 0 {
		t.Fatalf("expected socketed gems deleted from item_instance, found %d", count)
	}
	_ = db.QueryRow("SELECT count(*) FROM character_inventory WHERE item IN (501, 502, 503)").Scan(&count)
	if count != 0 {
		t.Fatalf("expected socketed gems deleted from character_inventory, found %d", count)
	}
}

func TestGiftWrappingAndOpeningAndRepairParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY(guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER, flags INTEGER, durability INTEGER, enchantments TEXT)",
		"CREATE TABLE character_gifts (guid INTEGER, item_guid INTEGER PRIMARY KEY, entry INTEGER, flags INTEGER)",
		"CREATE TABLE item_loot_template (Entry INTEGER, Item INTEGER, Chance REAL, MinCount INTEGER, MaxCount INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, InventoryType INTEGER, ContainerSlots INTEGER, BuyPrice INTEGER, MaxDurability INTEGER, displayid INTEGER)",
		"INSERT INTO characters VALUES (1, 10000, '')",
		// Item templates
		"INSERT INTO item_template VALUES (5042, 'Red Ribbon', 0, 0, 5, 0, 10)",
		"INSERT INTO item_template VALUES (5043, 'Red Gift Box', 0, 0, 5, 0, 11)",
		"INSERT INTO item_template VALUES (1234, 'Iron Sword', 13, 0, 50, 80, 20)",
		"INSERT INTO item_template VALUES (5555, 'Thick-shelled Clam', 0, 0, 10, 0, 30)",
		"INSERT INTO item_template VALUES (7777, 'Clam Meat', 0, 0, 5, 0, 40)",
		"INSERT INTO item_template VALUES (8001, 'Iron Shield', 14, 0, 100, 100, 50)",
		"INSERT INTO item_template VALUES (8002, 'Steel Mace', 13, 0, 150, 100, 60)",
		// Item loot
		"INSERT INTO item_loot_template VALUES (5555, 7777, 100.0, 2, 2)",
		// Inventory items
		// 1001: Wrapper in slot 23
		"INSERT INTO item_instance VALUES (1001, 5042, 1, 0, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 1001)",
		// 1002: Sword in slot 24
		"INSERT INTO item_instance VALUES (1002, 1234, 1, 0, 80, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 24, 1002)",
		// 2001: Clam in slot 25
		"INSERT INTO item_instance VALUES (2001, 5555, 1, 0, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 25, 2001)",
		// 3001: Damaged shield (20/100) in slot 26
		"INSERT INTO item_instance VALUES (3001, 8001, 1, 0, 20, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 26, 3001)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		CharactersStore: charStore,
		WorldStore:      charStore,
		creatureLoot:    make(map[uint64]*activeLootState),
	}
	sess := &session{
		server:       srv,
		conn:         s,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:   1,
			Money:  10000,
			Health: 1000,
		},
	}

	pChan := make(chan []byte, 8)
	opChan := make(chan uint16, 8)
	go func() {
		for {
			op, data, err := readServerFrame(c, nil)
			if err != nil {
				return
			}
			opChan <- op
			pChan <- data
		}
	}()

	ctx := context.Background()

	// 1. Gift Wrapping: Wrap item 1002 with wrapper 1001
	wrapPayload := []byte{0, 23, 0, 24}
	if !sess.handleWrapItem(ctx, wrapPayload) {
		t.Fatal("handleWrapItem failed")
	}

	// Verify wrapper deleted
	var wrapperCount int
	_ = db.QueryRow("SELECT count(*) FROM item_instance WHERE guid = 1001").Scan(&wrapperCount)
	if wrapperCount != 0 {
		t.Fatal("expected wrapper item 1001 deleted")
	}

	// Verify target item wrapped: entry changed to 5043, flags has 8
	var wrappedEntry, wrappedFlags uint32
	if err := db.QueryRow("SELECT itemEntry, flags FROM item_instance WHERE guid = 1002").Scan(&wrappedEntry, &wrappedFlags); err != nil {
		t.Fatal(err)
	}
	if wrappedEntry != 5043 || (wrappedFlags&8) == 0 {
		t.Fatalf("expected wrapped item entry 5043 with flag 8, got entry=%d flags=%d", wrappedEntry, wrappedFlags)
	}

	// Verify character_gifts record
	var giftOriginalEntry uint32
	if err := db.QueryRow("SELECT entry FROM character_gifts WHERE item_guid = 1002").Scan(&giftOriginalEntry); err != nil {
		t.Fatal(err)
	}
	if giftOriginalEntry != 1234 {
		t.Fatalf("expected original entry 1234 recorded in character_gifts, got %d", giftOriginalEntry)
	}

	// 2. Unwrapping: Open wrapped item 1002 (slot 24)
	openPayload := []byte{0, 24}
	if !sess.handleOpenItem(ctx, openPayload) {
		t.Fatal("handleOpenItem unwrapping failed")
	}

	// Verify item 1002 restored to 1234
	if err := db.QueryRow("SELECT itemEntry FROM item_instance WHERE guid = 1002").Scan(&wrappedEntry); err != nil {
		t.Fatal(err)
	}
	if wrappedEntry != 1234 {
		t.Fatalf("expected restored entry 1234 after unwrap, got %d", wrappedEntry)
	}

	// Verify character_gifts deleted
	var giftCount int
	_ = db.QueryRow("SELECT count(*) FROM character_gifts WHERE item_guid = 1002").Scan(&giftCount)
	if giftCount != 0 {
		t.Fatal("expected character_gifts record deleted after unwrap")
	}

	// 3. Container Loot: Open clam 2001 (slot 25)
drainLoop:
	for {
		select {
		case <-opChan:
			<-pChan
		default:
			break drainLoop
		}
	}

	openClam := []byte{0, 25}
	if !sess.handleOpenItem(ctx, openClam) {
		t.Fatal("handleOpenItem container failed")
	}

	select {
	case op := <-opChan:
		data := <-pChan
		if op != uint16(protocol.OpcodeSMSG_LOOT_RESPONSE) {
			t.Fatalf("expected SMSG_LOOT_RESPONSE (0x160), got 0x%04X", op)
		}
		r := protocol.NewReader(data)
		lGUID, _ := r.ReadU64()
		lType, _ := r.ReadU8()
		_, _ = r.ReadU32() // gold
		count, _ := r.ReadU8()
		if lGUID != 2001 || lType != 1 || count != 1 {
			t.Fatalf("unexpected loot response: guid=%d type=%d count=%d", lGUID, lType, count)
		}
		_, _ = r.ReadU8() // slot 0
		lootItemEntry, _ := r.ReadU32()
		lootItemCount, _ := r.ReadU32()
		if lootItemEntry != 7777 || lootItemCount != 2 {
			t.Fatalf("expected 2x item 7777 in clam loot, got %dx %d", lootItemCount, lootItemEntry)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_LOOT_RESPONSE for open item")
	}

	// 4. Single Item Repair: Repair shield 3001 (durability 20 -> 100, cost 800)
	repBuf := protocol.NewBuffer(17)
	repBuf.WriteU64(999)  // npcGUID
	repBuf.WriteU64(3001) // itemGUID
	repBuf.WriteU8(0)     // guildBank = 0

	if !sess.handleRepairItem(ctx, repBuf.Bytes()) {
		t.Fatal("handleRepairItem single item failed")
	}
	if sess.player.Money != 9200 {
		t.Fatalf("expected player money 9200 after 800 repair, got %d", sess.player.Money)
	}
	var shieldD uint32
	_ = db.QueryRow("SELECT durability FROM item_instance WHERE guid = 3001").Scan(&shieldD)
	if shieldD != 100 {
		t.Fatalf("expected shield durability 100, got %d", shieldD)
	}

	// 5. All Items Repair:
	// Damage shield back to 60 (cost 400) and add damaged mace 3002 (durability 50/100, cost 500). Total cost 900.
	_, _ = db.Exec("UPDATE item_instance SET durability = 60 WHERE guid = 3001")
	_, _ = db.Exec("INSERT INTO item_instance VALUES (3002, 8002, 1, 0, 50, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 27, 3002)")

	repAllBuf := protocol.NewBuffer(17)
	repAllBuf.WriteU64(999) // npcGUID
	repAllBuf.WriteU64(0)   // itemGUID = 0 (all items)
	repAllBuf.WriteU8(0)    // guildBank = 0

	if !sess.handleRepairItem(ctx, repAllBuf.Bytes()) {
		t.Fatal("handleRepairItem all items failed")
	}
	if sess.player.Money != 8300 {
		t.Fatalf("expected player money 8300 after repair all (9200 - 900), got %d", sess.player.Money)
	}
	var maceD uint32
	_ = db.QueryRow("SELECT durability FROM item_instance WHERE guid = 3001").Scan(&shieldD)
	_ = db.QueryRow("SELECT durability FROM item_instance WHERE guid = 3002").Scan(&maceD)
	if shieldD != 100 || maceD != 100 {
		t.Fatalf("expected both items repaired to 100, got shield=%d mace=%d", shieldD, maceD)
	}
}

func TestInventoryStackMergingAndDespawnParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY(guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER, enchantments TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT, InventoryType INTEGER, ContainerSlots INTEGER, BuyPrice INTEGER, stackable INTEGER)",
		"INSERT INTO characters VALUES (1, 10000, '')",
		"INSERT INTO item_template VALUES (50, 'Silk Cloth', 0, 0, 20, 20)",
		"INSERT INTO item_instance VALUES (101, 50, 5, ''), (102, 50, 10, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 101), (1, 0, 24, 102)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, WorldStore: charStore}
	sess := &session{server: srv, conn: s, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	opChan := make(chan uint16, 50)
	pChan := make(chan []byte, 50)
	go func() {
		for {
			op, data, err := readServerFrame(c, nil)
			if err != nil {
				return
			}
			opChan <- op
			pChan <- data
		}
	}()

	ctx := context.Background()

	// 1. Merge stack from slot 23 (count 5) into slot 24 (count 10)
	// Max stack is 20, so 5 fits entirely into 10 -> new count 15, item 101 deleted and despawned!
	if !sess.handleSwapInvItem(ctx, []byte{24, 23}) {
		t.Fatal("handleSwapInvItem failed for stack merge")
	}

	var count24 int64
	if err := db.QueryRow("SELECT count FROM item_instance WHERE guid = 102").Scan(&count24); err != nil || count24 != 15 {
		t.Fatalf("expected item 102 count 15, got %d, err %v", count24, err)
	}

	var count101 int
	_ = db.QueryRow("SELECT COUNT(1) FROM item_instance WHERE guid = 101").Scan(&count101)
	if count101 != 0 {
		t.Fatalf("expected item 101 deleted from item_instance, count=%d", count101)
	}

	var slot23Count int
	_ = db.QueryRow("SELECT COUNT(1) FROM character_inventory WHERE guid = 1 AND bag = 0 AND slot = 23").Scan(&slot23Count)
	if slot23Count != 0 {
		t.Fatalf("expected slot 23 empty, count=%d", slot23Count)
	}

	// Drain frames to find the despawn packet for item 101
	foundDespawn := false
	expectedGUID := uint64(101) | (uint64(0x4000) << 48)
	drainLoop:
	for {
		select {
		case op := <-opChan:
			data := <-pChan
			if op == uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) {
				// Check if this update packet contains outOfRange for item 101
				r := protocol.NewReader(data)
				blockCount, _ := r.ReadU32()
				for b := uint32(0); b < blockCount; b++ {
					updateType, err := r.ReadU8()
					if err != nil {
						break
					}
					if updateType == protocol.UpdateOutOfRangeObjects {
						numGUIDs, _ := r.ReadU32()
						for g := uint32(0); g < numGUIDs; g++ {
							packed, _ := r.ReadPackedGUID()
							if packed == expectedGUID {
								foundDespawn = true
							}
						}
					}
				}
			}
		case <-time.After(100 * time.Millisecond):
			break drainLoop
		}
	}
	if !foundDespawn {
		t.Fatal("expected despawn packet for merged/destroyed item 101")
	}

	// 2. Partial stack merge:
	// Insert item 103 (count 15) into slot 25
	_, _ = db.Exec("INSERT INTO item_instance VALUES (103, 50, 15, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 25, 103)")

	// Merge slot 24 (count 15) into slot 25 (count 15).
	// Max stack is 20, so free space is 5. Slot 25 should become 20, slot 24 should become 10.
	if !sess.handleSwapInvItem(ctx, []byte{25, 24}) {
		t.Fatal("handleSwapInvItem failed for partial stack merge")
	}

	var count25 int64
	_ = db.QueryRow("SELECT count FROM item_instance WHERE guid = 103").Scan(&count25)
	_ = db.QueryRow("SELECT count FROM item_instance WHERE guid = 102").Scan(&count24)
	if count25 != 20 || count24 != 10 {
		t.Fatalf("expected partial merge: slot 25=20, slot 24=10, got slot25=%d slot24=%d", count25, count24)
	}

	// 3. Destroy item: Destroy item 103 (count 20) in slot 25
	if !sess.handleDestroyItem(ctx, []byte{0, 25, 20}) {
		t.Fatal("handleDestroyItem failed")
	}
	var count103 int
	_ = db.QueryRow("SELECT COUNT(1) FROM item_instance WHERE guid = 103").Scan(&count103)
	if count103 != 0 {
		t.Fatalf("expected item 103 deleted, count=%d", count103)
	}
}

func TestPlayerVisibleEquipmentUpdateParity(t *testing.T) {
	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()

	srv := &Server{}
	sess := &session{
		server:       srv,
		conn:         s,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:      1,
			Equipment: "1234 0 0 0 5678 101", // slot 0 (head) = 1234, slot 2 (shoulders) = 5678 (enchant 101)
		},
	}

	opChan := make(chan uint16, 10)
	pChan := make(chan []byte, 10)
	go func() {
		for {
			op, data, err := readServerFrame(c, nil)
			if err != nil {
				return
			}
			opChan <- op
			pChan <- data
		}
	}()

	sess.sendPlayerUpdate()

	var op uint16
	var data []byte
	select {
	case op = <-opChan:
		data = <-pChan
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for sendPlayerUpdate")
	}

	if op == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		decompressed, err := protocol.DecompressUpdatePayload(data)
		if err != nil {
			t.Fatalf("failed to decompress: %v", err)
		}
		data = decompressed
	} else if op != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) {
		t.Fatalf("expected SMSG_UPDATE_OBJECT or SMSG_COMPRESSED_UPDATE_OBJECT, got 0x%04X", op)
	}

	r := protocol.NewReader(data)
	blockCount, err := r.ReadU32()
	if err != nil || blockCount == 0 {
		t.Fatalf("invalid block count: %v", err)
	}
	updateType, _ := r.ReadU8()
	if updateType != protocol.UpdateValues {
		t.Fatalf("expected UpdateValues (0), got %d", updateType)
	}
	guid, _ := r.ReadPackedGUID()
	if guid != 1 {
		t.Fatalf("expected player GUID 1, got %d", guid)
	}
}

func TestItemCreateBlockTypeMaskAndContainerEnd(t *testing.T) {
	itemGUID := uint64(8) | (uint64(0x4000) << 48)
	// Regular item (Hearthstone entry 6948)
	itemBlock := buildItemCreateBlockForLocationWithDurability(itemGUID, 6948, 1, 1, 1, 0, nil, 0, 0)
	r := protocol.NewReader(itemBlock)
	upType, _ := r.ReadU8()
	if upType != protocol.UpdateCreateObject2 {
		t.Fatalf("expected UpdateCreateObject2 (3), got %d", upType)
	}
	guid, _ := r.ReadPackedGUID()
	if guid != itemGUID {
		t.Fatalf("expected guid %x, got %x", itemGUID, guid)
	}
	typeID, _ := r.ReadU8()
	if typeID != 1 { // TYPEID_ITEM
		t.Fatalf("expected TYPEID_ITEM (1), got %d", typeID)
	}
	flags, _ := r.ReadU16()
	if flags != 0x0010 { // UPDATEFLAG_LOWGUID
		t.Fatalf("expected UPDATEFLAG_LOWGUID (0x0010), got %x", flags)
	}
	lowGUID, _ := r.ReadU32()
	if lowGUID != 8 {
		t.Fatalf("expected lowGUID 8, got %d", lowGUID)
	}
	maskBlocks, _ := r.ReadU8()
	if maskBlocks != 2 { // 64 fields = 2 blocks of 32 bits
		t.Fatalf("expected 2 mask blocks for item (64 fields), got %d", maskBlocks)
	}
	mask := make([]uint32, maskBlocks)
	for i := range mask {
		mask[i], _ = r.ReadU32()
	}
	// Verify OBJECT_FIELD_TYPE (index 2) is set in mask
	if mask[0]&(1<<2) == 0 {
		t.Fatal("expected OBJECT_FIELD_TYPE (index 2) set in mask")
	}
	// Read values until index 2
	var objectType uint32
	for i := 0; i <= 2; i++ {
		if mask[i/32]&(1<<uint(i%32)) != 0 {
			val, _ := r.ReadU32()
			if i == 2 {
				objectType = val
			}
		}
	}
	// TrinityCore: TYPEMASK_OBJECT (1) | TYPEMASK_ITEM (2) = 3
	if objectType != 0x03 {
		t.Fatalf("expected OBJECT_FIELD_TYPE = 0x03 (TYPEMASK_OBJECT | TYPEMASK_ITEM), got 0x%02X", objectType)
	}
	if (objectType & 0x02) == 0 {
		t.Fatal("TYPEMASK_ITEM bit (0x02) NOT SET! Client will reject item!")
	}

	// Container item (4 slots)
	bagGUID := uint64(9) | (uint64(0x4000) << 48)
	bagBlock := buildItemCreateBlockForLocationWithDurability(bagGUID, 4500, 1, 1, 1, 4, map[uint32]uint64{0: itemGUID}, 0, 0)
	br := protocol.NewReader(bagBlock)
	bUpType, _ := br.ReadU8()
	if bUpType != protocol.UpdateCreateObject2 {
		t.Fatalf("expected UpdateCreateObject2 (3), got %d", bUpType)
	}
	bGuid, _ := br.ReadPackedGUID()
	if bGuid != bagGUID {
		t.Fatalf("expected bagGUID %x, got %x", bagGUID, bGuid)
	}
	bTypeID, _ := br.ReadU8()
	if bTypeID != 2 { // TYPEID_CONTAINER
		t.Fatalf("expected TYPEID_CONTAINER (2), got %d", bTypeID)
	}
	_, _ = br.ReadU16()
	_, _ = br.ReadU32()
	bMaskBlocks, _ := br.ReadU8()
	if bMaskBlocks != 5 { // 138 fields = 5 blocks of 32 bits (CONTAINER_END)
		t.Fatalf("expected 5 mask blocks for container (138 fields = CONTAINER_END), got %d", bMaskBlocks)
	}
	bMask := make([]uint32, bMaskBlocks)
	for i := range bMask {
		bMask[i], _ = br.ReadU32()
	}
	var bObjectType uint32
	for i := 0; i <= 2; i++ {
		if bMask[i/32]&(1<<uint(i%32)) != 0 {
			val, _ := br.ReadU32()
			if i == 2 {
				bObjectType = val
			}
		}
	}
	// TrinityCore: TYPEMASK_OBJECT (1) | TYPEMASK_ITEM (2) | TYPEMASK_CONTAINER (4) = 7
	if bObjectType != 0x07 {
		t.Fatalf("expected container OBJECT_FIELD_TYPE = 0x07, got 0x%02X", bObjectType)
	}
}

func TestSendInventoryItemsAtomicDelivery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE characters (guid INTEGER, equipmentCache TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, ContainerSlots INTEGER, MaxDurability INTEGER)",
		"INSERT INTO item_template VALUES (6948, 0, 0)", // Hearthstone
		"INSERT INTO item_instance VALUES (8, 6948, 1, 0, 1, 0, '', 0, '', 0, 0, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 8)", // Hearthstone in backpack slot 23
		"INSERT INTO characters VALUES (1, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup failed on %s: %v", stmt, err)
		}
	}

	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()

	srv := &Server{
		CharactersStore: &database.Store{DB: db},
		WorldStore:      &database.Store{DB: db},
	}
	sess := &session{
		server:       srv,
		conn:         s,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID: 1,
		},
	}

	pktChan := make(chan []byte, 1)
	go func() {
		op, data, rErr := readServerFrame(c, nil)
		if rErr != nil {
			return
		}
		if op == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
			decompressed, dErr := protocol.DecompressUpdatePayload(data)
			if dErr == nil {
				data = decompressed
			}
		}
		pktChan <- data
	}()

	if err := sess.sendInventoryItems(context.Background()); err != nil {
		t.Fatalf("sendInventoryItems failed: %v", err)
	}

	var data []byte
	select {
	case data = <-pktChan:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for inventory update packet")
	}

	r := protocol.NewReader(data)
	blockCount, err := r.ReadU32()
	if err != nil || blockCount < 2 {
		t.Fatalf("expected at least 2 blocks (item create + player values), got count=%d, err=%v", blockCount, err)
	}

	// First block: CreateObject2 for Hearthstone
	upType1, _ := r.ReadU8()
	if upType1 != protocol.UpdateCreateObject2 {
		t.Fatalf("expected first block to be UpdateCreateObject2 (3), got %d", upType1)
	}
	guid1, _ := r.ReadPackedGUID()
	expectedHearthstoneGUID := uint64(8) | (uint64(0x4000) << 48)
	if guid1 != expectedHearthstoneGUID {
		t.Fatalf("expected item GUID %x, got %x", expectedHearthstoneGUID, guid1)
	}
}

func TestHandleSellItemMasksHighGuid(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER)",
		"CREATE TABLE characters (guid INTEGER, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, SellPrice INTEGER)",
		"INSERT INTO item_template VALUES (1001, 50)",
		"INSERT INTO item_instance VALUES (16, 1001, 1)",
		"INSERT INTO character_inventory VALUES (1, 0, 24, 16)",
		"INSERT INTO characters VALUES (1, 100, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup failed on %s: %v", stmt, err)
		}
	}

	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()
	go func() {
		buf := make([]byte, 1024)
		for {
			if _, err := c.Read(buf); err != nil {
				return
			}
		}
	}()

	srv := &Server{
		CharactersStore: &database.Store{DB: db},
		WorldStore:      &database.Store{DB: db},
	}
	sess := &session{
		server:       srv,
		conn:         s,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:  1,
			Money: 100,
		},
	}

	// Client sends itemGUID with high GUID (0x4000000000000010)
	sellBuf := protocol.NewBuffer(17)
	sellBuf.WriteU64(100)                                   // vendorGUID
	sellBuf.WriteU64(uint64(16) | (uint64(0x4000) << 48)) // itemGUID with high GUID
	sellBuf.WriteU8(1)                                     // count

	if !sess.handleSellItem(context.Background(), sellBuf.Bytes()) {
		t.Fatal("handleSellItem failed")
	}

	if sess.player.Money != 150 { // 100 + 50
		t.Fatalf("expected 150 money after sell, got %d", sess.player.Money)
	}
}

func TestStoreOrStackItem_BackpackAndEquippedBags(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER, BuyPrice INTEGER, SellPrice INTEGER, MaxDurability INTEGER, BuyCount INTEGER, stackable INTEGER, ContainerSlots INTEGER)",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO item_template VALUES (100, 1, 10, 5, 0, 1, 20, 0)", // stackable up to 20
		"INSERT INTO item_template VALUES (200, 2, 50, 25, 100, 1, 1, 0)", // non-stackable
		"INSERT INTO item_template VALUES (300, 3, 100, 50, 0, 1, 1, 4)",  // 4-slot bag
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 1000}}
	ctx := context.Background()

	// 1. Store item 100 (count = 5) into empty backpack
	res1, err := sess.storeOrStackItem(ctx, 1, 100, 5)
	if err != nil {
		t.Fatalf("res1 error: %v", err)
	}
	if res1.Slot != 23 || res1.BagKey != 0 || res1.IsStack || res1.NewCount != 5 || res1.InventoryCount != 5 {
		t.Fatalf("unexpected res1: %+v", res1)
	}

	// 2. Store item 100 (count = 3) -> should STACK onto slot 23
	res2, err := sess.storeOrStackItem(ctx, 1, 100, 3)
	if err != nil {
		t.Fatalf("res2 error: %v", err)
	}
	if res2.Slot != 23 || res2.BagKey != 0 || !res2.IsStack || res2.NewCount != 8 || res2.InventoryCount != 8 {
		t.Fatalf("unexpected res2: %+v", res2)
	}
	// Verify only 1 row exists in character_inventory
	var invCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM character_inventory WHERE guid = 1").Scan(&invCount)
	if invCount != 1 {
		t.Fatalf("expected 1 row in inventory, got %d", invCount)
	}

	// 3. Store non-stackable item 200 -> should take slot 24
	res3, err := sess.storeOrStackItem(ctx, 1, 200, 1)
	if err != nil {
		t.Fatalf("res3 error: %v", err)
	}
	if res3.Slot != 24 || res3.BagKey != 0 || res3.IsStack {
		t.Fatalf("unexpected res3: %+v", res3)
	}

	// 4. Fill remaining backpack slots (25..38)
	for s := 25; s <= 38; s++ {
		fakeGUID := int64(1000 + s)
		_, _ = db.Exec("INSERT INTO item_instance VALUES (?, 200, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')", fakeGUID)
		_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, ?, ?)", s, fakeGUID)
	}

	// 5. Equip 4-slot bag 300 into bag slot 19
	bagItemGUID := int64(5000)
	_, _ = db.Exec("INSERT INTO item_instance VALUES (5000, 300, 1, 0, 1, 0, '', 0, '', 0, 0, 0, '')")
	_, _ = db.Exec("INSERT INTO character_inventory VALUES (1, 0, 19, 5000)")

	// 6. Backpack is full (23..38). Store non-stackable item 200:
	// Should go into the equipped bag in slot 19, at container slot 0!
	res4, err := sess.storeOrStackItem(ctx, 1, 200, 1)
	if err != nil {
		t.Fatalf("res4 error: %v", err)
	}
	if res4.BagKey != bagItemGUID || res4.ClientBag != 19 || res4.Slot != 0 || res4.IsStack {
		t.Fatalf("expected item in bag 5000 slot 0, got %+v", res4)
	}

	// 7. Store stackable item 100 (count = 5):
	// Goes into bag 5000 at container slot 1 (since slot 23 in backpack is full at count 8 and item 100 can stack there? Wait! Slot 23 has count 8, max is 20, so it will stack onto slot 23 in backpack!)
	res5, err := sess.storeOrStackItem(ctx, 1, 100, 5)
	if err != nil {
		t.Fatalf("res5 error: %v", err)
	}
	if !res5.IsStack || res5.NewCount != 13 {
		t.Fatalf("expected res5 to stack onto slot 23, got %+v", res5)
	}

	// 8. Now fill slot 23 to max (20)
	_, _ = db.Exec("UPDATE item_instance SET count = 20 WHERE guid = ?", res1.ItemGUID)

	// 9. Store item 100 (count = 4): backpack has no room for item 100, so it goes into bag 5000 container slot 1!
	res6, err := sess.storeOrStackItem(ctx, 1, 100, 4)
	if err != nil {
		t.Fatalf("res6 error: %v", err)
	}
	if res6.BagKey != bagItemGUID || res6.ClientBag != 19 || res6.Slot != 1 || res6.IsStack {
		t.Fatalf("expected item 100 in bag 5000 slot 1, got %+v", res6)
	}

	// 10. Store item 100 (count = 3): stacks onto slot 1 inside bag 5000!
	res7, err := sess.storeOrStackItem(ctx, 1, 100, 3)
	if err != nil {
		t.Fatalf("res7 error: %v", err)
	}
	if res7.BagKey != bagItemGUID || res7.ClientBag != 19 || res7.Slot != 1 || !res7.IsStack || res7.NewCount != 7 {
		t.Fatalf("expected item 100 to stack in bag 5000 slot 1, got %+v", res7)
	}

	// 11. Fill remaining 2 slots in bag 5000 (slots 2 and 3)
	res8, err := sess.storeOrStackItem(ctx, 1, 200, 1)
	if err != nil || res8.Slot != 2 {
		t.Fatalf("res8 failed: %+v err=%v", res8, err)
	}
	res9, err := sess.storeOrStackItem(ctx, 1, 200, 1)
	if err != nil || res9.Slot != 3 {
		t.Fatalf("res9 failed: %+v err=%v", res9, err)
	}

	// 12. Both backpack and equipped bag 5000 are now completely full.
	// Storing another non-stackable item 200 MUST fail with errInventoryFull!
	_, err = sess.storeOrStackItem(ctx, 1, 200, 1)
	if !errors.Is(err, errInventoryFull) {
		t.Fatalf("expected errInventoryFull, got %v", err)
	}
}

func TestStoreOrStackItem_PartialStackOverflow(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER, BuyPrice INTEGER, SellPrice INTEGER, MaxDurability INTEGER, BuyCount INTEGER, stackable INTEGER, ContainerSlots INTEGER)",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO item_template VALUES (100, 1, 10, 5, 0, 1, 20, 0)", // stackable up to 20
		// Put an existing stack with 18 items at slot 23
		"INSERT INTO item_instance VALUES (10, 100, 1, 0, 18, 0, '', 0, '', 0, 0, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 10)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 1000}}
	ctx := context.Background()

	// Store 5 items: 2 should fill slot 23 to 20, remainder 3 should go to slot 24!
	res, err := sess.storeOrStackItem(ctx, 1, 100, 5)
	if err != nil {
		t.Fatalf("storeOrStackItem failed: %v", err)
	}

	// Verify existing stack at slot 23 was capped at 20
	var count23 int
	_ = db.QueryRow("SELECT count FROM item_instance WHERE guid = 10").Scan(&count23)
	if count23 != 20 {
		t.Fatalf("expected count 20 in slot 23, got %d", count23)
	}

	// Verify remainder 3 was stored in slot 24
	if res.Slot != 24 || res.BagKey != 0 || res.NewCount != 3 || res.InventoryCount != 23 {
		t.Fatalf("unexpected res for remainder: %+v", res)
	}
}


