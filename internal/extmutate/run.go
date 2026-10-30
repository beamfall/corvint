package extmutate

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// Schema is the discrimination document (CCE-V0-174).
const Schema = "e2e-external-discrimination/0"

// Verdict sources (CCE-V0-173).
const (
	VerdictExit     = "exit"
	VerdictProvider = "provider"
)

// WallTimeExhausted names a command --wall-time cut or left unstarted
// (CCE-V0-172): a run's infrastructure reason and rebuild result, and the
// restore rebuild result.
const WallTimeExhausted = "wall-time-exhausted"

// Per-(mutant, test) results and per-(hunk, test) states (CCE-V0-174).
const (
	Killed   = "killed"
	Survived = "survived"
	NotRun   = "not-run"
)

// Bounds and defaults (CCE-V0-172).
const (
	DefaultMaxMutants     = 20
	MaxMutants            = 500
	DefaultWallTime       = 30 * time.Minute
	MaxWallTime           = 24 * time.Hour
	DefaultRunTimeout     = 10 * time.Minute
	MaxRunTimeout         = 2 * time.Hour
	DefaultRebuildTimeout = 10 * time.Minute
	maxHunks              = 256
	maxExcluded           = 1024
	maxDirectoryNames     = 64
	maxArgv               = 64
	maxSourceBytes        = 4 << 20
	maxRunOutput          = 4 << 20
	maxGitOutput          = 32 << 20
)

// Refusal is a named, fail-closed refusal before any mutant is applied.
type Refusal struct {
	Code   string
	Detail string
}

func (r *Refusal) Error() string {
	if r.Detail == "" {
		return r.Code
	}
	return r.Code + ": " + r.Detail
}

func refuse(code, format string, args ...any) error {
	return &Refusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Options is one discrimination request.
type Options struct {
	Root           string // repository the base..head diff is read from
	Base, Head     string
	AppDir         string // clean worktree at Head that is mutated and restored
	TestsDir       string // where the test command runs; defaults to AppDir
	TestCommand    []string
	RebuildCommand []string // optional: run after each mutant is applied and after the last restore
	Verdict        string
	MaxMutants     int
	WallTime       time.Duration
	RunTimeout     time.Duration
	RebuildTimeout time.Duration
	// DirectoryMatch, ExcludeDirectories and KeepDirectories adjust the
	// directory-name exclusion (CCE-V0-237); MapPath is an optional cem/1.0
	// map whose hunk IDs the hunks are joined to (CCE-V0-238).
	DirectoryMatch     string
	ExcludeDirectories []string
	KeepDirectories    []string
	MapPath            string
	exclusion          Exclusion
}

// Document is the discrimination record (CCE-V0-174).
type Document struct {
	Schema         string       `json:"schema"`
	OperatorSet    string       `json:"operatorSet"`
	BaseRevision   string       `json:"baseRevision"`
	AppRevision    string       `json:"appRevision"`
	Tests          TestRevision `json:"testRevision"`
	TestCommand    []string     `json:"testCommand"`
	RebuildCommand []string     `json:"rebuildCommand"`
	Verdict        string       `json:"verdict"`
	// InfrastructureDistinguished is false for the exit verdict: a nonzero
	// exit counts as a kill whatever its cause.
	InfrastructureDistinguished bool           `json:"infrastructureDistinguished"`
	Bounds                      Bounds         `json:"bounds"`
	Outcome                     string         `json:"outcome"`
	Baseline                    Run            `json:"baseline"`
	Exclusion                   Exclusion      `json:"exclusion"`
	Excluded                    []ExcludedFile `json:"excluded"`
	ExcludedOmitted             int            `json:"excludedOmitted"`
	CEMMap                      *CEMMapPin     `json:"cemMap"`
	Hunks                       []Hunk         `json:"hunks"`
	Mutants                     []Mutant       `json:"mutants"`
	MutantsPlanned              int            `json:"mutantsPlanned"`
	MutantsOmitted              int            `json:"mutantsOmitted"`
	Records                     []Record       `json:"records"`
	RestoreRebuild              string         `json:"restoreRebuild"`
	AppDirRestored              bool           `json:"appDirRestored"`
}

// TestRevision pins the test side. Revision is empty when the tests directory
// is not a Git worktree. ChangesDigest is the SHA-256 of the tracked changes
// against HEAD (empty when clean or not a worktree), so a tests directory
// that is dirty at both ends but changed in between is not unchanged;
// Unchanged compares all three at start and end.
type TestRevision struct {
	Dir           string `json:"dir"`
	Revision      string `json:"revision"`
	Clean         bool   `json:"clean"`
	ChangesDigest string `json:"changesDigest"`
	Unchanged     bool   `json:"unchanged"`
}

// Bounds are the effective limits of the run.
type Bounds struct {
	MaxMutants       int   `json:"maxMutants"`
	WallTimeMS       int64 `json:"wallTimeMs"`
	RunTimeoutMS     int64 `json:"runTimeoutMs"`
	RebuildTimeoutMS int64 `json:"rebuildTimeoutMs"`
}

// ExcludedFile is one changed file that contributes no hunk, with the rule
// that excluded it (CCE-V0-236).
type ExcludedFile struct {
	Path string `json:"path"`
	Rule string `json:"rule"`
}

// CEMMapPin pins the cem/1.0 map the hunks were joined to (CCE-V0-238).
type CEMMapPin struct {
	Sha256       string `json:"sha256"`
	BaseRevision string `json:"baseRevision"`
	PatchSha256  string `json:"patchSha256"`
}

// CEM join results of one hunk (CCE-V0-238).
const (
	CEMJoined   = "joined"
	CEMUnjoined = "no-containing-cem-hunk"
)

// Hunk is one changed application hunk of the head side. CEMHunk is the
// containing CEM hunk's ID when a map was joined.
type Hunk struct {
	Path     string   `json:"path"`
	Language Language `json:"language"`
	NewStart int      `json:"newStart"`
	NewCount int      `json:"newCount"`
	Mutants  int      `json:"mutants"`
	CEMHunk  string   `json:"cemHunk,omitempty"`
	CEMJoin  string   `json:"cemJoin,omitempty"`
}

// Run is one rebuild-and-test execution: the baseline or one mutant.
type Run struct {
	Rebuild        string        `json:"rebuild"`
	ExitStatus     int           `json:"exitStatus"`
	ReceiptDigest  string        `json:"receiptDigest"`
	Infrastructure string        `json:"infrastructure,omitempty"`
	DurationMS     int64         `json:"durationMs"`
	Tests          []TestOutcome `json:"tests"`
}

// TestOutcome is one test's observed state in one run.
type TestOutcome struct {
	Test  string `json:"test"`
	State string `json:"state"`
}

// Mutant is one applied operator site and its per-test results.
type Mutant struct {
	ID   string `json:"id"`
	Hunk int    `json:"hunk"`
	Path string `json:"path"`
	Site
	Run     *Run            `json:"run,omitempty"`
	Results []MutantOutcome `json:"results"`
}

// MutantOutcome is one (mutant, test) result with its closed reason.
type MutantOutcome struct {
	Test   string `json:"test"`
	Result string `json:"result"`
	Reason string `json:"reason,omitempty"`
}

// Record is the per-(hunk, test) fold: killed when any mutant was killed,
// survived only when every mutant ran and survived, otherwise not-run.
type Record struct {
	Hunk     int    `json:"hunk"`
	Path     string `json:"path"`
	NewStart int    `json:"newStart"`
	NewCount int    `json:"newCount"`
	CEMHunk  string `json:"cemHunk,omitempty"`
	Test     string `json:"test"`
	Mutants  int    `json:"mutants"`
	Killed   int    `json:"killed"`
	Survived int    `json:"survived"`
	NotRun   int    `json:"notRun"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
}

func (o *Options) normalize() error {
	if o.Root == "" || o.Base == "" || o.Head == "" || o.AppDir == "" {
		return refuse("discriminate-input-required", "--root, --base, --head and --app-dir are required")
	}
	if o.TestsDir == "" {
		o.TestsDir = o.AppDir
	}
	for _, dir := range []*string{&o.Root, &o.AppDir, &o.TestsDir} {
		abs, err := filepath.Abs(*dir)
		if err != nil {
			return refuse("discriminate-input-required", "%v", err)
		}
		*dir = abs
	}
	if err := checkArgv("test", o.TestCommand, true); err != nil {
		return err
	}
	if err := checkArgv("rebuild", o.RebuildCommand, false); err != nil {
		return err
	}
	if o.Verdict == "" {
		o.Verdict = VerdictExit
	}
	if o.Verdict != VerdictExit && o.Verdict != VerdictProvider {
		return refuse("discriminate-verdict-invalid", "verdict must be exit or provider")
	}
	for _, bound := range []struct {
		value      *time.Duration
		def, limit time.Duration
		name       string
	}{{&o.WallTime, DefaultWallTime, MaxWallTime, "wall-time"}, {&o.RunTimeout, DefaultRunTimeout, MaxRunTimeout, "run-timeout"}, {&o.RebuildTimeout, DefaultRebuildTimeout, MaxRunTimeout, "rebuild-timeout"}} {
		if *bound.value == 0 {
			*bound.value = bound.def
		}
		if *bound.value < 0 || *bound.value > bound.limit {
			return refuse("discriminate-bound-invalid", "%s must be in (0, %s]", bound.name, bound.limit)
		}
	}
	if o.MaxMutants == 0 {
		o.MaxMutants = DefaultMaxMutants
	}
	if o.MaxMutants < 0 || o.MaxMutants > MaxMutants {
		return refuse("discriminate-bound-invalid", "max-mutants must be in 1..%d", MaxMutants)
	}
	return o.normalizeExclusion()
}

// normalizeExclusion builds the effective directory-name rule (CCE-V0-237).
func (o *Options) normalizeExclusion() error {
	o.exclusion = DefaultExclusion()
	if o.DirectoryMatch != "" {
		if o.DirectoryMatch != DirectoryMatchAny && o.DirectoryMatch != DirectoryMatchLeading {
			return refuse("discriminate-exclusion-invalid", "directory match must be any or leading")
		}
		o.exclusion.DirectoryMatch = o.DirectoryMatch
	}
	if len(o.ExcludeDirectories) > maxDirectoryNames || len(o.KeepDirectories) > maxDirectoryNames {
		return refuse("discriminate-exclusion-invalid", "at most %d directory names", maxDirectoryNames)
	}
	names := map[string]bool{}
	for _, name := range o.exclusion.Directories {
		names[name] = true
	}
	for _, list := range []struct {
		names []string
		keep  bool
	}{{o.ExcludeDirectories, false}, {o.KeepDirectories, true}} {
		for _, name := range list.names {
			if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
				return refuse("discriminate-exclusion-invalid", "%q is not one directory name", name)
			}
			if list.keep && !names[name] {
				return refuse("discriminate-exclusion-invalid", "%q is not an excluded directory name", name)
			}
			names[name] = !list.keep
		}
	}
	o.exclusion.Directories = []string{}
	for _, name := range sortedBoolKeys(names) {
		o.exclusion.Directories = append(o.exclusion.Directories, name)
	}
	return nil
}

func sortedBoolKeys(m map[string]bool) []string {
	keys := []string{}
	for k, v := range m {
		if v {
			keys = append(keys, k)
		}
	}
	sortStrings(keys)
	return keys
}

func checkArgv(name string, argv []string, required bool) error {
	if len(argv) == 0 && !required {
		return nil
	}
	if len(argv) == 0 || len(argv) > maxArgv || argv[0] == "" {
		return refuse("discriminate-command-invalid", "%s command must be a JSON argv of 1..%d tokens", name, maxArgv)
	}
	if secretscreen.MatchString(strings.Join(argv, " ")) {
		return refuse("discriminate-command-invalid", "%s command is secret-shaped", name)
	}
	return nil
}

// Discriminate runs the request. A refusal returns before the app directory
// is touched; once mutation starts, the document is always returned, with
// AppDirRestored false and an error when a restore could not be verified.
func Discriminate(ctx context.Context, opts Options) (Document, error) {
	if err := opts.normalize(); err != nil {
		return Document{}, err
	}
	base, err := revParse(ctx, opts.Root, opts.Base+"^{commit}")
	if err != nil {
		return Document{}, refuse("discriminate-revision-invalid", "base: %v", err)
	}
	head, err := revParse(ctx, opts.Root, opts.Head+"^{commit}")
	if err != nil {
		return Document{}, refuse("discriminate-revision-invalid", "head: %v", err)
	}
	if err := checkAppDir(ctx, opts.AppDir, head); err != nil {
		return Document{}, err
	}
	doc := Document{Schema: Schema, OperatorSet: OperatorSet, BaseRevision: base, AppRevision: head, TestCommand: opts.TestCommand, RebuildCommand: nonNil(opts.RebuildCommand),
		Verdict: opts.Verdict, InfrastructureDistinguished: opts.Verdict == VerdictProvider, Exclusion: opts.exclusion, Excluded: []ExcludedFile{},
		Hunks: []Hunk{}, Mutants: []Mutant{}, Records: []Record{}, RestoreRebuild: "none",
		Bounds: Bounds{MaxMutants: opts.MaxMutants, WallTimeMS: opts.WallTime.Milliseconds(), RunTimeoutMS: opts.RunTimeout.Milliseconds(), RebuildTimeoutMS: opts.RebuildTimeout.Milliseconds()}}
	doc.Tests = testRevision(ctx, opts.TestsDir)
	hunks, excluded, omitted, err := changedHunks(ctx, opts.Root, base, head, opts.exclusion)
	if err != nil {
		return Document{}, err
	}
	doc.Excluded, doc.ExcludedOmitted = excluded, omitted
	if opts.MapPath != "" {
		if doc.CEMMap, err = joinCEM(ctx, opts.Root, opts.MapPath, base, head, hunks); err != nil {
			return Document{}, err
		}
	}
	sources := map[string][]byte{}
	for _, h := range hunks {
		if _, ok := sources[h.Path]; ok {
			continue
		}
		data, err := readSource(filepath.Join(opts.AppDir, filepath.FromSlash(h.Path)))
		if err != nil {
			return Document{}, refuse("discriminate-source-unreadable", "%s: %v", h.Path, err)
		}
		sources[h.Path] = data
	}
	doc.Hunks = hunks
	planned := planMutants(doc.Hunks, sources)
	doc.MutantsPlanned = len(planned)
	if len(planned) > opts.MaxMutants {
		doc.MutantsOmitted = len(planned) - opts.MaxMutants
		planned = planned[:opts.MaxMutants]
	}
	doc.Mutants = planned
	deadline := time.Now().Add(opts.WallTime)
	doc.AppDirRestored = true
	if len(planned) == 0 {
		doc.Outcome = "no-mutant-sites"
		doc.Baseline = Run{Rebuild: "none", ExitStatus: -1, Tests: []TestOutcome{}}
		finish(ctx, &doc, opts, map[string]string{}, deadline)
		return doc, nil
	}
	doc.Baseline = execute(ctx, opts, "baseline", deadline)
	baseline := map[string]string{}
	for _, t := range doc.Baseline.Tests {
		baseline[t.Test] = t.State
	}
	baselineFailure := ""
	switch {
	case doc.Baseline.Rebuild == "failed":
		baselineFailure = "baseline-rebuild-failed"
	case doc.Baseline.Infrastructure != "":
		baselineFailure = "baseline-infrastructure"
	}
	var restoreErr error
	for i := range doc.Mutants {
		m := &doc.Mutants[i]
		switch {
		case baselineFailure != "":
			m.Results = notRunAll(baseline, baselineFailure)
			continue
		case restoreErr != nil:
			m.Results = notRunAll(baseline, "restore-failed")
			continue
		case ctx.Err() != nil:
			m.Results = notRunAll(baseline, "cancelled")
			continue
		case time.Now().After(deadline):
			m.Results = notRunAll(baseline, WallTimeExhausted)
			continue
		}
		file := filepath.Join(opts.AppDir, filepath.FromSlash(m.Path))
		original := sources[m.Path]
		mutated, err := applySite(original, m.Site)
		if err != nil {
			m.Results = notRunAll(baseline, "site-mismatch")
			continue
		}
		if err := writeSource(file, mutated); err != nil {
			restoreErr = restore(file, original)
			m.Results = notRunAll(baseline, "apply-failed")
			continue
		}
		run := execute(ctx, opts, m.ID, deadline)
		restoreErr = restore(file, original)
		m.Run = &run
		m.Results = classify(opts.Verdict, baseline, run)
	}
	if baselineFailure != "" {
		doc.Outcome = baselineFailure
	}
	finish(ctx, &doc, opts, baseline, deadline)
	if restoreErr != nil {
		doc.AppDirRestored = false
		doc.Outcome = "restore-failed"
		return doc, refuse("discriminate-restore-failed", "%v", restoreErr)
	}
	if !doc.AppDirRestored {
		doc.Outcome = "restore-failed"
		return doc, refuse("discriminate-restore-failed", "%s is not clean at %s after the run", opts.AppDir, head)
	}
	return doc, nil
}

// finish rebuilds the restored tree, verifies it and the test revision, and
// folds the records.
func finish(ctx context.Context, doc *Document, opts Options, baseline map[string]string, deadline time.Time) {
	if len(opts.RebuildCommand) != 0 && len(doc.Mutants) != 0 {
		doc.RestoreRebuild = boundedCommand(ctx, opts.RebuildCommand, opts.AppDir, opts.RebuildTimeout, deadline, "restore")
	}
	doc.AppDirRestored = doc.AppDirRestored && checkAppDir(ctx, opts.AppDir, doc.AppRevision) == nil
	end := testRevision(ctx, opts.TestsDir)
	doc.Tests.Unchanged = end.Revision == doc.Tests.Revision && end.Clean == doc.Tests.Clean &&
		end.ChangesDigest == doc.Tests.ChangesDigest && end.ChangesDigest != digestUnavailable
	if !doc.Tests.Unchanged {
		for i := range doc.Mutants {
			for j := range doc.Mutants[i].Results {
				if doc.Mutants[i].Results[j].Result != NotRun {
					doc.Mutants[i].Results[j] = MutantOutcome{Test: doc.Mutants[i].Results[j].Test, Result: NotRun, Reason: "test-revision-drift"}
				}
			}
		}
	}
	doc.Records = fold(doc.Hunks, doc.Mutants, baseline)
	if doc.Outcome == "" {
		doc.Outcome = "complete"
		for _, r := range doc.Records {
			if r.State == NotRun {
				doc.Outcome = "partial"
			}
		}
		if doc.MutantsOmitted != 0 || !doc.Tests.Unchanged {
			doc.Outcome = "partial"
		}
	}
}

func nonNil(argv []string) []string {
	if argv == nil {
		return []string{}
	}
	return argv
}

func notRunAll(baseline map[string]string, reason string) []MutantOutcome {
	out := []MutantOutcome{}
	for _, test := range sortedKeys(baseline) {
		out = append(out, MutantOutcome{Test: test, Result: NotRun, Reason: reason})
	}
	return out
}

// classify maps one mutant run onto every baseline test (CCE-V0-173).
func classify(verdict string, baseline map[string]string, run Run) []MutantOutcome {
	observed := map[string]string{}
	for _, t := range run.Tests {
		observed[t.Test] = t.State
	}
	out := []MutantOutcome{}
	for _, test := range sortedKeys(baseline) {
		result := MutantOutcome{Test: test, Result: NotRun}
		state, ok := observed[test]
		switch {
		case baseline[test] != "passed":
			result.Reason = "baseline-not-passing"
		case run.Rebuild == "failed":
			result.Reason = "rebuild-failed"
		case run.Infrastructure != "":
			result.Reason = "infrastructure: " + run.Infrastructure
		case !ok:
			result.Reason = "test-not-reported"
		case state == "failed":
			result.Result = Killed
		case state == "passed":
			result.Result = Survived
		default:
			result.Reason = "test-" + state
		}
		out = append(out, result)
	}
	return out
}

func fold(hunks []Hunk, mutants []Mutant, baseline map[string]string) []Record {
	records := []Record{}
	for index, h := range hunks {
		for _, test := range sortedKeys(baseline) {
			r := Record{Hunk: index, Path: h.Path, NewStart: h.NewStart, NewCount: h.NewCount, CEMHunk: h.CEMHunk, Test: test, State: NotRun}
			reason := ""
			for _, m := range mutants {
				if m.Hunk != index {
					continue
				}
				r.Mutants++
				for _, o := range m.Results {
					if o.Test != test {
						continue
					}
					switch o.Result {
					case Killed:
						r.Killed++
					case Survived:
						r.Survived++
					default:
						r.NotRun++
						if reason == "" {
							reason = o.Reason
						}
					}
				}
			}
			switch {
			case r.Killed > 0:
				r.State = Killed
			case r.Mutants == 0 && h.Mutants == 0:
				r.Reason = "no-operator-site"
			case r.Mutants < h.Mutants:
				// Some of the hunk's mutants were omitted by --max-mutants.
				r.Reason = "mutant-bound"
			case r.NotRun == 0 && r.Survived == r.Mutants:
				r.State = Survived
			default:
				r.Reason = reason
			}
			records = append(records, r)
		}
	}
	return records
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// execute rebuilds (when declared) and runs the test command once, each
// bounded by its own timeout cut to the wall time left (CCE-V0-172).
func execute(ctx context.Context, opts Options, id string, deadline time.Time) Run {
	started := time.Now()
	run := Run{Rebuild: "none", ExitStatus: -1, Tests: []TestOutcome{}}
	defer func() { run.DurationMS = time.Since(started).Milliseconds() }()
	if len(opts.RebuildCommand) != 0 {
		run.Rebuild = boundedCommand(ctx, opts.RebuildCommand, opts.AppDir, opts.RebuildTimeout, deadline, id)
		if run.Rebuild == WallTimeExhausted {
			run.Infrastructure = WallTimeExhausted
		}
		if run.Rebuild != "passed" {
			return run
		}
	}
	timeout, cut := commandTimeout(opts.RunTimeout, deadline)
	if timeout <= 0 {
		run.Infrastructure = WallTimeExhausted
		return run
	}
	obs := runCommand(ctx, opts.TestCommand, opts.TestsDir, timeout, id)
	sum := sha256.Sum256(obs.Stdout)
	run.ReceiptDigest = hex.EncodeToString(sum[:])
	if obs.ExitObserved {
		run.ExitStatus = obs.ExitStatus
	}
	switch {
	case !obs.Started:
		run.Infrastructure = "command-start-failed"
	case obs.Cancelled || ctx.Err() != nil:
		run.Infrastructure = "cancelled"
	case obs.TimedOut && cut:
		run.Infrastructure = WallTimeExhausted
	case obs.TimedOut:
		run.Infrastructure = "command-timeout"
	case !obs.OwnedProcessGroupCleanup:
		run.Infrastructure = "command-cleanup-unknown"
	}
	if run.Infrastructure != "" {
		return run
	}
	if opts.Verdict == VerdictExit {
		state := "failed"
		if obs.ExitObserved && obs.ExitStatus == 0 {
			state = "passed"
		}
		run.Tests = []TestOutcome{{Test: "command", State: state}}
		return run
	}
	if obs.StdoutOverflow || obs.OutputOverflow {
		run.Infrastructure = "receipt-overflow"
		return run
	}
	run.Tests, run.Infrastructure = providerOutcomes(obs.Stdout)
	return run
}

// commandTimeout cuts a command's own timeout to the wall time left; cut
// reports that the wall time is the tighter bound.
func commandTimeout(timeout time.Duration, deadline time.Time) (time.Duration, bool) {
	if left := time.Until(deadline); left < timeout {
		return left, true
	}
	return timeout, false
}

// boundedCommand runs a rebuild under commandTimeout: "passed", "failed",
// or WallTimeExhausted when the wall time left none or cut it.
func boundedCommand(ctx context.Context, argv []string, dir string, own time.Duration, deadline time.Time, id string) string {
	timeout, cut := commandTimeout(own, deadline)
	if timeout <= 0 {
		return WallTimeExhausted
	}
	obs := runCommand(ctx, argv, dir, timeout, id)
	switch {
	case obs.passed():
		return "passed"
	case obs.TimedOut && cut:
		return WallTimeExhausted
	default:
		return "failed"
	}
}

type observation struct{ procgroup.Observation }

func (o observation) passed() bool {
	return o.Started && !o.TimedOut && o.ExitObserved && o.ExitStatus == 0 && o.OwnedProcessGroupCleanup
}

func runCommand(ctx context.Context, argv []string, dir string, timeout time.Duration, id string) observation {
	resolved := append([]string{}, argv...)
	if path, err := exec.LookPath(argv[0]); err == nil {
		if abs, err := filepath.Abs(path); err == nil {
			resolved[0] = abs
		}
	}
	env := append(os.Environ(), "CORVINT_EXTMUTATE_RUN="+id)
	return observation{procgroup.Run(ctx, procgroup.Spec{Argv: resolved, Dir: dir, Env: env, Timeout: timeout, OutputLimit: maxRunOutput, OverflowPolicy: procgroup.OverflowTruncate})}
}

// providerOutcomes reads a Corvint JS or Ruby provider document from stdout
// through the strict test-validity decoder. A run-level infrastructure
// failure of the provider, a preflight failure included, makes every test
// not-run; it is never a kill (CCE-V0-173).
func providerOutcomes(stdout []byte) ([]TestOutcome, string) {
	// Qualified documents are compared byte for byte, trailing newline included.
	input, err := testvaliditydoc.Decode(stdout)
	if err != nil {
		return []TestOutcome{}, "receipt-invalid"
	}
	tests := []TestOutcome{}
	if receipt, ok := input.JSReceipt(); ok {
		if receipt.Cancelled {
			return tests, "provider-cancelled"
		}
		if x := receipt.Infrastructure; x != nil {
			if x.Phase != "" {
				return tests, x.Phase + ": " + x.Probe
			}
			return tests, "provider: " + x.Reason
		}
		for _, t := range receipt.Tests {
			id := t.ID
			if id == "" {
				id = t.FullName
			}
			tests = append(tests, TestOutcome{Test: id, State: string(t.State)})
		}
		return dedupe(tests)
	}
	if document, ok := input.RubyDocument(); ok {
		if document.Infrastructure != nil {
			return tests, "provider: ruby-infrastructure"
		}
		for _, e := range document.Examples {
			tests = append(tests, TestOutcome{Test: e.ID, State: e.Status})
		}
		return dedupe(tests)
	}
	return tests, "receipt-unsupported"
}

// dedupe refuses a run whose tests share an identity: neither row can be
// attributed.
func dedupe(tests []TestOutcome) ([]TestOutcome, string) {
	seen := map[string]bool{}
	for _, t := range tests {
		if t.Test == "" || seen[t.Test] {
			return []TestOutcome{}, "test-identity-ambiguous"
		}
		seen[t.Test] = true
	}
	return tests, ""
}

func applySite(source []byte, site Site) ([]byte, error) {
	start, ok := lineOffset(source, site.Line)
	if !ok {
		return nil, errors.New("line out of range")
	}
	at := start + site.Column - 1
	if at+len(site.Original) > len(source) || string(source[at:at+len(site.Original)]) != site.Original {
		return nil, errors.New("site text differs")
	}
	out := append([]byte{}, source[:at]...)
	out = append(out, site.Replacement...)
	return append(out, source[at+len(site.Original):]...), nil
}

func lineOffset(source []byte, line int) (int, bool) {
	offset := 0
	for n := 1; n < line; n++ {
		i := bytes.IndexByte(source[offset:], '\n')
		if i < 0 {
			return 0, false
		}
		offset += i + 1
	}
	return offset, true
}

func readSource(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
		return nil, errors.New("not a regular file within 4 MiB")
	}
	return os.ReadFile(path)
}

func writeSource(path string, data []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, info.Mode().Perm())
}

// restore writes the original bytes back and verifies them.
func restore(path string, original []byte) error {
	if err := writeSource(path, original); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, original) {
		return fmt.Errorf("%s differs from its original after restore", path)
	}
	return nil
}

func planMutants(hunks []Hunk, sources map[string][]byte) []Mutant {
	var mutants []Mutant
	byPath := map[string][]int{}
	var order []string
	for i, h := range hunks {
		if _, ok := byPath[h.Path]; !ok {
			order = append(order, h.Path)
		}
		byPath[h.Path] = append(byPath[h.Path], i)
	}
	for _, name := range order {
		indexes := byPath[name]
		language := hunks[indexes[0]].Language
		blockComment := false
		scanner := bufio.NewScanner(bytes.NewReader(sources[name]))
		scanner.Buffer(make([]byte, 0, 64<<10), maxSourceBytes)
		for line := 1; scanner.Scan(); line++ {
			sites := PlanLine(language, line, strings.TrimSuffix(scanner.Text(), "\r"), &blockComment)
			for _, index := range indexes {
				h := &hunks[index]
				if line < h.NewStart || line >= h.NewStart+h.NewCount {
					continue
				}
				for _, site := range sites {
					h.Mutants++
					mutants = append(mutants, Mutant{Hunk: index, Path: name, Site: site, Results: []MutantOutcome{}})
				}
			}
		}
	}
	// Mutants keep diff order: by hunk, then line and column.
	sortMutants(mutants)
	for i := range mutants {
		mutants[i].ID = "m" + strconv.Itoa(i+1)
	}
	return mutants
}

func revParse(ctx context.Context, dir, rev string) (string, error) {
	out, err := git(ctx, dir, "rev-parse", "--verify", "--end-of-options", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() > maxGitOutput {
		return nil, fmt.Errorf("git %s output exceeds %d bytes", args[0], maxGitOutput)
	}
	return stdout.Bytes(), nil
}

// checkAppDir requires the app directory to be a worktree whose HEAD is the
// head revision and whose tracked files are unmodified (CCE-V0-172).
func checkAppDir(ctx context.Context, dir, head string) error {
	at, err := revParse(ctx, dir, "HEAD")
	if err != nil {
		return refuse("discriminate-app-dir-invalid", "%v", err)
	}
	if at != head {
		return refuse("discriminate-app-dir-invalid", "%s is at %s, not the head revision %s", dir, at, head)
	}
	status, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return refuse("discriminate-app-dir-invalid", "%v", err)
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return refuse("discriminate-app-dir-invalid", "%s has modified tracked files", dir)
	}
	return nil
}

// digestUnavailable is a ChangesDigest whose diff could not be read; it never
// compares as unchanged.
const digestUnavailable = "unavailable"

func testRevision(ctx context.Context, dir string) TestRevision {
	t := TestRevision{Dir: dir, Unchanged: true}
	if revision, err := revParse(ctx, dir, "HEAD"); err == nil {
		t.Revision = revision
		status, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=no")
		t.Clean = err == nil && len(bytes.TrimSpace(status)) == 0
		if !t.Clean {
			t.ChangesDigest = digestUnavailable
			if diff, err := git(ctx, dir, "diff", "--binary", "--no-color", "--no-ext-diff", "--no-textconv", "HEAD", "--"); err == nil {
				sum := sha256.Sum256(diff)
				t.ChangesDigest = hex.EncodeToString(sum[:])
			}
		}
	}
	return t
}

// changedHunks lists the head-side hunks of application files (CCE-V0-170)
// and every other changed file with the rule that excluded it (CCE-V0-236).
func changedHunks(ctx context.Context, root, base, head string, exclusion Exclusion) ([]Hunk, []ExcludedFile, int, error) {
	names, err := git(ctx, root, "-c", "core.quotePath=false", "diff", "--name-status", "-z", "--no-renames", "--no-ext-diff", base, head, "--")
	if err != nil {
		return nil, nil, 0, refuse("discriminate-diff-unavailable", "%v", err)
	}
	out, err := git(ctx, root, "-c", "core.quotePath=false", "diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--no-textconv",
		"--src-prefix=a/", "--dst-prefix=b/", "--diff-algorithm=myers", "--no-indent-heuristic", base, head, "--")
	if err != nil {
		return nil, nil, 0, refuse("discriminate-diff-unavailable", "%v", err)
	}
	hunks := []Hunk{}
	var current string
	var language Language
	admitted := false
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			current, admitted = "", false
		case strings.HasPrefix(line, "+++ "):
			name := headName(strings.TrimPrefix(line, "+++ "))
			if name == "/dev/null" || !strings.HasPrefix(name, "b/") {
				continue
			}
			current = strings.TrimPrefix(name, "b/")
			var rule string
			language, rule = exclusion.Classify(current)
			admitted = rule == ""
		case strings.HasPrefix(line, "@@ ") && admitted:
			start, count, ok := newRange(line)
			if !ok {
				return nil, nil, 0, refuse("discriminate-diff-unavailable", "unparsed hunk header %q", line)
			}
			if count == 0 {
				continue
			}
			if len(hunks) == maxHunks {
				return nil, nil, 0, refuse("discriminate-bound-invalid", "the change has more than %d application hunks", maxHunks)
			}
			hunks = append(hunks, Hunk{Path: current, Language: language, NewStart: start, NewCount: count})
		}
	}
	withHunks := map[string]bool{}
	for _, h := range hunks {
		withHunks[h.Path] = true
	}
	excluded, omitted := []ExcludedFile{}, 0
	fields := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		status, name := fields[i], fields[i+1]
		if withHunks[name] {
			continue
		}
		_, rule := exclusion.Classify(name)
		switch {
		case strings.HasPrefix(status, "D"):
			rule = ExcludedDeleted
		case rule == "":
			// Pure deletions, a mode-only change, or a path the -U0 parse
			// could not attribute: never a silent drop.
			rule = ExcludedNoAdditions
		}
		if len(excluded) == maxExcluded {
			omitted++
			continue
		}
		excluded = append(excluded, ExcludedFile{Path: name, Rule: rule})
	}
	return hunks, excluded, omitted, nil
}

// headName undoes Git's rendering of a "+++" path: the trailing tab after a
// name with a space, and C-style quoting of a name with a quote, backslash or
// control character (CCE-V0-236).
func headName(name string) string {
	name = strings.TrimSuffix(name, "\t")
	if strings.HasPrefix(name, `"`) {
		if unquoted, err := strconv.Unquote(name); err == nil {
			return unquoted
		}
	}
	return name
}

func newRange(header string) (int, int, bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, false
	}
	startText, countText, hasCount := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
	start, err := strconv.Atoi(startText)
	if err != nil {
		return 0, 0, false
	}
	count := 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil {
			return 0, 0, false
		}
	}
	return start, count, true
}

// Encode is the document's canonical JSON with a trailing newline.
func (d Document) Encode() []byte {
	data, _ := json.MarshalIndent(d, "", "  ")
	return append(data, '\n')
}
