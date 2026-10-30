package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// deltaFixture builds a previous corpus with one records provider over the
// fixture, then commits a second revision that moves, changes, removes and adds
// source lines, and returns the root, the previous artifact and the current
// (provider-free) manifest.
func deltaFixture(t *testing.T) (string, *Artifact, Manifest) {
	t.Helper()
	root, m := fixture(t)
	rev := m.Repository.Revision
	anchor := func(p string, line int) Anchor { return tkAnchor(t, root, m, rev, p, line, line) }
	ev := func(a Anchor) Evidence {
		return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{a}, Limitations: []string{"delta fixture assertion"}}
	}
	countAnchor := anchor("src/value.go", 4)
	countAnchor.Symbol = "Count"
	viewAnchor := anchor("src/view.ts", 1)
	viewAnchor.Symbol = "showCount"
	reviewed := ev(anchor("src/value.go", 4))
	reviewed.Trust = "reviewed"
	record := ProviderRecord{Schema: ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository,
		Subjects: []Subject{
			{"adapter:count", "capability", "Count", "adapter", ev(countAnchor), nil},
			{"adapter:readme", "document", "Readme", "adapter", ev(anchor("src/readme.md", 3)), nil},
			{"adapter:report", "capability", "Report", "adapter", ev(anchor("src/report.py", 2)), nil},
			{"adapter:test", "test", "Count test", "adapter", ev(anchor("src/value_test.go", 3)), nil},
			{"adapter:view", "ui_surface", "View", "adapter", ev(viewAnchor), nil},
		},
		Claims: []Claim{
			{"adapter:count-claim", "adapter:count", "Count returns one", "adapter", reviewed},
			{"adapter:orphan-claim", "adapter:report", "Report is documented", "adapter", ev(anchor("src/readme.md", 1))},
			{"adapter:readme-claim", "adapter:readme", "Readme explains counting", "adapter", ev(anchor("src/readme.md", 3))},
			{"adapter:report-claim", "adapter:report", "Report returns one", "adapter", ev(anchor("src/report.py", 2))},
		},
		Relations:    []Relation{{"adapter:rel", "adapter:count", "adapter:readme", "documents", "adapter", ev(anchor("src/readme.md", 1))}},
		Journeys:     []Journey{},
		Observations: []ObservationLink{},
		Capabilities: []CapabilityDeclaration{{"subjects", "present", "fixture"}, {"claims", "present", "fixture"}, {"relations", "present", "fixture"}},
	}
	m = commitProvider(t, root, m, record, "evidence/provider.json")
	previous, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	write := func(p, data string) {
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("src/value.go", "package value\n\n// Extra notes.\n// More notes.\n\n// Count returns one.\nfunc Count() int { return 1 }\n")
	write("src/report.py", "def report_count():\n    return 2\n")
	write("src/show.ts", "export function showCount() { return 1; }\n")
	write("src/extra.go", "package value\n\nfunc Extra() int { return 2 }\n")
	git(t, root, "rm", "-q", "src/view.ts", "src/value_test.go")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "second revision")
	current, err := Inventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), "src", "2026-09-20T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return root, previous, current
}

// commitProvider commits record at path and declares it as a records provider.
func commitProvider(t *testing.T, root string, m Manifest, record ProviderRecord, path string) Manifest {
	t.Helper()
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", path)
	git(t, root, "commit", "-qm", "provider "+path)
	rev := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(slices.Clone(m.Providers), Provider{ID: record.ID, Kind: "records", Version: record.Version, Revision: rev, Record: path})
	m.Scopes = append(slices.Clone(m.Scopes), Scope{path, rev})
	m.Inputs = append(slices.Clone(m.Inputs), Input{Path: path, Revision: rev, Blob: git(t, root, "rev-parse", rev+":"+path), SHA256: Digest(data), Provider: record.ID, Purpose: "provider"})
	return m
}

func deltaEntries(t *testing.T, root string, previous *Artifact, current *Artifact) map[string]DeltaEntry {
	t.Helper()
	receipt, err := Delta(context.Background(), root, previous, current, "manifest", 0, MaxResults)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]DeltaEntry{}
	for _, e := range receipt.Entries {
		out[e.ID] = e
	}
	return out
}

func TestDCXV1360DeltaWorklistClasses(t *testing.T) {
	t.Parallel()
	root, previous, m := deltaFixture(t)
	current, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("DCX-V1-360 DCX-V1-361 DCX-V1-362 DCX-V1-363 classes", func(t *testing.T) {
		got := deltaEntries(t, root, previous, current)
		want := map[string]string{
			"adapter:readme": DeltaCarryForward, "adapter:readme-claim": DeltaCarryForward, "adapter:rel": DeltaCarryForward,
			"adapter:count": DeltaMoved, "adapter:count-claim": DeltaMoved, "adapter:view": DeltaMoved,
			"adapter:report": DeltaChanged, "adapter:report-claim": DeltaChanged,
			"adapter:test":         DeltaRemoved,
			"adapter:orphan-claim": DeltaUnresolved,
		}
		for id, class := range want {
			if got[id].Class != class {
				t.Errorf("%s class %q (%s), want %q", id, got[id].Class, got[id].Reason, class)
			}
		}
		count := got["adapter:count"].Anchors[0]
		if count.Current == nil || count.Current.Start != 7 || count.Current.SpanSHA256 != count.Previous.SpanSHA256 {
			t.Fatalf("moved anchor not re-pinned with its span hash: %+v", count)
		}
		view := got["adapter:view"].Anchors[0]
		if view.Current == nil || view.Current.Path != "src/show.ts" {
			t.Fatalf("symbol anchor did not re-resolve across paths: %+v", view)
		}
		report := got["adapter:report-claim"].Anchors[0]
		if report.Current == nil || !report.Current.Estimated || report.Current.SpanSHA256 == report.Previous.SpanSHA256 || len(report.ChangedLines) != 1 || report.ChangedLines[0].OldStart != 2 || report.ChangedLines[0].NewStart != 2 {
			t.Fatalf("changed anchor lacks mapped span and changed lines: %+v", report)
		}
		if !strings.Contains(got["adapter:orphan-claim"].Reason, "adapter:report") {
			t.Fatalf("orphan claim was not demoted for its unconfirmed subject: %+v", got["adapter:orphan-claim"])
		}
	})
	t.Run("DCX-V1-364 uncovered regions", func(t *testing.T) {
		got := deltaEntries(t, root, previous, current)
		for _, id := range []string{"src/value.go:3-5", "src/extra.go:1-3", "src/show.ts:1-1"} {
			if got[id].Class != DeltaUncovered || got[id].Region == nil {
				t.Errorf("%s not uncovered: %+v", id, got[id])
			}
		}
		if _, ok := got["src/report.py:2-2"]; ok {
			t.Fatal("a changed line covered by a previous claim was reported uncovered")
		}
		if len(got["src/extra.go:1-3"].Subjects) == 0 {
			t.Fatal("uncovered region names no current subjects")
		}
	})
	t.Run("DCX-V1-360 DCX-V1-367 paging and determinism", func(t *testing.T) {
		first, err := Delta(context.Background(), root, previous, current, "manifest", 0, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Entries) != 2 || first.NextOffset != 2 || first.Total < 10 || first.Counts[DeltaUncovered] != 3 {
			t.Fatalf("page: %+v", first)
		}
		again, err := Delta(context.Background(), root, previous, current, "manifest", 0, 2)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := Encode(first)
		b, _ := Encode(again)
		if !bytes.Equal(a, b) {
			t.Fatal("delta receipt is not deterministic")
		}
		last, err := Delta(context.Background(), root, previous, current, "manifest", first.Total-1, 2)
		if err != nil || len(last.Entries) != 1 || last.NextOffset != 0 {
			t.Fatalf("last page: %+v %v", last, err)
		}
		if first.Entries[0].Class != DeltaRemoved {
			t.Fatalf("worklist does not lead with removed records: %+v", first.Entries[0])
		}
		if _, err := Delta(context.Background(), root, previous, current, "manifest", 0, MaxResults+1); err == nil {
			t.Fatal("over-bound limit admitted")
		}
	})
	t.Run("DCX-V1-368 read-only", func(t *testing.T) {
		head, status := git(t, root, "rev-parse", "HEAD"), git(t, root, "status", "--porcelain")
		if _, err := Delta(context.Background(), root, previous, current, "manifest", 0, 8); err != nil {
			t.Fatal(err)
		}
		if _, err := CarryForward(context.Background(), root, previous, current, "adapter"); err != nil {
			t.Fatal(err)
		}
		if git(t, root, "rev-parse", "HEAD") != head || git(t, root, "status", "--porcelain") != status {
			t.Fatal("delta mutated the repository")
		}
	})
	t.Run("DCX-V1-369 refusals and unresolved anchors", func(t *testing.T) {
		other := *previous
		other.Manifest.Repository.ID = "other-repository"
		if _, err := Delta(context.Background(), root, &other, current, "manifest", 0, 8); err == nil || !strings.Contains(err.Error(), "different repositories") {
			t.Fatalf("cross-repository delta admitted: %v", err)
		}
		if _, err := CarryForward(context.Background(), root, previous, current, "native"); err == nil {
			t.Fatal("carry-forward admitted a non-records provider")
		}
		data, _ := Encode(previous)
		mutated, err := ParseArtifact(data)
		if err != nil {
			t.Fatal(err)
		}
		for i := range mutated.Subjects {
			if mutated.Subjects[i].ID == "adapter:readme" {
				mutated.Subjects[i].Evidence.Anchors[0].Repository = "additional"
			}
			if mutated.Subjects[i].ID == "adapter:count" {
				mutated.Subjects[i].Evidence.Anchors[0].SpanSHA256 = strings.Repeat("0", 64)
				mutated.Subjects[i].Evidence.Anchors[0].Symbol = ""
			}
		}
		got := deltaEntries(t, root, mutated, current)
		if got["adapter:readme"].Class != DeltaUnresolved || !strings.Contains(got["adapter:readme"].Reason, "which the current manifest does not declare") {
			t.Fatalf("undeclared-repository anchor (DCX-V1-425): %+v", got["adapter:readme"])
		}
		if got["adapter:count"].Class != DeltaChanged {
			t.Fatalf("an unconfirmable span hash was not changed: %+v", got["adapter:count"])
		}
		if got["adapter:count-claim"].Class != DeltaUnresolved {
			t.Fatalf("claim of an unconfirmed subject stayed carried: %+v", got["adapter:count-claim"])
		}
	})
}

func TestDCXV1366CarryForwardFeedsNextBuild(t *testing.T) {
	t.Parallel()
	root, previous, m := deltaFixture(t)
	current, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	record, err := CarryForward(context.Background(), root, previous, current, "adapter")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, s := range record.Subjects {
		ids = append(ids, s.ID)
	}
	for _, c := range record.Claims {
		ids = append(ids, c.ID)
	}
	for _, r := range record.Relations {
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	want := []string{"adapter:count", "adapter:count-claim", "adapter:readme", "adapter:readme-claim", "adapter:rel", "adapter:view"}
	if !slices.Equal(ids, want) {
		t.Fatalf("carried %v, want %v", ids, want)
	}
	next, err := Build(context.Background(), root, commitProvider(t, root, m, *record, "evidence/carried.json"))
	if err != nil {
		t.Fatalf("carried record does not feed the next build: %v", err)
	}
	previousClaims := map[string]Claim{}
	for _, c := range previous.Claims {
		previousClaims[c.ID] = c
	}
	for _, c := range next.Claims {
		old, ok := previousClaims[c.ID]
		if !ok {
			continue
		}
		if c.Text != old.Text || c.Subject != old.Subject || c.Evidence.Trust != old.Evidence.Trust || c.Evidence.ReportedTrust != old.Evidence.ReportedTrust || !slices.Equal(c.Evidence.Limitations, old.Evidence.Limitations) {
			t.Fatalf("carried claim %s changed: %+v vs %+v", c.ID, c.Evidence, old.Evidence)
		}
		for i, a := range c.Evidence.Anchors {
			if a.SpanSHA256 != old.Evidence.Anchors[i].SpanSHA256 || a.Revision != m.Repository.Revision {
				t.Fatalf("carried anchor not keyed by span hash at the current revision: %+v", a)
			}
		}
	}
	// The second delta against the new build carries nothing again: every
	// confirmed record is already present.
	again, err := CarryForward(context.Background(), root, previous, next, "adapter")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Subjects)+len(again.Claims)+len(again.Relations) != 0 {
		t.Fatalf("records already present were carried twice: %+v", again)
	}
}

func TestDCXV1363LineDiff(t *testing.T) {
	t.Parallel()
	lines := func(s string) []string { return splitLines([]byte(s)) }
	hunks := lineDiff(lines("a\nb\nc\nd\ne\n"), lines("a\nx\nc\nd\ny\nz\ne\n"), deltaMaxEdits)
	want := []DeltaHunk{{OldStart: 2, OldCount: 1, NewStart: 2, NewCount: 1}, {OldStart: 5, OldCount: 0, NewStart: 5, NewCount: 2}}
	if !slices.Equal(hunks, want) {
		t.Fatalf("hunks %+v, want %+v", hunks, want)
	}
	if got, ok := mapLine(hunks, 4, false); got != 4 || !ok {
		t.Fatalf("unchanged line mapped to %d", got)
	}
	if got, ok := mapLine(hunks, 5, false); got != 7 || !ok {
		t.Fatalf("line after insertion mapped to %d", got)
	}
	if got := lineDiff(lines("a\nb\n"), lines("a\nb\n"), deltaMaxEdits); len(got) != 0 {
		t.Fatalf("identical inputs differ: %+v", got)
	}
	coarse := lineDiff(lines("a\nb\nc\nd\n"), lines("w\nx\ny\nz\n"), 2)
	if len(coarse) != 1 || !coarse[0].Coarse || coarse[0].OldCount != 4 || coarse[0].NewCount != 4 {
		t.Fatalf("bounded diff did not degrade to one coarse hunk: %+v", coarse)
	}
	d := &deltaRun{}
	data := []byte("x\ndup\ny\ndup\n")
	a := Anchor{Start: 1, End: 1, SpanSHA256: Digest([]byte("dup\n"))}
	if same, others, bounded := d.place(data, a, nil); same || bounded || len(others) != 2 {
		t.Fatalf("ambiguous span placement: %v %v %v", same, others, bounded)
	}
}
