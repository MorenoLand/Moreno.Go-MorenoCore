package world

import (
	"context"
	"testing"

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

	// 3. Set Raid Difficulty
	rBuf := protocol.NewBuffer(4)
	rBuf.WriteU32(2) // 10 Heroic
	if !sess.handleSetRaidDifficulty(ctx, rBuf.Bytes()) {
		t.Fatal("handleSetRaidDifficulty failed")
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
