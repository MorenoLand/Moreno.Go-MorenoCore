package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleAcceptLevelGrant processes CMSG_ACCEPT_LEVEL_GRANT (0x420).
// Reference: WorldSession::HandleAcceptGrantLevel (ReferAFriendHandler.cpp:67).
func (s *session) handleAcceptLevelGrant(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadPackedGUID() // granter GUID

	if s.player.Level >= 80 {
		return true
	}

	s.player.Level++
	s.player.MaxHealth = 200 + uint32(s.player.Level)*50
	s.player.Health = s.player.MaxHealth
	if s.player.Powers[0] > 0 || s.player.MaxPowers[0] > 0 {
		s.player.MaxPowers[0] = 100 + uint32(s.player.Level)*40
		s.player.Powers[0] = s.player.MaxPowers[0]
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET level = ? WHERE guid = ?", s.player.Level, s.playerGUID)
	}

	s.sendPlayerUpdate()
	s.debug("level granted", "account", s.accountName, "level", s.player.Level)
	return true
}

// handleGrantLevel processes CMSG_GRANT_LEVEL (0x41F).
func (s *session) handleGrantLevel(ctx context.Context, payload []byte) bool {
	return s.handleAcceptLevelGrant(ctx, payload)
}

