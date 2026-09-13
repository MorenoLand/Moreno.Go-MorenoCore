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
	adtFile := flag.String("adt-file", "", "parse one local ADT file and print its chunk inventory")
	flag.Parse()
	if *adtFile != "" {
		data, err := os.ReadFile(*adtFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ADT read failed: %v\n", err)
			os.Exit(1)
		}
		info, err := parseADT(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ADT parse failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Parsed %s: mcnk=%d mh2o=%d liquid_layers=%d mcvt=%d mcly=%d mcal=%d mddf=%d modf=%d\n", filepath.Base(*adtFile), info.MCNKCount, info.MH2OCount, info.LiquidLayers, info.MCVTCount, info.MCLYCount, info.MCALCount, len(info.Doodads), len(info.WorldModels))
		if *input == "" && *wdtFile == "" {
			return
		}
	}
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
		fmt.Printf("Parsed %s: version=%d active_tiles=%d global_wmo=%t name=%s\n", filepath.Base(*wdtFile), info.Version, info.TileCount, info.HasGlobalWMO, info.GlobalWMO)
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
