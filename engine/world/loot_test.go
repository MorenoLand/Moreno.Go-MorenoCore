package world

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestLootingMoneyAndItems(t *testing.T) {
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
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER)",
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER, map INTEGER, position_x REAL, position_y REAL, position_z REAL, curhealth INTEGER)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, minGold INTEGER, maxGold INTEGER)",
		"CREATE TABLE creature_loot_template (Entry INTEGER, Item INTEGER, Chance REAL, QuestRequired INTEGER, LootMode INTEGER, GroupId INTEGER, MinCount INTEGER, MaxCount INTEGER)",
		"INSERT INTO characters VALUES (1, 100, '')",
		"INSERT INTO creature VALUES (1, 303, 0, 0, 0, 0, 0)",
		"INSERT INTO creature_template VALUES (303, 50, 50)",
		"INSERT INTO item_template VALUES (7001, 200)",
		"INSERT INTO creature_loot_template VALUES (303, 7001, 100.0, 0, 1, 0, 1, 1)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 100}}

	// 1. Send CMSG_LOOT
	targetGUID := creatureWorldGUID(1, 303)
	lootBuf := protocol.NewBuffer(8)
	lootBuf.WriteU64(targetGUID)
	if !sess.handleLoot(context.Background(), lootBuf.Bytes()) {
		t.Fatal("handleLoot failed")
	}
	if sess.activeLoot == nil || sess.activeLoot.Money != 50 {
		t.Fatalf("expected 50 copper in loot, got %v", sess.activeLoot)
	}

	// 2. Loot money
	if !sess.handleLootMoney(context.Background()) {
		t.Fatal("handleLootMoney failed")
	}
	if sess.player.Money != 150 {
		t.Fatalf("expected 150 money after loot, got %d", sess.player.Money)
	}

	// 3. Loot item from slot 0
	itemBuf := protocol.NewBuffer(1)
	itemBuf.WriteU8(0)
	if !sess.handleAutostoreLootItem(context.Background(), itemBuf.Bytes()) {
		t.Fatal("handleAutostoreLootItem failed")
	}
	var storedItemEntry int64
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 1").Scan(&storedItemEntry)
	if err != nil || storedItemEntry != 7001 {
		t.Fatalf("expected item 7001 stored in inventory, err=%v entry=%d", err, storedItemEntry)
	}

	// 4. Release loot
	releaseBuf := protocol.NewBuffer(8)
	releaseBuf.WriteU64(targetGUID)
	if !sess.handleLootRelease(releaseBuf.Bytes()) {
		t.Fatal("handleLootRelease failed")
	}
	if sess.activeLoot != nil {
		t.Fatal("expected activeLoot to be cleared")
	}
	if len(srv.creatureLoot) != 0 {
		t.Fatal("expected creature loot state to be cleared")
	}
}

func TestLootItemPushResultMatchesReferenceFlags(t *testing.T) {
	reader := protocol.NewReader(buildLootItemPushResult(26, 0, 23, 7001, 2, 5))
	if value, err := reader.ReadU64(); err != nil || value != 26 {
		t.Fatalf("player=%d err=%v", value, err)
	}
	for index, expected := range []uint32{0, 0, 1} {
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("flag %d=%d err=%v", index, value, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("bag=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 23 {
		t.Fatalf("slot=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 7001 {
		t.Fatalf("entry=%d err=%v", value, err)
	}
}

func TestHandleLootRoll(t *testing.T) {
	srv := &Server{
		groups:       make(map[uint64]*groupState),
		creatureLoot: make(map[uint64]*activeLootState),
		sessions:     make(map[*session]struct{}),
		Config:       config.Default(),
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   100,
		groupID:      1,
		player:       &playerState{GUID: 100},
	}
	srv.sessions[sess] = struct{}{}
	srv.groups[1] = &groupState{ID: 1, Members: []groupMember{{GUID: 100}}}

	// Populate loot in creatureLoot
	targetGUID := uint64(500)
	srv.creatureLoot[targetGUID] = &activeLootState{
		TargetGUID: targetGUID,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 12345},
		},
	}

	payload := protocol.NewBuffer(13)
	payload.WriteU64(targetGUID)
	payload.WriteU32(0) // slot 0
	payload.WriteU8(1)  // need roll

	done := make(chan struct{})
	go func() {
		if !sess.handleLootRoll(context.Background(), payload.Bytes()) {
			t.Error("handleLootRoll returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, rollPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if opcode != uint16(protocol.OpcodeSMSG_LOOT_ROLL) {
		t.Fatalf("expected SMSG_LOOT_ROLL (0x2A2), got %x", opcode)
	}
	if len(rollPayload) != 35 {
		t.Fatalf("expected 35 bytes, got %d", len(rollPayload))
	}
	r := protocol.NewReader(rollPayload)
	src, _ := r.ReadU64()
	slot, _ := r.ReadU32()
	player, _ := r.ReadU64()
	entry, _ := r.ReadU32()
	_, _ = r.ReadU32() // suffix
	_, _ = r.ReadU32() // propId
	rollNum, _ := r.ReadU8()
	rollType, _ := r.ReadU8()
	autoPass, _ := r.ReadU8()

	if src != targetGUID || slot != 0 || player != 100 || entry != 12345 || rollNum == 0 || rollNum > 100 || rollType != 1 || autoPass != 0 {
		t.Fatalf("unexpected fields: src=%d slot=%d player=%d entry=%d rollNum=%d rollType=%d autoPass=%d", src, slot, player, entry, rollNum, rollType, autoPass)
	}
}

func TestGameObjectLooting(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE gameobject (guid INTEGER PRIMARY KEY, id INTEGER, map INTEGER, position_x REAL, position_y REAL, position_z REAL)",
		"CREATE TABLE gameobject_template (entry INTEGER PRIMARY KEY, type INTEGER, data1 INTEGER, displayId INTEGER, name TEXT)",
		"CREATE TABLE gameobject_loot_template (Entry INTEGER, Item INTEGER, Chance REAL, QuestRequired INTEGER, LootMode INTEGER, GroupId INTEGER, MinCount INTEGER, MaxCount INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, displayid INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"INSERT INTO characters VALUES (1, 100, '')",
		"INSERT INTO gameobject VALUES (50, 1001, 0, 10.0, 20.0, 30.0)",
		"INSERT INTO gameobject_template VALUES (1001, 3, 2001, 555, 'Solid Chest')",
		"INSERT INTO gameobject_loot_template VALUES (2001, 8888, 100.0, 0, 1, 0, 2, 2)",
		"INSERT INTO item_template VALUES (8888, 777)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: worldStore, CharactersStore: worldStore, creatureLoot: make(map[uint64]*activeLootState)}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Map: 0, X: 11.0, Y: 20.0, Z: 30.0},
	}

	goGUID := uint64(50) | uint64(1001)<<24 | uint64(0xF110)<<48
	payload := protocol.NewBuffer(8)
	payload.WriteU64(goGUID)

	done := make(chan struct{})
	go func() {
		if !sess.handleLoot(context.Background(), payload.Bytes()) {
			t.Error("handleLoot returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if opcode != uint16(protocol.OpcodeSMSG_LOOT_RESPONSE) {
		t.Fatalf("expected SMSG_LOOT_RESPONSE (0x160), got 0x%x", opcode)
	}
	r := protocol.NewReader(data)
	tgt, _ := r.ReadU64()
	if tgt != goGUID {
		t.Fatalf("target mismatch: expected %d, got %d", goGUID, tgt)
	}
	lootType, _ := r.ReadU8()
	if lootType != 1 {
		t.Fatalf("expected lootType 1, got %d", lootType)
	}
	_, _ = r.ReadU32() // money
	count, _ := r.ReadU8()
	if count != 1 {
		t.Fatalf("expected 1 item, got %d", count)
	}
	_, _ = r.ReadU8() // slot
	itemEntry, _ := r.ReadU32()
	if itemEntry != 8888 {
		t.Fatalf("expected itemEntry 8888, got %d", itemEntry)
	}
	itemCount, _ := r.ReadU32()
	if itemCount != 2 {
		t.Fatalf("expected itemCount 2, got %d", itemCount)
	}

	// Now autostore item from slot 0
	itemBuf := protocol.NewBuffer(1)
	itemBuf.WriteU8(0)

	receivedOpcodes := make(chan uint16, 20)
	stopReader := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopReader:
				return
			default:
				_ = clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				op, _, err := readServerFrame(clientConn, nil)
				if err == nil {
					receivedOpcodes <- op
				}
			}
		}
	}()

	if !sess.handleAutostoreLootItem(context.Background(), itemBuf.Bytes()) {
		t.Fatal("handleAutostoreLootItem failed")
	}
	time.Sleep(50 * time.Millisecond)
	close(stopReader)

	gotLootRemoved := false
	gotItemPushResult := false
	for len(receivedOpcodes) > 0 {
		op := <-receivedOpcodes
		if op == uint16(protocol.OpcodeSMSG_LOOT_REMOVED) {
			gotLootRemoved = true
		}
		if op == uint16(protocol.OpcodeSMSG_ITEM_PUSH_RESULT) {
			gotItemPushResult = true
		}
	}

	if !gotLootRemoved {
		t.Fatal("expected SMSG_LOOT_REMOVED")
	}
	if !gotItemPushResult {
		t.Fatal("expected SMSG_ITEM_PUSH_RESULT")
	}

	var storedItemEntry int64
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 1").Scan(&storedItemEntry)
	if err != nil || storedItemEntry != 8888 {
		t.Fatalf("expected item 8888 stored in inventory, err=%v entry=%d", err, storedItemEntry)
	}

	// Release loot
	releaseBuf := protocol.NewBuffer(8)
	releaseBuf.WriteU64(goGUID)
	if !sess.handleLootRelease(releaseBuf.Bytes()) {
		t.Fatal("handleLootRelease failed")
	}
	if sess.activeLoot != nil {
		t.Fatal("expected activeLoot to be cleared")
	}
	if len(srv.creatureLoot) != 0 {
		t.Fatal("expected creatureLoot to be cleared")
	}
}

func TestGroupLootRollState_NeedWon(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"INSERT INTO characters VALUES (10, 100, ''), (20, 100, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		WorldStore:      worldStore,
		CharactersStore: worldStore,
		groups:          make(map[uint64]*groupState),
		creatureLoot:    make(map[uint64]*activeLootState),
		groupRolls:      make(map[string]*activeGroupRoll),
		sessions:        make(map[*session]struct{}),
		Config:          config.Default(),
	}

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerLoaded: true,
		playerGUID:   10,
		groupID:      100,
		player:       &playerState{GUID: 10},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerLoaded: true,
		playerGUID:   20,
		groupID:      100,
		player:       &playerState{GUID: 20},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	srv.groups[100] = &groupState{
		ID:            100,
		LootMethod:    3, // Group Loot
		LootThreshold: 2,
		Members: []groupMember{
			{GUID: 10},
			{GUID: 20},
		},
	}

	targetGUID := uint64(555)
	srv.creatureLoot[targetGUID] = &activeLootState{
		TargetGUID: targetGUID,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 54321, Count: 1},
		},
	}

	opcodes1 := make(chan uint16, 10)
	go func() {
		for {
			_ = cConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(cConn1, nil)
			if err != nil {
				return
			}
			opcodes1 <- op
		}
	}()

	go func() {
		for {
			_ = cConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, _, err := readServerFrame(cConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	srv.startGroupLootRoll(targetGUID, 0, 54321, 1, 0, 100)

	// Sess1 votes NEED (1)
	p1 := protocol.NewBuffer(13)
	p1.WriteU64(targetGUID)
	p1.WriteU32(0)
	p1.WriteU8(1)
	if !sess1.handleLootRoll(context.Background(), p1.Bytes()) {
		t.Fatal("sess1 handleLootRoll failed")
	}

	// Sess2 votes GREED (2)
	p2 := protocol.NewBuffer(13)
	p2.WriteU64(targetGUID)
	p2.WriteU32(0)
	p2.WriteU8(2)
	if !sess2.handleLootRoll(context.Background(), p2.Bytes()) {
		t.Fatal("sess2 handleLootRoll failed")
	}

	time.Sleep(100 * time.Millisecond)

	var gotStart1, gotWon1 bool
	for len(opcodes1) > 0 {
		op := <-opcodes1
		if op == uint16(protocol.OpcodeSMSG_LOOT_START_ROLL) {
			gotStart1 = true
		}
		if op == uint16(protocol.OpcodeSMSG_LOOT_ROLL_WON) {
			gotWon1 = true
		}
	}

	if !gotStart1 {
		t.Fatal("expected SMSG_LOOT_START_ROLL for sess1")
	}
	if !gotWon1 {
		t.Fatal("expected SMSG_LOOT_ROLL_WON for sess1")
	}

	var storedEntry int64
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 10").Scan(&storedEntry)
	if err != nil || storedEntry != 54321 {
		t.Fatalf("expected item 54321 stored for winner 10, got err=%v entry=%d", err, storedEntry)
	}
}

func TestGroupLootRollState_AllPassed(t *testing.T) {
	srv := &Server{
		groups:       make(map[uint64]*groupState),
		creatureLoot: make(map[uint64]*activeLootState),
		groupRolls:   make(map[string]*activeGroupRoll),
		sessions:     make(map[*session]struct{}),
		Config:       config.Default(),
	}

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerLoaded: true,
		playerGUID:   10,
		groupID:      200,
		player:       &playerState{GUID: 10},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerLoaded: true,
		playerGUID:   20,
		groupID:      200,
		player:       &playerState{GUID: 20},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	srv.groups[200] = &groupState{
		ID:            200,
		LootMethod:    3,
		LootThreshold: 2,
		Members:       []groupMember{{GUID: 10}, {GUID: 20}},
	}

	targetGUID := uint64(777)
	srv.creatureLoot[targetGUID] = &activeLootState{
		TargetGUID: targetGUID,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 9999, Count: 1},
		},
	}

	opcodes1 := make(chan uint16, 10)
	go func() {
		for {
			_ = cConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(cConn1, nil)
			if err != nil {
				return
			}
			opcodes1 <- op
		}
	}()

	go func() {
		for {
			_ = cConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, _, err := readServerFrame(cConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	srv.startGroupLootRoll(targetGUID, 0, 9999, 1, 0, 200)

	p1 := protocol.NewBuffer(13)
	p1.WriteU64(targetGUID)
	p1.WriteU32(0)
	p1.WriteU8(0) // pass
	_ = sess1.handleLootRoll(context.Background(), p1.Bytes())

	p2 := protocol.NewBuffer(13)
	p2.WriteU64(targetGUID)
	p2.WriteU32(0)
	p2.WriteU8(0) // pass
	_ = sess2.handleLootRoll(context.Background(), p2.Bytes())

	time.Sleep(100 * time.Millisecond)

	var gotAllPassed bool
	for len(opcodes1) > 0 {
		op := <-opcodes1
		if op == uint16(protocol.OpcodeSMSG_LOOT_ALL_PASSED) {
			gotAllPassed = true
		}
	}
	if !gotAllPassed {
		t.Fatal("expected SMSG_LOOT_ALL_PASSED")
	}
}

func TestGroupMoneySharingParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"INSERT INTO characters VALUES (1, 100, ''), (2, 200, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		WorldStore:      worldStore,
		CharactersStore: charStore,
		groups:          make(map[uint64]*groupState),
		sessions:        make(map[*session]struct{}),
		creatureLoot:    make(map[uint64]*activeLootState),
		Config:          config.Default(),
	}

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerLoaded: true,
		playerGUID:   1,
		groupID:      10,
		player:       &playerState{GUID: 1, Money: 100, Map: 0, X: 0, Y: 0, Z: 0},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerLoaded: true,
		playerGUID:   2,
		groupID:      10,
		player:       &playerState{GUID: 2, Money: 200, Map: 0, X: 10.0, Y: 0, Z: 0},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	srv.groups[10] = &groupState{
		ID:            10,
		LootMethod:    3,
		LootThreshold: 2,
		Members:       []groupMember{{GUID: 1}, {GUID: 2}},
	}

	loot := &activeLootState{
		TargetGUID: 1234,
		Money:      100, // 100 copper
		Items:      make(map[uint8]lootItem),
	}
	sess1.activeLoot = loot

	// Drain frames in background
	p1Packets := make(chan []byte, 5)
	p2Packets := make(chan []byte, 5)
	go func() {
		for {
			_ = cConn1.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			op, data, err := readServerFrame(cConn1, nil)
			if err != nil {
				return
			}
			if op == uint16(protocol.OpcodeSMSG_LOOT_MONEY_NOTIFY) {
				p1Packets <- data
			}
		}
	}()
	go func() {
		for {
			_ = cConn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			op, data, err := readServerFrame(cConn2, nil)
			if err != nil {
				return
			}
			if op == uint16(protocol.OpcodeSMSG_LOOT_MONEY_NOTIFY) {
				p2Packets <- data
			}
		}
	}()

	if !sess1.handleLootMoney(context.Background()) {
		t.Fatal("handleLootMoney failed")
	}

	time.Sleep(50 * time.Millisecond)

	// Both should receive 50 copper (100 / 2)
	if sess1.player.Money != 150 {
		t.Fatalf("expected sess1 money 150, got %d", sess1.player.Money)
	}
	if sess2.player.Money != 250 {
		t.Fatalf("expected sess2 money 250, got %d", sess2.player.Money)
	}

	// Verify SMSG_LOOT_MONEY_NOTIFY has copper=50 and alone=0 ("Your share is...")
	select {
	case d1 := <-p1Packets:
		r := protocol.NewReader(d1)
		copper, _ := r.ReadU32()
		alone, _ := r.ReadU8()
		if copper != 50 || alone != 0 {
			t.Fatalf("sess1 money notify mismatch: copper=%d, alone=%d", copper, alone)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for sess1 money notify")
	}

	select {
	case d2 := <-p2Packets:
		r := protocol.NewReader(d2)
		copper, _ := r.ReadU32()
		alone, _ := r.ReadU8()
		if copper != 50 || alone != 0 {
			t.Fatalf("sess2 money notify mismatch: copper=%d, alone=%d", copper, alone)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for sess2 money notify")
	}
}

func TestRoundRobinLootParity(t *testing.T) {
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
		"INSERT INTO characters VALUES (1, 100, ''), (2, 200, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		WorldStore:      worldStore,
		CharactersStore: charStore,
		groups:          make(map[uint64]*groupState),
		sessions:        make(map[*session]struct{}),
		creatureLoot:    make(map[uint64]*activeLootState),
		Config:          config.Default(),
	}

	sess1 := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1,
		groupID:      10,
		player:       &playerState{GUID: 1, Money: 100, Map: 0, X: 0, Y: 0, Z: 0},
	}
	sess2 := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   2,
		groupID:      10,
		player:       &playerState{GUID: 2, Money: 200, Map: 0, X: 1.0, Y: 0, Z: 0},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	grp := &groupState{
		ID:            10,
		LootMethod:    1, // Round Robin
		LootThreshold: 2,
		LooterGUID:    1, // Player 1's turn
		Members:       []groupMember{{GUID: 1}, {GUID: 2}},
	}
	srv.groups[10] = grp

	targetGUID := uint64(50) | uint64(1001)<<24 | uint64(0xF110)<<48
	loot := &activeLootState{
		TargetGUID:       targetGUID,
		MapID:            0,
		RoundRobinPlayer: 1, // Player 1 is round robin owner
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 5001, Count: 1, Quality: 1},
		},
	}
	sess1.activeLoot = loot
	sess2.activeLoot = loot

	// 1. Player 2 attempts to autostore slot 0 -> should be rejected because Player 1 is round robin owner
	p2Payload := []byte{0}
	sess2.handleAutostoreLootItem(context.Background(), p2Payload)
	if _, ok := loot.Items[0]; !ok {
		t.Fatal("expected item 0 still present in loot, player 2 should not have looted it")
	}

	// 2. Player 1 releases without taking it -> RoundRobinPlayer becomes 0 (open to everyone)
	relBuf := protocol.NewBuffer(8)
	relBuf.WriteU64(targetGUID)
	sess1.handleLootRelease(relBuf.Bytes())
	if loot.RoundRobinPlayer != 0 {
		t.Fatalf("expected RoundRobinPlayer to be cleared, got %d", loot.RoundRobinPlayer)
	}

	// 3. Now Player 2 can autostore it
	sess2.handleAutostoreLootItem(context.Background(), p2Payload)
	if _, ok := loot.Items[0]; ok {
		t.Fatal("expected item 0 to be looted by player 2 now that it was released")
	}
}

func TestMasterLootParity(t *testing.T) {
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
		"INSERT INTO characters VALUES (1, 100, ''), (2, 200, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		WorldStore:      worldStore,
		CharactersStore: charStore,
		groups:          make(map[uint64]*groupState),
		sessions:        make(map[*session]struct{}),
		creatureLoot:    make(map[uint64]*activeLootState),
		Config:          config.Default(),
	}

	sess1 := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1,
		groupID:      10,
		player:       &playerState{GUID: 1, Money: 100, Map: 0, X: 0, Y: 0, Z: 0},
	}
	sess2 := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   2,
		groupID:      10,
		player:       &playerState{GUID: 2, Money: 200, Map: 0, X: 5.0, Y: 0, Z: 0},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	grp := &groupState{
		ID:            10,
		LootMethod:    2, // Master Loot
		MasterLooter:  1, // Player 1 is ML
		LootThreshold: 2,
		Members:       []groupMember{{GUID: 1}, {GUID: 2}},
	}
	srv.groups[10] = grp

	targetGUID := uint64(888)
	loot := &activeLootState{
		TargetGUID: targetGUID,
		MapID:      0,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 19019, Count: 1, Quality: 5}, // Thunderfury (Legendary)
		},
	}
	sess1.activeLoot = loot
	sess2.activeLoot = loot

	// 1. Player 2 cannot autostore ML item
	sess2.handleAutostoreLootItem(context.Background(), []byte{0})
	if _, ok := loot.Items[0]; !ok {
		t.Fatal("expected item 0 still present, player 2 cannot autostore ML item")
	}

	// 2. Player 1 gives item to Player 2 via CMSG_LOOT_MASTER_GIVE
	giveBuf := protocol.NewBuffer(17)
	giveBuf.WriteU64(targetGUID)
	giveBuf.WriteU8(0) // slot 0
	giveBuf.WriteU64(2) // target player 2
	if !sess1.handleLootMasterGive(context.Background(), giveBuf.Bytes()) {
		t.Fatal("handleLootMasterGive failed")
	}

	// Item 0 should now be removed from loot
	if _, ok := loot.Items[0]; ok {
		t.Fatal("expected item 0 removed from loot after master give")
	}

	// Verify Player 2 received Thunderfury in inventory
	var entry int64
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 2").Scan(&entry)
	if err != nil || entry != 19019 {
		t.Fatalf("expected Thunderfury (19019) in Player 2 inventory, got entry=%d err=%v", entry, err)
	}
}

func TestGroupLoot_NeedBeforeGreed_ClassRestriction(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT DEFAULT '', AllowableClass INTEGER DEFAULT -1, DisenchantID INTEGER DEFAULT 0);
		CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER);
		CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, durability INTEGER);
		INSERT INTO item_template (entry, name, AllowableClass, DisenchantID) VALUES (12345, 'Warrior Plate', 1, 0);
	`); err != nil {
		t.Fatal(err)
	}

	worldStore := &database.Store{DB: db}
	charStore := &database.Store{DB: db}

	srv := &Server{
		WorldStore:      worldStore,
		CharactersStore: charStore,
		groups:          make(map[uint64]*groupState),
		creatureLoot:    make(map[uint64]*activeLootState),
		groupRolls:      make(map[string]*activeGroupRoll),
		sessions:        make(map[*session]struct{}),
		Config:          config.Default(),
	}

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerLoaded: true,
		playerGUID:   10,
		groupID:      300,
		player:       &playerState{GUID: 10, Class: 1}, // Warrior
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerLoaded: true,
		playerGUID:   20,
		groupID:      300,
		player:       &playerState{GUID: 20, Class: 8}, // Mage
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	srv.groups[300] = &groupState{
		ID:            300,
		LootMethod:    4, // Need Before Greed
		LootThreshold: 2,
		Members:       []groupMember{{GUID: 10}, {GUID: 20}},
	}

	targetGUID := uint64(999)
	srv.creatureLoot[targetGUID] = &activeLootState{
		TargetGUID: targetGUID,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 12345, Count: 1},
		},
	}

	masks := make(chan uint8, 4)
	go func() {
		for {
			_ = cConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, p, err := readServerFrame(cConn1, nil)
			if err != nil {
				return
			}
			if op == uint16(protocol.OpcodeSMSG_LOOT_START_ROLL) {
				r := protocol.NewReader(p)
				_, _ = r.ReadU64() // source
				_, _ = r.ReadU32() // map
				_, _ = r.ReadU32() // slot
				_, _ = r.ReadU32() // entry
				_, _ = r.ReadU32() // suffix
				_, _ = r.ReadU32() // prop
				_, _ = r.ReadU32() // count
				_, _ = r.ReadU32() // countdown
				mask, _ := r.ReadU8()
				masks <- mask
			}
		}
	}()

	mageMasks := make(chan uint8, 4)
	go func() {
		for {
			_ = cConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, p, err := readServerFrame(cConn2, nil)
			if err != nil {
				return
			}
			if op == uint16(protocol.OpcodeSMSG_LOOT_START_ROLL) {
				r := protocol.NewReader(p)
				_, _ = r.ReadU64() // source
				_, _ = r.ReadU32() // map
				_, _ = r.ReadU32() // slot
				_, _ = r.ReadU32() // entry
				_, _ = r.ReadU32() // suffix
				_, _ = r.ReadU32() // prop
				_, _ = r.ReadU32() // count
				_, _ = r.ReadU32() // countdown
				mask, _ := r.ReadU8()
				mageMasks <- mask
			}
		}
	}()

	srv.startGroupLootRoll(targetGUID, 0, 12345, 1, 0, 300)

	// Verify Warrior mask has NEED (0x02)
	select {
	case m1 := <-masks:
		if m1&rollFlagTypeNeed == 0 {
			t.Fatalf("expected Warrior mask to include rollFlagTypeNeed, got 0x%02X", m1)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for Warrior start roll packet")
	}

	// Verify Mage mask has NEED (0x02) cleared
	select {
	case m2 := <-mageMasks:
		if m2&rollFlagTypeNeed != 0 {
			t.Fatalf("expected Mage mask to NOT include rollFlagTypeNeed under Need Before Greed, got 0x%02X", m2)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for Mage start roll packet")
	}

	// If Mage attempts to roll NEED (1), server should convert to PASS (0)
	pMage := protocol.NewBuffer(13)
	pMage.WriteU64(targetGUID)
	pMage.WriteU32(0)
	pMage.WriteU8(1) // NEED
	sess2.handleLootRoll(context.Background(), pMage.Bytes())

	srv.lootMu.Lock()
	roll := srv.groupRolls[fmt.Sprintf("%d:%d", targetGUID, 0)]
	if roll == nil || roll.Votes[20] != rollPass {
		t.Fatalf("expected Mage vote converted to rollPass, got %v", roll.Votes[20])
	}
	srv.lootMu.Unlock()
}

func TestGroupLoot_Disenchant_LootDelivery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE item_template (entry INTEGER PRIMARY KEY, name TEXT DEFAULT '', AllowableClass INTEGER DEFAULT -1, DisenchantID INTEGER DEFAULT 0, stackable INTEGER DEFAULT 1, ContainerSlots INTEGER DEFAULT 0);
		CREATE TABLE disenchant_loot_template (Entry INTEGER, Item INTEGER, Chance REAL DEFAULT 100, MinCount INTEGER DEFAULT 1, MaxCount INTEGER DEFAULT 1);
		CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot));
		CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT);
		CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT);
		INSERT INTO characters VALUES (10, 100, ''), (20, 100, '');
		INSERT INTO item_template (entry, name, AllowableClass, DisenchantID, stackable, ContainerSlots) VALUES (7777, 'Magic Staff', -1, 50, 1, 0), (10940, 'Strange Dust', -1, 0, 20, 0);
		INSERT INTO disenchant_loot_template (Entry, Item, Chance, MinCount, MaxCount) VALUES (50, 10940, 100, 2, 2);
	`); err != nil {
		t.Fatal(err)
	}

	worldStore := &database.Store{DB: db}
	charStore := &database.Store{DB: db}

	srv := &Server{
		WorldStore:      worldStore,
		CharactersStore: charStore,
		groups:          make(map[uint64]*groupState),
		creatureLoot:    make(map[uint64]*activeLootState),
		groupRolls:      make(map[string]*activeGroupRoll),
		sessions:        make(map[*session]struct{}),
		Config:          config.Default(),
	}

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerLoaded: true,
		playerGUID:   10,
		groupID:      400,
		player:       &playerState{GUID: 10},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerLoaded: true,
		playerGUID:   20,
		groupID:      400,
		player:       &playerState{GUID: 20},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	srv.groups[400] = &groupState{
		ID:            400,
		LootMethod:    3, // Group Loot
		LootThreshold: 2,
		Members:       []groupMember{{GUID: 10}, {GUID: 20}},
	}

	targetGUID := uint64(8888)
	srv.creatureLoot[targetGUID] = &activeLootState{
		TargetGUID: targetGUID,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 7777, Count: 1},
		},
	}

	go func() {
		for {
			_ = cConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, _, err := readServerFrame(cConn1, nil)
			if err != nil {
				return
			}
		}
	}()

	go func() {
		for {
			_ = cConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, _, err := readServerFrame(cConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	srv.startGroupLootRoll(targetGUID, 0, 7777, 1, 0, 400)

	srv.lootMu.Lock()
	roll := srv.groupRolls[fmt.Sprintf("%d:%d", targetGUID, 0)]
	if roll == nil || roll.RollVoteMask&rollFlagTypeDisenchant == 0 {
		srv.lootMu.Unlock()
		t.Fatalf("expected rollVoteMask to include rollFlagTypeDisenchant for disenchantable item")
	}
	srv.lootMu.Unlock()

	// Sess1 votes DISENCHANT (3)
	p1 := protocol.NewBuffer(13)
	p1.WriteU64(targetGUID)
	p1.WriteU32(0)
	p1.WriteU8(rollDisenchant)
	sess1.handleLootRoll(context.Background(), p1.Bytes())

	// Sess2 votes PASS (0)
	p2 := protocol.NewBuffer(13)
	p2.WriteU64(targetGUID)
	p2.WriteU32(0)
	p2.WriteU8(rollPass)
	sess2.handleLootRoll(context.Background(), p2.Bytes())

	time.Sleep(100 * time.Millisecond)

	// Verify winner 10 received Strange Dust (10940) instead of raw Magic Staff (7777)
	var storedItem int64
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 10").Scan(&storedItem)
	if err != nil || storedItem != 10940 {
		t.Fatalf("expected Strange Dust (10940) in winner inventory from disenchant, got err=%v item=%d", err, storedItem)
	}
}

func TestGroupLootRoll_PlayerLeavesEarlyResolvesRoll(t *testing.T) {
	srv := &Server{
		groups:       make(map[uint64]*groupState),
		creatureLoot: make(map[uint64]*activeLootState),
		groupRolls:   make(map[string]*activeGroupRoll),
		sessions:     make(map[*session]struct{}),
		Config:       config.Default(),
	}

	sess1 := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   10,
		groupID:      500,
		player:       &playerState{GUID: 10, Map: 0},
	}
	sess2 := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   20,
		groupID:      500,
		player:       &playerState{GUID: 20, Map: 0},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	srv.groups[500] = &groupState{
		ID:            500,
		LootMethod:    3,
		LootThreshold: 2,
		Members:       []groupMember{{GUID: 10}, {GUID: 20}},
	}

	targetGUID := uint64(999)
	srv.creatureLoot[targetGUID] = &activeLootState{
		TargetGUID: targetGUID,
		Items: map[uint8]lootItem{
			0: {Slot: 0, ItemEntry: 12345, Count: 1},
		},
	}

	srv.startGroupLootRoll(targetGUID, 0, 12345, 1, 0, 500)

	// Sess1 votes NEED (1)
	p1 := protocol.NewBuffer(13)
	p1.WriteU64(targetGUID)
	p1.WriteU32(0)
	p1.WriteU8(1)
	sess1.handleLootRoll(context.Background(), p1.Bytes())

	// Player 20 hasn't voted yet. Roll should still be active in groupRolls.
	rollKey := fmt.Sprintf("%d:%d", targetGUID, 0)
	srv.lootMu.Lock()
	roll := srv.groupRolls[rollKey]
	srv.lootMu.Unlock()
	if roll == nil {
		t.Fatalf("expected active roll before player 20 leaves")
	}

	// Player 20 leaves group (or disconnects)
	srv.onPlayerLeaveGroupRolls(20, 500)

	// Roll should now be resolved immediately because player 20's vote was counted as pass
	srv.lootMu.Lock()
	rollAfter := srv.groupRolls[rollKey]
	srv.lootMu.Unlock()
	if rollAfter != nil {
		t.Fatalf("expected roll to be resolved immediately upon player 20 leaving, but it is still active")
	}
}



