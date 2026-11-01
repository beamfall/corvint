package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	bootstrapResamples = 10000
	relativeGate       = 0.20
	maxErroredLanes    = 3
	citableGate        = 0.70
	hardFailureGate    = 0.05
)

// changeMeta is everything the scorer needs about one change, carried in the
// report so `score --report` rebuilds every derived field without the manifest.
type changeMeta struct {
	ID          string   `json:"id"`
	Commit      string   `json:"commit"`
	BaseCommit  string   `json:"base_commit"`
	PatchSHA256 string   `json:"patch_sha256"`
	HunkCount   int      `json:"hunk_count"`
	Gold        []string `json:"gold"`
	// GoldSpans is the secondary gold: the one-based line ranges, at the
	// change, of the lines it added to each gold file (CRT-V0-004). It is nil
	// in a report written before CRT-V0-013, whose span rate is NOT_OBSERVED.
	GoldSpans    []span   `json:"gold_spans"`
	StemBaseline []string `json:"stem_baseline"`
}

type lane struct {
	ID         string     `json:"id"`
	Arm        string     `json:"arm"`
	Repeat     int        `json:"repeat"`
	ArmOrder   []string   `json:"arm_order"`
	ReplyState string     `json:"reply_state"`
	Citations  []citation `json:"citations"`
	Unknown    []string   `json:"unknown"`
	Miss       float64    `json:"miss"`
	// CitedNotGold is the lane's precision against the same gold (CRT-V0-013).
	CitedNotGold     citedNotGold `json:"cited_not_gold"`
	GoldHits         []string     `json:"gold_hits"`
	StemBaselineHits []string     `json:"stem_baseline_hits"`
	// Suggested is the seeded arm's distinct suggested paths, as the prologue
	// carried them; SeedHits is the gold among them (CRT-V0-012).
	Suggested      []string       `json:"suggested,omitempty"`
	SeedHits       []string       `json:"seed_hits,omitempty"`
	CEM            map[string]any `json:"cem,omitempty"`
	WallMs         any            `json:"wall_ms"`
	Tokens         any            `json:"tokens"`
	ToolCalls      any            `json:"tool_calls,omitempty"`
	ExitCode       any            `json:"exit_code"`
	Reply          string         `json:"reply"`
	ReplyTruncated bool           `json:"reply_truncated"`
	// StdoutTruncated records that the agent's stdout reached the capture
	// bound and was cut (CRT-V0-011).
	StdoutTruncated bool `json:"stdout_truncated,omitempty"`
	// StartedAt is the lane's launch time, UTC RFC 3339, which the
	// registration order is checked against (CRT-V0-014).
	StartedAt    string `json:"started_at,omitempty"`
	PromptSHA256 string `json:"prompt_sha256"`
	PromptBytes  int    `json:"prompt_bytes"`
	ReusedFrom   string `json:"reused_from,omitempty"`
	// ReusedRegistration is the sha256 of the registration the reused lane
	// first ran under, so a lane reused across a scorer-only re-registration of
	// an unchanged manifest is told apart from a resumed one (CRT-V0-016).
	ReusedRegistration string `json:"reused_registration,omitempty"`
	Error              string `json:"error,omitempty"`
}

type report struct {
	Profile        string                    `json:"profile"`
	Partition      string                    `json:"partition"`
	Pilot          bool                      `json:"pilot"`
	Repository     string                    `json:"repository"`
	SelectionRule  string                    `json:"selection_rule"`
	Population     int                       `json:"population"`
	Pairs          int                       `json:"pairs"`
	Seed           string                    `json:"seed"`
	Model          string                    `json:"model"`
	Effort         string                    `json:"effort"`
	ManifestSHA256 string                    `json:"manifest_sha256"`
	SkeletonSHA256 string                    `json:"skeleton_sha256"`
	Arms           []string                  `json:"arms"`
	Repeats        int                       `json:"repeats"`
	Prologues      map[string]map[string]any `json:"prologues"`
	Agent          map[string]any            `json:"agent"`
	CorvintGo      map[string]any            `json:"corvint"`
	Changes        []changeMeta              `json:"changes"`
	Lanes          []*lane                   `json:"lanes"`
	Registration   *registrationRecord       `json:"registration,omitempty"`
	// ReRegistered is the earlier registration of the report a re-declared
	// run reuses lanes from (CRT-V0-016).
	ReRegistered *reRegistration `json:"re_registered,omitempty"`
	Summary      map[string]any  `json:"summary,omitempty"`
	Invalid      string          `json:"invalid,omitempty"`
	Partial      bool            `json:"partial,omitempty"`
}

// finalize rebuilds every derived field from the raw ones, so a run and a
// later `score --report` produce the same bytes (CRT-V0-010).
func finalize(document *report) {
	meta := map[string]changeMeta{}
	for _, item := range document.Changes {
		meta[item.ID] = item
	}
	for _, item := range document.Lanes {
		scoreLane(item, meta[item.ID])
	}
	document.Summary = summarize(document, meta)
	reasons := []string{}
	if failed := erroredLanes(document); failed > maxErroredLanes {
		reasons = append(reasons, strconv.Itoa(failed)+" lanes errored, more than the "+
			strconv.Itoa(maxErroredLanes)+" CRT-V0-010 allows: the run is discarded, not re-scored")
	}
	reasons = append(reasons, registrationInvalid(document)...)
	document.Invalid = strings.Join(reasons, "; ")
}

func scoreLane(item *lane, meta changeMeta) {
	citations, state := extractCitations(item.Reply)
	item.ReplyState, item.Citations, item.Unknown = state, citations, extractUnknown(item.Reply)
	if item.Citations == nil {
		item.Citations = []citation{}
	}
	item.CitedNotGold = scoreCitedNotGold(item.Citations, meta)
	cited := map[string]bool{}
	for _, one := range item.Citations {
		cited[normalizePath(one.Path)] = true
	}
	stem := map[string]bool{}
	for _, one := range meta.StemBaseline {
		stem[normalizePath(one)] = true
	}
	suggested := map[string]bool{}
	for _, one := range item.Suggested {
		suggested[normalizePath(one)] = true
	}
	item.GoldHits, item.StemBaselineHits = []string{}, []string{}
	if item.Suggested != nil {
		item.SeedHits = []string{}
	}
	missed := 0
	for _, one := range meta.Gold {
		normalized := normalizePath(one)
		if cited[normalized] {
			item.GoldHits = append(item.GoldHits, one)
		} else {
			missed++
		}
		if stem[normalized] {
			item.StemBaselineHits = append(item.StemBaselineHits, one)
		}
		if suggested[normalized] {
			item.SeedHits = append(item.SeedHits, one)
		}
	}
	sort.Strings(item.GoldHits)
	sort.Strings(item.StemBaselineHits)
	sort.Strings(item.SeedHits)
	if len(meta.Gold) == 0 {
		item.Miss = 0
		return
	}
	item.Miss = round(float64(missed) / float64(len(meta.Gold)))
}

// citedNotGold is one lane's share of what it cited that the withheld gold
// does not hold: `paths` over its distinct cited paths, `spans` over its
// distinct cited lines, against the recorded gold spans of the same path.
// Each is NOT_OBSERVED when the lane cited nothing, and `spans` is also
// NOT_OBSERVED when the report recorded no span gold (CRT-V0-013).
type citedNotGold struct {
	Paths any `json:"paths"`
	Spans any `json:"spans"`
}

// interval is a one-based inclusive line range.
type interval struct{ start, end int }

func scoreCitedNotGold(citations []citation, meta changeMeta) citedNotGold {
	if len(citations) == 0 {
		return citedNotGold{Paths: notObserved, Spans: notObserved}
	}
	gold := map[string]bool{}
	for _, one := range meta.Gold {
		gold[normalizePath(one)] = true
	}
	paths := map[string]bool{}
	cited := map[string][]interval{}
	malformed := 0
	for _, one := range citations {
		path := normalizePath(one.Path)
		paths[path] = true
		start, end, ok := parseLines(one.Lines)
		if !ok || start < 1 {
			// A range the scorer cannot place counts as one cited line
			// outside the gold, so a malformed range never escapes the rate.
			malformed++
			continue
		}
		cited[path] = append(cited[path], interval{start, end})
	}
	outside := 0
	for path := range paths {
		if !gold[path] {
			outside++
		}
	}
	result := citedNotGold{Paths: round(float64(outside) / float64(len(paths))), Spans: notObserved}
	if meta.GoldSpans == nil {
		return result
	}
	spans := map[string][]interval{}
	for _, one := range meta.GoldSpans {
		start, end, ok := parseLines(one.Lines)
		if ok && start >= 1 && gold[normalizePath(one.Path)] {
			spans[normalizePath(one.Path)] = append(spans[normalizePath(one.Path)], interval{start, end})
		}
	}
	total, inside := float64(malformed), 0.0
	for path, ranges := range cited {
		merged := mergeIntervals(ranges)
		for _, one := range merged {
			total += float64(one.end-one.start) + 1
		}
		inside += overlap(merged, mergeIntervals(spans[path]))
	}
	result.Spans = round((total - inside) / total)
	return result
}

// mergeIntervals sorts ranges and joins the overlapping or adjacent ones, so
// a line cited twice is counted once.
func mergeIntervals(ranges []interval) []interval {
	if len(ranges) == 0 {
		return nil
	}
	sorted := append([]interval(nil), ranges...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left].start < sorted[right].start })
	merged := []interval{sorted[0]}
	for _, one := range sorted[1:] {
		last := &merged[len(merged)-1]
		if one.start <= last.end || one.start-last.end == 1 {
			if one.end > last.end {
				last.end = one.end
			}
			continue
		}
		merged = append(merged, one)
	}
	return merged
}

// overlap counts the lines two merged interval lists share.
func overlap(left, right []interval) float64 {
	shared := 0.0
	for _, one := range left {
		for _, other := range right {
			start, end := max(one.start, other.start), min(one.end, other.end)
			if start <= end {
				shared += float64(end-start) + 1
			}
		}
	}
	return shared
}

// citedNotGoldArms is the mean lane rate per arm over the lanes of the scored
// pairs that did not error and cited something, reported beside the miss
// estimate so an arm that buys recall by citing everything shows its cost.
func citedNotGoldArms(document *report, pairs []pair, arms ...string) map[string]any {
	scored := map[string]bool{}
	for _, item := range pairs {
		scored[item.id] = true
	}
	result := map[string]any{}
	for _, arm := range arms {
		paths, spans := []float64{}, []float64{}
		for _, item := range document.Lanes {
			if item.Arm != arm || !scored[item.ID] || laneFailed(item) {
				continue
			}
			if value, ok := item.CitedNotGold.Paths.(float64); ok {
				paths = append(paths, value)
			}
			if value, ok := item.CitedNotGold.Spans.(float64); ok {
				spans = append(spans, value)
			}
		}
		block := map[string]any{"lanes": len(paths), "paths": notObserved, "spans": notObserved, "span_lanes": len(spans)}
		if len(paths) != 0 {
			block["paths"] = round(mean(paths))
		}
		if len(spans) != 0 {
			block["spans"] = round(mean(spans))
		}
		result[arm] = block
	}
	return result
}

// pair is one change's mean miss per arm, over the repeats of that arm. The
// `treatment` field is the compared arm's mean, whichever arm the contrast
// names; the primary contrast keeps the pilot's field names.
type pair struct {
	id                 string
	control, treatment float64
	stem               float64
}

// laneMisses groups the scored lanes by change and arm, in first-seen change
// order, and marks the changes an errored lane removes whole.
func laneMisses(document *report) (map[string]map[string][]float64, map[string]bool, []string) {
	byChange := map[string]map[string][]float64{}
	errored := map[string]bool{}
	order := []string{}
	for _, item := range document.Lanes {
		if _, seen := byChange[item.ID]; !seen {
			byChange[item.ID] = map[string][]float64{}
			order = append(order, item.ID)
		}
		if laneFailed(item) {
			errored[item.ID] = true
			continue
		}
		byChange[item.ID][item.Arm] = append(byChange[item.ID][item.Arm], item.Miss)
	}
	return byChange, errored, order
}

// armPairs is the within-change pairing of `reference` against `compared`:
// every change both arms scored, neither errored (CRT-V0-002, CRT-V0-007).
func armPairs(byChange map[string]map[string][]float64, errored map[string]bool, order []string, meta map[string]changeMeta, reference, compared string) []pair {
	pairs := []pair{}
	for _, id := range order {
		arms := byChange[id]
		if errored[id] || len(arms[reference]) == 0 || len(arms[compared]) == 0 {
			continue
		}
		pairs = append(pairs, pair{
			id: id, control: mean(arms[reference]), treatment: mean(arms[compared]),
			stem: stemMiss(meta[id]),
		})
	}
	return pairs
}

// comparedArms is every arm the report ran other than control, in run order.
func comparedArms(document *report) []string {
	arms := []string{}
	for _, name := range document.Arms {
		if name != "control" {
			arms = append(arms, name)
		}
	}
	return arms
}

// summarize builds the paired estimates. A change with any errored lane is
// dropped whole, so the arms always cover the same changes (CRT-V0-007).
// The top level carries the primary contrast, control against `treatment`
// (or against the first other arm when treatment did not run), under the
// pilot's keys; `arms` carries the same block per compared arm; a seeded run
// adds `seed_coverage` and, beside a treatment arm, the `seeded_vs_treatment`
// contrast (CRT-V0-012).
func summarize(document *report, meta map[string]changeMeta) map[string]any {
	byChange, errored, order := laneMisses(document)
	compared := comparedArms(document)
	primary := "treatment"
	if !contains(compared, "treatment") && len(compared) != 0 {
		primary = compared[0]
	}
	summary := armSummary(document, meta, byChange, errored, order, primary)
	summary["errored_lanes"] = erroredLanes(document)
	if len(compared) > 1 || contains(compared, "seeded") {
		arms := map[string]any{}
		for _, arm := range compared {
			block := armSummary(document, meta, byChange, errored, order, arm)
			block["mcnemar_lanes"] = mcnemarLanes(document, "control", arm)
			arms[arm] = block
		}
		summary["arms"] = arms
	}
	summary["stem_baseline"] = stemBaselineBlock(document, meta, byChange, errored, order)
	summary["outcomes"] = separateOutcomes(document)
	if contains(compared, "stem") {
		contrasts := map[string]any{}
		for _, arm := range compared {
			if arm != "stem" {
				contrasts[arm+"_vs_stem"] = contrast(document, meta, byChange, errored, order, "stem", arm)
			}
		}
		summary["stem_contrasts"] = contrasts
	}
	if contains(compared, "seeded") {
		summary["seed_coverage"] = seedCoverage(document, meta)
		if contains(compared, "treatment") {
			summary["contrasts"] = map[string]any{"seeded_vs_treatment": contrast(document, meta, byChange, errored, order, "treatment", "seeded")}
		}
	}
	return summary
}

// armSummary is the pilot's summary block for one compared arm against
// control: the pair counts, mean misses, the paired delta and its relative
// size, the change-level McNemar, the arm's citable and verifier rates, the
// BCa interval and the CRT-V0-008 verdict wording.
func armSummary(document *report, meta map[string]changeMeta, byChange map[string]map[string][]float64, errored map[string]bool, order []string, arm string) map[string]any {
	pairs := armPairs(byChange, errored, order, meta, "control", arm)
	summary := map[string]any{
		"pairs_scored": len(pairs), "pairs_dropped": len(order) - len(pairs),
	}
	if len(pairs) == 0 {
		summary["verdict"] = "no scored pair: the run measured nothing"
		return summary
	}
	deltas := make([]float64, len(pairs))
	controls := make([]float64, len(pairs))
	for index, item := range pairs {
		deltas[index] = item.control - item.treatment
		controls[index] = item.control
	}
	delta := mean(deltas)
	controlMiss := mean(controls)
	relative := 0.0
	if controlMiss > 0 {
		relative = delta / controlMiss
	}
	summary["mean_miss"] = map[string]any{
		"control": round(controlMiss), "treatment": round(mean(missOf(pairs, "treatment"))),
		"stem_baseline": round(mean(missOf(pairs, "stem"))),
	}
	if arm != "treatment" {
		summary["mean_miss"].(map[string]any)[arm] = round(mean(missOf(pairs, "treatment")))
	}
	summary["delta"] = round(delta)
	summary["relative_reduction"] = round(relative)
	summary["cited_not_gold"] = citedNotGoldArms(document, pairs, "control", arm)
	summary["mcnemar"] = mcnemar(pairs)
	summary["citable"] = citableArm(document, arm)
	summary["verifier"] = verifierRatesArm(document, arm)
	if len(pairs) < 2 {
		// One pair has no resampling variability: its "interval" would be the
		// delta itself and could read as met (CRT-V0-008).
		summary["delta_ci95"] = notObserved
		summary["verdict"] = verdicts(delta, relative, 0, 0, controlMiss, summary)
		summary["verdict"].(map[string]any)["missed_evidence"] = notEstimable
		summary["verdict"].(map[string]any)["kill_readings"] = []string{}
		return summary
	}
	low, high := bca(deltas, document.Seed)
	summary["delta_ci95"] = map[string]any{"low": round(low), "high": round(high), "resamples": bootstrapResamples, "method": "BCa"}
	summary["verdict"] = verdicts(delta, relative, low, high, controlMiss, summary)
	return summary
}

// contrast is the paired difference between two compared arms, positive when
// `compared` missed less than `reference`, with the same interval method.
func contrast(document *report, meta map[string]changeMeta, byChange map[string]map[string][]float64, errored map[string]bool, order []string, reference, compared string) map[string]any {
	pairs := armPairs(byChange, errored, order, meta, reference, compared)
	result := map[string]any{"reference": reference, "compared": compared, "pairs_scored": len(pairs)}
	if len(pairs) == 0 {
		return result
	}
	deltas := make([]float64, len(pairs))
	for index, item := range pairs {
		deltas[index] = item.control - item.treatment
	}
	result["mean_miss"] = map[string]any{reference: round(mean(missOf(pairs, "control"))), compared: round(mean(missOf(pairs, "treatment")))}
	result["delta"] = round(mean(deltas))
	result["cited_not_gold"] = citedNotGoldArms(document, pairs, reference, compared)
	result["mcnemar_lanes"] = mcnemarLanes(document, reference, compared)
	if len(pairs) < 2 {
		result["delta_ci95"] = notObserved
		return result
	}
	low, high := bca(deltas, document.Seed+"\x00"+reference+"\x00"+compared)
	result["delta_ci95"] = map[string]any{"low": round(low), "high": round(high), "resamples": bootstrapResamples, "method": "BCa"}
	if reading, killed := seededKillReading(high); killed && reference == "treatment" && compared == "seeded" {
		result["kill_reading"] = reading
	}
	return result
}

// mcnemarLanes is the exact paired test on "missed anything at all" over
// lanes rather than changes: the control and compared lane of the same change
// and repeat form one pair. Averaging the repeats first, as the change-level
// test does, turns every change both arms sometimes missed into a tie, which
// is how the 2026-09-04 pilot reported zero discordant pairs over fifty lanes.
func mcnemarLanes(document *report, reference, compared string) map[string]any {
	type key struct {
		id     string
		repeat int
	}
	referenceMissed := map[key]bool{}
	comparedMissed := map[key]bool{}
	failed := map[string]bool{}
	for _, item := range document.Lanes {
		if laneFailed(item) {
			failed[item.ID] = true
			continue
		}
		switch item.Arm {
		case reference:
			referenceMissed[key{item.ID, item.Repeat}] = item.Miss > 0
		case compared:
			comparedMissed[key{item.ID, item.Repeat}] = item.Miss > 0
		}
	}
	b, c, pairs := 0, 0, 0
	for at, missed := range referenceMissed {
		other, paired := comparedMissed[at]
		if !paired || failed[at.id] {
			continue
		}
		pairs++
		if missed && !other {
			b++
		}
		if other && !missed {
			c++
		}
	}
	return map[string]any{
		"pairs": pairs, "reference_only_missed": b, "compared_only_missed": c,
		"p_value": round(exactBinomial(b, c)),
	}
}

// seedCoverage decomposes the seeded arm (CRT-V0-012): how much of the gold
// Corvint's suggestions reached, and the miss rate of the gold the agent was
// shown against the gold it was not. A seeded arm that misses what it was
// shown is an adoption result; one that misses only what it was not shown is a
// retrieval result, and the two are never read as one number.
func seedCoverage(document *report, meta map[string]changeMeta) map[string]any {
	goldItems, suggestedItems, missedSuggested, missedUnsuggested, lanes := 0, 0, 0, 0, 0
	for _, item := range document.Lanes {
		if item.Arm != "seeded" || laneFailed(item) {
			continue
		}
		lanes++
		cited := map[string]bool{}
		for _, one := range item.GoldHits {
			cited[normalizePath(one)] = true
		}
		seeded := map[string]bool{}
		for _, one := range item.SeedHits {
			seeded[normalizePath(one)] = true
		}
		for _, one := range meta[item.ID].Gold {
			normalized := normalizePath(one)
			goldItems++
			if seeded[normalized] {
				suggestedItems++
				if !cited[normalized] {
					missedSuggested++
				}
			} else if !cited[normalized] {
				missedUnsuggested++
			}
		}
	}
	result := map[string]any{"lanes": lanes, "gold_items": goldItems, "suggested_items": suggestedItems,
		"fraction": notObserved, "miss_given_suggested": notObserved, "miss_given_unsuggested": notObserved}
	if goldItems != 0 {
		result["fraction"] = round(float64(suggestedItems) / float64(goldItems))
	}
	if suggestedItems != 0 {
		result["miss_given_suggested"] = round(float64(missedSuggested) / float64(suggestedItems))
	}
	if unsuggested := goldItems - suggestedItems; unsuggested != 0 {
		result["miss_given_unsuggested"] = round(float64(missedUnsuggested) / float64(unsuggested))
	}
	return result
}

func missOf(pairs []pair, field string) []float64 {
	values := make([]float64, len(pairs))
	for index, item := range pairs {
		switch field {
		case "treatment":
			values[index] = item.treatment
		case "stem":
			values[index] = item.stem
		default:
			values[index] = item.control
		}
	}
	return values
}

// stemMiss is the floor a naming convention alone reaches on this change.
func stemMiss(meta changeMeta) float64 {
	if len(meta.Gold) == 0 {
		return 0
	}
	guessed := map[string]bool{}
	for _, one := range meta.StemBaseline {
		guessed[normalizePath(one)] = true
	}
	missed := 0
	for _, one := range meta.Gold {
		if !guessed[normalizePath(one)] {
			missed++
		}
	}
	return float64(missed) / float64(len(meta.Gold))
}

func erroredLanes(document *report) int {
	count := 0
	for _, item := range document.Lanes {
		if laneFailed(item) {
			count++
		}
	}
	return count
}

// laneFailed reports a lane that produced no observation: the harness recorded
// an error, or the agent process exited non-zero. A non-zero exit is an errored
// lane under CRT-V0-010, not the "cited nothing" reply of CRT-V0-006, so it is
// dropped from the estimate instead of scoring as a total miss. The exit code is
// read from the recorded field so `score --report` reaches the same verdict on a
// report written before the harness recorded the error. A reply cut at the
// reply bound with no citations block left is the harness's loss of the
// observation, so it is errored the same way.
func laneFailed(item *lane) bool {
	if item.Error != "" {
		return true
	}
	if _, state := extractCitations(item.Reply); item.ReplyTruncated && state != "PRESENT" {
		return true
	}
	code, observed := exitCodeOf(item.ExitCode)
	return observed && code != 0
}

func exitCodeOf(value any) (int, bool) {
	switch code := value.(type) {
	case int:
		return code, true
	case float64:
		return int(code), true
	case json.Number:
		parsed, err := code.Int64()
		return int(parsed), err == nil
	}
	return 0, false
}

// mcnemar is the exact paired test on "this arm missed anything at all".
func mcnemar(pairs []pair) map[string]any {
	b, c := 0, 0
	for _, item := range pairs {
		controlMissed, treatmentMissed := item.control > 0, item.treatment > 0
		if controlMissed && !treatmentMissed {
			b++
		}
		if treatmentMissed && !controlMissed {
			c++
		}
	}
	return map[string]any{
		"control_only_missed": b, "treatment_only_missed": c,
		"p_value": round(exactBinomial(b, c)),
	}
}

// exactBinomial is the two-sided exact test on the discordant pairs.
func exactBinomial(b, c int) float64 {
	n := b + c
	if n == 0 {
		return 1
	}
	smaller := b
	if c < b {
		smaller = c
	}
	total := 0.0
	for k := 0; k <= smaller; k++ {
		total += binomial(n, k)
	}
	probability := 2 * total / math.Pow(2, float64(n))
	if probability > 1 {
		return 1
	}
	return probability
}

func binomial(n, k int) float64 {
	result := 1.0
	for index := 0; index < k; index++ {
		result = result * float64(n-index) / float64(index+1)
	}
	return result
}

// citable is supported over material hunks (total less mechanical), over the
// treatment lanes whose map `cem status` reported (CRT-V0-007).
func citable(document *report) map[string]any {
	return citableArm(document, "treatment")
}

func citableArm(document *report, arm string) map[string]any {
	supported, material, lanes := 0.0, 0.0, 0
	for _, item := range document.Lanes {
		if item.Arm != arm || laneFailed(item) || item.CEM == nil {
			continue
		}
		counts, ok := item.CEM["counts"].(map[string]any)
		if !ok {
			continue
		}
		total, hasTotal := number(counts["total"])
		mechanical, _ := number(counts["mechanical"])
		value, hasSupported := number(counts["supported"])
		if !hasTotal || !hasSupported || total-mechanical <= 0 {
			continue
		}
		supported += value
		material += total - mechanical
		lanes++
	}
	if lanes == 0 {
		return map[string]any{"lanes": 0, "supported": 0, "material": 0, "fraction": notObserved}
	}
	return map[string]any{"lanes": lanes, "supported": supported, "material": material, "fraction": round(supported / material)}
}

// verifierRates counts the treatment lanes that cited something and asks how
// often `cem verify` failed while the harness's own replay said the citation
// was valid: the incorrect hard failures the target bounds (CRT-V0-007).
func verifierRates(document *report) map[string]any {
	return verifierRatesArm(document, "treatment")
}

func verifierRatesArm(document *report, arm string) map[string]any {
	citing, failures, incorrect := 0, 0, 0
	for _, item := range document.Lanes {
		if item.Arm != arm || laneFailed(item) || item.CEM == nil || len(item.Citations) == 0 {
			continue
		}
		citing++
		if item.CEM["verify_ok"] == true {
			continue
		}
		failures++
		if item.CEM["replay_ok"] == true {
			incorrect++
		}
	}
	if citing == 0 {
		return map[string]any{"citing_lanes": 0, "hard_failures": 0, "incorrect_hard_failures": 0, "incorrect_rate": notObserved}
	}
	return map[string]any{
		"citing_lanes": citing, "hard_failures": failures, "incorrect_hard_failures": incorrect,
		"incorrect_rate": round(float64(incorrect) / float64(citing)),
	}
}

// notEstimable is the missed-evidence reading when fewer than two pairs scored.
const notEstimable = "not estimable: fewer than two scored pairs"

// verdicts states each target in the wording of the amended CRT-V0-008
// (accepted 2026-10-25): the missed-evidence gate is met when the interval's
// lower bound exceeds 0 and the interval does not exclude the 20% target,
// whatever the point estimate; kill_readings lists the amended kill reading.
func verdicts(delta, relative, low, high, controlMiss float64, summary map[string]any) map[string]any {
	// The bootstrap interval is in absolute deltas, so scale the relative target.
	targetDelta := relativeGate * controlMiss
	missed := "consistent with 0 but not with 20%"
	switch {
	case low > 0 && high >= targetDelta:
		missed = "met: at least 20% fewer missed-evidence findings"
	case high < 0:
		missed = "not met: treatment missed more evidence than control"
	case low > 0:
		missed = "below target but interval excludes 0"
	case high >= targetDelta:
		missed = "consistent with 20% and with 0"
	}
	result := map[string]any{"missed_evidence": missed, "delta": round(delta), "kill_readings": killReadings(high)}
	if fraction, ok := number(summary["citable"].(map[string]any)["fraction"]); ok {
		result["citable_material_hunks"] = verdictAtLeast(fraction, citableGate)
	} else {
		result["citable_material_hunks"] = notObserved
	}
	if rate, ok := number(summary["verifier"].(map[string]any)["incorrect_rate"]); ok {
		result["incorrect_hard_failures"] = verdictBelow(rate, hardFailureGate)
	} else {
		result["incorrect_hard_failures"] = notObserved
	}
	return result
}

// killGainBound and killSeededBound are the amended CRT-V0-008 kill bounds.
const (
	killGainBound   = 0.10
	killSeededBound = 0.05
	killGain        = "upper bound below 0.10 absolute: rules out a 20% gain"
	killSeeded      = "the map's evidence adds nothing beyond its worklist"
)

// killReadings is the amended CRT-V0-008 kill reading for one arm against
// control: an upper bound below 0.10 absolute rules out a 20% gain.
func killReadings(high float64) []string {
	if high < killGainBound {
		return []string{killGain}
	}
	return []string{}
}

// seededKillReading is the amended CRT-V0-008 kill reading of the
// seeded_vs_treatment contrast: an upper bound at or below 0.05.
func seededKillReading(high float64) (string, bool) {
	return killSeeded, high <= killSeededBound
}

func verdictAtLeast(value, gate float64) string {
	if value >= gate {
		return "met"
	}
	return "not met"
}

func verdictBelow(value, gate float64) string {
	if value < gate {
		return "met"
	}
	return "not met"
}

// bca is the bias-corrected accelerated bootstrap over the paired deltas. Its
// resampling stream is derived from the run's seed, so the interval is
// reproducible from the report alone.
func bca(values []float64, seed string) (float64, float64) {
	n := len(values)
	observed := mean(values)
	stream := seededStream(seed + "\x00bca")
	replicates := make([]float64, bootstrapResamples)
	for index := range replicates {
		sample := make([]float64, n)
		for position := range sample {
			sample[position] = values[stream()%uint64(n)]
		}
		replicates[index] = mean(sample)
	}
	sort.Float64s(replicates)
	below := 0
	for _, value := range replicates {
		if value < observed {
			below++
		}
	}
	fraction := float64(below) / float64(bootstrapResamples)
	if fraction <= 0 {
		fraction = 1 / float64(2*bootstrapResamples)
	}
	if fraction >= 1 {
		fraction = 1 - 1/float64(2*bootstrapResamples)
	}
	z0 := normalQuantile(fraction)
	jackknife := make([]float64, n)
	for index := range values {
		total, count := 0.0, 0
		for position, value := range values {
			if position != index {
				total += value
				count++
			}
		}
		jackknife[index] = total / float64(count)
	}
	jackMean := mean(jackknife)
	numerator, denominator := 0.0, 0.0
	for _, value := range jackknife {
		difference := jackMean - value
		numerator += difference * difference * difference
		denominator += difference * difference
	}
	acceleration := 0.0
	if denominator > 0 {
		acceleration = numerator / (6 * math.Pow(denominator, 1.5))
	}
	return replicates[percentileIndex(z0, acceleration, -1.959964)], replicates[percentileIndex(z0, acceleration, 1.959964)]
}

func percentileIndex(z0, acceleration, z float64) int {
	adjusted := z0 + (z0+z)/(1-acceleration*(z0+z))
	position := int(normalCDF(adjusted) * float64(bootstrapResamples))
	if position < 0 {
		position = 0
	}
	if position >= bootstrapResamples {
		position = bootstrapResamples - 1
	}
	return position
}

func normalCDF(value float64) float64 {
	return 0.5 * math.Erfc(-value/math.Sqrt2)
}

// normalQuantile is the Acklam rational approximation to the inverse normal.
func normalQuantile(probability float64) float64 {
	a := []float64{-3.969683028665376e+01, 2.209460984245205e+02, -2.759285104469687e+02, 1.383577518672690e+02, -3.066479806614716e+01, 2.506628277459239e+00}
	b := []float64{-5.447609879822406e+01, 1.615858368580409e+02, -1.556989798598866e+02, 6.680131188771972e+01, -1.328068155288572e+01}
	c := []float64{-7.784894002430293e-03, -3.223964580411365e-01, -2.400758277161838e+00, -2.549732539343734e+00, 4.374664141464968e+00, 2.938163982698783e+00}
	d := []float64{7.784695709041462e-03, 3.224671290700398e-01, 2.445134137142996e+00, 3.754408661907416e+00}
	const low, high = 0.02425, 1 - 0.02425
	switch {
	case probability < low:
		q := math.Sqrt(-2 * math.Log(probability))
		return (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	case probability > high:
		q := math.Sqrt(-2 * math.Log(1-probability))
		return -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	}
	q := probability - 0.5
	r := q * q
	return (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r + a[5]) * q / (((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r + 1)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func round(value float64) float64 { return math.Round(value*1e6) / 1e6 }

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	}
	return 0, false
}

// rescore reads a report back and rebuilds every derived field from the raw
// ones, reproducing the report's bytes unchanged.
func rescore(arguments []string) (*report, string, error) {
	var input, output string
	flags := flag.NewFlagSet("cem-trial score", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&input, "report", "", "report to rescore")
	flags.StringVar(&output, "output", "", "report path (default stdout)")
	if err := flags.Parse(arguments); err != nil {
		return nil, "", err
	}
	if input == "" || flags.NArg() != 0 {
		return nil, "", errors.New("score takes --report FILE and optionally --output FILE")
	}
	document, err := loadReport(input)
	if err != nil {
		return nil, "", err
	}
	document.Partial = false
	finalize(document)
	return document, output, nil
}

// stemBaselineBlock pairs every arm the report ran, control included, with
// the mechanical stem worklist on the same changes (CRT-V0-017): delta is the
// stem baseline's miss minus the arm's, positive when the arm found more of
// the withheld evidence. An arm beats the baseline only when it misses at
// least 20% less and the paired BCa interval excludes zero; every other
// outcome, an unestimable one included, reads "not beaten", and `statement`
// says so in one sentence.
func stemBaselineBlock(document *report, meta map[string]changeMeta, byChange map[string]map[string][]float64, errored map[string]bool, order []string) map[string]any {
	arms := map[string]any{}
	beaten := []string{}
	for _, arm := range document.Arms {
		pairs := []pair{}
		for _, id := range order {
			if errored[id] || len(byChange[id][arm]) == 0 {
				continue
			}
			pairs = append(pairs, pair{id: id, control: stemMiss(meta[id]), treatment: mean(byChange[id][arm]), stem: stemMiss(meta[id])})
		}
		block := map[string]any{"pairs_scored": len(pairs), "verdict": "not beaten: no scored pair"}
		arms[arm] = block
		if len(pairs) == 0 {
			continue
		}
		deltas := make([]float64, len(pairs))
		for index, item := range pairs {
			deltas[index] = item.control - item.treatment
		}
		stem, delta := mean(missOf(pairs, "stem")), mean(deltas)
		block["mean_miss"] = map[string]any{"stem_baseline": round(stem), arm: round(mean(missOf(pairs, "treatment")))}
		block["delta"] = round(delta)
		block["relative_reduction"] = notObserved
		if stem > 0 {
			block["relative_reduction"] = round(delta / stem)
		}
		if len(pairs) < 2 {
			block["delta_ci95"] = notObserved
			block["verdict"] = "not beaten: " + notEstimable
			continue
		}
		low, high := bca(deltas, document.Seed+"\x00stem_baseline\x00"+arm)
		block["delta_ci95"] = map[string]any{"low": round(low), "high": round(high), "resamples": bootstrapResamples, "method": "BCa"}
		switch {
		case stem > 0 && delta/stem >= relativeGate && low > 0:
			block["verdict"] = "beaten: at least 20% fewer missed-evidence findings than the stem baseline, interval excludes 0"
			beaten = append(beaten, arm)
		case high < 0:
			block["verdict"] = "not beaten: the arm missed more evidence than the stem baseline"
		default:
			block["verdict"] = "not beaten: below 20% fewer missed-evidence findings or interval includes 0"
		}
	}
	statement := "the stem baseline is not beaten by any arm"
	if len(beaten) != 0 {
		statement = "the stem baseline is beaten on missed evidence by " + strings.Join(beaten, ", ") +
			"; adjudicated false findings are NOT_OBSERVED, so the reviewer bar is not established"
	}
	return map[string]any{
		"definition": "mechanical naming-convention test guesses recorded by select; no agent, no Corvint",
		"arms":       arms, "beaten_by": beaten, "statement": statement,
	}
}

// separateOutcomes keeps the two outcomes CRT-V0-018 distinguishes apart:
// evidence discovery is the withheld-gold miss every block above scores,
// from the evidence each lane cited (used); adjudicated defect detection
// needs a human adjudicator's records, which no report carries, so it is
// NOT_OBSERVED and never inferred from discovery. Evidence an agent opened
// without citing is NOT_OBSERVED too: no agent adapter records its reads.
func separateOutcomes(document *report) map[string]any {
	arms := map[string]any{}
	for _, arm := range document.Arms {
		lanes, citing, cited := 0, 0, 0
		for _, item := range document.Lanes {
			if item.Arm != arm || laneFailed(item) {
				continue
			}
			lanes++
			if len(item.Citations) != 0 {
				citing++
			}
			cited += len(item.Citations)
		}
		arms[arm] = map[string]any{
			"evidence_discovery": map[string]any{"metric": "miss", "lanes": lanes, "lanes_citing": citing, "citations_used": cited},
			"evidence_opened":    notObserved,
			"defect_detection":   map[string]any{"adjudicated_defects": notObserved, "adjudicated_false_findings": notObserved},
		}
	}
	return map[string]any{
		"arms":             arms,
		"defect_detection": "NOT_OBSERVED: no adjudication records; scored apart from evidence discovery and never inferred from it",
	}
}
