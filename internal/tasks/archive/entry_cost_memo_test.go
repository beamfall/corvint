// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"strconv"
	"testing"
)

// TCX-V0-976: EntryCost memoizes by path and size; a memoized answer is the
// one the uncached measure gives, a refusal is never memoized, and the memo
// stays bounded.
func TestTCXV0976_EntryCostMemoMatchesUncached(t *testing.T) {
	entryCosts.Clear()
	entryCostCount.Store(0)
	for i := 0; i < 200; i++ {
		path, size := "intent/tickets/T"+strconv.Itoa(i)+".json", uint64(1000+i*37)
		first, err := EntryCost(path, size)
		if err != nil {
			t.Fatal(err)
		}
		direct, err := entryCost(path, size)
		if err != nil || first != direct {
			t.Fatalf("%s: memo %+v, uncached %+v (%v)", path, first, direct, err)
		}
		if again, err := EntryCost(path, size); err != nil || again != first {
			t.Fatalf("%s: second %+v, first %+v (%v)", path, again, first, err)
		}
		// Another size of the same path is another entry.
		if other, err := EntryCost(path, size+1); err != nil || other == first {
			t.Fatalf("%s: size+1 %+v equals %+v (%v)", path, other, first, err)
		}
	}
	if n := entryCostCount.Load(); n != 400 {
		t.Fatalf("memo holds %d entries, want 400", n)
	}
	if _, err := EntryCost("", 1); err == nil {
		t.Fatal("an empty path was admitted")
	}
	if _, ok := entryCosts.Load(entryCostKey{"", 1}); ok {
		t.Fatal("a refusal was memoized")
	}
	entryCostCount.Store(entryCostsMax)
	if _, err := EntryCost("intent/tickets/over.json", 5); err != nil {
		t.Fatal(err)
	}
	if n := entryCostCount.Load(); n != 1 {
		t.Fatalf("a full memo was not dropped: %d entries", n)
	}
}
