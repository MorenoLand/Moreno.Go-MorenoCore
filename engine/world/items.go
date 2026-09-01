package world

import (
	"context"
	"strconv"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	equipSlotHead     uint8 = 0
	equipSlotNeck     uint8 = 1
	equipSlotShoulder uint8 = 2
	equipSlotBody     uint8 = 3
	equipSlotChest    uint8 = 4
	equipSlotWaist    uint8 = 5
	equipSlotLegs     uint8 = 6
	equipSlotFeet     uint8 = 7
	equipSlotWrists   uint8 = 8
	equipSlotHands    uint8 = 9
	equipSlotFinger1  uint8 = 10
	equipSlotFinger2  uint8 = 11
	equipSlotTrinket1 uint8 = 12
	equipSlotTrinket2 uint8 = 13
	equipSlotBack     uint8 = 14
	equipSlotMainhand uint8 = 15
	equipSlotOffhand  uint8 = 16
	equipSlotRanged   uint8 = 17
	equipSlotTabard   uint8 = 18
	equipSlotEnd      uint8 = 19

	invSlotBagStart  uint8 = 19
	invSlotBagEnd    uint8 = 23
	invSlotItemStart uint8 = 23
	invSlotItemEnd   uint8 = 39

	invSlotBag0 uint8 = 255
)

func (s *session) handleAutoEquipItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	srcBag := payload[0]
	srcSlot := payload[1]
	db := s.server.CharactersStore.DB
	if db == nil {
		return true
	}
	var itemGUID, itemEntry int64
	err := db.QueryRowContext(ctx, `SELECT ci.item, ii.itemEntry FROM character_inventory AS ci
		JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.bag = ? AND ci.slot = ? LIMIT 1`, s.playerGUID, srcBag, srcSlot).Scan(&itemGUID, &itemEntry)
	if err != nil || itemGUID == 0 || itemEntry == 0 {
		return true
	}
	var invType int64
	if s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT InventoryType FROM item_template WHERE entry = ? LIMIT 1", itemEntry).Scan(&invType)
	}
	destSlot := inventoryTypeToSlot(uint8(invType))
	if destSlot >= equipSlotEnd && (destSlot < invSlotBagStart || destSlot >= invSlotBagEnd) {
		return true
	}
	var existingItemGUID int64
	_ = db.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = 0 AND slot = ? LIMIT 1", s.playerGUID, destSlot).Scan(&existingItemGUID)
	if existingItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ?, bag = ? WHERE guid = ? AND item = ?", srcSlot, srcBag, s.playerGUID, existingItemGUID)
	}
	_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ?, bag = 0 WHERE guid = ? AND item = ?", destSlot, s.playerGUID, itemGUID)
	s.syncEquipmentCache(ctx)
	s.sendPlayerUpdate()
	s.debug("item auto-equipped", "account", s.accountName, "guid", s.playerGUID, "entry", itemEntry, "slot", destSlot)
	return true
}

func (s *session) handleAutoEquipItemSlot(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	reader := protocol.NewReader(payload)
	itemGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	dstSlot, err := reader.ReadU8()
	if err != nil {
		return false
	}
	if dstSlot >= equipSlotEnd {
		return true
	}
	db := s.server.CharactersStore.DB
	if db == nil {
		return true
	}
	var srcBag, srcSlot int64
	err = db.QueryRowContext(ctx, "SELECT bag, slot FROM character_inventory WHERE guid = ? AND item = ? LIMIT 1", s.playerGUID, itemGUID).Scan(&srcBag, &srcSlot)
	if err != nil {
		return true
	}
	var existingItemGUID int64
	_ = db.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = 0 AND slot = ? LIMIT 1", s.playerGUID, dstSlot).Scan(&existingItemGUID)
	if existingItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ?, bag = ? WHERE guid = ? AND item = ?", srcSlot, srcBag, s.playerGUID, existingItemGUID)
	}
	_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ?, bag = 0 WHERE guid = ? AND item = ?", dstSlot, s.playerGUID, itemGUID)
	s.syncEquipmentCache(ctx)
	s.sendPlayerUpdate()
	s.debug("item slot equipped", "account", s.accountName, "guid", s.playerGUID, "item", itemGUID, "slot", dstSlot)
	return true
}

func (s *session) handleSwapInvItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	srcSlot := payload[0]
	dstSlot := payload[1]
	if srcSlot == dstSlot {
		return true
	}
	db := s.server.CharactersStore.DB
	if db == nil {
		return true
	}
	var srcItemGUID, dstItemGUID int64
	_ = db.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = 0 AND slot = ? LIMIT 1", s.playerGUID, srcSlot).Scan(&srcItemGUID)
	_ = db.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = 0 AND slot = ? LIMIT 1", s.playerGUID, dstSlot).Scan(&dstItemGUID)
	if srcItemGUID == 0 && dstItemGUID == 0 {
		return true
	}
	if srcItemGUID != 0 && dstItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ? WHERE guid = ? AND item = ?", dstSlot, s.playerGUID, srcItemGUID)
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ? WHERE guid = ? AND item = ?", srcSlot, s.playerGUID, dstItemGUID)
	} else if srcItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ? WHERE guid = ? AND item = ?", dstSlot, s.playerGUID, srcItemGUID)
	} else if dstItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET slot = ? WHERE guid = ? AND item = ?", srcSlot, s.playerGUID, dstItemGUID)
	}
	s.syncEquipmentCache(ctx)
	s.sendPlayerUpdate()
	s.debug("inventory items swapped", "account", s.accountName, "src", srcSlot, "dst", dstSlot)
	return true
}

func (s *session) handleSwapItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	dstBag := payload[0]
	dstSlot := payload[1]
	srcBag := payload[2]
	srcSlot := payload[3]
	if dstBag == srcBag && dstSlot == srcSlot {
		return true
	}
	db := s.server.CharactersStore.DB
	if db == nil {
		return true
	}
	var srcItemGUID, dstItemGUID int64
	_ = db.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ? LIMIT 1", s.playerGUID, srcBag, srcSlot).Scan(&srcItemGUID)
	_ = db.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ? LIMIT 1", s.playerGUID, dstBag, dstSlot).Scan(&dstItemGUID)
	if srcItemGUID == 0 && dstItemGUID == 0 {
		return true
	}
	if srcItemGUID != 0 && dstItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET bag = ?, slot = ? WHERE guid = ? AND item = ?", dstBag, dstSlot, s.playerGUID, srcItemGUID)
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET bag = ?, slot = ? WHERE guid = ? AND item = ?", srcBag, srcSlot, s.playerGUID, dstItemGUID)
	} else if srcItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET bag = ?, slot = ? WHERE guid = ? AND item = ?", dstBag, dstSlot, s.playerGUID, srcItemGUID)
	} else if dstItemGUID != 0 {
		_, _ = db.ExecContext(ctx, "UPDATE character_inventory SET bag = ?, slot = ? WHERE guid = ? AND item = ?", srcBag, srcSlot, s.playerGUID, dstItemGUID)
	}
	s.syncEquipmentCache(ctx)
	s.sendPlayerUpdate()
	s.debug("items swapped across bags", "account", s.accountName, "srcBag", srcBag, "srcSlot", srcSlot, "dstBag", dstBag, "dstSlot", dstSlot)
	return true
}

func (s *session) handleDestroyItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 3 {
		return true
	}
	bag := payload[0]
	slot := payload[1]
	count := payload[2]
	db := s.server.CharactersStore.DB
	if db == nil {
		return true
	}
	var itemGUID, currentCount int64
	err := db.QueryRowContext(ctx, `SELECT ci.item, ii.count FROM character_inventory AS ci
		JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.bag = ? AND ci.slot = ? LIMIT 1`, s.playerGUID, bag, slot).Scan(&itemGUID, &currentCount)
	if err != nil || itemGUID == 0 {
		return true
	}
	if currentCount <= int64(count) || count == 0 {
		_, _ = db.ExecContext(ctx, "DELETE FROM character_inventory WHERE guid = ? AND item = ?", s.playerGUID, itemGUID)
		_, _ = db.ExecContext(ctx, "DELETE FROM item_instance WHERE guid = ?", itemGUID)
	} else {
		_, _ = db.ExecContext(ctx, "UPDATE item_instance SET count = count - ? WHERE guid = ?", count, itemGUID)
	}
	s.syncEquipmentCache(ctx)
	s.sendPlayerUpdate()
	s.debug("item destroyed", "account", s.accountName, "item", itemGUID, "bag", bag, "slot", slot, "count", count)
	return true
}

func (s *session) syncEquipmentCache(ctx context.Context) {
	if !s.playerLoaded || s.player == nil || s.server.CharactersStore.DB == nil {
		return
	}
	db := s.server.CharactersStore.DB
	slots := make([]uint32, equipSlotEnd)
	rows, err := db.QueryContext(ctx, `SELECT ci.slot, ii.itemEntry FROM character_inventory AS ci
		JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.bag = 0 AND ci.slot < ?`, s.playerGUID, equipSlotEnd)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var slot, entry int64
			if err := rows.Scan(&slot, &entry); err == nil && slot < int64(len(slots)) {
				slots[slot] = uint32(entry)
			}
		}
	}
	parts := make([]string, equipSlotEnd*2)
	for i := 0; i < int(equipSlotEnd); i++ {
		parts[i*2] = strconv.FormatUint(uint64(slots[i]), 10)
		parts[i*2+1] = "0"
	}
	cacheStr := strings.Join(parts, " ")
	s.player.Equipment = cacheStr
	_, _ = db.ExecContext(ctx, "UPDATE characters SET equipmentCache = ? WHERE guid = ?", cacheStr, s.playerGUID)
}

func inventoryTypeToSlot(invType uint8) uint8 {
	switch invType {
	case 1: // Head
		return equipSlotHead
	case 2: // Neck
		return equipSlotNeck
	case 3: // Shoulders
		return equipSlotShoulder
	case 4: // Body/Shirt
		return equipSlotBody
	case 5, 20: // Chest / Robe
		return equipSlotChest
	case 6: // Waist
		return equipSlotWaist
	case 7: // Legs
		return equipSlotLegs
	case 8: // Feet
		return equipSlotFeet
	case 9: // Wrists
		return equipSlotWrists
	case 10: // Hands
		return equipSlotHands
	case 11: // Finger
		return equipSlotFinger1
	case 12: // Trinket
		return equipSlotTrinket1
	case 13, 17, 21: // Weapon, 2HWeapon, Mainhand
		return equipSlotMainhand
	case 14, 22, 23: // Shield, Offhand, Holdable
		return equipSlotOffhand
	case 15, 25, 26: // Ranged, Thrown, RangedRight
		return equipSlotRanged
	case 16: // Back/Cloak
		return equipSlotBack
	case 18: // Bag
		return invSlotBagStart
	case 19: // Tabard
		return equipSlotTabard
	default:
		return 255
	}
}
