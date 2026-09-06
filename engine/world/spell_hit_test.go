package world

import (
	"math"
	"testing"
)

func TestCalculateSpellHitChance_PvEAndPvP(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}

	// 1. Equal level player vs creature (PvE): 96% hit chance
	hitPvE := sess.calculateSpellHitChance(80, false)
	if math.Abs(hitPvE-0.96) > 0.001 {
		t.Errorf("expected equal level PvE hit chance 0.96, got %.4f", hitPvE)
	}

	// 2. Equal level player vs player (PvP): 96% hit chance
	hitPvP := sess.calculateSpellHitChance(80, true)
	if math.Abs(hitPvP-0.96) > 0.001 {
		t.Errorf("expected equal level PvP hit chance 0.96, got %.4f", hitPvP)
	}

	// 3. Level 80 player vs Level 83 Raid Boss (+3 levels): 83% hit chance (17% miss)
	hitBoss := sess.calculateSpellHitChance(83, false)
	if math.Abs(hitBoss-0.83) > 0.001 {
		t.Errorf("expected level 83 boss PvE hit chance 0.83, got %.4f", hitBoss)
	}

	// 4. Add 17% spell hit rating (17 * 26.231995 = 445.94 rating)
	sess.player.CombatRatings[CombatRatingHitSpell] = 446
	hitBossCapped := sess.calculateSpellHitChance(83, false)
	if math.Abs(hitBossCapped-1.00) > 0.001 {
		t.Errorf("expected capped boss hit chance 1.00, got %.4f", hitBossCapped)
	}
}

func TestRollSpellHit_Accuracy(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}

	// 83% hit chance against boss across 20000 trials
	trials := 20000
	hits := 0
	for i := 0; i < trials; i++ {
		if sess.rollSpellHit(83, false) {
			hits++
		}
	}
	actualRate := float64(hits) / float64(trials)
	expectedRate := 0.83
	if math.Abs(actualRate-expectedRate) > 0.015 {
		t.Errorf("expected hit rate ~%.4f, got %.4f", expectedRate, actualRate)
	}
}
