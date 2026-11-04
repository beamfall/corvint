// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// LPCV-V0-7600: a third unrelated test makes a union fallback distinguishable
// from retaining just the order-dependent test and its prerequisite.
func TestInOrderWindows(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	tree := map[string]string{"go.mod": fixture["go.mod"], "ord/ord.go": fixture["ord/ord.go"], "ord/ord_test.go": fixture["ord/ord_test.go"] + "\nfunc TestC(t *testing.T) {}\n"}
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
	change := ix.Analyze([]FileDiff{{Path: "ord/ord.go", OldOID: oids["ord/ord.go"], NewOID: "changed", Hunks: []Hunk{{Start: 8, End: 8}}}}, Reader(context.Background(), root, base))
	p, why := ix.Narrow([]string{"ord"}, map[string][]string{"ord": {"TestA", "TestB", "TestC"}}, Effect{}, change)
	if why != "" || p["ord"].Mode != Tests || strings.Join(p["ord"].Tests, ",") != "TestA,TestB" {
		t.Fatalf("plans %+v why %s", p, why)
	}
	// Unsupported overlapping windows retain a named union fallback.
	if err := os.WriteFile(filepath.Join(root, "ord", "ord_test.go"), []byte(strings.Replace(tree["ord/ord_test.go"], "TestC(t *testing.T) {}", "TestC(t *testing.T) { t.Parallel() }", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	_, reason, err := windowOverlay(filepath.Join(root, "ord"), t.TempDir())
	if err != nil || reason != "parallel-test-windows" {
		t.Fatalf("fallback %s %v", reason, err)
	}
}

// LPCV-V0-7601: an executable without meta-data must not publish a parent-only
// record, even when its build explicitly strips GOFLAGS.
func TestUninstrumentedChild(t *testing.T) {
	requireGo(t)
	for _, mode := range []string{"covered", "cleared", "prebuilt", "missing-second"} {
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
				if mode == "covered" {
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
