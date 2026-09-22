//go:build darwin || linux

package appflows

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		if syscall.Kill(pid, 0) == nil {
			t.Errorf("descendant %d survived interruption", pid)
		}
	}
}
