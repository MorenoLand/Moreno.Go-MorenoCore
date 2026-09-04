package world

import (
	"context"
	"strconv"
	"strings"
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

// handlePetAction processes CMSG_PET_ACTION (0x175).
// Reference: WorldSession::HandlePetAction (PetHandler.cpp:73-259).
func (s *session) handlePetAction(ctx context.Context, payload []byte) bool {
	if len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	action, err := r.ReadU32()
	if err != nil {
		return false
	}
	var targetGUID uint64
	if len(payload) >= 20 {
		targetGUID, _ = r.ReadU64()
	}

	spellOrAction := action & 0x00FFFFFF
	actFlag := uint8((action >> 24) & 0xFF)

	const (
		actCommand  uint8 = 0x07
		actReaction uint8 = 0x06
		actDisabled uint8 = 0x81
		actEnabled  uint8 = 0xC1
		actPassive  uint8 = 0x01

		commandStay    uint32 = 0
		commandFollow  uint32 = 1
		commandAttack  uint32 = 2
		commandAbandon uint32 = 3

		reactPassive    uint32 = 0
		reactDefensive  uint32 = 1
		reactAggressive uint32 = 2

		aiReactionHostile uint32 = 2
	)

	switch actFlag {
	case actCommand:
		switch spellOrAction {
		case commandAttack:
			// Send hostile AI reaction (plays pet attack sound/growl)
			reactionBuf := protocol.NewBuffer(12)
			reactionBuf.WriteU64(petGUID)
			reactionBuf.WriteU32(aiReactionHostile)
			_ = s.write(uint16(protocol.OpcodeSMSG_AI_REACTION), reactionBuf.Bytes(), true)
			if s.server != nil {
				s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_AI_REACTION), reactionBuf.Bytes(), s)
			}
			s.debug("pet attack command", "account", s.accountName, "pet", petGUID, "target", targetGUID)
		case commandFollow:
			s.debug("pet follow command", "account", s.accountName, "pet", petGUID)
		case commandStay:
			s.debug("pet stay command", "account", s.accountName, "pet", petGUID)
		case commandAbandon:
			if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
				_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_pet WHERE owner = ? AND slot = 0", s.playerGUID)
			}
			buf := protocol.NewBuffer(8)
			buf.WriteU64(0)
			_ = s.write(uint16(protocol.OpcodeSMSG_PET_SPELLS), buf.Bytes(), true)
			s.debug("pet abandoned via command", "account", s.accountName, "pet", petGUID)
		}
	case actReaction:
		// Save react state (0 = passive, 1 = defensive, 2 = aggressive)
		if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_pet SET Reactstate = ? WHERE owner = ? AND slot = 0", spellOrAction, s.playerGUID)
		}
		s.debug("pet reaction state changed", "account", s.accountName, "pet", petGUID, "react", spellOrAction)
	case actDisabled, actEnabled, actPassive:
		if targetGUID != 0 {
			reactionBuf := protocol.NewBuffer(12)
			reactionBuf.WriteU64(petGUID)
			reactionBuf.WriteU32(aiReactionHostile)
			_ = s.write(uint16(protocol.OpcodeSMSG_AI_REACTION), reactionBuf.Bytes(), true)
			if s.server != nil {
				s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_AI_REACTION), reactionBuf.Bytes(), s)
			}
			if spellOrAction != 0 {
				castTimeStamp := uint32(time.Now().UnixMilli())
				spellTarget := protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnitWireMask, UnitGUID: targetGUID}
				goPkt := protocol.BuildSpellGo(petGUID, petGUID, 1, spellOrAction, spellCastFlagGo, castTimeStamp, []uint64{targetGUID}, nil, spellTarget)
				_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), goPkt, true)
				if s.server != nil {
					s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_SPELL_GO), goPkt, s)
				}
			}
		}
		s.debug("pet cast spell action", "account", s.accountName, "pet", petGUID, "spell", spellOrAction, "target", targetGUID)
	}
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
	petGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	castCount, err := r.ReadU8()
	if err != nil {
		return false
	}
	spellID, err := r.ReadU32()
	if err != nil {
		return false
	}
	castFlags, _ := r.ReadU8()
	target, _ := protocol.ReadSpellTargetData(r)
	if castFlags&0x02 != 0 {
		_, _ = r.ReadF32()
		_, _ = r.ReadF32()
	}

	if spellID > 0 {
		castTimeStamp := uint32(time.Now().UnixMilli())
		var hitTargets []uint64
		if target.UnitGUID != 0 {
			hitTargets = []uint64{target.UnitGUID}
		}
		goPkt := protocol.BuildSpellGo(petGUID, petGUID, castCount, spellID, spellCastFlagGo, castTimeStamp, hitTargets, nil, target)
		_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), goPkt, true)
		if s.server != nil {
			s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_SPELL_GO), goPkt, s)
		}
	}
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

	petName := ""
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
	petGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	count := 1
	if len(payload) >= 24 {
		count = 2
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var existingAbdata string
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(abdata, '') FROM character_pet WHERE owner = ? AND slot = 0", s.playerGUID).Scan(&existingAbdata)

	type actionSlot struct {
		actType uint8
		action  uint32
	}
	slots := make([]actionSlot, 10)
	slots[0] = actionSlot{actType: 0x07, action: 2} // Attack
	slots[1] = actionSlot{actType: 0x07, action: 1} // Follow
	slots[2] = actionSlot{actType: 0x07, action: 0} // Stay
	slots[7] = actionSlot{actType: 0x06, action: 2} // Aggressive
	slots[8] = actionSlot{actType: 0x06, action: 1} // Defensive
	slots[9] = actionSlot{actType: 0x06, action: 0} // Passive

	if existingAbdata != "" {
		tokens := strings.Fields(existingAbdata)
		if len(tokens) == 20 {
			for i := 0; i < 10; i++ {
				t, _ := strconv.ParseUint(tokens[i*2], 10, 8)
				a, _ := strconv.ParseUint(tokens[i*2+1], 10, 32)
				slots[i] = actionSlot{actType: uint8(t), action: uint32(a)}
			}
		}
	}

	for i := 0; i < count; i++ {
		pos, pErr := r.ReadU32()
		data, dErr := r.ReadU32()
		if pErr != nil || dErr != nil || pos >= 10 {
			continue
		}
		aType := uint8((data >> 24) & 0xFF)
		aAction := data & 0x00FFFFFF
		slots[pos] = actionSlot{actType: aType, action: aAction}

		// Synchronize pet_spell table when spell action buttons change active state
		petNumber := uint32(petGUID & 0xFFFFFFFF)
		if aAction > 0 && petNumber > 0 {
			if aType == 0xC1 { // ACT_ENABLED (autocast enabled)
				_, _ = cdb.ExecContext(ctx, "UPDATE pet_spell SET active = 1 WHERE guid = ? AND spell = ?", petNumber, aAction)
			} else if aType == 0x81 { // ACT_DISABLED (autocast disabled)
				_, _ = cdb.ExecContext(ctx, "UPDATE pet_spell SET active = 0 WHERE guid = ? AND spell = ?", petNumber, aAction)
			}
		}
	}

	var sb strings.Builder
	for i, sl := range slots {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.FormatUint(uint64(sl.actType), 10))
		sb.WriteByte(' ')
		sb.WriteString(strconv.FormatUint(uint64(sl.action), 10))
	}
	_, _ = cdb.ExecContext(ctx, "UPDATE character_pet SET abdata = ? WHERE owner = ? AND slot = 0", sb.String(), s.playerGUID)
	s.debug("pet set action updated", "account", s.accountName, "pet", petGUID)
	return true
}

// syncPetSpellAutocast updates pet_spell.active and synchronizes the action bar slot in character_pet.abdata.
func (s *session) syncPetSpellAutocast(ctx context.Context, petID uint32, spellID uint32, active uint8) {
	if s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil || petID == 0 || spellID == 0 {
		return
	}
	cdb := s.server.CharactersStore.DB

	// Update pet_spell table
	res, err := cdb.ExecContext(ctx, "UPDATE pet_spell SET active = ? WHERE guid = ? AND spell = ?", active, petID, spellID)
	if err == nil {
		if rows, _ := res.RowsAffected(); rows == 0 {
			_, _ = cdb.ExecContext(ctx, "INSERT INTO pet_spell (guid, spell, active) VALUES (?, ?, ?)", petID, spellID, active)
		}
	}

	// Update abdata if pet is currently active (slot = 0)
	var abdata string
	_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(abdata, '') FROM character_pet WHERE owner = ? AND id = ? AND slot = 0", s.playerGUID, petID).Scan(&abdata)
	if abdata != "" {
		tokens := strings.Fields(abdata)
		if len(tokens) == 20 {
			changed := false
			for i := 0; i < 10; i++ {
				action, _ := strconv.ParseUint(tokens[i*2+1], 10, 32)
				if uint32(action) == spellID {
					newType := uint8(0x81) // ACT_DISABLED
					if active != 0 {
						newType = 0xC1 // ACT_ENABLED
					}
					tokens[i*2] = strconv.FormatUint(uint64(newType), 10)
					changed = true
				}
			}
			if changed {
				newAbdata := strings.Join(tokens, " ")
				_, _ = cdb.ExecContext(ctx, "UPDATE character_pet SET abdata = ? WHERE owner = ? AND id = ?", newAbdata, s.playerGUID, petID)
			}
		}
	}
}

// handlePetSpellAutocast processes CMSG_PET_SPELL_AUTOCAST (0x2F3).
// Reference: WorldSession::HandlePetSpellAutocastOpcode (PetHandler.cpp:694).
func (s *session) handlePetSpellAutocast(ctx context.Context, payload []byte) bool {
	if len(payload) < 13 {
		return true
	}
	r := protocol.NewReader(payload)
	petGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	spellID, err := r.ReadU32()
	if err != nil {
		return false
	}
	state, err := r.ReadU8()
	if err != nil {
		return false
	}
	petNumber := uint32(petGUID & 0xFFFFFFFF)
	s.syncPetSpellAutocast(ctx, petNumber, spellID, state)
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
// Reference: WorldSession::HandleRequestPetInfoOpcode (PetHandler.cpp:412) and Player::SendPetSpells (Player.cpp:21107-21145).
func (s *session) handleRequestPetInfo(ctx context.Context, payload []byte) bool {
	if s.player == nil {
		return true
	}
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		buf := protocol.NewBuffer(8)
		buf.WriteU64(0)
		_ = s.write(uint16(protocol.OpcodeSMSG_PET_SPELLS), buf.Bytes(), true)
		return true
	}

	var petID, entry, modelID, level, reactState int64
	var petName, abdata string
	err := cdb.QueryRowContext(ctx, "SELECT id, entry, modelid, level, name, COALESCE(Reactstate, 1), COALESCE(abdata, '') FROM character_pet WHERE owner = ? AND slot = 0", s.playerGUID).Scan(&petID, &entry, &modelID, &level, &petName, &reactState, &abdata)
	if err != nil {
		// No active pet
		buf := protocol.NewBuffer(8)
		buf.WriteU64(0)
		_ = s.write(uint16(protocol.OpcodeSMSG_PET_SPELLS), buf.Bytes(), true)
		return true
	}

	// Active pet found: build SMSG_PET_SPELLS packet (Player.cpp:21107-21145)
	petGUID := uint64(petID) | (uint64(0xF140) << 48) // HighGuid::Pet = 0xF140
	buf := protocol.NewBuffer(8 + 2 + 4 + 1 + 1 + 2 + 4*10 + 1 + 1)
	buf.WriteU64(petGUID)
	buf.WriteU16(0) // family
	buf.WriteU32(0) // duration (0 = permanent)
	buf.WriteU8(uint8(reactState))  // react state (0 = passive, 1 = defensive, 2 = aggressive)
	buf.WriteU8(1)  // command state (1 = FOLLOW)
	buf.WriteU16(0) // flags

	// Check if custom action bar was saved
	hasCustomAB := false
	var customSlots [10]uint32
	if abdata != "" {
		tokens := strings.Fields(abdata)
		if len(tokens) == 20 {
			hasCustomAB = true
			for i := 0; i < 10; i++ {
				t, _ := strconv.ParseUint(tokens[i*2], 10, 8)
				a, _ := strconv.ParseUint(tokens[i*2+1], 10, 32)
				customSlots[i] = uint32(a) | (uint32(t) << 24)
			}
		}
	}

	if hasCustomAB {
		for i := 0; i < 10; i++ {
			buf.WriteU32(customSlots[i])
		}
	} else {
		// Query pet spells for slots 3..6
		var petSpells []uint32
		rows, sErr := cdb.QueryContext(ctx, "SELECT spell, active FROM pet_spell WHERE guid = ?", petID)
		if sErr == nil {
			defer rows.Close()
			for rows.Next() {
				var spID, active int64
				if rows.Scan(&spID, &active) == nil {
					actType := uint32(0x81) // ACT_DISABLED (castable)
					if active != 0 {
						actType = 0xC1 // ACT_ENABLED (autocast)
					}
					petSpells = append(petSpells, uint32(spID)|(actType<<24))
				}
			}
		}

		// 10 action bar slots (CharmInfo::InitPetActionBar, Unit.cpp:9942)
		buf.WriteU32(0x07000002) // Slot 0: Attack (COMMAND_ATTACK = 2 | ACT_COMMAND = 0x07)
		buf.WriteU32(0x07000001) // Slot 1: Follow (COMMAND_FOLLOW = 1 | ACT_COMMAND = 0x07)
		buf.WriteU32(0x07000000) // Slot 2: Stay   (COMMAND_STAY = 0 | ACT_COMMAND = 0x07)

		for i := 0; i < 4; i++ {
			if i < len(petSpells) {
				buf.WriteU32(petSpells[i])
			} else {
				buf.WriteU32(0)
			}
		}

		buf.WriteU32(0x06000002) // Slot 7: Aggressive (REACT_AGGRESSIVE = 2 | ACT_REACTION = 0x06)
		buf.WriteU32(0x06000001) // Slot 8: Defensive  (REACT_DEFENSIVE = 1 | ACT_REACTION = 0x06)
		buf.WriteU32(0x06000000) // Slot 9: Passive    (REACT_PASSIVE = 0 | ACT_REACTION = 0x06)
	}

	buf.WriteU8(0) // additional spells count
	buf.WriteU8(0) // cooldown count

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
