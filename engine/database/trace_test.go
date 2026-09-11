package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocoltrace"
	_ "modernc.org/sqlite"
)

func TestStoreRecordsRedactedDatabaseEvents(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recorder := protocoltrace.NewRecorder("database-test")
	store := &Store{Name: "characters", Backend: BackendSQLite, DB: db, TraceRecorder: recorder}
	if _, err := store.Exec(context.Background(), "CREATE TABLE sample (id INTEGER)", "secret"); err != nil {
		t.Fatal(err)
	}
	events := recorder.Snapshot().Events
	if len(events) != 1 || events[0].State != "database" {
		t.Fatalf("events=%+v", events)
	}
	payload, err := recorder.Snapshot().Payload(events[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) == "" || containsString(string(payload), "secret") {
		t.Fatalf("database trace leaked argument: %s", payload)
	}
}

func containsString(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
