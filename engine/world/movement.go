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
