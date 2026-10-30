package cli

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCD-V0-163/188: external PR identifiers remain exact in reports and evidence.
func TestV13179_BatchPRIdentityPreserved(t *testing.T) {
	for _, n := range []int64{1, 2147483647, 2147483648, 4294967297, 9223372036854775807} {
		t.Run(strconv.FormatInt(n, 10), func(t *testing.T) {
			want := strconv.FormatInt(n, 10)
			in := &batchInput{number: n}
			result := batchCloseValue(in, nil, "merge", "run", nil, "issued", true, nil)
			got, ok := result.Obj.Get("batchPr")
			if !ok || got.Str != want {
				t.Fatalf("manual report PR=%+v, want %s", got, want)
			}
			b := &gatedBatch{prURL: "pr", merge: "merge", runURL: "run"}
			raw := b.mergeDocument(n, 20, &gatedRow{candidate: "candidate", base: "base"})
			v, err := wire.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			got, ok = v.Obj.Get("batchPr")
			if !ok || got.Str != want {
				t.Fatalf("merge document PR=%+v, want %s", got, want)
			}
			// Independently constructed canonical bytes bind the exact identity in
			// the evidence digest, rather than only checking the decoded field.
			expected := []byte(`{"baseCommit":"base","batchPr":"` + want + `","candidateCommit":"candidate","lanePr":"20","mergeCommit":"merge","mergedAt":null,"prUrl":"pr","profile":"taskman-batch-merge/0","runUrl":"run","sealCommit":null}` + "\n")
			if !bytes.Equal(raw, expected) || wire.Sum(raw) != wire.Sum(expected) {
				t.Fatalf("merge evidence differs: %s", raw)
			}
		})
	}
}
