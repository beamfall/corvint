// SPDX-License-Identifier: Apache-2.0
package testid

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func listed(t *testing.T, name string, read func(*os.File) ([]Named, error)) []Entry {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	named, err := read(file)
	if err != nil {
		t.Fatal(err)
	}
	return List(named)
}

// CTI-V1-010: two equal Jest titles in a.test.js produce one ID; the second
// gets the final segment #2, both are marked duplicate, and a change that
// selects either one selects jest:a.test.js with duplicate-test-id.
func TestCTIV1010DuplicateJestTitlesWidenToFile(t *testing.T) {
	t.Parallel()
	entries := listed(t, "jest.json", func(f *os.File) ([]Named, error) { return JestJSON(Jest, f, "/repo") })
	want := []Entry{
		{ID: "jest:web/sum.test.js::sum::adds"},
		{ID: "jest:web/sum.test.js::sum::each::case 1"},
		{ID: "jest:web/sum.test.js::sum::each::case 2"},
		{ID: "jest:a.test.js::works", Duplicate: true},
		{ID: "jest:a.test.js::works::#2", Duplicate: true},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("List = %+v\nwant %+v", entries, want)
	}
	for _, requested := range []string{"jest:a.test.js::works", "jest:a.test.js::works::#2"} {
		got := Resolve(requested, entries, Hint{})
		if got != (Selection{Scope: ScopeFile, Selector: "jest:a.test.js", Runner: Jest, Reason: ReasonDuplicate}) {
			t.Errorf("Resolve(%q) = %+v", requested, got)
		}
	}
	if got := Resolve("jest:web/sum.test.js::sum::adds", entries, Hint{}); got != (Selection{Scope: ScopeTest, Selector: "jest:web/sum.test.js::sum::adds", Runner: Jest}) {
		t.Errorf("a unique test widened: %+v", got)
	}
	if got := Resolve("jest:web/sum.test.js", entries, Hint{}); got != (Selection{Scope: ScopeFile, Selector: "jest:web/sum.test.js", Runner: Jest}) {
		t.Errorf("a file-level request: %+v", got)
	}
	// An external _test package declaring the same name as an internal one
	// runs both under one package report: a duplicate, never one test.
	goDup := List([]Named{{ID: ID{Go, "p", []string{"TestX"}}}, {ID: ID{Go, "p", []string{"TestX"}}}, {ID: ID{Go, "p", []string{"TestY"}}}})
	if goDup[1].ID != "go:p::TestX::#2" || !goDup[0].Duplicate || !goDup[1].Duplicate || goDup[2].Duplicate {
		t.Errorf("Go duplicate listing %+v", goDup)
	}
	if got := Resolve("go:p::TestX", goDup, Hint{}); got.Selector != "go:p" || got.Reason != ReasonDuplicate {
		t.Errorf("Go duplicate selection %+v", got)
	}
	// File-level entries a report gives without a name path are not
	// suffixed: they already select the whole file.
	rspec := listed(t, "rspec.json", func(f *os.File) ([]Named, error) { return RSpecJSON(f) })
	for _, entry := range rspec {
		if entry != (Entry{ID: "rspec:spec/user_spec.rb", Reason: ReasonUnmapped}) {
			t.Errorf("rspec entry %+v", entry)
		}
	}
}

// CTI-V1-011: an unknown runner token or a malformed ID is never matched by
// guess: it widens to the hinted file, else every test of the hinted
// runner, else every test, with test-id-unmapped. A renamed test's old ID is
// reported dangling and selects its file; a test of a file whose report
// names no test path selects the file, unmapped, not dangling.
func TestCTIV1011UnknownAndMalformedWidenWithReason(t *testing.T) {
	t.Parallel()
	listing := List([]Named{{ID: ID{Vitest, "a.test.ts", []string{"new name"}}}})
	for _, c := range []struct {
		name      string
		requested string
		hint      Hint
		want      Selection
	}{
		{"unknown runner, file known", "cypress:a.cy.ts::x", Hint{Runner: Vitest, File: "a.test.ts"},
			Selection{Scope: ScopeFile, Selector: "vitest:a.test.ts", Runner: Vitest, Reason: ReasonUnmapped}},
		{"unknown runner, runner known", "cypress:a.cy.ts::x", Hint{Runner: Vitest},
			Selection{Scope: ScopeRunner, Runner: Vitest, Reason: ReasonUnmapped}},
		{"unknown runner, nothing known", "cypress:a.cy.ts::x", Hint{},
			Selection{Scope: ScopeAll, Reason: ReasonUnmapped}},
		{"malformed escape, file known", "vitest:a.test.ts::GET /a:b", Hint{Runner: Vitest, File: "a.test.ts"},
			Selection{Scope: ScopeFile, Selector: "vitest:a.test.ts", Runner: Vitest, Reason: ReasonUnmapped}},
		{"malformed, unknown hint runner", "not an id", Hint{Runner: "mocha", File: "a.js"},
			Selection{Scope: ScopeAll, Reason: ReasonUnmapped}},
		{"legacy unit key is not an ID", "typescript:vitest:a.test.ts", Hint{Runner: Vitest},
			Selection{Scope: ScopeRunner, Runner: Vitest, Reason: ReasonUnmapped}},
		{"renamed test", "vitest:a.test.ts::old name", Hint{},
			Selection{Scope: ScopeFile, Selector: "vitest:a.test.ts", Runner: Vitest, Reason: ReasonUnmapped, Dangling: true}},
		{"current test", "vitest:a.test.ts::new name", Hint{},
			Selection{Scope: ScopeTest, Selector: "vitest:a.test.ts::new name", Runner: Vitest}},
		{"no listing", "vitest:a.test.ts::new name", Hint{},
			Selection{Scope: ScopeFile, Selector: "vitest:a.test.ts", Runner: Vitest, Reason: ReasonUnmapped, Dangling: true}},
	} {
		entries := listing
		if c.name == "no listing" {
			entries = nil
		}
		if got := Resolve(c.requested, entries, c.hint); got != c.want {
			t.Errorf("%s: Resolve(%q) = %+v, want %+v", c.name, c.requested, got, c.want)
		}
	}
	rspec := List([]Named{{ID: ID{RSpec, "spec/user_spec.rb", nil}, Reason: ReasonUnmapped}})
	if got := Resolve("rspec:spec/user_spec.rb::User::saves", rspec, Hint{}); got != (Selection{Scope: ScopeFile, Selector: "rspec:spec/user_spec.rb", Runner: RSpec, Reason: ReasonUnmapped}) {
		t.Errorf("a test of a file listed without name paths: %+v", got)
	}
}
