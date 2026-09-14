package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/version"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestBinPlayerLoginHooksReturn(t *testing.T) {
	if _, err := os.Stat("../../bin/lua_scripts"); err != nil {
		t.Skipf("bin/lua_scripts unavailable: %v", err)
	}
	charDB, err := sql.Open("sqlite", "file:../../bin/characters.db?mode=ro")
	if err != nil {
		t.Skipf("characters database unavailable: %v", err)
	}
	defer charDB.Close()
	worldDB, err := sql.Open("sqlite", "file:../../bin/world.db?mode=ro")
	if err != nil {
		t.Skipf("world database unavailable: %v", err)
	}
	defer worldDB.Close()
	runtime := scripting.NewRuntime(scripting.Config{Enabled: true, ScriptPath: "../../bin/lua_scripts", CoreName: version.Product, CoreVersion: version.String(), CharacterDB: charDB, WorldDatabase: worldDB, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := runtime.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := &Server{CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: charDB}, WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: worldDB}, Features: &Features{Scripts: runtime}, sessions: make(map[*session]struct{})}
	state := &session{server: server, accountName: "DENVEOUS", accountID: 15, security: 3, playerGUID: 106, playerLoaded: true, player: &playerState{GUID: 106, Name: "Druz", Race: 8, Class: 1, Level: 9, Health: 100, MaxHealth: 100}, auras: make(map[uint32]struct{}), auraSlots: make(map[uint32]uint8), activeAuras: make(map[uint32]*activeAura)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := state.triggerPlayerEventValues(ctx, scripting.PlayerEventLogin, state.luaPlayer())
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("player login hooks did not return")
	}
}

func TestBinCharactersCanSpeakWithLoadedScripts(t *testing.T) {
	if _, err := os.Stat("../../bin/lua_scripts"); err != nil {
		t.Skipf("bin/lua_scripts unavailable: %v", err)
	}
	charDB, err := sql.Open("sqlite", "file:../../bin/characters.db?mode=ro")
	if err != nil {
		t.Skipf("characters database unavailable: %v", err)
	}
	defer charDB.Close()
	worldDB, err := sql.Open("sqlite", "file:../../bin/world.db?mode=ro")
	if err != nil {
		t.Skipf("world database unavailable: %v", err)
	}
	defer worldDB.Close()
	runtime := scripting.NewRuntime(scripting.Config{Enabled: true, ScriptPath: "../../bin/lua_scripts", CoreName: version.Product, CoreVersion: version.String(), CharacterDB: charDB, WorldDatabase: worldDB, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := runtime.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := &Server{CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: charDB}, WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: worldDB}, Features: &Features{Scripts: runtime}, Config: config.Default(), sessions: make(map[*session]struct{})}
	for _, guid := range []uint64{26, 106, 125, 127} {
		serverConn, clientConn := net.Pipe()
		state := &session{server: server, conn: serverConn, authed: true, accountName: "DENVEOUS", accountID: 15, playerGUID: guid, playerLoaded: true, auras: make(map[uint32]struct{}), auraSlots: make(map[uint32]uint8), activeAuras: make(map[uint32]*activeAura)}
		player, err := state.loadPlayerState(context.Background(), guid)
		if err != nil {
			serverConn.Close()
			clientConn.Close()
			t.Fatalf("guid %d load failed: %v", guid, err)
		}
		state.player = &player
		server.sessions[state] = struct{}{}
		payload := protocol.NewBuffer(32)
		payload.WriteU32(chatSay)
		language := uint32(1)
		if state.playerAlliance() {
			language = 7
		}
		payload.WriteU32(language)
		payload.WriteCString("bin chat")
		done := make(chan bool, 1)
		go func() { done <- state.handleMessageChat(context.Background(), payload.Bytes()) }()
		if opcode, _, err := readServerFrame(clientConn, nil); err != nil || (opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) && opcode != uint16(protocol.OpcodeSMSG_GM_MESSAGECHAT)) {
			serverConn.Close()
			clientConn.Close()
			t.Fatalf("guid %d chat opcode=%x err=%v", guid, opcode, err)
		}
		if !<-done {
			t.Fatalf("guid %d chat handler closed the session", guid)
		}
		delete(server.sessions, state)
		serverConn.Close()
		clientConn.Close()
	}
}
