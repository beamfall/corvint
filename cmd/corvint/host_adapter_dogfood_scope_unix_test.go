//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// V1-3022: the go.mod module line is read with a byte cap. A normal Corvint
// go.mod still applies, an over-long first line yields no guidance, and a FIFO
// go.mod is never opened for a blocking read.
func TestDogfoodWorkflowGoModReadIsBounded(t *testing.T) {
	write := func(content string) string {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return root
	}
	if !dogfoodWorkflowApplies(write(corvintModuleLine + "\n\ngo 1.27\n")) {
		t.Fatal("a normal Corvint go.mod no longer applies")
	}
	if !dogfoodWorkflowApplies(write(corvintModuleLine)) {
		t.Fatal("a Corvint go.mod without a trailing newline no longer applies")
	}
	if dogfoodWorkflowApplies(write("module example.com/other\n")) {
		t.Fatal("a foreign go.mod applies")
	}
	if dogfoodWorkflowApplies(write(corvintModuleLine + strings.Repeat(" ", goModFirstLineLimit) + "\n")) {
		t.Fatal("an over-long first line applies")
	}
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "go.mod"), 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan bool, 1)
	go func() { done <- dogfoodWorkflowApplies(root) }()
	select {
	case applies := <-done:
		if applies {
			t.Fatal("a FIFO go.mod applies")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dogfoodWorkflowApplies blocked on a FIFO go.mod")
	}
}
