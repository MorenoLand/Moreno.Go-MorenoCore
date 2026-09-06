package world

import (
	"math"
)

// Absorption aura types from TrinityCore SharedDefines.h:600-750
const (
	SpellAuraSchoolAbsorb uint32 = 69  // SPELL_AURA_SCHOOL_ABSORB (Power Word: Shield, Ice Barrier, Sacred Shield)
	SpellAuraManaShield   uint32 = 72  // SPELL_AURA_MANA_SHIELD (Mana Shield)
	SpellAuraMagicAbsorb  uint32 = 256 // SPELL_AURA_MAGIC_ABSORB (Anti-Magic Shell)
)

// applyAbsorptionShields applies active absorption shields (Power Word: Shield, Ice Barrier, etc.)
// to mitigate incoming damage of the given school mask.
// Returns absorbed damage and remaining unabsorbed damage.
// Reference: TrinityCore Unit::CalcAbsorbResist (Unit.cpp:2000-2080).
func (s *session) applyAbsorptionShields(damage uint32, schoolMask uint8) (absorbed uint32, remainingDamage uint32) {
	if s == nil || s.player == nil || damage == 0 {
		return 0, damage
	}

	remainingDamage = damage
	var exhaustedSpells []uint32

	s.castMu.Lock()
	if s.activeAuras != nil {
		for spellID, aura := range s.activeAuras {
			if aura == nil || aura.Stopped || aura.Amount == 0 {
				continue
			}

			// 1. School Absorb (SPELL_AURA_SCHOOL_ABSORB = 69)
			if aura.AuraType == SpellAuraSchoolAbsorb {
				// SchoolMask == 0 absorbs all schools, otherwise bitmask match
				if aura.SchoolMask == 0 || (aura.SchoolMask&uint32(schoolMask)) != 0 {
					absorbThis := aura.Amount
					if absorbThis > remainingDamage {
						absorbThis = remainingDamage
					}
					aura.Amount -= absorbThis
					remainingDamage -= absorbThis
					absorbed += absorbThis
					if aura.Amount == 0 {
						exhaustedSpells = append(exhaustedSpells, spellID)
					}
					if remainingDamage == 0 {
						break
					}
				}
				continue
			}

			// 2. Mana Shield (SPELL_AURA_MANA_SHIELD = 72)
			// In WotLK, Mana Shield absorbs all damage and drains 1.5 mana per point absorbed
			if aura.AuraType == SpellAuraManaShield && s.player.Powers[0] > 0 {
				currMana := s.player.Powers[0]
				neededMana := uint32(math.Ceil(float64(remainingDamage) * 1.5))
				absorbPossible := remainingDamage
				if currMana < neededMana {
					absorbPossible = uint32(float64(currMana) / 1.5)
				}
				if absorbPossible > aura.Amount {
					absorbPossible = aura.Amount
				}
				if absorbPossible > 0 {
					manaDrain := uint32(math.Ceil(float64(absorbPossible) * 1.5))
					if manaDrain > s.player.Powers[0] {
						s.player.Powers[0] = 0
					} else {
						s.player.Powers[0] -= manaDrain
					}
					aura.Amount -= absorbPossible
					remainingDamage -= absorbPossible
					absorbed += absorbPossible
					if aura.Amount == 0 {
						exhaustedSpells = append(exhaustedSpells, spellID)
					}
				}
				if remainingDamage == 0 {
					break
				}
				continue
			}

			// 3. Magic Absorb (SPELL_AURA_MAGIC_ABSORB = 256)
			// Absorbs non-physical magical damage (Anti-Magic Shell)
			if aura.AuraType == SpellAuraMagicAbsorb && schoolMask&1 == 0 {
				if aura.SchoolMask == 0 || (aura.SchoolMask&uint32(schoolMask)) != 0 {
					absorbThis := aura.Amount
					if absorbThis > remainingDamage {
						absorbThis = remainingDamage
					}
					aura.Amount -= absorbThis
					remainingDamage -= absorbThis
					absorbed += absorbThis
					if aura.Amount == 0 {
						exhaustedSpells = append(exhaustedSpells, spellID)
					}
					if remainingDamage == 0 {
						break
					}
				}
				continue
			}
		}
	}
	s.castMu.Unlock()

	// Remove exhausted shields outside the lock
	for _, id := range exhaustedSpells {
		s.removeAura(id)
	}

	return absorbed, remainingDamage
}

// applyCreatureAbsorptionShields applies active absorption shields on a creature.
func (s *Server) applyCreatureAbsorptionShields(creatureGUID uint64, damage uint32, schoolMask uint8) (absorbed uint32, remainingDamage uint32) {
	if s == nil || creatureGUID == 0 || damage == 0 {
		return 0, damage
	}

	remainingDamage = damage
	var exhaustedSpells []uint32

	s.auraMu.Lock()
	if s.activeCreatureAuras != nil {
		if auras, ok := s.activeCreatureAuras[creatureGUID]; ok && auras != nil {
			for spellID, aura := range auras {
				if aura == nil || aura.Stopped || aura.Amount == 0 {
					continue
				}
				if aura.AuraType == SpellAuraSchoolAbsorb || (aura.AuraType == SpellAuraMagicAbsorb && schoolMask&1 == 0) {
					if aura.SchoolMask == 0 || (aura.SchoolMask&uint32(schoolMask)) != 0 {
						absorbThis := aura.Amount
						if absorbThis > remainingDamage {
							absorbThis = remainingDamage
						}
						aura.Amount -= absorbThis
						remainingDamage -= absorbThis
						absorbed += absorbThis
						if aura.Amount == 0 {
							exhaustedSpells = append(exhaustedSpells, spellID)
						}
						if remainingDamage == 0 {
							break
						}
					}
				}
			}
		}
	}
	s.auraMu.Unlock()

	for _, id := range exhaustedSpells {
		s.removeCreatureAura(creatureGUID, id)
	}

	return absorbed, remainingDamage
}
