package world

import (
	"context"
	"database/sql"
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
