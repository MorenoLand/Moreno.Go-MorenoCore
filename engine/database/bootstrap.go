package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
)

var (
	createTablePattern = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?`)
	tableNamePattern   = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([^\s(]+)`)
)

func EnsureSchemas(ctx context.Context, c config.Config, set *Set) error {
	for _, store := range []*Store{set.Auth, set.Characters, set.World} {
		if err := ensureSchema(ctx, c, store); err != nil {
			return err
		}
	}
	return nil
}

func ensureSchema(ctx context.Context, c config.Config, store *Store) error {
	meta := "CREATE TABLE IF NOT EXISTS trinitygo_schema (id INTEGER NOT NULL PRIMARY KEY, version INTEGER NOT NULL)"
	if store.Backend != BackendSQLite {
		meta = "CREATE TABLE IF NOT EXISTS trinitygo_schema (id INT NOT NULL PRIMARY KEY, version INT NOT NULL)"
	}
	if _, err := store.Exec(ctx, meta); err != nil {
		return fmt.Errorf("%s schema metadata: %w", store.Name, err)
	}
	var version int
	err := store.DB.QueryRowContext(ctx, "SELECT version FROM trinitygo_schema WHERE id = 1").Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := store.Exec(ctx, "INSERT INTO trinitygo_schema (id, version) VALUES (1, 0)"); err != nil {
			return fmt.Errorf("%s schema metadata insert: %w", store.Name, err)
		}
		version = 0
	} else if err != nil {
		return fmt.Errorf("%s schema metadata read: %w", store.Name, err)
	}
	if version > 0 {
		return nil
	}
	dialect := "mysql"
	if store.Backend == BackendSQLite {
		dialect = "sqlite"
	}
	path := filepath.Join(c.SchemaDir, dialect, store.Name+".sql")
	script, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s schema file %s: %w", store.Name, path, err)
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s schema transaction: %w", store.Name, err)
	}
	for _, statement := range SplitSQL(string(script)) {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s schema statement: %w", store.Name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE trinitygo_schema SET version = 1 WHERE id = 1"); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s schema metadata update: %w", store.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s schema commit: %w", store.Name, err)
	}
	return nil
}

func normalizeMySQLCreateTable(statement string) string {
	statement = strings.TrimSpace(statement)
	if !createTablePattern.MatchString(statement) {
		return statement
	}
	return createTablePattern.ReplaceAllString(statement, "CREATE TABLE IF NOT EXISTS ")
}
