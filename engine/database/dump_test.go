package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestRoutineCreateDoesNotMatchTableComments(t *testing.T) {
	if routineCreate("CREATE TABLE `game_event` (`description` TEXT COMMENT 'Game Event System')") {
		t.Fatal("table comment was treated as a routine")
	}
	for _, statement := range []string{
		"CREATE PROCEDURE refresh_world() BEGIN SELECT 1; END",
		"CREATE DEFINER=`root`@`localhost` FUNCTION current_value() RETURNS INT RETURN 1",
		"CREATE TRIGGER account_update AFTER UPDATE ON account FOR EACH ROW SET NEW.online = NEW.online",
		"CREATE EVENT expire_bans ON SCHEDULE EVERY 1 DAY DO DELETE FROM account_banned",
	} {
		if !routineCreate(statement) {
			t.Fatalf("routine was not detected: %s", statement)
		}
	}
}

func TestImportSQLiteDumpForceReplacesOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.sql")
	output := filepath.Join(dir, "auth.db")
	if err := os.WriteFile(input, []byte("CREATE TABLE account (id INTEGER PRIMARY KEY); INSERT INTO account VALUES (2);"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE account (id INTEGER PRIMARY KEY); INSERT INTO account VALUES (1)"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ImportSQLiteDump(context.Background(), input, output, true); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", output)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id int
	if err := db.QueryRow("SELECT id FROM account").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Fatalf("id=%d", id)
	}
}
