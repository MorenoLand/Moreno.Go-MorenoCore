package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
)

// TestSQLitePathNotDoubleJoined guards the bin/bin/auth.db regression:
// Config.ResolvePaths folds DataDir into the database file names, so the
// connection helper must not prepend DataDir a second time and silently
// create a fresh empty database next to the populated one.
func TestSQLitePathNotDoubleJoined(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	populated := filepath.Join(workDir, "auth.db")
	if err := os.WriteFile(populated, []byte("SQLite format 3\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	c := config.Default()
	c.Backend = "sqlite"
	if err := c.Set("DataDir", "bin"); err != nil {
		t.Fatal(err)
	}
	c.ApplyWorkDir("bin")
	c.ResolvePaths()

	driver, dsn, backend, err := connection(c.Backend, c.AuthDatabaseFile, "", c)
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || backend != BackendSQLite {
		t.Fatalf("driver=%s backend=%v", driver, backend)
	}
	got := filepath.Clean(dsn)
	want := filepath.Clean(filepath.Join("bin", "auth.db"))
	if got != want {
		t.Fatalf("dsn=%s want=%s", got, want)
	}
	if _, err := os.Stat(filepath.Join(workDir, "bin")); !os.IsNotExist(err) {
		t.Fatalf("nested bin directory was created: %s", filepath.Join(workDir, "bin"))
	}
}

// TestSQLitePathFallsBackToDataDir verifies the bare relative filename case
// used when ResolvePaths is not applied: the file joins DataDir.
func TestSQLitePathFallsBackToDataDir(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	c := config.Default()
	c.Backend = "sqlite"
	if err := c.Set("DataDir", "bin"); err != nil {
		t.Fatal(err)
	}
	c.ApplyWorkDir("bin")

	driver, dsn, backend, err := connection(c.Backend, c.AuthDatabaseFile, "", c)
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || backend != BackendSQLite {
		t.Fatalf("driver=%s backend=%v", driver, backend)
	}
	got := filepath.Clean(dsn)
	want := filepath.Clean(filepath.Join("bin", "auth.db"))
	if got != want {
		t.Fatalf("dsn=%s want=%s", got, want)
	}
}
