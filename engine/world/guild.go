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
