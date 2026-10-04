package localcompletion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// PublicCommand invokes the existing CEM/OCM entrypoints, without a shell or
// reinterpretation of their evidence semantics.
type PublicCommand func(context.Context, string, []string, io.Writer, io.Writer) int

func Finish(ctx context.Context, root, key string, command PublicCommand) (Evaluation, error) {
	return FinishWithAggregateProfile(ctx, root, key, "", command)
}

func FinishWithAggregateProfile(ctx context.Context, root, key, profile string, command PublicCommand) (Evaluation, error) {
	if profile != "" && profile != tracerecordrepo.AggregateOutcomeProfile {
		return Evaluation{}, errors.New("aggregate-profile-unsupported")
	}
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	ctx, unlock, err := dogfoodoperation.Acquire(ctx, repo.auth.GitDir)
	if err != nil {
		return Evaluation{}, err
	}
	defer unlock()
	if err = repo.requireOwner(); err != nil {
		return Evaluation{}, err
	}
	saved, err := repo.load()
	if err != nil {
		return Evaluation{}, err
	}
	if saved.Lifecycle == "cancelled" {
		return Evaluation{}, errors.New("enrollment-cancelled")
	}
	if saved.AggregateOutcome != nil && profile == "" {
		return Evaluation{}, errors.New("aggregate-enrollment-required")
	}
	if profile == tracerecordrepo.AggregateOutcomeProfile {
		return repo.finishAggregate(ctx, saved, command)
	}
	evaluation, err := repo.evaluate(ctx, saved)
	if err != nil || evaluation.Satisfied {
		return evaluation, err
	}
	// evaluate already reports base-not-ancestor-of-target as unmet; refuse here
	// too so a moved HEAD stops the flow before any evidence write below.
	if slices.Contains(evaluation.Unmet, "base-not-ancestor-of-target") {
		return evaluation, nil
	}
	snap, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	if !snap.clean {
		return evaluation, nil
	}
	for _, check := range saved.Plan.Checks {
		if !repo.checkQualifies(ctx, saved, check, snap) {
			return evaluation, nil
		}
	}
	bootstrap, err := repo.prepareBindings(ctx, saved, snap, command, &evaluation)
	if err != nil {
		evaluation.Unmet = append(evaluation.Unmet, "evidence-bindings-required")
		evaluation.NextActions = append(evaluation.NextActions, []string{"corvint", "cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", saved.Plan.Base, "--target", snap.target})
		for index := range saved.Plan.Intents {
			evaluation.NextActions = append(evaluation.NextActions, []string{"corvint", "ocm", "status", "--map", fmt.Sprintf(".corvint/change.ocm.%03d.json", index+1), "--cem", ".corvint/change.cem.json", "--expected-base", saved.Plan.Base, "--target", snap.target})
		}
		return evaluation, err
	}
	if !repo.reportCurrent(saved, snap) {
		if err = repo.generateReports(ctx, saved, snap, bootstrap, command); err != nil {
			return Evaluation{}, err
		}
		return repo.evaluate(ctx, saved)
	}
	if saved.Review != saved.ReportSet.Digest {
		return repo.evaluate(ctx, saved)
	}
	if err = repo.coordinate(ctx, saved, snap, command); err != nil {
		return Evaluation{}, err
	}
	if err = repo.revalidate(ctx, saved, snap); err != nil {
		return Evaluation{}, err
	}
	checked := repo.runFlow(ctx, command, func(ctx context.Context, steps dogfoodflow.Runner, stdout, stderr io.Writer) (int, error) {
		return dogfoodflow.Check(ctx, dogfoodflow.CheckOptions{Root: repo.auth.Root, Base: saved.Plan.Base, BaseVerifier: steps, TreeVerifier: steps}, stdout, stderr)
	})
	if err = repo.saveProcess("final-check", checked); err != nil {
		return Evaluation{}, err
	}
	if !processPassed(checked) {
		return Evaluation{}, errors.New("final-check-failed")
	}
	if err = repo.revalidate(ctx, saved, snap); err != nil {
		return Evaluation{}, err
	}
	artifacts, err := repo.terminalArtifacts()
	if err != nil {
		return Evaluation{}, err
	}
	if err = repo.checkCoordinatorContext(ctx, saved, snap, true); err != nil {
		return Evaluation{}, err
	}
	saved.Terminal = &terminal{ReportSet: saved.ReportSet.Digest, CheckExit: checked.code, Artifacts: artifacts}
	saved.Lifecycle = "satisfied"
	if err = repo.save(saved); err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, saved)
}

func (repo *repository) prepareBindings(ctx context.Context, saved *state, snap snapshot, command PublicCommand, evaluation *Evaluation) (int, error) {
	manifest := []byte(strings.Join(saved.Plan.Intents, "\n") + "\n")
	for index, intent := range saved.Plan.Intents {
		mapPath := fmt.Sprintf(".corvint/change.ocm.%03d.json", index+1)
		args := []string{"ocm", "prepare", "--map", mapPath, "--cem", ".corvint/change.cem.json", "--intent", intent, "--expected-base", saved.Plan.Base, "--target", snap.target}
		result, err := repo.public(ctx, command, args)
		appendEvidence(evaluation, "ocm-prepare", mapPath, result, err)
		if err != nil {
			return 0, err
		}
		result, err = repo.public(ctx, command, []string{"ocm", "status", "--map", mapPath, "--cem", ".corvint/change.cem.json", "--expected-base", saved.Plan.Base, "--target", snap.target})
		appendEvidence(evaluation, "ocm-status", mapPath, result, err)
		if err != nil {
			return 0, err
		}
	}
	if err := writeFile(filepath.Join(repo.auth.Root, ".corvint/change.ocm-intents"), manifest); err != nil {
		return 0, err
	}
	aggregate, err := dogfoodocm.Status(ctx, dogfoodocm.Options{Root: repo.auth.Root, CEMPath: ".corvint/change.cem.json", ExpectedBase: saved.Plan.Base, Target: snap.target})
	if err != nil {
		if code := lrfrepo.CodeOf(err); code != "" {
			return 0, errors.New(code)
		}
		return 0, err
	}
	if !aggregate.OK {
		return 0, errors.New("ocm-bindings-required")
	}
	// Decision 0029 leaves untouched obligations honestly unassessed. Keep
	// their raw unknown counts and reasons for review, while an explicitly
	// assessed evidence gap still prevents completion. Scope adequacy is
	// reviewer-owned; shared file paths cannot establish requirement coverage.
	for _, obligation := range aggregate.Worklist {
		if obligation.Disposition == "unknown" && obligation.Reason != "unassessed" {
			return 0, errors.New("ocm-bindings-required")
		}
	}
	bootstrap := 0
	if err = repo.refreshAuthority(); err != nil {
		return 0, err
	}
	for _, intent := range saved.Plan.Intents {
		_, exists, err := repo.auth.LookupTreeEntry(ctx, saved.Plan.Base, intent)
		if err != nil {
			return 0, err
		}
		if !exists {
			bootstrap++
		}
	}
	args := []string{"cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", saved.Plan.Base, "--target", snap.target, "--max-unknown", strconv.Itoa(bootstrap), "--max-mechanical", "0"}
	result, err := repo.public(ctx, command, args)
	appendEvidence(evaluation, "cem-status", ".corvint/change.cem.json", result, err)
	if err != nil {
		return 0, errors.New("cem-bindings-required")
	}
	return bootstrap, nil
}

func (repo *repository) public(ctx context.Context, command PublicCommand, args []string) (map[string]any, error) {
	stdout, stderr := boundedBuffer{limit: maxArtifactBytes}, boundedBuffer{limit: maxArtifactBytes}
	code := command(ctx, repo.auth.Root, args, &stdout, &stderr)
	if stdout.overflow || stderr.overflow {
		return nil, errors.New("public-command-output-bound")
	}
	var result map[string]any
	if code != 0 {
		_ = json.Unmarshal(stdout.Bytes(), &result)
		var failure struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(stderr.Bytes(), &failure)
		if validCode(failure.Code) {
			return result, errors.New(failure.Code)
		}
		return result, errors.New("public-evidence-command-failed")
	}
	if json.Unmarshal(stdout.Bytes(), &result) != nil || result["ok"] != true {
		return nil, errors.New("invalid-public-evidence-result")
	}
	return result, nil
}

func validCode(code string) bool {
	if len(code) < 1 || len(code) > 96 {
		return false
	}
	for _, c := range code {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return false
		}
	}
	return true
}
func appendEvidence(evaluation *Evaluation, tool, mapPath string, result map[string]any, err error) {
	item := EvidenceWorklist{Tool: tool, Map: mapPath, Worklist: []any{}}
	if err != nil && validCode(err.Error()) {
		item.Code = err.Error()
	}
	if rows, ok := result["worklist"].([]any); ok {
		if len(rows) > 256 {
			item.Omitted = len(rows) - 256
			rows = rows[:256]
		}
		item.Worklist = rows
	}
	evaluation.Evidence = append(evaluation.Evidence, item)
}

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (buffer *boundedBuffer) WriteString(value string) (int, error) {
	return buffer.Write([]byte(value))
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	if buffer.Len()+len(data) > buffer.limit {
		buffer.overflow = true
		return 0, errors.New("output-bound-exceeded")
	}
	return buffer.Buffer.Write(data)
}

// Prevent io.Copy from bypassing Write through promoted Buffer.ReadFrom.
func (buffer *boundedBuffer) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{buffer}, reader)
}

func (repo *repository) bindingPaths(saved *state) []string {
	paths := []string{repo.local("plan.json"), filepath.Join(repo.auth.Root, ".corvint/change.cem.json"), filepath.Join(repo.auth.Root, ".corvint/change.ocm-intents")}
	for index, intent := range saved.Plan.Intents {
		paths = append(paths, filepath.Join(repo.auth.Root, intent), filepath.Join(repo.auth.Root, fmt.Sprintf(".corvint/change.ocm.%03d.json", index+1)))
	}
	return paths
}

func (repo *repository) generateReports(ctx context.Context, saved *state, snap snapshot, bootstrap int, command PublicCommand) error {
	set := &reportSet{PlanDigest: saved.PlanDigest, Target: snap.target, Bindings: []artifact{}, Reports: []artifact{}, Observations: []observation{}}
	for _, name := range repo.bindingPaths(saved) {
		item, err := artifactFor(name)
		if err != nil {
			return err
		}
		set.Bindings = append(set.Bindings, item)
	}
	commands := [][]string{{"cem", "report", "--map", ".corvint/change.cem.json", "--expected-base", saved.Plan.Base, "--target", snap.target, "--max-unknown", strconv.Itoa(bootstrap), "--max-mechanical", "0"}}
	for index := range saved.Plan.Intents {
		commands = append(commands, []string{"ocm", "report", "--map", fmt.Sprintf(".corvint/change.ocm.%03d.json", index+1), "--cem", ".corvint/change.cem.json", "--expected-base", saved.Plan.Base, "--target", snap.target})
	}
	for index, args := range commands {
		result, err := repo.public(ctx, command, args)
		if err != nil {
			return err
		}
		name, ok := result["report"].(string)
		if !ok {
			return errors.New("public-report-path-missing")
		}
		// Public writers own their path validation. Restrict returned reads to
		// the private Git directory or repository before opening any bytes.
		if !repo.pathAllowed(name) {
			return errors.New("public-report-path-invalid")
		}
		raw, err := readFile(name, maxArtifactBytes)
		if err != nil {
			return err
		}
		destination := repo.local(fmt.Sprintf("report-%s-%02d.md", snap.target, index))
		if err = writeFile(destination, raw); err != nil {
			return err
		}
		set.Reports = append(set.Reports, artifact{Path: destination, Digest: digest(raw)})
	}
	for _, check := range saved.Plan.Checks {
		set.Observations = append(set.Observations, *repo.latest(saved, check.ID))
	}
	set.Digest = valueDigest(*set)
	previous := saved.ReportSet
	saved.ReportSet = set
	if err := repo.revalidate(ctx, saved, snap); err != nil {
		saved.ReportSet = previous
		return err
	}
	saved.Review = ""
	saved.Terminal = nil
	saved.Coordination = nil
	saved.Lifecycle = "active"
	return repo.save(saved)
}

func (repo *repository) pathAllowed(name string) bool {
	if !filepath.IsAbs(name) {
		return false
	}
	for _, base := range []string{repo.auth.Root, repo.auth.GitDir} {
		relative, err := filepath.Rel(base, name)
		if err == nil && validPath(filepath.ToSlash(relative)) {
			return true
		}
	}
	return false
}

func (repo *repository) revalidate(ctx context.Context, saved *state, before snapshot) error {
	if err := repo.requireOwner(); err != nil {
		return err
	}
	now, err := repo.snapshot(ctx)
	if err != nil {
		return err
	}
	if !now.clean || now.target != before.target || now.tree != before.tree || !repo.reportCurrent(saved, now) {
		return errors.New("completion-evidence-drift")
	}
	for _, check := range saved.Plan.Checks {
		if !repo.checkQualifies(ctx, saved, check, now) {
			return errors.New("selected-check-unverified")
		}
	}
	return nil
}

func (repo *repository) coordinate(ctx context.Context, saved *state, snap snapshot, command PublicCommand) error {
	if saved.AggregateOutcome != nil {
		return repo.checkCoordinatorContext(ctx, saved, snap, false)
	}
	if len(saved.Coordination) > 0 && repo.artifactsCurrent(saved.Coordination) && repo.checkCoordinatorContext(ctx, saved, snap, false) == nil {
		return nil
	}
	if err := repo.revalidate(ctx, saved, snap); err != nil {
		return err
	}
	intents := repo.local("intents.txt")
	citations := repo.local("citations.tsv")
	if err := writeFile(intents, []byte(strings.Join(saved.Plan.Intents, "\n")+"\n")); err != nil {
		return err
	}
	if err := writeFile(citations, []byte{}); err != nil {
		return err
	}
	var captured *aggregateLegacyCapture
	options := dogfoodflow.ChangeOptions{Root: repo.auth.Root, Base: saved.Plan.Base, Task: "Local completion " + saved.PlanDigest, Verify: displayChecks(saved.Plan), Outcome: "passed", IntentsFile: intents, Citations: citations}
	options.CaptureLegacyRecord = func(attempt dogfoodflow.LegacyRecordAttempt) error {
		capture, err := repo.captureLegacyAdmission(attempt)
		if err != nil {
			return err
		}
		if capture != nil {
			captured = capture
		}
		return nil
	}
	result := repo.runFlow(ctx, command, func(ctx context.Context, steps dogfoodflow.Runner, _, stderr io.Writer) (int, error) {
		options.Steps = steps
		return dogfoodflow.Change(ctx, options, stderr)
	})
	if err := repo.saveProcess("coordination-time", result); err != nil {
		return err
	}
	if captured != nil {
		if err := repo.completeLegacyFailure(saved, snap, captured); err != nil {
			return err
		}
	}
	if !processPassed(result) {
		return errors.New("dogfood-coordination-failed")
	}
	if err := repo.revalidate(ctx, saved, snap); err != nil {
		return err
	}
	if err := repo.checkCoordinatorContext(ctx, saved, snap, false); err != nil {
		return err
	}
	// Exclude mutable dogfoodCheck from retry identity; the outcome and OCM
	// aggregate themselves must stay exact and the report is reparsed each time.
	coordination := []artifact{}
	for _, name := range []string{filepath.Join(repo.auth.GitDir, "corvint/local-outcome.json"), filepath.Join(repo.auth.Root, ".corvint/change.ocm-status.json")} {
		item, err := artifactFor(name)
		if err != nil {
			return err
		}
		coordination = append(coordination, item)
	}
	saved.Coordination = coordination
	return repo.save(saved)
}

// flowResult is one bounded in-process run of the daily dogfood flow.
type flowResult struct {
	code           int
	err            error
	stdout, stderr []byte
	overflow       bool
}

// runFlow runs the daily change or check inside this binary with the caller's
// public command for every step (LCP-V0-014): no repository script, VERSION or
// source build, and no DOGFOOD_* or CORVINT_BIN environment reaches it.
func (repo *repository) runFlow(ctx context.Context, command PublicCommand, run func(context.Context, dogfoodflow.Runner, io.Writer, io.Writer) (int, error)) flowResult {
	self, err := os.Executable()
	if err != nil {
		return flowResult{err: err}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	stdout, stderr := boundedBuffer{limit: maxLogBytes}, boundedBuffer{limit: maxLogBytes}
	code, err := run(ctx, dogfoodflow.Runner{Path: self, Run: dogfoodflow.Command(command)}, &stdout, &stderr)
	return flowResult{code: code, err: err, stdout: stdout.Bytes(), stderr: stderr.Bytes(), overflow: stdout.overflow || stderr.overflow}
}

func processPassed(result flowResult) bool {
	return result.err == nil && result.code == 0 && !result.overflow
}
func (repo *repository) saveProcess(prefix string, result flowResult) error {
	if secretscreen.MatchString(string(result.stdout)) || secretscreen.MatchString(string(result.stderr)) {
		return errors.New("log-secret-screened")
	}
	if err := writeFile(repo.local(prefix+".stdout"), result.stdout); err != nil {
		return err
	}
	return writeFile(repo.local(prefix+".stderr"), result.stderr)
}

func (repo *repository) terminalArtifacts() ([]artifact, error) {
	items := []artifact{}
	paths := repo.terminalPaths()
	if len(paths) == 0 {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	for _, name := range paths {
		item, err := artifactFor(name)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repo *repository) terminalPaths() []string {
	paths := []string{filepath.Join(repo.auth.Root, ".corvint/dogfood-report.json"), filepath.Join(repo.auth.Root, ".corvint/change.ocm-status.json"), filepath.Join(repo.auth.GitDir, "corvint/local-outcome.json"), repo.local("final-check.stdout"), repo.local("final-check.stderr")}
	raw, err := readFile(paths[0], maxArtifactBytes)
	var report struct {
		ContextAbstention string `json:"contextAbstentionEvidenceSha256"`
	}
	if parsed, parseErr := dogfoodflow.ParseAggregateReport(raw); err == nil && parseErr == nil && parsed.Profile == dogfoodflow.AggregateReportProfile {
		saved, loadErr := repo.load()
		if loadErr != nil || saved.AggregateOutcome == nil {
			return nil
		}
		directory := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
		paths[3], paths[4] = filepath.Join(directory, "check.stdout"), filepath.Join(directory, "check.stderr")
		paths = append(paths, filepath.Join(directory, "check.capture.json"))
	}
	if err == nil && json.Unmarshal(raw, &report) == nil && report.ContextAbstention != "" {
		for _, name := range []string{"coordination-time-impact-abstention.json", "coordination-time-impact.argv", "coordination-time-impact.json", "coordination-time-impact.stderr"} {
			paths = append(paths, filepath.Join(repo.auth.GitDir, "corvint", name))
		}
	}
	return paths
}
func (repo *repository) terminalCurrent(value *terminal) bool {
	return repo.terminalCurrentContext(context.Background(), value)
}
func (repo *repository) terminalCurrentContext(ctx context.Context, value *terminal) bool {
	saved, err := repo.load()
	if err != nil {
		return false
	}
	if saved.AggregateOutcome != nil {
		if saved.AggregateOutcome.Phase != "COMMITTED" {
			return false
		}
		snap, err := repo.snapshot(ctx)
		if err != nil {
			return false
		}
		if repo.validateAggregateSemantic(ctx, saved, snap, true) != nil || repo.validateAggregateCheckCapture(saved, snap) != nil {
			return false
		}
	}
	paths := repo.terminalPaths()
	if len(paths) == 0 {
		return false
	}
	if len(value.Artifacts) != len(paths) {
		return false
	}
	for index, name := range paths {
		if value.Artifacts[index].Path != name {
			return false
		}
	}
	return repo.artifactsCurrent(value.Artifacts)
}

func (repo *repository) checkCoordinator(saved *state, snap snapshot, final bool) error {
	return repo.checkCoordinatorContext(context.Background(), saved, snap, final)
}
func (repo *repository) checkCoordinatorContext(ctx context.Context, saved *state, snap snapshot, final bool) error {
	if saved.AggregateOutcome != nil {
		if err := repo.validateAggregateSemantic(ctx, saved, snap, true); err != nil {
			return err
		}
		if final {
			return repo.validateAggregateCheckCapture(saved, snap)
		}
		return nil
	}
	raw, err := readFile(filepath.Join(repo.auth.Root, ".corvint/dogfood-report.json"), maxArtifactBytes)
	if err != nil {
		return err
	}
	var result struct {
		Base     string `json:"base"`
		Target   string `json:"target"`
		Complete bool   `json:"complete"`
		Outcome  string `json:"localOutcomeEvidenceSha256"`
		Check    struct {
			OutputsAgree bool `json:"outputsAgree"`
		} `json:"dogfoodCheck"`
	}
	if _, err = wire.Parse(raw); err != nil {
		return err
	}
	if json.Unmarshal(raw, &result) != nil || !result.Complete || result.Base != saved.Plan.Base || result.Target != snap.target {
		return errors.New("dogfood-report-drift")
	}
	item, err := artifactFor(filepath.Join(repo.auth.GitDir, "corvint/local-outcome.json"))
	if err != nil || result.Outcome != "sha256:"+item.Digest {
		return errors.New("local-outcome-evidence-drift")
	}
	if final && !result.Check.OutputsAgree {
		return errors.New("verifier-disagreement")
	}
	return nil
}

func aggregateOperationContext(ctx context.Context) (context.Context, error) {
	if gitrun.OperationBudgetFrom(ctx) != nil {
		return ctx, nil
	}
	budget, err := gitrun.NewOperationBudget(64, time.Now().Add(300*time.Second), false)
	if err != nil {
		return nil, err
	}
	return gitrun.WithOperationBudget(ctx, budget), nil
}

func (repo *repository) aggregateExpected(ctx context.Context, saved *state, snap snapshot) (tracerecordrepo.AggregateExpected, error) {
	format, err := repo.git(ctx, "rev-parse", "--show-object-format")
	if err != nil {
		return tracerecordrepo.AggregateExpected{}, err
	}
	if strings.TrimSpace(string(format)) != "sha1" {
		return tracerecordrepo.AggregateExpected{}, errors.New("aggregate-object-format-unsupported")
	}
	return tracerecordrepo.AggregateExpected{ObjectFormat: "sha1", Base: saved.Plan.Base, Target: snap.target, Tree: snap.tree, AdmissionPolicy: tracerecordrepo.AggregateAdmissionPolicy, Task: "Local completion " + saved.PlanDigest, Verification: []string{displayChecks(saved.Plan)}, Outcome: "passed"}, nil
}

// Selected CEM/check qualification is outside the aggregate Git suboperation,
// as specified by ALO-V0-017. Its immutable inputs are bracketed again below.
func (repo *repository) aggregatePrerequisites(ctx context.Context, saved *state, snap snapshot) error {
	if err := repo.requireOwner(); err != nil {
		return err
	}
	if !snap.clean {
		return errors.New("uncommitted-work")
	}
	if !repo.reportCurrent(saved, snap) {
		return errors.New("report-set-stale")
	}
	if saved.ReportSet == nil || saved.Review != saved.ReportSet.Digest {
		return errors.New("review-required")
	}
	for _, check := range saved.Plan.Checks {
		if !repo.checkQualifies(gitrun.WithOperationBudget(ctx, nil), saved, check, snap) {
			return errors.New("selected-check-unverified")
		}
	}
	return nil
}

// validateAggregateSemantic is deliberately not a digest cache. Every positive
// consumer runs complete source-set validation in the contained native worker,
// bracketed by immutable identity and state/artifact reads without any write.
func (repo *repository) validateAggregateSemantic(ctx context.Context, saved *state, snap snapshot, requireComplete bool) error {
	if err := dogfoodoperation.Check(ctx); err != nil {
		return err
	}
	if err := repo.aggregatePrerequisites(ctx, saved, snap); err != nil {
		return err
	}
	opCtx, err := aggregateOperationContext(ctx)
	if err != nil {
		return err
	}
	opening, err := repo.snapshot(opCtx)
	if err != nil {
		return err
	}
	if !opening.clean || opening.target != snap.target || opening.tree != snap.tree {
		return errors.New("aggregate-binding-drift")
	}
	stateBefore, err := readFile(repo.local("state.json"), maxStateBytes)
	if err != nil {
		return err
	}
	var observedState state
	if strictJSON(stateBefore, &observedState, maxStateBytes) != nil || valueDigest(observedState) != valueDigest(saved) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if err = repo.requireBaseAncestor(opCtx, saved.Plan.Base, opening.target); err != nil {
		return err
	}
	a := saved.AggregateOutcome
	if a == nil {
		return errors.New("aggregate-enrollment-required")
	}
	if err = repo.validateAggregateLedger(a); err != nil {
		return err
	}
	rank, err := repo.aggregatePublishedPrefix(saved)
	if err != nil {
		return err
	}
	if rank < 1 || (requireComplete && rank < 2) {
		return errors.New("final-check-required")
	}
	expected, err := repo.aggregateExpected(opCtx, saved, opening)
	if err != nil {
		return err
	}
	if _, _, err = repo.validateLegacyFailure(saved, opening); err != nil {
		return err
	}
	outcome, err := readFile(repo.aggregateSourcePaths()["prior-outcome"], maxArtifactBytes)
	if err != nil {
		return err
	}
	report, reportErr := readFile(repo.aggregateSourcePaths()["prior-report"], maxArtifactBytes)
	if reportErr != nil && !os.IsNotExist(reportErr) {
		return reportErr
	}
	if _, err = dogfoodflow.RunAggregateWorker(opCtx, repo.auth.Root, "verify", expected, outcome); err != nil {
		return err
	}
	if rank >= 2 {
		value, err := dogfoodflow.ParseAggregateReport(report)
		if err != nil {
			return err
		}
		if requireComplete && value.CompletionState != "complete" {
			return errors.New("final-check-required")
		}
	}
	closing, err := repo.snapshot(opCtx)
	if err != nil {
		return err
	}
	if !closing.clean || closing.target != opening.target || closing.tree != opening.tree || !repo.reportCurrent(saved, closing) {
		return errors.New("aggregate-binding-drift")
	}
	if err = repo.requireOwner(); err != nil {
		return err
	}
	stateAfter, err := readFile(repo.local("state.json"), maxStateBytes)
	if err != nil || !bytes.Equal(stateBefore, stateAfter) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	afterOutcome, err := readFile(repo.aggregateSourcePaths()["prior-outcome"], maxArtifactBytes)
	if err != nil || !bytes.Equal(outcome, afterOutcome) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	afterReport, afterErr := readFile(repo.aggregateSourcePaths()["prior-report"], maxArtifactBytes)
	if !bytes.Equal(report, afterReport) || (reportErr == nil) != (afterErr == nil) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if err := opCtx.Err(); err != nil {
		return err
	}
	if !time.Now().Before(gitrun.OperationBudgetFrom(opCtx).Deadline()) {
		return errors.New("aggregate-operation-deadline")
	}
	return nil
}

func openAggregateOwner(root string) (*repository, *state, error) {
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, nil, err
	}
	ownerRepo := &repository{auth: auth, directory: filepath.Join(auth.GitDir, "corvint", "local-completion")}
	key, err := ownerRepo.owner()
	if err != nil || key == "" {
		return nil, nil, errors.New("aggregate-enrollment-required")
	}
	repo, err := open(auth.Root, key)
	if err != nil {
		return nil, nil, err
	}
	saved, err := repo.load()
	if err != nil {
		return nil, nil, err
	}
	if saved.AggregateOutcome == nil {
		return nil, nil, errors.New("aggregate-enrollment-required")
	}
	return repo, saved, nil
}

// ConfigureAggregateCheck supplies native owner/state authority rather than
// accepting identities from the report. Public callers require COMMITTED;
// only explicit enrolled Finish enables its already-published pending report.
func ConfigureAggregateCheck(options dogfoodflow.CheckOptions, allowPending bool) dogfoodflow.CheckOptions {
	prepared, admitted := false, false
	session, generation, binding, beforeReport, stateIdentity := "", "", "", "", ""
	options.AggregateBegin = func(ctx context.Context, root string, raw []byte) (*dogfoodflow.AggregateCheckStage, error) {
		repo, saved, err := openAggregateOwner(root)
		if err != nil {
			return nil, err
		}
		held, release, err := dogfoodoperation.Acquire(ctx, repo.auth.GitDir)
		if err != nil {
			return nil, err
		}
		defer release()
		if !allowPending && saved.AggregateOutcome.Phase != "COMMITTED" {
			return nil, errors.New("final-check-required")
		}
		opCtx, err := aggregateOperationContext(held)
		if err != nil {
			return nil, err
		}
		snap, err := repo.snapshot(opCtx)
		if err != nil {
			return nil, err
		}
		if err = repo.aggregatePrerequisites(opCtx, saved, snap); err != nil {
			return nil, err
		}
		current, err := readFile(repo.aggregateSourcePaths()["prior-report"], maxArtifactBytes)
		if err != nil || !bytes.Equal(current, raw) {
			return nil, errors.New("aggregate-prior-evidence-drift")
		}
		receipt, err := readFile(repo.aggregateSourcePaths()["prior-outcome"], maxArtifactBytes)
		if err != nil {
			return nil, err
		}
		if err = repo.preserveAggregateReplacement(opCtx, saved, receipt); err != nil {
			return nil, err
		}
		stage, err := repo.beginAggregateCheckCapture(opCtx, saved, snap, raw)
		if err != nil {
			return nil, err
		}
		stateRaw, err := readFile(repo.local("state.json"), maxStateBytes)
		if err != nil {
			_, _ = stage.Finish(-1, err)
			return nil, err
		}
		session, generation, binding = saved.Session, saved.Generation, saved.AggregateOutcome.BindingSHA256
		beforeReport, stateIdentity = prefixedDigest(raw), prefixedDigest(stateRaw)
		admitted = true
		return stage, nil
	}
	options.AggregateValidate = func(ctx context.Context, root string, raw []byte) error {
		if !admitted {
			return errors.New("aggregate-enrollment-required")
		}
		if err := dogfoodoperation.Check(ctx); err != nil {
			return err
		}
		repo, saved, err := openAggregateOwner(root)
		if err != nil {
			return err
		}
		held, release, err := dogfoodoperation.Acquire(ctx, repo.auth.GitDir)
		if err != nil {
			return err
		}
		defer release()
		if saved.Session != session || saved.Generation != generation || saved.AggregateOutcome.BindingSHA256 != binding || prefixedDigest(raw) != beforeReport {
			return errors.New("aggregate-binding-drift")
		}
		stateRaw, err := readFile(repo.local("state.json"), maxStateBytes)
		if err != nil || prefixedDigest(stateRaw) != stateIdentity {
			return errors.New("aggregate-prior-evidence-drift")
		}
		opCtx, err := aggregateOperationContext(held)
		if err != nil {
			return err
		}
		snap, err := repo.snapshot(opCtx)
		if err != nil {
			return err
		}
		if err = repo.validateAggregateSemantic(opCtx, saved, snap, true); err != nil {
			return err
		}
		prepared = true
		return nil
	}
	options.AggregatePublish = func(ctx context.Context, root string, capture dogfoodflow.AggregateCheckCapture) error {
		if !prepared {
			return errors.New("aggregate-enrollment-required")
		}
		prepared = false
		repo, saved, err := openAggregateOwner(root)
		if err != nil {
			return err
		}
		held, release, err := dogfoodoperation.Acquire(ctx, repo.auth.GitDir)
		if err != nil {
			return err
		}
		defer release()
		if saved.Session != session || saved.Generation != generation || saved.AggregateOutcome.BindingSHA256 != binding || prefixedDigest(capture.OriginalReport) != beforeReport {
			return errors.New("aggregate-binding-drift")
		}
		stateRaw, err := readFile(repo.local("state.json"), maxStateBytes)
		if err != nil || prefixedDigest(stateRaw) != stateIdentity {
			return errors.New("aggregate-prior-evidence-drift")
		}
		opCtx, err := aggregateOperationContext(held)
		if err != nil {
			return err
		}
		snap, err := repo.snapshot(opCtx)
		if err != nil {
			return err
		}
		if err = repo.validateAggregateSemantic(opCtx, saved, snap, true); err != nil {
			return err
		}
		if err = repo.saveAggregateCheckCapture(opCtx, saved, snap, capture); err != nil {
			return err
		}
		if capture.Exit != 0 || capture.Failed {
			return nil
		}
		old, err := dogfoodflow.ParseAggregateReport(capture.OriginalReport)
		if err != nil {
			return err
		}
		next, err := dogfoodflow.ParseAggregateReport(capture.ProposedReport)
		if err != nil {
			return err
		}
		if next.DogfoodCheck == nil || !next.DogfoodCheck.OutputsAgree {
			return errors.New("final-check-required")
		}
		old.DogfoodCheck = nil
		nextWithoutCheck := next
		nextWithoutCheck.DogfoodCheck = nil
		if valueDigest(old) != valueDigest(nextWithoutCheck) {
			return errors.New("aggregate-binding-drift")
		}
		if err = repo.publishAggregateRole(opCtx, saved, "check-report", capture.ProposedReport); err != nil {
			return err
		}
		return repo.commitAggregate(opCtx, saved, snap)
	}
	return options
}

func (repo *repository) commitAggregate(ctx context.Context, saved *state, snap snapshot) error {
	if err := repo.validateAggregateCheckCapture(saved, snap); err != nil {
		return err
	}
	if err := repo.validateAggregateSemantic(ctx, saved, snap, true); err != nil {
		return err
	}
	artifacts, err := repo.terminalArtifacts()
	if err != nil {
		return err
	}
	saved.Terminal = &terminal{ReportSet: saved.ReportSet.Digest, CheckExit: 0, Artifacts: artifacts}
	saved.AggregateOutcome.Phase = "COMMITTED"
	saved.Lifecycle = "satisfied"
	if err = repo.aggregateFaultPoint("before-terminal-state"); err != nil {
		return err
	}
	if err = aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if err = repo.save(saved); err != nil {
		return err
	}
	return repo.aggregateFaultPoint("terminal-state-committed")
}

func (repo *repository) finishAggregate(ctx context.Context, saved *state, command PublicCommand) (Evaluation, error) {
	// An identical qualified success is a semantic read-only no-op before any
	// preservation admission, snapshot ordinal, publication or state change.
	if saved.AggregateOutcome != nil && saved.AggregateOutcome.Phase == "COMMITTED" {
		evaluation, err := repo.evaluate(ctx, saved)
		if err != nil || evaluation.Satisfied {
			return evaluation, err
		}
	}
	snap, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	if err = repo.aggregatePrerequisites(ctx, saved, snap); err != nil {
		return Evaluation{}, err
	}
	opCtx, err := aggregateOperationContext(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	opening, err := repo.snapshot(opCtx)
	if err != nil {
		return Evaluation{}, err
	}
	if !opening.clean || opening.target != snap.target || opening.tree != snap.tree {
		return Evaluation{}, errors.New("aggregate-binding-drift")
	}
	if err = repo.requireBaseAncestor(opCtx, saved.Plan.Base, snap.target); err != nil {
		return Evaluation{}, err
	}
	expected, err := repo.aggregateExpected(opCtx, saved, snap)
	if err != nil {
		return Evaluation{}, err
	}
	failure, oldReport, err := repo.validateLegacyFailure(saved, snap)
	if err != nil {
		return Evaluation{}, err
	}
	var receipt []byte
	if saved.AggregateOutcome == nil {
		// Capacity is checked before starting the producer, using the actual old
		// evidence sizes and fixed remaining receipt/report/capture maxima.
		used, err := repo.aggregateHistoryUsage()
		if err != nil {
			return Evaluation{}, err
		}
		_, oldPayloads, err := repo.aggregateCaptureSnapshot(saved, "sha256:"+strings.Repeat("0", 64))
		if err != nil {
			return Evaluation{}, err
		}
		needed := int64(4*maxArtifactBytes + 4*maxLogBytes + 6*maxStateBytes)
		for _, raw := range oldPayloads {
			needed += int64(len(raw))
		}
		if used+needed > aggregateHistoryLimit {
			return Evaluation{}, errors.New("aggregate-history-bound-exceeded")
		}
		receipt, err = dogfoodflow.RunAggregateWorker(opCtx, repo.auth.Root, "produce", expected, nil)
		if err != nil {
			return Evaluation{}, err
		}
		// Producer bytes are never adopted solely on their self-consistent hashes.
		if _, err = dogfoodflow.RunAggregateWorker(opCtx, repo.auth.Root, "verify", expected, receipt); err != nil {
			return Evaluation{}, err
		}
		value, err := tracerecordrepo.ParseAggregateOutcome(receipt)
		if err != nil {
			return Evaluation{}, err
		}
		binding, err := tracerecordrepo.AggregateBinding(value)
		if err != nil {
			return Evaluation{}, err
		}
		oldOutcome, outErr := readFile(repo.aggregateSourcePaths()["prior-outcome"], maxArtifactBytes)
		liveReport, reportErr := readFile(repo.aggregateSourcePaths()["prior-report"], maxArtifactBytes)
		if outErr != nil || reportErr != nil || prefixedDigest(oldOutcome) != failure.StdoutSHA256 || !bytes.Equal(liveReport, oldReport) {
			return Evaluation{}, errors.New("aggregate-prior-evidence-drift")
		}
		failureRaw, err := readFile(repo.local("aggregate-outcome/legacy-failure.json"), maxStateBytes)
		if err != nil {
			return Evaluation{}, err
		}
		proposed := &aggregateState{Profile: tracerecordrepo.AggregateOutcomeProfile, BindingSHA256: binding, ReceiptSHA256: prefixedDigest(receipt), LegacyFailureSHA256: prefixedDigest(failureRaw), Phase: "PREPARED", Issued: []aggregateIssued{}, ExpectedOld: aggregateExpectedOld{&aggregateOld{prefixedDigest(oldOutcome), int64(len(oldOutcome)), "failed-recorder-output"}, &aggregateOld{prefixedDigest(oldReport), int64(len(oldReport)), "legacy-report"}}}
		if err = repo.preserveAggregate(opCtx, saved, proposed, receipt); err != nil {
			return Evaluation{}, err
		}
	} else {
		a := saved.AggregateOutcome
		directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
		receipt, err = readFile(filepath.Join(directory, "outcome.prepared"), maxArtifactBytes)
		if err != nil || prefixedDigest(receipt) != a.ReceiptSHA256 {
			return Evaluation{}, errors.New("aggregate-prior-evidence-drift")
		}
		if _, err = dogfoodflow.RunAggregateWorker(opCtx, repo.auth.Root, "verify", expected, receipt); err != nil {
			return Evaluation{}, err
		}
		if err = repo.preserveAggregate(opCtx, saved, a, receipt); err != nil {
			return Evaluation{}, err
		}
	}
	a := saved.AggregateOutcome
	if err = repo.publishAggregateRole(opCtx, saved, "outcome", receipt); err != nil {
		return Evaluation{}, err
	}
	rank, err := repo.aggregatePublishedPrefix(saved)
	if err != nil {
		return Evaluation{}, err
	}
	if rank < 2 {
		report, err := dogfoodflow.PrepareAggregateReport(oldReport, dogfoodflow.AggregateEnrollment{Session: saved.Session, Generation: saved.Generation, PlanDigest: saved.PlanDigest, BindingSHA256: a.BindingSHA256}, receipt)
		if err != nil {
			return Evaluation{}, err
		}
		if err = repo.publishAggregateRole(opCtx, saved, "report", report); err != nil {
			return Evaluation{}, err
		}
	}
	closing, err := repo.snapshot(opCtx)
	if err != nil {
		return Evaluation{}, err
	}
	if !closing.clean || closing.target != opening.target || closing.tree != opening.tree || !repo.reportCurrent(saved, closing) {
		return Evaluation{}, errors.New("aggregate-binding-drift")
	}
	if err = opCtx.Err(); err != nil {
		return Evaluation{}, err
	}
	if !time.Now().Before(gitrun.OperationBudgetFrom(opCtx).Deadline()) {
		return Evaluation{}, errors.New("aggregate-operation-deadline")
	}
	// A check report issued before an interrupted rename/terminal ack retains its
	// actual immutable zero-exit capture. Resume publication without inventing a run.
	if len(a.Issued) == 3 {
		raw, err := readFile(filepath.Join(repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256), "check-report.prepared"), maxArtifactBytes)
		if err != nil {
			return Evaluation{}, err
		}
		if err = repo.publishAggregateRole(opCtx, saved, "check-report", raw); err != nil {
			return Evaluation{}, err
		}
		if err = repo.commitAggregate(opCtx, saved, closing); err != nil {
			return Evaluation{}, err
		}
		return repo.evaluate(ctx, saved)
	}
	reportRaw, err := readFile(repo.aggregateSourcePaths()["prior-report"], maxArtifactBytes)
	if err != nil {
		return Evaluation{}, err
	}
	report, err := dogfoodflow.ParseAggregateReport(reportRaw)
	if err != nil {
		return Evaluation{}, err
	}
	if report.CompletionState != "complete" {
		return repo.evaluate(ctx, saved)
	}
	checked := repo.runFlow(ctx, command, func(ctx context.Context, steps dogfoodflow.Runner, stdout, stderr io.Writer) (int, error) {
		options := ConfigureAggregateCheck(dogfoodflow.CheckOptions{Root: repo.auth.Root, Base: saved.Plan.Base, BaseVerifier: steps, TreeVerifier: steps}, true)
		return dogfoodflow.Check(ctx, options, stdout, stderr)
	})
	if !processPassed(checked) {
		if err := dogfoodoperation.Check(ctx); err != nil {
			return Evaluation{}, err
		}
		return Evaluation{}, errors.New("final-check-failed")
	}
	current, err := repo.load()
	if err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, current)
}
