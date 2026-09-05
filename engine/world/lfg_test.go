package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func setupTestLFGServer(t *testing.T) (*Server, *sql.DB, *sql.DB) {
	t.Helper()
	charDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	charDB.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE character_instance (guid INTEGER, instance INTEGER, permanent INTEGER, extendState INTEGER, PRIMARY KEY (guid, instance))",
		"CREATE TABLE instance (id INTEGER PRIMARY KEY, map INTEGER, resettime INTEGER, difficulty INTEGER, completedEncounters INTEGER, data TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER)",
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, map INTEGER, position_x REAL, position_y REAL, position_z REAL, orientation REAL)",
	} {
		if _, err := charDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	worldDB.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE access_requirement (mapId INTEGER, difficulty INTEGER, level_min INTEGER, level_max INTEGER, item_level INTEGER, item INTEGER, item2 INTEGER, quest_done_A INTEGER, quest_done_H INTEGER, completed_achievement INTEGER, quest_failed_text TEXT, comment TEXT, PRIMARY KEY (mapId, difficulty))",
		"CREATE TABLE lfg_dungeon_template (dungeonId INTEGER PRIMARY KEY, name TEXT, position_x REAL, position_y REAL, position_z REAL, orientation REAL, VerifiedBuild INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, ItemLevel INTEGER)",
	} {
		if _, err := worldDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: charDB}
	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: worldDB}

	store := wotlk.NewStore("../../data/dbc")
	srv := &Server{
		CharactersStore: charStore,
		WorldStore:      worldStore,
		Data:            store,
		Features:        &Features{LFG: NewLFGManager(true)},
		sessions:        make(map[*session]struct{}),
		groups:          make(map[uint64]*groupState),
	}

	return srv, charDB, worldDB
}

func TestLFG_PlayerLockInfoCalculation(t *testing.T) {
	srv, charDB, worldDB := setupTestLFGServer(t)
	defer charDB.Close()
	defer worldDB.Close()

	// 1. Insert a raid lock for map 533 (Naxxramas), difficulty 1 (Normal 25), resettime far in future
	futureTime := time.Now().Add(24 * time.Hour).Unix()
	_, _ = charDB.Exec("INSERT INTO instance (id, map, resettime, difficulty) VALUES (101, 533, ?, 1)", futureTime)
	_, _ = charDB.Exec("INSERT INTO character_instance (guid, instance, permanent, extendState) VALUES (1, 101, 1, 0)")

	// 2. Insert access requirement for map 631 (Icecrown Citadel) with item_level 200
	_, _ = worldDB.Exec("INSERT INTO access_requirement (mapId, difficulty, item_level) VALUES (631, 0, 200)")

	// 3. Insert equipped items for player 1 with avg item level = 150 (below 200)
	_, _ = charDB.Exec("INSERT INTO character_inventory (guid, bag, slot, item) VALUES (1, 0, 0, 5001)") // Head slot
	_, _ = charDB.Exec("INSERT INTO item_instance (guid, itemEntry, count) VALUES (5001, 90001, 1)")
	_, _ = worldDB.Exec("INSERT INTO item_template (entry, ItemLevel) VALUES (90001, 150)")

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := &session{
		server:           srv,
		conn:             serverConn,
		playerGUID:       1,
		playerLoaded:     true,
		accountExpansion: 2, // WotLK
		player: &playerState{
			GUID:   1,
			Name:   "LockTester",
			Level:  80,
			Health: 1000,
		},
	}
	srv.sessions[sess] = struct{}{}

	// Verify getLockedDungeons calculates proper statuses
	locks := sess.getLockedDungeons(1)
	if len(locks) == 0 {
		t.Fatal("expected locks to be calculated from DBC and DB")
	}

	// Dungeon 227 is Naxxramas 25-man (Map 533, Difficulty 1). Should be LFGLockStatusRaidLocked (6)
	// Entry = 227 + (2 << 24) = 0x020000E3 = 33554659
	naxx25Entry := uint32(227 + (2 << 24))
	if status, ok := locks[naxx25Entry]; !ok || status != LFGLockStatusRaidLocked {
		t.Fatalf("expected Naxxramas 25 to be raid locked (6), got status=%d, ok=%v", status, ok)
	}

	// Verify SMSG_LFG_PLAYER_INFO packet wire format
	done := make(chan struct{})
	go func() {
		sess.handleLfdPlayerLockInfoRequest(context.Background(), nil)
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if op != uint16(protocol.OpcodeSMSG_LFG_PLAYER_INFO) {
		t.Fatalf("expected SMSG_LFG_PLAYER_INFO (0x36F), got 0x%04X", op)
	}
	r := protocol.NewReader(data)
	rCount, err := r.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	// Skip random dungeons
	for i := uint8(0); i < rCount; i++ {
		_, _ = r.ReadU32() // dungeon entry
		_, _ = r.ReadU8()  // done
		_, _ = r.ReadU32() // money
		_, _ = r.ReadU32() // xp
		_, _ = r.ReadU32() // unk1
		_, _ = r.ReadU32() // unk2
		_, _ = r.ReadU8()  // item rewards count
	}
	lCount, err := r.ReadU32()
	if err != nil {
		t.Fatal(err)
	}
	if lCount != uint32(len(locks)) {
		t.Fatalf("expected %d locks in packet, got %d", len(locks), lCount)
	}
}

func TestLFG_PartyLockInfoRequest(t *testing.T) {
	srv, charDB, worldDB := setupTestLFGServer(t)
	defer charDB.Close()
	defer worldDB.Close()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess1 := &session{
		server:           srv,
		conn:             serverConn,
		playerGUID:       10,
		playerLoaded:     true,
		accountExpansion: 2,
		groupID:          55,
		player:           &playerState{GUID: 10, Name: "PartyLeader", Level: 80},
	}
	sess2 := &session{
		server:           srv,
		playerGUID:       20,
		playerLoaded:     true,
		accountExpansion: 1, // TBC expansion -> will have WotLK dungeons locked for insufficient expansion
		groupID:          55,
		player:           &playerState{GUID: 20, Name: "PartyMember", Level: 70},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	grp := &groupState{
		ID:         55,
		LeaderGUID: 10,
		Members: []groupMember{
			{GUID: 10, Name: "PartyLeader"},
			{GUID: 20, Name: "PartyMember"},
		},
	}
	srv.groups[55] = grp

	done := make(chan struct{})
	go func() {
		sess1.handleLfdPartyLockInfoRequest(context.Background(), nil)
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if op != uint16(protocol.OpcodeSMSG_LFG_PARTY_INFO) {
		t.Fatalf("expected SMSG_LFG_PARTY_INFO (0x372), got 0x%04X", op)
	}
	r := protocol.NewReader(data)
	playerCount, err := r.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	if playerCount != 1 {
		t.Fatalf("expected 1 other party member lock block, got %d", playerCount)
	}
	memberGuid, err := r.ReadU64()
	if err != nil {
		t.Fatal(err)
	}
	if memberGuid != 20 {
		t.Fatalf("expected party member GUID 20, got %d", memberGuid)
	}
	lockCount, err := r.ReadU32()
	if err != nil {
		t.Fatal(err)
	}
	if lockCount == 0 {
		t.Fatal("expected party member to have locked dungeons")
	}
}

func TestLFG_RoleCheckFlow(t *testing.T) {
	srv, charDB, worldDB := setupTestLFGServer(t)
	defer charDB.Close()
	defer worldDB.Close()

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerGUID:   100,
		playerLoaded: true,
		groupID:      99,
		player:       &playerState{GUID: 100, Name: "Leader", Level: 80},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerGUID:   200,
		playerLoaded: true,
		groupID:      99,
		player:       &playerState{GUID: 200, Name: "Member", Level: 80},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	// Drain sess1 client pipe since server broadcasts role selection to all group members
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := cConn1.Read(buf); err != nil {
				return
			}
		}
	}()

	grp := &groupState{
		ID:         99,
		LeaderGUID: 100,
		Members: []groupMember{
			{GUID: 100, Name: "Leader"},
			{GUID: 200, Name: "Member"},
		},
	}
	srv.groups[99] = grp

	// 1. Leader initiates role check with dungeon 1
	rc := srv.Features.LFG.StartRoleCheck(99, 100, []uint32{1}, LFGRoleLeader|LFGRoleTank)
	if rc == nil || rc.State != LFGRoleCheckInitializing {
		t.Fatalf("expected role check initializing, got %+v", rc)
	}

	// 2. Member selects Healer role (CMSG_LFG_SET_ROLES)
	done := make(chan struct{})
	go func() {
		sess2.handleLfgSetRoles(context.Background(), []byte{byte(LFGRoleHealer)})
		close(done)
	}()

	// Drain frames from clientConn2
	_ = cConn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	opRC, _, err := readServerFrame(cConn2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opRC != uint16(protocol.OpcodeSMSG_LFG_ROLE_CHECK_UPDATE) {
		t.Fatalf("expected SMSG_LFG_ROLE_CHECK_UPDATE (0x363), got 0x%04X", opRC)
	}

	opParty, _, err := readServerFrame(cConn2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opParty != uint16(protocol.OpcodeSMSG_LFG_UPDATE_PARTY) {
		t.Fatalf("expected SMSG_LFG_UPDATE_PARTY (0x35B), got 0x%04X", opParty)
	}

	opChosen, _, err := readServerFrame(cConn2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opChosen != uint16(protocol.OpcodeSMSG_LFG_ROLE_CHOSEN) {
		t.Fatalf("expected SMSG_LFG_ROLE_CHOSEN (0x2BB), got 0x%04X", opChosen)
	}
	<-done

	// Verify both answered -> rolecheck completed
	updatedRC := srv.Features.LFG.GetRoleCheck(99)
	if updatedRC == nil || updatedRC.State != LFGRoleCheckFinished {
		t.Fatalf("expected role check finished, got %+v", updatedRC)
	}
}

func TestLFG_ProposalAcceptAndDecline(t *testing.T) {
	srv, charDB, worldDB := setupTestLFGServer(t)
	defer charDB.Close()
	defer worldDB.Close()

	cConn1, sConn1 := net.Pipe()
	defer cConn1.Close()
	defer sConn1.Close()

	cConn2, sConn2 := net.Pipe()
	defer cConn2.Close()
	defer sConn2.Close()

	// Drain client pipes in background so session packet sends do not block
	drain := func(c net.Conn) {
		buf := make([]byte, 4096)
		for {
			if _, err := c.Read(buf); err != nil {
				return
			}
		}
	}
	go drain(cConn1)
	go drain(cConn2)

	sess1 := &session{
		server:       srv,
		conn:         sConn1,
		playerGUID:   10,
		playerLoaded: true,
		player:       &playerState{GUID: 10, Name: "Player1", Level: 80, Map: 0, X: 100, Y: 200, Z: 50, Orientation: 1.5},
	}
	sess2 := &session{
		server:       srv,
		conn:         sConn2,
		playerGUID:   20,
		playerLoaded: true,
		player:       &playerState{GUID: 20, Name: "Player2", Level: 80, Map: 0, X: 110, Y: 210, Z: 50, Orientation: 1.5},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	// Scenario A: Player 1 accepts, Player 2 declines -> Proposal Fails
	prop1 := srv.Features.LFG.CreateProposal(18, 0, 10, map[uint64]uint8{
		10: LFGRoleTank,
		20: LFGRoleHealer,
	})

	// Player 1 accepts
	p1Payload := protocol.NewBuffer(5)
	p1Payload.WriteU32(prop1.ID)
	p1Payload.WriteU8(1) // accept
	sess1.handleLfgProposalResult(context.Background(), p1Payload.Bytes())

	// Proposal is still active, waiting for player 2
	curProp := srv.Features.LFG.GetProposal(prop1.ID)
	if curProp == nil || curProp.Players[10].Accept != LFGAnswerAgree {
		t.Fatalf("expected Player 1 agree recorded, got %+v", curProp)
	}

	// Player 2 declines
	p2Payload := protocol.NewBuffer(5)
	p2Payload.WriteU32(prop1.ID)
	p2Payload.WriteU8(0) // decline
	sess2.handleLfgProposalResult(context.Background(), p2Payload.Bytes())

	// Proposal should be removed from active proposals and marked failed
	if srv.Features.LFG.GetProposal(prop1.ID) != nil {
		t.Fatal("expected proposal to be removed upon decline")
	}

	// Scenario B: Both players accept -> Proposal Succeeds & Teleport triggers
	// Add entrance coords in lfg_dungeon_template for dungeon 18
	_, _ = worldDB.Exec("INSERT INTO lfg_dungeon_template (dungeonId, name, position_x, position_y, position_z, orientation) VALUES (18, 'Scarlet Monastery', 1688.0, 1053.0, 18.0, 0.5)")

	prop2 := srv.Features.LFG.CreateProposal(18, 0, 10, map[uint64]uint8{
		10: LFGRoleTank,
		20: LFGRoleHealer,
	})

	// Both accept
	p1Payload2 := protocol.NewBuffer(5)
	p1Payload2.WriteU32(prop2.ID)
	p1Payload2.WriteU8(1)
	sess1.handleLfgProposalResult(context.Background(), p1Payload2.Bytes())

	p2Payload2 := protocol.NewBuffer(5)
	p2Payload2.WriteU32(prop2.ID)
	p2Payload2.WriteU8(1)
	sess2.handleLfgProposalResult(context.Background(), p2Payload2.Bytes())

	// Verify players teleported to dungeon coords and entry point saved
	if sess1.player.X != 1688.0 || sess1.player.Y != 1053.0 {
		t.Fatalf("expected Player 1 teleported to dungeon, got X=%f Y=%f", sess1.player.X, sess1.player.Y)
	}
	if sess1.player.LfgEntryPointX != 100 || sess1.player.LfgEntryPointY != 200 {
		t.Fatalf("expected Player 1 entry point preserved, got X=%f Y=%f", sess1.player.LfgEntryPointX, sess1.player.LfgEntryPointY)
	}
}

func TestLFG_Teleport(t *testing.T) {
	srv, charDB, worldDB := setupTestLFGServer(t)
	defer charDB.Close()
	defer worldDB.Close()

	_, _ = worldDB.Exec("INSERT INTO lfg_dungeon_template (dungeonId, name, position_x, position_y, position_z, orientation) VALUES (18, 'Scarlet Monastery', 1688.0, 1053.0, 18.0, 0.5)")

	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		groupID:      10,
		player: &playerState{
			GUID:             1,
			Name:             "Teleporter",
			Health:           100,
			Map:              0,
			X:                -8949.95,
			Y:                512.28,
			Z:                96.35,
			LfgEntryPointMap: 0,
			LfgEntryPointX:   -8949.95,
			LfgEntryPointY:   512.28,
			LfgEntryPointZ:   96.35,
		},
	}
	srv.sessions[sess] = struct{}{}
	srv.groups[10] = &groupState{
		ID:           10,
		LeaderGUID:   1,
		IsLFG:        true,
		LFGDungeonID: 18,
		Members:      []groupMember{{GUID: 1, Name: "Teleporter"}},
	}

	// 1. Teleport when dead -> denied with LFGTeleportErrorPlayerDead (1)
	sess.player.Health = 0
	doneDead := make(chan struct{})
	go func() {
		sess.handleLfgTeleport(context.Background(), []byte{0}) // teleport in
		close(doneDead)
	}()
	_ = cConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opDead, dataDead, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneDead
	if opDead != uint16(protocol.OpcodeSMSG_LFG_TELEPORT_DENIED) {
		t.Fatalf("expected SMSG_LFG_TELEPORT_DENIED, got 0x%04X", opDead)
	}
	errDead, _ := protocol.NewReader(dataDead).ReadU32()
	if errDead != LFGTeleportErrorPlayerDead {
		t.Fatalf("expected error LFGTeleportErrorPlayerDead (1), got %d", errDead)
	}

	// 2. Teleport without LFG group -> denied with LFGTeleportErrorInvalidLocation (6)
	sess.player.Health = 100
	sess.groupID = 0
	doneInvalid := make(chan struct{})
	go func() {
		sess.handleLfgTeleport(context.Background(), []byte{0})
		close(doneInvalid)
	}()
	_ = cConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opInvalid, dataInvalid, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneInvalid
	if opInvalid != uint16(protocol.OpcodeSMSG_LFG_TELEPORT_DENIED) {
		t.Fatalf("expected SMSG_LFG_TELEPORT_DENIED, got 0x%04X", opInvalid)
	}
	errInvalid, _ := protocol.NewReader(dataInvalid).ReadU32()
	if errInvalid != LFGTeleportErrorInvalidLocation {
		t.Fatalf("expected error LFGTeleportErrorInvalidLocation (6), got %d", errInvalid)
	}

	// 3. Teleport in with LFG group -> teleports to dungeon entrance
	sess.groupID = 10
	grp := &groupState{
		ID:           10,
		LeaderGUID:   1,
		IsLFG:        true,
		LFGDungeonID: 18,
		Members:      []groupMember{{GUID: 1, Name: "Teleporter"}},
	}
	srv.groups[10] = grp

	// Drain cConn in background for teleport packet writes
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := cConn.Read(buf); err != nil {
				return
			}
		}
	}()

	sess.handleLfgTeleport(context.Background(), []byte{0}) // in
	if sess.player.X != 1688.0 || sess.player.Y != 1053.0 {
		t.Fatalf("expected teleport in to 1688, 1053, got X=%f Y=%f", sess.player.X, sess.player.Y)
	}

	// 4. Teleport out -> returns to saved entry point
	sess.handleLfgTeleport(context.Background(), []byte{1}) // out
	if sess.player.X != -8949.95 || sess.player.Y != 512.28 {
		t.Fatalf("expected teleport out to -8949.95, 512.28, got X=%f Y=%f", sess.player.X, sess.player.Y)
	}
}

func TestLFGTeleport_Denials(t *testing.T) {
	srv, charDB, worldDB := setupTestLFGServer(t)
	defer charDB.Close()
	defer worldDB.Close()

	_, _ = worldDB.Exec("INSERT INTO lfg_dungeon_template (dungeonId, name, position_x, position_y, position_z, orientation) VALUES (18, 'Scarlet Monastery', 1688.0, 1053.0, 18.0, 0.5)")

	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		groupID:      10,
		breathTimer:  -1,
		fatigueTimer: -1,
		auras:        make(map[uint32]struct{}),
		player: &playerState{
			GUID:      1,
			Name:      "DenialTester",
			Health:    100,
			MaxHealth: 100,
			Map:       0,
			X:         100.0,
			Y:         200.0,
			Z:         30.0,
		},
	}
	srv.sessions[sess] = struct{}{}
	srv.groups[10] = &groupState{
		ID:           10,
		LeaderGUID:   1,
		IsLFG:        true,
		LFGDungeonID: 18,
		Members:      []groupMember{{GUID: 1, Name: "DenialTester"}},
	}

	checkDenied := func(expectedErr uint32, desc string) {
		done := make(chan struct{})
		go func() {
			sess.handleLfgTeleport(context.Background(), []byte{0}) // in
			close(done)
		}()
		_ = cConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		op, data, err := readServerFrame(cConn, nil)
		if err != nil {
			t.Fatalf("%s: failed to read frame: %v", desc, err)
		}
		<-done
		if op != uint16(protocol.OpcodeSMSG_LFG_TELEPORT_DENIED) {
			t.Fatalf("%s: expected SMSG_LFG_TELEPORT_DENIED, got 0x%04X", desc, op)
		}
		code, _ := protocol.NewReader(data).ReadU32()
		if code != expectedErr {
			t.Fatalf("%s: expected error %d, got %d", desc, expectedErr, code)
		}
	}

	// 1. Ghost denial
	sess.player.Health = 1
	sess.player.PlayerFlags |= playerFlagGhost
	checkDenied(LFGTeleportErrorPlayerDead, "ghost")

	// Restore
	sess.player.PlayerFlags &^= playerFlagGhost
	sess.player.Health = 100

	// 2. Falling denial
	sess.isFalling = true
	checkDenied(LFGTeleportErrorFalling, "falling")
	sess.isFalling = false

	// 3. Fatigue denial (in dark water)
	sess.inDarkWater = true
	checkDenied(LFGTeleportErrorFatigue, "dark water")
	sess.inDarkWater = false

	// 4. Fatigue denial (outside dark water, regenerating timer)
	sess.fatigueTimer = 30000
	checkDenied(LFGTeleportErrorFatigue, "fatigue regenerating")
	sess.fatigueTimer = -1

	// 5. In vehicle denial
	sess.player.VehicleGUID = 12345
	checkDenied(LFGTeleportErrorInVehicle, "vehicle")
	sess.player.VehicleGUID = 0

	// 6. Freeze debuff denial (spell 9454)
	sess.auras[9454] = struct{}{}
	checkDenied(LFGTeleportErrorInvalidLocation, "freeze aura")
	delete(sess.auras, 9454)

	// 7. Taxi flight cancelled upon successful teleport in
	sess.inFlight = true
	sess.player.MountDisplayID = 200
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := cConn.Read(buf); err != nil {
				return
			}
		}
	}()
	sess.handleLfgTeleport(context.Background(), []byte{0})
	if sess.inFlight {
		t.Fatal("expected inFlight to be false after teleport in")
	}
	if sess.player.MountDisplayID != 0 {
		t.Fatalf("expected MountDisplayID to be reset to 0, got %d", sess.player.MountDisplayID)
	}
	if sess.player.X != 1688.0 || sess.player.Y != 1053.0 {
		t.Fatalf("expected teleport in to 1688, 1053, got %f, %f", sess.player.X, sess.player.Y)
	}
}

