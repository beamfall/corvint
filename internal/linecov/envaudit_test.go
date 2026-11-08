// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"os"

	"github.com/Beamfall/corvint/internal/execfile"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// GLS-V0-017: the audit parser binds each inherited read once, hashes the
// value, leaves neutral names out, binds a read of a collector-modified name,
// a `modified` line and a whole-environment read to EnvModified, and refuses
// a malformed line.
func TestEnvAuditParse(t *testing.T) {
	env := []string{"HOME=/h", "CI=true", "GOFLAGS=-race", "PWD=/x", "CORVINT_LINECOV_AUDIT=/a", "_=/usr/bin/env", "SHLVL=2"}
	data := "read\tCI\t\"true\"\nunset\tTERM\nread\tCI\t\"other\"\nread\tGOFLAGS\t\"-race\"\nunset\tPWD\nread\tCORVINT_LINECOV_AUDIT\t\"/a\"\nread\t_\t\"/usr/bin/env\"\nunset\tOLDPWD\nall\nmodified\tGOCOVERDIR\n"
	got, err := parseEnvAudit([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []EnvBinding{{EnvAll, EnvModified}, {"CI", EnvHash("true")}, {"CORVINT_LINECOV_AUDIT", EnvModified}, {"GOCOVERDIR", EnvModified}, {"GOFLAGS", EnvModified}, {"TERM", EnvUnset}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bindings %v, want %v", got, want)
	}
	if EnvAllHash(EnvValues(env)) != EnvAllHash(EnvValues([]string{"CI=true", "HOME=/h", "GOFLAGS=-tags=x", "PWD=/y", "_=/bin/go", "OLDPWD=/o"})) {
		t.Fatal("neutral variables took part in the environment hash")
	}
	if EnvAllHash(EnvValues(env)) == EnvAllHash(EnvValues([]string{"CI=true", "HOME=/other"})) {
		t.Fatal("a changed value left the environment hash unchanged")
	}
	for _, bad := range []string{"read\tCI\n", "read\tCI\tunquoted\n", "set\tCI\t\"x\"\n", "all\textra\n"} {
		if _, err := parseEnvAudit([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if _, err := parseEnvAudit([]byte("modified\t\n")); err == nil {
		t.Error("accepted a nameless modified line")
	}
	if got, err := parseEnvAudit(nil); err != nil || len(got) != 0 {
		t.Fatalf("empty audit: %v %v", got, err)
	}
}

// GLS-V0-018: a binding holds only under the recorded value; EnvUnbound
// names the first that does not.
func TestEnvUnbound(t *testing.T) {
	bindings := []EnvBinding{{"CI", EnvHash("true")}, {"TERM", EnvUnset}}
	if why := EnvUnbound(bindings, EnvValues([]string{"CI=true", "HOME=/h"})); why != "" {
		t.Fatalf("bound environment reported %q", why)
	}
	if why := EnvUnbound(bindings, EnvValues([]string{"HOME=/h"})); !strings.HasPrefix(why, "CI: recorded sha256:") {
		t.Fatalf("unset CI: %q", why)
	}
	if why := EnvUnbound(bindings, EnvValues([]string{"CI=true", "TERM=xterm"})); !strings.HasPrefix(why, "TERM: recorded unset") {
		t.Fatalf("set TERM: %q", why)
	}
	all := []EnvBinding{{EnvAll, EnvAllHash(EnvValues([]string{"A=1"}))}}
	if EnvUnbound(all, EnvValues([]string{"A=1", "GOFLAGS=x"})) != "" || EnvUnbound(all, EnvValues([]string{"A=2"})) == "" {
		t.Fatal("the whole-environment binding compares the environment hash")
	}
	// A read of what the collector set holds under no environment, not even
	// one that happens to carry the collector's literal marker value.
	for _, env := range [][]string{nil, {"GOCOVERDIR=/x"}, {"GOCOVERDIR=" + EnvModified}} {
		if why := EnvUnbound([]EnvBinding{{"GOCOVERDIR", EnvModified}}, EnvValues(env)); !strings.Contains(why, "collector set") {
			t.Fatalf("collector-modified binding held under %v: %q", env, why)
		}
	}
}

// GLS-V0-018: an index whose records carry no environment observation (built
// by hand, or by a collector before GLS-V0-017) binds no environment, so a
// plan under an environment is the environment-unbound FALLBACK and Narrow
// keeps every package whole; a nil environment compares nothing.
func TestUnboundIndexDecidesNothing(t *testing.T) {
	src := "package p\n\nfunc F() int {\n\tx := 1\n\treturn x\n}\n"
	recs := []Record{
		{Package: "p", Name: "TestF", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,5.10 2 1\n")},
		{Package: "p", Name: "TestG", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,5.10 2 0\n")},
		{Package: "p", Name: InOrder, Union: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,5.10 2 0\n")},
	}
	diff := []FileDiff{{Path: "p/p.go", OldOID: "x", NewOID: "y", NewSource: []byte(src), NewHunks: []Hunk{{4, 4}}, Hunks: []Hunk{{4, 4}}}}
	read := func(string) ([]byte, error) { return []byte(src), nil }
	heads := map[string][]string{"p": {"TestF", "TestG"}}
	unbound, err := Build("example.com/m", "c0", "go test", map[string]string{"p/p.go": "x"}, recs)
	if err != nil || unbound.EnvBound {
		t.Fatalf("hand-built records bound an environment: %v %v", unbound.EnvBound, err)
	}
	plans, reason := unbound.Narrow(os.Environ(), []string{"p"}, heads, Effect{}, unbound.Analyze(diff, read))
	if !strings.HasPrefix(reason, ReasonEnvironmentUnbound) || plans["p"].Mode != Whole {
		t.Fatalf("unbound index narrowed: %+v %q", plans, reason)
	}
	if plans, reason := unbound.Narrow(nil, []string{"p"}, heads, Effect{}, unbound.Analyze(diff, read)); reason != "" || plans["p"].Mode != Tests {
		t.Fatalf("nil environment compared something: %+v %q", plans, reason)
	}
	sel := unbound.Plan(PlanInput{Base: "c0", Head: "c1", Change: diff, ReadDrift: read, ReadChange: read, Units: []Unit{{ID: "go:example.com/m/p", Dir: "p", Witness: "p/p.go"}},
		Heads: func([]string) (map[string][]string, error) { return heads, nil }, Environ: os.Environ()})
	if sel.State != StateFallback || sel.Reason != ReasonEnvironmentUnbound {
		t.Fatalf("plan %s %s", sel.State, sel.Reason)
	}
	// The same records observed under an environment the run shares narrow;
	// a binding the run does not hold keeps the test and, through the
	// in-order record, the package.
	for i := range recs {
		recs[i].EnvObserved = true
	}
	recs[1].Env = []EnvBinding{{"LINECOV_TEST_G", EnvUnset}}
	bound, err := Build("example.com/m", "c0", "go test", map[string]string{"p/p.go": "x"}, recs)
	if err != nil || !bound.EnvBound || len(bound.Env) != 1 || len(bound.Tests[2].Env) != 1 { // tests sort as (in-order), TestF, TestG
		t.Fatalf("bound index %+v %v", bound, err)
	}
	if plans, reason := bound.Narrow([]string{"HOME=/h"}, []string{"p"}, heads, Effect{}, bound.Analyze(diff, read)); reason != "" || !reflect.DeepEqual(plans["p"].Tests, []string{"TestF"}) {
		t.Fatalf("bound index under its environment: %+v %q", plans, reason)
	}
	if plans, _ := bound.Narrow([]string{"LINECOV_TEST_G=1"}, []string{"p"}, heads, Effect{}, bound.Analyze(diff, read)); !reflect.DeepEqual(plans["p"].Tests, []string{"TestF", "TestG"}) || plans["p"].Why["TestG"] != WhyEnvironment {
		t.Fatalf("unbound test not kept: %+v", plans)
	}
	recs[2].Env = []EnvBinding{{"LINECOV_TEST_G", EnvUnset}}
	bound, _ = Build("example.com/m", "c0", "go test", map[string]string{"p/p.go": "x"}, recs)
	if plans, _ := bound.Narrow([]string{"LINECOV_TEST_G=1"}, []string{"p"}, heads, Effect{}, bound.Analyze(diff, read)); plans["p"].Mode != Whole {
		t.Fatalf("unbound in-order record did not keep the package: %+v", plans)
	}
	sel = bound.Plan(PlanInput{Base: "c0", Head: "c1", Change: diff, ReadDrift: read, ReadChange: read, Units: []Unit{{ID: "go:example.com/m/p", Dir: "p", Witness: "p/p.go"}},
		Heads: func([]string) (map[string][]string, error) { return heads, nil }, Environ: []string{"LINECOV_TEST_G=1"}})
	if sel.State != StateNarrowed || len(sel.Units) != 1 || sel.Units[0].Mode != Whole || sel.Units[0].Category != CategoryEnvironment {
		t.Fatalf("plan %s %+v", sel.State, sel.Units)
	}
}

// GLS-V0-009: a runtime setting the Go runtime reads itself is part of the
// identity, so an index collected under GOMAXPROCS=2 decides nothing for a
// runner without it.
func TestRunnerIDBindsRuntimeSettings(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	id := func(env ...string) string {
		t.Helper()
		base := []string{}
		for _, kv := range os.Environ() {
			if name, _, _ := strings.Cut(kv, "="); name != "GOMAXPROCS" && name != "GOGC" {
				base = append(base, kv)
			}
		}
		got, _, err := RunnerID(ctx, t.TempDir(), append(base, env...), DefaultTestTimeout, "")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	plain, procs, gc := id(), id("GOMAXPROCS=2"), id("GOGC=50")
	if plain == procs || plain == gc || procs == gc || !strings.Contains(procs, `runtime="GOMAXPROCS=2"`) || !strings.Contains(plain, `runtime=""`) {
		t.Fatalf("identities %q %q %q", plain, procs, gc)
	}
	if again := id("GOMAXPROCS=2"); again != procs {
		t.Fatalf("one setting, two identities %q and %q", procs, again)
	}
}

// GLS-V0-009: the go test -exec wrapper is part of the identity by content,
// so an index collected under a wrapper decides nothing for a run without
// it or under another build of it; an unreadable wrapper is an error.
func TestRunnerIDBindsExecWrapper(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.sh"), filepath.Join(dir, "b.sh")
	for p, body := range map[string]string{a: "#!/bin/sh\nexec \"$@\"\n", b: "#!/bin/sh\n# other\nexec \"$@\"\n"} {
		if err := execfile.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	plain, _, err := RunnerID(ctx, dir, nil, DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	withA, _, err := RunnerID(ctx, dir, nil, DefaultTestTimeout, a)
	if err != nil {
		t.Fatal(err)
	}
	withB, _, err := RunnerID(ctx, dir, nil, DefaultTestTimeout, b)
	if err != nil {
		t.Fatal(err)
	}
	if plain == withA || withA == withB || !strings.Contains(plain, `exec=""`) || !strings.Contains(withA, `exec="sha256:`) {
		t.Fatalf("identities %q %q %q", plain, withA, withB)
	}
	if again, _, err := RunnerID(ctx, dir, nil, DefaultTestTimeout, a); err != nil || again != withA {
		t.Fatalf("one wrapper, two identities %q and %q (%v)", withA, again, err)
	}
	if _, _, err := RunnerID(ctx, dir, nil, DefaultTestTimeout, filepath.Join(dir, "absent")); err == nil {
		t.Fatal("an unreadable wrapper was read")
	}
}

// GLS-V0-008, GLS-V0-009: collection runs every test process through the
// -exec wrapper, as `WRAPPER BINARY ARGS...` from the package directory, so
// the records show what the tests do under the wrapper the runs use.
func TestCollectUnderExecWrapper(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": fixture["go.mod"], "calc/calc.go": fixture["calc/calc.go"], "calc/calc_test.go": fixture["calc/calc_test.go"]})
	git(t, root, "init", "-q")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	marks := filepath.Join(t.TempDir(), "marks")
	wrapper := filepath.Join(t.TempDir(), "wrap.sh")
	if err := execfile.WriteFile(wrapper, []byte("#!/bin/sh\nprintf '%s %s\\n' \"$(pwd)\" \"$1\" >> \"$LINECOV_TEST_MARKS\"\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINECOV_TEST_MARKS", marks)
	out, err := Collect(context.Background(), root, CollectOptions{Module: "example.com/m", Packages: []string{"calc"}, Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute, TestTimeout: DefaultTestTimeout, Exec: wrapper})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range out {
		if o.Record == nil {
			t.Fatalf("%s.%s under the wrapper: %s", o.Package, o.Name, o.Divergent)
		}
	}
	data, err := os.ReadFile(marks)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	real, _ := filepath.EvalSymlinks(root)
	if len(lines) != len(out) || !strings.HasPrefix(lines[0], filepath.Join(real, "calc")+" ") || !strings.Contains(lines[0], ".test") {
		t.Fatalf("wrapper saw %d runs for %d outcomes: %q", len(lines), len(out), lines)
	}
}
