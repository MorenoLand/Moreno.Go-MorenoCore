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
	maskBlocks, err := reader.ReadU8()
	if err != nil || maskBlocks != 42 {
		t.Fatalf("mask blocks=%d err=%v", maskBlocks, err)
	}
	mask := make([]uint32, maskBlocks)
	for index := range mask {
		if mask[index], err = reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if mask[0]&0x3 != 0x3 {
		t.Fatalf("object guid mask=%x", mask[0])
	}
	values := make(map[int]uint32)
	for index := 0; index < playerValuesCount; index++ {
		if mask[index/32]&(1<<uint(index%32)) == 0 {
			continue
		}
		value, readErr := reader.ReadU32()
		if readErr != nil {
			t.Fatal(readErr)
		}
		values[index] = value
	}
	if values[0] != 26 || values[1] != 0 || values[2] != 0x19 {
		t.Fatalf("object values=%x/%x/%x", values[0], values[1], values[2])
	}
}
