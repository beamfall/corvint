package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// deltaLibrary commits doc.md holding text in a fresh repository beside root and returns its
// declaration and an anchor on line 1.
func deltaLibrary(t *testing.T, root, text string) (AdditionalRepository, Input, Anchor) {
	t.Helper()
	lib := t.TempDir()
	git(t, lib, "init", "-q")
	git(t, lib, "config", "user.name", "Corpus test")
	git(t, lib, "config", "user.email", "corpus@example.invalid")
	if err := os.WriteFile(filepath.Join(lib, "doc.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, lib, "add", ".")
	git(t, lib, "commit", "-qm", "lib")
	revision := git(t, lib, "rev-parse", "HEAD")
	rel, err := filepath.Rel(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	blob := git(t, lib, "rev-parse", revision+":doc.md")
	excerpt, err := span([]byte(text), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	extra := AdditionalRepository{ID: revision, Revision: revision, Root: filepath.ToSlash(rel)}
	in := Input{Path: "doc.md", Revision: revision, Blob: blob, SHA256: Digest([]byte(text)), Provider: "adapter", Purpose: "evidence"}
	anchor := Anchor{Repository: extra.ID, Revision: revision, Path: "doc.md", Blob: blob, SHA256: in.SHA256, Start: 1, End: 1, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}
	return extra, in, anchor
}

func TestDCXV1420DeltaSharedReadBound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, m := fixture(t)
	const size = 4096
	texts := []string{"alpha quokka\n" + strings.Repeat("a", size-13), "bravo quokka\n" + strings.Repeat("b", size-13)}
	record := ProviderRecord{Schema: ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{{"subjects", "present", "fixture"}}}
	for i, text := range texts {
		extra, in, anchor := deltaLibrary(t, root, text)
		m.AdditionalRepositories = append(m.AdditionalRepositories, extra)
		m.Scopes = append(m.Scopes, Scope{"doc.md", extra.Revision})
		m.Inputs = append(m.Inputs, in)
		ev := Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"delta fixture assertion"}}
		record.Subjects = append(record.Subjects, Subject{"adapter:lib-" + string(rune('a'+i)), "capability", "Lib quokka " + string(rune('a'+i)), "adapter", ev, nil})
	}
	slices.SortFunc(m.AdditionalRepositories, func(x, y AdditionalRepository) int { return strings.Compare(x.ID, y.ID) })
	m = commitProvider(t, root, m, record, "evidence/provider.json")
	a, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	// Each repository's blob fits the bound alone; both together do not.
	limit := size + size/2
	run, err := classifyDeltaWithin(ctx, root, a, a, limit)
	if err != nil {
		t.Fatal(err)
	}
	prefetched := run.pre.heldBytes()
	for _, repo := range run.extra {
		for _, pin := range repo.pins {
			prefetched += pin.pre.heldBytes()
		}
	}
	if prefetched > limit || run.read > limit || run.read < prefetched {
		t.Fatalf("delta read %d bytes (%d prefetched) across repositories, bound %d", run.read, prefetched, limit)
	}
	states := map[string]int{}
	for _, r := range run.records {
		if strings.HasPrefix(r.entry.ID, "adapter:lib-") {
			states[r.entry.Class]++
			if r.entry.Class == DeltaUnresolved && !strings.Contains(r.entry.Reason, "delta read bound") {
				t.Fatalf("over-bound anchor unresolved without the bound reason: %+v", r.entry)
			}
		}
	}
	if states[DeltaCarryForward] != 1 || states[DeltaUnresolved] != 1 {
		t.Fatalf("shared bound classes: %v", states)
	}
	full, err := classifyDeltaWithin(ctx, root, a, a, 3*size)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range full.records {
		if strings.HasPrefix(r.entry.ID, "adapter:lib-") && r.entry.Class != DeltaCarryForward {
			t.Fatalf("anchor within the bound not carried: %+v", r.entry)
		}
	}
	if full.read < 2*size || full.read > 3*size {
		t.Fatalf("both blobs within the bound read %d bytes, want at least %d", full.read, 2*size)
	}
}

func TestDCXV1425OpenDeltaCurrentUnavailableCheckout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDeltaAddRepoFixture(t)
	encode := func(a Artifact) []byte {
		t.Helper()
		a.SHA256 = ""
		digest, err := hashValue(a)
		if err != nil {
			t.Fatal(err)
		}
		a.SHA256 = digest
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	retained, err := Encode(f.previous)
	if err != nil {
		t.Fatal(err)
	}
	if a, err := OpenDeltaCurrent(ctx, f.root, retained); err != nil || a.deltaUnverified != nil {
		t.Fatalf("available checkout not rederived: %v", err)
	}
	if err := os.Rename(f.lib, f.lib+"-gone"); err != nil {
		t.Fatal(err)
	}
	a, err := OpenDeltaCurrent(ctx, f.root, retained)
	if err != nil || len(a.deltaUnverified) != 1 || !strings.Contains(a.deltaUnverified[0], f.extra.ID) {
		t.Fatalf("unavailable checkout: %v %v", err, a)
	}
	receipt, err := DeltaWith(ctx, f.root, f.previous, a, "artifact", DeltaOptions{Limit: MaxResults, Uncovered: DeltaUncoveredChanged})
	if err != nil {
		t.Fatal(err)
	}
	if e := entriesByID(receipt)["adapter:lib"]; e.Class != DeltaUnresolved || !strings.Contains(e.Reason, "checkout is unavailable") {
		t.Fatalf("anchor in the unavailable checkout: %+v", e)
	}
	if !slices.Contains(receipt.Limitations, a.deltaUnverified[0]) {
		t.Fatalf("receipt does not disclose the unverified current side: %v", receipt.Limitations)
	}
	// An available repository's input that does not name the blob its path holds is refused.
	tampered := *f.previous
	tampered.Manifest.Inputs = slices.Clone(tampered.Manifest.Inputs)
	for i, in := range tampered.Manifest.Inputs {
		if in.Revision == tampered.Manifest.Repository.Revision {
			tampered.Manifest.Inputs[i].Blob = f.extra.ID
			break
		}
	}
	if _, err := OpenDeltaCurrent(ctx, f.root, encode(tampered)); err == nil || !strings.Contains(err.Error(), "does not name the blob its path holds") {
		t.Fatalf("tampered primary input admitted: %v", err)
	}
}

func TestDCXV1424DeltaHistoricalPins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, m, extra, oldRevision := multiRevisionFixture(t)
	a, err := Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := DeltaWith(ctx, root, a, a, "artifact", DeltaOptions{Limit: MaxResults, Uncovered: DeltaUncoveredChanged})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range receipt.Entries {
		if e.Class != DeltaCarryForward {
			t.Fatalf("no-op delta over a historical pin listed %+v", e)
		}
	}
	oldBlob := ""
	for _, in := range m.Inputs {
		if in.Revision == oldRevision {
			oldBlob = in.Blob
		}
	}
	got := entriesByID(receipt)
	for id, want := range map[string]string{"adapter:lib-old": oldBlob, "adapter:lib": ""} {
		e := got[id]
		if e.Class != DeltaCarryForward || len(e.Anchors) != 1 || e.Anchors[0].Repository != extra.ID || e.Anchors[0].Current == nil || want != "" && e.Anchors[0].Current.Blob != want {
			t.Fatalf("%s not carried at its pinned revision: %+v", id, e)
		}
	}
	record, err := CarryForward(ctx, root, a, a, "adapter")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Subjects) != 0 {
		t.Fatalf("records present in the current corpus carried: %+v", record.Subjects)
	}
	if !slices.ContainsFunc(a.Subjects, func(s Subject) bool { return s.ID == "adapter:lib-old" }) {
		t.Fatal("fixture lost its historical-pin subject")
	}
}
