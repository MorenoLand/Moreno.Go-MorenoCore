package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/MorenoLand/Moreno.TrinityGo/engine/config"
	"github.com/MorenoLand/Moreno.TrinityGo/engine/service"
	"github.com/MorenoLand/Moreno.TrinityGo/engine/version"
)

func Run(kind *service.Kind) int {
	fs := flag.NewFlagSet(string(*kind), flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "configuration file")
	backend := fs.String("backend", "", "database backend: sqlite, mysql, or mariadb")
	dataDir := fs.String("data-dir", "", "runtime data directory")
	showVersion := fs.Bool("version", false, "show version")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Printf("%s %s (%s)\n", version.Product, version.String(), version.Revision())
		return 0
	}
	c, err := config.Load(*configPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	c.ApplyEnv()
	if *backend != "" {
		c.Backend = *backend
	}
	if *dataDir != "" {
		c.DataDir = *dataDir
	}
	if err := c.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := service.RunSingle(context.Background(), c, *kind, logger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func RunCombined() int {
	fs := flag.NewFlagSet("trinitygo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "configuration file")
	backend := fs.String("backend", "", "database backend: sqlite, mysql, or mariadb")
	dataDir := fs.String("data-dir", "", "runtime data directory")
	showVersion := fs.Bool("version", false, "show version")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Printf("%s %s (%s)\n", version.Product, version.String(), version.Revision())
		return 0
	}
	c, err := config.Load(*configPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	c.ApplyEnv()
	if *backend != "" {
		c.Backend = *backend
	}
	if *dataDir != "" {
		c.DataDir = *dataDir
	}
	if err := c.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := service.RunCombined(context.Background(), c, logger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
