package main

import (
	"os"

	"github.com/MorenoLand/Moreno.TrinityGo/engine/cli"
	"github.com/MorenoLand/Moreno.TrinityGo/engine/service"
)

func main() { kind := service.World; os.Exit(cli.Run(&kind)) }
