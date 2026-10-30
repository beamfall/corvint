package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/witness"
)

// AGW-V0-013: --range NAME=BASE..HEAD is accepted once each for app and tests
// with an optional --range-root per name, and every incomplete or ambiguous
// pairing is refused before any repository is read.
func TestParseWitnessRanges(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--range", "app=a1..a2", "--range=tests=t1..t2", "--range-root", "app=/x/app", "--evidence-in-repo", "run.json", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if options.ranges[witness.AppRange] != (witnessRange{base: "a1", head: "a2"}) || options.ranges[witness.TestsRange] != (witnessRange{base: "t1", head: "t2"}) || options.rangeRoots[witness.AppRange] != "/x/app" || options.base != "" {
		t.Fatalf("options = %+v", options)
	}
	for name, testCase := range map[string]struct {
		arguments []string
		reason    string
	}{
		"one range":          {[]string{"--range", "app=a..b"}, "requires both"},
		"unknown name":       {[]string{"--range", "lib=a..b", "--range", "tests=a..b"}, "the names are app and tests"},
		"repeated":           {[]string{"--range", "app=a..b", "--range", "app=c..d"}, "--range app is given twice"},
		"no head":            {[]string{"--range", "app=a..", "--range", "tests=a..b"}, "takes BASE..HEAD"},
		"no dots":            {[]string{"--range", "app=a", "--range", "tests=a..b"}, "takes BASE..HEAD"},
		"three dots":         {[]string{"--range", "app=a...b", "--range", "tests=a..b"}, "takes BASE..HEAD"},
		"no name":            {[]string{"--range", "a..b", "--range", "tests=a..b"}, "takes NAME=VALUE"},
		"with base":          {[]string{"--base", "x", "--range", "app=a..b", "--range", "tests=a..b"}, "replaces --base"},
		"with cem":           {[]string{"--cem", "x.json", "--range", "app=a..b", "--range", "tests=a..b"}, "name one repository"},
		"root without range": {[]string{"--base", "x", "--range-root", "app=/x"}, "--range-root requires"},
		"repeated root":      {[]string{"--range-root", "app=/x", "--range-root", "app=/y", "--range", "app=a..b", "--range", "tests=a..b"}, "--range-root app is given twice"},
		"missing value":      {[]string{"--range"}, "missing value for --range"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWitnessOptions(testCase.arguments); err == nil || !strings.Contains(err.Error(), testCase.reason) {
				t.Fatalf("%v: err = %v, want %q", testCase.arguments, err, testCase.reason)
			}
		})
	}
}

func revParse(t *testing.T, root string, revisions ...string) []string {
	t.Helper()
	command := exec.Command("git", append([]string{"rev-parse"}, revisions...)...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(output))
}

// AGW-V0-013: an app range in one checkout and a tests range in another
// compile as one paired report that names both ranges, sums both summaries,
// and under --check exits 1 while any obligation of either range is open.
func TestRunWitnessPairsTwoRanges(t *testing.T) {
	t.Parallel()
	app, tests := witnessCLIRange(t), witnessCLIRange(t)
	appCommits, testsCommits := revParse(t, app, "HEAD~1", "HEAD"), revParse(t, tests, "HEAD~1", "HEAD")
	arguments := []string{"--range", "app=HEAD~1..HEAD", "--range-root", "app=" + app, "--range", "tests=HEAD~1..HEAD"}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := runWitness(t.Context(), tests, append(arguments, "--json"), stdout, stderr); code != 0 {
		t.Fatalf("exit = %d; stderr %s", code, stderr.String())
	}
	var report witness.PairedReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	if report.Profile != witness.PairedProfile || len(report.Ranges) != 2 || report.Ranges[0].Name != "app" || report.Ranges[1].Name != "tests" {
		t.Fatalf("report = %+v", report)
	}
	for position, commits := range [][]string{appCommits, testsCommits} {
		got := report.Ranges[position].Report.Range
		if got.Base != commits[0] || got.Head != commits[1] {
			t.Fatalf("range %s = %+v, want %v", report.Ranges[position].Name, got, commits)
		}
	}
	if opened := report.Ranges[0].Report.Summary.Opened + report.Ranges[1].Report.Summary.Opened; report.Summary.Opened != opened || opened == 0 {
		t.Fatalf("paired summary %+v does not sum both ranges (%d)", report.Summary, opened)
	}
	stdout.Reset()
	if code := runWitness(t.Context(), tests, append(arguments, "--check"), stdout, stderr); code != 1 {
		t.Fatalf("--check exit = %d, want 1; stderr %s", code, stderr.String())
	}
	for _, want := range []string{"WITNESS " + witness.PairedProfile, "range app base=" + appCommits[0] + " head=" + appCommits[1], "range tests base=" + testsCommits[0] + " head=" + testsCommits[1], "\nRANGE app\n", "\nRANGE tests\n"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("text report lacks %q:\n%s", want, stdout.String())
		}
	}
}

// AGW-V0-013: a range whose base, head or root does not resolve refuses the
// whole paired report with exit 2, prints nothing, and names the range.
func TestRunWitnessPairedRefusesAnUnresolvedRange(t *testing.T) {
	t.Parallel()
	root := witnessCLIRange(t)
	unknown := strings.Repeat("a", 40)
	for _, testCase := range []struct {
		arguments []string
		reason    string
	}{
		{[]string{"--range", "app=HEAD~1..HEAD", "--range", "tests=HEAD~1.." + unknown}, "--range tests: head:"},
		{[]string{"--range", "app=" + unknown + "..HEAD", "--range", "tests=HEAD~1..HEAD"}, "--range app: base:"},
		{[]string{"--range", "app=HEAD~1..HEAD", "--range", "tests=HEAD~1..HEAD", "--range-root", "tests=" + t.TempDir()}, "--range-root tests:"},
	} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		if code := runWitness(t.Context(), root, testCase.arguments, stdout, stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), testCase.reason) {
			t.Fatalf("%v: exit = %d stdout %q stderr %q, want 2 naming %q", testCase.arguments, code, stdout.String(), stderr.String(), testCase.reason)
		}
	}
}

// AGW-V0-015, AGW-V0-036: the witness help names the merged-history pairing
// trailer and the one option that reads it.
func TestWitnessHelpDocumentsThePairingTrailer(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"Paired-Range: <name>=<repo>@<base>..<head>", "Only --paired-from REV reads it", "witness --paired-from REV --range-root app=PATH", "Explicit\n--range flags read no trailer."} {
		if !strings.Contains(witnessHelp, want) {
			t.Fatalf("witness help lacks %q", want)
		}
	}
}

// AGW-V0-015: the pairing trailer is documented, not computed. A tests head
// whose message carries a Paired-Range trailer (same parent and tree) yields
// the paired report the trailer-less head yields, up to the head object ID.
func TestRunWitnessPairedIgnoresAPairingTrailer(t *testing.T) {
	t.Parallel()
	// Distinct fixtures, so the app range shares no object ID with the tests head.
	app, tests := witnessGoModuleRange(t), witnessCLIRange(t)
	appCommits := revParse(t, app, "HEAD~1", "HEAD")
	arguments := []string{"--range", "app=HEAD~1..HEAD", "--range-root", "app=" + app, "--range", "tests=HEAD~1..HEAD", "--json"}
	report := func() string {
		t.Helper()
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		if code := runWitness(t.Context(), tests, arguments, stdout, stderr); code != 0 {
			t.Fatalf("exit = %d; stderr %s", code, stderr.String())
		}
		return stdout.String()
	}
	plainHead := revParse(t, tests, "HEAD")[0]
	plain := report()
	command := exec.Command("git", "commit", "-q", "--amend", "-m", "change\n\nPaired-Range: app=example.test/app@"+appCommits[0]+".."+appCommits[1])
	command.Dir = tests
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit --amend: %v\n%s", err, output)
	}
	trailerHead := revParse(t, tests, "HEAD")[0]
	if trailerHead == plainHead {
		t.Fatal("amend did not move the tests head")
	}
	if got, want := report(), strings.ReplaceAll(plain, plainHead, trailerHead); got != want {
		t.Fatalf("a Paired-Range trailer changed the paired report:\n%s\nwant\n%s", got, want)
	}
}

// amendTestsHead rewrites the tests checkout's HEAD message and returns the
// new head.
func amendTestsHead(t *testing.T, root, message string) string {
	t.Helper()
	command := exec.Command("git", "commit", "-q", "--amend", "-m", message)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit --amend: %v\n%s", err, output)
	}
	return revParse(t, root, "HEAD")[0]
}

// AGW-V0-036: --paired-from takes no explicit range, base, head or
// single-repository option, and requires the application checkout by name.
func TestParseWitnessPairedFrom(t *testing.T) {
	t.Parallel()
	options, err := parseWitnessOptions([]string{"--paired-from", "HEAD", "--range-root", "app=/x/app", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if options.pairedFrom != "HEAD" || options.rangeRoots[witness.AppRange] != "/x/app" || options.ranges != nil {
		t.Fatalf("options = %+v", options)
	}
	// AGW-V0-037: the application checkout is required only once the trailer
	// is read, so the refusal can name the trailer's repository.
	if options, err := parseWitnessOptions([]string{"--paired-from", "HEAD", "--range-root", "tests=/t"}); err != nil || options.pairedFrom != "HEAD" {
		t.Fatalf("no app root: options = %+v, err = %v", options, err)
	}
	for name, testCase := range map[string]struct {
		arguments []string
		reason    string
	}{
		"with range":  {[]string{"--paired-from", "HEAD", "--range-root", "app=/x", "--range", "app=a..b", "--range", "tests=a..b"}, "it replaces --range"},
		"with base":   {[]string{"--paired-from", "HEAD", "--range-root", "app=/x", "--base", "x"}, "replaces --base and --head"},
		"with head":   {[]string{"--paired-from=HEAD", "--range-root", "app=/x", "--head", "x"}, "replaces --base and --head"},
		"with cem":    {[]string{"--paired-from", "HEAD", "--range-root", "app=/x", "--cem", "m.json"}, "name one repository"},
		"no value":    {[]string{"--paired-from"}, "missing value for --paired-from"},
		"empty value": {[]string{"--paired-from=", "--range-root", "app=/x"}, "takes the tests commit"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWitnessOptions(testCase.arguments); err == nil || !strings.Contains(err.Error(), testCase.reason) {
				t.Fatalf("%v: err = %v, want %q", testCase.arguments, err, testCase.reason)
			}
		})
	}
}

// runWitnessOutput runs witness in root and returns stdout, stderr and the exit.
func runWitnessOutput(t *testing.T, root string, arguments ...string) (string, string, int) {
	t.Helper()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := runWitness(t.Context(), root, arguments, stdout, stderr)
	return stdout.String(), stderr.String(), code
}

// AGW-V0-036: --paired-from REV reads the app range from REV's Paired-Range
// trailer and the tests range as REV's parent to REV, and yields the paired
// report the equivalent explicit --range flags yield, plus a pairing member
// naming REV, the repository identity and the trailer.
func TestRunWitnessPairedFromReadsTheTrailer(t *testing.T) {
	t.Parallel()
	app, tests := witnessGoModuleRange(t), witnessCLIRange(t)
	appCommits := revParse(t, app, "HEAD~1", "HEAD")
	value := "app=git@example.test:org/app.git@" + appCommits[0] + ".." + appCommits[1]
	head := amendTestsHead(t, tests, "change\n\nPaired-Range: "+value)
	parent := revParse(t, tests, "HEAD~1")[0]
	explicit, stderr, code := runWitnessOutput(t, tests, "--range", "app="+appCommits[0]+".."+appCommits[1], "--range-root", "app="+app, "--range", "tests="+parent+".."+head, "--json")
	if code != 0 {
		t.Fatalf("explicit exit = %d; stderr %s", code, stderr)
	}
	trailer, stderr, code := runWitnessOutput(t, tests, "--paired-from", "HEAD", "--range-root", "app="+app, "--json")
	if code != 0 {
		t.Fatalf("--paired-from exit = %d; stderr %s", code, stderr)
	}
	var report witness.PairedReport
	if err := json.Unmarshal([]byte(trailer), &report); err != nil {
		t.Fatalf("%v: %s", err, trailer)
	}
	want := witness.Pairing{Source: "trailer", Commit: head, Repository: "git@example.test:org/app.git", Trailer: value}
	if report.Pairing == nil || *report.Pairing != want {
		t.Fatalf("pairing = %+v, want %+v", report.Pairing, want)
	}
	report.Pairing = nil
	without := new(bytes.Buffer)
	if err := emit(without, report); err != nil {
		t.Fatal(err)
	}
	if without.String() != explicit {
		t.Fatalf("the trailer report without its pairing member differs from the explicit report:\n%s\nwant\n%s", without.String(), explicit)
	}
	text, _, _ := runWitnessOutput(t, tests, "--paired-from", head, "--range-root", "app="+app)
	if !strings.Contains(text, "\npairing trailer commit="+head+" repository=git@example.test:org/app.git\n") {
		t.Fatalf("text report lacks the pairing line:\n%s", text)
	}
}

// AGW-V0-036: every trailer --paired-from cannot use refuses the whole report
// with exit 2 and no stdout, naming the reason; none is ignored.
func TestRunWitnessPairedFromRefusesAnUnusableTrailer(t *testing.T) {
	t.Parallel()
	app, other := witnessGoModuleRange(t), witnessCLIRange(t)
	appCommits := revParse(t, app, "HEAD~1", "HEAD")
	span := appCommits[0] + ".." + appCommits[1]
	for name, testCase := range map[string]struct {
		message, appRoot, reason string
	}{
		"no trailer":    {"change", app, "carries no Paired-Range trailer"},
		"two trailers":  {"change\n\nPaired-Range: app=r@" + span + "\nPaired-Range: app=s@" + span, app, "carries 2 Paired-Range trailers"},
		"tests name":    {"change\n\nPaired-Range: tests=r@" + span, app, "pairs only with app"},
		"abbreviated":   {"change\n\nPaired-Range: app=r@" + appCommits[0][:12] + ".." + appCommits[1], app, "full lowercase-hex commit object IDs"},
		"no repository": {"change\n\nPaired-Range: app=" + span, app, "with a repository identity"},
		"outside block": {"change\n\nPaired-Range: app=r@" + span + "\n\nmore prose", app, "outside the closing trailer block"},
		"unmapped":      {"change\n\nPaired-Range: app=r@" + span, other, "is not a commit in the mapped checkout"},
		"stale":         {"change\n\nPaired-Range: app=r@" + appCommits[1] + ".." + appCommits[0], app, "stale pairing"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tests := witnessCLIRange(t)
			amendTestsHead(t, tests, testCase.message)
			stdout, stderr, code := runWitnessOutput(t, tests, "--paired-from", "HEAD", "--range-root", "app="+testCase.appRoot, "--json")
			if code != 2 || stdout != "" || !strings.Contains(stderr, "--paired-from: ") || !strings.Contains(stderr, testCase.reason) {
				t.Fatalf("exit = %d stdout %q stderr %q, want 2 naming %q", code, stdout, stderr, testCase.reason)
			}
		})
	}
	// AGW-V0-037: without --range-root app the trailer is still read, and the
	// refusal names the trailer and its repository; a malformed trailer is
	// refused as malformed first; a revision that does not resolve names the
	// trailer it was to carry.
	for name, testCase := range map[string]struct {
		message, revision string
		reasons           []string
	}{
		"unknown repository": {"change\n\nPaired-Range: app=git@example.test:org/app.git@" + span, "HEAD", []string{"Paired-Range: app=git@example.test:org/app.git@" + span, "names repository git@example.test:org/app.git", "pass --range-root app=PATH"}},
		"malformed":          {"change\n\nPaired-Range: app=" + span, "HEAD", []string{"Paired-Range: app=" + span, "with a repository identity"}},
		"unresolvable":       {"change\n\nPaired-Range: app=r@" + span, "no-such-revision", []string{"cannot read a Paired-Range trailer", "no-such-revision"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tests := witnessCLIRange(t)
			amendTestsHead(t, tests, testCase.message)
			stdout, stderr, code := runWitnessOutput(t, tests, "--paired-from", testCase.revision, "--json")
			if code != 2 || stdout != "" || !strings.Contains(stderr, "--paired-from: ") {
				t.Fatalf("exit = %d stdout %q stderr %q, want exit 2 naming --paired-from", code, stdout, stderr)
			}
			for _, reason := range testCase.reasons {
				if !strings.Contains(stderr, reason) {
					t.Fatalf("stderr %q does not name %q", stderr, reason)
				}
			}
		})
	}
	// The trailer's own commit must have exactly one parent.
	tests := witnessCLIRange(t)
	rootCommit := revParse(t, tests, "HEAD~1")[0]
	if stdout, stderr, code := runWitnessOutput(t, tests, "--paired-from", rootCommit, "--range-root", "app="+app); code != 2 || stdout != "" || !strings.Contains(stderr, "exactly one parent") {
		t.Fatalf("root commit: exit = %d stdout %q stderr %q", code, stdout, stderr)
	}
}
