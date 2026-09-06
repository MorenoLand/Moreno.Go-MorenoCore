package world

import (
	"context"
	"testing"
)

func TestThreat_MatchUnitThreatToHighestThreat(t *testing.T) {
	tm := NewThreatManager(100)

	// Target 1 has 500 threat, Target 2 has 200 threat
	tm.AddThreat(1, 500, false)
	tm.AddThreat(2, 200, false)

	if tm.GetCurrentVictim() != 1 {
		t.Fatalf("expected victim 1, got %d", tm.GetCurrentVictim())
	}

	// Target 2 uses Taunt: threat should match highest (500) and victim switches to 2
	switched, newVictim := tm.MatchUnitThreatToHighestThreat(2)
	if !switched || newVictim != 2 {
		t.Fatalf("expected switched to victim 2, got switched=%v victim=%d", switched, newVictim)
	}
	if tm.GetThreat(2) != 500 {
		t.Fatalf("expected target 2 threat to match 500, got %f", tm.GetThreat(2))
	}
	if tm.GetCurrentVictim() != 2 {
		t.Fatalf("expected current victim 2, got %d", tm.GetCurrentVictim())
	}
}

func TestThreat_IsTauntSpell(t *testing.T) {
	taunts := []uint32{355, 694, 1161, 6795, 5209, 56222, 62124, 31789}
	for _, spellID := range taunts {
		if !isTauntSpell(spellID) {
			t.Fatalf("expected spell %d to be recognized as taunt", spellID)
		}
	}
	nonTaunts := []uint32{133, 116, 2098, 47897}
	for _, spellID := range nonTaunts {
		if isTauntSpell(spellID) {
			t.Fatalf("unexpected taunt classification for spell %d", spellID)
		}
	}
}

func TestThreat_ThreatMultipliers(t *testing.T) {
	sess := &session{
		player:      &playerState{Level: 80},
		activeAuras: make(map[uint32]*activeAura),
	}

	// Default baseline = 1.0
	if mult := sess.getThreatMultiplier(1); mult != 1.0 {
		t.Fatalf("expected 1.0 default threat multiplier, got %f", mult)
	}

	// Defensive Stance (71) -> 1.45x
	sess.activeAuras[71] = &activeAura{SpellID: 71}
	if mult := sess.getThreatMultiplier(1); mult < 1.44 || mult > 1.46 {
		t.Fatalf("expected 1.45 Defensive Stance multiplier, got %f", mult)
	}

	// Bear Form (5487) -> 1.30x
	delete(sess.activeAuras, 71)
	sess.activeAuras[5487] = &activeAura{SpellID: 5487}
	if mult := sess.getThreatMultiplier(1); mult < 1.29 || mult > 1.31 {
		t.Fatalf("expected 1.30 Bear Form multiplier, got %f", mult)
	}

	// Righteous Fury (25780): Holy damage (schoolMask 2) -> 1.80x, Physical -> 1.0x
	delete(sess.activeAuras, 5487)
	sess.activeAuras[25780] = &activeAura{SpellID: 25780}
	if mult := sess.getThreatMultiplier(2); mult < 1.79 || mult > 1.81 {
		t.Fatalf("expected 1.80 Righteous Fury holy multiplier, got %f", mult)
	}
	if mult := sess.getThreatMultiplier(1); mult != 1.0 {
		t.Fatalf("expected 1.0 Righteous Fury physical multiplier, got %f", mult)
	}

	// Generic SPELL_AURA_MOD_THREAT (AuraType 10)
	delete(sess.activeAuras, 25780)
	sess.activeAuras[999] = &activeAura{
		SpellID:  999,
		AuraType: 10,
		Amount:   50, // +50% threat
	}
	if mult := sess.getThreatMultiplier(1); mult < 1.49 || mult > 1.51 {
		t.Fatalf("expected 1.50 multiplier from SPELL_AURA_MOD_THREAT, got %f", mult)
	}
}

func TestThreat_HandleEffectTaunt(t *testing.T) {
	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
	}
	creatureGUID := uint64(5000)
	motion := &creatureMotion{
		GUID:      creatureGUID,
		ThreatMgr: NewThreatManager(creatureGUID),
	}
	srv.creatureMotion[creatureGUID] = motion

	// Player 1 has 300 threat
	motion.ThreatMgr.AddThreat(1, 300, false)

	// Player 2 uses Taunt
	sess2 := &session{
		server:     srv,
		playerGUID: 2,
		player:     &playerState{Level: 80},
	}
	sess2.handleEffectTaunt(context.Background(), creatureGUID, 355)

	if motion.TargetGUID != 2 {
		t.Fatalf("expected creature target to be taunter 2, got %d", motion.TargetGUID)
	}
	if motion.ThreatMgr.GetThreat(2) != 300 {
		t.Fatalf("expected taunter threat to match 300, got %f", motion.ThreatMgr.GetThreat(2))
	}
}
