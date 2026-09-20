package doccorpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func behaviorFixture(t *testing.T, edit func(*BehaviorRegistry)) (string, Manifest) {
	return behaviorFixtureWithRun(t, edit, false)
}
func behaviorFixtureWithRun(t *testing.T, edit func(*BehaviorRegistry), runtime bool, runtimeEdits ...func(*BehaviorRun)) (string, Manifest) {
	t.Helper()
	root, m := fixture(t)
	retain := func(path string, value any, purpose string) Anchor {
		data, err := Encode(value)
		if raw, ok := value.([]byte); ok {
			data = raw
			err = nil
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", path)
		git(t, root, "commit", "-qm", "synthetic behavior evidence")
		rev := git(t, root, "rev-parse", "HEAD")
		blob := git(t, root, "rev-parse", rev+":"+path)
		m.Scopes = append(m.Scopes, Scope{path, rev})
		m.Inputs = append(m.Inputs, Input{path, rev, blob, Digest(data), "behavior", purpose})
		return Anchor{Repository: m.Repository.ID, Revision: rev, Path: path, Blob: blob, SHA256: Digest(data), Start: 1, End: 1, SpanSHA256: Digest(data), Authority: "external-provider", Kind: "declared", Reason: "synthetic fixture; not observed consumer input"}
	}
	r := BehaviorRegistry{Schema: 2, ContractID: "migration", SourceRevision: m.Repository.Revision, DocumentationRevision: m.Repository.Revision}
	r.Revisions = BehaviorRevisions{App: Repository{strings.Repeat("1", 40), m.Repository.Revision}, E2E: m.Repository, Docs: Repository{strings.Repeat("2", 40), m.Repository.Revision}}
	expected := r.Revisions
	m.BehaviorRevisions = &expected
	r.Manifest = retain("docs/migrations/manifest.json", BehaviorMigration{Revisions: r.Revisions, Schema: 2, ContractID: r.ContractID, SourceRevision: r.SourceRevision, DocumentationRevision: r.DocumentationRevision}, "evidence")
	r.Flows = []BehaviorFlow{{ID: "summary", Evidence: declaredEvidence(t, root, m, "src/readme.md", 3).Anchors[0], Criteria: []string{"count-visible"}, Tests: []string{"project:count"}, RequiredPages: []string{"/summary"}, NegativeControls: []string{"wrong-count"}}}
	annotation := declaredEvidence(t, root, m, "src/view.ts", 1).Anchors[0]
	annotation.Kind = "review"
	r.Tests = []BehaviorTest{{ID: "project:count", Project: "chromium", Title: "count", Evidence: declaredEvidence(t, root, m, "src/view.ts", 1).Anchors[0], Flows: []string{"summary"}, Criteria: []string{"count-visible"}, Assertions: []BehaviorAssertion{{ID: "count-visible", Behavior: "show-count", Criterion: "count-visible", Annotation: annotation, Matcher: "toHaveText", Locator: "#count", Value: "1"}}}}
	r.Behaviors = []BehaviorSource{{ID: "show-count", Evidence: declaredEvidence(t, root, m, "src/view.ts", 1).Anchors[0], Flows: []string{"summary"}}}
	r.Flows[0].OrderedEvents = behaviorEvents("count-visible")
	r.Flows[0].Derivation = "generated"
	if edit != nil {
		edit(&r)
	}
	var native jstestprovider.Receipt
	if runtime {
		config := declaredEvidence(t, root, m, "src/value.go", 1).Anchors[0]
		native = jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", Identity: jstestprovider.Identity{ConfigFile: "/repo/config.cjs", ConfigDigest: config.SHA256, ConfigInputDigests: map[string]string{"/repo/config.cjs": config.SHA256}, TestFileDigests: map[string]string{"/repo/test.ts": r.Tests[0].Evidence.SHA256}, RunnerName: "playwright", RunnerVersion: jstestprovider.QualifiedPlaywrightVersion, NodeVersion: "v22", Argv: []string{"playwright", "test"}}, External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ReadyURL: "http://127.0.0.1:3000", DeclaredAppIdentity: "synthetic", ConfigOverride: "controlled"}, AppBuildAtStart: jstestprovider.AppBuildIdentity{Unknown: true}, AppBuildAtPublish: jstestprovider.AppBuildIdentity{Unknown: true}}
		outcome := jstestprovider.TestOutcome{Name: r.Tests[0].Title, FullName: "summary count", State: "passed", Anchor: &jstestprovider.Anchor{File: "/repo/test.ts", Line: 1}, Project: &jstestprovider.ProjectIdentity{Name: r.Tests[0].Project, Browser: "chromium", Device: "unknown", Use: json.RawMessage(`{}`), ConfigDigest: config.SHA256}, Attempts: []jstestprovider.Attempt{{State: "passed", Retry: 0}}}
		identityBytes, err := json.Marshal(struct {
			Identity jstestprovider.Identity
			Project  *jstestprovider.ProjectIdentity
			Anchor   *jstestprovider.Anchor
			FullName string
		}{native.Identity, outcome.Project, outcome.Anchor, outcome.FullName})
		if err != nil {
			t.Fatal(err)
		}
		outcome.ID = Digest(identityBytes)
		native.Tests = []jstestprovider.TestOutcome{outcome}
		r.Tests[0].ID = outcome.ID
		r.Flows[0].Tests = []string{outcome.ID}
	}
	discovery := BehaviorDiscovery{Schema: "corvint-playwright-discovery/1", Mode: "live-playwright-list", Revisions: r.Revisions, Config: declaredEvidence(t, root, m, "src/value.go", 1).Anchors[0]}
	for _, test := range r.Tests {
		discovery.Executions = append(discovery.Executions, BehaviorExecution{test.ID, test.Project, test.Evidence})
	}
	if runtime {
		discovery.Executions = append(discovery.Executions, BehaviorExecution{"discovered:unjoined", "webkit", r.Tests[0].Evidence})
	}
	r.Discovery = retain("evidence/discovery.json", discovery, "evidence")
	r.Discovery.Kind = "observed"
	r.ContractSHA256 = hashValue(r)
	p := ProviderRecord{Schema: BehaviorProviderSchema, ID: "behavior", Version: "1", Source: m.Repository, BehaviorContracts: &r}
	if runtime {
		nativeBytes, err := jstestprovider.EncodeQualified(native)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := testvaliditydoc.Decode(nativeBytes); err != nil {
			t.Fatal(err)
		}
		nativeAnchor := retain("evidence/native.json", nativeBytes, "observation")
		witness := BehaviorRun{Revisions: r.Revisions, Schema: "corvint-behavior-run/1", ContractID: r.ContractID, ContractSHA256: r.ContractSHA256, SourceRevision: r.SourceRevision, DocumentationRevision: r.DocumentationRevision, RunSHA256: nativeAnchor.SHA256, TestID: r.Tests[0].ID, Project: r.Tests[0].Project, Cleanup: "passed", Events: behaviorEvents("count-visible")}
		for _, edit := range runtimeEdits {
			edit(&witness)
		}
		witnessAnchor := retain("evidence/journey.json", witness, "evidence")
		witnessAnchor.Kind = "observed"
		r.Tests[0].Runtime = &BehaviorRuntime{Evidence: witnessAnchor, Observation: "behavior:run"}
		evidence := declaredEvidence(t, root, m, "src/view.ts", 1)
		p.Subjects = []Subject{{ID: "behavior:test", Kind: "test", Name: r.Tests[0].Title, Provider: "behavior", Evidence: evidence}}
		p.Capabilities = []CapabilityDeclaration{{"subjects", "present", "synthetic test"}, {"observations", "present", "synthetic qualified retained receipt"}}
		p.Observations = []ObservationLink{{ID: "behavior:run", Subject: "behavior:test", Input: nativeAnchor.Path, InputRevision: nativeAnchor.Revision, SourceRevision: r.SourceRevision, RunID: nativeAnchor.SHA256, Test: r.Tests[0].Title, TestID: r.Tests[0].ID, Project: r.Tests[0].Project, SourcePaths: map[string]string{"/repo/config.cjs": "src/value.go", "/repo/test.ts": "src/view.ts"}}}
	}
	a := retain("docs/migrations/test-behavior-contracts.json", p, "provider")
	m.Providers = append(m.Providers, Provider{"behavior", "records", "1", a.Revision, a.Path})
	return root, m
}

func behaviorEvents(assertion string) []BehaviorEvent {
	return []BehaviorEvent{
		{Sequence: 1, Kind: "page", ID: "/summary", Passed: true, Context: "context-1", Page: "page-1", Frame: "main", Navigation: "main-frame"},
		{Sequence: 2, Kind: "assertion", ID: assertion, Passed: true, Context: "context-1", Page: "page-1", Frame: "main", Behavior: "show-count", Criterion: assertion, Matcher: "toHaveText", Locator: "#count", Value: "1"},
		{Sequence: 3, Kind: "negative-control", ID: "wrong-count", Passed: true, Context: "context-1", Page: "page-1", Frame: "main"},
		{Sequence: 4, Kind: "page", ID: "/done", Passed: true, Context: "context-1", Page: "page-1", Frame: "main", Navigation: "main-frame"},
	}
}

func TestBehaviorContractCorpusRoundTrip(t *testing.T) {
	root, m := behaviorFixture(t, nil)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.BehaviorContracts) != 1 || len(a.BehaviorContracts[0].LinkedFlows) != 1 || len(a.BehaviorContracts[0].VerifiedFlows) != 0 {
		t.Fatalf("invalid declaration/verification split: %+v", a.BehaviorContracts)
	}
	raw, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), root, raw); err != nil {
		t.Fatal(err)
	}
	rows := behaviorCoverage(a)
	if len(rows) != 4 {
		t.Fatal(rows)
	}
	for _, row := range rows {
		if row.(map[string]any)["denominator"] != 1 {
			t.Fatal(row)
		}
	}
	if Impact(a, []string{"src/view.ts"}, "fresh")["selection"].(map[string]any)["narrowing_allowed"] != false {
		t.Fatal("narrowed suite")
	}
}

func TestBehaviorQualifiedReceiptEndToEnd(t *testing.T) {
	t.Run("DCP-V1-007 retained project identity", func(t *testing.T) {
		root, m := behaviorFixtureWithRun(t, nil, true)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.BehaviorContracts[0].VerifiedFlows) != 1 {
			t.Fatalf("retained qualified witness not verified: %+v %+v", a.BehaviorContracts, a.Observations)
		}
		if a.Observations[0].Document.Run.Freshness.State != "UNKNOWN" || a.Observations[0].Document.Run.Execution.State == "PASSED" {
			t.Fatal("native unknown axes promoted")
		}
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Open(context.Background(), root, data); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBehaviorContractGaps(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		edit       func(*BehaviorRegistry)
	}{
		{"reverse", "missing-reverse-link", func(r *BehaviorRegistry) { r.Flows[0].Tests = nil }},
		{"assertions", "assertion-free-ui-test", func(r *BehaviorRegistry) { r.Tests[0].Assertions = nil }},
		{"criterion", "contradiction", func(r *BehaviorRegistry) { r.Tests[0].Criteria = []string{"unrelated"} }},
		{"empty", "unreviewed-join", func(r *BehaviorRegistry) { r.Tests = nil; r.Flows[0].Tests = nil }},
		{"review", "confirmed-missing_e2e", func(r *BehaviorRegistry) {
			r.Flows[0].Tests = nil
			a := r.Flows[0].Evidence
			a.Kind = "review"
			r.Flows[0].MissingReview = &a
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, m := behaviorFixture(t, tc.edit)
			a, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, gap := range a.Gaps {
				if gap.Kind == tc.kind {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s: %+v", tc.kind, a.Gaps)
			}
			if len(a.BehaviorContracts[0].VerifiedFlows) != 0 {
				t.Fatal("invalid join verified")
			}
		})
	}
}

func TestBehaviorOrderedRuntime(t *testing.T) {
	// Synthetic already-projected native observation exercises only the join boundary.
	// The round-trip test separately exercises immutable provider admission.
	for _, change := range []string{"valid", "wrong-project", "retry", "cleanup", "stale-docs", "wrong-contract", "order", "page", "negative", "assertion", "wrong-run", "unreadable-source", "other-unknown", "incomplete", "unqualified-test", "stale"} {
		t.Run(change, func(t *testing.T) {
			r := BehaviorRegistry{ContractID: "contract", ContractSHA256: Digest([]byte("contract")), SourceRevision: "source", DocumentationRevision: "docs", Flows: []BehaviorFlow{{ID: "flow", RequiredPages: []string{"/summary"}, NegativeControls: []string{"wrong-count"}}}}
			test := BehaviorTest{ID: "exact-id", Project: "chromium", Title: "count", Flows: []string{"flow"}, Assertions: []BehaviorAssertion{{ID: "count", Behavior: "show-count", Criterion: "count", Matcher: "toHaveText", Locator: "#count", Value: "1"}}, Runtime: &BehaviorRuntime{Observation: "run"}}
			r.Flows[0].OrderedEvents = behaviorEvents("count")
			test.Evidence = Anchor{Path: "test.ts", SHA256: "test-digest", Start: 1, End: 1}
			run := BehaviorRun{Schema: "corvint-behavior-run/1", ContractID: r.ContractID, ContractSHA256: r.ContractSHA256, SourceRevision: r.SourceRevision, DocumentationRevision: r.DocumentationRevision, RunSHA256: "receipt", TestID: test.ID, Project: test.Project, Cleanup: "passed", Events: behaviorEvents("count")}
			switch change {
			case "wrong-project":
				run.Project = "webkit"
			case "retry":
				run.Retry = 1
			case "cleanup":
				run.Cleanup = "unknown"
			case "stale-docs":
				run.DocumentationRevision = "old"
			case "wrong-contract":
				run.ContractSHA256 = "old"
			case "order":
				run.Events[0].Sequence = 2
			case "page":
				run.Events[0].ID = "/elsewhere"
			case "negative":
				run.Events[2].ID = "other"
			case "assertion":
				run.Events[1].ID = "other"
			case "wrong-run":
				run.RunSHA256 = "other"
			}
			data, _ := Encode(run)
			test.Runtime.Evidence = Anchor{Revision: "runtime", Path: "run.json", Start: 1, SpanSHA256: Digest(data), Kind: "observed"}
			projection := testvalidity.Projection{}
			projection.Execution.State = "PASSED"
			projection.Freshness.State = "CURRENT"
			o := Observation{Link: ObservationLink{ID: "run", SourceRevision: "source"}, InputSHA256: "receipt", Document: testvaliditydoc.Document{Playwright: &jstestprovider.Receipt{}, Run: projection, Tests: []testvaliditydoc.Test{{ID: test.ID, Name: test.Title, Project: &jstestprovider.ProjectIdentity{Name: test.Project}, State: "passed", Attempts: []jstestprovider.Attempt{{State: "passed", Retry: 0}}}}}}
			o.Link.TestID = test.ID
			o.Link.Project = test.Project
			o.Document.Playwright.Tests = []jstestprovider.TestOutcome{{ID: test.ID, Anchor: &jstestprovider.Anchor{File: "test.ts", Line: 1}}}
			test.Evidence.SHA256 = Digest([]byte("test"))
			o.Document.Tests[0].Projection.Execution.State = "PASSED"
			o.Document.Playwright.Identity.TestFileDigests = map[string]string{"test.ts": test.Evidence.SHA256}
			o.Document.Playwright.Identity.ConfigFile = "test.ts"
			o.Document.Playwright.Identity.ConfigDigest = test.Evidence.SHA256
			c := &compiler{sources: map[string]contextindex.Source{inputKey("runtime", "run.json"): {Data: data}, inputKey("source", "test.ts"): {Data: []byte("test")}}, artifact: &Artifact{Observations: []Observation{o}}}
			discoveryBytes, _ := Encode(BehaviorDiscovery{Config: Anchor{Path: "test.ts", SHA256: test.Evidence.SHA256}})
			c.sources[inputKey("", "")] = contextindex.Source{Data: discoveryBytes}
			switch change {
			case "unreadable-source":
				delete(c.sources, inputKey("source", "test.ts"))
			case "other-unknown":
				c.artifact.Observations[0].Document.Run.Freshness.State = "UNKNOWN"
			case "incomplete":
				c.artifact.Observations[0].Document.Run.Execution.State = "INCOMPLETE"
			case "unqualified-test":
				c.artifact.Observations[0].Document.Tests[0].Projection.Execution.State = "UNKNOWN"
			case "stale":
				c.artifact.Observations[0].Document.Run.Freshness.State = "STALE"
			}
			if got := c.behaviorRunVerified(r, test); got != (change == "valid") {
				t.Fatalf("%s verified=%v", change, got)
			}
			if slices.Contains([]string{"page", "negative"}, change) && len(c.artifact.Gaps) == 0 {
				t.Fatal("missing runtime obligation gap")
			}
		})
	}
}

func TestBehaviorExactProjectObservation(t *testing.T) {
	link := ObservationLink{Test: "same title", TestID: "test-a", Project: "chromium"}
	test := testvaliditydoc.Test{Name: "same title", ID: "test-a", Project: &jstestprovider.ProjectIdentity{Name: "chromium"}}
	if !observationMatches(test, link) {
		t.Fatal("exact join refused")
	}
	test.Project.Name = "webkit"
	if observationMatches(test, link) {
		t.Fatal("cross-project title joined")
	}
}

func TestBehaviorAcceptanceAmendment(t *testing.T) {
	t.Run("DCP-V1-004 noncurrent behavior anchor", func(t *testing.T) {
		root, m := behaviorFixtureWithRun(t, func(r *BehaviorRegistry) { r.Behaviors[0].Evidence.Revision = r.Manifest.Revision }, true)
		var source Input
		var otherRevision string
		for _, input := range m.Inputs {
			if input.Path == "src/view.ts" {
				source = input
			}
			if input.Path == "docs/migrations/manifest.json" {
				otherRevision = input.Revision
			}
		}
		source.Revision = otherRevision
		source.Provider = "behavior"
		source.Purpose = "evidence"
		m.Inputs = append(m.Inputs, source)
		m.Scopes = append(m.Scopes, Scope{source.Path, otherRevision})
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.BehaviorContracts[0].VerifiedTests) > 0 {
			t.Fatal("digest-valid noncurrent behavior anchor verified")
		}
	})
	t.Run("DCP-V1-004 every assertion is reviewed and declared", func(t *testing.T) {
		for _, kind := range []string{"unreviewed", "unknown-behavior", "undeclared-criterion"} {
			t.Run(kind, func(t *testing.T) {
				extraEvent := behaviorEvents("count-visible")[1]
				extraEvent.ID = "extra"
				extraEvent.Sequence = 5
				root, m := behaviorFixtureWithRun(t, func(r *BehaviorRegistry) {
					a := r.Tests[0].Assertions[0]
					a.ID = "extra"
					switch kind {
					case "unreviewed":
						a.Annotation.Kind = "declared"
					case "unknown-behavior":
						a.Behavior = "unknown"
						extraEvent.Behavior = "unknown"
					case "undeclared-criterion":
						a.Criterion = "unknown"
						extraEvent.Criterion = "unknown"
					}
					r.Tests[0].Assertions = append(r.Tests[0].Assertions, a)
					r.Flows[0].OrderedEvents = append(r.Flows[0].OrderedEvents, extraEvent)
				}, true, func(r *BehaviorRun) { r.Events = append(r.Events, extraEvent) })
				a, err := Build(context.Background(), root, m)
				if err != nil {
					t.Fatal(err)
				}
				if len(a.BehaviorContracts[0].VerifiedTests) > 0 {
					t.Fatal("extra invalid assertion verified despite one good assertion")
				}
			})
		}
	})
	t.Run("DCP-V1-008 exact assertion and page identities", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			edit func(*BehaviorRun)
		}{
			{"same matcher wrong element", func(r *BehaviorRun) { r.Events[1].Locator = "#other" }},
			{"same matcher wrong value", func(r *BehaviorRun) { r.Events[1].Value = "2" }},
			{"same route missing assertion", func(r *BehaviorRun) {
				r.Events = append(r.Events[:1], r.Events[2:]...)
				for i := range r.Events {
					r.Events[i].Sequence = i + 1
				}
			}},
			{"right assertion wrong project", func(r *BehaviorRun) { r.Project = "webkit" }},
			{"out of order page visit", func(r *BehaviorRun) {
				r.Events[0], r.Events[3] = r.Events[3], r.Events[0]
				for i := range r.Events {
					r.Events[i].Sequence = i + 1
				}
			}},
			{"wrong browser context", func(r *BehaviorRun) { r.Events[0].Context = "other" }},
			{"wrong page", func(r *BehaviorRun) { r.Events[0].Page = "popup" }},
			{"frame navigation", func(r *BehaviorRun) { r.Events[0].Navigation = "frame"; r.Events[0].Frame = "child" }},
			{"redirect", func(r *BehaviorRun) { r.Events[0].Navigation = "redirect" }},
			{"popup", func(r *BehaviorRun) { r.Events[0].Navigation = "popup"; r.Events[0].ParentPage = "parent" }},
			{"setup", func(r *BehaviorRun) { r.Events[0].Navigation = "setup" }},
			{"stale docs revision", func(r *BehaviorRun) { r.Revisions.Docs.Revision = strings.Repeat("3", 40) }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root, m := behaviorFixtureWithRun(t, nil, true, tc.edit)
				a, err := Build(context.Background(), root, m)
				if err != nil {
					t.Fatal(err)
				}
				if len(a.BehaviorContracts[0].VerifiedTests) > 0 {
					t.Fatal("mismatched runtime verified")
				}
			})
		}
	})
	t.Run("DCP-V1-010 discovery denominator and generated provenance", func(t *testing.T) {
		root, m := behaviorFixtureWithRun(t, nil, true)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if a.BehaviorContracts[0].Registry.Flows[0].Derivation != "generated" {
			t.Fatal("prose promoted")
		}
		for _, raw := range behaviorCoverage(a) {
			row := raw.(map[string]any)
			if row["metric"] == "discovered_playwright_project_executions" && (row["denominator"] != 2 || row["value"] != 1) {
				t.Fatal(row)
			}
		}
		found := false
		for _, gap := range a.Gaps {
			if gap.Subject == "discovered:unjoined" && gap.Kind == "unreviewed-join" {
				found = true
			}
		}
		if !found {
			t.Fatal("unjoined live discovery execution hidden")
		}
		for _, member := range []string{"app", "e2e", "docs"} {
			t.Run(member, func(t *testing.T) {
				expected := *m.BehaviorRevisions
				switch member {
				case "app":
					expected.App.Revision = strings.Repeat("4", 40)
				case "e2e":
					expected.E2E.Revision = strings.Repeat("4", 40)
				case "docs":
					expected.Docs.Revision = strings.Repeat("4", 40)
				}
				copy := m
				copy.BehaviorRevisions = &expected
				stale, err := Build(context.Background(), root, copy)
				if err != nil {
					t.Fatal(err)
				}
				if len(stale.BehaviorContracts[0].VerifiedTests) > 0 {
					t.Fatal("stale revision member verified")
				}
			})
		}
	})
}
