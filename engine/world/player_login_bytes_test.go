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
