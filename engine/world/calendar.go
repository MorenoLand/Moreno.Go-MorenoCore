package world

import (
	"context"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleCalendarGetCalendar processes CMSG_CALENDAR_GET_CALENDAR (0x429).
// Reference: WorldSession::HandleCalendarGetCalendar (CalendarHandler.cpp:56).
func (s *session) handleCalendarGetCalendar(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	currTime := time.Now()

	type calEvent struct {
		id        uint64
		title     string
		eventType uint32
		dungeon   int32
		flags     uint32
		eventTime uint32
	}
	var events []calEvent

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		rows, err := s.server.CharactersStore.DB.QueryContext(ctx,
			`SELECT id, title, type, dungeon, flags, eventtime FROM calendar_events
			 WHERE creator = ? OR id IN (SELECT event FROM calendar_invites WHERE invitee = ?)`,
			s.playerGUID, s.playerGUID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var ev calEvent
				if err := rows.Scan(&ev.id, &ev.title, &ev.eventType, &ev.dungeon, &ev.flags, &ev.eventTime); err == nil {
					events = append(events, ev)
				}
			}
		}
	}

	buf := protocol.NewBuffer(128 + len(events)*64)
	buf.WriteU32(0)                   // invites count
	buf.WriteU32(uint32(len(events))) // events count
	for _, ev := range events {
		buf.WriteU64(ev.id)
		buf.WriteCString(ev.title)
		buf.WriteU32(ev.eventType)
		buf.WritePackedTime(time.Unix(int64(ev.eventTime), 0))
		buf.WriteU32(ev.flags)
		buf.WriteI32(ev.dungeon)
		buf.WriteU64(s.playerGUID) // creator
	}
	buf.WriteU32(uint32(currTime.Unix())) // server time
	buf.WritePackedTime(currTime)         // zone time
	buf.WriteU32(0)                       // bound instances count
	buf.WriteU32(1135753200)              // reset time default
	buf.WriteU32(0)                       // holiday count
	return s.write(uint16(protocol.OpcodeSMSG_CALENDAR_SEND_CALENDAR), buf.Bytes(), true) == nil
}

// handleCalendarGetNumPending processes CMSG_CALENDAR_GET_NUM_PENDING (0x447).
// Reference: WorldSession::HandleCalendarGetNumPending (CalendarHandler.cpp:773).
func (s *session) handleCalendarGetNumPending(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	var count uint32 = 0
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_ = s.server.CharactersStore.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM calendar_invites WHERE invitee = ? AND status = 0", s.playerGUID).Scan(&count)
	}
	buf := protocol.NewBuffer(4)
	buf.WriteU32(count)
	return s.write(uint16(protocol.OpcodeSMSG_CALENDAR_SEND_NUM_PENDING), buf.Bytes(), true) == nil
}

// handleCalendarGetEvent processes CMSG_CALENDAR_GET_EVENT (0x42A).
// Reference: WorldSession::HandleCalendarGetEvent (CalendarHandler.cpp:142).
func (s *session) handleCalendarGetEvent(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	eventID, _ := r.ReadU64()

	title := ""
	description := ""
	var eventType uint32
	var dungeon int32 = -1
	var flags uint32
	var eventTime uint32

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_ = s.server.CharactersStore.DB.QueryRowContext(ctx,
			"SELECT title, description, type, dungeon, flags, eventtime FROM calendar_events WHERE id = ?",
			eventID).Scan(&title, &description, &eventType, &dungeon, &flags, &eventTime)
	}

	buf := protocol.NewBuffer(64 + len(title) + len(description))
	buf.WriteU64(eventID)
	buf.WriteCString(title)
	buf.WriteU32(eventType)
	buf.WriteI32(dungeon)
	buf.WriteU32(flags)
	if eventTime > 0 {
		buf.WritePackedTime(time.Unix(int64(eventTime), 0))
	} else {
		buf.WritePackedTime(time.Now())
	}
	buf.WriteU32(0) // invites count
	return s.write(uint16(protocol.OpcodeSMSG_CALENDAR_SEND_EVENT), buf.Bytes(), true) == nil
}

// handleCalendarGuildFilter processes CMSG_CALENDAR_GUILD_FILTER (0x42B).
// Reference: WorldSession::HandleCalendarGuildFilter (CalendarHandler.cpp:194).
func (s *session) handleCalendarGuildFilter(ctx context.Context, payload []byte) bool {
	if len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	minLevel, _ := r.ReadU32()
	maxLevel, _ := r.ReadU32()
	minRank, _ := r.ReadU32()
	s.debug("calendar guild filter", "account", s.accountName, "minLevel", minLevel, "maxLevel", maxLevel, "minRank", minRank)
	return true
}

// handleCalendarArenaTeam processes CMSG_CALENDAR_ARENA_TEAM (0x42C).
// Reference: WorldSession::HandleCalendarArenaTeam (CalendarHandler.cpp:751).
func (s *session) handleCalendarArenaTeam(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	buf := protocol.NewBuffer(4)
	buf.WriteU32(0) // count of members
	return s.write(uint16(protocol.OpcodeSMSG_CALENDAR_ARENA_TEAM), buf.Bytes(), true) == nil
}

func (s *session) sendCalendarCommandResult(cmd uint32, err uint32) bool {
	buf := protocol.NewBuffer(8)
	buf.WriteU32(cmd)
	buf.WriteU32(err)
	return s.write(uint16(protocol.OpcodeSMSG_CALENDAR_COMMAND_RESULT), buf.Bytes(), true) == nil
}

// handleCalendarAddEvent processes CMSG_CALENDAR_ADD_EVENT (0x42D).
func (s *session) handleCalendarAddEvent(ctx context.Context, payload []byte) bool {
	if len(payload) < 16 {
		return s.sendCalendarCommandResult(0, 1)
	}
	r := protocol.NewReader(payload)
	title, _ := r.ReadCString()
	description, _ := r.ReadCString()
	eventType, _ := r.ReadU8()
	_, _ = r.ReadU8()  // repeatable
	_, _ = r.ReadU32() // maxInvites
	dungeonID, _ := r.ReadI32()
	packedEventTime, _ := r.ReadU32()
	packedUnkTime, _ := r.ReadU32()
	flags, _ := r.ReadU32()

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		var nextID uint64 = 1
		_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM calendar_events").Scan(&nextID)
		_, _ = cdb.ExecContext(ctx,
			`INSERT INTO calendar_events (id, creator, title, description, type, dungeon, eventtime, flags, time2)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			nextID, s.playerGUID, title, description, eventType, dungeonID, packedEventTime, flags, packedUnkTime)

		var nextInviteID uint64 = 1
		_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM calendar_invites").Scan(&nextInviteID)
		_, _ = cdb.ExecContext(ctx,
			`INSERT INTO calendar_invites (id, event, invitee, sender, status, statustime, rank, text)
			 VALUES (?, ?, ?, ?, 1, ?, 2, '')`,
			nextInviteID, nextID, s.playerGUID, s.playerGUID, time.Now().Unix())
	}
	return s.sendCalendarCommandResult(0, 0)
}

// handleCalendarUpdateEvent processes CMSG_CALENDAR_UPDATE_EVENT (0x42E).
func (s *session) handleCalendarUpdateEvent(ctx context.Context, payload []byte) bool {
	if len(payload) < 24 {
		return s.sendCalendarCommandResult(1, 1)
	}
	r := protocol.NewReader(payload)
	eventID, _ := r.ReadU64()
	title, _ := r.ReadCString()
	description, _ := r.ReadCString()
	eventType, _ := r.ReadU8()
	_, _ = r.ReadU8()  // repeatable
	_, _ = r.ReadU32() // maxInvites
	dungeonID, _ := r.ReadI32()
	packedEventTime, _ := r.ReadU32()
	packedUnkTime, _ := r.ReadU32()
	flags, _ := r.ReadU32()

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
			`UPDATE calendar_events SET title = ?, description = ?, type = ?, dungeon = ?, eventtime = ?, flags = ?, time2 = ?
			 WHERE id = ? AND creator = ?`,
			title, description, eventType, dungeonID, packedEventTime, flags, packedUnkTime, eventID, s.playerGUID)
	}
	return s.sendCalendarCommandResult(1, 0)
}

// handleCalendarRemoveEvent processes CMSG_CALENDAR_REMOVE_EVENT (0x42F).
func (s *session) handleCalendarRemoveEvent(ctx context.Context, payload []byte) bool {
	if len(payload) < 8 {
		return s.sendCalendarCommandResult(2, 1)
	}
	r := protocol.NewReader(payload)
	eventID, _ := r.ReadU64()

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM calendar_events WHERE id = ? AND creator = ?", eventID, s.playerGUID)
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM calendar_invites WHERE event = ?", eventID)
	}
	return s.sendCalendarCommandResult(2, 0)
}

// handleCalendarCopyEvent processes CMSG_CALENDAR_COPY_EVENT (0x430).
func (s *session) handleCalendarCopyEvent(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(3, 0)
}

// handleCalendarEventInvite processes CMSG_CALENDAR_EVENT_INVITE (0x431).
func (s *session) handleCalendarEventInvite(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(4, 0)
}

// handleCalendarEventRSVP processes CMSG_CALENDAR_EVENT_RSVP (0x432).
func (s *session) handleCalendarEventRSVP(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(5, 0)
}

// handleCalendarEventRemoveInvite processes CMSG_CALENDAR_EVENT_REMOVE_INVITE (0x433).
func (s *session) handleCalendarEventRemoveInvite(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(6, 0)
}

// handleCalendarEventStatus processes CMSG_CALENDAR_EVENT_STATUS (0x434).
func (s *session) handleCalendarEventStatus(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(7, 0)
}

// handleCalendarEventModeratorStatus processes CMSG_CALENDAR_EVENT_MODERATOR_STATUS (0x435).
func (s *session) handleCalendarEventModeratorStatus(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(8, 0)
}

// handleCalendarEventSignup processes CMSG_CALENDAR_EVENT_SIGNUP (0x4BA).
func (s *session) handleCalendarEventSignup(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(9, 0)
}

// handleCalendarComplain processes CMSG_CALENDAR_COMPLAIN (0x446).
// Reference: WorldSession::HandleCalendarComplain (CalendarHandler.cpp:760).
func (s *session) handleCalendarComplain(ctx context.Context, payload []byte) bool {
	if len(payload) < 16 {
		return true
	}
	r := protocol.NewReader(payload)
	eventID, _ := r.ReadU64()
	complainGUID, _ := r.ReadU64()
	s.debug("calendar complain", "account", s.accountName, "event", eventID, "guid", complainGUID)
	return true
}
