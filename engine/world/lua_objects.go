package world

import (
	"context"
	"database/sql"
	"math"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
)

type luaCreatureState struct {
	GUID         uint64
	Entry        uint32
	Name         string
	DisplayID    uint32
	Health       uint32
	MaxHealth    uint32
	GossipMenuID uint32
	NPCFlags     uint32
	Map          uint32
	X            float32
	Y            float32
	Z            float32
}

type luaGameObjectState struct {
	GUID      uint64
	LowGUID   uint32
	Entry     uint32
	DisplayID uint32
	Name      string
	Map       uint32
	X         float32
	Y         float32
	Z         float32
}

func (s *session) luaCreature(ctx context.Context, guid uint64) *scripting.Object {
	if uint16(guid>>48) != 0xF130 {
		return nil
	}
	low := uint32(guid & 0x00FFFFFF)
	entry := uint32((guid >> 24) & 0x00FFFFFF)
	var state luaCreatureState
	var displayID, health, maxLevel, gossipMenuID, npcFlags int64
	npcFlagArgs := []any{low, entry}
	npcFlagExpr := "t.npcflag"
	if flagClause, ok := s.server.gameEventNPCFlagClause(ctx, &npcFlagArgs); ok {
		npcFlagExpr = "(t.npcflag | " + flagClause + ")"
	}
	err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT c.guid, c.id, t.name, COALESCE(NULLIF(c.modelid, 0), t.modelid1), c.curhealth, t.maxlevel, t.gossip_menu_id, "+npcFlagExpr+", c.map, c.position_x, c.position_y, c.position_z FROM creature AS c JOIN creature_template AS t ON t.entry = c.id WHERE c.guid = ? AND c.id = ?", npcFlagArgs...).Scan(&state.GUID, &state.Entry, &state.Name, &displayID, &health, &maxLevel, &gossipMenuID, &npcFlags, &state.Map, &state.X, &state.Y, &state.Z)
	if err != nil && isMissingColumn(err) {
		err = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT c.guid, c.id, t.name, COALESCE(NULLIF(c.modelid, 0), t.modelid1), c.curhealth, t.maxlevel, t.gossip_menu_id, t.npcflag FROM creature AS c JOIN creature_template AS t ON t.entry = c.id WHERE c.guid = ? AND c.id = ?", low, entry).Scan(&state.GUID, &state.Entry, &state.Name, &displayID, &health, &maxLevel, &gossipMenuID, &npcFlags)
	}
	if err != nil {
		if !errorsIsNoRows(err) {
			s.debug("lua creature lookup failed", "account", s.accountName, "guid", guid, "error", err)
		}
		return nil
	}
	state.GUID = guid
	state.DisplayID, state.Health = uint32(displayID), uint32(maxUint64(health, 1))
	state.MaxHealth = state.Health
	if maxLevel > 0 && state.MaxHealth < uint32(maxLevel) {
		state.MaxHealth = uint32(maxLevel)
	}
	state.GossipMenuID, state.NPCFlags = uint32(gossipMenuID), uint32(npcFlags)
	methods := map[string]scripting.ObjectMethod{}
	methods["GetName"] = luaNoArgs(func() any { return state.Name })
	methods["GetEntry"] = luaNoArgs(func() any { return state.Entry })
	methods["GetDisplayId"] = luaNoArgs(func() any { return state.DisplayID })
	methods["GetGUID"] = luaNoArgs(func() any { return state.GUID })
	methods["GetGUIDLow"] = luaNoArgs(func() any { return uint32(state.GUID & 0x00FFFFFF) })
	methods["GetObjectType"] = luaNoArgs(func() any { return "Creature" })
	methods["GetMaxHealth"] = luaNoArgs(func() any { return state.MaxHealth })
	methods["IsInCombat"] = luaNoArgs(func() any { return false })
	methods["IsAlive"] = luaNoArgs(func() any { return state.Health > 0 })
	methods["AddAura"] = func(_ context.Context, args []any) ([]any, error) {
		spell, err := luaUint32Arg(args, 0)
		if err != nil {
			return nil, err
		}
		s.server.objectsMu.Lock()
		if s.server.creatureAuras == nil {
			s.server.creatureAuras = make(map[uint64]map[uint32]struct{})
		}
		if s.server.creatureAuras[state.GUID] == nil {
			s.server.creatureAuras[state.GUID] = make(map[uint32]struct{})
		}
		s.server.creatureAuras[state.GUID][spell] = struct{}{}
		s.server.objectsMu.Unlock()
		return nil, nil
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
		_, err = s.server.WorldStore.DB.ExecContext(context.Background(), "UPDATE creature SET curhealth = ? WHERE guid = ?", health, uint32(state.GUID&0x00FFFFFF))
		return nil, err
	}
	methods["SendBroadcastMessage"] = s.luaMessageMethod()
	return &scripting.Object{Type: "Creature", Fields: map[string]any{"Name": state.Name, "GUID": state.GUID, "Entry": state.Entry, "GossipMenuID": state.GossipMenuID, "NPCFlags": state.NPCFlags, "Map": state.Map, "X": state.X, "Y": state.Y, "Z": state.Z}, Methods: methods}
}

func (s *session) luaGameObject(ctx context.Context, guid uint64) *scripting.Object {
	if uint16(guid>>48) != 0xF110 {
		return nil
	}
	low := uint32(guid & 0x00FFFFFF)
	entry := uint32((guid >> 24) & 0x00FFFFFF)
	state, ok := s.loadLuaGameObject(ctx, low, entry)
	if !ok || s.server.isGameObjectHidden(state.GUID) {
		return nil
	}
	return s.luaGameObjectObject(state)
}

func (s *session) loadLuaGameObject(ctx context.Context, low, entry uint32) (luaGameObjectState, bool) {
	var state luaGameObjectState
	var mapID int64
	if err := s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT g.guid, g.id, t.displayId, t.name, g.map, g.position_x, g.position_y, g.position_z FROM gameobject AS g JOIN gameobject_template AS t ON t.entry = g.id WHERE g.guid = ? AND g.id = ?", low, entry).Scan(&state.LowGUID, &state.Entry, &state.DisplayID, &state.Name, &mapID, &state.X, &state.Y, &state.Z); err != nil {
		return state, false
	}
	state.Map, state.GUID = uint32(mapID), gameObjectGUID(state.LowGUID, state.Entry)
	return state, true
}

func (s *session) luaGameObjectObject(state luaGameObjectState) *scripting.Object {
	methods := map[string]scripting.ObjectMethod{}
	methods["GetName"] = luaNoArgs(func() any { return state.Name })
	methods["GetEntry"] = luaNoArgs(func() any { return state.Entry })
	methods["GetDisplayId"] = luaNoArgs(func() any { return state.DisplayID })
	methods["GetGUID"] = luaNoArgs(func() any { return state.GUID })
	methods["GetGUIDLow"] = luaNoArgs(func() any { return state.LowGUID })
	methods["GetObjectType"] = luaNoArgs(func() any { return "GameObject" })
	methods["GetMapId"] = luaNoArgs(func() any { return state.Map })
	methods["GetX"] = luaNoArgs(func() any { return state.X })
	methods["GetY"] = luaNoArgs(func() any { return state.Y })
	methods["GetZ"] = luaNoArgs(func() any { return state.Z })
	methods["RemoveFromWorld"] = func(_ context.Context, _ []any) ([]any, error) {
		s.server.objectsMu.Lock()
		if s.server.hiddenGameObjects == nil {
			s.server.hiddenGameObjects = make(map[uint64]struct{})
		}
		s.server.hiddenGameObjects[state.GUID] = struct{}{}
		s.server.objectsMu.Unlock()
		return nil, nil
	}
	return &scripting.Object{Type: "GameObject", Fields: map[string]any{"Name": state.Name, "GUID": state.GUID, "Entry": state.Entry}, Methods: methods}
}

func (s *session) nearestGameObject(ctx context.Context, mapID uint32, x, y float32, distance float32) *scripting.Object {
	rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT g.guid, g.id, t.displayId, t.name, g.position_x, g.position_y, g.position_z FROM gameobject AS g JOIN gameobject_template AS t ON t.entry = g.id WHERE g.map = ? AND g.position_x BETWEEN ? AND ? AND g.position_y BETWEEN ? AND ? ORDER BY g.guid", mapID, float64(x-distance), float64(x+distance), float64(y-distance), float64(y+distance))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var nearest luaGameObjectState
	nearestDistance := float32(math.MaxFloat32)
	for rows.Next() {
		var candidate luaGameObjectState
		if err := rows.Scan(&candidate.LowGUID, &candidate.Entry, &candidate.DisplayID, &candidate.Name, &candidate.X, &candidate.Y, &candidate.Z); err != nil {
			continue
		}
		candidate.Map = mapID
		candidate.GUID = gameObjectGUID(candidate.LowGUID, candidate.Entry)
		if s.server.isGameObjectHidden(candidate.GUID) {
			continue
		}
		currentDistance := float32(math.Hypot(float64(candidate.X-x), float64(candidate.Y-y)))
		if currentDistance <= distance && currentDistance < nearestDistance {
			nearest, nearestDistance = candidate, currentDistance
		}
	}
	if nearest.GUID == 0 {
		return nil
	}
	return s.luaGameObjectObject(nearest)
}

func (s *Server) isGameObjectHidden(guid uint64) bool {
	s.objectsMu.RLock()
	_, hidden := s.hiddenGameObjects[guid]
	s.objectsMu.RUnlock()
	return hidden
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}

func maxUint64(value int64, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}
