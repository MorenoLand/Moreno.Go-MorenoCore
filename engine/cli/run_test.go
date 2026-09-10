package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/service"
)

func TestDiscoverConfigExplicit(t *testing.T) {
	path := "/some/explicit/path.conf"
	found := discoverConfig(path, service.Auth, "")
	if found != path {
		t.Errorf("expected %q, got %q", path, found)
	}
}

func TestDiscoverConfigWorkDir(t *testing.T) {
	tempDir := t.TempDir()
	confPath := filepath.Join(tempDir, "authserver.conf")
	if err := os.WriteFile(confPath, []byte("[auth]\n"), 0o644); err != nil {
		t.Fatalf("failed to write test conf: %v", err)
	}

	found := discoverConfig("", service.Auth, tempDir)
	if found != confPath {
		t.Errorf("expected %q, got %q", confPath, found)
	}
}

func TestNewLogger(t *testing.T) {
	l1 := newLogger(false)
	if l1 == nil {
		t.Errorf("expected non-nil logger")
	}
	l2 := newLogger(true)
	if l2 == nil {
		t.Errorf("expected non-nil logger")
	}
}
