package world

import (
	"bytes"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestMovementInfoRoundTrip(t *testing.T) {
	original := movementInfo{GUID: 99, Flags: movementOnTransport | movementSwimming | movementFalling | movementSplineElevation, Flags2: movement2Interpolated, Time: 123, X: 1.5, Y: 2.5, Z: 3.5, Orientation: 0.5, Pitch: 0.25, HasPitch: true, FallTime: 456, Jump: [4]float32{1, 2, 3, 4}, HasJump: true, SplineElevation: 5, HasSpline: true, Transport: &transportMovement{GUID: 7, X: 6, Y: 7, Z: 8, Orientation: 9, Time: 10, Seat: -1, Time2: 11, HasTime2: true}}
	encoded := protocol.NewBuffer(128)
	writeMovementInfo(encoded, original)
	reader := protocol.NewReader(encoded.Bytes())
	guid, err := reader.ReadPackedGUID()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := readMovementInfo(reader)
	if err != nil {
		t.Fatal(err)
	}
	decoded.GUID = guid
	if reader.Remaining() != 0 {
		t.Fatalf("remaining bytes=%d", reader.Remaining())
	}
	reencoded := protocol.NewBuffer(128)
	writeMovementInfo(reencoded, decoded)
	if !bytes.Equal(encoded.Bytes(), reencoded.Bytes()) {
		t.Fatalf("movement bytes differ: %x != %x", encoded.Bytes(), reencoded.Bytes())
	}
}

func TestSanitizeMovementFlags(t *testing.T) {
	if got := sanitizeMovementFlags(movementForward | movementBackward | movementStrafeLeft | movementStrafeRight | movementTurnLeft | movementTurnRight | movementAscending | movementDescending); got != 0 {
		t.Fatalf("sanitized flags=%x", got)
	}
}

