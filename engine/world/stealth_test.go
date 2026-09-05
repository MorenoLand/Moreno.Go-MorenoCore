package world

import (
	"math"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestStealthDetection_BehindObserver(t *testing.T) {
	obs := &session{
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:        1,
			Level:       80,
			X:           0,
			Y:           0,
			Z:           0,
			Orientation: 0, // Facing East (+X)
			CombatReach: 1.5,
		},
	}

	target := &session{
		playerGUID:   2,
		playerLoaded: true,
		player: &playerState{
			GUID:        2,
			Level:       80,
			X:           -5.0, // Behind observer (-X)
			Y:           0,
			Z:           0,
			CombatReach: 1.5,
		},
		activeAuras: map[uint32]*activeAura{
			1787: {SpellID: 1787, AuraType: 16, Amount: 300}, // Stealth Rank 4
		},
		auras: map[uint32]struct{}{1787: {}},
	}

	// 1. Target is stealthed behind observer at 5.0 yards (> combat reach 1.5) -> cannot detect
	if obs.canDetectStealthOf(target) {
		t.Fatal("expected observer not to detect stealthed unit behind them at 5 yards")
	}

	// 2. Target moves into contact reach behind observer (1.0 yard < combat reach 1.5) -> detected!
	target.player.X = -1.0
	if !obs.canDetectStealthOf(target) {
		t.Fatal("expected observer to detect stealthed unit within combat reach (< 1.5 yards) even from behind")
	}

	// 3. Unstealthed target behind observer -> always detected
	target.activeAuras = nil
	target.auras = nil
	target.player.X = -10.0
	if !obs.canDetectStealthOf(target) {
		t.Fatal("expected unstealthed target to be detected")
	}
}

func TestStealthDetection_LevelDifferenceAndRating(t *testing.T) {
	// Observer is Level 60 facing East (0)
	obs := &session{
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:        1,
			Level:       60,
			X:           0,
			Y:           0,
			Z:           0,
			Orientation: 0, // Facing East
			CombatReach: 1.5,
		},
	}

	// Target has Stealth Rank 4 (300 rating)
	target := &session{
		playerGUID:   2,
		playerLoaded: true,
		player: &playerState{
			GUID:        2,
			Level:       80,
			X:           8.0, // In front of observer (+X)
			Y:           0,
			Z:           0,
			CombatReach: 1.5,
		},
		activeAuras: map[uint32]*activeAura{
			1787: {SpellID: 1787, AuraType: 16, Amount: 300},
		},
		auras: map[uint32]struct{}{1787: {}},
	}

	// Level 60 detection: 30 + (60 - 1)*5 = 325
	// Stealth rating: 300
	// Diff: 25 -> visibilityRange = 25 * 0.3 + 1.5 = 9.0 yards
	// At 8.0 yards (<= 9.0) -> detected
	if !obs.canDetectStealthOf(target) {
		t.Fatal("expected level 60 observer to detect level 80 stealth at 8.0 yards (range 9.0)")
	}

	// At 12.0 yards (> 9.0) -> not detected
	target.player.X = 12.0
	if obs.canDetectStealthOf(target) {
		t.Fatal("expected level 60 observer not to detect stealth at 12.0 yards (range 9.0)")
	}
}

func TestStealthDetection_TrackHiddenAndMasterOfDeception(t *testing.T) {
	obs := &session{
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:        1,
			Level:       60,
			X:           0,
			Y:           0,
			Z:           0,
			Orientation: 0,
			CombatReach: 1.5,
		},
	}

	// Target has Stealth Rank 4 (300) + Master of Deception (+15)
	target := &session{
		playerGUID:   2,
		playerLoaded: true,
		player: &playerState{
			GUID:        2,
			Level:       80,
			X:           6.0, // 6 yards away
			Y:           0,
			Z:           0,
			CombatReach: 1.5,
		},
		activeAuras: map[uint32]*activeAura{
			1787:  {SpellID: 1787, AuraType: 16, Amount: 300}, // Stealth
			13971: {SpellID: 13971, AuraType: 154, Amount: 15}, // Master of Deception
		},
		auras: map[uint32]struct{}{1787: {}, 13971: {}},
	}

	// Detection without detect stealth:
	// 325 - (300 + 15) = 10 -> visibilityRange = 10 * 0.3 + 1.5 = 4.5 yards
	// At 6.0 yards -> not detected!
	if obs.canDetectStealthOf(target) {
		t.Fatal("expected Master of Deception to prevent detection at 6.0 yards (range 4.5)")
	}

	// Observer activates Track Hidden (+30 stealth detect)
	obs.activeAuras = map[uint32]*activeAura{
		19885: {SpellID: 19885, AuraType: 17, Amount: 30},
	}
	obs.auras = map[uint32]struct{}{19885: {}}

	// Detection with Track Hidden:
	// 10 + 30 = 40 -> visibilityRange = 40 * 0.3 + 1.5 = 13.5 yards
	// At 6.0 yards -> detected!
	if !obs.canDetectStealthOf(target) {
		t.Fatal("expected Track Hidden (+30 detect) to detect target at 6.0 yards (range 13.5)")
	}
}

func TestStealth_MovementBroadcastFilter(t *testing.T) {
	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()

	c2, s2 := net.Pipe()
	defer c2.Close()
	defer s2.Close()

	c3, s3 := net.Pipe()
	defer c3.Close()
	defer s3.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	// p1: Stealthed player at (10, 0)
	p1 := &session{
		server:       srv,
		conn:         s1,
		authed:       true,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:        10,
			Level:       80,
			X:           10,
			Y:           0,
			Z:           0,
			CombatReach: 1.5,
		},
		activeAuras: map[uint32]*activeAura{
			1787: {SpellID: 1787, AuraType: 16, Amount: 300},
		},
		auras: map[uint32]struct{}{1787: {}},
	}

	// p2: Observer at (0, 0) facing AWAY from p1 (Orientation = Pi, facing West)
	p2 := &session{
		server:       srv,
		conn:         s2,
		authed:       true,
		playerLoaded: true,
		playerGUID:   20,
		player: &playerState{
			GUID:        20,
			Level:       80,
			X:           0,
			Y:           0,
			Z:           0,
			Orientation: float32(math.Pi), // Facing West (p1 is East at +10)
			CombatReach: 1.5,
		},
	}

	// p3: Observer at (8, 0) facing TOWARDS p1 (Orientation = 0, facing East, dist = 2 yards)
	p3 := &session{
		server:       srv,
		conn:         s3,
		authed:       true,
		playerLoaded: true,
		playerGUID:   30,
		player: &playerState{
			GUID:        30,
			Level:       80,
			X:           8,
			Y:           0,
			Z:           0,
			Orientation: 0, // Facing East towards p1
			CombatReach: 1.5,
		},
	}

	srv.sessions[p1] = struct{}{}
	srv.sessions[p2] = struct{}{}
	srv.sessions[p3] = struct{}{}

	p2Opcodes := make(chan uint16, 4)
	p3Opcodes := make(chan uint16, 4)

	go func() {
		for {
			op, _, err := readServerFrame(c2, nil)
			if err != nil {
				return
			}
			p2Opcodes <- op
		}
	}()
	go func() {
		for {
			op, _, err := readServerFrame(c3, nil)
			if err != nil {
				return
			}
			p3Opcodes <- op
		}
	}()

	info := movementInfo{GUID: 10, Time: 100, X: 10, Y: 1, Z: 0}
	buf := protocol.NewBuffer(64)
	writeMovementInfo(buf, info)

	srv.broadcastMovement(uint16(protocol.OpcodeMSG_MOVE_HEARTBEAT), buf.Bytes(), info, p1)

	// p3 should receive movement
	select {
	case op := <-p3Opcodes:
		if op != uint16(protocol.OpcodeMSG_MOVE_HEARTBEAT) {
			t.Fatalf("expected MSG_MOVE_HEARTBEAT on p3, got 0x%04X", op)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for p3 movement broadcast")
	}

	// p2 should NOT receive movement
	select {
	case op := <-p2Opcodes:
		t.Fatalf("p2 should not receive movement from undetected stealthed player, got 0x%04X", op)
	case <-time.After(50 * time.Millisecond):
		// Expected: no packet received
	}
}

func TestCreature_StealthAggro(t *testing.T) {
	motion := &creatureMotion{
		GUID:        100,
		Level:       60,
		X:           0,
		Y:           0,
		Orientation: 0, // Facing East
		CombatReach: 1.5,
		Faction:     14, // Hostile
	}

	targetSess := &session{
		playerGUID:   10,
		playerLoaded: true,
		player: &playerState{
			GUID:        10,
			Level:       80,
			X:           15.0, // 15 yards East
			Y:           0,
			Race:        1, // Human (hostile to faction 14)
			CombatReach: 1.5,
		},
		activeAuras: map[uint32]*activeAura{
			1787: {SpellID: 1787, AuraType: 16, Amount: 300}, // Stealth Rank 4
		},
		auras: map[uint32]struct{}{1787: {}},
	}

	// 1. Level 60 creature detection range vs 300 stealth is 9.0 yards.
	// Player is at 15.0 yards: creature cannot detect!
	if canCreatureDetectStealthOfPlayer(motion, targetSess, 15.0) {
		t.Fatal("expected creature not to detect stealth at 15 yards")
	}

	// 2. Player is behind creature at 5.0 yards: creature cannot detect!
	targetSess.player.X = -5.0
	if canCreatureDetectStealthOfPlayer(motion, targetSess, 5.0) {
		t.Fatal("expected creature not to detect stealth behind them")
	}

	// 3. Player enters contact reach at 1.0 yard (< 1.5): detected!
	targetSess.player.X = -1.0
	if !canCreatureDetectStealthOfPlayer(motion, targetSess, 1.0) {
		t.Fatal("expected creature to detect stealth in contact reach")
	}
}
