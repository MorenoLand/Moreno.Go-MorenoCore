package main

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/service"
)

func TestAuthServerKind(t *testing.T) {
	if service.Auth != "authserver" {
		t.Errorf("expected authserver, got %s", service.Auth)
	}
}
