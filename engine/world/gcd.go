package world

import (
	"math"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
)

// isOffGCDSpell identifies spells that ignore and bypass the Global Cooldown (GCD).
// Mirrors TrinityCore SpellHistory::HasGlobalCooldown (SpellHistory.cpp:60-90).
func (s *session) isOffGCDSpell(spell wotlk.Spell) bool {
	// Auto-repeat spells never trigger or obey GCD
	if spell.ID == 75 || spell.ID == 5019 || (spell.AttributesEx1&0x20 != 0) {
		return true
	}

	// Primary off-GCD abilities in WoW 3.3.5:
	switch spell.ID {
	// Interrupts
	case 1766,  // Kick (Rogue)
		6552,   // Pummel (Warrior)
		2139,   // Counterspell (Mage)
		72,     // Shield Bash (Warrior)
		57994,  // Wind Shear (Shaman)
		47528,  // Mind Freeze (Death Knight)
		19244:  // Spell Lock (Warlock Felhunter)
		return true

	// Defensive cooldowns
	case 22812, // Barkskin (Druid)
		61336,  // Survival Instincts (Druid)
		19263,  // Deterrence (Hunter)
		48707:  // Anti-Magic Shell (Death Knight)
		return true

	// Instant offensive/utility cooldowns
	case 11958, // Cold Snap (Mage)
		12043,  // Presence of Mind (Mage)
		16188,  // Nature's Swiftness (Shaman)
		17116,  // Nature's Swiftness (Druid)
		20572,  // Blood Fury (Orc racial)
		26297,  // Berserking (Troll racial)
		36554:  // Shadowstep (Rogue)
		return true
	}

	// In DBC: if StartRecoveryCategory is explicitly 0, spell is off-GCD
	if spell.StartRecoveryCategory == 0 && spell.StartRecoveryTime == 0 && spell.ID != 0 {
		return true
	}

	return false
}

// getGCDDuration resolves the duration of the Global Cooldown in milliseconds for this player.
// Base GCD is 1500ms for casters (reduced by spell haste to 1000ms minimum floor)
// and 1000ms flat for Rogues.
// Mirrors TrinityCore SpellHistory::TriggerGlobalCooldown (SpellHistory.cpp:95-120).
func (s *session) getGCDDuration(spell wotlk.Spell) int64 {
	base := int64(1500)
	if spell.StartRecoveryTime > 0 {
		base = int64(spell.StartRecoveryTime)
	}

	// Rogues have a 1.0s base GCD
	if s.player != nil && s.player.Class == 4 {
		return 1000
	}

	// Casters: 1500ms reduced by spell haste down to 1000ms minimum floor
	hastePct := s.getSpellHastePct()
	if hastePct > 0 {
		hasted := float64(base) / (1.0 + hastePct/100.0)
		base = int64(math.Round(hasted))
		if base < 1000 {
			base = 1000
		}
	}
	return base
}

// triggerGlobalCooldown initiates the Global Cooldown on the session.
func (s *session) triggerGlobalCooldown(spell wotlk.Spell) {
	if s.isOffGCDSpell(spell) {
		return
	}
	gcdDuration := s.getGCDDuration(spell)
	s.castMu.Lock()
	s.gcdEnd = time.Now().UnixMilli() + gcdDuration
	s.castMu.Unlock()
}

// isGCDActive checks whether the Global Cooldown is currently active and blocks the given spell.
func (s *session) isGCDActive(spell wotlk.Spell) bool {
	if s.isOffGCDSpell(spell) {
		return false
	}
	s.castMu.Lock()
	defer s.castMu.Unlock()
	return s.gcdEnd > time.Now().UnixMilli()
}
