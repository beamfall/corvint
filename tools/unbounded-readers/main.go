// SPDX-License-Identifier: Apache-2.0
// unbounded-readers is the AFP-V0-025 ratchet: it fails when a test unit selected on
// every change (AFP-V0-012 rule (d)) is not in the repository's recorded set.
package main

import (
	"context"
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
	Unreasoned []string `json:"unreasoned"`
	Stale      []string `json:"stale"`
}

type record struct {
	Profile string            `json:"profile"`
	Units   []string          `json:"units"`
	Reasons map[string]string `json:"reasons"`
}

// readRecord reads the closed {profile, units, reasons} record. units names the
// admitted unbounded test directories, strictly ascending; reasons maps them to
// the unbounded read and why it cannot be declared (AFP-V0-023). A unit without
// a reason is reported by run (AFP-V0-033), not refused here, so the message
// names it.
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
// the ones any change selects. A unit whose identity is its one test file (a
// shell test, NSA-V0-011) is listed by that file, since one directory holds
// many such units. It also maps each listed directory to its unit ids.
func unboundedTests(graph *affected.Graph) ([]string, map[string][]string) {
	directories := []string{}
	ids := map[string][]string{}
	for _, id := range graph.UnboundedReaders() {
		unit, ok := graph.Unit(id)
		directory := ""
		switch {
		case !ok || len(unit.Tests) == 0:
			continue
		case len(unit.Tests) == 1 && strings.HasSuffix(id, ":"+unit.Tests[0]):
			directory = unit.Tests[0]
		default:
			directory = path.Dir(unit.Tests[0])
		}
		directories = append(directories, directory)
		ids[directory] = append(ids[directory], id)
	}
	sort.Strings(directories)
	return directories, ids
}

// explain names, for one unrecorded unit, the import chain that makes it an
// unbounded reader (AFP-V0-500): the importing unit, each edge, and the unit
// whose own read is unbounded, so a new import is named instead of bisected.
func explain(graph *affected.Graph, id string) string {
	chain, reads := graph.UnboundedChain(id)
	if len(chain) == 0 {
		return id + ": no recorded cause"
	}
	if len(chain) == 1 {
		return fmt.Sprintf("%s: its own read is unbounded (%s)", id, reads)
	}
	edges := make([]string, 0, len(chain)-1)
	for i := len(chain) - 1; i > 0; i-- {
		unit, _ := graph.Unit(chain[i])
		verb := " reaches "
		switch {
		case slices.Contains(unit.Imports, chain[i-1]):
			verb = " imports "
		case slices.Contains(unit.TestImports, chain[i-1]):
			verb = " test-imports "
		}
		edges = append(edges, chain[i]+verb+chain[i-1])
	}
	return fmt.Sprintf("%s: import chain %s; %s is the unbounded reader (%s)", id, strings.Join(edges, ", "), chain[0], reads)
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
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	units, ids := unboundedTests(graph)
	// A set, not a count: two changes that each add a unit and its entry merge to a
	// record that still names both, where two identical edits of a count would not.
	unrecorded, unreasoned, stale := []string{}, []string{}, []string{}
	for i, directory := range units {
		// A second unbounded unit in one directory has no entry of its own.
		if !slices.Contains(recorded.Units, directory) || i > 0 && units[i-1] == directory {
			unrecorded = append(unrecorded, directory)
		}
	}
	for _, directory := range recorded.Units {
		if !slices.Contains(units, directory) {
			stale = append(stale, directory)
		}
		// Every admitted unit names its unbounded read (AFP-V0-033); a stale one
		// still needs it, so removing a reason never passes in place of the entry.
		if _, ok := recorded.Reasons[directory]; !ok {
			unreasoned = append(unreasoned, directory)
		}
	}
	out := report{Profile: profile, OK: len(unrecorded) == 0 && len(unreasoned) == 0, Count: len(units), Units: units, Unrecorded: unrecorded, Unreasoned: unreasoned, Stale: stale}
	encoded, err := json.Marshal(out, jsontext.WithIndent("  "))
	if err != nil {
		fmt.Fprintln(stderr, "unbounded-readers:", err)
		return 2
	}
	fmt.Fprintln(stdout, string(encoded))
	// A stale entry only reports: failing on it would break a merge of one change
	// that bounds a package with another that still lists it.
	if len(stale) > 0 {
		fmt.Fprintf(stderr, "unbounded-readers: %s records %v, which are not unbounded test packages; remove the entries to keep the gain.\n", recordPath, stale)
	}
	if len(unrecorded) > 0 {
		fmt.Fprintf(stderr, "unbounded-readers: %v are selected on every change and are not in units of %s. Declare each package's reads in %s (AFP-V0-023) or keep them inside its directory; add it to units only when neither is possible, as for a package whose tests launch a nested sandbox recorded in .corvint/test-nested-sandboxes.json (AFP-V0-615); a shell test is registered as described in docs/SHELL-TEST-REGISTRATION.md.\n",
			unrecorded, recordPath, affected.ReadScopesPath)
		for _, directory := range slices.Compact(slices.Clone(unrecorded)) {
			for _, id := range ids[directory] {
				fmt.Fprintf(stderr, "  %s\n", explain(graph, id))
			}
		}
	}
	if len(unreasoned) > 0 {
		fmt.Fprintf(stderr, "unbounded-readers: %v in units of %s have no entry in reasons. Name the read that is unbounded and why %s cannot declare it (AFP-V0-033).\n",
			unreasoned, recordPath, affected.ReadScopesPath)
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
