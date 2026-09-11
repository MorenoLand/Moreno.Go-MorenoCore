package world

import (
	"net"
	"testing"
)

func TestKickDuplicateAccountSession(t *testing.T) {
	oldServer, oldClient := net.Pipe()
	defer oldClient.Close()
	defer oldServer.Close()
	server := &Server{sessions: make(map[*session]struct{})}
	old := &session{server: server, conn: oldServer, authed: true, accountID: 42, accountName: "old"}
	current := &session{server: server, accountID: 42, accountName: "new"}
	server.sessions[old] = struct{}{}
	server.sessions[current] = struct{}{}
	server.kickDuplicateAccountSessions(42, current)
	buffer := make([]byte, 1)
	if _, err := oldClient.Read(buffer); err == nil {
		t.Fatal("expected duplicate session connection to close")
	}
}
