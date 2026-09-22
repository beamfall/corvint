//go:build darwin || linux || windows

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func qualifyTestExecutable(t *testing.T) (string, *os.File, executableEvidence) {
	t.Helper()
	directory := filepath.Join(stableTestDirectory(t, "executable-ancestor-"), "bin")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	file, evidence, failure := openExecutableStableRead(context.Background(), path)
	if failure != nil {
		t.Fatalf("qualify executable: %#v", failure)
	}
	t.Cleanup(func() { _ = file.Close() })
	return path, file, evidence
}

// Decision 0175: creating and removing an entry in an ancestor directory
// changes its modification time and link count, not what the path resolves to.
func TestExecutableVerificationIgnoresAncestorEntryChurn(t *testing.T) {
	t.Run("LOD-V0-005 ancestor churn", func(t *testing.T) {
		path, file, evidence := qualifyTestExecutable(t)
		sibling := filepath.Join(filepath.Dir(path), "sibling")
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = os.Mkdir(sibling, 0o755)
				_ = os.Remove(sibling)
			}
		}()
		drifted := 0
		for range 500 {
			if !verifyExecutableBinding(path, file, evidence) {
				drifted++
			}
		}
		close(stop)
		<-done
		if drifted != 0 {
			t.Fatalf("ancestor entry churn reported executable drift in %d of 500 verifications", drifted)
		}
	})
}

func TestExecutableVerificationDetectsReplacedExecutable(t *testing.T) {
	t.Run("LOD-V0-005 replaced executable", func(t *testing.T) {
		path, file, evidence := qualifyTestExecutable(t)
		replacement := path + ".new"
		if err := os.WriteFile(replacement, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal(err)
		}
		if verifyExecutableBinding(path, file, evidence) {
			t.Fatal("same-content executable replaced by rename was not drift")
		}
	})
}
