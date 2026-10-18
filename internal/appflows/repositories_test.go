package appflows

import (
	"context"
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
