// SPDX-License-Identifier: AGPL-3.0-or-later
// test-read-scopes-check fails when a test file of a package declared in
// .corvint/test-read-scopes.json names, as a string literal, a tracked
// repository path outside that package's directory and declared entries
// (AFP-V0-023). The confined CI shard would deny that read with an error that
// looks like a content failure; this check names the package and path first.
package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// rootLocatorToken is the Git option that finds the repository root, split so
// that this file does not carry it.
var rootLocatorToken = "--show-" + "toplevel"

const (
	// changeEvidencePath is the shared change-evidence map, tracked only between
	// a dogfood bind and its seal, which moves it (DCW-V0-027). Tests name it
	// for the fixture repositories they write, and it is no stable repository
	// path, so it never counts as tracked here.
	changeEvidencePath = ".corvint/change.cem.json"
	literalsPath       = ".corvint/test-read-scope-literals.json"
	literalsProfile    = "corvint-test-read-scope-literals/0"
	maxLiterals        = 1 << 16
)

// readLiterals reads the closed {profile, literals} record of literals the
// check reports but no confined test reads: fixture contents, expected
// messages, or a test the confined shard never runs. literals maps a declared
// package directory to each such repository path and the reason it is no read.
// An absent record is empty.
func readLiterals(root string) (map[string]map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(literalsPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var record struct {
		Profile  string                       `json:"profile"`
		Literals map[string]map[string]string `json:"literals"`
	}
	// json/v2 matches member names exactly and refuses a duplicate member or trailing data.
	bad := len(raw) > maxLiterals || json.Unmarshal(raw, &record, json.RejectUnknownMembers(true)) != nil ||
		record.Profile != literalsProfile || record.Literals == nil
	for _, paths := range record.Literals {
		bad = bad || len(paths) == 0
		for _, reason := range paths {
			bad = bad || strings.TrimSpace(reason) == ""
		}
	}
	if bad {
		return nil, fmt.Errorf("%s: want exactly {\"profile\":%q,\"literals\":{PACKAGE:{PATH:REASON}}} with a non-empty reason for every path", literalsPath, literalsProfile)
	}
	return record.Literals, nil
}

// finding is one test file literal that names an undeclared repository path.
type finding struct {
	pkg, file, path string
}

// tracked lists the paths Git tracks under root, except changeEvidencePath,
// and every directory above them, compared exactly: a case-insensitive file
// system must not turn a literal "makefile" into the tracked Makefile.
func tracked(root string) (map[string]bool, error) {
	command := exec.Command("git", "-C", root, "ls-files", "-z")
	command.Stderr = io.Discard
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	paths := map[string]bool{}
	for _, name := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if string(name) == changeEvidencePath {
			continue
		}
		for value := string(name); value != "." && !paths[value]; value = path.Dir(value) {
			paths[value] = true
		}
	}
	return paths, nil
}

// locatesRoot reports a package whose tests can reach the repository root, as
// AFP-V0-021 narrows a root-relative reader: one of their tokens is made only
// of parent components, or is the Git root lookup.
func locatesRoot(tokens [][]string) bool {
	for _, file := range tokens {
		for _, value := range file {
			if value == rootLocatorToken || parentOnly(value) {
				return true
			}
		}
	}
	return false
}

func parentOnly(value string) bool {
	parent := false
	for _, component := range strings.Split(value, "/") {
		switch component {
		case "", ".":
		case "..":
			parent = true
		default:
			return false
		}
	}
	return parent
}

// named resolves one AFP-V0-021 token of a test file in directory to every
// tracked repository path of two or more components it names. The token
// resolves against the package directory, where `go test` runs it; a token
// without `.` or `..` components also resolves against the repository root when
// the package's tests locate it, and both are kept, since a match in the package
// does not show which one the test reads. A "/"-anchored token (an import path
// under the module, or an absolute path) names nothing here, and one that climbs
// out of the repository is rule (d)'s.
func named(paths map[string]bool, directory, value string, fromRoot bool) []string {
	if value == "" || strings.HasPrefix(value, "/") {
		return nil
	}
	candidates := []string{path.Join(directory, value)}
	if fromRoot && !slices.ContainsFunc(strings.Split(value, "/"), func(component string) bool { return component == "." || component == ".." }) {
		candidates = append(candidates, path.Clean(value))
	}
	var targets []string
	for _, candidate := range candidates {
		if strings.Contains(candidate, "/") && paths[candidate] && !slices.Contains(targets, candidate) {
			targets = append(targets, candidate)
		}
	}
	return targets
}

// covered reports whether a confined test of directory may read target: it is
// in the package's subtree, is or lies under a declared entry, or is a directory
// above the package or an entry, which a test climbs through or joins further
// components onto.
func covered(directory, target string, entries []string) bool {
	if target == directory || strings.HasPrefix(target, directory+"/") || strings.HasPrefix(directory, target+"/") {
		return true
	}
	for _, entry := range entries {
		file, subtree := strings.CutSuffix(entry, "/")
		if target == file || subtree && strings.HasPrefix(target, entry) || strings.HasPrefix(entry, target+"/") {
			return true
		}
	}
	return false
}

// check returns the unrecorded findings and the recorded literals that no test
// file names any more.
func check(root string) ([]finding, []string, error) {
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		return nil, nil, err
	}
	if slices.Contains(graph.Frontier(), golang.FrontierReadScopesInvalid) {
		return nil, nil, errors.New(affected.ReadScopesPath + " is invalid, so no package is declared")
	}
	paths, err := tracked(root)
	if err != nil {
		return nil, nil, err
	}
	recorded, err := readLiterals(root)
	if err != nil {
		return nil, nil, err
	}
	used := map[string]bool{}
	var findings []finding
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if !unit.ReadScoped || len(unit.Tests) == 0 {
			continue
		}
		directory := path.Dir(unit.Tests[0])
		tokens := make([][]string, len(unit.Tests))
		for index, file := range unit.Tests {
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				return nil, nil, err
			}
			if tokens[index], err = golang.FilePathTokens(file, body, ""); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", file, err)
			}
		}
		fromRoot := locatesRoot(tokens)
		for index, file := range unit.Tests {
			seen := map[string]bool{}
			for _, value := range tokens[index] {
				for _, target := range named(paths, directory, value, fromRoot) {
					if seen[target] || covered(directory, target, unit.ReadScope) {
						continue
					}
					seen[target] = true
					if _, ok := recorded[directory][target]; ok {
						used[directory+" "+target] = true
						continue
					}
					findings = append(findings, finding{pkg: directory, file: file, path: target})
				}
			}
		}
	}
	var stale []string
	for directory, targets := range recorded {
		for target := range targets {
			if !used[directory+" "+target] {
				stale = append(stale, directory+" "+target)
			}
		}
	}
	slices.Sort(stale)
	return findings, stale, nil
}

func run(root string, stdout, stderr io.Writer) int {
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(stderr, "test-read-scopes-check:", err)
		return 2
	}
	findings, stale, err := check(root)
	if err != nil {
		fmt.Fprintln(stderr, "test-read-scopes-check:", err)
		return 2
	}
	// A stale entry only reports: failing on it would break a merge of one change
	// that drops a literal with another that still records it.
	for _, entry := range stale {
		fmt.Fprintf(stderr, "test-read-scopes-check: %s records %s, which no test file of the package names; remove it\n", literalsPath, entry)
	}
	for _, found := range findings {
		fmt.Fprintf(stderr, "test-read-scopes-check: package %s: %s names %s, outside its entries in %s; declare it there (AFP-V0-023), keep the read inside the package, or record in %s why it is no read\n",
			found.pkg, found.file, found.path, affected.ReadScopesPath, literalsPath)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "test-read-scopes-check: ok")
	return 0
}

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: test-read-scopes-check [--root DIR]")
		os.Exit(2)
	}
	os.Exit(run(*root, os.Stdout, os.Stderr))
}
