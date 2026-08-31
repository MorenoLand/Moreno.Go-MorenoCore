package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Migration struct {
	Version    int
	Name       string
	Statements []string
}

func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("migration %s must begin with a numeric version", entry.Name())
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, Migration{Version: version, Name: parts[1], Statements: SplitSQL(string(data))})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	for i := 1; i < len(result); i++ {
		if result[i-1].Version == result[i].Version {
			return nil, fmt.Errorf("duplicate migration version %d", result[i].Version)
		}
	}
	return result, nil
}

func ApplyMigrations(ctx context.Context, store *Store, migrations []Migration) error {
	meta := "CREATE TABLE IF NOT EXISTS trinitygo_migrations (version INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)"
	if store.Backend != BackendSQLite {
		meta = "CREATE TABLE IF NOT EXISTS trinitygo_migrations (version INT NOT NULL PRIMARY KEY, name VARCHAR(255) NOT NULL, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)"
	}
	if _, err := store.DB.ExecContext(ctx, meta); err != nil {
		return err
	}
	for _, migration := range migrations {
		var exists int
		err := store.DB.QueryRowContext(ctx, "SELECT 1 FROM trinitygo_migrations WHERE version = ?", migration.Version).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		tx, err := store.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, statement := range migration.Statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %d %s: %w", migration.Version, migration.Name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO trinitygo_migrations (version, name) VALUES (?, ?)", migration.Version, migration.Name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
