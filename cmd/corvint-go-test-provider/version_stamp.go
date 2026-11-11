// SPDX-License-Identifier: Apache-2.0

package main

// build is the first-parent commit count a release builder stamps with
// -ldflags "-X main.build=N" (PUB-V0-021, ARTIFACT-BIN-V0-002); an unstamped
// build reports no build member. sourceCommit is the commit it built from,
// stamped with "-X main.sourceCommit=SHA" (PUB-V0-052). Only the release
// builders stamp them; every other build leaves the defaults, and --version
// then prints the development version as before (ARTIFACT-BIN-V0-508).
//
// version is the release version the per-binary release builder names with
// "-X main.version=X.Y.Z": this binary is released on its own, outside the core
// version set, so its version is stamped rather than compiled from VERSION
// (ARTIFACT-BIN-V0-002). An unstamped build reports the development line.
var (
	build        = "0"
	sourceCommit = ""
	version      = ""
)
