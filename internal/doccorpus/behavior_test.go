package doccorpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func behaviorFixture(t *testing.T, edit func(*BehaviorRegistry)) (string, Manifest) {
	return behaviorFixtureWithRun(t, edit, false)
}
func behaviorFixtureWithRun(t *testing.T, edit func(*BehaviorRegistry), runtime bool) (string, Manifest) {
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
	r.Manifest = retain("docs/migrations/manifest.json", BehaviorMigration{2, r.ContractID, r.SourceRevision, r.DocumentationRevision}, "evidence")
	r.Flows = []BehaviorFlow{{ID: "summary", Evidence: declaredEvidence(t, root, m, "src/readme.md", 3).Anchors[0], Criteria: []string{"count-visible"}, Tests: []string{"project:count"}, RequiredPages: []string{"/summary"}, NegativeControls: []string{"wrong-count"}}}
	r.Tests = []BehaviorTest{{ID: "project:count", Project: "chromium", Title: "count", Evidence: declaredEvidence(t, root, m, "src/view.ts", 1).Anchors[0], Flows: []string{"summary"}, Criteria: []string{"count-visible"}, Assertions: []string{"count-visible"}}}
	r.Behaviors = []BehaviorSource{{ID: "show-count", Evidence: declaredEvidence(t, root, m, "src/view.ts", 1).Anchors[0], Flows: []string{"summary"}}}
	if edit != nil {
		edit(&r)
	}
	r.Flows[0].OrderedEvents = []string{"page:/summary", "assertion:count-visible", "negative-control:wrong-count"}
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
		witness := BehaviorRun{Schema: "corvint-behavior-run/1", ContractID: r.ContractID, ContractSHA256: r.ContractSHA256, SourceRevision: r.SourceRevision, DocumentationRevision: r.DocumentationRevision, RunSHA256: nativeAnchor.SHA256, TestID: r.Tests[0].ID, Project: r.Tests[0].Project, Cleanup: "passed", Events: []BehaviorEvent{{1, "page", "/summary", true}, {2, "assertion", "count-visible", true}, {3, "negative-control", "wrong-count", true}}}
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
			test := BehaviorTest{ID: "exact-id", Project: "chromium", Title: "count", Flows: []string{"flow"}, Assertions: []string{"count"}, Runtime: &BehaviorRuntime{Observation: "run"}}
			r.Flows[0].OrderedEvents = []string{"page:/summary", "assertion:count", "negative-control:wrong-count"}
			test.Evidence = Anchor{Path: "test.ts", SHA256: "test-digest", Start: 1, End: 1}
			run := BehaviorRun{Schema: "corvint-behavior-run/1", ContractID: r.ContractID, ContractSHA256: r.ContractSHA256, SourceRevision: r.SourceRevision, DocumentationRevision: r.DocumentationRevision, RunSHA256: "receipt", TestID: test.ID, Project: test.Project, Cleanup: "passed", Events: []BehaviorEvent{{1, "page", "/summary", true}, {2, "assertion", "count", true}, {3, "negative-control", "wrong-count", true}}}
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
