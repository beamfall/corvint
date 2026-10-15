package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// DCW-V0-063 (V1-1349, V1-1418): a plan row whose BASE span lies outside
// every hunk, but whose text recurs in an added region, is refused with the
// unchanged cite-span-not-stable code, and its fix line states that cause; a
// span a hunk edits keeps the DCW-V0-014 fix line.
func TestChangeDCWV0063NamesUnalignedCiteSpanCause(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	block := func(name string) string {
		return "if " + name + "; then\n  run " + name + "\nelse\n  skip\nfi\n"
	}
	testGit(t, root, "init", "-q")
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "gate.sh"), []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("# head\n" + block("a") + block("b") + "# tail\n")
	testGit(t, root, "add", "gate.sh")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	write("# HEAD\n" + block("a") + block("b") + "# tail\n" + block("c"))
	testGit(t, root, "commit", "-q", "-am", "change")
	target := testGit(t, root, "rev-parse", "HEAD")
	steps := Runner{Path: "corvint", Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		return cemcli.Run(ctx, root, args[1:], stdout, stderr)
	}}
	var prepareErr bytes.Buffer
	if code := steps.Run(context.Background(), root, []string{"cem", "prepare", "--base", base, "--target", target}, io.Discard, &prepareErr); code != 0 {
		t.Fatalf("cem prepare exit=%d: %s", code, prepareErr.String())
	}
	for _, tc := range []struct {
		name, lines, want string
		unaligned         bool
	}{
		{"outside every hunk", "9:11", "plan row 1 cites BASE lines outside every hunk, which git's line alignment leaves unchanged, but their text occurs more than once in the changed file", true},
		{"edited by a hunk", "1:1", "plan row 1 cites BASE lines that this change edits or deletes", false},
	} {
		evidence := t.TempDir()
		c := &change{flow: flow{ctx: context.Background(), root: root}, evidence: evidence, runTmp: t.TempDir(), citationCount: 1, options: ChangeOptions{Steps: steps}}
		status, reason := c.cite([]byte("1\tgate.sh\t"+tc.lines+"\timplementation\n"), filepath.Join(evidence, "cem-cite.json"))
		if status != "NOT_PRODUCED" || reason != "cite-span-not-stable" || c.citeSpanUnaligned != tc.unaligned {
			t.Fatalf("%s: status=%s reason=%s unaligned=%v", tc.name, status, reason, c.citeSpanUnaligned)
		}
		if hint := c.fixHint(step{name: "cem-cite", reason: reason}); !strings.Contains(hint, tc.want) {
			t.Fatalf("%s: fix line %q, want %q", tc.name, hint, tc.want)
		}
	}
}
