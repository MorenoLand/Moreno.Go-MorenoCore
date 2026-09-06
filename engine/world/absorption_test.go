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

func TestAbsorptionShield_SpecificSchoolPriorityOverGeneric(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:   1,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	// Specific Fire Ward (SchoolMask = 4, 500 absorb)
	fireWardID := uint32(543)
	sess.activeAuras[fireWardID] = &activeAura{
		SpellID:    fireWardID,
		AuraType:   SpellAuraSchoolAbsorb,
		Amount:     500,
		SchoolMask: 4, // Fire only
	}

	// Generic Power Word: Shield (SchoolMask = 0, 500 absorb)
	pwsID := uint32(17)
	sess.activeAuras[pwsID] = &activeAura{
		SpellID:    pwsID,
		AuraType:   SpellAuraSchoolAbsorb,
		Amount:     500,
		SchoolMask: 0, // All schools
	}

	// Fire damage 300 (schoolMask = 4)
	absorbed, remaining := sess.applyAbsorptionShields(300, 4)
	if absorbed != 300 || remaining != 0 {
		t.Fatalf("expected 300 absorbed 0 remaining, got %d, %d", absorbed, remaining)
	}

	// Fire Ward MUST absorb first (reduced to 200)
	if sess.activeAuras[fireWardID].Amount != 200 {
		t.Fatalf("expected Fire Ward reduced to 200, got %d", sess.activeAuras[fireWardID].Amount)
	}
	// Power Word: Shield MUST remain untouched at 500
	if sess.activeAuras[pwsID].Amount != 500 {
		t.Fatalf("expected PW:S to remain at 500, got %d", sess.activeAuras[pwsID].Amount)
	}
}

func TestAbsorptionShield_GenericPriorityOverManaShield(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:   1,
			Health: 1000,
			Powers: [7]uint32{1000}, // 1000 mana
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	// Generic Power Word: Shield (500 absorb)
	pwsID := uint32(17)
	sess.activeAuras[pwsID] = &activeAura{
		SpellID:    pwsID,
		AuraType:   SpellAuraSchoolAbsorb,
		Amount:     500,
		SchoolMask: 0,
	}

	// Mana Shield (500 absorb)
	manaShieldID := uint32(1463)
	sess.activeAuras[manaShieldID] = &activeAura{
		SpellID:    manaShieldID,
		AuraType:   SpellAuraManaShield,
		Amount:     500,
		SchoolMask: 1,
	}

	// Physical damage 300 (schoolMask = 1)
	absorbed, remaining := sess.applyAbsorptionShields(300, 1)
	if absorbed != 300 || remaining != 0 {
		t.Fatalf("expected 300 absorbed 0 remaining, got %d, %d", absorbed, remaining)
	}

	// Power Word: Shield MUST absorb first (reduced to 200)
	if sess.activeAuras[pwsID].Amount != 200 {
		t.Fatalf("expected PW:S reduced to 200, got %d", sess.activeAuras[pwsID].Amount)
	}
	// Mana Shield MUST remain untouched at 500
	if sess.activeAuras[manaShieldID].Amount != 500 {
		t.Fatalf("expected Mana Shield untouched at 500, got %d", sess.activeAuras[manaShieldID].Amount)
	}
	// Player mana MUST remain untouched at 1000
	if sess.player.Powers[0] != 1000 {
		t.Fatalf("expected player mana untouched at 1000, got %d", sess.player.Powers[0])
	}
}

func TestAbsorptionShield_AMSPriorityOverGeneric(t *testing.T) {
	sess := &session{
		player: &playerState{
			GUID:   1,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	// Anti-Magic Shell (1000 absorb)
	amsID := uint32(48707)
	sess.activeAuras[amsID] = &activeAura{
		SpellID:    amsID,
		AuraType:   SpellAuraMagicAbsorb,
		Amount:     1000,
		SchoolMask: 0,
	}

	// Power Word: Shield (500 absorb)
	pwsID := uint32(17)
	sess.activeAuras[pwsID] = &activeAura{
		SpellID:    pwsID,
		AuraType:   SpellAuraSchoolAbsorb,
		Amount:     500,
		SchoolMask: 0,
	}

	// Shadow damage 400 (schoolMask = 32)
	absorbed, remaining := sess.applyAbsorptionShields(400, 32)
	if absorbed != 400 || remaining != 0 {
		t.Fatalf("expected 400 absorbed 0 remaining, got %d, %d", absorbed, remaining)
	}

	// AMS MUST absorb first (reduced to 600)
	if sess.activeAuras[amsID].Amount != 600 {
		t.Fatalf("expected AMS reduced to 600, got %d", sess.activeAuras[amsID].Amount)
	}
	// PW:S MUST remain untouched at 500
	if sess.activeAuras[pwsID].Amount != 500 {
		t.Fatalf("expected PW:S untouched at 500, got %d", sess.activeAuras[pwsID].Amount)
	}
}
