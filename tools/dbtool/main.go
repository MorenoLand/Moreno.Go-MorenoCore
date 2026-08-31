package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.TrinityGo/engine/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dbtool schema|import-sql")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "schema":
		os.Exit(schema(os.Args[2:]))
	case "import-sql":
		os.Exit(importSQL(os.Args[2:]))
	case "verify":
		os.Exit(verify(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown dbtool command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func schema(args []string) int {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	auth := fs.String("auth", "", "auth SQL dump")
	characters := fs.String("characters", "", "characters SQL dump")
	world := fs.String("world", "", "world SQL dump")
	out := fs.String("output", "sql", "output schema directory")
	backend := fs.String("backend", "both", "mysql, sqlite, or both")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	inputs := map[string]string{"auth": *auth, "characters": *characters, "world": *world}
	for name, path := range inputs {
		if path == "" {
			fmt.Fprintf(os.Stderr, "--%s is required\n", name)
			return 2
		}
	}
	for dialect, enabled := range map[string]bool{"mysql": *backend == "mysql" || *backend == "both", "sqlite": *backend == "sqlite" || *backend == "both"} {
		if !enabled {
			continue
		}
		for name, path := range inputs {
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			converted, err := database.NormalizeSchemaScript(string(data), dialect)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s %s: %v\n", dialect, name, err)
				return 1
			}
			path := filepath.Join(*out, dialect, name+".sql")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if err := os.WriteFile(path, []byte(converted), 0644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Printf("wrote %s\n", path)
		}
	}
	return 0
}

func importSQL(args []string) int {
	fs := flag.NewFlagSet("import-sql", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("output-dir", ".", "SQLite output directory")
	force := fs.Bool("force", false, "replace existing output databases")
	auth := fs.String("auth", "", "auth SQL dump")
	characters := fs.String("characters", "", "characters SQL dump")
	world := fs.String("world", "", "world SQL dump")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	inputs := map[string]string{"auth": *auth, "characters": *characters, "world": *world}
	for name, path := range inputs {
		if path == "" {
			fmt.Fprintf(os.Stderr, "--%s is required\n", name)
			return 2
		}
		output := filepath.Join(*out, name+".db")
		if err := database.ImportSQLiteDump(context.Background(), path, output, *force); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			return 1
		}
		fmt.Printf("wrote %s\n", output)
	}
	return 0
}

func verify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("input-dir", ".", "SQLite database directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	expected := map[string]int{"auth": 19, "characters": 94, "world": 190}
	for name, tables := range expected {
		path := filepath.Join(*dir, name+".db")
		db, err := sql.Open("sqlite", path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			return 1
		}
		actualTables, views, rows, err := databaseStats(db)
		_ = db.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			return 1
		}
		if actualTables != tables {
			fmt.Fprintf(os.Stderr, "%s: found %d tables, expected %d\n", name, actualTables, tables)
			return 1
		}
		fmt.Printf("%s tables=%d views=%d rows=%d\n", name, actualTables, views, rows)
	}
	return 0
}

func databaseStats(db *sql.DB) (int, int, int64, error) {
	rows, err := db.Query("SELECT type, name FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY type, name")
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	tables, views := 0, 0
	var total int64
	for rows.Next() {
		var kind, name string
		if err := rows.Scan(&kind, &name); err != nil {
			return 0, 0, 0, err
		}
		if kind == "view" {
			views++
			continue
		}
		if name == "trinitygo_schema" {
			continue
		}
		tables++
		query := `SELECT COUNT(*) FROM "` + strings.ReplaceAll(name, `"`, `""`) + `"`
		var count int64
		if err := db.QueryRow(query).Scan(&count); err != nil {
			return 0, 0, 0, err
		}
		total += count
	}
	return tables, views, total, rows.Err()
}
