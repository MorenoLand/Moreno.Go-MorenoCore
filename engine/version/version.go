package version

import (
	"runtime/debug"
	"strings"
)

const Product = "MorenoCore"

var Base = "0.0.0-dev"
var Commit = "unknown"
var Modified = "false"

func String() string {
	commit, modified := buildRevision()
	if commit == "" || commit == "unknown" {
		return Base + "+gunknown"
	}
	short := commit
	if len(short) > 12 {
		short = short[:12]
	}
	suffix := "+g" + short
	if modified {
		suffix += ".dirty"
	}
	return Base + suffix
}

func Revision() string {
	commit, _ := buildRevision()
	if commit == "" {
		return "unknown"
	}
	return commit
}

func buildRevision() (string, bool) {
	commit := Commit
	modified := strings.EqualFold(Modified, "true")
	if commit != "unknown" && commit != "" {
		return commit, modified
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return commit, modified
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit = setting.Value
		case "vcs.modified":
			modified = strings.EqualFold(setting.Value, "true")
		}
	}
	return commit, modified
}
