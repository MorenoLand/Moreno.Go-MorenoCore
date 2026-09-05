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
	playerValuesCount                           = 1326
	objectFieldType                             = 2
	objectFieldEntry                            = 3
	objectFieldScale                            = 4
	unitFieldSummon                             = 8
	unitFieldBytes0                             = 23 // UNIT_FIELD_BYTES_0: Race, Class, Gender, PowerType
	unitFieldHealth                             = 24
	unitFieldPower1                             = 25
	unitFieldLevel                              = 54
	unitFieldFaction                            = 55
	unitFieldFlags                              = 59
	unitFieldAttackTime                         = 62
	unitFieldAttackTimeOffhand                  = 63
	unitFieldBoundingRadius                     = 65
	unitFieldCombatReach                        = 66
	unitFieldDisplayID                          = 67
	unitFieldNativeDisplayID                    = 68
	unitFieldPlayerFlags                        = 150
	unitFieldPlayerBytes                        = 153
	unitFieldPlayerBytes2                       = 154
	unitFieldPlayerBytes3                       = 155
	unitFieldGuildID                            = 151
	unitFieldGuildRank                          = 152
	unitFieldGuildTimestamp                     = 157
	unitFieldXP                                 = 634
	unitFieldNextLevelXP                        = 635
	unitFieldCoinage                            = 1170
	unitFieldMaxLevel                           = 1279
	playerFieldKnownTitles                      = 626
	unitFieldKnownCurrencies                    = 632
	unitFieldWatchedFaction                     = 1230
	unitFieldChosenTitle                        = 1195
	unitFieldAmmoID                             = 1198
	unitFieldPlayerSelfResSpell                 = 1199 // PLAYER_SELF_RES_SPELL = UNIT_END + 0x041B
	playerFieldDuelArbiter                      = 148  // PLAYER_DUEL_ARBITER = UNIT_END + 0x0000 (Size 2)
	playerFieldDuelTeam                         = 156  // PLAYER_DUEL_TEAM = UNIT_END + 0x0008 (Size 1)
	playerQuestLogStart                         = 158  // PLAYER_QUEST_LOG_1_1; stride 5 per TC MAX_QUEST_OFFSET
	playerQuestLogSlots                         = 25
	playerSkillInfoStart                        = 636
	playerMaxSkills                             = 128
	playerVisibleItemStart                      = 283
	playerVisibleItemCount                      = 19
	unitFieldMaxHealth                          = 32
	unitFieldMaxPower1                          = 33
	unitFieldRangedAttackTime                   = 64
	unitFieldMinDamage                          = 70
	unitFieldMaxDamage                          = 71
	unitFieldMinOffhandDamage                   = 72
	unitFieldMaxOffhandDamage                   = 73
	unitModCastSpeed                            = 80
	unitFieldStat0                              = 84 // Strength
	unitFieldStat1                              = 85 // Agility
	unitFieldStat2                              = 86 // Stamina
	unitFieldStat3                              = 87 // Intellect
	unitFieldStat4                              = 88 // Spirit
	unitFieldPosStat0                           = 89
	unitFieldNegStat0                           = 94
	unitFieldResistances                        = 99 // 99..105 (Physical/Armor, Holy, Fire, Nature, Frost, Shadow, Arcane)
	unitFieldBaseMana                           = 120
	unitFieldBaseHealth                         = 121
	unitFieldAttackPower                        = 123
	unitFieldAttackPowerMods                    = 124
	unitFieldAttackPowerMultiplier              = 125
	unitFieldRangedAttackPower                  = 126
	unitFieldRangedAttackPowerMods              = 127
	unitFieldRangedAttackPowerMultiplier        = 128
	unitFieldMinRangedDamage                    = 129
	unitFieldMaxRangedDamage                    = 130
	playerCharacterPoints1                      = 1020 // Free talent points
	playerCharacterPoints2                      = 1021
	playerBlockPercentage                       = 1024
	playerDodgePercentage                       = 1025
	playerParryPercentage                       = 1026
	playerCritPercentage                        = 1029
	playerRangedCritPercentage                  = 1030
	playerOffhandCritPercentage                 = 1031
	playerSpellCritPercentage1                  = 1032 // 1032..1038
	playerShieldBlock                           = 1039
	playerFieldModDamageDonePos                 = 1171 // 1171..1177
	playerFieldModDamageDoneNeg                 = 1178 // 1178..1184
	playerFieldModDamageDonePct                 = 1185 // 1185..1191
	playerFieldModHealingDonePos                = 1192
	playerFieldModHealingPct                    = 1193
	playerFieldModHealingDonePct                = 1194
	playerFieldCombatRating1                    = 1231 // 1231..1255
	unitFlagPlayerControlled             uint32 = 0x00000008
	unitFlagInCombat                     uint32 = 0x00080000
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
	PetGUID          uint64
	Name             string
	Race             uint8
	Class            uint8
	Gender           uint8
	Skin             uint8
	Face             uint8
	HairStyle        uint8
	HairColor        uint8
	FacialStyle      uint8
	BankBagSlots     uint8
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
	LfgEntryPointMap uint32
	LfgEntryPointX   float32
	LfgEntryPointY   float32
	LfgEntryPointZ   float32
	LfgEntryPointO   float32
	ExtraFlags       uint32
	AtLogin          uint32
	Zone             uint32
	Health           uint32
	MaxHealth        uint32
	BaseMana         uint32
	Powers           [7]uint32
	MaxPowers        [7]uint32
	Cinematic        uint32
	Movie            uint32
	KnownCurrency    uint32
	WatchedFaction   uint32
	AmmoID           uint32
	ChosenTitle      uint32
	KnownTitles      [6]uint32
	ActionBars       uint32
	PassOnGroupLoot  bool
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
	TotemSlots       [4]uint64
	PlayerFieldBytes uint32
	SelfResSpell     uint32
	DuelArbiter      uint64
	DuelTeam         uint32
	UnitFlags        uint32
	HomebindMap      uint32
	HomebindZone     uint32
	HomebindX        float32
	HomebindY        float32
	HomebindZ        float32
	Reputations       []playerReputation
	Talents           map[uint32]uint8
	TalentGroupsCount uint8
	ActiveTalentGroup uint8
	Glyphs            [2][6]uint16
	Stats             [5]uint32
	Armor             uint32
	Block             uint32
	AttackPower       uint32
	RangedAttackPower uint32
	MinDamage         float32
	MaxDamage         float32
	AttackTime        uint32
	MinOffhandDamage  float32
	MaxOffhandDamage  float32
	OffhandAttackTime uint32
	MinRangedDamage   float32
	MaxRangedDamage   float32
	RangedAttackTime  uint32
	CombatRatings     [25]uint32
	DungeonDifficulty uint8
	RaidDifficulty    uint8
	VehicleGUID       uint64
	VehicleSeat       int8
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
	var race, class, gender, level, playerFlags, mapID, extraFlags, atLogin, zone, deathExpireTime, cinematic int64
	var equipment sql.NullString
	if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT guid, name, race, class, gender, level, playerFlags, map, position_x, position_y, position_z, orientation, extra_flags, at_login, zone, equipmentCache, death_expire_time, COALESCE(cinematic, 0) FROM characters WHERE guid = ? AND account = ?", guid, s.accountID).Scan(&state.GUID, &state.Name, &race, &class, &gender, &level, &playerFlags, &mapID, &state.X, &state.Y, &state.Z, &state.Orientation, &extraFlags, &atLogin, &zone, &equipment, &deathExpireTime, &cinematic); err != nil {
		return playerState{}, err
	}
	s.deathExpireTime = deathExpireTime
	state.Race, state.Class, state.Gender, state.Level = uint8(race), uint8(class), uint8(gender), uint8(level)
	state.PlayerFlags, state.Map, state.ExtraFlags, state.AtLogin, state.Zone, state.Cinematic = uint32(playerFlags), uint32(mapID), uint32(extraFlags), uint32(atLogin), uint32(zone), uint32(cinematic)
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
	if (state.ExtraFlags&playerExtraGMChat != 0) || (state.ExtraFlags&playerExtraGMOn != 0) || s.security > 0 {
		s.gmChat = true
		state.ExtraFlags |= playerExtraGMChat
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
	s.loadPlayerTalents(ctx, &state)
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
	_ = s.calculatePlayerStats(ctx, &state)
	_ = s.loadPlayerReputations(ctx, &state)
	s.player = &state
	return state, nil
}

func (s *session) calculatePlayerStats(ctx context.Context, state *playerState) error {
	if state == nil || s.server == nil || s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return nil
	}
	lvl := state.Level
	if lvl < 1 {
		lvl = 1
	} else if lvl > 80 {
		lvl = 80
	}
	// 1. Base stats from player_levelstats
	var str, agi, sta, inte, spi int64
	err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT str, agi, sta, inte, spi FROM player_levelstats WHERE race = ? AND class = ? AND level = ?", state.Race, state.Class, lvl).Scan(&str, &agi, &sta, &inte, &spi)
	if err != nil {
		str = int64(20 + int(lvl)*2)
		agi = int64(20 + int(lvl)*2)
		sta = int64(20 + int(lvl)*2)
		inte = int64(20 + int(lvl)*2)
		spi = int64(20 + int(lvl)*2)
	}
	state.Stats[0] = uint32(str)
	state.Stats[1] = uint32(agi)
	state.Stats[2] = uint32(sta)
	state.Stats[3] = uint32(inte)
	state.Stats[4] = uint32(spi)

	// Base health and base mana from player_classlevelstats
	var baseHealth, baseMana int64
	_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT basehp, basemana FROM player_classlevelstats WHERE class = ? AND level = ?", state.Class, lvl).Scan(&baseHealth, &baseMana)
	if baseHealth <= 0 {
		baseHealth = int64(20 + int(lvl)*15)
	}
	if baseMana <= 0 {
		baseMana = int64(80 + int(lvl)*20)
	}
	state.BaseMana = uint32(baseMana)

	// Default unarmed weapon speeds and damages
	state.MinDamage = 1.0
	state.MaxDamage = 2.0
	state.AttackTime = 2000
	state.MinOffhandDamage = 1.0
	state.MaxOffhandDamage = 2.0
	state.OffhandAttackTime = 2000
	state.MinRangedDamage = 1.0
	state.MaxRangedDamage = 2.0
	state.RangedAttackTime = 2000
	state.Armor = 0
	state.Block = 0
	state.AttackPower = 0
	state.RangedAttackPower = 0
	for i := range state.CombatRatings {
		state.CombatRatings[i] = 0
	}

	// 2. Iterate equipped items (slots 0..18)
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		rows, err := s.server.CharactersStore.DB.QueryContext(ctx, `SELECT ci.slot, it.armor, it.block, it.delay, it.dmg_min1, it.dmg_max1,
			it.stat_type1, it.stat_value1, it.stat_type2, it.stat_value2, it.stat_type3, it.stat_value3, it.stat_type4, it.stat_value4,
			it.stat_type5, it.stat_value5, it.stat_type6, it.stat_value6, it.stat_type7, it.stat_value7, it.stat_type8, it.stat_value8,
			it.stat_type9, it.stat_value9, it.stat_type10, it.stat_value10
			FROM character_inventory AS ci
			JOIN item_instance AS ii ON ii.guid = ci.item
			JOIN item_template AS it ON it.entry = ii.itemEntry
			WHERE ci.guid = ? AND ci.bag = 0 AND ci.slot < 19`, state.GUID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var slot, armor, block, delay int64
				var minDmg, maxDmg float64
				var st [10]int64
				var sv [10]int64
				if scanErr := rows.Scan(&slot, &armor, &block, &delay, &minDmg, &maxDmg,
					&st[0], &sv[0], &st[1], &sv[1], &st[2], &sv[2], &st[3], &sv[3],
					&st[4], &sv[4], &st[5], &sv[5], &st[6], &sv[6], &st[7], &sv[7],
					&st[8], &sv[8], &st[9], &sv[9]); scanErr != nil {
					continue
				}
				if armor > 0 {
					state.Armor += uint32(armor)
				}
				if block > 0 {
					state.Block += uint32(block)
				}
				// Weapon slots: 15 = Main Hand, 16 = Off Hand, 17 = Ranged
				if slot == 15 {
					if minDmg > 0 {
						state.MinDamage = float32(minDmg)
					}
					if maxDmg > 0 {
						state.MaxDamage = float32(maxDmg)
					}
					if delay > 0 {
						state.AttackTime = uint32(delay)
					}
				} else if slot == 16 {
					if minDmg > 0 {
						state.MinOffhandDamage = float32(minDmg)
					}
					if maxDmg > 0 {
						state.MaxOffhandDamage = float32(maxDmg)
					}
					if delay > 0 {
						state.OffhandAttackTime = uint32(delay)
					}
				} else if slot == 17 {
					if minDmg > 0 {
						state.MinRangedDamage = float32(minDmg)
					}
					if maxDmg > 0 {
						state.MaxRangedDamage = float32(maxDmg)
					}
					if delay > 0 {
						state.RangedAttackTime = uint32(delay)
					}
				}
				// Stats
				for k := 0; k < 10; k++ {
					val := sv[k]
					if val == 0 {
						continue
					}
					switch st[k] {
					case 3: // Agility
						state.Stats[1] += uint32(val)
					case 4: // Strength
						state.Stats[0] += uint32(val)
					case 5: // Intellect
						state.Stats[3] += uint32(val)
					case 6: // Spirit
						state.Stats[4] += uint32(val)
					case 7: // Stamina
						state.Stats[2] += uint32(val)
					case 12: // Defense rating
						state.CombatRatings[0] += uint32(val)
					case 13: // Dodge rating
						state.CombatRatings[1] += uint32(val)
					case 14: // Parry rating
						state.CombatRatings[2] += uint32(val)
					case 15: // Block rating
						state.CombatRatings[3] += uint32(val)
					case 16, 17, 18, 31: // Hit rating
						state.CombatRatings[5] += uint32(val)
					case 19, 20, 21, 32: // Crit rating
						state.CombatRatings[8] += uint32(val)
					case 28, 29, 30, 36: // Haste rating
						state.CombatRatings[17] += uint32(val)
					case 38: // Attack power
						state.AttackPower += uint32(val)
					case 39: // Ranged attack power
						state.RangedAttackPower += uint32(val)
					}
				}
			}
		}
	}

	// 3. Derived stats (TrinityCore StatSystem.cpp:309-360)
	totalSta := state.Stats[2]
	totalInte := state.Stats[3]
	totalStr := state.Stats[0]
	totalAgi := state.Stats[1]

	state.MaxHealth = uint32(baseHealth) + (totalSta * 10)
	totalMana := uint32(baseMana) + (totalInte * 15)
	if state.Class == 1 { // Warrior: rage max 1000
		state.MaxPowers[1] = 1000
	} else if state.Class == 4 { // Rogue: energy max 100
		state.MaxPowers[3] = 100
	} else if state.Class == 6 { // Death Knight: runic power max 1000
		state.MaxPowers[6] = 1000
	} else {
		state.MaxPowers[0] = totalMana
		if state.Powers[0] > totalMana || (state.XP == 0 && state.Level == 1) || state.Powers[0] == 0 {
			state.Powers[0] = totalMana
		}
	}
	if state.Health > state.MaxHealth || state.Health <= 1 || (state.XP == 0 && state.Level == 1) {
		state.Health = state.MaxHealth
	}

	// Armor: item armor + Agility * 2
	state.Armor += totalAgi * 2

	// Base Attack Power (TC StatSystem.cpp:1034-1065)
	var baseAP int32
	switch state.Class {
	case 1, 2, 6: // Warrior, Paladin, Death Knight
		baseAP = int32(totalStr*2 + uint32(lvl)*3) - 20
	case 3, 4: // Hunter, Rogue
		baseAP = int32(totalStr + totalAgi + uint32(lvl)*2) - 20
	case 7, 11: // Shaman, Druid
		baseAP = int32(totalStr*2 + uint32(lvl)*2) - 20
	default:
		baseAP = int32(totalStr) - 10
	}
	if baseAP < 0 {
		baseAP = 0
	}
	state.AttackPower += uint32(baseAP)

	var baseRAP int32
	if state.Class == 3 { // Hunter
		baseRAP = int32(uint32(lvl)*2 + totalAgi*2) - 10
	} else if state.Class == 4 || state.Class == 1 { // Rogue, Warrior
		baseRAP = int32(uint32(lvl) + totalAgi) - 10
	}
	if baseRAP < 0 {
		baseRAP = 0
	}
	state.RangedAttackPower += uint32(baseRAP)
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
	optionalErr = s.server.CharactersStore.DB.QueryRowContext(ctx, `SELECT xp, money, health, power1, power2, power3, power4, power5, power6, power7,
		COALESCE(CAST(cinematic AS INTEGER), 0),
		COALESCE(CAST(knownCurrencies AS INTEGER), 0),
		COALESCE(CAST(watchedFaction AS INTEGER), 0),
		COALESCE(CAST(ammoId AS INTEGER), 0),
		COALESCE(CAST(actionBars AS INTEGER), 0)
		FROM characters WHERE guid = ? AND account = ?`, state.GUID, s.accountID).Scan(args...)
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
	var bankSlots int64
	_ = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COALESCE(bankSlots, 0) FROM characters WHERE guid = ?", state.GUID).Scan(&bankSlots)
	state.BankBagSlots = uint8(bankSlots)

	var specsCount, activeSpec int64
	_ = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COALESCE(talentGroupsCount, 1), COALESCE(activeTalentGroup, 0) FROM characters WHERE guid = ?", state.GUID).Scan(&specsCount, &activeSpec)
	if specsCount < 1 {
		specsCount = 1
	} else if specsCount > 2 {
		specsCount = 2
	}
	if activeSpec >= specsCount {
		activeSpec = 0
	}
	state.TalentGroupsCount = uint8(specsCount)
	state.ActiveTalentGroup = uint8(activeSpec)

	var chosenTitle int64
	var knownTitlesStr sql.NullString
	_ = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COALESCE(chosenTitle, 0), knownTitles FROM characters WHERE guid = ?", state.GUID).Scan(&chosenTitle, &knownTitlesStr)
	if chosenTitle > 0 {
		state.ChosenTitle = uint32(chosenTitle)
	}
	if knownTitlesStr.Valid && knownTitlesStr.String != "" {
		parts := strings.Fields(knownTitlesStr.String)
		for i := 0; i < len(parts) && i < 6; i++ {
			if val, err := strconv.ParseUint(parts[i], 10, 32); err == nil {
				state.KnownTitles[i] = uint32(val)
			}
		}
	}

	gRows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT talentGroup, glyph1, glyph2, glyph3, glyph4, glyph5, glyph6 FROM character_glyphs WHERE guid = ?", state.GUID)
	if err == nil {
		for gRows.Next() {
			var tg int64
			var g1, g2, g3, g4, g5, g6 int64
			if err := gRows.Scan(&tg, &g1, &g2, &g3, &g4, &g5, &g6); err == nil && tg >= 0 && tg < 2 {
				state.Glyphs[tg][0] = uint16(g1)
				state.Glyphs[tg][1] = uint16(g2)
				state.Glyphs[tg][2] = uint16(g3)
				state.Glyphs[tg][3] = uint16(g4)
				state.Glyphs[tg][4] = uint16(g5)
				state.Glyphs[tg][5] = uint16(g6)
			}
		}
		gRows.Close()
	}
	return nil
}

func (s *session) loadPlayerTalents(ctx context.Context, state *playerState) {
	state.Talents = make(map[uint32]uint8)
	if s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return
	}
	cdb := s.server.CharactersStore.DB
	rows, err := cdb.QueryContext(ctx, "SELECT spell FROM character_talent WHERE guid = ? AND talentGroup = ?", state.GUID, state.ActiveTalentGroup)
	if err != nil {
		return
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
				state.Talents[tid] = r
			}
		}
	}
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
			if !isAllowedClassSkill(state.Class, skill) {
				_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "DELETE FROM character_skills WHERE guid = ? AND skill = ?", state.GUID, skill)
				continue
			}
			if isLanguageSkill(skill) && (value == 0 || max == 0) {
				value = 300
				max = 300
			} else if isLevelScaledSkill(skill) {
				expectedMax := uint16(math.Max(float64(state.Level)*5, 5))
				if max > expectedMax || max == 0 {
					max = expectedMax
					if value > max {
						value = max
					}
					_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE character_skills SET value = ?, max = ? WHERE guid = ? AND skill = ?", value, max, state.GUID, skill)
				}
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

func isLevelScaledSkill(skill uint16) bool {
	switch skill {
	case 95, 162, 43, 44, 45, 46, 54, 55, 160, 172, 173, 176, 226, 228, 229, 473:
		return true
	}
	return false
}

func isAllowedClassSkill(class uint8, skill uint16) bool {
	switch skill {
	case 293: // Plate Mail
		return class == 1 || class == 2 || class == 6
	case 413: // Mail
		return class == 1 || class == 2 || class == 3 || class == 6 || class == 7
	case 414: // Leather
		return class != 5 && class != 8 && class != 9 // Not Priest, Mage, Warlock
	case 433: // Shield
		return class == 1 || class == 2 || class == 7
	// Talent / Class spell lines:
	case 26, 256, 257: // Warrior: Arms, Fury, Protection
		return class == 1
	case 184, 267, 594: // Paladin: Retribution, Protection, Holy
		return class == 2
	case 50, 51, 163: // Hunter: Beast Mastery, Survival, Marksmanship
		return class == 3
	case 253, 254, 255: // Rogue: Assassination, Combat, Subtlety
		return class == 4
	case 56, 78, 613: // Priest: Holy, Shadow, Discipline
		return class == 5
	case 770, 771, 772: // Death Knight: Blood, Frost, Unholy
		return class == 6
	case 373, 374, 375: // Shaman: Enhancement, Resto, Elemental
		return class == 7
	case 6, 8, 237: // Mage: Frost, Fire, Arcane
		return class == 8
	case 354, 355, 593: // Warlock: Demo, Affliction, Destro
		return class == 9
	case 134, 573, 574: // Druid: Feral, Resto, Balance
		return class == 11
	}
	return true
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

func classPowerType(class uint8) uint8 {
	switch class {
	case 1: // Warrior
		return 1 // Rage
	case 4: // Rogue
		return 3 // Energy
	case 6: // Death Knight
		return 6 // Runic Power
	default:
		return 0 // Mana
	}
}

func (s *Server) buildPlayerUpdate(state playerState) (*protocol.Packet, error) {
	values := make([]uint32, playerValuesCount)
	values[0] = uint32(state.GUID)
	values[objectFieldType] = 0x19
	values[objectFieldScale] = math.Float32bits(1)
	lvl := state.Level
	if lvl < 1 {
		lvl = 1
	} else if lvl > 80 {
		lvl = 80
	}
	values[unitFieldLevel] = uint32(lvl)
	powerType := classPowerType(state.Class)
	race := state.Race
	if race < 1 {
		race = 1
	}
	class := state.Class
	if class < 1 {
		class = 1
	}
	values[unitFieldBytes0] = uint32(race) | uint32(class)<<8 | uint32(state.Gender)<<16 | uint32(powerType)<<24
	values[unitFieldBytes1] = uint32(state.StandState)
	values[unitFieldFaction] = s.raceFaction(state.Race)
	values[unitFieldFlags] = unitFlagPlayerControlled | state.UnitFlags
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
	values[unitFieldPlayerBytes2] = uint32(state.FacialStyle) | uint32(state.SheathState)<<8 | uint32(state.BankBagSlots)<<16
	values[unitFieldGuildID] = state.GuildID
	values[unitFieldGuildRank] = uint32(state.GuildRank)
	values[unitFieldXP] = state.XP
	if state.Level > 0 && int(state.Level) < len(xpCurve) {
		values[unitFieldNextLevelXP] = xpCurve[state.Level]
	}
	values[unitFieldCoinage] = state.Money
	values[unitFieldMaxLevel] = 80
	values[unitFieldKnownCurrencies] = state.KnownCurrency
	values[unitFieldWatchedFaction] = state.WatchedFaction
	values[unitFieldChosenTitle] = state.ChosenTitle
	for i := 0; i < 6; i++ {
		values[playerFieldKnownTitles+i] = state.KnownTitles[i]
	}
	values[unitFieldAmmoID] = state.AmmoID
	if state.DuelArbiter != 0 {
		values[playerFieldDuelArbiter] = uint32(state.DuelArbiter)
		values[playerFieldDuelArbiter+1] = uint32(state.DuelArbiter >> 32)
	}
	if state.DuelTeam != 0 {
		values[playerFieldDuelTeam] = state.DuelTeam
	}
	if state.PetGUID != 0 {
		values[unitFieldSummon] = uint32(state.PetGUID)
		values[unitFieldSummon+1] = uint32(state.PetGUID >> 32)
	}
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
	values[unitFieldHealth] = maxUint32(state.Health, 1)
	values[unitFieldMaxHealth] = maxUint32(state.MaxHealth, 1)
	for i, power := range state.Powers {
		values[unitFieldPower1+i] = power
		values[unitFieldMaxPower1+i] = state.MaxPowers[i]
	}

	// Combat & Attack speeds
	attackTime := state.AttackTime
	if attackTime == 0 {
		attackTime = 2000
	}
	values[unitFieldAttackTime] = attackTime
	offhandAttackTime := state.OffhandAttackTime
	if offhandAttackTime == 0 {
		offhandAttackTime = 2000
	}
	values[unitFieldAttackTimeOffhand] = offhandAttackTime
	rangedAttackTime := state.RangedAttackTime
	if rangedAttackTime == 0 {
		rangedAttackTime = 2000
	}
	values[unitFieldRangedAttackTime] = rangedAttackTime
	values[unitModCastSpeed] = math.Float32bits(1.0)
	minDmg := state.MinDamage
	if minDmg <= 0 {
		minDmg = 1.0
	}
	maxDmg := state.MaxDamage
	if maxDmg <= minDmg {
		maxDmg = minDmg + 1.0
	}
	values[unitFieldMinDamage] = math.Float32bits(minDmg)
	values[unitFieldMaxDamage] = math.Float32bits(maxDmg)
	values[unitFieldMinOffhandDamage] = math.Float32bits(state.MinOffhandDamage)
	values[unitFieldMaxOffhandDamage] = math.Float32bits(state.MaxOffhandDamage)
	values[unitFieldMinRangedDamage] = math.Float32bits(state.MinRangedDamage)
	values[unitFieldMaxRangedDamage] = math.Float32bits(state.MaxRangedDamage)
	values[unitFieldAttackPowerMultiplier] = math.Float32bits(1.0)
	values[unitFieldRangedAttackPowerMultiplier] = math.Float32bits(1.0)
	values[unitFieldAttackPower] = state.AttackPower
	values[unitFieldRangedAttackPower] = state.RangedAttackPower

	// Base stats & Armor (displayed in character sheet)
	for i := 0; i < 5; i++ {
		statVal := state.Stats[i]
		if statVal == 0 {
			statVal = uint32(20 + int(state.Level)*2)
		}
		values[unitFieldStat0+i] = statVal
		values[unitFieldPosStat0+i] = statVal
	}
	armor := state.Armor
	if armor == 0 {
		armor = uint32((20 + int(state.Level)*2) * 2)
	}
	values[unitFieldResistances] = armor // Armor (resistance 0)

	values[unitFieldBaseHealth] = maxUint32(state.MaxHealth, 1)
	if state.BaseMana > 0 {
		values[unitFieldBaseMana] = state.BaseMana
	} else if len(state.MaxPowers) > 0 {
		values[unitFieldBaseMana] = maxUint32(state.MaxPowers[0], 1)
	}

	// Combat ratings
	for i := 0; i < 25; i++ {
		values[playerFieldCombatRating1+i] = state.CombatRatings[i]
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
	_ = mask.Set(unitFieldLevel)
	_ = mask.Set(unitFieldBytes0)
	_ = mask.Set(unitFieldMaxLevel)
	_ = mask.Set(unitFieldNextLevelXP)
	_ = mask.Set(unitFieldXP)
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

// buildPlayerValuesBlock assembles an UPDATETYPE_VALUES block (packed GUID,
// update mask, changed values) the way Object::_BuildValuesUpdate does for
// in-place field changes such as quest log slots.
func buildPlayerValuesBlock(guid uint64, fields map[int]uint32) []byte {
	values := make([]uint32, playerValuesCount)
	mask := protocol.NewUpdateMask(playerValuesCount)
	for index, value := range fields {
		if index < 0 || index >= playerValuesCount {
			continue
		}
		values[index] = value
		if err := mask.Set(index); err != nil {
			continue
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
	return block.Bytes()
}

func (s *Server) buildPlayerValuesUpdate(guid uint64, fields map[int]uint32) (*protocol.Packet, error) {
	block := buildPlayerValuesBlock(guid, fields)
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(block)
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
	return buildItemCreateBlockForLocationWithDurability(fullGUID, itemEntry, count, ownerGUID, containedGUID, containerSlots, contents, 0, 0)
}

func buildItemCreateBlockForLocationWithDurability(fullGUID uint64, itemEntry, count uint32, ownerGUID, containedGUID uint64, containerSlots uint32, contents map[uint32]uint64, curDurability, maxDurability uint32) []byte {
	if containerSlots > 36 {
		containerSlots = 36
	}
	var values []uint32
	var objectTypeId uint8
	if containerSlots > 0 {
		// TrinityCore: CONTAINER_END = ITEM_END + 0x004A = 64 + 74 = 138
		values = make([]uint32, 138)
		objectTypeId = 2 // TYPEID_CONTAINER
		// TrinityCore: TYPEMASK_OBJECT (1) | TYPEMASK_ITEM (2) | TYPEMASK_CONTAINER (4) = 7
		values[2] = 0x07
		values[64] = containerSlots
		for slot, itemGUID := range contents {
			if slot >= containerSlots {
				continue
			}
			values[66+slot*2] = uint32(itemGUID)
			values[67+slot*2] = uint32(itemGUID >> 32)
		}
	} else {
		// TrinityCore: ITEM_END = OBJECT_END + 0x003A = 6 + 58 = 64
		values = make([]uint32, 64)
		objectTypeId = 1 // TYPEID_ITEM
		// TrinityCore: TYPEMASK_OBJECT (1) | TYPEMASK_ITEM (2) = 3
		values[2] = 0x03
	}
	values[0] = uint32(fullGUID)
	values[1] = uint32(fullGUID >> 32)
	values[3] = itemEntry
	values[4] = math.Float32bits(1.0)
	values[6] = uint32(ownerGUID)
	values[7] = uint32(ownerGUID >> 32)
	values[8] = uint32(containedGUID)
	values[9] = uint32(containedGUID >> 32)
	values[14] = count
	if maxDurability > 0 {
		if curDurability == 0 {
			curDurability = maxDurability
		}
		values[60] = curDurability // ITEM_FIELD_DURABILITY = 60
		values[61] = maxDurability // ITEM_FIELD_MAXDURABILITY = 61
	} else {
		values[60] = 0
		values[61] = 0
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
	block.WriteU8(objectTypeId)
	block.WriteU16(0x0010)                        // update flags: UPDATEFLAG_LOWGUID (0x0010)
	block.WriteU32(uint32(fullGUID & 0xFFFFFFFF)) // lowguid
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for i := 0; i < len(values); i++ {
		if mask.Has(i) {
			block.WriteU32(values[i])
		}
	}
	return block.Bytes()
}

func buildContainerValuesUpdate(bagGUID uint64, containerSlots uint32, contents map[uint32]uint64) []byte {
	// TrinityCore: CONTAINER_END = 138
	totalFields := 138
	values := make([]uint32, totalFields)
	mask := protocol.NewUpdateMask(totalFields)
	for slot := uint32(0); slot < containerSlots; slot++ {
		lowIdx := int(66 + slot*2)
		highIdx := int(67 + slot*2)
		var itemGUID uint64
		if contents != nil {
			itemGUID = contents[slot]
		}
		values[lowIdx] = uint32(itemGUID)
		values[highIdx] = uint32(itemGUID >> 32)
		_ = mask.Set(lowIdx)
		_ = mask.Set(highIdx)
	}
	block := protocol.NewBuffer(64 + int(containerSlots)*8)
	block.WriteU8(protocol.UpdateValues)
	block.WritePackedGUID(bagGUID)
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index := 0; index < totalFields; index++ {
		if mask.Has(index) {
			block.WriteU32(values[index])
		}
	}
	return block.Bytes()
}

func (s *session) sendItemCreate(itemGUID uint64, itemEntry, count uint32, bag, slot uint8) error {
	fullGUID := uint64(itemGUID) | (uint64(0x4000) << 48)
	block := buildItemCreateBlock(fullGUID, itemEntry, count, s.playerGUID)
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(block)
	if (bag == 0 || bag == 255) && slot < 150 {
		fields := map[int]uint32{
			324 + int(slot)*2:     uint32(fullGUID),
			324 + int(slot)*2 + 1: uint32(fullGUID >> 32),
		}
		valBlock := buildPlayerValuesBlock(s.playerGUID, fields)
		if valBlock != nil {
			updates.AddUpdateBlock(valBlock)
		}
	}
	packet, err := updates.BuildPacket(0)
	if err != nil {
		return err
	}
	return s.write(packet.Opcode, packet.Payload.Bytes(), true)
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
		if item.bag == 0 && ((item.slot >= 19 && item.slot <= 22) || (item.slot >= 67 && item.slot <= 73)) {
			bagItems[item.itemGUID] = uint64(item.itemGUID) | (uint64(0x4000) << 48)
			bagItems[item.slot] = uint64(item.itemGUID) | (uint64(0x4000) << 48)
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
	itemTemplateInfo := func(entry int64) (uint32, uint32) {
		if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
			return 0, 0
		}
		var slots, maxD int64
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(ContainerSlots, 0), COALESCE(MaxDurability, 0) FROM item_template WHERE entry = ?", entry).Scan(&slots, &maxD)
		if slots > 36 {
			slots = 36
		}
		return uint32(slots), uint32(maxD)
	}
	itemDurability := func(guid int64) uint32 {
		var d int64
		_ = cdb.QueryRowContext(ctx, "SELECT COALESCE(durability, 0) FROM item_instance WHERE guid = ?", guid).Scan(&d)
		return uint32(d)
	}
	updates := protocol.NewUpdateData()
	fields := make(map[int]uint32)
	slotItems := make(map[int]uint64)
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
		cSlots, maxD := itemTemplateInfo(itemEntry)
		curD := itemDurability(itemGUID)
		block := buildItemCreateBlockForLocationWithDurability(fullGUID, uint32(itemEntry), uint32(count), s.playerGUID, containedGUID, cSlots, contents[int64(fullGUID)], curD, maxD)
		updates.AddUpdateBlock(block)

		if cSlots > 0 {
			valBlock := buildContainerValuesUpdate(fullGUID, cSlots, contents[int64(fullGUID)])
			updates.AddUpdateBlock(valBlock)
		}

		if bag == 0 && slot >= 0 && slot < 150 {
			slotItems[int(slot)] = fullGUID
		}
	}

	// TrinityCore: Buyback items and prices/timestamps (slots 74..85)
	for eslot := 0; eslot < 12; eslot++ {
		sl := 74 + eslot
		priceField := 1201 + eslot
		timeField := 1213 + eslot
		if bb := s.buyback[eslot]; bb != nil {
			slotItems[sl] = bb.ItemGUID
			fields[priceField] = bb.Price
			fields[timeField] = bb.Timestamp
			cSlots, maxD := itemTemplateInfo(int64(bb.ItemEntry))
			block := buildItemCreateBlockForLocationWithDurability(bb.ItemGUID, bb.ItemEntry, bb.Count, s.playerGUID, s.playerGUID, cSlots, nil, maxD, maxD)
			updates.AddUpdateBlock(block)
		} else {
			fields[priceField] = 0
			fields[timeField] = 0
		}
	}

	// TrinityCore: populate slots 0..149 so unequipped/empty slots are cleared to 0 (equipment, backpack, bank, bank bags, buyback, keyring, currency)
	for sl := 0; sl < 150; sl++ {
		invField := 324 + sl*2
		guid := slotItems[sl]
		fields[invField] = uint32(guid)
		fields[invField+1] = uint32(guid >> 32)
	}
	if s.player != nil {
		equipment := strings.Fields(s.player.Equipment)
		for slot := 0; slot < playerVisibleItemCount; slot++ {
			base := slot * 2
			itemID := uint32(0)
			enchant := uint32(0)
			if base < len(equipment) {
				if id, err := strconv.ParseUint(equipment[base], 10, 32); err == nil {
					itemID = uint32(id)
				}
			}
			if base+1 < len(equipment) {
				if enc, err := strconv.ParseUint(equipment[base+1], 10, 32); err == nil {
					enchant = uint32(enc)
				}
			}
			fields[playerVisibleItemStart+slot*2] = itemID
			fields[playerVisibleItemStart+slot*2+1] = enchant
		}
	}
	if len(fields) > 0 {
		valBlock := buildPlayerValuesBlock(s.playerGUID, fields)
		if valBlock != nil {
			updates.AddUpdateBlock(valBlock)
		}
	}
	if updates.HasData() {
		packet, err := updates.BuildPacket(0)
		if err == nil && packet != nil {
			_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
		}
	}
	return nil
}

// handleShowingCloak processes CMSG_SHOWING_CLOAK (0x2BA).
// Reference: WorldSession::HandleShowingCloakOpcode (CharacterHandler.cpp:1103).
func (s *session) handleShowingCloak(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	show := true
	if len(payload) > 0 {
		show = payload[0] != 0
	}
	const playerFlagsHideCloak = 0x800
	if show {
		s.player.PlayerFlags &^= playerFlagsHideCloak
	} else {
		s.player.PlayerFlags |= playerFlagsHideCloak
	}
	s.sendPlayerUpdate()
	return true
}

// handleShowingHelm processes CMSG_SHOWING_HELM (0x2B9).
// Reference: WorldSession::HandleShowingHelmOpcode (CharacterHandler.cpp:1096).
func (s *session) handleShowingHelm(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	show := true
	if len(payload) > 0 {
		show = payload[0] != 0
	}
	const playerFlagsHideHelm = 0x400
	if show {
		s.player.PlayerFlags &^= playerFlagsHideHelm
	} else {
		s.player.PlayerFlags |= playerFlagsHideHelm
	}
	s.sendPlayerUpdate()
	return true
}

// handleSetTitle processes CMSG_SET_TITLE (0x374).
// Reference: WorldSession::HandleSetTitleOpcode (MiscHandler.cpp:1236).
func (s *session) handleSetTitle(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	title, err := r.ReadI32()
	if err != nil || title <= 0 {
		s.player.ChosenTitle = 0
	} else {
		s.player.ChosenTitle = uint32(title)
	}
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET chosenTitle = ? WHERE guid = ?", s.player.ChosenTitle, s.playerGUID)
	}
	fields := map[int]uint32{
		unitFieldChosenTitle: s.player.ChosenTitle,
	}
	if pVal, pErr := s.server.buildPlayerValuesUpdate(s.playerGUID, fields); pErr == nil && pVal != nil {
		_ = s.write(pVal.Opcode, pVal.Payload.Bytes(), true)
	}
	s.sendPlayerUpdate()
	return true
}

// handleTogglePvP processes CMSG_TOGGLE_PVP (0x253).
// Reference: WorldSession::HandleTogglePvP (MiscHandler.cpp:485).
func (s *session) handleTogglePvP(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	const (
		playerFlagsInPvP    = 0x02
		playerFlagsPvPTimer = 0x04
	)
	if s.player.PlayerFlags&playerFlagsInPvP != 0 {
		s.player.PlayerFlags &^= playerFlagsInPvP
		s.player.PlayerFlags |= playerFlagsPvPTimer
	} else {
		s.player.PlayerFlags |= playerFlagsInPvP
		s.player.PlayerFlags &^= playerFlagsPvPTimer
	}
	s.sendPlayerUpdate()
	return true
}

// handleAcceptLevelGrant processes CMSG_ACCEPT_LEVEL_GRANT (0x420).
// Reference: WorldSession::HandleAcceptGrantLevel (ReferAFriendHandler.cpp:67).
func (s *session) handleAcceptLevelGrant(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	granterGUID, _ := r.ReadPackedGUID()

	if s.player.Level >= 80 {
		return true
	}

	s.player.Level++
	s.player.MaxHealth = 200 + uint32(s.player.Level)*50
	s.player.Health = s.player.MaxHealth
	if s.player.Powers[0] > 0 || s.player.MaxPowers[0] > 0 {
		s.player.MaxPowers[0] = 100 + uint32(s.player.Level)*40
		s.player.Powers[0] = s.player.MaxPowers[0]
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET level = ?, health = ? WHERE guid = ?", s.player.Level, s.player.Health, s.playerGUID)
		if granterGUID != 0 {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET grantableLevels = CASE WHEN grantableLevels > 0 THEN grantableLevels - 1 ELSE 0 END WHERE guid = ?", granterGUID)
		}
	}

	s.updatePetOnLevelUp(ctx)
	s.sendPlayerUpdate()
	s.debug("level granted", "account", s.accountName, "level", s.player.Level, "granter", granterGUID)
	return true
}

// handleGrantLevel processes CMSG_GRANT_LEVEL (0x41F).
// Reference: WorldSession::HandleGrantLevel (ReferAFriendHandler.cpp:24).
func (s *session) handleGrantLevel(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	r := protocol.NewReader(payload)
	targetGUID, err := r.ReadPackedGUID()
	if err != nil || targetGUID == 0 || targetGUID == s.playerGUID {
		return true
	}
	if s.server != nil {
		targetSess := s.server.findSessionByGUID(targetGUID)
		if targetSess != nil && targetSess.player != nil && targetSess.playerLoaded {
			buf := protocol.NewBuffer(9)
			buf.WritePackedGUID(s.playerGUID)
			_ = targetSess.write(uint16(protocol.OpcodeSMSG_PROPOSE_LEVEL_GRANT), buf.Bytes(), true)
		}
	}
	return true
}

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
