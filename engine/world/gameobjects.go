package world

import (
	"context"
	"math"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	gameObjectTypeMask       uint32 = 0x00000021
	gameObjectUpdateFlags    uint16 = 0x0350
	gameObjectValuesCount           = 18
	gameObjectDisplayID             = 8
	gameObjectFlags                 = 9
	gameObjectParentRotation        = 10
	gameObjectDynamic               = 14
	gameObjectFaction               = 15
	gameObjectLevel                 = 16
	gameObjectBytes1                = 17
)

type gameObjectSpawn struct {
	GUID           uint32
	Entry          uint32
	Map            uint32
	X              float32
	Y              float32
	Z              float32
	Orientation    float32
	RotationX      float32
	RotationY      float32
	RotationZ      float32
	RotationW      float32
	State          uint8
	AnimProgress   uint8
	ArtKit         uint8
	Type           uint8
	DisplayID      uint32
	Size           float32
	Flags          uint32
	Faction        uint32
	ParentRotation [4]float32
}

func (s *Server) buildNearbyGameObjectUpdates(ctx context.Context, state playerState) (*protocol.Packet, int, error) {
	distance := float64(s.Config.VisibilityDistanceContinents)
	if distance <= 0 {
		return nil, 0, nil
	}
	query := `SELECT g.guid, g.id, g.map, g.position_x, g.position_y, g.position_z, g.orientation, g.rotation0, g.rotation1, g.rotation2, g.rotation3, g.state, g.animprogress, t.type, t.displayId, t.size, COALESCE(ta.flags, 0), COALESCE(ta.faction, 0), COALESCE(ta.artkit0, 0), COALESCE(ga.parent_rotation0, 0), COALESCE(ga.parent_rotation1, 0), COALESCE(ga.parent_rotation2, 0), COALESCE(ga.parent_rotation3, 1)
		FROM gameobject AS g
		JOIN gameobject_template AS t ON t.entry = g.id
		LEFT JOIN gameobject_template_addon AS ta ON ta.entry = g.id
		LEFT JOIN gameobject_addon AS ga ON ga.guid = g.guid
		LEFT JOIN game_event_gameobject AS geg ON geg.guid = g.guid
		WHERE g.map = ? AND g.position_x BETWEEN ? AND ? AND g.position_y BETWEEN ? AND ?
		AND (g.spawnMask = 0 OR (g.spawnMask & 1) <> 0)
		AND (? OR g.phaseMask = 0 OR (g.phaseMask & 1) <> 0)
		AND (geg.eventEntry IS NULL OR geg.eventEntry = 0)
		ORDER BY g.guid`
	isGM := state.ExtraFlags&playerExtraGMOn != 0 || state.PlayerFlags&playerFlagGM != 0
	rows, err := s.WorldStore.DB.QueryContext(ctx, query, state.Map, float64(state.X)-distance, float64(state.X)+distance, float64(state.Y)-distance, float64(state.Y)+distance, isGM)
	if err != nil {
		fallbackQuery := `SELECT g.guid, g.id, g.map, g.position_x, g.position_y, g.position_z, g.orientation, g.rotation0, g.rotation1, g.rotation2, g.rotation3, g.state, g.animprogress, t.type, t.displayId, t.size, COALESCE(ta.flags, 0), COALESCE(ta.faction, 0), COALESCE(ta.artkit0, 0), COALESCE(ga.parent_rotation0, 0), COALESCE(ga.parent_rotation1, 0), COALESCE(ga.parent_rotation2, 0), COALESCE(ga.parent_rotation3, 1)
			FROM gameobject AS g
			JOIN gameobject_template AS t ON t.entry = g.id
			LEFT JOIN gameobject_template_addon AS ta ON ta.entry = g.id
			LEFT JOIN gameobject_addon AS ga ON ga.guid = g.guid
			WHERE g.map = ? AND g.position_x BETWEEN ? AND ? AND g.position_y BETWEEN ? AND ?
			AND (g.spawnMask = 0 OR (g.spawnMask & 1) <> 0)
			AND (? OR g.phaseMask = 0 OR (g.phaseMask & 1) <> 0)
			ORDER BY g.guid`
		rows, err = s.WorldStore.DB.QueryContext(ctx, fallbackQuery, state.Map, float64(state.X)-distance, float64(state.X)+distance, float64(state.Y)-distance, float64(state.Y)+distance, isGM)
		if err != nil {
			if missingTable(err) {
				return nil, 0, nil
			}
			return nil, 0, err
		}
	}
	defer rows.Close()
	updates := protocol.NewUpdateData()
	count := 0
	for rows.Next() {
		var spawn gameObjectSpawn
		var guid, entry, mapID, stateValue, animProgress, objectType, displayID, flags, faction, artKit int64
		var x, y, z, orientation, rotationX, rotationY, rotationZ, rotationW, size, parentRotation0, parentRotation1, parentRotation2, parentRotation3 float64
		if err := rows.Scan(&guid, &entry, &mapID, &x, &y, &z, &orientation, &rotationX, &rotationY, &rotationZ, &rotationW, &stateValue, &animProgress, &objectType, &displayID, &size, &flags, &faction, &artKit, &parentRotation0, &parentRotation1, &parentRotation2, &parentRotation3); err != nil {
			return nil, count, err
		}
		if math.Hypot(x-float64(state.X), y-float64(state.Y)) > distance || !validMovementPosition(float32(x), float32(y), float32(z), float32(orientation)) {
			continue
		}
		spawn.GUID = uint32(guid)
		spawn.Entry = uint32(entry)
		spawn.Map = uint32(mapID)
		spawn.X, spawn.Y, spawn.Z, spawn.Orientation = float32(x), float32(y), float32(z), float32(orientation)
		spawn.RotationX, spawn.RotationY, spawn.RotationZ, spawn.RotationW = float32(rotationX), float32(rotationY), float32(rotationZ), float32(rotationW)
		spawn.State, spawn.AnimProgress, spawn.ArtKit, spawn.Type = uint8(stateValue), uint8(animProgress), uint8(artKit), uint8(objectType)
		spawn.DisplayID, spawn.Size, spawn.Flags, spawn.Faction = uint32(displayID), float32(size), uint32(flags), uint32(faction)
		spawn.ParentRotation = [4]float32{float32(parentRotation0), float32(parentRotation1), float32(parentRotation2), float32(parentRotation3)}
		// Server-side visibility: script-hidden objects stay visible to
		// GMs the way SetVisible(false) units remain visible to GM seers.
		if !isGM && s.isGameObjectHidden(gameObjectGUID(spawn.GUID, spawn.Entry)) {
			continue
		}
		updates.AddUpdateBlock(buildGameObjectUpdate(spawn))
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, count, err
	}
	if count == 0 {
		return nil, 0, nil
	}
	packet, err := updates.BuildPacket(0)
	return packet, count, err
}

func buildGameObjectUpdate(spawn gameObjectSpawn) []byte {
	rawGUID := gameObjectGUID(spawn.GUID, spawn.Entry)
	values := make([]uint32, gameObjectValuesCount)
	values[0] = uint32(rawGUID)
	values[1] = uint32(rawGUID >> 32)
	values[2] = gameObjectTypeMask
	values[objectFieldEntry] = spawn.Entry
	values[4] = math.Float32bits(spawn.Size)
	values[gameObjectDisplayID] = spawn.DisplayID
	values[gameObjectFlags] = spawn.Flags
	values[gameObjectParentRotation] = math.Float32bits(spawn.ParentRotation[0])
	values[gameObjectParentRotation+1] = math.Float32bits(spawn.ParentRotation[1])
	values[gameObjectParentRotation+2] = math.Float32bits(spawn.ParentRotation[2])
	values[gameObjectParentRotation+3] = math.Float32bits(spawn.ParentRotation[3])
	values[gameObjectDynamic] = 0xFFFF0000
	values[gameObjectFaction] = spawn.Faction
	values[gameObjectBytes1] = uint32(spawn.State) | uint32(spawn.Type)<<8 | uint32(spawn.ArtKit)<<16 | uint32(spawn.AnimProgress)<<24
	mask := protocol.NewUpdateMask(len(values))
	for index, value := range values {
		if value != 0 {
			_ = mask.Set(index)
		}
	}
	block := protocol.NewBuffer(256)
	block.WriteU8(protocol.UpdateCreateObject2)
	block.WritePackedGUID(rawGUID)
	block.WriteU8(5)
	block.WriteU16(gameObjectUpdateFlags)
	block.WriteU8(0)
	block.WriteF32(spawn.X)
	block.WriteF32(spawn.Y)
	block.WriteF32(spawn.Z)
	block.WriteF32(spawn.X)
	block.WriteF32(spawn.Y)
	block.WriteF32(spawn.Z)
	block.WriteF32(spawn.Orientation)
	block.WriteF32(0)
	block.WriteU32(spawn.GUID)
	block.WriteU64(packGameObjectRotation(spawn.RotationX, spawn.RotationY, spawn.RotationZ, spawn.RotationW))
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index, value := range values {
		if mask.Has(index) {
			block.WriteU32(value)
		}
	}
	return block.Bytes()
}

func gameObjectGUID(guid, entry uint32) uint64 {
	return uint64(guid) | uint64(entry)<<24 | uint64(0xF110)<<48
}

func packGameObjectRotation(x, y, z, w float32) uint64 {
	norm := math.Sqrt(float64(x*x + y*y + z*z + w*w))
	if norm == 0 || math.IsNaN(norm) || math.IsInf(norm, 0) {
		x, y, z, w = 0, 0, 0, 1
	} else {
		x, y, z, w = float32(float64(x)/norm), float32(float64(y)/norm), float32(float64(z)/norm), float32(float64(w)/norm)
	}
	const packYZ int64 = 1 << 20
	const packX int64 = packYZ << 1
	const packYZMask int64 = (packYZ << 1) - 1
	const packXMask int64 = (packX << 1) - 1
	wSign := int64(1)
	if w < 0 {
		wSign = -1
	}
	packedX := (int64(int32(float64(x)*float64(packX))) * wSign) & packXMask
	packedY := (int64(int32(float64(y)*float64(packYZ))) * wSign) & packYZMask
	packedZ := (int64(int32(float64(z)*float64(packYZ))) * wSign) & packYZMask
	return uint64(packedZ | packedY<<21 | packedX<<42)
}
