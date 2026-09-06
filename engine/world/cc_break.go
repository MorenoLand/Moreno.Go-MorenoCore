package world

// SpellAuraInterruptFlags bitmask values from TrinityCore SharedDefines.h:925-955
const (
	auraInterruptFlagHitBySpell   uint32 = 0x00000001 // removed by any damage (direct or periodic)
	auraInterruptFlagTakeDamage   uint32 = 0x00000002 // removed by damage taken
	auraInterruptFlagCast         uint32 = 0x00000004 // removed by casting
	auraInterruptFlagMove         uint32 = 0x00000008 // removed by moving
	auraInterruptFlagTurning      uint32 = 0x00000010 // removed by turning
	auraInterruptFlagJump         uint32 = 0x00000020 // removed by jumping
	auraInterruptFlagNotMounted   uint32 = 0x00000040 // removed by dismounting
	auraInterruptFlagNotSeated    uint32 = 0x00000080 // removed by standing up
	auraInterruptFlagChangeMap    uint32 = 0x00000100 // removed by changing map
	auraInterruptFlagEnterCombat  uint32 = 0x00000400 // removed on entering combat
	auraInterruptFlagDirectDamage uint32 = 0x00001000 // removed only by direct damage
	auraInterruptFlagLanding      uint32 = 0x02000000 // removed on landing
)

// getSpellAuraInterruptFlags returns the aura interrupt bitmask from Spell.dbc with
// hardcoded fallbacks for primary crowd-control and utility spells if DBC is unavailable.
func getSpellAuraInterruptFlags(spellID uint32, dbcFlags uint32) uint32 {
	if dbcFlags != 0 {
		return dbcFlags
	}
	switch spellID {
	case 1776: // Gouge (Rogue)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 6770, 2070, 11297: // Sap (Rogue)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 2094: // Blind (Rogue)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 118, 12824, 12825, 12826, 28271, 28272, 61305, 61721, 61780: // Polymorph (Mage)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 1499, 14308, 14309, 3355: // Freezing Trap (Hunter)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 20066: // Repentance (Paladin)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 2637, 18657, 18658: // Hibernate (Druid)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 9484, 9485, 10955: // Shackle Undead (Priest)
		return auraInterruptFlagTakeDamage | auraInterruptFlagDirectDamage
	case 1784: // Stealth (Rogue)
		return auraInterruptFlagTakeDamage
	case 5215: // Prowl (Druid)
		return auraInterruptFlagTakeDamage
	case 58984: // Shadowmeld (Night Elf)
		return auraInterruptFlagTakeDamage | auraInterruptFlagMove
	case 430, 431, 432, 1133, 1135, 1137: // Food / Drink
		return auraInterruptFlagMove | auraInterruptFlagTakeDamage | auraInterruptFlagCast
	default:
		return 0
	}
}

// procDamageAuras removes CC and break-on-damage auras (e.g. Gouge, Sap, Blind, Polymorph, Stealth)
// when the player takes damage. Mirrors TrinityCore Unit::ProcDamageAndSpellFor and RemoveAurasWithInterruptFlags.
func (s *session) procDamageAuras(isDirectDamage bool) {
	flags := auraInterruptFlagTakeDamage | auraInterruptFlagHitBySpell
	if isDirectDamage {
		flags |= auraInterruptFlagDirectDamage
	}
	s.removeAurasWithInterruptFlags(flags)
}

// procCastAuras removes auras broken by casting an active spell (e.g. Food, Drink, Stealth).
func (s *session) procCastAuras() {
	s.removeAurasWithInterruptFlags(auraInterruptFlagCast)
}
