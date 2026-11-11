// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"slices"
)

const (
	faultSetProfile = "fault-harness-fault-set/1"
	sealProfile     = "fault-harness-seal/1"
	// Minimums of FIH-V1-002.
	minimumNonCode      = 30
	minimumFlowReaching = 20
	minMutantsPerPR     = 3
	maxMutantsPerPR     = 5
)

// faultSet is the per-runner set that is sealed, then executed.
type faultSet struct {
	Profile               string        `json:"profile"`
	PreregistrationSHA256 string        `json:"preregistrationSha256"`
	Runner                string        `json:"runner"`
	Cases                 []harnessCase `json:"cases"`
}

// sealRecord is written when a set is sealed; execution refuses a set or
// pre-registration whose hash differs from it.
type sealRecord struct {
	Profile               string          `json:"profile"`
	Runner                string          `json:"runner"`
	PreregistrationSHA256 string          `json:"preregistrationSha256"`
	RuleSHA256            string          `json:"ruleSha256"`
	FaultSetSHA256        string          `json:"faultSetSha256"`
	CodeCases             int             `json:"codeCases"`
	RevertCases           int             `json:"revertCases"`
	MutantCases           int             `json:"mutantCases"`
	NonCodeCases          int             `json:"nonCodeCases"`
	FlowReachingFaults    int             `json:"flowReachingFaults"`
	Reverts               revertSelection `json:"reverts"`
}

// seal checks a fault set against its pre-registration and the reverted-fix
// selection and returns every refusal it finds; the set is sealed only when
// there is none.
func seal(p loadedPreregistration, sel revertSelection, set faultSet, setSHA string) (sealRecord, []error) {
	rec := sealRecord{Profile: sealProfile, Runner: p.Runner, PreregistrationSHA256: p.SHA256, RuleSHA256: sel.RuleSHA256, FaultSetSHA256: setSHA, Reverts: sel}
	var errs []error
	add := func(code, format string, args ...any) { errs = append(errs, refuse(code, format, args...)) }
	if set.Profile != faultSetProfile {
		add("fault-set-invalid", "profile %q, want %s", set.Profile, faultSetProfile)
	}
	if set.PreregistrationSHA256 != p.SHA256 {
		add("preregistration-changed", "the set was built for pre-registration %s, not %s", set.PreregistrationSHA256, p.SHA256)
	}
	if set.Runner != p.Runner {
		add("fault-set-invalid", "set runner %q, pre-registration runner %q", set.Runner, p.Runner)
	}
	if sel.RuleSHA256 != p.RevertRule.SHA256 {
		add("revert-rule-changed", "the selection used rule %s, the pre-registration holds %s", sel.RuleSHA256, p.RevertRule.SHA256)
	}
	operators := p.Operators[p.Language]
	caseIDs, faultIDs := map[string]bool{}, map[string]bool{}
	var reverts []int
	fills := 0
	for _, c := range set.Cases {
		if c.ID == "" || caseIDs[c.ID] {
			add("fault-set-invalid", "empty or repeated case id %q", c.ID)
		}
		caseIDs[c.ID] = true
		if !hexSHA1.MatchString(c.BaseSHA) || !hexSHA1.MatchString(c.HeadSHA) {
			add("fault-set-invalid", "%s: base and head need full commit SHAs", c.ID)
		}
		for _, f := range c.Faults {
			if f.ID == "" || faultIDs[f.ID] {
				add("fault-set-invalid", "%s: empty or repeated fault id %q", c.ID, f.ID)
			}
			faultIDs[f.ID] = true
			if f.FlowReaching {
				rec.FlowReachingFaults++
			}
		}
		switch c.Kind {
		case "revert":
			rec.RevertCases++
			reverts = append(reverts, c.PR)
			if len(c.Faults) != 1 || c.Faults[0].Operator != "revert" || c.Faults[0].Revert == nil {
				add("fault-set-invalid", "%s: a revert case holds exactly one revert fault", c.ID)
			}
		case "mutants":
			rec.MutantCases++
			if c.FillsShortfall {
				fills++
			}
			if n := len(c.Faults); n < minMutantsPerPR || n > maxMutantsPerPR {
				add("mutants-per-pr", "%s: %d mutants, want %d to %d per PR", c.ID, n, minMutantsPerPR, maxMutantsPerPR)
			}
			for _, f := range c.Faults {
				if !slices.Contains(operators, f.Operator) {
					add("operator-not-registered", "%s: operator %q is not in the %s list %v", f.ID, f.Operator, p.Language, operators)
				}
				if f.Revert != nil || validPath(f.Path) != nil || f.Line <= 0 || f.Before == "" {
					add("fault-set-invalid", "%s: a mutant needs a path, line and recorded bytes", f.ID)
				}
			}
		case "non-code":
			rec.NonCodeCases++
			if len(c.Faults) != 1 || c.Faults[0].Operator != "non-code" || !slices.Contains(nonCodeCategories, c.Faults[0].Category) {
				add("fault-set-invalid", "%s: a non-code case holds one non-code fault in one of %v", c.ID, nonCodeCategories)
			} else if f := c.Faults[0]; f.Revert != nil || validPath(f.Path) != nil || f.Line <= 0 || f.Before == "" {
				add("fault-set-invalid", "%s: a non-code fault needs a path, line and recorded bytes", f.ID)
			}
		default:
			add("fault-set-invalid", "%s: kind %q is not revert, mutants or non-code", c.ID, c.Kind)
		}
	}
	rec.CodeCases = rec.RevertCases + rec.MutantCases
	if !slices.Equal(reverts, sel.Reverts) {
		add("reverts-not-rule", "revert cases %v differ from the rule's selection %v", reverts, sel.Reverts)
	}
	if fills != sel.Shortfall {
		add("shortfall-not-filled", "the rule selected %d reverts, a shortfall of %d, and %d mutant PRs fill it", len(sel.Reverts), sel.Shortfall, fills)
	}
	if rec.CodeCases < p.OverSample {
		add("over-sample-not-met", "%d code PRs sealed, the pre-registration over-samples %d", rec.CodeCases, p.OverSample)
	}
	if rec.NonCodeCases < minimumNonCode {
		add("non-code-faults-below-minimum", "%d non-code faults against the minimum of %d", rec.NonCodeCases, minimumNonCode)
	}
	if p.Runner == "playwright" && rec.FlowReachingFaults < minimumFlowReaching {
		add("flow-reaching-faults-below-minimum", "%d against the minimum of %d", rec.FlowReachingFaults, minimumFlowReaching)
	}
	return rec, errs
}

// checkSealed refuses to execute a set or pre-registration that is not the
// sealed one.
func checkSealed(p loadedPreregistration, set faultSet, setSHA string, rec sealRecord) error {
	if rec.Profile != sealProfile || rec.PreregistrationSHA256 != p.SHA256 || set.PreregistrationSHA256 != p.SHA256 {
		return refuse("preregistration-changed", "the seal is for pre-registration %s, not %s", rec.PreregistrationSHA256, p.SHA256)
	}
	if rec.FaultSetSHA256 != setSHA {
		return refuse("fault-set-changed", "the fault set has SHA-256 %s, the seal holds %s", setSHA, rec.FaultSetSHA256)
	}
	if rec.Runner != p.Runner || set.Runner != p.Runner {
		return refuse("fault-set-invalid", "runner mismatch")
	}
	return nil
}

// checkResultsSealed refuses results that are not the measurement of the
// sealed set: another set hash, sealed counts that differ from the seal, fewer
// non-code cases than FIH-V1-002 requires, or a case list that is not the
// sealed one, each sealed case exactly once with its kind, PR and head and no
// other case. A results file with a case removed, added or repeated would
// otherwise change N and the misses the verdict counts.
func checkResultsSealed(set faultSet, setSHA string, rec sealRecord, res results) error {
	if res.FaultSetSHA256 != rec.FaultSetSHA256 || res.FaultSetSHA256 != setSHA {
		return refuse("fault-set-changed", "the results were measured on fault set %s, the seal holds %s", res.FaultSetSHA256, rec.FaultSetSHA256)
	}
	var reverts, mutants, nonCode int
	sealed := map[string]harnessCase{}
	for _, c := range set.Cases {
		switch c.Kind {
		case "revert":
			reverts++
		case "mutants":
			mutants++
		case "non-code":
			nonCode++
		}
		sealed[c.ID] = c
	}
	if len(sealed) != len(set.Cases) || reverts != rec.RevertCases || mutants != rec.MutantCases || reverts+mutants != rec.CodeCases ||
		nonCode != rec.NonCodeCases || reverts+mutants+nonCode != len(set.Cases) {
		return refuse("fault-set-invalid", "the set holds %d revert, %d mutant and %d non-code cases; the seal records %d, %d and %d (%d code)",
			reverts, mutants, nonCode, rec.RevertCases, rec.MutantCases, rec.NonCodeCases, rec.CodeCases)
	}
	if nonCode < minimumNonCode {
		return refuse("non-code-faults-below-minimum", "%d non-code faults sealed against the minimum of %d", nonCode, minimumNonCode)
	}
	seen := map[string]bool{}
	for _, r := range res.Cases {
		c, ok := sealed[r.ID]
		switch {
		case !ok:
			return refuse("results-not-sealed", "case %q is not in the sealed set", r.ID)
		case seen[r.ID]:
			return refuse("results-not-sealed", "case %q appears more than once", r.ID)
		case r.Kind != c.Kind || r.PR != c.PR || r.HeadSHA != c.HeadSHA:
			return refuse("results-not-sealed", "case %q is %s PR %d at %s in the results, %s PR %d at %s in the sealed set", r.ID, r.Kind, r.PR, r.HeadSHA, c.Kind, c.PR, c.HeadSHA)
		}
		if err := checkCaseFaults(c, r); err != nil {
			return err
		}
		seen[r.ID] = true
	}
	for _, c := range set.Cases {
		if !seen[c.ID] {
			return refuse("results-not-sealed", "sealed case %q has no result", c.ID)
		}
	}
	return nil
}

// checkCaseFaults refuses a counted or dropped case whose faults are not the
// sealed faults in their sealed order, or whose status disagrees with them, so
// a fault with a miss cannot be removed or its case relabelled as dropped.
func checkCaseFaults(c harnessCase, r caseResult) error {
	switch r.Status {
	case caseExcluded, caseError:
		return nil
	case caseCounted, caseDropped:
	default:
		return refuse("results-not-sealed", "case %q has status %q", r.ID, r.Status)
	}
	if len(r.Faults) != len(c.Faults) {
		return refuse("results-not-sealed", "case %q has %d fault results for %d sealed faults", r.ID, len(r.Faults), len(c.Faults))
	}
	killed := false
	for i, f := range r.Faults {
		if f.ID != c.Faults[i].ID {
			return refuse("results-not-sealed", "case %q: fault result %q where %q was sealed", r.ID, f.ID, c.Faults[i].ID)
		}
		switch f.Status {
		case faultKilled:
			killed = true
		case faultNoKill:
		default:
			return refuse("results-not-sealed", "case %q: fault %q has status %q in a %s case", r.ID, f.ID, f.Status, r.Status)
		}
	}
	if killed != (r.Status == caseCounted) {
		return refuse("results-not-sealed", "case %q has status %s but its faults say otherwise", r.ID, r.Status)
	}
	return nil
}

func formatRefusals(errs []error) string {
	s := ""
	for _, e := range errs {
		s += fmt.Sprintf("refused: %v\n", e)
	}
	return s
}
