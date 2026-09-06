package world

import (
	"math"
	"math/rand"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
)

// Combat rating indices for spells matching TrinityCore SharedDefines.h:1760-1790
const (
	CombatRatingHitSpell   uint8 = 7  // CR_HIT_SPELL
	CombatRatingCritSpell  uint8 = 10 // CR_CRIT_SPELL
	CombatRatingHasteSpell uint8 = 19 // CR_HASTE_SPELL
)

// getSpellHastePct returns the spell haste percentage from gear rating.
// Mirrors TrinityCore Player::GetRatingBonusValue(CR_HASTE_SPELL) (Player.cpp:8800).
func (s *session) getSpellHastePct() float64 {
	if s == nil || s.player == nil {
		return 0
	}
	rating := float64(s.player.CombatRatings[CombatRatingHasteSpell])
	if rating <= 0 {
		return 0
	}
	lvl := float64(s.player.Level)
	if lvl <= 0 {
		lvl = 80
	}
	// At level 80: 32.789989 rating = 1.0% spell haste (from gtCombatRatings.dbc)
	ratingPerPct := 32.789989 * (lvl / 80.0)
	if ratingPerPct < 5.0 {
		ratingPerPct = 5.0
	}
	return rating / ratingPerPct
}

// getSpellHitPct returns the bonus spell hit percentage from gear rating.
// Mirrors TrinityCore Player::GetRatingBonusValue(CR_HIT_SPELL) (Player.cpp:8800).
func (s *session) getSpellHitPct() float64 {
	if s == nil || s.player == nil {
		return 0
	}
	rating := float64(s.player.CombatRatings[CombatRatingHitSpell])
	if rating <= 0 {
		return 0
	}
	lvl := float64(s.player.Level)
	if lvl <= 0 {
		lvl = 80
	}
	// At level 80: 26.231995 rating = 1.0% spell hit (from gtCombatRatings.dbc)
	ratingPerPct := 26.231995 * (lvl / 80.0)
	if ratingPerPct < 5.0 {
		ratingPerPct = 5.0
	}
	return rating / ratingPerPct
}

// calculateSpellCastTime resolves the cast time in milliseconds, modified by spell haste.
// Mirrors TrinityCore Player::CalculateCastTime (Player.cpp:8800-8830):
// castTime = baseCastTime / (1.0 + hastePct / 100.0)
func (s *session) calculateSpellCastTime(spell wotlk.Spell) uint32 {
	baseCastTime := uint32(0)
	if s.server != nil && s.server.Data != nil && spell.CastingTimeIndex > 0 {
		if value, ok, err := s.server.Data.SpellCastTime(spell.CastingTimeIndex); err == nil && ok && value > 0 {
			baseCastTime = uint32(value)
		}
	}
	if baseCastTime == 0 || s.player == nil {
		return baseCastTime
	}

	hastePct := s.getSpellHastePct()
	if hastePct <= 0 {
		return baseCastTime
	}

	hastedTime := float64(baseCastTime) / (1.0 + hastePct/100.0)
	if hastedTime < 0 {
		hastedTime = 0
	}
	return uint32(math.Round(hastedTime))
}

// calculateSpellCritChance resolves the probability [0.0, 1.0] of a spell critical strike.
// Incorporates base class crit, Intellect bonus, Spell Crit Rating, and defender Resilience.
// Mirrors TrinityCore Player::GetSpellCritChance (Player.cpp:8400-8480).
func (s *session) calculateSpellCritChance(targetGUID uint64, schoolMask uint8) float64 {
	if s == nil || s.player == nil {
		return 0.05
	}

	// 1. Base crit: 5.0%
	critPct := 5.0

	lvl := float64(s.player.Level)
	if lvl <= 0 {
		lvl = 80
	}

	// 2. Intellect contribution: ~1.0% crit per 166.6667 intellect at level 80 (gtChanceToSpellCrit.dbc)
	intStat := float64(s.player.Stats[3]) // StatIndex 3 = Intellect
	intPerPct := 166.6667 * (lvl / 80.0)
	if intPerPct > 0 {
		critPct += intStat / intPerPct
	}

	// 3. Spell Crit Rating (CR_CRIT_SPELL = 10): 45.905987 rating per 1.0% crit at level 80
	rating := float64(s.player.CombatRatings[CombatRatingCritSpell])
	if rating > 0 {
		ratingPerPct := 45.905987 * (lvl / 80.0)
		if ratingPerPct < 5.0 {
			ratingPerPct = 5.0
		}
		critPct += rating / ratingPerPct
	}

	// 4. Defender resilience reduction (in PvP)
	if targetGUID != 0 && targetGUID != s.playerGUID && s.server != nil {
		if vicSess := s.server.findSessionByGUID(targetGUID); vicSess != nil {
			critBP := int32(math.Round(critPct * 100))
			vicSess.applyResilienceToMeleeCritChance(true, CombatRatingCritTakenSpell, &critBP)
			critPct = float64(critBP) / 100.0
		}
	}

	if critPct < 0 {
		critPct = 0
	}
	if critPct > 100 {
		critPct = 100
	}

	return critPct / 100.0
}

// rollSpellCrit rolls whether the spell achieves a critical strike.
func (s *session) rollSpellCrit(targetGUID uint64, schoolMask uint8) bool {
	chance := s.calculateSpellCritChance(targetGUID, schoolMask)
	return rand.Float64() < chance
}

// getSpellCritMultiplier calculates the critical strike damage/healing multiplier for a spell,
// factoring in the base 150% multiplier, talents/auras modifying critical bonus (AuraType 182),
// and metagem modifiers (+3% crit damage).
// Mirrors TrinityCore Unit::SpellCriticalDamageBonus (Unit.cpp:1650-1700).
func (s *session) getSpellCritMultiplier(spell wotlk.Spell) float64 {
	baseBonusPct := 50.0 // 1.5x base multiplier (1.0 + 50/100)
	extraBonusPct := 0.0
	metaBonusPct := 0.0

	if s == nil || s.player == nil {
		return 1.5
	}

	s.castMu.Lock()
	for _, aura := range s.activeAuras {
		if aura == nil || aura.Stopped {
			continue
		}
		// SPELL_AURA_MOD_CRIT_DAMAGE_BONUS = 182
		if aura.AuraType == 182 {
			if aura.SchoolMask == 0 || (spell.SchoolMask != 0 && aura.SchoolMask&spell.SchoolMask != 0) {
				extraBonusPct += float64(aura.Amount)
			}
		}
		// Chaotic Skyflare Diamond / Relentless Earthsiege Diamond (3% increased critical damage)
		if aura.SpellID == 26297 || aura.SpellID == 44795 || aura.SpellID == 55341 || aura.SpellID == 28557 {
			metaBonusPct += 3.0
		}
	}
	s.castMu.Unlock()

	// Check learned talent spells:
	// Ruin (Warlock 17959): 100% extra bonus for Destruction (Fire 4 / Shadow 32) spells
	// Elemental Fury (Shaman 16089): 100% extra bonus for Fire 4 / Nature 8 / Frost 16 spells
	// Ice Shards (Mage 15058): up to 100% extra bonus for Frost 16 spells
	// Spell Power (Mage 35581): 25%/50% extra bonus
	if s.hasActiveSpell(17959) && (spell.SchoolMask&4 != 0 || spell.SchoolMask&32 != 0) { // Ruin
		extraBonusPct += 50.0
	} else if s.hasActiveSpell(16089) && (spell.SchoolMask&4 != 0 || spell.SchoolMask&8 != 0 || spell.SchoolMask&16 != 0) { // Elemental Fury
		extraBonusPct += 50.0
	} else if s.hasActiveSpell(15058) && (spell.SchoolMask&16 != 0) { // Ice Shards Rank 3
		extraBonusPct += 50.0
	} else if s.hasActiveSpell(35581) { // Spell Power Rank 2
		extraBonusPct += 25.0
	}

	multiplier := 1.0 + (baseBonusPct+extraBonusPct)/100.0
	if metaBonusPct > 0 {
		multiplier *= (1.0 + metaBonusPct/100.0)
	}
	return multiplier
}
