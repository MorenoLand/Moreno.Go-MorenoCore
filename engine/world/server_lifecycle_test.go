package world

import (
	"testing"
	"time"
)

func TestServerStopCancelsTrackedTimersAndIsIdempotent(t *testing.T) {
	fired := make(chan struct{}, 1)
	server := &Server{activeCreatureAuras: map[uint64]map[uint32]*activeAura{1: {2: {TickTimer: time.AfterFunc(25*time.Millisecond, func() { fired <- struct{}{} })}}}}
	server.Stop()
	server.Stop()
	time.Sleep(60 * time.Millisecond)
	select {
	case <-fired:
		t.Fatal("tracked aura timer fired after server stop")
	default:
	}
}
