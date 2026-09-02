//go:build ignore

package database

import (
	"strings"
	"testing"
)

func TestGeneratedStatementRegistry(t *testing.T) {
	registry, err := NewStatementRegistry(AllStatements())
	if err != nil {
		t.Fatal(err)
	}
	if registry.Len() != 612 {
		t.Fatalf("statement count: %d", registry.Len())
	}
	for _, definition := range AllStatements() {
		if got, ok := registry.Get(definition.ID); !ok || got.SQL != definition.SQL || got.Async != definition.Async {
			t.Fatalf("missing or changed %s", definition.ID)
		}
	}
	if query, err := StatementSQL("LOGIN_SEL_LOGONCHALLENGE", BackendSQLite); err != nil || query == "" || strings.Contains(query, "UNIX_TIMESTAMP") {
		t.Fatalf("SQLite statement: %q %v", query, err)
	}
}

