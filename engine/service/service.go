package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/auth"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/world"
)

type Kind string

const (
	Auth  Kind = "authserver"
	World Kind = "worldserver"
)

type Service struct {
	Kind    Kind
	Address string
	Store   *database.Store
	Handler func(context.Context, net.Conn)
}

func RunCombined(ctx context.Context, c config.Config, logger *slog.Logger) error {
	stores, err := database.OpenSet(ctx, c)
	if err != nil {
		return err
	}
	defer stores.Close()
	authServer := auth.NewServer(stores.Auth, logger, c.RealmID, c)
	worldServer := world.NewServer(stores, logger, c.RealmID, c)
	if err := worldServer.Initialize(ctx); err != nil {
		return err
	}
	authService := &Service{Kind: Auth, Address: fmt.Sprintf(":%d", c.RealmServerPort), Store: stores.Auth, Handler: authServer.Handle}
	worldService := &Service{Kind: World, Address: fmt.Sprintf(":%d", c.WorldServerPort), Store: stores.World, Handler: worldServer.Handle}
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errs := make(chan error, 2)
	go func() { errs <- authService.Run(ctx, logger) }()
	go func() { errs <- worldService.Run(ctx, logger) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errs:
		if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}

func RunSingle(ctx context.Context, c config.Config, kind Kind, logger *slog.Logger) error {
	stores, err := database.OpenSet(ctx, c)
	if err != nil {
		return err
	}
	defer stores.Close()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if kind == Auth {
		server := auth.NewServer(stores.Auth, logger, c.RealmID, c)
		return (&Service{Kind: kind, Address: fmt.Sprintf(":%d", c.RealmServerPort), Store: stores.Auth, Handler: server.Handle}).Run(ctx, logger)
	}
	server := world.NewServer(stores, logger, c.RealmID, c)
	if err := server.Initialize(ctx); err != nil {
		return err
	}
	return (&Service{Kind: kind, Address: fmt.Sprintf(":%d", c.WorldServerPort), Store: stores.World, Handler: server.Handle}).Run(ctx, logger)
}

func authHandler(store *database.Store, logger *slog.Logger, realmID uint32) func(context.Context, net.Conn) {
	return auth.NewServer(store, logger, realmID).Handle
}

func (s *Service) Run(ctx context.Context, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", s.Address)
	if err != nil {
		return fmt.Errorf("%s listen %s: %w", s.Kind, s.Address, err)
	}
	defer listener.Close()
	logger.Info("service listening", "service", s.Kind, "address", listener.Addr().String(), "database", s.Store.Name, "backend", s.Store.Backend)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if s.Handler != nil {
			go s.Handler(ctx, conn)
		} else {
			go func() { defer conn.Close(); _, _ = io.Copy(io.Discard, conn) }()
		}
	}
}
