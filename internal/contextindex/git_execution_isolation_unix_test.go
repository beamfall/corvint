//go:build darwin || linux

package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

func TestBuildWithGitExecutionRoutesIsolatedStatusThroughQualifiedRawGit(t *testing.T) {
	t.Run("MCPV0-017", func(t *testing.T) {
		root := testRepository(t)
		cleanRoot := testRepository(t)
		// Exercise private Git validation as well as qualified status execution;
		// the default core/user fixture is accepted by the strict fast recognizer.
		testGit(t, root, "config", "status.renames", "true")
		canonicalRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "internal/token/token.go", "package token\n// dirty bytes are not evidence\n")
		ordinary, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		realGit, err = filepath.Abs(realGit)
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		logPath := filepath.Join(directory, "calls")
		wrapper := filepath.Join(directory, "qualified-git")
		quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
		body := fmt.Sprintf(`#!/bin/sh
[ "${CORVINT_QUALIFIED_WITNESS:-}" = 1 ] || exit 91
[ -z "${GIT_DIR+x}${GIT_INDEX_FILE+x}${GIT_CONFIG_COUNT+x}${CORVINT_POISON+x}" ] || exit 92
[ -n "${GIT_CEILING_DIRECTORIES:-}" ] || exit 93
printf '%%s\t%%s\n' "$GIT_CEILING_DIRECTORIES" "$*" >> %s
if [ "${CORVINT_DENY_BLOBS:-}" = 1 ]; then
  for arg do
    if [ "$arg" = cat-file ]; then
      printf 'immutable object unavailable' >&2
      exit 94
    fi
  done
fi
exec %s "$@"
`, quote(logPath), quote(realGit))
		if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "git"), []byte("#!/bin/sh\nexit 95\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		environment := append(qualifiedGitTestEnvironment(t), "CORVINT_QUALIFIED_WITNESS=1")
		scratch := t.TempDir()
		environment = append(environment, "TMPDIR="+scratch)
		before := append([]string(nil), environment...)
		canonicalScratch, err := filepath.EvalSymlinks(scratch)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("TMPDIR", "/nonexistent-corvint-poison")
		for name, value := range map[string]string{
			"PATH": directory, "HOME": "/nonexistent-corvint-poison",
			"GIT_DIR": "/nonexistent-corvint-poison", "GIT_INDEX_FILE": "/nonexistent-corvint-poison",
			"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.bare", "GIT_CONFIG_VALUE_0": "true", "CORVINT_POISON": "1",
		} {
			t.Setenv(name, value)
		}
		ctx := gitstatus.WithIsolation(context.Background())
		qualified, err := BuildWithGitExecution(ctx, root, wrapper, environment)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ordinary, qualified) {
			t.Fatal("composed execution changed immutable evidence or repository identity")
		}
		if !reflect.DeepEqual(environment, before) {
			t.Fatal("private status changed the caller's closed environment")
		}
		raw, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, operation := range []string{" config ", " ls-files ", " rev-parse ", " status ", " ls-tree ", " cat-file "} {
			if !strings.Contains(string(raw), operation) {
				t.Errorf("qualified wrapper did not observe %q: %s", operation, raw)
			}
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, " status ") {
				parts := strings.SplitN(line, "--git-dir=", 2)
				if len(parts) != 2 || !strings.Contains(parts[1], " --work-tree="+canonicalRoot) {
					t.Fatalf("isolated status lost its explicit repository paths: %s", line)
				}
				private := strings.SplitN(parts[1], " --work-tree=", 2)[0]
				if filepath.Base(private) != "metadata" || !strings.HasPrefix(filepath.Base(filepath.Dir(private)), "corvint-git-status-") || filepath.Dir(filepath.Dir(private)) != canonicalScratch {
					t.Fatalf("isolated status escaped its private scratch metadata: %s", line)
				}
			}
			if strings.Contains(line, " config ") && (!strings.Contains(line, "--file ") || !strings.Contains(line, "--no-includes")) {
				t.Fatalf("configuration parsing escaped the private snapshot: %s", line)
			}
		}
		originalEager := buildEagerBlobs
		defer func() { buildEagerBlobs = originalEager }()
		for _, eager := range []bool{false, true} {
			buildEagerBlobs = eager
			denied, err := BuildWithGitExecution(ctx, cleanRoot, wrapper, append(append([]string(nil), environment...), "CORVINT_DENY_BLOBS=1"))
			if denied != nil || err == nil || !strings.Contains(err.Error(), "immutable object unavailable") {
				t.Fatalf("immutable object failure fell back with eager=%v: index=%v error=%v", eager, denied != nil, err)
			}
		}
		entries, err := os.ReadDir(scratch)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if runtime.GOOS == "darwin" && entry.Name() == "xcrun_db" {
				continue
			}
			t.Fatalf("private status metadata survived composed execution: entry=%s", entry.Name())
		}
	})
}
