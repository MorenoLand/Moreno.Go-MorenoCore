//go:build ignore

package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

// TestBinDatabasesLoginFlow exercises the login-time state and visibility
// path against the runtime databases used by `go run . --work=bin/`.
func TestBinDatabasesLoginFlow(t *testing.T) {
	charDB, err := sql.Open("sqlite", "../../bin/characters.db")
	if err != nil {
		t.Skipf("bin/characters.db open failed: %v", err)
	}
	defer charDB.Close()
	worldDB, err := sql.Open("sqlite", "../../bin/world.db")
	if err != nil {
		t.Skipf("bin/world.db open failed: %v", err)
	}
	defer worldDB.Close()
	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: worldDB}
	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: charDB}
	server := &Server{WorldStore: worldStore, CharactersStore: charStore, Config: config.Default(), sessions: make(map[*session]struct{})}
	state := &session{server: server, playerGUID: 7, accountID: 17}
	ctx := context.Background()

	player, err := state.loadPlayerState(ctx, 7)
	if err != nil {
		t.Fatalf("loadPlayerState failed: %v", err)
	}
	t.Logf("player %s level %d map %d at %.1f,%.1f", player.Name, player.Level, player.Map, player.X, player.Y)

	// Raw sanity: same shape of query directly on the same connection.
	var raw int64
	if err := worldDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM creature AS c JOIN creature_template AS t ON t.entry = c.id LEFT JOIN game_event_creature AS gec ON gec.guid = c.guid WHERE c.map = ? AND c.position_x BETWEEN ? AND ? AND c.position_y BETWEEN ? AND ? AND (? OR c.phaseMask = 0 OR (c.phaseMask & 1) <> 0) AND (? OR (COALESCE(t.flags_extra, 0) & 1) = 0) AND (gec.eventEntry IS NULL OR gec.eventEntry = 0)", player.Map, float64(player.X)-100, float64(player.X)+100, float64(player.Y)-100, float64(player.Y)+100, false, false).Scan(&raw); err != nil {
		t.Fatalf("raw sanity query failed: %v", err)
	}
	t.Logf("raw sanity rows: %d", raw)

	packet, count, err := server.buildNearbyCreatureUpdates(ctx, player)
	if err != nil {
		t.Fatalf("creature visibility query failed: %v", err)
	}
	if count == 0 || packet == nil {
		t.Fatal("zero nearby creatures returned from bin/world.db")
	}
	t.Logf("nearby creatures: %d", count)

	goPacket, goCount, err := server.buildNearbyGameObjectUpdates(ctx, player)
	if err != nil {
		t.Fatalf("gameobject visibility query failed: %v", err)
	}
	t.Logf("nearby gameobjects: %d (packet=%v)", goCount, goPacket != nil)

	// The per-second world tick path must not fail either.
	server.updateActiveCreatures(ctx)
	t.Log("world tick creature motion pass ok")
}

