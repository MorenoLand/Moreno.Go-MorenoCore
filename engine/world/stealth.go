package world

import (
	"math"
)

// isStealthed checks if the session's player currently has an active stealth aura.
// Mirrors TrinityCore HasAuraType(SPELL_AURA_MOD_STEALTH) (SpellAuraDefines.h:96).
func (s *session) isStealthed() bool {
	if s.player == nil {
		return false
	}
	return s.hasAuraType(16) // SPELL_AURA_MOD_STEALTH
}

// getStealthValue returns the total stealth rating of the player.
// Mirrors TrinityCore m_stealth (SPELL_AURA_MOD_STEALTH + SPELL_AURA_MOD_STEALTH_LEVEL).
func (s *session) getStealthValue() int32 {
	if s.player == nil {
		return 0
	}
	return s.getTotalAuraModifier(16) + s.getTotalAuraModifier(154)
}

// getStealthDetectValue returns the total stealth detection bonus rating of the player.
// Mirrors TrinityCore m_stealthDetect (SPELL_AURA_MOD_STEALTH_DETECT).
func (s *session) getStealthDetectValue() int32 {
	if s.player == nil {
		return 0
	}
	return s.getTotalAuraModifier(17)
}

// hasInArc checks if the target position is within the observer's frontal arc.
// Mirrors TrinityCore Position::HasInArc (Position.cpp:120).
// An arc of math.Pi corresponds to a 180-degree frontal hemisphere.
func hasInArc(observerOri, observerX, observerY, targetX, targetY float32, arc float64) bool {
	dx := float64(targetX - observerX)
	dy := float64(targetY - observerY)
	angleToTarget := math.Atan2(dy, dx)
	diff := angleToTarget - float64(observerOri)
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}
	halfArc := arc / 2.0
	return diff >= -halfArc && diff <= halfArc
}

// canDetectStealthOf determines if the observer player can detect the target stealthed player.
// Mirrors TrinityCore WorldObject::CanDetectStealthOf (Object.cpp:1719-1790).
func (s *session) canDetectStealthOf(target *session) bool {
	if target == nil || target.player == nil {
		return true
	}
	if !target.isStealthed() {
		return true
	}
	if s.player == nil {
		return false
	}
	// GM or God mode sees all stealthed units
	if (s.player.ExtraFlags&playerExtraGMOn != 0) || (s.player.PlayerFlags&playerFlagGM != 0) || s.security > 0 {
		return true
	}
	// Target is in the same party/group: party members can always see each other
	if s.groupID != 0 && s.groupID == target.groupID {
		return true
	}
	// Detect Stealth aura (SPELL_AURA_TRACK_STEALTHED = 151)
	if s.hasAuraType(151) {
		return true
	}

	dist := float32(distance3D(s.player.X, s.player.Y, s.player.Z, target.player.X, target.player.Y, target.player.Z))
	combatReach := float32(1.5)
	if s.player.CombatReach > 0 {
		combatReach = s.player.CombatReach
	}
	// Contact distance always detects stealth (both front and back)
	if dist < combatReach {
		return true
	}

	// Back check: units cannot detect stealth behind them
	if !hasInArc(s.player.Orientation, s.player.X, s.player.Y, target.player.X, target.player.Y, math.Pi) {
		return false
	}

	obsLevel := s.player.Level
	if obsLevel == 0 {
		obsLevel = 1
	}

	detectionValue := int32(30) + int32(obsLevel-1)*5
	detectionValue += s.getStealthDetectValue()
	detectionValue -= target.getStealthValue()

	visibilityRange := float32(detectionValue)*0.3 + combatReach
	if visibilityRange > 30.0 { // MAX_PLAYER_STEALTH_DETECT_RANGE
		visibilityRange = 30.0
	}
	if visibilityRange <= 0 {
		return false
	}

	return dist <= visibilityRange
}

// canCreatureDetectStealthOfPlayer determines if a creature can detect a stealthed player.
// Mirrors TrinityCore WorldObject::CanDetectStealthOf for Creature observers.
func canCreatureDetectStealthOfPlayer(motion *creatureMotion, targetSess *session, dist float32) bool {
	if targetSess == nil || targetSess.player == nil {
		return true
	}
	if !targetSess.isStealthed() {
		return true
	}

	combatReach := float32(1.5)
	if motion != nil && motion.CombatReach > 0 {
		combatReach = motion.CombatReach
	}
	if dist < combatReach {
		return true
	}

	if motion != nil && !hasInArc(motion.Orientation, motion.X, motion.Y, targetSess.player.X, targetSess.player.Y, math.Pi) {
		return false
	}

	cLevel := uint32(1)
	if motion != nil && motion.Level > 0 {
		cLevel = motion.Level
	}

	detectionValue := int32(30) + int32(cLevel-1)*5
	detectionValue -= targetSess.getStealthValue()

	visibilityRange := float32(detectionValue)*0.3 + combatReach
	if visibilityRange <= 0 {
		return false
	}

	return dist <= visibilityRange
}
