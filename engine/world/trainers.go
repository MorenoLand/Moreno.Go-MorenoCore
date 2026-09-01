package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type trainerSpellRecord struct {
	SpellID       uint32
	ServiceState  uint8
	Cost          uint32
	TalentCost    uint32
	PrimaryProf   uint32
	ReqLevel      uint8
	ReqSkill      uint32
	ReqSkillValue uint32
	ReqSpell      uint32
	ReqSpellChain uint32
	ReqSpell3     uint32
}

func (s *session) handleTrainerList(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	trainerGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	return s.sendTrainerList(ctx, trainerGUID)
}

func (s *session) sendTrainerList(ctx context.Context, trainerGUID uint64) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return true
	}
	creatureEntry := uint32((trainerGUID >> 24) & 0xFFFFFF)
	rows, err := s.server.WorldStore.DB.QueryContext(ctx, `SELECT SpellID, COALESCE(MoneyCost, 0), COALESCE(ReqSkill, 0), COALESCE(ReqSkillValue, 0), COALESCE(ReqLevel, 0)
		FROM npc_trainer WHERE ID = ? OR ID = (SELECT trainer_id FROM creature_template WHERE entry = ? LIMIT 1)
		ORDER BY ReqLevel, SpellID LIMIT 128`, creatureEntry, creatureEntry)
	if err != nil {
		return true
	}
	defer rows.Close()
	var spells []trainerSpellRecord
	for rows.Next() {
		var spellID, moneyCost, reqSkill, reqSkillValue, reqLevel int64
		if err := rows.Scan(&spellID, &moneyCost, &reqSkill, &reqSkillValue, &reqLevel); err != nil {
			continue
		}
		serviceState := uint8(0) // Available
		if s.hasLearnedSpell(uint32(spellID)) {
			serviceState = 2 // Already Known
		} else if s.player.Level < uint8(reqLevel) || s.player.Money < uint32(moneyCost) {
			serviceState = 1 // Not Available
		}
		spells = append(spells, trainerSpellRecord{
			SpellID:       uint32(spellID),
			ServiceState:  serviceState,
			Cost:          uint32(moneyCost),
			ReqLevel:      uint8(reqLevel),
			ReqSkill:      uint32(reqSkill),
			ReqSkillValue: uint32(reqSkillValue),
		})
	}
	greeting := "Hello! Ready for some training?"
	packet := protocol.NewBuffer(8 + 4 + 4 + len(spells)*38 + len(greeting) + 1)
	packet.WriteU64(trainerGUID)
	packet.WriteU32(0) // TrainerType = 0 (Class/General)
	packet.WriteU32(uint32(len(spells)))
	for _, sp := range spells {
		packet.WriteU32(sp.SpellID)
		packet.WriteU8(sp.ServiceState)
		packet.WriteU32(sp.Cost)
		packet.WriteU32(sp.TalentCost)
		packet.WriteU32(sp.PrimaryProf)
		packet.WriteU8(sp.ReqLevel)
		packet.WriteU32(sp.ReqSkill)
		packet.WriteU32(sp.ReqSkillValue)
		packet.WriteU32(sp.ReqSpell)
		packet.WriteU32(sp.ReqSpellChain)
		packet.WriteU32(sp.ReqSpell3)
	}
	packet.WriteString(greeting)
	_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_LIST), packet.Bytes(), true)
	s.debug("trainer list sent", "account", s.accountName, "trainer", trainerGUID, "spells", len(spells))
	return true
}

func (s *session) handleTrainerBuySpell(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	reader := protocol.NewReader(payload)
	trainerGUID, err := reader.ReadU64()
	if err != nil {
		return false
	}
	spellID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if s.hasLearnedSpell(spellID) {
		return true
	}
	wdb := s.server.WorldStore.DB
	cdb := s.server.CharactersStore.DB
	if wdb == nil || cdb == nil {
		return true
	}
	creatureEntry := uint32((trainerGUID >> 24) & 0xFFFFFF)
	var moneyCost, reqLevel int64
	err = wdb.QueryRowContext(ctx, `SELECT COALESCE(MoneyCost, 0), COALESCE(ReqLevel, 0)
		FROM npc_trainer WHERE (ID = ? OR ID = (SELECT trainer_id FROM creature_template WHERE entry = ? LIMIT 1)) AND SpellID = ? LIMIT 1`,
		creatureEntry, creatureEntry, spellID).Scan(&moneyCost, &reqLevel)
	if err != nil {
		return true
	}
	if s.player.Money < uint32(moneyCost) || s.player.Level < uint8(reqLevel) {
		return true
	}
	s.player.Money -= uint32(moneyCost)
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", s.playerGUID, spellID)
	s.player.Spells = append(s.player.Spells, learnedSpell{ID: spellID, Active: true, Disabled: false})
	_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED), buildTrainerBuySucceeded(trainerGUID, spellID), true)
	s.sendPlayerUpdate()
	s.debug("trainer spell learned", "account", s.accountName, "spell", spellID, "cost", moneyCost)
	return true
}

func (s *session) hasLearnedSpell(spellID uint32) bool {
	for _, sp := range s.player.Spells {
		if sp.ID == spellID {
			return true
		}
	}
	return false
}

func buildTrainerBuySucceeded(trainerGUID uint64, spellID uint32) []byte {
	buf := protocol.NewBuffer(12)
	buf.WriteU64(trainerGUID)
	buf.WriteU32(spellID)
	return buf.Bytes()
}
