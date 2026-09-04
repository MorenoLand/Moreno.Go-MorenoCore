package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCreatureWaypointPatrol(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL, position_x REAL, position_y REAL, position_z REAL, orientation REAL, MovementType INTEGER NOT NULL DEFAULT 0, wander_distance REAL NOT NULL DEFAULT 0)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, speed_walk REAL NOT NULL DEFAULT 2.5)",
		"CREATE TABLE creature_addon (guid INTEGER PRIMARY KEY, path_id INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE waypoint_data (id INTEGER NOT NULL, point INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL, orientation REAL NOT NULL DEFAULT 0, move_type INTEGER NOT NULL DEFAULT 1, delay INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (id, point))",
		"INSERT INTO creature VALUES (10, 68, 0, 0, 0, 0, 0, 2, 0)",
		"INSERT INTO creature_template VALUES (68, 2.5)",
		"INSERT INTO creature_addon VALUES (10, 0)", // path defaults to spawn guid like TC
		"INSERT INTO waypoint_data VALUES (10, 1, 0, 10, 0, 0, 0, 0)",
		"INSERT INTO waypoint_data VALUES (10, 2, 10, 10, 0, 0, 1, 1)",
		"INSERT INTO waypoint_data VALUES (10, 3, 10, 0, 0, 0, 0, 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, Config: config.Default(), sessions: make(map[*session]struct{}), creatureMotion: make(map[uint64]*creatureMotion)}
	ctx := context.Background()

	motion := server.motionFor(ctx, 10, 68, 0, 0, 0, 0, 2, 0, 2.5)
	if len(motion.Points) != 3 {
		t.Fatalf("expected 3 waypoints, got %d", len(motion.Points))
	}
	if motion.Points[0].MoveType != 0 || motion.Points[1].MoveType != 1 {
		t.Fatalf("unexpected waypoint movement types: %d, %d", motion.Points[0].MoveType, motion.Points[1].MoveType)
	}

	// First step: walks toward point 1.
	now := time.Now()
	server.stepCreatureMotion(ctx, motion, nil, now)
	if !motion.Moving {
		t.Fatal("expected motion to start toward waypoint 1")
	}
	if motion.X != 0 || motion.Y != 10 {
		t.Fatalf("position should snap to waypoint target, got %f,%f", motion.X, motion.Y)
	}
	if duration := motion.MoveEnds.Sub(now); duration < 3900*time.Millisecond || duration > 4100*time.Millisecond {
		t.Fatalf("walk waypoint duration=%v", duration)
	}
	// Simulate move completion + no delay.
	after := now.Add(time.Duration(motion.MoveEnds.Sub(now)) + time.Second)
	server.stepCreatureMotion(ctx, motion, nil, after)
	if motion.NextIdx != 2 {
		t.Fatalf("expected waypoint 3 queued, got idx %d", motion.NextIdx)
	}
	server.stepCreatureMotion(ctx, motion, nil, after)
	if duration := motion.MoveEnds.Sub(after); duration < 1300*time.Millisecond || duration > 1500*time.Millisecond {
		t.Fatalf("run waypoint duration=%v", duration)
	}

	// Random wander sanity: MotionType 1 stays within wander_distance of home.
	wander := server.motionFor(ctx, 11, 68, 0, 5, 5, 0, 1, 3.0, 2.5)
	for i := 0; i < 20; i++ {
		server.stepCreatureMotion(ctx, wander, nil, time.Now().Add(time.Duration(i)*2*time.Second))
	}
	if d := dist2D(wander.X, wander.Y, wander.HomeX, wander.HomeY); d > 3.1 {
		t.Fatalf("wanderer drifted %f beyond wander_distance", d)
	}
}

func dist2D(ax, ay, bx, by float32) float64 {
	dx := float64(ax - bx)
	dy := float64(ay - by)
	return sqrt64(dx*dx + dy*dy)
}

func TestCreatureSpeedMultipliersUseReferenceBaseSpeeds(t *testing.T) {
	if got := creatureWalkVelocity(1); got != 2.5 {
		t.Fatalf("walk speed=%v", got)
	}
	if got := creatureRunVelocity(1.14286); got < 7.99 || got > 8.01 {
		t.Fatalf("run speed=%v", got)
	}
	if got := creatureWalkVelocity(0); got != 2.5 {
		t.Fatalf("default walk speed=%v", got)
	}
}

func TestCreatureCombatFlags(t *testing.T) {
	if !creatureCombatDisabled(0x00000100, 0) || !creatureCombatDisabled(0, 0x00002000) {
		t.Fatal("expected noncombat creature flags to disable combat")
	}
	if creatureCombatDisabled(0, 0) {
		t.Fatal("ordinary creature flags must remain attackable")
	}
}

func sqrt64(v float64) float64 {
	if v <= 0 {
		return 0
	}
	// Newton iteration avoids importing math here.
	x := v
	for i := 0; i < 40; i++ {
		x = (x + v/x) / 2
	}
	return x
}

func TestCreatureHostileAggroAndCombat(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Map: 0, X: 5.0, Y: 0.0, Z: 0.0, Race: 1, Health: 100}}
	server := &Server{sessions: make(map[*session]struct{}), creatureMotion: make(map[uint64]*creatureMotion)}
	server.sessions[sess] = struct{}{}

	motion := &creatureMotion{
		GUID:     creatureWorldGUID(100, 68),
		Entry:    68,
		Map:      0,
		X:        0.0,
		Y:        0.0,
		Z:        0.0,
		Faction:  14, // Monster (Hostile)
		Level:    5,
		Speed:    2.5,
		RunSpeed: 7.0,
	}

	players := []playerPos{{
		Map:    0,
		X:      5.0,
		Y:      0.0,
		Z:      0.0,
		GUID:   1,
		Race:   1,
		Level:  1,
		IsGM:   false,
		IsDead: false,
		Sess:   sess,
	}}

	now := time.Now()
	// Goroutine to drain server frames
	stopReader := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopReader:
				return
			default:
				_, _, err := readServerFrame(clientConn, nil)
				if err != nil {
					return
				}
			}
		}
	}()
	defer close(stopReader)

	// 1. Hostile creature within 15 yards should aggro on player
	server.stepCreatureMotion(context.Background(), motion, players, now)
	if !motion.InCombat {
		t.Fatal("expected creature to enter combat")
	}
	if motion.TargetGUID != 1 {
		t.Fatalf("expected creature target to be 1, got %d", motion.TargetGUID)
	}

	// 2. Creature pursues target at run speed
	server.stepCreatureMotion(context.Background(), motion, players, now)
	if motion.X != 5.0 || motion.Y != 0.0 {
		t.Fatalf("expected creature to move toward target, got pos %f,%f", motion.X, motion.Y)
	}
}

func TestCreatureSpellCastingInCombat(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature_template_spell (CreatureID INTEGER NOT NULL, `Index` INTEGER NOT NULL DEFAULT 0, Spell INTEGER DEFAULT NULL, PRIMARY KEY (CreatureID, `Index`))",
		"INSERT INTO creature_template_spell VALUES (68, 0, 133)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Map: 0, X: 15.0, Y: 0.0, Z: 0.0, Race: 1, Health: 100, MaxHealth: 100}}
	server := &Server{WorldStore: store, sessions: make(map[*session]struct{}), creatureMotion: make(map[uint64]*creatureMotion)}
	server.sessions[sess] = struct{}{}
	sess.server = server

	motion := &creatureMotion{
		GUID:       creatureWorldGUID(100, 68),
		Entry:      68,
		Map:        0,
		X:          0.0,
		Y:          0.0,
		Z:          0.0,
		Faction:    14,
		Level:      10,
		InCombat:   true,
		TargetGUID: 1,
	}

	players := []playerPos{{
		Map:    0,
		X:      15.0,
		Y:      0.0,
		Z:      0.0,
		GUID:   1,
		Race:   1,
		Level:  10,
		IsGM:   false,
		IsDead: false,
		Sess:   sess,
	}}

	opcodes := make(chan uint16, 20)
	stopReader := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopReader:
				return
			default:
				_ = clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				op, _, err := readServerFrame(clientConn, nil)
				if err == nil {
					opcodes <- op
				}
			}
		}
	}()

	now := time.Now()
	server.stepCreatureMotion(context.Background(), motion, players, now)
	time.Sleep(50 * time.Millisecond)
	close(stopReader)

	// Verify creature loaded spell 133
	if len(motion.Spells) != 1 || motion.Spells[0] != 133 {
		t.Fatalf("expected creature spells to contain 133, got %v", motion.Spells)
	}

	// Verify player received SMSG_SPELL_GO (0x132) and SMSG_SPELLNONMELEEDAMAGELOG (0x250)
	gotSpellGo := false
	gotDamageLog := false
	for len(opcodes) > 0 {
		op := <-opcodes
		if op == uint16(protocol.OpcodeSMSG_SPELL_GO) {
			gotSpellGo = true
		}
		if op == uint16(protocol.OpcodeSMSG_SPELLNONMELEEDAMAGELOG) {
			gotDamageLog = true
		}
	}

	if !gotSpellGo {
		t.Fatal("expected SMSG_SPELL_GO to be sent to player")
	}
	if !gotDamageLog {
		t.Fatal("expected SMSG_SPELLNONMELEEDAMAGELOG to be sent to player")
	}
	if sess.player.Health >= 100 {
		t.Fatalf("expected player health reduced from 100, got %d", sess.player.Health)
	}
	if sess.player.UnitFlags&unitFlagInCombat == 0 {
		t.Fatal("expected player unitFlagInCombat to be set")
	}
}

