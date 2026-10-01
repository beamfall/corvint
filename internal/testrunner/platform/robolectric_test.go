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
