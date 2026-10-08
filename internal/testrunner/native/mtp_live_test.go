//go:build darwin || linux

package native

import (
	"context"
	"encoding/json"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in only: these are operator-built native applications; the test does not
// restore packages or download an SDK. Receipts remain outside the repository.
// CORVINT_MTP_LIVE_TFM selects the built target framework (default net10.0).
// CORVINT_MTP_LIVE_RUN_DIR is the evidence parent; MTP admission (TRE-V0-025)
// needs it short, since the platform temporary directory can exceed the bound.
func TestMTPLiveExecution(t *testing.T) {
	root := os.Getenv("CORVINT_MTP_LIVE_ROOT")
	if root == "" {
		t.Skip("operator-built MTP fixture root not supplied")
	}
	tfm := os.Getenv("CORVINT_MTP_LIVE_TFM")
	if tfm == "" {
		tfm = "net10.0"
	}
	runDir, err := os.MkdirTemp(os.Getenv("CORVINT_MTP_LIVE_RUN_DIR"), "m")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained live evidence: %s", runDir)
	exe := filepath.Join(root, "sdk", "dotnet")
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	exeSHA := tr.Digest(b)
	for _, f := range []string{"nunit", "mstest", "xunit"} {
		for _, c := range []struct {
			name, method string
			exit         int
			complete     bool
		}{{"pass", "MtpProbe.Probe.Pass", 0, true}, {"fail", "MtpProbe.Probe.Fail", 2, true}, {"skip", "MtpProbe.Probe.Skip", 0, true}, {"zero", "MtpProbe.Probe.Missing", 8, false}, {"infra", "MtpProbe.InfrastructureProbe.Blocked", 2, true}} {
			t.Run(f+"/"+c.name, func(t *testing.T) {
				project := "bin/Debug/" + tfm + "/Probe.dll"
				inputs := map[string]string{}
				for _, n := range []string{project, "bin/Debug/" + tfm + "/Probe.runtimeconfig.json", "bin/Debug/" + tfm + "/Probe.deps.json", "Probe.cs", "Infrastructure.cs", "Probe.csproj"} {
					b, e := os.ReadFile(filepath.Join(root, f, n))
					if e != nil {
						t.Fatal(e)
					}
					inputs[n] = tr.Digest(b)
				}
				r := tr.Request{Runner: "dotnet-mtp-" + f, Root: filepath.Join(root, f), Executable: exe, ExecutableSha256: exeSHA, ReportDir: filepath.Join(runDir, f+"-"+c.name), Project: project, Selectors: []string{c.method}, InputFiles: inputs, TimeoutSeconds: 60}
				v, e := Build(r)
				if e != nil {
					t.Fatal(e)
				}
				x, e := tr.Execute(context.Background(), r, v)
				if e != nil {
					t.Fatal(e)
				}
				o, e := Parse(x.Input)
				if e != nil {
					t.Fatal(e)
				}
				o = tr.Normalize(x.Input, o)
				retained := struct {
					Request     tr.Request
					Invocation  tr.Invocation
					Execution   tr.Execution
					Observation tr.Observation
				}{r, v, x, o}
				raw, e := json.MarshalIndent(retained, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(runDir, f+"-"+c.name+".json"), append(raw, '\n'), 0600); e != nil {
					t.Fatal(e)
				}
				if x.Input.ExitCode != c.exit || o.Complete != c.complete {
					t.Fatalf("exit %d: %+v", x.Input.ExitCode, o)
				}
			})
		}
		// TRE-V0-025: a report directory past the native socket bound is refused
		// before launch; forcing the same invocation past admission reproduces the
		// native abort, which must never yield a complete observation.
		t.Run(f+"/long-report-dir", func(t *testing.T) {
			project := "bin/Debug/" + tfm + "/Probe.dll"
			inputs := map[string]string{}
			for _, n := range []string{project, "bin/Debug/" + tfm + "/Probe.runtimeconfig.json", "bin/Debug/" + tfm + "/Probe.deps.json"} {
				b, e := os.ReadFile(filepath.Join(root, f, n))
				if e != nil {
					t.Fatal(e)
				}
				inputs[n] = tr.Digest(b)
			}
			long := filepath.Join(runDir, f+"-long-"+strings.Repeat("x", 64))
			r := tr.Request{Runner: "dotnet-mtp-" + f, Root: filepath.Join(root, f), Executable: exe, ExecutableSha256: exeSHA, ReportDir: long, Project: project, Selectors: []string{"MtpProbe.Probe.Pass"}, InputFiles: inputs, TimeoutSeconds: 60}
			if _, e := Build(r); e == nil || !strings.Contains(e.Error(), "native pipe path") {
				t.Fatalf("long report directory admitted: %v", e)
			}
			short := r
			short.ReportDir = filepath.Join(runDir, f+"-n")
			v, e := Build(short)
			if e != nil {
				t.Fatal(e)
			}
			for i, a := range v.Argv {
				if a == short.ReportDir {
					v.Argv[i] = long
				}
			}
			x, e := tr.Execute(context.Background(), r, v)
			if e != nil {
				t.Logf("forced execution refused: %v", e)
				return
			}
			stderr, _ := os.ReadFile(filepath.Join(long, ".phase-00-stderr"))
			t.Logf("forced native exit %d: %.300s", x.Input.ExitCode, stderr)
			if o, e := Parse(x.Input); e == nil && tr.Normalize(x.Input, o).Complete {
				t.Fatalf("native socket abort produced a complete observation: %+v", o)
			}
		})
	}
}
