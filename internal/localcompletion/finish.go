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

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// PublicCommand invokes the existing CEM/OCM entrypoints, without a shell or
// reinterpretation of their evidence semantics.
type PublicCommand func(context.Context, string, []string, io.Writer, io.Writer) int

func Finish(ctx context.Context, root, key string, command PublicCommand) (Evaluation, error) {
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	unlock, err := repo.lock()
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
	if err = repo.coordinate(ctx, saved, snap); err != nil {
		return Evaluation{}, err
	}
	if err = repo.revalidate(ctx, saved, snap); err != nil {
		return Evaluation{}, err
	}
	checked := repo.runScript(ctx, "dogfood-check.sh", saved.Plan.Base, nil)
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
	if err = repo.checkCoordinator(saved, snap, true); err != nil {
		return Evaluation{}, err
	}
	saved.Terminal = &terminal{ReportSet: saved.ReportSet.Digest, CheckExit: checked.ExitStatus, Artifacts: artifacts}
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
	var stdout, stderr boundedBuffer
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
	overflow bool
}

func (buffer *boundedBuffer) WriteString(value string) (int, error) {
	return buffer.Write([]byte(value))
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	if buffer.Len()+len(data) > maxArtifactBytes {
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

func (repo *repository) coordinate(ctx context.Context, saved *state, snap snapshot) error {
	if len(saved.Coordination) > 0 && repo.artifactsCurrent(saved.Coordination) && repo.checkCoordinator(saved, snap, false) == nil {
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
	env := []string{"DOGFOOD_TASK=Local completion " + saved.PlanDigest, "DOGFOOD_VERIFY=" + displayChecks(saved.Plan), "DOGFOOD_OUTCOME=passed", "DOGFOOD_INTENTS_FILE=" + intents, "DOGFOOD_CITATIONS=" + citations}
	result := repo.runScript(ctx, "dogfood-change.sh", saved.Plan.Base, env)
	if err := repo.saveProcess("coordination-time", result); err != nil {
		return err
	}
	if !processPassed(result) {
		return errors.New("dogfood-coordination-failed")
	}
	if err := repo.revalidate(ctx, saved, snap); err != nil {
		return err
	}
	if err := repo.checkCoordinator(saved, snap, false); err != nil {
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

func (repo *repository) runScript(ctx context.Context, name, base string, extra []string) procgroup.Observation {
	env := []string{}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "DOGFOOD_") || strings.HasPrefix(entry, "CORVINT_BIN=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, extra...)
	bash, err := resolveExecutable("bash")
	if err != nil {
		return procgroup.Observation{Err: err, ExitStatus: -1}
	}
	return procgroup.Run(ctx, procgroup.Spec{Argv: []string{bash, filepath.Join(repo.auth.Root, "script", name), base}, Dir: repo.auth.Root, Env: env, Timeout: 10 * time.Minute, OutputLimit: maxLogBytes, StderrLimit: maxLogBytes})
}

func processPassed(result procgroup.Observation) bool {
	return result.Err == nil && result.ExitObserved && result.ExitStatus == 0 && result.WaitCompleted && result.PipesDrained && result.OwnedProcessGroupCleanup && !result.TimedOut && !result.Cancelled && !result.OutputOverflow
}
func (repo *repository) saveProcess(prefix string, result procgroup.Observation) error {
	if secretscreen.MatchString(string(result.Stdout)) || secretscreen.MatchString(string(result.Stderr)) {
		return errors.New("log-secret-screened")
	}
	if err := writeFile(repo.local(prefix+".stdout"), result.Stdout); err != nil {
		return err
	}
	return writeFile(repo.local(prefix+".stderr"), result.Stderr)
}

func (repo *repository) terminalArtifacts() ([]artifact, error) {
	items := []artifact{}
	for _, name := range repo.terminalPaths() {
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
	if err == nil && json.Unmarshal(raw, &report) == nil && report.ContextAbstention != "" {
		for _, name := range []string{"prechange-impact-abstention.json", "prechange-impact.argv", "prechange-impact.json", "prechange-impact.stderr"} {
			paths = append(paths, filepath.Join(repo.auth.GitDir, "corvint", name))
		}
	}
	return paths
}
func (repo *repository) terminalCurrent(value *terminal) bool {
	paths := repo.terminalPaths()
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
