package main

// Reporting beside the per-arm means: the frozen repository folds and the
// registration fields a first run is matched against, paired bootstrap
// differences between the retrieval arms and each baseline, per-arm wall
// time distributions, and the --summarize mode that re-reads reports and
// merges their arms by sample id so old reports keep summarising.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
)

const (
	bootstrapRounds   = 4000
	bootstrapSeed     = 1
	minLatencySamples = 30
	minRepoSamples    = 5
	maxReportBytes    = 256 << 20
	cacheCold         = "COLD_UNIQUE"
	cachePrimed       = "PRIMED_SHARED"
	cacheNone         = "NOT_APPLICABLE"
)

// folds is the frozen repository -> fold map (benchmarks/README.md, "Agent
// Retrieval Bench folds"): a greedy balance of each task's positive share.
// A repository outside it is `unassigned`; the map itself never changes.
var folds = map[string]string{
	"HypothesisWorks/hypothesis": "A", "astral-sh/ruff": "A", "caddyserver/caddy": "A", "gin-gonic/gin": "A",
	"huggingface/diffusers": "A", "ipython/ipython": "A", "microsoft/playwright": "A", "mockito/mockito": "A",
	"numpy/numpy": "A", "pytest-dev/pytest": "A", "python/mypy": "A", "scrapy/scrapy": "A", "vitejs/vite": "A",
	"vuejs/core":   "A",
	"clap-rs/clap": "B", "eslint/eslint": "B", "etcd-io/etcd": "B", "fastapi/fastapi": "B",
	"huggingface/transformers": "B", "pallets/click": "B", "pydantic/pydantic": "B", "pypa/pip": "B",
	"spring-projects/spring-boot": "B", "tokio-rs/tokio": "B", "tox-dev/tox": "B",
}

var armOrder = []string{"corvint", "context", "grep", "grep-ident", "bm25:all", "bm25:ident", "impact", "affected"}
var corvintVerbArms = map[string]bool{"corvint": true, "context": true, "impact": true, "affected": true}
var retrievalArms = []string{"corvint", "context"}
var baselineArms = []string{"grep", "grep-ident", "bm25:all", "bm25:ident"}
var pairedMetrics = []string{"recall@5", "recall@10", "recall@20", "mrr@k"}

func partition(repo string) string {
	if fold, known := folds[repo]; known {
		return fold
	}
	return "unassigned"
}

// foldMapDigest is the SHA-256 of the fold map as sorted `fold TAB repo LF`
// lines, so a registration can name the exact map it ran under.
func foldMapDigest() string {
	repos := make([]string, 0, len(folds))
	for repo := range folds {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	digest := sha256.New()
	for _, repo := range repos {
		fmt.Fprintf(digest, "%s\t%s\n", folds[repo], repo)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// allArms is the default selection; selectArms parses --arms, where `bm25`
// names both term sets.
func allArms() map[string]bool {
	selected := map[string]bool{}
	for _, name := range armOrder {
		selected[name] = true
	}
	return selected
}

func selectArms(list []string) (map[string]bool, error) {
	if len(list) == 0 {
		return allArms(), nil
	}
	known := allArms()
	selected := map[string]bool{}
	for _, name := range list {
		if name == "bm25" {
			selected["bm25:all"], selected["bm25:ident"] = true, true
			continue
		}
		if !known[name] {
			return nil, fmt.Errorf("--arms: unknown arm %q", name)
		}
		selected[name] = true
	}
	return selected, nil
}

func needsCorvint(selected map[string]bool) bool {
	for name := range corvintVerbArms {
		if selected[name] {
			return true
		}
	}
	return false
}

// cacheState observes the copy before a Corvint verb runs: an existing
// `.corvint/index` is the primed shared topology, its absence the cold one.
// The label describes what the harness provided, never an inferred hit.
func cacheState(root string) string {
	if _, err := statIndex(root); err == nil {
		return cachePrimed
	}
	return cacheCold
}

// registration names what a first run is matched against before it starts:
// the samples digest, the binary digest, the fold map digest, and the arms.
func registration(samplesDigest string, identity map[string]any, selected map[string]bool) map[string]any {
	arms := make([]string, 0, len(selected))
	for _, name := range armOrder {
		if selected[name] {
			arms = append(arms, name)
		}
	}
	return map[string]any{
		"samples_sha256":  samplesDigest,
		"corvint_sha256":  identity["sha256"],
		"fold_map_sha256": foldMapDigest(),
		"arms":            arms,
	}
}

func presentArms(reports []sampleReport) []string {
	present := make([]string, 0, len(armOrder))
	for _, name := range armOrder {
		for _, report := range reports {
			if _, ran := report.Arms[name]; ran {
				present = append(present, name)
				break
			}
		}
	}
	return present
}

// paired reports, per group and per retrieval-arm/baseline pair, the mean
// paired difference of recall@5/10/20 and MRR over the positive samples
// both arms answered, its bootstrap 95% interval (fixed seed, 4000 rounds,
// percentile method), win/loss/tie counts, and per-repository means for
// repositories with at least five samples. Recall is not a Bernoulli rate,
// so no Wilson interval is attached to it.
func paired(reports []sampleReport) map[string]any {
	groups := map[string][]sampleReport{}
	for _, report := range reports {
		if report.Stratum != "positive" {
			continue
		}
		task, fold := "task:"+report.TaskType, "fold:"+report.Partition
		for _, group := range []string{"all", task, fold, task + "/" + fold} {
			groups[group] = append(groups[group], report)
		}
	}
	result := map[string]any{}
	for group, members := range groups {
		block := map[string]any{}
		for _, retrieval := range retrievalArms {
			pairs := map[string]any{}
			for _, baseline := range baselineArms {
				if pair := pairedBlock(members, retrieval, baseline); len(pair) > 0 {
					pairs[baseline] = pair
				}
			}
			if len(pairs) > 0 {
				block[retrieval] = pairs
			}
		}
		if len(block) > 0 {
			result[group] = block
		}
	}
	return result
}

func pairedBlock(members []sampleReport, retrieval, baseline string) map[string]any {
	block := map[string]any{}
	for _, metric := range pairedMetrics {
		differences, repos := pairedDifferences(members, retrieval, baseline, metric)
		if len(differences) == 0 {
			continue
		}
		block[metric] = pairedSummary(differences, repos)
	}
	return block
}

func pairedDifferences(members []sampleReport, retrieval, baseline, metric string) ([]float64, []string) {
	differences := make([]float64, 0, len(members))
	repos := make([]string, 0, len(members))
	for _, member := range members {
		left, leftPresent := member.Metrics[retrieval][metric]
		right, rightPresent := member.Metrics[baseline][metric]
		if !leftPresent || !rightPresent {
			continue
		}
		differences = append(differences, left-right)
		repos = append(repos, member.Repo)
	}
	return differences, repos
}

func pairedSummary(differences []float64, repos []string) map[string]any {
	wins, losses, ties := 0, 0, 0
	perRepo := map[string][]float64{}
	for index, difference := range differences {
		switch {
		case difference > 0:
			wins++
		case difference < 0:
			losses++
		default:
			ties++
		}
		perRepo[repos[index]] = append(perRepo[repos[index]], difference)
	}
	low, high := bootstrap(differences)
	summary := map[string]any{
		"n": len(differences), "mean_diff": round(meanOf(differences)), "low": round(low), "high": round(high),
		"wins": wins, "losses": losses, "ties": ties,
	}
	repoMeans := map[string]any{}
	for repo, values := range perRepo {
		if len(values) >= minRepoSamples {
			repoMeans[repo] = map[string]any{"n": len(values), "mean_diff": round(meanOf(values))}
		}
	}
	if len(repoMeans) > 0 {
		summary["repos"] = repoMeans
	}
	return summary
}

func meanOf(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

// bootstrap is the percentile 95% interval of the mean over 4000 resamples
// from one fixed-seed generator per call, so the output is deterministic
// whatever order the blocks are computed in.
func bootstrap(differences []float64) (float64, float64) {
	generator := rand.New(rand.NewSource(bootstrapSeed))
	n := len(differences)
	means := make([]float64, bootstrapRounds)
	for round := range means {
		total := 0.0
		for draw := 0; draw < n; draw++ {
			total += differences[generator.Intn(n)]
		}
		means[round] = total / float64(n)
	}
	sort.Float64s(means)
	// Drop the same number of resamples from each tail: indexes k and B-1-k.
	tail := int(0.025 * bootstrapRounds)
	return means[tail], means[bootstrapRounds-1-tail]
}

// latency reports every arm's wall time per cache-state label as
// nearest-rank p50/p95, maximum, and median absolute deviation, never a
// mean, and NOT_RUN below thirty samples (benchmarks/README.md, latency
// protocol). Arms without a recorded wall time (older reports) are
// UNRECORDED.
func latency(reports []sampleReport) map[string]any {
	cells := map[string]map[string][]float64{}
	for _, name := range presentArms(reports) {
		cells[name] = map[string][]float64{}
	}
	for _, report := range reports {
		for name, answer := range report.Arms {
			if answer.WallMillis <= 0 {
				continue
			}
			label := answer.CacheState
			if label == "" {
				label = cacheNone
			}
			cells[name][label] = append(cells[name][label], answer.WallMillis)
			if answer.ColdMillis > 0 {
				cells[name]["OBSERVED_MISS"] = append(cells[name]["OBSERVED_MISS"], answer.ColdMillis)
			}
		}
	}
	result := map[string]any{}
	for name, labels := range cells {
		if len(labels) == 0 {
			labels["UNRECORDED"] = nil
		}
		distributions := map[string]any{}
		for label, walls := range labels {
			distributions[label] = distribution(walls)
		}
		result[name] = distributions
	}
	return result
}

func distribution(walls []float64) map[string]any {
	if len(walls) < minLatencySamples {
		return map[string]any{"n": len(walls), "state": "NOT_RUN"}
	}
	sorted := append([]float64(nil), walls...)
	sort.Float64s(sorted)
	median := nearestRank(sorted, 0.5)
	deviations := make([]float64, len(sorted))
	for index, value := range sorted {
		deviations[index] = math.Abs(value - median)
	}
	sort.Float64s(deviations)
	return map[string]any{
		"n":      len(sorted),
		"p50_ms": round(median), "p95_ms": round(nearestRank(sorted, 0.95)),
		"min_ms": round(sorted[0]), "max_ms": round(sorted[len(sorted)-1]), "mad_ms": round(nearestRank(deviations, 0.5)),
	}
}

func nearestRank(sorted []float64, percentile float64) float64 {
	rank := int(math.Ceil(percentile * float64(len(sorted))))
	return sorted[max(1, rank)-1]
}

// storedReport is the part of a written report --summarize reads back.
type storedReport struct {
	SamplesSHA256 string         `json:"samples_sha256"`
	Limit         int            `json:"limit"`
	Skipped       map[string]int `json:"skipped"`
	Corvint       map[string]any `json:"corvint"`
	Details       []sampleReport `json:"details"`
}

// resummarize merges the details of every --summarize report by sample id
// (a later report's arm replaces an earlier one's of the same name), keeps
// the first report's samples digest, limit, skips, and binary identity, and
// rebuilds every summary section. Reports over different samples files are
// refused, so a merged report never pairs arms across runs of different
// inputs.
func resummarize(configuration options) (map[string]any, error) {
	merged := map[string]*sampleReport{}
	order := make([]string, 0, 512)
	var first storedReport
	sources := make([]string, 0, len(configuration.summaries))
	for index, path := range configuration.summaries {
		data, err := readBounded(path, maxReportBytes)
		if err != nil {
			return nil, err
		}
		var stored storedReport
		if err := json.Unmarshal(data, &stored); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if index == 0 {
			first = stored
		}
		if stored.SamplesSHA256 != first.SamplesSHA256 {
			return nil, fmt.Errorf("%s is over different samples than %s", path, configuration.summaries[0])
		}
		digest := sha256.Sum256(data)
		sources = append(sources, hex.EncodeToString(digest[:]))
		for _, detail := range stored.Details {
			if name, known := unknownArm(detail); !known {
				return nil, fmt.Errorf("%s: sample %s has unknown arm %q", path, detail.ID, name)
			}
			existing, seen := merged[detail.ID]
			if !seen {
				copied := detail
				copied.Partition = partition(detail.Repo)
				if copied.Arms == nil {
					copied.Arms = map[string]arm{}
				}
				if copied.Metrics == nil {
					copied.Metrics = map[string]metrics{}
				}
				merged[detail.ID] = &copied
				order = append(order, detail.ID)
				continue
			}
			for name, answer := range detail.Arms {
				existing.Arms[name] = answer
				existing.Metrics[name] = detail.Metrics[name]
			}
		}
	}
	reports := make([]sampleReport, 0, len(order))
	for _, id := range order {
		reports = append(reports, *merged[id])
	}
	registered := registration(first.SamplesSHA256, first.Corvint, armSet(presentArms(reports)))
	registered["source_reports_sha256"] = sources
	return map[string]any{
		"profile":        profile,
		"samples_sha256": first.SamplesSHA256,
		"samples":        len(reports),
		"skipped":        first.Skipped,
		"limit":          first.Limit,
		"corvint":        first.Corvint,
		"registration":   registered,
		"arms":           summarize(reports, first.Limit),
		"paired":         paired(reports),
		"latency":        latency(reports),
		"details":        reports,
	}, nil
}

// unknownArm returns an arm name a stored detail carries outside
// armOrder; every summary section is keyed by armOrder, so such an arm has no
// place to be summarized.
func unknownArm(detail sampleReport) (string, bool) {
	known := armSet(armOrder)
	for name := range detail.Arms {
		if !known[name] {
			return name, false
		}
	}
	return "", true
}

func armSet(names []string) map[string]bool {
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	return selected
}
