// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package releasegate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestRunGitCapsOutputCopiedFromThePipe_GPKV09930 runs a pinned stand-in Git
// that prints past the limit; runGitProcess copies its pipe into limitBuffer
// with io.Copy, so the run must refuse at the bound rather than return every
// byte (V1-5122, GPK-V0-9930).
func TestRunGitCapsOutputCopiedFromThePipe_GPKV09930(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "git")
	if err := execfile.WriteFile(script, []byte("#!/bin/sh\nprintf '%064d\\n' 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptBytes, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	identity := gitIdentity{path: script, sha256: sha256Hex(scriptBytes)}
	output, err := runGitAt(context.Background(), identity, directory, 16, "rev-parse", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "exceeds release gate bound") {
		t.Fatalf("output %q err %v, want the release gate bound refusal", output, err)
	}
}
