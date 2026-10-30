package doccorpus

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// deltaAddRepoFixture is GitHub #1599's layout: the primary repository holds the records
// provider, and the provider's anchors live in a declared additional repository. The previous
// corpus pins the library at its first commit; the record also holds one anchorless subject.
type deltaAddRepoFixture struct {
	root, lib string
	base      Manifest
	extra     AdditionalRepository
	previous  *Artifact
}

const deltaLibDoc = "# Lib\n\nThe lib parses quokka values.\n\nIt also prints them.\n"

func newDeltaAddRepoFixture(t *testing.T) deltaAddRepoFixture {
	t.Helper()
	root, base := fixture(t)
	lib := t.TempDir()
	git(t, lib, "init", "-q")
	git(t, lib, "config", "user.name", "Corpus test")
	git(t, lib, "config", "user.email", "corpus@example.invalid")
	if err := os.WriteFile(filepath.Join(lib, "doc.md"), []byte(deltaLibDoc), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, lib, "add", ".")
	git(t, lib, "commit", "-qm", "lib")
	revision := git(t, lib, "rev-parse", "HEAD")
	rel, err := filepath.Rel(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	extra := AdditionalRepository{ID: revision, Revision: revision, Root: filepath.ToSlash(rel)}
	anchor := func(line int) Anchor {
		excerpt, err := span([]byte(deltaLibDoc), line, line)
		if err != nil {
			t.Fatal(err)
		}
		return Anchor{Repository: extra.ID, Revision: revision, Path: "doc.md", Blob: git(t, lib, "rev-parse", revision+":doc.md"), SHA256: Digest([]byte(deltaLibDoc)), Start: line, End: line, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}
	}
	ev := func(anchors ...Anchor) Evidence {
		return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: anchors, Limitations: []string{"delta fixture assertion"}}
	}
	loose := Evidence{Derivation: "declared", Trust: "generated", State: "unknown", Freshness: "fresh", Anchors: []Anchor{}, Unknown: "no source span was exported", Limitations: []string{"delta fixture assertion"}}
	record := ProviderRecord{Schema: ProviderSchema, ID: "adapter", Version: "1", Source: base.Repository,
		Subjects: []Subject{
			{"adapter:lib", "capability", "Lib quokka parser", "adapter", ev(anchor(3)), nil},
			{"adapter:loose", "capability", "Loose capability", "adapter", loose, nil},
			{"adapter:print", "capability", "Lib printer", "adapter", ev(anchor(5)), nil},
		},
		Claims:       []Claim{{"adapter:lib-claim", "adapter:lib", "The lib parses quokka values", "adapter", ev(anchor(3))}},
		Relations:    []Relation{},
		Journeys:     []Journey{},
		Observations: []ObservationLink{},
		Capabilities: []CapabilityDeclaration{{"subjects", "present", "fixture"}, {"claims", "present", "fixture"}},
	}
	m := withLibrary(t, base, extra, lib, "adapter")
	m = commitProvider(t, root, m, record, "evidence/provider.json")
	previous, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return deltaAddRepoFixture{root: root, lib: lib, base: base, extra: extra, previous: previous}
}

// withLibrary declares the library at extra's revision with doc.md as an evidence input of provider.
func withLibrary(t *testing.T, m Manifest, extra AdditionalRepository, lib, provider string) Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(lib, "doc.md"))
	if err != nil {
		t.Fatal(err)
	}
	m.AdditionalRepositories = []AdditionalRepository{extra}
	m.Scopes = append(slices.Clone(m.Scopes), Scope{"doc.md", extra.Revision})
	m.Inputs = append(slices.Clone(m.Inputs), Input{Path: "doc.md", Revision: extra.Revision, Blob: git(t, lib, "rev-parse", extra.Revision+":doc.md"), SHA256: Digest(data), Provider: provider, Purpose: "evidence"})
	return m
}

// currentWithLibrary builds a current corpus with no adapter records: an empty base provider
// owns the library input at the library's new revision.
func (f deltaAddRepoFixture) currentWithLibrary(t *testing.T, revision string) (Manifest, *Artifact) {
	t.Helper()
	extra := f.extra
	extra.Revision = revision
	empty := ProviderRecord{Schema: ProviderSchema, ID: "base", Version: "1", Source: f.base.Repository, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{}}
	m := commitProvider(t, f.root, f.base, empty, "evidence/base.json")
	m = withLibrary(t, m, extra, f.lib, "base")
	current, err := Build(context.Background(), f.root, m)
	if err != nil {
		t.Fatal(err)
	}
	return m, current
}

func entriesByID(receipt *DeltaReceipt) map[string]DeltaEntry {
	out := map[string]DeltaEntry{}
	for _, e := range receipt.Entries {
		out[e.ID] = e
	}
	return out
}

func TestDCXV1420DeltaAdditionalRepositoryAnchors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("DCX-V1-420 DCX-V1-421 DCX-V1-422 a same-input delta carries every anchored record and lists nothing else", func(t *testing.T) {
		t.Parallel()
		f := newDeltaAddRepoFixture(t)
		receipt, err := DeltaWith(ctx, f.root, f.previous, f.previous, "artifact", DeltaOptions{Limit: MaxResults, Uncovered: DeltaUncoveredChanged})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range receipt.Entries {
			if e.Class != DeltaCarryForward {
				t.Fatalf("no-op delta listed a non-carry entry: %+v", e)
			}
		}
		got := entriesByID(receipt)
		for _, id := range []string{"adapter:lib", "adapter:lib-claim", "adapter:print"} {
			e, ok := got[id]
			if !ok || e.Class != DeltaCarryForward || len(e.Anchors) != 1 || e.Anchors[0].Repository != f.extra.ID || e.Anchors[0].Current == nil {
				t.Fatalf("%s in the additional repository not carried: %+v", id, e)
			}
		}
		if receipt.Counts[DeltaUnresolved] != 0 || receipt.Counts[DeltaUncovered] != 0 || receipt.Total != receipt.Counts[DeltaCarryForward] {
			t.Fatalf("no-op counts: %+v", receipt.Counts)
		}
		want := []DeltaAnchorless{{Provider: "adapter", RecordKind: "subject", Count: 1, Reason: deltaAnchorlessReason}}
		if !reflect.DeepEqual(receipt.Anchorless, want) {
			t.Fatalf("anchorless summary: %+v", receipt.Anchorless)
		}
		all, err := DeltaWith(ctx, f.root, f.previous, f.previous, "artifact", DeltaOptions{Limit: MaxResults, Anchorless: DeltaAnchorlessList})
		if err != nil {
			t.Fatal(err)
		}
		subjects := 0
		for _, e := range all.Entries {
			if e.Class == DeltaUncovered && e.RecordKind == "subject" {
				subjects++
			}
		}
		if receipt.UncoveredSubjectsOmitted == 0 || subjects != receipt.UncoveredSubjectsOmitted || all.UncoveredSubjectsOmitted != 0 || all.Anchorless != nil {
			t.Fatalf("uncovered subjects: changed omitted %d, all listed %d (%d)", receipt.UncoveredSubjectsOmitted, subjects, all.UncoveredSubjectsOmitted)
		}
		if !slices.ContainsFunc(all.Entries, func(e DeltaEntry) bool {
			return e.ID == "adapter:loose" && e.Class == DeltaUnresolved && e.Reason == deltaAnchorlessReason
		}) || all.Counts[DeltaUnresolved] != 1 {
			t.Fatalf("anchorless record not listed under --anchorless list: %+v", all.Counts)
		}
		if strings.Contains(strings.Join(receipt.Limitations, "\n"), "reported unresolved and never carried") {
			t.Fatalf("stale additional-repository limitation: %v", receipt.Limitations)
		}
		// GitHub #1599: a corpus from an older builder over identical pinned inputs gives the
		// same worklist as the same artifact on both sides.
		older := *f.previous
		older.Builder.Version, older.Builder.Revision = "0.0.0-older", strings.Repeat("a", 40)
		upgrade, err := DeltaWith(ctx, f.root, &older, f.previous, "artifact", DeltaOptions{Limit: MaxResults, Uncovered: DeltaUncoveredChanged})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(upgrade.Entries, receipt.Entries) || !reflect.DeepEqual(upgrade.Counts, receipt.Counts) || !reflect.DeepEqual(upgrade.Anchorless, receipt.Anchorless) {
			t.Fatalf("builder upgrade changed the worklist: %+v", upgrade.Counts)
		}
	})
	t.Run("DCX-V1-420 DCX-V1-424 moved and changed anchors in the additional repository and a carry-forward that builds", func(t *testing.T) {
		t.Parallel()
		f := newDeltaAddRepoFixture(t)
		changed := "# Lib\n\nIntro.\nThe lib parses quokka values.\n\nIt also formats them.\n"
		if err := os.WriteFile(filepath.Join(f.lib, "doc.md"), []byte(changed), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, f.lib, "commit", "-qam", "lib change")
		revision := git(t, f.lib, "rev-parse", "HEAD")
		m, current := f.currentWithLibrary(t, revision)
		receipt, err := Delta(ctx, f.root, f.previous, current, "manifest", 0, MaxResults)
		if err != nil {
			t.Fatal(err)
		}
		got := entriesByID(receipt)
		lib := got["adapter:lib"]
		if lib.Class != DeltaMoved || lib.Anchors[0].Repository != f.extra.ID || lib.Anchors[0].Current == nil || lib.Anchors[0].Current.Start != 4 {
			t.Fatalf("moved additional-repository anchor: %+v", lib)
		}
		if p := got["adapter:print"]; p.Class != DeltaChanged || len(p.Anchors[0].ChangedLines) == 0 || p.Anchors[0].Current == nil || !p.Anchors[0].Current.Estimated {
			t.Fatalf("changed additional-repository anchor: %+v", p)
		}
		record, err := CarryForward(ctx, f.root, f.previous, current, "adapter")
		if err != nil {
			t.Fatal(err)
		}
		if len(record.Subjects) != 1 || len(record.Claims) != 1 {
			t.Fatalf("carried records: %+v", record)
		}
		pinned := record.Claims[0].Evidence.Anchors[0]
		if pinned.Repository != f.extra.ID || pinned.Revision != revision || pinned.Start != 4 || pinned.SpanSHA256 != f.previous.Claims[slices.IndexFunc(f.previous.Claims, func(c Claim) bool { return c.ID == "adapter:lib-claim" })].Evidence.Anchors[0].SpanSHA256 {
			t.Fatalf("carried anchor not re-pinned in the additional repository: %+v", pinned)
		}
		next, err := Build(ctx, f.root, commitProvider(t, f.root, m, *record, "evidence/carried.json"))
		if err != nil {
			t.Fatalf("carried additional-repository record does not build: %v", err)
		}
		again, err := CarryForward(ctx, f.root, f.previous, next, "adapter")
		if err != nil || len(again.Subjects)+len(again.Claims) != 0 {
			t.Fatalf("second carry-forward carried again: %+v %v", again, err)
		}
	})
	t.Run("DCX-V1-425 an undeclared or unreadable additional repository leaves its anchors unresolved with the reason", func(t *testing.T) {
		t.Parallel()
		f := newDeltaAddRepoFixture(t)
		undeclared, err := Build(ctx, f.root, f.base)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := Delta(ctx, f.root, f.previous, undeclared, "manifest", 0, MaxResults)
		if err != nil {
			t.Fatal(err)
		}
		if e := entriesByID(receipt)["adapter:lib"]; e.Class != DeltaUnresolved || !strings.Contains(e.Reason, "which the current manifest does not declare") {
			t.Fatalf("undeclared repository: %+v", e)
		}
		moved := *f.previous
		moved.Manifest.AdditionalRepositories = []AdditionalRepository{f.extra}
		moved.Manifest.AdditionalRepositories[0].Root = "../missing-checkout"
		receipt, err = Delta(ctx, f.root, f.previous, &moved, "artifact", 0, MaxResults)
		if err != nil {
			t.Fatal(err)
		}
		if e := entriesByID(receipt)["adapter:lib"]; e.Class != DeltaUnresolved || !strings.Contains(e.Reason, "checkout is unavailable") {
			t.Fatalf("missing checkout: %+v", e)
		}
	})
	t.Run("DCX-V1-423 the jsonl stream is the whole worklist once, equal to every page", func(t *testing.T) {
		t.Parallel()
		f := newDeltaAddRepoFixture(t)
		options := DeltaOptions{Anchorless: DeltaAnchorlessList}
		stream, err := DeltaStream(ctx, f.root, f.previous, f.previous, "artifact", options)
		if err != nil {
			t.Fatal(err)
		}
		again, err := DeltaStream(ctx, f.root, f.previous, f.previous, "artifact", options)
		if err != nil || !bytes.Equal(stream, again) {
			t.Fatalf("stream not deterministic: %v", err)
		}
		lines := strings.Split(strings.TrimSuffix(string(stream), "\n"), "\n")
		var header DeltaStreamHeader
		if err := json.Unmarshal([]byte(lines[0]), &header); err != nil || header.Schema != DeltaStreamSchema || header.Total != len(lines)-1 {
			t.Fatalf("stream header: %+v %v (%d lines)", header, err, len(lines))
		}
		paged := []DeltaEntry{}
		for offset := 0; ; {
			options.Offset, options.Limit = offset, 2
			page, err := DeltaWith(ctx, f.root, f.previous, f.previous, "artifact", options)
			if err != nil {
				t.Fatal(err)
			}
			paged = append(paged, page.Entries...)
			if !reflect.DeepEqual(page.Counts, header.Counts) || page.Total != header.Total {
				t.Fatalf("page and stream disagree: %+v %+v", page.Counts, header.Counts)
			}
			if page.NextOffset == 0 {
				break
			}
			offset = page.NextOffset
		}
		for i, e := range paged {
			line, err := Encode(e)
			if err != nil || string(line) != lines[i+1]+"\n" {
				t.Fatalf("stream line %d differs from paged entry %+v", i+1, e)
			}
		}
		if len(paged) != header.Total {
			t.Fatalf("paged %d, stream %d", len(paged), header.Total)
		}
	})
	t.Run("DCX-V1-421 DCX-V1-422 an invalid view mode is refused", func(t *testing.T) {
		t.Parallel()
		for _, o := range []DeltaOptions{{Uncovered: "some"}, {Anchorless: "none"}} {
			if _, err := deltaWorklist(ctx, "", nil, nil, o); err == nil {
				t.Fatalf("%+v accepted", o)
			}
		}
	})
}
