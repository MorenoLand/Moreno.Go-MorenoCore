package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	maxTalentsCount   uint32 = 150
	maxTalentRank     uint32 = 5
	maxGlyphSlotIndex uint8  = 6
)

// freeTalentPoints mirrors Player::GetFreeTalentPoints (Player.cpp:25462).
// Level < 10 grants 0 talent points.
// Level >= 10 grants (level - 9) talent points minus all spent points.
func (s *session) freeTalentPoints() uint32 {
	if s.player == nil || s.player.Level < 10 {
		return 0
	}
	totalPoints := uint32(s.player.Level - 9)
	var spent uint32
	for _, rank := range s.player.Talents {
		spent += uint32(rank + 1)
	}
	if spent >= totalPoints {
		return 0
	}
	return totalPoints - spent
}

// learnTalent mirrors Player::LearnTalent (Player.cpp:25460).
func (s *session) learnTalent(ctx context.Context, talentID, requestedRank uint32) bool {
	if s.player == nil || requestedRank >= maxTalentRank {
		return false
	}
	if s.player.Talents == nil {
		s.player.Talents = make(map[uint32]uint8)
	}
	curRank, has := s.player.Talents[talentID]
	if has && curRank >= uint8(requestedRank) {
		return false
	}
	neededPoints := requestedRank + 1
	if has {
		neededPoints = requestedRank - uint32(curRank)
	}
	if s.freeTalentPoints() < neededPoints {
		return false
	}

	var oldSpellID uint32
	if has && s.server != nil && s.server.Data != nil {
		if tEntry, ok, err := s.server.Data.Talent(talentID); err == nil && ok && uint32(curRank) < uint32(len(tEntry.SpellRank)) {
			oldSpellID = tEntry.SpellRank[curRank]
		}
	}

	var spellID uint32
	if s.server != nil && s.server.Data != nil {
		if tEntry, ok, err := s.server.Data.Talent(talentID); err == nil && ok && requestedRank < uint32(len(tEntry.SpellRank)) {
			spellID = tEntry.SpellRank[requestedRank]
		}
	}
	if spellID == 0 {
		spellID = talentID*10 + requestedRank + 1
	}

	s.player.Talents[talentID] = uint8(requestedRank)

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		if oldSpellID > 0 {
			_, _ = cdb.ExecContext(ctx, "DELETE FROM character_talent WHERE guid = ? AND spell = ? AND talentGroup = ?", s.playerGUID, oldSpellID, s.player.ActiveTalentGroup)
			_, _ = cdb.ExecContext(ctx, "DELETE FROM character_spell WHERE guid = ? AND spell = ?", s.playerGUID, oldSpellID)
		}
		_, _ = cdb.ExecContext(ctx, "INSERT INTO character_talent (guid, spell, talentGroup) VALUES (?, ?, ?)", s.playerGUID, spellID, s.player.ActiveTalentGroup)
		_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", s.playerGUID, spellID)
	}
	return true
}

func (s *session) loadTalentsForGroup(group uint8) map[uint32]uint8 {
	talents := make(map[uint32]uint8)
	if s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return talents
	}
	cdb := s.server.CharactersStore.DB
	rows, err := cdb.Query("SELECT spell FROM character_talent WHERE guid = ? AND talentGroup = ?", s.playerGUID, group)
	if err != nil {
		return talents
	}
	defer rows.Close()
	for rows.Next() {
		var spellID int64
		if err := rows.Scan(&spellID); err == nil && spellID > 0 {
			var tid uint32
			var r uint8
			var found bool
			if s.server.Data != nil {
				tid, r, found = s.server.Data.TalentBySpell(uint32(spellID))
			}
			if !found && spellID > 10 {
				tid = uint32((spellID - 1) / 10)
				r = uint8((spellID - 1) % 10)
				found = true
			}
			if found {
				talents[tid] = r
			}
		}
	}
	return talents
}

// sendTalentsInfo mirrors Player::SendTalentsInfoData (Player.cpp:25909).
func (s *session) sendTalentsInfo(pet bool) error {
	buf := protocol.NewBuffer(64)
	if pet {
		buf.WriteU8(1)
		buf.WriteU32(0) // unspentTalentPoints
		buf.WriteU8(0)  // talentCount
		return s.write(uint16(protocol.OpcodeSMSG_TALENTS_INFO), buf.Bytes(), true)
	}

	buf.WriteU8(0)
	buf.WriteU32(s.freeTalentPoints())
	specsCount := s.player.TalentGroupsCount
	if specsCount == 0 {
		specsCount = 1
	}
	buf.WriteU8(specsCount)
	buf.WriteU8(s.player.ActiveTalentGroup)

	for spec := uint8(0); spec < specsCount; spec++ {
		talents := s.player.Talents
		if spec != s.player.ActiveTalentGroup {
			talents = s.loadTalentsForGroup(spec)
		}
		talentCount := uint8(len(talents))
		buf.WriteU8(talentCount)
		for tid, rank := range talents {
			buf.WriteU32(tid)
			buf.WriteU8(rank)
		}
		buf.WriteU8(maxGlyphSlotIndex)
		for i := uint8(0); i < maxGlyphSlotIndex; i++ {
			glyphID := uint16(0)
			if int(spec) < len(s.player.Glyphs) && int(i) < len(s.player.Glyphs[spec]) {
				glyphID = s.player.Glyphs[spec][i]
			}
			buf.WriteU16(glyphID)
		}
	}
	return s.write(uint16(protocol.OpcodeSMSG_TALENTS_INFO), buf.Bytes(), true)
}

// handleLearnTalent processes CMSG_LEARN_TALENT (0x251).
// Reference: WorldSession::HandleLearnTalentOpcode (SkillHandler.cpp:27).
func (s *session) handleLearnTalent(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	talentID, err := r.ReadU32()
	if err != nil {
		return false
	}
	requestedRank, err := r.ReadU32()
	if err != nil {
		return false
	}

	s.learnTalent(ctx, talentID, requestedRank)
	_ = s.sendTalentsInfo(false)
	return true
}

// handleLearnPreviewTalents processes CMSG_LEARN_PREVIEW_TALENTS (0x4C1).
// Reference: WorldSession::HandleLearnPreviewTalents (SkillHandler.cpp:36).
func (s *session) handleLearnPreviewTalents(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	talentsCount, err := r.ReadU32()
	if err != nil {
		return false
	}

	for i := uint32(0); i < talentsCount && i < maxTalentsCount; i++ {
		talentID, err := r.ReadU32()
		if err != nil {
			return false
		}
		talentRank, err := r.ReadU32()
		if err != nil {
			return false
		}
		s.learnTalent(ctx, talentID, talentRank)
	}

	_ = s.sendTalentsInfo(false)
	return true
}

// handleUnlearnSkill processes CMSG_UNLEARN_SKILL (0x202).
// Reference: WorldSession::HandleUnlearnSkillOpcode (SkillHandler.cpp:93).
func (s *session) handleUnlearnSkill(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	skillID, err := r.ReadU32()
	if err != nil {
		return false
	}

	newSkills := make([]playerSkill, 0, len(s.player.Skills))
	for _, sk := range s.player.Skills {
		if uint32(sk.Skill) != skillID {
			newSkills = append(newSkills, sk)
		}
	}
	s.player.Skills = newSkills

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_skills WHERE guid = ? AND skill = ?", s.playerGUID, skillID)
	}

	s.sendPlayerUpdate()
	return true
}
