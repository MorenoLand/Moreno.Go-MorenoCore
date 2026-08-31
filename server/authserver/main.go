package main

import (
	"os"

	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/cli"
	"github.com/MorenoLand/Moreno.Go-MorenoCore5/engine/service"
)

func main() { kind := service.Auth; os.Exit(cli.Run(&kind)) }
