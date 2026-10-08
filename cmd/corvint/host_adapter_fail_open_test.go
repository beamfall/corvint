package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// failOpenInvocation is one shipped hook command: what the host executes and the hook input it
// sends for that event.
type failOpenInvocation struct {
	name    string
	argv    []string
	payload func(root string) map[string]any
	// usesIndex marks an invocation that reads the context index, so an unavailable index must
	// surface as a named degradation rather than a silent change of packet.
	usesIndex bool
}

// failOpenCase is one fault the host or the machine can impose on an invocation.
type failOpenCase struct {
	name string
	// stdin is what the host writes; nil means the valid hook payload.
	stdin []byte
	// holdStdin keeps stdin open after the payload, as a host that never closes it would.
	holdStdin bool
	// closeStdout closes the read end of stdout before the adapter writes.
	closeStdout bool
	// slowGit makes every Git spawn sleep past the declared host kill.
	slowGit bool
	// staleIndex leaves a corrupt snapshot and a recorded build cost no hook deadline can meet.
	staleIndex bool
	// check inspects one completed run beyond the assertions every case shares.
	check func(t *testing.T, invocation failOpenInvocation, run failOpenRun)
}

// failOpenRun is what one adapter process left behind.
type failOpenRun struct {
	stdout, stderr string
	spawns         []string
	elapsed        time.Duration
	// outlived is how long the last Git child ran after the adapter exited.
	outlived time.Duration
	// ledger reports that the run wrote the self-observation ledger.
	ledger bool
}

// failOpenSpawnLimit is the most PATH spawns any healthy invocation needs: session start, prompt
// and pre-compact read the repository with 15 Git processes.
const failOpenSpawnLimit = 15

// failOpenOutliveBound is how long a slow Git child may stay observable after the adapter exits
// (AHI-048). The adapter SIGKILLs every live child group before exiting, so this covers only the
// orphan's reap by its new parent; an abandoned slow child left running sleeps for seconds.
const failOpenOutliveBound = 100 * time.Millisecond

// failOpenShells are the shells a hook adapter might reach through PATH or $SHELL. Each is shimmed
// to record the spawn and fail, so a login shell (or any shell) cannot run unseen.
var failOpenShells = []string{"sh", "bash", "zsh", "dash", "ksh", "fish", "csh", "tcsh"}

// AHI-044: every shipped native hook adapter fails open under each fault in the matrix. Each run
// is the real `corvint` binary as the host executes it, with PATH holding only a shim directory, so
// every PATH-resolved spawn is counted and every shell spawn is recorded and refused.
func TestAHI044HookAdaptersFailOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("host adapters explicitly refuse unsupported Windows process-tree cleanup")
	}
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "corvint")
	build := procgroup.Run(t.Context(), procgroup.Spec{Argv: []string{goTool, "build", "-o", binary, "./cmd/corvint"}, Dir: root, Env: testEnvironment("GOTOOLCHAIN=local"), Timeout: 30 * time.Minute, OutputLimit: 1 << 20}) // hang detector, not a budget (decision 0082)
	if build.Err != nil || build.ExitStatus != 0 {
		t.Fatalf("real corvint binary: %v exit=%d\n%s\n%s", build.Err, build.ExitStatus, build.Stdout, build.Stderr)
	}

	invocations := failOpenInvocations(t, root)
	// At least one run must write a ledger, or the ledger allowance below is never exercised.
	var ledgerWrites atomic.Int32
	t.Cleanup(func() {
		if !t.Failed() && ledgerWrites.Load() == 0 {
			t.Error("no run wrote the self-observation ledger, so the ledger allowance went unexercised")
		}
	})
	cases := []failOpenCase{
		{name: "cold start without snapshot", check: func(t *testing.T, invocation failOpenInvocation, run failOpenRun) {
			if namesDegradation(run.stdout) && !loadDerived(run.stdout) {
				t.Fatalf("a healthy cold start degraded: %s", run.stdout)
			}
		}},
		{name: "empty stdin", stdin: []byte{}, check: refusedBeforeWork("malformed-hook-json")},
		{name: "malformed stdin", stdin: []byte(`{"session_id":`), check: refusedBeforeWork("malformed-hook-json")},
		{name: "stdout closed before write", closeStdout: true, check: func(t *testing.T, _ failOpenInvocation, run failOpenRun) {
			if !strings.Contains(run.stderr, "Corvint FALLBACK degraded: hook-stdout-unwritable") {
				t.Fatalf("closed stdout named no cause on stderr: %q", run.stderr)
			}
		}},
		{name: "index unavailable", staleIndex: true, check: func(t *testing.T, invocation failOpenInvocation, run failOpenRun) {
			if invocation.usesIndex && !strings.Contains(run.stdout, "index-snapshot-stale") && !loadDerived(run.stdout) {
				t.Fatalf("an unavailable index produced no named degradation: %s", run.stdout)
			}
		}},
		{name: "host never closes stdin", holdStdin: true, check: namesDelay(false)},
		{name: "slow git on PATH", slowGit: true, check: namesDelay(true)},
	}
	for _, invocation := range invocations {
		for _, test := range cases {
			t.Run(invocation.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				run := runFailOpenCase(t, binary, realGit, invocation, test)
				if run.ledger {
					ledgerWrites.Add(1)
				}
				t.Logf("%s/%s: %d PATH spawns, %s, Git outlived the adapter by %s", invocation.name, test.name, len(run.spawns), run.elapsed.Round(time.Millisecond), run.outlived.Round(time.Millisecond))
				// AHI-048: the exit path kills every live child group before os.Exit, so a sleeping
				// slow Git is gone once its new parent reaps it. A running Git can take longer to act
				// on the SIGKILL under host load, so other cases log the time (decision 0082).
				if test.slowGit && run.outlived >= failOpenOutliveBound {
					t.Fatalf("Git outlived the adapter by %s, at or above the %s exit-kill bound", run.outlived, failOpenOutliveBound)
				}
				test.check(t, invocation, run)
			})
		}
	}
}

// failOpenInvocations reads every shipped Claude Code and Codex hook command, so a newly shipped
// event joins the matrix without editing this test.
func failOpenInvocations(t *testing.T, root string) []failOpenInvocation {
	t.Helper()
	type declaration struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	read := func(host string) declaration {
		raw, err := os.ReadFile(filepath.Join(root, "integrations", host, "plugins", "corvint", "hooks", "hooks.json"))
		if err != nil {
			t.Fatal(err)
		}
		var parsed declaration
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	var invocations []failOpenInvocation
	for name, groups := range read("claude-code").Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook.Command != "corvint" || len(hook.Args) != 3 || hook.Args[0] != "adapter" || hook.Args[1] != "claude-code" {
					t.Fatalf("Claude Code %s ships an unexpected command %q %q", name, hook.Command, hook.Args)
				}
				event := hook.Args[2]
				invocations = append(invocations, failOpenInvocation{
					name: "claude-code " + event, argv: hook.Args,
					usesIndex: event == "session-start" || event == "user-prompt",
					payload: func(root string) map[string]any {
						return map[string]any{"session_id": "fail-open-session", "hook_event_name": name, "cwd": root, "source": "startup", "prompt": "change Fail in failopen.go", "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(root, "failopen.go")}, "trigger": "auto", "reason": "other", "stop_hook_active": false}
					},
				})
			}
		}
	}
	for name, groups := range read("codex").Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook.Command != "corvint adapter codex" || len(hook.Args) != 0 {
					t.Fatalf("Codex %s ships an unexpected command %q %q", name, hook.Command, hook.Args)
				}
				invocations = append(invocations, failOpenInvocation{
					name: "codex " + name, argv: []string{"adapter", "codex"},
					usesIndex: name == "SessionStart" || name == "UserPromptSubmit",
					payload: func(root string) map[string]any {
						return map[string]any{"session_id": "fail-open-session", "hook_event_name": name, "cwd": root, "source": "startup", "prompt": "change Fail in failopen.go", "stop_hook_active": false}
					},
				})
			}
		}
	}
	slices.SortFunc(invocations, func(a, b failOpenInvocation) int { return strings.Compare(a.name, b.name) })
	if len(invocations) == 0 {
		t.Fatal("no shipped hook invocations")
	}
	return invocations
}

// refusedBeforeWork requires the named refusal and no spawn at all: input the adapter cannot parse
// must not cost the host a Git process.
func refusedBeforeWork(code string) func(*testing.T, failOpenInvocation, failOpenRun) {
	return func(t *testing.T, _ failOpenInvocation, run failOpenRun) {
		if !strings.Contains(run.stdout, "Corvint FALLBACK degraded: "+code) {
			t.Fatalf("stdout named no %s: %s", code, run.stdout)
		}
		if len(run.spawns) != 0 {
			t.Fatalf("unparseable input still spawned %q", run.spawns)
		}
	}
}

// namesDelay requires a named degradation from an invocation the fault delayed: the deadline
// itself, or the refusal (a stale index, a timed-out repository probe) that a bounded step reached
// first. With spawnedOnly, an invocation that spawned no Git was not delayed by slow Git.
func namesDelay(spawnedOnly bool) func(*testing.T, failOpenInvocation, failOpenRun) {
	return func(t *testing.T, _ failOpenInvocation, run failOpenRun) {
		if spawnedOnly && len(run.spawns) == 0 {
			return
		}
		if !namesDegradation(run.stdout) {
			t.Fatalf("a delayed invocation named no degradation (%d spawns): %s", len(run.spawns), run.stdout)
		}
	}
}

// namesDegradation reports a named cause in either adapter wording: "FALLBACK degraded: <code>" for
// adapter faults, "Corvint fallback: <code>" for a rejected Corvint event.
func namesDegradation(stdout string) bool {
	return strings.Contains(stdout, "FALLBACK degraded: ") || strings.Contains(stdout, "Corvint fallback: ")
}

// loadDerived reports a degradation that host load alone can cause: a missed deadline or a
// timed-out repository probe. A healthy run may name one under load (decision 0082).
func loadDerived(stdout string) bool {
	return strings.Contains(stdout, "deadline") || strings.Contains(stdout, "repository-probe-timeout")
}

func runFailOpenCase(t *testing.T, binary, realGit string, invocation failOpenInvocation, test failOpenCase) failOpenRun {
	t.Helper()
	base := t.TempDir()
	repository, shims, home, temporary := filepath.Join(base, "repo"), filepath.Join(base, "shims"), filepath.Join(base, "home"), filepath.Join(base, "tmp")
	for _, directory := range []string{repository, shims, home, temporary} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeFailOpenRepository(t, realGit, repository, home)
	log, pids := filepath.Join(base, "spawns.log"), filepath.Join(base, "pids.log")
	delay := ""
	if test.slowGit {
		delay = "/bin/sleep 5\n" // PATH holds only the shims
	}
	// exec keeps the shim's pid, so pids.log names every Git process the adapter started.
	writeFailOpenShim(t, filepath.Join(shims, "git"), fmt.Sprintf("printf 'git %%s\\n' \"$*\" >> %s\necho $$ >> %s\n%sexec %s \"$@\"\n", shellQuote(log), shellQuote(pids), delay, shellQuote(realGit)))
	for _, shell := range failOpenShells {
		writeFailOpenShim(t, filepath.Join(shims, shell), fmt.Sprintf("printf 'shell %s %%s\\n' \"$*\" >> %s\nexit 97\n", shell, shellQuote(log)))
	}
	if test.staleIndex {
		writeFailOpenStaleIndex(t, binary, repository, home)
	}
	before := failOpenTree(t, base, log, pids)

	input := test.stdin
	if input == nil {
		var err error
		if input, err = json.Marshal(invocation.payload(repository)); err != nil {
			t.Fatal(err)
		}
	}
	// A hang detector, not a budget (decision 0082): the declared kill is at most 2 s.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, binary, invocation.argv...)
	command.Dir = repository
	command.Env = []string{"PATH=" + shims, "HOME=" + home, "TMPDIR=" + temporary, "SHELL=" + filepath.Join(shims, "zsh"), "CLAUDE_PROJECT_DIR=" + repository, "GIT_CONFIG_NOSYSTEM=1"}
	var stdout, stderr bytes.Buffer
	command.Stderr = &stderr
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stdin = stdinReader
	var stdoutWriter *os.File
	if test.closeStdout {
		var stdoutReader *os.File
		if stdoutReader, stdoutWriter, err = os.Pipe(); err != nil {
			t.Fatal(err)
		}
		_ = stdoutReader.Close()
		command.Stdout = stdoutWriter
	} else {
		command.Stdout = &stdout
	}
	start := time.Now()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = stdinReader.Close()
	if stdoutWriter != nil {
		_ = stdoutWriter.Close()
	}
	_, _ = stdinWriter.Write(input)
	if !test.holdStdin {
		_ = stdinWriter.Close()
	}
	err = command.Wait()
	elapsed := time.Since(start)
	_ = stdinWriter.Close()
	if ctx.Err() != nil {
		t.Fatalf("adapter hung past the hang detector: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	// Exit 0 is the non-blocking status of every native host; exit 2 blocks, and a signal death
	// names no cause.
	if err != nil || command.ProcessState.ExitCode() != 0 {
		t.Fatalf("adapter did not exit 0: %v (%s)\nstdout=%s\nstderr=%s", err, command.ProcessState, stdout.String(), stderr.String())
	}
	if !test.closeStdout && stderr.Len() != 0 {
		t.Fatalf("adapter wrote to stderr: %s", stderr.String())
	}
	// Compaction hooks write plain text to stdout (decision 0340); every other hook writes JSON.
	plain := invocation.name == "claude-code pre-compact" || invocation.name == "claude-code post-compact"
	if !test.closeStdout && !plain && !json.Valid(bytes.TrimSpace(stdout.Bytes())) {
		t.Fatalf("stdout is not one hook JSON value: %q", stdout.String())
	}
	// A Git child the adapter abandoned at its deadline is killed as the adapter exits (AHI-048);
	// wait for it before reading the tree, so a write it makes after the adapter exits still counts.
	outlived := waitFailOpenOrphans(t, pids)
	spawns := failOpenSpawns(t, log)
	for _, spawn := range spawns {
		if strings.HasPrefix(spawn, "shell ") {
			t.Fatalf("adapter spawned a shell: %q", spawn)
		}
	}
	if len(spawns) > failOpenSpawnLimit {
		t.Fatalf("adapter spawned %d processes, above the %d a healthy invocation needs: %q", len(spawns), failOpenSpawnLimit, spawns)
	}
	after := failOpenTree(t, base, log, pids)
	if changed := failOpenChanged(before, after); len(changed) != 0 {
		t.Fatalf("adapter wrote outside the declared local ledgers: %q", changed)
	}
	ledger := before[failOpenLedgers[0]] != after[failOpenLedgers[0]]
	return failOpenRun{stdout: stdout.String(), stderr: stderr.String(), spawns: spawns, elapsed: elapsed, outlived: outlived, ledger: ledger}
}

func writeFailOpenRepository(t *testing.T, realGit, repository, home string) {
	t.Helper()
	files := map[string]string{
		"AGENTS.md":   "# Agents\n\nRun `go test ./...` before changing failopen.go.\n",
		"failopen.go": "package failopen\n\n// Fail reports the fail-open marker.\nfunc Fail() string { return \"open\" }\n",
		// The ledger writers record only when Git ignores their entries, so the ledgers are live.
		".corvint/.gitignore": "self-observations.jsonl\n.self-observations.*\nunplanned-reads.jsonl\n",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repository, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repository, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := exec.Command(realGit, arguments...)
		command.Dir = repository
		// The explicit Env drops TestMain's GIT_CONFIG_PARAMETERS, so the fixture restores it:
		// detached auto maintenance after the commit races failOpenTree's walk of .git (V1-0351).
		// It stays out of the repository config, which would change the adapter's Git path.
		command.Env = []string{"HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_PARAMETERS='maintenance.auto'='false' 'gc.auto'='0'"}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

// writeFailOpenStaleIndex publishes a snapshot, corrupts it, and records a build cost no hook
// deadline can absorb, so the adapter can neither load nor rebuild the index in time.
func writeFailOpenStaleIndex(t *testing.T, binary, repository, home string) {
	t.Helper()
	command := exec.Command(binary, "--root", repository, "index")
	command.Env = []string{"PATH=" + filepath.Dir(binary) + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + home, "GIT_CONFIG_NOSYSTEM=1"}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("index: %v\n%s", err, output)
	}
	store := filepath.Join(repository, ".git", "corvint", "index")
	snapshots, err := filepath.Glob(filepath.Join(store, "*.gob"))
	if err != nil || len(snapshots) == 0 {
		t.Fatalf("no snapshot published under %s: %v", store, err)
	}
	for _, snapshot := range snapshots {
		if err := os.WriteFile(snapshot, []byte("not a snapshot"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(store, "build-cost.json"), []byte(`{"format":"corvint-index-build-cost/0","buildMilliseconds":600000}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeFailOpenShim(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
}

// waitFailOpenOrphans waits until every Git process the shims recorded has exited and returns how
// long that took. The bound is a hang detector, not a budget (decision 0082).
func waitFailOpenOrphans(t *testing.T, pids string) time.Duration {
	t.Helper()
	start := time.Now()
	for _, line := range failOpenSpawns(t, pids) {
		pid, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("pid log line %q: %v", line, err)
		}
		for failOpenAlive(pid) {
			if time.Since(start) > time.Minute {
				t.Fatalf("Git child %d outlived the adapter past the hang detector", pid)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	return time.Since(start)
}

func failOpenAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer process.Release()
	return process.Signal(syscall.Signal(0)) == nil
}

func failOpenSpawns(t *testing.T, log string) []string {
	t.Helper()
	raw, err := os.ReadFile(log)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// failOpenLedgers are the only paths an adapter may create or change: the bounded local ledgers
// of AGENTS.md invariant 4. The self-observation writer replaces its ledger through a
// `.self-observations.*` temporary in the same directory.
var failOpenLedgers = []string{"repo/.corvint/self-observations.jsonl", "repo/.corvint/unplanned-reads.jsonl"}

// failOpenTree fingerprints every entry under base except the logs the shims write. A
// directory or symlink is recorded by path and type, so one left behind (a scratch directory under
// TMPDIR) is still a change.
func failOpenTree(t *testing.T, base string, logs ...string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == base || slices.Contains(logs, path) {
			return nil
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !entry.Type().IsRegular() {
			target, _ := os.Readlink(path)
			tree[relative] = fmt.Sprintf("%s %s", entry.Type(), target)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[relative] = fmt.Sprintf("%s %d %x", info.Mode(), info.ModTime().UnixNano(), content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func failOpenChanged(before, after map[string]string) []string {
	var changed []string
	for path, fingerprint := range after {
		if before[path] != fingerprint && !slices.Contains(failOpenLedgers, path) && !strings.HasPrefix(path, "repo/.corvint/.self-observations.") {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	slices.Sort(changed)
	return changed
}
