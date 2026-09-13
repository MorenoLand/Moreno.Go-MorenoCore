package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestWMOConversionWritesVMAPRawGeometry(t *testing.T) {
	rootPayload := make([]byte, 64)
	binary.LittleEndian.PutUint32(rootPayload[4:], 1)
	binary.LittleEndian.PutUint32(rootPayload[32:], 123)
	mods := make([]byte, 32)
	copy(mods, []byte("MainSet"))
	binary.LittleEndian.PutUint32(mods[20:], 0)
	binary.LittleEndian.PutUint32(mods[24:], 1)

	modd := make([]byte, 40)
	binary.LittleEndian.PutUint32(modd, 0)
	for index, value := range []float32{1, 2, 3} {
		binary.LittleEndian.PutUint32(modd[4+index*4:], mathFloat32Bits(value))
	}
	binary.LittleEndian.PutUint32(modd[28:], mathFloat32Bits(1))
	binary.LittleEndian.PutUint32(modd[32:], mathFloat32Bits(1.5))
	rootData := bytes.Join([][]byte{wmoTestChunk("MOHD", rootPayload), wmoTestChunk("MODN", []byte("World\\Models\\Tree.m2\x00")), wmoTestChunk("MODS", mods), wmoTestChunk("MODD", modd)}, nil)
	root, err := parseWMORoot(rootData)
	if err != nil || root.Groups != 1 || root.ID != 123 || root.DoodadNames[0] != "World\\Models\\Tree.m2" || len(root.DoodadSets) != 1 || root.DoodadSets[0].StartIndex != 0 || root.DoodadSets[0].Count != 1 || len(root.Doodads) != 1 || root.Doodads[0].Scale != 1.5 {
		t.Fatalf("root=%+v err=%v", root, err)
	}
	groupPayload := make([]byte, 68)
	binary.LittleEndian.PutUint32(groupPayload[8:], 1)
	binary.LittleEndian.PutUint32(groupPayload[56:], 42)
	groupData := bytes.Join([][]byte{
		wmoTestChunk("MOGP", groupPayload),
		wmoTestChunk("MOPY", []byte{0x08, 0}),
		wmoTestChunk("MOVI", func() []byte {
			b := make([]byte, 6)
			binary.LittleEndian.PutUint16(b[0:], 0)
			binary.LittleEndian.PutUint16(b[2:], 1)
			binary.LittleEndian.PutUint16(b[4:], 2)
			return b
		}()),
		wmoTestChunk("MOVT", func() []byte {
			b := make([]byte, 36)
			for i, v := range []float32{0, 0, 0, 1, 0, 0, 0, 1, 0} {
				binary.LittleEndian.PutUint32(b[i*4:], mathFloat32Bits(v))
			}
			return b
		}()),
		wmoTestChunk("MOBA", func() []byte { b := make([]byte, 24); binary.LittleEndian.PutUint16(b[16:], 7); return b }()),
		wmoTestChunk("MODR", []byte{0, 0}),
	}, nil)
	group, err := parseWMOGroup(groupData)
	if err != nil || len(group.Indices) != 3 || len(group.Vertices) != 3 || len(group.Branches) != 1 || group.Branches[0] != 7 || len(group.DoodadRefs) != 1 || group.DoodadRefs[0] != 0 {
		t.Fatalf("group=%+v err=%v", group, err)
	}
	raw, err := writeRawWMO(root, []wmoGroupInfo{group})
	if err != nil || len(raw) < 24 || string(raw[:8]) != "VMAP047\x00" || binary.LittleEndian.Uint32(raw[12:16]) != 1 || binary.LittleEndian.Uint32(raw[16:20]) != 123 || !bytes.Contains(raw, []byte("DODM")) {
		t.Fatalf("raw len=%d err=%v header=%x", len(raw), err, raw[:20])
	}
}

func TestWMOGroupNameDetection(t *testing.T) {
	if !isWMOGroupName("World\\Buildings\\Town_007.wmo") || isWMOGroupName("World\\Buildings\\Town.wmo") || isWMOGroupName("Town_07.wmo") {
		t.Fatal("unexpected WMO group-name classification")
	}
}

func TestM2ConversionWritesVMAPRawGeometry(t *testing.T) {
	data := make([]byte, 240+6+36)
	copy(data, []byte("MD20"))
	binary.LittleEndian.PutUint32(data[216:], 3)
	binary.LittleEndian.PutUint32(data[220:], 240)
	binary.LittleEndian.PutUint32(data[224:], 3)
	binary.LittleEndian.PutUint32(data[228:], 246)
	for index, value := range []uint16{0, 1, 2} {
		binary.LittleEndian.PutUint16(data[240+index*2:], value)
	}
	for index, value := range []float32{0, 0, 0, 1, 0, 0, 0, 1, 0} {
		binary.LittleEndian.PutUint32(data[246+index*4:], mathFloat32Bits(value))
	}
	raw, err := extractM2(data)
	if err != nil || len(raw) < 56 || string(raw[:8]) != "VMAP047\x00" || binary.LittleEndian.Uint32(raw[8:12]) != 3 {
		t.Fatalf("raw len=%d err=%v header=%x", len(raw), err, raw[:minRaw(len(raw), 12)])
	}
}

func minRaw(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func wmoTestChunk(name string, payload []byte) []byte {
	result := make([]byte, 8+len(payload))
	copy(result, name)
	binary.LittleEndian.PutUint32(result[4:], uint32(len(payload)))
	copy(result[8:], payload)
	return result
}

func mathFloat32Bits(value float32) uint32 {
	return math.Float32bits(value)
}

func TestModelExtensions(t *testing.T) {
	valid := []string{"test.wmo", "creature.m2", "mesh.mdx", "world.wdt"}
	for _, f := range valid {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".wmo" && ext != ".m2" && ext != ".mdx" && ext != ".wdt" {
			t.Errorf("expected valid model extension for %s", f)
		}
	}

	invalid := []string{"sound.wav", "texture.blp", "db.dbc"}
	for _, f := range invalid {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".wmo" || ext == ".m2" || ext == ".mdx" || ext == ".wdt" {
			t.Errorf("expected invalid model extension for %s", f)
		}
	}
}

func TestPrintBanner(t *testing.T) {
	// Verify printBanner executes without panic
	printBanner()
}
