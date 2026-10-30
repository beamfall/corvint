package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// multiRepositoryOwnedSource builds primary repository A with a native provider
// and a records provider "adapter", plus a second repository B whose Ruby
// source src/pos.rb only the adapter claims (GitHub #1665 row 5, V1-2567).
func multiRepositoryOwnedSource(t *testing.T) (string, Manifest, AdditionalRepository) {
	t.Helper()
	lib := t.TempDir()
	git(t, lib, "init", "-q")
	git(t, lib, "config", "user.name", "Corpus test")
	git(t, lib, "config", "user.email", "corpus@example.invalid")
	ruby := []byte("class PosSync\n  def sync_with_pos\n    true\n  end\nend\n")
	if err := os.MkdirAll(filepath.Join(lib, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "src/pos.rb"), ruby, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, lib, "add", ".")
	git(t, lib, "commit", "-qm", "lib")
	libRevision := git(t, lib, "rev-parse", "HEAD")
	blob := git(t, lib, "rev-parse", libRevision+":src/pos.rb")
	excerpt, err := span(ruby, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	ev := Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{{Repository: libRevision, Revision: libRevision, Path: "src/pos.rb", Blob: blob, SHA256: Digest(ruby), Start: 1, End: 1, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}}, Limitations: []string{"provider declaration; structural anchor does not prove behavior"}}
	root, m := providerFixture(t, func(r *ProviderRecord) {
		r.Subjects = append(r.Subjects, Subject{"adapter:pos", "capability", "Point of sale sync", "adapter", ev, nil})
	})
	rel, err := filepath.Rel(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	extra := AdditionalRepository{ID: libRevision, Revision: libRevision, Root: filepath.ToSlash(rel)}
	m.AdditionalRepositories = []AdditionalRepository{extra}
	m.Scopes = append(m.Scopes, Scope{"src/pos.rb", libRevision})
	m.Inputs = append(m.Inputs, Input{Path: "src/pos.rb", Revision: libRevision, Blob: blob, SHA256: Digest(ruby), Provider: "adapter", Purpose: "source"})
	return root, m, extra
}

// DCS-V1-549: a source input of an additional repository's declared revision
// that only a records provider claims gets derived declarations, whose IDs
// name the repository, so docs_search finds the identifier in a
// multi-repository corpus; the declaration cites repository B.
func TestDCSV1549DerivedDeclarationsInAdditionalRepository(t *testing.T) {
	t.Parallel()
	root, m, extra := multiRepositoryOwnedSource(t)
	a := buildOwned(t, root, m)
	want := "native:symbol:" + extra.ID + ":src/pos.rb:2:sync_with_pos"
	var found *Subject
	for i := range a.Subjects {
		if a.Subjects[i].ID == want {
			found = &a.Subjects[i]
		}
	}
	if found == nil {
		t.Fatalf("no derived declaration %s: %v", want, subjectsWithPrefix(a, "native:symbol:"+extra.ID))
	}
	if an := found.Evidence.Anchors[0]; an.Repository != extra.ID || an.Revision != extra.Revision || an.Path != "src/pos.rb" || an.Start != 2 {
		t.Fatalf("derived declaration anchor should name repository B: %+v", an)
	}
	for _, kinds := range [][]string{nil, {"symbol"}} {
		if order := searchOrders(t, a, Request{Operation: "search", Query: "sync_with_pos", Kinds: kinds}); len(order) == 0 || order[0] != want {
			t.Fatalf("kinds %v: the derived declaration should rank first: %v", kinds, order)
		}
	}
	claimed := false
	for _, c := range a.Claims {
		claimed = claimed || c.ID == want+":declaration" && strings.Contains(c.Text, "def sync_with_pos")
	}
	if !claimed {
		t.Fatalf("no declaration claim for %s", want)
	}
	// The primary repository's own declarations keep their unqualified IDs.
	if len(subjectsWithPrefix(a, "native:symbol:src/value.go:")) == 0 {
		t.Fatalf("primary declarations lost their IDs: %v", subjectsWithPrefix(a, "native:symbol:"))
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), root, data); err != nil {
		t.Fatalf("multi-repository artifact should rederive: %v", err)
	}
	// Derivation 3: the verified read refuses a derivation 2 artifact, which
	// lacks these declarations, naming the rebuild.
	if n, err := strconv.Atoi(DerivationVersion); err != nil || n < 3 {
		t.Fatalf("additional-repository declarations need derivation 3 or later, got %s", DerivationVersion)
	}
	old := rehashed(t, a, func(c *Artifact) {
		c.Builder.Revision = strings.Repeat("a", 40)
		c.Builder.Engine = strings.Replace(c.Builder.Engine, "/derivation-"+DerivationVersion+"/", "/derivation-2/", 1)
	})
	if _, err := OpenRead(context.Background(), root, old, ReadOptions{}); err == nil || !strings.Contains(err.Error(), "derivation version differs (artifact 2, running builder "+DerivationVersion+"); rebuild the corpus") {
		t.Fatalf("a derivation 2 artifact should be refused naming the rebuild: %v", err)
	}
}
