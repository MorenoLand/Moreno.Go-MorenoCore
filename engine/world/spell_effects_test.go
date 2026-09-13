package world

import (
	"context"
	"testing"
)

func TestSpellEnergizeClampsToMaximumPower(t *testing.T) {
	sess := &session{playerGUID: 1, player: &playerState{GUID: 1, Health: 100, Powers: [7]uint32{90}, MaxPowers: [7]uint32{100}}}
	sess.applySpellEnergize(context.Background(), 1, 0, 25)
	if sess.player.Powers[0] != 100 {
		t.Fatalf("expected mana to clamp at 100, got %d", sess.player.Powers[0])
	}
}

func TestSpellPowerBurnDrainsMatchingPower(t *testing.T) {
	sess := &session{playerGUID: 1, player: &playerState{GUID: 1, Class: 8, Health: 100, Powers: [7]uint32{100}, MaxPowers: [7]uint32{100}}}
	if burned := sess.applySpellPowerBurn(context.Background(), 1, 0, 25, 1000); burned != 25 {
		t.Fatalf("expected 25 burned, got %d", burned)
	}
	if sess.player.Powers[0] != 75 {
		t.Fatalf("expected 75 mana remaining, got %d", sess.player.Powers[0])
	}
}

func TestSpellMaxHealthHealRestoresLivingPlayer(t *testing.T) {
	sess := &session{playerGUID: 1, player: &playerState{GUID: 1, Health: 25, MaxHealth: 100}}
	sess.executeSpellMaxHealthHeal(context.Background(), 1, 1000)
	if sess.player.Health != 100 {
		t.Fatalf("expected max-health heal to restore 100 health, got %d", sess.player.Health)
	}
}

func TestSpellMaxHealthHealDoesNotResurrect(t *testing.T) {
	sess := &session{playerGUID: 1, player: &playerState{GUID: 1, Health: 0, MaxHealth: 100}}
	sess.executeSpellMaxHealthHeal(context.Background(), 1, 1000)
	if sess.player.Health != 0 {
		t.Fatalf("expected max-health heal to leave dead player at 0 health, got %d", sess.player.Health)
	}
}

func TestSpellThreatEffectAddsCreatureThreat(t *testing.T) {
	creatureGUID := creatureWorldGUID(7, 123)
	server := &Server{sessions: make(map[*session]struct{}), creatureMotion: map[uint64]*creatureMotion{creatureGUID: {GUID: creatureGUID, Map: 0, Health: 100, MaxHealth: 100, X: 4, Y: 0, Z: 0}}}
	sess := &session{server: server, playerGUID: 1, player: &playerState{GUID: 1, Health: 100, MaxHealth: 100, Map: 0, X: 0, Y: 0, Z: 0}}
	server.sessions[sess] = struct{}{}
	sess.applySpellThreat(context.Background(), creatureGUID, 25)
	server.motionMu.Lock()
	motion := server.creatureMotion[creatureGUID]
	threat := motion.ThreatMgr.GetThreat(sess.playerGUID)
	inCombat, target := motion.InCombat, motion.TargetGUID
	server.motionMu.Unlock()
	if threat != 25 || !inCombat || target != sess.playerGUID {
		t.Fatalf("threat=%v inCombat=%v target=%d", threat, inCombat, target)
	}
}
