//go:build ignore

package world

import (
	"encoding/binary"
	"testing"
)

func TestBuildTimeSyncRequest(t *testing.T) {
	packet := buildTimeSyncRequest(7)
	if len(packet) != 4 || binary.LittleEndian.Uint32(packet) != 7 {
		t.Fatalf("packet=%x", packet)
	}
}

