// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// SourceBoundFrontiers names the reasons whose hidden edge the plugin's own
// import resolution binds only to a path it owns (AFP-V0-511). resolveImport
// matches a relative specifier or a configured path alias against unit paths
// with a source extension, so an unresolved one names a missing or renamed
// source file, never a path outside Owns. A dynamic `import(expr)` or
// `require(expr)` can load JSON or any other file, so it stays path-open, as do
// the runtime, config, scan and parse frontiers.
func (Language) SourceBoundFrontiers() []string {
	return []string{FrontierUnresolvedImport, FrontierPathAlias, FrontierBareImportUndeclared}
}

// FixtureClosedFrontiers names the path-open reasons whose hidden edge is a
// test runner (AFP-V0-511): the runner of a test file is resolved from the
// configuration files in its own and ancestor directories, so an unknown or
// ambiguous runner cannot bind a file below a `testdata` directory in which the
// plugin owns no path and raises no site.
func (Language) FixtureClosedFrontiers() []string {
	return []string{FrontierUnknownRunner, FrontierAmbiguousRunner}
}

// AncestorBoundFrontiers certifies the runner-resolution edges of classify:
// possibleRunners uses the test's own directory and ancestor manifests/configs;
// explicit runner imports bind source files Owns claims. Arbitrary dynamic
// imports and runtime dependencies have separate, path-open reasons. Ambiguous
// runner entries carry display tokens, so they retain their conservative floor.
func (Language) AncestorBoundFrontiers() []string {
	return []string{FrontierUnknownRunner}
}

// ResolutionInput reports a path the plugin scans without owning it
// (AFP-V0-511): the manifests and runner, bundler and TypeScript configuration
// files observedName admits beside source files, such as `package.json` or a
// `tsconfig*.json` or `jsconfig*.json` whose `paths` steer alias resolution. A change there may
// rebind any import, so the plan keeps every frontier entry.
func (Language) ResolutionInput(relative string) bool {
	return observedName(path.Base(relative))
}

// CrossLanguageFrontiers names the reasons that stand for a runtime edge from
// another plugin's code into a TypeScript unit (AFP-V0-818): an end-to-end
// runner drives a server or binary the repository may build in another
// language, an edge no plugin models, so the plan keeps such an entry even
// when the change reaches TypeScript only through terminal selections, unless
// a declared runtime edge (AFP-V0-819) models the site.
func (Language) CrossLanguageFrontiers() []string {
	return []string{FrontierE2ERuntimeDependency}
}

// FrontierDirection classifies each reason by where its hidden edge starts
// (AZU-V0-003, AZU-V0-008; TJAA-V0-1161). OUTBOUND reasons start at a
// reference in the site's own file: an unresolved relative import, an
// undeclared bare import, a dynamic `import()` or `require()`. An executable
// configuration value the plugin could not evaluate (a `testMatch`, `include`,
// `specPattern` or `globalSetup` whose collected files or loaded module are
// unknown) is DISCOVERY: outbound, but it also decides which files below the
// configuration are tests, so a dirty configuration or a dirty source below
// it keeps the entry; so is a mock call whose target is computed (SLF-V1-004),
// which decides what module the calling test replaces. PACKAGE_RUNNABLE reasons hide only how a
// known test's runner is invoked or which runner it is:
// `runtime-flags-unresolved` and `test-runner-unknown`. Every other reason is
// INBOUND: the unresolved alias, whose site may be the resolution table
// (`tsconfig.json`) under a blocked alias gate (TJAA-V0-059) rather than the
// importing file, so the edge does not start at the site; the end-to-end
// runtime dependency (a caller into another plugin's code); the ambiguous
// runner, the puppeteer host, Detox, Storybook, the cross-language test asset,
// and the unreadable, unparsed, over-bound and scan frontiers, whose site is
// not even known.
func (Language) FrontierDirection(reason string) string {
	switch reason {
	case FrontierUnresolvedImport, FrontierBareImportUndeclared, FrontierDynamicImport:
		return affected.FrontierOutbound
	case FrontierConfig, FrontierMockTarget:
		return affected.FrontierDiscovery
	case FrontierRuntimeFlags, FrontierUnknownRunner:
		return affected.FrontierPackageRunnable
	}
	return affected.FrontierInbound
}

// CoverageUnproven reports a changed manual mock, and any changed unit while a computed mock
// target is open (SLF-V1-004): the tests that load a mock are those that mock its module, which
// the plugin models only through literal targets and imports, so no absence of a consumer is
// proven and the unit is unknown NO_SELECTABLE_TEST, never Uncovered.
func (Language) CoverageUnproven(id string, frontier []string) bool {
	if manualMockPath(strings.TrimPrefix(id, "typescript:")) {
		return true
	}
	for _, entry := range frontier {
		if entry == FrontierMockTarget || strings.HasPrefix(entry, FrontierMockTarget+":") {
			return true
		}
	}
	return false
}

// RunnableUnit reports a test unit whose runner the plugin identified
// (AZU-V0-008, TJAA-V0-1161): its id is `typescript:<runner>:<path>` with a
// runner other than unknown (unitID), so the runner's adapter runs it as the
// package's tests. A source unit, whose id is `typescript:<path>`, and a unit
// of the unknown runner are not runnable.
func (Language) RunnableUnit(id string) bool {
	rest, ok := strings.CutPrefix(id, "typescript:")
	if !ok {
		return false
	}
	runner, _, found := strings.Cut(rest, ":")
	return found && runner != "" && runner != runnerUnknown && !strings.Contains(runner, "/")
}
