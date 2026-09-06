package world

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCreatureEvadeReturnAndImmunity(t *testing.T) {
	ctx := context.Background()

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	// Player session
	playerState := &playerState{
		GUID:        1,
		Map:         0,
		X:           50.0,
		Y:           0.0,
		Z:           0.0,
		Orientation: 0.0,
		Level:       80,
		Health:      1000,
		MaxHealth:   1000,
		AttackTime:  2000,
	}
	sess := &session{
		conn:         serverConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       playerState,
	}

	creatureGUID := creatureWorldGUID(100, 68)
	motion := &creatureMotion{
		GUID:        creatureGUID,
		Entry:       68,
		Map:         0,
		HomeX:       0.0,
		HomeY:       0.0,
		HomeZ:       0.0,
		X:           30.0,
		Y:           0.0,
		Z:           0.0,
		Orientation: 0.0,
		Faction:     14, // Hostile
		Level:       80,
		Health:      5000,
		MaxHealth:   5000,
		Speed:       2.5,
		RunSpeed:    7.0,
		InCombat:    true,
		TargetGUID:  1,
		ThreatMgr:   NewThreatManager(creatureGUID),
	}
	motion.ThreatMgr.AddThreat(1, 1000.0, false)

	server := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
	}
	server.sessions[sess] = struct{}{}
	server.creatureMotion[creatureGUID] = motion
	sess.server = server

	// Drain frames in background
	stopReader := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopReader:
				return
			default:
				_ = clientConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
				_, _, err := readServerFrame(clientConn, nil)
				if err != nil {
					return
				}
			}
		}
	}()
	defer close(stopReader)

	now := time.Now()

	// 1. Target moves far away (> 45yd): dist between (30, 0) and (80, 0) = 50yd > 45yd
	playerState.X = 80.0
	players := []playerPos{{
		Map:             0,
		X:               80.0,
		Y:               0.0,
		Z:               0.0,
		GUID:            1,
		Level:           80,
		FactionTemplate: 1,
		Sess:            sess,
	}}

	server.stepCreatureMotion(ctx, motion, players, now)

	// Creature must now be evading
	if !motion.Evading {
		t.Fatal("expected creature to enter Evade mode")
	}
	if motion.InCombat {
		t.Fatal("expected creature to clear InCombat on evade")
	}
	if motion.TargetGUID != 0 {
		t.Fatalf("expected creature target to be 0, got %d", motion.TargetGUID)
	}
	if !server.isCreatureEvading(creatureGUID) {
		t.Fatal("server.isCreatureEvading should return true")
	}

	target := combatTarget{
		GUID:   creatureGUID,
		Map:    0,
		X:      motion.X,
		Y:      motion.Y,
		Z:      motion.Z,
		Health: motion.Health,
		Level:  80,
	}

	// 2. While evading, melee attacks must roll Evade (damage = 0, no aggro)
	initialHealth := motion.Health
	sess.executeMeleeSwing(ctx, target, protocol.BaseAttack)
	if motion.Health != initialHealth {
		t.Fatalf("creature took damage while evading: %d -> %d", initialHealth, motion.Health)
	}
	if motion.InCombat {
		t.Fatal("creature should not enter combat from melee attack while evading")
	}

	// 3. While evading, ranged attacks must roll Evade (damage = 0, no aggro)
	playerState.MinRangedDamage = 50
	playerState.MaxRangedDamage = 100
	sess.executeRangedAttack(ctx, target, 75)
	if motion.Health != initialHealth {
		t.Fatalf("creature took damage from ranged attack while evading: %d -> %d", initialHealth, motion.Health)
	}
	if motion.InCombat {
		t.Fatal("creature should not enter combat from ranged attack while evading")
	}

	// 4. While evading, direct spell damage must miss/evade (damage = 0, no aggro)
	sess.executeDirectSpellDamage(ctx, creatureGUID, 133, 500, 4) // Fireball 500 Fire damage
	if motion.Health != initialHealth {
		t.Fatalf("creature took damage from spell while evading: %d -> %d", initialHealth, motion.Health)
	}
	if motion.InCombat {
		t.Fatal("creature should not enter combat from spell while evading")
	}

	// 5. While evading, auras cannot be applied
	moonfire := wotlk.Spell{
		ID:         8921,
		SchoolMask: 8, // Nature
	}
	dotEff := wotlk.SpellEffect{
		Effect:     6,  // SPELL_EFFECT_APPLY_AURA
		Aura:       3,  // SPELL_AURA_PERIODIC_DAMAGE
		BasePoints: 50, // 51 dmg per tick
		AuraPeriod: 3000,
	}
	sess.applyAuraToTarget(ctx, creatureGUID, moonfire, dotEff, 12000, 3000, 51, 8)
	server.auraMu.Lock()
	aurasCount := len(server.activeCreatureAuras[creatureGUID])
	server.auraMu.Unlock()
	if aurasCount != 0 {
		t.Fatalf("expected 0 active creature auras while evading, got %d", aurasCount)
	}

	// 6. While evading, taunts have no effect
	sess.handleEffectTaunt(ctx, creatureGUID, 355) // Warrior: Taunt
	if motion.InCombat {
		t.Fatal("creature should not enter combat from Taunt while evading")
	}

	// 7. While evading, hostile players within 15yd do NOT trigger aggro
	playersNear := []playerPos{{
		Map:             0,
		X:               2.0,
		Y:               0.0,
		Z:               0.0,
		GUID:            1,
		Level:           80,
		FactionTemplate: 1, // Hostile to Monster
		Sess:            sess,
	}}
	// stepCreatureMotion during return path
	server.stepCreatureMotion(ctx, motion, playersNear, now.Add(100*time.Millisecond))
	if motion.InCombat {
		t.Fatal("creature should not aggro on nearby player while still evading")
	}
	if !motion.Evading {
		t.Fatal("creature should still be in Evading state")
	}

	// 8. Creature completes return to home position (now >= motion.MoveEnds)
	afterArrival := motion.MoveEnds.Add(500 * time.Millisecond)
	server.stepCreatureMotion(ctx, motion, playersNear, afterArrival)

	if motion.Evading {
		t.Fatal("expected Evading to be false after arriving home")
	}
	if motion.X != motion.HomeX || motion.Y != motion.HomeY || motion.Z != motion.HomeZ {
		t.Fatalf("expected creature at home (%f,%f,%f), got (%f,%f,%f)",
			motion.HomeX, motion.HomeY, motion.HomeZ, motion.X, motion.Y, motion.Z)
	}

	// 9. Now that creature is home and no longer evading, hostile player within 15yd triggers aggro
	server.stepCreatureMotion(ctx, motion, playersNear, afterArrival.Add(100*time.Millisecond))
	if !motion.InCombat {
		t.Fatal("expected creature to enter combat once home and no longer evading")
	}
	if motion.TargetGUID != 1 {
		t.Fatalf("expected creature target to be 1, got %d", motion.TargetGUID)
	}
}
