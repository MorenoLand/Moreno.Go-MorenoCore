package world

import (
	"context"
	"database/sql"
	"math"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func setupTestArenaDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	queries := []string{
		`CREATE TABLE arena_team (
			arenaTeamId INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			captainGuid INTEGER NOT NULL,
			type INTEGER NOT NULL,
			backgroundColor INTEGER DEFAULT 0,
			emblemStyle INTEGER DEFAULT 0,
			emblemColor INTEGER DEFAULT 0,
			borderStyle INTEGER DEFAULT 0,
			borderColor INTEGER DEFAULT 0,
			rating INTEGER DEFAULT 1500,
			weekGames INTEGER DEFAULT 0,
			weekWins INTEGER DEFAULT 0,
			seasonGames INTEGER DEFAULT 0,
			seasonWins INTEGER DEFAULT 0,
			rank INTEGER DEFAULT 1
		);`,
		`CREATE TABLE arena_team_member (
			arenaTeamId INTEGER NOT NULL,
			guid INTEGER NOT NULL,
			weekGames INTEGER DEFAULT 0,
			weekWins INTEGER DEFAULT 0,
			seasonGames INTEGER DEFAULT 0,
			seasonWins INTEGER DEFAULT 0,
			personalRating INTEGER DEFAULT 1500,
			PRIMARY KEY(arenaTeamId, guid)
		);`,
		`CREATE TABLE characters (
			guid INTEGER PRIMARY KEY,
			account INTEGER NOT NULL,
			name TEXT NOT NULL,
			race INTEGER DEFAULT 1,
			class INTEGER DEFAULT 1,
			gender INTEGER DEFAULT 0,
			level INTEGER DEFAULT 80,
			health INTEGER DEFAULT 10000,
			arenaPoints INTEGER DEFAULT 0
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("failed to execute schema setup query: %v", err)
		}
	}

	return db
}

func setupArenaServerAndSession(t *testing.T, db *sql.DB, mapID uint32, guid uint64, name string, race uint8) (*Server, *session, net.Conn) {
	srv := &Server{
		sessions:   make(map[*session]struct{}),
		Data:       &wotlk.Store{},
		arenaState: make(map[uint32]*arenaBattlegroundState),
	}
	if db != nil {
		srv.CharactersStore = &database.Store{DB: db}
	}

	clientConn, serverConn := net.Pipe()

	// Drain connection to avoid net.Pipe blocking
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   guid,
		accountName:  name + "Acc",
		player: &playerState{
			Name:      name,
			Map:       mapID,
			Race:      race,
			Class:     1, // Warrior
			Level:     80,
			Health:    10000,
			MaxHealth: 10000,
			Powers:    [7]uint32{5000, 0, 0, 100, 0, 0, 0},
			MaxPowers: [7]uint32{5000, 1000, 0, 100, 0, 0, 1000},
		},
		auras: make(map[uint32]struct{}),
	}

	srv.sessions[sess] = struct{}{}
	return srv, sess, clientConn
}

func TestArena_MapAndLocationParity(t *testing.T) {
	// Verify all 5 arena maps return true for IsArenaMap
	maps := []uint32{ArenaMapNagrand, ArenaMapBladesEdge, ArenaMapRuinsOfLordaeron, ArenaMapDalaranSewers, ArenaMapRingOfValor}
	for _, m := range maps {
		if !IsArenaMap(m) {
			t.Errorf("expected map %d to be recognized as arena map", m)
		}
	}

	// Non-arena maps
	nonArena := []uint32{0, 1, 30, 489, 529, 607, 628}
	for _, m := range nonArena {
		if IsArenaMap(m) {
			t.Errorf("expected map %d NOT to be recognized as arena map", m)
		}
	}

	// Verify exact start coords for Nagrand (Map 559)
	goldX, goldY, goldZ, _ := GetArenaStartLocation(ArenaMapNagrand, ArenaTeamGold)
	if math.Abs(float64(goldX)-4027.60) > 0.01 || math.Abs(float64(goldY)-2972.78) > 0.01 || math.Abs(float64(goldZ)-12.07) > 0.01 {
		t.Errorf("nagrand gold coords mismatch: got (%f, %f, %f)", goldX, goldY, goldZ)
	}

	greenX, greenY, greenZ, greenO := GetArenaStartLocation(ArenaMapNagrand, ArenaTeamGreen)
	if math.Abs(float64(greenX)-4085.45) > 0.01 || math.Abs(float64(greenY)-2866.83) > 0.01 || math.Abs(float64(greenO)-math.Pi) > 0.01 {
		t.Errorf("nagrand green coords mismatch: got (%f, %f, %f, %f)", greenX, greenY, greenZ, greenO)
	}

	// Verify Blade's Edge (Map 562)
	beGoldX, _, beGoldZ, _ := GetArenaStartLocation(ArenaMapBladesEdge, ArenaTeamGold)
	if math.Abs(float64(beGoldX)-6292.66) > 0.01 || math.Abs(float64(beGoldZ)-4.96) > 0.01 {
		t.Errorf("blades edge gold coords mismatch: got (%f, %f)", beGoldX, beGoldZ)
	}

	// Verify Ruins of Lordaeron (Map 572)
	rlGoldX, rlGoldY, _, _ := GetArenaStartLocation(ArenaMapRuinsOfLordaeron, ArenaTeamGold)
	if math.Abs(float64(rlGoldX)-1277.87) > 0.01 || math.Abs(float64(rlGoldY)-1744.90) > 0.01 {
		t.Errorf("ruins of lordaeron gold coords mismatch: got (%f, %f)", rlGoldX, rlGoldY)
	}

	// Verify Dalaran Sewers (Map 617)
	dsGoldX, dsGoldY, _, _ := GetArenaStartLocation(ArenaMapDalaranSewers, ArenaTeamGold)
	if math.Abs(float64(dsGoldX)-1218.01) > 0.01 || math.Abs(float64(dsGoldY)-764.80) > 0.01 {
		t.Errorf("dalaran sewers gold coords mismatch: got (%f, %f)", dsGoldX, dsGoldY)
	}

	// Verify Ring of Valor (Map 618)
	rvGoldX, rvGoldY, _, _ := GetArenaStartLocation(ArenaMapRingOfValor, ArenaTeamGold)
	if math.Abs(float64(rvGoldX)-763.56) > 0.01 || math.Abs(float64(rvGoldY)-(-274.00)) > 0.01 {
		t.Errorf("ring of valor gold coords mismatch: got (%f, %f)", rvGoldX, rvGoldY)
	}
}

func TestArena_GameObjectDetection(t *testing.T) {
	// Shadow sight buffs
	if !isArenaGameObject(ArenaGOShadowSight1) || !isArenaGameObject(ArenaGOShadowSight2) {
		t.Errorf("expected Shadow Sight buff objects 184663 and 184664 to be recognized")
	}

	// Arena doors
	if !isArenaGameObject(ArenaGONA_Door1) || !isArenaGameObject(ArenaGOBE_Door1) || !isArenaGameObject(ArenaGORL_Door1) || !isArenaGameObject(ArenaGODS_Door1) {
		t.Errorf("expected arena doors to be recognized as arena game objects")
	}

	// Ring of Valor elevator and pillars
	if !isArenaGameObject(ArenaGORV_Elevator1) || !isArenaGameObject(ArenaGORV_Pillar1) || !isArenaGameObject(ArenaGORV_PillarCol1) {
		t.Errorf("expected Ring of Valor elevators and pillars to be recognized")
	}

	// Non-arena game objects
	if isArenaGameObject(179871) || isArenaGameObject(190722) || isArenaGameObject(184419) {
		t.Errorf("non-arena game objects should return false")
	}
}

func TestArena_PlayerAdmissionAndPreparation(t *testing.T) {
	srv, sess, conn := setupArenaServerAndSession(t, nil, ArenaMapNagrand, 1001, "GladiatorA", 1) // Human (Alliance)
	defer conn.Close()

	arena := srv.getOrCreateArenaState(ArenaMapNagrand, 1, ArenaType2v2, false)
	if arena == nil {
		t.Fatalf("failed to create arena state")
	}

	// Admit Alliance player to Gold team
	srv.addPlayerToArena(sess, ArenaTeamGold, arena)

	// Verify Arena Preparation aura (32727) applied during warmup
	if !sess.hasAura(SpellArenaPreparation) {
		t.Errorf("expected SpellArenaPreparation (32727) to be applied during warmup")
	}

	// Verify Alliance Gold flag (32724) applied
	if !sess.hasAura(SpellAllianceGoldFlag) {
		t.Errorf("expected SpellAllianceGoldFlag (32724) to be applied for Alliance on Gold team")
	}

	// Check alive counts
	if arena.GoldAlive != 1 || arena.GreenAlive != 0 {
		t.Errorf("expected GoldAlive=1, GreenAlive=0, got GoldAlive=%d, GreenAlive=%d", arena.GoldAlive, arena.GreenAlive)
	}

	// Admit Horde player to Green team
	_, sessH, connH := setupArenaServerAndSession(t, nil, ArenaMapNagrand, 1002, "GladiatorH", 2) // Orc (Horde)
	defer connH.Close()
	srv.sessions[sessH] = struct{}{}

	srv.addPlayerToArena(sessH, ArenaTeamGreen, arena)

	// Verify Horde Green flag (35775) applied
	if !sessH.hasAura(SpellHordeGreenFlag) {
		t.Errorf("expected SpellHordeGreenFlag (35775) to be applied for Horde on Green team")
	}

	if arena.GoldAlive != 1 || arena.GreenAlive != 1 {
		t.Errorf("expected GoldAlive=1, GreenAlive=1, got GoldAlive=%d, GreenAlive=%d", arena.GoldAlive, arena.GreenAlive)
	}
}

func TestArena_LifecycleAndWinConditions(t *testing.T) {
	db := setupTestArenaDB(t)
	defer db.Close()

	// Insert two 2v2 arena teams: Team 1 (Rating 1500) and Team 2 (Rating 1500)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating) VALUES (1, 'Alpha', 1001, 2, 1500), (2, 'Bravo', 1002, 2, 1500)")
	if err != nil {
		t.Fatalf("failed to insert arena teams: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, personalRating) VALUES (1, 1001, 1500), (2, 1002, 1500)")
	if err != nil {
		t.Fatalf("failed to insert arena team members: %v", err)
	}

	srv, sessA, connA := setupArenaServerAndSession(t, db, ArenaMapRuinsOfLordaeron, 1001, "AlphaWarrior", 1)
	defer connA.Close()

	_, sessB, connB := setupArenaServerAndSession(t, db, ArenaMapRuinsOfLordaeron, 1002, "BravoRogue", 2)
	defer connB.Close()
	srv.sessions[sessB] = struct{}{}

	arena := srv.getOrCreateArenaState(ArenaMapRuinsOfLordaeron, 1, ArenaType2v2, true)
	arena.GoldTeamID = 1
	arena.GoldTeamName = "Alpha"
	arena.GreenTeamID = 2
	arena.GreenTeamName = "Bravo"

	srv.addPlayerToArena(sessA, ArenaTeamGold, arena)
	srv.addPlayerToArena(sessB, ArenaTeamGreen, arena)

	// Initially in warmup
	if arena.Status != ArenaStatusWaitJoin {
		t.Errorf("expected ArenaStatusWaitJoin, got %d", arena.Status)
	}

	// Trigger match start
	srv.startArenaMatch(arena)

	if arena.Status != ArenaStatusInProgress {
		t.Errorf("expected ArenaStatusInProgress, got %d", arena.Status)
	}
	if sessA.hasAura(SpellArenaPreparation) || sessB.hasAura(SpellArenaPreparation) {
		t.Errorf("expected SpellArenaPreparation to be removed upon match start")
	}

	// Update arena damage scores
	srv.updateArenaDamageScore(sessA, 5000)
	srv.updateArenaDamageScore(sessB, 3000)
	srv.updateArenaHealingScore(sessA, 1200)

	if arena.Scores[1001].DamageDone != 5000 || arena.Scores[1001].HealingDone != 1200 {
		t.Errorf("player score mismatch for 1001: %+v", arena.Scores[1001])
	}
	if arena.Scores[1002].DamageDone != 3000 {
		t.Errorf("player score mismatch for 1002: %+v", arena.Scores[1002])
	}

	// Player B (Green team) dies
	sessB.player.Health = 0
	srv.handleArenaPlayerDeath(sessB)

	if arena.GreenAlive != 0 {
		t.Errorf("expected GreenAlive=0, got %d", arena.GreenAlive)
	}

	// Gold team (Alpha) should win!
	if arena.Winner != int8(ArenaTeamGold) {
		t.Errorf("expected Winner to be Gold (1), got %d", arena.Winner)
	}
	if arena.Status != ArenaStatusWaitLeave {
		t.Errorf("expected status ArenaStatusWaitLeave, got %d", arena.Status)
	}

	// Verify database rating settlement for Team 1 (won) and Team 2 (lost)
	var rating1, rating2, games1, wins1 uint32
	_ = db.QueryRowContext(ctx, "SELECT rating, weekGames, weekWins FROM arena_team WHERE arenaTeamId = 1").Scan(&rating1, &games1, &wins1)
	_ = db.QueryRowContext(ctx, "SELECT rating FROM arena_team WHERE arenaTeamId = 2").Scan(&rating2)

	if rating1 <= 1500 {
		t.Errorf("expected winner rating to increase above 1500, got %d", rating1)
	}
	if rating2 >= 1500 {
		t.Errorf("expected loser rating to decrease below 1500, got %d", rating2)
	}
	if games1 != 1 || wins1 != 1 {
		t.Errorf("expected weekGames=1 and weekWins=1 for team 1, got games=%d, wins=%d", games1, wins1)
	}
}

func TestArena_DrawTimeoutSettlement(t *testing.T) {
	db := setupTestArenaDB(t)
	defer db.Close()

	ctx := context.Background()
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating) VALUES (1, 'Alpha', 1001, 2, 1500), (2, 'Bravo', 1002, 2, 1500)")

	srv, sessA, connA := setupArenaServerAndSession(t, db, ArenaMapNagrand, 1001, "AlphaWarrior", 1)
	defer connA.Close()
	_, sessB, connB := setupArenaServerAndSession(t, db, ArenaMapNagrand, 1002, "BravoRogue", 2)
	defer connB.Close()
	srv.sessions[sessB] = struct{}{}

	arena := srv.getOrCreateArenaState(ArenaMapNagrand, 1, ArenaType2v2, true)
	arena.GoldTeamID = 1
	arena.GoldTeamName = "Alpha"
	arena.GreenTeamID = 2
	arena.GreenTeamName = "Bravo"

	srv.addPlayerToArena(sessA, ArenaTeamGold, arena)
	srv.addPlayerToArena(sessB, ArenaTeamGreen, arena)
	srv.startArenaMatch(arena)

	// Simulate 46 minutes elapsed (exceeds 45 minute limit)
	simulatedNow := arena.MatchStartTime.Add(46 * time.Minute)
	srv.updateArenaTick(arena, simulatedNow)

	if arena.Winner != ArenaTeamDraw {
		t.Errorf("expected draw (2) on timeout, got %d", arena.Winner)
	}

	// Verify both teams lost 16 rating points (-16 in 3.3.5)
	var rating1, rating2 uint32
	_ = db.QueryRowContext(ctx, "SELECT rating FROM arena_team WHERE arenaTeamId = 1").Scan(&rating1)
	_ = db.QueryRowContext(ctx, "SELECT rating FROM arena_team WHERE arenaTeamId = 2").Scan(&rating2)

	if rating1 != 1500-16 {
		t.Errorf("expected rating1 to be 1484 after draw, got %d", rating1)
	}
	if rating2 != 1500-16 {
		t.Errorf("expected rating2 to be 1484 after draw, got %d", rating2)
	}
}

func TestArena_ShadowSightInteraction(t *testing.T) {
	srv, sess, conn := setupArenaServerAndSession(t, nil, ArenaMapNagrand, 1001, "GladiatorA", 1)
	defer conn.Close()

	arena := srv.getOrCreateArenaState(ArenaMapNagrand, 1, ArenaType2v2, false)
	srv.addPlayerToArena(sess, ArenaTeamGold, arena)
	srv.startArenaMatch(arena)

	ctx := context.Background()
	// Click Shadow Sight GameObject (184663)
	handled := srv.handleArenaGameObjectUse(ctx, sess, 0x1234, ArenaGOShadowSight1)
	if !handled {
		t.Errorf("expected handleArenaGameObjectUse to handle Shadow Sight")
	}

	// Verify Shadow Sight aura (34709) applied
	if !sess.hasAura(SpellShadowSight) {
		t.Errorf("expected SpellShadowSight (34709) to be applied to player")
	}
}

func TestArena_PvPLogDataPacketSerialization(t *testing.T) {
	srv, sessA, connA := setupArenaServerAndSession(t, nil, ArenaMapBladesEdge, 1001, "PlayerA", 1)
	defer connA.Close()
	_, sessB, connB := setupArenaServerAndSession(t, nil, ArenaMapBladesEdge, 1002, "PlayerB", 2)
	defer connB.Close()
	srv.sessions[sessB] = struct{}{}

	arena := srv.getOrCreateArenaState(ArenaMapBladesEdge, 1, ArenaType2v2, true)
	arena.GoldTeamName = "GoldenBoys"
	arena.GreenTeamName = "GreenHulks"
	arena.GoldTeamScore.RatingChange = 24
	arena.GoldTeamScore.MatchmakerRating = 1524
	arena.GreenTeamScore.RatingChange = -18
	arena.GreenTeamScore.MatchmakerRating = 1482

	srv.addPlayerToArena(sessA, ArenaTeamGold, arena)
	srv.addPlayerToArena(sessB, ArenaTeamGreen, arena)

	arena.Scores[1001].KillingBlows = 1
	arena.Scores[1001].DamageDone = 12000
	arena.Scores[1001].HealingDone = 500

	arena.Scores[1002].DamageDone = 8500
	arena.Scores[1002].HealingDone = 200

	arena.Status = ArenaStatusWaitLeave
	arena.Winner = int8(ArenaTeamGold)

	packet := srv.buildArenaPvPLogDataPacket(arena)
	if len(packet) == 0 {
		t.Fatalf("expected non-empty PvP log data packet")
	}

	r := protocol.NewReader(packet)
	isArena, err := r.ReadU8()
	if err != nil || isArena != 1 {
		t.Errorf("expected isArena=1, got %d (err: %v)", isArena, err)
	}

	// Green rating info: loss=18, gain=0, mmr=1482
	greenLoss, _ := r.ReadU32()
	greenGain, _ := r.ReadU32()
	greenMMR, _ := r.ReadU32()
	if greenLoss != 18 || greenGain != 0 || greenMMR != 1482 {
		t.Errorf("green rating info mismatch: loss=%d, gain=%d, mmr=%d", greenLoss, greenGain, greenMMR)
	}

	// Gold rating info: loss=0, gain=24, mmr=1524
	goldLoss, _ := r.ReadU32()
	goldGain, _ := r.ReadU32()
	goldMMR, _ := r.ReadU32()
	if goldLoss != 0 || goldGain != 24 || goldMMR != 1524 {
		t.Errorf("gold rating info mismatch: loss=%d, gain=%d, mmr=%d", goldLoss, goldGain, goldMMR)
	}

	// Team names
	greenName, _ := r.ReadCString()
	goldName, _ := r.ReadCString()
	if greenName != "GreenHulks" || goldName != "GoldenBoys" {
		t.Errorf("team names mismatch: green=%s, gold=%s", greenName, goldName)
	}

	// Match ended & winner
	ended, _ := r.ReadU8()
	winner, _ := r.ReadU8()
	if ended != 1 || winner != 1 { // 1 = Gold/Alliance
		t.Errorf("expected ended=1, winner=1, got ended=%d, winner=%d", ended, winner)
	}

	// Player count
	numPlayers, _ := r.ReadU32()
	if numPlayers != 2 {
		t.Errorf("expected 2 player scores, got %d", numPlayers)
	}
}

func TestArena_BattlemasterJoinQueue(t *testing.T) {
	_, sess, conn := setupArenaServerAndSession(t, nil, 0, 1001, "PlayerA", 1)
	defer conn.Close()

	buf := protocol.NewBuffer(16)
	buf.WriteU64(123456) // battlemaster guid
	buf.WriteU8(1)       // arena slot 1 (3v3)
	buf.WriteU8(0)       // asGroup
	buf.WriteU8(1)       // isRated

	ctx := context.Background()
	handled := sess.handleBattlemasterJoinArena(ctx, buf.Bytes())
	if !handled {
		t.Errorf("expected handleBattlemasterJoinArena to return true")
	}

	if !sess.bgQueues[0].Active {
		t.Errorf("expected queue slot 0 to be active")
	}
	if sess.bgQueues[0].ArenaType != ArenaType3v3 {
		t.Errorf("expected ArenaType=3, got %d", sess.bgQueues[0].ArenaType)
	}
	if !sess.bgQueues[0].IsArena || !sess.bgQueues[0].IsRated {
		t.Errorf("expected IsArena=true and IsRated=true, got isArena=%v, isRated=%v", sess.bgQueues[0].IsArena, sess.bgQueues[0].IsRated)
	}
}
