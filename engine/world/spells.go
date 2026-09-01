package world

import (
	"context"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	spellAttributePassive uint32 = 0x00000040
	spellCastFlagStart    uint32 = 0x00000002
	spellCastFlagGo       uint32 = 0x00000100
)

func (s *session) handleCastSpell(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.server.Data == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	castID, err := reader.ReadU8()
	if err != nil {
		return false
	}
	spellID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	clientCastFlags, err := reader.ReadU8()
	if err != nil {
		return false
	}
	target, err := protocol.ReadSpellTargetData(reader)
	if err != nil {
		s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "malformed targets", "error", err)
		return false
	}
	if clientCastFlags&0x02 != 0 {
		if _, err = reader.ReadF32(); err != nil {
			return false
		}
		if _, err = reader.ReadF32(); err != nil {
			return false
		}
	}
	spell, found, err := s.server.Data.Spell(spellID)
	if err != nil {
		s.debug("spell lookup failed", "account", s.accountName, "spell", spellID, "error", err)
		return true
	}
	learned := s.hasActiveSpell(spellID)
	if !found || spell.Attributes&spellAttributePassive != 0 || !learned {
		s.debug("spell cast ignored", "account", s.accountName, "spell", spellID, "reason", spellCastIgnoreReason(spell, found, learned))
		return true
	}
	castTime := uint32(0)
	if value, ok, castErr := s.server.Data.SpellCastTime(spell.CastingTimeIndex); castErr == nil && ok && value > 0 {
		castTime = uint32(value)
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_SPELL_START), protocol.BuildSpellStart(s.playerGUID, s.playerGUID, castID, spellID, spellCastFlagStart, castTime, target), true); err != nil {
		return false
	}
	hitTargets := []uint64{s.playerGUID}
	if target.Flags&protocol.SpellTargetFlagUnitWireMask != 0 && target.UnitGUID != 0 {
		hitTargets[0] = target.UnitGUID
	}
	castTimeStamp := uint32(time.Now().UnixMilli())
	if err := s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), protocol.BuildSpellGo(s.playerGUID, s.playerGUID, castID, spellID, spellCastFlagGo, castTimeStamp, hitTargets, nil, target), true); err != nil {
		return false
	}
	s.debug("spell cast accepted", "account", s.accountName, "spell", spellID, "cast_id", castID, "cast_time", castTime)
	return true
}

func (s *session) hasActiveSpell(spellID uint32) bool {
	for _, spell := range s.player.Spells {
		if spell.ID == spellID {
			return spell.Active && !spell.Disabled
		}
	}
	return false
}

func spellCastIgnoreReason(spell wotlk.Spell, found, learned bool) string {
	if !found {
		return "unknown spell"
	}
	if spell.Attributes&spellAttributePassive != 0 {
		return "passive spell"
	}
	if !learned {
		return "spell not learned"
	}
	return "unavailable"
}

func (s *session) handleCancelCast(payload []byte) bool {
	reader := protocol.NewReader(payload)
	if _, err := reader.ReadU8(); err != nil {
		return false
	}
	if _, err := reader.ReadU32(); err != nil {
		return false
	}
	return true
}

func (s *session) handleCancelAura(payload []byte) bool {
	reader := protocol.NewReader(payload)
	spellID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	delete(s.auras, spellID)
	return true
}
