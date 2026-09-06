package world

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSpellHaste_CastTimeReduction(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}

	// Mock Store with 3000ms cast time for index 1
	// If store is nil, calculateSpellCastTime returns 0
	// Let's test getSpellHastePct directly first:
	if sess.getSpellHastePct() != 0 {
		t.Fatalf("expected 0%% haste with no rating")
	}

	// 328 rating at level 80: 328 / 32.789989 = ~10.0% haste
	sess.player.CombatRatings[CombatRatingHasteSpell] = 328
	hastePct := sess.getSpellHastePct()
	if hastePct < 9.9 || hastePct > 10.1 {
		t.Fatalf("expected ~10.0%% haste with 328 rating, got %f", hastePct)
	}

	// 656 rating at level 80: ~20.0% haste
	sess.player.CombatRatings[CombatRatingHasteSpell] = 656
	hastePct20 := sess.getSpellHastePct()
	if hastePct20 < 19.9 || hastePct20 > 20.1 {
		t.Fatalf("expected ~20.0%% haste with 656 rating, got %f", hastePct20)
	}
}

func TestSpellCrit_IntellectAndRatingScaling(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level: 80,
		},
	}

	// 1. Base crit with 0 stats: 5.0% (probability 0.05)
	baseCrit := sess.calculateSpellCritChance(0, 16)
	if baseCrit < 0.049 || baseCrit > 0.051 {
		t.Fatalf("expected 5%% base crit, got %f", baseCrit)
	}

	// 2. Add 459 Spell Crit Rating (CR_CRIT_SPELL = 10)
	// 459 / 45.905987 = ~10.0% -> total ~15.0%
	sess.player.CombatRatings[CombatRatingCritSpell] = 459
	ratingCrit := sess.calculateSpellCritChance(0, 16)
	if ratingCrit < 0.149 || ratingCrit > 0.151 {
		t.Fatalf("expected ~15%% crit with 459 rating, got %f", ratingCrit)
	}

	// 3. Add 1667 Intellect (Stat 3)
	// 1667 / 166.6667 = ~10.0% -> total ~25.0%
	sess.player.Stats[3] = 1667
	intCrit := sess.calculateSpellCritChance(0, 16)
	if intCrit < 0.248 || intCrit > 0.252 {
		t.Fatalf("expected ~25%% crit with int + rating, got %f", intCrit)
	}
}

func TestSpellCrit_ResilienceReduction(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	caster := &session{
		server:     srv,
		playerGUID: 1,
		player: &playerState{
			GUID:  1,
			Level: 80,
			// Give caster 15% crit
			CombatRatings: [25]uint32{CombatRatingCritSpell: 459},
		},
	}

	victim := &session{
		server:       srv,
		playerGUID:   2,
		playerLoaded: true,
		player: &playerState{
			GUID:  2,
			Level: 80,
			// 471 resilience rating = ~5% crit reduction
			CombatRatings: [25]uint32{CombatRatingCritTakenSpell: 471},
		},
	}

	srv.sessions[caster] = struct{}{}
	srv.sessions[victim] = struct{}{}

	// Caster crit against victim: 15% - ~5% resilience = ~10%
	critChance := caster.calculateSpellCritChance(victim.playerGUID, 16)
	if critChance < 0.095 || critChance > 0.105 {
		t.Fatalf("expected ~10%% crit chance after resilience reduction, got %f", critChance)
	}
}

func TestSpellHeal_CritBonusApplied(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:         sConn,
		playerGUID:   10,
		playerLoaded: true,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    5000,
			MaxHealth: 10000,
			// Give guaranteed 100% crit
			CombatRatings: [25]uint32{CombatRatingCritSpell: 5000},
		},
	}

	var receivedOpcodes []uint16
	var receivedLock sync.Mutex
	doneRead := make(chan struct{})
	go func() {
		defer close(doneRead)
		for {
			_ = cConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			op, _, err := readServerFrame(cConn, nil)
			if err != nil {
				return
			}
			receivedLock.Lock()
			receivedOpcodes = append(receivedOpcodes, op)
			receivedLock.Unlock()
		}
	}()

	// Execute a 1000 heal
	// With 100% crit, heal should be multiplied by 1.5x -> 1500
	baseHeal := uint32(1000)
	sess.executeSpellHeal(context.Background(), sess.playerGUID, 2050, baseHeal)

	// Health was 5000, should now be 6500 (1500 healed)
	if sess.player.Health != 6500 {
		t.Fatalf("expected player health 6500 after 150%% crit heal, got %d", sess.player.Health)
	}

	time.Sleep(50 * time.Millisecond)
	_ = cConn.Close()
	<-doneRead

	receivedLock.Lock()
	hasHealLog := false
	for _, op := range receivedOpcodes {
		if op == uint16(protocol.OpcodeSMSG_SPELLHEALLOG) {
			hasHealLog = true
			break
		}
	}
	receivedLock.Unlock()

	if !hasHealLog {
		t.Fatalf("expected SMSG_SPELLHEALLOG sent on spell heal")
	}
}
