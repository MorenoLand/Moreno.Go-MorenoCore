//go:build ignore

package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCalendarAndBattlefieldMgrHandlers(t *testing.T) {
	srv := &Server{Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. BattlefieldMgr tests
	entryBuf := protocol.NewBuffer(5)
	entryBuf.WriteU32(1)
	entryBuf.WriteU8(1)
	if !sess.handleBfEntryInviteResponse(ctx, entryBuf.Bytes()) {
		t.Fatal("handleBfEntryInviteResponse failed")
	}

	queueBuf := protocol.NewBuffer(5)
	queueBuf.WriteU32(1)
	queueBuf.WriteU8(1)
	if !sess.handleBfQueueInviteResponse(ctx, queueBuf.Bytes()) {
		t.Fatal("handleBfQueueInviteResponse failed")
	}

	exitBuf := protocol.NewBuffer(4)
	exitBuf.WriteU32(1)
	if !sess.handleBfQueueExitRequest(ctx, exitBuf.Bytes()) {
		t.Fatal("handleBfQueueExitRequest failed")
	}

	// 2. Calendar tests
	if !sess.handleCalendarGetCalendar(ctx, nil) {
		t.Fatal("handleCalendarGetCalendar failed")
	}

	if !sess.handleCalendarGetNumPending(ctx, nil) {
		t.Fatal("handleCalendarGetNumPending failed")
	}

	getEventBuf := protocol.NewBuffer(8)
	getEventBuf.WriteU64(100)
	if !sess.handleCalendarGetEvent(ctx, getEventBuf.Bytes()) {
		t.Fatal("handleCalendarGetEvent failed")
	}

	if !sess.handleCalendarGuildFilter(ctx, nil) {
		t.Fatal("handleCalendarGuildFilter failed")
	}

	arenaTeamBuf := protocol.NewBuffer(4)
	arenaTeamBuf.WriteU32(1)
	if !sess.handleCalendarArenaTeam(ctx, arenaTeamBuf.Bytes()) {
		t.Fatal("handleCalendarArenaTeam failed")
	}

	if !sess.handleCalendarAddEvent(ctx, nil) {
		t.Fatal("handleCalendarAddEvent failed")
	}

	if !sess.handleCalendarUpdateEvent(ctx, nil) {
		t.Fatal("handleCalendarUpdateEvent failed")
	}

	if !sess.handleCalendarRemoveEvent(ctx, nil) {
		t.Fatal("handleCalendarRemoveEvent failed")
	}

	if !sess.handleCalendarCopyEvent(ctx, nil) {
		t.Fatal("handleCalendarCopyEvent failed")
	}

	if !sess.handleCalendarEventInvite(ctx, nil) {
		t.Fatal("handleCalendarEventInvite failed")
	}

	if !sess.handleCalendarEventSignup(ctx, nil) {
		t.Fatal("handleCalendarEventSignup failed")
	}

	if !sess.handleCalendarEventRSVP(ctx, nil) {
		t.Fatal("handleCalendarEventRSVP failed")
	}

	if !sess.handleCalendarEventRemoveInvite(ctx, nil) {
		t.Fatal("handleCalendarEventRemoveInvite failed")
	}

	if !sess.handleCalendarEventStatus(ctx, nil) {
		t.Fatal("handleCalendarEventStatus failed")
	}

	if !sess.handleCalendarEventModeratorStatus(ctx, nil) {
		t.Fatal("handleCalendarEventModeratorStatus failed")
	}

	if !sess.handleCalendarComplain(ctx, nil) {
		t.Fatal("handleCalendarComplain failed")
	}
}

