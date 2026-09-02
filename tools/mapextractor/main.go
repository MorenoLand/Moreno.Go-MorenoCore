// Note: Map tile geometry extraction (ADT/liquid) is not implemented; currently performs DBC extraction only.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/tools/wowdata"
)

func main() {
	input := flag.String("input", "", "WoW client installation root or directory containing MPQ archives")
	output := flag.String("output", "data", "output data directory")
	flag.Parse()
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
