package world

import (
	"encoding/binary"
	"net"
	"testing"

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
