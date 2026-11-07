// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const (
	afpv2750Module = "example.test/m"
	afpv2750Reader = "ruby:implicit-test-dependency:spec/reader_spec.rb"
)

// afpv2750Plan writes the AFP-V0-2750 fixture repository and returns it with a
// receipt maker. A Ruby spec that reads repository files by a computed name
// bears on a deleted Go source, pkg/gone.go, through its path-open
// implicit-test-dependency entry (AFP-V0-2749,
// TestDeletedGoSourceStaysUnownedForOtherPlugins_AFPV2749 in
// internal/liveverify/affected). The receipt is the plan the planner writes for
// the deletion (AFP-V0-2610): pkg/gone.go is absent from the tree, raises no
// entry of its own, and selects its package and importer; the Ruby entry, when
// given, is the plan's only record of the Ruby reader.
func afpv2750Plan(t *testing.T) (string, func(...Unknown) gateselect.Receipt) {
	t.Helper()
	const module = afpv2750Module
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":              "module " + module + "\n\ngo 1.27\n",
		"pkg/pkg.go":          "package pkg\n",
		"pkg/pkg_test.go":     "package pkg\n",
		"user/user.go":        "package user\n\nimport _ \"" + module + "/pkg\"\n",
		"user/user_test.go":   "package user\n",
		"other/other.go":      "package other\n",
		"other/other_test.go": "package other\n",
		"spec/reader_spec.rb": "RSpec.describe 'reader' do\n  it { File.read(File.join(ROOT, name)) }\nend\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, user := module+"/pkg", module+"/user"
	receipt := func(unknown ...Unknown) gateselect.Receipt {
		var plan gateselect.Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Provider.Go.Packages = []string{pkg, user}
		plan.Plan.Scope = "BOUNDED"
		if len(unknown) != 0 {
			plan.Plan.Scope = "UNKNOWN"
		}
		plan.Plan.Dirty = []string{"pkg/gone.go"}
		for _, entry := range unknown {
			plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
				Reason string `json:"reason"`
				Detail string `json:"detail"`
			}{entry.Reason, entry.Detail})
		}
		return plan
	}
	return root, receipt
}

// AFP-V0-2750 (review fix round 5), gate-affected-select: the plan's bearing
// Ruby frontier makes the driver write a `suite ruby` line naming the entry,
// and its Go verdict stays the one a plan without the entry gives. With the
// blanket non-Go allowance back in gateselect.AllowedUnknown the driver admits
// the entry silently and names no suite: this test fails.
func TestFrontierSuiteNamedByGateAffectedSelect_AFPV2750(t *testing.T) {
	const module, reader = afpv2750Module, afpv2750Reader
	root, receipt := afpv2750Plan(t)
	frontier := Unknown{Reason: affected.UnknownLanguageFrontier, Detail: reader}
	lines := gateselect.Select(receipt(frontier), module, root)
	verdict := lines[len(lines)-1]
	without := gateselect.Select(receipt(), module, root)
	if want := without[len(without)-1]; verdict != want || !strings.HasPrefix(verdict, "run ") {
		t.Fatalf("driver verdict %q, want the narrowed Go verdict %q unchanged", verdict, want)
	}
	if !slices.Contains(lines, "suite ruby <- "+reader) {
		t.Fatalf("driver lines %q: the computed Ruby reader's suite must be named", lines)
	}
	for _, line := range lines {
		if strings.Contains(line, "/other") {
			t.Fatalf("driver line %q: the frontier must not widen the Go selection", line)
		}
	}
}

// AFP-V0-2750 (review fix round 5), `corvint test`: the same entry adds the
// declared Ruby suite to the execution manifest of a NARROWED run, as
// `declared:ruby` locally, as a still-required check on the ci profile, or as
// the declared no-suite assertion, and the Go units stay the driver's. With
// the blanket non-Go allowance back in gateselect.AllowedUnknown, Build drops
// the entry and never runs Ruby: this test fails.
func TestFrontierSuiteRunsInCorvintTest_AFPV2750(t *testing.T) {
	const module, reader = afpv2750Module, afpv2750Reader
	pkg, user := module+"/pkg", module+"/user"
	frontier := Unknown{Reason: affected.UnknownLanguageFrontier, Detail: reader}
	bundle := LanguageCommand{Language: "ruby", Argv: []string{"bundle", "exec", "rspec"}}
	for _, tc := range []struct {
		name     string
		profile  string
		declared LanguageCommand
		units    []string
		checks   []string
		noSuite  []string
	}{
		{"local runs the declared Ruby suite", ProfileLocal, bundle,
			[]string{"0 go:" + pkg + " whole", "0 go:" + user + " whole", "0 declared:ruby whole"}, []string{}, []string{}},
		{"ci names the declared Ruby suite as a check", ProfileCI, bundle,
			[]string{"0 go:" + pkg + " whole", "0 go:" + user + " whole"}, []string{"bundle exec rspec"}, []string{}},
		{"a declared no-suite assertion is recorded", ProfileLocal, LanguageCommand{Language: "ruby", None: "fixtures only"},
			[]string{"0 go:" + pkg + " whole", "0 go:" + user + " whole"}, []string{}, []string{"ruby: fixtures only"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixtureInput()
			in.RunnerProfile = tc.profile
			in.GoSelection = "run " + pkg + " " + user
			in.Universe = []string{module + "/other", pkg, user}
			in.Selected = []string{"go:" + pkg, "go:" + user}
			in.Unknown = []Unknown{frontier}
			declare(&in, tc.declared)
			m, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			if m.Scope != ScopeNarrowed || m.Reason != "" || len(m.Incomplete) != 0 {
				t.Fatalf("scope %s reason %q incomplete %q, want a complete narrowed run", m.Scope, m.Reason, m.Incomplete)
			}
			if !reflect.DeepEqual(unitIDs(m), tc.units) || !reflect.DeepEqual(m.Checks, tc.checks) || !reflect.DeepEqual(m.NoSuite, tc.noSuite) {
				t.Fatalf("units %q checks %q noSuite %q, want %q %q %q", unitIDs(m), m.Checks, m.NoSuite, tc.units, tc.checks, tc.noSuite)
			}
		})
	}
}
