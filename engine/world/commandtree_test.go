package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	"net"
)

func commandTestSession(t *testing.T) (*session, net.Conn, net.Conn) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	store := &database.Store{Name: "world", Backend: database.BackendSQLite}
	server := &Server{WorldStore: store, CharactersStore: store, Config: config.Default(), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Level: 1}}
	// Drain responses so ambiguity messages don't block the pipe writer.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := clientConn.Read(buf); err != nil {
				return
			}
		}
	}()
	return state, serverConn, clientConn
}

func TestCommandPartialMatching(t *testing.T) {
	state, serverConn, clientConn := commandTestSession(t)
	defer serverConn.Close()
	defer clientConn.Close()

	// '.mod spee 10' resolves to '.modify speed 10' and updates the speed.
	if !state.dispatchCommand(context.Background(), []string{"mod", "spee", "10"}) {
		t.Fatal("partial command dispatch failed")
	}
	if state.player == nil || state.player.Level == 0 {
		t.Fatal("player state disturbed")
	}

	// Exact match still works.
	if !state.dispatchCommand(context.Background(), []string{"gm", "off"}) {
		t.Fatal("exact command dispatch failed")
	}

	// Ambiguous token reports candidates instead of dispatching.
	if !state.dispatchCommand(context.Background(), []string{"gm", "o"}) {
		t.Fatal("ambiguous command should still be handled (with an error message)")
	}

	// Unknown top-level token returns false so chat falls through.
	if state.dispatchCommand(context.Background(), []string{"definitelynotacommand"}) {
		t.Fatal("unknown command must not be handled")
	}
}

func TestCommandTokenCanonicalization(t *testing.T) {
	state, serverConn, clientConn := commandTestSession(t)
	defer serverConn.Close()
	defer clientConn.Close()

	root := state.buildCommandTree()
	node, rewritten, consumed, ambiguous, _ := root.resolve([]string{"mod", "spee", "10"})
	if ambiguous != nil {
		t.Fatalf("unexpected ambiguity: %v", ambiguous)
	}
	if node.name != "modify" {
		t.Fatalf("expected node modify, got %s", node.name)
	}
	if consumed != 2 || len(rewritten) != 2 || rewritten[0] != "modify" || rewritten[1] != "speed" {
		t.Fatalf("consumed=%d rewritten=%v", consumed, rewritten)
	}

	_, rewritten, _, _, _ = root.resolve([]string{"mod", "spee"})
	if len(rewritten) != 2 || rewritten[0] != "modify" || rewritten[1] != "speed" {
		t.Fatalf("expected [modify speed], got %v", rewritten)
	}

	// Nested structural + trailing data preserved.
	_, rewritten, _, _, _ = root.resolve([]string{"gm", "c"})
	if len(rewritten) != 2 || rewritten[1] != "chat" {
		t.Fatalf("expected [gm chat], got %v", rewritten)
	}
	_ = protocol.UpdateValues
}

