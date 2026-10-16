package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-057, DCW-V0-058, CEM-V1-170: a change that adds only evidence
// records binds with its build-log entry as the intent and a uniform plan that
// cites a BASE decision, passes the check, seals, and its sealed map reads
// class record under `cem status --citation`.
func TestDogfoodDCWV0058RecordChangeBindsChecksAndSeals(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, _ := portableDogfoodRepo(t)
	cemWrite(t, root, "docs/decisions/0001-fixture.md", "# Fixture decision\nThe answer is two.\nRecord the evidence.\n")
	cemGit(t, root, "add", "docs/decisions")
	cemGit(t, root, "commit", "-qm", "decision")
	base := cemGit(t, root, "rev-parse", "HEAD")
	entry := "docs/build-log/2026-10-13-fixture.md"
	cemWrite(t, root, entry, "# Fixture record\nThe answer is two.\n")
	cemWrite(t, root, ".agent-evidence/fixture/run.txt", "go test ./fixture: ok\n")
	cemGit(t, root, "add", "docs/build-log", ".agent-evidence")
	cemGit(t, root, "commit", "-qm", "record")
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	for path, content := range map[string]string{
		citations: "1\tdocs/decisions/0001-fixture.md\t2:3\tdecision\n2\tdocs/decisions/0001-fixture.md\t2:3\tdecision\n",
		intents:   entry + "\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []string{"DOGFOOD_TASK=Record the fixture.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 1 || strings.Contains(stderr, "citation-plan-uniform") || strings.Contains(stderr, "invalid-requirements-section") {
		t.Fatalf("first pass exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	report, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	for _, want := range []string{
		`"complete": true`,
		`{"name": "cem-cite", "status": "PRODUCED", "reason": "none"}`,
		"\n  \"ocmStatus\": {\"state\": \"NOT_ASSESSED\", \"reason\": \"no-intent-declared\"}\n",
	} {
		if code != 0 || stdout != "" || dropGuardingNote(stderr) != absentAgentReceipts || !strings.Contains(string(report), want) {
			t.Fatalf("change exit=%d stderr=%s want %s in report=%s", code, stderr, want, report)
		}
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "check", base)
	if code != 0 || !strings.HasPrefix(stdout, "dogfood-check: SUMMARY cem=accepted hunks=2 supported=2 unknown=0") || !strings.HasSuffix(stdout, "\ndogfood-check: PASS\n") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "seal", base)
	sealed := ".corvint/changes/" + bind + ".cem.json"
	if code != 0 || !strings.HasSuffix(stdout, "dogfood-seal: PASS sealed="+sealed+"\n") {
		t.Fatalf("seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	code, stdout, stderr = run.exec(t, root, nil, "cem", "status", "--map", sealed, "--expected-base", base, "--target", bind, "--artifacts", root, "--citation")
	if code != 0 || !strings.Contains(stdout, `"citationClass":{"class":"record","role":"change"}`) {
		t.Fatalf("status exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
