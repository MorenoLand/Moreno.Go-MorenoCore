package world

import (
	"context"
	"net"
	"testing"
	"time"
)

func setupWGTestServer(t *testing.T) (*Server, *session, *session) {
	s1Conn, c1Conn := net.Pipe()
	s2Conn, c2Conn := net.Pipe()

	t.Cleanup(func() {
		_ = s1Conn.Close()
		_ = c1Conn.Close()
		_ = s2Conn.Close()
		_ = c2Conn.Close()
	})

	srv := &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		vehicleKits:        make(map[uint64]*VehicleKit),
	}

	// Alliance Player (Human)
	allySess := &session{
		conn:         s1Conn,
		playerGUID:   1001,
		playerLoaded: true,
		accountName:  "AllyPlayer",
		player: &playerState{
			GUID:   1001,
			Map:    WGMapID,
			Zone:   WGZoneID,
			Name:   "AllyWarrior",
			X:      5300.0,
			Y:      2800.0,
			Z:      410.0,
			Race:   1, // Human (Alliance)
			Health: 20000,
		},
		server: srv,
	}

	// Horde Player (Orc)
	hordeSess := &session{
		conn:         s2Conn,
		playerGUID:   1002,
		playerLoaded: true,
		accountName:  "HordePlayer",
		player: &playerState{
			GUID:   1002,
			Map:    WGMapID,
			Zone:   WGZoneID,
			Name:   "HordeShaman",
			X:      4500.0,
			Y:      2800.0,
			Z:      400.0,
			Race:   2, // Orc (Horde)
			Health: 20000,
		},
		server: srv,
	}

	srv.sessions[allySess] = struct{}{}
	srv.sessions[hordeSess] = struct{}{}

	// Drain client packets in background
	drain := func(c net.Conn) {
		buf := make([]byte, 4096)
		for {
			if _, err := c.Read(buf); err != nil {
				return
			}
		}
	}
	go drain(c1Conn)
	go drain(c2Conn)

	return srv, allySess, hordeSess
}

func TestWGInitialization(t *testing.T) {
	srv, _, _ := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()

	if wg.MapID != WGMapID || wg.ZoneID != WGZoneID {
		t.Fatalf("expected MapID %d ZoneID %d, got %d and %d", WGMapID, WGZoneID, wg.MapID, wg.ZoneID)
	}
	if wg.IsActive {
		t.Fatal("expected Wintergrasp to initially be in peace mode")
	}
	if len(wg.Workshops) != int(WGMaxWorkshops) {
		t.Fatalf("expected %d workshops, got %d", WGMaxWorkshops, len(wg.Workshops))
	}
	if len(wg.Towers) != int(WGMaxTowers) {
		t.Fatalf("expected %d towers, got %d", WGMaxTowers, len(wg.Towers))
	}
	if len(wg.Gates) != 2 {
		t.Fatalf("expected 2 gates, got %d", len(wg.Gates))
	}
}

func TestWGBattleLifecycle(t *testing.T) {
	srv, allySess, hordeSess := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()

	// 1. Start battle with Alliance as defenders
	srv.StartWGBattle(WGTeamAlliance)
	if !wg.IsActive {
		t.Fatal("expected Wintergrasp battle to be active")
	}
	if wg.DefenderTeam != WGTeamAlliance || wg.AttackerTeam() != WGTeamHorde {
		t.Fatalf("expected Alliance defenders and Horde attackers, got def=%d att=%d", wg.DefenderTeam, wg.AttackerTeam())
	}
	if wg.Timer != WGBattleDuration {
		t.Fatalf("expected battle duration %v, got %v", WGBattleDuration, wg.Timer)
	}

	// Base workshop distribution: Keep West, Keep East, NE, NW to Defender (Alliance = 4 workshops = 16 max vehicles)
	// SE, SW to Attacker (Horde = 2 workshops = 8 max vehicles)
	if wg.MaxVehiclesAlliance != 16 {
		t.Fatalf("expected Alliance max vehicles 16, got %d", wg.MaxVehiclesAlliance)
	}
	if wg.MaxVehiclesHorde != 8 {
		t.Fatalf("expected Horde max vehicles 8, got %d", wg.MaxVehiclesHorde)
	}

	// Players join war
	srv.handleWGPlayerEnter(allySess)
	srv.handleWGPlayerEnter(hordeSess)

	if !allySess.hasAura(WGSpellRecruit) || !hordeSess.hasAura(WGSpellRecruit) {
		t.Fatal("expected entering players during wartime to receive Recruit rank aura")
	}

	// 2. Tick reduction
	srv.TickWG(10 * time.Minute)
	if wg.Timer != 20*time.Minute {
		t.Fatalf("expected remaining timer 20m, got %v", wg.Timer)
	}

	// 3. Battle timeout victory -> Defenders (Alliance) win!
	srv.TickWG(25 * time.Minute) // Exceeds remaining 20m
	if wg.IsActive {
		t.Fatal("expected battle to end after timer expiry")
	}
	if wg.DefenderTeam != WGTeamAlliance {
		t.Fatalf("expected Alliance to remain defenders after timeout win, got %d", wg.DefenderTeam)
	}
	if wg.Winner != int8(WGTeamAlliance) {
		t.Fatalf("expected Alliance winner (0), got %d", wg.Winner)
	}
	if wg.StatsDefA != 1 {
		t.Fatalf("expected Alliance defense count 1, got %d", wg.StatsDefA)
	}
	if wg.Timer != WGNoWarDuration {
		t.Fatalf("expected peace duration %v, got %v", WGNoWarDuration, wg.Timer)
	}

	// Check rewards
	if !allySess.hasAura(WGSpellEssenceOfWintergrasp) {
		t.Fatal("expected victorious Alliance player to gain Essence of Wintergrasp")
	}
	if !allySess.hasAura(WGSpellVictoryReward) {
		t.Fatal("expected victorious Alliance player to gain Victory Reward")
	}
	if !hordeSess.hasAura(WGSpellDefeatReward) {
		t.Fatal("expected defeated Horde player to gain Defeat Reward")
	}
	if hordeSess.hasAura(WGSpellEssenceOfWintergrasp) {
		t.Fatal("expected defeated Horde player not to have Essence of Wintergrasp")
	}
}

func TestWGRankProgression(t *testing.T) {
	srv, allySess, hordeSess := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()
	srv.StartWGBattle(WGTeamAlliance)

	srv.handleWGPlayerEnter(allySess)
	srv.handleWGPlayerEnter(hordeSess)

	if wg.PlayerRanks[allySess.playerGUID] != WGSpellRecruit {
		t.Fatalf("expected initial rank Recruit, got %d", wg.PlayerRanks[allySess.playerGUID])
	}

	// 4 kills -> still Recruit
	for i := 0; i < 4; i++ {
		srv.handleWGPlayerDeath(hordeSess, allySess)
	}
	if wg.PlayerRanks[allySess.playerGUID] != WGSpellRecruit {
		t.Fatalf("expected rank Recruit after 4 kills, got %d", wg.PlayerRanks[allySess.playerGUID])
	}

	// 5th kill -> promoted to Corporal!
	srv.handleWGPlayerDeath(hordeSess, allySess)
	if wg.PlayerRanks[allySess.playerGUID] != WGSpellCorporal {
		t.Fatalf("expected rank Corporal after 5 kills, got %d", wg.PlayerRanks[allySess.playerGUID])
	}
	if !allySess.hasAura(WGSpellCorporal) {
		t.Fatal("expected player to have Corporal aura")
	}
	if allySess.hasAura(WGSpellRecruit) {
		t.Fatal("expected Recruit aura removed after promotion to Corporal")
	}

	// 4 kills as Corporal -> still Corporal
	for i := 0; i < 4; i++ {
		srv.handleWGPlayerDeath(hordeSess, allySess)
	}
	if wg.PlayerRanks[allySess.playerGUID] != WGSpellCorporal {
		t.Fatalf("expected rank Corporal after 4 more kills, got %d", wg.PlayerRanks[allySess.playerGUID])
	}

	// 5th kill as Corporal (10th total) -> promoted to First Lieutenant!
	srv.handleWGPlayerDeath(hordeSess, allySess)
	if wg.PlayerRanks[allySess.playerGUID] != WGSpellLieutenant {
		t.Fatalf("expected rank Lieutenant after 10 total kills, got %d", wg.PlayerRanks[allySess.playerGUID])
	}
	if !allySess.hasAura(WGSpellLieutenant) {
		t.Fatal("expected player to have Lieutenant aura")
	}
	if allySess.hasAura(WGSpellCorporal) {
		t.Fatal("expected Corporal aura removed after promotion to Lieutenant")
	}

	// Additional kills preserve Lieutenant rank without issues
	for i := 0; i < 3; i++ {
		srv.handleWGPlayerDeath(hordeSess, allySess)
	}
	if wg.PlayerRanks[allySess.playerGUID] != WGSpellLieutenant {
		t.Fatalf("expected rank Lieutenant to be preserved, got %d", wg.PlayerRanks[allySess.playerGUID])
	}

	// Player leaves WG -> auras stripped
	srv.handleWGPlayerLeave(allySess)
	if allySess.hasAura(WGSpellLieutenant) || allySess.hasAura(WGSpellRecruit) {
		t.Fatal("expected rank auras to be stripped on leaving Wintergrasp")
	}
}

func TestWGVehicleConstruction(t *testing.T) {
	srv, allySess, _ := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()
	srv.StartWGBattle(WGTeamAlliance)

	// 1. Recruit cannot build any vehicles
	canBuild, reason := wg.CanBuildVehicle(WGTeamAlliance, WGSpellRecruit, WGCreatureCatapult)
	if canBuild {
		t.Fatal("expected Recruit to be unable to build Catapult")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason for failure")
	}

	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellRecruit, WGCreatureDemolisher)
	if canBuild {
		t.Fatal("expected Recruit to be unable to build Demolisher")
	}

	// 2. Corporal can build Catapults, but not Demolishers or Siege Engines
	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellCorporal, WGCreatureCatapult)
	if !canBuild {
		t.Fatal("expected Corporal to be able to build Catapult")
	}

	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellCorporal, WGCreatureDemolisher)
	if canBuild {
		t.Fatal("expected Corporal to be unable to build Demolisher")
	}

	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellCorporal, WGCreatureSiegeEngineAlliance)
	if canBuild {
		t.Fatal("expected Corporal to be unable to build Siege Engine")
	}

	// 3. Lieutenant can build all three vehicle types
	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellLieutenant, WGCreatureCatapult)
	if !canBuild {
		t.Fatal("expected Lieutenant to be able to build Catapult")
	}
	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellLieutenant, WGCreatureDemolisher)
	if !canBuild {
		t.Fatal("expected Lieutenant to be able to build Demolisher")
	}
	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellLieutenant, WGCreatureSiegeEngineAlliance)
	if !canBuild {
		t.Fatal("expected Lieutenant to be able to build Siege Engine")
	}

	// 4. Vehicle spawning and capacity limits
	for i := uint32(0); i < wg.MaxVehiclesAlliance; i++ {
		vehicleGUID := uint64(5000 + i)
		if !srv.SpawnWGVehicle(vehicleGUID, WGCreatureDemolisher, WGTeamAlliance) {
			t.Fatalf("failed to spawn vehicle %d", i)
		}
	}
	if wg.VehiclesAlliance != wg.MaxVehiclesAlliance {
		t.Fatalf("expected Alliance vehicles %d, got %d", wg.MaxVehiclesAlliance, wg.VehiclesAlliance)
	}

	// Trying to spawn over limit
	canBuild, reason = wg.CanBuildVehicle(WGTeamAlliance, WGSpellLieutenant, WGCreatureDemolisher)
	if canBuild {
		t.Fatal("expected vehicle construction to be rejected when at limit")
	}
	if reason != "Alliance vehicle limit reached." {
		t.Fatalf("unexpected reason: %s", reason)
	}

	// Destroying a vehicle frees up capacity
	if !srv.DestroyWGVehicle(5000) {
		t.Fatal("failed to destroy vehicle 5000")
	}
	if wg.VehiclesAlliance != wg.MaxVehiclesAlliance-1 {
		t.Fatalf("expected Alliance vehicles %d, got %d", wg.MaxVehiclesAlliance-1, wg.VehiclesAlliance)
	}
	canBuild, _ = wg.CanBuildVehicle(WGTeamAlliance, WGSpellLieutenant, WGCreatureDemolisher)
	if !canBuild {
		t.Fatal("expected vehicle construction to be allowed after destroying a vehicle")
	}
	_ = allySess
}

func TestWGWorkshopCapture(t *testing.T) {
	srv, _, hordeSess := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()
	srv.StartWGBattle(WGTeamAlliance)

	initAllyCap := wg.MaxVehiclesAlliance
	initHordeCap := wg.MaxVehiclesHorde

	// Horde captures Sunken Ring (NE) from Alliance
	if !srv.CaptureWGWorkshop(WGWorkshopNE, WGTeamHorde, hordeSess.player.Name) {
		t.Fatal("failed to capture Sunken Ring workshop")
	}

	if wg.Workshops[WGWorkshopNE].TeamControl != WGTeamHorde {
		t.Fatalf("expected Sunken Ring to be controlled by Horde, got %d", wg.Workshops[WGWorkshopNE].TeamControl)
	}
	if wg.MaxVehiclesAlliance != initAllyCap-4 {
		t.Fatalf("expected Alliance cap to drop by 4 to %d, got %d", initAllyCap-4, wg.MaxVehiclesAlliance)
	}
	if wg.MaxVehiclesHorde != initHordeCap+4 {
		t.Fatalf("expected Horde cap to increase by 4 to %d, got %d", initHordeCap+4, wg.MaxVehiclesHorde)
	}

	// Keep workshops cannot be captured
	if srv.CaptureWGWorkshop(WGWorkshopKeepWest, WGTeamHorde, hordeSess.player.Name) {
		t.Fatal("expected Keep workshop capture to be rejected")
	}
}

func TestWGTowersAndPenalty(t *testing.T) {
	srv, allySess, hordeSess := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()
	srv.StartWGBattle(WGTeamAlliance)

	srv.handleWGPlayerEnter(allySess)
	srv.handleWGPlayerEnter(hordeSess)

	initialTimer := wg.Timer

	// 1. Damage Shadowsight tower
	if !srv.DamageWGBuilding(WGGameObjectTowerShadowsight) {
		t.Fatal("failed to damage Shadowsight tower")
	}
	if wg.Towers[WGTowerShadowsight].State != 1 {
		t.Fatalf("expected tower state 1 (Damaged), got %d", wg.Towers[WGTowerShadowsight].State)
	}

	// 2. Destroy Shadowsight tower
	if !srv.DestroyWGBuilding(WGGameObjectTowerShadowsight) {
		t.Fatal("failed to destroy Shadowsight tower")
	}
	if wg.Towers[WGTowerShadowsight].State != 2 {
		t.Fatalf("expected tower state 2 (Destroyed), got %d", wg.Towers[WGTowerShadowsight].State)
	}
	if wg.BrokenSouthTowers != 1 {
		t.Fatalf("expected 1 broken south tower, got %d", wg.BrokenSouthTowers)
	}
	if !allySess.hasAura(WGSpellTowerControl) {
		t.Fatal("expected defender (Alliance) to receive Tower Control buff on south tower destruction")
	}

	// 3. Destroy second south tower (Winter's Edge)
	if !srv.DestroyWGBuilding(WGGameObjectTowerWintersEdge) {
		t.Fatal("failed to destroy Winter's Edge tower")
	}
	if wg.BrokenSouthTowers != 2 {
		t.Fatalf("expected 2 broken south towers, got %d", wg.BrokenSouthTowers)
	}
	// Timer not yet penalized (requires all 3)
	if wg.Timer != initialTimer {
		t.Fatalf("expected timer unchanged after 2 towers, got %v", wg.Timer)
	}

	// 4. Destroy third south tower (Flamewatch) -> 10 minute penalty applied!
	if !srv.DestroyWGBuilding(WGGameObjectTowerFlamewatch) {
		t.Fatal("failed to destroy Flamewatch tower")
	}
	if wg.BrokenSouthTowers != 3 {
		t.Fatalf("expected 3 broken south towers, got %d", wg.BrokenSouthTowers)
	}
	expectedTimer := initialTimer - WGSouthTowerTimePenalty
	if wg.Timer != expectedTimer {
		t.Fatalf("expected timer reduced to %v, got %v", expectedTimer, wg.Timer)
	}
}

func TestWGVaultGateAndTitanRelicVictory(t *testing.T) {
	ctx := context.Background()
	srv, allySess, hordeSess := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()
	srv.StartWGBattle(WGTeamAlliance) // Alliance defends, Horde attacks

	srv.handleWGPlayerEnter(allySess)
	srv.handleWGPlayerEnter(hordeSess)

	// 1. Attacker tries to click Titan Relic before Vault Gate is broken -> rejected
	srv.handleWGGameObjectUse(ctx, hordeSess, 9001, WGGameObjectTitanRelic)
	if !wg.IsActive {
		t.Fatal("expected battle to remain active when Relic is not interactible")
	}

	// 2. Destroy Vault Gate -> Relic becomes interactible
	if !srv.DestroyWGBuilding(WGGameObjectVaultGate) {
		t.Fatal("failed to destroy Vault Gate")
	}
	if !wg.RelicInteractible {
		t.Fatal("expected RelicInteractible to be true after Vault Gate is destroyed")
	}

	// 3. Defender (Alliance) tries to click Titan Relic -> rejected (only attackers can click it)
	srv.handleWGGameObjectUse(ctx, allySess, 9001, WGGameObjectTitanRelic)
	if !wg.IsActive {
		t.Fatal("expected battle to remain active when defender clicks Titan Relic")
	}

	// 4. Attacker (Horde) clicks Titan Relic -> Victory for Horde!
	srv.handleWGGameObjectUse(ctx, hordeSess, 9001, WGGameObjectTitanRelic)
	if wg.IsActive {
		t.Fatal("expected battle to end after attacker clicks Titan Relic")
	}
	if wg.DefenderTeam != WGTeamHorde {
		t.Fatalf("expected Horde to become the new defender team, got %d", wg.DefenderTeam)
	}
	if wg.Winner != int8(WGTeamHorde) {
		t.Fatalf("expected Horde winner (1), got %d", wg.Winner)
	}
	if wg.StatsWonH != 1 {
		t.Fatalf("expected Horde win stat 1, got %d", wg.StatsWonH)
	}

	// Horde gains Essence of Wintergrasp and victory reward
	if !hordeSess.hasAura(WGSpellEssenceOfWintergrasp) {
		t.Fatal("expected victorious Horde player to gain Essence of Wintergrasp")
	}
	if !hordeSess.hasAura(WGSpellVictoryReward) {
		t.Fatal("expected victorious Horde player to gain Victory Reward")
	}
	if allySess.hasAura(WGSpellEssenceOfWintergrasp) {
		t.Fatal("expected defeated Alliance player to lose Essence of Wintergrasp")
	}
	if !allySess.hasAura(WGSpellDefeatReward) {
		t.Fatal("expected defeated Alliance player to gain Defeat Reward")
	}
}

func TestWGTenacity(t *testing.T) {
	srv, allySess, hordeSess := setupWGTestServer(t)
	wg := srv.getOrCreateWGState()
	srv.StartWGBattle(WGTeamAlliance)

	// Create 4 more Horde players to cause team imbalance (1 Ally vs 5 Horde)
	extraHordeConns := make([]net.Conn, 4)
	for i := 0; i < 4; i++ {
		sConn, cConn := net.Pipe()
		extraHordeConns[i] = sConn
		go func(c net.Conn) {
			buf := make([]byte, 1024)
			for {
				if _, err := c.Read(buf); err != nil {
					return
				}
			}
		}(cConn)

		extraSess := &session{
			conn:         sConn,
			playerGUID:   uint64(2000 + i),
			playerLoaded: true,
			accountName:  "ExtraHorde",
			player: &playerState{
				GUID:   uint64(2000 + i),
				Map:    WGMapID,
				Zone:   WGZoneID,
				Name:   "ExtraHorde",
				Race:   2, // Orc
				Health: 10000,
			},
			server: srv,
		}
		srv.sessions[extraSess] = struct{}{}
		srv.handleWGPlayerEnter(extraSess)
	}

	srv.handleWGPlayerEnter(allySess)
	srv.handleWGPlayerEnter(hordeSess)

	// Ratio: 5 Horde / 1 Ally -> newStack = (5/1 - 1) * 4 = 16
	if wg.TenacityTeam != 0 {
		t.Fatalf("expected Tenacity team to be Alliance (0), got %d", wg.TenacityTeam)
	}
	if wg.TenacityStack != 16 {
		t.Fatalf("expected Tenacity stack 16, got %d", wg.TenacityStack)
	}
	if !allySess.hasAura(WGSpellTenacity) {
		t.Fatal("expected outnumbered Alliance player to gain Tenacity aura")
	}
	if !allySess.hasAura(WGSpellGreatestHonor) {
		t.Fatal("expected Alliance player with stack >= 15 to gain Greatest Honor buff")
	}
	if hordeSess.hasAura(WGSpellTenacity) {
		t.Fatal("expected Horde player not to have Tenacity aura")
	}
}
