// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"testing"
)

// DCR-V1-7801: a reused encoder compares the same canonical bytes as EncodeTo,
// including escaped strings, nil/empty slices and empty fields after nonempty
// fields. Its framing newline cannot hide a missing, extra or changed byte.
func TestDCRV17801WorkerCanonicalComparison(t *testing.T) {
	var comparison elementEncodingComparer
	encoder := jsontext.NewEncoder(&comparison)
	for _, text := range append(scanTexts(), "\x00\b\f\r\n\t<>&", "") {
		for _, claim := range []Claim{{Text: text, Evidence: Evidence{Anchors: []Anchor{}, Limitations: []string{text}}}, {}} {
			canonical, err := Encode(claim)
			if err != nil {
				t.Fatal(err)
			}
			raw := canonical[:len(canonical)-1]
			variants := [][]byte{raw, append(bytes.Clone(raw), ' '), raw[:len(raw)-1]}
			changed := bytes.Clone(raw)
			changed[len(changed)/2] ^= 1
			variants = append(variants, changed)
			for i, want := range variants {
				comparison = elementEncodingComparer{encodingComparer: encodingComparer{want: want}}
				encoder.Reset(&comparison, json.Deterministic(true))
				if err := json.MarshalEncode(encoder, &claim); err != nil {
					t.Fatal(err)
				}
				if comparison.equalElement() != (i == 0) {
					t.Fatalf("variant %d text %q comparison %+v", i, text, comparison)
				}
			}
			for width := 1; width <= len(canonical); width++ {
				comparison = elementEncodingComparer{encodingComparer: encodingComparer{want: raw}}
				for offset := 0; offset < len(canonical); offset += width {
					comparison.Write(canonical[offset:min(offset+width, len(canonical))])
				}
				if !comparison.equalElement() {
					t.Fatalf("write width %d", width)
				}
				comparison.Write([]byte{'\n'})
				if comparison.equalElement() {
					t.Fatal("accepted a second framing newline")
				}
			}
		}
	}
}
