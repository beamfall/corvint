// Command ci-shard-costs refreshes and checks the advisory package costs that
// place packages in CI shards (AFP-V0-022). Costs only move packages between
// shards; they never select tests.
//
// Inputs are the hosted `go test -json` streams of one complete CI run, one
// file per shard job, either raw or as printed by
// `gh run view RUN --job JOB --log` (text before the JSON object is ignored):
//
//	ci-shard-costs refresh --revision SHA --run-url URL shard0.log ... shard3.log
//	ci-shard-costs check shard0.log ... shard3.log
//
// refresh rewrites the table from the observed terminal package outcomes and
// records the source run; it refuses logs that lack a package the current
// table lists unless --allow-removed is given. check exits 1 and prints one line per package whose
// observed time differs from its table entry by more than --factor, that the
// table is missing, or that the table still lists but the run did not execute.
//
//	ci-shard-costs check --advisory --shards 6 [--share 10] [--summary FILE] shard0.json ... shard5.json
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
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/.github/cishards"
)

const defaultTable = ".github/cishards/package-costs.json"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ci-shard-costs refresh|check [flags] LOG...")
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
	shards := fs.Int("shards", 0, "check --advisory: number of shard logs in one complete run")
	share := fs.Float64("share", 10, "check --advisory: percent of the ideal shard that makes a finding material")
	summary := fs.String("summary", "", "check --advisory: append the Markdown report to this file")
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	if (mode != "refresh" && mode != "check") || fs.NArg() == 0 || !(*factor >= 1) || math.IsInf(*factor, 0) || *floor < 0 {
		return 2, errors.New("usage: ci-shard-costs refresh|check [flags] LOG...")
	}
	if *advisory {
		if mode != "check" || *shards < 1 || *shards > cishards.MaxShards || !(*share > 0 && *share <= 100) {
			return 2, errors.New("usage: ci-shard-costs check --advisory --shards N [--share PERCENT] [--summary FILE] LOG...")
		}
		code, err := report(out, *summary, *table, fs.Args(), *shards, *share, *factor, floor.Milliseconds())
		if err != nil && *summary != "" {
			appendFile(*summary, fmt.Sprintf("%s\nAbstained: %v. No finding is reported for this run.\n", reportHeading, err))
		}
		return code, err
	}
	observed := map[string]int64{}
	for _, name := range fs.Args() {
		f, err := os.Open(name)
		if err != nil {
			return 2, err
		}
		err = observe(f, observed)
		f.Close()
		if err != nil {
			return 2, fmt.Errorf("%s: %w", name, err)
		}
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
		if err := replace(*table, raw); err != nil {
			return 2, err
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

// observe adds each package's terminal outcome from one `go test -json` stream.
// A failed or repeated package is refused: its time is not a cost of the suite.
func observe(r io.Reader, observed map[string]int64) error {
	_, err := scan(r, observed, false)
	return err
}

// observeStream is observe for a raw stream that must stand for one finished
// shard (AFP-V0-040): every line is a JSON event, every started package ends in
// a terminal outcome, and at least one package does.
func observeStream(r io.Reader, observed map[string]int64) error {
	n, err := scan(r, observed, true)
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

func scan(r io.Reader, observed map[string]int64, strict bool) (int, error) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1<<16), 8<<20)
	started := map[string]bool{}
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
			if _, seen := observed[e.Package]; seen {
				return terminal, fmt.Errorf("package %s has two terminal outcomes", e.Package)
			}
			// The table admits only positive costs; a package without tests costs the minimum.
			observed[e.Package] = max(1, int64(math.Round(e.Elapsed*1000)))
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
	return terminal, nil
}

func encode(observed map[string]int64, revision, runURL string) ([]byte, error) {
	raw, err := json.MarshalIndent(map[string]any{
		"profile":      "corvint-ci-package-costs/0",
		"source":       map[string]string{"revision": revision, "runURL": runURL, "goVersion": "go1.27.1"},
		"milliseconds": observed,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if _, ok := cishards.Costs(raw); !ok {
		return nil, errors.New("refused: the partition would reject this table (check --revision, --run-url and package times)")
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
func report(out io.Writer, summary, table string, logs []string, shards int, share, factor float64, floorMS int64) (int, error) {
	if len(logs) != shards {
		return 2, fmt.Errorf("%d of %d shard logs present; a partial run cannot measure the table", len(logs), shards)
	}
	observed := map[string]int64{}
	for _, name := range logs {
		f, err := os.Open(name)
		if err != nil {
			return 2, err
		}
		err = observeStream(f, observed)
		f.Close()
		if err != nil {
			return 2, fmt.Errorf("%s: %w", filepath.Base(name), err)
		}
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

	var b strings.Builder
	b.WriteString(reportHeading)
	fmt.Fprintf(&b, "\n%d shard logs, %d packages, %s observed; ideal shard %s. A finding is material when it misplaces at least %g%% of the ideal shard (%s). Costs change placement, never membership, and this report never fails CI. Refreshing `%s` is an operator step (`go run ./tools/ci-shard-costs refresh`, AFP-V0-022).\n\n",
		shards, len(observed), seconds(total), seconds(int64(ideal)), share, seconds(materialMS), table)
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
	if summary != "" {
		if err := appendFile(summary, b.String()); err != nil {
			return 2, err
		}
	}

	// One warning per material finding, keeping the last line for everything else.
	warnings := min(material, maxWarnings-1)
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
