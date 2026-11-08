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
