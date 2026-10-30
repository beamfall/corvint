package evalrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// The held-out slot-weight gate (learned-trace-admission-v0, LTA-V0-010). It
// scores a frozen golden's held-out `query` rows through the context packet
// under the default slot order and under each proposal. It reads no ledger
// and writes nothing; admission is the caller's.

// Classification labels, matching LTA-V0-001's vocabulary.
const (
	SlotDeltaImproved         = "improved"
	SlotDeltaRegressed        = "regressed"
	SlotDeltaNotDistinguished = "not distinguished"
	// slotDeltaMinCases is LTA-V0-001's two-case floor: a smaller change is
	// not distinguished.
	slotDeltaMinCases = 2
)

type slotCaseScore struct {
	criticalMisses, mustHit int
	top5Hit                 bool
}

type slotArm struct{ cases []slotCaseScore }

// EvaluateSlotWeights returns the gate report and the index of the proposal
// with the largest improvement, or -1 when none is classified improved.
func EvaluateSlotWeights(ctx context.Context, root, goldenPath string, proposals []contextindex.SlotWeights) (map[string]any, int, error) {
	goldenBytes, cases, err := loadGolden(goldenPath)
	if err != nil {
		return nil, -1, err
	}
	heldout := heldoutContextCases(cases)
	before, err := contextindex.Observe(ctx, root)
	if err != nil {
		return nil, -1, err
	}
	index, err := contextindex.BuildContext(ctx, root, "")
	if err != nil {
		return nil, -1, err
	}
	baseline, err := scoreSlotArm(ctx, index, heldout, nil)
	if err != nil {
		return nil, -1, err
	}
	results := make([]any, 0, len(proposals))
	best, bestNet := -1, 0
	for position, weights := range proposals {
		arm, armErr := scoreSlotArm(ctx, index, heldout, weights)
		if armErr != nil {
			return nil, -1, armErr
		}
		delta := slotDelta(baseline, arm)
		results = append(results, map[string]any{"weights": slotWeightsValue(weights), "arm": arm.report(), "delta": delta})
		net := delta["improved_cases"].(int) - delta["regressed_cases"].(int)
		if delta["classification"] == SlotDeltaImproved && net > bestNet {
			best, bestNet = position, net
		}
	}
	after, err := contextindex.Observe(ctx, root)
	if err != nil {
		return nil, -1, err
	}
	if after.Revision != before.Revision || after.CommitRevision != before.CommitRevision {
		return nil, -1, fmt.Errorf("repository revision changed during slot-weight evaluation")
	}
	digest := sha256.Sum256(goldenBytes)
	report := map[string]any{
		"goldens": goldenPath, "goldens_sha256": "sha256:" + hex.EncodeToString(digest[:]),
		"revision": index.Revision, "split_rule": SplitRule, "split": SplitHeldout,
		"heldout_cases": len(heldout), "baseline": baseline.report(), "proposals": results,
	}
	return report, best, nil
}

// heldoutContextCases keeps the held-out `query` rows: the context packet
// takes task text, so feature and impact rows have no context arm.
func heldoutContextCases(cases []goldenCase) []goldenCase {
	kept := []goldenCase{}
	for _, item := range cases {
		if item.Split == SplitHeldout && item.Mode == "query" && strings.TrimSpace(item.Text) != "" {
			kept = append(kept, item)
		}
	}
	return kept
}

func scoreSlotArm(ctx context.Context, index *contextindex.Index, cases []goldenCase, weights contextindex.SlotWeights) (slotArm, error) {
	var admitted *contextindex.AdmittedSlotWeights
	if weights != nil {
		admitted = &contextindex.AdmittedSlotWeights{Weights: weights}
	}
	arm := slotArm{}
	for _, item := range cases {
		packet, err := contextindex.TaskContextWeighted(ctx, index, item.Text, "", item.Limit, admitted)
		if err != nil {
			return slotArm{}, err
		}
		arm.cases = append(arm.cases, scoreSlotCase(item, packetResultPaths(packet)))
	}
	return arm, nil
}

func packetResultPaths(packet map[string]any) []string {
	paths := []string{}
	results, _ := packet["results"].([]any)
	for _, result := range results {
		row, _ := result.(map[string]any)
		paths = append(paths, stringValue(row["id"]))
	}
	return paths
}

// scoreSlotCase scores only path-bearing selectors: `symbol:PATH:NAME` and
// `file:PATH`. A feature or scenario selector has no context-packet form.
func scoreSlotCase(item goldenCase, paths []string) slotCaseScore {
	included := stringSet(paths)
	topFive := stringSet(paths[:min(5, len(paths))])
	score := slotCaseScore{}
	for _, path := range selectorPaths(item.Expected.Critical) {
		if _, found := included[path]; !found {
			score.criticalMisses++
		}
	}
	for _, path := range selectorPaths(item.Expected.MustInclude) {
		if _, found := included[path]; found {
			score.mustHit++
		}
	}
	relevant := append(append(append([]string{}, item.Expected.Relevant...), item.Expected.MustInclude...), item.Expected.Critical...)
	for _, path := range selectorPaths(relevant) {
		if _, found := topFive[path]; found {
			score.top5Hit = true
		}
	}
	return score
}

func selectorPaths(selectors []string) []string {
	paths := []string{}
	for _, selector := range selectors {
		kind, rest, _ := strings.Cut(selector, ":")
		if kind == "file" && rest != "" {
			paths = append(paths, rest)
		}
		if at := strings.LastIndex(rest, ":"); kind == "symbol" && at > 0 {
			paths = append(paths, rest[:at])
		}
	}
	return paths
}

func (arm slotArm) report() map[string]any {
	critical, must, top := 0, 0, 0
	for _, score := range arm.cases {
		critical += score.criticalMisses
		must += score.mustHit
		if score.top5Hit {
			top++
		}
	}
	return map[string]any{"critical_misses": critical, "must_include_hits": must, "top5_hits": top}
}

// slotDelta compares one proposal with the baseline case by case. A case
// improves on fewer critical misses, then more must-include hits, then a
// gained top-five hit; it regresses symmetrically.
func slotDelta(baseline, proposal slotArm) map[string]any {
	improved, regressed := 0, 0
	for position := range baseline.cases {
		switch compareSlotCase(baseline.cases[position], proposal.cases[position]) {
		case 1:
			improved++
		case -1:
			regressed++
		}
	}
	base, arm := baseline.report(), proposal.report()
	criticalDelta := arm["critical_misses"].(int) - base["critical_misses"].(int)
	classification := SlotDeltaNotDistinguished
	if criticalDelta > 0 || regressed-improved >= slotDeltaMinCases {
		classification = SlotDeltaRegressed
	} else if improved-regressed >= slotDeltaMinCases {
		classification = SlotDeltaImproved
	}
	return map[string]any{
		"critical_misses":   criticalDelta,
		"must_include_hits": arm["must_include_hits"].(int) - base["must_include_hits"].(int),
		"top5_hits":         arm["top5_hits"].(int) - base["top5_hits"].(int),
		"improved_cases":    improved, "regressed_cases": regressed, "classification": classification,
	}
}

func compareSlotCase(base, arm slotCaseScore) int {
	if arm.criticalMisses != base.criticalMisses {
		return slotSign(base.criticalMisses - arm.criticalMisses)
	}
	if arm.mustHit != base.mustHit {
		return slotSign(arm.mustHit - base.mustHit)
	}
	if arm.top5Hit != base.top5Hit {
		if arm.top5Hit {
			return 1
		}
		return -1
	}
	return 0
}

func slotSign(value int) int {
	if value > 0 {
		return 1
	}
	return -1
}

func slotWeightsValue(weights contextindex.SlotWeights) map[string]any {
	value := map[string]any{}
	for relation, weight := range weights {
		value[relation] = weight
	}
	return value
}
