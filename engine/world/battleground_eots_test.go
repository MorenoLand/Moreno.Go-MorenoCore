package world

import (
	"context"
	"net"
	"testing"
	"time"
)

func newEOTSTestSession(t *testing.T, srv *Server, guid uint64, race uint8, name string, x, y, z float32) (*session, net.Conn) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})
	go func() {
		buf := make([]byte, 1024)
		for {
			if _, err := clientConn.Read(buf); err != nil {
				return
			}
		}
	}()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerGUID:   guid,
		playerLoaded: true,
		player: &playerState{
			GUID:  guid,
			Name:  name,
			Race:  race,
			Map:   EOTSMapID,
			X:     x,
			Y:     y,
			Z:     z,
			Level: 80,
		},
	}
	srv.sessionsMu.Lock()
	srv.sessions[sess] = struct{}{}
	srv.sessionsMu.Unlock()
	return sess, clientConn
}

func setupEOTSServer() *Server {
	return &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		eotsState:          make(map[uint32]*eotsBattlegroundState),
	}
}

func TestEOTSGameObjectMapping(t *testing.T) {
	// 1. Center flag and dropped flag
	if !isEOTSGameObject(EOTSFlagCenterEntry) {
		t.Fatal("expected center flag to be recognized")
	}
	if !isEOTSGameObject(EOTSFlagDroppedEntry) {
		t.Fatal("expected dropped flag to be recognized")
	}

	// 2. Tower banners
	for tower := uint32(0); tower < EOTSTowerMax; tower++ {
		neutralEntry := getEOTSBannerEntry(tower, EOTSTowerStateNeutral)
		if !isEOTSGameObject(neutralEntry) {
			t.Fatalf("expected neutral entry %d recognized", neutralEntry)
		}
		tID, ok := getEOTSTowerIDFromBannerEntry(neutralEntry)
		if !ok || tID != tower {
			t.Fatalf("expected towerID %d, got %d", tower, tID)
		}

		allyEntry := getEOTSBannerEntry(tower, EOTSTowerStateControlledAlliance)
		if !isEOTSGameObject(allyEntry) {
			t.Fatalf("expected ally entry %d recognized", allyEntry)
		}
		tID, ok = getEOTSTowerIDFromBannerEntry(allyEntry)
		if !ok || tID != tower {
			t.Fatalf("expected towerID %d, got %d", tower, tID)
		}

		hordeEntry := getEOTSBannerEntry(tower, EOTSTowerStateControlledHorde)
		if !isEOTSGameObject(hordeEntry) {
			t.Fatalf("expected horde entry %d recognized", hordeEntry)
		}
		tID, ok = getEOTSTowerIDFromBannerEntry(hordeEntry)
		if !ok || tID != tower {
			t.Fatalf("expected towerID %d, got %d", tower, tID)
		}
	}

	if isEOTSGameObject(99999) {
		t.Fatal("unexpected true for entry 99999")
	}
}

func TestEOTSTowerCaptureAndBases(t *testing.T) {
	srv := setupEOTSServer()
	eots := srv.getOrCreateEOTSState(EOTSMapID)
	ctx := context.Background()

	// Alliance hero at Mage Tower (2228.4, 1330.4, 1199.0)
	allySess, _ := newEOTSTestSession(t, srv, 10, 1, "AllianceHero", 2228.4, 1330.4, 1199.0)
	// Horde hero at Mage Tower
	hordeSess, _ := newEOTSTestSession(t, srv, 20, 2, "HordeHero", 2228.4, 1330.4, 1199.0)

	mageNeutralEntry := getEOTSBannerEntry(EOTSTowerMage, EOTSTowerStateNeutral)

	// 1. Alliance claims Mage Tower
	handled := srv.handleEOTSGameObjectUse(ctx, allySess, 200, mageNeutralEntry)
	if !handled {
		t.Fatal("expected handleEOTSGameObjectUse to succeed")
	}
	if eots.Towers[EOTSTowerMage].State != EOTSTowerStateControlledAlliance {
		t.Fatalf("expected Mage Tower Alliance controlled, got %d", eots.Towers[EOTSTowerMage].State)
	}
	if eots.AllianceTowersCount != 1 || eots.HordeTowersCount != 0 {
		t.Fatalf("expected Alliance towers=1 Horde towers=0, got A=%d H=%d", eots.AllianceTowersCount, eots.HordeTowersCount)
	}

	// 2. Horde captures Mage Tower from Alliance
	mageAllyEntry := getEOTSBannerEntry(EOTSTowerMage, EOTSTowerStateControlledAlliance)
	handled = srv.handleEOTSGameObjectUse(ctx, hordeSess, 201, mageAllyEntry)
	if !handled {
		t.Fatal("expected handleEOTSGameObjectUse to succeed")
	}
	if eots.Towers[EOTSTowerMage].State != EOTSTowerStateControlledHorde {
		t.Fatalf("expected Mage Tower Horde controlled, got %d", eots.Towers[EOTSTowerMage].State)
	}
	if eots.AllianceTowersCount != 0 || eots.HordeTowersCount != 1 {
		t.Fatalf("expected Alliance towers=0 Horde towers=1, got A=%d H=%d", eots.AllianceTowersCount, eots.HordeTowersCount)
	}
}

func TestEOTSFlagPickupDropAndReturn(t *testing.T) {
	srv := setupEOTSServer()
	eots := srv.getOrCreateEOTSState(EOTSMapID)
	ctx := context.Background()

	// Player at center (2174.0, 1569.0, 1160.0)
	carrier, _ := newEOTSTestSession(t, srv, 10, 1, "FlagRunner", 2174.0, 1569.0, 1160.0)

	// 1. Pickup central flag
	handled := srv.handleEOTSGameObjectUse(ctx, carrier, 300, EOTSFlagCenterEntry)
	if !handled {
		t.Fatal("expected central flag pickup to succeed")
	}
	if eots.FlagState != EOTSFlagStateCarried || eots.FlagCarrierGUID != carrier.playerGUID {
		t.Fatalf("expected FlagStateCarried by player, got state=%d carrier=%d", eots.FlagState, eots.FlagCarrierGUID)
	}
	if !carrier.hasAura(EOTSSpellNetherstormFlag) {
		t.Fatal("expected carrier to have Netherstorm Flag aura 34976")
	}

	// Verify getEOTSFlagCarriers returns carrier
	carriers := srv.getEOTSFlagCarriers(EOTSMapID)
	if len(carriers) != 1 || carriers[0].playerGUID != carrier.playerGUID {
		t.Fatalf("expected 1 flag carrier, got %d", len(carriers))
	}

	// 2. Carrier dies -> flag drops
	srv.handleEOTSPlayerDeath(carrier)
	if eots.FlagState != EOTSFlagStateDropped || eots.FlagCarrierGUID != 0 {
		t.Fatalf("expected FlagStateDropped after death, got state=%d", eots.FlagState)
	}
	if carrier.hasAura(EOTSSpellNetherstormFlag) {
		t.Fatal("expected flag aura removed on death")
	}
	if eots.FlagDroppedGUID == 0 {
		t.Fatal("expected dropped flag gameobject GUID created")
	}

	// 3. Second player picks up dropped flag
	runner2, _ := newEOTSTestSession(t, srv, 20, 2, "HordeRunner", 2174.0, 1569.0, 1160.0)
	handled = srv.handleEOTSGameObjectUse(ctx, runner2, eots.FlagDroppedGUID, EOTSFlagDroppedEntry)
	if !handled {
		t.Fatal("expected dropped flag pickup to succeed")
	}
	if eots.FlagState != EOTSFlagStateCarried || eots.FlagCarrierGUID != runner2.playerGUID {
		t.Fatalf("expected runner2 to carry flag, got state=%d carrier=%d", eots.FlagState, eots.FlagCarrierGUID)
	}
	if !runner2.hasAura(EOTSSpellNetherstormFlag) {
		t.Fatal("expected runner2 to have Netherstorm Flag aura")
	}

	// 4. Runner 2 leaves battlefield -> flag drops and auto-returns
	srv.handleEOTSPlayerLeave(runner2)
	if eots.FlagState != EOTSFlagStateDropped {
		t.Fatalf("expected dropped flag after carrier left, got %d", eots.FlagState)
	}

	// Override return timer to fast expiry for testing
	eots.FlagReturnTimer.Stop()
	eots.mu.Lock()
	eots.FlagReturnTimer = time.AfterFunc(10*time.Millisecond, func() {
		eots.mu.Lock()
		defer eots.mu.Unlock()
		if eots.FlagState == EOTSFlagStateDropped {
			srv.despawnDynamicGameObject(eots.FlagDroppedGUID)
			eots.FlagDroppedGUID = 0
			eots.FlagState = EOTSFlagStateAtCenter
		}
	})
	eots.mu.Unlock()

	time.Sleep(25 * time.Millisecond)
	if eots.FlagState != EOTSFlagStateAtCenter {
		t.Fatalf("expected flag returned to center, got %d", eots.FlagState)
	}
}

func TestEOTSFlagCapturePointsAndWin(t *testing.T) {
	srv := setupEOTSServer()
	eots := srv.getOrCreateEOTSState(EOTSMapID)
	ctx := context.Background()

	// Alliance controls 3 towers
	eots.AllianceTowersCount = 3
	eots.Towers[EOTSTowerMage].State = EOTSTowerStateControlledAlliance

	// Alliance carrier at Mage Tower (2228.4, 1330.4, 1199.0)
	carrier, _ := newEOTSTestSession(t, srv, 10, 1, "AllianceScorer", 2228.4, 1330.4, 1199.0)
	carrier.applyAura(EOTSSpellNetherstormFlag)
	eots.FlagState = EOTSFlagStateCarried
	eots.FlagCarrierGUID = carrier.playerGUID

	mageAllyEntry := getEOTSBannerEntry(EOTSTowerMage, EOTSTowerStateControlledAlliance)

	// Carrier interacts with controlled Mage Tower banner -> CAPTURE!
	// 3 towers held -> awards 100 points!
	handled := srv.handleEOTSGameObjectUse(ctx, carrier, 400, mageAllyEntry)
	if !handled {
		t.Fatal("expected flag capture handled")
	}
	if eots.AllianceResources != 100 {
		t.Fatalf("expected 100 resources for 3-tower capture, got %d", eots.AllianceResources)
	}
	if carrier.hasAura(EOTSSpellNetherstormFlag) {
		t.Fatal("expected flag aura removed on capture")
	}
	if eots.FlagState != EOTSFlagStateAtCenter {
		t.Fatalf("expected flag state reset to center, got %d", eots.FlagState)
	}

	// Test 4-tower capture gives 500 points
	eots.AllianceTowersCount = 4
	eots.FlagState = EOTSFlagStateCarried
	eots.FlagCarrierGUID = carrier.playerGUID
	carrier.applyAura(EOTSSpellNetherstormFlag)

	srv.handleEOTSGameObjectUse(ctx, carrier, 401, mageAllyEntry)
	// 100 + 500 = 600 points
	if eots.AllianceResources != 600 {
		t.Fatalf("expected 600 resources after 4-tower capture, got %d", eots.AllianceResources)
	}

	// 2. Continuous resource generation (TickEOTSResources)
	// With 4 towers held, rate is 10 pts/sec
	srv.TickEOTSResources(eots, 2000) // 2 seconds -> +20 points = 620
	if eots.AllianceResources != 620 {
		t.Fatalf("expected 620 resources after 2s, got %d", eots.AllianceResources)
	}

	// 3. Win condition at 1600 points
	eots.AllianceResources = 1595
	srv.TickEOTSResources(eots, 1000) // 1 second -> +10 -> 1605 -> capped at 1600
	if eots.AllianceResources != 1600 {
		t.Fatalf("expected resources capped at 1600, got %d", eots.AllianceResources)
	}
	if eots.Winner != 0 {
		t.Fatalf("expected Alliance winner (0), got %d", eots.Winner)
	}
}
