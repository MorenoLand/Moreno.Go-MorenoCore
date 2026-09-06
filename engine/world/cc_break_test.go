package world

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCCBreak_DirectDamageBreaksCC(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:         sConn,
		playerGUID:   100,
		playerLoaded: true,
		player:       &playerState{GUID: 100, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
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

	// Apply Gouge (1776) and Polymorph (118) to the session
	sess.castMu.Lock()
	sess.auras[1776] = struct{}{}
	sess.auraSlots[1776] = 0
	sess.activeAuras[1776] = &activeAura{
		SpellID:            1776,
		TargetGUID:         100,
		AuraInterruptFlags: auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage,
	}

	sess.auras[118] = struct{}{}
	sess.auraSlots[118] = 1
	sess.activeAuras[118] = &activeAura{
		SpellID:            118,
		TargetGUID:         100,
		AuraInterruptFlags: auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage,
	}
	sess.castMu.Unlock()

	// Verify auras are active
	if !sess.hasAura(1776) || !sess.hasAura(118) {
		t.Fatalf("expected Gouge and Polymorph to be present")
	}

	// Taking direct damage (e.g. melee or spell) triggers procDamageAuras(true)
	sess.procDamageAuras(true)

	// Verify CC auras were broken
	if sess.hasAura(1776) {
		t.Fatalf("expected Gouge to be removed by direct damage")
	}
	if sess.hasAura(118) {
		t.Fatalf("expected Polymorph to be removed by direct damage")
	}

	time.Sleep(50 * time.Millisecond)
	_ = cConn.Close()
	<-doneRead

	receivedLock.Lock()
	hasAuraUpdate := false
	for _, op := range receivedOpcodes {
		if op == uint16(protocol.OpcodeSMSG_AURA_UPDATE) {
			hasAuraUpdate = true
			break
		}
	}
	receivedLock.Unlock()

	if !hasAuraUpdate {
		t.Fatalf("expected SMSG_AURA_UPDATE sent to client on CC break")
	}
}

func TestCCBreak_PeriodicDamageOnlyBreaksTakeDamageAuras(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:         sConn,
		playerGUID:   200,
		playerLoaded: true,
		player:       &playerState{GUID: 200, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// 1. Aura with ONLY DIRECT_DAMAGE (e.g. some root/stun breaking only on direct hits)
	sess.castMu.Lock()
	sess.auras[99901] = struct{}{}
	sess.auraSlots[99901] = 0
	sess.activeAuras[99901] = &activeAura{
		SpellID:            99901,
		TargetGUID:         200,
		AuraInterruptFlags: auraInterruptFlagDirectDamage,
	}

	// 2. Stealth (1784) with TAKE_DAMAGE (breaks on any damage, including periodic DoT)
	sess.auras[1784] = struct{}{}
	sess.auraSlots[1784] = 1
	sess.activeAuras[1784] = &activeAura{
		SpellID:            1784,
		TargetGUID:         200,
		AuraInterruptFlags: auraInterruptFlagTakeDamage,
	}
	sess.castMu.Unlock()

	// Periodic damage tick: procDamageAuras(false)
	sess.procDamageAuras(false)

	// Stealth must be removed by periodic damage
	if sess.hasAura(1784) {
		t.Fatalf("expected Stealth to be broken by periodic damage tick")
	}

	// Direct-damage-only aura must NOT be removed by periodic damage
	if !sess.hasAura(99901) {
		t.Fatalf("expected direct-damage-only aura to survive periodic damage tick")
	}
}

func TestCCBreak_CastBreaksFoodDrink(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:         sConn,
		playerGUID:   300,
		playerLoaded: true,
		player:       &playerState{GUID: 300, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// Food buff (430) with AURA_INTERRUPT_FLAG_CAST | AURA_INTERRUPT_FLAG_MOVE | AURA_INTERRUPT_FLAG_TAKE_DAMAGE
	sess.castMu.Lock()
	sess.auras[430] = struct{}{}
	sess.auraSlots[430] = 0
	sess.activeAuras[430] = &activeAura{
		SpellID:            430,
		TargetGUID:         300,
		AuraInterruptFlags: auraInterruptFlagCast | auraInterruptFlagMove | auraInterruptFlagTakeDamage,
	}
	sess.castMu.Unlock()

	if !sess.hasAura(430) {
		t.Fatalf("expected Food buff to be active")
	}

	// Casting a spell triggers procCastAuras()
	sess.procCastAuras()

	if sess.hasAura(430) {
		t.Fatalf("expected Food buff to be removed on casting a spell")
	}
}

func TestCCBreak_MovementBreaksFoodDrink(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:         sConn,
		playerGUID:   400,
		playerLoaded: true,
		player:       &playerState{GUID: 400, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// Drink buff (431)
	sess.castMu.Lock()
	sess.auras[431] = struct{}{}
	sess.auraSlots[431] = 0
	sess.activeAuras[431] = &activeAura{
		SpellID:            431,
		TargetGUID:         400,
		AuraInterruptFlags: auraInterruptFlagCast | auraInterruptFlagMove | auraInterruptFlagTakeDamage,
	}
	sess.castMu.Unlock()

	// Moving calls removeAurasWithInterruptFlags(0x08 = AURA_INTERRUPT_FLAG_MOVE)
	sess.removeAurasWithInterruptFlags(auraInterruptFlagMove)

	if sess.hasAura(431) {
		t.Fatalf("expected Drink buff to be removed on movement")
	}
}

func TestCCBreak_CreatureAndFallDamageIntegration(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	sess := &session{
		conn:         sConn,
		playerGUID:   500,
		playerLoaded: true,
		player:       &playerState{GUID: 500, Level: 80, Health: 5000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// Sap (6770)
	sess.castMu.Lock()
	sess.auras[6770] = struct{}{}
	sess.auraSlots[6770] = 0
	sess.activeAuras[6770] = &activeAura{
		SpellID:            6770,
		TargetGUID:         500,
		AuraInterruptFlags: auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage,
	}
	sess.castMu.Unlock()

	// Simulate fall damage handling (movement.go)
	damage := uint32(500)
	sess.player.Health -= damage
	sess.procDamageAuras(true)

	if sess.hasAura(6770) {
		t.Fatalf("expected Sap to break on fall damage")
	}
}
