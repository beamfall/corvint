// Package companioncli answers the shared command-line contract of the
// companion binaries a release ships beside corvint (CAP-V0-700): usage on
// --help, -h and help, a machine-readable help --json, and --version with an
// optional --json, each on stdout with exit 0 and without touching a
// repository, a port or a network.
package companioncli

import (
	"encoding/json"
	"io"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/Beamfall/corvint/internal/versionline"
)

const (
	// HelpProfile and VersionProfile name the two JSON documents.
	HelpProfile    = "corvint-companion-help/0"
	VersionProfile = "corvint-companion-version/0"
	// ExperimentalVersion is the common development release line of every
	// companion; delivery maturity remains separate from the release version.
	ExperimentalVersion = versionline.Development
)

// Spec names one companion: its binary name, version and usage text. Build
// and Commit are the release builder's `-X main.build=N` and
// `-X main.sourceCommit=SHA` stamps the binary's main package passes through
// (ARTIFACT-BIN-V0-002, PUB-V0-052); an unstamped build leaves them empty.
type Spec struct {
	Binary, Version, Usage string
	Build, Commit          string
}

var stampedCommit = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// Stamped reports whether a release builder stamped both the build number
// and the source commit, the same rule corvint applies (PUB-V0-052).
func (spec Spec) Stamped() bool {
	return spec.Build != "" && spec.Build != "0" && stampedCommit.MatchString(spec.Commit)
}

// DisplayVersion is the version the binary reports: the compiled release line
// for a stamped release build, otherwise the development form it was given.
func (spec Spec) DisplayVersion() string {
	if spec.Stamped() && spec.Version == versionline.Development {
		return versionline.Release
	}
	return spec.Version
}

// Line is the parsed form of the plain --version line (PUB-V0-053).
func (spec Spec) Line() versionline.Line {
	line := versionline.Line{Name: spec.Binary, Version: spec.DisplayVersion()}
	if spec.Build != "" && spec.Build != "0" {
		line.Build = spec.Build
	}
	if stampedCommit.MatchString(spec.Commit) {
		line.Commit = spec.Commit
	}
	return line
}

// Text is the plain --version line: `BINARY VERSION` for an unstamped build,
// as before, and `BINARY VERSION (build N, commit SHA)` once a release builder
// stamped the binary, so an installed set can be checked for one version
// (CMS-V0-002).
func (spec Spec) Text() string {
	line := spec.Line()
	if line.Build == "" && line.Commit == "" {
		return spec.Binary + " " + line.Version
	}
	return versionline.Format(spec.Binary, line.Version, line.Build, line.Commit, false)
}

// Answer handles the contract forms of arguments and returns the exit code
// and true, or false for any other argument list, which the binary parses as
// before. help --json lists the usage lines; --version prints Text.
func Answer(spec Spec, arguments []string, stdout io.Writer) (int, bool) {
	switch strings.Join(arguments, " ") {
	case "--help", "-h", "help":
		_, _ = io.WriteString(stdout, strings.TrimRight(spec.Usage, "\n")+"\n")
	case "help --json":
		writeJSON(stdout, map[string]any{
			"profile": HelpProfile, "binary": spec.Binary, "version": spec.DisplayVersion(),
			"usage": strings.Split(strings.TrimRight(spec.Usage, "\n"), "\n"),
		})
	case "--version":
		_, _ = io.WriteString(stdout, spec.Text()+"\n")
	case "--version --json":
		commit, state := buildCommit()
		payload := map[string]any{
			"profile": VersionProfile, "binary": spec.Binary, "version": spec.DisplayVersion(),
			"commit": commit, "commitState": state, "release": spec.Stamped(),
		}
		if line := spec.Line(); line.Build != "" {
			payload["build"] = line.Build
		}
		if spec.Stamped() {
			// A release builder compiles the exported tree of exactly the stamped commit (PUB-V0-050).
			payload["commit"], payload["commitState"] = spec.Commit, "clean"
		}
		writeJSON(stdout, payload)
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
