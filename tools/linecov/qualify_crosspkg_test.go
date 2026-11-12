// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// crossPackageFixture is GLS-V0-035's seeded-fault fixture set: package lib holds a site of every
// kind (tagless case expressions, an else-if, loop headers, a closure statement, plain statements
// and constants) and an unexecuted body for the control; its own test exercises only Total, so a
// fault in Grade or Map fails only a test of package app, which imports lib. A mutant there is
// killed across the package boundary, and the selector must keep app's test through the record
// that `-coverpkg` gave it of lib's blocks.
var crossPackageFixture = map[string]string{
	"go.mod": "module example.com/x\n\ngo 1.22\n",
	"lib/lib.go": `package lib

func Grade(n int) string {
	switch {
	case n >= 90:
		return "A"
	case n >= 50:
		return "B"
	}
	if n < 0 {
		return "invalid"
	} else if n < 10 {
		return "low"
	}
	return "C"
}

func Total(xs []int) int {
	t := 0
	for i := 0; i < len(xs); i++ {
		t += xs[i]
	}
	return t
}

func Map(xs []int, f func(int) int) []int {
	out := make([]int, 0, len(xs))
	visit(xs, func(x int) {
		out = append(out, f(x))
	})
	return out
}

func visit(xs []int, g func(int)) {
	for _, x := range xs {
		g(x)
	}
}

func Idle(n int) int {
	n--
	return n
}
`,
	"lib/lib_test.go": `package lib

import "testing"

func TestTotal(t *testing.T) {
	if Total([]int{1, 2, 3}) != 6 {
		t.Fatal("total")
	}
}
`,
	"app/app.go": `package app

import "example.com/x/lib"

func Report(n int) string { return "grade " + lib.Grade(n) }

func Scale(xs []int) []int { return lib.Map(xs, func(x int) int { return 3 * x }) }
`,
	"app/app_test.go": `package app

import "testing"

func TestReport(t *testing.T) {
	for n, want := range map[int]string{95: "grade A", 60: "grade B", -1: "grade invalid", 5: "grade low", 20: "grade C"} {
		if got := Report(n); got != want {
			t.Fatalf("Report(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestScale(t *testing.T) {
	if got := Scale([]int{1, 2}); len(got) != 2 || got[1] != 6 {
		t.Fatalf("scale %v", got)
	}
}
`,
}

func crossPackageModule(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixture shells out to POSIX paths")
	}
	root := t.TempDir()
	for name, body := range crossPackageFixture {
		write(t, root, name, body)
	}
	run(t, root, "git", "init", "-q")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	return root
}

// GLS-V0-035 (V1-4365): every site of the cross-package fixture, run against the whole suite,
// misses nothing; at least one mutant of lib of each kind is valid, a lib mutant is killed only
// by app's tests and kept for them, and the control on the unexecuted body keeps and fails none.
func TestQualifyCrossPackageFixtureHasNoMiss(t *testing.T) {
	root := crossPackageModule(t)
	index := filepath.Join(t.TempDir(), "index")
	if err := collect([]string{"-root", root, "-out", index, "-parallel", "4", "-test-timeout", "20s"}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	if err := qualify([]string{"-root", root, "-index", index, "-packages", "lib", "-suite", "lib,app", "-start", "0", "-count", "1000", "-controls", "1", "-test-timeout", "20s", "-report", report}); err != nil {
		t.Fatal(err)
	}
	r := readReport(t, report)
	c := r.Counts
	if c.Mutants < 10 || c.Killed == 0 || c.Misses != 0 || c.Controls != 1 || c.ControlsFailed != 0 || r.Sites != r.Order {
		t.Fatalf("counts %+v sites %d order %d", c, r.Sites, r.Order)
	}
	for _, kind := range siteKinds {
		if c.ByKind[kind] == 0 {
			t.Errorf("no valid %s mutant: %v", kind, c.ByKind)
		}
	}
	crossKills := 0
	for _, m := range r.Mutants {
		if m.State != "killed" || !strings.HasPrefix(m.Site.Path, "lib/") {
			continue
		}
		if len(m.Missed) != 0 {
			t.Errorf("mutant %+v missed %v", m.Site, m.Missed)
		}
		own := false
		for _, failing := range m.Failing {
			own = own || strings.HasPrefix(failing, "lib.")
		}
		if !own && slices.Contains(m.Failing, "app.TestReport") {
			crossKills++
		}
	}
	if crossKills == 0 {
		t.Fatalf("no lib mutant was killed only by app.TestReport: %+v", r.Mutants)
	}
	if !strings.HasPrefix(r.Controls[0].Site.Path, "lib/") || r.Controls[0].KeptByCoverage != 0 || r.Controls[0].KeptOfPackage != 0 {
		t.Errorf("control %+v kept a test", r.Controls[0])
	}
}

// GLS-V0-035 (V1-4365): an index whose app.TestReport record lost Grade's blocks (and app's
// in-order record with it), now attributed to lib.TestTotal, must report the miss of a Grade mutant that app.TestReport fails, and
// qualify must fail: the harness sees misses across the package boundary, not only within one.
func TestQualifyReportsACrossPackageSeededMiss(t *testing.T) {
	root := crossPackageModule(t)
	ctx := context.Background()
	commit := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	const deadline = 20 * time.Second
	run, goflags, err := runner(root, deadline, "")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := linecov.Collect(ctx, root, linecov.CollectOptions{Module: "example.com/x", Packages: []string{"lib", "app"}, Work: t.TempDir(), Parallel: 4, Timeout: time.Minute, SuiteTimeout: time.Minute, TestTimeout: deadline, GOFLAGS: goflags})
	if err != nil {
		t.Fatal(err)
	}
	// Grade's blocks are lines 4 to 15 of lib.go. Qualify samples only functions some record
	// executed (GLS-V0-030), so the blocks move to lib.TestTotal's record, as the single-package
	// seeded miss moves Double's: Grade stays executed, but no record of app's reaches it.
	gradeBlocks := regexp.MustCompile(`(?m)^(example\.com/x/lib/lib\.go:([4-9]|1[0-5])\.\d+,\d+\.\d+ \d+) \d+$`)
	in := func(r linecov.Record, pkg string) bool {
		return r.Package == pkg || strings.HasSuffix(r.Package, "/"+pkg)
	}
	var records []linecov.Record
	var moved []byte
	for _, o := range outcomes {
		r := *o.Record
		if in(r, "app") && r.Name == "TestReport" {
			for _, m := range gradeBlocks.FindAllSubmatch(r.Profile, -1) {
				moved = append(append(moved, m[1]...), " 1\n"...)
			}
		}
		if in(r, "app") && (r.Name == "TestReport" || r.Name == linecov.InOrder) {
			r.Profile = gradeBlocks.ReplaceAll(r.Profile, []byte("$1 0"))
		}
		records = append(records, r)
	}
	if len(moved) == 0 {
		t.Fatalf("no Grade block in app's TestReport record among %d outcomes", len(outcomes))
	}
	attributed := false
	for i, r := range records {
		if in(r, "lib") && r.Name == "TestTotal" {
			records[i].Profile = append(append([]byte{}, r.Profile...), moved...)
			attributed = true
		}
	}
	if !attributed {
		t.Fatal("no lib TestTotal record")
	}
	oids, err := linecov.OIDs(ctx, root, commit)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/x", commit, run, oids, records)
	if err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(t.TempDir(), "index")
	var b bytes.Buffer
	if err := ix.Encode(&b); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	err = qualify([]string{"-root", root, "-index", index, "-packages", "lib", "-suite", "lib,app", "-start", "0", "-count", "1000", "-test-timeout", "20s", "-report", report})
	if err == nil || !strings.Contains(err.Error(), "misses") {
		t.Fatalf("qualify accepted a seeded cross-package miss: %v", err)
	}
	r := readReport(t, report)
	found := false
	for _, m := range r.Mutants {
		if strings.HasPrefix(m.Site.Path, "lib/") && slices.Contains(m.Missed, "app.TestReport") {
			found = true
		}
	}
	if r.Counts.Misses == 0 || !found {
		t.Fatalf("no miss of app.TestReport on a lib mutant: %+v", r.Counts)
	}
}
