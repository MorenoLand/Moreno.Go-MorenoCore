package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestEquipmentSetSaveAndDelete(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	schema := `CREATE TABLE character_equipmentsets (
		guid INTEGER, setguid INTEGER, setindex INTEGER, name TEXT, iconname TEXT, ignore_mask INTEGER,
		item0 INTEGER, item1 INTEGER, item2 INTEGER, item3 INTEGER, item4 INTEGER, item5 INTEGER,
		item6 INTEGER, item7 INTEGER, item8 INTEGER, item9 INTEGER, item10 INTEGER, item11 INTEGER,
		item12 INTEGER, item13 INTEGER, item14 INTEGER, item15 INTEGER, item16 INTEGER, item17 INTEGER, item18 INTEGER,
		PRIMARY KEY (setguid)
	)`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	// Build CMSG_EQUIPMENT_SET_SAVE
	buf := protocol.NewBuffer(128)
	buf.WritePackedGUID(100)        // setGuid
	buf.WriteU32(0)                 // setindex
	buf.WriteCString("PvP Gear")    // name
	buf.WriteCString("INV_Helm_01") // iconName
	buf.WritePackedGUID(501)        // item0: guid 501
	buf.WritePackedGUID(1)          // item1: ignored (1)
	for i := 2; i < 19; i++ {
		buf.WritePackedGUID(0) // empty slots
	}

	if !sess.handleEquipmentSetSave(context.Background(), buf.Bytes()) {
		t.Fatal("handleEquipmentSetSave failed")
	}

	// Verify persistence in DB
	var name, icon string
	var setindex, ignoreMask, item0 uint32
	err = db.QueryRow("SELECT setindex, name, iconname, ignore_mask, item0 FROM character_equipmentsets WHERE guid = 1 AND setguid = 100").Scan(&setindex, &name, &icon, &ignoreMask, &item0)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if name != "PvP Gear" || icon != "INV_Helm_01" || setindex != 0 || ignoreMask != 2 || item0 != 501 {
		t.Fatalf("unexpected set row: name=%s icon=%s idx=%d mask=%d item0=%d", name, icon, setindex, ignoreMask, item0)
	}

	// Test delete
	delBuf := protocol.NewBuffer(16)
	delBuf.WritePackedGUID(100)
	if !sess.handleEquipmentSetDelete(context.Background(), delBuf.Bytes()) {
		t.Fatal("handleEquipmentSetDelete failed")
	}

	var count int
	err = db.QueryRow("SELECT COUNT(1) FROM character_equipmentsets WHERE setguid = 100").Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("expected 0 rows after delete, got %d err=%v", count, err)
	}
}

func TestEquipmentSetUse(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1},
	}

	buf := protocol.NewBuffer(128)
	for i := 0; i < 19; i++ {
		buf.WritePackedGUID(0) // itemGuid 0
		buf.WriteU8(0)         // bag
		buf.WriteU8(0)         // slot
	}

	done := make(chan struct{})
	go func() {
		if !sess.handleEquipmentSetUse(context.Background(), buf.Bytes()) {
			t.Error("handleEquipmentSetUse returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_EQUIPMENT_SET_USE_RESULT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	res, err := r.ReadU8()
	if err != nil || res != 0 {
		t.Fatalf("result=%d err=%v", res, err)
	}
}

func TestEquipmentSetListLoginAndSavedNotification(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	schema := `CREATE TABLE character_equipmentsets (
		guid INTEGER, setguid INTEGER, setindex INTEGER, name TEXT, iconname TEXT, ignore_mask INTEGER,
		item0 INTEGER, item1 INTEGER, item2 INTEGER, item3 INTEGER, item4 INTEGER, item5 INTEGER,
		item6 INTEGER, item7 INTEGER, item8 INTEGER, item9 INTEGER, item10 INTEGER, item11 INTEGER,
		item12 INTEGER, item13 INTEGER, item14 INTEGER, item15 INTEGER, item16 INTEGER, item17 INTEGER, item18 INTEGER,
		PRIMARY KEY (setguid)
	)`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store}
	sess := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   1,
		player:       &playerState{GUID: 1},
	}

	// 1. Test handleEquipmentSetSave sends SMSG_EQUIPMENT_SET_SAVED
	buf := protocol.NewBuffer(128)
	buf.WritePackedGUID(200)          // setGuid
	buf.WriteU32(1)                   // setindex
	buf.WriteCString("Raid Tank")     // name
	buf.WriteCString("INV_Shield_05") // iconName
	buf.WritePackedGUID(601)          // item0
	buf.WritePackedGUID(1)            // item1: ignored
	for i := 2; i < 19; i++ {
		buf.WritePackedGUID(0)
	}

	done := make(chan struct{})
	go func() {
		if !sess.handleEquipmentSetSave(context.Background(), buf.Bytes()) {
			t.Error("handleEquipmentSetSave failed")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op1, data1, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if op1 != uint16(protocol.OpcodeSMSG_EQUIPMENT_SET_SAVED) {
		t.Fatalf("expected SMSG_EQUIPMENT_SET_SAVED (0x137), got 0x%x", op1)
	}
	r1 := protocol.NewReader(data1)
	idx, err := r1.ReadU32()
	if err != nil || idx != 1 {
		t.Fatalf("expected set index 1, got %d err=%v", idx, err)
	}

	// 2. Test sendEquipmentSetList sends SMSG_EQUIPMENT_SET_LIST
	done2 := make(chan struct{})
	go func() {
		sess.sendEquipmentSetList(context.Background())
		close(done2)
	}()

	op2, data2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done2
	if op2 != uint16(protocol.OpcodeSMSG_EQUIPMENT_SET_LIST) {
		t.Fatalf("expected SMSG_EQUIPMENT_SET_LIST (0x4BC), got 0x%x", op2)
	}
	r2 := protocol.NewReader(data2)
	count, err := r2.ReadU32()
	if err != nil || count != 1 {
		t.Fatalf("expected 1 set in list, got %d err=%v", count, err)
	}
	setGuid, _ := r2.ReadPackedGUID()
	if setGuid != 200 {
		t.Fatalf("expected setGuid 200, got %d", setGuid)
	}
	setIdx, _ := r2.ReadU32()
	if setIdx != 1 {
		t.Fatalf("expected setIdx 1, got %d", setIdx)
	}
	setName, _ := r2.ReadCString()
	if setName != "Raid Tank" {
		t.Fatalf("expected setName 'Raid Tank', got %s", setName)
	}
}
