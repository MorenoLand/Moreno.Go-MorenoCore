package wotlk

import (
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
	Alliance          uint32
	RequiredExpansion uint32
}

type Class struct {
	ID                uint32
	SpellClassSet     uint32
	CinematicSequence uint32
	RequiredExpansion uint32
}

type SpellEffect struct {
	Effect          uint32
	BasePoints      int32
	Aura            uint32
	ImplicitTargetA uint32
	ImplicitTargetB uint32
	RadiusIndex     uint32
	MiscValue       int32
	TriggerSpell    uint32
}

type Spell struct {
	ID               uint32
	Attributes       uint32
	Targets          uint32
	CastingTimeIndex uint32
	RecoveryTime     uint32
	PowerType        uint32
	ManaCost         uint32
	RangeIndex       uint32
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
	alliance, err := record.Uint32(13)
	if err != nil {
		return Race{}, false, err
	}
	requiredExpansion, err := record.Uint32(68)
	if err != nil {
		return Race{}, false, err
	}
	return Race{ID: id, Flags: flags, FactionID: factionID, MaleDisplayID: maleDisplayID, FemaleDisplayID: femaleDisplayID, Alliance: alliance, RequiredExpansion: requiredExpansion}, true, nil
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
		{16, &spell.Targets},
		{28, &spell.CastingTimeIndex},
		{29, &spell.RecoveryTime},
		{41, &spell.PowerType},
		{42, &spell.ManaCost},
		{46, &spell.RangeIndex},
	}
	for _, value := range values {
		if *value.dest, err = record.Uint32(value.field); err != nil {
			return Spell{}, false, err
		}
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
		triggerSpell, err := record.Uint32(116 + i)
		if err != nil {
			return Spell{}, false, err
		}
		spell.Effects[i] = SpellEffect{Effect: effect, BasePoints: basePoints, Aura: aura, ImplicitTargetA: implicitTargetA, ImplicitTargetB: implicitTargetB, RadiusIndex: radiusIndex, MiscValue: miscValue, TriggerSpell: triggerSpell}
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

func (s *Store) LFGDungeon(id uint32) (LFGDungeon, bool, error) {
	file, err := s.File("LFGDungeons")
	if err != nil {
		return LFGDungeon{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return LFGDungeon{}, false, nil
	}
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
