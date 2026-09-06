package world

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestInterruptCast_ActiveCastCancelled(t *testing.T) {
	sConnA, cConnA := net.Pipe()
	defer sConnA.Close()
	defer cConnA.Close()

	sConnB, cConnB := net.Pipe()
	defer sConnB.Close()
	defer cConnB.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	attacker := &session{
		server:       srv,
		conn:         sConnA,
		playerGUID:   10,
		playerLoaded: true,
		player:       &playerState{GUID: 10, Level: 80},
	}

	target := &session{
		server:       srv,
		conn:         sConnB,
		playerGUID:   20,
		playerLoaded: true,
		player: &playerState{
			GUID:  20,
			Level: 80,
			Spells: []learnedSpell{
				{ID: 116, Active: true}, // Frostbolt (Frost)
				{ID: 122, Active: true}, // Frost Nova (Frost)
				{ID: 133, Active: true}, // Fireball (Fire)
			},
		},
		schoolLockouts: make(map[uint32]int64),
	}

	srv.sessions[attacker] = struct{}{}
	srv.sessions[target] = struct{}{}

	var receivedOpcodes []uint16
	var receivedLock sync.Mutex
	doneRead := make(chan struct{})
	go func() {
		defer close(doneRead)
		for {
			_ = cConnB.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(cConnB, nil)
			if err != nil {
				return
			}
			receivedLock.Lock()
			receivedOpcodes = append(receivedOpcodes, op)
			receivedLock.Unlock()
		}
	}()

	// Target starts casting Frostbolt (116), SchoolMask = 16 (Frost)
	timer := time.NewTimer(3 * time.Second)
	activeCast := &activeCastState{
		CastID:       1,
		SpellID:      116,
		Timer:        timer,
		StartAt:      time.Now(),
		CastTimeMs:   3000,
		InterruptFlg: spellInterruptFlagInterrupt,
	}
	target.castMu.Lock()
	target.activeCast = activeCast
	target.castMu.Unlock()

	// Attacker casts Kick (1766, 5s lockout) on Target
	kickSpell := wotlk.Spell{
		ID: 1766,
	}
	attacker.handleEffectInterruptCast(context.Background(), target.playerGUID, kickSpell, wotlk.SpellEffect{Effect: spellEffectInterruptCast})

	// Verify activeCast was cancelled
	target.castMu.Lock()
	remCast := target.activeCast
	target.castMu.Unlock()

	if remCast != nil {
		t.Fatalf("expected target.activeCast to be nil, got %v", remCast)
	}
	if !activeCast.Cancelled {
		t.Fatalf("expected activeCast.Cancelled to be true")
	}

	// Verify target received SMSG_CAST_FAILED (0x130)
	time.Sleep(100 * time.Millisecond)
	_ = cConnB.Close()
	<-doneRead

	receivedLock.Lock()
	hasCastFailed := false
	for _, op := range receivedOpcodes {
		if op == uint16(protocol.OpcodeSMSG_CAST_FAILED) {
			hasCastFailed = true
			break
		}
	}
	receivedLock.Unlock()

	if !hasCastFailed {
		t.Fatalf("expected SMSG_CAST_FAILED to be sent to target, got opcodes: %v", receivedOpcodes)
	}

	// Verify target's school lockout
	target.castMu.Lock()
	lockoutEnd := target.schoolLockouts[1] // Default schoolMask 1 if DBC is nil, or school mask
	target.castMu.Unlock()
	if lockoutEnd <= time.Now().Unix() {
		t.Fatalf("expected active schoolLockout for target, got end: %d", lockoutEnd)
	}
}

func TestInterruptCast_ActiveChannelCancelled(t *testing.T) {
	sConnA, cConnA := net.Pipe()
	defer sConnA.Close()
	defer cConnA.Close()

	sConnB, cConnB := net.Pipe()
	defer sConnB.Close()
	defer cConnB.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	attacker := &session{
		server:       srv,
		conn:         sConnA,
		playerGUID:   10,
		playerLoaded: true,
		player:       &playerState{GUID: 10, Level: 80},
	}

	target := &session{
		server:       srv,
		conn:         sConnB,
		playerGUID:   20,
		playerLoaded: true,
		player: &playerState{
			GUID:  20,
			Level: 80,
		},
		schoolLockouts: make(map[uint32]int64),
	}

	srv.sessions[attacker] = struct{}{}
	srv.sessions[target] = struct{}{}

	var receivedOpcodes []uint16
	var receivedLock sync.Mutex
	doneRead := make(chan struct{})
	go func() {
		defer close(doneRead)
		for {
			_ = cConnB.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(cConnB, nil)
			if err != nil {
				return
			}
			receivedLock.Lock()
			receivedOpcodes = append(receivedOpcodes, op)
			receivedLock.Unlock()
		}
	}()

	// Target is channeling Arcane Missiles (5143)
	channelTimer := time.NewTimer(5 * time.Second)
	activeChan := &activeChannelState{
		CastID:     2,
		SpellID:    5143,
		Timer:      channelTimer,
		DurationMs: 5000,
	}
	target.castMu.Lock()
	target.activeChannel = activeChan
	target.castMu.Unlock()

	// Attacker interrupts with Counterspell (2139, 8s lockout)
	csSpell := wotlk.Spell{
		ID: 2139,
	}
	attacker.handleEffectInterruptCast(context.Background(), target.playerGUID, csSpell, wotlk.SpellEffect{Effect: spellEffectInterruptCast})

	// Verify activeChannel was cleared and stopped
	target.castMu.Lock()
	remChan := target.activeChannel
	target.castMu.Unlock()

	if remChan != nil {
		t.Fatalf("expected target.activeChannel to be nil, got %v", remChan)
	}
	if !activeChan.Stopped {
		t.Fatalf("expected activeChan.Stopped to be true")
	}

	time.Sleep(100 * time.Millisecond)
	_ = cConnB.Close()
	<-doneRead

	receivedLock.Lock()
	hasCastFailed := false
	hasChannelUpdate := false
	for _, op := range receivedOpcodes {
		if op == uint16(protocol.OpcodeSMSG_CAST_FAILED) {
			hasCastFailed = true
		}
		if op == uint16(protocol.OpcodeMSG_CHANNEL_UPDATE) {
			hasChannelUpdate = true
		}
	}
	receivedLock.Unlock()

	if !hasCastFailed {
		t.Fatalf("expected SMSG_CAST_FAILED, got opcodes %v", receivedOpcodes)
	}
	if !hasChannelUpdate {
		t.Fatalf("expected MSG_CHANNEL_UPDATE (0), got opcodes %v", receivedOpcodes)
	}
}

func TestSchoolLockout_CastPreCheck(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	target := &session{
		conn:         sConn,
		playerGUID:   20,
		playerLoaded: true,
		player: &playerState{
			GUID:  20,
			Level: 80,
		},
		schoolLockouts: make(map[uint32]int64),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// Lock out Frost school (SchoolMask 16) for 5 seconds
	target.schoolLockouts[16] = time.Now().Unix() + 5

	// 1. Frost spell (Frost Nova 122, SchoolMask 16, PreventionType 1)
	frostSpell := wotlk.Spell{
		ID:             122,
		SchoolMask:     16,
		PreventionType: spellPreventionTypeSilence, // 1
	}
	if !target.isSchoolLocked(frostSpell) {
		t.Fatalf("expected Frost Nova (SchoolMask 16) to be locked during Frost lockout")
	}

	// 2. Fire spell (Fireball 133, SchoolMask 4, PreventionType 1)
	fireSpell := wotlk.Spell{
		ID:             133,
		SchoolMask:     4,
		PreventionType: spellPreventionTypeSilence, // 1
	}
	if target.isSchoolLocked(fireSpell) {
		t.Fatalf("expected Fireball (SchoolMask 4) NOT to be locked during Frost lockout")
	}

	// 3. Physical spell (Sinister Strike 1752, SchoolMask 1, PreventionType 2 = Pacify)
	physSpell := wotlk.Spell{
		ID:             1752,
		SchoolMask:     1,
		PreventionType: spellPreventionTypePacify, // 2
	}
	if target.isSchoolLocked(physSpell) {
		t.Fatalf("expected Physical spell with pacify prevention NOT to be locked")
	}

	// 4. Expired lockout
	target.schoolLockouts[16] = time.Now().Unix() - 1
	if target.isSchoolLocked(frostSpell) {
		t.Fatalf("expected expired lockout NOT to lock Frost Nova")
	}
}

func TestInterruptCast_UninterruptibleFlags(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	attacker := &session{
		server:       srv,
		playerGUID:   10,
		playerLoaded: true,
		player:       &playerState{GUID: 10, Level: 80},
	}

	target := &session{
		server:       srv,
		playerGUID:   20,
		playerLoaded: true,
		player:       &playerState{GUID: 20, Level: 80},
	}

	srv.sessions[attacker] = struct{}{}
	srv.sessions[target] = struct{}{}

	// Cast with InterruptFlags that do not include spellInterruptFlagInterrupt (0x01)
	timer := time.NewTimer(3 * time.Second)
	activeCast := &activeCastState{
		CastID:       1,
		SpellID:      99999,
		Timer:        timer,
		StartAt:      time.Now(),
		CastTimeMs:   3000,
		InterruptFlg: 0x02, // Does not have spellInterruptFlagInterrupt
	}
	target.castMu.Lock()
	target.activeCast = activeCast
	target.castMu.Unlock()

	kickSpell := wotlk.Spell{ID: 1766}
	attacker.handleEffectInterruptCast(context.Background(), target.playerGUID, kickSpell, wotlk.SpellEffect{Effect: spellEffectInterruptCast})

	target.castMu.Lock()
	remCast := target.activeCast
	target.castMu.Unlock()

	if remCast == nil {
		t.Fatalf("uninterruptible cast should not have been aborted")
	}
	if activeCast.Cancelled {
		t.Fatalf("uninterruptible cast should not be marked cancelled")
	}
}
