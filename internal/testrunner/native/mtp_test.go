package native

import (
	"bytes"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMTPNativeMatrix(t *testing.T) {
	for _, f := range []string{"nunit", "mstest", "xunit"} {
		for _, c := range []struct {
			name     string
			exit, n  int
			complete bool
		}{{"pass", 0, 1, true}, {"fail", 2, 1, true}, {"skip", 0, 1, true}, {"zero", 8, 0, false}, {"mixed2", 2, 3, true}, {"infra", 2, 1, true}} {
			t.Run(f+"/"+c.name, func(t *testing.T) {
				b, err := os.ReadFile(filepath.Join("testdata", "mtp", f, c.name+".trx"))
				if err != nil {
					t.Fatal(err)
				}
				o, err := Parse(tr.Input{Runner: "dotnet-mtp-" + f, ExitCode: c.exit, Reports: map[string][]byte{"results.trx": b}})
				if err != nil || o.Complete != c.complete || len(o.Tests) != c.n || o.RetryInformation != tr.NotReported {
					t.Fatalf("%+v %v", o, err)
				}
				for _, row := range o.Tests {
					if !strings.Contains(row.ID, "::") {
						t.Fatal(row)
					}
					if row.State == tr.Failed && (row.FailureKind != tr.Unknown || row.Message == "") {
						t.Fatal(row)
					}
				}
				if c.name == "skip" && f != "xunit" && !strings.Contains(o.Tests[0].Message, "MTP_SKIP") {
					t.Fatal("lost skip reason", o)
				}
				if c.name == "infra" && !strings.Contains(o.Tests[0].Message, "MTP_SETUP") {
					t.Fatal(o)
				}
			})
		}
	}
}
func TestMTPBindings(t *testing.T) {
	b, err := os.ReadFile("testdata/mtp/mstest/fail.trx")
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func([]byte) []byte{
		"duplicate-summary": func(b []byte) []byte {
			return bytes.Replace(b, []byte("</TestRun>"), []byte(`<ResultSummary outcome="Completed"/></TestRun>`), 1)
		},
		"duplicate-attribute": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`outcome="Failed"`), []byte(`outcome="Failed" outcome="Passed"`), 1)
		},
		"wrong-adapter": func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("executor://MSTest.Sdk/4.4.1"), []byte("executor://mstestadapter/v2"))
		},
		"counter": func(b []byte) []byte { return bytes.ReplaceAll(b, []byte(`failed="1"`), []byte(`failed="0"`)) },
		"entry": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`<TestEntry testId="18056bb1`), []byte(`<TestEntry testId="28056bb1`), 1)
		},
		"definition": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`<Execution id="efc084cd`), []byte(`<Execution id="afc084cd`), 1)
		},
		"trailing": func(b []byte) []byte { return append(b, []byte("<x/>")...) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(tr.Input{Runner: "dotnet-mtp-mstest", ExitCode: 2, Reports: map[string][]byte{"results.trx": edit(bytes.Clone(b))}}); err == nil {
				t.Fatal("accepted malformed report")
			}
		})
	}
	for _, exit := range []int{0, 1, 8, 10} {
		o, e := Parse(tr.Input{Runner: "dotnet-mtp-mstest", ExitCode: exit, Reports: map[string][]byte{"results.trx": b}})
		if e != nil || o.Complete {
			t.Fatalf("exit %d: %+v %v", exit, o, e)
		}
	}
	unexpected := bytes.ReplaceAll(b, []byte("Exit code indicates failure: '2'. Please refer to https://aka.ms/testingplatform/exitcodes for more information."), []byte("unreported discovery exception"))
	o, e := Parse(tr.Input{Runner: "dotnet-mtp-mstest", ExitCode: 2, Reports: map[string][]byte{"results.trx": unexpected}})
	if e != nil || o.Complete {
		t.Fatalf("%+v %v", o, e)
	}
}
func TestMTPBuild(t *testing.T) {
	for _, f := range []string{"nunit", "mstest", "xunit"} {
		r := tr.Request{Runner: "dotnet-mtp-" + f, Executable: "/sdk/dotnet", ReportDir: "/reports", Project: "bin/Probe.dll", Selectors: []string{"MtpProbe.Probe.Pass"}, InputFiles: map[string]string{"bin/Probe.dll": "sha", "bin/Probe.runtimeconfig.json": "sha", "bin/Probe.deps.json": "sha"}}
		v, e := Build(r)
		if e != nil || !reflect.DeepEqual(v.FailureExitCodes, []int{2}) || v.Argv[0] != r.Project || v.Environment["TESTINGPLATFORM_TELEMETRY_OPTOUT"] != "1" {
			t.Fatalf("%+v %v", v, e)
		}
		for _, bad := range []string{"../Probe.dll", "/tmp/Probe.dll", "Probe.csproj", "-probe.dll"} {
			q := r
			q.Project = bad
			if _, e := Build(q); e == nil {
				t.Fatal(bad)
			}
		}
		for _, bad := range []string{"A|B", "A=B", "A B", "A(1)", "A::id"} {
			q := r
			q.Selectors = []string{bad}
			if _, e := Build(q); e == nil {
				t.Fatal(bad)
			}
		}
		delete(r.InputFiles, "bin/Probe.deps.json")
		if _, e := Build(r); e == nil {
			t.Fatal("missing declared deps")
		}
	}
}

func mtpTestRequest(r *tr.Request) {
	if strings.HasPrefix(r.Runner, "dotnet-mtp-") {
		r.Project = "Probe.dll"
		r.InputFiles = map[string]string{"Probe.dll": "sha", "Probe.runtimeconfig.json": "sha", "Probe.deps.json": "sha"}
	}
}
