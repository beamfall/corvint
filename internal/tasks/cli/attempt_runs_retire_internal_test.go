//go:build darwin || linux

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0015_AttachOfARunRetiredMidReadIsMissing replays a finished run
// whose directory disappears after its record was read: attach refuses
// MISSING_EVIDENCE as for any absent run. A run whose record stays while its
// kept result is gone is still MALFORMED.
func TestATRV0015_AttachOfARunRetiredMidReadIsMissing(t *testing.T) {
	sum, status := strings.Repeat("0", 64), 0
	rec := &runRecord{State: runFinished, ResultSha256: &sum, ExitStatus: &status}
	replay := func(dir string) *wire.Result {
		t.Helper()
		var out bytes.Buffer
		replayRun(Env{Stdout: &out}, dir, rec)
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("result %q: %v", out.String(), err)
		}
		return res
	}
	gone := filepath.Join(t.TempDir(), "00000000000000aa")
	if res := replay(gone); res.Outcome != wire.OutcomeRefused || len(res.Codes) != 1 || res.Codes[0] != wire.CodeMissingEvidence {
		t.Fatalf("retired run: %s %v", res.Outcome, res.Codes)
	}
	kept := filepath.Join(t.TempDir(), "00000000000000bb")
	if err := os.MkdirAll(kept, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, runRecordName), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if res := replay(kept); len(res.Codes) != 1 || res.Codes[0] != wire.CodeMalformed {
		t.Fatalf("a record without its result: %s %v", res.Outcome, res.Codes)
	}
}
