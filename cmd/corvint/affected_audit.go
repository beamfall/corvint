package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// The `affected audit` wire and bounds (AFP-V0-445, AFP-V0-447).
const (
	affectedAuditInputProfile = "corvint-affected-audit-input/0"
	affectedAuditProfile      = "corvint-affected-audit/1"
	affectedAuditMaxManifest  = 1 << 20
	affectedAuditMaxChanges   = 256
	affectedAuditMaxOutcome   = 32 << 20
	affectedAuditMaxTests     = 100000
	affectedAuditMaxDepth     = 64
	affectedAuditConfidence   = 0.95
	// affectedAuditMaxDiff bounds one mutant patch (AFP-V0-447).
	affectedAuditMaxDiff = 8 << 20
)

// The audited selectors, in report order (AFP-V0-447).
const (
	auditSelectorAffected = "affected"
	auditSelectorAppmap   = "appmap-impact"
	auditSelectorE2E      = "e2e-compile"
)

var affectedAuditSelectorNames = []string{auditSelectorAffected, auditSelectorAppmap, auditSelectorE2E}

// Per-change, per-selector verdicts and scopes (AFP-V0-447).
const (
	auditVerdictSafe    = "SAFE"
	auditVerdictMiss    = "MISS"
	auditVerdictUnknown = "UNKNOWN"
	auditVerdictNotRun  = "NOT_RUN"

	auditScopeBounded  = "BOUNDED"
	auditScopeFallback = "FALLBACK"
	auditScopeError    = "ERROR"
	auditScopeNotRun   = "NOT_RUN"

	auditUnknown   = "UNKNOWN"
	auditUndefined = "UNDEFINED"
)

type affectedAuditInput struct {
	Profile string               `json:"profile"`
	Changes []affectedAuditInChg `json:"changes"`
}

type affectedAuditInChg struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind,omitzero"`
	Base       string                 `json:"base"`
	Head       string                 `json:"head,omitzero"`
	Revert     string                 `json:"revert,omitzero"`
	Diff       string                 `json:"diff,omitzero"`
	Format     string                 `json:"format,omitzero"`
	Baseline   string                 `json:"baseline,omitzero"`
	Changed    string                 `json:"changed,omitzero"`
	PathPrefix string                 `json:"path_prefix,omitzero"`
	Appmap     *affectedAuditInAppmap `json:"appmap,omitzero"`
	E2E        *affectedAuditInE2E    `json:"e2e,omitzero"`
}

type affectedAuditInAppmap struct {
	Map    string `json:"map"`
	Ledger string `json:"ledger,omitzero"`
}

// affectedAuditInE2E names the e2e manifest and, for a second (tests) repository, the
// aliased roots e2e-compile reads, as its --manifest-repo, --repo and --repo-revision flags
// (AFP-V0-490).
type affectedAuditInE2E struct {
	Manifest      string            `json:"manifest"`
	ManifestRepo  string            `json:"manifest_repo,omitzero"`
	Repos         map[string]string `json:"repos,omitzero"`
	RepoRevisions map[string]string `json:"repo_revisions,omitzero"`

	rootArgs []string // the validated flags, in alias order
}

// affectedAuditReport is the corvint-affected-audit/1 document.
type affectedAuditReport struct {
	Aggregate []affectedAuditAggregate `json:"aggregate"`
	Changes   []affectedAuditChange    `json:"changes"`
	Mutates   bool                     `json:"mutates"`
	OK        bool                     `json:"ok"`
	Profile   string                   `json:"profile"`
	Selectors []affectedAuditSel       `json:"selectors"`
	Tool      string                   `json:"tool"`
}

type affectedAuditSel struct {
	Name   string `json:"name"`
	Reason string `json:"reason,omitzero"`
	State  string `json:"state"`
}

type affectedAuditChange struct {
	Base        string                   `json:"base"`
	Head        string                   `json:"head"`
	ID          string                   `json:"id"`
	Kind        string                   `json:"kind"`
	Observation affectedAuditObservation `json:"observation"`
	Selectors   []affectedAuditSelection `json:"selectors"`
}

// affectedAuditObservation is what the outcome files observed for one change.
// State is OBSERVED, or UNKNOWN when the change lists no outcomes.
type affectedAuditObservation struct {
	MustRerun     []affectedAuditID `json:"must_rerun"`
	ObservedTests int               `json:"observed_tests"`
	Reason        string            `json:"reason,omitzero"`
	Removed       []affectedAuditID `json:"removed"`
	State         string            `json:"state"`
	Unobserved    []affectedAuditID `json:"unobserved"`
}

type affectedAuditSelection struct {
	ElapsedMS            int64                 `json:"elapsed_ms"`
	Error                *affectedAuditFailure `json:"error,omitzero"`
	Fallback             bool                  `json:"fallback"`
	Misses               []affectedAuditID     `json:"misses"`
	Name                 string                `json:"name"`
	Precision            string                `json:"precision"`
	Reason               string                `json:"reason,omitzero"`
	Recall               string                `json:"recall"`
	Scope                string                `json:"scope"`
	SelectedTests        int                   `json:"selected_tests"`
	SelectionSize        int                   `json:"selection_size"`
	SelectorState        string                `json:"selector_state,omitzero"`
	Roots                []appmap.RootPin      `json:"roots,omitzero"`
	TruePositives        int                   `json:"true_positives"`
	UnknownReasons       []string              `json:"unknown_reasons"`
	UnobservedUnselected []affectedAuditID     `json:"unobserved_unselected"`
	Verdict              string                `json:"verdict"`
}

type affectedAuditFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type affectedAuditID struct {
	Classname string `json:"classname,omitzero"`
	File      string `json:"file,omitzero"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

type affectedAuditAggregate struct {
	Changes            int    `json:"changes"`
	ChangesWithMiss    int    `json:"changes_with_miss"`
	Confidence         string `json:"confidence"`
	ElapsedMS          int64  `json:"elapsed_ms"`
	FallbackChanges    int    `json:"fallback_changes"`
	MissRateUpperBound string `json:"miss_rate_upper_bound"`
	MissedTests        int    `json:"missed_tests"`
	MustRerunTests     int    `json:"must_rerun_tests"`
	Name               string `json:"name"`
	ObservedChanges    int    `json:"observed_changes"`
	Precision          string `json:"precision"`
	Recall             string `json:"recall"`
	RuleOfThree        string `json:"rule_of_three,omitzero"`
	SelectedTests      int    `json:"selected_tests"`
	TruePositives      int    `json:"true_positives"`
	UnknownChanges     int    `json:"unknown_changes"`
	UpperBoundMethod   string `json:"upper_bound_method"`
}

// parseAffectedAuditOptions reads `affected audit --changes FILE`, exactly once (AFP-V0-445).
func parseAffectedAuditOptions(rest []string) (affectedInvocation, error) {
	invocation := affectedInvocation{NoCache: true}
	for index := 0; index < len(rest); index++ {
		name, value, inline := strings.Cut(rest[index], "=")
		if name != "--changes" {
			return affectedInvocation{}, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) {
				return affectedInvocation{}, argumentError("--changes requires exactly one value")
			}
			index++
			value = rest[index]
		}
		if invocation.AuditChanges != "" || value == "" {
			return affectedInvocation{}, argumentError("--changes requires exactly one nonempty value")
		}
		invocation.AuditChanges = value
	}
	if invocation.AuditChanges == "" {
		return affectedInvocation{}, argumentError("affected audit requires --changes FILE")
	}
	return invocation, nil
}

// runAffectedAudit scores every configured selector over each listed change
// against the observed outcomes (AFP-V0-445, AFP-V0-447). It exits 1 when any
// selector missed a test, 3 when none missed but some selector's result on some
// change is UNKNOWN (no outcomes, an unobserved unselected test, or a selector
// error), 2 on an input error, and 0 otherwise.
func runAffectedAudit(ctx context.Context, invocation affectedInvocation, stdout, stderr io.Writer) int {
	report, err := compileAffectedAudit(ctx, invocation)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	code := affectedAuditExitCode(report)
	report.OK = code == 0
	if err := gokernel.WriteCanonicalJSON(stdout, report); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected audit"})
		return 2
	}
	return code
}

func affectedAuditExitCode(report affectedAuditReport) int {
	unknown := false
	for _, aggregate := range report.Aggregate {
		if aggregate.ChangesWithMiss != 0 {
			return 1
		}
		unknown = unknown || aggregate.UnknownChanges != 0
	}
	if unknown {
		return 3
	}
	return 0
}

func affectedAuditError(format string, arguments ...any) error {
	return &gokernel.Error{Code: "unsupported-affected-audit", Message: fmt.Sprintf(format, arguments...)}
}

// validateAffectedAuditChange checks one manifest entry and fills its kind.
func validateAffectedAuditChange(change *affectedAuditInChg) error {
	if change.Kind == "" {
		change.Kind = "replay"
	}
	materialized := false
	switch change.Kind {
	case "replay":
		materialized = validGitObjectID(change.Head) && change.Revert == "" && change.Diff == ""
	case "revert":
		materialized = validGitObjectID(change.Revert) && change.Head == "" && change.Diff == ""
	case "mutant":
		materialized = change.Diff != "" && change.Head == "" && change.Revert == ""
	}
	outcomes := (change.Baseline == "" && change.Changed == "" && change.Format == "") ||
		(change.Baseline != "" && change.Changed != "" && (change.Format == "junit" || change.Format == "playwright-json"))
	if change.ID == "" || !validGitObjectID(change.Base) || !materialized || !outcomes ||
		(change.PathPrefix != "" && !affected.ValidRelativePath(change.PathPrefix)) ||
		(change.Appmap != nil && change.Appmap.Map == "") ||
		(change.E2E != nil && !affected.ValidRelativePath(change.E2E.Manifest)) {
		return affectedAuditError("--changes: change %q needs a unique id, a full base commit id, kind replay with a full head commit id, revert with the full commit id to revert on base, or mutant with a diff file applied on base; outcomes as format junit or playwright-json with baseline and changed files, or none (observation UNKNOWN); an optional repository-relative path_prefix; an optional appmap {map, ledger}; and an optional e2e {manifest, manifest_repo, repos, repo_revisions} with a repository-relative manifest", change.ID)
	}
	if change.E2E != nil {
		arguments, err := affectedAuditE2ERootArgs(*change.E2E)
		if err != nil {
			return affectedAuditError("--changes: change %q e2e: %v", change.ID, err)
		}
		change.E2E.rootArgs = arguments
	}
	return nil
}

// affectedAuditE2ERootArgs turns a change's e2e two-repository inputs into the e2e-compile
// --repo, --manifest-repo and --repo-revision flags, parsed by the same flag binding the
// companion uses, and refuses what that run would otherwise turn into an UNKNOWN: a root that
// is not absolute and clean, a revision that is not a full commit id, and a manifest_repo or
// repo_revisions alias without a repos entry (AFP-V0-490, AFP-V0-491).
func affectedAuditE2ERootArgs(e2e affectedAuditInE2E) ([]string, error) {
	var arguments []string
	for _, alias := range slices.Sorted(maps.Keys(e2e.Repos)) {
		if !rootalias.Valid(alias) {
			// A key holding "=" would re-split into another alias and root once joined.
			return nil, fmt.Errorf("repos alias %q is invalid", alias)
		}
		if !rootalias.ValidRoot(e2e.Repos[alias]) {
			return nil, fmt.Errorf("repos %q must be an absolute, clean path", alias)
		}
		arguments = append(arguments, "--repo", alias+"="+e2e.Repos[alias])
	}
	if e2e.ManifestRepo != "" {
		if e2e.Repos[e2e.ManifestRepo] == "" {
			return nil, fmt.Errorf("manifest_repo %q names no repos alias", e2e.ManifestRepo)
		}
		arguments = append(arguments, "--manifest-repo", e2e.ManifestRepo)
	}
	for _, alias := range slices.Sorted(maps.Keys(e2e.RepoRevisions)) {
		if !rootalias.Valid(alias) {
			return nil, fmt.Errorf("repo_revisions alias %q is invalid", alias)
		}
		if !validGitObjectID(e2e.RepoRevisions[alias]) {
			return nil, fmt.Errorf("repo_revisions %q must be a full lowercase commit id", alias)
		}
		arguments = append(arguments, "--repo-revision", alias+"="+e2e.RepoRevisions[alias])
	}
	flags := flag.NewFlagSet("affected audit e2e", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	roots := appmap.BindRootFlags(flags)
	if err := flags.Parse(arguments); err != nil {
		return nil, err
	}
	if err := roots.Check(); err != nil {
		return nil, err
	}
	return arguments, nil
}

func compileAffectedAudit(ctx context.Context, invocation affectedInvocation) (affectedAuditReport, error) {
	manifestPath := invocation.AuditChanges
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(invocation.Root, manifestPath)
	}
	body, err := readAffectedAuditFile(manifestPath, affectedAuditMaxManifest)
	if err != nil {
		return affectedAuditReport{}, affectedAuditError("--changes: %v", err)
	}
	var input affectedAuditInput
	if err := json.Unmarshal(body, &input, json.RejectUnknownMembers(true)); err != nil {
		return affectedAuditReport{}, affectedAuditError("--changes: %v", err)
	}
	if input.Profile != affectedAuditInputProfile || len(input.Changes) == 0 || len(input.Changes) > affectedAuditMaxChanges {
		return affectedAuditReport{}, affectedAuditError("--changes: profile must be %s with 1 to %d changes", affectedAuditInputProfile, affectedAuditMaxChanges)
	}
	seen := map[string]bool{}
	configured := map[string]bool{auditSelectorAffected: true}
	for index := range input.Changes {
		change := &input.Changes[index]
		if seen[change.ID] {
			return affectedAuditReport{}, affectedAuditError("--changes: change id %q is not unique", change.ID)
		}
		if err := validateAffectedAuditChange(change); err != nil {
			return affectedAuditReport{}, err
		}
		if change.E2E != nil {
			// The compile runs on a clone, so the companion's own --root collision check cannot
			// see the audited repository; refuse it here, through symlinks too (AFP-V0-491).
			audited, err := os.Stat(invocation.Root)
			for _, alias := range slices.Sorted(maps.Keys(change.E2E.Repos)) {
				if info, statErr := os.Stat(change.E2E.Repos[alias]); err == nil && statErr == nil && os.SameFile(audited, info) {
					return affectedAuditReport{}, affectedAuditError("--changes: change %q e2e: repos %q is the audited repository", change.ID, alias)
				}
			}
		}
		seen[change.ID] = true
		configured[auditSelectorAppmap] = configured[auditSelectorAppmap] || change.Appmap != nil
		configured[auditSelectorE2E] = configured[auditSelectorE2E] || change.E2E != nil
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return affectedAuditReport{}, affectedGitExecutableRefusal()
	}
	report := affectedAuditReport{Changes: []affectedAuditChange{}, Profile: affectedAuditProfile, Tool: "affected"}
	for _, name := range affectedAuditSelectorNames {
		selector := affectedAuditSel{Name: name, State: "RUN"}
		if !configured[name] {
			selector.State, selector.Reason = "NOT_RUN", "no change configures it"
		}
		report.Selectors = append(report.Selectors, selector)
	}
	directory := filepath.Dir(manifestPath)
	for _, change := range input.Changes {
		audited, err := auditAffectedChange(ctx, gitExecutable, invocation.Root, directory, change)
		if err != nil {
			return affectedAuditReport{}, err
		}
		report.Changes = append(report.Changes, audited)
	}
	report.Aggregate = aggregateAffectedAudit(report.Changes)
	return report, nil
}

type affectedAuditOutcome struct {
	id  affectedAuditID
	key string
}

// affectedAuditPick is one selector's selection for one change: everything on
// a fallback, else the named test files, units and test keys.
type affectedAuditPick struct {
	fallback bool
	files    map[string]bool
	units    map[string]bool
	keys     map[string]bool
	size     int
	state    string
	unknown  []string
	roots    []appmap.RootPin // the aliased roots e2e-compile read, with their revisions (AFP-V0-492)
}

func (pick affectedAuditPick) selects(id affectedAuditID) bool {
	if pick.fallback || (id.File != "" && pick.files[id.File]) {
		return true
	}
	if id.Classname != "" && (pick.units[id.Classname] || pick.units["go:"+id.Classname]) {
		return true
	}
	if id.File != "" && len(pick.keys) != 0 {
		name := id.Name
		if strings.HasPrefix(name, "[") {
			if _, rest, ok := strings.Cut(name, "] "); ok {
				name = rest
			}
		}
		return pick.keys[id.File+" > "+name]
	}
	return false
}

func auditAffectedChange(ctx context.Context, gitExecutable, root, directory string, change affectedAuditInChg) (affectedAuditChange, error) {
	var baseline, changed []affectedAuditOutcome
	if change.Baseline != "" {
		var err error
		if baseline, err = readAffectedAuditOutcomes(directory, change.Baseline, change.Format, change.PathPrefix); err != nil {
			return affectedAuditChange{}, affectedAuditError("change %q baseline: %v", change.ID, err)
		}
		if changed, err = readAffectedAuditOutcomes(directory, change.Changed, change.Format, change.PathPrefix); err != nil {
			return affectedAuditChange{}, affectedAuditError("change %q changed: %v", change.ID, err)
		}
	}
	var patch []byte
	if change.Diff != "" {
		name := change.Diff
		if !filepath.IsAbs(name) {
			name = filepath.Join(directory, name)
		}
		var err error
		if patch, err = readAffectedAuditFile(name, affectedAuditMaxDiff); err != nil {
			return affectedAuditChange{}, affectedAuditError("change %q diff: %v", change.ID, err)
		}
	}
	temporary, err := os.MkdirTemp("", "corvint-affected-audit-")
	if err != nil {
		return affectedAuditChange{}, affectedAuditError("cannot create a scratch clone: %v", err)
	}
	defer os.RemoveAll(temporary)
	clone, head, err := materializeAffectedAuditChange(ctx, gitExecutable, root, temporary, change, patch)
	if err != nil {
		return affectedAuditChange{}, err
	}
	audited := affectedAuditChange{Base: change.Base, Head: head, ID: change.ID, Kind: change.Kind, Selectors: []affectedAuditSelection{}}
	if change.Baseline == "" {
		audited.Observation = affectedAuditObservation{State: auditUnknown, Reason: "the change lists no outcome files",
			MustRerun: []affectedAuditID{}, Removed: []affectedAuditID{}, Unobserved: []affectedAuditID{}}
	} else {
		atBase, err := affectedAuditTrackedFiles(ctx, gitExecutable, clone, change.Base)
		if err != nil {
			return affectedAuditChange{}, affectedAuditError("change %q: %v", change.ID, err)
		}
		atHead, err := affectedAuditTrackedFiles(ctx, gitExecutable, clone, head)
		if err != nil {
			return affectedAuditChange{}, affectedAuditError("change %q: %v", change.ID, err)
		}
		if change.E2E != nil && len(change.E2E.Repos) > 0 {
			// A test may live in a second repository read at one revision for base and head, so
			// the application's tracked files are no evidence it was removed (AFP-V0-493).
			atBase, atHead = nil, nil
		}
		audited.Observation = observeAffectedAuditChange(baseline, changed, atBase, atHead)
	}
	for _, name := range affectedAuditSelectorNames {
		started := time.Now()
		pick, reason, err := pickAffectedAuditSelector(ctx, name, clone, directory, change, head)
		elapsed := time.Since(started).Milliseconds()
		audited.Selectors = append(audited.Selectors, scoreAffectedAuditSelector(name, pick, reason, err, elapsed, audited.Observation, changed))
	}
	return audited, nil
}

// materializeAffectedAuditChange builds the change's head in a private shared
// clone, so the audited repository's worktree, index, refs and objects are
// never touched: a replay checks out head; a revert or a mutant commits the
// reverted or patched tree on base with a fixed identity and date.
func materializeAffectedAuditChange(ctx context.Context, gitExecutable, root, temporary string, change affectedAuditInChg, patch []byte) (string, string, error) {
	if resolved, err := filepath.EvalSymlinks(temporary); err == nil {
		temporary = resolved
	}
	clone := filepath.Join(temporary, "repository")
	fail := func(what string, output []byte) (string, string, error) {
		return "", "", affectedAuditError("change %q: %s: %s", change.ID, what, strings.TrimSpace(string(output)))
	}
	cloning := hermeticGitCommand(ctx, gitExecutable, temporary, "clone", "--quiet", "--shared", "--no-checkout", "--", root, clone)
	// The one transport a local clone needs; every other stays refused.
	cloning.Env = append(cloning.Env, "GIT_ALLOW_PROTOCOL=file")
	if output, err := cloning.CombinedOutput(); err != nil {
		return fail("cannot clone the repository", output)
	}
	checkout := change.Head
	if change.Kind != "replay" {
		checkout = change.Base
	}
	if output, err := hermeticGitCommand(ctx, gitExecutable, clone, "checkout", "--quiet", "--detach", checkout).CombinedOutput(); err != nil {
		return fail("cannot check out "+checkout, output)
	}
	if change.Kind != "replay" {
		identity := []string{"-c", "user.name=corvint affected audit", "-c", "user.email=affected-audit@corvint.invalid", "-c", "commit.gpgsign=false"}
		var command *exec.Cmd
		if change.Kind == "revert" {
			parents, err := hermeticGitCommand(ctx, gitExecutable, clone, "rev-list", "--parents", "-n", "1", change.Revert).Output()
			if err != nil {
				return fail("cannot read the commit to revert "+change.Revert, parents)
			}
			arguments := append(identity, "revert", "--no-edit")
			if len(strings.Fields(string(parents))) > 2 {
				arguments = append(arguments, "-m", "1")
			}
			command = hermeticGitCommand(ctx, gitExecutable, clone, append(arguments, change.Revert)...)
		} else {
			applying := hermeticGitCommand(ctx, gitExecutable, clone, "apply", "--index", "-")
			applying.Stdin = bytes.NewReader(patch)
			if output, err := applying.CombinedOutput(); err != nil {
				return fail("cannot apply the diff on base", output)
			}
			command = hermeticGitCommand(ctx, gitExecutable, clone, append(identity, "commit", "--quiet", "--no-verify", "-m", "corvint affected audit mutant "+change.ID)...)
		}
		command.Env = append(command.Env, "GIT_AUTHOR_DATE=@0 +0000", "GIT_COMMITTER_DATE=@0 +0000")
		if output, err := command.CombinedOutput(); err != nil {
			return fail("cannot commit the "+change.Kind, output)
		}
	}
	head, err := hermeticGitCommand(ctx, gitExecutable, clone, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil {
		return fail("cannot resolve the change head", head)
	}
	return clone, strings.TrimSpace(string(head)), nil
}

// affectedAuditTrackedFiles lists the files of one commit's tree.
func affectedAuditTrackedFiles(ctx context.Context, gitExecutable, clone, commit string) (map[string]bool, error) {
	output, err := hermeticGitCommand(ctx, gitExecutable, clone, "ls-tree", "-r", "-z", "--name-only", commit).Output()
	if err != nil {
		return nil, fmt.Errorf("cannot list the tree of %s", commit)
	}
	files := map[string]bool{}
	for _, name := range strings.Split(string(output), "\x00") {
		if name != "" {
			files[name] = true
		}
	}
	return files, nil
}

// observeAffectedAuditChange derives what had to run: a changed-run test whose
// status differs from the baseline, or that the baseline lacks. A baseline test
// the changed run lacks is removed when its file is tracked at base and gone at
// head, and otherwise unobserved: its outcome is missing, never assumed unchanged.
func observeAffectedAuditChange(baseline, changed []affectedAuditOutcome, atBase, atHead map[string]bool) affectedAuditObservation {
	observation := affectedAuditObservation{State: "OBSERVED", ObservedTests: len(changed),
		MustRerun: []affectedAuditID{}, Removed: []affectedAuditID{}, Unobserved: []affectedAuditID{}}
	before, after := map[string]string{}, map[string]bool{}
	for _, outcome := range baseline {
		before[outcome.key] = outcome.id.Status
	}
	for _, outcome := range changed {
		after[outcome.key] = true
		if status, ok := before[outcome.key]; !ok || status != outcome.id.Status {
			observation.MustRerun = append(observation.MustRerun, outcome.id)
		}
	}
	for _, outcome := range baseline {
		if after[outcome.key] {
			continue
		}
		if outcome.id.File != "" && atBase[outcome.id.File] && !atHead[outcome.id.File] {
			observation.Removed = append(observation.Removed, outcome.id)
		} else {
			observation.Unobserved = append(observation.Unobserved, outcome.id)
		}
	}
	return observation
}

// pickAffectedAuditSelector runs one selector at the change head. A selector
// the change does not configure returns a NOT_RUN reason.
func pickAffectedAuditSelector(ctx context.Context, name, clone, directory string, change affectedAuditInChg, head string) (affectedAuditPick, string, error) {
	local := func(name string) string {
		if filepath.IsAbs(name) {
			return name
		}
		return filepath.Join(directory, name)
	}
	switch name {
	case auditSelectorAffected:
		receipt, err := compileAffected(ctx, affectedInvocation{Root: clone, Base: change.Base, Full: true, NoCache: true})
		if err != nil {
			return affectedAuditPick{}, "", err
		}
		pick := affectedAuditPick{fallback: receipt.Plan.Scope != "BOUNDED", files: map[string]bool{}, units: map[string]bool{},
			size: len(receipt.Plan.Selected), state: receipt.Plan.Scope}
		for _, selection := range receipt.Plan.Selected {
			pick.units[selection.UnitID] = true
			for _, test := range selection.Tests {
				pick.files[test] = true
			}
		}
		for _, unknown := range receipt.Plan.Unknown {
			pick.unknown = append(pick.unknown, unknown.Reason)
		}
		return pick, "", nil
	case auditSelectorAppmap:
		if change.Appmap == nil {
			return affectedAuditPick{}, "the change configures no appmap map", nil
		}
		applicationMap, err := appmap.LoadMap(local(change.Appmap.Map))
		if err != nil {
			return affectedAuditPick{}, "", err
		}
		var ledger *appmap.EdgeLedger
		if change.Appmap.Ledger != "" {
			if ledger, err = appmap.LoadEdgeLedger(local(change.Appmap.Ledger)); err != nil {
				return affectedAuditPick{}, "", err
			}
		}
		raw, err := appmap.Impact(ctx, applicationMap, ledger, change.Base, appmap.Options{Root: clone, Revision: head, Full: true})
		if err != nil {
			return affectedAuditPick{}, "", err
		}
		var impact appmap.ImpactReport
		if err := json.Unmarshal(raw, &impact); err != nil {
			return affectedAuditPick{}, "", affectedAuditError("change %q: unreadable appmap impact report", change.ID)
		}
		for kind, omitted := range impact.Omitted {
			if omitted != 0 {
				// A trimmed list is not the selection; it is never scored as one.
				return affectedAuditPick{}, "", affectedAuditError("change %q: the appmap impact report omitted %d %s", change.ID, omitted, kind)
			}
		}
		pick := affectedAuditPick{fallback: impact.Verdict == appmap.VerdictFullSuite, files: map[string]bool{}, keys: map[string]bool{},
			size: len(impact.SpecFiles) + len(impact.TestKeys), state: impact.Verdict}
		for _, file := range impact.SpecFiles {
			pick.files[file] = true
		}
		for _, key := range impact.TestKeys {
			pick.keys[key] = true
		}
		for _, unknown := range impact.Unknown {
			pick.unknown = append(pick.unknown, unknown.Reason)
		}
		return pick, "", nil
	default:
		if change.E2E == nil {
			return affectedAuditPick{}, "the change configures no e2e manifest", nil
		}
		return pickAffectedAuditE2E(ctx, clone, change, head)
	}
}

// affectedAuditE2ECompiler finds the corvint-test companion beside this
// executable, else on PATH. e2e-compile runs out of process so the core
// binary does not link the CCE environment's net/http (invariant 7).
func affectedAuditE2ECompiler() (string, error) {
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), "corvint-test"+filepath.Ext(self))
		if info, err := os.Stat(sibling); err == nil && info.Mode().IsRegular() {
			return sibling, nil
		}
	}
	return exec.LookPath("corvint-test")
}

// affectedAuditCompileReport is the part of the corvint-test e2e compile
// report (CCE-V0-005) the audit scores.
type affectedAuditCompileReport struct {
	Roots   []appmap.RootPin `json:"roots"`
	State   string           `json:"state"`
	Unknown []struct {
		Reason string `json:"reason"`
	} `json:"unknown"`
	Scenarios []struct {
		Placement string `json:"placement"`
	} `json:"scenarios"`
	Truncated int `json:"truncated"`
}

// pickAffectedAuditE2E runs `corvint-test e2e compile` on the clone, writing
// its scenarios to scratch beside the clone. It never falls back: an unknown
// span is listed, and a test outside the compiled scenarios that had to run
// is a miss.
func pickAffectedAuditE2E(ctx context.Context, clone string, change affectedAuditInChg, head string) (affectedAuditPick, string, error) {
	compiler, err := affectedAuditE2ECompiler()
	if err != nil {
		return affectedAuditPick{}, "", &gokernel.Error{Code: "e2e-compiler-unavailable", Message: "corvint-test is neither beside corvint nor on PATH"}
	}
	out := filepath.Join(filepath.Dir(clone), "e2e-compiled")
	if err := os.Mkdir(out, 0o700); err != nil {
		return affectedAuditPick{}, "", &gokernel.Error{Code: "e2e-compile-failed", Message: "cannot create the scenario scratch directory: " + err.Error()}
	}
	var stdout, stderr bytes.Buffer
	arguments := append([]string{"e2e", "compile", "--root", clone, "--base", change.Base, "--head", head,
		"--manifest", change.E2E.Manifest, "--out", out}, change.E2E.rootArgs...)
	command := exec.CommandContext(ctx, compiler, arguments...)
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return affectedAuditPick{}, "", &gokernel.Error{Code: "e2e-compile-failed", Message: strings.TrimSpace(stderr.String())}
	}
	var compiled affectedAuditCompileReport
	if err := json.Unmarshal(stdout.Bytes(), &compiled); err != nil {
		return affectedAuditPick{}, "", &gokernel.Error{Code: "e2e-compile-failed", Message: "unreadable e2e compile report: " + err.Error()}
	}
	if compiled.Truncated != 0 {
		// A trimmed list is not the selection; it is never scored as one.
		return affectedAuditPick{}, "", &gokernel.Error{Code: "e2e-compile-failed", Message: fmt.Sprintf("the e2e compile report dropped %d scenarios", compiled.Truncated)}
	}
	pick := affectedAuditPick{files: map[string]bool{}, state: compiled.State, roots: compiled.Roots}
	for _, scenario := range compiled.Scenarios {
		if scenario.Placement != "" {
			pick.files[scenario.Placement] = true
			pick.size++
		}
	}
	for _, unknown := range compiled.Unknown {
		pick.unknown = append(pick.unknown, unknown.Reason)
	}
	return pick, "", nil
}

func affectedAuditRatio(numerator, denominator int) string {
	if denominator == 0 {
		return auditUndefined
	}
	return affectedAuditDecimal(float64(numerator) / float64(denominator))
}

// scoreAffectedAuditSelector scores one selector on one change. Missing
// observation, an unobserved test the selector skipped, or a selector error
// is UNKNOWN, never SAFE.
func scoreAffectedAuditSelector(name string, pick affectedAuditPick, reason string, failure error, elapsed int64, observation affectedAuditObservation, changed []affectedAuditOutcome) affectedAuditSelection {
	scored := affectedAuditSelection{ElapsedMS: elapsed, Name: name, Misses: []affectedAuditID{}, UnknownReasons: []string{},
		UnobservedUnselected: []affectedAuditID{}, Recall: auditUnknown, Precision: auditUnknown}
	switch {
	case reason != "":
		scored.Scope, scored.Verdict, scored.Reason, scored.Recall, scored.Precision = auditScopeNotRun, auditVerdictNotRun, reason, auditVerdictNotRun, auditVerdictNotRun
		return scored
	case failure != nil:
		var kernel *gokernel.Error
		scored.Error = &affectedAuditFailure{Code: "selector-error", Message: failure.Error()}
		if errors.As(failure, &kernel) {
			scored.Error = &affectedAuditFailure{Code: kernel.Code, Message: kernel.Message}
		}
		scored.Scope, scored.Verdict = auditScopeError, auditVerdictUnknown
		return scored
	}
	scored.Fallback, scored.SelectionSize, scored.SelectorState, scored.Roots = pick.fallback, pick.size, pick.state, pick.roots
	scored.Scope = auditScopeBounded
	if pick.fallback {
		scored.Scope = auditScopeFallback
	}
	scored.UnknownReasons = sortedUniqueStrings(append([]string{}, pick.unknown...))
	if observation.State != "OBSERVED" {
		scored.Verdict = auditVerdictUnknown
		return scored
	}
	for _, outcome := range changed {
		if pick.selects(outcome.id) {
			scored.SelectedTests++
		}
	}
	for _, id := range observation.MustRerun {
		if pick.selects(id) {
			scored.TruePositives++
		} else {
			scored.Misses = append(scored.Misses, id)
		}
	}
	unobservedSelected := 0
	for _, id := range observation.Unobserved {
		if pick.selects(id) {
			unobservedSelected++
		} else {
			scored.UnobservedUnselected = append(scored.UnobservedUnselected, id)
		}
	}
	// Any unobserved test, selected or not, could have had to rerun, so it
	// leaves the recall denominator unknown.
	if len(observation.Unobserved) == 0 {
		scored.Recall = affectedAuditRatio(scored.TruePositives, len(observation.MustRerun))
	}
	if unobservedSelected == 0 {
		scored.Precision = affectedAuditRatio(scored.TruePositives, scored.SelectedTests)
	}
	switch {
	case len(scored.Misses) != 0:
		scored.Verdict = auditVerdictMiss
	case len(scored.UnobservedUnselected) != 0:
		scored.Verdict = auditVerdictUnknown
	default:
		scored.Verdict = auditVerdictSafe
	}
	return scored
}

func readAffectedAuditFile(name string, limit int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("over the %d-byte bound", limit)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err == nil && int64(len(body)) > limit {
		err = fmt.Errorf("over the %d-byte bound", limit)
	}
	return body, err
}

// readAffectedAuditOutcomes reads one outcome file, relative to the manifest
// directory unless absolute, into tests keyed by file, classname and name. A
// duplicate key keeps its worst status, so a retried failure is not hidden.
func readAffectedAuditOutcomes(directory, name, format, prefix string) ([]affectedAuditOutcome, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(directory, name)
	}
	body, err := readAffectedAuditFile(name, affectedAuditMaxOutcome)
	if err != nil {
		return nil, err
	}
	var ids []affectedAuditID
	if format == "junit" {
		ids, err = parseAffectedAuditJUnit(body)
	} else {
		ids, err = parseAffectedAuditPlaywright(body)
	}
	if err != nil {
		return nil, err
	}
	byKey := map[string]int{}
	var outcomes []affectedAuditOutcome
	for _, id := range ids {
		if id.File != "" {
			id.File = path.Clean(path.Join(prefix, filepath.ToSlash(id.File)))
		}
		key := id.File + "\x00" + id.Classname + "\x00" + id.Name
		if at, ok := byKey[key]; ok {
			if affectedAuditRank(id.Status) > affectedAuditRank(outcomes[at].id.Status) {
				outcomes[at].id.Status = id.Status
			}
			continue
		}
		byKey[key] = len(outcomes)
		outcomes = append(outcomes, affectedAuditOutcome{id: id, key: key})
	}
	slices.SortFunc(outcomes, func(a, b affectedAuditOutcome) int { return strings.Compare(a.key, b.key) })
	return outcomes, nil
}

func affectedAuditRank(status string) int {
	switch status {
	case "failed":
		return 3
	case "flaky":
		return 2
	case "passed":
		return 1
	}
	return 0
}

type affectedAuditJUnitCase struct {
	Name      string    `xml:"name,attr"`
	Classname string    `xml:"classname,attr"`
	File      string    `xml:"file,attr"`
	Failure   *struct{} `xml:"failure"`
	Error     *struct{} `xml:"error"`
	Skipped   *struct{} `xml:"skipped"`
}

// parseAffectedAuditJUnit streams JUnit XML: a testcase's file is its own
// attribute, else its enclosing testsuite's.
func parseAffectedAuditJUnit(body []byte) ([]affectedAuditID, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var suites []string
	var ids []affectedAuditID
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "testsuite", "testsuites":
				if len(suites) >= affectedAuditMaxDepth {
					return nil, fmt.Errorf("testsuite nesting over %d", affectedAuditMaxDepth)
				}
				file := ""
				for _, attribute := range element.Attr {
					if attribute.Name.Local == "file" {
						file = attribute.Value
					}
				}
				if file == "" && len(suites) != 0 {
					file = suites[len(suites)-1]
				}
				suites = append(suites, file)
			case "testcase":
				var testcase affectedAuditJUnitCase
				if err := decoder.DecodeElement(&testcase, &element); err != nil {
					return nil, err
				}
				if testcase.Name == "" {
					return nil, errors.New("testcase without a name")
				}
				if testcase.File == "" && len(suites) != 0 {
					testcase.File = suites[len(suites)-1]
				}
				status := "passed"
				if testcase.Failure != nil || testcase.Error != nil {
					status = "failed"
				} else if testcase.Skipped != nil {
					status = "skipped"
				}
				if ids = append(ids, affectedAuditID{Classname: testcase.Classname, File: testcase.File, Name: testcase.Name, Status: status}); len(ids) > affectedAuditMaxTests {
					return nil, fmt.Errorf("over %d testcases", affectedAuditMaxTests)
				}
			}
		case xml.EndElement:
			if (element.Name.Local == "testsuite" || element.Name.Local == "testsuites") && len(suites) != 0 {
				suites = suites[:len(suites)-1]
			}
		}
	}
	if ids == nil {
		return nil, errors.New("no testcase")
	}
	return ids, nil
}

type affectedAuditPWSuite struct {
	Title  string                 `json:"title"`
	File   string                 `json:"file"`
	Specs  []affectedAuditPWSpec  `json:"specs"`
	Suites []affectedAuditPWSuite `json:"suites"`
}

type affectedAuditPWSpec struct {
	Title string `json:"title"`
	File  string `json:"file"`
	Tests []struct {
		ProjectName string `json:"projectName"`
		Status      string `json:"status"`
	} `json:"tests"`
}

// parseAffectedAuditPlaywright reads a Playwright JSON reporter document: one
// test per spec and project, named by its title path.
func parseAffectedAuditPlaywright(body []byte) ([]affectedAuditID, error) {
	var document struct {
		Suites []affectedAuditPWSuite `json:"suites"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	var ids []affectedAuditID
	var walk func(suites []affectedAuditPWSuite, titles []string, depth int) error
	walk = func(suites []affectedAuditPWSuite, titles []string, depth int) error {
		if depth > affectedAuditMaxDepth {
			return fmt.Errorf("suite nesting over %d", affectedAuditMaxDepth)
		}
		for _, suite := range suites {
			nested := titles
			if suite.Title != "" && suite.Title != suite.File {
				nested = append(slices.Clone(titles), suite.Title)
			}
			for _, spec := range suite.Specs {
				file := spec.File
				if file == "" {
					file = suite.File
				}
				for _, test := range spec.Tests {
					status := map[string]string{"expected": "passed", "flaky": "flaky", "unexpected": "failed", "skipped": "skipped"}[test.Status]
					if status == "" {
						return fmt.Errorf("test status %q", test.Status)
					}
					name := strings.Join(append(slices.Clone(nested), spec.Title), " > ")
					if test.ProjectName != "" {
						name = "[" + test.ProjectName + "] " + name
					}
					if ids = append(ids, affectedAuditID{File: file, Name: name, Status: status}); len(ids) > affectedAuditMaxTests {
						return fmt.Errorf("over %d tests", affectedAuditMaxTests)
					}
				}
			}
			if err := walk(suite.Suites, nested, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(document.Suites, nil, 0); err != nil {
		return nil, err
	}
	if ids == nil {
		return nil, errors.New("no test")
	}
	return ids, nil
}

// aggregateAffectedAudit sums each selector over the changes it ran on. Only
// changes with a SAFE or MISS verdict count toward N; an UNKNOWN change is
// counted apart and never as safe. The bound is the exact one-sided 95%
// Clopper-Pearson upper bound on the per-change miss rate, with the rule of
// three when no observed change missed.
func aggregateAffectedAudit(changes []affectedAuditChange) []affectedAuditAggregate {
	aggregates := []affectedAuditAggregate{}
	for index, name := range affectedAuditSelectorNames {
		aggregate := affectedAuditAggregate{Name: name, Confidence: "0.95", UpperBoundMethod: "clopper-pearson-one-sided",
			Recall: auditUnknown, Precision: auditUnknown, MissRateUpperBound: auditUnknown}
		recallUnknown, precisionUnknown := false, false
		recallHits, recallTotal, precisionHits, precisionTotal := 0, 0, 0, 0
		for _, change := range changes {
			scored := change.Selectors[index]
			if scored.Verdict == auditVerdictNotRun {
				continue
			}
			aggregate.Changes++
			aggregate.ElapsedMS += scored.ElapsedMS
			if scored.Fallback {
				aggregate.FallbackChanges++
			}
			switch scored.Verdict {
			case auditVerdictUnknown:
				aggregate.UnknownChanges++
			case auditVerdictMiss:
				aggregate.ChangesWithMiss++
				aggregate.ObservedChanges++
			case auditVerdictSafe:
				aggregate.ObservedChanges++
			}
			if change.Observation.State == "OBSERVED" && scored.Scope != auditScopeError {
				aggregate.MissedTests += len(scored.Misses)
				aggregate.MustRerunTests += len(change.Observation.MustRerun)
				aggregate.TruePositives += scored.TruePositives
				aggregate.SelectedTests += scored.SelectedTests
			}
			if scored.Recall == auditUnknown {
				recallUnknown = true
			} else {
				recallHits += scored.TruePositives
				recallTotal += len(change.Observation.MustRerun)
			}
			if scored.Precision == auditUnknown {
				precisionUnknown = true
			} else {
				precisionHits += scored.TruePositives
				precisionTotal += scored.SelectedTests
			}
		}
		if aggregate.Changes == 0 {
			aggregate.Recall, aggregate.Precision, aggregate.MissRateUpperBound = auditVerdictNotRun, auditVerdictNotRun, auditVerdictNotRun
			aggregates = append(aggregates, aggregate)
			continue
		}
		// An UNKNOWN change or an UNKNOWN per-change ratio keeps the aggregate
		// ratio UNKNOWN: a ratio over the rest would read as covering every change.
		if !recallUnknown && aggregate.UnknownChanges == 0 {
			aggregate.Recall = affectedAuditRatio(recallHits, recallTotal)
		}
		if !precisionUnknown && aggregate.UnknownChanges == 0 {
			aggregate.Precision = affectedAuditRatio(precisionHits, precisionTotal)
		}
		// The bound is UNKNOWN for the same reason: an UNKNOWN change could be a miss.
		if aggregate.ObservedChanges != 0 && aggregate.UnknownChanges == 0 {
			aggregate.MissRateUpperBound = affectedAuditDecimal(clopperPearsonUpper(aggregate.ChangesWithMiss, aggregate.ObservedChanges, 1-affectedAuditConfidence))
			if aggregate.ChangesWithMiss == 0 {
				aggregate.RuleOfThree = affectedAuditDecimal(math.Min(1, 3/float64(aggregate.ObservedChanges)))
			}
		}
		aggregates = append(aggregates, aggregate)
	}
	return aggregates
}

func affectedAuditDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

// clopperPearsonUpper returns the p at which P(X <= k | n, p) = alpha.
func clopperPearsonUpper(k, n int, alpha float64) float64 {
	if k >= n {
		return 1
	}
	low, high := 0.0, 1.0
	for range 200 {
		middle := (low + high) / 2
		if binomialCDF(k, n, middle) > alpha {
			low = middle
		} else {
			high = middle
		}
	}
	return high
}

func binomialCDF(k, n int, p float64) float64 {
	total := 0.0
	for i := 0; i <= k; i++ {
		total += math.Exp(logChoose(n, i) + float64(i)*math.Log(p) + float64(n-i)*math.Log1p(-p))
	}
	return total
}

func logChoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}
