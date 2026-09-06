package world

import (
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
)

func TestPushback_AbortOnDirectDamage(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:   sConn,
		player: &playerState{Level: 80},
	}
	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	timer := time.NewTimer(3 * time.Second)
	sess.castMu.Lock()
	sess.activeCast = &activeCastState{
		CastID:       1,
		SpellID:      12051, // Evocation (or Bandage with abort on damage)
		Timer:        timer,
		StartAt:      time.Now(),
		CastTimeMs:   8000,
		InterruptFlg: spellInterruptAbortOnDmg, // 0x10
	}
	sess.castMu.Unlock()

	// Taking direct damage should immediately abort the cast
	sess.delayCurrentCast()

	sess.castMu.Lock()
	active := sess.activeCast
	sess.castMu.Unlock()

	if active != nil {
		t.Fatalf("expected cast to be aborted, got %v", active)
	}
}

func TestPushback_AuraReduction(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:        sConn,
		player:      &playerState{Level: 80},
		activeAuras: make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// 1. With 35% pushback reduction (Concentration Aura)
	sess.castMu.Lock()
	sess.activeAuras[19746] = &activeAura{
		SpellID:  19746,
		AuraType: 149, // SPELL_AURA_REDUCE_PUSHBACK
		Amount:   35,  // 35% reduction -> 500ms becomes 325ms delay
	}
	timer := time.NewTimer(5 * time.Second)
	sess.activeCast = &activeCastState{
		CastID:       1,
		SpellID:      2000,
		Timer:        timer,
		StartAt:      time.Now(),
		CastTimeMs:   5000,
		InterruptFlg: spellInterruptPushBack,
	}
	sess.castMu.Unlock()

	sess.delayCurrentCast()

	sess.castMu.Lock()
	pushbacks := sess.activeCast.Pushbacks
	startAt := sess.activeCast.StartAt
	sess.castMu.Unlock()

	if pushbacks != 1 {
		t.Fatalf("expected pushback count 1, got %d", pushbacks)
	}
	// Normal 500ms delay shifts StartAt by -500ms. With 35% reduction, shift is ~325ms.
	elapsed := time.Since(startAt)
	if elapsed > -250*time.Millisecond || elapsed < -400*time.Millisecond {
		t.Fatalf("expected startAt shifted by ~325ms, got elapsed %v", elapsed)
	}

	// 2. With 100% pushback reduction (e.g. talents + aura = immune to pushback)
	sess.castMu.Lock()
	sess.activeAuras[19746].Amount = 100
	sess.activeCast.Pushbacks = 0
	origStartAt := sess.activeCast.StartAt
	sess.castMu.Unlock()

	sess.delayCurrentCast()

	sess.castMu.Lock()
	pushbacks = sess.activeCast.Pushbacks
	newStartAt := sess.activeCast.StartAt
	sess.castMu.Unlock()

	if pushbacks != 0 {
		t.Fatalf("expected 0 pushbacks with 100%% reduction, got %d", pushbacks)
	}
	if newStartAt != origStartAt {
		t.Fatalf("expected StartAt unchanged with 100%% reduction")
	}
}

func TestPushback_ChannelAuraReduction(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:        sConn,
		player:      &playerState{Level: 80},
		activeAuras: make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// 50% pushback reduction
	sess.castMu.Lock()
	sess.activeAuras[100] = &activeAura{
		SpellID:  100,
		AuraType: 149,
		Amount:   50,
	}
	timer := time.NewTimer(4 * time.Second)
	sess.activeChannel = &activeChannelState{
		CastID:     1,
		SpellID:    10,
		DurationMs: 4000,
		Remaining:  4 * time.Second,
		Timer:      timer,
		Spell:      wotlk.Spell{ChannelInterrupt: channelFlagDelay},
	}
	sess.castMu.Unlock()

	sess.delayCurrentChannel()

	sess.castMu.Lock()
	rem := sess.activeChannel.Remaining
	pushbacks := sess.activeChannel.Pushbacks
	sess.castMu.Unlock()

	if pushbacks != 1 {
		t.Fatalf("expected 1 pushback, got %d", pushbacks)
	}
	// Normal 25% of 4000ms = 1000ms. With 50% reduction = 500ms delay.
	// Remaining was 4000ms -> should become 3500ms.
	if rem != 3500*time.Millisecond {
		t.Fatalf("expected remaining 3500ms, got %v", rem)
	}
}
