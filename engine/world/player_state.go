package world

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	playerValuesCount                 = 1326
	objectFieldType                   = 2
	objectFieldEntry                  = 3
	objectFieldScale                  = 4
	unitFieldHealth                   = 24
	unitFieldLevel                    = 54
	unitFieldFaction                  = 55
	unitFieldFlags                    = 59
	unitFieldAttackTime               = 62
	unitFieldAttackTimeOffhand        = 63
	unitFieldBoundingRadius           = 65
	unitFieldCombatReach              = 66
	unitFieldDisplayID                = 67
	unitFieldNativeDisplayID          = 68
	unitFieldPlayerFlags              = 150
	unitFieldPlayerBytes              = 153
	unitFieldPlayerBytes2             = 154
	unitFieldPlayerBytes3             = 155
	unitFieldGuildID                  = 151
	unitFieldGuildRank                = 152
	unitFieldGuildTimestamp           = 157
	unitFieldXP                       = 634
	unitFieldNextLevelXP              = 635
	unitFieldCoinage                  = 1170
	unitFieldMaxLevel                 = 1279
	unitFieldKnownCurrencies          = 632
	unitFieldWatchedFaction           = 1230
	unitFieldAmmoID                   = 1198
	playerVisibleItemStart            = 283
	playerVisibleItemCount            = 19
	unitFieldMaxHealth                = 32
	unitFieldMaxPower1                = 33
	unitFlagPlayerControlled   uint32 = 0x00000008
)

type playerState struct {
	GUID           uint64
	Name           string
	Race           uint8
	Class          uint8
	Gender         uint8
	Skin           uint8
	Face           uint8
	HairStyle      uint8
	HairColor      uint8
	FacialStyle    uint8
	Level          uint8
	XP             uint32
	Money          uint32
	PlayerFlags    uint32
	GuildID        uint32
	GuildRank      uint8
	Map            uint32
	X              float32
	Y              float32
	Z              float32
	Orientation    float32
	ExtraFlags     uint32
	AtLogin        uint32
	Zone           uint32
	Health         uint32
	MaxHealth      uint32
	Powers         [7]uint32
	MaxPowers      [7]uint32
	Cinematic      uint32
	KnownCurrency  uint32
	WatchedFaction uint32
	AmmoID         uint32
	ActionBars     uint32
	Spells         []learnedSpell
	Actions        [144]uint32
	Cooldowns      []spellCooldown
	Equipment      string
}

func (s *session) loadPlayerState(ctx context.Context, guid uint64) (playerState, error) {
	state := playerState{GUID: guid, Health: 1, MaxHealth: 1}
	var race, class, gender, level, playerFlags, mapID, extraFlags, atLogin, zone int64
	var equipment sql.NullString
	if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT guid, name, race, class, gender, level, playerFlags, map, position_x, position_y, position_z, orientation, extra_flags, at_login, zone, equipmentCache FROM characters WHERE guid = ? AND account = ?", guid, s.accountID).Scan(&state.GUID, &state.Name, &race, &class, &gender, &level, &playerFlags, &mapID, &state.X, &state.Y, &state.Z, &state.Orientation, &extraFlags, &atLogin, &zone, &equipment); err != nil {
		return playerState{}, err
	}
	if equipment.Valid {
		state.Equipment = equipment.String
	}
	state.Race, state.Class, state.Gender, state.Level = uint8(race), uint8(class), uint8(gender), uint8(level)
	state.PlayerFlags, state.Map, state.ExtraFlags, state.AtLogin, state.Zone = uint32(playerFlags), uint32(mapID), uint32(extraFlags), uint32(atLogin), uint32(zone)
	if err := s.loadOptionalPlayerState(ctx, &state); err != nil {
		return playerState{}, err
	}
	if err := s.CharGuild(ctx, &state); err != nil {
		return playerState{}, err
	}
	if err := s.loadPlayerPacketsState(ctx, &state); err != nil {
		return playerState{}, err
	}
	return state, nil
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
	if health > 0 {
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
	values[unitFieldPlayerFlags] = state.PlayerFlags
	values[unitFieldPlayerBytes] = uint32(state.Skin) | uint32(state.Face)<<8 | uint32(state.HairStyle)<<16 | uint32(state.HairColor)<<24
	values[unitFieldPlayerBytes2] = uint32(state.FacialStyle)
	values[unitFieldGuildID] = state.GuildID
	values[unitFieldGuildRank] = uint32(state.GuildRank)
	values[unitFieldXP] = state.XP
	values[unitFieldCoinage] = state.Money
	values[unitFieldMaxLevel] = 80
	values[unitFieldKnownCurrencies] = state.KnownCurrency
	values[unitFieldWatchedFaction] = state.WatchedFaction
	values[unitFieldAmmoID] = state.AmmoID
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
