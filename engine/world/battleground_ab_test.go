package world

import (
	"context"
	"net"
	"testing"
	"time"
)

func newABTestSession(t *testing.T, srv *Server, guid uint64, race uint8, name string, x, y, z float32) (*session, net.Conn) {
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
			Map:   ABMapID,
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

func setupABServer() *Server {
	return &Server{
		sessions:           make(map[*session]struct{}),
		dynamicGameObjects: make(map[uint64]*dynamicGameObjectState),
		abState:            make(map[uint32]*abBattlegroundState),
	}
}

func TestABBannerEntryMapping(t *testing.T) {
	// Test all 25 banner entries
	for node := uint32(0); node < ABNodeMax; node++ {
		neutralEntry := getABBannerEntry(node, ABNodeStateNeutral)
		if !isABBanner(neutralEntry) {
			t.Fatalf("expected entry %d to be recognized as AB banner", neutralEntry)
		}
		nodeID, ok := getABNodeIDFromBannerEntry(neutralEntry)
		if !ok || nodeID != node {
			t.Fatalf("expected nodeID %d, got %d (ok=%v)", node, nodeID, ok)
		}

		contAllyEntry := getABBannerEntry(node, ABNodeStateContestedAlliance)
		if !isABBanner(contAllyEntry) {
			t.Fatalf("expected entry %d to be recognized as AB banner", contAllyEntry)
		}
		nodeID, ok = getABNodeIDFromBannerEntry(contAllyEntry)
		if !ok || nodeID != node {
			t.Fatalf("expected nodeID %d, got %d", node, nodeID)
		}

		hordeContEntry := getABBannerEntry(node, ABNodeStateContestedHorde)
		if !isABBanner(hordeContEntry) {
			t.Fatalf("expected entry %d to be recognized as AB banner", hordeContEntry)
		}
		nodeID, ok = getABNodeIDFromBannerEntry(hordeContEntry)
		if !ok || nodeID != node {
			t.Fatalf("expected nodeID %d, got %d", node, nodeID)
		}

		allyCtrlEntry := getABBannerEntry(node, ABNodeStateControlledAlliance)
		if !isABBanner(allyCtrlEntry) {
			t.Fatalf("expected entry %d to be recognized as AB banner", allyCtrlEntry)
		}
		nodeID, ok = getABNodeIDFromBannerEntry(allyCtrlEntry)
		if !ok || nodeID != node {
			t.Fatalf("expected nodeID %d, got %d", node, nodeID)
		}

		hordeCtrlEntry := getABBannerEntry(node, ABNodeStateControlledHorde)
		if !isABBanner(hordeCtrlEntry) {
			t.Fatalf("expected entry %d to be recognized as AB banner", hordeCtrlEntry)
		}
		nodeID, ok = getABNodeIDFromBannerEntry(hordeCtrlEntry)
		if !ok || nodeID != node {
			t.Fatalf("expected nodeID %d, got %d", node, nodeID)
		}
	}

	// Non-banner entry
	if isABBanner(12345) {
		t.Fatal("unexpected true for entry 12345")
	}
	if _, ok := getABNodeIDFromBannerEntry(12345); ok {
		t.Fatal("unexpected true for entry 12345")
	}
}

func TestABNodeCaptureLifecycle(t *testing.T) {
	srv := setupABServer()
	ab := srv.getOrCreateABState(ABMapID)
	ab.CaptureDuration = 10 * time.Millisecond // fast timer for testing

	ctx := context.Background()

	// Alliance player (Human, Race 1) at Stables (1166.7, 1200.1, -56.7)
	allySess, _ := newABTestSession(t, srv, 10, 1, "AllianceHero", 1166.7, 1200.1, -56.7)
	// Horde player (Orc, Race 2) at Stables
	hordeSess, _ := newABTestSession(t, srv, 20, 2, "HordeHero", 1166.7, 1200.1, -56.7)

	stablesNeutralEntry := getABBannerEntry(ABNodeStables, ABNodeStateNeutral)

	// 1. Alliance clicks neutral Stables banner -> Contested Alliance
	handled := srv.handleABBannerUse(ctx, allySess, 100, stablesNeutralEntry)
	if !handled {
		t.Fatal("expected handleABBannerUse to return true")
	}
	if ab.Nodes[ABNodeStables].State != ABNodeStateContestedAlliance {
		t.Fatalf("expected Stables state Contested Alliance, got %d", ab.Nodes[ABNodeStables].State)
	}
	if ab.AllianceBasesCount != 0 {
		t.Fatalf("expected Alliance bases count 0 while contested, got %d", ab.AllianceBasesCount)
	}

	// 2. Wait for fast capture timer to expire -> Controlled Alliance
	time.Sleep(30 * time.Millisecond)
	if ab.Nodes[ABNodeStables].State != ABNodeStateControlledAlliance {
		t.Fatalf("expected Stables state Controlled Alliance after timer, got %d", ab.Nodes[ABNodeStables].State)
	}
	if ab.AllianceBasesCount != 1 {
		t.Fatalf("expected Alliance bases count 1, got %d", ab.AllianceBasesCount)
	}

	// 3. Horde assaults Alliance-controlled Stables -> Contested Horde
	stablesAllyEntry := getABBannerEntry(ABNodeStables, ABNodeStateControlledAlliance)
	handled = srv.handleABBannerUse(ctx, hordeSess, 101, stablesAllyEntry)
	if !handled {
		t.Fatal("expected handleABBannerUse to return true")
	}
	if ab.Nodes[ABNodeStables].State != ABNodeStateContestedHorde {
		t.Fatalf("expected Stables state Contested Horde, got %d", ab.Nodes[ABNodeStables].State)
	}
	// Alliance immediately loses the base count
	if ab.AllianceBasesCount != 0 {
		t.Fatalf("expected Alliance bases count to decrease to 0, got %d", ab.AllianceBasesCount)
	}

	// 4. Alliance defends before timer expires -> returns immediately to Controlled Alliance!
	stablesHordeContEntry := getABBannerEntry(ABNodeStables, ABNodeStateContestedHorde)
	handled = srv.handleABBannerUse(ctx, allySess, 102, stablesHordeContEntry)
	if !handled {
		t.Fatal("expected handleABBannerUse to return true")
	}
	if ab.Nodes[ABNodeStables].State != ABNodeStateControlledAlliance {
		t.Fatalf("expected Stables to return immediately to Controlled Alliance on defense, got %d", ab.Nodes[ABNodeStables].State)
	}
	if ab.AllianceBasesCount != 1 {
		t.Fatalf("expected Alliance bases count back to 1, got %d", ab.AllianceBasesCount)
	}

	// 5. Out of range check (player too far from banner)
	farSess, _ := newABTestSession(t, srv, 30, 2, "FarHorde", 500.0, 500.0, 0.0)
	handledFar := srv.handleABBannerUse(ctx, farSess, 103, stablesAllyEntry)
	if !handledFar {
		t.Fatal("expected handleABBannerUse to return true even if out of range")
	}
	// State should NOT have changed
	if ab.Nodes[ABNodeStables].State != ABNodeStateControlledAlliance {
		t.Fatal("expected state unchanged when player is out of range")
	}
}

func TestABResourceAccumulation(t *testing.T) {
	srv := setupABServer()
	ab := srv.getOrCreateABState(ABMapID)

	// 1. Alliance controls 1 base (10 pts every 12 sec)
	ab.AllianceBasesCount = 1

	// Tick 6 seconds (6000ms) -> 0 points
	srv.TickResources(ab, 6000)
	if ab.AllianceResources != 0 {
		t.Fatalf("expected 0 resources after 6s, got %d", ab.AllianceResources)
	}

	// Tick another 6 seconds (12000ms total) -> 10 points
	srv.TickResources(ab, 6000)
	if ab.AllianceResources != 10 {
		t.Fatalf("expected 10 resources after 12s, got %d", ab.AllianceResources)
	}

	// 2. Horde controls 3 bases (10 pts every 6 sec)
	ab.AllianceBasesCount = 0
	ab.HordeBasesCount = 3
	// Tick 18 seconds -> 3 ticks of 10 = 30 points
	srv.TickResources(ab, 18000)
	if ab.HordeResources != 30 {
		t.Fatalf("expected 30 resources after 18s with 3 bases, got %d", ab.HordeResources)
	}

	// 3. 5-base all-cap (30 pts every 1 sec)
	ab.AllianceBasesCount = 5
	ab.AllianceAccumMs = 0
	// Tick 10 seconds -> 10 * 30 = 300 points (+ 10 existing = 310)
	srv.TickResources(ab, 10000)
	if ab.AllianceResources != 310 {
		t.Fatalf("expected 310 resources, got %d", ab.AllianceResources)
	}

	// 4. Win condition (1600 points cap)
	ab.AllianceResources = 1590
	srv.TickResources(ab, 1000) // 1 second with 5 bases = +30 points -> 1620 -> capped at 1600
	if ab.AllianceResources != 1600 {
		t.Fatalf("expected resources capped at 1600, got %d", ab.AllianceResources)
	}
	if ab.Winner != 0 {
		t.Fatalf("expected Alliance winner (0), got %d", ab.Winner)
	}

	// Further ticks should do nothing once match has a winner
	srv.TickResources(ab, 10000)
	if ab.AllianceResources != 1600 {
		t.Fatalf("expected resources unchanged after win, got %d", ab.AllianceResources)
	}
}
