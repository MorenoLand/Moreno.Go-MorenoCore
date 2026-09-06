package world

import (
	"math"
	"testing"
	"time"

	protocol "github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestMeleeHitRating_ReducesMissChance(t *testing.T) {
	// Baseline dual-wielding level 80 vs 80: base miss 5% + dual wield 19% = 24% (2400 BP)
	missBaseline := 0
	trials := 2000
	for i := 0; i < trials; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, true, false, false, false, 0, 0, 0, 0)
		if outcome == protocol.MeleeHitMiss {
			missBaseline++
		}
	}
	baselinePct := float64(missBaseline) / float64(trials)
	if baselinePct < 0.18 || baselinePct > 0.30 {
		t.Errorf("expected baseline miss around 24%%, got %.2f%%", baselinePct*100)
	}

	// With +24% hit (2400 BP), miss should be completely eliminated (0%)
	missWithHit := 0
	for i := 0; i < trials; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, true, false, false, false, 0, 2400, 0, 0)
		if outcome == protocol.MeleeHitMiss {
			missWithHit++
		}
	}
	if missWithHit > 0 {
		t.Errorf("expected 0 misses with 24%% hit rating bonus, got %d misses", missWithHit)
	}
}

func TestExpertiseRating_ReducesDodgeAndParry(t *testing.T) {
	trials := 3000

	// Baseline: 5% dodge (500 BP), 5% parry (500 BP)
	dodgeBaseline, parryBaseline := 0, 0
	for i := 0; i < trials; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, true, true, 0, 1000, 0, 0)
		if outcome == protocol.MeleeHitDodge {
			dodgeBaseline++
		} else if outcome == protocol.MeleeHitParry {
			parryBaseline++
		}
	}
	if dodgeBaseline == 0 || parryBaseline == 0 {
		t.Fatalf("expected some dodges and parries at baseline, got dodge=%d, parry=%d", dodgeBaseline, parryBaseline)
	}

	// With 26 expertise (~6.5% reduction = 650 BP), both 5% dodge and 5% parry should be completely pushed off the table
	dodgeWithExp, parryWithExp := 0, 0
	for i := 0; i < trials; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, true, true, 0, 1000, 0, 650)
		if outcome == protocol.MeleeHitDodge {
			dodgeWithExp++
		} else if outcome == protocol.MeleeHitParry {
			parryWithExp++
		}
	}
	if dodgeWithExp > 0 || parryWithExp > 0 {
		t.Errorf("expected 0 dodges and 0 parries with 650 BP expertise, got dodge=%d, parry=%d", dodgeWithExp, parryWithExp)
	}
}

func TestMeleeCritRating_IncreasesCritOutcome(t *testing.T) {
	trials := 2000

	// Baseline: 5% crit (500 BP)
	critBaseline := 0
	for i := 0; i < trials; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, false, false, 0, 1000, 0, 0)
		if outcome == protocol.MeleeHitCrit {
			critBaseline++
		}
	}

	// With +30% crit bonus (3000 BP)
	critHigh := 0
	for i := 0; i < trials; i++ {
		outcome, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, false, false, 0, 1000, 3000, 0)
		if outcome == protocol.MeleeHitCrit {
			critHigh++
		}
	}

	critBasePct := float64(critBaseline) / float64(trials)
	critHighPct := float64(critHigh) / float64(trials)

	if critHighPct <= critBasePct+0.20 {
		t.Errorf("expected high crit around ~35%%, got base=%.2f%%, high=%.2f%%", critBasePct*100, critHighPct*100)
	}
}

func TestArmorPenetration_ReducesArmorMitigation(t *testing.T) {
	baseArmor := float64(10673)
	attackerLevel := uint8(80)
	rawDamage := uint32(1000)

	// 0% ArP
	dmgNoArP := calcArmorReducedDamage(baseArmor, attackerLevel, rawDamage, 0)

	// 50% ArP
	dmg50ArP := calcArmorReducedDamage(baseArmor, attackerLevel, rawDamage, 50.0)

	// 100% ArP (Hard Cap)
	dmg100ArP := calcArmorReducedDamage(baseArmor, attackerLevel, rawDamage, 100.0)

	if dmg50ArP <= dmgNoArP {
		t.Errorf("expected 50%% ArP to deal more damage than 0%% ArP: dmgNoArP=%d, dmg50ArP=%d", dmgNoArP, dmg50ArP)
	}
	if dmg100ArP <= dmg50ArP {
		t.Errorf("expected 100%% ArP to deal more damage than 50%% ArP: dmg50ArP=%d, dmg100ArP=%d", dmg50ArP, dmg100ArP)
	}

	// For a target with 3000 armor, 100% ArP should completely penetrate armor to 0
	dmgLowArmorNoArP := calcArmorReducedDamage(3000, 80, 1000, 0)
	dmgLowArmor100ArP := calcArmorReducedDamage(3000, 80, 1000, 100.0)

	if dmgLowArmor100ArP != 1000 {
		t.Errorf("expected 100%% ArP against 3000 armor to result in 1000 unmitigated damage, got %d (no ArP=%d)", dmgLowArmor100ArP, dmgLowArmorNoArP)
	}
}

func TestHastedMeleeAndRangedSpeed(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
			CombatRatings: [25]uint32{
				CombatRatingHasteMelee:  1640, // ~50% haste
				CombatRatingHasteRanged: 1640, // ~50% haste
			},
		},
	}

	baseSpeed := 2000 * time.Millisecond
	hastedMelee := sess.getHastedMeleeSpeed(baseSpeed)
	hastedRanged := sess.getHastedRangedSpeed(baseSpeed)

	// With ~50% haste, 2000ms base speed should become ~1333ms
	if hastedMelee > 1400*time.Millisecond || hastedMelee < 1250*time.Millisecond {
		t.Errorf("expected hasted melee speed ~1333ms, got %v", hastedMelee)
	}
	if hastedRanged > 1400*time.Millisecond || hastedRanged < 1250*time.Millisecond {
		t.Errorf("expected hasted ranged speed ~1333ms, got %v", hastedRanged)
	}
}

func TestItemModParsing_ExpertiseAndArmorPenetration(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}

	// Simulate equipping items with:
	// item mod 37 (ITEM_MOD_EXPERTISE_RATING) = 82 rating (~10 expertise)
	// item mod 44 (ITEM_MOD_ARMOR_PENETRATION_RATING) = 154 rating (~10% ArP)
	sess.player.CombatRatings[CombatRatingExpertise] = 82
	sess.player.CombatRatings[CombatRatingArmorPenetration] = 154

	expertise := sess.getExpertise()
	dodgeParryRed := sess.getExpertiseDodgeParryReductionPct()
	arpPct := sess.getArmorPenPct()

	if math.Abs(expertise-10.0) > 0.1 {
		t.Errorf("expected ~10.0 expertise, got %f", expertise)
	}
	if math.Abs(dodgeParryRed-2.5) > 0.1 {
		t.Errorf("expected ~2.5%% dodge/parry reduction, got %f", dodgeParryRed)
	}
	if math.Abs(arpPct-10.0) > 0.1 {
		t.Errorf("expected ~10.0%% ArP, got %f", arpPct)
	}
}
