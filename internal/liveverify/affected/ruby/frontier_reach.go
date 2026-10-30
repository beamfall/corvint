// SPDX-License-Identifier: Apache-2.0
package ruby

import (
	"path"
	"strings"
)

// SourceBoundFrontiers names the reasons whose hidden edge the plugin's own
// load resolution binds only to a Ruby file (AFP-V0-511). A relative,
// conditional, ambiguous or `$LOAD_PATH`-dependent load and a Rails autoload
// each name a feature the plugin resolves against its `.rb` unit paths or not
// at all; none can bind a path outside Owns. An implicit test dependency, a
// dynamic test definition, a Cucumber feature's ownership and an unreadable
// source file may each bind any change, so they stay path-open.
func (Language) SourceBoundFrontiers() []string {
	return []string{FrontierRelativeLoad, FrontierRailsAutoload, FrontierDynamicLoad, FrontierConditionalLoad, FrontierAmbiguousLoad}
}

// FixtureClosedFrontiers names the path-open reason whose hidden edge is a
// test runner (AFP-V0-511): an ambiguous framework binds the runner that
// executes a test file, resolved from its own and ancestor directories, never a
// file below a `testdata` directory in which the plugin owns no path.
func (Language) FixtureClosedFrontiers() []string {
	return []string{FrontierAmbiguousFramework}
}

// ResolutionInput reports the files the plugin scans without owning them
// (AFP-V0-511): a `Rakefile`, whose `$LOAD_PATH` additions steer load
// resolution, and a Cucumber `.feature` file, whose steps bind definitions.
func (Language) ResolutionInput(relative string) bool {
	base := path.Base(relative)
	return base == "Rakefile" || strings.HasSuffix(base, ".feature")
}
