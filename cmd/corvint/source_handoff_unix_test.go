//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// sourceGitScript writes an executable stand-in for /usr/bin/git.
func sourceGitScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
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
	ctx := context.WithValue(context.Background(), sourceGitExecutableKey{}, script)
	if got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3"); sourceViewCode(got) != "stale-commit" || got["view"] != nil {
		t.Fatalf("moved HEAD accepted: %v", got)
	}
}

// ESV-V0-003: cancelling a Git read sends TERM to the child's owned process
// group and reaps its descendants before the refusal is emitted.
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
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), sourceGitExecutableKey{}, script))
	defer cancel()
	go func() {
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(pidFile); err == nil {
				break
			}
		}
		cancel()
	}()
	started := time.Now()
	got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3")
	if sourceViewCode(got) != "repository-root" || time.Since(started) > 8*time.Second {
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
	ctx := context.WithValue(context.Background(), sourceGitExecutableKey{}, script)
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
	ctx := context.WithValue(context.Background(), sourceGitExecutableKey{}, script)
	got := runSourceViewTestContext(t, ctx, root, commit, packet, "--lines", "3:3")
	raw, err := os.ReadFile(commands)
	if err != nil || got["ok"] != true || strings.Count(string(raw), "\n") > 12 {
		t.Fatalf("Git commands=%d err=%v output=%v", strings.Count(string(raw), "\n"), err, got)
	}
}
