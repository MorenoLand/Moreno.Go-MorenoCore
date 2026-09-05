package world

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type learnedSpell struct {
	ID       uint32
	Active   bool
	Disabled bool
}

type spellCooldown struct {
	Spell       uint32
	Item        uint16
	Category    uint16
	End         int64
	CategoryEnd int64
}

func (s *session) loadPlayerPacketsState(ctx context.Context, state *playerState) error {
	spells, err := s.loadLearnedSpells(ctx, state.GUID, state.Race, state.Class)
	if err != nil {
		return err
	}
	actions, err := s.loadActionButtons(ctx, state.GUID, state.Race, state.Class)
	if err != nil {
		return err
	}
	cooldowns, err := s.loadSpellCooldowns(ctx, state.GUID)
	if err != nil {
		return err
	}
	state.Spells, state.Actions, state.Cooldowns = spells, actions, cooldowns
	return nil
}

func (s *session) loadLearnedSpells(ctx context.Context, guid uint64, race, class uint8) ([]learnedSpell, error) {
	defaults := defaultRacialSpells(race)
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return defaults, nil
	}
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT spell, active, disabled FROM character_spell WHERE guid = ? ORDER BY spell", guid)
	if err != nil {
		return defaults, nil
	}
	result := make([]learnedSpell, 0)
	for rows.Next() {
		var spell, active, disabled int64
		if err := rows.Scan(&spell, &active, &disabled); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result = append(result, learnedSpell{ID: uint32(spell), Active: active != 0, Disabled: disabled != 0})
	}
	_ = rows.Close()

	for _, def := range defaults {
		found := false
		for _, sp := range result {
			if sp.ID == def.ID {
				found = true
				break
			}
		}
		if !found {
			result = append(result, def)
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", guid, def.ID)
		}
	}

	// If player has few spells, ensure custom starter spells from playercreateinfo_spell_custom are also learned (if PlayerStart.AllSpells enabled)
	if s.server != nil && s.server.WorldStore != nil && s.server.WorldStore.DB != nil && s.server.Config.PlayerStartAllSpells {
		raceMask, classMask := playerCreateMask(race), playerCreateMask(class)
		crows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT Spell FROM playercreateinfo_spell_custom WHERE (racemask = 0 OR (racemask & ?) <> 0) AND (classmask = 0 OR (classmask & ?) <> 0)", raceMask, classMask)
		if err == nil {
			defer crows.Close()
			for crows.Next() {
				var customSpell int64
				if err := crows.Scan(&customSpell); err == nil && customSpell > 0 {
					id := uint32(customSpell)
					found := false
					for _, sp := range result {
						if sp.ID == id {
							found = true
							break
						}
					}
					if !found {
						result = append(result, learnedSpell{ID: id, Active: true})
						_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", guid, id)
					}
				}
			}
		}
	}
	return result, nil
}

func playerCreateMask(id uint8) uint32 {
	if id == 0 || id > 32 {
		return 0
	}
	return uint32(1) << (id - 1)
}

func defaultRacialSpells(race uint8) []learnedSpell {
	spells := make([]learnedSpell, 0, 4)
	spells = append(spells, learnedSpell{ID: 6603, Active: true})
	switch race {
	case 1:
		spells = append(spells, learnedSpell{ID: 668, Active: true})
	case 2:
		spells = append(spells, learnedSpell{ID: 669, Active: true})
	case 3:
		spells = append(spells, learnedSpell{ID: 668, Active: true}, learnedSpell{ID: 672, Active: true})
	case 4:
		spells = append(spells, learnedSpell{ID: 668, Active: true}, learnedSpell{ID: 671, Active: true})
	case 5:
		spells = append(spells, learnedSpell{ID: 669, Active: true}, learnedSpell{ID: 17737, Active: true})
	case 6:
		spells = append(spells, learnedSpell{ID: 669, Active: true}, learnedSpell{ID: 670, Active: true})
	case 7:
		spells = append(spells, learnedSpell{ID: 668, Active: true}, learnedSpell{ID: 7340, Active: true})
	case 8:
		spells = append(spells, learnedSpell{ID: 669, Active: true}, learnedSpell{ID: 7341, Active: true})
	case 10:
		spells = append(spells, learnedSpell{ID: 669, Active: true}, learnedSpell{ID: 813, Active: true})
	case 11:
		spells = append(spells, learnedSpell{ID: 668, Active: true}, learnedSpell{ID: 29932, Active: true})
	default:
		spells = append(spells, learnedSpell{ID: 668, Active: true})
	}
	return spells
}

func isLanguageSpell(spellID uint32) bool {
	switch spellID {
	case 668, 669, 670, 671, 672, 813, 7340, 7341, 17737, 29932:
		return true
	}
	return false
}

func (s *session) loadActionButtons(ctx context.Context, guid uint64, race, class uint8) ([144]uint32, error) {
	var result [144]uint32
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT button, action, type FROM character_action WHERE guid = ? AND spec = (SELECT activeTalentGroup FROM characters WHERE guid = ?) ORDER BY button", guid, guid)
	if err != nil {
		if missingTable(err) || isMissingColumn(err) {
			return result, nil
		}
		return result, err
	}
	defer rows.Close()
	hasActions := false
	for rows.Next() {
		var button, action, kind int64
		if err := rows.Scan(&button, &action, &kind); err != nil {
			return result, err
		}
		if button >= 0 && button < int64(len(result)) && action >= 0 && action < 0x01000000 && kind >= 0 && kind <= 255 {
			result[button] = uint32(action) | uint32(kind)<<24
			hasActions = true
		}
	}
	if !hasActions && s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		arows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT button, action, type FROM playercreateinfo_action WHERE race = ? AND class = ?", race, class)
		if err == nil {
			defer arows.Close()
			for arows.Next() {
				var button, action, kind int64
				if err := arows.Scan(&button, &action, &kind); err == nil {
					if button >= 0 && button < int64(len(result)) && action >= 0 && action < 0x01000000 && kind >= 0 && kind <= 255 {
						result[button] = uint32(action) | uint32(kind)<<24
						_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_action (guid, spec, button, action, type) VALUES (?, 0, ?, ?, ?)", guid, button, action, kind)
					}
				}
			}
		}
	}
	return result, rows.Err()
}

func (s *session) loadSpellCooldowns(ctx context.Context, guid uint64) ([]spellCooldown, error) {
	rows, err := s.server.CharactersStore.DB.QueryContext(ctx, "SELECT spell, item, categoryId, time, categoryEnd FROM character_spell_cooldown WHERE guid = ? AND time > ? ORDER BY spell", guid, time.Now().Unix())
	if err != nil {
		if missingTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	result := make([]spellCooldown, 0)
	for rows.Next() {
		var spell, item, category, end, categoryEnd int64
		if err := rows.Scan(&spell, &item, &category, &end, &categoryEnd); err != nil {
			return nil, err
		}
		result = append(result, spellCooldown{Spell: uint32(spell), Item: uint16(item), Category: uint16(category), End: end, CategoryEnd: categoryEnd})
	}
	return result, rows.Err()
}

func buildInitialSpells(state playerState) []byte {
	spells := append([]learnedSpell(nil), state.Spells...)
	sort.Slice(spells, func(i, j int) bool { return spells[i].ID < spells[j].ID })
	packet := protocol.NewBuffer(8 + len(spells)*6 + len(state.Cooldowns)*16)
	packet.WriteU8(0)
	count := 0
	for _, spell := range spells {
		if spell.Active && !spell.Disabled {
			count++
		}
	}
	packet.WriteU16(uint16(count))
	for _, spell := range spells {
		if spell.Active && !spell.Disabled {
			packet.WriteU32(spell.ID)
			packet.WriteU16(0)
		}
	}
	now := time.Now().Unix()
	cooldowns := append([]spellCooldown(nil), state.Cooldowns...)
	sort.Slice(cooldowns, func(i, j int) bool { return cooldowns[i].Spell < cooldowns[j].Spell })
	packet.WriteU16(uint16(len(cooldowns)))
	for _, cooldown := range cooldowns {
		packet.WriteU32(cooldown.Spell)
		packet.WriteU16(cooldown.Item)
		packet.WriteU16(cooldown.Category)
		cooldownTime := remainingMilliseconds(cooldown.End, now)
		categoryTime := remainingMilliseconds(cooldown.CategoryEnd, now)
		if cooldownTime == 0 {
			packet.WriteU32(0)
			packet.WriteU32(0)
		} else if categoryTime > 0 {
			packet.WriteU32(0)
			packet.WriteU32(categoryTime)
		} else {
			packet.WriteU32(cooldownTime)
			packet.WriteU32(0)
		}
	}
	return packet.Bytes()
}

func buildUnlearnSpells() []byte {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(0)
	return packet.Bytes()
}

func buildActionButtons(actions [144]uint32) []byte {
	packet := protocol.NewBuffer(1 + len(actions)*4)
	packet.WriteU8(1)
	for _, action := range actions {
		packet.WriteU32(action)
	}
	return packet.Bytes()
}

func remainingMilliseconds(end, now int64) uint32 {
	if end <= now {
		return 0
	}
	remaining := (end - now) * 1000
	if remaining > int64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(remaining)
}

func missingTable(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no such table")
}
