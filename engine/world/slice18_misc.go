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

// handleFarSight processes CMSG_FAR_SIGHT (0x27A).
func (s *session) handleFarSight(ctx context.Context, payload []byte) bool {
	return true
}

// handleForceMoveRootAck processes CMSG_FORCE_MOVE_ROOT_ACK (0x0E9).
func (s *session) handleForceMoveRootAck(ctx context.Context, payload []byte) bool {
	return true
}

// handleForceMoveUnrootAck processes CMSG_FORCE_MOVE_UNROOT_ACK (0x0EB).
func (s *session) handleForceMoveUnrootAck(ctx context.Context, payload []byte) bool {
	return true
}

// handleForceTurnRateChangeAck processes CMSG_FORCE_TURN_RATE_CHANGE_ACK (0x2DF).
func (s *session) handleForceTurnRateChangeAck(ctx context.Context, payload []byte) bool {
	return true
}

// handleGetChannelMemberCount processes CMSG_GET_CHANNEL_MEMBER_COUNT (0x3D3).
func (s *session) handleGetChannelMemberCount(ctx context.Context, payload []byte) bool {
	return true
}

// handleGetMirrorImageData processes CMSG_GET_MIRRORIMAGE_DATA (0x401).
// Reference: WorldSession::HandleMirrorImageDataRequest (SpellHandler.cpp:635).
func (s *session) handleGetMirrorImageData(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, _ := r.ReadU64()

	buf := protocol.NewBuffer(68)
	buf.WriteU64(guid)
	buf.WriteU32(0) // displayId
	buf.WriteU8(s.player.Race)
	buf.WriteU8(s.player.Gender)
	buf.WriteU8(s.player.Class)
	buf.WriteU8(s.player.Skin)
	buf.WriteU8(s.player.Face)
	buf.WriteU8(s.player.HairStyle)
	buf.WriteU8(s.player.HairColor)
	buf.WriteU8(s.player.FacialStyle)
	buf.WriteU32(0) // guildId
	for i := 0; i < 11; i++ {
		buf.WriteU32(0) // outfit item displays
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_MIRRORIMAGE_DATA), buf.Bytes(), true)
	return true
}

// handleGmTicketSystemToggle processes CMSG_GMTICKETSYSTEM_TOGGLE (0x29A).
func (s *session) handleGmTicketSystemToggle(ctx context.Context, payload []byte) bool {
	return true
}

// handleGrantLevel processes CMSG_GRANT_LEVEL (0x41F).
func (s *session) handleGrantLevel(ctx context.Context, payload []byte) bool {
	return s.handleAcceptLevelGrant(ctx, payload)
}

// handleGroupAssistantLeader processes CMSG_GROUP_ASSISTANT_LEADER (0x28F).
func (s *session) handleGroupAssistantLeader(ctx context.Context, payload []byte) bool {
	return true
}

// handleGroupChangeSubGroup processes CMSG_GROUP_CHANGE_SUB_GROUP (0x27E).
func (s *session) handleGroupChangeSubGroup(ctx context.Context, payload []byte) bool {
	return true
}

// handleEnableTaxi processes CMSG_ENABLETAXI (0x493).
func (s *session) handleEnableTaxi(ctx context.Context, payload []byte) bool {
	return s.handleTaxiNodeStatusQuery(ctx, payload)
}

// handleDismissCritter processes CMSG_DISMISS_CRITTER (0x48D).
func (s *session) handleDismissCritter(ctx context.Context, payload []byte) bool {
	return true
}

// handleChangeSeatsOnControlledVehicle processes CMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE (0x49B).
func (s *session) handleChangeSeatsOnControlledVehicle(ctx context.Context, payload []byte) bool {
	return true
}

// handleControllerEjectPassenger processes CMSG_CONTROLLER_EJECT_PASSENGER (0x4A9).
func (s *session) handleControllerEjectPassenger(ctx context.Context, payload []byte) bool {
	return true
}

// handleDismissControlledVehicle processes CMSG_DISMISS_CONTROLLED_VEHICLE (0x46D).
func (s *session) handleDismissControlledVehicle(ctx context.Context, payload []byte) bool {
	return true
}
