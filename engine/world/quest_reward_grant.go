package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	questRewardChoiceLimit = 6
	inventoryRewardBag     = 0
	inventoryRewardFirst   = 23
	inventoryRewardLast    = 38
	questInventoryFull     = 4
)

var errQuestInventoryFull = errors.New("quest reward inventory is full")

type inventoryRewardRecord struct {
	Bag      uint8
	Slot     uint8
	ItemGUID uint32
	Entry    uint32
	Count    uint32
}

type questItemGrant struct {
	Entry          uint32
	Count          uint32
	Bag            uint8
	Slot           uint32
	InventoryCount uint32
	Stacked        bool
}

func (s *session) handleQuestgiverChooseReward(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	giverGUID, err := reader.ReadU64()
	if err != nil {
		return true
	}
	questID, err := reader.ReadU32()
	if err != nil {
		return true
	}
	reward, err := reader.ReadU32()
	if err != nil {
		return true
	}
	if reward >= questRewardChoiceLimit {
		s.debug("quest reward rejected", "account", s.accountName, "quest", questID, "reward", reward, "reason", "invalid choice")
		return true
	}
	if _, ok := s.questGiverEnder(ctx, giverGUID, questID); !ok {
		return true
	}
	view, err := s.loadQuestRewardView(ctx, questID)
	if err != nil {
		s.debug("quest reward load failed", "account", s.accountName, "quest", questID, "error", err)
		return true
	}
	status, err := s.characterQuestStatus(ctx, questID)
	if err != nil {
		return false
	}
	if status != questStatusComplete && view.Detail.Flags&questAutoCompleteFlags == 0 {
		s.debug("quest reward rejected", "account", s.accountName, "quest", questID, "reason", "quest incomplete", "status", status)
		return true
	}
	if len(view.Detail.ChoiceItems) > 0 && (reward >= uint32(len(view.Detail.ChoiceItems)) || view.Detail.ChoiceItems[reward].ID == 0) {
		s.debug("quest reward rejected", "account", s.accountName, "quest", questID, "reward", reward, "reason", "choice unavailable")
		return true
	}
	if complete, itemErr := s.hasQuestRequiredItems(ctx, view.RequiredItems); itemErr != nil {
		return false
	} else if !complete {
		return true
	}
	grants, err := s.commitQuestReward(ctx, view, reward)
	if errors.Is(err, errQuestInventoryFull) {
		_ = s.write(uint16(protocol.OpcodeSMSG_QUESTGIVER_QUEST_FAILED), buildQuestFailed(view.Detail.ID, questInventoryFull), true)
		return true
	}
	if err != nil {
		s.debug("quest reward commit failed", "account", s.accountName, "quest", questID, "error", err)
		return false
	}
	for _, grant := range grants {
		if err := s.write(uint16(protocol.OpcodeSMSG_ITEM_PUSH_RESULT), buildItemPushResult(s.playerGUID, grant.Bag, grant.Slot, grant.Entry, grant.Count, grant.InventoryCount, grant.Stacked), true); err != nil {
			return false
		}
	}
	s.player.Money += view.Detail.RewardMoney
	if err := s.write(uint16(protocol.OpcodeSMSG_QUESTGIVER_QUEST_COMPLETE), buildQuestRewardComplete(view.Detail.ID, 0, view.Detail.RewardMoney, 0, view.Detail.RewardTalents, uint32(view.Detail.RewardArenaPoints)), true); err != nil {
		return false
	}
	s.gossip = nil
	s.gossipClosed = true
	s.debug("quest rewarded", "account", s.accountName, "quest", questID, "reward", reward, "items", len(grants), "money", view.Detail.RewardMoney)
	return true
}

func (s *session) commitQuestReward(ctx context.Context, view questRewardView, choice uint32) ([]questItemGrant, error) {
	s.server.inventoryMu.Lock()
	defer s.server.inventoryMu.Unlock()
	tx, err := s.server.CharactersStore.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := func(cause error) ([]questItemGrant, error) {
		_ = tx.Rollback()
		return nil, cause
	}
	inventory, err := loadInventoryRewardRecords(ctx, tx, s.playerGUID)
	if err != nil {
		return rollback(err)
	}
	if err := consumeQuestItems(ctx, tx, inventory, view.RequiredItems); err != nil {
		return rollback(err)
	}
	fixed := append([]questRewardItem(nil), view.Detail.RewardItems...)
	if len(view.Detail.ChoiceItems) > 0 && choice < uint32(len(view.Detail.ChoiceItems)) {
		fixed = append(fixed, view.Detail.ChoiceItems[choice])
	}
	grants, err := s.storeQuestRewardItems(ctx, tx, inventory, fixed)
	if err != nil {
		return rollback(err)
	}
	if view.Detail.RewardMoney != 0 {
		if _, err := tx.ExecContext(ctx, "UPDATE characters SET money = money + ? WHERE guid = ? AND account = ?", view.Detail.RewardMoney, s.playerGUID, s.accountID); err != nil {
			return rollback(err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM character_queststatus WHERE guid = ? AND quest = ?", s.playerGUID, view.Detail.ID); err != nil {
		return rollback(err)
	}
	rewardedInsert := "INSERT OR IGNORE INTO character_queststatus_rewarded (guid, quest, active) VALUES (?, ?, 1)"
	if s.server.CharactersStore.Backend != database.BackendSQLite {
		rewardedInsert = "INSERT IGNORE INTO character_queststatus_rewarded (guid, quest, active) VALUES (?, ?, 1)"
	}
	if _, err := tx.ExecContext(ctx, rewardedInsert, s.playerGUID, view.Detail.ID); err != nil {
		return rollback(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return grants, nil
}

func loadInventoryRewardRecords(ctx context.Context, tx *sql.Tx, guid uint64) ([]inventoryRewardRecord, error) {
	rows, err := tx.QueryContext(ctx, "SELECT ci.bag, ci.slot, ci.item, ii.itemEntry, ii.count FROM character_inventory AS ci JOIN item_instance AS ii ON ii.guid = ci.item WHERE ci.guid = ? ORDER BY ci.bag, ci.slot", guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]inventoryRewardRecord, 0)
	for rows.Next() {
		var bag, slot, itemGUID, entry, count int64
		if err := rows.Scan(&bag, &slot, &itemGUID, &entry, &count); err != nil {
			return nil, err
		}
		if bag < 0 || bag > 255 || slot < 0 || slot > 255 || itemGUID < 0 || itemGUID > int64(^uint32(0)) || entry < 0 || entry > int64(^uint32(0)) || count <= 0 {
			continue
		}
		if count > int64(^uint32(0)) {
			count = int64(^uint32(0))
		}
		result = append(result, inventoryRewardRecord{Bag: uint8(bag), Slot: uint8(slot), ItemGUID: uint32(itemGUID), Entry: uint32(entry), Count: uint32(count)})
	}
	return result, rows.Err()
}

func consumeQuestItems(ctx context.Context, tx *sql.Tx, inventory []inventoryRewardRecord, required []questRewardItem) error {
	needed := make(map[uint32]uint64)
	for _, item := range required {
		if item.ID != 0 && item.Quantity != 0 {
			needed[item.ID] += uint64(item.Quantity)
		}
	}
	ids := make([]uint32, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, entry := range ids {
		remaining := needed[entry]
		for index := range inventory {
			record := &inventory[index]
			if record.Entry != entry || record.Count == 0 || remaining == 0 {
				continue
			}
			remove := uint64(record.Count)
			if remove > remaining {
				remove = remaining
			}
			record.Count -= uint32(remove)
			remaining -= remove
			if record.Count == 0 {
				if _, err := tx.ExecContext(ctx, "DELETE FROM character_inventory WHERE item = ?", record.ItemGUID); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, "DELETE FROM item_instance WHERE guid = ?", record.ItemGUID); err != nil {
					return err
				}
			} else if _, err := tx.ExecContext(ctx, "UPDATE item_instance SET count = ? WHERE guid = ?", record.Count, record.ItemGUID); err != nil {
				return err
			}
		}
		if remaining != 0 {
			return fmt.Errorf("required quest item %d is unavailable", entry)
		}
	}
	return nil
}

func (s *session) storeQuestRewardItems(ctx context.Context, tx *sql.Tx, inventory []inventoryRewardRecord, items []questRewardItem) ([]questItemGrant, error) {
	grants := make([]questItemGrant, 0, len(items))
	var nextGUID uint32
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) FROM item_instance").Scan(&nextGUID); err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == 0 || item.Quantity == 0 {
			continue
		}
		var stackable int64
		if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT stackable FROM item_template WHERE entry = ?", item.ID).Scan(&stackable); err != nil {
			return nil, err
		}
		if stackable < 1 {
			stackable = 1
		}
		remaining := uint64(item.Quantity)
		for index := range inventory {
			record := &inventory[index]
			if record.Entry != item.ID || record.Count >= uint32(stackable) || remaining == 0 {
				continue
			}
			space := uint64(uint32(stackable) - record.Count)
			add := remaining
			if add > space {
				add = space
			}
			record.Count += uint32(add)
			remaining -= add
			if _, err := tx.ExecContext(ctx, "UPDATE item_instance SET count = ? WHERE guid = ?", record.Count, record.ItemGUID); err != nil {
				return nil, err
			}
			grants = append(grants, questItemGrant{Entry: item.ID, Count: uint32(add), InventoryCount: record.Count, Stacked: true})
		}
		for remaining != 0 {
			if len(inventory) >= inventoryRewardLast-inventoryRewardFirst+1 {
				occupied := make(map[uint8]bool)
				for _, record := range inventory {
					if record.Bag == inventoryRewardBag && record.Slot >= inventoryRewardFirst && record.Slot <= inventoryRewardLast && record.Count != 0 {
						occupied[record.Slot] = true
					}
				}
				if len(occupied) >= inventoryRewardLast-inventoryRewardFirst+1 {
					return nil, errQuestInventoryFull
				}
			}
			slot, ok := nextRewardSlot(inventory)
			if !ok {
				return nil, errQuestInventoryFull
			}
			add := remaining
			if add > uint64(stackable) {
				add = uint64(stackable)
			}
			nextGUID++
			if nextGUID == 0 {
				return nil, errors.New("item guid space exhausted")
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, giftCreatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text) VALUES (?, ?, ?, 0, 0, ?, 0, '', 0, '', 0, 0, 0, NULL)", nextGUID, item.ID, s.playerGUID, add); err != nil {
				return nil, err
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, ?, ?, ?)", s.playerGUID, inventoryRewardBag, slot, nextGUID); err != nil {
				return nil, err
			}
			inventory = append(inventory, inventoryRewardRecord{Bag: inventoryRewardBag, Slot: slot, ItemGUID: nextGUID, Entry: item.ID, Count: uint32(add)})
			remaining -= add
			grants = append(grants, questItemGrant{Entry: item.ID, Count: uint32(add), Bag: inventoryRewardBag, Slot: uint32(slot), InventoryCount: uint32(add)})
		}
	}
	return grants, nil
}

func nextRewardSlot(inventory []inventoryRewardRecord) (uint8, bool) {
	occupied := make(map[uint8]struct{})
	for _, record := range inventory {
		if record.Bag == inventoryRewardBag && record.Slot >= inventoryRewardFirst && record.Slot <= inventoryRewardLast && record.Count != 0 {
			occupied[record.Slot] = struct{}{}
		}
	}
	for slot := inventoryRewardFirst; slot <= inventoryRewardLast; slot++ {
		if _, ok := occupied[uint8(slot)]; !ok {
			return uint8(slot), true
		}
	}
	return 0, false
}

func buildItemPushResult(playerGUID uint64, bag uint8, slot, entry, count, inventoryCount uint32, stacked bool) []byte {
	packet := protocol.NewBuffer(48)
	packet.WriteU64(playerGUID)
	packet.WriteU32(1)
	packet.WriteU32(0)
	packet.WriteU32(0)
	packet.WriteU8(bag)
	if stacked {
		packet.WriteU32(^uint32(0))
	} else {
		packet.WriteU32(slot)
	}
	packet.WriteU32(entry)
	packet.WriteU32(0)
	packet.WriteI32(0)
	packet.WriteU32(count)
	packet.WriteU32(inventoryCount)
	return packet.Bytes()
}

func buildQuestRewardComplete(questID, xp, money, honor, talents, arena uint32) []byte {
	packet := protocol.NewBuffer(24)
	packet.WriteU32(questID)
	packet.WriteU32(xp)
	packet.WriteU32(money)
	packet.WriteU32(honor)
	packet.WriteU32(talents)
	packet.WriteU32(arena)
	return packet.Bytes()
}

func buildQuestFailed(questID, reason uint32) []byte {
	packet := protocol.NewBuffer(8)
	packet.WriteU32(questID)
	packet.WriteU32(reason)
	return packet.Bytes()
}
