package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestGuildQueryRosterAndInvite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, level INTEGER, class INTEGER, gender INTEGER, zone INTEGER, money INTEGER)",
		"CREATE TABLE guild (guildid INTEGER PRIMARY KEY, name TEXT, leaderguid INTEGER, EmblemStyle INTEGER, EmblemColor INTEGER, BorderStyle INTEGER, BorderColor INTEGER, BackgroundColor INTEGER, info TEXT, motd TEXT, createdate INTEGER, BankMoney INTEGER)",
		"CREATE TABLE guild_rank (guildid INTEGER, rid INTEGER, rname TEXT, rights INTEGER, BankMoneyPerDay INTEGER, PRIMARY KEY (guildid, rid))",
		"CREATE TABLE guild_member (guildid INTEGER, guid INTEGER PRIMARY KEY, rank INTEGER, pnote TEXT, offnote TEXT)",
		"INSERT INTO characters VALUES (1, 'GMPlayer', 80, 1, 0, 12, 1000)",
		"INSERT INTO characters VALUES (2, 'Newbie', 10, 2, 1, 12, 100)",
		"INSERT INTO guild VALUES (1, 'Knights of the Round', 1, 1, 2, 3, 4, 5, 'Best guild', 'Welcome all!', 1600000000, 50000)",
		"INSERT INTO guild_rank VALUES (1, 0, 'Guild Master', 4294967295, 1000000)",
		"INSERT INTO guild_rank VALUES (1, 1, 'Member', 64, 50000)",
		"INSERT INTO guild_member VALUES (1, 1, 0, 'Leader note', 'Officer note')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store, sessions: make(map[*session]struct{})}
	sessLeader := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "GMPlayer"}}
	sessNewbie := &session{server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "Newbie"}}
	srv.sessions[sessLeader] = struct{}{}
	srv.sessions[sessNewbie] = struct{}{}

	ctx := context.Background()

	// 1. Query guild info
	queryBuf := protocol.NewBuffer(4)
	queryBuf.WriteU32(1)
	if !sessLeader.handleGuildQuery(ctx, queryBuf.Bytes()) {
		t.Fatal("handleGuildQuery failed")
	}

	// 2. Query roster
	if !sessLeader.handleGuildRoster(ctx) {
		t.Fatal("handleGuildRoster failed")
	}

	// 3. Invite Newbie to guild
	invBuf := protocol.NewBuffer(32)
	invBuf.WriteCString("Newbie")
	if !sessLeader.handleGuildInvite(ctx, invBuf.Bytes()) {
		t.Fatal("handleGuildInvite failed")
	}
	if sessNewbie.guildInvitedID != 1 {
		t.Fatalf("expected guildInvitedID 1, got %d", sessNewbie.guildInvitedID)
	}

	// 4. Newbie accepts invite
	if !sessNewbie.handleGuildAccept(ctx) {
		t.Fatal("handleGuildAccept failed")
	}

	// 5. Verify Newbie is now in guild_member
	var count int64
	_ = db.QueryRow("SELECT COUNT(*) FROM guild_member WHERE guildid = 1").Scan(&count)
	if count != 2 {
		t.Fatalf("expected 2 guild members, got %d", count)
	}
}
