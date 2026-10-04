// SPDX-License-Identifier: AGPL-3.0-or-later
// unbounded-readers is the AFP-V0-025 ratchet: it fails unless the test units selected
// on every change (AFP-V0-012 rule (d)) are exactly the repository's recorded set.
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
	profile    = "corvint-unbounded-reader-set/0"
	recordPath = ".corvint/unbounded-readers.json"
	maxInput   = 1 << 16
)

type report struct {
	Profile    string   `json:"profile"`
	OK         bool     `json:"ok"`
	Count      int      `json:"count"`
	Units      []string `json:"units"`
	Unrecorded []string `json:"unrecorded"`
	Stale      []string `json:"stale"`
}

type record struct {
	Profile string            `json:"profile"`
	Units   []string          `json:"units"`
	Reasons map[string]string `json:"reasons"`
}

// readRecord reads the closed {profile, units, reasons} record. units names the
// admitted unbounded test directories, strictly ascending; reasons maps some of
// them to why their reads cannot be declared (AFP-V0-023).
func readRecord(root string) (record, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(recordPath)))
	if err != nil {
		return record{}, err
	}
	var out record
	// json/v2 matches member names exactly and refuses a duplicate member or trailing data.
	bad := len(raw) > maxInput || json.Unmarshal(raw, &out, json.RejectUnknownMembers(true)) != nil ||
		out.Profile != profile || out.Units == nil || out.Reasons == nil
	for i, directory := range out.Units {
		bad = bad || directory == "" || i > 0 && out.Units[i-1] >= directory
	}
	for directory, reason := range out.Reasons {
		bad = bad || !slices.Contains(out.Units, directory) || strings.TrimSpace(reason) == ""
	}
	if bad {
		return record{}, fmt.Errorf("%s: want exactly {\"profile\":%q,\"units\":[DIRECTORY...],\"reasons\":{DIRECTORY:REASON}} with units strictly ascending and every reasons directory in units", recordPath, profile)
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
	units := unboundedTests(graph)
	// A set, not a count: two changes that each add a unit and its entry merge to a
	// record that still names both, where two identical edits of a count would not.
	unrecorded, stale := []string{}, []string{}
	for _, directory := range units {
		if !slices.Contains(recorded.Units, directory) {
			unrecorded = append(unrecorded, directory)
		}
	}
	for _, directory := range recorded.Units {
		if !slices.Contains(units, directory) {
			stale = append(stale, directory)
		}
	}
	out := report{Profile: profile, OK: len(unrecorded) == 0 && len(stale) == 0, Count: len(units), Units: units, Unrecorded: unrecorded, Stale: stale}
	encoded, err := json.Marshal(out, jsontext.WithIndent("  "))
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	fmt.Fprintln(stdout, string(encoded))
	if len(unrecorded) > 0 {
		fmt.Fprintf(stderr, "unbounded-readers: %v are selected on every change and are not in units of %s. Declare each package's reads in %s (AFP-V0-023) or keep them inside its directory; add it to units only when neither is possible.\n",
			unrecorded, recordPath, affected.ReadScopesPath)
	}
	if len(stale) > 0 {
		fmt.Fprintf(stderr, "unbounded-readers: %s records %v, which are not unbounded test packages. Remove the entries to keep the gain.\n", recordPath, stale)
	}
	if !out.OK {
		return 1
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
