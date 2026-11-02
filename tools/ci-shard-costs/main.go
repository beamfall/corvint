// Command ci-shard-costs refreshes and checks the advisory package costs that
// place packages in CI shards (AFP-V0-022). Costs only move packages between
// shards; they never select tests.
//
// Inputs are the hosted `go test -json` streams of one complete CI run, one
// file per shard job, as raw completed JSON streams (AFP-V0-043). Logs printed by
// `gh run view --log` (prefixed lines) are refused; use the raw shard artifacts:
//
//	ci-shard-costs refresh --shards 8 --revision SHA --run-url URL shard0.log ... shard7.log
//	ci-shard-costs check --shards 8 shard0.log ... shard7.log
//
// --shards declares the CI shard count, the go-product-shard matrix size, and
// every mode needs exactly one log per shard. refresh rewrites the table from the
// observed terminal package outcomes and records the source run; it refuses logs
// that lack a package the current table lists unless --allow-removed is given. A
// package that the committed allow-list and slice file split (AFP-V0-041) must end
// exactly once per test slice, each in a different log: its cost is the sum of its
// slices, and any missing, extra, failed or unfinished slice refuses the run, as
// does a slice file that splits it into more slices than --shards, so neither a
// run made before the split nor a partial run can measure it. check exits 1 and
// prints one line per package whose observed time differs from its table entry
// by more than --factor, that the table is missing, or that the table still
// lists but the run did not execute.
//
//	ci-shard-costs refresh ... --history DIR [--keep N] [--at TIME] shard0.log ...
//
// --history (STR-V0-056) also retains the refreshed table, byte for byte, as
// DIR/<YYYYMMDDTHHMMSSZ>-<revision>.json, written before the table is replaced,
// and then deletes the retained tables beyond the newest --keep (1 to 64,
// default 5) by name; no other file of DIR is touched. `corvint test shards
// --durations` reads the retained files directly, oldest first in name order.
//
//	ci-shard-costs check --advisory --shards 8 [--share 10] [--summary FILE] shard0.json ... shard7.json
//
// The advisory form (AFP-V0-040) is the CI report: it needs exactly one raw
// `go test -json` stream per shard, each well formed and finished, appends a
// Markdown report to --summary, prints at most ten `::warning::` workflow
// commands, and exits 0 whatever it finds. A finding is
// material when the time it misplaces reaches --share percent of the ideal
// shard (all observed time divided by --shards); a drift of that size is
// reported even within --factor. Unusable input exits 2 and the report says
// why it abstained.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/.github/cishards"
)

const (
	defaultTable  = ".github/cishards/package-costs.json"
	defaultAllow  = ".github/cishards/test-split-allow.json"
	defaultSlices = ".github/cishards/test-slices.json"
	// defaultKeep and maxKeep bound the retained tables (STR-V0-056): the
	// default is the planner's default K, the maximum its table bound.
	defaultKeep = 5
	maxKeep     = 64
)

// retainedName is the only name `refresh --history` writes or deletes.
var retainedName = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{40}\.json$`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ci-shard-costs refresh|check --shards N [flags] LOG...")
		os.Exit(2)
	}
	code, err := run(os.Args[1], os.Args[2:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-shard-costs:", err)
	}
	os.Exit(code)
}

func run(mode string, args []string, out io.Writer) (int, error) {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	table := fs.String("table", defaultTable, "package cost table")
	revision := fs.String("revision", "", "refresh: full commit the source run tested")
	runURL := fs.String("run-url", "", "refresh: hosted run that produced the logs")
	factor := fs.Float64("factor", 2, "check: tolerated ratio between observed and recorded time")
	floor := fs.Duration("floor", 10*time.Second, "check: ignore differences smaller than this")
	allowRemoved := fs.Bool("allow-removed", false, "refresh: accept that packages in the current table were not executed")
	advisory := fs.Bool("advisory", false, "check: CI report that never fails on findings (AFP-V0-040)")
	shards := fs.Int("shards", 0, "CI shard count (the go-product-shard matrix): one log per shard of one complete run")
	share := fs.Float64("share", 10, "check --advisory: percent of the ideal shard that makes a finding material")
	summary := fs.String("summary", "", "check --advisory: append the Markdown report to this file")
	allow := fs.String("allow", defaultAllow, "AFP-V0-041 test-split allow-list")
	slices := fs.String("slices", defaultSlices, "AFP-V0-041 test slice file")
	history := fs.String("history", "", "refresh: retain the refreshed table in this directory (STR-V0-056)")
	keep := fs.Int("keep", defaultKeep, "refresh --history: retained tables to keep, 1 to 64")
	at := fs.String("at", "", "refresh --history: retention time, RFC 3339 in UTC (default now)")
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	retainedAt, err := retentionTime(mode, *history, *keep, *at, *revision)
	if err != nil {
		return 2, err
	}
	if (mode != "refresh" && mode != "check") || fs.NArg() == 0 || !(*factor >= 1) || math.IsInf(*factor, 0) || *floor < 0 {
		return 2, errors.New("usage: ci-shard-costs refresh|check --shards N [flags] LOG...")
	}
	if *advisory {
		if mode != "check" || *shards < 1 || *shards > cishards.MaxShards || !(*share > 0 && *share <= 100) {
			return 2, errors.New("usage: ci-shard-costs check --advisory --shards N [--share PERCENT] [--summary FILE] LOG...")
		}
		code, err := report(out, *summary, *table, *allow, *slices, fs.Args(), *shards, *share, *factor, floor.Milliseconds())
		if err != nil && *summary != "" {
			appendFile(*summary, fmt.Sprintf("%s\nAbstained: %v. No finding is reported for this run.\n", reportHeading, err))
		}
		return code, err
	}
	if *shards < 1 || *shards > cishards.MaxShards {
		return 2, errors.New("usage: ci-shard-costs refresh|check --shards N [flags] LOG...")
	}
	if fs.NArg() != *shards {
		return 2, fmt.Errorf("%d of %d shard logs present; a partial run cannot measure the table", fs.NArg(), *shards)
	}
	o := newOutcomes()
	for _, name := range fs.Args() {
		f, err := os.Open(name)
		if err != nil {
			return 2, err
		}
		err = observeStream(f, o)
		f.Close()
		if err != nil {
			return 2, fmt.Errorf("%s: %w", name, err)
		}
	}
	observed, err := combine(o, *shards, *allow, *slices)
	if err != nil {
		return 2, err
	}
	if len(observed) == 0 {
		return 2, errors.New("no terminal package outcome in the logs")
	}
	raw, err := os.ReadFile(*table)
	recorded, ok := cishards.Costs(raw)
	if mode == "refresh" {
		// The logs cannot prove they are one complete run. A package the current table
		// knows but the logs lack is the visible sign of a partial or narrowed input.
		if !*allowRemoved {
			for _, line := range drift(recorded, observed, math.MaxFloat64, 0) {
				if strings.HasPrefix(line, "stale ") {
					return 2, fmt.Errorf("refused: %s; pass every shard log of one complete run, or --allow-removed", line)
				}
			}
		}
		raw, err := encode(observed, *revision, *runURL)
		if err != nil {
			return 2, err
		}
		if *history != "" {
			if err := retain(*history, retainedAt.Format("20060102T150405Z")+"-"+*revision+".json", raw); err != nil {
				return 2, fmt.Errorf("retaining the table: %w", err)
			}
		}
		if err := replace(*table, raw); err != nil {
			return 2, err
		}
		if *history != "" {
			if err := prune(*history, *keep); err != nil {
				return 2, fmt.Errorf("pruning retained tables: %w", err)
			}
		}
		return 0, nil
	}
	if err != nil {
		return 2, err
	}
	if !ok {
		return 2, errors.New("cost table is invalid; CI is using the lexical fallback")
	}
	lines := drift(recorded, observed, *factor, floor.Milliseconds())
	for _, line := range lines {
		fmt.Fprintln(out, line)
	}
	if len(lines) != 0 {
		return 1, nil
	}
	return 0, nil
}

// retentionTime checks the STR-V0-056 flags and returns the retention time:
// --history only with refresh, --keep 1 to 64, --at only with --history and in
// UTC, and a lowercase full revision so the retained name can be pruned.
func retentionTime(mode, history string, keep int, at, revision string) (time.Time, error) {
	usage := errors.New("usage: ci-shard-costs refresh --history DIR [--keep N (1 to 64)] [--at RFC3339-UTC] --revision SHA ...")
	if keep < 1 || keep > maxKeep || (history == "" && at != "") || (history != "" && mode != "refresh") {
		return time.Time{}, usage
	}
	if history == "" {
		return time.Time{}, nil
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(revision) {
		return time.Time{}, errors.New("refresh --history needs a lowercase 40-hex --revision")
	}
	if at == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, at)
	if err != nil || !strings.HasSuffix(at, "Z") {
		return time.Time{}, usage
	}
	return parsed, nil
}

// retain writes one retained table into dir through the same sibling
// temporary file and rename as the table (STR-V0-056).
func retain(dir, name string, raw []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return replace(filepath.Join(dir, name), raw)
}

// prune deletes the retained tables of dir beyond the newest keep by name,
// which orders them by time; a file of any other name or type is left alone.
func prune(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	retained := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && retainedName.MatchString(entry.Name()) {
			retained = append(retained, entry.Name())
		}
	}
	for _, name := range retained[:max(0, len(retained)-keep)] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// replace writes the table through a sibling temporary file so an interrupted
// refresh cannot leave bytes the partition would reject.
func replace(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".package-costs-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Chmod(0o644)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// outcomes collects the terminal package outcomes of one run's shard logs
// before combine turns them into one cost per package.
type outcomes struct {
	elapsed map[string]int64 // milliseconds, summed over the package's outcomes
	count   map[string]int   // terminal outcomes, at most one per log
	open    map[string]bool  // started in some log without a terminal outcome there
}

func newOutcomes() *outcomes {
	return &outcomes{elapsed: map[string]int64{}, count: map[string]int{}, open: map[string]bool{}}
}

// observe adds each package's terminal outcome from one `go test -json` stream.
// A failed package, or one with two outcomes in one log, is refused: its time is
// not a cost of the suite.
func observe(r io.Reader, o *outcomes) error {
	_, err := scan(r, o, false)
	return err
}

// observeStream is observe for a raw stream that must stand for one finished
// shard (AFP-V0-040): every line is a JSON event, every started package ends in
// a terminal outcome, and at least one package does.
func observeStream(r io.Reader, o *outcomes) error {
	n, err := scan(r, o, true)
	if err == nil && n == 0 {
		err = errors.New("no terminal package outcome")
	}
	return err
}

// streamActions is the closed Action set of `go test -json`: the test2json
// TestEvent actions, the `attr` action of testing.T.Attr, the `artifacts`
// action of testing.T.ArtifactDir under -artifacts, and the interleaved
// BuildEvent actions, which carry ImportPath instead of Package.
var streamActions = map[string]bool{
	"start": true, "run": true, "pause": true, "cont": true, "pass": true, "bench": true,
	"fail": true, "output": true, "skip": true, "attr": true, "artifacts": true,
	"build-output": true, "build-fail": true,
}

// checkEvent refuses, for a strict stream, an event that is not structurally a
// `go test -json` event of a passing run: an unknown Action, a missing Package
// or ImportPath, any failure, or a package start or outcome out of sequence.
func checkEvent(line int, action, pkg, importPath, test string, started map[string]bool) error {
	switch {
	case !streamActions[action]:
		return fmt.Errorf("line %d has unknown Action %q", line, action)
	case action == "build-output" || action == "build-fail":
		if importPath == "" {
			return fmt.Errorf("line %d: %s event without ImportPath", line, action)
		}
		if action == "build-fail" {
			return fmt.Errorf("build of %s failed", importPath)
		}
		return nil
	case pkg == "":
		return fmt.Errorf("line %d: %s event without Package", line, action)
	case action == "fail" && test != "":
		return fmt.Errorf("test %s of package %s failed", test, pkg)
	case test != "":
		return nil
	case action == "start" && started[pkg]:
		return fmt.Errorf("package %s started twice", pkg)
	case (action == "pass" || action == "skip") && !started[pkg]:
		return fmt.Errorf("package %s has a terminal outcome without a start", pkg)
	}
	return nil
}

func scan(r io.Reader, o *outcomes, strict bool) (int, error) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1<<16), 8<<20)
	started := map[string]bool{}
	ended := map[string]bool{}
	terminal := 0
	for line := 1; s.Scan(); line++ {
		raw := s.Bytes()
		i := bytes.IndexByte(raw, '{')
		if strict && len(bytes.TrimSpace(raw)) != 0 && (i != 0 || !json.Valid(raw)) {
			return terminal, fmt.Errorf("line %d is not a go test -json event", line)
		}
		if i < 0 {
			continue
		}
		var e struct {
			Action     string
			Package    string
			ImportPath string
			Test       string
			Elapsed    float64
		}
		if json.Unmarshal(raw[i:], &e) != nil {
			if strict {
				return terminal, fmt.Errorf("line %d is not a go test -json event", line)
			}
			continue
		}
		if strict {
			if err := checkEvent(line, e.Action, e.Package, e.ImportPath, e.Test, started); err != nil {
				return terminal, err
			}
		}
		if e.Package == "" || e.Test != "" {
			continue
		}
		switch e.Action {
		case "start":
			started[e.Package] = true
		case "fail":
			return terminal, fmt.Errorf("package %s failed", e.Package)
		case "pass", "skip":
			// Slice placement puts sibling slices in different shards, so no package
			// ends twice in one log; combine decides how often it may end in a run.
			if ended[e.Package] {
				return terminal, fmt.Errorf("package %s has two terminal outcomes", e.Package)
			}
			ended[e.Package] = true
			o.elapsed[e.Package] += int64(math.Round(e.Elapsed * 1000))
			o.count[e.Package]++
			delete(started, e.Package)
			terminal++
		}
	}
	if err := s.Err(); err != nil {
		return terminal, err
	}
	if strict && len(started) != 0 {
		open := make([]string, 0, len(started))
		for p := range started {
			open = append(open, p)
		}
		sort.Strings(open)
		return terminal, fmt.Errorf("package %s started without a terminal outcome (unfinished stream)", open[0])
	}
	for p := range started {
		o.open[p] = true
	}
	return terminal, nil
}

// combine returns one cost per package. A package ends once, unless the committed
// allow-list and slice file split it over a partition of the declared `shards`,
// exactly as the partition does (AFP-V0-041): then it ends exactly once per slice
// and its cost is the sum. Any other count is refused, a single outcome included:
// one slice's time is indistinguishable from a whole run, so a run made before the
// split cannot measure a split package. The slice count comes from the slice file,
// so a package the file splits into more slices than `shards` is refused rather
// than counted whole. A split package that started without ending in some log is
// refused even in a hosted log, because its sum would be short. An unreadable or
// unusable slice input splits nothing, as in CI.
func combine(o *outcomes, shards int, allowPath, slicesPath string) (map[string]int64, error) {
	universe := make([]string, 0, len(o.count)+len(o.open))
	for p := range o.count {
		universe = append(universe, p)
	}
	for p := range o.open {
		if o.count[p] == 0 {
			universe = append(universe, p)
		}
	}
	sort.Strings(universe)
	allowRaw, _ := os.ReadFile(allowPath)
	slicesRaw, _ := os.ReadFile(slicesPath)
	split := cishards.SplitPackages(universe, shards, allowRaw, slicesRaw)
	declared := cishards.SplitPackages(universe, cishards.MaxSlices, allowRaw, slicesRaw)
	observed := make(map[string]int64, len(o.count))
	for _, p := range universe {
		n, k := o.count[p], len(split[p])
		switch {
		case k == 0 && len(declared[p]) != 0:
			return nil, fmt.Errorf("package %s is split into %d test slices, more than --shards %d", p, len(declared[p]), shards)
		case k != 0 && o.open[p]:
			return nil, fmt.Errorf("a test slice of package %s started without a terminal outcome", p)
		case k != 0 && n != k:
			return nil, fmt.Errorf("package %s has %d terminal outcomes for its %d test slices", p, n, k)
		case n > 1 && k == 0:
			return nil, fmt.Errorf("package %s has two terminal outcomes", p)
		case n > 0:
			// The table admits only positive costs; a package without tests costs the minimum.
			observed[p] = max(1, o.elapsed[p])
		}
	}
	return observed, nil
}

// sourceGoVersion is the toolchain recorded as the table's source goVersion.
// refresh runs under the toolchain that measured the source run, so it records
// its own runtime version rather than a constant; the partition admits only
// its pinned version, and any other toolchain is refused (decision 0522).
var sourceGoVersion = runtime.Version()

func encode(observed map[string]int64, revision, runURL string) ([]byte, error) {
	raw, err := json.MarshalIndent(map[string]any{
		"profile":      "corvint-ci-package-costs/0",
		"source":       map[string]string{"revision": revision, "runURL": runURL, "goVersion": sourceGoVersion},
		"milliseconds": observed,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, ok := cishards.Costs(raw); !ok {
		return nil, fmt.Errorf("refused: the partition would reject this table (check --revision, --run-url, package times and the source toolchain %s)", sourceGoVersion)
	}
	return raw, nil
}

// finding is one table entry that disagrees with a complete run. Times are in
// milliseconds; recorded is 0 for a missing package and observed is 0 for a stale one.
type finding struct {
	kind, pkg          string
	recorded, observed int64
}

func (f finding) String() string {
	switch f.kind {
	case "missing":
		return fmt.Sprintf("missing %s observed=%dms", f.pkg, f.observed)
	case "stale":
		return fmt.Sprintf("stale %s recorded=%dms", f.pkg, f.recorded)
	}
	return fmt.Sprintf("drift %s recorded=%dms observed=%dms", f.pkg, f.recorded, f.observed)
}

// misplaced is the time the table's placement got wrong. A stale entry names no
// package of the universe, so it misplaces nothing.
func (f finding) misplaced() int64 {
	if f.kind == "stale" {
		return 0
	}
	return max(f.observed-f.recorded, f.recorded-f.observed)
}

// findings lists drift beyond factor and floorMS, or of at least materialMS
// whatever the ratio, then every missing and stale package.
func findings(recorded, observed map[string]int64, factor float64, floorMS, materialMS int64) []finding {
	var out []finding
	for p, got := range observed {
		want, ok := recorded[p]
		if !ok {
			out = append(out, finding{"missing", p, 0, got})
			continue
		}
		lo, hi := min(got, want), max(got, want)
		if hi-lo >= materialMS || hi-lo >= floorMS && float64(hi) > factor*float64(lo) {
			out = append(out, finding{"drift", p, want, got})
		}
	}
	for p, want := range recorded {
		if _, ok := observed[p]; !ok {
			out = append(out, finding{"stale", p, want, 0})
		}
	}
	return out
}

func drift(recorded, observed map[string]int64, factor float64, floorMS int64) []string {
	var lines []string
	for _, f := range findings(recorded, observed, factor, floorMS, math.MaxInt64) {
		lines = append(lines, f.String())
	}
	sort.Strings(lines)
	return lines
}

const (
	reportHeading = "### CI shard cost table drift (advisory, AFP-V0-040)\n"
	// maxWarnings stays inside the ten warning annotations GitHub shows per step.
	maxWarnings = 10
)

// report is the advisory CI form of check. Findings never change its exit code;
// only input that cannot stand for one complete run does, and then it abstains.
func report(out io.Writer, summary, table, allow, slices string, logs []string, shards int, share, factor float64, floorMS int64) (int, error) {
	if len(logs) != shards {
		return 2, fmt.Errorf("%d of %d shard logs present; a partial run cannot measure the table", len(logs), shards)
	}
	o := newOutcomes()
	perShard := make([]*outcomes, len(logs))
	for i, name := range logs {
		raw, err := os.ReadFile(name)
		if err != nil {
			return 2, err
		}
		if err := observeStream(bytes.NewReader(raw), o); err != nil {
			return 2, fmt.Errorf("%s: %w", filepath.Base(name), err)
		}
		// The same stream parsed alone gives this shard's own load (AFP-V0-604).
		perShard[i] = newOutcomes()
		if err := observeStream(bytes.NewReader(raw), perShard[i]); err != nil {
			return 2, fmt.Errorf("%s: %w", filepath.Base(name), err)
		}
	}
	observed, err := combine(o, shards, allow, slices)
	if err != nil {
		return 2, err
	}
	if len(observed) == 0 {
		return 2, errors.New("no terminal package outcome in the logs")
	}
	raw, err := os.ReadFile(table)
	if err != nil {
		return 2, err
	}
	recorded, ok := cishards.Costs(raw)
	if !ok {
		return 2, errors.New("cost table is invalid; CI is using the lexical fallback")
	}
	var total int64
	for _, n := range observed {
		total += n
	}
	ideal := float64(total) / float64(shards)
	materialMS := max(1, int64(math.Ceil(ideal*share/100)))
	all := findings(recorded, observed, factor, floorMS, materialMS)
	sort.Slice(all, func(i, j int) bool {
		if a, b := all[i].misplaced(), all[j].misplaced(); a != b {
			return a > b
		}
		return all[i].String() < all[j].String()
	})
	material := 0
	for material < len(all) && all[material].misplaced() >= materialMS {
		material++
	}
	loads := shardLoads(perShard, o, recorded)
	slowest := 0
	for i, l := range loads {
		if l.observed > loads[slowest].observed {
			slowest = i
		}
	}
	stale := staleShard(loads[slowest], ideal, materialMS)

	var b strings.Builder
	b.WriteString(reportHeading)
	fmt.Fprintf(&b, "\n%d shard logs, %d packages, %s observed; ideal shard %s. A finding is material when it misplaces at least %g%% of the ideal shard (%s). Costs change placement, never membership, and this report never fails CI. Refreshing `%s` is an operator step (`go run ./tools/ci-shard-costs refresh --shards %d`, AFP-V0-022).\n\n",
		shards, len(observed), seconds(total), seconds(int64(ideal)), share, seconds(materialMS), table, shards)
	if len(all) == 0 {
		b.WriteString("No drift, missing or stale package.\n")
	} else {
		b.WriteString("| Finding | Package | Recorded | Observed | Misplaced | Of ideal shard |\n|---|---|---|---|---|---|\n")
		for i, f := range all {
			kind := f.kind
			if i < material {
				kind = "**" + kind + "** (material)"
			}
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s | %.1f%% |\n", kind, f.pkg, optional(f.recorded), optional(f.observed), seconds(f.misplaced()), 100*float64(f.misplaced())/ideal)
		}
	}
	fmt.Fprintf(&b, "\n#### Shard balance (AFP-V0-604)\n\nPredicted is what the committed table expected each shard to cost; observed is what it cost. The table is stale when the slowest shard exceeds the ideal shard by at least the material share and costs that much more than predicted.\n\n| Shard | Predicted | Observed | Over ideal |\n|---|---|---|---|\n")
	for i, l := range loads {
		fmt.Fprintf(&b, "| %d | %s | %s | %.1f%% |\n", i, seconds(l.predicted), seconds(l.observed), 100*(float64(l.observed)-ideal)/ideal)
	}
	if stale {
		fmt.Fprintf(&b, "\n**Stale:** shard %d is the slowest, %s over the ideal shard and %s over its prediction.\n", slowest, seconds(loads[slowest].observed-int64(ideal)), seconds(loads[slowest].observed-loads[slowest].predicted))
	} else {
		b.WriteString("\nThe slowest shard is within the material share of the ideal shard or of its prediction.\n")
	}
	if summary != "" {
		if err := appendFile(summary, b.String()); err != nil {
			return 2, err
		}
	}

	// One warning per material finding, keeping the last line for everything else;
	// a stale shard balance takes the first line (AFP-V0-604).
	budget := maxWarnings - 1
	if stale {
		budget--
		l := loads[slowest]
		fmt.Fprintf(out, "::warning title=CI shard balance stale::%s\n", escape(fmt.Sprintf("shard %d observed %s against %s predicted, %.0f%% over the ideal shard %s; refresh %s (AFP-V0-604)", slowest, seconds(l.observed), seconds(l.predicted), 100*(float64(l.observed)-ideal)/ideal, seconds(int64(ideal)), table)))
	}
	warnings := min(material, budget)
	for _, f := range all[:warnings] {
		fmt.Fprintf(out, "::warning title=CI shard cost drift::%s\n", escape(fmt.Sprintf("%s misplaces %s (%.0f%% of the ideal shard); refresh %s (AFP-V0-040)", f, seconds(f.misplaced()), 100*float64(f.misplaced())/ideal, table)))
	}
	if rest := all[warnings:]; len(rest) != 0 {
		counts := map[string]int{}
		for _, f := range rest {
			counts[f.kind]++
		}
		fmt.Fprintf(out, "::warning title=CI shard cost drift::%s\n", escape(fmt.Sprintf("%d further findings (%d drift, %d missing, %d stale); see the job summary (AFP-V0-040)", len(rest), counts["drift"], counts["missing"], counts["stale"])))
	}
	return 0, nil
}

// shardLoad is one shard's predicted and observed cost in milliseconds.
type shardLoad struct{ predicted, observed int64 }

// shardLoads measures each shard log against the committed table (AFP-V0-604).
// A shard's observed load is the sum of its package outcomes; its predicted load
// is the table's cost of the same packages, the median cost for a package the
// table lacks (as the partition places it), and, for a split package, the
// table's whole-package cost in the observed share of its slice: slice balance
// belongs to the AFP-V0-041 slice file, so an exact cost table is never stale.
func shardLoads(perShard []*outcomes, all *outcomes, recorded map[string]int64) []shardLoad {
	values := make([]int64, 0, len(recorded))
	for _, n := range recorded {
		values = append(values, n)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	var median int64
	if len(values) != 0 {
		median = values[len(values)/2]
	}
	loads := make([]shardLoad, len(perShard))
	for i, s := range perShard {
		for p, n := range s.count {
			if n == 0 {
				continue
			}
			loads[i].observed += max(1, s.elapsed[p])
			cost, ok := recorded[p]
			if !ok {
				cost = median
			}
			if all.count[p] > 1 && all.elapsed[p] > 0 {
				cost = cost * s.elapsed[p] / all.elapsed[p]
			}
			loads[i].predicted += cost
		}
	}
	return loads
}

// staleShard is the AFP-V0-604 trigger: the slowest shard exceeds the ideal
// shard by at least the material share and costs at least that much more than
// the table predicted, so refreshing the table, not luck, would rebalance it.
func staleShard(l shardLoad, ideal float64, materialMS int64) bool {
	return float64(l.observed)-ideal >= float64(materialMS) && l.observed-l.predicted >= materialMS
}

func seconds(ms int64) string { return fmt.Sprintf("%.1fs", float64(ms)/1000) }

func optional(ms int64) string {
	if ms == 0 {
		return "-"
	}
	return seconds(ms)
}

// escape encodes the characters a workflow command message cannot carry.
func escape(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
