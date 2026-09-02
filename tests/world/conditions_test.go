//go:build ignore

package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
)

func setupConditionsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE conditions (SourceTypeOrReferenceId INTEGER NOT NULL DEFAULT 0, SourceGroup INTEGER NOT NULL DEFAULT 0, SourceEntry INTEGER NOT NULL DEFAULT 0, SourceId INTEGER NOT NULL DEFAULT 0, ElseGroup INTEGER NOT NULL DEFAULT 0, ConditionTypeOrReference INTEGER NOT NULL DEFAULT 0, ConditionTarget INTEGER NOT NULL DEFAULT 0, ConditionValue1 INTEGER NOT NULL DEFAULT 0, ConditionValue2 INTEGER NOT NULL DEFAULT 0, ConditionValue3 INTEGER NOT NULL DEFAULT 0, NegativeCondition INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE game_event (eventEntry INTEGER PRIMARY KEY, start_time TEXT, end_time TEXT, occurence INTEGER NOT NULL DEFAULT 5184000, length INTEGER NOT NULL DEFAULT 2592000, holiday INTEGER NOT NULL DEFAULT 0, holidayStage INTEGER NOT NULL DEFAULT 0, description TEXT, world_event INTEGER NOT NULL DEFAULT 0, announce INTEGER DEFAULT 2)",
		"CREATE TABLE character_queststatus (guid INTEGER NOT NULL, quest INTEGER NOT NULL, status INTEGER NOT NULL, PRIMARY KEY (guid, quest))",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER NOT NULL, quest INTEGER NOT NULL, PRIMARY KEY (guid, quest))",
		"CREATE TABLE character_reputation (guid INTEGER NOT NULL, faction INTEGER NOT NULL, standing INTEGER NOT NULL)",
		"CREATE TABLE character_skills (guid INTEGER NOT NULL, skill INTEGER NOT NULL, value INTEGER NOT NULL)",
		"CREATE TABLE character_spell (guid INTEGER NOT NULL, spell INTEGER NOT NULL)",
		"CREATE TABLE character_inventory (guid INTEGER NOT NULL, bag INTEGER NOT NULL, slot INTEGER NOT NULL, item INTEGER NOT NULL)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER NOT NULL, owner_guid INTEGER NOT NULL, count INTEGER NOT NULL DEFAULT 1)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestGossipOptionConditionsSeasonalAndClass(t *testing.T) {
	db := setupConditionsDB(t)
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, CharactersStore: charStore, Config: config.Default()}
	// Human warrior level 10.
	state := &session{server: server, playerGUID: 42, player: &playerState{GUID: 42, Race: 1, Class: 1, Level: 10, Zone: 12, Map: 0}}
	ctx := context.Background()

	// Menu 5000 option 1: only during event 24 (not scheduled -> hidden).
	// Menu 5000 option 2: class 4 (warrior only -> shown for class 1? no: hidden).
	// Menu 5000 option 3: class 1 -> shown.
	// Menu 5000 option 4: ElseGroup OR - race 2 OR (race 1 AND level >= 8).
	for _, row := range []string{
		"INSERT INTO conditions VALUES (14, 5000, 1, 0, 0, 12, 0, 24, 0, 0, 0)",
		"INSERT INTO conditions VALUES (14, 5000, 2, 0, 0, 15, 0, 4, 0, 0, 0)",
		"INSERT INTO conditions VALUES (14, 5000, 3, 0, 0, 15, 0, 1, 0, 0, 0)",
		"INSERT INTO conditions VALUES (14, 5000, 4, 0, 0, 16, 0, 2, 0, 0, 0)",
		"INSERT INTO conditions VALUES (14, 5000, 4, 0, 1, 16, 0, 1, 0, 0, 0)",
		"INSERT INTO conditions VALUES (14, 5000, 4, 0, 1, 27, 0, 8, 3, 0, 0)",
		"INSERT INTO conditions VALUES (14, 5000, 5, 0, 0, 8, 0, 77, 0, 0, 0)",
	} {
		if _, err := db.Exec(row); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		option   uint32
		expected bool
		reason   string
	}{
		{1, false, "active event 24 not running"},
		{2, false, "class filter warrior-only vs class 4"},
		{3, true, "class filter matches class 1"},
		{4, true, "else group race 1 + level>=8 satisfied"},
		{5, false, "quest 77 not rewarded"},
	}
	for _, tc := range cases {
		got, err := state.meetGossipOptionConditions(ctx, 5000, tc.option, 68)
		if err != nil {
			t.Fatalf("option %d: %v", tc.option, err)
		}
		if got != tc.expected {
			t.Errorf("option %d: expected %v (%s), got %v", tc.option, tc.expected, tc.reason, got)
		}
	}

	// Reward quest 77 -> option 5 appears.
	if _, err := db.Exec("INSERT INTO character_queststatus_rewarded VALUES (42, 77)"); err != nil {
		t.Fatal(err)
	}
	if got, _ := state.meetGossipOptionConditions(ctx, 5000, 5, 68); !got {
		t.Error("option 5 should pass once quest 77 is rewarded")
	}
}

func TestActiveGameEventScheduling(t *testing.T) {
	db := setupConditionsDB(t)
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store}
	ctx := context.Background()

	// Event 2: daily 2h window always inside a wide range -> active.
	// Event 3: ended long ago -> inactive. Event 4: world_event driven -> inactive.
	for _, row := range []string{
		"INSERT INTO game_event VALUES (2, '2010-01-01 00:00:00', '2030-01-01 00:00:00', 1440, 1440, 0, 0, 'daily', 0, 2)",
		"INSERT INTO game_event VALUES (3, '2004-01-01 00:00:00', '2005-01-01 00:00:00', 1440, 120, 0, 0, 'old', 0, 2)",
		"INSERT INTO game_event VALUES (4, '2010-01-01 00:00:00', '2030-01-01 00:00:00', 1440, 120, 0, 0, 'world', 1, 2)",
	} {
		if _, err := db.Exec(row); err != nil {
			t.Fatal(err)
		}
	}
	active := server.activeGameEvents(ctx)
	if _, ok := active[2]; !ok {
		t.Error("event 2 should be active")
	}
	if _, ok := active[3]; ok {
		t.Error("event 3 should be inactive")
	}
	if _, ok := active[4]; ok {
		t.Error("world event 4 must stay inactive without its state machinery")
	}
}

