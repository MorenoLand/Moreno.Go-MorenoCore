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

func TestAchievementCastSpellAndSetVariants(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100, Race: 1}
	state, clientConn, db, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	// CAST_SPELL with no matching criterion in the fixture: no progress, no error.
	state.updateAchievementCriteria(criteriaTypeCastSpell, 12345, 1)
	if len(state.criteriaProgress) != 0 {
		t.Fatalf("unmatched cast created progress: %+v", state.criteriaProgress)
	}

	// Set-variant on reputation (absolute, never regresses).
	state.earnedAchievements = make(map[uint32]uint32)
	_ = db
	// Simulate a reputation criterion via the index directly.
	achievementIndex.mu.Lock()
	crit := achievementCriteriaEntry{ID: 9100, AchievementID: 5100, Type: criteriaTypeGainReputation, Asset: 72, Quantity: 42999}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeGainReputation, 72)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeGainReputation, 72)], crit)
	achievementIndex.byAchieve[5100] = append(achievementIndex.byAchieve[5100], crit)
	achievementIndex.achieveByID[5100] = achievementEntry{ID: 5100, Faction: -1}
	achievementIndex.mu.Unlock()

	state.setAchievementCriteria(criteriaTypeGainReputation, 72, 21000)
	progress := state.criteriaProgress[9100]
	if progress == nil || progress.Counter != 21000 {
		t.Fatalf("reputation progress=%+v", progress)
	}
	// Lower value must not regress.
	state.setAchievementCriteria(criteriaTypeGainReputation, 72, 15000)
	if state.criteriaProgress[9100].Counter != 21000 {
		t.Fatalf("absolute progress regressed: %d", state.criteriaProgress[9100].Counter)
	}
	// Meeting quantity completes the achievement.
	state.setAchievementCriteria(criteriaTypeGainReputation, 72, 42999)
	if _, earned := state.earnedAchievements[5100]; !earned {
		t.Fatal("reputation achievement not completed at exact threshold")
	}
}

func TestAchievementDeathAndOwnItemHooks(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 0, MaxHealth: 100, Race: 1}
	state, clientConn, db, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	_ = db

	achievementIndex.mu.Lock()
	deathCrit := achievementCriteriaEntry{ID: 9200, AchievementID: 5200, Type: criteriaTypeDeath, Asset: 0, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeDeath, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeDeath, 0)], deathCrit)
	achievementIndex.byAchieve[5200] = append(achievementIndex.byAchieve[5200], deathCrit)
	achievementIndex.achieveByID[5200] = achievementEntry{ID: 5200, Faction: -1}
	ownCrit := achievementCriteriaEntry{ID: 9300, AchievementID: 5300, Type: criteriaTypeOwnItem, Asset: 117, Quantity: 5}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeOwnItem, 117)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeOwnItem, 117)], ownCrit)
	achievementIndex.byAchieve[5300] = append(achievementIndex.byAchieve[5300], ownCrit)
	achievementIndex.achieveByID[5300] = achievementEntry{ID: 5300, Faction: -1}
	achievementIndex.mu.Unlock()

	state.updateAchievementCriteria(criteriaTypeDeath, 0, 1)
	if _, earned := state.earnedAchievements[5200]; !earned {
		t.Fatal("death achievement not completed")
	}
	state.updateAchievementCriteria(criteriaTypeOwnItem, 117, 3)
	state.updateAchievementCriteria(criteriaTypeOwnItem, 117, 2)
	if _, earned := state.earnedAchievements[5300]; !earned {
		t.Fatal("own-item achievement not completed at 5/5 stacks")
	}
}

func TestAchievementDamageHealAndGoldCriteria(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100, Race: 1}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	achievementIndex.mu.Lock()
	dmgCrit := achievementCriteriaEntry{ID: 9400, AchievementID: 5400, Type: criteriaTypeDamageDone, Asset: 0, Quantity: 1000}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeDamageDone, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeDamageDone, 0)], dmgCrit)
	achievementIndex.byAchieve[5400] = append(achievementIndex.byAchieve[5400], dmgCrit)
	achievementIndex.achieveByID[5400] = achievementEntry{ID: 5400, Faction: -1}
	mailCrit := achievementCriteriaEntry{ID: 9500, AchievementID: 5500, Type: criteriaTypeGoldSpentForMail, Asset: 0, Quantity: 100}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeGoldSpentForMail, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeGoldSpentForMail, 0)], mailCrit)
	achievementIndex.byAchieve[5500] = append(achievementIndex.byAchieve[5500], mailCrit)
	achievementIndex.achieveByID[5500] = achievementEntry{ID: 5500, Faction: -1}
	achievementIndex.mu.Unlock()

	state.updateAchievementCriteria(criteriaTypeDamageDone, 0, 350)
	state.updateAchievementCriteria(criteriaTypeDamageDone, 0, 650)
	if _, earned := state.earnedAchievements[5400]; !earned {
		t.Fatal("damage-done achievement not completed at 1000/1000")
	}
	state.updateAchievementCriteria(criteriaTypeGoldSpentForMail, 0, 30)
	if progress := state.criteriaProgress[9500]; progress == nil || progress.Counter != 30 {
		t.Fatalf("mail gold progress=%+v", progress)
	}
	// Nil-map sessions stay safe under the new hooks.
	fresh := &session{server: &Server{}, player: player}
	fresh.updateAchievementCriteria(criteriaTypeHealingDone, 0, 5)
	if fresh.criteriaProgress == nil {
		t.Fatal("lazy maps not initialized")
	}
}

func TestTimedAchievementLifecycle(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100, Race: 1}
	state, clientConn, db, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	achievementIndex.mu.Lock()
	timed := achievementCriteriaEntry{ID: 9600, AchievementID: 5600, Type: criteriaTypeKillCreature, Asset: 999, Quantity: 3, StartEvent: timedTypeCreature, StartAsset: 500, StartTimer: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeKillCreature, 999)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeKillCreature, 999)], timed)
	achievementIndex.byTimedEvent[typeAssetKey(timedTypeCreature, 500)] = append(achievementIndex.byTimedEvent[typeAssetKey(timedTypeCreature, 500)], timed)
	achievementIndex.byAchieve[5600] = append(achievementIndex.byAchieve[5600], timed)
	achievementIndex.achieveByID[5600] = achievementEntry{ID: 5600, Faction: -1}
	achievementIndex.mu.Unlock()

	// Killing the start creature (500) arms the timed criteria for criteria
	// driven by target creature 999.
	state.startTimedAchievement(timedTypeCreature, 500)
	if _, running := state.timedCriteria[9600]; !running {
		t.Fatal("timed criteria not armed")
	}
	if progress := state.criteriaProgress[9600]; progress == nil || progress.Counter != 0 {
		t.Fatalf("timed progress=%+v", progress)
	}
	var counter int64
	if err := db.QueryRow("SELECT counter FROM character_achievement_progress WHERE guid = 9 AND criteria = 9600").Scan(&counter); err != nil || counter != 0 {
		t.Fatalf("timed row missing: counter=%d err=%v", counter, err)
	}

	// Expiry resets progress and notifies.
	state.expireTimedAchievement(9600)
	if _, has := state.criteriaProgress[9600]; has {
		t.Fatal("progress not cleared on expiry")
	}
	if _, running := state.timedCriteria[9600]; running {
		t.Fatal("timer not cleared on expiry")
	}
	var count int64
	_ = db.QueryRow("SELECT COUNT(1) FROM character_achievement_progress WHERE guid = 9 AND criteria = 9600").Scan(&count)
	if count != 0 {
		t.Fatalf("persisted row survived expiry: %d", count)
	}

	// Completing the criterion inside the window stops the timer without reset.
	state.startTimedAchievement(timedTypeCreature, 500)
	state.criteriaProgress[9600].Counter = 3
	state.stopTimedAchievement(9600)
	if _, running := state.timedCriteria[9600]; running {
		t.Fatal("timer not stopped on completion")
	}
	if _, has := state.criteriaProgress[9600]; !has {
		t.Fatal("progress cleared by stop (only expiry should clear)")
	}
}

func TestTimedAchievementNoDoubleStart(t *testing.T) {
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	state.startTimedAchievement(timedTypeCreature, 500)
	state.startTimedAchievement(timedTypeCreature, 500)
	// Both calls target the same criteria; only one timer per criteria id.
	n := 0
	for id := range state.timedCriteria {
		if id == 9600 {
			n++
		}
	}
	if n > 1 {
		t.Fatalf("duplicate timers: %d", n)
	}
	for _, timer := range state.timedCriteria {
		timer.Stop()
	}
}
