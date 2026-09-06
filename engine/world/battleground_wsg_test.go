package world

import (
	"context"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestWSG_FlagPickupAndCapture(t *testing.T) {
	srv := &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		hiddenGameObjects:  make(map[uint64]struct{}),
		wsgState:           make(map[uint32]*wsgBattlegroundState),
	}

	hordePlayerGUID := uint64(100)
	hordeSess := &session{
		server:       srv,
		playerGUID:   hordePlayerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID:  hordePlayerGUID,
			Name:  "Thrall",
			Race:  2, // Orc -> Team 1 (Horde)
			Map:   WSGMapID,
			X:     1540.0,
			Y:     1480.0,
			Z:     350.0,
		},
	}
	srv.sessions[hordeSess] = struct{}{}

	allianceFlagGUID := gameObjectGUID(1, WSGAllianceFlagBaseEntry)
	hordeFlagGUID := gameObjectGUID(2, WSGHordeFlagBaseEntry)

	// Pre-populate dynamic states
	srv.dynamicGameObjects[allianceFlagGUID] = &dynamicGameObjectState{
		GUID:    allianceFlagGUID,
		LowGUID: 1,
		Entry:   WSGAllianceFlagBaseEntry,
		Map:     WSGMapID,
		X:       1540.0,
		Y:       1480.0,
		Z:       350.0,
		Type:    GameObjectTypeFlagStand,
		State:   GameObjectStateReady,
	}
	srv.dynamicGameObjects[hordeFlagGUID] = &dynamicGameObjectState{
		GUID:    hordeFlagGUID,
		LowGUID: 2,
		Entry:   WSGHordeFlagBaseEntry,
		Map:     WSGMapID,
		X:       916.0,
		Y:       1430.0,
		Z:       345.0,
		Type:    GameObjectTypeFlagStand,
		State:   GameObjectStateReady,
	}

	ctx := context.Background()

	// 1. Horde player clicks Alliance flag base -> Pickup
	usePayload := protocol.NewBuffer(8)
	usePayload.WriteU64(allianceFlagGUID)
	if !hordeSess.handleGameObjectUse(ctx, usePayload.Bytes()) {
		t.Fatal("handleGameObjectUse failed on flag pickup")
	}

	wsg := srv.getOrCreateWSGState(WSGMapID)
	if wsg.AllianceFlagState != WSGFlagStateOnPlayer {
		t.Fatalf("expected Alliance flag state to be on player (2), got %d", wsg.AllianceFlagState)
	}
	if wsg.AllianceCarrierGUID != hordePlayerGUID {
		t.Fatalf("expected carrier to be %d, got %d", hordePlayerGUID, wsg.AllianceCarrierGUID)
	}
	if !hordeSess.hasAura(WSGSpellSilverwingFlag) {
		t.Fatalf("expected horde player to have Silverwing Flag aura (%d)", WSGSpellSilverwingFlag)
	}
	if !srv.isGameObjectHidden(allianceFlagGUID) {
		t.Fatal("expected Alliance base flag to be hidden")
	}

	// 2. Horde player runs to Horde base flag and clicks it -> Capture!
	hordeSess.player.X = 916.0
	hordeSess.player.Y = 1430.0
	hordeSess.player.Z = 345.0

	capturePayload := protocol.NewBuffer(8)
	capturePayload.WriteU64(hordeFlagGUID)
	if !hordeSess.handleGameObjectUse(ctx, capturePayload.Bytes()) {
		t.Fatal("handleGameObjectUse failed on flag capture")
	}

	if wsg.HordeScore != 1 {
		t.Fatalf("expected Horde score to be 1, got %d", wsg.HordeScore)
	}
	if wsg.AllianceFlagState != WSGFlagStateOnBase {
		t.Fatalf("expected Alliance flag state to reset to base (1), got %d", wsg.AllianceFlagState)
	}
	if wsg.AllianceCarrierGUID != 0 {
		t.Fatalf("expected Alliance carrier to be 0, got %d", wsg.AllianceCarrierGUID)
	}
	if hordeSess.hasAura(WSGSpellSilverwingFlag) {
		t.Fatalf("expected Silverwing Flag aura to be removed after capture")
	}
	if srv.isGameObjectHidden(allianceFlagGUID) {
		t.Fatal("expected Alliance base flag to be unhidden after capture")
	}
}

func TestWSG_FlagDropOnDeathAndReturn(t *testing.T) {
	srv := &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		hiddenGameObjects:  make(map[uint64]struct{}),
		wsgState:           make(map[uint32]*wsgBattlegroundState),
	}

	allyPlayerGUID := uint64(200)
	allySess := &session{
		server:       srv,
		playerGUID:   allyPlayerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID:   allyPlayerGUID,
			Name:   "Anduin",
			Race:   1, // Human -> Team 0 (Alliance)
			Map:    WSGMapID,
			X:      916.0,
			Y:      1430.0,
			Z:      345.0,
			Health: 5000,
		},
	}
	srv.sessions[allySess] = struct{}{}

	hordeFlagGUID := gameObjectGUID(2, WSGHordeFlagBaseEntry)
	srv.dynamicGameObjects[hordeFlagGUID] = &dynamicGameObjectState{
		GUID:    hordeFlagGUID,
		LowGUID: 2,
		Entry:   WSGHordeFlagBaseEntry,
		Map:     WSGMapID,
		X:       916.0,
		Y:       1430.0,
		Z:       345.0,
		Type:    GameObjectTypeFlagStand,
		State:   GameObjectStateReady,
	}

	ctx := context.Background()

	// 1. Alliance player picks up Horde flag
	pickupPayload := protocol.NewBuffer(8)
	pickupPayload.WriteU64(hordeFlagGUID)
	allySess.handleGameObjectUse(ctx, pickupPayload.Bytes())

	wsg := srv.getOrCreateWSGState(WSGMapID)
	if wsg.HordeFlagState != WSGFlagStateOnPlayer || wsg.HordeCarrierGUID != allyPlayerGUID {
		t.Fatalf("expected Horde flag picked up by ally: state=%d, carrier=%d", wsg.HordeFlagState, wsg.HordeCarrierGUID)
	}

	// 2. Alliance player dies
	allySess.player.Health = 0
	allySess.killPlayer(ctx)

	if wsg.HordeFlagState != WSGFlagStateOnGround {
		t.Fatalf("expected Horde flag on ground (3), got %d", wsg.HordeFlagState)
	}
	if wsg.HordeCarrierGUID != 0 {
		t.Fatalf("expected Horde carrier to be cleared, got %d", wsg.HordeCarrierGUID)
	}
	if wsg.HordeDroppedGUID == 0 {
		t.Fatal("expected dropped flag GUID to be created")
	}

	// Verify dropped flag dynamic gameobject was registered
	dropped := srv.dynamicGameObjects[wsg.HordeDroppedGUID]
	if dropped == nil || dropped.Entry != WSGHordeFlagDroppedEntry {
		t.Fatalf("expected dropped gameobject state for %d", wsg.HordeDroppedGUID)
	}

	// 3. Horde player clicks dropped flag -> Return!
	hordePlayerGUID := uint64(300)
	hordeSess := &session{
		server:       srv,
		playerGUID:   hordePlayerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID: hordePlayerGUID,
			Name: "Garrosh",
			Race: 2, // Orc -> Team 1 (Horde)
			Map:  WSGMapID,
			X:    allySess.player.X,
			Y:    allySess.player.Y,
			Z:    allySess.player.Z,
		},
	}
	srv.sessions[hordeSess] = struct{}{}

	returnPayload := protocol.NewBuffer(8)
	returnPayload.WriteU64(wsg.HordeDroppedGUID)
	hordeSess.handleGameObjectUse(ctx, returnPayload.Bytes())

	if wsg.HordeFlagState != WSGFlagStateOnBase {
		t.Fatalf("expected Horde flag returned to base (1), got %d", wsg.HordeFlagState)
	}
	if srv.dynamicGameObjects[wsg.HordeDroppedGUID] != nil {
		t.Fatal("expected dropped flag gameobject to be removed after return")
	}
	if srv.isGameObjectHidden(hordeFlagGUID) {
		t.Fatal("expected Horde base flag to be unhidden after return")
	}
}

func TestWSG_BattlegroundPlayerPositionsWithCarriers(t *testing.T) {
	srv := &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		hiddenGameObjects:  make(map[uint64]struct{}),
		wsgState:           make(map[uint32]*wsgBattlegroundState),
	}

	// Horde flag carrier
	hordeCarrierGUID := uint64(101)
	hordeCarrier := &session{
		server:       srv,
		playerGUID:   hordeCarrierGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID: hordeCarrierGUID,
			Name: "OrcCarrier",
			Race: 2,
			Map:  WSGMapID,
			X:    1200.0,
			Y:    1300.0,
		},
	}
	srv.sessions[hordeCarrier] = struct{}{}

	// Alliance teammate viewer
	allyViewerGUID := uint64(201)
	allyViewer := &session{
		server:       srv,
		playerGUID:   allyViewerGUID,
		playerLoaded: true,
		activeAuras:  make(map[uint32]*activeAura),
		player: &playerState{
			GUID: allyViewerGUID,
			Name: "HumanWatcher",
			Race: 1,
			Map:  WSGMapID,
			X:    1500.0,
			Y:    1400.0,
		},
	}
	srv.sessions[allyViewer] = struct{}{}

	wsg := srv.getOrCreateWSGState(WSGMapID)
	wsg.AllianceFlagState = WSGFlagStateOnPlayer
	wsg.AllianceCarrierGUID = hordeCarrierGUID

	carriers := srv.getWSGFlagCarriers(WSGMapID)
	if len(carriers) != 1 || carriers[0].playerGUID != hordeCarrierGUID {
		t.Fatalf("expected 1 flag carrier with GUID %d, got %+v", hordeCarrierGUID, carriers)
	}

	// Verify handleBattlegroundPlayerPositions
	if !allyViewer.handleBattlegroundPlayerPositions(context.Background(), nil) {
		t.Fatal("handleBattlegroundPlayerPositions failed")
	}
}

func TestGameObject_DoorAndButtonInteraction(t *testing.T) {
	srv := &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		hiddenGameObjects:  make(map[uint64]struct{}),
	}

	sess := &session{
		server:       srv,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID: 1,
			Map:  0,
			X:    10.0,
			Y:    10.0,
			Z:    0.0,
		},
	}
	srv.sessions[sess] = struct{}{}

	// 1. Door (Type 0, starts Closed/Ready = 1)
	doorGUID := gameObjectGUID(10, 500)
	door := &dynamicGameObjectState{
		GUID:    doorGUID,
		LowGUID: 10,
		Entry:   500,
		Map:     0,
		X:       10.0,
		Y:       10.0,
		Z:       0.0,
		Type:    GameObjectTypeDoor,
		State:   GameObjectStateReady,
	}
	srv.dynamicGameObjects[doorGUID] = door

	ctx := context.Background()
	doorPayload := protocol.NewBuffer(8)
	doorPayload.WriteU64(doorGUID)

	// Click door -> Opens (State = 0)
	sess.handleGameObjectUse(ctx, doorPayload.Bytes())
	if door.State != GameObjectStateActive {
		t.Fatalf("expected door state to be Active/Open (0), got %d", door.State)
	}

	// Click door again -> Closes (State = 1)
	sess.handleGameObjectUse(ctx, doorPayload.Bytes())
	if door.State != GameObjectStateReady {
		t.Fatalf("expected door state to be Ready/Closed (1), got %d", door.State)
	}

	// 2. Button (Type 1, starts Ready = 1)
	buttonGUID := gameObjectGUID(20, 600)
	button := &dynamicGameObjectState{
		GUID:    buttonGUID,
		LowGUID: 20,
		Entry:   600,
		Map:     0,
		X:       10.0,
		Y:       10.0,
		Z:       0.0,
		Type:    GameObjectTypeButton,
		State:   GameObjectStateReady,
	}
	srv.dynamicGameObjects[buttonGUID] = button

	btnPayload := protocol.NewBuffer(8)
	btnPayload.WriteU64(buttonGUID)
	sess.handleGameObjectUse(ctx, btnPayload.Bytes())
	if button.State != GameObjectStateActive {
		t.Fatalf("expected button state to be Active/Pressed (0), got %d", button.State)
	}

	// Test scheduleGameObjectReset
	srv.scheduleGameObjectReset(buttonGUID, 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	if button.State != GameObjectStateReady {
		t.Fatalf("expected button state to reset to Ready (1), got %d", button.State)
	}
}
