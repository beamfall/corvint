package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

type record struct {
	DescriptorSHA256  string                           `json:"descriptor_sha256"`
	Version           string                           `json:"version"`
	Repetition        int                              `json:"repetition"`
	StartedAt         string                           `json:"started_at"`
	WallMS            float64                          `json:"wall_ms"`
	Exit              any                              `json:"exit"`
	StdoutSHA256      string                           `json:"stdout_sha256"`
	StderrSHA256      string                           `json:"stderr_sha256"`
	StdoutBytes       int                              `json:"stdout_bytes"`
	StderrBytes       int                              `json:"stderr_bytes"`
	EffectsObserved   []observedEffect                 `json:"effects_observed"`
	InconclusiveState *procgroup.CaseInconclusiveState `json:"inconclusive_state"`
	GOOS              string                           `json:"goos"`
	GOARCH            string                           `json:"goarch"`
}
type caseResult struct {
	ID string `json:"id"`
	procgroup.Adjudication
	Inconclusive    *procgroup.CaseInconclusiveState `json:"case_inconclusive_state"`
	Effects         []observedEffect                 `json:"case_effects_observed"`
	Records         []record                         `json:"records"`
	AcceptedBy      string                           `json:"accepted_by"`
	Detail          string                           `json:"detail,omitempty"`
	OverflowStreams []string                         `json:"overflow_streams"`
}
type report struct {
	Schema        string       `json:"schema"`
	HostProfile   string       `json:"host_profile"`
	Qualification string       `json:"qualification"`
	EffectSurface string       `json:"effect_surface"`
	Cases         []caseResult `json:"cases"`
}

func newReport() report {
	return report{Schema: "compat-replay-runner-v0", HostProfile: "owned-synthetic-process-group", Qualification: "owned process group only; session escape and external execution NOT_PRODUCED", EffectSurface: "net scratch filesystem transitions only; transient and outward-symlink effects unobservable", Cases: []caseResult{}}
}
func statePointer(s procgroup.CaseInconclusiveState) *procgroup.CaseInconclusiveState {
	if s == procgroup.InconclusiveNone {
		return nil
	}
	return &s
}

var requestBudget = 120 * time.Second

func replay(ctx context.Context, p preparedManifest) report {
	result := newReport()
	for _, t := range p.Tasks {
		budget, cancel := context.WithTimeout(ctx, requestBudget)
		result.Cases = append(result.Cases, replayCase(budget, p.SHA256, t))
		cancel()
	}
	return result
}
func replayCase(ctx context.Context, descriptorSHA string, p preparedTask) caseResult {
	result := caseResult{ID: p.Task.ID, AcceptedBy: "NOT_PRODUCED", Effects: []observedEffect{}, Records: []record{}, OverflowStreams: []string{}, Detail: p.Detail}
	refusal := p.Refusal
	if refusal == procgroup.RefusalNone && ctx.Err() != nil {
		refusal = procgroup.RefusalBudgetExpired
	}
	if refusal != procgroup.RefusalNone {
		result.Adjudication = procgroup.Adjudicate(6, refusal, nil)
		result.Inconclusive = statePointer(result.CaseInconclusiveState)
		return result
	}
	observations := []procgroup.Observation{}
	scratchIncomplete := false
	for index := 0; index < 6; index++ {
		if ctx.Err() != nil {
			observations = append(observations, procgroup.Observation{Cancelled: true})
			break
		}
		versionName := "old"
		binary := p.Old
		if index >= 3 {
			versionName = "new"
			binary = p.New
		}
		o, r, detail := repetition(ctx, descriptorSHA, p, binary, versionName, index%3+1)
		observations = append(observations, o)
		if detail != "" {
			result.Detail = detail
		}
		if strings.HasPrefix(detail, "scratch observation incomplete:") {
			scratchIncomplete = true
		}
		result.Effects = append(result.Effects, r.EffectsObserved...)
		if o.ExitObserved {
			result.Records = append(result.Records, r)
		}
		if o.StdoutOverflow {
			result.OverflowStreams = append(result.OverflowStreams, "stdout")
		}
		if o.StderrOverflow {
			result.OverflowStreams = append(result.OverflowStreams, "stderr")
		}
		if o.StdoutOverflow || o.StderrOverflow {
			break
		}
	}
	result.Effects = unionEffects(result.Effects)
	result.Adjudication = procgroup.Adjudicate(6, procgroup.RefusalNone, observations)
	if result.FullyObserved {
		result.Status = procgroup.StatusPass
		result.Outcome = compare(observations)
		if result.Outcome == procgroup.OutcomeInstability {
			result.CaseInconclusiveState = procgroup.InconclusiveInconsistent
			for _, start := range []int{0, 3} {
				if !same(observations[start], observations[start+1]) || !same(observations[start], observations[start+2]) {
					for i := start; i < start+3; i++ {
						result.Records[i].InconclusiveState = statePointer(procgroup.InconclusiveInconsistent)
					}
				}
			}
		}
		for _, e := range result.Effects {
			if e.Basis == "undeclared-observed" {
				result.Status = procgroup.StatusFail
				result.Outcome = procgroup.OutcomeWithheld
			}
		}
	}
	if result.FullyObserved && scratchIncomplete {
		result.Status = procgroup.StatusFail
		result.Outcome = procgroup.OutcomeWithheld
		result.CaseInconclusiveState = procgroup.InconclusiveInconsistent
	}
	result.Inconclusive = statePointer(result.CaseInconclusiveState)
	// The case's winning cause is never projected onto unaffected repetitions.
	// Lower-priority causes cannot contradict the envelope either.
	for i := range result.Records {
		r := &result.Records[i]
		if r.InconclusiveState != nil && *r.InconclusiveState != result.CaseInconclusiveState {
			r.InconclusiveState = nil
		}
	}
	return result
}
func repetition(ctx context.Context, sha string, p preparedTask, binary []byte, v string, n int) (procgroup.Observation, record, string) {
	r := record{DescriptorSHA256: sha, Version: v, Repetition: n, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, EffectsObserved: []observedEffect{}}
	fail := func(err error) (procgroup.Observation, record, string) {
		return procgroup.Observation{}, r, err.Error()
	}
	root, err := os.MkdirTemp("", "corvint-compat-")
	if err != nil {
		return fail(err)
	}
	defer removeScratch(root)
	scratch := filepath.Join(root, "scratch")
	if err = os.Mkdir(scratch, 0700); err != nil {
		return fail(err)
	}
	if err = materialize(p.Snapshot, scratch); err != nil {
		return fail(err)
	}
	executable := filepath.Join(root, "fixture")
	mode := p.OldMode
	if v == "new" {
		mode = p.NewMode
	}
	if err = os.WriteFile(executable, binary, mode); err != nil {
		return fail(err)
	}
	before, err := treeState(scratch)
	if err != nil {
		return fail(err)
	}
	started := time.Now()
	r.StartedAt = started.UTC().Format(time.RFC3339Nano)
	argv := append([]string{executable}, p.Task.Argv...)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: filepath.Join(scratch, p.Task.Cwd), Env: p.Task.Env, Stdin: p.Input, Timeout: 10 * time.Second, ShutdownTimeout: time.Second, InputLimit: inputLimit, OutputLimit: outputLimit})
	r.WallMS = float64(time.Since(started)) / float64(time.Millisecond)
	r.Exit = o.ExitStatus
	if o.Signal != "" {
		r.Exit = "signal:" + o.Signal
	}
	r.StdoutSHA256 = digest(o.Stdout)
	r.StderrSHA256 = digest(o.Stderr)
	r.StdoutBytes = len(o.Stdout)
	r.StderrBytes = len(o.Stderr)
	detail := ""
	after, err := treeState(scratch)
	if err != nil {
		detail = "scratch observation incomplete: " + err.Error()
	} else {
		r.EffectsObserved = diffEffects(before, after, p.Task.ExpectedEffects)
	}
	switch {
	case o.StdoutOverflow || o.StderrOverflow:
		r.InconclusiveState = statePointer(procgroup.InconclusiveResource)
	case o.TimedOut || o.Cancelled:
		r.InconclusiveState = statePointer(procgroup.InconclusiveTimeout)
	case !o.Started:
		r.InconclusiveState = statePointer(procgroup.InconclusiveSetup)
	case !o.ExitObserved || !o.PipesDrained || !o.OwnedProcessGroupCleanup:
		r.InconclusiveState = statePointer(procgroup.InconclusiveInconsistent)
	}
	if o.Err != nil {
		detail = errors.Join(o.Err, err).Error()
	}
	return o, r, detail
}
func same(a, b procgroup.Observation) bool {
	return a.ExitStatus == b.ExitStatus && a.Signal == b.Signal && bytes.Equal(a.Stdout, b.Stdout) && bytes.Equal(a.Stderr, b.Stderr)
}
func compare(o []procgroup.Observation) procgroup.Outcome {
	for _, start := range []int{0, 3} {
		if !same(o[start], o[start+1]) || !same(o[start], o[start+2]) {
			return procgroup.OutcomeInstability
		}
	}
	for _, a := range o[:3] {
		for _, b := range o[3:] {
			if !same(a, b) {
				return procgroup.OutcomeDifferent
			}
		}
	}
	return procgroup.OutcomeCompatible
}
