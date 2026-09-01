package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/service"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/version"
)

func Run(kind *service.Kind) int {
	fs := flag.NewFlagSet(string(*kind), flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "configuration file")
	backend := fs.String("backend", "", "database backend: sqlite, mysql, or mariadb")
	dataDir := fs.String("data-dir", "", "runtime data directory")
	showVersion := fs.Bool("version", false, "show version")
	debug := fs.Bool("debug", false, "enable basic authentication and runtime debug logs")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Printf("%s %s (%s)\n", version.Product, version.String(), version.Revision())
		return 0
	}
	selectedConfig := discoverConfig(*configPath, *kind)
	c, err := config.Load(selectedConfig)
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
	c.ResolvePaths()
	if err := c.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger := newLogger(*debug)
	if err := service.RunSingle(context.Background(), c, *kind, logger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func RunCombined() int {
	fs := flag.NewFlagSet("morenocore", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "configuration file")
	backend := fs.String("backend", "", "database backend: sqlite, mysql, or mariadb")
	dataDir := fs.String("data-dir", "", "runtime data directory")
	showVersion := fs.Bool("version", false, "show version")
	debug := fs.Bool("debug", false, "enable basic authentication and runtime debug logs")
	authOnly := fs.Bool("auth", false, "run only the authentication service")
	worldOnly := fs.Bool("world", false, "run only the world service")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Printf("%s %s (%s)\n", version.Product, version.String(), version.Revision())
		return 0
	}
	selectedConfig := discoverConfig(*configPath, "")
	c, err := config.Load(selectedConfig)
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
	c.ResolvePaths()
	if err := c.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger := newLogger(*debug)
	if *authOnly && *worldOnly {
		fmt.Fprintln(os.Stderr, "--auth and --world cannot be used together")
		return 2
	}
	if *authOnly {
		kind := service.Auth
		if err := service.RunSingle(context.Background(), c, kind, logger); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if *worldOnly {
		kind := service.World
		if err := service.RunSingle(context.Background(), c, kind, logger); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if err := service.RunCombined(context.Background(), c, logger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func newLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func discoverConfig(explicit string, kind service.Kind) string {
	if explicit != "" {
		return explicit
	}
	cwd, err := os.Getwd()
	if err != nil || strings.EqualFold(filepath.Base(filepath.Clean(cwd)), "bin") {
		return ""
	}
	candidates := []string{"bin/worldserver.conf", "worldserver.conf"}
	if kind == service.Auth {
		candidates = []string{"bin/authserver.conf", "authserver.conf"}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
