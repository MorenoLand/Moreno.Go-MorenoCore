package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestChannelAndCharServiceHandlers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, gender INTEGER, skin INTEGER, face INTEGER, hairStyle INTEGER, hairColor INTEGER, facialStyle INTEGER, race INTEGER, class INTEGER, level INTEGER)",
		"INSERT INTO characters VALUES (1, 'OldName', 0, 1, 2, 3, 4, 5, 1, 1, 80)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store, Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. Channel tests
	if !sess.handleChannelPassword(ctx, nil) ||
		!sess.handleChannelSetOwner(ctx, nil) ||
		!sess.handleChannelOwner(ctx, nil) ||
		!sess.handleChannelModerator(ctx, nil) ||
		!sess.handleChannelUnmoderator(ctx, nil) ||
		!sess.handleChannelMute(ctx, nil) ||
		!sess.handleChannelUnmute(ctx, nil) ||
		!sess.handleChannelInvite(ctx, nil) ||
		!sess.handleChannelKick(ctx, nil) ||
		!sess.handleChannelBan(ctx, nil) ||
		!sess.handleChannelUnban(ctx, nil) ||
		!sess.handleChannelAnnouncements(ctx, nil) ||
		!sess.handleChannelVoiceOn(ctx, nil) ||
		!sess.handleDeclineChannelInvite(ctx, nil) {
		t.Fatal("channel handler failed")
	}

	// 2. Character Rename
	renBuf := protocol.NewBuffer(16)
	renBuf.WriteU64(1)
	renBuf.WriteCString("NewName")
	if !sess.handleCharRename(ctx, renBuf.Bytes()) {
		t.Fatal("handleCharRename failed")
	}
	var name string
	_ = db.QueryRowContext(ctx, "SELECT name FROM characters WHERE guid = 1").Scan(&name)
	if name != "NewName" {
		t.Fatalf("expected NewName, got %s", name)
	}

	// 3. Character Customize
	custBuf := protocol.NewBuffer(24)
	custBuf.WriteU64(1)
	custBuf.WriteCString("CustomHero")
	custBuf.WriteU8(1) // gender
	custBuf.WriteU8(2) // skin
	custBuf.WriteU8(3) // face
	custBuf.WriteU8(4) // hairStyle
	custBuf.WriteU8(5) // hairColor
	custBuf.WriteU8(6) // facialHair
	if !sess.handleCharCustomize(ctx, custBuf.Bytes()) {
		t.Fatal("handleCharCustomize failed")
	}
	var gender int
	_ = db.QueryRowContext(ctx, "SELECT gender FROM characters WHERE guid = 1").Scan(&gender)
	if gender != 1 {
		t.Fatalf("expected gender 1, got %d", gender)
	}

	// 4. Character Race Change (Human 1 -> Dwarf 3, same team)
	raceBuf := protocol.NewBuffer(25)
	raceBuf.WriteU64(1)
	raceBuf.WriteCString("RaceHero")
	raceBuf.WriteU8(1) // gender
	raceBuf.WriteU8(2) // skin
	raceBuf.WriteU8(3) // face
	raceBuf.WriteU8(4) // hairStyle
	raceBuf.WriteU8(5) // hairColor
	raceBuf.WriteU8(6) // facialHair
	raceBuf.WriteU8(3) // Dwarf
	if !sess.handleCharRaceChange(ctx, raceBuf.Bytes()) {
		t.Fatal("handleCharRaceChange failed")
	}
	var race int
	_ = db.QueryRowContext(ctx, "SELECT race FROM characters WHERE guid = 1").Scan(&race)
	if race != 3 {
		t.Fatalf("expected race 3, got %d", race)
	}

	// 5. Character Faction Change (Dwarf 3 -> Orc 2, opposite team)
	factionBuf := protocol.NewBuffer(25)
	factionBuf.WriteU64(1)
	factionBuf.WriteCString("FactionHero")
	factionBuf.WriteU8(1) // gender
	factionBuf.WriteU8(2) // skin
	factionBuf.WriteU8(3) // face
	factionBuf.WriteU8(4) // hairStyle
	factionBuf.WriteU8(5) // hairColor
	factionBuf.WriteU8(6) // facialHair
	factionBuf.WriteU8(2) // Orc
	if !sess.handleCharFactionChange(ctx, factionBuf.Bytes()) {
		t.Fatal("handleCharFactionChange failed")
	}
	_ = db.QueryRowContext(ctx, "SELECT race FROM characters WHERE guid = 1").Scan(&race)
	if race != 2 {
		t.Fatalf("expected race 2, got %d", race)
	}

	// 5. Complete movie
	if !sess.handleCompleteMovie(ctx, nil) {
		t.Fatal("handleCompleteMovie failed")
	}

	// 6. Complain
	if !sess.handleComplain(ctx, nil) {
		t.Fatal("handleComplain failed")
	}
}
