package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0754AnchorSearchMatchesGitGrep (DCW-V0-754): the
// anchor search's -l listing and in-process match print exactly what the one
// git grep -z -n -o prints, for overlapping, nested and repeated patterns,
// several matches on a line, CRLF, a last line without LF, a binary file, a
// symbolic link and an excluded path; and with no match both report exit 1.
func TestProposeCitationsDCWV0754AnchorSearchMatchesGitGrep(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{
		"docs/specs/a-v0.md":  "# A\n\n- `ABC-V0-001`: names internal/x/x.go and x.go twice: x.go\r\n- `ABC-V0-002`: aaaa x.gox.go\nlast line names x.go",
		"docs/notes.md":       "nothing here\nABC-V0-001 without the definition marker\n- `ABC-V0-001`:- `ABC-V0-001`:\n",
		"internal/x/x.go":     "package x\n\n// see docs/specs/a-v0.md for x.go\n",
		"bin/data.bin":        "x.go\x00binary",
		"docs/build-log/b.md": "x.go is excluded\n",
	})
	if err := os.Symlink("x.go", filepath.Join(root, "internal/x/link.go")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	search := func(fast bool, patterns ...string) (string, int, bool) {
		t.Helper()
		previous, threshold := anchorGrepFastPath, anchorGrepFastPatterns
		anchorGrepFastPath, anchorGrepFastPatterns = fast, 1
		defer func() { anchorGrepFastPath, anchorGrepFastPatterns = previous, threshold }()
		p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &bytes.Buffer{}, base: base}, blobs: map[string][]byte{}, lines: map[string][][]byte{}}
		defer p.closeBlobs()
		arguments := []string{"grep", "-z", "-n", "-o", "-F", "-I"}
		for _, pattern := range patterns {
			arguments = append(arguments, "-e", pattern)
		}
		arguments = append(arguments, base, "--", ".", ":(exclude)docs/build-log/")
		return p.onlyMatching(arguments, patterns)
	}
	for index, patterns := range [][]string{
		{"x.go", "internal/x/x.go", "- `ABC-V0-001`:", "- `ABC-V0-002`:", "aa", "a", "x.gox"},
		{"x.go"},
		{"aa", "aaa"},
		{"no such text"},
	} {
		fastOut, fastStatus, fastBounded := search(true, patterns...)
		slowOut, slowStatus, slowBounded := search(false, patterns...)
		if fastOut != slowOut || fastStatus != slowStatus || fastBounded != slowBounded {
			t.Errorf("patterns %q:\nfast %d %v:\n%q\ngit %d %v:\n%q", patterns, fastStatus, fastBounded, fastOut, slowStatus, slowBounded, slowOut)
		}
		if index == 0 && strings.Count(slowOut, "\n") < 8 {
			t.Errorf("patterns %q: the fixture matches too little to compare:\n%q", patterns, slowOut)
		}
	}
}
