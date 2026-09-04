package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

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
	actBuf.WriteU8(1)     // Full update
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
	sessLeader.player.Money += 300 * 10000 // 300 gold
	buyBuf := protocol.NewBuffer(16)
	buyBuf.WriteU64(1001)
	buyBuf.WriteU8(1) // Tab 1 costs 250 gold
	if !sessLeader.handleGuildBankBuyTab(ctx, buyBuf.Bytes()) {
		t.Fatal("handleGuildBankBuyTab failed")
	}
	if sessLeader.player.Money != 700+50*10000 { // 300g - 250g = 50g remaining
		t.Fatalf("expected player money %d, got %d", 700+50*10000, sessLeader.player.Money)
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

func TestGuildBankEventLogsAndTabQueryParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, level INTEGER, class INTEGER, gender INTEGER, zone INTEGER, money INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, played_time INTEGER, text TEXT)",
		"CREATE TABLE guild (guildid INTEGER PRIMARY KEY, name TEXT, leaderguid INTEGER, EmblemStyle INTEGER, EmblemColor INTEGER, BorderStyle INTEGER, BorderColor INTEGER, BackgroundColor INTEGER, info TEXT, motd TEXT, createdate INTEGER, BankMoney INTEGER)",
		"CREATE TABLE guild_rank (guildid INTEGER, rid INTEGER, rname TEXT, rights INTEGER, BankMoneyPerDay INTEGER, PRIMARY KEY (guildid, rid))",
		"CREATE TABLE guild_bank_right (guildid INTEGER, TabId INTEGER, rid INTEGER, gbright INTEGER, SlotPerDay INTEGER, PRIMARY KEY (guildid, TabId, rid))",
		"CREATE TABLE guild_bank_tab (guildid INTEGER, TabId INTEGER, TabName TEXT, TabIcon TEXT, TabText TEXT, PRIMARY KEY (guildid, TabId))",
		"CREATE TABLE guild_bank_item (guildid INTEGER, TabId INTEGER, SlotId INTEGER, item_guid INTEGER, PRIMARY KEY (guildid, TabId, SlotId))",
		"CREATE TABLE guild_bank_eventlog (guildid INTEGER, LogGuid INTEGER, TabId INTEGER, EventType INTEGER, PlayerGuid INTEGER, ItemOrMoney INTEGER, ItemStackCount INTEGER, DestTabId INTEGER, TimeStamp INTEGER, PRIMARY KEY (guildid, LogGuid, TabId))",
		"CREATE TABLE guild_member (guildid INTEGER, guid INTEGER PRIMARY KEY, rank INTEGER, pnote TEXT, offnote TEXT)",
		"CREATE TABLE guild_member_withdraw (guid INTEGER PRIMARY KEY, tab0 INTEGER, tab1 INTEGER, tab2 INTEGER, tab3 INTEGER, tab4 INTEGER, tab5 INTEGER, money INTEGER)",
		"INSERT INTO characters VALUES (1, 'GMLeader', 80, 1, 0, 12, 100000)",
		"INSERT INTO characters VALUES (2, 'OfficerMember', 80, 2, 1, 12, 5000)",
		"INSERT INTO guild VALUES (10, 'ParityGuild', 1, 0, 0, 0, 0, 0, '', '', 0, 0)",
		"INSERT INTO guild_member VALUES (10, 1, 0, '', '')", // Rank 0 = Guild Master
		"INSERT INTO guild_member VALUES (10, 2, 1, '', '')", // Rank 1 = Officer
		"INSERT INTO guild_rank VALUES (10, 0, 'Guild Master', 0x001DF1FF, 0)",
		"INSERT INTO guild_rank VALUES (10, 1, 'Officer', 0x001DF1FF, 1000)", // 1000 copper per day
		"INSERT INTO guild_bank_tab VALUES (10, 0, 'Tab 1', 'INV_Misc_Bag_08', '')",
		"INSERT INTO guild_bank_tab VALUES (10, 1, 'Tab 2', 'INV_Misc_Bag_08', '')",
		// Tab 0 rights for Officer (rid 1): gbright=3 (view+deposit), SlotPerDay=1
		"INSERT INTO guild_bank_right VALUES (10, 0, 1, 3, 1)",
		// Tab 1 rights for Officer (rid 1): gbright=3 (view+deposit), SlotPerDay=1
		"INSERT INTO guild_bank_right VALUES (10, 1, 1, 3, 1)",
		// Item in Leader's inventory: item 101, itemEntry 5001, count 1
		"INSERT INTO item_instance VALUES (101, 5001, 1, 0, 1, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 101)",
		// Item in Officer's inventory: item 102, itemEntry 5002, count 5
		"INSERT INTO item_instance VALUES (102, 5002, 2, 0, 5, 0, '', 0, '', 0, 100, 0, '')",
		"INSERT INTO character_inventory VALUES (2, 0, 23, 102)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	charStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: charStore, WorldStore: charStore, sessions: make(map[*session]struct{})}

	serverConnLeader, clientConnLeader := net.Pipe()
	defer serverConnLeader.Close()
	defer clientConnLeader.Close()

	serverConnOfficer, clientConnOfficer := net.Pipe()
	defer serverConnOfficer.Close()
	defer clientConnOfficer.Close()

	sessLeader := &session{conn: serverConnLeader, server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "GMLeader", GuildID: 10, Money: 100000}}
	sessOfficer := &session{conn: serverConnOfficer, server: srv, playerGUID: 2, playerLoaded: true, player: &playerState{GUID: 2, Name: "OfficerMember", GuildID: 10, Money: 5000}}
	srv.sessions[sessLeader] = struct{}{}
	srv.sessions[sessOfficer] = struct{}{}

	ctx := context.Background()

	type packetData struct {
		opcode  uint16
		payload []byte
	}
	pktChanOfficer := make(chan packetData, 10)
	go func() {
		for {
			header := make([]byte, 4)
			if _, err := clientConnOfficer.Read(header); err != nil {
				return
			}
			sz := int(header[0])<<8 | int(header[1])
			op := uint16(header[2]) | uint16(header[3])<<8
			payload := make([]byte, sz-2)
			if _, err := clientConnOfficer.Read(payload); err != nil {
				return
			}
			pktChanOfficer <- packetData{opcode: op, payload: payload}
		}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := clientConnLeader.Read(buf); err != nil {
				return
			}
		}
	}()

	// 1. Leader deposits money (5000)
	depBuf := protocol.NewBuffer(16)
	depBuf.WriteU64(1001) // Banker GUID
	depBuf.WriteU32(5000)
	if !sessLeader.handleGuildBankDepositMoney(ctx, depBuf.Bytes()) {
		t.Fatal("handleGuildBankDepositMoney failed")
	}

	// 2. Leader deposits item 101 from bag 0 slot 23 into bank tab 0 slot 0
	swapBuf := protocol.NewBuffer(32)
	swapBuf.WriteU64(1001) // bankerGUID
	swapBuf.WriteU8(0)     // bankOnly = 0
	swapBuf.WriteU8(0)     // bankTab = 0
	swapBuf.WriteU8(0)     // bankSlot = 0
	swapBuf.WriteU32(5001) // itemID
	swapBuf.WriteU8(0)     // autoStore = 0
	swapBuf.WriteU8(0)     // containerSlot = 0
	swapBuf.WriteU8(23)    // containerItemSlot = 23
	swapBuf.WriteU8(0)     // toSlot = 0 (Deposit)
	swapBuf.WriteU32(1)    // stackCount = 1
	if !sessLeader.handleGuildBankSwapItems(ctx, swapBuf.Bytes()) {
		t.Fatal("handleGuildBankSwapItems deposit failed")
	}

	// 3. Officer withdraws item from tab 0 slot 0 into bag 0 slot 23
	withItemBuf := protocol.NewBuffer(32)
	withItemBuf.WriteU64(1001)
	withItemBuf.WriteU8(0)
	withItemBuf.WriteU8(0) // bankTab = 0
	withItemBuf.WriteU8(0) // bankSlot = 0
	withItemBuf.WriteU32(5001)
	withItemBuf.WriteU8(0)
	withItemBuf.WriteU8(0)  // containerSlot
	withItemBuf.WriteU8(23) // containerItemSlot
	withItemBuf.WriteU8(1)  // toSlot = 1 (Withdraw)
	withItemBuf.WriteU32(1)
	if !sessOfficer.handleGuildBankSwapItems(ctx, withItemBuf.Bytes()) {
		t.Fatal("handleGuildBankSwapItems withdraw failed")
	}

	// 4. Officer tries to withdraw 2nd item from tab 0 (limit is 1 per day) -> rejected
	// First put an item in tab 0 slot 1 so there's an item to withdraw
	_, _ = db.Exec("INSERT INTO guild_bank_item VALUES (10, 0, 1, 102)")
	withItemBuf2 := protocol.NewBuffer(32)
	withItemBuf2.WriteU64(1001)
	withItemBuf2.WriteU8(0)
	withItemBuf2.WriteU8(0)
	withItemBuf2.WriteU8(1) // slot 1
	withItemBuf2.WriteU32(5002)
	withItemBuf2.WriteU8(0)
	withItemBuf2.WriteU8(0)
	withItemBuf2.WriteU8(24)
	withItemBuf2.WriteU8(1)
	withItemBuf2.WriteU32(1)
	_ = sessOfficer.handleGuildBankSwapItems(ctx, withItemBuf2.Bytes())

	// Item 102 should still be in the bank because Officer's withdraw limit was 1
	var itemInBank uint64
	_ = db.QueryRow("SELECT item_guid FROM guild_bank_item WHERE guildid = 10 AND TabId = 0 AND SlotId = 1").Scan(&itemInBank)
	if itemInBank != 102 {
		t.Fatalf("expected item 102 to remain in bank due to withdraw limit, got %d", itemInBank)
	}

	// 5. Officer withdraws 400 money (within 1000 limit)
	withMoneyBuf := protocol.NewBuffer(16)
	withMoneyBuf.WriteU64(1001)
	withMoneyBuf.WriteU32(400)
	if !sessOfficer.handleGuildBankWithdrawMoney(ctx, withMoneyBuf.Bytes()) {
		t.Fatal("handleGuildBankWithdrawMoney failed")
	}
	if sessOfficer.player.Money != 5400 {
		t.Fatalf("expected officer money 5400, got %d", sessOfficer.player.Money)
	}

	// 6. Officer tries to withdraw 700 money (400 + 700 = 1100 > 1000 limit) -> rejected
	withMoneyBuf2 := protocol.NewBuffer(16)
	withMoneyBuf2.WriteU64(1001)
	withMoneyBuf2.WriteU32(700)
	_ = sessOfficer.handleGuildBankWithdrawMoney(ctx, withMoneyBuf2.Bytes())
	if sessOfficer.player.Money != 5400 {
		t.Fatalf("expected officer money unchanged at 5400, got %d", sessOfficer.player.Money)
	}

	// 7. Officer checks remaining money
	if !sessOfficer.handleGuildBankMoneyWithdrawn(ctx, nil) {
		t.Fatal("handleGuildBankMoneyWithdrawn failed")
	}
	select {
	case pkt := <-pktChanOfficer:
		if pkt.opcode == uint16(protocol.OpcodeMSG_GUILD_BANK_MONEY_WITHDRAWN) {
			r := protocol.NewReader(pkt.payload)
			rem, err := r.ReadI32()
			if err != nil || rem != 600 { // 1000 - 400 = 600 remaining
				t.Fatalf("expected remaining money 600, got %d, err %v", rem, err)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for MSG_GUILD_BANK_MONEY_WITHDRAWN")
	}

	// 8. Officer queries item tab log (tabID = 0)
	if !sessOfficer.handleGuildBankLogQuery(ctx, []byte{0}) {
		t.Fatal("handleGuildBankLogQuery tab 0 failed")
	}
	select {
	case pkt := <-pktChanOfficer:
		if pkt.opcode == uint16(protocol.OpcodeMSG_GUILD_BANK_LOG_QUERY) {
			r := protocol.NewReader(pkt.payload)
			tabID, err := r.ReadU8()
			if err != nil || tabID != 0 {
				t.Fatalf("expected tabID 0, got %d, err %v", tabID, err)
			}
			count, err := r.ReadU8()
			if err != nil || count < 2 { // deposit + withdraw
				t.Fatalf("expected at least 2 log entries on tab 0, got %d, err %v", count, err)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for MSG_GUILD_BANK_LOG_QUERY tab 0")
	}

	// 9. Officer queries money log (tabID = 6)
	if !sessOfficer.handleGuildBankLogQuery(ctx, []byte{6}) {
		t.Fatal("handleGuildBankLogQuery tab 6 failed")
	}
	select {
	case pkt := <-pktChanOfficer:
		if pkt.opcode == uint16(protocol.OpcodeMSG_GUILD_BANK_LOG_QUERY) {
			r := protocol.NewReader(pkt.payload)
			tabID, err := r.ReadU8()
			if err != nil || tabID != 6 {
				t.Fatalf("expected tabID 6 for money log, got %d, err %v", tabID, err)
			}
			count, err := r.ReadU8()
			if err != nil || count < 2 { // deposit (5000) + withdraw (400)
				t.Fatalf("expected at least 2 log entries on money tab, got %d, err %v", count, err)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for MSG_GUILD_BANK_LOG_QUERY tab 6")
	}
}
