//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"os"
	"path/filepath"
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

// LQP-V0-004/LED-V0-003: a receive/release FIFO proves the backend request is in flight.
func TestWorkspaceHeldBackendDrift(t *testing.T) {
	h := semanticHarnessOptions(t, "guard-barrier", true)
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
	f, e := os.Open(filepath.Join(h.scratch, "received"))
	if e != nil {
		t.Fatal(e)
	}
	var b [1]byte
	_, e = f.Read(b[:])
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(path, []byte("package p\nvar changed int\n"), 0600)
	if e = os.WriteFile(filepath.Join(h.scratch, "release"), []byte("1"), 0600); e != nil {
		t.Fatal(e)
	}
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
	for _, name := range []string{"received", "release"} {
		if e := syscall.Mkfifo(filepath.Join(h.scratch, name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	h.init(t)
	h.open(t)
	h.send(t, workspaceDefinition(81, h.uri))
	f, e := os.Open(filepath.Join(h.scratch, "received"))
	if e != nil {
		t.Fatal(e)
	}
	var b [1]byte
	_, e = f.Read(b[:])
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	h.send(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didSave", "params": map[string]any{"textDocument": map[string]string{"uri": h.uri}}})
	m := h.recv(t)
	if _, ok := m["result"]; ok || !strings.Contains(string(m["error"]), "-32801") {
		t.Fatalf("pending save did not stale: %s", m)
	}
	assertWorkspaceStale(t, h, 82)
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
