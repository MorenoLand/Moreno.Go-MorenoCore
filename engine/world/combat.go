package world

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

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
	if target.Map != s.player.Map {
		s.attackTarget = 0
		return s.sendAttackStop(victim, false) == nil
	}
	if s.attackTarget != 0 && s.attackTarget != victim {
		if err := s.sendAttackStop(s.attackTarget, false); err != nil {
			return false
		}
	}
	s.attackTarget = victim
	s.lastSwing = time.Now()
	s.debug("attack started", "account", s.accountName, "guid", victim)
	_ = s.write(uint16(protocol.OpcodeSMSG_ATTACK_START), buildAttackStart(s.playerGUID, victim), true)
	if distance3D(s.player.X, s.player.Y, s.player.Z, target.X, target.Y, target.Z) <= meleeAttackRange+2.0 {
		s.executeMeleeSwing(ctx, target)
	}
	return true
}

func (s *session) executeMeleeSwing(ctx context.Context, target combatTarget) {
	if s.player == nil || target.Health == 0 {
		return
	}
	damage := uint32(20 + int(s.player.Level)*5)
	overkill := uint32(0)
	if damage >= target.Health {
		overkill = damage - target.Health
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE), buildAttackerStateUpdate(s.playerGUID, target.GUID, damage, overkill), true)
	cdb := s.server.WorldStore.DB
	if damage >= target.Health {
		// Target dies
		if cdb != nil {
			_, _ = cdb.ExecContext(ctx, "UPDATE creature SET curhealth = 0 WHERE guid = ?", uint32(target.GUID&0xFFFFFF))
		}
		s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{
			unitFieldHealth:       0,
			unitFieldDynamicFlags: 1, // UNIT_DYNFLAG_LOOTABLE
		})
		_ = s.sendAttackStop(target.GUID, true)
		s.attackTarget = 0
		s.onCreatureKilled(ctx, target)
		s.debug("target slain", "account", s.accountName, "guid", target.GUID)
	} else {
		newHealth := target.Health - damage
		if cdb != nil {
			_, _ = cdb.ExecContext(ctx, "UPDATE creature SET curhealth = ? WHERE guid = ?", newHealth, uint32(target.GUID&0xFFFFFF))
		}
		s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{
			unitFieldHealth: newHealth,
		})
		s.server.triggerCreatureAggro(ctx, target.GUID, s.playerGUID)
	}
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
	var curHealth sql.NullInt64
	lowGUID := uint32(guid & 0x00FFFFFF)
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT c.guid, c.id, c.map, c.position_x, c.position_y, c.position_z, c.curhealth FROM creature AS c WHERE c.guid = ?", lowGUID).Scan(&low, &entry, &mapID, &target.X, &target.Y, &target.Z, &curHealth); err != nil {
		return target, err
	}
	target.GUID = creatureWorldGUID(uint32(low), uint32(entry))
	target.Map = uint32(mapID)
	if curHealth.Valid && curHealth.Int64 > 0 {
		target.Health = uint32(curHealth.Int64)
	} else {
		var tplHealth sql.NullInt64
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(maxlevel*30, 0), 100) FROM creature_template WHERE entry = ?", entry).Scan(&tplHealth)
		if tplHealth.Valid && tplHealth.Int64 > 0 {
			target.Health = uint32(tplHealth.Int64)
		} else {
			target.Health = 100
		}
	}
	return target, nil
}

func distance3D(x1, y1, z1, x2, y2, z2 float32) float64 {
	dx := float64(x1 - x2)
	dy := float64(y1 - y2)
	dz := float64(z1 - z2)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func buildAttackerStateUpdate(attacker, victim uint64, damage, overkill uint32) []byte {
	packet := protocol.NewBuffer(64)
	packet.WriteU32(0x00000002) // HitInfo: HITINFO_NORMALSWING2
	packet.WritePackedGUID(attacker)
	packet.WritePackedGUID(victim)
	packet.WriteU32(damage)          // Full damage
	packet.WriteU32(overkill)        // Overkill
	packet.WriteU8(1)                // Sub damage count
	packet.WriteU32(1)                // Damage school: Physical (1)
	packet.WriteF32(float32(damage)) // float sub damage
	packet.WriteU32(damage)          // uint32 sub damage
	packet.WriteU8(0)                // TargetState: VICTIMSTATE_HIT
	packet.WriteU32(0)               // Unknown
	packet.WriteU32(0)               // Melee spell ID
	return packet.Bytes()
}
