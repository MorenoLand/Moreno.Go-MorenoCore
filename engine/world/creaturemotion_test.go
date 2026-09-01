package world

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
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
		"CREATE TABLE waypoint_data (id INTEGER NOT NULL, point INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL, orientation REAL NOT NULL DEFAULT 0, delay INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (id, point))",
		"INSERT INTO creature VALUES (10, 68, 0, 0, 0, 0, 0, 2, 0)",
		"INSERT INTO creature_template VALUES (68, 2.5)",
		"INSERT INTO creature_addon VALUES (10, 0)", // path defaults to spawn guid like TC
		"INSERT INTO waypoint_data VALUES (10, 1, 0, 10, 0, 0, 0)",
		"INSERT INTO waypoint_data VALUES (10, 2, 10, 10, 0, 0, 1)",
		"INSERT INTO waypoint_data VALUES (10, 3, 10, 0, 0, 0, 0)",
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

	// First step: walks toward point 1.
	now := time.Now()
	server.stepCreatureMotion(ctx, motion, now)
	if !motion.Moving {
		t.Fatal("expected motion to start toward waypoint 1")
	}
	if motion.X != 0 || motion.Y != 10 {
		t.Fatalf("position should snap to waypoint target, got %f,%f", motion.X, motion.Y)
	}
	// Simulate move completion + no delay.
	after := now.Add(time.Duration(motion.MoveEnds.Sub(now)) + time.Second)
	server.stepCreatureMotion(ctx, motion, after)
	if motion.NextIdx != 2 {
		t.Fatalf("expected waypoint 3 queued, got idx %d", motion.NextIdx)
	}

	// Random wander sanity: MotionType 1 stays within wander_distance of home.
	wander := server.motionFor(ctx, 11, 68, 0, 5, 5, 0, 1, 3.0, 2.5)
	for i := 0; i < 20; i++ {
		server.stepCreatureMotion(ctx, wander, time.Now().Add(time.Duration(i)*2*time.Second))
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
