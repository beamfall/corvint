package testsource

import (
	"sort"

	"github.com/Beamfall/corvint/internal/testdiscovery"
)

// Shared Ruby lexical reading for the RSpec, seed-parity and teardown readers. These helpers
// outlived the retired actor-authorization check (V1-4822), which first carried them; the reader
// itself lives in internal/testdiscovery.

// rubyLine is testdiscovery.RubyLine, kept under its old name.
type rubyLine = testdiscovery.RubyLine

// rubyLines is testdiscovery.RubyLines, kept under its old name.
func rubyLines(text string) []rubyLine { return testdiscovery.RubyLines(text) }

// stripRubyComment is testdiscovery.StripRubyComment, kept under its old name.
func stripRubyComment(line string) string { return testdiscovery.StripRubyComment(line) }

// rubyOpener is testdiscovery.RubyOpener, kept under its old name.
var rubyOpener = testdiscovery.RubyOpener

// rubyDo is testdiscovery.RubyDo, kept under its old name.
var rubyDo = testdiscovery.RubyDo

// rubyEnd is testdiscovery.RubyEnd, kept under its old name.
var rubyEnd = testdiscovery.RubyEnd

// rubyOneLine is testdiscovery.RubyOneLine, kept under its old name.
var rubyOneLine = testdiscovery.RubyOneLine

// rubyEndlessDef is testdiscovery.RubyEndlessDef, kept under its old name.
var rubyEndlessDef = testdiscovery.RubyEndlessDef

// rubyOpens is testdiscovery.RubyOpens, kept under its old name.
func rubyOpens(text string) bool { return testdiscovery.RubyOpens(text) }

// rubyValueKeyword is testdiscovery.RubyValueKeyword, kept under its old name.
func rubyValueKeyword(text string) string { return testdiscovery.RubyValueKeyword(text) }

// rubyBlockEnd is testdiscovery.RubyBlockEnd, kept under its old name.
func rubyBlockEnd(lines []rubyLine, i int) int { return testdiscovery.RubyBlockEnd(lines, i) }

// sortedKeys returns a map's keys in deterministic order for the environment and teardown readers.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
