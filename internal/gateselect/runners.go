// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// This file gives rule (d) the execution edges of the planner's Go plugin
// (AFP-V0-037, internal/liveverify/affected/golang/binaryexecs.go): a package
// that runs a command's built binary runs the command's code, so a root
// locator in that code reaches it as an import would (AFP-V0-2992). The
// selector reads only repository Go sources, so it cannot follow a literal
// to the call that uses it as the plugin does; it takes every literal that
// can name a command directory as an edge instead, which selects the
// plugin's runners and possibly more, never fewer.

const (
	binaryExecsPath       = ".corvint/test-binary-execs.json"
	binaryExecsProfile    = "corvint-test-binary-execs/0"
	maxBinaryExecBytes    = 1 << 20
	maxBinaryExecPackages = 4096
	maxBinaryExecEntries  = 256
)

type binaryExecsFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// linkRunners records, for every command package, the packages holding a
// path token that can name its directory: in a non-test file the package's
// code may run the binary (codeRunners), and in test files alone only its
// tests may (testRunners). A token names a directory as the plugin's
// commandDirectory resolves it (root-anchored and plain tokens against the
// module root, climbing ones against the holder's directory) or as one of
// its component runs, the relation namers use.
func (index *repositoryIndex) linkRunners() {
	code := map[string]map[string]bool{}
	tests := map[string]map[string]bool{}
	for value, directories := range index.holders {
		for _, directory := range directories {
			for _, target := range index.commandTargets(value, directory) {
				edges := tests
				if slices.Contains(index.nonTestHolders[value], directory) {
					edges = code
				}
				if edges[target] == nil {
					edges[target] = map[string]bool{}
				}
				edges[target][directory] = true
			}
		}
	}
	for target, runners := range code {
		index.codeRunners[target] = sortedKeys(runners)
	}
	for target, runners := range tests {
		for runner := range runners {
			if code[target][runner] {
				delete(runners, runner)
			}
		}
		if len(runners) != 0 {
			index.testRunners[target] = sortedKeys(runners)
		}
	}
}

// commandTargets lists the command package directories, other than
// directory, that value held there can name.
func (index *repositoryIndex) commandTargets(value, directory string) []string {
	candidates := map[string]bool{}
	resolved := path.Clean(strings.TrimPrefix(value, "/"))
	if value == ".." || strings.HasPrefix(value, "../") {
		resolved = path.Join(directory, value)
	}
	candidates[resolved] = true
	for _, run := range componentRuns(value) {
		candidates[strings.Join(run.components, "/")] = true
	}
	var targets []string
	for candidate := range candidates {
		if pkg := index.packages[candidate]; pkg != nil && pkg.command && candidate != directory {
			targets = append(targets, candidate)
		}
	}
	return targets
}

// applyBinaryExecs adds the declared edges of .corvint/test-binary-execs.json
// as code runners: the declaration does not say which file runs the binary.
// A declared or listed directory inside a nested module is outside this
// index and is skipped. A declaration that is unreadable, oversized, not a
// regular file, or invalid in any other member sets execsErr, and rule (d)
// widens to every package (unresolved).
func (index *repositoryIndex) applyBinaryExecs(root string) {
	declared, err := readBinaryExecs(root)
	if err != nil {
		index.execsErr = err
		return
	}
	for _, directory := range sortedKeys(declared) {
		if index.nestedModule(directory + "/") {
			continue
		}
		if index.packages[directory] == nil {
			index.execsErr = fmt.Errorf("%s: package %q holds no Go package", binaryExecsPath, directory)
			return
		}
		for _, entry := range declared[directory] {
			if index.nestedModule(entry + "/") {
				continue
			}
			if pkg := index.packages[entry]; pkg == nil || !pkg.command {
				index.execsErr = fmt.Errorf("%s: package %q entry %q holds no command", binaryExecsPath, directory, entry)
				return
			}
			if !slices.Contains(index.codeRunners[entry], directory) {
				index.codeRunners[entry] = append(index.codeRunners[entry], directory)
				slices.Sort(index.codeRunners[entry])
			}
			if runners := slices.DeleteFunc(index.testRunners[entry], func(runner string) bool { return runner == directory }); len(runners) != 0 {
				index.testRunners[entry] = runners
			} else {
				delete(index.testRunners, entry)
			}
		}
	}
}

// readBinaryExecs returns the declared command directories by package
// directory: nil with no declaration, an error for one that is unreadable,
// oversized, not a regular file, or invalid in any member, as the plugin's
// readBinaryExecs checks it.
func readBinaryExecs(root string) (map[string][]string, error) {
	name := filepath.Join(root, filepath.FromSlash(binaryExecsPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", binaryExecsPath)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxBinaryExecBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBinaryExecBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", binaryExecsPath, maxBinaryExecBytes)
	}
	var declared binaryExecsFile
	if err := jsonv2.UnmarshalRead(bytes.NewReader(body), &declared, jsonv2.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != binaryExecsProfile || declared.Packages == nil || len(declared.Packages) > maxBinaryExecPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", binaryExecsPath, binaryExecsProfile, maxBinaryExecPackages)
	}
	for directory, entries := range declared.Packages {
		if !validRelativePath(directory) || len(entries) == 0 || len(entries) > maxBinaryExecEntries {
			return nil, fmt.Errorf("%s: package %q", binaryExecsPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !validRelativePath(entry) || entry <= previous || entry == directory {
				return nil, fmt.Errorf("%s: package %q entry %q", binaryExecsPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// runReach is reach(false, directories) over the edges along which a package
// runs another's code: the closure over non-test imports and code runners,
// then one level of test importers and test runners of every package reached,
// seeds included, never expanded past that level (AFP-V0-2992). It is the
// reach of rule (d), as the planner's Graph.unboundedReaders walks it.
func (index *repositoryIndex) runReach(directories []string) []string {
	seen := map[string]bool{}
	queue := append([]string(nil), directories...)
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		for _, runner := range slices.Concat(index.importers[next], index.codeRunners[next]) {
			if !seen[runner] {
				seen[runner] = true
				queue = append(queue, runner)
			}
		}
	}
	reached := slices.Concat(directories, sortedKeys(seen))
	for _, directory := range reached {
		for _, user := range slices.Concat(index.testImporters[directory], index.testRunners[directory]) {
			seen[user] = true
		}
	}
	return sortedKeys(seen)
}
