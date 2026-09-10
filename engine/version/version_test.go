package version

import (
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	v := String()
	if !strings.HasPrefix(v, Base) {
		t.Errorf("expected version string to start with base %q, got %q", Base, v)
	}
}

func TestRevision(t *testing.T) {
	rev := Revision()
	if rev == "" {
		t.Errorf("expected non-empty revision string")
	}
}
