package world

import (
	"context"
	"math"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type testPacket struct {
	op   uint16
	data []byte
}

func readPacketTimeout(t *testing.T, ch <-chan testPacket) testPacket {
	select {
	case p := <-ch:
		return p
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for server packet")
		return testPacket{}
	}
}

func TestPosition_NormalizeOrientation(t *testing.T) {
	cases := []struct {
		input    float32
		expected float32
	}{
		{0.0, 0.0},
		{float32(math.Pi), float32(math.Pi)},
		{float32(2 * math.Pi), 0.0},
		{float32(3 * math.Pi), float32(math.Pi)},
		{-float32(math.Pi / 2), float32(1.5 * math.Pi)},
		{-float32(3 * math.Pi), float32(math.Pi)},
	}

	for _, tc := range cases {
		got := normalizeOrientation(tc.input)
		if math.Abs(float64(got-tc.expected)) > 0.001 {
			t.Fatalf("normalizeOrientation(%f): expected %f, got %f", tc.input, tc.expected, got)
		}
	}
}

func TestPosition_HasInArc(t *testing.T) {
	// Observer at (0, 0) facing 0 (along positive X axis)
	obsOri := float32(0.0)
	obsX, obsY := float32(0.0), float32(0.0)

	// 1. Target straight ahead at (10, 0)
	if !hasInArc(obsOri, obsX, obsY, 10.0, 0.0, math.Pi) {
		t.Fatalf("expected target straight ahead to be in 180° arc")
	}
	if !hasInArc(obsOri, obsX, obsY, 10.0, 0.0, 2.0*math.Pi/3.0) {
		t.Fatalf("expected target straight ahead to be in 120° arc")
	}

	// 2. Target at 45°: (10, 10)
	if !hasInArc(obsOri, obsX, obsY, 10.0, 10.0, math.Pi) {
		t.Fatalf("expected target at 45° to be in 180° arc")
	}
	if !hasInArc(obsOri, obsX, obsY, 10.0, 10.0, 2.0*math.Pi/3.0) {
		t.Fatalf("expected target at 45° to be in 120° arc (halfArc is 60°)")
	}

	// 3. Target at ~75°: (2.5, 10) -> angle is ~76°
	if !hasInArc(obsOri, obsX, obsY, 2.5, 10.0, math.Pi) {
		t.Fatalf("expected target at 76° to be in 180° arc (halfArc is 90°)")
	}
	if hasInArc(obsOri, obsX, obsY, 2.5, 10.0, 2.0*math.Pi/3.0) {
		t.Fatalf("expected target at 76° to NOT be in 120° arc (halfArc is 60°)")
	}

	// 4. Target directly behind: (-10, 0)
	if hasInArc(obsOri, obsX, obsY, -10.0, 0.0, math.Pi) {
		t.Fatalf("expected target directly behind to NOT be in 180° arc")
	}
	if hasInArc(obsOri, obsX, obsY, -10.0, 0.0, 2.0*math.Pi/3.0) {
		t.Fatalf("expected target directly behind to NOT be in 120° arc")
	}
}

func TestPosition_IsInFrontAndBehindHelpers(t *testing.T) {
	targetOri := float32(0.0) // Target facing +X (angle 0)
	targetX, targetY := float32(0.0), float32(0.0)

	// Attacker at (10, 0): attacker is in front of target
	if !isInFrontOf(targetOri, targetX, targetY, 10.0, 0.0) {
		t.Fatalf("expected attacker at (10, 0) to be in front of target facing 0")
	}
	if isBehindTarget(targetOri, targetX, targetY, 10.0, 0.0) {
		t.Fatalf("expected attacker at (10, 0) to NOT be behind target facing 0")
	}

	// Attacker at (-10, 0): attacker is behind target
	if isInFrontOf(targetOri, targetX, targetY, -10.0, 0.0) {
		t.Fatalf("expected attacker at (-10, 0) to NOT be in front of target facing 0")
	}
	if !isBehindTarget(targetOri, targetX, targetY, -10.0, 0.0) {
		t.Fatalf("expected attacker at (-10, 0) to be behind target facing 0")
	}
}

func TestCombatDefense_AttacksFromBehind(t *testing.T) {
	// TrinityCore combat rules:
	// If attacker is behind defender:
	// - canParry = false
	// - canBlock = false
	// - if defender is player: canDodge = false (players cannot dodge from behind)
	// - if defender is creature: canDodge = true (creatures can dodge from behind)

	// 1. Attacking player from behind: 0 dodges, 0 parries, 0 blocks
	for i := 0; i < 3000; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, false, false)
		if outcome == protocol.MeleeHitDodge {
			t.Fatalf("unexpected dodge when player defender is attacked from behind (iteration %d)", i)
		}
		if outcome == protocol.MeleeHitParry {
			t.Fatalf("unexpected parry when defender is attacked from behind (iteration %d)", i)
		}
		if outcome == protocol.MeleeHitBlock {
			t.Fatalf("unexpected block when defender is attacked from behind (iteration %d)", i)
		}
	}

	// 2. Attacking creature from behind: parry and block are 0, but creature CAN dodge
	dodgeCount := 0
	for i := 0; i < 3000; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, false, false, false, false, true)
		if outcome == protocol.MeleeHitParry {
			t.Fatalf("unexpected parry when creature defender is attacked from behind")
		}
		if outcome == protocol.MeleeHitBlock {
			t.Fatalf("unexpected block when creature defender is attacked from behind")
		}
		if outcome == protocol.MeleeHitDodge {
			dodgeCount++
		}
	}
	if dodgeCount == 0 {
		t.Fatalf("expected creature defender to be able to dodge from behind, got 0 dodges in 3000 rolls")
	}
}

func TestSpellCast_BackstabBehindRequirement(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		Data:           wotlk.NewStore("../../data/dbc"),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:        1001,
			Level:       80,
			Map:         0,
			X:           0.0,
			Y:           0.0,
			Z:           0.0,
			Orientation: 0.0, // facing +X
			Powers:      [7]uint32{100, 100, 100, 100, 100, 100, 100},
			Spells:      []learnedSpell{{ID: 53, Active: true}}, // Backstab Rank 1
		},
	}
	srv.sessions[sess] = struct{}{}

	targetGUID := creatureWorldGUID(10, 500)
	// Target at (5, 0) facing +X (0.0): Target back is facing (-X), facing towards (0, 0)
	srv.creatureMotion[targetGUID] = &creatureMotion{
		GUID:        targetGUID,
		Map:         0,
		X:           5.0,
		Y:           0.0,
		Z:           0.0,
		Orientation: 0.0, // facing +X
		Health:      1000,
		MaxHealth:   1000,
		CombatReach: 1.5,
	}

	ctx := context.Background()

	frames := make(chan testPacket, 10)
	go func() {
		for {
			op, data, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			frames <- testPacket{op: op, data: data}
		}
	}()

	// Case A: Caster is at (10, 0) facing -X (math.Pi).
	// Target is at (5, 0) facing +X.
	// Caster is in FRONT of target!
	// Backstab should FAIL with SPELL_FAILED_NOT_BEHIND (57).
	sess.player.X = 10.0
	sess.player.Y = 0.0
	sess.player.Orientation = float32(math.Pi) // facing target

	castPkt := protocol.NewBuffer(32)
	castPkt.WriteU8(1)         // castCount
	castPkt.WriteU32(53)       // spellID = Backstab
	castPkt.WriteU8(0)         // castFlags
	protocol.WriteSpellTargetData(castPkt, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID})

	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt := readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED for Backstab in front of target, got opcode 0x%04X", pkt.op)
	}
	reader := protocol.NewReader(pkt.data)
	_, _ = reader.ReadU8()  // castCount
	_, _ = reader.ReadU32() // spellID
	failReason, _ := reader.ReadU8()
	if failReason != 57 { // SPELL_FAILED_NOT_BEHIND = 57
		t.Fatalf("expected fail reason SPELL_FAILED_NOT_BEHIND (57), got %d", failReason)
	}

	// Case B: Caster is at (0, 0) (behind target), but facing away from target (Orientation = math.Pi).
	// Should fail with SPELL_FAILED_UNIT_NOT_INFRONT (81) because caster is not facing target!
	sess.player.X = 0.0
	sess.player.Y = 0.0
	sess.player.Orientation = float32(math.Pi) // facing away (-X)

	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt = readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED for Backstab when facing away, got opcode 0x%04X", pkt.op)
	}
	reader = protocol.NewReader(pkt.data)
	_, _ = reader.ReadU8()
	_, _ = reader.ReadU32()
	failReason, _ = reader.ReadU8()
	if failReason != 81 { // SPELL_FAILED_UNIT_NOT_INFRONT = 81
		t.Fatalf("expected fail reason SPELL_FAILED_UNIT_NOT_INFRONT (81), got %d", failReason)
	}

	// Case C: Caster is at (0, 0) (behind target) AND facing target (Orientation = 0.0).
	// Backstab should SUCCEED!
	sess.player.X = 0.0
	sess.player.Y = 0.0
	sess.player.Orientation = 0.0 // facing +X (towards target)

	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt = readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_SPELL_START) && pkt.op != uint16(protocol.OpcodeSMSG_SPELL_GO) {
		t.Fatalf("expected SMSG_SPELL_START or SMSG_SPELL_GO on valid Backstab from behind, got opcode 0x%04X", pkt.op)
	}
}

func TestSpellCast_GougeFacingRequirement(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		Data:           wotlk.NewStore("../../data/dbc"),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:        1002,
			Level:       80,
			Map:         0,
			X:           0.0,
			Y:           0.0,
			Z:           0.0,
			Orientation: 0.0, // facing +X
			Powers:      [7]uint32{100, 100, 100, 100, 100, 100, 100},
			Spells:      []learnedSpell{{ID: 1776, Active: true}}, // Gouge Rank 1
		},
	}
	srv.sessions[sess] = struct{}{}

	targetGUID := creatureWorldGUID(11, 501)
	// Target at (5, 0)
	srv.creatureMotion[targetGUID] = &creatureMotion{
		GUID:        targetGUID,
		Map:         0,
		X:           5.0,
		Y:           0.0,
		Z:           0.0,
		Orientation: 0.0, // facing away (+X)
		Health:      1000,
		MaxHealth:   1000,
		CombatReach: 1.5,
	}

	ctx := context.Background()

	frames := make(chan testPacket, 10)
	go func() {
		for {
			op, data, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			frames <- testPacket{op: op, data: data}
		}
	}()

	castPkt := protocol.NewBuffer(32)
	castPkt.WriteU8(1)
	castPkt.WriteU32(1776) // Gouge
	castPkt.WriteU8(0)
	protocol.WriteSpellTargetData(castPkt, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID})

	// Case A: Target is facing +X (away from caster at 0, 0).
	// Gouge requires target to face caster -> fails with SPELL_FAILED_NOT_INFRONT (58).
	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt := readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED for Gouge when target faces away, got opcode 0x%04X", pkt.op)
	}
	reader := protocol.NewReader(pkt.data)
	_, _ = reader.ReadU8()
	_, _ = reader.ReadU32()
	failReason, _ := reader.ReadU8()
	if failReason != 58 { // SPELL_FAILED_NOT_INFRONT = 58
		t.Fatalf("expected fail reason SPELL_FAILED_NOT_INFRONT (58), got %d", failReason)
	}

	// Case B: Target turns to face caster (Orientation = math.Pi).
	// Caster is at (0, 0) facing target (Orientation = 0.0).
	// Gouge succeeds!
	srv.creatureMotion[targetGUID].Orientation = float32(math.Pi)

	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt = readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_SPELL_START) && pkt.op != uint16(protocol.OpcodeSMSG_SPELL_GO) {
		t.Fatalf("expected SMSG_SPELL_START or SMSG_SPELL_GO on valid Gouge, got opcode 0x%04X", pkt.op)
	}
}

func TestSpellCast_TargetNotInFrontRequirement(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		Data:           wotlk.NewStore("../../data/dbc"),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1003,
		player: &playerState{
			GUID:        1003,
			Level:       80,
			Map:         0,
			X:           0.0,
			Y:           0.0,
			Z:           0.0,
			Orientation: 0.0, // facing +X
			Powers:      [7]uint32{100, 100, 100, 100, 100, 100, 100},
			Spells:      []learnedSpell{{ID: 116, Active: true}}, // Frostbolt Rank 1
		},
	}
	srv.sessions[sess] = struct{}{}

	targetGUID := creatureWorldGUID(12, 502)
	// Target at (-10, 0): directly behind caster
	srv.creatureMotion[targetGUID] = &creatureMotion{
		GUID:        targetGUID,
		Map:         0,
		X:           -10.0,
		Y:           0.0,
		Z:           0.0,
		Orientation: 0.0,
		Health:      1000,
		MaxHealth:   1000,
		CombatReach: 1.5,
	}

	ctx := context.Background()

	frames := make(chan testPacket, 10)
	go func() {
		for {
			op, data, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			frames <- testPacket{op: op, data: data}
		}
	}()

	castPkt := protocol.NewBuffer(32)
	castPkt.WriteU8(1)
	castPkt.WriteU32(116) // Frostbolt
	castPkt.WriteU8(0)
	protocol.WriteSpellTargetData(castPkt, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID})

	// Case A: Target is behind caster -> fails with SPELL_FAILED_UNIT_NOT_INFRONT (81)
	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt := readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED when target is behind caster, got opcode 0x%04X", pkt.op)
	}
	reader := protocol.NewReader(pkt.data)
	_, _ = reader.ReadU8()
	_, _ = reader.ReadU32()
	failReason, _ := reader.ReadU8()
	if failReason != 81 { // SPELL_FAILED_UNIT_NOT_INFRONT = 81
		t.Fatalf("expected fail reason SPELL_FAILED_UNIT_NOT_INFRONT (81), got %d", failReason)
	}

	// Case B: Target moved to front of caster (+10, 0) -> Frostbolt succeeds!
	srv.creatureMotion[targetGUID].X = 10.0

	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt = readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_SPELL_START) && pkt.op != uint16(protocol.OpcodeSMSG_SPELL_GO) {
		t.Fatalf("expected SMSG_SPELL_START or SMSG_SPELL_GO on valid Frostbolt cast, got opcode 0x%04X", pkt.op)
	}
}

