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

// DCW-V0-335 (V1-2541): finish-lane's prepare pass cites every plan row
// against the map it prepared, into a removed probe output, so a row whose
// line text recurs in the changed file stops the run at prepare with cem
// cite's detail (the copies and a wider unique span) before any bind pass;
// the corrected plan passes prepare and reaches bind.
func TestFinishLaneDCWV0335PrepareRefusesUnstableCiteSpanBeforeBind(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	content := "package resume\n\nfunc a() error {\n\tif x {\n\t\treturn nil\n\t}\n\treturn errA\n}\n\nfunc b() error {\n\tif y {\n\t\treturn nil\n\t}\n\treturn errB\n}\n\n// end\n"
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "resume.go"), []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write(content)
	testGit(t, root, "add", "resume.go")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	write(strings.Replace(content, "// end\n", "// END\n", 1))
	testGit(t, root, "commit", "-q", "-am", "change")
	for _, tc := range []struct {
		name, lines string
		refused     bool
	}{
		{"duplicated line", "12:12", true},
		{"wider unique span", "11:12", false},
	} {
		plan := filepath.Join(t.TempDir(), "cites.tsv")
		if err := os.WriteFile(plan, []byte("1\tresume.go\t"+tc.lines+"\timplementation\n"), 0o666); err != nil {
			t.Fatal(err)
		}
		var labels []string
		steps := Runner{Path: "corvint", Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
			labels = append(labels, strings.Join(args, " "))
			if len(args) > 0 && args[0] == "cem" {
				return cemcli.Run(ctx, root, args[1:], stdout, stderr)
			}
			return 1
		}}
		var stdout, stderr bytes.Buffer
		code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr)
		if err != nil || code == 0 {
			t.Fatalf("%s: code=%d err=%v stdout=%q stderr=%q", tc.name, code, err, stdout.String(), stderr.String())
		}
		bound := false
		for _, label := range labels {
			bound = bound || (strings.HasPrefix(label, "cem cite ") && !strings.Contains(label, " --output .corvint/.cem-citations-probe."))
		}
		if !tc.refused {
			if !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare PASS hunks=1") || !bound {
				t.Fatalf("%s: prepare did not pass to bind; stdout=%q stderr=%q", tc.name, stdout.String(), stderr.String())
			}
			continue
		}
		for _, want := range []string{
			"dogfood-change: FAIL cem-cite-precheck refused plan rows=1",
			`row 1 hunk=1 path="resume.go" lines=12:12 cite-span-not-stable: `,
			"occurs 2 times in the changed file (at changed-file lines 5:5, 12:12)",
			"recovery: cite the wider unique span --lines 11:12",
		} {
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("%s: stderr lacks %q:\n%s", tc.name, want, stderr.String())
			}
		}
		if !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare FAIL exit=1") || !strings.Contains(stdout.String(), "prepare cem-cite-precheck NOT_PRODUCED cite-span-not-stable") || bound {
			t.Fatalf("%s: stdout=%q bound=%v labels=%q", tc.name, stdout.String(), bound, labels)
		}
		matches, _ := filepath.Glob(filepath.Join(root, ".corvint", ".cem-citations-probe.*"))
		if len(matches) != 0 {
			t.Fatalf("%s: probe output left: %q", tc.name, matches)
		}
		_ = os.Remove(filepath.Join(root, finishLaneSidecar))
	}
}
