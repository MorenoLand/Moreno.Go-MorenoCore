package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCreatureQuestMenuAndDialogStatus(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature_queststarter (id INTEGER NOT NULL, quest INTEGER NOT NULL)",
		"CREATE TABLE creature_questender (id INTEGER NOT NULL, quest INTEGER NOT NULL)",
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, QuestLevel INTEGER NOT NULL, MinLevel INTEGER NOT NULL, Flags INTEGER NOT NULL, LogTitle TEXT)",
		"CREATE TABLE character_queststatus (guid INTEGER NOT NULL, quest INTEGER NOT NULL, status INTEGER NOT NULL)",
		"INSERT INTO creature_queststarter VALUES (68, 100)",
		"INSERT INTO creature_questender VALUES (68, 101)",
		"INSERT INTO quest_template VALUES (100, 5, 1, 0, 'Start Quest')",
		"INSERT INTO quest_template VALUES (101, 6, 1, 0, 'Return Quest')",
		"INSERT INTO character_queststatus VALUES (99, 101, 3)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, CharactersStore: store, Config: config.Default()}
	state := &session{server: server, playerGUID: 99, player: &playerState{GUID: 99, Level: 10}}
	quests, err := state.loadCreatureQuestMenu(context.Background(), 68, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(quests) != 2 || quests[0].ID != 101 || quests[0].Icon != 4 || quests[1].ID != 100 || quests[1].Icon != 2 {
		t.Fatalf("quests=%+v", quests)
	}
	status, err := state.questDialogStatus(context.Background(), 68)
	if err != nil || status != questDialogIncomplete {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if _, err := db.Exec("UPDATE character_queststatus SET status = 1 WHERE guid = 99 AND quest = 101"); err != nil {
		t.Fatal(err)
	}
	status, err = state.questDialogStatus(context.Background(), 68)
	if err != nil || status != questDialogReward {
		t.Fatalf("reward status=%d err=%v", status, err)
	}
	packet := buildGossipMessage(gossipMenuState{TitleID: 123, Quests: quests})
	reader := protocol.NewReader(packet)
	if sender, err := reader.ReadU64(); err != nil || sender != 0 {
		t.Fatalf("sender=%d err=%v", sender, err)
	}
	for _, expected := range []uint32{0, 123, 0} {
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("gossip value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if questCount, err := reader.ReadU32(); err != nil || questCount != 2 {
		t.Fatalf("quest count=%d err=%v", questCount, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadI32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if title, err := reader.ReadCString(); err != nil || title != "Return Quest" {
		t.Fatalf("title=%q err=%v", title, err)
	}
}
