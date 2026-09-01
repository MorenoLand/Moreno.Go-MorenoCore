package world

import (
	"context"
	"database/sql"
	"errors"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	questStatusComplete   = 1
	questStatusIncomplete = 3
	questDialogNone       = 0
	questDialogIncomplete = 5
	questDialogReward     = 10
	questDialogAvailable  = 8
)

func (s *session) handleQuestgiverHello(ctx context.Context, payload []byte) bool {
	return s.handleGossipHello(ctx, payload)
}

func (s *session) handleQuestgiverStatusQuery(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	creature := s.luaCreature(ctx, guid)
	if creature == nil {
		return true
	}
	entry := objectUint32OrZero(creature, "Entry")
	status, err := s.questDialogStatus(ctx, entry)
	if err != nil {
		s.debug("quest status failed", "account", s.accountName, "entry", entry, "error", err)
		return false
	}
	packet := protocol.NewBuffer(9)
	packet.WriteU64(guid)
	packet.WriteU8(status)
	s.debug("quest status response", "account", s.accountName, "entry", entry, "status", status)
	return s.write(uint16(protocol.OpcodeSMSG_QUESTGIVER_STATUS), packet.Bytes(), true) == nil
}

func (s *session) isQuestRewarded(ctx context.Context, questID uint32) bool {
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return false
	}
	var count int64
	err := cdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM character_queststatus WHERE guid = ? AND quest = ? AND status = 2", s.playerGUID, questID).Scan(&count)
	if err == nil && count > 0 {
		return true
	}
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM character_queststatus_rewarded WHERE guid = ? AND quest = ?", s.playerGUID, questID).Scan(&count)
	return count > 0
}

func (s *session) canTakeQuest(ctx context.Context, questID uint32) (bool, error) {
	if s.player == nil {
		return false, nil
	}
	status, err := s.characterQuestStatus(ctx, questID)
	if err != nil {
		return false, err
	}
	if status != 0 {
		return false, nil
	}
	if s.isQuestRewarded(ctx, questID) {
		return false, nil
	}
	wdb := s.server.WorldStore.DB
	if wdb == nil {
		return false, nil
	}

	// 1. Seasonal / World Event checks
	var eventEntry sql.NullInt64
	if err := wdb.QueryRowContext(ctx, "SELECT eventEntry FROM game_event_seasonal_questrelation WHERE questId = ?", questID).Scan(&eventEntry); err == nil && eventEntry.Valid && eventEntry.Int64 > 0 {
		activeEvents := s.server.cachedActiveGameEvents(ctx)
		if _, active := activeEvents[eventEntry.Int64]; !active {
			return false, nil
		}
	}
	if err := wdb.QueryRowContext(ctx, "SELECT eventEntry FROM game_event_creature_quest WHERE quest = ?", questID).Scan(&eventEntry); err == nil && eventEntry.Valid && eventEntry.Int64 > 0 {
		activeEvents := s.server.cachedActiveGameEvents(ctx)
		if _, active := activeEvents[eventEntry.Int64]; !active {
			return false, nil
		}
	}

	var qLevel, minLvl, flags, allowableRaces int64
	var title string
	err = wdb.QueryRowContext(ctx, "SELECT QuestLevel, MinLevel, Flags, COALESCE(LogTitle, ''), AllowableRaces FROM quest_template WHERE ID = ?", questID).Scan(&qLevel, &minLvl, &flags, &title, &allowableRaces)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		err = wdb.QueryRowContext(ctx, "SELECT QuestLevel, MinLevel, Flags, COALESCE(LogTitle, '') FROM quest_template WHERE ID = ?", questID).Scan(&qLevel, &minLvl, &flags, &title)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
	}

	var prevQuest, maxLvl, allowableClasses, exclGroup, nextQuest, breadcrumb, specialFlags, addonRaces sql.NullInt64
	_ = wdb.QueryRowContext(ctx, `SELECT PrevQuestID, MaxLevel, AllowableClasses, ExclusiveGroup, NextQuestID, BreadcrumbForQuestId, SpecialFlags
		FROM quest_template_addon WHERE ID = ?`, questID).Scan(&prevQuest, &maxLvl, &allowableClasses, &exclGroup, &nextQuest, &breadcrumb, &specialFlags)
	if !allowableClasses.Valid && !prevQuest.Valid {
		_ = wdb.QueryRowContext(ctx, `SELECT PrevQuestID, MaxLevel, AllowableClasses, AllowableRaces, ExclusiveGroup, NextQuestID
			FROM quest_template_addon WHERE ID = ?`, questID).Scan(&prevQuest, &maxLvl, &allowableClasses, &addonRaces, &exclGroup, &nextQuest)
		if addonRaces.Valid && allowableRaces == 0 {
			allowableRaces = addonRaces.Int64
		}
	}

	// Min/Max Level check
	if minLvl > int64(s.player.Level) {
		return false, nil
	}
	if maxLvl.Valid && maxLvl.Int64 > 0 && int64(s.player.Level) > maxLvl.Int64 {
		return false, nil
	}

	// Class check (AllowableClasses bitmask: 1<<(class-1))
	if allowableClasses.Valid && allowableClasses.Int64 > 0 && s.player.Class > 0 {
		playerClassMask := int64(1 << uint(s.player.Class-1))
		if (allowableClasses.Int64 & playerClassMask) == 0 {
			return false, nil
		}
	}

	// Race check (AllowableRaces bitmask: 1<<(race-1))
	if allowableRaces > 0 && s.player.Race > 0 {
		playerRaceMask := int64(1 << uint(s.player.Race-1))
		if (allowableRaces & playerRaceMask) == 0 {
			return false, nil
		}
	}

	// PrevQuestID check (chain prerequisite)
	if prevQuest.Valid && prevQuest.Int64 != 0 {
		if prevQuest.Int64 > 0 {
			if !s.isQuestRewarded(ctx, uint32(prevQuest.Int64)) {
				return false, nil
			}
		} else {
			prevActiveStatus, _ := s.characterQuestStatus(ctx, uint32(-prevQuest.Int64))
			if prevActiveStatus != questStatusIncomplete && prevActiveStatus != questStatusComplete {
				return false, nil
			}
		}
	}

	// Breadcrumb check
	if breadcrumb.Valid && breadcrumb.Int64 > 0 {
		bStatus, _ := s.characterQuestStatus(ctx, uint32(breadcrumb.Int64))
		if bStatus != 0 || s.isQuestRewarded(ctx, uint32(breadcrumb.Int64)) {
			return false, nil
		}
	}

	// ExclusiveGroup check
	if exclGroup.Valid && exclGroup.Int64 != 0 && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		var exclCount int64
		if exclGroup.Int64 > 0 {
			_ = s.server.CharactersStore.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_queststatus_rewarded AS cqr
				JOIN quest_template_addon AS qta ON qta.ID = cqr.quest
				WHERE cqr.guid = ? AND qta.ExclusiveGroup = ?`, s.playerGUID, exclGroup.Int64).Scan(&exclCount)
			if exclCount > 0 {
				return false, nil
			}
		}
	}

	// ConditionMgr check (SourceType 19 & 20)
	meetsCond, err := s.meetQuestConditions(ctx, questID)
	if err != nil || !meetsCond {
		return false, nil
	}

	return true, nil
}

func (s *session) questDialogStatus(ctx context.Context, entry uint32) (uint8, error) {
	enderIDs, err := loadQuestRelationIDs(ctx, s.server.WorldStore.DB, "creature_questender", entry)
	if err != nil {
		return questDialogNone, err
	}
	for _, questID := range enderIDs {
		status, err := s.characterQuestStatus(ctx, questID)
		if err != nil {
			return questDialogNone, err
		}
		if status == questStatusComplete {
			return questDialogReward, nil
		}
		if status == questStatusIncomplete {
			return questDialogIncomplete, nil
		}
	}
	if s.player == nil {
		return questDialogNone, nil
	}
	starterIDs, err := loadQuestRelationIDs(ctx, s.server.WorldStore.DB, "creature_queststarter", entry)
	if err != nil {
		return questDialogNone, err
	}
	for _, questID := range starterIDs {
		canTake, err := s.canTakeQuest(ctx, questID)
		if err != nil {
			return questDialogNone, err
		}
		if canTake {
			return questDialogAvailable, nil
		}
	}
	return questDialogNone, nil
}

func (s *session) characterQuestStatus(ctx context.Context, questID uint32) (int64, error) {
	var status int64
	err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT status FROM character_queststatus WHERE guid = ? AND quest = ?", s.playerGUID, questID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) || (err != nil && missingTable(err)) {
		return 0, nil
	}
	return status, err
}

func (s *session) loadCreatureQuestMenu(ctx context.Context, entry uint32, playerLevel uint8) ([]gossipQuestItem, error) {
	starterIDs, err := loadQuestRelationIDs(ctx, s.server.WorldStore.DB, "creature_queststarter", entry)
	if err != nil {
		return nil, err
	}
	enderIDs, err := loadQuestRelationIDs(ctx, s.server.WorldStore.DB, "creature_questender", entry)
	if err != nil {
		return nil, err
	}
	quests := make([]gossipQuestItem, 0, len(starterIDs)+len(enderIDs))
	seen := make(map[uint32]struct{})
	for _, questID := range enderIDs {
		status, err := s.characterQuestStatus(ctx, questID)
		if err != nil {
			return nil, err
		}
		if status != questStatusComplete && status != questStatusIncomplete {
			continue
		}
		quest, err := s.loadQuestMenuItem(ctx, questID, 4, playerLevel)
		if err != nil {
			return nil, err
		}
		if quest != nil {
			quests = append(quests, *quest)
			seen[questID] = struct{}{}
		}
	}
	for _, questID := range starterIDs {
		if _, exists := seen[questID]; exists {
			continue
		}
		canTake, err := s.canTakeQuest(ctx, questID)
		if err != nil || !canTake {
			continue
		}
		quest, err := s.loadQuestMenuItem(ctx, questID, 2, playerLevel)
		if err != nil {
			return nil, err
		}
		if quest != nil {
			quests = append(quests, *quest)
		}
	}
	return quests, nil
}

func (s *session) loadQuestMenuItem(ctx context.Context, questID, icon uint32, playerLevel uint8) (*gossipQuestItem, error) {
	var level, minLevel, flags int64
	var title sql.NullString
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT QuestLevel, MinLevel, Flags, COALESCE(LogTitle, '') FROM quest_template WHERE ID = ?", questID).Scan(&level, &minLevel, &flags, &title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if minLevel > int64(playerLevel) {
		return nil, nil
	}
	return &gossipQuestItem{ID: questID, Icon: icon, Level: int32(level), Flags: uint32(flags), AutoComplete: uint32(flags)&questAutoCompleteFlags != 0, Title: title.String}, nil
}

func loadQuestRelationIDs(ctx context.Context, db *sql.DB, table string, entry uint32) ([]uint32, error) {
	rows, err := db.QueryContext(ctx, "SELECT quest FROM "+table+" WHERE id = ? ORDER BY quest", entry)
	if err != nil {
		if missingTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	result := make([]uint32, 0)
	for rows.Next() {
		var questID int64
		if err := rows.Scan(&questID); err != nil {
			return nil, err
		}
		result = append(result, uint32(questID))
	}
	return result, rows.Err()
}
