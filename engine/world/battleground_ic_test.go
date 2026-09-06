package world

import (
	"context"
	"net"
	"testing"
	"time"
)

func setupICTestServer(t *testing.T) (*Server, *session, *session) {
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
		icState:            make(map[uint32]*icBattlegroundState),
	}

	// Alliance Player (Human)
	allySess := &session{
		conn:         s1Conn,
		playerGUID:   1,
		playerLoaded: true,
		accountName:  "AllyPlayer",
		player: &playerState{
			GUID:   1,
			Map:    ICMapID,
			Name:   "AllyWarrior",
			X:      300.0,
			Y:      -800.0,
			Z:      50.0,
			Race:   1, // Human (Alliance)
			Health: 1000,
		},
		server: srv,
	}

	// Horde Player (Orc)
	hordeSess := &session{
		conn:         s2Conn,
		playerGUID:   2,
		playerLoaded: true,
		accountName:  "HordePlayer",
		player: &playerState{
			GUID:   2,
			Map:    ICMapID,
			Name:   "HordeHunter",
			X:      1200.0,
			Y:      -700.0,
			Z:      50.0,
			Race:   2, // Orc (Horde)
			Health: 1000,
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

func TestICInitialization(t *testing.T) {
	srv, allySess, _ := setupICTestServer(t)
	ic := srv.getOrCreateICState(ICMapID)
	if ic == nil {
		t.Fatal("expected icBattlegroundState to be created")
	}

	if ic.AllianceReinforcements != ICMaxReinforcements {
		t.Errorf("expected 300 alliance reinforcements, got %d", ic.AllianceReinforcements)
	}
	if ic.HordeReinforcements != ICMaxReinforcements {
		t.Errorf("expected 300 horde reinforcements, got %d", ic.HordeReinforcements)
	}
	if !ic.AllianceBossAlive || !ic.HordeBossAlive {
		t.Error("expected both bosses to be alive initially")
	}

	// 6 gates should be OK
	for i := 0; i < ICMaxGates; i++ {
		if ic.Gates[i].State != ICGateOK {
			t.Errorf("expected gate %d state to be ICGateOK (1), got %d", i, ic.Gates[i].State)
		}
		if ic.Gates[i].Health != ICGateDefaultMaxHealth {
			t.Errorf("expected gate %d health %d, got %d", i, ICGateDefaultMaxHealth, ic.Gates[i].Health)
		}
	}

	// Initial node states
	if ic.Nodes[ICNodeRefinery].State != ICNodeStateUncontrolled {
		t.Errorf("expected Refinery uncontrolled initially, got %d", ic.Nodes[ICNodeRefinery].State)
	}
	if ic.Nodes[ICNodeGraveyardA].State != ICNodeStateControlledA {
		t.Errorf("expected Alliance Keep graveyard controlled by Alliance, got %d", ic.Nodes[ICNodeGraveyardA].State)
	}
	if ic.Nodes[ICNodeGraveyardH].State != ICNodeStateControlledH {
		t.Errorf("expected Horde Keep graveyard controlled by Horde, got %d", ic.Nodes[ICNodeGraveyardH].State)
	}

	// Send initial world states without error
	srv.sendICInitialWorldStates(allySess)
}

func TestICPlayerDeathsAndReinforcements(t *testing.T) {
	srv, allySess, hordeSess := setupICTestServer(t)
	ic := srv.getOrCreateICState(ICMapID)

	// Alliance player dies -> Alliance loses 1 reinforcement
	srv.handleICPlayerDeath(allySess)
	if ic.AllianceReinforcements != ICMaxReinforcements-1 {
		t.Errorf("expected Alliance reinforcements %d, got %d", ICMaxReinforcements-1, ic.AllianceReinforcements)
	}

	// Horde player dies -> Horde loses 1 reinforcement
	srv.handleICPlayerDeath(hordeSess)
	if ic.HordeReinforcements != ICMaxReinforcements-1 {
		t.Errorf("expected Horde reinforcements %d, got %d", ICMaxReinforcements-1, ic.HordeReinforcements)
	}

	// Reduce Horde reinforcements to 1, then kill Horde player -> Alliance wins!
	ic.HordeReinforcements = 1
	srv.handleICPlayerDeath(hordeSess)
	if ic.HordeReinforcements != 0 {
		t.Errorf("expected Horde reinforcements 0, got %d", ic.HordeReinforcements)
	}
	if ic.Winner != int8(ICTeamAlliance) {
		t.Errorf("expected Alliance (0) to win when Horde reinforcements hit 0, got %d", ic.Winner)
	}
}

func TestICBossKills(t *testing.T) {
	// Scenario A: Alliance kills Horde Boss (Overlord Agmar) -> Alliance wins immediately!
	srv1, allySess1, _ := setupICTestServer(t)
	ic1 := srv1.getOrCreateICState(ICMapID)
	srv1.handleICCreatureKilled(allySess1, ICCreatureOverlordAgmar)
	if ic1.Winner != int8(ICTeamAlliance) {
		t.Errorf("expected Alliance (0) to win on Overlord Agmar kill, got %d", ic1.Winner)
	}
	if ic1.HordeBossAlive {
		t.Error("expected HordeBossAlive to be false")
	}

	// Scenario B: Horde kills Alliance Boss (Halford Wyrmbane) -> Horde wins immediately!
	srv2, _, hordeSess2 := setupICTestServer(t)
	ic2 := srv2.getOrCreateICState(ICMapID)
	srv2.handleICCreatureKilled(hordeSess2, ICCreatureHalfordWyrmbane)
	if ic2.Winner != int8(ICTeamHorde) {
		t.Errorf("expected Horde (1) to win on Halford Wyrmbane kill, got %d", ic2.Winner)
	}
	if ic2.AllianceBossAlive {
		t.Error("expected AllianceBossAlive to be false")
	}
}

func TestICNodeAssaultAndDefend(t *testing.T) {
	srv, allySess, hordeSess := setupICTestServer(t)
	ctx := context.Background()
	ic := srv.getOrCreateICState(ICMapID)
	ic.BannerCaptureDuration = 50 * time.Millisecond // Fast for test

	// Alliance assaults the Docks banner
	srv.handleICGameObjectUse(ctx, allySess, 100, ICGameObjectBannerDocks)
	if ic.Nodes[ICNodeDocks].State != ICNodeStateConflictA {
		t.Fatalf("expected Docks to be in ConflictA (1), got %d", ic.Nodes[ICNodeDocks].State)
	}

	// Wait for capture timer to resolve
	time.Sleep(75 * time.Millisecond)

	if ic.Nodes[ICNodeDocks].State != ICNodeStateControlledA {
		t.Fatalf("expected Docks to be ControlledA (3), got %d", ic.Nodes[ICNodeDocks].State)
	}
	if ic.Nodes[ICNodeDocks].Faction != ICTeamAlliance {
		t.Fatalf("expected Docks faction to be Alliance (0), got %d", ic.Nodes[ICNodeDocks].Faction)
	}

	// Horde now assaults the Docks
	srv.handleICGameObjectUse(ctx, hordeSess, 101, ICGameObjectBannerDocksA)
	if ic.Nodes[ICNodeDocks].State != ICNodeStateConflictH {
		t.Fatalf("expected Docks to be in ConflictH (2), got %d", ic.Nodes[ICNodeDocks].State)
	}

	// Alliance defends the Docks before capture timer expires!
	srv.handleICGameObjectUse(ctx, allySess, 102, ICGameObjectBannerDocksHCont)
	if ic.Nodes[ICNodeDocks].State != ICNodeStateControlledA {
		t.Fatalf("expected Docks to be defended and reverted to ControlledA (3), got %d", ic.Nodes[ICNodeDocks].State)
	}
}

func TestICGateDamageAndDestruction(t *testing.T) {
	srv, allySess, _ := setupICTestServer(t)
	ic := srv.getOrCreateICState(ICMapID)

	gate := &ic.Gates[ICHordeFrontGate]
	if gate.State != ICGateOK {
		t.Fatalf("expected gate OK")
	}

	// Deal 50,000 damage (health 70,000 > 60,000) -> remains ICGateOK
	srv.DamageICGate(ICMapID, ICHordeFrontGate, 50000, allySess)
	if gate.State != ICGateOK {
		t.Errorf("expected gate to remain ICGateOK, got %d", gate.State)
	}

	// Deal 15,000 damage (health 55,000 <= 60,000) -> becomes ICGateDamaged (2)
	srv.DamageICGate(ICMapID, ICHordeFrontGate, 15000, allySess)
	if gate.State != ICGateDamaged {
		t.Errorf("expected gate to be ICGateDamaged (2), got %d", gate.State)
	}

	// Destroy gate
	srv.DamageICGate(ICMapID, ICHordeFrontGate, 60000, allySess)
	if gate.State != ICGateDestroyed {
		t.Errorf("expected gate to be ICGateDestroyed (3), got %d", gate.State)
	}
	if gate.Health != 0 {
		t.Errorf("expected gate health 0, got %d", gate.Health)
	}
}

func TestICResourceTick(t *testing.T) {
	srv, _, _ := setupICTestServer(t)
	ic := srv.getOrCreateICState(ICMapID)
	ic.AllianceReinforcements = 250
	ic.HordeReinforcements = 250

	// Control Refinery for Alliance and Quarry for Horde
	ic.Nodes[ICNodeRefinery].State = ICNodeStateControlledA
	ic.Nodes[ICNodeQuarry].State = ICNodeStateControlledH

	// Resource tick awards +1 reinforcement to both
	srv.TickICResources(ic)
	if ic.AllianceReinforcements != 251 {
		t.Errorf("expected Alliance reinforcements 251, got %d", ic.AllianceReinforcements)
	}
	if ic.HordeReinforcements != 251 {
		t.Errorf("expected Horde reinforcements 251, got %d", ic.HordeReinforcements)
	}
}

func TestICVehicleDestructionAndLeave(t *testing.T) {
	srv, allySess, hordeSess := setupICTestServer(t)
	ic := srv.getOrCreateICState(ICMapID)

	// Destroy Demolisher
	srv.handleICCreatureKilled(allySess, ICCreatureDemolisher)
	if ic.VehiclesDestroyed[allySess.playerGUID] != 1 {
		t.Errorf("expected 1 vehicle destroyed for ally player, got %d", ic.VehiclesDestroyed[allySess.playerGUID])
	}

	// Destroy Siege Engine
	srv.handleICCreatureKilled(allySess, ICCreatureSiegeEngineH)
	if ic.VehiclesDestroyed[allySess.playerGUID] != 2 {
		t.Errorf("expected 2 vehicles destroyed, got %d", ic.VehiclesDestroyed[allySess.playerGUID])
	}

	// Test leave does not panic
	srv.handleICPlayerLeave(hordeSess)
}
