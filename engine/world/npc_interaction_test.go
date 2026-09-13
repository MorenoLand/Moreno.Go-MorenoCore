package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestCanInteractWithNPCRequiresMapRangeAndFlag(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER, map INTEGER, position_x REAL, position_y REAL, position_z REAL)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, npcflag INTEGER)",
		"INSERT INTO creature VALUES (7, 101, 0, 10, 0, 0)",
		"INSERT INTO creature_template VALUES (101, 128)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	sess := &session{server: &Server{WorldStore: store}, player: &playerState{Map: 0, X: 0, Y: 0, Z: 0}}
	guid := creatureWorldGUID(7, 101)
	if sess.canInteractWithNPC(context.Background(), guid, uint64(unitNPCFlagVendor)) {
		t.Fatal("remote vendor should not be interactable")
	}
	if _, err := db.Exec("UPDATE creature SET position_x = 3"); err != nil {
		t.Fatal(err)
	}
	if !sess.canInteractWithNPC(context.Background(), guid, uint64(unitNPCFlagVendor)) {
		t.Fatal("nearby vendor should be interactable")
	}
	if sess.canInteractWithNPC(context.Background(), guid, 0x20000) {
		t.Fatal("missing NPC flag should reject interaction")
	}
}
