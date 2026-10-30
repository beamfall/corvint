package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"slices"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/versionline"
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
	info, _ := debug.ReadBuildInfo()
	identity := versionIdentityFrom(version, build, sourceCommit, info)
	payload := map[string]any{
		"profile": "corvint-version/1", "tool": "corvint", "version": version,
		"displayVersion": identity.DisplayVersion, "release": identity.Release,
		"commit": identity.Commit, "commitState": identity.CommitState,
		"commands": commands, "harnessHosts": gokernel.HarnessHosts(),
	}
	if identity.Build != producerUnstamped {
		payload["build"] = identity.Build
	}
	return payload
}

// sourceCommit is the full commit a release builder built this binary from,
// stamped with -ldflags "-X main.sourceCommit=SHA" (PUB-V0-050). Only the
// release builders stamp it; every other build leaves it empty.
var sourceCommit = ""

// versionIdentity is the --version --json source identity (PUB-V0-050..053).
type versionIdentity struct {
	DisplayVersion, Build, Commit, CommitState string
	Release                                    bool
}

// versionIdentityFrom states each member from its own source, as
// producerFrom does (PUB-V0-028). A binary is a release build only when a
// release builder stamped both its build number and its source commit; any
// other build is a pre-release of the compiled VERSION (PUB-V0-052).
func versionIdentityFrom(versionValue, buildValue, stampedCommit string, info *debug.BuildInfo) versionIdentity {
	producer := producerFrom(versionValue, buildValue, info)
	identity := versionIdentity{Build: producer.Build, Commit: producer.Commit, CommitState: producer.CommitState}
	if producer.Build != producerUnstamped && producerRevision.MatchString(stampedCommit) {
		// A release builder compiles the exported tree of exactly this commit.
		identity.Commit, identity.CommitState, identity.Release = stampedCommit, "clean", true
	}
	identity.DisplayVersion = versionValue
	if !identity.Release {
		identity.DisplayVersion = developmentVersion(versionValue)
	}
	return identity
}

// developmentVersion names an unreleased build of VERSION as a pre-release of
// it: 1.0.0 becomes 1.0.0-dev and 1.0.0-rc.3 becomes 1.0.0-rc.3.dev.
func developmentVersion(versionValue string) string {
	return versionline.DevelopmentVersion(versionValue)
}

// versionText is the plain --version line (CRB-V0-003, PUB-V0-053):
// `Corvint DISPLAY (build N, commit SHA[, modified])`, without the build member
// when unstamped and with `commit unknown` when no commit is known.
func versionText(identity versionIdentity) string {
	stamped, commit := identity.Build, identity.Commit
	if stamped == producerUnstamped {
		stamped = ""
	}
	if commit == producerUnknown {
		commit = ""
	}
	return versionline.Format("Corvint", identity.DisplayVersion, stamped, commit, identity.CommitState == "modified")
}

// emitVersion prints the human version line, or with asJSON the
// corvint-version/1 identity.
func emitVersion(stdout, stderr io.Writer, asJSON bool) int {
	if asJSON {
		return emitPayload(stdout, stderr, "version", versionPayload())
	}
	info, _ := debug.ReadBuildInfo()
	_, _ = fmt.Fprintln(stdout, versionText(versionIdentityFrom(version, build, sourceCommit, info)))
	return 0
}
