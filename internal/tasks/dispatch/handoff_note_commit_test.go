package dispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestERGV0026_HandoffNoteCommit: a note without a commit keeps its earlier
// bytes (no commit member), a full commit id round-trips, and anything else
// in the commit member is refused (ERG-V0-026).
func TestERGV0026_HandoffNoteCommit(t *testing.T) {
	write := func(n *HandoffNote) string {
		p := filepath.Join(t.TempDir(), "note.json")
		if err := os.WriteFile(p, n.encode(), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	base := HandoffNote{Profile: HandoffNoteProfile, Worker: "w-1", Ticket: "T-1", Role: "implement", At: "2026-10-21T00:00:00Z", ContextTokens: "0", Messages: []string{}, Files: []NoteFile{}}
	if bytes.Contains(base.encode(), []byte(`"commit"`)) {
		t.Fatalf("a note without a commit gained a member: %s", base.encode())
	}
	full := base
	full.Commit = strings.Repeat("0a", 20)
	if n, err := ReadHandoffNote(write(&full)); err != nil || n.Commit != full.Commit {
		t.Fatalf("full commit: %v %+v", err, n)
	}
	short := base
	short.Commit = "0a0a0a0"
	if _, err := ReadHandoffNote(write(&short)); err == nil || !strings.Contains(err.Error(), "full commit id") {
		t.Fatalf("short commit accepted: %v", err)
	}
}
