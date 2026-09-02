package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestArenaTeamHandlers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, class INTEGER, level INTEGER)",
		"CREATE TABLE arena_team (arenaTeamId INTEGER PRIMARY KEY, name TEXT, captainGuid INTEGER, type INTEGER, rating INTEGER, seasonGames INTEGER, seasonWins INTEGER, weekGames INTEGER, weekWins INTEGER, rank INTEGER, backgroundColor INTEGER, emblemStyle INTEGER, emblemColor INTEGER, borderStyle INTEGER, borderColor INTEGER)",
		"CREATE TABLE arena_team_member (arenaTeamId INTEGER, guid INTEGER, weekGames INTEGER, weekWins INTEGER, seasonGames INTEGER, seasonWins INTEGER, personalRating INTEGER, PRIMARY KEY (arenaTeamId, guid))",
		"INSERT INTO characters VALUES (1, 'Captain', 1, 80)",
		"INSERT INTO characters VALUES (2, 'Partner', 2, 80)",
		"INSERT INTO arena_team VALUES (10, 'Gladiators', 1, 2, 1500, 10, 7, 5, 3, 1, 0, 1, 2, 3, 4)",
		"INSERT INTO arena_team_member VALUES (10, 1, 5, 3, 10, 7, 1500)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store, Config: config.Default()}
	sess1 := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Captain"}}
	sess2 := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Partner"}}
	srv.sessions = map[*session]struct{}{
		sess1: {},
		sess2: {},
	}
	ctx := context.Background()

	// 1. CMSG_ARENA_TEAM_QUERY
	qBuf := protocol.NewBuffer(4)
	qBuf.WriteU32(10)
	if !sess1.handleArenaTeamQuery(ctx, qBuf.Bytes()) {
		t.Fatal("handleArenaTeamQuery failed")
	}

	// 2. CMSG_ARENA_TEAM_ROSTER
	rBuf := protocol.NewBuffer(4)
	rBuf.WriteU32(10)
	if !sess1.handleArenaTeamRoster(ctx, rBuf.Bytes()) {
		t.Fatal("handleArenaTeamRoster failed")
	}

	// 3. CMSG_ARENA_TEAM_INVITE (Captain invites Partner)
	invBuf := protocol.NewBuffer(16)
	invBuf.WriteU32(10)
	invBuf.WriteCString("Partner")
	if !sess1.handleArenaTeamInvite(ctx, invBuf.Bytes()) {
		t.Fatal("handleArenaTeamInvite failed")
	}
	if sess2.arenaTeamInvited != 10 {
		t.Fatalf("expected Partner invited to team 10, got %d", sess2.arenaTeamInvited)
	}

	// 4. CMSG_ARENA_TEAM_ACCEPT (Partner accepts)
	if !sess2.handleArenaTeamAccept(ctx, nil) {
		t.Fatal("handleArenaTeamAccept failed")
	}
	if sess2.arenaTeamInvited != 0 {
		t.Fatalf("expected invite cleared after accept")
	}
	var count int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM arena_team_member WHERE arenaTeamId = 10 AND guid = 2").Scan(&count)
	if count != 1 {
		t.Fatalf("expected member record for partner in arena_team_member")
	}

	// 5. CMSG_ARENA_TEAM_LEADER (Pass leadership to Partner)
	leadBuf := protocol.NewBuffer(16)
	leadBuf.WriteU32(10)
	leadBuf.WriteCString("Partner")
	if !sess1.handleArenaTeamLeader(ctx, leadBuf.Bytes()) {
		t.Fatal("handleArenaTeamLeader failed")
	}
	var captain int
	_ = db.QueryRowContext(ctx, "SELECT captainGuid FROM arena_team WHERE arenaTeamId = 10").Scan(&captain)
	if captain != 2 {
		t.Fatalf("expected captain 2, got %d", captain)
	}

	// 6. CMSG_ARENA_TEAM_REMOVE (Remove Captain)
	remBuf := protocol.NewBuffer(16)
	remBuf.WriteU32(10)
	remBuf.WriteCString("Captain")
	if !sess2.handleArenaTeamRemove(ctx, remBuf.Bytes()) {
		t.Fatal("handleArenaTeamRemove failed")
	}
	_ = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM arena_team_member WHERE arenaTeamId = 10 AND guid = 1").Scan(&count)
	if count != 0 {
		t.Fatalf("expected Captain removed from team")
	}

	// 7. CMSG_ARENA_TEAM_LEAVE
	lBuf := protocol.NewBuffer(4)
	lBuf.WriteU32(10)
	if !sess2.handleArenaTeamLeave(ctx, lBuf.Bytes()) {
		t.Fatal("handleArenaTeamLeave failed")
	}

	// 8. CMSG_ARENA_TEAM_DISBAND
	dBuf := protocol.NewBuffer(4)
	dBuf.WriteU32(10)
	if !sess2.handleArenaTeamDisband(ctx, dBuf.Bytes()) {
		t.Fatal("handleArenaTeamDisband failed")
	}
	_ = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM arena_team WHERE arenaTeamId = 10").Scan(&count)
	if count != 0 {
		t.Fatalf("expected team deleted from DB")
	}
}
