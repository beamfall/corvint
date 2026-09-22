package main

import (
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
	if err != nil || strings.TrimSpace(string(output)) != "Corvint "+strings.TrimSpace(string(version))+" (build 0)" {
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
	result := procgroup.Run(ctx, procgroup.Spec{
		Argv: []string{bash, "script/dogfood-change_test.sh"}, Dir: moduleRoot(t),
		Env: os.Environ(), Timeout: 30 * time.Minute, OutputLimit: 1 << 20,
	})
	if result.Err != nil || result.ExitStatus != 0 || !result.ExitObserved || !result.WaitCompleted ||
		!result.PipesDrained || !result.OwnedProcessGroupCleanup || result.TimedOut || result.Cancelled || result.OutputOverflow {
		t.Fatalf("GOC-V0-009 context abstention coordinator: %v exit=%d\n%s\n%s", result.Err, result.ExitStatus, result.Stdout, result.Stderr)
	}
}
