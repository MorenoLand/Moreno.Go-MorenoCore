package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleArenaTeamQuery processes CMSG_ARENA_TEAM_QUERY (0x34B).
// Reference: WorldSession::HandleArenaTeamQueryOpcode (ArenaTeamHandler.cpp:61).
func (s *session) handleArenaTeamQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var name string
	var aType, bgCol, embStyle, embCol, bordStyle, bordCol uint32
	var rating, weekGames, weekWins, seasonGames, seasonWins, rank uint32
	err = cdb.QueryRowContext(ctx, "SELECT name, type, backgroundColor, emblemStyle, emblemColor, borderStyle, borderColor, rating, weekGames, weekWins, seasonGames, seasonWins, rank FROM arena_team WHERE arenaTeamId = ?", teamID).Scan(
		&name, &aType, &bgCol, &embStyle, &embCol, &bordStyle, &bordCol,
		&rating, &weekGames, &weekWins, &seasonGames, &seasonWins, &rank,
	)
	if err != nil {
		return true
	}

	// SMSG_ARENA_TEAM_QUERY_RESPONSE (0x34C)
	qBuf := protocol.NewBuffer(64 + len(name))
	qBuf.WriteU32(teamID)
	qBuf.WriteCString(name)
	qBuf.WriteU32(aType)
	qBuf.WriteU32(bgCol)
	qBuf.WriteU32(embStyle)
	qBuf.WriteU32(embCol)
	qBuf.WriteU32(bordStyle)
	qBuf.WriteU32(bordCol)
	_ = s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_QUERY_RESPONSE), qBuf.Bytes(), true)

	// SMSG_ARENA_TEAM_STATS (0x35B)
	sBuf := protocol.NewBuffer(28)
	sBuf.WriteU32(teamID)
	sBuf.WriteU32(rating)
	sBuf.WriteU32(weekGames)
	sBuf.WriteU32(weekWins)
	sBuf.WriteU32(seasonGames)
	sBuf.WriteU32(seasonWins)
	sBuf.WriteU32(rank)
	_ = s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_STATS), sBuf.Bytes(), true)
	return true
}

// handleArenaTeamRoster processes CMSG_ARENA_TEAM_ROSTER (0x34D).
// Reference: WorldSession::HandleArenaTeamRosterOpcode (ArenaTeamHandler.cpp:75).
func (s *session) handleArenaTeamRoster(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var aType, captainGuid uint32
	_ = cdb.QueryRowContext(ctx, "SELECT type, captainGuid FROM arena_team WHERE arenaTeamId = ?", teamID).Scan(&aType, &captainGuid)

	type memberInfo struct {
		guid           uint64
		name           string
		class          uint8
		level          uint8
		weekGames      uint32
		weekWins       uint32
		seasonGames    uint32
		seasonWins     uint32
		personalRating uint32
	}
	var members []memberInfo

	rows, err := cdb.QueryContext(ctx, "SELECT atm.guid, COALESCE(c.name, ''), COALESCE(c.class, 1), COALESCE(c.level, 80), atm.weekGames, atm.weekWins, atm.seasonGames, atm.seasonWins, atm.personalRating FROM arena_team_member AS atm LEFT JOIN characters AS c ON c.guid = atm.guid WHERE atm.arenaTeamId = ?", teamID)
	if err == nil {
		for rows.Next() {
			var m memberInfo
			var mGuid int64
			if rows.Scan(&mGuid, &m.name, &m.class, &m.level, &m.weekGames, &m.weekWins, &m.seasonGames, &m.seasonWins, &m.personalRating) == nil {
				m.guid = uint64(mGuid)
				members = append(members, m)
			}
		}
		rows.Close()
	}

	buf := protocol.NewBuffer(13 + len(members)*60)
	buf.WriteU32(teamID)
	buf.WriteU8(0) // unk308
	buf.WriteU32(uint32(len(members)))
	buf.WriteU32(aType)
	for _, m := range members {
		buf.WriteU64(m.guid)
		online := uint8(0)
		if s.server.findSessionByGUID(m.guid) != nil {
			online = 1
		}
		buf.WriteU8(online)
		buf.WriteCString(m.name)
		captainFlag := uint32(1)
		if m.guid == uint64(captainGuid) {
			captainFlag = 0
		}
		buf.WriteU32(captainFlag)
		buf.WriteU8(m.level)
		buf.WriteU8(m.class)
		buf.WriteU32(m.weekGames)
		buf.WriteU32(m.weekWins)
		buf.WriteU32(m.seasonGames)
		buf.WriteU32(m.seasonWins)
		buf.WriteU32(m.personalRating)
	}

	return s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_ROSTER), buf.Bytes(), true) == nil
}

// handleArenaTeamInvite processes CMSG_ARENA_TEAM_INVITE (0x34F).
// Reference: WorldSession::HandleArenaTeamInviteOpcode (ArenaTeamHandler.cpp:86).
func (s *session) handleArenaTeamInvite(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}
	invitedName, err := r.ReadCString()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var teamName string
	_ = cdb.QueryRowContext(ctx, "SELECT name FROM arena_team WHERE arenaTeamId = ?", teamID).Scan(&teamName)

	targetSess := s.server.findSessionByName(invitedName)
	if targetSess != nil {
		targetSess.arenaTeamInvited = teamID
		invBuf := protocol.NewBuffer(len(s.player.Name) + len(teamName) + 2)
		invBuf.WriteCString(s.player.Name)
		invBuf.WriteCString(teamName)
		_ = targetSess.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_INVITE), invBuf.Bytes(), true)
	}

	s.debug("arena team invite sent", "team", teamName, "target", invitedName)
	return true
}

// handleArenaTeamAccept processes CMSG_ARENA_TEAM_ACCEPT (0x351).
// Reference: WorldSession::HandleArenaTeamAcceptOpcode (ArenaTeamHandler.cpp:170).
func (s *session) handleArenaTeamAccept(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.arenaTeamInvited == 0 {
		return true
	}
	teamID := s.arenaTeamInvited
	s.arenaTeamInvited = 0

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "INSERT OR REPLACE INTO arena_team_member (arenaTeamId, guid, weekGames, weekWins, seasonGames, seasonWins, personalRating) VALUES (?, ?, 0, 0, 0, 0, 1500)", teamID, s.playerGUID)
	}

	rosterPayload := protocol.NewBuffer(4)
	rosterPayload.WriteU32(teamID)
	s.handleArenaTeamRoster(ctx, rosterPayload.Bytes())
	s.debug("arena team accepted", "team", teamID)
	return true
}

// handleArenaTeamDecline processes CMSG_ARENA_TEAM_DECLINE (0x352).
// Reference: WorldSession::HandleArenaTeamDeclineOpcode (ArenaTeamHandler.cpp:203).
func (s *session) handleArenaTeamDecline(ctx context.Context, payload []byte) bool {
	s.arenaTeamInvited = 0
	return true
}

// handleArenaTeamLeave processes CMSG_ARENA_TEAM_LEAVE (0x353).
// Reference: WorldSession::HandleArenaTeamLeaveOpcode (ArenaTeamHandler.cpp:211).
func (s *session) handleArenaTeamLeave(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "DELETE FROM arena_team_member WHERE arenaTeamId = ? AND guid = ?", teamID, s.playerGUID)
	}

	res := protocol.NewBuffer(12)
	res.WriteU32(3) // ERR_ARENA_TEAM_QUIT_S
	res.WriteCString("")
	res.WriteCString("")
	res.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_COMMAND_RESULT), res.Bytes(), true)
	return true
}

// handleArenaTeamRemove processes CMSG_ARENA_TEAM_REMOVE (0x354).
// Reference: WorldSession::HandleArenaTeamRemoveOpcode (ArenaTeamHandler.cpp:298).
func (s *session) handleArenaTeamRemove(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}
	name, err := r.ReadCString()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		var memberGUID int64
		if err := cdb.QueryRowContext(ctx, "SELECT guid FROM characters WHERE name = ?", name).Scan(&memberGUID); err == nil && memberGUID > 0 {
			_, _ = cdb.ExecContext(ctx, "DELETE FROM arena_team_member WHERE arenaTeamId = ? AND guid = ?", teamID, memberGUID)
		}
	}

	res := protocol.NewBuffer(12)
	res.WriteU32(3) // ERR_ARENA_TEAM_QUIT_S
	res.WriteCString("")
	res.WriteCString(name)
	res.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_COMMAND_RESULT), res.Bytes(), true)
	return true
}

// handleArenaTeamDisband processes CMSG_ARENA_TEAM_DISBAND (0x355).
// Reference: WorldSession::HandleArenaTeamDisbandOpcode (ArenaTeamHandler.cpp:266).
func (s *session) handleArenaTeamDisband(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "DELETE FROM arena_team_member WHERE arenaTeamId = ?", teamID)
		_, _ = cdb.ExecContext(ctx, "DELETE FROM arena_team WHERE arenaTeamId = ?", teamID)
	}

	res := protocol.NewBuffer(12)
	res.WriteU32(3)
	res.WriteCString("")
	res.WriteCString("")
	res.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_COMMAND_RESULT), res.Bytes(), true)
	return true
}

// handleArenaTeamLeader processes CMSG_ARENA_TEAM_LEADER (0x356).
// Reference: WorldSession::HandleArenaTeamLeaderOpcode (ArenaTeamHandler.cpp:361).
func (s *session) handleArenaTeamLeader(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	r := protocol.NewReader(payload)
	teamID, err := r.ReadU32()
	if err != nil {
		return false
	}
	name, err := r.ReadCString()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		var newLeaderGUID int64
		if err := cdb.QueryRowContext(ctx, "SELECT guid FROM characters WHERE name = ?", name).Scan(&newLeaderGUID); err == nil && newLeaderGUID > 0 {
			_, _ = cdb.ExecContext(ctx, "UPDATE arena_team SET captainGuid = ? WHERE arenaTeamId = ?", newLeaderGUID, teamID)
		}
	}

	res := protocol.NewBuffer(12)
	res.WriteU32(3)
	res.WriteCString("")
	res.WriteCString(name)
	res.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_ARENA_TEAM_COMMAND_RESULT), res.Bytes(), true)
	return true
}
