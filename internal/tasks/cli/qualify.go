package cli

import (
	"context"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Neighbour verdicts (TOL-V0-032).
const (
	neighbourPassed      = "PASSED"
	neighbourPreExisting = "PRE_EXISTING"
)

// qualifyRun is one candidate run of the verdict pack.
type qualifyRun struct {
	Run     int      `json:"run"`
	Report  string   `json:"report"`
	Step    runStep  `json:"step"`
	Tests   int      `json:"tests"`
	Retried []string `json:"retried"`
	Failed  []string `json:"failed"`
	Errors  int      `json:"errors"`
}

// neighbourResult is one neighbour test that failed on the candidate, with
// what the base commit showed for it.
type neighbourResult struct {
	Test    string `json:"test"`
	Error   string `json:"error"`
	Base    string `json:"base"` // FAILED, PASSED or NOT_OBSERVED
	Verdict string `json:"verdict"`
}

// verdictPack is the TOL-V0-032 verdict.json.
type verdictPack struct {
	Schema         string            `json:"schema"`
	RequestID      string            `json:"requestId"`
	Commit         string            `json:"commit"`
	Base           string            `json:"base"`
	ConfigSha256   string            `json:"configSha256"`
	Pool           string            `json:"pool"`
	Member         string            `json:"member"`
	Verdict        string            `json:"verdict"`
	Reasons        []string          `json:"reasons"`
	Specs          []string          `json:"specs"`
	Steps          []runStep         `json:"steps"`
	Runs           []qualifyRun      `json:"runs"`
	NeighbourSpecs []string          `json:"neighbourSpecs"`
	Neighbours     []neighbourResult `json:"neighbours"`
}

func qualify(env Env, cmd []string, args []string) *wire.Result {
	a := commonRunArgs{commit: "HEAD"}
	baseArg := ""
	if res := pairFlags(cmd, args, a.singles(map[string]*string{"--base": &baseArg}), map[string]*[]string{"--spec": &a.specs}, nil); res != nil {
		return res
	}
	if res := a.check(cmd); res != nil {
		return res
	}
	if baseArg == "" {
		return usage(cmd, "--base REV names the commit the candidate is compared with")
	}
	j, cfgSum, files, err := prepareJob(env, &a)
	if err != nil {
		return errorResult(cmd, err)
	}
	base, err := store.ResolveCommit(j.root, baseArg)
	if err != nil {
		return errorResult(cmd, wire.Errorf(wire.CodeMissingEvidence, "--base", "the repository holds no commit %s", prose(baseArg)))
	}
	if err := j.open(); err != nil {
		return errorResult(cmd, err)
	}
	ctx, stop := interruptContext()
	defer stop()
	v := verdictPack{Schema: verdictSchema, RequestID: j.requestID, Commit: j.commit, Base: base, ConfigSha256: string(cfgSum),
		Pool: j.pool, Reasons: []string{}, Specs: files, Runs: []qualifyRun{}, NeighbourSpecs: []string{}, Neighbours: []neighbourResult{}}
	fail := func(reason, detail string) { v.Reasons = append(v.Reasons, reason+": "+detail) }
	// Static checks run first and need no lane.
	for i, argv := range j.cfg.StaticChecks {
		name := "static/" + strconv.Itoa(i)
		if s := j.runCommand(ctx, name, argv, j.root, name+".log"); !s.OK {
			fail(reasonStatic, name+": "+firstNonEmpty(s.Error, s.Class+" "+s.ExitCode))
			break
		}
	}
	if len(v.Reasons) == 0 {
		if err := j.acquire(); err != nil {
			fail(reasonLane, "acquire: "+prose(err.Error()))
		} else {
			j.qualifyLane(ctx, &v, files, base, fail)
			j.release(ctx)
		}
	}
	v.Member = j.member
	if ctx.Err() != nil {
		fail(reasonLane, "interrupted")
	} else if err := j.unchanged(); err != nil {
		fail(reasonLane, "the checkout changed during the run: "+prose(err.Error()))
	}
	v.Verdict = "QUALIFIED"
	if len(v.Reasons) > 0 {
		v.Verdict = "NOT_QUALIFIED"
	}
	v.Steps = j.steps
	vsum, err := j.writeJSON("verdict.json", v)
	if err != nil {
		return errorResult(cmd, err)
	}
	msum, err := j.manifest()
	if err != nil {
		return errorResult(cmd, err)
	}
	result := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Untrusted: true, NotRetryable: true}
	if v.Verdict != "QUALIFIED" {
		result.Outcome, result.Codes = wire.OutcomeRefused, []string{wire.CodeGateFailed}
		result.Warnings = append(result.Warnings, prose(notQualifiedDetail+" "+strings.Join(v.Reasons, "; ")))
	}
	reasons := make([]wire.Value, 0, len(v.Reasons))
	for _, r := range v.Reasons {
		reasons = append(reasons, wire.String(prose(r)))
	}
	item := wire.NewObject().Set("commit", wire.String(v.Commit)).Set("base", wire.String(v.Base)).Set("verdict", wire.String(v.Verdict)).
		Set("reasons", wire.Array(reasons...)).Set("runs", wire.String(strconv.Itoa(len(v.Runs)))).
		Set("verdictFile", wire.String(prose(filepath.Join(j.dir, "verdict.json")))).Set("verdictSha256", wire.String(string(vsum))).
		Set("manifestSha256", wire.String(string(msum)))
	result.Items = []wire.Value{wire.ObjectValue(item)}
	return result
}

// qualifyLane runs, on the lane, the prep command, the candidate runs, the
// post-check and the neighbour comparison, stopping at the first reason.
func (j *runJob) qualifyLane(ctx context.Context, v *verdictPack, files []string, base string, fail func(string, string)) {
	if len(j.cfg.PrepCommand) > 0 {
		if s := j.runCommand(ctx, "prep", j.cfg.PrepCommand, j.root, "prep.log"); !s.OK {
			fail(reasonLane, "prep: "+s.Error)
			return
		}
	}
	for i := 1; i <= j.cfg.Runs && ctx.Err() == nil; i++ {
		name := "runs/" + strconv.Itoa(i)
		rep, s, errLine := j.runTests(ctx, name, j.root, files)
		r := qualifyRun{Run: i, Report: name + "/report.json", Step: s, Retried: []string{}, Failed: []string{}}
		if rep == nil {
			v.Runs = append(v.Runs, r)
			fail(reasonCandidate, "run "+strconv.Itoa(i)+" produced no admissible report: "+firstNonEmpty(errLine, s.Error))
			return
		}
		outs := rep.Outcomes()
		r.Tests, r.Errors = len(outs), rep.Errors()
		for _, o := range outs {
			label := testLabel(rep.RepoPath(j.root, o.File), o)
			if o.MaxRetry > 0 {
				r.Retried = append(r.Retried, label)
			}
			if !o.OK() {
				r.Failed = append(r.Failed, label+firstNonEmpty(errSuffix(o.Error), " ("+firstNonEmpty(o.Status, "no retry-0 result")+")"))
			}
		}
		v.Runs = append(v.Runs, r)
		switch {
		case len(r.Retried) > 0:
			fail(reasonRetry, "run "+strconv.Itoa(i)+" retried "+strings.Join(r.Retried, ", "))
		case len(r.Failed) > 0:
			fail(reasonCandidate, "run "+strconv.Itoa(i)+" failed "+strings.Join(r.Failed, ", "))
		case r.Errors > 0:
			fail(reasonCandidate, "run "+strconv.Itoa(i)+" reported "+strconv.Itoa(r.Errors)+" top-level errors")
		case r.Tests == 0:
			fail(reasonCandidate, "run "+strconv.Itoa(i)+" ran no tests")
		case !s.OK:
			fail(reasonCandidate, "run "+strconv.Itoa(i)+" exited "+s.Class+" "+s.ExitCode)
		}
		if len(v.Reasons) > 0 {
			return
		}
	}
	if ctx.Err() != nil {
		return
	}
	if len(j.cfg.PostCheck) > 0 {
		if s := j.runCommand(ctx, "postCheck", j.cfg.PostCheck, j.root, "post-check.log"); !s.OK {
			fail(reasonPostCheck, firstNonEmpty(s.Error, s.Class+" "+s.ExitCode))
			return
		}
	}
	if j.cfg.Neighbours == neighboursChanged {
		j.neighbours(ctx, v, files, base, fail)
	}
}

func testLabel(file string, o obligation.TestOutcome) string {
	l := file + " > " + strings.Join(o.TitlePath, " > ")
	if o.Project != "" {
		l += " [" + o.Project + "]"
	}
	return l
}

func errSuffix(e string) string {
	if e == "" {
		return ""
	}
	return " (" + e + ")"
}

// neighbours runs the spec files under the changed directories once, then
// re-runs the failing ones at the base commit in a temporary worktree. A
// failure the base shares is PRE_EXISTING; any other, including one the
// base run could not observe, is NEW_NEIGHBOUR_FAILURE.
func (j *runJob) neighbours(ctx context.Context, v *verdictPack, files []string, base string, fail func(string, string)) {
	changed, err := store.ChangedPaths(j.root, base, j.commit)
	if err != nil {
		fail(reasonNeighbour, "the changed paths are not observable: "+prose(err.Error()))
		return
	}
	var dirs []string
	for _, p := range changed {
		// A change at the repository root names no directory; it would
		// otherwise select every spec in the repository.
		if d := path.Dir(p); d != "." && !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return
	}
	specs, err := store.TreePathsAtCommit(j.root, j.commit, dirs, obligation.SpecFile)
	if err != nil {
		fail(reasonNeighbour, "the neighbour specs are not observable: "+prose(err.Error()))
		return
	}
	specs = slices.DeleteFunc(specs, func(s string) bool { return slices.Contains(files, s) })
	v.NeighbourSpecs = specs
	if len(specs) == 0 {
		return
	}
	rep, s, errLine := j.runTests(ctx, "neighbours/candidate", j.root, specs)
	if rep == nil {
		fail(reasonNeighbour, "the neighbour run produced no admissible report: "+firstNonEmpty(errLine, s.Error))
		return
	}
	type failure struct{ spec, label, err string }
	failing := map[string]failure{}
	var failingSpecs []string
	for _, o := range rep.Outcomes() {
		if o.OK() {
			continue
		}
		spec := rep.RepoPath(j.root, o.File)
		failing[spec+"\x00"+strings.Join(o.TitlePath, "\x00")+"\x00"+o.Project] = failure{spec, testLabel(spec, o), o.Error}
		if !slices.Contains(failingSpecs, spec) {
			failingSpecs = append(failingSpecs, spec)
		}
	}
	if len(failing) == 0 {
		if rep.Errors() > 0 || !s.OK {
			fail(reasonNeighbour, "the neighbour run exited "+s.Class+" "+s.ExitCode+" with "+strconv.Itoa(rep.Errors())+" top-level errors")
		}
		return
	}
	sort.Strings(failingSpecs)
	// The base observation: test key to "FAILED" or "PASSED"; nil when the
	// base run produced no admissible report.
	var atBase map[string]string
	werr := store.WithDetachedWorktree(j.root, base, func(wt string) error {
		if len(j.cfg.PrepCommand) > 0 {
			if s := j.runCommand(ctx, "neighbours/base-prep", j.cfg.PrepCommand, wt, "neighbours/base-prep.log"); !s.OK {
				return nil
			}
		}
		brep, _, _ := j.runTests(ctx, "neighbours/base", wt, failingSpecs)
		if brep == nil {
			return nil
		}
		atBase = map[string]string{}
		for _, o := range brep.Outcomes() {
			key := brep.RepoPath(wt, o.File) + "\x00" + strings.Join(o.TitlePath, "\x00") + "\x00" + o.Project
			atBase[key] = neighbourPassed
			if !o.OK() {
				atBase[key] = "FAILED"
			}
		}
		return nil
	})
	if werr != nil {
		fail(reasonNeighbour, prose(werr.Error()))
	}
	keys := make([]string, 0, len(failing))
	for k := range failing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var news []string
	for _, k := range keys {
		f := failing[k]
		n := neighbourResult{Test: f.label, Error: f.err, Base: "NOT_OBSERVED", Verdict: reasonNeighbour}
		if b, ok := atBase[k]; ok {
			n.Base = b
		}
		if n.Base == "FAILED" {
			n.Verdict = neighbourPreExisting
		} else {
			news = append(news, f.label+" (base "+n.Base+")")
		}
		v.Neighbours = append(v.Neighbours, n)
	}
	if len(news) > 0 {
		fail(reasonNeighbour, strings.Join(news, ", "))
	}
}
