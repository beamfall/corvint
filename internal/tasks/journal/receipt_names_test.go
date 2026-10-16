package journal

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// oracleReceiptSeq is the parse-and-format check receiptSeq used before
// V1-1410, kept here as the oracle receiptNameSeq must agree with.
func oracleReceiptSeq(name string) (uint64, bool) {
	if len(name) != 17 || !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	n, err := strconv.ParseUint(name[:12], 10, 64)
	canonical, _ := snapshot.ReceiptName(n)
	if err != nil || n == 0 || canonical != name {
		return 0, false
	}
	return n, true
}

// TestV11410_ReceiptNameSeqAgreesWithOracle covers V1-1410: the fixed-format
// receipt name check accepts exactly the names the parse-and-format oracle
// accepted, with the same sequence, over every single-byte edit of canonical
// names, every length from 0 to 24, and the digit boundaries.
func TestV11410_ReceiptNameSeqAgreesWithOracle(t *testing.T) {
	check := func(name string) {
		t.Helper()
		seq, ok := receiptNameSeq(name)
		want, wantOK := oracleReceiptSeq(name)
		if ok != wantOK || seq != want {
			t.Fatalf("%q: got (%d, %v), oracle (%d, %v)", name, seq, ok, want, wantOK)
		}
		if _, err := receiptSeq(name); (err == nil) != wantOK || (err != nil && (wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "invalid receipt filename"))) {
			t.Fatalf("%q: receiptSeq %v, oracle ok=%v", name, err, wantOK)
		}
	}
	bytes := []byte("0123456789/\\.-_+ aAjJnNsSoO\x00\xff")
	for _, n := range []uint64{0, 1, 9, 10, 4096, 999999999999} {
		canonical, _ := snapshot.ReceiptName(n)
		check(canonical)
		for i := 0; i < len(canonical); i++ {
			for _, c := range bytes {
				edited := []byte(canonical)
				edited[i] = c
				check(string(edited))
			}
		}
		for l := 0; l <= 24; l++ {
			check(canonical[:min(l, len(canonical))] + strings.Repeat("0", max(0, l-len(canonical))))
			check(strings.Repeat("0", max(0, l-len(canonical))) + canonical[:min(l, len(canonical))])
		}
	}
	for _, name := range []string{"", ".", "..", ".json", "000000000001.json.tmp-1-x", "000000000001.tmp-1-x.json", "1000000000000.json", "00000000000１.json"} {
		check(name)
	}
}

// TestV11410_ReceiptNamesFirstErrorOrder pins the listing-order checks of
// receiptNames: an aliased name, a repeated name, a temporary name and an
// invalid name are each judged at their own position, so the first failing
// name in listing order names the refusal, with the pre-V1-1410 codes and
// messages.
func TestV11410_ReceiptNamesFirstErrorOrder(t *testing.T) {
	one, _ := snapshot.ReceiptName(1)
	two, _ := snapshot.ReceiptName(2)
	far, _ := snapshot.ReceiptName(500000000000)
	temp := one + ".tmp-1-x"
	const (
		aliased = "JOURNAL_FORKED: receipts: duplicate or aliased inventory name"
		invalid = "JOURNAL_FORKED: 00000000001.json: invalid receipt filename"
	)
	cases := []struct {
		name     string
		names    []string
		err      string
		receipts []string
		staging  bool
	}{
		{"settled", []string{two, one}, "", []string{two, one}, false},
		{"sparse sequence", []string{far, one, far + ".tmp-2-y"}, "", []string{far, one}, true},
		{"temporary", []string{temp, two}, "", []string{two}, true},
		{"dot", []string{".", one}, aliased, nil, false},
		{"slash", []string{"a/b"}, aliased, nil, false},
		{"empty", []string{""}, aliased, nil, false},
		{"repeated settled", []string{one, two, one}, aliased, nil, false},
		{"repeated sparse", []string{far, one, far}, aliased, nil, false},
		{"repeated temporary", []string{temp, temp}, aliased, nil, false},
		{"invalid", []string{one, "00000000001.json"}, invalid, nil, false},
		{"zero", []string{"000000000000.json"}, "JOURNAL_FORKED: 000000000000.json: invalid receipt filename", nil, false},
		{"invalid before repeat", []string{one, temp, "00000000001.json", one}, invalid, nil, false},
		{"repeat before invalid", []string{one, one, "00000000001.json"}, aliased, nil, false},
		{"invalid before dot", []string{"00000000001.json", "."}, invalid, nil, false},
		{"dot before invalid", []string{".", "00000000001.json"}, aliased, nil, false},
		{"temporary before invalid", []string{temp, "00000000001.json"}, invalid, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := &observation{files: map[string]os.FileInfo{}}
			err := receiptNames(o, c.names)
			switch {
			case c.err == "" && err != nil:
				t.Fatalf("unexpected %v", err)
			case c.err != "" && (err == nil || err.Error() != c.err || wire.CodeOf(err) != wire.CodeJournalForked):
				t.Fatalf("got %v, want %s", err, c.err)
			}
			// A refused observation is discarded, so only an accepted one
			// has a settled name list and staging flag to compare.
			if c.err == "" && (o.staging != c.staging || strings.Join(o.receipts, ",") != strings.Join(c.receipts, ",")) {
				t.Fatalf("receipts %v staging %v, want %v %v", o.receipts, o.staging, c.receipts, c.staging)
			}
		})
	}
}

func benchReceiptNameList(n int, shuffled bool) []string {
	names := make([]string, n)
	for i := range names {
		names[i], _ = snapshot.ReceiptName(uint64(i + 1))
	}
	if shuffled {
		rand.New(rand.NewSource(1)).Shuffle(n, func(i, j int) { names[i], names[j] = names[j], names[i] })
	}
	return names
}

// BenchmarkReceiptNames measures the per-name work of receiptNames over an
// already listed receipts/ directory, apart from the listing itself.
func BenchmarkReceiptNames(b *testing.B) {
	for _, n := range []int{3000, 13000} {
		names := benchReceiptNameList(n, true)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				o := &observation{files: map[string]os.FileInfo{}}
				if err := receiptNames(o, names); err != nil || len(o.receipts) != n {
					b.Fatal(err, len(o.receipts))
				}
			}
		})
	}
}
