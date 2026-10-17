package platform

import (
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"strings"
	"testing"
)

func TestRobolectricOfflineSDKProfile(t *testing.T) {
	r := tr.Request{Runner: "robolectric", Root: "/source", Project: "classes", Executable: "/jdk/bin/java", ExecutableSha256: strings.Repeat("a", 64), Reporter: "/tools/console.jar", ReporterSha256: strings.Repeat("b", 64), Config: "/sdks/android-all-instrumented-15-robolectric-13954326-i7.jar", ConfigSha256: strings.Repeat("c", 64), Target: "35", ReportDir: "/fresh"}
	inv, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	argv := strings.Join(inv.Argv, " ")
	for _, want := range []string{"-Drobolectric.offline=true", "-Drobolectric.enabledSdks=35", "-Drobolectric.dependency.dir=/sdks", "--include-engine junit-vintage"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("missing %s: %s", want, argv)
		}
	}
	r.Target = "36"
	if _, e = Build(r); e == nil {
		t.Fatal("unqualified SDK tuple admitted")
	}
	r.Target = "35"
	r.Config = "/sdks/unrelated.jar"
	if _, e = Build(r); e == nil {
		t.Fatal("unrelated SDK pin admitted")
	}
}
func TestRobolectricNativeInitializationErrorIsNotAssertion(t *testing.T) {
	o, e := Parse(tr.Input{Runner: "robolectric", ExitCode: 1, Reports: map[string][]byte{"TEST-junit-vintage.xml": fixture(t, "robolectric-infrastructure.xml")}})
	if e != nil || len(o.Tests) != 1 || o.Tests[0].FailureKind != tr.Infrastructure || o.Tests[0].State != tr.Failed {
		t.Fatalf("%+v %v", o, e)
	}
}

func TestRobolectricZeroIsNotPass(t *testing.T) {
	o, e := Parse(tr.Input{Runner: "robolectric", ExitCode: 2, Reports: map[string][]byte{"TEST-junit-vintage.xml": fixture(t, "robolectric-zero.xml")}})
	if e == nil && o.Complete {
		t.Fatalf("empty native inventory admitted: %+v", o)
	}
}

// TRE-V0-077: a Robolectric class whose SDK is outside enabledSdks ran no test;
// the outcome-free Vintage runner leaf must not become a pass (V1-0594 witness).
func TestVintageRunnerLeafIsNotPass(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="JUnit Vintage" tests="1" skipped="0" failures="0" errors="0">
<testcase name="proof.BrokenTest" classname="proof.BrokenTest"><system-out><![CDATA[
unique-id: [engine:junit-vintage]/[runner:proof.BrokenTest]
]]></system-out></testcase>
</testsuite>`
	for _, runner := range []string{"robolectric", "junit4"} {
		o, e := Parse(tr.Input{Runner: runner, ExitCode: 0, Reports: map[string][]byte{"TEST-junit-vintage.xml": []byte(xml)}})
		if e != nil || o.Complete || len(o.Tests) != 0 {
			t.Fatalf("%s: runner leaf admitted: %+v %v", runner, o, e)
		}
	}
}
