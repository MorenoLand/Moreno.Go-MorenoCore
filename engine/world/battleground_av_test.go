package world

import (
	"context"
	"net"
	"testing"
	"time"
)

func setupAVTestServer(t *testing.T) (*Server, *session, *session) {
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
		avState:            make(map[uint32]*avBattlegroundState),
	}

	// Alliance Player (Human)
	allySess := &session{
		conn:         s1Conn,
		playerGUID:   1,
		playerLoaded: true,
		accountName:  "AllyPlayer",
		player: &playerState{
			GUID:   1,
			Map:    AVMapID,
			X:      553.0,
			Y:      -78.0,
			Z:      52.0,
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
			Map:    AVMapID,
			X:      553.0,
			Y:      -78.0,
			Z:      52.0,
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
			_, err := c.Read(buf)
			if err != nil {
				return
			}
		}
	}
	go drain(c1Conn)
	go drain(c2Conn)

	return srv, allySess, hordeSess
}

func TestAlteracValley_InitialStateAndWorldStates(t *testing.T) {
	srv, allySess, _ := setupAVTestServer(t)
	av := srv.getOrCreateAVState(AVMapID)

	if av.AllianceReinforcements != 600 || av.HordeReinforcements != 600 {
		t.Fatalf("expected 600 reinforcements, got A=%d H=%d", av.AllianceReinforcements, av.HordeReinforcements)
	}

	// Test Initial Owners
	// Alliance: First Aid (0), Stormpike (1), Stoneheart Grave (2), Dun Baldar S (7), Dun Baldar N (8), Icewing (9), Stoneheart Bunker (10)
	for _, id := range []uint32{0, 1, 2, 7, 8, 9, 10} {
		if av.Nodes[id].Owner != AVTeamAlliance {
			t.Fatalf("node %d (%s) expected Alliance owner, got %d", id, avNodeNames[id], av.Nodes[id].Owner)
		}
	}
	// Snowfall: Neutral (3)
	if av.Nodes[AVNodeSnowfallGrave].Owner != AVTeamNeutral {
		t.Fatalf("Snowfall expected Neutral owner, got %d", av.Nodes[AVNodeSnowfallGrave].Owner)
	}
	// Horde: Iceblood Grave (4), Frostwolf Grave (5), Relief Hut (6), Iceblood Tower (11), Tower Point (12), FW East (13), FW West (14)
	for _, id := range []uint32{4, 5, 6, 11, 12, 13, 14} {
		if av.Nodes[id].Owner != AVTeamHorde {
			t.Fatalf("node %d (%s) expected Horde owner, got %d", id, avNodeNames[id], av.Nodes[id].Owner)
		}
	}

	// Test Initial Mines
	if av.Mines[AVNorthMine].Owner != AVTeamNeutral || av.Mines[AVSouthMine].Owner != AVTeamNeutral {
		t.Fatalf("mines expected Neutral, got N=%d S=%d", av.Mines[AVNorthMine].Owner, av.Mines[AVSouthMine].Owner)
	}

	// Verify sendAVInitialWorldStates does not panic
	srv.sendAVInitialWorldStates(allySess)
}

func TestAlteracValley_PlayerDeathReinforcementsLoss(t *testing.T) {
	srv, allySess, hordeSess := setupAVTestServer(t)
	av := srv.getOrCreateAVState(AVMapID)

	// Alliance player dies -> Alliance loses 1 reinforcement
	srv.handleAVPlayerDeath(allySess)
	if av.AllianceReinforcements != 599 {
		t.Fatalf("expected 599 Alliance reinforcements after death, got %d", av.AllianceReinforcements)
	}

	// Horde player dies -> Horde loses 1 reinforcement
	srv.handleAVPlayerDeath(hordeSess)
	if av.HordeReinforcements != 599 {
		t.Fatalf("expected 599 Horde reinforcements after death, got %d", av.HordeReinforcements)
	}

	// Bring Horde reinforcements down to 1, then kill player -> Alliance wins
	av.HordeReinforcements = 1
	srv.handleAVPlayerDeath(hordeSess)
	if av.HordeReinforcements != 0 {
		t.Fatalf("expected 0 Horde reinforcements, got %d", av.HordeReinforcements)
	}
	if av.Winner != 0 { // Alliance wins
		t.Fatalf("expected Alliance victory (0), got %d", av.Winner)
	}
}

func TestAlteracValley_GeneralKillsInstantVictory(t *testing.T) {
	// Case 1: Vanndar killed -> Horde wins immediately
	srv1, _, hordeSess1 := setupAVTestServer(t)
	av1 := srv1.getOrCreateAVState(AVMapID)
	srv1.handleAVCreatureKilled(hordeSess1, AVCreatureVanndar)
	if av1.Winner != 1 {
		t.Fatalf("expected Horde victory (1) on Vanndar death, got %d", av1.Winner)
	}

	// Case 2: Drek'Thar killed -> Alliance wins immediately
	srv2, allySess2, _ := setupAVTestServer(t)
	av2 := srv2.getOrCreateAVState(AVMapID)
	srv2.handleAVCreatureKilled(allySess2, AVCreatureDrekThar)
	if av2.Winner != 0 {
		t.Fatalf("expected Alliance victory (0) on Drek'Thar death, got %d", av2.Winner)
	}
}

func TestAlteracValley_CaptainsKillsReinforcementsLoss(t *testing.T) {
	srv, allySess, hordeSess := setupAVTestServer(t)
	av := srv.getOrCreateAVState(AVMapID)

	// Slay Balinda -> Alliance loses 100 reinforcements
	srv.handleAVCreatureKilled(hordeSess, AVCreatureBalinda)
	if av.AllianceReinforcements != 500 {
		t.Fatalf("expected 500 Alliance reinforcements after Balinda killed, got %d", av.AllianceReinforcements)
	}
	if av.AllianceCaptainAlive {
		t.Fatal("expected AllianceCaptainAlive to be false")
	}

	// Killing Balinda a second time does not deduct again
	srv.handleAVCreatureKilled(hordeSess, AVCreatureBalinda)
	if av.AllianceReinforcements != 500 {
		t.Fatalf("expected 500 Alliance reinforcements on repeat kill, got %d", av.AllianceReinforcements)
	}

	// Slay Galvangar -> Horde loses 100 reinforcements
	srv.handleAVCreatureKilled(allySess, AVCreatureGalvangar)
	if av.HordeReinforcements != 500 {
		t.Fatalf("expected 500 Horde reinforcements after Galvangar killed, got %d", av.HordeReinforcements)
	}
	if av.HordeCaptainAlive {
		t.Fatal("expected HordeCaptainAlive to be false")
	}
}

func TestAlteracValley_TowerAssaultDefenseAndDestruction(t *testing.T) {
	ctx := context.Background()
	srv, allySess, hordeSess := setupAVTestServer(t)
	av := srv.getOrCreateAVState(AVMapID)
	av.CaptureDuration = 50 * time.Millisecond // fast timer for unit testing

	nodeID := AVNodeDunBaldarSouth
	// Dun Baldar South initial owner: Alliance, State: Controlled
	hordeSess.player.X = av.Nodes[nodeID].X
	hordeSess.player.Y = av.Nodes[nodeID].Y
	hordeSess.player.Z = av.Nodes[nodeID].Z

	// 1. Horde player assaults Dun Baldar South Bunker
	srv.handleAVGameObjectUse(ctx, hordeSess, 100, AVObjectBannerA)
	if av.Nodes[nodeID].State != AVNodeStateContestedHorde {
		t.Fatalf("expected ContestedHorde (2), got %d", av.Nodes[nodeID].State)
	}
	if av.Nodes[nodeID].CaptureTimer == nil {
		t.Fatal("expected burn timer to be started")
	}

	// 2. Alliance player defends the bunker before timer expires
	allySess.player.X = av.Nodes[nodeID].X
	allySess.player.Y = av.Nodes[nodeID].Y
	allySess.player.Z = av.Nodes[nodeID].Z
	srv.handleAVGameObjectUse(ctx, allySess, 100, AVObjectBannerContH)
	if av.Nodes[nodeID].State != AVNodeStateControlled {
		t.Fatalf("expected Controlled (0) after defense, got %d", av.Nodes[nodeID].State)
	}
	if av.Nodes[nodeID].Owner != AVTeamAlliance {
		t.Fatalf("expected Alliance owner, got %d", av.Nodes[nodeID].Owner)
	}
	if av.Nodes[nodeID].CaptureTimer != nil {
		t.Fatal("expected burn timer to be cancelled on defense")
	}

	// 3. Horde player assaults again and lets burn timer expire -> Bunker destroyed!
	srv.handleAVGameObjectUse(ctx, hordeSess, 100, AVObjectBannerA)
	time.Sleep(100 * time.Millisecond)

	if av.Nodes[nodeID].State != AVNodeStateDestroyed {
		t.Fatalf("expected Destroyed (3), got %d", av.Nodes[nodeID].State)
	}
	// Alliance must lose 75 reinforcements (600 - 75 = 525)
	if av.AllianceReinforcements != 525 {
		t.Fatalf("expected 525 Alliance reinforcements after bunker destroyed, got %d", av.AllianceReinforcements)
	}
}

func TestAlteracValley_GraveyardAssaultDefenseAndCapture(t *testing.T) {
	ctx := context.Background()
	srv, allySess, hordeSess := setupAVTestServer(t)
	av := srv.getOrCreateAVState(AVMapID)
	av.CaptureDuration = 50 * time.Millisecond // fast timer for unit testing

	nodeID := AVNodeIcebloodGrave // Initial owner: Horde
	allySess.player.X = av.Nodes[nodeID].X
	allySess.player.Y = av.Nodes[nodeID].Y
	allySess.player.Z = av.Nodes[nodeID].Z

	// 1. Alliance assaults Iceblood Graveyard
	srv.handleAVGameObjectUse(ctx, allySess, 200, AVObjectBannerH)
	if av.Nodes[nodeID].State != AVNodeStateContestedAlliance {
		t.Fatalf("expected ContestedAlliance (1), got %d", av.Nodes[nodeID].State)
	}

	// 2. Capture timer expires -> Fully controlled by Alliance
	time.Sleep(100 * time.Millisecond)
	if av.Nodes[nodeID].State != AVNodeStateControlled {
		t.Fatalf("expected Controlled (0), got %d", av.Nodes[nodeID].State)
	}
	if av.Nodes[nodeID].Owner != AVTeamAlliance {
		t.Fatalf("expected Alliance owner, got %d", av.Nodes[nodeID].Owner)
	}
	// Graveyards do NOT deduct reinforcements when captured (reinforcements only deduct on towers/captains/players)
	if av.HordeReinforcements != 600 {
		t.Fatalf("expected Horde reinforcements to remain 600, got %d", av.HordeReinforcements)
	}

	// 3. Horde player assaults back
	hordeSess.player.X = av.Nodes[nodeID].X
	hordeSess.player.Y = av.Nodes[nodeID].Y
	hordeSess.player.Z = av.Nodes[nodeID].Z
	srv.handleAVGameObjectUse(ctx, hordeSess, 200, AVObjectBannerA)
	if av.Nodes[nodeID].State != AVNodeStateContestedHorde {
		t.Fatalf("expected ContestedHorde (2), got %d", av.Nodes[nodeID].State)
	}
	// Timer expires -> Fully controlled by Horde again
	time.Sleep(100 * time.Millisecond)
	if av.Nodes[nodeID].State != AVNodeStateControlled || av.Nodes[nodeID].Owner != AVTeamHorde {
		t.Fatalf("expected Horde controlled, got state %d owner %d", av.Nodes[nodeID].State, av.Nodes[nodeID].Owner)
	}
}

func TestAlteracValley_MinesClaimAndTickReinforcements(t *testing.T) {
	ctx := context.Background()
	srv, allySess, _ := setupAVTestServer(t)
	av := srv.getOrCreateAVState(AVMapID)

	// Reduce Alliance reinforcements so we can observe positive generation
	av.AllianceReinforcements = 590

	// 1. Alliance claims North Mine by interacting with mine supply
	srv.handleAVGameObjectUse(ctx, allySess, 300, AVObjectMineSupplyN)
	if av.Mines[AVNorthMine].Owner != AVTeamAlliance {
		t.Fatalf("expected Alliance owner for North Mine, got %d", av.Mines[AVNorthMine].Owner)
	}

	// 2. Alliance claims South Mine via Boss Kill
	srv.handleAVCreatureKilled(allySess, AVCreatureColdtoothBoss1)
	if av.Mines[AVSouthMine].Owner != AVTeamAlliance {
		t.Fatalf("expected Alliance owner for South Mine, got %d", av.Mines[AVSouthMine].Owner)
	}

	// 3. Ticking mines adds +1 per controlled mine (+2 total)
	srv.TickAVMines(av, 45000)
	if av.AllianceReinforcements != 592 {
		t.Fatalf("expected 592 Alliance reinforcements after mine tick, got %d", av.AllianceReinforcements)
	}

	// 4. Reinforcements cannot exceed 600 max
	av.AllianceReinforcements = 599
	srv.TickAVMines(av, 45000)
	if av.AllianceReinforcements != 600 {
		t.Fatalf("expected 600 (capped), got %d", av.AllianceReinforcements)
	}
}
