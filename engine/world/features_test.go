package world

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSoloLFGEnablesOnLoginAndAllowsPartialGroups(t *testing.T) {
	lfg := NewLFGManager(true)
	if lfg.Solo() || !lfg.RequiresFullGroup(1, 5) {
		t.Fatal("solo LFG should start disabled and require a full group before login")
	}
	lfg.OnLogin()
	if !lfg.Solo() || lfg.RequiresFullGroup(1, 5) {
		t.Fatal("solo LFG did not enable partial groups")
	}
	if lfg.ToggleSolo() {
		t.Fatal("toggle should disable solo LFG")
	}
}

func TestLFGQueueLifecycle(t *testing.T) {
	lfg := NewLFGManager(true)
	result, entry := lfg.Join(7, LFGRoleTank|LFGRoleDamage, []uint32{0x01000001, 2, 1, 2}, "ready")
	if result != LFGJoinOK || entry.State != LFGStateQueued || len(entry.Dungeons) != 3 || entry.Dungeons[0] != 1 || entry.Dungeons[2] != 0x01000001 {
		t.Fatalf("join result=%d entry=%+v", result, entry)
	}
	if status, ok := lfg.Status(7); !ok || status.Comment != "ready" {
		t.Fatalf("status=%+v exists=%v", status, ok)
	}
	if !lfg.Leave(7) {
		t.Fatal("queue entry was not removed")
	}
}

func TestLFGJoinPacket(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	server := &Server{Features: &Features{LFG: NewLFGManager(true)}}
	state := &session{server: server, conn: serverConn, playerGUID: 7, accountName: "TEST", playerLoaded: true, player: &playerState{GUID: 7, Name: "Tester"}}
	payload := protocol.NewBuffer(64)
	payload.WriteU32(uint32(LFGRoleTank))
	payload.WriteBool(false)
	payload.WriteBool(false)
	payload.WriteU8(2)
	payload.WriteU32(0x01000001)
	payload.WriteU32(2)
	payload.WriteU8(3)
	payload.Write([]byte{0, 0, 0})
	payload.WriteCString("ready")
	done := make(chan bool, 1)
	go func() { done <- state.handleLFGJoin(payload.Bytes()) }()
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_LFG_JOIN_RESULT) || len(response) != 8 || binary.LittleEndian.Uint32(response) != LFGJoinOK {
		t.Fatalf("join result opcode=%x payload=%x", opcode, response)
	}
	opcode, response, err = readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_LFG_UPDATE_PLAYER) || len(response) == 0 {
		t.Fatalf("update opcode=%x payload=%x", opcode, response)
	}
	if !<-done {
		t.Fatal("join handler failed")
	}
	_ = serverConn.Close()
}

func Test310FlyerStateRecalculatesFromLearnedSpells(t *testing.T) {
	state := NewMountState(0, []LearnedMountSpell{{ID: 100, MountedFlightSpeed: 280}, {ID: 200, MountedFlightSpeed: 310}})
	if !state.Has310Flyer(true, 0) || state.PreferredFlightSpeed(true) != 310 {
		t.Fatal("310 flyer was not detected")
	}
	if state.UnlearnSpell(200) || state.PreferredFlightSpeed(true) != 280 {
		t.Fatal("310 flyer flag was not cleared")
	}
	state.LearnSpell(LearnedMountSpell{ID: 300, MountedFlightSpeed: 310})
	if !state.Has310Flyer(false, 0) || state.ExtraFlags()&PlayerExtraHas310Flyer == 0 {
		t.Fatal("learning a 310 flyer did not set the extra flag")
	}
}

func TestNPCBotAssignmentLimitsAndSpellOrdering(t *testing.T) {
	cfg := config.Default().NPCBots
	cfg.MaxBots = 1
	mgr := &NPCBotManager{config: cfg, bots: map[uint32]NpcBotData{70001: {Entry: 70001}, 70002: {Entry: 70002, Owner: 8}}, extras: map[uint32]NpcBotExtras{70001: {Entry: 70001, Class: 1}, 70002: {Entry: 70002, Class: 2}}}
	if !mgr.CanAssign(7, 70001) || mgr.CanAssign(8, 70001) {
		t.Fatal("assignment limits are incorrect")
	}
	if got := formatSpellList([]uint32{900, 100, 900, 300}); got != "100 300 900 900 " {
		t.Fatalf("spell ordering=%q", got)
	}
}

func TestLFGPlayerLockInfoAndSetRoles(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{sessions: make(map[*session]struct{})}
	sess := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   42,
		player:       &playerState{GUID: 42, Name: "DungeonRunner"},
	}

	// 1. Test Player Lock Info Request
	done := make(chan struct{})
	go func() {
		sess.handleLfdPlayerLockInfoRequest(context.Background(), nil)
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if op != uint16(protocol.OpcodeSMSG_LFG_PLAYER_INFO) {
		t.Fatalf("expected SMSG_LFG_PLAYER_INFO (0x36F), got 0x%x", op)
	}
	if len(data) != 5 {
		t.Fatalf("expected 5 bytes in SMSG_LFG_PLAYER_INFO, got %d", len(data))
	}

	// 2. Test Set Roles
	rolesPayload := []byte{byte(LFGRoleTank | LFGRoleDamage)}

	done2 := make(chan struct{})
	go func() {
		sess.handleLfgSetRoles(context.Background(), rolesPayload)
		close(done2)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op2, data2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done2
	if op2 != uint16(protocol.OpcodeSMSG_LFG_ROLE_CHOSEN) {
		t.Fatalf("expected SMSG_LFG_ROLE_CHOSEN (0x2BB), got 0x%x", op2)
	}
	r := protocol.NewReader(data2)
	guid, _ := r.ReadU64()
	ready, _ := r.ReadU8()
	roles, _ := r.ReadU32()
	if guid != 42 || ready != 1 || roles != uint32(LFGRoleTank|LFGRoleDamage) {
		t.Fatalf("unexpected role chosen fields: guid=%d ready=%d roles=%d", guid, ready, roles)
	}
}
