// SPDX-License-Identifier: Apache-2.0

// Package runnerpolicy is the shared CI/local consumer of runnerselection.
// All inputs are core-owned, admitted values; this is not a plugin or wire API.
// Production remains shadow-only until the protected rollout admits a runner.
package runnerpolicy

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// SamplingPath contains one owner-set decimal probability in [0,1]. The host
// reads its bytes from the protected base, never from the candidate checkout.
// Absence is unset, not a default or an observed rate. No reader writes it.
const SamplingPath = ".corvint/runner-selection-sample-rate"

type Sampling struct {
	rate   float64
	status string
}

func ParseSampling(data []byte, admitted bool) Sampling {
	if !admitted {
		return Sampling{status: "sampling-not-admitted"}
	}
	if len(data) == 0 {
		return Sampling{status: "sampling-unset"}
	}
	if len(data) > 128 {
		return Sampling{status: "sampling-capped"}
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return Sampling{status: "sampling-unset"}
	}
	for _, c := range text {
		if (c < '0' || c > '9') && c != '.' {
			return Sampling{status: "sampling-malformed"}
		}
	}
	rate, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return Sampling{status: "sampling-malformed"}
	}
	if rate == 0 {
		return Sampling{status: "sampling-zero"}
	}
	return Sampling{rate: rate, status: "owner-set"}
}

func (p Sampling) Status() string {
	if p.status == "" {
		return "sampling-unset"
	}
	return p.status
}
func (p Sampling) Rate() (float64, bool) {
	return p.rate, p.status == "owner-set" || p.status == "sampling-zero"
}

// Sample is deterministic for a pinned comparison and only schedules default
// branch FULL work. It does not run tests, invent a rate, or select a subset.
func (p Sampling) Sample(base, head string, defaultBranch bool) bool {
	if p.status != "owner-set" || !defaultBranch || !commit(base) || !commit(head) {
		return false
	}
	h := sha256.Sum256([]byte(base + "\x00" + head))
	draw := float64(binary.BigEndian.Uint64(h[:8])>>11) / (1 << 53)
	return draw < p.rate
}

type Request struct {
	Input    s.Input
	Sampling Sampling
	// RolloutAdmitted is a protected, latched admission of the first enabled
	// rollout step. Losing qualification later must not restore static narrowing.
	RolloutAdmitted bool
	Governing       map[string]testrunner.RunSet
	// Missing/capped/unverified history must be represented as incomplete.
	Findings         []Finding
	FindingsComplete bool
}

type Decision struct {
	Shadow  s.Plan
	Runs    map[string]testrunner.RunSet
	Reasons []string
	input   s.Input
}

func ResolveCI(req Request) Decision { return resolve(req) }

// ResolveLocal replays CI from the same admitted inputs and unions its governing
// sets. Different local runtime evidence can widen, never remove a CI test.
func ResolveLocal(ci, local Request) Decision {
	out, reference := resolve(local), resolve(ci)
	same := ci.Input.Base == local.Input.Base && ci.Input.Head == local.Input.Head && ci.Input.Repository == local.Input.Repository && reflect.DeepEqual(ci.Input.Changes, local.Input.Changes)
	for runner, want := range reference.Runs {
		have, ok := out.Runs[runner]
		if !same || !ok || (want.Full && !have.Full) || (!have.Full && !containsAll(have.Tests, want.Tests)) {
			out.Runs[runner] = wider(have, want, "local-ci-parity-widened")
			if !same || !ok {
				run := out.Runs[runner]
				run.Full = true
				run.Reason = "local-ci-inputs-unverifiable"
				out.Runs[runner] = run
			}
		}
	}
	for runner, run := range out.Runs {
		if _, ok := reference.Runs[runner]; !ok {
			run.Full = true
			run.Reason = "ci-runner-unavailable"
			out.Runs[runner] = run
		}
	}
	if !same {
		for runner, run := range out.Runs {
			run.Full = true
			run.Reason = "local-ci-inputs-unverifiable"
			out.Runs[runner] = run
		}
	}
	return out
}

func resolve(req Request) Decision {
	// Before rollout, preserve the selector's existing shadow output exactly.
	// In particular absent policy inputs must not alter frozen command goldens.
	if !req.RolloutAdmitted {
		out := Decision{Shadow: s.Select(req.Input), Runs: map[string]testrunner.RunSet{}, input: req.Input}
		for runner, run := range req.Governing {
			run.Tests = slices.Clone(run.Tests)
			out.Runs[runner] = run
		}
		for _, p := range out.Shadow.Runners {
			run, ok := req.Governing[p.Runner]
			if !ok {
				run = testrunner.RunSet{Full: true, Reason: "governing-plan-unavailable"}
			}
			run.Tests = slices.Clone(run.Tests)
			out.Runs[p.Runner] = run
		}
		return out
	}
	historyValid := req.FindingsComplete && len(req.Findings) <= s.MaxInputs
	if historyValid {
		for _, f := range req.Findings {
			if !validFinding(f) {
				historyValid = false
				break
			}
		}
	}
	in := req.Input
	in.Runners = append([]s.Runner(nil), in.Runners...)
	forceFull := map[string]string{}
	for i := range in.Runners {
		r := &in.Runners[i]
		r.Qualification.SamplingEnabled = req.Sampling.status == "owner-set"
		// Disable only the implicated classes; removing one causes the selector's
		// existing class-unqualified path to make any such change FULL.
		r.Qualification.Classes = append([]s.Class(nil), r.Qualification.Classes...)
		if !historyValid {
			forceFull[r.Name] = "miss-history-incomplete"
			continue
		}
		for _, f := range req.Findings {
			if f.Runner != r.Name {
				continue
			}
			if f.Kind == "unknown" {
				forceFull[r.Name] = "miss-observation-unknown:" + f.Reason
				continue
			}
			if f.Kind == "miss" {
				r.Qualification.Classes = slices.DeleteFunc(r.Qualification.Classes, func(c s.Class) bool { return slices.Contains(f.Classes, c) })
			}
		}
	}
	out := Decision{Shadow: s.Select(in), Runs: map[string]testrunner.RunSet{}, input: in}
	if req.Sampling.status != "owner-set" {
		out.Reasons = append(out.Reasons, req.Sampling.Status())
	}
	if !historyValid {
		out.Reasons = append(out.Reasons, "miss-history-incomplete")
	}
	for _, p := range out.Shadow.Runners {
		run := testrunner.RunSet{Full: p.Mode != "selected", Reason: strings.Join(p.Reasons, ","), Tests: []string{}}
		for _, t := range p.Selected {
			run.Tests = append(run.Tests, t.Test)
		}
		if reason := forceFull[p.Runner]; reason != "" {
			run.Full = true
			run.Reason = reason
			for _, test := range p.Excluded {
				run.Tests = append(run.Tests, test.Test)
			}
			slices.Sort(run.Tests)
		}
		if old, ok := out.Runs[p.Runner]; ok {
			run = wider(old, run, "runner-duplicate")
			run.Full = true
		}
		out.Runs[p.Runner] = run
	}
	// RSP-V0-046: only selector-backed runners can narrow after admission.
	// Governing runners outside that closed set still execute, always FULL.
	for runner, run := range req.Governing {
		if _, ok := out.Runs[runner]; !ok {
			run.Full = true
			run.Tests = slices.Clone(run.Tests)
			out.Runs[runner] = run
		}
	}
	// The raw selector plan is always shadow, including after admission. Runs is
	// the separate core execution decision; existing public plan shapes stay fixed.
	return out
}

func wider(a, b testrunner.RunSet, reason string) testrunner.RunSet {
	tests := append(slices.Clone(a.Tests), b.Tests...)
	slices.Sort(tests)
	return testrunner.RunSet{Full: a.Full || b.Full, Tests: slices.Compact(tests), Reason: reason}
}
func containsAll(have, want []string) bool {
	set := map[string]bool{}
	for _, t := range have {
		set[t] = true
	}
	for _, t := range want {
		if !set[t] {
			return false
		}
	}
	return true
}
func commit(v string) bool {
	if len(v) != 40 && len(v) != 64 {
		return false
	}
	for _, c := range v {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func knownClass(c s.Class) bool {
	return slices.Contains([]s.Class{s.BodyEdit, s.DeclarationEdit, s.DataEdit, s.Add, s.DeleteRename, s.ExecTarget, s.RunnerConfig, s.E2EServer}, c)
}
