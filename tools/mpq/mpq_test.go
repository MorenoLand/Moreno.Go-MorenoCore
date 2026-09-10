package mpq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"foo\\bar.dbc", "FOO/BAR.DBC"},
		{"FOO//BAR.DBC", "FOO/BAR.DBC"},
		{"a\\b\\c.mpq", "A/B/C.MPQ"},
	}
	for _, tc := range cases {
		actual := normalize(tc.input)
		if actual != tc.expected {
			t.Errorf("normalize(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestHashStringDeterministic(t *testing.T) {
	h1 := hashString("Interface/FrameXML/UI.xml", 0)
	h2 := hashString("Interface/FrameXML/UI.xml", 0)
	if h1 != h2 {
		t.Fatalf("expected deterministic hashString, got %x vs %x", h1, h2)
	}
	if h1 == 0 {
		t.Fatal("expected non-zero hash")
	}

	hType1 := hashString("test", 1)
	hType2 := hashString("test", 2)
	if hType1 == hType2 {
		t.Fatal("expected different hashes for different hashTypes")
	}
}

func TestArchivesDiscovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpq_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy .mpq files
	f1 := filepath.Join(tempDir, "common.mpq")
	f2 := filepath.Join(tempDir, "patch.mpq")
	_ = os.WriteFile(f1, []byte("dummy1"), 0o644)
	_ = os.WriteFile(f2, []byte("dummy2"), 0o644)

	archives, err := Archives(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 2 {
		t.Fatalf("expected 2 archives, got %d", len(archives))
	}
}

func TestOpenInvalidFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mpq_open_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	invalidFile := filepath.Join(tempDir, "invalid.mpq")
	_ = os.WriteFile(invalidFile, []byte("not an mpq file"), 0o644)

	_, err = Open(invalidFile)
	if err == nil {
		t.Fatal("expected error opening non-MPQ file")
	}
}
