package shell

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
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

// applyShellReadScopes read-scopes each declared test's unit when the whole
// declaration holds: every declared test is an observed shell test whose only
// unbounded read is the toolchain run (toolchainOnly). Any failure declares
// nothing and names FrontierShellReadScopesInvalid.
func applyShellReadScopes(root *affected.Source, units []affected.Unit, toolchainOnly map[string]bool, frontier map[string]bool) {
	declared, err := readShellReadScopes(root)
	if err != nil {
		frontier[FrontierShellReadScopesInvalid] = true
		return
	}
	if declared == nil {
		return
	}
	indexes := make(map[string]int, len(declared))
	for index, unit := range units {
		if len(unit.Tests) == 1 && len(unit.Sources) == 0 {
			if _, named := declared[unit.Tests[0]]; named && toolchainOnly[unit.Tests[0]] && unit.UnboundedReads == UnboundedRunsToolchain {
				indexes[unit.Tests[0]] = index
			}
		}
	}
	if len(indexes) != len(declared) {
		frontier[FrontierShellReadScopesInvalid] = true
		return
	}
	for test, index := range indexes {
		units[index].UnboundedReads = ""
		units[index].ReadScoped, units[index].ReadScope = true, append([]string{}, declared[test].Reads...)
	}
}
