package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
)

func TestApplyMigrationsIsOrderedAndIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	migrations := []Migration{{Version: 1, Name: "create", Statements: []string{"CREATE TABLE sample (id INTEGER PRIMARY KEY)"}}, {Version: 2, Name: "insert", Statements: []string{"INSERT INTO sample (id) VALUES (1)"}}}
	if err := ApplyMigrations(context.Background(), store, migrations); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(context.Background(), store, migrations); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sample").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rows=%d", count)
	}
}

func TestApplyMigrationsWithOptionsReappliesChangedHash(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	initial := []Migration{{Version: 1, Name: "create", Statements: []string{"CREATE TABLE sample (id INTEGER PRIMARY KEY)"}}, {Version: 2, Name: "insert", Statements: []string{"INSERT INTO sample (id) VALUES (1)"}}}
	if _, err := ApplyMigrationsWithOptions(context.Background(), store, initial, MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	changed := []Migration{{Version: 1, Name: "create", Statements: initial[0].Statements}, {Version: 2, Name: "insert", Statements: []string{"INSERT INTO sample (id) VALUES (2)"}}}
	result, err := ApplyMigrationsWithOptions(context.Background(), store, changed, MigrationOptions{RedundancyChecks: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 1 || result.Rehashed != 0 {
		t.Fatalf("result=%+v", result)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sample").Scan(&count); err != nil || count != 2 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
}

func TestApplyMigrationsRollbackAndOrphanCleanup(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	base := []Migration{{Version: 1, Name: "create", Statements: []string{"CREATE TABLE sample (id INTEGER PRIMARY KEY)"}}}
	if _, err := ApplyMigrationsWithOptions(context.Background(), store, base, MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	broken := []Migration{{Version: 2, Name: "broken", Statements: []string{"INSERT INTO sample (id) VALUES (2)", "INSERT INTO missing_table VALUES (1)"}}}
	if _, err := ApplyMigrationsWithOptions(context.Background(), store, broken, MigrationOptions{}); err == nil {
		t.Fatal("expected failed migration")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sample").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled-back rows=%d err=%v", count, err)
	}
	if _, err := db.Exec("INSERT INTO trinitygo_migrations (version, name, hash, state) VALUES (99, 'orphan', 'x', 'RELEASED')"); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyMigrationsWithOptions(context.Background(), store, base, MigrationOptions{CleanOrphans: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Orphans != 1 {
		t.Fatalf("result=%+v", result)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM trinitygo_migrations WHERE version = 99").Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan count=%d err=%v", count, err)
	}
}

func TestLoadMigrationsWalksArchivedDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "archived"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_first.sql"), []byte("CREATE TABLE first (id INTEGER)"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "archived", "002_old.sql"), []byte("CREATE TABLE old (id INTEGER)"), 0644); err != nil {
		t.Fatal(err)
	}
	migrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 || migrations[0].State != MigrationStateReleased || migrations[1].State != MigrationStateArchived || migrations[0].Hash == "" || migrations[1].Hash == "" {
		t.Fatalf("migrations=%+v", migrations)
	}
}

func TestConfiguredMigrationsApplyFromSchemaUpdatesDirectory(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := t.TempDir()
	updates := filepath.Join(root, "sqlite", "updates", "auth")
	if err := os.MkdirAll(updates, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(updates, "001_create.sql"), []byte("CREATE TABLE applied (id INTEGER PRIMARY KEY)"), 0644); err != nil {
		t.Fatal(err)
	}
	c := config.Default()
	c.SchemaDir = root
	c.UpdatesEnableDatabases = 1
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	if err := applyConfiguredMigrations(context.Background(), c, store); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'applied'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("applied table count=%d err=%v", count, err)
	}
}
