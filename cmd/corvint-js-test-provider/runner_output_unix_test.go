//go:build darwin || linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// openWithin opens path as --runner-output, failing the test if the open
// blocks.
func openWithin(t *testing.T, path string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, closeOutput, err := openRunnerOutput(path, &bytes.Buffer{})
		if err == nil {
			closeOutput()
		}
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("opening %s blocked", path)
		return nil
	}
}

// PWP-V0-028: --runner-output never truncates through a symlink.
func TestPWPV0028RunnerOutputRefusesSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "unrelated.txt")
	if err := os.WriteFile(target, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "runner.log")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := openWithin(t, link); err == nil || !strings.HasPrefix(err.Error(), "runner-output-unavailable") {
		t.Fatalf("symlinked runner output: %v", err)
	}
	if data, _ := os.ReadFile(target); string(data) != "keep me" {
		t.Fatalf("symlink target was truncated: %q", data)
	}
}

// PWP-V0-028: a FIFO, with or without a reader, is refused without blocking.
func TestPWPV0028RunnerOutputRefusesFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "runner.log")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := openWithin(t, fifo); err == nil || !strings.HasPrefix(err.Error(), "runner-output-unavailable") {
		t.Fatalf("FIFO without a reader: %v", err)
	}
	reader, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := openWithin(t, fifo); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("FIFO with a reader: %v", err)
	}
}

// PWP-V0-028: an existing log keeps no permissive mode; it becomes 0600.
func TestPWPV0028RunnerOutputExistingFileBecomes0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner.log")
	if err := os.WriteFile(path, []byte("earlier run"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := openWithin(t, path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != 0 {
		t.Fatalf("existing runner output: %v %v", info, err)
	}
}

// PWP-V0-028: procgroup's early-end notice makes the close report
// runner-output-incomplete and stops later writes.
func TestPWPV0028RunnerOutputReportsEarlyEnd(t *testing.T) {
	var stderr bytes.Buffer
	output, closeOutput, err := openRunnerOutput(filepath.Join(t.TempDir(), "runner.log"), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	output.TeeIncomplete()
	if _, err := output.Write([]byte("late")); err == nil {
		t.Fatal("an ended copy accepted a write")
	}
	closeOutput()
	if !strings.HasPrefix(stderr.String(), "runner-output-incomplete") {
		t.Fatalf("stderr %q", stderr.String())
	}
}
