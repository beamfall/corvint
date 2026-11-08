// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// GLS-V0-009: only admitted flags may describe the collected sources.
func TestCallerOverlayRefused(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": fixture["go.mod"]})
	for _, flags := range []string{"-overlay=caller.json", "--overlay=caller.json", "-future-build-transform=unknown"} {
		if err := checkGoFlags(flags); err == nil {
			t.Fatalf("admitted %s", flags)
		}
		if _, _, err := RunnerID(context.Background(), root, append(os.Environ(), "GOFLAGS="+flags), DefaultTestTimeout, ""); err == nil {
			t.Fatalf("runner admitted %s", flags)
		}
		if _, err := Collect(context.Background(), root, CollectOptions{GOFLAGS: flags}); err == nil {
			t.Fatalf("collector admitted %s", flags)
		}
	}
}

// GLS-V0-008: the ordered launch has no isolated owner. Explicit Env also
// prevents an incidental whole-environment binding from hiding the gap.
func TestOrderedChildMutation(t *testing.T) {
	for _, mode := range []string{"go-child", "unknown-launcher"} {
		t.Run(mode, func(t *testing.T) { testOrderedChildMutation(t, mode) })
	}
}

func testOrderedChildMutation(t *testing.T, mode string) {
	requireGo(t)
	ctx := context.Background()
	root := t.TempDir()
	source := "package lib\nfunc F() int { return 1 }\n"
	writeTree(t, root, map[string]string{
		"go.mod": fixture["go.mod"], "lib/lib.go": source,
		"cmd/kid/main.go": `package main
import "example.com/m/lib"
func main() { if lib.F() != 1 { panic("mutation") } }
`,
		"ord/ord.go": "package ord\nvar ready bool\n",
		"ord/ord_test.go": `package ord
import ("os/exec"; "testing")
func TestA(t *testing.T) { ready = true }
func TestB(t *testing.T) {
 if ready { c := exec.Command("../kid"); c.Env = []string{"PATH=/usr/bin:/bin"}; if err := c.Run(); err != nil { t.Fatal(err) } }
}
`,
	})
	if mode == "unknown-launcher" {
		p := filepath.Join(root, "ord/ord_test.go")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.Replace(string(data), `exec.Command("../kid")`, `exec.Command("/bin/sh", "-c", "../kid")`, 1))
		if err := os.WriteFile(p, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	build := func() {
		c := exec.Command("go", "build", "-o", "kid", "./cmd/kid")
		c.Dir = root
		c.Env = append(os.Environ(), "GOFLAGS=")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
	}
	build()
	out, err := Collect(ctx, root, CollectOptions{Module: "example.com/m", Packages: []string{"ord"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var records []Record
	for _, o := range out {
		if o.Name == InOrder {
			if o.Record != nil || !strings.Contains(o.Divergent, "uninstrumented-child") {
				t.Fatalf("unsafe suite: %+v", o)
			}
		} else {
			if o.Record == nil {
				t.Fatalf("isolated: %+v", o)
			}
			records = append(records, *o.Record)
		}
	}
	if len(records) != 2 {
		t.Fatalf("isolated records: %d", len(records))
	}
	oids, err := OIDs(ctx, root, base)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Build("example.com/m", base, "test", oids, records)
	if err != nil {
		t.Fatal(err)
	}
	mutant := strings.Replace(source, "return 1", "return 2", 1)
	writeTree(t, root, map[string]string{"lib/lib.go": mutant})
	build()
	changes := ix.Analyze([]FileDiff{{Path: "lib/lib.go", OldOID: oids["lib/lib.go"], NewOID: "changed", Hunks: []Hunk{{2, 2}}, NewHunks: []Hunk{{2, 2}}, NewSource: []byte(mutant)}}, Reader(ctx, root, base))
	plans, _ := ix.Narrow(os.Environ(), []string{"ord"}, map[string][]string{"ord": {"TestA", "TestB"}}, Effect{}, changes)
	if plans["ord"].Mode != Whole {
		t.Fatalf("ordered child omitted: %+v", plans)
	}
	if !failingTests(t, root, []string{"ord"})["ord.TestB"] {
		t.Fatal("mutation did not fail ordered child")
	}
}

// GLS-V0-017/018: direct and library syscall reads bind their inputs; an
// unknown whole-environment consumer cannot certify a finite set of names.
func TestSyscallEnvironmentMutation(t *testing.T) {
	requireGo(t)
	for _, tc := range []struct{ name, imports, condition, key, before, after string }{
		{"direct", `"syscall"`, `v, _ := syscall.Getenv("LINECOV_DIRECT"); v != "skip"`, "LINECOV_DIRECT", "skip", "run"},
		{"timezone", `"time"`, `_, offset := time.Now().Zone(); offset != 0`, "TZ", "UTC", "Etc/GMT-1"},
		{"cmd-environ", `"os/exec"; "strings"`, `skip := false; for _, kv := range exec.Command("unused").Environ() { if strings.HasPrefix(kv, "LINECOV_DIRECT=") { skip = strings.HasSuffix(kv, "skip") } }; !skip`, "LINECOV_DIRECT", "skip", "run"},
		{"unknown-consumer", `"syscall"; "strings"`, `skip := false; for _, kv := range syscall.Environ() { if strings.HasPrefix(kv, "LINECOV_DIRECT=") { skip = strings.HasSuffix(kv, "skip") } }; !skip`, "LINECOV_DIRECT", "skip", "run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			source := "package p\nfunc F() int { return 1 }\n"
			// Put the initialization before the final condition into the test body.
			at := strings.LastIndex(tc.condition, ";")
			setup, condition := tc.condition[:at+1], tc.condition[at+1:]
			writeTree(t, root, map[string]string{"go.mod": fixture["go.mod"], "p/p.go": source, "p/p_test.go": "package p\nimport (\"testing\"; " + tc.imports + ")\nfunc TestF(t *testing.T) { " + setup + " if " + condition + " { if F() != 1 { t.Fatal(\"mutation\") } } }\n"})
			git(t, root, "init", "-q")
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "base")
			base := git(t, root, "rev-parse", "HEAD")
			t.Setenv(tc.key, tc.before)
			out, err := Collect(ctx, root, CollectOptions{Module: "example.com/m", Packages: []string{"p"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			var records []Record
			for _, o := range out {
				if o.Record == nil {
					t.Fatalf("collection: %+v", o)
				}
				records = append(records, *o.Record)
				want := EnvBinding{tc.key, EnvHash(tc.before)}
				if tc.name == "unknown-consumer" || tc.name == "cmd-environ" {
					want = EnvBinding{EnvAll, EnvModified}
				}
				found := false
				for _, b := range o.Record.Env {
					found = found || b == want
				}
				if !found {
					t.Fatalf("missing binding %v in %v", want, o.Record.Env)
				}
			}
			oids, err := OIDs(ctx, root, base)
			if err != nil {
				t.Fatal(err)
			}
			ix, err := Build("example.com/m", base, "test", oids, records)
			if err != nil {
				t.Fatal(err)
			}
			mutant := strings.Replace(source, "return 1", "return 2", 1)
			if err := os.WriteFile(filepath.Join(root, "p/p.go"), []byte(mutant), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(tc.key, tc.after)
			changes := ix.Analyze([]FileDiff{{Path: "p/p.go", OldOID: oids["p/p.go"], NewOID: "changed", Hunks: []Hunk{{2, 2}}, NewHunks: []Hunk{{2, 2}}, NewSource: []byte(mutant)}}, Reader(ctx, root, base))
			plans, _ := ix.Narrow(os.Environ(), []string{"p"}, map[string][]string{"p": {"TestF"}}, Effect{}, changes)
			if plans["p"].Mode != Whole {
				t.Fatalf("environment mutation omitted: %+v", plans)
			}
			if !failingTests(t, root, []string{"p"})["p.TestF"] {
				t.Fatal("environment mutation did not fail")
			}
		})
	}
}
