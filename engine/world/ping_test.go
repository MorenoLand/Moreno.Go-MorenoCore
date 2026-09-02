package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func newPingTestServer(t *testing.T, maxOverSpeedPings uint32, grantSkipPermission bool) (*Server, *session, net.Conn) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE rbac_account_permissions (accountId INTEGER NOT NULL, permissionId INTEGER NOT NULL, granted INTEGER NOT NULL, realmId INTEGER NOT NULL)",
		"CREATE TABLE rbac_default_permissions (secId INTEGER NOT NULL, permissionId INTEGER NOT NULL, realmId INTEGER NOT NULL)",
		"CREATE TABLE rbac_linked_permissions (id INTEGER NOT NULL, linkedId INTEGER NOT NULL)",
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT NOT NULL, session_key_auth BLOB, last_ip TEXT, locked INTEGER, lock_country TEXT, os TEXT, online INTEGER NOT NULL DEFAULT 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if grantSkipPermission {
		if _, err := db.Exec("INSERT INTO rbac_account_permissions (accountId, permissionId, granted, realmId) VALUES (7, 23, 1, 1)"); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	stores := &database.Set{Auth: store, Characters: store, World: store}
	server := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	server.Config.MaxOverSpeedPings = maxOverSpeedPings
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	state := &session{server: server, conn: serverConn, authed: true, accountID: 7, accountName: "test"}
	return server, state, clientConn
}

func pingPayload(ping, latency uint32) []byte {
	buffer := protocol.NewBuffer(8)
	buffer.WriteU32(ping)
	buffer.WriteU32(latency)
	return buffer.Bytes()
}

// acceptPingWithPong runs handlePing in a goroutine and consumes the SMSG_PONG
// reply so the synchronous net.Pipe write cannot deadlock.
func acceptPingWithPong(t *testing.T, state *session, clientConn net.Conn, payload []byte) bool {
	t.Helper()
	result := make(chan bool, 1)
	go func() { result <- state.handlePing(context.Background(), payload) }()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != opcodePong {
		t.Fatalf("expected pong, got opcode=%x", opcode)
	}
	return <-result
}

func TestPingPongEchoStoresLatency(t *testing.T) {
	_, state, clientConn := newPingTestServer(t, 2, false)
	if !acceptPingWithPong(t, state, clientConn, pingPayload(123, 456)) {
		t.Fatal("ping was rejected")
	}
	if state.latency.Load() != 456 {
		t.Fatalf("latency=%d", state.latency.Load())
	}
	if state.lastPing.IsZero() {
		t.Fatal("last ping not recorded")
	}
	if state.overSpeedPings != 0 {
		t.Fatalf("first ping incremented over-speed counter to %d", state.overSpeedPings)
	}
}

func TestPingOverspeedCounterIncrementsAndKicks(t *testing.T) {
	_, state, clientConn := newPingTestServer(t, 2, false)
	state.lastPing = time.Now().Add(-1 * time.Second)
	if !acceptPingWithPong(t, state, clientConn, pingPayload(1, 100)) {
		t.Fatal("second ping under the limit was rejected")
	}
	if state.overSpeedPings != 1 {
		t.Fatalf("over-speed counter=%d", state.overSpeedPings)
	}
	if state.latency.Load() != 100 {
		t.Fatalf("latency=%d", state.latency.Load())
	}
	// Reference: with MaxOverspeedPings = 2 the connection is kicked once the
	// counter strictly exceeds the limit (counter > maxAllowed).
	state.overSpeedPings = 2
	state.lastPing = time.Now()
	if state.handlePing(context.Background(), pingPayload(2, 150)) {
		t.Fatal("ping over the over-speed limit should kick the session")
	}
	if state.latency.Load() != 100 {
		t.Fatalf("kicked ping must not update latency, got %d", state.latency.Load())
	}
}

func TestPingOverspeedCounterResetsAfterWindow(t *testing.T) {
	_, state, clientConn := newPingTestServer(t, 2, false)
	state.lastPing = time.Now().Add(-(overspeedPingWindow + 3*time.Second))
	state.overSpeedPings = 5
	if !acceptPingWithPong(t, state, clientConn, pingPayload(3, 200)) {
		t.Fatal("ping after the window should be accepted")
	}
	if state.overSpeedPings != 0 {
		t.Fatalf("over-speed counter=%d", state.overSpeedPings)
	}
	if state.latency.Load() != 200 {
		t.Fatalf("latency=%d", state.latency.Load())
	}
}

func TestPingOverspeedZeroConfigDisablesKick(t *testing.T) {
	_, state, clientConn := newPingTestServer(t, 0, false)
	state.lastPing = time.Now()
	state.overSpeedPings = 1000
	if !acceptPingWithPong(t, state, clientConn, pingPayload(4, 250)) {
		t.Fatal("MaxOverspeedPings = 0 must disable the over-speed check")
	}
}

func TestPingOverspeedRBACSkipPermission(t *testing.T) {
	_, state, clientConn := newPingTestServer(t, 2, true)
	state.lastPing = time.Now()
	state.overSpeedPings = 2
	if !acceptPingWithPong(t, state, clientConn, pingPayload(5, 300)) {
		t.Fatal("session with RBAC_PERM_SKIP_CHECK_OVERSPEED_PING must not be kicked")
	}
	if state.latency.Load() != 300 {
		t.Fatalf("latency=%d", state.latency.Load())
	}
}

func TestPingMalformedPayloadRejected(t *testing.T) {
	_, state, _ := newPingTestServer(t, 2, false)
	if state.handlePing(context.Background(), pingPayload(1, 1)[:4]) {
		t.Fatal("short ping payload should be rejected")
	}
	if state.handlePing(context.Background(), pingPayload(1, 1)[:7]) {
		t.Fatal("truncated ping payload should be rejected")
	}
}

func TestUnauthenticatedPingClosesConnection(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT NOT NULL, session_key_auth BLOB, last_ip TEXT, locked INTEGER, lock_country TEXT, os TEXT, online INTEGER NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	stores := &database.Set{Auth: store, Characters: store, World: store}
	server := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.Handle(context.Background(), serverConn)
	challengeHeader := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, challengeHeader); err != nil {
		t.Fatal(err)
	}
	challengePayload := make([]byte, 40)
	if _, err := io.ReadFull(clientConn, challengePayload); err != nil {
		t.Fatal(err)
	}
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// Reference: WorldSocket::HandlePing drops the connection when no world
	// session is attached yet (peer not authenticated or recently kicked).
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_PING), pingPayload(9, 50), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := clientConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("unauthenticated ping should close the connection")
	}
}

