// SPDX-License-Identifier: Apache-2.0
package main

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// recordReadersPath is the AFP-V0-2745 ratchet record: for each record
// directory, the Go test packages without a declared read scope that every new
// entry of the directory selects as a literal or glob reader, each with the
// reason it reads the directory or why its read scope is not declared.
const (
	recordReadersPath    = ".corvint/record-readers.json"
	recordReadersProfile = "corvint-record-readers/0"
)

// recordProbes names, per record directory, a probe entry that no literal
// names by its file name, so only readers of the directory itself select it.
var recordProbes = []struct{ directory, probe string }{
	{affected.SealedChangesDirectory, affected.SealedChangesDirectory + strings.Repeat("f", 40) + ".cem.json"},
	{affected.BuildLogDirectory, affected.BuildLogDirectory + "0000-00-00-record-readers-probe.md"},
}

type recordReaders struct {
	Profile     string                       `json:"profile"`
	Directories map[string]map[string]string `json:"directories"`
}

// readRecordReaders reads the closed record; an absent record is the empty
// set for every directory, so a repository with no record directory reader
// needs none. A record that is oversized, not exactly the grammar, names a
// directory other than the probed ones, or holds a blank reason is refused.
func readRecordReaders(root string) (map[string]map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(recordReadersPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out recordReaders
	bad := len(raw) > maxInput || json.Unmarshal(raw, &out, json.RejectUnknownMembers(true)) != nil ||
		out.Profile != recordReadersProfile || out.Directories == nil
	for directory, readers := range out.Directories {
		bad = bad || !slices.ContainsFunc(recordProbes, func(p struct{ directory, probe string }) bool { return p.directory == directory }) || readers == nil
		for _, reason := range readers {
			bad = bad || strings.TrimSpace(reason) == ""
		}
	}
	if bad {
		return nil, fmt.Errorf("%s: want exactly {\"profile\":%q,\"directories\":{DIRECTORY:{PACKAGE:REASON}}} with DIRECTORY one of %s and %s and every reason non-blank", recordReadersPath, recordReadersProfile, affected.SealedChangesDirectory, affected.BuildLogDirectory)
	}
	return out.Directories, nil
}

// checkRecordReaders is the AFP-V0-2745 ratchet. It reports false, naming
// each one on stderr, when a Go test package without a declared read scope
// reads a record directory and the record does not list it; a listed package
// that no longer reads the directory is reported without failing, as the
// AFP-V0-025 ratchet treats a stale entry.
func checkRecordReaders(graph *affected.Graph, recorded map[string]map[string]string, stderr io.Writer) bool {
	ok := true
	for _, record := range recordProbes {
		readers := map[string]bool{}
		for _, id := range graph.RecordReaders(record.probe) {
			if unit, found := graph.Unit(id); found && len(unit.Tests) > 0 {
				readers[path.Dir(unit.Tests[0])] = true
			}
		}
		listed := recorded[record.directory]
		var unrecorded, stale []string
		for _, directory := range slices.Sorted(maps.Keys(readers)) {
			if _, present := listed[directory]; !present {
				unrecorded = append(unrecorded, directory)
			}
		}
		for _, directory := range slices.Sorted(maps.Keys(listed)) {
			if !readers[directory] {
				stale = append(stale, directory)
			}
		}
		if len(stale) > 0 {
			fmt.Fprintf(stderr, "unbounded-readers: %s lists %v under %s, which no longer read that directory; remove the entries to keep the gain.\n", recordReadersPath, stale, record.directory)
		}
		if len(unrecorded) > 0 {
			ok = false
			fmt.Fprintf(stderr, "unbounded-readers: %v name the record directory %s by a path literal or glob and declare no read scope, so every change that adds an entry there selects them (AFP-V0-2745). Declare each package's reads in %s (AFP-V0-023), with %s when its tests really read the directory and without it when the literal names a fixture repository's copy; mark a fixture literal with %s; or, only when neither is possible, add the package to %q of %s with the reason it reads the directory.\n",
				unrecorded, record.directory, affected.ReadScopesPath, record.directory, golang.FixturePathMarker, record.directory, recordReadersPath)
		}
	}
	return ok
}
