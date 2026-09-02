package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBattlegroundHandlers(t *testing.T) {
	srv := &Server{Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. CMSG_BATTLEMASTER_HELLO
	bmBuf := protocol.NewBuffer(8)
	bmBuf.WriteU64(12345)
	if !sess.handleBattlemasterHello(ctx, bmBuf.Bytes()) {
		t.Fatal("handleBattlemasterHello failed")
	}

	// 2. CMSG_BATTLEFIELD_LIST
	listBuf := protocol.NewBuffer(8)
	listBuf.WriteU32(1) // Warsong Gulch
	listBuf.WriteU8(0)
	listBuf.WriteU8(1)
	if !sess.handleBattlefieldList(ctx, listBuf.Bytes()) {
		t.Fatal("handleBattlefieldList failed")
	}

	// 3. CMSG_BATTLEMASTER_JOIN (Queue for WSG)
	joinBuf := protocol.NewBuffer(17)
	joinBuf.WriteU64(12345)
	joinBuf.WriteU32(1) // WSG
	joinBuf.WriteU32(0) // First available
	joinBuf.WriteU8(0)  // Solo
	if !sess.handleBattlemasterJoin(ctx, joinBuf.Bytes()) {
		t.Fatal("handleBattlemasterJoin failed")
	}
	if !sess.bgQueues[0].Active || sess.bgQueues[0].BgTypeID != 1 {
		t.Fatalf("expected queue 0 active for WSG: %+v", sess.bgQueues[0])
	}

	// 4. CMSG_BATTLEMASTER_JOIN_ARENA (Queue for Arena in slot 1)
	arenaBuf := protocol.NewBuffer(8)
	arenaBuf.WriteU64(0)
	arenaBuf.WriteU8(2) // 2v2
	arenaBuf.WriteU8(0)
	arenaBuf.WriteU8(1) // Rated
	if !sess.handleBattlemasterJoinArena(ctx, arenaBuf.Bytes()) {
		t.Fatal("handleBattlemasterJoinArena failed")
	}
	if !sess.bgQueues[1].Active || sess.bgQueues[1].BgTypeID != 4 {
		t.Fatalf("expected queue 1 active for Arena: %+v", sess.bgQueues[1])
	}

	// 5. CMSG_BATTLEFIELD_STATUS
	if !sess.handleBattlefieldStatus(ctx, nil) {
		t.Fatal("handleBattlefieldStatus failed")
	}

	// 6. CMSG_BATTLEFIELD_PORT (Leave queue for WSG)
	portBuf := protocol.NewBuffer(9)
	portBuf.WriteU8(0)
	portBuf.WriteU8(0)
	portBuf.WriteU32(1) // WSG
	portBuf.WriteU16(0x1F90)
	portBuf.WriteU8(0)  // action 0 = leave queue
	if !sess.handleBattlefieldPort(ctx, portBuf.Bytes()) {
		t.Fatal("handleBattlefieldPort failed")
	}
	if sess.bgQueues[0].Active {
		t.Fatalf("expected queue 0 inactive after leaving: %+v", sess.bgQueues[0])
	}
}
