package world

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestDiminishing_GroupResolution(t *testing.T) {
	cases := []struct {
		spellID  uint32
		mechanic uint32
		expected DiminishingGroup
	}{
		{33786, 0, DiminishingCyclone},
		{1833, 0, DiminishingOpeningStun},    // Cheap Shot
		{9005, 0, DiminishingOpeningStun},    // Pounce
		{408, 0, DiminishingControlledStun},  // Kidney Shot
		{853, 0, DiminishingControlledStun},  // Hammer of Justice
		{5211, 0, DiminishingControlledStun}, // Bash
		{44572, 0, DiminishingControlledStun}, // Deep Freeze
		{100, 0, DiminishingCharge},          // Charge
		{5782, 0, DiminishingFear},           // Fear
		{2094, 0, DiminishingFear},           // Blind
		{8122, 0, DiminishingFear},           // Psychic Scream
		{6789, 0, DiminishingHorror},         // Death Coil
		{118, 0, DiminishingDisorient},       // Polymorph
		{6770, 0, DiminishingDisorient},      // Sap
		{1776, 0, DiminishingDisorient},      // Gouge
		{122, 0, DiminishingControlledRoot},  // Frost Nova
		{339, 0, DiminishingControlledRoot},  // Entangling Roots
		{15487, 0, DiminishingSilence},       // Silence
		{47476, 0, DiminishingSilence},       // Strangulate
		{676, 0, DiminishingDisarm},          // Disarm
		{605, 0, DiminishingMindControl},     // Mind Control
		{710, 0, DiminishingBanish},          // Banish
		{355, 0, DiminishingTaunt},           // Taunt
		{1715, 0, DiminishingLimitOnly},      // Hamstring
		{31661, 0, DiminishingDragonsBreath}, // Dragon's Breath
		{19503, 0, DiminishingScatterShot},   // Scatter Shot
		{0, 1, DiminishingMindControl},       // MECHANIC_CHARM
		{0, 2, DiminishingDisorient},         // MECHANIC_DISORIENTED
		{0, 3, DiminishingDisarm},            // MECHANIC_DISARM
		{0, 5, DiminishingFear},              // MECHANIC_FEAR
		{0, 7, DiminishingControlledRoot},    // MECHANIC_ROOT
		{0, 9, DiminishingSilence},           // MECHANIC_SILENCE
		{0, 10, DiminishingSleep},            // MECHANIC_SLEEP
		{0, 11, DiminishingLimitOnly},        // MECHANIC_SNARE
		{0, 12, DiminishingControlledStun},   // MECHANIC_STUN
		{0, 18, DiminishingBanish},           // MECHANIC_BANISH
		{0, 24, DiminishingHorror},           // MECHANIC_HORROR
	}

	for _, c := range cases {
		actual := getDiminishingReturnsGroup(c.spellID, c.mechanic)
		if actual != c.expected {
			t.Errorf("spellID %d mechanic %d: expected DR group %d, got %d", c.spellID, c.mechanic, c.expected, actual)
		}
	}
}

func TestDiminishing_PvPDurationLimit(t *testing.T) {
	sess := &session{player: &playerState{GUID: 1, Level: 80}}

	// 1. Fear (base 20s = 20000ms) capped to 10000ms in PvP
	_, dur1, ok1 := sess.applyDiminishingToDuration(5782, 5, 20000, true)
	if !ok1 || dur1 != 10000 {
		t.Fatalf("expected Fear capped to 10000ms, got %d (ok=%v)", dur1, ok1)
	}

	// In PvE (isPvP = false), not capped
	_, durPvE, okPvE := sess.applyDiminishingToDuration(5782, 5, 20000, false)
	if !okPvE || durPvE != 20000 {
		t.Fatalf("expected Fear uncapped in PvE, got %d", durPvE)
	}

	// 2. Polymorph (base 50s = 50000ms) capped to 10000ms in PvP
	_, dur2, ok2 := sess.applyDiminishingToDuration(118, 17, 50000, true)
	if !ok2 || dur2 != 10000 {
		t.Fatalf("expected Polymorph capped to 10000ms, got %d", dur2)
	}

	// 3. Kidney Shot (base 6s = 6000ms) stays 6000ms (below 10s cap)
	_, dur3, ok3 := sess.applyDiminishingToDuration(408, 12, 6000, true)
	if !ok3 || dur3 != 6000 {
		t.Fatalf("expected Kidney Shot to remain 6000ms, got %d", dur3)
	}
}

func TestDiminishing_ProgressionAndImmunity(t *testing.T) {
	sess := &session{player: &playerState{GUID: 1, Level: 80}}
	spellID := uint32(5782) // Fear
	mechanic := uint32(5)   // MECHANIC_FEAR
	baseDur := uint32(20000)

	// Hit 1: 100% (capped at 10s) -> 10000ms
	grp1, dur1, ok1 := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if !ok1 || dur1 != 10000 || grp1 != DiminishingFear {
		t.Fatalf("Hit 1: expected 10000ms, got %d", dur1)
	}
	sess.incrDiminishing(grp1)
	sess.applyDiminishingAura(grp1, true)
	sess.applyDiminishingAura(grp1, false) // aura fades

	// Hit 2: 50% of 10s -> 5000ms
	grp2, dur2, ok2 := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if !ok2 || dur2 != 5000 {
		t.Fatalf("Hit 2: expected 5000ms (50%%), got %d", dur2)
	}
	sess.incrDiminishing(grp2)
	sess.applyDiminishingAura(grp2, true)
	sess.applyDiminishingAura(grp2, false)

	// Hit 3: 25% of 10s -> 2500ms
	grp3, dur3, ok3 := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if !ok3 || dur3 != 2500 {
		t.Fatalf("Hit 3: expected 2500ms (25%%), got %d", dur3)
	}
	sess.incrDiminishing(grp3)
	sess.applyDiminishingAura(grp3, true)
	sess.applyDiminishingAura(grp3, false)

	// Hit 4: 0% -> Immune!
	_, dur4, ok4 := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if ok4 || dur4 != 0 {
		t.Fatalf("Hit 4: expected Immune (ok=false, dur=0), got ok=%v, dur=%d", ok4, dur4)
	}
}

func TestDiminishing_ResetTimer(t *testing.T) {
	sess := &session{player: &playerState{GUID: 1, Level: 80}}
	spellID := uint32(853) // Hammer of Justice
	mechanic := uint32(12)
	baseDur := uint32(6000)

	// Apply 1st Stun
	grp, dur1, ok1 := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if !ok1 || dur1 != 6000 {
		t.Fatalf("Hit 1: expected 6000ms, got %d", dur1)
	}
	sess.incrDiminishing(grp)
	sess.applyDiminishingAura(grp, true)
	sess.applyDiminishingAura(grp, false) // aura fades now

	// 10 seconds later (under 15s window) -> still at level 2 (50%)
	sess.diminishing[grp].hitTime = time.Now().Add(-10 * time.Second)
	_, dur2, ok2 := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if !ok2 || dur2 != 3000 {
		t.Fatalf("expected 50%% stun after 10s, got %d", dur2)
	}

	// 16 seconds later (past 15s window) -> reset to level 1 (100%)
	sess.diminishing[grp].hitTime = time.Now().Add(-16 * time.Second)
	_, durReset, okReset := sess.applyDiminishingToDuration(spellID, mechanic, baseDur, true)
	if !okReset || durReset != 6000 {
		t.Fatalf("expected stun reset to 100%% (6000ms) after 16s, got %d", durReset)
	}
}

func TestDiminishing_IndependentCategories(t *testing.T) {
	sess := &session{player: &playerState{GUID: 1, Level: 80}}

	// Apply 3 fears until target is immune
	fearGrp := DiminishingFear
	sess.incrDiminishing(fearGrp)
	sess.incrDiminishing(fearGrp)
	sess.incrDiminishing(fearGrp)

	// Fear should be immune
	_, _, fearOk := sess.applyDiminishingToDuration(5782, 5, 20000, true)
	if fearOk {
		t.Fatal("expected Fear to be immune")
	}

	// Stun (Hammer of Justice) must still be 100%!
	stunGrp, stunDur, stunOk := sess.applyDiminishingToDuration(853, 12, 6000, true)
	if !stunOk || stunDur != 6000 || stunGrp != DiminishingControlledStun {
		t.Fatalf("expected Stun unaffected by Fear DR, got dur=%d, ok=%v", stunDur, stunOk)
	}

	// Silence (Strangulate) must still be 100%!
	silenceGrp, silenceDur, silenceOk := sess.applyDiminishingToDuration(47476, 9, 5000, true)
	if !silenceOk || silenceDur != 5000 || silenceGrp != DiminishingSilence {
		t.Fatalf("expected Silence unaffected by Fear/Stun DR, got dur=%d, ok=%v", silenceDur, silenceOk)
	}
}

func TestDiminishing_AuraApplicationAndImmunePacket(t *testing.T) {
	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	caster := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Level: 80, Health: 100, MaxHealth: 100},
	}
	target := &session{
		server:       srv,
		playerGUID:   2,
		playerLoaded: true,
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
		player:       &playerState{GUID: 2, Level: 80, Health: 100, MaxHealth: 100},
	}
	srv.sessions[caster] = struct{}{}
	srv.sessions[target] = struct{}{}

	fearSpell := wotlk.Spell{
		ID:       5782, // Fear
		Mechanic: 5,    // MECHANIC_FEAR
	}
	fearEff := wotlk.SpellEffect{
		Aura: 7, // SPELL_AURA_MOD_FEAR
	}

	// Advance target's fear DR to Immune level
	target.incrDiminishing(DiminishingFear)
	target.incrDiminishing(DiminishingFear)
	target.incrDiminishing(DiminishingFear)

	done := make(chan struct{})
	go func() {
		caster.applyAuraToTarget(context.Background(), 2, fearSpell, fearEff, 20000, 0, 0, 0)
		close(done)
	}()

	_ = cConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, data, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatalf("failed to read frame: %v", err)
	}
	<-done

	// Verify caster received SMSG_CAST_FAILED with SPELL_FAILED_IMMUNE (38)
	if op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED, got 0x%04X", op)
	}
	r := protocol.NewReader(data)
	_, _ = r.ReadU8()          // castID
	spID, _ := r.ReadU32()     // spellID
	failReason, _ := r.ReadU8() // result
	if spID != 5782 {
		t.Fatalf("expected spell 5782, got %d", spID)
	}
	if failReason != 38 { // SPELL_FAILED_IMMUNE
		t.Fatalf("expected fail reason SPELL_FAILED_IMMUNE (38), got %d", failReason)
	}

	// Verify target never received the aura
	if target.hasAura(5782) {
		t.Fatal("target should not have received aura when immune")
	}
}
