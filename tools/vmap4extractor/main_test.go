package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestModelExtensions(t *testing.T) {
	valid := []string{"test.wmo", "creature.m2", "mesh.mdx", "world.wdt"}
	for _, f := range valid {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".wmo" && ext != ".m2" && ext != ".mdx" && ext != ".wdt" {
			t.Errorf("expected valid model extension for %s", f)
		}
	}

	invalid := []string{"sound.wav", "texture.blp", "db.dbc"}
	for _, f := range invalid {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".wmo" || ext == ".m2" || ext == ".mdx" || ext == ".wdt" {
			t.Errorf("expected invalid model extension for %s", f)
		}
	}
}

func TestPrintBanner(t *testing.T) {
	// Verify printBanner executes without panic
	printBanner()
}
