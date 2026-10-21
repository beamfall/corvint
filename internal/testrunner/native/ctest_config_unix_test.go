//go:build darwin || linux

package native

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TRE-V0-113: the run-owned test directory keeps the build tree's top-level
// CTest configuration. A failing CTestCustom pre-test hook still fails the
// run, and the DartConfiguration.tcl default TimeOut still times a test out,
// while CTest's bookkeeping stays out of the build tree.
func TestCTestRunKeepsBuildTreeConfiguration(t *testing.T) {
	executable, err := exec.LookPath("ctest")
	if err != nil {
		t.Skip("ctest unavailable")
	}
	if executable, err = filepath.EvalSymlinks(executable); err != nil {
		t.Fatal(err)
	}
	toolBytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, files map[string]string, expected []string) (tr.Observation, error) {
		t.Helper()
		build, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		inputs := map[string]string{}
		for name, body := range files {
			body = strings.ReplaceAll(body, "@BUILD@", build)
			if err := os.WriteFile(filepath.Join(build, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			inputs[name] = tr.Digest([]byte(body))
		}
		reports := filepath.Join(t.TempDir(), "reports")
		r := tr.Request{Runner: "ctest", Executable: executable, ExecutableSha256: tr.Digest(toolBytes), Root: build, ReportDir: reports, TimeoutSeconds: 120, InputFiles: inputs, ExpectedTests: expected}
		inv, err := Build(r)
		if err != nil {
			t.Fatal(err)
		}
		result, err := tr.Execute(context.Background(), r, inv)
		if _, statErr := os.Stat(filepath.Join(build, "Testing")); statErr == nil {
			t.Fatalf("ctest wrote its bookkeeping into the build tree %s", build)
		}
		if err != nil {
			return tr.Observation{}, err
		}
		observation, err := Parse(result.Input)
		if err == nil {
			observation = tr.Normalize(result.Input, observation)
		}
		return observation, err
	}
	t.Run("pre-test hook failure", func(t *testing.T) {
		observation, err := run(t, map[string]string{
			"CTestTestfile.cmake": "add_test(pass \"/usr/bin/true\")\n",
			"CTestCustom.cmake":   "set(CTEST_CUSTOM_PRE_TEST \"/bin/sh -c \\\"exit 3\\\"\")\n",
		}, []string{"pass"})
		if err == nil && observation.Complete {
			t.Fatalf("a failing CTestCustom pre-test hook produced a complete run: %+v", observation)
		}
	})
	t.Run("DartConfiguration TimeOut", func(t *testing.T) {
		observation, err := run(t, map[string]string{
			"CTestTestfile.cmake":   "add_test(slow \"/bin/sleep\" \"5\")\n",
			"DartConfiguration.tcl": "SourceDirectory: @BUILD@\nBuildDirectory: @BUILD@\nTimeOut: 1\n",
		}, []string{"slow"})
		if err != nil || len(observation.Tests) != 1 || observation.Tests[0].State != tr.Failed {
			t.Fatalf("DartConfiguration TimeOut not applied: err=%v observation=%+v", err, observation)
		}
	})
}
