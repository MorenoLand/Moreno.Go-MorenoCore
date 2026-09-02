//go:build ignore

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

func newReputationTestSession(t *testing.T, reputations []playerReputation) (*session, net.Conn, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE character_reputation (guid INTEGER NOT NULL, faction INTEGER NOT NULL, flags INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	for _, reputation := range reputations {
		if _, err := db.Exec("INSERT INTO character_reputation (guid, faction, flags) VALUES (?, ?, ?)", uint64(9), reputation.FactionID, reputation.Flags); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	server := &Server{CharactersStore: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RealmID: 1}
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 9, player: &playerState{Reputations: reputations}}
	return state, clientConn, db
}

func setWatchedFactionPayload(faction uint32) []byte {
	buffer := protocol.NewBuffer(4)
	buffer.WriteU32(faction)
	return buffer.Bytes()
}

func setFactionInactivePayload(listID uint32, inactive uint8) []byte {
	buffer := protocol.NewBuffer(5)
	buffer.WriteU32(listID)
	buffer.WriteU8(inactive)
	return buffer.Bytes()
}

func TestSetWatchedFactionUpdatesFieldAndPushesValues(t *testing.T) {
	state, clientConn, _ := newReputationTestSession(t, nil)
	result := make(chan bool, 1)
	go func() { result <- state.handleSetWatchedFaction(setWatchedFactionPayload(72)) }()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("expected update object opcode, got opcode=%x", opcode)
	}
	if !<-result {
		t.Fatal("watched faction handler failed")
	}
	if state.player.WatchedFaction != 72 {
		t.Fatalf("watched faction=%d", state.player.WatchedFaction)
	}
	decompressed := payload
	if opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		decompressed, err = protocol.DecompressUpdatePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader := protocol.NewReader(decompressed)
	if _, err := reader.ReadU32(); err != nil { // block count
		t.Fatal(err)
	}
	updateType, err := reader.ReadU8()
	if err != nil || updateType != protocol.UpdateValues {
		t.Fatalf("update type=%d err=%v", updateType, err)
	}
	packed, err := reader.ReadPackedGUID()
	if err != nil || packed != 9 {
		t.Fatalf("packed guid=%d err=%v", packed, err)
	}
	blockCount, err := reader.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	maskWords := make([]uint32, blockCount)
	for index := 0; index < int(blockCount); index++ {
		maskWords[index], err = reader.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
	}
	hasField := func(index int) bool {
		return maskWords[index/32]&(1<<uint(index%32)) != 0
	}
	if !hasField(unitFieldWatchedFaction) {
		t.Fatal("watched faction field not set in mask")
	}
	var value uint32
	for index := 0; index <= unitFieldWatchedFaction; index++ {
		if hasField(index) {
			value, err = reader.ReadU32()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if value != 72 {
		t.Fatalf("watched faction value=%d", value)
	}
}

func TestSetWatchedFactionWithoutPlayerIsIgnored(t *testing.T) {
	state := &session{server: &Server{}}
	if !state.handleSetWatchedFaction(setWatchedFactionPayload(72)) {
		t.Fatal("watched faction without player should be ignored")
	}
}

func TestSetWatchedFactionMalformedPayloadRejected(t *testing.T) {
	state, _, _ := newReputationTestSession(t, nil)
	if state.handleSetWatchedFaction(setWatchedFactionPayload(72)[:2]) {
		t.Fatal("truncated watched faction payload should be rejected")
	}
}

func TestSetFactionInactiveTogglesFlagAndPersists(t *testing.T) {
	state, _, db := newReputationTestSession(t, []playerReputation{{FactionID: 72, ListID: 72, Standing: 3000, Flags: factionFlagVisible}})
	payload := setFactionInactivePayload(72, 1)
	if !state.handleSetFactionInactive(context.Background(), payload) {
		t.Fatal("faction inactive handler failed")
	}
	if state.player.Reputations[0].Flags&factionFlagInactive == 0 {
		t.Fatalf("flags=%x missing inactive bit", state.player.Reputations[0].Flags)
	}
	var flags int64
	if err := db.QueryRow("SELECT flags FROM character_reputation WHERE guid = 9 AND faction = 72").Scan(&flags); err != nil {
		t.Fatal(err)
	}
	if uint8(flags)&factionFlagInactive == 0 {
		t.Fatalf("persisted flags=%x missing inactive bit", uint8(flags))
	}
	// Reference ignores the request when the state already matches.
	before := state.player.Reputations[0].Flags
	if !state.handleSetFactionInactive(context.Background(), setFactionInactivePayload(72, 1)) {
		t.Fatal("redundant inactive request should be accepted as a no-op")
	}
	if state.player.Reputations[0].Flags != before {
		t.Fatal("redundant request changed flags")
	}
	if !state.handleSetFactionInactive(context.Background(), setFactionInactivePayload(72, 0)) {
		t.Fatal("clearing inactive failed")
	}
	if state.player.Reputations[0].Flags&factionFlagInactive != 0 {
		t.Fatalf("flags=%x inactive bit not cleared", state.player.Reputations[0].Flags)
	}
}

func TestSetFactionInactiveRejectsHiddenAndInvisible(t *testing.T) {
	reputations := []playerReputation{
		{FactionID: 21, ListID: 21, Flags: factionFlagHidden | factionFlagVisible},
		{FactionID: 22, ListID: 22, Flags: factionFlagInvisibleForced | factionFlagVisible},
		{FactionID: 23, ListID: 23, Flags: 0},
	}
	state, _, _ := newReputationTestSession(t, reputations)
	for _, listID := range []uint32{21, 22, 23} {
		if !state.handleSetFactionInactive(context.Background(), setFactionInactivePayload(listID, 1)) {
			t.Fatalf("inactive request for list %d should be a no-op", listID)
		}
	}
	for index := range state.player.Reputations {
		if state.player.Reputations[index].Flags&factionFlagInactive != 0 {
			t.Fatalf("faction %d became inactive against the reference guard", state.player.Reputations[index].FactionID)
		}
	}
}

func TestSetFactionInactiveUnknownListIDIgnored(t *testing.T) {
	state, _, _ := newReputationTestSession(t, []playerReputation{{FactionID: 72, ListID: 72, Flags: factionFlagVisible}})
	if !state.handleSetFactionInactive(context.Background(), setFactionInactivePayload(999, 1)) {
		t.Fatal("unknown list id should be ignored")
	}
	if state.player.Reputations[0].Flags&factionFlagInactive != 0 {
		t.Fatal("unknown list id changed faction state")
	}
}

func TestPlayerLogoutConsumesPacketWithoutStateChange(t *testing.T) {
	state, _, _ := newReputationTestSession(t, nil)
	state.logoutAt = time.Now().Add(5 * time.Second)
	if !state.handlePlayerLogout() {
		t.Fatal("player logout handler failed")
	}
	if state.logoutAt.IsZero() {
		t.Fatal("player logout must not clear the pending logout deadline")
	}
	if !state.playerLoaded {
		t.Fatal("player logout must not complete the logout itself")
	}
}

