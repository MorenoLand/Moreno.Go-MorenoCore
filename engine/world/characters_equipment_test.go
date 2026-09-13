package world

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestLoadEnumEquipmentFallsBackToInventory(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER)",
		"INSERT INTO item_instance VALUES (100, 9001), (101, 9002)",
		"INSERT INTO character_inventory VALUES (1, 0, 0, 100), (1, 0, 15, 101)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	sess := &session{server: &Server{CharactersStore: store}}
	character := enumCharacter{GUID: 1, Equipment: ""}
	sess.loadEnumEquipment(context.Background(), &character)
	fields := strings.Fields(character.Equipment)
	if len(fields) != int(equipSlotEnd)*2 || fields[0] != "9001" || fields[30] != "9002" {
		t.Fatalf("equipment=%q", character.Equipment)
	}
}
