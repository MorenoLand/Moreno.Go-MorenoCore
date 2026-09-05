package world

import (
	"context"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	mirrorTimerFatigue uint32 = 0
	mirrorTimerBreath  uint32 = 1
	mirrorTimerFire    uint32 = 2

	maxBreathTimerMs int32 = 180000 // 3 minutes in 3.3.5 (Player.cpp:826)
)

// buildStartMirrorTimer builds SMSG_START_MIRROR_TIMER (0x1D9).
// Mirrors TrinityCore WorldPackets::Misc::StartMirrorTimer::Write.
func buildStartMirrorTimer(timerType uint32, value, maxValue uint32, scale int32, paused bool, spellID uint32) []byte {
	buf := protocol.NewBuffer(21)
	buf.WriteU32(timerType)
	buf.WriteU32(value)
	buf.WriteU32(maxValue)
	buf.WriteI32(scale)
	if paused {
		buf.WriteU8(1)
	} else {
		buf.WriteU8(0)
	}
	buf.WriteU32(spellID)
	return buf.Bytes()
}

// buildPauseMirrorTimer builds SMSG_PAUSE_MIRROR_TIMER (0x1DA).
func buildPauseMirrorTimer(timerType uint32, paused bool) []byte {
	buf := protocol.NewBuffer(5)
	buf.WriteU32(timerType)
	if paused {
		buf.WriteU8(1)
	} else {
		buf.WriteU8(0)
	}
	return buf.Bytes()
}

// buildStopMirrorTimer builds SMSG_STOP_MIRROR_TIMER (0x1DB).
func buildStopMirrorTimer(timerType uint32) []byte {
	buf := protocol.NewBuffer(4)
	buf.WriteU32(timerType)
	return buf.Bytes()
}

func (s *session) sendMirrorTimer(timerType uint32, value, maxValue uint32, scale int32) {
	pkt := buildStartMirrorTimer(timerType, value, maxValue, scale, false, 0)
	_ = s.write(uint16(protocol.OpcodeSMSG_START_MIRROR_TIMER), pkt, true)
}

func (s *session) stopMirrorTimer(timerType uint32) {
	pkt := buildStopMirrorTimer(timerType)
	_ = s.write(uint16(protocol.OpcodeSMSG_STOP_MIRROR_TIMER), pkt, true)
}

// handleEnterSwimming initializes breath countdown if water breathing is not active.
func (s *session) handleEnterSwimming() {
	if s.player == nil || s.player.Health == 0 {
		return
	}
	// Water breathing check (SPELL_AURA_WATER_BREATHING = 82, SPELL_AURA_MOD_WATER_BREATHING = 155)
	if s.hasAuraType(82) || s.hasAuraType(155) {
		s.breathTimer = -1
		return
	}
	if (s.player.ExtraFlags&playerExtraGMOn != 0) || (s.player.PlayerFlags&playerFlagGM != 0) || s.security > 0 {
		s.breathTimer = -1
		return
	}
	if s.breathTimer <= 0 {
		s.breathTimer = maxBreathTimerMs
	}
	s.lastBreathTick = time.Now()
	s.sendMirrorTimer(mirrorTimerBreath, uint32(s.breathTimer), uint32(maxBreathTimerMs), -1)
}

// handleExitSwimming switches breath mirror timer into regen mode (+10 scale) until full.
func (s *session) handleExitSwimming() {
	if s.breathTimer != -1 && s.breathTimer < maxBreathTimerMs {
		s.lastBreathTick = time.Now()
		s.sendMirrorTimer(mirrorTimerBreath, uint32(s.breathTimer), uint32(maxBreathTimerMs), 10)
	} else if s.breathTimer >= maxBreathTimerMs {
		s.breathTimer = -1
		s.stopMirrorTimer(mirrorTimerBreath)
	}
}

// handleDrowningTick updates breath countdown or deals drowning environmental damage.
// Mirrors TrinityCore Player::HandleDrowning (Player.cpp:860-900).
func (s *session) handleDrowningTick(ctx context.Context, now time.Time) {
	if s.player == nil || s.player.Health == 0 {
		if s.breathTimer != -1 {
			s.breathTimer = -1
			s.stopMirrorTimer(mirrorTimerBreath)
		}
		return
	}

	if s.isSwimming {
		// Water breathing check
		if s.hasAuraType(82) || s.hasAuraType(155) || (s.player.ExtraFlags&playerExtraGMOn != 0) || (s.player.PlayerFlags&playerFlagGM != 0) || s.security > 0 {
			if s.breathTimer != -1 {
				s.breathTimer = -1
				s.stopMirrorTimer(mirrorTimerBreath)
			}
			return
		}

		if s.breathTimer == -1 {
			s.breathTimer = maxBreathTimerMs
			s.lastBreathTick = now
			s.sendMirrorTimer(mirrorTimerBreath, uint32(s.breathTimer), uint32(maxBreathTimerMs), -1)
			return
		}

		if s.lastBreathTick.IsZero() {
			s.lastBreathTick = now
			return
		}

		diffMs := int32(now.Sub(s.lastBreathTick).Milliseconds())
		if diffMs <= 0 {
			return
		}
		s.lastBreathTick = now
		s.breathTimer -= diffMs

		if s.breathTimer <= 0 {
			// Drowning tick: deal damage every second (Player.cpp:880-884)
			s.breathTimer = 0
			damage := s.player.MaxHealth / 5
			if damage == 0 {
				damage = 1
			}
			s.environmentalDamage(ctx, damageDrowning, damage)
		}
	} else if s.breathTimer != -1 {
		// Regenerate breath at 10x speed when surfaced (Player.cpp:894)
		if s.lastBreathTick.IsZero() {
			s.lastBreathTick = now
			return
		}
		diffMs := int32(now.Sub(s.lastBreathTick).Milliseconds())
		if diffMs <= 0 {
			return
		}
		s.lastBreathTick = now
		s.breathTimer += 10 * diffMs
		if s.breathTimer >= maxBreathTimerMs {
			s.breathTimer = -1
			s.stopMirrorTimer(mirrorTimerBreath)
		}
	}
}

// updatePlayerUnderwater iterates over active sessions and processes drowning/breath ticks.
func (s *Server) updatePlayerUnderwater(ctx context.Context, now time.Time) {
	s.sessionsMu.RLock()
	var sessions []*session
	for sess := range s.sessions {
		if sess.playerLoaded && sess.player != nil && (sess.isSwimming || (sess.breathTimer != -1 && sess.breathTimer < maxBreathTimerMs)) {
			sessions = append(sessions, sess)
		}
	}
	s.sessionsMu.RUnlock()

	for _, sess := range sessions {
		sess.handleDrowningTick(ctx, now)
	}
}
