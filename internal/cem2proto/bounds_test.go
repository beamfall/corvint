package cem2proto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// firstEvidenceRow is the canonical text of the body's first evidence row.
func firstEvidenceRow(t testing.TB, body []byte) string {
	t.Helper()
	b, err := DecodeBody(body)
	if err != nil {
		t.Fatal(err)
	}
	e := b.Evidence[0]
	row := fmt.Sprintf("[%d,%d,%d]", e.Path, e.Start, e.End)
	if !bytes.Contains(body, []byte(row)) {
		t.Fatalf("row %s not in body", row)
	}
	return row
}

// CEM2-V0-023: a span outside 0 <= start < end is refused from the bytes
// alone, whichever layer would otherwise see it first.
func TestCEM2V0023SpanRefusedBeforeGit(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	row := firstEvidenceRow(t, body)
	path := row[1:strings.Index(row, ",")]
	for name, replacement := range map[string]string{
		"negative start":      "[" + path + ",-5,3]",
		"negative end":        "[" + path + ",0,-1]",
		"empty span":          "[" + path + ",7,7]",
		"inverted span":       "[" + path + ",9,3]",
		"start above range":   "[" + path + ",9007199254740992,9007199254740993]",
		"end above range":     "[" + path + ",0,9007199254740992]",
		"non-integer start":   "[" + path + ",\"0\",3]",
		"fractional end":      "[" + path + ",0,3.5]",
		"exponent end":        "[" + path + ",0,3e2]",
		"start at wire limit": "[" + path + ",9007199254740991,9007199254740991]",
	} {
		t.Run(name, func(t *testing.T) {
			tampered := bytes.Replace(body, []byte(row), []byte(replacement), 1)
			h, err := DecodeHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			h.BodySha256 = Sha256Hex(tampered)
			resealed, err := EncodeHeader(h)
			if err != nil {
				t.Fatal(err)
			}
			refuse(t, f, resealed, tampered, CodeMalformed, true)
		})
	}
}

// CEM2-V0-023: the projection refuses a negative start itself, so an
// in-process Body that skipped DecodeBody cannot slice out of range.
func TestCEM2V0023ResolveRefusesNegativeStart(t *testing.T) {
	f := newFixture(t, nil)
	_, body := f.encode(t, false)
	b := decoded(t, body)
	for name, edit := range map[string]func(*EvidenceRow){
		"negative start": func(e *EvidenceRow) { e.Start, e.End = -5, 3 },
		"both negative":  func(e *EvidenceRow) { e.Start, e.End = -9, -1 },
	} {
		t.Run(name, func(t *testing.T) {
			hostile := b
			hostile.Evidence = append([]EvidenceRow(nil), b.Evidence...)
			edit(&hostile.Evidence[0])
			_, err := resolveEvidence(context.Background(), mustSource(t, f.root), f.base, hostile)
			wantCode(t, err, CodeEvidenceUnresolved)
		})
	}
}

// CEM2-V0-024: a hunk carries at most the cem/1.0 basis bound, checked
// before any Git read.
func TestCEM2V0024BasesPerHunkBound(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	withBases := func(n int) Body {
		b := decoded(t, body)
		b.Hunks[0].Basis = nil
		for len(b.Hunks[0].Basis) < n {
			b.Hunks[0].Basis = append(b.Hunks[0].Basis, BasisRow{Evidence: 0, Relation: 0})
		}
		return b
	}
	h, tampered := reseal(t, header, withBases(wire.MaxBases+1))
	refuse(t, f, h, tampered, CodeBoundExceeded, true)
	_, atBound := reseal(t, header, withBases(wire.MaxBases))
	if _, err := DecodeBody(atBound); err != nil {
		t.Fatalf("a hunk at the %d-basis bound must decode: %v", wire.MaxBases, err)
	}
}

// CEM2-V0-024: aggregate basis references are bounded before any Git read,
// and the bound sits above every body that decodes per-hunk in range.
func TestCEM2V0024ReferenceTotalBound(t *testing.T) {
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	withReferences := func(total int) Body {
		b := decoded(t, body)
		b.Hunks = nil
		for total > 0 {
			n := min(total, wire.MaxBases)
			row := HunkRow{Reason: 0}
			for k := 0; k < n; k++ {
				row.Basis = append(row.Basis, BasisRow{Evidence: 0, Relation: 0})
			}
			b.Hunks = append(b.Hunks, row)
			total -= n
		}
		return b
	}
	h, tampered := reseal(t, header, withReferences(MaxBasisReferences+1))
	refuse(t, f, h, tampered, CodeBoundExceeded, true)
	_, atBound := reseal(t, header, withReferences(MaxBasisReferences))
	if _, err := DecodeBody(atBound); err != nil {
		t.Fatalf("a body at the %d-reference bound must decode: %v", MaxBasisReferences, err)
	}
	if MaxBasisReferences >= wire.MaxHunks*wire.MaxBases {
		t.Fatalf("the aggregate bound %d does not tighten the per-hunk product %d", MaxBasisReferences, wire.MaxHunks*wire.MaxBases)
	}
}

// CEM2-V0-024: the aggregate bound is sound. Its per-reference size is the
// shortest canonical basis object, and adding a basis grows a real
// projection by at least that many bytes, so no body refused by the bound
// could have projected within wire.MaxMapBytes.
func TestCEM2V0024MinProjectedBasisBytes(t *testing.T) {
	shortest := Relations[0]
	for _, relation := range Relations {
		if len(relation) < len(shortest) {
			shortest = relation
		}
	}
	id := wire.EvidenceIdentity(strings.Repeat("a", 40), "p", wire.Span{Start: 0, End: 1}, strings.Repeat("b", 64))
	object := `{"evidenceId":"` + id + `","relation":"` + shortest + `"}`
	if len(object) != minProjectedBasisBytes {
		t.Fatalf("shortest basis object is %d bytes; constant is %d", len(object), minProjectedBasisBytes)
	}
	f := newFixture(t, nil)
	header, body := f.encode(t, false)
	before, err := f.project(t, header, body)
	if err != nil {
		t.Fatal(err)
	}
	b := decoded(t, body)
	b.Hunks[0].Basis = append(b.Hunks[0].Basis, BasisRow{Evidence: 2, Relation: indexOf(Relations, shortest)})
	h, grown := reseal(t, header, b)
	after, err := f.project(t, h, grown)
	if err != nil {
		t.Fatal(err)
	}
	if delta := len(after) - len(before); delta < minProjectedBasisBytes {
		t.Fatalf("one %s basis added %d bytes; the bound assumes at least %d", shortest, delta, minProjectedBasisBytes)
	}
}

// CEM2-V0-024: a body inside every pre-Git bound whose expansion exceeds the
// cem/1.0 map bound is refused as a bound, not reported as a projection.
func TestCEM2V0024ExpandedOutputBound(t *testing.T) {
	const changed = 1101 // 1101*32 references stay under MaxBasisReferences
	var base, target strings.Builder
	for i := 0; i < changed*10; i++ {
		fmt.Fprintf(&base, "l%d\n", i)
		if i%10 == 5 {
			fmt.Fprintf(&target, "m%d\n", i)
		} else {
			fmt.Fprintf(&target, "l%d\n", i)
		}
	}
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	writeTree(t, root, map[string]string{"big.txt": base.String()})
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "base")
	baseOID := git(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"big.txt": target.String()})
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "target")
	targetOID := git(t, root, "rev-parse", "HEAD")
	patchBytes, err := mustSource(t, root).Patch(context.Background(), baseOID, targetOID)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Hunks) != changed {
		t.Fatalf("generated patch has %d hunks; want %d", len(parsed.Hunks), changed)
	}
	b := Body{Paths: []string{"big.txt"}}
	for i := 0; i < wire.MaxEvidence; i++ {
		b.Evidence = append(b.Evidence, EvidenceRow{Path: 0, Start: int64(i), End: int64(i + 1)})
	}
	for i := 0; i < changed; i++ {
		row := HunkRow{Reason: 0}
		for k := 0; k < wire.MaxBases; k++ {
			row.Basis = append(row.Basis, BasisRow{Evidence: (i*wire.MaxBases + k) % wire.MaxEvidence, Relation: 0})
		}
		b.Hunks = append(b.Hunks, row)
	}
	body, err := EncodeBody(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBody(body); err != nil {
		t.Fatalf("the body must pass every pre-Git bound: %v", err)
	}
	header, err := EncodeHeader(Header{Spec: ProtoSpec, Base: baseOID, PatchSha256: Sha256Hex(patchBytes), BodySha256: Sha256Hex(body), Role: wire.StableRoleChange})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Project(context.Background(), mustSource(t, root), targetOID, header, body)
	if got != nil {
		t.Fatalf("an over-bound expansion produced %d bytes", len(got))
	}
	wantCode(t, err, CodeBoundExceeded)
}

// staticSource replays one fixture's Git facts without running Git.
type staticSource struct {
	patch []byte
	blobs map[string]Blob
}

func (s staticSource) Patch(context.Context, string, string) ([]byte, error) { return s.patch, nil }

func (s staticSource) Blobs(_ context.Context, _ string, paths []string) (map[string]Blob, error) {
	out := map[string]Blob{}
	for _, p := range paths {
		if blob, ok := s.blobs[p]; ok {
			out[p] = blob
		}
	}
	return out, nil
}

func staticFixture(t testing.TB) (*fixture, staticSource, Header) {
	t.Helper()
	f := newFixture(t, nil)
	ctx := context.Background()
	src := mustSource(t, f.root)
	patchBytes, err := src.Patch(ctx, f.base, f.target)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := src.Blobs(ctx, f.base, []string{"docs/spec.md", "main.go", "notes.txt", "docs"})
	if err != nil {
		t.Fatal(err)
	}
	header, _, err := Encode(f.stable, false)
	if err != nil {
		t.Fatal(err)
	}
	h, err := DecodeHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	return f, staticSource{patch: patchBytes, blobs: blobs}, h
}

// projectHostile projects body under a header resealed to it and requires a
// typed refusal or a valid cem/1.0 map; a panic fails the caller.
func projectHostile(t *testing.T, src Source, target string, h Header, body []byte) error {
	t.Helper()
	h.BodySha256 = Sha256Hex(body)
	header, err := EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Project(context.Background(), src, target, header, body)
	if err != nil {
		if out != nil {
			t.Fatalf("refusal %v also produced %d bytes", err, len(out))
		}
		var typed *Error
		if !errors.As(err, &typed) {
			t.Fatalf("untyped refusal %v", err)
		}
		return err
	}
	if _, err := wire.ParseStable(out); err != nil {
		t.Fatalf("projection is not cem/1.0: %v", err)
	}
	return nil
}

// hostileBodies are hand-built canonical and non-canonical attacks.
func hostileBodies(body []byte, row string) map[string][]byte {
	path := row[1:strings.Index(row, ",")]
	replace := func(old, new string) []byte { return bytes.Replace(body, []byte(old), []byte(new), 1) }
	deep := strings.Repeat("[", 10000) + strings.Repeat("]", 10000)
	return map[string][]byte{
		"empty":               {},
		"null":                []byte("null"),
		"array root":          []byte("[]"),
		"empty object":        []byte("{}"),
		"deep nesting":        []byte(`{"evidence":[],"hunks":` + deep + `,"paths":[]}`),
		"negative start":      replace(row, "["+path+",-5,3]"),
		"start past blob":     replace(row, "["+path+",1000000,1000001]"),
		"huge span":           replace(row, "["+path+",0,9007199254740991]"),
		"path index negative": replace(row, "[-1,0,3]"),
		"path index huge":     replace(row, "[9007199254740991,0,3]"),
		"short row":           replace(row, "["+path+",0]"),
		"pinned bad digests":  replace(row, "["+path+",0,3,\"x\",\"y\"]"),
		"basis not pair":      []byte(`{"evidence":[[0,0,1]],"hunks":[[0,[0]]],"paths":["main.go"]}`),
		"basis huge index":    []byte(`{"evidence":[[0,0,1]],"hunks":[[0,[9007199254740991,0]]],"paths":["main.go"]}`),
		"reason not int":      []byte(`{"evidence":[],"hunks":[["x"]],"paths":[]}`),
		"empty hunk row":      []byte(`{"evidence":[],"hunks":[[]],"paths":[]}`),
		"path traversal":      []byte(`{"evidence":[[0,0,1]],"hunks":[[1]],"paths":["../x"]}`),
		"invalid utf8":        append([]byte(`{"evidence":[],"hunks":[],"paths":["`), 0xff, '"', ']', '}'),
		"provenance array":    []byte(`{"evidence":[],"hunks":[],"paths":[],"provenance":[]}`),
		"provenance typed":    []byte(`{"evidence":[],"hunks":[],"paths":[],"provenance":{"bindCommit":1,"converter":1,"sourceBlobOid":1,"sourceSha256":1,"sourceSpec":1}}`),
	}
}

// CEM2-V0-025: hostile bodies are refused with a typed error and never panic.
// Some pass DecodeBody (a span past the blob is decidable only from Git) and
// are refused by the projection.
func TestCEM2V0025HostileBodiesNeverPanic(t *testing.T) {
	f, src, h := staticFixture(t)
	_, body := f.encode(t, false)
	for name, hostile := range hostileBodies(body, firstEvidenceRow(t, body)) {
		t.Run(name, func(t *testing.T) {
			if projectHostile(t, src, f.target, h, hostile) == nil {
				t.Fatalf("hostile body %q projected", name)
			}
		})
	}
}

// CEM2-V0-025: arbitrary body bytes never panic the decoder or the projection.
// The seed corpus runs under go test; go test -fuzz extends it.
func FuzzCEM2V0025Project(f *testing.F) {
	fx, src, h := staticFixture(f)
	_, body, err := Encode(fx.stable, false)
	if err != nil {
		f.Fatal(err)
	}
	_, pinned, err := Encode(fx.stable, true)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(body)
	f.Add(pinned)
	for _, hostile := range hostileBodies(body, firstEvidenceRow(f, body)) {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			t.Skip("bounded fuzz input")
		}
		if b, err := DecodeBody(data); err == nil {
			if _, err := EncodeBody(b); err != nil {
				t.Fatalf("decoded body does not re-encode: %v", err)
			}
		}
		_ = projectHostile(t, src, fx.target, h, data)
	})
}
