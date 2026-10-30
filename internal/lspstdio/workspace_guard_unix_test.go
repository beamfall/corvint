//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package lspstdio

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// LED-V0-003/LQP-V0-004: a third file affects definitions independently of overlays.
func TestWorkspaceThirdFileDrift(t *testing.T) {
	h := semanticHarnessOptions(t, "normal", true)
	path := filepath.Join(h.root, "third.go")
	if err := os.WriteFile(path, []byte("package p\nvar old int\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h.init(t)
	h.open(t)
	if err := os.WriteFile(path, []byte("package p\nvar changed int\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 44, "method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri}, "position": map[string]int{"line": 2, "character": 8}}})
	m := h.recv(t)
	if _, ok := m["result"]; ok {
		t.Fatal("original overlay-only engine returned definition after third-file drift")
	}
}

func workspaceDefinition(id int, uri string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 2, "character": 8}}}
}
func assertWorkspaceStale(t *testing.T, h *semanticHarness, id int) {
	t.Helper()
	h.send(t, workspaceDefinition(id, h.uri))
	m := h.recv(t)
	if _, ok := m["result"]; ok {
		t.Fatal("stale definition result")
	}
	if !strings.Contains(string(m["error"]), "-32801") {
		t.Fatalf("not stale: %s", m["error"])
	}
}

// backendBarrierDeadline bounds each FIFO handshake with the guard-barrier backend. A blocking
// FIFO open has no deadline of its own, so without this a backend or session that ended before
// the barrier held the package until the 30 m test timeout (V1-2671, GH #1766).
const backendBarrierDeadline = 15 * time.Second

// workspaceBarrierTestDeadline bounds a whole barrier test, including its unbounded recv calls.
const workspaceBarrierTestDeadline = 45 * time.Second

// fifoHandshake runs one blocking FIFO open plus a one-byte read or write. It fails the test with
// the session's own exit error when the session ends first, and with a goroutine dump when the
// deadline passes; either way it unblocks the stuck open with a non-blocking peer open.
func fifoHandshake(t *testing.T, h *semanticHarness, name string, write bool) {
	t.Helper()
	path := filepath.Join(h.scratch, name)
	done := make(chan error, 1)
	go func() {
		flag := os.O_RDONLY
		if write {
			flag = os.O_WRONLY
		}
		f, e := os.OpenFile(path, flag, 0)
		if e != nil {
			done <- e
			return
		}
		var b [1]byte
		if write {
			b[0] = '1'
			_, e = f.Write(b[:])
		} else {
			_, e = f.Read(b[:])
		}
		f.Close()
		done <- e
	}()
	unblock := func() {
		// A non-blocking open of the opposite end completes the stuck open, which then reads EOF
		// or writes into a FIFO nobody drains. The worker may not have reached its open yet (a
		// non-blocking writer open fails with ENXIO until a reader exists), so retry briefly and
		// join the worker rather than leak it.
		flag := os.O_WRONLY
		if write {
			flag = os.O_RDONLY
		}
		giveUp := time.After(5 * time.Second)
		for {
			if f, e := os.OpenFile(path, flag|syscall.O_NONBLOCK, 0); e == nil {
				f.Close()
			}
			select {
			case <-done:
				return
			case <-giveUp:
				t.Logf("backend barrier %s worker did not return after unblock", name)
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	timer := time.NewTimer(backendBarrierDeadline)
	defer timer.Stop()
	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("backend barrier %s: %v", name, e)
		}
	case e := <-h.done:
		h.done <- e // keep it for the harness cleanup join
		unblock()
		t.Fatalf("semantic session ended before backend barrier %s: %v", name, e)
	case <-timer.C:
		unblock()
		buf := make([]byte, 1<<20)
		t.Fatalf("backend barrier %s not reached within %s; goroutines:\n%s", name, backendBarrierDeadline, buf[:runtime.Stack(buf, true)])
	}
}
func awaitBackendBarrier(t *testing.T, h *semanticHarness) {
	t.Helper()
	fifoHandshake(t, h, "received", false)
}
func releaseBackendBarrier(t *testing.T, h *semanticHarness) {
	t.Helper()
	fifoHandshake(t, h, "release", true)
}

// LQP-V0-004/LED-V0-003: a receive/release FIFO proves the backend request is in flight.
func TestWorkspaceHeldBackendDrift(t *testing.T) {
	h := semanticHarnessOptions(t, "guard-barrier", true)
	h.bound(t, workspaceBarrierTestDeadline)
	for _, name := range []string{"received", "release"} {
		if e := syscall.Mkfifo(filepath.Join(h.scratch, name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	path := filepath.Join(h.root, "third.go")
	os.WriteFile(path, []byte("package p\nvar old int\n"), 0600)
	h.init(t)
	h.open(t)
	h.send(t, workspaceDefinition(41, h.uri))
	awaitBackendBarrier(t, h)
	os.WriteFile(path, []byte("package p\nvar changed int\n"), 0600)
	releaseBackendBarrier(t, h)
	m := h.recv(t)
	if _, ok := m["result"]; ok || !strings.Contains(string(m["error"]), "-32801") {
		t.Fatalf("held stale: %s", m)
	}
	os.WriteFile(path, []byte("package p\nvar old int\n"), 0600)
	assertWorkspaceStale(t, h, 42)
}
func TestWorkspaceBranchRootAndHealthy(t *testing.T) {
	for _, kind := range []string{"branch", "root", "healthy", "save"} {
		t.Run(kind, func(t *testing.T) {
			h := semanticHarnessOptions(t, "normal", true)
			h.init(t)
			h.open(t)
			h.send(t, workspaceDefinition(2, h.uri))
			m := h.recv(t)
			if _, ok := m["result"]; !ok {
				t.Fatalf("healthy initial: %s", m)
			}
			switch kind {
			case "branch":
				fixtureGit(t, h.root, "checkout", "-qb", "other")
				assertWorkspaceStale(t, h, 3)
			case "root":
				if e := os.Rename(h.root, h.root+"-old"); e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { os.RemoveAll(h.root + "-old") })
				os.Mkdir(h.root, 0700)
				assertWorkspaceStale(t, h, 3)
			case "save":
				h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]string{"uri": h.uri}}})
				assertWorkspaceStale(t, h, 3)
			case "healthy":
				h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": h.uri, "version": 2}, "contentChanges": []map[string]string{{"text": "package p\n/*😀*/ var x int\nvar y = x\n"}}}})
				h.send(t, workspaceDefinition(3, h.uri))
				m = h.recv(t)
				if _, ok := m["result"]; !ok {
					t.Fatalf("healthy overlay: %s", m)
				}
			}
		})
	}
}
func TestWorkspaceWholeRootAndBounds(t *testing.T) {
	root, _ := filepath.EvalSymlinks(contextFixture(t))
	os.MkdirAll(filepath.Join(root, "a"), 0700)
	os.MkdirAll(filepath.Join(root, "b"), 0700)
	os.MkdirAll(filepath.Join(root, "vendor"), 0700)
	for _, p := range []string{"go.work", "a/go.mod", "b/go.mod", "vendor/modules.txt", ".ignored", "embedded.bin"} {
		os.WriteFile(filepath.Join(root, p), []byte("fixture\n"), 0600)
	}
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	before, e := observeWorkspace(ctx, root)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(root, "b/go.mod"), []byte("changed\n"), 0600)
	after, e := observeWorkspace(ctx, root)
	if e != nil || after.digest == before.digest {
		t.Fatal("contained go.work module not observed", e)
	}
	os.Symlink("/tmp", filepath.Join(root, "escape"))
	if _, e = observeWorkspace(ctx, root); e == nil {
		t.Fatal("symlink accepted")
	}
	os.Remove(filepath.Join(root, "escape"))
	f, e := os.Create(filepath.Join(root, "huge"))
	if e != nil {
		t.Fatal(e)
	}
	f.Truncate((16 << 20) + 1)
	f.Close()
	if _, e = observeWorkspace(ctx, root); e == nil {
		t.Fatal("size ceiling accepted")
	}
	os.Remove(filepath.Join(root, "huge"))
	budget := time.Duration(0)
	g := workspaceGuard{baseline: after}
	if g.check(ctx, root, &budget) == nil || !g.stale {
		t.Fatal("zero budget did not latch")
	}
}

func TestWorkspaceObservationCost(t *testing.T) {
	root, _ := filepath.EvalSymlinks(contextFixture(t))
	ctx := context.Background()
	start := time.Now()
	a, e := observeWorkspace(ctx, root)
	first := time.Since(start)
	start = time.Now()
	b, f := observeWorkspace(ctx, root)
	t.Logf("first=%s second=%s stable=%v errors=%v/%v", first, time.Since(start), a.digest == b.digest, e, f)
	if e != nil || f != nil {
		t.Fatal(e, f)
	}
}

// LED-V0-003/005: notification invalidation joins once and reports stale once, not healthy cancellation.
func TestWorkspacePendingSave(t *testing.T) {
	h := semanticHarnessOptions(t, "guard-barrier", true)
	h.bound(t, workspaceBarrierTestDeadline)
	for _, name := range []string{"received", "release"} {
		if e := syscall.Mkfifo(filepath.Join(h.scratch, name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	h.init(t)
	h.open(t)
	h.send(t, workspaceDefinition(81, h.uri))
	awaitBackendBarrier(t, h)
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]string{"uri": h.uri}}})
	m := h.recv(t)
	if _, ok := m["result"]; ok || !strings.Contains(string(m["error"]), "-32801") {
		t.Fatalf("pending save did not stale: %s", m)
	}
	assertWorkspaceStale(t, h, 82)
}

// V1-2671 (GH #1766): a pre-dispatch observation that overruns its budget latches stale and the
// session answers the definition itself, so the backend never reaches the FIFO barrier. The nightly
// hang was this contract meeting a deadline-free barrier open under the 200 ms product budget on a
// loaded macOS runner. Pin the contract under the bounded harness: the stale reply arrives, the
// guard stays latched, and the backend still completes the shutdown exchange, which it could not do
// from inside the barrier handler. A dispatch despite the overrun parks the backend at a barrier
// nobody releases: each definition is answered only at its 20 s request limit and the shutdown
// exchange cannot complete, so the test fails inside its 45 s deadline instead of hanging.
func TestWorkspaceOverBudgetAnswersStaleBeforeDispatch(t *testing.T) {
	h := semanticHarnessOptions(t, "guard-barrier", true)
	h.bound(t, workspaceBarrierTestDeadline)
	for _, name := range []string{"received", "release"} {
		if e := syscall.Mkfifo(filepath.Join(h.scratch, name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	previous := workspaceObservationBudget
	workspaceObservationBudget = time.Nanosecond
	t.Cleanup(func() { workspaceObservationBudget = previous })
	h.init(t)
	h.open(t)
	assertWorkspaceStale(t, h, 81)
	assertWorkspaceStale(t, h, 82)
	h.send(t, map[string]any{"jsonrpc": "2.0", "id": 83, "method": "shutdown"})
	if m := h.recv(t); string(m["result"]) != "null" || m["error"] != nil {
		t.Fatalf("shutdown response: %s", m)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	select {
	case e := <-h.done:
		h.done <- e
		if e != nil {
			t.Fatalf("session did not retire cleanly after the stale reply: %v", e)
		}
	case <-time.After(backendBarrierDeadline):
		t.Fatal("exit did not retire the session")
	}
}
func TestWorkspaceSameBytesReplacement(t *testing.T) {
	root, _ := filepath.EvalSymlinks(contextFixture(t))
	ctx := context.Background()
	baseline, e := observeWorkspace(ctx, root)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "go.mod")
	info, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	body, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	replacement := filepath.Join(root, "replacement")
	os.WriteFile(replacement, body, info.Mode())
	os.Chtimes(replacement, info.ModTime(), info.ModTime())
	os.Rename(replacement, path)
	g := workspaceGuard{baseline: baseline}
	budget := workspaceObservationBudget
	if g.check(ctx, root, &budget) == nil {
		t.Fatal("same-byte/mtime replacement inode passed")
	}
}
