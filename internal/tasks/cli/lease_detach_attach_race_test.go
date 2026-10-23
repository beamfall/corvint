//go:build darwin || linux

package cli_test

import (
	"bytes"
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
