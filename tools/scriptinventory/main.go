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

type scriptEntry struct {
	Path          string
	Lines         int
	Registrations []string
}

var addSC = regexp.MustCompile(`\bAddSC_[A-Za-z0-9_]+\s*\(`)
var luaRegistration = regexp.MustCompile(`\b(Register(?:Player|Server|Creature|GameObject|Item|Group|Guild|Spell|Map|AreaTrigger|Weather|BG|Global)Event|CreateLuaEvent)\b`)

func main() {
	reference := flag.String("reference", "", "Pinned TrinityCore checkout")
	repo := flag.String("repo", ".", "MorenoCore Go checkout")
	output := flag.String("output", "docs/SCRIPT_PARITY_MANIFEST.md", "Manifest output path")
	flag.Parse()
	if *reference == "" {
		fmt.Fprintln(os.Stderr, "-reference is required")
		os.Exit(2)
	}
	manifest, err := buildManifest(*reference, *repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dir := filepath.Dir(*output); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := os.WriteFile(*output, []byte(manifest), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote script parity manifest to %s\n", *output)
}

func buildManifest(reference, repo string) (string, error) {
	refRoot := filepath.Join(reference, "src", "server", "scripts")
	entries, err := collectScripts(refRoot, refRoot, false)
	if err != nil {
		return "", fmt.Errorf("collect reference scripts: %w", err)
	}
	goRoot := filepath.Join(repo, "engine", "scripting")
	goEntries, err := collectScripts(goRoot, goRoot, true)
	if err != nil {
		return "", fmt.Errorf("collect Go scripting sources: %w", err)
	}
	refRegistrations := make([]string, 0)
	for _, entry := range entries {
		refRegistrations = append(refRegistrations, entry.Registrations...)
	}
	goRegistrations := make([]string, 0)
	for _, entry := range goEntries {
		goRegistrations = append(goRegistrations, entry.Registrations...)
	}
	sort.Strings(refRegistrations)
	sort.Strings(goRegistrations)
	var out strings.Builder
	fmt.Fprintln(&out, "# Script parity manifest")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "This generated inventory compares the pinned reference script tree with the current Go scripting package. A listed reference file is not considered converted unless its behavior has a verified Go implementation or generated equivalent.")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "- Reference script files: %d\n- Go scripting source files: %d\n- Reference `AddSC_` registrations: %d\n- Go hook registrations: %d\n", len(entries), len(goEntries), len(unique(refRegistrations)), len(unique(goRegistrations)))
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## Reference files")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Status | Reference path | Lines | Registration symbols |")
	fmt.Fprintln(&out, "| --- | --- | ---: | --- |")
	for _, entry := range entries {
		registrations := strings.Join(entry.Registrations, ", ")
		if registrations == "" {
			registrations = "—"
		}
		fmt.Fprintf(&out, "| unported | `%s` | %d | %s |\n", filepath.ToSlash(entry.Path), entry.Lines, registrations)
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## Go scripting sources")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Path | Lines | Hook registrations |")
	fmt.Fprintln(&out, "| --- | ---: | --- |")
	for _, entry := range goEntries {
		registrations := strings.Join(entry.Registrations, ", ")
		if registrations == "" {
			registrations = "—"
		}
		fmt.Fprintf(&out, "| `%s` | %d | %s |\n", filepath.ToSlash(entry.Path), entry.Lines, registrations)
	}
	return out.String(), nil
}

func collectScripts(root, base string, goSources bool) ([]scriptEntry, error) {
	entries := make([]scriptEntry, 0)
	err := filepath.WalkDir(root, func(path string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if goSources {
			if ext != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
		} else if ext != ".cpp" && ext != ".h" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := 0
		if len(data) > 0 {
			lines = 1 + strings.Count(string(data), "\n")
		}
		matches := addSC.FindAllString(string(data), -1)
		if goSources {
			matches = luaRegistration.FindAllString(string(data), -1)
		}
		for i := range matches {
			matches[i] = strings.TrimSpace(strings.TrimSuffix(matches[i], "("))
		}
		sort.Strings(matches)
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		entries = append(entries, scriptEntry{Path: relative, Lines: lines, Registrations: unique(matches)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func unique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
