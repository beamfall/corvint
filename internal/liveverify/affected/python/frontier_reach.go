// SPDX-License-Identifier: Apache-2.0
package python

import (
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// SourceBoundFrontiers names the reasons whose hidden edge the plugin's own
// import resolution binds only to a Python module (AFP-V0-511). A conditional,
// dynamic or unresolved relative import and an unresolved `pytest_plugins`
// entry each name a module, which the plugin resolves to a `.py` path it owns
// or to nothing; none can bind a path outside Owns. An unresolved
// in-repository absolute import (SLF-V1-002) names a module too. An
// unreadable source file hides every edge it may hold, so it stays path-open.
func (Language) SourceBoundFrontiers() []string {
	return []string{FrontierConditionalImport, FrontierDynamicImport, FrontierRelativeImport, FrontierPytestPlugins, FrontierAbsoluteImport}
}

// FrontierDirection declares FrontierAbsoluteImport OUTBOUND (SLF-V1-002):
// its hidden edge starts at the import in the site's own file, so a bearing
// entry selects the importing unit with its dependents. Every other Python
// reason keeps the INBOUND default and is named whenever it bears.
func (Language) FrontierDirection(reason string) string {
	if reason == FrontierAbsoluteImport {
		return affected.FrontierOutbound
	}
	return affected.FrontierInbound
}

// CoverageUnproven reports that no changed Python module can be proven
// untested while an unresolved in-repository absolute import is open: that
// import may be its consumer, so the module is unknown NO_SELECTABLE_TEST,
// never Uncovered (SLF-V1-002).
func (Language) CoverageUnproven(_ string, frontier []string) bool {
	return slices.ContainsFunc(frontier, func(entry string) bool {
		return entry == FrontierAbsoluteImport || strings.HasPrefix(entry, FrontierAbsoluteImport+":")
	})
}

// FixtureClosedFrontiers declares no fixture-closed reason: the plugin's only
// path-open reason is an unreadable source file (AFP-V0-511).
func (Language) FixtureClosedFrontiers() []string { return nil }

// ResolutionInput reports no path while SLF-V1-001 is held: the plugin then
// scans only the `.py` files it owns, so no unowned path steers its
// resolution (AFP-V0-511). With that rule on, the pytest configuration files
// it reads for import roots are resolution inputs.
func (Language) ResolutionInput(relative string) bool {
	return importRootsNarrowing && slices.Contains(pytestConfigNames, path.Base(relative))
}
