package main

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type adtInfo struct {
	HasMHDR   bool
	HasMCIN   bool
	HasMTEX   bool
	HasMMDX   bool
	HasMMID   bool
	HasMWMO   bool
	HasMWID   bool
	HasMDDF   bool
	HasMODF   bool
	MCNKCount int
	MH2OCount int
	MCLQCount int
	MCVTCount int
	MCLYCount int
	MCALCount int
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
		case "MCNK":
			info.MCNKCount++
			countADTSubchunks(chunk, &info)
		}
		offset += size
	}
	if info.MCNKCount == 0 {
		return adtInfo{}, errors.New("ADT MCNK chunks not found")
	}
	return info, nil
}

func countADTSubchunks(chunk []byte, info *adtInfo) {
	const mcnkHeaderSize = 128
	if len(chunk) <= mcnkHeaderSize {
		return
	}
	for offset := mcnkHeaderSize; offset+8 <= len(chunk); {
		name := string(chunk[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(chunk[offset+4 : offset+8]))
		offset += 8
		if size < 0 || size > len(chunk)-offset {
			return
		}
		switch name {
		case "MCVT":
			info.MCVTCount++
		case "MCLY":
			info.MCLYCount++
		case "MCAL":
			info.MCALCount++
		case "MCLQ":
			info.MCLQCount++
		}
		offset += size
	}
}
