package wowdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/dbc"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/tools/mpq"
)

func ExtractDBC(input, output string) (int, error) {
	archives, err := mpq.Archives(input)
	if err != nil {
		return 0, err
	}
	if len(archives) == 0 {
		return 0, fmt.Errorf("no MPQ archives found under %s", input)
	}
	destination := filepath.Join(output, "dbc")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return 0, err
	}
	written := make(map[string]struct{})
	for _, path := range archives {
		archive, err := mpq.Open(path)
		if err != nil {
			return len(written), err
		}
		files, listErr := archive.ListFiles()
		if listErr != nil {
			archive.Close()
			continue
		}
		for _, name := range files {
			if !strings.EqualFold(filepath.Ext(name), ".dbc") {
				continue
			}
			data, err := archive.ReadFile(name)
			if err != nil {
				archive.Close()
				return len(written), fmt.Errorf("read %s from %s: %w", name, path, err)
			}
			if _, err := dbc.Parse(data); err != nil {
				archive.Close()
				return len(written), fmt.Errorf("invalid DBC %s: %w", name, err)
			}
			base := filepath.Base(filepath.FromSlash(name))
			if err := os.WriteFile(filepath.Join(destination, base), data, 0o644); err != nil {
				archive.Close()
				return len(written), err
			}
			written[strings.ToLower(base)] = struct{}{}
		}
		if err := archive.Close(); err != nil {
			return len(written), err
		}
	}
	return len(written), nil
}
