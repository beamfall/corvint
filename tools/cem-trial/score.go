package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math"
	"sort"
	"strconv"
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
	ID           string   `json:"id"`
	Commit       string   `json:"commit"`
	BaseCommit   string   `json:"base_commit"`
	PatchSHA256  string   `json:"patch_sha256"`
	HunkCount    int      `json:"hunk_count"`
	Gold         []string `json:"gold"`
	StemBaseline []string `json:"stem_baseline"`
}

type lane struct {
	ID               string         `json:"id"`
	Arm              string         `json:"arm"`
	Repeat           int            `json:"repeat"`
	ArmOrder         []string       `json:"arm_order"`
	ReplyState       string         `json:"reply_state"`
	Citations        []citation     `json:"citations"`
	Unknown          []string       `json:"unknown"`
	Miss             float64        `json:"miss"`
	GoldHits         []string       `json:"gold_hits"`
	StemBaselineHits []string       `json:"stem_baseline_hits"`
	CEM              map[string]any `json:"cem,omitempty"`
	WallMs           any            `json:"wall_ms"`
	Tokens           any            `json:"tokens"`
	ToolCalls        any            `json:"tool_calls,omitempty"`
	ExitCode         any            `json:"exit_code"`
	Reply            string         `json:"reply"`
	ReplyTruncated   bool           `json:"reply_truncated"`
	PromptSHA256     string         `json:"prompt_sha256"`
	PromptBytes      int            `json:"prompt_bytes"`
	ReusedFrom       string         `json:"reused_from,omitempty"`
	Error            string         `json:"error,omitempty"`
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
	Summary        map[string]any            `json:"summary,omitempty"`
	Invalid        string                    `json:"invalid,omitempty"`
	Partial        bool                      `json:"partial,omitempty"`
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
	document.Invalid = ""
	if failed := erroredLanes(document); failed > maxErroredLanes {
		document.Invalid = strconv.Itoa(failed) + " lanes errored, more than the " +
			strconv.Itoa(maxErroredLanes) + " CRT-V0-010 allows: the run is discarded, not re-scored"
	}
}

func scoreLane(item *lane, meta changeMeta) {
	citations, state := extractCitations(item.Reply)
	item.ReplyState, item.Citations, item.Unknown = state, citations, extractUnknown(item.Reply)
	if item.Citations == nil {
		item.Citations = []citation{}
	}
	cited := map[string]bool{}
	for _, one := range item.Citations {
		cited[normalizePath(one.Path)] = true
	}
	stem := map[string]bool{}
	for _, one := range meta.StemBaseline {
		stem[normalizePath(one)] = true
	}
	item.GoldHits, item.StemBaselineHits = []string{}, []string{}
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
	}
	sort.Strings(item.GoldHits)
	sort.Strings(item.StemBaselineHits)
	if len(meta.Gold) == 0 {
		item.Miss = 0
		return
	}
	item.Miss = round(float64(missed) / float64(len(meta.Gold)))
}

// pair is one change's mean miss per arm, over the repeats of that arm.
type pair struct {
	id                 string
	control, treatment float64
	stem               float64
}

// summarize builds the paired estimate. A change with any errored lane is
// dropped whole, so the arms always cover the same changes (CRT-V0-007).
func summarize(document *report, meta map[string]changeMeta) map[string]any {
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
	pairs := []pair{}
	for _, id := range order {
		arms := byChange[id]
		if errored[id] || len(arms["control"]) == 0 || len(arms["treatment"]) == 0 {
			continue
		}
		pairs = append(pairs, pair{
			id: id, control: mean(arms["control"]), treatment: mean(arms["treatment"]),
			stem: stemMiss(meta[id]),
		})
	}
	summary := map[string]any{
		"pairs_scored": len(pairs), "pairs_dropped": len(order) - len(pairs),
		"errored_lanes": erroredLanes(document),
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
	summary["delta"] = round(delta)
	summary["relative_reduction"] = round(relative)
	summary["mcnemar"] = mcnemar(pairs)
	summary["citable"] = citable(document)
	summary["verifier"] = verifierRates(document)
	if len(pairs) < 2 {
		// One pair has no resampling variability: its "interval" would be the
		// delta itself and could read as met (CRT-V0-008).
		summary["delta_ci95"] = notObserved
		summary["verdict"] = verdicts(delta, relative, 0, 0, controlMiss, summary)
		summary["verdict"].(map[string]any)["missed_evidence"] = notEstimable
		return summary
	}
	low, high := bca(deltas, document.Seed)
	summary["delta_ci95"] = map[string]any{"low": round(low), "high": round(high), "resamples": bootstrapResamples, "method": "BCa"}
	summary["verdict"] = verdicts(delta, relative, low, high, controlMiss, summary)
	return summary
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
	supported, material, lanes := 0.0, 0.0, 0
	for _, item := range document.Lanes {
		if item.Arm != "treatment" || laneFailed(item) || item.CEM == nil {
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
	citing, failures, incorrect := 0, 0, 0
	for _, item := range document.Lanes {
		if item.Arm != "treatment" || laneFailed(item) || item.CEM == nil || len(item.Citations) == 0 {
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

// verdicts states each target in the wording CRT-V0-008 fixes: 30 pairs is an
// estimation run, so the missed-evidence gate is met only when the point
// estimate reaches the target and the interval excludes zero.
func verdicts(delta, relative, low, high, controlMiss float64, summary map[string]any) map[string]any {
	// The bootstrap interval is in absolute deltas, so scale the relative target.
	targetDelta := relativeGate * controlMiss
	missed := "consistent with 0 but not with 20%"
	switch {
	case relative >= relativeGate && low > 0:
		missed = "met: at least 20% fewer missed-evidence findings"
	case high < 0:
		missed = "not met: treatment missed more evidence than control"
	case low > 0:
		missed = "below target but interval excludes 0"
	case low <= 0 && high >= 0 && low <= targetDelta && high >= targetDelta:
		missed = "consistent with 20% and with 0"
	}
	result := map[string]any{"missed_evidence": missed, "delta": round(delta)}
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
