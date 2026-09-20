package doccorpus

import (
	"context"
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
	t.Helper()
	root, m := fixture(t)
	retain := func(path string, value any, purpose string) Anchor {
		data, err := Encode(value)
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
	r.ContractSHA256 = hashValue(r)
	p := ProviderRecord{Schema: BehaviorProviderSchema, ID: "behavior", Version: "1", Source: m.Repository, BehaviorContracts: &r}
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
	for _, change := range []string{"valid", "wrong-project", "retry", "cleanup", "stale-docs", "wrong-contract", "order", "page", "negative", "assertion", "wrong-run"} {
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
			o.Document.Playwright.Identity.TestFileDigests = map[string]string{"test.ts": "test-digest"}
			c := &compiler{sources: map[string]contextindex.Source{inputKey("runtime", "run.json"): {Data: data}}, artifact: &Artifact{Observations: []Observation{o}}}
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
