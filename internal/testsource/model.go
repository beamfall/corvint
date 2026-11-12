package testsource

import (
	"github.com/Beamfall/corvint/internal/testdiscovery"
)

// source is testdiscovery.Source, kept under its old name.
type source = testdiscovery.Source

// decl is testdiscovery.Decl, kept under its old name.
type decl = testdiscovery.Decl

// parseSource is testdiscovery.ParseSource, kept under its old name.
func parseSource(name, text string) *source { return testdiscovery.ParseSource(name, text) }

// within is testdiscovery.Within, kept under its old name.
func within(i int, r [2]int) bool { return testdiscovery.Within(i, r) }

// describes returns the literal titles of the describes enclosing declaration d, outermost first.
func describes(s *source, d decl) []string {
	titles := []string{}
	for _, outer := range s.Decls {
		if outer.Kind == "describe" && outer.At < d.At && within(d.At, outer.Body) {
			titles = append(titles, outer.Title)
		}
	}
	return titles
}

// expectChain is testdiscovery.ExpectChain, kept under its old name.
type expectChain = testdiscovery.ExpectChain
