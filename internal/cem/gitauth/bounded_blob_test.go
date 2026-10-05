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

func TestBoundedBlobOneMiBHeaderAdmission(t *testing.T) {
	t.Run("DLT-V0-003 smaller and cumulative bounds refuse before body", func(t *testing.T) {
		const limit = 1 << 20
		for _, cumulative := range []bool{false, true} {
			t.Run(fmt.Sprintf("cumulative=%v", cumulative), func(t *testing.T) {
				root, _, head := makeRepo(t)
				r := open(t, root)
				entry, _, err := r.LookupTreeEntry(context.Background(), head, "f.go")
				if err != nil {
					t.Fatal(err)
				}
				size := limit + 1
				if cumulative {
					size = limit
					r.blobBytes = MaxTotalBlobBytes - limit + 1
				}
				dir := t.TempDir()
				calls := filepath.Join(dir, "calls")
				quoted := "'" + strings.ReplaceAll(calls, "'", "'\\''") + "'"
				// Supply only the header and wait for another request. A consumer
				// admitting this body must time out; a header refusal returns now.
				script := fmt.Sprintf("#!/bin/sh\nprintf x >>%s\nIFS= read -r oid\nprintf '%%s blob %d\\n' \"$oid\"\nIFS= read -r next\n", quoted, size)
				if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				body, err := r.BlobBytesBounded(ctx, entry.OID, limit)
				if err == nil || body != nil || ctx.Err() != nil {
					t.Fatalf("header was not refused before requesting body: bytes=%d err=%v deadline=%v", len(body), err, ctx.Err())
				}
				raw, err := os.ReadFile(calls)
				if err != nil || string(raw) != "x" {
					t.Fatalf("fallback launched: %q %v", raw, err)
				}
			})
		}
	})
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

func TestBlobBytesWithinReportsOverBoundHeader(t *testing.T) {
	t.Run("DLT-V0-003 V1-0747 consumer-owned refusal before body", func(t *testing.T) {
		root, _, head := makeRepo(t)
		r := open(t, root)
		entry, _, err := r.LookupTreeEntry(context.Background(), head, "f.go")
		if err != nil {
			t.Fatal(err)
		}
		if body, over, err := r.BlobBytesWithin(context.Background(), entry.OID, len(bodyV2)-1); err != nil || !over || body != nil {
			t.Fatalf("bound+1 blob: %q over=%v %v", body, over, err)
		}
		if body, over, err := r.BlobBytesWithin(context.Background(), entry.OID, len(bodyV2)); err != nil || over || string(body) != bodyV2 {
			t.Fatalf("exact-bound blob: %q over=%v %v", body, over, err)
		}
		if _, over, err := r.BlobBytesWithin(context.Background(), "not-an-oid", 1024); err == nil || over {
			t.Fatalf("invalid OID: over=%v %v", over, err)
		}
	})
}
