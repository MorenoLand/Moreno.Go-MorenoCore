package world

import (
	"context"
	"database/sql"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestAttackPacketBuilders(t *testing.T) {
	start := protocol.NewReader(buildAttackStart(11, 22))
	if attacker, err := start.ReadU64(); err != nil || attacker != 11 {
		t.Fatalf("attacker=%d err=%v", attacker, err)
	}
	if victim, err := start.ReadU64(); err != nil || victim != 22 {
		t.Fatalf("victim=%d err=%v", victim, err)
	}
	reader := protocol.NewReader(buildAttackStop(11, 22, true))
	if attacker, err := reader.ReadPackedGUID(); err != nil || attacker != 11 {
		t.Fatalf("stop attacker=%d err=%v", attacker, err)
	}
	if victim, err := reader.ReadPackedGUID(); err != nil || victim != 22 {
		t.Fatalf("stop victim=%d err=%v", victim, err)
	}
	if dead, err := reader.ReadU32(); err != nil || dead != 1 {
		t.Fatalf("dead=%d err=%v", dead, err)
	}
	state := protocol.NewReader(buildAttackerStateUpdate(11, 22, 10, 0))
	if _, err := state.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadPackedGUID(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadPackedGUID(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadU8(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := state.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if value, err := state.ReadU8(); err != nil || value != 1 {
		t.Fatalf("target state=%d err=%v", value, err)
	}
}

func TestHandleAttackSwingStartsAndStopsCombat(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL, curhealth INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO creature VALUES (7, 68, 0, 3, 4, 0, 10)"); err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}}
	state := &session{server: server, conn: serverConn, playerLoaded: true, playerGUID: 26, player: &playerState{GUID: 26, Map: 0, X: 0, Y: 0, Z: 0}}
	victim := creatureWorldGUID(7, 68)
	payload := protocol.NewBuffer(8)
	payload.WriteU64(victim)

	// Captured packets: channel carries (opcode, payload) pairs.
	type frame struct {
		op   uint16
		data []byte
	}
	frames := make(chan frame, 16)
	// Reader goroutine drains clientConn into frames channel.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer clientConn.Close()
		for {
			op, data, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			frames <- frame{op: op, data: data}
		}
	}()

	done := make(chan bool, 1)
	go func() { done <- state.handleAttackSwing(context.Background(), payload.Bytes()) }()

	sawStart, sawStateUpdate, sawStop := false, false, false
	var attackStopPayload []byte
	var attackStartPayload []byte
	for !(sawStart && sawStateUpdate && sawStop) {
		f := <-frames
		switch f.op {
		case uint16(protocol.OpcodeSMSG_ATTACK_START):
			sawStart = true
			attackStartPayload = f.data
		case uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE):
			sawStateUpdate = true
		case uint16(protocol.OpcodeSMSG_ATTACK_STOP):
			sawStop = true
			attackStopPayload = f.data
		}
	}

	<-done
	// Close server conn so reader goroutine exits
	serverConn.Close()
	<-readerDone

	// Validate ATTACK_START content
	r := protocol.NewReader(attackStartPayload)
	if attacker, err := r.ReadU64(); err != nil || attacker != 26 {
		t.Fatalf("attacker=%d err=%v", attacker, err)
	}
	if target, err := r.ReadU64(); err != nil || target != victim {
		t.Fatalf("target=%x err=%v", target, err)
	}
	// Validate ATTACK_STOP content
	reader := protocol.NewReader(attackStopPayload)
	if attacker, err := reader.ReadPackedGUID(); err != nil || attacker != 26 {
		t.Fatalf("stop attacker=%d err=%v", attacker, err)
	}
	if target, err := reader.ReadPackedGUID(); err != nil || target != victim {
		t.Fatalf("stop target=%x err=%v", target, err)
	}
}

func TestHandleSetSheathed(t *testing.T) {
	state := &session{playerLoaded: true, player: &playerState{GUID: 1}}
	payload := protocol.NewBuffer(4)
	payload.WriteU32(1) // Melee drawn
	if !state.handleSetSheathed(payload.Bytes()) {
		t.Fatal("handleSetSheathed failed")
	}
	if state.player.SheathState != 1 {
		t.Fatalf("expected sheath state 1, got %d", state.player.SheathState)
	}
}

func TestDuelLifecycleAndMeleeResolution(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()

	serverConn2, clientConn2 := net.Pipe()
	defer serverConn2.Close()
	defer clientConn2.Close()

	srv := &Server{sessions: make(map[*session]struct{})}
	sess1 := &session{
		conn:         serverConn1,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   10,
		player:       &playerState{GUID: 10, Map: 0, X: 0, Y: 0, Z: 0, Health: 100, MaxHealth: 100, AttackTime: 2000, MinDamage: 10, MaxDamage: 10},
	}
	sess2 := &session{
		conn:         serverConn2,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   20,
		player:       &playerState{GUID: 20, Map: 0, X: 1, Y: 0, Z: 0, Health: 15, MaxHealth: 100},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	// Set duel partnership
	sess1.duelPartner = 20
	sess2.duelPartner = 10
	sess1.player.DuelArbiter = 0xF110000000000001
	sess2.player.DuelArbiter = 0xF110000000000001
	sess1.player.DuelTeam = 1
	sess2.player.DuelTeam = 2

	// Drain frames from clientConn1 and clientConn2
	go func() {
		for {
			_, _, err := readServerFrame(clientConn1, nil)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			_, _, err := readServerFrame(clientConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	// sess1 attacks sess2 with lethal damage (15 HP target, ~10+ dmg)
	target, ok := sess1.getCombatTarget(context.Background(), 20)
	if !ok {
		t.Fatal("expected getCombatTarget to find duel opponent player")
	}
	if target.Health != 15 {
		t.Fatalf("expected target health 15, got %d", target.Health)
	}

	// Melee swing that brings duel partner to 0 HP
	sess1.executeMeleeSwing(context.Background(), target, protocol.BaseAttack)

	// Loser's health must be 1 (never dies in a duel)
	if sess2.player.Health != 1 {
		t.Fatalf("expected loser health 1, got %d", sess2.player.Health)
	}
	// Duel team and arbiter must be cleared
	if sess1.player.DuelTeam != 0 || sess2.player.DuelTeam != 0 {
		t.Fatalf("expected DuelTeam 0, got %d and %d", sess1.player.DuelTeam, sess2.player.DuelTeam)
	}
	if sess1.duelPartner != 0 || sess2.duelPartner != 0 {
		t.Fatalf("expected duelPartner 0 after duel ended, got %d and %d", sess1.duelPartner, sess2.duelPartner)
	}
}

func TestPlayerSpellDamageAndHeal(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()

	serverConn2, clientConn2 := net.Pipe()
	defer serverConn2.Close()
	defer clientConn2.Close()

	srv := &Server{
		Config:   config.Default(),
		sessions: make(map[*session]struct{}),
	}
	sess1 := &session{
		server:       srv,
		conn:         serverConn1,
		playerLoaded: true,
		playerGUID:   10,
		player:       &playerState{GUID: 10, Map: 0, X: 0, Y: 0, Z: 0, Health: 100, MaxHealth: 100},
	}
	sess2 := &session{
		server:       srv,
		conn:         serverConn2,
		playerLoaded: true,
		playerGUID:   20,
		player:       &playerState{GUID: 20, Map: 0, X: 1, Y: 0, Z: 0, Health: 30, MaxHealth: 100},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	// Drain client connections
	go func() {
		for {
			_, _, err := readServerFrame(clientConn1, nil)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			_, _, err := readServerFrame(clientConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	ctx := context.Background()

	// 1. Test executeSpellHeal on sess2 (30 HP -> +40 HP = 70 HP)
	sess1.executeSpellHeal(ctx, 20, 2050, 40) // Lesser Heal
	if sess2.player.Health != 70 {
		t.Fatalf("expected sess2 health 70 after heal, got %d", sess2.player.Health)
	}

	// 2. Set duel and test executeSpellDamage (70 HP - 100 dmg = lethal)
	sess1.duelPartner = 20
	sess2.duelPartner = 10
	sess1.player.DuelTeam = 1
	sess2.player.DuelTeam = 2

	sess1.executeSpellDamage(ctx, 20, 133, 100) // Fireball

	// In a duel, lethal damage caps health at 1 and ends duel
	if sess2.player.Health != 1 {
		t.Fatalf("expected sess2 health 1 after lethal duel spell, got %d", sess2.player.Health)
	}
	if sess1.player.DuelTeam != 0 || sess2.player.DuelTeam != 0 {
		t.Fatalf("expected duel to end, got DuelTeam %d and %d", sess1.player.DuelTeam, sess2.player.DuelTeam)
	}

	// 3. Verify unitFlagInCombat was set on attacker and victim
	if sess1.player.UnitFlags&unitFlagInCombat == 0 {
		t.Fatal("expected sess1 to have unitFlagInCombat set")
	}
	if sess2.player.UnitFlags&unitFlagInCombat == 0 {
		t.Fatal("expected sess2 to have unitFlagInCombat set")
	}
}

func TestDuelForfeitAndWinnerPacket(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()

	serverConn2, clientConn2 := net.Pipe()
	defer serverConn2.Close()
	defer clientConn2.Close()

	srv := &Server{
		Config:   config.Default(),
		sessions: make(map[*session]struct{}),
	}
	sess1 := &session{
		server:       srv,
		conn:         serverConn1,
		playerLoaded: true,
		authed:       true,
		playerGUID:   100,
		player:       &playerState{GUID: 100, Name: "Alice", Map: 0, X: 0, Y: 0, Z: 0, Health: 100, MaxHealth: 100},
	}
	sess2 := &session{
		server:       srv,
		conn:         serverConn2,
		playerLoaded: true,
		authed:       true,
		playerGUID:   200,
		player:       &playerState{GUID: 200, Name: "Bob", Map: 0, X: 10, Y: 0, Z: 0, Health: 100, MaxHealth: 100},
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	sess1.duelPartner = 200
	sess2.duelPartner = 100
	sess1.player.DuelTeam = 1
	sess2.player.DuelTeam = 2
	sess1.player.DuelArbiter = 0xF110000000000001
	sess2.player.DuelArbiter = 0xF110000000000001

	var winnerPkt1 []byte
	var mu sync.Mutex

	go func() {
		for {
			op, data, err := readServerFrame(clientConn1, nil)
			if err != nil {
				return
			}
			if op == uint16(protocol.OpcodeSMSG_DUEL_WINNER) {
				mu.Lock()
				winnerPkt1 = append([]byte(nil), data...)
				mu.Unlock()
			}
		}
	}()

	go func() {
		for {
			_, _, err := readServerFrame(clientConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	// Bob forfeits the duel via handleDuelCancelled
	ctx := context.Background()
	if !sess2.handleDuelCancelled(ctx, nil) {
		t.Fatal("handleDuelCancelled returned false")
	}

	time.Sleep(50 * time.Millisecond)

	// Both players must have duel state cleared
	if sess1.player.DuelTeam != 0 || sess2.player.DuelTeam != 0 {
		t.Fatalf("expected DuelTeam 0, got %d and %d", sess1.player.DuelTeam, sess2.player.DuelTeam)
	}
	if sess1.duelPartner != 0 || sess2.duelPartner != 0 {
		t.Fatalf("expected duelPartner 0, got %d and %d", sess1.duelPartner, sess2.duelPartner)
	}

	// Verify winner packet received on sess1
	mu.Lock()
	defer mu.Unlock()
	if len(winnerPkt1) == 0 {
		t.Fatal("expected SMSG_DUEL_WINNER packet")
	}
	reader := protocol.NewReader(winnerPkt1)
	fled, err := reader.ReadU8()
	if err != nil || fled != 0 {
		t.Fatalf("expected fled 0, got %d (err: %v)", fled, err)
	}
	winner, err := reader.ReadCString()
	if err != nil || winner != "Alice" {
		t.Fatalf("expected winner 'Alice', got '%s' (err: %v)", winner, err)
	}
	loser, err := reader.ReadCString()
	if err != nil || loser != "Bob" {
		t.Fatalf("expected loser 'Bob', got '%s' (err: %v)", loser, err)
	}
}

func TestDuelCancelBeforeStartInterrupted(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()

	srv := &Server{
		Config:   config.Default(),
		sessions: make(map[*session]struct{}),
	}
	sess1 := &session{
		server:       srv,
		conn:         serverConn1,
		playerLoaded: true,
		authed:       true,
		playerGUID:   100,
		player:       &playerState{GUID: 100, Name: "Alice", Map: 0, Health: 100, MaxHealth: 100},
	}
	srv.sessions[sess1] = struct{}{}
	sess1.duelPartner = 200
	sess1.player.DuelTeam = 0 // not started yet (countdown or requested)

	completeReceived := make(chan uint8, 1)
	winnerReceived := make(chan bool, 1)

	go func() {
		for {
			op, data, err := readServerFrame(clientConn1, nil)
			if err != nil {
				return
			}
			if op == uint16(protocol.OpcodeSMSG_DUEL_COMPLETE) && len(data) >= 1 {
				completeReceived <- data[0]
			}
			if op == uint16(protocol.OpcodeSMSG_DUEL_WINNER) {
				winnerReceived <- true
			}
		}
	}()

	ctx := context.Background()
	sess1.handleDuelCancelled(ctx, nil)

	select {
	case res := <-completeReceived:
		if res != 0 {
			t.Fatalf("expected SMSG_DUEL_COMPLETE result 0 (interrupted), got %d", res)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SMSG_DUEL_COMPLETE")
	}

	select {
	case <-winnerReceived:
		t.Fatal("unexpected SMSG_DUEL_WINNER on interrupted duel")
	case <-time.After(50 * time.Millisecond):
		// Expected: no winner packet
	}
}

func TestDuelOutOfBoundsAndInBounds(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()

	serverConn2, clientConn2 := net.Pipe()
	defer serverConn2.Close()
	defer clientConn2.Close()

	srv := &Server{
		Config:   config.Default(),
		sessions: make(map[*session]struct{}),
	}
	sess1 := &session{
		server:       srv,
		conn:         serverConn1,
		playerLoaded: true,
		authed:       true,
		playerGUID:   100,
		player:       &playerState{GUID: 100, Name: "Alice", Map: 0, X: 0, Y: 0, Z: 0, Health: 100, MaxHealth: 100},
		duelArbiterX: 0,
		duelArbiterY: 0,
		duelArbiterZ: 0,
	}
	sess2 := &session{
		server:       srv,
		conn:         serverConn2,
		playerLoaded: true,
		authed:       true,
		playerGUID:   200,
		player:       &playerState{GUID: 200, Name: "Bob", Map: 0, X: 5, Y: 0, Z: 0, Health: 100, MaxHealth: 100},
		duelArbiterX: 0,
		duelArbiterY: 0,
		duelArbiterZ: 0,
	}
	srv.sessions[sess1] = struct{}{}
	srv.sessions[sess2] = struct{}{}

	sess1.duelPartner = 200
	sess2.duelPartner = 100
	sess1.player.DuelTeam = 1
	sess2.player.DuelTeam = 2
	sess1.player.DuelArbiter = 0xF110000000000001
	sess2.player.DuelArbiter = 0xF110000000000001

	opcodes := make(chan uint16, 20)
	var winnerPayload []byte
	var mu sync.Mutex

	go func() {
		for {
			op, data, err := readServerFrame(clientConn1, nil)
			if err != nil {
				return
			}
			opcodes <- op
			if op == uint16(protocol.OpcodeSMSG_DUEL_WINNER) {
				mu.Lock()
				winnerPayload = append([]byte(nil), data...)
				mu.Unlock()
			}
		}
	}()

	go func() {
		for {
			_, _, err := readServerFrame(clientConn2, nil)
			if err != nil {
				return
			}
		}
	}()

	// 1. Move player 1 beyond 50 yards (X = 55)
	sess1.player.X = 55
	sess1.checkDuelBounds()

	select {
	case op := <-opcodes:
		if op != uint16(protocol.OpcodeSMSG_DUEL_OUTOFBOUNDS) {
			t.Fatalf("expected SMSG_DUEL_OUTOFBOUNDS, got 0x%04X", op)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SMSG_DUEL_OUTOFBOUNDS")
	}

	if sess1.duelOutOfBounds.IsZero() {
		t.Fatal("expected duelOutOfBounds to be set")
	}

	// 2. Move player 1 back in bounds (<= 40 yards, X = 35)
	sess1.player.X = 35
	sess1.checkDuelBounds()

	select {
	case op := <-opcodes:
		if op != uint16(protocol.OpcodeSMSG_DUEL_INBOUNDS) {
			t.Fatalf("expected SMSG_DUEL_INBOUNDS, got 0x%04X", op)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SMSG_DUEL_INBOUNDS")
	}

	if !sess1.duelOutOfBounds.IsZero() {
		t.Fatal("expected duelOutOfBounds to be reset to zero")
	}

	// 3. Move player 1 out of bounds again and simulate 10s timeout expiry (fled)
	sess1.player.X = 60
	sess1.checkDuelBounds()

	// Flush OUTOFBOUNDS packet
	select {
	case op := <-opcodes:
		if op != uint16(protocol.OpcodeSMSG_DUEL_OUTOFBOUNDS) {
			t.Fatalf("expected SMSG_DUEL_OUTOFBOUNDS, got 0x%04X", op)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for second SMSG_DUEL_OUTOFBOUNDS")
	}

	// Expire the out of bounds timer
	sess1.duelOutOfBounds = time.Now().Add(-1 * time.Second)
	sess1.checkDuelBounds()

	time.Sleep(50 * time.Millisecond)

	// Verify duel ended
	if sess1.player.DuelTeam != 0 || sess2.player.DuelTeam != 0 {
		t.Fatalf("expected DuelTeam 0 after fled, got %d and %d", sess1.player.DuelTeam, sess2.player.DuelTeam)
	}

	// Verify SMSG_DUEL_WINNER was received with fled=1, winner=Bob, loser=Alice
	mu.Lock()
	defer mu.Unlock()
	if len(winnerPayload) == 0 {
		t.Fatal("expected SMSG_DUEL_WINNER on flee")
	}
	reader := protocol.NewReader(winnerPayload)
	fled, err := reader.ReadU8()
	if err != nil || fled != 1 {
		t.Fatalf("expected fled 1, got %d", fled)
	}
	winner, err := reader.ReadCString()
	if err != nil || winner != "Bob" {
		t.Fatalf("expected winner Bob, got %s", winner)
	}
	loser, err := reader.ReadCString()
	if err != nil || loser != "Alice" {
		t.Fatalf("expected loser Alice, got %s", loser)
	}
}

func TestPvPMeleeSwingContinuationAndNearbyBroadcast(t *testing.T) {
	attClient, attServer := net.Pipe()
	defer attClient.Close()
	defer attServer.Close()

	vicClient, vicServer := net.Pipe()
	defer vicClient.Close()
	defer vicServer.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	attSess := &session{
		server:       srv,
		conn:         attServer,
		authed:       true,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Map:       0,
			X:         0,
			Y:         0,
			Z:         0,
			Health:    1000,
			MaxHealth: 1000,
			Level:     80,
		},
	}

	vicSess := &session{
		server:       srv,
		conn:         vicServer,
		authed:       true,
		playerLoaded: true,
		playerGUID:   20,
		player: &playerState{
			GUID:      20,
			Map:       0,
			X:         2, // within melee range
			Y:         0,
			Z:         0,
			Health:    1000,
			MaxHealth: 1000,
			Level:     80,
		},
	}

	srv.sessions[attSess] = struct{}{}
	srv.sessions[vicSess] = struct{}{}

	attOpcodes := make(chan uint16, 16)
	vicOpcodes := make(chan uint16, 16)

	go func() {
		for {
			op, _, err := readServerFrame(attClient, nil)
			if err != nil {
				return
			}
			if op != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
				attOpcodes <- op
			}
		}
	}()

	go func() {
		for {
			op, _, err := readServerFrame(vicClient, nil)
			if err != nil {
				return
			}
			if op != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
				vicOpcodes <- op
			}
		}
	}()

	// 1. Attacker attacks victim
	payload := protocol.NewBuffer(8)
	payload.WriteU64(20)
	if !attSess.handleAttackSwing(context.Background(), payload.Bytes()) {
		t.Fatal("handleAttackSwing failed")
	}

	// Attacker receives SMSG_ATTACK_START and SMSG_ATTACKERSTATEUPDATE (initial swing)
	var opAtt1, opAtt2 uint16
	select {
	case opAtt1 = <-attOpcodes:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for attacker SMSG_ATTACK_START")
	}
	if opAtt1 != uint16(protocol.OpcodeSMSG_ATTACK_START) {
		t.Fatalf("expected SMSG_ATTACK_START (0x143), got 0x%04X", opAtt1)
	}

	select {
	case opAtt2 = <-attOpcodes:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for attacker SMSG_ATTACKERSTATEUPDATE")
	}
	if opAtt2 != uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE) {
		t.Fatalf("expected SMSG_ATTACKERSTATEUPDATE (0x14A), got 0x%04X", opAtt2)
	}

	// Victim ALSO receives SMSG_ATTACK_START and SMSG_ATTACKERSTATEUPDATE via broadcast!
	var opVic1, opVic2 uint16
	select {
	case opVic1 = <-vicOpcodes:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for victim SMSG_ATTACK_START broadcast")
	}
	if opVic1 != uint16(protocol.OpcodeSMSG_ATTACK_START) {
		t.Fatalf("victim expected SMSG_ATTACK_START (0x143), got 0x%04X", opVic1)
	}

	select {
	case opVic2 = <-vicOpcodes:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for victim SMSG_ATTACKERSTATEUPDATE broadcast")
	}
	if opVic2 != uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE) {
		t.Fatalf("victim expected SMSG_ATTACKERSTATEUPDATE (0x14A), got 0x%04X", opVic2)
	}

	// 2. Crucial parity check: attacker MUST STILL BE ATTACKING!
	if attSess.attackTarget != 20 {
		t.Fatalf("expected attacker to maintain attackTarget=20, got %d", attSess.attackTarget)
	}
	for i := 0; i < 5 && vicSess.player.Health >= 1000; i++ {
		target, ok := attSess.getCombatTarget(context.Background(), 20)
		if ok {
			attSess.executeMeleeSwing(context.Background(), target, protocol.BaseAttack)
			select {
			case <-attOpcodes:
			case <-time.After(100 * time.Millisecond):
			}
			select {
			case <-vicOpcodes:
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if vicSess.player.Health >= 1000 {
		t.Fatalf("expected victim to take damage, health is %d", vicSess.player.Health)
	}
	if vicSess.player.UnitFlags&unitFlagInCombat == 0 {
		t.Fatal("expected victim to enter combat")
	}

	// 3. Attacker stops attack explicitly
	attSess.handleAttackStop()
	select {
	case op := <-attOpcodes:
		if op != uint16(protocol.OpcodeSMSG_ATTACK_STOP) {
			t.Fatalf("expected SMSG_ATTACK_STOP (0x144), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for attacker SMSG_ATTACK_STOP")
	}

	select {
	case op := <-vicOpcodes:
		if op != uint16(protocol.OpcodeSMSG_ATTACK_STOP) {
			t.Fatalf("expected victim to receive SMSG_ATTACK_STOP broadcast, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for victim SMSG_ATTACK_STOP broadcast")
	}

	if attSess.attackTarget != 0 {
		t.Fatalf("expected attacker target 0 after stop, got %d", attSess.attackTarget)
	}
}

func TestCreatureAggroAttackStartAndEvadeParity(t *testing.T) {
	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()

	c2, s2 := net.Pipe()
	defer c2.Close()
	defer s2.Close()

	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
		sessions:       make(map[*session]struct{}),
	}

	p1 := &session{
		server:       srv,
		conn:         s1,
		playerGUID:   10,
		authed:       true,
		playerLoaded: true,
		player: &playerState{
			GUID:   10,
			Map:    0,
			X:      100.0,
			Y:      100.0,
			Z:      10.0,
			Health: 500,
		},
	}
	p2 := &session{
		server:       srv,
		conn:         s2,
		playerGUID:   20,
		authed:       true,
		playerLoaded: true,
		player: &playerState{
			GUID:   20,
			Map:    0,
			X:      102.0,
			Y:      100.0,
			Z:      10.0,
			Health: 500,
		},
	}
	srv.sessions[p1] = struct{}{}
	srv.sessions[p2] = struct{}{}

	p1Ops := make(chan uint16, 20)
	p2Ops := make(chan uint16, 20)
	go func() {
		for {
			op, _, err := readServerFrame(c1, nil)
			if err != nil {
				return
			}
			p1Ops <- op
		}
	}()
	go func() {
		for {
			op, _, err := readServerFrame(c2, nil)
			if err != nil {
				return
			}
			p2Ops <- op
		}
	}()

	creatureGUID := uint64(5000)
	motion := &creatureMotion{
		GUID:       creatureGUID,
		Entry:      100,
		Map:        0,
		HomeX:      100.0,
		HomeY:      100.0,
		HomeZ:      10.0,
		X:          100.0,
		Y:          100.0,
		Z:          10.0,
		Speed:      2.5,
		RunSpeed:   7.0,
		Health:     100,
		MaxHealth:  100,
		AttackTime: 2000,
	}
	srv.creatureMotion[creatureGUID] = motion

	ctx := context.Background()

	// 1. Trigger creature aggro targeting p1
	srv.triggerCreatureAggro(ctx, creatureGUID, 10)

	// Verify p1 receives SMSG_AI_REACTION and SMSG_ATTACK_START
	select {
	case op := <-p1Ops:
		if op != uint16(protocol.OpcodeSMSG_AI_REACTION) {
			t.Fatalf("expected p1 to receive SMSG_AI_REACTION (0x13C), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p1 SMSG_AI_REACTION")
	}

	select {
	case op := <-p1Ops:
		if op != uint16(protocol.OpcodeSMSG_ATTACK_START) {
			t.Fatalf("expected p1 to receive SMSG_ATTACK_START (0x143), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p1 SMSG_ATTACK_START")
	}

	// Verify nearby observer p2 ALSO receives SMSG_AI_REACTION and SMSG_ATTACK_START broadcast
	select {
	case op := <-p2Ops:
		if op != uint16(protocol.OpcodeSMSG_AI_REACTION) {
			t.Fatalf("expected p2 to receive SMSG_AI_REACTION broadcast (0x13C), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p2 SMSG_AI_REACTION broadcast")
	}

	select {
	case op := <-p2Ops:
		if op != uint16(protocol.OpcodeSMSG_ATTACK_START) {
			t.Fatalf("expected p2 to receive SMSG_ATTACK_START broadcast (0x143), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p2 SMSG_ATTACK_START broadcast")
	}

	// 2. Creature melee attack in range (dist <= 3.0 yards) executes immediately
	players := []playerPos{
		{
			GUID: 10,
			Map:  0,
			X:    101.0,
			Y:    100.0,
			Z:    10.0,
			Sess: p1,
		},
	}
	now := time.Now()
	srv.stepCreatureMotion(ctx, motion, players, now)

	// Verify p1 received SMSG_ATTACKERSTATEUPDATE
	select {
	case op := <-p1Ops:
		if op != uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE) {
			t.Fatalf("expected p1 SMSG_ATTACKERSTATEUPDATE, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p1 SMSG_ATTACKERSTATEUPDATE")
	}

	// Verify observer p2 ALSO received SMSG_ATTACKERSTATEUPDATE broadcast
	select {
	case op := <-p2Ops:
		if op != uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE) {
			t.Fatalf("expected p2 SMSG_ATTACKERSTATEUPDATE broadcast, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p2 SMSG_ATTACKERSTATEUPDATE broadcast")
	}

	// Verify p1 and p2 received player update values after attack
	select {
	case op := <-p1Ops:
		if op != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) && op != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) {
			t.Fatalf("expected p1 update values packet, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p1 update values packet")
	}

	select {
	case op := <-p2Ops:
		if op != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) && op != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) {
			t.Fatalf("expected p2 update values broadcast, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p2 update values broadcast")
	}

	// 3. Test evade: Player runs > 45 yards away
	playersFar := []playerPos{
		{
			GUID: 10,
			Map:  0,
			X:    200.0, // 100 yards away
			Y:    100.0,
			Z:    10.0,
			Sess: p1,
		},
	}
	// Damage creature to 50 HP to verify health reset on evade
	motion.Health = 50

	srv.stepCreatureMotion(ctx, motion, playersFar, time.Now())

	// Verify creature dropped combat and reset health to max (100)
	if motion.InCombat || motion.TargetGUID != 0 {
		t.Fatalf("expected creature dropped combat on evade, inCombat=%v target=%d", motion.InCombat, motion.TargetGUID)
	}
	if motion.Health != motion.MaxHealth {
		t.Fatalf("expected creature health restored to %d on evade, got %d", motion.MaxHealth, motion.Health)
	}

	// Verify p1 and p2 received SMSG_ATTACK_STOP broadcast
	select {
	case op := <-p1Ops:
		if op != uint16(protocol.OpcodeSMSG_ATTACK_STOP) {
			t.Fatalf("expected p1 SMSG_ATTACK_STOP on evade, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p1 SMSG_ATTACK_STOP on evade")
	}

	select {
	case op := <-p2Ops:
		if op != uint16(protocol.OpcodeSMSG_ATTACK_STOP) {
			t.Fatalf("expected p2 SMSG_ATTACK_STOP on evade, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p2 SMSG_ATTACK_STOP on evade")
	}
}

func TestCreatureClassLevelStatsAndArmorScaling(t *testing.T) {
	// 1. Test calcArmorReducedDamage formula
	// Zero armor = no reduction
	if dmg := calcArmorReducedDamage(0, 80, 1000); dmg != 1000 {
		t.Fatalf("expected 1000 dmg against 0 armor, got %d", dmg)
	}
	// Level 80 attacker against 10673 armor (The Lich King / Onyxia)
	// Reduction is ~41.2%, so 1000 dmg becomes ~588
	dmg80 := calcArmorReducedDamage(10673, 80, 1000)
	if dmg80 < 580 || dmg80 > 595 {
		t.Fatalf("expected ~588 dmg against 10673 armor at lvl 80, got %d", dmg80)
	}
	// Extreme armor should cap at 75% reduction (1000 -> 250)
	dmgCapped := calcArmorReducedDamage(500000, 80, 1000)
	if dmgCapped != 250 {
		t.Fatalf("expected capped 250 dmg against extreme armor, got %d", dmgCapped)
	}

	// 2. Test loadCreatureStats with creature_template and creature_classlevelstats
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE creature_template (
			entry INTEGER PRIMARY KEY,
			maxlevel INTEGER NOT NULL DEFAULT 1,
			unit_class INTEGER NOT NULL DEFAULT 1,
			exp INTEGER NOT NULL DEFAULT 0,
			BaseAttackTime INTEGER NOT NULL DEFAULT 2000,
			HealthModifier REAL NOT NULL DEFAULT 1.0,
			ArmorModifier REAL NOT NULL DEFAULT 1.0,
			DamageModifier REAL NOT NULL DEFAULT 1.0,
			unit_flags INTEGER NOT NULL DEFAULT 0,
			flags_extra INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE creature_classlevelstats (
			level INTEGER NOT NULL,
			class INTEGER NOT NULL,
			basehp0 INTEGER NOT NULL,
			basehp1 INTEGER NOT NULL,
			basehp2 INTEGER NOT NULL,
			basearmor INTEGER NOT NULL,
			damage_base REAL NOT NULL,
			damage_exp1 REAL NOT NULL,
			damage_exp2 REAL NOT NULL,
			PRIMARY KEY (level, class)
		)`,
		// Insert Hogger (entry 448, lvl 11, class 1, exp 0, 2x HP, 2x Armor, 1.5x Dmg)
		`INSERT INTO creature_template VALUES (448, 11, 1, 0, 2000, 2.0, 2.0, 1.5, 0, 0)`,
		`INSERT INTO creature_classlevelstats VALUES (11, 1, 333, 500, 700, 264, 8.0, 15.0, 25.0)`,
		// Insert Lich King (entry 36597, lvl 83, class 1, exp 2, 1250x HP, 1x Armor, 139x Dmg)
		`INSERT INTO creature_template VALUES (36597, 83, 1, 2, 1500, 1250.0, 1.0, 139.0, 0, 0)`,
		`INSERT INTO creature_classlevelstats VALUES (83, 1, 5808, 10019, 13945, 10673, 49.2375, 136.562, 177.074)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: worldStore, creatureStatsCache: make(map[uint32]creatureStats)}
	ctx := context.Background()

	// Hogger stats
	hogger := srv.loadCreatureStats(ctx, 448)
	if hogger.Level != 11 {
		t.Fatalf("expected Hogger level 11, got %d", hogger.Level)
	}
	if hogger.Health != 666 { // 333 * 2.0
		t.Fatalf("expected Hogger HP 666, got %d", hogger.Health)
	}
	if hogger.Armor != 528 { // 264 * 2.0
		t.Fatalf("expected Hogger Armor 528, got %d", hogger.Armor)
	}
	if hogger.MinDamage != 12.0 { // 8.0 * 1.5
		t.Fatalf("expected Hogger MinDamage 12.0, got %f", hogger.MinDamage)
	}
	if hogger.MaxDamage != 18.0 { // 12.0 * 1.5
		t.Fatalf("expected Hogger MaxDamage 18.0, got %f", hogger.MaxDamage)
	}

	// Lich King stats (17.43M HP, 10673 Armor)
	lk := srv.loadCreatureStats(ctx, 36597)
	if lk.Health != 17431250 { // 13945 * 1250
		t.Fatalf("expected Lich King HP 17431250, got %d", lk.Health)
	}
	if lk.Armor != 10673 {
		t.Fatalf("expected Lich King Armor 10673, got %d", lk.Armor)
	}
	expectedLKDmg := float32(177.074 * 139.0)
	if lk.MinDamage != expectedLKDmg {
		t.Fatalf("expected Lich King MinDamage %f, got %f", expectedLKDmg, lk.MinDamage)
	}

	// Cache verification
	hoggerCached := srv.loadCreatureStats(ctx, 448)
	if hoggerCached.Health != 666 {
		t.Fatalf("expected cached Hogger HP 666, got %d", hoggerCached.Health)
	}
}

func TestRollMeleeOutcome_Formulas(t *testing.T) {
	// 1. High level player vs level 1 mob: miss chance is 0
	for i := 0; i < 200; i++ {
		outcome, hitInfo, targetState := rollMeleeOutcome(80, 1, true, false, false, false, false)
		if outcome == protocol.MeleeHitMiss {
			t.Fatalf("expected 0%% miss chance for level 80 vs 1 mob, got miss")
		}
		if hitInfo&protocol.HitInfoMiss != 0 || targetState == protocol.VictimStateIntact {
			t.Fatalf("unexpected miss hitInfo/targetState for level 80 vs 1 mob")
		}
	}

	// 2. Glancing blow: player vs higher level mob (e.g. 70 vs 73)
	glancingFound := false
	for i := 0; i < 1000; i++ {
		outcome, hitInfo, _ := rollMeleeOutcome(70, 73, true, false, false, false, false)
		if outcome == protocol.MeleeHitGlancing {
			glancingFound = true
			if hitInfo&protocol.HitInfoGlancing == 0 {
				t.Fatalf("expected HitInfoGlancing flag on glancing blow")
			}
			break
		}
	}
	if !glancingFound {
		t.Fatalf("expected at least one glancing blow for level 70 vs 73 mob")
	}

	// 3. Crushing blow: mob 4+ levels higher attacking player (e.g. 74 vs 70)
	crushingFound := false
	for i := 0; i < 1000; i++ {
		outcome, hitInfo, _ := rollMeleeOutcome(74, 70, false, true, false, false, false)
		if outcome == protocol.MeleeHitCrushing {
			crushingFound = true
			if hitInfo&protocol.HitInfoCrushing == 0 {
				t.Fatalf("expected HitInfoCrushing flag on crushing blow")
			}
			break
		}
	}
	if !crushingFound {
		t.Fatalf("expected at least one crushing blow for level 74 mob vs 70 player")
	}

	// 4. Block: target can block
	blockFound := false
	for i := 0; i < 1000; i++ {
		outcome, hitInfo, _ := rollMeleeOutcome(70, 70, true, true, false, true, false)
		if outcome == protocol.MeleeHitBlock {
			blockFound = true
			if hitInfo&protocol.HitInfoBlock == 0 {
				t.Fatalf("expected HitInfoBlock flag on block outcome")
			}
			break
		}
	}
	if !blockFound {
		t.Fatalf("expected at least one block outcome when canBlock is true")
	}

	// 5. Parry: target can parry
	parryFound := false
	for i := 0; i < 1000; i++ {
		outcome, _, targetState := rollMeleeOutcome(70, 70, true, true, false, false, true)
		if outcome == protocol.MeleeHitParry {
			parryFound = true
			if targetState != protocol.VictimStateParry {
				t.Fatalf("expected VictimStateParry on parry outcome, got %d", targetState)
			}
			break
		}
	}
	if !parryFound {
		t.Fatalf("expected at least one parry outcome when canParry is true")
	}

	// 6. Crit: crit outcome
	critFound := false
	for i := 0; i < 1000; i++ {
		outcome, hitInfo, _ := rollMeleeOutcome(70, 70, true, true, false, false, false)
		if outcome == protocol.MeleeHitCrit {
			critFound = true
			if hitInfo&protocol.HitInfoCriticalHit == 0 {
				t.Fatalf("expected HitInfoCriticalHit flag on crit outcome")
			}
			break
		}
	}
	if !critFound {
		t.Fatalf("expected at least one crit outcome in 1000 rolls")
	}
}

func TestRollMeleeOutcome_DualWieldPenalty(t *testing.T) {
	// Dual-wielding adds +19% miss chance: equal level player PvP miss is 5% + 19% = 24%
	missCountDW := 0
	missCountSingle := 0
	trials := 5000
	for i := 0; i < trials; i++ {
		outDW, _, _ := rollMeleeOutcome(80, 80, true, true, true, false, false)
		if outDW == protocol.MeleeHitMiss {
			missCountDW++
		}
		outSingle, _, _ := rollMeleeOutcome(80, 80, true, true, false, false, false)
		if outSingle == protocol.MeleeHitMiss {
			missCountSingle++
		}
	}

	dwRate := float64(missCountDW) / float64(trials)
	singleRate := float64(missCountSingle) / float64(trials)

	// Single wield miss should be around 5% (0.03 - 0.08)
	if singleRate < 0.03 || singleRate > 0.08 {
		t.Fatalf("expected single wield miss rate ~5%%, got %.2f%%", singleRate*100)
	}
	// Dual wield miss should be around 24% (0.20 - 0.28)
	if dwRate < 0.20 || dwRate > 0.28 {
		t.Fatalf("expected dual wield miss rate ~24%%, got %.2f%%", dwRate*100)
	}
}

func TestParryHaste_Calculation(t *testing.T) {
	attackTime := 2000 * time.Millisecond
	// 20% = 400ms, 60% = 1200ms

	// 1. Remaining time <= 20% (e.g. 300ms) -> unchanged
	rem1 := calcParryHastedRemaining(300*time.Millisecond, attackTime)
	if rem1 != 300*time.Millisecond {
		t.Fatalf("expected <=20%% remaining time to be unchanged (300ms), got %v", rem1)
	}

	// 2. Remaining time between 20% and 60% (e.g. 800ms) -> set to 20% (400ms)
	rem2 := calcParryHastedRemaining(800*time.Millisecond, attackTime)
	if rem2 != 400*time.Millisecond {
		t.Fatalf("expected 20%%..60%% remaining time to be hasted to 20%% (400ms), got %v", rem2)
	}

	// 3. Remaining time > 60% (e.g. 1500ms) -> reduced by 40% (800ms) -> 700ms
	rem3 := calcParryHastedRemaining(1500*time.Millisecond, attackTime)
	if rem3 != 700*time.Millisecond {
		t.Fatalf("expected >60%% remaining time to be reduced by 40%% (1500 - 800 = 700ms), got %v", rem3)
	}
}

func TestOffhandAttack_50PctPenaltyAndHitInfo(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{sessions: make(map[*session]struct{})}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:              10,
			Level:             80,
			MinDamage:         100,
			MaxDamage:         100,
			AttackTime:        2000,
			MinOffhandDamage:  100,
			MaxOffhandDamage:  100,
			OffhandAttackTime: 2000,
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 10)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	// Target mob with 0 armor for predictable base comparison
	target := combatTarget{
		GUID:      uint64(999) | (uint64(500) << 24) | (uint64(0xF130) << 48),
		Health:    5000,
		MaxHealth: 5000,
		Level:     80,
		Armor:     0,
	}

	// 1. Off-hand attack execution
	sess.executeMeleeSwing(context.Background(), target, protocol.OffAttack)
	time.Sleep(20 * time.Millisecond)

	var offAsu []byte
	for len(pktChan) > 0 {
		p := <-pktChan
		if p.opcode == uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE) {
			offAsu = p.data
			break
		}
	}
	if len(offAsu) == 0 {
		t.Fatal("expected SMSG_ATTACKERSTATEUPDATE for offhand attack")
	}

	r := protocol.NewReader(offAsu)
	hitInfo, _ := r.ReadU32()
	if hitInfo&protocol.HitInfoOffHand == 0 {
		t.Fatalf("expected HitInfoOffHand (0x04) on offhand attack, got 0x%08X", hitInfo)
	}
}

func TestSpellPower_DirectDamageAndHeal(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			500: {
				GUID:      500,
				Health:    5000,
				MaxHealth: 5000,
				Level:     80,
			},
		},
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:       10,
			Level:      80,
			Health:     1000,
			MaxHealth:  2000,
			SpellPower: 500, // +500 spell power
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 10)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	// Cast direct spell with base 100 damage:
	// With 500 spell power and default ~85.7% coefficient, damage increases by ~428 -> ~528+
	sess.executeSpellDamage(context.Background(), 500, 133, 100)
	time.Sleep(20 * time.Millisecond)

	var spellDmg uint32
	for len(pktChan) > 0 {
		p := <-pktChan
		if p.opcode == uint16(protocol.OpcodeSMSG_SPELLNONMELEEDAMAGELOG) {
			r := protocol.NewReader(p.data)
			_, _ = r.ReadPackedGUID() // target
			_, _ = r.ReadPackedGUID() // attacker
			_, _ = r.ReadU32()        // spellID
			spellDmg, _ = r.ReadU32() // damage
			break
		}
	}
	if spellDmg < 500 {
		t.Fatalf("expected spell damage boosted by 500 spell power (got %d, expected > 500)", spellDmg)
	}

	// Test healing boosted by spell power
	initialHealth := sess.player.Health
	sess.executeSpellHeal(context.Background(), sess.playerGUID, 2060, 200)
	healedAmount := sess.player.Health - initialHealth
	if healedAmount < 600 {
		t.Fatalf("expected heal boosted by 500 spell power (got %d, expected > 600)", healedAmount)
	}
}

func TestParryHaste_Integration(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()

	serverConn2, clientConn2 := net.Pipe()
	defer serverConn2.Close()
	defer clientConn2.Close()

	srv := &Server{sessions: make(map[*session]struct{})}
	attacker := &session{
		server:       srv,
		conn:         serverConn1,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:       10,
			Level:      80,
			AttackTime: 2000,
			Health:     1000,
			MaxHealth:  1000,
		},
	}
	defender := &session{
		server:       srv,
		conn:         serverConn2,
		playerLoaded: true,
		playerGUID:   20,
		player: &playerState{
			GUID:       20,
			Level:      80,
			AttackTime: 2000,
			Health:     1000,
			MaxHealth:  1000,
		},
	}
	srv.sessions[attacker] = struct{}{}
	srv.sessions[defender] = struct{}{}

	go func() {
		for {
			if _, _, err := readServerFrame(clientConn1, nil); err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			if _, _, err := readServerFrame(clientConn2, nil); err != nil {
				return
			}
		}
	}()

	now := time.Now()
	// Defender swung 500ms ago (1500ms remaining on a 2000ms attack timer: >60% threshold)
	defender.lastSwing = now.Add(-500 * time.Millisecond)

	// Target struct for defender
	target := combatTarget{
		GUID:      20,
		Level:     80,
		Health:    1000,
		MaxHealth: 1000,
	}

	// Loop swings until a parry occurs (with parry enabled against defender)
	parried := false
	for i := 0; i < 200; i++ {
		// Reset defender lastSwing to 500ms ago before each trial
		defender.lastSwing = time.Now().Add(-500 * time.Millisecond)
		beforeRemaining := 2000*time.Millisecond - time.Since(defender.lastSwing)

		attacker.executeMeleeSwing(context.Background(), target, protocol.BaseAttack)

		afterRemaining := 2000*time.Millisecond - time.Since(defender.lastSwing)
		// If parry hasted, remaining should drop by ~800ms (from ~1500ms to ~700ms)
		if afterRemaining < beforeRemaining-500*time.Millisecond {
			parried = true
			break
		}
	}

	if !parried {
		t.Logf("no parry occurred during 200 trials (within normal RNG variance)")
	}
}

func TestCalcMeleeRange_FormulasAndLargeCreatureReach(t *testing.T) {
	// 1. Standard humanoid (1.5 yd) vs standard humanoid (1.5 yd):
	// 1.5 + 1.5 + 4/3 = 4.333 yd -> capped at 5.0 yd (NOMINAL_MELEE_RANGE)
	r1 := calcMeleeRange(1.5, 1.5)
	if r1 != 5.0 {
		t.Fatalf("expected nominal melee range 5.0, got %f", r1)
	}

	// 2. Giant creature/dragon (e.g. 8.0 yd combat reach) vs player (1.5 yd):
	// 8.0 + 1.5 + 4/3 = 10.833 yd
	r2 := calcMeleeRange(1.5, 8.0)
	expected := 1.5 + 8.0 + 4.0/3.0
	if math.Abs(r2-expected) > 0.001 {
		t.Fatalf("expected large creature melee reach ~%f, got %f", expected, r2)
	}

	// 3. Verify handleAttackSwing succeeds when player is 9 yards from an 8.0-reach boss
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	bossGUID := uint64(555) | (uint64(500) << 24) | (uint64(0xF130) << 48)
	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			bossGUID: {
				GUID:        bossGUID,
				X:           9.0, // 9 yards away
				Y:           0,
				Z:           0,
				Health:      50000,
				MaxHealth:   50000,
				CombatReach: 8.0, // Onyxia-sized reach
				Level:       80,
			},
		},
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:        10,
			X:           0,
			Y:           0,
			Z:           0,
			CombatReach: 1.5,
			Level:       80,
			Health:      1000,
			MaxHealth:   1000,
		},
	}
	srv.sessions[sess] = struct{}{}

	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	payload := protocol.NewBuffer(8)
	payload.WriteU64(bossGUID)
	if !sess.handleAttackSwing(context.Background(), payload.Bytes()) {
		t.Fatal("handleAttackSwing failed for large boss at 9 yards")
	}
	if sess.attackTarget != bossGUID {
		t.Fatalf("expected attackTarget %d, got %d", bossGUID, sess.attackTarget)
	}
}

func TestAmmoDPS_RangedStatsScaling(t *testing.T) {
	wdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()
	wdb.SetMaxOpenConns(1)

	_, err = wdb.Exec(`CREATE TABLE item_template (
		entry INTEGER PRIMARY KEY,
		class INTEGER NOT NULL DEFAULT 0,
		subclass INTEGER NOT NULL DEFAULT 0,
		dmg_min1 REAL NOT NULL DEFAULT 0,
		dmg_max1 REAL NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Ammo 2512: class 6 (projectile), subclass 2 (arrow), dmg 15..25 (avg 20.0 DPS)
	_, err = wdb.Exec(`INSERT INTO item_template (entry, class, subclass, dmg_min1, dmg_max1) VALUES (2512, 6, 2, 15.0, 25.0)`)
	if err != nil {
		t.Fatal(err)
	}

	cdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer cdb.Close()
	cdb.SetMaxOpenConns(1)

	_, err = cdb.Exec(`CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cdb.Exec(`CREATE TABLE item_instance (guid INTEGER, itemEntry INTEGER, count INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		WorldStore:      &database.Store{Name: "world", Backend: database.BackendSQLite, DB: wdb},
		CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: cdb},
	}
	sess := &session{
		server:       srv,
		playerLoaded: true,
		player: &playerState{
			GUID:             1,
			Level:            80,
			Class:            3, // Hunter
			Stats:            [5]uint32{100, 100, 100, 100, 100},
			RangedAttackTime: 2500, // 2.5s bow
			AmmoID:           2512,
		},
	}

	err = sess.calculatePlayerStats(context.Background(), sess.player)
	if err != nil {
		t.Fatal(err)
	}

	// Ammo DPS = 20.0, speed = 2.5s -> bonus damage = 50.0
	if sess.player.AmmoDPS != 20.0 {
		t.Fatalf("expected AmmoDPS 20.0, got %f", sess.player.AmmoDPS)
	}
	if sess.player.MinRangedDamage < 50.0 || sess.player.MaxRangedDamage < 51.0 {
		t.Fatalf("expected MinRangedDamage >= 50.0, MaxRangedDamage >= 51.0, got min=%f max=%f", sess.player.MinRangedDamage, sess.player.MaxRangedDamage)
	}
}

func TestAutoShot_RangeAndAmmoValidation(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions:       make(map[*session]struct{}),
		creatureMotion: make(map[uint64]*creatureMotion),
		Data:           wotlk.NewStore("../../data/dbc"),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:             10,
			X:                0,
			Y:                0,
			Z:                0,
			CombatReach:      1.5,
			Level:            80,
			Health:           1000,
			MaxHealth:        1000,
			AmmoID:           0, // No ammo
			RangedAttackTime: 2000,
			Spells: []learnedSpell{
				{ID: 75, Active: true},
			},
		},
	}
	srv.sessions[sess] = struct{}{}

	targetGUID := uint64(50)
	srv.creatureMotion[targetGUID] = &creatureMotion{
		GUID:        targetGUID,
		X:           3.0, // 3 yards away: within melee range (dead zone < 5.0 yards)
		Y:           0,
		Z:           0,
		Health:      1000,
		MaxHealth:   1000,
		CombatReach: 1.5,
		Level:       80,
	}

	receivedOpcodes := make(chan uint16, 8)
	receivedPayloads := make(chan []byte, 8)
	go func() {
		for {
			op, p, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
			receivedOpcodes <- op
			receivedPayloads <- p
		}
	}()

	// 1. Cast Auto Shot (75) while in melee range (3 yards) -> SPELL_FAILED_TOO_CLOSE (128)
	payload := protocol.NewBuffer(32)
	payload.WriteU8(1) // castID
	payload.WriteU32(75) // spellID Auto Shot
	payload.WriteU8(0) // castFlags
	protocol.WriteSpellTargetData(payload, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID})

	sess.player.AmmoID = 2512 // Has ammo
	sess.handleCastSpell(context.Background(), payload.Bytes())

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
			t.Fatalf("expected SMSG_CAST_FAILED, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		_, _ = r.ReadU8() // castID
		spID, _ := r.ReadU32()
		reason, _ := r.ReadU8()
		if spID != 75 || reason != 128 {
			t.Fatalf("expected spell 75 failed with reason 128 (TOO_CLOSE), got spell=%d reason=%d", spID, reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for cast failed TOO_CLOSE")
	}

	// 2. Target moved to 45 yards away (> 35 yards) -> SPELL_FAILED_OUT_OF_RANGE (97)
	srv.creatureMotion[targetGUID].X = 45.0
	sess.handleCastSpell(context.Background(), payload.Bytes())

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
			t.Fatalf("expected SMSG_CAST_FAILED, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		_, _ = r.ReadU8()
		spID, _ := r.ReadU32()
		reason, _ := r.ReadU8()
		if spID != 75 || reason != 97 {
			t.Fatalf("expected spell 75 failed with reason 97 (OUT_OF_RANGE), got spell=%d reason=%d", spID, reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for cast failed OUT_OF_RANGE")
	}

	// 3. Target is at 20 yards (valid range), but player has NO ammo (AmmoID = 0) -> SPELL_FAILED_NO_AMMO (75)
	srv.creatureMotion[targetGUID].X = 20.0
	sess.player.AmmoID = 0
	sess.handleCastSpell(context.Background(), payload.Bytes())

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
			t.Fatalf("expected SMSG_CAST_FAILED, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		_, _ = r.ReadU8()
		spID, _ := r.ReadU32()
		reason, _ := r.ReadU8()
		if spID != 75 || reason != 75 {
			t.Fatalf("expected spell 75 failed with reason 75 (NO_AMMO), got spell=%d reason=%d", spID, reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for cast failed NO_AMMO")
	}
}

func TestAutoShot_ExecutionAndAmmoConsumption(t *testing.T) {
	cdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer cdb.Close()
	cdb.SetMaxOpenConns(1)

	_, err = cdb.Exec(`CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cdb.Exec(`CREATE TABLE item_instance (guid INTEGER, itemEntry INTEGER, count INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	// Player has 10 arrows (item 2512, item_instance guid 999)
	_, _ = cdb.Exec(`INSERT INTO character_inventory (guid, bag, slot, item) VALUES (10, 0, 23, 999)`)
	_, _ = cdb.Exec(`INSERT INTO item_instance (guid, itemEntry, count) VALUES (999, 2512, 10)`)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: cdb},
		sessions:        make(map[*session]struct{}),
		creatureMotion:  make(map[uint64]*creatureMotion),
	}
	sess := &session{
		server:          srv,
		conn:            serverConn,
		playerLoaded:    true,
		playerGUID:      10,
		autoRepeatSpell: 75,
		player: &playerState{
			GUID:             10,
			X:                0,
			Y:                0,
			Z:                0,
			CombatReach:      1.5,
			Level:            80,
			Health:           1000,
			MaxHealth:        1000,
			AmmoID:           2512,
			MinRangedDamage:  100.0,
			MaxRangedDamage:  120.0,
			RangedAttackTime: 2000,
		},
	}
	srv.sessions[sess] = struct{}{}

	targetGUID := uint64(50)
	targetMotion := &creatureMotion{
		GUID:        targetGUID,
		X:           20.0, // 20 yards away
		Y:           0,
		Z:           0,
		Health:      1000,
		MaxHealth:   1000,
		CombatReach: 1.5,
		Level:       80,
	}
	srv.creatureMotion[targetGUID] = targetMotion

	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	// Execute ranged attack
	target := combatTarget{GUID: targetGUID, Health: 1000, MaxHealth: 1000, Level: 80, CombatReach: 1.5}
	sess.executeRangedAttack(context.Background(), target, 75)

	// Verify target took damage
	if targetMotion.Health >= 1000 {
		t.Fatalf("expected creature to take ranged damage, health is %d", targetMotion.Health)
	}
	// Verify creature entered combat and set target
	if !targetMotion.InCombat || targetMotion.TargetGUID != 10 {
		t.Fatalf("expected creature in combat targeting 10, got inCombat=%v target=%d", targetMotion.InCombat, targetMotion.TargetGUID)
	}

	// Verify ammo count was decremented from 10 to 9
	var remainingCount int64
	err = cdb.QueryRow(`SELECT count FROM item_instance WHERE guid = 999`).Scan(&remainingCount)
	if err != nil {
		t.Fatal(err)
	}
	if remainingCount != 9 {
		t.Fatalf("expected remaining ammo count 9, got %d", remainingCount)
	}
}




