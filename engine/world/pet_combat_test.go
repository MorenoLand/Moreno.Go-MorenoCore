package world

import (
	"context"
	"testing"
	"time"
)

func TestPetCommand_AttackFollowStay(t *testing.T) {
	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
	}

	ownerGUID := uint64(100)
	petGUID := uint64(0xF140000000000001)
	targetGUID := uint64(0xF130000000000002)

	motion := &creatureMotion{
		GUID:        petGUID,
		Health:      1000,
		MaxHealth:   1000,
		OwnerGUID:   ownerGUID,
		PetCommand:  PetCommandFollow,
		PetReact:    PetReactDefensive,
		AttackTime:  2000,
	}
	srv.creatureMotion[petGUID] = motion

	// 1. Order Attack
	srv.onPetCommandAttack(petGUID, targetGUID)
	if motion.TargetGUID != targetGUID {
		t.Errorf("expected pet target %d, got %d", targetGUID, motion.TargetGUID)
	}
	if !motion.InCombat {
		t.Errorf("expected pet InCombat to be true")
	}
	if motion.PetCommand != PetCommandAttack {
		t.Errorf("expected pet command Attack (%d), got %d", PetCommandAttack, motion.PetCommand)
	}

	// 2. Order Follow (Recall)
	srv.onPetCommandFollow(petGUID)
	if motion.TargetGUID != 0 {
		t.Errorf("expected pet target cleared (0), got %d", motion.TargetGUID)
	}
	if motion.InCombat {
		t.Errorf("expected pet InCombat to be false")
	}
	if motion.PetCommand != PetCommandFollow {
		t.Errorf("expected pet command Follow (%d), got %d", PetCommandFollow, motion.PetCommand)
	}

	// 3. Order Stay
	srv.onPetCommandStay(petGUID)
	if motion.TargetGUID != 0 {
		t.Errorf("expected pet target cleared (0), got %d", motion.TargetGUID)
	}
	if motion.InCombat {
		t.Errorf("expected pet InCombat to be false")
	}
	if motion.PetCommand != PetCommandStay {
		t.Errorf("expected pet command Stay (%d), got %d", PetCommandStay, motion.PetCommand)
	}
}

func TestPetDefensive_TriggerOnAggro(t *testing.T) {
	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
	}

	ownerGUID := uint64(100)
	petGUID := uint64(0xF140000000000001)
	enemyGUID := uint64(0xF130000000000002)

	// Pet in Defensive mode
	motion := &creatureMotion{
		GUID:        petGUID,
		Health:      1000,
		MaxHealth:   1000,
		OwnerGUID:   ownerGUID,
		PetCommand:  PetCommandFollow,
		PetReact:    PetReactDefensive,
		AttackTime:  2000,
	}
	srv.creatureMotion[petGUID] = motion

	// Owner is attacked by enemy
	srv.triggerPetDefensive(ownerGUID, enemyGUID)

	if motion.TargetGUID != enemyGUID {
		t.Errorf("expected pet to acquire enemy %d, got %d", enemyGUID, motion.TargetGUID)
	}
	if !motion.InCombat {
		t.Errorf("expected pet to enter combat")
	}

	// Now switch to Passive mode
	srv.onPetSetReaction(petGUID, PetReactPassive)
	srv.onPetCommandFollow(petGUID)

	anotherEnemy := uint64(0xF130000000000003)
	srv.triggerPetDefensive(ownerGUID, anotherEnemy)

	if motion.TargetGUID != 0 {
		t.Errorf("expected passive pet to ignore defensive aggro, but got target %d", motion.TargetGUID)
	}
	if motion.InCombat {
		t.Errorf("expected passive pet to remain out of combat")
	}
}

func TestPetMeleeAttack_DamageApplication(t *testing.T) {
	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
	}

	petGUID := uint64(0xF140000000000001)
	targetGUID := uint64(0xF130000000000002)

	petMotion := &creatureMotion{
		GUID:       petGUID,
		Level:      80,
		MinDamage:  100,
		MaxDamage:  100,
		AttackTime: 2000,
		Health:     5000,
		MaxHealth:  5000,
	}
	targetMotion := &creatureMotion{
		GUID:      targetGUID,
		Level:     80,
		Health:    1000,
		MaxHealth: 1000,
		Armor:     0, // 0 armor for deterministic damage
	}

	srv.creatureMotion[petGUID] = petMotion
	srv.creatureMotion[targetGUID] = targetMotion

	now := time.Now()
	for i := 0; i < 5 && targetMotion.Health == 1000; i++ {
		srv.executePetMeleeAttack(context.Background(), petMotion, targetGUID, false, nil, targetMotion.Health, targetMotion.Armor, uint8(targetMotion.Level), now)
	}

	if targetMotion.Health >= 1000 {
		t.Errorf("expected target health to decrease from 1000, got %d", targetMotion.Health)
	}
}

func TestPetAutocast_ExecutionAndDamage(t *testing.T) {
	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
	}

	petGUID := uint64(0xF140000000000001)
	targetGUID := uint64(0xF130000000000002)

	petMotion := &creatureMotion{
		GUID:           petGUID,
		Level:          80,
		Health:         5000,
		MaxHealth:      5000,
		AutocastSpells: []uint32{47964}, // Firebolt
	}
	targetMotion := &creatureMotion{
		GUID:      targetGUID,
		Level:     80,
		Health:    1000,
		MaxHealth: 1000,
	}

	srv.creatureMotion[petGUID] = petMotion
	srv.creatureMotion[targetGUID] = targetMotion

	now := time.Now()
	srv.executePetAutocast(context.Background(), petMotion, targetGUID, now)

	expectedDmg := uint32(80*2 + 10) // 170
	expectedHealth := uint32(1000 - expectedDmg)
	if targetMotion.Health != expectedHealth {
		t.Errorf("expected target health %d after autocast spell, got %d", expectedHealth, targetMotion.Health)
	}
	if petMotion.LastSpell != now {
		t.Errorf("expected pet LastSpell timestamp to update to %v", now)
	}

	// Test autocast toggle off
	srv.onPetToggleAutocast(petGUID, 47964, false)
	if len(petMotion.AutocastSpells) != 0 {
		t.Errorf("expected autocast spells empty after toggle off, got %v", petMotion.AutocastSpells)
	}

	// Test autocast toggle on
	srv.onPetToggleAutocast(petGUID, 47964, true)
	if len(petMotion.AutocastSpells) != 1 || petMotion.AutocastSpells[0] != 47964 {
		t.Errorf("expected autocast spell 47964 re-added, got %v", petMotion.AutocastSpells)
	}
}
