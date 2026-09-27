// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The runner is exercised against a Corvint binary built from this checkout;
// the offline provider build inside it sees only a copy of ../main.go.
func TestProviderKitConformanceRunner(t *testing.T) {
	t.Run("EEP-V0-021 two-transport conformance", func(t *testing.T) {
		corvint := filepath.Join(t.TempDir(), "corvint")
		build := exec.CommandContext(t.Context(), "go", "build", "-o", corvint, "../../../../cmd/corvint")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build corvint: %v\n%s", err, output)
		}
		var out bytes.Buffer
		if err := run(t.Context(), []string{"-kit", "..", "-corvint", corvint}, &out); err != nil {
			t.Fatalf("conformance: %v\n%s", err, out.Bytes())
		}
		if !strings.HasSuffix(out.String(), "kit 0.2.0 conformance: 9 cases agree over file and command transports\n") {
			t.Fatalf("summary: %s", out.Bytes())
		}
		if err := run(t.Context(), []string{"-kit", "..", "-corvint", corvint, "-provider-version", "0.2.0"}, &out); err == nil {
			t.Fatal("a provider version the source does not declare passed conformance")
		}
	})
}

// A provider that never exits is reported as a timeout refusal within the
// probe's bound rather than hanging the runner (V1-0136).
func TestRefusalBoundsNeverExitingProvider(t *testing.T) {
	provider := filepath.Join(t.TempDir(), "provider")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\nexec sleep 3600\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	defer func(saved time.Duration) { refusalTimeout = saved }(refusalTimeout)
	refusalTimeout = 200 * time.Millisecond
	start := time.Now()
	err := (&harness{provider: provider}).refusal(t.Context())
	if err == nil || !strings.Contains(err.Error(), "timeout refusal") {
		t.Fatalf("refusal of a never-exiting provider: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("refusal took %s", elapsed)
	}
}
