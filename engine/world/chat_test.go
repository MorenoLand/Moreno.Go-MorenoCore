package world

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestLuaChatHookCanCancelMessage(t *testing.T) {
	runtime := scripting.NewRuntime(scripting.Config{Enabled: true})
	if err := runtime.LoadString(`RegisterPlayerEvent(18, function(event, player, msg) if msg == "blocked" then return false end end)`); err != nil {
		t.Fatal(err)
	}
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Features: &Features{Scripts: runtime}, sessions: make(map[*session]struct{})}
	state := &session{server: server, accountName: "TEST", playerGUID: 99, playerLoaded: true, player: &playerState{GUID: 99, Name: "Tester", Map: 0}, auras: make(map[uint32]struct{})}
	for _, message := range []struct {
		text      string
		cancelled bool
	}{
		{text: "blocked", cancelled: true},
		{text: "allowed", cancelled: false},
	} {
		payload := protocol.NewBuffer(32)
		payload.WriteU32(chatSay)
		payload.WriteU32(1)
		payload.WriteCString(message.text)
		if !state.handleMessageChat(context.Background(), payload.Bytes()) {
			t.Fatalf("message %q closed the session", message.text)
		}
	}
}

func TestBroadcastSayUsesSenderReceiverGUID(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0}}
	server.sessions[state] = struct{}{}
	done := make(chan struct{})
	go func() {
		server.broadcastChat(state, nil, chatSay, 1, "hello", "")
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	if value, err := reader.ReadU8(); err != nil || value != chatSay {
		t.Fatalf("type=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	<-done
}

func TestBroadcastGMChatIncludesChatTag(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, gmChat: true, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0, PlayerFlags: playerFlagGM}}
	server.sessions[state] = struct{}{}
	done := make(chan struct{})
	go func() {
		server.broadcastChat(state, nil, chatSay, 1, "hello", "")
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "hello" {
		t.Fatalf("message=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 4 {
		t.Fatalf("chat tag=%d err=%v", value, err)
	}
	<-done
}
