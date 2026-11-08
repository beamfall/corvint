// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func qualifiedFixture(t *testing.T) (string, Receipt) {
	t.Helper()
	root := writeFixture(t, map[string]string{
		"go.mod":              "module " + fixtureModule + "\n\ngo 1.22\n",
		"core/core.go":        "package core\nfunc Value() int { return 1 }\n",
		"user/user_test.go":   "package user\nimport (\"testing\"; \"" + fixtureModule + "/core\")\nfunc TestValue(t *testing.T) { _ = core.Value() }\n",
		"quiet/quiet_test.go": "package quiet\nimport \"testing\"\nfunc TestQuiet(t *testing.T) {}\n",
	})
	var plan Receipt
	plan.Plan.Scope = "BOUNDED"
	plan.Plan.Dirty = []string{"core/core.go"}
	plan.Provider.Go.State = "RUNNABLE"
	return root, plan
}

// AFP-V0-9500: an actual immutable base permits additive narrowing; changing
// an existing declaration invalidates that proof and restores the importer.
func TestQualifiedSelectionRecomputesAdditive_AFPV09500(t *testing.T) {
	root, plan := qualifiedFixture(t)
	fixtureGit(t, root, "init", "-q")
	fixtureGit(t, root, "add", "-A")
	fixtureGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "base")
	plan.Range.Base = strings.TrimSpace(fixtureGit(t, root, "rev-parse", "HEAD"))
	for _, tc := range []struct{ source, want string }{
		{"package core\nfunc Value() int { return 1 }\nfunc Fresh() int { return 2 }\n", "NOTHING "},
		{"package core\nfunc Value() int { return 3 }\nfunc Fresh() int { return 2 }\n", "run " + fixtureModule + "/user"},
	} {
		if err := os.WriteFile(filepath.Join(root, "core", "core.go"), []byte(tc.source), 0o644); err != nil {
			t.Fatal(err)
		}
		lines := SelectQualified(plan, fixtureModule, root)
		if got := lines[len(lines)-1]; !strings.HasPrefix(got, tc.want) {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
}

// AFP-V0-9500: both consumers use the same fresh planner proof. The no-test
// library is not an execution obligation; its test importer is, even when the
// receipt omits it. A receipt-only positive obligation is never intersected out.
func TestQualifiedSelectionProofAndParity_AFPV09500(t *testing.T) {
	root, plan := qualifiedFixture(t)
	check := func(want string) {
		t.Helper()
		direct := SelectQualified(plan, fixtureModule, root)
		indexed := SelectQualifiedIndexed(plan, fixtureModule, root, StartIndex(root, fixtureModule, nil))
		if !reflect.DeepEqual(direct, indexed) {
			t.Fatalf("consumer divergence:\n%v\n%v", direct, indexed)
		}
		if got := direct[len(direct)-1]; got != want {
			t.Fatalf("verdict %q, want %q; audit=%v", got, want, direct)
		}
	}
	check("run " + fixtureModule + "/user")
	plan.Provider.Go.Packages = []string{fixtureModule + "/quiet"}
	check("run " + fixtureModule + "/quiet " + fixtureModule + "/user")
	// A supplied NARROWED claim with no immutable base cannot prune user.
	plan.Plan.GoAdditive = append(plan.Plan.GoAdditive, struct {
		Unit         string   `json:"unit"`
		State        string   `json:"state"`
		Paths        []string `json:"paths"`
		Unreferenced []string `json:"unreferenced"`
	}{"go:" + fixtureModule + "/core", "NARROWED", []string{"core/core.go"}, []string{"go:" + fixtureModule + "/user"}})
	check("run " + fixtureModule + "/quiet " + fixtureModule + "/user")
}

func TestQualifiedSelectionRetainsUnobservedRuntimeReader_AFPV09500(t *testing.T) {
	root, plan := qualifiedFixture(t)
	if err := os.Mkdir(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"quiet/quiet_test.go","runs":"other:server","reason":"drives server"}]}`
	if err := os.WriteFile(filepath.Join(root, ".corvint", "runtime-edges.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := SelectQualified(plan, fixtureModule, root)
	if got, want := lines[len(lines)-1], "run "+fixtureModule+"/quiet "+fixtureModule+"/user"; got != want {
		t.Fatalf("unobserved runtime reader lost: %v", lines)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "unresolved "+fixtureModule+"/quiet:") {
		t.Fatalf("unobserved runtime reader has no named reason: %v", lines)
	}
}

// AFP-V0-9500 negative controls: unknown, unsupported, malformed and capped
// inputs cannot establish exclusion. Every failure names its widening reason.
func TestQualifiedSelectionFailsClosed_AFPV09500(t *testing.T) {
	for _, name := range []string{"unknown", "scope", "empty unsupported scope", "path", "package", "source"} {
		t.Run(name, func(t *testing.T) {
			root, plan := qualifiedFixture(t)
			switch name {
			case "unknown":
				plan.Plan.Scope = "UNKNOWN"
			case "scope":
				plan.Plan.Scope = "future"
			case "empty unsupported scope":
				plan.Plan.Scope = "future"
				plan.Plan.Dirty = nil
				plan.Provider.Go.State = "EMPTY_SELECTION"
			case "path":
				plan.Plan.Dirty = []string{"../core/core.go"}
			case "package":
				plan.Provider.Go.Packages = []string{"outside.test/pkg"}
			case "source":
				root = writeFixture(t, map[string]string{
					"go.mod":       "module " + fixtureModule + "\n",
					"core/core.go": "package core\n" + strings.Repeat(" ", 9<<20),
				})
			}
			lines := SelectQualified(plan, fixtureModule, root)
			if verdict := lines[len(lines)-1]; !strings.HasPrefix(verdict, "FALLBACK ") || verdict == "FALLBACK " {
				t.Fatalf("unqualified %s narrowed: %v", name, lines)
			}
		})
	}
}

// AFP-V0-9500: only valid slash paths with a recognized nested-module
// certificate can turn empty conservative attribution into NOTHING.
func TestQualifiedSelectionModuleCoverageFailsClosed_AFPV09500(t *testing.T) {
	for _, tc := range []struct {
		name, dirty, detail string
		nothing             bool
	}{
		{"valid", "nested/x.go", "go:unobserved-module:nested:nested/x.go", true},
		{"reported malformed path", "nested//x.go", "go:unobserved-module:nested:nested//x.go", false},
		{"parent element", "nested/../x.go", "go:unobserved-module:nested:nested/../x.go", false},
		{"unknown certificate form", "nested/x.go", "go:future-module:nested:nested/x.go", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFixture(t, map[string]string{
				"go.mod":        "module " + fixtureModule + "\n",
				"nested/go.mod": "module example.test/nested\n",
				"nested/x.go":   "package nested\n",
			})
			var plan Receipt
			plan.Plan.Scope = "UNKNOWN"
			plan.Provider.Go.State = "EMPTY_SELECTION"
			plan.Plan.Dirty = []string{tc.dirty}
			plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
				Reason string `json:"reason"`
				Detail string `json:"detail"`
			}{"LANGUAGE_FRONTIER", tc.detail})
			for _, lines := range [][]string{SelectQualified(plan, fixtureModule, root), SelectQualifiedIndexed(plan, fixtureModule, root, StartIndex(root, fixtureModule, nil))} {
				prefix := "FALLBACK "
				if tc.nothing {
					prefix = "NOTHING "
				}
				if got := lines[len(lines)-1]; !strings.HasPrefix(got, prefix) {
					t.Fatalf("got %q, want %q; audit=%v", got, prefix, lines)
				}
			}
		})
	}
}

// AFP-V0-9500: a declared runtime edge is invisible to the Go-only index, so
// a root test that runs a unit inside the dirty nested module, or an edge the
// downgrade cannot place, blocks the NOTHING downgrade. An edge elsewhere does not.
func TestQualifiedSelectionModuleCoverageRuntimeEdges_AFPV09500(t *testing.T) {
	for _, tc := range []struct {
		name, runs string
		nothing    bool
	}{
		{"edge into nested module", "typescript:nested/server.cjs", false},
		{"go edge outside root module", "go:example.test/nested", false},
		{"edge elsewhere", "go:" + fixtureModule + "/core", true},
		{"invalid declaration", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"e2e/run_test.go","runs":"` + tc.runs + `","reason":"drives it"}]}`
			if tc.runs == "" {
				body = "{"
			}
			root := writeFixture(t, map[string]string{
				"go.mod":                      "module " + fixtureModule + "\n",
				"core/core.go":                "package core\n",
				"e2e/run_test.go":             "package e2e\nimport \"testing\"\nfunc TestRun(t *testing.T) {}\n",
				"nested/go.mod":               "module example.test/nested\n",
				"nested/x.go":                 "package nested\n",
				".corvint/runtime-edges.json": body,
			})
			var plan Receipt
			plan.Plan.Scope = "UNKNOWN"
			plan.Provider.Go.State = "EMPTY_SELECTION"
			plan.Plan.Dirty = []string{"nested/x.go"}
			plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
				Reason string `json:"reason"`
				Detail string `json:"detail"`
			}{"LANGUAGE_FRONTIER", "go:unobserved-module:nested:nested/x.go"})
			prefix := "FALLBACK "
			if tc.nothing {
				prefix = "NOTHING "
			}
			if lines := SelectQualified(plan, fixtureModule, root); !strings.HasPrefix(lines[len(lines)-1], prefix) {
				t.Fatalf("got %q, want %q; audit=%v", lines[len(lines)-1], prefix, lines)
			}
		})
	}
}
