package main

import (
	"os"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/cli"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/service"
)

func main() { kind := service.World; os.Exit(cli.Run(&kind)) }
