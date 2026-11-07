// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	if err := os.WriteFile(filepath.Join(root, "bin", "getconf"), []byte("#!/bin/sh\necho 8\n"), 0700); err != nil {
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
