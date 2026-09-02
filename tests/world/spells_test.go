//go:build ignore

package world

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestHandleCancelMountAura(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1, MountDisplayID: 123}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelMountAura(nil) {
			t.Error("handleCancelMountAura returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	if player.MountDisplayID != 0 {
		t.Fatalf("expected MountDisplayID 0, got %d", player.MountDisplayID)
	}
}

func TestHandleCancelGrowthAura(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
		scale:        2.5,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelGrowthAura(nil) {
			t.Error("handleCancelGrowthAura returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	if s.scale != 1.0 {
		t.Fatalf("expected scale 1.0, got %f", s.scale)
	}
}

func TestHandleCancelAutoRepeatSpell(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 42}
	s := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   42,
		player:       player,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelAutoRepeatSpell(nil) {
			t.Error("handleCancelAutoRepeatSpell returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_CANCEL_AUTO_REPEAT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	guid, err := r.ReadPackedGUID()
	if err != nil || guid != 42 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
}

func TestHandleCancelTempEnchantment(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(16) // slot 16

	done := make(chan struct{})
	go func() {
		if !s.handleCancelTempEnchantment(context.Background(), payload.Bytes()) {
			t.Error("handleCancelTempEnchantment returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
}

func TestHandleCorpseMapPositionQuery(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	s := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(0)

	done := make(chan struct{})
	go func() {
		if !s.handleCorpseMapPositionQuery(payload.Bytes()) {
			t.Error("handleCorpseMapPositionQuery returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	for i := 0; i < 4; i++ {
		val, err := r.ReadF32()
		if err != nil || val != 0.0 {
			t.Fatalf("coord[%d]=%f err=%v", i, val, err)
		}
	}
}

