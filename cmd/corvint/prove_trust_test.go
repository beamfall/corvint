package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

var proveTrustClasses = []string{
	contextindex.TrustProjectAuthority, contextindex.TrustRepositoryContent, contextindex.TrustRepositoryHistory,
	contextindex.TrustExternalProvider, contextindex.TrustToolOutput,
}

// oldProveRow is the proof row shape before `trust` and `refusal` were added.
type oldProveRow struct {
	Authority string                `json:"authority"`
	BlobHash  string                `json:"blob_hash"`
	Falsified string                `json:"falsified"`
	Falsifier string                `json:"falsifier"`
	Kind      string                `json:"kind"`
	Line      int                   `json:"line"`
	Path      string                `json:"path"`
	Result    string                `json:"result"`
	Detail    string                `json:"detail,omitempty"`
	Witness   *proveMutationWitness `json:"witness,omitempty"`
}

// TestProveRowsCarryOneTrustClassAndOldConsumersDecode: every proof row of a
// real proof carries one `trust` value derived from its authority, none is
// refused, and a consumer of the previous row shape decodes the wire to
// exactly the wire minus the new members (FPK-V0-032).
func TestProveRowsCarryOneTrustClassAndOldConsumersDecode(t *testing.T) {
	t.Parallel()
	root := proveFixtureRepository(t)
	receipt, _, stderr, code := runProveCLI(t, root, "--task", authorityStartPrompt, "--limit", "3")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	rows := proofRows(t, receipt)
	if len(rows) == 0 {
		t.Fatal("no proof rows")
	}
	for _, row := range rows {
		class, ok := row["trust"].(string)
		if !ok || !slices.Contains(proveTrustClasses, class) {
			t.Fatalf("row %v carries trust %v, want one of %v", row, row["trust"], proveTrustClasses)
		}
		if class != contextindex.TrustClass(row["authority"].(string)) {
			t.Fatalf("row %v: trust %s is not derived from authority %v", row, class, row["authority"])
		}
		if _, refused := row["refusal"]; refused {
			t.Fatalf("row %v refused on a repository-only packet", row)
		}
	}
	wire, err := json.Marshal(receipt["proof"].(map[string]any)["rows"])
	if err != nil {
		t.Fatal(err)
	}
	var old []oldProveRow
	if err := json.Unmarshal(wire, &old); err != nil {
		t.Fatalf("old consumer cannot decode %s: %v", wire, err)
	}
	stripped := []any{}
	for _, row := range rows {
		delete(row, "trust")
		delete(row, "refusal")
		stripped = append(stripped, row)
	}
	want, _ := gokernel.CanonicalJSON(stripped)
	got, _ := gokernel.CanonicalJSON(old)
	if string(got) != string(want) {
		t.Fatalf("old consumer sees changed rows:\n%s\n%s", got, want)
	}
}

// TestProveRefusesATaintedRowAsBasis: a packet row whose authority derives a
// tainted class keeps falsifier none whatever the tables say, so it can never
// be proven, and its refusal names the row; a repository row is untouched.
func TestProveRefusesATaintedRowAsBasis(t *testing.T) {
	t.Parallel()
	packet := map[string]any{"results": []any{
		map[string]any{"kind": "learned-path", "id": "pkg/a.go", "evidence": []any{map[string]any{
			"path": "pkg/a.go", "line": 3, "blob_hash": "x", "authority": contextindex.UnverifiedLedgerAuthority, "reason": "learned",
		}}},
		map[string]any{"kind": "reverse-import", "id": "pkg/b.go", "evidence": []any{map[string]any{
			"path": "pkg/b.go", "line": 1, "blob_hash": "y", "authority": "syntax", "reason": "imports",
		}}},
		map[string]any{"kind": "fetched", "id": "https://example.test", "evidence": []any{map[string]any{
			"path": "docs/x.md", "line": 1, "blob_hash": "z", "authority": "web-page", "reason": "fetched",
		}}},
	}}
	rows := assignFalsifiers(packet)
	if len(rows) != 3 {
		t.Fatalf("rows = %v", rows)
	}
	tainted := rows[0]
	if tainted.Trust != contextindex.TrustToolOutput || tainted.Falsifier != falsifierNone {
		t.Fatalf("tainted row = %+v, want tool-output with falsifier none", tainted)
	}
	for _, part := range []string{"tool-output", "learned-path", "pkg/a.go:3", "cannot satisfy a basis"} {
		if !strings.Contains(tainted.Refusal, part) {
			t.Fatalf("refusal %q does not name %q", tainted.Refusal, part)
		}
	}
	clean := rows[1]
	if clean.Trust != contextindex.TrustRepositoryContent || clean.Refusal != "" || clean.Falsifier == falsifierNone {
		t.Fatalf("repository row = %+v, want repository-content, no refusal, a falsifier", clean)
	}
	unlisted := rows[2]
	if unlisted.Trust != contextindex.TrustToolOutput || unlisted.Falsifier != falsifierNone || unlisted.Refusal == "" {
		t.Fatalf("unlisted authority row = %+v, want tool-output and refused", unlisted)
	}
	for index := range rows {
		rows[index].Falsified = falsifiedPass
	}
	summary := summarizeProof(packet, rows)
	if summary.ProvenResults != 1 || summary.UnprovenResults != 2 {
		t.Fatalf("a PASS on a tainted row counted as proven: %+v", summary)
	}
	if verdict := resultVerdict("learned-path", "pkg/a.go", rows); verdict != falsifiedNotRun {
		t.Fatalf("tainted result verdict = %q, want %q", verdict, falsifiedNotRun)
	}
	row := affectedRow("pkg/a_test.go", "pkg/a.go")
	if row.Trust != contextindex.TrustRepositoryContent || row.Refusal != "" {
		t.Fatalf("affected row = %+v, want repository-content and no refusal", row)
	}
}
