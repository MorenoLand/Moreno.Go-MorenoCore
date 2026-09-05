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

func TestVehicleSeatsAndPassengerFlow(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	driver := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   100,
		player: &playerState{
			GUID: 100,
			Map:  0,
			X:    100,
			Y:    100,
			Z:    10,
		},
	}

	passenger := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   200,
		player: &playerState{
			GUID:           200,
			Map:            0,
			X:              102,
			Y:              100,
			Z:              10,
			MountDisplayID: 55,
		},
	}

	srv.sessions[driver] = struct{}{}
	srv.sessions[passenger] = struct{}{}

	ctx := context.Background()

	// 1. Passenger enters driver vehicle
	enterBuf := protocol.NewBuffer(9)
	enterBuf.WriteU64(100)
	enterBuf.WriteU8(1)
	if !passenger.handlePlayerVehicleEnter(ctx, enterBuf.Bytes()) {
		t.Fatal("handlePlayerVehicleEnter failed")
	}
	if passenger.player.VehicleGUID != 100 || passenger.player.VehicleSeat != 1 {
		t.Fatalf("expected passenger in vehicle 100 seat 1, got veh=%d seat=%d",
			passenger.player.VehicleGUID, passenger.player.VehicleSeat)
	}
	if passenger.player.MountDisplayID != 0 {
		t.Fatal("expected passenger dismounted upon entering vehicle")
	}

	// 2. Next seat
	if !passenger.handleRequestVehicleNextSeat(ctx, nil) {
		t.Fatal("handleRequestVehicleNextSeat failed")
	}
	if passenger.player.VehicleSeat != 2 {
		t.Fatalf("expected seat 2, got %d", passenger.player.VehicleSeat)
	}

	// 3. Prev seat
	if !passenger.handleRequestVehiclePrevSeat(ctx, nil) {
		t.Fatal("handleRequestVehiclePrevSeat failed")
	}
	if passenger.player.VehicleSeat != 1 {
		t.Fatalf("expected seat 1, got %d", passenger.player.VehicleSeat)
	}

	// 4. Switch seat
	if !passenger.handleRequestVehicleSwitchSeat(ctx, []byte{3}) {
		t.Fatal("handleRequestVehicleSwitchSeat failed")
	}
	if passenger.player.VehicleSeat != 3 {
		t.Fatalf("expected seat 3, got %d", passenger.player.VehicleSeat)
	}

	// 5. Change seats on controlled vehicle
	changeBuf := protocol.NewBuffer(9)
	changeBuf.WriteU64(100)
	changeBuf.WriteU8(4)
	if !passenger.handleChangeSeatsOnControlledVehicle(ctx, changeBuf.Bytes()) {
		t.Fatal("handleChangeSeatsOnControlledVehicle failed")
	}
	if passenger.player.VehicleSeat != 4 {
		t.Fatalf("expected seat 4, got %d", passenger.player.VehicleSeat)
	}

	// 6. Driver ejects passenger
	ejectBuf := protocol.NewBuffer(8)
	ejectBuf.WriteU64(200)
	if !driver.handleControllerEjectPassenger(ctx, ejectBuf.Bytes()) {
		t.Fatal("handleControllerEjectPassenger failed")
	}
	if passenger.player.VehicleGUID != 0 || passenger.player.VehicleSeat != 0 {
		t.Fatalf("expected passenger ejected, got veh=%d seat=%d",
			passenger.player.VehicleGUID, passenger.player.VehicleSeat)
	}

	// 7. Passenger re-enters, driver dismisses vehicle
	if !passenger.handlePlayerVehicleEnter(ctx, enterBuf.Bytes()) {
		t.Fatal("handlePlayerVehicleEnter failed")
	}
	if passenger.player.VehicleGUID != 100 {
		t.Fatal("expected passenger re-entered vehicle")
	}
	if !driver.handleDismissControlledVehicle(ctx, nil) {
		t.Fatal("handleDismissControlledVehicle failed")
	}
	if passenger.player.VehicleGUID != 0 {
		t.Fatal("expected passenger ejected on driver vehicle dismiss")
	}

	// 8. Passenger self exit
	_ = passenger.handlePlayerVehicleEnter(ctx, enterBuf.Bytes())
	if !passenger.handleRequestVehicleExit(ctx, nil) {
		t.Fatal("handleRequestVehicleExit failed")
	}
	if passenger.player.VehicleGUID != 0 {
		t.Fatal("expected passenger exited vehicle")
	}
}

func TestFallDamage_HeightThresholdAndDamageScaling(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
			X:         0,
			Y:         0,
			Z:         50.0,
		},
		lastFallZ:    50.0,
		lastFallTime: 100,
	}
	srv.sessions[sess] = struct{}{}

	receivedOpcodes := make(chan uint16, 16)
	receivedPayloads := make(chan []byte, 16)
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

	// 1. Small fall: from 50.0 to 45.0 (zDiff = 5.0 < 14.57) -> no damage
	infoSmall := movementInfo{
		GUID:     10,
		Time:     200,
		X:        0,
		Y:        0,
		Z:        45.0,
		FallTime: 200,
	}
	bufSmall := protocol.NewBuffer(64)
	writeMovementInfo(bufSmall, infoSmall)

	if !sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), bufSmall.Bytes()) {
		t.Fatal("handleMovement MSG_MOVE_FALL_LAND failed")
	}
	if sess.player.Health != 1000 {
		t.Fatalf("expected health 1000 after small fall, got %d", sess.player.Health)
	}
	if sess.lastFallZ != 45.0 {
		t.Fatalf("expected lastFallZ updated to 45.0, got %f", sess.lastFallZ)
	}

	// 2. Fall with damage: set apex at 50.0, land at 25.0 (zDiff = 25.0 >= 14.57)
	// damageperc = 0.018 * 25.0 - 0.2426 = 0.2074 -> 207 damage on 1000 max health
	sess.lastFallZ = 50.0
	infoDamage := movementInfo{
		GUID:     10,
		Time:     400,
		X:        0,
		Y:        0,
		Z:        25.0,
		FallTime: 400,
	}
	bufDamage := protocol.NewBuffer(64)
	writeMovementInfo(bufDamage, infoDamage)

	if !sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), bufDamage.Bytes()) {
		t.Fatal("handleMovement MSG_MOVE_FALL_LAND failed")
	}
	if sess.player.Health != 793 {
		t.Fatalf("expected health 793 (1000 - 207), got %d", sess.player.Health)
	}

	select {
	case op := <-receivedOpcodes:
		if op != uint16(protocol.OpcodeSMSG_ENVIRONMENTAL_DAMAGE_LOG) {
			t.Fatalf("expected SMSG_ENVIRONMENTAL_DAMAGE_LOG, got 0x%04X", op)
		}
		p := <-receivedPayloads
		r := protocol.NewReader(p)
		vicGUID, _ := r.ReadU64()
		dmgType, _ := r.ReadU8()
		amount, _ := r.ReadU32()
		if vicGUID != 10 || dmgType != 2 || amount != 207 {
			t.Fatalf("unexpected damage log: victim=%d type=%d amount=%d", vicGUID, dmgType, amount)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for SMSG_ENVIRONMENTAL_DAMAGE_LOG")
	}

	// 3. Lethal fall: set apex at 100.0, land at 0.0 (zDiff = 100.0) -> fatal fall damage
	sess.lastFallZ = 100.0
	infoLethal := movementInfo{
		GUID:     10,
		Time:     600,
		X:        0,
		Y:        0,
		Z:        0.0,
		FallTime: 600,
	}
	bufLethal := protocol.NewBuffer(64)
	writeMovementInfo(bufLethal, infoLethal)

	if !sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), bufLethal.Bytes()) {
		t.Fatal("handleMovement lethal MSG_MOVE_FALL_LAND failed")
	}
	if sess.player.Health != 0 {
		t.Fatalf("expected health 0 after lethal fall, got %d", sess.player.Health)
	}
	if sess.player.PlayerFieldBytes&playerFieldByteReleaseTimer == 0 {
		t.Fatal("expected playerFieldByteReleaseTimer flag set upon death")
	}
}

func TestFallDamage_ImmunitiesAndSafeFall(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
			X:         0,
			Y:         0,
			Z:         50.0,
		},
		lastFallZ:    50.0,
		lastFallTime: 100,
	}
	srv.sessions[sess] = struct{}{}

	go func() {
		for {
			_, _, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
		}
	}()

	infoFall := movementInfo{
		GUID:     10,
		Time:     200,
		X:        0,
		Y:        0,
		Z:        0.0, // 50 yards fall
		FallTime: 500,
	}
	buf := protocol.NewBuffer(64)
	writeMovementInfo(buf, infoFall)

	// 1. Feather Fall aura immunity (AuraType 105)
	sess.activeAuras = map[uint32]*activeAura{
		130: {SpellID: 130, AuraType: 105},
	}
	sess.auras = map[uint32]struct{}{130: {}}
	sess.lastFallZ = 50.0
	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), buf.Bytes())
	if sess.player.Health != 1000 {
		t.Fatalf("expected Feather Fall to prevent fall damage, health=%d", sess.player.Health)
	}

	// 2. Flight immunity (inFlight = true)
	sess.activeAuras = nil
	sess.auras = nil
	sess.inFlight = true
	sess.lastFallZ = 50.0
	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), buf.Bytes())
	if sess.player.Health != 1000 {
		t.Fatalf("expected inFlight to prevent fall damage, health=%d", sess.player.Health)
	}

	// 3. GM immunity
	sess.inFlight = false
	sess.player.PlayerFlags |= playerFlagGM
	sess.lastFallZ = 50.0
	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), buf.Bytes())
	if sess.player.Health != 1000 {
		t.Fatalf("expected GM mode to prevent fall damage, health=%d", sess.player.Health)
	}
	sess.player.PlayerFlags &^= playerFlagGM

	// 4. Safe Fall passive reduction (spell 1860, 17 yards reduction)
	sess.player.Spells = []learnedSpell{{ID: 1860, Active: true}}
	sess.lastFallZ = 50.0
	info25Yards := movementInfo{
		GUID:     10,
		Time:     400,
		X:        0,
		Y:        0,
		Z:        25.0, // 25 yards fall
		FallTime: 300,
	}
	buf25 := protocol.NewBuffer(64)
	writeMovementInfo(buf25, info25Yards)

	// 25 - 17 = 8 yards effective fall (< 14.57 threshold) -> no damage
	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), buf25.Bytes())
	if sess.player.Health != 1000 {
		t.Fatalf("expected Safe Fall to negate 25 yard fall damage, health=%d", sess.player.Health)
	}
}

func TestLanding_ParachuteAuraRemoval(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   10,
		player: &playerState{
			GUID:      10,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
			X:         0,
			Y:         0,
			Z:         5.0,
		},
		activeAuras: make(map[uint32]*activeAura),
		auras:       make(map[uint32]struct{}),
		auraSlots:   make(map[uint32]uint8),
		lastFallZ:   5.0,
	}
	srv.sessions[sess] = struct{}{}

	go func() {
		for {
			_, _, err := readServerFrame(clientConn, nil)
			if err != nil {
				return
			}
		}
	}()

	// 1. Parachute removed on landing (MSG_MOVE_FALL_LAND)
	sess.activeAuras[54649] = &activeAura{SpellID: 54649, AuraInterruptFlags: 0x02000000}
	sess.auras[54649] = struct{}{}
	sess.auraSlots[54649] = 0

	infoLand := movementInfo{GUID: 10, Time: 100, X: 0, Y: 0, Z: 5.0}
	bufLand := protocol.NewBuffer(64)
	writeMovementInfo(bufLand, infoLand)

	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_FALL_LAND), bufLand.Bytes())
	if _, exists := sess.activeAuras[54649]; exists {
		t.Fatal("expected parachute aura to be removed from activeAuras upon landing")
	}
	if _, exists := sess.auras[54649]; exists {
		t.Fatal("expected parachute aura to be removed from auras upon landing")
	}

	// 2. Parachute removed on swimming (MSG_MOVE_START_SWIM)
	sess.activeAuras[54649] = &activeAura{SpellID: 54649, AuraInterruptFlags: 0x02000000}
	sess.auras[54649] = struct{}{}
	sess.auraSlots[54649] = 0

	infoSwim := movementInfo{GUID: 10, Time: 200, X: 0, Y: 0, Z: 5.0, Flags: movementSwimming, HasPitch: true}
	bufSwim := protocol.NewBuffer(64)
	writeMovementInfo(bufSwim, infoSwim)

	sess.handleMovement(context.Background(), uint32(protocol.OpcodeMSG_MOVE_START_SWIM), bufSwim.Bytes())
	if _, exists := sess.activeAuras[54649]; exists {
		t.Fatal("expected parachute aura to be removed upon start swimming")
	}
	if _, exists := sess.auras[54649]; exists {
		t.Fatal("expected parachute aura to be removed from auras upon start swimming")
	}
}
