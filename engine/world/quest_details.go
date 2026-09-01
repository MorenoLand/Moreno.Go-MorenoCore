package world

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	questRewardItems       = 4
	questChoiceItems       = 6
	questRewardFactions    = 5
	questDetailEmotes      = 4
	questAutoCompleteFlags = 0x00010000
)

type questRewardItem struct {
	ID        uint32
	Quantity  uint32
	DisplayID uint32
}

type questDescEmote struct {
	Type  uint32
	Delay uint32
}

type questDetailData struct {
	ID                    uint32
	Title                 string
	Objectives            string
	Details               string
	Flags                 uint32
	SuggestedGroupNum     uint32
	RewardMoney           uint32
	RewardXPDifficulty    uint32
	RewardBonusMoney      uint32
	RewardDisplaySpell    uint32
	RewardSpell           int32
	RewardHonor           uint32
	RewardKillHonor       float32
	RewardTitleID         uint32
	RewardTalents         uint32
	RewardArenaPoints     int32
	RewardItems           []questRewardItem
	ChoiceItems           []questRewardItem
	RewardFactionID       [questRewardFactions]uint32
	RewardFactionValue    [questRewardFactions]int32
	RewardFactionOverride [questRewardFactions]int32
	DescEmotes            [questDetailEmotes]questDescEmote
}

func (s *session) handleQuestgiverQueryQuest(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	questID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if len(payload) > 12 {
		if _, err := reader.ReadU8(); err != nil {
			return false
		}
	}
	creature := s.luaCreature(ctx, guid)
	var creatureEntry uint32
	if creature != nil {
		creatureEntry = objectUint32OrZero(creature, "Entry")
	} else {
		// Fallback: extract entry from packed GUID or look up spawn GUID
		creatureEntry = uint32((guid >> 24) & 0x00FFFFFF)
		if creatureEntry == 0 {
			spawnGUID := uint32(guid & 0x00FFFFFF)
			_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT id FROM creature WHERE guid = ?", spawnGUID).Scan(&creatureEntry)
		}
	}
	if creatureEntry == 0 || !s.creatureHasQuest(ctx, creatureEntry, questID) {
		return s.sendGossipComplete()
	}
	data, err := s.loadQuestDetailData(ctx, questID)
	if errors.Is(err, sql.ErrNoRows) {
		return s.sendGossipComplete()
	}
	if err != nil {
		s.debug("quest details failed", "account", s.accountName, "quest", questID, "error", err)
		return false
	}
	packet := buildQuestGiverDetails(data, guid, 0)
	s.debug("quest details response", "account", s.accountName, "quest", questID)
	return s.write(uint16(protocol.OpcodeSMSG_QUEST_GIVER_QUEST_DETAILS), packet, true) == nil
}

func (s *session) handleQuestgiverAcceptQuest(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	questID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if len(payload) > 12 {
		if _, err := reader.ReadU32(); err != nil {
			return false
		}
	}
	if !s.questgiverStartsQuest(ctx, guid, questID) {
		s.debug("quest accept rejected", "account", s.accountName, "quest", questID, "guid", guid, "reason", "questgiver does not start quest")
		return true
	}
	// TrinityCore re-validates CanTakeQuest on accept before AddQuest.
	canTake, err := s.canTakeQuest(ctx, questID)
	if err != nil || !canTake {
		s.debug("quest accept rejected", "account", s.accountName, "quest", questID, "reason", "canTakeQuest failed", "error", err)
		return s.sendGossipComplete()
	}
	query := "INSERT OR IGNORE INTO character_queststatus (guid, quest, status) VALUES (?, ?, ?)"
	if s.server.CharactersStore.Backend != database.BackendSQLite {
		query = "INSERT IGNORE INTO character_queststatus (guid, quest, status) VALUES (?, ?, ?)"
	}
	if _, err := s.server.CharactersStore.DB.ExecContext(ctx, query, s.playerGUID, questID, questStatusIncomplete); err != nil {
		if missingTable(err) {
			return true
		}
		s.debug("quest accept failed", "account", s.accountName, "quest", questID, "error", err)
		return false
	}
	// Player::AddQuest claims a free log slot and updates PLAYER_QUEST_LOG
	// so the client shows the quest immediately; the field change rides a
	// values update, not an object re-create.
	if s.player != nil {
		for slot := 0; slot < playerQuestLogSlots; slot++ {
			if s.player.QuestLog[slot].QuestID == 0 {
				s.player.QuestLog[slot] = questLogEntry{QuestID: questID}
				s.sendPlayerQuestLogUpdate(slot)
				break
			}
		}
	}

	// Grant starter item if specified in quest_template (e.g. quest items, letters, containers)
	var startItem int64
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT StartItem FROM quest_template WHERE ID = ?", questID).Scan(&startItem); err == nil && startItem > 0 {
		s.grantQuestStartItem(ctx, uint32(startItem))
	}

	s.debug("quest accepted", "account", s.accountName, "quest", questID)
	return s.sendGossipComplete()
}

func (s *session) grantQuestStartItem(ctx context.Context, itemEntry uint32) {
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return
	}
	usedSlots := make(map[uint8]bool)
	rows, err := cdb.QueryContext(ctx, "SELECT slot FROM character_inventory WHERE guid = ? AND bag = 0", s.playerGUID)
	if err == nil {
		for rows.Next() {
			var sl int64
			if rows.Scan(&sl) == nil {
				usedSlots[uint8(sl)] = true
			}
		}
		rows.Close()
	}
	freeSlot := uint8(0xFF)
	for sl := uint8(23); sl <= 38; sl++ {
		if !usedSlots[sl] {
			freeSlot = sl
			break
		}
	}
	if freeSlot == 0xFF {
		return
	}
	var nextGUID uint64
	if err := cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&nextGUID); err != nil || nextGUID == 0 {
		nextGUID = uint64(time.Now().UnixNano())
	}
	_, _ = cdb.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text) VALUES (?, ?, ?, 0, 1, 0, 0, 0, '', 0, 100, 0, '')", nextGUID, itemEntry, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)", s.playerGUID, freeSlot, nextGUID)
	_ = s.sendInventoryItems(ctx)
	s.sendPlayerUpdate()
}

func (s *session) handleQuestgiverCancel() bool {
	s.gossip = nil
	s.gossipClosed = true
	return s.write(uint16(protocol.OpcodeSMSG_GOSSIP_COMPLETE), nil, true) == nil
}

func (s *session) handleQuestLogRemoveQuest(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	slot := int(payload[0])
	if slot < 0 || slot >= playerQuestLogSlots {
		return true
	}
	questID := s.player.QuestLog[slot].QuestID
	if questID == 0 {
		return true
	}
	s.player.QuestLog[slot] = questLogEntry{}
	s.sendPlayerQuestLogUpdate(slot)

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "DELETE FROM character_queststatus WHERE guid = ? AND quest = ?", s.playerGUID, questID)
		_, _ = cdb.ExecContext(ctx, "DELETE FROM character_queststatus_daily WHERE guid = ? AND quest = ?", s.playerGUID, questID)
	}
	if s.server.Features != nil && s.server.Features.Scripts != nil {
		_, _ = s.server.Features.Scripts.TriggerPlayerEvent(ctx, 43, 43, s.luaPlayer(), questID)
	}
	s.debug("quest abandoned", "account", s.accountName, "quest", questID, "slot", slot)
	return true
}

func (s *session) questgiverStartsQuest(ctx context.Context, guid uint64, questID uint32) bool {
	high := uint16(guid >> 48)
	if high == 0xF110 {
		entry := uint32((guid >> 24) & 0x00FFFFFF)
		return questRelationExists(ctx, s.server.WorldStore.DB, "gameobject_queststarter", entry, questID)
	}
	entry := uint32((guid >> 24) & 0x00FFFFFF)
	if entry != 0 && s.creatureStartsQuest(ctx, entry, questID) {
		return true
	}
	creature := s.luaCreature(ctx, guid)
	if creature != nil {
		return s.creatureStartsQuest(ctx, objectUint32OrZero(creature, "Entry"), questID)
	}
	var spawnEntry uint32
	spawnGUID := uint32(guid & 0x00FFFFFF)
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT id FROM creature WHERE guid = ?", spawnGUID).Scan(&spawnEntry); err == nil && spawnEntry != 0 {
		return s.creatureStartsQuest(ctx, spawnEntry, questID)
	}
	return false
}

func (s *session) sendGossipComplete() bool {
	s.gossip = nil
	s.gossipClosed = true
	return s.write(uint16(protocol.OpcodeSMSG_GOSSIP_COMPLETE), nil, true) == nil
}

func (s *session) creatureHasQuest(ctx context.Context, entry, questID uint32) bool {
	return s.creatureStartsQuest(ctx, entry, questID) || s.creatureEndsQuest(ctx, entry, questID)
}

func (s *session) creatureStartsQuest(ctx context.Context, entry, questID uint32) bool {
	return questRelationExists(ctx, s.server.WorldStore.DB, "creature_queststarter", entry, questID)
}

func (s *session) creatureEndsQuest(ctx context.Context, entry, questID uint32) bool {
	return questRelationExists(ctx, s.server.WorldStore.DB, "creature_questender", entry, questID)
}

func questRelationExists(ctx context.Context, db *sql.DB, table string, entry, questID uint32) bool {
	var count int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(1) FROM "+table+" WHERE id = ? AND quest = ?", entry, questID).Scan(&count); err != nil {
		return false
	}
	return count != 0
}

func (s *session) loadQuestDetailData(ctx context.Context, questID uint32) (questDetailData, error) {
	var data questDetailData
	var title, objectives, details sql.NullString
	var flags, suggestedGroup, rewardMoney, rewardXP, rewardBonusMoney, rewardDisplaySpell, rewardHonor, rewardTitle, rewardTalents int64
	var rewardSpell, rewardArenaPoints int64
	var rewardKillHonor float64
	var rewardIDs [questRewardItems]int64
	var rewardCounts [questRewardItems]int64
	var choiceIDs [questChoiceItems]int64
	var choiceCounts [questChoiceItems]int64
	var factionIDs [questRewardFactions]int64
	var factionValues [questRewardFactions]int64
	var factionOverrides [questRewardFactions]int64
	columns := []string{"LogTitle", "LogDescription", "QuestDescription", "Flags", "SuggestedGroupNum", "RewardMoney", "RewardXPDifficulty", "RewardBonusMoney", "RewardDisplaySpell", "RewardSpell", "RewardHonor", "RewardKillHonor", "RewardTitle", "RewardTalents", "RewardArenaPoints"}
	targets := []any{&title, &objectives, &details, &flags, &suggestedGroup, &rewardMoney, &rewardXP, &rewardBonusMoney, &rewardDisplaySpell, &rewardSpell, &rewardHonor, &rewardKillHonor, &rewardTitle, &rewardTalents, &rewardArenaPoints}
	for index := 1; index <= questRewardItems; index++ {
		columns = append(columns, "RewardItem"+itoa(index), "RewardAmount"+itoa(index))
		targets = append(targets, &rewardIDs[index-1], &rewardCounts[index-1])
	}
	for index := 1; index <= questChoiceItems; index++ {
		columns = append(columns, "RewardChoiceItemID"+itoa(index), "RewardChoiceItemQuantity"+itoa(index))
		targets = append(targets, &choiceIDs[index-1], &choiceCounts[index-1])
	}
	for index := 1; index <= questRewardFactions; index++ {
		columns = append(columns, "RewardFactionID"+itoa(index), "RewardFactionValue"+itoa(index), "RewardFactionOverride"+itoa(index))
		targets = append(targets, &factionIDs[index-1], &factionValues[index-1], &factionOverrides[index-1])
	}
	query := "SELECT " + strings.Join(columns, ", ") + " FROM quest_template WHERE ID = ?"
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, query, questID).Scan(targets...); err != nil {
		return data, err
	}
	data.ID, data.Title, data.Objectives, data.Details = questID, title.String, objectives.String, details.String
	data.Flags, data.SuggestedGroupNum, data.RewardMoney, data.RewardXPDifficulty = uint32(flags), uint32(suggestedGroup), uint32(rewardMoney), uint32(rewardXP)
	data.RewardBonusMoney, data.RewardDisplaySpell, data.RewardSpell, data.RewardHonor = uint32(rewardBonusMoney), uint32(rewardDisplaySpell), int32(rewardSpell), uint32(rewardHonor)
	data.RewardKillHonor, data.RewardTitleID, data.RewardTalents, data.RewardArenaPoints = float32(rewardKillHonor), uint32(rewardTitle), uint32(rewardTalents), int32(rewardArenaPoints)
	for index, itemID := range rewardIDs {
		if itemID == 0 {
			continue
		}
		data.RewardItems = append(data.RewardItems, questRewardItem{ID: uint32(itemID), Quantity: uint32(rewardCounts[index]), DisplayID: s.itemDisplayID(ctx, uint32(itemID))})
	}
	for index, itemID := range choiceIDs {
		if itemID == 0 {
			continue
		}
		data.ChoiceItems = append(data.ChoiceItems, questRewardItem{ID: uint32(itemID), Quantity: uint32(choiceCounts[index]), DisplayID: s.itemDisplayID(ctx, uint32(itemID))})
	}
	for index := range data.RewardFactionID {
		data.RewardFactionID[index], data.RewardFactionValue[index], data.RewardFactionOverride[index] = uint32(factionIDs[index]), int32(factionValues[index]), int32(factionOverrides[index])
	}
	rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT Emote1, Emote2, Emote3, Emote4, EmoteDelay1, EmoteDelay2, EmoteDelay3, EmoteDelay4 FROM quest_details WHERE ID = ?", questID)
	if err == nil {
		if rows.Next() {
			var types [questDetailEmotes]int64
			var delays [questDetailEmotes]int64
			dest := make([]any, 0, questDetailEmotes*2)
			for index := range types {
				dest = append(dest, &types[index])
			}
			for index := range delays {
				dest = append(dest, &delays[index])
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return data, err
			}
			for index := range data.DescEmotes {
				data.DescEmotes[index] = questDescEmote{Type: uint32(types[index]), Delay: uint32(delays[index])}
			}
		}
		rows.Close()
	} else if !missingTable(err) {
		return data, err
	}
	return data, nil
}

func (s *session) itemDisplayID(ctx context.Context, entry uint32) uint32 {
	var displayID int64
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT displayid FROM item_template WHERE entry = ?", entry).Scan(&displayID); err != nil {
		return 0
	}
	return uint32(displayID)
}

func buildQuestGiverDetails(data questDetailData, giverGUID, informGUID uint64) []byte {
	packet := protocol.NewBuffer(512)
	packet.WriteU64(giverGUID)
	packet.WriteU64(informGUID)
	packet.WriteU32(data.ID)
	packet.WriteCString(data.Title)
	packet.WriteCString(data.Details)
	packet.WriteCString(data.Objectives)
	packet.WriteU8(1)
	packet.WriteU32(data.Flags)
	packet.WriteU32(data.SuggestedGroupNum)
	packet.WriteU8(0)
	packet.WriteU32(uint32(len(data.ChoiceItems)))
	for _, item := range data.ChoiceItems {
		packet.WriteU32(item.ID)
		packet.WriteU32(item.Quantity)
		packet.WriteU32(item.DisplayID)
	}
	packet.WriteU32(uint32(len(data.RewardItems)))
	for _, item := range data.RewardItems {
		packet.WriteU32(item.ID)
		packet.WriteU32(item.Quantity)
		packet.WriteU32(item.DisplayID)
	}
	packet.WriteU32(data.RewardMoney)
	packet.WriteU32(data.RewardXPDifficulty)
	packet.WriteU32(data.RewardHonor)
	packet.WriteF32(data.RewardKillHonor)
	packet.WriteU32(data.RewardDisplaySpell)
	packet.WriteI32(data.RewardSpell)
	packet.WriteU32(data.RewardTitleID)
	packet.WriteU32(data.RewardTalents)
	packet.WriteI32(data.RewardArenaPoints)
	packet.WriteU32(0)
	for _, value := range data.RewardFactionID {
		packet.WriteU32(value)
	}
	for _, value := range data.RewardFactionValue {
		packet.WriteI32(value)
	}
	for _, value := range data.RewardFactionOverride {
		packet.WriteI32(value)
	}
	packet.WriteI32(int32(len(data.DescEmotes)))
	for _, emote := range data.DescEmotes {
		packet.WriteU32(emote.Type)
		packet.WriteU32(emote.Delay)
	}
	return packet.Bytes()
}

func itoa(value int) string {
	if value < 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	if digits == "" {
		return "0"
	}
	return digits
}
