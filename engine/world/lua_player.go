package world

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func (s *Server) luaPlayers() []*scripting.Object {
	s.sessionsMu.RLock()
	players := make([]*scripting.Object, 0, len(s.sessions))
	for value := range s.sessions {
		if value.playerLoaded {
			if player := value.luaPlayer(); player != nil {
				players = append(players, player)
			}
		}
	}
	s.sessionsMu.RUnlock()
	return players
}

func (s *session) triggerPlayerEvent(ctx context.Context, event int, args ...any) {
	if s.server == nil || s.server.Features == nil || s.server.Features.Scripts == nil {
		return
	}
	if _, err := s.server.Features.Scripts.TriggerPlayerEvent(ctx, event, append([]any{event}, args...)...); err != nil {
		s.debug("lua player event failed", "account", s.accountName, "event", event, "error", err)
	}
}

func (s *Server) findPlayer(guid uint64) *scripting.Object {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for value := range s.sessions {
		if value.playerLoaded && value.playerGUID == guid {
			return value.luaPlayer()
		}
	}
	return nil
}

func (s *session) luaPlayer() *scripting.Object {
	if !s.playerLoaded || s.player == nil {
		return nil
	}
	state := s.player
	fields := map[string]any{"Name": state.Name, "GUID": state.GUID, "GUIDLow": uint32(state.GUID), "MapId": state.Map, "Level": state.Level, "Race": state.Race, "Class": state.Class, "Gender": state.Gender}
	methods := map[string]scripting.ObjectMethod{}
	methods["GetName"] = luaNoArgs(func() any { return state.Name })
	methods["GetGUID"] = luaNoArgs(func() any { return state.GUID })
	methods["GetGUIDLow"] = luaNoArgs(func() any { return uint32(state.GUID) })
	methods["GetMapId"] = luaNoArgs(func() any { return state.Map })
	methods["GetInstanceId"] = luaNoArgs(func() any { return uint32(0) })
	methods["GetObjectType"] = luaNoArgs(func() any { return "Player" })
	methods["GetZ"] = luaNoArgs(func() any { return state.Z })
	methods["GetLevel"] = luaNoArgs(func() any { return state.Level })
	methods["GetRace"] = luaNoArgs(func() any { return state.Race })
	methods["GetClass"] = luaNoArgs(func() any { return state.Class })
	methods["GetGender"] = luaNoArgs(func() any { return state.Gender })
	methods["GetDbcLocale"] = luaNoArgs(func() any { return uint32(0) })
	methods["GetPowerType"] = luaNoArgs(func() any { return uint32(0) })
	methods["GetCoinage"] = luaNoArgs(func() any { return state.Money })
	methods["GetMaxHealth"] = luaNoArgs(func() any { return state.MaxHealth })
	methods["GetGMRank"] = luaNoArgs(func() any { return s.security })
	methods["IsGM"] = luaNoArgs(func() any { return s.security > 0 })
	methods["IsInCombat"] = luaNoArgs(func() any { return false })
	methods["IsAlive"] = luaNoArgs(func() any { return state.Health > 0 })
	methods["HasAura"] = func(_ context.Context, args []any) ([]any, error) {
		spell, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		_, ok := s.auras[spell]
		return []any{ok}, nil
	}
	methods["AddAura"] = func(_ context.Context, args []any) ([]any, error) {
		spell, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		s.applyAura(spell)
		return nil, nil
	}
	methods["RemoveAura"] = func(_ context.Context, args []any) ([]any, error) {
		spell, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		s.removeAura(spell)
		return nil, nil
	}
	methods["SetCoinage"] = func(_ context.Context, args []any) ([]any, error) {
		money, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		state.Money = money
		_, err = s.server.CharactersStore.DB.Exec("UPDATE characters SET money = ? WHERE guid = ?", money, state.GUID)
		return nil, err
	}
	methods["ModifyMoney"] = func(_ context.Context, args []any) ([]any, error) {
		delta, err := luaInt64Arg(args, 0)
		if err != nil {
			return nil, err
		}
		money := int64(state.Money) + delta
		if money < 0 {
			money = 0
		}
		if money > math.MaxUint32 {
			money = math.MaxUint32
		}
		state.Money = uint32(money)
		_, err = s.server.CharactersStore.DB.Exec("UPDATE characters SET money = ? WHERE guid = ?", state.Money, state.GUID)
		return []any{true}, err
	}
	methods["SetGender"] = func(_ context.Context, args []any) ([]any, error) {
		gender, err := luaUint32Arg(args, 0)
		if err != nil || gender > 1 {
			if err == nil {
				err = fmt.Errorf("gender must be 0 or 1")
			}
			return nil, err
		}
		state.Gender = uint8(gender)
		_, err = s.server.CharactersStore.DB.Exec("UPDATE characters SET gender = ? WHERE guid = ?", state.Gender, state.GUID)
		return nil, err
	}
	methods["SetHealth"] = func(_ context.Context, args []any) ([]any, error) {
		health, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		if health > state.MaxHealth {
			health = state.MaxHealth
		}
		state.Health = health
		_, err = s.server.CharactersStore.DB.Exec("UPDATE characters SET health = ? WHERE guid = ?", health, state.GUID)
		return nil, err
	}
	methods["LearnSpell"] = func(_ context.Context, args []any) ([]any, error) {
		spell, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		query := "INSERT OR IGNORE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)"
		if s.server.CharactersStore.Backend != "sqlite" {
			query = "INSERT IGNORE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)"
		}
		_, err = s.server.CharactersStore.DB.Exec(query, state.GUID, spell)
		return nil, err
	}
	methods["RemoveSpell"] = func(_ context.Context, args []any) ([]any, error) {
		spell, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		_, err = s.server.CharactersStore.DB.Exec("DELETE FROM character_spell WHERE guid = ? AND spell = ?", state.GUID, spell)
		return nil, err
	}
	methods["HasQuest"] = func(_ context.Context, args []any) ([]any, error) {
		quest, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		var count int64
		err = s.server.CharactersStore.DB.QueryRow("SELECT COUNT(1) FROM character_queststatus WHERE guid = ? AND quest = ?", state.GUID, quest).Scan(&count)
		if isMissingTableError(err) {
			return []any{false}, nil
		}
		return []any{err == nil && count > 0}, err
	}
	methods["CompleteQuest"] = func(_ context.Context, args []any) ([]any, error) {
		quest, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		_, err = s.server.CharactersStore.DB.Exec("UPDATE character_queststatus SET status = 1 WHERE guid = ? AND quest = ?", state.GUID, quest)
		if isMissingTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	methods["SendBroadcastMessage"] = s.luaMessageMethod()
	methods["SendAreaTriggerMessage"] = s.luaMessageMethod()
	methods["SendNotification"] = s.luaMessageMethod()
	methods["Teleport"] = func(_ context.Context, args []any) ([]any, error) {
		if len(args) < 5 {
			return nil, fmt.Errorf("Teleport requires map and position")
		}
		mapID, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		position := make([]float32, 4)
		for index := range position {
			value, valueErr := luaFloat32Arg(args, index+1)
			if valueErr != nil {
				return nil, valueErr
			}
			position[index] = value
		}
		state.Map, state.X, state.Y, state.Z, state.Orientation = mapID, position[0], position[1], position[2], position[3]
		packet := protocol.NewBuffer(20)
		packet.WriteU32(state.Map)
		packet.WriteF32(state.X)
		packet.WriteF32(state.Y)
		packet.WriteF32(state.Z)
		packet.WriteF32(state.Orientation)
		return nil, s.write(uint16(protocol.OpcodeSMSG_NEW_WORLD), packet.Bytes(), true)
	}
	methods["Mute"] = func(_ context.Context, args []any) ([]any, error) {
		seconds, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		_, err = s.server.AuthStore.DB.Exec("UPDATE account SET mutetime = ? WHERE id = ?", seconds, s.accountID)
		return nil, err
	}
	methods["GetSelection"] = func(ctx context.Context, _ []any) ([]any, error) {
		if s.selection == 0 {
			return []any{nil}, nil
		}
		if player := s.server.findPlayer(s.selection); player != nil {
			return []any{player}, nil
		}
		if creature := s.luaCreature(ctx, s.selection); creature != nil {
			return []any{creature}, nil
		}
		if object := s.luaGameObject(ctx, s.selection); object != nil {
			return []any{object}, nil
		}
		return []any{nil}, nil
	}
	methods["SetScale"] = func(_ context.Context, args []any) ([]any, error) {
		value, err := luaFloat32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		s.scale = value
		return nil, nil
	}
	methods["EmoteState"] = func(_ context.Context, args []any) ([]any, error) {
		value, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		s.emoteState = value
		return nil, nil
	}
	methods["SetPlayerLock"] = func(_ context.Context, args []any) ([]any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("SetPlayerLock requires a boolean")
		}
		value, ok := args[0].(bool)
		if !ok {
			return nil, fmt.Errorf("SetPlayerLock requires a boolean")
		}
		s.playerLocked = value
		return nil, nil
	}
	methods["GossipComplete"] = s.luaGossipComplete
	methods["GossipClearMenu"] = s.luaGossipClearMenu
	methods["GossipMenuAddItem"] = s.luaGossipMenuAddItem
	methods["GossipSendMenu"] = s.luaGossipSendMenu
	methods["SetBinding"] = func(_ context.Context, _ []any) ([]any, error) { return nil, nil }
	methods["SetNotRefundable"] = func(_ context.Context, _ []any) ([]any, error) { return nil, nil }
	methods["PlayDirectSound"] = func(_ context.Context, _ []any) ([]any, error) { return nil, nil }
	methods["GetItemByPos"] = func(_ context.Context, _ []any) ([]any, error) { return []any{nil}, nil }
	methods["GetNearestGameObject"] = func(ctx context.Context, args []any) ([]any, error) {
		if len(args) == 0 {
			return []any{nil}, nil
		}
		rangeValue, err := luaFloat32Arg(args, 0)
		if err != nil || rangeValue < 0 {
			return []any{nil}, err
		}
		object := s.nearestGameObject(ctx, state.Map, state.X, state.Y, rangeValue)
		return []any{object}, nil
	}
	return &scripting.Object{Type: "Player", Fields: fields, Methods: methods}
}

func (s *session) luaMessageMethod() scripting.ObjectMethod {
	return func(_ context.Context, args []any) ([]any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("message is required")
		}
		message, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("message must be a string")
		}
		return nil, s.write(uint16(protocol.OpcodeSMSG_MESSAGECHAT), protocol.BuildSystemChatMessage(message), true)
	}
}

func luaNoArgs(value func() any) scripting.ObjectMethod {
	return func(_ context.Context, _ []any) ([]any, error) { return []any{value()}, nil }
}

func luaUint32Arg(args []any, index int) (uint32, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("argument %d is required", index+1)
	}
	value, err := luaUint64(args[index])
	if err != nil || value > math.MaxUint32 {
		return 0, fmt.Errorf("argument %d must be uint32", index+1)
	}
	return uint32(value), nil
}

func luaUint64(value any) (uint64, error) {
	switch value := value.(type) {
	case uint8:
		return uint64(value), nil
	case uint16:
		return uint64(value), nil
	case uint32:
		return uint64(value), nil
	case uint64:
		return value, nil
	case int:
		if value >= 0 {
			return uint64(value), nil
		}
	case int64:
		if value >= 0 {
			return uint64(value), nil
		}
	case float64:
		if value >= 0 && value <= math.MaxUint64 && value == math.Trunc(value) {
			return uint64(value), nil
		}
	case float32:
		if value >= 0 && value == float32(math.Trunc(float64(value))) {
			return uint64(value), nil
		}
	case string:
		return strconv.ParseUint(value, 10, 64)
	}
	return 0, fmt.Errorf("value is not unsigned")
}

func luaInt64Arg(args []any, index int) (int64, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("argument %d is required", index+1)
	}
	switch value := args[index].(type) {
	case int:
		return int64(value), nil
	case int64:
		return value, nil
	case float64:
		if value >= math.MinInt64 && value <= math.MaxInt64 && value == math.Trunc(value) {
			return int64(value), nil
		}
	case string:
		return strconv.ParseInt(value, 10, 64)
	}
	return 0, fmt.Errorf("argument %d must be int64", index+1)
}

func luaFloat32Arg(args []any, index int) (float32, error) {
	if index >= len(args) {
		return 0, fmt.Errorf("argument %d is required", index+1)
	}
	var value float64
	switch current := args[index].(type) {
	case float64:
		value = current
	case float32:
		value = float64(current)
	case int:
		value = float64(current)
	case int64:
		value = float64(current)
	default:
		return 0, fmt.Errorf("argument %d must be number", index+1)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > maxPositionCoordinate {
		return 0, fmt.Errorf("argument %d is outside position limits", index+1)
	}
	return float32(value), nil
}

func isMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "no such table") || strings.Contains(value, "doesn't exist") || strings.Contains(value, "unknown table")
}
