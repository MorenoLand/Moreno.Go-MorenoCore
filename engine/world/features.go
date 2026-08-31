package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

type Features struct {
	Config  config.Config
	LFG     *LFGManager
	NPCBots *NPCBotManager
}

func NewFeatures(c config.Config, stores *database.Set) *Features {
	return &Features{Config: c, LFG: NewLFGManager(c.SoloLFGEnable), NPCBots: NewNPCBotManager(stores.Characters, stores.World, c.NPCBots)}
}

func (f *Features) Initialize(ctx context.Context) error {
	return f.NPCBots.Initialize(ctx)
}

func (f *Features) OnPlayerLogin() {
	f.LFG.OnLogin()
}
