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

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func writeTalentDBC(t *testing.T, dir string, id, tabID, tierID, col uint32, ranks [5]uint32, prereqTalent, prereqRank uint32) {
	t.Helper()
	const fieldCount = 23
	record := make([]uint32, fieldCount)
	record[0] = id
	record[1] = tabID
	record[2] = tierID
	record[3] = col
	for i := 0; i < 5; i++ {
		record[4+i] = ranks[i]
	}
	record[13] = prereqTalent
	record[16] = prereqRank
	recordBytes := make([]byte, fieldCount*4)
	for i, val := range record {
		binary.LittleEndian.PutUint32(recordBytes[i*4:(i+1)*4], val)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dir, "Talent.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

type talentDBCRecord struct {
	id           uint32
	tabID        uint32
	tierID       uint32
	col          uint32
	ranks        [5]uint32
	prereqTalent uint32
	prereqRank   uint32
}

func writeMultipleTalentDBC(t *testing.T, dir string, records []talentDBCRecord) {
	t.Helper()
	const fieldCount = 23
	var allBytes []byte
	for _, rec := range records {
		record := make([]uint32, fieldCount)
		record[0] = rec.id
		record[1] = rec.tabID
		record[2] = rec.tierID
		record[3] = rec.col
		for i := 0; i < 5; i++ {
			record[4+i] = rec.ranks[i]
		}
		record[13] = rec.prereqTalent
		record[16] = rec.prereqRank
		recordBytes := make([]byte, fieldCount*4)
		for i, val := range record {
			binary.LittleEndian.PutUint32(recordBytes[i*4:(i+1)*4], val)
		}
		allBytes = append(allBytes, recordBytes...)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(records)))
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dir, "Talent.dbc"), append(header, append(allBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSkillsTestSession(t *testing.T, player *playerState) (*session, net.Conn, *Server) {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE IF NOT EXISTS character_talent (guid INTEGER, spell INTEGER, talentGroup INTEGER, PRIMARY KEY(guid, spell, talentGroup))",
		"CREATE TABLE IF NOT EXISTS character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER, PRIMARY KEY(guid, spell))",
		"CREATE TABLE IF NOT EXISTS character_skills (guid INTEGER, skill INTEGER, value INTEGER, max INTEGER, PRIMARY KEY(guid, skill))",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	server := &Server{
		CharactersStore: charStore,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		RealmID:         1,
		Config:          config.Default(),
		sessions:        make(map[*session]struct{}),
	}
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	s := &session{
		server:       server,
		conn:         serverConn,
		authed:       true,
		accountID:    1,
		playerLoaded: player != nil,
		playerGUID:   10,
		player:       player,
	}
	server.sessions[s] = struct{}{}
	return s, clientConn, server
}

func TestHandleLearnTalentFlow(t *testing.T) {
	dbcDir := t.TempDir()
	writeTalentDBC(t, dbcDir, 1, 10, 0, 0, [5]uint32{101, 102, 103, 104, 105}, 0, 0)

	player := &playerState{
		GUID:    10,
		Name:    "Hero",
		Level:   80, // 71 free talent points
		Talents: make(map[uint32]uint8),
	}
	s, clientConn, server := newSkillsTestSession(t, player)
	server.Data = wotlk.NewStore(dbcDir)

	// Learn talent 1 rank 0
	payload := protocol.NewBuffer(8)
	payload.WriteU32(1) // talentID
	payload.WriteU32(0) // requestedRank

	done := make(chan struct{})
	go func() {
		if !s.handleLearnTalent(context.Background(), payload.Bytes()) {
			t.Error("handleLearnTalent returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, talPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_TALENTS_INFO) {
		t.Fatalf("unexpected opcode %x", opcode)
	}

	r := protocol.NewReader(talPayload)
	pet, err := r.ReadU8()
	if err != nil || pet != 0 {
		t.Fatalf("pet=%d err=%v", pet, err)
	}
	freePoints, err := r.ReadU32()
	if err != nil || freePoints != 70 {
		t.Fatalf("freePoints=%d err=%v", freePoints, err)
	}
	specsCount, err := r.ReadU8()
	if err != nil || specsCount != 1 {
		t.Fatalf("specsCount=%d err=%v", specsCount, err)
	}
	activeSpec, err := r.ReadU8()
	if err != nil || activeSpec != 0 {
		t.Fatalf("activeSpec=%d err=%v", activeSpec, err)
	}
	talCount, err := r.ReadU8()
	if err != nil || talCount != 1 {
		t.Fatalf("talCount=%d err=%v", talCount, err)
	}
	tid, err := r.ReadU32()
	if err != nil || tid != 1 {
		t.Fatalf("tid=%d err=%v", tid, err)
	}
	rank, err := r.ReadU8()
	if err != nil || rank != 0 {
		t.Fatalf("rank=%d err=%v", rank, err)
	}

	// Verify database persistence
	var count int
	err = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_talent WHERE guid = 10 AND spell = 101").Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("character_talent count=%d err=%v", count, err)
	}
	err = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_spell WHERE guid = 10 AND spell = 101").Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("character_spell count=%d err=%v", count, err)
	}

	// Learn invalid rank >= 5
	badPayload := protocol.NewBuffer(8)
	badPayload.WriteU32(1)
	badPayload.WriteU32(5)
	if s.learnTalent(context.Background(), 1, 5) {
		t.Fatal("learnTalent should reject rank >= 5")
	}

	// Low level player (< 10) has no talent points
	lowLvlPlayer := &playerState{GUID: 11, Level: 9}
	lowSess, _, _ := newSkillsTestSession(t, lowLvlPlayer)
	if lowSess.freeTalentPoints() != 0 {
		t.Fatalf("low level player should have 0 talent points, got %d", lowSess.freeTalentPoints())
	}
	if lowSess.learnTalent(context.Background(), 1, 0) {
		t.Fatal("low level player should not be able to learn talents")
	}
}

func TestHandleLearnPreviewTalents(t *testing.T) {
	player := &playerState{
		GUID:    10,
		Name:    "Hero",
		Level:   80,
		Talents: make(map[uint32]uint8),
	}
	s, clientConn, _ := newSkillsTestSession(t, player)

	payload := protocol.NewBuffer(32)
	payload.WriteU32(2) // 2 talents
	payload.WriteU32(1) // talent 1
	payload.WriteU32(0) // rank 0
	payload.WriteU32(2) // talent 2
	payload.WriteU32(0) // rank 0

	done := make(chan struct{})
	go func() {
		if !s.handleLearnPreviewTalents(context.Background(), payload.Bytes()) {
			t.Error("handleLearnPreviewTalents returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, talPayload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_TALENTS_INFO) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(talPayload)
	_, _ = r.ReadU8()  // pet
	_, _ = r.ReadU32() // freePoints
	_, _ = r.ReadU8()  // specsCount
	_, _ = r.ReadU8()  // activeSpec
	count, err := r.ReadU8()
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if len(player.Talents) != 2 {
		t.Fatalf("expected 2 learned talents, got %d", len(player.Talents))
	}
}

func TestHandleUnlearnSkillFlow(t *testing.T) {
	player := &playerState{
		GUID:  10,
		Name:  "Gatherer",
		Level: 50,
		Skills: []playerSkill{
			{Skill: 182, Value: 150, Max: 300}, // Herbalism
			{Skill: 185, Value: 200, Max: 300}, // Cooking
		},
	}
	s, clientConn, server := newSkillsTestSession(t, player)

	if _, err := server.CharactersStore.DB.Exec("INSERT INTO character_skills (guid, skill, value, max) VALUES (10, 182, 150, 300), (10, 185, 200, 300)"); err != nil {
		t.Fatal(err)
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(182) // unlearn Herbalism

	done := make(chan struct{})
	go func() {
		if !s.handleUnlearnSkill(context.Background(), payload.Bytes()) {
			t.Error("handleUnlearnSkill returned false")
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

	if len(player.Skills) != 1 || player.Skills[0].Skill != 185 {
		t.Fatalf("unexpected remaining skills: %+v", player.Skills)
	}

	var count int
	err = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_skills WHERE guid = 10 AND skill = 182").Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("character_skills count for 182=%d err=%v", count, err)
	}
}

func TestDualSpecTalentPersistenceAndWipe(t *testing.T) {
	dbcDir := t.TempDir()
	writeMultipleTalentDBC(t, dbcDir, []talentDBCRecord{
		{id: 1, tabID: 10, tierID: 0, col: 0, ranks: [5]uint32{101, 102, 103, 104, 105}},
		{id: 2, tabID: 10, tierID: 0, col: 1, ranks: [5]uint32{201, 202, 203, 204, 205}},
	})

	player := &playerState{
		GUID:              10,
		Name:              "Hero",
		Level:             80,
		TalentGroupsCount: 2,
		ActiveTalentGroup: 0,
		Talents:           make(map[uint32]uint8),
	}
	s, clientConn, server := newSkillsTestSession(t, player)
	server.Data = wotlk.NewStore(dbcDir)

	ctx := context.Background()

	// 1. Learn talent 1 in spec 0
	if !s.learnTalent(ctx, 1, 0) {
		t.Fatal("failed to learn talent 1 in spec 0")
	}

	// 2. Switch active spec to 1 and learn talent 2 in spec 1
	player.ActiveTalentGroup = 1
	player.Talents = make(map[uint32]uint8)
	if !s.learnTalent(ctx, 2, 0) {
		t.Fatal("failed to learn talent 2 in spec 1")
	}

	// 3. Verify both are saved in character_talent under distinct talentGroup
	var countSpec0, countSpec1 int
	_ = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_talent WHERE guid = 10 AND talentGroup = 0 AND spell = 101").Scan(&countSpec0)
	_ = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_talent WHERE guid = 10 AND talentGroup = 1 AND spell = 201").Scan(&countSpec1)
	if countSpec0 != 1 || countSpec1 != 1 {
		t.Fatalf("expected 1 talent in each spec, got spec0=%d spec1=%d", countSpec0, countSpec1)
	}

	// 4. Test loadPlayerTalents restores spec 1
	restoredState := playerState{
		GUID:              10,
		ActiveTalentGroup: 1,
	}
	s.loadPlayerTalents(ctx, &restoredState)
	if len(restoredState.Talents) != 1 || restoredState.Talents[2] != 0 {
		t.Fatalf("expected restored talent 2 rank 0, got %+v", restoredState.Talents)
	}

	// 5. Test talent wipe only wipes the active spec (spec 1)
	wipeBuf := protocol.NewBuffer(8)
	wipeBuf.WriteU64(999)
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for {
			_ = clientConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()
	if !s.handleTalentWipeConfirm(ctx, wipeBuf.Bytes()) {
		t.Error("handleTalentWipeConfirm failed")
	}
	<-drainDone

	// Spec 0 should still exist, spec 1 should be gone
	_ = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_talent WHERE guid = 10 AND talentGroup = 0 AND spell = 101").Scan(&countSpec0)
	_ = server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_talent WHERE guid = 10 AND talentGroup = 1 AND spell = 201").Scan(&countSpec1)
	if countSpec0 != 1 || countSpec1 != 0 {
		t.Fatalf("expected spec0=1 spec1=0 after wipe, got spec0=%d spec1=%d", countSpec0, countSpec1)
	}
}
