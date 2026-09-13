package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestLoadOptionalPlayerStateRestAndDrunkenness(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE characters (
		guid INTEGER PRIMARY KEY, account INTEGER, xp INTEGER, money INTEGER, health INTEGER,
		power1 INTEGER, power2 INTEGER, power3 INTEGER, power4 INTEGER, power5 INTEGER, power6 INTEGER, power7 INTEGER,
		cinematic INTEGER, knownCurrencies INTEGER, watchedFaction INTEGER, ammoId INTEGER, actionBars INTEGER,
		restState INTEGER, drunk INTEGER, bankSlots INTEGER, talentGroupsCount INTEGER, activeTalentGroup INTEGER,
		chosenTitle INTEGER, knownTitles TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO characters
		(guid, account, xp, money, health, power1, power2, power3, power4, power5, power6, power7,
		 cinematic, knownCurrencies, watchedFaction, ammoId, actionBars, restState, drunk, bankSlots,
		 talentGroupsCount, activeTalentGroup, chosenTitle, knownTitles)
		VALUES (1, 7, 10, 20, 30, 1, 2, 3, 4, 5, 6, 7, 0, 9, 10, 11, 12, 2, 77, 4, 2, 1, 123, '')`); err != nil {
		t.Fatal(err)
	}
	sess := &session{server: &Server{CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}}, accountID: 7}
	state := playerState{GUID: 1}
	if err := sess.loadOptionalPlayerState(context.Background(), &state); err != nil {
		t.Fatal(err)
	}
	if state.RestState != 2 || state.DrunkenState != 77 {
		t.Fatalf("rest=%d drunk=%d", state.RestState, state.DrunkenState)
	}
}

func TestLoadPlayerSkillsNormalizesLanguageAndLevelRanges(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE character_skills (guid INTEGER, skill INTEGER, value INTEGER, max INTEGER, PRIMARY KEY (guid, skill))"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO character_skills VALUES (1, 98, 1, 1), (1, 43, 1, 1)"); err != nil {
		t.Fatal(err)
	}
	sess := &session{server: &Server{CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}}}
	state := playerState{GUID: 1, Race: 1, Class: 1, Level: 20}
	if err := sess.loadPlayerSkills(context.Background(), &state); err != nil {
		t.Fatal(err)
	}
	values := make(map[uint16]playerSkill)
	for _, skill := range state.Skills {
		values[skill.Skill] = skill
	}
	if values[98].Value != 300 || values[98].Max != 300 || values[43].Value != 1 || values[43].Max != 100 {
		t.Fatalf("skills=%+v", values)
	}
	var languageMax, levelMax int
	if err := db.QueryRow("SELECT max FROM character_skills WHERE guid = 1 AND skill = 98").Scan(&languageMax); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT max FROM character_skills WHERE guid = 1 AND skill = 43").Scan(&levelMax); err != nil {
		t.Fatal(err)
	}
	if languageMax != 300 || levelMax != 100 {
		t.Fatalf("persisted max values=%d/%d", languageMax, levelMax)
	}
}
