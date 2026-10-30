package contextindex

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

var trustClasses = []string{TrustProjectAuthority, TrustRepositoryContent, TrustRepositoryHistory, TrustExternalProvider, TrustToolOutput}

// TestTrustClassIsClosedAndDeterministic: every listed label maps to one of
// the five classes, an unlisted label is tool-output, and only the fetched
// and tool-produced classes are tainted (TCP-V0-023).
func TestTrustClassIsClosedAndDeterministic(t *testing.T) {
	t.Parallel()
	for authority, class := range trustByAuthority {
		if !slices.Contains(trustClasses, class) {
			t.Fatalf("%s maps to %q, outside the enum", authority, class)
		}
		if got := TrustClass(authority); got != class {
			t.Fatalf("TrustClass(%s) = %q, want %q", authority, got, class)
		}
	}
	cases := map[string]string{
		"project-instructions": TrustProjectAuthority, "repository-spec": TrustProjectAuthority,
		SyntaxAuthority: TrustRepositoryContent, "vocabulary": TrustRepositoryContent,
		"git-history": TrustRepositoryHistory, "external-provider": TrustExternalProvider,
		UnverifiedLedgerAuthority: TrustToolOutput, "": TrustToolOutput, "fetched-web-page": TrustToolOutput,
	}
	for authority, want := range cases {
		if got := TrustClass(authority); got != want {
			t.Fatalf("TrustClass(%q) = %q, want %q", authority, got, want)
		}
	}
	for _, class := range trustClasses {
		want := class == TrustExternalProvider || class == TrustToolOutput
		if TrustTainted(class) != want {
			t.Fatalf("TrustTainted(%s) = %v, want %v", class, !want, want)
		}
	}
}

func trustFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":              "module example.test/trust\n\ngo 1.27.0\n",
		"AGENTS.md":           "Standing instructions.\n",
		"cache/demux.go":      "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go": "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"server/server.go":    "package server\n\nimport \"example.test/trust/cache\"\n\nfunc Run() string { return cache.Split(\"k\") }\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// TestTaskContextRowsCarryOneTrustClass: every evidence row of a packet
// carries exactly one `trust` value, equal to the class its own authority
// derives, and the governing row is project authority.
func TestTaskContextRowsCarryOneTrustClass(t *testing.T) {
	t.Parallel()
	index := trustFixture(t)
	packet, err := TaskContext(context.Background(), index, "Reviewer asks whether `Split` handles empty keys", "cache/demux.go", 20)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	if len(results) < 3 {
		t.Fatalf("want a governing, pair and importer row, got %v", results)
	}
	for _, result := range results {
		for _, row := range mapsFromAny(result["evidence"]) {
			class, ok := row["trust"].(string)
			if !ok || !slices.Contains(trustClasses, class) {
				t.Fatalf("row %v carries trust %v, want one of %v", row, row["trust"], trustClasses)
			}
			if class != TrustClass(row["authority"].(string)) {
				t.Fatalf("row %v: trust %s is not derived from authority %v", row, class, row["authority"])
			}
			if result["kind"] == governingRelation && class != TrustProjectAuthority {
				t.Fatalf("governing row %v is %s, want %s", row, class, TrustProjectAuthority)
			}
		}
	}
	coverage := packet["coverage"].(map[string]any)
	if refused := coverage["governance_refused"].([]any); len(refused) != 0 {
		t.Fatalf("governance_refused = %v, want none", refused)
	}
}

// TestTaskContextGovernanceRefusesATaintedReservedRow: a reserved row whose
// authority derives a tainted class satisfies neither the governance receipt
// nor the critical selectors, and `governance_refused` names it (TCP-V0-023).
func TestTaskContextGovernanceRefusesATaintedReservedRow(t *testing.T) {
	t.Parallel()
	index := trustFixture(t)
	compiler := newTaskContextCompiler(index, "Reconcile the ledger", "")
	tainted := contextRow{kind: governingRelation, path: "AGENTS.md", authority: UnverifiedLedgerAuthority, score: 1000}
	compiler.reserved = []contextRow{tainted}
	if got := compiler.governance(); got != "unresolved" {
		t.Fatalf("governance = %q, want unresolved", got)
	}
	carried, missing := compiler.criticalSelectors([]contextRow{tainted})
	if len(carried) != 0 || len(missing) != 0 {
		t.Fatalf("critical = %v, critical_missing = %v, want neither to carry the tainted row", carried, missing)
	}
	refused := compiler.governanceRefused()
	if len(refused) != 1 {
		t.Fatalf("governance_refused = %v, want one entry", refused)
	}
	entry := refused[0].(map[string]any)
	if entry["relation"] != governingRelation || entry["path"] != "AGENTS.md" || entry["trust"] != TrustToolOutput {
		t.Fatalf("refusal %v does not name the row", entry)
	}
	compiler.reserved = []contextRow{{kind: governingRelation, path: "AGENTS.md", authority: "project-instructions", score: 1000}}
	if got := compiler.governance(); got != "reserved" || len(compiler.governanceRefused()) != 0 {
		t.Fatalf("project-authority row: governance = %q, refused = %v", got, compiler.governanceRefused())
	}
}

// oldContextEvidence is the evidence row shape before `trust` was added.
type oldContextEvidence struct {
	Authority   string `json:"authority"`
	BlobHash    string `json:"blob_hash"`
	Confidence  string `json:"confidence"`
	EvidenceGap string `json:"evidence_gap,omitempty"`
	Line        int    `json:"line"`
	Path        string `json:"path"`
	Reason      string `json:"reason"`
}

// TestTaskContextWireIsAdditiveForAnOldConsumer: a consumer decoding the
// previous evidence-row shape reads today's wire unchanged, and the wire minus
// the new members is exactly what that consumer re-encodes.
func TestTaskContextWireIsAdditiveForAnOldConsumer(t *testing.T) {
	t.Parallel()
	index := trustFixture(t)
	packet, err := TaskContext(context.Background(), index, "Reviewer asks whether `Split` handles empty keys", "cache/demux.go", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range mapsFromAny(packet["results"]) {
		wire, err := json.Marshal(result["evidence"])
		if err != nil {
			t.Fatal(err)
		}
		var old []oldContextEvidence
		if err := json.Unmarshal(wire, &old); err != nil {
			t.Fatalf("old consumer cannot decode %s: %v", wire, err)
		}
		stripped := []any{}
		for _, row := range mapsFromAny(result["evidence"]) {
			delete(row, "trust")
			stripped = append(stripped, row)
		}
		want, _ := gokernel.CanonicalJSON(stripped)
		got, _ := gokernel.CanonicalJSON(old)
		if string(got) != string(want) {
			t.Fatalf("old consumer sees a changed row:\n%s\n%s", got, want)
		}
	}
	coverage := packet["coverage"].(map[string]any)
	if _, ok := coverage["governance_refused"]; !ok {
		t.Fatalf("coverage lacks governance_refused: %v", coverage)
	}
}
