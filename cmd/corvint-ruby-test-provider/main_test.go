package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const cannedReport = `{"version":"3.13.2","examples":[{"id":"./spec/user_spec.rb[1:1]","full_description":"User saves","status":"passed","file_path":"./spec/user_spec.rb","line_number":2}],"summary":{"example_count":1,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}}`

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			environment = append(environment, entry)
		}
	}
	command.Env = append(environment, "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// LPCV-V0-055, LPCV-V0-090, LPCV-V0-093..095: a retained run writes the
// document under .corvint/test-evidence, leaves git status empty, and
// test-validity discovery projects association, execution and CURRENT
// freshness from it.
func TestLPCVV0095RetainedRunLeavesWorktreeCleanAndDiscovers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake needs a POSIX shell")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.email", "test@example.invalid")
	git(t, root, "config", "user.name", "test")
	write(t, filepath.Join(root, "spec", "spec_helper.rb"), "RSpec.configure { }\n", 0o644)
	write(t, filepath.Join(root, "spec", "user_spec.rb"), "RSpec.describe User do\n  it 'saves'\nend\n", 0o644)
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "fixture")
	bin := t.TempDir()
	write(t, filepath.Join(bin, "report.json"), cannedReport, 0o644)
	rspec := filepath.Join(bin, "rspec")
	write(t, rspec, "#!/bin/sh\nformat=\nprevious=\nfor argument in \"$@\"; do\n  if [ \"$previous\" = --format ]; then format=\"$argument\"; fi\n  if [ \"$previous\" = --out ] && [ \"$format\" = json ]; then cat '"+filepath.Join(bin, "report.json")+"' > \"$argument\"; fi\n  previous=\"$argument\"\ndone\nexit 0\n", 0o755)

	for attempt := 0; attempt < 2; attempt++ {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{"rspec", "--retain", "--dir", root, "--rspec", rspec, "--timeout", "1m"}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d stderr=%s", code, stderr.String())
		}
		var document rubytestprovider.Document
		if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
			t.Fatal(err)
		}
		if document.RepositoryBefore == nil || document.RepositoryBefore.DirtyState != "clean" {
			t.Fatalf("attempt %d saw a dirty worktree: %+v", attempt, document.RepositoryBefore)
		}
		if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
			t.Fatalf("retained run left the worktree dirty: %q", status)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, ".corvint", ".gitignore")); err != nil || string(data) != "*\n" {
		t.Fatalf(".gitignore=%q %v", data, err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".corvint", "test-evidence", "corvint-ruby-test-provider-*.json"))
	if err != nil || len(matches) != 2 {
		t.Fatalf("retained=%v %v", matches, err)
	}
	discovered, err := testvaliditydoc.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if discovered.Source != rubytestprovider.Producer || len(discovered.Tests) != 1 || discovered.Discovery.Abstention != nil {
		t.Fatalf("discovered=%+v", discovered)
	}
	projection := discovered.Tests[0].Projection
	if discovered.Tests[0].ID != "spec/user_spec.rb:2" || projection.Association.State != testvalidity.AssociationAssociated || projection.Execution.State != testvalidity.ExecutionPassed || projection.Freshness.State != testvalidity.FreshnessCurrent {
		t.Fatalf("projection=%+v", projection)
	}
}

// LPCV-V0-090: refusals exit 2 and print no document.
func TestLPCVV0090RefusalsExitTwo(t *testing.T) {
	outside := t.TempDir()
	for _, args := range [][]string{{}, {"unknown"}, {"rspec", "--dir", outside, "--rspec", "true"}, {"rspec", "extra"}, {"rspec", "--dir", outside, "--spec", "../x_spec.rb"}, {"rspec", "--formatter", "progress"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("%v: exit %d stdout=%q", args, code, stdout.String())
		}
	}
	var stdout bytes.Buffer
	if code := run(context.Background(), []string{"--help"}, &stdout, &stdout); code != 0 || !strings.Contains(stdout.String(), "-exec-prefix") || !strings.Contains(stdout.String(), rubytestprovider.FormatterCorvint) {
		t.Fatalf("help exit %d: %s", code, stdout.String())
	}
}
