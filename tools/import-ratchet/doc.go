// Package importratchet holds only the area import-ratchet test (IRT-V0-001
// to IRT-V0-006, docs/specs/import-ratchet-v0.md). areas.json maps package
// path prefixes to layered areas; baseline.json lists the wrong-way
// cross-area imports that existed when the ratchet was introduced. The test
// fails on any wrong-way import not in the baseline and on any baseline entry
// that no longer exists, so the baseline can only shrink.
package importratchet

import _ "embed"

// The manifest and baseline are build inputs of this package, so an edit of
// either reaches the test through the package's own dependency graph.
var (
	//go:embed areas.json
	areasJSON []byte
	//go:embed baseline.json
	baselineJSON []byte
)
