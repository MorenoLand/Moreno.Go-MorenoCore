package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestMovementTracksTransportState(t *testing.T) {
	sess := &session{server: &Server{sessions: make(map[*session]struct{})}, playerLoaded: true, playerGUID: 9, player: &playerState{GUID: 9, Map: 0}}
	info := movementInfo{GUID: 9, Flags: movementOnTransport | movementForward, X: 10, Y: 20, Z: 30, Orientation: 1, Transport: &transportMovement{GUID: 77, X: 1, Y: 2, Z: 3, Orientation: 4, Seat: 2}}
	payload := protocol.NewBuffer(96)
	writeMovementInfo(payload, info)
	if !sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_HEARTBEAT), payload.Bytes()) {
		t.Fatal("transport movement rejected")
	}
	if sess.player.TransportGUID != 77 || sess.player.TransportX != 1 || sess.player.TransportY != 2 || sess.player.TransportZ != 3 || sess.player.TransportO != 4 || sess.player.TransportSeat != 2 {
		t.Fatalf("transport state=%+v", sess.player)
	}
	offTransport := movementInfo{GUID: 9, Flags: movementForward, X: 11, Y: 21, Z: 31, Orientation: 2}
	payload = protocol.NewBuffer(64)
	writeMovementInfo(payload, offTransport)
	if !sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_HEARTBEAT), payload.Bytes()) {
		t.Fatal("off-transport movement rejected")
	}
	if sess.player.TransportGUID != 0 || sess.player.TransportX != 0 || sess.player.TransportSeat != 0 {
		t.Fatalf("transport state not cleared=%+v", sess.player)
	}
}
