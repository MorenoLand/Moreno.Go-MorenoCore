package world

import (
	"context"
	"database/sql"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// writeAchievementDBCs writes minimal Achievement.dbc and
// Achievement_Criteria.dbc fixtures with one achievement (5000) driven by one
// kill-creature criterion (9001) requiring 2 kills of creature 300.
func writeAchievementDBCs(t *testing.T, dir string) {
	t.Helper()
	// Achievement.dbc: 62 u32 fields (highest used: 61)
	achFields := 62
	achRow := make([]uint32, achFields)
	achRow[0] = 5000 // ID
	achRow[1] = 0xFFFFFFFF
	achRow[38] = 1
	achRow[39] = 10 // points
	achRow[41] = 0
	achRow[60] = 0
	achRow[61] = 0
	// Achievement_Criteria.dbc: 31 fields (highest used: 29)
	critFields := 31
	critRow := make([]uint32, critFields)
	critRow[0] = 9001 // ID
	critRow[1] = 5000 // achievement
	critRow[2] = 0    // KILL_CREATURE
	critRow[3] = 300  // asset: creature entry
	critRow[4] = 2    // quantity: 2 kills

	write := func(name string, rows ...[]uint32) {
		var body []byte
		for _, row := range rows {
			for _, v := range row {
				body = append(body, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
			}
		}
		out := []byte("WDBC")
		put := func(v uint32) {
			out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
		}
		put(uint32(len(rows)))
		put(uint32(len(rows[0])))
		put(uint32(len(rows[0])) * 4)
		put(1)
		out = append(out, body...)
		out = append(out, 0)
		if err := os.WriteFile(filepath.Join(dir, name), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Achievement.dbc", achRow)
	write("Achievement_Criteria.dbc", critRow)
}

func newAchievementTestSession(t *testing.T, player *playerState) (*session, net.Conn, *sql.DB, *Server) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE character_achievement (guid INTEGER NOT NULL, achievement INTEGER NOT NULL, date INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (guid, achievement))",
		"CREATE TABLE character_achievement_progress (guid INTEGER NOT NULL, criteria INTEGER NOT NULL, counter INTEGER NOT NULL, date INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (guid, criteria))",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	writeAchievementDBCs(t, dir)
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	server := &Server{CharactersStore: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Data: wotlk.NewStore(dir)}
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 9, player: player}
	state.earnedAchievements = make(map[uint32]uint32)
	state.criteriaProgress = make(map[uint32]*criteriaProgressState)
	return state, clientConn, db, server
}

func TestAchievementCriteriaProgressAndCompletion(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100, Race: 1}
	state, clientConn, db, server := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	// First kill: criteria 9001 progress 1/2, no completion yet.
	state.updateAchievementCriteria(criteriaTypeKillCreature, 300, 1)
	if progress := state.criteriaProgress[9001]; progress == nil || progress.Counter != 1 {
		t.Fatalf("progress=%+v", progress)
	}
	if _, earned := state.earnedAchievements[5000]; earned {
		t.Fatal("achievement completed early")
	}
	var counter int64
	if err := db.QueryRow("SELECT counter FROM character_achievement_progress WHERE guid = 9 AND criteria = 9001").Scan(&counter); err != nil || counter != 1 {
		t.Fatalf("persisted counter=%d err=%v", counter, err)
	}

	// Different creature: no matching criteria.
	state.updateAchievementCriteria(criteriaTypeKillCreature, 301, 1)
	if progress := state.criteriaProgress[9001]; progress.Counter != 1 {
		t.Fatalf("unrelated kill moved progress: %+v", progress)
	}

	// Second kill: criterion satisfied, achievement 5000 completes.
	state.updateAchievementCriteria(criteriaTypeKillCreature, 300, 1)
	if _, earned := state.earnedAchievements[5000]; !earned {
		t.Fatal("achievement not completed at 2/2")
	}
	var achDate int64
	if err := db.QueryRow("SELECT date FROM character_achievement WHERE guid = 9 AND achievement = 5000").Scan(&achDate); err != nil || achDate == 0 {
		t.Fatalf("achievement not persisted: date=%d err=%v", achDate, err)
	}

	// Further kills on the completed achievement do nothing.
	before := state.criteriaProgress[9001].Counter
	state.updateAchievementCriteria(criteriaTypeKillCreature, 300, 1)
	if state.criteriaProgress[9001].Counter != before {
		t.Fatal("progress moved after achievement completion")
	}
	_ = server
}

func TestAllAchievementDataPacketLayout(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	now := uint32(time.Now().Unix())
	state.earnedAchievements[5000] = now
	state.criteriaProgress[9001] = &criteriaProgressState{CriteriaID: 9001, Counter: 2, Date: now}
	result := make(chan bool, 1)
	go func() { result <- true; state.sendAllAchievementData() }()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-result
	if opcode != uint16(protocol.OpcodeSMSG_ALL_ACHIEVEMENT_DATA) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	id, _ := reader.ReadU32()
	date, _ := reader.ReadU32()
	if id != 5000 || date != now {
		t.Fatalf("earned block id=%d date=%d", id, date)
	}
	sep, _ := reader.ReadU32()
	if sep != 0xFFFFFFFF {
		t.Fatalf("first separator=%x", sep)
	}
	critID, _ := reader.ReadU32()
	if critID != 9001 {
		t.Fatalf("criteria id=%d", critID)
	}
	quantity, err := reader.ReadPackedGUID()
	if err != nil || quantity != 2 {
		t.Fatalf("packed quantity=%d err=%v", quantity, err)
	}
	playerGUID, err := reader.ReadPackedGUID()
	if err != nil || playerGUID != 9 {
		t.Fatalf("packed player=%d err=%v", playerGUID, err)
	}
	rest := payload[reader.Position():]
	// flags u32, date u32, elapsed u32, creation u32, then final -1
	if len(rest) < 20 {
		t.Fatalf("short tail=%d", len(rest))
	}
	if got := binary.LittleEndian.Uint32(rest[16:20]); got != 0xFFFFFFFF {
		t.Fatalf("tail separator=%x", got)
	}
}

func TestInspectAchievementsReturnsTargetState(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100}
	state, clientConn, db, _ := newAchievementTestSession(t, player)
	now := uint32(time.Now().Unix())
	if _, err := db.Exec("INSERT INTO character_achievement (guid, achievement, date) VALUES (77, 5000, ?)", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO character_achievement_progress (guid, criteria, counter, date) VALUES (77, 9001, 2, ?)", now); err != nil {
		t.Fatal(err)
	}
	result := make(chan bool, 1)
	payload := protocol.NewBuffer(8)
	payload.WriteU64(77)
	go func() { result <- true; state.handleQueryInspectAchievements(context.Background(), payload.Bytes()) }()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-result
	if opcode != uint16(protocol.OpcodeSMSG_RESPOND_INSPECT_ACHIEVEMENTS) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(response)
	target, err := reader.ReadPackedGUID()
	if err != nil || target != 77 {
		t.Fatalf("target=%d err=%v", target, err)
	}
	id, _ := reader.ReadU32()
	date, _ := reader.ReadU32()
	if id != 5000 || date != now {
		t.Fatalf("inspect earned id=%d date=%d", id, date)
	}
	if sep, _ := reader.ReadU32(); sep != 0xFFFFFFFF {
		t.Fatalf("separator=%x", sep)
	}
	critID, _ := reader.ReadU32()
	if critID != 9001 {
		t.Fatalf("inspect criteria=%d", critID)
	}
}
