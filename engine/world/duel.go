package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleDuelAccepted processes CMSG_DUEL_ACCEPTED (0x16C).
// Reference: WorldSession::HandleDuelAcceptedOpcode (DuelHandler.cpp:25).
func (s *session) handleDuelAccepted(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	buf := protocol.NewBuffer(4)
	buf.WriteU32(3000) // 3000ms duel countdown
	_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_COUNTDOWN), buf.Bytes(), true)
	return true
}

// handleDuelCancelled processes CMSG_DUEL_CANCELLED (0x16D).
// Reference: WorldSession::HandleDuelCancelledOpcode (DuelHandler.cpp:53).
func (s *session) handleDuelCancelled(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	buf := protocol.NewBuffer(1)
	buf.WriteU8(0) // interrupted
	_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_COMPLETE), buf.Bytes(), true)
	return true
}
