package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
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
	sess1.executeMeleeSwing(context.Background(), target)

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

