package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/tools/mpq"
)

type wmoRootInfo struct {
	Groups      uint32
	ID          uint32
	DoodadNames map[uint32]string
	DoodadSets  []wmoDoodadSet
	Doodads     []wmoDoodad
}

type wmoDoodadSet struct {
	Name       string
	StartIndex uint32
	Count      uint32
}

type wmoDoodad struct {
	NameIndex uint32
	Position  [3]float32
	Rotation  [4]float32
	Scale     float32
	Color     uint32
}

type wmoGroupInfo struct {
	Flags    uint32
	ID       uint32
	Low      [3]float32
	High     [3]float32
	Branches []uint32
	Indices  []uint16
	Vertices [][3]float32
}

type wmoChunk struct {
	Name string
	Data []byte
}

func extractWMO(archive *mpq.Archive, rootName string, files []string) ([]byte, error) {
	rootData, err := archive.ReadFile(rootName)
	if err != nil {
		return nil, fmt.Errorf("read WMO root %s: %w", rootName, err)
	}
	root, err := parseWMORoot(rootData)
	if err != nil {
		return nil, fmt.Errorf("parse WMO root %s: %w", rootName, err)
	}
	groupNames := make(map[string]string, len(files))
	for _, name := range files {
		groupNames[normalizeWMOName(name)] = name
	}
	base := strings.TrimSuffix(rootName, filepath.Ext(rootName))
	groups := make([]wmoGroupInfo, 0, root.Groups)
	for index := uint32(0); index < root.Groups; index++ {
		wanted := fmt.Sprintf("%s_%03d.wmo", base, index)
		actual, ok := groupNames[normalizeWMOName(wanted)]
		if !ok {
			return nil, fmt.Errorf("WMO group %s is missing", wanted)
		}
		data, err := archive.ReadFile(actual)
		if err != nil {
			return nil, fmt.Errorf("read WMO group %s: %w", actual, err)
		}
		group, err := parseWMOGroup(data)
		if err != nil {
			return nil, fmt.Errorf("parse WMO group %s: %w", actual, err)
		}
		groups = append(groups, group)
	}
	return writeRawWMO(root, groups)
}

func normalizeWMOName(name string) string {
	return strings.ToLower(filepath.ToSlash(name))
}

func isWMOGroupName(name string) bool {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if len(base) < 4 || base[len(base)-4] != '_' {
		return false
	}
	for _, value := range base[len(base)-3:] {
		if value < '0' || value > '9' {
			return false
		}
	}
	return strings.EqualFold(filepath.Ext(name), ".wmo")
}

func parseWMORoot(data []byte) (wmoRootInfo, error) {
	var root wmoRootInfo
	chunks, err := parseWMOChunks(data)
	if err != nil {
		return root, err
	}
	for _, chunk := range chunks {
		switch chunk.Name {
		case "MOHD":
			if len(chunk.Data) < 36 {
				return root, errors.New("truncated MOHD chunk")
			}
			root.Groups = binary.LittleEndian.Uint32(chunk.Data[4:8])
			root.ID = binary.LittleEndian.Uint32(chunk.Data[32:36])
			if root.Groups > 65535 {
				return root, errors.New("WMO group count is unreasonable")
			}
		case "MODN":
			root.DoodadNames = parseWMONameOffsets(chunk.Data)
		case "MODS":
			sets, err := parseWMODoodadSets(chunk.Data)
			if err != nil {
				return root, err
			}
			root.DoodadSets = sets
		case "MODD":
			doodads, err := parseWMODoodads(chunk.Data)
			if err != nil {
				return root, err
			}
			root.Doodads = doodads
		}
	}
	if root.Groups == 0 && root.ID == 0 {
		return root, errors.New("MOHD chunk not found")
	}
	return root, nil
}

func parseWMONameOffsets(data []byte) map[uint32]string {
	names := make(map[uint32]string)
	for offset := 0; offset < len(data); {
		end := offset
		for end < len(data) && data[end] != 0 {
			end++
		}
		if end > offset {
			names[uint32(offset)] = string(data[offset:end])
		}
		offset = end + 1
	}
	return names
}

func parseWMODoodadSets(data []byte) ([]wmoDoodadSet, error) {
	const recordSize = 32
	if len(data)%recordSize != 0 {
		return nil, fmt.Errorf("invalid MODS size %d", len(data))
	}
	sets := make([]wmoDoodadSet, len(data)/recordSize)
	for index := range sets {
		base := index * recordSize
		name := data[base : base+20]
		if nul := bytes.IndexByte(name, 0); nul >= 0 {
			name = name[:nul]
		}
		sets[index] = wmoDoodadSet{Name: string(name), StartIndex: binary.LittleEndian.Uint32(data[base+20:]), Count: binary.LittleEndian.Uint32(data[base+24:])}
	}
	return sets, nil
}

func parseWMODoodads(data []byte) ([]wmoDoodad, error) {
	const recordSize = 40
	if len(data)%recordSize != 0 {
		return nil, fmt.Errorf("invalid MODD size %d", len(data))
	}
	doodads := make([]wmoDoodad, len(data)/recordSize)
	for index := range doodads {
		base := index * recordSize
		doodad := &doodads[index]
		doodad.NameIndex = binary.LittleEndian.Uint32(data[base:]) & 0x00FFFFFF
		for axis := 0; axis < 3; axis++ {
			doodad.Position[axis] = mathFloat32(data[base+4+axis*4:])
		}
		for axis := 0; axis < 4; axis++ {
			doodad.Rotation[axis] = mathFloat32(data[base+16+axis*4:])
		}
		doodad.Scale = mathFloat32(data[base+32:])
		doodad.Color = binary.LittleEndian.Uint32(data[base+36:])
	}
	return doodads, nil
}

func parseWMOGroup(data []byte) (wmoGroupInfo, error) {
	var group wmoGroupInfo
	chunks, err := parseWMOChunks(data)
	if err != nil {
		return group, err
	}
	var mopy []byte
	var movi []byte
	var movt []byte
	var moba []byte
	for _, chunk := range chunks {
		switch chunk.Name {
		case "MOGP":
			if len(chunk.Data) < 60 {
				return group, errors.New("truncated MOGP chunk")
			}
			group.Flags = binary.LittleEndian.Uint32(chunk.Data[8:12])
			for i := 0; i < 3; i++ {
				group.Low[i] = mathFloat32(chunk.Data[12+i*4:])
				group.High[i] = mathFloat32(chunk.Data[24+i*4:])
			}
			group.ID = binary.LittleEndian.Uint32(chunk.Data[56:60])
		case "MOPY":
			mopy = chunk.Data
		case "MOVI":
			movi = chunk.Data
		case "MOVT":
			movt = chunk.Data
		case "MOBA":
			moba = chunk.Data
		}
	}
	if len(mopy) < 2 || len(movi) == 0 || len(movi)%2 != 0 || len(movt) == 0 || len(movt)%12 != 0 {
		return group, errors.New("WMO group geometry chunks are incomplete")
	}
	vertices := make([][3]float32, len(movt)/12)
	for i := range vertices {
		for axis := 0; axis < 3; axis++ {
			vertices[i][axis] = mathFloat32(movt[i*12+axis*4:])
		}
	}
	indices := make([]uint16, len(movi)/2)
	for i := range indices {
		indices[i] = binary.LittleEndian.Uint16(movi[i*2:])
	}
	used := make([]bool, len(vertices))
	filtered := make([]uint16, 0, len(indices))
	triangleCount := len(indices) / 3
	for triangle := 0; triangle < triangleCount && triangle*2+1 < len(mopy); triangle++ {
		flags := mopy[triangle*2]
		render := flags&0x20 != 0 && flags&0x04 == 0
		if flags&0x08 == 0 && !render {
			continue
		}
		for corner := 0; corner < 3; corner++ {
			index := indices[triangle*3+corner]
			if int(index) >= len(vertices) {
				return wmoGroupInfo{}, errors.New("WMO triangle index is out of range")
			}
			used[index] = true
			filtered = append(filtered, index)
		}
	}
	remap := make([]uint16, len(vertices))
	compact := make([][3]float32, 0)
	for index, present := range used {
		if present {
			remap[index] = uint16(len(compact))
			compact = append(compact, vertices[index])
		}
	}
	for index := range filtered {
		filtered[index] = remap[filtered[index]]
	}
	group.Vertices = compact
	group.Indices = filtered
	if len(moba)%24 == 0 {
		group.Branches = make([]uint32, len(moba)/24)
		for index := range group.Branches {
			group.Branches[index] = uint32(binary.LittleEndian.Uint16(moba[index*24+16:]))
		}
	}
	return group, nil
}

func parseWMOChunks(data []byte) ([]wmoChunk, error) {
	chunks := make([]wmoChunk, 0)
	for offset := 0; offset < len(data); {
		if len(data)-offset < 8 {
			return nil, errors.New("truncated WMO chunk header")
		}
		name := canonicalWMOChunk(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4:]))
		offset += 8
		if size < 0 || size > len(data)-offset {
			return nil, fmt.Errorf("invalid WMO chunk %q size %d", name, size)
		}
		chunks = append(chunks, wmoChunk{Name: name, Data: data[offset : offset+size]})
		offset += size
	}
	return chunks, nil
}

func canonicalWMOChunk(raw []byte) string {
	direct := string(raw)
	reversed := string([]byte{raw[3], raw[2], raw[1], raw[0]})
	known := func(value string) bool {
		switch value {
		case "MOHD", "MOGP", "MOPY", "MOVI", "MOVT", "MOBA", "MLIQ", "MODN", "MODS", "MODD":
			return true
		default:
			return false
		}
	}
	if known(direct) {
		return direct
	}
	return reversed
}

func mathFloat32(data []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data))
}

func writeRawWMO(root wmoRootInfo, groups []wmoGroupInfo) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("VMAP047")
	output.WriteByte(0)
	_ = binary.Write(&output, binary.LittleEndian, uint32(0))
	_ = binary.Write(&output, binary.LittleEndian, uint32(len(groups)))
	_ = binary.Write(&output, binary.LittleEndian, root.ID)
	for _, group := range groups {
		_ = binary.Write(&output, binary.LittleEndian, group.Flags)
		_ = binary.Write(&output, binary.LittleEndian, group.ID)
		_ = binary.Write(&output, binary.LittleEndian, group.Low)
		_ = binary.Write(&output, binary.LittleEndian, group.High)
		_ = binary.Write(&output, binary.LittleEndian, uint32(0))
		writeRawChunk(&output, "GRP ", uint32(4+len(group.Branches)*4))
		_ = binary.Write(&output, binary.LittleEndian, uint32(len(group.Branches)))
		_ = binary.Write(&output, binary.LittleEndian, group.Branches)
		writeRawChunk(&output, "INDX", uint32(4+len(group.Indices)*2))
		_ = binary.Write(&output, binary.LittleEndian, uint32(len(group.Indices)))
		_ = binary.Write(&output, binary.LittleEndian, group.Indices)
		writeRawChunk(&output, "VERT", uint32(4+len(group.Vertices)*12))
		_ = binary.Write(&output, binary.LittleEndian, uint32(len(group.Vertices)))
		_ = binary.Write(&output, binary.LittleEndian, group.Vertices)
	}
	return output.Bytes(), nil
}

func writeRawChunk(output *bytes.Buffer, name string, size uint32) {
	output.WriteString(name)
	_ = binary.Write(output, binary.LittleEndian, size)
}
