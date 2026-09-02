package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleAlterAppearance processes CMSG_ALTER_APPEARANCE (0x426).
// Reference: WorldSession::HandleAlterAppearance (CharacterHandler.cpp:1274).
func (s *session) handleAlterAppearance(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	hair, err := r.ReadU32()
	if err != nil {
		return false
	}
	color, err := r.ReadU32()
	if err != nil {
		return false
	}
	facialHair, err := r.ReadU32()
	if err != nil {
		return false
	}
	skinColor, err := r.ReadU32()
	if err != nil {
		return false
	}

	s.player.HairStyle = uint8(hair)
	s.player.HairColor = uint8(color)
	s.player.FacialStyle = uint8(facialHair)
	if skinColor > 0 {
		s.player.Skin = uint8(skinColor)
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET hairStyle = ?, hairColor = ?, facialStyle = ?, skin = ? WHERE guid = ?",
			s.player.HairStyle, s.player.HairColor, s.player.FacialStyle, s.player.Skin, s.playerGUID)
	}

	res := protocol.NewBuffer(4)
	res.WriteU32(0) // BARBER_SHOP_RESULT_SUCCESS
	_ = s.write(uint16(protocol.OpcodeSMSG_BARBER_SHOP_RESULT), res.Bytes(), true)
	s.sendPlayerUpdate()
	s.debug("alter appearance applied", "account", s.accountName, "hair", hair, "color", color)
	return true
}
