package world

import (
	"testing"
)

func TestAbsorptionShield_FullAbsorption(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:   1,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	// Power Word: Shield with 500 absorption pool
	shieldSpellID := uint32(17)
	sess.activeAuras[shieldSpellID] = &activeAura{
		SpellID:    shieldSpellID,
		AuraType:   SpellAuraSchoolAbsorb,
		Amount:     500,
		SchoolMask: 0, // all schools
	}

	absorbed, remaining := sess.applyAbsorptionShields(300, 1) // Physical damage 300
	if absorbed != 300 {
		t.Errorf("expected 300 absorbed, got %d", absorbed)
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining damage, got %d", remaining)
	}
	if sess.activeAuras[shieldSpellID].Amount != 200 {
		t.Errorf("expected 200 remaining in shield pool, got %d", sess.activeAuras[shieldSpellID].Amount)
	}
}

func TestAbsorptionShield_ExhaustionAndRemoval(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:   1,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	shieldSpellID := uint32(17)
	sess.activeAuras[shieldSpellID] = &activeAura{
		SpellID:    shieldSpellID,
		AuraType:   SpellAuraSchoolAbsorb,
		Amount:     400,
		SchoolMask: 0,
	}

	absorbed, remaining := sess.applyAbsorptionShields(600, 1)
	if absorbed != 400 {
		t.Errorf("expected 400 absorbed, got %d", absorbed)
	}
	if remaining != 200 {
		t.Errorf("expected 200 remaining damage, got %d", remaining)
	}
	// Shield should be exhausted and removed from activeAuras
	if _, exists := sess.activeAuras[shieldSpellID]; exists {
		t.Errorf("expected shield aura to be removed after exhaustion")
	}
}

func TestAbsorptionShield_MagicAbsorb_DoesNotAbsorbPhysical(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:   1,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	// Anti-Magic Shell (SpellAuraMagicAbsorb = 256)
	amsSpellID := uint32(48707)
	sess.activeAuras[amsSpellID] = &activeAura{
		SpellID:    amsSpellID,
		AuraType:   SpellAuraMagicAbsorb,
		Amount:     5000,
		SchoolMask: 0,
	}

	// Physical damage (schoolMask = 1)
	absorbed, remaining := sess.applyAbsorptionShields(1000, 1)
	if absorbed != 0 {
		t.Errorf("expected 0 physical damage absorbed by Magic Absorb, got %d", absorbed)
	}
	if remaining != 1000 {
		t.Errorf("expected 1000 remaining damage, got %d", remaining)
	}

	// Shadow damage (schoolMask = 32)
	absorbedShadow, remainingShadow := sess.applyAbsorptionShields(1000, 32)
	if absorbedShadow != 1000 {
		t.Errorf("expected 1000 shadow damage absorbed, got %d", absorbedShadow)
	}
	if remainingShadow != 0 {
		t.Errorf("expected 0 remaining shadow damage, got %d", remainingShadow)
	}
}

func TestManaShield_DrainsMana(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:     1,
			Health:   1000,
			Powers:   [7]uint32{1500}, // 1500 Mana
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	manaShieldID := uint32(1463)
	sess.activeAuras[manaShieldID] = &activeAura{
		SpellID:    manaShieldID,
		AuraType:   SpellAuraManaShield,
		Amount:     2000,
		SchoolMask: 1,
	}

	// 200 damage absorbed requires 200 * 1.5 = 300 mana
	absorbed, remaining := sess.applyAbsorptionShields(200, 1)
	if absorbed != 200 {
		t.Errorf("expected 200 absorbed, got %d", absorbed)
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
	if sess.player.Powers[0] != 1200 {
		t.Errorf("expected 1200 mana remaining (1500 - 300), got %d", sess.player.Powers[0])
	}
}

func TestCreatureAbsorptionShield(t *testing.T) {
	srv := &Server{
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
	}

	creatureGUID := uint64(55555)
	shieldSpellID := uint32(9999)
	srv.activeCreatureAuras[creatureGUID] = map[uint32]*activeAura{
		shieldSpellID: {
			SpellID:    shieldSpellID,
			AuraType:   SpellAuraSchoolAbsorb,
			Amount:     1000,
			SchoolMask: 0,
		},
	}

	absorbed, remaining := srv.applyCreatureAbsorptionShields(creatureGUID, 400, 4) // Fire damage
	if absorbed != 400 {
		t.Errorf("expected 400 absorbed, got %d", absorbed)
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
	if srv.activeCreatureAuras[creatureGUID][shieldSpellID].Amount != 600 {
		t.Errorf("expected 600 remaining shield pool on creature, got %d", srv.activeCreatureAuras[creatureGUID][shieldSpellID].Amount)
	}
}
