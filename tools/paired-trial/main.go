// Command paired-trial analyses a paired agent trial offline (AHI-217, V1-2756). It reads the
// per-session receipts of an "off" and an "on" arm, pairs them by task and replicate, and reports
// per-pair cost and resolution deltas, a percentile bootstrap interval on the pooled cost ratio
// on/off, and an exact McNemar test on resolution. It runs no model, reads only its input file
// and writes only the report to standard output.
//
// A receipt that lacks the injected frame, the per-step ledger or the grader's per-test results is
// not dropped: the field is reported NOT_OBSERVED, and --require-complete refuses the input.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"sort"
)

const (
	receiptProfile = "corvint-paired-trial-receipt/0"
	reportProfile  = "corvint-paired-trial-analysis/0"
)

// Receipt is one agent session of one arm on one task.
type Receipt struct {
	Profile   string  `json:"profile"`
	Task      string  `json:"task"`
	Replicate int     `json:"replicate"`
	Arm       string  `json:"arm"`
	Resolved  *bool   `json:"resolved"`
	CostUSD   float64 `json:"cost_usd"`
	// Frame is the exact injected context: its byte count and SHA-256, and optionally the stored
	// file. An arm that injects nothing records {"bytes":0}. A missing frame is NOT_OBSERVED.
	Frame  *Frame  `json:"frame"`
	Steps  []Step  `json:"steps"`
	Grader *Grader `json:"grader"`
}

type Frame struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256,omitempty"`
	Path   string `json:"path,omitempty"`
	// BaseTree is the tree the frame was probed on; it must be the task's base, not the edited checkout.
	BaseTree string `json:"base_tree,omitempty"`
}

// Step is one model turn of the per-step ledger.
type Step struct {
	Tool         string  `json:"tool"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read_tokens"`
	CacheWrite   int     `json:"cache_write_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type Grader struct {
	Tests []TestResult `json:"tests"`
}

type TestResult struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
}

type pairRow struct {
	Task         string  `json:"task"`
	Replicate    int     `json:"replicate"`
	OffCostUSD   float64 `json:"off_cost_usd"`
	OnCostUSD    float64 `json:"on_cost_usd"`
	CostDeltaUSD float64 `json:"cost_delta_usd"`
	OffResolved  bool    `json:"off_resolved"`
	OnResolved   bool    `json:"on_resolved"`
	Flip         string  `json:"flip"`
}

type report struct {
	Profile          string         `json:"profile"`
	Pairs            []pairRow      `json:"pairs"`
	Unpaired         []string       `json:"unpaired"`
	CostRatio        float64        `json:"cost_ratio_on_over_off"`
	CostRatioCI      [2]float64     `json:"cost_ratio_ci"`
	CI               float64        `json:"ci_level"`
	Resamples        int            `json:"bootstrap_resamples"`
	Seed             uint64         `json:"bootstrap_seed"`
	McNemar          mcnemar        `json:"mcnemar"`
	NotObserved      map[string]int `json:"not_observed"`
	LedgerMismatches []string       `json:"ledger_cost_mismatches"`
}

type mcnemar struct {
	OnOnly  int     `json:"on_only"`
	OffOnly int     `json:"off_only"`
	Both    int     `json:"both"`
	Neither int     `json:"neither"`
	PExact  float64 `json:"p_exact_two_sided"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "paired-trial:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("paired-trial", flag.ContinueOnError)
	input := fs.String("receipts", "", "JSON Lines file of "+receiptProfile+" receipts")
	resamples := fs.Int("resamples", 10000, "bootstrap resamples")
	seed := fs.Uint64("seed", 1, "bootstrap seed")
	level := fs.Float64("ci", 0.95, "interval level")
	complete := fs.Bool("require-complete", false, "refuse receipts missing the frame, step ledger or grader tests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *resamples < 100 || *level <= 0 || *level >= 1 {
		return errors.New("--receipts is required, --resamples must be at least 100 and --ci in (0,1)")
	}
	receipts, err := readReceipts(*input)
	if err != nil {
		return err
	}
	rep, err := analyse(receipts, *resamples, *seed, *level)
	if err != nil {
		return err
	}
	if *complete {
		for field, n := range rep.NotObserved {
			if n > 0 {
				return fmt.Errorf("%d receipt(s) do not record %s", n, field)
			}
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", " ")
	return enc.Encode(rep)
}

func readReceipts(path string) ([]Receipt, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var receipts []Receipt
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var r Receipt
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if r.Profile != receiptProfile || r.Task == "" || (r.Arm != "on" && r.Arm != "off") || r.Resolved == nil || r.CostUSD < 0 {
			return nil, fmt.Errorf("line %d: want profile %s with task, arm on|off, resolved and a non-negative cost_usd", line, receiptProfile)
		}
		receipts = append(receipts, r)
	}
	return receipts, scanner.Err()
}

func analyse(receipts []Receipt, resamples int, seed uint64, level float64) (report, error) {
	rep := report{Profile: reportProfile, CI: level, Resamples: resamples, Seed: seed, Unpaired: []string{}, LedgerMismatches: []string{},
		NotObserved: map[string]int{"frame": 0, "frame_base_tree": 0, "steps": 0, "grader_tests": 0}}
	type key struct {
		task      string
		replicate int
	}
	arms := map[key]map[string]Receipt{}
	for _, r := range receipts {
		k := key{r.Task, r.Replicate}
		if arms[k] == nil {
			arms[k] = map[string]Receipt{}
		}
		if _, dup := arms[k][r.Arm]; dup {
			return rep, fmt.Errorf("task %s replicate %d has two %s receipts", r.Task, r.Replicate, r.Arm)
		}
		arms[k][r.Arm] = r
		if r.Frame == nil {
			rep.NotObserved["frame"]++
		} else if r.Frame.Bytes > 0 && r.Frame.BaseTree == "" {
			rep.NotObserved["frame_base_tree"]++
		}
		if len(r.Steps) == 0 {
			rep.NotObserved["steps"]++
		} else {
			sum := 0.0
			for _, s := range r.Steps {
				sum += s.CostUSD
			}
			if math.Abs(sum-r.CostUSD) > 1e-6 {
				rep.LedgerMismatches = append(rep.LedgerMismatches, fmt.Sprintf("%s#%d/%s: steps %.6f != session %.6f", r.Task, r.Replicate, r.Arm, sum, r.CostUSD))
			}
		}
		if r.Grader == nil || len(r.Grader.Tests) == 0 {
			rep.NotObserved["grader_tests"]++
		}
	}
	keys := make([]key, 0, len(arms))
	for k := range arms {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].task != keys[j].task {
			return keys[i].task < keys[j].task
		}
		return keys[i].replicate < keys[j].replicate
	})
	for _, k := range keys {
		off, okOff := arms[k]["off"]
		on, okOn := arms[k]["on"]
		if !okOff || !okOn {
			rep.Unpaired = append(rep.Unpaired, fmt.Sprintf("%s#%d", k.task, k.replicate))
			continue
		}
		row := pairRow{Task: k.task, Replicate: k.replicate, OffCostUSD: off.CostUSD, OnCostUSD: on.CostUSD,
			CostDeltaUSD: on.CostUSD - off.CostUSD, OffResolved: *off.Resolved, OnResolved: *on.Resolved}
		switch {
		case row.OnResolved && row.OffResolved:
			row.Flip, rep.McNemar.Both = "both", rep.McNemar.Both+1
		case row.OnResolved:
			row.Flip, rep.McNemar.OnOnly = "on-only", rep.McNemar.OnOnly+1
		case row.OffResolved:
			row.Flip, rep.McNemar.OffOnly = "off-only", rep.McNemar.OffOnly+1
		default:
			row.Flip, rep.McNemar.Neither = "neither", rep.McNemar.Neither+1
		}
		rep.Pairs = append(rep.Pairs, row)
	}
	if len(rep.Pairs) == 0 {
		return rep, errors.New("no task has both an off and an on receipt")
	}
	rep.McNemar.PExact = mcnemarExact(rep.McNemar.OnOnly, rep.McNemar.OffOnly)
	ratio, ok := costRatio(rep.Pairs)
	if !ok {
		return rep, errors.New("the off arm's total cost is zero, so the cost ratio is undefined")
	}
	rep.CostRatio = ratio
	rep.CostRatioCI = bootstrapRatio(rep.Pairs, resamples, seed, level)
	return rep, nil
}

func costRatio(pairs []pairRow) (float64, bool) {
	on, off := 0.0, 0.0
	for _, p := range pairs {
		on += p.OnCostUSD
		off += p.OffCostUSD
	}
	return on / off, off > 0
}

// bootstrapRatio is the percentile interval of the pooled ratio over pairs resampled with
// replacement; a resample whose off total is zero is skipped and counts against no tail.
func bootstrapRatio(pairs []pairRow, resamples int, seed uint64, level float64) [2]float64 {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	ratios := make([]float64, 0, resamples)
	sample := make([]pairRow, len(pairs))
	for range resamples {
		for i := range sample {
			sample[i] = pairs[rng.IntN(len(pairs))]
		}
		if r, ok := costRatio(sample); ok {
			ratios = append(ratios, r)
		}
	}
	sort.Float64s(ratios)
	tail := (1 - level) / 2
	at := func(q float64) float64 {
		i := int(math.Floor(q * float64(len(ratios)-1)))
		return ratios[max(0, min(len(ratios)-1, i))]
	}
	return [2]float64{at(tail), at(1 - tail)}
}

// mcnemarExact is the two-sided exact binomial p-value on the discordant pairs, capped at 1.
func mcnemarExact(b, c int) float64 {
	n := b + c
	if n == 0 {
		return 1
	}
	k := min(b, c)
	p := 0.0
	for i := 0; i <= k; i++ {
		p += math.Exp(lchoose(n, i) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*p)
}

func lchoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}
