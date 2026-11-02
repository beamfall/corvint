//go:build !windows

package changee2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// CCE-V0-102 (make cce-live): a SIGTERM while the workers run stops every worker and the
// processes they started, then removes the scratch directory, so an interrupted run leaves
// neither orphaned browsers or servers nor workers writing into a deleted directory.
func TestCCELiveScriptReapsWorkersOnSignal_CCEV0102(t *testing.T) {
	bin, marks, tmp := t.TempDir(), t.TempDir(), t.TempDir()
	// The fake test binary lists two live tests; each run starts a long-lived child (a stand-in
	// for a browser or server), records its pid and waits on it.
	fakeTest := "#!/bin/sh\n" +
		"case \"$1\" in -test.list) printf 'TestCCEV1Live\\nTestCCEV2Live\\n'; exit 0;; esac\n" +
		"sleep 300 &\n" +
		"echo $! > \"" + marks + "/$$.child\"\n" +
		"wait\n"
	fakeGo := "#!/bin/sh\n" +
		"# go test -c -o OUT .\n" +
		"cat > \"$4\" <<'EOF'\n" + fakeTest + "EOF\n" +
		"chmod +x \"$4\"\n"
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "cce-live.sh", "2", "1m")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TMPDIR="+tmp)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	children := func() []int {
		files, _ := filepath.Glob(filepath.Join(marks, "*.child"))
		pids := []int{}
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if pid, perr := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && perr == nil {
				pids = append(pids, pid)
			}
		}
		return pids
	}
	deadline := time.Now().Add(30 * time.Second)
	for len(children()) < 2 {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("the workers did not start:\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	pids := children()
	t.Cleanup(func() {
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Error("an interrupted run exited 0")
	}
	deadline = time.Now().Add(15 * time.Second)
	for _, pid := range pids {
		for syscall.Kill(pid, 0) == nil {
			if time.Now().After(deadline) {
				t.Fatalf("worker child %d still runs after the run was terminated:\n%s", pid, out.String())
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "cce-live.*")); len(left) != 0 {
		t.Errorf("scratch directory left behind: %v", left)
	}
}

// CCE-V0-102 (make cce-live): run to completion, the workers read every listed test and the run
// reports each one.
func TestCCELiveScriptRunsEveryListedTest_CCEV0102(t *testing.T) {
	bin := t.TempDir()
	fakeTest := "#!/bin/sh\n" +
		"case \"$1\" in -test.list) printf 'TestCCEV1Live\\nTestCCEV2Live\\n'; exit 0;; esac\n" +
		"name=$(echo \"$*\" | sed 's/.*\\^\\(TestCCEV[0-9]*Live\\).*/\\1/')\n" +
		"echo \"--- PASS: $name (0.00s)\"\n"
	fakeGo := "#!/bin/sh\ncat > \"$4\" <<'EOF'\n" + fakeTest + "EOF\nchmod +x \"$4\"\n"
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "cce-live.sh", "2", "1m")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ok cce-live: 2 live tests") || !strings.Contains(string(out), "--- PASS: TestCCEV2Live ") {
		t.Fatalf("%v\n%s", err, out)
	}
}
