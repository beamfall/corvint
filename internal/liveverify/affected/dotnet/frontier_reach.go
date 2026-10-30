// SPDX-License-Identifier: Apache-2.0
package dotnet

import (
	"path"
	"strings"
)

// SourceBoundFrontiers names the reason whose hidden edge binds only a C#
// source file (AFP-V0-511): an unparsed `.cs` file hides test attributes and
// type references the plugin resolves only between the `.cs` paths it owns; it
// records no path tokens. A source file assigned to no project binds a
// `.csproj` the plugin does not own, so it stays path-open.
func (Language) SourceBoundFrontiers() []string {
	return []string{FrontierUnparsedSource}
}

// FixtureClosedFrontiers names the path-open reasons whose hidden edge is the
// test runner (AFP-V0-511): dynamic test discovery and an unresolved VSTest
// filter bind the set of tests `dotnet test` executes for a project, never a
// file below a `testdata` directory in which the plugin owns no path.
func (Language) FixtureClosedFrontiers() []string {
	return []string{FrontierDynamicDiscovery, FrontierRunner}
}

// ResolutionInput reports the files the plugin scans without owning them
// (AFP-V0-511): project files, `Directory.Build.props` and `.targets`, which
// assign sources to projects and name test runners, and `.feature` files.
func (Language) ResolutionInput(relative string) bool {
	lower := strings.ToLower(path.Base(relative))
	return strings.HasSuffix(lower, ".csproj") || strings.HasSuffix(lower, ".fsproj") ||
		strings.HasSuffix(lower, ".vbproj") || lower == "directory.build.props" ||
		lower == "directory.build.targets" || strings.HasSuffix(lower, ".feature")
}
