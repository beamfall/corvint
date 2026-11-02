package contextindex

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

// TestTestLinkTableEqualsMaps_IDXSNAPV0809 proves the snapshot's test-link
// table answers like the per-packet maps: every lookup on every indexed name,
// path and token, the candidates and frame candidates of every tracked path
// as an anchor, and the whole task-context packet for the shared queries and
// for subjects, are identical with and without the table; the table survives
// its binary round trip (IDX-SNAP-V0-809).
func TestTestLinkTableEqualsMaps_IDXSNAPV0809(t *testing.T) {
	for name, index := range symbolTermIndexes(t) {
		table := buildTestLinkTable(index.Symbols)
		data, err := table.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		decoded := &testLinkTable{}
		if err := decoded.UnmarshalBinary(data); err != nil || !reflect.DeepEqual(decoded, table) {
			t.Fatalf("%s: table round trip: %v", name, err)
		}
		slow, fast := *index, *index
		slow.testLinks, fast.testLinks = nil, decoded
		maps := buildTestLinkMaps(index.Symbols)
		withTable := &testLinker{links: decoded}
		for key, want := range maps.declared {
			if got := withTable.declaredOf(key); !slices.Equal(got, want) {
				t.Fatalf("%s: declared %q = %v, want %v", name, key, got, want)
			}
		}
		for key, want := range maps.declaredBy {
			if got := withTable.declaredByOf(key); !slices.Equal(got, want) {
				t.Fatalf("%s: declaredBy %q = %v, want %v", name, key, got, want)
			}
		}
		for key, want := range maps.declaredTokens {
			if got := withTable.declaredTokensOf(key); !slices.Equal(got, want) {
				t.Fatalf("%s: declaredTokens %q = %v, want %v", name, key, got, want)
			}
		}
		for key, want := range maps.declaredByToken {
			if got := withTable.declaredByTokenOf(key); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
				t.Fatalf("%s: declaredByToken %q = %v, want the sorted %v", name, key, got, want)
			}
		}
		for key, want := range maps.testNames {
			if got := withTable.testNamesOf(key); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: testNames %q differ", name, key)
			}
		}
		for _, probe := range []string{"", "zz-missing", "Demux"} {
			if withTable.declaredOf(probe) != nil && maps.declared[probe] == nil {
				t.Fatalf("%s: declared %q found only in the table", name, probe)
			}
		}

		slowCompiler := newTaskContextCompiler(&slow, "test linking", "")
		fastCompiler := newTaskContextCompiler(&fast, "test linking", "")
		slowLinker, fastLinker := slowCompiler.newTestLinker(), fastCompiler.newTestLinker()
		if fastLinker.links == nil || slowLinker.links != nil {
			t.Fatalf("%s: the table was not taken", name)
		}
		anchors := make([]string, 0, len(slowLinker.tracked))
		for path := range slowLinker.tracked {
			anchors = append(anchors, path)
		}
		slices.Sort(anchors)
		// Both linkers are read-only once their frame indexes exist, so the
		// anchors are compared on every CPU.
		fastLinker.frameIndex()
		slowLinker.frameIndex()
		sortedNames := func(names []testName) []testName {
			names = slices.Clone(names)
			slices.SortFunc(names, func(left, right testName) int {
				if left.name != right.name {
					return compareStrings(left.name, right.name)
				}
				return compareStrings(left.path, right.path)
			})
			return names
		}
		byPath := func(candidates []*testCandidate) []*testCandidate {
			slices.SortFunc(candidates, func(left, right *testCandidate) int { return compareStrings(left.path, right.path) })
			return candidates
		}
		mismatch := make([]string, len(anchors))
		forEachAnchor(len(anchors), func(position int) {
			anchor := anchors[position]
			switch {
			case !reflect.DeepEqual(sortedNames(fastLinker.testNamesAt(anchor)), sortedNames(slowLinker.testNamesAt(anchor))):
				mismatch[position] = "test names at " + anchor
			case !reflect.DeepEqual(fastLinker.candidates(anchor), slowLinker.candidates(anchor)):
				mismatch[position] = "candidates of " + anchor
			// frameCandidates returns map order; its caller ranks by outranks.
			case !reflect.DeepEqual(byPath(fastLinker.frameCandidates(anchor)), byPath(slowLinker.frameCandidates(anchor))):
				mismatch[position] = "frame candidates of " + anchor
			}
		})
		for _, problem := range mismatch {
			if problem != "" {
				t.Fatalf("%s: %s differ", name, problem)
			}
		}
		for _, text := range symbolTermQueries {
			for _, subject := range []string{"", anchors[0], anchors[len(anchors)/2]} {
				slowPacket, slowErr := TaskContext(context.Background(), &slow, text, subject, 20)
				fastPacket, fastErr := TaskContext(context.Background(), &fast, text, subject, 20)
				if (slowErr == nil) != (fastErr == nil) || slowErr == nil && mustJSON(t, fastPacket) != mustJSON(t, slowPacket) {
					t.Fatalf("%s %q subject %q: task-context packets differ", name, text, subject)
				}
			}
		}
	}
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

// TestTestLinkTableRefusals_IDXSNAPV0809: a table built for another symbol
// count is not used, a truncated or out-of-range table is refused when
// decoded, and a written snapshot loads with the table equal to a fresh build.
func TestTestLinkTableRefusals_IDXSNAPV0809(t *testing.T) {
	index := taskContextFixture(t)
	table := buildTestLinkTable(index.Symbols)
	stale := *table
	stale.symbols++
	if stale.usable(index) || (*testLinkTable)(nil).usable(index) || !table.usable(index) {
		t.Fatal("usable is wrong")
	}
	data, _ := table.MarshalBinary()
	for _, broken := range [][]byte{data[:len(data)-1], append(append([]byte{}, data...), 0), data[:3]} {
		if err := (&testLinkTable{}).UnmarshalBinary(broken); err == nil {
			t.Fatalf("a malformed table of %d bytes decoded", len(broken))
		}
	}
	if len(table.definerIDs) == 0 {
		t.Fatal("fixture has no definers")
	}
	forged := *table
	forged.definerIDs = slices.Clone(table.definerIDs)
	forged.definerIDs[0] = uint32(len(table.paths))
	bad, _ := forged.MarshalBinary()
	if err := (&testLinkTable{}).UnmarshalBinary(bad); err == nil {
		t.Fatal("a table naming a path outside its list decoded")
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit || loaded.testLinks == nil || !reflect.DeepEqual(loaded.testLinks, buildTestLinkTable(loaded.Symbols)) {
		t.Fatalf("the loaded snapshot does not carry the table: hit=%v err=%v", hit, err)
	}
}
