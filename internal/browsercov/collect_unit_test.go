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

func TestPickSuffix(t *testing.T) {
	two := []string{"a/util.js", "b/util.js"}
	for _, c := range []struct {
		ps   []string
		src  string
		want string
	}{
		{[]string{"a/util.js"}, "webpack:///virtual/x.js", "a/util.js"},
		{two, "webpack:///b/util.js", "b/util.js"},
		{two, "webpack:///util.js", ""},
		{two, "webpack:///virtual/x.js", ""},
	} {
		if got := pickSuffix(c.ps, c.src); got != c.want {
			t.Errorf("pickSuffix(%v, %q) = %q, want %q", c.ps, c.src, got, c.want)
		}
	}
}

func TestUndumped(t *testing.T) {
	roles := map[int]string{10: "runner", 20: "process", 30: "process"}
	if got := undumped(roles, map[int]bool{20: true, 30: true}); got != "" {
		t.Errorf("all dumped: %q", got)
	}
	if got := undumped(roles, map[int]bool{20: true}); !strings.Contains(got, "process 30 ") {
		t.Errorf("killed child: %q", got)
	}
}

func TestRootRel(t *testing.T) {
	root := t.TempDir()
	got, err := rootRel(root, filepath.Join(root, "cypress", "e2e", "app.cy.js"))
	if err != nil || got != "cypress/e2e/app.cy.js" {
		t.Errorf("rootRel = %q, %v", got, err)
	}
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		if got, err := rootRel(root, filepath.Join(real, "a.cy.js")); err != nil || got != "a.cy.js" {
			t.Errorf("rootRel(resolved) = %q, %v", got, err)
		}
	}
	if _, err := rootRel(root, filepath.Join(filepath.Dir(root), "x.cy.js")); err == nil {
		t.Error("a spec outside the root was accepted")
	}
	if _, err := rootRel(root, "cypress/e2e/app.cy.js"); err == nil {
		t.Error("a relative spec was accepted")
	}
}
