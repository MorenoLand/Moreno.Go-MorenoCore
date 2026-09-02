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

func TestQuestChainPrerequisites(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, QuestLevel INTEGER NOT NULL, MinLevel INTEGER NOT NULL, Flags INTEGER NOT NULL, LogTitle TEXT)",
		"CREATE TABLE quest_template_addon (ID INTEGER PRIMARY KEY, MaxLevel INTEGER, AllowableClasses INTEGER, AllowableRaces INTEGER, PrevQuestID INTEGER, NextQuestID INTEGER, ExclusiveGroup INTEGER)",
		"CREATE TABLE character_queststatus (guid INTEGER NOT NULL, quest INTEGER NOT NULL, status INTEGER NOT NULL)",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER NOT NULL, quest INTEGER NOT NULL)",
		"INSERT INTO quest_template VALUES (201, 5, 1, 0, 'Chain Part 1')",
		"INSERT INTO quest_template VALUES (202, 6, 1, 0, 'Chain Part 2')",
		"INSERT INTO quest_template_addon VALUES (202, 80, 0, 0, 201, 0, 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, CharactersStore: store, Config: config.Default()}
	state := &session{server: server, playerGUID: 99, player: &playerState{GUID: 99, Level: 10}}
	ctx := context.Background()

	// 1. Part 2 should NOT be available because Part 1 is not rewarded yet
	canTake, err := state.canTakeQuest(ctx, 202)
	if err != nil {
		t.Fatal(err)
	}
	if canTake {
		t.Fatal("expected canTake=false for Part 2 when Part 1 not rewarded")
	}

	// 2. Mark Part 1 as rewarded
	if _, err := db.Exec("INSERT INTO character_queststatus_rewarded VALUES (99, 201)"); err != nil {
		t.Fatal(err)
	}

	// 3. Part 2 should now be available!
	canTake, err = state.canTakeQuest(ctx, 202)
	if err != nil {
		t.Fatal(err)
	}
	if !canTake {
		t.Fatal("expected canTake=true for Part 2 after Part 1 rewarded")
	}
}

func TestQuestgiverAcceptQuestItemStarterAndGuidZero(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, QuestLevel INTEGER NOT NULL, MinLevel INTEGER NOT NULL, Flags INTEGER NOT NULL, LogTitle TEXT, AllowableRaces INTEGER DEFAULT 0)",
		"CREATE TABLE character_queststatus (guid INTEGER NOT NULL, quest INTEGER NOT NULL, status INTEGER NOT NULL)",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER NOT NULL, quest INTEGER NOT NULL)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, startquest INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER NOT NULL, owner_guid INTEGER, count INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"INSERT INTO quest_template VALUES (301, 5, 1, 0, 'Letter Quest', 0)",
		"INSERT INTO item_template VALUES (5001, 301)",
		"INSERT INTO item_instance VALUES (9001, 5001, 99, 1)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{WorldStore: store, CharactersStore: store, Config: config.Default()}
	state := &session{server: server, playerGUID: 99, playerLoaded: true, player: &playerState{GUID: 99, Level: 10}}
	ctx := context.Background()

	// 1. Accept quest via item GUID (highguid 0x4000)
	itemGUID := uint64(9001) | (uint64(0x4000) << 48)
	acceptPayload := protocol.NewBuffer(12)
	acceptPayload.WriteU64(itemGUID)
	acceptPayload.WriteU32(301)
	if !state.handleQuestgiverAcceptQuest(ctx, acceptPayload.Bytes()) {
		t.Fatal("handleQuestgiverAcceptQuest failed for item quest starter")
	}

	// Verify quest was added to quest log
	if state.player.QuestLog[0].QuestID != 301 {
		t.Fatalf("expected quest 301 in player quest log slot 0, got %d", state.player.QuestLog[0].QuestID)
	}

	// 2. Accept another quest with GUID 0
	if _, err := db.Exec("INSERT INTO quest_template VALUES (302, 5, 1, 0, 'Direct Quest', 0)"); err != nil {
		t.Fatal(err)
	}
	acceptZeroPayload := protocol.NewBuffer(12)
	acceptZeroPayload.WriteU64(0)
	acceptZeroPayload.WriteU32(302)
	if !state.handleQuestgiverAcceptQuest(ctx, acceptZeroPayload.Bytes()) {
		t.Fatal("handleQuestgiverAcceptQuest failed for guid 0")
	}
	if state.player.QuestLog[1].QuestID != 302 {
		t.Fatalf("expected quest 302 in player quest log slot 1, got %d", state.player.QuestLog[1].QuestID)
	}
}

