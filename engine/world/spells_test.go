package world

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestHandleCancelMountAura(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1, MountDisplayID: 123}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelMountAura(nil) {
			t.Error("handleCancelMountAura returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	if player.MountDisplayID != 0 {
		t.Fatalf("expected MountDisplayID 0, got %d", player.MountDisplayID)
	}
}

func TestHandleCancelGrowthAura(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
		scale:        2.5,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelGrowthAura(nil) {
			t.Error("handleCancelGrowthAura returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	if s.scale != 1.0 {
		t.Fatalf("expected scale 1.0, got %f", s.scale)
	}
}

func TestHandleCancelAutoRepeatSpell(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 42}
	s := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   42,
		player:       player,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelAutoRepeatSpell(nil) {
			t.Error("handleCancelAutoRepeatSpell returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_CANCEL_AUTO_REPEAT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	guid, err := r.ReadPackedGUID()
	if err != nil || guid != 42 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
}

func TestHandleCancelTempEnchantment(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(16) // slot 16

	done := make(chan struct{})
	go func() {
		if !s.handleCancelTempEnchantment(context.Background(), payload.Bytes()) {
			t.Error("handleCancelTempEnchantment returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
}

func TestHandleCorpseMapPositionQuery(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	s := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(0)

	done := make(chan struct{})
	go func() {
		if !s.handleCorpseMapPositionQuery(payload.Bytes()) {
			t.Error("handleCorpseMapPositionQuery returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	for i := 0; i < 4; i++ {
		val, err := r.ReadF32()
		if err != nil || val != 0.0 {
			t.Fatalf("coord[%d]=%f err=%v", i, val, err)
		}
	}
}

func TestMissileSpellTravelDelay(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			100: {GUID: 100, Map: 0, X: 20, Y: 0, Z: 0, Health: 100},
		},
	}
	s := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Map: 0, X: 0, Y: 0, Z: 0, Level: 1},
	}
	ctx := context.Background()

	// Drain frames in background
	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	spell := wotlk.Spell{
		ID:    686, // Shadow Bolt
		Speed: 20,  // 20 yards/sec -> 20 yards = 1000ms
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2, BasePoints: 24}, // 25 damage
		},
	}

	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: 100,
	}

	s.finishSpellCast(ctx, 1, 686, spell, target)

	// Immediately at t=5ms, target health must NOT be damaged yet
	time.Sleep(20 * time.Millisecond)
	srv.motionMu.Lock()
	healthEarly := srv.creatureMotion[100].Health
	srv.motionMu.Unlock()
	if healthEarly != 100 {
		t.Fatalf("expected target health 100 immediately upon cast release, got %d", healthEarly)
	}

	// After travel time (1000ms + margin), target health must be damaged
	time.Sleep(1100 * time.Millisecond)
	srv.motionMu.Lock()
	healthLate := srv.creatureMotion[100].Health
	srv.motionMu.Unlock()
	if healthLate >= 100 {
		t.Fatalf("expected target health < 100 after projectile arrival, got %d", healthLate)
	}
}

func TestCalculateSpellPowerCost_ManaCostPct(t *testing.T) {
	s := &session{
		player: &playerState{
			Powers:    [7]uint32{1000, 0, 0, 100, 0, 0, 0},
			MaxPowers: [7]uint32{1000, 0, 0, 100, 0, 0, 0},
		},
	}
	// Demon Armor Rank 1: PowerType = 0 (Mana), ManaCost = 0, ManaCostPct = 12%
	spell := wotlk.Spell{
		ID:          687,
		PowerType:   0,
		ManaCost:    0,
		ManaCostPct: 12,
	}
	cost := s.calculateSpellPowerCost(spell)
	// 12% of 1000 base mana = 120 mana
	if cost != 120 {
		t.Errorf("expected cost 120, got %d", cost)
	}

	// Flat cost spell: ManaCost = 50, ManaCostPct = 0
	flatSpell := wotlk.Spell{
		ID:        123,
		PowerType: 0,
		ManaCost:  50,
	}
	if s.calculateSpellPowerCost(flatSpell) != 50 {
		t.Errorf("expected flat cost 50, got %d", s.calculateSpellPowerCost(flatSpell))
	}
}

func TestIsSelfCastOnly(t *testing.T) {
	// Demon Armor (all active effects target TARGET_UNIT_CASTER = 1)
	selfSpell := wotlk.Spell{
		ID: 687,
		Effects: [3]wotlk.SpellEffect{
			{Effect: 6, ImplicitTargetA: 1}, // SPELL_EFFECT_APPLY_AURA on CASTER
			{Effect: 0},
			{Effect: 0},
		},
	}
	if !isSelfCastOnly(selfSpell) {
		t.Error("expected selfSpell to be identified as self-cast only")
	}

	// Shadow Bolt (targets enemy: ImplicitTargetA = 6)
	enemySpell := wotlk.Spell{
		ID: 686,
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2, ImplicitTargetA: 6}, // TARGET_UNIT_TARGET_ENEMY = 6
		},
	}
	if isSelfCastOnly(enemySpell) {
		t.Error("expected enemySpell to NOT be identified as self-cast only")
	}
}

func TestSpellCastPowerDeductionAndBroadcast(t *testing.T) {
	casterServerConn, casterClientConn := net.Pipe()
	defer casterServerConn.Close()
	defer casterClientConn.Close()

	observerServerConn, observerClientConn := net.Pipe()
	defer observerServerConn.Close()
	defer observerClientConn.Close()

	srv := &Server{sessions: make(map[*session]struct{})}

	casterSess := &session{
		conn:         casterServerConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   100,
		player:       &playerState{GUID: 100, Powers: [7]uint32{200}, MaxPowers: [7]uint32{200}, Map: 0, X: 10, Y: 10, Z: 0},
	}
	observerSess := &session{
		conn:         observerServerConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   200,
		player:       &playerState{GUID: 200, Map: 0, X: 12, Y: 10, Z: 0},
	}
	srv.sessions[casterSess] = struct{}{}
	srv.sessions[observerSess] = struct{}{}

	spell := wotlk.Spell{
		ID:        133,
		PowerType: 0,  // Mana
		ManaCost:  45, // costs 45 mana
	}
	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: 100,
	}

	done := make(chan struct{})
	go func() {
		casterSess.finishSpellCast(context.Background(), 1, 133, spell, target)
		close(done)
	}()

	// 1. Caster reads SMSG_SPELL_GO, then SMSG_UPDATE_OBJECT with power delta
	_ = casterClientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op1, _, err := readServerFrame(casterClientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if op1 != uint16(protocol.OpcodeSMSG_SPELL_GO) {
		t.Fatalf("expected SMSG_SPELL_GO, got 0x%x", op1)
	}
	op2, _, err := readServerFrame(casterClientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if op2 != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op2 != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("expected SMSG_UPDATE_OBJECT or SMSG_COMPRESSED_UPDATE_OBJECT, got 0x%x", op2)
	}

	// 2. Observer receives broadcast of power update
	_ = observerClientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	obsOp, _, err := readServerFrame(observerClientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if obsOp != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && obsOp != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("expected observer to receive SMSG_UPDATE_OBJECT or SMSG_COMPRESSED_UPDATE_OBJECT, got 0x%x", obsOp)
	}

	<-done

	if casterSess.player.Powers[0] != 200-45 {
		t.Fatalf("expected caster mana 155, got %d", casterSess.player.Powers[0])
	}
}
