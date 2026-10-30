package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

func deltaEvidence(anchors ...Anchor) Evidence {
	return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: anchors, Limitations: []string{"delta repair fixture"}}
}

func deltaProviderRecord(m Manifest, id string, subjects []Subject, claims []Claim) ProviderRecord {
	caps := []CapabilityDeclaration{}
	if len(subjects) > 0 {
		caps = append(caps, CapabilityDeclaration{"subjects", "present", "fixture"})
	}
	if len(claims) > 0 {
		caps = append(caps, CapabilityDeclaration{"claims", "present", "fixture"})
	}
	return ProviderRecord{Schema: ProviderSchema, ID: id, Version: "1", Source: m.Repository, Subjects: subjects, Claims: claims, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: caps}
}

func commitEdits(t *testing.T, root string, edits map[string]string) Manifest {
	t.Helper()
	for p, data := range edits {
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "edit")
	m, err := Inventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), "src", "2026-09-20T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDCXV1360DeltaRepairs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("DCX-V1-362 DCX-V1-363 end-of-file spans and changed symbol lines", func(t *testing.T) {
		t.Parallel()
		root, m := fixture(t)
		rev := m.Repository.Revision
		// Lines 4..5 and 3..4 end on the empty line after the final newline,
		// which the build's span numbering admits.
		eof := tkAnchor(t, root, m, rev, "src/value.go", 4, 5)
		eofChanged := tkAnchor(t, root, m, rev, "src/readme.md", 3, 4)
		view := tkAnchor(t, root, m, rev, "src/view.ts", 1, 1)
		view.Symbol = "showCount"
		record := deltaProviderRecord(m, "adapter", []Subject{
			{"adapter:eof", "capability", "Count", "adapter", deltaEvidence(eof), nil},
			{"adapter:eof-changed", "document", "Readme", "adapter", deltaEvidence(eofChanged), nil},
			{"adapter:view", "ui_surface", "View", "adapter", deltaEvidence(view), nil},
		}, nil)
		previous, err := Build(ctx, root, commitProvider(t, root, m, record, "evidence/provider.json"))
		if err != nil {
			t.Fatal(err)
		}
		cur := commitEdits(t, root, map[string]string{
			"src/value.go":  "// lead\npackage value\n\n// Count returns one.\nfunc Count() int { return 1 }\n",
			"src/readme.md": "# Count\n\nCounts two values.\n",
			"src/view.ts":   "export function showCount() { return 2; }\n",
		})
		current, err := Build(ctx, root, cur)
		if err != nil {
			t.Fatal(err)
		}
		got := deltaEntries(t, root, previous, current)
		if e := got["adapter:eof"]; e.Class != DeltaMoved || e.Anchors[0].Current == nil || e.Anchors[0].Current.Start != 5 || e.Anchors[0].Current.End != 6 {
			t.Fatalf("end-of-file span not moved: %+v", e)
		}
		if e := got["adapter:eof-changed"]; e.Class != DeltaChanged || len(e.Anchors[0].ChangedLines) == 0 {
			t.Fatalf("changed end-of-file span not classified with changed lines: %+v", e)
		}
		e := got["adapter:view"]
		if e.Class != DeltaChanged || !strings.Contains(e.Reason+e.Anchors[0].Reason, "symbol") {
			t.Fatalf("TypeScript symbol anchor not changed through symbol re-resolution: %+v", e)
		}
		if a := e.Anchors[0]; len(a.ChangedLines) != 1 || a.ChangedLines[0].OldStart != 1 || a.ChangedLines[0].NewStart != 1 || a.Current == nil {
			t.Fatalf("changed symbol anchor lacks changed lines: %+v", a)
		}
	})
	t.Run("DCX-V1-364 uncovered sub-ranges and uncited subjects", func(t *testing.T) {
		t.Parallel()
		root, previous, m := deltaFixture(t)
		line := tkAnchor(t, root, m, m.Repository.Revision, "src/extra.go", 1, 1)
		lonely := tkAnchor(t, root, m, m.Repository.Revision, "src/extra.go", 3, 3)
		record := deltaProviderRecord(m, "cover", []Subject{
			{"cover:extra", "capability", "Extra", "cover", deltaEvidence(line), nil},
			{"cover:lonely", "capability", "Lonely", "cover", deltaEvidence(lonely), nil},
		}, []Claim{{"cover:extra-claim", "cover:extra", "Extra package line", "cover", deltaEvidence(line)}})
		current, err := Build(ctx, root, commitProvider(t, root, m, record, "evidence/cover.json"))
		if err != nil {
			t.Fatal(err)
		}
		got := deltaEntries(t, root, previous, current)
		if e := got["src/extra.go:2-3"]; e.Class != DeltaUncovered || e.Region == nil || e.Region.Start != 2 {
			t.Fatalf("uncovered sub-range of a partly covered new file missing: %+v", e)
		}
		if _, ok := got["src/extra.go:1-3"]; ok {
			t.Fatal("a partly covered region was reported whole")
		}
		if e := got["cover:lonely"]; e.Class != DeltaUncovered || e.RecordKind != "subject" {
			t.Fatalf("subject no claim names is not uncovered: %+v", e)
		}
		if _, ok := got["cover:extra"]; ok {
			t.Fatal("a cited subject was reported uncovered")
		}
		for _, c := range []struct {
			s, e   int
			covers [][2]int
			want   [][2]int
		}{
			{1, 10, [][2]int{{3, 4}, {1, 1}, {6, 20}}, [][2]int{{2, 2}, {5, 5}}},
			{1, 5, nil, [][2]int{{1, 5}}},
			{2, 4, [][2]int{{1, 9}}, [][2]int{}},
			{1, 6, [][2]int{{2, 3}, {3, 4}}, [][2]int{{1, 1}, {5, 6}}},
		} {
			if got := uncoveredRanges(c.s, c.e, c.covers); !slices.Equal(got, c.want) {
				t.Errorf("uncoveredRanges(%d, %d, %v) = %v, want %v", c.s, c.e, c.covers, got, c.want)
			}
		}
	})
	t.Run("DCX-V1-365 DCX-V1-366 cross-provider dependency refused until built", func(t *testing.T) {
		t.Parallel()
		root, m := fixture(t)
		rev := m.Repository.Revision
		a := tkAnchor(t, root, m, rev, "src/readme.md", 1, 1)
		m = commitProvider(t, root, m, deltaProviderRecord(m, "other", []Subject{{"other:s", "document", "Readme", "other", deltaEvidence(a), nil}}, nil), "evidence/other.json")
		m = commitProvider(t, root, m, deltaProviderRecord(m, "adapter", nil, []Claim{{"adapter:c", "other:s", "Readme names the count", "adapter", deltaEvidence(a)}}), "evidence/adapter.json")
		previous, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		cur := commitEdits(t, root, map[string]string{"src/report.py": "def report_count():\n    return 2\n"})
		current, err := Build(ctx, root, cur)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CarryForward(ctx, root, previous, current, "adapter"); err == nil || !strings.Contains(err.Error(), "adapter:c needs other:s (carried by provider other)") {
			t.Fatalf("carry-forward with a dependency carried by another provider admitted: %v", err)
		}
		other, err := CarryForward(ctx, root, previous, current, "other")
		if err != nil {
			t.Fatal(err)
		}
		withOther := commitProvider(t, root, cur, *other, "evidence/other-carried.json")
		next, err := Build(ctx, root, withOther)
		if err != nil {
			t.Fatal(err)
		}
		adapter, err := CarryForward(ctx, root, previous, next, "adapter")
		if err != nil {
			t.Fatal(err)
		}
		if len(adapter.Claims) != 1 || adapter.Claims[0].ID != "adapter:c" {
			t.Fatalf("claim not carried once its subject is built: %+v", adapter)
		}
		if _, err := Build(ctx, root, commitProvider(t, root, withOther, *adapter, "evidence/adapter-carried.json")); err != nil {
			t.Fatalf("carried claim does not import: %v", err)
		}
	})
	t.Run("DCX-V1-367 read bound checked before the body is read", func(t *testing.T) {
		t.Parallel()
		root, m := fixture(t)
		inputs := map[string]Input{}
		for _, in := range m.Inputs {
			inputs[in.Path] = in
		}
		value, view := inputs["src/value.go"], inputs["src/view.ts"]
		size := len("package value\n\n// Count returns one.\nfunc Count() int { return 1 }\n")
		ictx := gitstatus.WithIsolation(ctx)
		auth, err := gitauth.Open(root, newGitBudget(ictx))
		if err != nil {
			t.Fatal(err)
		}
		release := auth.BeginObjectSession()
		defer release()
		run := func(limit int) *deltaRun {
			return &deltaRun{auth: auth, pre: newPathPrefetch(m.Repository.Revision), blobs: map[string][]byte{}, blobReasons: map[string]string{}, readLimit: limit}
		}
		d := run(size - 1)
		if data, reason, err := d.blob(ictx, value.Blob, value.SHA256); err != nil || data != nil || !strings.Contains(reason, "bytes left under the 64 MiB delta read bound") || d.read != 0 {
			t.Fatalf("over-budget blob admitted: %q %q %v read=%d", data, reason, err, d.read)
		}
		d = run(size)
		if _, reason, err := d.blob(ictx, value.Blob, value.SHA256); err != nil || reason != "" || d.read != size {
			t.Fatalf("blob at the exact budget refused: %q %v read=%d", reason, err, d.read)
		}
		if _, reason, err := d.blob(ictx, view.Blob, view.SHA256); err != nil || !strings.Contains(reason, "read bound was reached before blob") || d.read != size {
			t.Fatalf("blob after an exhausted budget admitted: %q %v read=%d", reason, err, d.read)
		}
	})
	t.Run("DCX-V1-367 hashing bound on the unchanged path and placement windows", func(t *testing.T) {
		t.Parallel()
		d := &deltaRun{hashLimit: 4}
		lines := spanLines([]byte("dup\nx\n"))
		if h, ok := d.spanHash(lines, 1, 1); !ok || h != Digest([]byte("dup\n")) || d.hashed != 4 {
			t.Fatalf("span at the exact bound refused: %v hashed=%d", ok, d.hashed)
		}
		if _, ok := d.spanHash(lines, 2, 2); ok || d.hashed != 4 {
			t.Fatalf("span past the bound hashed: hashed=%d", d.hashed)
		}
		data := []byte("x\ndup\n")
		pinned := Anchor{Start: 2, End: 2, SpanSHA256: Digest([]byte("dup\n"))}
		if same, _, bounded := (&deltaRun{hashLimit: 4}).place(data, pinned, nil); !same || bounded {
			t.Fatalf("pinned window at the exact bound: same=%v bounded=%v", same, bounded)
		}
		if same, _, bounded := (&deltaRun{hashLimit: 3}).place(data, pinned, nil); same || !bounded {
			t.Fatalf("pinned window past the bound: same=%v bounded=%v", same, bounded)
		}
		scan := Anchor{Start: 1, End: 1, SpanSHA256: Digest([]byte("dup\n"))}
		if _, others, bounded := (&deltaRun{hashLimit: 6}).place(data, scan, nil); bounded || !slices.Equal(others, []int{2}) {
			t.Fatalf("scan at the exact bound: %v %v", others, bounded)
		}
		if _, _, bounded := (&deltaRun{hashLimit: 5}).place(data, scan, nil); !bounded {
			t.Fatal("scan window past the bound was hashed")
		}

		root, previous, m := deltaFixture(t)
		current, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		run, err := classifyDelta(ctx, root, previous, current)
		if err != nil {
			t.Fatal(err)
		}
		var readme Anchor
		for _, s := range previous.Subjects {
			if s.ID == "adapter:readme" {
				readme = s.Evidence.Anchors[0]
			}
		}
		width := len("Counts a value.\n")
		run.hashed, run.hashLimit, run.verified = 0, width-1, nil
		if da, _, err := run.classifyAnchor(ctx, readme); err != nil || da.State != DeltaUnresolved || da.Reason != deltaHashBoundReason {
			t.Fatalf("unchanged-blob confirmation skipped the hashing bound: %+v %v", da, err)
		}
		run.hashed, run.hashLimit, run.verified = 0, width, nil
		if da, _, err := run.classifyAnchor(ctx, readme); err != nil || da.State != DeltaCarryForward || run.hashed != width {
			t.Fatalf("unchanged-blob confirmation at the exact bound: %+v %v hashed=%d", da, err, run.hashed)
		}
	})
}
