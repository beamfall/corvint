//go:build darwin || linux

package appflows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// AFU-V0-010: interruption must retire the observer's child and grandchild.
func TestAFUV0NoDescendantsOnInterruption(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("optional Node runtime not installed; web-flows gate requires it")
	}
	root, _ := fixture(t)
	assets := t.TempDir()
	script := `import {spawn} from 'node:child_process';
import {writeFileSync} from 'node:fs';
const child = spawn(process.execPath, ['-e', "const {spawn}=require('node:child_process');const fs=require('node:fs'); const c=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'}); fs.writeFileSync('grandchild.pid',String(c.pid));setInterval(()=>{},1000);"], {stdio:'ignore'});
writeFileSync('child.pid',String(child.pid));
setInterval(()=>{},1000);
`
	if err := os.WriteFile(filepath.Join(assets, "observe.mjs"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"scan.mjs", "lifecycle.mjs", "package-lock.json"} {
		if err := os.WriteFile(filepath.Join(assets, p), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var observedErr error
	go func() { _, observedErr = Observe(ctx, root, "flows.json", assets, true); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("observer did not join")
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		pids = nil
		for _, name := range []string{"child.pid", "grandchild.pid"} {
			b, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				continue
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err == nil {
				pids = append(pids, pid)
			}
		}
		if len(pids) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(pids) != 2 {
		t.Fatal("observer descendants never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancellation timed out")
	}
	if observedErr == nil {
		t.Fatal("interrupted observation succeeded")
	}
	for _, pid := range pids {
		if state, retired := awaitDescendantRetired(pid, 2*time.Second); !retired {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("descendant %d survived interruption (%s)", pid, state)
		}
	}
}

// awaitDescendantRetired polls until pid is absent or an exited zombie, or
// until bound. The observer kills its descendants but does not reap them:
// once the leader exits they are orphans that PID 1 or the nearest subreaper
// reaps, so signal 0 can still reach their zombies when the interrupted
// observation returns (V1-1363). It returns the last state seen.
func awaitDescendantRetired(pid int, bound time.Duration) (string, bool) {
	deadline := time.Now().Add(bound)
	for {
		state := descendantState(pid)
		if state == "absent" || state == "zombie" {
			return state, true
		}
		if time.Now().After(deadline) {
			return state, false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// descendantState is "absent", "zombie" (exited, awaiting its reap) or the
// live state the process table reports. Only signal 0's ESRCH proves absence;
// a process table that cannot be read leaves the process unretired, and one
// that exits meanwhile reads ESRCH on the next poll. A Linux zombie leader
// whose other threads still run is live, as in groupreap's quiet proof
// (PGO-V0-006).
func descendantState(pid int) string {
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return "absent"
	}
	if runtime.GOOS == "linux" {
		stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		end := bytes.LastIndexByte(stat, ')')
		if err != nil || end < 0 || end+2 >= len(stat) {
			return fmt.Sprintf("state unreadable: %v", err)
		}
		state := string(stat[end+2])
		if state != "Z" && state != "X" {
			return "state " + state
		}
		status, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
		if err == nil && bytes.Contains(status, []byte("\nThreads:\t1\n")) {
			return "zombie"
		}
		return "state " + state + " with other threads or unreadable status"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	state := strings.TrimSpace(string(out))
	if strings.HasPrefix(state, "Z") {
		return "zombie"
	}
	if err != nil || state == "" {
		return fmt.Sprintf("state unreadable: %v", err)
	}
	return "state " + state
}
