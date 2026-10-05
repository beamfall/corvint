package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0086_AttemptWorktreePathIsPathText pins the corrected bound of the
// attempt's `worktreePath` (V1-0772): it is an absolute PathText of at most
// 4096 bytes, not a 128-byte Identifier. A supervised stage worktree composes
// an operator work root with an Identifier program id, so a path longer than
// 128 bytes decodes and round-trips exactly, while an empty, relative,
// hostile or over-4096-byte path is still refused with its code.
func TestCALV0086_AttemptWorktreePathIsPathText(t *testing.T) {
	a := accountingAttempt()
	long := "/" + strings.Repeat("w", 200) + "/program/1/1/implement-1"
	a.WorktreePath = &long
	raw, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodeAttempt(raw)
	if err != nil || b.WorktreePath == nil || *b.WorktreePath != long {
		t.Fatalf("a %d-byte absolute worktree path did not decode: %v", len(long), err)
	}
	if again, err := b.Encode(); err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("worktree path bytes changed: %v", err)
	}
	quoted := []byte(`"worktreePath":"` + long + `"`)
	if !bytes.Contains(raw, quoted) {
		t.Fatal("encoding does not carry the worktree path")
	}
	for name, c := range map[string]struct{ value, code string }{
		"empty":    {"", wire.CodeMalformed},
		"relative": {"worktree", wire.CodeMalformed},
		"hostile":  {`/work\ttree`, wire.CodeMalformed},
		"over":     {"/" + strings.Repeat("w", wire.MaxPathTextBytes), wire.CodeLimitExceeded},
	} {
		bad := bytes.Replace(raw, quoted, []byte(`"worktreePath":"`+c.value+`"`), 1)
		if _, err := DecodeAttempt(bad); wire.CodeOf(err) != c.code {
			t.Fatalf("%s worktree path: want %s, got %v", name, c.code, err)
		}
	}
	at := "/" + strings.Repeat("w", wire.MaxPathTextBytes-1)
	if _, err := DecodeAttempt(bytes.Replace(raw, quoted, []byte(`"worktreePath":"`+at+`"`), 1)); err != nil {
		t.Fatalf("a %d-byte worktree path was refused: %v", len(at), err)
	}
}
