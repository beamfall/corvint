// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// GLS-V0-008: a test binary that declares no named test still runs its
// TestMain, init and package variables. Without a whole suite record of that
// binary no record can show a change misses it, so the unit runs whole in both
// Plan and Narrow; with one, a change it reaches keeps the unit whole and a
// change it misses excludes it.
func TestSuiteOnlyPackageKeptWithoutSuiteRecord(t *testing.T) {
	src := func(pkg string) string { return "package " + pkg + "\n\nfunc F() int {\n\tx := 1\n\treturn x\n}\n" }
	files := map[string]string{"m/m.go": src("m"), "o/o.go": src("o")}
	read := func(p string) ([]byte, error) { return []byte(files[p]), nil }
	diff := func(p string) []FileDiff {
		return []FileDiff{{Path: p, OldOID: "x", NewOID: "y", NewSource: []byte(files[p]), NewHunks: []Hunk{{4, 4}}, Hunks: []Hunk{{4, 4}}}}
	}
	other := []Record{
		{Package: "o", Name: "TestO", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/o/o.go:4.2,5.10 2 1\n")},
		{Package: "o", Name: InOrder, Union: true, EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/o/o.go:4.2,5.10 2 1\n")},
	}
	suite := Record{Package: "m", Name: InOrder, Union: true, EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/m/m.go:4.2,5.10 2 1\n")}
	incomplete := suite
	incomplete.Incomplete = true
	heads := map[string][]string{"m": {}}
	cases := []struct {
		name     string
		suite    *Record
		changed  string
		mode     Mode
		category string
	}{
		{"no suite record", nil, "o/o.go", Whole, CategorySuiteUnrecorded},
		{"incomplete suite record", &incomplete, "o/o.go", Whole, CategorySuiteUnrecorded},
		{"suite record reaches the change", &suite, "m/m.go", Whole, CategoryOrderSensitive},
		{"suite record misses the change", &suite, "o/o.go", Exclude, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recs := append([]Record(nil), other...)
			if tc.suite != nil {
				recs = append(recs, *tc.suite)
			}
			ix, err := Build("example.com/m", "c0", "go test", map[string]string{"m/m.go": "x", "o/o.go": "x"}, recs)
			if err != nil || !ix.EnvBound {
				t.Fatalf("build: bound=%v %v", ix != nil && ix.EnvBound, err)
			}
			sel := ix.Plan(PlanInput{Base: "c0", Head: "c1", Change: diff(tc.changed), ReadDrift: read, ReadChange: read,
				Units:   []Unit{{ID: "go:example.com/m/m", Dir: "m", Witness: "m/m.go"}},
				Heads:   func([]string) (map[string][]string, error) { return heads, nil },
				Environ: []string{"HOME=/h"}})
			if sel.State != StateNarrowed || len(sel.Units) != 1 || sel.Units[0].Mode != tc.mode || sel.Units[0].Category != tc.category {
				t.Fatalf("plan %s %s %+v", sel.State, sel.Reason, sel.Units)
			}
			plans, reason := ix.Narrow([]string{"HOME=/h"}, []string{"m"}, heads, Effect{}, ix.Analyze(diff(tc.changed), read))
			if reason != "" || plans["m"].Mode != tc.mode {
				t.Fatalf("narrow %+v %q", plans, reason)
			}
		})
	}
}

// GLS-V0-008: the collector records a suite run for a test binary whose
// package declares only TestMain, and the record shows what TestMain reaches.
// A suite-only run that starts an uninstrumented child has no tests to keep
// in its place, so it diverges and the package stays unordered.
func TestCollectSuiteOnlyPackage(t *testing.T) {
	requireGo(t)
	tree := map[string]string{
		"go.mod":          fixture["go.mod"],
		"tm/tm.go":        "package tm\n\nvar ready bool\n\nfunc Init() {\n\tready = true\n}\n\nfunc Unused() int {\n\tx := 1\n\treturn x\n}\n",
		"tm/tm_test.go":   "package tm\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) {\n\tInit()\n\tos.Exit(m.Run())\n}\n",
		"child/main.go":   "package main\n\nfunc main() {}\n",
		"kid/kid.go":      "package kid\n\nfunc K() {}\n",
		"kid/kid_test.go": "package kid\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) {\n\tif err := exec.Command(\"../prebuilt-child\").Run(); err != nil {\n\t\tpanic(err)\n\t}\n\tK()\n\tos.Exit(m.Run())\n}\n",
	}
	root := t.TempDir()
	writeTree(t, root, tree)
	git(t, root, "init", "-q")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	build := exec.Command("go", "build", "-o", "prebuilt-child", "./child")
	build.Dir = root
	build.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("prebuild: %v %s", err, out)
	}
	out, err := Collect(context.Background(), root, CollectOptions{Module: "example.com/m", Packages: []string{"tm", "kid"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var tm, kid []Outcome
	for _, o := range out {
		switch o.Package {
		case "tm":
			tm = append(tm, o)
		case "kid":
			kid = append(kid, o)
		}
	}
	if len(tm) != 1 || tm[0].Name != InOrder || tm[0].Record == nil {
		t.Fatalf("TestMain-only package outcomes %+v", tm)
	}
	reached := false
	for _, line := range strings.Split(string(tm[0].Record.Profile), "\n") {
		if strings.HasPrefix(line, "example.com/m/tm/tm.go:6.") && !strings.HasSuffix(line, " 0") {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("suite record does not show TestMain reaching Init:\n%s", tm[0].Record.Profile)
	}
	if len(kid) != 1 || kid[0].Name != InOrder || kid[0].Record != nil || !strings.Contains(kid[0].Divergent, "uninstrumented-child") {
		t.Fatalf("suite-only run over an uninstrumented child kept a record: %+v", kid)
	}

	ix, err := Build("example.com/m", "c0", "go test", map[string]string{"tm/tm.go": "x"}, []Record{*tm[0].Record})
	if err != nil {
		t.Fatal(err)
	}
	read := func(string) ([]byte, error) { return []byte(tree["tm/tm.go"]), nil }
	heads := map[string][]string{"tm": {}}
	// A change the suite record misses in the package itself only compiles it.
	for line, want := range map[int]Mode{6: Whole, 10: Compile} {
		d := []FileDiff{{Path: "tm/tm.go", OldOID: "x", NewOID: "y", NewSource: []byte(tree["tm/tm.go"]), NewHunks: []Hunk{{line, line}}, Hunks: []Hunk{{line, line}}}}
		if plans, reason := ix.Narrow(nil, []string{"tm"}, heads, Effect{}, ix.Analyze(d, read)); reason != "" || plans["tm"].Mode != want {
			t.Errorf("change at tm.go:%d: %+v %q, want %s", line, plans, reason, want)
		}
	}
}
