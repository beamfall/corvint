// SPDX-License-Identifier: Apache-2.0
package typescript

import "path"

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

// ResolutionInput reports a path the plugin scans without owning it
// (AFP-V0-511): the manifests and runner, bundler and TypeScript configuration
// files observedName admits beside source files, such as `package.json` or a
// `tsconfig*.json` or `jsconfig*.json` whose `paths` steer alias resolution. A change there may
// rebind any import, so the plan keeps every frontier entry.
func (Language) ResolutionInput(relative string) bool {
	return observedName(path.Base(relative))
}
