// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testrunner/contract"
	"github.com/Beamfall/corvint/internal/testrunner/killswitch"
)

// slfv1007Switch parses one CORVINT_SELECTION value.
func slfv1007Switch(t *testing.T, value string) killswitch.Switch {
	t.Helper()
	parsed, err := killswitch.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// SLF-V1-007: one function maps each plan.unknown entry to one effect
// (ignored, adds-suite:<runner> or forces-full) under the allowlist corvint
// test applies after owner decision R32, an entry it does not know forces
// full, and every entry carries a non-empty hint. Build reads the same
// decision, so its scope is FULL exactly when the verdict's is full and each
// adds-suite runner gets its declared suite (or, with none declared, an
// INCOMPLETE reason naming it).
func TestSelectionVerdictEffectPerUnknown_SLFV1007(t *testing.T) {
	cases := []struct {
		name   string
		entry  Unknown
		effect string
		edit   func(*Input)
	}{
		{"go no selectable test", Unknown{Reason: "NO_SELECTABLE_TEST", Detail: "go:example.test/m/a"}, EffectIgnored, nil},
		{"driver no selectable test", Unknown{Reason: "NO_SELECTABLE_TEST", Detail: "driver:make:gate"}, EffectIgnored, nil},
		{"python no selectable test", Unknown{Reason: "NO_SELECTABLE_TEST", Detail: "python:pkg/mod.py"}, EffectAddsSuite + "python", nil},
		{"typescript discovery frontier", Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "typescript:executable-config-unresolved:vitest.config.ts:1 `include`"}, EffectAddsSuite + "typescript", nil},
		{"go build constraint frontier", Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "go:build-constraint-variants"}, EffectIgnored, nil},
		{"driver frontier", Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "driver:workflow-unparsed:ci.yml"}, EffectIgnored, nil},
		{"go frontier off the allowlist", Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "go:cgo:a/a.go"}, EffectForcesFull, nil},
		{"nested module", Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "go:unobserved-module:nested:nested/x.go"}, EffectAddsSuite + "go", nil},
		{"binary consumer", Unknown{Reason: "UNSELECTED_BINARY_CONSUMER", Detail: "script/smoke.sh"}, EffectIgnored, nil},
		{"flow binding", Unknown{Reason: "E2E_FLOWS_INVALID", Detail: ".corvint/e2e-flows.json"}, EffectIgnored, nil}, //corvint:fixture-path
		{"attributed unowned path", Unknown{Reason: "UNOWNED_DIRTY_PATH", Detail: "notes/plan.txt"}, EffectIgnored, nil},
		{"unattributed unowned path", Unknown{Reason: "UNOWNED_DIRTY_PATH", Detail: "notes/plan.txt"}, EffectForcesFull, func(input *Input) {
			input.GoSelection, input.Universe = "NOTHING no Go universe", nil
		}},
		{"code the function does not know", Unknown{Reason: "SOMETHING_NEW", Detail: "x"}, EffectForcesFull, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := fixtureInput()
			input.Unknown = []Unknown{test.entry}
			if test.edit != nil {
				test.edit(&input)
			}
			verdict := input.SelectionVerdict()
			if len(verdict.Unknown) != 1 || verdict.Unknown[0].Effect != test.effect || strings.TrimSpace(verdict.Unknown[0].Hint) == "" {
				t.Fatalf("verdict %+v, want effect %s with a hint", verdict, test.effect)
			}
			manifest, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if full := manifest.Scope == ScopeFull; full != (verdict.Scope == VerdictFull) {
				t.Fatalf("Build scope %s (%s) disagrees with verdict scope %s", manifest.Scope, manifest.Reason, verdict.Scope)
			}
			if runner, ok := strings.CutPrefix(test.effect, EffectAddsSuite); ok && runner != "go" {
				if !slices.ContainsFunc(manifest.Units, func(unit Unit) bool { return unit.Unit == "declared:"+runner }) &&
					!slices.Contains(manifest.NoSuite, runner) && !slices.ContainsFunc(manifest.Incomplete, func(reason string) bool { return strings.HasPrefix(reason, runner+": ") }) {
					t.Fatalf("adds-suite:%s but the manifest neither runs nor names its suite: %+v", runner, manifest)
				}
			}
		})
	}
}

// SLF-V1-007: an UNOWNED_DIRTY_PATH is ignored only when the Go selector
// attributed it, read from the AFP-V0-012 verdict; with a fallback verdict,
// or with none (a projection that runs no selector), it forces full. The
// plan's own hint wins over the effect's.
func TestSelectionVerdictAttributionAndHints_SLFV1007(t *testing.T) {
	entry := Unknown{Reason: "UNOWNED_DIRTY_PATH", Detail: "notes/plan.txt"}
	input := fixtureInput()
	input.Unknown = []Unknown{entry}
	if got := input.SelectionVerdict(); got.Unknown[0].Effect != EffectIgnored || got.Scope != VerdictNarrowed {
		t.Fatalf("a run verdict over a Go universe: %+v", got)
	}
	input.GoSelection = "FALLBACK the selection is empty"
	if got := input.SelectionVerdict(); got.Unknown[0].Effect != EffectForcesFull || got.Scope != VerdictFull {
		t.Fatalf("a fallback verdict: %+v", got)
	}
	projected := Input{Unknown: []Unknown{entry}}.SelectionVerdict()
	if projected.Unknown[0].Effect != EffectForcesFull || projected.Scope != VerdictFull || projected.Reason != "no AFP-V0-012 attribution verdict" {
		t.Fatalf("no Go verdict: %+v", projected)
	}
	hinted := fixtureInput()
	hinted.Unknown = []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "go:x", Hint: "  the plan's hint "}}
	if got := hinted.SelectionVerdict(); got.Unknown[0].Hint != "the plan's hint" {
		t.Fatalf("hint %q", got.Unknown[0].Hint)
	}
}

// SLF-V1-007: each runner gets one R16 tier, the one of the execution its
// selections get: line for Go when line evidence narrowed the plan, file
// otherwise; flow for a file a declared Playwright provider runs; full for a
// unit its language's declared full command runs, for a runner whose suite an
// entry adds, and for every runner once the scope is full. Driver and
// declared-consumer units name no runner.
func TestSelectionVerdictRunnerTiers_SLFV1007(t *testing.T) {
	selected := []string{"go:example.test/m/a", "typescript:playwright:e2e/a.spec.ts", "ruby:spec/a_spec.rb", "driver:make:gate", "declared-consumer:script/smoke.sh"}
	frontier := Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "python:dynamic-import:pkg/a.py"}
	lines := map[string]LineUnit{"example.test/m/a": {Mode: "tests", Pattern: "^TestA$", Reason: "selected"}}
	with := func(edit func(*Input)) SelectionVerdict {
		input := fixtureInput()
		input.Selected = selected
		edit(&input)
		return input.SelectionVerdict()
	}
	got := with(func(input *Input) {
		input.Unknown, input.Declared, input.LineUnits = []Unknown{frontier}, withProviders(), lines
	})
	want := SelectionVerdict{Scope: VerdictNarrowed, Unknown: []EffectEntry{{Reason: frontier.Reason, Detail: frontier.Detail, Effect: EffectAddsSuite + "python", Hint: got.Unknown[0].Hint}},
		Runners: []RunnerVerdict{{"go", "line"}, {"python", "full"}, {"ruby", "full"}, {"typescript", "flow"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if file := with(func(input *Input) { input.Selected = selected[:1] }); !reflect.DeepEqual(file.Runners, []RunnerVerdict{{"go", "file"}}) {
		t.Fatalf("without line evidence %+v", file.Runners)
	}
	if bare := with(func(input *Input) { input.Selected = selected[1:2] }); !reflect.DeepEqual(bare.Runners, []RunnerVerdict{{"typescript", "full"}}) {
		t.Fatalf("a Playwright file with no declared provider runs the declared typescript suite: %+v", bare.Runners)
	}
	full := with(func(input *Input) {
		input.Unknown, input.LineUnits = []Unknown{{Reason: "SOMETHING_NEW", Detail: "x"}}, lines
	})
	for _, runner := range full.Runners {
		if runner.Tier != "full" {
			t.Fatalf("forces-full leaves %+v", full.Runners)
		}
	}
	if full.Scope != VerdictFull || full.Reason == "" || len(full.Runners) != 3 || !reflect.DeepEqual(full.EffectCounts(), []string{"forces-full=1"}) {
		t.Fatalf("full %+v counts %q", full, full.EffectCounts())
	}
	if empty := with(func(input *Input) { input.Selected = nil }); empty.Unknown == nil || empty.Runners == nil || empty.Scope != VerdictNarrowed || empty.Reason != "" {
		t.Fatalf("empty %+v: members must encode as arrays", empty)
	}
}

// SLF-V1-007: corvint test's verdict comes from the scope decision Build
// applies, so every FULL cause that is not a plan.unknown entry, a removed
// runner, the `full` kill switch (RNC-V1-008), no plan, an unreadable
// declaration and the Go selector's fallback,
// gives the full verdict with Build's own FULL reason, and a narrowed Build
// gives the narrowed verdict.
func TestSelectionVerdictFullCausesMatchBuild_SLFV1007(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Input)
		scope string
	}{
		{"narrowed", func(*Input) {}, VerdictNarrowed},
		{"runner removed", func(input *Input) { input.Removed = []string{"jest@."} }, VerdictFull},
		{"kill switch full", func(input *Input) { input.KillSwitch = slfv1007Switch(t, "full") }, VerdictFull},
		{"kill switch full after a removed runner", func(input *Input) {
			input.Removed, input.KillSwitch = []string{"jest@."}, slfv1007Switch(t, "full")
		}, VerdictFull},
		{"kill switch for one runner stays narrowed", func(input *Input) { input.KillSwitch = slfv1007Switch(t, "full:pytest") }, VerdictNarrowed},
		{"no plan", func(input *Input) { input.PlanError = "graph-unavailable: no graph" }, VerdictFull},
		{"declaration unreadable", func(input *Input) { input.Declared.Unreadable = "permission denied" }, VerdictFull},
		{"go selector fallback", func(input *Input) { input.GoSelection = "FALLBACK the selection is empty while 1 path(s) are dirty" }, VerdictFull},
		{"no go selector verdict", func(input *Input) { input.GoSelection = "" }, VerdictFull},
		{"selected package outside the universe", func(input *Input) { input.GoSelection = "run example.test/m/z" }, VerdictFull},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := fixtureInput()
			input.Selected = []string{"go:example.test/m/a", "python:tests/test_a.py"}
			test.edit(&input)
			verdict := input.SelectionVerdict()
			manifest, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if verdict.Scope != test.scope || (manifest.Scope == ScopeFull) != (verdict.Scope == VerdictFull) {
				t.Fatalf("verdict %+v, Build %s (%s), want %s", verdict, manifest.Scope, manifest.Reason, test.scope)
			}
			if verdict.Scope == VerdictFull {
				if verdict.Reason == "" || verdict.Reason != manifest.Reason {
					t.Fatalf("verdict reason %q, Build reason %q", verdict.Reason, manifest.Reason)
				}
				for _, runner := range verdict.Runners {
					if runner.Tier != "full" {
						t.Fatalf("full verdict leaves %+v", verdict.Runners)
					}
				}
			}
		})
	}
}

// SLF-V1-007: a tier describes what Build runs. A Python unit with no
// frontier runs the declared full pytest command, so python is full; a
// Vitest file runs through the provider's whole-directory suite, so
// typescript is full; a Playwright file runs through its provider alone, so
// typescript is flow; the ci profile runs no provider.
func TestSelectionVerdictTierIsTheExecution_SLFV1007(t *testing.T) {
	cases := []struct {
		name     string
		selected string
		profile  string
		unit     string
		tier     string
	}{
		{"python unit runs the declared full command", "python:tests/test_a.py", ProfileLocal, "declared:python", "full"},
		{"vitest provider runs its directory's suite", "typescript:vitest:web/a.test.ts", ProfileLocal, "typescript:vitest:web/a.test.ts", "full"},
		{"playwright provider runs the selected file", "typescript:playwright:e2e/a.spec.ts", ProfileLocal, "typescript:playwright:e2e/a.spec.ts", "flow"},
		{"ci profile runs no provider", "typescript:playwright:e2e/a.spec.ts", ProfileCI, "", "full"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := fixtureInput()
			input.Declared = withProviders()
			declare(&input, pythonArgv, typescriptArgv)
			input.RunnerProfile = test.profile
			input.Selected = []string{test.selected}
			verdict := input.SelectionVerdict()
			var tier string
			for _, runner := range verdict.Runners {
				if strings.HasPrefix(test.selected, runner.Runner+":") {
					tier = runner.Tier
				}
			}
			if verdict.Scope != VerdictNarrowed || tier != test.tier {
				t.Fatalf("verdict %+v, want %s narrowed at %s", verdict, test.selected, test.tier)
			}
			manifest, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if test.unit != "" && !slices.ContainsFunc(manifest.Units, func(unit Unit) bool { return unit.Unit == test.unit }) {
				t.Fatalf("Build runs no %s: %v", test.unit, otherUnits(manifest))
			}
		})
	}
}

// SLF-V1-007, RNC-V1-019, RNC-V1-008, RNC-V1-002: the unit kinds the runner
// contract and the kill switch add keep the tier the execution has. A
// contract unit runs its selected files in their owning instance, file; an
// instance whose inventory record is not current runs declared:<instance>,
// and so does one whose runner `full:<runner>` names, so that runner is
// full while the scope stays narrowed; `full:go` runs the whole universe and
// each nested module (go-module:<dir>), so go is full.
func TestSelectionVerdictContractAndKillSwitchTiers_SLFV1007(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(*Input)
		runners []RunnerVerdict
		unit    string
	}{
		{"contract units are file", func(*Input) {}, []RunnerVerdict{{"python", "file"}, {"ruby", "file"}, {"typescript", "file"}}, "pytest:services/a/tests/test_calc.py"},
		{"an instance without a current inventory is full", func(input *Input) {
			input.FullInstances = map[string]string{"jest@.": "inventory-absent"}
		}, []RunnerVerdict{{"python", "file"}, {"ruby", "file"}, {"typescript", "full"}}, "declared:jest@."},
		{"full:pytest makes python full", func(input *Input) {
			input.KillSwitch = slfv1007Switch(t, "full:pytest")
		}, []RunnerVerdict{{"python", "full"}, {"ruby", "file"}, {"typescript", "file"}}, "declared:pytest@services/a"},
		{"full:pytest with no selected python file still runs it", func(input *Input) {
			input.Selected, input.KillSwitch = []string{"ruby:rspec:spec/a_spec"}, slfv1007Switch(t, "full:pytest")
		}, []RunnerVerdict{{"python", "full"}, {"ruby", "file"}}, "declared:pytest@services/a"},
		{"full:go runs nested modules whole", func(input *Input) {
			input.Selected, input.KillSwitch = []string{"ruby:rspec:spec/a_spec"}, slfv1007Switch(t, "full:go")
			input.Instances = append(input.Instances, contract.Instance{Runner: "go", Root: "tools/x", Config: "tools/x/go.mod"})
		}, []RunnerVerdict{{"go", "full"}, {"ruby", "file"}}, "go-module:tools/x"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := contractInput()
			test.edit(&input)
			verdict := input.SelectionVerdict()
			if verdict.Scope != VerdictNarrowed || !reflect.DeepEqual(verdict.Runners, test.runners) {
				t.Fatalf("verdict %+v, want narrowed %+v", verdict, test.runners)
			}
			manifest, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if manifest.Scope != ScopeNarrowed || !slices.ContainsFunc(manifest.Units, func(unit Unit) bool { return unit.Unit == test.unit }) {
				t.Fatalf("Build %s runs no %s: %q", manifest.Scope, test.unit, contractUnitKeys(manifest))
			}
		})
	}
}
