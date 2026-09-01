package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildPlayerUpdateKeepsMovementAndUpdateMaskAligned(t *testing.T) {
	server := &Server{}
	packet, err := server.buildPlayerUpdate(playerState{GUID: 26, Level: 21, Map: 0, X: 1, Y: 2, Z: 3, Orientation: 4})
	if err != nil {
		t.Fatal(err)
	}
	payload := packet.Payload.Bytes()
	if packet.Opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		payload, err = protocol.DecompressUpdatePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader := protocol.NewReader(payload)
	if blocks, err := reader.ReadU32(); err != nil || blocks != 1 {
		t.Fatalf("blocks=%d err=%v", blocks, err)
	}
	if updateType, err := reader.ReadU8(); err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", updateType, err)
	}
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != 26 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
	if objectType, err := reader.ReadU8(); err != nil || objectType != 4 {
		t.Fatalf("object type=%d err=%v", objectType, err)
	}
	if flags, err := reader.ReadU16(); err != nil || flags != 0x0061 {
		t.Fatalf("update flags=%x err=%v", flags, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU16(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 4; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 9; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	if maskBlocks, err := reader.ReadU8(); err != nil || maskBlocks != 42 {
		t.Fatalf("mask blocks=%d err=%v", maskBlocks, err)
	}
}
