package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCheck(t *testing.T, root string, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(root, arguments, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestASDV0996RepositoryGuideResolves(t *testing.T) {
	code, stdout, stderr := runCheck(t, filepath.Join("..", ".."))
	if code != 0 || !strings.HasPrefix(stdout, "flow-guide-check: ok (") {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
}

func TestASDV0997RepositoryRefusalsAreHintedOrListed(t *testing.T) {
	code, stdout, stderr := runCheck(t, filepath.Join("..", ".."), "--audit")
	if code != 0 || !strings.Contains(stdout, "surface=dogfoodflow sites=") || !strings.Contains(stdout, "surface=corvint-tasks NOT_CLASSIFIED refusal_call_sites=") {
		t.Fatalf("audit exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
}

const tree = `{"schema":"corvint-command-tree/0","binary":"%s","globalFlags":["--root"],"nodes":[{"path":["dogfood"],"command":"corvint dogfood","flags":["--refresh"]},{"path":["dogfood","propose-citations"],"command":"corvint dogfood propose-citations","flags":["--refresh"]}]}`

const guide = "# Lane orchestration\n\n## 13. Lane flow guide\n\nRun `corvint dogfood propose-citations --refresh PLAN BASE > draft.tsv`,\n`make doc-gates BASE=..`, `script/heavy.sh make doc-gates` and `git status`; bare `corvint`, `make` and `script/*.sh` are prose.\n\n| Refusal | Recovery |\n|---|---|\n| `heavy.sh: REFUSED whole-reason-missing` | set the reason |\n\n## 14. Next\n\n`corvint missing-verb` outside the section is not checked.\n"

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		guidePath:                        guide,
		skillPath:                        "Run `make doc-gates`.\n",
		treesDir + "/corvint.json":       strings.Replace(tree, "%s", "corvint", 1),
		treesDir + "/corvint-tasks.json": strings.Replace(tree, "%s", "corvint-tasks", 1),
		"Makefile":                       ".PHONY: doc-gates\ndoc-gates:\n\t@true\nVAR := x\n",
		"script/heavy.sh":                "refuse \"whole-reason-missing set HEAVY_WHOLE_REASON\"\nrefuse \"invalid-slots HEAVY_SLOTS=$slots (want 1..64)\"\n",
		"script/heavy_test.sh":           "refuse \"test-only-code\"\n",
		dogfoodDir + "/check.go":         "package dogfoodflow\n\nvar failRecoveries = map[string]string{\"report-drift\": \"rerun\"}\n\nfunc f(c interface{ fail(string, ...string) }) {\n\tc.fail(\"report-drift\")\n\tc.fail(\"own-fix\", \"  fix: rerun it\")\n}\n",
		baselinePath:                     "# header\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func edit(t *testing.T, root, name, old, replacement string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), old) {
		t.Fatalf("%s lacks %q", name, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), old, replacement, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestASDV0996DriftFails(t *testing.T) {
	if code, stdout, stderr := runCheck(t, fixture(t)); code != 0 {
		t.Fatalf("clean fixture exit %d: %s%s", code, stdout, stderr)
	}
	for _, test := range []struct{ file, old, replacement, want string }{
		{guidePath, "propose-citations --refresh", "propose-citations --rerfesh", "corvint dogfood propose-citations has no flag --rerfesh"},
		{guidePath, "corvint dogfood propose-citations", "corvint dogfood propose", "no corvint subcommand dogfood propose"},
		// The dogfood dispatcher owns --refresh, but propose is no child of it (ASD-V0-996).
		{guidePath, "corvint dogfood propose-citations --refresh", "corvint dogfood propose --refresh", "no corvint subcommand dogfood propose"},
		{guidePath, "`make doc-gates BASE=..`", "`make doc-gate BASE=..`", "no Makefile rule doc-gate"},
		{guidePath, "script/heavy.sh make doc-gates", "script/heavy.sh make docgates", "no Makefile rule docgates"},
		{guidePath, "`script/heavy.sh make", "`script/light.sh make", "no such script script/light.sh"},
		{guidePath, "REFUSED whole-reason-missing", "REFUSED whole-reason-absent", "refusal code whole-reason-absent in the recovery table occurs in no non-test source"},
		{skillPath, "make doc-gates", "make doc-gates-all", ".claude/skills/corvint-lane-flow/SKILL.md:1: `make doc-gates-all`: no Makefile rule doc-gates-all"},
	} {
		root := fixture(t)
		edit(t, root, test.file, test.old, test.replacement)
		if code, _, stderr := runCheck(t, root); code != 1 || !strings.Contains(stderr, test.want) {
			t.Errorf("%s -> %s: exit %d, stderr %s; want %q", test.old, test.replacement, code, stderr, test.want)
		}
	}
	// The multi-line span is checked as one command, with its line number.
	root := fixture(t)
	edit(t, root, guidePath, "`make doc-gates BASE=..`", "`make doc-gates\nmissing-rule BASE=..`")
	if code, _, stderr := runCheck(t, root); code != 1 || !strings.Contains(stderr, guidePath+":6: `make doc-gates missing-rule BASE=..`: no Makefile rule missing-rule") {
		t.Errorf("multi-line span: exit %d, stderr %s", code, stderr)
	}
}

func TestASDV0997NewUnhintedRefusalFails(t *testing.T) {
	for _, test := range []struct{ file, old, replacement, want string }{
		{"script/heavy.sh", "refuse \"invalid-slots", "refuse \"lock-dir-unavailable /tmp\"\nrefuse \"invalid-slots", "script/heavy.sh:2: refusal lock-dir-unavailable names no recovery"},
		{dogfoodDir + "/check.go", "c.fail(\"report-drift\")", "c.fail(\"report-drift\")\n\tc.fail(\"new-drift\", \"  paths=x\")", dogfoodDir + "/check.go:7: refusal new-drift names no recovery"},
	} {
		root := fixture(t)
		edit(t, root, test.file, test.old, test.replacement)
		if code, _, stderr := runCheck(t, root); code != 1 || !strings.Contains(stderr, test.want) {
			t.Errorf("%s: exit %d, stderr %s; want %q", test.replacement, code, stderr, test.want)
		}
		// Listing the pair admits it.
		pair := strings.SplitN(test.want, ":", 2)[0]
		code := strings.Fields(strings.SplitN(test.want, "refusal ", 2)[1])[0]
		edit(t, root, baselinePath, "# header\n", "# header\n"+pair+"\t"+code+"\n")
		if exit, stdout, stderr := runCheck(t, root); exit != 0 {
			t.Errorf("listed %s %s: exit %d: %s%s", pair, code, exit, stdout, stderr)
		}
	}
	// A refusal in a test script is not a site.
	root := fixture(t)
	if code, stdout, _ := runCheck(t, root, "--audit"); code != 0 || !strings.Contains(stdout, "surface=script sites=2 hinted=2 unhinted=0") || !strings.Contains(stdout, "surface=dogfoodflow sites=2 hinted=2 unhinted=0") {
		t.Errorf("audit exit %d: %s", code, stdout)
	}
}

func TestASDV0997StaleRowFails(t *testing.T) {
	root := fixture(t)
	edit(t, root, baselinePath, "# header\n", "# header\nscript/heavy.sh\tinvalid-slots\n")
	if code, _, stderr := runCheck(t, root); code != 1 || !strings.Contains(stderr, "stale row \"script/heavy.sh\\tinvalid-slots\"") {
		t.Fatalf("stale row: exit %d, stderr %s", code, stderr)
	}
	edit(t, root, baselinePath, "script/heavy.sh\tinvalid-slots\n", "malformed row\n")
	if code, _, stderr := runCheck(t, root); code != 1 || !strings.Contains(stderr, "want PATH<TAB>CODE") {
		t.Fatalf("malformed row: exit %d, stderr %s", code, stderr)
	}
	if code, _, stderr := runCheck(t, root, "--bogus"); code != 2 || !strings.Contains(stderr, "usage:") {
		t.Fatalf("usage: exit %d, stderr %s", code, stderr)
	}
}
