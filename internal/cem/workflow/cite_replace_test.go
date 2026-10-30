package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCiteReplaceCEMV1694 is CEM-V1-694 and CEM-V1-695 (V1-3168), positive
// first: a guarded replacement drops only the named hunk's earlier citations
// and keeps every other hunk's, then a stale digest, a stale base, an invalid
// span, a policy refusal and an interrupted write each leave the map's bytes
// unchanged.
func TestCiteReplaceCEMV1694(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	cite := func(options CiteOptions) error {
		options.MapPath = wire.ExcludedCEMPath
		_, err := openSession(t, root).Cite(ctx(), options)
		return err
	}
	read := func() ([]byte, *wire.Map) {
		raw, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
		if err != nil {
			t.Fatal(err)
		}
		return raw, document
	}
	for _, row := range []CiteOptions{
		{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "implementation"},
		{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "2:2", Relation: "specification"},
		{Hunk: "2", EvidencePath: "docs/rule.txt", Lines: "3:3", Relation: "specification"},
	} {
		if err := cite(row); err != nil {
			t.Fatal(err)
		}
	}
	_, before := read()
	digest := wire.DocumentHunkDigest(before, 0)
	untouched := wire.DocumentHunkDigest(before, 1)

	if err := cite(CiteOptions{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "3:3", Relation: "decision", Replace: true, ExpectedHunkDigest: digest}); err != nil {
		t.Fatalf("guarded replacement refused: %v", err)
	}
	replaced, after := read()
	if got := after.Hunks[0].Basis; len(got) != 1 || got[0].Relation != "decision" || after.Hunks[0].Disposition != "supported" {
		t.Fatalf("hunk 1 basis after replacement = %+v", got)
	}
	if wire.DocumentHunkDigest(after, 1) != untouched {
		t.Fatal("replacing hunk 1 changed hunk 2")
	}
	if len(after.Evidence) != 1 {
		t.Fatalf("evidence after replacement = %+v; want the earlier hunk 1 spans pruned and the shared span kept", after.Evidence)
	}

	current := wire.DocumentHunkDigest(after, 0)
	staleBase := wire.HunkDigest(target, after.PatchSha256, after.Hunks[0].ID, after.Hunks[0].Disposition, after.Hunks[0].Reason, after.Hunks[0].Basis)
	stalePatch := wire.HunkDigest(after.BaseRevision, digest, after.Hunks[0].ID, after.Hunks[0].Disposition, after.Hunks[0].Reason, after.Hunks[0].Basis)
	corvintDir := filepath.Join(root, ".corvint")
	for _, tc := range []struct {
		name   string
		row    CiteOptions
		code   string
		before func()
		after  func()
	}{
		{"stale old digest", CiteOptions{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "implementation", Replace: true, ExpectedHunkDigest: digest}, cemcode.HunkDigestMismatch, nil, nil},
		{"stale base", CiteOptions{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "implementation", Replace: true, ExpectedHunkDigest: staleBase}, cemcode.HunkDigestMismatch, nil, nil},
		{"stale patch", CiteOptions{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "implementation", Replace: true, ExpectedHunkDigest: stalePatch}, cemcode.HunkDigestMismatch, nil, nil},
		{"invalid span", CiteOptions{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "9:9", Relation: "implementation", Replace: true, ExpectedHunkDigest: current}, cemcode.LineRangeOutOfRange, nil, nil},
		{"policy: evidence absent at base", CiteOptions{Hunk: "1", EvidencePath: "src/new.txt", Lines: "1:1", Relation: "implementation", Replace: true, ExpectedHunkDigest: current}, cemcode.MissingEvidence, nil, nil},
		{"interrupted write", CiteOptions{Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "implementation", Replace: true, ExpectedHunkDigest: current}, cemcode.PublishFailed,
			// The staged output cannot be created beside the map, so the
			// publication fails after every check passed.
			func() { _ = os.Chmod(corvintDir, 0o555) }, func() { _ = os.Chmod(corvintDir, 0o755) }},
	} {
		// A read-only directory does not block file creation on Windows.
		if tc.before != nil && runtime.GOOS == "windows" {
			continue
		}
		if tc.before != nil {
			tc.before()
		}
		err := cite(tc.row)
		if tc.after != nil {
			tc.after()
		}
		if code := cemcode.CodeOf(err); code != tc.code {
			t.Errorf("%s: got %v, want %s", tc.name, err, tc.code)
		}
		if raw, _ := read(); !bytes.Equal(raw, replaced) {
			t.Errorf("%s: a refused replacement changed the map", tc.name)
		}
	}

	// The guarded mark refuses a stale digest and applies the current one.
	mark := func(digest string) error {
		_, err := openSession(t, root).Mark(ctx(), MarkOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown", Reason: "conflicting-evidence", ExpectedHunkDigest: digest})
		return err
	}
	if err := mark(digest); cemcode.CodeOf(err) != cemcode.HunkDigestMismatch {
		t.Fatalf("stale guarded mark: %v", err)
	}
	if raw, _ := read(); !bytes.Equal(raw, replaced) {
		t.Fatal("a refused guarded mark changed the map")
	}
	if err := mark(current); err != nil {
		t.Fatalf("guarded mark refused: %v", err)
	}
	if _, final := read(); final.Hunks[0].Disposition != "unknown" || wire.DocumentHunkDigest(final, 1) != untouched {
		t.Fatalf("guarded mark result = %+v", final.Hunks[0])
	}
}
