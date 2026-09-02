package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleBinderActivate processes CMSG_BINDER_ACTIVATE (0x1B5).
// Reference: WorldSession::HandleBinderActivateOpcode (NPCHandler.cpp:247).
func (s *session) handleBinderActivate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	npcGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	// Update player homebind location
	s.player.HomebindMap = s.player.Map
	s.player.HomebindZone = s.player.Zone
	s.player.HomebindX = s.player.X
	s.player.HomebindY = s.player.Y
	s.player.HomebindZ = s.player.Z

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET homebind_map = ?, homebind_zone = ?, homebind_x = ?, homebind_y = ?, homebind_z = ? WHERE guid = ?",
			s.player.HomebindMap, s.player.HomebindZone, s.player.HomebindX, s.player.HomebindY, s.player.HomebindZ, s.playerGUID)
	}

	// Send trainer buy succeeded with homebind spell 3286
	res := protocol.NewBuffer(12)
	res.WriteU64(npcGUID)
	res.WriteU32(3286)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED), res.Bytes(), true)
	s.sendGossipComplete()
	s.sendPlayerUpdate()
	s.debug("homebind set", "account", s.accountName, "npc", npcGUID, "map", s.player.HomebindMap, "zone", s.player.HomebindZone)
	return true
}
