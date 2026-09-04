package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestInstancesAndDifficulty(t *testing.T) {
	srv := &Server{}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	ctx := context.Background()

	// 1. Reset Instances
	if !sess.handleResetInstances(ctx, nil) {
		t.Fatal("handleResetInstances failed")
	}

	// 2. Set Dungeon Difficulty
	dBuf := protocol.NewBuffer(4)
	dBuf.WriteU32(1) // Heroic
	if !sess.handleSetDungeonDifficulty(ctx, dBuf.Bytes()) {
		t.Fatal("handleSetDungeonDifficulty failed")
	}
	if sess.player.DungeonDifficulty != 1 {
		t.Fatalf("expected DungeonDifficulty=1, got %d", sess.player.DungeonDifficulty)
	}

	// 3. Set Raid Difficulty
	rBuf := protocol.NewBuffer(4)
	rBuf.WriteU32(2) // 10 Heroic
	if !sess.handleSetRaidDifficulty(ctx, rBuf.Bytes()) {
		t.Fatal("handleSetRaidDifficulty failed")
	}
	if sess.player.RaidDifficulty != 2 {
		t.Fatalf("expected RaidDifficulty=2, got %d", sess.player.RaidDifficulty)
	}

	// 4. Instance Lock Response
	if !sess.handleInstanceLockResponse(ctx, nil) {
		t.Fatal("handleInstanceLockResponse failed")
	}

	// 5. Saved Instance Extend
	if !sess.handleSetSavedInstanceExtend(ctx, nil) {
		t.Fatal("handleSetSavedInstanceExtend failed")
	}
}

func TestPlayerDisplayAndTitles(t *testing.T) {
	srv := &Server{}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	ctx := context.Background()

	// Cloak toggle
	if !sess.handleShowingCloak(ctx, []byte{0}) {
		t.Fatal("handleShowingCloak failed")
	}
	if sess.player.PlayerFlags&0x800 == 0 {
		t.Fatal("expected cloak to be hidden")
	}
	if !sess.handleShowingCloak(ctx, []byte{1}) {
		t.Fatal("handleShowingCloak failed")
	}
	if sess.player.PlayerFlags&0x800 != 0 {
		t.Fatal("expected cloak to be shown")
	}

	// Helm toggle
	if !sess.handleShowingHelm(ctx, []byte{0}) {
		t.Fatal("handleShowingHelm failed")
	}
	if sess.player.PlayerFlags&0x400 == 0 {
		t.Fatal("expected helm to be hidden")
	}

	// Set Title
	tBuf := protocol.NewBuffer(4)
	tBuf.WriteI32(42)
	if !sess.handleSetTitle(ctx, tBuf.Bytes()) {
		t.Fatal("handleSetTitle failed")
	}
	if sess.player.ChosenTitle != 42 {
		t.Fatalf("expected title 42, got %d", sess.player.ChosenTitle)
	}

	// Toggle PvP
	if !sess.handleTogglePvP(ctx, nil) {
		t.Fatal("handleTogglePvP failed")
	}
	if sess.player.PlayerFlags&0x02 == 0 {
		t.Fatal("expected player to be in PvP")
	}
}

func TestVehiclesAndSeats(t *testing.T) {
	srv := &Server{}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	ctx := context.Background()

	if !sess.handlePlayerVehicleEnter(ctx, nil) ||
		!sess.handleRequestVehicleExit(ctx, nil) ||
		!sess.handleRequestVehicleNextSeat(ctx, nil) ||
		!sess.handleRequestVehiclePrevSeat(ctx, nil) ||
		!sess.handleRequestVehicleSwitchSeat(ctx, nil) {
		t.Fatal("vehicle handlers returned false")
	}
}

func TestItemsQuestsAndSpells(t *testing.T) {
	srv := &Server{}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	ctx := context.Background()

	// Items
	if !sess.handleOpenItem(ctx, nil) ||
		!sess.handleWrapItem(ctx, nil) ||
		!sess.handleRepairItem(ctx, nil) ||
		!sess.handleSocketGems(ctx, nil) {
		t.Fatal("item action handlers returned false")
	}

	ammoBuf := protocol.NewBuffer(4)
	ammoBuf.WriteU32(2512)
	if !sess.handleSetAmmo(ctx, ammoBuf.Bytes()) {
		t.Fatal("handleSetAmmo failed")
	}
	if sess.player.AmmoID != 2512 {
		t.Fatalf("expected ammoID 2512, got %d", sess.player.AmmoID)
	}

	// Movement ACKs
	if !sess.handleMoveFeatherFallAck(ctx, nil) ||
		!sess.handleMoveHoverAck(ctx, nil) ||
		!sess.handleMoveWaterWalkAck(ctx, nil) ||
		!sess.handleMoveKnockBackAck(ctx, nil) ||
		!sess.handleMoveNotActiveMover(ctx, nil) ||
		!sess.handleMoveFallReset(ctx, nil) ||
		!sess.handleMoveSplineDone(ctx, nil) ||
		!sess.handleMoveChngTransport(ctx, nil) ||
		!sess.handleMoveSetFly(ctx, nil) ||
		!sess.handleSummonResponse(ctx, nil) ||
		!sess.handleMountSpecialAnim(ctx, nil) {
		t.Fatal("movement ack handlers returned false")
	}

	// Spells & Talents
	if !sess.handleTotemDestroyed(ctx, nil) ||
		!sess.handleSpellClick(ctx, nil) ||
		!sess.handleTalentWipeConfirm(ctx, nil) {
		t.Fatal("spell handlers returned false")
	}

	// Quests & Inspect
	if !sess.handleQuestConfirmAccept(ctx, nil) ||
		!sess.handleQuestPoiQuery(ctx, nil) ||
		!sess.handleQueryQuestsCompleted(ctx, nil) ||
		!sess.handleQuestlogSwapQuest(ctx, nil) ||
		!sess.handlePushQuestToParty(ctx, nil) ||
		!sess.handleQuestPushResult(ctx, nil) ||
		!sess.handleQuestgiverStatusMultipleQuery(ctx, nil) ||
		!sess.handleQueryInspectAchievements(ctx, nil) ||
		!sess.handleRaidReadyCheckFinished(ctx, nil) {
		t.Fatal("quest handlers returned false")
	}

	// Guild Permissions, Event Log & Inspect
	if !sess.handleGuildEventLogQuery(ctx, nil) ||
		!sess.handleGuildPermissions(ctx, nil) ||
		!sess.handleInspectArenaTeams(ctx, nil) ||
		!sess.handleInspectHonorStats(ctx, nil) ||
		!sess.handlePvpLogData(ctx, nil) {
		t.Fatal("guild query handlers returned false")
	}

	// Spirit Healer & Corpse
	if !sess.handleSpiritHealerActivate(ctx, nil) ||
		!sess.handleCorpseQuery(ctx, nil) {
		t.Fatal("death query handlers returned false")
	}
}

func TestTitlePersistenceAndKnownTitles(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE characters (
		guid INTEGER PRIMARY KEY, account INTEGER, xp INTEGER, money INTEGER, health INTEGER,
		power1 INTEGER, power2 INTEGER, power3 INTEGER, power4 INTEGER, power5 INTEGER, power6 INTEGER, power7 INTEGER,
		cinematic INTEGER, knownCurrencies INTEGER, watchedFaction INTEGER, ammoId INTEGER, actionBars INTEGER,
		chosenTitle INTEGER, knownTitles TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO characters (guid, account, xp, money, health, power1, power2, power3, power4, power5, power6, power7, cinematic, knownCurrencies, watchedFaction, ammoId, actionBars, chosenTitle, knownTitles) VALUES (1, 1, 0, 0, 100, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 15, '1 2 4 8 16 32')")
	if err != nil {
		t.Fatal(err)
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1}}
	ctx := context.Background()

	sess.accountID = 1
	// 1. Test loadOptionalPlayerState loads chosenTitle and knownTitles
	state := playerState{GUID: 1}
	if err := sess.loadOptionalPlayerState(ctx, &state); err != nil {
		t.Fatal(err)
	}
	if state.ChosenTitle != 15 {
		t.Fatalf("expected chosenTitle=15, got %d", state.ChosenTitle)
	}
	if state.KnownTitles[0] != 1 || state.KnownTitles[1] != 2 || state.KnownTitles[5] != 32 {
		t.Fatalf("unexpected knownTitles: %+v", state.KnownTitles)
	}

	// 2. Test handleSetTitle updates DB
	tBuf := protocol.NewBuffer(4)
	tBuf.WriteI32(28)
	if !sess.handleSetTitle(ctx, tBuf.Bytes()) {
		t.Fatal("handleSetTitle failed")
	}
	var dbChosen int
	_ = db.QueryRow("SELECT chosenTitle FROM characters WHERE guid = 1").Scan(&dbChosen)
	if dbChosen != 28 {
		t.Fatalf("expected db chosenTitle=28, got %d", dbChosen)
	}
}

func TestResetInstances_SoloOutside_Success(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE character_instance (guid INTEGER, instance INTEGER, permanent INTEGER, extendState INTEGER, PRIMARY KEY (guid, instance))",
		"INSERT INTO character_instance VALUES (1, 10, 0, 0)", // non-permanent
		"INSERT INTO character_instance VALUES (1, 11, 1, 0)", // permanent
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Map: 0}, // Outside dungeon (Eastern Kingdoms)
	}

	done := make(chan uint16, 1)
	go func() {
		op, _, _ := readServerFrame(clientConn, nil)
		done <- op
	}()

	if !sess.handleResetInstances(context.Background(), nil) {
		t.Fatal("handleResetInstances failed")
	}

	select {
	case op := <-done:
		if op != uint16(protocol.OpcodeSMSG_INSTANCE_RESET) {
			t.Fatalf("expected SMSG_INSTANCE_RESET, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for SMSG_INSTANCE_RESET")
	}

	// Verify non-permanent bind removed, permanent bind preserved
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM character_instance WHERE guid = 1 AND instance = 10").Scan(&count)
	if count != 0 {
		t.Fatalf("expected non-perm instance 10 deleted, got count=%d", count)
	}
	_ = db.QueryRow("SELECT COUNT(*) FROM character_instance WHERE guid = 1 AND instance = 11").Scan(&count)
	if count != 1 {
		t.Fatalf("expected perm instance 11 preserved, got count=%d", count)
	}
}

func TestResetInstances_SoloInside_Fails(t *testing.T) {
	srv := &Server{}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Map: 33}, // Shadowfang Keep (dungeon)
	}

	ops := make(chan uint16, 2)
	go func() {
		for i := 0; i < 2; i++ {
			_ = clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			ops <- op
		}
	}()

	if !sess.handleResetInstances(context.Background(), nil) {
		t.Fatal("handleResetInstances failed")
	}

	time.Sleep(50 * time.Millisecond)
	var gotFailed, gotNotify bool
	for len(ops) > 0 {
		op := <-ops
		if op == uint16(protocol.OpcodeSMSG_INSTANCE_RESET_FAILED) {
			gotFailed = true
		}
		if op == uint16(protocol.OpcodeSMSG_RESET_FAILED_NOTIFY) {
			gotNotify = true
		}
	}
	if !gotFailed {
		t.Fatal("expected SMSG_INSTANCE_RESET_FAILED")
	}
	if !gotNotify {
		t.Fatal("expected SMSG_RESET_FAILED_NOTIFY")
	}
}

func TestResetInstances_GroupMemberInside_Fails(t *testing.T) {
	srv := &Server{
		groups:   make(map[uint64]*groupState),
		sessions: make(map[*session]struct{}),
	}
	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerGUID:   1,
		groupID:      100,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Map: 0}, // Outside
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerGUID:   2,
		groupID:      100,
		playerLoaded: true,
		player:       &playerState{GUID: 2, Map: 43}, // Inside Wailing Caverns
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	srv.groups[100] = &groupState{
		ID:         100,
		LeaderGUID: 1,
		Members:    []groupMember{{GUID: 1}, {GUID: 2}},
	}

	ops1 := make(chan uint16, 2)
	go func() {
		_ = cConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		op, _, err := readServerFrame(cConn1, nil)
		if err == nil {
			ops1 <- op
		}
	}()

	ops2 := make(chan uint16, 2)
	go func() {
		_ = cConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		op, _, err := readServerFrame(cConn2, nil)
		if err == nil {
			ops2 <- op
		}
	}()

	if !sess1.handleResetInstances(context.Background(), nil) {
		t.Fatal("handleResetInstances failed")
	}

	time.Sleep(50 * time.Millisecond)
	select {
	case op := <-ops1:
		if op != uint16(protocol.OpcodeSMSG_INSTANCE_RESET_FAILED) {
			t.Fatalf("expected SMSG_INSTANCE_RESET_FAILED for leader, got 0x%04X", op)
		}
	default:
		t.Fatal("leader did not receive SMSG_INSTANCE_RESET_FAILED")
	}

	select {
	case op := <-ops2:
		if op != uint16(protocol.OpcodeSMSG_RESET_FAILED_NOTIFY) {
			t.Fatalf("expected SMSG_RESET_FAILED_NOTIFY for member inside, got 0x%04X", op)
		}
	default:
		t.Fatal("member inside did not receive SMSG_RESET_FAILED_NOTIFY")
	}
}

func TestResetInstances_GroupAllOutside_Success(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE character_instance (guid INTEGER, instance INTEGER, permanent INTEGER, extendState INTEGER, PRIMARY KEY (guid, instance))",
		"INSERT INTO character_instance VALUES (1, 10, 0, 0)",
		"INSERT INTO character_instance VALUES (2, 10, 0, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		CharactersStore: charStore,
		groups:          make(map[uint64]*groupState),
		sessions:        make(map[*session]struct{}),
	}
	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerGUID:   1,
		groupID:      100,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Map: 0},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerGUID:   2,
		groupID:      100,
		playerLoaded: true,
		player:       &playerState{GUID: 2, Map: 1},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}
	srv.groups[100] = &groupState{
		ID:         100,
		LeaderGUID: 1,
		Members:    []groupMember{{GUID: 1}, {GUID: 2}},
	}

	ops1 := make(chan uint16, 2)
	go func() {
		_ = cConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		op, _, err := readServerFrame(cConn1, nil)
		if err == nil {
			ops1 <- op
		}
	}()

	ops2 := make(chan uint16, 2)
	go func() {
		_ = cConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		op, _, err := readServerFrame(cConn2, nil)
		if err == nil {
			ops2 <- op
		}
	}()

	if !sess1.handleResetInstances(context.Background(), nil) {
		t.Fatal("handleResetInstances failed")
	}

	time.Sleep(50 * time.Millisecond)
	select {
	case op := <-ops1:
		if op != uint16(protocol.OpcodeSMSG_INSTANCE_RESET) {
			t.Fatalf("expected SMSG_INSTANCE_RESET for sess1, got 0x%04X", op)
		}
	default:
		t.Fatal("sess1 did not receive SMSG_INSTANCE_RESET")
	}

	select {
	case op := <-ops2:
		if op != uint16(protocol.OpcodeSMSG_INSTANCE_RESET) {
			t.Fatalf("expected SMSG_INSTANCE_RESET for sess2, got 0x%04X", op)
		}
	default:
		t.Fatal("sess2 did not receive SMSG_INSTANCE_RESET")
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM character_instance WHERE permanent = 0").Scan(&count)
	if count != 0 {
		t.Fatalf("expected all non-permanent bindings removed, got count=%d", count)
	}
}

func TestInstanceLockResponse_Accept(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE instance (id INTEGER PRIMARY KEY, map INTEGER, resettime INTEGER, difficulty INTEGER, completedEncounters INTEGER, data TEXT)",
		"CREATE TABLE character_instance (guid INTEGER, instance INTEGER, permanent INTEGER, extendState INTEGER, PRIMARY KEY (guid, instance))",
		"INSERT INTO instance VALUES (50, 33, 2000000000, 1, 0, '')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore}
	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Map: 33},
	}
	sess.setPendingBind(50, 33, 1, 60000)

	ops := make(chan uint16, 5)
	go func() {
		for {
			_ = cConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(cConn, nil)
			if err != nil {
				return
			}
			ops <- op
		}
	}()

	if !sess.handleInstanceLockResponse(context.Background(), []byte{1}) {
		t.Fatal("handleInstanceLockResponse failed")
	}

	time.Sleep(50 * time.Millisecond)
	var gotSaveCreated, gotLockoutAdded, gotRaidInfo bool
	for len(ops) > 0 {
		op := <-ops
		if op == uint16(protocol.OpcodeSMSG_INSTANCE_SAVE_CREATED) {
			gotSaveCreated = true
		}
		if op == uint16(protocol.OpcodeSMSG_CALENDAR_RAID_LOCKOUT_ADDED) {
			gotLockoutAdded = true
		}
		if op == uint16(protocol.OpcodeSMSG_RAID_INSTANCE_INFO) {
			gotRaidInfo = true
		}
	}

	if !gotSaveCreated {
		t.Fatal("expected SMSG_INSTANCE_SAVE_CREATED")
	}
	if !gotLockoutAdded {
		t.Fatal("expected SMSG_CALENDAR_RAID_LOCKOUT_ADDED")
	}
	if !gotRaidInfo {
		t.Fatal("expected SMSG_RAID_INSTANCE_INFO")
	}

	var perm int
	err = db.QueryRow("SELECT permanent FROM character_instance WHERE guid = 1 AND instance = 50").Scan(&perm)
	if err != nil || perm != 1 {
		t.Fatalf("expected permanent bind 1 in DB, got perm=%d err=%v", perm, err)
	}
	if sess.hasPendingBind() {
		t.Fatal("expected pending bind cleared")
	}
}

func TestInstanceLockResponse_Decline(t *testing.T) {
	srv := &Server{}
	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Map: 33},
	}
	sess.setPendingBind(50, 33, 1, 60000)

	if !sess.handleInstanceLockResponse(context.Background(), []byte{0}) {
		t.Fatal("handleInstanceLockResponse failed")
	}

	if sess.hasPendingBind() {
		t.Fatal("expected pending bind cleared on decline")
	}
}

func TestSetSavedInstanceExtend(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE instance (id INTEGER PRIMARY KEY, map INTEGER, resettime INTEGER, difficulty INTEGER, completedEncounters INTEGER, data TEXT)",
		"CREATE TABLE character_instance (guid INTEGER, instance INTEGER, permanent INTEGER, extendState INTEGER, PRIMARY KEY (guid, instance))",
		"INSERT INTO instance VALUES (70, 533, 2000000000, 0, 0, '')",
		"INSERT INTO character_instance VALUES (1, 70, 1, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore}
	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1},
	}

	ops := make(chan uint16, 5)
	go func() {
		for {
			_ = cConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			op, _, err := readServerFrame(cConn, nil)
			if err != nil {
				return
			}
			ops <- op
		}
	}()

	// 1. Toggle extend ON (1)
	p1 := protocol.NewBuffer(9)
	p1.WriteU32(533) // Map 533 (Naxxramas)
	p1.WriteU32(0)   // Difficulty 0
	p1.WriteU8(1)    // Toggle ON
	if !sess.handleSetSavedInstanceExtend(context.Background(), p1.Bytes()) {
		t.Fatal("handleSetSavedInstanceExtend failed")
	}

	time.Sleep(50 * time.Millisecond)
	var gotLockoutUpdated bool
	for len(ops) > 0 {
		op := <-ops
		if op == uint16(protocol.OpcodeSMSG_CALENDAR_RAID_LOCKOUT_UPDATED) {
			gotLockoutUpdated = true
		}
	}
	if !gotLockoutUpdated {
		t.Fatal("expected SMSG_CALENDAR_RAID_LOCKOUT_UPDATED")
	}

	var extendState int
	_ = db.QueryRow("SELECT extendState FROM character_instance WHERE guid = 1 AND instance = 70").Scan(&extendState)
	if extendState != 2 {
		t.Fatalf("expected extendState=2 (EXTEND_STATE_EXTENDED), got %d", extendState)
	}

	// 2. Toggle extend OFF (0)
	p2 := protocol.NewBuffer(9)
	p2.WriteU32(533)
	p2.WriteU32(0)
	p2.WriteU8(0)
	if !sess.handleSetSavedInstanceExtend(context.Background(), p2.Bytes()) {
		t.Fatal("handleSetSavedInstanceExtend failed")
	}

	_ = db.QueryRow("SELECT extendState FROM character_instance WHERE guid = 1 AND instance = 70").Scan(&extendState)
	if extendState != 0 {
		t.Fatalf("expected extendState=0 (EXTEND_STATE_NORMAL), got %d", extendState)
	}
}

