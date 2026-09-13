package main

import (
	"encoding/binary"
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
	info, err := parseWDT(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != 18 || !info.HasMain || !info.HasGlobalWMO || info.TileCount != 2 || info.MPHD[0] != 0x1234 {
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

func TestParseADTChunkInventory(t *testing.T) {
	mcnk := make([]byte, 128)
	heights := make([]byte, 145*4)
	for index := 0; index < 145; index++ {
		binary.LittleEndian.PutUint32(heights[index*4:], math.Float32bits(10))
	}
	binary.LittleEndian.PutUint32(heights[len(heights)-4:], math.Float32bits(20))
	mcnk = append(mcnk, wdtChunk("MCVT", heights)...)
	mcnk = append(mcnk, wdtChunk("MCLY", make([]byte, 4))...)
	mcnk = append(mcnk, wdtChunk("MCAL", make([]byte, 4))...)
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
	data = append(data, wdtChunk("MCNK", mcnk)...)
	info, err := parseADT(data)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasMHDR || !info.HasMCIN || !info.HasMTEX || info.MH2OCount != 1 || info.MH2OHeaders != 256 || info.LiquidLayers != 1 || info.LiquidInstances != 1 || info.LiquidTiles != 1 || info.LiquidAttributes != 1 || info.LiquidExistsBytes != 1 || info.LiquidVertexBytes != 20 || info.MCNKCount != 1 || info.MCVTCount != 1 || info.MCVTHeights != 145 || info.HeightMin != 10 || info.HeightMax != 20 || info.MCLYCount != 1 || info.MCALCount != 1 {
		t.Fatalf("unexpected ADT info: %+v", info)
	}
}

func TestParseADTRejectsMissingMCNK(t *testing.T) {
	if _, err := parseADT(wdtChunk("MHDR", make([]byte, 16))); err == nil {
		t.Fatal("expected missing MCNK error")
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
