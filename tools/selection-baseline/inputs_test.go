// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func baselineFile(t *testing.T, root, path, body string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func baselineGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "-c", "maintenance.auto=false"}, args...)...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

// ASD-V0-944: use actual Git diffs, the production compilation closure and
// Corvint's actual reader index, including a declared non-source runtime input.
func TestBaselineInputDecisionASDV0944(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".gitignore":                         "ignored.go\n",
		"go.mod":                             "module example.com/baseline\n",
		"cmd/corvint/main.go":                "package main\nimport _ \"example.com/baseline/internal/engine\"\nfunc main() {}\n",
		"internal/engine/engine.go":          "package engine\n",
		"tools/gate-affected-select/main.go": "package main\nfunc main() {}\n",
		"tools/selection-baseline/main.go":   "package main\nfunc main() {}\n",
		".corvint/test-read-scopes.json":     `{"profile":"corvint-test-read-scopes/0","packages":{"tools/selection-baseline":["baseline-input/"]}}`,
	}
	for p, b := range files {
		baselineFile(t, root, p, b)
	}
	baselineGit(t, root, "init", "-q")
	baselineGit(t, root, "add", ".")
	baselineGit(t, root, "commit", "-qm", "base")
	base := baselineGit(t, root, "rev-parse", "HEAD")
	for _, tc := range []struct {
		path, body string
		want       bool
		reason     string
	}{
		{"docs/guide.md", "guide\n", false, "disjoint"},
		{"tools/unrelated/main.go", "package main\n", false, "disjoint"},
		{"top.txt", "unrelated\n", false, "disjoint"},
		{"internal/engine/engine.go", "package engine\n// changed\n", true, "internal/engine/engine.go"},
		{"internal/engine/ignored.go", "package engine\n", true, "internal/engine/ignored.go"},
		{"benchmarks/selection-baseline/changes.json", "{}", true, "changes.json"},
		{".github/cishards/package-costs.json", "{}", true, "package-costs.json"},
		{"baseline-input/config.txt", "changed", true, "matching reader input baseline-input/config.txt"},
		{".corvint/test-read-scopes.json", "invalid", true, "unknown read set"},
		{"internal/engine/engine.go", "not Go", true, "compilation closure unavailable"},
	} {
		t.Run(tc.path+tc.reason, func(t *testing.T) {
			baselineFile(t, root, tc.path, tc.body)
			got, reason := inputDecision(context.Background(), root, base)
			if got != tc.want || !strings.Contains(reason, tc.reason) {
				t.Fatalf("run=%v reason=%q, want %v / %q", got, reason, tc.want, tc.reason)
			}
			if original, ok := files[tc.path]; ok {
				baselineFile(t, root, tc.path, original)
			} else if err := os.Remove(filepath.Join(root, tc.path)); err != nil {
				t.Fatal(err)
			}
		})
	}
	if got, reason := inputDecision(context.Background(), root, strings.Repeat("0", 40)); !got || !strings.Contains(reason, "base commit unavailable") {
		t.Fatalf("missing base: %v %s", got, reason)
	}
}

func TestBaselineUnknownReadSetASDV0944(t *testing.T) {
	for _, bounds := range [][]string{
		nil,
		{"unresolved m/tools/selection-baseline: \"dynamic read\"\n"},
		{"bound m/other declared docs/a.md\n"},
	} {
		if run, why := decideInputs([]string{"docs/a.md"}, []string{"internal/engine/"}, bounds, "m/tools/selection-baseline", "base"); !run || !strings.Contains(why, "unknown read set") {
			t.Fatalf("unknown bounds skipped: %v %s", run, why)
		}
	}
	root := t.TempDir()
	for _, data := range []string{"", "{}", `{"Dir":"p","Incomplete":true}`} {
		if _, err := compilationDirs(root, strings.NewReader(data)); err == nil {
			t.Fatalf("unknown compilation accepted: %s", data)
		}
	}
	for _, module := range []map[string]any{
		{}, {"Version": "v1.0.0", "Replace": map[string]any{"Version": ""}}, {"Main": true},
	} {
		data, _ := json.Marshal(map[string]any{"Dir": t.TempDir(), "Module": module})
		if _, err := compilationDirs(root, strings.NewReader(string(data))); err == nil || !strings.Contains(err.Error(), "unpinned external") {
			t.Fatalf("unpinned compilation accepted: %s / %v", data, err)
		}
	}
	data, _ := json.Marshal(map[string]any{"Dir": root, "CgoFiles": []string{"native.go"}})
	if _, err := compilationDirs(root, strings.NewReader(string(data))); err == nil || !strings.Contains(err.Error(), "native compilation read set is unbounded") {
		t.Fatalf("native compiler reads accepted: %v", err)
	}
}

// A participating module's manifest can change resolution without touching its
// imported package subtree. Local module graphs must not be declared disjoint.
func TestBaselineModuleManifestOnlyASDV0944(t *testing.T) {
	for _, workspace := range []bool{false, true} {
		t.Run(map[bool]string{false: "replacement", true: "workspace"}[workspace], func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOWORK", "off")
			baselineFile(t, root, "go.mod", "module example.com/baseline\n\ngo 1.27.0\nrequire example.com/lib v0.0.0\nreplace example.com/lib => ./lib\n")
			baselineFile(t, root, "lib/go.mod", "module example.com/lib\n\ngo 1.27.0\n")
			baselineFile(t, root, "lib/engine/engine.go", "package engine\n")
			baselineFile(t, root, "cmd/corvint/main.go", "package main\nimport _ \"example.com/lib/engine\"\nfunc main() {}\n")
			for _, p := range []string{"tools/gate-affected-select/main.go", "tools/selection-baseline/main.go"} {
				baselineFile(t, root, p, "package main\nfunc main() {}\n")
			}
			baselineFile(t, root, ".corvint/test-read-scopes.json", `{"profile":"corvint-test-read-scopes/0","packages":{"tools/selection-baseline":["benchmarks/selection-baseline/"]}}`)
			if workspace {
				baselineFile(t, root, "go.work", "go 1.27.0\nuse (\n.\n./lib\n)\n")
				t.Setenv("GOWORK", filepath.Join(root, "go.work"))
				baselineFile(t, root, "go.mod", "module example.com/baseline\n\ngo 1.27.0\nrequire example.com/lib v0.0.0\n")
			}
			baselineGit(t, root, "init", "-q")
			baselineGit(t, root, "add", ".")
			baselineGit(t, root, "commit", "-qm", "base")
			base := baselineGit(t, root, "rev-parse", "HEAD")
			baselineFile(t, root, "lib/go.mod", "module example.com/lib\n\ngo 1.27.0\n// manifest-only change\n")
			if diff := baselineGit(t, root, "diff", "--name-only", base); diff != "lib/go.mod" {
				t.Fatalf("not a manifest-only fixture: %s", diff)
			}
			if run, why := inputDecision(context.Background(), root, base); !run || !strings.Contains(why, "workspace or local-module") {
				t.Fatalf("manifest skipped: %v %s", run, why)
			}
			// Modules without imported packages can still change version selection.
			baselineFile(t, root, "cmd/corvint/main.go", "package main\nfunc main() {}\n")
			baselineGit(t, root, "add", ".")
			baselineGit(t, root, "commit", "-qm", "no imported lib package")
			base = baselineGit(t, root, "rev-parse", "HEAD")
			baselineFile(t, root, "lib/go.mod", "module example.com/lib\n\ngo 1.27.0\n// another manifest-only change\n")
			if run, why := inputDecision(context.Background(), root, base); !run || !strings.Contains(why, "workspace or local-module") {
				t.Fatalf("unimported module manifest skipped: %v %s", run, why)
			}
		})
	}
}

func TestBaselineCIBaseASDV0944(t *testing.T) {
	path := filepath.Join(t.TempDir(), "event.json")
	base := strings.Repeat("a", 40)
	for event, body := range map[string]string{
		"pull_request": `{"pull_request":{"base":{"sha":"` + base + `"}}}`,
		"merge_group":  `{"merge_group":{"base_sha":"` + base + `"}}`,
		"push":         `{"before":"` + base + `"}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := eventBase(path, event); got != base || err != nil {
			t.Fatalf("%s: %q %v", event, got, err)
		}
	}
	if _, err := eventBase(path, "workflow_dispatch"); err == nil {
		t.Fatal("unknown event accepted")
	}
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := eventBase(path, "push"); err == nil {
		t.Fatal("malformed event accepted")
	}
}

func TestBaselineMemoryWorkersASDV0925(t *testing.T) {
	for available, want := range map[uint64]int{0: 1, 512 << 20: 1, 4 << 30: 4, 32 << 30: 16} {
		if got := jobsForMemory(available); got != want {
			t.Fatalf("%d bytes: %d, want %d", available, got, want)
		}
	}
	for value, want := range map[string]int{"1": 1, "3": 3, "999": 58} {
		t.Setenv("SELECTION_BASELINE_JOBS", value)
		if got := replayJobs(58); got != want {
			t.Fatalf("override %s: %d, want %d", value, got, want)
		}
	}
	t.Setenv("SELECTION_BASELINE_JOBS", "")
	if got := replayJobs(58); got < 1 || got > 16 || got > memoryJobs() {
		t.Fatalf("unbounded default: %d", got)
	}
}

// Reports remain stable under changes in input ordering as well as worker
// completion ordering (covered by the replay test and full 58-change replay).
func TestBaselineInputReasonIsDeterministic(t *testing.T) {
	bounds := []string{"bound m/tools/selection-baseline declared tools/selection-baseline/main.go\n"}
	for _, paths := range [][]string{{"go.mod", "Makefile"}, {"Makefile", "go.mod"}} {
		_, reason := decideInputs(paths, []string{"cmd/corvint/"}, bounds, "m/tools/selection-baseline", "base")
		if reason != "matching input Makefile" {
			t.Fatal(reason)
		}
	}
	// Ensure a pinned external module is admitted, whereas a local replacement
	// outside the worktree cannot be bounded by a diff here.
	root := t.TempDir()
	baselineFile(t, root, "cmd/corvint/main.go", "package main\n")
	data, _ := json.Marshal(map[string]any{"Dir": filepath.Join(root, "cmd", "corvint"), "GoFiles": []string{"main.go"}})
	external, _ := json.Marshal(map[string]any{"Dir": t.TempDir(), "Module": map[string]any{"Version": "v1.0.0"}})
	if _, err := compilationDirs(root, strings.NewReader(string(data)+string(external))); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "cmd", "corvint", "main.go")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "cmd", "corvint", "main.go")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := compilationDirs(root, strings.NewReader(string(data))); err == nil {
		t.Fatal("symbolic compilation input accepted")
	}
}
