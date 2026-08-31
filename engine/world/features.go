package world

import (
	"context"
	"log/slog"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
)

type Features struct {
	Config  config.Config
	LFG     *LFGManager
	NPCBots *NPCBotManager
	Scripts *scripting.Runtime
}

func NewFeatures(c config.Config, stores *database.Set, logger *slog.Logger) *Features {
	return &Features{Config: c, LFG: NewLFGManager(c.SoloLFGEnable), NPCBots: NewNPCBotManager(stores.Characters, stores.World, c.NPCBots), Scripts: scripting.NewRuntime(scripting.Config{Enabled: c.LuaEnabled, ScriptPath: c.LuaScriptPath, CoreExpansion: c.Expansion, AuthDatabase: stores.Auth.DB, CharacterDB: stores.Characters.DB, WorldDatabase: stores.World.DB, Logger: logger})}
}

func (f *Features) Initialize(ctx context.Context) error {
	if err := f.NPCBots.Initialize(ctx); err != nil {
		return err
	}
	return f.Scripts.Load(ctx)
}

func (f *Features) OnPlayerLogin() {
	f.LFG.OnLogin()
}
