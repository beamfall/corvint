package appflows

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFU-V1-069 AFU-V1-067: the built-in global names apply inside each declared repository, global_paths
// may name a declared repository's qualified prefix, and the provider and coverage refuse undeclared IDs.
func TestAFUV1069QualifiedGlobalPaths(t *testing.T) {
	t.Parallel()
	origin := strings.Repeat("a", 40)
	provider := SelectionProvider{Repositories: []SelectionRepository{{ID: "app", Origin: origin}}, GlobalPaths: []string{"fixtures", "app:config"}}
	s := testSelector(t, t.TempDir(), nil, provider, nil)
	for p, want := range map[string]bool{
		"app:Gemfile": true, "app:src/Dockerfile.dev": true, "app:config": true, "app:config/db.yml": true, "Gemfile": true,
		"app:src/billing/invoice.rb": false, "app:configure.rb": false, "other:Gemfile": false, "fixtures/users.json": true,
	} {
		if got := s.global(p); got != want {
			t.Errorf("global(%q) = %v, want %v", p, got, want)
		}
	}
	base := SelectionProvider{Schema: SelectionProviderSchema, Flows: "flows", RunnerConfig: "playwright.config.ts", GlobalPaths: []string{"app:config"},
		Repositories: []SelectionRepository{{ID: "app", Origin: origin}}}
	if err := validSelectionProvider(base); err != nil {
		t.Fatalf("declared qualified global path refused: %v", err)
	}
	for name, edit := range map[string]func(*SelectionProvider){
		"undeclared global":  func(p *SelectionProvider) { p.GlobalPaths = []string{"other:config"} },
		"unsafe qualified":   func(p *SelectionProvider) { p.GlobalPaths = []string{"app:../config"} },
		"duplicate id":       func(p *SelectionProvider) { p.Repositories = append(p.Repositories, p.Repositories[0]) },
		"abbreviated origin": func(p *SelectionProvider) { p.Repositories[0].Origin = "abc123" },
		"colon in id":        func(p *SelectionProvider) { p.Repositories[0].ID = "a:b" },
	} {
		p := base
		p.Repositories = append([]SelectionRepository{}, base.Repositories...)
		edit(&p)
		if validSelectionProvider(p) == nil {
			t.Errorf("%s: provider accepted", name)
		}
	}
	base.Inventory = []InventoryTest{{TestKey: "t", Project: "chromium", Path: "e2e/t.spec.ts"}}
	for name, repos := range map[string]map[string]string{"undeclared": {"other": origin}, "abbreviated": {"app": "abc"}} {
		coverage := CoverageFile{Schema: CoverageSchema, Records: []CoverageRecord{{TestKey: "t", Commit: origin, Repositories: repos}}}
		if validCoverage(base, coverage) == nil {
			t.Errorf("%s coverage repository accepted", name)
		}
	}
}

// AFU-V1-071: a qualified target is a source target only, reviewed_at_repository rides exactly with
// reviewed_at on such a link, and the link is reviewed only against a bound repository.
func TestAFUV1071QualifiedLinkTargets(t *testing.T) {
	t.Parallel()
	members := map[string]string{"pay": "step"}
	oid := strings.Repeat("c", 40)
	source := LinkTarget{Type: "source", Path: "app:src/pay.rb"}
	for name, c := range map[string]struct {
		link FlowLink
		ok   bool
	}{
		"unreviewed qualified":      {FlowLink{From: "pay", Basis: "declared", Target: source}, true},
		"reviewed qualified":        {FlowLink{From: "pay", Basis: "declared", Target: source, ReviewedAt: oid, ReviewedAtRepository: oid}, true},
		"reviewed without app":      {FlowLink{From: "pay", Basis: "declared", Target: source, ReviewedAt: oid}, false},
		"app commit without review": {FlowLink{From: "pay", Basis: "declared", Target: source, ReviewedAtRepository: oid}, false},
		"app commit on root target": {FlowLink{From: "pay", Basis: "declared", Target: LinkTarget{Type: "source", Path: "src/pay.rb"}, ReviewedAt: oid, ReviewedAtRepository: oid}, false},
		"abbreviated app commit":    {FlowLink{From: "pay", Basis: "declared", Target: source, ReviewedAt: oid, ReviewedAtRepository: "abc"}, false},
		"qualified test target":     {FlowLink{From: "pay", Basis: "declared", Target: LinkTarget{Type: "test", TestKey: "t", Path: "app:e2e/t.spec.ts"}}, false},
		"unsafe qualified path":     {FlowLink{From: "pay", Basis: "declared", Target: LinkTarget{Type: "source", Path: "app:../pay.rb"}}, false},
	} {
		if err := validateLink(c.link, members, false); (err == nil) != c.ok {
			t.Errorf("%s: validateLink = %v, want ok %v", name, err, c.ok)
		}
	}

	app := t.TempDir()
	writeRaw(t, app, "src/pay.rb", []byte("def pay; end\nUNRELATED = 1\n"))
	gitTest(t, app, "init", "-q")
	a0 := commitAll(t, app, "app")
	writeRaw(t, app, "src/pay.rb", []byte("def pay; end\nUNRELATED = 2\n"))
	a1 := commitAll(t, app, "unrelated line")
	writeRaw(t, app, "src/pay.rb", []byte("def pay(n); end\nUNRELATED = 2\n"))
	a2 := commitAll(t, app, "reviewed line")

	root := t.TempDir()
	intent := sampleIntent("checkout")
	intent.Links[1].Target = LinkTarget{Type: "source", Path: "app:src/pay.rb", StartLine: 1, EndLine: 1}
	writeIntent(t, root, "flows/checkout.json", intent)
	writeRaw(t, root, "checkout.spec.ts", []byte("test('pays', () => {})\n"))
	gitTest(t, root, "init", "-q")
	anchor := commitAll(t, root, "fixture")
	state := func(reviewedAt string, repos map[string]RepositoryPin) EvaluatedLink {
		t.Helper()
		set := reviewSet(t, root, anchor)
		set.Flows[0].Links[1].ReviewedAtRepository = reviewedAt
		links, err := EvaluateLinksAcross(context.Background(), root, set, "HEAD", repos)
		if err != nil {
			t.Fatal(err)
		}
		return links[1]
	}
	if got := state(a0, nil); got.ReviewState != ReviewTargetUnpinned || got.Blob != "" {
		t.Fatalf("unbound repository: %+v", got)
	}
	for name, c := range map[string]struct {
		reviewed, revision, want string
	}{
		"same commit":              {a0, a0, ReviewReviewed},
		"other line changed":       {a0, a1, ReviewReviewed},
		"reviewed line changed":    {a0, a2, ReviewStale},
		"review after the base":    {a1, a0, ReviewNotAncestor},
		"review commit not in app": {strings.Repeat("e", 40), a0, ReviewAnchorUnknown},
	} {
		got := state(c.reviewed, map[string]RepositoryPin{"app": {Dir: app, Revision: c.revision}})
		if got.ReviewState != c.want || got.ReviewedAtRepository != c.reviewed {
			t.Errorf("%s: state %q, want %q (%+v)", name, got.ReviewState, c.want, got)
		}
		if c.want == ReviewReviewed && got.Blob == "" {
			t.Errorf("%s: reviewed qualified link has no application blob", name)
		}
	}
	if got := state(a0, map[string]RepositoryPin{"other": {Dir: app, Revision: a0}}); got.ReviewState != ReviewTargetUnpinned {
		t.Fatalf("a link naming an unbound ID was %q", got.ReviewState)
	}
}

// AFU-V1-070: every coverage tier path is a canonical root path or a canonical path of a declared
// repository; an undeclared ID or a non-canonical path could miss the change it names and become an
// exclusion proof, so the coverage file is malformed.
func TestAFUV1070CoverageTierPathsValidated(t *testing.T) {
	t.Parallel()
	origin := strings.Repeat("a", 40)
	p := SelectionProvider{Repositories: []SelectionRepository{{ID: "app", Origin: origin}}, Inventory: []InventoryTest{{TestKey: "t", Project: "chromium", Path: "e2e/t.spec.ts"}}}
	coverage := func(path string) CoverageFile {
		return CoverageFile{Schema: CoverageSchema, Records: []CoverageRecord{{TestKey: "t", Commit: origin, Repositories: map[string]string{"app": origin},
			Tiers: []CoverageTier{{Tier: "server", Complete: true, Paths: []string{"e2e/t.spec.ts", path}}}}}}
	}
	for _, ok := range []string{"app:src/search.rb", "src/search.rb"} {
		if err := validCoverage(p, coverage(ok)); err != nil {
			t.Errorf("canonical tier path %q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"other:src/search.rb", "app:../src/search.rb", "app:./src/search.rb", "app:", "./src/search.rb", "../src/search.rb", "x:y:z.rb"} {
		if validCoverage(p, coverage(bad)) == nil {
			t.Errorf("tier path %q accepted", bad)
		}
	}
}

// AFU-V1-068: a checkout whose ancestry Git rewrites, through info/grafts or a shallow boundary,
// cannot bind: rev-list and merge-base honor both even with replace objects disabled.
func TestAFUV1068AncestryRewrittenRefused(t *testing.T) {
	t.Parallel()
	app := t.TempDir()
	writeRaw(t, app, "src/pay.rb", []byte("def pay; end\n"))
	gitTest(t, app, "init", "-q")
	origin := commitAll(t, app, "app")
	writeRaw(t, app, "src/pay.rb", []byte("def pay(n); end\n"))
	commitAll(t, app, "second")
	if reason := ancestryRewritten(context.Background(), app); reason != "" {
		t.Fatalf("plain checkout refused: %s", reason)
	}
	grafted := t.TempDir()
	gitTest(t, grafted, "clone", "-q", app, ".")
	gitTest(t, grafted, "checkout", "-q", "--orphan", "forged")
	forged := commitAll(t, grafted, "foreign root")
	if err := os.WriteFile(filepath.Join(grafted, ".git", "info", "grafts"), []byte(forged+" "+origin+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := SelectionRepository{ID: "app", Origin: origin}
	rows, bound, codes := bindRepositories(context.Background(), t.TempDir(), []SelectionRepository{repo}, []E2ERepositoryInput{{ID: "app", Source: grafted, Base: origin}})
	if len(bound) != 0 || len(codes) != 1 || codes[0].Code != CodeRepositoryUnbound || !strings.Contains(rows[0].Reason, "grafts") {
		t.Fatalf("grafted checkout bound: rows %+v codes %+v", rows, codes)
	}
	shallow := t.TempDir()
	gitTest(t, shallow, "clone", "-q", "--depth", "1", "file://"+app, ".")
	if reason := ancestryRewritten(context.Background(), shallow); reason != "checkout is shallow" {
		t.Fatalf("shallow checkout reason %q", reason)
	}
}

// AFP-V0-156: a source binds only at the top of a Git worktree, proved from an empty prefix rather
// than a located top path: a plain directory and a Git directory are not checkouts, and a
// subdirectory is not the top. No non-test source of this package names the repository-root
// locator option that marks it an unbounded reader (AFP-V0-012 rule (d)).
func TestAFPV0156BindRepositoryTopLevel(t *testing.T) {
	t.Parallel()
	app := t.TempDir()
	writeRaw(t, app, "src/pay.rb", []byte("def pay; end\n"))
	gitTest(t, app, "init", "-q")
	origin := commitAll(t, app, "app")
	repo := SelectionRepository{ID: "app", Origin: origin}
	for source, want := range map[string]string{
		t.TempDir():                "not a Git checkout",
		filepath.Join(app, ".git"): "not a Git checkout",
		filepath.Join(app, "src"):  "not the top level of a Git checkout",
	} {
		rows, bound, _ := bindRepositories(context.Background(), t.TempDir(), []SelectionRepository{repo}, []E2ERepositoryInput{{ID: "app", Source: source, Base: origin}})
		if len(bound) != 0 || len(rows) != 1 || rows[0].Reason != want {
			t.Errorf("%s: rows %+v, want reason %q", source, rows, want)
		}
	}
	rows, bound, codes := bindRepositories(context.Background(), t.TempDir(), []SelectionRepository{repo}, []E2ERepositoryInput{{ID: "app", Source: app, Base: origin}})
	if len(bound) != 1 || len(codes) != 0 {
		t.Fatalf("top-level checkout refused: rows %+v codes %+v", rows, codes)
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if data, err := os.ReadFile(name); err != nil {
			t.Fatal(err)
		} else if strings.Contains(string(data), "--show-"+"toplevel") {
			t.Errorf("%s names the repository-root locator", name)
		}
	}
}
