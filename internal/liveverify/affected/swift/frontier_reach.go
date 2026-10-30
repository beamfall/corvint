// SPDX-License-Identifier: Apache-2.0
package swift

import "path"

// SourceBoundFrontiers names the reason whose hidden edge binds only a Swift
// source file (AFP-V0-511): a conditional compilation block hides imports and
// declarations the plugin resolves only between the `.swift` paths its targets
// list. A source file mapped to no target binds a manifest or project file the
// plugin does not own, so it stays path-open.
func (Language) SourceBoundFrontiers() []string {
	return []string{FrontierConditionalCompilation}
}

// FixtureClosedFrontiers names the path-open reasons whose hidden edge is the
// test runner (AFP-V0-511): dynamic test discovery and an unresolved test
// framework bind which tests a target's runner executes, never a file below a
// `testdata` directory in which the plugin owns no path.
func (Language) FixtureClosedFrontiers() []string {
	return []string{FrontierDynamicTestDiscovery, FrontierFrameworkUnresolved}
}

// ResolutionInput reports the files the plugin scans without owning them
// (AFP-V0-511): the package manifest, Xcode project and scheme files, YAML and
// the Appium manifests metadataFile admits, which map sources to targets and
// name test frameworks.
func (Language) ResolutionInput(relative string) bool {
	return metadataFile(path.Base(relative))
}
