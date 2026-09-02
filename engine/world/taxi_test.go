package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestTaxiMenuAndActivation(t *testing.T) {
	srv := &Server{}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}

	flightMasterGUID := creatureWorldGUID(1, 404)

	// 1. Status query
	statusBuf := protocol.NewBuffer(8)
	statusBuf.WriteU64(flightMasterGUID)
	if !sess.handleTaxiNodeStatusQuery(context.Background(), statusBuf.Bytes()) {
		t.Fatal("handleTaxiNodeStatusQuery failed")
	}

	// 2. Query available nodes
	queryBuf := protocol.NewBuffer(8)
	queryBuf.WriteU64(flightMasterGUID)
	if !sess.handleTaxiQueryAvailableNodes(context.Background(), queryBuf.Bytes()) {
		t.Fatal("handleTaxiQueryAvailableNodes failed")
	}

	// 3. Activate taxi flight
	actBuf := protocol.NewBuffer(16)
	actBuf.WriteU64(flightMasterGUID)
	actBuf.WriteU32(1) // source
	actBuf.WriteU32(2) // dest
	if !sess.handleActivateTaxi(context.Background(), actBuf.Bytes()) {
		t.Fatal("handleActivateTaxi failed")
	}
}

