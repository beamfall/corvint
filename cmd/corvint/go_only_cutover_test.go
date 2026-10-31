package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"

	"github.com/Beamfall/corvint/internal/versionline"
)

func TestGoOnlySourceAndVersion(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	for _, pattern := range []string{"src/corvint_cli.py", "src/context_corvint*.py", "pyproject.toml"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil || len(matches) != 0 {
			t.Fatalf("GOC-V0-001 retired path %s: %v %v", pattern, matches, err)
		}
	}
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil || strings.TrimSpace(string(version)) == "" {
		t.Fatalf("GOC-V0-001 VERSION: %q %v", version, err)
	}
	output, err := candidateCommand("--version").CombinedOutput()
	// PUB-V0-053: an unstamped source build omits the build member and names a development version.
	line, parsed := versionline.Parse(string(output))
	if err != nil || !parsed || line.Name != "Corvint" || line.Version != versionline.DevelopmentVersion(strings.TrimSpace(string(version))) || line.Build != "" || line.Commit == "" {
		t.Fatalf("GOC-V0-001 native version: %q %v", output, err)
	}
	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("GOC-V0-001 CI workflows: %v %v", workflows, err)
	}
	for _, workflow := range workflows {
		body, err := os.ReadFile(workflow)
		if err != nil {
			t.Fatalf("GOC-V0-001 CI workflow %s: %v", workflow, err)
		}
		lower := strings.ToLower(string(body))
		for _, needle := range []string{"wheel", "pypi", "twine", "bdist_wheel", "cibuildwheel", "setup.py"} {
			if strings.Contains(lower, needle) {
				t.Fatalf("GOC-V0-001 CI workflow %s retains a wheel release job (%q)", workflow, needle)
			}
		}
	}
}

func TestGoOnlyContextAbstentionRemainsClosed(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("dogfood coordinator is a POSIX shell contract")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// A hang detector matching go test -timeout, not a budget (decision 0082): the coordinator
	// measured 53 s at load 33, so the former 60 s limit killed it on a loaded host.
	argv := []string{bash, "script/dogfood-change_test.sh"}
	result := procgroup.Run(ctx, procgroup.Spec{
		Argv: argv, Dir: moduleRoot(t),
		// DCW-V0-807 (V1-3383): a lane's exported closeout inputs must not reach the fixtures.
		Env: append(os.Environ(), "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=/nonexistent/cites.tsv",
			"DOGFOOD_INTENTS_FILE=/nonexistent/intents.txt", "CORVINT_BIN=/nonexistent/corvint"),
		Timeout: 30 * time.Minute, OutputLimit: 1 << 20,
	})
	if result.Err != nil || result.ExitStatus != 0 || !result.ExitObserved || !result.WaitCompleted ||
		!result.PipesDrained || !result.OwnedProcessGroupCleanup || result.TimedOut || result.Cancelled || result.OutputOverflow {
		t.Fatalf("GOC-V0-009 context abstention coordinator: %s", shellTestFailure(argv, result))
	}
}

// shellTestFailure names what stopped a shell test (DCW-V0-351, V1-2827): the
// failing case lines the script's ERR trap printed, or NOT_OBSERVED when it
// printed none, the command, the exit, and a bounded tail of each stream.
func shellTestFailure(argv []string, result procgroup.Observation) string {
	const maxCases, maxTailLines, maxTailBytes = 8, 40, 4096
	var cases []string
	for _, line := range strings.Split(string(result.Stderr), "\n") {
		if strings.Contains(line, "_test: FAIL ") && len(cases) < maxCases {
			cases = append(cases, line)
		}
	}
	if len(cases) == 0 {
		cases = []string{"failing case NOT_OBSERVED: the script printed no FAIL line"}
	}
	tail := func(stream []byte) string {
		lines := strings.Split(strings.TrimRight(string(stream), "\n"), "\n")
		if len(lines) > maxTailLines {
			lines = lines[len(lines)-maxTailLines:]
		}
		text := strings.Join(lines, "\n")
		if len(text) > maxTailBytes {
			text = "..." + text[len(text)-maxTailBytes:]
		}
		return text
	}
	return fmt.Sprintf("%s\ncommand: %s\nerr=%v exit=%d timedOut=%t cancelled=%t overflow=%t\nstderr tail (last %d lines, %d bytes max):\n%s\nstdout tail (last %d lines, %d bytes max):\n%s",
		strings.Join(cases, "\n"), strings.Join(argv, " "), result.Err, result.ExitStatus, result.TimedOut, result.Cancelled, result.OutputOverflow,
		maxTailLines, maxTailBytes, tail(result.Stderr), maxTailLines, maxTailBytes, tail(result.Stdout))
}

// DCW-V0-351 (V1-2827): a forced failing assertion in a copy of
// dogfood-change_test.sh reaches the wrapper's failure message as its case,
// line and command, not as a bare exit status.
func TestDCWV0351ShellTestFailureNamesTheFailingCase(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("dogfood coordinator is a POSIX shell contract")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(moduleRoot(t), "script", "dogfood-change_test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	anchor := "# The reviewer documentation must name the OCM map path"
	if !strings.Contains(string(source), anchor) {
		t.Fatalf("anchor %q not in dogfood-change_test.sh", anchor)
	}
	forced := strings.Replace(string(source), anchor, "# Forced failing case (V1-2827).\ntest forced = failure\n"+anchor, 1)
	dir := t.TempDir()
	script := filepath.Join(dir, "dogfood-change_test.sh")
	if err := os.WriteFile(script, []byte(forced), 0o600); err != nil {
		t.Fatal(err)
	}
	// The script sources its sibling rg fallback helper.
	helper, err := os.ReadFile(filepath.Join(moduleRoot(t), "script", "test-rg-fallback.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test-rg-fallback.sh"), helper, 0o600); err != nil {
		t.Fatal(err)
	}
	line := strings.Count(forced[:strings.Index(forced, "test forced = failure")], "\n") + 1
	argv := []string{bash, script}
	result := procgroup.Run(t.Context(), procgroup.Spec{Argv: argv, Dir: t.TempDir(), Env: os.Environ(), Timeout: time.Minute, OutputLimit: 1 << 16})
	if result.ExitStatus == 0 {
		t.Fatalf("the forced failure passed: %s", result.Stderr)
	}
	message := shellTestFailure(argv, result)
	for _, want := range []string{
		fmt.Sprintf("dogfood-change_test: FAIL line=%d case=Forced failing case (V1-2827). command=test forced = failure\n", line),
		"command: " + bash + " " + script + "\n",
		fmt.Sprintf("exit=%d ", result.ExitStatus),
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message lacks %q:\n%s", want, message)
		}
	}
	if got := shellTestFailure(argv, procgroup.Observation{ExitStatus: 1, Stderr: []byte("no trap\n")}); !strings.HasPrefix(got, "failing case NOT_OBSERVED") {
		t.Fatalf("no FAIL line: %q", got)
	}
}
