package world

import (
	"context"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestStandStateChangeValidation(t *testing.T) {
	state := &session{server: &Server{}, playerLoaded: true, player: &playerState{Map: 0, Health: 100}}
	payload := protocol.NewBuffer(4)
	payload.WriteU32(2)
	if !state.handleStandStateChange(context.Background(), payload.Bytes()) || state.player.StandState != 2 {
		t.Fatalf("stand state=%d", state.player.StandState)
	}
	invalid := protocol.NewBuffer(4)
	invalid.WriteU32(4)
	if !state.handleStandStateChange(context.Background(), invalid.Bytes()) || state.player.StandState != 2 {
		t.Fatalf("invalid stand state changed to %d", state.player.StandState)
	}
}

func TestEmoteValidation(t *testing.T) {
	state := &session{server: &Server{}, playerLoaded: true, playerGUID: 9, player: &playerState{Map: 0, Health: 100}}
	wave := protocol.NewBuffer(4)
	wave.WriteU32(17)
	if !state.handleEmote(wave.Bytes()) {
		t.Fatal("wave emote failed")
	}
	invalid := protocol.NewBuffer(4)
	invalid.WriteU32(99)
	if !state.handleEmote(invalid.Bytes()) {
		t.Fatal("invalid emote should be ignored")
	}
}
