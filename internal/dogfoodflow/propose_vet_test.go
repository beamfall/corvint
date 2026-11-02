package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0840VetsCarriedRows pins V1-3844, V1-3595 and
// V1-3673: a refresh plan carries a row citing a line the change itself
// removes (pre.go 10, "first line 6") and a row whose text occurs more than
// once after the change (pre.go 18, "}"). Before DCW-V0-840 both were carried
// verbatim and the bind's cite precheck refused them cite-span-not-stable;
// now each is re-pinned to the nearest surviving line with the refusal named
// (DCW-V0-980), a stable row is kept, and the printed draft passes the real
// cem cite precheck.
func TestProposeCitationsDCWV0840VetsCarriedRows(t *testing.T) {
	root, base := preflightRepository(t, nil)
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 2 {
		t.Fatalf("draft rows = %q\n%s", rows, stderr.String())
	}
	// DCW-V0-980: each refused row is re-pinned to the nearest line the
	// change leaves intact: 9 ("first line 5") and 17 ("return 1").
	cases := [][3]string{
		{"pre.go\t10:10\timplementation", "overlap BASE lines 10:10 that hunk ", "pre.go\t9:9\timplementation"},
		{"pre.go\t18:18\timplementation", "after the change (it must occur exactly once in the changed file)", "pre.go\t17:17\timplementation"},
	}
	for _, kept := range []bool{false, true} {
		plan := []string{}
		for index, row := range rows {
			id, _, _ := strings.Cut(row, "\t")
			cite := cases[index][0]
			if kept && index == 1 {
				cite = "pre.go\t20:20\timplementation"
			}
			plan = append(plan, id+"\t"+cite)
		}
		planPath := writeInput(t, strings.Join(plan, "\n")+"\n")
		if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: planPath}); code == 0 || !strings.Contains(out+errText, "cite-span-not-stable") {
			t.Fatalf("the hand-kept plan was not refused by the precheck: code=%d\n%s\n%s", code, out, errText)
		}
		stdout.Reset()
		stderr.Reset()
		if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: planPath}, &stdout, &stderr); code != 0 || err != nil {
			t.Fatalf("ProposeCitations --refresh = %d, %v\n%s", code, err, stderr.String())
		}
		refreshed := textLines(stdout.Bytes())
		for index, line := range plan {
			id, _, _ := strings.Cut(line, "\t")
			if kept && index == 1 {
				if !containsLine(refreshed, line) {
					t.Errorf("the stable row %q was not kept:\n%s", line, stdout.String())
				}
				continue
			}
			if containsLine(refreshed, line) || !containsLine(refreshed, id+"\t"+cases[index][2]) {
				t.Errorf("the refused row %q was not re-pinned to %q:\n%s", line, cases[index][2], stdout.String())
			}
			if output := stdout.String() + stderr.String(); !strings.Contains(output, cases[index][1]) || !strings.Contains(output, "DCW-V0-980") {
				t.Errorf("the re-pin note does not name %q:\n%s", cases[index][1], output)
			}
		}
		next := filepath.Join(filepath.Dir(planPath), "plan.new")
		if err := os.WriteFile(next, stdout.Bytes(), 0o666); err != nil {
			t.Fatal(err)
		}
		if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: next}); code != 0 {
			t.Fatalf("the vetted draft was refused: code=%d\nstdout=%s\nstderr=%s\nplan=%s", code, out, errText, stdout.String())
		}
	}
}
