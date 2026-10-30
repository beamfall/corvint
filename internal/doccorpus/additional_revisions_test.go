package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// multiRevisionFixture builds primary repository A plus repository B pinned at two revisions:
// adapter:lib-old is anchored at B's older revision and adapter:lib at its current one, both on
// src/value.go, the path A's adapter:count also documents.
func multiRevisionFixture(t *testing.T) (string, Manifest, AdditionalRepository, string) {
	t.Helper()
	lib := t.TempDir()
	git(t, lib, "init", "-q")
	git(t, lib, "config", "user.name", "Corpus test")
	git(t, lib, "config", "user.email", "corpus@example.invalid")
	commit := func(text string) (string, []byte) {
		doc := []byte("package lib\n\n// " + text + "\nfunc Quokka() {}\n")
		if err := os.MkdirAll(filepath.Join(lib, "src"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lib, "src/value.go"), doc, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, lib, "add", ".")
		git(t, lib, "commit", "-qm", text)
		return git(t, lib, "rev-parse", "HEAD"), doc
	}
	oldRevision, oldDoc := commit("Quokka parses values.")
	newRevision, newDoc := commit("Quokka parses values twice.")
	id := git(t, lib, "rev-list", "--max-parents=0", "HEAD")
	evidence := func(revision string, doc []byte) Evidence {
		excerpt, err := span(doc, 3, 3)
		if err != nil {
			t.Fatal(err)
		}
		blob := git(t, lib, "rev-parse", revision+":src/value.go")
		return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{{Repository: id, Revision: revision, Path: "src/value.go", Blob: blob, SHA256: Digest(doc), Start: 3, End: 3, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}}, Limitations: []string{"provider declaration; structural anchor does not prove behavior"}}
	}
	root, m := providerFixture(t, func(r *ProviderRecord) {
		r.Subjects = append(r.Subjects, Subject{"adapter:lib", "capability", "Lib quokka parser", "adapter", evidence(newRevision, newDoc), nil}, Subject{"adapter:lib-old", "capability", "Lib quokka parser before", "adapter", evidence(oldRevision, oldDoc), nil})
	})
	rel, err := filepath.Rel(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	extra := AdditionalRepository{ID: id, Revision: newRevision, Revisions: []string{oldRevision}, Root: filepath.ToSlash(rel)}
	m.AdditionalRepositories = []AdditionalRepository{extra}
	for _, pin := range []struct {
		revision string
		doc      []byte
	}{{newRevision, newDoc}, {oldRevision, oldDoc}} {
		m.Scopes = append(m.Scopes, Scope{"src/value.go", pin.revision})
		m.Inputs = append(m.Inputs, Input{Path: "src/value.go", Revision: pin.revision, Blob: git(t, lib, "rev-parse", pin.revision+":src/value.go"), SHA256: Digest(pin.doc), Provider: "adapter", Purpose: "evidence"})
	}
	return root, m, extra, oldRevision
}

func TestDCPV1308AdditionalRepositoryAtTwoRevisions(t *testing.T) {
	t.Parallel()
	root, m, extra, oldRevision := multiRevisionFixture(t)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	read := func(r Request) Receipt {
		t.Helper()
		receipt, err := ReadQuery(context.Background(), root, data, r)
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	cites := func(r Receipt, revision string) bool {
		for _, c := range r.Citations {
			if c.Repository == extra.ID && c.Revision == revision && c.Path == "src/value.go" {
				return true
			}
		}
		return false
	}
	if r := read(Request{Operation: "get", ID: "adapter:lib-old"}); !cites(r, oldRevision) || cites(r, extra.Revision) {
		t.Fatalf("older-revision anchor did not report its revision: %+v", r.Citations)
	}
	if r := read(Request{Operation: "get", ID: "adapter:lib"}); !cites(r, extra.Revision) {
		t.Fatalf("current-revision anchor lost: %+v", r.Citations)
	}
	// DCP-V1-309: freshness compares only the declared current revision.
	if state, limits, err := Freshness(context.Background(), root, a); err != nil || state != "fresh" {
		t.Fatalf("two pinned revisions not fresh: %s %v %v", state, limits, err)
	}
	lib := filepath.Join(root, filepath.FromSlash(extra.Root))
	if err := os.WriteFile(filepath.Join(lib, "src/value.go"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if state, limits, err := Freshness(context.Background(), root, a); err != nil || state != "stale" || strings.Count(strings.Join(limits, "\n"), "additional repository "+extra.ID+": working-tree input differs: src/value.go") != 1 {
		t.Fatalf("current revision change not reported once: %s %v %v", state, limits, err)
	}
}

func TestDCPV1308AdditionalRevisionRefusals(t *testing.T) {
	t.Parallel()
	root, m, extra, oldRevision := multiRevisionFixture(t)
	for name, revisions := range map[string][]string{
		"empty list":         {},
		"current revision":   {extra.Revision},
		"primary revision":   {m.Repository.Revision},
		"repeated revision":  {oldRevision, oldRevision},
		"unsorted revisions": {oldRevision, strings.Repeat("0", 40)},
		"not an object id":   {"HEAD"},
	} {
		if name == "unsorted revisions" && oldRevision < strings.Repeat("0", 40) {
			continue
		}
		bad := m
		r := extra
		r.Revisions = revisions
		bad.AdditionalRepositories = []AdditionalRepository{r}
		if _, err := Build(context.Background(), root, bad); !strings.Contains(corpusCode(err), "corpus-refused: invalid additional repository declaration") {
			t.Fatalf("%s: %s", name, corpusCode(err))
		}
	}
	over := extra
	over.Revisions = nil
	for i := range 8 {
		over.Revisions = append(over.Revisions, strings.Repeat(string("0123456789abcdef"[i]), 40))
	}
	slices.Sort(over.Revisions)
	bad := m
	bad.AdditionalRepositories = []AdditionalRepository{over}
	if _, err := Build(context.Background(), root, bad); !strings.Contains(corpusCode(err), "pinned repository revision count") {
		t.Fatalf("revision bound not enforced: %s", corpusCode(err))
	}
}

func TestDCPV1310LocateRepositoryQualifier(t *testing.T) {
	t.Parallel()
	root, m, extra, _ := multiRevisionFixture(t)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	read := func(r Request) Receipt {
		t.Helper()
		receipt, err := ReadQuery(context.Background(), root, data, r)
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	subjects := func(r Receipt) []string {
		out := []string{}
		for _, v := range r.Results {
			if s, ok := v.(Subject); ok {
				out = append(out, s.ID)
			}
		}
		slices.Sort(out)
		return out
	}
	for _, op := range []string{"locate", "recommend-tests"} {
		all := read(Request{Operation: op, Path: "src/value.go"})
		if got := subjects(all); !slices.Contains(got, "adapter:count") || !slices.Contains(got, "adapter:lib") || !slices.Contains(got, "adapter:lib-old") {
			t.Fatalf("%s without repository: %v", op, got)
		}
		// DCP-V1-311: without a qualifier each result names its repository.
		for _, v := range all.Results {
			id := recordID(v)
			want := m.Repository.ID
			if strings.HasPrefix(id, "adapter:lib") {
				want = extra.ID
			}
			if all.ResultRepositories[id] != want {
				t.Fatalf("%s result %s named repository %q, want %q", op, id, all.ResultRepositories[id], want)
			}
		}
		if got := subjects(read(Request{Operation: op, Path: "src/value.go", Repository: extra.ID})); !slices.Equal(got, []string{"adapter:lib", "adapter:lib-old"}) {
			t.Fatalf("%s repository B returned %v", op, got)
		}
		if got := subjects(read(Request{Operation: op, Path: "src/value.go", Repository: m.Repository.ID})); slices.Contains(got, "adapter:lib") || !slices.Contains(got, "adapter:count") {
			t.Fatalf("%s repository A returned %v", op, got)
		}
	}
	for name, r := range map[string]Request{
		"undeclared repository": {Operation: "locate", Path: "src/value.go", Repository: strings.Repeat("e", 40)},
		"other operation":       {Operation: "search", Query: "quokka", Repository: extra.ID},
	} {
		if _, err := ReadQuery(context.Background(), root, data, r); !strings.Contains(corpusCode(err), "repository qualifier") {
			t.Fatalf("%s: %s", name, corpusCode(err))
		}
	}
}

func TestDCPV1311SingleRepositoryLocateUnchanged(t *testing.T) {
	t.Parallel()
	root, m := providerFixture(t, nil)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ReadQuery(context.Background(), root, data, Request{Operation: "locate", Path: "src/value.go"})
	if err != nil || r.ResultRepositories != nil || len(r.Results) == 0 {
		t.Fatalf("single-repository locate changed: %+v %v", r.ResultRepositories, err)
	}
	encoded, err := Encode(r)
	if err != nil || strings.Contains(string(encoded), "result_repositories") {
		t.Fatalf("single-repository receipt carries result_repositories: %v", err)
	}
}
