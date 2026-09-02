package world

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

// TestRealWorldDBNearbyCreatures runs the live visibility query against the
// repository's world.db to catch dialect/SQL breakage the in-memory
// schemas cannot.
func TestRealWorldDBNearbyCreatures(t *testing.T) {
	if testing.Short() {
		t.Skip("long test against real world.db")
	}
	dbPath := "../../bin/world.db"
	if fi, err := os.Stat(dbPath); err != nil || fi.Size() == 0 {
		dbPath = "../../world.db"
		if fi2, err2 := os.Stat(dbPath); err2 != nil || fi2.Size() == 0 {
			t.Skip("real world.db not found, skipping")
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Skipf("world.db open failed: %v", err)
	}
	defer db.Close()
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, Config: config.Default()}
	state := playerState{Map: 0, X: -7647, Y: -3051, Z: 100, GUID: 1}
	packet, count, err := server.buildNearbyCreatureUpdates(context.Background(), state)
	if err != nil {
		t.Fatalf("creature visibility query failed: %v", err)
	}
	if count == 0 {
		t.Fatal("real world.db returned zero nearby creatures around Elwynn spawn")
	}
	if packet == nil {
		t.Fatal("no update packet built")
	}
}

