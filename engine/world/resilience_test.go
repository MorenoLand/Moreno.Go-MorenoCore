package world

import (
	"testing"
)

func TestResilience_StatsScalingAndCaps(t *testing.T) {
	// At level 80, scale is ~94.271225 rating per 1%
	// 943 rating should give ~10% crit chance reduction
	critRed, critDmgRed, dmgRed := getResilienceStats(80, 943)
	if critRed < 9.9 || critRed > 10.1 {
		t.Fatalf("expected ~10.0%% crit chance reduction, got %f", critRed)
	}
	// 10% * 2.2 = 22% crit damage reduction
	if critDmgRed < 21.9 || critDmgRed > 22.1 {
		t.Fatalf("expected ~22.0%% crit damage reduction, got %f", critDmgRed)
	}
	// 10% * 2.0 = 20% all damage reduction
	if dmgRed < 19.9 || dmgRed > 20.1 {
		t.Fatalf("expected ~20.0%% all damage reduction, got %f", dmgRed)
	}

	// Test 33% cap on crit damage reduction:
	// With 2000 rating at level 80 -> ~21.2% resilience -> 21.2 * 2.2 = 46.6% -> capped at 33.0%
	_, critDmgRedCap, _ := getResilienceStats(80, 2000)
	if critDmgRedCap != 33.0 {
		t.Fatalf("expected crit damage reduction capped at 33.0%%, got %f", critDmgRedCap)
	}

	// Test level 70 scaling (~45.3365 rating per 1%)
	critRed70, _, _ := getResilienceStats(70, 453)
	if critRed70 < 9.9 || critRed70 > 10.1 {
		t.Fatalf("expected ~10.0%% crit reduction at level 70, got %f", critRed70)
	}
}

func TestResilience_ApplyToCritChanceAndDamage(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}
	sess.player.CombatRatings[CombatRatingCritTakenMelee] = 943 // ~10% crit reduction

	// 1. Crit chance reduction in basis points (500 bp = 5.0%, should drop by ~1000 bp -> clamp to 0)
	critBP := int32(500)
	sess.applyResilienceToMeleeCritChance(true, CombatRatingCritTakenMelee, &critBP)
	if critBP != 0 {
		t.Fatalf("expected crit chance reduced to 0, got %d", critBP)
	}

	// Now with smaller rating: 188 rating (~2.0% reduction = 200 bp)
	sess.player.CombatRatings[CombatRatingCritTakenMelee] = 188
	critBP = int32(500)
	sess.applyResilienceToMeleeCritChance(true, CombatRatingCritTakenMelee, &critBP)
	if critBP < 290 || critBP > 310 {
		t.Fatalf("expected crit chance around 300 bp, got %d", critBP)
	}

	// 2. Non-crit damage mitigation:
	// 943 rating (~10% resilience -> 20% all damage reduction)
	sess.player.CombatRatings[CombatRatingCritTakenMelee] = 943
	damage := uint32(1000)
	sess.applyResilienceToDamage(true, &damage, false, CombatRatingCritTakenMelee)
	// 1000 * (1 - 0.20) = 800
	if damage < 795 || damage > 805 {
		t.Fatalf("expected non-crit damage ~800, got %d", damage)
	}

	// 3. Crit damage mitigation:
	// 1000 crit damage:
	// crit damage reduction = 22% -> 1000 * 0.78 = 780
	// damage reduction = 20% -> 780 * 0.80 = 624
	damage = uint32(1000)
	sess.applyResilienceToDamage(true, &damage, true, CombatRatingCritTakenMelee)
	if damage < 620 || damage > 630 {
		t.Fatalf("expected crit damage ~624, got %d", damage)
	}

	// 4. Mobs/creatures do not suffer resilience reduction (attackerIsPlayer == false)
	damage = uint32(1000)
	sess.applyResilienceToDamage(false, &damage, true, CombatRatingCritTakenMelee)
	if damage != 1000 {
		t.Fatalf("expected no reduction from PvE attackers, got %d", damage)
	}
}

func TestResilience_RollMeleeOutcomeWithResilience(t *testing.T) {
	// Normal roll against equal level player has base 5% crit (500 bp)
	// With 500 bp critReductionBP, crit chance should drop to 0%
	// Running 2000 iterations to verify no crits occur
	for i := 0; i < 2000; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, false, 500)
		if outcome == 6 { // protocol.MeleeHitCrit = 6
			t.Fatalf("unexpected crit with 100%% resilience crit reduction at iteration %d", i)
		}
	}
}
