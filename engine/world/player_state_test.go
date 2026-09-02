package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestPlayerCreateMask(t *testing.T) {
	if playerCreateMask(3) != 0x04 || playerCreateMask(11) != 0x400 || playerCreateMask(0) != 0 {
		t.Fatalf("masks are incorrect")
	}
}

func TestBuildInitialReputations(t *testing.T) {
	payload := buildInitialReputations(playerState{Reputations: []playerReputation{{ListID: 72, Standing: 42999, Flags: 1}}})
	reader := protocol.NewReader(payload)
	count, err := reader.ReadU32()
	if err != nil || count != 128 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	for index := uint32(0); index < count; index++ {
		flags, flagsErr := reader.ReadU8()
		standing, standingErr := reader.ReadU32()
		if flagsErr != nil || standingErr != nil {
			t.Fatalf("entry %d flags=%v standing=%v", index, flagsErr, standingErr)
		}
		if index == 72 && (flags != 1 || standing != 42999) {
			t.Fatalf("stormwind entry flags=%d standing=%d", flags, standing)
		}
	}
}

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
	if fallTime, err := reader.ReadU32(); err != nil || fallTime != 0 {
		t.Fatalf("fall time=%d err=%v", fallTime, err)
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
