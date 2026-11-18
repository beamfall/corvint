// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/affectedcheck"
)

type runtimeWitnessLanguage struct{ units []affected.Unit }

func (runtimeWitnessLanguage) Name() string       { return "go" }
func (runtimeWitnessLanguage) Owns(p string) bool { return strings.HasSuffix(p, ".go") }
func (l runtimeWitnessLanguage) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: l.units}, nil
}

func TestRuntimeWideningConsumersAndWitnesses_CINV1941(t *testing.T) {
	const holder, command, downstream, reader, unrelated = "go:holder", "go:cmd", "go:downstream", "go:reader", "go:unrelated"
	const consumer = "declared-consumer:integration/run.sh"
	run := []string{"sh", "integration/run.sh"}
	// The command has no tests. Its declared consumers must still participate
	// in the closure, and a consumer without an argv must remain uncertain.
	language := runtimeWitnessLanguage{units: []affected.Unit{
		{ID: holder, Sources: []string{"holder/main.go"}, Tests: []string{"holder/main_test.go"}},
		{ID: command, Sources: []string{"cmd/main.go"}, Imports: []string{holder}, ExecConsumers: []string{"integration/manual.sh", "integration/run.sh"}, ExecConsumerRuns: map[string][]string{"integration/run.sh": run}},
		{ID: downstream, Sources: []string{"downstream/main.go"}, Tests: []string{"downstream/main_test.go"}, TestImports: []string{holder}},
		{ID: reader, Sources: []string{"reader/main.go"}, Tests: []string{"reader/main_test.go"}, PathTokens: []string{"holder/main.go"}, ReadScoped: true, ReadScope: []string{"holder/"}},
		{ID: unrelated, Sources: []string{"unrelated/main.go"}, Tests: []string{"unrelated/main_test.go"}},
	}}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, affected.RuntimeEdgesPath), []byte(`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"holder/main_test.go","runs":"future:unknown","reason":"new runtime form"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	graph, err := affected.BuildWithRuntimeEdgeWidening(context.Background(), root, language)
	if err != nil {
		t.Fatal(err)
	}
	// An unknown source form has no ownership/build proof. Like a holder test
	// or source edit, it must retain the entire closure and its binary consumers.
	for _, dirty := range []string{"holder/main_test.go", "holder/main.go", "future/input.unrecognized"} {
		t.Run(dirty, func(t *testing.T) {
			plan := affected.Select(graph.WithChangeStatus(map[string]affected.ChangeStatus{dirty: affected.ChangeModified}), []string{dirty})
			var ids []string
			for _, selection := range plan.Selected {
				ids = append(ids, selection.UnitID)
				if !affectedcheck.WitnessResolves(graph, selection) {
					t.Fatalf("independent witness validation failed: %+v", selection)
				}
				if selection.UnitID == consumer {
					if !slices.Equal(selection.Run, run) || !slices.Equal(selection.Tests, []string{"integration/run.sh"}) {
						t.Fatalf("declared runner changed: %+v", selection)
					}
				} else {
					chain, reason := graph.UnboundedChain(selection.UnitID)
					if selection.Witness.Kind != affected.WitnessUnboundedReader || selection.Witness.Cause == nil || !slices.Equal(chain, []string{selection.UnitID}) || reason != selection.Witness.Cause.Reads || !slices.Contains(graph.Frontier(), reason) {
						t.Fatalf("degradation absent from unbounded model: %+v", selection)
					}
				}
			}
			slices.Sort(ids)
			if !slices.Equal(ids, []string{consumer, downstream, holder, reader}) {
				t.Fatalf("closure = %v", ids)
			}
			if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
				return u.Reason == affected.UnknownUnselectedBinaryConsumer && u.Detail == "integration/manual.sh"
			}) {
				t.Fatalf("unrun consumer uncertainty lost: %+v", plan.Unknown)
			}
		})
	}
	// Closed witness admission: an arbitrary unbounded-read cause is not a
	// runtime degradation and cannot certify that a command must be rebuilt.
	language.units[1].UnboundedReads = "future:unknown-proof-form"
	plain, err := affected.Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	forged := affected.Selection{UnitID: consumer, Kind: "DECLARED_CONSUMER", Tests: []string{"integration/run.sh"}, Run: run, Witness: affected.Witness{Kind: "DECLARED_CONSUMER", DirtyPath: "future/input.unrecognized", Via: []string{command, consumer}}}
	if affectedcheck.WitnessResolves(plain, forged) {
		t.Fatal("unknown unbounded cause certified a consumer build")
	}
	for _, selection := range affected.Select(plain, []string{"holder/main_test.go"}).Selected {
		if selection.UnitID == consumer {
			t.Fatal("ordinary holder test change rebuilt the binary without degradation")
		}
	}
}
