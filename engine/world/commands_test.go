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
	if !state.executeCommand(ctx, "gm visible off") {
		t.Fatal("gm visible off failed")
	}
	if player.ExtraFlags&playerExtraGMInvisible == 0 || player.PlayerFlags&playerFlagGhost != 0 {
		t.Fatalf("invisible state used the player ghost flag: extra=%x flags=%x", player.ExtraFlags, player.PlayerFlags)
	}
	if !state.executeCommand(ctx, "gm visible on") {
		t.Fatal("gm visible on failed")
	}
	if player.ExtraFlags&playerExtraGMInvisible != 0 {
		t.Fatal("gm visible on did not clear invisible flag")
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

func TestWorldTeleportRBACGate(t *testing.T) {
	player := &playerState{
		GUID:        99,
		Map:         0,
		X:           100,
		Y:           200,
		Z:           300,
		Orientation: 1.0,
		ExtraFlags:  0,
	}
	state := &session{
		playerLoaded: true,
		playerGUID:   99,
		player:       player,
		security:     0,
	}
	ctx := context.Background()

	// Forge CMSG_WORLD_TELEPORT packet: time(4), map(4), x(4), y(4), z(4), o(4)
	buf := protocol.NewBuffer(24)
	buf.WriteU32(12345) // time
	buf.WriteU32(1)     // map
	buf.WriteF32(500)   // x
	buf.WriteF32(600)   // y
	buf.WriteF32(700)   // z
	buf.WriteF32(2.5)   // o

	// 1. Non-GM player attempt: must be blocked
	state.handleWorldTeleport(ctx, buf.Bytes())
	if player.Map != 0 || player.X != 100 {
		t.Fatalf("unauthorized CMSG_WORLD_TELEPORT succeeded: map=%d x=%f", player.Map, player.X)
	}

	// 2. GM player attempt: must succeed
	state.security = 1
	state.handleWorldTeleport(ctx, buf.Bytes())
	if player.Map != 1 || player.X != 500 || player.Y != 600 || player.Z != 700 {
		t.Fatalf("authorized CMSG_WORLD_TELEPORT failed: map=%d x=%f y=%f z=%f", player.Map, player.X, player.Y, player.Z)
	}
}


