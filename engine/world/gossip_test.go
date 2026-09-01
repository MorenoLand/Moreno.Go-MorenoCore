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
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, name TEXT NOT NULL, modelid1 INTEGER NOT NULL, maxlevel INTEGER NOT NULL)",
		"INSERT INTO creature VALUES (321, 68, 0, 100)",
		"INSERT INTO creature_template VALUES (68, 'Stormwind Guard', 3167, 80)",
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
