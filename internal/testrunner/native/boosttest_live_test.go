//go:build darwin || linux

package native

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TestBoostTestLiveExecution traces TRE-V0-036 and TRE-V0-039. It compiles the
// pinned fixture against an operator-supplied, checksum-verified Boost 1.92.0
// source tree and runs it through the common executor. It is opt-in because no
// Boost installation is a repository dependency.
func TestBoostTestLiveExecution(t *testing.T) {
	boostRoot := os.Getenv("CORVINT_BOOST_ROOT")
	if boostRoot == "" {
		t.Skip("live Boost.Test qualification is opt-in (CORVINT_BOOST_ROOT)")
	}
	version, err := os.ReadFile(filepath.Join(boostRoot, "boost", "version.hpp"))
	if err != nil || !bytes.Contains(version, []byte("#define BOOST_VERSION 109200\n")) {
		t.Fatalf("CORVINT_BOOST_ROOT is not Boost 1.92.0: %v", err)
	}
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Fatal("C++ compiler unavailable", err)
	}
	p, _ := boostRecorded(t)
	source, err := base64.StdEncoding.DecodeString(p.FixtureSource)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err = os.WriteFile(filepath.Join(root, "probe.cpp"), source, 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	run := func(name string, mode int, expected, selectors []string) (tr.Observation, tr.Input) {
		t.Helper()
		exe := filepath.Join(bin, "probe"+strconv.Itoa(mode))
		if _, statErr := os.Stat(exe); statErr != nil {
			cmd := exec.Command(compiler, "-std=c++17", "-O0", "-DPROOF_MODE="+strconv.Itoa(mode), "-I", boostRoot, "probe.cpp", "-o", exe)
			cmd.Dir = root
			if out, cerr := cmd.CombinedOutput(); cerr != nil {
				t.Fatalf("compile mode %d: %v\n%s", mode, cerr, out)
			}
		}
		exeBytes, rerr := os.ReadFile(exe)
		if rerr != nil {
			t.Fatal(rerr)
		}
		r := tr.Request{Runner: boostRunner, Executable: exe, ExecutableSha256: tr.Digest(exeBytes), Root: root, ReportDir: filepath.Join(t.TempDir(), "reports"), TimeoutSeconds: 120, InputFiles: map[string]string{"probe.cpp": tr.Digest(source)}, Target: boostTarget, ExpectedTests: expected, Selectors: selectors}
		inv, berr := Build(r)
		if berr != nil {
			t.Fatalf("%s build: %v", name, berr)
		}
		result, xerr := tr.Execute(context.Background(), r, inv)
		if xerr != nil {
			t.Fatalf("%s execution: %v; input=%+v", name, xerr, result.Input)
		}
		o, perr := Parse(result.Input)
		if perr != nil {
			t.Logf("%s parse refusal: %v", name, perr)
		}
		return tr.Normalize(result.Input, o), result.Input
	}
	o, in := run("pass-fail-disabled", 0, boostInventoryFor(0), nil)
	states := boostStates(o)
	if !o.Complete || in.ExitCode != 201 || len(o.Tests) != 4 || states[boostID("math/passes")] != tr.Passed || states[boostID("math/fails")] != tr.Failed || states[boostID("math/disabled_case")] != tr.Skipped || states[boostID("top_level")] != tr.Passed {
		t.Fatalf("pass-fail-disabled: exit=%d %+v", in.ExitCode, o)
	}
	o, in = run("selected-pass", 0, boostInventoryFor(0), []string{boostID("math/passes"), boostID("top_level")})
	states = boostStates(o)
	if !o.Complete || in.ExitCode != 0 || states[boostID("math/passes")] != tr.Passed || states[boostID("top_level")] != tr.Passed || states[boostID("math/fails")] != tr.Skipped {
		t.Fatalf("selected-pass: exit=%d %+v", in.ExitCode, o)
	}
	for name, c := range map[string]struct {
		mode      int
		expected  []string
		selectors []string
	}{
		"missing-expected":      {1, append(boostInventoryFor(1), boostID("math/fails")), nil},
		"suite-fixture-failure": {2, boostInventoryFor(2), nil},
		"uncaught-exception":    {3, boostInventoryFor(3), nil},
		"selected-no-match":     {0, append(boostInventoryFor(0), boostID("math/nomatch")), []string{boostID("math/nomatch")}},
	} {
		o, in = run(name, c.mode, c.expected, c.selectors)
		if o.Complete {
			t.Fatalf("%s admitted: exit=%d %+v", name, in.ExitCode, o)
		}
		for _, v := range o.Tests {
			if v.State != tr.Unknown {
				t.Fatalf("%s exposed %s for %s", name, v.State, v.ID)
			}
		}
		t.Logf("%s: exit=%d problems=%d", name, in.ExitCode, len(o.Problems))
	}
}
