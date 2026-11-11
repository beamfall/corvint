// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// AFP-V0-9731 (V1-4971): the compact plan's testCount is the number of
// runnable top-level tests, the ones `go test -list` would run for the
// package, and the old test-file count stays under its own name,
// testFileCount, in the compact and by-path wires and their text forms.
// Both are pinned on one fixture whose files hold tests, a lowercase
// near-miss, a benchmark, a fuzz target, examples with and without output
// and the TestMain hook. (Go vet refuses a lowercase Test near-miss, so the
// helper is plain lowercase.)
func TestAffectedTestCountsMatchGoTestList_AFPV09731(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("go test -list needs the Go toolchain on PATH")
	}
	root := t.TempDir()
	writeAffectedFiles(t, root, map[string]string{
		"go.mod":        "module example.test/counts\n\ngo 1.22\n",
		"p/p.go":        "package p\n\nfunc Value() int { return 1 }\n",
		"p/p_test.go":   "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) { os.Exit(m.Run()) }\n\nfunc TestOne(t *testing.T) { _ = Value() }\n\nfunc TestTwo(t *testing.T) {}\n\nfunc testHelper(t *testing.T) {}\n\nfunc BenchmarkValue(b *testing.B) {}\n\nfunc FuzzValue(f *testing.F) { f.Fuzz(func(t *testing.T, n int) {}) }\n\nfunc ExampleValue() {\n\tfmt.Println(Value())\n\t// Output: 1\n}\n\nfunc ExampleValue_silent() {\n\t_ = Value()\n}\n",
		"p/q_test.go":   "package p\n\nimport \"testing\"\n\nfunc TestThree(t *testing.T) {}\n\nfunc helper(t *testing.T) {}\n",
		"p/r_test.go":   "package p\n",
		"docs/notes.md": "notes\n",
	})
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	// The package's own source is dirty, so the plan selects it directly.
	if err := os.WriteFile(filepath.Join(root, "p", "p.go"), []byte("package p\n\nfunc Value() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// go test -list names every test, example and fuzz target the package
	// runs; benchmarks are listed but not run by default and the trailer
	// line starts with "ok". A lowercase helper and an example without output
	// are not listed.
	cache := os.Getenv("CORVINT_GOCACHE")
	if cache == "" {
		cache = os.Getenv("GOCACHE")
	}
	if cache == "" {
		cache = filepath.Join(t.TempDir(), "gocache")
	}
	list := exec.Command(goTool, "test", "-count=1", "-list", ".", "./p")
	list.Dir = root
	list.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off", "GOCACHE="+cache)
	output, err := list.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -list: %v\n%s", err, output)
	}
	var listed []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" || strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "Benchmark") {
			continue
		}
		listed = append(listed, line)
	}
	slices.Sort(listed)
	if want := []string{"ExampleValue", "FuzzValue", "TestOne", "TestThree", "TestTwo"}; !slices.Equal(listed, want) {
		t.Fatalf("go test -list named %v, the fixture expects %v:\n%s", listed, want, output)
	}

	compact, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("affected exit %d: %s", code, stderr)
	}
	const unit = "go:example.test/counts/p"
	var selection map[string]any
	for _, raw := range compact["plan"].(map[string]any)["selected"].([]any) {
		if item := raw.(map[string]any); item["unitId"] == unit {
			selection = item
		}
	}
	if selection == nil {
		t.Fatalf("%s is not selected: %v", unit, compact["plan"])
	}
	if selection["testCount"] != float64(len(listed)) {
		t.Fatalf("testCount=%v, go test -list runs %d: %v", selection["testCount"], len(listed), listed)
	}
	if selection["testFileCount"] != float64(3) {
		t.Fatalf("testFileCount=%v, the package has 3 test files", selection["testFileCount"])
	}
	// The full plan keeps the file list the compact count summarizes.
	full, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("affected --full exit %d: %s", code, stderr)
	}
	for _, raw := range full["plan"].(map[string]any)["selected"].([]any) {
		if item := raw.(map[string]any); item["unitId"] == unit {
			if files := item["tests"].([]any); len(files) != 3 || item["testFileCount"] != nil || item["testCount"] != nil {
				t.Fatalf("the full plan changed: %v", item)
			}
		}
	}
	// The text and by-path forms carry both counts under their names.
	text, textStderr, code := runAffectedForm(t, root, false, "--format", "text")
	if code != 0 || !strings.Contains(text, "selected "+unit+" tests=5 test-files=3 ") {
		t.Fatalf("text form (exit %d) lacks both counts:\n%s%s", code, text, textStderr)
	}
	// --path plans at a clean HEAD, so the dirty edit is committed first.
	affectedGit(t, root, "commit", "-qam", "edit")
	byPath, _, stderr, code := runAffectedCLI(t, root, "--by-path", "--path", "p/p.go", "--path", "docs/notes.md")
	if code != 0 {
		t.Fatalf("by-path exit %d: %s", code, stderr)
	}
	found := false
	for _, raw := range byPath["paths"].([]any) {
		for _, added := range raw.(map[string]any)["additions"].([]any) {
			if item := added.(map[string]any); item["unitId"] == unit {
				found = true
				if item["testCount"] != float64(5) || item["testFileCount"] != float64(3) {
					t.Fatalf("by-path counts: %v", item)
				}
			}
		}
	}
	if !found {
		t.Fatalf("by-path lists no addition for %s: %v", unit, byPath["paths"])
	}
}
