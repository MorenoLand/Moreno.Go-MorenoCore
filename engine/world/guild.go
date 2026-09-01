package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type guildMemberInfo struct {
	GUID        uint64
	Name        string
	RankID      int32
	Level       uint8
	ClassID     uint8
	Gender      uint8
	AreaID      int32
	Note        string
	OfficerNote string
	Online      bool
}

type guildRankInfo struct {
	RankID    uint32
	Name      string
	Rights    uint32
	GoldLimit uint32
}

func (s *session) handleGuildQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	reader := protocol.NewReader(payload)
	guildID, err := reader.ReadU32()
	if err != nil || guildID == 0 {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var name string
	var emblemStyle, emblemColor, borderStyle, borderColor, bgColor int64
	err = cdb.QueryRowContext(ctx, "SELECT name, EmblemStyle, EmblemColor, BorderStyle, BorderColor, BackgroundColor FROM guild WHERE guildid = ? LIMIT 1", guildID).Scan(&name, &emblemStyle, &emblemColor, &borderStyle, &borderColor, &bgColor)
	if err != nil {
		return true
	}
	rankRows, err := cdb.QueryContext(ctx, "SELECT rid, rname FROM guild_rank WHERE guildid = ? ORDER BY rid", guildID)
	var ranks []string
	if err == nil {
		defer rankRows.Close()
		for rankRows.Next() {
			var rid int
			var rname string
			if scanErr := rankRows.Scan(&rid, &rname); scanErr == nil {
				ranks = append(ranks, rname)
			}
		}
	}
	if len(ranks) == 0 {
		ranks = []string{"Guild Master", "Officer", "Veteran", "Member", "Initiate"}
	}
	buf := protocol.NewBuffer(256)
	buf.WriteU32(guildID)
	buf.WriteCString(name)
	for _, r := range ranks {
		buf.WriteCString(r)
	}
	buf.WriteU32(uint32(emblemStyle))
	buf.WriteU32(uint32(emblemColor))
	buf.WriteU32(uint32(borderStyle))
	buf.WriteU32(uint32(borderColor))
	buf.WriteU32(uint32(bgColor))
	buf.WriteU32(uint32(len(ranks)))
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_QUERY_RESPONSE), buf.Bytes(), true)
	s.debug("guild query response sent", "guild_id", guildID, "name", name)
	return true
}

func (s *session) handleGuildRoster(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID int64
	err := cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID)
	if err != nil || guildID == 0 {
		// Not in guild
		resBuf := protocol.NewBuffer(12)
		resBuf.WriteI32(5) // GUILD_COMMAND_ROSTER
		resBuf.WriteCString("")
		resBuf.WriteI32(1) // ERR_GUILD_PLAYER_NOT_IN_GUILD
		_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_COMMAND_RESULT), resBuf.Bytes(), true)
		return true
	}
	var motd, info string
	_ = cdb.QueryRowContext(ctx, "SELECT motd, info FROM guild WHERE guildid = ? LIMIT 1", guildID).Scan(&motd, &info)

	var ranks []guildRankInfo
	rankRows, err := cdb.QueryContext(ctx, "SELECT rid, rname, rights, BankMoneyPerDay FROM guild_rank WHERE guildid = ? ORDER BY rid", guildID)
	if err == nil {
		defer rankRows.Close()
		for rankRows.Next() {
			var rid, rights, goldLimit int64
			var rname string
			if scanErr := rankRows.Scan(&rid, &rname, &rights, &goldLimit); scanErr == nil {
				ranks = append(ranks, guildRankInfo{
					RankID:    uint32(rid),
					Name:      rname,
					Rights:    uint32(rights),
					GoldLimit: uint32(goldLimit),
				})
			}
		}
	}
	if len(ranks) == 0 {
		ranks = []guildRankInfo{
			{RankID: 0, Name: "Guild Master", Rights: 0xFFFFFFFF, GoldLimit: 1000000},
			{RankID: 1, Name: "Officer", Rights: 0x000000FF, GoldLimit: 500000},
			{RankID: 2, Name: "Veteran", Rights: 0x00000040, GoldLimit: 100000},
			{RankID: 3, Name: "Member", Rights: 0x00000040, GoldLimit: 50000},
			{RankID: 4, Name: "Initiate", Rights: 0x00000040, GoldLimit: 0},
		}
	}

	var members []guildMemberInfo
	memRows, err := cdb.QueryContext(ctx, `SELECT gm.guid, c.name, gm.rank, c.level, c.class, c.gender, c.zone, gm.pnote, gm.offnote
		FROM guild_member AS gm
		JOIN characters AS c ON c.guid = gm.guid
		WHERE gm.guildid = ?`, guildID)
	if err == nil {
		defer memRows.Close()
		for memRows.Next() {
			var mGuid, rank, lvl, cls, gnd, zone int64
			var mName, pNote, offNote string
			if scanErr := memRows.Scan(&mGuid, &mName, &rank, &lvl, &cls, &gnd, &zone, &pNote, &offNote); scanErr == nil {
				online := s.server.findSessionByGUID(uint64(mGuid)) != nil
				members = append(members, guildMemberInfo{
					GUID:        uint64(mGuid),
					Name:        mName,
					RankID:      int32(rank),
					Level:       uint8(lvl),
					ClassID:     uint8(cls),
					Gender:      uint8(gnd),
					AreaID:      int32(zone),
					Note:        pNote,
					OfficerNote: offNote,
					Online:      online,
				})
			}
		}
	}

	buf := protocol.NewBuffer(512 + len(members)*64)
	buf.WriteU32(uint32(len(members)))
	buf.WriteCString(motd)
	buf.WriteCString(info)
	buf.WriteU32(uint32(len(ranks)))
	for _, r := range ranks {
		buf.WriteU32(r.Rights)
		buf.WriteU32(r.GoldLimit)
		for i := 0; i < 6; i++ {
			buf.WriteU32(0) // TabFlags
			buf.WriteU32(0) // TabWithdrawLimit
		}
	}
	for _, m := range members {
		buf.WriteU64(m.GUID)
		if m.Online {
			buf.WriteU8(1)
		} else {
			buf.WriteU8(0)
		}
		buf.WriteCString(m.Name)
		buf.WriteI32(m.RankID)
		buf.WriteU8(m.Level)
		buf.WriteU8(m.ClassID)
		buf.WriteU8(m.Gender)
		buf.WriteI32(m.AreaID)
		if !m.Online {
			buf.WriteF32(0) // LastSave
		}
		buf.WriteCString(m.Note)
		buf.WriteCString(m.OfficerNote)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_ROSTER), buf.Bytes(), true)
	return true
}

func (s *session) handleGuildInvite(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	reader := protocol.NewReader(payload)
	targetName, err := reader.ReadCString()
	if err != nil || targetName == "" {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID int64
	var guildName string
	err = cdb.QueryRowContext(ctx, `SELECT g.guildid, g.name FROM guild_member AS gm
		JOIN guild AS g ON g.guildid = gm.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &guildName)
	if err != nil || guildID == 0 {
		return true
	}
	var targetGUID int64
	err = cdb.QueryRowContext(ctx, "SELECT guid FROM characters WHERE UPPER(name) = UPPER(?) LIMIT 1", targetName).Scan(&targetGUID)
	if err != nil || targetGUID == 0 {
		return true
	}
	targetSess := s.server.findSessionByGUID(uint64(targetGUID))
	if targetSess == nil || targetSess.player == nil {
		return true
	}
	targetSess.guildInvitedID = uint32(guildID)

	invBuf := protocol.NewBuffer(128)
	invBuf.WriteCString(s.player.Name)
	invBuf.WriteCString(guildName)
	_ = targetSess.write(uint16(protocol.OpcodeSMSG_GUILD_INVITE), invBuf.Bytes(), true)
	s.debug("guild invite sent", "from", s.player.Name, "to", targetName, "guild", guildName)
	return true
}

func (s *session) handleGuildAccept(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil || s.guildInvitedID == 0 {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	guildID := s.guildInvitedID
	s.guildInvitedID = 0
	_, _ = cdb.ExecContext(ctx, "REPLACE INTO guild_member (guildid, guid, rank, pnote, offnote) VALUES (?, ?, 4, '', '')", guildID, s.playerGUID)

	// Broadcast join event
	eventBuf := protocol.NewBuffer(64)
	eventBuf.WriteU8(3) // GE_JOINED
	eventBuf.WriteU8(1) // Param count
	eventBuf.WriteCString(s.player.Name)
	eventBuf.WriteU64(s.playerGUID)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)

	return s.handleGuildRoster(ctx)
}

func (s *session) handleGuildDecline(ctx context.Context) bool {
	s.guildInvitedID = 0
	return true
}

func (s *session) handleGuildLeave(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_member WHERE guid = ?", s.playerGUID)
	eventBuf := protocol.NewBuffer(64)
	eventBuf.WriteU8(4) // GE_LEFT
	eventBuf.WriteU8(1)
	eventBuf.WriteCString(s.player.Name)
	eventBuf.WriteU64(s.playerGUID)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)
	return true
}

func (s *session) handleGuildMotd(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	reader := protocol.NewReader(payload)
	motd, err := reader.ReadCString()
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
	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET motd = ? WHERE guildid = ?", motd, guildID)

	eventBuf := protocol.NewBuffer(128)
	eventBuf.WriteU8(5) // GE_MOTD
	eventBuf.WriteU8(1)
	eventBuf.WriteCString(motd)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)
	return true
}

func (s *session) handleGuildBankQueryTab(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 10 {
		return true
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU64() // banker
	tab, _ := reader.ReadU8()
	fullUpdate, _ := reader.ReadU8()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}
	var guildID, bankMoney int64
	err := cdb.QueryRowContext(ctx, `SELECT g.guildid, g.BankMoney FROM guild_member AS gm
		JOIN guild AS g ON g.guildid = gm.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &bankMoney)
	if err != nil || guildID == 0 {
		return true
	}

	buf := protocol.NewBuffer(256)
	buf.WriteU64(uint64(bankMoney))
	buf.WriteU8(tab)
	buf.WriteI32(1000000) // WithdrawalsRemaining
	buf.WriteU8(fullUpdate)
	if tab == 0 && fullUpdate != 0 {
		// Tab info
		buf.WriteU8(1) // 1 tab
		buf.WriteCString("General")
		buf.WriteCString("INV_Misc_Bag_08")
	}
	buf.WriteU8(0) // 0 items in tab for now
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_BANK_LIST), buf.Bytes(), true)
	return true
}
