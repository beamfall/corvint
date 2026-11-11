package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
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

	// DegradationDetails is the compact receipt's notice per code (ACX-V0-750).
	DegradationDetails map[string]string `json:"degradationDetails,omitempty"`

	// E2E names the plan's E2E selections when the plan carries plan.e2e
	// (APK-V0-9970, V1-5254); absent otherwise, so the verdict is unchanged.
	E2E *affectedVerdictE2E `json:"e2e,omitempty"`
}

// affectedVerdictE2E is plan.e2e reduced to what a lane runs: the scope, the
// inventory count, each selected file, the distinct flow and variation pairs
// that selected files, and the distinct unknown reasons (APK-V0-9970). Test
// keys, changed paths, targets and unknown details stay in the plan.
type affectedVerdictE2E struct {
	Scope      string                   `json:"scope"`
	Inventory  int                      `json:"inventory"`
	Selected   []affectedVerdictE2EFile `json:"selected"`
	Variations []affectedVerdictFlow    `json:"variations"`
	Unknown    []string                 `json:"unknown,omitempty"`
}

type affectedVerdictE2EFile struct {
	Path    string `json:"path"`
	Project string `json:"project,omitempty"`
}

type affectedVerdictFlow struct {
	Flow      string `json:"flow"`
	Variation string `json:"variation"`
}

// affectedVerdictE2EOf reduces plan.e2e for the verdict (APK-V0-9970).
func affectedVerdictE2EOf(plan *affected.E2EPlan) *affectedVerdictE2E {
	if plan == nil {
		return nil
	}
	out := &affectedVerdictE2E{Scope: plan.Scope, Inventory: plan.Inventory.Count,
		Selected: []affectedVerdictE2EFile{}, Variations: []affectedVerdictFlow{}}
	seenFlow, seenReason := map[affectedVerdictFlow]bool{}, map[string]bool{}
	for _, entry := range plan.Selected {
		out.Selected = append(out.Selected, affectedVerdictE2EFile{Path: entry.Path, Project: entry.Project})
		for _, hit := range entry.Flows {
			pair := affectedVerdictFlow{Flow: hit.Flow, Variation: hit.Variation}
			if !seenFlow[pair] {
				seenFlow[pair] = true
				out.Variations = append(out.Variations, pair)
			}
		}
	}
	for _, unknown := range plan.Unknown {
		if !seenReason[unknown.Reason] {
			seenReason[unknown.Reason] = true
			out.Unknown = append(out.Unknown, unknown.Reason)
		}
	}
	sort.Slice(out.Variations, func(i, j int) bool {
		if out.Variations[i].Flow != out.Variations[j].Flow {
			return out.Variations[i].Flow < out.Variations[j].Flow
		}
		return out.Variations[i].Variation < out.Variations[j].Variation
	})
	sort.Strings(out.Unknown)
	return out
}

// affectedVerdictRange is the plan range without its path list, which can be
// as long as the branch: the base and how many paths the range holds.
type affectedVerdictRange struct {
	Base  string `json:"base"`
	Paths int    `json:"paths"`
}

// affectedVerdictTiers is the AFP-V0-162 ranking reduced to its counts and
// the names of the tier-0 and tier-1 units, each sorted. TestIDs maps each
// of those units that names a canonical test ID to it (CTI-V1-009), derived
// as the corvint-test-run/0 receipt derives its testId, so the plan, the
// receipt and the MCP result name a selection by the same string; it is
// absent when no listed unit names one.
type affectedVerdictTiers struct {
	Counts  [3]int            `json:"counts"`
	Summary string            `json:"summary"`
	TestIDs map[string]string `json:"testIds,omitempty"`
	Tier0   []string          `json:"tier0"`
	Tier1   []string          `json:"tier1"`
}

// affectedVerdictDetail tells a reader where the plan's detail is.
const affectedVerdictDetail = "rerun without --summary for the affected-plan/1 plan (witnesses, unknown details, advice), or with --full for every exclusion and test file"

func affectedVerdictOf(receipt affectedCompactReceipt, goDir func(string) (string, bool)) affectedVerdict {
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
		for _, unit := range append(append([]string{}, tiers.Tier0...), tiers.Tier1...) {
			if id := sessiontest.UnitTestID(unit, goDir); id != "" {
				if tiers.TestIDs == nil {
					tiers.TestIDs = map[string]string{}
				}
				tiers.TestIDs[unit] = id
			}
		}
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

		DegradationDetails: receipt.DegradationDetails,
		E2E:                affectedVerdictE2EOf(receipt.Plan.E2E),
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
	writeAffectedDegradations(&out, verdict.Degradations, verdict.DegradationDetails)
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
	if e2e := verdict.E2E; e2e != nil {
		// APK-V0-9970: the E2E selections after the tier lines.
		fmt.Fprintf(&out, "e2e-scope %s inventory=%d selected=%d\n", textWord(e2e.Scope), e2e.Inventory, len(e2e.Selected))
		for _, pair := range e2e.Variations {
			fmt.Fprintf(&out, "e2e-variation %s %s\n", textWord(pair.Flow), textWord(pair.Variation))
		}
		for _, file := range e2e.Selected {
			fmt.Fprintf(&out, "e2e %s", textWord(file.Path))
			if file.Project != "" {
				fmt.Fprintf(&out, " project %s", textWord(file.Project))
			}
			out.WriteString("\n")
		}
		for _, reason := range e2e.Unknown {
			fmt.Fprintf(&out, "e2e-unknown %s\n", textWord(reason))
		}
	}
	return out.String()
}

// writeAffectedVerdict writes the --summary document for a compact receipt;
// root's go.mod resolves the canonical test IDs of its Go units.
func writeAffectedVerdict(receipt any, root, format string, stdout, stderr io.Writer) int {
	compact, ok := receipt.(affectedCompactReceipt)
	if !ok {
		emitError(stderr, argumentError("--summary has no verdict for this receipt"))
		return 2
	}
	verdict := affectedVerdictOf(compact, sessiontest.GoUnitDir(root))
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
