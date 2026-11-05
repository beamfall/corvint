// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"bytes"
	"math/rand/v2"
	"os"
	"reflect"
	"testing"
)

func TestDCRV18001CanonicalRecordMatchesEncoder(t *testing.T) {
	t.Run("DCR-V1-8001 canonical", func(t *testing.T) {
		r := rand.New(rand.NewPCG(8001, 2588))
		texts := []string{"", "ordinary", "\"\\\b\f\n\r\t\x00\x1f", "<>&/", "é😀\u2028\u2029\x7f", "field\":null,\"id\":\""}
		var fill func(reflect.Value)
		fill = func(v reflect.Value) {
			switch v.Kind() {
			case reflect.Struct:
				for i := range v.NumField() {
					fill(v.Field(i))
				}
			case reflect.String:
				v.SetString(texts[r.IntN(len(texts))])
			case reflect.Int:
				v.SetInt(int64(r.IntN(20) - 10))
			case reflect.Slice:
				n := r.IntN(4)
				if n == 0 {
					return
				}
				v.Set(reflect.MakeSlice(v.Type(), n-1, n-1))
				for i := range v.Len() {
					fill(v.Index(i))
				}
			default:
				t.Fatalf("uncovered record field: %s", v.Type())
			}
		}
		var buf []byte
		for range 500 {
			for _, v := range []any{new(Claim), new(Subject)} {
				fill(reflect.ValueOf(v).Elem())
				want, err := Encode(v)
				if err != nil {
					t.Fatal(err)
				}
				var supported bool
				buf, supported = appendCanonicalRecord(buf[:0], v)
				if !supported || !bytes.Equal(buf, want[:len(want)-1]) {
					t.Fatalf("%T: %s != %s", v, buf, want)
				}
			}
		}
		// A worker's warmed canonical comparison allocates no object per record.
		c := Claim{ID: "id", Subject: "subject", Text: "ledger", Evidence: Evidence{Anchors: []Anchor{{Path: "docs/a.md"}}}}
		buf, _ = appendCanonicalRecord(buf[:0], &c)
		if n := testing.AllocsPerRun(100, func() { buf, _ = appendCanonicalRecord(buf[:0], &c) }); n != 0 {
			t.Fatalf("canonical comparison allocations: %v", n)
		}
		if _, supported := appendCanonicalRecord(nil, new(Relation)); supported {
			t.Fatal("unsupported record took typed path")
		}
	})
}

// BenchmarkDCRV18001ArtifactDecode uses the retained DCR-V1-418 fixture,
// excluding fixture construction and freshness from the decode measurement.
func BenchmarkDCRV18001ArtifactDecode(b *testing.B) {
	path := os.Getenv("CORVINT_PERFCORPUS_ARTIFACT")
	if path == "" {
		b.Skip("set CORVINT_PERFCORPUS_ARTIFACT")
	}
	for b.Loop() {
		f, err := os.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		_, _, err = decodeArtifactReader(f, nil, nil)
		f.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDCRV18001CanonicalComparison(b *testing.B) {
	c := Claim{ID: "native:docs/a.md#claim-1", Subject: "native:docs/a.md", Text: "The operator opens ledger 123 and confirms entry 456 before saving.", Provider: "native", Evidence: Evidence{
		Derivation: "observed", Trust: "generated", State: "supported", Freshness: "fresh", Limitations: []string{"structural evidence"},
		Anchors: []Anchor{{Repository: "example", Revision: "0123456789012345678901234567890123456789", Path: "docs/a.md", Blob: "0123456789012345678901234567890123456789", Start: 1, End: 3, Kind: "derived", Reason: "source lines"}},
	}}
	raw, err := Encode(&c)
	if err != nil {
		b.Fatal(err)
	}
	raw = raw[:len(raw)-1]
	b.Run("Encoder", func(b *testing.B) {
		for b.Loop() {
			compare := encodingComparer{want: raw}
			if _, err := EncodeTo(&compare, &c); err != nil || compare.differ || compare.n != int64(len(raw)) {
				b.Fatal("encoder mismatch")
			}
		}
	})
	b.Run("Typed", func(b *testing.B) {
		var buf []byte
		for b.Loop() {
			buf, _ = appendCanonicalRecord(buf[:0], &c)
			if !bytes.Equal(buf, raw) {
				b.Fatal("typed mismatch")
			}
		}
	})
}
