package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
)

func TestAreaEnemyTargetTypesMatchReferenceCategories(t *testing.T) {
	for _, target := range []uint32{2, 15, 16, 24, 28, 54, 104} {
		if !isAreaEnemyTargetType(target) {
			t.Fatalf("target %d was not classified as an area enemy target", target)
		}
	}
	for _, target := range []uint32{1, 6, 18, 20, 21, 30, 56, 59} {
		if isAreaEnemyTargetType(target) {
			t.Fatalf("target %d was incorrectly classified as an area enemy target", target)
		}
	}
}

func TestAreaEnemySpellRecognizesPersistentEnemyAura(t *testing.T) {
	spell := wotlk.Spell{ID: 1000, Effects: [3]wotlk.SpellEffect{{Effect: 129, ImplicitTargetA: 28}}}
	if !isHarmfulSpell(spell) || !isAreaEnemySpell(spell) {
		t.Fatal("persistent enemy area aura was not classified as harmful area damage")
	}
}
