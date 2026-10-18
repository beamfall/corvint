package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// e2eRepositoryFixture is a test repository T, holding the Playwright suite, the provider and the
// flows, beside an application repository A at ../app (AFU-V1-067..071).
type e2eRepositoryFixture struct {
	root, app        string
	base, anchor     string
	appOrigin        string
	appBase, appHead string
}

const (
	e2eBillingKey = "billing.spec.ts > bills"
	e2eSearchKey  = "search.spec.ts > finds"
)

// buildE2ERepositoryFixture commits A at its origin, then T with flows linking each test to an A
// file, reviewed at T's first commit and A's origin when reviewed is set. Coverage names A's origin.
func buildE2ERepositoryFixture(t *testing.T, reviewed bool, coverage func(app string) []appflows.CoverageRecord) e2eRepositoryFixture {
	t.Helper()
	dir := t.TempDir()
	f := e2eRepositoryFixture{root: filepath.Join(dir, "tests"), app: filepath.Join(dir, "app")}
	shopWrite(t, f.app, map[string]string{
		"Gemfile": "source 'https://rubygems.org'\n", "src/billing/invoice.rb": "def invoice; end\n",
		"src/catalog/search.rb": "def search; end\n", "src/admin/report.rb": "def report; end\n",
	})
	shopGit(t, f.app, "init", "-q")
	f.appOrigin = shopCommit(t, f.app, "A: application")
	f.appBase = f.appOrigin
	provider := map[string]any{
		"schema": appflows.SelectionProviderSchema, "flows": "flows", "runner_config": "playwright.config.ts", "discovery": "discovery.json",
		"coverage": "coverage.json", "global_paths": []string{}, "flow_tiers": map[string][]string{"billing": {"client", "server"}, "search": {"client", "server"}},
		"inventory": []map[string]string{
			{"test_key": e2eBillingKey, "project": "chromium", "path": "e2e/billing.spec.ts"},
			{"test_key": e2eSearchKey, "project": "chromium", "path": "e2e/search.spec.ts"},
		},
		"repositories": []map[string]string{{"id": "app", "origin": f.appOrigin}},
	}
	files := map[string]string{
		".gitignore": "coverage.json\ndiscovery.json\n", "playwright.config.ts": `export default { testDir: "e2e", projects: [{ name: "chromium" }] };`,
		"e2e/billing.spec.ts": "test(\"bills\", () => 1);\n", "e2e/search.spec.ts": "test(\"finds\", () => 1);\n",
		"e2e-provider.json": e2eJSON(t, provider),
	}
	flows := func(anchor string) {
		for flow, source := range map[string]string{"billing": "app:src/billing/invoice.rb", "search": "app:src/catalog/search.rb"} {
			test := map[string]string{"billing": e2eBillingKey, "search": e2eSearchKey}[flow]
			sourceLink := map[string]any{"from": "act", "basis": "declared", "target": map[string]any{"type": "source", "path": source}}
			testLink := map[string]any{"from": flow + ".basic", "basis": "declared", "target": map[string]any{"type": "test", "path": "e2e/" + flow + ".spec.ts", "test_key": test}}
			if anchor != "" {
				sourceLink["reviewed_at"], sourceLink["reviewed_at_repository"], testLink["reviewed_at"] = anchor, f.appOrigin, anchor
			}
			files["flows/"+flow+".json"] = e2eJSON(t, map[string]any{
				"schema": "application-flow-intent/1", "flow_id": flow, "revision": 1, "kind": "ui", "actor": "user", "preconditions": []string{},
				"steps":      []map[string]string{{"step_id": "act", "action": "do " + flow}},
				"outcomes":   []map[string]string{{"outcome_id": "done", "behavior": "done visible", "matcher": "toBeVisible", "locator": "#done", "value": "done"}},
				"variations": []map[string]any{{"variation_id": flow + ".basic", "preconditions": []string{}, "steps": []string{"act"}, "observable_facts": []string{}, "outcomes": []string{"done"}, "projects": []string{"chromium"}}},
				"links":      []any{sourceLink, testLink},
			})
		}
	}
	flows("")
	shopWrite(t, f.root, files)
	shopGit(t, f.root, "init", "-q")
	f.anchor = shopCommit(t, f.root, "T: suite, provider and intents")
	f.base = f.anchor
	if reviewed {
		flows(f.anchor)
		shopWrite(t, f.root, files)
		f.base = shopCommit(t, f.root, "T: review at the first commit")
	}
	records := coverage(f.appOrigin)
	for i := range records {
		records[i].Commit = f.base
	}
	raw, err := json.Marshal(appflows.CoverageFile{Schema: appflows.CoverageSchema, Records: records})
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, f.root, map[string]string{"coverage.json": string(raw)})
	writeE2EDiscovery(t, f.root, f.base)
	return f
}

// e2eAppCoverage is complete two-tier coverage per test naming A's origin; each test covers its A file.
func e2eAppCoverage(app string) []appflows.CoverageRecord {
	record := func(key, spec, source string) appflows.CoverageRecord {
		return appflows.CoverageRecord{TestKey: key, Repositories: map[string]string{"app": app}, Tiers: []appflows.CoverageTier{
			{Tier: "client", Complete: true, Paths: []string{spec}}, {Tier: "server", Complete: true, Paths: []string{source}}}}
	}
	return []appflows.CoverageRecord{
		record(e2eBillingKey, "e2e/billing.spec.ts", "app:src/billing/invoice.rb"),
		record(e2eSearchKey, "e2e/search.spec.ts", "app:src/catalog/search.rb"),
	}
}

func noE2ECoverage(string) []appflows.CoverageRecord { return []appflows.CoverageRecord{} }

func e2eJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// appChange commits one A change and makes the new commit A's HEAD.
func (f *e2eRepositoryFixture) appChange(t *testing.T, name, body string) {
	t.Helper()
	shopWrite(t, f.app, map[string]string{name: body})
	f.appHead = shopCommit(t, f.app, "A: change "+name)
}

// select runs e2e-safe over T at its base with A bound at ../app and A's base, plus extra arguments.
func (f e2eRepositoryFixture) selectWith(t *testing.T, arguments ...string) appflows.E2ESelection {
	t.Helper()
	all := append([]string{"--base", f.base, "--provider", "e2e-provider.json", "--selection-profile", "e2e-safe"}, arguments...)
	receipt, _, stderr, code := runAffectedArguments(t, f.root, all...)
	if code != 0 {
		t.Fatalf("affected %v exited %d: %s", arguments, code, stderr)
	}
	raw, err := json.Marshal(testSelection(t, receipt))
	if err != nil {
		t.Fatal(err)
	}
	var selection appflows.E2ESelection
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selection); err != nil {
		t.Fatalf("test_selection is not e2e-safe-selection/1: %v\n%s", err, raw)
	}
	checkE2EProofDigests(t, raw)
	return selection
}

func (f e2eRepositoryFixture) selectBound(t *testing.T) appflows.E2ESelection {
	t.Helper()
	return f.selectWith(t, "--repository", "app=../app", "--repository-base", "app="+f.appBase)
}

func e2eHasFallback(t *testing.T, selection appflows.E2ESelection, want appflows.E2EFallback) {
	t.Helper()
	e2eFullSuite2(t, selection)
	if !slices.Contains(selection.Fallback, want) {
		t.Fatalf("fallback %+v lacks %+v", selection.Fallback, want)
	}
}

// e2eFullSuite2 is e2eFullSuite for the two-test fixture.
func e2eFullSuite2(t *testing.T, selection appflows.E2ESelection) {
	t.Helper()
	if selection.State != "full-relevant-suite-required" || selection.StateReason != "e2e-fallback" || len(selection.OmittedTests) != 0 || len(selection.Selected) != 2 {
		t.Fatalf("a fallback code must yield the full relevant suite: %+v", selection)
	}
}

// AFU-V1-067: --repository under e2e-safe needs --base and one --repository-base per bound ID;
// --repository-base is refused elsewhere and malformed or unpaired values are argument errors.
func TestAFUV1067RepositoryArguments(t *testing.T) {
	t.Parallel()
	f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
	profile := []string{"--provider", "e2e-provider.json", "--selection-profile", "e2e-safe"}
	base := "app=" + f.appBase
	for _, c := range []struct {
		arguments []string
		message   string
	}{
		{append([]string{"--repository", "app=../app", "--repository-base", base}, profile...), "requires --base"},
		{append([]string{"--base", f.base, "--repository", "app=../app"}, profile...), "needs one --repository-base"},
		{append([]string{"--base", f.base, "--repository-base", base}, profile...), "every id must be bound by --repository"},
		{append([]string{"--base", f.base, "--repository", "app=../app", "--repository-base", base, "--repository-base", base}, profile...), "is given twice"},
		{append([]string{"--base", f.base, "--repository", "app=../app", "--repository-base", "app=HEAD"}, profile...), "expected ID=FULL_COMMIT"},
		{append([]string{"--base", f.base, "--repository", "app=../app", "--repository-base", "a:b=" + f.appBase}, profile...), "expected ID=FULL_COMMIT"},
		{[]string{"--base", f.base, "--provider", "e2e-provider.json", "--repository", "app=../app", "--repository-base", base}, "--repository-base requires --selection-profile e2e-safe"},
	} {
		if _, _, stderr, code := runAffectedArguments(t, f.root, c.arguments...); code != 2 || !strings.Contains(stderr, c.message) {
			t.Errorf("%v: exit %d %q, want 2 and %q", c.arguments, code, stderr, c.message)
		}
	}
	if selection := f.selectBound(t); len(selection.Repositories) != 1 || selection.Repositories[0].State != appflows.RepositoryBound {
		t.Fatalf("declared repository with its base did not bind: %+v", selection.Repositories)
	}
}

// AFU-V1-068 AFU-V1-070 AFU-V1-071: a change in application repository A selects the test in test
// repository T whose coverage or reviewed link names the changed A file, omits the other on the
// coverage or reviewed-links basis, and binds A's base, revision and tree into the advice.
func TestAFUV1068SeparateTestRepositorySelects(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		coverage func(string) []appflows.CoverageRecord
		basis    string
	}{
		{"coverage", e2eAppCoverage, appflows.BasisCoverage},
		{"reviewed-links", noE2ECoverage, appflows.BasisReviewedLinks},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := buildE2ERepositoryFixture(t, true, c.coverage)
			f.appChange(t, "src/billing/invoice.rb", "def invoice(total); end\n")
			selection := f.selectBound(t)
			if selection.State != "narrow-selection-allowed" || len(selection.Selected) != 1 || selection.Selected[0].TestKey != e2eBillingKey {
				t.Fatalf("A change did not select the T billing test alone: %+v", selection)
			}
			if !slices.Contains(selection.Selected[0].Reasons, "linked-to-closure") {
				t.Fatalf("billing reasons %v lack linked-to-closure", selection.Selected[0].Reasons)
			}
			if len(selection.OmittedTests) != 1 || selection.OmittedTests[0].TestKey != e2eSearchKey || selection.OmittedTests[0].Basis != c.basis {
				t.Fatalf("search not omitted on the %s basis: %+v", c.basis, selection.OmittedTests)
			}
			proof := e2eProof(t, selection, selection.OmittedTests[0])
			if c.basis == appflows.BasisReviewedLinks && (len(proof.Links) == 0 || proof.Links[0].ReviewedAtRepository != f.appOrigin) {
				t.Fatalf("reviewed-links proof does not name the reviewed A commit: %+v", proof)
			}
			if c.basis == appflows.BasisCoverage && proof.Coverage.Repositories["app"] != f.appOrigin {
				t.Fatalf("coverage proof does not name the A commit: %+v", proof)
			}
			tree := shopGit(t, f.app, "rev-parse", "HEAD^{tree}")
			want := []appflows.E2ERepository{{ID: "app", Origin: f.appOrigin, State: appflows.RepositoryBound, Base: f.appBase, Revision: f.appHead, Tree: tree}}
			if !slices.Equal(selection.Repositories, want) {
				t.Fatalf("repositories %+v, want %+v", selection.Repositories, want)
			}
			if !slices.Equal(selection.DisjointFrom, []string{"app:src/billing/invoice.rb"}) {
				t.Fatalf("disjoint_from %v does not name the qualified A change", selection.DisjointFrom)
			}
		})
	}
	t.Run("uncommitted A change", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		f.appHead = f.appOrigin
		shopWrite(t, f.app, map[string]string{"src/catalog/search.rb": "def search(q); end\n"})
		selection := f.selectBound(t)
		if selection.State != "narrow-selection-allowed" || len(selection.Selected) != 1 || selection.Selected[0].TestKey != e2eSearchKey {
			t.Fatalf("uncommitted A change did not select the T search test: %+v", selection)
		}
		if selection.Repositories[0].WorktreePaths != 1 || selection.Repositories[0].Revision != f.appOrigin {
			t.Fatalf("uncommitted A path not counted: %+v", selection.Repositories)
		}
	})
}

// AFU-V1-068 AFU-V1-069: an A path no coverage record or reviewed link names, an unbound,
// undeclared, foreign or grafted repository, an A path a qualified path cannot name and an A global
// file each fail closed.
func TestAFUV1069UnreachedApplicationPathFailsClosed(t *testing.T) {
	t.Parallel()
	t.Run("unreached A path", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		f.appChange(t, "src/admin/report.rb", "def report(day); end\n")
		selection := f.selectBound(t)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeUnmappedChange, Subject: e2eSearchKey})
		if !slices.Equal(e2eCodes(selection), []string{appflows.CodeUnmappedChange}) {
			t.Fatalf("codes %v, want only %s", e2eCodes(selection), appflows.CodeUnmappedChange)
		}
	})
	t.Run("declared repository not bound", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		f.appChange(t, "src/billing/invoice.rb", "def invoice(total); end\n")
		selection := f.selectWith(t)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeRepositoryUnbound, Subject: "app"})
		if len(selection.Repositories) != 1 || selection.Repositories[0].State != appflows.RepositoryUnbound {
			t.Fatalf("unbound repository row missing: %+v", selection.Repositories)
		}
	})
	t.Run("undeclared binding", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		selection := f.selectWith(t, "--repository", "app=../app", "--repository-base", "app="+f.appBase, "--repository", "other=../app", "--repository-base", "other="+f.appBase)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeRepositoryUnbound, Subject: "other"})
	})
	t.Run("foreign checkout", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		selection := f.selectWith(t, "--repository", "app=.", "--repository-base", "app="+f.base)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeRepositoryUnbound, Subject: "app"})
		if reason := selection.Repositories[0].Reason; !strings.Contains(reason, "origin") {
			t.Fatalf("foreign checkout reason %q does not name the origin", reason)
		}
	})
	t.Run("subdirectory checkout", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		selection := f.selectWith(t, "--repository", "app=../app/src", "--repository-base", "app="+f.appBase)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeRepositoryUnbound, Subject: "app"})
	})
	t.Run("grafted foreign checkout", func(t *testing.T) {
		t.Parallel()
		// A foreign history grafted onto the declared origin passes rev-list and merge-base, which
		// honor info/grafts even with replace objects disabled; the checkout must not bind.
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		graft := filepath.Join(filepath.Dir(f.app), "graft")
		shopGit(t, filepath.Dir(f.app), "clone", "-q", f.app, graft)
		shopGit(t, graft, "checkout", "-q", "--orphan", "forged")
		shopWrite(t, graft, map[string]string{"src/billing/invoice.rb": "def invoice(forged); end\n"})
		forged := shopCommit(t, graft, "foreign root")
		shopWrite(t, graft, map[string]string{".git/info/grafts": forged + " " + f.appOrigin + "\n"})
		selection := f.selectWith(t, "--repository", "app=../graft", "--repository-base", "app="+f.appOrigin)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeRepositoryUnbound, Subject: "app"})
		if reason := selection.Repositories[0].Reason; !strings.Contains(reason, "grafts") {
			t.Fatalf("grafted checkout reason %q does not name the grafts", reason)
		}
	})
	t.Run("A path a qualified path cannot name", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("a colon cannot name a Windows file")
		}
		// The A change x:y.ts would read as app:x:y.ts, which T can hold as a root file of its own;
		// the root graph must never own it, so the repository does not bind.
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		shopWrite(t, f.root, map[string]string{"app:x:y.ts": "export const y = 1;\n"})
		f.base = shopCommit(t, f.root, "T: a root file named like a qualified path")
		writeE2EDiscovery(t, f.root, f.base)
		f.appChange(t, "x:y.ts", "export const y = 2;\n")
		selection := f.selectBound(t)
		e2eHasFallback(t, selection, appflows.E2EFallback{Code: appflows.CodeRepositoryUnbound, Subject: "app"})
		if reason := selection.Repositories[0].Reason; !strings.Contains(reason, "qualified path cannot name") {
			t.Fatalf("unrepresentable A path reason %q", reason)
		}
	})
	t.Run("A global file", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, true, e2eAppCoverage)
		f.appChange(t, "Gemfile", "source 'https://gems.example'\n")
		e2eHasFallback(t, f.selectBound(t), appflows.E2EFallback{Code: appflows.CodeGlobalPathChanged, Subject: "app:Gemfile"})
	})
}

// AFU-V1-070: coverage narrows only when the record names A's commit and no covered A path changed
// between that commit and A's base; links are left unreviewed so coverage is the only basis.
func TestAFUV1070CoverageRepositoryCommitRequired(t *testing.T) {
	t.Parallel()
	t.Run("record without the A commit", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, false, func(app string) []appflows.CoverageRecord {
			records := e2eAppCoverage(app)
			records[1].Repositories = nil
			return records
		})
		f.appChange(t, "src/billing/invoice.rb", "def invoice(total); end\n")
		e2eHasFallback(t, f.selectBound(t), appflows.E2EFallback{Code: appflows.CodeMapStale, Subject: e2eSearchKey})
	})
	t.Run("covered A path changed since the record", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, false, e2eAppCoverage)
		f.appChange(t, "src/catalog/search.rb", "def search(q); end\n")
		f.appBase = f.appHead
		f.appChange(t, "src/billing/invoice.rb", "def invoice(total); end\n")
		e2eHasFallback(t, f.selectBound(t), appflows.E2EFallback{Code: appflows.CodeMapStale, Subject: e2eSearchKey})
	})
	t.Run("fresh record narrows", func(t *testing.T) {
		t.Parallel()
		f := buildE2ERepositoryFixture(t, false, e2eAppCoverage)
		f.appChange(t, "src/billing/invoice.rb", "def invoice(total); end\n")
		selection := f.selectBound(t)
		if selection.State != "narrow-selection-allowed" || len(selection.OmittedTests) != 1 || selection.OmittedTests[0].Basis != appflows.BasisCoverage {
			t.Fatalf("fresh coverage naming the A commit did not narrow: %+v", selection)
		}
	})
}
