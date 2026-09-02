package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleBfEntryInviteResponse processes CMSG_BATTLEFIELD_MGR_ENTRY_INVITE_RESPONSE (0x4DF).
// Reference: WorldSession::HandleBfEntryInviteResponse (BattlefieldHandler.cpp:143).
func (s *session) handleBfEntryInviteResponse(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	r := protocol.NewReader(payload)
	battleID, err := r.ReadU32()
	if err != nil {
		return false
	}
	accepted, err := r.ReadU8()
	if err != nil {
		return false
	}

	if accepted != 0 {
		buf := protocol.NewBuffer(9)
		buf.WriteU32(battleID)
		buf.WriteU8(0) // unk
		buf.WriteU32(1) // clear afk
		_ = s.write(uint16(protocol.OpcodeSMSG_BATTLEFIELD_MGR_ENTERED), buf.Bytes(), true)
	}
	s.debug("battlefield entry invite response", "battle", battleID, "accepted", accepted)
	return true
}

// handleBfQueueInviteResponse processes CMSG_BATTLEFIELD_MGR_QUEUE_INVITE_RESPONSE (0x4E2).
// Reference: WorldSession::HandleBfQueueInviteResponse.
func (s *session) handleBfQueueInviteResponse(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	r := protocol.NewReader(payload)
	battleID, _ := r.ReadU32()
	accepted, _ := r.ReadU8()

	s.debug("battlefield queue invite response", "battle", battleID, "accepted", accepted)
	return true
}

// handleBfQueueExitRequest processes CMSG_BATTLEFIELD_MGR_EXIT_REQUEST (0x4E7).
// Reference: WorldSession::HandleBfQueueExitRequest (BattlefieldHandler.cpp:173).
func (s *session) handleBfQueueExitRequest(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	battleID, _ := r.ReadU32()

	buf := protocol.NewBuffer(9)
	buf.WriteU32(battleID)
	buf.WriteU8(0) // reason 0 = normal exit
	buf.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_BATTLEFIELD_MGR_EJECTED), buf.Bytes(), true)
	s.debug("battlefield exit request", "battle", battleID)
	return true
}
