//go:build darwin || linux

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-311 (V1-1559): an attach that reads the record while its launcher
// finishes starting the child never reports SUPERVISOR_LOST. The launcher
// records child.json before it releases the launch lock, so the attach must
// read the lock first and the child second. The gap between the two reads is
// held open: the launch completes inside it, which an attach that read the
// child first answered LOST with a child already running.
func TestATRV0311_AttachNeverReportsALaunchCompletingBetweenItsReadsLost(t *testing.T) {
	root, repo, _ := detachClaimStore(t)
	const requestID = "claim-launching"
	dir := cli.LeaseDetachDir(repo, requestID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "request.json"), map[string]string{"profile": cli.LeaseDetachProfile, "command": "claim", "requestId": requestID, "holder": "", "argvSha256": "0"})
	// The launcher: it holds the launch lock and has not recorded a child.
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	id, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil || id == "" {
		t.Fatalf("identity of the test process: %q %v", id, err)
	}
	completed := 0
	t.Cleanup(cli.SetLeaseDetachProbeGap(func() {
		completed++
		// The launch completes: the child is recorded, then the lock goes.
		now := time.Now().UTC().Format(time.RFC3339)
		writeJSON(t, filepath.Join(dir, "child.json"), map[string]any{"pid": os.Getpid(), "identity": id, "startedAt": now, "ceilingAt": now})
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	}))
	x := cliRun(t, root, "claim", "--attach", "--request-id", requestID)
	if completed != 1 {
		t.Fatalf("the launch was completed %d times inside the attach, want once", completed)
	}
	if hasCode(x.res, wire.CodeSupervisorLost) || x.code != 75 || field(x.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("attach while the launch completed: code %d %s", x.code, x.stdout)
	}
}

// lostRequest records a detached request whose recorded child is gone and no
// launcher runs, the state in which an attach may answer LOST, and returns
// its record directory and launch-lock path.
func lostRequest(t *testing.T, requestID string) (string, string, string) {
	t.Helper()
	root, repo, _ := detachClaimStore(t)
	dir := cli.LeaseDetachDir(repo, requestID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "request.json"), map[string]string{"profile": cli.LeaseDetachProfile, "command": "claim", "requestId": requestID, "holder": "", "argvSha256": "0"})
	lockPath := filepath.Join(dir, "lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	writeJSON(t, filepath.Join(dir, "child.json"), map[string]any{"pid": os.Getpid(), "identity": "the-ended-child", "startedAt": now, "ceilingAt": now})
	return root, dir, lockPath
}

// relauncher is a retry of the request that tries the launch lock as a
// launcher does, without waiting, at the attach's chosen liveness reads, and
// once it holds the lock pauses there before recording its child.
type relauncher struct {
	lock     *os.File
	reads    int
	from     int // the first liveness read at which it tries the lock
	acquired bool
	refused  int
}

func newRelauncher(t *testing.T, lockPath string, from int) *relauncher {
	t.Helper()
	f, err := os.OpenFile(lockPath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return &relauncher{lock: f, from: from}
}

// probe is the attach's identity probe: every liveness read of the recorded
// child calls it once. The child is gone (another identity holds its pid).
func (r *relauncher) probe(int) (string, error) {
	r.reads++
	if r.reads >= r.from && !r.acquired {
		switch err := syscall.Flock(int(r.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); {
		case err == nil:
			r.acquired = true
		case errors.Is(err, syscall.EWOULDBLOCK):
			r.refused++
		}
	}
	return "a-replacement-process", nil
}

// ATR-V0-311 (V1-1559, review finding 3): a retry that takes the launch lock
// late, after the attach has last seen it free and before the attach's final
// liveness read, and pauses before recording its child, must not be reported
// LOST: the attach holds the lock shared from before the liveness read until
// it answers, so the retry either cannot take it until the answer is chosen
// or already holds it and the answer is PENDING. An attach that read the
// lock twice and liveness twice answered SUPERVISOR_LOST here while the
// retry held the lock.
func TestATRV0311_LateRelaunchIsNeverReportedLost(t *testing.T) {
	cases := []struct {
		name string
		from int // the liveness read at which the retry first tries the lock
	}{
		{"retry at the second liveness read", 2},
		{"retry at every liveness read", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, _, lockPath := lostRequest(t, "claim-late-relaunch")
			r := newRelauncher(t, lockPath, c.from)
			t.Cleanup(cli.SetLeaseDetachIdentity(r.probe))
			x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-late-relaunch")
			lost := hasCode(x.res, wire.CodeSupervisorLost)
			if r.acquired && lost {
				t.Fatalf("the attach reported LOST while a retry held the launch lock (liveness reads %d): %s", r.reads, x.stdout)
			}
			if r.reads >= c.from && !r.acquired && r.refused == 0 {
				t.Fatalf("the retry neither took nor was refused the lock")
			}
			if !lost && (x.code != 75 || field(x.res.Items[0], "state").Str != "PENDING") {
				t.Fatalf("attach: code %d %s, want LOST or PENDING", x.code, x.stdout)
			}
			if lost && r.reads >= c.from && r.refused == 0 {
				t.Fatalf("LOST with no relaunch refused during the decision: %s", x.stdout)
			}
		})
	}
}

// ATR-V0-311 (V1-1559): an attach to a lost request whose retry already
// holds the launch lock answers PENDING, not LOST, and leaves the lock to it.
func TestATRV0311_AttachDuringARelaunchIsPending(t *testing.T) {
	root, _, lockPath := lostRequest(t, "claim-relaunching")
	r := newRelauncher(t, lockPath, 1)
	if err := syscall.Flock(int(r.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cli.SetLeaseDetachIdentity(func(int) (string, error) { return "a-replacement-process", nil }))
	x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-relaunching")
	if hasCode(x.res, wire.CodeSupervisorLost) || x.code != 75 || field(x.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("attach during a relaunch: code %d %s", x.code, x.stdout)
	}
	// Once the relaunch has gone without recording a child, the request is lost.
	if err := syscall.Flock(int(r.lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-relaunching"); !hasCode(x.res, wire.CodeSupervisorLost) {
		t.Fatalf("attach after the relaunch ended: code %d %s, want SUPERVISOR_LOST", x.code, x.stdout)
	}
}

// ATR-V0-311 (V1-1559), end to end on a real launch: a child delayed at
// startup (the injected CORVINT_TEST_LEASE_DETACH_START_MS) is attached to
// continuously from the moment the request is recorded, through the launch
// and the delay, and no attach answers SUPERVISOR_LOST; the claim commits.
func TestATRV0311_AttachDuringChildStartupIsNeverLost(t *testing.T) {
	root, _, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_START_MS", "1500")
	const requestID = "claim-startup"
	claim := []string{"claim", id, "--holder", "agent", "--request-id", requestID}
	launched := make(chan run, 1)
	go func() {
		var out, errb bytes.Buffer
		code := cli.Run(cli.Env{Cwd: root, Args: append(append([]string{}, claim...), "--detach", "--wait", "0"), Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		res, _ := wire.DecodeResult(out.Bytes())
		launched <- run{code: code, stdout: out.Bytes(), res: res}
	}()
	attaches, pending := 0, 0
	for done := false; !done; attaches++ {
		x := cliRun(t, root, "claim", "--attach", "--request-id", requestID)
		switch {
		case hasCode(x.res, wire.CodeSupervisorLost):
			t.Fatalf("attach %d reported the starting child lost: %s", attaches, x.stdout)
		case x.code == 75:
			pending++
		case x.code == 0:
			done = true
			if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "attemptId").Str == "" {
				t.Fatalf("the delayed child did not commit the claim: %s", x.stdout)
			}
		}
		time.Sleep(2 * time.Millisecond)
		if attaches > 20000 {
			t.Fatalf("no result after %d attaches", attaches)
		}
	}
	if l := <-launched; l.res == nil || hasCode(l.res, wire.CodeSupervisorLost) {
		t.Fatalf("the launching caller reported the child lost: %s", l.stdout)
	}
	if pending == 0 {
		t.Fatal("no attach observed the startup delay")
	}
}
