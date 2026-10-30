// SPDX-License-Identifier: Apache-2.0
package python

// SourceBoundFrontiers names the reasons whose hidden edge the plugin's own
// import resolution binds only to a Python module (AFP-V0-511). A conditional,
// dynamic or unresolved relative import and an unresolved `pytest_plugins`
// entry each name a module, which the plugin resolves to a `.py` path it owns
// or to nothing; none can bind a path outside Owns. An unreadable source file
// hides every edge it may hold, so it stays path-open.
func (Language) SourceBoundFrontiers() []string {
	return []string{FrontierConditionalImport, FrontierDynamicImport, FrontierRelativeImport, FrontierPytestPlugins}
}

// FixtureClosedFrontiers declares no fixture-closed reason: the plugin's only
// path-open reason is an unreadable source file (AFP-V0-511).
func (Language) FixtureClosedFrontiers() []string { return nil }

// ResolutionInput reports no path: the plugin scans only the `.py` files it
// owns, so no unowned path steers its resolution (AFP-V0-511).
func (Language) ResolutionInput(string) bool { return false }
