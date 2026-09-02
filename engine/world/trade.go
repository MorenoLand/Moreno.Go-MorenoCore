package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	tradeStatusBeginTrade  = 0
	tradeStatusOpenWindow  = 1
	tradeStatusTradeChange = 2
	tradeStatusCancelled   = 3
	tradeStatusTradeAccept = 4
	tradeStatusUnaccept    = 5
	tradeStatusComplete    = 7
	tradeStatusCloseWindow = 8
	tradeStatusOnlyTarget  = 9
	tradeStatusNotEligible = 10
	tradeStatusInitiated   = 11
)

type tradeSlotItem struct {
	ItemGUID   uint64
	ItemEntry  uint32
	DisplayID  uint32
	StackCount uint32
}

type playerTradeState struct {
	Partner  *session
	Money    uint32
	Items    map[uint8]tradeSlotItem
	Accepted bool
}

func (s *session) handleInitiateTrade(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	targetGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	targetSess := s.server.findSessionByGUID(targetGUID)
	if targetSess == nil || targetSess.player == nil {
		return true
	}
	s.trade = &playerTradeState{Partner: targetSess, Items: make(map[uint8]tradeSlotItem)}
	targetSess.trade = &playerTradeState{Partner: s, Items: make(map[uint8]tradeSlotItem)}

	// Send SMSG_TRADE_STATUS (TRADE_STATUS_BEGIN_TRADE) to target
	buf := protocol.NewBuffer(12)
	buf.WriteU32(tradeStatusBeginTrade)
	buf.WriteU64(s.playerGUID)
	_ = targetSess.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), buf.Bytes(), true)
	s.debug("trade initiated", "from", s.accountName, "to", targetSess.accountName)
	return true
}

func (s *session) handleBeginTrade(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil || s.trade.Partner == nil {
		return true
	}
	partner := s.trade.Partner
	buf := protocol.NewBuffer(8)
	buf.WriteU32(tradeStatusOpenWindow)
	buf.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), buf.Bytes(), true)
	_ = partner.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), buf.Bytes(), true)
	s.debug("trade window opened", "player1", s.accountName, "player2", partner.accountName)
	return true
}

func (s *session) handleSetTradeGold(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil || len(payload) < 4 {
		return true
	}
	reader := protocol.NewReader(payload)
	gold, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if gold > s.player.Money {
		gold = s.player.Money
	}
	s.trade.Money = gold
	s.trade.Accepted = false
	if s.trade.Partner != nil {
		s.trade.Partner.trade.Accepted = false
		s.sendTradeStatusExtended(true)
	}
	return true
}

func (s *session) handleSetTradeItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil || len(payload) < 3 {
		return true
	}
	tradeSlot := payload[0]
	bag := payload[1]
	slot := payload[2]
	if tradeSlot >= 7 {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var itemGUID int64
	err := cdb.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ? LIMIT 1", s.playerGUID, bag, slot).Scan(&itemGUID)
	if err != nil || itemGUID == 0 {
		return true
	}
	var itemEntry, count int64
	_ = cdb.QueryRowContext(ctx, "SELECT itemEntry, count FROM item_instance WHERE guid = ? LIMIT 1", itemGUID).Scan(&itemEntry, &count)
	s.trade.Items[tradeSlot] = tradeSlotItem{
		ItemGUID:   uint64(itemGUID),
		ItemEntry:  uint32(itemEntry),
		StackCount: uint32(count),
	}
	s.trade.Accepted = false
	if s.trade.Partner != nil {
		s.trade.Partner.trade.Accepted = false
		s.sendTradeStatusExtended(true)
	}
	return true
}

func (s *session) handleClearTradeItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil || len(payload) < 1 {
		return true
	}
	tradeSlot := payload[0]
	delete(s.trade.Items, tradeSlot)
	s.trade.Accepted = false
	if s.trade.Partner != nil {
		s.trade.Partner.trade.Accepted = false
		s.sendTradeStatusExtended(true)
	}
	return true
}

func (s *session) handleAcceptTrade(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil || s.trade.Partner == nil {
		return true
	}
	s.trade.Accepted = true
	partner := s.trade.Partner

	acceptBuf := protocol.NewBuffer(4)
	acceptBuf.WriteU32(tradeStatusTradeAccept)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), acceptBuf.Bytes(), true)
	_ = partner.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), acceptBuf.Bytes(), true)

	if partner.trade != nil && partner.trade.Accepted {
		// Both accepted -> execute trade
		s.completeTrade(ctx, partner)
	}
	return true
}

func (s *session) completeTrade(ctx context.Context, partner *session) {
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return
	}
	// Money transfer
	s.player.Money = s.player.Money - s.trade.Money + partner.trade.Money
	partner.player.Money = partner.player.Money - partner.trade.Money + s.trade.Money
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", partner.player.Money, partner.playerGUID)

	// Items transfer
	for _, it := range s.trade.Items {
		_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET owner_guid = ? WHERE guid = ?", partner.playerGUID, it.ItemGUID)
		_, _ = cdb.ExecContext(ctx, "UPDATE character_inventory SET guid = ? WHERE item = ?", partner.playerGUID, it.ItemGUID)
	}
	for _, it := range partner.trade.Items {
		_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET owner_guid = ? WHERE guid = ?", s.playerGUID, it.ItemGUID)
		_, _ = cdb.ExecContext(ctx, "UPDATE character_inventory SET guid = ? WHERE item = ?", s.playerGUID, it.ItemGUID)
	}

	completeBuf := protocol.NewBuffer(4)
	completeBuf.WriteU32(tradeStatusComplete)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), completeBuf.Bytes(), true)
	_ = partner.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), completeBuf.Bytes(), true)

	s.sendPlayerUpdate()
	partner.sendPlayerUpdate()

	s.trade = nil
	partner.trade = nil
	s.debug("trade completed successfully", "player1", s.accountName, "player2", partner.accountName)
}

func (s *session) handleUnacceptTrade(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil {
		return true
	}
	s.trade.Accepted = false
	unacceptBuf := protocol.NewBuffer(4)
	unacceptBuf.WriteU32(tradeStatusUnaccept)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), unacceptBuf.Bytes(), true)
	if s.trade.Partner != nil {
		s.trade.Partner.trade.Accepted = false
		_ = s.trade.Partner.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), unacceptBuf.Bytes(), true)
	}
	return true
}

func (s *session) handleCancelTrade(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil || s.trade == nil {
		return true
	}
	partner := s.trade.Partner
	cancelBuf := protocol.NewBuffer(4)
	cancelBuf.WriteU32(tradeStatusCancelled)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), cancelBuf.Bytes(), true)
	s.trade = nil
	if partner != nil {
		partner.trade = nil
		_ = partner.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS), cancelBuf.Bytes(), true)
	}
	return true
}

func (s *session) sendTradeStatusExtended(traderData bool) {
	if s.trade == nil || s.trade.Partner == nil {
		return
	}
	data := s.trade
	if traderData {
		data = s.trade.Partner.trade
	}
	if data == nil {
		return
	}
	buf := protocol.NewBuffer(256)
	if traderData {
		buf.WriteU8(1)
	} else {
		buf.WriteU8(0)
	}
	buf.WriteU32(0) // tradeID
	buf.WriteU32(7) // tradeSlotCount
	buf.WriteU32(7) // tradeSlotCount
	buf.WriteU32(data.Money)
	buf.WriteU32(0) // spell
	for i := uint8(0); i < 7; i++ {
		buf.WriteU8(i)
		if it, ok := data.Items[i]; ok {
			buf.WriteU32(it.ItemEntry)
			buf.WriteU32(it.DisplayID)
			buf.WriteU32(it.StackCount)
			buf.WriteU32(0) // wrapped
			buf.WriteU64(0) // giftCreator
			buf.WriteU32(0) // permEnchant
			for j := 0; j < 3; j++ {
				buf.WriteU32(0) // gem sockets
			}
			buf.WriteU64(0) // creator
			buf.WriteU32(0) // charges
			buf.WriteU32(0) // suffixFactor
			buf.WriteU32(0) // randomPropId
			buf.WriteU32(0) // lockId
			buf.WriteU32(0) // maxDurability
			buf.WriteU32(0) // durability
		} else {
			for j := 0; j < 18; j++ {
				buf.WriteU32(0)
			}
		}
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_TRADE_STATUS_EXTENDED), buf.Bytes(), true)
}
