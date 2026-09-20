package playwrightminimize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const LiveSchema = "corvint-playwright-suite-interaction-live/0"

// Command is operator-owned reset/cleanup code, pinned before authorization.
// Its successful exit is evidence of execution, not proof of reset semantics.
type Command struct {
	Argv             []string `json:"argv"`
	ExecutableSHA256 string   `json:"executable_sha256"`
}

type LiveRequest struct {
	Request      Request                  `json:"request"`
	Repository   string                   `json:"repository"`
	Corpus       []byte                   `json:"corpus"`
	StabilityIDs []string                 `json:"stability_ids"`
	Original     []byte                   `json:"original"`
	Isolated     []byte                   `json:"isolated"`
	Config       jstestprovider.E2EConfig `json:"config"`
	Reset        Command                  `json:"reset"`
	Cleanup      Command                  `json:"cleanup"`
}

type LivePlan struct {
	Schema string      `json:"schema"`
	Input  LiveRequest `json:"input"`
	Plan   Plan        `json:"plan"`
	Digest string      `json:"digest"`
}

type ProcessEvidence struct {
	Descendants *procgroup.DescendantObservation `json:"descendants"`
	Error       string                           `json:"error,omitempty"`
	Stdout      []byte                           `json:"stdout"`
	Stderr      []byte                           `json:"stderr"`
	Exit        int                              `json:"exit"`
	Started     bool                             `json:"started"`
	Completed   bool                             `json:"completed"`
	Cleanup     bool                             `json:"cleanup"`
	Cancelled   bool                             `json:"cancelled"`
	TimedOut    bool                             `json:"timed_out"`
	Overflow    bool                             `json:"overflow"`
	Usage       *procgroup.ResourceUsage         `json:"usage"`
}

type LiveTrialEvidence struct {
	ResetGeneration string                 `json:"reset_generation"`
	Reset           ProcessEvidence        `json:"reset"`
	Cleanup         ProcessEvidence        `json:"cleanup"`
	Receipt         jstestprovider.Receipt `json:"receipt"`
}

func LivePlanRequest(ctx context.Context, input LiveRequest) (LivePlan, error) {
	if !filepath.IsAbs(input.Repository) || !filepath.IsAbs(input.Config.Dir) || !input.Config.ExternalServer || input.Config.ApplicationAttestation == nil || input.Request.ResetPolicy.RestartApplication || !input.Request.ResetPolicy.ClearBrowserState {
		return LivePlan{}, errors.New("live profile requires absolute repositories, attested external application and fresh-browser non-restart reset")
	}
	if len(input.Config.TestArgv) != 0 || len(input.Config.ServerArgv) != 0 {
		return LivePlan{}, errors.New("live profile constructs test selectors and never owns the application server")
	}
	for _, command := range []Command{input.Reset, input.Cleanup} {
		if err := validateCommand(command); err != nil {
			return LivePlan{}, err
		}
	}
	if digestJSON(struct {
		Reset   Command
		Cleanup Command
	}{input.Reset, input.Cleanup}) != input.Request.ResetPolicy.Digest {
		return LivePlan{}, errors.New("reset policy does not bind commands")
	}
	original, err := decodeNative(input.Original)
	if err != nil {
		return LivePlan{}, err
	}
	isolated, err := decodeNative(input.Isolated)
	if err != nil {
		return LivePlan{}, err
	}
	fixed := input.Request.OriginalFailure.Identity.Fixed
	if input.Config.RunnerName != fixed.Runner || input.Config.RunnerVersion != fixed.RunnerVersion || input.Config.ConfigFile != original.Identity.ConfigFile || fixed.FixtureSchema != "playwright-use" || !slices.Contains(input.Config.DeclaredEnvKeys, "CORVINT_MINIMIZER_SEED") || os.Getenv("CORVINT_MINIMIZER_SEED") != fixed.SeedIdentity {
		return LivePlan{}, errors.New("execution config or declared seed does not match baseline")
	}
	configBytes, err := os.ReadFile(input.Config.ConfigFile)
	if err != nil || digestBytes(configBytes) != fixed.ConfigDigest {
		return LivePlan{}, errors.New("execution config bytes drifted")
	}
	if digestBytes(input.Original) != input.Request.OriginalFailure.ReceiptDigest || digestBytes(input.Isolated) != input.Request.IsolatedPass.ReceiptDigest {
		return LivePlan{}, errors.New("baseline receipt bytes mismatch")
	}
	if err := validateLiveBaseline(original, input.Request.OriginalFailure, input.Request.Target, nil); err != nil {
		return LivePlan{}, err
	}
	if err := validateLiveBaseline(isolated, input.Request.IsolatedPass, input.Request.Target, original); err != nil {
		return LivePlan{}, err
	}
	artifact, err := doccorpus.Open(ctx, input.Repository, input.Corpus)
	if err != nil {
		return LivePlan{}, fmt.Errorf("stability rederivation: %w", err)
	}
	if !stabilityBinds(artifact, input) {
		return LivePlan{}, errors.New("stability aggregate does not bind the original and isolated receipts and identities")
	}
	input.Request.Qualification = Qualification{RunnerQualified: true, RunnerReceiptDigest: digestBytes(input.Original), StabilityQualified: true, StabilityReceiptDigest: digestBytes(input.Corpus), ApplicationAttested: true, ApplicationReceiptDigest: digestJSON(original.ApplicationAttestation)}
	plan, err := BuildPlan(input.Request)
	if err != nil {
		return LivePlan{}, err
	}
	for _, trial := range plan.Trials {
		if trial.Identity.WorkerTopology.Shard != "" || trial.Identity.WorkerTopology.FullyParallel {
			return LivePlan{}, errors.New("live profile supports unsharded project-default parallelism only")
		}
		for _, id := range trial.Identity.Order {
			if findNative(original, id) == nil {
				return LivePlan{}, errors.New("trial member missing from qualified baseline")
			}
		}
	}
	result := LivePlan{Schema: LiveSchema, Input: input, Plan: plan}
	result.Digest = digestJSON(result)
	return result, nil
}

func ExecuteLive(ctx context.Context, plan LivePlan, approvedDigest string) (Report, error) {
	if approvedDigest == "" || approvedDigest != plan.Digest {
		return Report{}, errors.New("operator-authorization-required")
	}
	rebuilt, err := LivePlanRequest(ctx, plan.Input)
	if err != nil {
		return Report{}, err
	}
	if !reflect.DeepEqual(rebuilt, plan) {
		return Report{}, errors.New("live-plan-drift")
	}
	original, _ := decodeNative(plan.Input.Original)
	runner := &liveRunner{input: plan.Input, original: original}
	report, err := Execute(ctx, plan.Plan, Authorization{OperatorApproved: true, PlanDigest: plan.Plan.Digest}, runner)
	report.Limitations = append(report.Limitations, "cleanup covers the owned process group and observed PID/start identities; fast detach/reparent between 20ms snapshots may remain unobserved", "reset semantics are operator-owned; successful command execution is not independent proof that arbitrary application state was restored")
	return report, err
}

type liveRunner struct {
	input    LiveRequest
	original *jstestprovider.Receipt
}

func (r *liveRunner) Run(ctx context.Context, trial Trial) (result TrialReceipt, err error) {
	result = TrialReceipt{TrialID: trial.ID, Identity: trial.Identity, ResetPolicy: trial.ResetPolicy}
	evidence := &LiveTrialEvidence{ResetGeneration: digestJSON(trial)}
	result.Live = evidence
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		evidence.Cleanup = runCommand(cleanupCtx, r.input.Cleanup, r.input.Config.Dir, trial)
		result.CleanupSucceeded = processSucceeded(evidence.Cleanup)
		result.Evidence.Cleanup = refs(evidence.Cleanup, "operator cleanup process and descendant observation")
		result.Evidence.Resources = refs(evidence, "retained process accounting; absent accounting stays unknown")
		result.Digest = receiptDigest(result)
	}()
	evidence.Reset = runCommand(ctx, r.input.Reset, r.input.Config.Dir, trial)
	result.ResetSucceeded = processSucceeded(evidence.Reset)
	result.Evidence.Setup = refs(evidence.Reset, "operator reset process bound to trial and seed")
	if !result.ResetSucceeded {
		return result, errors.New("reset-failed")
	}
	cfg := r.input.Config
	cfg.ObserveDescendants = true
	cfg.TestArgv = []string{fmt.Sprintf("--workers=%d", trial.Identity.WorkerTopology.Workers), "--retries=0", "--repeat-each=1", "--no-deps", "--forbid-only", "--project=" + trial.Identity.Fixed.Project}
	for _, id := range trial.Identity.Order {
		test := findNative(r.original, id)
		cfg.TestArgv = append(cfg.TestArgv, fmt.Sprintf("%s:%d", test.Anchor.File, test.Anchor.Line))
	}
	native, runErr := jstestprovider.RunE2E(ctx, cfg)
	evidence.Receipt = native
	result.Evidence.PageAssertions = refs(native, "qualified Playwright outcomes, page failures and artifacts")
	result.Evidence.Retries = refs(native.Tests, "actual attempt and retry observations")
	result.Evidence.ServerHealth = refs(native.ApplicationAttestation, "application health and instance observations before and after")
	if native.ApplicationAttestation != nil {
		if native.ApplicationAttestation.Before != nil {
			result.ApplicationAttestationStart = "sha256:" + native.ApplicationAttestation.Before.OutputDigest
		}
		if native.ApplicationAttestation.After != nil {
			result.ApplicationAttestationPublish = "sha256:" + native.ApplicationAttestation.After.OutputDigest
		}
		for _, failure := range native.ApplicationAttestation.Failures {
			if failure == "application-restarted" {
				result.Failures = append(result.Failures, FailureObservation{Class: FailureRestart, EvidenceDigest: digestJSON(native.ApplicationAttestation), Summary: failure})
			}
		}
	}
	target := correspondingNative(&native, findNative(r.original, r.input.Request.Target))
	if target != nil {
		result.Outcome = nativeOutcome(*target)
		for _, attempt := range target.Attempts {
			if attempt.State == jstestprovider.StatePassed {
				continue
			}
			class := FailureAssertion
			if attempt.FailureKind == "browser-or-fixture" {
				class = FailureFixture
			}
			if attempt.State == jstestprovider.StateTimedOut {
				class = FailureSynchronization
			}
			result.Failures = append(result.Failures, FailureObservation{Class: class, EvidenceDigest: digestJSON(attempt), Summary: string(attempt.State) + ": " + attempt.FailureKind})
		}
	}
	if runErr != nil {
		return result, runErr
	}
	if native.DescendantObservation == nil || !native.DescendantObservation.Absent {
		return result, errors.New("runner observed-descendant cleanup unresolved")
	}
	baseline := Baseline{Identity: trial.Identity, Outcome: result.Outcome}
	if trial.Kind == TrialLoad && native.Schedule != nil {
		observed := make([]string, 0, len(native.Schedule.Starts))
		for _, start := range native.Schedule.Starts {
			for _, id := range trial.Identity.Order {
				expected := findNative(r.original, id)
				if expected != nil && start.FullName == expected.FullName && start.File == expected.Anchor.File && start.Line == expected.Anchor.Line {
					observed = append(observed, id)
				}
			}
		}
		want, got := slices.Clone(trial.Identity.Order), slices.Clone(observed)
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(want, got) {
			return result, errors.New("observed load membership mismatch")
		}
		baseline.Identity.Order = observed
	}
	if err := validateLiveBaseline(&native, baseline, r.input.Request.Target, r.original); err != nil {
		return result, err
	}
	return result, nil
}

func validateLiveBaseline(r *jstestprovider.Receipt, baseline Baseline, targetID string, reference *jstestprovider.Receipt) error {
	if reference == nil {
		reference = r
	}
	if r.Profile != jstestprovider.AttestedExternalProfile || r.Schedule == nil {
		return errors.New("attested receipt and observed schedule required")
	}
	fixed := baseline.Identity.Fixed
	if fixed.RunnerVersion != "1.63.0" {
		return errors.New("live minimizer requires the qualified 1.63 system-Chrome tuple")
	}
	if r.Schedule.Workers != baseline.Identity.WorkerTopology.Workers || len(r.Schedule.Starts) != len(baseline.Identity.Order) || len(r.Tests) != len(baseline.Identity.Order) {
		return errors.New("observed schedule mismatch")
	}
	if r.TestRepositoryAtStart == nil || r.TestRepositoryAtStart.Revision != fixed.TestRevision || r.Identity.ConfigDigest != strings.TrimPrefix(fixed.ConfigDigest, "sha256:") || r.Identity.RunnerName != fixed.Runner || r.Identity.RunnerVersion != fixed.RunnerVersion {
		return errors.New("runner identity mismatch")
	}
	for i, id := range baseline.Identity.Order {
		expected := findNative(reference, id)
		actual := correspondingNative(r, expected)
		if actual == nil || !jstestprovider.QualifiedReceiptBindingReady(*r, *actual) {
			return errors.New("unqualified selected test")
		}
		revision, ok := jstestprovider.QualifiedApplicationRevision(*r, *actual)
		if !ok || revision != fixed.ApplicationRevision || actual.Project.Name != fixed.Project || actual.Project.Browser != fixed.Browser || digestBytes(actual.Project.Use) != fixed.FixtureDigest {
			return errors.New("application/project/fixture identity mismatch")
		}
		start := r.Schedule.Starts[i]
		if start.FullName != expected.FullName || start.File != expected.Anchor.File || start.Line != expected.Anchor.Line || start.Project != fixed.Project || start.Retry != 0 || start.Retries != 0 || start.Worker < 0 || start.FullyParallel != baseline.Identity.WorkerTopology.FullyParallel {
			return errors.New("observed test order/topology mismatch")
		}
		if "sha256:"+r.ApplicationAttestation.Before.OutputDigest != fixed.ApplicationAttestationDigest {
			return errors.New("application instance mismatch")
		}
		var use struct {
			Browser struct {
				Version string `json:"browserVersion"`
			} `json:"corvintBrowser"`
		}
		if json.Unmarshal(actual.Project.Use, &use) != nil || use.Browser.Version != fixed.BrowserVersion {
			return errors.New("observed browser version mismatch")
		}
		if r.Identity.Environment["CORVINT_MINIMIZER_SEED"] != fixed.SeedIdentity {
			return errors.New("seed identity missing from declared environment")
		}
	}
	target := correspondingNative(r, findNative(reference, targetID))
	if target == nil || nativeOutcome(*target) != baseline.Outcome {
		return errors.New("baseline target outcome mismatch")
	}
	return nil
}

func stabilityBinds(a *doccorpus.Artifact, input LiveRequest) bool {
	original, err := decodeNative(input.Original)
	if err != nil {
		return false
	}
	isolated, err := decodeNative(input.Isolated)
	if err != nil {
		return false
	}
	first := findNative(original, input.Request.Target)
	second := correspondingNative(isolated, first)
	if first == nil || second == nil {
		return false
	}
	ids := map[string]string{strings.TrimPrefix(input.Request.OriginalFailure.ReceiptDigest, "sha256:"): first.ID, strings.TrimPrefix(input.Request.IsolatedPass.ReceiptDigest, "sha256:"): second.ID}
	wanted := map[string]bool{strings.TrimPrefix(input.Request.OriginalFailure.ReceiptDigest, "sha256:"): false, strings.TrimPrefix(input.Request.IsolatedPass.ReceiptDigest, "sha256:"): false}
	for _, report := range a.StabilityEvidence {
		if !slices.Contains(input.StabilityIDs, report.ID) {
			continue
		}
		for _, c := range report.ContributingReceipts {
			if _, ok := wanted[c.Receipt.SHA256]; !ok {
				continue
			}
			if report.TestID != ids[c.Receipt.SHA256] {
				return false
			}
			f := input.Request.OriginalFailure.Identity.Fixed
			if c.Identity.ApplicationRevision != f.ApplicationRevision || c.Identity.TestRevision != f.TestRevision || c.Identity.ConfigSHA256 != strings.TrimPrefix(f.ConfigDigest, "sha256:") || c.Identity.Project != f.Project {
				return false
			}
			wanted[c.Receipt.SHA256] = true
		}
	}
	for _, found := range wanted {
		if !found {
			return false
		}
	}
	return true
}

func decodeNative(data []byte) (*jstestprovider.Receipt, error) {
	input, err := testvaliditydoc.Decode(data)
	if err != nil {
		return nil, err
	}
	r := testvaliditydoc.Project(input).Playwright
	if r == nil {
		return nil, errors.New("Playwright receipt required")
	}
	return r, nil
}

func findNative(r *jstestprovider.Receipt, id string) *jstestprovider.TestOutcome {
	for i := range r.Tests {
		if r.Tests[i].ID == id {
			return &r.Tests[i]
		}
	}
	return nil
}

func correspondingNative(r *jstestprovider.Receipt, expected *jstestprovider.TestOutcome) *jstestprovider.TestOutcome {
	if expected == nil || expected.Anchor == nil || expected.Project == nil {
		return nil
	}
	var found *jstestprovider.TestOutcome
	for i := range r.Tests {
		t := &r.Tests[i]
		if t.FullName == expected.FullName && reflect.DeepEqual(t.Anchor, expected.Anchor) && t.Project != nil && t.Project.Name == expected.Project.Name {
			if found != nil {
				return nil
			}
			found = t
		}
	}
	return found
}

func nativeOutcome(t jstestprovider.TestOutcome) Outcome {
	if t.State == jstestprovider.StatePassed {
		return OutcomePassed
	}
	return OutcomeFailed
}

func validateCommand(c Command) error {
	if len(c.Argv) != 1 || !filepath.IsAbs(c.Argv[0]) {
		return errors.New("reset/cleanup must name one absolute pinned executable without arguments")
	}
	info, err := os.Lstat(c.Argv[0])
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 || info.Mode()&0111 == 0 {
		return errors.New("reset/cleanup executable unavailable")
	}
	data, err := os.ReadFile(c.Argv[0])
	if err != nil || digestBytes(data) != c.ExecutableSHA256 {
		return errors.New("reset/cleanup executable drift")
	}
	return nil
}

func runCommand(ctx context.Context, c Command, dir string, trial Trial) ProcessEvidence {
	if validateCommand(c) != nil {
		return ProcessEvidence{Error: "reset/cleanup executable unavailable or changed"}
	}
	executable, err := os.ReadFile(c.Argv[0])
	if err != nil || digestBytes(executable) != c.ExecutableSHA256 {
		return ProcessEvidence{Error: "reset/cleanup executable changed before staging"}
	}
	scratch, err := os.MkdirTemp("", "corvint-minimize-command-")
	if err != nil {
		return ProcessEvidence{Error: "reset/cleanup staging unavailable"}
	}
	defer os.RemoveAll(scratch)
	staged := filepath.Join(scratch, "command")
	if err := os.WriteFile(staged, executable, 0700); err != nil {
		return ProcessEvidence{Error: "reset/cleanup staging failed"}
	}
	data, _ := json.Marshal(struct {
		Trial           Trial  `json:"trial"`
		ResetGeneration string `json:"reset_generation"`
	}{trial, digestJSON(trial)})
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: []string{staged}, Dir: dir, Env: os.Environ(), Stdin: data, Timeout: 5 * time.Second, OutputLimit: 64 << 10, ObserveDescendants: true})
	failure := ""
	if obs.Err != nil {
		failure = obs.Err.Error()
	}
	return ProcessEvidence{Descendants: obs.DescendantObservation, Error: failure, Stdout: obs.Stdout, Stderr: obs.Stderr, Exit: obs.ExitStatus, Started: obs.Started, Completed: obs.WaitCompleted && obs.ExitObserved, Cleanup: obs.OwnedProcessGroupCleanup, Cancelled: obs.Cancelled, TimedOut: obs.TimedOut, Overflow: obs.OutputOverflow || obs.StdoutOverflow || obs.StderrOverflow, Usage: obs.Usage}
}

func processSucceeded(p ProcessEvidence) bool {
	return p.Started && p.Completed && p.Cleanup && p.Descendants != nil && p.Descendants.Absent && p.Error == "" && p.Exit == 0 && !p.Cancelled && !p.TimedOut && !p.Overflow
}
func digestBytes(data []byte) string { return "sha256:" + doccorpus.Digest(data) }
func digestJSON(v any) string        { data, _ := json.Marshal(v); return digestBytes(data) }
func refs(v any, detail string) []EvidenceRef {
	return []EvidenceRef{{Digest: digestJSON(v), Detail: detail}}
}

// DecodeLiveRequest rejects unknown fields and trailing JSON before any command.
func DecodeLiveRequest(data []byte, v any) error {
	if len(data) > 32<<20 {
		return errors.New("live request exceeds bound")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
