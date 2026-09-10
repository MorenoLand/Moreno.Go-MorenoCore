package service

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
)

func TestServiceRunCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &Service{
		Kind:    Auth,
		Address: "127.0.0.1:0",
		Store:   &database.Store{Name: "auth", Backend: "sqlite"},
		Handler: func(ctx context.Context, c net.Conn) {
			_ = c.Close()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Run(ctx, logger)
	}()

	// Allow listener to start
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error on cancel, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for service to stop")
	}
}

func TestServiceRunWithNilStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &Service{
		Kind:    World,
		Address: "127.0.0.1:0",
		Store:   nil,
		Handler: func(ctx context.Context, c net.Conn) {
			_ = c.Close()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Run(ctx, logger)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error on cancel, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for service to stop")
	}
}
