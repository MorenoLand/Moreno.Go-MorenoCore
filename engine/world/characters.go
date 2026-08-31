package world

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	characterFlagHideHelm        uint32 = 0x00000400
	characterFlagHideCloak       uint32 = 0x00000800
	characterFlagGhost           uint32 = 0x00002000
	characterFlagRename          uint32 = 0x00004000
	characterFlagLockedByBilling uint32 = 0x01000000
	characterFlagDeclined        uint32 = 0x02000000
	characterCustomizeNone       uint32 = 0
	characterCustomizeCustomize  uint32 = 0x00000001
	characterCustomizeFaction    uint32 = 0x00010000
	characterCustomizeRace       uint32 = 0x00100000
	atLoginRename                uint64 = 0x001
	atLoginCustomize             uint64 = 0x008
	atLoginFirst                 uint64 = 0x020
	atLoginChangeFaction         uint64 = 0x040
	atLoginChangeRace            uint64 = 0x080
	inventorySlotBagEnd                 = 23
	charCreateSuccess                   = 47
	charCreateError                     = 48
	charCreateFailed                    = 49
	charCreateDisabled                  = 51
	charCreateServerLimit               = 53
	charCreateAccountLimit              = 54
	charCreateExpansion                 = 57
	charCreateExpansionClass            = 58
	charCreateLevelRequirement          = 59
	charCreateUniqueClassLimit          = 60
	charCreateNameInUse                 = 50
	charDeleteSuccess                   = 71
	charDeleteFailed                    = 72
)

type enumCharacter struct {
	GUID        uint64
	Name        string
	Race        uint8
	Class       uint8
	Gender      uint8
	Skin        uint8
	Face        uint8
	HairStyle   uint8
	HairColor   uint8
	FacialStyle uint8
	Level       uint8
	Zone        uint32
	Map         uint32
	X           float32
	Y           float32
	Z           float32
	GuildID     uint32
	PlayerFlags uint32
	AtLogin     uint16
	PetEntry    uint32
	PetDisplay  uint32
	PetLevel    uint32
	Equipment   string
	Banned      bool
}

func (s *session) handleCharEnum(ctx context.Context) bool {
	_, _ = s.server.CharactersStore.ExecStatement(ctx, "CHAR_DEL_EXPIRED_BANS")
	rows, err := s.server.CharactersStore.QueryStatement(ctx, "CHAR_SEL_ENUM", 0, s.accountID)
	if err != nil {
		return false
	}
	defer rows.Close()
	packet := protocol.NewBuffer(1 + 128)
	packet.WriteU8(0)
	s.legitimate = make(map[uint64]struct{})
	count := uint8(0)
	for rows.Next() {
		character, err := scanEnumCharacter(rows)
		if err != nil {
			return false
		}
		if character.Race == 0 || character.Class == 0 || character.Gender > 2 {
			continue
		}
		buildEnumCharacter(packet, character)
		if !character.Banned {
			s.legitimate[character.GUID] = struct{}{}
		}
		if count < 255 {
			count++
		}
	}
	if err := rows.Err(); err != nil {
		return false
	}
	if err := packet.Put(0, []byte{count}); err != nil {
		return false
	}
	return s.write(uint16(protocol.OpcodeSMSG_CHAR_ENUM), packet.Bytes(), true) == nil
}

func (s *session) handleCharCreate(ctx context.Context, payload []byte) bool {
	b := protocol.NewReader(payload)
	name, err := b.ReadCString()
	if err != nil {
		return false
	}
	values := make([]uint8, 9)
	for i := range values {
		values[i], err = b.ReadU8()
		if err != nil {
			return false
		}
	}
	race, class, gender := values[0], values[1], values[2]
	if !validCharacterName(name) || race == 0 || class == 0 || gender > 2 {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateFailed)
	}
	if disabled := s.server.Config.CharacterCreatingDisabled; disabled != 0 {
		team := raceTeam(race)
		if (team == 1 && disabled&1 != 0) || (team == 2 && disabled&2 != 0) {
			return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateDisabled)
		}
	}
	if !playableRace(race) || s.server.Config.CharacterCreatingDisabledRaceMask&(uint32(1)<<(race-1)) != 0 || !playableClass(class) || s.server.Config.CharacterCreatingDisabledClassMask&(uint32(1)<<(class-1)) != 0 {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateDisabled)
	}
	if raceExpansion(race) > s.server.Config.Expansion {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateExpansion)
	}
	if classExpansion(class) > s.server.Config.Expansion {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateExpansionClass)
	}
	if class == 6 {
		if s.server.Config.DeathKnightsPerRealm == 0 {
			return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateUniqueClassLimit)
		}
		var deathKnights int64
		if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COUNT(guid) FROM characters WHERE account = ? AND class = ?", s.accountID, class).Scan(&deathKnights); err != nil {
			return false
		}
		if deathKnights >= int64(s.server.Config.DeathKnightsPerRealm) {
			return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateUniqueClassLimit)
		}
		if required := s.server.Config.CharacterCreatingMinLevelForDeathKnight; required > 0 {
			var maxLevel sql.NullInt64
			if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT MAX(level) FROM characters WHERE account = ? AND class <> ?", s.accountID, class).Scan(&maxLevel); err != nil {
				return false
			}
			if !maxLevel.Valid || maxLevel.Int64 < int64(required) {
				return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateLevelRequirement)
			}
		}
	}
	row, err := s.server.CharactersStore.QueryRowStatement(ctx, "CHAR_SEL_CHECK_NAME", name)
	if err != nil {
		return false
	}
	var exists int64
	if err := row.Scan(&exists); err == nil {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), 50)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false
	}
	var spawn struct {
		Map, Zone            uint32
		X, Y, Z, Orientation float32
	}
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT map, zone, position_x, position_y, position_z, orientation FROM playercreateinfo WHERE race = ? AND class = ?", race, class).Scan(&spawn.Map, &spawn.Zone, &spawn.X, &spawn.Y, &spawn.Z, &spawn.Orientation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), 49)
		}
		return false
	}
	var accountCharacters int64
	if err := s.server.CharactersStore.QueryRowContext(ctx, "SELECT COUNT(guid) FROM characters WHERE account = ?", s.accountID).Scan(&accountCharacters); err != nil {
		return false
	}
	if accountCharacters >= int64(s.server.Config.CharactersPerAccount) {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateAccountLimit)
	}
	var realmCharacterCount int64
	if err := s.server.CharactersStore.QueryRowContext(ctx, "SELECT COUNT(guid) FROM characters WHERE account = ?", s.accountID).Scan(&realmCharacterCount); err != nil {
		return false
	}
	if realmCharacterCount >= int64(s.server.Config.CharactersPerRealm) {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateServerLimit)
	}
	var guid uint64
	if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM characters").Scan(&guid); err != nil {
		return false
	}
	args := []any{uint32(guid), s.accountID, name, race, class, gender, uint8(1), uint32(0), uint32(0), values[3], values[4], values[5], values[6], values[7], uint8(0), uint8(0), uint32(0), uint16(spawn.Map), uint32(0), uint8(0), spawn.X, spawn.Y, spawn.Z, spawn.Orientation, float32(0), float32(0), float32(0), float32(0), uint32(0), "", uint8(0), uint32(0), uint32(0), float32(0), uint32(0), uint8(0), uint32(0), uint32(0), uint16(0), uint8(0), uint16(atLoginFirst), uint16(spawn.Zone), uint32(0), "", uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint64(0), uint32(0), uint8(0), uint32(1), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint8(1), uint8(0), "", "", uint32(0), "", uint8(0), uint32(0)}
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_INS_CHARACTER", args...); err != nil {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), 48)
	}
	var realmCharacters sql.NullInt64
	if err := s.server.AuthStore.QueryRowContext(ctx, "SELECT SUM(numchars) FROM realmcharacters WHERE acctid = ?", s.accountID).Scan(&realmCharacters); err != nil {
		return false
	}
	count := uint32(1)
	if realmCharacters.Valid && realmCharacters.Int64 > 0 {
		count = uint32(realmCharacters.Int64) + 1
	}
	if _, err := s.server.AuthStore.ExecStatement(ctx, "LOGIN_REP_REALM_CHARACTERS", count, s.accountID, s.server.RealmID); err != nil {
		return false
	}
	s.legitimate[guid] = struct{}{}
	return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), 47)
}

func (s *session) handleCharDelete(ctx context.Context, payload []byte) bool {
	b := protocol.NewReader(payload)
	guid, err := b.ReadPackedGUID()
	if err != nil {
		return false
	}
	if _, ok := s.legitimate[guid]; !ok {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_DELETE), 72)
	}
	var accountID uint32
	err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT account FROM characters WHERE guid = ?", guid).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) || err != nil || accountID != s.accountID {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_DELETE), 72)
	}
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_DEL_CHARACTER", guid); err != nil {
		return false
	}
	delete(s.legitimate, guid)
	return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_DELETE), 71)
}

func (s *session) handlePlayerLogin(ctx context.Context, payload []byte) bool {
	b := protocol.NewReader(payload)
	guid, err := b.ReadPackedGUID()
	if err != nil {
		return false
	}
	if _, ok := s.legitimate[guid]; !ok {
		return false
	}
	var mapID uint32
	var x, y, z, orientation float32
	err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT map, position_x, position_y, position_z, orientation FROM characters WHERE guid = ? AND account = ?", guid, s.accountID).Scan(&mapID, &x, &y, &z, &orientation)
	if err != nil {
		return false
	}
	packet := protocol.NewBuffer(20)
	packet.WriteU32(mapID)
	packet.WriteF32(x)
	packet.WriteF32(y)
	packet.WriteF32(z)
	packet.WriteF32(orientation)
	if err := s.write(uint16(protocol.OpcodeSMSG_NEW_WORLD), packet.Bytes(), true); err != nil {
		return false
	}
	timePacket := protocol.NewBuffer(12)
	timePacket.WritePackedTime(time.Now())
	timePacket.WriteF32(0.5)
	timePacket.WriteU32(0)
	if err := s.write(uint16(protocol.OpcodeSMSG_LOGIN_SET_TIME_SPEED), timePacket.Bytes(), true); err != nil {
		return false
	}
	s.server.Features.OnPlayerLogin()
	if s.server.Config.SoloLFGAnnounce {
		message := protocol.BuildSystemChatMessage("This server is running |cff4CFF00Solo Dungeon Finder|r module.")
		if err := s.write(uint16(protocol.OpcodeSMSG_MESSAGECHAT), message, true); err != nil {
			return false
		}
	}
	return true
}

func scanEnumCharacter(rows *sql.Rows) (enumCharacter, error) {
	var c enumCharacter
	var guid uint32
	var race, class, gender, skin, face, hairStyle, hairColor, facialStyle, level, zone, mapID int64
	var x, y, z float64
	var guildID, playerFlags, atLogin, petEntry, petDisplay, petLevel, banned sql.NullInt64
	var equipment sql.NullString
	err := rows.Scan(&guid, &c.Name, &race, &class, &gender, &skin, &face, &hairStyle, &hairColor, &facialStyle, &level, &zone, &mapID, &x, &y, &z, &guildID, &playerFlags, &atLogin, &petEntry, &petDisplay, &petLevel, &equipment, &banned)
	if err != nil {
		return c, err
	}
	c.GUID = uint64(guid)
	c.Race, c.Class, c.Gender, c.Skin, c.Face = uint8(race), uint8(class), uint8(gender), uint8(skin), uint8(face)
	c.HairStyle, c.HairColor, c.FacialStyle, c.Level = uint8(hairStyle), uint8(hairColor), uint8(facialStyle), uint8(level)
	c.Zone, c.Map = uint32(zone), uint32(mapID)
	c.X, c.Y, c.Z = float32(x), float32(y), float32(z)
	if guildID.Valid {
		c.GuildID = uint32(guildID.Int64)
	}
	if playerFlags.Valid {
		c.PlayerFlags = uint32(playerFlags.Int64)
	}
	if atLogin.Valid {
		c.AtLogin = uint16(atLogin.Int64)
	}
	if petEntry.Valid {
		c.PetEntry = uint32(petEntry.Int64)
	}
	if petDisplay.Valid {
		c.PetDisplay = uint32(petDisplay.Int64)
	}
	if petLevel.Valid {
		c.PetLevel = uint32(petLevel.Int64)
	}
	if equipment.Valid {
		c.Equipment = equipment.String
	}
	if banned.Valid {
		c.Banned = banned.Int64 != 0
	}
	return c, nil
}

func buildEnumCharacter(packet *protocol.Buffer, c enumCharacter) {
	packet.WritePackedGUID(c.GUID)
	packet.WriteCString(c.Name)
	packet.WriteU8(c.Race)
	packet.WriteU8(c.Class)
	packet.WriteU8(c.Gender)
	packet.WriteU8(c.Skin)
	packet.WriteU8(c.Face)
	packet.WriteU8(c.HairStyle)
	packet.WriteU8(c.HairColor)
	packet.WriteU8(c.FacialStyle)
	packet.WriteU8(c.Level)
	packet.WriteU32(c.Zone)
	packet.WriteU32(c.Map)
	packet.WriteF32(c.X)
	packet.WriteF32(c.Y)
	packet.WriteF32(c.Z)
	packet.WriteU32(c.GuildID)
	flags := uint32(0)
	if c.PlayerFlags&characterFlagHideHelm != 0 {
		flags |= characterFlagHideHelm
	}
	if c.PlayerFlags&characterFlagHideCloak != 0 {
		flags |= characterFlagHideCloak
	}
	if c.PlayerFlags&characterFlagGhost != 0 {
		flags |= characterFlagGhost
	}
	if c.AtLogin&uint16(atLoginRename) != 0 {
		flags |= characterFlagRename
	}
	if c.Banned {
		flags |= characterFlagLockedByBilling
	}
	flags |= characterFlagDeclined
	packet.WriteU32(flags)
	customize := characterCustomizeNone
	if c.AtLogin&uint16(atLoginCustomize) != 0 {
		customize = characterCustomizeCustomize
	}
	if c.AtLogin&uint16(atLoginChangeFaction) != 0 {
		customize = characterCustomizeFaction
	}
	if c.AtLogin&uint16(atLoginChangeRace) != 0 {
		customize = characterCustomizeRace
	}
	packet.WriteU32(customize)
	if c.AtLogin&uint16(atLoginFirst) != 0 {
		packet.WriteU8(1)
	} else {
		packet.WriteU8(0)
	}
	petDisplay, petLevel, petFamily := uint32(0), uint32(0), uint32(0)
	if c.PetEntry != 0 && c.PlayerFlags&characterFlagGhost == 0 && (c.Class == 3 || c.Class == 6 || c.Class == 9) {
		petDisplay, petLevel = c.PetDisplay, c.PetLevel
	}
	packet.WriteU32(petDisplay)
	packet.WriteU32(petLevel)
	packet.WriteU32(petFamily)
	for i := 0; i < inventorySlotBagEnd; i++ {
		packet.WriteU32(0)
		packet.WriteU8(0)
		packet.WriteU32(0)
	}
	_ = strings.TrimSpace(c.Equipment)
}

func sendCharacterResult(s *session, opcode uint16, result uint8) bool {
	return s.write(opcode, []byte{result}, true) == nil
}

func validCharacterName(name string) bool {
	runes := []rune(name)
	if len(runes) < 2 || len(runes) > 12 {
		return false
	}
	for _, r := range runes {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func playableRace(race uint8) bool {
	switch race {
	case 1, 2, 3, 4, 5, 6, 7, 8, 10, 11:
		return true
	default:
		return false
	}
}

func playableClass(class uint8) bool {
	switch class {
	case 1, 2, 3, 4, 5, 6, 7, 8, 9, 11:
		return true
	default:
		return false
	}
}

func raceTeam(race uint8) uint8 {
	switch race {
	case 1, 3, 4, 7, 11:
		return 1
	case 2, 5, 6, 8, 10:
		return 2
	default:
		return 0
	}
}

func raceExpansion(race uint8) uint32 {
	if race == 10 || race == 11 {
		return 1
	}
	return 0
}

func classExpansion(class uint8) uint32 {
	if class == 6 {
		return 2
	}
	return 0
}
