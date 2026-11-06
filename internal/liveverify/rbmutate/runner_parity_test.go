package rbmutate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// errText is an error's text, or "<nil>".
func errText(err error) string { return fmt.Sprint(err) }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestRSpecPathMatchesTheRegistry traces FPK-V0-079 and FPK-V0-080 through
// the narrowed runner path (V1-4752): for the rbmutate request shape,
// buildRSpec and parseRSpec give the same invocation and observation bytes
// and the same errors as registry.Build and registry.Parse, for admitted and
// refused requests, so dropping the registry import changes no plan or receipt.
func TestRSpecPathMatchesTheRegistry(t *testing.T) {
	if family, ok := registry.Lookup("rspec"); !ok || family.Name != "testrunner/dynamic" {
		t.Fatalf("rspec is not registered by the dynamic family: %+v %v", family, ok)
	}
	runDir := filepath.Join(t.TempDir(), "corvint-rbmutate-1")
	runner := containerRunner{
		profile: &ContainerProfile{
			Executable: "/usr/local/bin/docker", ExecutableSha256: tr.Digest([]byte("cli")),
			ExecPrefix: []string{"run", "--rm", "-v", RunPlaceholder + ":/work", "-w", "/work/tree", "img", "rspec"}, Container: "/work",
		},
		configuration: settings{changedPath: "lib/a.rb", testPath: "spec/a_spec.rb", target: "spec/a_spec.rb", perRun: 90 * time.Second},
	}
	inputs := map[string]string{"lib/a.rb": tr.Digest([]byte("a")), "spec/a_spec.rb": tr.Digest([]byte("s"))}
	admitted := runner.request(runDir, inputs, tr.Digest([]byte("options")))
	probe := runner.request(runDir, map[string]string{"lib/a.rb": tr.Digest(nil)}, "")
	examples := runner
	examples.configuration.target = "spec/a_spec.rb:3:9"
	cited := examples.request(runDir, inputs, tr.Digest([]byte("options")))

	refused := map[string]func(*tr.Request){
		"relative executable": func(r *tr.Request) { r.Executable = "docker" },
		"relative report dir": func(r *tr.Request) { r.ReportDir = "report" },
		"option selector":     func(r *tr.Request) { r.Selectors = []string{"--require"} },
		"glob selector":       func(r *tr.Request) { r.Selectors = []string{"spec/*_spec.rb"} },
		"parent selector":     func(r *tr.Request) { r.Selectors = []string{"../spec/a_spec.rb"} },
		"option config":       func(r *tr.Request) { r.Config = "--require" },
		"unknown formatter":   func(r *tr.Request) { r.Formatter = "corvint-rspec-json/9" },
		"selection version":   func(r *tr.Request) { r.ExpectedSelection = &tr.Selection{Version: "corvint-test-selection/0"} },
		"selection matcher": func(r *tr.Request) {
			r.ExpectedSelection = &tr.Selection{Version: tr.SelectionVersion, Matcher: tr.NightwatchSessionElided}
		},
		// The two non-member cases are a tripwire: buildRSpec names only dynamic's
		// exec members while the registry names every family's. They match while
		// rspec is the only exec member; rbmutate itself always requests rspec.
		"non-member runner": func(r *tr.Request) { r.Runner = "pytest" },
		"non-member formatter": func(r *tr.Request) {
			r.Runner = "minitest"
			r.ExecPrefix = nil
			r.PathMap = nil
			r.Formatter = "corvint-rspec-json/0"
		},
	}
	cases := map[string]tr.Request{"admitted": admitted, "probe": probe, "cited examples": cited}
	for name, edit := range refused {
		request := admitted
		request.Selectors = append([]string(nil), admitted.Selectors...)
		edit(&request)
		cases[name] = request
	}
	for name, request := range cases {
		t.Run("build/"+name, func(t *testing.T) {
			want, wantErr := registry.Build(request)
			got, gotErr := buildRSpec(request)
			if errText(gotErr) != errText(wantErr) {
				t.Fatalf("error %q, registry %q", errText(gotErr), errText(wantErr))
			}
			if mustJSON(t, got) != mustJSON(t, want) || tr.Identity(got) != tr.Identity(want) {
				t.Fatalf("invocation differs from the registry:\n%s\n%s", mustJSON(t, got), mustJSON(t, want))
			}
			if _, refusal := refused[name]; refusal == (gotErr == nil) {
				t.Fatalf("admission %v for %s", gotErr, name)
			}
		})
	}

	invocation, err := buildRSpec(admitted)
	if err != nil {
		t.Fatal(err)
	}
	report := `{"version":"3.13.0","examples":[` +
		`{"id":"./spec/a_spec.rb[1:1]","description":"adds","full_description":"A adds","status":"passed","file_path":"./spec/a_spec.rb","line_number":3,"run_time":0.001,"pending_message":null},` +
		`{"id":"./spec/a_spec.rb[1:2]","description":"subtracts","full_description":"A subtracts","status":"failed","file_path":"./spec/a_spec.rb","line_number":9,"run_time":0.001,"pending_message":null,"exception":{"class":"RSpec::Expectations::ExpectationNotMetError","message":"expected 1","backtrace":[]}}],` +
		`"summary":{"duration":0.01,"example_count":2,"failure_count":1,"pending_count":0,"errors_outside_of_examples_count":0},"summary_line":"2 examples, 1 failure"}`
	base := tr.Input{
		Runner: "rspec", Selectors: admitted.Selectors, SuccessExitCodes: invocation.SuccessExitCodes, FailureExitCodes: invocation.FailureExitCodes,
		ExitCode: 1, Reports: map[string][]byte{containerReport: []byte(report)}, Stdout: []byte("out"),
	}
	parses := map[string]func(*tr.Input){
		"report":          func(*tr.Input) {},
		"exit mismatch":   func(in *tr.Input) { in.ExitCode = 0 },
		"malformed":       func(in *tr.Input) { in.Reports = map[string][]byte{containerReport: []byte(`{"examples":`)} },
		"no report":       func(in *tr.Input) { in.Reports = nil },
		"timed out":       func(in *tr.Input) { in.TimedOut = true; in.ExitCode = tr.KilledExitCode },
		"overflow":        func(in *tr.Input) { in.Overflow = true },
		"execution issue": func(in *tr.Input) { in.ExecutionProblems = []tr.Problem{{Code: "input-changed", Detail: "lib/a.rb"}} },
	}
	for name, edit := range parses {
		t.Run("parse/"+name, func(t *testing.T) {
			in := base
			edit(&in)
			want, wantErr := registry.Parse(in)
			got, gotErr := parseRSpec(in)
			if errText(gotErr) != errText(wantErr) {
				t.Fatalf("error %q, registry %q", errText(gotErr), errText(wantErr))
			}
			if mustJSON(t, got) != mustJSON(t, want) || tr.Identity(got) != tr.Identity(want) {
				t.Fatalf("observation differs from the registry:\n%s\n%s", mustJSON(t, got), mustJSON(t, want))
			}
			if name == "report" && (!got.Complete || len(got.Tests) != 2) {
				t.Fatalf("the parity fixture is not a complete RSpec report: %+v", got)
			}
		})
	}
}
