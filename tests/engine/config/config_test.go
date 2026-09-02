//go:build ignore

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

func TestMaxOverspeedPingsParsing(t *testing.T) {
	c := Default()
	if c.MaxOverSpeedPings != 2 {
		t.Fatalf("default MaxOverSpeedPings=%d", c.MaxOverSpeedPings)
	}
	if err := c.Set("MaxOverspeedPings", "5"); err != nil {
		t.Fatal(err)
	}
	if c.MaxOverSpeedPings != 5 {
		t.Fatalf("MaxOverSpeedPings=%d", c.MaxOverSpeedPings)
	}
	// Reference: World.cpp clamps non-zero values below 2 to 2.
	if err := c.Set("MaxOverspeedPings", "1"); err != nil {
		t.Fatal(err)
	}
	if c.MaxOverSpeedPings != 2 {
		t.Fatalf("clamped MaxOverSpeedPings=%d", c.MaxOverSpeedPings)
	}
	// Reference: 0 disables the over-speed check entirely.
	if err := c.Set("MaxOverspeedPings", "0"); err != nil {
		t.Fatal(err)
	}
	if c.MaxOverSpeedPings != 0 {
		t.Fatalf("disabled MaxOverSpeedPings=%d", c.MaxOverSpeedPings)
	}
	t.Setenv("MORENOCORE_MAX_OVERSPEED_PINGS", "3")
	c = Default()
	c.ApplyEnv()
	if c.MaxOverSpeedPings != 3 {
		t.Fatalf("env MaxOverSpeedPings=%d", c.MaxOverSpeedPings)
	}
}

func TestStartPlayerMoneyDefaultAndUnrecognizedKeys(t *testing.T) {
	c := Default()
	if c.StartPlayerMoney != 10000 {
		t.Fatalf("expected StartPlayerMoney 10000, got %d", c.StartPlayerMoney)
	}
	if c.StartPlayerLevel != 1 {
		t.Fatalf("expected StartPlayerLevel 1, got %d", c.StartPlayerLevel)
	}
	if err := c.Set("SomeCustomUnknownKey", "hello"); err != nil {
		t.Fatal(err)
	}
	if len(c.UnrecognizedKeys) != 1 || c.UnrecognizedKeys[0] != "SomeCustomUnknownKey" {
		t.Fatalf("expected unrecognized key recorded, got %+v", c.UnrecognizedKeys)
	}
}


