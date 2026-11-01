//go:build darwin || linux

package contextindex

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/execfile"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// DCW-V0-075 (V1-1652): the per-phase Git deadline is 30 seconds and has no
// override.
func TestDCWV0075GitDeadlineIsThirtySeconds(t *testing.T) {
	if gitDeadline != 30*time.Second {
		t.Fatalf("gitDeadline = %v, want 30s (DCW-V0-075 documents this value)", gitDeadline)
	}
}

// DCW-V0-075 (V1-1652): a Git slower than its deadline, as on a loaded host,
// refuses with the stable uncoded deadline error that dogfood change retries
// once and then admits, and the build returns within a bound.
func TestDCWV0075SlowGitOverrunsTheDeadline(t *testing.T) {
	root := testRepository(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if realGit, err = filepath.Abs(realGit); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "git")
	body := "#!/bin/sh\n/bin/sleep 2\nexec " + strconv.Quote(realGit) + " \"$@\"\n"
	if err := execfile.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	// A caller deadline shorter than gitDeadline ends the same per-phase
	// context, so the overrun is reproduced without shortening the constant.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = BuildWithGitExecution(ctx, root, wrapper, qualifiedGitTestEnvironment(t))
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "" || failure.Message != "Git repository index exceeded its deadline" || !errors.Is(failure.Cause, context.DeadlineExceeded) {
		t.Fatalf("slow Git = %#v, want the uncoded deadline refusal", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("slow Git build returned after %v, want a bounded overrun", elapsed)
	}
}

// IDX-SNAP-V0-912: only a context the refresh writer marked drops the shared
// 30-second deadline, keeping its own; an unmarked context keeps 30 seconds
// even under a longer parent deadline.
func TestWriterBoundsReplaceTheGitDeadline_IDXSNAPV0912(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelParent()
	ordinary, cancel := withGitDeadline(parent)
	defer cancel()
	if deadline, ok := ordinary.Deadline(); !ok || time.Until(deadline) > gitDeadline {
		t.Fatalf("unmarked deadline %v (%t), want at most %s", deadline, ok, gitDeadline)
	}
	writer, cancelWriter := withGitDeadline(WithWriterBounds(parent))
	defer cancelWriter()
	parentDeadline, _ := parent.Deadline()
	if deadline, ok := writer.Deadline(); !ok || !deadline.Equal(parentDeadline) {
		t.Fatalf("writer deadline %v (%t), want the parent's %v", deadline, ok, parentDeadline)
	}
	cancelParent()
	<-writer.Done()
}

// DRC-V0-910 (V1-2585): a Git that cannot start keeps the stable public
// message and names its closed start class: ENOENT for a missing executable,
// ETXTBSY for one still open for writing (the parallel-test race of
// V1-2585), and nothing for an ordinary Git failure.
func TestGitStartErrorClass_DRCV0910(t *testing.T) {
	root := testRepository(t)
	busy := filepath.Join(t.TempDir(), "git")
	writer, err := os.OpenFile(busy, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.WriteString("#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatal(err)
	}
	failing := filepath.Join(t.TempDir(), "git")
	if err := execfile.WriteFile(failing, []byte("#!/bin/sh\necho nope >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, executable, class string }{
		{"ENOENT", filepath.Join(t.TempDir(), "missing-git"), "ENOENT"},
		{"ETXTBSY", busy, "ETXTBSY"},
		{"exit", failing, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.class == "ETXTBSY" && runtime.GOOS != "linux" {
				// Darwin executes a script that is still open for writing; the race is Linux's.
				t.Skip("ETXTBSY on exec of a write-open file is Linux behavior")
			}
			ctx := withGitExecution(context.Background(), test.executable, os.Environ())
			_, err := git(ctx, root, 1<<20, nil, "rev-parse", "HEAD")
			if err == nil {
				t.Fatal("Git started")
			}
			if got := GitStartErrorClass(err); got != test.class {
				t.Fatalf("class = %q, want %q (err %v)", got, test.class, err)
			}
			if test.class != "" && err.Error() != "Git error: cannot start Git" {
				t.Fatalf("public message changed: %q", err.Error())
			}
		})
	}
	if GitStartErrorClass(errors.New("unrelated")) != "" {
		t.Fatal("a non-Git error has a start class")
	}
}

// DRC-V0-910: the classification itself, on every platform, for start errors
// wrapped as os/exec returns them.
func TestGitStartErrorClassMapsEachErrno_DRCV0910(t *testing.T) {
	for start, want := range map[error]string{
		&fs.PathError{Op: "fork/exec", Path: "/x/git", Err: syscall.ETXTBSY}: "ETXTBSY",
		&fs.PathError{Op: "fork/exec", Path: "/x/git", Err: syscall.ENOENT}:  "ENOENT",
		&exec.Error{Name: "git", Err: exec.ErrNotFound}:                      "ENOENT",
		&fs.PathError{Op: "fork/exec", Path: "/x/git", Err: syscall.EACCES}:  "EACCES",
		&fs.PathError{Op: "fork/exec", Path: "/x/git", Err: syscall.EPERM}:   "EPERM",
		&fs.PathError{Op: "fork/exec", Path: "/x/git", Err: syscall.ENOEXEC}: "ENOEXEC",
		errors.New("something else"):                                         "other",
	} {
		err := &Error{Message: "Git error: cannot start Git", gitFailure: &GitFailure{ExitCode: -1, StartError: start}}
		if got := GitStartErrorClass(err); got != want {
			t.Errorf("%v: class %q, want %q", start, got, want)
		}
	}
	if got := GitStartErrorClass(&Error{Message: "Git error: x", gitFailure: &GitFailure{ExitCode: 3}}); got != "" {
		t.Errorf("exit failure class %q", got)
	}
}
