package auth

import (
	"io"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocoltrace"
)

func TestTraceConnRecordsAuthMessagePair(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	recorder := protocoltrace.NewRecorder("auth-test")
	conn := &traceConn{Conn: serverConn, recorder: recorder}
	conn.begin(logonChallenge)
	go func() { _, _ = clientConn.Write([]byte{1, 2, 3}) }()
	data := make([]byte, 3)
	if _, err := io.ReadFull(conn, data); err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = conn.Write([]byte{logonChallenge, wowSuccess, 0}) }()
	response := make([]byte, 3)
	if _, err := io.ReadFull(clientConn, response); err != nil {
		t.Fatal(err)
	}
	conn.end()
	events := recorder.Snapshot().Events
	if len(events) != 2 || events[0].Direction != protocoltrace.ClientToServer || events[1].Direction != protocoltrace.ServerToClient {
		t.Fatalf("events=%+v", events)
	}
	if events[0].Opcode != uint32(logonChallenge) || events[1].Opcode != uint32(logonChallenge) {
		t.Fatalf("opcodes=%d,%d", events[0].Opcode, events[1].Opcode)
	}
}
