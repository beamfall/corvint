//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// V1-1498: the finish-signal helper is bound to its parent's lifetime, so one
// whose named parent is not its parent exits 3 before it runs finish.
func TestLocalCompletionFinishSignalHelperExitsWhenParentGone(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestLocalCompletionFinishSignalHelper$")
	command.Env = append(os.Environ(), "CORVINT_FINISH_SIGNAL_ROOT="+t.TempDir(), "CORVINT_FINISH_SIGNAL_MARKER="+filepath.Join(t.TempDir(), "held"), testsupport.ParentPIDEnv+"="+strconv.Itoa(os.Getppid()))
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 3 || !strings.Contains(stderr.String(), "parent gone") {
		t.Fatalf("helper with a gone parent: %v, stderr %q; want exit 3 naming the parent", err, stderr.String())
	}
}

// TestLocalCompletionFinishSignalHelper is the re-executed `corvint dogfood
// finish` for LCP-V0-060: the real CLI argv through runProfiledContext under the
// same signal.NotifyContext main installs, with only the final strict check's
// second CEM status held open until that context ends.
func TestLocalCompletionFinishSignalHelper(t *testing.T) {
	root := os.Getenv("CORVINT_FINISH_SIGNAL_ROOT")
	if root == "" {
		return
	}
	// The helper holds until its parent signals it; it exits if the parent dies first (V1-1498).
	testsupport.ExitWhenParentGone()
	marker := os.Getenv("CORVINT_FINISH_SIGNAL_MARKER")
	wrapFinishCommand = func(next localcompletion.PublicCommand) localcompletion.PublicCommand {
		statuses := 0
		return func(ctx context.Context, root string, args []string, out, errout io.Writer) int {
			if len(args) > 1 && args[0] == "cem" && args[1] == "status" {
				statuses++
				if statuses > 1 {
					_ = os.WriteFile(marker, []byte("held"), 0600)
					<-ctx.Done()
					return 2
				}
			}
			return next(ctx, root, args, out, errout)
		}
	}
	signalled, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	args := []string{"--root", root, "dogfood", "finish", "--session-key", os.Getenv("CORVINT_FINISH_SIGNAL_KEY")}
	os.Exit(runProfiledContext(signalled, signalled, args, strings.NewReader(""), os.Stdout, os.Stderr, os.LookupEnv))
}

// assertFinishSignalCLI pins LCP-V0-060 (V1-3192): a real SIGINT and SIGTERM
// delivered to the `corvint dogfood finish` process while its final strict
// check runs exit 2 with one parseable finish-interrupted refusal on stderr
// naming the step, both retained logs and the rerun command, put the
// unsatisfied active policy envelope on stdout (never empty stdout), and write
// no enrollment state.
func assertFinishSignalCLI(t *testing.T, root, key, statePath string) {
	t.Helper()
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		marker := filepath.Join(t.TempDir(), "held")
		command := exec.Command(os.Args[0], "-test.run=^TestLocalCompletionFinishSignalHelper$")
		command.Env = append(os.Environ(), "CORVINT_FINISH_SIGNAL_ROOT="+root, "CORVINT_FINISH_SIGNAL_KEY="+key, "CORVINT_FINISH_SIGNAL_MARKER="+marker, testsupport.ParentPIDEntry())
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		if err = command.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		deadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(deadline) {
			if _, err = os.Stat(marker); err == nil {
				break
			}
			select {
			case err = <-done:
				t.Fatalf("%s: finish exited before the final check was held: %v stdout=%s stderr=%s", sig, err, stdout.String(), stderr.String())
			case <-time.After(20 * time.Millisecond):
			}
		}
		if _, err = os.Stat(marker); err != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
			t.Fatalf("%s: final strict check never reached", sig)
		}
		if err = command.Process.Signal(sig); err != nil {
			t.Fatal(err)
		}
		var exit *exec.ExitError
		select {
		case err = <-done:
		case <-time.After(time.Minute):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
			t.Fatalf("%s: finish did not stop", sig)
		}
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatalf("%s: exit %v stderr=%s", sig, err, stderr.String())
		}
		var policy struct {
			OK     bool   `json:"ok"`
			Tool   string `json:"tool"`
			Policy struct {
				Lifecycle   string     `json:"lifecycle"`
				Satisfied   bool       `json:"satisfied"`
				NextActions [][]string `json:"nextActions"`
			} `json:"policy"`
		}
		if err = json.Unmarshal(stdout.Bytes(), &policy); err != nil || policy.OK || policy.Tool != "dogfood-finish" ||
			policy.Policy.Lifecycle != "active" || policy.Policy.Satisfied || len(policy.Policy.NextActions) == 0 {
			t.Fatalf("%s: stdout lacks the active finish policy: %v %q", sig, err, stdout.String())
		}
		var refusal struct {
			OK    bool   `json:"ok"`
			Code  string `json:"code"`
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err = json.Unmarshal(stderr.Bytes(), &refusal); err != nil {
			t.Fatalf("%s: stderr is not one JSON refusal: %v %q", sig, err, stderr.String())
		}
		message := refusal.Error.Message
		if refusal.OK || refusal.Code != "finish-interrupted" || refusal.Error.Code != "finish-interrupted" ||
			!strings.Contains(message, "step=final-check") ||
			!strings.HasSuffix(message, " rerun=corvint dogfood finish --session-key "+key) {
			t.Fatalf("%s: interrupted refusal misreported: %s", sig, stderr.String())
		}
		_, logs, _ := strings.Cut(message, " logs=")
		logs, _, _ = strings.Cut(logs, " ")
		names := strings.Split(logs, ",")
		if len(names) != 2 || !strings.HasSuffix(names[0], "/final-check.stdout") || !strings.HasSuffix(names[1], "/final-check.stderr") {
			t.Fatalf("%s: interrupted refusal hides its logs: %s", sig, message)
		}
		for _, name := range names {
			if !strings.HasPrefix(name, "$GIT_DIR/corvint/local-completion/"+key+"/") {
				t.Fatalf("%s: log outside the enrollment: %s", sig, name)
			}
			if _, err = os.Stat(filepath.Join(root, ".git", strings.TrimPrefix(name, "$GIT_DIR/"))); err != nil {
				t.Fatalf("%s: log %s not retained: %v", sig, name, err)
			}
		}
		if after, _ := os.ReadFile(statePath); !bytes.Equal(after, before) {
			t.Fatalf("%s: interrupted finish wrote state", sig)
		}
	}
}
