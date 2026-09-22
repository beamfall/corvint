//go:build darwin || linux

package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func qualifiedGitTestEnvironment(t *testing.T) []string {
	t.Helper()
	return []string{
		"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(),
	}
}

// WQO-V0-005 / VPO-V0-010: every opening, blob and closing Git read uses
// the supplied execution identity, including concurrent repository observations.
func TestBuildWithGitExecutionIgnoresAmbientAndPreservesIndex(t *testing.T) {
	root := testRepository(t)
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
	log := filepath.Join(directory, "calls")
	wrapper := filepath.Join(directory, "qualified-git")
	body := fmt.Sprintf("#!/bin/sh\n[ -z \"${GIT_DIR+x}${GIT_INDEX_FILE+x}${GIT_CONFIG_COUNT+x}${CORVINT_POISON+x}\" ] || exit 91\nprintf '%%s\\n' \"$*\" >> %s\nexec %s \"$@\"\n", strconv.Quote(log), strconv.Quote(realGit))
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte("#!/bin/sh\nexit 92\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	environment := qualifiedGitTestEnvironment(t)
	for name, value := range map[string]string{
		"PATH": directory, "TMPDIR": "/nonexistent-corvint-poison", "HOME": "/nonexistent-corvint-poison",
		"GIT_DIR": "/nonexistent-corvint-poison", "GIT_INDEX_FILE": "/nonexistent-corvint-poison",
		"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.bare", "GIT_CONFIG_VALUE_0": "true", "CORVINT_POISON": "1",
	} {
		t.Setenv(name, value)
	}
	qualified, err := BuildWithGitExecution(context.Background(), root, wrapper, environment)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ordinary, qualified) {
		t.Fatal("qualified build changed ordinary index identity or evidence")
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"rev-parse", "status", "ls-tree", "cat-file"} {
		if !strings.Contains(string(raw), operation) {
			t.Errorf("qualified Git did not receive %s: %s", operation, raw)
		}
	}
}

func TestBuildWithGitExecutionRejectsAmbientFallback(t *testing.T) {
	for _, input := range []struct {
		executable string
		env        []string
	}{{"git", []string{}}, {"/usr/bin/git", nil}, {"/no-such-corvint-git", []string{}}} {
		index, err := BuildWithGitExecution(context.Background(), t.TempDir(), input.executable, input.env)
		if index != nil || err == nil {
			t.Fatalf("unqualified input admitted: %#v", input)
		}
	}
}

func TestBuildWithGitExecutionOwnsEnvironment(t *testing.T) {
	root := testRepository(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	wrapper := filepath.Join(directory, "git")
	body := fmt.Sprintf("#!/bin/sh\n[ \"$LANG\" = C ] || { printf 'environment changed' >&2; exit 94; }\nexec %s \"$@\"\n", strconv.Quote(realGit))
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	environment := qualifiedGitTestEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = withGitExecution(ctx, wrapper, environment)
	// Mutation follows the synchronous copy; all Git calls must use that copy.
	environment[1] = "LANG=poison"
	if _, err := Build(ctx, root); err != nil {
		t.Fatal(err)
	}
}

// WQO-V0-042: even an unchanged worktree cannot replace immutable blob
// acquisition. A failed cat-file must fail the closure rather than read disk.
func TestBuildWithGitExecutionRequiresImmutableObjects(t *testing.T) {
	t.Run("WQO-V0-042", func(t *testing.T) {
		root := testRepository(t)
		ordinary, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		environment := qualifiedGitTestEnvironment(t)
		qualified, err := BuildWithGitExecution(context.Background(), root, realGit, environment)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ordinary, qualified) {
			t.Fatal("immutable blob acquisition changed the clean index")
		}
		wrapper := filepath.Join(t.TempDir(), "git")
		body := fmt.Sprintf("#!/bin/sh\nfor arg do\n[ \"$arg\" != cat-file ] || { printf 'immutable object unavailable' >&2; exit 93; }\ndone\nexec %s \"$@\"\n", strconv.Quote(realGit))
		if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
		qualified, err = BuildWithGitExecution(context.Background(), root, wrapper, environment)
		if qualified != nil || err == nil || !strings.Contains(err.Error(), "immutable object unavailable") {
			t.Fatalf("immutable acquisition fell back to worktree: index=%v error=%v", qualified != nil, err)
		}
	})
}

// VPO-V0-010: a retained identity prefix cannot hide an over-limit stream.
func TestBuildWithGitExecutionRejectsTruncatedOutput(t *testing.T) {
	wrapper := filepath.Join(t.TempDir(), "git")
	body := "#!/bin/sh\nprintf '%s' '" + strings.Repeat("a", maxIdentityBytes+1) + "'\n"
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	index, err := BuildWithGitExecution(context.Background(), t.TempDir(), wrapper, qualifiedGitTestEnvironment(t))
	if index != nil || err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("truncated output admitted: index=%v error=%v", index != nil, err)
	}
}

// WQO-V0-005 / VPO-V0-010: pipe ownership is bounded even after a successful
// leader exit; cancellation kills the owned descendants. These children remain
// in their Git process group, so this does not qualify escaped-child containment.
func TestBuildWithGitExecutionOwnsPipesAndCancellation(t *testing.T) {
	for _, cancelBuild := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-%t", cancelBuild), func(t *testing.T) {
			root := testRepository(t)
			directory := t.TempDir()
			wrapper := filepath.Join(directory, "git")
			ending := "exit 0"
			if cancelBuild {
				ending = "wait \"$child\""
			}
			body := fmt.Sprintf("#!/bin/sh\n/bin/sleep 10 &\nchild=$!\nprintf '%%s\\n' \"$child\" > %s/child-$$\n%s\n", strconv.Quote(directory), ending)
			if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
				t.Fatal(err)
			}
			// Register bounded fallback cleanup before either Git invocation starts.
			t.Cleanup(func() {
				for _, pid := range qualifiedGitChildPIDs(t, directory) {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			environment := qualifiedGitTestEnvironment(t)
			completed := make(chan error, 1)
			started := time.Now()
			go func() {
				_, err := BuildWithGitExecution(ctx, root, wrapper, environment)
				completed <- err
			}()
			deadline := time.Now().Add(3 * time.Second)
			for len(qualifiedGitChildPIDs(t, directory)) < 2 && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			pids := qualifiedGitChildPIDs(t, directory)
			if len(pids) != 2 {
				t.Fatalf("expected two registered Git descendants, got %v", pids)
			}
			if cancelBuild {
				cancel()
			}
			select {
			case err := <-completed:
				if err == nil {
					t.Fatal("incomplete Git output produced successful acquisition")
				}
				if !cancelBuild && !strings.Contains(err.Error(), "WaitDelay") {
					t.Fatalf("expected incomplete pipe capture, got %v", err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("qualified Git acquisition did not return within its bound")
			}
			if time.Since(started) > 4*time.Second {
				t.Fatal("qualified Git acquisition exceeded lifecycle bound")
			}
			for _, pid := range pids {
				deadline = time.Now().Add(3 * time.Second)
				for syscall.Kill(pid, 0) != syscall.ESRCH && time.Now().Before(deadline) {
					time.Sleep(10 * time.Millisecond)
				}
				if syscall.Kill(pid, 0) != syscall.ESRCH {
					t.Errorf("owned Git descendant %d survived", pid)
				}
			}
		})
	}
}

func qualifiedGitChildPIDs(t *testing.T, directory string) []int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(directory, "child-*"))
	if err != nil {
		t.Fatal(err)
	}
	var pids []int
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}
