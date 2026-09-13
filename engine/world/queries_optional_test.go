package world

import (
	"context"
	"database/sql"
	"testing"
)

func TestLoadQuestItemsTreatsOptionalSchemaColumnsAsEmpty(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE creature_questitem (CreatureEntry INTEGER, ItemId INTEGER)"); err != nil {
		t.Fatal(err)
	}
	items, err := loadQuestItems(context.Background(), db, "creature_questitem", "CreatureEntry", "Idx", "ItemId", 123, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 6 {
		t.Fatalf("item slots=%d", len(items))
	}
	for i, item := range items {
		if item != 0 {
			t.Fatalf("item[%d]=%d", i, item)
		}
	}
}
