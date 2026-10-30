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

// TestERGV0028_CheckpointCommit: the note's commit is the attempt's
// worktree HEAD when it records a worktree, else its branch in the work
// root; no branch or worktree, a resolver error, or anything but a full
// commit id names no commit (ERG-V0-028).
func TestERGV0028_CheckpointCommit(t *testing.T) {
	full := strings.Repeat("0b", 20)
	var asked []string
	resolve := func(answer string, err error) func(dir, rev string) (string, error) {
		return func(dir, rev string) (string, error) {
			asked = append(asked, dir+"@"+rev)
			return answer, err
		}
	}
	for _, tc := range []struct {
		name     string
		a        Attempt
		answer   string
		err      error
		want, at string
	}{
		{"worktree", Attempt{Worktree: "/wt", Branch: "b"}, full, nil, full, "/wt@HEAD"},
		{"branch", Attempt{Branch: "b"}, full, nil, full, "/root@refs/heads/b"},
		{"neither", Attempt{}, full, nil, "", ""},
		{"short", Attempt{Branch: "b"}, "0b0b0b0", nil, "", "/root@refs/heads/b"},
		{"error", Attempt{Branch: "b"}, "", os.ErrNotExist, "", "/root@refs/heads/b"},
	} {
		asked = nil
		if got := CheckpointCommit(resolve(tc.answer, tc.err), "/root", tc.a); got != tc.want || strings.Join(asked, ",") != tc.at {
			t.Errorf("%s: commit %q asked %v", tc.name, got, asked)
		}
	}
	if got := CheckpointCommit(nil, "/root", Attempt{Branch: "b"}); got != "" {
		t.Errorf("nil resolver named %q", got)
	}
}
