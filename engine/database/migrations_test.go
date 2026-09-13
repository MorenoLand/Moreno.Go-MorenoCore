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
	if _, err := db.Exec("INSERT INTO updates (name, hash, state) VALUES ('orphan.sql', 'x', 'RELEASED')"); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyMigrationsWithOptions(context.Background(), store, base, MigrationOptions{CleanOrphans: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Orphans != 1 {
		t.Fatalf("result=%+v", result)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM updates WHERE name = 'orphan.sql'").Scan(&count); err != nil || count != 0 {
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

func TestEnsureSchemaHonorsUpdatesAutoSetup(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sqlite"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sqlite", "auth.sql"), []byte("CREATE TABLE should_not_exist (id INTEGER)"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := config.Default()
	c.SchemaDir = root
	c.UpdatesAutoSetup = false
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	if err := ensureSchema(context.Background(), c, store); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'should_not_exist'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("auto-setup-disabled table count=%d err=%v", count, err)
	}
}

func TestConfiguredMigrationsLoadsUpdatesInclude(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := t.TempDir()
	include := filepath.Join(root, "included")
	if err := os.MkdirAll(include, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(include, "002_included.sql"), []byte("CREATE TABLE included (id INTEGER PRIMARY KEY)"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE updates_include (path TEXT PRIMARY KEY, state TEXT NOT NULL); INSERT INTO updates_include VALUES (?, 'ARCHIVED')", include); err != nil {
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
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'included'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("included table count=%d err=%v", count, err)
	}
	var state string
	if err := db.QueryRow("SELECT state FROM updates WHERE name = '002_included.sql'").Scan(&state); err != nil || state != MigrationStateArchived {
		t.Fatalf("included update state=%q err=%v", state, err)
	}
}
