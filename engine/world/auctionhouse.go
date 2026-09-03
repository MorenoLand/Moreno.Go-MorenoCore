package world

import (
	"context"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type auctionRecord struct {
	ID         uint32
	ItemGUID   uint64
	ItemEntry  uint32
	ItemCount  uint32
	Owner      uint64
	StartBid   uint32
	Buyout     uint32
	Bidder     uint64
	Bid        uint32
	ExpireTime int64
	Deposit    uint32
}

func (s *session) handleAuctionHello(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	packet := protocol.NewBuffer(13)
	packet.WriteU64(guid)
	packet.WriteU32(1) // Neutral / Standard AH ID
	packet.WriteU8(1)  // Enabled
	_ = s.write(uint16(protocol.OpcodeMSG_AUCTION_HELLO), packet.Bytes(), true)
	s.debug("auction hello handled", "account", s.accountName, "auctioneer", guid)
	return true
}

func (s *session) handleAuctionListItems(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	listFrom, _ := reader.ReadU32()
	searchedName, _ := reader.ReadCString()
	cdb := s.server.CharactersStore.DB
	wdb := s.server.WorldStore.DB
	if cdb == nil || wdb == nil {
		return true
	}
	query := `SELECT ah.id, ah.itemguid, ah.item_template, ah.itemowner, ah.buyoutprice, ah.time, ah.buyguid, ah.lastbid, ah.startbid, ah.deposit, COALESCE(ii.count, 1)
		FROM auctionhouse AS ah
		LEFT JOIN item_instance AS ii ON ii.guid = ah.itemguid
		LEFT JOIN item_template AS it ON it.entry = ah.item_template
		WHERE ah.time > ?`
	args := []interface{}{time.Now().Unix()}
	if searchedName != "" {
		query += ` AND UPPER(it.name) LIKE UPPER(?)`
		args = append(args, "%"+searchedName+"%")
	}
	query += ` ORDER BY ah.id LIMIT 50 OFFSET ?`
	args = append(args, listFrom)
	rows, err := cdb.QueryContext(ctx, query, args...)
	if err != nil {
		return true
	}
	defer rows.Close()
	var auctions []auctionRecord
	for rows.Next() {
		var id, iGuid, iTmpl, owner, buyout, expTime, bidder, lastBid, startBid, deposit, count int64
		if err := rows.Scan(&id, &iGuid, &iTmpl, &owner, &buyout, &expTime, &bidder, &lastBid, &startBid, &deposit, &count); err == nil {
			auctions = append(auctions, auctionRecord{
				ID:         uint32(id),
				ItemGUID:   uint64(iGuid),
				ItemEntry:  uint32(iTmpl),
				ItemCount:  uint32(count),
				Owner:      uint64(owner),
				Buyout:     uint32(buyout),
				ExpireTime: expTime,
				Bidder:     uint64(bidder),
				Bid:        uint32(lastBid),
				StartBid:   uint32(startBid),
				Deposit:    uint32(deposit),
			})
		}
	}
	packet := protocol.NewBuffer(12 + len(auctions)*120)
	packet.WriteU32(uint32(len(auctions))) // Count
	for _, a := range auctions {
		writeAuctionInfo(packet, a)
	}
	packet.WriteU32(uint32(len(auctions))) // Total count
	packet.WriteU32(300)                   // Delay
	_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_LIST_RESULT), packet.Bytes(), true)
	s.debug("auction list items sent", "account", s.accountName, "count", len(auctions))
	return true
}

func (s *session) handleAuctionSellItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 24 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64() // auctioneer
	itemCount, err := reader.ReadU32()
	if err != nil || itemCount == 0 {
		return false
	}
	itemGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	stackCount, err := reader.ReadU32()
	if err != nil {
		return false
	}
	bid, _ := reader.ReadU32()
	buyout, _ := reader.ReadU32()
	etime, _ := reader.ReadU32() // minutes
	if etime == 0 {
		etime = 1440 // 24 hours
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var itemEntry int64
	err = cdb.QueryRowContext(ctx, "SELECT itemEntry FROM item_instance WHERE guid = ? AND owner_guid = ? LIMIT 1", itemGUID, s.playerGUID).Scan(&itemEntry)
	if err != nil {
		_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(0, 0, 2), true) // ERR_AUCTION_ITEM_NOT_FOUND = 2
		return true
	}
	deposit := uint32(100) // 1 silver base deposit
	if s.player.Money < deposit {
		_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(0, 0, 3), true) // ERR_AUCTION_NOT_ENOUGH_MONEY = 3
		return true
	}
	s.player.Money -= deposit
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM character_inventory WHERE guid = ? AND item = ?", s.playerGUID, itemGUID)
	now := time.Now().Unix()
	expire := now + int64(etime*60)
	var nextID int64
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM auctionhouse").Scan(&nextID)
	if nextID <= 0 {
		nextID = 1
	}
	_, _ = cdb.ExecContext(ctx, `INSERT INTO auctionhouse (id, houseid, itemguid, item_template, itemCount, itemowner, buyoutprice, time, buyguid, lastbid, startbid, deposit)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)`, nextID, itemGUID, itemEntry, stackCount, s.playerGUID, buyout, expire, bid, deposit)
	_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(uint32(nextID), 0, 0), true) // AUCTION_SELL_ITEM = 0, ERR_AUCTION_OK = 0
	_ = s.sendInventoryItems(ctx)
	s.sendPlayerUpdate()
	s.debug("auction created", "account", s.accountName, "auction_id", nextID, "item", itemEntry)
	return true
}

func (s *session) handleAuctionPlaceBid(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	auctionID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	price, err := reader.ReadU32()
	if err != nil {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var itemGUID, itemEntry, ownerGUID, buyout, bidderGUID, lastBid int64
	err = cdb.QueryRowContext(ctx, "SELECT itemguid, item_template, itemowner, buyoutprice, buyguid, lastbid FROM auctionhouse WHERE id = ? LIMIT 1", auctionID).Scan(&itemGUID, &itemEntry, &ownerGUID, &buyout, &bidderGUID, &lastBid)
	if err != nil {
		return true
	}
	if s.player.Money < price {
		_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(auctionID, 1, 3), true)
		return true
	}
	s.player.Money -= price
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	if buyout > 0 && price >= uint32(buyout) {
		// Buyout success!
		_, _ = cdb.ExecContext(ctx, "DELETE FROM auctionhouse WHERE id = ?", auctionID)
		// Send won mail with item to buyer
		now := time.Now().Unix()
		var nextMailID int64
		_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM mail").Scan(&nextMailID)
		_, _ = cdb.ExecContext(ctx, "INSERT INTO mail (id, messageType, stationery, mailTemplateId, sender, receiver, subject, body, has_items, expire_time, deliver_time, money, cod, checked) VALUES (?, 2, 41, 0, ?, ?, 'Auction won: Item', '', 1, ?, ?, 0, 0, 0)",
			nextMailID, ownerGUID, s.playerGUID, now+30*86400, now)
		_, _ = cdb.ExecContext(ctx, "INSERT INTO mail_items (mail_id, item_guid, item_template, receiver) VALUES (?, ?, ?, ?)", nextMailID, itemGUID, itemEntry, s.playerGUID)
		_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET owner_guid = ? WHERE guid = ?", s.playerGUID, itemGUID)
		s.sendMailNotify(uint64(s.playerGUID))
		// Send profit mail to seller
		var sellerMailID int64
		_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM mail").Scan(&sellerMailID)
		_, _ = cdb.ExecContext(ctx, "INSERT INTO mail (id, messageType, stationery, mailTemplateId, sender, receiver, subject, body, has_items, expire_time, deliver_time, money, cod, checked) VALUES (?, 2, 41, 0, ?, ?, 'Auction successful: Item', '', 0, ?, ?, ?, 0, 0)",
			sellerMailID, s.playerGUID, ownerGUID, now+30*86400, now, price)
		s.sendMailNotify(uint64(ownerGUID))
		_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(auctionID, 1, 0), true)
	} else {
		// Outbid
		if bidderGUID != 0 && lastBid > 0 {
			// Refund previous bidder via mail
			now := time.Now().Unix()
			var refundMailID int64
			_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM mail").Scan(&refundMailID)
			_, _ = cdb.ExecContext(ctx, "INSERT INTO mail (id, messageType, stationery, mailTemplateId, sender, receiver, subject, body, has_items, expire_time, deliver_time, money, cod, checked) VALUES (?, 2, 41, 0, ?, ?, 'Auction outbid', '', 0, ?, ?, ?, 0, 0)",
				refundMailID, ownerGUID, bidderGUID, now+30*86400, now, lastBid)
			s.sendMailNotify(uint64(bidderGUID))
		}
		_, _ = cdb.ExecContext(ctx, "UPDATE auctionhouse SET buyguid = ?, lastbid = ? WHERE id = ?", s.playerGUID, price, auctionID)
		_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(auctionID, 1, 0), true)
	}
	s.sendPlayerUpdate()
	s.debug("auction bid placed", "account", s.accountName, "auction_id", auctionID, "price", price)
	return true
}

func (s *session) handleAuctionListOwnerItems(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	rows, err := cdb.QueryContext(ctx, `SELECT ah.id, ah.itemguid, ah.item_template, ah.itemowner, ah.buyoutprice, ah.time, ah.buyguid, ah.lastbid, ah.startbid, ah.deposit, COALESCE(ii.count, 1)
		FROM auctionhouse AS ah
		LEFT JOIN item_instance AS ii ON ii.guid = ah.itemguid
		WHERE ah.itemowner = ? AND ah.time > ?`, s.playerGUID, time.Now().Unix())
	if err != nil {
		return true
	}
	defer rows.Close()
	var auctions []auctionRecord
	for rows.Next() {
		var id, iGuid, iTmpl, owner, buyout, expTime, bidder, lastBid, startBid, deposit, count int64
		if err := rows.Scan(&id, &iGuid, &iTmpl, &owner, &buyout, &expTime, &bidder, &lastBid, &startBid, &deposit, &count); err == nil {
			auctions = append(auctions, auctionRecord{
				ID:         uint32(id),
				ItemGUID:   uint64(iGuid),
				ItemEntry:  uint32(iTmpl),
				ItemCount:  uint32(count),
				Owner:      uint64(owner),
				Buyout:     uint32(buyout),
				ExpireTime: expTime,
				Bidder:     uint64(bidder),
				Bid:        uint32(lastBid),
				StartBid:   uint32(startBid),
				Deposit:    uint32(deposit),
			})
		}
	}
	packet := protocol.NewBuffer(12 + len(auctions)*120)
	packet.WriteU32(uint32(len(auctions)))
	for _, a := range auctions {
		writeAuctionInfo(packet, a)
	}
	packet.WriteU32(uint32(len(auctions)))
	packet.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_OWNER_LIST_RESULT), packet.Bytes(), true)
	return true
}

func (s *session) handleAuctionListBidderItems(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	rows, err := cdb.QueryContext(ctx, `SELECT ah.id, ah.itemguid, ah.item_template, ah.itemowner, ah.buyoutprice, ah.time, ah.buyguid, ah.lastbid, ah.startbid, ah.deposit, COALESCE(ii.count, 1)
		FROM auctionhouse AS ah
		LEFT JOIN item_instance AS ii ON ii.guid = ah.itemguid
		WHERE ah.buyguid = ? AND ah.time > ?`, s.playerGUID, time.Now().Unix())
	if err != nil {
		return true
	}
	defer rows.Close()
	var auctions []auctionRecord
	for rows.Next() {
		var id, iGuid, iTmpl, owner, buyout, expTime, bidder, lastBid, startBid, deposit, count int64
		if err := rows.Scan(&id, &iGuid, &iTmpl, &owner, &buyout, &expTime, &bidder, &lastBid, &startBid, &deposit, &count); err == nil {
			auctions = append(auctions, auctionRecord{
				ID:         uint32(id),
				ItemGUID:   uint64(iGuid),
				ItemEntry:  uint32(iTmpl),
				ItemCount:  uint32(count),
				Owner:      uint64(owner),
				Buyout:     uint32(buyout),
				ExpireTime: expTime,
				Bidder:     uint64(bidder),
				Bid:        uint32(lastBid),
				StartBid:   uint32(startBid),
				Deposit:    uint32(deposit),
			})
		}
	}
	packet := protocol.NewBuffer(12 + len(auctions)*120)
	packet.WriteU32(uint32(len(auctions)))
	for _, a := range auctions {
		writeAuctionInfo(packet, a)
	}
	packet.WriteU32(uint32(len(auctions)))
	packet.WriteU32(300)
	_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_BIDDER_LIST_RESULT), packet.Bytes(), true)
	return true
}

func (s *session) handleAuctionRemoveItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	auctionID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var itemGUID, itemEntry, ownerGUID int64
	err = cdb.QueryRowContext(ctx, "SELECT itemguid, item_template, itemowner FROM auctionhouse WHERE id = ? AND itemowner = ? LIMIT 1", auctionID, s.playerGUID).Scan(&itemGUID, &itemEntry, &ownerGUID)
	if err != nil {
		return true
	}
	_, _ = cdb.ExecContext(ctx, "DELETE FROM auctionhouse WHERE id = ?", auctionID)
	// Mail item back to owner
	now := time.Now().Unix()
	var nextMailID int64
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM mail").Scan(&nextMailID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO mail (id, messageType, stationery, mailTemplateId, sender, receiver, subject, body, has_items, expire_time, deliver_time, money, cod, checked) VALUES (?, 2, 41, 0, ?, ?, 'Auction cancelled', '', 1, ?, ?, 0, 0, 0)",
		nextMailID, ownerGUID, ownerGUID, now+30*86400, now)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO mail_items (mail_id, item_guid, item_template, receiver) VALUES (?, ?, ?, ?)", nextMailID, itemGUID, itemEntry, ownerGUID)
	s.sendMailNotify(uint64(ownerGUID))
	_ = s.write(uint16(protocol.OpcodeSMSG_AUCTION_COMMAND_RESULT), buildAuctionCommandResult(auctionID, 2, 0), true) // AUCTION_CANCEL = 2, ERR_AUCTION_OK = 0
	return true
}

func (s *session) sendMailNotify(receiverGUID uint64) {
	if s.server == nil {
		return
	}
	targetSess := s.server.findSessionByGUID(receiverGUID)
	if targetSess != nil {
		recvPacket := protocol.NewBuffer(4)
		recvPacket.WriteF32(0) // time remaining
		_ = targetSess.write(uint16(protocol.OpcodeSMSG_RECEIVED_MAIL), recvPacket.Bytes(), true)
	}
}

func writeAuctionInfo(buf *protocol.Buffer, a auctionRecord) {
	now := time.Now().Unix()
	timeLeftMs := uint32(0)
	if a.ExpireTime > now {
		timeLeftMs = uint32((a.ExpireTime - now) * 1000)
	}
	buf.WriteU32(a.ID)
	buf.WriteU32(a.ItemEntry)
	for i := 0; i < 6; i++ {
		buf.WriteU32(0)
		buf.WriteU32(0)
		buf.WriteU32(0)
	}
	buf.WriteI32(0) // RandomPropertyId
	buf.WriteU32(0) // SuffixFactor
	buf.WriteU32(a.ItemCount)
	buf.WriteU32(0) // SpellCharges
	buf.WriteU32(0) // ItemFlags
	buf.WriteU64(a.Owner)
	buf.WriteU32(a.StartBid)
	minOutBid := uint32(0)
	if a.Bid > 0 {
		minOutBid = a.Bid * 5 / 100
		if minOutBid == 0 {
			minOutBid = 1
		}
	}
	buf.WriteU32(minOutBid)
	buf.WriteU32(a.Buyout)
	buf.WriteU32(timeLeftMs)
	buf.WriteU64(a.Bidder)
	buf.WriteU32(a.Bid)
}

func buildAuctionCommandResult(auctionID, action, result uint32) []byte {
	buf := protocol.NewBuffer(12)
	buf.WriteU32(auctionID)
	buf.WriteU32(action)
	buf.WriteU32(result)
	return buf.Bytes()
}

// handleAuctionListPendingSales processes CMSG_AUCTION_LIST_PENDING_SALES (0x48F).
// Reference: WorldSession::HandleAuctionListPendingSales (AuctionHouseHandler.cpp:812).
func (s *session) handleAuctionListPendingSales(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	if len(payload) >= 8 {
		r := protocol.NewReader(payload)
		_, _ = r.ReadU64() // auctioneer GUID (reference AuctionHouseHandler.cpp:816)
	}
	buf := protocol.NewBuffer(4)
	buf.WriteU32(0) // count = 0 pending sales
	return s.write(uint16(protocol.OpcodeSMSG_AUCTION_LIST_PENDING_SALES), buf.Bytes(), true) == nil
}
