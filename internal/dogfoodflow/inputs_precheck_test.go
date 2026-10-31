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

// preflightRepository commits a base and a change editing two separate
// lines of one Go file, so the prepared map holds two hunks; extra adds more
// paths to the change.
func preflightRepository(t *testing.T, extra map[string]string) (root, base string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	content := "package pre\n\n// First is one.\nfunc First() int {\n" + numbered("\t// first", 12) + "\treturn 1\n}\n\n// Second is two.\nfunc Second() int {\n" + numbered("\t// second", 12) + "\treturn 2\n}\n"
	writeTree(t, root, map[string]string{"pre.go": content})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base = testGit(t, root, "rev-parse", "HEAD")
	changed := strings.Replace(content, "\t// first line 6\n", "\t// first line 6 changed\n", 1)
	changed = strings.Replace(changed, "\t// second line 6\n", "\t// second line 6 changed\n", 1)
	files := map[string]string{"pre.go": changed}
	for name, text := range extra {
		files[name] = text
	}
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	return root, base
}

// preflightRun runs the inputs pre-flight with real cem prepare and cite and
// returns its exit code, output and every step's argv.
func preflightRun(t *testing.T, root string, options ChangeOptions) (int, string, string, []string) {
	t.Helper()
	var labels []string
	options.Root = root
	// DCW-V0-800: the pre-flight refuses before prepare without both outcome
	// inputs, so the cases that test later refusals supply them.
	if options.Verify == "" && options.VerifyFile == "" {
		options.Verify = "go version"
	}
	if options.Outcome == "" {
		options.Outcome = "passed"
	}
	options.Steps = Runner{Path: "corvint", Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		labels = append(labels, strings.Join(args, " "))
		if len(args) > 0 && args[0] == "cem" {
			return cemcli.Run(ctx, root, args[1:], stdout, stderr)
		}
		return 1
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), FinishLaneOptions{Change: options, Preflight: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("FinishLane: %v\nstdout=%s\nstderr=%s", err, &stdout, &stderr)
	}
	return code, stdout.String(), stderr.String(), labels
}

func writeInput(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

// assertNothingBound checks that the pre-flight left HEAD, the sidecar and
// the probe outputs as they were and never ran a binding cite.
func assertNothingBound(t *testing.T, root, head string, labels []string) {
	t.Helper()
	if got := testGit(t, root, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved from %s to %s", head, got)
	}
	if _, err := os.Lstat(filepath.Join(root, finishLaneSidecar)); !os.IsNotExist(err) {
		t.Errorf("the prepared map was left at %s: %v", finishLaneSidecar, err)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".corvint", ".cem-citations*")); len(matches) != 0 {
		t.Errorf("citation stage or probe left: %q", matches)
	}
	for _, label := range labels {
		if strings.HasPrefix(label, "cem cite ") && !strings.Contains(label, " --output .corvint/.cem-citations-probe.") {
			t.Errorf("a binding cite ran: %s", label)
		}
	}
}

// TestDCWV0380InputsCheckRefusesVerifySyntaxBeforeAnyStep (V1-2970): a `|` in
// the verify file is refused by name and line before any step runs. The
// alternation holds a character class, a form DCW-V0-801 still refuses.
func TestDCWV0380InputsCheckRefusesVerifySyntaxBeforeAnyStep(t *testing.T) {
	root, base := preflightRepository(t, nil)
	head := testGit(t, root, "rev-parse", "HEAD")
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	verify := writeInput(t, "go test -run TestA ./pre\ngo test -run '^(TestA|Test[B])$' ./pre\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: verify})
	if code != 2 || len(labels) != 0 {
		t.Fatalf("code=%d steps=%q stdout=%q stderr=%q", code, labels, stdout, stderr)
	}
	for _, want := range []string{
		"dogfood-inputs-check: REFUSE unsupported-verify-syntax\n",
		"  input: --verify " + verify + " line 2\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	assertNothingBound(t, root, head, labels)
}

// TestDCWV0379InputsCheckRefusesEachPlanAndManifestDefect (V1-2970): each
// defect the bind would refuse stops the pre-flight at prepare, naming the
// bind step's reason and the offending input, with nothing bound.
func TestDCWV0379InputsCheckRefusesEachPlanAndManifestDefect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		extra   map[string]string
		plan    string
		intents string
		want    []string
	}{
		{
			name: "uniform plan",
			plan: "1\tpre.go\t3:3\timplementation\n2\tpre.go\t3:3\timplementation\n",
			want: []string{"FAIL dogfood-inputs-precheck cem-cite citation-plan-uniform input=", " rows=all hunks=2 span=", "prepare dogfood-inputs-precheck NOT_PRODUCED citation-plan-uniform"},
		},
		{
			// DCW-V0-402 (V1-3226): an unregistered relation refuses the whole
			// plan before any cite and names the full allowed list.
			name: "relation tests",
			plan: "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\ttests\n",
			want: []string{"FAIL dogfood-inputs-precheck cem-cite citation-plan-relation-unknown input=", ` row 2 relation="tests"`,
				"one of call-site, decision, dependency, implementation, incident, specification, test-claim;", "takes test-claim (tests is not a relation)",
				"prepare dogfood-inputs-precheck NOT_PRODUCED citation-plan-relation-unknown"},
		},
		{
			name: "ordinal past the map",
			plan: "1\tpre.go\t3:3\timplementation\n3\tpre.go\t20:20\timplementation\n",
			want: []string{"FAIL dogfood-inputs-precheck cem-cite citation-plan-map-mismatch input=", " row 2 ordinal 3 is past the prepared map's 2 hunks", "prepare dogfood-inputs-precheck NOT_PRODUCED citation-plan-map-mismatch"},
		},
		{
			name: "cite span the change edits",
			plan: "1\tpre.go\t10:10\timplementation\n2\tpre.go\t3:3\timplementation\n",
			want: []string{"FAIL cem-cite-precheck refused plan rows=1", `row 1 hunk=1 path="pre.go" lines=10:10 cite-span-not-stable`, "prepare cem-cite-precheck NOT_PRODUCED cite-span-not-stable"},
		},
		{
			name:  "empty file in the diff",
			extra: map[string]string{"empty.txt": ""},
			plan:  "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n",
			want:  []string{"prepare cem-prepare NOT_PRODUCED metadata-only-change-unsupported"},
		},
		// An unsorted intents manifest no longer reaches prepare: it is
		// refused before any step (DCW-V0-882,
		// TestDCWV0882InputsCheckRefusesUnsortedIntentsFirst).
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base := preflightRepository(t, tc.extra)
			head := testGit(t, root, "rev-parse", "HEAD")
			options := ChangeOptions{Base: base, Citations: writeInput(t, tc.plan)}
			if tc.intents != "" {
				options.IntentsFile = writeInput(t, tc.intents)
			}
			code, stdout, stderr, labels := preflightRun(t, root, options)
			if code == 0 || !strings.Contains(stdout, "dogfood-inputs-check: prepare FAIL exit=") || strings.Contains(stdout, "inputs-check PASS") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stdout+stderr, want) {
					t.Errorf("output lacks %q:\nstdout=%s\nstderr=%s", want, stdout, stderr)
				}
			}
			assertNothingBound(t, root, head, labels)
		})
	}
}

// TestDCWV0380InputsCheckPassesBindsNothing (V1-2970): valid inputs pass the
// pre-flight, which then removes the prepared map and commits nothing.
func TestDCWV0380InputsCheckPassesBindsNothing(t *testing.T) {
	root, base := preflightRepository(t, nil)
	head := testGit(t, root, "rev-parse", "HEAD")
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	verify := writeInput(t, "go test -run TestA ./pre\ngo test -run TestB ./pre\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: verify})
	want := "dogfood-inputs-check: prepare PASS hunks=2\ndogfood-inputs-check: inputs-check PASS nothing was bound or committed\n"
	if code != 0 || stdout != want {
		t.Fatalf("code=%d stdout=%q want %q stderr=%q", code, stdout, want, stderr)
	}
	assertNothingBound(t, root, head, labels)
}
