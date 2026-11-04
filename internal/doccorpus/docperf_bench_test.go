// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"testing"
)

// BenchmarkDocperfCanonicalClaim isolates the worker's canonical comparison.
// All arms use the same JSON implementation and must compare every byte.
func BenchmarkDocperfCanonicalClaim(b *testing.B) {
	c := Claim{ID: "native:claim:docs/feature.md:123", Subject: "native:subject:docs/feature.md", Text: "The operator opens the ledger and confirms an entry before saving.", Provider: "native", Evidence: Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{{Path: "docs/feature.md", Start: 12, End: 13}}, Limitations: []string{"structural evidence"}}}
	raw, err := Encode(c)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("write", func(b *testing.B) {
		for b.Loop() {
			out := encodingComparer{want: raw}
			if _, err := EncodeTo(&out, &c); err != nil || !out.equal() {
				b.Fatal(err)
			}
		}
	})
	b.Run("reset", func(b *testing.B) {
		var out encodingComparer
		enc := jsontext.NewEncoder(&out, json.Deterministic(true))
		for b.Loop() {
			out = encodingComparer{want: raw}
			enc.Reset(&out, json.Deterministic(true))
			if err := json.MarshalEncode(enc, &c); err != nil || out.differ || out.n != int64(len(raw)) {
				b.Fatal(err)
			}
		}
	})
	b.Run("marshal", func(b *testing.B) {
		for b.Loop() {
			got, err := json.Marshal(&c, json.Deterministic(true))
			if err != nil || !bytes.Equal(got, raw[:len(raw)-1]) {
				b.Fatal(err)
			}
		}
	})
}
