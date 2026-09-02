package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	characterFlagHideHelm        uint32 = 0x00000400
	characterFlagHideCloak       uint32 = 0x00000800
	characterFlagGhost           uint32 = 0x00002000
	characterFlagRename          uint32 = 0x00004000
	characterFlagLockedByBilling uint32 = 0x01000000
	characterFlagDeclined        uint32 = 0x02000000
	playerFlagGhost              uint32 = 0x00000010
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
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_DEL_EXPIRED_BANS"); err != nil {
		s.debug("character enumeration cleanup failed", "account", s.accountName, "error", err)
	}
	rows, err := s.server.CharactersStore.QueryStatement(ctx, "CHAR_SEL_ENUM", 0, s.accountID)
	if err != nil {
		s.debug("character enumeration query failed", "account", s.accountName, "error", err)
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
			s.debug("character enumeration scan failed", "account", s.accountName, "error", err)
			return false
		}
		if character.Race == 0 || character.Class == 0 || character.Gender > 2 {
			continue
		}
		s.buildEnumCharacter(ctx, packet, character)
		if !character.Banned {
			s.legitimate[character.GUID] = struct{}{}
		}
		if count < 255 {
			count++
		}
	}
	if err := rows.Err(); err != nil {
		s.debug("character enumeration rows failed", "account", s.accountName, "error", err)
		return false
	}
	if err := packet.Put(0, []byte{count}); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_CHAR_ENUM), packet.Bytes(), true); err != nil {
		s.debug("character enumeration response failed", "account", s.accountName, "error", err)
		return false
	}
	s.debug("character enumeration sent", "account", s.accountName, "count", count)
	return true
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
	raceAllowed, raceRequiredExpansion := s.server.raceDefinition(race)
	classAllowed, classRequiredExpansion := s.server.classDefinition(class)
	if !raceAllowed || s.server.Config.CharacterCreatingDisabledRaceMask&(uint32(1)<<(race-1)) != 0 || !classAllowed || s.server.Config.CharacterCreatingDisabledClassMask&(uint32(1)<<(class-1)) != 0 {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateDisabled)
	}
	if raceRequiredExpansion > s.server.Config.Expansion {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), charCreateExpansion)
	}
	if classRequiredExpansion > s.server.Config.Expansion {
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

	startLevel := uint8(1)
	if s.server.Config.StartPlayerLevel > 0 {
		startLevel = uint8(s.server.Config.StartPlayerLevel)
	}
	if class == 6 { // Death Knight
		if s.server.Config.StartDeathKnightPlayerLevel > 0 {
			startLevel = uint8(s.server.Config.StartDeathKnightPlayerLevel)
		} else {
			startLevel = 55
		}
	}
	startMoney := s.server.Config.StartPlayerMoney
	startHonor := s.server.Config.StartHonorPoints
	startArena := s.server.Config.StartArenaPoints

	args := []any{
		uint32(guid), s.accountID, name, race, class, gender,
		startLevel, uint32(0), startMoney,
		values[3], values[4], values[5], values[6], values[7],
		uint8(0), uint8(0), uint32(0), uint16(spawn.Map), uint32(0), uint8(0),
		spawn.X, spawn.Y, spawn.Z, spawn.Orientation,
		float32(0), float32(0), float32(0), float32(0), uint32(0), "",
		uint8(0), // cinematic = 0
		uint32(0), uint32(0), float32(0), uint32(0), uint8(0), uint32(0), uint32(0),
		uint16(0), uint8(0), uint16(atLoginFirst), uint16(spawn.Zone), uint32(0), "",
		startArena, startHonor, uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0),
		uint64(0), uint32(0), uint8(0), uint32(1),
		uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0),
		uint8(1), uint8(0), "", "", uint32(0), "", uint8(0), uint32(0),
	}
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_INS_CHARACTER", args...); err != nil {
		return sendCharacterResult(s, uint16(protocol.OpcodeSMSG_CHAR_CREATE), 48)
	}

	// Populate starter spells, skills, actions, equipment
	s.createStarterSpells(ctx, guid, race, class)
	s.createStarterSkills(ctx, guid, race, class)
	s.createStarterActions(ctx, guid, race, class)
	s.createStarterOutfit(ctx, guid, race, class, gender)

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
	guid, err := b.ReadU64()
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

func (s *session) handlePlayerLogin(ctx context.Context, payload []byte) (success bool) {
	var guid uint64
	defer func() {
		if !success {
			s.debug("player login failed", "account", s.accountName, "guid", guid)
		}
	}()
	if s.playerLoaded {
		return false
	}
	b := protocol.NewReader(payload)
	guid, err := b.ReadU64()
	if err != nil {
		return false
	}
	if _, ok := s.legitimate[guid]; !ok {
		return false
	}
	state, err := s.loadPlayerState(ctx, guid)
	if err != nil {
		return false
	}
	mounts, err := s.loadMountState(ctx, guid)
	if err != nil {
		return false
	}
	s.mounts = mounts

	// Stream core login verification and capabilities
	if err := s.write(uint16(protocol.OpcodeSMSG_LOGIN_VERIFY_WORLD), buildLoginVerifyWorld(state), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_ACCOUNT_DATA_TIMES), buildAccountDataTimes(time.Now(), characterAccountDataMask), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_FEATURE_SYSTEM_STATUS), buildFeatureSystemStatus(), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_MOTD), buildMotd(s.server.Config.Motd), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_LEARNED_DANCE_MOVES), buildLearnedDanceMoves(), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_INSTANCE_DIFFICULTY), buildInstanceDifficulty(), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_INITIAL_SPELLS), buildInitialSpells(state), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_SEND_UNLEARN_SPELLS), buildUnlearnSpells(), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_ACTION_BUTTONS), buildActionButtons(state.Actions), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_INITIALIZE_FACTIONS), buildInitialReputations(state), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_SET_FORCED_REACTIONS), buildForcedReactions(), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_TUTORIAL_FLAGS), buildTutorialFlags(s.tutorials), true); err != nil {
		return false
	}
	if state.Cinematic == 0 {
		cinematicID := getStartingCinematicID(state.Race, state.Class)
		if cinematicID > 0 {
			cinematicBuf := protocol.NewBuffer(4)
			cinematicBuf.WriteU32(cinematicID)
			_ = s.write(uint16(protocol.OpcodeSMSG_TRIGGER_CINEMATIC), cinematicBuf.Bytes(), true)
		}
		state.Cinematic = 1
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET cinematic = 1 WHERE guid = ?", guid)
	}
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_CHAR_ONLINE", guid); err != nil {
		return false
	}
	if _, err := s.server.AuthStore.ExecStatement(ctx, "LOGIN_UPD_ACCOUNT_ONLINE", s.accountID); err != nil {
		return false
	}
	s.playerGUID = guid
	s.player = &state
	s.playerLoaded = true
	s.triggerPlayerEvent(ctx, scripting.PlayerEventLogin, s.luaPlayer())
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
	updates, err := s.server.buildPlayerUpdate(state)
	if err != nil {
		return false
	}
	if err := s.write(updates.Opcode, updates.Payload.Bytes(), true); err != nil {
		return false
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_INIT_WORLD_STATES), buildInitWorldStates(state), true); err != nil {
		return false
	}

	// Concurrently query nearby creatures and gameobjects
	var nearbyCreatures, nearbyGameObjects *protocol.Packet
	var creatureCount, goCount int
	var creatureErr, goErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		nearbyCreatures, creatureCount, creatureErr = s.server.buildNearbyCreatureUpdates(ctx, state)
	}()
	go func() {
		defer wg.Done()
		nearbyGameObjects, goCount, goErr = s.server.buildNearbyGameObjectUpdates(ctx, state)
	}()
	wg.Wait()

	if creatureErr != nil {
		s.debug("nearby creature load failed", "account", s.accountName, "error", creatureErr)
		return false
	} else if nearbyCreatures != nil {
		if err := s.write(nearbyCreatures.Opcode, nearbyCreatures.Payload.Bytes(), true); err != nil {
			return false
		}
		s.debug("nearby creatures sent", "account", s.accountName, "count", creatureCount)
	}
	if goErr != nil {
		s.debug("nearby gameobjects load failed", "account", s.accountName, "error", goErr)
		return false
	} else if nearbyGameObjects != nil {
		if err := s.write(nearbyGameObjects.Opcode, nearbyGameObjects.Payload.Bytes(), true); err != nil {
			return false
		}
		s.debug("nearby gameobjects sent", "account", s.accountName, "count", goCount)
	}
	s.lastStreamX, s.lastStreamY, s.lastStreamZ = state.X, state.Y, state.Z
	_ = s.sendInventoryItems(ctx)
	if err := s.write(uint16(protocol.OpcodeSMSG_TIME_SYNC_REQ), buildTimeSyncRequest(0), true); err != nil {
		return false
	}
	s.debug("player login complete", "account", s.accountName, "guid", s.playerGUID, "map", state.Map, "x", state.X, "y", state.Y, "z", state.Z)
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

func (s *session) buildEnumCharacter(ctx context.Context, packet *protocol.Buffer, c enumCharacter) {
	packet.WriteU64(c.GUID)
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
	if c.PlayerFlags&playerFlagGhost != 0 {
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
	if c.PetEntry != 0 && c.PlayerFlags&playerFlagGhost == 0 && (c.Class == 3 || c.Class == 6 || c.Class == 9) {
		petDisplay, petLevel = c.PetDisplay, c.PetLevel
	}
	packet.WriteU32(petDisplay)
	packet.WriteU32(petLevel)
	packet.WriteU32(petFamily)
	equipment := strings.Fields(c.Equipment)
	for slot := 0; slot < inventorySlotBagEnd; slot++ {
		itemIndex := slot * 2
		if itemIndex >= len(equipment) {
			writeEmptyEnumEquipment(packet)
			continue
		}
		itemID, err := strconv.ParseUint(equipment[itemIndex], 10, 32)
		if err != nil || itemID == 0 || s.server == nil || s.server.WorldStore == nil {
			writeEmptyEnumEquipment(packet)
			continue
		}
		var displayID, inventoryType int64
		err = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT displayid, InventoryType FROM item_template WHERE entry = ? LIMIT 1", itemID).Scan(&displayID, &inventoryType)
		if err != nil {
			writeEmptyEnumEquipment(packet)
			continue
		}
		packet.WriteU32(uint32(displayID))
		packet.WriteU8(uint8(inventoryType))
		packet.WriteU32(0)
	}
}

func writeEmptyEnumEquipment(packet *protocol.Buffer) {
	packet.WriteU32(0)
	packet.WriteU8(0)
	packet.WriteU32(0)
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

func (s *Server) raceDefinition(id uint8) (bool, uint32) {
	if s.Data != nil {
		if race, found, err := s.Data.Race(uint32(id)); err == nil && found {
			return wotlk.IsPlayableRace(race), race.RequiredExpansion
		}
	}
	return playableRace(id), raceExpansion(id)
}

func (s *Server) classDefinition(id uint8) (bool, uint32) {
	if s.Data != nil {
		if class, found, err := s.Data.Class(uint32(id)); err == nil && found {
			return class.ID != 0, class.RequiredExpansion
		}
	}
	return playableClass(id), classExpansion(id)
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

func (s *session) loadMountState(ctx context.Context, guid uint64) (*MountState, error) {
	var extraFlags int64
	if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT extra_flags FROM characters WHERE guid = ? AND account = ?", guid, s.accountID).Scan(&extraFlags); err != nil {
		return nil, err
	}
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT spell FROM character_spell WHERE guid = ? AND active <> 0 AND disabled = 0 ORDER BY spell", guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	spells := make([]LearnedMountSpell, 0)
	for rows.Next() {
		var spellID uint32
		if err := rows.Scan(&spellID); err != nil {
			return nil, err
		}
		if s.server.Data == nil {
			continue
		}
		spell, found, err := s.server.Data.Spell(spellID)
		if err != nil || !found {
			continue
		}
		for _, effect := range spell.Effects {
			if effect.Aura == wotlk.MountedFlightSpeedAura {
				spells = append(spells, LearnedMountSpell{ID: spellID, MountedFlightSpeed: int(effect.BasePoints + 1)})
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return NewMountState(uint32(extraFlags), spells), nil
}

func (s *session) LearnMountSpell(ctx context.Context, guid uint64, spellID uint32) error {
	if s.mounts == nil || s.server.Data == nil {
		return fmt.Errorf("mount data is unavailable")
	}
	spell, found, err := s.server.Data.Spell(spellID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("spell %d not found", spellID)
	}
	for _, effect := range spell.Effects {
		if effect.Aura == wotlk.MountedFlightSpeedAura {
			s.mounts.LearnSpell(LearnedMountSpell{ID: spellID, MountedFlightSpeed: int(effect.BasePoints + 1)})
			_, err = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET extra_flags = ? WHERE guid = ? AND account = ?", s.mounts.ExtraFlags(), guid, s.accountID)
			return err
		}
	}
	return nil
}

func getStartingCinematicID(race, class uint8) uint32 {
	if class == 6 { // Death Knight
		return 165
	}
	switch race {
	case 1: // Human
		return 81
	case 2: // Orc
		return 21
	case 3: // Dwarf
		return 41
	case 4: // Night Elf
		return 61
	case 5: // Undead
		return 2
	case 6: // Tauren
		return 141
	case 7: // Gnome
		return 101
	case 8: // Troll
		return 121
	case 10: // Blood Elf
		return 162
	case 11: // Draenei
		return 163
	default:
		return 0
	}
}

func (s *session) createStarterOutfit(ctx context.Context, guid uint64, race, class, gender uint8) {
	cdb := s.server.CharactersStore.DB
	wdb := s.server.WorldStore.DB
	if cdb == nil || wdb == nil {
		return
	}

	var itemIDs []uint32
	seen := make(map[uint32]bool)

	// 1. Get starter items from CharStartOutfit DBC
	if s.server.Data != nil {
		if outfit, err := s.server.Data.CharStartOutfit(race, class, gender); err == nil && len(outfit) > 0 {
			for _, id := range outfit {
				if id > 0 && !seen[id] {
					seen[id] = true
					itemIDs = append(itemIDs, id)
				}
			}
		}
	}

	// 2. Also check custom items from playercreateinfo_item
	rows, err := wdb.QueryContext(ctx, "SELECT itemid, amount FROM playercreateinfo_item WHERE (race = ? OR race = 0) AND (class = ? OR class = 0)", race, class)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var customID, amount int64
			if err := rows.Scan(&customID, &amount); err == nil && customID > 0 {
				id := uint32(customID)
				if !seen[id] {
					seen[id] = true
					itemIDs = append(itemIDs, id)
				}
			}
		}
	}

	if len(itemIDs) == 0 {
		return
	}

	occupiedSlots := make(map[uint8]bool)
	equippedSlots := make([]uint32, equipSlotEnd)
	firstBackpackSlot := uint8(23)

	for _, itemEntry := range itemIDs {
		var invType, buyCount, itemClass, itemSubclass int64
		err := wdb.QueryRowContext(ctx, "SELECT InventoryType, BuyCount, class, subclass FROM item_template WHERE entry = ?", itemEntry).Scan(&invType, &buyCount, &itemClass, &itemSubclass)
		if err != nil {
			continue
		}

		slot := inventoryTypeToSlot(uint8(invType))
		targetBag := uint8(0)
		targetSlot := uint8(0)

		if slot < equipSlotEnd && !occupiedSlots[slot] {
			targetSlot = slot
			occupiedSlots[slot] = true
			equippedSlots[slot] = itemEntry
		} else if slot == equipSlotFinger1 && !occupiedSlots[equipSlotFinger2] {
			targetSlot = equipSlotFinger2
			occupiedSlots[equipSlotFinger2] = true
			equippedSlots[equipSlotFinger2] = itemEntry
		} else if slot == equipSlotTrinket1 && !occupiedSlots[equipSlotTrinket2] {
			targetSlot = equipSlotTrinket2
			occupiedSlots[equipSlotTrinket2] = true
			equippedSlots[equipSlotTrinket2] = itemEntry
		} else if slot == equipSlotMainhand && !occupiedSlots[equipSlotOffhand] && (invType == 13 || invType == 22 || invType == 23) {
			targetSlot = equipSlotOffhand
			occupiedSlots[equipSlotOffhand] = true
			equippedSlots[equipSlotOffhand] = itemEntry
		} else {
			// Find first empty backpack slot in 23..38
			found := false
			for bpSlot := firstBackpackSlot; bpSlot <= 38; bpSlot++ {
				if !occupiedSlots[bpSlot] {
					targetSlot = bpSlot
					occupiedSlots[bpSlot] = true
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		count := int64(1)
		if buyCount > 1 {
			count = buyCount
		}

		var nextItemGUID uint64
		if err := cdb.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&nextItemGUID); err != nil || nextItemGUID == 0 {
			nextItemGUID = uint64(time.Now().UnixNano())
		}

		insItemQuery := "INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text) VALUES (?, ?, ?, 0, ?, 0, 0, 0, '', 0, 100, 0, '')"
		if _, err := cdb.ExecContext(ctx, insItemQuery, nextItemGUID, itemEntry, guid, count); err != nil {
			continue
		}

		insInvQuery := "INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, ?, ?, ?)"
		_, _ = cdb.ExecContext(ctx, insInvQuery, guid, targetBag, targetSlot, nextItemGUID)
	}

	// Update equipmentCache in characters table
	parts := make([]string, equipSlotEnd*2)
	for i := 0; i < int(equipSlotEnd); i++ {
		parts[i*2] = strconv.FormatUint(uint64(equippedSlots[i]), 10)
		parts[i*2+1] = "0"
	}
	cacheStr := strings.Join(parts, " ")
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET equipmentCache = ? WHERE guid = ?", cacheStr, guid)
}

func (s *session) createStarterSpells(ctx context.Context, guid uint64, race, class uint8) {
	cdb := s.server.CharactersStore.DB
	wdb := s.server.WorldStore.DB
	if cdb == nil || wdb == nil {
		return
	}

	spells := defaultRacialSpells(race)
	seen := make(map[uint32]bool)
	for _, sp := range spells {
		seen[sp.ID] = true
		_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", guid, sp.ID)
	}

	raceMask, classMask := playerCreateMask(race), playerCreateMask(class)
	rows, err := wdb.QueryContext(ctx, "SELECT Spell FROM playercreateinfo_spell_custom WHERE (racemask = 0 OR (racemask & ?) <> 0) AND (classmask = 0 OR (classmask & ?) <> 0)", raceMask, classMask)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var spellID int64
			if err := rows.Scan(&spellID); err == nil && spellID > 0 {
				id := uint32(spellID)
				if !seen[id] {
					seen[id] = true
					_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", guid, id)
				}
			}
		}
	}
}

func (s *session) createStarterSkills(ctx context.Context, guid uint64, race, class uint8) {
	cdb := s.server.CharactersStore.DB
	wdb := s.server.WorldStore.DB
	if cdb == nil || wdb == nil {
		return
	}

	var racialLangSkill uint32
	switch race {
	case 1:
		racialLangSkill = 98 // Common
	case 2:
		racialLangSkill = 109 // Orcish
	case 3:
		racialLangSkill = 111 // Dwarven
	case 4:
		racialLangSkill = 113 // Darnassian
	case 5:
		racialLangSkill = 673 // Gutterspeak
	case 6:
		racialLangSkill = 115 // Taurahe
	case 7:
		racialLangSkill = 313 // Gnomish
	case 8:
		racialLangSkill = 315 // Troll
	case 10:
		racialLangSkill = 137 // Thalassian
	case 11:
		racialLangSkill = 759 // Draenei
	}
	if racialLangSkill != 0 {
		_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, ?, 300, 300)", guid, racialLangSkill)
	}
	if race == 3 || race == 4 || race == 7 || race == 11 {
		_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, 98, 300, 300)", guid)
	} else if race == 5 || race == 6 || race == 8 || race == 10 {
		_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, 109, 300, 300)", guid)
	}

	raceMask, classMask := playerCreateMask(race), playerCreateMask(class)
	rows, err := wdb.QueryContext(ctx, "SELECT skill, rank FROM playercreateinfo_skills WHERE (raceMask = 0 OR (raceMask & ?) <> 0) AND (classMask = 0 OR (classMask & ?) <> 0)", raceMask, classMask)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var skillID, rank int64
			if err := rows.Scan(&skillID, &rank); err == nil && skillID > 0 {
				val := 1
				max := 300
				if rank > 0 {
					val = int(rank)
				}
				_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_skills (guid, skill, value, max) VALUES (?, ?, ?, ?)", guid, skillID, val, max)
			}
		}
	}
}

func (s *session) createStarterActions(ctx context.Context, guid uint64, race, class uint8) {
	cdb := s.server.CharactersStore.DB
	wdb := s.server.WorldStore.DB
	if cdb == nil || wdb == nil {
		return
	}
	rows, err := wdb.QueryContext(ctx, "SELECT button, action, type FROM playercreateinfo_action WHERE race = ? AND class = ?", race, class)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var button, action, kind int64
			if err := rows.Scan(&button, &action, &kind); err == nil {
				_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_action (guid, spec, button, action, type) VALUES (?, 0, ?, ?, ?)", guid, button, action, kind)
			}
		}
	}
}
