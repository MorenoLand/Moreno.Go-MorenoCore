//go:build ignore

package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
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

func TestCalendarEventCRUDWithDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE calendar_events (
			id INTEGER PRIMARY KEY,
			creator INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			type INTEGER NOT NULL DEFAULT 4,
			dungeon INTEGER NOT NULL DEFAULT -1,
			eventtime INTEGER NOT NULL DEFAULT 0,
			flags INTEGER NOT NULL DEFAULT 0,
			time2 INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE calendar_invites (
			id INTEGER PRIMARY KEY,
			event INTEGER NOT NULL DEFAULT 0,
			invitee INTEGER NOT NULL DEFAULT 0,
			sender INTEGER NOT NULL DEFAULT 0,
			status INTEGER NOT NULL DEFAULT 0,
			statustime INTEGER NOT NULL DEFAULT 0,
			rank INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL DEFAULT ''
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, Config: config.Default()}
	sess := &session{server: srv, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// Drain frames in background
	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	// 1. Add event
	addBuf := protocol.NewBuffer(64)
	addBuf.WriteCString("ICC Raid")
	addBuf.WriteCString("Bring flasks")
	addBuf.WriteU8(1)  // eventType
	addBuf.WriteU8(0)  // repeatable
	addBuf.WriteU32(25) // maxInvites
	addBuf.WriteI32(631) // dungeonID
	addBuf.WriteU32(uint32(time.Now().Unix())) // eventPackedTime
	addBuf.WriteU32(0) // unkPackedTime
	addBuf.WriteU32(0) // flags

	if !sess.handleCalendarAddEvent(ctx, addBuf.Bytes()) {
		t.Fatal("handleCalendarAddEvent failed")
	}

	// Verify event inserted into DB
	var title string
	err = db.QueryRow("SELECT title FROM calendar_events WHERE id = 1").Scan(&title)
	if err != nil || title != "ICC Raid" {
		t.Fatalf("expected 'ICC Raid' in calendar_events, got '%s' err=%v", title, err)
	}

	// 2. Query calendar
	if !sess.handleCalendarGetCalendar(ctx, nil) {
		t.Fatal("handleCalendarGetCalendar failed")
	}

	// 3. Update event
	updBuf := protocol.NewBuffer(64)
	updBuf.WriteU64(1) // eventID
	updBuf.WriteCString("ICC 25H Raid")
	updBuf.WriteCString("Updated notes")
	updBuf.WriteU8(1)
	updBuf.WriteU8(0)
	updBuf.WriteU32(25)
	updBuf.WriteI32(631)
	updBuf.WriteU32(uint32(time.Now().Unix()))
	updBuf.WriteU32(0)
	updBuf.WriteU32(0)

	if !sess.handleCalendarUpdateEvent(ctx, updBuf.Bytes()) {
		t.Fatal("handleCalendarUpdateEvent failed")
	}

	_ = db.QueryRow("SELECT title FROM calendar_events WHERE id = 1").Scan(&title)
	if title != "ICC 25H Raid" {
		t.Fatalf("expected 'ICC 25H Raid', got '%s'", title)
	}

	// 4. Remove event
	remBuf := protocol.NewBuffer(8)
	remBuf.WriteU64(1)
	if !sess.handleCalendarRemoveEvent(ctx, remBuf.Bytes()) {
		t.Fatal("handleCalendarRemoveEvent failed")
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM calendar_events WHERE id = 1").Scan(&count)
	if count != 0 {
		t.Fatalf("expected event deleted, count=%d", count)
	}
}


