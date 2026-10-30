package testplan

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// TestRealProviderDocumentsNeverReuse drives every provider document a real run can emit through
// Run, not a synthetic projection: in V0 no real document yields a CURRENT, passing, JavaScript e2e
// witness, so REUSED stays synthetic-only and every witness is rejected with a named reason
// (TCN-V0-004, TCN-V0-016, V1-1031).
func TestRealProviderDocumentsNeverReuse(t *testing.T) {
	const spec = "test('books', () => {})\n"
	root := gitRepo(t, map[string]string{"tests/e2e/book.spec.ts": spec})
	bound := filepath.Join(root, "tests", "e2e", "book.spec.ts")

	// The only JavaScript receipts that may carry a test id and project are the qualified external
	// profiles; their app lifecycle is never verifiable, so freshness stays UNKNOWN even when the
	// bound spec digest matches the evaluated revision.
	qualified, err := jstestprovider.EncodeQualified(jstestprovider.Receipt{
		Profile: jstestprovider.ExternalProfile, Kind: "e2e",
		External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "none",
			ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true},
		Identity: jstestprovider.Identity{TestFileDigests: map[string]string{bound: digestOf(spec)}},
		Tests: []jstestprovider.TestOutcome{{ID: "book-1", Project: &jstestprovider.ProjectIdentity{Name: "chromium"},
			Name: "books a slot", FullName: "books a slot", State: jstestprovider.StatePassed,
			Anchor: &jstestprovider.Anchor{File: "tests/e2e/book.spec.ts", Line: 1}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	qualifiedPath := writeTemp(t, "qualified.json", string(qualified))
	evidence, err := gather(context.Background(), Request{Root: root, Tests: []string{qualifiedPath}}, decode(t, inputJSON(t, variation("V1"))))
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if got := evidence.Documents[0].Tests[0].Projection.Freshness; got.State != testvalidity.FreshnessUnknown ||
		got.Reason != "retained-external-app-lifecycle-unverifiable" {
		t.Fatalf("qualified receipt freshness %s/%s", got.State, got.Reason)
	}

	// A completed Go session event projects tests without ids, so no witness can name one.
	goSession, _ := json.Marshal(map[string]any{"profile": "corvint-go-live-session-event/0", "state": "passed", "identity": "abc",
		"sequence": 1, "scope": []any{}, "detail": "", "projection": map[string]any{},
		"testProjections":        []any{map[string]any{"package": "example.com/app", "name": "TestBook", "action": "pass", "projection": map[string]any{}}},
		"testProjectionsOmitted": 0})
	goPath := writeTemp(t, "go-session.json", string(goSession))

	in := inputJSON(t,
		variation("V1", set("witnesses", []any{map[string]any{"test_id": "book-1", "project": "chromium", "basis": "declared"}})),
		variation("V2", set("witnesses", []any{map[string]any{"test_id": "TestBook", "basis": "declared"}})),
	)
	p, err := Run(context.Background(), Request{Root: root, Input: in, Tests: []string{qualifiedPath, goPath}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(p.Reused) != 0 || p.Counts.Reused != 0 {
		t.Fatalf("a real provider document reached REUSED: %+v", p.Reused)
	}
	// The qualified fixture is rejected on its first failing axis; its UNKNOWN freshness, checked
	// above, would reject it even with every live prerequisite met.
	if len(p.WitnessRejections) != 2 || p.WitnessRejections[0].VariationID != "V1" || p.WitnessRejections[0].Reason == "" ||
		p.WitnessRejections[1].VariationID != "V2" || p.WitnessRejections[1].Reason != WitnessNotFound {
		t.Fatalf("rejections %+v", p.WitnessRejections)
	}

	// An unprofiled receipt cannot carry a test id at all, and the freshness profile is not a
	// test-validity input: Run refuses both before planning.
	refused := map[string]string{
		"unprofiled with an id": `{"receipt":{"kind":"e2e","tests":[{"id":"book-1","name":"books a slot","state":"passed"}]}}`,
		"freshness profile":     `{"receipt":{"profile":"` + jstestprovider.FreshnessProfile + `","kind":"e2e","tests":[]}}`,
	}
	for name, raw := range refused {
		path := writeTemp(t, "refused.json", raw)
		if _, err := Run(context.Background(), Request{Root: root, Input: in, Tests: []string{path}}); code(err) != "invalid-test-validity-receipt" {
			t.Errorf("%s: %v", name, err)
		}
	}
}
