package world

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// unitDynFlagLootable marks a corpse lootable in UNIT_FIELD_DYNAMIC_FLAGS.
const unitDynFlagLootable uint32 = 0x00000001

// creatureRespawn records when a killed spawn restores its health.
type creatureRespawn struct {
	At          time.Time
	Health      uint32
	Entry       uint32
	Map         uint32
	X, Y        float32
}

// xpTable mirrors TDB 3.3.5a `player_xp_for_level`; preferred from the world
// DB when populated (per-dialect SQL), otherwise this reference curve.
var xpCurve = [81]uint32{
	0, 400, 900, 1400, 2000, 2700, 3400, 4200, 5100, 6100, 7200,
	8400, 9700, 11100, 12600, 14200, 15900, 17700, 19600, 21600, 23800,
	26100, 28500, 31000, 33600, 36300, 39100, 42000, 45000, 48100, 51300,
	54700, 58200, 61800, 65600, 69500, 73600, 77800, 82200, 86800, 91600,
	96600, 101800, 107200, 112800, 118600, 124500, 130600, 136900, 143400, 150100,
	157000, 164100, 171400, 178900, 186600, 194500, 202700, 211000, 219600, 228400,
	237400, 246700, 256200, 266000, 276100, 286400, 297000, 307900, 319000, 330400,
	342100, 354000, 366200, 378700, 391500, 404700, 418200, 432000, 446200, 0,
}

var (
	xpTableMu   sync.RWMutex
	xpTableRows []uint32
	xpTableInit bool
)

func (s *Server) xpForLevel(ctx context.Context, level uint32) uint32 {
	if level == 0 || level >= uint32(len(xpCurve)) {
		return 0
	}
	if s.WorldStore != nil && s.WorldStore.DB != nil {
		xpTableMu.RLock()
		ready := xpTableInit && xpTableRows != nil
		rows := xpTableRows
		xpTableMu.RUnlock()
		if !ready {
			loaded := make([]uint32, len(xpCurve))
			copy(loaded, xpCurve[:])
			query := "SELECT Level, Experience FROM player_xp_for_level"
			if result, err := s.WorldStore.DB.QueryContext(ctx, query); err == nil {
				for result.Next() {
					var level, experience int64
					if result.Scan(&level, &experience) == nil && level > 0 && level < int64(len(loaded)) {
						loaded[level] = uint32(experience)
					}
				}
				result.Close()
				any := false
				for _, v := range loaded {
					if v != 0 {
						any = true
						break
					}
				}
				if !any {
					copy(loaded, xpCurve[:])
				}
			}
			xpTableMu.Lock()
			xpTableRows = loaded
			xpTableInit = true
			xpTableMu.Unlock()
			rows = loaded
		}
		if rows != nil {
			return rows[level]
		}
	}
	return xpCurve[level]
}

// baseXPForLevel loads exploration_basexp (TC's _baseXPTable) for the kill
// XP formula; falls back to the curve above when the table has no row.
func (s *Server) baseXPForLevel(ctx context.Context, level uint32) uint32 {
	if s.WorldStore == nil || s.WorldStore.DB == nil {
		return xpCurve[level]
	}
	var base int64
	if err := s.WorldStore.DB.QueryRowContext(ctx, "SELECT basexp FROM exploration_basexp WHERE level = ?", level).Scan(&base); err != nil || base <= 0 {
		fallback := xpCurve[level]
		if fallback == 0 {
			return 0
		}
		return fallback / 100
	}
	return uint32(base)
}

// grayLevel ports Formulas::XP::GetGrayLevel.
func grayLevel(playerLevel uint32) uint32 {
	switch {
	case playerLevel <= 5:
		return 0
	case playerLevel <= 39:
		return playerLevel - 5 - playerLevel/10
	case playerLevel <= 59:
		return playerLevel - 1 - playerLevel/5
	default:
		return playerLevel - 9
	}
}

// zeroDifference ports Formulas::XP::GetZeroDifference.
func zeroDifference(playerLevel uint32) uint32 {
	switch {
	case playerLevel < 8:
		return 5
	case playerLevel < 10:
		return 6
	case playerLevel < 12:
		return 7
	case playerLevel < 16:
		return 8
	case playerLevel < 20:
		return 9
	case playerLevel < 30:
		return 11
	case playerLevel < 40:
		return 12
	case playerLevel < 45:
		return 13
	case playerLevel < 50:
		return 14
	case playerLevel < 55:
		return 15
	case playerLevel < 60:
		return 16
	default:
		return 17
	}
}

// killXPGain ports Formulas::XP::BaseGain for solo kills.
func (s *Server) killXPGain(ctx context.Context, playerLevel, mobLevel uint32) uint32 {
	baseExp := s.baseXPForLevel(ctx, mobLevel)
	if baseExp == 0 {
		return 0
	}
	if mobLevel >= playerLevel {
		levelDiff := mobLevel - playerLevel
		if levelDiff > 4 {
			levelDiff = 4
		}
		return ((playerLevel*5 + baseExp) * (20 + levelDiff) / 10 + 1) / 2
	}
	if mobLevel > grayLevel(playerLevel) {
		zd := zeroDifference(playerLevel)
		return (playerLevel*5 + baseExp) * (zd + mobLevel - playerLevel) / zd
	}
	return 0
}

// onCreatureKilled runs the full death chain for a melee kill: XP and
// level-ups, lootable corpse flag, respawn scheduling and quest credit.
func (s *session) onCreatureKilled(ctx context.Context, target combatTarget) {
	if s.player == nil {
		return
	}
	creatureEntry := uint32((target.GUID >> 24) & 0xFFFFFF)
	guid := uint32(target.GUID & 0x00FFFFFF)
	now := time.Now()

	// XP with the reference gray/zero-difference curve.
	var mobLevel int64
	_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(maxlevel, 0), minlevel, 1) FROM creature_template WHERE entry = ?", creatureEntry).Scan(&mobLevel)
	xp := s.server.killXPGain(ctx, uint32(s.player.Level), uint32(mobLevel))
	if xp > 0 {
		s.grantXP(ctx, xp)
	}

	// Mark the corpse lootable for everyone in range (dynamic flags update).
	s.server.broadcastCreatureValueUpdate(target.GUID, s.player.Map, target.X, target.Y, map[int]uint32{unitFieldDynamicFlags: unitDynFlagLootable})

	// Schedule the respawn with the spawn's original health.
	s.server.scheduleCreatureRespawn(ctx, guid, uint32(math.Max(float64(target.Health), 1)), now)

	// Quest kill credit: RequiredNpcOrGo entries plus KillCredit templates.
	s.creditQuestKills(ctx, creatureEntry, target.GUID)
}

// grantXP applies XP with repeated level-ups and updates the client fields.
func (s *session) grantXP(ctx context.Context, amount uint32) {
	if s.player == nil || amount == 0 {
		return
	}
	s.player.XP += amount
	for s.player.Level < 80 {
		needed := s.server.xpForLevel(ctx, uint32(s.player.Level))
		if needed == 0 || s.player.XP < needed {
			break
		}
		s.player.XP -= needed
		s.player.Level++
		s.player.MaxHealth += 60
		if s.player.Health > s.player.MaxHealth {
			s.player.Health = s.player.MaxHealth
		} else {
			s.player.Health = s.player.MaxHealth
		}
		// SMSG_LEVELUP_INFO: level, talent points, and per-power gains.
		levelPacket := protocol.NewBuffer(40)
		levelPacket.WriteU32(uint32(s.player.Level))
		levelPacket.WriteU32(1) // talent points
		for i := 0; i < 7; i++ {
			levelPacket.WriteU32(0)
		}
		_ = s.write(uint16(protocol.OpcodeSMSG_LEVELUP_INFO), levelPacket.Bytes(), true)
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET level = ?, xp = ?, health = ? WHERE guid = ?", s.player.Level, s.player.XP, s.player.Health, s.playerGUID)
		}
	}
	s.sendPlayerUpdate()
}

// broadcastCreatureValueUpdate pushes an UPDATETYPE_VALUES block for a
// creature to sessions within visibility range.
func (s *Server) broadcastCreatureValueUpdate(guid uint64, mapID uint32, x, y float32, fields map[int]uint32) {
	values := make([]uint32, 256)
	mask := protocol.NewUpdateMask(256)
	for index, value := range fields {
		if index < 0 || index >= 256 {
			continue
		}
		values[index] = value
		_ = mask.Set(index)
	}
	block := protocol.NewBuffer(64)
	block.WriteU8(protocol.UpdateValues)
	block.WritePackedGUID(guid)
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index := 0; index < 256; index++ {
		if mask.Has(index) {
			block.WriteU32(values[index])
		}
	}
	distance := s.Config.VisibilityDistanceContinents
	if distance <= 0 {
		distance = 150.0
	}
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for sess := range s.sessions {
		if !sess.playerLoaded || sess.player == nil || sess.player.Map != mapID {
			continue
		}
		if math.Hypot(float64(x-sess.player.X), float64(y-sess.player.Y)) <= distance {
			_ = sess.write(uint16(protocol.OpcodeSMSG_UPDATE_OBJECT), block.Bytes(), true)
		}
	}
}

// scheduleCreatureRespawn stores spawn health to restore after
// creature.spawntimesecs like Map::AddToActiveMap respawn handling.
func (s *Server) scheduleCreatureRespawn(ctx context.Context, guid, health uint32, now time.Time) {
	if s.WorldStore == nil || s.WorldStore.DB == nil {
		return
	}
	var seconds int64
	if err := s.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(spawntimesecs, 0), 300) FROM creature WHERE guid = ?", guid).Scan(&seconds); err != nil || seconds <= 0 {
		seconds = 300
	}
	s.motionMu.Lock()
	if s.creatureRespawns == nil {
		s.creatureRespawns = make(map[uint32]creatureRespawn)
	}
	s.creatureRespawns[guid] = creatureRespawn{At: now.Add(time.Duration(seconds) * time.Second), Health: health}
	s.motionMu.Unlock()
}

// processCreatureRespawns restores expired spawns; called from world tick.
func (s *Server) processCreatureRespawns(ctx context.Context, now time.Time) {
	if s.WorldStore == nil || s.WorldStore.DB == nil {
		return
	}
	s.motionMu.Lock()
	due := make([]creatureRespawn, 0, 8)
	for guid, respawn := range s.creatureRespawns {
		if now.After(respawn.At) {
			due = append(due, respawn)
			delete(s.creatureRespawns, guid)
		}
	}
	s.motionMu.Unlock()
	for _, respawn := range due {
		if _, err := s.WorldStore.DB.ExecContext(ctx, "UPDATE creature SET curhealth = ? WHERE guid = ?", respawn.Health, respawn.Entry); err != nil {
			continue
		}
	}
}

// creditQuestKills advances RequiredNpcOrGo objectives for quests in the
// log, sending SMSG_QUESTUPDATE_ADD_KILL per TC
// Player::KilledMonster / SendQuestUpdateAddCreatureOrGo.
func (s *session) creditQuestKills(ctx context.Context, creatureEntry uint32, victimGUID uint64) {
	if s.player == nil || s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return
	}
	// KillCredit templates redirect credit to another entry.
	var credits [2]int64
	_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(KillCredit1, 0), COALESCE(KillCredit2, 0) FROM creature_template WHERE entry = ?", creatureEntry).Scan(&credits[0], &credits[1])
	entries := []uint32{creatureEntry}
	for _, credit := range credits {
		if credit > 0 {
			entries = append(entries, uint32(credit))
		}
	}
	for slot := 0; slot < playerQuestLogSlots; slot++ {
		entry := s.player.QuestLog[slot]
		if entry.QuestID == 0 {
			continue
		}
		var reqIDs, reqCounts [4]int64
		err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT RequiredNpcOrGo1, RequiredNpcOrGo2, RequiredNpcOrGo3, RequiredNpcOrGo4, RequiredNpcOrGoCount1, RequiredNpcOrGoCount2, RequiredNpcOrGoCount3, RequiredNpcOrGoCount4 FROM quest_template WHERE ID = ?", entry.QuestID).Scan(&reqIDs[0], &reqIDs[1], &reqIDs[2], &reqIDs[3], &reqCounts[0], &reqCounts[1], &reqCounts[2], &reqCounts[3])
		if err != nil {
			continue
		}
		progressed := false
		for objective := 0; objective < 4; objective++ {
			required := uint32(reqCounts[objective])
			if required == 0 || reqIDs[objective] == 0 {
				continue
			}
			match := false
			for _, e := range entries {
				if uint32(reqIDs[objective]) == e {
					match = true
					break
				}
			}
			if !match {
				continue
			}
			if uint32(entry.Counters[objective]) >= required {
				continue
			}
			entry.Counters[objective]++
			progressed = true
			update := protocol.NewBuffer(24)
			update.WriteU32(entry.QuestID)
			update.WriteU32(uint32(reqIDs[objective]))
			update.WriteU32(uint32(entry.Counters[objective]))
			update.WriteU32(required)
			update.WriteU64(victimGUID)
			_ = s.write(uint16(protocol.OpcodeSMSG_QUESTUPDATE_ADD_KILL), update.Bytes(), true)
			countColumn := "mobcount" + string(rune('1'+objective))
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_queststatus SET "+countColumn+" = ? WHERE guid = ? AND quest = ?", int64(entry.Counters[objective]), s.playerGUID, entry.QuestID)
		}
		if !progressed {
			continue
		}
		s.player.QuestLog[slot] = entry
		s.sendPlayerQuestLogUpdate(slot)
		if s.questObjectivesComplete(ctx, entry.QuestID, entry) {
			entry.State = 1
			s.player.QuestLog[slot] = entry
			s.sendPlayerQuestLogUpdate(slot)
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_queststatus SET status = ? WHERE guid = ? AND quest = ?", questStatusComplete, s.playerGUID, entry.QuestID)
			_ = s.write(uint16(protocol.OpcodeSMSG_QUESTUPDATE_COMPLETE), nil, true)
		}
	}
}

// questObjectivesComplete checks kill and item objectives against the log
// entry and character inventory.
func (s *session) questObjectivesComplete(ctx context.Context, questID uint32, entry questLogEntry) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return false
	}
	var reqIDs, reqCounts [4]int64
	var itemIDs, itemCounts [6]int64
	err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT RequiredNpcOrGo1, RequiredNpcOrGo2, RequiredNpcOrGo3, RequiredNpcOrGo4, RequiredNpcOrGoCount1, RequiredNpcOrGoCount2, RequiredNpcOrGoCount3, RequiredNpcOrGoCount4, RequiredItemId1, RequiredItemId2, RequiredItemId3, RequiredItemId4, RequiredItemId5, RequiredItemId6, RequiredItemCount1, RequiredItemCount2, RequiredItemCount3, RequiredItemCount4, RequiredItemCount5, RequiredItemCount6 FROM quest_template WHERE ID = ?", questID).Scan(&reqIDs[0], &reqIDs[1], &reqIDs[2], &reqIDs[3], &reqCounts[0], &reqCounts[1], &reqCounts[2], &reqCounts[3], &itemIDs[0], &itemIDs[1], &itemIDs[2], &itemIDs[3], &itemIDs[4], &itemIDs[5], &itemCounts[0], &itemCounts[1], &itemCounts[2], &itemCounts[3], &itemCounts[4], &itemCounts[5])
	if err != nil {
		return false
	}
	for objective := 0; objective < 4; objective++ {
		if reqCounts[objective] > 0 && uint32(entry.Counters[objective]) < uint32(reqCounts[objective]) {
			return false
		}
	}
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		for index := 0; index < 6; index++ {
			if itemIDs[index] == 0 || itemCounts[index] == 0 {
				continue
			}
			var have int64
			_ = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(ii.count), 0) FROM character_inventory ci JOIN item_instance ii ON ii.guid = ci.item WHERE ci.guid = ? AND ii.itemEntry = ?", s.playerGUID, itemIDs[index]).Scan(&have)
			if have < itemCounts[index] {
				return false
			}
		}
	}
	return true
}
