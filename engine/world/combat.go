package world

import (
	"context"
	"database/sql"
	"math"
	"math/rand/v2"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const meleeAttackRange = 5.0

type combatTarget struct {
	GUID       uint64
	Map        uint32
	X          float32
	Y          float32
	Z          float32
	Health     uint32
	UnitFlags  uint32
	FlagsExtra uint32
}

func (s *session) getCombatTarget(ctx context.Context, guid uint64) (combatTarget, bool) {
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	// Check if target is an online player (e.g. duel opponent or PvP target)
	if s.server != nil {
		if playerSess := s.server.findSessionByGUID(guid); playerSess != nil && playerSess.player != nil {
			return combatTarget{
				GUID:       guid,
				Map:        playerSess.player.Map,
				X:          playerSess.player.X,
				Y:          playerSess.player.Y,
				Z:          playerSess.player.Z,
				Health:     playerSess.player.Health,
				UnitFlags:  playerSess.player.UnitFlags,
				FlagsExtra: 0,
			}, true
		}
	}
	s.server.motionMu.Lock()
	if s.server.creatureMotion != nil {
		motion := s.server.creatureMotion[guid]
		if motion == nil {
			low := uint32(guid & 0x00FFFFFF)
			entry := uint32((guid >> 24) & 0x00FFFFFF)
			stdKey := creatureWorldGUID(low, entry)
			motion = s.server.creatureMotion[stdKey]
		}
		if motion != nil {
			target := combatTarget{
				GUID:       guid,
				Map:        motion.Map,
				X:          motion.X,
				Y:          motion.Y,
				Z:          motion.Z,
				Health:     motion.Health,
				UnitFlags:  motion.UnitFlags,
				FlagsExtra: motion.FlagsExtra,
			}
			s.server.motionMu.Unlock()
			return target, true
		}
	}
	s.server.motionMu.Unlock()

	target, err := s.loadCombatTarget(ctx, guid)
	if err != nil {
		return combatTarget{}, false
	}
	return target, true
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
	target, ok := s.getCombatTarget(ctx, victim)
	if !ok {
		s.debug("attack target not found", "account", s.accountName, "victim", victim)
		return true
	}
	if target.Health == 0 {
		s.attackTarget = 0
		return s.write(uint16(protocol.OpcodeSMSG_ATTACK_SWING_DEAD_TARGET), nil, true) == nil
	}
	if creatureCombatDisabled(target.UnitFlags, target.FlagsExtra) {
		s.attackTarget = 0
		return s.sendAttackStop(victim, false) == nil
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
	if s.player.MaxDamage > s.player.MinDamage && s.player.MinDamage > 0 {
		attSpeed := float64(s.player.AttackTime) / 1000.0
		if attSpeed <= 0 {
			attSpeed = 2.0
		}
		apBonus := (float64(s.player.AttackPower) * attSpeed) / 14.0
		baseDmg := float64(s.player.MinDamage) + rand.Float64()*float64(s.player.MaxDamage-s.player.MinDamage)
		damage = uint32(baseDmg + apBonus)
	}
	if damage < 1 {
		damage = 1
	}
	overkill := uint32(0)
	if damage >= target.Health {
		overkill = damage - target.Health
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE), buildAttackerStateUpdate(s.playerGUID, target.GUID, damage, overkill), true)

	// If target is an online player (e.g. duel opponent or PvP)
	if s.server != nil {
		if playerSess := s.server.findSessionByGUID(target.GUID); playerSess != nil && playerSess.player != nil {
			if damage >= playerSess.player.Health {
				if s.duelPartner == target.GUID && s.player.DuelTeam != 0 {
					// Duel defeat: loser drops to 1 HP and kneels (TC: Player::DuelComplete)
					playerSess.player.Health = 1
					playerSess.sendPlayerUpdate()
					s.endDuel(true, s.playerGUID)
				} else {
					playerSess.player.Health = 0
					playerSess.sendPlayerUpdate()
					playerSess.killPlayer(ctx)
				}
			} else {
				playerSess.player.Health -= damage
				playerSess.sendPlayerUpdate()
			}
			_ = s.sendAttackStop(target.GUID, true)
			s.attackTarget = 0
			return
		}
	}

	low := uint32(target.GUID & 0x00FFFFFF)
	entry := uint32((target.GUID >> 24) & 0x00FFFFFF)
	stdKey := creatureWorldGUID(low, entry)

	if damage >= target.Health {
		// Target dies
		s.server.motionMu.Lock()
		motion := s.server.creatureMotion[target.GUID]
		if motion == nil {
			motion = s.server.creatureMotion[stdKey]
		}
		if motion != nil {
			motion.Health = 0
			motion.InCombat = false
			motion.TargetGUID = 0
			motion.Moving = false
		}
		s.server.motionMu.Unlock()

		s.server.stopCreatureMotion(target.Map, target.GUID, target.X, target.Y, target.Z)
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
		s.server.motionMu.Lock()
		motion := s.server.creatureMotion[target.GUID]
		if motion == nil {
			motion = s.server.creatureMotion[stdKey]
		}
		if motion != nil {
			motion.Health = newHealth
		}
		s.server.motionMu.Unlock()

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
	var low, entry, mapID, unitFlags, flagsExtra int64
	var curHealth sql.NullInt64
	lowGUID := uint32(guid & 0x00FFFFFF)
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT c.guid, c.id, c.map, c.position_x, c.position_y, c.position_z, c.curhealth FROM creature AS c WHERE c.guid = ?", lowGUID).Scan(&low, &entry, &mapID, &target.X, &target.Y, &target.Z, &curHealth); err != nil {
		return target, err
	}
	target.GUID = creatureWorldGUID(uint32(low), uint32(entry))
	target.Map = uint32(mapID)
	_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(unit_flags, 0), COALESCE(flags_extra, 0) FROM creature_template WHERE entry = ?", entry).Scan(&unitFlags, &flagsExtra)
	target.UnitFlags, target.FlagsExtra = uint32(unitFlags), uint32(flagsExtra)
	if curHealth.Valid {
		if curHealth.Int64 > 0 {
			target.Health = uint32(curHealth.Int64)
		}
	} else {
		var tplHealth sql.NullInt64
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(maxlevel*30, 0), 100) FROM creature_template WHERE entry = ?", entry).Scan(&tplHealth)
		if tplHealth.Valid && tplHealth.Int64 > 0 {
			target.Health = uint32(tplHealth.Int64)
		} else {
			target.Health = 100
		}
	}

	s.server.motionMu.Lock()
	if s.server.creatureMotion == nil {
		s.server.creatureMotion = make(map[uint64]*creatureMotion)
	}
	if motion := s.server.creatureMotion[target.GUID]; motion != nil {
		target.X, target.Y, target.Z = motion.X, motion.Y, motion.Z
		target.UnitFlags, target.FlagsExtra = motion.UnitFlags, motion.FlagsExtra
		if motion.Health > 0 {
			target.Health = motion.Health
		} else {
			motion.Health = target.Health
		}
	} else {
		motion := &creatureMotion{
			GUID:       target.GUID,
			Entry:      uint32(entry),
			Map:        target.Map,
			HomeX:      target.X,
			HomeY:      target.Y,
			HomeZ:      target.Z,
			X:          target.X,
			Y:          target.Y,
			Z:          target.Z,
			Speed:      2.5,
			RunSpeed:   7.0,
			UnitFlags:  uint32(unitFlags),
			FlagsExtra: uint32(flagsExtra),
			Health:     target.Health,
			MaxHealth:  target.Health,
			Refreshed:  time.Now(),
		}
		s.server.creatureMotion[target.GUID] = motion
		if guid != target.GUID {
			s.server.creatureMotion[guid] = motion
		}
	}
	s.server.motionMu.Unlock()
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
	packet.WriteU32(1)               // Damage school: Physical (1)
	packet.WriteF32(float32(damage)) // float sub damage
	packet.WriteU32(damage)          // uint32 sub damage
	packet.WriteU8(1)                // TargetState: VICTIMSTATE_HIT
	packet.WriteU32(0)               // Unknown
	packet.WriteU32(0)               // Melee spell ID
	return packet.Bytes()
}

// handleDuelAccepted processes CMSG_DUEL_ACCEPTED (0x16C).
// Reference: WorldSession::HandleDuelAcceptedOpcode (DuelHandler.cpp:25).
func (s *session) handleDuelAccepted(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	buf := protocol.NewBuffer(4)
	buf.WriteU32(3000) // 3000ms duel countdown
	_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_COUNTDOWN), buf.Bytes(), true)
	var partner *session
	if s.duelPartner != 0 && s.server != nil {
		partner = s.server.findSessionByGUID(s.duelPartner)
		if partner != nil {
			_ = partner.write(uint16(protocol.OpcodeSMSG_DUEL_COUNTDOWN), buf.Bytes(), true)
		}
	}

	// After 3-second countdown, set PLAYER_DUEL_TEAM to start the duel! (TC: Player::UpdateDuelFlag)
	time.AfterFunc(3*time.Second, func() {
		if s.duelPartner == 0 || s.player == nil {
			return
		}
		s.player.DuelTeam = 1
		s.sendPlayerUpdate()
		if partner != nil && partner.player != nil {
			partner.player.DuelTeam = 2
			partner.sendPlayerUpdate()
		}
	})
	return true
}

// handleDuelCancelled processes CMSG_DUEL_CANCELLED (0x16D).
// Reference: WorldSession::HandleDuelCancelledOpcode (DuelHandler.cpp:53).
func (s *session) handleDuelCancelled(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	s.endDuel(false, 0)
	return true
}

// endDuel cleans up duel flags, clears arbiter/team, and emits SMSG_DUEL_COMPLETE.
// Reference: Player::DuelComplete (Player.cpp:7435-7450).
func (s *session) endDuel(won bool, winnerGUID uint64) {
	result := uint8(0) // 0 = interrupted / fled / cancelled
	if won {
		result = 1 // 1 = won
	}
	buf := protocol.NewBuffer(1)
	buf.WriteU8(result)

	s.player.DuelArbiter = 0
	s.player.DuelTeam = 0
	s.sendPlayerUpdate()
	_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_COMPLETE), buf.Bytes(), true)

	if s.duelPartner != 0 && s.server != nil {
		if partner := s.server.findSessionByGUID(s.duelPartner); partner != nil && partner.player != nil {
			partner.player.DuelArbiter = 0
			partner.player.DuelTeam = 0
			partner.sendPlayerUpdate()
			partner.duelPartner = 0
			_ = partner.write(uint16(protocol.OpcodeSMSG_DUEL_COMPLETE), buf.Bytes(), true)
		}
		s.duelPartner = 0
	}
}
