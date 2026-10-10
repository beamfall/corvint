package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The experimental batch runner and final qualification (TOL-V0-028..033,
// GitHub #714): `run-batch` runs a ticket's named specs once with a
// configured command, credits the ledger from the report and writes a
// step-level summary; `qualify` runs the static checks, the candidate specs
// N times at retry 0, the post-check and the changed directories, re-runs
// failing neighbours at the base commit and writes a verdict pack.

const (
	runConfigSchema    = "corvint-tasks-run-config/0"
	batchSummarySchema = "corvint-tasks-batch-summary/0"
	verdictSchema      = "corvint-tasks-qualify-verdict/0"
	maxRunConfigBytes  = 64 << 10
	maxRunRuns         = 20
	maxRunTimeout      = 3600
	maxSummaryBytes    = 4 << 20
	maxPriorRuns       = 1024
	// repeatFailureDetail prefixes the TOL-V0-031 LOOP_DETECTED refusal.
	repeatFailureDetail = "OBLIGATION_REPEAT_FAILURE:"
	// notQualifiedDetail prefixes the TOL-V0-032 GATE_FAILED refusal.
	notQualifiedDetail = "QUALIFY_NOT_QUALIFIED:"
	// laneFailedDetail prefixes a run-batch refusal whose lane was
	// interrupted or could not be released (TOL-V0-030, 033).
	laneFailedDetail = "LANE_FAILED:"
)

// Neighbour policies (TOL-V0-028).
const (
	neighboursNone    = "NONE"
	neighboursChanged = "CHANGED_DIRECTORIES"
)

// NOT_QUALIFIED reasons (TOL-V0-032).
const (
	reasonStatic    = "STATIC_CHECK_FAILED"
	reasonRetry     = "RETRY"
	reasonCandidate = "CANDIDATE_FAILURE"
	reasonPostCheck = "POST_CHECK_FAILED"
	reasonNeighbour = "NEW_NEIGHBOUR_FAILURE"
	reasonLane      = "LANE_FAILED"
)

// jobInterruptContext and jobLeaseCommand are seams that tests replace to
// interrupt a job or refuse its lane release deterministically.
var (
	jobInterruptContext = interruptContext
	jobLeaseCommand     = leaseCommand
)

var runRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,47}$`)

// runConfig is the closed local configuration file (TOL-V0-028).
type runConfig struct {
	Schema         string     `json:"schema"`
	TestCommand    []string   `json:"testCommand"`
	PrepCommand    []string   `json:"prepCommand"`
	PostCheck      []string   `json:"postCheck"`
	StaticChecks   [][]string `json:"staticChecks"`
	CaptureCommand []string   `json:"captureCommand"`
	Runs           int        `json:"runs"`
	Neighbours     string     `json:"neighbours"`
	FixturePaths   []string   `json:"fixturePaths"`
	Env            []string   `json:"env"`
	TimeoutSeconds int        `json:"timeoutSeconds"`
}

func configErr(f string, args ...any) error {
	return wire.Errorf(wire.CodeMalformed, "--config", f, args...)
}

// readRunConfig reads and checks the configuration file; it returns the
// config and the digest of its bytes.
func readRunConfig(file string) (*runConfig, wire.Digest, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, "", configErr("cannot read the configuration: %v", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxRunConfigBytes+1))
	if err != nil {
		return nil, "", configErr("cannot read the configuration: %v", err)
	}
	if len(raw) > maxRunConfigBytes {
		return nil, "", wire.Errorf(wire.CodeLimitExceeded, "--config", "the configuration exceeds %d bytes", maxRunConfigBytes)
	}
	var c runConfig
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, "", configErr("the configuration is not one closed %s object: %v", runConfigSchema, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, "", configErr("the configuration holds more than one JSON value")
	}
	if c.Schema != runConfigSchema {
		return nil, "", configErr("schema must be %s", runConfigSchema)
	}
	argvs := map[string][]string{"testCommand": c.TestCommand, "prepCommand": c.PrepCommand, "postCheck": c.PostCheck, "captureCommand": c.CaptureCommand}
	for i, s := range c.StaticChecks {
		argvs["staticChecks/"+string(wire.CountOf(int64(i)))] = s
		if len(s) == 0 {
			return nil, "", configErr("staticChecks/%d is empty", i)
		}
	}
	if len(c.TestCommand) == 0 {
		return nil, "", configErr("testCommand is required")
	}
	for name, argv := range argvs {
		for _, a := range argv {
			if a == "" || strings.ContainsRune(a, 0) {
				return nil, "", configErr("%s holds an empty or NUL argument", name)
			}
		}
	}
	if c.Runs < 1 || c.Runs > maxRunRuns {
		return nil, "", configErr("runs must be 1..%d", maxRunRuns)
	}
	if c.Neighbours != neighboursNone && c.Neighbours != neighboursChanged {
		return nil, "", configErr("neighbours must be %s or %s", neighboursNone, neighboursChanged)
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > maxRunTimeout {
		return nil, "", configErr("timeoutSeconds must be 1..%d", maxRunTimeout)
	}
	for _, p := range c.FixturePaths {
		if !repoRelative(p) {
			return nil, "", configErr("fixturePaths take repository-relative paths")
		}
	}
	for _, n := range c.Env {
		if n == "" || strings.ContainsAny(n, "=\x00") {
			return nil, "", configErr("env takes variable names")
		}
	}
	return &c, wire.Sum(raw), nil
}

func repoRelative(p string) bool {
	return p != "" && !path.IsAbs(p) && !strings.HasPrefix(path.Clean(p), "..") && !strings.ContainsAny(p, "\\\n\r\x00")
}

// runJob is the state one run-batch or qualify job shares: the checkout it
// runs in, its results directory and its optional lane.
type runJob struct {
	env                 Env
	cfg                 *runConfig
	root, commit, dir   string
	requestID           string
	pool                string
	attempt, generation string
	member, allocation  string
	steps               []runStep
}

// runStep is one step of the job as the summary and verdict record it.
type runStep struct {
	Step     string `json:"step"`
	Class    string `json:"class"`
	ExitCode string `json:"exitCode"`
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Log      string `json:"log"`
}

func (j *runJob) record(step string, run store.BatchCommandRun, log string) runStep {
	s := runStep{Step: step, Class: run.Class, ExitCode: run.ExitCode, OK: run.Passed(), Log: log}
	if !s.OK {
		s.Error = obligation.FirstActionableLine(string(run.Output))
	}
	j.steps = append(j.steps, s)
	return s
}

// commonRunArgs are the flags run-batch and qualify share.
type commonRunArgs struct {
	config, commit, out, requestID, pool, attempt, generation string
	specs                                                     []string
}

func (a *commonRunArgs) singles(m map[string]*string) map[string]*string {
	for k, v := range map[string]*string{"--config": &a.config, "--commit": &a.commit, "--out": &a.out, "--request-id": &a.requestID,
		"--pool": &a.pool, "--attempt": &a.attempt, "--generation": &a.generation} {
		m[k] = v
	}
	return m
}

func (a *commonRunArgs) check(cmd []string) *wire.Result {
	if a.config == "" || a.out == "" || a.requestID == "" || len(a.specs) == 0 {
		return usage(cmd, "--config FILE, --out DIR, --request-id ID and at least one --spec PATH are required")
	}
	if !runRequestID.MatchString(a.requestID) {
		return usage(cmd, "--request-id names the results directory: 1..48 bytes of [A-Za-z0-9._-], starting with a letter or digit")
	}
	if (a.attempt == "") != (a.generation == "") {
		return usage(cmd, "--attempt and --generation are given together or not at all")
	}
	if a.pool != "" && a.attempt == "" {
		return usage(cmd, "--pool needs the claimed --attempt and --generation")
	}
	for _, s := range a.specs {
		if !repoRelative(s) {
			return usage(cmd, "--spec takes repository-relative paths without line breaks")
		}
	}
	return nil
}

// prepareJob reads the config, checks the checkout is a clean checkout of
// the commit, and resolves the spec files there. It creates nothing.
func prepareJob(env Env, a *commonRunArgs) (*runJob, wire.Digest, []string, error) {
	cfg, cfgSum, err := readRunConfig(a.config)
	if err != nil {
		return nil, "", nil, err
	}
	root, err := store.WorktreeRoot(env.Cwd)
	if err != nil {
		return nil, "", nil, err
	}
	commit, err := store.ResolveCommit(root, a.commit)
	if err != nil {
		return nil, "", nil, wire.Errorf(wire.CodeMissingEvidence, "--commit", "the repository holds no commit %s", prose(a.commit))
	}
	head, changed, err := store.WorktreeState(root)
	if err != nil {
		return nil, "", nil, err
	}
	if head != commit {
		return nil, "", nil, wire.Errorf(wire.CodeStaleTree, "--commit", "the checkout is at %s, not %s; the job runs only the checked-out commit", head, commit)
	}
	if changed != "" {
		return nil, "", nil, wire.Errorf(wire.CodeDirtyWorktree, "--commit", "the checkout differs from %s (%s); commit or discard the changes first", commit, prose(changed))
	}
	files, err := store.TreePathsAtCommit(root, commit, a.specs, obligation.SpecFile)
	if err != nil {
		return nil, "", nil, err
	}
	if len(files) == 0 {
		return nil, "", nil, wire.Errorf(wire.CodeMissingEvidence, "--spec", "no spec file lies under the named --spec paths at %s", commit)
	}
	out, err := filepath.Abs(a.out)
	if err != nil {
		return nil, "", nil, wire.Errorf(wire.CodeMalformed, "--out", "cannot resolve the results directory: %v", err)
	}
	j := &runJob{env: env, cfg: cfg, root: root, commit: commit, dir: filepath.Join(out, a.requestID), requestID: a.requestID,
		pool: a.pool, attempt: a.attempt, generation: a.generation}
	return j, cfgSum, files, nil
}

// unchanged refuses when the checkout no longer holds exactly the commit.
func (j *runJob) unchanged() error {
	head, changed, err := store.WorktreeState(j.root)
	if err != nil {
		return err
	}
	if head != j.commit {
		return wire.Errorf(wire.CodeStaleTree, "--commit", "the checkout moved from %s to %s", j.commit, head)
	}
	if changed != "" {
		return wire.Errorf(wire.CodeDirtyWorktree, "--commit", "the checkout differs from %s (%s)", j.commit, prose(changed))
	}
	return nil
}

// open creates the job's results directory; an existing one refuses, so a
// job never overwrites an earlier run's evidence.
func (j *runJob) open() error {
	if err := os.MkdirAll(filepath.Dir(j.dir), 0o755); err != nil {
		return wire.Errorf(wire.CodeMalformed, "--out", "cannot create the results directory: %v", err)
	}
	if err := os.Mkdir(j.dir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "%s already holds request %s; a job never overwrites earlier evidence, so use a new request id", filepath.Dir(j.dir), j.requestID)
		}
		return wire.Errorf(wire.CodeMalformed, "--out", "cannot create the results directory: %v", err)
	}
	return nil
}

// acquire takes a pool member for the claimed attempt with the existing
// pool acquire verb (and its health preparation), when a pool is named.
func (j *runJob) acquire() error {
	if j.pool == "" {
		return nil
	}
	res := jobLeaseCommand(j.env, "pool acquire", []string{"--attempt", j.attempt, "--generation", j.generation, "--pool", j.pool, "--request-id", j.requestID + ".acquire"})
	if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		j.steps = append(j.steps, runStep{Step: "acquire", Class: "REFUSED", Error: strings.Join(append(append([]string{}, res.Codes...), res.Warnings...), "; ")})
		code := wire.CodeResourceCollision
		if len(res.Codes) > 0 {
			code = res.Codes[0]
		}
		return wire.Errorf(code, "--pool", "pool acquire was refused: %s", prose(strings.Join(res.Warnings, "; ")))
	}
	alloc, _ := res.Items[0].Obj.Get("poolAllocation")
	if alloc.Kind == wire.KindObject {
		id, _ := alloc.Obj.Get("allocationId")
		m, _ := alloc.Obj.Get("memberId")
		j.allocation, j.member = id.Str, m.Str
	}
	j.steps = append(j.steps, runStep{Step: "acquire", Class: "OK", OK: true})
	return nil
}

// release captures the lane's server-side errors before the lane is reset,
// then returns the allocation with the existing pool release verb. Both run
// on a fresh context, not the job's, so an interrupt that stopped the tests
// still leaves the capture and the release; the capture is bounded by its
// own timeout (TOL-V0-030, 033). A refused release is returned with its code.
func (j *runJob) release() error {
	if len(j.cfg.CaptureCommand) > 0 {
		run := store.RunBatchCommand(context.Background(), j.cfg.CaptureCommand, j.root, j.environment(""), j.cfg.TimeoutSeconds)
		j.writeLog("capture.log", run.Output)
		j.record("capture", run, "capture.log")
	}
	if j.allocation == "" {
		return nil
	}
	res := jobLeaseCommand(j.env, "pool release", []string{"--attempt", j.attempt, "--generation", j.generation, "--allocation", j.allocation, "--request-id", j.requestID + ".release"})
	s := runStep{Step: "release", Class: "OK", OK: res.Outcome == wire.OutcomeOK}
	if s.OK {
		j.steps = append(j.steps, s)
		return nil
	}
	s.Class, s.Error = "REFUSED", strings.Join(append(append([]string{}, res.Codes...), res.Warnings...), "; ")
	j.steps = append(j.steps, s)
	return wire.Errorf(firstNonEmpty(append(append([]string{}, res.Codes...), wire.CodeUnsupported)...), "--pool",
		"%s pool release was refused for allocation %s: %s", laneFailedDetail, j.allocation, prose(s.Error))
}

// environment is the configured environment plus the runner's variables.
func (j *runJob) environment(report string) []string {
	extra := []string{"CORVINT_RUN_DIR=" + j.dir, "CORVINT_POOL_ID=" + j.pool, "CORVINT_POOL_MEMBER=" + j.member}
	if report != "" {
		extra = append(extra, "PLAYWRIGHT_JSON_OUTPUT_NAME="+report)
	}
	return store.BatchEnvironment(j.cfg.Env, extra...)
}

func (j *runJob) writeLog(name string, b []byte) {
	p := filepath.Join(j.dir, filepath.FromSlash(name))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, b, 0o644)
}

// runCommand runs one configured command in dir and records it.
func (j *runJob) runCommand(ctx context.Context, step string, argv []string, dir, logName string) runStep {
	run := store.RunBatchCommand(ctx, argv, dir, j.environment(""), j.cfg.TimeoutSeconds)
	j.writeLog(logName, run.Output)
	return j.record(step, run, logName)
}

// runTests runs the test command once on specs in dir; the report lands at
// <name>/report.json and the output at <name>/output.log. rep is nil when
// the run produced no admissible report, and errLine says why.
func (j *runJob) runTests(ctx context.Context, name, dir string, specs []string) (*obligation.Report, runStep, string) {
	report := filepath.Join(j.dir, filepath.FromSlash(name), "report.json")
	_ = os.MkdirAll(filepath.Dir(report), 0o755)
	run := store.RunBatchCommand(ctx, append(append([]string{}, j.cfg.TestCommand...), specs...), dir, j.environment(report), j.cfg.TimeoutSeconds)
	j.writeLog(name+"/output.log", run.Output)
	s := j.record(name, run, name+"/output.log")
	rep, err := obligation.ReadReport(report, obligationQualifiedVersions())
	if err != nil {
		return nil, s, prose(err.Error())
	}
	if resolved, err := filepath.EvalSymlinks(rep.RootDir); err == nil {
		rep.RootDir = resolved
	}
	return rep, s, ""
}

// manifest writes manifest.json: every file under the results directory
// with its sha256 and size, sorted by path.
func (j *runJob) manifest() (wire.Digest, error) {
	type entry struct {
		Path   string `json:"path"`
		Sha256 string `json:"sha256"`
		Size   int64  `json:"size"`
	}
	entries := []entry{}
	err := filepath.WalkDir(j.dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(j.dir, p)
		if rel == "manifest.json" {
			return nil
		}
		// Stream the hash: an artifact can be far larger than memory.
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		n, err := io.Copy(h, f)
		if err != nil {
			return err
		}
		entries = append(entries, entry{Path: filepath.ToSlash(rel), Sha256: hex.EncodeToString(h.Sum(nil)), Size: n})
		return nil
	})
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "--out", "cannot read the results directory: %v", err)
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Path < entries[b].Path })
	return j.writeJSON("manifest.json", map[string]any{"files": entries})
}

func (j *runJob) writeJSON(name string, v any) (wire.Digest, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "--out", "cannot encode %s: %v", name, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(j.dir, name), raw, 0o644); err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "--out", "cannot write %s: %v", name, err)
	}
	return wire.Sum(raw), nil
}

func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
}

// batchSummary is the TOL-V0-029 summary.json.
type batchSummary struct {
	Schema        string             `json:"schema"`
	RequestID     string             `json:"requestId"`
	TicketID      string             `json:"ticketId"`
	Commit        string             `json:"commit"`
	FixtureDigest string             `json:"fixtureDigest"`
	ConfigSha256  string             `json:"configSha256"`
	ReportSha256  string             `json:"reportSha256"`
	Pool          string             `json:"pool"`
	Member        string             `json:"member"`
	Steps         []runStep          `json:"steps"`
	Witness       batchWitness       `json:"witness"`
	Obligations   []obligation.Cause `json:"obligations"`
}

type batchWitness struct {
	Outcome string   `json:"outcome"`
	Written bool     `json:"written"`
	Codes   []string `json:"codes"`
	Detail  string   `json:"detail"`
}

func runBatch(env Env, cmd []string, args []string) *wire.Result {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	target := args[0]
	a := commonRunArgs{commit: "HEAD"}
	role, idsArg := "OWNER", ""
	if res := pairFlags(cmd, args[1:], a.singles(map[string]*string{"--role": &role, "--ids": &idsArg}),
		map[string]*[]string{"--spec": &a.specs}, nil); res != nil {
		return res
	}
	if res := a.check(cmd); res != nil {
		return res
	}
	if role == "WORKER" && a.attempt == "" {
		return usage(cmd, "--role WORKER witnesses under its own --attempt and --generation")
	}
	var ids []string
	if idsArg != "" {
		var err error
		if ids, err = obligationIDList(idsArg); err != nil {
			return errorResult(cmd, err)
		}
	}
	var rec *ticket.Record
	var ledger *ticket.ObligationLedger
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		r, l, ferr := obligationTarget(rc, target)
		if ferr != nil {
			return ferr
		}
		if l == nil {
			return wire.Errorf(wire.CodeMalformed, "--target", "%s ticket %s has no obligation ledger; seed it first", ticket.ObligationUnknownDetail, r.TicketID.Raw)
		}
		rec, ledger = r, l
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	j, cfgSum, files, err := prepareJob(env, &a)
	if err != nil {
		return errorResult(cmd, err)
	}
	fixture, err := store.FixtureDigest(j.root, j.commit, append(append([]string{}, j.cfg.FixturePaths...), a.specs...))
	if err != nil {
		return errorResult(cmd, err)
	}
	// TOL-V0-031: a repeated failure refuses before any lane is acquired.
	if err := repeatFailures(filepath.Dir(j.dir), rec.TicketID.Raw, string(fixture), ledger, ids); err != nil {
		return errorResult(cmd, err)
	}
	_, present, content, err := store.FilesAtCommit(j.root, j.commit, files)
	if err != nil {
		return errorResult(cmd, err)
	}
	sources := map[string][]byte{}
	for _, p := range files {
		if b, ok := content[p]; ok && present[p] {
			sources[p] = b
		}
	}
	if err := j.open(); err != nil {
		return errorResult(cmd, err)
	}
	ctx, stop := jobInterruptContext()
	defer stop()
	summary := batchSummary{Schema: batchSummarySchema, RequestID: j.requestID, TicketID: rec.TicketID.Raw, Commit: j.commit,
		FixtureDigest: string(fixture), ConfigSha256: string(cfgSum), Pool: j.pool, Obligations: []obligation.Cause{}}
	var rep *obligation.Report
	var reportErr, earlier string
	// failCode is the job's refusal code when nothing is witnessed: the lane's
	// own code for a refused acquire or release (TOL-V0-030), else GATE_FAILED.
	failCode := wire.CodeGateFailed
	notReached := obligation.CauseNotReached
	postLog, postStatus := "", ""
	if err := j.acquire(); err != nil {
		earlier, failCode = "acquire: "+prose(err.Error()), wire.CodeOf(err)
	} else {
		if len(j.cfg.PrepCommand) > 0 {
			if s := j.runCommand(ctx, "prep", j.cfg.PrepCommand, j.root, "prep.log"); !s.OK {
				earlier = "prep: " + s.Error
			}
		}
		if earlier == "" {
			var s runStep
			rep, s, reportErr = j.runTests(ctx, "test", j.root, files)
			if rep == nil {
				earlier = "test: " + firstNonEmpty(reportErr, s.Error)
			}
		}
		if earlier == "" && len(j.cfg.PostCheck) > 0 {
			s := j.runCommand(ctx, "postCheck", j.cfg.PostCheck, j.root, "post-check.log")
			postLog, postStatus = filepath.Join(j.dir, "post-check.log"), s.ExitCode
			if s.Class != "EXIT" {
				postStatus = "1" // a post-check that did not exit failed
			}
		}
		if err := j.release(); err != nil {
			// A lane that could not be returned fails the job: nothing is
			// witnessed and the release refusal is the job's result.
			earlier, rep, notReached, failCode = prose(err.Error()), nil, obligation.CauseUncredited, wire.CodeOf(err)
		}
	}
	summary.Member = j.member
	// A report from an interrupted job or a checkout that changed under the
	// run credits nothing.
	if ctx.Err() != nil && failCode == wire.CodeGateFailed {
		earlier, rep, notReached = laneFailedDetail+" interrupted", nil, obligation.CauseUncredited
	} else if rep != nil {
		if err := j.unchanged(); err != nil {
			earlier, rep, notReached = "the checkout changed during the run: "+prose(err.Error()), nil, obligation.CauseUncredited
		}
	}
	result := success(cmd, rc)
	if rep == nil {
		// Nothing ran to an admissible report, so every obligation in scope
		// was not reached because an earlier step failed.
		for _, e := range ledger.Entries {
			c := obligation.Cause{ID: e.ID, State: e.State, Cause: notReached, Error: earlier}
			switch {
			case ids != nil && !slices.Contains(ids, e.ID):
				c.Cause, c.Error = obligation.CauseOutOfScope, ""
			case e.State == ticket.ObligationWitnessed:
				c.Cause, c.Error = obligation.CauseAlreadyWitnessed, ""
			case e.State == ticket.ObligationDeferred:
				c.Cause, c.Error = obligation.CauseDeferred, ""
			}
			summary.Obligations = append(summary.Obligations, c)
		}
		summary.Witness = batchWitness{Outcome: "NOT_RUN", Codes: []string{}, Detail: earlier}
		result.Outcome, result.Codes = wire.OutcomeRefused, []string{failCode}
		result.Warnings = append(result.Warnings, prose("the batch did not reach a report: "+earlier))
	} else {
		summary.ReportSha256 = string(rep.Sha256)
		wargs := []string{target, "--request-id", j.requestID, "--expected-revision", string(rec.Revision), "--commit", j.commit,
			"--from-playwright-report", filepath.Join(j.dir, "test", "report.json"), "--role", role}
		if idsArg != "" {
			wargs = append(wargs, "--ids", idsArg)
		}
		if postLog != "" {
			wargs = append(wargs, "--post-check", postLog, "--post-check-status", postStatus)
		}
		if role == "WORKER" {
			wargs = append(wargs, "--attempt", j.attempt, "--generation", j.generation)
		}
		// The job context is checked again just before the witness mutation is
		// submitted; once submitted, the mutation is atomic (TOL-V0-033).
		w := obligationsWitnessJob(ctx, env, []string{"ticket", "obligations", "witness"}, append([]string{"--target"}, wargs...))
		written := false
		if w.Outcome == wire.OutcomeOK && len(w.Items) == 1 {
			v, _ := w.Items[0].Obj.Get("written")
			written = v.Kind == wire.KindBool && v.Bool
		}
		summary.Witness = batchWitness{Outcome: w.Outcome, Written: written, Codes: append([]string{}, w.Codes...), Detail: strings.Join(w.Warnings, "; ")}
		refusal := ""
		if w.Outcome != wire.OutcomeOK {
			refusal = strings.Join(append(append([]string{}, w.Codes...), w.Warnings...), ": ")
		}
		res, causes, err := classifyBatch(j, rep, ledger, sources, ids, written, refusal)
		if err != nil {
			return errorResult(cmd, err)
		}
		_ = res
		summary.Obligations = causes
		if w.Outcome != wire.OutcomeOK {
			result.Outcome, result.Codes = w.Outcome, append([]string{}, w.Codes...)
			result.Warnings = append(result.Warnings, w.Warnings...)
		}
		result.Mutation = w.Mutation
	}
	summary.Steps = j.steps
	sum, err := j.writeJSON("summary.json", summary)
	if err != nil {
		return errorResult(cmd, err)
	}
	result.NotRetryable = true
	result.Untrusted = true
	result.Items = []wire.Value{wire.ObjectValue(batchItem(summary, filepath.Join(j.dir, "summary.json"), sum))}
	return result
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

// classifyBatch recomputes the witness classification from the ledger
// before the write and assigns each obligation its TOL-V0-029 cause.
func classifyBatch(j *runJob, rep *obligation.Report, l *ticket.ObligationLedger, sources map[string][]byte, ids []string, written bool, refusal string) (*obligation.Result, []obligation.Cause, error) {
	paths := rep.SourcePaths(l.Prefix, j.root)
	_, present, content, err := store.FilesAtCommit(j.root, j.commit, paths)
	if err != nil {
		return nil, nil, err
	}
	res, err := obligation.Classify(rep, l, j.root, obligation.Source{Present: present, Content: content}, ids)
	if err != nil {
		return nil, nil, err
	}
	return res, obligation.BatchCauses(rep, l, res, j.root, obligation.NamedTests(sources, l.Prefix), ids, written, refusal), nil
}

func batchItem(s batchSummary, file string, sum wire.Digest) *wire.Object {
	counts := map[string]int{}
	list := make([]wire.Value, 0, len(s.Obligations))
	for _, c := range s.Obligations {
		counts[c.Cause]++
		o := wire.NewObject().Set("id", wire.String(c.ID)).Set("state", wire.String(c.State)).Set("cause", wire.String(c.Cause))
		o.Set("error", optionalProse(c.Error)).Set("test", optionalProse(c.Test))
		list = append(list, wire.ObjectValue(o))
	}
	names := make([]string, 0, len(counts))
	for k := range counts {
		names = append(names, k)
	}
	sort.Strings(names)
	co := wire.NewObject()
	for _, k := range names {
		co.Set(k, wire.String(string(wire.CountOf(int64(counts[k])))))
	}
	steps := make([]wire.Value, 0, len(s.Steps))
	for _, st := range s.Steps {
		steps = append(steps, wire.ObjectValue(wire.NewObject().Set("step", wire.String(st.Step)).Set("class", wire.String(st.Class)).
			Set("ok", wire.Bool(st.OK)).Set("error", optionalProse(st.Error))))
	}
	return wire.NewObject().Set("ticketId", wire.String(s.TicketID)).Set("commit", wire.String(s.Commit)).
		Set("summary", wire.String(prose(file))).Set("summarySha256", wire.String(string(sum))).
		Set("fixtureDigest", wire.String(s.FixtureDigest)).Set("written", wire.Bool(s.Witness.Written)).
		Set("steps", wire.Array(steps...)).Set("causes", wire.ObjectValue(co)).Set("obligations", wire.Array(list...))
}

func optionalProse(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(prose(s))
}

// repeatFailures refuses LOOP_DETECTED when at least two earlier batch
// summaries of this ticket in the results directory, at the same fixture
// digest, record one in-scope OPEN, DEFECT or BLOCKED obligation as FAILED
// with the same error line (TOL-V0-031).
func repeatFailures(out, ticketID, fixture string, l *ticket.ObligationLedger, ids []string) error {
	entries, err := os.ReadDir(out)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return wire.Errorf(wire.CodeMalformed, "--out", "cannot read the results directory: %v", err)
	}
	if len(entries) > maxPriorRuns {
		return wire.Errorf(wire.CodeLimitExceeded, "--out", "the results directory holds more than %d entries; archive earlier runs", maxPriorRuns)
	}
	type key struct{ id, err string }
	counts := map[key][]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		file := filepath.Join(out, e.Name(), "summary.json")
		info, err := os.Lstat(file)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxSummaryBytes {
			return wire.Errorf(wire.CodeLimitExceeded, "--out", "%s exceeds %d bytes", file, maxSummaryBytes)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return wire.Errorf(wire.CodeMalformed, "--out", "cannot read %s: %v", file, err)
		}
		var s batchSummary
		if err := json.Unmarshal(raw, &s); err != nil || s.Schema != batchSummarySchema {
			return wire.Errorf(wire.CodeMalformed, "--out", "%s is not a %s document", file, batchSummarySchema)
		}
		if s.TicketID != ticketID || s.FixtureDigest != fixture {
			continue
		}
		for _, c := range s.Obligations {
			if c.Cause == obligation.CauseFailed {
				counts[key{c.ID, c.Error}] = append(counts[key{c.ID, c.Error}], s.RequestID)
			}
		}
	}
	var repeated []string
	for k, runs := range counts {
		e := l.Entry(k.id)
		if len(runs) < 2 || e == nil || (ids != nil && !slices.Contains(ids, k.id)) {
			continue
		}
		if e.State != ticket.ObligationOpen && e.State != ticket.ObligationDefect && e.State != ticket.ObligationBlocked {
			continue
		}
		sort.Strings(runs)
		repeated = append(repeated, k.id+" ("+strings.Join(runs, ", ")+": "+firstNonEmpty(k.err, "no error line")+")")
	}
	if len(repeated) == 0 {
		return nil
	}
	sort.Strings(repeated)
	return wire.Errorf(wire.CodeLoopDetected, "--ids", "%s %s failed twice with the same error at the same fixture evidence; change the fixtures, defer the obligation, or leave it out with --ids", repeatFailureDetail, prose(strings.Join(repeated, "; ")))
}
