package store

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-138: a carried review fold continues only across a chained prefix
// and otherwise refolds from receipt 1; every answer equals the whole-history
// fold.
func TestCALV0138_ReviewFoldCarriesOnlyAChainedPrefix(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, 120)
	full := func(last uint64) any {
		t.Helper()
		audit, err := FoldExternalReviews(repo, last, nil)
		if err != nil {
			t.Fatal(err)
		}
		return audit
	}
	var f ReviewFold
	fold := func(last uint64) any {
		t.Helper()
		audit, err := f.Fold(repo, last)
		if err != nil {
			t.Fatal(err)
		}
		if want := full(last); !reflect.DeepEqual(audit, want) {
			t.Fatalf("last %d: carried fold differs from the whole-history fold", last)
		}
		return audit
	}

	first := fold(70)
	if f.seq != 70 {
		t.Fatalf("carried seq %d", f.seq)
	}
	if again := fold(120); again != first || f.seq != 120 {
		t.Fatalf("a chained extension refolded from receipt 1 (seq %d)", f.seq)
	}
	if same := fold(120); same != first {
		t.Fatal("an unchanged head refolded from receipt 1")
	}
	if shorter := fold(100); shorter == first || f.seq != 100 {
		t.Fatal("a shorter history continued the carried fold")
	}
	carried := f.audit
	f.sum = wire.Sum([]byte("rewritten"))
	if rewritten := fold(120); rewritten == carried || f.seq != 120 {
		t.Fatal("a rewritten carried receipt continued the carried fold")
	}
	if empty := fold(0); f.audit != nil || f.seq != 0 || empty == nil {
		t.Fatal("an empty history kept carried state")
	}
}

// BenchmarkCALV0138_DispatcherTickFold guards the per-tick fold cost of an
// idle dispatcher: one receipt read and hash, not the whole history (compare
// BenchmarkCALV0135_FoldReceiptBindings).
func BenchmarkCALV0138_DispatcherTickFold(b *testing.B) {
	repo := historyStore(b, 2000)
	var f ReviewFold
	if _, err := f.Fold(repo, 2000); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := f.Fold(repo, 2000); err != nil {
			b.Fatal(err)
		}
	}
}
