package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

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

