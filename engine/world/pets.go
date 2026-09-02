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

// StableResultCode mirrors NPCHandler.cpp:48.
const (
	stableErrMoney       uint8 = 0x01 // STABLE_ERR_MONEY
	stableErrStable      uint8 = 0x06 // STABLE_ERR_STABLE
	stableSuccessStable  uint8 = 0x08 // STABLE_SUCCESS_STABLE
	stableSuccessUnslot  uint8 = 0x09 // STABLE_SUCCESS_UNSTABLE
	stableSuccessBuySlot uint8 = 0x0A // STABLE_SUCCESS_BUY_SLOT
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
		res.WriteU8(stableErrStable)
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
	if len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	critterGUID, _ := r.ReadU64()
	s.debug("dismiss critter", "account", s.accountName, "critter", critterGUID)
	return true
}

// handlePetAbandon processes CMSG_PET_ABANDON (0x176).
// Reference: WorldSession::HandlePetAbandonOpcode (PetHandler.cpp:52).
func (s *session) handlePetAbandon(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	// Pet object GUIDs carry the character_pet id in the low 32 bits; the
	// reference deletes the owned pet records outright (Pet::Unsummon with
	// PET_SAVE_AS_DELETED for abandon).
	petID := uint32(petGUID & 0xFFFFFFFF)
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil || petID == 0 {
		return true
	}
	cdb := s.server.CharactersStore.DB
	result, err := cdb.ExecContext(ctx, "DELETE FROM character_pet WHERE owner = ? AND id = ?", s.playerGUID, petID)
	if err == nil {
		if rows, _ := result.RowsAffected(); rows > 0 {
			_, _ = cdb.ExecContext(ctx, "DELETE FROM character_pet_declinedname WHERE owner = ? AND id = ?", s.playerGUID, petID)
			s.debug("pet abandoned", "account", s.accountName, "pet", petID)
		}
	}
	return true
}

// creatureHasNpcFlag resolves a spawned creature's template npcflag, used for
// stablemaster and other service guards.
func (s *session) creatureHasNpcFlag(ctx context.Context, guid uint64, flag uint32) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil || guid == 0 {
		return false
	}
	var npcFlag uint32
	err := s.server.WorldStore.DB.QueryRowContext(ctx,
		"SELECT COALESCE(t.npcflag, 0) FROM creature c JOIN creature_template t ON t.entry = c.id1 WHERE c.guid = ?", guid).Scan(&npcFlag)
	return err == nil && npcFlag&flag != 0
}

// npcFlagStablemaster mirrors UNIT_NPC_FLAG_STABLEMASTER (UnitDefines.h:208).
const npcFlagStablemaster = 0x00400000

// checkStableMaster mirrors WorldSession::CheckStableMaster: the player
// opening their own stable requires GM state, otherwise the target creature
// must be a stablemaster.
func (s *session) checkStableMaster(ctx context.Context, guid uint64) bool {
	if guid == s.playerGUID {
		return s.player.ExtraFlags&playerExtraGMOn != 0
	}
	return s.creatureHasNpcFlag(ctx, guid, npcFlagStablemaster)
}

// sendPetStableResult mirrors WorldSession::SendPetStableResult.
func (s *session) sendPetStableResult(code uint8) {
	buf := protocol.NewBuffer(1)
	buf.WriteU8(code)
	_ = s.write(uint16(protocol.OpcodeSMSG_STABLE_RESULT), buf.Bytes(), true)
}

// handleStablePet processes CMSG_STABLE_PET (0x270).
// Reference: WorldSession::HandleStablePet (NPCHandler.cpp:410): a living
// player stables its active hunter pet (slot 0) into the first free stable
// slot; results flow through SMSG_STABLE_RESULT.
func (s *session) handleStablePet(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	if s.player.Health == 0 || !s.checkStableMaster(ctx, npcGUID) {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	var currentPetType, currentHealth int64
	err = cdb.QueryRowContext(ctx, "SELECT PetType, curhealth FROM character_pet WHERE owner = ? AND slot = 0", s.playerGUID).Scan(&currentPetType, &currentHealth)
	if err != nil || currentPetType != 1 || currentHealth <= 0 { // only living hunter pets
		s.sendPetStableResult(stableErrStable)
		return true
	}
	var freeSlot int64 = -1
	rows, err := cdb.QueryContext(ctx, "SELECT slot FROM character_pet WHERE owner = ? AND slot > 0 ORDER BY slot", s.playerGUID)
	if err == nil {
		taken := map[int64]struct{}{}
		for rows.Next() {
			var slot int64
			if rows.Scan(&slot) == nil {
				taken[slot] = struct{}{}
			}
		}
		rows.Close()
		for slot := int64(1); slot <= 4; slot++ {
			if _, used := taken[slot]; !used {
				freeSlot = slot
				break
			}
		}
	}
	if freeSlot < 0 {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	if _, err := cdb.ExecContext(ctx, "UPDATE character_pet SET slot = ?, savetime = ? WHERE owner = ? AND slot = 0", freeSlot, time.Now().Unix(), s.playerGUID); err != nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	s.sendPetStableResult(stableSuccessStable)
	return true
}

// handleStableRevivePet processes CMSG_STABLE_REVIVE_PET (0x274).
// Reference: WorldSession::HandleStableRevivePet (NPCHandler.cpp:455): the
// stablemaster revives the player's dead current pet.
func (s *session) handleStableRevivePet(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	if !s.checkStableMaster(ctx, npcGUID) {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	var level int64
	if err := cdb.QueryRowContext(ctx, "SELECT level FROM character_pet WHERE owner = ? AND slot = 0 AND curhealth <= 0", s.playerGUID).Scan(&level); err != nil {
		// nothing dead to revive is still a successful interaction
		s.sendPetStableResult(stableSuccessUnslot)
		return true
	}
	if _, err := cdb.ExecContext(ctx, "UPDATE character_pet SET curhealth = 1 WHERE owner = ? AND slot = 0", s.playerGUID); err != nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	s.sendPetStableResult(stableSuccessUnslot)
	return true
}

// handleStableSwapPet processes CMSG_STABLE_SWAP_PET (0x275).
// Reference: WorldSession::HandleStableSwapPet: swap the active pet with a
// stabled one by pet number.
func (s *session) handleStableSwapPet(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	petNumber, err := r.ReadU32()
	if err != nil {
		return false
	}
	if !s.checkStableMaster(ctx, npcGUID) {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	var stabledSlot int64
	err = cdb.QueryRowContext(ctx, "SELECT slot FROM character_pet WHERE owner = ? AND id = ? AND slot > 0", s.playerGUID, petNumber).Scan(&stabledSlot)
	if err != nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	if _, err := cdb.ExecContext(ctx, "UPDATE character_pet SET slot = ? WHERE owner = ? AND slot = 0", stabledSlot, s.playerGUID); err != nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	if _, err := cdb.ExecContext(ctx, "UPDATE character_pet SET slot = 0, savetime = ? WHERE owner = ? AND id = ?", time.Now().Unix(), s.playerGUID, petNumber); err != nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	s.sendPetStableResult(stableSuccessUnslot)
	return true
}

// handleUnstablePet processes CMSG_UNSTABLE_PET (0x272? see dispatch).
// Reference: WorldSession::HandleUnstablePet: move a stabled pet into the
// active slot; an occupied active slot must use the swap opcode instead.
func (s *session) handleUnstablePet(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	petNumber, err := r.ReadU32()
	if err != nil {
		return false
	}
	if !s.checkStableMaster(ctx, npcGUID) {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	var active int64
	if err := cdb.QueryRowContext(ctx, "SELECT COUNT(1) FROM character_pet WHERE owner = ? AND slot = 0", s.playerGUID).Scan(&active); err != nil || active > 0 {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	result, err := cdb.ExecContext(ctx, "UPDATE character_pet SET slot = 0, savetime = ? WHERE owner = ? AND id = ? AND slot > 0", time.Now().Unix(), s.playerGUID, petNumber)
	if err != nil {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		s.sendPetStableResult(stableErrStable)
		return true
	}
	s.sendPetStableResult(stableSuccessUnslot)
	return true
}

// Pet command handlers below require a live pet entity (spawned pet object,
// pet AI, charm info) which the Go server does not model yet; the packets are
// consumed and logged until the pet entity system exists. Reference:
// PetHandler.cpp HandlePetAction (73), HandlePetCancelAuraOpcode (215),
// HandlePetCastSpellOpcode (241), HandlePetLearnTalent (265), HandlePetRename
// (284), HandlePetSetAction (328), HandlePetSpellAutocastOpcode (365),

func (s *session) handlePetAction(ctx context.Context, payload []byte) bool {
	if len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	action, _ := r.ReadU32()
	s.debug("pet action", "account", s.accountName, "pet", petGUID, "action", action)
	return true
}

func (s *session) handlePetCancelAura(ctx context.Context, payload []byte) bool {
	if len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	spellID, _ := r.ReadU32()
	petNumber := uint32(petGUID & 0xFFFFFF)
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM pet_aura WHERE guid = ? AND spell = ?", petNumber, spellID)
	}
	s.debug("pet cancel aura", "account", s.accountName, "pet", petNumber, "spell", spellID)
	return true
}

func (s *session) handlePetCastSpell(ctx context.Context, payload []byte) bool {
	if len(payload) < 13 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	castCount, _ := r.ReadU8()
	spellID, _ := r.ReadU32()
	s.debug("pet cast spell", "account", s.accountName, "pet", petGUID, "spell", spellID, "castCount", castCount)
	return true
}

func (s *session) handlePetLearnTalent(ctx context.Context, payload []byte) bool {
	if len(payload) < 16 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	talentID, _ := r.ReadU32()
	rank, _ := r.ReadU32()
	petNumber := uint32(petGUID & 0xFFFFFF)
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "INSERT OR REPLACE INTO pet_spell (guid, spell, active) VALUES (?, ?, 1)", petNumber, talentID)
	}
	s.debug("pet learn talent", "account", s.accountName, "pet", petNumber, "talent", talentID, "rank", rank)
	return true
}

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
	if len(payload) < 16 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	slot, _ := r.ReadU32()
	action, _ := r.ReadU32()
	s.debug("pet set action", "account", s.accountName, "pet", petGUID, "slot", slot, "action", action)
	return true
}

// handlePetSpellAutocast processes CMSG_PET_SPELL_AUTOCAST (0x1F3).
// Reference: WorldSession::HandlePetSpellAutocastOpcode (PetHandler.cpp:365).
func (s *session) handlePetSpellAutocast(ctx context.Context, payload []byte) bool {
	if len(payload) < 13 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	spellID, _ := r.ReadU32()
	state, _ := r.ReadU8()
	petNumber := uint32(petGUID & 0xFFFFFF)
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE pet_spell SET active = ? WHERE guid = ? AND spell = ?", state, petNumber, spellID)
	}
	s.debug("pet spell autocast", "account", s.accountName, "pet", petNumber, "spell", spellID, "state", state)
	return true
}

// handlePetStopAttack processes CMSG_PET_STOP_ATTACK (0x2EA).
// Reference: WorldSession::HandlePetStopAttack (PetHandler.cpp:401).
func (s *session) handlePetStopAttack(ctx context.Context, payload []byte) bool {
	if len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	s.debug("pet stop attack", "account", s.accountName, "pet", petGUID)
	return true
}

// handleRequestPetInfo processes CMSG_REQUEST_PET_INFO (0x279).
// Reference: WorldSession::HandleRequestPetInfoOpcode (PetHandler.cpp:412).
func (s *session) handleRequestPetInfo(ctx context.Context, payload []byte) bool {
	if s.player == nil {
		return true
	}
	buf := protocol.NewBuffer(8)
	buf.WriteU64(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_PET_SPELLS), buf.Bytes(), true)
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

	type petInfo struct {
		ID    uint32
		Entry uint32
		Level uint32
		Name  string
		Slot  uint8
	}
	var pets []petInfo
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT id, entry, level, name, slot FROM character_pet WHERE owner = ? AND slot > 0 ORDER BY slot", s.playerGUID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var p petInfo
				if err := rows.Scan(&p.ID, &p.Entry, &p.Level, &p.Name, &p.Slot); err == nil {
					pets = append(pets, p)
				}
			}
		}
	}

	buf := protocol.NewBuffer(16 + len(pets)*32)
	buf.WriteU64(npcGUID)
	buf.WriteU8(uint8(len(pets)))
	buf.WriteU8(4) // num slots
	for _, p := range pets {
		buf.WriteU32(p.ID)
		buf.WriteU32(p.Entry)
		buf.WriteU32(p.Level)
		buf.WriteCString(p.Name)
		buf.WriteU8(p.Slot)
	}
	_ = s.write(uint16(protocol.OpcodeMSG_LIST_STABLED_PETS), buf.Bytes(), true)
	return true
}

// handleLearnPreviewTalentsPet processes CMSG_LEARN_PREVIEW_TALENTS_PET (0x4C2).
// Reference: WorldSession::HandleLearnPreviewTalentsPet (PetHandler.cpp:430).
func (s *session) handleLearnPreviewTalentsPet(ctx context.Context, payload []byte) bool {
	if len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, _ := r.ReadU64()
	talentCount, _ := r.ReadU32()
	s.debug("pet preview talents learned", "account", s.accountName, "pet", petGUID, "count", talentCount)
	return true
}
