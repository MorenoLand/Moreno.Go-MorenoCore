package world

import (
	"context"
	"database/sql"
	"testing"

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
