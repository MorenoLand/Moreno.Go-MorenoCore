package protocol

import (
	"bytes"
	"testing"
)

func TestUpdateMaskEncoding(t *testing.T) {
	mask := NewUpdateMask(65)
	if err := mask.Set(0); err != nil {
		t.Fatal(err)
	}
	if err := mask.Set(32); err != nil {
		t.Fatal(err)
	}
	if err := mask.Set(64); err != nil {
		t.Fatal(err)
	}
	b := NewBuffer(12)
	mask.AppendTo(b)
	if !bytes.Equal(b.Bytes(), []byte{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}) {
		t.Fatalf("mask=%x", b.Bytes())
	}
}

func TestUpdateDataCompressionRoundTrip(t *testing.T) {
	updates := NewUpdateData()
	updates.AddOutOfRangeGUID(2)
	updates.AddOutOfRangeGUID(1)
	updates.AddUpdateBlock(bytes.Repeat([]byte{7}, 120))
	packet, err := updates.BuildPacket(1)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Opcode != uint16(OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("opcode=%x", packet.Opcode)
	}
	payload, err := DecompressUpdatePayload(packet.Payload.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 13 || payload[0] != 2 || payload[4] != UpdateOutOfRangeObjects {
		t.Fatalf("payload=%x", payload[:minInt(len(payload), 24)])
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
