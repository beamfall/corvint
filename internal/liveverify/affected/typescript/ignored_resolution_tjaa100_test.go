package typescript

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-100 (PR #1753 review): an import whose first candidate is an excluded ignored file
// stays unresolved and never falls through to a later candidate the runtime would not load.
func TestIgnoredCandidateStaysUnresolved_TJAA_V0_100(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "tests/a.test.ts", "import { value } from './helper.cjs'\n")
	write(t, root, "tests/helper.js", "export const value = 1\n")
	write(t, root, "tests/helper.cjs", "module.exports = require('../src/value.js')\n")
	write(t, root, "src/value.js", "module.exports = 2\n")
	release, err := affected.HoldIgnoredExclusion(root, []string{"tests/helper.cjs"})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !sites.has(FrontierUnresolvedImport, "tests/a.test.ts:1") {
		t.Fatalf("unresolved sites=%v", sites[FrontierUnresolvedImport])
	}
	if got, local, unresolved := resolveImport("tests/a.test.ts", "./helper.js", map[string]string{"tests/helper.js": "js"}, affected.HeldIgnoredMatcher(root)); got != "js" || !local || unresolved {
		t.Fatalf("an earlier unexcluded candidate must still win: %q %v %v", got, local, unresolved)
	}
}
