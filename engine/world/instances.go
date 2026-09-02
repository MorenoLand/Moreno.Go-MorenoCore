package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleResetInstances processes CMSG_RESET_INSTANCES (0x31D).
// Reference: WorldSession::HandleResetInstancesOpcode (MiscHandler.cpp:1255).
func (s *session) handleResetInstances(ctx context.Context, payload []byte) bool {
	return true
}

// handleSetDungeonDifficulty processes MSG_SET_DUNGEON_DIFFICULTY (0x329).
// Reference: WorldSession::HandleSetDungeonDifficultyOpcode (MiscHandler.cpp:1268).
func (s *session) handleSetDungeonDifficulty(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	mode, err := r.ReadU32()
	if err != nil {
		return false
	}

	buf := protocol.NewBuffer(4)
	buf.WriteU32(mode)
	_ = s.write(uint16(protocol.OpcodeMSG_SET_DUNGEON_DIFFICULTY), buf.Bytes(), true)
	return true
}

// handleSetRaidDifficulty processes MSG_SET_RAID_DIFFICULTY (0x4EB).
// Reference: WorldSession::HandleSetRaidDifficultyOpcode (MiscHandler.cpp:1323).
func (s *session) handleSetRaidDifficulty(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	mode, err := r.ReadU32()
	if err != nil {
		return false
	}

	buf := protocol.NewBuffer(4)
	buf.WriteU32(mode)
	_ = s.write(uint16(protocol.OpcodeMSG_SET_RAID_DIFFICULTY), buf.Bytes(), true)
	return true
}

// handleInstanceLockResponse processes CMSG_INSTANCE_LOCK_RESPONSE (0x13F).
// Reference: WorldSession::HandleInstanceLockResponse (MiscHandler.cpp:1449).
func (s *session) handleInstanceLockResponse(ctx context.Context, payload []byte) bool {
	return true
}

// handleSetSavedInstanceExtend processes CMSG_SET_SAVED_INSTANCE_EXTEND (0x292).
// Reference: WorldSession::HandleSetSavedInstanceExtend (MiscHandler.cpp:1455).
func (s *session) handleSetSavedInstanceExtend(ctx context.Context, payload []byte) bool {
	return true
}
