//go:build unix

package gitrun

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// fakeBatch answers `ok*` requests with one blob record, hangs on `hang`, and
// reports every other request missing, like `git cat-file --batch`.
const fakeBatch = `while IFS= read -r line; do
case "$line" in
ok*) printf '%s blob 3\nhi!\n' "$line" ;;
hang) sleep 30 ;;
*) printf '%s missing\n' "$line" ;;
esac
done`

func TestSessionServesRecordsAndReapsOnEveryExit(t *testing.T) {
	session := &Session{}
	args := []string{"-c", fakeBatch}
	admit := func(string, []string, int) bool { return true }
	read := func(request string, perOp time.Duration) (string, bool, error) {
		header, body, ok, err := session.Read(context.Background(), perOp, shell(), args, request, admit)
		return header + "|" + string(body), ok, err
	}
	gone := func(pid int) bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }
	if got, ok, err := read("ok1", time.Second); !ok || err != nil || got != "ok1 blob 3|hi!" {
		t.Fatalf("first record %q %t %v", got, ok, err)
	}
	pid := session.command.Process.Pid
	if got, ok, err := read("ok2", time.Second); !ok || err != nil || got != "ok2 blob 3|hi!" || session.command.Process.Pid != pid {
		t.Fatalf("second record did not share the child: %q %t %v", got, ok, err)
	}
	if _, ok, err := read("absent", time.Second); ok || err != nil || session.command != nil || !gone(pid) {
		t.Fatalf("bodiless answer kept its child: %t %v", ok, err)
	}
	if _, ok, err := read("hang", 100*time.Millisecond); ok || cemcode.CodeOf(err) != cemcode.GitTimeout || session.command != nil {
		t.Fatalf("hung request: %t %v", ok, err)
	}
	if _, ok, _ := read("ok3", time.Second); !ok {
		t.Fatal("session did not restart after a timeout")
	}
	pid = session.command.Process.Pid
	session.Close()
	if _, ok, err := read("ok4", time.Second); ok || err != nil || session.command != nil || !gone(pid) {
		t.Fatalf("closed session served or kept its child: %t %v", ok, err)
	}
}

// TestSessionStopIsBoundedWhenEscapedDescendantHoldsPipes proves a hung
// request whose descendant left the child's group holding stdout and stderr
// still times out without the stop blocking on those pipes.
func TestSessionStopIsBoundedWhenEscapedDescendantHoldsPipes(t *testing.T) {
	previous := pipeDrainDelay
	pipeDrainDelay = 100 * time.Millisecond
	t.Cleanup(func() { pipeDrainDelay = previous })
	options := shell()
	options.Dir = t.TempDir()
	session := &Session{}
	admit := func(string, []string, int) bool { return true }
	started := time.Now()
	_, _, ok, err := session.Read(context.Background(), 300*time.Millisecond, options,
		[]string{"-c", "set -m; sleep 30 & echo $! > escaped; " + fakeBatch}, "hang", admit)
	elapsed := time.Since(started)
	if data, readErr := os.ReadFile(options.Dir + "/escaped"); readErr == nil {
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	if ok || cemcode.CodeOf(err) != cemcode.GitTimeout || session.command != nil || elapsed > 10*time.Second {
		t.Fatalf("hung request: %t %v after %s", ok, err, elapsed)
	}
}
