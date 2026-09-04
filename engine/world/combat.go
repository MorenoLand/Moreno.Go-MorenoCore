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
	s.lastCombatTime = time.Now()
	if s.player != nil && s.player.UnitFlags&unitFlagInCombat == 0 {
		s.player.UnitFlags |= unitFlagInCombat
		s.sendPlayerUpdate()
	}
	s.debug("attack started", "account", s.accountName, "guid", victim)
	startPayload := buildAttackStart(s.playerGUID, victim)
	_ = s.write(uint16(protocol.OpcodeSMSG_ATTACK_START), startPayload, true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_ATTACK_START), startPayload, s)
	}
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
	asuPayload := buildAttackerStateUpdate(s.playerGUID, target.GUID, damage, overkill)
	_ = s.write(uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE), asuPayload, true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE), asuPayload, s)
	}

	// If target is an online player (e.g. duel opponent or PvP)
	if s.server != nil {
		if playerSess := s.server.findSessionByGUID(target.GUID); playerSess != nil && playerSess.player != nil {
			if playerSess.player.UnitFlags&unitFlagInCombat == 0 {
				playerSess.player.UnitFlags |= unitFlagInCombat
			}
			playerSess.lastCombatTime = time.Now()
			if damage >= playerSess.player.Health {
				if s.duelPartner == target.GUID && s.player.DuelTeam != 0 {
					// Duel defeat: loser drops to 1 HP and kneels (TC: Player::DuelComplete)
					playerSess.player.Health = 1
					playerSess.sendPlayerUpdate()
					s.endDuel(true, s.playerGUID, false)
				} else {
					playerSess.player.Health = 0
					playerSess.sendPlayerUpdate()
					playerSess.killPlayer(ctx)
				}
				_ = s.sendAttackStop(target.GUID, true)
				s.attackTarget = 0
			} else {
				playerSess.player.Health -= damage
				playerSess.sendPlayerUpdate()
			}
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
	payload := buildAttackStop(s.playerGUID, victim, nowDead)
	_ = s.write(uint16(protocol.OpcodeSMSG_ATTACK_STOP), payload, true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_ATTACK_STOP), payload, s)
	}
	return nil
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

	// Initialize arbiter coordinates if not yet set
	if s.duelArbiterX == 0 && s.duelArbiterY == 0 && partner != nil && partner.player != nil {
		midX := s.player.X + (partner.player.X-s.player.X)/2
		midY := s.player.Y + (partner.player.Y-s.player.Y)/2
		midZ := s.player.Z
		s.duelArbiterX, s.duelArbiterY, s.duelArbiterZ = midX, midY, midZ
		partner.duelArbiterX, partner.duelArbiterY, partner.duelArbiterZ = midX, midY, midZ
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
	// Player surrendered in an active duel using /forfeit (TC: HandleDuelCancelledOpcode:66)
	if s.player.DuelTeam != 0 && s.duelPartner != 0 {
		s.endDuel(true, s.duelPartner, false)
		return true
	}
	s.endDuel(false, 0, false)
	return true
}

// DuelCompleteType mirrors TrinityCore's DuelCompleteType enum.
type DuelCompleteType uint8

const (
	DuelInterrupted DuelCompleteType = 0
	DuelWon         DuelCompleteType = 1
	DuelFled        DuelCompleteType = 2
)

// buildDuelWinner builds SMSG_DUEL_WINNER (0x16B).
// Reference: Player::DuelComplete (Player.cpp:7353-7358).
func buildDuelWinner(fled bool, winnerName, loserName string) []byte {
	packet := protocol.NewBuffer(1 + len(winnerName) + 1 + len(loserName) + 1)
	if fled {
		packet.WriteU8(1)
	} else {
		packet.WriteU8(0)
	}
	packet.WriteCString(winnerName)
	packet.WriteCString(loserName)
	return packet.Bytes()
}

// castVisualSpell broadcasts SMSG_SPELL_GO for instant non-combat visual spells (e.g. kneel 7267, victory cheer 52852).
func (s *session) castVisualSpell(spellID uint32) {
	if s == nil || s.player == nil {
		return
	}
	castPkt := protocol.NewBuffer(16)
	castPkt.WritePackedGUID(s.playerGUID)
	castPkt.WritePackedGUID(s.playerGUID)
	castPkt.WriteU8(1)
	castPkt.WriteU32(spellID)
	castPkt.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), castPkt.Bytes(), true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_SPELL_GO), castPkt.Bytes(), s)
	}
}

// checkDuelBounds checks distance to duel arbiter (flag).
// > 50yd sends SMSG_DUEL_OUTOFBOUNDS and starts 10s timer.
// <= 40yd sends SMSG_DUEL_INBOUNDS and resets timer.
// > 10s out of bounds ends duel as fled.
// Reference: Player::CheckDuelOutOfBounds (Player.cpp:7290-7321).
func (s *session) checkDuelBounds() {
	if s == nil || s.player == nil || s.player.DuelTeam == 0 || s.duelPartner == 0 || s.player.DuelArbiter == 0 {
		return
	}
	dist := distance3D(s.player.X, s.player.Y, s.player.Z, s.duelArbiterX, s.duelArbiterY, s.duelArbiterZ)
	now := time.Now()
	if s.duelOutOfBounds.IsZero() {
		if dist > 50.0 {
			s.duelOutOfBounds = now.Add(10 * time.Second)
			_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_OUTOFBOUNDS), []byte{}, true)
			partner := s.duelPartner
			time.AfterFunc(10*time.Second, func() {
				if s.player != nil && s.player.DuelTeam != 0 && s.duelPartner == partner && !s.duelOutOfBounds.IsZero() && time.Now().After(s.duelOutOfBounds) {
					s.checkDuelBounds()
				}
			})
		}
	} else {
		if dist <= 40.0 {
			s.duelOutOfBounds = time.Time{}
			_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_INBOUNDS), []byte{}, true)
		} else if now.After(s.duelOutOfBounds) {
			s.endDuel(false, s.duelPartner, true)
		}
	}
}

// endDuel cleans up duel flags, clears arbiter/team, and emits SMSG_DUEL_COMPLETE and SMSG_DUEL_WINNER.
// Reference: Player::DuelComplete (Player.cpp:7328-7442).
func (s *session) endDuel(won bool, winnerGUID uint64, fled bool) {
	partnerGUID := s.duelPartner
	var partner *session
	if partnerGUID != 0 && s.server != nil {
		partner = s.server.findSessionByGUID(partnerGUID)
	}

	duelEnded := won || fled
	completeResult := uint8(0)
	if duelEnded {
		completeResult = 1
	}

	buf := protocol.NewBuffer(1)
	buf.WriteU8(completeResult)
	_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_COMPLETE), buf.Bytes(), true)
	if partner != nil {
		_ = partner.write(uint16(protocol.OpcodeSMSG_DUEL_COMPLETE), buf.Bytes(), true)
	}

	if duelEnded && winnerGUID != 0 {
		var winnerSess, loserSess *session
		if s.playerGUID == winnerGUID {
			winnerSess = s
			loserSess = partner
		} else {
			winnerSess = partner
			loserSess = s
		}

		winnerName := ""
		loserName := ""
		if winnerSess != nil && winnerSess.player != nil {
			winnerName = winnerSess.player.Name
		}
		if loserSess != nil && loserSess.player != nil {
			loserName = loserSess.player.Name
		}

		winnerPkt := buildDuelWinner(fled, winnerName, loserName)
		_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_WINNER), winnerPkt, true)
		if partner != nil {
			_ = partner.write(uint16(protocol.OpcodeSMSG_DUEL_WINNER), winnerPkt, true)
		}
		if s.server != nil && s.player != nil {
			s.server.sessionsMu.RLock()
			for target := range s.server.sessions {
				if target == s || target == partner || !target.authed || !target.playerLoaded || target.player == nil {
					continue
				}
				if target.player.Map == s.player.Map {
					_ = target.write(uint16(protocol.OpcodeSMSG_DUEL_WINNER), winnerPkt, true)
				}
			}
			s.server.sessionsMu.RUnlock()
		}

		// Spells: Winner casts 52852 (Victory cheer)
		if winnerSess != nil {
			winnerSess.castVisualSpell(52852)
		}
		// Loser casts 7267 (Beg / surrender kneel) if won normally
		if !fled && loserSess != nil {
			loserSess.castVisualSpell(7267)
		}
	}

	// Stop combat on both
	_ = s.sendAttackStop(s.attackTarget, false)
	s.attackTarget = 0
	if partner != nil {
		_ = partner.sendAttackStop(partner.attackTarget, false)
		partner.attackTarget = 0
	}

	// Clean up fields on s
	if s.player != nil {
		s.player.DuelArbiter = 0
		s.player.DuelTeam = 0
		s.sendPlayerUpdate()
	}
	s.duelPartner = 0
	s.duelOutOfBounds = time.Time{}

	// Clean up fields on partner
	if partner != nil {
		if partner.player != nil {
			partner.player.DuelArbiter = 0
			partner.player.DuelTeam = 0
			partner.sendPlayerUpdate()
		}
		partner.duelPartner = 0
		partner.duelOutOfBounds = time.Time{}
	}
}
