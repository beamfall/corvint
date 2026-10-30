//go:build darwin

package analyzerexec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSandboxCommandSanitizesAmbientStateAndEscapesPaths(t *testing.T) {
	path := `/private/tmp/a"quoted\\path` + "\n" + `leaf`
	literal := sandboxLiteral(path)
	if !strings.Contains(literal, `\"`) || !strings.Contains(literal, `\\`) || strings.Contains(literal, "\n") {
		t.Fatalf("sandbox path escaping is unsafe: %q", literal)
	}
	command := containedCommand(context.Background(), "(version 1)", "/private/tmp/corvint-analyzer")
	if command.Dir != "/" || command.Env == nil || len(command.Env) != 0 {
		t.Fatalf("sandbox command inherits ambient state: dir=%q env=%#v", command.Dir, command.Env)
	}
}

func TestSandboxProfileDeniesDataMountAndAmbientMachIPC(t *testing.T) {
	profile := sandboxProfile("/dev/fd/3")
	if strings.Contains(profile, `(allow mach-lookup`) || strings.Contains(profile, `(allow mach-register`) || strings.Contains(profile, "system.logger") || strings.Contains(profile, "/System/Library") || strings.Contains(profile, `(subpath "/")`) {
		t.Fatalf("sandbox profile has a broad system or Mach allowance: %s", profile)
	}
	for _, rule := range []string{
		`(deny file-read* (subpath "/System"))`,
		`(deny file-read* (subpath "/System/Volumes/Data"))`,
		`(deny mach-lookup)`,
		`(deny mach-register)`,
		`(allow process-exec* (literal "/dev/fd/3"))`,
		`(allow file-read-data (literal "/"))`,
	} {
		if !strings.Contains(profile, rule) {
			t.Fatalf("sandbox profile lacks required rule %q: %s", rule, profile)
		}
	}
}

// The plan's 100 ms contract deadline (the validatePlanShape cap) also bounds
// validation and staging, which a loaded host can exhaust before launch; the
// deadline then correctly wins and the precondition was never reached. Only an
// attempt whose cancellation provably preceded that deadline is judged; the
// retry wall is a hang detector, not a budget (decision 0082).
func TestCancellationImmediatelyBeforeStartRetainsNoStartSemantics(t *testing.T) {
	previous := executeContained
	defer func() { executeContained = previous }()
	end := time.Now().Add(60 * time.Second)
	for attempt := 1; ; attempt++ {
		plan := testPlan(t, "cancel-before-start")
		ctx, cancel := context.WithCancel(context.Background())
		cancelledBeforeDeadline := false
		started := time.Now()
		executeContained = func(command *exec.Cmd) error {
			cancel()
			cancelledBeforeDeadline = time.Since(started) < plan.Timeout
			return command.Run()
		}
		result, err := Run(ctx, plan)
		cancel()
		if cancelledBeforeDeadline {
			if !Is(err, Cancelled) || result.Started || result.Completed || result.Termination != TerminationNotRun || result.CleanupState != CleanupNotRun || !result.ValidCleanupObservation() {
				t.Fatalf("pre-start cancellation result=%#v err=%v", result, err)
			}
			return
		}
		if time.Now().After(end) {
			t.Fatalf("no attempt reached launch within the contract deadline in %d attempts over 60s: last result=%#v err=%v", attempt, result, err)
		}
	}
}

func TestMachOMinimumOSIsBoundToObservedHost(t *testing.T) {
	file, err := os.Open(nativeHelper(t))
	if err != nil {
		t.Fatal(err)
	}
	candidate, candidateErr := NativeExecutablePlatform(file)
	file.Close()
	host, hostErr := ActualHostPlatform()
	if candidateErr != nil || hostErr != nil || !nativeExecutableMatchesHost(candidate, host) {
		t.Fatalf("candidate=%#v host=%#v candidateErr=%v hostErr=%v", candidate, host, candidateErr, hostErr)
	}
	future := candidate
	future.ABI = "v1z141z3"
	if nativeExecutableMatchesHost(future, host) {
		t.Fatalf("future minimum OS accepted: candidate=%#v host=%#v", future, host)
	}
}

func TestContainedTimeoutReapsProcess(t *testing.T) {
	staging := requireContainedBackend(t)
	plan := testPlan(t, "hang")
	plan.StagingParent = staging
	result, err := Run(context.Background(), plan)
	if !Is(err, Timeout) || !result.Started || !result.Completed || result.Termination != TerminationTimedOut || result.CleanupState != CleanupObserved || !result.ValidCleanupObservation() {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestContainedStreamLimitsAreIndependent(t *testing.T) {
	staging := requireContainedBackend(t)
	for _, test := range []struct {
		name, request  string
		stdout, stderr int
		want           Failure
	}{
		{"stdout at cap", "stdout:3", 3, 4, ""},
		{"stdout above cap", "stdout:4", 3, 4, Limit},
		{"stderr at cap", "stderr:4", 3, 4, ""},
		{"stderr above cap", "stderr:5", 3, 4, Limit},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := testPlan(t, test.request)
			plan.StagingParent = staging
			plan.MaxStdoutBytes, plan.MaxStderrBytes = test.stdout, test.stderr
			result, err := Run(context.Background(), plan)
			if test.want == "" {
				if err != nil || !result.ValidCleanupObservation() {
					t.Fatalf("result=%#v err=%v", result, err)
				}
				return
			}
			if !Is(err, test.want) || !result.Started || !result.Completed || result.CleanupState != CleanupObserved || !result.ValidCleanupObservation() {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

// requireContainedBackend runs the exact production profile outside the plan
// cap first, so only an observed sandbox_apply refusal skips. It then returns
// an owner-private staging parent whose content-addressed helper entry has
// completed one contained run: a fresh staged inode's first exec is slower
// than the 100 ms plan cap on Darwin (decision 0148), so a Timeout says only
// that one attempt did not finish in time. The retry wall is a hang detector
// (decision 0082); exhausting it is a failure, not a skip.
func requireContainedBackend(t *testing.T) string {
	t.Helper()
	helper := nativeHelper(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := containedCommand(ctx, sandboxProfile(helper), helper)
	command.Stdin = strings.NewReader("contained-preflight")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	if bytes.Contains(stderr.Bytes(), []byte("sandbox_apply: Operation not permitted")) {
		t.Skipf("parent process forbids nested Darwin sandbox installation: %s", stderr.Bytes())
	}
	if err != nil {
		t.Fatalf("production sandbox profile did not run the helper outside the plan cap: %v stderr=%q", err, stderr.Bytes())
	}
	plan := testPlan(t, "contained-probe")
	end := time.Now().Add(60 * time.Second)
	for attempt := 1; ; attempt++ {
		result, err := Run(context.Background(), plan)
		if !Is(err, Timeout) {
			if err != nil || !result.ValidCleanupObservation() {
				t.Fatalf("contained backend probe result=%#v err=%v", result, err)
			}
			return plan.StagingParent
		}
		if time.Now().After(end) {
			t.Fatalf("no contained probe completed within the contract deadline in %d attempts over 60s: last=%#v", attempt, result)
		}
	}
}

// ACC-V0-002, ACC-V0-019: a first-launch assessment timeout is terminal;
// staging must not hide a payload warmup or turn that launch into a retry.
func TestFirstInvocationTimeoutIsNotRetried(t *testing.T) {
	plan := testPlan(t, "first-launch")
	previous := executeContained
	defer func() { executeContained = previous }()
	calls := 0
	executeContained = func(command *exec.Cmd) error {
		calls++
		// No payload runs: model an OS launch assessment outlasting the cap.
		time.Sleep(2 * plan.Timeout)
		return context.DeadlineExceeded
	}
	end := time.Now().Add(60 * time.Second)
	for {
		result, err := Run(context.Background(), plan)
		if !Is(err, Timeout) || result.Started || result.Completed || result.Termination != TerminationNotRun || !result.ValidCleanupObservation() || len(result.Stdout) != 0 || len(result.Stderr) != 0 {
			t.Fatalf("first-launch timeout result=%#v err=%v", result, err)
		}
		if calls != 0 {
			if calls != 1 {
				t.Fatalf("first-launch timeout executed %d times, want one", calls)
			}
			return
		}
		if time.Now().After(end) {
			t.Fatal("no attempt reached launch before the contract deadline over 60s")
		}
	}
}
