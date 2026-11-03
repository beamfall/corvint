// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAloneOneTest(t *testing.T) {
	for _, tc := range []struct {
		ran  []result
		want string
	}{
		{[]result{{Name: "a", State: "passed"}}, ""},
		{[]result{{Name: "a", State: "passed"}, {Name: "b", State: "skipped"}}, ""},
		{[]result{{Name: "a", State: "passed"}, {Name: "a", State: "passed"}}, "running it alone ran 2 tests of that name"},
		{[]result{{Name: "a", State: "failed"}}, "failed when run alone"},
		{[]result{{Name: "a", State: "passed"}, {Name: "b", State: "passed"}}, `running it alone also ran ["b"]`},
		{nil, "did not run when run alone"},
	} {
		if got := alone(tc.ran, "a"); got != tc.want {
			t.Errorf("alone(%v) = %q, want %q", tc.ran, got, tc.want)
		}
	}
}

func TestDuplicateNames(t *testing.T) {
	got := duplicates([]result{{Name: "a"}, {Name: "b"}, {Name: "a", State: "skipped"}})
	if len(got) != 1 || !got["a"] {
		t.Fatalf("duplicates = %v, want only a", got)
	}
}

func TestAnyFailed(t *testing.T) {
	var pass, fail browserFile
	if err := json.Unmarshal([]byte(`{"tests":[{"title":["x"],"state":"passed"},{"title":["y"],"state":"pending"}]}`), &pass); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"tests":[{"title":["x"],"state":"passed"},{"title":["y"],"state":"failed"}]}`), &fail); err != nil {
		t.Fatal(err)
	}
	if anyFailed([]browserFile{pass}) || !anyFailed([]browserFile{pass, fail}) {
		t.Fatal("anyFailed must be true only when a test failed")
	}
}

// TestServerStopKillsLeftovers: a child that ignores SIGTERM outlives its
// shell, and stop still kills it after the grace period.
func TestServerStopKillsLeftovers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are unix-only")
	}
	old := serverGrace
	serverGrace = 300 * time.Millisecond
	defer func() { serverGrace = old }()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	c := &collector{Options: Options{Root: dir, Server: `(trap '' TERM; exec sleep 60) & echo $! >` + pidFile + `; wait`}}
	stop, err := c.startServer(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	for i := 0; i < 100 && pid == 0; i++ {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("child never started")
	}
	stop()
	for i := 0; i < 50 && alive(pid); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if alive(pid) {
		_ = signalPid(pid, sigKill)
		t.Fatalf("child %d survived stop", pid)
	}
}
