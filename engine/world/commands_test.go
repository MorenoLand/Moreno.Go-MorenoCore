package world

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestExecuteCommands(t *testing.T) {
	root, err := packageRoot()
	if err != nil {
		t.Fatal(err)
	}
	stores := makeMemoryStores(t, root)
	server := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	// Drain client frames in background
	receivedOpcodes := make(chan uint16, 100)
	go func() {
		for {
			opcode, _, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedOpcodes <- opcode
		}
	}()

	player := &playerState{
		GUID:        99,
		Name:        "Tester",
		Map:         0,
		X:           100,
		Y:           200,
		Z:           300,
		Orientation: 1.5,
		Level:       1,
		Health:      100,
		MaxHealth:   100,
		Money:       50,
		ExtraFlags:  0,
	}

	state := &session{
		server:       server,
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   99,
		player:       player,
		accountID:    7,
		accountName:  "TEST",
	}
	server.sessions[state] = struct{}{}

	ctx := context.Background()

	// 1. .help
	if !state.executeCommand(ctx, "help") {
		t.Fatal("help command failed")
	}
	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
			t.Fatalf("unexpected opcode %x", op)
		}
	default:
		t.Fatal("no frame received for help")
	}

	// 2. .gm on / .gm off
	if !state.executeCommand(ctx, "gm on") {
		t.Fatal("gm on failed")
	}
	if player.ExtraFlags&0x01 == 0 {
		t.Fatal("gm on did not set flag")
	}
	if !state.executeCommand(ctx, "gm off") {
		t.Fatal("gm off failed")
	}
	if player.ExtraFlags&0x01 != 0 {
		t.Fatal("gm off did not clear flag")
	}

	// 3. .modify hp 5000
	if !state.executeCommand(ctx, "modify hp 5000") {
		t.Fatal("modify hp failed")
	}
	if player.Health != 5000 || player.MaxHealth != 5000 {
		t.Fatalf("hp=%d maxhp=%d", player.Health, player.MaxHealth)
	}

	// 4. .modify money 12345
	if !state.executeCommand(ctx, "modify money 12345") {
		t.Fatal("modify money failed")
	}
	if player.Money != 12345 {
		t.Fatalf("money=%d", player.Money)
	}

	// 5. .modify level 80
	if !state.executeCommand(ctx, "modify level 80") {
		t.Fatal("modify level failed")
	}
	if player.Level != 80 {
		t.Fatalf("level=%d", player.Level)
	}

	// 6. .character rename
	if !state.executeCommand(ctx, "character rename") {
		t.Fatal("character rename failed")
	}
	if player.AtLogin&0x01 == 0 {
		t.Fatal("rename flag not set")
	}

	// 7. .go xyz 10 20 30 1
	if !state.executeCommand(ctx, "go xyz 10 20 30 1") {
		t.Fatal("go xyz failed")
	}
	if player.Map != 1 || player.X != 10 || player.Y != 20 || player.Z != 30 {
		t.Fatalf("position=%v map=%d", []float32{player.X, player.Y, player.Z}, player.Map)
	}

	// 8. .gm fly on / off
	if !state.executeCommand(ctx, "gm fly on") {
		t.Fatal("gm fly on failed")
	}
	if !state.executeCommand(ctx, "gm fly off") {
		t.Fatal("gm fly off failed")
	}
}
