// Package evalrepo evaluates a frozen Corvint retrieval corpus without mutation.
package evalrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

const (
	schemaVersion      = 1
	maximumCases       = 200
	promotionMinTraces = 50
	promotionMinRatio  = 0.90
)

type goldenCase struct {
	ID   string
	Mode string
	// ModeRepr is the Python repr() of the raw "mode" field, used only for
	// the oracle-matching "unknown golden mode" error text (repr quotes a
	// string but not a bool/None/number).
	ModeRepr    string
	Text        string
	FeatureID   string
	Paths       []string
	Limit       int
	BudgetBytes *int
	// BudgetMalformed is set when the golden row's budget_bytes is present,
	// non-null, and not a JSON integer. Unlike limit, the Python oracle's
	// validate_budget raises a distinct "budget_bytes must be an integer"
	// message for a bad type versus its "budget_bytes must be between %d and
	// %d" for an out-of-range integer, so a malformed value cannot reuse
	// caseLimit's trick of coercing to a sentinel int and letting the verb's
	// own (single-message) refusal stand in: evaluateCase checks this flag
	// itself, after the verb has run with no budget at all, to reach the
	// oracle's exact type-error text in its place.
	BudgetMalformed bool
	Partition       string
	Split           string
	Expected        expectations
}

type expectations struct {
	MustInclude []string
	Critical    []string
	MustExclude []string
	Relevant    []string
	State       any
}

type counters struct {
	mustTotal, mustHit, criticalTotal, criticalHit                  int
	excludeTotal, excludeViolations                                 int
	abstentionTotal, abstentionHit, wideningTotal, wideningHit      int
	epistemicTotal, epistemicHit, top5Total, top5Hit, positiveCases int
	relevantBytes, totalResultBytes, packetBytes                    int
	budgetTotal, budgetHit, budgetOverflows, criticalOverflows      int
	authoritativeResults, advisoryResults                           int
	caseStates                                                      []string
	caseChecks                                                      []bool
	caseScores                                                      []caseScore
	scoredEnvelopes                                                 []scoredEnvelope
	details                                                         []any
}

type caseScore struct {
	id, split                                       string
	criticalMisses, relevantBytes, totalResultBytes int
	criticalTotal, excludeViolations                int
	mustHitBytes                                    []int
	mustHit, mustTotal                              int
	abstentionApplicable, abstentionHit             bool
	epistemicApplicable, epistemicHit               bool
	top5Applicable, top5Hit                         bool
}

type scoredEnvelope struct {
	caseID  string
	encoded []byte
}

// Evaluate returns the Python-compatible eval report for one pinned corpus.
func Evaluate(ctx context.Context, root, goldenPath string, fixturePaths ...string) (map[string]any, error) {
	report, _, err := evaluate(ctx, root, goldenPath, PurposeScore, fixturePaths)
	return report, err
}

// EvaluateComparable returns the Evaluate report over the rows the purpose may
// read, plus the REC-V0 comparability block for the baseline arm. For
// PurposeScore the report is identical to Evaluate's.
func EvaluateComparable(ctx context.Context, root, goldenPath string, purpose Purpose, fixturePaths ...string) (map[string]any, map[string]any, error) {
	report, run, err := evaluate(ctx, root, goldenPath, purpose, fixturePaths)
	if err != nil {
		return nil, nil, err
	}
	return report, run.counts.comparability(purpose, run.withheld), nil
}

type baselineRun struct {
	counts   counters
	withheld int
}

func evaluate(ctx context.Context, root, goldenPath string, purpose Purpose, fixturePaths []string) (map[string]any, baselineRun, error) {
	if len(fixturePaths) > 1 {
		return nil, baselineRun{}, fmt.Errorf("eval accepts at most one learned trace fixture")
	}
	started := time.Now()
	goldenBytes, allCases, err := loadGolden(goldenPath)
	if err != nil {
		return nil, baselineRun{}, err
	}
	cases, withheld, err := casesForPurpose(allCases, purpose)
	if err != nil {
		return nil, baselineRun{}, err
	}
	identity, err := contextindex.Build(ctx, root)
	if err != nil {
		return nil, baselineRun{}, err
	}
	head, revision := identity.CommitRevision, identity.Revision
	canonicalGolden, err := filepath.Abs(filepath.Join(root, "testing/context-retrieval-goldens.json"))
	if err != nil {
		return nil, baselineRun{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(canonicalGolden); resolveErr == nil {
		canonicalGolden = resolved
	}
	resolvedGolden, err := filepath.Abs(goldenPath)
	if err != nil {
		return nil, baselineRun{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(resolvedGolden); resolveErr == nil {
		resolvedGolden = resolved
	}
	if len(fixturePaths) == 1 {
		fixture, fixtureErr := loadTraceFixture(fixturePaths[0], identity, allCases)
		if fixtureErr != nil {
			return nil, baselineRun{}, fixtureErr
		}
		absent, snapshotErr := contextindex.NewQueryTraceSnapshot("absent", nil)
		if snapshotErr != nil {
			return nil, baselineRun{}, snapshotErr
		}
		learned, snapshotErr := contextindex.NewQueryTraceSnapshot("ready", fixture.traces)
		if snapshotErr != nil {
			return nil, baselineRun{}, snapshotErr
		}
		baselineCounts, armErr := evaluateCases(ctx, root, identity, cases, head, revision, &absent, purpose)
		if armErr != nil {
			return nil, baselineRun{}, armErr
		}
		fixtureStarted := time.Now()
		learnedCounts, armErr := evaluateCases(ctx, root, identity, cases, head, revision, &learned, purpose)
		if armErr != nil {
			return nil, baselineRun{}, armErr
		}
		if armErr := requireIdenticalScoredEnvelopes(baselineCounts.scoredEnvelopes, learnedCounts.scoredEnvelopes); armErr != nil {
			return nil, baselineRun{}, armErr
		}
		passed := 0
		for _, record := range fixture.traces {
			if record.Outcome == "passed" {
				passed++
			}
		}
		baseline := baselineCounts.report(resolvedGolden == canonicalGolden, canonicalGolden, goldenBytes, head, revision, "absent", 0, started)
		learnedArm := learnedCounts.report(resolvedGolden == canonicalGolden, canonicalGolden, goldenBytes, head, revision, "ready", passed, fixtureStarted)
		learnedArm["trace_fixture"] = map[string]any{"path": fixture.path, "sha256": fixture.sha256}
		baseline["learned_trace_arm"] = learnedArm
		baseline["learned_trace_delta"] = scoreDeltas(baselineCounts, learnedCounts)
		return baseline, baselineRun{counts: baselineCounts, withheld: withheld}, nil
	}
	counts, err := evaluateCases(ctx, root, identity, cases, head, revision, nil, purpose)
	if err != nil {
		return nil, baselineRun{}, err
	}
	traceIndex, err := contextindex.Build(ctx, root)
	if err != nil {
		return nil, baselineRun{}, err
	}
	if traceIndex.Revision != revision {
		return nil, baselineRun{}, fmt.Errorf("repository revision changed during trace replay evaluation")
	}
	traces, traceState, err := tracerecordrepo.Read(ctx, root, traceIndex)
	if err != nil {
		return nil, baselineRun{}, err
	}
	passedTraces := 0
	for _, trace := range traces {
		if trace.Outcome == "passed" {
			passedTraces++
		}
	}
	if passedTraces != 0 {
		return nil, baselineRun{}, fmt.Errorf("native Go eval trace replay is not implemented")
	}
	result := counts.report(resolvedGolden == canonicalGolden, canonicalGolden, goldenBytes, head, revision, traceState, passedTraces, started)
	return result, baselineRun{counts: counts, withheld: withheld}, nil
}

// evaluateCases scores every case against one shared, already-built index.
// EvalQuery, EvalFeature and EvalImpact only read the index they are given
// (ranking, receipt assembly and the history-learning stage all treat it as
// pinned bytes), so reusing the same *contextindex.Index across cases, and
// across the baseline/learned arms, cannot let one case's evaluation leak
// into another's. What the old per-case contextindex.Build call actually
// guarded against -- the repository moving out from under a long eval run --
// is re-proved per case with contextindex.Observe, which reads the same
// header fields Build derives from Git without compiling an index.
func evaluateCases(ctx context.Context, root string, index *contextindex.Index, cases []goldenCase, head, revision string, snapshot *contextindex.QueryTraceSnapshot, purpose Purpose) (counters, error) {
	if err := requireReadable(purpose, cases); err != nil {
		return counters{}, err
	}
	counts := counters{details: []any{}, caseStates: []string{}, caseChecks: []bool{}, caseScores: []caseScore{}}
	for _, item := range cases {
		observation, observeErr := contextindex.Observe(ctx, root)
		if observeErr != nil {
			return counters{}, observeErr
		}
		if observation.Revision != revision {
			return counters{}, fmt.Errorf("repository revision changed during evaluation")
		}
		receipt, receiptErr := evaluateCase(ctx, index, item, snapshot)
		if receiptErr != nil {
			return counters{}, receiptErr
		}
		if observation.CommitRevision != head || historyTip(receipt, head) != head {
			return counters{}, fmt.Errorf("repository history changed during evaluation")
		}
		if err := counts.addCase(item, receipt); err != nil {
			return counters{}, err
		}
	}
	return counts, nil
}

func loadGolden(path string) ([]byte, []goldenCase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid retrieval golden: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("invalid retrieval golden: %v", err)
	}
	rawCases, ok := payload["cases"].([]any)
	if integer(payload["schemaVersion"], 0) != schemaVersion || !ok {
		return nil, nil, fmt.Errorf("retrieval golden must contain schemaVersion: 1 and cases[]")
	}
	if len(rawCases) > maximumCases {
		return nil, nil, fmt.Errorf("retrieval golden exceeds 200-case bound")
	}
	cases := make([]goldenCase, 0, len(rawCases))
	for _, rawCase := range rawCases {
		object, ok := rawCase.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("retrieval golden cases must be objects")
		}
		item := goldenCase{
			ID: pythonStrDefault(object, "id", ""), Mode: pythonStr(object["mode"]),
			ModeRepr: pythonRepr(object["mode"]),
			Text:     stringValue(object["text"]), FeatureID: stringValue(object["feature_id"]),
			Paths: casePaths(object), Limit: caseLimit(object),
			Partition: pythonStrDefault(object, "partition", "development"),
		}
		item.Split = CaseSplit(item.ID)
		item.BudgetBytes, item.BudgetMalformed = caseBudget(object)
		expected, _ := object["expected"].(map[string]any)
		item.Expected = expectations{
			MustInclude: stringsValue(expected["must_include"]), Critical: stringsValue(expected["critical"]),
			MustExclude: stringsValue(expected["must_exclude"]), Relevant: stringsValue(expected["relevant"]),
		}
		if value, present := expected["state"]; present {
			item.Expected.State = value
		}
		cases = append(cases, item)
	}
	return raw, cases, nil
}

func evaluateCase(ctx context.Context, index *contextindex.Index, item goldenCase, snapshot *contextindex.QueryTraceSnapshot) (map[string]any, error) {
	// A malformed budget_bytes must not reach the verb at all: contextindex's
	// own budget check (unchanged here) only ever reports the out-of-range
	// message, so a bad type would silently surface as that wrong text
	// instead of the oracle's "must be an integer". Running the verb with no
	// budget first reproduces the oracle's own precedence -- validate_budget
	// runs last, after limit, text/feature_id/paths and ranking, so any
	// earlier-precedence refusal here is unaffected. Only when the verb would
	// otherwise have succeeded do we substitute the oracle's type-error text.
	budget := item.BudgetBytes
	if item.BudgetMalformed {
		budget = nil
	}
	receipt, err := dispatchCase(ctx, index, item, snapshot, budget)
	if err != nil {
		return nil, err
	}
	if item.BudgetMalformed {
		return nil, fmt.Errorf("budget_bytes must be an integer")
	}
	return receipt, nil
}

func dispatchCase(ctx context.Context, index *contextindex.Index, item goldenCase, snapshot *contextindex.QueryTraceSnapshot, budget *int) (map[string]any, error) {
	switch item.Mode {
	case "query":
		if snapshot != nil {
			return contextindex.EvalQuery(ctx, index, item.Text, item.Limit, budget, *snapshot)
		}
		return contextindex.EvalQuery(ctx, index, item.Text, item.Limit, budget)
	case "feature":
		return contextindex.EvalFeature(index, item.FeatureID, item.Limit, budget)
	case "impact":
		return contextindex.EvalImpact(index, item.Paths, item.Limit, budget)
	default:
		return nil, fmt.Errorf("unknown golden mode: %s", item.ModeRepr)
	}
}

func (counts *counters) addCase(item goldenCase, receipt map[string]any) error {
	encoded, err := contextindex.CanonicalJSON(receipt)
	if err != nil {
		return err
	}
	envelope, err := newScoredEnvelope(item, receipt)
	if err != nil {
		return err
	}
	counts.scoredEnvelopes = append(counts.scoredEnvelopes, envelope)
	counts.packetBytes += len(encoded)
	state := stringValue(receipt["state"])
	counts.caseStates = append(counts.caseStates, state)
	if item.BudgetBytes != nil {
		counts.budgetTotal++
		if len(encoded) <= *item.BudgetBytes {
			counts.budgetHit++
		}
	}
	if state == "BUDGETED" {
		counts.budgetOverflows++
	}
	if state == "CRITICAL_EVIDENCE_OVERFLOW" {
		counts.criticalOverflows++
	}
	results := mapsValue(receipt["results"])
	coverage, _ := receipt["coverage"].(map[string]any)
	counts.authoritativeResults += integer(coverage["authoritative_results"], len(results))
	counts.advisoryResults += integer(coverage["advisory_results"], 0)
	actual := selectorSet(results)
	must, critical := stringSet(item.Expected.MustInclude), stringSet(item.Expected.Critical)
	mustExclude := stringSet(item.Expected.MustExclude)
	relevant := stringSet(append(append(append([]string{}, item.Expected.Relevant...), item.Expected.MustInclude...), item.Expected.Critical...))
	score := caseScore{
		id: item.ID, split: item.Split, criticalTotal: len(critical),
		criticalMisses: len(difference(critical, actual)),
		mustHit:        intersectionSize(must, actual), mustTotal: len(must),
	}
	counts.mustTotal += len(must)
	counts.mustHit += intersectionSize(must, actual)
	counts.criticalTotal += len(critical)
	counts.criticalHit += intersectionSize(critical, actual)
	counts.excludeTotal += len(mustExclude)
	unexpected := intersection(mustExclude, actual)
	counts.excludeViolations += len(unexpected)
	score.excludeViolations = len(unexpected)
	if len(relevant) != 0 {
		counts.positiveCases++
	}
	expectedState := item.Expected.State
	stateMatches := expectedState == nil || state == stringValue(expectedState)
	if stringValue(expectedState) == "OUT_OF_SCOPE" {
		score.abstentionApplicable = true
		score.epistemicApplicable = true
		counts.abstentionTotal++
		counts.epistemicTotal++
		hit := stateMatches && len(results) == 0
		if hit {
			counts.abstentionHit++
			counts.epistemicHit++
		}
		score.abstentionHit, score.epistemicHit = hit, hit
	} else if stringValue(expectedState) == "NEEDS_WIDENING" {
		score.epistemicApplicable = true
		counts.wideningTotal++
		counts.epistemicTotal++
		abstention, _ := receipt["abstention"].(map[string]any)
		hit := stateMatches && len(results) != 0 && boolValue(abstention["active"])
		if hit {
			counts.wideningHit++
			counts.epistemicHit++
		}
		score.epistemicHit = hit
	}
	if len(relevant) != 0 {
		score.top5Applicable = true
		counts.top5Total++
		score.top5Hit = intersects(relevant, selectorSet(results[:min(5, len(results))]))
		if score.top5Hit {
			counts.top5Hit++
		}
	} else if stringValue(expectedState) == "OUT_OF_SCOPE" {
		score.top5Applicable = true
		counts.top5Total++
		score.top5Hit = len(results) == 0
		if score.top5Hit {
			counts.top5Hit++
		}
	}
	actualDetails := make([]any, 0, len(results))
	pendingMust := stringSet(item.Expected.MustInclude)
	for _, result := range results {
		selector := selectorKey(result)
		weight, err := weightedJSON(result)
		if err != nil {
			return err
		}
		counts.totalResultBytes += len(weight)
		score.totalResultBytes += len(weight)
		if _, first := pendingMust[selector]; first {
			delete(pendingMust, selector)
			score.mustHitBytes = append(score.mustHitBytes, score.totalResultBytes)
		}
		_, isRelevant := relevant[selector]
		if isRelevant {
			counts.relevantBytes += len(weight)
			score.relevantBytes += len(weight)
		}
		actualDetails = append(actualDetails, map[string]any{"selector": selector, "relevant": isRelevant})
	}
	missing := difference(union(must, critical), actual)
	normalState := state == "READY" || state == "BUDGETED"
	counts.caseChecks = append(counts.caseChecks, len(missing) == 0 && len(unexpected) == 0 && stateMatches && (expectedState != nil || normalState))
	counts.caseScores = append(counts.caseScores, score)
	counts.details = append(counts.details, map[string]any{
		"id": item.ID, "state": state, "partition": item.Partition, "missing": stringsAny(missing),
		"unexpected": stringsAny(unexpected), "expected_state": expectedState, "state_matches": stateMatches,
		"packet_bytes": len(encoded), "budget_bytes": pointerValue(item.BudgetBytes), "actual": actualDetails,
	})
	return nil
}

func newScoredEnvelope(item goldenCase, receipt map[string]any) (scoredEnvelope, error) {
	field := func(name string) map[string]any {
		value, present := receipt[name]
		return map[string]any{"present": present, "value": value}
	}
	encoded, err := contextindex.CanonicalJSON(map[string]any{
		"case":     map[string]any{"id": item.ID, "mode": item.Mode},
		"request":  field("request"),
		"revision": field("revision"),
		"intent":   field("intent"),
	})
	if err != nil {
		return scoredEnvelope{}, err
	}
	return scoredEnvelope{caseID: item.ID, encoded: encoded}, nil
}

func requireIdenticalScoredEnvelopes(baseline, learned []scoredEnvelope) error {
	if len(baseline) != len(learned) {
		return fmt.Errorf("baseline and learned scored envelope counts differ: %d != %d", len(baseline), len(learned))
	}
	for index := range baseline {
		if bytes.Equal(baseline[index].encoded, learned[index].encoded) {
			continue
		}
		return fmt.Errorf(
			"baseline and learned scored envelope differs at case %d (%q != %q)",
			index, baseline[index].caseID, learned[index].caseID,
		)
	}
	return nil
}

func (counts *counters) report(canonical bool, canonicalGolden string, golden []byte, head, revision, traceState string, passedTraces int, started time.Time) map[string]any {
	precision := ratio(counts.relevantBytes, counts.totalResultBytes, 0)
	budgetCompliance := ratio(counts.budgetHit, counts.budgetTotal, 1)
	abstentionAccuracy := ratio(counts.abstentionHit, counts.abstentionTotal, 1)
	wideningAccuracy := ratio(counts.wideningHit, counts.wideningTotal, 1)
	epistemicAccuracy := ratio(counts.epistemicHit, counts.epistemicTotal, 1)
	top5Success := ratio(counts.top5Hit, counts.top5Total, 1)
	replayRecall, replayTop5Success, replayTasks := 0.0, 0.0, 0
	checks := map[string]any{
		"canonical_frozen_corpus": canonical, "positive_evaluation_cases": counts.positiveCases > 0,
		"positive_result_bytes": counts.totalResultBytes > 0, "minimum_passed_traces": passedTraces >= promotionMinTraces,
		"minimum_replayed_traces": replayTasks >= promotionMinTraces, "frozen_precision": precision >= promotionMinRatio,
		"frozen_top5_success": top5Success >= promotionMinRatio, "trace_replay_recall": replayRecall >= promotionMinRatio,
		"trace_replay_top5_success": replayTop5Success >= promotionMinRatio,
		"zero_critical_misses":      counts.criticalHit == counts.criticalTotal,
		"abstention_accuracy":       abstentionAccuracy == 1.0, "epistemic_state_accuracy": epistemicAccuracy == 1.0,
		"budget_compliance": budgetCompliance == 1.0,
	}
	blocked := make([]string, 0)
	for key, value := range checks {
		if !boolValue(value) {
			blocked = append(blocked, key)
		}
	}
	sort.Strings(blocked)
	evidenceComplete := counts.criticalHit == counts.criticalTotal && counts.mustHit == counts.mustTotal && counts.excludeViolations == 0
	for _, check := range counts.caseChecks {
		evidenceComplete = evidenceComplete && check
	}
	state := "CONFLICT"
	if containsString(counts.caseStates, "STALE_INDEX") {
		state = "STALE_INDEX"
	} else if evidenceComplete {
		state = "READY"
	}
	digest := sha256.Sum256(golden)
	latency := round(float64(time.Since(started).Nanoseconds())/1_000_000, 3)
	return map[string]any{
		"schema_version": schemaVersion, "revision": revision, "history_tip": head, "state": state,
		"metrics": map[string]any{
			"cases": len(counts.details), "recall": ratio(counts.mustHit, counts.mustTotal, 1),
			"must_read_hits": counts.mustHit, "must_read_total": counts.mustTotal,
			"relevant_result_bytes": counts.relevantBytes, "total_result_bytes": counts.totalResultBytes,
			"critical_evidence_misses": counts.criticalTotal - counts.criticalHit, "critical_evidence_total": counts.criticalTotal,
			"serialized_result_byte_weighted_precision": precision, "must_exclude_total": counts.excludeTotal,
			"must_exclude_violations": counts.excludeViolations, "abstention_accuracy": abstentionAccuracy,
			"abstention_cases": counts.abstentionTotal, "abstention_hits": counts.abstentionHit,
			"widening_accuracy": wideningAccuracy, "widening_cases": counts.wideningTotal, "widening_hits": counts.wideningHit,
			"epistemic_state_accuracy": epistemicAccuracy, "epistemic_state_cases": counts.epistemicTotal,
			"epistemic_state_hits": counts.epistemicHit, "top5_task_success": top5Success,
			"top5_task_hits": counts.top5Hit, "top5_task_total": counts.top5Total,
			"packet_bytes": counts.packetBytes, "latency_ms": latency, "budget_compliance": budgetCompliance,
			"budgeted_cases": counts.budgetTotal, "budget_overflows": counts.budgetOverflows,
			"critical_evidence_overflows": counts.criticalOverflows, "authoritative_results": counts.authoritativeResults,
			"advisory_results": counts.advisoryResults, "trace_state": traceState, "passed_trace_count": passedTraces,
			"trace_replay_tasks": replayTasks, "trace_replay_recall": replayRecall,
			"trace_replay_top5_success": replayTop5Success,
		},
		"promotion": map[string]any{
			"ready":         allChecks(checks),
			"frozen_corpus": map[string]any{"canonical": canonical, "path": canonicalGolden, "sha256": fmt.Sprintf("%x", digest)},
			"thresholds": map[string]any{
				"minimum_passed_traces": promotionMinTraces, "minimum_replayed_traces": promotionMinTraces,
				"minimum_ratio": 0.9, "critical_evidence_misses": 0, "abstention_accuracy": 1.0,
				"epistemic_state_accuracy": 1.0, "budget_compliance": 1.0,
			},
			"checks": checks, "blocked_by": stringsAny(blocked),
		},
		"cases": counts.details,
	}
}

func scoreDeltas(baseline, fixture counters) []any {
	type metric struct {
		name, gate string
		baseline   any
		fixture    any
		changed    func(caseScore, caseScore) bool
		passed     func() bool
		floor      any
	}
	baselinePrecision := ratio(baseline.relevantBytes, baseline.totalResultBytes, 0)
	fixturePrecision := ratio(fixture.relevantBytes, fixture.totalResultBytes, 0)
	baselineAbstention := ratio(baseline.abstentionHit, baseline.abstentionTotal, 1)
	fixtureAbstention := ratio(fixture.abstentionHit, fixture.abstentionTotal, 1)
	baselineEpistemic := ratio(baseline.epistemicHit, baseline.epistemicTotal, 1)
	fixtureEpistemic := ratio(fixture.epistemicHit, fixture.epistemicTotal, 1)
	metrics := []metric{
		{
			name: "critical_evidence_misses", gate: "must not rise",
			baseline: baseline.criticalTotal - baseline.criticalHit, fixture: fixture.criticalTotal - fixture.criticalHit,
			changed: func(left, right caseScore) bool { return left.criticalMisses != right.criticalMisses },
			passed: func() bool {
				return fixture.criticalTotal-fixture.criticalHit <= baseline.criticalTotal-baseline.criticalHit
			},
		},
		{
			name: "abstention_accuracy", gate: "must not fall", baseline: baselineAbstention, fixture: fixtureAbstention,
			changed: func(left, right caseScore) bool {
				return left.abstentionApplicable != right.abstentionApplicable || left.abstentionHit != right.abstentionHit
			},
			passed: func() bool { return fixtureAbstention >= baselineAbstention },
		},
		{
			name: "epistemic_state_accuracy", gate: "must not fall", baseline: baselineEpistemic, fixture: fixtureEpistemic,
			changed: func(left, right caseScore) bool {
				return left.epistemicApplicable != right.epistemicApplicable || left.epistemicHit != right.epistemicHit
			},
			passed: func() bool { return fixtureEpistemic >= baselineEpistemic },
		},
		{
			name: "serialized_result_byte_weighted_precision", gate: "minimum floor",
			baseline: baselinePrecision, fixture: fixturePrecision, floor: 0.80,
			changed: func(left, right caseScore) bool {
				return left.relevantBytes != right.relevantBytes || left.totalResultBytes != right.totalResultBytes
			},
			passed: func() bool { return fixturePrecision >= 0.80 },
		},
		{
			name: "recall", gate: "informational",
			baseline: ratio(baseline.mustHit, baseline.mustTotal, 1), fixture: ratio(fixture.mustHit, fixture.mustTotal, 1),
			changed: func(left, right caseScore) bool {
				return left.mustHit != right.mustHit || left.mustTotal != right.mustTotal
			},
		},
		{
			name: "top_five_task_success", gate: "informational",
			baseline: ratio(baseline.top5Hit, baseline.top5Total, 1), fixture: ratio(fixture.top5Hit, fixture.top5Total, 1),
			changed: func(left, right caseScore) bool {
				return left.top5Applicable != right.top5Applicable || left.top5Hit != right.top5Hit
			},
		},
	}
	rows := make([]any, 0, len(metrics))
	for _, item := range metrics {
		changed := 0
		for index := range baseline.caseScores {
			if item.changed(baseline.caseScores[index], fixture.caseScores[index]) {
				changed++
			}
		}
		delta := numericDelta(item.baseline, item.fixture)
		classification := "distinguished"
		if changed < 2 {
			delta = "not distinguished"
			classification = "not distinguished"
		}
		row := map[string]any{
			"metric": item.name, "baseline": item.baseline, "fixture": item.fixture,
			"delta": delta, "classification": classification, "case_change_count": changed,
			"gate": item.gate,
		}
		if item.passed != nil {
			row["passed"] = item.passed()
		}
		if item.floor != nil {
			row["floor"] = item.floor
		}
		rows = append(rows, row)
	}
	return rows
}

func numericDelta(baseline, fixture any) any {
	if left, ok := baseline.(int); ok {
		return fixture.(int) - left
	}
	return round(fixture.(float64)-baseline.(float64), 6)
}

func Encode(value any) ([]byte, error) { return canonicalJSON(value) }

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.RoundToEven(value*scale) / scale
}

func ratio(numerator, denominator int, empty float64) float64 {
	if denominator == 0 {
		return empty
	}
	return round(float64(numerator)/float64(denominator), 6)
}

func historyTip(receipt map[string]any, fallback string) string {
	learning, _ := receipt["learning"].(map[string]any)
	return stringDefault(learning["history_tip"], fallback)
}

func selectorKey(value map[string]any) string {
	return stringValue(value["kind"]) + ":" + stringValue(value["id"])
}

func selectorSet(values []map[string]any) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[selectorKey(value)] = struct{}{}
	}
	return result
}

func mapsValue(value any) []map[string]any {
	values, _ := value.([]any)
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if object, ok := value.(map[string]any); ok {
			result = append(result, object)
		}
	}
	return result
}

func integer(value any, fallback int) int {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	}
	return fallback
}

// caseLimit reads a golden row's limit. An absent member defaults to 10; a
// present member that is not a JSON integer maps to 0, which the case's verb
// refuses with its own limit message, as the Python oracle does.
func caseLimit(object map[string]any) int {
	value, present := object["limit"]
	if !present {
		return 10
	}
	return integer(value, 0)
}

// caseBudget reads a golden row's budget_bytes. An absent or null member
// means no budget check at all, matching the Python oracle's
// validate_budget(None); a present member that is not a JSON integer cannot
// be represented as *int, so it is reported back as malformed instead of
// being coerced to a sentinel value (see BudgetMalformed).
func caseBudget(object map[string]any) (budget *int, malformed bool) {
	value, present := object["budget_bytes"]
	if !present || value == nil {
		return nil, false
	}
	parsed, ok := exactInteger(value)
	if !ok {
		return nil, true
	}
	return &parsed, false
}

// exactInteger reports whether value decoded from golden/case JSON (via a
// decoder using UseNumber) is a JSON integer, as opposed to a bool, string,
// null, or a json.Number carrying a fraction or exponent.
func exactInteger(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Int64()
	return int(parsed), err == nil
}

// casePaths reads a golden row's impact paths. A value that is not a JSON
// list maps to nil, which the "impact" verb refuses as a non-list the same
// way the oracle's isinstance(paths, list) check does. A list member that is
// not a JSON string is kept as the empty-string sentinel so it still reaches
// cleanImpactPath, whose own "impact paths must be non-empty strings" is the
// identical message the oracle's _clean_impact_path raises for a non-string
// entry -- dropping the member instead, as stringsValue does, would silently
// admit the remaining well-typed paths where the oracle refuses the whole
// case.
func casePaths(object map[string]any) []string {
	values, ok := object["paths"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, len(values))
	for index, value := range values {
		if typed, ok := value.(string); ok {
			result[index] = typed
		}
	}
	return result
}

func stringValue(value any) string { typed, _ := value.(string); return typed }
func stringDefault(value any, fallback string) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return fallback
}
func boolValue(value any) bool { typed, _ := value.(bool); return typed }

// pythonStrDefault renders golden field key the way Python's
// case.get(key, fallback) followed by str(...) would: the fallback applies
// only when the key is wholly absent, not when it is present with a JSON
// null (which the oracle stringifies as "None").
func pythonStrDefault(object map[string]any, key, fallback string) string {
	value, present := object[key]
	if !present {
		return fallback
	}
	return pythonStr(value)
}

// pythonStr renders a JSON-decoded value (the decoder runs with UseNumber,
// so a JSON number arrives as json.Number) the way Python's str() renders
// the same value after json.loads: "None"/"True"/"False" for null/bools, the
// exact digits for an int, Python's shortest round-trip repr for a float,
// and strings unchanged.
func pythonStr(value any) string {
	switch typed := value.(type) {
	case nil:
		return "None"
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case string:
		return typed
	case json.Number:
		return pythonNumberStr(typed)
	default:
		return fmt.Sprint(typed)
	}
}

// pythonRepr renders a JSON-decoded value the way Python's repr() would.
// This differs from pythonStr only for strings, which repr() quotes.
func pythonRepr(value any) string {
	if typed, ok := value.(string); ok {
		return pythonStringRepr(typed)
	}
	return pythonStr(value)
}

// pythonStringRepr quotes a string the way Python's repr() does: single
// quotes, unless the string contains one but no double quote.
func pythonStringRepr(value string) string {
	quote := byte('\'')
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var builder strings.Builder
	builder.WriteByte(quote)
	for _, character := range value {
		switch character {
		case rune(quote), '\\':
			builder.WriteByte('\\')
			builder.WriteRune(character)
		default:
			builder.WriteRune(character)
		}
	}
	builder.WriteByte(quote)
	return builder.String()
}

// pythonNumberStr renders a JSON number the way Python renders the same
// literal after json.loads: a literal with no ".", "e", or "E" decodes to an
// arbitrary-precision int (big.Int.String reproduces str(int) exactly,
// including collapsing "-0" to "0"); anything else decodes to a float,
// rendered with Python's float repr rules.
func pythonNumberStr(number json.Number) string {
	text := string(number)
	if !strings.ContainsAny(text, ".eE") {
		if parsed, ok := new(big.Int).SetString(text, 10); ok {
			return parsed.String()
		}
	}
	value, err := number.Float64()
	if err != nil {
		return text
	}
	return pythonFloatStr(value)
}

// pythonFloatStr renders a float64 the way Python's str()/repr() does:
// fixed notation with at least one fractional digit, switching to
// scientific notation (with a sign and at least two exponent digits) once
// the decimal point would fall at or before position -4 or beyond 16.
func pythonFloatStr(value float64) string {
	sign := ""
	if math.Signbit(value) {
		sign = "-"
		value = -value
	}
	if value == 0 {
		return sign + "0.0"
	}
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponentText, _ := strings.Cut(scientific, "e")
	exponent, _ := strconv.Atoi(exponentText)
	digits := strings.Replace(mantissa, ".", "", 1)
	decimalPoint := exponent + 1
	if decimalPoint <= -4 || decimalPoint > 16 {
		coefficient := digits[:1]
		if len(digits) > 1 {
			coefficient += "." + digits[1:]
		}
		exponentSign, magnitude := "+", decimalPoint-1
		if magnitude < 0 {
			exponentSign, magnitude = "-", -magnitude
		}
		return fmt.Sprintf("%s%se%s%02d", sign, coefficient, exponentSign, magnitude)
	}
	var integerPart, fractionPart string
	switch {
	case decimalPoint <= 0:
		integerPart, fractionPart = "0", strings.Repeat("0", -decimalPoint)+digits
	case decimalPoint >= len(digits):
		integerPart, fractionPart = digits+strings.Repeat("0", decimalPoint-len(digits)), "0"
	default:
		integerPart, fractionPart = digits[:decimalPoint], digits[decimalPoint:]
	}
	return sign + integerPart + "." + fractionPart
}

func stringsValue(value any) []string {
	values, _ := value.([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if typed, ok := value.(string); ok {
			result = append(result, typed)
		}
	}
	return result
}

func stringsAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
func union(left, right map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(left)+len(right))
	for value := range left {
		result[value] = struct{}{}
	}
	for value := range right {
		result[value] = struct{}{}
	}
	return result
}
func intersectionSize(left, right map[string]struct{}) int { return len(intersection(left, right)) }
func intersects(left, right map[string]struct{}) bool      { return intersectionSize(left, right) != 0 }

func intersection(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range left {
		if _, ok := right[value]; ok {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func difference(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range left {
		if _, ok := right[value]; !ok {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func allChecks(checks map[string]any) bool {
	for _, value := range checks {
		if !boolValue(value) {
			return false
		}
	}
	return true
}
func pointerValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
