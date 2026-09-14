package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

type statementExerciseResult struct {
	name      string
	total     int
	executed  int
	queried   int
	maxFields int
	failures  []statementAuditFailure
}

func statementExerciseArgs(query string) []any {
	args := make([]any, statementBindCount(query))
	for index := range args {
		args[index] = int64(0)
	}
	return args
}

func statementExerciseArgsFor(ctx context.Context, db *sql.DB, id database.StatementID, query string) []any {
	args := statementExerciseArgs(query)
	switch id {
	case "LOGIN_INS_ALDL_IP_LOGGING", "LOGIN_INS_FACL_IP_LOGGING":
		var accountID int64
		if len(args) >= 5 && db.QueryRowContext(ctx, "SELECT id FROM account ORDER BY id LIMIT 1").Scan(&accountID) == nil {
			args[0], args[4] = accountID, accountID
		}
	case "WORLD_INS_WAYPOINT_SCRIPT":
		if len(args) == 1 {
			var nextGUID int64
			if db.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM waypoint_scripts").Scan(&nextGUID) == nil {
				args[0] = nextGUID
			}
		}
	}
	return args
}

func exerciseStatement(ctx context.Context, db *sql.DB, definition database.StatementDefinition) (int, bool, error) {
	query, err := database.StatementSQL(definition.ID, database.BackendSQLite)
	if err != nil {
		return 0, false, err
	}
	args := statementExerciseArgsFor(ctx, db, definition.ID, query)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	if strings.HasPrefix(trimmed, "SELECT") {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return 0, true, err
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return 0, true, err
		}
		for rows.Next() {
			values := make([]any, len(columns))
			destinations := make([]any, len(columns))
			for index := range values {
				destinations[index] = &values[index]
			}
			if err := rows.Scan(destinations...); err != nil {
				rows.Close()
				return len(columns), true, err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return len(columns), true, err
		}
		if err := rows.Close(); err != nil {
			return len(columns), true, err
		}
		return len(columns), true, nil
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return 0, false, err
	}
	return 0, false, nil
}

func exerciseStatements(ctx context.Context, inputDir string) ([]statementExerciseResult, error) {
	definitions := map[string][]database.StatementDefinition{"auth": {}, "characters": {}, "world": {}}
	for _, definition := range database.AllStatements() {
		name, err := statementDatabase(definition.ID)
		if err != nil {
			return nil, err
		}
		definitions[name] = append(definitions[name], definition)
	}
	results := make([]statementExerciseResult, 0, 3)
	for _, name := range []string{"auth", "characters", "world"} {
		sort.Slice(definitions[name], func(i, j int) bool { return definitions[name][i].ID < definitions[name][j].ID })
		path := filepath.Join(inputDir, name+".db")
		db, err := sql.Open("sqlite", path)
		if err != nil {
			return nil, fmt.Errorf("%s database %s: %w", name, path, err)
		}
		db.SetMaxOpenConns(1)
		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s database %s: %w", name, path, err)
		}
		result := statementExerciseResult{name: name, total: len(definitions[name]), failures: make([]statementAuditFailure, 0)}
		for _, definition := range definitions[name] {
			fields, queried, err := exerciseStatement(ctx, db, definition)
			if err != nil {
				result.failures = append(result.failures, statementAuditFailure{id: definition.ID, err: err})
				continue
			}
			result.executed++
			if queried {
				result.queried++
				if fields > result.maxFields {
					result.maxFields = fields
				}
			}
		}
		if err := db.Close(); err != nil {
			return nil, fmt.Errorf("%s database close: %w", name, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func statementExercise(args []string) int {
	fs := flag.NewFlagSet("statement-exercise", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("input-dir", ".", "SQLite database directory")
	strict := fs.Bool("strict", false, "return failure when any statement cannot execute")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	results, err := exerciseStatements(context.Background(), *dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	total, executed, queried, failures := 0, 0, 0, 0
	for _, result := range results {
		fmt.Printf("%s statements=%d executed=%d queried=%d max_fields=%d failures=%d\n", result.name, result.total, result.executed, result.queried, result.maxFields, len(result.failures))
		total += result.total
		executed += result.executed
		queried += result.queried
		failures += len(result.failures)
		for _, failure := range result.failures {
			fmt.Printf("  %s: %v\n", failure.id, failure.err)
		}
	}
	fmt.Printf("total statements=%d executed=%d queried=%d failures=%d backend=sqlite transaction=rollback-per-statement\n", total, executed, queried, failures)
	if *strict && failures != 0 {
		return 1
	}
	return 0
}
