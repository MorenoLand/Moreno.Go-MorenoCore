package world

import (
	"context"
	"time"

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

// handleDismissCritter processes CMSG_DISMISS_CRITTER (0x48D).
func (s *session) handleDismissCritter(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetAbandon processes CMSG_PET_ABANDON (0x176).
// Reference: WorldSession::HandlePetAbandonOpcode (PetHandler.cpp:52).
func (s *session) handlePetAbandon(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetAction processes CMSG_PET_ACTION (0x175).
// Reference: WorldSession::HandlePetAction (PetHandler.cpp:73).
func (s *session) handlePetAction(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetCancelAura processes CMSG_PET_CANCEL_AURA (0x26A).
// Reference: WorldSession::HandlePetCancelAuraOpcode (PetHandler.cpp:215).
func (s *session) handlePetCancelAura(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetCastSpell processes CMSG_PET_CAST_SPELL (0x1F0).
// Reference: WorldSession::HandlePetCastSpellOpcode (PetHandler.cpp:241).
func (s *session) handlePetCastSpell(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetLearnTalent processes CMSG_PET_LEARN_TALENT (0x486).
// Reference: WorldSession::HandlePetLearnTalent (PetHandler.cpp:265).
func (s *session) handlePetLearnTalent(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetNameQuery processes CMSG_PET_NAME_QUERY (0x052).
// Reference: WorldSession::HandlePetNameQuery (PetHandler.cpp:16).
func (s *session) handlePetNameQuery(ctx context.Context, payload []byte) bool {
	if len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	petNumber, _ := r.ReadU32()
	_, _ = r.ReadU64() // petGUID

	petName := "Pet"
	var saveTime uint32
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_ = s.server.CharactersStore.DB.QueryRowContext(ctx,
			"SELECT name, savetime FROM character_pet WHERE id = ? LIMIT 1", petNumber).Scan(&petName, &saveTime)
	}

	buf := protocol.NewBuffer(32 + len(petName))
	buf.WriteU32(petNumber)
	buf.WriteCString(petName)
	buf.WriteU32(saveTime) // timestamp
	buf.WriteU8(0)         // declined
	_ = s.write(uint16(protocol.OpcodeSMSG_PET_NAME_QUERY_RESPONSE), buf.Bytes(), true)
	return true
}

// handlePetRename processes CMSG_PET_RENAME (0x177).
// Reference: WorldSession::HandlePetRenameOpcode (PetHandler.cpp:284).
func (s *session) handlePetRename(ctx context.Context, payload []byte) bool {
	if len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	newName, _ := r.ReadCString()
	if newName == "" {
		return true
	}
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		petNumber := uint32(petGUID & 0xFFFFFF)
		now := time.Now().Unix()
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
			"UPDATE character_pet SET name = ?, renamed = 1, savetime = ? WHERE id = ? AND owner = ?",
			newName, now, petNumber, s.playerGUID)
	}
	return true
}

// handlePetSetAction processes CMSG_PET_SET_ACTION (0x174).
// Reference: WorldSession::HandlePetSetAction (PetHandler.cpp:328).
func (s *session) handlePetSetAction(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetSpellAutocast processes CMSG_PET_SPELL_AUTOCAST (0x1F3).
// Reference: WorldSession::HandlePetSpellAutocastOpcode (PetHandler.cpp:365).
func (s *session) handlePetSpellAutocast(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetStopAttack processes CMSG_PET_STOP_ATTACK (0x2EA).
// Reference: WorldSession::HandlePetStopAttack (PetHandler.cpp:401).
func (s *session) handlePetStopAttack(ctx context.Context, payload []byte) bool {
	return true
}

// handleRequestPetInfo processes CMSG_REQUEST_PET_INFO (0x279).
// Reference: WorldSession::HandleRequestPetInfoOpcode (PetHandler.cpp:412).
func (s *session) handleRequestPetInfo(ctx context.Context, payload []byte) bool {
	return true
}

// handleStablePet processes CMSG_STABLE_PET (0x270).
// Reference: WorldSession::HandleStablePet (NPCHandler.cpp:410).
func (s *session) handleStablePet(ctx context.Context, payload []byte) bool {
	return true
}

// handleStableRevivePet processes CMSG_STABLE_REVIVE_PET (0x274).
// Reference: WorldSession::HandleStableRevivePet (NPCHandler.cpp:455).
func (s *session) handleStableRevivePet(ctx context.Context, payload []byte) bool {
	return true
}

// handleStableSwapPet processes CMSG_STABLE_SWAP_PET (0x275).
// Reference: WorldSession::HandleStableSwapPet (NPCHandler.cpp:478).
func (s *session) handleStableSwapPet(ctx context.Context, payload []byte) bool {
	return true
}

// handleUnstablePet processes CMSG_UNSTABLE_PET (0x271).
// Reference: WorldSession::HandleUnstablePet (NPCHandler.cpp:435).
func (s *session) handleUnstablePet(ctx context.Context, payload []byte) bool {
	return true
}

// handleListStabledPets processes MSG_LIST_STABLED_PETS (0x26F).
// Reference: WorldSession::HandleListStabledPetsOpcode (NPCHandler.cpp:520).
func (s *session) handleListStabledPets(ctx context.Context, payload []byte) bool {
	if len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, _ := r.ReadU64()

	buf := protocol.NewBuffer(16)
	buf.WriteU64(npcGUID)
	buf.WriteU8(0) // num pets
	buf.WriteU8(4) // num slots
	_ = s.write(uint16(protocol.OpcodeMSG_LIST_STABLED_PETS), buf.Bytes(), true)
	return true
}

// handleLearnPreviewTalentsPet processes CMSG_LEARN_PREVIEW_TALENTS_PET (0x4C2).
// Reference: WorldSession::HandleLearnPreviewTalentsPet (PetHandler.cpp:430).
func (s *session) handleLearnPreviewTalentsPet(ctx context.Context, payload []byte) bool {
	return true
}


