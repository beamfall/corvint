// SPDX-License-Identifier: Apache-2.0
package registry

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gitrun"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// RSP-V0-050/055: a real script adapter record can support shadow exclusion
// under synthetic admitted qualification. This does not qualify a live runner.
func TestRSPV0050FileOnlySelection(t *testing.T) {
	root, head, _ := scriptFixture(t)
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	trace := scriptObservation()
	trace.Reads.Entries = []proctrace.Path{scriptPath("data")}
	r, err := ScriptTraceRecord(context.Background(), scriptHeader(head), trace, repo)
	if err != nil {
		t.Fatal(err)
	}
	other := strings.Repeat("9", 40)
	key := testrunner.InventoryKey{Checkout: other, Runner: "script", ConfigDigest: r.Runner.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	in := runnerselection.Input{Base: head, Head: other, Repository: "main", Repositories: map[string]bool{"main": true}, DiffComplete: true,
		Changes:             []runnerselection.Change{{Repository: "main", Path: "unrelated", Kind: "modify", Decoded: true}},
		DirectoriesComplete: true, BaseDirectories: map[testevidence.PathKey]bool{{Repository: "main", Path: "."}: true},
		Snapshot: testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{{Repository: "main", Path: "data"}: r.Reads.Items[0].OID}},
		Runners: []runnerselection.Runner{{Name: "script", Identity: r.Runner, Execution: r.Execution,
			Pieces:        testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Present: true}},
			Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: ScriptDeclarationPath}}},
			Qualification: runnerselection.Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []runnerselection.Class{runnerselection.DataEdit, runnerselection.DeleteRename}, Runner: r.Runner, Execution: r.Execution},
			Records:       []runnerselection.Evidence{{Record: r, Admitted: true}}, IndexBase: head, IndexVerified: true}}}
	assertExcluded := func() {
		t.Helper()
		p := runnerselection.Select(in)
		if !p.Shadow || len(p.Runners[0].Excluded) != 1 || len(p.Runners[0].Selected) != 0 || !reflect.DeepEqual(p.Runners[0].Excluded[0].Components, []string{"reads", "listings", "lookups", "execs", "external"}) {
			t.Fatalf("file-only exclusion failed: %+v", p)
		}
	}
	assertExcluded()
	for _, kind := range []string{"modify", "delete"} {
		in.Changes[0].Path, in.Changes[0].Kind = "data", kind
		p := runnerselection.Select(in).Runners[0]
		if len(p.Selected) != 1 || len(p.Excluded) != 0 {
			t.Fatalf("file dependency lost: %+v", p)
		}
	}
	in.Changes[0].Path, in.Changes[0].Kind = "unrelated", "modify"
	for _, name := range r.SelectionComponents() {
		for _, state := range []testevidence.State{testevidence.Absent, testevidence.Incomplete} {
			x := r
			switch name {
			case "reads":
				x.Reads = testevidence.Component[testevidence.Blob]{State: state, Reason: "not-observed"}
			case "listings":
				x.Listings = testevidence.Component[testevidence.Path]{State: state, Reason: "not-observed"}
			case "lookups":
				x.Lookups = testevidence.Component[testevidence.Path]{State: state, Reason: "not-observed"}
			case "execs":
				x.Execs = testevidence.Component[testevidence.Executable]{State: state, Reason: "not-observed"}
			case "external":
				x.External = testevidence.Component[testevidence.Contact]{State: state, Reason: "not-observed"}
			}
			in.Runners[0].Records[0].Record = x
			p := runnerselection.Select(in).Runners[0]
			if len(p.Selected) != 1 || len(p.Excluded) != 0 {
				t.Fatalf("missing %s admitted: %+v", name, p)
			}
			if ready, _ := x.CollectionReady(true); ready {
				t.Fatalf("missing %s collection ready", name)
			}
		}
	}
	x := r
	x.Executed.Reason = "future-kind"
	in.Runners[0].Records[0].Record = x
	if p := runnerselection.Select(in).Runners[0]; len(p.Selected) != 1 || len(p.Excluded) != 0 {
		t.Fatalf("unknown capability excluded: %+v", p)
	}
	b := r
	b.Sources = append([]testevidence.Producer{}, r.Sources...)
	b.Sources[0].Run = "second"
	u, err := testevidence.Union(r, b)
	if err != nil {
		t.Fatal(err)
	}
	in.Runners[0].Records[0].Record = u
	in.Runners[0].Qualification.SingleCollectionQualified = false
	assertExcluded()
}
