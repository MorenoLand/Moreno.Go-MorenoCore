package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestKickDuplicateAccountSession(t *testing.T) {
	oldServer, oldClient := net.Pipe()
	defer oldClient.Close()
	defer oldServer.Close()
	server := &Server{sessions: make(map[*session]struct{})}
	old := &session{server: server, conn: oldServer, authed: true, accountID: 42, accountName: "old"}
	current := &session{server: server, accountID: 42, accountName: "new"}
	server.sessions[old] = struct{}{}
	server.sessions[current] = struct{}{}
	server.kickDuplicateAccountSessions(42, current)
	buffer := make([]byte, 1)
	if _, err := oldClient.Read(buffer); err == nil {
		t.Fatal("expected duplicate session connection to close")
	}
	if !old.superseded {
		t.Fatal("expected duplicate session to be marked superseded")
	}
	if _, exists := server.sessions[old]; exists {
		t.Fatal("expected duplicate session to be removed from active session map")
	}
}

func TestWardenOSAllowedMatchesReferenceGate(t *testing.T) {
	for _, test := range []struct {
		os      string
		allowed bool
	}{
		{os: "Win", allowed: true},
		{os: "OSX", allowed: true},
		{os: "Linux", allowed: false},
		{os: "", allowed: false},
	} {
		if got := wardenOSAllowed(test.os); got != test.allowed {
			t.Fatalf("os=%q allowed=%v want=%v", test.os, got, test.allowed)
		}
	}
}

func TestNormalizeLoginMuteTimePersistsNegativeDuration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE account (id INTEGER PRIMARY KEY, mutetime INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO account (id, mutetime) VALUES (7, -120)"); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Unix()
	got := normalizeLoginMuteTime(context.Background(), db, 7, -120)
	if got < before+119 {
		t.Fatalf("normalized mute=%d before=%d", got, before)
	}
	var stored int64
	if err := db.QueryRow("SELECT mutetime FROM account WHERE id = 7").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != got {
		t.Fatalf("stored mute=%d returned=%d", stored, got)
	}
}
