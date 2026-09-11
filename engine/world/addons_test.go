package world

import (
	"bytes"
	"compress/zlib"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestAddonInfoResponseKeepsClientAddonsNonBanned(t *testing.T) {
	packed := protocol.NewBuffer(64)
	packed.WriteU32(1)
	packed.WriteCString("Blizzard_CombatLog")
	packed.WriteU8(1)
	packed.WriteU32(0x4c1c776d)
	packed.WriteU32(0)
	packed.WriteU32(0)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(packed.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	payload := protocol.NewBuffer(4 + compressed.Len())
	payload.WriteU32(uint32(packed.Len()))
	payload.Write(compressed.Bytes())
	response := protocol.NewReader(buildAddonInfoResponse(payload.Bytes()))
	status, err := response.ReadU8()
	if err != nil || status != 2 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	infoProvided, err := response.ReadU8()
	if err != nil || infoProvided != 1 {
		t.Fatalf("infoProvided=%d err=%v", infoProvided, err)
	}
	keyProvided, err := response.ReadU8()
	if err != nil || keyProvided != 0 {
		t.Fatalf("keyProvided=%d err=%v", keyProvided, err)
	}
	if revision, err := response.ReadU32(); err != nil || revision != 0 {
		t.Fatalf("revision=%d err=%v", revision, err)
	}
	if urlProvided, err := response.ReadU8(); err != nil || urlProvided != 0 {
		t.Fatalf("urlProvided=%d err=%v", urlProvided, err)
	}
	banned, err := response.ReadU32()
	if err != nil || banned != 0 {
		t.Fatalf("banned=%d err=%v", banned, err)
	}
}
