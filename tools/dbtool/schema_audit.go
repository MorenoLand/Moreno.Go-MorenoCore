package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

type schemaInventory struct {
	Objects map[string]string
}

var (
	schemaObjectPattern = regexp.MustCompile(`(?is)^CREATE\s+(TABLE|VIEW|(?:UNIQUE\s+)?INDEX)\s+(?:IF\s+NOT\s+EXISTS\s+)?[\x60\"]?([^\x60\"\s(]+)`)
	schemaIndexPattern  = regexp.MustCompile(`(?is)^CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?[\x60\"]?[^\x60\"\s(]+[\x60\"]?\s+ON\s+[\x60\"]?([^\x60\"\s(]+)[\x60\"]?\s*\(([^)]*)\)`)
)

func loadSchemaInventory(dir string) (schemaInventory, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return schemaInventory{}, err
	}
	result := schemaInventory{Objects: make(map[string]string)}
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".sql" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return schemaInventory{}, err
		}
		normalized, err := database.NormalizeSchemaScript(string(data), "sqlite")
		if err != nil {
			return schemaInventory{}, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		prefix := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		for _, statement := range database.SplitSQL(normalized) {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			match := schemaObjectPattern.FindStringSubmatch(statement)
			key := prefix + "/statement/" + digestSchemaText(statement)
			definition := statement
			if len(match) == 3 {
				kind := strings.ToUpper(strings.ReplaceAll(match[1], " ", "_"))
				key = prefix + "/" + kind + "/" + strings.ToLower(match[2])
				if indexMatch := schemaIndexPattern.FindStringSubmatch(statement); len(indexMatch) == 4 {
					unique := ""
					if strings.TrimSpace(indexMatch[1]) != "" {
						unique = "UNIQUE/"
					}
					columns := strings.ToLower(indexMatch[3])
					columns = strings.NewReplacer("`", "", `"`, "", " ", "", "\t", "", "\r", "", "\n", "").Replace(columns)
					key = prefix + "/INDEX/" + unique + strings.ToLower(indexMatch[2]) + "/" + columns
					definition = "INDEX/" + unique + strings.ToLower(indexMatch[2]) + "/" + columns
				} else {
					definition = canonicalSchemaDefinition(statement)
				}
			}
			result.Objects[key] = definition
		}
	}
	return result, nil
}

func canonicalSchemaDefinition(value string) string {
	value = strings.NewReplacer("`", "", `"`, "").Replace(value)
	if strings.HasPrefix(strings.TrimSpace(strings.ToLower(value)), "create table") {
		inlinePrimary := regexp.MustCompile(`(?i)\b([a-z0-9_]+)\s+([a-z]+)\s+primary\s+key\b`)
		match := inlinePrimary.FindStringSubmatch(value)
		if len(match) == 3 {
			value = inlinePrimary.ReplaceAllString(value, "$1 $2")
			if close := strings.LastIndex(value, ")"); close >= 0 {
				value = value[:close] + ", PRIMARY KEY (" + match[1] + ")" + value[close:]
			}
		}
	}
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func digestSchemaText(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])[:16]
}

func schemaDifferences(left, right schemaInventory) (missing, extra, mismatched []string) {
	leftKeys := make([]string, 0, len(left.Objects))
	for key := range left.Objects {
		leftKeys = append(leftKeys, key)
	}
	sort.Strings(leftKeys)
	for _, key := range leftKeys {
		leftValue := left.Objects[key]
		rightValue, ok := right.Objects[key]
		if !ok {
			missing = append(missing, key)
		} else if leftValue != rightValue {
			mismatched = append(mismatched, key)
		}
	}
	rightKeys := make([]string, 0, len(right.Objects))
	for key := range right.Objects {
		rightKeys = append(rightKeys, key)
	}
	sort.Strings(rightKeys)
	for _, key := range rightKeys {
		if _, ok := left.Objects[key]; !ok {
			extra = append(extra, key)
		}
	}
	return missing, extra, mismatched
}

func schemaAuditReport(mysqlDir, sqliteDir string) (string, int, error) {
	mysql, err := loadSchemaInventory(mysqlDir)
	if err != nil {
		return "", 0, err
	}
	sqlite, err := loadSchemaInventory(sqliteDir)
	if err != nil {
		return "", 0, err
	}
	missing, extra, mismatched := schemaDifferences(mysql, sqlite)
	var report strings.Builder
	report.WriteString("Generated from the tracked MySQL and SQLite schema scripts.\n\n")
	report.WriteString("This is a schema inventory and dialect-drift report; it does not prove runtime database compatibility.\n\n")
	fmt.Fprintf(&report, "| Dialect | Objects |\n| --- | ---: |\n| MySQL | %d |\n| SQLite | %d |\n\n", len(mysql.Objects), len(sqlite.Objects))
	fmt.Fprintf(&report, "| Comparison | Count |\n| --- | ---: |\n| Missing from SQLite | %d |\n| Extra in SQLite | %d |\n| Definition mismatches | %d |\n", len(missing), len(extra), len(mismatched))
	writeSchemaKeys := func(title string, keys []string) {
		fmt.Fprintf(&report, "\n## %s\n\n", title)
		if len(keys) == 0 {
			report.WriteString("None.\n")
			return
		}
		for _, key := range keys {
			fmt.Fprintf(&report, "- `%s`\n", key)
		}
	}
	writeSchemaKeys("Missing from SQLite", missing)
	writeSchemaKeys("Extra in SQLite", extra)
	writeSchemaKeys("Definition mismatches", mismatched)
	status := 0
	if len(missing) != 0 || len(extra) != 0 || len(mismatched) != 0 {
		status = 1
	}
	return report.String(), status, nil
}
