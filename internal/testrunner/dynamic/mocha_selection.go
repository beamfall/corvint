package dynamic

import (
	"path/filepath"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// reconcileMochaSelection checks native Mocha inventory against the exact file
// selectors and expected identities that were admitted for the run. Mocha
// merges configured spec files with positional selectors and only warns about
// a selector that matches nothing, so either form exits zero (TRE-V0-024).
// The comparison is lexical: Parse stays pure, so a non-canonical root, a
// directory selector or a native path outside the selected files stays
// incomplete instead of being resolved.
func reconcileMochaSelection(in tr.Input, o *tr.Observation) {
	if in.Runner != "mocha" {
		return
	}
	files := make([]string, len(in.Selectors))
	selected := map[string]bool{}
	for i, s := range in.Selectors {
		if !filepath.IsAbs(s) {
			s = filepath.Join(in.SourceRoot, filepath.FromSlash(s))
		}
		files[i] = filepath.Clean(s)
		selected[files[i]] = false
	}
	expected := map[string]bool{}
	for _, id := range in.Expected {
		expected[id] = true
	}
	for _, t := range o.Tests {
		if len(selected) > 0 {
			if _, ok := selected[t.File]; ok {
				selected[t.File] = true
			} else {
				problem(o, "unselected-test-file", t.ID)
			}
		}
		if len(expected) > 0 && !expected[t.ID] {
			problem(o, "unexpected-observed-test", t.ID)
		}
	}
	for _, f := range files {
		if !selected[f] {
			problem(o, "selector-without-tests", f)
		}
	}
}
