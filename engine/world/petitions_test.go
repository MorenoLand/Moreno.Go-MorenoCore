package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestPetitionLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, money INTEGER)",
		"CREATE TABLE guild (guildid INTEGER PRIMARY KEY, name TEXT, leaderguid INTEGER, EmblemStyle INTEGER, EmblemColor INTEGER, BorderStyle INTEGER, BorderColor INTEGER, BackgroundColor INTEGER, info TEXT, motd TEXT, createdate INTEGER, BankMoney INTEGER)",
		"CREATE TABLE guild_rank (guildid INTEGER, rid INTEGER, rname TEXT, rights INTEGER, BankMoneyPerDay INTEGER, PRIMARY KEY (guildid, rid))",
		"CREATE TABLE guild_member (guildid INTEGER, guid INTEGER PRIMARY KEY, rank INTEGER, pnote TEXT, offnote TEXT)",
		"CREATE TABLE petition (ownerguid INTEGER, petitionguid INTEGER, name TEXT, type INTEGER, PRIMARY KEY (ownerguid, type))",
		"CREATE TABLE petition_sign (ownerguid INTEGER, petitionguid INTEGER, playerguid INTEGER, player_account INTEGER, type INTEGER, PRIMARY KEY (petitionguid, playerguid))",
		"INSERT INTO characters VALUES (1, 'Founder', 500000)",
		"INSERT INTO characters VALUES (2, 'Signer1', 1000)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sessLeader := &session{server: srv, playerGUID: 1, accountID: 10, playerLoaded: true, player: &playerState{GUID: 1, Name: "Founder", Money: 500000}}
	sessSigner := &session{server: srv, playerGUID: 2, accountID: 11, playerLoaded: true, player: &playerState{GUID: 2, Name: "Signer1", Money: 1000}}

	ctx := context.Background()

	// 1. Show Petition List
	listBuf := protocol.NewBuffer(8)
	listBuf.WriteU64(1001)
	if !sessLeader.handlePetitionShowList(ctx, listBuf.Bytes()) {
		t.Fatal("handlePetitionShowList failed")
	}

	// 2. Buy Petition
	buyBuf := protocol.NewBuffer(64)
	buyBuf.WriteU64(1001) // npc
	buyBuf.WriteU32(0)
	buyBuf.WriteU64(0)
	buyBuf.WriteCString("The Vanguard")
	buyBuf.WriteU32(1)
	if !sessLeader.handlePetitionBuy(ctx, buyBuf.Bytes()) {
		t.Fatal("handlePetitionBuy failed")
	}

	var pName string
	var petitionGUID int64
	err = db.QueryRow("SELECT name, petitionguid FROM petition WHERE ownerguid = 1").Scan(&pName, &petitionGUID)
	if err != nil || pName != "The Vanguard" {
		t.Fatalf("petition buy not saved: %v, name: %s", err, pName)
	}

	// 3. Query Petition
	qBuf := protocol.NewBuffer(16)
	qBuf.WriteU32(uint32(petitionGUID & 0xFFFFFFFF))
	qBuf.WriteU64(uint64(petitionGUID))
	if !sessSigner.handlePetitionQuery(ctx, qBuf.Bytes()) {
		t.Fatal("handlePetitionQuery failed")
	}

	// 4. Sign Petition
	signBuf := protocol.NewBuffer(16)
	signBuf.WriteU64(uint64(petitionGUID))
	signBuf.WriteU8(0)
	if !sessSigner.handlePetitionSign(ctx, signBuf.Bytes()) {
		t.Fatal("handlePetitionSign failed")
	}

	// 5. Show Signatures
	showBuf := protocol.NewBuffer(8)
	showBuf.WriteU64(uint64(petitionGUID))
	if !sessLeader.handlePetitionShowSignatures(ctx, showBuf.Bytes()) {
		t.Fatal("handlePetitionShowSignatures failed")
	}

	// 6. Turn In Petition
	turnBuf := protocol.NewBuffer(8)
	turnBuf.WriteU64(uint64(petitionGUID))
	if !sessLeader.handleTurnInPetition(ctx, turnBuf.Bytes()) {
		t.Fatal("handleTurnInPetition failed")
	}

	var gCount int64
	_ = db.QueryRow("SELECT COUNT(*) FROM guild WHERE name = 'The Vanguard'").Scan(&gCount)
	if gCount != 1 {
		t.Fatalf("expected 1 guild, got %d", gCount)
	}

	// 7. Tabard Vendor & Emblem Save
	tvBuf := protocol.NewBuffer(8)
	tvBuf.WriteU64(1001)
	if !sessLeader.handleTabardVendorActivate(ctx, tvBuf.Bytes()) {
		t.Fatal("handleTabardVendorActivate failed")
	}

	emblemBuf := protocol.NewBuffer(32)
	emblemBuf.WriteU64(1001)
	emblemBuf.WriteU32(1) // style
	emblemBuf.WriteU32(2) // color
	emblemBuf.WriteU32(3) // bStyle
	emblemBuf.WriteU32(4) // bColor
	emblemBuf.WriteU32(5) // bgColor
	if !sessLeader.handleSaveGuildEmblem(ctx, emblemBuf.Bytes()) {
		t.Fatal("handleSaveGuildEmblem failed")
	}

	var style, color int64
	_ = db.QueryRow("SELECT EmblemStyle, EmblemColor FROM guild WHERE guildid = 1").Scan(&style, &color)
	if style != 1 || color != 2 {
		t.Fatalf("unexpected emblem values: style=%d, color=%d", style, color)
	}
}
