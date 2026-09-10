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
	"strings"
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

// snapshotAchievementIndex snapshots the global criteria index and restores
// it when the test finishes, keeping index mutations test-local so later
// suites never observe synthetic criteria through shared state.
func snapshotAchievementIndex(t *testing.T) {
	t.Helper()
	achievementIndex.mu.Lock()
	typeAssetCopy := make(map[uint64][]achievementCriteriaEntry, len(achievementIndex.byTypeAsset))
	for k, v := range achievementIndex.byTypeAsset {
		typeAssetCopy[k] = append([]achievementCriteriaEntry(nil), v...)
	}
	timedCopy := make(map[uint64][]achievementCriteriaEntry, len(achievementIndex.byTimedEvent))
	for k, v := range achievementIndex.byTimedEvent {
		timedCopy[k] = append([]achievementCriteriaEntry(nil), v...)
	}
	byTypeCopy := make(map[uint32][]achievementCriteriaEntry, len(achievementIndex.byType))
	for k, v := range achievementIndex.byType {
		byTypeCopy[k] = append([]achievementCriteriaEntry(nil), v...)
	}
	exploreCopy := make(map[uint32][]uint32, len(achievementIndex.exploreByZone))
	for k, v := range achievementIndex.exploreByZone {
		exploreCopy[k] = append([]uint32(nil), v...)
	}
	byIDCopy := make(map[uint32]achievementCriteriaEntry, len(achievementIndex.byID))
	for k, v := range achievementIndex.byID {
		byIDCopy[k] = v
	}
	byAchieveCopy := make(map[uint32][]achievementCriteriaEntry, len(achievementIndex.byAchieve))
	for k, v := range achievementIndex.byAchieve {
		byAchieveCopy[k] = append([]achievementCriteriaEntry(nil), v...)
	}
	achieveByIDCopy := make(map[uint32]achievementEntry, len(achievementIndex.achieveByID))
	for k, v := range achievementIndex.achieveByID {
		achieveByIDCopy[k] = v
	}
	byConditionCopy := make(map[uint32][]achievementCriteriaEntry, len(achievementIndex.byCondition))
	for k, v := range achievementIndex.byCondition {
		byConditionCopy[k] = append([]achievementCriteriaEntry(nil), v...)
	}
	criteriaDataCopy := make(map[uint32][]criteriaDataEntry, len(achievementIndex.criteriaData))
	for k, v := range achievementIndex.criteriaData {
		criteriaDataCopy[k] = append([]criteriaDataEntry(nil), v...)
	}
	loaded := achievementIndex.loaded
	achievementIndex.mu.Unlock()
	t.Cleanup(func() {
		achievementIndex.mu.Lock()
		achievementIndex.byTypeAsset = typeAssetCopy
		achievementIndex.byTimedEvent = timedCopy
		achievementIndex.byType = byTypeCopy
		achievementIndex.byCondition = byConditionCopy
		achievementIndex.criteriaData = criteriaDataCopy
		achievementIndex.exploreByZone = exploreCopy
		achievementIndex.byID = byIDCopy
		achievementIndex.byAchieve = byAchieveCopy
		achievementIndex.achieveByID = achieveByIDCopy
		achievementIndex.loaded = loaded
		achievementIndex.mu.Unlock()
	})
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
	snapshotAchievementIndex(t)
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
	snapshotAchievementIndex(t)
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
	snapshotAchievementIndex(t)
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
	snapshotAchievementIndex(t)
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
	snapshotAchievementIndex(t)
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

func TestHonorableKillAndBGObjectiveCriteria(t *testing.T) {
	snapshotAchievementIndex(t)
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100, Race: 1, Class: 1}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	victim := &session{server: state.server, authed: true, playerLoaded: true, playerGUID: 20, player: &playerState{GUID: 20, Level: 10, Race: 2, Class: 8}}

	achievementIndex.mu.Lock()
	hk := achievementCriteriaEntry{ID: 9700, AchievementID: 5700, Type: criteriaTypeHonorableKill, Asset: 0, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeHonorableKill, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeHonorableKill, 0)], hk)
	achievementIndex.byAchieve[5700] = append(achievementIndex.byAchieve[5700], hk)
	achievementIndex.achieveByID[5700] = achievementEntry{ID: 5700, Faction: -1}
	obj := achievementCriteriaEntry{ID: 9800, AchievementID: 5800, Type: criteriaTypeBGObjective, Asset: 3, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeBGObjective, 3)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeBGObjective, 3)], obj)
	achievementIndex.byAchieve[5800] = append(achievementIndex.byAchieve[5800], obj)
	achievementIndex.achieveByID[5800] = achievementEntry{ID: 5800, Faction: -1}
	achievementIndex.mu.Unlock()

	// Duel kills do not credit honorable kills.
	state.duelPartner = 20
	state.server.creditHonorableKill(state, victim)
	if len(state.criteriaProgress) != 0 {
		t.Fatal("duel kill credited an honorable kill")
	}
	state.duelPartner = 0

	state.server.creditHonorableKill(state, victim)
	if _, earned := state.earnedAchievements[5700]; !earned {
		t.Fatal("honorable kill achievement not completed")
	}

	// BG objective credit goes to the assaulting session by GUID.
	state.server.sessions = map[*session]struct{}{state: {}}
	state.server.creditBGObjectiveCapture(9, 3)
	if _, earned := state.earnedAchievements[5800]; !earned {
		t.Fatal("BG objective achievement not completed")
	}
}

func TestExploreZoneBitsAndCriteria(t *testing.T) {
	snapshotAchievementIndex(t)
	dir := t.TempDir()
	// AreaTable: zone 12 has AreaBit 5, level gate 0. WorldMapOverlay 700 covers zones {12, 0, 0, 0}.
	writeMini := func(name string, rows ...[]uint32) {
		var body []byte
		for _, row := range rows {
			for _, v := range row {
				body = append(body, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
			}
		}
		out := []byte("WDBC")
		put := func(v uint32) { out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
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
	areaRow := make([]uint32, 11)
	areaRow[0] = 12
	areaRow[3] = 5 // AreaBit
	areaRow[10] = 0
	writeMini("AreaTable.dbc", areaRow)
	overlayRow := make([]uint32, 6)
	overlayRow[0] = 700
	overlayRow[2] = 12
	writeMini("WorldMapOverlay.dbc", overlayRow)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE characters (guid INTEGER PRIMARY KEY, exploredZones TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO characters (guid, exploredZones) VALUES (9, '')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE character_achievement_progress (guid INTEGER NOT NULL, criteria INTEGER NOT NULL, counter INTEGER NOT NULL, date INTEGER NOT NULL, PRIMARY KEY (guid, criteria))"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	server := &Server{CharactersStore: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Data: wotlk.NewStore(dir)}
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })
	player := &playerState{GUID: 9, Level: 10, Health: 100, MaxHealth: 100, Race: 1}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 9, player: player}
	state.earnedAchievements = make(map[uint32]uint32)
	state.criteriaProgress = make(map[uint32]*criteriaProgressState)
	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	server.loadAchievementIndex()
	achievementIndex.mu.Lock()
	explore := achievementCriteriaEntry{ID: 9900, AchievementID: 5900, Type: criteriaTypeExplore, Asset: 700, Quantity: 1}
	achievementIndex.byID[9900] = explore
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeExplore, 700)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeExplore, 700)], explore)
	achievementIndex.byType[criteriaTypeExplore] = append(achievementIndex.byType[criteriaTypeExplore], explore)
	achievementIndex.exploreByZone[12] = append(achievementIndex.exploreByZone[12], 9900)
	achievementIndex.byAchieve[5900] = append(achievementIndex.byAchieve[5900], explore)
	achievementIndex.achieveByID[5900] = achievementEntry{ID: 5900, Faction: -1}
	achievementIndex.mu.Unlock()

	ctx := context.Background()
	state.exploreZone(ctx, 12)
	if player.ExploredZones[0]&(1<<5) == 0 {
		t.Fatalf("area bit not set: %x", player.ExploredZones[0])
	}
	var hex string
	if err := db.QueryRow("SELECT exploredZones FROM characters WHERE guid = 9").Scan(&hex); err != nil || len(hex) != playerExploredZonesCount*8 {
		t.Fatalf("persisted blob len=%d err=%v", len(hex), err)
	}
	if _, earned := state.earnedAchievements[5900]; !earned {
		t.Fatal("explore achievement not completed")
	}

	// Second exploration of the same zone is a no-op.
	before := player.ExploredZones[0]
	state.exploreZone(ctx, 12)
	if player.ExploredZones[0] != before {
		t.Fatal("bit state changed on re-exploration")
	}

	// Round-trip load restores the bit.
	state2 := &session{server: server, playerGUID: 9, player: &playerState{GUID: 9}}
	state2.loadExploredZones(ctx)
	if state2.player.ExploredZones[0]&(1<<5) == 0 {
		t.Fatal("blob round-trip lost the area bit")
	}
}

func TestBGArenaAndDeathDetailCriteria(t *testing.T) {
	snapshotAchievementIndex(t)
	player := &playerState{GUID: 9, Level: 20, Health: 100, MaxHealth: 100, Race: 1, Map: 529}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	achievementIndex.mu.Lock()
	winCrit := achievementCriteriaEntry{ID: 10000, AchievementID: 6000, Type: criteriaTypeWinBG, Asset: 529, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeWinBG, 529)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeWinBG, 529)], winCrit)
	achievementIndex.byID[10000] = winCrit
	achievementIndex.byAchieve[6000] = append(achievementIndex.byAchieve[6000], winCrit)
	achievementIndex.achieveByID[6000] = achievementEntry{ID: 6000, Faction: -1}
	kbp := achievementCriteriaEntry{ID: 10100, AchievementID: 6100, Type: criteriaTypeKilledByPlayer, Asset: 0, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeKilledByPlayer, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeKilledByPlayer, 0)], kbp)
	achievementIndex.byID[10100] = kbp
	achievementIndex.byAchieve[6100] = append(achievementIndex.byAchieve[6100], kbp)
	achievementIndex.achieveByID[6100] = achievementEntry{ID: 6100, Faction: -1}
	achievementIndex.mu.Unlock()

	state.server.sessions = map[*session]struct{}{state: {}}
	state.server.creditBattlegroundWin(529, 0) // Alliance win on the map
	if _, earned := state.earnedAchievements[6000]; !earned {
		t.Fatal("BG win achievement not credited")
	}

	victim := &session{server: state.server, authed: true, playerLoaded: true, playerGUID: 20, player: &playerState{GUID: 20, Level: 20, Race: 2, Class: 8, Health: 1, MaxHealth: 100}}
	killer := &session{server: state.server, authed: true, playerLoaded: true, playerGUID: 9, player: player}
	state.server.creditHonorableKill(killer, victim)
	if _, earned := victim.earnedAchievements[6100]; !earned {
		t.Fatal("KILLED_BY_PLAYER not credited to the victim")
	}
}

func TestHighestStatPowerExaltedAndRatingCriteria(t *testing.T) {
	snapshotAchievementIndex(t)
	player := &playerState{GUID: 9, Level: 70, Health: 100, MaxHealth: 100, Race: 1}
	player.Stats = [5]uint32{100, 90, 120, 85, 95}
	player.MaxPowers = [7]uint32{4000, 0, 0, 0, 0, 0, 0}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)

	achievementIndex.mu.Lock()
	statCrit := achievementCriteriaEntry{ID: 10200, AchievementID: 6200, Type: criteriaTypeHighestStat, Asset: 0, Quantity: 80}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeHighestStat, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeHighestStat, 0)], statCrit)
	achievementIndex.byID[10200] = statCrit
	achievementIndex.byAchieve[6200] = append(achievementIndex.byAchieve[6200], statCrit)
	achievementIndex.achieveByID[6200] = achievementEntry{ID: 6200, Faction: -1}
	powCrit := achievementCriteriaEntry{ID: 10300, AchievementID: 6300, Type: criteriaTypeHighestPower, Asset: 0, Quantity: 3000}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeHighestPower, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeHighestPower, 0)], powCrit)
	achievementIndex.byID[10300] = powCrit
	achievementIndex.byAchieve[6300] = append(achievementIndex.byAchieve[6300], powCrit)
	achievementIndex.achieveByID[6300] = achievementEntry{ID: 6300, Faction: -1}
	exCrit := achievementCriteriaEntry{ID: 10400, AchievementID: 6400, Type: criteriaTypeExaltedRep, Asset: 72, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeExaltedRep, 72)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeExaltedRep, 72)], exCrit)
	achievementIndex.byID[10400] = exCrit
	achievementIndex.byAchieve[6400] = append(achievementIndex.byAchieve[6400], exCrit)
	achievementIndex.achieveByID[6400] = achievementEntry{ID: 6400, Faction: -1}
	achievementIndex.mu.Unlock()

	// Absolute stats: strength 100 over the 80 threshold completes.
	state.setAchievementCriteria(criteriaTypeHighestStat, 0, 100)
	if _, earned := state.earnedAchievements[6200]; !earned {
		t.Fatal("highest-stat achievement not completed at 100 >= 80")
	}
	// Lower values never regress and do not re-fire.
	state.setAchievementCriteria(criteriaTypeHighestStat, 0, 60)
	if state.criteriaProgress[10200].Counter != 100 {
		t.Fatal("highest-stat regressed")
	}
	// Power: 4000 mana over 3000 threshold.
	state.setAchievementCriteria(criteriaTypeHighestPower, 0, 4000)
	if _, earned := state.earnedAchievements[6300]; !earned {
		t.Fatal("highest-power achievement not completed")
	}
	// Exalted: 42000+ standing fires once per faction.
	state.updateAchievementCriteria(criteriaTypeExaltedRep, 72, 1)
	if _, earned := state.earnedAchievements[6400]; !earned {
		t.Fatal("exalted achievement not completed")
	}
}

func TestSpellTargetEmoteAndUseCriteria(t *testing.T) {
	player := &playerState{GUID: 9, Level: 20, Health: 100, MaxHealth: 100, Race: 1}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	target := achievementCriteriaEntry{ID: 10500, AchievementID: 6500, Type: criteriaTypeBeSpellTarget, Asset: 0, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeBeSpellTarget, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeBeSpellTarget, 0)], target)
	achievementIndex.byID[10500] = target
	achievementIndex.byAchieve[6500] = append(achievementIndex.byAchieve[6500], target)
	achievementIndex.achieveByID[6500] = achievementEntry{ID: 6500, Faction: -1}
	emote := achievementCriteriaEntry{ID: 10600, AchievementID: 6600, Type: criteriaTypeDoEmote, Asset: 17, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeDoEmote, 17)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeDoEmote, 17)], emote)
	achievementIndex.byID[10600] = emote
	achievementIndex.byAchieve[6600] = append(achievementIndex.byAchieve[6600], emote)
	achievementIndex.achieveByID[6600] = achievementEntry{ID: 6600, Faction: -1}
	achievementIndex.mu.Unlock()

	state.updateAchievementCriteria(criteriaTypeBeSpellTarget, 0, 1)
	if _, earned := state.earnedAchievements[6500]; !earned {
		t.Fatal("BE_SPELL_TARGET not completed")
	}
	state.updateAchievementCriteria(criteriaTypeDoEmote, 17, 1)
	if _, earned := state.earnedAchievements[6600]; !earned {
		t.Fatal("DO_EMOTE not completed")
	}
	// Unmatched emote does nothing.
	state.updateAchievementCriteria(criteriaTypeDoEmote, 999, 1)
	if len(state.criteriaProgress) != 2 {
		t.Fatalf("unexpected criteria progress: %+v", state.criteriaProgress)
	}
}

func TestEquipAndRollCriteria(t *testing.T) {
	player := &playerState{GUID: 9, Level: 20, Health: 100, MaxHealth: 100, Race: 1}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	equip := achievementCriteriaEntry{ID: 10700, AchievementID: 6700, Type: criteriaTypeEquipItem, Asset: 19019, Quantity: 1}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeEquipItem, 19019)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeEquipItem, 19019)], equip)
	achievementIndex.byID[10700] = equip
	achievementIndex.byAchieve[6700] = append(achievementIndex.byAchieve[6700], equip)
	achievementIndex.achieveByID[6700] = achievementEntry{ID: 6700, Faction: -1}
	need := achievementCriteriaEntry{ID: 10800, AchievementID: 6800, Type: criteriaTypeRollNeed, Asset: 0, Quantity: 90}
	achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeRollNeed, 0)] = append(achievementIndex.byTypeAsset[typeAssetKey(criteriaTypeRollNeed, 0)], need)
	achievementIndex.byID[10800] = need
	achievementIndex.byAchieve[6800] = append(achievementIndex.byAchieve[6800], need)
	achievementIndex.achieveByID[6800] = achievementEntry{ID: 6800, Faction: -1}
	achievementIndex.mu.Unlock()

	state.updateAchievementCriteria(criteriaTypeEquipItem, 19019, 1)
	if _, earned := state.earnedAchievements[6700]; !earned {
		t.Fatal("EQUIP_ITEM not completed")
	}
	// Roll winner semantics: set-variant with the roll value; 95 >= 90 completes.
	state.setAchievementCriteria(criteriaTypeRollNeed, 0, 95)
	if _, earned := state.earnedAchievements[6800]; !earned {
		t.Fatal("ROLL_NEED_ON_LOOT not completed at 95 >= 90")
	}
	// Lower rolls never regress the highest roll tracked.
	state.setAchievementCriteria(criteriaTypeRollNeed, 0, 50)
	if state.criteriaProgress[10800].Counter != 95 {
		t.Fatal("roll progress regressed")
	}
}

func TestExtendedCriteriaTypes(t *testing.T) {
	player := &playerState{GUID: 9, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1, Money: 50000}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	entries := []achievementCriteriaEntry{
		{ID: 11001, AchievementID: 7001, Type: criteriaTypeWinDuel, Asset: 0, Quantity: 1},
		{ID: 11002, AchievementID: 7002, Type: criteriaTypeLoseDuel, Asset: 0, Quantity: 1},
		{ID: 11003, AchievementID: 7003, Type: criteriaTypeCreateAuction, Asset: 0, Quantity: 1},
		{ID: 11004, AchievementID: 7004, Type: criteriaTypeHighestAuctionBid, Asset: 0, Quantity: 1000},
		{ID: 11005, AchievementID: 7005, Type: criteriaTypeWonAuctions, Asset: 0, Quantity: 1},
		{ID: 11006, AchievementID: 7006, Type: criteriaTypeHighestHealth, Asset: 0, Quantity: 1500},
		{ID: 11007, AchievementID: 7007, Type: criteriaTypeHighestArmor, Asset: 0, Quantity: 2000},
		{ID: 11008, AchievementID: 7008, Type: criteriaTypeHighestHitDealt, Asset: 0, Quantity: 500},
		{ID: 11009, AchievementID: 7009, Type: criteriaTypeHighestHitReceived, Asset: 0, Quantity: 400},
		{ID: 11010, AchievementID: 7010, Type: criteriaTypeTotalDamageReceived, Asset: 0, Quantity: 1000},
		{ID: 11011, AchievementID: 7011, Type: criteriaTypeHighestHealCasted, Asset: 0, Quantity: 800},
		{ID: 11012, AchievementID: 7012, Type: criteriaTypeTotalHealingReceived, Asset: 0, Quantity: 1200},
		{ID: 11013, AchievementID: 7013, Type: criteriaTypeHighestHealingRecv, Asset: 0, Quantity: 600},
		{ID: 11014, AchievementID: 7014, Type: criteriaTypeQuestAbandoned, Asset: 0, Quantity: 1},
		{ID: 11015, AchievementID: 7015, Type: criteriaTypeFlightPathsTaken, Asset: 0, Quantity: 1},
		{ID: 11016, AchievementID: 7016, Type: criteriaTypeHonoredRep, Asset: 0, Quantity: 1},
		{ID: 11017, AchievementID: 7017, Type: criteriaTypeReveredRep, Asset: 0, Quantity: 1},
	}
	for _, entry := range entries {
		key := typeAssetKey(entry.Type, entry.Asset)
		achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], entry)
		achievementIndex.byID[entry.ID] = entry
		achievementIndex.byAchieve[entry.AchievementID] = append(achievementIndex.byAchieve[entry.AchievementID], entry)
		achievementIndex.achieveByID[entry.AchievementID] = achievementEntry{ID: entry.AchievementID, Faction: -1}
	}
	achievementIndex.mu.Unlock()

	// 1. Duel criteria
	state.updateAchievementCriteria(criteriaTypeWinDuel, 0, 1)
	if _, earned := state.earnedAchievements[7001]; !earned {
		t.Fatal("WIN_DUEL not completed")
	}
	state.updateAchievementCriteria(criteriaTypeLoseDuel, 0, 1)
	if _, earned := state.earnedAchievements[7002]; !earned {
		t.Fatal("LOSE_DUEL not completed")
	}

	// 2. Auction criteria
	state.updateAchievementCriteria(criteriaTypeCreateAuction, 0, 1)
	if _, earned := state.earnedAchievements[7003]; !earned {
		t.Fatal("CREATE_AUCTION not completed")
	}
	state.setAchievementCriteria(criteriaTypeHighestAuctionBid, 0, 1500)
	if _, earned := state.earnedAchievements[7004]; !earned {
		t.Fatal("HIGHEST_AUCTION_BID not completed")
	}
	state.updateAchievementCriteria(criteriaTypeWonAuctions, 0, 1)
	if _, earned := state.earnedAchievements[7005]; !earned {
		t.Fatal("WON_AUCTIONS not completed")
	}

	// 3. Stat criteria (health & armor)
	state.setAchievementCriteria(criteriaTypeHighestHealth, 0, 2000)
	if _, earned := state.earnedAchievements[7006]; !earned {
		t.Fatal("HIGHEST_HEALTH not completed")
	}
	state.setAchievementCriteria(criteriaTypeHighestArmor, 0, 2500)
	if _, earned := state.earnedAchievements[7007]; !earned {
		t.Fatal("HIGHEST_ARMOR not completed")
	}

	// 4. Combat damage & healing extremes
	state.setAchievementCriteria(criteriaTypeHighestHitDealt, 0, 600)
	if _, earned := state.earnedAchievements[7008]; !earned {
		t.Fatal("HIGHEST_HIT_DEALT not completed")
	}
	state.setAchievementCriteria(criteriaTypeHighestHitReceived, 0, 450)
	if _, earned := state.earnedAchievements[7009]; !earned {
		t.Fatal("HIGHEST_HIT_RECEIVED not completed")
	}
	state.updateAchievementCriteria(criteriaTypeTotalDamageReceived, 0, 1000)
	if _, earned := state.earnedAchievements[7010]; !earned {
		t.Fatal("TOTAL_DAMAGE_RECEIVED not completed")
	}
	state.setAchievementCriteria(criteriaTypeHighestHealCasted, 0, 900)
	if _, earned := state.earnedAchievements[7011]; !earned {
		t.Fatal("HIGHEST_HEAL_CASTED not completed")
	}
	state.updateAchievementCriteria(criteriaTypeTotalHealingReceived, 0, 1200)
	if _, earned := state.earnedAchievements[7012]; !earned {
		t.Fatal("TOTAL_HEALING_RECEIVED not completed")
	}
	state.setAchievementCriteria(criteriaTypeHighestHealingRecv, 0, 700)
	if _, earned := state.earnedAchievements[7013]; !earned {
		t.Fatal("HIGHEST_HEALING_RECEIVED not completed")
	}

	// 5. Quest abandon & flight paths
	state.updateAchievementCriteria(criteriaTypeQuestAbandoned, 0, 1)
	if _, earned := state.earnedAchievements[7014]; !earned {
		t.Fatal("QUEST_ABANDONED not completed")
	}
	state.updateAchievementCriteria(criteriaTypeFlightPathsTaken, 0, 1)
	if _, earned := state.earnedAchievements[7015]; !earned {
		t.Fatal("FLIGHT_PATHS_TAKEN not completed")
	}

	// 6. Reputation standings
	state.updateAchievementCriteria(criteriaTypeHonoredRep, 0, 1)
	if _, earned := state.earnedAchievements[7016]; !earned {
		t.Fatal("HONORED_REPUTATION not completed")
	}
	state.updateAchievementCriteria(criteriaTypeReveredRep, 0, 1)
	if _, earned := state.earnedAchievements[7017]; !earned {
		t.Fatal("REVERED_REPUTATION not completed")
	}
}

func TestAll124CriteriaTypesCoverage(t *testing.T) {
	if CriteriaTypeCount != 124 {
		t.Fatalf("expected CriteriaTypeCount == 124, got %d", CriteriaTypeCount)
	}

	// Verify all 124 criteria types have valid canonical names
	for i := uint32(0); i < CriteriaTypeCount; i++ {
		name := CriteriaTypeName(i)
		if name == "" || strings.HasPrefix(name, "UnknownCriteriaType") {
			t.Fatalf("criteria type %d has invalid or missing name: %q", i, name)
		}
	}

	// Verify out-of-range returns unknown
	if name := CriteriaTypeName(999); !strings.HasPrefix(name, "UnknownCriteriaType") {
		t.Fatalf("expected UnknownCriteriaType for 999, got %q", name)
	}

	player := &playerState{GUID: 42, Level: 80, Health: 5000, MaxHealth: 5000, Race: 1}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	// Register a criteria and achievement for every single type 0..123
	achievementIndex.mu.Lock()
	for i := uint32(0); i < CriteriaTypeCount; i++ {
		critID := 20000 + i
		achID := 8000 + i
		entry := achievementCriteriaEntry{
			ID:            critID,
			AchievementID: achID,
			Type:          i,
			Asset:         0,
			Quantity:      1,
		}
		key := typeAssetKey(entry.Type, entry.Asset)
		achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], entry)
		achievementIndex.byID[critID] = entry
		achievementIndex.byAchieve[achID] = append(achievementIndex.byAchieve[achID], entry)
		achievementIndex.achieveByID[achID] = achievementEntry{ID: achID, Faction: -1}
	}
	achievementIndex.mu.Unlock()

	// Trigger progress on all 124 criteria types and verify all 124 achievements complete
	for i := uint32(0); i < CriteriaTypeCount; i++ {
		achID := 8000 + i
		state.updateAchievementCriteria(i, 0, 1)
		if _, earned := state.earnedAchievements[achID]; !earned {
			t.Fatalf("criteria type %d (%s) failed to complete achievement %d", i, CriteriaTypeName(i), achID)
		}
	}
}

func TestCriteriaWildcardAssetMatching(t *testing.T) {
	player := &playerState{GUID: 43, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	// Specific criterion: Kill Hogger (448)
	specific := achievementCriteriaEntry{ID: 30001, AchievementID: 9001, Type: criteriaTypeKillCreature, Asset: 448, Quantity: 1}
	// General criterion: Kill any creature (asset 0)
	general := achievementCriteriaEntry{ID: 30002, AchievementID: 9002, Type: criteriaTypeKillCreature, Asset: 0, Quantity: 1}

	keySpecific := typeAssetKey(specific.Type, specific.Asset)
	keyGeneral := typeAssetKey(general.Type, general.Asset)

	achievementIndex.byTypeAsset[keySpecific] = append(achievementIndex.byTypeAsset[keySpecific], specific)
	achievementIndex.byTypeAsset[keyGeneral] = append(achievementIndex.byTypeAsset[keyGeneral], general)
	achievementIndex.byID[30001] = specific
	achievementIndex.byID[30002] = general
	achievementIndex.byAchieve[9001] = append(achievementIndex.byAchieve[9001], specific)
	achievementIndex.byAchieve[9002] = append(achievementIndex.byAchieve[9002], general)
	achievementIndex.achieveByID[9001] = achievementEntry{ID: 9001, Faction: -1}
	achievementIndex.achieveByID[9002] = achievementEntry{ID: 9002, Faction: -1}
	achievementIndex.mu.Unlock()

	// Update with specific asset 448
	state.updateAchievementCriteria(criteriaTypeKillCreature, 448, 1)

	// Both specific and general should be earned!
	if _, earned := state.earnedAchievements[9001]; !earned {
		t.Fatal("expected specific achievement 9001 earned")
	}
	if _, earned := state.earnedAchievements[9002]; !earned {
		t.Fatal("expected wildcard general achievement 9002 earned")
	}
}

func TestAdditionalRequirementsConditions(t *testing.T) {
	player := &playerState{GUID: 44, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1, Map: 0}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	// Criterion 31001: requires BG map 529 (Arathi Basin)
	bgCrit := achievementCriteriaEntry{
		ID:            31001,
		AchievementID: 9101,
		Type:          criteriaTypeKillCreature,
		Asset:         100,
		Quantity:      1,
		ReqType1:      criteriaConditionBGMap,
		ReqAsset1:     529,
	}
	// Criterion 31002: requires player not to be in a group
	soloCrit := achievementCriteriaEntry{
		ID:            31002,
		AchievementID: 9102,
		Type:          criteriaTypeKillCreature,
		Asset:         200,
		Quantity:      1,
		ReqType1:      criteriaConditionNotInGroup,
	}

	keyBG := typeAssetKey(bgCrit.Type, bgCrit.Asset)
	keySolo := typeAssetKey(soloCrit.Type, soloCrit.Asset)
	achievementIndex.byTypeAsset[keyBG] = append(achievementIndex.byTypeAsset[keyBG], bgCrit)
	achievementIndex.byTypeAsset[keySolo] = append(achievementIndex.byTypeAsset[keySolo], soloCrit)
	achievementIndex.byID[31001] = bgCrit
	achievementIndex.byID[31002] = soloCrit
	achievementIndex.byAchieve[9101] = append(achievementIndex.byAchieve[9101], bgCrit)
	achievementIndex.byAchieve[9102] = append(achievementIndex.byAchieve[9102], soloCrit)
	achievementIndex.achieveByID[9101] = achievementEntry{ID: 9101, Faction: -1}
	achievementIndex.achieveByID[9102] = achievementEntry{ID: 9102, Faction: -1}
	achievementIndex.mu.Unlock()

	// 1. BG Map condition: player is on map 0, kill should not award progress
	state.updateAchievementCriteria(criteriaTypeKillCreature, 100, 1)
	if state.criteriaProgress[31001] != nil {
		t.Fatal("expected no progress on map 0 for criteria requiring map 529")
	}

	// Move player to map 529: kill now awards progress
	state.player.Map = 529
	state.updateAchievementCriteria(criteriaTypeKillCreature, 100, 1)
	if state.criteriaProgress[31001] == nil || state.criteriaProgress[31001].Counter != 1 {
		t.Fatalf("expected progress on map 529, got %+v", state.criteriaProgress[31001])
	}
	if _, earned := state.earnedAchievements[9101]; !earned {
		t.Fatal("expected achievement 9101 earned on map 529")
	}

	// 2. Not in group condition: player is in group 99, kill should not award progress
	state.groupID = 99
	state.updateAchievementCriteria(criteriaTypeKillCreature, 200, 1)
	if state.criteriaProgress[31002] != nil {
		t.Fatal("expected no progress while in group for solo criteria")
	}

	// Leave group: kill now awards progress
	state.groupID = 0
	state.updateAchievementCriteria(criteriaTypeKillCreature, 200, 1)
	if state.criteriaProgress[31002] == nil || state.criteriaProgress[31002].Counter != 1 {
		t.Fatalf("expected progress after leaving group, got %+v", state.criteriaProgress[31002])
	}
	if _, earned := state.earnedAchievements[9102]; !earned {
		t.Fatal("expected achievement 9102 earned when not in group")
	}
}

func TestAchievementCriteriaDataRules(t *testing.T) {
	player := &playerState{
		GUID:              45,
		Level:             80,
		Health:            1000,
		MaxHealth:         1000,
		Race:              1,   // Human
		Class:             1,   // Warrior
		Map:               571, // Northrend
		Zone:              1519,
		DungeonDifficulty: 2, // Heroic
		DrunkenState:      2,
		ChosenTitle:       15,
	}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	// Set active aura for aura rule
	state.activeAuras = map[uint32]*activeAura{
		12345: {SpellID: 12345, Stopped: false},
	}

	achievementIndex.mu.Lock()
	crit := achievementCriteriaEntry{
		ID:            32001,
		AchievementID: 9201,
		Type:          criteriaTypeKillCreature,
		Asset:         500,
		Quantity:      1,
	}
	key := typeAssetKey(crit.Type, crit.Asset)
	achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], crit)
	achievementIndex.byID[32001] = crit
	achievementIndex.byAchieve[9201] = append(achievementIndex.byAchieve[9201], crit)
	achievementIndex.achieveByID[9201] = achievementEntry{ID: 9201, Faction: -1}

	// Add multi-rule set matching player's current state
	achievementIndex.criteriaData[32001] = []criteriaDataEntry{
		{Type: criteriaDataTypeMapID, Value1: 571},
		{Type: criteriaDataTypeSArea, Value1: 1519},
		{Type: criteriaDataTypeSPlayerClassRace, Value1: 1, Value2: 1},
		{Type: criteriaDataTypeSKnownTitle, Value1: 15},
		{Type: criteriaDataTypeSAura, Value1: 12345},
		{Type: criteriaDataTypeMapDifficulty, Value1: 2},
		{Type: criteriaDataTypeSDrunk, Value1: 1},
	}
	achievementIndex.mu.Unlock()

	// Meets all requirements -> awards progress and completes
	state.updateAchievementCriteria(criteriaTypeKillCreature, 500, 1)
	if _, earned := state.earnedAchievements[9201]; !earned {
		t.Fatal("expected achievement 9201 earned with all criteriaData rules matching")
	}

	// Now test failing one rule: wrong map
	delete(state.earnedAchievements, 9201)
	delete(state.criteriaProgress, 32001)
	state.player.Map = 0
	state.updateAchievementCriteria(criteriaTypeKillCreature, 500, 1)
	if state.criteriaProgress[32001] != nil {
		t.Fatal("expected no progress when criteriaData map rule fails")
	}
}

func TestResetAchievementCriteriaOnDeath(t *testing.T) {
	player := &playerState{GUID: 46, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1}
	state, clientConn, db, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	crit := achievementCriteriaEntry{
		ID:            33001,
		AchievementID: 9301,
		Type:          criteriaTypeKillCreature,
		Asset:         600,
		Quantity:      2, // Needs 2 kills
		ReqType1:      criteriaConditionNoDeath,
	}
	key := typeAssetKey(crit.Type, crit.Asset)
	achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], crit)
	achievementIndex.byCondition[criteriaConditionNoDeath] = append(achievementIndex.byCondition[criteriaConditionNoDeath], crit)
	achievementIndex.byID[33001] = crit
	achievementIndex.byAchieve[9301] = append(achievementIndex.byAchieve[9301], crit)
	achievementIndex.achieveByID[9301] = achievementEntry{ID: 9301, Faction: -1}
	achievementIndex.mu.Unlock()

	// 1st kill: progress 1/2
	state.updateAchievementCriteria(criteriaTypeKillCreature, 600, 1)
	if state.criteriaProgress[33001] == nil || state.criteriaProgress[33001].Counter != 1 {
		t.Fatalf("expected progress 1, got %+v", state.criteriaProgress[33001])
	}

	// Player dies
	state.player.Health = 0
	state.killPlayer(context.Background())

	// Progress must be reset to nil
	if state.criteriaProgress[33001] != nil {
		t.Fatalf("expected progress reset on death, got %+v", state.criteriaProgress[33001])
	}

	// Database row must be deleted
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM character_achievement_progress WHERE guid = 46 AND criteria = 33001").Scan(&count)
	if count != 0 {
		t.Fatalf("expected DB progress deleted on death, got count %d", count)
	}

	// Verify removeCriteriaProgress directly sends SMSG_CRITERIA_DELETED
	serverConnDirect, clientConnDirect := net.Pipe()
	t.Cleanup(func() { clientConnDirect.Close() })
	directSess := &session{conn: serverConnDirect, authed: true, playerLoaded: true, playerGUID: 46}
	directSess.criteriaProgress = map[uint32]*criteriaProgressState{
		33001: {CriteriaID: 33001, Counter: 1},
	}
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		op, payload, err := readServerFrame(clientConnDirect, nil)
		if err != nil {
			t.Errorf("readServerFrame err=%v", err)
			return
		}
		if op != uint16(protocol.OpcodeSMSG_CRITERIA_DELETED) {
			t.Errorf("expected SMSG_CRITERIA_DELETED (0x49E), got 0x%X", op)
			return
		}
		r := protocol.NewReader(payload)
		cid, _ := r.ReadU32()
		if cid != 33001 {
			t.Errorf("expected criteria ID 33001, got %d", cid)
		}
	}()
	directSess.removeCriteriaProgress(33001)
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SMSG_CRITERIA_DELETED packet")
	}
}

func TestResetAchievementCriteriaOnArenaLoss(t *testing.T) {
	playerW := &playerState{GUID: 47, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1, Map: ArenaMapNagrand}
	sessW, clientW, _, server := newAchievementTestSession(t, playerW)
	sessW.playerGUID = 47
	drainServerFrames(t, clientW)

	playerL := &playerState{GUID: 48, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1, Map: ArenaMapNagrand}
	serverConnL, clientL := net.Pipe()
	t.Cleanup(func() { clientL.Close() })
	sessL := &session{server: server, conn: serverConnL, authed: true, playerLoaded: true, playerGUID: 48, player: playerL}
	sessL.earnedAchievements = make(map[uint32]uint32)
	sessL.criteriaProgress = make(map[uint32]*criteriaProgressState)
	drainServerFrames(t, clientL)

	server.sessionsMu.Lock()
	if server.sessions == nil {
		server.sessions = make(map[*session]struct{})
	}
	server.sessions[sessW] = struct{}{}
	server.sessions[sessL] = struct{}{}
	server.sessionsMu.Unlock()
	t.Cleanup(func() {
		server.sessionsMu.Lock()
		delete(server.sessions, sessW)
		delete(server.sessions, sessL)
		server.sessionsMu.Unlock()
	})

	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	crit := achievementCriteriaEntry{
		ID:            34001,
		AchievementID: 9401,
		Type:          criteriaTypeWinArena,
		Asset:         ArenaMapNagrand,
		Quantity:      10, // 10 wins without losing
		ReqType1:      criteriaConditionNoLose,
	}
	key := typeAssetKey(crit.Type, crit.Asset)
	achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], crit)
	achievementIndex.byCondition[criteriaConditionNoLose] = append(achievementIndex.byCondition[criteriaConditionNoLose], crit)
	achievementIndex.byID[34001] = crit
	achievementIndex.byAchieve[9401] = append(achievementIndex.byAchieve[9401], crit)
	achievementIndex.achieveByID[9401] = achievementEntry{ID: 9401, Faction: -1}
	achievementIndex.mu.Unlock()

	// Both players start with 5 wins progress
	sessW.criteriaProgress[34001] = &criteriaProgressState{CriteriaID: 34001, Counter: 5}
	sessL.criteriaProgress[34001] = &criteriaProgressState{CriteriaID: 34001, Counter: 5}

	// Arena ends: sessW wins, sessL loses
	winners := map[uint64]struct{}{47: {}}
	server.creditArenaParticipants(ArenaMapNagrand, nil, winners)

	// Winner has 6 wins now
	if sessW.criteriaProgress[34001] == nil || sessW.criteriaProgress[34001].Counter != 6 {
		t.Fatalf("expected winner progress 6, got %+v", sessW.criteriaProgress[34001])
	}

	// Loser progress was reset to nil
	if sessL.criteriaProgress[34001] != nil {
		t.Fatalf("expected loser progress reset to nil, got %+v", sessL.criteriaProgress[34001])
	}
}

func TestResetAchievementCriteriaOnBGLeave(t *testing.T) {
	player := &playerState{GUID: 49, Level: 80, Health: 1000, MaxHealth: 1000, Race: 1, Map: 529}
	state, clientConn, _, _ := newAchievementTestSession(t, player)
	drainServerFrames(t, clientConn)
	snapshotAchievementIndex(t)

	achievementIndex.mu.Lock()
	crit := achievementCriteriaEntry{
		ID:            35001,
		AchievementID: 9501,
		Type:          criteriaTypeBGObjective,
		Asset:         1,
		Quantity:      5,
		ReqType1:      criteriaConditionBGMap,
		ReqAsset1:     529,
	}
	key := typeAssetKey(crit.Type, crit.Asset)
	achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], crit)
	achievementIndex.byCondition[criteriaConditionBGMap] = append(achievementIndex.byCondition[criteriaConditionBGMap], crit)
	achievementIndex.byID[35001] = crit
	achievementIndex.byAchieve[9501] = append(achievementIndex.byAchieve[9501], crit)
	achievementIndex.achieveByID[9501] = achievementEntry{ID: 9501, Faction: -1}
	achievementIndex.mu.Unlock()

	// Progress 2/5 on map 529
	state.criteriaProgress[35001] = &criteriaProgressState{CriteriaID: 35001, Counter: 2}

	// Player leaves battlefield
	state.handleLeaveBattlefield(context.Background(), nil)

	// Progress for map 529 criteria must be reset
	if state.criteriaProgress[35001] != nil {
		t.Fatalf("expected progress reset on BG leave, got %+v", state.criteriaProgress[35001])
	}
}




