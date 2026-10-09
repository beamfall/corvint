package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplan"
)

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func runTestPlanCLI(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(arguments, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func testPlanGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir, c.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

func testPlanVariation(id string, extra map[string]any) map[string]any {
	v := map[string]any{
		"variation_id": id, "spec": "tests/e2e/book.spec.ts", "app": "admin", "setup": "scenarios/club.ts",
		"screen": "screen:admin:club.teesheet", "user": "club-admin", "org": "club-a",
		"action": []any{"element:" + id}, "assertion": []any{"element:see"}, "requires": []any{}, "changes": []any{}, "destructive": false,
	}
	for k, value := range extra {
		v[k] = value
	}
	return v
}

func writeTestPlanFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testPlanInput(t *testing.T, variations ...map[string]any) string {
	t.Helper()
	rows := []any{}
	for _, v := range variations {
		rows = append(rows, v)
	}
	data, err := json.Marshal(map[string]any{"schema": "test-consolidation-input/0", "variations": rows})
	if err != nil {
		t.Fatal(err)
	}
	return writeTestPlanFile(t, "input.json", data)
}

// testPlanRepository commits one spec file and a .corvint directory; it returns the root and the
// spec's absolute path.
func testPlanRepository(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testPlanGit(t, root, "init", "-q")
	spec := filepath.Join(root, "tests", "e2e", "book.spec.ts")
	for path, content := range map[string]string{spec: "test('books')\n", filepath.Join(root, ".corvint", "keep"): "x\n"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testPlanGit(t, root, "add", "-A")
	testPlanGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "fixture")
	return root, spec
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		data := []byte{}
		if info.Mode().IsRegular() {
			if data, err = os.ReadFile(path); err != nil {
				return err
			}
		}
		sum := sha256.Sum256(data)
		b.WriteString(rel + " " + info.Mode().String() + " " + info.ModTime().String() + " " + hex.EncodeToString(sum[:]) + "\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TCN-V0-001: consolidate reads only its named files and Git, writes only stdout, exits 0 with a
// candidate plan, and leaves every byte of the repository, its index and .corvint unchanged.
func TestTestPlanConsolidateIsReadOnly(t *testing.T) {
	root, spec := testPlanRepository(t)
	sum := sha256.Sum256([]byte("test('books')\n"))
	receipt, _ := json.Marshal(map[string]any{"receipt": map[string]any{"kind": "e2e",
		"identity": map[string]any{"testFileDigests": map[string]string{spec: hex.EncodeToString(sum[:])}},
		"tests":    []any{map[string]any{"name": "books", "state": "passed"}}}})
	tests := writeTestPlanFile(t, "receipt.json", receipt)
	input := testPlanInput(t, testPlanVariation("V1", nil), testPlanVariation("V2", map[string]any{"destructive": true}),
		testPlanVariation("V3", map[string]any{"witnesses": []any{map[string]any{"test_id": "absent", "basis": "declared"}}}))
	before := snapshotTree(t, root)

	code, table, stderr := runTestPlanCLI(t, "--root", root, "test-plan", "consolidate", "--input", input, "--tests", tests)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	if !strings.HasPrefix(table, "<!-- corvint test-consolidation-table/0 ") || !strings.Contains(table, "| T001 | tests/e2e/book.spec.ts | V1, V3 | - |\n") ||
		!strings.Contains(table, " anchors=NOT_RUN ") || !strings.Contains(table, "| T002 | tests/e2e/book.spec.ts | V2 | destructive-change |\n") {
		t.Fatalf("table:\n%s", table)
	}
	code, document, stderr := runTestPlanCLI(t, "--root="+root, "test-plan", "consolidate", "--format=json", "--input", input, "--tests", tests)
	if code != 0 || stderr != "" {
		t.Fatalf("json exit %d stderr %s", code, stderr)
	}
	var plan struct {
		Authority         string `json:"authority"`
		EvaluatedRevision string `json:"evaluated_revision"`
		TableDigest       string `json:"table_digest"`
		WitnessRejections []any  `json:"witness_rejections"`
	}
	if err := json.Unmarshal([]byte(document), &plan); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(testPlanGit(t, root, "rev-parse", "HEAD"))
	tableSum := sha256.Sum256([]byte(table))
	if plan.Authority != "candidate" || plan.EvaluatedRevision != head || plan.TableDigest != "sha256:"+hex.EncodeToString(tableSum[:]) || len(plan.WitnessRejections) != 1 {
		t.Fatalf("plan %+v", plan)
	}
	library, err := testplan.Run(context.Background(), testplan.Request{Root: root, Input: mustRead(t, input), Tests: []string{tests}})
	if err != nil || string(library.Table()) != table || string(library.JSON()) != document {
		t.Fatalf("the CLI bytes differ from the library plan: %v", err)
	}
	if after := snapshotTree(t, root); after != before {
		t.Fatalf("the repository changed:\n%s\n---\n%s", before, after)
	}
}

// TCN-V0-001: invalid arguments or input exit 2 with nothing on stdout.
func TestTestPlanRefusesInvalidUse(t *testing.T) {
	input := testPlanInput(t, testPlanVariation("V1", nil))
	plan := writeTestPlanFile(t, "plan.md", []byte("no table\n"))
	big := writeTestPlanFile(t, "big.json", bytes.Repeat([]byte(" "), 8<<20+1))
	invalid := writeTestPlanFile(t, "invalid.json", []byte(`{"schema":"test-consolidation-input/0","variations":[]}`))
	cases := map[string]struct {
		code      string
		arguments []string
	}{
		"no subcommand":         {"test-plan-invalid-arguments", nil},
		"unknown subcommand":    {"test-plan-invalid-arguments", []string{"merge", "--input", input}},
		"missing input":         {"test-plan-invalid-arguments", []string{"consolidate"}},
		"input twice":           {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--input", input}},
		"revision twice":        {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--revision", "HEAD", "--revision", "HEAD"}},
		"format xml":            {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--format", "xml"}},
		"format on check":       {"test-plan-invalid-arguments", []string{"check", "--input", input, "--plan", plan, "--format", "json"}},
		"plan on consolidate":   {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--plan", plan}},
		"check without plan":    {"test-plan-invalid-arguments", []string{"check", "--input", input}},
		"max-steps low":         {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--max-steps", "1"}},
		"max-steps high":        {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--max-steps=33"}},
		"max-steps text":        {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--max-steps", "eight"}},
		"unknown option":        {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--write"}},
		"missing value":         {"test-plan-invalid-arguments", []string{"consolidate", "--input"}},
		"empty value":           {"test-plan-invalid-arguments", []string{"consolidate", "--input="}},
		"revision without file": {"test-plan-invalid-arguments", []string{"consolidate", "--input", input, "--revision", "HEAD"}},
		"absent input":          {"test-plan-invalid-input", []string{"consolidate", "--input", filepath.Join(t.TempDir(), "absent.json")}},
		"input directory":       {"test-plan-invalid-input", []string{"consolidate", "--input", t.TempDir()}},
		"oversized input":       {"test-plan-invalid-input", []string{"consolidate", "--input", big}},
		"invalid input":         {"test-plan-invalid-input", []string{"consolidate", "--input", invalid}},
		"oversized plan":        {"test-plan-invalid-input", []string{"check", "--input", input, "--plan", big}},
		"bad receipt":           {"invalid-test-validity-receipt", []string{"consolidate", "--input", input, "--tests", invalid}},
		"bad map":               {"appmap-invalid-map", []string{"consolidate", "--input", input, "--map", invalid}},
	}
	root, _ := testPlanRepository(t)
	for name, c := range cases {
		code, stdout, stderr := runTestPlanCLI(t, append([]string{"--root", root, "test-plan"}, c.arguments...)...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "`+c.code+`"`) {
			t.Errorf("%s: exit %d stdout %q stderr %s", name, code, stdout, stderr)
		}
	}
}

// TCN-V0-011: check exits 0 on a matching COMPLETE plan, 1 on a mismatch, a missing header or an
// INCOMPLETE plan, with nothing on stdout when it fails.
func TestTestPlanCheckExitCodes(t *testing.T) {
	input := testPlanInput(t, testPlanVariation("V1", nil), testPlanVariation("V2", nil))
	code, table, _ := runTestPlanCLI(t, "test-plan", "consolidate", "--input", input)
	if code != 0 {
		t.Fatalf("consolidate exit %d", code)
	}
	plan := writeTestPlanFile(t, "plan.md", []byte("# Review\n\n"+strings.ReplaceAll(table, "|\n", "|  \n")+"\nApproved.\n"))
	if code, stdout, stderr := runTestPlanCLI(t, "test-plan", "check", "--plan", plan, "--input", input); code != 0 || stdout != table || stderr != "" {
		t.Fatalf("matching plan: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
	edited := writeTestPlanFile(t, "edited.md", []byte(strings.Replace(table, "V1, V2", "V2, V1", 1)))
	headerless := writeTestPlanFile(t, "headerless.md", []byte(table[strings.Index(table, "\n")+1:]))
	incompleteInput := testPlanInput(t, testPlanVariation("V1", nil), testPlanVariation("V2", nil), testPlanVariation("V3", map[string]any{"changes": nil}))
	_, incompleteTable, _ := runTestPlanCLI(t, "test-plan", "consolidate", "--input", incompleteInput)
	incomplete := writeTestPlanFile(t, "incomplete.md", []byte(incompleteTable))
	for name, c := range map[string]struct {
		code      string
		arguments []string
	}{
		"edited row":          {"test-plan-mismatch", []string{"--plan", edited, "--input", input}},
		"max-steps disagrees": {"test-plan-mismatch", []string{"--plan", plan, "--input", input, "--max-steps", "4"}},
		"no header":           {"test-plan-header-missing", []string{"--plan", headerless, "--input", input}},
		"incomplete":          {"test-plan-incomplete", []string{"--plan", incomplete, "--input", incompleteInput}},
	} {
		code, stdout, stderr := runTestPlanCLI(t, append([]string{"test-plan", "check"}, c.arguments...)...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, `"code": "`+c.code+`"`) {
			t.Errorf("%s: exit %d stdout %q stderr %s", name, code, stdout, stderr)
		}
	}
}

func TestTestPlanHelp(t *testing.T) {
	code, stdout, _ := runTestPlanCLI(t, "test-plan", "--help")
	if code != 0 || !strings.Contains(stdout, "test-plan consolidate --input FILE") || !strings.Contains(rootHelp, "test-plan") {
		t.Fatalf("exit %d help %q", code, stdout)
	}
}
