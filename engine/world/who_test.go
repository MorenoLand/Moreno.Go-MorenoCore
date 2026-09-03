package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestHandleKeepAlive(t *testing.T) {
	s := &session{}
	if !s.handleKeepAlive() {
		t.Fatal("handleKeepAlive should return true")
	}
}

func newWhoTestServer(t *testing.T) (*Server, *sql.DB, *sql.DB) {
	t.Helper()
	authDB, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	charDB, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		authDB.Close()
		charDB.Close()
	})

	authDB.SetMaxOpenConns(1)
	charDB.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT, email TEXT, last_ip TEXT)",
		"CREATE TABLE rbac_account_permissions (accountId INTEGER, permissionId INTEGER, granted INTEGER, realmId INTEGER, PRIMARY KEY (accountId, permissionId, realmId))",
		"CREATE TABLE rbac_default_permissions (secId INTEGER, permissionId INTEGER, realmId INTEGER)",
		"CREATE TABLE rbac_linked_permissions (id INTEGER, linkedId INTEGER)",
	} {
		if _, err := authDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	for _, stmt := range []string{
		"CREATE TABLE guild (guildid INTEGER PRIMARY KEY, name TEXT)",
	} {
		if _, err := charDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{
		AuthStore:       &database.Store{Name: "auth", Backend: database.BackendSQLite, DB: authDB},
		CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: charDB},
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config:          config.Default(),
		RealmID:         1,
		sessions:        make(map[*session]struct{}),
	}
	return server, authDB, charDB
}

func TestHandleWhoFiltersAndPacketLayout(t *testing.T) {
	server, _, charDB := newWhoTestServer(t)
	dbcDir := t.TempDir()
	writeAreaTableDBC(t, dbcDir, [][5]uint32{{12, 0, 0, 0, 0}})
	server.Data = wotlk.NewStore(dbcDir)

	if _, err := charDB.Exec("INSERT INTO guild (guildid, name) VALUES (1, 'Knights')"); err != nil {
		t.Fatal(err)
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	viewer := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Viewer", Level: 80, Race: 1, Class: 1}}
	alice := &session{server: server, authed: true, playerLoaded: true, playerGUID: 2, player: &playerState{GUID: 2, Name: "Alice", Level: 80, Race: 1, Class: 2, Zone: 12, GuildID: 1}}
	bob := &session{server: server, authed: true, playerLoaded: true, playerGUID: 3, player: &playerState{GUID: 3, Name: "Bob", Level: 20, Race: 2, Class: 1, Zone: 14}}
	charlie := &session{server: server, authed: true, playerLoaded: true, playerGUID: 4, player: &playerState{GUID: 4, Name: "Charlie", Level: 80, Race: 1, Class: 5, Zone: 12, ExtraFlags: playerExtraGMInvisible}}

	server.sessions[viewer] = struct{}{}
	server.sessions[alice] = struct{}{}
	server.sessions[bob] = struct{}{}
	server.sessions[charlie] = struct{}{}

	// Query: Level 1-30 -> only Bob should match
	whoPayload := protocol.NewBuffer(64)
	whoPayload.WriteU32(1)      // levelMin
	whoPayload.WriteU32(30)     // levelMax
	whoPayload.WriteCString("") // playerName
	whoPayload.WriteCString("") // guildName
	whoPayload.WriteU32(0)      // raceMask
	whoPayload.WriteU32(0)      // classMask
	whoPayload.WriteU32(0)      // zonesCount
	whoPayload.WriteU32(0)      // strCount

	done := make(chan struct{})
	go func() {
		if !viewer.handleWho(context.Background(), whoPayload.Bytes()) {
			t.Error("handleWho returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_WHO) {
		t.Fatalf("unexpected opcode %x", opcode)
	}

	r := protocol.NewReader(payload)
	displayCount, err := r.ReadU32()
	if err != nil || displayCount != 1 {
		t.Fatalf("displayCount=%d err=%v", displayCount, err)
	}
	matchCount, err := r.ReadU32()
	if err != nil || matchCount != 1 {
		t.Fatalf("matchCount=%d err=%v", matchCount, err)
	}
	name, err := r.ReadCString()
	if err != nil || name != "Bob" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	guild, err := r.ReadCString()
	if err != nil || guild != "" {
		t.Fatalf("guild=%q err=%v", guild, err)
	}
	lvl, err := r.ReadU32()
	if err != nil || lvl != 20 {
		t.Fatalf("level=%d err=%v", lvl, err)
	}
	class, err := r.ReadU32()
	if err != nil || class != 1 {
		t.Fatalf("class=%d err=%v", class, err)
	}
	race, err := r.ReadU32()
	if err != nil || race != 2 {
		t.Fatalf("race=%d err=%v", race, err)
	}
	gender, err := r.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	_ = gender
	zone, err := r.ReadU32()
	if err != nil || zone != 14 {
		t.Fatalf("zone=%d err=%v", zone, err)
	}

	// Test malformed packet: zonesCount > 10
	badZones := protocol.NewBuffer(64)
	badZones.WriteU32(0)
	badZones.WriteU32(100)
	badZones.WriteCString("")
	badZones.WriteCString("")
	badZones.WriteU32(0)
	badZones.WriteU32(0)
	badZones.WriteU32(11) // > 10
	if viewer.handleWho(context.Background(), badZones.Bytes()) {
		t.Fatal("handleWho should fail with zonesCount > 10")
	}
}

func TestHandleWhoIsPermissionsAndLookup(t *testing.T) {
	server, authDB, _ := newWhoTestServer(t)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	viewer := &session{server: server, conn: serverConn, authed: true, accountID: 10, security: 3, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Admin"}}
	target := &session{server: server, authed: true, accountID: 20, playerLoaded: true, playerGUID: 2, player: &playerState{GUID: 2, Name: "Target"}}
	server.sessions[viewer] = struct{}{}
	server.sessions[target] = struct{}{}

	if _, err := authDB.Exec("INSERT INTO account (id, username, email, last_ip) VALUES (20, 'targetuser', 'target@example.com', '127.0.0.1')"); err != nil {
		t.Fatal(err)
	}

	// Case 1: Permission NOT granted
	payload := protocol.NewBuffer(16)
	payload.WriteCString("Target")

	done := make(chan struct{})
	go func() {
		if !viewer.handleWhoIs(context.Background(), payload.Bytes()) {
			t.Error("handleWhoIs returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, notifyPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_NOTIFICATION) {
		t.Fatalf("expected SMSG_NOTIFICATION, got %x", opcode)
	}
	nr := protocol.NewReader(notifyPayload)
	notif, _ := nr.ReadCString()
	if notif != "You do not have permission to use that command." {
		t.Fatalf("unexpected notification: %q", notif)
	}

	// Grant permission 43
	if _, err := authDB.Exec("INSERT INTO rbac_account_permissions (accountId, permissionId, granted, realmId) VALUES (10, 43, 1, -1)"); err != nil {
		t.Fatal(err)
	}

	// Case 2: Permission granted, character online
	done2 := make(chan struct{})
	go func() {
		if !viewer.handleWhoIs(context.Background(), payload.Bytes()) {
			t.Error("handleWhoIs returned false")
		}
		close(done2)
	}()

	opcode2, whoisPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done2
	if opcode2 != uint16(protocol.OpcodeSMSG_WHOIS) {
		t.Fatalf("expected SMSG_WHOIS, got %x", opcode2)
	}
	wr := protocol.NewReader(whoisPayload)
	whoisMsg, _ := wr.ReadCString()
	expected := "Target's account is targetuser, e-mail: target@example.com, last ip: 127.0.0.1"
	if whoisMsg != expected {
		t.Fatalf("expected %q, got %q", expected, whoisMsg)
	}
}

func TestHandleInspectChecksAndPacketLayout(t *testing.T) {
	server, _, _ := newWhoTestServer(t)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	// Both Alliance (Race 1 = Human)
	inspector := &session{
		server:       server,
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Name: "Inspector", Race: 1, Map: 0, X: 10.0, Y: 10.0, Z: 0.0},
	}
	target := &session{
		server:       server,
		authed:       true,
		playerLoaded: true,
		playerGUID:   2,
		player: &playerState{
			GUID:      2,
			Name:      "Target",
			Race:      1,
			Map:       0,
			X:         15.0, // distance = 5.0 yards (< 28.0 yards)
			Y:         10.0,
			Z:         0.0,
			Equipment: "12345 55 67890 0",
		},
	}
	server.sessions[inspector] = struct{}{}
	server.sessions[target] = struct{}{}

	payload := protocol.NewBuffer(8)
	payload.WriteU64(2)

	done := make(chan struct{})
	go func() {
		if !inspector.handleInspect(context.Background(), payload.Bytes()) {
			t.Error("handleInspect returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, inspectPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_INSPECT_TALENT) {
		t.Fatalf("expected SMSG_INSPECT_TALENT, got %x", opcode)
	}

	r := protocol.NewReader(inspectPayload)
	guid, err := r.ReadPackedGUID()
	if err != nil || guid != 2 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
	unspentTalents, err := r.ReadU32()
	if err != nil || unspentTalents != 0 {
		t.Fatalf("unspentTalents=%d err=%v", unspentTalents, err)
	}
	talentGroupCount, err := r.ReadU8()
	if err != nil || talentGroupCount != 0 {
		t.Fatalf("talentGroupCount=%d err=%v", talentGroupCount, err)
	}
	talentGroupIndex, err := r.ReadU8()
	if err != nil || talentGroupIndex != 0 {
		t.Fatalf("talentGroupIndex=%d err=%v", talentGroupIndex, err)
	}

	// SlotUsedMask: slot 0 (1<<0 = 1) and slot 1 (1<<1 = 2) -> 3
	slotUsedMask, err := r.ReadU32()
	if err != nil || slotUsedMask != 3 {
		t.Fatalf("slotUsedMask=%d err=%v", slotUsedMask, err)
	}
	// Item 0: entry 12345, enchantMask 1, enchant 55
	entry0, err := r.ReadU32()
	if err != nil || entry0 != 12345 {
		t.Fatalf("entry0=%d err=%v", entry0, err)
	}
	enchMask0, err := r.ReadU16()
	if err != nil || enchMask0 != 1 {
		t.Fatalf("enchMask0=%d err=%v", enchMask0, err)
	}
	ench0, err := r.ReadU16()
	if err != nil || ench0 != 55 {
		t.Fatalf("ench0=%d err=%v", ench0, err)
	}
	// Item 1: entry 67890, enchantMask 0
	entry1, err := r.ReadU32()
	if err != nil || entry1 != 67890 {
		t.Fatalf("entry1=%d err=%v", entry1, err)
	}
	enchMask1, err := r.ReadU16()
	if err != nil || enchMask1 != 0 {
		t.Fatalf("enchMask1=%d err=%v", enchMask1, err)
	}

	// Test inspecting target with talents
	target.player.Talents = map[uint32]uint8{123: 5}
	target.player.Level = 80
	doneTalents := make(chan struct{})
	go func() {
		if !inspector.handleInspect(context.Background(), payload.Bytes()) {
			t.Error("handleInspect with talents returned false")
		}
		close(doneTalents)
	}()
	_, talentPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneTalents
	tr := protocol.NewReader(talentPayload)
	_, _ = tr.ReadPackedGUID()
	_, _ = tr.ReadU32() // unspent
	tGroupCount, _ := tr.ReadU8()
	tGroupIdx, _ := tr.ReadU8()
	if tGroupCount != 1 || tGroupIdx != 0 {
		t.Fatalf("expected tGroupCount=1, tGroupIdx=0, got %d, %d", tGroupCount, tGroupIdx)
	}
	tCount, _ := tr.ReadU8()
	if tCount != 1 {
		t.Fatalf("expected tCount=1, got %d", tCount)
	}
	tID, _ := tr.ReadU32()
	tRank, _ := tr.ReadU8()
	if tID != 123 || tRank != 5 {
		t.Fatalf("expected talent 123 rank 5, got %d rank %d", tID, tRank)
	}

	// Out of range check (> 28 yards): target at X=100
	target.player.X = 100.0
	if !inspector.handleInspect(context.Background(), payload.Bytes()) {
		t.Fatal("handleInspect should return true safely when out of range")
	}

	// Hostile check: Target is Horde (Race 2 = Orc)
	target.player.X = 15.0
	target.player.Race = 2
	if !inspector.handleInspect(context.Background(), payload.Bytes()) {
		t.Fatal("handleInspect should return true safely when target is hostile")
	}
}
