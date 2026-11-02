package shell

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file reads the project's shell test read-scope declaration
// (TSP-V0-950). A shell test that runs a build or language toolchain is an
// unbounded reader (NSA-V0-011); the declaration states the repository paths
// such a test reads outside its own directory, so the test is read-scoped as
// an AFP-V0-023 Go package is instead of being selected on every change. No
// wrapper confines a shell test, so the reads are the project's claim; a
// declaration naming anything other than a toolchain-only unbounded test
// declares nothing.

// FrontierShellReadScopesInvalid reports a shell read-scope declaration that
// is present but unreadable or invalid. Nothing is then declared.
const FrontierShellReadScopesInvalid = "shell:read-scopes-invalid"

// ReadScopesProfile is the declaration's closed profile name.
const ReadScopesProfile = "corvint-shell-read-scopes/0"

const (
	maxShellReadScopeBytes  = 64 << 10
	maxShellReadScopeTests  = 64
	maxShellReadScopeReads  = 256
	maxShellReadScopeReason = 512
)

// ReadScope is one declared shell test: the paths it reads outside its
// directory and the project's stated reason.
type ReadScope struct {
	Reads  []string `json:"reads"`
	Reason string   `json:"reason"`
}

type readScopesFile struct {
	Profile string               `json:"profile"`
	Tests   map[string]ReadScope `json:"tests"`
}

// readShellReadScopes returns the declared tests by path: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file or invalid in any member.
func readShellReadScopes(root *affected.Source) (map[string]ReadScope, error) {
	body, err := root.ReadBounded(affected.ShellReadScopesPath, maxShellReadScopeBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared readScopesFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != ReadScopesProfile || len(declared.Tests) == 0 || len(declared.Tests) > maxShellReadScopeTests {
		return nil, fmt.Errorf("%s: profile must be %s with 1 to %d tests", affected.ShellReadScopesPath, ReadScopesProfile, maxShellReadScopeTests)
	}
	for test, entry := range declared.Tests {
		if !affected.ValidRelativePath(test) || !hasAnySuffix(test, Suffixes) {
			return nil, fmt.Errorf("%s: test %q", affected.ShellReadScopesPath, test)
		}
		if len(entry.Reads) == 0 || len(entry.Reads) > maxShellReadScopeReads || !sortedUnique(entry.Reads) {
			return nil, fmt.Errorf("%s: test %q reads must be 1 to %d sorted unique entries", affected.ShellReadScopesPath, test, maxShellReadScopeReads)
		}
		for _, read := range entry.Reads {
			if !affected.ValidReadScopeEntry(read) {
				return nil, fmt.Errorf("%s: test %q read %q", affected.ShellReadScopesPath, test, read)
			}
		}
		if entry.Reason == "" || len(entry.Reason) > maxShellReadScopeReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: test %q reason", affected.ShellReadScopesPath, test)
		}
	}
	return declared.Tests, nil
}

// applyShellReadScopes read-scopes each declared test's unit that is an
// observed shell test whose only unbounded read is the toolchain run
// (toolchainOnly). A declaration that does not read declares nothing and names
// FrontierShellReadScopesInvalid; an entry that names no such unit declares
// nothing for that test alone and names it (TSP-V0-831).
func applyShellReadScopes(root *affected.Source, units []affected.Unit, toolchainOnly map[string]bool, frontier map[string]bool) {
	declared, err := readShellReadScopes(root)
	if err != nil {
		frontier[FrontierShellReadScopesInvalid] = true
		return
	}
	if declared == nil {
		return
	}
	byID := make(map[string]affected.Unit, len(units))
	for _, unit := range units {
		byID[unit.ID] = unit
	}
	indexes := make(map[string]int, len(declared))
	invalid := make(map[string]string, len(declared))
	for index, unit := range units {
		if len(unit.Tests) != 1 || len(unit.Sources) != 0 {
			continue
		}
		test := unit.Tests[0]
		if _, named := declared[test]; !named {
			continue
		}
		if reason := invalidEntry(unit, toolchainOnly[test], byID); reason != "" {
			invalid[test] = reason
			continue
		}
		indexes[test] = index
	}
	for test := range declared {
		if _, applied := indexes[test]; !applied && invalid[test] == "" {
			invalid[test] = "is not an observed shell test"
		}
	}
	for test, reason := range invalid {
		frontier[FrontierShellReadScopesInvalid+":"+test+" "+reason] = true
	}
	for test, index := range indexes {
		units[index].UnboundedReads = ""
		units[index].ReadScoped, units[index].ReadScope = true, append([]string{}, declared[test].Reads...)
	}
}

// invalidEntry returns why a declared shell test unit cannot be read-scoped,
// or "" when it can (TSP-V0-831).
func invalidEntry(unit affected.Unit, toolchainOnly bool, byID map[string]affected.Unit) string {
	if !toolchainOnly || unit.UnboundedReads != UnboundedRunsToolchain {
		return "runs no toolchain"
	}
	if reached := reachesUnbounded(unit, byID); reached != "" {
		return "reaches " + reached + ", an unbounded reader"
	}
	return ""
}

// reachesUnbounded returns the smallest id of a unit that a unit reaches,
// through the files it sources, runs or executes (Imports, TestImports,
// Execs), whose own reads are unbounded or that locates the repository root,
// or "" when it reaches none. A read-scoped unit does not inherit such reads,
// so a declaration over it would hide them (TSP-V0-950).
func reachesUnbounded(unit affected.Unit, byID map[string]affected.Unit) string {
	seen := map[string]bool{unit.ID: true}
	pending := slices.Concat(unit.Imports, unit.TestImports, unit.Execs)
	found := ""
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		reached, known := byID[id]
		if !known {
			continue
		}
		if reached.UnboundedReads != "" || reached.LocatesRoot {
			if found == "" || id < found {
				found = id
			}
		}
		pending = append(pending, slices.Concat(reached.Imports, reached.TestImports, reached.Execs)...)
	}
	return found
}
