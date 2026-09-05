package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildQuestGiverRequestItems(t *testing.T) {
	view := questRewardView{Detail: questDetailData{ID: 9, Title: "Quest", Flags: 10, SuggestedGroupNum: 11}, RequestText: "Bring it", RequiredItems: []questRewardItem{{ID: 12, Quantity: 13, DisplayID: 14}}}
	reader := protocol.NewReader(buildQuestGiverRequestItems(view, 15, 16, true, false))
	for _, expected := range []uint64{15} {
		if value, err := reader.ReadU64(); err != nil || value != expected {
			t.Fatalf("giver=%d err=%v", value, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != view.Detail.ID {
		t.Fatalf("quest=%d err=%v", value, err)
	}
	for _, expected := range []string{view.Detail.Title, view.RequestText} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	for _, expected := range []uint32{0, 16, 0, view.Detail.Flags, view.Detail.SuggestedGroupNum, 0, 1, 12, 13, 14, 3, 4, 8, 16} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
}

func TestBuildQuestGiverOfferReward(t *testing.T) {
	view := questRewardView{Detail: questDetailData{ID: 20, Title: "Quest", Flags: 21, SuggestedGroupNum: 22, ChoiceItems: []questRewardItem{{ID: 23, Quantity: 24, DisplayID: 25}}, RewardItems: []questRewardItem{{ID: 26, Quantity: 27, DisplayID: 28}}, RewardMoney: 29, RewardXPDifficulty: 30, RewardHonor: 31, RewardKillHonor: 1.5, RewardDisplaySpell: 32, RewardSpell: -33, RewardTitleID: 34, RewardTalents: 35, RewardArenaPoints: -36, RewardFactionID: [questRewardFactions]uint32{37}, RewardFactionValue: [questRewardFactions]int32{-42}, RewardFactionOverride: [questRewardFactions]int32{-47}}, RewardText: "Reward", OfferEmotes: []questDescEmote{{Type: 38, Delay: 39}}}
	reader := protocol.NewReader(buildQuestGiverOfferReward(view, 40, true))
	if value, err := reader.ReadU64(); err != nil || value != 40 {
		t.Fatalf("giver=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != view.Detail.ID {
		t.Fatalf("quest=%d err=%v", value, err)
	}
	for _, expected := range []string{view.Detail.Title, view.RewardText} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("auto=%d err=%v", value, err)
	}
	for _, expected := range []uint32{view.Detail.Flags, view.Detail.SuggestedGroupNum, 1, 39, 38, 1, 23, 24, 25, 1, 26, 27, 28, view.Detail.RewardMoney, view.Detail.RewardXPDifficulty, view.Detail.RewardHonor} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadF32(); err != nil || value != view.Detail.RewardKillHonor {
		t.Fatalf("honor=%f err=%v", value, err)
	}
	for _, expected := range []int32{int32(view.Detail.RewardDisplaySpell), view.Detail.RewardSpell, int32(view.Detail.RewardTitleID), int32(view.Detail.RewardTalents), view.Detail.RewardArenaPoints} {
		if value, err := reader.ReadI32(); err != nil || value != expected {
			t.Fatalf("signed=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("faction flags=%d err=%v", value, err)
	}
	for index := 0; index < questRewardFactions; index++ {
		if value, err := reader.ReadU32(); err != nil || value != view.Detail.RewardFactionID[index] {
			t.Fatalf("faction id=%d err=%v", value, err)
		}
	}
	for index := 0; index < questRewardFactions; index++ {
		if value, err := reader.ReadI32(); err != nil || value != view.Detail.RewardFactionValue[index] {
			t.Fatalf("faction value=%d err=%v", value, err)
		}
	}
	for index := 0; index < questRewardFactions; index++ {
		if value, err := reader.ReadI32(); err != nil || value != view.Detail.RewardFactionOverride[index] {
			t.Fatalf("faction override=%d err=%v", value, err)
		}
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining=%d", reader.Remaining())
	}
}

func TestBuildQuestRewardPackets(t *testing.T) {
	reader := protocol.NewReader(buildQuestRewardComplete(1, 2, 3, 4, 5, 6))
	for expected := uint32(1); expected <= 6; expected++ {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining=%d", reader.Remaining())
	}
	reader = protocol.NewReader(buildItemPushResult(7, 0, 23, 8, 9, 10, false))
	if value, err := reader.ReadU64(); err != nil || value != 7 {
		t.Fatalf("guid=%d err=%v", value, err)
	}
	for _, expected := range []uint32{1, 0, 0} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("header=%d expected=%d err=%v", value, expected, err)
		}
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []uint32{23, 8, 0, 0, 9, 10} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("item=%d expected=%d err=%v", value, expected, err)
		}
	}
}

func TestQuestItemConsumptionAndRewardInEquippedBags(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, account INTEGER, money INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER, PRIMARY KEY (guid, bag, slot))",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, owner_guid INTEGER, creatorGuid INTEGER, giftCreatorGuid INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, durability INTEGER, playedTime INTEGER, text TEXT)",
		"CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER, explored INTEGER, timer INTEGER, PRIMARY KEY (guid, quest))",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER, quest INTEGER, active INTEGER, PRIMARY KEY (guid, quest))",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, stackable INTEGER, ContainerSlots INTEGER, MaxDurability INTEGER)",
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER, map INTEGER)",
		"CREATE TABLE creature_queststarter (id INTEGER, quest INTEGER, PRIMARY KEY (id, quest))",
		"CREATE TABLE creature_questender (id INTEGER, quest INTEGER, PRIMARY KEY (id, quest))",
		"CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, LogTitle TEXT, LogDescription TEXT, QuestDescription TEXT, Flags INTEGER, SuggestedGroupNum INTEGER, RewardMoney INTEGER, RewardXPDifficulty INTEGER, RewardBonusMoney INTEGER, RewardDisplaySpell INTEGER, RewardSpell INTEGER, RewardHonor INTEGER, RewardKillHonor REAL, RewardTitle INTEGER, RewardTalents INTEGER, RewardArenaPoints INTEGER, RewardItem1 INTEGER, RewardAmount1 INTEGER, RewardItem2 INTEGER, RewardAmount2 INTEGER, RewardItem3 INTEGER, RewardAmount3 INTEGER, RewardItem4 INTEGER, RewardAmount4 INTEGER, RequiredItemId1 INTEGER, RequiredItemCount1 INTEGER, RequiredItemId2 INTEGER, RequiredItemCount2 INTEGER, RequiredItemId3 INTEGER, RequiredItemCount3 INTEGER, RequiredItemId4 INTEGER, RequiredItemCount4 INTEGER, RequiredItemId5 INTEGER, RequiredItemCount5 INTEGER, RequiredItemId6 INTEGER, RequiredItemCount6 INTEGER, RewardChoiceItemID1 INTEGER, RewardChoiceItemQuantity1 INTEGER, RewardChoiceItemID2 INTEGER, RewardChoiceItemQuantity2 INTEGER, RewardChoiceItemID3 INTEGER, RewardChoiceItemQuantity3 INTEGER, RewardChoiceItemID4 INTEGER, RewardChoiceItemQuantity4 INTEGER, RewardChoiceItemID5 INTEGER, RewardChoiceItemQuantity5 INTEGER, RewardChoiceItemID6 INTEGER, RewardChoiceItemQuantity6 INTEGER, RewardFactionID1 INTEGER, RewardFactionValue1 INTEGER, RewardFactionOverride1 INTEGER, RewardFactionID2 INTEGER, RewardFactionValue2 INTEGER, RewardFactionOverride2 INTEGER, RewardFactionID3 INTEGER, RewardFactionValue3 INTEGER, RewardFactionOverride3 INTEGER, RewardFactionID4 INTEGER, RewardFactionValue4 INTEGER, RewardFactionOverride4 INTEGER, RewardFactionID5 INTEGER, RewardFactionValue5 INTEGER, RewardFactionOverride5 INTEGER)",
		"CREATE TABLE quest_request_items (ID INTEGER PRIMARY KEY, EmoteOnComplete INTEGER, EmoteOnIncomplete INTEGER, CompletionText TEXT)",
		"CREATE TABLE quest_offer_reward (ID INTEGER PRIMARY KEY, Emote1 INTEGER, Emote2 INTEGER, Emote3 INTEGER, Emote4 INTEGER, EmoteDelay1 INTEGER, EmoteDelay2 INTEGER, EmoteDelay3 INTEGER, EmoteDelay4 INTEGER, RewardText TEXT)",

		// Setup player 1 with 100 copper
		"INSERT INTO characters VALUES (1, 1, 100)",

		// Quest 55: requires 5 of item 8888, rewards 1 of item 9999 and 50 copper
		"INSERT INTO quest_template (ID, LogTitle, LogDescription, QuestDescription, Flags, SuggestedGroupNum, RewardMoney, RewardXPDifficulty, RewardBonusMoney, RewardDisplaySpell, RewardSpell, RewardHonor, RewardKillHonor, RewardTitle, RewardTalents, RewardArenaPoints, RewardItem1, RewardAmount1, RewardItem2, RewardAmount2, RewardItem3, RewardAmount3, RewardItem4, RewardAmount4, RequiredItemId1, RequiredItemCount1, RequiredItemId2, RequiredItemCount2, RequiredItemId3, RequiredItemCount3, RequiredItemId4, RequiredItemCount4, RequiredItemId5, RequiredItemCount5, RequiredItemId6, RequiredItemCount6, RewardChoiceItemID1, RewardChoiceItemQuantity1, RewardChoiceItemID2, RewardChoiceItemQuantity2, RewardChoiceItemID3, RewardChoiceItemQuantity3, RewardChoiceItemID4, RewardChoiceItemQuantity4, RewardChoiceItemID5, RewardChoiceItemQuantity5, RewardChoiceItemID6, RewardChoiceItemQuantity6, RewardFactionID1, RewardFactionValue1, RewardFactionOverride1, RewardFactionID2, RewardFactionValue2, RewardFactionOverride2, RewardFactionID3, RewardFactionValue3, RewardFactionOverride3, RewardFactionID4, RewardFactionValue4, RewardFactionOverride4, RewardFactionID5, RewardFactionValue5, RewardFactionOverride5) VALUES (55, 'Bag Quest', 'Turn in item', 'Details', 0, 0, 50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9999, 1, 0, 0, 0, 0, 0, 0, 8888, 5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)",
		"INSERT INTO quest_request_items VALUES (55, 0, 0, 'Got the items?')",
		"INSERT INTO quest_offer_reward VALUES (55, 0, 0, 0, 0, 0, 0, 0, 0, 'Thanks!')",
		"INSERT INTO character_queststatus VALUES (1, 55, 1, 1, 0)", // Complete (status=1)

		// Questgiver creature 200
		"INSERT INTO creature VALUES (1, 200, 0)",
		"INSERT INTO creature_queststarter VALUES (200, 55)",
		"INSERT INTO creature_questender VALUES (200, 55)",

		// Templates for items
		"INSERT INTO item_template VALUES (4500, 1, 4, 0)", // 4-slot container bag
		"INSERT INTO item_template VALUES (8888, 20, 0, 0)", // Quest item (stackable 20)
		"INSERT INTO item_template VALUES (9999, 1, 0, 0)",  // Reward item
		"INSERT INTO item_template VALUES (7777, 1, 0, 0)",  // Dummy item filling backpack

		// Equip 4-slot bag at slot 19 (bag GUID 500)
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count) VALUES (500, 4500, 1, 0, 1)",
		"INSERT INTO character_inventory VALUES (1, 0, 19, 500)",

		// Quest items located INSIDE equipped bag (bagKey 500, slot 0, item GUID 501, entry 8888, count 5)
		"INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count) VALUES (501, 8888, 1, 0, 5)",
		"INSERT INTO character_inventory VALUES (1, 500, 0, 501)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup failed on %s: %v", stmt, err)
		}
	}

	// Completely fill the backpack (bag 0, slots 23..38) with dummy items (item GUIDs 600..615)
	for sl := uint8(23); sl <= 38; sl++ {
		itemGUID := 600 + int(sl)
		if _, err := db.Exec("INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count) VALUES (?, 7777, 1, 0, 1)", itemGUID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO character_inventory VALUES (1, 0, ?, ?)", sl, itemGUID); err != nil {
			t.Fatal(err)
		}
	}

	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		AuthStore:       store,
		CharactersStore: store,
		WorldStore:      store,
		sessions:        make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         sConn,
		accountName:  "testuser",
		accountID:    1,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:  1,
			Level: 10,
			Money: 100,
			Map:   0,
		},
	}

	giverGUID := creatureWorldGUID(1, 200)

	// Turn in quest: choose reward 0
	choosePayload := protocol.NewBuffer(16)
	choosePayload.WriteU64(giverGUID)
	choosePayload.WriteU32(55)
	choosePayload.WriteU32(0)

	type serverPkt struct {
		opcode uint16
		data   []byte
	}
	receivedPkts := make(chan serverPkt, 64)
	go func() {
		for {
			op, data, err := readServerFrame(cConn, nil)
			if err != nil {
				return
			}
			receivedPkts <- serverPkt{opcode: op, data: data}
		}
	}()

	done := make(chan struct{})
	go func() {
		sess.handleQuestgiverChooseReward(context.Background(), choosePayload.Bytes())
		close(done)
	}()

	// 1. Must receive SMSG_DESTROY_OBJECT for consumed quest item 501
	pkt1 := <-receivedPkts
	if pkt1.opcode != uint16(protocol.OpcodeSMSG_DESTROY_OBJECT) {
		t.Fatalf("expected SMSG_DESTROY_OBJECT (0x0AA), got 0x%04X", pkt1.opcode)
	}
	rdDestroy := protocol.NewReader(pkt1.data)
	destGUID, _ := rdDestroy.ReadU64()
	expectedFullGUID := uint64(501) | (uint64(0x4000) << 48)
	if destGUID != expectedFullGUID {
		t.Fatalf("expected destroyed full GUID 0x%016X, got 0x%016X", expectedFullGUID, destGUID)
	}

	// 2. May receive SMSG_UPDATE_OBJECT from despawnItem
	pkt2 := <-receivedPkts
	if pkt2.opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) {
		t.Fatalf("expected SMSG_UPDATE_OBJECT from despawnItem, got 0x%04X", pkt2.opcode)
	}

	// 3. Must receive SMSG_ITEM_PUSH_RESULT for reward item 9999
	// Since backpack was full, it must go into equipped bag (clientBag = 19, slot = 0 because 501 was consumed and freed slot 0!)
	pkt3 := <-receivedPkts
	if pkt3.opcode != uint16(protocol.OpcodeSMSG_ITEM_PUSH_RESULT) {
		t.Fatalf("expected SMSG_ITEM_PUSH_RESULT (0x166), got 0x%04X", pkt3.opcode)
	}
	rdPush := protocol.NewReader(pkt3.data)
	_, _ = rdPush.ReadU64() // playerGUID
	_, _ = rdPush.ReadU32() // 1
	_, _ = rdPush.ReadU32() // 0
	_, _ = rdPush.ReadU32() // 0
	pushBag, _ := rdPush.ReadU8()
	pushSlot, _ := rdPush.ReadU32()
	pushEntry, _ := rdPush.ReadU32()
	if pushBag != 19 {
		t.Fatalf("expected reward in equipped bag 19, got bag %d", pushBag)
	}
	if pushSlot != 0 {
		t.Fatalf("expected reward in freed slot 0 of bag 19, got slot %d", pushSlot)
	}
	if pushEntry != 9999 {
		t.Fatalf("expected reward item 9999, got entry %d", pushEntry)
	}

	// 4. SMSG_QUESTGIVER_QUEST_COMPLETE
	pkt4 := <-receivedPkts
	if pkt4.opcode != uint16(protocol.OpcodeSMSG_QUESTGIVER_QUEST_COMPLETE) {
		t.Fatalf("expected SMSG_QUESTGIVER_QUEST_COMPLETE, got 0x%04X", pkt4.opcode)
	}

	<-done

	// Verify in DB that item 501 was deleted
	var remainingCount int
	_ = db.QueryRow("SELECT COUNT(1) FROM item_instance WHERE guid = 501").Scan(&remainingCount)
	if remainingCount != 0 {
		t.Fatalf("expected consumed quest item 501 to be deleted from item_instance")
	}

	// Verify reward item 9999 is stored in character_inventory at bag 500, slot 0
	var storedEntry int
	err = db.QueryRow("SELECT ii.itemEntry FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = 1 AND ci.bag = 500 AND ci.slot = 0").Scan(&storedEntry)
	if err != nil || storedEntry != 9999 {
		t.Fatalf("expected reward item 9999 in bag 500 slot 0, got err=%v entry=%d", err, storedEntry)
	}

	// Verify player received money: 100 + 50 = 150
	if sess.player.Money != 150 {
		t.Fatalf("expected player money 150, got %d", sess.player.Money)
	}

	// Verify quest is marked as rewarded
	var rewarded int
	_ = db.QueryRow("SELECT active FROM character_queststatus_rewarded WHERE guid = 1 AND quest = 55").Scan(&rewarded)
	if rewarded != 1 {
		t.Fatalf("expected quest 55 to be marked as rewarded")
	}
}
