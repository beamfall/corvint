// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"reflect"
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

// DCR-V1-7801: batch decoding is still the ordinary closed artifact decoder,
// including a noncanonical early member followed by an undecodable member in
// a later batch. Scanner segmentation and read windows cannot change that order.
func TestDCRV17801BatchMatchesClosedArtifact(t *testing.T) {
	_, a, _ := builtFixture(t)
	data := rehashed(t, a, func(a *Artifact) {
		sample := a.Claims[0]
		a.Claims = make([]Claim, 2*splitChunk+3)
		texts := scanTexts()
		for i := range a.Claims {
			a.Claims[i] = sample
			a.Claims[i].ID = fmt.Sprintf("batch:%04d", i)
			a.Claims[i].Text = texts[i%len(texts)]
		}
	})
	noncanonical := bytes.Replace(data, []byte(`"id":"batch:0000"`), []byte(`"id":"\u0062atch:0000"`), 1)
	unknown := bytes.Replace(data, []byte(`"id":"batch:1026"`), []byte(`"extra":0,"id":"batch:1026"`), 1)
	both := bytes.Replace(noncanonical, []byte(`"id":"batch:1026"`), []byte(`"extra":0,"id":"batch:1026"`), 1)
	for _, raw := range [][]byte{data, noncanonical, unknown, both} {
		closed, err := decodeArtifact(raw)
		wantError := ""
		if err != nil {
			wantError = fail("invalid closed JSON input").Error()
		} else if !EncodedEqual(closed, raw) {
			wantError = fail("noncanonical artifact").Error()
		}
		for _, window := range []int{0, 4096, 1 << 20} {
			got := decodeWith(raw, boundaryScan{}, window, 16)
			if got.err != wantError {
				t.Fatalf("window %d: batch error %q, closed oracle %q", window, got.err, wantError)
			}
			if wantError == "" && !reflect.DeepEqual(got.artifact, closed) {
				t.Fatalf("window %d: batch records differ from closed artifact", window)
			}
		}
	}
}
