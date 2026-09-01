package world

import (
	"context"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type mailEntryRecord struct {
	ID           uint32
	MessageType  uint8
	Stationery   uint32
	Sender       uint64
	Receiver     uint64
	Subject      string
	Body         string
	Money        uint32
	COD          uint32
	Checked      uint32
	ExpireTime   int64
	DeliverTime  int64
	MailTemplate uint32
	Items        []mailItemRecord
}

type mailItemRecord struct {
	AttachID      uint32
	ItemEntry     uint32
	Count         uint32
	MaxDurability uint32
	Durability    uint32
}

func (s *session) handleGetMailList(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}
	db := s.server.CharactersStore.DB
	now := time.Now().Unix()
	rows, err := db.QueryContext(ctx, `SELECT id, messageType, stationery, mailTemplateId, sender, receiver, subject, body, expire_time, deliver_time, money, cod, checked
		FROM mail WHERE receiver = ? AND deliver_time <= ? ORDER BY id DESC LIMIT 50`, s.playerGUID, now)
	if err != nil {
		return true
	}
	defer rows.Close()
	var mails []mailEntryRecord
	for rows.Next() {
		var id, msgType, stat, tmpl, sender, receiver, exp, del, money, cod, checked int64
		var subject, body string
		if err := rows.Scan(&id, &msgType, &stat, &tmpl, &sender, &receiver, &subject, &body, &exp, &del, &money, &cod, &checked); err != nil {
			continue
		}
		m := mailEntryRecord{
			ID:           uint32(id),
			MessageType:  uint8(msgType),
			Stationery:   uint32(stat),
			Sender:       uint64(sender),
			Receiver:     uint64(receiver),
			Subject:      subject,
			Body:         body,
			ExpireTime:   exp,
			DeliverTime:  del,
			Money:        uint32(money),
			COD:          uint32(cod),
			Checked:      uint32(checked),
			MailTemplate: uint32(tmpl),
		}
		if m.Stationery == 0 {
			m.Stationery = 41 // Standard default letter stationery
		}
		// Load attached items
		iRows, iErr := db.QueryContext(ctx, `SELECT mi.item_guid, mi.item_template, COALESCE(ii.count, 1), COALESCE(ii.durability, 0)
			FROM mail_items AS mi
			LEFT JOIN item_instance AS ii ON ii.guid = mi.item_guid
			WHERE mi.mail_id = ?`, id)
		if iErr == nil {
			for iRows.Next() {
				var iGuid, iTmpl, iCount, iDur int64
				if iRows.Scan(&iGuid, &iTmpl, &iCount, &iDur) == nil {
					m.Items = append(m.Items, mailItemRecord{
						AttachID:   uint32(iGuid),
						ItemEntry:  uint32(iTmpl),
						Count:      uint32(iCount),
						Durability: uint32(iDur),
					})
				}
			}
			iRows.Close()
		}
		mails = append(mails, m)
	}
	// Build SMSG_MAIL_LIST_RESULT (0x23B)
	packet := protocol.NewBuffer(512)
	packet.WriteU32(uint32(len(mails))) // TotalNumRecords
	packet.WriteU8(uint8(len(mails)))   // Mails.size()
	for _, m := range mails {
		daysLeft := float32(m.ExpireTime-now) / 86400.0
		if daysLeft < 0 {
			daysLeft = 0
		}
		msgBuf := protocol.NewBuffer(128)
		msgBuf.WriteU32(m.ID)
		msgBuf.WriteU8(m.MessageType)
		if m.MessageType == 0 {
			msgBuf.WriteU64(m.Sender)
		} else {
			msgBuf.WriteU32(uint32(m.Sender))
		}
		msgBuf.WriteU32(m.COD)
		msgBuf.WriteU32(0) // PackageID
		msgBuf.WriteU32(m.Stationery)
		msgBuf.WriteU32(m.Money)
		msgBuf.WriteU32(m.Checked)
		msgBuf.WriteF32(daysLeft)
		msgBuf.WriteU32(m.MailTemplate)
		msgBuf.WriteString(m.Subject)
		msgBuf.WriteString(m.Body)
		msgBuf.WriteU8(uint8(len(m.Items)))
		for pos, it := range m.Items {
			msgBuf.WriteU8(uint8(pos))
			msgBuf.WriteU32(it.AttachID)
			msgBuf.WriteU32(it.ItemEntry)
			for j := 0; j < 6; j++ {
				msgBuf.WriteU32(0)
				msgBuf.WriteU32(0)
				msgBuf.WriteU32(0)
			}
			msgBuf.WriteU32(0) // RandomPropertiesID
			msgBuf.WriteU32(0) // RandomPropertiesSeed
			msgBuf.WriteU32(it.Count)
			msgBuf.WriteU32(0) // Charges
			msgBuf.WriteU32(it.MaxDurability)
			msgBuf.WriteU32(it.Durability)
			msgBuf.WriteU8(1) // Unlocked
		}
		entryBytes := msgBuf.Bytes()
		packet.WriteU16(uint16(len(entryBytes)))
		packet.Write(entryBytes)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_MAIL_LIST_RESULT), packet.Bytes(), true)
	s.debug("mail list sent", "account", s.accountName, "mails", len(mails))
	return true
}

func (s *session) handleSendMail(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 20 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64() // mailbox GUID
	targetName, err := reader.ReadCString()
	if err != nil || targetName == "" {
		return false
	}
	subject, err := reader.ReadCString()
	if err != nil {
		return false
	}
	body, err := reader.ReadCString()
	if err != nil {
		return false
	}
	stationery, err := reader.ReadU32()
	if err != nil {
		stationery = 41
	}
	_, _ = reader.ReadU32() // packageID
	attachCount, err := reader.ReadU8()
	if err != nil {
		attachCount = 0
	}
	type itemAttachment struct {
		Slot     uint8
		ItemGUID uint64
	}
	var attachments []itemAttachment
	for i := uint8(0); i < attachCount; i++ {
		slot, _ := reader.ReadU8()
		itemGUID, _ := reader.ReadU64()
		if itemGUID != 0 {
			attachments = append(attachments, itemAttachment{Slot: slot, ItemGUID: itemGUID})
		}
	}
	money, _ := reader.ReadU32()
	cod, _ := reader.ReadU32()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	// Find receiver
	var receiverGUID int64
	err = cdb.QueryRowContext(ctx, "SELECT guid FROM characters WHERE UPPER(name) = UPPER(?) LIMIT 1", targetName).Scan(&receiverGUID)
	if err != nil || receiverGUID == 0 {
		_ = s.write(uint16(protocol.OpcodeSMSG_SEND_MAIL_RESULT), buildSendMailResult(0, 0, 1), true) // MAIL_ERR_RECIPIENT_NOT_FOUND = 1
		return true
	}
	postageFee := uint32(30)
	if len(attachments) > 0 {
		postageFee = uint32(30 * len(attachments))
	}
	totalRequired := postageFee + money
	if s.player.Money < totalRequired {
		_ = s.write(uint16(protocol.OpcodeSMSG_SEND_MAIL_RESULT), buildSendMailResult(0, 0, 3), true) // MAIL_ERR_NOT_ENOUGH_MONEY = 3
		return true
	}
	s.player.Money -= totalRequired
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	now := time.Now().Unix()
	expire := now + 30*86400
	hasItems := 0
	if len(attachments) > 0 {
		hasItems = 1
	}
	var nextMailID int64
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM mail").Scan(&nextMailID)
	if nextMailID <= 0 {
		nextMailID = 1
	}
	// TrinityCore MailDraft::SendMailTo stores checked as MAIL_CHECK_MASK_HAS_BODY (0x10)
	// when a body text is present and MAIL_CHECK_MASK_COPIED (0x04) otherwise.
	checked := uint32(0x04)
	if body != "" {
		checked = 0x10
	}
	_, err = cdb.ExecContext(ctx, `INSERT INTO mail (id, messageType, stationery, mailTemplateId, sender, receiver, subject, body, has_items, expire_time, deliver_time, money, cod, checked)
		VALUES (?, 0, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nextMailID, stationery, s.playerGUID, receiverGUID, subject, body, hasItems, expire, now, money, cod, checked)
	if err != nil {
		return true
	}
	for _, att := range attachments {
		var itemEntry int64
		_ = cdb.QueryRowContext(ctx, "SELECT itemEntry FROM item_instance WHERE guid = ?", att.ItemGUID).Scan(&itemEntry)
		_, _ = cdb.ExecContext(ctx, "DELETE FROM character_inventory WHERE guid = ? AND item = ?", s.playerGUID, att.ItemGUID)
		_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET owner_guid = ? WHERE guid = ?", receiverGUID, att.ItemGUID)
		_, _ = cdb.ExecContext(ctx, "INSERT INTO mail_items (mail_id, item_guid, item_template, receiver) VALUES (?, ?, ?, ?)", nextMailID, att.ItemGUID, itemEntry, receiverGUID)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_SEND_MAIL_RESULT), buildSendMailResult(uint32(nextMailID), 0, 0), true) // MAIL_OK = 0
	s.sendPlayerUpdate()
	// Notify receiver if online
	targetSess := s.server.findSessionByGUID(uint64(receiverGUID))
	if targetSess != nil {
		recvPacket := protocol.NewBuffer(4)
		recvPacket.WriteF32(0) // time remaining
		_ = targetSess.write(uint16(protocol.OpcodeSMSG_RECEIVED_MAIL), recvPacket.Bytes(), true)
	}
	s.debug("mail sent successfully", "from", s.accountName, "to", targetName, "mail_id", nextMailID)
	return true
}

func (s *session) handleMailTakeMoney(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	mailID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var money int64
	err = cdb.QueryRowContext(ctx, "SELECT money FROM mail WHERE id = ? AND receiver = ? LIMIT 1", mailID, s.playerGUID).Scan(&money)
	if err != nil || money <= 0 {
		return true
	}
	s.player.Money += uint32(money)
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "UPDATE mail SET money = 0 WHERE id = ?", mailID)
	_ = s.write(uint16(protocol.OpcodeSMSG_SEND_MAIL_RESULT), buildSendMailResult(mailID, 1, 0), true) // MAIL_MONEY_TAKEN = 1, MAIL_OK = 0
	s.sendPlayerUpdate()
	s.debug("mail money collected", "account", s.accountName, "mail_id", mailID, "money", money)
	return true
}

func (s *session) handleMailTakeItem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	mailID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	attachID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var itemEntry int64
	err = cdb.QueryRowContext(ctx, "SELECT item_template FROM mail_items WHERE mail_id = ? AND item_guid = ? LIMIT 1", mailID, attachID).Scan(&itemEntry)
	if err != nil || itemEntry == 0 {
		return true
	}
	// Find free backpack slot
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
	_, _ = cdb.ExecContext(ctx, "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)", s.playerGUID, freeSlot, attachID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM mail_items WHERE mail_id = ? AND item_guid = ?", mailID, attachID)
	// Check if any items left
	var remainingCount int64
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM mail_items WHERE mail_id = ?", mailID).Scan(&remainingCount)
	if remainingCount == 0 {
		_, _ = cdb.ExecContext(ctx, "UPDATE mail SET has_items = 0 WHERE id = ?", mailID)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_SEND_MAIL_RESULT), buildSendMailResult(mailID, 2, 0), true) // MAIL_ITEM_TAKEN = 2, MAIL_OK = 0
	s.sendPlayerUpdate()
	s.debug("mail item collected", "account", s.accountName, "mail_id", mailID, "item", itemEntry, "slot", freeSlot)
	return true
}

func (s *session) handleMailDelete(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	mailID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "DELETE FROM mail WHERE id = ? AND receiver = ?", mailID, s.playerGUID)
		_, _ = cdb.ExecContext(ctx, "DELETE FROM mail_items WHERE mail_id = ?", mailID)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_SEND_MAIL_RESULT), buildSendMailResult(mailID, 3, 0), true) // MAIL_DELETED = 3, MAIL_OK = 0
	s.debug("mail deleted", "account", s.accountName, "mail_id", mailID)
	return true
}

func (s *session) handleMailMarkAsRead(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64()
	mailID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE mail SET checked = (checked | 1) WHERE id = ? AND receiver = ?", mailID, s.playerGUID)
	}
	return true
}

func (s *session) handleQueryNextMailTime(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	// TrinityCore HandleQueryNextMailTime: unread and already delivered mails
	// (checked & MAIL_CHECK_MASK_READ) == 0 are listed once per sender (max 3
	// entries); when none exist the client is told -DAY so no mail icon shows.
	type nextMailEntry struct {
		Sender      uint64
		AltSender   uint32
		MessageType uint8
		Stationery  uint32
		TimeLeft    float32
	}
	var entries []nextMailEntry
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		rows, err := s.server.CharactersStore.DB.QueryContext(ctx, `SELECT messageType, stationery, sender, deliver_time
			FROM mail WHERE receiver = ? AND (checked & 1) = 0 AND deliver_time <= ? ORDER BY id DESC`, s.playerGUID, time.Now().Unix())
		if err == nil {
			seenSenders := make(map[uint64]struct{})
			for rows.Next() {
				var msgType, stationery, sender int64
				var deliverTime int64
				if err := rows.Scan(&msgType, &stationery, &sender, &deliverTime); err != nil {
					continue
				}
				senderGUID := uint64(sender)
				if _, dup := seenSenders[senderGUID]; dup {
					continue
				}
				seenSenders[senderGUID] = struct{}{}
				entries = append(entries, nextMailEntry{
					Sender:      senderGUID,
					MessageType: uint8(msgType),
					Stationery:  uint32(stationery),
					TimeLeft:    float32(deliverTime - time.Now().Unix()),
				})
				if len(seenSenders) > 2 {
					break
				}
			}
			rows.Close()
		}
	}
	buf := protocol.NewBuffer(32)
	if len(entries) > 0 {
		buf.WriteF32(0) // NextMailTime: mail is ready now
	} else {
		buf.WriteF32(-86400) // -DAY: no unread mail, hides the notification
	}
	buf.WriteU32(uint32(len(entries)))
	for _, entry := range entries {
		if entry.MessageType == 0 { // MAIL_NORMAL sends the full player GUID
			buf.WriteU64(entry.Sender)
		} else {
			buf.WriteU64(0)
		}
		if entry.MessageType != 0 {
			buf.WriteU32(entry.AltSender) // AltSenderID
			buf.WriteU32(uint32(entry.MessageType))
		} else {
			buf.WriteU32(0)
			buf.WriteU32(0)
		}
		buf.WriteU32(entry.Stationery)
		buf.WriteF32(entry.TimeLeft)
	}
	_ = s.write(uint16(protocol.OpcodeMSG_QUERY_NEXT_MAIL_TIME), buf.Bytes(), true)
	return true
}

func buildSendMailResult(mailID, action, result uint32) []byte {
	buf := protocol.NewBuffer(12)
	buf.WriteU32(mailID)
	buf.WriteU32(action)
	buf.WriteU32(result)
	return buf.Bytes()
}
