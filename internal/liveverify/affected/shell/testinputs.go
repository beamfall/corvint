// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file reads the project's shell test input declaration (AFP-V0-2400,
// V1-3489). A shell test can read repository paths through a word the plugin
// does not see as a path, such as the bare `Makefile` a scan prints, or a
// `find script -name '*.sh'` listing. The declaration names those inputs, and
// each one joins the test unit's path globs, so a dirty path it holds selects
// the test (PATH_GLOB_READER, witnessed by the declared glob). It only adds:
// the unit keeps every literal, edge and unbounded read the plugin observed,
// so no declaration, stale or wrong, can narrow a selection.

// ShellTestInputsPath is the repository's declaration of the inputs each
// declared shell test reads by a word the plugin does not resolve. A plan with
// it dirty, deleted included, is UNKNOWN (affected.FrontierShellTestInputsChanged).
const ShellTestInputsPath = affected.ShellTestInputsPath

// ShellTestInputsProfile is the declaration's closed profile name.
const ShellTestInputsProfile = "corvint-shell-test-inputs/0"

// FrontierShellTestInputsInvalid reports a shell test input declaration that
// is present but unreadable or invalid, or an entry that names no observed
// shell test. Nothing is then added for it, and the plugin-wide frontier names
// it, so a plan the shell plugin takes part in is UNKNOWN.
const FrontierShellTestInputsInvalid = "shell:test-inputs-invalid"

const (
	maxShellTestInputBytes  = 64 << 10
	maxShellTestInputTests  = 64
	maxShellTestInputs      = 64
	maxShellTestInputReason = 512
)

// TestInputs is one declared shell test: the inputs it reads and the
// project's stated reason.
type TestInputs struct {
	Inputs []string `json:"inputs"`
	Reason string   `json:"reason"`
}

type testInputsFile struct {
	Profile string                `json:"profile"`
	Tests   map[string]TestInputs `json:"tests"`
}

// validTestInput reports whether entry is a repository-relative file, a
// directory ending in one `/`, or a pattern whose last component holds a
// `path.Match` wildcard below such a directory (`script/*.sh`).
func validTestInput(entry string) bool {
	if affected.ValidReadScopeEntry(entry) && !strings.ContainsAny(entry, "*?[\\") {
		return true
	}
	directory, name := path.Split(entry)
	if directory == "" || !affected.ValidReadScopeEntry(directory) || strings.ContainsAny(directory, "*?[\\") || name == "" || !strings.ContainsAny(name, "*?[") || strings.Contains(name, "/") {
		return false
	}
	_, err := path.Match(name, "")
	return err == nil
}

// inputGlob is the anchored glob a declared input joins the unit with: a
// file or directory matches itself and every path below it, a pattern the
// paths its last component matches.
func inputGlob(entry string) string {
	return "/" + strings.TrimSuffix(entry, "/")
}

// readShellTestInputs returns the declared tests by path: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file or invalid in any member.
func readShellTestInputs(root *affected.Source) (map[string]TestInputs, error) {
	body, err := root.ReadBounded(ShellTestInputsPath, maxShellTestInputBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared testInputsFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != ShellTestInputsProfile || len(declared.Tests) == 0 || len(declared.Tests) > maxShellTestInputTests {
		return nil, fmt.Errorf("%s: profile must be %s with 1 to %d tests", ShellTestInputsPath, ShellTestInputsProfile, maxShellTestInputTests)
	}
	for test, entry := range declared.Tests {
		if !affected.ValidRelativePath(test) || !hasAnySuffix(test, Suffixes) {
			return nil, fmt.Errorf("%s: test %q", ShellTestInputsPath, test)
		}
		if len(entry.Inputs) == 0 || len(entry.Inputs) > maxShellTestInputs || !sortedUnique(entry.Inputs) {
			return nil, fmt.Errorf("%s: test %q inputs must be 1 to %d sorted unique entries", ShellTestInputsPath, test, maxShellTestInputs)
		}
		for _, input := range entry.Inputs {
			if !validTestInput(input) {
				return nil, fmt.Errorf("%s: test %q input %q", ShellTestInputsPath, test, input)
			}
		}
		if entry.Reason == "" || len(entry.Reason) > maxShellTestInputReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: test %q reason", ShellTestInputsPath, test)
		}
	}
	return declared.Tests, nil
}

// applyShellTestInputs adds each declared test's inputs to its observed shell
// test unit's path globs (AFP-V0-2400). An invalid declaration adds nothing
// and names FrontierShellTestInputsInvalid; an entry that names no observed
// shell test adds nothing and is named the same way with its test.
func applyShellTestInputs(root *affected.Source, units []affected.Unit, frontier map[string]bool) {
	declared, err := readShellTestInputs(root)
	if err != nil {
		frontier[FrontierShellTestInputsInvalid] = true
		return
	}
	applied := make(map[string]bool, len(declared))
	for index, unit := range units {
		if len(unit.Tests) != 1 || len(unit.Sources) != 0 {
			continue
		}
		entry, named := declared[unit.Tests[0]]
		if !named {
			continue
		}
		globs := append([]string{}, unit.PathGlobs...)
		for _, input := range entry.Inputs {
			globs = append(globs, inputGlob(input))
		}
		units[index].PathGlobs = sortedUniqueStrings(globs)
		applied[unit.Tests[0]] = true
	}
	for test := range declared {
		if !applied[test] {
			frontier[FrontierShellTestInputsInvalid+":"+test+" is not an observed shell test"] = true
		}
	}
}

// sortedUniqueStrings returns values sorted with duplicates removed.
func sortedUniqueStrings(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return sortedKeys(set)
}
