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
	// DCW-V0-960: nor without an intent manifest.
	if options.IntentsFile == "" {
		options.IntentsFile = writeInput(t, "#no-intent-declared\n")
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
		{
			name:    "unsorted intents manifest",
			plan:    "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n",
			intents: "docs/specs/b.md\ndocs/specs/a.md\n",
			// DCW-V0-387: the fix names the first out-of-order line and the sort rule.
			want: []string{"FAIL dogfood-inputs-precheck ocm-aggregate missing-intent-scope input=--intents ", "prepare dogfood-inputs-precheck NOT_PRODUCED missing-intent-scope",
				`line 2 "docs/specs/a.md" does not sort after line 1 "docs/specs/b.md"`, "LC_ALL=C sort -u"},
		},
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

// TestDCWV0960InputsCheckRefusesAbsentIntentManifest (V1-3047): without an
// intent manifest the pre-flight refuses before prepare, naming --intents and
// the no-intent declaration, and with the outcome refusal when both apply;
// the full pass's ocm-aggregate would refuse missing-intent-scope only after
// the bind commit.
func TestDCWV0960InputsCheckRefusesAbsentIntentManifest(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	for _, tc := range []struct {
		name    string
		options ChangeOptions
		outcome bool
	}{
		{"intents only", ChangeOptions{Verify: "go version", Outcome: "passed"}, false},
		{"intents and outcome", ChangeOptions{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := tc.options
			options.Root, options.Base, options.Citations = root, base, plan
			calls := 0
			options.Steps = Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
				calls++
				return 1
			}}
			var stdout, stderr bytes.Buffer
			code, err := FinishLane(context.Background(), FinishLaneOptions{Change: options, Preflight: true}, &stdout, &stderr)
			out := stderr.String()
			if err != nil || code != 2 || calls != 0 || !strings.HasPrefix(out, "dogfood-inputs-check: REFUSE intent-scope-not-provided\n") ||
				!strings.Contains(out, "  fix: pass --intents FILE (or DOGFOOD_INTENTS_FILE; make takes INTENTS=<intents.txt>)") ||
				!strings.Contains(out, "#no-intent-declared") ||
				strings.Contains(out, "outcome-input-not-provided") != tc.outcome {
				t.Fatalf("code=%d err=%v calls=%d stderr:\n%s", code, err, calls, out)
			}
		})
	}
}

// refusalLines runs finish-lane, as the pre-flight or the closeout's full
// run, and returns its REFUSE and fix lines with the run's prefix removed, and
// how many steps ran.
func refusalLines(t *testing.T, options ChangeOptions, preflight bool) ([]string, int) {
	t.Helper()
	calls := 0
	options.Steps = Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), FinishLaneOptions{Change: options, Preflight: preflight}, &stdout, &stderr)
	if err != nil || code != 2 {
		t.Fatalf("preflight=%t: code=%d err=%v\n%s", preflight, code, err, &stderr)
	}
	lines := []string{}
	for _, line := range strings.Split(stderr.String(), "\n") {
		line = strings.NewReplacer("dogfood-inputs-check: ", "PREFIX: ", "dogfood-finish-lane: ", "PREFIX: ", "dogfood-change: ", "PREFIX: ").Replace(line)
		if strings.Contains(line, "REFUSE ") || strings.HasPrefix(line, "  fix: ") {
			lines = append(lines, strings.ReplaceAll(line, "corvint dogfood inputs-check", "corvint dogfood finish-lane"))
		}
	}
	return lines, calls
}

// TestDCWV0965InputsCheckRefusesWhatCloseoutRefuses (V1-3047): an intent
// manifest over the OCM aggregate obligation bound and each verify line the
// closeout refuses as unsupported-verify-syntax are refused by the pre-flight
// before any step, with the closeout run's own refusal code and fix line.
func TestDCWV0965InputsCheckRefusesWhatCloseoutRefuses(t *testing.T) {
	defer func(bound int) { aggregateObligationBound = bound }(aggregateObligationBound)
	aggregateObligationBound = 256
	root, base, manifest := obligationRepository(t, "docs/specs", 200, 40, 30)
	plan := writeInput(t, "1\tunknown\tinsufficient-evidence\tintents only\n")
	cases := []struct {
		name, code string
		options    ChangeOptions
	}{
		{"aggregate bound", "REFUSE too-many-obligations", ChangeOptions{IntentsFile: manifest, Verify: "go version"}},
		{"anchored -run name", "REFUSE unsupported-verify-syntax", ChangeOptions{IntentsFile: manifest, VerifyFile: writeInput(t, "go test -run ^TestA$ ./pre\n")}},
		{"quoted anchored -run name", "REFUSE unsupported-verify-syntax", ChangeOptions{IntentsFile: manifest, VerifyFile: writeInput(t, "go test -run '^TestA$' ./pre\n")}},
		{"unquoted alternation", "REFUSE unsupported-verify-syntax", ChangeOptions{IntentsFile: manifest, VerifyFile: writeInput(t, "go test -run TestA|TestB ./pre\n")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := tc.options
			options.Root, options.Base, options.Citations, options.Outcome = root, base, plan, "passed"
			preflight, preflightCalls := refusalLines(t, options, true)
			closeout, _ := refusalLines(t, options, false)
			if preflightCalls != 0 || len(preflight) < 2 || !strings.Contains(preflight[0], tc.code) || strings.Join(preflight, "\n") != strings.Join(closeout, "\n") {
				t.Fatalf("pre-flight (%d steps):\n%s\ncloseout:\n%s", preflightCalls, strings.Join(preflight, "\n"), strings.Join(closeout, "\n"))
			}
		})
	}
}
