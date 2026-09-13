package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	modelFlagM2         uint32 = 1
	modelFlagWorldSpawn uint32 = 1 << 1
	modelFlagHasBound   uint32 = 1 << 2
)

func buildADTDirBin(info adtInfo, mapID, tileX, tileY uint32) ([]byte, error) {
	ids := make(map[[2]uint32]uint32)
	nextID := uint32(1)
	uniqueID := func(clientID uint32, doodadID uint16) uint32 {
		key := [2]uint32{clientID, uint32(doodadID)}
		if value, ok := ids[key]; ok {
			return value
		}
		ids[key] = nextID
		nextID++
		return ids[key]
	}
	var output bytes.Buffer
	order := info.InstanceOrder
	if len(order) == 0 {
		order = make([]adtModelInstanceRef, 0, len(info.WorldModels)+len(info.Doodads))
		for index := range info.Doodads {
			order = append(order, adtModelInstanceRef{Doodad: true, Index: index})
		}
		for index := range info.WorldModels {
			order = append(order, adtModelInstanceRef{Index: index})
		}
	}
	for _, reference := range order {
		if reference.Doodad {
			if reference.Index < 0 || reference.Index >= len(info.Doodads) {
				return nil, fmt.Errorf("MDDF instance index %d is out of range", reference.Index)
			}
			instance := info.Doodads[reference.Index]
			if int(instance.NameID) >= len(info.DoodadNames) {
				return nil, fmt.Errorf("MDDF name id %d is out of range", instance.NameID)
			}
			name := plainModelName(info.DoodadNames[instance.NameID])
			if name == "" {
				return nil, fmt.Errorf("MDDF name id %d has an empty model name", instance.NameID)
			}
			writeDirRecord(&output, mapID, tileX, tileY, modelSpawn{Flags: modelFlagM2 | worldSpawnFlag(tileX, tileY), ID: uniqueID(instance.UniqueID, 0), Position: fixModelVector(instance.Position), Rotation: instance.Rotation, Scale: instance.Scale, Name: name})
			continue
		}
		if reference.Index < 0 || reference.Index >= len(info.WorldModels) {
			return nil, fmt.Errorf("MODF instance index %d is out of range", reference.Index)
		}
		instance := info.WorldModels[reference.Index]
		if instance.Flags&1 != 0 {
			continue
		}
		if int(instance.NameID) >= len(info.WorldModelNames) {
			return nil, fmt.Errorf("MODF name id %d is out of range", instance.NameID)
		}
		name := plainModelName(info.WorldModelNames[instance.NameID])
		if name == "" {
			return nil, fmt.Errorf("MODF name id %d has an empty model name", instance.NameID)
		}
		position := instance.Position
		if position[0] == 0 && position[2] == 0 {
			position[0], position[2] = 533.33333*32, 533.33333*32
		}
		writeDirRecord(&output, mapID, tileX, tileY, modelSpawn{Flags: modelFlagHasBound | worldSpawnFlag(tileX, tileY), ADTID: instance.NameSet, ID: uniqueID(instance.UniqueID, 0), Position: fixModelVector(position), Rotation: instance.Rotation, Scale: 1, HasBounds: true, BoundsMin: fixModelVector(instance.BoundsMin), BoundsMax: fixModelVector(instance.BoundsMax), Name: name})
	}
	return output.Bytes(), nil
}

func buildWDTDirBin(info wdtInfo, mapID uint32) ([]byte, error) {
	order := make([]adtModelInstanceRef, len(info.GlobalWMOModels))
	for index := range order {
		order[index] = adtModelInstanceRef{Index: index}
	}
	return buildADTDirBin(adtInfo{WorldModels: info.GlobalWMOModels, WorldModelNames: info.GlobalWMOModelNames, InstanceOrder: order}, mapID, 65, 65)
}

type modelSpawn struct {
	Flags                uint32
	ADTID                uint16
	ID                   uint32
	Position, Rotation   [3]float32
	Scale                float32
	HasBounds            bool
	BoundsMin, BoundsMax [3]float32
	Name                 string
}

func writeDirRecord(output *bytes.Buffer, mapID, tileX, tileY uint32, spawn modelSpawn) {
	_ = binary.Write(output, binary.LittleEndian, mapID)
	_ = binary.Write(output, binary.LittleEndian, tileX)
	_ = binary.Write(output, binary.LittleEndian, tileY)
	_ = binary.Write(output, binary.LittleEndian, spawn.Flags)
	_ = binary.Write(output, binary.LittleEndian, uint16(spawn.ADTID))
	_ = binary.Write(output, binary.LittleEndian, spawn.ID)
	_ = binary.Write(output, binary.LittleEndian, spawn.Position)
	_ = binary.Write(output, binary.LittleEndian, spawn.Rotation)
	_ = binary.Write(output, binary.LittleEndian, spawn.Scale)
	if spawn.HasBounds {
		_ = binary.Write(output, binary.LittleEndian, spawn.BoundsMin)
		_ = binary.Write(output, binary.LittleEndian, spawn.BoundsMax)
	}
	_ = binary.Write(output, binary.LittleEndian, uint32(len(spawn.Name)))
	output.WriteString(spawn.Name)
}

func worldSpawnFlag(tileX, tileY uint32) uint32 {
	if tileX == 65 && tileY == 65 {
		return modelFlagWorldSpawn
	}
	return 0
}

func fixModelVector(value [3]float32) [3]float32 {
	return [3]float32{value[2], value[0], value[1]}
}

func plainModelName(name string) string {
	if separator := strings.LastIndexAny(name, "\\/"); separator >= 0 {
		name = name[separator+1:]
	}
	if len(name) < 3 {
		return name
	}
	data := []byte(name)
	for index := 0; index < len(data)-3; index++ {
		previousAlpha := index > 0 && isASCIIAlpha(data[index-1])
		if previousAlpha && data[index] >= 'A' && data[index] <= 'Z' {
			data[index] |= 0x20
		} else if !previousAlpha && data[index] >= 'a' && data[index] <= 'z' {
			data[index] &^= 0x20
		}
	}
	for index := len(data) - 3; index < len(data); index++ {
		if data[index] >= 'A' && data[index] <= 'Z' {
			data[index] |= 0x20
		}
	}
	for index := 0; index < len(data)-3; index++ {
		if data[index] == ' ' {
			data[index] = '_'
		}
	}
	return string(data)
}

func isASCIIAlpha(value byte) bool {
	return (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}
