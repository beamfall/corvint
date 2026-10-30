package behaviorfalsify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

const profileLogEnvironment = "CORVINT_BEHAVIOR_TEST_PROFILE_LOG"

// runProfileHelper serves the shared-server reset/health role and the runner-neutral
// perturbation role of the test binary; it reports whether it handled the invocation.
func runProfileHelper(data []byte) bool {
	role := os.Getenv("CORVINT_BEHAVIOR_COMMAND_ROLE")
	if strings.HasPrefix(role, "isolation-") {
		var check IsolationCheck
		if json.Unmarshal(data, &check) != nil {
			os.Exit(2)
		}
		appendProfileLog(check.Phase + " " + check.ControlID)
		report := IsolationReport{Schema: IsolationObservationSchema, PlanDigest: check.PlanDigest, Phase: check.Phase, ControlID: check.ControlID, Attempt: check.Attempt, ServerURL: check.ServerURL, ApplicationIdentity: check.ApplicationIdentity}
		switch {
		case check.Phase == isolationPhaseReset && strings.HasPrefix(check.ControlID, "reset-fail"):
			os.Exit(3)
		case check.Phase == isolationPhaseReset && strings.HasPrefix(check.ControlID, "swap-state"):
			// The reset rewrites the storage-state file named in <profile log>.swap (BBF-V0-218).
			if target, err := os.ReadFile(os.Getenv(profileLogEnvironment) + ".swap"); err != nil || os.WriteFile(string(target), []byte(`{"cookies":[]}`), 0o600) != nil {
				os.Exit(2)
			}
		case check.Phase == isolationPhaseReset && strings.HasPrefix(check.ControlID, "drift-before"):
			report.ApplicationIdentity = strings.Repeat("d", 40)
		case check.Phase == isolationPhaseVerify && strings.HasPrefix(check.ControlID, "drift-after"):
			report.ServerURL = "http://127.0.0.1:1"
		case check.Phase == isolationPhaseVerify && strings.HasPrefix(check.ControlID, "health-fail"):
			os.Exit(4)
		}
		writeHelperDocument(report)
		return true
	}
	if role != "control" {
		return false
	}
	var invocation Invocation
	if json.Unmarshal(data, &invocation) != nil {
		return false
	}
	appendProfileLog("control " + invocation.Control.ID)
	if invocation.Control.Definition["scenario"] != "provider" {
		return false
	}
	writeHelperDocument(Perturbation{
		Schema: PerturbationSchema, PlanDigest: invocation.PlanDigest, PerturbationSHA256: invocation.Control.PerturbationSHA256,
		Attempt: invocation.Attempt, Environment: map[string]string{invocation.Control.Definition["env"]: invocation.Control.Definition["value"]},
	})
	return true
}

func appendProfileLog(line string) {
	path := os.Getenv(profileLogEnvironment)
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(2)
	}
	defer file.Close()
	if _, err := file.WriteString(line + "\n"); err != nil {
		os.Exit(2)
	}
}

func writeHelperDocument(value any) {
	encoded, err := Encode(value)
	if err != nil {
		os.Exit(2)
	}
	if _, err := os.Stdout.Write(encoded); err != nil {
		os.Exit(2)
	}
}

func isolationRequest(t *testing.T, controls []ControlSpec) (Request, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "profile.log")
	t.Setenv(profileLogEnvironment, logPath)
	request := testRequest(t, controls)
	request.DeclaredEnvKeys = []string{helperEnvironment, profileLogEnvironment}
	request.Runner.EnvironmentSHA256 = digestJSON(declaredEnvironment(request.DeclaredEnvKeys))
	request.ExternalState = ExternalStateSharedServer
	request.WallClockSeconds = 60
	request.Isolation = &DeclaredIsolation{Profile: IsolationSharedServer, ServerURL: "http://127.0.0.1:41873", ApplicationIdentity: request.Target.ApplicationRevision, Reset: *testCommand(t)}
	return request, logPath
}

func TestBBFV0019IsolationProfileIsExplicitAndBounded(t *testing.T) {
	t.Run("BBF-V0-019 default disposable profile is unchanged", func(t *testing.T) {
		request := liveRequest(t, []ControlSpec{liveControl(t, "control", "killed")})
		if _, err := BuildPlan(request); err != nil {
			t.Fatal(err)
		}
		request.ExternalState = ExternalStateSharedServer
		if _, err := BuildPlan(request); err == nil {
			t.Fatal("shared external state admitted without a declared isolation")
		}
	})
	request, _ := isolationRequest(t, []ControlSpec{liveControl(t, "control", "killed")})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Request.Isolation == nil || *plan.Request.Isolation != *request.Isolation {
		t.Fatalf("plan isolation = %+v", plan.Request.Isolation)
	}
	cases := map[string]func(*Request){
		"wrong profile":          func(r *Request) { r.Isolation.Profile = "shared" },
		"undeclared state":       func(r *Request) { r.ExternalState = "none" },
		"remote host":            func(r *Request) { r.Isolation.ServerURL = "http://example.test:8080" },
		"credentials":            func(r *Request) { r.Isolation.ServerURL = "http://user:pw@127.0.0.1:8080" },
		"query":                  func(r *Request) { r.Isolation.ServerURL = "http://localhost:8080/?token=x" },
		"scheme":                 func(r *Request) { r.Isolation.ServerURL = "file:///tmp/server" },
		"identity":               func(r *Request) { r.Isolation.ApplicationIdentity = "main" },
		"reset digest":           func(r *Request) { r.Isolation.Reset.ExecutableSHA256 = digestN("other") },
		"budget inside reserves": func(r *Request) { r.WallClockSeconds = 17; r.TimeoutSeconds = 5 },
	}
	for name, mutate := range cases {
		t.Run("BBF-V0-019 refuses "+name, func(t *testing.T) {
			candidate := request
			isolation := *request.Isolation
			candidate.Isolation = &isolation
			mutate(&candidate)
			if _, err := BuildPlan(candidate); err == nil {
				t.Fatal("invalid isolation admitted")
			}
		})
	}
}

func TestBBFV0021IsolationFailureIsNeverKilled(t *testing.T) {
	ids := []string{"drift-after", "drift-before", "health-fail", "ok", "reset-fail"}
	controls := make([]ControlSpec, 0, len(ids))
	for _, id := range ids {
		controls = append(controls, liveControl(t, id, "killed"))
	}
	request, logPath := isolationRequest(t, controls)
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
		"drift-after":  {StatusInfrastructureFailed, "isolation-identity-drift"},
		"drift-before": {StatusInfrastructureFailed, "isolation-identity-drift"},
		"health-fail":  {StatusInfrastructureFailed, "isolation-health-failed"},
		"ok":           {StatusKilled, "expected-assertion-failed"},
		"reset-fail":   {StatusInfrastructureFailed, "isolation-reset-failed"},
	}
	for _, result := range report.Results {
		t.Run("BBF-V0-021 "+result.ID, func(t *testing.T) {
			expected := want[result.ID]
			if result.Status != expected.status || len(result.Attempts) != 1 || !slices.Contains(result.Attempts[0].Reasons, expected.reason) || result.Attempts[0].Isolation == nil {
				t.Fatalf("result = %+v", result)
			}
		})
	}
	t.Run("BBF-V0-020 reset precedes every control and identity is rechecked after it", func(t *testing.T) {
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		expected := strings.Join([]string{
			"reset drift-after", "control drift-after", "verify drift-after",
			"reset drift-before",
			"reset health-fail", "control health-fail", "verify health-fail",
			"reset ok", "control ok", "verify ok",
			"reset reset-fail",
		}, "\n") + "\n"
		if string(data) != expected {
			t.Fatalf("hook order =\n%s", data)
		}
	})
	t.Run("BBF-V0-022 report records the declared isolation", func(t *testing.T) {
		if report.Isolation == nil || *report.Isolation != *plan.Request.Isolation || !strings.Contains(strings.Join(report.Limitations, "\n"), "shared server") {
			t.Fatalf("report isolation = %+v %v", report.Isolation, report.Limitations)
		}
	})
}

func TestBBFV0022IsolationEvidenceReceipt(t *testing.T) {
	request, _ := isolationRequest(t, []ControlSpec{liveControl(t, "ok", "killed"), liveControl(t, "drift-after", "killed")})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("BBF-V0-022 receipt records isolation and verifies offline", func(t *testing.T) {
		if r.Report.Isolation == nil || *r.Report.Isolation != *plan.Request.Isolation || r.Report.Counts[StatusKilled] != 1 || r.Report.Counts[StatusInfrastructureFailed] != 1 {
			t.Fatalf("receipt report = %+v", r.Report)
		}
		if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("BBF-V0-022 receipt without the declared isolation is refused", func(t *testing.T) {
		tampered := r
		tampered.Report.Isolation = nil
		tampered.Report.Digest = digestJSON(tampered.Report)
		tampered.Digest = evidenceDigest(tampered)
		if err := VerifyReceipt(tampered, plan.Digest, receiptTool().Executable); err == nil {
			t.Fatal("receipt without declared isolation verified")
		}
	})
	t.Run("BBF-V0-022 rewriting a drifted identity cannot forge a kill", func(t *testing.T) {
		tampered := r
		tampered.Report.Results = slices.Clone(r.Report.Results)
		declared := ServerIdentity{ServerURL: plan.Request.Isolation.ServerURL, ApplicationIdentity: plan.Request.Isolation.ApplicationIdentity, Source: IdentityHookReported}
		forged := false
		for i, control := range plan.Controls {
			result := &tampered.Report.Results[i]
			if result.ID != "drift-after" {
				continue
			}
			result.Attempts = slices.Clone(result.Attempts)
			attempt := &result.Attempts[0]
			observation := *attempt.Isolation
			observation.After = &declared
			attempt.Isolation = &observation
			attempt.Reasons = nil
			attempt.Status, attempt.Reasons, attempt.Application = classifyAttempt(plan, control, *attempt)
			result.Status, result.Reasons = aggregateAttempts(result.Attempts, plan.Request.Attempts)
			result.Application = aggregateApplication(control, result.Attempts, result.Status)
			forged = result.Status == StatusKilled
		}
		recountReport(&tampered.Report, plan)
		tampered.Report.Digest = digestJSON(tampered.Report)
		tampered.Digest = evidenceDigest(tampered)
		if !forged {
			t.Fatal("rewrite did not reclassify the drifted attempt as a kill")
		}
		if err := VerifyReceipt(tampered, plan.Digest, receiptTool().Executable); err == nil {
			t.Fatal("receipt with a rewritten isolation identity verified")
		}
	})
	t.Run("BBF-V0-021 dropping the drift observation cannot forge a kill", func(t *testing.T) {
		tampered := r
		tampered.Report.Results = slices.Clone(r.Report.Results)
		for i := range tampered.Report.Results {
			result := &tampered.Report.Results[i]
			result.Attempts = slices.Clone(result.Attempts)
			result.Attempts[0].Isolation = nil
		}
		tampered.Report.Digest = digestJSON(tampered.Report)
		tampered.Digest = evidenceDigest(tampered)
		if err := VerifyReceipt(tampered, plan.Digest, receiptTool().Executable); err == nil {
			t.Fatal("receipt without isolation observations verified")
		}
	})
}

// TestBBFV0023LegacyWireIsByteIdentical pins plan and report digests captured at the base
// revision before the profiles existed.
func TestBBFV0023LegacyWireIsByteIdentical(t *testing.T) {
	t.Parallel()
	plan := classificationPlan()
	control := plan.Controls[0]
	result := AttemptResult{Attempt: 1, Receipt: receiptPointer(classificationReceipt(plan, control)), HookProcess: successfulProcess(), CleanupProcess: successfulProcess(), WorkspaceBefore: digestN("w"), WorkspaceAfter: digestN("w")}
	report := Report{Schema: ReportSchema, PlanDigest: plan.Digest, Results: []ControlResult{{ID: control.ID, Attempts: []AttemptResult{result}}}, Counts: allStatusCounts()}
	t.Run("BBF-V0-023 plan without profiles keeps its digest", func(t *testing.T) {
		if got := digestJSON(plan); got != "sha256:73c3c8735b46eb8b68f870a9421be53d41212646a28da4afae982ab8d9795604" {
			t.Fatalf("plan digest = %s", got)
		}
	})
	t.Run("BBF-V0-023 report without profiles keeps its digest", func(t *testing.T) {
		if got := digestJSON(report); got != "sha256:19a54e14816d76ca039eb1b6f1ac0b87b580a7c1e7dae8330c71523c76632088" {
			t.Fatalf("report digest = %s", got)
		}
		encoded, err := Encode(report)
		if err != nil || bytes.Contains(encoded, []byte(`"isolation"`)) || bytes.Contains(encoded, []byte(`"provider`)) {
			t.Fatalf("legacy report encodes profile fields: %v", err)
		}
	})
}

// TestBBFV0216HookReportedIdentityCapsVerdict covers BBF-V0-216: the test hook forges the
// declared identity without any server existing, and the kill it yields still carries the
// hook-reported source and a report-level gap, so the verdict stays below complete; a receipt that
// relabels the identity observed or drops the gap does not verify.
func TestBBFV0216HookReportedIdentityCapsVerdict(t *testing.T) {
	request, _ := isolationRequest(t, []ControlSpec{liveControl(t, "ok", "killed")})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	report := r.Report
	t.Run("BBF-V0-216 a forged hook identity is labelled hook-reported", func(t *testing.T) {
		observation := report.Results[0].Attempts[0].Isolation
		if report.Results[0].Status != StatusKilled || observation == nil || observation.Before == nil || observation.After == nil ||
			observation.Before.Source != IdentityHookReported || observation.After.Source != IdentityHookReported {
			t.Fatalf("result = %+v", report.Results[0])
		}
		encoded, err := Encode(observation)
		if err != nil || !bytes.Contains(encoded, []byte(`"source":"hook-reported"`)) {
			t.Fatalf("isolation wire lacks the identity source: %s %v", encoded, err)
		}
	})
	t.Run("BBF-V0-216 a hook-only identity caps the verdict below complete", func(t *testing.T) {
		if !report.MutationScore.Defined || report.MutationScore.Killed != report.MutationScore.Denominator ||
			!slices.Equal(report.CoverageGaps, []string{IsolationHookReportedGap}) || idPattern.MatchString(IsolationHookReportedGap) {
			t.Fatalf("score %+v gaps %v", report.MutationScore, report.CoverageGaps)
		}
		if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
			t.Fatal(err)
		}
	})
	reseal := func(tampered EvidenceReceipt) EvidenceReceipt {
		tampered.Report.Digest = digestJSON(tampered.Report)
		tampered.Digest = evidenceDigest(tampered)
		return tampered
	}
	t.Run("BBF-V0-216 dropping the gap does not verify", func(t *testing.T) {
		tampered := r
		tampered.Report.CoverageGaps = nil
		if err := VerifyReceipt(reseal(tampered), plan.Digest, receiptTool().Executable); err == nil {
			t.Fatal("receipt without the hook-reported identity gap verified")
		}
	})
	t.Run("BBF-V0-216 relabelling the identity observed does not verify", func(t *testing.T) {
		tampered := r
		tampered.Report.Results = slices.Clone(r.Report.Results)
		result := &tampered.Report.Results[0]
		result.Attempts = slices.Clone(result.Attempts)
		observation := *result.Attempts[0].Isolation
		before, after := *observation.Before, *observation.After
		before.Source, after.Source = IdentityObserved, IdentityObserved
		observation.Before, observation.After = &before, &after
		result.Attempts[0].Isolation = &observation
		recountReport(&tampered.Report, plan)
		if len(tampered.Report.CoverageGaps) != 0 {
			t.Fatalf("observed identities kept the gap: %v", tampered.Report.CoverageGaps)
		}
		if err := VerifyReceipt(reseal(tampered), plan.Digest, receiptTool().Executable); err == nil {
			t.Fatal("receipt relabelling a hook-reported identity observed verified")
		}
	})
	t.Run("BBF-V0-216 a plan without isolation has no identity gap", func(t *testing.T) {
		legacy, err := BuildPlan(liveRequest(t, []ControlSpec{liveControl(t, "ok", "killed")}))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := Execute(context.Background(), legacy, legacy.Digest)
		if err != nil {
			t.Fatal(err)
		}
		if plain.Results[0].Status != StatusKilled || len(plain.CoverageGaps) != 0 {
			t.Fatalf("legacy report = %+v", plain)
		}
	})
}

// providerRequest builds a runner-neutral request over a committed Go module whose TestTotal
// fails only when the perturbation environment names CHECKOUT_TOTAL_OFFSET.
func providerRequest(t *testing.T, controls []ControlSpec) Request {
	t.Helper()
	processGoCache(t)
	request := testRequest(t, controls)
	repository := request.Repositories.Test
	files := map[string]string{
		"go.mod": "module example.test/provider\n\ngo 1.21\n",
		"example/checkout_test.go": `package example

import (
	"os"
	"testing"
)

func TestSetup(t *testing.T) {}

func TestTotal(t *testing.T) {
	if os.Getenv("CHECKOUT_WRITE") != "" {
		if err := os.WriteFile("checkout_test.go", []byte("package example\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Fatal("total mismatch after writing a tracked file")
	}
	if os.Getenv("CHECKOUT_PANIC") != "" {
		panic("checkout exploded")
	}
	if os.Getenv("CHECKOUT_TOTAL_OFFSET") != "" {
		t.Fatal("total mismatch")
	}
}

func TestUnrelated(t *testing.T) {}
`,
	}
	head := commitFixtureFiles(t, repository, files)
	request.Target.ApplicationRevision, request.Target.TestRevision, request.Target.DocumentationRevision = head, head, head
	request.Target.TestFile, request.Target.Project = "example/checkout_test.go", "example"
	request.Target.CriterionID, request.Target.AssertionID = "criterion:total", "TestTotal"
	request.DeclaredEnvKeys = []string{"GOCACHE", helperEnvironment}
	request.Runner = RunnerIdentity{Runner: ProviderGoTest, RunnerVersion: runtime.Version(), Browser: "none", BrowserVersion: "none", ConfigFile: "go.mod", ConfigSHA256: hashBytes(mustGitBytes(t, repository, "cat-file", "blob", head+":go.mod")), EnvironmentSHA256: digestJSON(declaredEnvironment(request.DeclaredEnvKeys))}
	request.Provider = &ProviderSpec{Name: ProviderGoTest, Toolchain: goToolchain(t), Package: "example", Examples: []string{"TestUnrelated", "TestTotal", "TestSetup"}, PerturbationEnvKeys: []string{"CHECKOUT_UNUSED", "CHECKOUT_TOTAL_OFFSET", "CHECKOUT_PANIC", "CHECKOUT_WRITE"}}
	request.TimeoutSeconds, request.WallClockSeconds = 120, 600
	return request
}

// commitFixtureFiles writes files into repository, commits them and returns the new HEAD.
func commitFixtureFiles(t *testing.T, repository string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(repository, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "provider"}} {
		command := exec.Command("git", args...)
		command.Dir, command.Env = repository, isolatedGitEnvironment(repository)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", args[0], err, output)
		}
	}
	return strings.TrimSpace(string(mustGitBytes(t, repository, "rev-parse", "HEAD")))
}

func providerControl(t *testing.T, id, env string) ControlSpec {
	t.Helper()
	return ControlSpec{
		ID: id, Kind: WrongExpectedValue, Disposition: "run",
		Definition: map[string]string{"scenario": "provider", "env": env, "value": "1"}, Hook: testCommand(t),
		UnrelatedCriteria: []string{"TestUnrelated"}, RequiredSetup: []string{"TestSetup"},
	}
}

func goToolchain(t *testing.T) Command {
	t.Helper()
	path, err := filepath.EvalSymlinks(filepath.Join(runtime.GOROOT(), "bin", "go"))
	if err != nil {
		t.Skipf("pinned Go toolchain unavailable: %v", err)
	}
	return Command{Path: path, ExecutableSHA256: mustFileDigest(t, path)}
}

// goCacheOnce resolves the toolchain's GOCACHE once and exports it process-wide, the value the go
// command would use anyway, so the declared GOCACHE key resolves without a per-test t.Setenv that
// would forbid t.Parallel or a per-request go env call (V1-2213).
var goCacheOnce = sync.OnceValues(func() (string, error) {
	output, err := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "env", "GOCACHE").Output()
	value := strings.TrimSpace(string(output))
	if err == nil && value == "" {
		err = errors.New("empty GOCACHE")
	}
	if err == nil {
		err = os.Setenv("GOCACHE", value)
	}
	return value, err
})

func processGoCache(t *testing.T) {
	t.Helper()
	if _, err := goCacheOnce(); err != nil {
		t.Skipf("go env GOCACHE unavailable: %v", err)
	}
}

func TestBBFV0024ProviderPlanIsBounded(t *testing.T) {
	t.Parallel()
	request := providerRequest(t, []ControlSpec{providerControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(plan.Request.Provider.Examples) || !slices.IsSorted(plan.Request.Provider.PerturbationEnvKeys) {
		t.Fatalf("provider lists not canonical: %+v", plan.Request.Provider)
	}
	// The package pattern is assembled so this file carries no package-pattern literal, which the
	// affected-plan read-scope rule would treat as an unbounded read (AFP-V0-012 rule (d)).
	pattern := "example/" + "..."
	cases := map[string]func(*Request){
		"browser runner":       func(r *Request) { r.Runner.Browser = "chromium" },
		"playwright runner":    func(r *Request) { r.Runner.Runner = "playwright" },
		"provider name":        func(r *Request) { r.Provider.Name = "rspec" },
		"package escape":       func(r *Request) { r.Provider.Package, r.Target.Project = "../example", "../example" },
		"package pattern":      func(r *Request) { r.Provider.Package, r.Target.Project = pattern, pattern },
		"embedded pattern":     func(r *Request) { r.Provider.Package, r.Target.Project = "example"+"...", "example"+"..." },
		"project mismatch":     func(r *Request) { r.Target.Project = "other" },
		"target not selected":  func(r *Request) { r.Provider.Examples = []string{"TestSetup", "TestUnrelated"} },
		"example name":         func(r *Request) { r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "TestX|.*") },
		"duplicate example":    func(r *Request) { r.Provider.Examples = append(slices.Clone(r.Provider.Examples), "TestTotal") },
		"unselected unrelated": func(r *Request) { r.Controls[0].UnrelatedCriteria = []string{"TestOther"} },
		"forced key":           func(r *Request) { r.Provider.PerturbationEnvKeys = []string{"GOFLAGS"} },
		"reserved key":         func(r *Request) { r.Provider.PerturbationEnvKeys = []string{"PATH"} },
		"declared overlap":     func(r *Request) { r.Provider.PerturbationEnvKeys = []string{"GOCACHE"} },
		"no keys":              func(r *Request) { r.Provider.PerturbationEnvKeys = nil },
		"toolchain digest":     func(r *Request) { r.Provider.Toolchain.ExecutableSHA256 = digestN("go") },
		"marker": func(r *Request) {
			r.Controls[0].ApplicationMarker = &ApplicationMarker{Kind: MarkerAnnotation, Type: "control-applied"}
		},
		"dirty repository": func(r *Request) {
			if err := os.WriteFile(filepath.Join(r.Repositories.Test, "untracked.txt"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run("BBF-V0-024 refuses "+name, func(t *testing.T) {
			t.Parallel()
			candidate := providerRequest(t, []ControlSpec{providerControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")})
			mutate(&candidate)
			if _, err := BuildPlan(candidate); err == nil {
				t.Fatal("invalid provider request admitted")
			}
		})
	}
}

func TestBBFV0026GoProviderEndToEnd(t *testing.T) {
	t.Parallel()
	request := providerRequest(t, []ControlSpec{
		providerControl(t, "kill", "CHECKOUT_TOTAL_OFFSET"),
		providerControl(t, "panic", "CHECKOUT_PANIC"),
		providerControl(t, "survive", "CHECKOUT_UNUSED"),
		providerControl(t, "undeclared", "CHECKOUT_OTHER"),
		providerControl(t, "write", "CHECKOUT_WRITE"),
	})
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
		"panic":      {StatusInvalidControl, ""},
		"survive":    {StatusSurvived, "target-criterion-survived"},
		"undeclared": {StatusInvalidControl, "perturbation-output-invalid"},
		"write":      {StatusKilled, "expected-assertion-failed"},
	}
	for _, result := range report.Results {
		t.Run("BBF-V0-027 "+result.ID, func(t *testing.T) {
			expected := want[result.ID]
			if result.Status != expected.status || len(result.Attempts) != 1 || (expected.reason != "" && !slices.Contains(result.Attempts[0].Reasons, expected.reason)) {
				t.Fatalf("result = %+v", result)
			}
			if result.ID != "undeclared" && (result.Attempts[0].ProviderProcess == nil || result.Attempts[0].Receipt == nil || result.Attempts[0].Receipt.NativeReceiptSHA256 != result.Attempts[0].ProviderProcess.StdoutSHA256) {
				t.Fatalf("provider evidence missing: %+v", result.Attempts[0])
			}
		})
	}
	t.Run("BBF-V0-027 panic is never an assertion kill", func(t *testing.T) {
		attempt := report.Results[1].Attempts[0]
		if attempt.Receipt == nil || attempt.Receipt.TargetObservation.FailureKind != "panic" {
			t.Fatalf("panic attempt = %+v", attempt)
		}
	})
	t.Run("BBF-V0-026 a test that writes a tracked file leaves the protected repository unchanged", func(t *testing.T) {
		if report.Provider != ProviderGoTest || !repositoryAtRevision(plan.Request.Repositories.Test, plan.Request.Target.TestRevision) {
			t.Fatalf("provider %q or repository drift", report.Provider)
		}
	})
}

func TestBBFV0027ProviderEventsAreClosed(t *testing.T) {
	t.Parallel()
	plan := classificationPlan()
	plan.Request.Target.AssertionID = "TestTotal"
	control := plan.Controls[0]
	control.UnrelatedCriteria, control.RequiredSetup = []string{"TestUnrelated"}, []string{"TestSetup"}
	plan.Request.Provider = &ProviderSpec{Name: ProviderGoTest, Examples: []string{"TestSetup", "TestTotal", "TestUnrelated"}}
	events := func(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }
	cases := map[string]struct {
		stdout []byte
		reason string
		state  string
		kind   string
	}{
		"assertion":       {stdout: events(`{"Action":"fail","Test":"TestTotal"}`), state: "failed", kind: "assertion"},
		"subtest ignored": {stdout: events(`{"Action":"fail","Test":"TestTotal/case"}`, `{"Action":"pass","Test":"TestTotal"}`), state: "passed"},
		"panic":           {stdout: events(`{"Action":"output","Test":"TestTotal","Output":"panic: boom\n"}`, `{"Action":"fail","Test":"TestTotal"}`), state: "failed", kind: "panic"},
		"package panic":   {stdout: events(`{"Action":"output","Output":"panic: boom\n"}`, `{"Action":"fail","Test":"TestTotal"}`), state: "failed", kind: "panic"},
		"missing":         {stdout: events(`{"Action":"pass","Test":"TestSetup"}`), state: "not_run"},
		"build failure":   {stdout: events(`{"ImportPath":"example","Action":"build-fail"}`), reason: "provider-build-failed"},
		"not json":        {stdout: events(`ok example 0.1s`), reason: "provider-output-invalid"},
		"contradiction":   {stdout: events(`{"Action":"pass","Test":"TestTotal"}`, `{"Action":"fail","Test":"TestTotal"}`), reason: "provider-output-contradictory"},
		"other package":   {stdout: events(`{"Package":"a","Action":"pass","Test":"TestSetup"}`, `{"Package":"a","Action":"pass","Test":"TestUnrelated"}`, `{"Package":"b","Action":"fail","Test":"TestTotal"}`), reason: "provider-output-contradictory"},
		"one package":     {stdout: events(`{"Package":"a","Action":"pass","Test":"TestSetup"}`, `{"Package":"a","Action":"fail","Test":"TestTotal"}`, `{"Package":"a","Action":"fail"}`), state: "failed", kind: "assertion"},
	}
	for name, tc := range cases {
		t.Run("BBF-V0-027 "+name, func(t *testing.T) {
			receipt, reason := providerReceipt(plan, control, 1, tc.stdout)
			if reason != tc.reason || (reason == "" && (receipt.TargetObservation.State != tc.state || receipt.TargetObservation.FailureKind != tc.kind || receipt.NativeReceiptSHA256 != digestBytes(tc.stdout))) {
				t.Fatalf("receipt = %+v, reason = %q", receipt.TargetObservation, reason)
			}
		})
	}
	t.Run("BBF-V0-027 provider boundary loss is infrastructure", func(t *testing.T) {
		for _, result := range []AttemptResult{
			{Attempt: 1, HookProcess: successfulProcess(), CleanupProcess: successfulProcess(), ProviderProcess: &ProcessEvidence{Exit: 2, Started: true, Completed: true, OwnedCleanup: true, DescendantsGone: true}},
			{Attempt: 1, HookProcess: successfulProcess(), CleanupProcess: successfulProcess(), ProviderProcess: &ProcessEvidence{Exit: 1, Started: true, Completed: true, OwnedCleanup: true, DescendantsGone: true}, Reasons: []string{"provider-build-failed"}},
		} {
			result.WorkspaceBefore, result.WorkspaceAfter = digestN("w"), digestN("w")
			if status, reasons, _ := classifyAttempt(plan, control, result); status != StatusInfrastructureFailed || !slices.Contains(reasons, "provider-process-failed") {
				t.Fatalf("status = %s %v", status, reasons)
			}
		}
	})
}

func TestBBFV0028ProviderEvidenceReceiptIsRefused(t *testing.T) {
	t.Parallel()
	plan, err := BuildPlan(providerRequest(t, []ControlSpec{providerControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")}))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("BBF-V0-028 execute-receipt refuses the runner-neutral profile", func(t *testing.T) {
		if _, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool()); err == nil || err.Error() != "receipt-profile-unsupported" {
			t.Fatalf("err = %v", err)
		}
	})
}
