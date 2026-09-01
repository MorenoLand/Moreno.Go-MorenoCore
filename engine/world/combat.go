package world

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const meleeAttackRange = 5.0

type combatTarget struct {
	GUID   uint64
	Map    uint32
	X      float32
	Y      float32
	Z      float32
	Health uint32
}

func (s *session) handleAttackSwing(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	victim, err := reader.ReadU64()
	if err != nil {
		s.debug("attack rejected", "account", s.accountName, "error", err)
		return false
	}
	target, err := s.loadCombatTarget(ctx, victim)
	if errors.Is(err, sql.ErrNoRows) {
		s.attackTarget = 0
		return s.sendAttackStop(0, false) == nil
	}
	if err != nil {
		s.debug("attack target lookup failed", "account", s.accountName, "guid", victim, "error", err)
		return false
	}
	if target.Health == 0 {
		s.attackTarget = 0
		return s.write(uint16(protocol.OpcodeSMSG_ATTACK_SWING_DEAD_TARGET), nil, true) == nil
	}
	if target.Map != s.player.Map || distance3D(s.player.X, s.player.Y, s.player.Z, target.X, target.Y, target.Z) > meleeAttackRange {
		s.debug("attack out of range", "account", s.accountName, "guid", victim)
		return s.write(uint16(protocol.OpcodeSMSG_ATTACK_SWING_NOT_IN_RANGE), nil, true) == nil
	}
	if s.attackTarget != 0 && s.attackTarget != victim {
		if err := s.sendAttackStop(s.attackTarget, false); err != nil {
			return false
		}
	}
	s.attackTarget = victim
	s.debug("attack started", "account", s.accountName, "guid", victim)
	return s.write(uint16(protocol.OpcodeSMSG_ATTACK_START), buildAttackStart(s.playerGUID, victim), true) == nil
}

func (s *session) handleAttackStop() bool {
	if !s.playerLoaded {
		return true
	}
	victim := s.attackTarget
	s.attackTarget = 0
	if err := s.sendAttackStop(victim, false); err != nil {
		s.debug("attack stop failed", "account", s.accountName, "error", err)
		return false
	}
	s.debug("attack stopped", "account", s.accountName, "guid", victim)
	return true
}

func (s *session) handleSetSheathed(payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	reader := protocol.NewReader(payload)
	state, err := reader.ReadU32()
	if err != nil {
		return false
	}
	s.player.SheathState = uint8(state)
	s.sendPlayerUpdate()
	s.debug("sheath state changed", "account", s.accountName, "state", state)
	return true
}

func (s *session) sendAttackStop(victim uint64, nowDead bool) error {
	return s.write(uint16(protocol.OpcodeSMSG_ATTACK_STOP), buildAttackStop(s.playerGUID, victim, nowDead), true)
}

func buildAttackStart(attacker, victim uint64) []byte {
	packet := protocol.NewBuffer(16)
	packet.WriteU64(attacker)
	packet.WriteU64(victim)
	return packet.Bytes()
}

func buildAttackStop(attacker, victim uint64, nowDead bool) []byte {
	packet := protocol.NewBuffer(24)
	packet.WritePackedGUID(attacker)
	packet.WritePackedGUID(victim)
	if nowDead {
		packet.WriteU32(1)
	} else {
		packet.WriteU32(0)
	}
	return packet.Bytes()
}

func (s *session) loadCombatTarget(ctx context.Context, guid uint64) (combatTarget, error) {
	var target combatTarget
	var low, entry, mapID int64
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT guid, id, map, position_x, position_y, position_z, curhealth FROM creature WHERE guid = ?", uint32(guid&0xFFFFFF)).Scan(&low, &entry, &mapID, &target.X, &target.Y, &target.Z, &target.Health); err != nil {
		return target, err
	}
	target.GUID = creatureWorldGUID(uint32(low), uint32(entry))
	if target.GUID != guid {
		return combatTarget{}, sql.ErrNoRows
	}
	target.Map = uint32(mapID)
	return target, nil
}

func distance3D(x1, y1, z1, x2, y2, z2 float32) float64 {
	dx := float64(x1 - x2)
	dy := float64(y1 - y2)
	dz := float64(z1 - z2)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
