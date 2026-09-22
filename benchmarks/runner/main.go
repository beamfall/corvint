// Command benchmark-runner executes Corvint's pinned multi-repository retrieval benchmark.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/evalrepo"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const learnedTraceFixtureSHA256 = "3955230be6d3ec4e0ff877786efe5d4d3f4dd77e2d8a2162ef065d36d2e09d81"

// splitManifestSHA256 registers benchmarks/eval-split-v0.json (REC-V0-002).
const splitManifestSHA256 = "956125d981645c64500178a75b4662e51a00db282ec8ff39638ea0c5887d8eea"

var expectedStates = set("READY", "OUT_OF_SCOPE", "NEEDS_WIDENING")
var selectorKinds = set("decision", "feature", "learned-path", "path", "reference", "reverse-import", "scenario", "spec", "symbol", "test")

var firstRunEvidence = map[string]map[string]any{
	"1365eec079f861d8abe03123afdeefa4bd9da23f51bb2c7efce4366f553c87b1": {
		"partition": "heldout-v2", "artifact": "corvint-v4-blind-v2-first-run.json",
		"artifact_sha256":  "6cd5bd34b0d0be2b7bceaa1f0bb44a69ed03bc36be1217862bff2a51cf28a356",
		"corvint_identity": "working-tree:f6b6c6b564667652c6396f65e88f395493e23176e7b6ccce13def07902f76ad3",
		"aggregate":        map[string]any{"cases": 5, "recall": .071429, "critical_evidence_misses": 7, "serialized_result_byte_weighted_precision": .182141, "top5_task_success": .4, "abstention_accuracy": 0.0, "epistemic_state_accuracy": 0.0},
	},
	"4fb67a876e35090f3fd454099311963fdbe82df1aab63d042c345503bb32a10d": {
		"partition": "heldout-v3", "first_observed_at": "2026-08-22", "artifact": "corvint-v4-blind-v3-first-run.json",
		"artifact_sha256":  "5b50ff71aa639d695e352be8967f5c9bb7e644d59a08cc0c24ffa048a614d0b2",
		"corvint_identity": "working-tree:7727bd8ce38b8c28ce987e95cadd73e58c2f9d34e0d5e801daf768a8baddf27a",
		"aggregate":        map[string]any{"cases": 5, "recall": .181818, "critical_evidence_misses": 8, "serialized_result_byte_weighted_precision": .258638, "top5_task_success": .6, "abstention_accuracy": 0.0, "epistemic_state_accuracy": 0.0},
	},
}

type repository struct {
	ID, URL, Commit, Language, Cases string
	Public                           bool
}

type manifest struct {
	SchemaVersion int          `json:"schema_version"`
	Repositories  []repository `json:"repositories"`
}

type options struct {
	manifest, cache, output string
	checkouts               map[string]string
	prepare, enforce        bool
	repos                   []string
	purpose                 evalrepo.Purpose
}

type stringFlags []string

func (values *stringFlags) String() string         { return strings.Join(*values, ",") }
func (values *stringFlags) Set(value string) error { *values = append(*values, value); return nil }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root, err := corvintRoot()
	if err != nil {
		fail(err)
	}
	var checkout stringFlags
	opts := options{checkouts: map[string]string{}}
	flag.StringVar(&opts.manifest, "manifest", filepath.Join(root, "benchmarks", "manifest.json"), "benchmark manifest")
	flag.Var(&checkout, "checkout", "REPOSITORY=PATH")
	flag.StringVar(&opts.cache, "cache", filepath.Join(root, ".corvint-benchmark-cache"), "checkout cache")
	flag.BoolVar(&opts.prepare, "prepare", false, "fetch missing public repositories")
	flag.StringVar(&opts.output, "output", "", "write report atomically")
	flag.BoolVar(&opts.enforce, "enforce-v4", false, "exit 1 unless the V4 gate passes")
	repos := flag.String("repos", "", "comma-separated repository IDs")
	purpose := flag.String("purpose", string(evalrepo.PurposeScore), "score reads every row; tune reads dev rows only")
	writeSplit := flag.String("write-split-manifest", "", "write the split manifest for the manifest's corpora and exit")
	flag.Parse()
	if flag.NArg() != 0 {
		fail(fmt.Errorf("unrecognized arguments: %s", strings.Join(flag.Args(), " ")))
	}
	if *writeSplit != "" {
		digest, err := writeSplitManifest(root, opts.manifest, *writeSplit)
		if err != nil {
			fail(err)
		}
		fmt.Println(digest)
		return
	}
	opts.purpose = evalrepo.Purpose(*purpose)
	for _, value := range checkout {
		name, path, ok := strings.Cut(value, "=")
		if !ok || name == "" || path == "" || opts.checkouts[name] != "" {
			fail(errors.New("--checkout must be a unique REPOSITORY=PATH pair"))
		}
		resolved, resolveErr := filepath.Abs(path)
		if resolveErr != nil {
			fail(resolveErr)
		}
		opts.checkouts[name] = resolved
	}
	if *repos != "" {
		for _, id := range strings.Split(*repos, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				fail(errors.New("--repos must be a comma-separated list of repository ids"))
			}
			opts.repos = append(opts.repos, id)
		}
		if len(set(opts.repos...)) != len(opts.repos) {
			fail(errors.New("--repos contains duplicate repository ids"))
		}
	}
	result, err := run(ctx, root, opts)
	if err != nil {
		fail(err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	encoded = append(encoded, '\n')
	if opts.output == "" {
		_, err = os.Stdout.Write(encoded)
	} else {
		err = atomicWrite(opts.output, encoded)
	}
	if err != nil {
		fail(err)
	}
	if opts.enforce && !result["v4_release"].(map[string]any)["ready"].(bool) {
		os.Exit(1)
	}
}

func fail(err error) {
	encoded, _ := json.Marshal(map[string]any{"error": err.Error(), "ok": false})
	_, _ = fmt.Fprintln(os.Stderr, string(encoded))
	os.Exit(2)
}

func corvintRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(executable), "..", ".."))
	if _, err = os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return root, nil
	}
	working, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for candidate := working; ; candidate = filepath.Dir(candidate) {
		if _, statErr := os.Stat(filepath.Join(candidate, "go.mod")); statErr == nil {
			return candidate, nil
		}
		if filepath.Dir(candidate) == candidate {
			return "", errors.New("cannot locate Corvint repository root")
		}
	}
}

func run(ctx context.Context, corvintRoot string, opts options) (map[string]any, error) {
	manifestPath, _ := filepath.Abs(opts.manifest)
	m, raw, err := loadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	digest := sha(raw)
	firstRun := firstRunEvidence[digest]
	releaseManifest := samePath(manifestPath, filepath.Join(corvintRoot, "benchmarks", "manifest.json"))
	selected := set()
	for _, id := range opts.repos {
		selected[id] = struct{}{}
	}
	if opts.repos == nil {
		for _, entry := range m.Repositories {
			selected[entry.ID] = struct{}{}
		}
	}
	known := set()
	for _, entry := range m.Repositories {
		known[entry.ID] = struct{}{}
	}
	for id := range selected {
		if _, ok := known[id]; !ok {
			return nil, fmt.Errorf("--repos references unknown repository ids: %s", id)
		}
	}
	var skipped []string
	for id := range known {
		if _, ok := selected[id]; !ok {
			skipped = append(skipped, id)
		}
	}
	sort.Strings(skipped)
	fixturePath := filepath.Join(corvintRoot, "benchmarks", "learned-trace-fixture-v0.json")
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read learned trace fixture: %w", err)
	}
	if sha(fixtureBytes) != learnedTraceFixtureSHA256 {
		return nil, errors.New("learned trace fixture digest does not match the registered digest")
	}
	splitPath := filepath.Join(corvintRoot, "benchmarks", "eval-split-v0.json")
	splits, err := loadSplitManifest(splitPath)
	if err != nil {
		return nil, err
	}
	purpose := choose(opts.purpose == "", evalrepo.PurposeScore, opts.purpose)
	var repositories []any
	globalIDs := set()
	learnedContracts := map[string]map[string]int{}
	for _, entry := range m.Repositories {
		if _, ok := selected[entry.ID]; !ok {
			continue
		}
		checkout, resolveErr := resolveCheckout(ctx, entry, opts)
		if resolveErr != nil {
			return nil, resolveErr
		}
		casesPath := entry.Cases
		if !filepath.IsAbs(casesPath) {
			casesPath = filepath.Join(corvintRoot, casesPath)
		}
		payload, index, validateErr := validateCases(ctx, entry, checkout, casesPath)
		if validateErr != nil {
			return nil, validateErr
		}
		caseBytes, readErr := os.ReadFile(casesPath)
		if readErr != nil {
			return nil, readErr
		}
		registered, splitErr := splits.VerifyCorpus(entry.Cases, caseBytes)
		if splitErr != nil {
			return nil, splitErr
		}
		if releaseManifest && !registered {
			return nil, fmt.Errorf("split manifest does not register release corpus %s", entry.Cases)
		}
		payload["cases"] = readableCases(maps(payload["cases"]), purpose)
		before := sha(fixtureBytes)
		evaluation, comparability, evalErr := evalrepo.EvaluateComparable(ctx, checkout, casesPath, purpose, fixturePath)
		if evalErr != nil {
			return nil, evalErr
		}
		comparability["split_manifest"] = choose(registered, "registered", "unregistered")
		afterBytes, readErr := os.ReadFile(fixturePath)
		if readErr != nil {
			return nil, fmt.Errorf("cannot reread learned trace fixture: %w", readErr)
		}
		if sha(afterBytes) != before {
			return nil, errors.New("learned trace fixture changed during evaluation")
		}
		learned, _ := evaluation["learned_trace_arm"].(map[string]any)
		expectedFixture := map[string]any{"path": fixturePath, "sha256": before}
		if !equalJSON(learned["trace_fixture"], expectedFixture) {
			return nil, errors.New("reported learned trace fixture does not match verified input")
		}
		pinnedTree, gitErr := git(ctx, checkout, "rev-parse", entry.Commit+"^{tree}")
		if gitErr != nil {
			return nil, gitErr
		}
		if stringValue(evaluation["revision"]) != pinnedTree {
			return nil, fmt.Errorf("evaluation revision does not match manifest for %s", entry.ID)
		}
		contract, auditErr := auditEvaluation(payload, evaluation)
		if auditErr != nil {
			return nil, auditErr
		}
		learnedContract, learnedErr := auditEvaluation(payload, learned)
		if learnedErr != nil {
			return nil, fmt.Errorf("learned trace arm %w", learnedErr)
		}
		learnedContracts[entry.ID] = learnedContract
		cases := maps(evaluation["cases"])
		declaredCounts, evaluationCounts := map[string]int{}, map[string]int{}
		for i, item := range maps(payload["cases"]) {
			id := stringValue(item["id"])
			if _, duplicate := globalIDs[id]; duplicate {
				return nil, fmt.Errorf("case id is not globally unique: %s", id)
			}
			globalIDs[id] = struct{}{}
			declared := stringDefault(item["partition"], "development")
			status := evaluationPartition(declared, firstRun)
			declaredCounts[declared]++
			evaluationCounts[status]++
			cases[i]["declared_partition"] = declared
			cases[i]["evaluation_partition"] = status
			cases[i]["split"] = evalrepo.CaseSplit(id)
			cases[i]["downstream_outcome"] = evalrepo.NotRecordedOutcome()
		}
		corvint := map[string]any{"state": evaluation["state"], "revision": evaluation["revision"], "metrics": evaluation["metrics"], "cases": evaluation["cases"], "learned_trace_arm": learned, "learned_trace_delta": evaluation["learned_trace_delta"]}
		baseline := exactBaseline(index, payload)
		repositories = append(repositories, map[string]any{"id": entry.ID, "url": entry.URL, "commit": entry.Commit, "language": entry.Language, "public": entry.Public, "corpus_sha256": sha(caseBytes), "partitions": map[string]any{"declared": declaredCounts, "evaluation": evaluationCounts}, "contract": contract, "corvint": corvint, "baseline": baseline, "comparability": comparability})
	}
	baselineAggregate := aggregate(repositories, skipped)
	checks := thresholds(baselineAggregate)
	learnedRepositories := make([]any, 0, len(repositories))
	changed := map[string]int{}
	for _, rawRepo := range repositories {
		repo := rawRepo.(map[string]any)
		corvint := repo["corvint"].(map[string]any)
		learned := corvint["learned_trace_arm"].(map[string]any)
		copyRepo := cloneMap(repo)
		copyRepo["corvint"] = map[string]any{"state": learned["state"], "metrics": learned["metrics"]}
		copyRepo["contract"] = learnedContracts[stringValue(repo["id"])]
		learnedRepositories = append(learnedRepositories, copyRepo)
		for _, row := range maps(corvint["learned_trace_delta"]) {
			changed[stringValue(row["metric"])] += integer(row["case_change_count"])
		}
	}
	learnedAggregate := aggregate(learnedRepositories, skipped)
	learnedReport := learnedTraceReport(baselineAggregate, learnedAggregate, changed)
	learnedReport["aggregate"] = learnedAggregate
	checks["learned_trace_gate"] = learnedReport["ready"].(bool)
	if purpose != evalrepo.PurposeScore {
		checks["score_purpose"] = false
	}
	releaseReady := releaseManifest && allTrue(checks)
	comparability, err := comparabilityReport(repositories, purpose, splitPath)
	if err != nil {
		return nil, err
	}
	if !releaseManifest {
		checks["release_manifest"] = false
	}
	allDeclared, allEvaluation := map[string]int{}, map[string]int{}
	for _, rawRepo := range repositories {
		partitions := rawRepo.(map[string]any)["partitions"].(map[string]any)
		addCounts(allDeclared, partitions["declared"].(map[string]int))
		addCounts(allEvaluation, partitions["evaluation"].(map[string]int))
	}
	identity, err := git(ctx, corvintRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	return map[string]any{"schema_version": 1, "corvint_commit": identity, "manifest_sha256": digest, "evaluation_engine": map[string]any{"name": "native-go", "learned_trace_fixture": map[string]any{"path": fixturePath, "sha256": learnedTraceFixtureSHA256}}, "provenance": map[string]any{"track": choose(releaseManifest, "v4-release-development", "challenge-replay"), "declared_partitions": allDeclared, "evaluation_partitions": allEvaluation, "first_run_evidence": firstRun}, "repositories": repositories, "aggregate": baselineAggregate, "comparability": comparability, "v4_release": map[string]any{"applicable": releaseManifest, "ready": releaseReady, "checks": checks, "blocked_by": failedKeys(checks)}, "learned_trace": learnedReport}, nil
}

func loadSplitManifest(path string) (evalrepo.SplitManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return evalrepo.SplitManifest{}, fmt.Errorf("cannot read split manifest: %w", err)
	}
	if sha(raw) != splitManifestSHA256 {
		return evalrepo.SplitManifest{}, errors.New("split manifest digest does not match the registered digest")
	}
	var splits evalrepo.SplitManifest
	if err := json.Unmarshal(raw, &splits); err != nil {
		return evalrepo.SplitManifest{}, fmt.Errorf("invalid split manifest: %w", err)
	}
	return splits, nil
}

// writeSplitManifest derives the split manifest from the manifest's corpora.
func writeSplitManifest(corvintRoot, manifestPath, output string) (string, error) {
	m, _, err := loadManifest(manifestPath)
	if err != nil {
		return "", err
	}
	corpora := map[string][]byte{}
	for _, entry := range m.Repositories {
		raw, readErr := os.ReadFile(filepath.Join(corvintRoot, entry.Cases))
		if readErr != nil {
			return "", readErr
		}
		corpora[entry.Cases] = raw
	}
	splits, err := evalrepo.NewSplitManifest(corpora)
	if err != nil {
		return "", err
	}
	encoded, err := json.MarshalIndent(splits, "", "  ")
	if err != nil {
		return "", err
	}
	encoded = append(encoded, '\n')
	return sha(encoded), atomicWrite(output, encoded)
}

// readableCases keeps the rows the purpose may read (REC-V0-004).
func readableCases(cases []map[string]any, purpose evalrepo.Purpose) []any {
	readable := make([]any, 0, len(cases))
	for _, item := range cases {
		if purpose == evalrepo.PurposeScore || evalrepo.CaseSplit(stringValue(item["id"])) == evalrepo.SplitDev {
			readable = append(readable, item)
		}
	}
	return readable
}

// comparabilityReport sums the per-corpus splits side by side and validates
// every case's downstream outcome slot (REC-V0-003, REC-V0-006).
func comparabilityReport(repositories []any, purpose evalrepo.Purpose, splitPath string) (map[string]any, error) {
	parts := map[string][]map[string]any{}
	outcomes := map[string]int{}
	withheld := 0
	for _, rawRepo := range repositories {
		repo := rawRepo.(map[string]any)
		block := repo["comparability"].(map[string]any)
		withheld += integer(block["heldout_rows_withheld"])
		for name, metrics := range block["splits"].(map[string]any) {
			parts[name] = append(parts[name], metrics.(map[string]any))
		}
		for _, item := range maps(repo["corvint"].(map[string]any)["cases"]) {
			if err := evalrepo.ValidateDownstreamOutcome(item["downstream_outcome"]); err != nil {
				return nil, fmt.Errorf("case %s: %w", stringValue(item["id"]), err)
			}
			outcomes[stringValue(item["downstream_outcome"].(map[string]any)["state"])]++
		}
	}
	splits := map[string]any{}
	for name, metrics := range parts {
		splits[name] = evalrepo.MergeSplitMetrics(metrics)
	}
	splitBytes, _ := os.ReadFile(splitPath)
	return map[string]any{
		"purpose": string(purpose), "split_manifest": map[string]any{"path": splitPath, "sha256": sha(splitBytes)},
		"split_rule": evalrepo.SplitRule, "heldout_provenance": evalrepo.HeldoutProvenance,
		"heldout_rows_withheld": withheld, "splits": splits, "downstream_outcomes": outcomes,
	}, nil
}

func loadManifest(path string) (manifest, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, nil, fmt.Errorf("invalid JSON %s: %w", path, err)
	}
	var m manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return manifest{}, nil, fmt.Errorf("invalid JSON %s: %w", path, err)
	}
	if m.SchemaVersion != 1 || m.Repositories == nil {
		return manifest{}, nil, errors.New("manifest requires schema_version 1 and repositories[]")
	}
	ids := set()
	for _, entry := range m.Repositories {
		if entry.ID == "" || entry.URL == "" || entry.Commit == "" || entry.Language == "" || entry.Cases == "" {
			return manifest{}, nil, errors.New("incomplete repository entry")
		}
		if len(entry.Commit) != 40 {
			return manifest{}, nil, fmt.Errorf("repository commit must be a full SHA: %s", entry.ID)
		}
		if _, ok := ids[entry.ID]; ok {
			return manifest{}, nil, errors.New("repository ids must be present and unique")
		}
		ids[entry.ID] = struct{}{}
	}
	return m, raw, nil
}

func resolveCheckout(ctx context.Context, entry repository, opts options) (string, error) {
	root := opts.checkouts[entry.ID]
	if root == "" {
		root = filepath.Join(opts.cache, entry.ID)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if !opts.prepare {
			return "", fmt.Errorf("missing checkout for %s: pass --checkout %s=PATH or use --prepare", entry.ID, entry.ID)
		}
		if !entry.Public {
			return "", fmt.Errorf("%s requires an explicit --checkout", entry.ID)
		}
		if entries, readErr := os.ReadDir(root); readErr == nil && len(entries) != 0 {
			return "", fmt.Errorf("benchmark cache path is not empty: %s", root)
		}
		if err = os.MkdirAll(root, 0o755); err != nil {
			return "", err
		}
		for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", entry.URL}, {"fetch", "--depth=1", "origin", entry.Commit}, {"checkout", "--detach", "-q", "FETCH_HEAD"}} {
			if _, err = git(ctx, root, args...); err != nil {
				return "", err
			}
		}
	}
	commit, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if commit != entry.Commit {
		return "", fmt.Errorf("%s checkout is %s, expected %s", entry.ID, commit, entry.Commit)
	}
	dirty, err := git(ctx, root, "status", "--porcelain=v1", "--untracked-files=no")
	if err != nil {
		return "", err
	}
	if dirty != "" {
		return "", fmt.Errorf("%s checkout has tracked changes", entry.ID)
	}
	return root, nil
}

func validateCases(ctx context.Context, entry repository, root, path string) (map[string]any, *contextindex.Index, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var payload map[string]any
	if err = json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON %s: %w", path, err)
	}
	if integer(payload["schemaVersion"]) != 1 || payload["cases"] == nil {
		return nil, nil, fmt.Errorf("%s requires schemaVersion 1 and cases[]", path)
	}
	if commit := stringDefault(payload["commit"], entry.Commit); commit != entry.Commit {
		return nil, nil, fmt.Errorf("case commit does not match manifest for %s", entry.ID)
	}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return nil, nil, err
	}
	if head != entry.Commit {
		return nil, nil, fmt.Errorf("case checkout does not match manifest for %s", entry.ID)
	}
	index, err := contextindex.BuildEval(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	symbols, records := set(), set()
	for _, symbol := range index.Symbols {
		symbols["symbol:"+symbol.Path+":"+symbol.Name] = struct{}{}
	}
	for id := range index.Features {
		records["feature:"+id] = struct{}{}
	}
	for id := range index.Scenarios {
		records["scenario:"+id] = struct{}{}
	}
	ids := set()
	for _, item := range maps(payload["cases"]) {
		id := stringValue(item["id"])
		if id == "" {
			return nil, nil, fmt.Errorf("case ids must be present and unique in %s", path)
		}
		if _, exists := ids[id]; exists {
			return nil, nil, fmt.Errorf("case ids must be present and unique in %s", path)
		}
		ids[id] = struct{}{}
		if integer(item["budget_bytes"]) <= 0 {
			return nil, nil, fmt.Errorf("case requires a positive budget_bytes value: %s", id)
		}
		expected, ok := item["expected"].(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("case expected must be an object: %s", id)
		}
		state := stringDefault(expected["state"], "READY")
		if _, ok = expectedStates[state]; !ok {
			return nil, nil, fmt.Errorf("invalid expected epistemic state in %s: %q", id, state)
		}
		proofs := maps(item["ground_truth"])
		_, hasRationale := item["rationale"]
		if len(proofs) == 0 && state != "OUT_OF_SCOPE" && hasRationale {
			return nil, nil, fmt.Errorf("case requires revision-pinned ground_truth: %s", id)
		}
		points := set()
		for _, proof := range proofs {
			proofPath := stringValue(proof["path"])
			if _, tracked := index.Tracked[proofPath]; !tracked {
				return nil, nil, fmt.Errorf("ground truth is not tracked in %s: %s", id, proofPath)
			}
			line := integer(proof["line"])
			contains := stringValue(proof["contains"])
			actual, lineErr := revisionLine(ctx, root, entry.Commit, proofPath, line)
			if lineErr != nil {
				return nil, nil, lineErr
			}
			if contains == "" || !strings.Contains(actual, contains) {
				return nil, nil, fmt.Errorf("ground truth moved for %s: %s:%d does not contain %q", id, proofPath, line, contains)
			}
			points[fmt.Sprintf("%s:%d", proofPath, line)] = struct{}{}
		}
		lists := map[string][]string{}
		for _, key := range []string{"must_include", "critical", "relevant", "must_exclude"} {
			values, listErr := stringList(expected[key], "expected."+key, id)
			if listErr != nil {
				return nil, nil, listErr
			}
			lists[key] = values
		}
		positive, forbidden := set(append(append(append([]string{}, lists["must_include"]...), lists["critical"]...), lists["relevant"]...)...), set(lists["must_exclude"]...)
		if intersects(positive, forbidden) {
			return nil, nil, fmt.Errorf("positive and forbidden selectors overlap in %s", id)
		}
		if state == "OUT_OF_SCOPE" && len(positive) != 0 {
			return nil, nil, fmt.Errorf("OUT_OF_SCOPE case contains positive selectors: %s", id)
		}
		if state != "OUT_OF_SCOPE" && len(positive) == 0 {
			return nil, nil, fmt.Errorf("positive case contains no expected evidence: %s", id)
		}
		for selector := range union(positive, forbidden) {
			kind, _, _ := strings.Cut(selector, ":")
			if _, ok := selectorKinds[kind]; !ok {
				return nil, nil, fmt.Errorf("unknown expected selector kind in %s: %s", id, selector)
			}
			selectorPath := selectorPath(selector)
			valid := false
			if kind == "symbol" {
				_, valid = symbols[selector]
			} else if selectorPath != "" {
				_, valid = index.Tracked[selectorPath]
			} else {
				_, valid = records[selector]
			}
			if !valid {
				return nil, nil, fmt.Errorf("unresolved expected selector in %s: %s", id, selector)
			}
			if state != "OUT_OF_SCOPE" && len(proofs) != 0 && (contains(lists["must_include"], selector) || contains(lists["critical"], selector)) {
				exact := selectorProofPoints(index, item, selector)
				if exact != nil && !intersects(exact, points) {
					return nil, nil, fmt.Errorf("selector lacks exact source proof in %s: %s", id, selector)
				}
			}
		}
	}
	return payload, index, nil
}

func selectorPath(selector string) string {
	kind, value, ok := strings.Cut(selector, ":")
	if !ok {
		return ""
	}
	if kind == "symbol" {
		path, _, found := strings.Cut(value, ":")
		if found {
			return path
		}
		return ""
	}
	if _, ok := set("path", "test", "reference", "reverse-import", "learned-path", "spec", "decision")[kind]; ok {
		return value
	}
	return ""
}

func selectorProofPoints(index *contextindex.Index, item map[string]any, selector string) map[string]struct{} {
	kind, value, ok := strings.Cut(selector, ":")
	if !ok {
		return nil
	}
	result := set()
	if kind == "symbol" {
		path, name, found := strings.Cut(value, ":")
		if !found {
			return result
		}
		var symbols []contextindex.Symbol
		for _, candidate := range index.Symbols {
			if candidate.Path == path {
				symbols = append(symbols, candidate)
			}
		}
		sort.Slice(symbols, func(i, j int) bool { return symbols[i].Line < symbols[j].Line })
		source := index.Sources[path]
		text, _, _ := source.Text()
		lineCount := len(strings.Split(strings.TrimSuffix(text, "\n"), "\n"))
		for i, symbol := range symbols {
			if symbol.Name != name {
				continue
			}
			end := lineCount + 1
			for _, following := range symbols[i+1:] {
				if declarationKind(symbol.Kind) && !declarationKind(following.Kind) {
					continue
				}
				end = following.Line
				break
			}
			for line := symbol.Line; line < end; line++ {
				result[fmt.Sprintf("%s:%d", path, line)] = struct{}{}
			}
		}
		return result
	}
	if kind == "feature" || kind == "scenario" {
		records := index.Features
		if kind == "scenario" {
			records = index.Scenarios
		}
		if record, found := records[value]; found {
			result[fmt.Sprintf("%s:%d", record.Path, record.Line)] = struct{}{}
		}
		return result
	}
	if kind == "reference" || kind == "reverse-import" {
		source, found := index.Sources[value]
		if !found {
			return result
		}
		text, valid, loaded := source.Text()
		if !valid || !loaded {
			return result
		}
		for _, changed := range stringsAny(item["paths"]) {
			needles := set(strings.TrimSuffix(filepath.Base(changed), filepath.Ext(changed)))
			if kind == "reference" {
				for _, symbol := range index.Symbols {
					if symbol.Path == changed {
						needles[symbol.Name] = struct{}{}
					}
				}
			}
			for line, body := range strings.Split(text, "\n") {
				for needle := range needles {
					if strings.Contains(body, needle) {
						result[fmt.Sprintf("%s:%d", value, line+1)] = struct{}{}
						break
					}
				}
			}
		}
		return result
	}
	return nil
}

func auditEvaluation(payload, evaluation map[string]any) (map[string]int, error) {
	expectedCases, actualCases := maps(payload["cases"]), maps(evaluation["cases"])
	if len(actualCases) != len(expectedCases) {
		return nil, errors.New("evaluation case count does not match the corpus")
	}
	counts := map[string]int{}
	for i, expectedCase := range expectedCases {
		actualCase := actualCases[i]
		if stringValue(actualCase["id"]) != stringValue(expectedCase["id"]) {
			return nil, errors.New("evaluation case order or identity does not match the corpus")
		}
		expected := expectedCase["expected"].(map[string]any)
		actual := set()
		for _, row := range maps(actualCase["actual"]) {
			if selector := stringValue(row["selector"]); selector != "" {
				actual[selector] = struct{}{}
			}
		}
		must, critical, forbidden := set(stringsAny(expected["must_include"])...), set(stringsAny(expected["critical"])...), set(stringsAny(expected["must_exclude"])...)
		missing, unexpected := sortedDifference(union(must, critical), actual), sortedIntersection(forbidden, actual)
		if !equalJSON(actualCase["missing"], missing) || !equalJSON(actualCase["unexpected"], unexpected) {
			return nil, fmt.Errorf("evaluation selector accounting mismatch: %s", stringValue(expectedCase["id"]))
		}
		state := stringDefault(expected["state"], "READY")
		counts["cases"]++
		if len(maps(expectedCase["ground_truth"])) != 0 {
			counts["revision_pinned_cases"]++
		}
		if len(must)+len(critical)+len(stringsAny(expected["relevant"])) != 0 {
			counts["positive_cases"]++
		}
		counts["must_read_total"] += len(must)
		counts["must_read_hits"] += intersectionSize(must, actual)
		counts["critical_evidence_total"] += len(critical)
		counts["critical_evidence_misses"] += len(sortedDifference(critical, actual))
		counts["must_exclude_total"] += len(forbidden)
		counts["must_exclude_violations"] += len(unexpected)
		counts["expected_state_cases"]++
		if stringValue(actualCase["state"]) == state {
			counts["expected_state_hits"]++
		}
		if state == "OUT_OF_SCOPE" || state == "NEEDS_WIDENING" {
			counts["epistemic_state_cases"]++
			if stringValue(actualCase["state"]) == state {
				counts["epistemic_state_hits"]++
			}
		}
	}
	metrics, _ := evaluation["metrics"].(map[string]any)
	for _, key := range []string{"cases", "must_read_total", "must_read_hits", "critical_evidence_total", "critical_evidence_misses", "must_exclude_total", "must_exclude_violations", "epistemic_state_cases", "epistemic_state_hits"} {
		if integer(metrics[key]) != counts[key] {
			return nil, fmt.Errorf("evaluation metric does not replay from case evidence: %s", key)
		}
	}
	return counts, nil
}

func exactBaseline(index *contextindex.Index, payload map[string]any) map[string]any {
	started := time.Now()
	total, hits, abstentionTotal, abstentionHits := 0, 0, 0, 0
	details := []any{}
	for _, item := range maps(payload["cases"]) {
		if stringValue(item["mode"]) != "query" {
			continue
		}
		total++
		terms := terms(stringValue(item["text"]))
		type score struct {
			overlap, count int
			path           string
		}
		var scored []score
		for path, source := range index.Sources {
			text, valid, loaded := source.Text()
			if !valid || !loaded {
				continue
			}
			lower := strings.ToLower(text)
			overlap, count := 0, 0
			pathTerms := set(termsFor(path)...)
			for term := range terms {
				if _, inPath := pathTerms[term]; inPath || strings.Contains(lower, term) {
					overlap++
					count += strings.Count(lower, term)
				}
			}
			if overlap != 0 {
				scored = append(scored, score{overlap, count, path})
			}
		}
		sort.Slice(scored, func(i, j int) bool {
			if scored[i].overlap != scored[j].overlap {
				return scored[i].overlap > scored[j].overlap
			}
			if scored[i].count != scored[j].count {
				return scored[i].count > scored[j].count
			}
			return scored[i].path < scored[j].path
		})
		paths := []string{}
		for i := 0; i < len(scored) && i < 5; i++ {
			paths = append(paths, scored[i].path)
		}
		expected := item["expected"].(map[string]any)
		expectedPaths := set()
		for _, key := range []string{"must_include", "critical", "relevant"} {
			for _, selector := range stringsAny(expected[key]) {
				if path := selectorPath(selector); path != "" {
					expectedPaths[path] = struct{}{}
				}
			}
		}
		hit := len(expectedPaths) == 0 && len(paths) == 0 || intersects(expectedPaths, set(paths...))
		if hit {
			hits++
		}
		abstention := stringValue(expected["state"]) == "OUT_OF_SCOPE"
		if abstention {
			abstentionTotal++
			if len(paths) == 0 {
				abstentionHits++
			}
		}
		details = append(details, map[string]any{"id": item["id"], "top5_hit": hit, "top5_paths": paths})
	}
	return map[string]any{"query_cases": total, "top5_hits": hits, "top5_success": ratio(hits, total, 1), "abstention_cases": abstentionTotal, "abstention_hits": abstentionHits, "abstention_accuracy": ratio(abstentionHits, abstentionTotal, 1), "latency_ms": roundN(float64(time.Since(started).Microseconds())/1000, 3), "cases": details}
}

func aggregate(repositories []any, skipped []string) map[string]any {
	metrics, contracts := []map[string]any{}, []map[string]int{}
	public, baselineCases, baselineHits, baselineAbstentionCases, baselineAbstentionHits := 0, 0, 0, 0, 0
	ready, latency := true, 0.0
	for _, raw := range repositories {
		repo := raw.(map[string]any)
		if boolean(repo["public"]) {
			public++
		}
		corvint := repo["corvint"].(map[string]any)
		metrics = append(metrics, corvint["metrics"].(map[string]any))
		contracts = append(contracts, repo["contract"].(map[string]int))
		ready = ready && stringValue(corvint["state"]) == "READY"
		base := repo["baseline"].(map[string]any)
		baselineCases += integer(base["query_cases"])
		baselineHits += integer(base["top5_hits"])
		baselineAbstentionCases += integer(base["abstention_cases"])
		baselineAbstentionHits += integer(base["abstention_hits"])
		latency += floating(metrics[len(metrics)-1]["latency_ms"])
	}
	sum := func(key string) int {
		value := 0
		for _, row := range metrics {
			value += integer(row[key])
		}
		return value
	}
	contractSum := func(key string) int {
		value := 0
		for _, row := range contracts {
			value += row[key]
		}
		return value
	}
	totalWeight, relevantWeight, topTotal, topHits, abstentionTotal, abstentionHits, epistemicTotal, epistemicHits, mustTotal, mustHits, budgetTotal := sum("total_result_bytes"), sum("relevant_result_bytes"), sum("top5_task_total"), sum("top5_task_hits"), sum("abstention_cases"), sum("abstention_hits"), sum("epistemic_state_cases"), sum("epistemic_state_hits"), sum("must_read_total"), sum("must_read_hits"), sum("budgeted_cases")
	return map[string]any{"repositories": len(repositories), "skipped_repository_ids": skipped, "public_repositories": public, "cases": sum("cases"), "positive_cases": contractSum("positive_cases"), "revision_pinned_cases": contractSum("revision_pinned_cases"), "budgeted_cases": budgetTotal, "must_read_total": contractSum("must_read_total"), "critical_evidence_total": contractSum("critical_evidence_total"), "must_exclude_total": contractSum("must_exclude_total"), "must_exclude_violations": sum("must_exclude_violations"), "repository_evaluations_ready": ready, "recall": ratio(mustHits, mustTotal, 1), "critical_evidence_misses": sum("critical_evidence_misses"), "serialized_result_byte_weighted_precision": ratio(relevantWeight, totalWeight, 0), "top5_task_success": ratio(topHits, topTotal, 1), "abstention_accuracy": ratio(abstentionHits, abstentionTotal, 1), "epistemic_state_accuracy": ratio(epistemicHits, epistemicTotal, 1), "epistemic_state_cases": epistemicTotal, "expected_state_accuracy": ratio(contractSum("expected_state_hits"), contractSum("expected_state_cases"), 0), "budget_compliance": ratio(budgetTotal-sum("budget_overflows")-sum("critical_evidence_overflows"), budgetTotal, 1), "latency_ms": roundN(latency, 3), "exact_search_baseline": map[string]any{"query_cases": baselineCases, "top5_success": ratio(baselineHits, baselineCases, 1), "abstention_accuracy": ratio(baselineAbstentionHits, baselineAbstentionCases, 1)}}
}

func thresholds(m map[string]any) map[string]bool {
	return map[string]bool{"all_manifest_repositories_included": len(stringsAny(m["skipped_repository_ids"])) == 0, "five_repositories": integer(m["repositories"]) >= 5, "four_public_repositories": integer(m["public_repositories"]) >= 4, "twenty_five_cases": integer(m["cases"]) >= 25, "positive_cases_present": integer(m["positive_cases"]) > 0, "twenty_revision_pinned_cases": integer(m["revision_pinned_cases"]) >= 20, "critical_ground_truth_present": integer(m["critical_evidence_total"]) >= integer(m["positive_cases"]), "forbidden_controls_present": integer(m["must_exclude_total"]) > 0, "epistemic_cases_present": integer(m["epistemic_state_cases"]) > 0, "all_cases_budgeted": integer(m["budgeted_cases"]) == integer(m["cases"]), "all_repository_evaluations_ready": boolean(m["repository_evaluations_ready"]), "full_recall": floating(m["recall"]) == 1, "zero_forbidden_hits": integer(m["must_exclude_violations"]) == 0, "zero_critical_misses": integer(m["critical_evidence_misses"]) == 0, "precision": floating(m["serialized_result_byte_weighted_precision"]) >= .8, "top5_success": floating(m["top5_task_success"]) >= .9, "abstention": floating(m["abstention_accuracy"]) == 1, "epistemic_state_accuracy": floating(m["epistemic_state_accuracy"]) == 1, "expected_state_accuracy": floating(m["expected_state_accuracy"]) == 1, "budget_compliance": floating(m["budget_compliance"]) == 1}
}

func learnedTraceReport(baseline, fixture map[string]any, changed map[string]int) map[string]any {
	rows := [][3]string{{"critical_evidence_misses", "critical_evidence_misses", "must not rise"}, {"abstention_accuracy", "abstention_accuracy", "must not fall"}, {"epistemic_state_accuracy", "epistemic_state_accuracy", "must not fall"}, {"serialized_result_byte_weighted_precision", "serialized_result_byte_weighted_precision", "minimum floor"}, {"recall", "recall", "informational"}, {"top_five_task_success", "top5_task_success", "informational"}}
	checks := map[string]bool{"critical_evidence_misses": floating(fixture["critical_evidence_misses"]) <= floating(baseline["critical_evidence_misses"]), "abstention_accuracy": floating(fixture["abstention_accuracy"]) >= floating(baseline["abstention_accuracy"]), "epistemic_state_accuracy": floating(fixture["epistemic_state_accuracy"]) >= floating(baseline["epistemic_state_accuracy"]), "serialized_result_byte_weighted_precision": floating(fixture["serialized_result_byte_weighted_precision"]) >= .8}
	delta := []any{}
	for _, spec := range rows {
		count := changed[spec[0]]
		value := any(round6(floating(fixture[spec[1]]) - floating(baseline[spec[1]])))
		classification := "distinguished"
		if count < 2 {
			value, classification = "not distinguished", "not distinguished"
		}
		row := map[string]any{"metric": spec[0], "baseline": baseline[spec[1]], "fixture": fixture[spec[1]], "delta": value, "classification": classification, "case_change_count": count, "gate": spec[2]}
		if passed, ok := checks[spec[0]]; ok {
			row["passed"] = passed
		}
		if spec[0] == "serialized_result_byte_weighted_precision" {
			row["floor"] = .8
		}
		delta = append(delta, row)
	}
	return map[string]any{"ready": allTrue(checks), "blocked_by": failedKeys(checks), "checks": checks, "delta": delta}
}

var termPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]*`)
var camelLower = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var camelUpper = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
var stopWords = set(strings.Fields("a an and are as at be by code for from how in is it of on or that the this to where with feature find change")...)
var termAliases = map[string][]string{"argument": {"parameter"}, "arguments": {"parameter"}, "generate": {"gen"}, "generated": {"gen"}, "generator": {"gen"}, "maximum": {"max"}, "minimum": {"min"}, "parameter": {"argument"}, "parameters": {"argument"}, "sync": {"synchronous"}, "synchronous": {"sync"}}

func terms(value string) map[string]struct{} { return set(termsFor(value)...) }
func termsFor(value string) []string {
	value = camelLower.ReplaceAllString(value, "$1-$2")
	value = camelUpper.ReplaceAllString(value, "$1-$2")
	values := set()
	for _, raw := range termPattern.FindAllString(strings.ToLower(strings.ReplaceAll(value, "_", "-")), -1) {
		for _, token := range strings.Split(raw, "-") {
			if len(token) <= 1 {
				continue
			}
			if _, stopped := stopWords[token]; stopped {
				continue
			}
			values[token] = struct{}{}
			if len(token) > 4 && strings.HasSuffix(token, "s") && !strings.HasSuffix(token, "ss") && !strings.HasSuffix(token, "as") && !strings.HasSuffix(token, "is") && !strings.HasSuffix(token, "us") {
				values[strings.TrimSuffix(token, "s")] = struct{}{}
			}
			for _, alias := range termAliases[token] {
				values[alias] = struct{}{}
			}
			if len(token) > 5 && strings.HasSuffix(token, "ify") {
				values[strings.TrimSuffix(token, "ify")] = struct{}{}
			}
		}
		if len(raw) > 2 {
			if _, stopped := stopWords[raw]; !stopped {
				values[raw] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}
func evaluationPartition(declared string, evidence map[string]any) string {
	if declared == "development" {
		return declared
	}
	if declared == "blind-v1" || strings.HasPrefix(declared, "development-after-blind-") {
		return "development-after-observation"
	}
	if evidence != nil && declared == stringValue(evidence["partition"]) {
		return "development-after-first-run"
	}
	if strings.HasPrefix(declared, "heldout-") {
		return "unattested-heldout"
	}
	return declared
}
func revisionLine(ctx context.Context, root, revision, path string, number int) (string, error) {
	if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") {
		return "", fmt.Errorf("invalid ground-truth path: %q", path)
	}
	raw, err := runProcess(ctx, root, 30*time.Second, "git", "-C", root, "show", revision+":"+path)
	if err != nil {
		return "", fmt.Errorf("cannot read ground truth %s at revision %s: %w", path, revision, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if number < 1 || number > len(lines) {
		return "", fmt.Errorf("ground-truth line %d is outside %s", number, path)
	}
	return lines[number-1], nil
}
func git(ctx context.Context, root string, args ...string) (string, error) {
	output, err := runProcess(ctx, root, 2*time.Minute, "git", append([]string{"-C", root}, args...)...)
	if err != nil {
		return "", fmt.Errorf("Git failed for %s: %w", root, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func runProcess(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) ([]byte, error) {
	executable, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: append([]string{executable}, args...), Dir: dir, Env: os.Environ(), Timeout: timeout,
		ShutdownTimeout: 2 * time.Second, InputLimit: 1, OutputLimit: 16 << 20, StderrLimit: 1 << 20,
	})
	if observation.Err != nil {
		return nil, observation.Err
	}
	if observation.ExitStatus != 0 {
		return nil, fmt.Errorf("exit %d: %s", observation.ExitStatus, strings.TrimSpace(string(observation.Stderr)))
	}
	if !observation.WaitCompleted || !observation.PipesDrained || !observation.OwnedProcessGroupCleanup {
		return nil, errors.New("process cleanup incomplete")
	}
	return observation.Stdout, nil
}
func atomicWrite(path string, data []byte) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return err
	}
	temporary := absolute + ".tmp"
	if err = os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, absolute)
}
func stringList(value any, field, id string) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a list of non-empty strings in %s", field, id)
	}
	result, seen := []string{}, set()
	for _, raw := range values {
		item, ok := raw.(string)
		if !ok || item == "" {
			return nil, fmt.Errorf("%s must be a list of non-empty strings in %s", field, id)
		}
		if _, duplicate := seen[item]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate selectors in %s", field, id)
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result, nil
}
func sha(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func set(values ...string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
func union(left, right map[string]struct{}) map[string]struct{} {
	result := set()
	for key := range left {
		result[key] = struct{}{}
	}
	for key := range right {
		result[key] = struct{}{}
	}
	return result
}
func intersects(left, right map[string]struct{}) bool { return intersectionSize(left, right) != 0 }
func intersectionSize(left, right map[string]struct{}) int {
	count := 0
	for key := range left {
		if _, ok := right[key]; ok {
			count++
		}
	}
	return count
}
func sortedDifference(left, right map[string]struct{}) []string {
	result := []string{}
	for key := range left {
		if _, ok := right[key]; !ok {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}
func sortedIntersection(left, right map[string]struct{}) []string {
	result := []string{}
	for key := range left {
		if _, ok := right[key]; ok {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}
func maps(value any) []map[string]any {
	values, _ := value.([]any)
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if row, ok := value.(map[string]any); ok {
			result = append(result, row)
		}
	}
	return result
}
func stringsAny(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		result := []string{}
		for _, value := range values {
			if text, ok := value.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return []string{}
	}
}
func stringValue(value any) string { text, _ := value.(string); return text }
func stringDefault(value any, fallback string) string {
	if text := stringValue(value); text != "" {
		return text
	}
	return fallback
}
func integer(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		return int(number)
	case json.Number:
		parsed, _ := number.Int64()
		return int(parsed)
	default:
		return 0
	}
}
func floating(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case int:
		return float64(number)
	default:
		return 0
	}
}
func boolean(value any) bool { result, _ := value.(bool); return result }
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func declarationKind(kind string) bool {
	_, ok := set("class", "func", "interface", "method", "type")[kind]
	return ok
}
func equalJSON(left, right any) bool {
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return string(a) == string(b)
}
func ratio(numerator, denominator int, empty float64) float64 {
	if denominator == 0 {
		return empty
	}
	return round6(float64(numerator) / float64(denominator))
}
func round6(value float64) float64 {
	return roundN(value, 6)
}
func roundN(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
func choose[T any](condition bool, yes, no T) T {
	if condition {
		return yes
	}
	return no
}
func samePath(left, right string) bool {
	a, _ := filepath.Abs(left)
	b, _ := filepath.Abs(right)
	return a == b
}
func cloneMap(source map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range source {
		result[key] = value
	}
	return result
}
func addCounts(target, source map[string]int) {
	for key, value := range source {
		target[key] += value
	}
}
func allTrue(values map[string]bool) bool {
	for _, value := range values {
		if !value {
			return false
		}
	}
	return true
}
func failedKeys(values map[string]bool) []string {
	result := []string{}
	for key, value := range values {
		if !value {
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}
