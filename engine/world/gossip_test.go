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

