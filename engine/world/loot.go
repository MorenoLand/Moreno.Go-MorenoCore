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
	creatureEntry := uint32((targetGUID >> 24) & 0xFFFFFF)
	loot := &activeLootState{
		TargetGUID: targetGUID,
		LootType:   1, // LOOT_CORPSE
		Items:      make(map[uint8]lootItem),
	}
	// Query min/max gold from creature_template
	var minGold, maxGold int64
	_ = wdb.QueryRowContext(ctx, "SELECT minGold, maxGold FROM creature_template WHERE entry = ? LIMIT 1", creatureEntry).Scan(&minGold, &maxGold)
	if maxGold > minGold && maxGold > 0 {
		loot.Money = uint32(minGold + int64(rand.Intn(int(maxGold-minGold+1))))
	} else if minGold > 0 {
		loot.Money = uint32(minGold)
	}
	// Query creature_loot_template
	rows, err := wdb.QueryContext(ctx, `SELECT l.Item, l.Chance, l.MinCount, l.MaxCount, COALESCE(t.displayid, 0)
		FROM creature_loot_template AS l
		LEFT JOIN item_template AS t ON t.entry = l.Item
		WHERE l.Entry = ? ORDER BY l.Item LIMIT 16`, creatureEntry)
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
	// Send SMSG_LOOT_RESPONSE (0x160)
	packet := protocol.NewBuffer(8 + 1 + 4 + 1 + len(loot.Items)*22)
	packet.WriteU64(targetGUID)
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
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_RESPONSE), packet.Bytes(), true)
	s.debug("loot response sent", "account", s.accountName, "target", targetGUID, "money", loot.Money, "items", len(loot.Items))
	return true
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
	_, _ = cdb.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, played_time, text) VALUES (?, ?, ?, 0, ?, 0, '', 0, '', 0, 0, 0, '')", nextGUID, it.ItemEntry, s.playerGUID, it.Count)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)", s.playerGUID, freeSlot, nextGUID)
	delete(s.activeLoot.Items, lootSlot)
	removed := protocol.NewBuffer(1)
	removed.WriteU8(lootSlot)
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_REMOVED), removed.Bytes(), true)
	s.sendPlayerUpdate()
	s.debug("loot item stored", "account", s.accountName, "item", it.ItemEntry, "slot", freeSlot)
	return true
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
	s.activeLoot = nil
	release := protocol.NewBuffer(9)
	release.WriteU64(targetGUID)
	release.WriteU8(1)
	_ = s.write(uint16(protocol.OpcodeSMSG_LOOT_RELEASE_RESPONSE), release.Bytes(), true)
	s.debug("loot released", "account", s.accountName, "target", targetGUID)
	return true
}
