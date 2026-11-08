// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"slices"
	"testing"
)

// RSP-V0-050/055: only the complete script marker waives native prerequisites.
func TestRSPV0050SelectionCapability(t *testing.T) {
	r := fixture()
	r.Runner.Plugin, r.Test = "script", "script:fixture"
	r.Executed = Missing[ExecutedFile]("script-kind")
	r.Regions = Missing[RegionsFile]("script-kind")
	r.Loaded = Missing[Blob]("script-kind")
	if r.SelectionCapability() != FileSelection {
		t.Fatal("script capability lost")
	}
	if ready, reason := r.CollectionReady(true); !ready {
		t.Fatal(reason)
	}
	if ready, reason := r.CollectionReady(false); ready || reason != "collection-count-insufficient" {
		t.Fatal(ready, reason)
	}
	for _, change := range []func(*Record){
		func(x *Record) { x.Executed.Reason = "future-kind" },
		func(x *Record) { x.Regions.State = Incomplete },
		func(x *Record) { x.Loaded = Component[Blob]{State: Present} },
		func(x *Record) { x.Loaded.Items = []Blob{blob()} },
	} {
		x := r
		change(&x)
		if x.SelectionCapability() == FileSelection {
			t.Fatal("unproven capability admitted")
		}
		if ready, _ := x.CollectionReady(true); ready {
			t.Fatal("unproven collection admitted")
		}
	}
	u, err := Union(r, next(r))
	if err != nil {
		t.Fatal(err)
	}
	if ready, reason := u.CollectionReady(false); !ready || u.SelectionCapability() != FileSelection {
		t.Fatal(ready, reason)
	}
	x := next(r)
	x.Executed.Reason = "future-kind"
	u, err = Union(r, x)
	if err != nil {
		t.Fatal(err)
	}
	if ready, _ := u.CollectionReady(true); ready {
		t.Fatal("mixed capability union admitted")
	}
}

// RSP-V0-050/055: the marker cannot waive another runner's native prerequisites,
// either directly or by unioning two collections carrying the same marker.
func TestRSPV0050SelectionCapabilityRunnerBinding(t *testing.T) {
	for _, tc := range []struct{ plugin, test string }{
		{"plugin", "suite/test"},
		{"go", "script:fixture"},
		{"script-other", "script:fixture"},
		{"script", "suite/test"},
	} {
		t.Run(tc.plugin+"/"+tc.test, func(t *testing.T) {
			r := fixture()
			r.Runner.Plugin, r.Test = tc.plugin, tc.test
			r.Executed = Missing[ExecutedFile]("script-kind")
			r.Regions = Missing[RegionsFile]("script-kind")
			r.Loaded = Missing[Blob]("script-kind")
			u, err := Union(r, next(r))
			if err != nil {
				t.Fatal(err)
			}
			for _, record := range []Record{r, u} {
				if record.SelectionCapability() != NativeSelection {
					t.Fatal("marker conferred file-only capability on wrong runner")
				}
				if got := record.MissingSelectionComponents(); !slices.Equal(got, []string{"executed", "regions", "loaded"}) {
					t.Fatalf("native prerequisites waived: %v", got)
				}
				if ready, reason := record.CollectionReady(true); ready || reason != "collection-components-missing" {
					t.Fatalf("unbound marker became ready: %v %s", ready, reason)
				}
			}
			if u.Executed.State != Incomplete || u.Regions.State != Incomplete || u.Loaded.State != Incomplete {
				t.Fatal("union preserved an unbound file-only marker")
			}
		})
	}
}
