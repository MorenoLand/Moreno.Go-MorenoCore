package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocoltrace"
)

func main() {
	expectedPath := flag.String("expected", "", "reference protocol trace JSONL")
	actualPath := flag.String("actual", "", "Go protocol trace JSONL")
	replayPath := flag.String("replay", "", "protocol trace JSONL to replay")
	outputPath := flag.String("output", "", "diff JSON output path; stdout when omitted")
	compareTiming := flag.Bool("compare-timing", false, "compare relative event timing")
	timingTolerance := flag.Duration("timing-tolerance", 0, "allowed relative timing difference")
	flag.Parse()
	if *replayPath != "" {
		if err := replay(*replayPath); err != nil {
			fail(err)
		}
		return
	}
	if *expectedPath == "" || *actualPath == "" {
		fail(fmt.Errorf("-expected and -actual are required unless -replay is used"))
	}
	expected, err := loadFile(*expectedPath)
	if err != nil {
		fail(err)
	}
	actual, err := loadFile(*actualPath)
	if err != nil {
		fail(err)
	}
	differences, err := protocoltrace.Diff(expected, actual, protocoltrace.CompareOptions{CompareTiming: *compareTiming, TimingTolerance: *timingTolerance})
	if err != nil {
		fail(err)
	}
	result := struct {
		Equal       bool                       `json:"equal"`
		Differences []protocoltrace.Difference `json:"differences"`
	}{Equal: len(differences) == 0, Differences: differences}
	var data []byte
	data, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if *outputPath == "" {
		_, _ = os.Stdout.Write(data)
	} else if err := os.WriteFile(*outputPath, data, 0o644); err != nil {
		fail(err)
	}
	if !result.Equal {
		os.Exit(1)
	}
}

func loadFile(path string) (protocoltrace.Trace, error) {
	file, err := os.Open(path)
	if err != nil {
		return protocoltrace.Trace{}, err
	}
	defer file.Close()
	return protocoltrace.Load(file)
}

func replay(path string) error {
	trace, err := loadFile(path)
	if err != nil {
		return err
	}
	return trace.Replay(replayPrinter{})
}

type replayPrinter struct{}

func (replayPrinter) Handle(direction protocoltrace.Direction, opcode uint32, payload []byte, state string) error {
	fmt.Printf("%s opcode=%d payload=%x state=%s\n", direction, opcode, payload, state)
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
