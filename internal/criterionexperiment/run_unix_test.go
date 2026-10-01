//go:build darwin || linux

package criterionexperiment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func goRequest(t *testing.T) Request {
	t.Helper()
	r := requestFixture()
	p, e := exec.LookPath("go")
	if e != nil {
		t.Fatal(e)
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	r.GoBinary = p
	r.GoSha256 = Digest(b)
	return r
}

// CEX-V0-004: actual fixed argv execution and exact executable bytes.
func TestFixedRunnerActualPassFailureAndBinding(t *testing.T) {
	r := goRequest(t)
	c := r.Criteria[0]
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			body := `if st,e:=os.Stat("helper");e!=nil||st.Mode().Perm()!=0755{t.Fatal("mode mismatch")};`
			want := "pass"
			if fail {
				body += `t.Fatal("EXPECTED")`
				want = "expected-failure"
			}
			files := map[string][]byte{"go.mod": []byte("module example.test/sample\ngo 1.27.1\n"), "oracle_test.go": []byte("package sample\nimport (\"testing\";\"os\")\nfunc TestRepair(t *testing.T){" + body + "}\n")}
			files["helper"] = []byte("helper")
			stdout, stderr, exit, complete, e := runScenario(context.Background(), r, c, files, map[string]string{"helper": "100755"}, t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if got := Classify(stdout, exit, complete, c.Test, c.Assertion, "example.test/sample"); got != want {
				t.Fatalf("got %s exit=%d complete=%t stderr=%s stdout=%s", got, exit, complete, stderr, stdout)
			}
		})
	}
	r.GoSha256 = strings.Repeat("0", 64)
	if _, _, _, _, e := runScenario(context.Background(), r, c, nil, nil, t.TempDir()); e == nil {
		t.Fatal("wrong executable admitted")
	}
}

// CEX-V0-004: cancellation retires the ordinary process group.
func TestRunnerCancellationRetiresOrdinaryDescendants(t *testing.T) {
	r := goRequest(t)
	c := r.Criteria[0]
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	// The test process starts an ordinary same-group child. Cancellation must
	// retire both the Go command and its descendants, not merely the leader.
	src := fmt.Sprintf(`package sample
import("testing";"os";"os/exec";"strconv";"time")
func TestRepair(t *testing.T){c:=exec.Command("/bin/sleep","120");if e:=c.Start();e!=nil{t.Fatal(e)};if e:=os.WriteFile(%q,[]byte(strconv.Itoa(c.Process.Pid)),0600);e!=nil{t.Fatal(e)};time.Sleep(120*time.Second)}
`, pidfile)
	files := map[string][]byte{"go.mod": []byte("module example.test/sample\ngo 1.27.1\n"), "oracle_test.go": []byte(src)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan bool, 1)
	go func() { _, _, _, complete, _ := runScenario(ctx, r, c, files, nil, dir); done <- complete }()
	deadline := time.Now().Add(90 * time.Second)
	pid := 0
	for time.Now().Before(deadline) {
		if b, e := os.ReadFile(pidfile); e == nil {
			pid, _ = strconv.Atoi(string(b))
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case complete := <-done:
		if complete {
			t.Fatal("cancelled run complete")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner failed to stop")
	}
	if pid == 0 {
		t.Fatal("descendant never started")
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("descendant survived interruption")
}

// CEX-V0-004 and CEX-V0-005: output overflow promptly cancels execution and
// cannot create an eligible test failure even when the marker was printed.
func TestRunnerOutputOverflowCancelsGroup(t *testing.T) {
	r := goRequest(t)
	c := r.Criteria[0]
	files := map[string][]byte{"go.mod": []byte("module example.test/sample\ngo 1.27.1\n"), "oracle_test.go": []byte(`package sample
import("testing";"fmt";"strings";"time")
func TestRepair(t *testing.T){fmt.Print(strings.Repeat("X",2<<20));time.Sleep(120*time.Second)}
`)}
	started := time.Now()
	stdout, _, _, complete, e := runScenario(context.Background(), r, c, files, nil, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if complete || len(stdout) > 1<<20 || time.Since(started) > 30*time.Second {
		t.Fatalf("overflow not bounded: complete=%t bytes=%d elapsed=%s", complete, len(stdout), time.Since(started))
	}
}

// CEX-V0-005, CEX-REVIEW-001: real Go diagnostics with marker substrings or
// marker-bearing filenames are wrong assertions, never eligible control kills.
func TestFixedRunnerWrongAssertionDiagnostic(t *testing.T) {
	r := goRequest(t)
	c := r.Criteria[0]
	for _, tc := range []struct{ name, file, message string }{{"substring", "oracle_test.go", "UNEXPECTED"}, {"filename", "EXPECTED_test.go", "WRONG"}, {"nested source location", "wrong_test.go", "WRONG nested.go:9: EXPECTED"}} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string][]byte{"go.mod": []byte("module example.test/sample\ngo 1.27.1\n"), tc.file: []byte("package sample\nimport \"testing\"\nfunc TestRepair(t *testing.T){t.Fatal(" + strconv.Quote(tc.message) + ")}\n")}
			stdout, stderr, exit, complete, e := runScenario(context.Background(), r, c, files, nil, t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if !complete || exit != 1 {
				t.Fatalf("wrong assertion setup failed: complete=%t exit=%d stderr=%s", complete, exit, stderr)
			}
			if got := Classify(stdout, exit, complete, c.Test, c.Assertion, "example.test/sample"); got != "invalid-control" {
				t.Fatalf("wrong assertion classified %s", got)
			}
		})
	}
}
