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
	Effect     uint32
	BasePoints int32
	Aura       uint32
}

type Spell struct {
	ID      uint32
	Effects [3]SpellEffect
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
		spell.Effects[i] = SpellEffect{Effect: effect, BasePoints: basePoints, Aura: aura}
	}
	return spell, true, nil
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
