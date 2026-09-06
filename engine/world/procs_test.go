package world

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

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
