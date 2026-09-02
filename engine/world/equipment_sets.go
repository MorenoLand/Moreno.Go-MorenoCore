package world

import (
	"context"
	"fmt"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	maxEquipmentSetIndex uint32 = 10
	equipmentSlotEnd     uint32 = 19
)

// handleEquipmentSetSave processes CMSG_EQUIPMENT_SET_SAVE (0x4BD).
// Reference: WorldSession::HandleEquipmentSetSave (CharacterHandler.cpp:1492).
func (s *session) handleEquipmentSetSave(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	setGuid, err := r.ReadPackedGUID()
	if err != nil {
		return false
	}
	index, err := r.ReadU32()
	if err != nil || index >= maxEquipmentSetIndex {
		return false
	}
	name, err := r.ReadCString()
	if err != nil {
		return false
	}
	iconName, err := r.ReadCString()
	if err != nil {
		return false
	}

	var items [19]uint64
	var ignoreMask uint32
	for i := uint32(0); i < equipmentSlotEnd; i++ {
		itemGuid, err := r.ReadPackedGUID()
		if err != nil {
			break
		}
		if itemGuid == 1 {
			ignoreMask |= 1 << i
			continue
		}
		items[i] = itemGuid
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		placeholders := strings.Repeat("?, ", 24) + "?"
		cols := "guid, setguid, setindex, name, iconname, ignore_mask"
		for i := 0; i < 19; i++ {
			cols += fmt.Sprintf(", item%d", i)
		}
		args := []any{s.playerGUID, setGuid, index, name, iconName, ignoreMask}
		for i := 0; i < 19; i++ {
			args = append(args, items[i])
		}
		query := fmt.Sprintf("REPLACE INTO character_equipmentsets (%s) VALUES (%s)", cols, placeholders)
		_, _ = cdb.ExecContext(ctx, query, args...)
	}
	return true
}

// handleEquipmentSetDelete processes CMSG_DELETEEQUIPMENT_SET (0x13E).
// Reference: WorldSession::HandleEquipmentSetDelete (CharacterHandler.cpp:1544).
func (s *session) handleEquipmentSetDelete(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	setGuid, err := r.ReadPackedGUID()
	if err != nil {
		return false
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_equipmentsets WHERE setguid = ? AND guid = ?", setGuid, s.playerGUID)
	}
	return true
}

// handleEquipmentSetUse processes CMSG_EQUIPMENT_SET_USE (0x4D5).
// Reference: WorldSession::HandleEquipmentSetUse (CharacterHandler.cpp:1554).
func (s *session) handleEquipmentSetUse(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)

	for i := uint32(0); i < equipmentSlotEnd; i++ {
		itemGuid, err := r.ReadPackedGUID()
		if err != nil {
			break
		}
		srcbag, err := r.ReadU8()
		if err != nil {
			break
		}
		srcslot, err := r.ReadU8()
		if err != nil {
			break
		}
		if itemGuid == 1 || itemGuid == 0 {
			continue
		}
		// If item is in bags, swap to equipment slot
		if srcbag != 0 || srcslot >= uint8(equipmentSlotEnd) {
			_ = s.handleSwapInvItem(ctx, []byte{srcbag, srcslot, 0, uint8(i)})
		}
	}

	// Send SMSG_EQUIPMENT_SET_USE_RESULT (0x4D6) with 0 = success
	buf := protocol.NewBuffer(1)
	buf.WriteU8(0) // 0 = ERR_EQUIPMENT_SET_USE_SUCCESS
	_ = s.write(uint16(protocol.OpcodeSMSG_EQUIPMENT_SET_USE_RESULT), buf.Bytes(), true)
	return true
}
