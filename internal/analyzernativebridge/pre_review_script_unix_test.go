//go:build !windows

package analyzernativebridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type preReviewReceipt struct {
	Result   string `json:"result"`
	Commands []struct {
		Name    string `json:"name"`
		Result  string `json:"result"`
		Status  int    `json:"exit_status"`
		Failure struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"failure_artifact"`
	} `json:"commands"`
}

type preReviewFixture struct {
	root, script, gitDir, tmp string
	head                      string
}

func TestNativeBridgePreReviewFaultDiagnosticsSurviveCleanup(t *testing.T) {
	fixture := newPreReviewFixture(t)
	result := startPreReviewCommand(t, context.Background(), fixture.command(map[string]string{
		"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE":        "package-and-clis",
		"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_OUTPUT_BYTES": "20000",
		"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_OUTPUT_TEXT":  "Authorization: Bearer visible-token\ntoken=api-secret password=other-secret https://user:pass@example.invalid",
	})).wait()
	if result == nil {
		t.Fatal("fault-injected pre-review passed")
	}
	receipt := fixture.receipt(t)
	if receipt.Result != "FAIL" || len(receipt.Commands) != 2 {
		t.Fatalf("receipt=%+v", receipt)
	}
	stage := receipt.Commands[1]
	if stage.Name != "package-and-clis" || stage.Result != "FAIL" || stage.Status != 97 || stage.Failure.Path == "" || !strings.HasPrefix(stage.Failure.SHA256, "sha256:") {
		t.Fatalf("stage=%+v", stage)
	}
	artifact := filepath.Join(fixture.gitDir, "corvint", "native-bridge-pre-review", stage.Failure.Path)
	info, err := os.Stat(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || info.Size() > 16*1024 {
		t.Fatalf("artifact mode=%o bytes=%d", info.Mode().Perm(), info.Size())
	}
	content, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"stage=package-and-clis\n",
		"exit_status=97\n",
		"output_sha256=sha256:",
		"output_bytes=",
		"captured_bytes=12288\n",
		"truncated=true\n",
		"[output truncated after 12288 bytes]",
	} {
		if !strings.Contains(string(content), required) {
			t.Fatalf("artifact missing %q", required)
		}
	}
	digest := sha256.Sum256(content)
	if got := "sha256:" + hex.EncodeToString(digest[:]); got != stage.Failure.SHA256 {
		t.Fatalf("artifact sha256=%s receipt=%s", got, stage.Failure.SHA256)
	}
	for _, forbidden := range []string{"visible-token", "api-secret", "other-secret", "user:pass"} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("artifact retained secret %q", forbidden)
		}
	}
	fixture.assertNoResidue(t)
}

// NJB-007: both signal traps must reap the stage child and remove owned files.
func TestNativeBridgePreReviewLockAndSignalCleanup(t *testing.T) {
	t.Run("NJB-007 lock and signal cleanup", func(t *testing.T) {
		for name, signal := range map[string]os.Signal{"int": syscall.SIGINT, "term": syscall.SIGTERM} {
			t.Run(name, func(t *testing.T) {
				fixture := newPreReviewFixture(t)
				first := startPreReviewCommand(t, context.Background(), fixture.command(map[string]string{
					"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE":         "package-and-clis",
					"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_SLEEP_SECONDS": "30",
				}))
				childPID := fixture.waitForFaultChild(t)
				owner, err := os.ReadFile(filepath.Join(fixture.gitDir, "corvint", "native-bridge-pre-review.lock", "owner"))
				if err != nil || !strings.Contains(string(owner), "type=corvint-native-bridge-pre-review/v1\n") {
					t.Fatalf("lock owner=%q err=%v", owner, err)
				}
				second := startPreReviewCommand(t, context.Background(), fixture.command(map[string]string{
					"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE": "package-and-clis",
				}))
				err = second.wait()
				output := second.output.String()
				if err == nil || !strings.Contains(string(output), "already running for this worktree") {
					t.Fatalf("second err=%v output=%s", err, output)
				}
				if err := first.command.Process.Signal(signal); err != nil {
					t.Fatal(err)
				}
				if err := first.wait(); err == nil {
					t.Fatal("interrupted pre-review passed")
				}
				assertPreReviewReaped(t, first, childPID)
				fixture.assertNoResidue(t)
			})
		}
	})
}

// NJB-007: a passing early-return subtest exercises testing cleanup unwinding,
// including the ownership path used by Fatal after Start but before explicit Wait.
func TestNativeBridgePreReviewEarlyReturnCleanup(t *testing.T) {
	fixture := newPreReviewFixture(t)
	var childPID int
	var running *preReviewCommand
	t.Run("NJB-007 early return cleanup", func(t *testing.T) {
		running = startPreReviewCommand(t, context.Background(), fixture.command(map[string]string{
			"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE":         "package-and-clis",
			"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_SLEEP_SECONDS": "30",
		}))
		childPID = fixture.waitForFaultChild(t)
	})
	assertPreReviewReaped(t, running, childPID)
	fixture.assertNoResidue(t)
}

// NJB-007: normal completion and deadline cancellation both reap the known child.
// Readiness comes from the stage's PID marker, not an assumed startup sleep.
func TestNativeBridgePreReviewCompletionAndDeadlineCleanup(t *testing.T) {
	t.Run("NJB-007 completion and deadline cleanup", func(t *testing.T) {
		for _, mode := range []string{"normal", "deadline"} {
			t.Run(mode, func(t *testing.T) {
				fixture := newPreReviewFixture(t)
				seconds := "1"
				if mode == "deadline" {
					seconds = "30"
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				running := startPreReviewCommand(t, ctx, fixture.command(map[string]string{
					"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE":         "package-and-clis",
					"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_SLEEP_SECONDS": seconds,
				}))
				childPID := fixture.waitForFaultChild(t)
				err := running.wait()
				if mode == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("deadline result=%v", err)
				}
				if mode == "normal" && (err == nil || ctx.Err() != nil) {
					t.Fatalf("normal fault result=%v context=%v", err, ctx.Err())
				}
				assertPreReviewReaped(t, running, childPID)
				fixture.assertNoResidue(t)
			})
		}
	})
}

// NJB-007: a helper that refuses TERM still loses its whole owned group.
// Unlike the script trap cases, this only promises process cleanup.
func TestNativeBridgePreReviewHardKillFallback(t *testing.T) {
	t.Run("NJB-007 hard kill fallback", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "child-pid")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		command := exec.Command("sh", "-c", `trap '' TERM INT
sleep 30 &
child=$!
printf '%s\n' "$child" > "$1"
wait "$child"`, "pre-review-kill-fixture", marker)
		running := startPreReviewCommand(t, ctx, command)
		var childPID int
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if content, err := os.ReadFile(marker); err == nil {
				childPID, _ = strconv.Atoi(strings.TrimSpace(string(content)))
				if childPID > 0 {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		if childPID == 0 {
			t.Fatal("TERM-resistant child never became ready")
		}
		cancel()
		if err := running.wait(); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation result=%v", err)
		}
		assertPreReviewReaped(t, running, childPID)
	})
}

func assertPreReviewReaped(t *testing.T, running *preReviewCommand, childPID int) {
	t.Helper()
	if running.command.ProcessState == nil {
		t.Fatal("direct child was not waited")
	}
	for _, pid := range []int{running.command.Process.Pid, childPID} {
		if !waitDescendantReaped(pid, 2*time.Second) {
			t.Fatalf("owned pid=%d survived cleanup", pid)
		}
	}
}

// Every script has one Wait owner and an absolute deadline. Register cleanup
// immediately after Start so assertion exits cancel and reap before TempDir removal.
// TERM gives the script's trap a bounded chance to remove its lock and run root;
// group KILL is the fallback, whose filesystem cleanup cannot be promised.
type preReviewCommand struct {
	command *exec.Cmd
	output  bytes.Buffer
	done    chan struct{}
	err     error
}

func startPreReviewCommand(t *testing.T, parent context.Context, command *exec.Cmd) *preReviewCommand {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	running := &preReviewCommand{command: command, done: make(chan struct{})}
	configureProcessGroup(command)
	command.Stdout, command.Stderr = &running.output, &running.output
	command.WaitDelay = time.Second
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); <-running.done })
	go func() {
		defer close(running.done)
		defer cancel()
		exited := make(chan error, 1)
		go func() { exited <- command.Wait() }()
		select {
		case running.err = <-exited:
		case <-ctx.Done():
			_ = command.Process.Signal(syscall.SIGTERM)
			grace := time.NewTimer(10 * time.Second)
			select {
			case <-exited:
			case <-grace.C:
				killProcessTree(command)
				<-exited
			}
			grace.Stop()
			running.err = ctx.Err()
		}
		killProcessTree(command)
	}()
	return running
}

func (running *preReviewCommand) wait() error {
	<-running.done
	return running.err
}

func preReviewGitOutput(t *testing.T, command *exec.Cmd) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := boundedCombinedOutput(ctx, command)
	if err != nil {
		t.Fatalf("git: %v\n%s", err, output)
	}
	return output
}

func newPreReviewFixture(t *testing.T) preReviewFixture {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	script, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "tools", "native-bridge-pre-review.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "fixture")
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, "tools", "native-bridge-pre-review.sh")
	if err := os.WriteFile(scriptPath, script, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "--quiet", root},
		{"-C", root, "config", "user.email", "pre-review@example.invalid"},
		{"-C", root, "config", "user.name", "Pre Review Test"},
		{"-C", root, "add", "tools/native-bridge-pre-review.sh"},
		{"-C", root, "commit", "--quiet", "-m", "pre-review fixture"},
	} {
		preReviewGitOutput(t, exec.Command("git", args...))
	}
	gitDir := gitOutput(t, root, "rev-parse", "--git-dir")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return preReviewFixture{
		root:   root,
		script: scriptPath,
		gitDir: gitDir,
		tmp:    newPreReviewTemp(t),
		head:   gitOutput(t, root, "rev-parse", "HEAD"),
	}
}

func newPreReviewTemp(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tmp")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	output := preReviewGitOutput(t, exec.Command("git", append([]string{"-C", root}, args...)...))
	return strings.TrimSpace(string(output))
}

func (fixture preReviewFixture) command(values map[string]string) *exec.Cmd {
	command := exec.Command(fixture.script)
	command.Dir = fixture.root
	command.Env = mergePreReviewEnv(values, "TMPDIR="+fixture.tmp)
	return command
}

func mergePreReviewEnv(values map[string]string, additions ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(values)+len(additions))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "TMPDIR" || strings.HasPrefix(key, "CORVINT_NATIVE_BRIDGE_PRE_REVIEW_") {
			continue
		}
		env = append(env, entry)
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return append(env, additions...)
}

func (fixture preReviewFixture) receipt(t *testing.T) preReviewReceipt {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(fixture.gitDir, "corvint", "native-bridge-pre-review", fixture.head+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt preReviewReceipt
	if err := json.Unmarshal(content, &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func (fixture preReviewFixture) waitForFaultChild(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(fixture.tmp)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "corvint-native-bridge-pre-review.") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(fixture.tmp, entry.Name(), "package-and-clis.out"))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(content), "\n") {
				if !strings.HasPrefix(line, "child_pid=") {
					continue
				}
				pid, err := strconv.Atoi(strings.TrimPrefix(line, "child_pid="))
				if err != nil {
					t.Fatal(err)
				}
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fault child pid was not recorded")
	return 0
}

func (fixture preReviewFixture) assertNoResidue(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(fixture.tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "corvint-native-bridge-pre-review.") {
			t.Fatalf("temporary residue=%s", entry.Name())
		}
	}
	lock := filepath.Join(fixture.gitDir, "corvint", "native-bridge-pre-review.lock")
	if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock residue=%s err=%v", lock, err)
	}
}
