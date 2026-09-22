package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredPerfProtocolPreservesEvidence(t *testing.T) {
	t.Run("GOC-V0-005 cancelled measurement cannot execute", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "report.json")
		frozen := []byte("{\"slicePerformanceStatus\":\"NOT_RUN\"}\n")
		if err := os.WriteFile(output, frozen, 0600); err != nil {
			t.Fatal(err)
		}
		for _, verb := range []string{"run", "diagnose", "capture"} {
			err := runCLI(context.Background(), []string{verb, "--out", output})
			if err == nil || !strings.HasPrefix(err.Error(), "retired-python-protocol:") {
				t.Fatalf("%s: %v", verb, err)
			}
			got, err := os.ReadFile(output)
			if err != nil || string(got) != string(frozen) {
				t.Fatalf("historical evidence changed: %q %v", got, err)
			}
		}
	})
}
