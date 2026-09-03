package world

import (
	"context"
	"math/rand"

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
	return nil
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
	target, validTarget := s.getCombatTarget(ctx, s.activeLoot.TargetGUID)
	if !validTarget || target.Map != s.player.Map || target.Health != 0 || distance3D(s.player.X, s.player.Y, s.player.Z, target.X, target.Y, target.Z) > 5.0 {
		return s.sendLootError(s.activeLoot.TargetGUID, 4) == nil
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	// Find free slot in backpack (23 to 38)
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
		return true
	}
	var nextGUID int64
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&nextGUID)
	if nextGUID <= 0 {
		nextGUID = 1
	}
	tx, err := cdb.BeginTx(ctx, nil)
	if err != nil {
		return true
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text) VALUES (?, ?, ?, 0, ?, 0, '', 0, '', 0, 0, 0, '')", nextGUID, it.ItemEntry, s.playerGUID, it.Count); err != nil {
		_ = tx.Rollback()
		s.debug("loot item insert failed", "account", s.accountName, "item", it.ItemEntry, "error", err)
		return true
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)", s.playerGUID, freeSlot, nextGUID); err != nil {
		_ = tx.Rollback()
		s.debug("loot inventory insert failed", "account", s.accountName, "item", it.ItemEntry, "error", err)
		return true
	}
	if err = tx.Commit(); err != nil {
		return true
	}
	inventoryCount := int64(0)
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(SUM(ii.count), 0) FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = ? AND ii.itemEntry = ?", s.playerGUID, it.ItemEntry).Scan(&inventoryCount)
	delete(s.activeLoot.Items, lootSlot)
	removed := protocol.NewBuffer(1)
	removed.WriteU8(lootSlot)
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), removed.Bytes(), true)
	_ = s.sendItemCreate(uint64(nextGUID), it.ItemEntry, it.Count, 0, freeSlot)
	_ = s.sendInventoryItems(ctx)
	_ = s.write(uint16(protocol.OpcodeSMSG_ITEM_PUSH_RESULT), buildLootItemPushResult(s.playerGUID, 0, uint32(freeSlot), it.ItemEntry, it.Count, uint32(inventoryCount)), true)
	s.sendPlayerUpdate()
	if s.activeLoot.Money == 0 && len(s.activeLoot.Items) == 0 {
		s.clearCreatureLoot(s.activeLoot)
	}
	s.debug("loot item stored", "account", s.accountName, "item", it.ItemEntry, "slot", freeSlot)
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
			delete(s.activeLoot.Items, slotID)
			if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
				cdb := s.server.CharactersStore.DB
				var nextGUID int64
				_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&nextGUID)
				if nextGUID <= 0 {
					nextGUID = 1
				}
				_, _ = cdb.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text) VALUES (?, ?, ?, 0, ?, 0, '', 0, '', 0, 0, 0, '')", nextGUID, it.ItemEntry, targetGUID, it.Count)
				_, _ = cdb.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, 23, ?)", targetGUID, nextGUID)
				_ = targetSess.sendInventoryItems(ctx)
			}
			buf := protocol.NewBuffer(9)
			buf.WriteU64(lootGUID)
			buf.WriteU8(slotID)
			_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), buf.Bytes(), true)
		}
	}
	return true
}

// handleLootRoll processes CMSG_LOOT_ROLL (0x2A0).
// Reference: WorldSession::HandleLootRoll (GroupHandler.cpp:470).
func (s *session) handleLootRoll(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 13 {
		return true
	}
	r := protocol.NewReader(payload)
	itemGUID, _ := r.ReadU64()
	itemSlot, _ := r.ReadU32()
	rollType, _ := r.ReadU8() // 0 = pass, 1 = need, 2 = greed, 3 = disenchant

	if s.server != nil && s.groupID != 0 {
		buf := protocol.NewBuffer(22)
		buf.WriteU64(itemGUID)
		buf.WriteU32(itemSlot)
		buf.WriteU64(s.playerGUID)
		buf.WriteU8(rollType)
		buf.WriteU8(0) // autoPass
		s.server.broadcastToGroup(s.groupID, uint16(protocol.OpcodeSMSG_LOOT_ROLL), buf.Bytes())
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
