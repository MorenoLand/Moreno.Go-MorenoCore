//go:build ignore

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
	sessLeader := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "GMPlayer", Money: 1000}}
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

	// 6. Guild Info
	if !sessLeader.handleGuildInfo(ctx) {
		t.Fatal("handleGuildInfo failed")
	}

	// 7. Public & Officer Notes
	noteBuf := protocol.NewBuffer(64)
	noteBuf.WriteCString("Newbie")
	noteBuf.WriteCString("Great recruit")
	if !sessLeader.handleGuildSetPublicNote(ctx, noteBuf.Bytes()) {
		t.Fatal("handleGuildSetPublicNote failed")
	}
	offNoteBuf := protocol.NewBuffer(64)
	offNoteBuf.WriteCString("Newbie")
	offNoteBuf.WriteCString("Potential officer")
	if !sessLeader.handleGuildSetOfficerNote(ctx, offNoteBuf.Bytes()) {
		t.Fatal("handleGuildSetOfficerNote failed")
	}
	var pnote, offnote string
	_ = db.QueryRow("SELECT pnote, offnote FROM guild_member WHERE guid = 2").Scan(&pnote, &offnote)
	if pnote != "Great recruit" || offnote != "Potential officer" {
		t.Fatalf("unexpected notes: %q, %q", pnote, offnote)
	}

	// 8. Info Text
	infoBuf := protocol.NewBuffer(64)
	infoBuf.WriteCString("Updated guild information")
	if !sessLeader.handleGuildInfoText(ctx, infoBuf.Bytes()) {
		t.Fatal("handleGuildInfoText failed")
	}

	// 9. Add Rank & Del Rank
	rankBuf := protocol.NewBuffer(32)
	rankBuf.WriteCString("Raider")
	if !sessLeader.handleGuildAddRank(ctx, rankBuf.Bytes()) {
		t.Fatal("handleGuildAddRank failed")
	}
	_ = db.QueryRow("SELECT COUNT(*) FROM guild_rank WHERE guildid = 1").Scan(&count)
	if count != 3 {
		t.Fatalf("expected 3 ranks, got %d", count)
	}
	if !sessLeader.handleGuildDelRank(ctx) {
		t.Fatal("handleGuildDelRank failed")
	}

	// 10. Promote & Demote
	targetBuf := protocol.NewBuffer(32)
	targetBuf.WriteCString("Newbie")
	if !sessLeader.handleGuildPromote(ctx, targetBuf.Bytes()) {
		t.Fatal("handleGuildPromote failed")
	}
	if !sessLeader.handleGuildDemote(ctx, targetBuf.Bytes()) {
		t.Fatal("handleGuildDemote failed")
	}

	// 11. Guild Bank operations
	for _, stmt := range []string{
		"CREATE TABLE guild_bank_tab (guildid INTEGER, TabId INTEGER, TabName TEXT, TabIcon TEXT, TabText TEXT, PRIMARY KEY (guildid, TabId))",
		"CREATE TABLE guild_bank_item (guildid INTEGER, TabId INTEGER, SlotId INTEGER, item_guid INTEGER, PRIMARY KEY (guildid, TabId, SlotId))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER)",
		"INSERT INTO guild_bank_tab VALUES (1, 0, 'General', 'INV_Misc_Bag_08', 'Bank Rules')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	actBuf := protocol.NewBuffer(16)
	actBuf.WriteU64(1001) // Banker GUID
	actBuf.WriteU8(1)    // Full update
	if !sessLeader.handleGuildBankerActivate(ctx, actBuf.Bytes()) {
		t.Fatal("handleGuildBankerActivate failed")
	}

	qTabBuf := protocol.NewBuffer(16)
	qTabBuf.WriteU64(1001)
	qTabBuf.WriteU8(0) // Tab 0
	qTabBuf.WriteU8(1) // Full update
	if !sessLeader.handleGuildBankQueryTab(ctx, qTabBuf.Bytes()) {
		t.Fatal("handleGuildBankQueryTab failed")
	}

	// Deposit & Withdraw Money
	depBuf := protocol.NewBuffer(16)
	depBuf.WriteU64(1001)
	depBuf.WriteU32(500)
	if !sessLeader.handleGuildBankDepositMoney(ctx, depBuf.Bytes()) {
		t.Fatal("handleGuildBankDepositMoney failed")
	}
	if sessLeader.player.Money != 500 {
		t.Fatalf("expected player money 500, got %d", sessLeader.player.Money)
	}

	withBuf := protocol.NewBuffer(16)
	withBuf.WriteU64(1001)
	withBuf.WriteU32(200)
	if !sessLeader.handleGuildBankWithdrawMoney(ctx, withBuf.Bytes()) {
		t.Fatal("handleGuildBankWithdrawMoney failed")
	}
	if sessLeader.player.Money != 700 {
		t.Fatalf("expected player money 700, got %d", sessLeader.player.Money)
	}

	// Buy Tab & Update Tab
	buyBuf := protocol.NewBuffer(16)
	buyBuf.WriteU64(1001)
	buyBuf.WriteU8(1)
	if !sessLeader.handleGuildBankBuyTab(ctx, buyBuf.Bytes()) {
		t.Fatal("handleGuildBankBuyTab failed")
	}
	updBuf := protocol.NewBuffer(64)
	updBuf.WriteU64(1001)
	updBuf.WriteU8(1)
	updBuf.WriteCString("Raiding Gear")
	updBuf.WriteCString("INV_Sword_04")
	if !sessLeader.handleGuildBankUpdateTab(ctx, updBuf.Bytes()) {
		t.Fatal("handleGuildBankUpdateTab failed")
	}

	// Bank Text & Log Query
	if !sessLeader.handleQueryGuildBankText(ctx, []byte{0}) {
		t.Fatal("handleQueryGuildBankText failed")
	}
	setTxtBuf := protocol.NewBuffer(64)
	setTxtBuf.WriteU8(0)
	setTxtBuf.WriteCString("New Tab Description")
	if !sessLeader.handleSetGuildBankText(ctx, setTxtBuf.Bytes()) {
		t.Fatal("handleSetGuildBankText failed")
	}
	if !sessLeader.handleGuildBankLogQuery(ctx, []byte{0}) {
		t.Fatal("handleGuildBankLogQuery failed")
	}
	if !sessLeader.handleGuildBankMoneyWithdrawn(ctx, nil) {
		t.Fatal("handleGuildBankMoneyWithdrawn failed")
	}

	// 12. Slice 18 Misc Handlers
	if !sessLeader.handleDuelAccepted(ctx, nil) {
		t.Fatal("handleDuelAccepted failed")
	}
	if !sessLeader.handleDuelCancelled(ctx, nil) {
		t.Fatal("handleDuelCancelled failed")
	}
	mirrorBuf := protocol.NewBuffer(8)
	mirrorBuf.WriteU64(1)
	if !sessLeader.handleGetMirrorImageData(ctx, mirrorBuf.Bytes()) {
		t.Fatal("handleGetMirrorImageData failed")
	}
	if !sessLeader.handleFarSight(ctx, nil) || !sessLeader.handleForceMoveRootAck(ctx, nil) ||
		!sessLeader.handleForceMoveUnrootAck(ctx, nil) || !sessLeader.handleForceTurnRateChangeAck(ctx, nil) ||
		!sessLeader.handleGetChannelMemberCount(ctx, nil) || !sessLeader.handleGmTicketSystemToggle(ctx, nil) ||
		!sessLeader.handleGroupAssistantLeader(ctx, nil) || !sessLeader.handleGroupChangeSubGroup(ctx, nil) ||
		!sessLeader.handleDismissCritter(ctx, nil) || !sessLeader.handleChangeSeatsOnControlledVehicle(ctx, nil) ||
		!sessLeader.handleControllerEjectPassenger(ctx, nil) || !sessLeader.handleDismissControlledVehicle(ctx, nil) {
		t.Fatal("misc handlers returned false")
	}

	// 13. Disband
	if !sessLeader.handleGuildDisband(ctx) {
		t.Fatal("handleGuildDisband failed")
	}
	_ = db.QueryRow("SELECT COUNT(*) FROM guild WHERE guildid = 1").Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 guilds after disband, got %d", count)
	}
}

