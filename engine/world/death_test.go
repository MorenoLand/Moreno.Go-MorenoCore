package world

import (
	"context"
	"database/sql"
	"encoding/binary"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func newDeathTestSession(t *testing.T, player *playerState) (*session, net.Conn, *Server) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, death_expire_time INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE corpse (guid INTEGER NOT NULL, posX REAL NOT NULL, posY REAL NOT NULL, posZ REAL NOT NULL, orientation REAL NOT NULL, mapId INTEGER NOT NULL, displayId INTEGER NOT NULL DEFAULT 0, itemCache TEXT NOT NULL, bytes1 INTEGER NOT NULL DEFAULT 0, bytes2 INTEGER NOT NULL DEFAULT 0, guildId INTEGER NOT NULL DEFAULT 0, flags INTEGER NOT NULL DEFAULT 0, dynFlags INTEGER NOT NULL DEFAULT 0, time INTEGER NOT NULL DEFAULT 0, corpseType INTEGER NOT NULL DEFAULT 0, instanceId INTEGER NOT NULL DEFAULT 0, phaseMask INTEGER NOT NULL DEFAULT 1)",
		"CREATE TABLE graveyard_zone (ID INTEGER NOT NULL, GhostZone INTEGER NOT NULL, Faction INTEGER NOT NULL)",
		"CREATE TABLE IF NOT EXISTS creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL DEFAULT 0, map INTEGER NOT NULL DEFAULT 0, zoneId INTEGER NOT NULL DEFAULT 0, areaId INTEGER NOT NULL DEFAULT 0, position_x REAL NOT NULL DEFAULT 0, position_y REAL NOT NULL DEFAULT 0, position_z REAL NOT NULL DEFAULT 0, orientation REAL NOT NULL DEFAULT 0, curhealth INTEGER NOT NULL DEFAULT 1, curmana INTEGER NOT NULL DEFAULT 0, npcflag INTEGER NOT NULL DEFAULT 0, unit_flags INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE IF NOT EXISTS creature_template (entry INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', npcflag INTEGER NOT NULL DEFAULT 0, unit_flags INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE IF NOT EXISTS character_homebind (guid INTEGER PRIMARY KEY, mapId INTEGER NOT NULL DEFAULT 0, zoneId INTEGER NOT NULL DEFAULT 0, posX REAL NOT NULL DEFAULT 0, posY REAL NOT NULL DEFAULT 0, posZ REAL NOT NULL DEFAULT 0)",
		"CREATE TABLE IF NOT EXISTS character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE IF NOT EXISTS item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, durability INTEGER)",
		"CREATE TABLE IF NOT EXISTS item_template (entry INTEGER PRIMARY KEY, MaxDurability INTEGER)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	characters := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	worldStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	server := &Server{CharactersStore: characters, WorldStore: worldStore, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RealmID: 1, Config: config.Default()}
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	state := &session{server: server, conn: serverConn, authed: true, accountID: 7, accountName: "test", playerLoaded: player != nil, playerGUID: 9, player: player}
	return state, clientConn, server
}

// writeWorldSafeLocsDBC writes a minimal WorldSafeLocs.dbc with the given
// records using the reference layout "nifffxxxxxxxxxxxxxxxxx".
func writeWorldSafeLocsDBC(t *testing.T, dir string, records [][5]uint32) {
	t.Helper()
	const fieldCount = 22
	recordBytes := make([]byte, 0, len(records)*fieldCount*4)
	for _, record := range records {
		for _, value := range record {
			var encoded [4]byte
			binary.LittleEndian.PutUint32(encoded[:], value)
			recordBytes = append(recordBytes, encoded[:]...)
		}
		for i := 5; i < fieldCount; i++ {
			recordBytes = append(recordBytes, 0, 0, 0, 0)
		}
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(records)))
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1) // string block size
	if err := os.WriteFile(filepath.Join(dir, "WorldSafeLocs.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSpellDBC writes a minimal Spell.dbc containing one spell with the given effect.
func writeSpellDBC(t *testing.T, dir string, id, effect uint32, basePoints, miscValue int32) {
	t.Helper()
	const fieldCount = 234
	record := make([]uint32, fieldCount)
	record[0] = id
	record[71] = effect
	record[80] = uint32(basePoints)
	record[110] = uint32(miscValue)
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
	if err := os.WriteFile(filepath.Join(dir, "Spell.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeAreaTableDBC writes a minimal AreaTable.dbc with given records.
func writeAreaTableDBC(t *testing.T, dir string, records [][5]uint32) {
	t.Helper()
	const fieldCount = 36
	recordBytes := make([]byte, 0, len(records)*fieldCount*4)
	for _, record := range records {
		for _, value := range record {
			var encoded [4]byte
			binary.LittleEndian.PutUint32(encoded[:], value)
			recordBytes = append(recordBytes, encoded[:]...)
		}
		for i := 5; i < fieldCount; i++ {
			recordBytes = append(recordBytes, 0, 0, 0, 0)
		}
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(records)))
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dir, "AreaTable.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func floatBits(value float32) uint32 {
	return math.Float32bits(value)
}

// drainServerFrames consumes everything the session writes so synchronous
// net.Pipe writes cannot deadlock tests that do not assert packet order.
func drainServerFrames(t *testing.T, conn net.Conn) {
	t.Helper()
	go func() {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			if _, _, err := readServerFrame(conn, nil); err != nil {
				return
			}
		}
	}()
}

func TestKillPlayerStartsDeathState(t *testing.T) {
	player := &playerState{GUID: 9, Health: 0, MaxHealth: 100, Map: 0, X: 1, Y: 2, Z: 3}
	state, clientConn, _ := newDeathTestSession(t, player)
	frames := make(chan uint16, 8)
	go func() {
		if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Error(err)
		}
		for {
			opcode, _, err := readServerFrame(clientConn, nil)
			if err != nil {
				close(frames)
				return
			}
			frames <- opcode
		}
	}()
	state.killPlayer(context.Background())
	var seenRoot, seenDelay bool
	timeout := time.After(2 * time.Second)
collect:
	for {
		select {
		case opcode, ok := <-frames:
			if !ok {
				break collect
			}
			switch opcode {
			case uint16(protocol.OpcodeSMSG_FORCE_MOVE_ROOT):
				seenRoot = true
			case uint16(protocol.OpcodeSMSG_CORPSE_RECLAIM_DELAY):
				seenDelay = true
			}
			if seenRoot && seenDelay {
				break collect
			}
		case <-timeout:
			break collect
		}
	}
	if !seenRoot || !seenDelay {
		t.Fatalf("root=%v delay=%v", seenRoot, seenDelay)
	}
	if player.PlayerFieldBytes&playerFieldByteReleaseTimer == 0 {
		t.Fatalf("release timer bit not set: %x", player.PlayerFieldBytes)
	}
	if state.deathTimer.IsZero() || time.Until(state.deathTimer) > autoRepopDelay+time.Second {
		t.Fatalf("death timer=%v", state.deathTimer)
	}
	if state.deathExpireTime <= time.Now().Unix() {
		t.Fatalf("death expire time=%d", state.deathExpireTime)
	}
}

func TestCorpseReclaimDelayProgression(t *testing.T) {
	state, _, _ := newDeathTestSession(t, &playerState{GUID: 9, Health: 0})
	now := time.Now().Unix()
	state.deathExpireTime = now - 10
	if got := state.corpseReclaimDelaySeconds(false); got != 30 {
		t.Fatalf("first delay=%d", got)
	}
	state.deathExpireTime = now + deathExpireStepSeconds + 100
	if got := state.corpseReclaimDelaySeconds(false); got != 60 {
		t.Fatalf("second delay=%d", got)
	}
	state.deathExpireTime = now + 3*deathExpireStepSeconds + 100
	if got := state.corpseReclaimDelaySeconds(false); got != 120 {
		t.Fatalf("capped delay=%d", got)
	}
	state.server.Config.DeathCorpseReclaimDelayPvE = false
	if got := state.corpseReclaimDelaySeconds(false); got != 0 {
		t.Fatalf("disabled PvE delay=%d", got)
	}
	state.deathExpireTime = now - 10
	state.server.Config.DeathCorpseReclaimDelayPvP = false
	if got := state.corpseReclaimDelaySeconds(true); got != 30 {
		t.Fatalf("disabled PvP delay=%d", got)
	}
}

func TestBuildPlayerRepopCreatesCorpseAndGhost(t *testing.T) {
	player := &playerState{GUID: 9, Health: 0, MaxHealth: 100, Map: 0, X: 1.5, Y: 2.5, Z: 3.5, Orientation: 0.5, Equipment: "1 2 3", GuildID: 5, StandState: 1, SheathState: 1}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)
	state.buildPlayerRepop(context.Background())
	if player.PlayerFlags&playerFlagGhost == 0 {
		t.Fatal("ghost flag not set")
	}
	if player.Health != 1 {
		t.Fatalf("ghost health=%d", player.Health)
	}
	if player.PlayerFieldBytes&playerFieldByteReleaseTimer != 0 {
		t.Fatalf("release timer bit not cleared: %x", player.PlayerFieldBytes)
	}
	if !state.deathTimer.IsZero() {
		t.Fatal("death timer not cleared")
	}
	var posX, posY, posZ, orientation float64
	var mapID, corpseType int64
	if err := server.CharactersStore.DB.QueryRow("SELECT posX, posY, posZ, orientation, mapId, corpseType FROM corpse WHERE guid = 9").Scan(&posX, &posY, &posZ, &orientation, &mapID, &corpseType); err != nil {
		t.Fatal(err)
	}
	if posX != 1.5 || posY != 2.5 || posZ != 3.5 || orientation != 0.5 || mapID != 0 || corpseType != int64(corpseTypePvE) {
		t.Fatalf("corpse record=(%v,%v,%v,%v,%d,%d)", posX, posY, posZ, orientation, mapID, corpseType)
	}
	var bytes1, bytes2 int64
	if err := server.CharactersStore.DB.QueryRow("SELECT bytes1, bytes2 FROM corpse WHERE guid = 9").Scan(&bytes1, &bytes2); err != nil {
		t.Fatal(err)
	}
	if bytes1 != 1 || bytes2 != 256 {
		t.Fatalf("corpse bytes1=%d bytes2=%d", bytes1, bytes2)
	}
}

func TestRepopRequestGuards(t *testing.T) {
	alive := &playerState{GUID: 9, Health: 100}
	state, _, _ := newDeathTestSession(t, alive)
	payload := protocol.NewBuffer(1)
	payload.WriteU8(1)
	if !state.handleRepopRequest(context.Background(), payload.Bytes()) {
		t.Fatal("alive repop should be ignored")
	}
	if alive.PlayerFlags&playerFlagGhost != 0 {
		t.Fatal("alive player became ghost")
	}
	ghost := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost}
	state2, _, _ := newDeathTestSession(t, ghost)
	if !state2.handleRepopRequest(context.Background(), payload.Bytes()) {
		t.Fatal("ghost repop should be ignored")
	}
	if ghost.Health != 1 || ghost.PlayerFlags&playerFlagGhost == 0 {
		t.Fatal("ghost state changed")
	}
	if state2.handleRepopRequest(context.Background(), nil) {
		t.Fatal("empty repop payload should be rejected")
	}
}

func TestClosestGraveyardSelection(t *testing.T) {
	dir := t.TempDir()
	writeWorldSafeLocsDBC(t, dir, [][5]uint32{
		{1, 0, floatBits(-100), floatBits(-100), floatBits(10)}, // same map, far
		{2, 0, floatBits(10), floatBits(10), floatBits(10)},     // same map, near
		{3, 0, floatBits(20), floatBits(20), floatBits(10)},     // same map, mid, horde only
		{4, 1, floatBits(5), floatBits(5), floatBits(5)},        // other map
		{10, 1, floatBits(6), floatBits(6), floatBits(6)},       // default horde yard (Crossroads)
	})
	data := wotlk.NewStore(dir)
	state, _, server := newDeathTestSession(t, &playerState{GUID: 9, Health: 0})
	server.Data = data
	if _, err := server.WorldStore.DB.Exec("INSERT INTO graveyard_zone (ID, GhostZone, Faction) VALUES (2, 12, 0), (1, 12, 0), (3, 12, 67), (4, 12, 0)"); err != nil {
		t.Fatal(err)
	}
	_ = state
	// Alliance player at (9, 9): nearest same-map graveyard is id 2.
	loc, ok := server.closestGraveyard(context.Background(), 9, 9, 9, 0, 12, teamAlliance)
	if !ok || loc.ID != 2 {
		t.Fatalf("graveyard=%+v ok=%v", loc, ok)
	}
	// Horde player sees the horde-only yard 3 as nearest.
	loc, ok = server.closestGraveyard(context.Background(), 19, 19, 9, 0, 12, teamHorde)
	if !ok || loc.ID != 3 {
		t.Fatalf("horde graveyard=%+v ok=%v", loc, ok)
	}
	// A zone with no links falls back to the default graveyard per team.
	loc, ok = server.closestGraveyard(context.Background(), 0, 0, 0, 0, 999, teamAlliance)
	if !ok || loc.ID != defaultGraveyardAlliance {
		t.Fatalf("default graveyard=%+v ok=%v", loc, ok)
	}
	loc, ok = server.closestGraveyard(context.Background(), 0, 0, 0, 0, 999, teamHorde)
	if !ok || loc.ID != defaultGraveyardHorde {
		t.Fatalf("default horde graveyard=%+v ok=%v", loc, ok)
	}
	// A map with no same-map link falls back to the first other-map link
	// (reference keeps the first entry it encounters, order is not guaranteed).
	loc, ok = server.closestGraveyard(context.Background(), 9, 9, 9, 5, 12, teamAlliance)
	if !ok || loc.MapID == 5 {
		t.Fatalf("cross-map graveyard=%+v ok=%v", loc, ok)
	}
}

func TestHandleReclaimCorpseFlow(t *testing.T) {
	player := &playerState{GUID: 9, Health: 1, MaxHealth: 100, PlayerFlags: playerFlagGhost, Map: 0, X: 10, Y: 10, Z: 10}
	player.Powers = [7]uint32{0, 0, 0, 50, 0, 0, 0}
	player.MaxPowers = [7]uint32{100, 100, 100, 100, 0, 0, 0}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)
	payload := protocol.NewBuffer(8)
	payload.WritePackedGUID(9)
	// No corpse yet: ignored.
	if !state.handleReclaimCorpse(context.Background(), payload.Bytes()) {
		t.Fatal("reclaim without corpse should be ignored")
	}
	if _, err := server.CharactersStore.DB.Exec("INSERT INTO corpse (guid, posX, posY, posZ, orientation, mapId, itemCache, time, corpseType) VALUES (9, 10, 10, 10, 0, 0, '', ?, ?)", time.Now().Unix()-3600, corpseTypePvE); err != nil {
		t.Fatal(err)
	}
	if !state.handleReclaimCorpse(context.Background(), payload.Bytes()) {
		t.Fatal("reclaim failed")
	}
	if player.Health != 50 {
		t.Fatalf("health=%d", player.Health)
	}
	if player.Powers[0] != 50 || player.Powers[1] != 0 || player.Powers[3] != 50 {
		t.Fatalf("powers=%v", player.Powers)
	}
	if player.PlayerFlags&playerFlagGhost != 0 {
		t.Fatal("ghost flag not cleared")
	}
	var corpseType int64
	if err := server.CharactersStore.DB.QueryRow("SELECT corpseType FROM corpse WHERE guid = 9").Scan(&corpseType); err != nil {
		t.Fatal(err)
	}
	if corpseType != int64(corpseTypeBones) {
		t.Fatalf("corpse type=%d", corpseType)
	}
}

func TestHandleReclaimCorpseGuards(t *testing.T) {
	fresh := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost, Map: 0, X: 10, Y: 10, Z: 10}
	state, clientConn, server := newDeathTestSession(t, fresh)
	drainServerFrames(t, clientConn)
	payload := protocol.NewBuffer(8)
	payload.WritePackedGUID(9)
	if _, err := server.CharactersStore.DB.Exec("INSERT INTO corpse (guid, posX, posY, posZ, orientation, mapId, itemCache, time, corpseType) VALUES (9, 10, 10, 10, 0, 0, '', ?, ?)", time.Now().Unix(), corpseTypePvE); err != nil {
		t.Fatal(err)
	}
	if !state.handleReclaimCorpse(context.Background(), payload.Bytes()) {
		t.Fatal("reclaim handler failed")
	}
	if fresh.Health != 1 || fresh.PlayerFlags&playerFlagGhost == 0 {
		t.Fatal("reclaim during delay window resurrected the player")
	}
	if _, err := server.CharactersStore.DB.Exec("UPDATE corpse SET posX = 500, posY = 500, time = ?", time.Now().Unix()-3600); err != nil {
		t.Fatal(err)
	}
	if !state.handleReclaimCorpse(context.Background(), payload.Bytes()) {
		t.Fatal("reclaim handler failed")
	}
	if fresh.Health != 1 {
		t.Fatal("out of range reclaim resurrected the player")
	}
}

func TestUpdatePlayerDeathTimersAutoReleases(t *testing.T) {
	player := &playerState{GUID: 9, Health: 0, Map: 0, X: 1, Y: 2, Z: 3}
	state, clientConn, server := newDeathTestSession(t, player)
	server.addSession(state)
	defer server.removeSession(state)
	drainServerFrames(t, clientConn)
	state.deathTimer = time.Now().Add(-time.Second)
	server.updatePlayerDeathTimers(context.Background(), time.Now())
	if player.PlayerFlags&playerFlagGhost == 0 {
		t.Fatal("auto release did not convert the player to a ghost")
	}
	if player.Health != 1 {
		t.Fatalf("ghost health=%d", player.Health)
	}
	var corpseType int64
	if err := server.CharactersStore.DB.QueryRow("SELECT corpseType FROM corpse WHERE guid = 9").Scan(&corpseType); err != nil {
		t.Fatal(err)
	}
	if corpseType != int64(corpseTypePvE) {
		t.Fatalf("corpse type=%d", corpseType)
	}
}

func resurrectResponsePayload(resurrecter uint64, response uint8) []byte {
	payload := protocol.NewBuffer(9)
	payload.WriteU64(resurrecter)
	payload.WriteU8(response)
	return payload.Bytes()
}

func TestResurrectResponseAcceptFlow(t *testing.T) {
	player := &playerState{GUID: 9, Health: 1, MaxHealth: 100, PlayerFlags: playerFlagGhost, Map: 0, X: 10, Y: 10, Z: 10}
	player.Powers = [7]uint32{0, 40, 0, 20, 0, 0, 0}
	player.MaxPowers = [7]uint32{100, 100, 100, 100, 0, 0, 0}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)
	if _, err := server.CharactersStore.DB.Exec("INSERT INTO corpse (guid, posX, posY, posZ, orientation, mapId, itemCache, time, corpseType) VALUES (9, 10, 10, 10, 0, 0, '', ?, ?)", time.Now().Unix()-3600, corpseTypePvE); err != nil {
		t.Fatal(err)
	}
	state.setResurrectRequestData(77, 1, 50.5, 60.5, 70.5, 321, 222)
	if !state.handleResurrectResponse(context.Background(), resurrectResponsePayload(77, 1)) {
		t.Fatal("accepted resurrect response failed")
	}
	if state.resurrection != nil {
		t.Fatal("resurrect request not cleared")
	}
	if player.PlayerFlags&playerFlagGhost != 0 || player.Health != 321 || player.Powers[0] != 222 || player.Powers[1] != 0 || player.Powers[3] != 100 {
		t.Fatalf("player=%+v powers=%v", player, player.Powers)
	}
	if player.Map != 1 || player.X != 50.5 || player.Y != 60.5 || player.Z != 70.5 {
		t.Fatalf("player not teleported to caster location: %+v", player)
	}
	var corpseType int64
	if err := server.CharactersStore.DB.QueryRow("SELECT corpseType FROM corpse WHERE guid = 9").Scan(&corpseType); err != nil {
		t.Fatal(err)
	}
	if corpseType != int64(corpseTypeBones) {
		t.Fatalf("corpse type=%d", corpseType)
	}
}

func TestResurrectResponseRejectAndGuards(t *testing.T) {
	ghost := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost}
	state, clientConn, _ := newDeathTestSession(t, ghost)
	drainServerFrames(t, clientConn)
	state.setResurrectRequestData(77, 0, 1, 2, 3, 100, 50)
	// Reject clears the stored request without resurrecting.
	if !state.handleResurrectResponse(context.Background(), resurrectResponsePayload(77, 0)) {
		t.Fatal("rejected resurrect response failed")
	}
	if state.resurrection != nil {
		t.Fatal("rejected response did not clear the request")
	}
	// Mismatched resurrecter guid is ignored.
	state.setResurrectRequestData(77, 0, 1, 2, 3, 100, 50)
	if !state.handleResurrectResponse(context.Background(), resurrectResponsePayload(78, 1)) {
		t.Fatal("mismatched resurrecter handling failed")
	}
	if state.resurrection == nil || ghost.PlayerFlags&playerFlagGhost == 0 {
		t.Fatal("mismatched resurrecter resurrected or cleared the request")
	}
	// Alive players ignore the packet entirely.
	alive := &playerState{GUID: 9, Health: 100}
	state2, clientConn2, _ := newDeathTestSession(t, alive)
	drainServerFrames(t, clientConn2)
	state2.setResurrectRequestData(77, 0, 1, 2, 3, 100, 50)
	if !state2.handleResurrectResponse(context.Background(), resurrectResponsePayload(77, 1)) {
		t.Fatal("alive resurrect response should be ignored")
	}
	if alive.Health != 100 || alive.PlayerFlags&playerFlagGhost != 0 {
		t.Fatal("alive player was resurrected")
	}
	// Malformed payload is rejected.
	if state2.handleResurrectResponse(context.Background(), resurrectResponsePayload(77, 1)[:5]) {
		t.Fatal("truncated resurrect response should be rejected")
	}
}

func TestSetResurrectRequestDataOverwriteGuard(t *testing.T) {
	ghost := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost}
	state, _, _ := newDeathTestSession(t, ghost)
	state.setResurrectRequestData(77, 0, 1, 2, 3, 100, 50)
	state.setResurrectRequestData(78, 0, 4, 5, 6, 200, 60)
	if state.resurrection.GUID != 77 || state.resurrection.Health != 100 {
		t.Fatalf("resurrect request overwritten: %+v", state.resurrection)
	}
}

func TestSendResurrectRequestPacketLayout(t *testing.T) {
	ghost := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost}
	state, clientConn, _ := newDeathTestSession(t, ghost)
	result := make(chan bool, 1)
	go func() { result <- true; state.sendResurrectRequest(77, "Healer", true, false) }()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-result
	if opcode != uint16(protocol.OpcodeSMSG_RESURRECT_REQUEST) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil || guid != 77 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
	nameLen, err := reader.ReadU32()
	if err != nil || nameLen != 7 {
		t.Fatalf("name length=%d err=%v", nameLen, err)
	}
	name, err := reader.ReadString(int(nameLen) - 1)
	if err != nil || name != "Healer" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	sickness, err := reader.ReadU8()
	ignoreTimer, err2 := reader.ReadU8()
	if err != nil || err2 != nil || sickness != 1 || ignoreTimer != 0 {
		t.Fatalf("flags=%d/%d err=%v/%v", sickness, ignoreTimer, err, err2)
	}
}

func TestHandleSelfResFlow(t *testing.T) {
	dbcDir := t.TempDir()
	writeSpellDBC(t, dbcDir, 20608, spellEffectResurrectNew, 199, 150)
	ghost := &playerState{
		GUID:         9,
		Health:       0,
		MaxHealth:    500,
		PlayerFlags:  playerFlagGhost,
		SelfResSpell: 20608,
		Map:          1,
		X:            10.0,
		Y:            20.0,
		Z:            30.0,
	}
	state, clientConn, server := newDeathTestSession(t, ghost)
	server.Data = wotlk.NewStore(dbcDir)
	drainServerFrames(t, clientConn)

	if !state.handleSelfRes(context.Background()) {
		t.Fatal("handleSelfRes failed")
	}
	if ghost.SelfResSpell != 0 {
		t.Fatalf("SelfResSpell not cleared: %d", ghost.SelfResSpell)
	}
	if state.resurrection == nil {
		t.Fatal("resurrection request data was not created")
	}
	if state.resurrection.GUID != 9 || state.resurrection.Health != 200 || state.resurrection.Mana != 150 {
		t.Fatalf("unexpected resurrect data: %+v", state.resurrection)
	}

	// Calling again with SelfResSpell=0 is a no-op
	if !state.handleSelfRes(context.Background()) {
		t.Fatal("handleSelfRes with 0 spell failed")
	}

	// Alive player calling handleSelfRes clears the field but does not create a resurrect request
	alive := &playerState{GUID: 10, Health: 100, SelfResSpell: 20608}
	state2, clientConn2, server2 := newDeathTestSession(t, alive)
	server2.Data = wotlk.NewStore(dbcDir)
	drainServerFrames(t, clientConn2)
	if !state2.handleSelfRes(context.Background()) {
		t.Fatal("alive handleSelfRes failed")
	}
	if state2.resurrection != nil {
		t.Fatal("alive player should not receive resurrect request")
	}
	if alive.SelfResSpell != 0 {
		t.Fatalf("alive player SelfResSpell not cleared: %d", alive.SelfResSpell)
	}
}

func TestAreaSpiritHealerQueryAndQueue(t *testing.T) {
	player := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)

	// Insert a spirit healer creature and a regular non-spirit creature
	if _, err := server.WorldStore.DB.Exec("INSERT INTO creature_template (entry, name, npcflag) VALUES (1234, 'Spirit Healer', 16384), (5678, 'Vendor', 128)"); err != nil {
		t.Fatal(err)
	}
	if _, err := server.WorldStore.DB.Exec("INSERT INTO creature (guid, id, npcflag) VALUES (55, 1234, 0), (56, 5678, 0)"); err != nil {
		t.Fatal(err)
	}

	spiritGUID := uint64(55) | (uint64(1234) << 24) | (uint64(0xF130) << 48)
	vendorGUID := uint64(56) | (uint64(5678) << 24) | (uint64(0xF130) << 48)

	queryPayload := protocol.NewBuffer(8)
	queryPayload.WriteU64(spiritGUID)

	// Valid query
	if !state.handleAreaSpiritHealerQuery(context.Background(), queryPayload.Bytes()) {
		t.Fatal("valid spirit healer query failed")
	}
	// Truncated query payload
	if state.handleAreaSpiritHealerQuery(context.Background(), queryPayload.Bytes()[:4]) {
		t.Fatal("truncated query payload should fail")
	}
	// Non-spirit creature query
	vendorPayload := protocol.NewBuffer(8)
	vendorPayload.WriteU64(vendorGUID)
	if !state.handleAreaSpiritHealerQuery(context.Background(), vendorPayload.Bytes()) {
		t.Fatal("non-spirit creature query should succeed without error")
	}

	// Valid queue
	if !state.handleAreaSpiritHealerQueue(context.Background(), queryPayload.Bytes()) {
		t.Fatal("valid spirit healer queue failed")
	}
	// Truncated queue payload
	if state.handleAreaSpiritHealerQueue(context.Background(), queryPayload.Bytes()[:4]) {
		t.Fatal("truncated queue payload should fail")
	}
	// Non-spirit creature queue
	if !state.handleAreaSpiritHealerQueue(context.Background(), vendorPayload.Bytes()) {
		t.Fatal("non-spirit creature queue should succeed without error")
	}
}

func TestSendAreaSpiritHealerTimePacketLayout(t *testing.T) {
	player := &playerState{GUID: 9, Health: 1, PlayerFlags: playerFlagGhost}
	state, clientConn, _ := newDeathTestSession(t, player)
	result := make(chan bool, 1)
	go func() { result <- true; state.sendAreaSpiritHealerTime(77, 15000) }()
	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-result
	if opcode != uint16(protocol.OpcodeSMSG_AREA_SPIRIT_HEALER_TIME) {
		t.Fatalf("opcode=%x", opcode)
	}
	if len(payload) != 12 {
		t.Fatalf("expected 12 bytes, got %d", len(payload))
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil || guid != 77 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
	timeLeft, err := reader.ReadU32()
	if err != nil || timeLeft != 15000 {
		t.Fatalf("timeLeft=%d err=%v", timeLeft, err)
	}
}

func TestHandleHearthAndResurrect(t *testing.T) {
	dbcDir := t.TempDir()
	writeAreaTableDBC(t, dbcDir, [][5]uint32{{4197, 571, 0, 0, wotlk.AreaFlagWintergrasp2}})
	ghost := &playerState{
		GUID:         9,
		Health:       0,
		MaxHealth:    1000,
		PlayerFlags:  playerFlagGhost,
		Zone:         4197,
		Map:          571,
		X:            10.0,
		Y:            20.0,
		Z:            30.0,
		HomebindMap:  0,
		HomebindZone: 12,
		HomebindX:    100.5,
		HomebindY:    200.5,
		HomebindZ:    30.5,
	}
	state, clientConn, server := newDeathTestSession(t, ghost)
	server.Data = wotlk.NewStore(dbcDir)
	drainServerFrames(t, clientConn)

	if !state.handleHearthAndResurrect(context.Background()) {
		t.Fatal("handleHearthAndResurrect failed")
	}
	if ghost.PlayerFlags&playerFlagGhost != 0 {
		t.Fatal("ghost flag not cleared")
	}
	if ghost.Health != 1000 {
		t.Fatalf("health not restored to max: %d", ghost.Health)
	}
	if ghost.Map != 0 || ghost.X != 100.5 || ghost.Y != 200.5 || ghost.Z != 30.5 {
		t.Fatalf("player not teleported to homebind: %+v", ghost)
	}

	// Flying player should be ignored
	ghost2 := &playerState{GUID: 10, Health: 0, PlayerFlags: playerFlagGhost, Zone: 4197}
	state2, clientConn2, server2 := newDeathTestSession(t, ghost2)
	server2.Data = wotlk.NewStore(dbcDir)
	state2.inFlight = true
	drainServerFrames(t, clientConn2)
	if !state2.handleHearthAndResurrect(context.Background()) {
		t.Fatal("inFlight handleHearthAndResurrect failed")
	}
	if ghost2.PlayerFlags&playerFlagGhost == 0 || ghost2.Health != 0 {
		t.Fatal("inFlight player was resurrected")
	}

	// Non-Wintergrasp area should be ignored
	ghost3 := &playerState{GUID: 11, Health: 0, PlayerFlags: playerFlagGhost, Zone: 12}
	state3, clientConn3, server3 := newDeathTestSession(t, ghost3)
	server3.Data = wotlk.NewStore(dbcDir)
	drainServerFrames(t, clientConn3)
	if !state3.handleHearthAndResurrect(context.Background()) {
		t.Fatal("non-wintergrasp handleHearthAndResurrect failed")
	}
	if ghost3.PlayerFlags&playerFlagGhost == 0 || ghost3.Health != 0 {
		t.Fatal("non-wintergrasp player was resurrected")
	}
}

func TestDurabilityLossOnDeath(t *testing.T) {
	player := &playerState{GUID: 9, Health: 0, MaxHealth: 100}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)

	cdb := server.CharactersStore.DB

	// Insert item templates:
	// Entry 100: MaxDurability = 50
	// Entry 101: MaxDurability = 100
	_, _ = cdb.Exec("INSERT INTO item_template (entry, MaxDurability) VALUES (100, 50), (101, 100)")

	// Insert item instances:
	// Item 1001: entry 100, curDur = 50
	// Item 1002: entry 101, curDur = 100
	_, _ = cdb.Exec("INSERT INTO item_instance (guid, itemEntry, durability) VALUES (1001, 100, 50), (1002, 101, 100)")

	// Item 1001 is equipped (bag 0, slot 0)
	// Item 1002 is in backpack (bag 0, slot 23)
	_, _ = cdb.Exec("INSERT INTO character_inventory (guid, bag, slot, item) VALUES (9, 0, 0, 1001), (9, 0, 23, 1002)")

	ctx := context.Background()
	state.killPlayer(ctx)

	// Equipped item should lose 10% of 50 = 5 durability (50 -> 45)
	var durEquip, durBag uint32
	_ = cdb.QueryRow("SELECT durability FROM item_instance WHERE guid = 1001").Scan(&durEquip)
	_ = cdb.QueryRow("SELECT durability FROM item_instance WHERE guid = 1002").Scan(&durBag)

	if durEquip != 45 {
		t.Fatalf("expected equipped item durability 45, got %d", durEquip)
	}
	if durBag != 100 {
		t.Fatalf("expected inventory item durability 100 (unaffected by death), got %d", durBag)
	}
}

func TestSpiritHealerActivateDurabilityAndResSickness(t *testing.T) {
	// Level 15 player
	player := &playerState{GUID: 9, Level: 15, Health: 1, MaxHealth: 100, PlayerFlags: playerFlagGhost}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)

	cdb := server.CharactersStore.DB

	// Create corpse for player
	_, _ = cdb.Exec("INSERT INTO corpse (guid, posX, posY, posZ, orientation, mapId, corpseType) VALUES (9, 0, 0, 0, 0, 0, 1)")

	// Spawn spirit healer creature
	spiritGUID := uint64(55555)
	_, _ = cdb.Exec("INSERT INTO creature (guid, id, map, npcflag) VALUES (?, 6491, 0, ?)", spiritGUID, npcFlagSpiritHealer)
	_, _ = cdb.Exec("INSERT INTO creature_template (entry, npcflag) VALUES (6491, ?)", npcFlagSpiritHealer)

	// Insert item templates:
	// Entry 200: MaxDurability = 40
	// Entry 201: MaxDurability = 80
	_, _ = cdb.Exec("INSERT INTO item_template (entry, MaxDurability) VALUES (200, 40), (201, 80)")

	// Insert item instances:
	// Item 2001: entry 200, curDur = 40
	// Item 2002: entry 201, curDur = 80
	_, _ = cdb.Exec("INSERT INTO item_instance (guid, itemEntry, durability) VALUES (2001, 200, 40), (2002, 201, 80)")

	// Item 2001 is equipped (bag 0, slot 1)
	// Item 2002 is in backpack (bag 0, slot 24)
	_, _ = cdb.Exec("INSERT INTO character_inventory (guid, bag, slot, item) VALUES (9, 0, 1, 2001), (9, 0, 24, 2002)")

	// Payload with spirit healer GUID
	payload := protocol.NewBuffer(8)
	payload.WriteU64(spiritGUID)

	ctx := context.Background()
	if !state.handleSpiritHealerActivate(ctx, payload.Bytes()) {
		t.Fatal("handleSpiritHealerActivate returned false")
	}

	// 1. Check player resurrected at 50% health
	if player.Health != 50 {
		t.Fatalf("expected resurrected health 50, got %d", player.Health)
	}
	if player.PlayerFlags&playerFlagGhost != 0 {
		t.Fatal("expected ghost flag removed")
	}

	// 2. Both equipped and inventory items lose 25% durability:
	// Item 2001: 40 - 25% (10) = 30
	// Item 2002: 80 - 25% (20) = 60
	var durEquip, durBag uint32
	_ = cdb.QueryRow("SELECT durability FROM item_instance WHERE guid = 2001").Scan(&durEquip)
	_ = cdb.QueryRow("SELECT durability FROM item_instance WHERE guid = 2002").Scan(&durBag)

	if durEquip != 30 {
		t.Fatalf("expected equipped item durability 30, got %d", durEquip)
	}
	if durBag != 60 {
		t.Fatalf("expected inventory item durability 60, got %d", durBag)
	}

	// 3. Level 15 player should have Resurrection Sickness (spell 15007)
	if _, ok := state.auras[15007]; !ok {
		t.Fatal("expected Resurrection Sickness aura 15007 applied to level 15 player")
	}

	// 4. Corpse should now be bones (corpseTypeBones = 0)
	var cType int
	_ = cdb.QueryRow("SELECT corpseType FROM corpse WHERE guid = 9").Scan(&cType)
	if cType != int(corpseTypeBones) {
		t.Fatalf("expected corpseType %d (bones), got %d", corpseTypeBones, cType)
	}
}

func TestSpiritHealerLowLevelNoSickness(t *testing.T) {
	// Level 10 player (should NOT get sickness)
	player := &playerState{GUID: 9, Level: 10, Health: 1, MaxHealth: 100, PlayerFlags: playerFlagGhost}
	state, clientConn, server := newDeathTestSession(t, player)
	drainServerFrames(t, clientConn)

	cdb := server.CharactersStore.DB
	spiritGUID := uint64(55556)
	_, _ = cdb.Exec("INSERT INTO creature (guid, id, map, npcflag) VALUES (?, 6491, 0, ?)", spiritGUID, npcFlagSpiritHealer)
	_, _ = cdb.Exec("INSERT INTO creature_template (entry, npcflag) VALUES (6491, ?)", npcFlagSpiritHealer)

	payload := protocol.NewBuffer(8)
	payload.WriteU64(spiritGUID)

	ctx := context.Background()
	if !state.handleSpiritHealerActivate(ctx, payload.Bytes()) {
		t.Fatal("handleSpiritHealerActivate returned false")
	}

	if _, ok := state.auras[15007]; ok {
		t.Fatal("expected level 10 player to NOT receive Resurrection Sickness")
	}
}

