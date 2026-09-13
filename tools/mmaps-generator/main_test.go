package main

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestPrintBanner(t *testing.T) {
	printBanner()
}

func TestDiscoverMapTilesAndHeader(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, mapFileHeaderSize)
	copy(data[:4], mapMagic)
	copy(data[4:8], mapVersionMagic)
	path := filepath.Join(dir, "0013102.map")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	groups, err := discoverMapTiles(dir, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[1]) != 1 {
		t.Fatalf("groups=%v", groups)
	}
	if groups[1][0].TileX != 2 || groups[1][0].TileY != 31 {
		t.Fatalf("tile coordinates=%+v", groups[1][0])
	}
	header := buildNavMeshHeader(groups[1])
	if len(header) != 28 {
		t.Fatalf("header length=%d", len(header))
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(header[0:4])); got != float32(31-2)*moveMapGridSize {
		t.Fatalf("origin x=%f", got)
	}
	if got := binary.LittleEndian.Uint32(header[20:24]); got != 1 {
		t.Fatalf("max tiles=%d", got)
	}
	if got := binary.LittleEndian.Uint32(header[24:28]); got != 1<<22 {
		t.Fatalf("max polys=%d", got)
	}
}
