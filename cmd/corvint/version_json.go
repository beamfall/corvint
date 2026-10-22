package main

import (
	"fmt"
	"io"
	"slices"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// isVersionRequest reports `--version` or `--version --json` as the whole
// remaining argument list.
func isVersionRequest(arguments []string) bool {
	return len(arguments) > 0 && arguments[0] == "--version" && (len(arguments) == 1 || len(arguments) == 2 && arguments[1] == "--json")
}

// versionPayload is the machine-readable identity a plugin or script reads to
// detect binary skew before it calls a verb or names a host (PRS-V1-020,
// V1-0300): the release version, the build, every top-level verb this binary
// dispatches and every harness host its adapter schema admits.
func versionPayload() map[string]any {
	commands := slices.Clone(topLevelCommands)
	slices.Sort(commands)
	return map[string]any{
		"profile": "corvint-version/1", "tool": "corvint", "version": version, "build": build,
		"commands": commands, "harnessHosts": gokernel.HarnessHosts(),
	}
}

// emitVersion prints the human version line, or with asJSON the
// corvint-version/1 identity.
func emitVersion(stdout, stderr io.Writer, asJSON bool) int {
	if asJSON {
		return emitPayload(stdout, stderr, "version", versionPayload())
	}
	_, _ = fmt.Fprintln(stdout, "Corvint "+version+" (build "+build+")")
	return 0
}
