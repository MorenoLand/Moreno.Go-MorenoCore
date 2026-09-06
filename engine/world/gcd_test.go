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

func TestGCD_TriggerAndBlock(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		Data:           wotlk.NewStore("../../data/dbc"),
	}

	targetGUID := creatureWorldGUID(1, 100)
	srv.creatureMotion[targetGUID] = &creatureMotion{
		GUID:        targetGUID,
		Map:         0,
		X:           10.0,
		Y:           0.0,
		Z:           0.0,
		Orientation: float32(math.Pi),
		Health:      10000,
		MaxHealth:   10000,
		CombatReach: 1.5,
	}

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerLoaded: true,
		playerGUID:   2001,
		player: &playerState{
			GUID:        2001,
			Level:       80,
			Class:       8, // Mage
			Map:         0,
			X:           0.0,
			Y:           0.0,
			Z:           0.0,
			Orientation: 0.0,
			Powers:      [7]uint32{10000, 10000, 10000, 10000, 10000, 10000, 10000},
			Spells:      []learnedSpell{{ID: 133, Active: true}}, // Fireball
		},
		auras:       make(map[uint32]struct{}),
		auraSlots:   make(map[uint32]uint8),
		activeAuras: make(map[uint32]*activeAura),
	}
	srv.sessions[sess] = struct{}{}

	frames := make(chan testPacket, 20)
	go func() {
		for {
			op, data, err := readServerFrame(cConn, nil)
			if err != nil {
				return
			}
			frames <- testPacket{op: op, data: data}
		}
	}()

	ctx := context.Background()

	// Cast Fireball (133)
	castPkt := protocol.NewBuffer(32)
	castPkt.WriteU8(1)
	castPkt.WriteU32(133)
	castPkt.WriteU8(0)
	protocol.WriteSpellTargetData(castPkt, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID})

	sess.handleCastSpell(ctx, castPkt.Bytes())

	pkt := readPacketTimeout(t, frames)
	if pkt.op != uint16(protocol.OpcodeSMSG_SPELL_START) {
		t.Fatalf("expected SMSG_SPELL_START for first cast, got 0x%04X", pkt.op)
	}

	// Verify GCD is now active
	if !sess.isGCDActive(wotlk.Spell{ID: 133, StartRecoveryCategory: 133, StartRecoveryTime: 1500}) {
		t.Fatalf("expected GCD to be active immediately after starting cast")
	}

	// Immediately attempt second cast -> must be rejected with SPELL_FAILED_NOT_READY (47)
	castPkt2 := protocol.NewBuffer(32)
	castPkt2.WriteU8(2)
	castPkt2.WriteU32(133)
	castPkt2.WriteU8(0)
	protocol.WriteSpellTargetData(castPkt2, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID})

	sess.handleCastSpell(ctx, castPkt2.Bytes())

	pktFail := readPacketTimeout(t, frames)
	if pktFail.op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED while GCD is active, got 0x%04X", pktFail.op)
	}
	r := protocol.NewReader(pktFail.data)
	_, _ = r.ReadU8()  // castID
	_, _ = r.ReadU32() // spellID
	failReason, _ := r.ReadU8()
	if failReason != 47 { // SPELL_FAILED_NOT_READY = 47
		t.Fatalf("expected fail reason 47 (NOT_READY), got %d", failReason)
	}
}

func TestGCD_OffGCDSpellsBypass(t *testing.T) {
	srv := &Server{
		Data: wotlk.NewStore("../../data/dbc"),
	}

	sess := &session{
		server: srv,
		player: &playerState{
			GUID:  2002,
			Class: 4, // Rogue
		},
		gcdEnd: time.Now().UnixMilli() + 5000, // active GCD for 5 seconds
	}

	// Normal on-GCD spell should be blocked
	fireball := wotlk.Spell{ID: 133, StartRecoveryCategory: 133, StartRecoveryTime: 1500}
	if !sess.isGCDActive(fireball) {
		t.Fatalf("expected Fireball to be blocked while GCD is active")
	}

	// Off-GCD abilities must NOT be blocked
	offGCDSpells := []wotlk.Spell{
		{ID: 1766, StartRecoveryCategory: 0, StartRecoveryTime: 0},  // Kick
		{ID: 2139, StartRecoveryCategory: 0, StartRecoveryTime: 0},  // Counterspell
		{ID: 6552, StartRecoveryCategory: 0, StartRecoveryTime: 0},  // Pummel
		{ID: 57994, StartRecoveryCategory: 0, StartRecoveryTime: 0}, // Wind Shear
		{ID: 22812, StartRecoveryCategory: 0, StartRecoveryTime: 0}, // Barkskin
		{ID: 48707, StartRecoveryCategory: 0, StartRecoveryTime: 0}, // Anti-Magic Shell
		{ID: 75},                                                    // Auto Shot
	}

	for _, sp := range offGCDSpells {
		if sess.isGCDActive(sp) {
			t.Fatalf("expected off-GCD spell %d to bypass active GCD", sp.ID)
		}
	}
}

func TestGCD_RogueFlat1000ms(t *testing.T) {
	rogueSess := &session{
		player: &playerState{
			Class: 4, // Rogue
		},
	}

	sinisterStrike := wotlk.Spell{ID: 1752, StartRecoveryCategory: 133, StartRecoveryTime: 1500}
	duration := rogueSess.getGCDDuration(sinisterStrike)
	if duration != 1000 {
		t.Fatalf("expected Rogue GCD duration 1000ms, got %d", duration)
	}

	casterSess := &session{
		player: &playerState{
			Class: 8, // Mage
		},
	}
	casterDuration := casterSess.getGCDDuration(sinisterStrike)
	if casterDuration != 1500 {
		t.Fatalf("expected Caster base GCD duration 1500ms, got %d", casterDuration)
	}
}

func TestGCD_SpellHasteScalingAndFloor(t *testing.T) {
	mageSess := &session{
		player: &playerState{
			Class: 8, // Mage
			Level: 80,
		},
	}

	spell := wotlk.Spell{ID: 133, StartRecoveryCategory: 133, StartRecoveryTime: 1500}

	// 0 haste -> 1500ms
	if d := mageSess.getGCDDuration(spell); d != 1500 {
		t.Fatalf("expected 1500ms at 0 haste, got %d", d)
	}

	// 328 rating is ~10% haste -> 1500 / 1.10 = 1363.6 -> 1364ms
	mageSess.player.CombatRatings[CombatRatingHasteSpell] = 328
	if d := mageSess.getGCDDuration(spell); d != 1364 {
		t.Fatalf("expected 1364ms at ~10%% haste, got %d", d)
	}

	// 1640 rating is ~50% haste -> 1500 / 1.50 = 1000ms
	mageSess.player.CombatRatings[CombatRatingHasteSpell] = 1640
	if d := mageSess.getGCDDuration(spell); d != 1000 {
		t.Fatalf("expected 1000ms at ~50%% haste, got %d", d)
	}

	// Extreme haste: 3279 rating (~100% haste) -> 1500 / 2.00 = 750ms -> MUST FLOOR AT 1000ms
	mageSess.player.CombatRatings[CombatRatingHasteSpell] = 3279
	if d := mageSess.getGCDDuration(spell); d != 1000 {
		t.Fatalf("expected 1000ms minimum floor at 100%% haste, got %d", d)
	}
}

func TestGCD_ExpiryAllowsNextCast(t *testing.T) {
	sess := &session{
		player: &playerState{Class: 8},
		gcdEnd: time.Now().UnixMilli() - 100, // expired 100ms ago
	}

	spell := wotlk.Spell{ID: 133, StartRecoveryCategory: 133, StartRecoveryTime: 1500}
	if sess.isGCDActive(spell) {
		t.Fatalf("expected GCD to not be active after expiry")
	}
}
