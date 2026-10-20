package typescript

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestChangedSymbols_TJAA_V0_033(t *testing.T) {
	base := `import { test } from "@playwright/test"
// shared login fixture
export const user = "u"
export function login(name: string) {
  return name
}
const internal = 1
`
	for _, testCase := range []struct {
		name                       string
		after                      string
		beforeExists, afterExists  bool
		added, removed, modified   []string
		other, additive, unresolve bool
	}{
		{name: "additive export", after: base + "export const admin = \"a\"\nexport type Role = string\n", beforeExists: true, afterExists: true, added: []string{"Role", "admin"}, additive: true},
		{name: "additive export between statements", after: strings.Replace(base, "const internal", "export { user as person }\nconst internal", 1), beforeExists: true, afterExists: true, added: []string{"person"}, additive: true},
		{name: "comment and layout only", after: strings.Replace(strings.Replace(base, "// shared login fixture\n", "", 1), "return name", "return    name // same", 1), beforeExists: true, afterExists: true, additive: true},
		{name: "modified export", after: strings.Replace(base, "return name", "return name.trim()", 1), beforeExists: true, afterExists: true, modified: []string{"login"}},
		{name: "string content is significant", after: strings.Replace(base, `"u"`, `"u "`, 1), beforeExists: true, afterExists: true, modified: []string{"user"}},
		{name: "removed export", after: strings.Replace(base, "export const user = \"u\"\n", "", 1), beforeExists: true, afterExists: true, removed: []string{"user"}},
		{name: "other statement", after: strings.Replace(base, "internal = 1", "internal = 2", 1), beforeExists: true, afterExists: true, other: true},
		{name: "additive export beside other change", after: strings.Replace(base, "internal = 1", "internal = 2", 1) + "export const admin = 1\n", beforeExists: true, afterExists: true, added: []string{"admin"}, other: true},
		{name: "new file", after: "export const fresh = 1\n", afterExists: true, added: []string{"fresh"}, additive: true},
		{name: "deleted file", beforeExists: true, removed: []string{"login", "user"}, other: true},
		{name: "unmodelled export form", after: base + "export = user\n", beforeExists: true, afterExists: true, other: true},
		{name: "unterminated string does not tokenize", after: base + "export const broken = \"x\n", beforeExists: true, afterExists: true, unresolve: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			before := []byte(base)
			if !testCase.beforeExists {
				before = nil
			}
			change := ChangedSymbols("tests/support/fixture.ts", before, testCase.beforeExists, []byte(testCase.after), testCase.afterExists)
			if testCase.unresolve {
				if change.State != PlaywrightSymbolsUnresolved || change.Detail == "" || change.AdditiveOnly {
					t.Fatalf("want UNRESOLVED: %+v", change)
				}
				return
			}
			want := func(names []string) []string {
				if names == nil {
					return []string{}
				}
				return names
			}
			if change.State != PlaywrightSymbolsResolved || !slices.Equal(change.Added, want(testCase.added)) || !slices.Equal(change.Removed, want(testCase.removed)) || !slices.Equal(change.Modified, want(testCase.modified)) || change.OtherChanged != testCase.other || change.AdditiveOnly != testCase.additive {
				t.Fatalf("%+v", change)
			}
		})
	}
}

func TestChangedSymbolsUnresolved_TJAA_V0_033(t *testing.T) {
	if change := ChangedSymbols("tests/support/view.tsx", []byte("export const a = 1"), true, []byte("export const a = 1\nexport const b = 2"), true); change.State != PlaywrightSymbolsUnresolved || change.AdditiveOnly {
		t.Fatalf("jsx: %+v", change)
	}
	large := []byte("export const a = \"" + strings.Repeat("x", playwrightSymbolMaxBytes) + "\"")
	if change := ChangedSymbols("tests/support/big.ts", large, true, large, true); change.State != PlaywrightSymbolsUnresolved {
		t.Fatalf("oversize: %+v", change)
	}

	root := narrowingFixture(t, narrowingDynamicMobile)
	failing := func(string) ([]byte, bool, error) { return nil, false, errors.New("unread") }
	changes := playwrightChangedSymbols(root, []string{"README.md", "tests/support/fixture.ts"}, failing)
	if len(changes) != 1 || changes[0].State != PlaywrightSymbolsUnresolved || changes[0].Detail != "comparison base content is unreadable" {
		t.Fatalf("unreadable base: %+v", changes)
	}
	if changes := playwrightChangedSymbols(root, []string{"tests/support/fixture.ts"}, nil); len(changes) != 1 || changes[0].Detail != "no comparison base was read" {
		t.Fatalf("nil reader: %+v", changes)
	}
	dirty := []string{}
	for index := 0; index <= playwrightSymbolMaxPaths; index++ {
		dirty = append(dirty, "tests/support/fixture.ts")
	}
	same := func(string) ([]byte, bool, error) { return []byte(`export const user = "u"`), true, nil }
	changes = playwrightChangedSymbols(root, dirty, same)
	if len(changes) != playwrightSymbolMaxPaths+1 || changes[0].State != PlaywrightSymbolsResolved || changes[playwrightSymbolMaxPaths].State != PlaywrightSymbolsUnresolved {
		t.Fatalf("path bound: first %+v last %+v", changes[0], changes[len(changes)-1])
	}
}

func TestPlaywrightNarrowingReportsSymbols_TJAA_V0_033(t *testing.T) {
	root := narrowingFixture(t, narrowingDynamicMobile)
	write(t, root, "tests/support/fixture.ts", "export const user = \"u\"\nexport const admin = \"a\"\n")
	before := func(relative string) ([]byte, bool, error) {
		if relative != "tests/support/fixture.ts" {
			return nil, false, errors.New("unexpected path")
		}
		return []byte(`export const user = "u"`), true, nil
	}
	plan, err := SelectPlaywrightWithBase(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil, before)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Narrowed == nil || len(plan.Narrowed.Symbols) != 1 {
		t.Fatalf("narrowing: %+v", plan.Narrowed)
	}
	symbols := plan.Narrowed.Symbols[0]
	if symbols.State != PlaywrightSymbolsResolved || !symbols.AdditiveOnly || !slices.Equal(symbols.Added, []string{"admin"}) {
		t.Fatalf("symbols: %+v", symbols)
	}
	// Additive symbols are evidence only: the candidates still include the importing test.
	if got := narrowingCandidates(plan.Narrowed); !slices.Contains(got, "chromium tests/login.spec.ts") {
		t.Fatalf("candidates %v", got)
	}
}
