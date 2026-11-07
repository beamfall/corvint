// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// LPCV-V0-7600: retain the prerequisite even when C reaches the same block
// alone. An unrelated test distinguishes attribution from a suite union.
func TestInOrderWindows(t *testing.T) {
	for _, tc := range []struct {
		name, source, tests, want string
		line                      int
	}{
		{"unrelated", fixture["ord/ord.go"], fixture["ord/ord_test.go"] + "\nfunc TestC(t *testing.T) {}\n", "TestA,TestB", 8},
		{"shared-isolated-block", `package ord
var ready bool
func g() int {
 return 1
}
`, `package ord
import "testing"
func TestA(t *testing.T) { ready = true }
func TestB(t *testing.T) { if ready { g() } }
func TestC(t *testing.T) { g() }
`, "TestA,TestB,TestC", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireGo(t)
			root := t.TempDir()
			tree := map[string]string{"go.mod": fixture["go.mod"], "ord/ord.go": tc.source, "ord/ord_test.go": tc.tests + "\nfunc TestD(t *testing.T) {}\n"}
			writeTree(t, root, tree)
			git(t, root, "init", "-q")
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "base")
			base := git(t, root, "rev-parse", "HEAD")
			out, err := Collect(context.Background(), root, CollectOptions{Module: "example.com/m", Packages: []string{"ord"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			var records []Record
			for _, o := range out {
				if o.Record == nil || o.WindowFallback != "" {
					t.Fatalf("outcome %+v", o)
				}
				records = append(records, *o.Record)
			}
			oids, err := OIDs(context.Background(), root, base)
			if err != nil {
				t.Fatal(err)
			}
			ix, err := Build("example.com/m", base, "test", oids, records)
			if err != nil {
				t.Fatal(err)
			}
			change := ix.Analyze([]FileDiff{{Path: "ord/ord.go", OldOID: oids["ord/ord.go"], NewOID: "changed", Hunks: []Hunk{{Start: tc.line, End: tc.line}}}}, Reader(context.Background(), root, base))
			p, why := ix.Narrow([]string{"ord"}, map[string][]string{"ord": {"TestA", "TestB", "TestC", "TestD"}}, Effect{}, change)
			if why != "" || p["ord"].Mode != Tests || strings.Join(p["ord"].Tests, ",") != tc.want {
				t.Fatalf("plans %+v why %s", p, why)
			}
			// Unsupported overlapping windows retain a named union fallback.
			if err := os.WriteFile(filepath.Join(root, "ord", "ord_test.go"), []byte(strings.Replace(tree["ord/ord_test.go"], "TestD(t *testing.T) {}", "TestD(t *testing.T) { t.Parallel() }", 1)), 0600); err != nil {
				t.Fatal(err)
			}
			_, reason, err := windowOverlay(filepath.Join(root, "ord"), t.TempDir())
			if err != nil || reason != "parallel-test-windows" {
				t.Fatalf("fallback %s %v", reason, err)
			}
		})
	}
}

// LPCV-V0-7600: a root reached through a symlink (macOS TMPDIR under /var is
// one) still builds its window overlay, so ordering stays available.
func TestInOrderWindowsSymlinkedRoot(t *testing.T) {
	requireGo(t)
	real := t.TempDir()
	writeTree(t, real, map[string]string{"go.mod": fixture["go.mod"], "ord/ord.go": fixture["ord/ord.go"], "ord/ord_test.go": fixture["ord/ord_test.go"]})
	root := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, root); err != nil {
		t.Skipf("symlink: %v", err)
	}
	out, err := Collect(context.Background(), root, CollectOptions{Module: "example.com/m", Packages: []string{"ord"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range out {
		if o.Record == nil || o.WindowFallback != "" {
			t.Fatalf("outcome %+v", o)
		}
	}
}

// LPCV-V0-7600: retain the original dump failure even if the corresponding
// test window is consequently absent, or its order marker is unavailable.
func TestInvalidWindowDumpReason(t *testing.T) {
	for _, order := range []bool{false, true} {
		t.Run(strconv.FormatBool(order), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "TestA"), 0700); err != nil {
				t.Fatal(err)
			}
			if order {
				if err := os.WriteFile(filepath.Join(dir, "order"), []byte("TestA\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out := attributeWindows(func(string) ([]byte, error) { return nil, errors.New("invalid dump") }, []Outcome{
				{Package: "p", Name: "TestA", Record: &Record{}},
				{Package: "p", Name: InOrder, Record: &Record{}},
			}, map[string]string{"p": dir})
			if got := out[1].WindowFallback; got != "window-dump-invalid" {
				t.Fatalf("fallback %q", got)
			}
		})
	}
}

// LPCV-V0-7601: an executable without meta-data must not publish a parent-only
// record, even when its build explicitly strips GOFLAGS.
func TestUninstrumentedChild(t *testing.T) {
	requireGo(t)
	for _, mode := range []string{"covered", "covered-delayed-wait", "partial", "partial-replace", "partial-replace-wait", "partial-replace-first-wait", "cleared", "prebuilt", "missing-second"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			tree := map[string]string{}
			for p, src := range fixture {
				if !strings.HasPrefix(p, "ord/") {
					tree[p] = src
				}
			}
			if mode == "cleared" {
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], "bin := filepath.Join", `t.Setenv("GOFLAGS", "")
 bin := filepath.Join`, 1)
			}
			if mode == "partial" {
				// The helper still emits counters, but calc.Double is not
				// instrumented. Its caller must never publish a partial record.
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], `"build", "-o"`, `"build", "-coverpkg=example.com/m/cmd/tool", "-o"`, 1)
			}
			if mode == "partial-replace" {
				// A prior full build at the same path must not authorize different
				// bytes from a later partial build.
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], "out, err := exec.Command(bin).Output()", `if out,err:=exec.Command("go","build","-coverpkg=example.com/m/cmd/tool","-o",bin,"../cmd/tool").CombinedOutput();err!=nil{t.Fatalf("partial rebuild: %v %s",err,out)}
 out, err := exec.Command(bin).Output()`, 1)
			}
			if mode == "partial-replace-wait" {
				// LPCV-V0-7601/954: the original successful command cannot confer
				// its full-module proof on a partial replacement through Wait replay.
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], `if out, err := exec.Command("go", "build", "-o", bin, "../cmd/tool").CombinedOutput(); err != nil {`, `build := exec.Command("go", "build", "-o", bin, "../cmd/tool")
 if out, err := build.CombinedOutput(); err != nil {`, 1)
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], "out, err := exec.Command(bin).Output()", `if out,err:=exec.Command("go","build","-coverpkg=example.com/m/cmd/tool","-o",bin,"../cmd/tool").CombinedOutput();err!=nil{t.Fatalf("partial rebuild: %v %s",err,out)}
 if err:=build.Wait();err==nil{t.Fatal("repeated Wait must fail")}
 out, err := exec.Command(bin).Output()`, 1)
			}
			if mode == "covered-delayed-wait" || mode == "partial-replace-first-wait" {
				// EOF observes completion without calling Cmd.Wait. The first Wait
				// must not certify bytes installed by a subsequent partial build.
				src := strings.Replace(tree["e2e/e2e_test.go"], `"os/exec"`, `"os/exec"; "io"`, 1)
				src = strings.Replace(src, `if out, err := exec.Command("go", "build", "-o", bin, "../cmd/tool").CombinedOutput(); err != nil {`, `build := exec.Command("go", "build", "-o", bin, "../cmd/tool")
 stderr, err := build.StderrPipe(); if err != nil { t.Fatal(err) }
 if err := build.Start(); err != nil { t.Fatal(err) }
 if out, err := io.ReadAll(stderr); err != nil || len(out) != 0 {`, 1)
				beforeWait := ""
				if mode == "partial-replace-first-wait" {
					beforeWait = `if out,err:=exec.Command("go","build","-coverpkg=example.com/m/cmd/tool","-o",bin,"../cmd/tool").CombinedOutput();err!=nil{t.Fatalf("partial rebuild: %v %s",err,out)}
 `
				}
				tree["e2e/e2e_test.go"] = strings.Replace(src, "out, err := exec.Command(bin).Output()", beforeWait+`if err:=build.Wait();err!=nil{t.Fatalf("first Wait: %v",err)}
 out, err := exec.Command(bin).Output()`, 1)
			}
			if mode == "prebuilt" {
				tree["e2e/e2e_test.go"] = `package e2e
 import("os/exec";"testing")
 func TestTool(t *testing.T){if err:=exec.Command("../prebuilt-tool").Run();err!=nil{t.Fatal(err)}}
 `
			}
			if mode == "missing-second" {
				tree["cmd/tool/main.go"] = strings.Replace(tree["cmd/tool/main.go"], `"fmt"`, `"fmt"
 "os"
 "syscall"`, 1)
				tree["cmd/tool/main.go"] = strings.Replace(tree["cmd/tool/main.go"], "func main() {", `func main() {
 if os.Getenv("KILL_CHILD")=="1" { syscall.Kill(os.Getpid(),syscall.SIGKILL) }
 `, 1)
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], "out, err := exec.Command(bin).Output()", `out, err := exec.Command(bin).Output()
 t.Setenv("KILL_CHILD","1")
 if err:=exec.Command(bin).Run(); err==nil {t.Fatal("child should be killed")}
 `, 1)
			}
			writeTree(t, root, tree)
			git(t, root, "init", "-q")
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "base")
			if mode == "prebuilt" {
				cmd := exec.Command("go", "build", "-o", "prebuilt-tool", "./cmd/tool")
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "GOFLAGS=")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("prebuild: %v %s", err, out)
				}
			}
			out, err := Collect(context.Background(), root, CollectOptions{Module: "example.com/m", Packages: []string{"e2e"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range out {
				if mode == "covered" || mode == "covered-delayed-wait" {
					if o.Record == nil {
						t.Fatalf("covered child refused %+v", o)
					}
				} else {
					reason := "uninstrumented-child"
					if mode == "missing-second" {
						reason = "missing-child-counters"
					}
					if o.Record != nil || !strings.Contains(o.Divergent, reason) {
						t.Fatalf("unsafe record %+v", o)
					}
				}
			}
		})
	}
}

// LPCV-V0-7602: Build already accepts language-neutral record profiles. Ruby
// and Go records share deterministic bytes and the same public lookups.
func TestNeutralBuildLookups(t *testing.T) {
	records := []Record{{Package: "spec/a.rb", Name: "case", Profile: []byte("mode: atomic\nm/lib/a.rb:2.0,2.0 1 1\nm/lib/a.rb:1.1,4.4 1 1\n")}}
	a, err := Build("m", "commit", "ruby", map[string]string{"lib/a.rb": "oid"}, records)
	if err != nil {
		t.Fatal(err)
	}
	if b := a.BlocksAtLine("lib/a.rb", 2); len(b) != 2 {
		t.Fatalf("blocks %v", b)
	}
	if ts := a.TestsOfFile("lib/a.rb"); len(ts) != 1 || ts[0].Name != "case" {
		t.Fatalf("tests %v", ts)
	}
	if _, ok := a.FileByPath("missing"); ok {
		t.Fatal("unknown file found")
	}
	var x, y bytes.Buffer
	if err = a.Encode(&x); err != nil {
		t.Fatal(err)
	}
	b, err := Build("m", "commit", "ruby", map[string]string{"lib/a.rb": "oid"}, records)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.Encode(&y); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x.Bytes(), y.Bytes()) {
		t.Fatal("different bytes for equal records")
	}
}

func TestMergeCollectorIdentity(t *testing.T) {
	a, err := Build("m", "c", "runner", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build("m", "c", "runner", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a.TestTimeout = time.Second
	b.TestTimeout = 2 * time.Second
	if _, err := Merge(a, b); err == nil {
		t.Fatal("different recorded deadlines merged")
	}
	b.TestTimeout = a.TestTimeout
	a.Language = "ruby"
	b.Language = "ruby"
	a.Command = []string{"ruby", "a.rb"}
	b.Command = []string{"ruby", "b.rb"}
	if _, err := Merge(a, b); err == nil {
		t.Fatal("different suite commands merged")
	}
	b.Command = a.Command
	a.WindowFallback = map[string]string{"p": "parallel-test-windows"}
	b.WindowFallback = map[string]string{"q": "unsupported-test-entrypoint"}
	merged, err := Merge(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if merged.TestTimeout != time.Second || len(merged.WindowFallback) != 2 {
		t.Fatalf("lost collection metadata %+v", merged)
	}
}

// LPCV-V0-7600: a surviving goroutine runs after A's cleanup snapshot and
// before B's entry snapshot. B also reaches the block in isolation; that overlap
// must not erase A's ownership. No reset may destroy the suite-wide receipt.
func TestBetweenWindowReach(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"before-TestA", "TestA", "before-TestB", "TestB"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "order"), []byte("TestA\nTestB\n"), 0600); err != nil {
		t.Fatal(err)
	}
	profile := func(n int) []byte {
		return []byte("mode: atomic\nexample.com/m/p/p.go:3.1,3.20 1 " + strconv.Itoa(n) + "\n")
	}
	out := attributeWindows(func(path string) ([]byte, error) {
		switch filepath.Base(path) {
		case "before-TestA", "TestA":
			return profile(0), nil
		case "before-TestB":
			return profile(1), nil // asynchronous execution in the gap
		default:
			return profile(2), nil
		}
	}, []Outcome{
		{Package: "p", Name: "TestA", Record: &Record{Package: "p", Name: "TestA", Profile: profile(0)}},
		{Package: "p", Name: "TestB", Record: &Record{Package: "p", Name: "TestB", Profile: profile(1)}},
		{Package: "p", Name: InOrder, Record: &Record{Package: "p", Name: InOrder, Profile: profile(2)}},
	}, map[string]string{"p": dir})
	var records []Record
	for _, o := range out {
		if o.WindowFallback != "" {
			t.Fatalf("unexpected fallback: %+v", o)
		}
		records = append(records, *o.Record)
	}
	ix, err := Build("example.com/m", "base", "test", map[string]string{"p/p.go": "oid"}, records)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, test := range ix.TestsOfFile("p/p.go") {
		found = found || test.Name == "TestA"
	}
	if !found {
		t.Fatal("gap reach dropped the surviving goroutine's caller")
	}
	// The real overlay must preserve the independent suite exit counters too.
	writeTree(t, dir, map[string]string{"p.go": "package p\nfunc f() {}\n", "p_test.go": "package p\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n"})
	overlay, reason, err := windowOverlay(dir, t.TempDir())
	if err != nil || reason != "" {
		t.Fatalf("overlay: %s %v", reason, err)
	}
	var mapping struct{ Replace map[string]string }
	data, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &mapping); err != nil {
		t.Fatal(err)
	}
	for _, path := range mapping.Replace {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(source, []byte("ClearCounters")) || !bytes.Contains(source, []byte("before-TestA")) {
			t.Fatal("overlay can lose counters between test windows")
		}
	}
}
