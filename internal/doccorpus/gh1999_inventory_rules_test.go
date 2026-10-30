package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// inventoryRepo commits files into a new repository under parent and builds its single-repository
// inventory corpus over scopes with inv as its declaration.
func inventoryRepo(t *testing.T, parent, name string, files map[string]string, scopes []string, inv *RepositoryInventory) *Artifact {
	t.Helper()
	root := filepath.Join(parent, name)
	for p, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", name)
	m, err := ManifestInventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), scopes, ManifestSchema, "2026-10-29T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	m.RepositoryInventory = inv
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
	if _, err := Open(context.Background(), root, data); err != nil {
		t.Fatalf("inventory artifact does not rederive: %v", err)
	}
	return a
}

func gapReasons(t *testing.T, a *Artifact, kind string) []string {
	t.Helper()
	out := []string{}
	for _, g := range inventoryGaps(t, a, kind) {
		out = append(out, g.Reason)
	}
	return out
}

// TestDCIV1006ToDCIV1010InventoryRulesNeedEvidence: a package.json without publish evidence is a
// package manifest, not a published package; publishConfig or a release job makes it a package;
// a gemspec is a gem package that a Gemfile joins; deploy jobs carry their named environment or
// state it NOT_OBSERVED, and a release job is no deployable unit; an unsupported CI file is named;
// and an unjoined reference under a declared first-party prefix is graded apart from third-party
// ones (GitHub #1999).
func TestDCIV1006ToDCIV1010InventoryRulesNeedEvidence(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	svc := inventoryRepo(t, parent, "svc", map[string]string{
		"package.json":                        "{\n  \"name\": \"svc\",\n  \"dependencies\": {\n    \"@acme/ui\": \"1\",\n    \"left-pad\": \"1\"\n  }\n}\n",
		"packages/pub/package.json":           "{\n  \"name\": \"@acme/pub\",\n  \"publishConfig\": {\n    \"access\": \"public\"\n  }\n}\n",
		"Gemfile":                             "source \"https://rubygems.org\"\ngem \"acme-lib\"\ngem 'rails', '~> 7'\n",
		"lib/acme-lib.gemspec":                "Gem::Specification.new do |spec|\n  spec.name = \"acme-lib\"\n  spec.add_dependency \"rack\"\n  spec.add_development_dependency \"rspec\"\nend\n",
		"Dockerfile":                          "FROM ruby:3.3.4\n",
		".github/workflows/deploy-preprd.yml": "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: preprd\n",
		".github/workflows/deploy-prod.yml":   "on: push\njobs:\n  deploy:\n    environment:\n      name: production\n      url: https://example.invalid\n  deploy-dynamic:\n    environment: ${{ inputs.env }}\n",
	}, []string{".github", "Dockerfile", "Gemfile", "lib", "package.json", "packages"}, &RepositoryInventory{FirstParty: []string{"@acme/", "acme-"}})
	subjects := map[string]Subject{}
	for _, s := range svc.Subjects {
		subjects[s.ID] = s
	}
	for id, want := range map[string][2]string{
		"native:package:package.json":                                        {"package_manifest", "svc"},
		"native:package:packages/pub/package.json":                           {"package", "@acme/pub"},
		"native:package:lib/acme-lib.gemspec":                                {"package", "acme-lib"},
		"native:deployable:.github/workflows/deploy-preprd.yml:deploy":       {"deployable_unit", "deploy (preprd)"},
		"native:deployable:.github/workflows/deploy-prod.yml:deploy":         {"deployable_unit", "deploy (production)"},
		"native:deployable:.github/workflows/deploy-prod.yml:deploy-dynamic": {"deployable_unit", "deploy-dynamic"},
	} {
		if s := subjects[id]; s.Kind != want[0] || s.Name != want[1] {
			t.Fatalf("subject %s = %+v, want %v", id, s, want)
		}
	}
	if s := subjects["native:deployable:.github/workflows/deploy-prod.yml:deploy"]; len(s.Evidence.Anchors) != 2 || s.Evidence.Anchors[1].Start != 5 {
		t.Fatalf("the named environment should be anchored at its name line: %+v", s.Evidence.Anchors)
	}
	claims := map[string]Claim{}
	for _, c := range svc.Claims {
		claims[c.ID] = c
	}
	if c := claims["native:package:package.json:derivation"]; !strings.Contains(c.Text, "publish evidence NOT_OBSERVED") {
		t.Fatalf("an unpublished manifest should state its missing publish evidence: %+v", c)
	}
	if c := claims["native:deployable:.github/workflows/deploy-prod.yml:deploy-dynamic:derivation"]; !strings.Contains(c.Text, "environment is NOT_OBSERVED") {
		t.Fatalf("a deploy job without a literal environment should state it unknown: %+v", c)
	}
	joined := false
	for _, r := range svc.Relations {
		joined = joined || r.ID == "native:reference:Gemfile:2:gem:acme-lib:resolves" && r.To == "native:package:lib/acme-lib.gemspec"
	}
	if !joined {
		t.Fatalf("the Gemfile gem should join the gemspec: %+v", svc.Relations)
	}
	firstParty := gapReasons(t, svc, "first-party-reference-outside-corpus")
	if len(firstParty) != 1 || !strings.HasPrefix(firstParty[0], "native:reference:package.json:4:npm:@acme/ui: ") || !strings.Contains(firstParty[0], "prefix @acme/") {
		t.Fatalf("the first-party dependency should be graded apart: %v", firstParty)
	}
	outside := []string{}
	for _, reason := range gapReasons(t, svc, "outside-corpus-reference") {
		outside = append(outside, strings.SplitN(reason, ": ", 2)[0])
	}
	slices.Sort(outside)
	if !slices.Equal(outside, []string{"native:reference:Dockerfile:1:image:ruby:3.3.4", "native:reference:Gemfile:3:gem:rails", "native:reference:lib/acme-lib.gemspec:3:gem:rack", "native:reference:package.json:5:npm:left-pad"}) {
		t.Fatalf("third-party references %v", outside)
	}

	lib := inventoryRepo(t, parent, "lib", map[string]string{
		"package.json":                  "{\n  \"name\": \"@acme/core\"\n}\n",
		".github/workflows/release.yml": "on: push\njobs:\n  release:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish\n",
		".gitlab-ci.yml":                "deploy:\n  environment: production\n",
	}, []string{".github", ".gitlab-ci.yml", "package.json"}, &RepositoryInventory{})
	for _, s := range lib.Subjects {
		if s.Kind == "deployable_unit" {
			t.Fatalf("a release job should not be a deployable unit: %+v", s)
		}
		if s.ID == "native:package:package.json" && s.Kind != "package" {
			t.Fatalf("a release job should be publish evidence: %+v", s)
		}
	}
	if g := gapReasons(t, lib, "inventory-ci-unsupported"); len(g) != 1 || !strings.Contains(g[0], "GitLab CI") || !strings.Contains(g[0], "NOT_OBSERVED") {
		t.Fatalf("the unsupported CI file should be named: %v", g)
	}
	if g := gapReasons(t, lib, "deployable-evidence-unknown"); len(g) != 1 || !strings.Contains(g[0], "inventory-ci-unsupported") {
		t.Fatalf("the deployable gap should point at the unsupported CI: %v", g)
	}

	r, err := Estate(context.Background(), parent, EstateInput{Schema: EstateSchema, Corpora: []EstateEntry{{Root: "lib", Artifact: "corpus.json"}, {Root: "svc", Artifact: "corpus.json"}}})
	if err != nil {
		t.Fatal(err)
	}
	if ref := findEstateReference(t, r, "npm", "@acme/ui"); ref.Grade != "first-party" || ref.State != "outside-estate" {
		t.Fatalf("the estate should keep the first-party grade: %+v", ref)
	}
	if ref := findEstateReference(t, r, "gem", "rails"); ref.Grade != "ungraded" {
		t.Fatalf("a third-party reference should stay ungraded: %+v", ref)
	}

	m := Manifest{}
	m.Providers = []Provider{{ID: "native", Kind: "native"}}
	for _, prefixes := range [][]string{{}, {"b", "a"}, {"a", "a"}, {"bad prefix"}} {
		m.RepositoryInventory = &RepositoryInventory{FirstParty: prefixes}
		if err := validateRepositoryInventory(m); err == nil || !strings.Contains(err.Error(), "first_party") {
			t.Fatalf("first_party %v should be refused: %v", prefixes, err)
		}
	}
}
