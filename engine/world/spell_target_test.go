package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestGroundSpellDBCRecords(t *testing.T) {
	store := wotlk.NewStore("../../data/dbc")
	for _, test := range []struct {
		spellID uint32
		effect0 uint32
		target0 uint32
		effect1 uint32
		aura1   uint32
	}{
		{spellID: 122, effect0: 2, target0: 22, effect1: 6, aura1: 26},
		{spellID: 5740, effect0: 27, target0: 28, effect1: 6, aura1: 23},
		{spellID: 2120, effect0: 2, target0: 16, effect1: 27, aura1: 3},
		{spellID: 11113, effect0: 2, target0: 22, effect1: 6, aura1: 33},
		{spellID: 6143, effect0: 6, target0: 1, effect1: 6, aura1: 74},
	} {
		spell, found, err := store.Spell(test.spellID)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatalf("spell %d not found", test.spellID)
		}
		if spell.Effects[0].Effect != test.effect0 || spell.Effects[0].ImplicitTargetA != test.target0 || spell.Effects[1].Effect != test.effect1 || spell.Effects[1].Aura != test.aura1 {
			t.Fatalf("spell=%d effects=%+v", test.spellID, spell.Effects)
		}
	}
}

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

func TestSpellAreaEnemyTargetsIncludeHostilePlayers(t *testing.T) {
	server := &Server{Data: wotlk.NewStore("../../data/dbc"), sessions: make(map[*session]struct{}), creatureMotion: make(map[uint64]*creatureMotion)}
	caster := &session{server: server, authed: true, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Map: 0, Race: 1, Class: 8, Level: 21, X: 0, Y: 0, Z: 0}}
	hostile := &session{server: server, authed: true, playerLoaded: true, playerGUID: 2, player: &playerState{GUID: 2, Map: 0, Race: 2, Class: 1, Level: 21, X: 3, Y: 0, Z: 0, Health: 100, MaxHealth: 100}}
	friendly := &session{server: server, authed: true, playerLoaded: true, playerGUID: 3, player: &playerState{GUID: 3, Map: 0, Race: 3, Class: 1, Level: 21, X: 3, Y: 1, Z: 0, Health: 100, MaxHealth: 100}}
	server.sessions[caster] = struct{}{}
	server.sessions[hostile] = struct{}{}
	server.sessions[friendly] = struct{}{}
	spell := wotlk.Spell{ID: 122, SchoolMask: 16, Effects: [3]wotlk.SpellEffect{{Effect: 2, ImplicitTargetA: 22, RadiusIndex: 13}}}
	targets := caster.spellAreaEnemyTargets(context.Background(), spell, protocol.SpellTargetData{})
	if len(targets) != 1 || targets[0] != hostile.playerGUID {
		t.Fatalf("area player targets=%v, want hostile player %d only", targets, hostile.playerGUID)
	}
}
