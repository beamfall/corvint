package opencodequalification

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const witnessErrorLine = `{"error":"Error: leaked from elsewhere"}` + "\n"

func shortWitnessTimeout(t *testing.T) {
	t.Helper()
	previous := frameWitnessTimeout
	frameWitnessTimeout = 50 * time.Millisecond
	t.Cleanup(func() { frameWitnessTimeout = previous })
}

// TestAHI620WitnessLogReadIsBounded: an oversized witness log is refused, not read whole.
func TestAHI620WitnessLogReadIsBounded(t *testing.T) {
	root := t.TempDir()
	over := append([]byte(witnessErrorLine), bytes.Repeat([]byte("\n"), maxEvidenceBytes)...)
	if err := os.WriteFile(filepath.Join(root, "witness.jsonl"), over, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularBounded(filepath.Join(root, "witness.jsonl"), maxEvidenceBytes); !errors.Is(err, errEvidenceBound) {
		t.Fatalf("oversized witness log: err %v, want errEvidenceBound", err)
	}
	if got := witnessLastError(root); got != "" {
		t.Fatalf("oversized witness log was parsed: %q", got)
	}
	exact := append([]byte(witnessErrorLine), bytes.Repeat([]byte("\n"), maxEvidenceBytes-len(witnessErrorLine))...)
	if err := os.WriteFile(filepath.Join(root, "witness.jsonl"), exact, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := witnessLastError(root); got != "Error: leaked from elsewhere" {
		t.Fatalf("witness log at the bound: %q", got)
	}
}

// TestAHI620WitnessLogRefusesSymlink: a symlinked witness log cannot import another run's evidence.
func TestAHI620WitnessLogRefusesSymlink(t *testing.T) {
	other := filepath.Join(t.TempDir(), "witness.jsonl")
	if err := os.WriteFile(other, []byte(witnessErrorLine), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(other, filepath.Join(root, "witness.jsonl")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := readRegularBounded(filepath.Join(root, "witness.jsonl"), maxEvidenceBytes); !errors.Is(err, errNotRegularEvidence) {
		t.Fatalf("symlinked witness log: err %v, want errNotRegularEvidence", err)
	}
	if got := witnessLastError(root); got != "" {
		t.Fatalf("symlinked witness log was imported: %q", got)
	}
}

// TestAHI620WitnessCauseReportsSaveFailure: the diagnostic does not claim an unsaved frame was saved.
func TestAHI620WitnessCauseReportsSaveFailure(t *testing.T) {
	shortWitnessTimeout(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "failed-frame.txt"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := waitFrame(context.Background(), root, "sidebar", []string{"Corvint"}, nil, 0)
	if err == nil {
		t.Fatal("waitFrame succeeded without a frame")
	}
	got := err.Error()
	if !strings.Contains(got, "frame not saved to "+root+"/failed-frame.txt: ") {
		t.Errorf("error %q does not report the save failure", got)
	}
	if strings.Contains(got, "frame saved to") {
		t.Errorf("error %q claims the frame was saved", got)
	}
}
