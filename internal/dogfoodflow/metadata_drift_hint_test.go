package dogfoodflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDCWV0729RecordIndexFailedNamesMetadataDrift (V1-0683): when the
// recorder's kept stderr names gitstatus metadata drift, the record-index-failed
// fix names concurrent Git activity instead of uncommitted changes; without
// that text the earlier advice stays.
func TestDCWV0729RecordIndexFailedNamesMetadataDrift(t *testing.T) {
	evidence := t.TempDir()
	row := step{"local-outcome", "NOT_PRODUCED", "record-index-failed"}
	c := &change{flow: flow{ctx: context.Background(), root: t.TempDir(), base: "abc"}, evidence: evidence}
	stderr := filepath.Join(evidence, "local-outcome.stderr")
	if err := os.WriteFile(stderr, []byte("Git status cannot safely observe repository metadata: repository metadata changed during observation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hint := c.fixHint(row)
	for _, want := range []string{"concurrent Git activity", "not an uncommitted change", "rerun corvint dogfood change abc", "(DCW-V0-729)"} {
		if !strings.Contains(hint, want) {
			t.Errorf("drift hint lacks %q: %q", want, hint)
		}
	}
	if err := os.WriteFile(stderr, []byte("record index: worktree is dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hint := c.fixHint(row); strings.Contains(hint, "DCW-V0-729") || !strings.Contains(hint, "the worktree has uncommitted changes") {
		t.Fatalf("non-drift hint: %q", hint)
	}
}
