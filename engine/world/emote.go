package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func (s *session) handleStandStateChange(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	reader := protocol.NewReader(payload)
	state, err := reader.ReadU32()
	if err != nil || state > 3 {
		return true
	}
	s.player.StandState = uint8(state)
	s.server.broadcastPlayerValuesUpdate(s.player.Map, map[int]uint32{unitFieldBytes1: state})
	s.debug("stand state changed", "account", s.accountName, "state", state)
	return true
}

func (s *session) handleEmote(payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 || s.player.Health == 0 {
		return true
	}
	reader := protocol.NewReader(payload)
	emote, err := reader.ReadU32()
	if err != nil || (emote != 0 && emote != 17) {
		return true
	}
	packet := protocol.NewBuffer(12)
	packet.WriteU32(emote)
	packet.WriteU64(s.playerGUID)
	s.server.sessionsMu.RLock()
	defer s.server.sessionsMu.RUnlock()
	for member := range s.server.sessions {
		if !member.playerLoaded || member.player == nil || member.player.Map != s.player.Map {
			continue
		}
		_ = member.write(uint16(protocol.OpcodeSMSG_EMOTE), packet.Bytes(), true)
	}
	s.debug("emote sent", "account", s.accountName, "emote", emote)
	return true
}

func (s *Server) broadcastPlayerValuesUpdate(mapID uint32, fields map[int]uint32) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for member := range s.sessions {
		if !member.playerLoaded || member.player == nil || member.player.Map != mapID {
			continue
		}
		guidPacket, buildErr := s.buildPlayerValuesUpdate(member.playerGUID, fields)
		if buildErr == nil && guidPacket != nil {
			_ = member.write(guidPacket.Opcode, guidPacket.Payload.Bytes(), true)
		}
	}
}
