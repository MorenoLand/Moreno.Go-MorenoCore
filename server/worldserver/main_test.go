package main

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/service"
)

func TestWorldServerKind(t *testing.T) {
	if service.World != "worldserver" {
		t.Errorf("expected worldserver, got %s", service.World)
	}
}
