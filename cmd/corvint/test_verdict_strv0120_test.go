package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// TestSTRV0120DryRunVerdictAgreesAndNamesTheDeclaration pins V1-3530: a
// repository whose declaration cannot be read plans scope FULL that cannot
// run; the human verdict line and the dry-run JSON verdict both say
// INCOMPLETE, name the missing declaration, and the run exits 3, which the
// help documents. Since RNC-V1-013 only Corvint's own repository without
// .corvint/test-commands.json reads .github/cishards/test-commands.json, so
// the fixture declares Corvint's module and lacks both files.
func TestSTRV0120DryRunVerdictAgreesAndNamesTheDeclaration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/Beamfall/corvint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	declared := sessiontest.ReadDeclared(root)
	if declared.Unreadable == "" {
		t.Fatal("an empty repository's declaration reads as readable")
	}
	m, err := sessiontest.Build(sessiontest.Input{Declared: declared, ShardCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	var human bytes.Buffer
	testVerdict(&human, m)
	wantHuman := "corvint-test: VERDICT INCOMPLETE: scope FULL is planned but cannot run complete (the declared full scope is unreadable: .github/cishards/test-commands.json: "
	if !strings.Contains(human.String(), wantHuman) || !strings.Contains(human.String(), "exit 3 is INCOMPLETE, not FAIL\n") {
		t.Fatalf("human verdict:\n%s\nwant %q", &human, wantHuman)
	}
	var stdout, stderr bytes.Buffer
	code := testDryRun(m, "sha256:abc", declared, root, testOptions{shardCount: 1}, testinventory.Verdict{}, &stdout, &stderr)
	var document struct {
		Scope      string   `json:"scope"`
		Verdict    string   `json:"verdict"`
		Incomplete []string `json:"incomplete"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("dry-run JSON: %v\n%s", err, &stdout)
	}
	if code != 3 || document.Scope != "FULL" || document.Verdict != "INCOMPLETE" ||
		len(document.Incomplete) == 0 || !strings.Contains(document.Incomplete[0], sessiontest.DeclarationPath) {
		t.Fatalf("exit=%d document=%+v stderr=%s", code, document, &stderr)
	}
	if testPlanVerdict(sessiontest.Manifest{Scope: "NARROWED"}) != "PLANNED" {
		t.Fatal("a complete plan is not PLANNED")
	}
	if !strings.Contains(testHelp, "A --dry-run exits 0 when its plan can run complete (verdict PLANNED) and 3\nwhen it cannot (verdict INCOMPLETE") {
		t.Fatal("the test help does not document the dry-run exit 3")
	}
}
