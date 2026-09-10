package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/tools/wowdata"
)

func TestExtractDBCMissingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "does_not_exist")
	outputDir := filepath.Join(tempDir, "output")

	_, err := wowdata.ExtractDBC(nonExistent, outputDir)
	if err == nil {
		t.Fatalf("expected error when extracting from non-existent directory")
	}
}

func TestExtractDBCInvalidDir(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Empty dir without MPQs should return 0 count or error
	count, err := wowdata.ExtractDBC(tempDir, outputDir)
	if err != nil && count != 0 {
		t.Fatalf("unexpected count %d with err: %v", count, err)
	}
	_ = os.RemoveAll(tempDir)
}
