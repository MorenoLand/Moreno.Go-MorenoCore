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
	var creatureEntry uint32
	if creature := s.luaCreature(ctx, trainerGUID); creature != nil {
		creatureEntry = objectUint32OrZero(creature, "Entry")
	}
	if creatureEntry == 0 {
		creatureEntry = uint32((trainerGUID >> 24) & 0x00FFFFFF)
	}
	if creatureEntry == 0 && s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		spawnGUID := uint32(trainerGUID & 0x00FFFFFF)
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT id FROM creature WHERE guid = ?", spawnGUID).Scan(&creatureEntry)
	}

	var spells []trainerSpellRecord
	greeting := "Hello! Ready for some training?"
	var trainerType uint32 = 0

	// 1. TrinityCore 3.3.5: trainer_spell + trainer + creature_default_trainer
	rows, err := s.server.WorldStore.DB.QueryContext(ctx, `SELECT ts.SpellId, COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqSkillLine, 0), COALESCE(ts.ReqSkillRank, 0), COALESCE(ts.ReqLevel, 0),
		COALESCE(ts.ReqAbility1, 0), COALESCE(ts.ReqAbility2, 0), COALESCE(ts.ReqAbility3, 0),
		COALESCE(t.Type, 0), COALESCE(t.Requirement, 0), COALESCE(t.Greeting, 'Hello! Ready for some training?')
		FROM trainer_spell AS ts
		LEFT JOIN trainer AS t ON t.Id = ts.TrainerId
		WHERE ts.TrainerId IN (SELECT TrainerId FROM creature_default_trainer WHERE CreatureId = ?)
		   OR ts.TrainerId = ?
		ORDER BY ts.ReqLevel, ts.SpellId LIMIT 512`, creatureEntry, creatureEntry)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var spellID, moneyCost, reqSkill, reqSkillValue, reqLevel, reqAb1, reqAb2, reqAb3, tType, tReq int64
			var greet sql.NullString
			if err := rows.Scan(&spellID, &moneyCost, &reqSkill, &reqSkillValue, &reqLevel, &reqAb1, &reqAb2, &reqAb3, &tType, &tReq, &greet); err != nil {
				continue
			}
			if tType == 0 && tReq != 0 && tReq != int64(s.player.Class) {
				continue
			}
			if tType == 1 && tReq != 0 && tReq != int64(s.player.Race) {
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
			} else if reqSkill > 0 && s.getSkillValue(uint32(reqSkill)) < uint32(reqSkillValue) {
				serviceState = 1 // Not Available
			} else if reqAb1 > 0 && !s.hasLearnedSpell(uint32(reqAb1)) {
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
			FROM npc_trainer WHERE ID = ?
			ORDER BY ReqLevel, SpellID LIMIT 128`, creatureEntry)
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
				} else if reqSkill > 0 && s.getSkillValue(uint32(reqSkill)) < uint32(reqSkillValue) {
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
				} else if reqSkill > 0 && s.getSkillValue(uint32(reqSkill)) < uint32(reqSkillValue) {
					serviceState = 1 // Not Available
				} else if reqAb1 > 0 && !s.hasLearnedSpell(uint32(reqAb1)) {
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
	var creatureEntry uint32
	if creature := s.luaCreature(ctx, trainerGUID); creature != nil {
		creatureEntry = objectUint32OrZero(creature, "Entry")
	}
	if creatureEntry == 0 {
		creatureEntry = uint32((trainerGUID >> 24) & 0x00FFFFFF)
	}
	if creatureEntry == 0 && s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		spawnGUID := uint32(trainerGUID & 0x00FFFFFF)
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT id FROM creature WHERE guid = ?", spawnGUID).Scan(&creatureEntry)
	}

	var moneyCost, reqLevel, tType, tReq, reqSkill, reqSkillValue, reqSpell1 int64
	err = wdb.QueryRowContext(ctx, `SELECT COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqLevel, 0), COALESCE(t.Type, 0), COALESCE(t.Requirement, 0),
		COALESCE(ts.ReqSkillLine, 0), COALESCE(ts.ReqSkillRank, 0), COALESCE(ts.ReqAbility1, 0)
		FROM trainer_spell AS ts
		LEFT JOIN trainer AS t ON t.Id = ts.TrainerId
		WHERE (ts.TrainerId IN (SELECT TrainerId FROM creature_default_trainer WHERE CreatureId = ?)
		   OR ts.TrainerId = ?) AND ts.SpellId = ? LIMIT 1`,
		creatureEntry, creatureEntry, spellID).Scan(&moneyCost, &reqLevel, &tType, &tReq, &reqSkill, &reqSkillValue, &reqSpell1)
	if err == nil {
		if tType == 0 && tReq != 0 && tReq != int64(s.player.Class) {
			_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 0), true)
			return true
		}
		if tType == 1 && tReq != 0 && tReq != int64(s.player.Race) {
			_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 0), true)
			return true
		}
	} else if s.player != nil && s.player.Class > 0 {
		// Fallback to class trainer only for player's own class
		err = wdb.QueryRowContext(ctx, `SELECT COALESCE(ts.MoneyCost, 0), COALESCE(ts.ReqLevel, 0),
			COALESCE(ts.ReqSkillLine, 0), COALESCE(ts.ReqSkillRank, 0), COALESCE(ts.ReqAbility1, 0)
			FROM trainer_spell AS ts
			JOIN trainer AS t ON t.Id = ts.TrainerId
			WHERE t.Type = 0 AND t.Requirement = ? AND ts.SpellId = ? LIMIT 1`,
			s.player.Class, spellID).Scan(&moneyCost, &reqLevel, &reqSkill, &reqSkillValue, &reqSpell1)
	}
	if err != nil {
		_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 0), true)
		s.debug("trainer spell not found", "account", s.accountName, "trainer", trainerGUID, "entry", creatureEntry, "spell", spellID)
		return true
	}
	if s.player.Money < uint32(moneyCost) {
		_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 1), true) // NotEnoughMoney = 1
		return true
	}
	if s.player.Level < uint8(reqLevel) {
		_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 2), true) // NotEnoughSkill = 2
		return true
	}
	if reqSkill > 0 && s.getSkillValue(uint32(reqSkill)) < uint32(reqSkillValue) {
		_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 2), true)
		return true
	}
	if reqSpell1 > 0 && !s.hasLearnedSpell(uint32(reqSpell1)) {
		_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED), buildTrainerBuyFailed(trainerGUID, spellID, 2), true)
		return true
	}

	s.player.Money -= uint32(moneyCost)
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)

	// 1. Play visual on trainer NPC (TC: npc->SendPlaySpellVisual(179))
	visualBuf := protocol.NewBuffer(12)
	visualBuf.WriteU64(trainerGUID)
	visualBuf.WriteU32(179) // SpellVisualKit 179: Trainer Teach
	_ = s.write(uint16(protocol.OpcodeSMSG_PLAY_SPELL_VISUAL), visualBuf.Bytes(), true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PLAY_SPELL_VISUAL), visualBuf.Bytes(), s)
	}

	// 2. Play impact on player with sound chime (TC: npc->SendPlaySpellImpact(player->GetGUID(), 362))
	impactBuf := protocol.NewBuffer(12)
	impactBuf.WriteU64(s.playerGUID)
	impactBuf.WriteU32(362) // SpellVisualKit 362: Spell Learn Chime & Impact
	_ = s.write(uint16(protocol.OpcodeSMSG_PLAY_SPELL_IMPACT), impactBuf.Bytes(), true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PLAY_SPELL_IMPACT), impactBuf.Bytes(), s)
	}

	// 3. Learn the spell and handle skill upgrades
	s.learnSpell(ctx, spellID)

	// 4. Check spell_learn_spell for dependent spells (e.g. Find Herbs, Smelting, Campfire, etc.)
	var depSpells []uint32
	depRows, dErr := wdb.QueryContext(ctx, "SELECT SpellID FROM spell_learn_spell WHERE entry = ?", spellID)
	if dErr == nil {
		for depRows.Next() {
			var depSpell uint32
			if err := depRows.Scan(&depSpell); err == nil && depSpell > 0 {
				depSpells = append(depSpells, depSpell)
			}
		}
		_ = depRows.Close()
	}
	for _, depSpell := range depSpells {
		if !s.hasLearnedSpell(depSpell) {
			s.learnSpell(ctx, depSpell)
		}
	}

	// 5. Send teach succeeded (TC: SendTeachSucceeded(npc, player, spellId))
	_ = s.write(uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED), buildTrainerBuySucceeded(trainerGUID, spellID), true)
	s.sendPlayerUpdate()
	_ = s.sendTrainerList(ctx, trainerGUID)
	s.debug("trainer spell learned", "account", s.accountName, "spell", spellID, "cost", moneyCost)
	return true
}

func (s *session) learnSpell(ctx context.Context, spellID uint32) {
	if s.hasLearnedSpell(spellID) {
		return
	}
	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", s.playerGUID, spellID)
	}
	s.player.Spells = append(s.player.Spells, learnedSpell{ID: spellID, Active: true, Disabled: false})

	learnedBuf := protocol.NewBuffer(6)
	learnedBuf.WriteU32(spellID)
	learnedBuf.WriteU16(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_LEARNED_SPELL), learnedBuf.Bytes(), true)

	// Check if spell teaches or upgrades a skill (SPELL_EFFECT_LEARN_SKILL = 118)
	if s.server != nil && s.server.Data != nil {
		if sp, ok, err := s.server.Data.Spell(spellID); err == nil && ok {
			for _, eff := range sp.Effects {
				if eff.Effect == 118 && eff.MiscValue > 0 { // SPELL_EFFECT_LEARN_SKILL
					skillID := uint16(eff.MiscValue)
					tierIdx := eff.BasePoints + 1 // 1=Apprentice, 2=Journeyman, 3=Expert, 4=Artisan, 5=Master, 6=Grand Master
					var maxVal uint16
					switch tierIdx {
					case 1:
						maxVal = 75
					case 2:
						maxVal = 150
					case 3:
						maxVal = 225
					case 4:
						maxVal = 300
					case 5:
						maxVal = 375
					case 6:
						maxVal = 450
					default:
						if tierIdx > 0 {
							maxVal = uint16(tierIdx * 75)
						} else {
							maxVal = 75
						}
					}
					s.setOrUpdateSkill(ctx, skillID, maxVal)
				}
			}
		}
	}
}

func (s *session) setOrUpdateSkill(ctx context.Context, skillID, maxVal uint16) {
	if s.player == nil {
		return
	}
	found := false
	for i := range s.player.Skills {
		if s.player.Skills[i].Skill == skillID {
			found = true
			if s.player.Skills[i].Max < maxVal {
				s.player.Skills[i].Max = maxVal
			}
			if s.player.Skills[i].Value < 1 {
				s.player.Skills[i].Value = 1
			}
			if skillID == 762 { // Riding reaches max value immediately
				s.player.Skills[i].Value = maxVal
			}
			if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
				_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_skills SET value = ?, max = ? WHERE guid = ? AND skill = ?", s.player.Skills[i].Value, s.player.Skills[i].Max, s.playerGUID, skillID)
			}
			break
		}
	}
	if !found {
		val := uint16(1)
		if skillID == 762 { // Riding starts at max
			val = maxVal
		}
		newSk := playerSkill{
			Skill: skillID,
			Step:  1,
			Value: val,
			Max:   maxVal,
			Bonus: 0,
		}
		s.player.Skills = append(s.player.Skills, newSk)
		if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, ?, ?, ?)", s.playerGUID, skillID, val, maxVal)
		}
	}
}

func (s *session) getSkillValue(skillID uint32) uint32 {
	if s.player == nil {
		return 0
	}
	for _, sk := range s.player.Skills {
		if uint32(sk.Skill) == skillID {
			return uint32(sk.Value)
		}
	}
	return 0
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

func buildTrainerBuyFailed(trainerGUID uint64, spellID, reason uint32) []byte {
	buf := protocol.NewBuffer(16)
	buf.WriteU64(trainerGUID)
	buf.WriteU32(spellID)
	buf.WriteU32(reason)
	return buf.Bytes()
}
