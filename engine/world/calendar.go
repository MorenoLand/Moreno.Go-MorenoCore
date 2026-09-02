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
	buf := protocol.NewBuffer(64)
	buf.WriteU32(0) // invites count
	buf.WriteU32(0) // events count
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
	buf := protocol.NewBuffer(4)
	buf.WriteU32(0) // 0 pending calendar invites
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

	buf := protocol.NewBuffer(32)
	buf.WriteU64(eventID)
	buf.WriteCString("") // title
	buf.WriteU32(0)      // type
	buf.WriteU32(0)      // dungeonID
	buf.WriteU32(0)      // flags
	buf.WritePackedTime(time.Now())
	buf.WriteU32(0)      // invites count
	return s.write(uint16(protocol.OpcodeSMSG_CALENDAR_SEND_EVENT), buf.Bytes(), true) == nil
}

// handleCalendarGuildFilter processes CMSG_CALENDAR_GUILD_FILTER (0x42B).
func (s *session) handleCalendarGuildFilter(ctx context.Context, payload []byte) bool {
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
	return s.sendCalendarCommandResult(0, 0)
}

// handleCalendarUpdateEvent processes CMSG_CALENDAR_UPDATE_EVENT (0x42E).
func (s *session) handleCalendarUpdateEvent(ctx context.Context, payload []byte) bool {
	return s.sendCalendarCommandResult(1, 0)
}

// handleCalendarRemoveEvent processes CMSG_CALENDAR_REMOVE_EVENT (0x42F).
func (s *session) handleCalendarRemoveEvent(ctx context.Context, payload []byte) bool {
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
func (s *session) handleCalendarComplain(ctx context.Context, payload []byte) bool {
	return true
}
