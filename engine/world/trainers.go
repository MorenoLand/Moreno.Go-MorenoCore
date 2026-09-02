package world

import (
	"context"
	"database/sql"

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
	if creatureEntry == 0 {
		creatureEntry = uint32(trainerGUID & 0xFFFFFF)
	}
	var spawnEntry uint32
	spawnGUID := uint32(trainerGUID & 0xFFFFFF)
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT id FROM creature WHERE guid = ?", spawnGUID).Scan(&spawnEntry); err == nil && spawnEntry != 0 {
		creatureEntry = spawnEntry
	}

	var spells []trainerSpellRecord
	greeting := "Hello! Ready for some training?"
	var trainerType uint32 = 0

	// 1. TrinityCore 3.3.5: trainer_spell + trainer + creature_default_trainer
	rows, err := s.server.WorldStore.DB.QueryContext(ctx, `SELECT ts.SpellId, COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqSkillLine, 0), COALESCE(ts.ReqSkillRank, 0), COALESCE(ts.ReqLevel, 0),
		COALESCE(ts.ReqAbility1, 0), COALESCE(ts.ReqAbility2, 0), COALESCE(ts.ReqAbility3, 0),
		COALESCE(t.Type, 0), COALESCE(t.Greeting, 'Hello! Ready for some training?')
		FROM trainer_spell AS ts
		LEFT JOIN trainer AS t ON t.Id = ts.TrainerId
		WHERE ts.TrainerId = (SELECT TrainerId FROM creature_default_trainer WHERE CreatureId = ? LIMIT 1)
		   OR ts.TrainerId = (SELECT trainer_id FROM creature_template WHERE entry = ? LIMIT 1)
		   OR ts.TrainerId = (SELECT trainer_spell FROM creature_template WHERE entry = ? LIMIT 1)
		   OR ts.TrainerId = ?
		ORDER BY ts.ReqLevel, ts.SpellId LIMIT 128`, creatureEntry, creatureEntry, creatureEntry, creatureEntry)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var spellID, moneyCost, reqSkill, reqSkillValue, reqLevel, reqAb1, reqAb2, reqAb3, tType int64
			var greet sql.NullString
			if err := rows.Scan(&spellID, &moneyCost, &reqSkill, &reqSkillValue, &reqLevel, &reqAb1, &reqAb2, &reqAb3, &tType, &greet); err != nil {
				continue
			}
			if greet.Valid && greet.String != "" {
				greeting = greet.String
			}
			trainerType = uint32(tType)
			serviceState := uint8(0) // Available
			if s.hasLearnedSpell(uint32(spellID)) {
				serviceState = 2 // Already Known
			} else if s.player.Level < uint8(reqLevel) {
				serviceState = 1 // Not Available
			}
			spells = append(spells, trainerSpellRecord{
				SpellID:       uint32(spellID),
				ServiceState:  serviceState,
				Cost:          uint32(moneyCost),
				ReqLevel:      uint8(reqLevel),
				ReqSkill:      uint32(reqSkill),
				ReqSkillValue: uint32(reqSkillValue),
				ReqSpell:      uint32(reqAb1),
				ReqSpellChain: uint32(reqAb2),
				ReqSpell3:     uint32(reqAb3),
			})
		}
	}

	// 2. Fallback to legacy npc_trainer if trainer_spell returned no rows
	if len(spells) == 0 {
		fbRows, fbErr := s.server.WorldStore.DB.QueryContext(ctx, `SELECT SpellID, COALESCE(MoneyCost, 0), COALESCE(ReqSkill, 0), COALESCE(ReqSkillValue, 0), COALESCE(ReqLevel, 0)
			FROM npc_trainer WHERE ID = ? OR ID = (SELECT trainer_id FROM creature_template WHERE entry = ? LIMIT 1)
			ORDER BY ReqLevel, SpellID LIMIT 128`, creatureEntry, creatureEntry)
		if fbErr == nil {
			defer fbRows.Close()
			for fbRows.Next() {
				var spellID, moneyCost, reqSkill, reqSkillValue, reqLevel int64
				if err := fbRows.Scan(&spellID, &moneyCost, &reqSkill, &reqSkillValue, &reqLevel); err != nil {
					continue
				}
				serviceState := uint8(0)
				if s.hasLearnedSpell(uint32(spellID)) {
					serviceState = 2
				} else if s.player.Level < uint8(reqLevel) {
					serviceState = 1
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
		}
	}

	// 3. Fallback to class trainer matching player's class (TrinityCore _classTrainers)
	if len(spells) == 0 && s.player != nil && s.player.Class > 0 {
		ctRows, ctErr := s.server.WorldStore.DB.QueryContext(ctx, `SELECT ts.SpellId, COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqSkillLine, 0), COALESCE(ts.ReqSkillRank, 0), COALESCE(ts.ReqLevel, 0),
			COALESCE(ts.ReqAbility1, 0), COALESCE(ts.ReqAbility2, 0), COALESCE(ts.ReqAbility3, 0),
			COALESCE(t.Type, 0), COALESCE(t.Greeting, 'Hello! Ready for some training?')
			FROM trainer_spell AS ts
			JOIN trainer AS t ON t.Id = ts.TrainerId
			WHERE t.Type = 0 AND t.Requirement = ?
			ORDER BY ts.ReqLevel, ts.SpellId LIMIT 128`, s.player.Class)
		if ctErr == nil {
			defer ctRows.Close()
			for ctRows.Next() {
				var spellID, moneyCost, reqSkill, reqSkillValue, reqLevel, reqAb1, reqAb2, reqAb3, tType int64
				var greet sql.NullString
				if err := ctRows.Scan(&spellID, &moneyCost, &reqSkill, &reqSkillValue, &reqLevel, &reqAb1, &reqAb2, &reqAb3, &tType, &greet); err != nil {
					continue
				}
				if greet.Valid && greet.String != "" {
					greeting = greet.String
				}
				trainerType = uint32(tType)
				serviceState := uint8(0) // Available
				if s.hasLearnedSpell(uint32(spellID)) {
					serviceState = 2 // Already Known
				} else if s.player.Level < uint8(reqLevel) {
					serviceState = 1 // Not Available
				}
				spells = append(spells, trainerSpellRecord{
					SpellID:       uint32(spellID),
					ServiceState:  serviceState,
					Cost:          uint32(moneyCost),
					ReqLevel:      uint8(reqLevel),
					ReqSkill:      uint32(reqSkill),
					ReqSkillValue: uint32(reqSkillValue),
					ReqSpell:      uint32(reqAb1),
					ReqSpellChain: uint32(reqAb2),
					ReqSpell3:     uint32(reqAb3),
				})
			}
		}
	}

	packet := protocol.NewBuffer(8 + 4 + 4 + len(spells)*38 + len(greeting) + 1)
	packet.WriteU64(trainerGUID)
	packet.WriteU32(trainerType)
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
	packet.WriteCString(greeting)
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
	if creatureEntry == 0 {
		creatureEntry = uint32(trainerGUID & 0xFFFFFF)
	}
	var spawnEntry uint32
	spawnGUID := uint32(trainerGUID & 0xFFFFFF)
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT id FROM creature WHERE guid = ?", spawnGUID).Scan(&spawnEntry); err == nil && spawnEntry != 0 {
		creatureEntry = spawnEntry
	}

	var moneyCost, reqLevel int64
	err = wdb.QueryRowContext(ctx, `SELECT COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqLevel, 0)
		FROM trainer_spell AS ts
		WHERE (ts.TrainerId = (SELECT TrainerId FROM creature_default_trainer WHERE CreatureId = ? LIMIT 1)
		   OR ts.TrainerId = (SELECT trainer_id FROM creature_template WHERE entry = ? LIMIT 1)
		   OR ts.TrainerId = (SELECT trainer_spell FROM creature_template WHERE entry = ? LIMIT 1)
		   OR ts.TrainerId = ?) AND ts.SpellId = ? LIMIT 1`,
		creatureEntry, creatureEntry, creatureEntry, creatureEntry, spellID).Scan(&moneyCost, &reqLevel)
	if err != nil {
		// Fallback to legacy npc_trainer
		err = wdb.QueryRowContext(ctx, `SELECT COALESCE(MoneyCost, 0), COALESCE(ReqLevel, 0)
			FROM npc_trainer WHERE (ID = ? OR ID = (SELECT trainer_id FROM creature_template WHERE entry = ? LIMIT 1)) AND SpellID = ? LIMIT 1`,
			creatureEntry, creatureEntry, spellID).Scan(&moneyCost, &reqLevel)
		if err != nil && s.player != nil && s.player.Class > 0 {
			// Fallback to class trainer
			err = wdb.QueryRowContext(ctx, `SELECT COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqLevel, 0)
				FROM trainer_spell AS ts
				JOIN trainer AS t ON t.Id = ts.TrainerId
				WHERE t.Type = 0 AND t.Requirement = ? AND ts.SpellId = ? LIMIT 1`,
				s.player.Class, spellID).Scan(&moneyCost, &reqLevel)
		}
		if err != nil {
			return true
		}
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
