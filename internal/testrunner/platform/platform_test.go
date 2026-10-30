package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := readFixture("testdata", name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Fixtures are actual outputs of the named native runner versions. No synthetic
// generic-JUnit success record qualifies another framework.
func TestNativePlatformReports(t *testing.T) {
	for _, tc := range []struct {
		runner, file, report                 string
		exit, tests, passed, failed, skipped int
	}{
		{"junit-platform", "jupiter.xml", "TEST-junit-jupiter.xml", 1, 4, 1, 2, 1},
		{"junit4", "vintage.xml", "TEST-junit-vintage.xml", 1, 1, 0, 1, 0},
		{"kotlin-test", "kotlin-junit.xml", "TEST-junit-jupiter.xml", 1, 4, 1, 2, 1},
		{"kotest", "kotest.xml", "TEST-kotest.xml", 1, 4, 1, 2, 1},
		{"robolectric", "robolectric.xml", "TEST-junit-vintage.xml", 1, 4, 1, 2, 1},
		{"testng", "testng.xml", "testng-results.xml", 3, 4, 1, 2, 1},
		{"swift-testing", "swift.jsonl", "swift-events.jsonl", 1, 3, 1, 1, 1},
		{"bats", "bats.tap", "", 1, 3, 1, 1, 1},
		{"shellspec", "shellspec.xml", "", 101, 3, 1, 1, 1},
	} {
		t.Run(tc.runner, func(t *testing.T) {
			in := tr.Input{Runner: tc.runner, ExitCode: tc.exit, Reports: map[string][]byte{}}
			if tc.report == "" {
				in.Stdout = fixture(t, tc.file)
			} else {
				in.Reports[tc.report] = fixture(t, tc.file)
			}
			o, err := Parse(in)
			if err != nil || !o.Complete || len(o.Tests) != tc.tests {
				t.Fatalf("%+v err=%v", o, err)
			}
			counts := map[string]int{}
			for _, test := range o.Tests {
				counts[test.State]++
			}
			if counts[tr.Passed] != tc.passed || counts[tr.Failed] != tc.failed || counts[tr.Skipped] != tc.skipped {
				t.Fatal(counts)
			}
			in.ExitCode = 99
			if o, err = Parse(in); err == nil && o.Complete {
				t.Fatal("contradictory exit admitted")
			}
			in.ExitCode = tc.exit
			in.TimedOut = true
			if o, err = Parse(in); err != nil || o.Complete {
				t.Fatalf("timeout %+v %v", o, err)
			}
			for _, test := range o.Tests {
				if test.State == tr.Passed {
					t.Fatal("timeout left a passing observation")
				}
			}
		})
	}
}

func TestPlatformRejectsIncompleteAndAmbiguousNativeReports(t *testing.T) {
	for _, tc := range []struct {
		runner, file, report string
		exit                 int
	}{
		{"junit-platform", "jupiter.xml", "TEST-junit-jupiter.xml", 1},
		{"testng", "testng.xml", "testng-results.xml", 3},
		{"swift-testing", "swift.jsonl", "swift-events.jsonl", 1},
		{"bats", "bats.tap", "", 1},
		{"shellspec", "shellspec.xml", "", 101},
	} {
		t.Run(tc.runner, func(t *testing.T) {
			b := fixture(t, tc.file)
			for _, raw := range [][]byte{nil, b[:len(b)/2], append(append([]byte{}, b...), b...)} {
				in := tr.Input{Runner: tc.runner, ExitCode: tc.exit, Reports: map[string][]byte{}}
				if tc.report == "" {
					in.Stdout = raw
				} else {
					in.Reports[tc.report] = raw
				}
				if o, e := Parse(in); e == nil && o.Complete {
					t.Fatal("incomplete/duplicate report admitted")
				}
			}
		})
	}
	bad := bytes.Replace(fixture(t, "jupiter.xml"), []byte(`tests="4"`), []byte(`tests="5"`), 1)
	if _, e := Parse(tr.Input{Runner: "junit-platform", ExitCode: 1, Reports: map[string][]byte{"TEST-junit-jupiter.xml": bad}}); e == nil {
		t.Fatal("suite count mismatch admitted")
	}
	bad = bytes.Replace(fixture(t, "shellspec.xml"), []byte(`<skip message=`), []byte(`<skipped message=`), 1)
	if _, e := Parse(tr.Input{Runner: "shellspec", ExitCode: 101, Stdout: bad}); e == nil {
		t.Fatal("foreign skip dialect admitted")
	}
}

func TestPlatformFixedBuildsAndUnavailableObligations(t *testing.T) {
	r := tr.Request{Runner: "junit-platform", Root: "/source", Executable: "/tools/java", ExecutableSha256: strings.Repeat("a", 64), Project: "classes", Reporter: "/tools/console.jar", ReporterSha256: strings.Repeat("b", 64), ReportDir: "/reports", Selectors: []string{"Example#criterion"}}
	i, e := Build(r)
	if e != nil || strings.Join(i.Argv, " ") == "" || len(i.ReportPaths) != 1 {
		t.Fatalf("%+v %v", i, e)
	}
	r.Selectors = []string{"--scan-class-path"}
	if _, e = Build(r); e == nil {
		t.Fatal("option admitted as selector")
	}
	r.Selectors = nil
	r.Runner = "swift-xctest"
	if _, e = Build(r); e == nil {
		t.Fatal("SwiftPM profile admitted without selected Package.swift pin")
	}
	r.Runner = "bats"
	r.Project = "../outside.bats"
	if _, e = Build(r); e == nil {
		t.Fatal("escaping source admitted")
	}
}

func TestXcodeNativeCountsSkipAndDestination(t *testing.T) {
	in := tr.Input{Runner: "xcode-xctest", ExitCode: 65, Reports: map[string][]byte{"xcode-tests.json": fixture(t, "xcode-tests.json"), "xcode-summary.json": fixture(t, "xcode-summary.json")}}
	o, err := Parse(in)
	if err != nil || !o.Complete || len(o.Tests) != 6 {
		t.Fatalf("%+v %v", o, err)
	}
	counts := map[string]int{}
	for _, test := range o.Tests {
		counts[test.State]++
	}
	if counts[tr.Passed] != 2 || counts[tr.Failed] != 2 || counts[tr.Skipped] != 2 {
		t.Fatal(counts)
	}
	in.Reports["xcode-summary.json"] = bytes.Replace(in.Reports["xcode-summary.json"], []byte(`"totalTestCount":6`), []byte(`"totalTestCount":7`), 1)
	if _, err = Parse(in); err == nil {
		t.Fatal("Xcode aggregate mismatch admitted")
	}
}

// TestXcodeBuildFailureAndZeroTests covers V1-2182 (TRE-V0-117) with the
// retained Xcode 27.0 build-error and zero-test xcresult views: a build
// failure is BUILD_FAILURE and an empty plan is NO_TESTS, neither an invalid
// native report; any other destination-less shape stays invalid.
func TestXcodeBuildFailureAndZeroTests(t *testing.T) {
	codes := func(o tr.Observation) map[string]bool {
		m := map[string]bool{}
		for _, p := range o.Problems {
			m[p.Code] = true
		}
		return m
	}
	build := tr.Input{Runner: "xcode-xctest", ExitCode: 65, Stderr: fixture(t, "xcode-build-error-stderr.txt"), Reports: map[string][]byte{"xcode-tests.json": fixture(t, "xcode-build-error-tests.json"), "xcode-summary.json": fixture(t, "xcode-build-error-summary.json")}}
	o, err := Parse(build)
	if c := codes(o); err != nil || o.Complete || len(o.Tests) != 0 || !c["BUILD_FAILURE"] || c["INVALID_NATIVE_REPORT"] {
		t.Fatalf("build failure: %+v %v", o, err)
	}
	zero := tr.Input{Runner: "xcode-xctest", ExitCode: 0, Reports: map[string][]byte{"xcode-tests.json": fixture(t, "xcode-zero-tests.json"), "xcode-summary.json": fixture(t, "xcode-zero-summary.json")}}
	o, err = Parse(zero)
	if c := codes(o); err != nil || o.Complete || len(o.Tests) != 0 || !c["NO_TESTS"] || c["BUILD_FAILURE"] || c["INVALID_NATIVE_REPORT"] {
		t.Fatalf("zero tests: %+v %v", o, err)
	}
	for name, mutate := range map[string]func(*tr.Input){
		"build exit 0":         func(in *tr.Input) { in.ExitCode = 0 },
		"build without report": func(in *tr.Input) { in.Stderr = nil },
		"build with summary run": func(in *tr.Input) {
			in.Reports["xcode-summary.json"] = bytes.Replace(in.Reports["xcode-summary.json"], []byte(`"result":"unknown"`), []byte(`"result":"Failed"`), 1)
		},
		"zero exit 65": func(in *tr.Input) {
			in.ExitCode = 65
			in.Reports["xcode-tests.json"] = fixture(t, "xcode-zero-tests.json")
		},
	} {
		in := build
		in.Reports = map[string][]byte{"xcode-tests.json": build.Reports["xcode-tests.json"], "xcode-summary.json": build.Reports["xcode-summary.json"]}
		mutate(&in)
		if o, err := Parse(in); err == nil || o.Complete || !codes(o)["INVALID_NATIVE_REPORT"] {
			t.Fatalf("%s admitted: %+v %v", name, o, err)
		}
	}
}

func TestInvalidPartialReportNeverRetainsPassingRows(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		in := tr.Input{Runner: "bats", ExitCode: 0, Stdout: []byte("1..2\nok 1 first\nmalformed\n"), Interrupted: interrupted}
		o, err := Parse(in)
		if err == nil || o.Complete || len(o.Tests) != 1 {
			t.Fatalf("invalid partial report: %+v %v", o, err)
		}
		want := tr.Unknown
		if interrupted {
			want = tr.Interrupted
		}
		if o.Tests[0].State != want || len(o.Tests[0].Attempts) != 0 {
			t.Fatalf("partial pass retained: %+v", o)
		}
	}
}

func TestXcodeRejectsContainerContradictions(t *testing.T) {
	for _, result := range []string{"Failed", "Skipped"} {
		tests := []byte(`{"devices":[{"deviceId":"mac","platform":"macOS"}],"testNodes":[{"nodeType":"Test Suite","result":"` + result + `","name":"suite","children":[{"nodeType":"Test Case","result":"Passed","name":"test","nodeIdentifierURL":"test://com.apple.xcode/p/t/c/test"}]}]}`)
		summary := []byte(`{"totalTestCount":1,"passedTests":1,"failedTests":0,"skippedTests":0,"expectedFailures":0,"startTime":1,"finishTime":2,"result":"Passed","devicesAndConfigurations":[{"device":{"deviceId":"mac"}}]}`)
		o, err := Parse(tr.Input{Runner: "xcode-xctest", Reports: map[string][]byte{"xcode-tests.json": tests, "xcode-summary.json": summary}})
		if err == nil || o.Complete || o.Tests[0].State == tr.Passed {
			t.Fatalf("container contradiction retained: %+v %v", o, err)
		}
	}
}

func TestXcodeBindsSelectedSharedScheme(t *testing.T) {
	r := tr.Request{Runner: "xcode-xctest", Root: "/source", Executable: "/tools/xcodebuild", ExecutableSha256: strings.Repeat("a", 64), Project: "Proof.xcodeproj", Config: "/unrelated/Proof.xcscheme", ConfigSha256: strings.Repeat("b", 64), ReportDir: "/reports", Tools: map[string]tr.Tool{"xcresulttool": {Executable: "/tools/xcresulttool", Sha256: strings.Repeat("c", 64)}}}
	if _, err := Build(r); err == nil {
		t.Fatal("unrelated scheme pin admitted")
	}
	r.Config = "/source/Proof.xcodeproj/xcshareddata/xcschemes/Proof.xcscheme"
	if _, err := Build(r); err != nil {
		t.Fatal(err)
	}
}

func TestAggregateReportDoesNotInventAttempts(t *testing.T) {
	o, err := Parse(tr.Input{Runner: "bats", Stdout: []byte("1..1\nnot ok 1 failure\n# command failed\n"), ExitCode: 1})
	if err != nil || !o.Complete || len(o.Tests) != 1 || len(o.Tests[0].Attempts) != 0 || o.RetryInformation != tr.NotReported || o.Tests[0].FailureKind != tr.Unknown || o.Tests[0].Message != "command failed\n" {
		t.Fatalf("aggregate attempt invented or failure details lost: %+v %v", o, err)
	}
}

// Report bundles preserve original capture bytes; undeclared names never become empty reports.
func readFixture(root, name string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, "native-report-fixtures.json"))
	if err != nil {
		return nil, err
	}
	var inventory map[string]struct {
		Bytes  []byte `json:"bytesBase64"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		return nil, err
	}
	entry, declared := inventory[filepath.ToSlash(name)]
	if !declared {
		return nil, fmt.Errorf("native report fixture not declared: %s", name)
	}
	if entry.Bytes == nil || entry.SHA256 != tr.Digest(entry.Bytes) {
		return nil, fmt.Errorf("native report fixture digest mismatch: %s", name)
	}
	return entry.Bytes, nil
}

func TestNativeReportFixtureInventory(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-report-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 32 {
		t.Fatalf("unexpected report inventory: %d", len(inventory))
	}
	for name := range inventory {
		if _, err := readFixture("testdata", name); err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
	}
	if _, err := readFixture("testdata", "undeclared-missing-fixture.txt"); err == nil {
		t.Fatal("undeclared missing fixture was hidden")
	}
	for _, declaration := range []string{`{"bytesBase64":"YQ==","sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`, `{"bytesBase64":"","sha256":"wrong"}`, `{"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "native-report-fixtures.json"), []byte(`{"capture":`+declaration+`}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readFixture(root, "capture"); err == nil {
			t.Fatal("invalid declaration accepted")
		}
	}
}

// TestSwiftTestingSelectorMatchesSourceSuffixedID is the TRE-V0-075 unit
// witness: SwiftPM 6.4 matches --filter against the full Swift Testing ID,
// whose function form ends in "/File.swift:line:column", so the exact
// selector ends at that "/" or at the end of a suite ID and nowhere else.
func TestSwiftTestingSelectorMatchesSourceSuffixedID(t *testing.T) {
	r := tr.Request{Runner: "swift-testing", Root: t.TempDir(), Executable: "/tools/swift", ExecutableSha256: strings.Repeat("b", 64), ReportDir: "/fresh", Selectors: []string{"ProofTests.passCriterion()", "ProofTests.Suite"}}
	v, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	var filters []*regexp.Regexp
	for n, a := range v.Argv {
		if a == "--filter" {
			filters = append(filters, regexp.MustCompile(v.Argv[n+1]))
		}
	}
	match := func(id string) bool {
		for _, f := range filters {
			if f.MatchString(id) {
				return true
			}
		}
		return false
	}
	for _, id := range []string{"ProofTests.passCriterion()/Proof.swift:4:2", "ProofTests.Suite", "ProofTests.Suite/inner()/Proof.swift:9:3"} {
		if !match(id) {
			t.Fatalf("exact selector missed native ID %q (filters %v)", id, filters)
		}
	}
	for _, id := range []string{"ProofTests.passCriterion()x/Proof.swift:4:2", "ProofTests.passCriterion2()/Proof.swift:5:2", "ProofTests.SuiteTwo", "Other.ProofTests.passCriterion()/P.swift:1:1"} {
		if match(id) {
			t.Fatalf("selector widened to %q", id)
		}
	}
	r.Selectors = []string{"ProofTests.passCriterion()/Proof.swift:4:2"}
	if _, e = Build(r); e == nil {
		t.Fatal("source-suffixed selector admitted")
	}
}

// TRE-V0-112 / V1-1535: a package directory whose symlink-resolved SwiftPM
// identity collides with the toolchain swift-testing package is refused
// before execution; an unresolvable package directory is refused too.
func TestSwiftTestingRefusesReservedPackageIdentity(t *testing.T) {
	src := t.TempDir()
	for _, d := range []string{"swift-testing", "Fixtures/Swift-Testing", "Testing", "okpkg", "swift-testing-fixture"} {
		if err := os.MkdirAll(filepath.Join(src, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{"alias-bad": "swift-testing", "swift-testing-link/swift-testing": "../okpkg"}
	for link, target := range links {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(src, link)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(src, link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	base := tr.Request{Runner: "swift-testing", Executable: "/tools/swift", ExecutableSha256: strings.Repeat("b", 64), ReportDir: "/fresh"}
	for _, c := range []struct {
		root, project string
		refused       string
	}{
		{filepath.Join(src, "swift-testing"), "", "reserved toolchain package identity"},
		{src, "Fixtures/Swift-Testing", "reserved toolchain package identity"},
		{src, "alias-bad", "reserved toolchain package identity"},
		{src, "swift-testing-link/swift-testing", ""},
		{src, "Testing", ""},
		{src, "okpkg", ""},
		{filepath.Join(src, "swift-testing-fixture"), "", ""},
		{src, "missing", "cannot be resolved"},
	} {
		r := base
		r.Root, r.Project = c.root, c.project
		_, err := Build(r)
		if c.refused == "" && err != nil || c.refused != "" && (err == nil || !strings.Contains(err.Error(), c.refused)) {
			t.Fatalf("%s/%s: want refusal %q, err=%v", c.root, c.project, c.refused, err)
		}
	}
}
