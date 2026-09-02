//go:build ignore

package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestKillXPGainMatchesTrinityFormula(t *testing.T) {
	server := &Server{}
	ctx := context.Background()

	// Same-level kill: ((pl*5 + base) * 24 / 10 + 1) / 2 with base from the
	// fallback curve (xpCurve[level]/100).
	gain := server.killXPGain(ctx, 10, 10)
	if gain == 0 {
		t.Fatal("same-level kill must award XP")
	}
	// Gray mob (level 1 vs level 60 player) awards none.
	if g := server.killXPGain(ctx, 60, 1); g != 0 {
		t.Fatalf("gray kill should award 0, got %d", g)
	}
	// Higher mob beats same-level mob in XP.
	if high := server.killXPGain(ctx, 10, 14); high <= gain {
		t.Fatalf("+4 mob (%d) should beat same level (%d)", high, gain)
	}
}

func TestQuestKillCreditAndCompletion(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, maxlevel INTEGER, minlevel INTEGER, KillCredit1 INTEGER NOT NULL DEFAULT 0, KillCredit2 INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, RequiredNpcOrGo1 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGo2 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGo3 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGo4 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGoCount1 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGoCount2 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGoCount3 INTEGER NOT NULL DEFAULT 0, RequiredNpcOrGoCount4 INTEGER NOT NULL DEFAULT 0, RequiredItemId1 INTEGER NOT NULL DEFAULT 0, RequiredItemId2 INTEGER NOT NULL DEFAULT 0, RequiredItemId3 INTEGER NOT NULL DEFAULT 0, RequiredItemId4 INTEGER NOT NULL DEFAULT 0, RequiredItemId5 INTEGER NOT NULL DEFAULT 0, RequiredItemId6 INTEGER NOT NULL DEFAULT 0, RequiredItemCount1 INTEGER NOT NULL DEFAULT 0, RequiredItemCount2 INTEGER NOT NULL DEFAULT 0, RequiredItemCount3 INTEGER NOT NULL DEFAULT 0, RequiredItemCount4 INTEGER NOT NULL DEFAULT 0, RequiredItemCount5 INTEGER NOT NULL DEFAULT 0, RequiredItemCount6 INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE exploration_basexp (level INTEGER PRIMARY KEY, basexp INTEGER NOT NULL)",
		"CREATE TABLE player_xp_for_level (Level INTEGER PRIMARY KEY, Experience INTEGER NOT NULL)",
		"CREATE TABLE character_queststatus (guid INTEGER NOT NULL, quest INTEGER NOT NULL, status INTEGER NOT NULL, mobcount1 INTEGER NOT NULL DEFAULT 0, mobcount2 INTEGER NOT NULL DEFAULT 0, mobcount3 INTEGER NOT NULL DEFAULT 0, mobcount4 INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (guid, quest))",
		"INSERT INTO creature_template VALUES (68, 75, 75, 0, 0)",
		"INSERT INTO creature_template VALUES (6, 5, 5, 68, 0)", // credit redirector
		"INSERT INTO quest_template (ID, RequiredNpcOrGo1, RequiredNpcOrGoCount1) VALUES (300, 68, 2)",
		"INSERT INTO quest_template (ID, RequiredNpcOrGo1, RequiredNpcOrGoCount1) VALUES (301, 6, 1)",
		"INSERT INTO exploration_basexp VALUES (10, 91)",
		"INSERT INTO character_queststatus VALUES (99, 300, 3, 0, 0, 0, 0)",
		"INSERT INTO character_queststatus VALUES (99, 301, 3, 0, 0, 0, 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, CharactersStore: store, sessions: make(map[*session]struct{})}
	state := &session{server: server, playerGUID: 99, player: &playerState{GUID: 99, Level: 10, QuestLog: [playerQuestLogSlots]questLogEntry{{QuestID: 300}, {QuestID: 301}}}}
	ctx := context.Background()

	victim := creatureWorldGUID(7, 68)

	// Two kills complete quest 300.
	state.creditQuestKills(ctx, 68, victim)
	if state.player.QuestLog[0].Counters[0] != 1 {
		t.Fatalf("expected counter 1 after first kill, got %d", state.player.QuestLog[0].Counters[0])
	}
	if state.player.QuestLog[0].State != 0 {
		t.Fatal("quest must remain incomplete after one of two kills")
	}
	state.creditQuestKills(ctx, 68, victim)
	if state.player.QuestLog[0].Counters[0] != 2 {
		t.Fatalf("expected counter 2, got %d", state.player.QuestLog[0].Counters[0])
	}
	if state.player.QuestLog[0].State != 1 {
		t.Fatal("quest should be complete after required kills")
	}

	// KillCredit redirector entry 6 credits objective on template 68.
	state.creditQuestKills(ctx, 6, creatureWorldGUID(8, 6))
	if state.player.QuestLog[1].Counters[0] != 1 {
		t.Fatalf("kill credit redirect failed, counter %d", state.player.QuestLog[1].Counters[0])
	}

	// Overkill protection: extra kills past the requirement don't increment.
	state.creditQuestKills(ctx, 68, victim)
	if state.player.QuestLog[0].Counters[0] != 2 {
		t.Fatalf("counter must cap at requirement, got %d", state.player.QuestLog[0].Counters[0])
	}
}

