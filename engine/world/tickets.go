package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	gmTicketQueueStatusEnabled    uint32 = 1
	gmTicketStatusDefault         uint32 = 10
	gmTicketResponseCreateSuccess uint32 = 1
	gmTicketResponseUpdateSuccess uint32 = 1
	gmTicketResponseDeleted       uint32 = 9
)

// handleGMTicketSystemStatus processes CMSG_GMTICKET_SYSTEMSTATUS (0x21A).
// Reference: WorldSession::HandleGMTicketSystemStatusOpcode (TicketHandler.cpp:185).
func (s *session) handleGMTicketSystemStatus(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(4)
	buf.WriteU32(gmTicketQueueStatusEnabled)
	return s.write(uint16(protocol.OpcodeSMSG_GMTICKET_SYSTEMSTATUS), buf.Bytes(), true) == nil
}

// handleGMTicketGetTicket processes CMSG_GMTICKET_GETTICKET (0x211).
// Reference: WorldSession::HandleGMTicketGetTicketOpcode (TicketHandler.cpp:170) and TicketMgr::SendTicket (TicketMgr.cpp:446).
func (s *session) handleGMTicketGetTicket(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(4)
	buf.WriteU32(gmTicketStatusDefault)
	return s.write(uint16(protocol.OpcodeSMSG_GMTICKET_GETTICKET), buf.Bytes(), true) == nil
}

// handleGMTicketCreate processes CMSG_GMTICKET_CREATE (0x205).
// Reference: WorldSession::HandleGMTicketCreateOpcode (TicketHandler.cpp:34).
func (s *session) handleGMTicketCreate(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	_, _ = r.ReadU32()     // mapId
	_, _ = r.ReadF32()     // x
	_, _ = r.ReadF32()     // y
	_, _ = r.ReadF32()     // z
	_, _ = r.ReadCString() // message

	buf := protocol.NewBuffer(4)
	buf.WriteU32(gmTicketResponseCreateSuccess)
	return s.write(uint16(protocol.OpcodeSMSG_GMTICKET_CREATE), buf.Bytes(), true) == nil
}

// handleGMTicketUpdate processes CMSG_GMTICKET_UPDATETEXT (0x207).
// Reference: WorldSession::HandleGMTicketUpdateOpcode (TicketHandler.cpp:130).
func (s *session) handleGMTicketUpdate(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	_, _ = r.ReadCString() // message

	buf := protocol.NewBuffer(4)
	buf.WriteU32(gmTicketResponseUpdateSuccess)
	return s.write(uint16(protocol.OpcodeSMSG_GMTICKET_UPDATETEXT), buf.Bytes(), true) == nil
}

// handleGMTicketDelete processes CMSG_GMTICKET_DELETETICKET (0x217).
// Reference: WorldSession::HandleGMTicketDeleteOpcode (TicketHandler.cpp:155).
func (s *session) handleGMTicketDelete(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(4)
	buf.WriteU32(gmTicketResponseDeleted)
	return s.write(uint16(protocol.OpcodeSMSG_GMTICKET_DELETETICKET), buf.Bytes(), true) == nil
}

// handleGMResponseResolve processes CMSG_GMRESPONSE_RESOLVE (0x4F0).
// Reference: WorldSession::HandleGMResponseResolve (TicketHandler.cpp:272).
func (s *session) handleGMResponseResolve(ctx context.Context, payload []byte) bool {
	bufStatus := protocol.NewBuffer(1)
	bufStatus.WriteU8(0) // getSurvey = 0
	_ = s.write(uint16(protocol.OpcodeSMSG_GMRESPONSE_STATUS_UPDATE), bufStatus.Bytes(), true)

	bufDel := protocol.NewBuffer(4)
	bufDel.WriteU32(gmTicketResponseDeleted)
	return s.write(uint16(protocol.OpcodeSMSG_GMTICKET_DELETETICKET), bufDel.Bytes(), true) == nil
}

// handleGMSurveySubmit processes CMSG_GMSURVEY_SUBMIT (0x32A).
// Reference: WorldSession::HandleGMSurveySubmit (TicketHandler.cpp:194).
func (s *session) handleGMSurveySubmit(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	_, _ = r.ReadU32() // mainSurvey
	return true
}

// handleGMReportLag processes CMSG_GM_REPORT_LAG (0x502).
// Reference: WorldSession::HandleReportLag (TicketHandler.cpp:248).
func (s *session) handleGMReportLag(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	_, _ = r.ReadU32() // lagType
	_, _ = r.ReadU32() // mapId
	_, _ = r.ReadF32() // x
	_, _ = r.ReadF32() // y
	_, _ = r.ReadF32() // z
	return true
}

// handleGmTicketSystemToggle processes CMSG_GMTICKETSYSTEM_TOGGLE (0x29A).
func (s *session) handleGmTicketSystemToggle(ctx context.Context, payload []byte) bool {
	return true
}

// handleBug processes CMSG_BUG (0x1CA).
// Reference: WorldSession::HandleBugOpcode (MiscHandler.cpp:551).
func (s *session) handleBug(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	suggestion, err := r.ReadU32()
	if err != nil {
		return false
	}
	contentLen, err := r.ReadU32()
	if err != nil {
		return false
	}
	content, err := r.ReadString(int(contentLen))
	if err != nil {
		return false
	}
	typeLen, err := r.ReadU32()
	if err != nil {
		return false
	}
	typeStr, err := r.ReadString(int(typeLen))
	if err != nil {
		return false
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		// CHAR_INS_BUG_REPORT: INSERT INTO bugreport (type, content) VALUES(?, ?)
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "INSERT INTO bugreport (type, content) VALUES (?, ?)", typeStr, content)
	}

	s.debug("bug report received", "account", s.accountName, "suggestion", suggestion, "type", typeStr, "len", len(content))
	return true
}
