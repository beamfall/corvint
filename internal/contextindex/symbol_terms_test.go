package contextindex

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// symbolTermQueries cover exact names, compact-name containment, multi-term
// overlap, context-only matches, version narrowing, test intent and an empty
// query.
var symbolTermQueries = []string{
	"demux", "Demux", "split the cache key", "cache demux keys", "where is the snapshot decoded",
	"chunk the term table", "v2 api handler", "fix the failing test for split", "", "zz no such words zz",
	"LoadSnapshot", "evalPrepareSymbols ranking", "index refresh worker profile", "docs readme install",
}

func symbolTermIndexes(t *testing.T) map[string]*Index {
	t.Helper()
	indexes := map[string]*Index{"fixture": taskContextFixture(t)}
	if root := os.Getenv("CORVINT_SNAPSHOT_PARITY_ROOT"); root != "" {
		built, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		indexes["repository"] = built
	}
	return indexes
}

// TestSymbolTermTableRankingEqualsFullPass_IDXSNAPV0807 proves the
// precomputed path equals the full pass: for every query, the ranked symbol
// candidates and the term frequencies evalRankSymbols returns, the evalRank
// results and the whole task-context packet are identical with and without
// the table, and the table survives its binary round trip (IDX-SNAP-V0-807).
func TestSymbolTermTableRankingEqualsFullPass_IDXSNAPV0807(t *testing.T) {
	for name, index := range symbolTermIndexes(t) {
		index.vocabulary()
		if index.Vocabulary.SymbolWindows.keyCount() < 0 {
			index.Vocabulary.SymbolWindows = index.buildSymbolWindows()
		}
		table := buildSymbolTermTable(index)
		data, err := table.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		decoded := &symbolTermTable{}
		if err := decoded.UnmarshalBinary(data); err != nil || !reflect.DeepEqual(decoded, table) {
			t.Fatalf("%s: table round trip: %v", name, err)
		}
		slow, fast := *index, *index
		slow.symbolTerms, fast.symbolTerms = nil, decoded
		for _, text := range symbolTermQueries {
			trimmed := TrimPythonSpace(text)
			queryText := pythonLower(trimmed)
			intent := evalInferIntent(queryText)
			queryTerms := evalRelevanceTerms(terms(trimmed), intent)
			if !fast.symbolTerms.usable(&fast, fast.symbolWindowTerms(queryTerms)) {
				t.Fatalf("%s %q: the fast path is not taken", name, text)
			}
			slowSymbols, slowFrequency := evalRankSymbols(&slow, queryText, queryTerms, intent)
			fastSymbols, fastFrequency := evalRankSymbols(&fast, queryText, queryTerms, intent)
			if !reflect.DeepEqual(fastSymbols, slowSymbols) || !reflect.DeepEqual(fastFrequency, slowFrequency) {
				t.Fatalf("%s %q: ranked symbols differ (%d fast, %d slow)", name, text, len(fastSymbols), len(slowSymbols))
			}
			slowRank, slowErr := evalRank(&slow, text, 20)
			fastRank, fastErr := evalRank(&fast, text, 20)
			if (slowErr == nil) != (fastErr == nil) || slowErr == nil && !reflect.DeepEqual(mustJSON(t, fastRank.results), mustJSON(t, slowRank.results)) {
				t.Fatalf("%s %q: evalRank results differ", name, text)
			}
			slowPacket, slowErr := TaskContext(context.Background(), &slow, text, "", 20)
			fastPacket, fastErr := TaskContext(context.Background(), &fast, text, "", 20)
			if (slowErr == nil) != (fastErr == nil) || slowErr == nil && mustJSON(t, fastPacket) != mustJSON(t, slowPacket) {
				t.Fatalf("%s %q: task-context packets differ", name, text)
			}
		}
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// TestSymbolTermTableUnusableFallsBack_IDXSNAPV0807: a table built for
// another symbol count, or an index without symbol windows, takes the full
// pass; a truncated or out-of-range table is refused when decoded.
func TestSymbolTermTableUnusableFallsBack_IDXSNAPV0807(t *testing.T) {
	index := taskContextFixture(t)
	table := buildSymbolTermTable(index)
	stale := *table
	stale.symbols++
	windows := []map[string]struct{}{}
	if stale.usable(index, windows) || table.usable(index, nil) || (*symbolTermTable)(nil).usable(index, windows) {
		t.Fatal("an unusable table was used")
	}
	data, err := table.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, broken := range [][]byte{data[:len(data)-1], append(append([]byte{}, data...), 0), data[:3]} {
		if err := (&symbolTermTable{}).UnmarshalBinary(broken); err == nil {
			t.Fatalf("a malformed table of %d bytes decoded", len(broken))
		}
	}
	if len(table.nameTermIDs) == 0 {
		t.Fatal("fixture has no name terms")
	}
	outOfRange := *table
	outOfRange.nameTermIDs = append([]uint32{}, table.nameTermIDs...)
	outOfRange.nameTermIDs[0] = uint32(len(table.terms))
	forged, _ := outOfRange.MarshalBinary()
	if err := (&symbolTermTable{}).UnmarshalBinary(forged); err == nil {
		t.Fatal("a table naming a term outside its list decoded")
	}
}

// TestSnapshotCarriesSymbolTermTable_IDXSNAPV0807: a written snapshot loads
// with the table attached and equal to a fresh build; the event loads do not
// read it.
func TestSnapshotCarriesSymbolTermTable_IDXSNAPV0807(t *testing.T) {
	index := taskContextFixture(t)
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if loaded.symbolTerms == nil || !reflect.DeepEqual(loaded.symbolTerms, buildSymbolTermTable(loaded)) {
		t.Fatal("the loaded snapshot does not carry the table built from its symbols")
	}
	event, hit, err := LoadEventSnapshot(context.Background(), index.Root, false)
	if err != nil || !hit || event.symbolTerms != nil {
		t.Fatalf("event load: hit=%v err=%v table=%v", hit, err, event != nil && event.symbolTerms != nil)
	}
}
