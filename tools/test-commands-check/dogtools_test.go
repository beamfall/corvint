// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"github.com/Beamfall/corvint/internal/execfile"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func dogtoolsScript(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "script", name))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), name)
	if err = execfile.WriteFile(p, body, 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDTLV0007ShardCoverageAndFailure(t *testing.T) {
	t.Run("DTL-V0-007", func(t *testing.T) {
		script := dogtoolsScript(t, "go-test-shard.sh")
		stub := `#!/usr/bin/env bash
	if [[ " $* " == *' -list '* ]]; then
	 printf '%s\n' TestA TestB TestC ExampleD FuzzE 'ok example/p 0.01s'
	else
	 echo "$*"
	 exit 7
	fi
	`
		if err := execfile.WriteFile(filepath.Join(filepath.Dir(script), "heavy.sh"), []byte(stub), 0755); err != nil {
			t.Fatal(err)
		}
		all := []string{"TestA", "TestB", "TestC", "ExampleD", "FuzzE"}
		seen := map[string]int{}
		for _, i := range []string{"1", "2", "3"} {
			c := exec.Command("bash", script, "--list-only", "./p/", i, "3")
			out, err := c.Output()
			if err != nil {
				t.Fatal(err)
			}
			pattern, err := regexp.Compile(strings.TrimSpace(string(out)))
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range all {
				if pattern.MatchString(name) {
					seen[name]++
				}
			}
		}
		for _, name := range all {
			if seen[name] != 1 {
				t.Fatalf("%s selected %d times", name, seen[name])
			}
		}
		// Fault injection: the selected command's failure must reach the caller.
		c := exec.Command("bash", script, "./p/", "1", "3")
		if err := c.Run(); err == nil || c.ProcessState.ExitCode() != 7 {
			t.Fatalf("lost failure: %v", err)
		}
		c = exec.Command("bash", script, "./p/", "0", "3")
		if err := c.Run(); err == nil || c.ProcessState.ExitCode() != 2 {
			t.Fatalf("bad shard admitted: %v", err)
		}
	})
}

func TestDTLV0012TrackedIgnoredFault(t *testing.T) {
	t.Run("DTL-V0-012", func(t *testing.T) {
		script := dogtoolsScript(t, "tracked-ignored-check.sh")
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "test"}, {"config", "user.email", "test@example.invalid"}} {
			c := exec.Command("git", args...)
			c.Dir = dir
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
		}
		run := func(want int) {
			t.Helper()
			c := exec.Command("bash", script)
			c.Dir = dir
			out, err := c.CombinedOutput()
			got := 0
			if err != nil {
				got = c.ProcessState.ExitCode()
			}
			if got != want {
				t.Fatalf("code=%d want=%d %s", got, want, out)
			}
		}
		run(0)
		if err := execfile.WriteFile(filepath.Join(dir, "tracked"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		c := exec.Command("git", "add", "tracked")
		c.Dir = dir
		if err := c.Run(); err != nil {
			t.Fatal(err)
		}
		if err := execfile.WriteFile(filepath.Join(dir, ".gitignore"), []byte("tracked\n"), 0644); err != nil {
			t.Fatal(err)
		}
		run(1)
		// Negative control: ignored untracked paths do not fail the gate.
		c = exec.Command("git", "rm", "--cached", "tracked")
		c.Dir = dir
		if err := c.Run(); err != nil {
			t.Fatal(err)
		}
		run(0)
	})
}

func TestDTLV0015FourDigitBlocks(t *testing.T) {
	t.Run("DTL-V0-015", func(t *testing.T) {
		script := dogtoolsScript(t, "requirement-ids-free.sh")
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "test"}, {"config", "user.email", "test@example.invalid"}} {
			c := exec.Command("git", args...)
			c.Dir = dir
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		p := filepath.Join(dir, "spec.md")
		if err := execfile.WriteFile(p, []byte("ABC-V0-2240 ABC-V0-2279 ABC-V0-2280\n"), 0644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "base"}} {
			c := exec.Command("git", args...)
			c.Dir = dir
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		c := exec.Command("bash", script, "ABC-V0", "2240", "2279", "HEAD")
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err == nil || c.ProcessState.ExitCode() != 1 || !strings.Contains(string(out), "used ABC-V0-2240") || !strings.Contains(string(out), "used ABC-V0-2279") || strings.Contains(string(out), "used ABC-V0-2280") {
			t.Fatalf("%v %s", err, out)
		}
	})
}

func TestDTLV0006TimingPreservesFailure(t *testing.T) {
	t.Run("DTL-V0-006", func(t *testing.T) {
		script := dogtoolsScript(t, "doc-gate-timed.sh")
		c := exec.Command("bash", script, "fixture", "sh", "-c", "exit 7")
		out, err := c.CombinedOutput()
		if err == nil || c.ProcessState.ExitCode() != 7 || !strings.Contains(string(out), "target=fixture seconds=") || !strings.Contains(string(out), "exit=7") {
			t.Fatalf("%v %s", err, out)
		}
	})
}

func TestDTLV0014DiskHeadroom(t *testing.T) {
	t.Run("DTL-V0-014", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join("..", "..", "script", "heavy.sh"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		extract := func(name string) string {
			t.Helper()
			start := strings.Index(text, "\n"+name+"() {")
			if start < 0 {
				t.Fatal(name)
			}
			part := text[start+1:]
			end := strings.Index(part, "\n}\n")
			if end < 0 {
				t.Fatal(name)
			}
			return part[:end+3] + "\n"
		}
		functions := extract("local_disk") + extract("tmpfs_defaults")
		run := func(tail string, want int) {
			t.Helper()
			script := functions + "\nselection_log(){ :; }; tmp_fstype(){ echo tmpfs; }; heavy_min_free=3; heavy_refuse_free=1;\n" + tail
			c := exec.Command("bash", "-c", script)
			out, err := c.CombinedOutput()
			got := 0
			if err != nil {
				got = c.ProcessState.ExitCode()
			}
			if got != want {
				t.Fatalf("code=%d want=%d err=%v %s", got, want, err, out)
			}
		}
		run(`free_gb(){ echo 10; }; GOCACHE=/roomy; TMPDIR=/roomy; GOTMPDIR=/roomy; local_disk`, 0)
		run(`free_gb(){ if [[ $1 == /tmp ]]; then echo 0; else echo 10; fi; }; GOCACHE=/roomy; TMPDIR=/roomy; GOTMPDIR=/roomy; local_disk`, 1)
		run(`free_gb(){ if [[ $1 == /small ]]; then echo 0; else echo 10; fi; }; GOCACHE=/roomy; TMPDIR=/roomy; GOTMPDIR=/small; local_disk`, 1)
		run(`unset TMPDIR GOCACHE CORVINT_GOCACHE; CORVINT_SCRATCH_ROOT=/roomy; tmpfs_defaults; [[ $TMPDIR == /roomy/corvint-tmp && $CORVINT_GOCACHE == /roomy/corvint-go-build-cache ]]`, 0)
	})
}

func TestDTLV0003WorktreeBuild(t *testing.T) {
	t.Run("DTL-V0-003", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join("..", "..", "script", "lane-corvint.sh"))
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		scripts := filepath.Join(root, "script")
		if err := os.Mkdir(scripts, 0700); err != nil {
			t.Fatal(err)
		}
		for name, text := range map[string]string{
			"lane-corvint.sh":      string(body),
			"lane-binary-check.sh": "#!/bin/sh\necho 'STALE installed build'\nexit 1\n",
			"heavy.sh":             "#!/bin/sh\n[ -z \"${CORVINT_DOGTOOLS_FAIL-}\" ] || exit 2\nwhile [ \"$1\" != -o ]; do shift; done\nshift\nprintf '#!/bin/sh\\necho worktree\\n' > \"$1\"\nchmod +x \"$1\"\n",
		} {
			if err := execfile.WriteFile(filepath.Join(scripts, name), []byte(text), 0755); err != nil {
				t.Fatal(err)
			}
		}
		c := exec.Command("git", "init", "-q", root)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		c = exec.Command("bash", filepath.Join(scripts, "lane-corvint.sh"))
		out, err := c.Output()
		if err != nil {
			t.Fatal(err)
		}
		binary := strings.TrimSpace(string(out))
		if binary != filepath.Join(root, ".git", "corvint", "lane-corvint") {
			t.Fatal(binary)
		}
		if got, err := exec.Command(binary).Output(); err != nil || strings.TrimSpace(string(got)) != "worktree" {
			t.Fatalf("%v %q", err, got)
		}
		c = exec.Command("bash", filepath.Join(scripts, "lane-corvint.sh"))
		c.Env = append(os.Environ(), "CORVINT_DOGTOOLS_FAIL=1")
		out, err = c.CombinedOutput()
		if err == nil || c.ProcessState.ExitCode() != 2 || !strings.Contains(string(out), "worktree-build-failed") {
			t.Fatalf("%v %s", err, out)
		}
	})
}

// dogtoolsGit runs git in dir and fails the test on error.
func dogtoolsGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.invalid"}, args...)...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// dogtoolsEnv is the test process environment without any HEAVY_* setting of the caller (the
// enclosing heavy.sh run sets HEAVY_WHOLE_REASON and HEAVY_HELD), plus extra.
func dogtoolsEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HEAVY_") || strings.HasPrefix(kv, "CORVINT_SCRATCH_ROOT=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// TestDTLV0015FourDigitOpenRefCollision: a four-digit ID that another open branch defines is
// reported by the block check's open-ref filter, with exact boundaries and three-digit IDs kept.
func TestDTLV0015FourDigitOpenRefCollision(t *testing.T) {
	t.Run("DTL-V0-015", func(t *testing.T) {
		script := dogtoolsScript(t, "requirement-id-collisions.sh")
		dir := t.TempDir()
		dogtoolsGit(t, dir, "init", "-q", "-b", "main")
		tsv := filepath.Join(dir, "docs", "specs", "REQUIREMENTS.tsv")
		if err := os.MkdirAll(filepath.Dir(tsv), 0755); err != nil {
			t.Fatal(err)
		}
		head := "id\tfile\ttitle\tacceptance\tacceptedBy\n"
		row := func(id string) string { return id + "\tdocs/specs/abc.md\t(t)\tstanding\tt\n" }
		write := func(text string) {
			t.Helper()
			if err := execfile.WriteFile(tsv, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			dogtoolsGit(t, dir, "add", ".")
			dogtoolsGit(t, dir, "commit", "-qm", "index")
		}
		write(head + row("ABC-V0-001"))
		dogtoolsGit(t, dir, "checkout", "-q", "-b", "other")
		write(head + row("ABC-V0-001") + row("ABC-V0-150") + row("ABC-V0-2240") + row("ABC-V0-2280"))
		dogtoolsGit(t, dir, "checkout", "-q", "main")
		check := func(lo, hi string, want int) string {
			t.Helper()
			c := exec.Command("bash", script, "--block", "ABC-V0", lo, hi, "--base", "main")
			c.Dir = dir
			out, err := c.CombinedOutput()
			got := 0
			if err != nil {
				got = c.ProcessState.ExitCode()
			}
			if got != want {
				t.Fatalf("block %s..%s: code=%d want=%d %s", lo, hi, got, want, out)
			}
			return string(out)
		}
		out := check("2240", "2279", 1)
		if !strings.Contains(out, "used ABC-V0-2240 other (") || strings.Contains(out, "ABC-V0-2280") {
			t.Fatalf("four-digit open-ref collision not named exactly: %s", out)
		}
		// Negative control: a block next to the open ref's IDs stays free.
		if out = check("2241", "2279", 0); !strings.Contains(out, "free ABC-V0-2241..ABC-V0-2279") {
			t.Fatal(out)
		}
		// Three-digit compatibility.
		if out = check("100", "199", 1); !strings.Contains(out, "used ABC-V0-150 other (") {
			t.Fatal(out)
		}
	})
}

// TestDTLV0014ScratchRootRecovery: the admission check before host choice uses the local
// defaults the run would apply, so a configured scratch root recovers a full home cache, while
// heavy.sh's own transport TMPDIR is still checked.
func TestDTLV0014ScratchRootRecovery(t *testing.T) {
	t.Run("DTL-V0-014", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join("..", "..", "script", "heavy.sh"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		var functions string
		for _, name := range []string{"writable_dir", "writable_paths", "local_disk", "tmpfs_defaults"} {
			start := strings.Index(text, "\n"+name+"() {")
			if start < 0 {
				t.Fatal(name)
			}
			part := text[start+1:]
			end := strings.Index(part, "\n}\n")
			if end < 0 {
				t.Fatal(name)
			}
			functions += part[:end+3] + "\n"
		}
		root := t.TempDir()
		full := filepath.Join(root, "full")
		roomy := filepath.Join(root, "roomy")
		run := func(tail string, want int) string {
			t.Helper()
			script := functions + `
selection_log() { :; }; heavy_exit() { exit "$1"; }; refuse() { echo "REFUSE $*"; exit 3; }
heavy_min_free=3; heavy_refuse_free=1
unset XDG_CACHE_HOME GOCACHE CORVINT_GOCACHE GOTMPDIR TMPDIR CORVINT_SCRATCH_ROOT
HOME=/dtl-full-home
lock_dir=` + filepath.Join(root, "locks") + `
FULL=` + full + ` ROOMY=` + roomy + `
free_gb() { case $1 in /dtl-full-home/* | "$FULL" | "$FULL"/*) echo 0 ;; *) echo 10 ;; esac; }
` + tail
			c := exec.Command("bash", "-c", script)
			c.Env = dogtoolsEnv()
			out, err := c.CombinedOutput()
			got := 0
			if err != nil {
				got = c.ProcessState.ExitCode()
			}
			if got != want {
				t.Fatalf("code=%d want=%d err=%v %s", got, want, err, out)
			}
			return string(out)
		}
		// Recovery: full home cache, roomy /tmp and a roomy configured scratch root admit, and
		// nothing is exported or announced before the host choice.
		out := run(`tmp_fstype() { echo tmpfs; }; export CORVINT_SCRATCH_ROOT=$ROOMY
writable_paths control env GOTOOLCHAIN=local go test ./x
[[ -z ${TMPDIR-} && -z ${CORVINT_GOCACHE-} ]]`, 0)
		if strings.Contains(out, "this run uses") {
			t.Fatalf("defaults announced before host choice: %s", out)
		}
		// A non-tmpfs host's cache default derives from TMPDIR, not the user cache.
		run(`tmp_fstype() { echo ext4; }; export TMPDIR=$ROOMY/t; writable_paths control go test ./x`, 0)
		// Negative control: without a scratch root a tmpfs host defaults to the full user cache.
		if out = run(`tmp_fstype() { echo tmpfs; }; writable_paths control go test ./x`, 2); !strings.Contains(out, "/dtl-full-home/.cache/corvint-go-build-cache free=0G") {
			t.Fatal(out)
		}
		// Fault injection: a full scratch root refuses.
		run(`tmp_fstype() { echo tmpfs; }; export CORVINT_SCRATCH_ROOT=$FULL; writable_paths control go test ./x`, 2)
		// Transport stays separate: argv overrides do not hide a full heavy.sh TMPDIR.
		if out = run(`tmp_fstype() { echo ext4; }; mkdir -p "$FULL"; export TMPDIR=$FULL
writable_paths control env TMPDIR=$ROOMY GOCACHE=$ROOMY/c go test ./x`, 2); !strings.Contains(out, full+" free=0G") {
			t.Fatal(out)
		}
	})
}

// TestDTLV0014RelativeScratchRootTerminates: a relative CORVINT_SCRATCH_ROOT that does not exist
// yet is resolved against $PWD, and the real free_gb walk stops instead of looping on a path whose
// parent never changes. The bounded run fails on a hang.
func TestDTLV0014RelativeScratchRootTerminates(t *testing.T) {
	t.Run("DTL-V0-014", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join("..", "..", "script", "heavy.sh"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		var functions string
		for _, name := range []string{"writable_dir", "writable_paths", "local_disk", "tmpfs_defaults", "free_gb"} {
			start := strings.Index(text, "\n"+name+"() {")
			if start < 0 {
				t.Fatal(name)
			}
			part := text[start+1:]
			end := strings.Index(part, "\n}\n")
			if end < 0 {
				t.Fatal(name)
			}
			functions += part[:end+3] + "\n"
		}
		work := t.TempDir()
		script := functions + `
selection_log() { :; }; heavy_exit() { exit "$1"; }; refuse() { echo "REFUSE $*"; exit 3; }
tmp_fstype() { echo tmpfs; }
heavy_min_free=0; heavy_refuse_free=0
unset XDG_CACHE_HOME GOCACHE CORVINT_GOCACHE GOTMPDIR TMPDIR
export CORVINT_SCRATCH_ROOT=lane-scratch
lock_dir=` + filepath.Join(work, "locks") + `
[[ $(free_gb lane-scratch/x) =~ ^[0-9]+$ ]] || { echo "free_gb printed no size"; exit 4; }
[[ $(free_gb missing) =~ ^[0-9]+$ ]] || { echo "free_gb printed no size for a bare name"; exit 4; }
writable_paths control env GOTOOLCHAIN=local go test ./x
# An argv TMPDIR override that is a missing bare name reaches the same walk.
writable_paths control env TMPDIR=rel-tmp go test ./x
tmpfs_defaults 2> /dev/null
[[ $TMPDIR == "$PWD/lane-scratch/corvint-tmp" && $CORVINT_GOCACHE == "$PWD/lane-scratch/corvint-go-build-cache" ]] || { echo "unresolved TMPDIR=$TMPDIR CORVINT_GOCACHE=$CORVINT_GOCACHE"; exit 5; }
`
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, "bash", "-c", script)
		c.Dir, c.Env, c.WaitDelay = work, dogtoolsEnv(), 2*time.Second
		out, err := c.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("relative scratch root hung the disk check: %s", out)
		}
		if err != nil {
			t.Fatalf("%v %s", err, out)
		}
	})
}

// TestDTLV0007ShardInventoryRealPolicy: the documented shard command's inventory call passes the
// real heavy.sh selection policy without HEAVY_WHOLE_REASON or a manifest, and runs no test;
// an empty or post-args -list stays under the whole-package policy.
func TestDTLV0007ShardInventoryRealPolicy(t *testing.T) {
	t.Run("DTL-V0-007", func(t *testing.T) {
		ran := filepath.Join(t.TempDir(), "ran")
		stub := "#!/bin/sh\n[ \"$1\" = test ] || exit 0\ncase \" $* \" in *' -list '*) printf '%s\\n' TestA TestB ExampleC 'ok example/p 0.01s' ;; *) echo \"$*\" >> " + ran + " ;; esac\n"
		root, scripts, work, env := dogtoolsShardFixture(t, stub)
		selection := filepath.Join(root, "selection.log")
		c := exec.Command("bash", filepath.Join(scripts, "go-test-shard.sh"), "--list-only", "./p/", "1", "2")
		c.Dir, c.Env = work, env
		out, err := c.Output()
		if err != nil {
			stderr := ""
			if e, ok := err.(*exec.ExitError); ok {
				stderr = string(e.Stderr)
			}
			t.Fatalf("documented inventory refused: %v %s", err, stderr)
		}
		if got := strings.TrimSpace(string(out)); got != "^(ExampleC|TestB)$" {
			t.Fatalf("pattern %q", got)
		}
		log, err := os.ReadFile(selection)
		if err != nil || !strings.Contains(string(log), "whole reason=list-only") {
			t.Fatalf("list-only admission not logged: %v %s", err, log)
		}
		if _, err := os.Stat(ran); !os.IsNotExist(err) {
			t.Fatalf("the inventory ran tests: %v", err)
		}
		// Negative controls: an empty -list, a -list after -args, or -exec runs tests and is refused.
		for _, args := range [][]string{
			{"go", "test", "-list", "", "./p/"},
			{"go", "test", "-list", "^Test", "-list=", "./p/"},
			{"go", "test", "./p/", "-args", "-list", "^Test"},
			{"go", "test", "-list", "^Test", "-exec", "env", "./p/"},
		} {
			c = exec.Command("bash", append([]string{filepath.Join(scripts, "heavy.sh")}, args...)...)
			c.Dir, c.Env = work, env
			out, err := c.CombinedOutput()
			if err == nil || c.ProcessState.ExitCode() != 2 || !strings.Contains(string(out), "whole-package go test without HEAVY_WHOLE_REASON") {
				t.Fatalf("%q admitted: %v %s", args, err, out)
			}
		}
		if _, err := os.Stat(ran); !os.IsNotExist(err) {
			t.Fatalf("a refused command ran: %v", err)
		}
	})
}

// dogtoolsShardFixture: a real heavy.sh and go-test-shard.sh under ROOT/script with STUB as the
// go on PATH, a work directory holding ./p/, and an environment with no offload.
func dogtoolsShardFixture(t *testing.T, stub string) (root, scripts, work string, env []string) {
	t.Helper()
	root = t.TempDir()
	scripts = filepath.Join(root, "script")
	bin := filepath.Join(root, "bin")
	work = filepath.Join(root, "work")
	for _, d := range []string{scripts, bin, filepath.Join(work, "p")} {
		if err := os.MkdirAll(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"heavy.sh", "go-test-shard.sh"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "script", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = execfile.WriteFile(filepath.Join(scripts, name), body, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	env = dogtoolsEnv("PATH="+bin+":"+os.Getenv("PATH"), "HEAVY_CONFIG="+filepath.Join(root, "no-config"),
		"HEAVY_LOCK_DIR="+filepath.Join(root, "locks"), "HEAVY_SELECTION_LOG="+filepath.Join(root, "selection.log"),
		"HEAVY_FALLBACK_LOG="+filepath.Join(root, "fallback.log"), "HEAVY_SLOTS=1", "HEAVY_MAX_LOAD=999999",
		"HEAVY_MIN_FREE_GB=0", "HEAVY_REFUSE_FREE_GB=0", "HEAVY_CACHE_TRIM_HOURS=0", "HEAVY_GO_TEST_TIMEOUT=off",
		"HEAVY_TMP_FSTYPE=disk", "HEAVY_CI_REPO=off", "TMPDIR="+root, "GOCACHE="+filepath.Join(root, "gocache"), "GOFLAGS=")
	return root, scripts, work, env
}

// dogtoolsGoflagsStub: a go that answers `go env GOFLAGS` and applies GOFLAGS words before the
// command-line flags, the last value of -list/-test.list, -skip/-test.skip and -run/-test.run
// winning, as the go command does. A non-empty list prints matching names; otherwise each run
// name is appended to RAN with the effective -tags value.
func dogtoolsGoflagsStub(ran string) string {
	return `#!/usr/bin/env bash
if [[ $1 == env ]]; then [[ ${2-} == GOFLAGS ]] && printf '%s\n' "${GOFLAGS-}"; exit 0; fi
[[ $1 == test ]] || exit 0
shift
list= skip= run= tags=
set -- ${GOFLAGS-} "$@"
while (($#)); do
  a=$1; shift
  case $a in --*) a=${a#-} ;; esac
  case $a in -list | -test.list | -skip | -test.skip | -run | -test.run | -tags | -count | -timeout) a="$a=$1"; shift ;; esac
  case $a in
    -list=* | -test.list=*) list=${a#*=} ;;
    -skip=* | -test.skip=*) skip=${a#*=} ;;
    -run=* | -test.run=*) run=${a#*=} ;;
    -tags=*) tags=${a#*=} ;;
  esac
done
if [[ -n $list ]]; then for n in TestA TestB ExampleC; do [[ $n =~ $list ]] && echo "$n"; done; echo 'ok example/p 0.01s'; exit 0; fi
for n in TestA TestB ExampleC; do
  [[ -n $run ]] && ! [[ $n =~ $run ]] && continue
  [[ -n $skip && $n =~ $skip ]] && continue
  echo "$n tags=$tags" >> ` + ran + `
done
`
}

// TestDTLV0007InheritedGoflagsKeepCoverage: selection flags inherited through GOFLAGS (-skip,
// -list, -test.skip) neither drop a name from the inventory nor stop a shard from running it,
// a build setting (-tags) still reaches every shard, and an unsupported inherited flag is
// refused as inherited-goflags-unsupported before anything runs.
func TestDTLV0007InheritedGoflagsKeepCoverage(t *testing.T) {
	t.Run("DTL-V0-007", func(t *testing.T) {
		for _, goflags := range []string{"-skip=TestA", "-list=Test", "-test.skip=TestA", "-tags=x -skip=TestA", "-tags=x --test.list=Test"} {
			ran := filepath.Join(t.TempDir(), "ran")
			_, scripts, work, env := dogtoolsShardFixture(t, dogtoolsGoflagsStub(ran))
			env = append(env, "GOFLAGS="+goflags)
			for _, i := range []string{"1", "2"} {
				c := exec.Command("bash", filepath.Join(scripts, "go-test-shard.sh"), "./p/", i, "2")
				c.Dir, c.Env = work, env
				if out, err := c.CombinedOutput(); err != nil {
					t.Fatalf("GOFLAGS=%s shard %s: %v %s", goflags, i, err, out)
				}
			}
			body, _ := os.ReadFile(ran)
			wantTags := ""
			if strings.Contains(goflags, "-tags=x") {
				wantTags = "x"
			}
			got := map[string]bool{}
			for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
				name, tags, _ := strings.Cut(line, " tags=")
				if tags != wantTags {
					t.Fatalf("GOFLAGS=%s: build setting lost: %q", goflags, line)
				}
				got[name] = true
			}
			if len(got) != 3 || !got["TestA"] || !got["TestB"] || !got["ExampleC"] {
				t.Fatalf("GOFLAGS=%s: shards ran %q, want TestA TestB ExampleC", goflags, body)
			}
		}
		// Negative control: an inherited flag outside the supported set is refused, nothing runs.
		for _, goflags := range []string{"-toolexec=/bin/true", "-bench=.", "-tags=x -o=/dev/null"} {
			ran := filepath.Join(t.TempDir(), "ran")
			_, scripts, work, env := dogtoolsShardFixture(t, dogtoolsGoflagsStub(ran))
			c := exec.Command("bash", filepath.Join(scripts, "go-test-shard.sh"), "./p/", "1", "2")
			c.Dir, c.Env = work, append(env, "GOFLAGS="+goflags)
			out, err := c.CombinedOutput()
			if err == nil || c.ProcessState.ExitCode() != 2 || !strings.Contains(string(out), "go-test-shard: REFUSE inherited-goflags-unsupported") {
				t.Fatalf("GOFLAGS=%s admitted: %v %s", goflags, err, out)
			}
			if _, err := os.Stat(ran); !os.IsNotExist(err) {
				t.Fatalf("GOFLAGS=%s: a refused shard ran: %v", goflags, err)
			}
		}
	})
}

// TestDTLV0007ListAliasAndArgsRefused: list-only admission refuses a test.-prefixed alias, any
// argument after -args, and a non-empty -exec on the command line or in GOFLAGS; a -run selection
// is not admitted when a test.run alias can override it. The shard commands, with their empty
// neutralizing values, stay admitted.
func TestDTLV0007ListAliasAndArgsRefused(t *testing.T) {
	t.Run("DTL-V0-007", func(t *testing.T) {
		ran := filepath.Join(t.TempDir(), "ran")
		stub := "#!/bin/sh\n[ \"$1\" = test ] || exit 0\necho \"$*\" >> " + ran + "\n"
		_, scripts, work, env := dogtoolsShardFixture(t, stub)
		heavy := filepath.Join(scripts, "heavy.sh")
		for _, tc := range []struct {
			goflags string
			args    []string
		}{
			{"", []string{"go", "test", "-list", "Test", "-test.list=", "./p/"}},
			{"", []string{"go", "test", "-list", "Test", "--test.list", "", "./p/"}},
			{"", []string{"go", "test", "-list", "Test", "./p/", "-args", "-test.list="}},
			{"", []string{"go", "test", "-list", "Test", "./p/", "-args", "-v"}},
			{"", []string{"go", "test", "-list", "Test", "-exec=env", "./p/"}},
			{"", []string{"env", "GOFLAGS=-exec=env", "go", "test", "-list", "Test", "./p/"}},
			{"-exec=env", []string{"go", "test", "-list", "Test", "./p/"}},
			{"", []string{"go", "test", "-run", "TestA", "-test.run=", "./p/"}},
			{"", []string{"go", "test", "-run", "TestA", "./p/", "-args", "-test.run", ""}},
			// go passes a -- to the test binary, which stops parsing flags there.
			{"", []string{"go", "test", "./p/", "--", "-list=TestA"}},
			{"", []string{"go", "test", "-list", "Test", "./p/", "--"}},
			{"", []string{"go", "test", "./p/", "--", "-run", "TestA"}},
			{"", []string{"go", "test", "-run", "TestA", "./p/", "--"}},
		} {
			c := exec.Command("bash", append([]string{heavy}, tc.args...)...)
			c.Dir, c.Env = work, append(append([]string{}, env...), "GOFLAGS="+tc.goflags)
			out, err := c.CombinedOutput()
			if err == nil || c.ProcessState.ExitCode() != 2 || !strings.Contains(string(out), "whole-package go test without HEAVY_WHOLE_REASON") {
				t.Fatalf("GOFLAGS=%q %q admitted: %v %s", tc.goflags, tc.args, err, out)
			}
			// The -run refusal names the -- cause the spec lists.
			if strings.Join(tc.args, " ") == "go test -run TestA ./p/ --" && !strings.Contains(string(out), "a -- argument ends the test binary flags") {
				t.Fatalf("-- refusal does not name its cause: %s", out)
			}
		}
		if _, err := os.Stat(ran); !os.IsNotExist(err) {
			t.Fatalf("a refused command ran: %v", err)
		}
		// Positive controls: the shard inventory and execution forms, and a -run with ordinary
		// binary arguments, are still admitted.
		// An inherited -exec that the command line's empty -exec= overrides is neutral.
		inventory := []string{"go", "test", "-count=1", "-skip=", "-short=false", "-c=false", "-n=false", "-fuzz=", "-exec=", "-list", "^(Test|Example|Fuzz)", "./p/"}
		for _, tc := range []struct {
			goflags string
			args    []string
		}{
			{"", inventory},
			{"-exec=env", inventory},
			{"", []string{"go", "test", "-count=1", "-skip=", "-short=false", "-c=false", "-n=false", "-fuzz=", "-exec=", "-list=", "-run", "^(TestA)$", "./p/"}},
			{"", []string{"go", "test", "-run", "TestA", "./p/", "-args", "-update"}},
		} {
			c := exec.Command("bash", append([]string{heavy}, tc.args...)...)
			c.Dir, c.Env = work, append(append([]string{}, env...), "GOFLAGS="+tc.goflags)
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("GOFLAGS=%q %q refused: %v %s", tc.goflags, tc.args, err, out)
			}
		}
	})
}
