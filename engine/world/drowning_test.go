package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestMirrorTimer_PacketEncoding(t *testing.T) {
	// 1. Start mirror timer packet (21 bytes)
	startData := buildStartMirrorTimer(mirrorTimerBreath, 180000, 180000, -1, false, 0)
	if len(startData) != 21 {
		t.Fatalf("expected 21 bytes for SMSG_START_MIRROR_TIMER, got %d", len(startData))
	}
	r := protocol.NewReader(startData)
	timerType, _ := r.ReadU32()
	val, _ := r.ReadU32()
	maxVal, _ := r.ReadU32()
	scale, _ := r.ReadU32() // int32(-1) is 0xFFFFFFFF
	paused, _ := r.ReadU8()
	spellID, _ := r.ReadU32()
	if timerType != 1 || val != 180000 || maxVal != 180000 || scale != 0xFFFFFFFF || paused != 0 || spellID != 0 {
		t.Fatalf("unexpected StartMirrorTimer data: timer=%d val=%d max=%d scale=%d paused=%d spell=%d",
			timerType, val, maxVal, scale, paused, spellID)
	}

	// 2. Pause mirror timer packet (5 bytes)
	pauseData := buildPauseMirrorTimer(mirrorTimerBreath, true)
	if len(pauseData) != 5 {
		t.Fatalf("expected 5 bytes for SMSG_PAUSE_MIRROR_TIMER, got %d", len(pauseData))
	}
	r = protocol.NewReader(pauseData)
	timerType, _ = r.ReadU32()
	paused, _ = r.ReadU8()
	if timerType != 1 || paused != 1 {
		t.Fatalf("unexpected PauseMirrorTimer data: timer=%d paused=%d", timerType, paused)
	}

	// 3. Stop mirror timer packet (4 bytes)
	stopData := buildStopMirrorTimer(mirrorTimerBreath)
	if len(stopData) != 4 {
		t.Fatalf("expected 4 bytes for SMSG_STOP_MIRROR_TIMER, got %d", len(stopData))
	}
	r = protocol.NewReader(stopData)
	timerType, _ = r.ReadU32()
	if timerType != 1 {
		t.Fatalf("unexpected StopMirrorTimer data: timer=%d", timerType)
	}
}

func TestDrowning_EnterAndExitSwimming(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
			X:         0,
			Y:         0,
			Z:         0,
		},
		breathTimer: -1,
	}
	srv.sessions[sess] = struct{}{}

	receivedOpcodes := make(chan uint16, 8)
	receivedPayloads := make(chan []byte, 8)
	go func() {
		for {
			op, p, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedOpcodes <- op
			receivedPayloads <- p
		}
	}()

	// 1. Enter water: MSG_MOVE_START_SWIM
	infoSwim := movementInfo{GUID: 10, Time: 100, X: 0, Y: 0, Z: 0, Flags: movementSwimming, HasPitch: true}
	bufSwim := protocol.NewBuffer(64)
	writeMovementInfo(bufSwim, infoSwim)

	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_START_SWIM), bufSwim.Bytes())

	if !sess.isSwimming {
		t.Fatal("expected isSwimming to be true")
	}
	if sess.breathTimer != maxBreathTimerMs {
		t.Fatalf("expected breathTimer=%d, got %d", maxBreathTimerMs, sess.breathTimer)
	}

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_START_MIRROR_TIMER) {
			t.Fatalf("expected SMSG_START_MIRROR_TIMER, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		timerType, _ := r.ReadU32()
		val, _ := r.ReadU32()
		if timerType != 1 || val != uint32(maxBreathTimerMs) {
			t.Fatalf("unexpected StartMirrorTimer: timer=%d val=%d", timerType, val)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_START_MIRROR_TIMER on enter water")
	}

	// 2. Simulate breathing underwater for 30 seconds
	sess.breathTimer = 150000

	// 3. Surface from water: MSG_MOVE_STOP_SWIM
	infoStop := movementInfo{GUID: 10, Time: 200, X: 0, Y: 0, Z: 0}
	bufStop := protocol.NewBuffer(64)
	writeMovementInfo(bufStop, infoStop)

	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_STOP_SWIM), bufStop.Bytes())

	if sess.isSwimming {
		t.Fatal("expected isSwimming to be false")
	}

	// Client should receive mirror timer update with scale=10 (regen mode)
	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_START_MIRROR_TIMER) {
			t.Fatalf("expected SMSG_START_MIRROR_TIMER (regen), got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		timerType, _ := r.ReadU32()
		val, _ := r.ReadU32()
		_, _ = r.ReadU32()
		scale, _ := r.ReadU32()
		if timerType != 1 || val != 150000 || scale != 10 {
			t.Fatalf("unexpected StartMirrorTimer regen: timer=%d val=%d scale=%d", timerType, val, scale)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_START_MIRROR_TIMER regen on surface")
	}
}

func TestDrowning_WaterBreathingImmunity(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
			X:         0,
			Y:         0,
			Z:         0,
		},
		breathTimer: -1,
		activeAuras: map[uint32]*activeAura{
			5697: {SpellID: 5697, AuraType: 82}, // Unending Breath (SPELL_AURA_WATER_BREATHING = 82)
		},
		auras: map[uint32]struct{}{5697: {}},
	}
	srv.sessions[sess] = struct{}{}

	infoSwim := movementInfo{GUID: 10, Time: 100, X: 0, Y: 0, Z: 0, Flags: movementSwimming, HasPitch: true}
	bufSwim := protocol.NewBuffer(64)
	writeMovementInfo(bufSwim, infoSwim)

	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_START_SWIM), bufSwim.Bytes())

	if !sess.isSwimming {
		t.Fatal("expected isSwimming to be true")
	}
	if sess.breathTimer != -1 {
		t.Fatalf("expected breathTimer to stay -1 under water breathing immunity, got %d", sess.breathTimer)
	}
}

func TestDrowning_DamageTickAndDeath(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
			X:         0,
			Y:         0,
			Z:         0,
		},
		isSwimming:     true,
		breathTimer:    0, // Breath completely depleted
		lastBreathTick: time.Now().Add(-1 * time.Second),
	}
	srv.sessions[sess] = struct{}{}

	receivedOpcodes := make(chan uint16, 64)
	receivedPayloads := make(chan []byte, 64)
	go func() {
		for {
			op, p, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedOpcodes <- op
			receivedPayloads <- p
		}
	}()

	// 1. Drowning tick: deals 20% max health damage (200 on 1000 max health)
	sess.handleDrowningTick(context.Background(), time.Now())

	if sess.player.Health != 800 {
		t.Fatalf("expected health 800 (1000 - 200), got %d", sess.player.Health)
	}

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_ENVIRONMENTAL_DAMAGE_LOG) {
			t.Fatalf("expected SMSG_ENVIRONMENTAL_DAMAGE_LOG, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		vicGUID, _ := r.ReadU64()
		dmgType, _ := r.ReadU8()
		amount, _ := r.ReadU32()
		if vicGUID != 10 || dmgType != 1 || amount != 200 { // dmgType 1 = DAMAGE_DROWNING
			t.Fatalf("unexpected damage log: victim=%d type=%d amount=%d", vicGUID, dmgType, amount)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_ENVIRONMENTAL_DAMAGE_LOG on drowning tick")
	}

	// 2. Lethal drowning: health drops to 0
	sess.player.Health = 150
	sess.lastBreathTick = time.Now().Add(-1 * time.Second)
	sess.handleDrowningTick(context.Background(), time.Now())

	if sess.player.Health != 0 {
		t.Fatalf("expected health 0 after lethal drowning, got %d", sess.player.Health)
	}
	if sess.player.PlayerFieldBytes&playerFieldByteReleaseTimer == 0 {
		t.Fatal("expected playerFieldByteReleaseTimer flag set upon drowning death")
	}
}

func TestFatigue_EnterAndExitDarkWater(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
		},
		fatigueTimer: -1,
	}
	srv.sessions[sess] = struct{}{}

	receivedOpcodes := make(chan uint16, 64)
	receivedPayloads := make(chan []byte, 64)
	go func() {
		for {
			op, p, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedOpcodes <- op
			receivedPayloads <- p
		}
	}()

	// 1. Enter dark water: setInDarkWater(true)
	sess.setInDarkWater(true)

	if !sess.inDarkWater {
		t.Fatal("expected inDarkWater to be true")
	}
	if sess.fatigueTimer != maxFatigueTimerMs {
		t.Fatalf("expected fatigueTimer=%d, got %d", maxFatigueTimerMs, sess.fatigueTimer)
	}

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_START_MIRROR_TIMER) {
			t.Fatalf("expected SMSG_START_MIRROR_TIMER, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		timerType, _ := r.ReadU32()
		val, _ := r.ReadU32()
		maxVal, _ := r.ReadU32()
		scale, _ := r.ReadI32()
		if timerType != mirrorTimerFatigue || val != uint32(maxFatigueTimerMs) || maxVal != uint32(maxFatigueTimerMs) || scale != -1 {
			t.Fatalf("unexpected StartMirrorTimer: timer=%d val=%d max=%d scale=%d", timerType, val, maxVal, scale)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_START_MIRROR_TIMER on dark water entry")
	}

	// 2. Partial fatigue consumption (20 seconds in dark water)
	sess.fatigueTimer = 40000
	sess.setInDarkWater(false)

	// Should switch to regen (+10 scale)
	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_START_MIRROR_TIMER) {
			t.Fatalf("expected SMSG_START_MIRROR_TIMER on exit, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		timerType, _ := r.ReadU32()
		val, _ := r.ReadU32()
		maxVal, _ := r.ReadU32()
		scale, _ := r.ReadI32()
		if timerType != mirrorTimerFatigue || val != 40000 || maxVal != uint32(maxFatigueTimerMs) || scale != 10 {
			t.Fatalf("unexpected regen StartMirrorTimer: timer=%d val=%d max=%d scale=%d", timerType, val, maxVal, scale)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_START_MIRROR_TIMER on dark water exit")
	}

	// 3. Complete regeneration: tick fatigue outside dark water
	sess.lastFatigueTick = time.Now().Add(-5 * time.Second) // 5s at 10x = 50,000ms added -> 40,000 + 50,000 >= 60,000
	sess.handleFatigueTick(context.Background(), time.Now())

	if sess.fatigueTimer != -1 {
		t.Fatalf("expected fatigueTimer to be -1 after full regen, got %d", sess.fatigueTimer)
	}

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_STOP_MIRROR_TIMER) {
			t.Fatalf("expected SMSG_STOP_MIRROR_TIMER after full regen, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		timerType, _ := r.ReadU32()
		if timerType != mirrorTimerFatigue {
			t.Fatalf("unexpected StopMirrorTimer: timer=%d", timerType)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_STOP_MIRROR_TIMER")
	}
}

func TestFatigue_ImmunityGMAndFlight(t *testing.T) {
	// 1. GM immunity
	gmSess := &session{
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:        10,
			Level:       80,
			Health:      1000,
			MaxHealth:   1000,
			PlayerFlags: playerFlagGM,
		},
		fatigueTimer: -1,
	}

	gmSess.setInDarkWater(true)
	if gmSess.fatigueTimer != -1 {
		t.Fatalf("expected GM player to bypass fatigue, got %d", gmSess.fatigueTimer)
	}

	// 2. Flight immunity
	flightSess := &session{
		playerLoaded: true,
		playerGUID:   11,
		inFlight:     true,
		player: &playerState{
			GUID:      11,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
		},
		fatigueTimer: -1,
	}

	flightSess.setInDarkWater(true)
	if flightSess.fatigueTimer != -1 {
		t.Fatalf("expected inFlight player to bypass fatigue, got %d", flightSess.fatigueTimer)
	}
}

func TestFatigue_DamageTickAndExhaustionDeath(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
		},
		inDarkWater:     true,
		fatigueTimer:    0, // Fatigue completely depleted
		lastFatigueTick: time.Now().Add(-1 * time.Second),
	}
	srv.sessions[sess] = struct{}{}

	receivedOpcodes := make(chan uint16, 64)
	receivedPayloads := make(chan []byte, 64)
	go func() {
		for {
			op, p, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedOpcodes <- op
			receivedPayloads <- p
		}
	}()

	// 1. Fatigue tick: deals 20% max health damage (200 on 1000 max health)
	sess.handleFatigueTick(context.Background(), time.Now())

	if sess.player.Health != 800 {
		t.Fatalf("expected health 800 (1000 - 200), got %d", sess.player.Health)
	}

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_ENVIRONMENTAL_DAMAGE_LOG) {
			t.Fatalf("expected SMSG_ENVIRONMENTAL_DAMAGE_LOG, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		vicGUID, _ := r.ReadU64()
		dmgType, _ := r.ReadU8()
		amount, _ := r.ReadU32()
		if vicGUID != 10 || dmgType != damageExhausted || amount != 200 { // dmgType 0 = DAMAGE_EXHAUSTED
			t.Fatalf("unexpected damage log: victim=%d type=%d amount=%d", vicGUID, dmgType, amount)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_ENVIRONMENTAL_DAMAGE_LOG on fatigue tick")
	}

	// 2. Lethal exhaustion: health drops to 0
	sess.player.Health = 100
	sess.lastFatigueTick = time.Now().Add(-1 * time.Second)
	sess.handleFatigueTick(context.Background(), time.Now())

	if sess.player.Health != 0 {
		t.Fatalf("expected health 0 after lethal exhaustion, got %d", sess.player.Health)
	}
	if sess.player.PlayerFieldBytes&playerFieldByteReleaseTimer == 0 {
		t.Fatal("expected playerFieldByteReleaseTimer flag set upon exhaustion death")
	}
}

func TestFatigue_GhostRepopAtGraveyard(t *testing.T) {
	dir := t.TempDir()
	writeWorldSafeLocsDBC(t, dir, [][5]uint32{
		{2, 0, floatBits(10), floatBits(10), floatBits(10)},
	})
	data := wotlk.NewStore(dir)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE graveyard_zone (ID INTEGER NOT NULL, GhostZone INTEGER NOT NULL, Faction INTEGER NOT NULL);
		INSERT INTO graveyard_zone (ID, GhostZone, Faction) VALUES (2, 12, 0);
	`); err != nil {
		t.Fatalf("failed to create graveyard_zone table: %v", err)
	}

	srv := &Server{
		WorldStore: &database.Store{DB: db},
		Data:       data,
		sessions:   make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:        10,
			Race:        1, // Human (Alliance)
			Level:       80,
			Health:      1,
			MaxHealth:   1000,
			PlayerFlags: playerFlagGhost,
			Map:         0,
			Zone:        12,
			X:           5000,
			Y:           5000,
			Z:           10,
		},
		inDarkWater:     true,
		fatigueTimer:    0, // Exhaustion depleted as ghost
		lastFatigueTick: time.Now().Add(-1 * time.Second),
	}
	srv.sessions[sess] = struct{}{}

	sess.handleFatigueTick(context.Background(), time.Now())

	if sess.player.X != 10 || sess.player.Y != 10 {
		t.Fatalf("expected ghost to be teleported to graveyard (10, 10), got (%f, %f)", sess.player.X, sess.player.Y)
	}
}
