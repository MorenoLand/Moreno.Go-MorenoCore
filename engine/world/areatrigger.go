package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleAreaTrigger processes CMSG_AREATRIGGER (0x0B4).
// Reference: WorldSession::HandleAreaTriggerOpcode (MiscHandler.cpp:645).
func (s *session) handleAreaTrigger(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	triggerID, err := r.ReadU32()
	if err != nil {
		return false
	}

	wdb := s.server.WorldStore.DB
	if wdb == nil {
		return true
	}

	// 1. Area explored / Quest objective completion
	var questID uint32
	if err := wdb.QueryRowContext(ctx, "SELECT quest FROM areatrigger_involvedrelation WHERE id = ?", triggerID).Scan(&questID); err == nil && questID != 0 {
		status, _ := s.characterQuestStatus(ctx, questID)
		if status == questStatusIncomplete {
			cdb := s.server.CharactersStore.DB
			if cdb != nil {
				_, _ = cdb.ExecContext(ctx, "UPDATE character_queststatus SET status = ? WHERE guid = ? AND quest = ?", questStatusComplete, s.playerGUID, questID)
			}
			for slot := 0; slot < playerQuestLogSlots; slot++ {
				if s.player.QuestLog[slot].QuestID == questID {
					s.player.QuestLog[slot].State = questCompleteStateFlag(questStatusComplete)
					s.sendPlayerQuestLogUpdate(slot)
					break
				}
			}
		}
	}

	// 2. Tavern / Inn resting trigger
	var tavernID uint32
	if err := wdb.QueryRowContext(ctx, "SELECT id FROM areatrigger_tavern WHERE id = ?", triggerID).Scan(&tavernID); err == nil && tavernID != 0 {
		s.player.ExtraFlags |= 0x00000001 // PLAYER_FLAGS_RESTING
		s.sendPlayerUpdate()
		return true
	}

	// 3. Teleport trigger
	var targetMap int64
	var targetX, targetY, targetZ, targetOri float64
	if err := wdb.QueryRowContext(ctx, "SELECT target_map, target_position_x, target_position_y, target_position_z, target_orientation FROM areatrigger_teleport WHERE id = ?", triggerID).Scan(&targetMap, &targetX, &targetY, &targetZ, &targetOri); err == nil {
		s.teleportTo(uint32(targetMap), float32(targetX), float32(targetY), float32(targetZ), float32(targetOri))
		return true
	}

	s.debug("areatrigger handled", "account", s.accountName, "trigger", triggerID)
	return true
}
