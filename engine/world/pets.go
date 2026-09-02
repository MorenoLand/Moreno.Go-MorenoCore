package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

var stableSlotPrices = []uint32{
	500,     // 5s
	50000,   // 5g
	500000,  // 50g
	1500000, // 150g
}

const (
	stableSuccessBuySlot = 8
	stableErrMoney       = 1
	stableErrFull        = 0
)

// handleBuyStableSlot processes CMSG_BUY_STABLE_SLOT (0x272).
// Reference: WorldSession::HandleBuyStableSlot (NPCHandler.cpp:563).
func (s *session) handleBuyStableSlot(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadU64() // npcGUID

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return false
	}

	var purchasedCount int
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(1) FROM character_pet WHERE owner = ? AND slot > 0", s.playerGUID).Scan(&purchasedCount)

	if purchasedCount >= len(stableSlotPrices) {
		res := protocol.NewBuffer(1)
		res.WriteU8(stableErrFull)
		_ = s.write(uint16(protocol.OpcodeSMSG_STABLE_RESULT), res.Bytes(), true)
		return true
	}

	cost := stableSlotPrices[purchasedCount]
	if s.player.Money < cost {
		res := protocol.NewBuffer(1)
		res.WriteU8(stableErrMoney)
		_ = s.write(uint16(protocol.OpcodeSMSG_STABLE_RESULT), res.Bytes(), true)
		return true
	}

	s.player.Money -= cost
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)

	res := protocol.NewBuffer(1)
	res.WriteU8(stableSuccessBuySlot)
	_ = s.write(uint16(protocol.OpcodeSMSG_STABLE_RESULT), res.Bytes(), true)
	s.sendPlayerUpdate()
	s.debug("stable slot purchased", "account", s.accountName, "slot", purchasedCount+1)
	return true
}
