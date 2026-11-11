// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/Beamfall/corvint/internal/companioncli"

// build is the first-parent commit count a release builder stamps with
// -ldflags "-X main.build=N" (PUB-V0-021, ARTIFACT-BIN-V0-002); an unstamped
// build reports no build member. sourceCommit is the commit it built from,
// stamped with "-X main.sourceCommit=SHA" (PUB-V0-052). Only the release
// builders stamp them; every other build leaves the defaults, and --version
// then prints the development version as before (ARTIFACT-BIN-V0-508).
var (
	build        = "0"
	sourceCommit = ""
)

// versionSpec is this executable's companion version identity (CAP-V0-700),
// the line the beside check compares the core against (CMS-V0-002).
func versionSpec() companioncli.Spec {
	return companioncli.Spec{Binary: binaryName, Version: companioncli.ExperimentalVersion, Build: build, Commit: sourceCommit}
}

// mcpServerVersion is the serverInfo.version `corvint-docs mcp` reports: the
// release line and this build's stamp, so a client can tell builds apart.
func mcpServerVersion() string {
	commit := sourceCommit
	if commit == "" {
		commit = "unknown"
	}
	return companioncli.ExperimentalVersion + " (build " + build + ", commit " + commit + ")"
}
