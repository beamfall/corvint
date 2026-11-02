// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// checkBoth runs the check on root sequentially (one package at a time, its
// files in order, no memoized parse) and then in parallel, and returns both
// outputs with their exit codes and the parallel run's duration.
func checkBoth(t *testing.T, root string) (sequential, parallel string, elapsed time.Duration) {
	t.Helper()
	once := func(workers int, files bool) string {
		previousWorkers, previousFiles := checkWorkers, parallelFiles
		checkWorkers, parallelFiles = workers, files
		defer func() { checkWorkers, parallelFiles = previousWorkers, previousFiles }()
		parsedTests = sync.Map{}
		var stdout, stderr bytes.Buffer
		code := run(root, &stdout, &stderr)
		return strings.Join([]string{string(rune('0' + code)), stdout.String(), stderr.String()}, "\n")
	}
	sequential = once(1, false)
	start := time.Now()
	parallel = once(8, true)
	return sequential, parallel, time.Since(start)
}

// TestDCWV0759ParallelCheckMatchesTheSequentialCheck (V1-4238): the check that
// runs packages and files in parallel, memoizes each file's parse and runs each
// go list once per package reports exactly what the sequential check reports,
// byte for byte, on a fixture with every finding kind (a named path, a go list
// and a go build dependency, a nested sandbox, a stale recorded literal) and on
// this repository.
func TestDCWV0759ParallelCheckMatchesTheSequentialCheck(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	list := "package gitnotes\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestLists(t *testing.T) {\n\tgoTool, _ := exec.LookPath(\"go\")\n" +
		"\t_, _ = exec.Command(goTool, \"list\", \"-deps\", \".\").Output()\n}\n"
	build := "package main\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestBuilds(t *testing.T) {\n\tgoPath, _ := exec.LookPath(\"go\")\n" +
		"\t_ = exec.Command(goPath, \"build\", \"-o\", \"./out\", \".\")\n\t_ = exec.Command(goPath, \"vet\", \".\")\n}\n"
	files := map[string]string{
		"go.mod":                         "module example.test/m\n\ngo 1.27\n",
		"protocol/manifest.json":         "{}\n",
		"internal/cem/sim/s.go":          "package sim\n",
		"internal/cem/verify/v.go":       "package verify\n\nimport _ \"example.test/m/internal/cem/sim\"\n",
		"internal/gitnotes/g.go":         "package gitnotes\n\nimport _ \"example.test/m/internal/cem/verify\"\n",
		"internal/gitnotes/a_test.go":    list,
		"internal/gitnotes/b_test.go":    "package gitnotes\n\nvar top = \"" + strings.Repeat(up, 2) + "\"\n\nvar manifest = top + \"protocol/manifest.json\"\n",
		"internal/archivepath/a.go":      "package archivepath\n",
		"tools/trial/main.go":            "package main\n\nimport _ \"example.test/m/internal/archivepath\"\n\nfunc main() {}\n",
		"tools/trial/runner_test.go":     build,
		"tools/other/o.go":               "package other\n",
		"tools/other/o_test.go":          "package other\n\nvar manifest = \"protocol/manifest.json\"\n",
		"internal/sandboxed/s.go":        "package sandboxed\n",
		"internal/sandboxed/s_test.go":   "package sandboxed\n",
		sandboxesPath:                    `{"profile":"corvint-test-nested-sandboxes/0","packages":{"internal/sandboxed":"bubblewrap"}}`,
		literalsPath:                     `{"profile":"corvint-test-read-scope-literals/0","literals":{"tools/other":{"protocol/gone.json":"no longer named"}}}`,
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"internal/gitnotes":["go.mod"],"tools/trial":["go.mod"],"tools/other":[],"internal/sandboxed":[]}}`,
	}
	sequential, parallel, _ := checkBoth(t, writeRepository(t, files))
	if sequential != parallel {
		t.Fatalf("fixture: the parallel check differs:\nparallel:\n%s\nsequential:\n%s", parallel, sequential)
	}
	for _, want := range []string{"runs go list -deps", "builds its own package", "nested sandbox", "names protocol/manifest.json", "records tools/other protocol/gone.json"} {
		if !strings.Contains(sequential, want) {
			t.Errorf("the fixture lacks the %q finding, so it compares less than it should:\n%s", want, sequential)
		}
	}
	if testing.Short() {
		return
	}
	out, err := exec.Command("git", "rev-parse", rootLocatorToken).Output()
	if err != nil {
		t.Skipf("no repository: %v", err)
	}
	sequential, parallel, elapsed := checkBoth(t, strings.TrimSpace(string(out)))
	if sequential != parallel {
		t.Fatalf("repository: the parallel check differs:\nparallel:\n%s\nsequential:\n%s", parallel, sequential)
	}
	t.Logf("repository: parallel check %.2fs, identical to the sequential check", elapsed.Seconds())
}
