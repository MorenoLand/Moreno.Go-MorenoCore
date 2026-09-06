package world

import (
	"context"
	"database/sql"
	"math"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func setupAreaTriggerTest(t *testing.T) (*Server, *session, *sql.DB, *sql.DB) {
	worldDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	charDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = worldDB.Close()
		_ = charDB.Close()
	})

	// Setup schemas
	_, err = worldDB.Exec(`
		CREATE TABLE areatrigger_involvedrelation (id INTEGER PRIMARY KEY, quest INTEGER);
		CREATE TABLE areatrigger_tavern (id INTEGER PRIMARY KEY, name TEXT);
		CREATE TABLE areatrigger_teleport (id INTEGER PRIMARY KEY, name TEXT, target_map INTEGER, target_position_x REAL, target_position_y REAL, target_position_z REAL, target_orientation REAL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = charDB.Exec(`
		CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER, PRIMARY KEY (guid, quest));
		CREATE TABLE corpse (guid INTEGER PRIMARY KEY, posX REAL, posY REAL, posZ REAL, orientation REAL, mapId INTEGER, displayId INTEGER, itemCache TEXT, bytes1 INTEGER, bytes2 INTEGER, flags INTEGER, dynFlags INTEGER, time INTEGER, corpseType INTEGER, instanceId INTEGER);
	`)
	if err != nil {
		t.Fatal(err)
	}

	s1Conn, c1Conn := net.Pipe()
	t.Cleanup(func() {
		_ = s1Conn.Close()
		_ = c1Conn.Close()
	})
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, err := c1Conn.Read(buf); err != nil {
				return
			}
		}
	}()

	srv := &Server{
		WorldStore:      &database.Store{DB: worldDB},
		CharactersStore: &database.Store{DB: charDB},
		Data:            wotlk.NewStore(t.TempDir()),
	}

	sess := &session{
		conn:         s1Conn,
		playerGUID:   101,
		playerLoaded: true,
		accountName:  "TestPlayer",
		player: &playerState{
			GUID:   101,
			Map:    0,
			X:      100.0,
			Y:      200.0,
			Z:         30.0,
			Health:    1000,
			MaxHealth: 1000,
		},
		server: srv,
	}

	return srv, sess, worldDB, charDB
}

func TestAreaTriggerInFlightIgnored(t *testing.T) {
	ctx := context.Background()
	_, sess, worldDB, _ := setupAreaTriggerTest(t)

	_, _ = worldDB.Exec("INSERT INTO areatrigger_tavern (id, name) VALUES (1, 'Inn')")

	// Set in-flight state
	sess.inFlight = true

	payload := protocol.NewBuffer(4)
	payload.WriteU32(1)

	if !sess.handleAreaTrigger(ctx, payload.Bytes()) {
		t.Fatal("expected handleAreaTrigger to succeed")
	}

	if sess.player.ExtraFlags&0x00000001 != 0 {
		t.Fatal("expected resting flag NOT to be set when player is in flight")
	}
}

func TestAreaTriggerTavernResting(t *testing.T) {
	ctx := context.Background()
	_, sess, worldDB, _ := setupAreaTriggerTest(t)

	_, _ = worldDB.Exec("INSERT INTO areatrigger_tavern (id, name) VALUES (50, 'Goldshire Inn')")

	payload := protocol.NewBuffer(4)
	payload.WriteU32(50)

	if !sess.handleAreaTrigger(ctx, payload.Bytes()) {
		t.Fatal("expected handleAreaTrigger to succeed")
	}

	if sess.player.ExtraFlags&0x00000001 == 0 {
		t.Fatal("expected resting flag to be set after tavern trigger")
	}
}

func TestAreaTriggerQuestCompletion(t *testing.T) {
	ctx := context.Background()
	_, sess, worldDB, charDB := setupAreaTriggerTest(t)

	const questID = 8820
	const triggerID = 120
	_, _ = worldDB.Exec("INSERT INTO areatrigger_involvedrelation (id, quest) VALUES (?, ?)", triggerID, questID)
	_, _ = charDB.Exec("INSERT INTO character_queststatus (guid, quest, status) VALUES (?, ?, ?)", sess.playerGUID, questID, questStatusIncomplete)

	sess.player.QuestLog[0] = questLogEntry{
		QuestID: questID,
		State:   questCompleteStateFlag(questStatusIncomplete),
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(triggerID)

	if !sess.handleAreaTrigger(ctx, payload.Bytes()) {
		t.Fatal("expected handleAreaTrigger to succeed")
	}

	var status int
	_ = charDB.QueryRow("SELECT status FROM character_queststatus WHERE guid = ? AND quest = ?", sess.playerGUID, questID).Scan(&status)
	if status != int(questStatusComplete) {
		t.Fatalf("expected quest status complete (%d), got %d", questStatusComplete, status)
	}
	if sess.player.QuestLog[0].State != questCompleteStateFlag(questStatusComplete) {
		t.Fatalf("expected quest log slot state to be completed")
	}
}

func TestAreaTriggerTeleportAndGhostRevive(t *testing.T) {
	ctx := context.Background()
	_, sess, worldDB, charDB := setupAreaTriggerTest(t)

	const triggerID = 300
	_, _ = worldDB.Exec("INSERT INTO areatrigger_teleport (id, name, target_map, target_position_x, target_position_y, target_position_z, target_orientation) VALUES (?, 'Dungeon Entrance', 33, 40.0, 50.0, 10.0, 3.14)", triggerID)

	// Set ghost flag & corpse
	sess.player.PlayerFlags |= playerFlagGhost
	sess.player.Health = 1
	_, _ = charDB.Exec("INSERT INTO corpse (guid, posX, posY, posZ, orientation, mapId, displayId, itemCache, bytes1, bytes2, flags, dynFlags, time, corpseType, instanceId) VALUES (?, 100.0, 200.0, 30.0, 0.0, 0, 0, '', 0, 0, 0, 0, 1000, 1, 0)", sess.playerGUID)

	payload := protocol.NewBuffer(4)
	payload.WriteU32(triggerID)

	if !sess.handleAreaTrigger(ctx, payload.Bytes()) {
		t.Fatal("expected handleAreaTrigger to succeed")
	}

	// Verify teleport
	if sess.player.Map != 33 || sess.player.X != 40.0 || sess.player.Y != 50.0 || sess.player.Z != 10.0 {
		t.Fatalf("unexpected player position after teleport: Map=%d X=%f Y=%f Z=%f", sess.player.Map, sess.player.X, sess.player.Y, sess.player.Z)
	}

	// Verify ghost resurrected
	if sess.player.PlayerFlags&playerFlagGhost != 0 {
		t.Fatal("expected ghost flag cleared after dungeon entrance resurrection")
	}
	if sess.player.Health != 500 {
		t.Fatalf("expected half health (500) after resurrection, got %d", sess.player.Health)
	}
}

func TestAreaTriggerRadiusDBCValidation(t *testing.T) {
	ctx := context.Background()
	srv, sess, worldDB, _ := setupAreaTriggerTest(t)

	// Create real DBC in TempDir
	dbcDir := t.TempDir()
	srv.Data = wotlk.NewStore(dbcDir)

	const fieldCount = 10
	recBytes := make([]byte, fieldCount*4)
	// ID 500, Map 0, X 100, Y 200, Z 30, Radius 10
	importBinaryPutUint32(recBytes[0:4], 500)
	importBinaryPutUint32(recBytes[4:8], 0)
	importBinaryPutFloat32(recBytes[8:12], 100.0)
	importBinaryPutFloat32(recBytes[12:16], 200.0)
	importBinaryPutFloat32(recBytes[16:20], 30.0)
	importBinaryPutFloat32(recBytes[20:24], 10.0) // Radius 10.0

	header := make([]byte, 20)
	copy(header, "WDBC")
	importBinaryPutUint32(header[4:8], 1)
	importBinaryPutUint32(header[8:12], fieldCount)
	importBinaryPutUint32(header[12:16], fieldCount*4)
	importBinaryPutUint32(header[16:20], 1)

	_ = os.WriteFile(filepath.Join(dbcDir, "AreaTrigger.dbc"), append(header, append(recBytes, 0)...), 0o644)

	_, _ = worldDB.Exec("INSERT INTO areatrigger_tavern (id, name) VALUES (500, 'Inn')")

	// Player is at 100.0, 200.0, 30.0 (distance 0 <= 10) -> Success
	payload := protocol.NewBuffer(4)
	payload.WriteU32(500)

	if !sess.handleAreaTrigger(ctx, payload.Bytes()) {
		t.Fatal("expected handleAreaTrigger to succeed")
	}
	if sess.player.ExtraFlags&0x00000001 == 0 {
		t.Fatal("expected resting flag set when inside radius")
	}

	// Move player far away: 500.0, 500.0, 30.0 (distance ~424 > 10)
	sess.player.ExtraFlags &= ^uint32(0x00000001) // reset
	sess.player.X = 500.0
	sess.player.Y = 500.0

	if !sess.handleAreaTrigger(ctx, payload.Bytes()) {
		t.Fatal("expected handleAreaTrigger to return true (ignored out of range)")
	}
	if sess.player.ExtraFlags&0x00000001 != 0 {
		t.Fatal("expected resting flag NOT to be set when player is out of radius")
	}
}

func importBinaryPutUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func importBinaryPutFloat32(b []byte, v float32) {
	importBinaryPutUint32(b, math.Float32bits(v))
}
