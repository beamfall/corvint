package testrunner

import (
	"fmt"
	"strings"
)

// Selection is a closed, versioned stable test-selection expectation. It is
// admitted with the plan before launch and matched after native observation.
// It never rewrites native test identities, which keep their run/session parts.
type Selection struct {
	Version string   `json:"version"`
	Matcher string   `json:"matcher"`
	Tests   []string `json:"tests"`
}

const (
	SelectionVersion = "corvint-test-selection/1"
	// NightwatchSessionElided projects a Nightwatch native identity
	// ModulePath::Name::TestEnv::SessionID::Case onto ModulePath::Name::TestEnv::Case.
	NightwatchSessionElided = "nightwatch-session-elided/1"
	maxSelectionKeyBytes    = 4096
)

type selectionMatcher struct {
	runner string
	key    func(Test) (string, bool)
}

// selectionMatchers binds each closed matcher to the one runner whose native
// identity shape it understands.
var selectionMatchers = map[string]selectionMatcher{NightwatchSessionElided: {"nightwatch", nightwatchStableKey}}

// AdmitSelection refuses a selection that is not closed, bounded, unique and
// bound to the request's runner. Exact ExpectedTests and a Selection are
// mutually exclusive so coverage has exactly one source.
func AdmitSelection(r Request) error {
	s := r.ExpectedSelection
	if s == nil {
		return nil
	}
	if s.Version != SelectionVersion {
		return fmt.Errorf("unsupported test selection version %q", s.Version)
	}
	if m, ok := selectionMatchers[s.Matcher]; !ok || m.runner != r.Runner {
		return fmt.Errorf("test selection matcher %q is not admitted for runner %q", s.Matcher, r.Runner)
	}
	if len(r.ExpectedTests) > 0 {
		return fmt.Errorf("expectedTests and expectedSelection are mutually exclusive")
	}
	if len(s.Tests) == 0 || len(s.Tests) > MaxTests {
		return fmt.Errorf("test selection requires 1..%d stable identities", MaxTests)
	}
	seen := map[string]bool{}
	for _, k := range s.Tests {
		if !selectionKey(k) {
			return fmt.Errorf("invalid stable test selection identity %q", k)
		}
		if seen[k] {
			return fmt.Errorf("duplicate stable test selection identity %q", k)
		}
		seen[k] = true
	}
	return nil
}

// selectionKey accepts exactly four non-empty, separator-free components.
func selectionKey(k string) bool {
	if len(k) > maxSelectionKeyBytes || strings.ContainsAny(k, "\x00\r\n") {
		return false
	}
	parts := strings.Split(k, "::")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		// A component edge colon would make ":::" split ambiguously.
		if p == "" || strings.HasPrefix(p, ":") || strings.HasSuffix(p, ":") {
			return false
		}
	}
	return true
}

// nightwatchStableKey derives the stable key from the structured native fields,
// not by trusting a split of the identity string. Every component must be
// separator-free so the projection is injective, and the session must exist
// without an edge colon that could move a character across the TestEnv boundary.
func nightwatchStableKey(t Test) (string, bool) {
	prefix, suffix := t.File+"::"+t.Suite+"::", "::"+t.Name
	if len(t.ID) < len(prefix)+len(suffix) || !strings.HasPrefix(t.ID, prefix) || !strings.HasSuffix(t.ID, suffix) {
		return "", false
	}
	middle := strings.Split(t.ID[len(prefix):len(t.ID)-len(suffix)], "::")
	if len(middle) != 2 || middle[1] == "" || strings.HasPrefix(middle[1], ":") || strings.HasSuffix(middle[1], ":") {
		return "", false
	}
	k := t.File + "::" + t.Suite + "::" + middle[0] + "::" + t.Name
	return k, selectionKey(k)
}

// selectionProblems checks the observed native inventory against an admitted
// selection: every stable identity once, nothing extra, nothing aliased.
func selectionProblems(in Input, tests []Test) []Problem {
	s := in.ExpectedSelection
	var out []Problem
	add := func(code, detail string) { out = append(out, Problem{code, detail}) }
	if err := AdmitSelection(Request{Runner: in.Runner, ExpectedTests: in.Expected, ExpectedSelection: s}); err != nil {
		add("invalid-test-selection", err.Error())
		return out
	}
	expected := map[string]bool{}
	for _, k := range s.Tests {
		expected[k] = true
	}
	observed := map[string]string{}
	for _, t := range tests {
		k, ok := selectionMatchers[s.Matcher].key(t)
		switch prior, dup := observed[k]; {
		case !ok:
			add("unmatchable-selected-test", t.ID)
		case dup:
			add("aliased-selected-test", prior+" | "+t.ID)
		case !expected[k]:
			observed[k] = t.ID
			add("extra-selected-test", t.ID)
		default:
			observed[k] = t.ID
		}
	}
	for _, k := range s.Tests {
		if _, ok := observed[k]; !ok {
			add("missing-selected-test", k)
		}
	}
	return out
}
