package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
)

func TestSoloLFGEnablesOnLoginAndAllowsPartialGroups(t *testing.T) {
	lfg := NewLFGManager(true)
	if lfg.Solo() || !lfg.RequiresFullGroup(1, 5) {
		t.Fatal("solo LFG should start disabled and require a full group before login")
	}
	lfg.OnLogin()
	if !lfg.Solo() || lfg.RequiresFullGroup(1, 5) {
		t.Fatal("solo LFG did not enable partial groups")
	}
	if lfg.ToggleSolo() {
		t.Fatal("toggle should disable solo LFG")
	}
}

func Test310FlyerStateRecalculatesFromLearnedSpells(t *testing.T) {
	state := NewMountState(0, []LearnedMountSpell{{ID: 100, MountedFlightSpeed: 280}, {ID: 200, MountedFlightSpeed: 310}})
	if !state.Has310Flyer(true, 0) || state.PreferredFlightSpeed(true) != 310 {
		t.Fatal("310 flyer was not detected")
	}
	if state.UnlearnSpell(200) || state.PreferredFlightSpeed(true) != 280 {
		t.Fatal("310 flyer flag was not cleared")
	}
	state.LearnSpell(LearnedMountSpell{ID: 300, MountedFlightSpeed: 310})
	if !state.Has310Flyer(false, 0) || state.ExtraFlags()&PlayerExtraHas310Flyer == 0 {
		t.Fatal("learning a 310 flyer did not set the extra flag")
	}
}

func TestNPCBotAssignmentLimitsAndSpellOrdering(t *testing.T) {
	cfg := config.Default().NPCBots
	cfg.MaxBots = 1
	mgr := &NPCBotManager{config: cfg, bots: map[uint32]NpcBotData{70001: {Entry: 70001}, 70002: {Entry: 70002, Owner: 8}}, extras: map[uint32]NpcBotExtras{70001: {Entry: 70001, Class: 1}, 70002: {Entry: 70002, Class: 2}}}
	if !mgr.CanAssign(7, 70001) || mgr.CanAssign(8, 70001) {
		t.Fatal("assignment limits are incorrect")
	}
	if got := formatSpellList([]uint32{900, 100, 900, 300}); got != "100 300 900 900 " {
		t.Fatalf("spell ordering=%q", got)
	}
}
