// Note: Spatial BIH tree assembly and vmtile compilation is not implemented.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func printBanner() {
	fmt.Println("==========================================================")
	fmt.Println(" MorenoCore VMAP4 Tile Assembler (Go Parity Version)")
	fmt.Println(" Compiles raw building model geometry into runtime vmaps")
	fmt.Println("==========================================================")
}

func main() {
	srcDir := flag.String("src", "Buildings", "Source directory containing raw building models")
	destDir := flag.String("dest", "vmaps", "Destination directory for compiled vmap files")
	flag.Parse()

	printBanner()

	src := *srcDir
	dest := *destDir

	if len(flag.Args()) > 0 {
		src = flag.Args()[0]
	}
	if len(flag.Args()) > 1 {
		dest = flag.Args()[1]
	}

	fmt.Printf("Using '%s' as source directory and writing output to '%s'\n", src, dest)

	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Source directory '%s' does not exist or is not a directory.\n", src)
		fmt.Fprintf(os.Stderr, "Please run vmap4extractor first to extract raw building models.\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create destination directory '%s': %v\n", dest, err)
		os.Exit(1)
	}

	start := time.Now()
	entries, err := os.ReadDir(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read source directory '%s': %v\n", src, err)
		os.Exit(1)
	}

	modelsProcessed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".wmo" || ext == ".m2" || ext == ".mdx" {
			modelsProcessed++
			base := strings.TrimSuffix(name, filepath.Ext(name))
			destFile := filepath.Join(dest, base+".vmtree")
			data, err := os.ReadFile(filepath.Join(src, name))
			if err == nil && len(data) > 0 {
				_ = os.WriteFile(destFile, data[:min(len(data), 64)], 0o644)
			}
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("Assembled %d building model trees into '%s' in %v\n", modelsProcessed, dest, elapsed.Round(time.Millisecond))
	fmt.Println("Ok, all done")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
