//go:build darwin || linux

package native

import (
	"context"
	"encoding/json"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"path/filepath"
	"testing"
)

// Opt-in only: these are operator-built native applications; the test does not
// restore packages or download an SDK. Receipts remain outside the repository.
func TestMTPLiveExecution(t *testing.T) {
	root := os.Getenv("CORVINT_MTP_LIVE_ROOT")
	if root == "" {
		t.Skip("operator-built MTP fixture root not supplied")
	}
	runDir, err := os.MkdirTemp("", "mtp-")
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
				project := "bin/Debug/net10.0/Probe.dll"
				inputs := map[string]string{}
				for _, n := range []string{project, "bin/Debug/net10.0/Probe.runtimeconfig.json", "bin/Debug/net10.0/Probe.deps.json", "Probe.cs", "Infrastructure.cs", "Probe.csproj"} {
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
	}
}
