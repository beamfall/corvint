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
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	if (mode != "refresh" && mode != "check") || fs.NArg() == 0 || !(*factor >= 1) || math.IsInf(*factor, 0) || *floor < 0 {
		return 2, errors.New("usage: ci-shard-costs refresh|check [flags] LOG...")
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
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1<<16), 8<<20)
	for s.Scan() {
		line := s.Bytes()
		i := bytes.IndexByte(line, '{')
		if i < 0 {
			continue
		}
		var e struct {
			Action  string
			Package string
			Test    string
			Elapsed float64
		}
		if json.Unmarshal(line[i:], &e) != nil || e.Package == "" || e.Test != "" {
			continue
		}
		switch e.Action {
		case "fail":
			return fmt.Errorf("package %s failed", e.Package)
		case "pass", "skip":
			if _, seen := observed[e.Package]; seen {
				return fmt.Errorf("package %s has two terminal outcomes", e.Package)
			}
			// The table admits only positive costs; a package without tests costs the minimum.
			observed[e.Package] = max(1, int64(math.Round(e.Elapsed*1000)))
		}
	}
	return s.Err()
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

func drift(recorded, observed map[string]int64, factor float64, floorMS int64) []string {
	var lines []string
	for p, got := range observed {
		want, ok := recorded[p]
		if !ok {
			lines = append(lines, fmt.Sprintf("missing %s observed=%dms", p, got))
			continue
		}
		lo, hi := min(got, want), max(got, want)
		if hi-lo >= floorMS && float64(hi) > factor*float64(lo) {
			lines = append(lines, fmt.Sprintf("drift %s recorded=%dms observed=%dms", p, want, got))
		}
	}
	for p, want := range recorded {
		if _, ok := observed[p]; !ok {
			lines = append(lines, fmt.Sprintf("stale %s recorded=%dms", p, want))
		}
	}
	sort.Strings(lines)
	return lines
}
