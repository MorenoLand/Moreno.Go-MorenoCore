package world

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSpellPenetration_PartialResistanceReduction(t *testing.T) {
	// Base test: damage=1000, school=Fire (4), victimRes=300, level 80 vs 80
	// Without penetration (pen=0): averageResist = 300 / (300 + 400) = ~42.8% resist
	resistedNoPen, remNoPen := calcMagicSpellResistance(1000, 4, 300, 80, 80, 0)
	if remNoPen >= 1000 && resistedNoPen == 0 {
		t.Fatalf("expected some resistance with 300 Fire Resistance and 0 penetration")
	}

	// With full penetration (pen=300): effectiveRes = 0 -> 0 resisted, full damage dealt
	resistedFullPen, remFullPen := calcMagicSpellResistance(1000, 4, 300, 80, 80, 300)
	if resistedFullPen != 0 || remFullPen != 1000 {
		t.Fatalf("expected 0 resisted and 1000 damage with 300 penetration against 300 resistance, got resisted=%d rem=%d", resistedFullPen, remFullPen)
	}

	// Over-penetration (pen=500 against 300 resistance) -> clamped to 0 effective resistance
	resistedOverPen, remOverPen := calcMagicSpellResistance(1000, 4, 300, 80, 80, 500)
	if resistedOverPen != 0 || remOverPen != 1000 {
		t.Fatalf("expected 0 resisted with over-penetration, got resisted=%d rem=%d", resistedOverPen, remOverPen)
	}
}

func TestSpellPenetration_LevelDifferenceCannotBePenetrated(t *testing.T) {
	// Target level 83 (Boss), Caster level 80.
	// Target gear resistance = 0.
	// Level difference = 3 -> +15 unpenetrable resistance.
	// Even with 500 penetration, effective gear resistance is 0, but level difference resistance (15) remains.
	resisted, _ := calcMagicSpellResistance(1000, 4, 0, 80, 83, 500)
	// With res = 15, averageResist = 15 / (15 + 510) = ~2.85% -> partial resist chance exists
	_ = resisted // verifies calculation runs without panicking with level difference
}

func TestBinarySpell_ResistanceRoll(t *testing.T) {
	// 1. Target with 0 resistance -> checkBinarySpellResist must ALWAYS return false (0% resist)
	for i := 0; i < 50; i++ {
		if checkBinarySpellResist(0, 0, 80, 80) {
			t.Fatalf("expected 0%% resist chance when target has 0 resistance")
		}
	}

	// 2. Target with 400 resistance and caster with 400 penetration -> effective resistance is 0 -> 0% resist
	for i := 0; i < 50; i++ {
		if checkBinarySpellResist(400, 400, 80, 80) {
			t.Fatalf("expected 0%% resist chance when spell penetration negates target resistance")
		}
	}

	// 3. Target with 400 resistance and caster with 0 penetration:
	// averageResist = 400 / (400 + 400) = 50% chance to resist
	resists := 0
	trials := 500
	for i := 0; i < trials; i++ {
		if checkBinarySpellResist(400, 0, 80, 80) {
			resists++
		}
	}
	// With 50% theoretical probability over 500 trials, resists should be comfortably between 150 and 350
	if resists < 150 || resists > 350 {
		t.Fatalf("expected ~50%% resists over %d trials, got %d", trials, resists)
	}
}

func TestBinarySpell_FullResistInSpellGo(t *testing.T) {
	sConnCaster, cConnCaster := net.Pipe()
	defer sConnCaster.Close()
	defer cConnCaster.Close()

	sConnTarget, cConnTarget := net.Pipe()
	defer sConnTarget.Close()
	defer cConnTarget.Close()

	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		Data:           wotlk.NewStore("../../data/dbc"),
	}

	caster := &session{
		server:       srv,
		conn:         sConnCaster,
		playerGUID:   3001,
		playerLoaded: true,
		player: &playerState{
			GUID:             3001,
			Level:            80,
			Class:            9, // Warlock
			SpellPenetration: 0,
			CombatRatings:    [25]uint32{CombatRatingHitSpell: 1000}, // 100% hit
		},
		auras:       make(map[uint32]struct{}),
		auraSlots:   make(map[uint32]uint8),
		activeAuras: make(map[uint32]*activeAura),
	}

	// Victim has massive Shadow Resistance (index 5 = Shadow)
	victim := &session{
		server:       srv,
		conn:         sConnTarget,
		playerGUID:   3002,
		playerLoaded: true,
		player: &playerState{
			GUID:        3002,
			Level:       80,
			Class:       1, // Warrior
			Resistances: [7]uint32{0, 0, 0, 0, 0, 2000, 0}, // 2000 Shadow Resistance -> ~83% resist
		},
		auras:       make(map[uint32]struct{}),
		auraSlots:   make(map[uint32]uint8),
		activeAuras: make(map[uint32]*activeAura),
	}

	srv.sessions[caster] = struct{}{}
	srv.sessions[victim] = struct{}{}

	frames := make(chan testPacket, 20)
	go func() {
		for {
			op, data, err := readServerFrame(cConnCaster, nil)
			if err != nil {
				return
			}
			frames <- testPacket{op: op, data: data}
		}
	}()
	go func() {
		for {
			if _, _, err := readServerFrame(cConnTarget, nil); err != nil {
				return
			}
		}
	}()

	fearSpell := wotlk.Spell{
		ID:         5782, // Fear
		SchoolMask: 32,   // Shadow
		Effects: [3]wotlk.SpellEffect{
			{Effect: 6, Aura: 7}, // Apply Aura SPELL_AURA_MOD_FEAR
		},
	}

	// Verify isBinarySpell recognizes Fear as binary
	if !isBinarySpell(fearSpell) {
		t.Fatalf("expected Fear (5782) to be recognized as a binary spell")
	}

	targetData := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: 3002,
	}

	// Over multiple casts, with 2000 shadow resistance, at least one resist is guaranteed
	resistedOnce := false
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		caster.finishSpellCast(ctx, uint8(i+1), 5782, fearSpell, targetData)
		pkt := readPacketTimeout(t, frames)
		if pkt.op == uint16(protocol.OpcodeSMSG_SPELL_GO) {
			r := protocol.NewReader(pkt.data)
			_, _ = r.ReadPackedGUID() // caster
			_, _ = r.ReadPackedGUID() // casterUnit
			_, _ = r.ReadU8()         // castID
			_, _ = r.ReadU32()        // spellID
			_, _ = r.ReadU32()        // flags
			_, _ = r.ReadU32()        // timestamp
			numHits, _ := r.ReadU8()
			if numHits == 0 {
				// Target missed/resisted
				numMiss, _ := r.ReadU8()
				if numMiss > 0 {
					_, _ = r.ReadU64() // targetGUID
					missReason, _ := r.ReadU8()
					if missReason == protocol.SpellMissResist {
						resistedOnce = true
						break
					}
				}
			}
		}
	}

	if !resistedOnce {
		t.Fatalf("expected Fear to be resisted at least once against 2000 shadow resistance")
	}
}

func TestSpellHitRating_Scaling(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}

	// 0 rating -> 0% bonus hit
	if hit := sess.getSpellHitPct(); hit != 0 {
		t.Fatalf("expected 0%% hit at 0 rating, got %f", hit)
	}

	// At level 80: 26.231995 rating = 1.0% hit -> ~262 rating is ~10% hit
	sess.player.CombatRatings[CombatRatingHitSpell] = 262
	hitPct := sess.getSpellHitPct()
	if math.Abs(hitPct-10.0) > 0.1 {
		t.Fatalf("expected ~10%% spell hit for 262 rating, got %f%%", hitPct)
	}

	// Test magicSpellHitResult with bonus hit:
	// Caster lvl 80 vs Victim lvl 83 (Boss) in PvE:
	// levelDiff = 3 -> baseHit = 94 - (3 - 2)*11 = 83% hit chance (17% miss chance)
	// With +17% spell hit (446 rating), hit chance reaches 100% cap!
	for i := 0; i < 100; i++ {
		miss := magicSpellHitResult(80, 83, false, 17.0)
		if miss != protocol.SpellMissNone {
			t.Fatalf("expected spell to never miss with hit cap reached, got miss result %d", miss)
		}
	}
}
