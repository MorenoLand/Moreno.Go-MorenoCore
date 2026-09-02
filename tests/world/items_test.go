//go:build ignore

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

