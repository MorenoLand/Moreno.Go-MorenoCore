package world

import (
	"context"
	"testing"
)

func newGroupTestServer() *Server {
	return &Server{
		sessions: make(map[*session]struct{}),
		groups:   make(map[uint64]*groupState),
	}
}

func addSess(srv *Server, guid uint64, name string) *session {
	s := &session{
		server:       srv,
		playerGUID:   guid,
		playerLoaded: true,
		player:       &playerState{GUID: guid, Name: name, Level: 10},
	}
	srv.sessionsMu.Lock()
	srv.sessions[s] = struct{}{}
	srv.sessionsMu.Unlock()
	return s
}

// nilWrite satisfies write without a real net.Conn.
func (s *session) testWrite(_ uint16, _ []byte, _ bool) error { return nil }

func TestGroupInviteAcceptLeave(t *testing.T) {
	srv := newGroupTestServer()
	alice := addSess(srv, 1, "Alice")
	bob := addSess(srv, 2, "Bob")

	ctx := context.Background()

	// Alice invites Bob
	// Manually set up invite since we don't have a real conn
	bob.pendingGroupLeader = alice.playerGUID

	// Bob accepts
	if !alice.handleGroupAccept(ctx, nil) {
		// alice is receiver here: swap
	}
	bob.pendingGroupLeader = alice.playerGUID
	if !bob.handleGroupAccept(ctx, nil) {
		t.Fatal("bob accept failed")
	}

	if bob.groupID == 0 {
		t.Fatal("bob should be in a group")
	}
	if alice.groupID != bob.groupID {
		t.Fatal("alice and bob should be in the same group")
	}

	srv.groupsMu.RLock()
	g := srv.groups[alice.groupID]
	srv.groupsMu.RUnlock()
	if g == nil {
		t.Fatal("group not found in server")
	}
	if len(g.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(g.Members))
	}
	if g.LeaderGUID != alice.playerGUID {
		t.Fatalf("expected Alice (%d) to be leader, got %d", alice.playerGUID, g.LeaderGUID)
	}

	// Bob leaves
	if !bob.handleGroupDisband(ctx, nil) {
		t.Fatal("bob disband failed")
	}

	if bob.groupID != 0 {
		t.Fatal("bob should not be in a group after leaving")
	}
	// Group should be dissolved (only 1 member = Alice, then disband)
	if alice.groupID != 0 {
		t.Fatal("alice should not be in a group after it was dissolved")
	}
}

func TestGroupSetLeaderAndDisband(t *testing.T) {
	srv := newGroupTestServer()
	alice := addSess(srv, 1, "Alice")
	bob := addSess(srv, 2, "Bob")
	charlie := addSess(srv, 3, "Charlie")

	ctx := context.Background()

	// Manually create a group
	g := &groupState{
		ID:            newGroupID(),
		LeaderGUID:    alice.playerGUID,
		LootThreshold: 2,
		DungeonDiff:   1,
		RaidDiff:      1,
		Members: []groupMember{
			{GUID: alice.playerGUID, Name: "Alice"},
			{GUID: bob.playerGUID, Name: "Bob"},
			{GUID: charlie.playerGUID, Name: "Charlie"},
		},
	}
	srv.groupsMu.Lock()
	srv.groups[g.ID] = g
	srv.groupsMu.Unlock()
	alice.groupID = g.ID
	bob.groupID = g.ID
	charlie.groupID = g.ID

	// Alice (leader) sets Bob as new leader
	payload := make([]byte, 8)
	putU64LE(payload, bob.playerGUID)
	if !alice.handleGroupSetLeader(ctx, payload) {
		t.Fatal("set leader failed")
	}
	srv.groupsMu.RLock()
	if g.LeaderGUID != bob.playerGUID {
		t.Fatalf("expected Bob (%d) as leader, got %d", bob.playerGUID, g.LeaderGUID)
	}
	srv.groupsMu.RUnlock()

	// Bob (now leader) disbands
	if !bob.handleGroupDisband(ctx, nil) {
		t.Fatal("disband failed")
	}
	if alice.groupID != 0 || bob.groupID != 0 || charlie.groupID != 0 {
		t.Fatal("all members should have left the group after disband")
	}
}

func TestGroupRaidConvert(t *testing.T) {
	srv := newGroupTestServer()
	alice := addSess(srv, 1, "Alice")
	bob := addSess(srv, 2, "Bob")

	g := &groupState{
		ID:         newGroupID(),
		LeaderGUID: alice.playerGUID,
		Members: []groupMember{
			{GUID: alice.playerGUID, Name: "Alice"},
			{GUID: bob.playerGUID, Name: "Bob"},
		},
	}
	srv.groupsMu.Lock()
	srv.groups[g.ID] = g
	srv.groupsMu.Unlock()
	alice.groupID = g.ID
	bob.groupID = g.ID

	ctx := context.Background()
	if !alice.handleGroupRaidConvert(ctx, nil) {
		t.Fatal("raid convert failed")
	}
	srv.groupsMu.RLock()
	if !g.IsRaid {
		t.Fatal("group should be raid after convert")
	}
	srv.groupsMu.RUnlock()
}

func TestGroupDecline(t *testing.T) {
	srv := newGroupTestServer()
	alice := addSess(srv, 1, "Alice")
	bob := addSess(srv, 2, "Bob")

	ctx := context.Background()
	bob.pendingGroupLeader = alice.playerGUID

	// Bob declines
	if !bob.handleGroupDecline(ctx, nil) {
		t.Fatal("decline failed")
	}
	if bob.pendingGroupLeader != 0 {
		t.Fatal("pendingGroupLeader should be 0 after decline")
	}
}

// putU64LE writes a uint64 little-endian to a byte slice.
func putU64LE(b []byte, v uint64) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}
