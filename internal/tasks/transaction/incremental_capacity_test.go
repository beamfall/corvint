package transaction

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// cycled returns in after n UNPAUSE/PAUSE pairs: 2n more receipts and
// request index files, still paused.
func cycled(t *testing.T, in Input, n int) Input {
	t.Helper()
	for x := 0; x < n; x++ {
		for _, op := range []string{Unpause, Pause} {
			r := Model(admin(op, fmt.Sprintf("cycle-%s-%d", op, x)), in)
			if r.Kind != "Transaction" {
				t.Fatal(op, x, r)
			}
			in = commitModel(t, in, r.Plan)
		}
	}
	return in
}

// TestJSGV0003_SummarizedCapacityMeasuresOnlyChangedEntries checks that a
// summarized inventory's capacity forks charge only the entries a plan
// changes (JSG-V0-003, JSG-V0-014): the count of entry measurements is the
// same at two history sizes and two explicit inventory sizes, the prefixes
// equal the whole measurement's, and the base is unchanged afterwards.
func TestJSGV0003_SummarizedCapacityMeasuresOnlyChangedEntries(t *testing.T) {
	counts := map[string]uint64{}
	for _, n := range []int{4, 16} {
		in := cycled(t, paused(t), n)
		h, e := snapshot.DecodeHead(in.Head)
		if e != nil {
			t.Fatal(e)
		}
		for _, k := range []uint64{1, h.LastSeq.Uint64()} {
			name := fmt.Sprintf("n=%d k=%d", n, k)
			s := in
			s.Inventory = summarize(t, in.Inventory, k)
			if s.Inventory.incremental == nil {
				t.Fatal(name, "summarized inventory is not incremental")
			}
			whole := in
			whole.Inventory = summarize(t, in.Inventory, k)
			whole.Inventory.incremental = nil
			// UNPAUSE removes the barrier as well as posting.
			r, w := Model(admin(Unpause, "inc"), s), Model(admin(Unpause, "inc"), whole)
			if r.Kind != "Transaction" || w.Kind != "Transaction" {
				t.Fatal(name, r, w)
			}
			files, dirs, cost := s.Inventory.explicitFiles(), s.Inventory.Directories(), *s.Inventory.incremental
			before := entryMeasurements.Load()
			got, e := CheckCapacity(r.Plan)
			if e != nil {
				t.Fatal(name, e)
			}
			counts[name] = entryMeasurements.Load() - before
			want, e := CheckCapacity(w.Plan)
			if e != nil {
				t.Fatal(name, e)
			}
			if !reflect.DeepEqual(got.Prefixes, want.Prefixes) {
				t.Fatalf("%s: incremental capacity differs\n got %+v\nwant %+v", name, got.Prefixes, want.Prefixes)
			}
			if got.Final != nil || want.Final == nil {
				t.Fatal(name, "a summarized fork returned its shared inventory")
			}
			if s.Inventory.Incomplete() || !reflect.DeepEqual(s.Inventory.explicitFiles(), files) || !reflect.DeepEqual(s.Inventory.Directories(), dirs) || *s.Inventory.incremental != cost {
				t.Fatal(name, "capacity fork leaked into its base inventory")
			}
		}
	}
	var first uint64
	for name, c := range counts {
		if first == 0 {
			first = c
		}
		// Each changed entry is measured old and new, plus the reserve
		// receipt and request of the UNPAUSE peak; a handful per plan.
		if c == 0 || c != first || c > 32 {
			t.Fatalf("%s: %d entry measurements, want one count independent of size: %v", name, c, counts)
		}
	}
}
