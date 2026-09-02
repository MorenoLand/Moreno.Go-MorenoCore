package world

import (
	"context"
	"database/sql"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

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
