package world

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type lootItem struct {
	Slot          uint8
	ItemEntry     uint32
	Count         uint32
	DisplayInfoID uint32
}

type activeLootState struct {
	TargetGUID uint64
	MapID      uint32
	LootType   uint8
	Money      uint32
	Items      map[uint8]lootItem
}

type activeGroupRoll struct {
	SourceGUID          uint64
	Slot                uint32
	ItemEntry           uint32
	ItemCount           uint32
	RandomSuffix        uint32
	RandomPropID        uint32
	RollVoteMask        uint8
	GroupID             uint64
	MapID               uint32
	StartedAt           time.Time
	Duration            time.Duration
	TotalPlayersRolling int
	TotalNeed           int
	TotalGreed          int
	TotalPass           int
	Votes               map[uint64]uint8
	Rolls               map[uint64]uint8
	Timer               *time.Timer
}

func (s *session) handleLoot(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	targetGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	wdb := s.server.WorldStore.DB
	if wdb == nil {
		return true
	}
	high := uint16(targetGUID >> 48)

	if high == 0xF110 {
		lowGUID := uint32(targetGUID & 0x00FFFFFF)
		entry := uint32((targetGUID >> 24) & 0x00FFFFFF)

		var goMap uint32
		var goX, goY, goZ float32
		var data1 int64
		err := wdb.QueryRowContext(ctx, `SELECT g.map, g.position_x, g.position_y, g.position_z, COALESCE(t.data1, 0)
			FROM gameobject AS g
			JOIN gameobject_template AS t ON t.entry = g.id
			WHERE g.guid = ? AND g.id = ? LIMIT 1`, lowGUID, entry).Scan(&goMap, &goX, &goY, &goZ, &data1)
		if err != nil {
			_ = wdb.QueryRowContext(ctx, `SELECT g.map, g.position_x, g.position_y, g.position_z, COALESCE(t.data1, 0)
				FROM gameobject AS g
				JOIN gameobject_template AS t ON t.entry = g.id
				WHERE g.guid = ? LIMIT 1`, lowGUID).Scan(&goMap, &goX, &goY, &goZ, &data1)
		}
		if goMap != s.player.Map || distance3D(s.player.X, s.player.Y, s.player.Z, goX, goY, goZ) > 10.0 {
			return s.sendLootError(targetGUID, 4) == nil
		}
		lootID := data1
		if lootID == 0 {
			lootID = int64(entry)
		}

		s.server.lootMu.Lock()
		if s.server.creatureLoot == nil {
			s.server.creatureLoot = make(map[uint64]*activeLootState)
		}
		loot := s.server.creatureLoot[targetGUID]
		newLoot := loot == nil
		if newLoot {
			loot = &activeLootState{TargetGUID: targetGUID, MapID: goMap, LootType: 1, Items: make(map[uint8]lootItem)}
			s.server.creatureLoot[targetGUID] = loot
		}
		s.server.lootMu.Unlock()

		if !newLoot {
			s.activeLoot = loot
			return s.sendLootResponse(loot) == nil
		}

		rows, err := wdb.QueryContext(ctx, `SELECT l.Item, l.Chance, l.MinCount, l.MaxCount, COALESCE(t.displayid, 0)
			FROM gameobject_loot_template AS l
			LEFT JOIN item_template AS t ON t.entry = l.Item
			WHERE l.Entry = ? ORDER BY l.Item LIMIT 16`, lootID)
		if err == nil {
			defer rows.Close()
			var slot uint8 = 0
			for rows.Next() {
				var itemID int64
				var chance float64
				var minCount, maxCount, displayID int64
				if err := rows.Scan(&itemID, &chance, &minCount, &maxCount, &displayID); err != nil {
					continue
				}
				roll := rand.Float64() * 100.0
				if chance > 0 && roll > chance {
					continue
				}
				count := uint32(minCount)
				if maxCount > minCount {
					count += uint32(rand.Intn(int(maxCount - minCount + 1)))
				}
				if count == 0 {
					count = 1
				}
				loot.Items[slot] = lootItem{
					Slot:          slot,
					ItemEntry:     uint32(itemID),
					Count:         count,
					DisplayInfoID: uint32(displayID),
				}
				slot++
				if slot >= 16 {
					break
				}
			}
		}
		s.activeLoot = loot
		return s.sendLootResponse(loot) == nil
	}

	target, ok := s.getCombatTarget(ctx, targetGUID)
	if !ok || target.Map != s.player.Map || distance3D(s.player.X, s.player.Y, s.player.Z, target.X, target.Y, target.Z) > 5.0 {
		return s.sendLootError(targetGUID, 4) == nil
	}
	if target.Health != 0 {
		return s.sendLootError(targetGUID, 0) == nil
	}
	guid := uint32(targetGUID & 0x00FFFFFF)
	creatureEntry := uint32((targetGUID >> 24) & 0x00FFFFFF)
	stdKey := creatureWorldGUID(guid, creatureEntry)

	s.server.lootMu.Lock()
	if s.server.creatureLoot == nil {
		s.server.creatureLoot = make(map[uint64]*activeLootState)
	}
	loot := s.server.creatureLoot[targetGUID]
	if loot == nil {
		loot = s.server.creatureLoot[stdKey]
	}
	newLoot := loot == nil
	if newLoot {
		loot = &activeLootState{TargetGUID: targetGUID, MapID: target.Map, LootType: 1, Items: make(map[uint8]lootItem)}
		s.server.creatureLoot[targetGUID] = loot
		s.server.creatureLoot[stdKey] = loot
	}
	s.server.lootMu.Unlock()
	if !newLoot {
		s.activeLoot = loot
		return s.sendLootResponse(loot) == nil
	}
	// Query min/max gold and lootid from creature_template
	var minGold, maxGold, lootID int64
	_ = wdb.QueryRowContext(ctx, "SELECT minGold, maxGold FROM creature_template WHERE entry = ? LIMIT 1", creatureEntry).Scan(&minGold, &maxGold)
	_ = wdb.QueryRowContext(ctx, "SELECT lootid FROM creature_template WHERE entry = ? LIMIT 1", creatureEntry).Scan(&lootID)
	if lootID == 0 {
		lootID = int64(creatureEntry)
	}
	if maxGold > minGold && maxGold > 0 {
		loot.Money = uint32(minGold + int64(rand.Intn(int(maxGold-minGold+1))))
	} else if minGold > 0 {
		loot.Money = uint32(minGold)
	}
	// Query creature_loot_template
	rows, err := wdb.QueryContext(ctx, `SELECT l.Item, l.Chance, l.MinCount, l.MaxCount, COALESCE(t.displayid, 0)
		FROM creature_loot_template AS l
		LEFT JOIN item_template AS t ON t.entry = l.Item
		WHERE l.Entry = ? ORDER BY l.Item LIMIT 16`, lootID)
	if err == nil {
		defer rows.Close()
		var slot uint8 = 0
		for rows.Next() {
			var itemID int64
			var chance float64
			var minCount, maxCount, displayID int64
			if err := rows.Scan(&itemID, &chance, &minCount, &maxCount, &displayID); err != nil {
				continue
			}
			// Roll chance (0-100%)
			roll := rand.Float64() * 100.0
			if chance > 0 && roll > chance {
				continue
			}
			count := uint32(minCount)
			if maxCount > minCount {
				count += uint32(rand.Intn(int(maxCount - minCount + 1)))
			}
			if count == 0 {
				count = 1
			}
			loot.Items[slot] = lootItem{
				Slot:          slot,
				ItemEntry:     uint32(itemID),
				Count:         count,
				DisplayInfoID: uint32(displayID),
			}
			slot++
			if slot >= 16 {
				break
			}
		}
	}
	s.activeLoot = loot
	return s.sendLootResponse(loot) == nil
}

func (s *session) sendLootResponse(loot *activeLootState) error {
	packet := protocol.NewBuffer(8 + 1 + 4 + 1 + len(loot.Items)*22)
	packet.WriteU64(loot.TargetGUID)
	packet.WriteU8(loot.LootType)
	packet.WriteU32(loot.Money)
	packet.WriteU8(uint8(len(loot.Items)))
	for _, it := range loot.Items {
		packet.WriteU8(it.Slot)
		packet.WriteU32(it.ItemEntry)
		packet.WriteU32(it.Count)
		packet.WriteU32(it.DisplayInfoID)
		packet.WriteU32(0) // RandomPropertyId
		packet.WriteU32(0) // RandomSuffix
		packet.WriteU8(0)  // LootSlotType (Normal)
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_LOOT_RESPONSE), packet.Bytes(), true); err != nil {
		return err
	}
	s.debug("loot response sent", "account", s.accountName, "target", loot.TargetGUID, "money", loot.Money, "items", len(loot.Items))

	if s.server != nil && s.groupID != 0 {
		s.server.groupsMu.Lock()
		grp := s.server.groups[s.groupID]
		s.server.groupsMu.Unlock()
		if grp != nil {
			if grp.LootMethod == 2 { // Master Loot
				s.sendLootMasterList(loot)
			} else if grp.LootMethod == 3 || grp.LootMethod == 4 { // Group Loot / Need Before Greed
				for _, it := range loot.Items {
					s.server.startGroupLootRoll(loot.TargetGUID, uint32(it.Slot), it.ItemEntry, it.Count, loot.MapID, s.groupID)
				}
			}
		}
	}
	return nil
}

func (s *session) sendLootMasterList(loot *activeLootState) {
	if s.server == nil || s.groupID == 0 {
		return
	}
	s.server.groupsMu.Lock()
	grp := s.server.groups[s.groupID]
	s.server.groupsMu.Unlock()
	if grp == nil || grp.LootMethod != 2 { // 2 = MASTER_LOOT
		return
	}
	members := s.server.getGroupSessions(s.groupID)
	buf := protocol.NewBuffer(1 + len(members)*8)
	buf.WriteU8(uint8(len(members)))
	for _, m := range members {
		buf.WriteU64(m.playerGUID)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_MASTER_LIST), buf.Bytes(), true)
}

func (s *session) sendLootError(guid uint64, code uint8) error {
	packet := protocol.NewBuffer(10)
	packet.WriteU64(guid)
	packet.WriteU8(0)
	packet.WriteU8(code)
	return s.write(uint16(protocol.OpcodeSMSG_LOOT_RESPONSE), packet.Bytes(), true)
}

func (s *session) clearCreatureLoot(loot *activeLootState) {
	if loot == nil || s.server == nil {
		return
	}
	high := uint16(loot.TargetGUID >> 48)
	if high == 0xF110 {
		s.server.lootMu.Lock()
		delete(s.server.creatureLoot, loot.TargetGUID)
		s.server.lootMu.Unlock()
		return
	}
	guid := uint32(loot.TargetGUID & 0x00FFFFFF)
	creatureEntry := uint32((loot.TargetGUID >> 24) & 0x00FFFFFF)
	stdKey := creatureWorldGUID(guid, creatureEntry)

	s.server.lootMu.Lock()
	delete(s.server.creatureLoot, loot.TargetGUID)
	delete(s.server.creatureLoot, stdKey)
	s.server.lootMu.Unlock()
	s.server.broadcastCreatureValuesUpdate(loot.MapID, loot.TargetGUID, map[int]uint32{unitFieldDynamicFlags: 0})
	s.server.broadcastCreatureValuesUpdate(loot.MapID, stdKey, map[int]uint32{unitFieldDynamicFlags: 0})
}

func (s *session) handleLootMoney(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil || s.activeLoot == nil || s.activeLoot.Money == 0 {
		return true
	}
	copper := s.activeLoot.Money
	s.activeLoot.Money = 0
	s.player.Money += copper
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	}
	notify := protocol.NewBuffer(5)
	notify.WriteU32(copper)
	notify.WriteU8(1) // Alone
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_MONEY_NOTIFY), notify.Bytes(), true)
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_CLEAR_MONEY), nil, true)
	s.sendPlayerUpdate()
	if s.activeLoot.Money == 0 && len(s.activeLoot.Items) == 0 {
		s.clearCreatureLoot(s.activeLoot)
	}
	s.debug("loot money collected", "account", s.accountName, "copper", copper)
	return true
}

func (s *session) handleAutostoreLootItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.activeLoot == nil || len(payload) < 1 {
		return true
	}
	lootSlot := payload[0]
	it, ok := s.activeLoot.Items[lootSlot]
	if !ok {
		return true
	}
	high := uint16(s.activeLoot.TargetGUID >> 48)
	if high == 0xF110 {
		if s.activeLoot.MapID != s.player.Map {
			return s.sendLootError(s.activeLoot.TargetGUID, 4) == nil
		}
	} else {
		target, validTarget := s.getCombatTarget(ctx, s.activeLoot.TargetGUID)
		if !validTarget || target.Map != s.player.Map || target.Health != 0 || distance3D(s.player.X, s.player.Y, s.player.Z, target.X, target.Y, target.Z) > 5.0 {
			return s.sendLootError(s.activeLoot.TargetGUID, 4) == nil
		}
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	res, err := s.storeOrStackItem(ctx, s.playerGUID, it.ItemEntry, it.Count)
	if err != nil {
		if errors.Is(err, errInventoryFull) {
			s.sendEquipError(equipErrInvFull, 0)
		}
		return true
	}
	delete(s.activeLoot.Items, lootSlot)
	removed := protocol.NewBuffer(1)
	removed.WriteU8(lootSlot)
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), removed.Bytes(), true)
	_ = s.sendInventoryItems(ctx)

	slotForPush := uint32(res.Slot)
	if res.IsStack {
		slotForPush = 0xFFFFFFFF
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_ITEM_PUSH_RESULT), buildLootItemPushResult(s.playerGUID, res.ClientBag, slotForPush, it.ItemEntry, it.Count, res.InventoryCount), true)
	s.sendPlayerUpdate()
	if s.activeLoot.Money == 0 && len(s.activeLoot.Items) == 0 {
		s.clearCreatureLoot(s.activeLoot)
	}
	s.debug("loot item stored", "account", s.accountName, "item", it.ItemEntry, "slot", res.Slot, "bag", res.ClientBag, "stacked", res.IsStack)
	return true
}

func buildLootItemPushResult(playerGUID uint64, bag uint8, slot, entry, count, inventoryCount uint32) []byte {
	packet := protocol.NewBuffer(48)
	packet.WriteU64(playerGUID)
	packet.WriteU32(0)
	packet.WriteU32(0)
	packet.WriteU32(1)
	packet.WriteU8(bag)
	packet.WriteU32(slot)
	packet.WriteU32(entry)
	packet.WriteU32(0)
	packet.WriteI32(0)
	packet.WriteU32(count)
	packet.WriteU32(inventoryCount)
	return packet.Bytes()
}

func (s *session) handleLootRelease(payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	targetGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	if s.activeLoot == nil || s.activeLoot.TargetGUID != targetGUID {
		return true
	}
	loot := s.activeLoot
	s.activeLoot = nil
	if loot.Money == 0 && len(loot.Items) == 0 {
		s.clearCreatureLoot(loot)
	}
	release := protocol.NewBuffer(9)
	release.WriteU64(targetGUID)
	release.WriteU8(1)
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_RELEASE_RESPONSE), release.Bytes(), true)
	s.debug("loot released", "account", s.accountName, "target", targetGUID)
	return true
}

// handleLootMasterGive processes CMSG_LOOT_MASTER_GIVE (0x2A3).
// Reference: WorldSession::HandleLootMasterGiveOpcode (LootHandler.cpp:392).
func (s *session) handleLootMasterGive(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 17 {
		return true
	}
	r := protocol.NewReader(payload)
	lootGUID, _ := r.ReadU64()
	slotID, _ := r.ReadU8()
	targetGUID, _ := r.ReadU64()

	var targetSess *session
	if s.server != nil {
		targetSess = s.server.findSessionByGUID(targetGUID)
	}
	if targetSess == nil || targetSess.player == nil {
		buf := protocol.NewBuffer(9)
		buf.WriteU64(lootGUID)
		buf.WriteU8(3) // LOOT_ERROR_PLAYER_NOT_FOUND
		_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), buf.Bytes(), true)
		return true
	}

	if s.activeLoot != nil {
		if it, ok := s.activeLoot.Items[slotID]; ok {
			if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
				res, err := targetSess.storeOrStackItem(ctx, targetGUID, it.ItemEntry, it.Count)
				if err != nil {
					buf := protocol.NewBuffer(9)
					buf.WriteU64(lootGUID)
					buf.WriteU8(4) // LOOT_ERROR_INVENTORY_FULL
					_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), buf.Bytes(), true)
					return true
				}
				_ = res
				delete(s.activeLoot.Items, slotID)
				_ = targetSess.sendInventoryItems(ctx)
				targetSess.sendPlayerUpdate()
			} else {
				delete(s.activeLoot.Items, slotID)
			}
			buf := protocol.NewBuffer(9)
			buf.WriteU64(lootGUID)
			buf.WriteU8(slotID)
			_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), buf.Bytes(), true)
		}
	}
	return true
}

func buildLootRollPayload(itemGUID uint64, slot uint32, rollType uint8) []byte {
	buf := protocol.NewBuffer(13)
	buf.WriteU64(itemGUID)
	buf.WriteU32(slot)
	buf.WriteU8(rollType)
	return buf.Bytes()
}

func (s *Server) startGroupLootRoll(sourceGUID uint64, slot uint32, itemEntry uint32, itemCount uint32, mapID uint32, groupID uint64) {
	if groupID == 0 {
		return
	}
	members := s.getGroupSessions(groupID)
	if len(members) == 0 {
		return
	}

	rollKey := fmt.Sprintf("%d:%d", sourceGUID, slot)
	s.lootMu.Lock()
	if s.groupRolls == nil {
		s.groupRolls = make(map[string]*activeGroupRoll)
	}
	if existing := s.groupRolls[rollKey]; existing != nil {
		s.lootMu.Unlock()
		return
	}

	roll := &activeGroupRoll{
		SourceGUID:          sourceGUID,
		Slot:                slot,
		ItemEntry:           itemEntry,
		ItemCount:           itemCount,
		RollVoteMask:        0x0F, // ROLL_ALL_TYPE_MASK
		GroupID:             groupID,
		MapID:               mapID,
		StartedAt:           time.Now(),
		Duration:            60 * time.Second,
		TotalPlayersRolling: len(members),
		Votes:               make(map[uint64]uint8),
		Rolls:               make(map[uint64]uint8),
	}
	s.groupRolls[rollKey] = roll
	s.lootMu.Unlock()

	// Build SMSG_LOOT_START_ROLL (0x2A1)
	buf := protocol.NewBuffer(8 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 1)
	buf.WriteU64(sourceGUID)
	buf.WriteU32(mapID)
	buf.WriteU32(slot)
	buf.WriteU32(itemEntry)
	buf.WriteU32(0) // randomSuffix
	buf.WriteU32(0) // randomPropId
	buf.WriteU32(itemCount)
	buf.WriteU32(60000) // 60s countdown
	buf.WriteU8(roll.RollVoteMask)

	s.broadcastToGroup(groupID, uint16(protocol.OpcodeSMSG_LOOT_START_ROLL), buf.Bytes())

	// Auto-pass check for players with PassOnGroupLoot
	for _, m := range members {
		if m.player != nil && m.player.PassOnGroupLoot {
			m.handleLootRoll(context.Background(), buildLootRollPayload(sourceGUID, slot, 0))
		}
	}

	// Arm 60s countdown
	roll.Timer = time.AfterFunc(60*time.Second, func() {
		s.resolveGroupLootRoll(rollKey)
	})
}

func (s *Server) resolveGroupLootRoll(rollKey string) {
	s.lootMu.Lock()
	roll := s.groupRolls[rollKey]
	if roll == nil {
		s.lootMu.Unlock()
		return
	}
	delete(s.groupRolls, rollKey)
	if roll.Timer != nil {
		roll.Timer.Stop()
	}
	s.lootMu.Unlock()

	var winnerGUID uint64
	var maxRoll uint8
	var winningType uint8

	if roll.TotalNeed > 0 {
		for guid, vote := range roll.Votes {
			if vote == 1 { // NEED
				rNum := roll.Rolls[guid]
				if rNum > maxRoll || winnerGUID == 0 {
					maxRoll = rNum
					winnerGUID = guid
					winningType = 1
				}
			}
		}
	} else if roll.TotalGreed > 0 {
		for guid, vote := range roll.Votes {
			if vote == 2 || vote == 3 { // GREED or DISENCHANT
				rNum := roll.Rolls[guid]
				if rNum > maxRoll || winnerGUID == 0 {
					maxRoll = rNum
					winnerGUID = guid
					winningType = vote
				}
			}
		}
	}

	if winnerGUID != 0 {
		// Broadcast SMSG_LOOT_ROLL_WON (0x29F)
		wonBuf := protocol.NewBuffer(35)
		wonBuf.WriteU64(roll.SourceGUID)
		wonBuf.WriteU32(roll.Slot)
		wonBuf.WriteU32(roll.ItemEntry)
		wonBuf.WriteU32(roll.RandomSuffix)
		wonBuf.WriteU32(roll.RandomPropID)
		wonBuf.WriteU64(winnerGUID)
		wonBuf.WriteU8(maxRoll)
		wonBuf.WriteU8(winningType)
		s.broadcastToGroup(roll.GroupID, uint16(protocol.OpcodeSMSG_LOOT_ROLL_WON), wonBuf.Bytes())

		s.deliverGroupLootItem(roll, winnerGUID)
	} else {
		// Broadcast SMSG_LOOT_ALL_PASSED (0x29E)
		passBuf := protocol.NewBuffer(24)
		passBuf.WriteU64(roll.SourceGUID)
		passBuf.WriteU32(roll.Slot)
		passBuf.WriteU32(roll.ItemEntry)
		passBuf.WriteU32(roll.RandomPropID)
		passBuf.WriteU32(roll.RandomSuffix)
		s.broadcastToGroup(roll.GroupID, uint16(protocol.OpcodeSMSG_LOOT_ALL_PASSED), passBuf.Bytes())
	}
}

func (s *Server) deliverGroupLootItem(roll *activeGroupRoll, winnerGUID uint64) {
	winnerSess := s.findSessionByGUID(winnerGUID)
	if winnerSess == nil || winnerSess.player == nil || s.CharactersStore == nil || s.CharactersStore.DB == nil {
		return
	}
	ctx := context.Background()
	res, err := winnerSess.storeOrStackItem(ctx, winnerGUID, roll.ItemEntry, roll.ItemCount)
	if err != nil {
		buf := protocol.NewBuffer(9)
		buf.WriteU64(roll.SourceGUID)
		buf.WriteU8(4) // LOOT_ERROR_INVENTORY_FULL
		_ = winnerSess.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), buf.Bytes(), true)
		return
	}
	_ = res
	_ = winnerSess.sendInventoryItems(ctx)
	winnerSess.sendPlayerUpdate()

	s.lootMu.Lock()
	if cLoot := s.creatureLoot[roll.SourceGUID]; cLoot != nil {
		delete(cLoot.Items, uint8(roll.Slot))
	}
	s.lootMu.Unlock()

	remBuf := protocol.NewBuffer(9)
	remBuf.WriteU64(roll.SourceGUID)
	remBuf.WriteU8(uint8(roll.Slot))
	s.broadcastToGroup(roll.GroupID, uint16(protocol.OpcodeSMSG_LOOT_REMOVED), remBuf.Bytes())
}

// handleLootRoll processes CMSG_LOOT_ROLL (0x2A0).
// Reference: WorldSession::HandleLootRoll (GroupHandler.cpp:470), Group::SendLootRoll (Group.cpp:995), Group::CountRollVote (Group.cpp:1452).
func (s *session) handleLootRoll(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 13 {
		return true
	}
	r := protocol.NewReader(payload)
	itemGUID, _ := r.ReadU64()
	itemSlot, _ := r.ReadU32()
	rollType, _ := r.ReadU8() // 0 = pass, 1 = need, 2 = greed, 3 = disenchant

	if s.server == nil || s.groupID == 0 {
		return true
	}

	rollKey := fmt.Sprintf("%d:%d", itemGUID, itemSlot)
	s.server.lootMu.Lock()
	roll := s.server.groupRolls[rollKey]
	if roll == nil {
		var itemEntry uint32
		if loot := s.server.creatureLoot[itemGUID]; loot != nil {
			if li, ok := loot.Items[uint8(itemSlot)]; ok {
				itemEntry = li.ItemEntry
			}
		}
		s.server.lootMu.Unlock()

		rollNumber := uint8(128) // pass
		if rollType > 0 {
			rollNumber = uint8(rand.Intn(100) + 1) // 1..100
		}

		buf := protocol.NewBuffer(35)
		buf.WriteU64(itemGUID)     // sourceGuid (guid of loot object)
		buf.WriteU32(itemSlot)     // slot
		buf.WriteU64(s.playerGUID) // targetGuid (rolling player)
		buf.WriteU32(itemEntry)    // itemEntryId
		buf.WriteU32(0)            // randomSuffix
		buf.WriteU32(0)            // randomPropId
		buf.WriteU8(rollNumber)    // rollNumber
		buf.WriteU8(rollType)      // rollType (0: pass, 1: need, 2: greed, 3: disenchant)
		buf.WriteU8(0)             // autoPass
		s.server.broadcastToGroup(s.groupID, uint16(protocol.OpcodeSMSG_LOOT_ROLL), buf.Bytes())
		return true
	}

	// If player already voted on this roll, ignore duplicate
	if _, voted := roll.Votes[s.playerGUID]; voted {
		s.server.lootMu.Unlock()
		return true
	}

	rollNumber := uint8(128) // pass
	if rollType > 0 {
		rollNumber = uint8(rand.Intn(100) + 1) // 1..100
	}

	roll.Votes[s.playerGUID] = rollType
	roll.Rolls[s.playerGUID] = rollNumber

	switch rollType {
	case 0:
		roll.TotalPass++
	case 1:
		roll.TotalNeed++
	default:
		roll.TotalGreed++
	}

	itemEntry := roll.ItemEntry
	randomSuffix := roll.RandomSuffix
	randomPropID := roll.RandomPropID
	totalDone := (roll.TotalPass + roll.TotalNeed + roll.TotalGreed) >= roll.TotalPlayersRolling
	s.server.lootMu.Unlock()

	buf := protocol.NewBuffer(35)
	buf.WriteU64(itemGUID)
	buf.WriteU32(itemSlot)
	buf.WriteU64(s.playerGUID)
	buf.WriteU32(itemEntry)
	buf.WriteU32(randomSuffix)
	buf.WriteU32(randomPropID)
	buf.WriteU8(rollNumber)
	buf.WriteU8(rollType)
	buf.WriteU8(0) // autoPass
	s.server.broadcastToGroup(s.groupID, uint16(protocol.OpcodeSMSG_LOOT_ROLL), buf.Bytes())

	if totalDone {
		s.server.resolveGroupLootRoll(rollKey)
	}

	return true
}

// handleOptOutOfLoot processes CMSG_OPT_OUT_OF_LOOT (0x409).
// Reference: WorldSession::HandleOptOutOfLootOpcode (GroupHandler.cpp:1066).
func (s *session) handleOptOutOfLoot(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	passOnLoot, _ := r.ReadU32()
	s.player.PassOnGroupLoot = (passOnLoot != 0)
	return true
}
