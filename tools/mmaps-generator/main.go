// MorenoCore MoveMap Generator: compiles navigation mesh tiles from map and vmap data.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const moveMapGridSize float32 = 533.3333

type mapTile struct {
	MapID uint32
	TileX uint32
	TileY uint32
	Path  string
}

func discoverMapTiles(dir string, targetMap int) (map[uint32][]mapTile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	groups := make(map[uint32][]mapTile)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".map") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(data) < 12 {
			return nil, fmt.Errorf("map tile %s is shorter than the reference 12-byte header", path)
		}
		tile := mapTile{MapID: binary.LittleEndian.Uint32(data[0:4]), TileX: binary.LittleEndian.Uint32(data[4:8]), TileY: binary.LittleEndian.Uint32(data[8:12]), Path: path}
		if targetMap >= 0 && tile.MapID != uint32(targetMap) {
			continue
		}
		groups[tile.MapID] = append(groups[tile.MapID], tile)
	}
	return groups, nil
}

func buildNavMeshHeader(tiles []mapTile) []byte {
	var maxX, maxY uint32
	for _, tile := range tiles {
		if tile.TileX > maxX {
			maxX = tile.TileX
		}
		if tile.TileY > maxY {
			maxY = tile.TileY
		}
	}
	originX := float32(31-int32(maxX)) * moveMapGridSize
	originZ := float32(31-int32(maxY)) * moveMapGridSize
	header := make([]byte, 28)
	binary.LittleEndian.PutUint32(header[0:4], math.Float32bits(originX))
	binary.LittleEndian.PutUint32(header[4:8], math.Float32bits(math.SmallestNonzeroFloat32))
	binary.LittleEndian.PutUint32(header[8:12], math.Float32bits(originZ))
	binary.LittleEndian.PutUint32(header[12:16], math.Float32bits(moveMapGridSize))
	binary.LittleEndian.PutUint32(header[16:20], math.Float32bits(moveMapGridSize))
	binary.LittleEndian.PutUint32(header[20:24], uint32(len(tiles)))
	binary.LittleEndian.PutUint32(header[24:28], 1<<22)
	return header
}

func writeNavMeshHeaders(output string, groups map[uint32][]mapTile) (int, error) {
	mapIDs := make([]uint32, 0, len(groups))
	for mapID := range groups {
		mapIDs = append(mapIDs, mapID)
	}
	sort.Slice(mapIDs, func(i, j int) bool { return mapIDs[i] < mapIDs[j] })
	for _, mapID := range mapIDs {
		path := filepath.Join(output, fmt.Sprintf("%03d.mmap", mapID))
		if err := os.WriteFile(path, buildNavMeshHeader(groups[mapID]), 0o644); err != nil {
			return 0, err
		}
	}
	return len(mapIDs), nil
}

func printBanner() {
	fmt.Println("==========================================================")
	fmt.Println(" MorenoCore MoveMap Generator (Go Parity Version)")
	fmt.Println(" Builds navigation mesh tiles from map and vmap data")
	fmt.Println("==========================================================")
}

func main() {
	mapsDir := flag.String("maps", "maps", "Path to extracted map data directory")
	vmapsDir := flag.String("vmaps", "vmaps", "Path to compiled vmap data directory")
	outputDir := flag.String("output", "mmaps", "Output directory for compiled navigation mesh tiles")
	threads := flag.Int("threads", runtime.NumCPU(), "Number of concurrent generator worker threads")
	targetMap := flag.Int("map", -1, "Target map ID (-1 for all maps)")
	skipLiquid := flag.Bool("skipLiquid", false, "Skip liquid geometry calculation")
	flag.Parse()

	printBanner()

	fmt.Printf("Settings: maps='%s', vmaps='%s', output='%s', threads=%d, skipLiquid=%v\n",
		*mapsDir, *vmapsDir, *outputDir, *threads, *skipLiquid)

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory '%s': %v\n", *outputDir, err)
		os.Exit(1)
	}

	start := time.Now()

	startMaps, err := discoverMapTiles(*mapsDir, *targetMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to inspect map geometry files: %v\n", err)
		os.Exit(1)
	}
	mapsFound := 0
	for _, tiles := range startMaps {
		mapsFound += len(tiles)
	}

	fmt.Printf("Located %d map geometry files in '%s'\n", mapsFound, *mapsDir)
	if *targetMap >= 0 {
		fmt.Printf("Generating MoveMap tiles for map %d...\n", *targetMap)
	} else {
		fmt.Println("Generating MoveMap tiles across all maps...")
	}

	if mapsFound == 0 {
		fmt.Println("No reference .map geometry files were found; no output was fabricated.")
		return
	}
	headers, err := writeNavMeshHeaders(*outputDir, startMaps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write navigation headers: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d reference-format navigation headers; tile geometry generation is not implemented and no .mmtile files were fabricated.\n", headers)
	os.Exit(1)

	elapsed := time.Since(start)
	fmt.Printf("MoveMap tile generation finished in %v. Output directory: '%s'\n", elapsed.Round(time.Millisecond), *outputDir)
}
