package golang_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TSP-V0-003: a declared gated root locator stops making its dependents
// unbounded readers; a declaration that is invalid, names a package that does
// not locate the root, or names a gate its sources do not carry declares
// nothing and widens every plan; a dirty declaration leaves the plan UNKNOWN.
func TestGatedRootLocators_TSP_V0_003(t *testing.T) {
	base := map[string]string{
		"go.mod":              "module example.test/m\n",
		"loc/loc.go":          "package loc\n\nimport \"os\"\n\nconst Gate = \"/gate/root\"\n\nfunc Bind() string { dir, _ := os.Getwd(); return dir }\n",
		"loc/loc_test.go":     "package loc\n",
		"user/user.go":        "package user\n\nimport _ \"example.test/m/loc\"\n",
		"user/user_test.go":   "package user\n",
		"other/other.go":      "package other\n",
		"other/other_test.go": "package other\n",
		"plain/plain.go":      "package plain\n",
	}
	const valid = `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"locates only behind /gate/root"}}}`
	for _, tc := range []struct {
		name        string
		declaration string
		userChosen  bool
		invalid     bool
	}{
		{name: "undeclared", userChosen: true},
		{name: "declared", declaration: valid},
		{name: "gate not carried", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/elsewhere","reason":"r"}}}`, userChosen: true, invalid: true},
		{name: "not a locator", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r"},"plain":{"gate":"/gate/root","reason":"r"}}}`, userChosen: true, invalid: true},
		{name: "no package", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r"},"gone":{"gate":"/gate/root","reason":"r"}}}`, userChosen: true, invalid: true},
		{name: "relative gate", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"gate/root","reason":"r"}}}`, userChosen: true, invalid: true},
		{name: "unknown member", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r","reads":[]}}}`, userChosen: true, invalid: true},
		{name: "unparsed", declaration: `{`, userChosen: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			if tc.declaration != "" {
				files[affected.GatedRootLocatorsPath] = tc.declaration
			}
			root := t.TempDir()
			writeFiles(t, root, files)
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"other/other.go"})
			if got := slices.Contains(plan.SelectedTests(), "user/user_test.go"); got != tc.userChosen {
				t.Errorf("user selected %v, want %v (selected %v)", got, tc.userChosen, plan.SelectedTests())
			}
			invalid := slices.Contains(plan.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierGatedLocatorsInvalid})
			if invalid != tc.invalid {
				t.Errorf("invalid frontier %v, want %v (unknown %v)", invalid, tc.invalid, plan.Unknown)
			}
			if !tc.invalid && plan.Scope != affected.ScopeBounded {
				t.Errorf("scope %s (unknown %v), want BOUNDED", plan.Scope, plan.Unknown)
			}
			if !slices.Contains(plan.SelectedTests(), "loc/loc_test.go") {
				t.Errorf("the locator's own tests stay unbounded readers: selected %v", plan.SelectedTests())
			}
			// A change the locator's dependency reaches still selects the user.
			if reached := affected.Select(graph, []string{"loc/loc.go"}); !slices.Contains(reached.SelectedTests(), "user/user_test.go") {
				t.Errorf("loc/loc.go: user not selected (selected %v)", reached.SelectedTests())
			}
			if tc.declaration == "" {
				return
			}
			changed := affected.Select(graph, []string{affected.GatedRootLocatorsPath})
			if changed.Scope != affected.ScopeUnknown || !slices.Contains(changed.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: affected.FrontierGatedLocatorsChanged}) {
				t.Errorf("dirty declaration: scope %s, unknown %v; want the changed frontier", changed.Scope, changed.Unknown)
			}
		})
	}
}
