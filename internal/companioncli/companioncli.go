// Package companioncli answers the shared command-line contract of the
// companion binaries a release ships beside corvint (CAP-V0-700): usage on
// --help, -h and help, a machine-readable help --json, and --version with an
// optional --json, each on stdout with exit 0 and without touching a
// repository, a port or a network.
package companioncli

import (
	"encoding/json"
	"io"
	"runtime/debug"
	"strings"
)

const (
	// HelpProfile and VersionProfile name the two JSON documents.
	HelpProfile    = "corvint-companion-help/0"
	VersionProfile = "corvint-companion-version/0"
	// ExperimentalVersion is the version every companion reports until it
	// ships its own release line.
	ExperimentalVersion = "0.1.0-experimental"
)

// Spec names one companion: its binary name, version and usage text.
type Spec struct {
	Binary, Version, Usage string
}

// Answer handles the contract forms of arguments and returns the exit code
// and true, or false for any other argument list, which the binary parses as
// before. help --json lists the usage lines; --version prints
// `BINARY VERSION`.
func Answer(spec Spec, arguments []string, stdout io.Writer) (int, bool) {
	switch strings.Join(arguments, " ") {
	case "--help", "-h", "help":
		_, _ = io.WriteString(stdout, strings.TrimRight(spec.Usage, "\n")+"\n")
	case "help --json":
		writeJSON(stdout, map[string]any{
			"profile": HelpProfile, "binary": spec.Binary, "version": spec.Version,
			"usage": strings.Split(strings.TrimRight(spec.Usage, "\n"), "\n"),
		})
	case "--version":
		_, _ = io.WriteString(stdout, spec.Binary+" "+spec.Version+"\n")
	case "--version --json":
		commit, state := buildCommit()
		writeJSON(stdout, map[string]any{
			"profile": VersionProfile, "binary": spec.Binary, "version": spec.Version,
			"commit": commit, "commitState": state,
		})
	default:
		return 0, false
	}
	return 0, true
}

func writeJSON(stdout io.Writer, value map[string]any) {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

// buildCommit is the VCS revision Go stamped into the binary, or unknown for a
// build without VCS information (a release build passes -buildvcs=false).
func buildCommit() (commit, state string) {
	commit, state = "unknown", "unknown"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return commit, state
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit = setting.Value
		case "vcs.modified":
			state = map[string]string{"true": "modified", "false": "clean"}[setting.Value]
		}
	}
	if commit == "unknown" {
		state = "unknown"
	}
	return commit, state
}
