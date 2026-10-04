package testacceptance

import (
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// PTF-V0-004/006/008: an early actual serving mismatch has no native test row;
// missing later observations cannot erase it. Legacy requests stay separate.
func TestPTFV0EarlyStaleWithoutRow(t *testing.T) {
	source := []byte("current source")
	config := jstestprovider.FreshnessConfig{ArtifactPath: "app.html", SourceDigest: strings.TrimPrefix(Hash(source), "sha256:"), DocumentURL: "http://127.0.0.1:4394/"}
	native := jstestprovider.Receipt{Profile: jstestprovider.FreshnessProfile, Freshness: &jstestprovider.FreshnessBinding{ArtifactPath: config.ArtifactPath, Source: source, SourceDigest: config.SourceDigest, DocumentURL: config.DocumentURL, ArtifactAfter: strings.Repeat("b", 64)}}
	raw, e := jstestprovider.EncodeFreshness(native)
	if e != nil {
		t.Fatal(e)
	}
	request := Request{Schema: FreshRequestSchema, Repeat: 2, Tests: []Test{{ID: "counter"}}, Freshness: &FreshnessOptions{Provider: config}}
	report := Report{Runs: []Run{{Kind: "repeat", Ordinal: 1, NativeReceipt: raw, ReceiptSHA256: Hash(raw)}, {Kind: "repeat", Ordinal: 2}}}
	classify(&report, request)
	if report.Verdict != "rejected" || report.Assessments[0].Validity.Freshness.State != testvalidity.FreshnessStale {
		t.Fatalf("observed stale lost: %+v", report)
	}
	request.Schema = RequestSchema
	if Validate(request) == nil {
		t.Fatal("legacy request admitted freshness fields")
	}
}

// These synthetic transcripts exercise the pure join, not actual execution.
// The live tests separately retain real N2 and standalone CLI witnesses.
func parentNativeFixture(t *testing.T) (Request, jstestprovider.Receipt) {
	t.Helper()
	rawHash := func(s string) string { return strings.TrimPrefix(Hash([]byte(s)), "sha256:") }
	repo := jstestprovider.ApplicationRepositoryIdentity{RootCommit: strings.Repeat("1", 40), Revision: strings.Repeat("2", 40), Tree: strings.Repeat("3", 40), DirtyState: "clean"}
	source := []byte("<button>one</button>")
	cmd := jstestprovider.FreshCommandIdentity{Argv: []string{"/node", "/entry.cjs"}, ExecutableDigest: rawHash("node"), EntrypointDigest: rawHash("entry")}
	binding := jstestprovider.FreshCommandBinding{Expected: cmd, Before: &cmd, After: &cmd}
	env := map[string]string{"PATH": "/bin", "LANG": "C", "LC_ALL": "C", "TMPDIR": "/tmp"}
	identity := jstestprovider.Identity{RunnerName: "playwright", NodeVersion: "v22.23.3", RunnerVersion: "1.63.0", ConfigFile: "/repo/config.cjs", ConfigDigest: rawHash("config"), TestFileDigests: map[string]string{"/repo/test.cjs": rawHash("test")}, Argv: cmd.Argv, Environment: env}
	identity.ConfigInputDigests = map[string]string{identity.ConfigFile: identity.ConfigDigest}
	identity.PackageDigest = rawHash("/repo/package.json=" + rawHash("package") + "\n/repo/package-lock.json=" + rawHash("lock") + "\n")
	use := json.RawMessage(`{"baseURL":"http://127.0.0.1:4394/","browserName":"chromium","channel":"","headless":true,"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.3","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/cache/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	row := jstestprovider.TestOutcome{Name: "counter one", FullName: "chromium > counter one", State: jstestprovider.StatePassed, Anchor: &jstestprovider.Anchor{File: "/repo/test.cjs", Line: 1}, Project: &jstestprovider.ProjectIdentity{Name: "chromium", Browser: "chromium", Device: "unknown", Use: use, ConfigDigest: identity.ConfigDigest}, Attempts: []jstestprovider.Attempt{{State: jstestprovider.StatePassed, FailureKind: "none"}}}
	manifest := &jstestprovider.FreshDependencyManifest{Roots: []string{"/cache/node_modules/playwright"}, Files: map[string]string{"/cache/node_modules/playwright/lib/impl.js": rawHash("implementation"), "/entry.cjs": cmd.EntrypointDigest, row.Anchor.File: rawHash("test"), identity.ConfigFile: identity.ConfigDigest}}
	deps := &jstestprovider.FreshDependencyObservation{Files: manifest.Files, Failures: []string{}}
	row.ImportedFiles = map[string]string{row.Anchor.File: rawHash("test"), "/cache/node_modules/playwright/lib/impl.js": rawHash("implementation")}
	row.ID = acceptNativeID(identity, row)
	leader := jstestprovider.FreshProcessIdentity{PID: 42, Start: "synthetic-start"}
	served := jstestprovider.FreshServeObservation{URL: "http://127.0.0.1:4394/", Process: leader, Body: source, ServerImports: map[string]string{"/entry.cjs": cmd.EntrypointDigest}, ObserverImports: map[string]string{"/entry.cjs": cmd.EntrypointDigest}}
	absent := &procgroup.DescendantObservation{Scope: "synthetic-unit", IntervalMS: 20, Absent: true, Limitations: []string{"synthetic"}}
	gone := true
	input, _ := json.Marshal(struct {
		Response []byte `json:"response"`
	}{source})
	fresh := &jstestprovider.FreshnessBinding{ProductExpected: repo, TestExpected: repo, ProductBefore: &repo, ProductAfter: &repo, ArtifactPath: "app.html", Source: source, SourceDigest: rawHash(string(source)), ArtifactBefore: rawHash(string(source)), ArtifactAfter: rawHash(string(source)), DocumentURL: served.URL, Leader: &leader, ServedBefore: &served, ServedAfter: &served, Runner: binding, Server: binding, Observer: binding, ObserverConfigBefore: rawHash("observer"), ObserverConfigAfter: rawHash("observer"), EnvironmentBefore: env, EnvironmentAfter: env, IdentityAfter: &identity, ServerGone: &gone, ServerDescendants: absent, Dependencies: manifest, DependencyDigest: jstestprovider.FreshDependencyDigest(manifest), DependenciesBefore: deps, DependenciesAfter: deps, ServerInputDigest: strings.TrimPrefix(Hash(append(input, '\n')), "sha256:")}
	expectation := jstestprovider.ApplicationAttestationExpectation{Repository: jstestprovider.ApplicationRepositoryExpectation{RootCommit: repo.RootCommit, Revision: repo.Revision, Tree: repo.Tree, DirtyPolicy: "require-clean"}, Build: jstestprovider.ApplicationArtifactIdentity{Kind: "source", Digest: Hash(source)}, Configuration: jstestprovider.ApplicationArtifactIdentity{Kind: "source", Digest: "sha256:" + identity.ConfigDigest}, InstanceKind: "process"}
	config, _ := json.Marshal(struct {
		Profile     string                                           `json:"profile"`
		Expectation jstestprovider.ApplicationAttestationExpectation `json:"expectation"`
	}{"corvint-application-attestation-config/0", expectation})
	provider := jstestprovider.ApplicationAttestationProviderIdentity{Profile: jstestprovider.ApplicationAttestationProviderProfile, Argv: []string{"/provider"}, ExecutablePath: "/provider", ExecutableDigest: rawHash("provider"), ConfigPath: "/provider.json", ConfigDigest: strings.TrimPrefix(Hash(append(config, '\n')), "sha256:"), Environment: map[string]string{}}
	observation := jstestprovider.ApplicationAttestationObservation{Attestation: jstestprovider.ApplicationAttestation{Profile: jstestprovider.ApplicationAttestationProfile, Repository: repo, Build: expectation.Build, Configuration: expectation.Configuration, Instance: jstestprovider.ApplicationInstanceIdentity{Kind: "process", ID: "42", StartGeneration: leader.Start}, Health: jstestprovider.ApplicationHealth{State: "healthy"}}}
	refreshParentAttestation(&observation)
	native := jstestprovider.Receipt{Profile: jstestprovider.FreshnessProfile, Kind: "e2e", Identity: identity, Tests: []jstestprovider.TestOutcome{row}, TestRepositoryAtStart: &repo, TestRepositoryAtPublish: &repo, External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ReadyURL: served.URL, ConfigOverride: "synthetic-unit"}, Freshness: fresh, DescendantObservation: absent, ApplicationAttestation: &jstestprovider.ApplicationAttestationReceipt{Provider: provider, Expectation: expectation, Before: &observation, After: &observation}}
	request := Request{Schema: FreshRequestSchema, Repeat: 2, Tests: []Test{{ID: "counter", File: row.Anchor.File, Line: 1, Title: row.Name, Project: row.Project.Name}}, Config: identity.ConfigFile, Package: "/repo/package.json", Lockfile: "/repo/package-lock.json", RunnerVersion: identity.RunnerVersion, Inputs: []File{{identity.ConfigFile, "sha256:" + identity.ConfigDigest}, {row.Anchor.File, "sha256:" + rawHash("test")}, {"/repo/package.json", "sha256:" + rawHash("package")}, {"/repo/package-lock.json", "sha256:" + rawHash("lock")}}, Freshness: &FreshnessOptions{Provider: jstestprovider.FreshnessConfig{ProductExpected: repo, TestExpected: repo, ArtifactPath: fresh.ArtifactPath, SourceDigest: fresh.SourceDigest, DocumentURL: fresh.DocumentURL, Runner: cmd, Server: cmd, Observer: cmd, ObserverConfigDigest: fresh.ObserverConfigBefore, Dependencies: manifest, DependencyDigest: fresh.DependencyDigest}, Attestation: Command{Argv: provider.Argv, ExecutableSHA256: "sha256:" + provider.ExecutableDigest}, AttestationConfiguration: File{provider.ConfigPath, "sha256:" + provider.ConfigDigest}}}
	if !freshRequestBinding(native, request) || jstestprovider.FreshnessCurrency(native, row) != testvalidity.FreshnessCurrent {
		t.Fatal("synthetic baseline must satisfy the existing native contract")
	}
	return request, native
}

func refreshParentAttestation(o *jstestprovider.ApplicationAttestationObservation) {
	raw, _ := json.Marshal(o.Attestation)
	o.OutputDigest = strings.TrimPrefix(Hash(append(raw, '\n')), "sha256:")
}

func parentJoinFixture(t *testing.T, states [][]behaviorfalsify.Status, attempts int) (Request, Control, []jstestprovider.Receipt) {
	t.Helper()
	request, baseline := parentNativeFixture(t)
	plan := behaviorfalsify.Plan{Schema: behaviorfalsify.PlanSchema, Request: behaviorfalsify.Request{Attempts: attempts, Target: behaviorfalsify.TargetIdentity{CriterionID: "count-one", AssertionID: "counter-one", TestID: "counter", TestTitle: "counter one", Project: "chromium"}, Runner: behaviorfalsify.RunnerIdentity{Runner: "playwright", RunnerVersion: "1.63.0"}}, WorkspaceSHA256: Hash([]byte("workspace"))}
	mutant := []byte("<button>two</button>")
	definition := map[string]string{"artifactPath": "app.html", "sourceSha256": baseline.Freshness.SourceDigest, "from": "one", "to": "two", "mutantSha256": strings.TrimPrefix(Hash(mutant), "sha256:")}
	for i, row := range states {
		disposition := "run"
		if len(row) == 1 && row[0] == behaviorfalsify.StatusNotSupported {
			disposition = "not_supported"
		}
		control := behaviorfalsify.ControlSpec{ID: fmt.Sprintf("control-%d", i+1), Kind: behaviorfalsify.ChangedFixtureValue, Disposition: disposition, Definition: definition}
		plan.Request.Controls = append(plan.Request.Controls, control)
		plan.Controls = append(plan.Controls, behaviorfalsify.PlannedControl{ControlSpec: control, Ordinal: i + 1, PerturbationSHA256: Hash([]byte(control.ID))})
	}
	preimage := acceptCanonicalJSON(plan)
	plan.Digest = Hash(preimage)
	tool := behaviorfalsify.ToolIdentity{Name: "corvint-behavior-falsify", Version: "synthetic-unit", Revision: strings.Repeat("4", 40), Executable: Hash([]byte("synthetic-tool")), SourceDirty: true}
	request.Tests[0].Control, request.Tests[0].Tool = &plan, &tool
	e := &behaviorfalsify.EvidenceReceipt{Schema: behaviorfalsify.ReceiptSchema, PlanPreimage: preimage, Approval: behaviorfalsify.Approval{Schema: behaviorfalsify.ApprovalSchema, PlanDigest: plan.Digest, Method: "explicit-approve-plan"}, Tool: tool, Unsupported: []string{"authenticated-operator-identity", "authenticated-hook-semantics", "independent-process-and-artifact-observation"}, Report: behaviorfalsify.Report{Schema: behaviorfalsify.ReportSchema, PlanDigest: plan.Digest, Fallback: "full-relevant-suite"}}
	e.Approval.Digest = Hash(acceptCanonicalJSON(e.Approval))
	success := behaviorfalsify.ProcessEvidence{Started: true, Completed: true, OwnedCleanup: true, DescendantsGone: true}
	for i, row := range states {
		planned := plan.Controls[i]
		result := behaviorfalsify.ControlResult{ID: planned.ID, Kind: planned.Kind, Ordinal: planned.Ordinal, Status: behaviorfalsify.StatusKilled, Reasons: []string{"every-attempt-killed"}}
		if planned.Disposition == "not_supported" || len(row) == 0 {
			result.Status, result.Reasons = behaviorfalsify.StatusNotSupported, nil
			if len(row) == 0 {
				result.Status = behaviorfalsify.StatusNotRun
			}
		} else {
			for j, state := range row {
				attempt := behaviorfalsify.AttemptResult{Attempt: j + 1, Status: state}
				if state == behaviorfalsify.StatusSurvived {
					result.Status, result.Reasons = state, []string{"one-or-more-attempts-survived"}
				}
				if state != behaviorfalsify.StatusKilled && state != behaviorfalsify.StatusSurvived {
					result.Attempts = append(result.Attempts, attempt)
					continue
				}
				raw, _ := json.Marshal(baseline)
				var native jstestprovider.Receipt
				if err := json.Unmarshal(raw, &native); err != nil {
					t.Fatal(err)
				}
				mutation, _ := responseDefinition(planned)
				native.Freshness.Mutation = &mutation
				native.Freshness.Leader.PID = 43 + i*16 + j
				for _, served := range []*jstestprovider.FreshServeObservation{native.Freshness.ServedBefore, native.Freshness.ServedAfter} {
					served.Process = *native.Freshness.Leader
					served.Body = mutant
				}
				input, _ := json.Marshal(struct {
					Response []byte `json:"response"`
				}{mutant})
				native.Freshness.ServerInputDigest = strings.TrimPrefix(Hash(append(input, '\n')), "sha256:")
				for _, observed := range []*jstestprovider.ApplicationAttestationObservation{native.ApplicationAttestation.Before, native.ApplicationAttestation.After} {
					observed.Attestation.Instance.ID = strconv.Itoa(native.Freshness.Leader.PID)
					refreshParentAttestation(observed)
				}
				target := behaviorfalsify.CriterionObservation{CriterionID: "count-one", AssertionID: "counter-one", State: "passed"}
				outcome, reason := "passed", "target-criterion-survived"
				if state == behaviorfalsify.StatusKilled {
					native.Tests[0].State = jstestprovider.StateFailed
					native.Tests[0].Attempts[0].State = jstestprovider.StateFailed
					native.Tests[0].Attempts[0].FailureKind = "assertion-or-test"
					native.Tests[0].FailureMessage = "Error: PTF-ASSERTION:counter-one\nexpect(locator).toHaveText(expected) failed"
					target.State, target.FailureKind, outcome, reason = "failed", "assertion", "failed", "expected-assertion-failed"
				}
				nativeBytes, err := jstestprovider.EncodeFreshness(native)
				if err != nil {
					t.Fatal(err)
				}
				hook := behaviorfalsify.HookReceipt{Schema: behaviorfalsify.HookSchema, PlanDigest: plan.Digest, PerturbationSHA256: planned.PerturbationSHA256, Target: plan.Request.Target, Runner: plan.Request.Runner, Attempt: j + 1, NativeReceiptSHA256: Hash(nativeBytes), TestOutcome: outcome, TargetObservation: target, Artifacts: []behaviorfalsify.Artifact{{Path: "native.json", SHA256: Hash(nativeBytes)}}}
				hookBytes := append(acceptCanonicalJSON(hook), '\n')
				attempt.HookProcess, attempt.CleanupProcess = success, success
				attempt.HookProcess.StdoutSHA256 = Hash(hookBytes)
				attempt.Receipt, attempt.RetainedArtifacts, attempt.Reasons = &hook, hook.Artifacts, []string{reason}
				attempt.WorkspaceBefore, attempt.WorkspaceAfter = plan.WorkspaceSHA256, plan.WorkspaceSHA256
				result.Attempts = append(result.Attempts, attempt)
				e.RawAttempts = append(e.RawAttempts, behaviorfalsify.RawAttempt{ControlID: planned.ID, Ordinal: i + 1, Attempt: j + 1, HookBytes: hookBytes, HookSHA256: Hash(hookBytes), NativeBytes: nativeBytes, NativeSHA256: Hash(nativeBytes)})
			}
		}
		e.Report.Results = append(e.Report.Results, result)
	}
	control := Control{ID: "counter", PlanDigest: plan.Digest, Evidence: e, Unsupported: e.Unsupported, Status: "killed", Cleanup: Cleanup{OwnedGroup: true, Descendants: baseline.DescendantObservation}}
	for _, result := range e.Report.Results {
		if result.Status == behaviorfalsify.StatusSurvived {
			control.Status = "survived"
		}
	}
	parentSeal(&control)
	return request, control, []jstestprovider.Receipt{baseline, baseline}
}

// Seal the synthetic transcript's accounting, leaving its attempt statuses and
// raw correspondence untouched so the real generic verifier still checks them.
func parentSeal(c *Control) {
	e := c.Evidence
	counts := map[behaviorfalsify.Status]int{}
	for _, s := range []behaviorfalsify.Status{behaviorfalsify.StatusKilled, behaviorfalsify.StatusSurvived, behaviorfalsify.StatusNotSupported, behaviorfalsify.StatusNotRun, behaviorfalsify.StatusInfrastructureFailed, behaviorfalsify.StatusInvalidControl} {
		counts[s] = 0
	}
	e.Report.Requested, e.Report.Supported, e.Report.Executed = len(e.Report.Results), 0, 0
	e.Report.CoverageGaps = nil
	var plan behaviorfalsify.Plan
	_ = behaviorfalsify.Decode(e.PlanPreimage, &plan)
	for i, result := range e.Report.Results {
		counts[result.Status]++
		if plan.Controls[i].Disposition == "run" {
			e.Report.Supported++
		}
		if result.Status != behaviorfalsify.StatusKilled {
			e.Report.CoverageGaps = append(e.Report.CoverageGaps, result.ID)
		}
		for _, attempt := range result.Attempts {
			if attempt.HookProcess.Started {
				e.Report.Executed++
				break
			}
		}
	}
	e.Report.Counts = counts
	denom := counts[behaviorfalsify.StatusKilled] + counts[behaviorfalsify.StatusSurvived]
	e.Report.MutationScore = behaviorfalsify.MutationScore{Defined: denom > 0, Killed: counts[behaviorfalsify.StatusKilled], Denominator: denom}
	parentDigest(c)
}

func parentDigest(c *Control) {
	e := c.Evidence
	e.Report.Digest = ""
	e.Report.Digest = Hash(acceptCanonicalJSON(e.Report))
	e.Digest = ""
	e.Digest = Hash(acceptCanonicalJSON(e))
	c.ReceiptDigest = e.Digest
}

// PTF-V0-006/007/008 and NEA-V0-003/004: every approved control and every
// planned attempt must join, even when the generic aggregate validly survives.
func TestPTFV0ParentCompleteAttemptOutcome(t *testing.T) {
	k, s := behaviorfalsify.StatusKilled, behaviorfalsify.StatusSurvived
	for _, tc := range []struct {
		name   string
		states [][]behaviorfalsify.Status
		want   string
	}{
		{"all-killed", [][]behaviorfalsify.Status{{k, k}, {k, k}}, testvalidity.StrengthKilled},
		{"all-survived", [][]behaviorfalsify.Status{{s, s}, {s, s}}, testvalidity.StrengthSurvived},
		{"mixed-last-attempt-survives", [][]behaviorfalsify.Status{{k, k}, {k, s}}, testvalidity.StrengthSurvived},
		{"survivor-missing-planned-attempt", [][]behaviorfalsify.Status{{s}, {k, k}}, testvalidity.StrengthNotMeasured},
		{"survivor-infrastructure-attempt", [][]behaviorfalsify.Status{{s, behaviorfalsify.StatusInfrastructureFailed}}, testvalidity.StrengthNotMeasured},
		{"survivor-invalid-attempt", [][]behaviorfalsify.Status{{s, behaviorfalsify.StatusInvalidControl}}, testvalidity.StrengthNotMeasured},
		{"survivor-unrun-control", [][]behaviorfalsify.Status{{s, s}, {}}, testvalidity.StrengthNotMeasured},
		{"survivor-unsupported-control", [][]behaviorfalsify.Status{{s, s}, {behaviorfalsify.StatusNotSupported}}, testvalidity.StrengthNotMeasured},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, c, baselines := parentJoinFixture(t, tc.states, 2)
			if err := behaviorfalsify.VerifyReceipt(*c.Evidence, c.PlanDigest, c.Evidence.Tool.Executable); err != nil {
				t.Fatalf("generic transcript must verify: %v", err)
			}
			gaps := append([]string(nil), c.Evidence.Report.CoverageGaps...)
			if got := joinedFreshControl(c, r.Tests[0], r, baselines); got != tc.want {
				t.Fatalf("strength=%s want %s", got, tc.want)
			}
			if !slices.Equal(gaps, c.Evidence.Report.CoverageGaps) {
				t.Fatal("join erased generic survivor gaps")
			}
		})
	}
}

// PTF-V0-006/008: the supported attempt bounds are inclusive, and a survivor
// in the final control's final attempt cannot be hidden by earlier kills.
func TestPTFV0ParentCompleteAttemptOutcomeBounds(t *testing.T) {
	for _, attempts := range []int{1, 16} {
		for _, survives := range []bool{false, true} {
			t.Run(fmt.Sprintf("attempts-%d-survives-%v", attempts, survives), func(t *testing.T) {
				states := make([][]behaviorfalsify.Status, 2)
				for i := range states {
					states[i] = make([]behaviorfalsify.Status, attempts)
					for j := range states[i] {
						states[i][j] = behaviorfalsify.StatusKilled
					}
				}
				want := testvalidity.StrengthKilled
				if survives {
					states[1][attempts-1] = behaviorfalsify.StatusSurvived
					want = testvalidity.StrengthSurvived
				}
				r, c, baselines := parentJoinFixture(t, states, attempts)
				if err := behaviorfalsify.VerifyReceipt(*c.Evidence, c.PlanDigest, c.Evidence.Tool.Executable); err != nil {
					t.Fatal(err)
				}
				if got := joinedFreshControl(c, r.Tests[0], r, baselines); got != want {
					t.Fatalf("strength=%s want %s", got, want)
				}
			})
		}
	}
}

// Replace opaque native bytes while preserving the real generic verifier's
// hash and hook correspondence. This does not repair native semantics.
func parentReplaceNative(t *testing.T, c *Control, index int, data []byte) {
	t.Helper()
	raw := &c.Evidence.RawAttempts[index]
	var hook behaviorfalsify.HookReceipt
	if err := behaviorfalsify.Decode(raw.HookBytes, &hook); err != nil {
		t.Fatal(err)
	}
	hook.NativeReceiptSHA256 = Hash(data)
	hook.Artifacts[0].SHA256 = Hash(data)
	raw.NativeBytes, raw.NativeSHA256 = data, Hash(data)
	raw.HookBytes = append(acceptCanonicalJSON(hook), '\n')
	raw.HookSHA256 = Hash(raw.HookBytes)
	attempt := &c.Evidence.Report.Results[raw.Ordinal-1].Attempts[raw.Attempt-1]
	attempt.Receipt, attempt.RetainedArtifacts = &hook, hook.Artifacts
	attempt.HookProcess.StdoutSHA256 = raw.HookSHA256
	parentDigest(c)
}

// PTF-V0-006/008: a valid generic receipt alone cannot prove native execution
// semantics, even after every native/hook/report hash has been resealed.
func TestPTFV0ParentCompleteAttemptOutcomeNativeForgery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*jstestprovider.Receipt)
	}{
		{"passed-native-for-killed-hook", func(n *jstestprovider.Receipt) {
			n.Tests[0].State = jstestprovider.StatePassed
			n.Tests[0].Attempts[0].State = jstestprovider.StatePassed
			n.Tests[0].FailureMessage = ""
		}},
		{"wrong-assertion", func(n *jstestprovider.Receipt) { n.Tests[0].FailureMessage = "PTF-ASSERTION:unrelated" }},
		{"wrong-title", func(n *jstestprovider.Receipt) { n.Tests[0].FullName += " other" }},
		{"wrong-project", func(n *jstestprovider.Receipt) { n.Tests[0].Project.Name = "other" }},
		{"duplicate-row", func(n *jstestprovider.Receipt) { n.Tests = append(n.Tests, n.Tests[0]) }},
		{"duplicate-attempt", func(n *jstestprovider.Receipt) {
			n.Tests[0].Attempts = append(n.Tests[0].Attempts, n.Tests[0].Attempts[0])
		}},
		{"source-drift", func(n *jstestprovider.Receipt) { n.Freshness.ArtifactAfter = strings.Repeat("e", 64) }},
		{"environment-drift", func(n *jstestprovider.Receipt) { n.Freshness.EnvironmentAfter["LANG"] = "other" }},
		{"dependency-drift", func(n *jstestprovider.Receipt) {
			n.Freshness.DependenciesAfter.Files["/entry.cjs"] = strings.Repeat("e", 64)
		}},
		{"response-drift", func(n *jstestprovider.Receipt) { n.Freshness.ServedAfter.Body = []byte("unexpected") }},
		{"unknown-cleanup", func(n *jstestprovider.Receipt) { n.Freshness.ServerGone = nil }},
		{"opaque", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, s := behaviorfalsify.StatusKilled, behaviorfalsify.StatusSurvived
			r, c, base := parentJoinFixture(t, [][]behaviorfalsify.Status{{s, s}, {k, k}}, 2)
			data := []byte("opaque native evidence")
			if tc.change != nil {
				n, err := jstestprovider.DecodeFreshness(c.Evidence.RawAttempts[3].NativeBytes)
				if err != nil {
					t.Fatal(err)
				}
				tc.change(&n)
				data, err = jstestprovider.EncodeFreshness(n)
				if err != nil {
					t.Fatal(err)
				}
			}
			parentReplaceNative(t, &c, 3, data)
			if err := behaviorfalsify.VerifyReceipt(*c.Evidence, c.PlanDigest, c.Evidence.Tool.Executable); err != nil {
				t.Fatalf("forgery must remain generic-valid: %v", err)
			}
			if got := joinedFreshControl(c, r.Tests[0], r, base); got != testvalidity.StrengthNotMeasured {
				t.Fatalf("forgery measured %s", got)
			}
		})
	}
}

// PTF-V0-006/008: missing, duplicated, reordered or inconsistent correspondence
// must never measure strength. Generic-invalid cases are retained as such.
func TestPTFV0ParentCompleteAttemptOutcomeCorrespondence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Request, *Control)
	}{
		{"missing-raw", func(_ *Request, c *Control) { c.Evidence.RawAttempts = c.Evidence.RawAttempts[:3] }},
		{"extra-raw", func(_ *Request, c *Control) {
			c.Evidence.RawAttempts = append(c.Evidence.RawAttempts, c.Evidence.RawAttempts[0])
		}},
		{"duplicate-raw", func(_ *Request, c *Control) { c.Evidence.RawAttempts[3] = c.Evidence.RawAttempts[2] }},
		{"reordered-raw", func(_ *Request, c *Control) {
			c.Evidence.RawAttempts[2], c.Evidence.RawAttempts[3] = c.Evidence.RawAttempts[3], c.Evidence.RawAttempts[2]
		}},
		{"wrong-control-id", func(_ *Request, c *Control) { c.Evidence.RawAttempts[3].ControlID = "other" }},
		{"wrong-ordinal", func(_ *Request, c *Control) { c.Evidence.RawAttempts[3].Ordinal = 1 }},
		{"wrong-attempt", func(_ *Request, c *Control) { c.Evidence.RawAttempts[3].Attempt = 1 }},
		{"extra-gap", func(_ *Request, c *Control) {
			c.Evidence.Report.CoverageGaps = append(c.Evidence.Report.CoverageGaps, "unexplained")
		}},
		{"missing-gap", func(_ *Request, c *Control) { c.Evidence.Report.CoverageGaps = nil }},
		{"wrong-count", func(_ *Request, c *Control) { c.Evidence.Report.Executed-- }},
		{"wrong-result", func(_ *Request, c *Control) { c.Evidence.Report.Results[1].Status = behaviorfalsify.StatusKilled }},
		{"wrong-control-status", func(_ *Request, c *Control) { c.Status = "killed" }},
		{"missing-unsupported", func(_ *Request, c *Control) { c.Unsupported = nil }},
		{"caller-plan-mismatch", func(r *Request, _ *Control) { r.Tests[0].Control.Request.Attempts = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, s := behaviorfalsify.StatusKilled, behaviorfalsify.StatusSurvived
			r, c, base := parentJoinFixture(t, [][]behaviorfalsify.Status{{k, k}, {k, s}}, 2)
			tc.change(&r, &c)
			parentDigest(&c)
			if got := joinedFreshControl(c, r.Tests[0], r, base); got != testvalidity.StrengthNotMeasured {
				t.Fatalf("invalid correspondence measured %s", got)
			}
		})
	}
}

// PTF-V0-006/008: the reverse outcome mismatch also stays unmeasured, and
// every repeat baseline is necessary even when all controls are complete.
func TestPTFV0ParentCompleteAttemptOutcomeSurvivorMismatch(t *testing.T) {
	s := behaviorfalsify.StatusSurvived
	r, c, baselines := parentJoinFixture(t, [][]behaviorfalsify.Status{{s, s}}, 2)
	t.Run("missing-baseline", func(t *testing.T) {
		if got := joinedFreshControl(c, r.Tests[0], r, baselines[:1]); got != testvalidity.StrengthNotMeasured {
			t.Fatalf("missing baseline measured %s", got)
		}
	})
	t.Run("failed-native-for-survived-hook", func(t *testing.T) {
		n, err := jstestprovider.DecodeFreshness(c.Evidence.RawAttempts[1].NativeBytes)
		if err != nil {
			t.Fatal(err)
		}
		n.Tests[0].State = jstestprovider.StateFailed
		n.Tests[0].Attempts[0].State = jstestprovider.StateFailed
		n.Tests[0].Attempts[0].FailureKind = "assertion-or-test"
		n.Tests[0].FailureMessage = "Error: PTF-ASSERTION:counter-one\nexpect(locator).toHaveText(expected) failed"
		data, err := jstestprovider.EncodeFreshness(n)
		if err != nil {
			t.Fatal(err)
		}
		parentReplaceNative(t, &c, 1, data)
		if err := behaviorfalsify.VerifyReceipt(*c.Evidence, c.PlanDigest, c.Evidence.Tool.Executable); err != nil {
			t.Fatal(err)
		}
		if got := joinedFreshControl(c, r.Tests[0], r, baselines); got != testvalidity.StrengthNotMeasured {
			t.Fatalf("outcome mismatch measured %s", got)
		}
	})
}

// NEA-V0-007/009: hand-authored body oracle, independent of the renderer.
// Each corruption preserves unrelated tokens that the old substring proof used.
func TestPTFV0ParentBodyBinding(t *testing.T) {
	minimum, maximum := 12.2, 35.8
	r := Report{Verdict: "rejected", Product: Repository{Commit: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40)}, TestRepository: Repository{Commit: strings.Repeat("3", 40), Tree: strings.Repeat("4", 40)}, Environment: "fixture-env", Build: "fixture-build", ExecutableSHA256: "sha256:" + strings.Repeat("a", 64), RequestDigest: "sha256:" + strings.Repeat("b", 64), Unknowns: []string{"unknown-one", "unknown-two"}, Assessments: []Assessment{{ID: "counter", Verdict: "rejected", Reasons: []string{"negative-control-survived", "test-failed"}, Validity: testvalidity.Projection{Strength: testvalidity.Axis{State: testvalidity.StrengthSurvived}}, Repeats: RepeatSummary{Passed: 2, Failed: 1, Other: 0, MinDurationMS: &minimum, MaxDurationMS: &maximum}, Cleanup: "observed-absent", Order: OrderEvidence{Status: "nondeterministic-in-isolation", RequestedFileOrder: "not-varied", IsolatedStates: []string{"passed", "failed"}}}}}
	r.Body = "**Overall verdict: rejected**\n" +
		"| Product revision | `1111111111111111111111111111111111111111` (tree `2222222222222222222222222222222222222222`) |\n" +
		"| Test-repository revision | `3333333333333333333333333333333333333333` (tree `4444444444444444444444444444444444444444`) |\n" +
		"| Environment | `fixture-env` |\n| Corvint build | `fixture-build` |\n" +
		"| Companion executable | `sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` |\n" +
		"| Request digest | `sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb` |\n" +
		"| Repeats | 3 per test, retries 0, workers 1 |\n" +
		"| `counter` | rejected | 2/1/0 | 12-36 | SURVIVED | observed-absent | nondeterministic-in-isolation | not-varied | passed, failed |\n" +
		"- `counter`: negative-control-survived, test-failed\nUnknowns: unknown-one, unknown-two\n"
	if err := parentBodyBinding(r, 3); err != nil {
		t.Fatal(err)
	}
	// Preserve the prior proof's blind spots as a regression discriminator:
	// all its tokens still occur in these corrupt bodies.
	oldProofAccepts := func(body string) bool {
		a := r.Assessments[0]
		for _, value := range []string{r.Verdict, r.Product.Commit, r.TestRepository.Commit, r.Environment, r.Build, r.ExecutableSHA256, r.RequestDigest, a.Validity.Strength.State, a.Order.Status, a.Order.RequestedFileOrder} {
			if !strings.Contains(body, value) {
				return false
			}
		}
		for _, reason := range a.Reasons {
			if !strings.Contains(body, reason) {
				return false
			}
		}
		return !strings.Contains(body, "UNVALIDATED")
	}
	for _, tc := range []struct{ name, from, to string }{
		{"overall", "**Overall verdict: rejected**", "**Overall verdict: accepted**"},
		{"product-commit", strings.Repeat("1", 40), strings.Repeat("9", 40)},
		{"product-tree", strings.Repeat("2", 40), strings.Repeat("9", 40)},
		{"test-commit", strings.Repeat("3", 40), strings.Repeat("9", 40)},
		{"test-tree", strings.Repeat("4", 40), strings.Repeat("9", 40)},
		{"environment", "`fixture-env`", "`other-env`"},
		{"build", "`fixture-build`", "`other-build`"},
		{"executable", strings.Repeat("a", 64), strings.Repeat("c", 64)},
		{"request", strings.Repeat("b", 64), strings.Repeat("c", 64)},
		{"repeats", "3 per test", "32 per test"},
		{"test-id", "| `counter` |", "| `other` |"},
		{"test-verdict", "| rejected | 2/1/0", "| accepted | 2/1/0"},
		{"passed-count", "2/1/0", "3/1/0"},
		{"failed-count", "2/1/0", "2/0/0"},
		{"other-count", "2/1/0", "2/1/1"},
		{"minimum", "12-36", "13-36"},
		{"maximum", "12-36", "12-37"},
		{"strength", "| SURVIVED |", "| KILLED |"},
		{"cleanup", "| observed-absent |", "| survivors |"},
		{"order", "| nondeterministic-in-isolation |", "| fails-in-isolation |"},
		{"file-order", "| not-varied |", "| same-outcome |"},
		{"isolated-state", "| passed, failed |", "| passed, passed |"},
		{"reasons", "negative-control-survived, test-failed", "negative-control-survived"},
		{"unknowns", "unknown-one, unknown-two", "unknown-one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := r
			changed.Body = strings.Replace(r.Body, tc.from, tc.to, 1)
			if changed.Body == r.Body {
				t.Fatal("corruption did not apply")
			}
			if slices.Contains([]string{"product-tree", "test-tree", "repeats", "passed-count", "failed-count", "other-count", "minimum", "maximum", "cleanup", "isolated-state", "unknowns"}, tc.name) && !oldProofAccepts(changed.Body) {
				t.Fatal("regression discriminator must remain accepted by the previous incomplete proof")
			}
			if err := parentBodyBinding(changed, 3); err == nil {
				t.Fatal("corrupt body qualified")
			}
		})
	}
	t.Run("duplicate-row", func(t *testing.T) {
		changed := r
		changed.Body += "| `counter` | rejected | 2/1/0 | 12-36 | SURVIVED | observed-absent | nondeterministic-in-isolation | not-varied | passed, failed |\n"
		if err := parentBodyBinding(changed, 3); err == nil {
			t.Fatal("duplicate row qualified")
		}
	})
}
