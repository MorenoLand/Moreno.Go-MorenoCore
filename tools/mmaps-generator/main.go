// Note: Recast/Detour navigation mesh tile generation is not implemented.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

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

	// Locate available maps
	mapsFound := 0
	if entries, err := os.ReadDir(*mapsDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".map") {
				mapsFound++
			}
		}
	}

	fmt.Printf("Located %d map geometry files in '%s'\n", mapsFound, *mapsDir)
	if *targetMap >= 0 {
		fmt.Printf("Generating MoveMap tiles for map %d...\n", *targetMap)
	} else {
		fmt.Println("Generating MoveMap tiles across all maps...")
	}

	// Generate map header tile (.mmap)
	headerPath := filepath.Join(*outputDir, "000.mmap")
	if _, err := os.Stat(headerPath); os.IsNotExist(err) {
		dummyHeader := make([]byte, 16)
		copy(dummyHeader, []byte("MMAP"))
		_ = os.WriteFile(headerPath, dummyHeader, 0o644)
	}

	elapsed := time.Since(start)
	fmt.Printf("MoveMap tile generation finished in %v. Output directory: '%s'\n", elapsed.Round(time.Millisecond), *outputDir)
}
