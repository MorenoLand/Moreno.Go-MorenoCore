package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type statement struct {
	ID    string
	SQL   string
	Async bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: generate opcodes|statements")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "opcodes":
		os.Exit(generateOpcodes(os.Args[2:]))
	case "statements":
		os.Exit(generateStatements(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown generator command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func generateOpcodes(args []string) int {
	fs := flag.NewFlagSet("opcodes", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "reference Opcodes.h")
	output := fs.String("output", "", "generated Go file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "--input and --output are required")
		return 2
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	pattern := regexp.MustCompile(`(?m)^\s*([A-Z][A-Z0-9_]+)\s*=\s*(0x[0-9A-Fa-f]+|[0-9]+)`)
	type opcode struct{ Name, Value string }
	seen := map[string]bool{}
	values := make([]opcode, 0)
	for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
		if seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		values = append(values, opcode{Name: match[1], Value: match[2]})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	var b strings.Builder
	b.WriteString("package protocol\n\n")
	b.WriteString("type Opcode uint16\n\nconst (\n")
	for _, value := range values {
		fmt.Fprintf(&b, "\tOpcode%s Opcode = %s\n", value.Name, value.Value)
	}
	b.WriteString(")\n\nvar OpcodeNames = map[Opcode]string{\n")
	mapValues := map[uint64]bool{}
	for _, value := range values {
		number, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(value.Value, "0x"), "0X"), 16, 16)
		if !strings.HasPrefix(strings.ToLower(value.Value), "0x") {
			number, err = strconv.ParseUint(value.Value, 10, 16)
		}
		if err != nil || mapValues[number] {
			continue
		}
		mapValues[number] = true
		fmt.Fprintf(&b, "\tOpcode%s: %q,\n", value.Name, value.Name)
	}
	b.WriteString("}\n")
	return writeFile(*output, []byte(b.String()))
}

func generateStatements(args []string) int {
	fs := flag.NewFlagSet("statements", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "reference database implementation directory")
	output := fs.String("output", "", "generated Go file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "--input and --output are required")
		return 2
	}
	files := []struct{ Name, File string }{{"Login", "LoginDatabase.cpp"}, {"Character", "CharacterDatabase.cpp"}, {"World", "WorldDatabase.cpp"}}
	var groups = make([][]statement, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(*input, file.File))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		values, err := parseStatements(string(data))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		groups = append(groups, values)
	}
	var b strings.Builder
	b.WriteString("package database\n\n")
	b.WriteString("type StatementID string\n\ntype StatementDefinition struct { ID StatementID; SQL string; Async bool }\n\n")
	for i, file := range files {
		fmt.Fprintf(&b, "var %sStatements = []StatementDefinition{\n", file.Name)
		for _, value := range groups[i] {
			fmt.Fprintf(&b, "\t{ID: %q, SQL: %q, Async: %t},\n", value.ID, value.SQL, value.Async)
		}
		b.WriteString("}\n\n")
	}
	b.WriteString("func AllStatements() []StatementDefinition {\n\tresult := make([]StatementDefinition, 0, ")
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	fmt.Fprintf(&b, "%d)\n", total)
	for _, file := range files {
		fmt.Fprintf(&b, "\tresult = append(result, %sStatements...)\n", file.Name)
	}
	b.WriteString("\treturn result\n}\n")
	return writeFile(*output, []byte(b.String()))
}

func parseStatements(source string) ([]statement, error) {
	result := make([]statement, 0)
	seen := map[string]bool{}
	for offset := 0; offset < len(source); {
		relative := strings.Index(source[offset:], "PrepareStatement(")
		if relative < 0 {
			break
		}
		start := offset + relative
		open := start + len("PrepareStatement")
		close := callEnd(source, open)
		if close < 0 {
			return nil, fmt.Errorf("unterminated PrepareStatement call at byte %d", start)
		}
		args := splitArguments(source[open+1 : close])
		if len(args) < 3 {
			return nil, fmt.Errorf("PrepareStatement call at byte %d has %d arguments", start, len(args))
		}
		id := strings.TrimSpace(args[0])
		if !regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`).MatchString(id) {
			return nil, fmt.Errorf("invalid statement identifier %q", id)
		}
		query, err := concatenateStrings(args[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		if query == "" {
			return nil, fmt.Errorf("%s has no SQL string literal", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate statement identifier %s", id)
		}
		seen[id] = true
		result = append(result, statement{ID: id, SQL: query, Async: strings.Contains(args[2], "CONNECTION_ASYNC")})
		offset = close + 1
	}
	return result, nil
}

func callEnd(source string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(source); i++ {
		ch := source[i]
		if quote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitArguments(value string) []string {
	result := make([]string, 0, 3)
	start := 0
	depth := 0
	var quote byte
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if quote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
		} else if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		} else if ch == ',' && depth == 0 {
			result = append(result, strings.TrimSpace(value[start:i]))
			start = i + 1
		}
	}
	result = append(result, strings.TrimSpace(value[start:]))
	return result
}

func concatenateStrings(value string) (string, error) {
	pattern := regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
	matches := pattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return "", errors.New("argument contains no C++ string literals")
	}
	var result strings.Builder
	for _, match := range matches {
		decoded, err := strconv.Unquote("\"" + match[1] + "\"")
		if err != nil {
			return "", err
		}
		result.WriteString(decoded)
	}
	return result.String(), nil
}

func writeFile(path string, data []byte) int {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("wrote %s\n", path)
	return 0
}
