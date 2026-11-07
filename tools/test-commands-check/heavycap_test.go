// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

func heavycapEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return append(out, key+"="+value)
}

func TestASDV02477LocalRuntimeShare(t *testing.T) {
	root, scripts, work, env := dogtoolsShardFixture(t, "#!/bin/sh\n[ \"$1\" = env ] && exit 0\nprintf 'inherited=%s\\n' \"$GOMAXPROCS\"\n")
	// Remove the enclosing test runner's cap to exercise the local default.
	if err := execfile.WriteFile(filepath.Join(root, "bin", "getconf"), []byte("#!/bin/sh\necho 8\n"), 0700); err != nil {
		t.Fatal(err)
	}
	env = heavycapEnv(env, "HEAVY_SLOTS", "2")
	var clean []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, "GOMAXPROCS=") {
			clean = append(clean, kv)
		}
	}
	for _, tc := range []struct {
		args                  []string
		override, share, want string
	}{
		{[]string{"go", "test", "-run", "TestA", "./p"}, "", "", "4"},
		{[]string{"go", "test", "-run", "TestA", "./p"}, "7", "", "7"},
		{[]string{"go", "test", "-run", "TestA", "./p"}, "", "3", "3"},
		{[]string{"GOMAXPROCS=9", "go", "test", "-run", "TestA", "./p"}, "", "", "9"},
		{[]string{"env", "GOMAXPROCS=11", "go", "test", "-run", "TestA", "./p"}, "7", "", "11"},
		{[]string{"--admit-only", "go", "test", "-run", "TestA", "./p"}, "", "", "4"},
	} {
		cmd := exec.Command("bash", append([]string{filepath.Join(scripts, "heavy.sh")}, tc.args...)...)
		cmd.Dir, cmd.Env = work, clean
		if tc.override != "" {
			cmd.Env = heavycapEnv(cmd.Env, "GOMAXPROCS", tc.override)
		}
		if tc.share != "" {
			cmd.Env = heavycapEnv(cmd.Env, "HEAVY_P", tc.share)
		}
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "inherited="+tc.want+"\n") {
			t.Fatalf("%v override=%s: %v %s", tc.args, tc.override, err, out)
		}
	}
}

// ASD-V0-2478: source changes must reach old tests, including an existing test broken by the edit.
func TestASDV02478SourceEditRunsExistingTests(t *testing.T) {
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	_, scripts, work, env := dogtoolsShardFixture(t, "#!/bin/sh\nexec \"$HEAVYCAP_GO\" \"$@\"\n")
	env = heavycapEnv(env, "HEAVYCAP_GO", realGo)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(work, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.invalid/fixture\n\ngo 1.24.0\n")
	write("p/a.go", "package p\nfunc Value() int { return 1 }\n")
	write("p/a_test.go", "package p\nimport \"testing\"\nfunc TestExisting(t *testing.T) { if Value()!=1 { t.Fatal(\"source regression\") } }\nfunc TestOther(t *testing.T) {}\n")
	dogtoolsGit(t, work, "init", "-q")
	dogtoolsGit(t, work, "add", ".")
	dogtoolsGit(t, work, "commit", "-qm", "base")
	dogtoolsGit(t, work, "update-ref", "refs/remotes/origin/main", "HEAD")
	run := func(want int, args ...string) string {
		t.Helper()
		cmd := exec.Command("bash", append([]string{filepath.Join(scripts, "heavy.sh")}, args...)...)
		cmd.Dir, cmd.Env = work, env
		out, err := cmd.CombinedOutput()
		got := 0
		if err != nil {
			got = cmd.ProcessState.ExitCode()
		}
		if got != want {
			t.Fatalf("%v exit=%d want=%d: %v %s", args, got, want, err, out)
		}
		return string(out)
	}
	run(2, "go", "test", "./p") // unchanged is still refused
	write("p/a_test.go", "package p\nimport \"testing\"\nfunc TestExisting(t *testing.T) { if Value()!=1 { t.Fatal(\"source regression\") } }\nfunc TestOther(t *testing.T) { t.Log(\"changed\") }\n")
	run(2, "go", "test", "./p") // a test-only edit is not whole-package authority
	out := run(0, "go", "test", "-v", "-run", "^TestOther$", "./p")
	if !strings.Contains(out, "--- PASS: TestOther") || strings.Contains(out, "=== RUN   TestExisting") {
		t.Fatal(out)
	}
	write("p/a.go", "package p\nfunc Value() int { return 2 }\n")
	out = run(1, "go", "test", "-count=1", "./p")
	if !strings.Contains(out, "source regression") {
		t.Fatalf("existing failure not reached: %s", out)
	}
	// A source edit cannot justify excluding the old test it breaks, including aliases and
	// inherited flags. Refusal must precede execution, rather than report a successful whole run.
	for _, args := range [][]string{
		{"go", "test", "-skip=TestExisting", "./p"},
		{"go", "test", "-skip", "TestExisting", "./p"},
		{"go", "test", "--skip=TestExisting", "./p"},
		{"go", "test", "-test.skip=TestExisting", "./p"},
		{"go", "test", "--test.skip=TestExisting", "./p"},
		{"go", "test", "-count", "0", "./p"},
		{"go", "test", "-count=0", "./p"},
		{"env", "GOFLAGS=-skip=TestExisting", "go", "test", "./p"},
		{"env", "GOFLAGS=-p=1\n-skip=TestExisting", "go", "test", "./p"},
		{"env", "GOFLAGS=-test.skip=TestExisting", "go", "test", "./p"},
		{"env", "GOFLAGS=-run=TestOther", "go", "test", "./p"},
		{"env", "GOFLAGS=-count=0", "go", "test", "./p"},
	} {
		out = run(2, args...)
		if !strings.Contains(out, "REFUSED whole-reason-missing") || strings.Contains(out, "ok \t") {
			t.Fatalf("filtered source run was not refused before execution: %s", out)
		}
	}
	// Neither the small-package nor the new-package shortcut may bypass exclusion validation.
	for _, pkg := range []string{"tools/small", "new"} {
		if err := os.MkdirAll(filepath.Join(work, pkg), 0700); err != nil {
			t.Fatal(err)
		}
		write(pkg+"/a.go", "package p\nfunc Value() int { return 2 }\n")
		write(pkg+"/a_test.go", "package p\nimport \"testing\"\nfunc TestExisting(t *testing.T) { if Value()!=1 { t.Fatal(\"source regression\") } }\n")
		run(2, "go", "test", "-skip=TestExisting", "./"+pkg)
	}
	originalEnv := env
	env = heavycapEnv(env, "GOFLAGS", "-skip=TestExisting")
	run(2, "go", "test", "./p")
	env = originalEnv
	goenv := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(goenv, []byte("GOFLAGS=-skip=TestExisting\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(2, "env", "GOENV="+goenv, "go", "test", "./p")
	// Unsetting an exported value exposes GOENV defaults in the actual child. The guard
	// must not authorize a run using the still-exported value instead.
	env = heavycapEnv(env, "GOFLAGS", "-p=1")
	child := exec.Command("env", "-u", "GOFLAGS", "GOENV="+goenv, realGo, "env", "GOFLAGS")
	child.Dir, child.Env = work, env
	resolved, err := child.CombinedOutput()
	if err != nil || strings.TrimSpace(string(resolved)) != "-skip=TestExisting" {
		t.Fatalf("unset fixture GOFLAGS=%q: %v", resolved, err)
	}
	run(2, "env", "-u", "GOFLAGS", "GOENV="+goenv, "go", "test", "./p")
	run(2, "env", "--unset=GOFLAGS", "GOENV="+goenv, "go", "test", "./p")
	env = originalEnv
	// These patterns run the existing failure in all three automatic admission routes.
	for _, pkg := range []string{"p", "tools/small", "new"} {
		for _, pattern := range []string{".", ".*", "^.+", ""} {
			out = run(1, "go", "test", "-count=1", "-run", pattern, "./"+pkg)
			if !strings.Contains(out, "source regression") {
				t.Fatalf("all-matching run did not reach existing failure: %s", out)
			}
		}
		run(1, "go", "test", "--run", ".", "./"+pkg)
	}
	out = run(1, "env", "GOFLAGS=-run=.", "go", "test", "-count=1", "./p")
	if !strings.Contains(out, "source regression") {
		t.Fatalf("inherited all-matching run did not reach existing failure: %s", out)
	}
	// The final repeated -run is effective; a restrictive inherited pattern stays refused.
	run(1, "go", "test", "-run=TestOther", "-run=.", "./p")
	run(2, "env", "GOFLAGS=-run=TestOther", "go", "test", "-run=.", "./p")

	// A HEAD-bound manifest must select the exact exclusion; a whole-unit witness is not
	// authority for -skip. A supported exclusion still reaches the broken TestExisting.
	gitHead := exec.Command("git", "rev-parse", "HEAD")
	gitHead.Dir = work
	headBytes, err := gitHead.Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headBytes))
	write("selection.json", `{"profile":"corvint-test-manifest/0","source":{"head":"`+head+`"},"units":[{"unit":"go:example.invalid/fixture/p","slice":"whole"},{"unit":"go:example.invalid/fixture/p","slice":"1/2","flag":"-skip","pattern":"TestOther"}]}`)
	env = heavycapEnv(env, "HEAVY_SELECTION", filepath.Join(work, "selection.json"))
	// With source admission disabled, the unchanged package must use the whole-unit manifest.
	dogtoolsGit(t, work, "update-ref", "refs/remotes/origin/main", "HEAD")
	dogtoolsGit(t, work, "checkout", "--", "p/a.go")
	for _, args := range [][]string{
		{"go", "test", "-count=1", "-v", "-run", ".", "./p"},
		{"go", "test", "-count=1", "-v", "--run=.*", "./p"},
		{"go", "test", "-count=1", "-v", "--run", ".", "./p"},
		{"env", "GOFLAGS=-run=.", "go", "test", "-count=1", "-v", "./p"},
	} {
		out = run(0, args...)
		if !strings.Contains(out, "=== RUN   TestExisting") || !strings.Contains(out, "=== RUN   TestOther") {
			t.Fatalf("whole manifest run omitted a test: %s", out)
		}
	}
	write("p/a.go", "package p\nfunc Value() int { return 2 }\n")
	run(2, "go", "test", "-skip=TestExisting", "./p")
	run(2, "env", "GOFLAGS=-skip=TestExisting", "go", "test", "./p")
	run(2, "go", "test", "-run=.", "-skip=TestExisting", "./p")
	env = heavycapEnv(env, "GOFLAGS", "-p=1")
	run(2, "env", "-u", "GOFLAGS", "GOENV="+goenv, "go", "test", "-run=.", "./p")
	env = heavycapEnv(env, "GOFLAGS", "")
	out = run(1, "go", "test", "-count=1", "-skip=TestOther", "./p")
	if !strings.Contains(out, "source regression") {
		t.Fatalf("manifest exclusion hid the existing failure: %s", out)
	}
	env = originalEnv
	dogtoolsGit(t, work, "add", ".") // staged source changes qualify too
	out = run(1, "go", "test", "-count=1", "./p")
	if !strings.Contains(out, "source regression") {
		t.Fatal(out)
	}
	dogtoolsGit(t, work, "commit", "-qm", "source regression")
	out = run(1, "go", "test", "-count=1", "./p") // committed lane diff still qualifies
	if !strings.Contains(out, "source regression") {
		t.Fatal(out)
	}
	// A new source file in an existing package is also a source change, even before staging.
	dogtoolsGit(t, work, "update-ref", "refs/remotes/origin/main", "HEAD")
	run(2, "go", "test", "./p")
	write("p/new.go", "package p\nvar NewSource = true\n")
	out = run(1, "go", "test", "-count=1", "./p")
	if !strings.Contains(out, "source regression") {
		t.Fatal(out)
	}
	// Pre-commit callers may work in the package directory and name it as '.'.
	oldWork := work
	work = filepath.Join(work, "p")
	out = run(1, "go", "test", "-count=1", ".")
	work = oldWork
	if !strings.Contains(out, "source regression") {
		t.Fatal(out)
	}

	run(2, "go", "test", "./...")
	run(2, "go", "test", "./p", "./missing")
}
