package wotlk

import "testing"

func TestPlayableRaceAndMountSpeedRules(t *testing.T) {
	if !IsPlayableRace(Race{Alliance: 0}) || IsPlayableRace(Race{Alliance: 2}) || IsPlayableRace(Race{Flags: 1}) {
		t.Fatal("playable race flags are incorrect")
	}
	spell := Spell{Effects: [3]SpellEffect{{BasePoints: 309, Aura: MountedFlightSpeedAura}, {BasePoints: 279, Aura: MountedFlightSpeedAura}, {BasePoints: 149}}}
	if !HasMountedFlightSpeed(spell, 310) || !HasMountedFlightSpeed(spell, 280) || HasMountedFlightSpeed(spell, 150) {
		t.Fatal("mount speed detection is incorrect")
	}
}
