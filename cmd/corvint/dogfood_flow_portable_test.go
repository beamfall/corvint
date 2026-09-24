package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/localcompletion"
)

// portableDogfoodRepo is a Git repository that is not Corvint: no script/, no
// VERSION and no Corvint source, only a small Go module with one intent.
func portableDogfoodRepo(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "go.mod", "module example.com/portable\n\ngo 1.22\n")
	cemWrite(t, root, "AGENTS.md", "# Fixture authority\nUse intent.md to govern the fixture.\n")
	cemWrite(t, root, "intent.md", "# Intent\n\n## Requirements\n\n- `FIXTURE-LCP-001`: The answer is two.\n- `FIXTURE-LCP-002`: The existing companion behavior remains unchanged.\n")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 1 }\n")
	cemWrite(t, root, "fixture/fixture_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) {\n t.Run(\"FIXTURE-LCP-001\",func(t *testing.T){ if Answer()!=2 { t.Fatal(\"answer\") } })\n}\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	// The private outputs and local trace store the daily path writes in the
	// worktree; the recorder and the clean check require them ignored.
	exclude := ".corvint/dogfood-report.json\n.corvint/change.ocm-intents\n.corvint/change.ocm-status.json\n.corvint/change.ocm.*.json\n.corvint/self-observations.jsonl\n.context-corvint/\n"
	if err = os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte(exclude), 0644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "commit", "-qam", "change")
	for _, absent := range []string{"script", "VERSION", "cmd/corvint"} {
		if _, err = os.Lstat(filepath.Join(root, absent)); !os.IsNotExist(err) {
			t.Fatalf("fixture carries %s", absent)
		}
	}
	return root, base
}

type portableRun struct {
	binary string
	env    []string
}

// portableDogfoodRunner builds this binary and an environment whose PATH
// resolves `corvint` to a failing impostor and whose CORVINT_BIN names a
// missing file, so every step must run as the executable itself.
func portableDogfoodRunner(t *testing.T) portableRun {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "bin", "corvint")
	if err := workBuildCorvintWithoutVCS(binary); err != nil {
		t.Fatal(err)
	}
	impostor := t.TempDir()
	if err := os.WriteFile(filepath.Join(impostor, "corvint"), []byte("#!/bin/sh\nexit 99\n"), 0755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PATH="+impostor+string(os.PathListSeparator)+os.Getenv("PATH"), "CORVINT_BIN="+filepath.Join(impostor, "missing"))
	return portableRun{binary: binary, env: env}
}

func (run portableRun) exec(t *testing.T, root string, extra []string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr strings.Builder
	command := exec.Command(run.binary, args...)
	command.Dir = root
	command.Env = append(append([]string{}, run.env...), extra...)
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

func (run portableRun) ok(t *testing.T, root string, args ...string) string {
	t.Helper()
	code, stdout, stderr := run.exec(t, root, nil, args...)
	if code != 0 {
		t.Fatalf("%v exit=%d stdout=%s stderr=%s", args, code, stdout, stderr)
	}
	return stdout
}

// DCW-V0-020, DCW-V0-021 and DCW-V0-023: the installed binary alone runs change, check and
// seal in a repository with no Corvint scripts, VERSION or source.
func TestDogfoodDailyPathRunsFromBinaryInForeignRepository(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	if err := os.WriteFile(citations, []byte("1\tintent.md\t1:5\tspecification\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intents, []byte("intent.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	// The first pass prepares the sidecar; whether it is already complete is
	// the recorder's concern (docs/DOGFOOD.md, Daily adopter path step 4).
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code > 1 {
		t.Fatalf("first change exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("change exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "check", base)
	if code != 0 || !strings.HasSuffix(stdout, "dogfood-check: PASS\n") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	nested := filepath.Join(root, "fixture")
	if code, _, stderr = run.exec(t, nested, nil, "dogfood", "check", base); code != 2 || stderr != "dogfood-check: REFUSE not-repository-root\n" {
		t.Fatalf("nested root exit=%d stderr=%s", code, stderr)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "seal", base)
	if code != 0 || !strings.HasSuffix(stdout, "dogfood-seal: PASS sealed=.corvint/changes/"+bind+".cem.json\n") {
		t.Fatalf("seal exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	cemGit(t, root, "cat-file", "-e", "HEAD:.corvint/changes/"+bind+".cem.json")
	if subject := cemGit(t, root, "log", "-1", "--format=%s"); subject != "chore: seal change evidence" {
		t.Fatalf("seal commit %q", subject)
	}
}

// LCP-V0-014: finish runs the change and the final check in-process from the
// installed binary, ignoring CORVINT_BIN and the DOGFOOD_* inputs.
func TestDogfoodFinishRunsFromBinaryInForeignRepository(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	run.env = append(run.env, "DOGFOOD_OUTCOME=failed", "DOGFOOD_VERIFY=false", "DOGFOOD_CITATIONS="+filepath.Join(root, "missing.tsv"), "DOGFOOD_INTENTS_FILE="+filepath.Join(root, "missing"))
	key := localcompletion.HashSession(t.Name())
	plan, _ := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture"}, TimeoutSeconds: 60}}})
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	session := []string{"--session-key", key}
	run.ok(t, root, append([]string{"dogfood", "begin", "--plan", planPath}, session...)...)
	run.ok(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	run.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	target := cemGit(t, root, "rev-parse", "HEAD")
	run.ok(t, root, "ocm", "prepare", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--intent", "intent.md", "--expected-base", base, "--target", target)
	run.ok(t, root, append([]string{"dogfood", "verify", "--check", "actual-test"}, session...)...)
	run.ok(t, root, "ocm", "link", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--obligation", "FIXTURE-LCP-001", "--hunk", "1", "--test-path", "fixture/fixture_test.go", "--claim", "test:TestAnswer/case:fixture-lcp", "--expected-base", base, "--target", target)
	var inspected, finished struct {
		Policy struct {
			Lifecycle       string `json:"lifecycle"`
			Satisfied       bool   `json:"satisfied"`
			ReportSetDigest string `json:"reportSetDigest"`
		} `json:"policy"`
	}
	// Before review, finish runs the coordinator and the final check, then
	// exits 1 naming the report set to review.
	code, output, stderr := run.exec(t, root, nil, append([]string{"dogfood", "finish"}, session...)...)
	if err := json.Unmarshal([]byte(output), &inspected); err != nil || code != 1 || inspected.Policy.ReportSetDigest == "" {
		t.Fatalf("finish before review exit=%d stdout=%s stderr=%s %v", code, output, stderr, err)
	}
	run.ok(t, root, append([]string{"dogfood", "review", "--report-set", inspected.Policy.ReportSetDigest}, session...)...)
	output = run.ok(t, root, append([]string{"dogfood", "finish"}, session...)...)
	if err := json.Unmarshal([]byte(output), &finished); err != nil || !finished.Policy.Satisfied || finished.Policy.Lifecycle != "satisfied" {
		t.Fatalf("finish: %s %v", output, err)
	}
}
