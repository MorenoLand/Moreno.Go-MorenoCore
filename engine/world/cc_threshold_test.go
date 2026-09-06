package world

import (
	"testing"
)

func TestRootAura_DamageThresholdAccumulation(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:      1,
			Health:    10000,
			MaxHealth: 10000, // 15% root threshold = 1500
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	frostNovaID := uint32(122)
	sess.activeAuras[frostNovaID] = &activeAura{
		SpellID:  frostNovaID,
		AuraType: 26, // SPELL_AURA_MOD_ROOT
	}

	// Minor periodic damage of 100 should not break root (DamageTaken becomes 100 < 1500)
	sess.procDamageAuras(false, 100)
	if _, exists := sess.activeAuras[frostNovaID]; !exists {
		t.Fatalf("expected root to remain active after 100 damage (threshold 1500)")
	}

	// Accumulate more damage: 500
	sess.procDamageAuras(false, 500)
	if _, exists := sess.activeAuras[frostNovaID]; !exists {
		t.Fatalf("expected root to remain active after 600 total damage")
	}

	// Deal 1000 more damage -> total 1600 >= 1500 threshold: root must break!
	sess.procDamageAuras(false, 1000)
	if _, exists := sess.activeAuras[frostNovaID]; exists {
		t.Fatalf("expected root to break after 1600 total damage exceeded 1500 threshold")
	}
}

func TestFearAura_DamageThresholdAccumulation(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:      1,
			Health:    10000,
			MaxHealth: 10000, // 10% fear threshold = 1000
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	fearSpellID := uint32(5782)
	sess.activeAuras[fearSpellID] = &activeAura{
		SpellID:  fearSpellID,
		AuraType: 7, // SPELL_AURA_MOD_FEAR
	}

	// 200 damage should not break fear
	sess.procDamageAuras(false, 200)
	if _, exists := sess.activeAuras[fearSpellID]; !exists {
		t.Fatalf("expected fear to remain active after 200 damage (threshold 1000)")
	}

	// Additional 900 damage -> total 1100 >= 1000 threshold: fear must break!
	sess.procDamageAuras(false, 900)
	if _, exists := sess.activeAuras[fearSpellID]; exists {
		t.Fatalf("expected fear to break after 1100 total damage exceeded 1000 threshold")
	}
}

func TestRootAura_HeavyDirectHit_BreaksImmediately(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:      1,
			Health:    10000,
			MaxHealth: 10000, // threshold = 1500
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	frostNovaID := uint32(122)
	sess.activeAuras[frostNovaID] = &activeAura{
		SpellID:  frostNovaID,
		AuraType: 26,
	}

	// Direct hit of 2000 damage immediately breaks the root
	sess.procDamageAuras(true, 2000)
	if _, exists := sess.activeAuras[frostNovaID]; exists {
		t.Fatalf("expected root to break immediately from 2000 direct damage hit")
	}
}

func TestCreatureRoot_DamageThreshold(t *testing.T) {
	srv := &Server{
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
	}

	creatureGUID := uint64(77777)
	rootID := uint32(339) // Entangling Roots
	srv.activeCreatureAuras[creatureGUID] = map[uint32]*activeAura{
		rootID: {
			SpellID:  rootID,
			AuraType: 26,
		},
	}

	// Creature max health 10,000 -> 15% threshold = 1500
	srv.procCreatureDamageAuras(creatureGUID, false, 500, 10000)
	if _, exists := srv.activeCreatureAuras[creatureGUID][rootID]; !exists {
		t.Fatalf("expected creature root to remain after 500 damage")
	}

	// Additional 1200 damage -> total 1700 >= 1500: breaks!
	srv.procCreatureDamageAuras(creatureGUID, false, 1200, 10000)
	if _, exists := srv.activeCreatureAuras[creatureGUID][rootID]; exists {
		t.Fatalf("expected creature root to break after exceeding 1500 threshold")
	}
}
