package behaviorfalsify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	rspecSetupExample     = "spec/checkout_spec.rb[1:1]"
	rspecTotalExample     = "spec/checkout_spec.rb[1:2]"
	rspecUnrelatedExample = "spec/checkout_spec.rb[1:3]"
	// rspecQualificationImage is the disposable owner-approved image holding the pinned RSpec gems.
	rspecQualificationImage = "corvint-rspec-qual:2026-10-20"
)

// rspecCheckoutSpec is a real RSpec file whose second example fails on CHECKOUT_TOTAL_OFFSET,
// raises outside an expectation on CHECKOUT_RAISE and writes a tracked file on CHECKOUT_WRITE.
const rspecCheckoutSpec = `RSpec.describe "checkout" do
  it("sets up") { expect(1).to eq(1) }
  it("totals") do
    File.write(__FILE__, "# rewritten\n") if ENV["CHECKOUT_WRITE"]
    raise "checkout exploded" if ENV["CHECKOUT_RAISE"]
    expect(ENV.fetch("CHECKOUT_TOTAL_OFFSET", "") + ENV.fetch("CHECKOUT_WRITE", "")).to eq("")
  end
  it("is unrelated") { expect(true).to be(true) }
end
`

// fakeRSpec is a /bin/sh stand-in for the pinned RSpec command: it refuses any argv or forced
// environment other than the binding's and writes the JSON report RSpec's stock formatter would.
const fakeRSpec = `#!/bin/sh
[ "$1 $2 $3 $4" = "--format json --out .corvint-behavior-rspec.json" ] || exit 7
[ "$BUNDLE_FROZEN" = true ] && [ "${SPEC_OPTS-unset}" = "" ] && [ -f spec/checkout_spec.rb ] || exit 8
status=passed exception=""
if [ -n "$CHECKOUT_TOTAL_OFFSET" ]; then status=failed exception=',"exception":{"class":"RSpec::Expectations::ExpectationNotMetError","message":"x"}'; fi
if [ -n "$CHECKOUT_RAISE" ]; then status=failed exception=',"exception":{"class":"RuntimeError","message":"checkout exploded"}'; fi
if [ -n "$CHECKOUT_WRITE" ]; then printf '# rewritten\n' > spec/checkout_spec.rb; status=failed exception=',"exception":{"class":"RSpec::Expectations::ExpectationNotMetError","message":"x"}'; fi
printf '{"version":"3.13.6","examples":[{"id":"./spec/checkout_spec.rb[1:1]","status":"passed"},{"id":"./spec/checkout_spec.rb[1:2]","status":"%s"%s},{"id":"./spec/checkout_spec.rb[1:3]","status":"passed"}],"summary":{"example_count":3,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}}' "$status" "$exception" > .corvint-behavior-rspec.json
[ "$status" = passed ] || exit 1
`

// rspecRequest builds a runner-neutral RSpec request over a committed project whose toolchain is
// the given executable.
func rspecRequest(t *testing.T, toolchain Command, controls []ControlSpec) Request {
	t.Helper()
	request := testRequest(t, controls)
	repository := request.Repositories.Test
	head := commitFixtureFiles(t, repository, map[string]string{"example/.rspec": "--order defined\n", "example/Gemfile": "source \"https://rubygems.org\"\n", "example/spec/checkout_spec.rb": rspecCheckoutSpec})
	request.Target.ApplicationRevision, request.Target.TestRevision, request.Target.DocumentationRevision = head, head, head
	request.Target.TestFile, request.Target.Project = "example/spec/checkout_spec.rb", "example"
	request.Target.CriterionID, request.Target.AssertionID = "criterion:total", rspecTotalExample
	request.DeclaredEnvKeys = []string{helperEnvironment}
	request.Runner = RunnerIdentity{Runner: ProviderRSpec, RunnerVersion: "3.13.6", Browser: "none", BrowserVersion: "none", ConfigFile: "example/.rspec", ConfigSHA256: hashBytes(mustGitBytes(t, repository, "cat-file", "blob", head+":example/.rspec")), EnvironmentSHA256: digestJSON(declaredEnvironment(request.DeclaredEnvKeys))}
	request.Provider = &ProviderSpec{Name: ProviderRSpec, Toolchain: toolchain, Package: "example", Examples: []string{rspecUnrelatedExample, rspecTotalExample, rspecSetupExample}, PerturbationEnvKeys: []string{"CHECKOUT_UNUSED", "CHECKOUT_TOTAL_OFFSET", "CHECKOUT_RAISE", "CHECKOUT_WRITE"}}
	request.TimeoutSeconds, request.WallClockSeconds = 120, 600
	return request
}

func rspecControl(t *testing.T, id, env string) ControlSpec {
	t.Helper()
	control := providerControl(t, id, env)
	control.UnrelatedCriteria, control.RequiredSetup = []string{rspecUnrelatedExample}, []string{rspecSetupExample}
	return control
}

func writeToolchain(t *testing.T, dir, script string) Command {
	t.Helper()
	path := filepath.Join(dir, "rspec")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return Command{Path: path, ExecutableSHA256: mustFileDigest(t, path)}
}

func TestBBFV0029RSpecProviderPlanIsBounded(t *testing.T) {
	t.Parallel()
	toolchain := writeToolchain(t, t.TempDir(), fakeRSpec)
	plan, err := BuildPlan(rspecRequest(t, toolchain, []ControlSpec{rspecControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(plan.Request.Provider.Examples) {
		t.Fatalf("provider examples not canonical: %+v", plan.Request.Provider)
	}
	cases := map[string]func(*Request){
		"runner mismatch": func(r *Request) { r.Runner.Runner = ProviderGoTest },
		"go config":       func(r *Request) { r.Runner.ConfigFile = "example/go.mod" },
		"go test name":    func(r *Request) { r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "TestTotal") },
		"line selector": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "spec/checkout_spec.rb:3")
		},
		"parent component": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "../spec/checkout_spec.rb[1:1]")
		},
		"dot prefix": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "./spec/checkout_spec.rb[1:1]")
		},
		"zero index": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "spec/checkout_spec.rb[1:0]")
		},
		"two ids": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "spec/checkout_spec.rb[1:1,1:2]")
		},
		"not ruby": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "spec/checkout_spec.py[1:1]")
		},
		"oversized": func(r *Request) {
			r.Provider.Examples = append(slices.Clone(r.Provider.Examples), strings.Repeat("a/", 520)+"x.rb[1]")
		},
		"forced bundler key":   func(r *Request) { r.Provider.PerturbationEnvKeys = []string{"BUNDLE_FROZEN"} },
		"forced spec opts key": func(r *Request) { r.Provider.PerturbationEnvKeys = []string{"SPEC_OPTS"} },
		"unselected setup":     func(r *Request) { r.Controls[0].RequiredSetup = []string{"spec/checkout_spec.rb[1:9]"} },
	}
	for name, mutate := range cases {
		t.Run("BBF-V0-029 refuses "+name, func(t *testing.T) {
			t.Parallel()
			candidate := rspecRequest(t, toolchain, []ControlSpec{rspecControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")})
			mutate(&candidate)
			if _, err := BuildPlan(candidate); err == nil {
				t.Fatal("invalid rspec provider request admitted")
			}
		})
	}
	t.Run("BBF-V0-029 Gemfile config is admitted", func(t *testing.T) {
		candidate := rspecRequest(t, toolchain, []ControlSpec{rspecControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")})
		candidate.Runner.ConfigFile = "example/Gemfile"
		candidate.Runner.ConfigSHA256 = hashBytes(mustGitBytes(t, candidate.Repositories.Test, "cat-file", "blob", candidate.Target.TestRevision+":example/Gemfile"))
		if _, err := BuildPlan(candidate); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBBFV0030RSpecProviderEndToEnd(t *testing.T) {
	t.Parallel()
	toolchain := writeToolchain(t, t.TempDir(), fakeRSpec)
	request := rspecRequest(t, toolchain, []ControlSpec{
		rspecControl(t, "kill", "CHECKOUT_TOTAL_OFFSET"),
		rspecControl(t, "raise", "CHECKOUT_RAISE"),
		rspecControl(t, "survive", "CHECKOUT_UNUSED"),
		rspecControl(t, "undeclared", "CHECKOUT_OTHER"),
		rspecControl(t, "write", "CHECKOUT_WRITE"),
	})
	assertRSpecControls(t, request)
}

// assertRSpecControls executes the five standard RSpec controls and checks each classification.
func assertRSpecControls(t *testing.T, request Request) {
	t.Helper()
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		status Status
		reason string
	}{
		"kill":       {StatusKilled, "expected-assertion-failed"},
		"raise":      {StatusInvalidControl, ""},
		"survive":    {StatusSurvived, "target-criterion-survived"},
		"undeclared": {StatusInvalidControl, "perturbation-output-invalid"},
		"write":      {StatusKilled, "expected-assertion-failed"},
	}
	for _, result := range report.Results {
		t.Run("BBF-V0-031 "+result.ID, func(t *testing.T) {
			expected := want[result.ID]
			if result.Status != expected.status || len(result.Attempts) != 1 || (expected.reason != "" && !slices.Contains(result.Attempts[0].Reasons, expected.reason)) {
				t.Fatalf("result = %+v", result)
			}
			if result.ID != "undeclared" && (result.Attempts[0].ProviderProcess == nil || result.Attempts[0].Receipt == nil) {
				t.Fatalf("provider evidence missing: %+v", result.Attempts[0])
			}
		})
	}
	t.Run("BBF-V0-031 an error raised outside an expectation is never an assertion kill", func(t *testing.T) {
		attempt := report.Results[1].Attempts[0]
		if attempt.Receipt == nil || attempt.Receipt.TargetObservation.FailureKind != "panic" {
			t.Fatalf("raise attempt = %+v", attempt)
		}
	})
	t.Run("BBF-V0-030 a spec that writes a tracked file leaves the protected repository unchanged", func(t *testing.T) {
		if report.Provider != ProviderRSpec || !repositoryAtRevision(plan.Request.Repositories.Test, plan.Request.Target.TestRevision) {
			t.Fatalf("provider %q or repository drift", report.Provider)
		}
	})
}

func TestBBFV0031RSpecReportIsClosed(t *testing.T) {
	t.Parallel()
	plan := classificationPlan()
	plan.Request.Target.AssertionID = rspecTotalExample
	control := plan.Controls[0]
	control.UnrelatedCriteria, control.RequiredSetup = []string{rspecUnrelatedExample}, []string{rspecSetupExample}
	plan.Request.Provider = &ProviderSpec{Name: ProviderRSpec, Examples: []string{rspecSetupExample, rspecTotalExample, rspecUnrelatedExample}}
	report := func(count, outside int, examples ...string) []byte {
		return []byte(`{"version":"3.13.6","examples":[` + strings.Join(examples, ",") + `],"summary":{"example_count":` + strconv.Itoa(count) + `,"errors_outside_of_examples_count":` + strconv.Itoa(outside) + `}}`)
	}
	example := func(id, status, class string) string {
		if class == "" {
			return `{"id":"./` + id + `","status":"` + status + `"}`
		}
		return `{"id":"./` + id + `","status":"` + status + `","exception":{"class":"` + class + `"}}`
	}
	cases := map[string]struct {
		report []byte
		reason string
		state  string
		kind   string
	}{
		"expectation":     {report: report(1, 0, example(rspecTotalExample, "failed", "RSpec::Expectations::ExpectationNotMetError")), state: "failed", kind: "assertion"},
		"aggregate":       {report: report(1, 0, example(rspecTotalExample, "failed", "RSpec::Expectations::MultipleExpectationsNotMetError")), state: "failed", kind: "assertion"},
		"mock":            {report: report(1, 0, example(rspecTotalExample, "failed", "RSpec::Mocks::MockExpectationError")), state: "failed", kind: "assertion"},
		"raise":           {report: report(1, 0, example(rspecTotalExample, "failed", "RuntimeError")), state: "failed", kind: "panic"},
		"no exception":    {report: report(1, 0, example(rspecTotalExample, "failed", "")), state: "failed", kind: "panic"},
		"passed":          {report: report(1, 0, example(rspecTotalExample, "passed", "")), state: "passed"},
		"pending":         {report: report(1, 0, example(rspecTotalExample, "pending", "")), state: "skipped"},
		"missing":         {report: report(1, 0, example(rspecSetupExample, "passed", "")), state: "not_run"},
		"outside error":   {report: report(0, 1), reason: "provider-build-failed"},
		"absent":          {report: nil, reason: "provider-output-invalid"},
		"not json":        {report: []byte("3 examples, 0 failures"), reason: "provider-output-invalid"},
		"no examples":     {report: []byte(`{"summary":{"example_count":0,"errors_outside_of_examples_count":0}}`), reason: "provider-output-invalid"},
		"no summary":      {report: []byte(`{"examples":[]}`), reason: "provider-output-invalid"},
		"unknown status":  {report: report(1, 0, example(rspecTotalExample, "errored", "")), reason: "provider-output-invalid"},
		"duplicate":       {report: report(2, 0, example(rspecTotalExample, "passed", ""), example(rspecTotalExample, "failed", "RuntimeError")), reason: "provider-output-contradictory"},
		"unselected":      {report: report(1, 0, example("spec/checkout_spec.rb[1:9]", "passed", "")), reason: "provider-output-contradictory"},
		"count mismatch":  {report: report(3, 0, example(rspecTotalExample, "passed", "")), reason: "provider-output-contradictory"},
		"line-style id":   {report: report(1, 0, `{"id":"./spec/checkout_spec.rb:3","status":"failed"}`), reason: "provider-output-contradictory"},
		"stripped prefix": {report: report(1, 0, `{"id":"`+rspecTotalExample+`","status":"failed","exception":{"class":"RSpec::Expectations::ExpectationNotMetError"}}`), state: "failed", kind: "assertion"},
	}
	for name, tc := range cases {
		t.Run("BBF-V0-031 "+name, func(t *testing.T) {
			receipt, reason := providerReceipt(plan, control, 1, tc.report)
			if reason != tc.reason || (reason == "" && (receipt.TargetObservation.State != tc.state || receipt.TargetObservation.FailureKind != tc.kind || receipt.NativeReceiptSHA256 != digestBytes(tc.report))) {
				t.Fatalf("receipt = %+v, reason = %q", receipt.TargetObservation, reason)
			}
		})
	}
}

func TestBBFV0032RSpecEvidenceReceiptIsRefused(t *testing.T) {
	t.Parallel()
	toolchain := writeToolchain(t, t.TempDir(), fakeRSpec)
	plan, err := BuildPlan(rspecRequest(t, toolchain, []ControlSpec{rspecControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")}))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("BBF-V0-032 execute-receipt fails closed for the RSpec provider", func(t *testing.T) {
		if _, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool()); err == nil || err.Error() != "receipt-profile-unsupported" {
			t.Fatalf("err = %v", err)
		}
	})
}

// TestBBFV0033LiveRSpecNegativeControls runs the RSpec negative controls with the real pinned
// RSpec gems inside the owner-approved disposable image, through a docker-prefixed toolchain.
func TestBBFV0033LiveRSpecNegativeControls(t *testing.T) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("BBF-V0-033 live RSpec controls need docker on PATH")
	}
	if exec.Command(docker, "image", "inspect", rspecQualificationImage).Run() != nil {
		t.Skipf("BBF-V0-033 live RSpec controls need the %s image", rspecQualificationImage)
	}
	host, err := exec.Command(docker, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		t.Skipf("BBF-V0-033 docker context unavailable: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("BBF-V0-033 home directory unavailable")
	}
	// Desktop docker VMs share the home directory, not the system temporary directory, so the
	// provider scratch and its staged project live under a disposable home cache directory.
	if err := os.MkdirAll(filepath.Join(home, ".cache"), 0o700); err != nil {
		t.Skipf("BBF-V0-033 home cache unavailable: %v", err)
	}
	scratch, err := os.MkdirTemp(filepath.Join(home, ".cache"), "corvint-live-bbf-")
	if err != nil {
		t.Skipf("BBF-V0-033 home cache unavailable: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(scratch) })
	t.Setenv("TMPDIR", scratch)
	wrapper := "#!/bin/sh\nexec " + shellQuote(docker) + " -H " + shellQuote(strings.TrimSpace(string(host))) +
		` run --rm --network none -e BUNDLE_FROZEN -e SPEC_OPTS -e CHECKOUT_TOTAL_OFFSET -e CHECKOUT_RAISE -e CHECKOUT_UNUSED -e CHECKOUT_WRITE -v "$PWD:$PWD" -w "$PWD" ` +
		rspecQualificationImage + " rspec \"$@\"\n"
	toolchain := writeToolchain(t, t.TempDir(), wrapper)
	request := rspecRequest(t, toolchain, []ControlSpec{
		rspecControl(t, "kill", "CHECKOUT_TOTAL_OFFSET"),
		rspecControl(t, "raise", "CHECKOUT_RAISE"),
		rspecControl(t, "survive", "CHECKOUT_UNUSED"),
		rspecControl(t, "undeclared", "CHECKOUT_OTHER"),
		rspecControl(t, "write", "CHECKOUT_WRITE"),
	})
	assertRSpecControls(t, request)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
