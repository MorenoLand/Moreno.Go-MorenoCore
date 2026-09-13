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
