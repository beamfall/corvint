package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// ANL-V0-004: end-to-end map -> ledger -> declaration -> proven requirement, no browser or writes.
func TestANLV0004EdgesCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	var m appmap.Map
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"appmap", "edges", "--map", mapFile, "--full"}
	code, out, diag = runFlowsCLI(root, base...)
	if code != 0 {
		t.Fatal(diag)
	}
	var ledger appmap.EdgeLedger
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || len(ledger.Edges) == 0 {
		t.Fatalf("%s %v", out, err)
	}
	id := ledger.Edges[0].ID
	require := append(append([]string{}, base...), "--require", id)
	code, out, diag = runFlowsCLI(root, require...)
	if code != 1 {
		t.Fatalf("unproven %d %s %s", code, out, diag)
	}
	receipt, key := rvnReceipt(t, m.Revision)
	declarations := filepath.Join(t.TempDir(), "declarations.json")
	// ANL-V0-024: the only prover assuming the edge it selects on is self-proof, so unsatisfied.
	data, _ := json.Marshal(appmap.EdgeDeclarations{Schema: appmap.EdgeDeclarationsSchema, Tests: []appmap.EdgeDeclaration{{TestKey: key, Proves: []string{id}, Assumes: []string{id}}}})
	if err := os.WriteFile(declarations, data, 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, append(base, "--receipt", receipt, "--declarations", declarations)...)
	if code != 1 {
		t.Fatalf("self-proof %d %s %s", code, out, diag)
	}
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || ledger.Proven != 1 || ledger.Satisfied || len(ledger.Unsatisfied) != 1 || ledger.Unsatisfied[0] != id {
		t.Fatal(out, err)
	}
	data, _ = json.Marshal(appmap.EdgeDeclarations{Schema: appmap.EdgeDeclarationsSchema, Tests: []appmap.EdgeDeclaration{{TestKey: key, Proves: []string{id}}, {TestKey: "spec.ts:other", Assumes: []string{id}}}})
	if err := os.WriteFile(declarations, data, 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, append(require, "--receipt", receipt, "--declarations", declarations)...)
	if code != 0 {
		t.Fatalf("proven %d %s %s", code, out, diag)
	}
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || ledger.Proven != 1 || !ledger.Satisfied {
		t.Fatal(out, err)
	}
	code, _, _ = runFlowsCLI(root, append(base, "--declarations", declarations)...)
	if code != 1 {
		t.Fatal("declaration invented proof", code)
	}
	if err := os.WriteFile(declarations, []byte(`{"schema":"application-edge-declarations/0","tests":[],"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, _ = runFlowsCLI(root, append(base, "--declarations", declarations)...)
	if code != 2 {
		t.Fatal("invalid sidecar", code)
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	status, err := cmd.Output()
	if err != nil || len(status) != 0 {
		t.Fatalf("read mutated root %s %v", status, err)
	}
}

// ANL-V0-023: map -> reporter JSON -> observe -> edges --observation proves the observed crossing
// with a receipt and no declaration; the read writes nothing.
func TestANLV0023EdgesObservationCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	var m appmap.Map
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, _ = runFlowsCLI(root, "appmap", "edges", "--map", mapFile, "--full")
	var ledger appmap.EdgeLedger
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || code != 0 {
		t.Fatal(out, err)
	}
	urlOf := func(id string) string {
		for _, s := range m.Screens {
			if s.ID == id && s.Template != "" && !strings.ContainsAny(s.Template, ":*") {
				return "https://x.test" + m.HashPrefix + regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(s.Template, "1")
			}
		}
		return ""
	}
	var edge appmap.LedgerEdge
	for _, e := range ledger.Edges {
		if e.From != e.To && urlOf(e.From) != "" && urlOf(e.To) != "" && e.Trigger == appmap.StatusUnknown {
			edge = e
			break
		}
	}
	if edge.ID == "" {
		t.Fatal("fixture has no distinct-screen UNKNOWN-trigger edge")
	}
	receipt, key := rvnReceipt(t, m.Revision)
	trace := filepath.Join(dir, "events.json")
	body := `{"schema":"playwright-navigation-events/0","test_key":"` + key + `","events":[{"kind":"navigate","url":"` + urlOf(edge.From) + `"},{"kind":"navigate","url":"` + urlOf(edge.To) + `","action":{"kind":"click","selector":"#next"}}]}`
	if err := os.WriteFile(trace, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "observe", "--map", mapFile, "--trace", trace, "--full")
	if code != 0 {
		t.Fatal(diag)
	}
	obsFile := filepath.Join(dir, "obs.json")
	if err := os.WriteFile(obsFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"appmap", "edges", "--map", mapFile, "--full", "--require", edge.ID, "--receipt", receipt}
	if code, _, _ = runFlowsCLI(root, base...); code != 1 {
		t.Fatal("receipt alone proved the edge", code)
	}
	code, out, diag = runFlowsCLI(root, append(base, "--observation", obsFile)...)
	if code != 0 {
		t.Fatalf("%d %s %s", code, out, diag)
	}
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || ledger.Proven != 1 || len(ledger.Observations) != 1 {
		t.Fatal(out, err)
	}
	if code, _, _ = runFlowsCLI(root, append(base, "--observation", mapFile)...); code != 2 {
		t.Fatal("non-observation accepted", code)
	}
	// The count bound refuses before any file is read: none of these paths exist.
	many := append([]string{}, base...)
	for i := 0; i <= appmap.MaxLedgerObservations; i++ {
		many = append(many, "--observation", filepath.Join(dir, "absent", strconv.Itoa(i)))
	}
	if code, _, diag = runFlowsCLI(root, many...); code != 2 || !strings.Contains(diag, "at most 16 --observation") {
		t.Fatal("observation count bound", code, diag)
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("read mutated root %s %v", status, err)
	}
}
