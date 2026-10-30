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
	if err := os.WriteFile(filepath.Join(assets, "observe.mjs"), []byte(descendantObserverScript), 0600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"scan.mjs", "lifecycle.mjs", "proxy-headers.mjs", "package-lock.json"} {
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
	pids, err := awaitObserverDescendants(root, done, func() error { return observedErr }, descendantReadinessBound)
	if err != nil {
		t.Fatal(err)
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

// descendantObserverScript is the fixture provider: it starts a child that
// starts a grandchild, and each parent publishes its descendant's pid only on
// the 'spawn' event, so a pid file names a process that exec'd. Publication
// writes a temporary name and renames it, so a reader never sees a partial
// file. A failed spawn or an early child exit is published too, so readiness
// fails at once with the cause instead of waiting out its bound (V1-2007).
const descendantObserverScript = `import {spawn} from 'node:child_process';
import {renameSync, writeFileSync} from 'node:fs';
const publish = (name, value) => { writeFileSync(name + '.tmp', value); renameSync(name + '.tmp', name); };
const child = spawn(process.execPath, ['-e', "const {spawn}=require('node:child_process');const fs=require('node:fs');const publish=(n,v)=>{fs.writeFileSync(n+'.tmp',v);fs.renameSync(n+'.tmp',n);};const c=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'});c.on('spawn',()=>publish('grandchild.pid',String(c.pid)));c.on('error',e=>publish('grandchild.error',String(e)));setInterval(()=>{},1000);"], {stdio:'ignore'});
child.on('spawn', () => publish('child.pid', String(child.pid)));
child.on('error', (err) => publish('child.error', String(err)));
child.on('exit', (code, signal) => publish('child.exit', String(code) + ' ' + String(signal)));
setInterval(()=>{},1000);
`

// descendantReadinessBound only detects a provider that never becomes ready:
// readiness returns as soon as both pids are published, and an observer that
// returns or a descendant that fails fails at once. Two Node start-ups took
// over 10 s once on a loaded hosted runner (V1-2007), so the bound is wide;
// interruption cleanup, which the test proves, has its own bounds.
const descendantReadinessBound = 60 * time.Second

// awaitObserverDescendants polls root for the child and grandchild pids the
// fixture provider publishes. It fails at once when the observer returns or a
// descendant reports a failed spawn or an early exit, and at bound otherwise;
// each failure names every readiness file's state and the observer's state.
func awaitObserverDescendants(root string, done <-chan struct{}, observed func() error, bound time.Duration) ([]int, error) {
	deadline := time.Now().Add(bound)
	for {
		pids, states := readDescendantPids(root)
		if len(pids) == 2 {
			return pids, nil
		}
		select {
		case <-done:
			return nil, fmt.Errorf("observer returned before its descendants were ready (%v): %s", observed(), states)
		default:
		}
		for _, name := range []string{"child.error", "grandchild.error", "child.exit"} {
			if b, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				return nil, fmt.Errorf("observer descendants failed before ready (%s %q; observer still running): %s", name, strings.TrimSpace(string(b)), states)
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("observer descendants not ready after %v (observer still running): %s", bound, states)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readDescendantPids returns the parsed pids in child, grandchild order and a
// description of each pid file's state.
func readDescendantPids(root string) ([]int, string) {
	var pids []int
	var states []string
	for _, name := range []string{"child.pid", "grandchild.pid"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			states = append(states, name+" missing")
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil || pid <= 0 {
			states = append(states, fmt.Sprintf("%s unparsable %q", name, b))
			continue
		}
		pids = append(pids, pid)
		states = append(states, fmt.Sprintf("%s %d", name, pid))
	}
	return pids, strings.Join(states, ", ")
}

// AFU-V0-010: the readiness wait names the missing descendant and the
// observer's state instead of a bare timeout (V1-2007).
func TestAFUV0NoDescendantsReadinessDiagnostics(t *testing.T) {
	write := func(t *testing.T, root, name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	running := make(chan struct{})
	for _, tc := range []struct {
		name  string
		files map[string]string
		done  bool
		want  []string
	}{
		{"observer returned", map[string]string{"child.pid": "41"}, true,
			[]string{"observer returned before its descendants were ready (provider failed)", "child.pid 41", "grandchild.pid missing"}},
		{"grandchild spawn failed", map[string]string{"child.pid": "41", "grandchild.error": "Error: spawn EAGAIN"}, false,
			[]string{`grandchild.error "Error: spawn EAGAIN"`, "observer still running", "grandchild.pid missing"}},
		{"child exited", map[string]string{"child.pid": "41", "child.exit": "1 null"}, false,
			[]string{`child.exit "1 null"`, "grandchild.pid missing"}},
		{"bound elapsed", map[string]string{"child.pid": "41", "grandchild.pid": "undefined"}, false,
			[]string{"not ready after 50ms (observer still running)", "child.pid 41", `grandchild.pid unparsable "undefined"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, value := range tc.files {
				write(t, root, name, value)
			}
			done := running
			if tc.done {
				closed := make(chan struct{})
				close(closed)
				done = closed
			}
			pids, err := awaitObserverDescendants(root, done, func() error { return errors.New("provider failed") }, 50*time.Millisecond)
			if err == nil || pids != nil {
				t.Fatalf("readiness = %v, %v; want a refusal", pids, err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("readiness error %q lacks %q", err, want)
				}
			}
		})
	}
	root := t.TempDir()
	write(t, root, "child.pid", "41\n")
	write(t, root, "grandchild.pid", "42")
	pids, err := awaitObserverDescendants(root, running, func() error { return nil }, time.Second)
	if err != nil || len(pids) != 2 || pids[0] != 41 || pids[1] != 42 {
		t.Fatalf("readiness = %v, %v; want [41 42]", pids, err)
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
