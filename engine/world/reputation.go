package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	factionFlagAtWar           uint8 = 0x02
	factionFlagHidden          uint8 = 0x04
	factionFlagInvisibleForced uint8 = 0x08
	factionFlagInactive        uint8 = 0x20
	factionFlagVisible         uint8 = 0x01
)

// handleSetWatchedFaction mirrors WorldSession::HandleSetWatchedFactionOpcode
// (CharacterHandler.cpp): read the reputation list index and store it in
// PLAYER_FIELD_WATCHED_FACTION_INDEX, pushing the field change to the owning
// client as a values update.
func (s *session) handleSetWatchedFaction(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	fact, err := reader.ReadU32()
	if err != nil {
		return false
	}
	s.player.WatchedFaction = fact
	packet, err := s.server.buildPlayerValuesUpdate(s.playerGUID, map[int]uint32{unitFieldWatchedFaction: fact})
	if err != nil {
		s.debug("watched faction values update build failed", "account", s.accountName, "error", err)
		return true
	}
	_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
	return true
}

// handleSetFactionInactive mirrors WorldSession::HandleSetFactionInactiveOpcode
// plus ReputationMgr::SetInactive: toggle FACTION_FLAG_INACTIVE on the faction
// matching the reputation list ID. Hidden, forced-invisible, or not-yet-visible
// factions cannot be inactivated, and already-matching states are ignored. The
// flag is persisted immediately like the at-war toggle and only reaches the
// client through SMSG_INITIALIZE_FACTIONS on the next login, matching the
// reference where SendState carries standing but never flags.
func (s *session) handleSetFactionInactive(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 5 {
		return true
	}
	reader := protocol.NewReader(payload)
	listID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	inactive, err := reader.ReadU8()
	if err != nil {
		return false
	}
	for index := range s.player.Reputations {
		reputation := &s.player.Reputations[index]
		if reputation.ListID != listID {
			continue
		}
		if inactive != 0 {
			if reputation.Flags&(factionFlagInvisibleForced|factionFlagHidden) != 0 || reputation.Flags&factionFlagVisible == 0 {
				return true
			}
			if reputation.Flags&factionFlagInactive != 0 {
				return true
			}
			reputation.Flags |= factionFlagInactive
		} else {
			if reputation.Flags&factionFlagInactive == 0 {
				return true
			}
			reputation.Flags &^= factionFlagInactive
		}
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_reputation SET flags = ? WHERE guid = ? AND faction = ?", reputation.Flags, s.playerGUID, reputation.FactionID)
		}
		s.debug("faction inactive state changed", "account", s.accountName, "faction", reputation.FactionID, "inactive", inactive != 0)
		return true
	}
	return true
}

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
