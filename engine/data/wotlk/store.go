package wotlk

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/dbc"
)

type Store struct {
	Dir   string
	mu    sync.RWMutex
	files map[string]*dbc.File

	taxiOnce sync.Once
	taxi     *taxiNetwork
	taxiErr  error
}

const MountedFlightSpeedAura uint32 = 207

type Race struct {
	ID                uint32
	Flags             uint32
	FactionID         uint32
	MaleDisplayID     uint32
	FemaleDisplayID   uint32
	CinematicSequence uint32
	Alliance          uint32
	RequiredExpansion uint32
}

type Class struct {
	ID                uint32
	SpellClassSet     uint32
	CinematicSequence uint32
	RequiredExpansion uint32
}

type Reputation struct {
	ID             uint32
	ReputationList int32
	BaseStanding   int32
	DefaultFlags   uint8
}

type FactionTemplate struct {
	ID           uint32
	Faction      uint32
	Flags        uint32
	FactionGroup uint32
	FriendGroup  uint32
	EnemyGroup   uint32
	Enemies      [4]uint32
	Friends      [4]uint32
}

type SpellEffect struct {
	Effect          uint32
	BasePoints      int32
	Aura            uint32
	AuraPeriod      uint32
	ImplicitTargetA uint32
	ImplicitTargetB uint32
	RadiusIndex     uint32
	MiscValue       int32
	TriggerSpell    uint32
}

type Spell struct {
	ID               uint32
	Attributes       uint32
	AttributesEx1    uint32
	SchoolMask       uint32
	Targets          uint32
	CastingTimeIndex uint32
	RecoveryTime     uint32
	PowerType        uint32
	ManaCost         uint32
	ManaCostPct      uint32
	RangeIndex       uint32
	InterruptFlags   uint32
	ChannelInterrupt uint32
	DurationIndex    uint32
	Speed            float32
	Effects          [3]SpellEffect
}

type LFGDungeon struct {
	ID             uint32
	MinLevel       uint32
	MaxLevel       uint32
	MapID          int32
	Difficulty     uint32
	Flags          uint32
	TypeID         uint32
	ExpansionLevel uint32
	GroupID        uint32
}

func (d LFGDungeon) Entry() uint32 {
	return d.ID + (d.TypeID << 24)
}

// WorldSafeLoc mirrors WorldSafeLocsEntry (DBCStructure.h): fields ID (u32),
// continent/map (u32), and x/y/z (f32); locale name fields follow and are not
// used by the server.
type WorldSafeLoc struct {
	ID    uint32
	MapID uint32
	X     float32
	Y     float32
	Z     float32
}

// AreaTableEntry mirrors AreaTableEntry (DBCStructure.h): fields ID (u32),
// ContinentID/map (u32), ParentAreaID/zone (u32), AreaBit (u32), and Flags (u32).
type AreaTableEntry struct {
	ID           uint32
	ContinentID  uint32
	ParentAreaID uint32
	AreaBit      uint32
	Flags        uint32
	Name         string
}

// TalentEntry mirrors TrinityCore's TalentEntry (DBCStructure.h:1663).
// Layout: "niiiiiiiixxxxixxixxxxxx" (23 fields).
type TalentEntry struct {
	ID           uint32
	TabID        uint32
	TierID       uint32
	ColumnIndex  uint32
	SpellRank    [5]uint32
	PrereqTalent uint32
	PrereqRank   uint32
}

// MapEntry mirrors TrinityCore's MapEntry (DBCStructure.h:1072, format "nxiixssssssssssssssssxix...").
type MapEntry struct {
	ID           uint32
	InstanceType uint32
	Flags        uint32
	MapName      string
	AreaTableID  uint32
	CorpseMapID  int32
	CorpseX      float32
	CorpseY      float32
	ExpansionID  uint32
	RaidOffset   uint32
	MaxPlayers   uint32
}

func (m MapEntry) IsDungeon() bool {
	return m.InstanceType == 1 || m.InstanceType == 2
}

func (m MapEntry) IsNonRaidDungeon() bool {
	return m.InstanceType == 1
}

func (m MapEntry) IsRaid() bool {
	return m.InstanceType == 2
}

func (m MapEntry) IsBattleground() bool {
	return m.InstanceType == 3
}

func (m MapEntry) IsBattleArena() bool {
	return m.InstanceType == 4
}

const (
	AreaFlagWintergrasp2 uint32 = 0x08000000 // AREA_FLAG_WINTERGRASP_2 (DBCEnums.h:274)
)

func NewStore(dir string) *Store {
	return &Store{Dir: dir, files: make(map[string]*dbc.File)}
}

func (s *Store) File(name string) (*dbc.File, error) {
	s.mu.RLock()
	file := s.files[name]
	s.mu.RUnlock()
	if file != nil {
		return file, nil
	}
	loaded, err := dbc.Open(filepath.Join(s.Dir, name+".dbc"))
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", name, err)
	}
	s.mu.Lock()
	if existing := s.files[name]; existing != nil {
		file = existing
	} else {
		s.files[name] = loaded
		file = loaded
	}
	s.mu.Unlock()
	return file, nil
}

func (s *Store) Race(id uint32) (Race, bool, error) {
	file, err := s.File("ChrRaces")
	if err != nil {
		return Race{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return Race{}, false, nil
	}
	flags, err := record.Uint32(1)
	if err != nil {
		return Race{}, false, err
	}
	factionID, err := record.Uint32(2)
	if err != nil {
		return Race{}, false, err
	}
	maleDisplayID, err := record.Uint32(4)
	if err != nil {
		return Race{}, false, err
	}
	femaleDisplayID, err := record.Uint32(5)
	if err != nil {
		return Race{}, false, err
	}
	cinematic, _ := record.Uint32(12)
	alliance, err := record.Uint32(13)
	if err != nil {
		return Race{}, false, err
	}
	requiredExpansion, err := record.Uint32(68)
	if err != nil {
		return Race{}, false, err
	}
	return Race{ID: id, Flags: flags, FactionID: factionID, MaleDisplayID: maleDisplayID, FemaleDisplayID: femaleDisplayID, CinematicSequence: cinematic, Alliance: alliance, RequiredExpansion: requiredExpansion}, true, nil
}

func (s *Store) Class(id uint32) (Class, bool, error) {
	file, err := s.File("ChrClasses")
	if err != nil {
		return Class{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return Class{}, false, nil
	}
	spellClassSet, err := record.Uint32(56)
	if err != nil {
		return Class{}, false, err
	}
	cinematic, err := record.Uint32(58)
	if err != nil {
		return Class{}, false, err
	}
	requiredExpansion, err := record.Uint32(59)
	if err != nil {
		return Class{}, false, err
	}
	return Class{ID: id, SpellClassSet: spellClassSet, CinematicSequence: cinematic, RequiredExpansion: requiredExpansion}, true, nil
}

func (s *Store) Reputation(id uint32, race, class uint8) (Reputation, bool, error) {
	file, err := s.File("Faction")
	if err != nil {
		return Reputation{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return Reputation{}, false, nil
	}
	list, err := record.Int32(1)
	if err != nil {
		return Reputation{}, false, err
	}
	rep := Reputation{ID: id, ReputationList: list}
	if list < 0 {
		return rep, true, nil
	}
	raceMask := uint32(1) << uint(race-1)
	classMask := uint32(1) << uint(class-1)
	for i := 0; i < 4; i++ {
		raceMaskValue, raceErr := record.Uint32(2 + i)
		classMaskValue, classErr := record.Uint32(6 + i)
		base, baseErr := record.Int32(10 + i)
		flags, flagsErr := record.Uint32(14 + i)
		if raceErr != nil || classErr != nil || baseErr != nil || flagsErr != nil {
			return Reputation{}, false, fmt.Errorf("read faction %d reputation masks: %w", id, firstDBCError(raceErr, classErr, baseErr, flagsErr))
		}
		if (raceMaskValue&raceMask != 0 || (raceMaskValue == 0 && classMaskValue != 0)) && (classMaskValue&classMask != 0 || classMaskValue == 0) {
			rep.BaseStanding = base
			rep.DefaultFlags = uint8(flags)
			break
		}
	}
	return rep, true, nil
}

func (s *Store) FactionTemplate(id uint32) (FactionTemplate, bool, error) {
	file, err := s.File("FactionTemplate")
	if err != nil {
		return FactionTemplate{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return FactionTemplate{}, false, nil
	}
	template := FactionTemplate{ID: id}
	values := []*uint32{&template.Faction, &template.Flags, &template.FactionGroup, &template.FriendGroup, &template.EnemyGroup}
	for field, destination := range values {
		if *destination, err = record.Uint32(field + 1); err != nil {
			return FactionTemplate{}, false, err
		}
	}
	for index := range template.Enemies {
		if template.Enemies[index], err = record.Uint32(6 + index); err != nil {
			return FactionTemplate{}, false, err
		}
		if template.Friends[index], err = record.Uint32(10 + index); err != nil {
			return FactionTemplate{}, false, err
		}
	}
	return template, true, nil
}

func firstDBCError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CharStartOutfit(race, class, gender uint8) ([]uint32, error) {
	file, err := s.File("CharStartOutfit")
	if err != nil {
		return nil, err
	}
	for i := 0; i < int(file.RecordCount); i++ {
		record, err := file.Record(i)
		if err != nil {
			continue
		}
		data := record.Data()
		if len(data) >= 104 && file.RecordSize == 296 {
			// In 3.3.5a CharStartOutfit.dbc (format dbbbXiiii...):
			// Offset 0..3: ID (uint32)
			// Offset 4: Race (uint8)
			// Offset 5: Class (uint8)
			// Offset 6: Gender (uint8)
			// Offset 7: OutfitID (uint8)
			// Offset 8..103: 24 Item IDs (int32)
			r := data[4]
			c := data[5]
			g := data[6]
			if r == race && c == class && g == gender {
				var items []uint32
				for j := 0; j < 24; j++ {
					offset := 8 + j*4
					itemID := int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
					if itemID > 0 {
						items = append(items, uint32(itemID))
					}
				}
				return items, nil
			}
		} else {
			r, _ := record.Uint32(1)
			c, _ := record.Uint32(2)
			g, _ := record.Uint32(3)
			if uint8(r) == race && uint8(c) == class && uint8(g) == gender {
				var items []uint32
				for j := 5; j <= 28; j++ {
					itemID, err := record.Int32(j)
					if err == nil && itemID > 0 {
						items = append(items, uint32(itemID))
					}
				}
				return items, nil
			}
		}
	}
	return nil, nil
}

func (s *Store) Spell(id uint32) (Spell, bool, error) {
	file, err := s.File("Spell")
	if err != nil {
		return Spell{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return Spell{}, false, nil
	}
	spell := Spell{ID: id}
	values := []struct {
		field int
		dest  *uint32
	}{
		{4, &spell.Attributes},
		{225, &spell.SchoolMask}, // Spell.dbc field 225 = SchoolMask (DBCStructure.h:1492)
		{16, &spell.Targets},
		{28, &spell.CastingTimeIndex},
		{29, &spell.RecoveryTime},
		{41, &spell.PowerType},
		{42, &spell.ManaCost},
		{204, &spell.ManaCostPct}, // Spell.dbc field 204 = ManaCostPct (DBCStructure.h:1476)
		{46, &spell.RangeIndex},
		{6, &spell.AttributesEx1},   // Spell.dbc field 6 = AttributesExB (DBCStructure.h:1398)
		{31, &spell.InterruptFlags}, // DBCStructure.h:1421
		{33, &spell.ChannelInterrupt},
		{40, &spell.DurationIndex},
	}
	for _, value := range values {
		if *value.dest, err = record.Uint32(value.field); err != nil {
			return Spell{}, false, err
		}
	}
	if speed, speedErr := record.Float32(47); speedErr == nil {
		spell.Speed = speed
	}
	for i := range spell.Effects {
		effect, err := record.Uint32(71 + i)
		if err != nil {
			return Spell{}, false, err
		}
		basePoints, err := record.Int32(80 + i)
		if err != nil {
			return Spell{}, false, err
		}
		aura, err := record.Uint32(95 + i)
		if err != nil {
			return Spell{}, false, err
		}
		implicitTargetA, err := record.Uint32(86 + i)
		if err != nil {
			return Spell{}, false, err
		}
		implicitTargetB, err := record.Uint32(89 + i)
		if err != nil {
			return Spell{}, false, err
		}
		radiusIndex, err := record.Uint32(92 + i)
		if err != nil {
			return Spell{}, false, err
		}
		miscValue, err := record.Int32(110 + i)
		if err != nil {
			return Spell{}, false, err
		}
		auraPeriod, err := record.Uint32(98 + i) // EffectAuraPeriod, DBCStructure.h:1492 area (98-100)
		if err != nil {
			return Spell{}, false, err
		}
		triggerSpell, err := record.Uint32(116 + i)
		if err != nil {
			return Spell{}, false, err
		}
		spell.Effects[i] = SpellEffect{Effect: effect, BasePoints: basePoints, Aura: aura, AuraPeriod: auraPeriod, ImplicitTargetA: implicitTargetA, ImplicitTargetB: implicitTargetB, RadiusIndex: radiusIndex, MiscValue: miscValue, TriggerSpell: triggerSpell}
	}
	return spell, true, nil
}

func (s *Store) SpellCastTime(id uint32) (int32, bool, error) {
	if id == 0 {
		return 0, true, nil
	}
	file, err := s.File("SpellCastTimes")
	if err != nil {
		return 0, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return 0, false, nil
	}
	base, err := record.Int32(1)
	if err != nil {
		return 0, false, err
	}
	return base, true, nil
}

func (s *Store) parseLFGDungeon(id uint32, record dbc.Record) (LFGDungeon, bool, error) {
	result := LFGDungeon{ID: id}
	values := []struct {
		field int
		dest  *uint32
	}{
		{18, &result.MinLevel},
		{19, &result.MaxLevel},
		{24, &result.Difficulty},
		{25, &result.Flags},
		{26, &result.TypeID},
		{29, &result.ExpansionLevel},
		{31, &result.GroupID},
	}
	var err error
	for _, value := range values {
		if *value.dest, err = record.Uint32(value.field); err != nil {
			return LFGDungeon{}, false, err
		}
	}
	mapID, err := record.Int32(23)
	if err != nil {
		return LFGDungeon{}, false, err
	}
	result.MapID = mapID
	return result, true, nil
}

func (s *Store) LFGDungeon(id uint32) (LFGDungeon, bool, error) {
	file, err := s.File("LFGDungeons")
	if err != nil {
		return LFGDungeon{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return LFGDungeon{}, false, nil
	}
	return s.parseLFGDungeon(id, record)
}

func (s *Store) LFGDungeons() ([]LFGDungeon, error) {
	file, err := s.File("LFGDungeons")
	if err != nil {
		return nil, err
	}
	dungeons := make([]LFGDungeon, 0, file.Records())
	for i := 0; i < file.Records(); i++ {
		record, err := file.Record(i)
		if err != nil {
			continue
		}
		id := record.Uint32Unchecked(0)
		dungeon, ok, err := s.parseLFGDungeon(id, record)
		if err == nil && ok {
			dungeons = append(dungeons, dungeon)
		}
	}
	return dungeons, nil
}

func IsSupportedLFGType(typeID uint32) bool {
	return typeID == 1 || typeID == 2 || typeID == 5 || typeID == 6
}

func IsPlayableRace(race Race) bool {
	return race.Alliance != 2 && race.Flags&1 == 0
}

func HasMountedFlightSpeed(spell Spell, speed int32) bool {
	for _, effect := range spell.Effects {
		if effect.Aura == MountedFlightSpeedAura && effect.BasePoints+1 == speed {
			return true
		}
	}
	return false
}

// WorldSafeLoc loads one WorldSafeLocs.dbc record by id. The record layout is
// "nifff" (DBCfmt.h WorldSafeLocsEntryfmt): id, map, x, y, z, then locale
// string references that the server does not use.
func (s *Store) WorldSafeLoc(id uint32) (WorldSafeLoc, bool, error) {
	file, err := s.File("WorldSafeLocs")
	if err != nil {
		return WorldSafeLoc{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return WorldSafeLoc{}, false, nil
	}
	mapID, err := record.Uint32(1)
	if err != nil {
		return WorldSafeLoc{}, false, err
	}
	x, err := record.Float32(2)
	if err != nil {
		return WorldSafeLoc{}, false, err
	}
	y, err := record.Float32(3)
	if err != nil {
		return WorldSafeLoc{}, false, err
	}
	z, err := record.Float32(4)
	if err != nil {
		return WorldSafeLoc{}, false, err
	}
	return WorldSafeLoc{ID: id, MapID: mapID, X: x, Y: y, Z: z}, true, nil
}

// Area loads one AreaTable.dbc record by id. Record layout is "niiiixxxxxissssssssssssssssxiiiiixxx".
func (s *Store) Area(id uint32) (AreaTableEntry, bool, error) {
	file, err := s.File("AreaTable")
	if err != nil {
		return AreaTableEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return AreaTableEntry{}, false, nil
	}
	continentID, err := record.Uint32(1)
	if err != nil {
		return AreaTableEntry{}, false, err
	}
	parentAreaID, err := record.Uint32(2)
	if err != nil {
		return AreaTableEntry{}, false, err
	}
	areaBit, err := record.Uint32(3)
	if err != nil {
		return AreaTableEntry{}, false, err
	}
	flags, err := record.Uint32(4)
	if err != nil {
		return AreaTableEntry{}, false, err
	}
	name, _ := record.String(11)
	return AreaTableEntry{
		ID:           id,
		ContinentID:  continentID,
		ParentAreaID: parentAreaID,
		AreaBit:      areaBit,
		Flags:        flags,
		Name:         name,
	}, true, nil
}

// Talent loads a talent record by ID from Talent.dbc.
func (s *Store) Talent(id uint32) (TalentEntry, bool, error) {
	file, err := s.File("Talent")
	if err != nil {
		return TalentEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return TalentEntry{}, false, nil
	}
	tabID, err := record.Uint32(1)
	if err != nil {
		return TalentEntry{}, false, err
	}
	tierID, err := record.Uint32(2)
	if err != nil {
		return TalentEntry{}, false, err
	}
	col, err := record.Uint32(3)
	if err != nil {
		return TalentEntry{}, false, err
	}
	var ranks [5]uint32
	for i := 0; i < 5; i++ {
		ranks[i], _ = record.Uint32(4 + i)
	}
	prereqTalent, _ := record.Uint32(13)
	prereqRank, _ := record.Uint32(16)
	return TalentEntry{
		ID:           id,
		TabID:        tabID,
		TierID:       tierID,
		ColumnIndex:  col,
		SpellRank:    ranks,
		PrereqTalent: prereqTalent,
		PrereqRank:   prereqRank,
	}, true, nil
}

// TalentBySpell scans Talent.dbc for a talent record that grants the given spell.
func (s *Store) TalentBySpell(spellID uint32) (uint32, uint8, bool) {
	if spellID == 0 {
		return 0, 0, false
	}
	file, err := s.File("Talent")
	if err != nil {
		return 0, 0, false
	}
	for i := 0; i < file.Records(); i++ {
		rec, err := file.Record(i)
		if err != nil {
			continue
		}
		for r := 0; r < 5; r++ {
			sp, _ := rec.Uint32(4 + r)
			if sp == spellID {
				id, _ := rec.Uint32(0)
				return id, uint8(r), true
			}
		}
	}
	return 0, 0, false
}

// SpellDuration mirrors SpellInfo::GetDuration from SpellDuration.dbc
// (DBCStructure.h:1524, format "niii"): Duration + DurationPerLevel * (level-1)
// clamped to MaxDuration when positive. A negative base duration (-1) means
// infinite and is returned as-is.
func (s *Store) SpellDuration(id uint32, level uint32) (int32, bool, error) {
	if id == 0 {
		return 0, true, nil
	}
	file, err := s.File("SpellDuration")
	if err != nil {
		return 0, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return 0, false, nil
	}
	base, err := record.Int32(1)
	if err != nil {
		return 0, false, err
	}
	if base < 0 {
		return base, true, nil // -1 infinite
	}
	if level == 0 {
		level = 1
	}
	perLevel, err := record.Int32(2)
	if err != nil {
		return 0, false, err
	}
	maxDuration, err := record.Int32(3)
	if err != nil {
		return 0, false, err
	}
	duration := base + perLevel*int32(level-1)
	if maxDuration > 0 && duration > maxDuration {
		duration = maxDuration
	}
	return duration, true, nil
}

func (s *Store) Map(id uint32) (MapEntry, bool, error) {
	file, err := s.File("Map")
	if err != nil {
		return MapEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return MapEntry{}, false, nil
	}
	instType, err := record.Uint32(2)
	if err != nil {
		return MapEntry{}, false, err
	}
	flags, _ := record.Uint32(3)
	name, _ := record.String(5)
	areaTableID, _ := record.Uint32(22)
	corpseMapID, _ := record.Int32(59)
	corpseX, _ := record.Float32(60)
	corpseY, _ := record.Float32(61)
	expansionID, _ := record.Uint32(63)
	raidOffset, _ := record.Uint32(64)
	maxPlayers, _ := record.Uint32(65)

	return MapEntry{
		ID:           id,
		InstanceType: instType,
		Flags:        flags,
		MapName:      name,
		AreaTableID:  areaTableID,
		CorpseMapID:  corpseMapID,
		CorpseX:      corpseX,
		CorpseY:      corpseY,
		ExpansionID:  expansionID,
		RaidOffset:   raidOffset,
		MaxPlayers:   maxPlayers,
	}, true, nil
}
