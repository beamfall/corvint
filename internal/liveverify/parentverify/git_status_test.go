//go:build darwin || linux

package parentverify

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParentObservationCannotExecuteConfiguredFilter(t *testing.T) {
	t.Run("EAF-V0-007", func(t *testing.T) {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		git = resolved(t, git)
		root := resolved(t, t.TempDir())
		fixtureGit := func(args ...string) {
			t.Helper()
			cmd := exec.Command(git, append([]string{"-C", root}, args...)...)
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git fixture: %v %s", err, data)
			}
		}
		fixtureGit("init", "-q")
		source := filepath.Join(root, "value.go")
		if err := os.WriteFile(source, []byte("package value\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go filter=hostile\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		fixtureGit("add", ".")
		fixtureGit("-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "fixture")
		marker := filepath.Join(t.TempDir(), "filter-executed")
		fixtureGit("config", "filter.hostile.clean", "touch "+marker)
		if err := os.WriteFile(source, []byte("package other\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Unix(1700000000, 0)
		if err := os.Chtimes(source, old, old); err != nil {
			t.Fatal(err)
		}
		digest, _, err := stableExecutableFile(git)
		if err != nil {
			t.Fatal(err)
		}
		config := Config{GitExecutable: git, GitExecutableSHA256: digest, RepositoryRoot: root,
			TemporaryParent: resolved(t, t.TempDir()), Timeout: time.Second * 10, OutputLimitBytes: 8 << 20}
		run := func(ctx context.Context, executable string, argv, environment []string, cwd string, _ time.Duration) (CommandResult, error) {
			cmd := exec.CommandContext(ctx, executable, argv...)
			cmd.Dir, cmd.Env = cwd, environment
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			return CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Started: err == nil,
				Exited: err == nil, ExitCode: 0, ProcessCleanupDone: true, PipesDrained: true, WaitCompleted: true}, err
		}
		_, _, err = observeRepository(context.Background(), config, run)
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf("parent observation executed configured filter: %v", statErr)
		}
		if !errors.Is(err, ErrCommand) {
			t.Fatalf("unsafe parent observation error=%v", err)
		}
	})
}
