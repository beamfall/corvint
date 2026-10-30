package doccorpus

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

// DCS-V1-572: the retry of a compile refused by the output byte bound reads the inputs the
// refused compile pinned, and builds the bytes a retry that pinned them again builds.
func TestDCSV1572RetryFromPinnedInputsIsByteIdentical(t *testing.T) {
	// Not parallel: it builds an over-bound corpus like TestDCSV1569DerivationStopsAtTheOutputBound.
	var big strings.Builder
	for i := range MaxRecords + 8 {
		fmt.Fprintf(&big, "def f%05d():\n    return %d\n", i, i)
	}
	files := map[string]string{"src/big.py": big.String(), "src/readme.md": "# Big\n\nMany functions.\n"}
	root, m := ownedSourceCorpus(t, files, []string{"src/big.py"}, false)
	ctx := context.Background()
	_, result, err := compileOnce(ctx, root, m, nil, derivationPlan{}, nil)
	if err == nil || result.outputBytes == 0 || result.pinned == nil {
		t.Fatalf("the first compile should be refused by the output bound with its inputs pinned: %v %d %v", err, result.outputBytes, result.pinned != nil)
	}
	plan := retryPlan(derivationPlan{}, result, err)
	reused, again, err := compileOnce(ctx, root, m, nil, plan, result.pinned)
	if err != nil || again.pinned != nil {
		t.Fatalf("the retry from pinned inputs should build and hand on nothing pinned: %v", err)
	}
	repinned, _, err := compileOnce(ctx, root, m, nil, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Encode(repinned)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Encode(reused)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the retry from pinned inputs encoded %d bytes, the re-pinned retry %d", len(got), len(want))
	}
}

// DCS-V1-572: the shared line table selects exactly the bytes span selects and refuses
// exactly what it refuses.
func TestDCSV1572AnchorLinesSelectWhatSpanSelects(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "\n", "a", "a\n", "a\nb", "a\nb\n", "\n\nx\n\n", "  \n\tq\r\nz", strings.Repeat("y", maxSpanBytes+1) + "\nz\n"} {
		data := []byte(text)
		lines := newAnchorLines(data)
		if lines.sha256 != Digest(data) {
			t.Fatalf("%q: digest", text)
		}
		for start := -1; start <= 7; start++ {
			for end := -1; end <= 7; end++ {
				want, wantErr := span(data, start, end)
				got, gotErr := lines.span(data, start, end)
				if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !bytes.Equal(got, want) {
					t.Fatalf("%q %d-%d: got %q %v, want %q %v", text, start, end, got, gotErr, want, wantErr)
				}
			}
		}
	}
}
