package world

import (
	"context"
	"database/sql"
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


