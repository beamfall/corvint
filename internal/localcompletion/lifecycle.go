package localcompletion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

func Begin(ctx context.Context, root, key string, raw []byte) (Evaluation, error) {
	var plan Plan
	if err := strictJSON(raw, &plan, MaxPlanBytes); err != nil {
		return Evaluation{}, err
	}
	if err := validatePlan(plan); err != nil {
		return Evaluation{}, err
	}
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	unlock, err := repo.lock()
	if err != nil {
		return Evaluation{}, err
	}
	defer unlock()
	base, err := repo.auth.Resolve(ctx, plan.Base)
	if err != nil || base != plan.Base {
		return Evaluation{}, errors.New("base-unavailable")
	}
	owner, err := repo.owner()
	if err != nil {
		return Evaluation{}, err
	}
	if owner != "" {
		other := *repo
		other.session = owner
		prior, loadErr := other.load()
		if loadErr != nil {
			return Evaluation{}, loadErr
		}
		if prior.Lifecycle == "active" && owner != key {
			return Evaluation{}, errors.New("worktree-already-enrolled")
		}
		if prior.Lifecycle == "satisfied" && owner != key {
			current, evaluationErr := other.evaluate(ctx, prior)
			if evaluationErr != nil {
				return Evaluation{}, evaluationErr
			}
			if !current.Satisfied {
				return Evaluation{}, errors.New("worktree-prior-completion-stale")
			}
		}
	}
	existing, err := repo.load()
	if err == nil {
		if existing.PlanDigest != valueDigest(plan) || existing.Lifecycle == "cancelled" {
			if existing.Lifecycle == "active" {
				return Evaluation{}, errors.New("enrollment-plan-conflict")
			}
			if existing.Lifecycle == "satisfied" {
				previous, evaluationErr := repo.evaluate(ctx, existing)
				if evaluationErr != nil {
					return Evaluation{}, evaluationErr
				}
				if !previous.Satisfied {
					return Evaluation{}, errors.New("prior-completion-stale")
				}
			}
			if err = repo.archiveGeneration(existing); err != nil {
				return Evaluation{}, err
			}
		} else {
			if existing.Lifecycle == "active" && owner != key {
				if err = writeFile(filepath.Join(repo.directory, "owner"), []byte(key+"\n")); err != nil {
					return Evaluation{}, err
				}
			}
			return repo.evaluate(ctx, existing)
		}
	} else if !os.IsNotExist(err) {
		return Evaluation{}, err
	}
	generation, err := repo.nextGeneration(valueDigest(plan))
	if err != nil {
		return Evaluation{}, err
	}
	repo.generation = generation
	snap, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	// finish diffs base against HEAD as two trees, so only ancestry makes that
	// delta this history's change; refuse before any local write.
	if err = repo.requireBaseAncestor(ctx, plan.Base, snap.target); err != nil {
		return Evaluation{}, err
	}
	saved := &state{Session: key, Lifecycle: "active", Plan: plan, PlanDigest: valueDigest(plan), Generation: generation, Observations: []observation{}, IntentPointers: []IntentPointer{}}
	for _, check := range plan.Checks {
		executable, resolveErr := resolveExecutable(check.Argv[0])
		if resolveErr != nil {
			return Evaluation{}, resolveErr
		}
		saved.Executables = append(saved.Executables, executable)
	}
	// Intents resolve before plan.json is written: that write creates the
	// generation directory, so a refusal after it would consume a generation.
	for _, intent := range plan.Intents {
		pointer := IntentPointer{Path: intent, Revision: snap.target}
		entry, present, lookupErr := repo.auth.LookupTreeEntry(ctx, snap.target, intent)
		if lookupErr != nil {
			return Evaluation{}, lookupErr
		}
		// Intents pin at enrollment HEAD, not the base, so finish's bootstrap intents
		// resolve (decision 0170); a path HEAD lacks would pin an empty blob hash.
		if !present {
			return Evaluation{}, errors.New("intent-path-not-found")
		}
		// Finish prepares every intent's numbered Requirements section; refuse a
		// section it would reject now, before a generation exists (LCP-V0-037).
		var data []byte
		over := entry.Type != "blob"
		if !over {
			if data, over, lookupErr = repo.auth.BlobBytesWithin(ctx, entry.OID, lrfrepo.MaxIntentBytes); lookupErr != nil {
				return Evaluation{}, lookupErr
			}
		}
		if over {
			return Evaluation{}, errors.New("repository-object-unavailable")
		}
		if code := lrfrepo.IntentRequirementsRefusal(data); code != "" {
			return Evaluation{}, errors.New(code)
		}
		pointer.BlobHash = entry.OID
		saved.IntentPointers = append(saved.IntentPointers, pointer)
	}
	planRaw, _ := json.Marshal(plan)
	if err = writeFile(repo.local("plan.json"), planRaw); err != nil {
		return Evaluation{}, err
	}
	// Preserve the genuine initial receipts before later coordination overwrites
	// its global files. Absence remains absence rather than a synthetic receipt.
	for _, name := range []string{"prechange-query.json", "prechange-impact.json"} {
		data, readErr := readFile(filepath.Join(repo.auth.GitDir, "corvint", name), maxArtifactBytes)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return Evaluation{}, readErr
		}
		if secretscreen.MatchString(string(data)) {
			metadata, _ := json.Marshal(map[string]string{"originalDigest": digest(data), "state": "NOT_PRODUCED", "reason": "initial-receipt-secret-screened"})
			if err = writeFile(repo.local("initial-"+name+".refusal.json"), metadata); err != nil {
				return Evaluation{}, err
			}
			continue
		}
		if err = writeFile(repo.local("initial-"+name), data); err != nil {
			return Evaluation{}, err
		}
	}
	if err = repo.save(saved); err != nil {
		return Evaluation{}, err
	}
	if err = writeFile(filepath.Join(repo.directory, "owner"), []byte(key+"\n")); err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, saved)
}

// Observed contains immutable repository facts from a completed evaluation.
// Its zero value cannot match a repository observation.
type Observed struct {
	target, tree, content   string
	root, gitDir, commonDir string
}

// Matches proves that the caller's closing observation names the evaluated content.
func (observed Observed) Matches(target, tree string) bool {
	return observed.target != "" && observed.tree != "" && observed.target == target && observed.tree == tree
}

func Evaluate(ctx context.Context, root, key string) (Evaluation, error) {
	result, _, err := EvaluateObserved(ctx, root, key)
	return result, err
}

// EvaluateObserved evaluates policy and returns the immutable facts it proved.
func EvaluateObserved(ctx context.Context, root, key string) (Evaluation, Observed, error) {
	var observed Observed
	result, err := evaluateRead(ctx, root, key, nil, &observed)
	return result, observed, err
}

// EvaluateAgainst re-reads every private policy input and checks Git status before
// and after. The caller must first prove observed.Matches against its closing
// repository observation. An absent or cross-repository observation falls back
// to full evaluation; ancestry is always re-proved.
func EvaluateAgainst(ctx context.Context, root, key string, observed Observed) (Evaluation, error) {
	return evaluateRead(ctx, root, key, &observed, nil)
}

func evaluateRead(ctx context.Context, root, key string, pinned, observed *Observed) (Evaluation, error) {
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	saved, err := repo.load()
	if os.IsNotExist(err) {
		return repo.inactive()
	}
	if err != nil {
		return Evaluation{}, err
	}
	if pinned != nil && (!pinned.Matches(pinned.target, pinned.tree) || pinned.root != repo.auth.Root || pinned.gitDir != repo.auth.GitDir || pinned.commonDir != repo.auth.CommonDir) {
		pinned = nil
	}
	return repo.evaluateObserved(ctx, saved, pinned, observed)
}

func (repo *repository) evaluate(ctx context.Context, saved *state) (Evaluation, error) {
	return repo.evaluateObserved(ctx, saved, nil, nil)
}

func (repo *repository) evaluateObserved(ctx context.Context, saved *state, pinned, observed *Observed) (Evaluation, error) {
	result := Evaluation{Lifecycle: saved.Lifecycle, Unmet: []string{}, Base: saved.Plan.Base, PlanDigest: saved.PlanDigest, Intents: saved.Plan.Intents, IntentPointers: saved.IntentPointers, Plan: &saved.Plan}
	if saved.Lifecycle == "cancelled" {
		result.NoActionReason = "enrollment-cancelled"
		return result, nil
	}
	var snap snapshot
	var err error
	if pinned == nil {
		snap, err = repo.snapshot(ctx)
	} else {
		snap = snapshot{target: pinned.target, tree: pinned.tree, content: pinned.content}
		if err = repo.refreshAuthority(); err == nil {
			snap.clean, err = repo.clean(ctx)
		}
	}
	if err != nil {
		return result, err
	}
	result.Target = snap.target
	if saved.Sealed != nil {
		result.EvidenceTarget = saved.Sealed.Binding.EvidenceTarget
	}
	// finish diffs base against HEAD as two trees, so ancestry must still hold
	// here too: HEAD can move to unrelated history after begin already checked it.
	// Ancestry depends on mutable shallow/graft metadata, so it is re-proved
	// on every read rather than reused from the observation.
	err = repo.requireBaseAncestor(ctx, saved.Plan.Base, snap.target)
	if err != nil {
		if err.Error() != "base-not-ancestor-of-target" {
			return result, err
		}
		result.Unmet = append(result.Unmet, "base-not-ancestor-of-target")
	}
	if err = repo.requireOwner(); err != nil {
		result.Unmet = append(result.Unmet, "worktree-owner-mismatch")
	}
	if !snap.clean {
		result.Unmet = append(result.Unmet, "uncommitted-work")
	}
	repositoryReady := len(result.Unmet) == 0
	var verificationActions [][]string
	for _, check := range saved.Plan.Checks {
		qualified, refused := repo.checkQualification(ctx, saved, check, snap)
		summary := CheckObservation{ID: check.ID, Qualified: qualified, Argv: repo.executionArgv(saved, check), CurrentTarget: snap.target, Exit: -1, ReuseRefused: refused}
		if observed := repo.latest(saved, check.ID); observed != nil {
			summary.TestedCommit = observed.Target
			summary.Exit, _ = strconv.Atoi(observed.Exit)
			summary.TimedOut = observed.TimedOut
			summary.Cancelled = observed.Cancelled
			summary.SecretScreened = observed.SecretScreened
			summary.Stdout = observed.Stdout.Path
			summary.Stderr = observed.Stderr.Path
		}
		result.Checks = append(result.Checks, summary)
		if !qualified {
			result.Unmet = append(result.Unmet, "selected-check-unverified:"+check.ID)
			verificationActions = append(verificationActions, []string{"corvint", "dogfood", "verify", "--session-key", repo.session, "--check", check.ID})
		}
	}
	attemptsExhausted := len(verificationActions) > 0 && len(saved.Observations) >= maxAttempts
	if attemptsExhausted {
		result.Unmet = append(result.Unmet, "verification-attempt-bound-exceeded")
	}
	reviewRequired := false
	if saved.ReportSet == nil {
		result.Unmet = append(result.Unmet, "reports-not-produced")
	} else if !repo.reportCurrent(ctx, saved, snap) {
		result.Unmet = append(result.Unmet, "report-set-stale")
	} else {
		result.ReportSetDigest = saved.ReportSet.Digest
		for _, item := range saved.ReportSet.Reports {
			result.Reports = append(result.Reports, item.Path)
		}
		if saved.Review != saved.ReportSet.Digest {
			result.Unmet = append(result.Unmet, "review-required")
			reviewRequired = true
		}
	}
	if saved.Terminal == nil || saved.ReportSet == nil || saved.Terminal.CheckExit != 0 || saved.Terminal.ReportSet != saved.ReportSet.Digest || !repo.terminalCurrentContext(ctx, saved.Terminal) {
		result.Unmet = append(result.Unmet, "final-check-required")
	}
	result.Satisfied = saved.Lifecycle == "satisfied" && len(result.Unmet) == 0
	var end snapshot
	if pinned == nil {
		end, err = repo.driftSnapshot(ctx)
	} else {
		end = snap
		// A moved HEAD with an identical tree stays clean, so re-resolve the commit.
		if err = repo.refreshAuthority(); err == nil {
			end.clean, err = repo.clean(ctx)
		}
		if err == nil {
			end.target, err = repo.auth.Resolve(ctx, "HEAD")
		}
	}
	if err != nil {
		return Evaluation{}, err
	}
	if end.target != snap.target || end.tree != snap.tree || end.clean != snap.clean {
		return Evaluation{}, errors.New("repository-snapshot-drift")
	}
	if !result.Satisfied {
		switch {
		case !repositoryReady || attemptsExhausted:
			// Preserve the handoff handle while mutations cannot advance this enrollment.
			result.NextActions = [][]string{{"corvint", "dogfood", "status", "--session-key", repo.session}}
		case len(verificationActions) > 0:
			result.NextActions = verificationActions
		case reviewRequired:
			result.NextActions = [][]string{{"corvint", "dogfood", "review", "--session-key", repo.session, "--report-set", saved.ReportSet.Digest}}
		default:
			result.NextActions = [][]string{{"corvint", "dogfood", "finish", "--session-key", repo.session}}
			if saved.AggregateOutcome != nil {
				result.NextActions[0] = append(result.NextActions[0], "--aggregate-outcome-profile", saved.AggregateOutcome.Profile)
			}
		}
	} else {
		result.NoActionReason = "workflow-satisfied"
	}
	if observed != nil {
		*observed = Observed{target: snap.target, tree: snap.tree, content: snap.content,
			root: repo.auth.Root, gitDir: repo.auth.GitDir, commonDir: repo.auth.CommonDir}
	}
	return result, nil
}

func (repo *repository) latest(saved *state, checkID string) *observation {
	for index := len(saved.Observations) - 1; index >= 0; index-- {
		if saved.Observations[index].CheckID == checkID {
			return &saved.Observations[index]
		}
	}
	return nil
}

func (repo *repository) checkQualification(ctx context.Context, saved *state, check Check, snap snapshot) (bool, string) {
	observed := repo.latest(saved, check.ID)
	if observed == nil || observed.CheckDigest != repo.executionDigest(saved, check) || !observed.Passed || !observed.Clean || observed.Exit != "0" || observed.TimedOut || observed.Cancelled || observed.Overflow || observed.SecretScreened {
		return false, ""
	}
	if !repo.artifactsCurrent([]artifact{observed.Stdout, observed.Stderr}) {
		return false, ""
	}
	if observed.Target == snap.target {
		return observed.Tree == snap.tree, ""
	}
	if check.AllowSealArchiveReuse && repo.sealArchiveEquivalent(ctx, saved, observed, snap) {
		return true, ""
	}
	if !check.AllowCemSidecarOnlyReuse {
		return false, ""
	}
	// Another target reruns (CEM-V1-150, CEM-V1-208) unless the plan allows
	// sidecar-only reuse and the stable verifier accepts the sidecar (CEM-V1-205).
	reason, _ := repo.stableSidecarReuse(ctx, saved, observed, snap)
	return reason == "", reason
}

func (repo *repository) artifactsCurrent(items []artifact) bool {
	if len(items) > 96 {
		return false
	}
	for _, item := range items {
		allowed := false
		for _, base := range []string{repo.auth.Root, repo.auth.GitDir} {
			relative, err := filepath.Rel(base, item.Path)
			if err == nil && validPath(filepath.ToSlash(relative)) {
				allowed = true
			}
		}
		if !allowed {
			return false
		}
	}
	return artifactsCurrent(items)
}

func (repo *repository) reportCurrent(ctx context.Context, saved *state, snap snapshot) bool {
	set := saved.ReportSet
	if saved.Sealed != nil && repo.validateSealed(ctx, saved, snap) != nil {
		return false
	}
	if set == nil || set.Target != snap.target || set.PlanDigest != saved.PlanDigest || len(set.Bindings) != 3+2*len(saved.Plan.Intents) || len(set.Reports) != 1+len(saved.Plan.Intents) || len(set.Observations) != len(saved.Plan.Checks) {
		return false
	}
	for index, name := range repo.bindingPaths(saved) {
		if set.Bindings[index].Path != name {
			return false
		}
	}
	for index, item := range set.Reports {
		if item.Path != repo.local(fmt.Sprintf("report-%s-%02d.md", snap.target, index)) {
			return false
		}
	}
	copy := *set
	copy.Digest = ""
	if valueDigest(copy) != set.Digest || !repo.artifactsCurrent(set.Bindings) || !repo.artifactsCurrent(set.Reports) {
		return false
	}
	for index, check := range saved.Plan.Checks {
		latest := repo.latest(saved, check.ID)
		if latest == nil || valueDigest(*latest) != valueDigest(set.Observations[index]) {
			return false
		}
	}
	return true
}

func Verify(ctx context.Context, root, key, checkID string) (Evaluation, error) {
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
	var selected *Check
	for index := range saved.Plan.Checks {
		if saved.Plan.Checks[index].ID == checkID {
			selected = &saved.Plan.Checks[index]
		}
	}
	if selected == nil {
		return Evaluation{}, errors.New("unknown-selected-check")
	}
	if len(saved.Observations) >= maxAttempts {
		return Evaluation{}, errors.New("verification-attempt-bound-exceeded")
	}
	before, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	if !before.clean {
		return Evaluation{}, errors.New("uncommitted-work")
	}
	result := procgroup.Run(ctx, procgroup.Spec{Argv: repo.executionArgv(saved, *selected), Dir: repo.auth.Root, Env: os.Environ(), Timeout: time.Duration(selected.TimeoutSeconds) * time.Second, OutputLimit: maxLogBytes, StderrLimit: maxLogBytes})
	observed := observation{CheckID: checkID, CheckDigest: repo.executionDigest(saved, *selected), Target: before.target, Tree: before.tree, ContentDigest: before.content, Clean: true, Exit: strconv.Itoa(result.ExitStatus), TimedOut: result.TimedOut, Cancelled: result.Cancelled, Overflow: result.OutputOverflow}
	after, afterErr := repo.driftSnapshot(ctx)
	observed.Clean = afterErr == nil && after.clean && before.target == after.target && before.tree == after.tree
	observed.Passed = observed.Clean && result.Err == nil && result.ExitObserved && result.ExitStatus == 0 && result.WaitCompleted && result.PipesDrained && result.OwnedProcessGroupCleanup && !result.OutputOverflow
	if secretscreen.MatchString(string(result.Stdout)) || secretscreen.MatchString(string(result.Stderr)) {
		observed.SecretScreened = true
		observed.Passed = false
		result.Stdout = screenedLog(result.Stdout)
		result.Stderr = screenedLog(result.Stderr)
	}
	for index, data := range [][]byte{result.Stdout, result.Stderr} {
		name := repo.local(fmt.Sprintf("check-%03d-%d.log", len(saved.Observations)+1, index))
		if err = writeFile(name, data); err != nil {
			return Evaluation{}, err
		}
		item := artifact{Path: name, Digest: digest(data)}
		if index == 0 {
			observed.Stdout = item
		} else {
			observed.Stderr = item
		}
	}
	if err = repo.requireOwner(); err != nil {
		return Evaluation{}, err
	}
	saved.Lifecycle = "active"
	saved.Terminal = nil
	saved.Observations = append(saved.Observations, observed)
	if err = repo.save(saved); err != nil {
		return Evaluation{}, err
	}
	if ctx.Err() != nil {
		return Evaluation{}, errors.New("verification-cancelled")
	}
	evaluation, err := repo.evaluate(ctx, saved)
	if err == nil && !observed.Passed {
		failed := failedTestNames(result.Stdout, result.Stderr)
		for index := range evaluation.Checks {
			if evaluation.Checks[index].ID == checkID {
				evaluation.Checks[index].FailedTests = failed
			}
		}
	}
	return evaluation, err
}

func Review(ctx context.Context, root, key, reportDigest string) (Evaluation, error) {
	if !keyPattern.MatchString(reportDigest) {
		return Evaluation{}, errors.New("invalid-report-set-digest")
	}
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
	snap, err := repo.snapshot(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	if !snap.clean || !repo.reportCurrent(ctx, saved, snap) || saved.ReportSet.Digest != reportDigest {
		return Evaluation{}, errors.New("report-set-stale")
	}
	for _, check := range saved.Plan.Checks {
		if !repo.checkQualifies(ctx, saved, check, snap) {
			return Evaluation{}, errors.New("selected-check-unverified")
		}
	}
	saved.Review = reportDigest
	if err = repo.save(saved); err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, saved)
}

// Cancel records an explicit non-success. An active, unsatisfied enrollment is
// closed only with a nonempty reason (LCP-V0-038): moving to a new base takes a
// fresh worktree and enrollment instead, so a bare cancel there is refused with
// the check and observation counts it would close.
func Cancel(ctx context.Context, root, key, reason string) (Evaluation, error) {
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
	if saved.Lifecycle == "active" && reason == "" {
		return Evaluation{}, &CancelReasonError{Checks: len(saved.Plan.Checks), Observations: len(saved.Observations)}
	}
	// A coordinator's preservation hold keeps the ACTIVE state; only its
	// release, which records the authorization, admits the cancel (LCP-V0-880).
	if held, holdErr := repo.readHold(); holdErr != nil {
		return Evaluation{}, holdErr
	} else if held != nil && saved.Lifecycle == "active" {
		return Evaluation{}, &HoldError{Reason: held.Reason, Session: key}
	}
	saved.Lifecycle = "cancelled"
	saved.Terminal = nil
	if err = repo.save(saved); err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, saved)
}

func displayChecks(plan Plan) string {
	all := make([][]string, 0, len(plan.Checks))
	for _, check := range plan.Checks {
		all = append(all, check.Argv)
	}
	return "local-completion-checks-sha256:" + valueDigest(all)
}

func (repo *repository) archiveGeneration(saved *state) error {
	count, err := repo.generationCount()
	if err != nil {
		return err
	}
	if count >= 16 {
		return errors.New("enrollment-generation-bound-exceeded")
	}
	raw, err := readFile(repo.local("state.json"), maxStateBytes)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(repo.directory, repo.session, saved.Generation, "state.json"), raw)
}

func (repo *repository) nextGeneration(planDigest string) (string, error) {
	count, err := repo.generationCount()
	if err != nil {
		return "", err
	}
	if count >= 16 {
		return "", errors.New("enrollment-generation-bound-exceeded")
	}
	return fmt.Sprintf("%s-%03d", planDigest, count+1), nil
}

func (repo *repository) generationCount() (int, error) {
	directory := filepath.Join(repo.directory, repo.session)
	if err := checkParents(directory); err != nil {
		return 0, err
	}
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, errors.New("invalid-local-state-directory")
	}
	file, err := os.Open(directory)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	// Sixteen generations and the current state locator are the complete
	// bounded directory. Read one extra entry to detect overflow without
	// allocating for arbitrary user-inserted directory contents.
	entries, err := file.ReadDir(18)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	if len(entries) > 17 {
		return 0, errors.New("enrollment-generation-bound-exceeded")
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count, nil
}

func (repo *repository) executionArgv(saved *state, check Check) []string {
	argv := append([]string(nil), check.Argv...)
	for index, candidate := range saved.Plan.Checks {
		if candidate.ID == check.ID {
			argv[0] = saved.Executables[index]
		}
	}
	return argv
}
func (repo *repository) executionDigest(saved *state, check Check) string {
	return valueDigest(struct {
		Check Check
		Argv  []string
	}{check, repo.executionArgv(saved, check)})
}

// inactive evaluates a key with no enrollment. It names another session's
// active enrollment of the worktree, so a Stop that releases here is not
// silent about a gate this key cannot see.
func (repo *repository) inactive() (Evaluation, error) {
	result := Evaluation{Lifecycle: "inactive", NoActionReason: "enrollment-inactive", Unmet: []string{}, Intents: []string{}, IntentPointers: []IntentPointer{}}
	owner, err := repo.owner()
	if err != nil || owner == "" {
		return result, err
	}
	other := *repo
	other.session = owner
	prior, err := other.load()
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return Evaluation{}, err
	}
	if prior.Lifecycle == "active" {
		result.Owner = owner
	}
	return result, nil
}
