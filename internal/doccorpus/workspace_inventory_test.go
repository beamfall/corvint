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

// workspaceRepos commits an application repository and a separate end-to-end test repository
// side by side and returns their roots: app holds docs, tests holds page objects, a navigation
// doc and one empty page that the native provider reports as a gap.
func workspaceRepos(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	files := map[string]map[string]string{
		"app":   {"docs/login.md": "# Login\n\nUsers sign in with a password.\n", "src/app.go": "package app\n\nfunc Login() {}\n"},
		"tests": {"pages/login.md": "# Login page\n\nThe login page object opens the form.\n", "pages/login.page.ts": "export class LoginPage {\n  open() {}\n}\n", "pages/empty.md": ""},
	}
	for _, name := range []string{"app", "tests"} {
		root := filepath.Join(parent, name)
		for p, content := range files[name] {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, p), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		git(t, root, "init", "-q")
		git(t, root, "config", "user.name", "Corpus test")
		git(t, root, "config", "user.email", "corpus@example.invalid")
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", name)
	}
	return filepath.Join(parent, "app"), filepath.Join(parent, "tests")
}

func workspaceManifest(t *testing.T, app, tests string) Manifest {
	t.Helper()
	m, err := InventoryWorkspace(context.Background(), app, git(t, app, "rev-parse", "HEAD"), []string{"docs", "src"}, ManifestSchema, "2026-10-28T00:00:00Z", []WorkspaceRepository{{Alias: "e2e", Root: tests, Scopes: []string{"pages"}}})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func subjectIDs(a *Artifact) []string {
	ids := []string{}
	for _, s := range a.Subjects {
		ids = append(ids, s.ID)
	}
	return ids
}

func receiptIDs(r Receipt) []string {
	ids := []string{}
	for _, v := range r.Results {
		ids = append(ids, recordID(v))
	}
	return ids
}

// TestDCPV1600WorkspaceManifestPinsEachRepository: the test repository is an aliased additional
// repository pinned at its own HEAD, and its docs and page objects become native subjects keyed by
// alias plus path whose anchors name that repository and revision (DCP-V1-600, DCP-V1-601).
func TestDCPV1600WorkspaceManifestPinsEachRepository(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	testsRevision := git(t, tests, "rev-parse", "HEAD")
	if len(m.AdditionalRepositories) != 1 {
		t.Fatalf("additional repositories %+v", m.AdditionalRepositories)
	}
	extra := m.AdditionalRepositories[0]
	if extra.Alias != "e2e" || extra.Revision != testsRevision || extra.Root != "../tests" || extra.ID != testsRevision {
		t.Fatalf("additional repository %+v, want alias e2e at %s", extra, testsRevision)
	}
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	ids := subjectIDs(a)
	for _, want := range []string{"native:file:docs/login.md", "native:file:src/app.go", "native:file:e2e:pages/login.md", "native:file:e2e:pages/login.page.ts", "native:symbol:e2e:pages/login.page.ts:1:LoginPage"} {
		if !slices.Contains(ids, want) {
			t.Fatalf("subject %s missing from %v", want, ids)
		}
	}
	for _, s := range a.Subjects {
		if !strings.Contains(s.ID, ":e2e:") {
			continue
		}
		for _, anchor := range s.Evidence.Anchors {
			if anchor.Repository != extra.ID || anchor.Revision != testsRevision {
				t.Fatalf("%s anchor %+v is not pinned in the test repository", s.ID, anchor)
			}
		}
	}
	claims := map[string]bool{}
	for _, c := range a.Claims {
		claims[c.ID] = true
	}
	if !claims["native:file:e2e:pages/login.md:paragraph:3"] {
		t.Fatalf("test repository doc paragraph claim missing")
	}
}

// TestDCPV1600WorkspaceAliasRefusals: a repeated or invalid alias, or a native input in an
// additional repository without an alias, is refused (DCP-V1-600).
func TestDCPV1600WorkspaceAliasRefusals(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	for name, edit := range map[string]func(*Manifest){
		"invalid alias": func(m *Manifest) { m.AdditionalRepositories[0].Alias = "E2E" },
		"no alias":      func(m *Manifest) { m.AdditionalRepositories[0].Alias = "" },
	} {
		bad := m
		bad.AdditionalRepositories = slices.Clone(m.AdditionalRepositories)
		edit(&bad)
		if _, err := Build(context.Background(), app, bad); corpusCode(err) == "" || !strings.HasPrefix(corpusCode(err), "corpus-refused") {
			t.Fatalf("%s: got %v, want corpus-refused", name, err)
		}
	}
	_, err := InventoryWorkspace(context.Background(), app, git(t, app, "rev-parse", "HEAD"), []string{"docs"}, ManifestSchema, "2026-10-28T00:00:00Z", []WorkspaceRepository{{Alias: "e2e", Root: tests, Scopes: []string{"pages"}}, {Alias: "e2e", Root: tests, Scopes: []string{"pages"}}})
	if err == nil {
		t.Fatal("a repeated workspace repository was admitted")
	}
}

// TestDCPV1602TraceAndGapsByPathInEitherRepository: trace and gaps answer for a file path in the
// primary repository and, qualified by alias or root commit, in the test repository; the receipt
// records the resolution (DCP-V1-602).
func TestDCPV1602TraceAndGapsByPathInEitherRepository(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	extra := m.AdditionalRepositories[0]
	for _, c := range []struct{ repository, path, id string }{
		{"", "docs/login.md", "native:file:docs/login.md"},
		{"e2e", "pages/login.page.ts", "native:file:e2e:pages/login.page.ts"},
		{extra.ID, "pages/login.md", "native:file:e2e:pages/login.md"},
	} {
		r, err := Query(a, Request{Operation: "trace", Path: c.path, Repository: c.repository}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		if r.State != "ready" || r.PathResolution == nil || r.PathResolution.ID != c.id || !slices.Contains(receiptIDs(r), c.id) {
			t.Fatalf("trace %s in %q: state %s resolution %+v results %v", c.path, c.repository, r.State, r.PathResolution, receiptIDs(r))
		}
	}
	r, err := Query(a, Request{Operation: "gaps", Path: "pages/empty.md", Repository: "e2e"}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Results) != 1 || r.Results[0].(Gap).Subject != "native:file:e2e:pages/empty.md" || r.Results[0].(Gap).Kind != "not-collected" {
		t.Fatalf("gaps for the empty test page: %+v", r.Results)
	}
	if r, err := Query(a, Request{Operation: "gaps", Path: "pages/empty.md"}, "fresh", nil); err != nil || len(r.Results) != 0 {
		t.Fatalf("an unqualified path names the primary repository: %+v %v", r.Results, err)
	}
	if r, err := Query(a, Request{Operation: "locate", Path: "pages/login.md", Repository: "e2e"}, "fresh", nil); err != nil || !slices.Equal(receiptIDs(r), []string{"native:file:e2e:pages/login.md"}) {
		t.Fatalf("locate by alias: %v %v", receiptIDs(r), err)
	}
	for name, request := range map[string]Request{
		"id and path":      {Operation: "trace", ID: "native:file:docs/login.md", Path: "docs/login.md"},
		"undeclared alias": {Operation: "trace", Path: "docs/login.md", Repository: "web"},
		"qualifier on get": {Operation: "get", ID: "native:file:docs/login.md", Repository: "e2e"},
	} {
		if _, err := Query(a, request, "fresh", nil); !strings.HasPrefix(corpusCode(err), "corpus-refused") {
			t.Fatalf("%s: got %v, want corpus-refused", name, err)
		}
	}
}

// TestDCPV1603SingleRootManifestByteIdentical: without further repositories the workspace
// inventory is InventoryScopes' manifest byte for byte, and its artifact keeps its bytes, its
// unqualified native IDs and no alias member (DCP-V1-603).
func TestDCPV1603SingleRootManifestByteIdentical(t *testing.T) {
	t.Parallel()
	app, _ := workspaceRepos(t)
	revision := git(t, app, "rev-parse", "HEAD")
	plain, err := InventoryScopes(context.Background(), app, revision, []string{"docs", "src"}, ManifestSchema, "2026-10-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	bound, err := InventoryWorkspace(context.Background(), app, revision, []string{"docs", "src"}, ManifestSchema, "2026-10-28T00:00:00Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	plainBytes, _ := Encode(plain)
	boundBytes, _ := Encode(bound)
	if !bytes.Equal(plainBytes, boundBytes) {
		t.Fatalf("single-root manifest changed:\n%s\n%s", plainBytes, boundBytes)
	}
	a, err := Build(context.Background(), app, plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(context.Background(), app, bound)
	if err != nil {
		t.Fatal(err)
	}
	aBytes, _ := Encode(a)
	bBytes, _ := Encode(b)
	if !bytes.Equal(aBytes, bBytes) || bytes.Contains(aBytes, []byte(`"alias"`)) || bytes.Contains(aBytes, []byte(`"additional_repositories"`)) {
		t.Fatal("single-root artifact bytes changed or gained a repository member")
	}
	if !slices.Contains(subjectIDs(a), "native:file:docs/login.md") {
		t.Fatalf("primary native IDs are qualified: %v", subjectIDs(a))
	}
}

// TestDCPV1605CrossRepositoryRelationTracesBothWays: a records-provider relation from an
// application doc to a test-repository page, anchored in the test repository, is admitted and
// trace returns it from either end, by ID or by path (DCP-V1-605).
func TestDCPV1605CrossRepositoryRelationTracesBothWays(t *testing.T) {
	t.Parallel()
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	extra := m.AdditionalRepositories[0]
	page, err := os.ReadFile(filepath.Join(tests, "pages/login.md"))
	if err != nil {
		t.Fatal(err)
	}
	excerpt, err := span(page, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	ev := Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{{Repository: extra.ID, Revision: extra.Revision, Path: "pages/login.md", Blob: git(t, tests, "rev-parse", extra.Revision+":pages/login.md"), SHA256: Digest(page), Start: 3, End: 3, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture coverage declaration"}}, Limitations: []string{"provider declaration; coverage is not verified behavior"}}
	record := ProviderRecord{Schema: ProviderSchema, ID: "coverage", Version: "1", Source: m.Repository, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{{"coverage:login-tested-by-page", "native:file:docs/login.md", "native:file:e2e:pages/login.md", "tested_by", "coverage", ev}}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{{"relations", "present", "explicit cross-repository coverage relation"}}}
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "evidence/coverage.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, app, "add", "evidence/coverage.json")
	git(t, app, "commit", "-qm", "coverage")
	revision := git(t, app, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "coverage", Kind: "records", Version: "1", Revision: revision, Record: "evidence/coverage.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/coverage.json", revision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/coverage.json", Revision: revision, Blob: git(t, app, "rev-parse", revision+":evidence/coverage.json"), SHA256: Digest(data), Provider: "coverage", Purpose: "provider"})
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []Request{
		{Operation: "trace", ID: "native:file:docs/login.md"},
		{Operation: "trace", ID: "native:file:e2e:pages/login.md"},
		{Operation: "trace", Path: "pages/login.md", Repository: "e2e"},
		{Operation: "related", ID: "native:file:e2e:pages/login.md"},
	} {
		r, err := Query(a, request, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(receiptIDs(r), "coverage:login-tested-by-page") {
			t.Fatalf("%+v does not reach the cross-repository relation: %v", request, receiptIDs(r))
		}
	}
}
