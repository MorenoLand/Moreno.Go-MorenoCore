package world

import (
	"context"
	"database/sql"

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
	targetSess.guildInviterGUID = s.playerGUID

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
	s.guildInviterGUID = 0
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
	if s.guildInviterGUID != 0 && s.server != nil && s.player != nil {
		inviterSess := s.server.findSessionByGUID(s.guildInviterGUID)
		if inviterSess != nil && inviterSess.playerLoaded {
			eventBuf := protocol.NewBuffer(64)
			eventBuf.WriteU8(2) // GE_DECLINED
			eventBuf.WriteU8(1)
			eventBuf.WriteCString(s.player.Name)
			_ = inviterSess.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)
		}
	}
	s.guildInvitedID = 0
	s.guildInviterGUID = 0
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

// handleGuildCreate processes CMSG_GUILD_CREATE (0x081).
// Reference: WorldSession::HandleGuildCreateOpcode (GuildHandler.cpp:38).
func (s *session) handleGuildCreate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	guildName, err := r.ReadCString()
	if err != nil || guildName == "" {
		return true
	}
	s.debug("guild create rejected", "account", s.accountName, "name", guildName)
	return true
}

// handleGuildInfo processes CMSG_GUILD_INFO (0x087).
// Reference: WorldSession::HandleGuildInfoOpcode (GuildHandler.cpp:77).
func (s *session) handleGuildInfo(ctx context.Context) bool {
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
		return true
	}

	var name string
	var createdDate int64
	err = cdb.QueryRowContext(ctx, "SELECT name, createdate FROM guild WHERE guildid = ? LIMIT 1", guildID).Scan(&name, &createdDate)
	if err != nil {
		return true
	}

	var memberCount, accountCount int64
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(DISTINCT c.account) FROM guild_member gm JOIN characters c ON c.guid = gm.guid WHERE gm.guildid = ?", guildID).Scan(&memberCount, &accountCount)

	buf := protocol.NewBuffer(128)
	buf.WriteCString(name)
	buf.WriteU32(uint32(createdDate))
	buf.WriteI32(int32(memberCount))
	buf.WriteI32(int32(accountCount))
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_INFO), buf.Bytes(), true)
	s.debug("guild info sent", "guild", name, "members", memberCount)
	return true
}

// handleGuildPromote processes CMSG_GUILD_PROMOTE (0x08B).
// Reference: WorldSession::HandleGuildPromoteOpcode (GuildHandler.cpp:95).
func (s *session) handleGuildPromote(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	targetName, err := r.ReadCString()
	if err != nil || targetName == "" {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, myRank int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid, rank FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID, &myRank)
	if err != nil || guildID == 0 {
		return true
	}

	var targetGUID, targetRank int64
	err = cdb.QueryRowContext(ctx, `SELECT gm.guid, gm.rank FROM guild_member gm
		JOIN characters c ON c.guid = gm.guid
		WHERE gm.guildid = ? AND UPPER(c.name) = UPPER(?) LIMIT 1`, guildID, targetName).Scan(&targetGUID, &targetRank)
	if err != nil || targetGUID == 0 {
		return true
	}

	if targetRank <= myRank+1 || targetRank <= 1 {
		return true
	}

	newRank := targetRank - 1
	_, _ = cdb.ExecContext(ctx, "UPDATE guild_member SET rank = ? WHERE guid = ? AND guildid = ?", newRank, targetGUID, guildID)

	eventBuf := protocol.NewBuffer(128)
	eventBuf.WriteU8(0) // GE_PROMOTION
	eventBuf.WriteU8(2)
	eventBuf.WriteCString(s.player.Name)
	eventBuf.WriteCString(targetName)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)

	return s.handleGuildRoster(ctx)
}

// handleGuildDemote processes CMSG_GUILD_DEMOTE (0x08C).
// Reference: WorldSession::HandleGuildDemoteOpcode (GuildHandler.cpp:104).
func (s *session) handleGuildDemote(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	targetName, err := r.ReadCString()
	if err != nil || targetName == "" {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, myRank int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid, rank FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID, &myRank)
	if err != nil || guildID == 0 {
		return true
	}

	var targetGUID, targetRank int64
	err = cdb.QueryRowContext(ctx, `SELECT gm.guid, gm.rank FROM guild_member gm
		JOIN characters c ON c.guid = gm.guid
		WHERE gm.guildid = ? AND UPPER(c.name) = UPPER(?) LIMIT 1`, guildID, targetName).Scan(&targetGUID, &targetRank)
	if err != nil || targetGUID == 0 {
		return true
	}

	var maxRank int64 = 4
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(rid), 4) FROM guild_rank WHERE guildid = ?", guildID).Scan(&maxRank)

	if targetRank <= myRank || targetRank >= maxRank {
		return true
	}

	newRank := targetRank + 1
	_, _ = cdb.ExecContext(ctx, "UPDATE guild_member SET rank = ? WHERE guid = ? AND guildid = ?", newRank, targetGUID, guildID)

	eventBuf := protocol.NewBuffer(128)
	eventBuf.WriteU8(1) // GE_DEMOTION
	eventBuf.WriteU8(2)
	eventBuf.WriteCString(s.player.Name)
	eventBuf.WriteCString(targetName)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)

	return s.handleGuildRoster(ctx)
}

// handleGuildLeader processes CMSG_GUILD_LEADER (0x090).
// Reference: WorldSession::HandleGuildSetGuildMaster (GuildHandler.cpp:129).
func (s *session) handleGuildLeader(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	newMasterName, err := r.ReadCString()
	if err != nil || newMasterName == "" {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err = cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	var newLeaderGUID int64
	err = cdb.QueryRowContext(ctx, `SELECT gm.guid FROM guild_member gm
		JOIN characters c ON c.guid = gm.guid
		WHERE gm.guildid = ? AND UPPER(c.name) = UPPER(?) LIMIT 1`, guildID, newMasterName).Scan(&newLeaderGUID)
	if err != nil || newLeaderGUID == 0 || uint64(newLeaderGUID) == s.playerGUID {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET leaderguid = ? WHERE guildid = ?", newLeaderGUID, guildID)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild_member SET rank = 1 WHERE guid = ? AND guildid = ?", s.playerGUID, guildID)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild_member SET rank = 0 WHERE guid = ? AND guildid = ?", newLeaderGUID, guildID)

	eventBuf := protocol.NewBuffer(128)
	eventBuf.WriteU8(7) // GE_LEADER_CHANGED
	eventBuf.WriteU8(2)
	eventBuf.WriteCString(s.player.Name)
	eventBuf.WriteCString(newMasterName)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)

	return s.handleGuildRoster(ctx)
}

// handleGuildRemove processes CMSG_GUILD_REMOVE (0x08E).
// Reference: WorldSession::HandleGuildRemoveOpcode (GuildHandler.cpp:51).
func (s *session) handleGuildRemove(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	removee, err := r.ReadCString()
	if err != nil || removee == "" {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, myRank int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid, rank FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID, &myRank)
	if err != nil || guildID == 0 {
		return true
	}

	var targetGUID, targetRank int64
	err = cdb.QueryRowContext(ctx, `SELECT gm.guid, gm.rank FROM guild_member gm
		JOIN characters c ON c.guid = gm.guid
		WHERE gm.guildid = ? AND UPPER(c.name) = UPPER(?) LIMIT 1`, guildID, removee).Scan(&targetGUID, &targetRank)
	if err != nil || targetGUID == 0 {
		return true
	}

	if targetRank <= myRank {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_member WHERE guid = ? AND guildid = ?", targetGUID, guildID)

	eventBuf := protocol.NewBuffer(128)
	eventBuf.WriteU8(5) // GE_REMOVED
	eventBuf.WriteU8(2)
	eventBuf.WriteCString(removee)
	eventBuf.WriteCString(s.player.Name)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)

	return s.handleGuildRoster(ctx)
}

// handleGuildDisband processes CMSG_GUILD_DISBAND (0x08F).
// Reference: WorldSession::HandleGuildDelete (GuildHandler.cpp:121).
func (s *session) handleGuildDisband(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err := cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild WHERE guildid = ?", guildID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_member WHERE guildid = ?", guildID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_rank WHERE guildid = ?", guildID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_bank_tab WHERE guildid = ?", guildID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_bank_item WHERE guildid = ?", guildID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_bank_eventlog WHERE guildid = ?", guildID)

	eventBuf := protocol.NewBuffer(32)
	eventBuf.WriteU8(8) // GE_DISBANDED
	eventBuf.WriteU8(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)
	s.debug("guild disbanded", "guild_id", guildID)
	return true
}

// handleGuildAddRank processes CMSG_GUILD_ADD_RANK (0x232).
// Reference: WorldSession::HandleGuildAddRankOpcode (GuildHandler.cpp:181).
func (s *session) handleGuildAddRank(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	rankName, err := r.ReadCString()
	if err != nil || rankName == "" {
		return false
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err = cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	var maxRid sql.NullInt64
	_ = cdb.QueryRowContext(ctx, "SELECT MAX(rid) FROM guild_rank WHERE guildid = ?", guildID).Scan(&maxRid)
	newRid := 0
	if maxRid.Valid {
		newRid = int(maxRid.Int64) + 1
	}
	if newRid > 9 {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, ?, ?, 0x00000040, 0)", guildID, newRid, rankName)
	return s.handleGuildRoster(ctx)
}

// handleGuildDelRank processes CMSG_GUILD_DEL_RANK (0x233).
// Reference: WorldSession::HandleGuildDeleteRank (GuildHandler.cpp:189).
func (s *session) handleGuildDelRank(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err := cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	var maxRid sql.NullInt64
	_ = cdb.QueryRowContext(ctx, "SELECT MAX(rid) FROM guild_rank WHERE guildid = ?", guildID).Scan(&maxRid)
	if !maxRid.Valid || maxRid.Int64 <= 1 {
		return true
	}

	lowestRank := maxRid.Int64
	_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_rank WHERE guildid = ? AND rid = ?", guildID, lowestRank)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild_member SET rank = ? WHERE guildid = ? AND rank = ?", lowestRank-1, guildID, lowestRank)

	return s.handleGuildRoster(ctx)
}

// handleGuildRank processes CMSG_GUILD_RANK (0x231).
// Reference: WorldSession::HandleGuildSetRankPermissions (GuildHandler.cpp:166).
func (s *session) handleGuildRank(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	rankID, err := r.ReadU32()
	if err != nil {
		return false
	}
	rights, err := r.ReadU32()
	if err != nil {
		return false
	}
	rankName, err := r.ReadCString()
	if err != nil {
		return false
	}
	goldLimit, _ := r.ReadU32()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err = cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "UPDATE guild_rank SET rname = ?, rights = ?, BankMoneyPerDay = ? WHERE guildid = ? AND rid = ?", rankName, rights, goldLimit, guildID, rankID)
	return s.handleGuildRoster(ctx)
}

// handleGuildSetPublicNote processes CMSG_GUILD_SET_PUBLIC_NOTE (0x234).
// Reference: WorldSession::HandleGuildSetPublicNoteOpcode (GuildHandler.cpp:146).
func (s *session) handleGuildSetPublicNote(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	targetName, err := r.ReadCString()
	if err != nil || targetName == "" {
		return false
	}
	note, _ := r.ReadCString()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID)
	if err != nil || guildID == 0 {
		return true
	}

	_, _ = cdb.ExecContext(ctx, `UPDATE guild_member SET pnote = ?
		WHERE guildid = ? AND guid = (SELECT guid FROM characters WHERE UPPER(name) = UPPER(?) LIMIT 1)`, note, guildID, targetName)

	return s.handleGuildRoster(ctx)
}

// handleGuildSetOfficerNote processes CMSG_GUILD_SET_OFFICER_NOTE (0x235).
// Reference: WorldSession::HandleGuildSetOfficerNoteOpcode (GuildHandler.cpp:156).
func (s *session) handleGuildSetOfficerNote(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 2 {
		return true
	}
	r := protocol.NewReader(payload)
	targetName, err := r.ReadCString()
	if err != nil || targetName == "" {
		return false
	}
	note, _ := r.ReadCString()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID int64
	err = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&guildID)
	if err != nil || guildID == 0 {
		return true
	}

	_, _ = cdb.ExecContext(ctx, `UPDATE guild_member SET offnote = ?
		WHERE guildid = ? AND guid = (SELECT guid FROM characters WHERE UPPER(name) = UPPER(?) LIMIT 1)`, note, guildID, targetName)

	return s.handleGuildRoster(ctx)
}

// handleGuildInfoText processes CMSG_GUILD_INFO_TEXT (0x2FC).
// Reference: WorldSession::HandleGuildUpdateInfoText (GuildHandler.cpp:197).
func (s *session) handleGuildInfoText(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	r := protocol.NewReader(payload)
	infoText, err := r.ReadCString()
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

	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET info = ? WHERE guildid = ?", infoText, guildID)
	return true
}

func (s *Server) getGuildName(ctx context.Context, guildID uint32) string {
	if guildID == 0 || s.CharactersStore == nil || s.CharactersStore.DB == nil {
		return ""
	}
	var name string
	_ = s.CharactersStore.DB.QueryRowContext(ctx, "SELECT name FROM guild WHERE guildid = ? LIMIT 1", guildID).Scan(&name)
	return name
}

// handleGuildEventLogQuery processes MSG_GUILD_EVENT_LOG_QUERY (0x3FF).
// Reference: WorldSession::HandleGuildEventLogQueryOpcode (GuildHandler.cpp:111).
func (s *session) handleGuildEventLogQuery(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(1)
	buf.WriteU8(0) // count = 0 entries
	_ = s.write(uint16(protocol.OpcodeMSG_GUILD_EVENT_LOG_QUERY), buf.Bytes(), true)
	return true
}

// handleGuildPermissions processes MSG_GUILD_PERMISSIONS (0x3FD).
// Reference: WorldSession::HandleGuildPermissions (GuildHandler.cpp:89).
func (s *session) handleGuildPermissions(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	var rankID, rights, goldLimit int64
	rankID = 0
	rights = 0xFFFFFFFF
	goldLimit = 1000000

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		_ = cdb.QueryRowContext(ctx, `SELECT gr.rid, gr.rights, gr.BankMoneyPerDay
			FROM guild_member gm
			JOIN guild_rank gr ON gr.guildid = gm.guildid AND gr.rid = gm.rank
			WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&rankID, &rights, &goldLimit)
	}

	buf := protocol.NewBuffer(16 + 6*8)
	buf.WriteU32(uint32(rankID))
	buf.WriteU32(uint32(rights))
	buf.WriteU32(uint32(goldLimit))
	buf.WriteU8(6) // 6 tabs
	for i := 0; i < 6; i++ {
		buf.WriteU32(0xFFFFFFFF) // full rights
		buf.WriteU32(1000)       // slot limit
	}
	_ = s.write(uint16(protocol.OpcodeMSG_GUILD_PERMISSIONS), buf.Bytes(), true)
	return true
}

// handleInspectArenaTeams processes MSG_INSPECT_ARENA_TEAMS (0x377).
// Reference: WorldSession::HandleInspectArenaTeamsOpcode (ArenaTeamHandler.cpp:333).
func (s *session) handleInspectArenaTeams(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	targetGUID, _ := r.ReadU64()

	buf := protocol.NewBuffer(8 + 3*24)
	buf.WriteU64(targetGUID)
	for slot := 0; slot < 3; slot++ {
		buf.WriteU32(0) // arenaTeamID
		buf.WriteU32(0) // rating
		buf.WriteU32(0) // seasonGames
		buf.WriteU32(0) // seasonWins
		buf.WriteU32(0) // played
		buf.WriteU32(0) // personalRating
	}
	_ = s.write(uint16(protocol.OpcodeMSG_INSPECT_ARENA_TEAMS), buf.Bytes(), true)
	return true
}

// handleInspectHonorStats processes MSG_INSPECT_HONOR_STATS (0x2D6).
// Reference: WorldSession::HandleInspectHonorStatsOpcode (MiscHandler.cpp:749).
func (s *session) handleInspectHonorStats(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	targetGUID, _ := r.ReadU64()

	buf := protocol.NewBuffer(8 + 6*4)
	buf.WriteU64(targetGUID)
	buf.WriteU32(0) // honorPoints
	buf.WriteU32(0) // killsToday
	buf.WriteU32(0) // killsYesterday
	buf.WriteU32(0) // lifetimeHK
	buf.WriteU32(0) // honorToday
	buf.WriteU32(0) // honorYesterday
	_ = s.write(uint16(protocol.OpcodeMSG_INSPECT_HONOR_STATS), buf.Bytes(), true)
	return true
}

// handlePvpLogData processes MSG_PVP_LOG_DATA (0x2E0).
// Reference: WorldSession::HandlePVPLogDataOpcode (BattlegroundHandler.cpp:211).
func (s *session) handlePvpLogData(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(16)
	buf.WriteU8(0)  // arena (0)
	buf.WriteU32(0) // count (0)
	buf.WriteU8(0)  // winner
	_ = s.write(uint16(protocol.OpcodeMSG_PVP_LOG_DATA), buf.Bytes(), true)
	return true
}

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
		resBuf.WriteI32(1) // ERR_GUILD_PLAYER_NOT_IN_GUILD
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
// Reference: WorldSession::HandleGuildBankSwapItems (GuildHandler.cpp:320).
func (s *session) handleGuildBankSwapItems(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	bankerGUID, _ := r.ReadU64()
	bankOnly, _ := r.ReadU8()

	guildID := s.player.GuildID
	if guildID == 0 || s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB

	var bankTab uint8
	if bankOnly != 0 {
		bankTab, _ = r.ReadU8()
		bankSlot, _ := r.ReadU8()
		_, _ = r.ReadU32() // itemID
		bankTab1, _ := r.ReadU8()
		bankSlot1, _ := r.ReadU8()
		_, _ = r.ReadU32() // itemID1

		var item1, item2 uint64
		_ = cdb.QueryRowContext(ctx, "SELECT item_guid FROM guild_bank_item WHERE guildid = ? AND TabId = ? AND SlotId = ?", guildID, bankTab, bankSlot).Scan(&item1)
		_ = cdb.QueryRowContext(ctx, "SELECT item_guid FROM guild_bank_item WHERE guildid = ? AND TabId = ? AND SlotId = ?", guildID, bankTab1, bankSlot1).Scan(&item2)

		_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_bank_item WHERE guildid = ? AND ((TabId = ? AND SlotId = ?) OR (TabId = ? AND SlotId = ?))", guildID, bankTab, bankSlot, bankTab1, bankSlot1)
		if item1 != 0 {
			_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_bank_item (guildid, TabId, SlotId, item_guid) VALUES (?, ?, ?, ?)", guildID, bankTab1, bankSlot1, item1)
		}
		if item2 != 0 {
			_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_bank_item (guildid, TabId, SlotId, item_guid) VALUES (?, ?, ?, ?)", guildID, bankTab, bankSlot, item2)
		}
	} else {
		bankTab, _ = r.ReadU8()
		bankSlot, _ := r.ReadU8()
		_, _ = r.ReadU32() // itemID
		autoStore, _ := r.ReadU8()

		var containerSlot, containerItemSlot, toSlot uint8
		if autoStore != 0 {
			_, _ = r.ReadU32() // bankItemCount
			toSlot, _ = r.ReadU8()
			_, _ = r.ReadU32() // stackCount
			containerSlot = 0
			containerItemSlot = 23
		} else {
			containerSlot, _ = r.ReadU8()
			containerItemSlot, _ = r.ReadU8()
			toSlot, _ = r.ReadU8()
			_, _ = r.ReadU32() // stackCount
		}

		if toSlot != 0 {
			// Bank -> Player Inventory (Withdraw)
			var bankItemGUID uint64
			err := cdb.QueryRowContext(ctx, "SELECT item_guid FROM guild_bank_item WHERE guildid = ? AND TabId = ? AND SlotId = ?", guildID, bankTab, bankSlot).Scan(&bankItemGUID)
			if err == nil && bankItemGUID > 0 {
				_, _ = cdb.ExecContext(ctx, "DELETE FROM guild_bank_item WHERE guildid = ? AND TabId = ? AND SlotId = ?", guildID, bankTab, bankSlot)
				_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_inventory (guid, bag, slot, item) VALUES (?, ?, ?, ?)", s.playerGUID, containerSlot, containerItemSlot, bankItemGUID)
				_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET owner_guid = ? WHERE guid = ?", s.playerGUID, bankItemGUID)
				_ = s.sendInventoryItems(ctx)
				s.sendPlayerUpdate()
			}
		} else {
			// Player Inventory -> Bank (Deposit)
			var invItemGUID uint64
			err := cdb.QueryRowContext(ctx, "SELECT item FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ?", s.playerGUID, containerSlot, containerItemSlot).Scan(&invItemGUID)
			if err == nil && invItemGUID > 0 {
				_, _ = cdb.ExecContext(ctx, "DELETE FROM character_inventory WHERE guid = ? AND bag = ? AND slot = ?", s.playerGUID, containerSlot, containerItemSlot)
				_, _ = cdb.ExecContext(ctx, "REPLACE INTO guild_bank_item (guildid, TabId, SlotId, item_guid) VALUES (?, ?, ?, ?)", guildID, bankTab, bankSlot, invItemGUID)
				_, _ = cdb.ExecContext(ctx, "UPDATE item_instance SET owner_guid = 0 WHERE guid = ?", invItemGUID)
				_ = s.sendInventoryItems(ctx)
				s.sendPlayerUpdate()
			}
		}
	}
	s.sendGuildBankList(ctx, bankerGUID, bankTab, false)
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

const (
	guildCharterItemID = 5863
	guildCharterCost   = 1000 // 10s
)

// handlePetitionBuy processes CMSG_PETITION_BUY (0x1B6).
// Reference: WorldSession::HandlePetitionBuyOpcode (PetitionsHandler.cpp:48).
func (s *session) handlePetitionBuy(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadU64() // npc GUID
	_, _ = r.ReadU32() // 0
	_, _ = r.ReadU64() // 0
	name, err := r.ReadCString()
	if err != nil || name == "" {
		return true
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	// Check if player already in guild
	var existingGuild int64
	_ = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&existingGuild)
	if existingGuild != 0 {
		return true
	}

	petitionGUID := s.playerGUID + 100000
	_, _ = cdb.ExecContext(ctx, "REPLACE INTO petition (ownerguid, petitionguid, name, type) VALUES (?, ?, ?, 9)",
		s.playerGUID, petitionGUID, name)
	return true
}

// handlePetitionShowSignatures processes CMSG_PETITION_SHOW_SIGNATURES (0x1BE).
// Reference: WorldSession::HandlePetitionShowSignatures (PetitionsHandler.cpp:220).
func (s *session) handlePetitionShowSignatures(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	_ = cdb.QueryRowContext(ctx, "SELECT ownerguid FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID)

	rows, err := cdb.QueryContext(ctx, "SELECT playerguid FROM petition_sign WHERE petitionguid = ?", petitionGUID)
	var signs []uint64
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pguid int64
			if err := rows.Scan(&pguid); err == nil {
				signs = append(signs, uint64(pguid))
			}
		}
	}

	buf := protocol.NewBuffer(32 + len(signs)*12)
	buf.WriteU64(petitionGUID)
	buf.WriteU64(uint64(ownerGUID))
	buf.WriteU32(uint32(petitionGUID & 0xFFFFFFFF))
	buf.WriteU8(uint8(len(signs)))
	for _, sg := range signs {
		buf.WriteU64(sg)
		buf.WriteU32(0)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_SHOW_SIGNATURES), buf.Bytes(), true)
	return true
}

// handlePetitionQuery processes CMSG_PETITION_QUERY (0x1C6).
// Reference: WorldSession::HandleQueryPetition (PetitionsHandler.cpp:261).
func (s *session) handlePetitionQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	guildGUID, _ := r.ReadU32()
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	var name string
	var pType int64
	err = cdb.QueryRowContext(ctx, "SELECT ownerguid, name, type FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID, &name, &pType)
	if err != nil {
		return true
	}

	buf := protocol.NewBuffer(128 + len(name))
	buf.WriteU32(guildGUID)
	buf.WriteU64(uint64(ownerGUID))
	buf.WriteCString(name)
	buf.WriteU8(0)
	buf.WriteU32(9) // 9 signs needed for guild
	buf.WriteU32(9)
	buf.WriteU32(0)
	for i := 0; i < 10; i++ {
		buf.WriteCString("")
	}
	buf.WriteU32(uint32(pType))
	buf.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_QUERY_RESPONSE), buf.Bytes(), true)
	return true
}

// handlePetitionSign processes CMSG_PETITION_SIGN (0x1C0).
// Reference: WorldSession::HandlePetitionSignOpcode (PetitionsHandler.cpp:328).
func (s *session) handlePetitionSign(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	var pType int64
	err = cdb.QueryRowContext(ctx, "SELECT ownerguid, type FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID, &pType)
	if err != nil {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "INSERT OR IGNORE INTO petition_sign (ownerguid, petitionguid, playerguid, player_account, type) VALUES (?, ?, ?, ?, ?)",
		ownerGUID, petitionGUID, s.playerGUID, s.accountID, pType)

	buf := protocol.NewBuffer(20)
	buf.WriteU64(petitionGUID)
	buf.WriteU64(s.playerGUID)
	buf.WriteU32(0) // success
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_SIGN_RESULTS), buf.Bytes(), true)
	return true
}

// handleTurnInPetition processes CMSG_TURN_IN_PETITION (0x1C4).
// Reference: WorldSession::HandleTurnInPetitionOpcode (PetitionsHandler.cpp:557).
func (s *session) handleTurnInPetition(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	var guildName string
	err = cdb.QueryRowContext(ctx, "SELECT ownerguid, name FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID, &guildName)
	if err != nil || ownerGUID == 0 || guildName == "" {
		return true
	}

	var maxGuildID sql.NullInt64
	_ = cdb.QueryRowContext(ctx, "SELECT MAX(guildid) FROM guild").Scan(&maxGuildID)
	newGuildID := int64(1)
	if maxGuildID.Valid {
		newGuildID = maxGuildID.Int64 + 1
	}

	// Create guild
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild (guildid, name, leaderguid, createdate) VALUES (?, ?, ?, ?)",
		newGuildID, guildName, ownerGUID, timeNow())

	// Default ranks
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 0, 'Guild Master', 4294967295, 1000000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 1, 'Officer', 255, 500000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 2, 'Veteran', 64, 100000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 3, 'Member', 64, 50000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 4, 'Initiate', 64, 0)", newGuildID)

	// Add leader
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_member (guildid, guid, rank, pnote, offnote) VALUES (?, ?, 0, '', '')", newGuildID, ownerGUID)

	// Add signers
	var signers []int64
	rows, _ := cdb.QueryContext(ctx, "SELECT playerguid FROM petition_sign WHERE petitionguid = ?", petitionGUID)
	if rows != nil {
		for rows.Next() {
			var signerGUID int64
			if err := rows.Scan(&signerGUID); err == nil && signerGUID != ownerGUID {
				signers = append(signers, signerGUID)
			}
		}
		rows.Close()
	}
	for _, signerGUID := range signers {
		_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_member (guildid, guid, rank, pnote, offnote) VALUES (?, ?, 4, '', '')", newGuildID, signerGUID)
	}

	// Clean up petition
	_, _ = cdb.ExecContext(ctx, "DELETE FROM petition WHERE petitionguid = ?", petitionGUID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM petition_sign WHERE petitionguid = ?", petitionGUID)

	buf := protocol.NewBuffer(4)
	buf.WriteU32(0) // success
	_ = s.write(uint16(protocol.OpcodeSMSG_TURN_IN_PETITION_RESULTS), buf.Bytes(), true)
	return true
}

// handleOfferPetition processes CMSG_OFFER_PETITION (0x1B3).
// Reference: WorldSession::HandleOfferPetitionOpcode (PetitionsHandler.cpp:514).
func (s *session) handleOfferPetition(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 20 {
		return true
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadU32() // junk
	petitionGUID, _ := r.ReadU64()
	targetGUID, _ := r.ReadU64()

	if s.server != nil {
		targetSess := s.server.findSessionByGUID(targetGUID)
		if targetSess != nil && targetSess.playerLoaded {
			buf := protocol.NewBuffer(24)
			buf.WriteU64(petitionGUID)
			buf.WriteU64(s.playerGUID)
			_ = targetSess.write(uint16(protocol.OpcodeSMSG_PETITION_SHOW_SIGNATURES), buf.Bytes(), true)
		}
	}
	return true
}

// handlePetitionShowList processes CMSG_PETITION_SHOWLIST (0x1BB).
// Reference: WorldSession::HandlePetitionShowListOpcode (PetitionsHandler.cpp:408).
func (s *session) handlePetitionShowList(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, _ := r.ReadU64()

	buf := protocol.NewBuffer(24)
	buf.WriteU64(npcGUID)
	buf.WriteU8(1)                   // count = 1 petition item
	buf.WriteU32(1)                  // index = 1
	buf.WriteU32(guildCharterItemID) // 5863 Guild Charter
	buf.WriteU32(CHARTER_DISPLAY_ID) // 16161
	buf.WriteU32(guildCharterCost)   // 1000 copper
	buf.WriteU32(0)                  // 0
	buf.WriteU32(9)                  // 9 required signs
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_SHOWLIST), buf.Bytes(), true)
	return true
}

// handlePetitionDecline processes MSG_PETITION_DECLINE (0x1C2).
// Reference: WorldSession::HandlePetitionDeclineOpcode (PetitionsHandler.cpp:520).
func (s *session) handlePetitionDecline(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(8)
	buf.WriteU64(s.playerGUID)
	_ = s.write(uint16(protocol.OpcodeMSG_PETITION_DECLINE), buf.Bytes(), true)
	return true
}

// handlePetitionRename processes MSG_PETITION_RENAME (0x1C1).
// Reference: WorldSession::HandlePetitionRenameOpcode (PetitionsHandler.cpp:738).
func (s *session) handlePetitionRename(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	newName, err := r.ReadCString()
	if err != nil || newName == "" {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "UPDATE petition SET name = ? WHERE petitionguid = ? AND ownerguid = ?", newName, petitionGUID, s.playerGUID)
	}

	buf := protocol.NewBuffer(16 + len(newName))
	buf.WriteU64(petitionGUID)
	buf.WriteCString(newName)
	_ = s.write(uint16(protocol.OpcodeMSG_PETITION_RENAME), buf.Bytes(), true)
	return true
}

// handleTabardVendorActivate processes MSG_TABARDVENDOR_ACTIVATE (0x1FA).
// Reference: WorldSession::HandleTabardVendorActivateOpcode (NPCHandler.cpp:665).
func (s *session) handleTabardVendorActivate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	vendorGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	buf := protocol.NewBuffer(8)
	buf.WriteU64(vendorGUID)
	_ = s.write(uint16(protocol.OpcodeMSG_TABARDVENDOR_ACTIVATE), buf.Bytes(), true)
	return true
}

// handleSaveGuildEmblem processes MSG_SAVE_GUILD_EMBLEM (0x1FB).
// Reference: WorldSession::HandleSaveGuildEmblemOpcode (GuildHandler.cpp:205).
func (s *session) handleSaveGuildEmblem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 28 {
		return true
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadU64() // vendor
	style, _ := r.ReadU32()
	color, _ := r.ReadU32()
	bStyle, _ := r.ReadU32()
	bColor, _ := r.ReadU32()
	bgColor, _ := r.ReadU32()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err := cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	const tabardCost = 100000 // 10 gold
	if s.player.Money < tabardCost {
		return true
	}

	s.player.Money -= tabardCost
	s.sendPlayerMoneyUpdate()
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET EmblemStyle = ?, EmblemColor = ?, BorderStyle = ?, BorderColor = ?, BackgroundColor = ? WHERE guildid = ?",
		style, color, bStyle, bColor, bgColor, guildID)

	res := protocol.NewBuffer(4)
	res.WriteU32(0) // ERR_GUILDEMBLEM_SUCCESS
	_ = s.write(uint16(protocol.OpcodeMSG_SAVE_GUILD_EMBLEM), res.Bytes(), true)

	// GE_TABARDCHANGE = 9
	eventBuf := protocol.NewBuffer(8)
	eventBuf.WriteU8(9)
	eventBuf.WriteU8(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)
	return true
}

const CHARTER_DISPLAY_ID = 16161
