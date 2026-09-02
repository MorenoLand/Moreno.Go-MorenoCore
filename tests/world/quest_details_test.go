//go:build ignore

package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildQuestGiverDetails(t *testing.T) {
	data := questDetailData{ID: 100, Title: "A Test Quest", Details: "Details", Objectives: "Objectives", Flags: 7, SuggestedGroupNum: 3, RewardMoney: 4, RewardXPDifficulty: 5, RewardBonusMoney: 6, RewardDisplaySpell: 7, RewardSpell: -8, RewardHonor: 9, RewardKillHonor: 1.5, RewardTitleID: 10, RewardTalents: 11, RewardArenaPoints: -12, ChoiceItems: []questRewardItem{{ID: 20, Quantity: 2, DisplayID: 200}}, RewardItems: []questRewardItem{{ID: 21, Quantity: 3, DisplayID: 201}}, DescEmotes: [questDetailEmotes]questDescEmote{{Type: 30, Delay: 31}}}
	reader := protocol.NewReader(buildQuestGiverDetails(data, 0x1122, 0))
	for _, expected := range []uint64{0x1122, 0} {
		if value, err := reader.ReadU64(); err != nil || value != expected {
			t.Fatalf("guid=%x expected=%x err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != data.ID {
		t.Fatalf("quest=%d err=%v", value, err)
	}
	for _, expected := range []string{data.Title, data.Details, data.Objectives} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("auto launched=%d err=%v", value, err)
	}
	for _, expected := range []uint32{data.Flags, data.SuggestedGroupNum} {
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("start cheat=%d err=%v", value, err)
	}
	for _, expected := range []uint32{1, 20, 2, 200, 1, 21, 3, 201, data.RewardMoney, data.RewardXPDifficulty, data.RewardHonor} {
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("reward value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadF32(); err != nil || value != data.RewardKillHonor {
		t.Fatalf("honor=%f err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != data.RewardDisplaySpell {
		t.Fatalf("display spell=%d err=%v", value, err)
	}
	if value, err := reader.ReadI32(); err != nil || value != data.RewardSpell {
		t.Fatalf("reward spell=%d err=%v", value, err)
	}
	for _, expected := range []uint32{data.RewardTitleID, data.RewardTalents} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("reward value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadI32(); err != nil || value != data.RewardArenaPoints {
		t.Fatalf("arena=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("faction flags=%d err=%v", value, err)
	}
	for index := 0; index < questRewardFactions*3; index++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if value, err := reader.ReadI32(); err != nil || value != questDetailEmotes {
		t.Fatalf("emote count=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 30 {
		t.Fatalf("emote type=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 31 {
		t.Fatalf("emote delay=%d err=%v", value, err)
	}
}

func TestQuestLogRemoveQuest(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE character_queststatus_daily (guid INTEGER, quest INTEGER, time INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO character_queststatus VALUES (1, 123, 3)"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	sess.player.QuestLog[2] = questLogEntry{QuestID: 123}

	if !sess.handleQuestLogRemoveQuest(context.Background(), []byte{2}) {
		t.Fatal("handleQuestLogRemoveQuest returned false")
	}
	if sess.player.QuestLog[2].QuestID != 0 {
		t.Fatalf("expected quest log slot 2 to be empty, got quest %d", sess.player.QuestLog[2].QuestID)
	}
	var count int
	_ = db.QueryRow("SELECT COUNT(1) FROM character_queststatus WHERE guid = 1 AND quest = 123").Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 rows in character_queststatus, got %d", count)
	}
}

func TestQuestAutoAcceptOnlySpecialFlags4(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, QuestLevel INTEGER NOT NULL, MinLevel INTEGER NOT NULL, Flags INTEGER NOT NULL, LogTitle TEXT, Details TEXT, Objectives TEXT, SuggestedGroupNum INTEGER DEFAULT 0, RewardMoney INTEGER DEFAULT 0, RewardXPDifficulty INTEGER DEFAULT 0, RewardBonusMoney INTEGER DEFAULT 0, RewardDisplaySpell INTEGER DEFAULT 0, RewardSpell INTEGER DEFAULT 0, RewardHonor INTEGER DEFAULT 0, RewardKillHonor REAL DEFAULT 0, RewardTitleID INTEGER DEFAULT 0, RewardTalents INTEGER DEFAULT 0, RewardArenaPoints INTEGER DEFAULT 0, AllowableRaces INTEGER DEFAULT 0)",
		"CREATE TABLE quest_template_addon (ID INTEGER PRIMARY KEY, SpecialFlags INTEGER DEFAULT 0, PrevQuestID INTEGER DEFAULT 0, MaxLevel INTEGER DEFAULT 0, AllowableClasses INTEGER DEFAULT 0, ExclusiveGroup INTEGER DEFAULT 0, NextQuestID INTEGER DEFAULT 0, BreadcrumbForQuestId INTEGER DEFAULT 0)",
		"CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER)",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER, quest INTEGER)",
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER)",
		"CREATE TABLE creature_queststarter (id INTEGER, quest INTEGER)",
		// Quest 201 has old fabricated flag 0x00080000, SpecialFlags = 0: must NOT auto-accept
		"INSERT INTO quest_template VALUES (201, 5, 1, 524288, 'Ordinary Quest', 'Details', 'Objectives', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)",
		"INSERT INTO quest_template_addon (ID, SpecialFlags) VALUES (201, 0)",
		// Quest 202 has SpecialFlags = 4: MUST auto-accept
		"INSERT INTO quest_template VALUES (202, 5, 1, 0, 'Auto Quest', 'Details', 'Objectives', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)",
		"INSERT INTO quest_template_addon (ID, SpecialFlags) VALUES (202, 4)",
		"INSERT INTO creature VALUES (50, 1000)",
		"INSERT INTO creature_queststarter VALUES (1000, 201)",
		"INSERT INTO creature_queststarter VALUES (1000, 202)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: store, CharactersStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Level: 10}}
	ctx := context.Background()

	// 1. Query quest 201 (ordinary): should NOT be added to player quest log
	buf201 := protocol.NewBuffer(16)
	buf201.WriteU64(50)
	buf201.WriteU32(201)
	sess.handleQuestgiverQueryQuest(ctx, buf201.Bytes())
	if sess.player.QuestLog[0].QuestID == 201 {
		t.Fatal("ordinary quest with flag 0x00080000 should NOT auto-accept")
	}

	// 2. Query quest 202 (SpecialFlags=4): MUST auto-accept into player quest log
	buf202 := protocol.NewBuffer(16)
	buf202.WriteU64(50)
	buf202.WriteU32(202)
	sess.handleQuestgiverQueryQuest(ctx, buf202.Bytes())
	if sess.player.QuestLog[0].QuestID != 202 {
		t.Fatalf("auto-accept quest (SpecialFlags=4) was not accepted into quest log, slot 0 = %d", sess.player.QuestLog[0].QuestID)
	}
}

func TestTalkToQuestAutoCompletes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, QuestLevel INTEGER NOT NULL, MinLevel INTEGER NOT NULL, Flags INTEGER NOT NULL, LogTitle TEXT, AllowableRaces INTEGER DEFAULT 0)",
		"CREATE TABLE quest_template_addon (ID INTEGER PRIMARY KEY, SpecialFlags INTEGER DEFAULT 0, PrevQuestID INTEGER DEFAULT 0, MaxLevel INTEGER DEFAULT 0, AllowableClasses INTEGER DEFAULT 0, ExclusiveGroup INTEGER DEFAULT 0, NextQuestID INTEGER DEFAULT 0, BreadcrumbForQuestId INTEGER DEFAULT 0)",
		"CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER)",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER, quest INTEGER)",
		"CREATE TABLE creature_queststarter (id INTEGER, quest INTEGER)",
		"CREATE TABLE creature_questender (id INTEGER, quest INTEGER)",
		// Quest 350: Talk-to/delivery quest with no kill or item requirements
		"INSERT INTO quest_template VALUES (350, 1, 1, 0, 'Walk to NPC', 0)",
		"INSERT INTO creature_queststarter VALUES (10, 350)",
		"INSERT INTO creature_questender VALUES (20, 350)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: store, CharactersStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Level: 1}}
	ctx := context.Background()

	// Accept quest 350
	acceptBuf := protocol.NewBuffer(16)
	acceptBuf.WriteU64(0)
	acceptBuf.WriteU32(350)
	sess.handleQuestgiverAcceptQuest(ctx, acceptBuf.Bytes())

	// Because quest 350 has 0 requirements, it must immediately transition to complete
	if sess.player.QuestLog[0].QuestID != 350 {
		t.Fatalf("expected quest 350 in quest log, got %d", sess.player.QuestLog[0].QuestID)
	}
	if sess.player.QuestLog[0].State != 1 {
		t.Fatalf("expected quest 350 state to be completed (1), got %d", sess.player.QuestLog[0].State)
	}

	// Ender NPC (20) dialog status must return questDialogReward (10)
	status, err := sess.questDialogStatus(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if status != questDialogReward {
		t.Fatalf("expected questDialogReward (10) for talk-to quest ender, got %d", status)
	}
}


