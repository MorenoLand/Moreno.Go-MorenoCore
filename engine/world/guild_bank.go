package world

import (
	"context"
	"database/sql"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type guildBankTabInfo struct {
	ID   uint8
	Name string
	Icon string
	Text string
}

type guildBankSlotItem struct {
	Slot  uint8
	Entry uint32
	Count int32
}

// handleGuildBankerActivate processes CMSG_GUILD_BANKER_ACTIVATE (0x3E6).
// Reference: WorldSession::HandleGuildBankActivate (GuildHandler.cpp:251).
func (s *session) handleGuildBankerActivate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	fullUpdate := uint8(1)
	if len(payload) >= 9 {
		fullUpdate, _ = r.ReadU8()
	}

	return s.sendGuildBankList(ctx, bankerGUID, 0, fullUpdate != 0)
}

// handleGuildBankQueryTab processes CMSG_GUILD_BANK_QUERY_TAB (0x3E7).
// Reference: WorldSession::HandleGuildBankQueryTab (GuildHandler.cpp:271).
func (s *session) handleGuildBankQueryTab(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	tabID, err := r.ReadU8()
	if err != nil {
		return false
	}
	fullUpdate := uint8(0)
	if len(payload) >= 10 {
		fullUpdate, _ = r.ReadU8()
	}

	return s.sendGuildBankList(ctx, bankerGUID, tabID, fullUpdate != 0)
}

func (s *session) sendGuildBankList(ctx context.Context, bankerGUID uint64, tabID uint8, fullUpdate bool) bool {
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, bankMoney int64
	err := cdb.QueryRowContext(ctx, `SELECT g.guildid, g.BankMoney FROM guild_member AS gm
		JOIN guild AS g ON g.guildid = gm.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &bankMoney)
	if err != nil || guildID == 0 {
		resBuf := protocol.NewBuffer(12)
		resBuf.WriteI32(11) // GUILD_COMMAND_VIEW_TAB
		resBuf.WriteCString("")
		resBuf.WriteI32(1)  // ERR_GUILD_PLAYER_NOT_IN_GUILD
		_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_COMMAND_RESULT), resBuf.Bytes(), true)
		return true
	}

	var tabs []guildBankTabInfo
	rows, err := cdb.QueryContext(ctx, "SELECT TabId, TabName, TabIcon, COALESCE(TabText, '') FROM guild_bank_tab WHERE guildid = ? ORDER BY TabId", guildID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t guildBankTabInfo
			var tid int
			if scanErr := rows.Scan(&tid, &t.Name, &t.Icon, &t.Text); scanErr == nil {
				t.ID = uint8(tid)
				tabs = append(tabs, t)
			}
		}
	}
	if len(tabs) == 0 {
		tabs = []guildBankTabInfo{
			{ID: 0, Name: "General", Icon: "INV_Misc_Bag_08", Text: ""},
		}
	}

	var items []guildBankSlotItem
	itemRows, err := cdb.QueryContext(ctx, `SELECT gbi.SlotId, ii.itemEntry, ii.count
		FROM guild_bank_item gbi
		JOIN item_instance ii ON ii.guid = gbi.item_guid
		WHERE gbi.guildid = ? AND gbi.TabId = ?`, guildID, tabID)
	if err == nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var it guildBankSlotItem
			var slot, entry, count int64
			if scanErr := itemRows.Scan(&slot, &entry, &count); scanErr == nil {
				it.Slot = uint8(slot)
				it.Entry = uint32(entry)
				it.Count = int32(count)
				items = append(items, it)
			}
		}
	}

	buf := protocol.NewBuffer(256 + len(tabs)*64 + len(items)*32)
	buf.WriteU64(uint64(bankMoney))
	buf.WriteU8(tabID)
	buf.WriteI32(1000000) // WithdrawalsRemaining
	if fullUpdate {
		buf.WriteU8(1)
	} else {
		buf.WriteU8(0)
	}

	if tabID == 0 && fullUpdate {
		buf.WriteU8(uint8(len(tabs)))
		for _, tab := range tabs {
			buf.WriteCString(tab.Name)
			buf.WriteCString(tab.Icon)
		}
	}

	buf.WriteU8(uint8(len(items)))
	for _, it := range items {
		buf.WriteU8(it.Slot)
		buf.WriteU32(it.Entry)
		if it.Entry != 0 {
			buf.WriteI32(0) // Flags
			buf.WriteI32(0) // RandomPropertiesID
			buf.WriteI32(it.Count)
			buf.WriteI32(0) // EnchantmentID
			buf.WriteU8(0)  // Charges
			buf.WriteU8(0)  // SocketEnchant count
		}
	}

	return s.write(uint16(protocol.OpcodeSMSG_GUILD_BANK_LIST), buf.Bytes(), true) == nil
}

// handleGuildBankSwapItems processes CMSG_GUILD_BANK_SWAP_ITEMS (0x3E9).
func (s *session) handleGuildBankSwapItems(ctx context.Context, payload []byte) bool {
	return true
}

// handleGuildBankBuyTab processes CMSG_GUILD_BANK_BUY_TAB (0x3EA).
// Reference: WorldSession::HandleGuildBankBuyTab (GuildHandler.cpp:340).
func (s *session) handleGuildBankBuyTab(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	tabID, err := r.ReadU8()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID)
	if err != nil || guildID == 0 {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "INSERT OR IGNORE INTO guild_bank_tab (guildid, TabId, TabName, TabIcon, TabText) VALUES (?, ?, ?, 'INV_Misc_Bag_08', '')",
		guildID, tabID, "Tab "+string(rune('1'+tabID)))

	return s.sendGuildBankList(ctx, bankerGUID, tabID, true)
}

// handleGuildBankUpdateTab processes CMSG_GUILD_BANK_UPDATE_TAB (0x3EB).
// Reference: WorldSession::HandleGuildBankUpdateTab (GuildHandler.cpp:349).
func (s *session) handleGuildBankUpdateTab(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 10 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	tabID, err := r.ReadU8()
	if err != nil {
		return false
	}
	name, err := r.ReadCString()
	if err != nil {
		return false
	}
	icon, err := r.ReadCString()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID)
	if err != nil || guildID == 0 {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "UPDATE guild_bank_tab SET TabName = ?, TabIcon = ? WHERE guildid = ? AND TabId = ?", name, icon, guildID, tabID)

	return s.sendGuildBankList(ctx, bankerGUID, tabID, true)
}

// handleGuildBankDepositMoney processes CMSG_GUILD_BANK_DEPOSIT_MONEY (0x3EC).
// Reference: WorldSession::HandleGuildBankDepositMoney (GuildHandler.cpp:284).
func (s *session) handleGuildBankDepositMoney(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	amount, err := r.ReadU32()
	if err != nil || amount == 0 || s.player.Money < amount {
		return true
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID)
	if err != nil || guildID == 0 {
		return true
	}

	s.player.Money -= amount
	s.sendPlayerMoneyUpdate()
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET BankMoney = BankMoney + ? WHERE guildid = ?", amount, guildID)

	return s.sendGuildBankList(ctx, bankerGUID, 0, false)
}

// handleGuildBankWithdrawMoney processes CMSG_GUILD_BANK_WITHDRAW_MONEY (0x3ED).
// Reference: WorldSession::HandleGuildBankWithdrawMoney (GuildHandler.cpp:295).
func (s *session) handleGuildBankWithdrawMoney(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	amount, err := r.ReadU32()
	if err != nil || amount == 0 {
		return true
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID, bankMoney int64
	err = cdb.QueryRowContext(ctx, `SELECT g.guildid, g.BankMoney FROM guild_member gm
		JOIN guild g ON g.guildid = gm.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &bankMoney)
	if err != nil || guildID == 0 || uint64(bankMoney) < uint64(amount) {
		return true
	}

	s.player.Money += amount
	s.sendPlayerMoneyUpdate()
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET BankMoney = BankMoney - ? WHERE guildid = ?", amount, guildID)

	return s.sendGuildBankList(ctx, bankerGUID, 0, false)
}

// handleGuildBankLogQuery processes MSG_GUILD_BANK_LOG_QUERY (0x3EE).
// Reference: WorldSession::HandleGuildBankLogQuery (GuildHandler.cpp:360).
func (s *session) handleGuildBankLogQuery(ctx context.Context, payload []byte) bool {
	tabID := uint8(0)
	if len(payload) >= 1 {
		r := protocol.NewReader(payload)
		tabID, _ = r.ReadU8()
	}

	buf := protocol.NewBuffer(8)
	buf.WriteU8(tabID)
	buf.WriteU8(0) // 0 entries
	_ = s.write(uint16(protocol.OpcodeMSG_GUILD_BANK_LOG_QUERY), buf.Bytes(), true)
	return true
}

// handleGuildBankMoneyWithdrawn processes MSG_GUILD_BANK_MONEY_WITHDRAWN (0x3FE).
// Reference: WorldSession::HandleGuildBankMoneyWithdrawn (GuildHandler.cpp:238).
func (s *session) handleGuildBankMoneyWithdrawn(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(8)
	buf.WriteI32(1000000) // remaining withdraw money
	_ = s.write(uint16(protocol.OpcodeMSG_GUILD_BANK_MONEY_WITHDRAWN), buf.Bytes(), true)
	return true
}

// handleQueryGuildBankText processes MSG_QUERY_GUILD_BANK_TEXT (0x40A).
// Reference: WorldSession::HandleGuildBankTextQuery (GuildHandler.cpp:368).
func (s *session) handleQueryGuildBankText(ctx context.Context, payload []byte) bool {
	if len(payload) < 1 {
		return true
	}
	r := protocol.NewReader(payload)
	tabID, err := r.ReadU8()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	tabText := ""
	if cdb != nil {
		var guildID int64
		if err := cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID); err == nil && guildID > 0 {
			var text sql.NullString
			_ = cdb.QueryRowContext(ctx, "SELECT TabText FROM guild_bank_tab WHERE guildid = ? AND TabId = ? LIMIT 1", guildID, tabID).Scan(&text)
			if text.Valid {
				tabText = text.String
			}
		}
	}

	buf := protocol.NewBuffer(64 + len(tabText))
	buf.WriteU8(tabID)
	buf.WriteCString(tabText)
	_ = s.write(uint16(protocol.OpcodeMSG_QUERY_GUILD_BANK_TEXT), buf.Bytes(), true)
	return true
}

// handleSetGuildBankText processes CMSG_SET_GUILD_BANK_TEXT (0x40B).
// Reference: WorldSession::HandleGuildBankSetTabText (GuildHandler.cpp:376).
func (s *session) handleSetGuildBankText(ctx context.Context, payload []byte) bool {
	if len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	tabID, err := r.ReadU8()
	if err != nil {
		return false
	}
	tabText, err := r.ReadCString()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		var guildID int64
		if err := cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID); err == nil && guildID > 0 {
			_, _ = cdb.ExecContext(ctx, "UPDATE guild_bank_tab SET TabText = ? WHERE guildid = ? AND TabId = ?", tabText, guildID, tabID)
		}
	}
	return true
}

