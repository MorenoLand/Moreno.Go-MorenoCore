// MorenoCore Map & DBC Extractor: extracts client database and map data from MPQs.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/tools/wowdata"
)

func main() {
	input := flag.String("input", "", "WoW client installation root or directory containing MPQ archives")
	output := flag.String("output", "data", "output data directory")
	wdtFile := flag.String("wdt-file", "", "parse one local WDT file and print its active tile count")
	flag.Parse()
	if *wdtFile != "" {
		data, err := os.ReadFile(*wdtFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WDT read failed: %v\n", err)
			os.Exit(1)
		}
		info, err := parseWDT(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WDT parse failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Parsed %s: version=%d active_tiles=%d global_wmo=%t\n", filepath.Base(*wdtFile), info.Version, info.TileCount, info.HasGlobalWMO)
		if *input == "" {
			return
		}
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		os.Exit(2)
	}
	count, err := wowdata.ExtractDBC(*input, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DBC extraction failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Extracted and validated %d DBC files into %s\n", count, *output)
}
