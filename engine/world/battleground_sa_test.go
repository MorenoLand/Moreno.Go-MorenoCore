package world

import (
	"context"
	"net"
	"testing"
	"time"
)

func setupSATestServer(t *testing.T) (*Server, *session, *session) {
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
		saState:            make(map[uint32]*saBattlegroundState),
	}

	// Alliance Player (Human)
	allySess := &session{
		conn:         s1Conn,
		playerGUID:   1,
		playerLoaded: true,
		accountName:  "AllyPlayer",
		player: &playerState{
			GUID:   1,
			Map:    SAMapID,
			Name:   "AllyWarrior",
			X:      1400.0,
			Y:      100.0,
			Z:      30.0,
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
			Map:    SAMapID,
			Name:   "HordeShaman",
			X:      1400.0,
			Y:      100.0,
			Z:      30.0,
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
			_ = c.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			if _, err := c.Read(buf); err != nil {
				return
			}
		}
	}
	go drain(c1Conn)
	go drain(c2Conn)

	return srv, allySess, hordeSess
}

func TestSAInitialization(t *testing.T) {
	srv, allySess, _ := setupSATestServer(t)
	sa := srv.getOrCreateSAState(SAMapID)
	if sa == nil {
		t.Fatal("expected saBattlegroundState to be created")
	}

	if sa.Status != SAStatusWarmup {
		t.Errorf("expected initial status SAStatusWarmup, got %d", sa.Status)
	}
	if sa.Attackers != SATeamAlliance {
		t.Errorf("expected initial attackers SATeamAlliance (0), got %d", sa.Attackers)
	}

	// 6 gates should be OK
	for i := 0; i < SAMaxGates; i++ {
		if sa.Gates[i].State != SAGateOK {
			t.Errorf("expected gate %d to be SAGateOK (1), got %d", i, sa.Gates[i].State)
		}
		if sa.Gates[i].Health != SAGateDefaultMaxHealth {
			t.Errorf("expected gate %d health %d, got %d", i, SAGateDefaultMaxHealth, sa.Gates[i].Health)
		}
	}

	// Initial graveyards
	if sa.Graveyards[SABeachGY] != SATeamAlliance {
		t.Errorf("expected beach GY owned by Alliance (0), got %d", sa.Graveyards[SABeachGY])
	}
	if sa.Graveyards[SADefenderLastGY] != SATeamHorde {
		t.Errorf("expected defender GY owned by Horde (1), got %d", sa.Graveyards[SADefenderLastGY])
	}
	if sa.Graveyards[SARightCapturableGY] != SATeamHorde {
		t.Errorf("expected East GY owned by Horde (1), got %d", sa.Graveyards[SARightCapturableGY])
	}
	if sa.Graveyards[SALeftCapturableGY] != SATeamHorde {
		t.Errorf("expected West GY owned by Horde (1), got %d", sa.Graveyards[SALeftCapturableGY])
	}
	if sa.Graveyards[SACentralCapturableGY] != SATeamHorde {
		t.Errorf("expected Central GY owned by Horde (1), got %d", sa.Graveyards[SACentralCapturableGY])
	}

	// Send initial world states without error
	srv.sendSAInitialWorldStates(allySess)
}

func TestSAGateDamageAndDestruction(t *testing.T) {
	srv, allySess, _ := setupSATestServer(t)
	sa := srv.getOrCreateSAState(SAMapID)
	sa.Status = SAStatusRoundOne

	// Initial Green gate state
	if sa.Gates[SAGateGreen].State != SAGateOK {
		t.Fatalf("expected gate OK")
	}

	// Damage green gate by 40,000 (health becomes 60,000 > 50,000) -> remains SAGateOK
	srv.DamageGate(SAMapID, SAGateGreen, 40000, allySess)
	if sa.Gates[SAGateGreen].State != SAGateOK {
		t.Errorf("expected gate to remain SAGateOK, got %d", sa.Gates[SAGateGreen].State)
	}

	// Damage by another 15,000 (health becomes 45,000 <= 50,000) -> becomes SAGateDamaged (2)
	srv.DamageGate(SAMapID, SAGateGreen, 15000, allySess)
	if sa.Gates[SAGateGreen].State != SAGateDamaged {
		t.Errorf("expected gate to be SAGateDamaged (2), got %d", sa.Gates[SAGateGreen].State)
	}

	// Deal remaining damage to destroy gate -> becomes SAGateDestroyed (3)
	srv.DamageGate(SAMapID, SAGateGreen, 50000, allySess)
	if sa.Gates[SAGateGreen].State != SAGateDestroyed {
		t.Errorf("expected gate to be SAGateDestroyed (3), got %d", sa.Gates[SAGateGreen].State)
	}
	if sa.Gates[SAGateGreen].Health != 0 {
		t.Errorf("expected gate health 0, got %d", sa.Gates[SAGateGreen].Health)
	}
	if !sa.GateDestroyed {
		t.Errorf("expected sa.GateDestroyed true")
	}
	if sa.GatesDestroyed[allySess.playerGUID] != 1 {
		t.Errorf("expected allySess to have 1 gate destroyed, got %d", sa.GatesDestroyed[allySess.playerGUID])
	}
}

func TestSAInteractionPrerequisites(t *testing.T) {
	srv, _, _ := setupSATestServer(t)
	sa := srv.getOrCreateSAState(SAMapID)

	// Initially no gates are destroyed
	if sa.canInteractWithObject(SAGameObjectFlagLeftA) {
		t.Error("expected Left Flag interaction to be blocked when Green and Blue gates are intact")
	}
	if sa.canInteractWithObject(SAGameObjectFlagRightA) {
		t.Error("expected Right Flag interaction to be blocked when Green and Blue gates are intact")
	}
	if sa.canInteractWithObject(SAGameObjectFlagCentralA) {
		t.Error("expected Central Flag interaction to be blocked when Red and Purple gates are intact")
	}
	if sa.canInteractWithObject(SAGameObjectTitanRelic) {
		t.Error("expected Titan Relic interaction to be blocked when Yellow and Ancient gates are intact")
	}

	// Destroy Green Gate -> Left and Right flags become interactable
	sa.Gates[SAGateGreen].State = SAGateDestroyed
	if !sa.canInteractWithObject(SAGameObjectFlagLeftA) {
		t.Error("expected Left Flag to be interactable once Green gate is destroyed")
	}
	if !sa.canInteractWithObject(SAGameObjectFlagRightA) {
		t.Error("expected Right Flag to be interactable once Green gate is destroyed")
	}
	// Central flag and Relic should still be blocked
	if sa.canInteractWithObject(SAGameObjectFlagCentralA) {
		t.Error("expected Central Flag to remain blocked")
	}
	if sa.canInteractWithObject(SAGameObjectTitanRelic) {
		t.Error("expected Titan Relic to remain blocked")
	}

	// Destroy Purple Gate -> Central flag becomes interactable
	sa.Gates[SAGatePurple].State = SAGateDestroyed
	if !sa.canInteractWithObject(SAGameObjectFlagCentralA) {
		t.Error("expected Central Flag to be interactable once Purple gate is destroyed")
	}

	// Destroy Yellow Gate only -> Titan Relic still blocked (needs Ancient Gate too)
	sa.Gates[SAGateYellow].State = SAGateDestroyed
	if sa.canInteractWithObject(SAGameObjectTitanRelic) {
		t.Error("expected Titan Relic to remain blocked when Ancient gate is still intact")
	}

	// Destroy Ancient Gate -> Titan Relic becomes interactable!
	sa.Gates[SAGateAncient].State = SAGateDestroyed
	if !sa.canInteractWithObject(SAGameObjectTitanRelic) {
		t.Error("expected Titan Relic to be interactable once both Yellow and Ancient gates are destroyed")
	}
	if !sa.canInteractWithObject(SAGameObjectTitanRelic2) {
		t.Error("expected Titan Relic 2 to be interactable once both Yellow and Ancient gates are destroyed")
	}
}

func TestSAGraveyardCapturing(t *testing.T) {
	srv, allySess, hordeSess := setupSATestServer(t)
	ctx := context.Background()
	sa := srv.getOrCreateSAState(SAMapID)
	sa.Status = SAStatusRoundOne
	sa.Attackers = SATeamAlliance

	// Destroy green gate to allow courtyard flag clicks
	sa.Gates[SAGateGreen].State = SAGateDestroyed

	// Defender (Horde) clicking flag should not capture
	srv.handleSAGameObjectUse(ctx, hordeSess, 100, SAGameObjectFlagLeftH)
	if sa.Graveyards[SALeftCapturableGY] == SATeamAlliance {
		t.Error("defender should not capture graveyard")
	}

	// Attacker (Alliance) clicking Left flag captures West GY
	srv.handleSAGameObjectUse(ctx, allySess, 100, SAGameObjectFlagLeftA)
	if sa.Graveyards[SALeftCapturableGY] != SATeamAlliance {
		t.Errorf("expected West GY captured by Alliance (0), got %d", sa.Graveyards[SALeftCapturableGY])
	}

	// Attacker clicking Right flag captures East GY
	srv.handleSAGameObjectUse(ctx, allySess, 101, SAGameObjectFlagRightA)
	if sa.Graveyards[SARightCapturableGY] != SATeamAlliance {
		t.Errorf("expected East GY captured by Alliance (0), got %d", sa.Graveyards[SARightCapturableGY])
	}

	// Central flag before red/purple gate destroyed should be blocked
	srv.handleSAGameObjectUse(ctx, allySess, 102, SAGameObjectFlagCentralA)
	if sa.Graveyards[SACentralCapturableGY] == SATeamAlliance {
		t.Error("Central GY should not be captured before red or purple gate is destroyed")
	}

	// Destroy Red Gate and capture Central GY
	sa.Gates[SAGateRed].State = SAGateDestroyed
	srv.handleSAGameObjectUse(ctx, allySess, 102, SAGameObjectFlagCentralA)
	if sa.Graveyards[SACentralCapturableGY] != SATeamAlliance {
		t.Errorf("expected Central GY captured by Alliance (0), got %d", sa.Graveyards[SACentralCapturableGY])
	}
}

func TestSARoundTransitionsAndTimerCapping(t *testing.T) {
	srv, allySess, hordeSess := setupSATestServer(t)
	ctx := context.Background()
	sa := srv.getOrCreateSAState(SAMapID)
	sa.WarmupLength = 10 * time.Second
	sa.SecondWarmupLength = 5 * time.Second
	sa.RoundLength = 600 * time.Second

	// Tick warmup -> Round 1
	srv.TickSA(sa, 10*time.Second)
	if sa.Status != SAStatusRoundOne {
		t.Fatalf("expected SAStatusRoundOne, got %d", sa.Status)
	}
	if sa.EndRoundTimer != 600*time.Second {
		t.Errorf("expected Round 1 EndRoundTimer 600s, got %v", sa.EndRoundTimer)
	}

	// Destroy all required gates for Alliance attacker in Round 1
	sa.Gates[SAGateGreen].State = SAGateDestroyed
	sa.Gates[SAGateRed].State = SAGateDestroyed
	sa.Gates[SAGateYellow].State = SAGateDestroyed
	sa.Gates[SAGateAncient].State = SAGateDestroyed

	// Alliance breaches in 4 minutes (240s)
	sa.TotalTime = 240 * time.Second
	srv.handleSAGameObjectUse(ctx, allySess, 200, SAGameObjectTitanRelic)

	// Round 1 completed!
	if sa.RoundScores[0].Time != 240*time.Second {
		t.Errorf("expected Round 1 time 240s, got %v", sa.RoundScores[0].Time)
	}
	if sa.RoundScores[0].Winner != SATeamAlliance {
		t.Errorf("expected Round 1 winner Alliance (0), got %d", sa.RoundScores[0].Winner)
	}

	// Status should now be SecondWarmup, and Attackers should be Horde!
	if sa.Status != SAStatusSecondWarmup {
		t.Fatalf("expected SAStatusSecondWarmup, got %d", sa.Status)
	}
	if sa.Attackers != SATeamHorde {
		t.Fatalf("expected Horde (1) to be attackers for Round 2, got %d", sa.Attackers)
	}

	// Gates should have been reset to OK
	for i := 0; i < SAMaxGates; i++ {
		if sa.Gates[i].State != SAGateOK {
			t.Errorf("expected gate %d reset to SAGateOK, got %d", i, sa.Gates[i].State)
		}
	}

	// Tick second warmup -> Round 2 starts!
	srv.TickSA(sa, 5*time.Second)
	if sa.Status != SAStatusRoundTwo {
		t.Fatalf("expected SAStatusRoundTwo, got %d", sa.Status)
	}

	// Round 2 timer MUST BE CAPPED to Round 1 completion time (240s)!
	if sa.EndRoundTimer != 240*time.Second {
		t.Errorf("expected Round 2 timer capped to 240s, got %v", sa.EndRoundTimer)
	}

	// Horde breaks all gates in Round 2
	sa.Gates[SAGateBlue].State = SAGateDestroyed
	sa.Gates[SAGatePurple].State = SAGateDestroyed
	sa.Gates[SAGateYellow].State = SAGateDestroyed
	sa.Gates[SAGateAncient].State = SAGateDestroyed

	// Scenario A: Horde breaches in 3 minutes (180s < 240s) -> Horde wins!
	sa.TotalTime = 180 * time.Second
	srv.handleSAGameObjectUse(ctx, hordeSess, 200, SAGameObjectTitanRelic)

	if sa.Status != SAStatusFinished {
		t.Fatalf("expected SAStatusFinished, got %d", sa.Status)
	}
	if sa.Winner != 1 {
		t.Errorf("expected Horde (1) to win because 180s < 240s, got winner %d", sa.Winner)
	}
}

func TestSARoundTwoExpirationAllianceWins(t *testing.T) {
	srv, allySess, _ := setupSATestServer(t)
	ctx := context.Background()
	sa := srv.getOrCreateSAState(SAMapID)
	sa.WarmupLength = 0
	sa.SecondWarmupLength = 0
	sa.RoundLength = 600 * time.Second

	// Start Round 1
	srv.TickSA(sa, 1*time.Second)

	// Alliance finishes Round 1 in 300 seconds (5 mins)
	sa.Gates[SAGateYellow].State = SAGateDestroyed
	sa.Gates[SAGateAncient].State = SAGateDestroyed
	sa.TotalTime = 300 * time.Second
	srv.handleSAGameObjectUse(ctx, allySess, 200, SAGameObjectTitanRelic)

	// Round 2 starts
	srv.TickSA(sa, 1*time.Second)
	if sa.Status != SAStatusRoundTwo {
		t.Fatalf("expected Round 2")
	}
	if sa.EndRoundTimer != 300*time.Second {
		t.Fatalf("expected EndRoundTimer 300s, got %v", sa.EndRoundTimer)
	}

	// Horde in Round 2 fails to breach before 300s expires!
	srv.TickSA(sa, 301*time.Second)

	if sa.Status != SAStatusFinished {
		t.Fatalf("expected SAStatusFinished, got %d", sa.Status)
	}
	// Alliance wins because Horde failed to beat Round 1 time
	if sa.Winner != 0 {
		t.Errorf("expected Alliance (0) to win match, got %d", sa.Winner)
	}
}

func TestSABothTeamsFailToBreachDraw(t *testing.T) {
	srv, _, _ := setupSATestServer(t)
	sa := srv.getOrCreateSAState(SAMapID)
	sa.WarmupLength = 0
	sa.SecondWarmupLength = 0
	sa.RoundLength = 600 * time.Second

	// Start Round 1
	srv.TickSA(sa, 1*time.Second)

	// Round 1 reaches full 600s expiration without relic click
	srv.TickSA(sa, 600*time.Second)
	if sa.Status != SAStatusSecondWarmup {
		t.Fatalf("expected SAStatusSecondWarmup, got %d", sa.Status)
	}
	if sa.RoundScores[0].Time != 600*time.Second {
		t.Errorf("expected Round 1 time 600s, got %v", sa.RoundScores[0].Time)
	}

	// Start Round 2
	srv.TickSA(sa, 1*time.Second)
	if sa.Status != SAStatusRoundTwo {
		t.Fatalf("expected SAStatusRoundTwo, got %d", sa.Status)
	}
	if sa.EndRoundTimer != 600*time.Second {
		t.Errorf("expected Round 2 timer 600s, got %v", sa.EndRoundTimer)
	}

	// Round 2 also expires at 600s without relic click
	srv.TickSA(sa, 600*time.Second)
	if sa.Status != SAStatusFinished {
		t.Fatalf("expected SAStatusFinished, got %d", sa.Status)
	}
	if sa.Winner != 2 {
		t.Errorf("expected Draw (2), got %d", sa.Winner)
	}
}

func TestSADemolisherKillAndLeave(t *testing.T) {
	srv, _, hordeSess := setupSATestServer(t)
	sa := srv.getOrCreateSAState(SAMapID)

	if !sa.DemolishersAlive[SATeamAlliance] {
		t.Errorf("expected demolishers alive initially")
	}

	// Horde player destroys an Alliance Demolisher
	srv.handleSACreatureKilled(hordeSess, SACreatureDemolisher)
	if sa.DemolishersAlive[SATeamAlliance] {
		t.Errorf("expected Alliance demolishers alive to be false")
	}
	if sa.DemolishersDestroyed[hordeSess.playerGUID] != 1 {
		t.Errorf("expected 1 demolisher destroyed for horde player, got %d", sa.DemolishersDestroyed[hordeSess.playerGUID])
	}

	// Verify death and leave handlers do not panic
	srv.handleSAPlayerDeath(hordeSess)
	srv.handleSAPlayerLeave(hordeSess)
}
