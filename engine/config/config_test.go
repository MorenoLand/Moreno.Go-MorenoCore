package config

import "testing"

func TestFeatureConfigurationDefaultsAndOverrides(t *testing.T) {
	c := Default()
	if !c.SoloLFGEnable || !c.SoloLFGAnnounce || !c.NPCBots.Enable || c.CharactersPerAccount != 50 || c.CharactersPerRealm != 10 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if err := c.Set("CharacterCreating.Disabled.RaceMask", "1024"); err != nil {
		t.Fatal(err)
	}
	if err := c.Set("NpcBot.Mult.Damage.Physical", "2.5"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MORENOCORE_SOLO_LFG_ENABLE", "false")
	t.Setenv("MORENOCORE_NPCBOT_MAX_BOTS", "50")
	c.ApplyEnv()
	if c.CharacterCreatingDisabledRaceMask != 1024 || c.NPCBots.DamagePhysicalMultiplier != 2.5 || c.SoloLFGEnable || c.NPCBots.MaxBots != 50 {
		t.Fatalf("overrides were not applied: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
