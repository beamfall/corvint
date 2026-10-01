//go:build unix

package gitauth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBoundedBlobRejectsHeaderWithoutFallback(t *testing.T) {
	t.Run("DLT-V0-003 hostile header refusal", testBoundedBlobRejectsHeaderWithoutFallback)
}
func testBoundedBlobRejectsHeaderWithoutFallback(t *testing.T) {
	for _, response := range []string{"wrong blob 3\\nabc\\n", "%s tree 3\\nabc\\n", "%s blob -1\\n", "%s blob 67108864\\n", "%s blob 3\\na\\n", "%s blob text\\n"} {
		t.Run(response, func(t *testing.T) {
			root, _, head := makeRepo(t)
			r := open(t, root)
			entry, _, err := r.LookupTreeEntry(context.Background(), head, "f.go")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			quoted := "'" + strings.ReplaceAll(calls, "'", "'\\''") + "'"
			script := "#!/bin/sh\nprintf x >>" + quoted + "\nIFS= read -r oid\nprintf '" + response + "' \"$oid\"\n"
			if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if body, err := r.BlobBytesBounded(context.Background(), entry.OID, 1024); err == nil || body != nil {
				t.Fatalf("bad header admitted %q %v", body, err)
			}
			raw, err := os.ReadFile(calls)
			if err != nil || len(raw) != 1 {
				t.Fatalf("fallback invoked %q %v", raw, err)
			}
		})
	}
}
func TestBoundedBlobCumulativeBudgetBeforeBody(t *testing.T) {
	t.Run("DLT-V0-003 cumulative budget before allocation", testBoundedBlobCumulativeBudgetBeforeBody)
}
func testBoundedBlobCumulativeBudgetBeforeBody(t *testing.T) {
	root, _, head := makeRepo(t)
	r := open(t, root)
	entry, _, err := r.LookupTreeEntry(context.Background(), head, "f.go")
	if err != nil {
		t.Fatal(err)
	}
	r.blobBytes = MaxTotalBlobBytes
	if _, err := r.BlobBytesBounded(context.Background(), entry.OID, 4<<20); err == nil {
		t.Fatal("cumulative bound bypassed")
	}
}
func TestBoundedBlobCancellationRetiresDescendant(t *testing.T) {
	t.Run("DLT-V0-003 cancellation retires descendant", testBoundedBlobCancellationRetiresDescendant)
}
func testBoundedBlobCancellationRetiresDescendant(t *testing.T) {
	root, _, head := makeRepo(t)
	r := open(t, root)
	entry, _, err := r.LookupTreeEntry(context.Background(), head, "f.go")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child")
	quoted := "'" + strings.ReplaceAll(pidFile, "'", "'\\''") + "'"
	script := "#!/bin/sh\nIFS= read -r oid\nprintf '%s blob 3\\n' \"$oid\"\nsleep 30 &\necho $! >" + quoted + "\nwait\n"
	os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := r.BlobBytesBounded(ctx, entry.OID, 1024); done <- err }()
	var pid int
	for i := 0; i < 200; i++ {
		data, e := os.ReadFile(pidFile)
		if e == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		cancel()
		<-done
		t.Fatal("fixture descendant never started")
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel admitted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not return")
	}
	for i := 0; i < 100; i++ {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(fmt.Sprintf("descendant %d remains", pid))
}
