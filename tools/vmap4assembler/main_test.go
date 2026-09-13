package main

import (
	"bytes"
	"encoding/binary"
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
