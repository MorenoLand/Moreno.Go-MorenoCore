package world

import (
	"context"
	"encoding/hex"
	"math"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	movementOnTransport     uint32 = 0x00000200
	movementFalling         uint32 = 0x00001000
	movementSwimming        uint32 = 0x00200000
	movementFlying          uint32 = 0x02000000
	movementSplineElevation uint32 = 0x04000000
	movementRoot            uint32 = 0x00000800
	movementForward         uint32 = 0x00000001
	movementBackward        uint32 = 0x00000002
	movementStrafeLeft      uint32 = 0x00000004
	movementStrafeRight     uint32 = 0x00000008
	movementTurnLeft        uint32 = 0x00000010
	movementTurnRight       uint32 = 0x00000020
	movementAscending       uint32 = 0x00400000
	movementDescending      uint32 = 0x00800000
	movement2Pitch          uint16 = 0x00000020
	movement2Interpolated   uint16 = 0x00000400
	maxPositionCoordinate          = 17066.666
)

type movementInfo struct {
	GUID            uint64
	Flags           uint32
	Flags2          uint16
	Time            uint32
	X               float32
	Y               float32
	Z               float32
	Orientation     float32
	Transport       *transportMovement
	Pitch           float32
	HasPitch        bool
	FallTime        uint32
	Jump            [4]float32
	HasJump         bool
	SplineElevation float32
	HasSpline       bool
}

type transportMovement struct {
	GUID        uint64
	X           float32
	Y           float32
	Z           float32
	Orientation float32
	Time        uint32
	Seat        int8
	Time2       uint32
	HasTime2    bool
}

func (s *session) handleMovement(ctx context.Context, opcode uint32, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	b := protocol.NewReader(payload)
	guid, err := b.ReadPackedGUID()
	if err != nil {
		s.debug("movement rejected", "account", s.accountName, "reason", "malformed guid", "opcode", opcode)
		return false
	}
	if guid != s.playerGUID {
		prefix := payload
		if len(prefix) > 24 {
			prefix = prefix[:24]
		}
		s.debug("movement rejected", "account", s.accountName, "reason", "mover mismatch", "guid", guid, "expected", s.playerGUID, "prefix", hex.EncodeToString(prefix))
		return true
	}
	info, err := readMovementInfo(b)
	if err != nil {
		s.debug("movement rejected", "account", s.accountName, "reason", "malformed movement", "opcode", opcode, "error", err)
		return false
	}
	info.GUID = guid
	if !validMovementPosition(info.X, info.Y, info.Z, info.Orientation) {
		s.debug("movement rejected", "account", s.accountName, "reason", "invalid position", "guid", guid)
		return true
	}
	info.Flags &^= movementRoot
	info.Flags = sanitizeMovementFlags(info.Flags)
	if info.Flags&(movementForward|movementBackward|movementStrafeLeft|movementStrafeRight|movementFalling) != 0 {
		s.interruptCurrentCast()
	}
	s.player.X, s.player.Y, s.player.Z, s.player.Orientation = info.X, info.Y, info.Z, info.Orientation
	if opcode == uint32(protocol.OpcodeMSG_MOVE_STOP) || opcode == uint32(protocol.OpcodeMSG_MOVE_HEARTBEAT) || opcode == uint32(protocol.OpcodeMSG_MOVE_FALL_LAND) {
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
				"UPDATE characters SET position_x = ?, position_y = ?, position_z = ?, orientation = ?, map = ?, zone = ? WHERE guid = ?",
				info.X, info.Y, info.Z, info.Orientation, s.player.Map, s.player.Zone, s.playerGUID)
		}
	}
	packet := protocol.NewBuffer(len(payload))
	writeMovementInfo(packet, info)
	s.server.broadcastMovement(uint16(opcode), packet.Bytes(), info, s)
	s.debug("movement accepted", "account", s.accountName, "guid", guid, "x", info.X, "y", info.Y, "z", info.Z)
	dx := float64(info.X - s.lastStreamX)
	dy := float64(info.Y - s.lastStreamY)
	if dx*dx+dy*dy > 30.0*30.0 {
		s.streamNearbyObjects(ctx)
	}
	return true
}

func (s *session) handleSetActiveMover(payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	s.debug("active mover received", "account", s.accountName, "guid", guid, "expected", s.playerGUID)
	return true
}

func (s *session) handleTimeSyncResponse(payload []byte) bool {
	reader := protocol.NewReader(payload)
	counter, err := reader.ReadU32()
	if err != nil {
		return false
	}
	clientTime, err := reader.ReadU32()
	if err != nil {
		return false
	}
	s.debug("time sync response", "account", s.accountName, "counter", counter, "client_time", clientTime)
	return true
}

func readMovementInfo(b *protocol.Buffer) (movementInfo, error) {
	var result movementInfo
	var err error
	if result.Flags, err = b.ReadU32(); err != nil {
		return result, err
	}
	if result.Flags2, err = b.ReadU16(); err != nil {
		return result, err
	}
	if result.Time, err = b.ReadU32(); err != nil {
		return result, err
	}
	values := []*float32{&result.X, &result.Y, &result.Z, &result.Orientation}
	for _, value := range values {
		if *value, err = b.ReadF32(); err != nil {
			return result, err
		}
	}
	if result.Flags&movementOnTransport != 0 {
		transport := &transportMovement{}
		if transport.GUID, err = b.ReadPackedGUID(); err != nil {
			return result, err
		}
		values = []*float32{&transport.X, &transport.Y, &transport.Z, &transport.Orientation}
		for _, value := range values {
			if *value, err = b.ReadF32(); err != nil {
				return result, err
			}
		}
		if transport.Time, err = b.ReadU32(); err != nil {
			return result, err
		}
		if transport.Seat, err = b.ReadI8(); err != nil {
			return result, err
		}
		if result.Flags2&movement2Interpolated != 0 {
			if transport.Time2, err = b.ReadU32(); err != nil {
				return result, err
			}
			transport.HasTime2 = true
		}
		result.Transport = transport
	}
	if result.Flags&(movementSwimming|movementFlying) != 0 || result.Flags2&movement2Pitch != 0 {
		if result.Pitch, err = b.ReadF32(); err != nil {
			return result, err
		}
		result.HasPitch = true
	}
	if result.FallTime, err = b.ReadU32(); err != nil {
		return result, err
	}
	if result.Flags&movementFalling != 0 {
		for index := range result.Jump {
			if result.Jump[index], err = b.ReadF32(); err != nil {
				return result, err
			}
		}
		result.HasJump = true
	}
	if result.Flags&movementSplineElevation != 0 {
		if result.SplineElevation, err = b.ReadF32(); err != nil {
			return result, err
		}
		result.HasSpline = true
	}
	return result, nil
}

func writeMovementInfo(b *protocol.Buffer, info movementInfo) {
	b.WritePackedGUID(info.GUID)
	b.WriteU32(info.Flags)
	b.WriteU16(info.Flags2)
	b.WriteU32(info.Time)
	b.WriteF32(info.X)
	b.WriteF32(info.Y)
	b.WriteF32(info.Z)
	b.WriteF32(info.Orientation)
	if info.Flags&movementOnTransport != 0 && info.Transport != nil {
		b.WritePackedGUID(info.Transport.GUID)
		b.WriteF32(info.Transport.X)
		b.WriteF32(info.Transport.Y)
		b.WriteF32(info.Transport.Z)
		b.WriteF32(info.Transport.Orientation)
		b.WriteU32(info.Transport.Time)
		b.WriteI8(info.Transport.Seat)
		if info.Flags2&movement2Interpolated != 0 && info.Transport.HasTime2 {
			b.WriteU32(info.Transport.Time2)
		}
	}
	if info.HasPitch {
		b.WriteF32(info.Pitch)
	}
	b.WriteU32(info.FallTime)
	if info.HasJump {
		for _, value := range info.Jump {
			b.WriteF32(value)
		}
	}
	if info.HasSpline {
		b.WriteF32(info.SplineElevation)
	}
}

func sanitizeMovementFlags(flags uint32) uint32 {
	if flags&movementForward != 0 && flags&movementBackward != 0 {
		flags &^= movementForward | movementBackward
	}
	if flags&movementStrafeLeft != 0 && flags&movementStrafeRight != 0 {
		flags &^= movementStrafeLeft | movementStrafeRight
	}
	if flags&movementTurnLeft != 0 && flags&movementTurnRight != 0 {
		flags &^= movementTurnLeft | movementTurnRight
	}
	if flags&movementAscending != 0 && flags&movementDescending != 0 {
		flags &^= movementAscending | movementDescending
	}
	return flags
}

func validMovementPosition(x, y, z, orientation float32) bool {
	for _, value := range []float32{x, y, z, orientation} {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}
	return math.Abs(float64(x)) <= maxPositionCoordinate && math.Abs(float64(y)) <= maxPositionCoordinate && math.Abs(float64(z)) <= maxPositionCoordinate
}

func (s *Server) addSession(value *session) {
	s.sessionsMu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[*session]struct{})
	}
	s.sessions[value] = struct{}{}
	s.sessionsMu.Unlock()
}

func (s *Server) removeSession(session *session) {
	s.sessionsMu.Lock()
	delete(s.sessions, session)
	s.sessionsMu.Unlock()
}

func (s *Server) broadcastMovement(opcode uint16, payload []byte, info movementInfo, source *session) {
	s.sessionsMu.RLock()
	targets := make([]*session, 0, len(s.sessions))
	for target := range s.sessions {
		if target == source || !target.authed || !target.playerLoaded || target.player == nil || target.player.Map != source.player.Map {
			continue
		}
		targets = append(targets, target)
	}
	s.sessionsMu.RUnlock()
	for _, target := range targets {
		if err := target.write(opcode, payload, true); err != nil {
			target.debug("movement broadcast failed", "account", target.accountName, "guid", info.GUID, "error", err)
		}
	}
}

// handleForceMoveRootAck processes CMSG_FORCE_MOVE_ROOT_ACK (0x0E9).
// Reference: WorldSession::HandleMoveRootAck (MiscHandler.cpp:945).
func (s *session) handleForceMoveRootAck(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	b := protocol.NewReader(payload)
	_, _ = b.ReadPackedGUID()
	_, _ = b.ReadU32() // ack index
	info, err := readMovementInfo(b)
	if err == nil && validMovementPosition(info.X, info.Y, info.Z, info.Orientation) {
		s.player.X, s.player.Y, s.player.Z, s.player.Orientation = info.X, info.Y, info.Z, info.Orientation
	}
	return true
}

// handleForceMoveUnrootAck processes CMSG_FORCE_MOVE_UNROOT_ACK (0x0EB).
// Reference: WorldSession::HandleMoveUnRootAck (MiscHandler.cpp:919).
func (s *session) handleForceMoveUnrootAck(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	b := protocol.NewReader(payload)
	_, _ = b.ReadPackedGUID()
	_, _ = b.ReadU32() // ack index
	info, err := readMovementInfo(b)
	if err == nil && validMovementPosition(info.X, info.Y, info.Z, info.Orientation) {
		s.player.X, s.player.Y, s.player.Z, s.player.Orientation = info.X, info.Y, info.Z, info.Orientation
	}
	return true
}

func (s *session) handleMovementAck(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	b := protocol.NewReader(payload)
	_, _ = b.ReadPackedGUID()
	_, _ = b.ReadU32() // ack index
	info, err := readMovementInfo(b)
	if err == nil && validMovementPosition(info.X, info.Y, info.Z, info.Orientation) {
		s.player.X, s.player.Y, s.player.Z, s.player.Orientation = info.X, info.Y, info.Z, info.Orientation
	}
	return true
}

// handleForceTurnRateChangeAck processes CMSG_FORCE_TURN_RATE_CHANGE_ACK (0x2DF).
func (s *session) handleForceTurnRateChangeAck(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

func (s *Server) broadcastToNearby(opcode uint16, payload []byte, source *session) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for target := range s.sessions {
		if !target.authed || !target.playerLoaded || target.player == nil {
			continue
		}
		if source != nil && (target == source || target.player.Map != source.player.Map) {
			continue
		}
		_ = target.write(opcode, payload, true)
	}
}

// handleMoveFeatherFallAck processes CMSG_MOVE_FEATHER_FALL_ACK (0x2CF).
func (s *session) handleMoveFeatherFallAck(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

// handleMoveHoverAck processes CMSG_MOVE_HOVER_ACK (0x0F6).
func (s *session) handleMoveHoverAck(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

// handleMoveWaterWalkAck processes CMSG_MOVE_WATER_WALK_ACK (0x2D0).
func (s *session) handleMoveWaterWalkAck(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

// handleMoveKnockBackAck processes CMSG_MOVE_KNOCK_BACK_ACK (0x0F0).
func (s *session) handleMoveKnockBackAck(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

// handleMoveNotActiveMover processes CMSG_MOVE_NOT_ACTIVE_MOVER (0x2D1).
func (s *session) handleMoveNotActiveMover(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

// handleMoveFallReset processes CMSG_MOVE_FALL_RESET (0x0CA).
func (s *session) handleMoveFallReset(ctx context.Context, payload []byte) bool {
	return s.handleMovementAck(ctx, payload)
}

// handleMoveSplineDone processes CMSG_MOVE_SPLINE_DONE (0x2C9).
// Reference: WorldSession::HandleMoveSplineDoneOpcode (TaxiHandler.cpp:201).
func (s *session) handleMoveSplineDone(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	b := protocol.NewReader(payload)
	_, _ = b.ReadPackedGUID()
	info, err := readMovementInfo(b)
	if err == nil && validMovementPosition(info.X, info.Y, info.Z, info.Orientation) {
		s.player.X, s.player.Y, s.player.Z, s.player.Orientation = info.X, info.Y, info.Z, info.Orientation
		s.sendPlayerUpdate()
	}
	return true
}

// handleMoveChngTransport processes CMSG_MOVE_CHNG_TRANSPORT (0x38D).
func (s *session) handleMoveChngTransport(ctx context.Context, payload []byte) bool {
	return true
}

// handleMoveSetFly processes CMSG_MOVE_SET_FLY (0x0D6).
func (s *session) handleMoveSetFly(ctx context.Context, payload []byte) bool {
	return true
}

// handleMoveTimeSkipped processes CMSG_MOVE_TIME_SKIPPED (0x2CE).
func (s *session) handleMoveTimeSkipped(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	buf := protocol.NewBuffer(len(payload) + 8)
	buf.WritePackedGUID(s.playerGUID)
	buf.Write(payload)
	s.server.broadcastToNearby(uint16(protocol.OpcodeMSG_MOVE_TIME_SKIPPED), buf.Bytes(), s)
	return true
}

// handleSummonResponse processes CMSG_SUMMON_RESPONSE (0x1AC).
// Reference: WorldSession::HandleSummonResponseOpcode (MovementHandler.cpp:613).
func (s *session) handleSummonResponse(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	summonerGUID, _ := r.ReadU64()
	agree, _ := r.ReadU8()
	if agree == 0 || s.player.Health == 0 {
		return true
	}

	if s.server != nil {
		summonerSess := s.server.findSessionByGUID(summonerGUID)
		if summonerSess != nil && summonerSess.playerLoaded && summonerSess.player != nil {
			s.player.Map = summonerSess.player.Map
			s.player.X = summonerSess.player.X
			s.player.Y = summonerSess.player.Y
			s.player.Z = summonerSess.player.Z
			s.player.Orientation = summonerSess.player.Orientation
			s.sendPlayerUpdate()
		}
	}
	return true
}

// handleMountSpecialAnim processes CMSG_MOUNTSPECIAL_ANIM (0x171).
func (s *session) handleMountSpecialAnim(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	buf := protocol.NewBuffer(8)
	buf.WritePackedGUID(s.playerGUID)
	s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_MOUNTSPECIAL_ANIM), buf.Bytes(), s)
	return true
}

// handleChangeSeatsOnControlledVehicle processes CMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE (0x49B).
// Reference: WorldSession::HandleChangeSeatsOnControlledVehicle (VehicleHandler.cpp:52).
func (s *session) handleChangeSeatsOnControlledVehicle(ctx context.Context, payload []byte) bool {
	if len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	vehGUID, _ := r.ReadU64()
	seat, _ := r.ReadU8()
	s.debug("change seats on vehicle", "account", s.accountName, "vehicle", vehGUID, "seat", seat)
	return true
}

// handleControllerEjectPassenger processes CMSG_CONTROLLER_EJECT_PASSENGER (0x4A9).
// Reference: WorldSession::HandleEjectPassenger (VehicleHandler.cpp:151).
func (s *session) handleControllerEjectPassenger(ctx context.Context, payload []byte) bool {
	if len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	passGUID, _ := r.ReadU64()
	s.debug("controller eject passenger", "account", s.accountName, "passenger", passGUID)
	return true
}

// handleDismissControlledVehicle processes CMSG_DISMISS_CONTROLLED_VEHICLE (0x46D).
// Reference: WorldSession::HandleDismissControlledVehicle (VehicleHandler.cpp:27).
func (s *session) handleDismissControlledVehicle(ctx context.Context, payload []byte) bool {
	if len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	vehGUID, _ := r.ReadU64()
	s.debug("dismiss controlled vehicle", "account", s.accountName, "vehicle", vehGUID)
	return true
}

// handlePlayerVehicleEnter processes CMSG_PLAYER_VEHICLE_ENTER (0x46E).
// Reference: WorldSession::HandleEnterPlayerVehicle (VehicleHandler.cpp:129).
func (s *session) handlePlayerVehicleEnter(ctx context.Context, payload []byte) bool {
	if len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	vehGUID, _ := r.ReadU64()
	seat, _ := r.ReadU8()
	s.debug("player vehicle enter", "account", s.accountName, "vehicle", vehGUID, "seat", seat)
	return true
}

// handleRequestVehicleExit processes CMSG_REQUEST_VEHICLE_EXIT (0x46F).
// Reference: WorldSession::HandleRequestVehicleExit (VehicleHandler.cpp:190).
func (s *session) handleRequestVehicleExit(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	s.sendPlayerUpdate()
	return true
}

// handleRequestVehicleNextSeat processes CMSG_REQUEST_VEHICLE_NEXT_SEAT (0x470).
// Reference: WorldSession::HandleChangeSeatsOnControlledVehicle (VehicleHandler.cpp:77).
func (s *session) handleRequestVehicleNextSeat(ctx context.Context, payload []byte) bool {
	s.debug("request vehicle next seat", "account", s.accountName)
	return true
}

// handleRequestVehiclePrevSeat processes CMSG_REQUEST_VEHICLE_PREV_SEAT (0x471).
// Reference: WorldSession::HandleChangeSeatsOnControlledVehicle (VehicleHandler.cpp:74).
func (s *session) handleRequestVehiclePrevSeat(ctx context.Context, payload []byte) bool {
	s.debug("request vehicle prev seat", "account", s.accountName)
	return true
}

// handleRequestVehicleSwitchSeat processes CMSG_REQUEST_VEHICLE_SWITCH_SEAT (0x472).
// Reference: WorldSession::HandleChangeSeatsOnControlledVehicle (VehicleHandler.cpp:108).
func (s *session) handleRequestVehicleSwitchSeat(ctx context.Context, payload []byte) bool {
	if len(payload) < 1 {
		return true
	}
	seat := payload[0]
	s.debug("request vehicle switch seat", "account", s.accountName, "seat", seat)
	return true
}
