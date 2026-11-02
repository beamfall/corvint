package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedVerdictProfile names the `affected --summary` document
// (AFP-V0-872).
const affectedVerdictProfile = "affected-summary/1"

// affectedVerdict is the few-line verdict `affected --summary` writes in
// place of the affected-plan/1 document (AFP-V0-872): the plan's summary, its
// per-tier counts with the tier-0 and tier-1 unit names a lane runs locally,
// and any degradation or warning. It restates the compact receipt, so it
// never changes the selection; the plan itself stays one command away.
type affectedVerdict struct {
	Degradations []string             `json:"degradations,omitempty"`
	Detail       string               `json:"detail"`
	Mutates      bool                 `json:"mutates"`
	OK           bool                 `json:"ok"`
	Profile      string               `json:"profile"`
	Range        affectedVerdictRange `json:"range"`
	Revision     string               `json:"revision"`
	Summary      affectedSummary      `json:"summary"`
	Tiers        affectedVerdictTiers `json:"tiers"`
	Tool         string               `json:"tool"`
	Warning      string               `json:"warning,omitempty"`
}

// affectedVerdictRange is the plan range without its path list, which can be
// as long as the branch: the base and how many paths the range holds.
type affectedVerdictRange struct {
	Base  string `json:"base"`
	Paths int    `json:"paths"`
}

// affectedVerdictTiers is the AFP-V0-162 ranking reduced to its counts and
// the names of the tier-0 and tier-1 units, each sorted.
type affectedVerdictTiers struct {
	Counts  [3]int   `json:"counts"`
	Summary string   `json:"summary"`
	Tier0   []string `json:"tier0"`
	Tier1   []string `json:"tier1"`
}

// affectedVerdictDetail tells a reader where the plan's detail is.
const affectedVerdictDetail = "rerun without --summary for the affected-plan/1 plan (witnesses, unknown details, advice), or with --full for every exclusion and test file"

func affectedVerdictOf(receipt affectedCompactReceipt) affectedVerdict {
	tiers := affectedVerdictTiers{Summary: fmt.Sprintf(affectedTierSummary, 0, 0, 0), Tier0: []string{}, Tier1: []string{}}
	if receipt.Tiers != nil {
		tiers.Counts = receipt.Tiers.Counts
		tiers.Summary = receipt.Tiers.Summary
		for unit, tier := range receipt.Tiers.Units {
			switch tier {
			case 0:
				tiers.Tier0 = append(tiers.Tier0, unit)
			case 1:
				tiers.Tier1 = append(tiers.Tier1, unit)
			}
		}
		sort.Strings(tiers.Tier0)
		sort.Strings(tiers.Tier1)
	}
	return affectedVerdict{
		Degradations: receipt.Degradations,
		Detail:       affectedVerdictDetail,
		Mutates:      receipt.Mutates,
		OK:           receipt.OK,
		Profile:      affectedVerdictProfile,
		Range:        affectedVerdictRange{Base: receipt.Range.Base, Paths: len(receipt.Range.Paths)},
		Revision:     receipt.Revision,
		Summary:      receipt.Summary,
		Tiers:        tiers,
		Tool:         receipt.Tool,
		Warning:      receipt.Warning,
	}
}

// renderAffectedVerdictText is the `--summary --format text` form: the
// AFP-V0-928 header, degraded and warning lines, the tier counts, then one
// line per tier-0 and tier-1 unit.
func renderAffectedVerdictText(verdict affectedVerdict) string {
	var out strings.Builder
	fmt.Fprintf(&out, "affected %s revision=%s", verdict.Summary.Line, verdict.Revision)
	if verdict.Range.Base != "" {
		fmt.Fprintf(&out, " base=%s", verdict.Range.Base)
	}
	out.WriteString("\n")
	for _, degradation := range verdict.Degradations {
		fmt.Fprintf(&out, "degraded %s\n", textWord(degradation))
	}
	if verdict.Warning != "" {
		fmt.Fprintf(&out, "warning %s\n", textWord(verdict.Warning))
	}
	fmt.Fprintf(&out, "tiers %s\n", verdict.Tiers.Summary)
	for _, unit := range verdict.Tiers.Tier0 {
		fmt.Fprintf(&out, "tier0 %s\n", textWord(unit))
	}
	for _, unit := range verdict.Tiers.Tier1 {
		fmt.Fprintf(&out, "tier1 %s\n", textWord(unit))
	}
	return out.String()
}

// writeAffectedVerdict writes the --summary document for a compact receipt.
func writeAffectedVerdict(receipt any, format string, stdout, stderr io.Writer) int {
	compact, ok := receipt.(affectedCompactReceipt)
	if !ok {
		emitError(stderr, argumentError("--summary has no verdict for this receipt"))
		return 2
	}
	verdict := affectedVerdictOf(compact)
	var err error
	if format == "text" {
		_, err = io.WriteString(stdout, renderAffectedVerdictText(verdict))
	} else {
		err = gokernel.WriteCanonicalJSON(stdout, verdict)
	}
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected summary"})
		return 2
	}
	return 0
}
