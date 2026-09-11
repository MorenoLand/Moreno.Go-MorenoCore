package world

import (
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocoltrace"
)

func TestSessionWriteRecordsServerPacket(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	recorder := protocoltrace.NewRecorder("test")
	sess := &session{server: &Server{TraceRecorder: recorder}, conn: serverConn}
	payload := []byte{1, 2, 3}
	done := make(chan error, 1)
	go func() { done <- sess.write(uint16(protocol.OpcodeSMSG_PONG), payload, false) }()
	if _, _, err := readServerFrame(clientConn, nil); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	trace := recorder.Snapshot()
	if len(trace.Events) != 1 {
		t.Fatalf("events=%d", len(trace.Events))
	}
	event := trace.Events[0]
	if event.Direction != protocoltrace.ServerToClient || event.Opcode != uint32(protocol.OpcodeSMSG_PONG) {
		t.Fatalf("unexpected event: %+v", event)
	}
	got, err := trace.Payload(event)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("payload=%x err=%v", got, err)
	}
}
