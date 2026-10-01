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

func TestVSTestLiveExecution(t *testing.T) {
	root, exe := os.Getenv("CORVINT_VSTEST_LIVE_ROOT"), os.Getenv("CORVINT_VSTEST_LIVE_EXE")
	if root == "" || exe == "" {
		t.Skip("operator-built VSTest fixtures and host not supplied")
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	sha := tr.Digest(b)
	out, e := os.MkdirTemp("", "vstest-")
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("retained live evidence: %s", out)
	for _, f := range []struct{ name, dir, assembly, source, project string }{{"nunit", "dotnet", "Probe", "Probe.cs", "Probe.csproj"}, {"mstest", "mstest", "Probe", "Probe.cs", "Probe.csproj"}, {"xunit", "xunit", "NativeProbe", "UnitTest1.cs", "xunit.csproj"}} {
		for _, c := range []struct {
			name, selector string
			exit, count    int
		}{{"mixed", "", 1, 3}, {"pass", "Probe.Pass", 0, 1}, {"skip", "Probe.Skip", 0, 1}} {
			t.Run(f.name+"/"+c.name, func(t *testing.T) {
				stem := "bin/Debug/net9.0/" + f.assembly
				inputs := map[string]string{}
				for _, n := range []string{stem + ".dll", stem + ".deps.json", stem + ".runtimeconfig.json", f.source, f.project} {
					b, e := os.ReadFile(filepath.Join(root, f.dir, n))
					if e != nil {
						t.Fatal(e)
					}
					inputs[n] = tr.Digest(b)
				}
				r := tr.Request{Runner: "dotnet-vstest-" + f.name, Root: filepath.Join(root, f.dir), Executable: exe, ExecutableSha256: sha, Project: stem + ".dll", InputFiles: inputs, ReportDir: filepath.Join(out, f.name+"-"+c.name), TimeoutSeconds: 60}
				if c.selector != "" {
					r.Selectors = []string{c.selector}
				}
				v, e := Build(r)
				if e != nil {
					t.Fatal(e)
				}
				x, execErr := tr.Execute(context.Background(), r, v)
				o, parseErr := Parse(x.Input)
				o = tr.Normalize(x.Input, o)
				raw, e := json.MarshalIndent(struct {
					Request     tr.Request
					Invocation  tr.Invocation
					Execution   tr.Execution
					Observation tr.Observation
				}{r, v, x, o}, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(out, f.name+"-"+c.name+".json"), append(raw, '\n'), 0600); e != nil {
					t.Fatal(e)
				}
				if execErr != nil || parseErr != nil || x.Input.ExitCode != c.exit || !o.Complete || len(o.Tests) != c.count {
					t.Fatalf("execute=%v parse=%v exit=%d observation=%+v", execErr, parseErr, x.Input.ExitCode, o)
				}
			})
		}
	}
}
