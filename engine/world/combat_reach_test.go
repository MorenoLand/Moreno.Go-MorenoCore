package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestCreatureCombatReachUsesModelInfo(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, maxlevel INTEGER, unit_class INTEGER, exp INTEGER, BaseAttackTime INTEGER, HealthModifier REAL, ArmorModifier REAL, DamageModifier REAL, unit_flags INTEGER, flags_extra INTEGER, modelid1 INTEGER)",
		"CREATE TABLE creature_model_info (DisplayID INTEGER PRIMARY KEY, CombatReach REAL)",
		"INSERT INTO creature_template VALUES (68, 20, 1, 0, 2000, 1, 1, 1, 0, 0, 9001)",
		"INSERT INTO creature_model_info VALUES (9001, 4.25)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}, creatureStatsCache: make(map[uint32]creatureStats)}
	stats := server.loadCreatureStats(context.Background(), 68)
	if stats.CombatReach != 4.25 {
		t.Fatalf("combat reach=%v", stats.CombatReach)
	}
	motion := server.motionFor(context.Background(), 10, 68, 0, 0, 0, 0, 0, 0, 2.5)
	if motion.CombatReach != 4.25 {
		t.Fatalf("motion combat reach=%v", motion.CombatReach)
	}
}
