package testvalidity

import "sort"

// Never-written assertion targets of RSpec examples (docs/specs/test-source-analysis-v0.md,
// TSA-V0-443..449). internal/testsource reads them from committed spec and application source;
// this file holds the vocabulary.

// RuleRBAssertTargetNeverWritten: an example asserts a plain attr_accessor or attr_reader of an
// object it builds, and nothing reachable from the methods it exercises on that object writes the
// accessor, so the assertion reads the reader's initial value whatever the code does. The reader
// does not run the code, so the example abstains (TSA-V0-444).
const RuleRBAssertTargetNeverWritten = "RB-ASSERT-TARGET-NEVER-WRITTEN"

// RubyApp is the committed application source an RSpec example's receiver class is resolved in
// (TSA-V0-448): Paths are the sorted committed paths of the `.rb` files under a spec prefix's
// app/, lib/ and config/ and its db/schema.rb, and Read returns one of them, or false when it
// cannot be read within the bounds. ReadAll reads every path and reports whether all of them read,
// so a scan of Paths through Read sees the whole application (TSA-V0-447); nil means it cannot.
type RubyApp struct {
	Paths   []string
	Read    func(path string) (string, bool)
	ReadAll func() bool
}

// RubyAppFiles is the RubyApp of an in-memory file set.
func RubyAppFiles(files map[string]string) *RubyApp {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return &RubyApp{Paths: paths, Read: func(path string) (string, bool) {
		text, ok := files[path]
		return text, ok
	}, ReadAll: func() bool { return true }}
}
