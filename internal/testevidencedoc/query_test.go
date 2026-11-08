// SPDX-License-Identifier: Apache-2.0
package testevidencedoc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func fixture(t *testing.T) (runnerselection.Input, testevidence.Record) {
	t.Helper()
	data, e := os.ReadFile("testdata/record.json")
	if e != nil {
		t.Fatal(e)
	}
	r, e := testevidence.Decode(data)
	if e != nil {
		t.Fatal(e)
	}
	b := testevidence.Blob{Path: testevidence.Path{Repository: r.Repository, Lexical: "src.go", Resolved: "src.go"}, OID: strings.Repeat("2", 40)}
	r.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present, Items: []testevidence.ExecutedFile{{Blob: b, Executed: []testevidence.Range{{Start: 2, StartColumn: 1, End: 4, EndColumn: 1}}}}}
	r.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}
	r.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	r.Reads = testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{b}}
	r.Listings = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Lookups = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Execs = testevidence.Component[testevidence.Executable]{State: testevidence.Present}
	r.External = testevidence.Component[testevidence.Contact]{State: testevidence.Present}
	head := strings.Repeat("9", 40)
	key := testrunner.InventoryKey{Checkout: head, Runner: r.Runner.Plugin, ConfigDigest: r.Runner.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	rr := runnerselection.Runner{Name: r.Runner.Plugin, Identity: r.Runner, Execution: r.Execution,
		Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: "suite/test"}}},
		Pieces:        testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Present: true}},
		Qualification: runnerselection.Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []runnerselection.Class{runnerselection.BodyEdit, runnerselection.DeclarationEdit, runnerselection.DataEdit}, Runner: r.Runner, Execution: r.Execution},
		Records:       []runnerselection.Evidence{{Record: r, Admitted: true}}, IndexBase: r.Commit, IndexVerified: true}
	return runnerselection.Input{Base: r.Commit, Head: head, Repository: r.Repository, Repositories: map[string]bool{r.Repository: true}, DiffComplete: true, DirectoriesComplete: true, BaseDirectories: map[testevidence.PathKey]bool{{Repository: r.Repository, Path: "."}: true}, Snapshot: testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{{Repository: r.Repository, Path: "src.go"}: b.OID}}, Runners: []runnerselection.Runner{rr}}, r
}

func TestRSPV0051QueriesPositiveAndNegative(t *testing.T) {
	for _, tc := range []struct {
		name       string
		q          Query
		components []string
	}{{"path", Query{Path: "src.go"}, []string{"executed", "reads"}}, {"range", Query{Path: "src.go", Start: 3, End: 3}, []string{"executed", "reads"}}, {"end-exclusive", Query{Path: "src.go", Start: 4, End: 4}, []string{"reads"}}, {"test", Query{Test: "suite/test"}, []string{}}, {"miss", Query{Path: "other.go"}, []string{}}} {
		t.Run(tc.name, func(t *testing.T) {
			in, r := fixture(t)
			q := tc.q
			q.Revision = in.Head
			d := Project(q, in, []testevidence.Record{r}, "")
			if d.Status != "OBSERVED" {
				t.Fatalf("%+v", d)
			}
			got := []string{}
			for _, m := range d.Matches {
				got = append(got, m.Component)
				if len(m.RecordDigest) != 64 || m.RecordCommit != r.Commit {
					t.Fatal(m)
				}
			}
			if !slices.Equal(got, tc.components) {
				t.Fatalf("%v != %v", got, tc.components)
			}
			if q.Test != "" && (len(d.Records) != 1 || d.Records[0].Record.Reads.State != testevidence.Present) {
				t.Fatal(d)
			}
		})
	}
}

func TestRSPV0051Abstentions(t *testing.T) {
	for _, reason := range []string{"inventory-unavailable", "run-context-unresolved", "no-fresh-record", "stale-record", "malformed-record", "identity-missing"} {
		t.Run(reason, func(t *testing.T) {
			in, r := fixture(t)
			records := []testevidence.Record{r}
			q := Query{Test: r.Test, Revision: in.Head}
			switch reason {
			case "inventory-unavailable":
				in.Runners[0].Inventory.Complete = false
			case "run-context-unresolved":
				in.Runners[0].Execution.EnvironmentDigest = ""
			case "no-fresh-record":
				records = nil
			case "stale-record":
				in.Snapshot.Blobs = nil
			case "malformed-record":
				records[0].CollectionCount = 0
			case "identity-missing":
				records[0].Runner = testevidence.RunnerIdentity{}
			}
			d := Project(q, in, records, "")
			if d.Status != "UNKNOWN" || !slices.Contains(d.Reasons, reason) || len(d.Matches) != 0 {
				t.Fatalf("%+v", d)
			}
			if reason == "stale-record" && d.Records[0].FreshnessReason != "blob-drift-or-missing" {
				t.Fatal(d)
			}
		})
	}
	in, r := fixture(t)
	q := Query{Path: "src.go", Revision: in.Head}
	d := Project(q, in, []testevidence.Record{r}, "record-cap-exceeded")
	if d.Status != "UNKNOWN" || !slices.Contains(d.Reasons, "record-cap-exceeded") {
		t.Fatal(d)
	}
}

func TestRSPV0051ExplainSelector(t *testing.T) {
	in, r := fixture(t)
	q := Query{Explain: r.Test, Revision: in.Head, Base: in.Base}
	for _, tc := range []struct {
		name, path string
		full       bool
	}{{"selected", "src.go", false}, {"excluded", "other.go", false}, {"unqualified", "other.go", true}} {
		t.Run(tc.name, func(t *testing.T) {
			in.Changes = []runnerselection.Change{{Repository: r.Repository, Path: tc.path, Kind: "modify", Decoded: true}}
			in.Runners[0].Qualification.Admitted = !tc.full
			in.Runners[0].Qualification.Passed = !tc.full
			d := Project(q, in, []testevidence.Record{r}, "")
			p := d.Explanation.Runners[0]
			switch tc.name {
			case "selected":
				if len(p.Selected) != 1 || !slices.Contains(p.Selected[0].Rules, "rule:d") {
					t.Fatal(p)
				}
			case "excluded":
				if len(p.Excluded) != 1 || len(p.Selected) != 0 {
					t.Fatal(p)
				}
			case "unqualified":
				if p.Mode != "full" || len(p.Excluded) != 0 || !slices.Contains(d.Reasons, "unqualified") {
					t.Fatal(d)
				}
			}
		})
	}
}

func TestRSPV0051Goldens(t *testing.T) {
	for _, shape := range []string{"path", "range", "test", "explain"} {
		t.Run(shape, func(t *testing.T) {
			in, r := fixture(t)
			q := Query{Revision: in.Head}
			switch shape {
			case "path":
				q.Path = "src.go"
			case "range":
				q.Path = "src.go"
				q.Start = 3
				q.End = 3
			case "test":
				q.Test = r.Test
			case "explain":
				q.Explain = r.Test
				q.Base = in.Base
				in.Runners = []runnerselection.Runner{{Name: r.Runner.Plugin}}
			}
			d := Project(q, in, []testevidence.Record{r}, "")
			for _, format := range []string{"json", "text"} {
				var got bytes.Buffer
				if e := Write(&got, d, format); e != nil {
					t.Fatal(e)
				}
				want, e := os.ReadFile(filepath.Join("testdata", shape+"."+format))
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(got.Bytes(), want) {
					t.Fatalf("%s golden differs:\n%s", format, got.String())
				}
			}
		})
	}
}

func TestRSPV0051InvalidAndCapped(t *testing.T) {
	for _, args := range [][]string{{}, {"../secret"}, {"x", "--lines", "0:0"}, {"x", "--lines", "3:2"}, {"--explain", "test"}, {"x", "--test", "test"}, {"x", "--index", "../index"}, {"x", "--format", "yaml"}, {"x", "--revision", "--all"}, {"x", "--lines", "1:999999999999999"}, {"x", "--test", ""}} {
		if _, _, err := Parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if _, _, err := Parse([]string{"src.go", "--lines=1:2"}); err != nil {
		t.Fatal(err)
	}
	in, r := fixture(t)
	records := make([]testevidence.Record, runnerselection.MaxInputs+1)
	d := Project(Query{Test: r.Test, Revision: in.Head}, in, records, "")
	if !slices.Contains(d.Reasons, "record-cap-exceeded") {
		t.Fatal(d)
	}
	r.Test = strings.Repeat("x", MaxResultBytes)
	d = Project(Query{Path: "src.go", Revision: in.Head}, in, []testevidence.Record{r}, "")
	raw, _ := json.Marshal(d)
	if len(raw) > MaxResultBytes || !slices.Contains(d.Reasons, "result-cap-exceeded") {
		t.Fatalf("unbounded result %d", len(raw))
	}
}

// Incomplete reads never certify a smaller answer, even beside a fresh hit.
func TestRSPV0051IncompleteInventoryCoverage(t *testing.T) {
	in, r := fixture(t)
	q := Query{Path: "src.go", Revision: in.Head}
	in.Runners[0].Inventory.Tests = append(in.Runners[0].Inventory.Tests, testrunner.InventoryTest{ID: "suite/missing", Path: "suite/missing"})
	d := Project(q, in, []testevidence.Record{r}, "")
	if d.Status != "UNKNOWN" || !slices.Contains(d.Reasons, "no-fresh-record") || len(d.Matches) == 0 {
		t.Fatal(d)
	}
	in, r = fixture(t)
	in.Runners[0].Inventory.Tests = nil
	d = Project(q, in, []testevidence.Record{r}, "")
	if d.Status != "UNKNOWN" || !slices.Contains(d.Reasons, "inventory-unavailable") || len(d.Matches) != 0 {
		t.Fatal(d)
	}
	in, r = fixture(t)
	q = Query{Explain: r.Test, Base: in.Base, Revision: in.Head}
	d = Project(q, in, []testevidence.Record{r}, "records-unreadable")
	if d.Explanation.Runners[0].Mode != "full" || len(d.Explanation.Runners[0].Excluded) != 0 {
		t.Fatal(d)
	}
}
