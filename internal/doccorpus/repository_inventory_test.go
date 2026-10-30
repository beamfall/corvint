package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// inventoryRepos commits a service repository and a shared library repository side by side.
// The service builds an image, ships a chart and deploys from CI; it consumes the library's Go
// module, a module no covered repository declares and a reusable workflow named by the library's
// declared alias. The library declares a Go module and an npm package and no deployable unit.
func inventoryRepos(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	files := map[string]map[string]string{
		"app": {
			"go.mod":                      "module example.invalid/app\n\ngo 1.27.1\n\nrequire (\n\texample.invalid/lib v1.0.0\n\texample.invalid/outside v0.3.0\n\texample.invalid/indirect v0.1.0 // indirect\n)\n",
			"deploy/Dockerfile":           "FROM golang:1.27 AS build\nRUN go build ./...\nFROM build AS test\nFROM registry.example.invalid/base/runtime:3\n",
			"deploy/chart/Chart.yaml":     "apiVersion: v2\nname: app-chart\nversion: 1.0.0\ndependencies:\n  - name: lib-chart\n    version: 1.x\n",
			".github/workflows/ci.yml":    "name: ci\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n  deploy-prod:\n    uses: example/shared-workflows/.github/workflows/deploy.yml@main\n",
			"docs/overview.md":            "# Service\n\nServes requests.\n",
			"deploy/chart/values.yaml":    "replicas: 1\n",
			"deploy/chart/templates/a.md": "# Template\n\nRendered.\n",
		},
		"lib": {
			"go.mod":       "module example.invalid/lib\n\ngo 1.27.1\n",
			"package.json": "{\n  \"name\": \"@example/ui\",\n  \"version\": \"1.0.0\",\n  \"dependencies\": {\n    \"left-pad\": \"1.3.0\"\n  }\n}\n",
			"lib.go":       "package lib\n\nfunc Shared() {}\n",
		},
	}
	for _, name := range []string{"app", "lib"} {
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
	return filepath.Join(parent, "app"), filepath.Join(parent, "lib")
}

func inventoryManifest(t *testing.T, app, lib string) Manifest {
	t.Helper()
	m, err := InventoryWorkspace(context.Background(), app, git(t, app, "rev-parse", "HEAD"), []string{".github", "deploy", "docs", "go.mod"}, ManifestSchema, "2026-10-28T00:00:00Z", []WorkspaceRepository{{Alias: "lib", Root: lib, Scopes: []string{"go.mod", "lib.go", "package.json"}}})
	if err != nil {
		t.Fatal(err)
	}
	m.RepositoryInventory = &RepositoryInventory{Repositories: []RepositoryDeclaration{{Repository: "lib", Name: "shared-lib", Aliases: []string{"example/shared-workflows"}, Archived: true}}}
	return m
}

func inventoryGaps(t *testing.T, a *Artifact, kind string) []Gap {
	t.Helper()
	r, err := Query(a, Request{Operation: "gaps", Kinds: []string{kind}, Limit: MaxResults}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	out := []Gap{}
	for _, v := range r.Results {
		out = append(out, v.(Gap))
	}
	return out
}

// TestDCPV1700DCPV1701InfoListsCoveredRepositories: info and validate list every covered repository
// with its identity, declared name, pinned revision and freshness; a live reader observes each
// checkout, a pure reader reports it unknown, and a manifest without the declaration or further
// repositories keeps its info bytes (DCP-V1-700, DCP-V1-701).
func TestDCPV1700DCPV1701InfoListsCoveredRepositories(t *testing.T) {
	t.Parallel()
	app, lib := inventoryRepos(t)
	m := inventoryManifest(t, app, lib)
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"info", "validate"} {
		r, err := ReadQuery(context.Background(), app, data, Request{Operation: op})
		if err != nil {
			t.Fatal(err)
		}
		listed, ok := r.Results[0].(map[string]any)["repositories"].([]RepositoryListing)
		if !ok || len(listed) != 2 {
			t.Fatalf("%s repositories %#v", op, r.Results[0])
		}
		primary, extra := listed[0], listed[1]
		if primary.ID != m.Repository.ID || primary.Role != "primary" || primary.Revision != m.Repository.Revision || primary.NameSource != "undeclared" || primary.Freshness != "fresh" || primary.Archived != "unknown" {
			t.Fatalf("%s primary %+v", op, primary)
		}
		if extra.ID != m.AdditionalRepositories[0].ID || extra.Name != "shared-lib" || extra.NameSource != "repository_inventory" || extra.Alias != "lib" || extra.Revision != m.AdditionalRepositories[0].Revision || extra.Freshness != "fresh" || extra.Archived != "declared" || !slices.Equal(extra.Aliases, []string{"example/shared-workflows"}) {
			t.Fatalf("%s additional %+v", op, extra)
		}
	}
	if err := os.WriteFile(filepath.Join(lib, "lib.go"), []byte("package lib\n\nfunc Changed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, lib, "commit", "-qam", "move")
	r, err := ReadQuery(context.Background(), app, data, Request{Operation: "info"})
	if err != nil {
		t.Fatal(err)
	}
	listed := r.Results[0].(map[string]any)["repositories"].([]RepositoryListing)
	if listed[0].Freshness != "fresh" || listed[1].Freshness != "stale" {
		t.Fatalf("per-repository freshness %+v", listed)
	}
	pure, err := Query(a, Request{Operation: "info"}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if l := pure.Results[0].(map[string]any)["repositories"].([]RepositoryListing); l[0].Freshness != "unknown" || l[0].FreshnessReason == "" {
		t.Fatalf("pure reader freshness %+v", l)
	}
	root, single := fixture(t)
	plain, err := Build(context.Background(), root, single)
	if err != nil {
		t.Fatal(err)
	}
	info, err := Query(plain, Request{Operation: "info"}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, listed := info.Results[0].(map[string]any)["repositories"]; listed || slices.ContainsFunc(plain.Subjects, func(s Subject) bool { return s.Kind == "repository" }) {
		t.Fatalf("undeclared single-repository corpus changed: %#v", info.Results[0])
	}
	for _, bad := range []RepositoryInventory{
		{Repositories: []RepositoryDeclaration{{Repository: "missing"}}},
		{Repositories: []RepositoryDeclaration{{Repository: "lib", Aliases: []string{"b", "a"}}}},
		{Repositories: []RepositoryDeclaration{}},
	} {
		refused := inventoryManifest(t, app, lib)
		refused.RepositoryInventory = &bad
		if _, err := Build(context.Background(), app, refused); err == nil || !strings.Contains(err.Error(), "repository inventory") {
			t.Fatalf("declaration %+v admitted: %v", bad, err)
		}
	}
}

// TestDCPV1702RepositorySubjectsSearchableByNameAndAlias: each covered repository is a repository
// subject named by its declaration, found by search on its name and on each alias, and resolved by
// get on an alias (DCP-V1-702).
func TestDCPV1702RepositorySubjectsSearchableByNameAndAlias(t *testing.T) {
	t.Parallel()
	app, lib := inventoryRepos(t)
	m := inventoryManifest(t, app, lib)
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	libID := repositorySubjectID("native", m.AdditionalRepositories[0].ID)
	for _, query := range []string{"shared-lib", "lib", "shared-workflows"} {
		r, err := Query(a, Request{Operation: "search", Query: query, Kinds: []string{"repository"}}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(receiptIDs(r), libID) {
			t.Fatalf("search %q missed %s: %v", query, libID, receiptIDs(r))
		}
	}
	got, err := Query(a, Request{Operation: "get", ID: "example/shared-workflows"}, "fresh", nil)
	if err != nil || got.AliasResolution == nil || got.AliasResolution.ID != libID {
		t.Fatalf("alias get %+v %v", got, err)
	}
}

// TestDCPV1703DCPV1704DCPV1705DeployablesPackagesAndReconciliation: deployable units and packages
// come only from declared inputs by the closed rules, each anchored at its line; references to a
// covered package or alias join as depends_on relations, the others are outside-corpus gaps; an
// archived declaration and every repository without evidence are reported, never inferred
// (DCP-V1-703, DCP-V1-704, DCP-V1-705).
func TestDCPV1703DCPV1704DCPV1705DeployablesPackagesAndReconciliation(t *testing.T) {
	t.Parallel()
	app, lib := inventoryRepos(t)
	m := inventoryManifest(t, app, lib)
	a, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]Subject{}
	for _, s := range a.Subjects {
		subjects[s.ID] = s
	}
	libRevision := m.AdditionalRepositories[0].Revision
	for id, want := range map[string]struct {
		kind, name, path, revision string
		line                       int
	}{
		"native:deployable:deploy/Dockerfile":                    {"deployable_unit", "deploy/Dockerfile", "deploy/Dockerfile", m.Repository.Revision, 1},
		"native:deployable:deploy/chart/Chart.yaml":              {"deployable_unit", "app-chart", "deploy/chart/Chart.yaml", m.Repository.Revision, 2},
		"native:deployable:.github/workflows/ci.yml:deploy-prod": {"deployable_unit", "deploy-prod (prod)", ".github/workflows/ci.yml", m.Repository.Revision, 7},
		"native:package:go.mod":                                  {"package", "example.invalid/app", "go.mod", m.Repository.Revision, 1},
		"native:package:lib:go.mod":                              {"package", "example.invalid/lib", "go.mod", libRevision, 1},
		"native:package:lib:package.json":                        {"package_manifest", "@example/ui", "package.json", libRevision, 2},
	} {
		s, ok := subjects[id]
		if !ok || s.Kind != want.kind || s.Name != want.name || len(s.Evidence.Anchors) != 1 {
			t.Fatalf("subject %s = %+v, want %+v", id, s, want)
		}
		anchor := s.Evidence.Anchors[0]
		if anchor.Path != want.path || anchor.Revision != want.revision || anchor.Start != want.line || s.Evidence.Derivation != "source-derived" || !strings.Contains(anchor.Reason, "repository inventory rule") {
			t.Fatalf("subject %s anchor %+v", id, anchor)
		}
	}
	if _, ok := subjects["native:deployable:.github/workflows/ci.yml:test"]; ok {
		t.Fatal("a test job was derived as a deploy job")
	}
	relations := map[string]Relation{}
	for _, r := range a.Relations {
		relations[r.ID] = r
	}
	libRepository := repositorySubjectID("native", m.AdditionalRepositories[0].ID)
	if r := relations["native:reference:go.mod:6:go:example.invalid/lib:resolves"]; r.From != "native:package:go.mod" || r.To != "native:package:lib:go.mod" || r.Type != "depends_on" || len(r.Evidence.Anchors) != 1 {
		t.Fatalf("covered Go module join %+v", r)
	}
	if r := relations["native:reference:.github/workflows/ci.yml:8:action:example/shared-workflows:resolves"]; r.From != "native:deployable:.github/workflows/ci.yml:deploy-prod" || r.To != libRepository {
		t.Fatalf("declared alias join %+v", r)
	}
	outside := []string{}
	for _, g := range inventoryGaps(t, a, "outside-corpus-reference") {
		outside = append(outside, g.Reason)
		if !strings.Contains(g.Reason, "NOT_OBSERVED") {
			t.Fatalf("outside reference states no uncertainty: %+v", g)
		}
	}
	for _, want := range []string{"native:reference:go.mod", "native:reference:deploy/Dockerfile", "native:reference:deploy/chart/Chart.yaml", "native:reference:.github/workflows/ci.yml", "native:reference:lib:package.json"} {
		if !slices.ContainsFunc(outside, func(s string) bool { return strings.HasPrefix(s, want+":") }) {
			t.Fatalf("outside-corpus reference %s missing from %v", want, outside)
		}
	}
	if slices.ContainsFunc(outside, func(s string) bool { return strings.Contains(s, "indirect") }) || len(outside) != 6 {
		t.Fatalf("outside references %v", outside)
	}
	if g := inventoryGaps(t, a, "archived-repository"); len(g) != 1 || g[0].Subject != libRepository {
		t.Fatalf("archived gaps %+v", g)
	}
	if g := inventoryGaps(t, a, "deployable-evidence-unknown"); len(g) != 1 || g[0].Subject != libRepository {
		t.Fatalf("deployable unknown gaps %+v", g)
	}
	if g := inventoryGaps(t, a, "package-evidence-unknown"); len(g) != 0 {
		t.Fatalf("package unknown gaps %+v", g)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), app, data); err != nil {
		t.Fatalf("inventory artifact does not rederive: %v", err)
	}
}

// TestDCPV1706OneLineDependenciesAndUnindentedChartLists: two package.json dependencies on one
// line are two distinct references instead of a duplicate record that refuses the corpus, and a
// chart's unindented dependency list is read up to the next top-level key, so its covered entry
// joins and its uncovered one is a gap (DCP-V1-703, DCP-V1-704, DCP-V1-706).
func TestDCPV1706OneLineDependenciesAndUnindentedChartLists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"deploy/web/package.json":  "{\"name\": \"@example/web\", \"dependencies\": {\"alpha\": \"1.0.0\", \"beta\": \"1.0.0\"}}\n",
		"deploy/api/package.json":  "{\n  \"name\": \"@example/api\",\n  \"dependencies\": {\n    \"delta\": \"1.0.0\", \"gamma\": \"1.0.0\"\n  }\n}\n",
		"deploy/shared/Chart.yaml": "apiVersion: v2\nname: shared\nversion: 1.0.0\n",
		"deploy/app/Chart.yaml":    "apiVersion: v2\nname: app\ndependencies:\n- name: shared\n  version: 1.x\n- name: external\n  version: 2.x\nappVersion: \"1\"\n",
	}
	for p, content := range files {
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
	git(t, root, "commit", "-qm", "deploy")
	m, err := Inventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), "deploy", "2026-10-29T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	m.RepositoryInventory = &RepositoryInventory{}
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatalf("one-line dependencies refused the corpus: %v", err)
	}
	claims := map[string]Claim{}
	for _, c := range a.Claims {
		claims[c.ID] = c
	}
	for _, id := range []string{"native:reference:deploy/web/package.json:1:npm:alpha", "native:reference:deploy/web/package.json:1:npm:beta"} {
		if c, ok := claims[id]; !ok || c.Subject != "native:package:deploy/web/package.json" || c.Evidence.Anchors[0].Start != 1 {
			t.Fatalf("reference %s = %+v", id, c)
		}
	}
	joined := false
	for _, r := range a.Relations {
		joined = joined || r.ID == "native:reference:deploy/app/Chart.yaml:4:chart:shared:resolves" && r.From == "native:deployable:deploy/app/Chart.yaml" && r.To == "native:deployable:deploy/shared/Chart.yaml" && r.Type == "depends_on"
	}
	if !joined {
		t.Fatalf("unindented covered chart dependency did not join: %+v", a.Relations)
	}
	outside := []string{}
	for _, g := range inventoryGaps(t, a, "outside-corpus-reference") {
		outside = append(outside, strings.SplitN(g.Reason, ": ", 2)[0])
	}
	slices.Sort(outside)
	if !slices.Equal(outside, []string{"native:reference:deploy/api/package.json:4:npm:delta", "native:reference:deploy/api/package.json:4:npm:gamma", "native:reference:deploy/app/Chart.yaml:6:chart:external", "native:reference:deploy/web/package.json:1:npm:alpha", "native:reference:deploy/web/package.json:1:npm:beta"}) {
		t.Fatalf("outside references %v", outside)
	}
}
