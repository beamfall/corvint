package platform

import (
	"bytes"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"strings"
	"testing"
)

func TestNativeJVMBuildReports(t *testing.T) {
	for _, tc := range []struct{ runner, file string }{{"gradle-junit", "gradle.xml"}, {"maven-surefire", "surefire.xml"}} {
		t.Run(tc.runner, func(t *testing.T) {
			in := tr.Input{Runner: tc.runner, ExitCode: 1, Reports: map[string][]byte{"TEST-MatrixTest.xml": fixture(t, tc.file)}}
			o, e := Parse(in)
			if e != nil || !o.Complete || len(o.Tests) != 4 {
				t.Fatalf("%+v %v", o, e)
			}
			kinds := map[string]int{}
			for _, row := range o.Tests {
				if row.State == tr.Failed {
					kinds[row.FailureKind]++
				}
				if len(row.Attempts) != 0 {
					t.Fatal("invented aggregate attempt")
				}
			}
			if kinds[tr.Assertion] != 1 {
				t.Fatal(kinds)
			}
			if tc.runner == "gradle-junit" && kinds[tr.Unknown] != 1 {
				t.Fatal("Gradle exception classified as assertion")
			}
			if tc.runner == "maven-surefire" && kinds[tr.Infrastructure] != 1 {
				t.Fatal("Surefire error lost")
			}
			in.Reports["TEST-MatrixTest.xml"] = bytes.Replace(in.Reports["TEST-MatrixTest.xml"], []byte(`tests="4"`), []byte(`tests="5"`), 1)
			if o, e = Parse(in); e == nil || o.Complete {
				t.Fatal("inventory contradiction admitted")
			}
		})
	}
}

func TestNativeAndroidOutcomesAndNeutralTransportExit(t *testing.T) {
	for _, tc := range []struct {
		runner, file     string
		pass, fail, skip int
	}{{"android-junit", "android.txt", 1, 2, 1}, {"uiautomator", "uiautomator.txt", 1, 1, 1}, {"espresso", "espresso-infrastructure.txt", 0, 2, 1}, {"espresso", "espresso.txt", 1, 1, 1}} {
		t.Run(tc.runner, func(t *testing.T) {
			in := tr.Input{Runner: tc.runner, ExitCode: 0, OutcomeNeutralExitCodes: []int{0}, Stdout: fixture(t, tc.file)}
			o, e := Parse(in)
			if e != nil {
				t.Fatal(e)
			}
			o = tr.Normalize(in, o)
			if !o.Complete {
				t.Fatalf("%+v", o)
			}
			counts := map[string]int{}
			for _, row := range o.Tests {
				counts[row.State]++
				if tc.file == "espresso-infrastructure.txt" && row.FailureKind == tr.Assertion {
					t.Fatal("focus infrastructure failure claimed assertion")
				}
			}
			if counts[tr.Passed] != tc.pass || counts[tr.Failed] != tc.fail || counts[tr.Skipped] != tc.skip {
				t.Fatal(counts)
			}
			for _, bad := range [][]byte{in.Stdout[:len(in.Stdout)/2], bytes.Replace(in.Stdout, []byte("INSTRUMENTATION_CODE: -1"), []byte("INSTRUMENTATION_CODE: 0"), 1), bytes.Replace(in.Stdout, []byte("numtests=3"), []byte("numtests=4"), 1)} {
				if bytes.Equal(bad, in.Stdout) {
					continue
				}
				in2 := in
				in2.Stdout = bad
				if o, e = Parse(in2); e == nil && o.Complete {
					t.Fatal("incomplete/inconsistent instrumentation admitted")
				}
			}
		})
	}
}

func TestExtendedFixedBuilds(t *testing.T) {
	r := tr.Request{Root: "/source", Executable: "/tools/runner", ExecutableSha256: strings.Repeat("a", 64), ReportDir: "/fresh", Project: "app", Config: "/source/app/build.gradle", ConfigSha256: strings.Repeat("b", 64), Target: ":test", Runner: "gradle-junit", Tools: map[string]tr.Tool{"java": {Executable: "/jdk/bin/java", Sha256: strings.Repeat("c", 64)}}}
	i, e := Build(r)
	if e != nil || string(i.Files["corvint.init.gradle"]) != gradleReportScript || len(i.ReportPatterns) != 1 {
		t.Fatalf("%+v %v", i, e)
	}
	r.Runner = "maven-surefire"
	r.Config = "/source/app/pom.xml"
	r.Target = ""
	if _, e = Build(r); e != nil {
		t.Fatal(e)
	}
	r.Runner = "android-junit"
	r.Target = "emulator-5554"
	r.Project = "local.proof.test/androidx.test.runner.AndroidJUnitRunner"
	r.Selectors = []string{"local.proof.Case#test"}
	i, e = Build(r)
	if e != nil || len(i.SuccessExitCodes) != 0 || len(i.FailureExitCodes) != 0 || len(i.OutcomeNeutralExitCodes) != 1 {
		t.Fatalf("%+v %v", i, e)
	}
	r.Target = "physical-unknown"
	if _, e = Build(r); e == nil {
		t.Fatal("unqualified physical device profile admitted")
	}
}

func TestAndroidNativeZeroTestsAbstains(t *testing.T) {
	o, e := Parse(tr.Input{Runner: "android-junit", Stdout: fixture(t, "android-zero.txt"), ExitCode: 0, OutcomeNeutralExitCodes: []int{0}})
	if e != nil || o.Complete || len(o.Tests) != 0 {
		t.Fatalf("%+v %v", o, e)
	}
}

func TestGradleRejectsSimpleClassShortcut(t *testing.T) {
	r := tr.Request{Root: "/source", Executable: "/tools/gradle", ExecutableSha256: strings.Repeat("a", 64), ReportDir: "/fresh", Project: "app", Config: "/source/app/build.gradle", ConfigSha256: strings.Repeat("b", 64), Target: ":test", Runner: "gradle-junit", Tools: map[string]tr.Tool{"java": {Executable: "/jdk/bin/java", Sha256: strings.Repeat("c", 64)}}}
	for _, selector := range []string{"Case#pass", "Outer.Case#pass", "a..Case#pass", "a.Case#*"} {
		r.Selectors = []string{selector}
		if _, err := Build(r); err == nil {
			t.Fatalf("Gradle shortcut or ambiguous selector admitted: %q", selector)
		}
	}
	r.Selectors = []string{"a.Case#pass"}
	inv, err := Build(r)
	if err != nil || strings.Join(inv.Argv[len(inv.Argv)-2:], " ") != "--tests a.Case.pass" {
		t.Fatalf("%+v %v", inv, err)
	}
}
