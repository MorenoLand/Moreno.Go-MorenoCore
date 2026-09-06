package world

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	protocol "github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestRollPPMChance_FormulaAccuracy(t *testing.T) {
	// 1.0 PPM with 2000ms weapon speed: chance = 2000 / 60000 = 1/30 = ~3.333%
	trials := 20000
	procs := 0
	for i := 0; i < trials; i++ {
		if RollPPMChance(1.0, 2000) {
			procs++
		}
	}
	actualRate := float64(procs) / float64(trials)
	expectedRate := 2000.0 / 60000.0
	if math.Abs(actualRate-expectedRate) > 0.01 {
		t.Errorf("expected PPM rate ~%.4f, got %.4f", expectedRate, actualRate)
	}

	// 1.0 PPM with 3600ms weapon speed: chance = 3600 / 60000 = 6.0%
	procs36 := 0
	for i := 0; i < trials; i++ {
		if RollPPMChance(1.0, 3600) {
			procs36++
		}
	}
	actualRate36 := float64(procs36) / float64(trials)
	expectedRate36 := 3600.0 / 60000.0
	if math.Abs(actualRate36-expectedRate36) > 0.01 {
		t.Errorf("expected PPM rate ~%.4f, got %.4f", expectedRate36, actualRate36)
	}
}

func TestGetEquipmentEnchant_SlotParsing(t *testing.T) {
	// Build equipmentCache string: 19 slots * 2 parts (itemEntry enchantID)
	parts := make([]string, 19*2)
	for i := 0; i < 19*2; i++ {
		parts[i] = "0"
	}
	// Slot 15 (Main Hand): item 49623 (Shadowmourne), enchant 3789 (Berserking)
	parts[15*2] = "49623"
	parts[15*2+1] = "3789"

	// Slot 16 (Off Hand): item 50730 (Havoc's Call), enchant 2673 (Mongoose)
	parts[16*2] = "50730"
	parts[16*2+1] = "2673"

	sess := &session{
		player: &playerState{
			Equipment: strings.Join(parts, " "),
		},
	}

	mainEnchant := sess.getEquipmentEnchant(15)
	if mainEnchant != EnchantIDBerserking {
		t.Errorf("expected main hand enchant %d, got %d", EnchantIDBerserking, mainEnchant)
	}

	offEnchant := sess.getEquipmentEnchant(16)
	if offEnchant != EnchantIDMongoose {
		t.Errorf("expected off hand enchant %d, got %d", EnchantIDMongoose, offEnchant)
	}
}

func TestGetEquipmentItem_SlotParsing(t *testing.T) {
	parts := make([]string, 19*2)
	for i := 0; i < 19*2; i++ {
		parts[i] = "0"
	}
	parts[equipSlotTrinket1*2] = "50362" // DBW Normal
	parts[equipSlotTrinket2*2] = "50343" // WFS Heroic
	parts[equipSlotFinger1*2] = "50402"  // Ashen Band Might

	sess := &session{
		player: &playerState{
			Equipment: strings.Join(parts, " "),
		},
	}

	if sess.getEquipmentItem(equipSlotTrinket1) != 50362 {
		t.Errorf("expected trinket 1 item 50362, got %d", sess.getEquipmentItem(equipSlotTrinket1))
	}
	if sess.getEquipmentItem(equipSlotTrinket2) != 50343 {
		t.Errorf("expected trinket 2 item 50343, got %d", sess.getEquipmentItem(equipSlotTrinket2))
	}
	if sess.getEquipmentItem(equipSlotFinger1) != 50402 {
		t.Errorf("expected finger 1 item 50402, got %d", sess.getEquipmentItem(equipSlotFinger1))
	}
}

func TestProcWeaponEnchantments_TriggerOnHitOnly(t *testing.T) {
	// Build equipment with 1000 PPM (guaranteed proc) Berserking on main hand
	parts := make([]string, 19*2)
	for i := 0; i < 19*2; i++ {
		parts[i] = "0"
	}
	parts[15*2] = "12345"
	parts[15*2+1] = fmt.Sprintf("%d", EnchantIDBerserking)

	sess := &session{
		playerGUID: 1,
		player: &playerState{
			GUID:       1,
			Level:      80,
			Health:     10000,
			MaxHealth:  10000,
			AttackTime: 60000, // 60s attack time guarantees 1.0 PPM proc (chance = 60000/60000 = 1.0)
			Equipment:  strings.Join(parts, " "),
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	target := combatTarget{
		GUID:   2,
		Health: 10000,
	}

	// Miss outcome should NOT proc
	sess.procWeaponEnchantments(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitMiss)
	if _, exists := sess.activeAuras[ProcSpellBerserking]; exists {
		t.Fatalf("expected miss to NOT proc enchantment")
	}

	// Dodge outcome should NOT proc
	sess.procWeaponEnchantments(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitDodge)
	if _, exists := sess.activeAuras[ProcSpellBerserking]; exists {
		t.Fatalf("expected dodge to NOT proc enchantment")
	}

	// Normal hit with guaranteed PPM must proc
	sess.procWeaponEnchantments(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitNormal)
	if _, exists := sess.activeAuras[ProcSpellBerserking]; !exists {
		t.Fatalf("expected Berserking to proc on normal hit with 100%% PPM chance")
	}
}

func TestInternalCooldown_EnchantBlackMagic(t *testing.T) {
	parts := make([]string, 19*2)
	for i := 0; i < 19*2; i++ {
		parts[i] = "0"
	}
	parts[15*2] = "49623"
	parts[15*2+1] = fmt.Sprintf("%d", EnchantIDBlackMagic)

	sess := &session{
		playerGUID: 1,
		player: &playerState{
			GUID:       1,
			Level:      80,
			Health:     10000,
			MaxHealth:  10000,
			AttackTime: 60000, // 100% PPM proc chance
			Equipment:  strings.Join(parts, " "),
		},
		activeAuras: make(map[uint32]*activeAura),
		procICD:     make(map[uint32]time.Time),
	}

	target := combatTarget{
		GUID:   2,
		Health: 10000,
	}

	// First hit should proc Black Magic and activate 35s ICD
	sess.procWeaponEnchantments(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitNormal)
	if _, exists := sess.activeAuras[ProcSpellBlackMagic]; !exists {
		t.Fatalf("expected Black Magic to proc on first hit")
	}
	if !sess.isProcOnCooldown(ProcSpellBlackMagic) {
		t.Fatalf("expected Black Magic to be on ICD after proc")
	}
	rem := sess.getProcRemainingCooldown(ProcSpellBlackMagic)
	if rem <= 30*time.Second || rem > 36*time.Second {
		t.Errorf("expected ~35s remaining ICD, got %v", rem)
	}

	// Clear active aura to test if it re-procs while on ICD
	delete(sess.activeAuras, ProcSpellBlackMagic)

	// Second hit while on ICD MUST NOT proc
	sess.procWeaponEnchantments(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitNormal)
	if _, exists := sess.activeAuras[ProcSpellBlackMagic]; exists {
		t.Fatalf("expected Black Magic NOT to proc while on internal cooldown")
	}

	// Expire the ICD manually
	sess.procICD[ProcSpellBlackMagic] = time.Now().Add(-1 * time.Second)
	if sess.isProcOnCooldown(ProcSpellBlackMagic) {
		t.Fatalf("expected ICD to be expired")
	}

	// Third hit after ICD expired MUST proc again
	sess.procWeaponEnchantments(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitNormal)
	if _, exists := sess.activeAuras[ProcSpellBlackMagic]; !exists {
		t.Fatalf("expected Black Magic to proc again after ICD expired")
	}
}

func TestInternalCooldown_TrinketWFS(t *testing.T) {
	parts := make([]string, 19*2)
	for i := 0; i < 19*2; i++ {
		parts[i] = "0"
	}
	parts[equipSlotTrinket1*2] = fmt.Sprintf("%d", ItemWhisperingFangedSkullNorm)

	sess := &session{
		playerGUID: 1,
		player: &playerState{
			GUID:      1,
			Level:     80,
			Health:    10000,
			Equipment: strings.Join(parts, " "),
		},
		activeAuras: make(map[uint32]*activeAura),
		procICD:     make(map[uint32]time.Time),
	}

	target := combatTarget{
		GUID:   2,
		Health: 10000,
	}

	// Trigger attacks until WFS procs (35% chance)
	procced := false
	for attempt := 0; attempt < 50; attempt++ {
		sess.procItemAndTrinketEffects(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitNormal)
		if _, exists := sess.activeAuras[ProcSpellWFSNorm]; exists {
			procced = true
			break
		}
	}
	if !procced {
		t.Fatalf("expected WFS to proc within 50 attempts")
	}

	// Verify 45s ICD is active
	if !sess.isProcOnCooldown(ProcSpellWFSNorm) {
		t.Fatalf("expected WFS to be on ICD")
	}

	// Clear aura and verify it does NOT re-proc while on ICD
	delete(sess.activeAuras, ProcSpellWFSNorm)
	for attempt := 0; attempt < 20; attempt++ {
		sess.procItemAndTrinketEffects(context.Background(), target, protocol.BaseAttack, protocol.MeleeHitNormal)
	}
	if _, exists := sess.activeAuras[ProcSpellWFSNorm]; exists {
		t.Fatalf("expected WFS NOT to proc while on ICD")
	}
}

func TestInternalCooldown_CasterSundial(t *testing.T) {
	parts := make([]string, 19*2)
	for i := 0; i < 19*2; i++ {
		parts[i] = "0"
	}
	parts[equipSlotTrinket1*2] = fmt.Sprintf("%d", ItemSundialOfTheExiled)

	sess := &session{
		playerGUID: 1,
		player: &playerState{
			GUID:      1,
			Level:     80,
			Health:    10000,
			Equipment: strings.Join(parts, " "),
		},
		activeAuras: make(map[uint32]*activeAura),
		procICD:     make(map[uint32]time.Time),
	}

	target := combatTarget{
		GUID:   2,
		Health: 10000,
	}

	procced := false
	for attempt := 0; attempt < 50; attempt++ {
		sess.procSpellCastAndHitEffects(context.Background(), target, 133)
		if _, exists := sess.activeAuras[ProcSpellSundialOfTheExiled]; exists {
			procced = true
			break
		}
	}
	if !procced {
		t.Fatalf("expected Sundial of the Exiled to proc within 50 attempts")
	}

	if !sess.isProcOnCooldown(ProcSpellSundialOfTheExiled) {
		t.Fatalf("expected Sundial to have 45s ICD")
	}

	delete(sess.activeAuras, ProcSpellSundialOfTheExiled)
	for attempt := 0; attempt < 20; attempt++ {
		sess.procSpellCastAndHitEffects(context.Background(), target, 133)
	}
	if _, exists := sess.activeAuras[ProcSpellSundialOfTheExiled]; exists {
		t.Fatalf("expected Sundial NOT to proc while on ICD")
	}
}
