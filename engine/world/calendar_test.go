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
		`CREATE TABLE characters (
			guid INTEGER PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			level INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE instance (
			id INTEGER PRIMARY KEY,
			map INTEGER NOT NULL DEFAULT 0,
			resettime INTEGER NOT NULL DEFAULT 0,
			difficulty INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE character_instance (
			guid INTEGER NOT NULL DEFAULT 0,
			instance INTEGER NOT NULL DEFAULT 0,
			permanent INTEGER NOT NULL DEFAULT 0,
			extendState INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (guid, instance)
		)`,
		`CREATE TABLE instance_reset (
			mapid INTEGER NOT NULL DEFAULT 0,
			difficulty INTEGER NOT NULL DEFAULT 0,
			resettime INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (mapid, difficulty)
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
	addBuf := protocol.NewBuffer(72)
	addBuf.WriteCString("ICC Raid")
	addBuf.WriteCString("Bring flasks")
	addBuf.WriteU8(1)                          // eventType
	addBuf.WriteU8(0)                          // repeatable
	addBuf.WriteU32(25)                        // maxInvites
	addBuf.WriteI32(631)                       // dungeonID
	addBuf.WriteU32(uint32(time.Now().Unix())) // eventPackedTime
	addBuf.WriteU32(0)                         // lockDate
	addBuf.WriteU32(0)                         // flags
	addBuf.WriteU32(0)                         // invite count

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

	// 3. Update event — format per TrinityCore CalendarUpdateEvent::Read:
	// eventID(u64), moderatorID(u64), title, description, type(u8), repeatable(u8),
	// maxInvites(u32), textureID/dungeonID(i32), time(u32), lockDate(u32), flags(u32)
	updBuf := protocol.NewBuffer(72)
	updBuf.WriteU64(1) // eventID
	updBuf.WriteU64(0) // moderatorID
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

	// 4. Remove event — format per TrinityCore CalendarRemoveEvent::Read:
	// eventID(u64), moderatorID(u64), isSignUp(u8)
	remBuf := protocol.NewBuffer(17)
	remBuf.WriteU64(1) // eventID
	remBuf.WriteU64(0) // moderatorID
	remBuf.WriteU8(0)  // isSignUp
	if !sess.handleCalendarRemoveEvent(ctx, remBuf.Bytes()) {
		t.Fatal("handleCalendarRemoveEvent failed")
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM calendar_events WHERE id = 1").Scan(&count)
	if count != 0 {
		t.Fatalf("expected event deleted, count=%d", count)
	}
}

func TestCalendarGetEventWithInvitees(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE calendar_events (
			id INTEGER PRIMARY KEY, creator INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
			type INTEGER NOT NULL DEFAULT 4, dungeon INTEGER NOT NULL DEFAULT -1,
			eventtime INTEGER NOT NULL DEFAULT 0, flags INTEGER NOT NULL DEFAULT 0,
			time2 INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE calendar_invites (
			id INTEGER PRIMARY KEY, event INTEGER NOT NULL DEFAULT 0,
			invitee INTEGER NOT NULL DEFAULT 0, sender INTEGER NOT NULL DEFAULT 0,
			status INTEGER NOT NULL DEFAULT 0, statustime INTEGER NOT NULL DEFAULT 0,
			rank INTEGER NOT NULL DEFAULT 0, text TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', level INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE instance (id INTEGER PRIMARY KEY, map INTEGER NOT NULL DEFAULT 0, resettime INTEGER NOT NULL DEFAULT 0, difficulty INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE character_instance (guid INTEGER NOT NULL DEFAULT 0, instance INTEGER NOT NULL DEFAULT 0, permanent INTEGER NOT NULL DEFAULT 0, extendState INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (guid, instance))`,
		`CREATE TABLE instance_reset (mapid INTEGER NOT NULL DEFAULT 0, difficulty INTEGER NOT NULL DEFAULT 0, resettime INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (mapid, difficulty))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	// Seed data
	_, _ = db.Exec("INSERT INTO calendar_events (id, creator, title, description, type, dungeon, flags, eventtime, time2) VALUES (99, 1, 'Big Raid', 'Good luck', 1, 631, 0, 1000, 0)")
	_, _ = db.Exec("INSERT INTO calendar_invites (id, event, invitee, sender, status, statustime, rank, text) VALUES (1, 99, 1, 1, 1, 1000, 2, 'ready')")
	_, _ = db.Exec("INSERT INTO characters (guid, name, level) VALUES (1, 'Hero', 80)")

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, Config: config.Default()}
	sess := &session{server: srv, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	getEventBuf := protocol.NewBuffer(8)
	getEventBuf.WriteU64(99)
	if !sess.handleCalendarGetEvent(ctx, getEventBuf.Bytes()) {
		t.Fatal("handleCalendarGetEvent failed")
	}

	// Test get calendar also includes this event
	if !sess.handleCalendarGetCalendar(ctx, nil) {
		t.Fatal("handleCalendarGetCalendar failed")
	}
}

func TestCalendarSendCalendarPacketStructure(t *testing.T) {
	// Verifies SMSG_CALENDAR_SEND_CALENDAR wire format matches
	// TrinityCore CalendarSendCalendar::Write exactly:
	//   u32(invites.size) + [invites...] + u32(events.size) + [events...] +
	//   u32(serverNow) + packedTime(serverTime) +
	//   u32(lockouts.size) + [lockouts...] +
	//   u32(raidOrigin=1135753200) +
	//   u32(raidResets.size) + [raidResets...] +
	//   u32(holidays.size=0)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE calendar_events (id INTEGER PRIMARY KEY, creator INTEGER NOT NULL DEFAULT 0, title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', type INTEGER NOT NULL DEFAULT 4, dungeon INTEGER NOT NULL DEFAULT -1, eventtime INTEGER NOT NULL DEFAULT 0, flags INTEGER NOT NULL DEFAULT 0, time2 INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE calendar_invites (id INTEGER PRIMARY KEY, event INTEGER NOT NULL DEFAULT 0, invitee INTEGER NOT NULL DEFAULT 0, sender INTEGER NOT NULL DEFAULT 0, status INTEGER NOT NULL DEFAULT 0, statustime INTEGER NOT NULL DEFAULT 0, rank INTEGER NOT NULL DEFAULT 0, text TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', level INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE instance (id INTEGER PRIMARY KEY, map INTEGER NOT NULL DEFAULT 0, resettime INTEGER NOT NULL DEFAULT 0, difficulty INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE character_instance (guid INTEGER NOT NULL DEFAULT 0, instance INTEGER NOT NULL DEFAULT 0, permanent INTEGER NOT NULL DEFAULT 0, extendState INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (guid, instance))`,
		`CREATE TABLE instance_reset (mapid INTEGER NOT NULL DEFAULT 0, difficulty INTEGER NOT NULL DEFAULT 0, resettime INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (mapid, difficulty))`,
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

	var received []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		op, payload, err := readServerFrame(clientConn, nil)
		if err != nil {
			return
		}
		if op == uint16(protocol.OpcodeSMSG_CALENDAR_SEND_CALENDAR) {
			received = payload
		}
	}()

	if !sess.handleCalendarGetCalendar(ctx, nil) {
		t.Fatal("handleCalendarGetCalendar failed")
	}

	serverConn.Close()
	<-done

	if len(received) < 4 {
		t.Fatalf("packet too short: %d bytes", len(received))
	}
	r := protocol.NewReader(received)

	// invites count = 0
	invCount, _ := r.ReadU32()
	if invCount != 0 {
		t.Fatalf("expected 0 invites, got %d", invCount)
	}
	// events count = 0
	evCount, _ := r.ReadU32()
	if evCount != 0 {
		t.Fatalf("expected 0 events, got %d", evCount)
	}
	// serverNow (u32)
	_, _ = r.ReadU32()
	// serverTime (packed u32)
	_, _ = r.ReadU32()
	// lockouts count = 0
	lockCount, _ := r.ReadU32()
	if lockCount != 0 {
		t.Fatalf("expected 0 lockouts, got %d", lockCount)
	}
	// raidOrigin constant
	raidOrigin, _ := r.ReadU32()
	if raidOrigin != 1135753200 {
		t.Fatalf("expected raidOrigin=1135753200, got %d", raidOrigin)
	}
	// raidResets count = 0
	raidResetCount, _ := r.ReadU32()
	if raidResetCount != 0 {
		t.Fatalf("expected 0 raidResets, got %d", raidResetCount)
	}
	// holidays count = 0
	holidayCount, _ := r.ReadU32()
	if holidayCount != 0 {
		t.Fatalf("expected 0 holidays, got %d", holidayCount)
	}
}
