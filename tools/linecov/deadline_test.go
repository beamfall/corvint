// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/linecov"
)

// GLS-V0-020: a watch run executes under the runtime the index's runner
// records, verbose and with its deadline, not the go test default.
func TestWatchRunsRecordedTestRuntime(t *testing.T) {
	runtime := strings.Join(linecov.TestRuntime(17*time.Second), " ")
	ix := &linecov.Index{Runner: "go1 test=" + strconv.Quote(runtime), TestTimeout: time.Minute}
	got := strings.Join(watchTestArgs(ix, Invocation{Package: "./p", Run: "^(TestA)$"}), " ")
	if want := "go test -count=1 -json -timeout=17s -run ^(TestA)$ ./p"; got != want {
		t.Fatalf("watch argv %q, want %q", got, want)
	}
}

// LPCV-V0-7603: qualification passes exactly the recorded deadline to mutants.
func TestQualificationDeadline(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	t.Setenv("DEADLINE_LOG", log)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DEADLINE_LOG\"\nprintf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"m/p\"}'\n"
	if err := execfile.WriteFile(filepath.Join(dir, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, _, err := failingTests(dir, "m", []string{"p"}, 17*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "-timeout=17s\n") {
		t.Fatalf("deadline lost: %s", b)
	}
	if _, _, err = failingTests(dir, "m", []string{"p"}, 0); err == nil {
		t.Fatal("unknown deadline admitted")
	}
}

// GLS-V0-031: a `--- FAIL:` line a test prints (another run's output) is a
// result go test frames for a test the package has not got; only declared
// tests fail, and a declared failure is never dropped.
func TestDeclaredFailuresDropPhantoms(t *testing.T) {
	heads := map[string][]string{"tools/linecov": {"TestReal"}, "p": {"TestP"}}
	got := declaredFailures(map[string]bool{"tools/linecov.TestReal": true, "tools/linecov.TestFail": true, "p.TestP": true, "q.TestQ": true}, heads)
	if len(got) != 2 || !got["tools/linecov.TestReal"] || !got["p.TestP"] {
		t.Fatalf("declared failures %v", got)
	}
}

// GLS-V0-054 (V1-4514 review of the qualify timeout): when a suite run's
// deadline expires, the test binary panics and go test -json reports the
// package failed with no result for the running test (the stream below is
// go1.27.3's for a test that sleeps past -timeout=2s). The harness has no
// declared failing test of the package, so it counts the package a crash,
// and qualify counts every test the package declares as failing: a timeout
// can add misses, never hide one. A package with no result is an error.
func TestQualifySuiteTimeoutCountsEveryTestFailing(t *testing.T) {
	dir := t.TempDir()
	events := `{"Action":"start","Package":"m/p"}
{"Action":"run","Package":"m/p","Test":"TestA"}
{"Action":"pass","Package":"m/p","Test":"TestA","Elapsed":0}
{"Action":"run","Package":"m/p","Test":"TestHang"}
{"Action":"output","Package":"m/p","Test":"TestHang","Output":"panic: test timed out after 2s\n"}
{"Action":"output","Package":"m/p","Test":"TestHang","Output":"\trunning tests:\n"}
{"Action":"output","Package":"m/p","Test":"TestHang","Output":"\t\tTestHang (2s)\n"}
{"Action":"output","Package":"m/p","Output":"FAIL\tm/p\t2.005s\n"}
{"Action":"fail","Package":"m/p","Elapsed":2.005}
`
	if err := os.WriteFile(filepath.Join(dir, "events"), []byte(events), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \" $* \" in *' ./p '*) cat \"$EVENTS\"; exit 1;; esac\nexit 1\n"
	if err := execfile.WriteFile(filepath.Join(dir, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVENTS", filepath.Join(dir, "events"))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	failed, b, err := failingTests(dir, "m", []string{"p"}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	heads := map[string][]string{"p": {"TestA", "TestHang", "TestZ"}}
	failed = declaredFailures(failed, heads)
	b.settle(failed, heads)
	if len(failed) != 0 || !b.crash["p"] || len(b.build) != 0 {
		t.Fatalf("timed-out package: failed %v crash %v build %v, want no test failure and a crash of p", failed, b.crash, b.build)
	}
	if _, _, err := failingTests(dir, "m", []string{"q"}, 2*time.Second); err == nil || !strings.Contains(err.Error(), "no result for q") {
		t.Fatalf("a package with no result: %v, want an error", err)
	}
}
