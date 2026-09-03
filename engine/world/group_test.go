package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
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

func TestRequestPartyMemberStats(t *testing.T) {
	srv := newGroupTestServer()
	alice := addSess(srv, 1, "Alice")
	bob := addSess(srv, 2, "Bob")
	bob.player.Class = 1 // Warrior (Rage = 1)
	bob.player.Health = 500
	bob.player.MaxHealth = 1000
	bob.player.Powers[1] = 75
	bob.player.MaxPowers[1] = 100
	bob.player.Level = 45
	bob.player.Zone = 12
	bob.player.X = 100.0
	bob.player.Y = 200.0

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	alice.conn = serverConn

	payload := protocol.NewBuffer(8)
	payload.WriteU64(2)

	done := make(chan struct{})
	go func() {
		if !alice.handleRequestPartyMemberStats(context.Background(), payload.Bytes()) {
			t.Error("handleRequestPartyMemberStats failed")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, statsPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if opcode != uint16(protocol.OpcodeSMSG_PARTY_MEMBER_STATS) {
		t.Fatalf("expected SMSG_PARTY_MEMBER_STATS (0x7E), got %x", opcode)
	}
	r := protocol.NewReader(statsPayload)
	guid, _ := r.ReadPackedGUID()
	mask, _ := r.ReadU32()
	status, _ := r.ReadU16()
	hp, _ := r.ReadU32()
	maxHp, _ := r.ReadU32()
	powerType, _ := r.ReadU8()
	power, _ := r.ReadU16()
	maxPower, _ := r.ReadU16()
	lvl, _ := r.ReadU16()
	zone, _ := r.ReadU16()
	x, _ := r.ReadU16()
	y, _ := r.ReadU16()

	if guid != 2 || mask != 0x000001FF || status != 1 || hp != 500 || maxHp != 1000 || powerType != 1 || power != 75 || maxPower != 100 || lvl != 45 || zone != 12 || x != 100 || y != 200 {
		t.Fatalf("unexpected stats: guid=%d mask=%x status=%d hp=%d maxHp=%d powerType=%d power=%d maxPower=%d lvl=%d zone=%d x=%d y=%d",
			guid, mask, status, hp, maxHp, powerType, power, maxPower, lvl, zone, x, y)
	}
}

func TestRequestRaidInfoAndExtend(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE instance (id INTEGER PRIMARY KEY, map INTEGER, resettime INTEGER, difficulty INTEGER, completedEncounters INTEGER, data TEXT)",
		"CREATE TABLE character_instance (guid INTEGER, instance INTEGER, permanent INTEGER, extendState INTEGER, PRIMARY KEY (guid, instance))",
		"INSERT INTO instance VALUES (10, 534, 2000000000, 1, 0, '')", // Map 534 (Hyjal), 25-man
		"INSERT INTO character_instance VALUES (1, 10, 1, 0)",          // bound to player 1
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := newGroupTestServer()
	srv.CharactersStore = store
	sess := addSess(srv, 1, "Alice")

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	sess.conn = serverConn

	// 1. Request Raid Info
	done := make(chan struct{})
	go func() {
		if !sess.handleRequestRaidInfo(context.Background()) {
			t.Error("handleRequestRaidInfo failed")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if opcode != uint16(protocol.OpcodeSMSG_RAID_INSTANCE_INFO) {
		t.Fatalf("expected SMSG_RAID_INSTANCE_INFO, got %x", opcode)
	}
	r := protocol.NewReader(payload)
	count, _ := r.ReadU32()
	if count != 1 {
		t.Fatalf("expected 1 lock, got %d", count)
	}
	mapID, _ := r.ReadU32()
	diff, _ := r.ReadU32()
	instID, _ := r.ReadU64()
	notExpired, _ := r.ReadU8()
	extended, _ := r.ReadU8()
	rem, _ := r.ReadU32()

	if mapID != 534 || diff != 1 || instID != 10 || notExpired != 1 || extended != 0 || rem == 0 {
		t.Fatalf("unexpected lock info: map=%d diff=%d inst=%d notExpired=%d extended=%d rem=%d", mapID, diff, instID, notExpired, extended, rem)
	}

	// 2. Extend lock
	extPayload := protocol.NewBuffer(6)
	extPayload.WriteU32(534)
	extPayload.WriteU8(1)
	extPayload.WriteU8(1) // extend = 1

	doneExt := make(chan struct{})
	go func() {
		if !sess.handleSetSavedInstanceExtend(context.Background(), extPayload.Bytes()) {
			t.Error("handleSetSavedInstanceExtend failed")
		}
		close(doneExt)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode2, payload2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneExt

	if opcode2 != uint16(protocol.OpcodeSMSG_RAID_INSTANCE_INFO) {
		t.Fatalf("expected refreshed SMSG_RAID_INSTANCE_INFO, got %x", opcode2)
	}
	r2 := protocol.NewReader(payload2)
	_, _ = r2.ReadU32() // count = 1
	_, _ = r2.ReadU32() // mapID
	_, _ = r2.ReadU32() // diff
	_, _ = r2.ReadU64() // instID
	_, _ = r2.ReadU8()  // notExpired
	ext2, _ := r2.ReadU8()
	if ext2 != 1 {
		t.Fatalf("expected extended=1, got %d", ext2)
	}
}
