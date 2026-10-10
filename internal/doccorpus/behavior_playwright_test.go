package doccorpus

import (
	"bytes"
	"context"
	jsonstd "encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// playwrightBehaviorRepo is a generic Git repository holding a real Playwright 1.63.0 project
// (testdata/playwright-behavior) and the caller-run listing and report of that project.
type playwrightBehaviorRepo struct {
	root, revision, repository string
	revisions                  BehaviorRevisions
	migration                  []byte
	listing, report            []byte
	receipt                    []byte
	receiptIDs                 map[string]string
	// retained keeps each committed input so an unchanged document keeps its first anchor.
	retained map[string]BehaviorAdapterInput
	// extraTests registers further tests on an existing alpha execution's anchor.
	extraTests []playwrightExtraTest
}

type playwrightExtraTest struct {
	id, title string
	line      int
	fixtures  []string
}

// playwrightReceiptCase is one alpha outcome of the synthetic receipt; describe is its enclosing
// describe title.
type playwrightReceiptCase struct {
	describe, title string
	line            int
	state           jstestprovider.ExecutionState
	attempts        []jstestprovider.Attempt
}

const playwrightBehaviorConfig = "playwright.config.ts"
const playwrightBehaviorSpec = "e2e/items.spec.ts"

func playwrightBehaviorTestdata(t *testing.T, name, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "playwright-behavior", name))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(raw, []byte("@ROOT@"), []byte(root))
}

func newPlaywrightBehaviorRepo(t *testing.T) playwrightBehaviorRepo {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	files := map[string][]byte{
		playwrightBehaviorConfig: playwrightBehaviorTestdata(t, "playwright.config.ts", root),
		playwrightBehaviorSpec:   playwrightBehaviorTestdata(t, "items.spec.ts", root),
		"docs/items.md":          []byte("# Items\n\nThe items page lists every seeded item and hides the empty state.\n"),
	}
	for path, data := range files {
		writePlaywrightBehaviorFile(t, root, path, data)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	revision := git(t, root, "rev-parse", "HEAD")
	repo := playwrightBehaviorRepo{root: root, revision: revision, repository: git(t, root, "rev-list", "--max-parents=0", "HEAD"), retained: map[string]BehaviorAdapterInput{}}
	repo.revisions = BehaviorRevisions{App: Repository{strings.Repeat("1", 40), revision}, E2E: Repository{repo.repository, revision}, Docs: Repository{strings.Repeat("2", 40), revision}}
	repo.migration = behaviorAdapterRaw(t, BehaviorMigration{Revisions: repo.revisions, Schema: 2, ContractID: "items-contract", SourceRevision: revision, DocumentationRevision: revision})
	repo.listing = playwrightBehaviorTestdata(t, "list.json", root)
	repo.report = playwrightBehaviorTestdata(t, "report.json", root)
	repo.receipt, repo.receiptIDs = playwrightBehaviorReceipt(t, root, files)
	return repo
}

// commitPlaywrightBehaviorTest commits one more config-selected test file and advances the
// migration to that commit, leaving the captured listing stale.
func commitPlaywrightBehaviorTest(t *testing.T, repo *playwrightBehaviorRepo, path string) {
	t.Helper()
	writePlaywrightBehaviorFile(t, repo.root, path, []byte("import { test } from '@playwright/test';\n\ntest('extra', async () => {});\n"))
	git(t, repo.root, "add", path)
	git(t, repo.root, "commit", "-qm", "add "+path)
	repo.revision = git(t, repo.root, "rev-parse", "HEAD")
	repo.revisions.App.Revision, repo.revisions.E2E.Revision, repo.revisions.Docs.Revision = repo.revision, repo.revision, repo.revision
	repo.migration = behaviorAdapterRaw(t, BehaviorMigration{Revisions: repo.revisions, Schema: 2, ContractID: "items-contract", SourceRevision: repo.revision, DocumentationRevision: repo.revision})
}

func writePlaywrightBehaviorFile(t *testing.T, root, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
		t.Fatal(err)
	}
}

// playwrightBehaviorReceipt is a synthetic qualified receipt for the alpha project of the same
// run: it is shaped like Corvint's runner output, not live evidence.
func playwrightBehaviorReceipt(t *testing.T, root string, files map[string][]byte, extra ...playwrightReceiptCase) ([]byte, map[string]string) {
	t.Helper()
	config, spec := root+"/"+playwrightBehaviorConfig, root+"/"+playwrightBehaviorSpec
	configDigest := Digest(files[playwrightBehaviorConfig])
	native := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", Identity: jstestprovider.Identity{ConfigFile: config, ConfigDigest: configDigest, ConfigInputDigests: map[string]string{config: configDigest}, TestFileDigests: map[string]string{spec: Digest(files[playwrightBehaviorSpec])}, RunnerName: "playwright", RunnerVersion: "1.63.0", NodeVersion: "v22.23.2", Argv: []string{"playwright", "test"}}, External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ReadyURL: "http://127.0.0.1:3000", DeclaredAppIdentity: "synthetic", ConfigOverride: "controlled"}, AppBuildAtStart: jstestprovider.AppBuildIdentity{Unknown: true}, AppBuildAtPublish: jstestprovider.AppBuildIdentity{Unknown: true}}
	use := jsonstd.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	cases := append([]playwrightReceiptCase{
		{"items", "lists items", 7, "passed", []jstestprovider.Attempt{{State: "passed", Retry: 0}}},
		{"items", "rejects empty item", 17, "failed", []jstestprovider.Attempt{{State: "failed", Retry: 0, FailureKind: "assertion"}, {State: "failed", Retry: 1, FailureKind: "assertion"}}},
		{"items", "archives item", 21, "skipped", []jstestprovider.Attempt{{State: "skipped", Retry: 0}}},
		{"items", "retries item", 23, "flaky", []jstestprovider.Attempt{{State: "failed", Retry: 0, FailureKind: "assertion"}, {State: "passed", Retry: 1}}},
	}, extra...)
	ids := map[string]string{}
	for _, c := range cases {
		// FullName is titlePath().join(' > ') as the qualified reporter writes it: root, project,
		// file relative to testDir, describe, title.
		outcome := jstestprovider.TestOutcome{Name: c.title, FullName: " > alpha > items.spec.ts > " + c.describe + " > " + c.title, State: c.state, Retries: len(c.attempts) - 1, Anchor: &jstestprovider.Anchor{File: spec, Line: c.line}, Project: &jstestprovider.ProjectIdentity{Name: "alpha", Browser: "chromium", Device: "unknown", Use: use, ConfigDigest: configDigest}, Attempts: c.attempts}
		identity, err := jsonstd.Marshal(struct {
			Identity jstestprovider.Identity
			Project  *jstestprovider.ProjectIdentity
			Anchor   *jstestprovider.Anchor
			FullName string
		}{native.Identity, outcome.Project, outcome.Anchor, outcome.FullName})
		if err != nil {
			t.Fatal(err)
		}
		outcome.ID = Digest(identity)
		if _, ok := ids[c.title]; !ok {
			ids[c.title] = outcome.ID
		}
		ids[c.describe+" > "+c.title] = outcome.ID
		native.Tests = append(native.Tests, outcome)
	}
	raw, err := jstestprovider.EncodeQualified(native)
	if err != nil {
		t.Fatal(err)
	}
	return raw, ids
}

func (r playwrightBehaviorRepo) discover(t *testing.T) BehaviorDiscovery {
	t.Helper()
	raw, err := BuildPlaywrightDiscovery(context.Background(), PlaywrightDiscoveryInput{Root: r.root, Migration: r.migration, ConfigPath: playwrightBehaviorConfig, Listing: r.listing, Receipt: r.receipt})
	if err != nil {
		t.Fatal(err)
	}
	var discovery BehaviorDiscovery
	if err := decode(raw, &discovery); err != nil {
		t.Fatal(err)
	}
	encoded, _ := Encode(discovery)
	if !bytes.Equal(encoded, raw) {
		t.Fatal("discovery output is not canonical")
	}
	return discovery
}

// commitInputs retains each changed document at a new revision and returns full-file adapter
// inputs; an unchanged document keeps the anchor it was first retained at.
func (r playwrightBehaviorRepo) commitInputs(t *testing.T, documents map[string][]byte, kinds map[string]string) []BehaviorAdapterInput {
	t.Helper()
	ids, changed := []string{}, []string{}
	for id, document := range documents {
		ids = append(ids, id)
		if retained, ok := r.retained[id]; !ok || retained.Document != string(document) || retained.Anchor.Kind != kinds[id] {
			changed = append(changed, id)
		}
	}
	slices.Sort(ids)
	slices.Sort(changed)
	if len(changed) > 0 {
		for _, id := range changed {
			writePlaywrightBehaviorFile(t, r.root, "evidence/"+id+".json", documents[id])
			git(t, r.root, "add", "evidence/"+id+".json")
		}
		git(t, r.root, "commit", "-qm", "retain evidence")
		revision := git(t, r.root, "rev-parse", "HEAD")
		for _, id := range changed {
			path, digest := "evidence/"+id+".json", Digest(documents[id])
			r.retained[id] = BehaviorAdapterInput{ID: id, Anchor: Anchor{Repository: r.repository, Revision: revision, Path: path, Blob: git(t, r.root, "rev-parse", revision+":"+path), SHA256: digest, Start: 1, End: 1, SpanSHA256: digest, Authority: "external-provider", Kind: kinds[id], Reason: "synthetic retained input"}, Document: string(documents[id])}
		}
	}
	inputs := []BehaviorAdapterInput{}
	for _, id := range ids {
		inputs = append(inputs, r.retained[id])
	}
	return inputs
}

// playwrightBehaviorRequest maps one reviewed variation onto the clean alpha test and registers
// the other alpha tests and the beta clean test without a contract.
func (r playwrightBehaviorRepo) request(t *testing.T, discovery BehaviorDiscovery, runtime map[string]*BehaviorRuntime, observations []ObservationLink, extra []BehaviorAdapterInput) BehaviorAdapterRequest {
	t.Helper()
	executions := map[string]BehaviorExecution{}
	for _, execution := range discovery.Executions {
		executions[execution.Project+"\x00"+execution.Evidence.Path+"\x00"+itoa(execution.Evidence.Start)] = execution
	}
	execution := func(project string, line int) BehaviorExecution {
		found, ok := executions[project+"\x00"+playwrightBehaviorSpec+"\x00"+itoa(line)]
		if !ok {
			t.Fatalf("discovery lacks %s:%d", project, line)
		}
		return found
	}
	clean := execution("alpha", 7)
	doc := declaredEvidence(t, r.root, Manifest{Repository: Repository{r.repository, r.revision}}, "docs/items.md", 3).Anchors[0]
	doc.Revision, doc.Blob = r.revision, git(t, r.root, "rev-parse", r.revision+":docs/items.md")
	annotation := clean.Evidence
	annotation.Kind, annotation.Reason = "review", "reviewed assertion annotation"
	events := []BehaviorEvent{
		{Sequence: 1, Kind: "page", ID: "/items", Passed: true, Context: "context-1", Page: "page-1", Frame: "main", Navigation: "main-frame"},
		{Sequence: 2, Kind: "assertion", ID: "items-visible", Passed: true, Context: "context-1", Page: "page-1", Frame: "main", Behavior: "list-items", Criterion: "items-visible", Matcher: "toHaveText", Locator: "#count", Value: "2"},
		{Sequence: 3, Kind: "negative-control", ID: "empty-hidden", Passed: true, Context: "context-1", Page: "page-1", Frame: "main"},
		{Sequence: 4, Kind: "page", ID: "/done", Passed: true, Context: "context-1", Page: "page-1", Frame: "main", Navigation: "main-frame"},
	}
	flows := []any{map[string]any{"id": "items-flow", "derivation": "declared", "evidence": doc, "required_pages": []string{"/items", "/done"}, "negative_controls": []string{"empty-hidden"}, "ordered_events": events}}
	variations := []any{map[string]any{"id": "items-visible", "flow": "items-flow", "preconditions": []string{"seeded-items"}, "actions": []string{"open-items"}, "observable_facts": []string{"items-count-visible"}, "expected_outcomes": []BehaviorAdapterOutcome{{ID: "items-visible", Behavior: "list-items", Matcher: "toHaveText", Locator: "#count", Value: "2"}}, "projects": []string{"alpha"}, "tests": []string{clean.ID}}}
	candidates := []any{map[string]any{"id": "list-items", "evidence": clean.Evidence, "flows": []string{"items-flow"}}}
	test := func(execution BehaviorExecution, title string, contract bool) map[string]any {
		row := map[string]any{"id": execution.ID, "project": execution.Project, "title": title, "evidence": execution.Evidence, "flows": []string{}, "criteria": []string{}, "assertions": []BehaviorAssertion{}, "variation_claims": []BehaviorAdapterTestClaim{}}
		if contract {
			row["flows"], row["criteria"], row["fixtures"] = []string{"items-flow"}, []string{"items-visible"}, []string{"seeded-items"}
			row["assertions"] = []BehaviorAssertion{{ID: "items-visible", Behavior: "list-items", Criterion: "items-visible", Annotation: annotation, Matcher: "toHaveText", Locator: "#count", Value: "2"}}
			row["variation_claims"] = []BehaviorAdapterTestClaim{{VariationID: "items-visible", Preconditions: []string{"seeded-items"}, Actions: []string{"open-items"}, ObservableFacts: []string{"items-count-visible"}}}
		}
		if witness := runtime[execution.ID]; witness != nil {
			row["runtime"] = witness
		}
		return row
	}
	tests := []any{
		test(clean, "lists items", true),
		test(execution("alpha", 17), "rejects empty item", false),
		test(execution("alpha", 21), "archives item", false),
		test(execution("alpha", 23), "retries item", false),
		test(execution("beta", 7), "lists items", false),
	}
	for _, extra := range r.extraTests {
		found := execution("alpha", extra.line)
		row := test(BehaviorExecution{ID: extra.id, Project: "alpha", Evidence: found.Evidence}, extra.title, false)
		row["fixtures"] = extra.fixtures
		tests = append(tests, row)
	}
	discoveryRaw := behaviorAdapterRaw(t, discovery)
	documents := map[string][]byte{
		"flows": behaviorAdapterRaw(t, map[string]any{"items": flows}), "variations": behaviorAdapterRaw(t, map[string]any{"items": variations}),
		"candidates": behaviorAdapterRaw(t, map[string]any{"items": candidates}), "tests": behaviorAdapterRaw(t, map[string]any{"items": tests}),
		"migration": r.migration, "discovery": discoveryRaw, "receipt": r.receipt,
	}
	kinds := map[string]string{"flows": "review", "variations": "review", "candidates": "declared", "tests": "declared", "migration": "declared", "discovery": "observed", "receipt": "observed"}
	inputs := append(r.commitInputs(t, documents, kinds), extra...)
	fields := func(names ...string) map[string]string {
		out := map[string]string{}
		for _, name := range names {
			out[name] = "/" + name
		}
		return out
	}
	return BehaviorAdapterRequest{
		Schema: BehaviorAdapterRequestSchema, ProviderID: "behavior", ProviderVersion: "1", ContractID: "items-contract",
		Source: r.revisions.E2E, Revisions: r.revisions, SourceRevision: r.revision, DocumentationRevision: r.revision,
		MigrationInput: "migration", DiscoveryInput: "discovery", Inputs: inputs, Observations: observations,
		Mappings: []BehaviorAdapterMapping{
			{Kind: "flows", Input: "flows", Records: "/items", Fields: fields("id", "derivation", "evidence", "required_pages", "negative_controls", "ordered_events")},
			{Kind: "variations", Input: "variations", Records: "/items", Fields: fields("id", "flow", "preconditions", "actions", "observable_facts", "expected_outcomes", "projects", "tests")},
			{Kind: "candidates", Input: "candidates", Records: "/items", Fields: fields("id", "evidence", "flows")},
			{Kind: "tests", Input: "tests", Records: "/items", Fields: fields("id", "project", "title", "evidence", "flows", "criteria", "assertions", "variation_claims", "runtime", "fixtures", "roles")},
		},
	}
}

func itoa(value int) string {
	raw, _ := jsonstd.Marshal(value)
	return string(raw)
}

func playwrightCoverage(result BehaviorAdapterResult, metric string) BehaviorAdapterCoverage {
	for _, row := range result.Coverage {
		if row.Metric == metric {
			return row
		}
	}
	return BehaviorAdapterCoverage{}
}

func playwrightFrontierKinds(result BehaviorAdapterResult, subject string) []string {
	kinds := []string{}
	for _, diagnostic := range result.Frontier {
		if diagnostic.Subject == subject {
			kinds = append(kinds, diagnostic.Kind)
		}
	}
	return kinds
}

func importPlaywright(t *testing.T, request BehaviorAdapterRequest, receiptInput string, report []byte) PlaywrightWitnessImport {
	t.Helper()
	raw, err := ImportPlaywrightWitnesses(behaviorAdapterRaw(t, request), receiptInput, report)
	if err != nil {
		t.Fatal(err)
	}
	var bundle PlaywrightWitnessImport
	if err := decode(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func playwrightReasons(bundle PlaywrightWitnessImport) map[string]string {
	reasons := map[string]string{}
	for _, entry := range bundle.Unwitnessed {
		reasons[entry.TestID] = entry.Reason
	}
	return reasons
}

func editPlaywrightJSON(t *testing.T, raw []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := jsonstd.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	edit(value)
	out, err := jsonstd.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// editPlaywrightReportTest edits the report entry for one spec title and project.
func editPlaywrightReportTest(t *testing.T, raw []byte, title, project string, edit func(spec, test map[string]any)) []byte {
	t.Helper()
	return editPlaywrightJSON(t, raw, func(report map[string]any) {
		var walk func(suites []any)
		walk = func(suites []any) {
			for _, suite := range suites {
				suite := suite.(map[string]any)
				for _, spec := range asSlice(suite["specs"]) {
					spec := spec.(map[string]any)
					if spec["title"] != title {
						continue
					}
					for _, test := range asSlice(spec["tests"]) {
						if test := test.(map[string]any); test["projectName"] == project {
							edit(spec, test)
						}
					}
				}
				walk(asSlice(suite["suites"]))
			}
		}
		walk(asSlice(report["suites"]))
	})
}

func asSlice(value any) []any {
	values, _ := value.([]any)
	return values
}

func TestPlaywrightDiscoveryProducerRoundTrip(t *testing.T) {
	t.Run("DCP-V1-046 DCP-V1-047 canonical live discovery accepted by behavior-adapter", func(t *testing.T) {
		repo := newPlaywrightBehaviorRepo(t)
		discovery := repo.discover(t)
		if discovery.Schema != PlaywrightDiscoverySchema || discovery.Mode != PlaywrightDiscoveryMode || discovery.Revisions != repo.revisions || len(discovery.Executions) != 8 {
			t.Fatalf("discovery identity: %+v", discovery)
		}
		config := discovery.Config
		configBytes, _ := os.ReadFile(filepath.Join(repo.root, playwrightBehaviorConfig))
		if config.Path != playwrightBehaviorConfig || config.Revision != repo.revision || config.Repository != repo.repository || config.Kind != "observed" || config.Start != 1 || config.SHA256 != Digest(configBytes) || config.Blob != git(t, repo.root, "rev-parse", repo.revision+":"+playwrightBehaviorConfig) {
			t.Fatalf("config anchor: %+v", config)
		}
		adopted := 0
		for _, execution := range discovery.Executions {
			if execution.Evidence.Path != playwrightBehaviorSpec || execution.Evidence.Revision != repo.revision || execution.Evidence.Start != execution.Evidence.End || execution.Evidence.Kind != "observed" {
				t.Fatalf("execution anchor: %+v", execution)
			}
			if slices.Contains(mapValues(repo.receiptIDs), execution.ID) {
				adopted++
				if execution.Project != "alpha" {
					t.Fatalf("receipt identity adopted for a foreign project: %+v", execution)
				}
			} else if !strings.HasPrefix(execution.ID, "playwright:") || execution.Project != "beta" {
				t.Fatalf("unmatched execution identity: %+v", execution)
			}
		}
		if adopted != 4 {
			t.Fatalf("adopted %d receipt identities, want 4", adopted)
		}
		request := repo.request(t, discovery, nil, nil, nil)
		result := buildBehaviorAdapter(t, request, nil)
		clean := repo.receiptIDs["lists items"]
		if kinds := playwrightFrontierKinds(result, clean); slices.Contains(kinds, "unregistered-test-execution") || slices.Contains(kinds, "stale-anchor") || !slices.Contains(kinds, "missing-runtime-witness") {
			t.Fatalf("clean test frontier: %v", kinds)
		}
		if row := playwrightCoverage(result, "discovered_project_executions"); row.Value != 5 || row.Denominator != 8 {
			t.Fatalf("discovered executions: %+v", row)
		}
		if row := playwrightCoverage(result, "linked_contracts"); row.Value != 1 || row.Denominator != 1 {
			t.Fatalf("linked contracts: %+v frontier=%+v", row, result.Frontier)
		}
		if row := playwrightCoverage(result, "runtime_witnessed_contracts"); row.Value != 0 {
			t.Fatalf("runtime witnessed without a witness: %+v", row)
		}
	})
	t.Run("DCP-V1-046 without a receipt keeps Playwright test identities", func(t *testing.T) {
		repo := newPlaywrightBehaviorRepo(t)
		repo.receipt = nil
		discovery := repo.discover(t)
		for _, execution := range discovery.Executions {
			if !strings.HasPrefix(execution.ID, "playwright:") {
				t.Fatalf("identity without a receipt: %+v", execution)
			}
		}
	})
}

func mapValues(values map[string]string) []string {
	out := []string{}
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func TestPlaywrightDiscoveryProducerRefusals(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, repo *playwrightBehaviorRepo) (configPath string)
		want string
	}{
		{"filtered", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.listing = editPlaywrightJSON(t, repo.listing, func(v map[string]any) {
				config := v["config"].(map[string]any)
				config["argv"] = append(asSlice(config["argv"]), "--grep", "items")
			})
			return playwrightBehaviorConfig
		}, "listing refused"},
		{"sharded", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.listing = editPlaywrightJSON(t, repo.listing, func(v map[string]any) {
				v["config"].(map[string]any)["shard"] = map[string]any{"current": 1, "total": 2}
			})
			return playwrightBehaviorConfig
		}, "sharded"},
		{"errored", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.listing = editPlaywrightJSON(t, repo.listing, func(v map[string]any) {
				v["errors"] = []any{map[string]any{"message": "syntax error"}}
			})
			return playwrightBehaviorConfig
		}, "error"},
		{"foreign config", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.listing = editPlaywrightJSON(t, repo.listing, func(v map[string]any) {
				v["config"].(map[string]any)["configFile"] = repo.root + "/other.config.ts"
			})
			return playwrightBehaviorConfig
		}, "config"},
		{"out of root", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.listing = editPlaywrightJSON(t, repo.listing, func(v map[string]any) {
				v["config"].(map[string]any)["rootDir"] = "/elsewhere/e2e"
			})
			return playwrightBehaviorConfig
		}, "outside"},
		{"dirty working tree", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			writePlaywrightBehaviorFile(t, repo.root, playwrightBehaviorSpec, []byte("// changed after commit\n"))
			return playwrightBehaviorConfig
		}, "working-tree"},
		{"untracked config", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			writePlaywrightBehaviorFile(t, repo.root, "untracked.config.ts", []byte("export default {};\n"))
			repo.listing = editPlaywrightJSON(t, repo.listing, func(v map[string]any) {
				v["config"].(map[string]any)["configFile"] = repo.root + "/untracked.config.ts"
			})
			return "untracked.config.ts"
		}, "not a regular Git blob"},
		{"foreign repository", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			revisions := repo.revisions
			revisions.E2E.ID = strings.Repeat("3", 40)
			repo.migration = behaviorAdapterRaw(t, BehaviorMigration{Revisions: revisions, Schema: 2, ContractID: "items-contract", SourceRevision: repo.revision, DocumentationRevision: repo.revision})
			return playwrightBehaviorConfig
		}, "source repository"},
		{"receipt config differs", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			files := map[string][]byte{playwrightBehaviorConfig: []byte("export default {};\n"), playwrightBehaviorSpec: playwrightBehaviorTestdata(t, "items.spec.ts", repo.root)}
			repo.receipt, _ = playwrightBehaviorReceipt(t, repo.root, files)
			return playwrightBehaviorConfig
		}, "different config"},
		{"documentation revision is not an object id", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.migration = behaviorAdapterRaw(t, BehaviorMigration{Revisions: repo.revisions, Schema: 2, ContractID: "items-contract", SourceRevision: repo.revision, DocumentationRevision: "not-a-git-revision"})
			return playwrightBehaviorConfig
		}, "schema-2 migration"},
		{"documentation revision disagrees with the docs corpus revision", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.migration = behaviorAdapterRaw(t, BehaviorMigration{Revisions: repo.revisions, Schema: 2, ContractID: "items-contract", SourceRevision: repo.revision, DocumentationRevision: strings.Repeat("4", 40)})
			return playwrightBehaviorConfig
		}, "schema-2 migration"},
		{"receipt not qualified", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			repo.receipt = []byte(`{"tests":[]}`)
			return playwrightBehaviorConfig
		}, "qualified"},
		{"stale listing omits a committed test file", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			commitPlaywrightBehaviorTest(t, repo, "e2e/extra.spec.ts")
			return playwrightBehaviorConfig
		}, `omits project "alpha" test e2e/extra.spec.ts`},
		{"listing omits a test file at the revision absent from the working tree", func(t *testing.T, repo *playwrightBehaviorRepo) string {
			commitPlaywrightBehaviorTest(t, repo, "e2e/extra.spec.ts")
			if err := os.Remove(filepath.Join(repo.root, "e2e", "extra.spec.ts")); err != nil {
				t.Fatal(err)
			}
			return playwrightBehaviorConfig
		}, "selects at the source revision"},
	}
	for _, c := range cases {
		t.Run("DCP-V1-048 "+c.name, func(t *testing.T) {
			repo := newPlaywrightBehaviorRepo(t)
			configPath := c.edit(t, &repo)
			out, err := BuildPlaywrightDiscovery(context.Background(), PlaywrightDiscoveryInput{Root: repo.root, Migration: repo.migration, ConfigPath: configPath, Listing: repo.listing, Receipt: repo.receipt})
			t.Log(err)
			if err == nil || !strings.Contains(err.Error(), c.want) || out != nil {
				t.Fatalf("got %v with %d output bytes, want refusal containing %q and no output", err, len(out), c.want)
			}
		})
	}
}

func TestPlaywrightWitnessImporter(t *testing.T) {
	t.Run("DCP-V1-049 DCP-V1-050 clean run counts toward runtime_witnessed_contracts", func(t *testing.T) {
		repo := newPlaywrightBehaviorRepo(t)
		discovery := repo.discover(t)
		request := repo.request(t, discovery, nil, nil, nil)
		bundle := importPlaywright(t, request, "receipt", repo.report)
		clean := repo.receiptIDs["lists items"]
		if bundle.Schema != PlaywrightWitnessImportSchema || bundle.ReportSHA256 != Digest(repo.report) || bundle.Receipt.SHA256 != Digest(repo.receipt) || len(bundle.Witnesses) != 1 || bundle.Witnesses[0].TestID != clean {
			t.Fatalf("bundle: %+v", bundle)
		}
		witness := bundle.Witnesses[0]
		var run BehaviorRun
		if err := decode([]byte(witness.Document), &run); err != nil || witness.SHA256 != Digest([]byte(witness.Document)) || run.RunSHA256 != Digest(repo.receipt) || witness.ReportSHA256 != Digest(repo.report) || run.Project != "alpha" || run.Retry != 0 || len(run.Events) != 4 || !slices.Equal(run.Fixtures, []string{"seeded-items"}) {
			t.Fatalf("witness: %+v run=%+v err=%v", witness, run, err)
		}
		reasons := playwrightReasons(bundle)
		want := map[string]string{repo.receiptIDs["rejects empty item"]: "failed", repo.receiptIDs["archives item"]: "skipped", repo.receiptIDs["retries item"]: "flaky-after-retry"}
		for id, reason := range want {
			if reasons[id] != reason {
				t.Fatalf("reason for %s = %q, want %q (%v)", id, reasons[id], reason, reasons)
			}
		}
		beta := 0
		for id, reason := range reasons {
			if strings.HasPrefix(id, "playwright:") {
				beta++
				if reason != "receipt-test-missing" {
					t.Fatalf("beta reason %q", reason)
				}
			}
		}
		if beta != 1 || len(reasons) != 4 {
			t.Fatalf("unwitnessed: %v", reasons)
		}
		inputs := repo.commitInputs(t, map[string][]byte{"run": []byte(witness.Document)}, map[string]string{"run": "observed"})
		runtime := map[string]*BehaviorRuntime{clean: {Evidence: inputs[0].Anchor, Observation: witness.Observation.ID}}
		wired := repo.request(t, discovery, runtime, []ObservationLink{witness.Observation}, inputs)
		result := buildBehaviorAdapter(t, wired, nil)
		if row := playwrightCoverage(result, "runtime_witnessed_contracts"); row.Value != 1 || row.Denominator != 1 {
			t.Fatalf("runtime witnessed: %+v frontier=%+v", row, playwrightFrontierKinds(result, clean))
		}
		if result.Provider.BehaviorContracts.ContractSHA256 != bundle.ContractSHA256 {
			t.Fatal("contract digest changed after wiring the witness")
		}
	})
	t.Run("DCP-V1-051 DCP-V1-052 unclean evidence stays unwitnessed with a named reason", func(t *testing.T) {
		repo := newPlaywrightBehaviorRepo(t)
		discovery := repo.discover(t)
		request := repo.request(t, discovery, nil, nil, nil)
		clean := repo.receiptIDs["lists items"]
		annotations := func(edit func([]any) []any) func(spec, test map[string]any) {
			return func(_, test map[string]any) {
				result := asSlice(test["results"])[0].(map[string]any)
				result["annotations"] = edit(asSlice(result["annotations"]))
			}
		}
		without := func(kind string) func([]any) []any {
			return func(values []any) []any {
				return slices.DeleteFunc(values, func(v any) bool { return v.(map[string]any)["type"] == kind })
			}
		}
		cases := []struct {
			name   string
			report []byte
			want   string
		}{
			{"report test missing", editPlaywrightReportTest(t, editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(spec, _ map[string]any) { spec["title"] = "renamed" }), "lists items", "beta", func(spec, _ map[string]any) { spec["title"] = "renamed" }), "report-test-missing"},
			{"report ran another project only", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(spec, _ map[string]any) { spec["title"] = "renamed" }), "foreign-project"},
			{"report foreign project", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) { test["projectName"] = "gamma" }), "foreign-project"},
			{"report disagrees", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) { test["status"] = "unexpected" }), "report-receipt-disagree"},
			{"report flaky", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) { test["status"] = "flaky" }), "flaky-after-retry"},
			{"report skipped", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) { test["status"] = "skipped" }), "skipped"},
			{"events unobserved", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(without(PlaywrightEventAnnotation))), "events-unobserved"},
			{"event malformed", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(func(v []any) []any {
				return append(v, map[string]any{"type": PlaywrightEventAnnotation, "description": `{"sequence":5,"unknown":true}`})
			})), "event-malformed"},
			{"cleanup unobserved", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(without(PlaywrightCleanupAnnotation))), "cleanup-unobserved"},
			{"cleanup failed", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(func(v []any) []any {
				return append(without(PlaywrightCleanupAnnotation)(v), map[string]any{"type": PlaywrightCleanupAnnotation, "description": "failed"})
			})), "cleanup-failed"},
			{"fixture mismatch", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(without(PlaywrightFixtureAnnotation))), "fixture-role-mismatch"},
			{"report retry absent", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) {
				delete(asSlice(test["results"])[0].(map[string]any), "retry")
			}), "report-test-missing"},
			{"report retry null", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) {
				asSlice(test["results"])[0].(map[string]any)["retry"] = nil
			}), "report-test-missing"},
			{"event empty", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(func(v []any) []any {
				return append(without(PlaywrightEventAnnotation)(v), map[string]any{"type": PlaywrightEventAnnotation, "description": "{}"})
			})), "event-malformed"},
			{"events reversed", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(func(v []any) []any {
				events := slices.DeleteFunc(slices.Clone(v), func(a any) bool { return a.(map[string]any)["type"] != PlaywrightEventAnnotation })
				slices.Reverse(events)
				return append(without(PlaywrightEventAnnotation)(v), events...)
			})), "event-malformed"},
			{"event duplicate", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(func(v []any) []any {
				return append(v, playwrightEventAnnotation(t, v, 0, func(event map[string]any) { event["sequence"] = 5 }))
			})), "event-malformed"},
			{"event not passing", editPlaywrightReportTest(t, repo.report, "lists items", "alpha", annotations(func(v []any) []any {
				replaced := playwrightEventAnnotation(t, v, 3, func(event map[string]any) { event["passed"] = false })
				events := slices.DeleteFunc(slices.Clone(v), func(a any) bool { return a.(map[string]any)["type"] != PlaywrightEventAnnotation })
				return append(without(PlaywrightEventAnnotation)(v), append(events[:3:3], replaced)...)
			})), "event-malformed"},
			{"report foreign config", editPlaywrightJSON(t, repo.report, func(v map[string]any) { v["config"].(map[string]any)["configFile"] = repo.root + "/other.config.ts" }), "report-foreign-config"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				bundle := importPlaywright(t, request, "receipt", c.report)
				if len(bundle.Witnesses) != 0 || playwrightReasons(bundle)[clean] != c.want {
					t.Fatalf("witnesses=%d reason=%q want %q", len(bundle.Witnesses), playwrightReasons(bundle)[clean], c.want)
				}
			})
		}
		t.Run("receipt foreign project", func(t *testing.T) {
			var envelope struct {
				Receipt jstestprovider.Receipt `json:"receipt"`
			}
			if err := jsonstd.Unmarshal(repo.receipt, &envelope); err != nil {
				t.Fatal(err)
			}
			receipt := envelope.Receipt
			for index := range receipt.Tests {
				if receipt.Tests[index].ID == clean {
					receipt.Tests[index].Project.Name = "gamma"
				}
			}
			raw, err := jstestprovider.EncodeQualified(receipt)
			if err != nil {
				t.Fatal(err)
			}
			repo := repo
			repo.receipt = raw
			bundle := importPlaywright(t, repo.request(t, discovery, nil, nil, nil), "receipt", repo.report)
			if len(bundle.Witnesses) != 0 || playwrightReasons(bundle)[clean] != "foreign-project" {
				t.Fatalf("bundle: %+v", bundle)
			}
		})
		t.Run("one report result witnesses only the receipt test with its full title", func(t *testing.T) {
			repo := repo
			repo.retained = maps.Clone(repo.retained)
			repo.receipt, repo.receiptIDs = playwrightBehaviorReceipt(t, repo.root, playwrightBehaviorFiles(t, repo.root), playwrightReceiptCase{"group-b", "lists items", 7, "passed", []jstestprovider.Attempt{{State: "passed", Retry: 0}}})
			other := repo.receiptIDs["group-b > lists items"]
			repo.extraTests = []playwrightExtraTest{{id: other, title: "lists items", line: 7, fixtures: []string{"seeded-items"}}}
			bundle := importPlaywright(t, repo.request(t, discovery, nil, nil, nil), "receipt", repo.report)
			if len(bundle.Witnesses) != 1 || bundle.Witnesses[0].TestID != clean || playwrightReasons(bundle)[other] != "report-test-missing" {
				t.Fatalf("witnesses=%+v reasons=%v", bundle.Witnesses, playwrightReasons(bundle))
			}
		})
		t.Run("receipt outcomes sharing a full title stay unwitnessed", func(t *testing.T) {
			repo := repo
			repo.retained = maps.Clone(repo.retained)
			repo.receipt, _ = playwrightBehaviorReceipt(t, repo.root, playwrightBehaviorFiles(t, repo.root), playwrightReceiptCase{"items", "lists items", 7, "passed", []jstestprovider.Attempt{{State: "passed", Retry: 0}}})
			bundle := importPlaywright(t, repo.request(t, discovery, nil, nil, nil), "receipt", repo.report)
			if len(bundle.Witnesses) != 0 || playwrightReasons(bundle)[clean] != "report-ambiguous" {
				t.Fatalf("witnesses=%+v reasons=%v", bundle.Witnesses, playwrightReasons(bundle))
			}
		})
		t.Run("receipt unqualified", func(t *testing.T) {
			bundle := importPlaywright(t, request, "candidates", repo.report)
			if len(bundle.Witnesses) != 0 || playwrightReasons(bundle)[clean] != "receipt-unqualified" {
				t.Fatalf("bundle: %+v", bundle)
			}
		})
		t.Run("plain report without a receipt input is refused", func(t *testing.T) {
			if _, err := ImportPlaywrightWitnesses(behaviorAdapterRaw(t, request), "absent", repo.report); err == nil {
				t.Fatal("missing receipt input accepted")
			}
			if _, err := ImportPlaywrightWitnesses(behaviorAdapterRaw(t, request), "discovery", repo.report); err == nil {
				t.Fatal("discovery input accepted as a receipt")
			}
		})
	})
}

// playwrightEventAnnotation returns a copy of the index-th event annotation with its event edited.
func playwrightEventAnnotation(t *testing.T, annotations []any, index int, edit func(map[string]any)) map[string]any {
	t.Helper()
	events := slices.DeleteFunc(slices.Clone(annotations), func(a any) bool { return a.(map[string]any)["type"] != PlaywrightEventAnnotation })
	var event map[string]any
	if err := jsonstd.Unmarshal([]byte(events[index].(map[string]any)["description"].(string)), &event); err != nil {
		t.Fatal(err)
	}
	edit(event)
	raw, err := jsonstd.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"type": PlaywrightEventAnnotation, "description": string(raw)}
}

// playwrightBehaviorFiles reads the committed config and spec bytes the receipt digests.
func playwrightBehaviorFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, path := range []string{playwrightBehaviorConfig, playwrightBehaviorSpec} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	return files
}
