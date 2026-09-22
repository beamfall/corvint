package patch

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// FuzzParseBodyMatchesDeclaredRanges checks the frozen parser's accounting on
// arbitrary bytes: an accepted patch has 1..MaxHunks hunks, each hunk body
// consumes exactly its declared old and new counts and carries a changed line,
// and each content address is the SHA-256 of that hunk's raw body bytes.
func FuzzParseBodyMatchesDeclaredRanges(f *testing.F) {
	seeds, _ := filepath.Glob(filepath.Join("..", "..", "..", "interop", "cem-0.1", "patches", "*.patch"))
	for _, seed := range seeds {
		if data, err := os.ReadFile(seed); err == nil {
			f.Add(data)
		}
	}
	f.Add([]byte("--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		parsed, err := Parse(raw)
		if err != nil {
			return
		}
		if len(parsed.Hunks) == 0 || len(parsed.Hunks) > wire.MaxHunks {
			t.Fatalf("accepted patch has %d hunks", len(parsed.Hunks))
		}
		for _, hunk := range parsed.Hunks {
			checkHunkAccounting(t, hunk)
		}
	})
}

func checkHunkAccounting(t *testing.T, hunk *Hunk) {
	t.Helper()
	oldLines := map[byte]int64{' ': 1, '-': 1}
	newLines := map[byte]int64{' ': 1, '+': 1}
	var old, new, changed int64
	for _, line := range hunk.Body {
		old += oldLines[line.Prefix]
		new += newLines[line.Prefix]
		changed += 1 - oldLines[line.Prefix]*newLines[line.Prefix]
	}
	if old != hunk.OldRange.Count || new != hunk.NewRange.Count || changed == 0 {
		t.Fatalf("hunk %s body old=%d new=%d changed=%d, declared %+v %+v", hunk.ID, old, new, changed, hunk.OldRange, hunk.NewRange)
	}
	digest := sha256.Sum256(hunk.RawBody)
	if hex.EncodeToString(digest[:]) != hunk.ContentSha256 {
		t.Fatalf("hunk %s content address does not match its raw body", hunk.ID)
	}
}
