package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type sourceCounts struct {
	Files int
	Lines int
}

type toolStatus struct {
	Reference string
	GoPath    string
	Scaffold  bool
}

var (
	opcodePattern         = regexp.MustCompile(`DEFINE_(?:SERVER_)?(?:OPCODE_)?HANDLER\(\s*([A-Z0-9_]+)`)
	goHandlerPattern      = regexp.MustCompile(`case uint32\(protocol\.Opcode([A-Z0-9_]+)\)`)
	statementPattern      = regexp.MustCompile(`PrepareStatement\(\s*([A-Z0-9_]+)`)
	statementSQLPattern   = regexp.MustCompile(`PrepareStatement\(\s*([A-Z0-9_]+)\s*,\s*"((?:[^"\\]|\\.)*)"`)
	goStatementSQLPattern = regexp.MustCompile(`ID:\s*"([A-Z0-9_]+)"\s*,\s*SQL:\s*"((?:[^"\\]|\\.)*)"`)
	schemaPattern         = regexp.MustCompile(`(?i)CREATE\s+(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?[\x60]?([A-Za-z0-9_]+)[\x60]?`)
)

func main() {
	reference := flag.String("reference", "", "checked-out TrinityCore reference directory")
	repo := flag.String("repo", ".", "MorenoCore repository directory")
	output := flag.String("output", "docs/PARITY_COVERAGE.md", "coverage report path")
	flag.Parse()
	if *reference == "" {
		fail("-reference is required")
	}
	if _, err := os.Stat(*reference); err != nil {
		fail(err.Error())
	}
	if _, err := os.Stat(*repo); err != nil {
		fail(err.Error())
	}
	report, err := buildReport(*reference, *repo)
	if err != nil {
		fail(err.Error())
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*output, []byte(report), 0o644); err != nil {
		fail(err.Error())
	}
	fmt.Printf("Wrote parity coverage report to %s\n", *output)
}

func buildReport(reference, repo string) (string, error) {
	refServer, err := countSources(filepath.Join(reference, "src", "server"))
	if err != nil {
		return "", err
	}
	refTools, err := countSources(filepath.Join(reference, "src", "tools"))
	if err != nil {
		return "", err
	}
	goSources, err := countSources(repo)
	if err != nil {
		return "", err
	}
	refOpcodes, err := matches(filepath.Join(reference, "src", "server"), opcodePattern, func(path string) bool { return filepath.Base(path) == "Opcodes.cpp" })
	if err != nil {
		return "", err
	}
	goOpcodes, err := matches(filepath.Join(repo, "engine", "world"), goHandlerPattern, nil)
	if err != nil {
		return "", err
	}
	refStatementSQL, err := statementDetails(filepath.Join(reference, "src", "server"), statementSQLPattern, nil)
	if err != nil {
		return "", err
	}
	goStatementSQL, err := statementDetails(filepath.Join(repo, "engine", "database"), goStatementSQLPattern, nil)
	if err != nil {
		return "", err
	}
	refStatements := keys(refStatementSQL)
	goStatements := keys(goStatementSQL)
	refSchema, err := sqlMatches(filepath.Join(reference, "sql"), schemaPattern)
	if err != nil {
		return "", err
	}
	goMySQLSchema, err := sqlMatches(filepath.Join(repo, "sql", "mysql"), schemaPattern)
	if err != nil {
		return "", err
	}
	goSQLiteSchema, err := sqlMatches(filepath.Join(repo, "sql", "sqlite"), schemaPattern)
	if err != nil {
		return "", err
	}
	refScripts, err := countSources(filepath.Join(reference, "src", "server", "scripts"))
	if err != nil {
		return "", err
	}
	goScripts, err := countSources(filepath.Join(repo, "engine", "scripting"))
	if err != nil {
		return "", err
	}
	refTests, err := countSources(filepath.Join(reference, "tests"))
	if err != nil {
		return "", err
	}
	goTests := countTestSources(repo)
	tools := []toolStatus{
		{Reference: "map_extractor", GoPath: "tools/mapextractor"},
		{Reference: "vmap4_extractor", GoPath: "tools/vmap4extractor"},
		{Reference: "vmap4_assembler", GoPath: "tools/vmap4assembler"},
		{Reference: "mmaps_generator", GoPath: "tools/mmaps-generator"},
	}
	for index := range tools {
		content, err := readGoFiles(filepath.Join(repo, tools[index].GoPath))
		if err != nil {
			return "", err
		}
		tools[index].Scaffold = strings.Contains(strings.ToLower(content), "scaffolded") || strings.Contains(strings.ToLower(content), "not implemented")
	}
	var report strings.Builder
	fmt.Fprintf(&report, "Generated from the checked-out reference and current MorenoCore source.\n\n")
	fmt.Fprintf(&report, "This report is an inventory aid; matching counts do not prove behavioral parity.\n\n")
	fmt.Fprintf(&report, "| Area | Reference | Go | Missing reference symbols |\n| --- | ---: | ---: | ---: |\n")
	fmt.Fprintf(&report, "| Server source files / lines | %d / %d | %d / %d | — |\n", refServer.Files, refServer.Lines, goSources.Files, goSources.Lines)
	fmt.Fprintf(&report, "| Tool source files / lines | %d / %d | — | — |\n", refTools.Files, refTools.Lines)
	fmt.Fprintf(&report, "| Opcode handlers | %d | %d | %d |\n", len(refOpcodes), len(goOpcodes), len(difference(refOpcodes, goOpcodes)))
	fmt.Fprintf(&report, "| Prepared statement identifiers | %d | %d | %d |\n", len(refStatements), len(goStatements), len(difference(refStatements, goStatements)))
	fmt.Fprintf(&report, "| Prepared statement SQL mismatches | — | — | %d |\n", len(sqlDifferences(refStatementSQL, goStatementSQL)))
	fmt.Fprintf(&report, "| Schema tables/views | %d | %d mysql / %d sqlite | %d mysql / %d sqlite |\n", len(refSchema), len(goMySQLSchema), len(goSQLiteSchema), len(difference(keys(refSchema), keys(goMySQLSchema))), len(difference(keys(refSchema), keys(goSQLiteSchema))))
	fmt.Fprintf(&report, "| Script source files / lines | %d / %d | %d / %d | — |\n", refScripts.Files, refScripts.Lines, goScripts.Files, goScripts.Lines)
	fmt.Fprintf(&report, "| Test source files / lines | %d / %d | %d / %d | — |\n", refTests.Files, refTests.Lines, goTests.Files, goTests.Lines)
	fmt.Fprintf(&report, "\n## Missing opcode handlers\n\n%s\n", list(difference(refOpcodes, goOpcodes)))
	fmt.Fprintf(&report, "## Missing prepared statements\n\n%s\n", list(difference(refStatements, goStatements)))
	fmt.Fprintf(&report, "## Prepared statement SQL mismatches\n\n%s\n", list(sqlDifferences(refStatementSQL, goStatementSQL)))
	fmt.Fprintf(&report, "## Missing schema tables/views\n\n### MySQL\n\n%s\n\n### SQLite\n\n%s\n", list(difference(keys(refSchema), keys(goMySQLSchema))), list(difference(keys(refSchema), keys(goSQLiteSchema))))
	fmt.Fprintf(&report, "## Extraction tools\n\n| Reference tool | Go path | Status |\n| --- | --- | --- |\n")
	for _, tool := range tools {
		status := "implemented source"
		if tool.Scaffold {
			status = "contains explicit scaffold/not implemented path"
		}
		fmt.Fprintf(&report, "| `%s` | `%s` | %s |\n", tool.Reference, tool.GoPath, status)
	}
	return report.String(), nil
}

func countSources(root string) (sourceCounts, error) {
	counts := sourceCounts{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isSource(path) {
			return nil
		}
		counts.Files++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		counts.Lines += strings.Count(string(data), "\n")
		return nil
	})
	return counts, err
}

func matches(root string, pattern *regexp.Regexp, filter func(string) bool) ([]string, error) {
	values := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isSource(path) || filter != nil && !filter(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			if len(match) > 1 {
				values[match[1]] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func statementDetails(root string, pattern *regexp.Regexp, filter func(string) bool) (map[string]string, error) {
	values := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isSource(path) || filter != nil && !filter(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			if len(match) > 2 {
				values[match[1]] = match[2]
			}
		}
		return nil
	})
	return values, err
}

func sqlMatches(root string, pattern *regexp.Regexp) (map[string]string, error) {
	values := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".sql" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			if len(match) > 1 {
				values[strings.ToLower(match[1])] = filepath.Base(path)
			}
		}
		return nil
	})
	return values, err
}

func sqlDifferences(reference, implementation map[string]string) []string {
	values := make([]string, 0)
	for name, sql := range reference {
		implementationSQL, ok := implementation[name]
		if ok && normalizeSQL(sql) != normalizeSQL(implementationSQL) {
			values = append(values, name)
		}
	}
	sort.Strings(values)
	return values
}

func normalizeSQL(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func keys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func countOptionalSources(root string) sourceCounts {
	counts, err := countSources(root)
	if err != nil {
		return sourceCounts{}
	}
	return counts
}

func countTestSources(root string) sourceCounts {
	counts := sourceCounts{}
	if _, err := os.Stat(root); err != nil {
		return counts
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(path), "_test.go") {
			return nil
		}
		if data, err := os.ReadFile(path); err == nil {
			counts.Files++
			counts.Lines += strings.Count(string(data), "\n")
		}
		return nil
	})
	return counts
}

func readGoFiles(root string) (string, error) {
	var content strings.Builder
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content.Write(data)
		return nil
	})
	return content.String(), err
}

func difference(reference, implementation []string) []string {
	known := make(map[string]struct{}, len(implementation))
	for _, value := range implementation {
		known[value] = struct{}{}
	}
	missing := make([]string, 0)
	for _, value := range reference {
		if _, ok := known[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}

func list(values []string) string {
	if len(values) == 0 {
		return "No missing symbols detected."
	}
	var output strings.Builder
	for _, value := range values {
		fmt.Fprintf(&output, "- `%s`\n", value)
	}
	return output.String()
}

func isSource(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".cpp" || extension == ".h" || extension == ".go"
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
