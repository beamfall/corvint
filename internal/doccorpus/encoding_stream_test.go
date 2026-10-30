package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// largeArtifact builds a /2 corpus whose canonical encoding is larger than MaxBytes.
func largeArtifact(t *testing.T) (*Artifact, []byte) {
	t.Helper()
	root, m := providerFixture(t, func(p *ProviderRecord) {
		p.Schema = AdoptionProviderSchema
		sample := p.Claims[0]
		p.Claims = nil
		for i := 0; i < 6000; i++ {
			c := sample
			c.ID = fmt.Sprintf("adapter:claim:%05d", i)
			c.Text = strings.Repeat("source-declared bounded corpus evidence ", 16)
			p.Claims = append(p.Claims, c)
		}
	})
	m.Schema = ManifestSchemaV2
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= MaxBytes {
		t.Fatal("fixture does not exceed the legacy byte bound")
	}
	return a, raw
}

func TestDCPV1085StreamedEncodingIsEncode(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-085 pre-sized and streamed encodings equal Encode", func(t *testing.T) {
		a, raw := largeArtifact(t)
		small := Request{Operation: "get", ID: "adapter:claim:00001"}
		smallRaw, err := Encode(small)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			value any
			want  []byte
		}{{a, raw}, {small, smallRaw}} {
			n := len(c.want)
			for _, hint := range []int{-1, 0, 1, n - 2, n - 1, n, n + 1, 2 * n} {
				got, err := EncodeSized(c.value, hint)
				if err != nil || !bytes.Equal(got, c.want) {
					t.Fatalf("EncodeSized hint %d differs from Encode: %v", hint, err)
				}
			}
			if !EncodedEqual(c.value, c.want) {
				t.Fatal("EncodedEqual rejected the Encode bytes")
			}
			flipped := bytes.Clone(c.want)
			flipped[n/2] ^= 1
			for name, want := range map[string][]byte{"no newline": c.want[:n-1], "extra byte": append(bytes.Clone(c.want), '\n'), "truncated": c.want[:n/2], "flipped": flipped, "empty": nil, "newline replaced": append(bytes.Clone(c.want[:n-1]), ' ')} {
				if EncodedEqual(c.value, want) {
					t.Fatalf("EncodedEqual accepted %s", name)
				}
			}
			got, err := hashValue(c.value)
			if err != nil || got != Digest(c.want) {
				t.Fatalf("streamed hash differs from Digest(Encode): %v", err)
			}
		}
	})
	t.Run("DCP-V1-085 output bounds unchanged", func(t *testing.T) {
		a, _ := largeArtifact(t)
		over := Request{Operation: "search", Query: strings.Repeat("q", MaxBytes)}
		if _, err := Encode(over); err == nil {
			t.Fatal("fixture does not exceed the Encode bound")
		}
		if _, err := EncodeSized(over, MaxBytes*2); err == nil || !strings.Contains(err.Error(), "output bound exceeded") {
			t.Fatalf("EncodeSized bound: %v", err)
		}
		if _, err := hashValue(over); err == nil || !strings.Contains(err.Error(), "output bound exceeded") {
			t.Fatalf("hashValue bound: %v", err)
		}
		// A /1 artifact above the legacy bound is refused for its size, before canonical form or digest.
		legacy := *a
		legacy.Schema = Schema
		data, err := json.Marshal(legacy, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		_, err = ParseArtifact(append(data, '\n'))
		var refusal *Error
		if !errors.As(err, &refusal) || refusal.Message != "output bound exceeded" {
			t.Fatalf("legacy artifact over bound: %v", err)
		}
	})
}

// TestDCPV1085ParseAllocationIgnoresEncoderPool brackets ParseArtifact with TotalAlloc on a cold
// json/v2 encoder pool (emptied by two GC cycles) and on the following, possibly warm, calls.
// json.Marshal's pooled buffer made the cold call cost several times the output (V1-1380).
func TestDCPV1085ParseAllocationIgnoresEncoderPool(t *testing.T) {
	t.Run("DCP-V1-085 ParseArtifact allocation follows input size", func(t *testing.T) {
		_, raw := largeArtifact(t)
		measure := func(cold bool) uint64 {
			if cold {
				runtime.GC()
				runtime.GC()
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			if _, err := ParseArtifact(raw); err != nil {
				t.Fatal(err)
			}
			runtime.ReadMemStats(&after)
			return after.TotalAlloc - before.TotalAlloc
		}
		samples := []uint64{measure(true), measure(false), measure(false), measure(true)}
		low, high := samples[0], samples[0]
		for _, s := range samples {
			low, high = min(low, s), max(high, s)
		}
		t.Logf("parse_artifact_total_alloc=%v input_bytes=%d race=%v", samples, len(raw), raceEnabled)
		if float64(high-low) > 0.10*float64(low) {
			t.Fatalf("ParseArtifact allocation varies with encoder pool state: %v", samples)
		}
		if !raceEnabled && high > 3*uint64(len(raw)) {
			t.Fatalf("ParseArtifact allocated %d bytes for %d input bytes", high, len(raw))
		}
	})
}
