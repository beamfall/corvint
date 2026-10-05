//go:build darwin || linux

package gokernel

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// SOL-V0-010, AHI-044 (V1-0767): a held ledger lock delays a coded OpenCode event by at most
// adapterCodeAppendBound and leaves its response unchanged; the row lands once the lock is free.
func TestOpenCodeAdapterCodesDoNotWaitOnAHeldLedgerLock(t *testing.T) {
	root := testRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, ".corvint")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	request := request(root, "post-tool", `{"adapterCodes":["post-tool-path-not-project-relative"],"changedPaths":["src/main.go"]}`)
	request.Host, request.CorvintVersion = "opencode", "1.0.0-test"
	started := time.Now()
	result, err := HandleEvent(request)
	if elapsed := time.Since(started); err != nil || elapsed > adapterCodeAppendBound+5*time.Second {
		t.Fatalf("held ledger lock: err=%v elapsed=%s", err, elapsed)
	}
	if result["ok"] != true {
		t.Fatalf("held ledger lock changed the response: %v", result)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(directory, "self-observations.jsonl")
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if data, _ := os.ReadFile(ledger); bytes.Contains(data, []byte(`"post-tool-path-not-project-relative"`)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the abandoned append did not land after the lock was released")
		}
	}
}
