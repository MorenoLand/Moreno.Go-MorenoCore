package world

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, name TEXT NOT NULL, modelid1 INTEGER NOT NULL, maxlevel INTEGER NOT NULL, gossip_menu_id INTEGER NOT NULL, npcflag INTEGER NOT NULL)",
		"CREATE TABLE gameobject (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL)",
		"CREATE TABLE gameobject_template (entry INTEGER PRIMARY KEY, displayId INTEGER NOT NULL, name TEXT NOT NULL)",
		"INSERT INTO creature VALUES (321, 68, 3167, 100)",
		"INSERT INTO creature_template VALUES (68, 'Stormwind Guard', 3167, 80, 0, 1)",
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
	if values, err := creature.Methods["IsGossip"](context.Background(), nil); err != nil || values[0] != true {
		t.Fatalf("gossip=%v err=%v", values, err)
	}
	if _, err := creature.Methods["AddAura"](context.Background(), []any{float64(123)}); err != nil {
		t.Fatal(err)
	}
	if values, err := creature.Methods["HasAura"](context.Background(), []any{float64(123)}); err != nil || values[0] != true {
		t.Fatalf("aura values=%v err=%v", values, err)
	}
	if _, err := creature.Methods["RemoveAura"](context.Background(), []any{float64(123)}); err != nil {
		t.Fatal(err)
	}
	if values, err := creature.Methods["HasAura"](context.Background(), []any{float64(123)}); err != nil || values[0] != false {
		t.Fatalf("removed aura values=%v err=%v", values, err)
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
	if values, err := object.Methods["GetDisplayId"](context.Background(), nil); err != nil || values[0] != uint32(1234) {
		t.Fatalf("display=%v err=%v", values, err)
	}
	if values, err := object.Methods["GetGoState"](context.Background(), nil); err != nil || values[0] != uint32(1) {
		t.Fatalf("go state=%v err=%v", values, err)
	}
	if _, err := object.Methods["Despawn"](context.Background(), nil); err != nil || !server.isGameObjectHidden(object.Fields["GUID"].(uint64)) {
		t.Fatalf("despawn err=%v", err)
	}
	if _, err := object.Methods["Respawn"](context.Background(), nil); err != nil || server.isGameObjectHidden(object.Fields["GUID"].(uint64)) {
		t.Fatalf("respawn err=%v", err)
	}
	if _, err := object.Methods["RemoveFromWorld"](context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !server.isGameObjectHidden(object.Fields["GUID"].(uint64)) {
		t.Fatalf("object was not hidden")
	}
}

func TestLuaPlayerMuteStoresAbsoluteExpiry(t *testing.T) {
	auth, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Close()
	if _, err := auth.Exec("CREATE TABLE account (id INTEGER PRIMARY KEY, mutetime INTEGER NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Exec("INSERT INTO account (id, mutetime) VALUES (7, 0)"); err != nil {
		t.Fatal(err)
	}
	server := &Server{AuthStore: &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: auth}}
	session := &session{server: server, accountID: 7, playerLoaded: true, player: &playerState{GUID: 99, Name: "Tester"}}
	seconds := uint32(60)
	before := time.Now().Unix()
	if _, err := session.luaPlayer().Methods["Mute"](context.Background(), []any{float64(seconds)}); err != nil {
		t.Fatal(err)
	}
	var stored int64
	if err := auth.QueryRow("SELECT mutetime FROM account WHERE id = 7").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored < before+int64(seconds) || session.muteTime != stored {
		t.Fatalf("stored mute expiry=%d session=%d before=%d", stored, session.muteTime, before)
	}
}

func TestLuaPlayerGameplayBindings(t *testing.T) {
	characters, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer characters.Close()
	characters.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE character_inventory (guid INTEGER NOT NULL, item INTEGER NOT NULL)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER NOT NULL, count INTEGER NOT NULL)",
		"INSERT INTO character_inventory VALUES (99, 5001)",
		"INSERT INTO item_instance VALUES (5001, 1001, 3)",
	} {
		if _, err := characters.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: characters}, Config: config.Default()}
	var rawCount int
	if err := characters.QueryRow("SELECT COALESCE(SUM(ii.count), 0) FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = ? AND ii.itemEntry = ?", 99, 1001).Scan(&rawCount); err != nil || rawCount != 3 {
		t.Fatalf("raw item count=%d err=%v", rawCount, err)
	}
	sess := &session{server: server, playerLoaded: true, player: &playerState{GUID: 99, Race: 1, Class: 8, Gender: 0, Level: 20, GuildID: 7, Health: 80, MaxHealth: 100, Powers: [7]uint32{40}, MaxPowers: [7]uint32{100}, MountDisplayID: 123, Spells: []learnedSpell{{ID: 133, Active: true}}, Skills: []playerSkill{{Skill: 98, Value: 300, Max: 300}}}, isMoving: true, isSwimming: true}
	player := sess.luaPlayer()
	if values, err := player.Methods["HasSpell"](context.Background(), []any{float64(133)}); err != nil || values[0] != true {
		t.Fatalf("spell values=%v err=%v", values, err)
	}
	if values, err := player.Methods["HasSkill"](context.Background(), []any{float64(98)}); err != nil || values[0] != true {
		t.Fatalf("skill values=%v err=%v", values, err)
	}
	if values, err := player.Methods["GetItemCount"](context.Background(), []any{float64(1001)}); err != nil || values[0] != uint32(3) {
		t.Fatalf("item values=%v err=%v", values, err)
	}
	if values, err := player.Methods["HasItem"](context.Background(), []any{float64(1001), float64(3)}); err != nil || values[0] != true {
		t.Fatalf("has item values=%v err=%v", values, err)
	}
	for name, want := range map[string]any{"GetGuildId": uint32(7), "GetTeam": uint32(0), "GetPower": uint32(40), "GetMaxPower": uint32(100), "GetPowerType": uint8(0), "IsInWater": true, "IsMoving": true, "CanFly": true} {
		values, err := player.Methods[name](context.Background(), nil)
		if err != nil || len(values) != 1 || values[0] != want {
			t.Fatalf("%s values=%v err=%v", name, values, err)
		}
	}
}
