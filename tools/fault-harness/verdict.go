// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

const verdictProfile = "fault-harness-verdict/1"

// releaseSentence is the G3 release wording, copied exactly (review minor 20).
const releaseSentence = "0 misses in %d pre-registered fault-injected cases per runner on pinned repos; 95 percent upper bound on the per-change miss rate 3/%d; not a guarantee."

// Verdict statuses.
const (
	statusZeroMiss     = "ZERO_MISS"
	statusMiss         = "MISS"
	statusInvalid      = "INVALID"
	statusDisqualified = "DISQUALIFIED"
)

type miss struct {
	Case  string `json:"case"`
	Fault string `json:"fault"`
	Test  string `json:"test"`
}

// section is the accounting of one fault population: code faults (reverts and
// mutants) or non-code faults, each with its own N and bound (FIH-V1-002).
type section struct {
	// The four counts of FIH-V1-014; N is NarrowedWithKill.
	Sealed           int `json:"sealed"`
	DroppedNoKill    int `json:"droppedNoKill"`
	FullOrUnknown    int `json:"fullOrUnknown"`
	NarrowedWithKill int `json:"narrowedWithKill"`
	// Cases outside the four counts: excluded with pin-unavailable, and
	// failed measurements.
	Excluded int    `json:"excluded"`
	Errors   int    `json:"errors"`
	Misses   []miss `json:"misses"`
	// MissedCases counts the cases in N with at least one miss; the Wilson
	// bound is per case, like N, so a case with two missed faults counts once.
	MissedCases int    `json:"missedCases"`
	Status      string `json:"status"`
	// Sentence is printed only for a zero-miss verdict.
	Sentence string `json:"sentence,omitempty"`
	Bound    string `json:"bound,omitempty"`
	// WilsonUpper95 is the 95 percent Wilson upper bound on the per-change
	// miss rate, printed beside 3/N (settled release-tasks S6).
	WilsonUpper95 float64 `json:"wilsonUpper95"`
	tiers         []string
}

type verdictDoc struct {
	Profile               string           `json:"profile"`
	Runner                string           `json:"runner"`
	PreregistrationSHA256 string           `json:"preregistrationSha256"`
	FaultSetSHA256        string           `json:"faultSetSha256"`
	Status                string           `json:"status"`
	Reasons               []string         `json:"reasons"`
	Code                  section          `json:"code"`
	NonCode               section          `json:"nonCode"`
	Baselines             []baselineRecord `json:"baselines"`
	OmittedQuarantined    struct {
		Count int           `json:"count"`
		Tests []omittedTest `json:"tests"`
	} `json:"omittedQuarantined"`
	// Granularity is "line-level proven", "file-level" or "not proven"
	// (FIH-V1-015).
	Granularity string `json:"granularity"`
}

// wilson returns the 95 percent Wilson score interval for k events in n.
func wilson(k, n int) (lower, upper float64) {
	if n <= 0 {
		return 0, 1
	}
	const z = 1.959963984540054
	p, nn := float64(k)/float64(n), float64(n)
	denom := 1 + z*z/nn
	center := p + z*z/(2*nn)
	margin := z * math.Sqrt(p*(1-p)/nn+z*z/(4*nn*nn))
	round := func(x float64) float64 { return math.Round(x*1e6) / 1e6 }
	return round(math.Max(0, (center-margin)/denom)), round(math.Min(1, (center+margin)/denom))
}

// account fills a section from its cases. A case is counted in N when one of
// its faults has a confirmed kill under a NARROWED plan; its misses are the
// confirmed kills those plans' selected runs did not observe (FIH-V1-006).
func account(cases []caseResult) section {
	s := section{Misses: []miss{}}
	for _, c := range cases {
		s.Sealed++
		switch c.Status {
		case caseExcluded:
			s.Excluded++
			continue
		case caseError:
			s.Errors++
			continue
		case caseDropped:
			s.DroppedNoKill++
			continue
		}
		narrowed, missed := false, false
		for _, f := range c.Faults {
			if f.Status != faultKilled || f.PlanKind != "NARROWED" {
				continue
			}
			if !narrowed {
				s.tiers = append(s.tiers, f.Tier)
			}
			narrowed = true
			for _, t := range f.Misses {
				s.Misses = append(s.Misses, miss{Case: c.ID, Fault: f.ID, Test: t})
			}
			if len(f.Misses) > 0 && !missed {
				missed = true
				s.MissedCases++
			}
		}
		if narrowed {
			s.NarrowedWithKill++
		} else {
			s.FullOrUnknown++
		}
	}
	_, s.WilsonUpper95 = wilson(s.MissedCases, s.NarrowedWithKill)
	if s.NarrowedWithKill > 0 {
		s.Bound = fmt.Sprintf("3/%d", s.NarrowedWithKill)
	}
	return s
}

// buildVerdict computes the per-runner verdict of FIH-V1-007 from results.
func buildVerdict(res results) verdictDoc {
	v := verdictDoc{Profile: verdictProfile, Runner: res.Runner, PreregistrationSHA256: res.PreregistrationSHA256, FaultSetSHA256: res.FaultSetSHA256, Reasons: []string{}, Baselines: res.Baselines}
	if v.Baselines == nil {
		v.Baselines = []baselineRecord{}
	}
	var code, nonCode []caseResult
	for _, c := range res.Cases {
		if c.Kind == "non-code" {
			nonCode = append(nonCode, c)
		} else {
			code = append(code, c)
		}
	}
	v.Code, v.NonCode = account(code), account(nonCode)
	v.OmittedQuarantined.Tests = omittedQuarantined(res.Baselines, res.Cases)
	v.OmittedQuarantined.Count = len(v.OmittedQuarantined.Tests)
	reason := func(format string, args ...any) { v.Reasons = append(v.Reasons, fmt.Sprintf(format, args...)) }
	disqualified, invalid := false, false
	if len(res.Baselines) == 0 {
		invalid = true
		reason("no baseline was measured")
	}
	for _, b := range res.Baselines {
		if b.Disqualified {
			disqualified = true
			reason("baseline %s: %d of %d tests quarantined, above 2 percent", b.SHA, len(b.Quarantined), b.Suite)
		}
		if b.Invalid {
			invalid = true
			reason("baseline %s: %d of %d tests excluded, above 5 percent", b.SHA, len(b.Quarantined)+len(b.Unrunnable), b.Suite)
		}
	}
	for _, x := range []struct {
		name string
		s    *section
		min  int
	}{{"code", &v.Code, minimumN}, {"non-code", &v.NonCode, 1}} {
		s := x.s
		switch {
		case len(s.Misses) > 0:
			s.Status = statusMiss
			reason("%s: %d misses in %d of N = %d cases; a miss blocks and is never waived as a flake", x.name, len(s.Misses), s.MissedCases, s.NarrowedWithKill)
		case s.NarrowedWithKill < x.min:
			s.Status = statusInvalid
			reason("%s: N = %d is below %d", x.name, s.NarrowedWithKill, x.min)
		default:
			s.Status = statusZeroMiss
		}
		if s.Errors > 0 {
			invalid = true
			reason("%s: %d failed measurements", x.name, s.Errors)
		}
	}
	// FIH-V1-002 and FIH-V1-014 do not say whether an excluded case counts
	// toward the non-code minimum of 30; the verdict fails closed: an
	// excluded case is never measured, so it counts toward neither N nor
	// the minimum, and fewer than 30 measured non-code cases are INVALID.
	if measured := v.NonCode.Sealed - v.NonCode.Excluded; measured < minimumNonCode {
		invalid = true
		reason("non-code: %d of %d sealed cases measured after %d exclusions, below the minimum of %d", measured, v.NonCode.Sealed, v.NonCode.Excluded, minimumNonCode)
	}
	if excluded := v.Code.Excluded + v.NonCode.Excluded; excluded > res.ExclusionBudget {
		invalid = true
		reason("%d cases excluded with pin-unavailable, above the budget of %d", excluded, res.ExclusionBudget)
	}
	switch {
	case disqualified:
		v.Status = statusDisqualified
	case v.Code.Status == statusMiss || v.NonCode.Status == statusMiss:
		v.Status = statusMiss
	case invalid || v.Code.Status == statusInvalid || v.NonCode.Status == statusInvalid:
		v.Status = statusInvalid
	default:
		v.Status = statusZeroMiss
		for _, s := range []*section{&v.Code, &v.NonCode} {
			s.Sentence = fmt.Sprintf(releaseSentence, s.NarrowedWithKill, s.NarrowedWithKill)
		}
	}
	v.Granularity = "not proven"
	if v.Status == statusZeroMiss {
		tiers := append(slices.Clone(v.Code.tiers), v.NonCode.tiers...)
		switch {
		case !slices.ContainsFunc(tiers, func(t string) bool { return t != "line" }):
			v.Granularity = "line-level proven"
		case !slices.ContainsFunc(tiers, func(t string) bool { return t != "line" && t != "file" }):
			v.Granularity = "file-level"
		default:
			reason("a counted plan records no line or file tier, so the runner is not called proven at either granularity")
		}
	}
	return v
}

// printVerdict writes the human-readable verdict.
func printVerdict(w io.Writer, v verdictDoc) {
	fmt.Fprintf(w, "runner %s: %s\n", v.Runner, v.Status)
	for _, x := range []struct {
		name string
		s    section
	}{{"code", v.Code}, {"non-code", v.NonCode}} {
		s := x.s
		fmt.Fprintf(w, "%s: sealed %d, dropped (no kill) %d, plan FULL or UNKNOWN %d, narrowed with a confirmed kill (N) %d", x.name, s.Sealed, s.DroppedNoKill, s.FullOrUnknown, s.NarrowedWithKill)
		if s.Excluded+s.Errors > 0 {
			fmt.Fprintf(w, "; excluded %d, failed measurements %d", s.Excluded, s.Errors)
		}
		fmt.Fprintln(w)
		if s.Sentence != "" {
			fmt.Fprintf(w, "%s: %s (95 percent Wilson upper bound %.2f percent)\n", x.name, s.Sentence, 100*s.WilsonUpper95)
		}
		for _, m := range s.Misses {
			fmt.Fprintf(w, "%s: MISS %s %s %s\n", x.name, m.Case, m.Fault, m.Test)
		}
	}
	var names []string
	for _, t := range v.OmittedQuarantined.Tests {
		names = append(names, t.Test)
	}
	fmt.Fprintf(w, "omitted quarantined tests: %d", v.OmittedQuarantined.Count)
	if len(names) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(names, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "granularity: %s\n", v.Granularity)
	for _, r := range v.Reasons {
		fmt.Fprintf(w, "reason: %s\n", r)
	}
}
