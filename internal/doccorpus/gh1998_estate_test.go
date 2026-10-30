package doccorpus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// estateCorpus builds a single-repository inventory corpus of root and writes it to
// root/corpus.json, returning its estate entry relative to parent.
func estateCorpus(t *testing.T, parent, root string, scopes []string, aliases []string) EstateEntry {
	t.Helper()
	revision := git(t, root, "rev-parse", "HEAD")
	m, err := ManifestInventory(context.Background(), root, revision, scopes, ManifestSchema, "2026-10-29T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	m.RepositoryInventory = &RepositoryInventory{}
	if aliases != nil {
		m.RepositoryInventory.Repositories = []RepositoryDeclaration{{Repository: m.Repository.ID, Aliases: aliases}}
	}
	encoded, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := ParseManifest(encoded); err != nil || parsed.RepositoryInventory == nil {
		t.Fatalf("an empty repository_inventory declaration should survive encoding: %v", err)
	}
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(parent, root)
	if err != nil {
		t.Fatal(err)
	}
	return EstateEntry{Root: filepath.ToSlash(rel), Artifact: "corpus.json"}
}

func findEstateReference(t *testing.T, r EstateReceipt, ecosystem, name string) EstateReference {
	t.Helper()
	for _, ref := range r.References {
		if ref.Ecosystem == ecosystem && ref.Name == name {
			return ref
		}
	}
	t.Fatalf("estate holds no %s reference %s: %+v", ecosystem, name, r.References)
	return EstateReference{}
}

// TestDCIV1001DCIV1002DCIV1003DCIV1005EstateReconcilesAcrossCorpora: a service and the library it consumes
// built as separate corpora each leave the other's names outside the corpus; the estate receipt
// over both lists both repositories once and joins the Go module and the reusable workflow by
// exact name, keeps an undeclared name outside the estate, loses no outside reference, and marks
// a name two repositories declare ambiguous instead of choosing (GitHub #1998).
func TestDCIV1001DCIV1002DCIV1003DCIV1005EstateReconcilesAcrossCorpora(t *testing.T) {
	t.Parallel()
	app, lib := inventoryRepos(t)
	parent := filepath.Dir(app)
	fork := filepath.Join(parent, "fork")
	if err := os.MkdirAll(fork, 0o700); err != nil {
		t.Fatal(err)
	}
	git(t, fork, "init", "-q")
	git(t, fork, "config", "user.name", "Corpus test")
	git(t, fork, "config", "user.email", "corpus@example.invalid")
	commitFiles(t, fork, map[string]string{"go.mod": "module example.invalid/lib\n\ngo 1.27.1\n"})
	entries := []EstateEntry{
		estateCorpus(t, parent, lib, []string{"go.mod", "lib.go", "package.json"}, []string{"example/shared-workflows"}),
		estateCorpus(t, parent, app, []string{".github", "deploy", "docs", "go.mod"}, nil),
	}
	outside := 0
	for _, e := range entries {
		a, err := OpenReadFile(context.Background(), filepath.Join(parent, e.Root), e.Artifact, ReadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		outside += len(inventoryGaps(t, a, "outside-corpus-reference"))
	}
	r, err := Estate(context.Background(), parent, EstateInput{Schema: EstateSchema, Corpora: entries})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Corpora) != 2 || len(r.Repositories) != 2 || len(r.References) != outside || len(r.Unknown) != 0 {
		t.Fatalf("estate should hold both corpora, both repositories and every outside reference (%d): %+v", outside, r)
	}
	module := findEstateReference(t, r, "go", "example.invalid/lib")
	if module.State != "resolved" || len(module.Targets) != 1 || module.Targets[0].Corpus != "lib/corpus.json" || module.Targets[0].Subject != "native:package:go.mod" {
		t.Fatalf("the library module should join the library corpus: %+v", module)
	}
	workflow := findEstateReference(t, r, "action", "example/shared-workflows")
	if workflow.State != "resolved" || workflow.Targets[0].Subject != repositorySubjectID("native", workflow.Targets[0].Repository) {
		t.Fatalf("the reusable workflow should join the library repository by its declared alias: %+v", workflow)
	}
	if ref := findEstateReference(t, r, "go", "example.invalid/outside"); ref.State != "outside-estate" || !strings.Contains(ref.Reason, "NOT_OBSERVED") {
		t.Fatalf("an undeclared module should stay outside the estate: %+v", ref)
	}
	if ref := findEstateReference(t, r, "npm", "left-pad"); ref.State != "outside-estate" || ref.Corpus != "lib/corpus.json" {
		t.Fatalf("the library's own outside reference should be kept: %+v", ref)
	}
	estateCorpus(t, parent, fork, []string{"go.mod"}, nil)
	in, err := ParseEstate([]byte(`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"lib","artifact":"corpus.json"},{"root":"fork","artifact":"corpus.json"},{"root":"app","artifact":"corpus.json"}]}`))
	if err != nil || len(in.Corpora) != 3 || in.Corpora[0].Root != "app" {
		t.Fatalf("the estate file should parse sorted: %+v %v", in, err)
	}
	if _, err := ParseEstate([]byte(`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"../../x","artifact":"a.json"},{"root":".","artifact":"a.json"}]}`)); err != nil {
		t.Fatalf("the estate file should parse sorted: %+v %v", in, err)
	}
	r, err = Estate(context.Background(), parent, in)
	if err != nil {
		t.Fatal(err)
	}
	if module := findEstateReference(t, r, "go", "example.invalid/lib"); module.State != "ambiguous" || len(module.Targets) != 2 {
		t.Fatalf("a name two repositories declare should be ambiguous: %+v", module)
	}
	for _, bad := range []string{
		`{"schema":"corvint-corpus-estate/1","corpora":[]}`,
		`{"schema":"corvint-corpus-estate/0","corpora":[{"root":".","artifact":"a.json"}]}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"/x","artifact":"a.json"}]}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"a/../b","artifact":"a.json"}]}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"a","artifact":"../a.json"}]}`,
		`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"a","artifact":"b.json"},{"root":"a","artifact":"b.json"}]}`,
	} {
		if _, err := ParseEstate([]byte(bad)); err == nil {
			t.Fatalf("estate %s should be refused", bad)
		}
	}
	_, err = Estate(context.Background(), parent, EstateInput{Schema: EstateSchema, Corpora: []EstateEntry{{Root: "app", Artifact: "missing.json"}}})
	var e *Error
	if !errors.As(err, &e) || !strings.HasPrefix(e.Message, "estate corpus app/missing.json: ") {
		t.Fatalf("an unreadable corpus should be refused naming it: %v", err)
	}
}

// TestDCIV1004ShallowRepositoryRefusalNamesIt: a shallow further checkout is refused naming its
// workspace alias at manifest time and its repository at build time, not with the anonymous
// history refusal (GitHub #1998).
func TestDCIV1004ShallowRepositoryRefusalNamesIt(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	head := git(t, tests, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(tests, ".git", "shallow"), []byte(head+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := func(err error, prefix string) {
		t.Helper()
		var e *contextindex.Error
		if !errors.As(err, &e) || e.Code != "corpus-invalid-revision" || e.Message != prefix+": corpus identity requires complete ungrafted history" {
			t.Fatalf("a shallow checkout should be refused naming %q: %v", prefix, err)
		}
	}
	_, err := InventoryWorkspace(context.Background(), app, git(t, app, "rev-parse", "HEAD"), []string{"docs"}, ManifestSchema, "2026-10-29T00:00:00Z", []WorkspaceRepository{{Alias: "e2e", Root: tests, Scopes: []string{"pages"}}})
	want(err, "repository e2e")
	_, err = Build(context.Background(), app, m)
	want(err, "additional repository "+m.AdditionalRepositories[0].ID)
}
