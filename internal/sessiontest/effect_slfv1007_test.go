// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

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
// attributed it; with no Go selector (corvint affected, the MCP tool) the
// attribution is undecided and the entry forces full, with a hint naming
// that. The plan's own hint wins over the effect's.
func TestSelectionVerdictAttributionAndHints_SLFV1007(t *testing.T) {
	entry := Unknown{Reason: "UNOWNED_DIRTY_PATH", Detail: "notes/plan.txt"}
	attributed := SelectionVerdictOf(VerdictInput{Unknown: []Unknown{entry}, Attribution: AttributionAll})
	undecided := SelectionVerdictOf(VerdictInput{Unknown: []Unknown{entry}})
	if attributed.Unknown[0].Effect != EffectIgnored || attributed.Scope != VerdictNarrowed {
		t.Fatalf("attributed %+v", attributed)
	}
	if undecided.Unknown[0].Effect != EffectForcesFull || undecided.Scope != VerdictFull || !strings.Contains(undecided.Unknown[0].Hint, "does not decide") {
		t.Fatalf("undecided %+v", undecided)
	}
	input := fixtureInput()
	input.Unknown = []Unknown{entry}
	if got := input.SelectionVerdict().Unknown[0].Effect; got != EffectIgnored {
		t.Fatalf("corvint test with a run verdict over a Go universe: effect %s", got)
	}
	input.GoSelection = "FALLBACK the selection is empty"
	if got := input.SelectionVerdict().Unknown[0].Effect; got != EffectForcesFull {
		t.Fatalf("corvint test with a fallback verdict: effect %s", got)
	}
	hinted := SelectionVerdictOf(VerdictInput{Unknown: []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "go:x", Hint: "  the plan's hint "}}})
	if hinted.Unknown[0].Hint != "the plan's hint" {
		t.Fatalf("hint %q", hinted.Unknown[0].Hint)
	}
}

// SLF-V1-007: each runner gets one R16 tier: line for Go when line evidence
// narrowed the plan, file otherwise, flow for a Playwright selection, full
// for a runner whose suite an entry adds, and full for every runner once any
// entry forces full. Driver and declared-consumer units name no runner.
func TestSelectionVerdictRunnerTiers_SLFV1007(t *testing.T) {
	selected := []string{"go:example.test/m/a", "typescript:playwright:e2e/a.spec.ts", "typescript:src/a.test.ts", "ruby:spec/a_spec.rb", "driver:make:gate", "declared-consumer:script/smoke.sh"}
	frontier := Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "python:dynamic-import:pkg/a.py"}
	got := SelectionVerdictOf(VerdictInput{Selected: selected, Unknown: []Unknown{frontier}, LineNarrowed: true})
	want := SelectionVerdict{Scope: VerdictNarrowed, Unknown: []EffectEntry{{Reason: frontier.Reason, Detail: frontier.Detail, Effect: EffectAddsSuite + "python", Hint: got.Unknown[0].Hint}},
		Runners: []RunnerVerdict{{"go", "line"}, {"python", "full"}, {"ruby", "file"}, {"typescript", "flow"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if file := SelectionVerdictOf(VerdictInput{Selected: selected[:1]}); !reflect.DeepEqual(file.Runners, []RunnerVerdict{{"go", "file"}}) {
		t.Fatalf("without line evidence %+v", file.Runners)
	}
	full := SelectionVerdictOf(VerdictInput{Selected: selected, Unknown: []Unknown{{Reason: "SOMETHING_NEW", Detail: "x"}}, LineNarrowed: true})
	for _, runner := range full.Runners {
		if runner.Tier != "full" {
			t.Fatalf("forces-full leaves %+v", full.Runners)
		}
	}
	if full.Scope != VerdictFull || len(full.Runners) != 3 || !reflect.DeepEqual(full.EffectCounts(), []string{"forces-full=1"}) {
		t.Fatalf("full %+v counts %q", full, full.EffectCounts())
	}
	if empty := SelectionVerdictOf(VerdictInput{}); empty.Unknown == nil || empty.Runners == nil || empty.Scope != VerdictNarrowed {
		t.Fatalf("empty %+v: members must encode as arrays", empty)
	}
}
