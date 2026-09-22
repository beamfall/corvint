package contextindex

import (
	"bytes"
	"context"
	"reflect"
	"testing"
)

// symbolTermsDistributivityPairs are name/path pairs chosen for the shapes the
// distributivity argument turns on: a camelCase boundary that would be split
// differently if the two sides were joined without a space, an acronym run, a
// snake_case and a hyphenated identifier, digits abutting letters, a name that
// is entirely a stop word, an empty name, non-ASCII text, and the two runes
// Unicode lowercases into ASCII.
var symbolTermsDistributivityPairs = [][2]string{
	{"EnforceSessionRevocation", "internal/auth/session.go"},
	{"enforce_session_revocation", "internal/auth/session_test.go"},
	{"parseHTTPRequest", "internal/http/parse.go"},
	{"HTTPServerHandler", "cmd/corvint/main.go"},
	{"v2", "api/v2/client.go"},
	{"queries", "internal/queries/aliases.go"},
	{"the", "docs/specs/the-index.md"},
	{"", "internal/empty/empty.go"},
	{"r\u00e9vocationNa\u00efve", "docs/caf\u00e9/\u03a9.md"},
	{"\u212Aelvin\u0130dentifier", "internal/\u212Akey/session\u0130d.go"},
	{"X", "a/b"},
	{"AlreadySplit-Name", "already-split/path-name.go"},
	{"trailing", "path/with/trailing/"},
	{"leadingUnderscore", "_internal/_hidden/file_name.go"},
}

// TestSymbolTermsDistributeOverASpace is the property the path cache rests on:
// terms("<name> <path>") is exactly terms(name) united with terms(path). No
// stage of terms reaches across a space -- camelSplit inserts a hyphen only
// between two adjacent letters or digits, ToLower and the '_' rewrite are per
// character, and asciiWords begins a word at an ASCII letter and ends it at the
// first character outside [letter digit _ -] -- so the join contributes nothing
// and separates the two sides in every stage.
func TestSymbolTermsDistributeOverASpace(t *testing.T) {
	for _, pair := range symbolTermsDistributivityPairs {
		assertSymbolTermsDistribute(t, pair[0], pair[1])
	}
}

// FuzzSymbolTermsDistributeOverASpace drives the same property over arbitrary
// text, which is the only way to reach the byte sequences the table did not
// think of -- in particular the boundary characters camelSplit and asciiWords
// classify differently.
func FuzzSymbolTermsDistributeOverASpace(f *testing.F) {
	for _, pair := range symbolTermsDistributivityPairs {
		f.Add(pair[0], pair[1])
	}
	f.Fuzz(func(t *testing.T, name, sourcePath string) {
		assertSymbolTermsDistribute(t, name, sourcePath)
	})
}

func assertSymbolTermsDistribute(t *testing.T, name, sourcePath string) {
	t.Helper()
	want := terms(name + " " + sourcePath)
	cache := evalSymbolTerms{}
	got, _ := cache.of(name, sourcePath)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("terms(%q + \" \" + %q):\n got = %v\nwant = %v", name, sourcePath, sortedKeys(got), sortedKeys(want))
	}
	// A second name against the same cached path is the case the cache exists
	// for, and the case a stale entry would break.
	second := "Second" + name + "Declaration"
	if again, _ := cache.of(second, sourcePath); !reflect.DeepEqual(terms(second+" "+sourcePath), again) {
		t.Fatalf("cached path diverged for %q in %q: got %v", second, sourcePath, sortedKeys(again))
	}
	// A different path against a warm cache must invalidate it.
	other := sourcePath + "/other_module.go"
	if switched, _ := cache.of(name, other); !reflect.DeepEqual(terms(name+" "+other), switched) {
		t.Fatalf("cache failed to invalidate from %q to %q: got %v", sourcePath, other, sortedKeys(switched))
	}
}

// TestSymbolTermsDistributivityIsSpecificToTheSeparator is the non-vacuity
// control for the property above. terms does not distribute over an arbitrary
// join -- concatenating the two sides with no separator lets camelSplit and
// asciiWords read across the seam -- so the property being asserted is a claim
// about the space, not a tautology about terms.
func TestSymbolTermsDistributivityIsSpecificToTheSeparator(t *testing.T) {
	name, sourcePath := "EnforceSessionRevocation", "internal/auth/session.go"
	union := terms(name)
	for term := range terms(sourcePath) {
		union[term] = struct{}{}
	}
	if reflect.DeepEqual(terms(name+sourcePath), union) {
		t.Fatal("terms distributes over an empty join too, so the space is not what the property tests")
	}
}

// TestPathTermCacheEmitsIdenticalReceipts is the standing proof that caching a
// path's terms across the declarations that share it is a pure allocation
// change. Each receipt is emitted both ways in one binary and compared byte for
// byte, over the same contrasting task and budget matrix the keep-set parity
// test uses.
func TestPathTermCacheEmitsIdenticalReceipts(t *testing.T) {
	pinRoot, _ := pinParityRepository(t)
	roots := append([]string{evalQueryRepository(t), pinRoot}, parityRepositoriesFromEnvironment()...)
	for _, repository := range roots {
		index, err := BuildEval(context.Background(), repository)
		if err != nil {
			t.Fatalf("BuildEval(%s): %v", repository, err)
		}
		expected := withRecomputedPathTerms(t, func() []parityReceipt {
			return emitParityReceipts(index)
		})
		actual := emitParityReceipts(index)
		compared, states := compareParityReceipts(t, repository, "recomputed", "    cached", expected, actual)
		if compared == 0 {
			t.Fatalf("no receipt could be compared in %s", repository)
		}
		t.Logf("%s: %d receipts byte-identical, states %v", repository, compared, states)
	}
}

// TestPathTermCacheReceiptComparisonHasTeeth is the negative control for the
// test above: it proves that receipt bytes move when a symbol's path terms
// move, which is the only quantity the cache can get wrong. A parity harness
// that could not see this difference would pass whatever the cache did.
func TestPathTermCacheReceiptComparisonHasTeeth(t *testing.T) {
	root := evalQueryRepository(t)
	index, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	task := "session expiry device revocation enforcement"
	expected, _, err := evalContextReceipt(index, task, 0)
	if err != nil {
		t.Fatal(err)
	}
	moved := *index
	moved.Symbols = append([]Symbol(nil), index.Symbols...)
	changed := false
	for position := range moved.Symbols {
		if moved.Symbols[position].Path == "internal/auth/session.go" {
			moved.Symbols[position].Path = "internal/session/authority.go"
			changed = true
		}
	}
	if !changed {
		t.Fatal("the fixture no longer carries the symbol this control moves")
	}
	actual, _, err := evalContextReceipt(&moved, task, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(expected, actual) {
		t.Fatal("a symbol's path terms changed and the receipt did not, so the parity comparison proves nothing")
	}
}

// withRecomputedPathTerms runs one matrix over the per-symbol construction. It
// is not parallel-safe by construction, which is why the parity test is the
// only caller and why it emits a whole matrix inside one bracket.
func withRecomputedPathTerms[Result any](t *testing.T, emit func() Result) Result {
	t.Helper()
	evalRecomputePathTerms = true
	defer func() { evalRecomputePathTerms = false }()
	return emit()
}
