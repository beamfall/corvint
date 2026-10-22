//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// sourceGitScript writes an executable stand-in for /usr/bin/git. A child
// process writes it, so this test process never holds a write descriptor a
// concurrent fork could inherit, which on Linux makes exec of the stand-in
// fail "text file busy" before any Git step runs (V1-2433).
func sourceGitScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "git")
	write := exec.Command("/bin/sh", "-c", `umask 077 && /bin/cat > "$1" && /bin/chmod 700 "$1"`, "sh", path)
	write.Stdin = strings.NewReader("#!/bin/sh\n" + body)
	if output, err := write.CombinedOutput(); err != nil {
		t.Fatalf("write git stand-in: %v %s", err, output)
	}
	return path
}

// ESV-V0-001: HEAD is verified again after extraction, so a HEAD that moves
// during the read refuses instead of emitting success.
func TestSourceViewReverifiesHeadBeforeSuccess(t *testing.T) {
	t.Parallel()
	root, commit, packet := sourceViewRepository(t)
	seen := filepath.Join(t.TempDir(), "seen")
	script := sourceGitScript(t, `for a in "$@"; do prev=$last; last=$a; done
if [ "$prev $last" = "rev-parse HEAD" ] && [ -e '`+seen+`' ]; then echo `+strings.Repeat("0", 40)+`; exit 0; fi
if [ "$prev $last" = "rev-parse HEAD" ]; then : > '`+seen+`'; fi
exec /usr/bin/git "$@"
`)
	ctx := liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, script))
	if got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3"); sourceViewCode(got) != "stale-commit" || got["view"] != nil {
		t.Fatalf("moved HEAD accepted: %v", got)
	}
}

// ESV-V0-003: cancelling a Git read sends TERM to the child's owned process
// group and reaps its descendants before the refusal is emitted; the refusal
// names the cancellation (ESV-V0-014). The test lifts the read deadline and the
// TERM grace, so host load can neither fire the deadline before the trap exists
// nor let SIGKILL beat the trap's marker; a stop that skips TERM still fails
// the marker check. Linux holds the exited leader unreaped during the grace, so
// signal 0 never reports the group gone and the whole grace elapses: keep it
// well under the elapsed bound (V1-2319).
func TestSourceViewCancellationTerminatesGitProcessGroup(t *testing.T) {
	t.Parallel()
	root, commit, packet := sourceViewRepository(t)
	directory := t.TempDir()
	marker, pidFile := filepath.Join(directory, "term"), filepath.Join(directory, "pid")
	script := sourceGitScript(t, `trap 'echo term > '`+marker+`'; exit 143' TERM
/bin/sleep 30 >/dev/null 2>&1 &
echo $! > '`+pidFile+`.tmp' && mv '`+pidFile+`.tmp' '`+pidFile+`'
wait
`)
	base := context.WithValue(liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, script)), sourceTerminateGraceKey{}, 5*time.Second)
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	go func() {
		defer cancel()
		for {
			if _, err := os.Stat(pidFile); err == nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	started := time.Now()
	got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3")
	if sourceViewCode(got) != "deadline-exceeded" || time.Since(started) > 25*time.Second {
		t.Fatalf("cancelled read=%v after %v", got, time.Since(started))
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("git descendant %d survived cancellation: %v", pid, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("git child did not receive TERM: %v", err)
	}
}

// Interface: a FIFO packet refuses without blocking on open.
func TestSourceViewRefusesFIFOPacket(t *testing.T) {
	t.Parallel()
	root, commit, _ := sourceViewRepository(t)
	fifo := filepath.Join(t.TempDir(), "packet")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--root", root, "--packet", fifo, "--commit", commit, "--result", "0", "--evidence", "0", "--lines", "3:3"}
	var stdout strings.Builder
	if status := runSourceViewAdapter(context.Background(), args, &stdout); status != 1 || !strings.Contains(stdout.String(), `"code":"packet-file-type"`) {
		t.Fatalf("FIFO packet status=%d output=%s", status, stdout.String())
	}
}

// ESV-V0-001: the read bytes must hash to the selected blob OID under the
// repository object format, so a Git read returning other bytes refuses.
func TestSourceViewVerifiesBlobObjectDigest(t *testing.T) {
	t.Parallel()
	root, commit, packet := sourceViewRepository(t)
	script := sourceGitScript(t, `case " $* " in *" cat-file blob "*) /usr/bin/git "$@" | tr c C; exit 0;; esac
exec /usr/bin/git "$@"
`)
	ctx := liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, script))
	if got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3"); sourceViewCode(got) != "stale-blob" || got["view"] != nil {
		t.Fatalf("substituted blob bytes accepted: %v", got)
	}
}

// ESV-V0-003: one source view runs a fixed Git command sequence within the
// frozen 12-command bound.
func TestSourceViewStaysWithinGitCommandBudget(t *testing.T) {
	t.Parallel()
	root, commit, packet := sourceViewRepository(t)
	commands := filepath.Join(t.TempDir(), "commands")
	script := sourceGitScript(t, `echo >> '`+commands+`'
exec /usr/bin/git "$@"
`)
	ctx := liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, script))
	got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3")
	raw, err := os.ReadFile(commands)
	if err != nil || got["ok"] != true || strings.Count(string(raw), "\n") > 12 {
		t.Fatalf("Git commands=%d err=%v output=%v", strings.Count(string(raw), "\n"), err, got)
	}
}

// ESV-V0-011: a git cat-file read that fails refuses as git-failed in the source
// view and in context --expand, and only a read over the blob limit refuses as
// blob-budget or expand-budget (V1-2258).
func TestSourceBlobReadFailureIsDistinctFromBudget(t *testing.T) {
	t.Parallel()
	root, commit, packet := sourceViewRepository(t)
	failing := sourceGitScript(t, `case " $* " in *" cat-file blob "*) echo 'fatal: object unreadable' >&2; exit 128;; esac
exec /usr/bin/git "$@"
`)
	oversize := sourceGitScript(t, `case " $* " in *" cat-file blob "*) head -c 1048578 /dev/zero; exit 0;; esac
exec /usr/bin/git "$@"
`)
	for _, test := range []struct{ name, script, want string }{
		{"failed-read", failing, "git-failed"},
		{"oversize-read", oversize, "blob-budget"},
	} {
		ctx := liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, test.script))
		if got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3"); sourceViewCode(got) != test.want || got["view"] != nil {
			t.Fatalf("source view %s: %v, want %s", test.name, got, test.want)
		}
	}
	tree := gitOutput(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitOutput(t, root, "rev-parse", "HEAD:spec.md")
	handle := "cv1:" + tree + ":" + blob + ":all:spec.md"
	for _, test := range []struct{ name, script, want string }{
		{"failed-read", failing, "git-failed"},
		{"oversize-read", oversize, "expand-budget"},
	} {
		ctx := liftedSourceDeadline(context.WithValue(context.Background(), sourceGitExecutableKey{}, test.script))
		_, err := expandContextHandle(ctx, root, handle, 4096)
		var failure *gokernel.Error
		if !errors.As(err, &failure) || failure.Code != test.want {
			t.Fatalf("context --expand %s: %v, want %s", test.name, err, test.want)
		}
	}
}

// ESV-V0-014: when the sequence bound expires or the caller cancels partway
// through `adapter source-view`, every Git step refuses as deadline-exceeded
// instead of the code its cut-short read would otherwise imply (V1-2320). Each
// case stalls exactly one step; before the fix the cases answered
// repository-root, object-format, stale-commit, stale-tree, missing-path or
// git-failed.
func TestSourceViewDeadlineExpiryRefusesAsDeadlineExceeded(t *testing.T) {
	t.Parallel()
	root, commit, packet := sourceViewRepository(t)
	for _, step := range []struct{ name, pattern string }{
		{"toplevel", `*" --show-toplevel "*`},
		{"object-format", `*" --show-object-format "*`},
		{"head", "head"},
		{"commit-tree", `*"^{tree} "*`},
		{"ls-tree", `*" ls-tree "*`},
		{"cat-file", `*" cat-file blob "*`},
		{"recheck", "recheck"},
	} {
		for _, cancelled := range []bool{false, true} {
			marker := filepath.Join(t.TempDir(), "seen")
			body := `case " $* " in ` + step.pattern + `) exec /bin/sleep 30;; esac
exec /usr/bin/git "$@"
`
			if step.pattern == "head" || step.pattern == "recheck" {
				first, second := "exec /bin/sleep 30", ": > '"+marker+"'"
				if step.pattern == "recheck" {
					first, second = ": > '"+marker+"'", "exec /bin/sleep 30"
				}
				body = `case " $* " in *" rev-parse HEAD "*) if [ -e '` + marker + `' ]; then ` + second + `; else ` + first + `; fi;; esac
exec /usr/bin/git "$@"
`
			}
			ctx := context.WithValue(context.Background(), sourceGitExecutableKey{}, sourceGitScript(t, body))
			if cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.WithValue(ctx, sourceDeadlineKey{}, time.Minute))
				timer := time.AfterFunc(time.Second, cancel)
				defer timer.Stop()
			} else {
				ctx = context.WithValue(ctx, sourceDeadlineKey{}, time.Second)
			}
			got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3")
			if sourceViewCode(got) != "deadline-exceeded" || got["view"] != nil {
				t.Errorf("%s cancelled=%t: %v, want deadline-exceeded", step.name, cancelled, got["error"])
			}
		}
	}
}

// ESV-V0-013: when the sequence bound expires partway through `context
// --expand`, every Git step refuses as deadline-exceeded instead of the
// validation code its failed read would otherwise imply (V1-1412). Each case
// stalls exactly one step, and the bound expires only once that step's Git
// stand-in reports it has stalled, so host load cannot move the expiry to an
// earlier step (V1-2433). The one-minute bound is a hang detector, not the
// expiry under test (decision 0082). Before the V1-1412 fix 13 of the 14 cases
// answered repository-root, invalid-handle, missing-handle or git-failed.
func TestContextExpandDeadlineExpiryRefusesAsDeadlineExceeded(t *testing.T) {
	t.Parallel()
	root, _, _ := sourceViewRepository(t)
	tree := gitOutput(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitOutput(t, root, "rev-parse", "HEAD:spec.md")
	for _, step := range []struct{ name, pattern string }{
		{"toplevel", `*" --show-toplevel "*`},
		{"object-format", `*" --show-object-format "*`},
		{"head-tree", `*" HEAD^{tree} "*`},
		{"ls-tree", `*" ls-tree "*`},
		{"disambiguate", `*" --disambiguate="*`},
		{"cat-file", `*" cat-file blob "*`},
		{"recheck", "recheck"},
	} {
		for _, rangeText := range []string{"all", "1-999999999"} {
			fixture := t.TempDir()
			marker, stalled := filepath.Join(fixture, "seen"), filepath.Join(fixture, "stalled")
			stall := `: > '` + stalled + `'; exec /bin/sleep 30`
			body := `case " $* " in ` + step.pattern + `) ` + stall + `;; esac
exec /usr/bin/git "$@"
`
			if step.pattern == "recheck" {
				body = `case " $* " in *" HEAD^{tree} "*) if [ -e '` + marker + `' ]; then ` + stall + `; fi; : > '` + marker + `';; esac
exec /usr/bin/git "$@"
`
			}
			parent, expire := workFinalInterruption("deadline")
			ctx := context.WithValue(context.WithValue(parent, sourceGitExecutableKey{}, sourceGitScript(t, body)), sourceDeadlineKey{}, time.Minute)
			watcherDone, watcher := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(watcher)
				sourceExpireWhenStalled(stalled, expire, watcherDone)
			}()
			_, err := expandContextHandle(ctx, root, "cv1:"+tree+":"+blob+":"+rangeText+":spec.md", 1<<20)
			close(watcherDone)
			<-watcher
			want, wantStalled := "deadline-exceeded", true
			if step.name == "recheck" && rangeText != "all" {
				// The range refuses before the stalled recheck runs.
				want, wantStalled = "invalid-handle", false
			}
			var failure *gokernel.Error
			if !errors.As(err, &failure) || failure.Code != want {
				t.Errorf("%s %s: %v, want %s", step.name, rangeText, err, want)
			}
			if _, statErr := os.Stat(stalled); (statErr == nil) != wantStalled {
				t.Errorf("%s %s: stalled step reached = %t, want %t", step.name, rangeText, statErr == nil, wantStalled)
			}
		}
	}
}

// ESV-V0-013: the sequence bound itself expires. The first Git step stalls
// past a one-second bound with no deadline on the caller's context, so the
// refusal can only come from expandContextHandle's own bound. The stall is at
// the first step, so no earlier step can absorb host load: the bound started
// before that step's process and its per-command timer, so it has expired by
// the time the stopped step returns. If the bound did not expire, the stopped
// step would refuse as repository-root.
func TestContextExpandSequenceBoundExpires(t *testing.T) {
	t.Parallel()
	root, _, _ := sourceViewRepository(t)
	tree := gitOutput(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitOutput(t, root, "rev-parse", "HEAD:spec.md")
	stalled := filepath.Join(t.TempDir(), "stalled")
	body := `case " $* " in *" --show-toplevel "*) : > '` + stalled + `'; exec /bin/sleep 30;; esac
exec /usr/bin/git "$@"
`
	ctx := context.WithValue(context.WithValue(context.Background(), sourceGitExecutableKey{}, sourceGitScript(t, body)), sourceDeadlineKey{}, time.Second)
	started := time.Now()
	_, err := expandContextHandle(ctx, root, "cv1:"+tree+":"+blob+":all:spec.md", 1<<20)
	var failure *gokernel.Error
	if !errors.As(err, &failure) || failure.Code != "deadline-exceeded" || !strings.Contains(failure.Message, "bound expired") {
		t.Fatalf("stalled first step: %v, want deadline-exceeded from the expired bound", err)
	}
	if _, statErr := os.Stat(stalled); statErr != nil {
		t.Fatalf("the stalled step was not reached: %v", statErr)
	}
	// A hang detector, not a budget (decision 0082): the stand-in sleeps 30 s.
	if elapsed := time.Since(started); elapsed > 25*time.Second {
		t.Fatalf("the bound did not stop the stalled step: %s", elapsed)
	}
}

// sourceExpireWhenStalled calls expire once the stalled file exists, or
// returns when done closes first.
func sourceExpireWhenStalled(stalled string, expire func(), done <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if _, err := os.Stat(stalled); err == nil {
				expire()
				return
			}
		}
	}
}
