package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Errorf("min(3, 5) = %d, want 3", min(3, 5))
	}
	if min(10, 2) != 2 {
		t.Errorf("min(10, 2) = %d, want 2", min(10, 2))
	}
	if min(4, 4) != 4 {
		t.Errorf("min(4, 4) = %d, want 4", min(4, 4))
	}
}

func TestBIHBuildsSerializedInteriorNodes(t *testing.T) {
	primitives := make([]bihPrimitive, 4)
	for index := range primitives {
		low := float32(index * 10)
		primitives[index] = bihPrimitive{Low: vector3{low, 0, 0}, High: vector3{low + 1, 1, 1}, Index: uint32(index)}
	}
	low, high, tree, objects := buildBIH(primitives)
	if low != (vector3{0, 0, 0}) || high != (vector3{31, 1, 1}) || len(tree) <= 3 || len(objects) != 4 {
		t.Fatalf("unexpected BIH bounds/tree low=%v high=%v tree=%d objects=%d", low, high, len(tree), len(objects))
	}
	seen := make(map[uint32]bool, len(objects))
	for _, index := range objects {
		if index >= 4 || seen[index] {
			t.Fatalf("invalid BIH object permutation: %v", objects)
		}
		seen[index] = true
	}
}

func TestMapTreeAndTileAssemblyWritesReferenceFiles(t *testing.T) {
	dest := t.TempDir()
	spawn := &mapSpawnRecord{Flags: modelFlagHasBound, ID: 7, BoundsLow: vector3{0, 0, 0}, BoundsHigh: vector3{10, 10, 10}, Name: "Building.wmo"}
	assembly := &mapAssembly{Unique: map[uint32]*mapSpawnRecord{7: spawn}, Tiles: map[uint32][]uint32{packTileID(1, 2): {7}}}
	if err := writeMapFiles(dest, 571, assembly); err != nil {
		t.Fatal(err)
	}
	tree, err := os.ReadFile(filepath.Join(dest, "571.vmtree"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) < 13 || string(tree[:8]) != vMapMagic || tree[8] != 1 || string(tree[9:13]) != "NODE" || !bytes.Contains(tree, []byte("GOBJ")) {
		t.Fatalf("unexpected vmtree header: %x", tree[:min(len(tree), 32)])
	}
	tile, err := os.ReadFile(filepath.Join(dest, "571_01_02.vmtile"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tile) < 12 || string(tile[:8]) != vMapMagic || binary.LittleEndian.Uint32(tile[8:12]) != 1 {
		t.Fatalf("unexpected vmtile header: %x", tile[:min(len(tile), 20)])
	}
}

func TestAssembleMapTreesCalculatesM2BoundsFromRawModel(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	var raw bytes.Buffer
	raw.Write(append([]byte(rawVMapMagic), 0))
	for _, value := range []uint32{0, 1, 42} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []uint32{0, 0} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []float32{0, 0, 0, 1, 1, 1} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}
	raw.WriteString("GRP ")
	if err := binary.Write(&raw, binary.LittleEndian, uint32(8)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []uint32{0} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	raw.WriteString("INDX")
	if err := binary.Write(&raw, binary.LittleEndian, uint32(10)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []uint16{0, 1, 2} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	raw.WriteString("VERT")
	if err := binary.Write(&raw, binary.LittleEndian, uint32(40)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []vector3{{0, 0, 0}, {2, 0, 0}, {0, 3, 0}} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "model.m2"), raw.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	dirFile, err := os.Create(filepath.Join(src, "dir_bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []uint32{1, 2, 3} {
		if err := binary.Write(dirFile, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeModelSpawn(dirFile, mapSpawnRecord{Flags: modelFlagM2, ID: 7, Position: vector3{1, 2, 3}, Scale: 1, Name: "model.m2"}); err != nil {
		t.Fatal(err)
	}
	if err := dirFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := assembleMapTrees(src, dest); err != nil {
		t.Fatal(err)
	}
	tile, err := os.Open(filepath.Join(dest, "001_02_03.vmtile"))
	if err != nil {
		t.Fatal(err)
	}
	defer tile.Close()
	magic := make([]byte, 8)
	if _, err := io.ReadFull(tile, magic); err != nil || string(magic) != vMapMagic {
		t.Fatalf("tile magic=%q err=%v", magic, err)
	}
	var count uint32
	if err := binary.Read(tile, binary.LittleEndian, &count); err != nil || count != 1 {
		t.Fatalf("tile count=%d err=%v", count, err)
	}
	spawn, err := readModelSpawn(tile)
	if err != nil {
		t.Fatal(err)
	}
	if spawn.Flags&modelFlagHasBound == 0 || spawn.BoundsLow != (vector3{1, 2, 3}) || spawn.BoundsHigh != (vector3{3, 5, 3}) {
		t.Fatalf("unexpected calculated M2 bounds: %+v", spawn)
	}
}

func TestPrintBanner(t *testing.T) {
	printBanner()
}

func TestRawVMAPModelConversionWritesCompleteVMO(t *testing.T) {
	var raw bytes.Buffer
	raw.Write(append([]byte(rawVMapMagic), 0))
	for _, value := range []uint32{0, 1, 42, 1, 2} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []float32{-1, -1, -1, 1, 1, 1} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}
	raw.WriteString("GRP ")
	if err := binary.Write(&raw, binary.LittleEndian, uint32(8)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}
	raw.WriteString("INDX")
	if err := binary.Write(&raw, binary.LittleEndian, uint32(10)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []uint16{0, 1, 2} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	raw.WriteString("VERT")
	if err := binary.Write(&raw, binary.LittleEndian, uint32(40)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&raw, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []vector3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}} {
		if err := binary.Write(&raw, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	model, err := readRawModel(bytes.NewReader(raw.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeVMO(&output, model); err != nil {
		t.Fatal(err)
	}
	data := output.Bytes()
	if len(data) <= 64 || string(data[:8]) != vMapMagic || !bytes.Contains(data, []byte("WMOD")) || !bytes.Contains(data, []byte("GMOD")) || !bytes.Contains(data, []byte("MBIH")) || !bytes.Contains(data, []byte("GBIH")) {
		t.Fatalf("invalid assembled output length=%d prefix=%q", len(data), data[:min(len(data), 8)])
	}
	trim := bytes.Index(data, []byte("TRIM"))
	if trim < 0 || binary.LittleEndian.Uint32(data[trim+4:]) != 16 || binary.LittleEndian.Uint32(data[trim+8:]) != 1 || binary.LittleEndian.Uint32(data[trim+12:]) != 0 || binary.LittleEndian.Uint32(data[trim+16:]) != 1 || binary.LittleEndian.Uint32(data[trim+20:]) != 2 {
		t.Fatalf("TRIM did not use reference 32-bit indices: offset=%d", trim)
	}
}
