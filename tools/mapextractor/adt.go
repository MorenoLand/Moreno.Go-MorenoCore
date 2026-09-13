package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type adtInfo struct {
	HasMHDR           bool
	HasMCIN           bool
	HasMTEX           bool
	HasMMDX           bool
	HasMMID           bool
	HasMWMO           bool
	HasMWID           bool
	HasMDDF           bool
	HasMODF           bool
	MCNKCount         int
	MH2OCount         int
	MCLQCount         int
	MCVTCount         int
	MCLYCount         int
	MCALCount         int
	MCVTHeights       int
	HeightMin         float32
	HeightMax         float32
	MH2OHeaders       int
	LiquidLayers      int
	LiquidAttributes  int
	LiquidInstances   int
	LiquidTiles       int
	LiquidExistsBytes int
	LiquidVertexBytes int
	MCLQBytes         int
	Cells             []adtCellInfo
}

type adtCellInfo struct {
	Flags      uint32
	X          uint32
	Y          uint32
	Layers     uint32
	DoodadRefs uint32
	AreaID     uint32
	Holes      uint32
}

func parseADT(data []byte) (adtInfo, error) {
	var info adtInfo
	for offset := 0; offset < len(data); {
		if len(data)-offset < 8 {
			return adtInfo{}, errors.New("truncated ADT chunk header")
		}
		name := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if size < 0 || size > len(data)-offset {
			return adtInfo{}, fmt.Errorf("invalid ADT chunk %q size %d", name, size)
		}
		chunk := data[offset : offset+size]
		switch name {
		case "MHDR":
			info.HasMHDR = true
		case "MCIN":
			info.HasMCIN = true
		case "MTEX":
			info.HasMTEX = true
		case "MMDX":
			info.HasMMDX = true
		case "MMID":
			info.HasMMID = true
		case "MWMO":
			info.HasMWMO = true
		case "MWID":
			info.HasMWID = true
		case "MDDF":
			info.HasMDDF = true
		case "MODF":
			info.HasMODF = true
		case "MH2O":
			info.MH2OCount++
			if err := parseMH2O(chunk, &info); err != nil {
				return adtInfo{}, err
			}
		case "MCNK":
			info.MCNKCount++
			if err := countADTSubchunks(chunk, &info); err != nil {
				return adtInfo{}, err
			}
		}
		offset += size
	}
	if info.MCNKCount == 0 {
		return adtInfo{}, errors.New("ADT MCNK chunks not found")
	}
	return info, nil
}

func parseMH2O(chunk []byte, info *adtInfo) error {
	const headerBytes = 256 * 12
	if len(chunk) < headerBytes {
		return fmt.Errorf("truncated ADT MH2O chunk: got %d, want at least %d", len(chunk), headerBytes)
	}
	info.MH2OHeaders = 256
	for index := 0; index < 256; index++ {
		base := index * 12
		offsetInstances := binary.LittleEndian.Uint32(chunk[base:])
		layers := binary.LittleEndian.Uint32(chunk[base+4:])
		offsetAttributes := binary.LittleEndian.Uint32(chunk[base+8:])
		if layers == 0 {
			continue
		}
		info.LiquidLayers += int(layers)
		if offsetInstances == 0 || uint64(offsetInstances) >= uint64(len(chunk)) {
			return fmt.Errorf("invalid ADT MH2O instance offset %d", offsetInstances)
		}
		instancesEnd := uint64(offsetInstances) + uint64(layers)*24
		if instancesEnd > uint64(len(chunk)) {
			return fmt.Errorf("truncated ADT MH2O instances at offset %d", offsetInstances)
		}
		for layer := uint32(0); layer < layers; layer++ {
			base := int(offsetInstances + layer*24)
			lvf := binary.LittleEndian.Uint16(chunk[base+2:])
			width, height := chunk[base+14], chunk[base+15]
			if width == 0 || height == 0 || width > 8 || height > 8 {
				return fmt.Errorf("invalid ADT MH2O liquid dimensions %d x %d", width, height)
			}
			info.LiquidInstances++
			info.LiquidTiles += int(width) * int(height)
			existsOffset := binary.LittleEndian.Uint32(chunk[base+16:])
			if existsOffset != 0 {
				bitmapBytes := (uint32(width)*uint32(height) + 7) / 8
				if uint64(existsOffset)+uint64(bitmapBytes) > uint64(len(chunk)) {
					return fmt.Errorf("invalid ADT MH2O exists bitmap offset %d", existsOffset)
				}
				info.LiquidExistsBytes += int(bitmapBytes)
			}
			vertexOffset := binary.LittleEndian.Uint32(chunk[base+20:])
			if vertexOffset != 0 {
				bytesPerVertex := map[uint16]uint64{0: 5, 1: 8, 2: 1, 3: 9}[lvf]
				if bytesPerVertex == 0 {
					return fmt.Errorf("unsupported ADT MH2O liquid vertex format %d", lvf)
				}
				vertexBytes := uint64(width+1) * uint64(height+1) * bytesPerVertex
				if uint64(vertexOffset)+vertexBytes > uint64(len(chunk)) {
					return fmt.Errorf("invalid ADT MH2O vertex offset %d", vertexOffset)
				}
				info.LiquidVertexBytes += int(vertexBytes)
			}
		}
		if offsetAttributes != 0 {
			if uint64(offsetAttributes)+16 > uint64(len(chunk)) {
				return fmt.Errorf("invalid ADT MH2O attribute offset %d", offsetAttributes)
			}
			info.LiquidAttributes++
		}
	}
	return nil
}

func countADTSubchunks(chunk []byte, info *adtInfo) error {
	const mcnkHeaderSize = 128
	if len(chunk) <= mcnkHeaderSize {
		return nil
	}
	cell := adtCellInfo{Flags: binary.LittleEndian.Uint32(chunk[0:]), X: binary.LittleEndian.Uint32(chunk[4:]), Y: binary.LittleEndian.Uint32(chunk[8:]), Layers: binary.LittleEndian.Uint32(chunk[12:]), DoodadRefs: binary.LittleEndian.Uint32(chunk[16:]), AreaID: binary.LittleEndian.Uint32(chunk[52:]), Holes: binary.LittleEndian.Uint32(chunk[60:])}
	if cell.X >= 16 || cell.Y >= 16 {
		return fmt.Errorf("invalid ADT MCNK cell coordinates %d,%d", cell.X, cell.Y)
	}
	info.Cells = append(info.Cells, cell)
	for offset := mcnkHeaderSize; offset+8 <= len(chunk); {
		name := string(chunk[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(chunk[offset+4 : offset+8]))
		offset += 8
		if size < 0 || size > len(chunk)-offset {
			return fmt.Errorf("invalid ADT MCNK subchunk %q size %d", name, size)
		}
		switch name {
		case "MCVT":
			if size < 145*4 {
				return fmt.Errorf("truncated ADT MCVT subchunk: got %d, want %d", size, 145*4)
			}
			info.MCVTCount++
			previousHeights := info.MCVTHeights
			info.MCVTHeights += size / 4
			for index := 0; index < 145; index++ {
				value := math.Float32frombits(binary.LittleEndian.Uint32(chunk[offset+index*4:]))
				if previousHeights == 0 && index == 0 {
					info.HeightMin, info.HeightMax = value, value
				} else {
					if value < info.HeightMin {
						info.HeightMin = value
					}
					if value > info.HeightMax {
						info.HeightMax = value
					}
				}
			}
		case "MCLY":
			info.MCLYCount++
		case "MCAL":
			info.MCALCount++
		case "MCLQ":
			if size < 804 {
				return fmt.Errorf("truncated ADT MCLQ subchunk: got %d, want %d", size, 804)
			}
			info.MCLQCount++
			info.MCLQBytes += size
		}
		offset += size
	}
	return nil
}
