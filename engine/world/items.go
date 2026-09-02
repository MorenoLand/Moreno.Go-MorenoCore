package world

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// handleItemNameQuery processes CMSG_ITEM_NAME_QUERY (0x2C4).
// Reference: WorldSession::HandleItemNameQueryOpcode (ItemHandler.cpp:812).
func (s *session) handleItemNameQuery(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	itemID, err := r.ReadU32()
	if err != nil {
		return false
	}
	_, _ = r.ReadU64() // skip guid

	var name string
	var invType uint32
	if s.server != nil && s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT name, InventoryType FROM item_template WHERE entry = ? LIMIT 1", itemID).Scan(&name, &invType)
	}
	if name == "" {
		return true
	}

	buf := protocol.NewBuffer(len(name) + 16)
	buf.WriteU32(itemID)
	buf.WriteCString(name)
	buf.WriteU32(invType)
	return s.write(uint16(protocol.OpcodeSMSG_ITEM_NAME_QUERY_RESPONSE), buf.Bytes(), true) == nil
}

// handleItemTextQuery processes CMSG_ITEM_TEXT_QUERY (0x243).
// Reference: WorldSession::HandleItemTextQuery (ItemHandler.cpp:1211).
func (s *session) handleItemTextQuery(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	itemGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	var text string
	var found bool
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT text FROM item_text WHERE id = (SELECT itemTextId FROM item_instance WHERE guid = ?) LIMIT 1", itemGUID).Scan(&text)
		if err == nil {
			found = true
		} else {
			var count int
			_ = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM item_instance WHERE guid = ?", itemGUID).Scan(&count)
			if count > 0 {
				found = true
				text = ""
			}
		}
	}

	buf := protocol.NewBuffer(len(text) + 16)
	if found {
		buf.WriteU8(0) // has text
		buf.WriteU64(itemGUID)
		buf.WriteCString(text)
	} else {
		buf.WriteU8(1) // no text
	}
	return s.write(uint16(protocol.OpcodeSMSG_ITEM_TEXT_QUERY_RESPONSE), buf.Bytes(), true) == nil
}

// handleItemRefundInfo processes CMSG_ITEM_REFUND_INFO (0x4B3).
// Reference: WorldSession::HandleItemRefundInfoRequest (ItemHandler.cpp:1169) and Player::SendRefundInfo (Player.cpp:26503).
func (s *session) handleItemRefundInfo(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	itemGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	if s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}

	var itemEntry int64
	err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT itemEntry FROM item_instance WHERE guid = ? LIMIT 1", itemGUID).Scan(&itemEntry)
	if err != nil || itemEntry == 0 {
		return true
	}

	var buyPrice uint32
	if s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT BuyPrice FROM item_template WHERE entry = ? LIMIT 1", itemEntry).Scan(&buyPrice)
	}

	buf := protocol.NewBuffer(64)
	buf.WriteU64(itemGUID)
	buf.WriteU32(buyPrice) // money cost
	buf.WriteU32(0)        // honor points
	buf.WriteU32(0)        // arena points
	for i := 0; i < 5; i++ {
		buf.WriteU32(0) // item requirement id
		buf.WriteU32(0) // item requirement count
	}
	buf.WriteU32(0)    // unk
	buf.WriteU32(7200) // remaining seconds (2 hours)
	return s.write(uint16(protocol.OpcodeSMSG_ITEM_REFUND_INFO_RESPONSE), buf.Bytes(), true) == nil
}

// handleItemRefund processes CMSG_ITEM_REFUND (0x4B4).
// Reference: WorldSession::HandleItemRefund (ItemHandler.cpp:1186) and Player::RefundItem (Player.cpp:26574).
func (s *session) handleItemRefund(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	itemGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil || s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}

	var itemEntry int64
	err = s.server.CharactersStore.DB.QueryRowContext(ctx, `SELECT ii.itemEntry FROM item_instance AS ii
		JOIN character_inventory AS ci ON ci.item = ii.guid
		WHERE ii.guid = ? AND ci.guid = ? LIMIT 1`, itemGUID, s.playerGUID).Scan(&itemEntry)

	buf := protocol.NewBuffer(16)
	buf.WriteU64(itemGUID)
	if err != nil || itemEntry == 0 {
		buf.WriteU32(10) // error (expired or not refundable)
		return s.write(uint16(protocol.OpcodeSMSG_ITEM_REFUND_RESULT), buf.Bytes(), true) == nil
	}

	var buyPrice uint32
	if s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT BuyPrice FROM item_template WHERE entry = ? LIMIT 1", itemEntry).Scan(&buyPrice)
	}

	_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_inventory WHERE item = ? AND guid = ?", itemGUID, s.playerGUID)
	_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM item_instance WHERE guid = ?", itemGUID)

	if buyPrice > 0 {
		s.player.Money += buyPrice
		s.sendPlayerUpdate()
	}

	buf.WriteU32(0) // success
	return s.write(uint16(protocol.OpcodeSMSG_ITEM_REFUND_RESULT), buf.Bytes(), true) == nil
}

const (
	equipErrOk           = 0
	equipErrItemNotFound = 5
	equipErrYouAreDead   = 17
	equipErrNotInCombat  = 20
)

func (s *session) sendEquipError(errCode uint8, itemGUID uint64) {
	buf := protocol.NewBuffer(18)
	buf.WriteU8(errCode)
	if errCode != equipErrOk {
		buf.WriteU64(itemGUID)
		buf.WriteU64(0)
		buf.WriteU8(0)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_INVENTORY_CHANGE_FAILURE), buf.Bytes(), true)
}

// handleUseItem processes CMSG_USE_ITEM (0x0AB).
// Reference: WorldSession::HandleUseItemOpcode (SpellHandler.cpp:73).
func (s *session) handleUseItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	// Player cannot use items while dead (SpellHandler.cpp:194)
	if s.player.Health == 0 {
		s.sendEquipError(equipErrYouAreDead, 0)
		return true
	}
	r := protocol.NewReader(payload)
	bagIndex, err := r.ReadU8()
	if err != nil {
		return false
	}
	slot, err := r.ReadU8()
	if err != nil {
		return false
	}
	castCount, err := r.ReadU8()
	if err != nil {
		return false
	}
	spellID, err := r.ReadU32()
	if err != nil {
		return false
	}
	itemGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	_, err = r.ReadU32() // glyphIndex
	if err != nil {
		return false
	}
	_, err = r.ReadU8() // castFlags
	if err != nil {
		return false
	}

	target, _ := protocol.ReadSpellTargetData(r)

	// Validate item existence in specified bag and slot
	bagKey, ok := s.inventoryBagKey(ctx, bagIndex)
	if !ok {
		s.sendEquipError(equipErrItemNotFound, itemGUID)
		return true
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return false
	}

	var dbItemGUID int64
	var itemEntry int64
	var count int64
	err = cdb.QueryRowContext(ctx, `SELECT ci.item, ii.itemEntry, ii.count
		FROM character_inventory AS ci
		JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.bag = ? AND ci.slot = ? LIMIT 1`, s.playerGUID, bagKey, slot).Scan(&dbItemGUID, &itemEntry, &count)
	if err != nil || count <= 0 {
		s.sendEquipError(equipErrItemNotFound, itemGUID)
		return true
	}
	rawItemGUID := uint64(dbItemGUID)
	fullItemGUID := rawItemGUID | (uint64(0x4000) << 48)
	if itemGUID != 0 && itemGUID != rawItemGUID && itemGUID != fullItemGUID {
		s.sendEquipError(equipErrItemNotFound, itemGUID)
		return true
	}

	// Check player spell cooldown
	nowUnix := time.Now().Unix()
	for _, cd := range s.player.Cooldowns {
		if cd.Spell == spellID && cd.End > nowUnix {
			return true
		}
	}

	// Cast spell
	if spellID != 0 && s.server != nil && s.server.Data != nil {
		if spell, found, err := s.server.Data.Spell(spellID); err == nil && found {
			s.finishSpellCast(ctx, castCount, spellID, spell, target)
		}
	}

	// If consumable item (class == 0), decrement count or remove from inventory
	var class int64
	if s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT class FROM item_template WHERE entry = ?", itemEntry).Scan(&class)
		if class == 0 { // ITEM_CLASS_CONSUMABLE
			if count > 1 {
				_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET count = count - 1 WHERE guid = ?", dbItemGUID)
			} else {
				_, _ = cdb.ExecContext(ctx, "DELETE FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ?", s.playerGUID, bagKey, slot)
				_, _ = cdb.ExecContext(ctx, "DELETE FROM item_instance WHERE guid = ?", dbItemGUID)
			}
			_ = s.sendInventoryItems(ctx)
			s.sendPlayerUpdate()
		}
	}

	return true
}

func (s *session) inventoryBagKey(ctx context.Context, bag uint8) (int64, bool) {
	if bag == 0 || bag == invSlotBag0 {
		return 0, true
	}
	if bag < invSlotBagStart || bag >= invSlotBagEnd || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return 0, false
	}
	var itemGUID int64
	err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = 0 AND slot = ? LIMIT 1", s.playerGUID, bag).Scan(&itemGUID)
	return itemGUID, err == nil && itemGUID != 0
}

func (s *session) inventoryItemAt(ctx context.Context, bag, slot uint8) (int64, int64, int64, error) {
	bagKey, ok := s.inventoryBagKey(ctx, bag)
	if !ok {
		return 0, 0, 0, sql.ErrNoRows
	}
	var itemGUID, itemEntry, count int64
	err := s.server.CharactersStore.DB.QueryRowContext(ctx, `SELECT ci.item, ii.itemEntry, ii.count
		FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.bag = ? AND ci.slot = ? LIMIT 1`, s.playerGUID, bagKey, slot).Scan(&itemGUID, &itemEntry, &count)
	return itemGUID, itemEntry, count, err
}

func (s *session) handleSplitItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	srcBag, err := reader.ReadU8()
	if err != nil {
		return false
	}
	srcSlot, err := reader.ReadU8()
	if err != nil {
		return false
	}
	dstBag, err := reader.ReadU8()
	if err != nil {
		return false
	}
	dstSlot, err := reader.ReadU8()
	if err != nil {
		return false
	}
	count, err := reader.ReadU32()
	if err != nil || count == 0 || srcBag == dstBag && srcSlot == dstSlot {
		return true
	}
	srcGUID, srcEntry, srcCount, err := s.inventoryItemAt(ctx, srcBag, srcSlot)
	if err != nil || srcGUID == 0 || count >= uint32(srcCount) {
		return true
	}
	dstKey, ok := s.inventoryBagKey(ctx, dstBag)
	if !ok {
		return true
	}
	db := s.server.CharactersStore.DB
	var dstGUID, dstEntry, dstCount int64
	if err := db.QueryRowContext(ctx, `SELECT ci.item, ii.itemEntry, ii.count
		FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? AND ci.bag = ? AND ci.slot = ? LIMIT 1`, s.playerGUID, dstKey, dstSlot).Scan(&dstGUID, &dstEntry, &dstCount); err != nil && err != sql.ErrNoRows {
		return true
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return true
	}
	if dstGUID != 0 {
		if dstEntry != srcEntry {
			_ = tx.Rollback()
			return true
		}
		if _, err = tx.ExecContext(ctx, "UPDATE item_instance SET count = count + ? WHERE guid = ?", count, dstGUID); err != nil {
			_ = tx.Rollback()
			return true
		}
	} else {
		var newGUID int64
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&newGUID); err != nil || newGUID <= 0 {
			_ = tx.Rollback()
			return true
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, count) VALUES (?, ?, ?, ?)", newGUID, srcEntry, s.playerGUID, count); err != nil {
			_ = tx.Rollback()
			return true
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, ?, ?, ?)", s.playerGUID, dstKey, dstSlot, newGUID); err != nil {
			_ = tx.Rollback()
			return true
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE item_instance SET count = count - ? WHERE guid = ?", count, srcGUID); err != nil {
		_ = tx.Rollback()
		return true
	}
	if err = tx.Commit(); err != nil {
		return true
	}
	_ = s.sendInventoryItems(ctx)
	s.debug("item stack split", "account", s.accountName, "source_bag", srcBag, "source_slot", srcSlot, "destination_bag", dstBag, "destination_slot", dstSlot, "count", count)
	return true
}

func (s *session) handleAutoStoreBagItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 3 || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}
	srcBag, srcSlot, dstBag := payload[0], payload[1], payload[2]
	itemGUID, _, _, err := s.inventoryItemAt(ctx, srcBag, srcSlot)
	if err != nil || itemGUID == 0 {
		return true
	}
	dstKey, ok := s.inventoryBagKey(ctx, dstBag)
	if !ok {
		return true
	}
	slot, ok := s.freeInventorySlot(ctx, dstKey)
	if !ok {
		return true
	}
	if _, err := s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_inventory SET bag = ?, slot = ? WHERE guid = ? AND item = ?", dstKey, slot, s.playerGUID, itemGUID); err != nil {
		return true
	}
	_ = s.sendInventoryItems(ctx)
	s.debug("item moved into bag", "account", s.accountName, "item", itemGUID, "bag", dstBag, "slot", slot)
	return true
}

func (s *session) freeInventorySlot(ctx context.Context, bagKey int64) (uint8, bool) {
	first, last := int64(invSlotItemStart), int64(invSlotItemEnd-1)
	if bagKey != 0 {
		first, last = 0, 35
		if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
			return 0, false
		}
		var slots int64
		if err := s.server.WorldStore.DB.QueryRowContext(ctx, `SELECT COALESCE(ContainerSlots, 0) FROM item_template WHERE entry = (SELECT itemEntry FROM item_instance WHERE guid = ?)`, bagKey).Scan(&slots); err != nil || slots <= 0 {
			return 0, false
		}
		if slots-1 < last {
			last = slots - 1
		}
	}
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT slot FROM character_inventory WHERE guid = ? AND bag = ?", s.playerGUID, bagKey)
	if err != nil {
		return 0, false
	}
	defer rows.Close()
	used := make(map[int64]struct{})
	for rows.Next() {
		var slot int64
		if rows.Scan(&slot) == nil {
			used[slot] = struct{}{}
		}
	}
	for slot := first; slot <= last; slot++ {
		if _, exists := used[slot]; !exists {
			return uint8(slot), true
		}
	}
	return 0, false
}

// handleOpenItem processes CMSG_OPEN_ITEM (0x0AC).
// Reference: WorldSession::HandleOpenItemOpcode (SpellHandler.cpp:183).
func (s *session) handleOpenItem(ctx context.Context, payload []byte) bool {
	return true
}

// handleReadItem processes CMSG_READ_ITEM (0x0AD).
// Reference: WorldSession::HandleReadItem (ItemHandler.cpp:340).
func (s *session) handleReadItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	bag := payload[0]
	slot := payload[1]
	itemGUID, _, _, err := s.inventoryItemAt(ctx, bag, slot)
	if err != nil || itemGUID == 0 {
		return true
	}
	buf := protocol.NewBuffer(8)
	buf.WriteU64(uint64(itemGUID))
	_ = s.write(uint16(protocol.OpcodeSMSG_READ_ITEM_OK), buf.Bytes(), true)
	return true
}

// handlePageTextQuery processes CMSG_PAGE_TEXT_QUERY (0x05A).
// Reference: WorldSession::HandleQueryPageText (QueryHandler.cpp:277).
func (s *session) handlePageTextQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	pageID, err := r.ReadU32()
	if err != nil {
		return false
	}

	var text string
	var nextPageID uint32
	if s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT Text, NextPageID FROM page_text WHERE ID = ? LIMIT 1", pageID).Scan(&text, &nextPageID)
	}

	buf := protocol.NewBuffer(32 + len(text))
	buf.WriteU32(pageID)
	buf.WriteCString(text)
	buf.WriteU32(nextPageID)
	_ = s.write(uint16(protocol.OpcodeSMSG_PAGE_TEXT_QUERY_RESPONSE), buf.Bytes(), true)
	return true
}

// handleWrapItem processes CMSG_WRAP_ITEM (0x1D3).
// Reference: WorldSession::HandleWrapItemOpcode (ItemHandler.cpp:802).
func (s *session) handleWrapItem(ctx context.Context, payload []byte) bool {
	return true
}

// handleRepairItem processes CMSG_REPAIR_ITEM (0x1F8 / 0x2A8).
// Reference: WorldSession::HandleRepairItemOpcode (NPCHandler.cpp:717).
func (s *session) handleRepairItem(ctx context.Context, payload []byte) bool {
	return true
}

// handleSocketGems processes CMSG_SOCKET_GEMS (0x464).
// Reference: WorldSession::HandleSocketOpcode (ItemHandler.cpp:920).
func (s *session) handleSocketGems(ctx context.Context, payload []byte) bool {
	return true
}

// handleSetAmmo processes CMSG_SET_AMMO (0x268).
// Reference: WorldSession::HandleSetAmmoOpcode (ItemHandler.cpp:772).
func (s *session) handleSetAmmo(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	itemEntry, _ := r.ReadU32()
	s.player.AmmoID = itemEntry
	s.sendPlayerUpdate()
	return true
}

const (
	maxEquipmentSetIndex uint32 = 10
	equipmentSlotEnd     uint32 = 19
)

// handleEquipmentSetSave processes CMSG_EQUIPMENT_SET_SAVE (0x4BD).
// Reference: WorldSession::HandleEquipmentSetSave (CharacterHandler.cpp:1492).
func (s *session) handleEquipmentSetSave(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	setGuid, err := r.ReadPackedGUID()
	if err != nil {
		return false
	}
	index, err := r.ReadU32()
	if err != nil || index >= maxEquipmentSetIndex {
		return false
	}
	name, err := r.ReadCString()
	if err != nil {
		return false
	}
	iconName, err := r.ReadCString()
	if err != nil {
		return false
	}

	var items [19]uint64
	var ignoreMask uint32
	for i := uint32(0); i < equipmentSlotEnd; i++ {
		itemGuid, err := r.ReadPackedGUID()
		if err != nil {
			break
		}
		if itemGuid == 1 {
			ignoreMask |= 1 << i
			continue
		}
		items[i] = itemGuid
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		placeholders := strings.Repeat("?, ", 24) + "?"
		cols := "guid, setguid, setindex, name, iconname, ignore_mask"
		for i := 0; i < 19; i++ {
			cols += fmt.Sprintf(", item%d", i)
		}
		args := []any{s.playerGUID, setGuid, index, name, iconName, ignoreMask}
		for i := 0; i < 19; i++ {
			args = append(args, items[i])
		}
		query := fmt.Sprintf("REPLACE INTO character_equipmentsets (%s) VALUES (%s)", cols, placeholders)
		_, _ = cdb.ExecContext(ctx, query, args...)
	}
	return true
}

// handleEquipmentSetDelete processes CMSG_DELETEEQUIPMENT_SET (0x13E).
// Reference: WorldSession::HandleEquipmentSetDelete (CharacterHandler.cpp:1544).
func (s *session) handleEquipmentSetDelete(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	setGuid, err := r.ReadPackedGUID()
	if err != nil {
		return false
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_equipmentsets WHERE setguid = ? AND guid = ?", setGuid, s.playerGUID)
	}
	return true
}

// handleEquipmentSetUse processes CMSG_EQUIPMENT_SET_USE (0x4D5).
// Reference: WorldSession::HandleEquipmentSetUse (CharacterHandler.cpp:1554).
func (s *session) handleEquipmentSetUse(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)

	for i := uint32(0); i < equipmentSlotEnd; i++ {
		itemGuid, err := r.ReadPackedGUID()
		if err != nil {
			break
		}
		srcbag, err := r.ReadU8()
		if err != nil {
			break
		}
		srcslot, err := r.ReadU8()
		if err != nil {
			break
		}
		if itemGuid == 1 || itemGuid == 0 {
			continue
		}
		// If item is in bags, swap to equipment slot
		if srcbag != 0 || srcslot >= uint8(equipmentSlotEnd) {
			_ = s.handleSwapInvItem(ctx, []byte{srcbag, srcslot, 0, uint8(i)})
		}
	}

	// Send SMSG_EQUIPMENT_SET_USE_RESULT (0x4D6) with 0 = success
	buf := protocol.NewBuffer(1)
	buf.WriteU8(0) // 0 = ERR_EQUIPMENT_SET_USE_SUCCESS
	_ = s.write(uint16(protocol.OpcodeSMSG_EQUIPMENT_SET_USE_RESULT), buf.Bytes(), true)
	return true
}

