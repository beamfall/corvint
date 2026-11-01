package cem2proto

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// countingSource records whether a projection reached Git.
type countingSource struct {
	inner Source
	calls int
}

func (c *countingSource) Patch(ctx context.Context, base, target string) ([]byte, error) {
	c.calls++
	return c.inner.Patch(ctx, base, target)
}

func (c *countingSource) Blobs(ctx context.Context, base string, paths []string) (map[string]Blob, error) {
	c.calls++
	return c.inner.Blobs(ctx, base, paths)
}

// refuse projects a tampered pair and requires the code, with no Git read
// when the refusal is decidable from the bytes alone.
func refuse(t *testing.T, f *fixture, header, body []byte, code string, beforeGit bool) {
	t.Helper()
	src := &countingSource{inner: mustSource(t, f.root)}
	got, err := Project(context.Background(), src, f.target, header, body)
	if got != nil {
		t.Fatalf("refusal produced %d bytes of cem/1.0", len(got))
	}
	wantCode(t, err, code)
	if beforeGit && src.calls != 0 {
		t.Fatalf("%s was decided after %d Git reads", code, src.calls)
	}
}

func TestCEM2V0007BodySha256Mismatch(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	b := decoded(t, body)
	b.Hunks[2].Reason = 5
	tampered, err := EncodeBody(b)
	if err != nil {
		t.Fatal(err)
	}
	refuse(t, f, header, tampered, CodeBodySha256Mismatch, true)
}

func TestCEM2V0007PatchSha256Mismatch(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	h, _ := DecodeHeader(header)
	h.PatchSha256 = strings.Repeat("0", 64)
	header, _ = EncodeHeader(h)
	refuse(t, f, header, body, CodePatchSha256Mismatch, false)
}

func TestCEM2V0007HunkCountMismatch(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	for name, edit := range map[string]func(*Body){
		"dropped": func(b *Body) { b.Hunks = b.Hunks[:len(b.Hunks)-1] },
		"added":   func(b *Body) { b.Hunks = append(b.Hunks, HunkRow{Reason: 1}) },
	} {
		t.Run(name, func(t *testing.T) {
			b := decoded(t, body)
			edit(&b)
			h, tampered := reseal(t, header, b)
			refuse(t, f, h, tampered, CodeHunkCountMismatch, false)
		})
	}
}

func TestCEM2V0007IndexOutOfRange(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	for name, edit := range map[string]func(*Body){
		"path index":     func(b *Body) { b.Evidence[0].Path = len(b.Paths) },
		"evidence index": func(b *Body) { b.Hunks[0].Basis[0].Evidence = len(b.Evidence) },
	} {
		t.Run(name, func(t *testing.T) {
			b := decoded(t, body)
			edit(&b)
			h, tampered := reseal(t, header, b)
			refuse(t, f, h, tampered, CodeIndexOutOfRange, true)
		})
	}
}

func TestCEM2V0007CodeOutOfRange(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	for name, edit := range map[string]func(*Body){
		"reason":   func(b *Body) { b.Hunks[1].Reason = len(Reasons) },
		"relation": func(b *Body) { b.Hunks[0].Basis[1].Relation = len(Relations) },
	} {
		t.Run(name, func(t *testing.T) {
			b := decoded(t, body)
			edit(&b)
			h, tampered := reseal(t, header, b)
			refuse(t, f, h, tampered, CodeCodeOutOfRange, true)
		})
	}
}

func TestCEM2V0007MalformedBody(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	for name, raw := range map[string]string{
		"unknown member":      strings.Replace(string(body), `{"evidence"`, `{"confidence":1,"evidence"`, 1),
		"fabricated id":       strings.Replace(string(body), `,"paths"`, `,"ids":["evidence:sha256:00"],"paths"`, 1),
		"non-canonical":       strings.Replace(string(body), `,"hunks"`, `, "hunks"`, 1),
		"missing member":      strings.Replace(string(body), `{"evidence":`, `{"x":`, 1),
		"short evidence row":  strings.Replace(string(body), `"evidence":[[0,`, `"evidence":[[`, 1),
		"pinned row, 4 items": strings.Replace(string(body), `"evidence":[[0,`, `"evidence":[["x",0,`, 1),
		"empty hunk row":      strings.Replace(string(body), `"hunks":[[`, `"hunks":[[],[`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if raw == string(body) {
				t.Fatal("tamper did not apply")
			}
			h, err := DecodeHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			h.BodySha256 = Sha256Hex([]byte(raw))
			resealed, _ := EncodeHeader(h)
			refuse(t, f, resealed, []byte(raw), CodeMalformed, true)
		})
	}
}

func TestCEM2V0007UnknownCriticalIsUnsupported(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	h, _ := DecodeHeader(header)
	h.Critical = []string{"cem/1.1:ext/coverage"}
	header, _ = EncodeHeader(h)
	refuse(t, f, header, body, CodeUnsupportedCritical, true)
	_, err := DecodeHeader(header)
	var typed *Error
	if !errors.As(err, &typed) || !typed.Unsupported() {
		t.Fatalf("critical entry = %v; want an abstention", err)
	}
}

// CEM2-V0-018: delta maps are a deferral.
func TestCEM2V0007ParentsAreUnsupported(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	h, _ := DecodeHeader(header)
	h.Parents = []string{strings.Repeat("d", 64)}
	header, _ = EncodeHeader(h)
	refuse(t, f, header, body, CodeUnsupported, true)
}

func TestCEM2V0007PinnedMismatch(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, true)
	for name, edit := range map[string]func(*Body){
		"blob oid":    func(b *Body) { b.Evidence[0].BlobOid = strings.Repeat("e", len(b.Evidence[0].BlobOid)) },
		"span sha256": func(b *Body) { b.Evidence[1].SpanSha256 = strings.Repeat("f", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			b := decoded(t, body)
			edit(&b)
			h, tampered := reseal(t, header, b)
			refuse(t, f, h, tampered, CodePinnedMismatch, false)
		})
	}
}

func TestCEM2V0007EvidenceUnresolved(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	for name, edit := range map[string]func(*Body){
		"absent at base":  func(b *Body) { b.Paths[b.Evidence[0].Path] = "new.txt" },
		"span past end":   func(b *Body) { b.Evidence[0].End = 1 << 20 },
		"empty span":      func(b *Body) { b.Evidence[0].End = b.Evidence[0].Start },
		"directory entry": func(b *Body) { b.Paths[b.Evidence[0].Path] = "docs" },
	} {
		t.Run(name, func(t *testing.T) {
			b := decoded(t, body)
			edit(&b)
			h, tampered := reseal(t, header, b)
			refuse(t, f, h, tampered, CodeEvidenceUnresolved, false)
		})
	}
}

// CEM2-V0-010: over-bound bodies are refused before any Git read.
func TestCEM2V0010BoundsRefusedBeforeGit(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	for name, edit := range map[string]func(*Body){
		"hunks": func(b *Body) {
			for len(b.Hunks) <= wire.MaxHunks {
				b.Hunks = append(b.Hunks, HunkRow{Reason: 1})
			}
		},
		"evidence": func(b *Body) {
			for len(b.Evidence) <= wire.MaxEvidence {
				b.Evidence = append(b.Evidence, b.Evidence[0])
			}
		},
		"path bytes": func(b *Body) { b.Paths[0] = strings.Repeat("p", wire.MaxPathBytes+1) },
	} {
		t.Run(name, func(t *testing.T) {
			b := decoded(t, body)
			edit(&b)
			h, tampered := reseal(t, header, b)
			refuse(t, f, h, tampered, CodeBoundExceeded, true)
		})
	}
	big := bytes.Repeat([]byte(" "), wire.MaxMapBytes+1)
	h, _ := DecodeHeader(header)
	h.BodySha256 = Sha256Hex(big)
	resealed, _ := EncodeHeader(h)
	refuse(t, f, resealed, big, CodeBoundExceeded, true)
}

// CEM2-V0-019: derived mode cannot carry a forged evidence id, span hash,
// hunk range or hunk id; the projection recomputes each from Git.
func TestCEM2V0019DerivedVectorsAreUnrepresentable(t *testing.T) {
	f := newFixture(t, nil)
	for name, mutate := range map[string]func(*wire.StableMap){
		"moved hunk range": func(m *wire.StableMap) { m.Change.Hunks[2].NewRange.Start += 3 },
		"forged hunk id":   func(m *wire.StableMap) { m.Change.Hunks[2].ID = "hunk:sha256:" + strings.Repeat("0", 64) },
		"forged span hash": func(m *wire.StableMap) {
			e := &m.Change.Evidence[0]
			e.SpanSha256 = strings.Repeat("0", 64)
			e.ID = wire.EvidenceIdentity(e.BlobOid, e.Path, e.Span, e.SpanSha256)
			for i := range m.Change.Hunks {
				for j := range m.Change.Hunks[i].Basis {
					if m.Change.Hunks[i].Basis[j].EvidenceID == f.stable.Change.Evidence[0].ID {
						m.Change.Hunks[i].Basis[j].EvidenceID = e.ID
					}
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := cloneMap(f.stable)
			mutate(m)
			header, body, err := Encode(m, false)
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.project(t, header, body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, f.original) {
				t.Fatal("the forged member survived projection; derived mode must recompute it")
			}
		})
	}
	t.Run("forged evidence id", func(t *testing.T) {
		m := cloneMap(f.stable)
		m.Change.Evidence[0].ID = "evidence:sha256:" + strings.Repeat("0", 64)
		if _, _, err := Encode(m, false); err == nil {
			t.Fatal("a forged evidence id must not encode")
		}
	})
}

// CEM2-V0-019: a range edit with a recomputed span hash and swapped evidence
// indices are different, admissible maps (V1-3771 decides their semantics).
func TestCEM2V0019AdmissibleClaimEdits(t *testing.T) {
	f := newFixture(t, nil)
	for name, edit := range map[string]func(*Body){
		"span range edit": func(b *Body) { b.Evidence[0].Start++ },
		"evidence swapped": func(b *Body) {
			b.Hunks[0].Basis[0].Evidence, b.Hunks[1].Basis[0].Evidence = b.Hunks[1].Basis[0].Evidence, b.Hunks[0].Basis[0].Evidence
		},
	} {
		for _, pinned := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/derived", true: "/pinned"}[pinned], func(t *testing.T) {
				header, body := f.encode(t, pinned)
				b := decoded(t, body)
				edit(&b)
				if pinned && name == "span range edit" {
					stale, staleHeader := b, header
					stale.Evidence = append([]EvidenceRow(nil), b.Evidence...)
					staleHeader, staleBody := reseal(t, staleHeader, stale)
					refuse(t, f, staleHeader, staleBody, CodePinnedMismatch, false)
					e := &b.Evidence[0]
					e.SpanSha256 = Sha256Hex([]byte(fixtureBase[b.Paths[e.Path]][e.Start:e.End]))
				}
				h, tampered := reseal(t, header, b)
				got, err := f.project(t, h, tampered)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(got, f.original) {
					t.Fatal("the edit must yield a different map")
				}
				if _, err := wire.ParseStable(got); err != nil {
					t.Fatalf("admissible edit is not valid cem/1.0: %v", err)
				}
			})
		}
	}
}

func cloneMap(src *wire.StableMap) *wire.StableMap {
	m := *src
	m.Change.Evidence = append([]wire.Evidence(nil), src.Change.Evidence...)
	m.Change.Hunks = nil
	for _, h := range src.Change.Hunks {
		h.Basis = append([]wire.Basis(nil), h.Basis...)
		m.Change.Hunks = append(m.Change.Hunks, h)
	}
	return &m
}
