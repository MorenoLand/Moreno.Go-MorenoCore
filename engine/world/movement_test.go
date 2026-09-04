package world

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestMovementInfoRoundTrip(t *testing.T) {
	original := movementInfo{GUID: 99, Flags: movementOnTransport | movementSwimming | movementFalling | movementSplineElevation, Flags2: movement2Interpolated, Time: 123, X: 1.5, Y: 2.5, Z: 3.5, Orientation: 0.5, Pitch: 0.25, HasPitch: true, FallTime: 456, Jump: [4]float32{1, 2, 3, 4}, HasJump: true, SplineElevation: 5, HasSpline: true, Transport: &transportMovement{GUID: 7, X: 6, Y: 7, Z: 8, Orientation: 9, Time: 10, Seat: -1, Time2: 11, HasTime2: true}}
	encoded := protocol.NewBuffer(128)
	writeMovementInfo(encoded, original)
	reader := protocol.NewReader(encoded.Bytes())
	guid, err := reader.ReadPackedGUID()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := readMovementInfo(reader)
	if err != nil {
		t.Fatal(err)
	}
	decoded.GUID = guid
	if reader.Remaining() != 0 {
		t.Fatalf("remaining bytes=%d", reader.Remaining())
	}
	reencoded := protocol.NewBuffer(128)
	writeMovementInfo(reencoded, decoded)
	if !bytes.Equal(encoded.Bytes(), reencoded.Bytes()) {
		t.Fatalf("movement bytes differ: %x != %x", encoded.Bytes(), reencoded.Bytes())
	}
}

func TestSanitizeMovementFlags(t *testing.T) {
	if got := sanitizeMovementFlags(movementForward | movementBackward | movementStrafeLeft | movementStrafeRight | movementTurnLeft | movementTurnRight | movementAscending | movementDescending); got != 0 {
		t.Fatalf("sanitized flags=%x", got)
	}
}

func TestMovementAcksAndBroadcasts(t *testing.T) {
	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()

	c2, s2 := net.Pipe()
	defer c2.Close()
	defer s2.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	p1 := &session{
		server:       srv,
		conn:         s1,
		authed:       true,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID: 10,
			Map:  0,
			X:    10,
			Y:    20,
			Z:    30,
		},
	}

	p2 := &session{
		server:       srv,
		conn:         s2,
		authed:       true,
		playerLoaded: true,
		playerGUID:   20,
		player: &playerState{
			GUID: 20,
			Map:  0,
			X:    15,
			Y:    25,
			Z:    30,
		},
	}

	srv.sessions[p1] = struct{}{}
	srv.sessions[p2] = struct{}{}

	go func() {
		for {
			_, _, err := readServerFrame(c1, nil)
			if err != nil {
				return
			}
		}
	}()

	p2Opcodes := make(chan uint16, 16)
	go func() {
		for {
			op, _, err := readServerFrame(c2, nil)
			if err != nil {
				return
			}
			p2Opcodes <- op
		}
	}()

	// Build movement payload for p1
	buf := protocol.NewBuffer(64)
	buf.WritePackedGUID(10)
	buf.WriteU32(1) // ack index
	info := movementInfo{
		GUID:        10,
		Flags:       0,
		Time:        1000,
		X:           12,
		Y:           22,
		Z:           30,
		Orientation: 1.0,
		HasJump:     true,
		Jump:        [4]float32{1.0, 0.5, 5.0, 10.0},
	}
	writeRawMovementInfo(buf, info)

	// 1. Root ACK
	if !p1.handleForceMoveRootAck(context.Background(), buf.Bytes()) {
		t.Fatal("handleForceMoveRootAck failed")
	}
	if !p1.rooted {
		t.Fatal("expected p1 to be rooted")
	}
	select {
	case op := <-p2Opcodes:
		if op != uint16(protocol.OpcodeMSG_MOVE_ROOT) {
			t.Fatalf("expected MSG_MOVE_ROOT (0x0EC), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for MSG_MOVE_ROOT broadcast")
	}

	// 2. Unroot ACK
	if !p1.handleForceMoveUnrootAck(context.Background(), buf.Bytes()) {
		t.Fatal("handleForceMoveUnrootAck failed")
	}
	if p1.rooted {
		t.Fatal("expected p1 to be unrooted")
	}
	select {
	case op := <-p2Opcodes:
		if op != uint16(protocol.OpcodeMSG_MOVE_UNROOT) {
			t.Fatalf("expected MSG_MOVE_UNROOT (0x0ED), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for MSG_MOVE_UNROOT broadcast")
	}

	// 3. Knockback ACK
	if !p1.handleMoveKnockBackAck(context.Background(), buf.Bytes()) {
		t.Fatal("handleMoveKnockBackAck failed")
	}
	select {
	case op := <-p2Opcodes:
		if op != uint16(protocol.OpcodeMSG_MOVE_KNOCK_BACK) {
			t.Fatalf("expected MSG_MOVE_KNOCK_BACK (0x0F1), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for MSG_MOVE_KNOCK_BACK broadcast")
	}

	// 4. Spline Done taxi flight finish
	p1.inFlight = true
	p1.player.MountDisplayID = 1234
	if !p1.handleMoveSplineDone(context.Background(), buf.Bytes()) {
		t.Fatal("handleMoveSplineDone failed")
	}
	if p1.inFlight {
		t.Fatal("expected inFlight to be false after spline done")
	}
	if p1.player.MountDisplayID != 0 {
		t.Fatalf("expected mount display ID 0 after flight, got %d", p1.player.MountDisplayID)
	}
	select {
	case op := <-p2Opcodes:
		if op != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
			t.Fatalf("expected update object broadcast, got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
	}

	// 5. Time skipped
	tsBuf := protocol.NewBuffer(16)
	tsBuf.WritePackedGUID(10)
	tsBuf.WriteU32(50) // 50ms skipped
	if !p1.handleMoveTimeSkipped(context.Background(), tsBuf.Bytes()) {
		t.Fatal("handleMoveTimeSkipped failed")
	}
	select {
	case op := <-p2Opcodes:
		if op != uint16(protocol.OpcodeMSG_MOVE_TIME_SKIPPED) {
			t.Fatalf("expected MSG_MOVE_TIME_SKIPPED (0x2CE), got 0x%04X", op)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for MSG_MOVE_TIME_SKIPPED broadcast")
	}
}

func TestSummonRequestAndResponseFlow(t *testing.T) {
	cTgt, sTgt := net.Pipe()
	defer cTgt.Close()
	defer sTgt.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	summoner := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:        10,
			Map:         0,
			X:           100,
			Y:           200,
			Z:           300,
			Orientation: 1.5,
			Health:      1000,
		},
	}

	target := &session{
		server:       srv,
		conn:         sTgt,
		authed:       true,
		playerLoaded: true,
		playerGUID:   20,
		player: &playerState{
			GUID:        20,
			Map:         1,
			X:           10,
			Y:           20,
			Z:           30,
			Orientation: 0.5,
			Health:      500,
		},
	}

	srv.sessions[summoner] = struct{}{}
	srv.sessions[target] = struct{}{}

	type pkt struct {
		op   uint16
		data []byte
	}
	tgtPackets := make(chan pkt, 16)
	go func() {
		for {
			op, data, err := readServerFrame(cTgt, nil)
			if err != nil {
				return
			}
			tgtPackets <- pkt{op: op, data: data}
		}
	}()

	// 1. Send summon request to target
	target.sendSummonRequest(10, 12)

	var p pkt
	select {
	case p = <-tgtPackets:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_SUMMON_REQUEST")
	}
	if p.op != uint16(protocol.OpcodeSMSG_SUMMON_REQUEST) {
		t.Fatalf("expected SMSG_SUMMON_REQUEST (0x2AB), got 0x%04X", p.op)
	}
	r := protocol.NewReader(p.data)
	sGuid, _ := r.ReadU64()
	zone, _ := r.ReadU32()
	dur, _ := r.ReadU32()
	if sGuid != 10 || zone != 12 || dur != 120000 {
		t.Fatalf("invalid summon request packet: guid=%d zone=%d dur=%d", sGuid, zone, dur)
	}
	if target.summonExpire.IsZero() || target.summonerGUID != 10 {
		t.Fatal("expected target session to track pending summon state")
	}

	// 2. Reject if target in combat
	target.player.UnitFlags |= unitFlagInCombat
	respBuf := protocol.NewBuffer(9)
	respBuf.WriteU64(10)
	respBuf.WriteU8(1) // agree

	if !target.handleSummonResponse(context.Background(), respBuf.Bytes()) {
		t.Fatal("handleSummonResponse failed")
	}
	if target.player.Map != 1 {
		t.Fatal("target should NOT teleport while in combat")
	}

	// 3. Out of combat, re-request and accept -> should teleport
	target.player.UnitFlags &^= unitFlagInCombat
	target.sendSummonRequest(10, 12)

	select {
	case <-tgtPackets: // drain second request
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for second SMSG_SUMMON_REQUEST")
	}

	if !target.handleSummonResponse(context.Background(), respBuf.Bytes()) {
		t.Fatal("handleSummonResponse failed")
	}
	if target.player.Map != 0 || target.player.X != 100 || target.player.Y != 200 || target.player.Z != 300 {
		t.Fatalf("expected target teleported to (0, 100, 200, 300), got (%d, %f, %f, %f)",
			target.player.Map, target.player.X, target.player.Y, target.player.Z)
	}
	if !target.summonExpire.IsZero() || target.summonerGUID != 0 {
		t.Fatal("expected summon state cleared after teleport")
	}
}
