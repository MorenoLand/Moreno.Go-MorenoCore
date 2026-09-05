package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCreatureGossipLuaHookWritesClientMenu(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, modelid INTEGER NOT NULL, curhealth INTEGER NOT NULL)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, name TEXT NOT NULL, modelid1 INTEGER NOT NULL, maxlevel INTEGER NOT NULL, gossip_menu_id INTEGER NOT NULL, npcflag INTEGER NOT NULL)",
		"INSERT INTO creature VALUES (321, 68, 0, 100)",
		"INSERT INTO creature_template VALUES (68, 'Stormwind Guard', 3167, 80, 0, 1)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	runtime := scripting.NewRuntime(scripting.Config{Enabled: true})
	if err := runtime.LoadString(`RegisterCreatureGossipEvent(68, 1, function(event, player, creature) player:GossipMenuAddItem(3, "Talk", 0, 7, nil, "Confirm", 5); player:GossipSendMenu(1, creature) end); RegisterCreatureGossipEvent(68, 2, function(event, player, creature, sender, action) if sender == 0 and action == 7 then player:GossipComplete() end end)`); err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, Config: config.Default(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Features: &Features{Scripts: runtime}}
	guid := uint64(321) | uint64(68)<<24 | uint64(0xF130)<<48
	state := &session{server: server, conn: serverConn, accountName: "TEST", playerLoaded: true, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0}, auras: make(map[uint32]struct{})}
	hello := protocol.NewBuffer(8)
	hello.WriteU64(guid)
	done := make(chan bool, 1)
	go func() { done <- state.handleGossipHello(context.Background(), hello.Bytes()) }()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_GOSSIP_MESSAGE) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	if value, err := reader.ReadU64(); err != nil || value != guid {
		t.Fatalf("guid=%x err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("menu=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 1 {
		t.Fatalf("title=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 1 {
		t.Fatalf("items=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("item id=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 3 {
		t.Fatalf("icon=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("coded=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 5 {
		t.Fatalf("box money=%d err=%v", value, err)
	}
	for _, expected := range []string{"Talk", "Confirm"} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if questCount, err := reader.ReadU32(); err != nil || questCount != 0 {
		t.Fatalf("quest count=%d err=%v", questCount, err)
	}
	if result := <-done; !result {
		t.Fatalf("gossip hello failed")
	}
	selectPayload := protocol.NewBuffer(16)
	selectPayload.WriteU64(guid)
	selectPayload.WriteU32(0)
	selectPayload.WriteU32(0)
	selectDone := make(chan bool, 1)
	go func() { selectDone <- state.handleGossipSelectOption(context.Background(), selectPayload.Bytes()) }()
	opcode, payload, err = readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_GOSSIP_COMPLETE) || len(payload) != 0 {
		t.Fatalf("close opcode=%x payload=%x", opcode, payload)
	}
	if result := <-selectDone; !result {
		t.Fatalf("gossip selection failed")
	}
}

func TestPrepareCreatureGossipLoadsDatabaseOptions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE gossip_menu (MenuID INTEGER NOT NULL, TextID INTEGER NOT NULL)",
		"CREATE TABLE gossip_menu_option (MenuID INTEGER NOT NULL, OptionID INTEGER NOT NULL, OptionIcon INTEGER NOT NULL, OptionText TEXT, OptionType INTEGER NOT NULL, OptionNpcFlag INTEGER NOT NULL, ActionMenuID INTEGER NOT NULL, ActionPoiID INTEGER NOT NULL, BoxCoded INTEGER NOT NULL, BoxMoney INTEGER NOT NULL, BoxText TEXT)",
		"INSERT INTO gossip_menu VALUES (7, 123)",
		"INSERT INTO gossip_menu_option VALUES (7, 2, 3, 'Open', 1, 1, 8, 9, 0, 10, '')",
		"INSERT INTO gossip_menu_option VALUES (7, 3, 4, 'Hidden', 1, 2, 0, 0, 0, 0, '')",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}}
	state := &session{server: server}
	menu, err := state.prepareCreatureGossip(context.Background(), 99, 68, 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if menu.TitleID != 123 || menu.MenuID != 7 || len(menu.Items) != 1 {
		t.Fatalf("menu=%+v", menu)
	}
	item, ok := menu.Items[2]
	if !ok || item.Message != "Open" || item.Action != 1 || item.ActionMenuID != 8 || item.ActionPoiID != 9 || item.BoxMoney != 10 {
		t.Fatalf("item=%+v", item)
	}
}

func TestGossipSpecialOptionsAndServices(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, modelid INTEGER NOT NULL, curhealth INTEGER NOT NULL)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, name TEXT NOT NULL, modelid1 INTEGER NOT NULL, maxlevel INTEGER NOT NULL, gossip_menu_id INTEGER NOT NULL, npcflag INTEGER NOT NULL)",
		"INSERT INTO creature VALUES (501, 101, 0, 100)",
		"INSERT INTO creature_template VALUES (101, 'Banker Bob', 1, 80, 0, 131072)", // 131072 = 0x20000 = UNIT_NPC_FLAG_BANKER
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: store, Config: config.Default()}
	guid := uint64(501) | uint64(101)<<24 | uint64(0xF130)<<48
	sess := &session{server: srv, conn: serverConn, accountName: "TEST", playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. Banker single-service auto opens bank
	hBuf := protocol.NewBuffer(8)
	hBuf.WriteU64(guid)
	go func() {
		sess.handleGossipHello(ctx, hBuf.Bytes())
	}()

	op, data, err := readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_SHOW_BANK) {
		t.Fatalf("expected SMSG_SHOW_BANK (0x%x), got op=0x%x err=%v", protocol.OpcodeSMSG_SHOW_BANK, op, err)
	}
	r := protocol.NewReader(data)
	if bGuid, _ := r.ReadU64(); bGuid != guid {
		t.Fatalf("expected banker guid %x, got %x", guid, bGuid)
	}

	// 2. Select Option 16 (unlearn talents) sends MSG_TALENT_WIPE_CONFIRM
	sess.gossip = &gossipMenuState{
		SenderGUID: guid,
		MenuID:     1,
		Items: map[uint32]gossipMenuItem{
			0: {Action: 16}, // GOSSIP_OPTION_UNLEARNTALENTS
		},
	}
	selBuf := protocol.NewBuffer(16)
	selBuf.WriteU64(guid)
	selBuf.WriteU32(1) // menu 1
	selBuf.WriteU32(0) // list 0

	go func() {
		sess.handleGossipSelectOption(ctx, selBuf.Bytes())
	}()

	op, data, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeMSG_TALENT_WIPE_CONFIRM) {
		t.Fatalf("expected MSG_TALENT_WIPE_CONFIRM (0x%x), got op=0x%x err=%v", protocol.OpcodeMSG_TALENT_WIPE_CONFIRM, op, err)
	}
	r = protocol.NewReader(data)
	wGuid, _ := r.ReadU64()
	cost, _ := r.ReadU32()
	if wGuid != guid || cost != 100000 {
		t.Fatalf("expected wipe guid %x cost 100000, got guid %x cost %d", guid, wGuid, cost)
	}
}

func TestBinderActivateParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, homebind_map INTEGER, homebind_zone INTEGER, homebind_x REAL, homebind_y REAL, homebind_z REAL)",
		"INSERT INTO characters VALUES (1, 0, 0, 0.0, 0.0, 0.0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store, Config: config.Default()}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1,
		player: &playerState{
			GUID: 1,
			Map:  0,
			Zone: 1519,
			X:    -8867.0,
			Y:    673.0,
			Z:    97.0,
		},
	}

	binderGUID := uint64(55555) | (uint64(0xF130) << 48)
	bindBuf := protocol.NewBuffer(8)
	bindBuf.WriteU64(binderGUID)

	go func() {
		sess.handleBinderActivate(context.Background(), bindBuf.Bytes())
	}()

	// 1. SMSG_SPELL_GO (spell 3286)
	op1, _, err := readServerFrame(clientConn, nil)
	if err != nil || op1 != uint16(protocol.OpcodeSMSG_SPELL_GO) {
		t.Fatalf("expected SMSG_SPELL_GO (0x%x), got op=0x%x err=%v", protocol.OpcodeSMSG_SPELL_GO, op1, err)
	}

	// 2. SMSG_BIND_POINT_UPDATE (20 bytes: X, Y, Z, Map, Area)
	op2, data2, err := readServerFrame(clientConn, nil)
	if err != nil || op2 != uint16(protocol.OpcodeSMSG_BIND_POINT_UPDATE) || len(data2) != 20 {
		t.Fatalf("expected SMSG_BIND_POINT_UPDATE (0x%x) len 20, got op=0x%x len=%d err=%v", protocol.OpcodeSMSG_BIND_POINT_UPDATE, op2, len(data2), err)
	}

	// 3. SMSG_PLAYER_BOUND (12 bytes: BinderGUID, AreaID)
	op3, data3, err := readServerFrame(clientConn, nil)
	if err != nil || op3 != uint16(protocol.OpcodeSMSG_PLAYER_BOUND) || len(data3) != 12 {
		t.Fatalf("expected SMSG_PLAYER_BOUND (0x%x) len 12, got op=0x%x len=%d err=%v", protocol.OpcodeSMSG_PLAYER_BOUND, op3, len(data3), err)
	}
	r3 := protocol.NewReader(data3)
	bg, _ := r3.ReadU64()
	area, _ := r3.ReadU32()
	if bg != binderGUID || area != 1519 {
		t.Fatalf("expected binder %x area 1519, got binder %x area %d", binderGUID, bg, area)
	}

	// 4. SMSG_TRAINER_BUY_SUCCEEDED (12 bytes)
	op4, data4, err := readServerFrame(clientConn, nil)
	if err != nil || op4 != uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED) || len(data4) != 12 {
		t.Fatalf("expected SMSG_TRAINER_BUY_SUCCEEDED (0x%x), got op=0x%x err=%v", protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED, op4, err)
	}

	// 5. SMSG_GOSSIP_COMPLETE
	op5, _, err := readServerFrame(clientConn, nil)
	if err != nil || op5 != uint16(protocol.OpcodeSMSG_GOSSIP_COMPLETE) {
		t.Fatalf("expected SMSG_GOSSIP_COMPLETE (0x%x), got op=0x%x err=%v", protocol.OpcodeSMSG_GOSSIP_COMPLETE, op5, err)
	}

	// Verify DB state
	var dbMap, dbZone int
	var dbX, dbY, dbZ float64
	err = db.QueryRow("SELECT homebind_map, homebind_zone, homebind_x, homebind_y, homebind_z FROM characters WHERE guid = 1").Scan(&dbMap, &dbZone, &dbX, &dbY, &dbZ)
	if err != nil {
		t.Fatal(err)
	}
	if dbMap != 0 || dbZone != 1519 || dbX != -8867.0 || dbY != 673.0 || dbZ != 97.0 {
		t.Fatalf("homebind mismatch in db: map=%d zone=%d x=%f y=%f z=%f", dbMap, dbZone, dbX, dbY, dbZ)
	}
}


