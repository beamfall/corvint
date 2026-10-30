package doccorpus

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// additionalRepositoryFixture builds primary repository A with a records provider whose claim
// adapter:lib is anchored in a second, separately committed repository B at its own revision.
func additionalRepositoryFixture(t *testing.T, anchor func(*ProviderRecord, *Anchor)) (string, Manifest, AdditionalRepository) {
	t.Helper()
	lib := t.TempDir()
	git(t, lib, "init", "-q")
	git(t, lib, "config", "user.name", "Corpus test")
	git(t, lib, "config", "user.email", "corpus@example.invalid")
	doc := []byte("# Lib\n\nThe lib parses quokka values.\n")
	if err := os.WriteFile(filepath.Join(lib, "doc.md"), doc, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, lib, "add", ".")
	git(t, lib, "commit", "-qm", "lib")
	libRevision := git(t, lib, "rev-parse", "HEAD")
	blob := git(t, lib, "rev-parse", libRevision+":doc.md")
	excerpt, err := span(doc, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	ev := Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{{Repository: libRevision, Revision: libRevision, Path: "doc.md", Blob: blob, SHA256: Digest(doc), Start: 3, End: 3, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}}, Limitations: []string{"provider declaration; structural anchor does not prove behavior"}}
	root, m := providerFixture(t, func(r *ProviderRecord) {
		if anchor != nil {
			anchor(r, &ev.Anchors[0])
		}
		r.Subjects = append(r.Subjects, Subject{"adapter:lib", "capability", "Lib quokka parser", "adapter", ev, nil})
		r.Claims = append(r.Claims, Claim{"adapter:lib-claim", "adapter:lib", "The lib parses quokka values", "adapter", ev})
	})
	rel, err := filepath.Rel(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	extra := AdditionalRepository{ID: libRevision, Revision: libRevision, Root: filepath.ToSlash(rel)}
	m.AdditionalRepositories = []AdditionalRepository{extra}
	m.Scopes = append(m.Scopes, Scope{"doc.md", libRevision})
	m.Inputs = append(m.Inputs, Input{Path: "doc.md", Revision: libRevision, Blob: blob, SHA256: Digest(doc), Provider: "adapter", Purpose: "evidence"})
	return root, m, extra
}

func corpusCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code + ": " + e.Message
	}
	if err != nil {
		return "untyped: " + err.Error()
	}
	return ""
}

func TestDCPV1175AdditionalRepositoryAnchors(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-175 DCP-V1-176 DCP-V1-177 DCP-V1-178 two repositories", func(t *testing.T) {
		root, m, extra := additionalRepositoryFixture(t, nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		var found *Claim
		for i := range a.Claims {
			if a.Claims[i].ID == "adapter:lib-claim" {
				found = &a.Claims[i]
			}
		}
		if found == nil || found.Evidence.Anchors[0].Repository != extra.ID || found.Evidence.Anchors[0].Revision != extra.Revision {
			t.Fatalf("additional repository anchor lost: %+v", found)
		}
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		cited := func(op string, r Request) {
			t.Helper()
			r.Operation = op
			receipt, err := ReadQuery(context.Background(), root, data, r)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range receipt.Citations {
				if c.Repository == extra.ID && c.Revision == extra.Revision && c.Path == "doc.md" {
					return
				}
			}
			t.Fatalf("%s did not cite repository B: %+v", op, receipt.Citations)
		}
		cited("search", Request{Query: "quokka"})
		cited("get", Request{ID: "adapter:lib"})
		state, _, err := Freshness(context.Background(), root, a)
		if err != nil || state != "fresh" {
			t.Fatalf("pinned two-repository corpus not fresh: %s %v", state, err)
		}
		// A change in A outside every declared scope leaves the corpus fresh.
		if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("x\n"), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "unrelated.txt")
		git(t, root, "commit", "-qm", "unrelated")
		if state, _, err := Freshness(context.Background(), root, a); err != nil || state != "fresh" {
			t.Fatalf("unrelated change in A staled B evidence: %s %v", state, err)
		}
		lib := filepath.Join(root, filepath.FromSlash(extra.Root))
		if err := os.WriteFile(filepath.Join(lib, "doc.md"), []byte("# Lib\n\nchanged\n"), 0600); err != nil {
			t.Fatal(err)
		}
		state, limits, err := Freshness(context.Background(), root, a)
		if err != nil || state != "stale" || !strings.Contains(strings.Join(limits, "\n"), "additional repository "+extra.ID+": working-tree input differs: doc.md") {
			t.Fatalf("change in B not reported stale: %s %v %v", state, limits, err)
		}
		// DCP-V1-280: the overlay's reuse key covers every declared checkout, so it is never the
		// primary observation alone and equals what a reader observes before the next call.
		overlay, err := LiveFreshness(context.Background(), root, a)
		observed, observeErr := CorpusObservation(context.Background(), root, a)
		primary, primaryErr := contextindex.Observe(context.Background(), root)
		if err != nil || observeErr != nil || primaryErr != nil || overlay.Observation() == "" || overlay.Observation() != observed || observed == ObservationKey(primary) {
			t.Fatalf("multi-repository overlay key %q, observed %q: %v %v %v", overlay.Observation(), observed, err, observeErr, primaryErr)
		}
	})
}

func TestDCPV1176AdditionalRepositoryRefusals(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-175 DCP-V1-176 refusals", func(t *testing.T) {
		root, m, extra := additionalRepositoryFixture(t, nil)
		build := func(edit func(*Manifest)) string {
			t.Helper()
			c := m
			c.AdditionalRepositories = append([]AdditionalRepository(nil), m.AdditionalRepositories...)
			edit(&c)
			_, err := Build(context.Background(), root, c)
			if err == nil {
				t.Fatal("refusal expected")
			}
			return corpusCode(err)
		}
		// Undeclared repository: without the declaration B's revision is resolved only in A's
		// object store, which does not hold it, so the build refuses (build() fails on success).
		build(func(c *Manifest) { c.AdditionalRepositories = nil })
		cases := map[string]func(*Manifest){
			"primary identity": func(c *Manifest) { c.AdditionalRepositories[0].ID = c.Repository.ID },
		}
		for _, bad := range []string{"/abs", "a/../b", ".", "", "a\\b", "x/"} {
			cases["root "+bad] = func(c *Manifest) { c.AdditionalRepositories[0].Root = bad }
		}
		cases["duplicate"] = func(c *Manifest) {
			c.AdditionalRepositories = append(c.AdditionalRepositories, c.AdditionalRepositories[0])
		}
		cases["primary revision"] = func(c *Manifest) {
			c.AdditionalRepositories[0].Revision = c.Repository.Revision
		}
		for name, edit := range cases {
			if got := build(edit); got != "corpus-refused: invalid additional repository declaration" {
				t.Fatalf("%s: %q", name, got)
			}
		}
		if got := build(func(c *Manifest) { c.AdditionalRepositories[0].Root = "../" + filepath.Base(root) }); got != "corpus-refused: invalid additional repository declaration" {
			t.Fatalf("checkout resolving to the primary root: %q", got)
		}
		if got := build(func(c *Manifest) { c.AdditionalRepositories[0].Root = "../missing-checkout" }); got != "corpus-input-unavailable: additional repository checkout unavailable: "+extra.ID+": repository-object-unavailable: root path does not resolve" {
			t.Fatalf("missing checkout: %q", got)
		}
		if got := build(func(c *Manifest) { c.AdditionalRepositories[0].ID = strings.Repeat("f", len(extra.ID)) }); got != "corpus-refused: repository identity mismatch" {
			t.Fatalf("identity mismatch: %q", got)
		}
	})
	t.Run("DCP-V1-177 anchor naming an undeclared repository", func(t *testing.T) {
		for name, wrong := range map[string]func(*ProviderRecord, *Anchor){
			"primary":    func(r *ProviderRecord, a *Anchor) { a.Repository = r.Source.ID },
			"undeclared": func(_ *ProviderRecord, a *Anchor) { a.Repository = strings.Repeat("e", len(a.Repository)) },
		} {
			root, m, _ := additionalRepositoryFixture(t, wrong)
			if _, err := Build(context.Background(), root, m); corpusCode(err) != "corpus-refused: invalid anchor identity or reason" {
				t.Fatalf("%s: anchor at B's revision naming another repository not refused: %v", name, err)
			}
		}
	})
}

func TestDCPV1179SingleRepositoryManifestUnchanged(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-179 identity", func(t *testing.T) {
		root, m := fixture(t)
		data, err := Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("additional_repositories")) {
			t.Fatal("single-repository manifest gained an additional_repositories field")
		}
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		overlay, err := LiveFreshness(context.Background(), root, a)
		if err != nil || overlay.Observation() == "" {
			t.Fatalf("single-repository overlay lost its reusable observation: %v", err)
		}
	})
}
