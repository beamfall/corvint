//go:build !unix

package main

import (
	"os"
	"os/exec"
)

// Off Unix the ledger refuses its directory and runs every step (GL-V0-005),
// so command lookup, the temporary and the home directory stay the
// process's, as they were before V1-4188.

func lookPath(file string, _ invocation) (string, error) {
	path, err := exec.LookPath(file)
	if err != nil {
		return file, err
	}
	return path, nil
}

func tempDir(invocation) string { return os.TempDir() }

func userHomeDir(invocation) (string, error) { return os.UserHomeDir() }
