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

// DCW-V0-063 (V1-1349, V1-1418): a plan row whose BASE span no hunk edits
// (git -U0 leaves it unchanged), but whose text recurs in an added region, is refused with the
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
		{"edited by no hunk", "9:11", "plan row 1 cites BASE lines that no hunk edits (at most hunk context), which git's line alignment leaves unchanged, but their text occurs more than once in the changed file", true},
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

// DCW-V0-114 (V1-1724): one dogfood-change pass names every refused cite row
// of the plan, not only the first; the probes leave the prepared map and the
// repository unchanged, and unknown rows are not probed.
func TestChangeDCWV0114ReportsEveryRefusedCiteRow(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "gate.sh"), []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("# head\nrun a\nrun b\n# tail\n")
	testGit(t, root, "add", "gate.sh")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	write("# HEAD\nrun a\nrun b\n# tail\nrun c\n")
	testGit(t, root, "commit", "-q", "-am", "change")
	target := testGit(t, root, "rev-parse", "HEAD")
	steps := Runner{Path: "corvint", Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		return cemcli.Run(ctx, root, args[1:], stdout, stderr)
	}}
	var prepareErr bytes.Buffer
	if code := steps.Run(context.Background(), root, []string{"cem", "prepare", "--base", base, "--target", target}, io.Discard, &prepareErr); code != 0 {
		t.Fatalf("cem prepare exit=%d: %s", code, prepareErr.String())
	}
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	before, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	plan := "1\tgate.sh\t1:1\timplementation\n" + // edited BASE line: refused
		"1\tgate.sh\t2:2\timplementation\n" + // unchanged unique line: admitted
		"1\tunknown\tinsufficient-evidence\tnot probed\n" +
		"1\tgate.sh\t1:1\timplementation\n" // refused again
	evidence := t.TempDir()
	var report bytes.Buffer
	c := &change{flow: flow{ctx: context.Background(), root: root, stderr: &report, base: base, target: target}, evidence: evidence, runTmp: t.TempDir(), citationCount: 4, citationStage: ".corvint/.cem-citations.test.json", options: ChangeOptions{Steps: steps}}
	status, reason := c.cite([]byte(plan), filepath.Join(evidence, "cem-cite.json"))
	if status != "NOT_PRODUCED" || reason != "cite-span-not-stable" || c.citationFailedRow != 1 {
		t.Fatalf("status=%s reason=%s row=%d", status, reason, c.citationFailedRow)
	}
	want := []string{
		`row 1 hunk=1 path="gate.sh" lines=1:1 cite-span-not-stable`,
		`row 4 hunk=1 path="gate.sh" lines=1:1 cite-span-not-stable`,
	}
	if strings.Join(c.citationRefusals, "\n") != strings.Join(want, "\n") {
		t.Fatalf("refusals:\n%s\nwant:\n%s", strings.Join(c.citationRefusals, "\n"), strings.Join(want, "\n"))
	}
	after, err := os.ReadFile(mapPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("prepared map changed by the probes (err=%v)", err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(root, ".corvint", ".cem-citations*")); len(leftovers) != 0 {
		t.Fatalf("probe or stage outputs left behind: %v", leftovers)
	}
	c.rows = []step{{name: "cem-cite", status: "NOT_PRODUCED", reason: reason}}
	c.reportFailures()
	for _, line := range append([]string{"refused plan rows=2"}, want...) {
		if !strings.Contains(report.String(), line) {
			t.Fatalf("report lacks %q:\n%s", line, report.String())
		}
	}
}
