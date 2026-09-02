package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	bankSlotPrice1 uint32 = 1000    // 10s
	bankSlotPrice2 uint32 = 10000   // 1g
	bankSlotPrice3 uint32 = 100000  // 10g
	bankSlotPrice4 uint32 = 250000  // 25g
	bankSlotPrice5 uint32 = 500000  // 50g
	bankSlotPrice6 uint32 = 1000000 // 100g
	bankSlotPrice7 uint32 = 2500000 // 250g

	bankSlotStart uint8 = 39
	bankSlotEnd   uint8 = 66 // 28 bank item slots (39..66)
	bagSlotStart  uint8 = 23
	bagSlotEnd    uint8 = 38 // 16 backpack slots (23..38)
)

var bankBagSlotPrices = []uint32{
	bankSlotPrice1,
	bankSlotPrice2,
	bankSlotPrice3,
	bankSlotPrice4,
	bankSlotPrice5,
	bankSlotPrice6,
	bankSlotPrice7,
}

// handleBankerActivate processes CMSG_BANKER_ACTIVATE (0x1B7).
// Reference: WorldSession::HandleBankerActivateOpcode (BankHandler.cpp:46).
func (s *session) handleBankerActivate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	res := protocol.NewBuffer(8)
	res.WriteU64(bankerGUID)
	return s.write(uint16(protocol.OpcodeSMSG_SHOW_BANK), res.Bytes(), true) == nil
}

// handleBuyBankSlot processes CMSG_BUY_BANK_SLOT (0x1B9).
// Reference: WorldSession::HandleBuyBankSlotOpcode (BankHandler.cpp:138).
func (s *session) handleBuyBankSlot(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return false
	}

	// Count purchased bank bag slots
	var purchasedCount int
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(1) FROM character_inventory WHERE guid = ? AND bag = 0 AND slot >= 67 AND slot <= 73", s.playerGUID).Scan(&purchasedCount)

	res := protocol.NewBuffer(12)
	res.WriteU64(bankerGUID)
	if purchasedCount >= len(bankBagSlotPrices) {
		res.WriteU32(1) // ERR_BANKSLOT_FAILED_TOO_MANY
		_ = s.write(uint16(protocol.OpcodeSMSG_BUY_BANK_SLOT_RESULT), res.Bytes(), true)
		return true
	}

	cost := bankBagSlotPrices[purchasedCount]
	if s.player.Money < cost {
		res.WriteU32(2) // ERR_BANKSLOT_INSUFFICIENT_FUNDS
		_ = s.write(uint16(protocol.OpcodeSMSG_BUY_BANK_SLOT_RESULT), res.Bytes(), true)
		return true
	}

	s.player.Money -= cost
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	nextSlot := 67 + purchasedCount
	_, _ = cdb.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, 0)", s.playerGUID, nextSlot)

	res.WriteU32(0) // ERR_BANKSLOT_OK
	_ = s.write(uint16(protocol.OpcodeSMSG_BUY_BANK_SLOT_RESULT), res.Bytes(), true)
	s.sendPlayerUpdate()
	return true
}

// handleAutoBankItem processes CMSG_AUTOBANK_ITEM (0x283).
// Reference: WorldSession::HandleAutoBankItemOpcode (BankHandler.cpp:62).
func (s *session) handleAutoBankItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	srcBag, err := r.ReadU8()
	if err != nil {
		return false
	}
	srcSlot, err := r.ReadU8()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return false
	}

	var itemGUID uint64
	err = cdb.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ? AND item != 0 LIMIT 1", s.playerGUID, srcBag, srcSlot).Scan(&itemGUID)
	if err != nil || itemGUID == 0 {
		return true
	}

	// Find free bank slot among 39..66
	occupied := make(map[uint8]bool)
	rows, err := cdb.QueryContext(ctx, "SELECT slot FROM character_inventory WHERE guid = ? AND bag = 0 AND slot >= ? AND slot <= ?", s.playerGUID, bankSlotStart, bankSlotEnd)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sl uint8
			if rows.Scan(&sl) == nil {
				occupied[sl] = true
			}
		}
	}

	var destSlot uint8 = 0xFF
	for sl := bankSlotStart; sl <= bankSlotEnd; sl++ {
		if !occupied[sl] {
			destSlot = sl
			break
		}
	}
	if destSlot == 0xFF {
		return true // Bank full
	}

	_, _ = cdb.ExecContext(ctx, "UPDATE character_inventory SET bag = 0, slot = ? WHERE guid = ? AND bag = ? AND slot = ?", destSlot, s.playerGUID, srcBag, srcSlot)
	s.sendPlayerUpdate()
	return true
}

// handleAutoStoreBankItem processes CMSG_AUTOSTORE_BANK_ITEM (0x282).
// Reference: WorldSession::HandleAutoStoreBankItemOpcode (BankHandler.cpp:95).
func (s *session) handleAutoStoreBankItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	srcBag, err := r.ReadU8()
	if err != nil {
		return false
	}
	srcSlot, err := r.ReadU8()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return false
	}

	var itemGUID uint64
	err = cdb.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ? AND item != 0 LIMIT 1", s.playerGUID, srcBag, srcSlot).Scan(&itemGUID)
	if err != nil || itemGUID == 0 {
		return true
	}

	isBank := (srcBag == 0 && srcSlot >= bankSlotStart && srcSlot <= bankSlotEnd)

	if isBank {
		// Move from bank to backpack (slots 23..38)
		occupied := make(map[uint8]bool)
		rows, err := cdb.QueryContext(ctx, "SELECT slot FROM character_inventory WHERE guid = ? AND bag = 0 AND slot >= ? AND slot <= ?", s.playerGUID, bagSlotStart, bagSlotEnd)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sl uint8
				if rows.Scan(&sl) == nil {
					occupied[sl] = true
				}
			}
		}
		var destSlot uint8 = 0xFF
		for sl := bagSlotStart; sl <= bagSlotEnd; sl++ {
			if !occupied[sl] {
				destSlot = sl
				break
			}
		}
		if destSlot == 0xFF {
			return true // Bags full
		}
		_, _ = cdb.ExecContext(ctx, "UPDATE character_inventory SET bag = 0, slot = ? WHERE guid = ? AND bag = ? AND slot = ?", destSlot, s.playerGUID, srcBag, srcSlot)
	} else {
		// Move from backpack to bank (slots 39..66)
		occupied := make(map[uint8]bool)
		rows, err := cdb.QueryContext(ctx, "SELECT slot FROM character_inventory WHERE guid = ? AND bag = 0 AND slot >= ? AND slot <= ?", s.playerGUID, bankSlotStart, bankSlotEnd)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sl uint8
				if rows.Scan(&sl) == nil {
					occupied[sl] = true
				}
			}
		}
		var destSlot uint8 = 0xFF
		for sl := bankSlotStart; sl <= bankSlotEnd; sl++ {
			if !occupied[sl] {
				destSlot = sl
				break
			}
		}
		if destSlot == 0xFF {
			return true // Bank full
		}
		_, _ = cdb.ExecContext(ctx, "UPDATE character_inventory SET bag = 0, slot = ? WHERE guid = ? AND bag = ? AND slot = ?", destSlot, s.playerGUID, srcBag, srcSlot)
	}

	s.sendPlayerUpdate()
	return true
}
