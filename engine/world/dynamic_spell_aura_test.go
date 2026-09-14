package world

import (
	"context"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
)

func TestDynamicSpellAuraAppliesToCreatureEnteringArea(t *testing.T) {
	server := &Server{Data: wotlk.NewStore("../../data/dbc"), sessions: make(map[*session]struct{}), creatureMotion: make(map[uint64]*creatureMotion), dynamicSpellObjects: make(map[uint64]*dynamicSpellObjectState), creatureAuras: make(map[uint64]map[uint32]struct{}), activeCreatureAuras: make(map[uint64]map[uint32]*activeAura)}
	caster := &session{server: server, authed: true, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Map: 0, Race: 1, Class: 8, Level: 21, X: 0, Y: 0, Z: 0}}
	server.sessions[caster] = struct{}{}
	creatureGUID := creatureWorldGUID(200, 68)
	server.creatureMotion[creatureGUID] = &creatureMotion{GUID: creatureGUID, Entry: 68, Map: 0, X: 3, Y: 0, Z: 0, Faction: 14, Health: 100, MaxHealth: 100, Level: 21}
	spell, found, err := server.Data.Spell(2120)
	if err != nil || !found {
		t.Fatalf("spell found=%v err=%v", found, err)
	}
	server.dynamicSpellObjects[dynamicSpellGUID(1)] = &dynamicSpellObjectState{GUID: dynamicSpellGUID(1), CasterGUID: 1, SpellID: uint64(spell.ID), Map: 0, X: 0, Y: 0, Z: 0, Radius: 8, SpellData: spell, AuraEffect: spell.Effects[1], AuraDurationMs: 8000, AuraPeriodMs: 2000, AuraAmount: 12, AuraSchoolMask: uint8(spell.SchoolMask), NextAuraTick: time.Now().Add(-time.Second)}
	server.updateDynamicSpellAuras(context.Background(), time.Now())
	server.auraMu.Lock()
	_, auraFound := server.activeCreatureAuras[creatureGUID][spell.ID]
	server.auraMu.Unlock()
	if !auraFound {
		t.Fatal("dynamic area aura was not applied to creature inside ground effect")
	}
}
