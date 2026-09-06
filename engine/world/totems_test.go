package world

import (
	"context"
	"testing"
)

func TestTotem_SummonAndBuffPulse(t *testing.T) {
	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		activeTotems:   make(map[uint64][4]*activeTotem),
	}

	playerGUID := uint64(501)
	sess := &session{
		server:       srv,
		playerGUID:   playerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID:       playerGUID,
			Name:       "Farseer",
			Level:      80,
			Map:        0,
			X:          100.0,
			Y:          100.0,
			Z:          10.0,
			Health:     10000,
			MaxHealth:  10000,
			TotemSlots: [4]uint64{},
		},
	}
	srv.sessions[sess] = struct{}{}

	ctx := context.Background()

	// 1. Cast Stoneskin Totem (Spell 8071, Earth slot 0)
	sess.summonTotem(ctx, 8071)

	totemGUID := sess.player.TotemSlots[TotemSlotEarth]
	if totemGUID == 0 {
		t.Fatal("expected slot 0 to have totem GUID, got 0")
	}

	// Verify Stoneskin Buff aura (8072) was applied
	if !sess.hasAura(8072) {
		t.Fatal("expected Stoneskin buff aura (8072) on player")
	}

	// Verify totem creature exists in creatureMotion
	srv.motionMu.Lock()
	motion := srv.creatureMotion[totemGUID]
	srv.motionMu.Unlock()
	if motion == nil {
		t.Fatalf("expected totem creature in creatureMotion for GUID %d", totemGUID)
	}

	// 2. Summon Strength of Earth Totem (Spell 8075, replaces Earth slot 0)
	sess.summonTotem(ctx, 8075)

	newTotemGUID := sess.player.TotemSlots[TotemSlotEarth]
	if newTotemGUID == 0 || newTotemGUID == totemGUID {
		t.Fatalf("expected new totem GUID in slot 0, got %d (old was %d)", newTotemGUID, totemGUID)
	}

	// Old buff (8072) should be removed, new buff (8076) applied
	if sess.hasAura(8072) {
		t.Fatal("expected old Stoneskin buff (8072) to be removed")
	}
	if !sess.hasAura(8076) {
		t.Fatal("expected new Strength of Earth buff (8076) to be applied")
	}

	// Old creature should be removed from creatureMotion
	srv.motionMu.Lock()
	oldMotion := srv.creatureMotion[totemGUID]
	newMotion := srv.creatureMotion[newTotemGUID]
	srv.motionMu.Unlock()
	if oldMotion != nil {
		t.Fatal("expected old totem creature to be removed from creatureMotion")
	}
	if newMotion == nil {
		t.Fatal("expected new totem creature in creatureMotion")
	}

	// 3. Destroy totem via CMSG_TOTEM_DESTROYED
	destroyPayload := []byte{TotemSlotEarth}
	sess.handleTotemDestroyed(ctx, destroyPayload)

	if sess.player.TotemSlots[TotemSlotEarth] != 0 {
		t.Fatalf("expected slot 0 to be cleared, got %d", sess.player.TotemSlots[TotemSlotEarth])
	}
	if sess.hasAura(8076) {
		t.Fatal("expected Strength of Earth buff to be removed after totem destruction")
	}
}

func TestTotem_TotemicRecall(t *testing.T) {
	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		activeTotems:   make(map[uint64][4]*activeTotem),
	}

	playerGUID := uint64(502)
	sess := &session{
		server:       srv,
		playerGUID:   playerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID:       playerGUID,
			Name:       "ShamanLord",
			Level:      80,
			Map:        0,
			X:          200.0,
			Y:          200.0,
			Z:          10.0,
			Health:     10000,
			MaxHealth:  10000,
			TotemSlots: [4]uint64{},
		},
	}
	srv.sessions[sess] = struct{}{}

	ctx := context.Background()

	// Summon 4 totems across all 4 slots
	sess.summonTotem(ctx, 8071) // Earth: Stoneskin
	sess.summonTotem(ctx, 8227) // Fire: Flametongue
	sess.summonTotem(ctx, 5675) // Water: Mana Spring
	sess.summonTotem(ctx, 8512) // Air: Windfury

	for slot := 0; slot < 4; slot++ {
		if sess.player.TotemSlots[slot] == 0 {
			t.Fatalf("expected slot %d to be populated", slot)
		}
	}

	// Buffs should be present
	if !sess.hasAura(8072) || !sess.hasAura(52109) || !sess.hasAura(5677) || !sess.hasAura(8515) {
		t.Fatal("expected all 4 totem buffs to be present")
	}

	// Cast Totemic Recall (36936)
	sess.destroyAllTotems()

	for slot := 0; slot < 4; slot++ {
		if sess.player.TotemSlots[slot] != 0 {
			t.Fatalf("expected slot %d to be cleared after Totemic Recall, got %d", slot, sess.player.TotemSlots[slot])
		}
	}

	// All buffs should be removed
	if sess.hasAura(8072) || sess.hasAura(52109) || sess.hasAura(5677) || sess.hasAura(8515) {
		t.Fatal("expected all 4 totem buffs to be removed after Totemic Recall")
	}
}

func TestTotem_HealingStreamPulse(t *testing.T) {
	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		activeTotems:   make(map[uint64][4]*activeTotem),
	}

	playerGUID := uint64(503)
	sess := &session{
		server:       srv,
		playerGUID:   playerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID:       playerGUID,
			Name:       "RestoShaman",
			Level:      80,
			Map:        0,
			X:          300.0,
			Y:          300.0,
			Z:          10.0,
			Health:     5000,
			MaxHealth:  10000,
			TotemSlots: [4]uint64{},
		},
	}
	srv.sessions[sess] = struct{}{}

	ctx := context.Background()

	// Cast Healing Stream Totem (Spell 5394, Water slot 2)
	sess.summonTotem(ctx, 5394)

	if sess.player.TotemSlots[TotemSlotWater] == 0 {
		t.Fatal("expected water slot to have totem GUID")
	}

	// Initial pulse should have healed player for 50 HP (from 5000 to 5050)
	if sess.player.Health <= 5000 {
		t.Fatalf("expected player health > 5000 after Healing Stream pulse, got %d", sess.player.Health)
	}

	sess.destroyTotem(TotemSlotWater)
	if sess.player.TotemSlots[TotemSlotWater] != 0 {
		t.Fatal("expected water slot to be cleared")
	}
}
