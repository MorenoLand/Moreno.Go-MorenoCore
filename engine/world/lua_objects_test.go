package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
)

func TestLuaWorldObjectBindings(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, modelid INTEGER NOT NULL, curhealth INTEGER NOT NULL)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, name TEXT NOT NULL, modelid1 INTEGER NOT NULL, maxlevel INTEGER NOT NULL)",
		"CREATE TABLE gameobject (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL)",
		"CREATE TABLE gameobject_template (entry INTEGER PRIMARY KEY, displayId INTEGER NOT NULL, name TEXT NOT NULL)",
		"INSERT INTO creature VALUES (321, 68, 3167, 100)",
		"INSERT INTO creature_template VALUES (68, 'Stormwind Guard', 3167, 80)",
		"INSERT INTO gameobject VALUES (654, 9001, 0, 4, 3, 2)",
		"INSERT INTO gameobject_template VALUES (9001, 1234, 'Test Chest')",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}, Config: config.Default(), hiddenGameObjects: make(map[uint64]struct{}), creatureAuras: make(map[uint64]map[uint32]struct{})}
	creatureGUID := uint64(321) | uint64(68)<<24 | uint64(0xF130)<<48
	state := &session{server: server, accountName: "TEST", playerLoaded: true, playerGUID: 99, selection: creatureGUID, player: &playerState{GUID: 99, Name: "Tester", Map: 0, X: 0, Y: 0, Z: 0}, auras: make(map[uint32]struct{})}
	creature := state.luaCreature(context.Background(), creatureGUID)
	if creature == nil || creature.Type != "Creature" {
		t.Fatalf("creature=%v", creature)
	}
	if values, err := creature.Methods["GetName"](context.Background(), nil); err != nil || values[0] != "Stormwind Guard" {
		t.Fatalf("name=%v err=%v", values, err)
	}
	if _, err := creature.Methods["SetHealth"](context.Background(), []any{float64(50)}); err != nil {
		t.Fatal(err)
	}
	var health int
	if err := db.QueryRow("SELECT curhealth FROM creature WHERE guid = 321").Scan(&health); err != nil || health != 50 {
		t.Fatalf("health=%d err=%v", health, err)
	}
	player := state.luaPlayer()
	if values, err := player.Methods["GetSelection"](context.Background(), nil); err != nil || len(values) != 1 || values[0].(*scripting.Object).Type != "Creature" {
		t.Fatalf("selection=%v err=%v", values, err)
	}
	objectValues, err := player.Methods["GetNearestGameObject"](context.Background(), []any{float64(10)})
	if err != nil || len(objectValues) != 1 || objectValues[0] == nil {
		t.Fatalf("nearest=%v err=%v", objectValues, err)
	}
	object := objectValues[0].(*scripting.Object)
	if object.Type != "GameObject" || object.Fields["Name"] != "Test Chest" {
		t.Fatalf("object=%v", object)
	}
	if _, err := object.Methods["RemoveFromWorld"](context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !server.isGameObjectHidden(object.Fields["GUID"].(uint64)) {
		t.Fatalf("object was not hidden")
	}
}
