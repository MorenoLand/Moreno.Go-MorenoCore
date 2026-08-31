package main

import (
	"os"

	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/cli"
	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/service"
)

func main() { kind := service.World; os.Exit(cli.Run(&kind)) }
