package contextindex

import (
	"bytes"
	"context"
	"reflect"
	"testing"
)

// TestSymbolWindowTableEmitsIdenticalReceipts is the standing proof that
// reading a symbol's context terms out of SymbolWindows is a pure allocation
// change against scanning its window at query time. Both are a tokenisation
// of symbolContextText, so one binary emits the receipt matrix under each
// construction and compares the bytes.
func TestSymbolWindowTableEmitsIdenticalReceipts(t *testing.T) {
	pinRoot, _ := pinParityRepository(t)
	roots := append([]string{evalQueryRepository(t), pinRoot}, parityRepositoriesFromEnvironment()...)
	for _, repository := range roots {
		index := evalIndexWithSymbolWindows(t, repository)
		scanned := withScannedContextWindow(t, func() []parityReceipt { return emitParityReceipts(index) })
		tabled := emitParityReceipts(index)
		compared, states := compareParityReceipts(t, repository, "scanned", " tabled", scanned, tabled)
		if compared == 0 {
			t.Fatalf("no receipt could be compared in %s", repository)
		}
		t.Logf("%s: %d receipts byte-identical, states %v", repository, compared, states)
	}
}

// TestSymbolWindowTableReceiptComparisonHasTeeth is the negative control: a
// table that reaches no symbol empties every candidate's context terms. The
// ranking reads those terms only to rescue a candidate whose name and path
// miss the query while its window holds three query terms, so the fixture
// gains exactly that symbol, asked for in its window's own words, and the
// empty table must drop it from the receipt.
func TestSymbolWindowTableReceiptComparisonHasTeeth(t *testing.T) {
	root := evalQueryRepository(t)
	writeTestFile(t, root, "internal/billing/ledger.go", "package billing\n\n"+
		"// Reconcile settles the outstanding invoice balance.\n"+
		"func Reconcile() bool { return true }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "reconcile the ledger")
	index := evalIndexWithSymbolWindows(t, root)
	empty := *index
	vocabulary := *index.Vocabulary
	vocabulary.SymbolWindows = windowPostings{KeyOffsets: []uint32{0}, Offsets: []uint32{0}}
	empty.Vocabulary = &vocabulary
	task := "settles outstanding invoice balance"
	expected, _, err := evalContextReceipt(index, task, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(expected, []byte("Reconcile")) {
		t.Fatalf("the window terms did not admit the control symbol: %s", expected)
	}
	actual, _, err := evalContextReceipt(&empty, task, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(expected, actual) {
		t.Fatal("an empty table moved no receipt, so the parity comparison proves nothing")
	}
}

// TestSymbolWindowTableRoundTripsAndChecks pins the wire layout: the table a
// snapshot carries decodes to the table that was built, and check rejects a
// posting outside the index it was decoded for.
func TestSymbolWindowTableRoundTripsAndChecks(t *testing.T) {
	index := evalIndexWithSymbolWindows(t, evalQueryRepository(t))
	table := index.Vocabulary.SymbolWindows
	if table.keyCount() <= 0 {
		t.Fatal("the fixture built an empty window table")
	}
	encoded, err := table.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var decoded windowPostings
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, table) {
		t.Fatal("the window table did not survive its wire encoding")
	}
	if err := decoded.check(len(index.Symbols)); err != nil {
		t.Fatalf("a table built from the index failed its own check: %v", err)
	}
	if err := decoded.check(1); err == nil {
		t.Fatal("check accepted postings beyond the index")
	}
	if err := decoded.UnmarshalBinary(encoded[:len(encoded)-1]); err == nil {
		t.Fatal("a truncated table decoded without error")
	}
	damaged := table
	damaged.Data = append([]byte(nil), table.Data...)
	damaged.Data[len(damaged.Data)-1] = 0x80
	if err := damaged.check(len(index.Symbols)); err == nil {
		t.Fatal("check accepted a posting list that does not decode")
	}
}

// TestSymbolWindowTableSurvivesTheSnapshot is the parity claim on the path
// the product takes: the table `index` writes into the gob snapshot is the
// table a query loads, and the loaded index emits the same receipt as the
// built one.
func TestSymbolWindowTableSurvivesTheSnapshot(t *testing.T) {
	root := evalQueryRepository(t)
	built, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if built.Vocabulary == nil || built.Vocabulary.SymbolWindows.keyCount() <= 0 {
		t.Fatal("Build attached no symbol window table")
	}
	if _, err := WriteSnapshot(built); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("snapshot load: hit=%v err=%v", hit, err)
	}
	if !reflect.DeepEqual(loaded.Vocabulary.SymbolWindows, built.Vocabulary.SymbolWindows) {
		t.Fatal("the loaded symbol window table differs from the built one")
	}
	task := "session expiry device revocation enforcement"
	expected, _, err := evalContextReceipt(built, task, 0)
	if err != nil {
		t.Fatal(err)
	}
	actual, _, err := evalContextReceipt(loaded, task, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatal("the loaded snapshot emitted a different receipt from the built index")
	}
}

// evalIndexWithSymbolWindows builds the eval index the way index does for a
// snapshot, with the term table and its symbol windows attached.
func evalIndexWithSymbolWindows(t *testing.T, repository string) *Index {
	t.Helper()
	index, err := BuildEval(context.Background(), repository)
	if err != nil {
		t.Fatalf("BuildEval(%s): %v", repository, err)
	}
	index.Vocabulary = index.buildVocabulary()
	index.Vocabulary.SymbolWindows = index.buildSymbolWindows()
	return index
}

func withScannedContextWindow[Result any](t *testing.T, emit func() Result) Result {
	t.Helper()
	evalScannedContextWindow = true
	defer func() { evalScannedContextWindow = false }()
	return emit()
}
