package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWorldFrameEncoding(t *testing.T) {
	frame, headerSize, err := EncodeServerFrame(0x1dd, []byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if headerSize != 4 || !bytes.Equal(frame, []byte{0, 6, 0xdd, 0x01, 1, 2, 3, 4}) {
		t.Fatalf("frame=%x header=%d", frame, headerSize)
	}
	client := []byte{0, 8, 0xdc, 1, 0, 0, 9, 8, 7, 6}
	header, payload, err := ReadClientFrame(bytes.NewReader(client), nil)
	if err != nil {
		t.Fatal(err)
	}
	if header.Opcode != 0x1dc || header.PayloadSize != 4 || !bytes.Equal(payload, []byte{9, 8, 7, 6}) {
		t.Fatalf("header=%+v payload=%x", header, payload)
	}
	var largePayload = make([]byte, 0x8000)
	frame, headerSize, err = EncodeServerFrame(2, largePayload)
	if err != nil {
		t.Fatal(err)
	}
	if headerSize != 5 || frame[0]&0x80 == 0 || int(frame[1])<<8|int(frame[2]) != len(largePayload)+2 {
		t.Fatalf("large frame header=%x", frame[:5])
	}
	if binary.LittleEndian.Uint16(frame[3:5]) != 2 {
		t.Fatalf("opcode=%x", frame[3:5])
	}
}

