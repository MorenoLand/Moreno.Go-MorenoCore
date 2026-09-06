package world

import (
	"context"
	"database/sql"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	_ "modernc.org/sqlite"
)

func TestCalcRatingDelta(t *testing.T) {
	// 1. Equal ratings
	gain, loss := CalcRatingDelta(1500, 1500)
	if gain != 16 || loss != 16 {
		t.Fatalf("expected gain 16 and loss 16 for equal ratings, got gain=%d loss=%d", gain, loss)
	}

	// 2. High rating beats low rating
	gainHigh, lossHigh := CalcRatingDelta(2000, 1500)
	if gainHigh >= 16 || gainHigh < 1 {
		t.Fatalf("expected small gain when favored team wins, got %d", gainHigh)
	}
	if lossHigh != gainHigh {
		t.Fatalf("expected loss equal to gain for ratings >= 1000, got gain=%d loss=%d", gainHigh, lossHigh)
	}

	// 3. Underdog beats favored team
	gainUnderdog, lossUnderdog := CalcRatingDelta(1500, 2000)
	if gainUnderdog <= 16 {
		t.Fatalf("expected large gain when underdog wins, got %d", gainUnderdog)
	}
	if lossUnderdog != gainUnderdog {
		t.Fatalf("expected loss equal to gain for ratings >= 1000, got gain=%d loss=%d", gainUnderdog, lossUnderdog)
	}

	// 4. Low rating loss mitigation (< 1000)
	// Winner 900, Loser 800 (gain = 12, loss = 10 because 12 * 800 / 1000 = 9.6 -> 10)
	gainLow, lossLow := CalcRatingDelta(900, 800)
	if lossLow >= gainLow {
		t.Fatalf("expected mitigated loss for loser with rating < 1000, got gain=%d loss=%d", gainLow, lossLow)
	}
	expectedLoss := int32(math.Round(float64(gainLow) * (800.0 / 1000.0)))
	if lossLow != expectedLoss {
		t.Fatalf("expected mitigated loss %d, got %d", expectedLoss, lossLow)
	}
}

func TestCalculateArenaPoints(t *testing.T) {
	// Rating <= 150 yields 0
	if pts := CalculateArenaPoints(150, ArenaTeamType5v5); pts != 0 {
		t.Fatalf("expected 0 points for rating 150, got %d", pts)
	}
	if pts := CalculateArenaPoints(0, ArenaTeamType2v2); pts != 0 {
		t.Fatalf("expected 0 points for rating 0, got %d", pts)
	}

	// Rating 1500:
	// 5v5: 0.22*1500 + 14 = 344
	pts5v5_1500 := CalculateArenaPoints(1500, ArenaTeamType5v5)
	if pts5v5_1500 != 344 {
		t.Fatalf("expected 344 points for 1500 rating 5v5, got %d", pts5v5_1500)
	}

	// 2v2 (76%): 344 * 0.76 = 261.44 -> 261
	pts2v2_1500 := CalculateArenaPoints(1500, ArenaTeamType2v2)
	if pts2v2_1500 != 261 {
		t.Fatalf("expected 261 points for 1500 rating 2v2, got %d", pts2v2_1500)
	}

	// 3v3 (88%): 344 * 0.88 = 302.72 -> 303
	pts3v3_1500 := CalculateArenaPoints(1500, ArenaTeamType3v3)
	if pts3v3_1500 != 303 {
		t.Fatalf("expected 303 points for 1500 rating 3v3, got %d", pts3v3_1500)
	}

	// Rating 2000:
	// 5v5: 1511.26 / (1 + 1639.28 * exp(-0.00412 * 2000)) = 1054.92 -> 1055
	pts5v5_2000 := CalculateArenaPoints(2000, ArenaTeamType5v5)
	if pts5v5_2000 != 1055 {
		t.Fatalf("expected 1055 points for 2000 rating 5v5, got %d", pts5v5_2000)
	}

	// 2v2 (76%): 1054.92 * 0.76 = 801.74 -> 802
	pts2v2_2000 := CalculateArenaPoints(2000, ArenaTeamType2v2)
	if pts2v2_2000 != 802 {
		t.Fatalf("expected 802 points for 2000 rating 2v2, got %d", pts2v2_2000)
	}
}

func setupArenaTestDB(t *testing.T) (*Server, *sql.DB) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	schema := `
	CREATE TABLE arena_team (
		arenaTeamId INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		captainGuid INTEGER NOT NULL,
		type INTEGER NOT NULL,
		rating INTEGER NOT NULL DEFAULT 1500,
		seasonGames INTEGER NOT NULL DEFAULT 0,
		seasonWins INTEGER NOT NULL DEFAULT 0,
		weekGames INTEGER NOT NULL DEFAULT 0,
		weekWins INTEGER NOT NULL DEFAULT 0,
		rank INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE arena_team_member (
		arenaTeamId INTEGER NOT NULL,
		guid INTEGER NOT NULL,
		weekGames INTEGER NOT NULL DEFAULT 0,
		weekWins INTEGER NOT NULL DEFAULT 0,
		seasonGames INTEGER NOT NULL DEFAULT 0,
		seasonWins INTEGER NOT NULL DEFAULT 0,
		personalRating INTEGER NOT NULL DEFAULT 1500,
		PRIMARY KEY (arenaTeamId, guid)
	);

	CREATE TABLE characters (
		guid INTEGER PRIMARY KEY,
		account INTEGER NOT NULL DEFAULT 1,
		name TEXT NOT NULL,
		arenaPoints INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		CharactersStore: store,
		WorldStore:      store,
		Config:          config.Default(),
		sessions:        make(map[*session]struct{}),
	}
	return srv, db
}

func TestRecordArenaMatchResult(t *testing.T) {
	srv, db := setupArenaTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Insert Winner Team (Team 1, 2v2, rating 1500)
	_, err := db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating) VALUES (1, 'Winners', 10, 2, 1500)")
	if err != nil {
		t.Fatalf("insert team 1 failed: %v", err)
	}
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, personalRating) VALUES (1, 10, 1500), (1, 11, 1500)")

	// Insert Loser Team (Team 2, 2v2, rating 1500)
	_, err = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating) VALUES (2, 'Losers', 20, 2, 1500)")
	if err != nil {
		t.Fatalf("insert team 2 failed: %v", err)
	}
	// One member normal (1500), one member low rating (< 1000, e.g. 600)
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, personalRating) VALUES (2, 20, 1500), (2, 21, 600)")

	gain, loss, err := srv.RecordArenaMatchResult(ctx, 1, 2, []uint64{10, 11}, []uint64{20, 21})
	if err != nil {
		t.Fatalf("RecordArenaMatchResult failed: %v", err)
	}
	if gain != 16 || loss != 16 {
		t.Fatalf("expected gain=16 loss=16, got %d, %d", gain, loss)
	}

	// Verify winner team stats
	var winRating, winWeekGames, winWeekWins, winSeasonGames, winSeasonWins uint32
	_ = db.QueryRowContext(ctx, "SELECT rating, weekGames, weekWins, seasonGames, seasonWins FROM arena_team WHERE arenaTeamId = 1").Scan(
		&winRating, &winWeekGames, &winWeekWins, &winSeasonGames, &winSeasonWins)
	if winRating != 1516 || winWeekGames != 1 || winWeekWins != 1 || winSeasonGames != 1 || winSeasonWins != 1 {
		t.Fatalf("unexpected winner team stats: rating=%d games=%d wins=%d", winRating, winWeekGames, winWeekWins)
	}

	// Verify loser team stats
	var loseRating, loseWeekGames, loseWeekWins uint32
	_ = db.QueryRowContext(ctx, "SELECT rating, weekGames, weekWins FROM arena_team WHERE arenaTeamId = 2").Scan(
		&loseRating, &loseWeekGames, &loseWeekWins)
	if loseRating != 1484 || loseWeekGames != 1 || loseWeekWins != 0 {
		t.Fatalf("unexpected loser team stats: rating=%d games=%d wins=%d", loseRating, loseWeekGames, loseWeekWins)
	}

	// Verify winner members personal rating
	var pRating10 uint32
	_ = db.QueryRowContext(ctx, "SELECT personalRating FROM arena_team_member WHERE arenaTeamId = 1 AND guid = 10").Scan(&pRating10)
	if pRating10 != 1516 {
		t.Fatalf("expected winner member personal rating 1516, got %d", pRating10)
	}

	// Verify loser members:
	// guid 20 (normal 1500): 1500 - 16 = 1484
	var pRating20 uint32
	_ = db.QueryRowContext(ctx, "SELECT personalRating FROM arena_team_member WHERE arenaTeamId = 2 AND guid = 20").Scan(&pRating20)
	if pRating20 != 1484 {
		t.Fatalf("expected loser member 20 personal rating 1484, got %d", pRating20)
	}

	// guid 21 (low 600): loss scaled down by 600/1000 = 0.6 -> round(16 * 0.6) = 10 -> 600 - 10 = 590
	var pRating21 uint32
	_ = db.QueryRowContext(ctx, "SELECT personalRating FROM arena_team_member WHERE arenaTeamId = 2 AND guid = 21").Scan(&pRating21)
	if pRating21 != 590 {
		t.Fatalf("expected loser member 21 personal rating 590, got %d", pRating21)
	}
}

func TestFlushWeeklyArenaPoints(t *testing.T) {
	srv, db := setupArenaTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Insert characters
	// Char 1: Qualified, in 5v5 team with rating 1500
	// Char 2: Under 10 games team (not qualified)
	// Char 3: Under 30% player participation (not qualified)
	// Char 4: Personal rating > 150 below team rating (not qualified)
	// Char 5: Near cap (9800 points), tests 10000 cap
	_, _ = db.ExecContext(ctx, `
		INSERT INTO characters (guid, name, arenaPoints) VALUES
		(1, 'HeroQualified', 0),
		(2, 'FewGames', 0),
		(3, 'LowParticipation', 0),
		(4, 'LowPersonalRating', 0),
		(5, 'NearCap', 9800)
	`)

	// Team 1: 5v5, rating 1500, 10 weekGames
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating, weekGames, weekWins) VALUES (1, 'TeamA', 1, 5, 1500, 10, 8)")
	// Member 1 (Char 1): played 10 games, personal rating 1500 -> qualified! (344 pts)
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, weekGames, weekWins, personalRating) VALUES (1, 1, 10, 8, 1500)")

	// Team 2: 5v5, rating 1500, 5 weekGames (< 10 min)
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating, weekGames, weekWins) VALUES (2, 'TeamB', 2, 5, 1500, 5, 4)")
	// Member 2 (Char 2): 5 games played -> not qualified (team weekGames < 10)
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, weekGames, weekWins, personalRating) VALUES (2, 2, 5, 4, 1500)")

	// Team 3: 5v5, rating 1500, 20 weekGames
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating, weekGames, weekWins) VALUES (3, 'TeamC', 3, 5, 1500, 20, 15)")
	// Member 3 (Char 3): played 5 games (< 30% of 20 = 6) -> not qualified
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, weekGames, weekWins, personalRating) VALUES (3, 3, 5, 4, 1500)")

	// Team 4: 5v5, rating 1500, 10 weekGames
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating, weekGames, weekWins) VALUES (4, 'TeamD', 4, 5, 1500, 10, 8)")
	// Member 4 (Char 4): personal rating 1340 (1500 - 1340 = 160 > 150 diff) -> not qualified
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, weekGames, weekWins, personalRating) VALUES (4, 4, 10, 8, 1340)")

	// Team 5: 5v5, rating 1500, 10 weekGames
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating, weekGames, weekWins) VALUES (5, 'TeamE', 5, 5, 1500, 10, 8)")
	// Member 5 (Char 5): qualified! 344 pts, but already at 9800 -> should cap at 10000
	_, _ = db.ExecContext(ctx, "INSERT INTO arena_team_member (arenaTeamId, guid, weekGames, weekWins, personalRating) VALUES (5, 5, 10, 8, 1500)")

	// Also simulate an active online session for Char 1
	sess1 := &session{
		server:       srv,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1, Name: "HeroQualified", ArenaPoints: 0},
	}
	srv.sessions[sess1] = struct{}{}

	totalGranted, err := srv.FlushWeeklyArenaPoints(ctx)
	if err != nil {
		t.Fatalf("FlushWeeklyArenaPoints failed: %v", err)
	}

	// Char 1 gets 344, Char 5 gets 344 (total granted: 688)
	if totalGranted != 688 {
		t.Fatalf("expected totalGranted 688, got %d", totalGranted)
	}

	// Check DB points
	var pts1, pts2, pts3, pts4, pts5 uint32
	_ = db.QueryRowContext(ctx, "SELECT arenaPoints FROM characters WHERE guid = 1").Scan(&pts1)
	_ = db.QueryRowContext(ctx, "SELECT arenaPoints FROM characters WHERE guid = 2").Scan(&pts2)
	_ = db.QueryRowContext(ctx, "SELECT arenaPoints FROM characters WHERE guid = 3").Scan(&pts3)
	_ = db.QueryRowContext(ctx, "SELECT arenaPoints FROM characters WHERE guid = 4").Scan(&pts4)
	_ = db.QueryRowContext(ctx, "SELECT arenaPoints FROM characters WHERE guid = 5").Scan(&pts5)

	if pts1 != 344 {
		t.Fatalf("expected char 1 to have 344 points, got %d", pts1)
	}
	if pts2 != 0 {
		t.Fatalf("expected char 2 to have 0 points, got %d", pts2)
	}
	if pts3 != 0 {
		t.Fatalf("expected char 3 to have 0 points, got %d", pts3)
	}
	if pts4 != 0 {
		t.Fatalf("expected char 4 to have 0 points, got %d", pts4)
	}
	if pts5 != 10000 {
		t.Fatalf("expected char 5 to be capped at 10000 points, got %d", pts5)
	}

	// Check online session points
	if sess1.player.ArenaPoints != 344 {
		t.Fatalf("expected online session player to have 344 arena points, got %d", sess1.player.ArenaPoints)
	}

	// Verify weekGames were reset on arena_team and arena_team_member
	var remainingWeekGames int
	_ = db.QueryRowContext(ctx, "SELECT SUM(weekGames) FROM arena_team").Scan(&remainingWeekGames)
	if remainingWeekGames != 0 {
		t.Fatalf("expected 0 weekGames remaining on arena_team, got %d", remainingWeekGames)
	}
	_ = db.QueryRowContext(ctx, "SELECT SUM(weekGames) FROM arena_team_member").Scan(&remainingWeekGames)
	if remainingWeekGames != 0 {
		t.Fatalf("expected 0 weekGames remaining on arena_team_member, got %d", remainingWeekGames)
	}
}
