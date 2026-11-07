package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDWCV0754AnchorSearchMatchesGitGrep (DWC-V0-754): the
// anchor search's -l listing and in-process match print exactly what the one
// git grep -z -n -o prints, for overlapping, nested and repeated patterns,
// several matches on a line, CRLF, a last line without LF, a binary file, a
// symbolic link and an excluded path; and with no match both report exit 1.
func TestProposeCitationsDWCV0754AnchorSearchMatchesGitGrep(t *testing.T) {
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
		p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &bytes.Buffer{}, base: base}, blobs: map[string][]byte{}, lines: map[string][][]byte{}, batchBlobs: true}
		defer p.closeBlobBatch()
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

// TestProposeCitationsDWCV0754LongLineStaysBoundedAndInterruptible (review):
// a long line with an overlapping match at every byte keeps only the matches
// still open, stops at the output bound as Git's output would, and a deadline
// passed during the scan stops it.
func TestProposeCitationsDWCV0754LongLineStaysBoundedAndInterruptible(t *testing.T) {
	data := []byte(strings.Repeat("a", 4<<20))
	matcher := newFixedMatcher([]string{"a", "aa", "aaa"})
	var out strings.Builder
	if matcher.records(&out, "f", data, 1<<20, nil) || out.Len() > 1<<20 {
		t.Fatalf("a 4 MiB line under a 1 MiB bound: wrote %d bytes and did not report the overflow", out.Len())
	}
	out.Reset()
	if !matcher.records(&out, "f", data[:64], 1<<20, nil) || out.String() != strings.Repeat("f\x001\x00aaa\n", 21)+"f\x001\x00a\n" {
		t.Fatalf("64 bytes of a: %q, want 21 leftmost-longest aaa and one a", out.String())
	}
	ticks := 0
	stopped := func() (stopped bool) {
		defer func() { stopped = recover() != nil }()
		matcher.records(&strings.Builder{}, "f", data, 64<<20, func() {
			if ticks++; ticks == 3 {
				panic("deadline")
			}
		})
		return false
	}()
	if !stopped || ticks != 3 {
		t.Fatalf("the scan did not stop at its third tick: stopped=%v ticks=%d", stopped, ticks)
	}
}
