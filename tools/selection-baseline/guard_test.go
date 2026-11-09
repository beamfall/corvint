// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// TestTSPV07330GuardDecidesTheGithubScope_TSPV07330 holds the baseline to CI's .github rule:
// the guard's `0` narrows, its `1` names the inputs, and anything else fails closed.
func TestTSPV07330GuardDecidesTheGithubScope_TSPV07330(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the guard and its stubs are sh scripts")
	ctx := context.Background()
	dir := t.TempDir()
	for _, tc := range []struct {
		name, body string
		changed    bool
		inputs     []string
	}{
		{"no input", "echo 0\n", false, nil},
		{"inputs", "printf '1\\n.github/forward-rows/narrow.sh\\n'\n", true, []string{".github/forward-rows/narrow.sh"}}, //corvint:fixture-path
		{"unreadable", "printf 'unreadable\\nno base\\n'\n", true, []string{"unreadable: unreadable; no base"}},
		{"one without inputs", "echo 1\n", true, []string{"unreadable: 1"}},
		{"empty", ":\n", true, []string{"unreadable: "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".sh")
			if err := os.WriteFile(guard, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			changed, inputs := githubGuard(ctx, dir, guard, "b", "h")
			if changed != tc.changed || !reflect.DeepEqual(inputs, tc.inputs) {
				t.Fatalf("githubGuard = %v %q, want %v %q", changed, inputs, tc.changed, tc.inputs)
			}
		})
	}
	failing := filepath.Join(dir, "exit.sh")
	if err := os.WriteFile(failing, []byte("exit 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, inputs := githubGuard(ctx, dir, failing, "b", "h"); !changed || len(inputs) != 1 || !strings.HasPrefix(inputs[0], "unreadable: ") {
		t.Fatalf("failing guard = %v %q, want a fail-closed change", changed, inputs)
	}
	if changed, _ := githubGuard(ctx, dir, filepath.Join(dir, "absent.sh"), "b", "h"); !changed {
		t.Fatal("a missing guard narrowed")
	}
}

// TestTSPV07330RealGuardNarrowsAnUnguardedWorkflow_TSPV07330 runs this repository's own guard,
// the copy replay reads, over a real repository: another workflow narrows and a narrowing script
// forces FULL, as CI decides (AFP-V0-2720).
func TestTSPV07330RealGuardNarrowsAnUnguardedWorkflow_TSPV07330(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the guard is an sh script")
	guard, err := filepath.Abs(filepath.Join("..", "..", filepath.FromSlash(guardPath)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(guard); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(path, body string) string {
		t.Helper()
		name := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-qm", path)
		return git("rev-parse", "HEAD")
	}
	git("init", "-q")
	base := commit(".github/forward-rows/narrow.sh", "echo base\n")   //corvint:fixture-path
	other := commit(".github/workflows/other.yml", "name: other\n")   //corvint:fixture-path
	narrow := commit(".github/forward-rows/narrow.sh", "echo head\n") //corvint:fixture-path
	ctx := context.Background()
	if changed, inputs := githubGuard(ctx, repo, guard, base, other); changed {
		t.Fatalf("an unguarded workflow forced FULL: %q", inputs)
	}
	if changed, inputs := githubGuard(ctx, repo, guard, other, narrow); !changed || !reflect.DeepEqual(inputs, []string{".github/forward-rows/narrow.sh"}) { //corvint:fixture-path
		t.Fatalf("a narrowing script = %v %q, want FULL naming it", changed, inputs)
	}
}

// TestTSPV07330MissingGuardKeepsTheReport_TSPV07330: a replaying checkout without the guard still
// reports every change, each failing closed as a .github change with an `unreadable:` input.
func TestTSPV07330MissingGuardKeepsTheReport_TSPV07330(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the fixture planner wrapper is an sh script")
	root := t.TempDir()
	baselineFile(t, root, "go.mod", "module m\n")
	baselineFile(t, root, ".github/cishards/package-costs.json", `{"milliseconds":{"m/a":1}}`)
	baselineGit(t, root, "init", "-q")
	baselineGit(t, root, "add", ".")
	baselineGit(t, root, "commit", "-qm", "base")
	base := baselineGit(t, root, "rev-parse", "HEAD")
	baselineFile(t, root, "id", "only")
	baselineGit(t, root, "add", "id")
	baselineGit(t, root, "commit", "-qm", "only")
	set := changeSet{Profile: changesProfile, Changes: []change{{ID: "only", Kind: "merged", Base: base, Head: baselineGit(t, root, "rev-parse", "HEAD")}}}
	testbin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "planner")
	body := "#!/bin/sh\nexec '" + strings.ReplaceAll(testbin, "'", "'\\''") + "' -test.run=^TestBaselineReplayHelper$ -- \"$@\"\n"
	if err := execfile.WriteFile(bin, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SB_BASELINE_HELPER", "1")
	t.Setenv("SB_BASELINE_ORDER_DIR", t.TempDir())
	t.Setenv("SELECTION_BASELINE_JOBS", "1")
	t.Setenv("SELECTION_BASELINE_CACHE", "off")
	result, err := replay(context.Background(), root, set, "changes.json", bin, bin)
	if err != nil {
		t.Fatalf("replay without a guard = %v, want a report", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(result.Results))
	}
	res := result.Results[0]
	if !res.GithubChange || len(res.GithubInputs) != 1 || !strings.HasPrefix(res.GithubInputs[0], "unreadable:") {
		t.Fatalf("missing guard = %v %q, want a change with an unreadable: input", res.GithubChange, res.GithubInputs)
	}
}

// TSP-V0-7330: a result whose guard could not run is not recorded in the cache.
func TestUnreadableGuardResultIsNotCached(t *testing.T) {
	if cacheable(Result{GithubChange: true, GithubInputs: []string{"unreadable: sh not found"}}) {
		t.Fatal("an unreadable: guard result is cacheable, want it measured again next run")
	}
	if !cacheable(Result{GithubChange: true, GithubInputs: []string{".github/workflows/ci.yml"}}) || !cacheable(Result{}) {
		t.Fatal("a guard result that ran is not cacheable")
	}
}
