package wowdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractDBCNoArchives(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "extract_dbc_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "out")
	count, err := ExtractDBC(tempDir, outDir)
	if err == nil {
		t.Fatalf("expected error for empty dir, got count=%d", count)
	}
}
