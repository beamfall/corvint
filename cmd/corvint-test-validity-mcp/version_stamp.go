// SPDX-License-Identifier: Apache-2.0

package main

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
