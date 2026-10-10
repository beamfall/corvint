package doccorpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// behaviorAdapterGuide returns the behavior-adapter guide and its marked
// migration example with the documented placeholder replaced by the /1
// provider member, read by reflection rather than spelled here.
func behaviorAdapterGuide(t *testing.T) (string, []byte) {
	t.Helper()
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "DOCUMENTATION-CORPUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(guide), "<!-- DCP-V1-045 migration example -->\n```json\n")
	example, _, closed := strings.Cut(rest, "```")
	if !found || !closed || strings.Count(example, `"PROVIDER_MEMBER"`) != 1 {
		t.Fatal("guide lacks the marked DCP-V1-045 migration example")
	}
	return string(guide), []byte(strings.Replace(example, `"PROVIDER_MEMBER"`, legacyV1Member(t), 1))
}

// TestBehaviorAdapterGuideMigrationExample proves DCP-V1-045: the guide's
// minimal migration record closed-decodes, and a request that follows the
// guide's anchor placement and later-commit rule is accepted, while inputs
// anchored outside the provider repository are refused as documented.
func TestBehaviorAdapterGuideMigrationExample(t *testing.T) {
	guide, example := behaviorAdapterGuide(t)
	var migration BehaviorMigration
	if err := decode(example, &migration); err != nil {
		t.Fatalf("guide migration example does not decode: %v\n%s", err, example)
	}
	if migration.Schema != 2 || !textOK(migration.ContractID) || !validBehaviorRevisions(migration.Revisions) || migration.SourceRevision != migration.Revisions.E2E.Revision || migration.DocumentationRevision != migration.Revisions.Docs.Revision {
		t.Fatalf("guide migration example is not a minimal schema-2 identity: %+v", migration)
	}
	source := migration.Revisions.E2E
	later := strings.Repeat("4", 40)
	if later == migration.SourceRevision {
		t.Fatal("the later input commit must differ from the described revision")
	}
	discovery := behaviorAdapterRaw(t, BehaviorDiscovery{Schema: "corvint-playwright-discovery/1", Mode: "live-playwright-list", Revisions: migration.Revisions, Executions: []BehaviorExecution{}})
	empty := []byte("{\"items\":[]}\n")
	input := func(id string, raw []byte, kind string) BehaviorAdapterInput {
		digest := Digest(raw)
		return BehaviorAdapterInput{ID: id, Anchor: Anchor{Repository: source.ID, Revision: later, Path: "evidence/" + id + ".json", Blob: strings.Repeat("5", 40), SHA256: digest, Start: 1, End: 1, SpanSHA256: digest, Authority: "external-provider", Kind: kind, Reason: "guide example"}, Document: string(raw)}
	}
	request := BehaviorAdapterRequest{
		Schema: BehaviorAdapterRequestSchema, ProviderID: "behavior", ProviderVersion: "1", ContractID: migration.ContractID,
		Source: source, Revisions: migration.Revisions, SourceRevision: migration.SourceRevision, DocumentationRevision: migration.DocumentationRevision,
		MigrationInput: "migration", DiscoveryInput: "discovery",
		Inputs: []BehaviorAdapterInput{input("migration", example, "declared"), input("discovery", discovery, "observed"), input("flows", empty, "review"), input("variations", empty, "review"), input("candidates", empty, "declared"), input("tests", empty, "declared")},
		Mappings: []BehaviorAdapterMapping{
			{Kind: "flows", Input: "flows", Records: "/items", Fields: map[string]string{"id": "/id", "derivation": "/derivation", "evidence": "/evidence", "required_pages": "/required_pages", "negative_controls": "/negative_controls", "ordered_events": "/ordered_events"}},
			{Kind: "variations", Input: "variations", Records: "/items", Fields: map[string]string{"id": "/id", "flow": "/flow", "preconditions": "/preconditions", "actions": "/actions", "observable_facts": "/observable_facts", "expected_outcomes": "/expected_outcomes", "projects": "/projects", "tests": "/tests"}},
			{Kind: "candidates", Input: "candidates", Records: "/items", Fields: map[string]string{"id": "/id", "evidence": "/evidence", "flows": "/flows"}},
			{Kind: "tests", Input: "tests", Records: "/items", Fields: map[string]string{"id": "/id", "project": "/project", "title": "/title", "evidence": "/evidence", "flows": "/flows", "criteria": "/criteria", "assertions": "/assertions", "variation_claims": "/variation_claims"}},
		},
		Observations: []ObservationLink{},
	}
	if _, err := BuildBehaviorAdapter(behaviorAdapterRaw(t, request), nil); err != nil {
		t.Fatalf("request built from the guide example is refused: %v", err)
	}
	const correction = "supply immutable Git identities from the provider repository"
	if !strings.Contains(strings.Join(strings.Fields(guide), " "), correction) {
		t.Fatal("guide does not state the input anchor placement correction")
	}
	for _, repository := range []string{migration.Revisions.App.ID, migration.Revisions.Docs.ID} {
		misplaced := request
		misplaced.Inputs = append([]BehaviorAdapterInput(nil), request.Inputs...)
		misplaced.Inputs[0].Anchor.Repository = repository
		if _, err := BuildBehaviorAdapter(behaviorAdapterRaw(t, misplaced), nil); err == nil || !strings.Contains(err.Error(), correction) {
			t.Fatalf("input anchored in repository %s: %v", repository, err)
		}
	}
}
