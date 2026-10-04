package golang

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file reads the project-owned read-scope declaration (AFP-V0-023). Its
// grammar is mirrored by tools/gate-affected-select (readscopes.go) and by the
// protected confinement wrapper .github/testconfine, which enforces it; the
// three cannot share code because each is built apart from this module.

// FrontierReadScopesInvalid reports a read-scope declaration that is present
// but unreadable or invalid. No package is then declared, so every root-locating
// package stays an unbounded reader and the plan widens.
const FrontierReadScopesInvalid = "go:test-read-scopes-invalid"

// ReadScopesProfile is the declaration's closed profile name.
const ReadScopesProfile = "corvint-test-read-scopes/0"

const (
	maxReadScopeBytes    = 1 << 20
	maxReadScopePackages = 4096
	maxReadScopeEntries  = 256
)

type readScopeFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// readScopes returns the declared scopes by package directory: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file, or invalid in any member.
func readScopes(root *affected.Source) (map[string][]string, error) {
	body, err := root.ReadBounded(affected.ReadScopesPath, maxReadScopeBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared readScopeFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != ReadScopesProfile || declared.Packages == nil || len(declared.Packages) > maxReadScopePackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", affected.ReadScopesPath, ReadScopesProfile, maxReadScopePackages)
	}
	for directory, entries := range declared.Packages {
		if !affected.ValidRelativePath(directory) || entries == nil || len(entries) > maxReadScopeEntries {
			return nil, fmt.Errorf("%s: package %q", affected.ReadScopesPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !affected.ValidReadScopeEntry(entry) || entry <= previous {
				return nil, fmt.Errorf("%s: package %q entry %q", affected.ReadScopesPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// applyReadScopes marks each declared unit, given each unit's directory. Any
// failure, including a declaration naming a directory that holds no observed
// package, declares nothing and widens every plan instead.
func applyReadScopes(root *affected.Source, units []affected.Unit, directories []string, frontier map[string]bool) {
	scopes, err := readScopes(root)
	if err == nil {
		err = matchReadScopes(scopes, units, directories)
	}
	if err != nil {
		frontier[FrontierReadScopesInvalid] = true
		for index := range units {
			units[index].ReadScoped, units[index].ReadScope = false, nil
		}
	}
}

func matchReadScopes(scopes map[string][]string, units []affected.Unit, directories []string) error {
	matched := 0
	for index, directory := range directories {
		entries, declared := scopes[directory]
		if !declared {
			continue
		}
		units[index].ReadScoped, units[index].ReadScope = true, append([]string{}, entries...)
		matched++
	}
	if matched != len(scopes) {
		return fmt.Errorf("%s declares a directory that holds no Go package", affected.ReadScopesPath)
	}
	return nil
}
