package world

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	playerValuesCount                  = 1326
	objectFieldType                    = 2
	objectFieldEntry                   = 3
	objectFieldScale                   = 4
	unitFieldHealth                    = 24
	unitFieldLevel                     = 54
	unitFieldFaction                   = 55
	unitFieldFlags                     = 59
	unitFieldAttackTime                = 62
	unitFieldAttackTimeOffhand         = 63
	unitFieldBoundingRadius            = 65
	unitFieldCombatReach               = 66
	unitFieldDisplayID                 = 67
	unitFieldNativeDisplayID           = 68
	unitFieldPlayerFlags               = 150
	unitFieldPlayerBytes               = 153
	unitFieldPlayerBytes2              = 154
	unitFieldPlayerBytes3              = 155
	unitFieldGuildID                   = 151
	unitFieldGuildRank                 = 152
	unitFieldGuildTimestamp            = 157
	unitFieldXP                        = 634
	unitFieldNextLevelXP               = 635
	unitFieldCoinage                   = 1170
	unitFieldMaxLevel                  = 1279
	unitFieldKnownCurrencies           = 632
	unitFieldWatchedFaction            = 1230
	unitFieldAmmoID                    = 1198
	unitFieldPlayerSelfResSpell        = 1199 // PLAYER_SELF_RES_SPELL = UNIT_END + 0x041B
	playerQuestLogStart                = 158  // PLAYER_QUEST_LOG_1_1; stride 5 per TC MAX_QUEST_OFFSET
	playerQuestLogSlots                = 25
	playerSkillInfoStart               = 636
	playerMaxSkills                    = 128
	playerVisibleItemStart             = 283
	playerVisibleItemCount             = 19
	unitFieldMaxHealth                 = 32
	unitFieldMaxPower1                 = 33
	unitFieldRangedAttackTime          = 64
	unitFieldMinDamage                 = 70
	unitFieldMaxDamage                 = 71
	unitFieldMinOffhandDamage          = 72
	unitFieldMaxOffhandDamage          = 73
	unitModCastSpeed                   = 80
	unitFieldStat0                     = 84 // Strength
	unitFieldStat1                     = 85 // Agility
	unitFieldStat2                     = 86 // Stamina
	unitFieldStat3                     = 87 // Intellect
	unitFieldStat4                     = 88 // Spirit
	unitFieldPosStat0                  = 89
	unitFieldNegStat0                  = 94
	unitFieldResistances               = 99 // 99..105 (Physical/Armor, Holy, Fire, Nature, Frost, Shadow, Arcane)
	unitFieldBaseMana                  = 120
	unitFieldBaseHealth                = 121
	unitFieldAttackPower               = 123
	unitFieldAttackPowerMods           = 124
	unitFieldAttackPowerMultiplier     = 125
	unitFieldRangedAttackPower         = 126
	unitFieldRangedAttackPowerMods     = 127
	unitFieldRangedAttackPowerMultiplier = 128
	unitFieldMinRangedDamage           = 129
	unitFieldMaxRangedDamage           = 130
	playerCharacterPoints1             = 1020 // Free talent points
	playerCharacterPoints2             = 1021
	playerBlockPercentage              = 1024
	playerDodgePercentage              = 1025
	playerParryPercentage              = 1026
	playerCritPercentage               = 1029
	playerRangedCritPercentage         = 1030
	playerOffhandCritPercentage        = 1031
	playerSpellCritPercentage1         = 1032 // 1032..1038
	playerShieldBlock                  = 1039
	playerFieldModDamageDonePos        = 1171 // 1171..1177
	playerFieldModDamageDoneNeg        = 1178 // 1178..1184
	playerFieldModDamageDonePct        = 1185 // 1185..1191
	playerFieldModHealingDonePos       = 1192
	playerFieldModHealingPct           = 1193
	playerFieldModHealingDonePct       = 1194
	playerFieldCombatRating1           = 1231 // 1231..1255
	unitFlagPlayerControlled    uint32 = 0x00000008
)

// questCompleteStateFlag sets the per-slot complete bit the client reads
// from PLAYER_QUEST_LOG_x_2 (nonzero marks objectives complete).
func questCompleteStateFlag(status int64) uint32 {
	if status == questStatusComplete {
		return 1
	}
	return 0
}

// questLogEntry mirrors one PLAYER_QUEST_LOG slot: quest id, state byte,
// four objective counters packed into two uint32 fields and the timer.
type questLogEntry struct {
	QuestID  uint32
	State    uint32
	Timer    uint32
	Counters [4]uint16
}

type playerSkill struct {
	Skill uint16
	Step  uint16
	Value uint16
	Max   uint16
	Bonus uint16
}

type playerState struct {
	GUID             uint64
	Name             string
	Race             uint8
	Class            uint8
	Gender           uint8
	Skin             uint8
	Face             uint8
	HairStyle        uint8
	HairColor        uint8
	FacialStyle      uint8
	Level            uint8
	XP               uint32
	Money            uint32
	PlayerFlags      uint32
	GuildID          uint32
	GuildRank        uint8
	Map              uint32
	X                float32
	Y                float32
	Z                float32
	Orientation      float32
	ExtraFlags       uint32
	AtLogin          uint32
	Zone             uint32
	Health           uint32
	MaxHealth        uint32
	Powers           [7]uint32
	MaxPowers        [7]uint32
	Cinematic        uint32
	KnownCurrency    uint32
	WatchedFaction   uint32
	AmmoID           uint32
	ActionBars       uint32
	Skills           []playerSkill
	Spells           []learnedSpell
	Actions          [144]uint32
	Cooldowns        []spellCooldown
	Equipment        string
	SheathState      uint8
	TaxiMask         [taxiMaskSize]uint32
	QuestLog         [playerQuestLogSlots]questLogEntry
	MountDisplayID   uint32
	StandState       uint8
	PlayerFieldBytes uint32
	SelfResSpell     uint32
	HomebindMap      uint32
	HomebindZone     uint32
	HomebindX        float32
	HomebindY        float32
	HomebindZ        float32
	Reputations      []playerReputation
	Talents          map[uint32]uint8
}

type playerReputation struct {
	FactionID uint32
	ListID    uint32
	Standing  int32
	Base      int32
	Flags     uint8
}

func (s *session) loadPlayerState(ctx context.Context, guid uint64) (playerState, error) {
	state := playerState{GUID: guid, Health: 1, MaxHealth: 1}
	var race, class, gender, level, playerFlags, mapID, extraFlags, atLogin, zone, deathExpireTime int64
	var equipment sql.NullString
	if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT guid, name, race, class, gender, level, playerFlags, map, position_x, position_y, position_z, orientation, extra_flags, at_login, zone, equipmentCache, death_expire_time FROM characters WHERE guid = ? AND account = ?", guid, s.accountID).Scan(&state.GUID, &state.Name, &race, &class, &gender, &level, &playerFlags, &mapID, &state.X, &state.Y, &state.Z, &state.Orientation, &extraFlags, &atLogin, &zone, &equipment, &deathExpireTime); err != nil {
		return playerState{}, err
	}
	s.deathExpireTime = deathExpireTime
	state.Race, state.Class, state.Gender, state.Level = uint8(race), uint8(class), uint8(gender), uint8(level)
	state.PlayerFlags, state.Map, state.ExtraFlags, state.AtLogin, state.Zone = uint32(playerFlags), uint32(mapID), uint32(extraFlags), uint32(atLogin), uint32(zone)
	if equipment.Valid {
		state.Equipment = equipment.String
	}
	// TrinityCore LoadFromDB/InitStatsForLevel cleans transient player flags
	// (AFK/DND/GM/GHOST) before GM state is re-applied from extra_flags per
	// GM.LoginState (0 off, 1 on, 2 saved state).
	transient := uint32(playerFlagAFK | playerFlagDND | playerFlagGM | playerFlagGhost | playerFlagAllowOnlyAbility)
	state.PlayerFlags &= ^transient
	loginState := 2
	if s.server.Config.GMLoginState >= 0 && s.server.Config.GMLoginState <= 2 {
		loginState = s.server.Config.GMLoginState
	}
	if loginState == 1 || (loginState == 2 && state.ExtraFlags&playerExtraGMOn != 0) {
		state.ExtraFlags |= playerExtraGMOn
		state.PlayerFlags |= playerFlagGM
	} else {
		state.ExtraFlags &= ^playerExtraGMOn
		state.PlayerFlags &= ^playerFlagGM
	}
	var homeMap, homeZone int64
	var homeX, homeY, homeZ float32
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT mapId, zoneId, posX, posY, posZ FROM character_homebind WHERE guid = ?", guid).Scan(&homeMap, &homeZone, &homeX, &homeY, &homeZ); err == nil {
			state.HomebindMap = uint32(homeMap)
			state.HomebindZone = uint32(homeZone)
			state.HomebindX = homeX
			state.HomebindY = homeY
			state.HomebindZ = homeZ
		}
	}
	if state.HomebindMap == 0 && state.HomebindX == 0 && state.HomebindY == 0 && state.HomebindZ == 0 {
		state.HomebindMap = state.Map
		state.HomebindZone = state.Zone
		state.HomebindX = state.X
		state.HomebindY = state.Y
		state.HomebindZ = state.Z
	}
	state.Talents = make(map[uint32]uint8)
	s.player = &state

	// Rebuild the quest log slots, taxi masks, guild info, skills, and packet states concurrently
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			qRows, qErr := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT quest, status, explored, timer, mobcount1, mobcount2, mobcount3, mobcount4 FROM character_queststatus WHERE guid = ? AND status IN (1, 3) ORDER BY quest", guid)
			if qErr == nil {
				slot := 0
				for qRows.Next() && slot < playerQuestLogSlots {
					var questID, status, explored, timer, mob1, mob2, mob3, mob4 int64
					if err := qRows.Scan(&questID, &status, &explored, &timer, &mob1, &mob2, &mob3, &mob4); err != nil {
						continue
					}
					state.QuestLog[slot] = questLogEntry{
						QuestID:  uint32(questID),
						State:    questCompleteStateFlag(status),
						Timer:    uint32(timer),
						Counters: [4]uint16{uint16(mob1), uint16(mob2), uint16(mob3), uint16(mob4)},
					}
					slot++
				}
				qRows.Close()
			}
		}
	}()
	go func() {
		defer wg.Done()
		var taximask sql.NullString
		if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT taximask FROM characters WHERE guid = ?", guid).Scan(&taximask); err == nil {
			s.loadTaxiMask(taximask)
		} else {
			// TrinityCore PlayerTaxi::InitTaxiNodesForLevel seeds the race and
			// continent starting nodes for characters without saved masks.
			s.initTaxiNodesForLevel()
		}
	}()
	go func() {
		defer wg.Done()
		_ = s.CharGuild(ctx, &state)
	}()
	go func() {
		defer wg.Done()
		_ = s.loadPlayerSkills(ctx, &state)
	}()
	go func() {
		defer wg.Done()
		_ = s.loadPlayerPacketsState(ctx, &state)
	}()
	wg.Wait()

	_ = s.loadOptionalPlayerState(ctx, &state)
	_ = s.loadClassLevelStats(ctx, &state)
	_ = s.loadPlayerReputations(ctx, &state)
	s.player = &state
	return state, nil
}

func (s *session) loadClassLevelStats(ctx context.Context, state *playerState) error {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return nil
	}
	var baseHealth, baseMana int64
	err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT basehp, basemana FROM player_classlevelstats WHERE class = ? AND level = ?", state.Class, state.Level).Scan(&baseHealth, &baseMana)
	if err != nil {
		if missingTable(err) || isMissingColumn(err) {
			return nil
		}
		return err
	}
	if baseHealth > 0 {
		if state.Health <= 1 && state.MaxHealth <= 1 && state.XP == 0 {
			state.Health = uint32(baseHealth)
		}
		state.MaxHealth = uint32(baseHealth)
		if state.Health > state.MaxHealth {
			state.Health = state.MaxHealth
		}
	}
	if baseMana > 0 && state.Powers[0] == 0 {
		state.Powers[0] = uint32(baseMana)
	}
	if baseMana > 0 && state.MaxPowers[0] == 0 {
		state.MaxPowers[0] = uint32(baseMana)
	}
	return nil
}

func (s *session) loadPlayerReputations(ctx context.Context, state *playerState) error {
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil || s.server.Data == nil {
		return nil
	}
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT faction, standing, flags FROM character_reputation WHERE guid = ? ORDER BY faction", state.GUID)
	if err != nil {
		if isMissingColumn(err) || strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var faction, standing, flags int64
		if err := rows.Scan(&faction, &standing, &flags); err != nil {
			continue
		}
		reputation, found, err := s.server.Data.Reputation(uint32(faction), state.Race, state.Class)
		if err != nil {
			return err
		}
		if !found || reputation.ReputationList < 0 || reputation.ReputationList >= 128 {
			continue
		}
		state.Reputations = append(state.Reputations, playerReputation{FactionID: uint32(faction), ListID: uint32(reputation.ReputationList), Standing: int32(standing), Base: reputation.BaseStanding, Flags: uint8(flags)})
	}
	return rows.Err()
}

func buildInitialReputations(state playerState) []byte {
	values := make([]playerReputation, 128)
	for _, reputation := range state.Reputations {
		if reputation.ListID < uint32(len(values)) {
			values[reputation.ListID] = reputation
		}
	}
	packet := protocol.NewBuffer(4 + len(values)*5)
	packet.WriteU32(uint32(len(values)))
	for _, reputation := range values {
		packet.WriteU8(reputation.Flags)
		packet.WriteU32(uint32(reputation.Standing))
	}
	return packet.Bytes()
}

func buildForcedReactions() []byte {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(0)
	return packet.Bytes()
}

func (s *session) loadOptionalPlayerState(ctx context.Context, state *playerState) error {
	var xp, money, health, cinematic, knownCurrency, watchedFaction, ammoID, actionBars int64
	var powers [7]int64
	var optionalErr error
	args := []any{&xp, &money, &health}
	for i := range powers {
		args = append(args, &powers[i])
	}
	args = append(args, &cinematic, &knownCurrency, &watchedFaction, &ammoID, &actionBars)
	optionalErr = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT xp, money, health, power1, power2, power3, power4, power5, power6, power7, cinematic, knownCurrencies, watchedFaction, ammoId, actionBars FROM characters WHERE guid = ? AND account = ?", state.GUID, s.accountID).Scan(args...)
	if optionalErr != nil {
		if isMissingColumn(optionalErr) || errors.Is(optionalErr, sql.ErrNoRows) {
			return nil
		}
		return optionalErr
	}
	state.XP, state.Money = uint32(xp), uint32(money)
	if health >= 0 {
		state.Health, state.MaxHealth = uint32(health), uint32(health)
	}
	for i, power := range powers {
		state.Powers[i] = uint32(power)
		state.MaxPowers[i] = uint32(power)
	}
	state.Cinematic, state.KnownCurrency, state.WatchedFaction, state.AmmoID, state.ActionBars = uint32(cinematic), uint32(knownCurrency), uint32(watchedFaction), uint32(ammoID), uint32(actionBars)
	return nil
}

func (s *session) CharGuild(ctx context.Context, state *playerState) error {
	var guildID, rank int64
	err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT guildid, rank FROM guild_member WHERE guid = ?", state.GUID).Scan(&guildID, &rank)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		if isMissingColumn(err) || strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return err
	}
	state.GuildID, state.GuildRank = uint32(guildID), uint8(rank)
	return nil
}

func (s *session) loadPlayerSkills(ctx context.Context, state *playerState) error {
	defaults := defaultRacialSkills(state.Race, state.Class)
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		state.Skills = defaults
		return nil
	}
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT skill, value, max FROM character_skills WHERE guid = ?", state.GUID)
	if err != nil {
		state.Skills = defaults
		return nil
	}
	skills := make([]playerSkill, 0, 16)
	for rows.Next() {
		var skill, value, max uint16
		if err := rows.Scan(&skill, &value, &max); err == nil {
			if isLanguageSkill(skill) && (value == 0 || max == 0) {
				value = 300
				max = 300
			}
			skills = append(skills, playerSkill{Skill: skill, Step: 1, Value: value, Max: max})
		}
	}
	_ = rows.Close()
	for _, def := range defaults {
		found := false
		for i, sk := range skills {
			if sk.Skill == def.Skill {
				found = true
				if isLanguageSkill(sk.Skill) && (sk.Value == 0 || sk.Max == 0) {
					skills[i].Value = 300
					skills[i].Max = 300
					_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, ?, 300, 300)", state.GUID, def.Skill)
				}
				break
			}
		}
		if !found {
			skills = append(skills, def)
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, ?, ?, ?)", state.GUID, def.Skill, def.Value, def.Max)
		}
	}
	state.Skills = skills
	return nil
}

func isLanguageSkill(skill uint16) bool {
	switch skill {
	case 98, 109, 111, 113, 115, 137, 313, 315, 673, 759:
		return true
	}
	return false
}

func defaultRacialSkills(race, class uint8) []playerSkill {
	skills := make([]playerSkill, 0, 8)
	switch race {
	case 1: // Human
		skills = append(skills, playerSkill{Skill: 98, Value: 300, Max: 300, Step: 1})
	case 2: // Orc
		skills = append(skills, playerSkill{Skill: 109, Value: 300, Max: 300, Step: 1})
	case 3: // Dwarf
		skills = append(skills, playerSkill{Skill: 98, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 111, Value: 300, Max: 300, Step: 1})
	case 4: // Night Elf
		skills = append(skills, playerSkill{Skill: 98, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 113, Value: 300, Max: 300, Step: 1})
	case 5: // Undead
		skills = append(skills, playerSkill{Skill: 109, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 673, Value: 300, Max: 300, Step: 1})
	case 6: // Tauren
		skills = append(skills, playerSkill{Skill: 109, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 115, Value: 300, Max: 300, Step: 1})
	case 7: // Gnome
		skills = append(skills, playerSkill{Skill: 98, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 313, Value: 300, Max: 300, Step: 1})
	case 8: // Troll
		skills = append(skills, playerSkill{Skill: 109, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 315, Value: 300, Max: 300, Step: 1})
	case 10: // Blood Elf
		skills = append(skills, playerSkill{Skill: 109, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 137, Value: 300, Max: 300, Step: 1})
	case 11: // Draenei
		skills = append(skills, playerSkill{Skill: 98, Value: 300, Max: 300, Step: 1}, playerSkill{Skill: 759, Value: 300, Max: 300, Step: 1})
	default:
		skills = append(skills, playerSkill{Skill: 98, Value: 300, Max: 300, Step: 1})
	}
	return skills
}

func isMissingColumn(err error) bool {
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "no such column") || strings.Contains(value, "unknown column")
}

func (s *Server) buildPlayerUpdate(state playerState) (*protocol.Packet, error) {
	values := make([]uint32, playerValuesCount)
	values[0] = uint32(state.GUID)
	values[objectFieldType] = 0x19
	values[objectFieldScale] = math.Float32bits(1)
	values[unitFieldHealth] = state.Health
	values[unitFieldLevel] = uint32(state.Level)
	values[unitFieldBytes1] = uint32(state.StandState)
	values[unitFieldFaction] = s.raceFaction(state.Race)
	values[unitFieldFlags] = unitFlagPlayerControlled
	values[unitFieldAttackTime] = 2000
	values[unitFieldAttackTimeOffhand] = 2000
	values[unitFieldBoundingRadius] = math.Float32bits(0.306349)
	values[unitFieldCombatReach] = math.Float32bits(1.5)
	if s.Data != nil {
		if race, found, err := s.Data.Race(uint32(state.Race)); err == nil && found {
			values[unitFieldDisplayID] = race.MaleDisplayID
			if state.Gender != 0 {
				values[unitFieldDisplayID] = race.FemaleDisplayID
			}
			values[unitFieldNativeDisplayID] = values[unitFieldDisplayID]
		}
	}
	if state.MountDisplayID != 0 {
		values[unitFieldMountDisplayID] = state.MountDisplayID
	}
	values[unitFieldPlayerFlags] = state.PlayerFlags
	values[unitFieldPlayerFieldBytes] = state.PlayerFieldBytes
	values[unitFieldPlayerSelfResSpell] = state.SelfResSpell
	values[unitFieldPlayerBytes] = uint32(state.Skin) | uint32(state.Face)<<8 | uint32(state.HairStyle)<<16 | uint32(state.HairColor)<<24
	values[unitFieldPlayerBytes2] = uint32(state.FacialStyle) | uint32(state.SheathState)<<8
	values[unitFieldGuildID] = state.GuildID
	values[unitFieldGuildRank] = uint32(state.GuildRank)
	values[unitFieldXP] = state.XP
	values[unitFieldCoinage] = state.Money
	values[unitFieldMaxLevel] = 80
	values[unitFieldKnownCurrencies] = state.KnownCurrency
	values[unitFieldWatchedFaction] = state.WatchedFaction
	values[unitFieldAmmoID] = state.AmmoID
	for slot := 0; slot < playerQuestLogSlots; slot++ {
		entry := state.QuestLog[slot]
		base := playerQuestLogStart + slot*5
		values[base] = entry.QuestID
		values[base+1] = entry.State
		values[base+2] = uint32(entry.Counters[0]) | uint32(entry.Counters[1])<<16
		values[base+3] = uint32(entry.Counters[2]) | uint32(entry.Counters[3])<<16
		values[base+4] = entry.Timer
	}
	for i, sk := range state.Skills {
		if i >= playerMaxSkills {
			break
		}
		idx := playerSkillInfoStart + i*3
		values[idx] = uint32(sk.Skill) | uint32(sk.Step)<<16
		values[idx+1] = uint32(sk.Value) | uint32(sk.Max)<<16
		values[idx+2] = uint32(sk.Bonus)
	}
	equipment := strings.Fields(state.Equipment)
	for slot := 0; slot < playerVisibleItemCount; slot++ {
		base := slot * 2
		if base >= len(equipment) {
			break
		}
		itemID, err := strconv.ParseUint(equipment[base], 10, 32)
		if err != nil || itemID == 0 {
			continue
		}
		values[playerVisibleItemStart+slot*2] = uint32(itemID)
		if base+1 < len(equipment) {
			if enchant, parseErr := strconv.ParseUint(equipment[base+1], 10, 32); parseErr == nil {
				values[playerVisibleItemStart+slot*2+1] = uint32(enchant)
			}
		}
	}
	for i, power := range state.Powers {
		values[unitFieldHealth+1+i] = power
		values[unitFieldMaxPower1+i] = state.MaxPowers[i]
	}
	values[unitFieldMaxHealth] = state.MaxHealth

	// Combat & Attack speeds (prevents client div-by-zero crashes in PaperDollFrame_UpdateStats)
	values[unitFieldRangedAttackTime] = 2000
	values[unitModCastSpeed] = math.Float32bits(1.0)
	values[unitFieldMinDamage] = math.Float32bits(1.0)
	values[unitFieldMaxDamage] = math.Float32bits(2.0)
	values[unitFieldMinOffhandDamage] = math.Float32bits(1.0)
	values[unitFieldMaxOffhandDamage] = math.Float32bits(2.0)
	values[unitFieldMinRangedDamage] = math.Float32bits(1.0)
	values[unitFieldMaxRangedDamage] = math.Float32bits(2.0)
	values[unitFieldAttackPowerMultiplier] = math.Float32bits(1.0)
	values[unitFieldRangedAttackPowerMultiplier] = math.Float32bits(1.0)
	values[unitFieldAttackPower] = uint32(20 + int(state.Level)*2)
	values[unitFieldRangedAttackPower] = uint32(20 + int(state.Level)*2)

	// Base stats & Armor (displayed in character sheet)
	baseStat := uint32(20 + int(state.Level)*2)
	for i := 0; i < 5; i++ {
		values[unitFieldStat0+i] = baseStat
		values[unitFieldPosStat0+i] = baseStat
	}
	values[unitFieldResistances] = baseStat * 2 // Armor = Agility * 2

	values[unitFieldBaseHealth] = maxUint32(state.MaxHealth, 1)
	if len(state.MaxPowers) > 0 {
		values[unitFieldBaseMana] = maxUint32(state.MaxPowers[0], 1)
	}

	// Modifiers & Ratings (required by client PaperDoll formulas)
	for i := 0; i < 7; i++ {
		values[playerFieldModDamageDonePct+i] = math.Float32bits(1.0)
		values[playerSpellCritPercentage1+i] = math.Float32bits(5.0)
	}
	values[playerFieldModHealingPct] = math.Float32bits(1.0)
	values[playerFieldModHealingDonePct] = math.Float32bits(1.0)
	values[playerCritPercentage] = math.Float32bits(5.0)
	values[playerRangedCritPercentage] = math.Float32bits(5.0)
	values[playerOffhandCritPercentage] = math.Float32bits(5.0)

	// Free talent points & spent points
	if state.Level >= 10 {
		var spent uint32
		for _, rank := range state.Talents {
			spent += uint32(rank + 1)
		}
		totalPoints := uint32(state.Level - 9)
		if totalPoints > spent {
			values[playerCharacterPoints1] = totalPoints - spent
		}
		values[playerCharacterPoints2] = spent
	}

	mask := protocol.NewUpdateMask(len(values))
	for index, value := range values {
		if value != 0 {
			if err := mask.Set(index); err != nil {
				return nil, err
			}
		}
	}
	if err := mask.Set(1); err != nil {
		return nil, err
	}
	block := protocol.NewBuffer(256)
	block.WriteU8(protocol.UpdateCreateObject2)
	block.WritePackedGUID(state.GUID)
	block.WriteU8(4)
	block.WriteU16(0x0061)
	block.WriteU32(0)
	block.WriteU16(0)
	block.WriteU32(uint32(time.Now().UnixMilli()))
	block.WriteF32(state.X)
	block.WriteF32(state.Y)
	block.WriteF32(state.Z)
	block.WriteF32(state.Orientation)
	block.WriteU32(0)
	for _, speed := range []float32{2.5, 7, 4.5, 4.722222, 2.5, 7, 4.5, 3.141594, 3.14} {
		block.WriteF32(speed)
	}
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index, value := range values {
		if mask.Has(index) {
			block.WriteU32(value)
		}
	}
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(block.Bytes())
	return updates.BuildPacket(0)
}

func (s *Server) raceFaction(race uint8) uint32 {
	if s.Data != nil {
		if value, found, err := s.Data.Race(uint32(race)); err == nil && found {
			return value.FactionID
		}
	}
	return 0
}

// buildPlayerValuesUpdate assembles an UPDATETYPE_VALUES block (packed GUID,
// update mask, changed values) the way Object::_BuildValuesUpdate does for
// in-place field changes such as quest log slots.
func (s *Server) buildPlayerValuesUpdate(guid uint64, fields map[int]uint32) (*protocol.Packet, error) {
	values := make([]uint32, playerValuesCount)
	mask := protocol.NewUpdateMask(playerValuesCount)
	for index, value := range fields {
		if index < 0 || index >= playerValuesCount {
			continue
		}
		values[index] = value
		if err := mask.Set(index); err != nil {
			return nil, err
		}
	}
	block := protocol.NewBuffer(64 + len(fields)*4)
	block.WriteU8(protocol.UpdateValues)
	block.WritePackedGUID(guid)
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index := 0; index < playerValuesCount; index++ {
		if mask.Has(index) {
			block.WriteU32(values[index])
		}
	}
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(block.Bytes())
	return updates.BuildPacket(0)
}

// sendPlayerQuestLogUpdate pushes one quest log slot (or its clearing) to
// the owning client via a values update, mirroring SetQuestSlot + the
// resulting _BuildValuesUpdate on accept/abandon.
func (s *session) sendPlayerQuestLogUpdate(slot int) {
	if s.player == nil || slot < 0 || slot >= playerQuestLogSlots {
		return
	}
	base := playerQuestLogStart + slot*5
	entry := s.player.QuestLog[slot]
	fields := map[int]uint32{
		base:     entry.QuestID,
		base + 1: entry.State,
		base + 2: uint32(entry.Counters[0]) | uint32(entry.Counters[1])<<16,
		base + 3: uint32(entry.Counters[2]) | uint32(entry.Counters[3])<<16,
		base + 4: entry.Timer,
	}
	packet, err := s.server.buildPlayerValuesUpdate(s.playerGUID, fields)
	if err != nil {
		s.debug("quest log values update build failed", "account", s.accountName, "error", err)
		return
	}
	_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
}

// sendPlayerMountUpdate pushes UNIT_FIELD_MOUNTDISPLAYID as a values update.
func (s *session) sendPlayerMountUpdate() {
	if s.player == nil {
		return
	}
	packet, err := s.server.buildPlayerValuesUpdate(s.playerGUID, map[int]uint32{unitFieldMountDisplayID: s.player.MountDisplayID})
	if err != nil {
		return
	}
	_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
}

// currentPlayer returns the live player state for timer callbacks.
func (s *session) currentPlayer() *playerState {
	return s.player
}

// sendPlayerMoneyUpdate pushes PLAYER_FIELD_COINAGE as a values update.
func (s *session) sendPlayerMoneyUpdate() {
	if s.player == nil {
		return
	}
	packet, err := s.server.buildPlayerValuesUpdate(s.playerGUID, map[int]uint32{unitFieldCoinage: s.player.Money})
	if err != nil {
		return
	}
	_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
}

func buildItemCreateBlock(fullGUID uint64, itemEntry, count uint32, ownerGUID uint64) []byte {
	return buildItemCreateBlockForLocation(fullGUID, itemEntry, count, ownerGUID, ownerGUID, 0, nil)
}

func buildItemCreateBlockWithContents(fullGUID uint64, itemEntry, count uint32, ownerGUID uint64, containerSlots uint32, contents map[uint32]uint64) []byte {
	return buildItemCreateBlockForLocation(fullGUID, itemEntry, count, ownerGUID, ownerGUID, containerSlots, contents)
}

func buildItemCreateBlockForLocation(fullGUID uint64, itemEntry, count uint32, ownerGUID, containedGUID uint64, containerSlots uint32, contents map[uint32]uint64) []byte {
	if containerSlots > 36 {
		containerSlots = 36
	}
	values := make([]uint32, 68+containerSlots*2)
	values[0] = uint32(fullGUID)
	values[1] = uint32(fullGUID >> 32)
	values[2] = 0x03 // TYPEID_ITEM
	values[3] = itemEntry
	values[4] = math.Float32bits(1.0)
	values[6] = uint32(ownerGUID)
	values[7] = uint32(ownerGUID >> 32)
	values[8] = uint32(containedGUID)
	values[9] = uint32(containedGUID >> 32)
	values[14] = count
	values[16] = 100 // Durability
	values[17] = 100 // MaxDurability
	if containerSlots > 0 {
		values[64] = containerSlots
		for slot, itemGUID := range contents {
			if slot >= containerSlots {
				continue
			}
			values[66+slot*2] = uint32(itemGUID)
			values[67+slot*2] = uint32(itemGUID >> 32)
		}
	}

	mask := protocol.NewUpdateMask(len(values))
	for idx, val := range values {
		if val != 0 {
			_ = mask.Set(idx)
		}
	}
	_ = mask.Set(1)

	block := protocol.NewBuffer(128)
	block.WriteU8(protocol.UpdateCreateObject2)
	block.WritePackedGUID(fullGUID)
	block.WriteU8(1) // TYPEID_ITEM
	block.WriteU8(0) // update flags
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for i := 0; i < len(values); i++ {
		if mask.Has(i) {
			block.WriteU32(values[i])
		}
	}
	return block.Bytes()
}

func (s *session) sendItemCreate(itemGUID uint64, itemEntry, count uint32, bag, slot uint8) error {
	fullGUID := uint64(itemGUID) | (uint64(0x4000) << 48)
	block := buildItemCreateBlock(fullGUID, itemEntry, count, s.playerGUID)
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(block)
	packet, err := updates.BuildPacket(0)
	if err != nil {
		return err
	}
	if err := s.write(packet.Opcode, packet.Payload.Bytes(), true); err != nil {
		return err
	}
	packSlotField := 364 + int(slot-23)*2
	if slot < 23 {
		packSlotField = 318 + int(slot)*2
	}
	fields := map[int]uint32{
		packSlotField:     uint32(fullGUID),
		packSlotField + 1: uint32(fullGUID >> 32),
		unitFieldCoinage:  s.player.Money,
	}
	playerPacket, err := s.server.buildPlayerValuesUpdate(s.playerGUID, fields)
	if err == nil && playerPacket != nil {
		_ = s.write(playerPacket.Opcode, playerPacket.Payload.Bytes(), true)
	}
	return nil
}

func (s *session) sendInventoryItems(ctx context.Context) error {
	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return nil
	}
	rows, err := cdb.QueryContext(ctx, `SELECT ci.bag, ci.slot, ci.item, ii.itemEntry, ii.count
		FROM character_inventory AS ci
		JOIN item_instance AS ii ON ii.guid = ci.item
		WHERE ci.guid = ? ORDER BY ci.bag, ci.slot`, s.playerGUID)
	if err != nil {
		if missingTable(err) || isMissingColumn(err) {
			return nil
		}
		return err
	}
	defer rows.Close()

	type inventoryItem struct {
		bag, slot, itemGUID, itemEntry, count int64
	}
	items := make([]inventoryItem, 0)
	bagItems := make(map[int64]uint64)
	for rows.Next() {
		var item inventoryItem
		if err := rows.Scan(&item.bag, &item.slot, &item.itemGUID, &item.itemEntry, &item.count); err != nil {
			continue
		}
		items = append(items, item)
		if item.bag == 0 && item.slot >= 19 && item.slot <= 22 {
			bagItems[item.itemGUID] = uint64(item.itemGUID) | (uint64(0x4000) << 48)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	contents := make(map[int64]map[uint32]uint64)
	for _, item := range items {
		if item.bag == 0 {
			continue
		}
		bagGUID, ok := bagItems[item.bag]
		if !ok || item.slot < 0 {
			continue
		}
		if contents[int64(bagGUID)] == nil {
			contents[int64(bagGUID)] = make(map[uint32]uint64)
		}
		contents[int64(bagGUID)][uint32(item.slot)] = uint64(item.itemGUID) | (uint64(0x4000) << 48)
	}
	containerSlots := func(entry int64) uint32 {
		if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
			return 0
		}
		var slots int64
		if s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(ContainerSlots, 0) FROM item_template WHERE entry = ?", entry).Scan(&slots) != nil || slots <= 0 {
			return 0
		}
		if slots > 36 {
			slots = 36
		}
		return uint32(slots)
	}
	updates := protocol.NewUpdateData()
	fields := make(map[int]uint32)
	for _, item := range items {
		bag, slot, itemGUID, itemEntry, count := item.bag, item.slot, item.itemGUID, item.itemEntry, item.count
		if count <= 0 {
			count = 1
		}
		fullGUID := uint64(itemGUID) | (uint64(0x4000) << 48)
		containedGUID := uint64(s.playerGUID)
		if bag != 0 {
			containedGUID = bagItems[bag]
		}
		block := buildItemCreateBlockForLocation(fullGUID, uint32(itemEntry), uint32(count), s.playerGUID, containedGUID, containerSlots(itemEntry), contents[int64(fullGUID)])
		updates.AddUpdateBlock(block)

		if bag == 0 {
			if slot >= 23 && slot <= 38 {
				packField := 364 + int(slot-23)*2
				fields[packField] = uint32(fullGUID)
				fields[packField+1] = uint32(fullGUID >> 32)
			} else if slot < 23 {
				invField := 318 + int(slot)*2
				fields[invField] = uint32(fullGUID)
				fields[invField+1] = uint32(fullGUID >> 32)
			}
		}
	}
	if updates.HasData() {
		packet, err := updates.BuildPacket(0)
		if err == nil && packet != nil {
			_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
		}
	}
	if len(fields) > 0 {
		packet, err := s.server.buildPlayerValuesUpdate(s.playerGUID, fields)
		if err == nil && packet != nil {
			_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
		}
	}
	return nil
}
