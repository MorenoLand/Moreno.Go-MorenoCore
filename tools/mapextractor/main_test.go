package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/tools/wowdata"
)

func wdtChunk(name string, payload []byte) []byte {
	chunk := make([]byte, 8+len(payload))
	copy(chunk, name)
	binary.LittleEndian.PutUint32(chunk[4:8], uint32(len(payload)))
	copy(chunk[8:], payload)
	return chunk
}

func TestParseWDTMainTiles(t *testing.T) {
	mphd := make([]byte, 32)
	binary.LittleEndian.PutUint32(mphd, 0x1234)
	main := make([]byte, 64*64*8)
	binary.LittleEndian.PutUint32(main[(3*64+2)*8:], 1)
	binary.LittleEndian.PutUint32(main[(3*64+2)*8+4:], 7)
	binary.LittleEndian.PutUint32(main[(63*64+63)*8:], 2)
	data := append(wdtChunk("MVER", func() []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, 18); return b }()), wdtChunk("MPHD", mphd)...)
	data = append(data, wdtChunk("MAIN", main)...)
	data = append(data, wdtChunk("MWMO", []byte("World\\Map.wmo\x00"))...)
	modf := make([]byte, 64)
	binary.LittleEndian.PutUint32(modf[0:], 0)
	binary.LittleEndian.PutUint32(modf[4:], 77)
	binary.LittleEndian.PutUint32(modf[8:], math.Float32bits(1))
	binary.LittleEndian.PutUint16(modf[60:], 4)
	binary.LittleEndian.PutUint16(modf[62:], 1024)
	data = append(data, wdtChunk("MODF", modf)...)
	info, err := parseWDT(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != 18 || !info.HasMain || !info.HasGlobalWMO || info.GlobalWMO != "World\\Map.wmo" || len(info.GlobalWMOModelNames) != 1 || len(info.GlobalWMOModels) != 1 || info.GlobalWMOModels[0].UniqueID != 77 || info.GlobalWMOModels[0].NameSet != 4 || info.TileCount != 2 || info.MPHD[0] != 0x1234 {
		t.Fatalf("unexpected WDT info: %+v", info)
	}
	if info.Tiles[3][2].Exists != 1 || info.Tiles[3][2].Data != 7 || info.Tiles[63][63].Exists != 2 {
		t.Fatalf("unexpected WDT tile data: %+v", info.Tiles[3][2])
	}
}

func TestParseWDTRejectsMissingMain(t *testing.T) {
	if _, err := parseWDT(wdtChunk("MVER", []byte{18, 0, 0, 0})); err == nil {
		t.Fatal("expected missing MAIN error")
	}
}

func TestParseWDTRejectsTruncatedMain(t *testing.T) {
	if _, err := parseWDT(wdtChunk("MAIN", make([]byte, 8))); err == nil {
		t.Fatal("expected truncated MAIN error")
	}
}

func TestBuildWDTDirBinUsesWorldSpawnRecord(t *testing.T) {
	payload, err := buildWDTDirBin(wdtInfo{GlobalWMOModelNames: []string{"World\\Global.WMO"}, GlobalWMOModels: []adtWorldModelInstance{{NameID: 0, UniqueID: 42, NameSet: 3, Position: [3]float32{1, 2, 3}, BoundsMin: [3]float32{-1, -2, -3}, BoundsMax: [3]float32{4, 5, 6}}}}, 571)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 22 || binary.LittleEndian.Uint32(payload[0:]) != 571 || binary.LittleEndian.Uint32(payload[4:]) != 65 || binary.LittleEndian.Uint32(payload[8:]) != 65 || binary.LittleEndian.Uint32(payload[12:]) != modelFlagHasBound|modelFlagWorldSpawn || binary.LittleEndian.Uint16(payload[16:]) != 3 || binary.LittleEndian.Uint32(payload[18:]) != 1 {
		t.Fatalf("unexpected WDT dir_bin header: %x", payload[:22])
	}
}

func TestParseADTChunkInventory(t *testing.T) {
	mcnk := make([]byte, 128)
	binary.LittleEndian.PutUint32(mcnk[0:], 0x10)
	binary.LittleEndian.PutUint32(mcnk[4:], 2)
	binary.LittleEndian.PutUint32(mcnk[8:], 3)
	binary.LittleEndian.PutUint32(mcnk[12:], 1)
	binary.LittleEndian.PutUint32(mcnk[16:], 4)
	binary.LittleEndian.PutUint32(mcnk[52:], 42)
	binary.LittleEndian.PutUint32(mcnk[60:], 0x100)
	heights := make([]byte, 145*4)
	for index := 0; index < 145; index++ {
		binary.LittleEndian.PutUint32(heights[index*4:], math.Float32bits(10))
	}
	binary.LittleEndian.PutUint32(heights[len(heights)-4:], math.Float32bits(20))
	mcnk = append(mcnk, wdtChunk("MCVT", heights)...)
	mcnk = append(mcnk, wdtChunk("MCLY", make([]byte, 4))...)
	mcnk = append(mcnk, wdtChunk("MCAL", make([]byte, 4))...)
	mcnk = append(mcnk, wdtChunk("MCLQ", make([]byte, 804))...)
	data := append(wdtChunk("MHDR", make([]byte, 16)), wdtChunk("MCIN", make([]byte, 8))...)
	data = append(data, wdtChunk("MTEX", []byte("texture.blp\x00"))...)
	mh2o := make([]byte, 3133)
	binary.LittleEndian.PutUint32(mh2o[0:], 3072)
	binary.LittleEndian.PutUint32(mh2o[4:], 1)
	binary.LittleEndian.PutUint32(mh2o[8:], 3096)
	binary.LittleEndian.PutUint32(mh2o[3072+16:], 3112)
	binary.LittleEndian.PutUint32(mh2o[3072+20:], 3113)
	mh2o[3072+14], mh2o[3072+15] = 1, 1
	data = append(data, wdtChunk("MH2O", mh2o)...)
	mddf := make([]byte, 36)
	binary.LittleEndian.PutUint32(mddf[0:], 2)
	binary.LittleEndian.PutUint32(mddf[4:], 99)
	binary.LittleEndian.PutUint32(mddf[8:], math.Float32bits(1))
	binary.LittleEndian.PutUint32(mddf[20:], math.Float32bits(2))
	binary.LittleEndian.PutUint16(mddf[32:], 1024)
	binary.LittleEndian.PutUint16(mddf[34:], 1)
	data = append(data, wdtChunk("MDDF", mddf)...)
	modf := make([]byte, 64)
	binary.LittleEndian.PutUint32(modf[0:], 3)
	binary.LittleEndian.PutUint32(modf[4:], 100)
	binary.LittleEndian.PutUint32(modf[8:], math.Float32bits(3))
	binary.LittleEndian.PutUint32(modf[32:], math.Float32bits(-1))
	binary.LittleEndian.PutUint32(modf[44:], math.Float32bits(1))
	binary.LittleEndian.PutUint16(modf[56:], 2)
	binary.LittleEndian.PutUint16(modf[58:], 4)
	binary.LittleEndian.PutUint16(modf[60:], 5)
	binary.LittleEndian.PutUint16(modf[62:], 1024)
	data = append(data, wdtChunk("MODF", modf)...)
	data = append(data, wdtChunk("MCNK", mcnk)...)
	info, err := parseADT(data)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasMHDR || !info.HasMCIN || !info.HasMTEX || info.MH2OCount != 1 || info.MH2OHeaders != 256 || info.LiquidLayers != 1 || info.LiquidInstances != 1 || info.LiquidTiles != 1 || info.LiquidAttributes != 1 || info.LiquidExistsBytes != 1 || info.LiquidVertexBytes != 20 || info.MCNKCount != 1 || len(info.Cells) != 1 || info.Cells[0].X != 2 || info.Cells[0].Y != 3 || info.Cells[0].AreaID != 42 || info.Cells[0].Holes != 0x100 || info.MCVTCount != 1 || info.MCVTHeights != 145 || info.HeightMin != 10 || info.HeightMax != 20 || info.MCLYCount != 1 || info.MCALCount != 1 || info.MCLQCount != 1 || info.MCLQBytes != 804 || len(info.Doodads) != 1 || info.Doodads[0].NameID != 2 || info.Doodads[0].UniqueID != 99 || info.Doodads[0].Scale != 1 || len(info.WorldModels) != 1 || info.WorldModels[0].NameID != 3 || info.WorldModels[0].DoodadSet != 4 || info.WorldModels[0].NameSet != 5 {
		t.Fatalf("unexpected ADT info: %+v", info)
	}
}

func TestParseADTRejectsMissingMCNK(t *testing.T) {
	if _, err := parseADT(wdtChunk("MHDR", make([]byte, 16))); err == nil {
		t.Fatal("expected missing MCNK error")
	}
}

func TestBuildADTDirBinMatchesModelSpawnLayout(t *testing.T) {
	info := adtInfo{
		DoodadNames:     []string{"World\\Models\\Tree M2.MDX"},
		WorldModelNames: []string{"World\\Buildings\\Storm Wind.WMO"},
		Doodads:         []adtDoodadInstance{{NameID: 0, UniqueID: 11, Position: [3]float32{1, 2, 3}, Rotation: [3]float32{4, 5, 6}, Scale: 2}},
		WorldModels:     []adtWorldModelInstance{{NameID: 0, UniqueID: 22, Position: [3]float32{7, 8, 9}, Rotation: [3]float32{10, 11, 12}, BoundsMin: [3]float32{-1, -2, -3}, BoundsMax: [3]float32{4, 5, 6}, NameSet: 9}},
		InstanceOrder:   []adtModelInstanceRef{{Doodad: true, Index: 0}, {Index: 0}},
	}
	payload, err := buildADTDirBin(info, 571, 65, 65)
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(payload)
	readRecord := func(hasBounds bool) (uint32, uint16, uint32, [3]float32, [3]float32, float32, [3]float32, [3]float32, string) {
		var mapID, tileX, tileY, flags, id, nameLen uint32
		var adtID uint16
		var position, rotation, boundsMin, boundsMax [3]float32
		var scale float32
		if err := binary.Read(reader, binary.LittleEndian, &mapID); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &tileX); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &tileY); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &flags); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &adtID); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &id); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &position); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &rotation); err != nil {
			t.Fatal(err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &scale); err != nil {
			t.Fatal(err)
		}
		if hasBounds {
			if err := binary.Read(reader, binary.LittleEndian, &boundsMin); err != nil {
				t.Fatal(err)
			}
			if err := binary.Read(reader, binary.LittleEndian, &boundsMax); err != nil {
				t.Fatal(err)
			}
		}
		if err := binary.Read(reader, binary.LittleEndian, &nameLen); err != nil {
			t.Fatal(err)
		}
		name := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, name); err != nil {
			t.Fatal(err)
		}
		return flags, adtID, id, position, rotation, scale, boundsMin, boundsMax, string(name)
	}
	flags, adtID, id, position, rotation, scale, boundsMin, boundsMax, name := readRecord(false)
	if flags != modelFlagM2|modelFlagWorldSpawn || adtID != 0 || id != 1 || position != [3]float32{3, 1, 2} || rotation != [3]float32{4, 5, 6} || scale != 2 || boundsMin != [3]float32{} || boundsMax != [3]float32{} || name != "Tree_M2.mdx" {
		t.Fatalf("unexpected M2 dir_bin record flags=%d adt=%d id=%d pos=%v rot=%v scale=%v min=%v max=%v name=%q", flags, adtID, id, position, rotation, scale, boundsMin, boundsMax, name)
	}
	flags, adtID, id, position, rotation, scale, boundsMin, boundsMax, name = readRecord(true)
	if flags != modelFlagHasBound|modelFlagWorldSpawn || adtID != 9 || id != 2 || position != [3]float32{9, 7, 8} || rotation != [3]float32{10, 11, 12} || scale != 1 || boundsMin != [3]float32{-3, -1, -2} || boundsMax != [3]float32{6, 4, 5} || name != "Storm_Wind.wmo" {
		t.Fatalf("unexpected WMO dir_bin record flags=%d adt=%d id=%d pos=%v rot=%v scale=%v min=%v max=%v name=%q", flags, adtID, id, position, rotation, scale, boundsMin, boundsMax, name)
	}
	if reader.Len() != 0 {
		t.Fatalf("unexpected trailing dir_bin bytes=%d", reader.Len())
	}
}

func TestBuildADTDirBinExpandsWMODoodadSet(t *testing.T) {
	modelDir := t.TempDir()
	metadata := bytes.NewBuffer(nil)
	metadata.WriteString("MCWM")
	_ = binary.Write(metadata, binary.LittleEndian, uint32(1))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(1))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(0))
	name := "World\\Models\\Tree.m2"
	_ = binary.Write(metadata, binary.LittleEndian, uint32(len(name)))
	metadata.WriteString(name)
	_ = binary.Write(metadata, binary.LittleEndian, uint32(1))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(0))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(1))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(1))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(0))
	_ = binary.Write(metadata, binary.LittleEndian, [3]float32{1, 2, 3})
	_ = binary.Write(metadata, binary.LittleEndian, [4]float32{0, 0, 0, 1})
	_ = binary.Write(metadata, binary.LittleEndian, float32(1.5))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(0))
	_ = binary.Write(metadata, binary.LittleEndian, uint32(1))
	_ = binary.Write(metadata, binary.LittleEndian, uint16(0))
	rawWMO := append([]byte("VMAP047\x00"), make([]byte, 12)...)
	doodadChunk := append([]byte("DODM"), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(doodadChunk[4:], uint32(metadata.Len()))
	rawWMO = append(rawWMO, doodadChunk...)
	rawWMO = append(rawWMO, metadata.Bytes()...)
	if err := os.WriteFile(filepath.Join(modelDir, "Building.wmo"), rawWMO, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "Tree.m2"), append([]byte("VMAP047\x00"), []byte{3, 0, 0, 0}...), 0644); err != nil {
		t.Fatal(err)
	}
	info := adtInfo{WorldModelNames: []string{"Building.wmo"}, WorldModels: []adtWorldModelInstance{{NameID: 0, UniqueID: 44, Position: [3]float32{10, 20, 30}, BoundsMin: [3]float32{-1, -1, -1}, BoundsMax: [3]float32{1, 1, 1}, DoodadSet: 0}}, InstanceOrder: []adtModelInstanceRef{{Index: 0}}}
	payload, err := buildADTDirBinWithModelDir(info, 571, 1, 2, modelDir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(payload, []byte("Building.wmo")) != 1 || bytes.Count(payload, []byte("Tree.m2")) != 1 {
		t.Fatalf("expanded dir_bin missing WMO/doodad names: %q", payload)
	}
}

func TestExtractDBCMissingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "does_not_exist")
	outputDir := filepath.Join(tempDir, "output")

	_, err := wowdata.ExtractDBC(nonExistent, outputDir)
	if err == nil {
		t.Fatalf("expected error when extracting from non-existent directory")
	}
}

func TestExtractDBCInvalidDir(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Empty dir without MPQs should return 0 count or error
	count, err := wowdata.ExtractDBC(tempDir, outputDir)
	if err != nil && count != 0 {
		t.Fatalf("unexpected count %d with err: %v", count, err)
	}
	_ = os.RemoveAll(tempDir)
}
