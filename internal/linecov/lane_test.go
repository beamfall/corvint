// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"context"
	"os"
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
	for _, cleared := range []bool{false, true} {
		t.Run(map[bool]string{false: "covered", true: "cleared"}[cleared], func(t *testing.T) {
			root := t.TempDir()
			tree := map[string]string{}
			for p, s := range fixture {
				if strings.HasPrefix(p, "ord/") {
					continue
				}
				tree[p] = s
			}
			if cleared {
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], "\"os/exec\"", "\"os/exec\"\n\"os\"", 1)
				tree["e2e/e2e_test.go"] = strings.Replace(tree["e2e/e2e_test.go"], "bin := filepath.Join", `t.Setenv("GOFLAGS", "")
 _ = os.Getenv("GOFLAGS")
 bin := filepath.Join`, 1)
			}
			writeTree(t, root, tree)
			git(t, root, "init", "-q")
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "base")
			out, err := Collect(context.Background(), root, CollectOptions{Module: "example.com/m", Packages: []string{"e2e"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range out {
				if cleared {
					if o.Record != nil || !strings.Contains(o.Divergent, "uninstrumented-child") {
						t.Fatalf("unsafe record %+v", o)
					}
				} else if o.Record == nil {
					t.Fatalf("covered child refused %+v", o)
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
