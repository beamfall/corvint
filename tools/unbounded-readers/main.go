// SPDX-License-Identifier: AGPL-3.0-or-later
// unbounded-readers is the AFP-V0-025 ratchet: it fails when more test units are
// selected on every change (AFP-V0-012 rule (d)) than the repository's recorded ceiling.
package main

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

const (
	profile     = "corvint-unbounded-reader-ceiling/0"
	ceilingPath = ".corvint/unbounded-readers.json"
	maxInput    = 1 << 16
)

type report struct {
	Profile string   `json:"profile"`
	OK      bool     `json:"ok"`
	Ceiling int      `json:"ceiling"`
	Count   int      `json:"count"`
	Units   []string `json:"units"`
	Stale   []string `json:"staleReasons"`
}

type record struct {
	Profile string            `json:"profile"`
	Ceiling *int              `json:"ceiling"`
	Reasons map[string]string `json:"reasons"`
}

// readRecord reads the closed {profile, ceiling, reasons} record. reasons maps a
// test directory to why its reads cannot be declared (AFP-V0-023).
func readRecord(root string) (record, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ceilingPath)))
	if err != nil {
		return record{}, err
	}
	var out record
	// json/v2 matches member names exactly and refuses a duplicate member or trailing data.
	bad := len(raw) > maxInput || json.Unmarshal(raw, &out, json.RejectUnknownMembers(true)) != nil ||
		out.Profile != profile || out.Ceiling == nil || *out.Ceiling < 0 || out.Reasons == nil
	for directory, reason := range out.Reasons {
		bad = bad || directory == "" || strings.TrimSpace(reason) == ""
	}
	if bad {
		return record{}, fmt.Errorf("%s: want exactly {\"profile\":%q,\"ceiling\":N,\"reasons\":{DIRECTORY:REASON}} with N >= 0", ceilingPath, profile)
	}
	return out, nil
}

// unboundedTests lists the directories of the unbounded units that have tests:
// the ones any change selects.
func unboundedTests(graph *affected.Graph) []string {
	directories := []string{}
	for _, id := range graph.UnboundedReaders() {
		if unit, ok := graph.Unit(id); ok && len(unit.Tests) > 0 {
			directories = append(directories, path.Dir(unit.Tests[0]))
		}
	}
	sort.Strings(directories)
	return directories
}

func run(root string, stdout, stderr io.Writer) int {
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	recorded, err := readRecord(root)
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	graph, err := affected.Build(root, languages.All()...)
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	units, ceiling := unboundedTests(graph), *recorded.Ceiling
	stale := []string{}
	for directory := range recorded.Reasons {
		if !slices.Contains(units, directory) {
			stale = append(stale, directory)
		}
	}
	sort.Strings(stale)
	out := report{Profile: profile, OK: len(units) <= ceiling && len(stale) == 0, Ceiling: ceiling, Count: len(units), Units: units, Stale: stale}
	encoded, err := json.Marshal(out, jsontext.WithIndent("  "))
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	fmt.Fprintln(stdout, string(encoded))
	switch {
	case len(stale) > 0:
		fmt.Fprintf(stderr, "unbounded-readers: %s records a reason for %v, which is not an unbounded test package. Remove the entry.\n", ceilingPath, stale)
		return 1
	case !out.OK:
		fmt.Fprintf(stderr, "unbounded-readers: %d test units are selected on every change, above the ceiling %d in %s. Declare the new package's reads in %s (AFP-V0-023) or keep them inside its directory.\n",
			out.Count, ceiling, ceilingPath, affected.ReadScopesPath)
		return 1
	case out.Count < ceiling:
		fmt.Fprintf(stderr, "unbounded-readers: %d is below the ceiling %d; lower %s to keep the gain.\n", out.Count, ceiling, ceilingPath)
	}
	return 0
}

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: unbounded-readers [--root DIR]")
		os.Exit(2)
	}
	os.Exit(run(*root, os.Stdout, os.Stderr))
}
