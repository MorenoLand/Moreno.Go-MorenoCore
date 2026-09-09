package world

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// Achievement system core, mirroring the reference:
//   - AchievementMgr::SendAllAchievementData (login dump: earned list, -1,
//     progress list, -1 via SMSG_ALL_ACHIEVEMENT_DATA)
//   - WorldSession::HandleQueryInspectAchievements -> RespondInspectAchievements
//     (packed target GUID + earned/progress blocks, Achievements of others
//     show only completed achievements)
//   - AchievementMgr::SetCriteriaProgress + CriteriaUpdate wire format
//     (AchievementPackets.cpp): criteria id, packed quantity, packed player
//     GUID, flags, date, elapsed, creation time
//   - AchievementMgr::CompletedCriteriaForAchievement: a criterion completes
//     at progress >= criteria.Quantity (field 4 of Achievement_Criteria.dbc)
//   - AchievementMgr::CompletedAchievement: SMSG_ACHIEVEMENT_EARNED to the
//     player and nearby players, persisted to character_achievement
//
// DBC layouts per DBCStructure.h: Achievement.dbc fields 0/1/2/38/39/41/60/61
// (ID, Faction, InstanceID, Category, Points, Flags, MinimumCriteria,
// SharesCriteria); Achievement_Criteria.dbc fields 0-4 (ID, AchievementID,
// Type, Asset, Quantity), 26-29 (Flags, StartEvent, StartAsset, StartTimer).

// Criteria types wired so far; the reference defines ~130.
const (
	criteriaTypeKillCreature     = 0  // ACHIEVEMENT_CRITERIA_TYPE_KILL_CREATURE
	criteriaTypeWinBG            = 1  // ACHIEVEMENT_CRITERIA_TYPE_WIN_BG
	criteriaTypeReachLevel       = 5  // ACHIEVEMENT_CRITERIA_TYPE_REACH_LEVEL
	criteriaTypeReachSkillLevel  = 7  // ACHIEVEMENT_CRITERIA_TYPE_REACH_SKILL_LEVEL
	criteriaTypeDeath            = 17 // ACHIEVEMENT_CRITERIA_TYPE_DEATH
	criteriaTypeKilledByCreature = 20 // ACHIEVEMENT_CRITERIA_TYPE_KILLED_BY_CREATURE
	criteriaTypeCompleteQuest    = 27 // ACHIEVEMENT_CRITERIA_TYPE_COMPLETE_QUEST
	criteriaTypeCastSpell        = 29 // ACHIEVEMENT_CRITERIA_TYPE_CAST_SPELL
	criteriaTypeLearnSpell       = 34 // ACHIEVEMENT_CRITERIA_TYPE_LEARN_SPELL
	criteriaTypeOwnItem          = 36 // ACHIEVEMENT_CRITERIA_TYPE_OWN_ITEM
	criteriaTypeBuyBankSlot      = 45 // ACHIEVEMENT_CRITERIA_TYPE_BUY_BANK_SLOT
	criteriaTypeUseItem          = 41 // ACHIEVEMENT_CRITERIA_TYPE_USE_ITEM
	criteriaTypeLootItem         = 42 // ACHIEVEMENT_CRITERIA_TYPE_LOOT_ITEM
	criteriaTypeGainReputation   = 46 // ACHIEVEMENT_CRITERIA_TYPE_GAIN_REPUTATION
	criteriaTypeLootMoney        = 67 // ACHIEVEMENT_CRITERIA_TYPE_LOOT_MONEY
	criteriaTypeDamageDone       = 13 // ACHIEVEMENT_CRITERIA_TYPE_DAMAGE_DONE
	criteriaTypeHealingDone      = 55 // ACHIEVEMENT_CRITERIA_TYPE_HEALING_DONE
	criteriaTypeQuestCount       = 9  // ACHIEVEMENT_CRITERIA_TYPE_COMPLETE_QUEST_COUNT
	criteriaTypeRollNeed         = 50 // ACHIEVEMENT_CRITERIA_TYPE_ROLL_NEED_ON_LOOT
	criteriaTypeRollGreed        = 51 // ACHIEVEMENT_CRITERIA_TYPE_ROLL_GREED_ON_LOOT
	criteriaTypeMoneyFromVendor  = 59 // ACHIEVEMENT_CRITERIA_TYPE_MONEY_FROM_VENDORS
	criteriaTypeMoneyFromQuest   = 62 // ACHIEVEMENT_CRITERIA_TYPE_MONEY_FROM_QUEST_REWARD
	criteriaTypeGoldSpentForMail = 66 // ACHIEVEMENT_CRITERIA_TYPE_GOLD_SPENT_FOR_MAIL
)

type achievementEntry struct {
	ID              uint32
	Faction         int32 // -1 any, 0 horde, 1 alliance
	Category        uint32
	Points          uint32
	Flags           uint32
	MinimumCriteria uint32
}

type achievementCriteriaEntry struct {
	ID            uint32
	AchievementID uint32
	Type          uint32
	Asset         uint32
	Quantity      uint32 // required count
}

type criteriaProgressState struct {
	CriteriaID uint32
	Counter    uint32
	Date       uint32
}

type achievementRuntime struct {
	mu          sync.RWMutex
	byTypeAsset map[uint64][]achievementCriteriaEntry // key: type<<32 | asset
	byID        map[uint32]achievementCriteriaEntry
	byAchieve   map[uint32][]achievementCriteriaEntry
	achieveByID map[uint32]achievementEntry
	loaded      bool
}

var achievementIndex = &achievementRuntime{
	byTypeAsset: make(map[uint64][]achievementCriteriaEntry),
	byID:        make(map[uint32]achievementCriteriaEntry),
	byAchieve:   make(map[uint32][]achievementCriteriaEntry),
	achieveByID: make(map[uint32]achievementEntry),
}

func typeAssetKey(criterionType, asset uint32) uint64 {
	return uint64(criterionType)<<32 | uint64(asset)
}

// loadAchievementIndex builds the criteria/achievement index from the DBC
// stores once per process.
func (s *Server) loadAchievementIndex() {
	achievementIndex.mu.Lock()
	defer achievementIndex.mu.Unlock()
	if achievementIndex.loaded || s.Data == nil {
		return
	}
	file, err := s.Data.File("Achievement_Criteria")
	if err != nil {
		return
	}
	for i := 0; i < file.Records(); i++ {
		record, err := file.Record(i)
		if err != nil {
			continue
		}
		id, err := record.Uint32(0)
		if err != nil {
			continue
		}
		achievementID, err := record.Uint32(1)
		if err != nil {
			continue
		}
		criterionType, err := record.Uint32(2)
		if err != nil {
			continue
		}
		asset, err := record.Uint32(3)
		if err != nil {
			continue
		}
		quantity, err := record.Uint32(4)
		if err != nil {
			continue
		}
		entry := achievementCriteriaEntry{ID: id, AchievementID: achievementID, Type: criterionType, Asset: asset, Quantity: quantity}
		key := typeAssetKey(criterionType, asset)
		achievementIndex.byTypeAsset[key] = append(achievementIndex.byTypeAsset[key], entry)
		achievementIndex.byID[id] = entry
		achievementIndex.byAchieve[achievementID] = append(achievementIndex.byAchieve[achievementID], entry)
	}
	if af, err := s.Data.File("Achievement"); err == nil {
		for i := 0; i < af.Records(); i++ {
			record, err := af.Record(i)
			if err != nil {
				continue
			}
			id, err := record.Uint32(0)
			if err != nil {
				continue
			}
			faction, _ := record.Int32(1)
			category, _ := record.Uint32(38)
			points, _ := record.Uint32(39)
			flags, _ := record.Uint32(41)
			minimum, _ := record.Uint32(60)
			achievementIndex.achieveByID[id] = achievementEntry{ID: id, Faction: faction, Category: category, Points: points, Flags: flags, MinimumCriteria: minimum}
		}
	}
	achievementIndex.loaded = true
}

// loadAchievementState reads character_achievement and progress rows for a
// player, mirroring AchievementMgr::LoadFromDB.
func (s *session) loadAchievementState(ctx context.Context) {
	s.earnedAchievements = make(map[uint32]uint32)
	s.criteriaProgress = make(map[uint32]*criteriaProgressState)
	cdb := s.server.CharactersStore
	if cdb == nil || cdb.DB == nil {
		return
	}
	rows, err := cdb.DB.QueryContext(ctx, "SELECT achievement, date FROM character_achievement WHERE guid = ?", s.playerGUID)
	if err == nil {
		for rows.Next() {
			var id, date uint32
			if rows.Scan(&id, &date) == nil {
				s.earnedAchievements[id] = date
			}
		}
		rows.Close()
	}
	rows, err = cdb.DB.QueryContext(ctx, "SELECT criteria, counter, date FROM character_achievement_progress WHERE guid = ?", s.playerGUID)
	if err == nil {
		for rows.Next() {
			var id, counter, date uint32
			if rows.Scan(&id, &counter, &date) == nil {
				s.criteriaProgress[id] = &criteriaProgressState{CriteriaID: id, Counter: counter, Date: date}
			}
		}
		rows.Close()
	}
}

// writeEarnedAchievement appends one EarnedAchievement block (u32 id, u32 date).
func writeEarnedAchievement(buffer *protocol.Buffer, id, date uint32) {
	buffer.WriteU32(id)
	buffer.WriteU32(date)
}

// writeCriteriaProgress appends one CriteriaProgress block per
// AchievementPackets.cpp: criteria id, packed quantity, packed player GUID,
// flags, date, time from start, time from create.
func writeCriteriaProgress(buffer *protocol.Buffer, playerGUID uint64, progress *criteriaProgressState) {
	buffer.WriteU32(progress.CriteriaID)
	buffer.WritePackedGUID(uint64(progress.Counter))
	buffer.WritePackedGUID(playerGUID)
	buffer.WriteU32(0) // flags
	buffer.WriteU32(progress.Date)
	buffer.WriteU32(0) // elapsed time
	buffer.WriteU32(0) // creation time
}

// sendAllAchievementData mirrors AchievementMgr::SendAllAchievementData:
// earned block, -1 separator, progress block, -1 separator.
func (s *session) sendAllAchievementData() {
	if s.player == nil {
		return
	}
	packet := protocol.NewBuffer(256)
	ids := make([]uint32, 0, len(s.earnedAchievements))
	for id := range s.earnedAchievements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		writeEarnedAchievement(packet, id, s.earnedAchievements[id])
	}
	packet.WriteU32(0xFFFFFFFF)
	progressIDs := make([]uint32, 0, len(s.criteriaProgress))
	for id := range s.criteriaProgress {
		progressIDs = append(progressIDs, id)
	}
	sort.Slice(progressIDs, func(i, j int) bool { return progressIDs[i] < progressIDs[j] })
	for _, id := range progressIDs {
		writeCriteriaProgress(packet, s.playerGUID, s.criteriaProgress[id])
	}
	packet.WriteU32(0xFFFFFFFF)
	_ = s.write(uint16(protocol.OpcodeSMSG_ALL_ACHIEVEMENT_DATA), packet.Bytes(), true)
}

// handleQueryInspectAchievements answers CMSG_QUERY_INSPECT_ACHIEVEMENTS with
// the target's real achievement state from the database, mirroring
// RespondInspectAchievements (packed target GUID, earned block, -1,
// progress block, -1).
func (s *session) handleQueryInspectAchievements(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	targetGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	cdb := s.server.CharactersStore
	if cdb == nil || cdb.DB == nil {
		return true
	}
	earned := make(map[uint32]uint32)
	progress := make(map[uint32]*criteriaProgressState)
	if rows, err := cdb.DB.QueryContext(ctx, "SELECT achievement, date FROM character_achievement WHERE guid = ?", targetGUID); err == nil {
		for rows.Next() {
			var id, date uint32
			if rows.Scan(&id, &date) == nil {
				earned[id] = date
			}
		}
		rows.Close()
	}
	if rows, err := cdb.DB.QueryContext(ctx, "SELECT criteria, counter, date FROM character_achievement_progress WHERE guid = ?", targetGUID); err == nil {
		for rows.Next() {
			var id, counter, date uint32
			if rows.Scan(&id, &counter, &date) == nil {
				progress[id] = &criteriaProgressState{CriteriaID: id, Counter: counter, Date: date}
			}
		}
		rows.Close()
	}
	packet := protocol.NewBuffer(64)
	packet.WritePackedGUID(targetGUID)
	ids := make([]uint32, 0, len(earned))
	for id := range earned {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		writeEarnedAchievement(packet, id, earned[id])
	}
	packet.WriteU32(0xFFFFFFFF)
	progressIDs := make([]uint32, 0, len(progress))
	for id := range progress {
		progressIDs = append(progressIDs, id)
	}
	sort.Slice(progressIDs, func(i, j int) bool { return progressIDs[i] < progressIDs[j] })
	for _, id := range progressIDs {
		writeCriteriaProgress(packet, targetGUID, progress[id])
	}
	packet.WriteU32(0xFFFFFFFF)
	return s.write(uint16(protocol.OpcodeSMSG_RESPOND_INSPECT_ACHIEVEMENTS), packet.Bytes(), true) == nil
}

// sendCriteriaUpdate mirrors AchievementPackets CriteriaUpdate.
func (s *session) sendCriteriaUpdate(progress *criteriaProgressState) {
	packet := protocol.NewBuffer(32)
	packet.WriteU32(progress.CriteriaID)
	packet.WritePackedGUID(uint64(progress.Counter))
	packet.WritePackedGUID(s.playerGUID)
	packet.WriteU32(0) // flags
	packet.WriteU32(progress.Date)
	packet.WriteU32(0) // elapsed
	packet.WriteU32(0) // creation
	_ = s.write(uint16(protocol.OpcodeSMSG_CRITERIA_UPDATE), packet.Bytes(), true)
}

// updateAchievementCriteria advances every criterion matching the type and
// asset by quantity, mirroring AchievementMgr::UpdateAchievementCriteria for
// the counter-style criteria this server tracks.
func (s *session) updateAchievementCriteria(criterionType, asset uint32, quantity uint32) {
	if s.player == nil || s.server == nil {
		return
	}
	if s.earnedAchievements == nil {
		s.earnedAchievements = make(map[uint32]uint32)
	}
	if s.criteriaProgress == nil {
		s.criteriaProgress = make(map[uint32]*criteriaProgressState)
	}
	s.server.loadAchievementIndex()
	achievementIndex.mu.RLock()
	criteriaList, ok := achievementIndex.byTypeAsset[typeAssetKey(criterionType, asset)]
	if !ok {
		achievementIndex.mu.RUnlock()
		return
	}
	matched := make([]achievementCriteriaEntry, len(criteriaList))
	copy(matched, criteriaList)
	achievementIndex.mu.RUnlock()

	for _, criterion := range matched {
		if _, done := s.earnedAchievements[criterion.AchievementID]; done {
			continue
		}
		progress := s.criteriaProgress[criterion.ID]
		if progress == nil {
			progress = &criteriaProgressState{CriteriaID: criterion.ID}
			s.criteriaProgress[criterion.ID] = progress
		}
		progress.Counter += quantity
		progress.Date = uint32(time.Now().Unix())
		s.sendCriteriaUpdate(progress)
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(context.Background(),
				"REPLACE INTO character_achievement_progress (guid, criteria, counter, date) VALUES (?, ?, ?, ?)",
				s.playerGUID, criterion.ID, progress.Counter, progress.Date)
		}
		if criterion.Quantity > 0 && progress.Counter >= criterion.Quantity {
			s.checkAchievementComplete(criterion.AchievementID)
		}
	}
}

// setAchievementCriteria sets an absolute criteria value (level, skill
// value, reputation standing) mirroring the reference PROGRESS_SET updates.
func (s *session) setAchievementCriteria(criterionType, asset, value uint32) {
	if s.player == nil || s.server == nil {
		return
	}
	if s.earnedAchievements == nil {
		s.earnedAchievements = make(map[uint32]uint32)
	}
	if s.criteriaProgress == nil {
		s.criteriaProgress = make(map[uint32]*criteriaProgressState)
	}
	s.server.loadAchievementIndex()
	achievementIndex.mu.RLock()
	criteriaList, ok := achievementIndex.byTypeAsset[typeAssetKey(criterionType, asset)]
	if !ok {
		criteriaList, ok = achievementIndex.byTypeAsset[typeAssetKey(criterionType, 0)]
	}
	if !ok {
		achievementIndex.mu.RUnlock()
		return
	}
	matched := make([]achievementCriteriaEntry, len(criteriaList))
	copy(matched, criteriaList)
	achievementIndex.mu.RUnlock()

	for _, criterion := range matched {
		if _, done := s.earnedAchievements[criterion.AchievementID]; done {
			continue
		}
		progress := s.criteriaProgress[criterion.ID]
		if progress == nil {
			progress = &criteriaProgressState{CriteriaID: criterion.ID}
			s.criteriaProgress[criterion.ID] = progress
		}
		if progress.Counter >= value {
			continue // absolute values never regress
		}
		progress.Counter = value
		progress.Date = uint32(time.Now().Unix())
		s.sendCriteriaUpdate(progress)
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(context.Background(),
				"REPLACE INTO character_achievement_progress (guid, criteria, counter, date) VALUES (?, ?, ?, ?)",
				s.playerGUID, criterion.ID, progress.Counter, progress.Date)
		}
		if criterion.Quantity > 0 && progress.Counter >= criterion.Quantity {
			s.checkAchievementComplete(criterion.AchievementID)
		}
	}
}

// checkAchievementComplete completes the achievement when every tracked
// criterion of it has met its quantity, then announces and persists it.
func (s *session) checkAchievementComplete(achievementID uint32) {
	achievementIndex.mu.RLock()
	criteria := achievementIndex.byAchieve[achievementID]
	entry, hasEntry := achievementIndex.achieveByID[achievementID]
	achievementIndex.mu.RUnlock()
	if len(criteria) == 0 || !hasEntry {
		return
	}
	if entry.Faction >= 0 {
		team := playerTeam(s.player.Race)
		if (entry.Faction == 0 && team != teamHorde) || (entry.Faction == 1 && team != teamAlliance) {
			return
		}
	}
	completedCount := uint32(0)
	for _, criterion := range criteria {
		progress := s.criteriaProgress[criterion.ID]
		if progress != nil && criterion.Quantity > 0 && progress.Counter >= criterion.Quantity {
			completedCount++
		}
	}
	needed := entry.MinimumCriteria
	if needed == 0 {
		needed = uint32(len(criteria))
	}
	if completedCount < needed {
		return
	}
	s.completeAchievement(achievementID)
}

// completeAchievement mirrors AchievementMgr::CompletedAchievement:
// SMSG_ACHIEVEMENT_EARNED (packed earner, id, time, initial=1) to the player
// and nearby players, persisted to character_achievement.
func (s *session) completeAchievement(achievementID uint32) {
	if _, done := s.earnedAchievements[achievementID]; done {
		return
	}
	now := uint32(time.Now().Unix())
	s.earnedAchievements[achievementID] = now
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(context.Background(),
			"INSERT OR IGNORE INTO character_achievement (guid, achievement, date) VALUES (?, ?, ?)",
			s.playerGUID, achievementID, now)
	}
	packet := protocol.NewBuffer(24)
	packet.WritePackedGUID(s.playerGUID)
	packet.WriteU32(achievementID)
	packet.WriteU32(now)
	packet.WriteU32(1) // initial
	_ = s.write(uint16(protocol.OpcodeSMSG_ACHIEVEMENT_EARNED), packet.Bytes(), true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_ACHIEVEMENT_EARNED), packet.Bytes(), s)
	}
	s.debug("achievement earned", "account", s.accountName, "guid", s.playerGUID, "achievement", achievementID)
}

// achievementCriteriaCount is used by tests to inspect the index size.
func achievementCriteriaCount() int {
	achievementIndex.mu.RLock()
	defer achievementIndex.mu.RUnlock()
	return len(achievementIndex.byID)
}
