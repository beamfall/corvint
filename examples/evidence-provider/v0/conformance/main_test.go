package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
