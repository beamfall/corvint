package platform

import (
	"bytes"
	"os"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
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
