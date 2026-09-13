package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildManifestInventoriesReferenceAndGoRegistrations(t *testing.T) {
	root := t.TempDir()
	ref := filepath.Join(root, "reference")
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(ref, "src", "server", "scripts", "World"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "engine", "scripting"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref, "src", "server", "scripts", "World", "world.cpp"), []byte("void AddSC_world() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "engine", "scripting", "runtime.go"), []byte("func RegisterPlayerEvent() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := buildManifest(ref, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Reference script files: 1", "AddSC_world", "RegisterPlayerEvent", "unported"} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
}
