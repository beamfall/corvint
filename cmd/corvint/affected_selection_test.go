package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// affectedSelectionRepository is the affected fixture with a declared gate
// and one committed edit to core/core.go after base, plus a V1 record that
// maps core/core.go to one entity verified by core/core_test.go. The Swift
// sources are removed so the plan scope is bounded: a language frontier keeps the
// selection unknown (ETS-V0-003).
func affectedSelectionRepository(t *testing.T) (root, base, record string) {
	t.Helper()
	root = affectedFixtureRepository(t)
	origin := strings.TrimSpace(affectedGit(t, root, "rev-list", "--max-parents=0", "HEAD"))
	for _, swift := range []string{"Package.swift", "swift"} {
		if err := os.RemoveAll(filepath.Join(root, swift)); err != nil {
			t.Fatal(err)
		}
	}
	writeAffectedGateDeclarations(t, root)
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// committed edit\n")
	affectedGit(t, root, "commit", "-qam", "edit core")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	body := `{"schema":"external-evidence-provider/1","provider":{"id":"mockdocs","revision":"1"},
"repositories":[{"id":"application","origin":"` + origin + `","revision":"` + head + `"}],
"entities":[{"id":"cap-core","kind":"capability","summary":"Core."}],
"relations":[
{"from":{"repository":"application","path":"core/core.go"},"to":{"provider":"mockdocs","entity":"cap-core"},"type":"implements","evidence":"declared","rule":"map","reference":"docs/map.md"},
{"from":{"repository":"application","path":"core/core_test.go"},"to":{"provider":"mockdocs","entity":"cap-core"},"type":"verifies","evidence":"observed","rule":"unit-run","reference":"ci/1"}]}`
	record = filepath.Join(t.TempDir(), "provider.json")
	if err := os.WriteFile(record, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, base, record
}

func appendFile(t *testing.T, path, text string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, []byte(text)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runAffectedArguments(t *testing.T, root string, arguments ...string) (map[string]any, []byte, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root, "affected"}, arguments...), strings.NewReader(""), &stdout, &stderr)
	var receipt map[string]any
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
		}
	}
	return receipt, stdout.Bytes(), stderr.String(), code
}

func testSelection(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	selection, ok := receipt["advice"].(map[string]any)["test_selection"].(map[string]any)
	if !ok {
		t.Fatalf("advice has no test_selection: %v", receipt["advice"])
	}
	return selection
}

// ETS-V0-001, ETS-V0-002, ETS-V0-009: without --provider the receipt has no
// new member; with one, every other member is byte-identical and the
// mandatory checks are echoed, never removed.
func TestAffectedSelectionAddsOneMemberAndKeepsEverythingElse(t *testing.T) {
	t.Parallel()
	root, base, record := affectedSelectionRepository(t)
	plain, plainBytes, stderr, code := runAffectedArguments(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if bytes.Contains(plainBytes, []byte("test_selection")) {
		t.Fatal("a run without --provider must not carry test_selection")
	}
	advised, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selection := testSelection(t, advised)
	delete(advised["advice"].(map[string]any), "test_selection")
	for _, member := range []string{"advice", "plan", "provider", "range"} {
		want, _ := json.Marshal(plain[member])
		got, _ := json.Marshal(advised[member])
		if !bytes.Equal(want, got) {
			t.Fatalf("%s changed with --provider:\n%s\n%s", member, want, got)
		}
	}
	if selection["state"] != "narrow-selection-allowed" || selection["profile"] != "strict" {
		t.Fatalf("a fresh verified committed range must allow narrowing: %v %v", selection["state_reason"], selection["scope"])
	}
	mandatory, _ := json.Marshal(selection["mandatory"])
	var checks []any
	for _, check := range affectedChecks(t, plain) {
		if check["kind"] == "mandatory" {
			checks = append(checks, check)
		}
	}
	if want, _ := json.Marshal(checks); len(checks) == 0 || !bytes.Equal(mandatory, want) {
		t.Fatalf("mandatory %s, want %s", mandatory, want)
	}
	selected := selection["selected"].([]any)
	if len(selected) != 1 || selected[0].(map[string]any)["test"].(map[string]any)["path"] != "core/core_test.go" {
		t.Fatalf("selected=%v", selected)
	}
}

// ETS-V0-003, ETS-V0-006: a language frontier leaves the selection unknown,
// an uncommitted edit is never described by a record, and an unavailable
// record blocks.
func TestAffectedSelectionFailsClosed(t *testing.T) {
	t.Parallel()
	root, base, record := affectedSelectionRepository(t)
	appendFile(t, filepath.Join(root, "core", "core.go"), "// uncommitted\n")
	receipt, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selection := testSelection(t, receipt)
	if selection["state"] != "full-relevant-suite-required" || !bytes.Contains(mustJSON(t, selection["blocking_reasons"]), []byte("worktree-dirty-path")) {
		t.Fatalf("a dirty path must not narrow: %v %v", selection["state"], selection["blocking_reasons"])
	}
	receipt, _, stderr, code = runAffectedArguments(t, root, "--base", base, "--provider", filepath.Join(t.TempDir(), "absent.json"))
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if selection := testSelection(t, receipt); selection["state"] != "blocked" || selection["state_reason"] != "provider-unavailable" {
		t.Fatalf("an unavailable record must block: %v", selection["state"])
	}
	if err := os.WriteFile(filepath.Join(root, "Package.swift"), []byte("// swift-tools-version:5.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code = runAffectedArguments(t, root, "--base", base, "--provider", record)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if selection := testSelection(t, receipt); selection["state"] != "unknown" || selection["state_reason"] != "incomplete-affected-scope" {
		t.Fatalf("an unbounded plan must leave the selection unknown: %v %v", selection["state_reason"], receipt["plan"])
	}
}

// ETS-V1-005..007: a bound checkout's worktree is read through Git; a clean
// one keeps the narrow selection and drops the not-inspected limitation, and
// any dirty path in it widens the selection.
func TestAffectedSelectionInspectsCheckout(t *testing.T) {
	t.Parallel()
	root, base, record := affectedSelectionRepository(t)
	e2e := t.TempDir()
	affectedGit(t, e2e, "init", "-q")
	if err := os.MkdirAll(filepath.Join(e2e, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e2e, "tests", "core.spec.ts"), []byte("test('core')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, e2e, "add", ".")
	affectedGit(t, e2e, "commit", "-qm", "e2e")
	e2eHead := strings.TrimSpace(affectedGit(t, e2e, "rev-parse", "HEAD"))
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.Replace(body, []byte(`"repositories":[`), []byte(`"repositories":[{"id":"e2e","origin":"`+e2eHead+`","revision":"`+e2eHead+`"},`), 1)
	body = bytes.Replace(body, []byte(`"relations":[`), []byte(`"relations":[{"from":{"repository":"e2e","path":"tests/core.spec.ts"},"to":{"provider":"mockdocs","entity":"cap-core"},"type":"verifies","evidence":"observed","rule":"e2e-run","reference":"ci/2"},`), 1)
	if err := os.WriteFile(record, body, 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record, "--repository", "e2e="+e2e)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selection := testSelection(t, receipt)
	if selection["state"] != "narrow-selection-allowed" || bytes.Contains(mustJSON(t, selection["selected"]), []byte("checkout-worktree-not-inspected")) {
		t.Fatalf("a clean inspected checkout must narrow without the not-inspected limitation: %v %s", selection["state"], mustJSON(t, selection))
	}
	appendFile(t, filepath.Join(e2e, "tests", "core.spec.ts"), "// uncommitted\n")
	receipt, _, stderr, code = runAffectedArguments(t, root, "--base", base, "--provider", record, "--repository", "e2e="+e2e)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selection = testSelection(t, receipt)
	if selection["state"] != "full-relevant-suite-required" || !bytes.Contains(mustJSON(t, selection["blocking_reasons"]), []byte("checkout-worktree-dirty")) {
		t.Fatalf("a dirty checkout must widen: %v %v", selection["state"], selection["blocking_reasons"])
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// EEP-TR-001, EEP-TR-005: `affected --provider-command` yields the same
// test_selection as `--provider` over the same bytes, apart from the provider
// row's source, and counts toward the shared provider bound.
func TestAffectedProviderCommandMatchesFile(t *testing.T) {
	t.Parallel()
	root, base, record := affectedSelectionRepository(t)
	fromFile, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	argv, _ := json.Marshal([]string{"/bin/cat", record})
	fromCommand, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider-command", string(argv))
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	fileSelection, commandSelection := testSelection(t, fromFile), testSelection(t, fromCommand)
	fileRows, commandRows := fileSelection["provider_evidence"].([]any), commandSelection["provider_evidence"].([]any)
	if len(fileRows) != 1 || len(commandRows) != 1 {
		t.Fatalf("provider rows: file %d command %d", len(fileRows), len(commandRows))
	}
	fileRow, commandRow := fileRows[0].(map[string]any), commandRows[0].(map[string]any)
	if commandRow["source"] != "command:"+string(argv) || fileRow["source"] != record {
		t.Fatalf("sources: file %v command %v", fileRow["source"], commandRow["source"])
	}
	delete(fileRow, "source")
	delete(commandRow, "source")
	want, _ := json.Marshal(fileSelection)
	got, _ := json.Marshal(commandSelection)
	if !bytes.Equal(want, got) {
		t.Fatalf("test_selection differs by transport:\nfile:    %s\ncommand: %s", want, got)
	}
	arguments := []string{"--base", base}
	for range 4 {
		arguments = append(arguments, "--provider", record)
	}
	arguments = append(arguments, "--provider-command", string(argv))
	if _, _, stderr, code := runAffectedArguments(t, root, arguments...); code != 2 || !strings.Contains(stderr, "at most 4 providers") {
		t.Fatalf("a fifth provider by command must be refused: %d %s", code, stderr)
	}
}

// ETS-V0-001: the new flags are bounded and dependent, and unknown flags
// still fail as before.
func TestAffectedSelectionArguments(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	for _, tc := range []struct {
		arguments []string
		message   string
	}{
		{[]string{"--repository", "e2e=../e2e"}, "require --provider"},
		{[]string{"--selection-profile", "coverage"}, "require --provider"},
		{[]string{"--provider", "p.json", "--selection-profile", "all"}, "must be strict, coverage or e2e-safe"},
		{[]string{"--provider", "p.json", "--repository", "e2e=a", "--repository", "e2e=b"}, "bound twice"},
		{[]string{"--provider"}, "requires exactly one value"},
		{[]string{"--provider", "p.json", "--limit", "3"}, "unrecognized arguments"},
		{[]string{"--provider-command", `["bin/provider"]`}, "absolute, clean path"},
		{[]string{"--provider-command", "p.json"}, "expected a JSON array"},
	} {
		_, stdout, stderr, code := runAffectedArguments(t, root, tc.arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, tc.message) {
			t.Errorf("%v: exit %d stderr %q, want exit 2 with %q", tc.arguments, code, stderr, tc.message)
		}
	}
	providers := []string{}
	for range 5 {
		providers = append(providers, "--provider", "p.json")
	}
	if _, _, stderr, code := runAffectedArguments(t, root, providers...); code != 2 || !strings.Contains(stderr, "at most 4 providers") {
		t.Errorf("a fifth provider must be refused: %d %s", code, stderr)
	}
	if !strings.Contains(affectedHelp, "--provider-command ARGV_JSON") {
		t.Error("help must document --provider-command")
	}
	if !strings.Contains(affectedHelp, "--selection-profile strict|coverage") {
		t.Error("help must document --selection-profile")
	}
}

// EEP-V2-006: a direct path-to-path test relation narrows only under the V2
// profile; the same relation under V1 stays unsupported and widens.
func TestAffectedSelectionPathRelation(t *testing.T) {
	t.Parallel()
	root, base, _ := affectedSelectionRepository(t)
	origin := strings.TrimSpace(affectedGit(t, root, "rev-list", "--max-parents=0", "HEAD"))
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	record := func(schema string) string {
		body := `{"schema":"` + schema + `","provider":{"id":"mockdocs","revision":"1"},
"repositories":[{"id":"application","origin":"` + origin + `","revision":"` + head + `"}],"entities":[],
"relations":[{"from":{"repository":"application","path":"core/core_test.go"},"to":{"repository":"application","path":"core/core.go"},"type":"verifies","evidence":"observed","rule":"unit-run","reference":"ci/1"}]}`
		path := filepath.Join(t.TempDir(), "provider.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	advised, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record("external-evidence-provider/2"))
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selection := testSelection(t, advised)
	selected := selection["selected"].([]any)
	if selection["state"] != "narrow-selection-allowed" || len(selected) != 1 {
		t.Fatalf("V2 path relation: state %v selected %v", selection["state"], selected)
	}
	if row := selected[0].(map[string]any); row["test"].(map[string]any)["path"] != "core/core_test.go" || row["subject"].(map[string]any)["path"] != "core/core.go" {
		t.Fatalf("path row = %v", row)
	}
	legacy, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--provider", record("external-evidence-provider/1"))
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if selection := testSelection(t, legacy); selection["state"] != "full-relevant-suite-required" || len(selection["selected"].([]any)) != 0 {
		t.Fatalf("V1 path relation must not narrow: %v", selection["state"])
	}
}
