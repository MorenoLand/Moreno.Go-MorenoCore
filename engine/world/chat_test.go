package world

import (
	"context"
	"io"
	"log/slog"
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
