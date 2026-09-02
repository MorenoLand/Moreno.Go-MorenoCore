package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const factionFlagAtWar uint8 = 0x02

func (s *session) handleSetFactionAtWar(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	reader := protocol.NewReader(payload)
	listID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	flag, err := reader.ReadU8()
	if err != nil {
		return false
	}
	for index := range s.player.Reputations {
		reputation := &s.player.Reputations[index]
		if reputation.ListID != listID {
			continue
		}
		if flag != 0 {
			reputation.Flags |= factionFlagAtWar
		} else {
			reputation.Flags &^= factionFlagAtWar
		}
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_reputation SET flags = ? WHERE guid = ? AND faction = ?", reputation.Flags, s.playerGUID, reputation.FactionID)
		}
		s.debug("faction war state changed", "account", s.accountName, "faction", reputation.FactionID, "at_war", flag != 0)
		return true
	}
	return true
}
