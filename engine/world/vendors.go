package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type vendorItemRecord struct {
	Slot          uint32
	ItemEntry     uint32
	DisplayInfoID uint32
	MaxCount      int32
	BuyPrice      uint32
	MaxDurability uint32
	BuyCount      uint32
	ExtendedCost  uint32
}

func (s *session) handleListInventory(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	vendorGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	return s.sendVendorList(ctx, vendorGUID)
}

func (s *session) sendVendorList(ctx context.Context, vendorGUID uint64) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return true
	}
	creatureEntry := uint32((vendorGUID >> 24) & 0xFFFFFF)
	rows, err := s.server.WorldStore.DB.QueryContext(ctx, `SELECT v.slot, v.item, v.maxcount, v.ExtendedCost,
		COALESCE(t.displayid, 0), COALESCE(t.BuyPrice, 0), COALESCE(t.MaxDurability, 0), COALESCE(t.BuyCount, 1)
		FROM npc_vendor AS v
		LEFT JOIN item_template AS t ON t.entry = v.item
		WHERE v.entry = ? ORDER BY v.slot LIMIT 128`, creatureEntry)
	if err != nil {
		return true
	}
	defer rows.Close()
	var items []vendorItemRecord
	var fallbackSlot uint32 = 1
	for rows.Next() {
		var slot, item, maxCount, extCost, display, buyPrice, maxDur, buyCount int64
		if err := rows.Scan(&slot, &item, &maxCount, &extCost, &display, &buyPrice, &maxDur, &buyCount); err != nil {
			continue
		}
		itemSlot := uint32(slot)
		if itemSlot == 0 {
			itemSlot = fallbackSlot
		}
		fallbackSlot++
		inStock := int32(-1)
		if maxCount > 0 {
			inStock = int32(maxCount)
		}
		if buyCount <= 0 {
			buyCount = 1
		}
		items = append(items, vendorItemRecord{
			Slot:          itemSlot,
			ItemEntry:     uint32(item),
			DisplayInfoID: uint32(display),
			MaxCount:      inStock,
			BuyPrice:      uint32(buyPrice),
			MaxDurability: uint32(maxDur),
			BuyCount:      uint32(buyCount),
			ExtendedCost:  uint32(extCost),
		})
	}
	packet := protocol.NewBuffer(8 + 1 + len(items)*32)
	packet.WriteU64(vendorGUID)
	packet.WriteU8(uint8(len(items)))
	for _, it := range items {
		packet.WriteU32(it.Slot)
		packet.WriteU32(it.ItemEntry)
		packet.WriteU32(it.DisplayInfoID)
		packet.WriteU32(uint32(it.MaxCount))
		packet.WriteU32(it.BuyPrice)
		packet.WriteU32(it.MaxDurability)
		packet.WriteU32(it.BuyCount)
		packet.WriteU32(it.ExtendedCost)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_LIST_INVENTORY), packet.Bytes(), true)
	s.debug("vendor list sent", "account", s.accountName, "vendor", vendorGUID, "items", len(items))
	return true
}

func (s *session) handleBuyItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 14 {
		return true
	}
	reader := protocol.NewReader(payload)
	vendorGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	itemEntry, err := reader.ReadU32()
	if err != nil {
		return false
	}
	slot, err := reader.ReadU32()
	if err != nil {
		return false
	}
	count, err := reader.ReadU32()
	if err != nil || count == 0 {
		count = 1
	}
	return s.processBuyItem(ctx, vendorGUID, itemEntry, slot, count)
}

func (s *session) handleBuyItemInSlot(ctx context.Context, payload []byte) bool {
	return s.handleBuyItem(ctx, payload)
}

func (s *session) processBuyItem(ctx context.Context, vendorGUID uint64, itemEntry, slot, count uint32) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}
	var buyPrice, buyCount int64
	err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT BuyPrice, BuyCount FROM item_template WHERE entry = ? LIMIT 1", itemEntry).Scan(&buyPrice, &buyCount)
	if err != nil {
		return true
	}
	if buyCount <= 0 {
		buyCount = 1
	}
	totalCost := uint32(buyPrice) * (count / uint32(buyCount))
	if count%uint32(buyCount) != 0 {
		totalCost = uint32(buyPrice) * ((count / uint32(buyCount)) + 1)
	}
	if s.player.Money < totalCost {
		_ = s.write(uint16(protocol.OpcodeSMSG_BUY_FAILED), buildBuyFailed(vendorGUID, itemEntry, 2), true) // BUY_ERR_NOT_ENOUGHT_MONEY = 2
		return true
	}
	// Find free inventory slot (23 to 38 in backpack)
	cdb := s.server.CharactersStore.DB
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
		_ = s.write(uint16(protocol.OpcodeSMSG_BUY_FAILED), buildBuyFailed(vendorGUID, itemEntry, 1), true) // BUY_ERR_CANT_CARRY_MORE = 1
		return true
	}
	s.player.Money -= totalCost
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	// Create item instance
	var nextGUID int64
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&nextGUID)
	if nextGUID <= 0 {
		nextGUID = 1
	}
	_, _ = cdb.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, played_time, text) VALUES (?, ?, ?, 0, ?, 0, '', 0, '', 0, 0, 0, '')", nextGUID, itemEntry, s.playerGUID, count)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)", s.playerGUID, freeSlot, nextGUID)
	_ = s.write(uint16(protocol.OpcodeSMSG_BUY_ITEM), buildBuySucceeded(vendorGUID, itemEntry, count, count), true)
	_ = s.sendItemCreate(uint64(nextGUID), itemEntry, count, 0, freeSlot)
	s.debug("item bought from vendor", "account", s.accountName, "item", itemEntry, "count", count, "cost", totalCost, "slot", freeSlot)
	return true
}

func (s *session) handleSellItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 17 {
		return true
	}
	reader := protocol.NewReader(payload)
	vendorGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	itemGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	count, err := reader.ReadU8()
	if err != nil || count == 0 {
		count = 1
	}
	cdb := s.server.CharactersStore.DB
	wdb := s.server.WorldStore.DB
	if cdb == nil || wdb == nil {
		return true
	}
	var itemEntry, currentCount int64
	err = cdb.QueryRowContext(ctx, `SELECT ii.itemEntry, ii.count FROM character_inventory AS ci
		JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.item = ? LIMIT 1`, s.playerGUID, itemGUID).Scan(&itemEntry, &currentCount)
	if err != nil || itemEntry == 0 {
		return true
	}
	var sellPrice int64
	_ = wdb.QueryRowContext(ctx, "SELECT SellPrice FROM item_template WHERE entry = ? LIMIT 1", itemEntry).Scan(&sellPrice)
	if sellPrice <= 0 {
		sellPrice = 1
	}
	earned := uint32(sellPrice) * uint32(count)
	s.player.Money += earned
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	if currentCount <= int64(count) {
		_, _ = cdb.ExecContext(ctx, "DELETE FROM character_inventory WHERE guid = ? AND item = ?", s.playerGUID, itemGUID)
		_, _ = cdb.ExecContext(ctx, "DELETE FROM item_instance WHERE guid = ?", itemGUID)
	} else {
		_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET count = count - ? WHERE guid = ?", count, itemGUID)
	}
	s.syncEquipmentCache(ctx)
	_ = s.write(uint16(protocol.OpcodeSMSG_SELL_ITEM), buildSellResult(vendorGUID, itemGUID, 0), true)
	s.sendPlayerUpdate()
	s.debug("item sold to vendor", "account", s.accountName, "item", itemEntry, "guid", itemGUID, "count", count, "earned", earned)
	return true
}

func buildBuyFailed(vendorGUID uint64, itemEntry uint32, result uint8) []byte {
	buf := protocol.NewBuffer(13)
	buf.WriteU64(vendorGUID)
	buf.WriteU32(itemEntry)
	buf.WriteU8(result)
	return buf.Bytes()
}

func buildBuySucceeded(vendorGUID uint64, itemEntry, newCount, buyCount uint32) []byte {
	buf := protocol.NewBuffer(20)
	buf.WriteU64(vendorGUID)
	buf.WriteU32(itemEntry)
	buf.WriteU32(newCount)
	buf.WriteU32(buyCount)
	return buf.Bytes()
}

func buildSellResult(vendorGUID, itemGUID uint64, result uint8) []byte {
	buf := protocol.NewBuffer(17)
	buf.WriteU64(vendorGUID)
	buf.WriteU64(itemGUID)
	buf.WriteU8(result)
	return buf.Bytes()
}
