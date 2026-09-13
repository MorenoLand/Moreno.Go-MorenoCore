package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	_ "modernc.org/sqlite"
)

func TestClearOnlineStateMatchesWorldStartup(t *testing.T) {
	authDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()
	charactersDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer charactersDB.Close()
	for _, statement := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, online INTEGER NOT NULL)",
		"CREATE TABLE realmcharacters (acctid INTEGER, realmid INTEGER)",
		"INSERT INTO account VALUES (7, 1), (8, 1), (9, 0)",
		"INSERT INTO realmcharacters VALUES (7, 1), (8, 2)",
	} {
		if _, err := authDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, online INTEGER NOT NULL)",
		"CREATE TABLE character_battleground_data (guid INTEGER PRIMARY KEY, instanceId INTEGER NOT NULL)",
		"INSERT INTO characters VALUES (100, 1), (101, 0)",
		"INSERT INTO character_battleground_data VALUES (100, 55)",
	} {
		if _, err := charactersDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{AuthStore: &database.Store{DB: authDB}, CharactersStore: &database.Store{DB: charactersDB}, RealmID: 1}
	server.clearOnlineState(context.Background())
	var accountOnline, characterOnline, instanceID int
	if err := authDB.QueryRow("SELECT online FROM account WHERE id = 7").Scan(&accountOnline); err != nil {
		t.Fatal(err)
	}
	if err := charactersDB.QueryRow("SELECT online FROM characters WHERE guid = 100").Scan(&characterOnline); err != nil {
		t.Fatal(err)
	}
	if err := charactersDB.QueryRow("SELECT instanceId FROM character_battleground_data WHERE guid = 100").Scan(&instanceID); err != nil {
		t.Fatal(err)
	}
	if accountOnline != 0 || characterOnline != 0 || instanceID != 0 {
		t.Fatalf("online=%d character=%d instance=%d", accountOnline, characterOnline, instanceID)
	}
	var otherOnline int
	if err := authDB.QueryRow("SELECT online FROM account WHERE id = 8").Scan(&otherOnline); err != nil {
		t.Fatal(err)
	}
	if otherOnline != 1 {
		t.Fatalf("realm 2 account was cleared: %d", otherOnline)
	}
}
