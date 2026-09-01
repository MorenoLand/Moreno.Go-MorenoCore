package world

import (
	"context"
	"fmt"
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
	nowUnix := time.Now().Unix()
	for _, cd := range s.player.Cooldowns {
		if cd.Spell == spellID && cd.End > nowUnix {
			_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 47), true) // SPELL_FAILED_NOT_READY = 47
			s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "on cooldown")
			return true
		}
	}
	cost := spell.ManaCost
	pType := spell.PowerType
	if cost > 0 && pType < 7 && s.player.Powers[pType] < cost {
		_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 85), true) // SPELL_FAILED_NO_POWER = 85
		s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "not enough power", "power", s.player.Powers[pType], "cost", cost)
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
	if pType < 7 && cost > 0 {
		s.player.Powers[pType] -= cost
		s.sendPlayerUpdate()
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			col := fmt.Sprintf("power%d", pType+1)
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, fmt.Sprintf("UPDATE characters SET %s = ? WHERE guid = ?", col), s.player.Powers[pType], s.playerGUID)
		}
	}
	if spell.RecoveryTime > 0 {
		cooldownEnd := nowUnix + int64((spell.RecoveryTime+999)/1000)
		s.player.Cooldowns = append(s.player.Cooldowns, spellCooldown{Spell: spellID, End: cooldownEnd})
		_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_COOLDOWN), buildSpellCooldown(s.playerGUID, spellID, uint32(spell.RecoveryTime)), true)
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "INSERT OR REPLACE INTO character_spell_cooldown (guid, spell, item, time, categoryId, categoryEnd) VALUES (?, ?, 0, ?, 0, 0)", s.playerGUID, spellID, cooldownEnd)
		}
	}
	s.debug("spell cast accepted", "account", s.accountName, "spell", spellID, "cast_id", castID, "cast_time", castTime, "cost", cost)
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

func buildCastFailed(castID uint8, spellID uint32, result uint8) []byte {
	buf := protocol.NewBuffer(6)
	buf.WriteU8(castID)
	buf.WriteU32(spellID)
	buf.WriteU8(result)
	return buf.Bytes()
}

func buildSpellCooldown(playerGUID uint64, spellID uint32, cooldownDurationMs uint32) []byte {
	buf := protocol.NewBuffer(8 + 1 + 4 + 4)
	buf.WriteU64(playerGUID)
	buf.WriteU8(0) // flags = 0
	buf.WriteU32(spellID)
	buf.WriteU32(cooldownDurationMs)
	return buf.Bytes()
}
