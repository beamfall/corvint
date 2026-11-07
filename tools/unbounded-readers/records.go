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

// recordDirectories are the record directories the ratchet guards; each new
// entry of one is matched against every literal and glob of a package by
// Graph.RecordReaders (AFP-V0-2747), not one synthetic name.
var recordDirectories = []string{affected.SealedChangesDirectory, affected.BuildLogDirectory}

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
		bad = bad || !slices.Contains(recordDirectories, directory) || readers == nil
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
// AFP-V0-025 ratchet treats a stale entry. A walker of a record directory
// fails whether or not the record lists it (AFP-V0-2747): no selector selects
// it for a new entry, so a record entry would only hide a reader.
func checkRecordReaders(graph *affected.Graph, recorded map[string]map[string]string, stderr io.Writer) bool {
	ok := true
	directories := func(ids []string) map[string]bool {
		out := map[string]bool{}
		for _, id := range ids {
			if unit, found := graph.Unit(id); found && len(unit.Tests) > 0 {
				out[path.Dir(unit.Tests[0])] = true
			}
		}
		return out
	}
	for _, directory := range recordDirectories {
		found, walking := graph.RecordReaders(directory)
		readers, walkers := directories(found), directories(walking)
		if len(walkers) > 0 {
			ok = false
			fmt.Fprintf(stderr, "unbounded-readers: %v name %s's parent directory by a path literal, join no literal that reaches %s, and declare no read scope, so a walk or a name computed at run time could read every new entry while no selector selects them for one (AFP-V0-2747); a record entry cannot admit them. Declare each package's reads in %s (AFP-V0-023), with %s when its tests really read the directory, or mark a fixture literal with %s.\n",
				slices.Sorted(maps.Keys(walkers)), directory, directory, affected.ReadScopesPath, directory, golang.FixturePathMarker)
		}
		listed := recorded[directory]
		var unrecorded, stale []string
		for _, pkg := range slices.Sorted(maps.Keys(readers)) {
			if _, present := listed[pkg]; !present {
				unrecorded = append(unrecorded, pkg)
			}
		}
		for _, pkg := range slices.Sorted(maps.Keys(listed)) {
			if !readers[pkg] {
				stale = append(stale, pkg)
			}
		}
		if len(stale) > 0 {
			fmt.Fprintf(stderr, "unbounded-readers: %s lists %v under %s, which no longer read that directory; remove the entries to keep the gain.\n", recordReadersPath, stale, directory)
		}
		if len(unrecorded) > 0 {
			ok = false
			fmt.Fprintf(stderr, "unbounded-readers: %v name the record directory %s by a path literal or glob and declare no read scope, so every change that adds an entry there selects them (AFP-V0-2745). Declare each package's reads in %s (AFP-V0-023), with %s when its tests really read the directory and without it when the literal names a fixture repository's copy; mark a fixture literal with %s; or, only when neither is possible, add the package to %q of %s with the reason it reads the directory.\n",
				unrecorded, directory, affected.ReadScopesPath, directory, golang.FixturePathMarker, directory, recordReadersPath)
		}
	}
	return ok
}
