// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// testScopePrefix marks a scope entry `@test:DIR` (GL-V0-775): the step is a
// `go test -run PATTERN ./DIR` whose inputs are the files `go list -deps -test`
// compiles or embeds into DIR's test binary, plus every path the package's
// .corvint/test-read-scopes.json declaration admits (its subtree, its declared
// entries and the declaration itself), plus the gate tooling.
const testScopePrefix = "@test:"

// depsScopePrefix marks `@deps:DIR` (GL-V0-787): DIR's `go list -deps -test`
// files only, for a step whose reads the scope's other entries declare, so no
// read-scope declaration for DIR is needed. `@all` selects every worktree path,
// `!PATTERN` removes the paths it matches from `@all`, and a plain entry (a
// `*.ext` suffix, a `DIR/` prefix, a one-component `DIR/*.ext` glob or an exact
// path) adds paths back.
const (
	depsScopePrefix = "@deps:"
	allScope        = "@all"
)

const (
	readScopesFile    = ".corvint/test-read-scopes.json"
	readScopesProfile = "corvint-test-read-scopes/0"
)

// testScopes are the batch-precheck go-test doc gates keyed on their package's
// test closure (GL-V0-776, V1-4160). Each target in Makefile is exactly one
// `go test -run PATTERN ./DIR` of a package the read-scope declaration bounds,
// so a seal that adds only a CEM, or a lane that edits an unrelated package,
// hits the pass instead of rerunning it on the whole-tree key.
var testScopes = map[string][]string{
	"shell-test-registration-check": {testScopePrefix + "tools/test-commands-check"},
	"go-version-pins-check":         {testScopePrefix + "tools/capabilities-check"},
	// GL-V0-788 (V1-4193): TestRepositoryShellForeignCallersComplete_TSP_V0_005 reads the
	// tracked listing, every source-walk file except .md, .json, .jsonl and .tsv ones (the
	// walk skips hidden directories), the shell plugin's .sh and foreign-source files under
	// it and .github, and the .corvint/ declarations; a sealed CEM or a build log is none.
	"shell-foreign-callers-check": {depsScopePrefix + "internal/liveverify/affected", allScope, "!*.md", "!*.json", "!*.jsonl", "!*.tsv", ".corvint/*.json"},
}

func init() {
	for name, scope := range testScopes {
		scopes[name] = scope
	}
}

// scopedInputs is inputs, except that a scope with `@test:DIR` entries digests
// each package's test closure (GL-V0-775). A closure that cannot be proven (no
// declaration for DIR, an unreadable declaration, a failing `go list`, a listed
// file the worktree digest does not hold) keys the step on the whole tree and
// says so; it never narrows on a doubt.
func (l *ledger) scopedInputs(scope []string) string {
	var dirs, deps, rest, excluded []string
	all := false
	for _, s := range scope {
		if dir, ok := strings.CutPrefix(s, testScopePrefix); ok {
			dirs = append(dirs, dir)
		} else if dir, ok := strings.CutPrefix(s, depsScopePrefix); ok {
			deps = append(deps, dir)
		} else if s == allScope {
			all = true
		} else if pattern, ok := strings.CutPrefix(s, "!"); ok {
			excluded = append(excluded, pattern)
		} else {
			rest = append(rest, s)
		}
	}
	if len(dirs)+len(deps) == 0 && !all && len(excluded) == 0 {
		return l.inputs(scope)
	}
	selected := map[string]bool{}
	for _, part := range []struct {
		dirs     []string
		declared bool
	}{{dirs, true}, {deps, false}} {
		if len(part.dirs) == 0 {
			continue
		}
		closure, err := l.testClosure(part.dirs, part.declared)
		if err != nil {
			fmt.Fprintf(l.stdout, "%sSCOPE %s unavailable: %s; keyed on the whole tree\n", prefixOut, strings.Join(scope, " "), err)
			return l.inputs([]string{treeScope})
		}
		for p := range closure {
			selected[p] = true
		}
	}
	h := sha256.New()
	for _, entry := range l.entries {
		if selected[entry.path] || matchesScope(entry.path, rest) || (all && !matchesScope(entry.path, excluded)) || inScope(entry.path, tooling) {
			fmt.Fprintf(h, "%s\n", entry.line)
		}
	}
	return "test-closure " + hex.EncodeToString(h.Sum(nil))
}

// matchesScope is inScope plus one-component `DIR/*.ext` globs.
func matchesScope(p string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(pattern, "*") && !strings.HasPrefix(pattern, "*.") {
			if ok, err := path.Match(pattern, p); err == nil && ok {
				return true
			}
		} else if inScope(p, []string{pattern}) {
			return true
		}
	}
	return false
}

// testClosure is the set of worktree paths the `@test:` packages in dirs read,
// or, without declared, only their `go list -deps -test` files (`@deps:`).
func (l *ledger) testClosure(dirs []string, declared bool) (map[string]bool, error) {
	if l.root == "" {
		return nil, errors.New("no repository root")
	}
	module, err := modulePath(filepath.Join(l.root, "go.mod"))
	if err != nil {
		return nil, err
	}
	readScopes := map[string][]string{}
	if declared {
		if readScopes, err = readTestScopes(filepath.Join(l.root, filepath.FromSlash(readScopesFile))); err != nil {
			return nil, err
		}
	}
	patterns := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if _, ok := readScopes[dir]; declared && !ok {
			return nil, fmt.Errorf("%s declares no read scope for %s", readScopesFile, dir)
		}
		patterns = append(patterns, "./"+dir)
	}
	listed, err := l.listedFilesFor(module, patterns...)
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, entry := range l.entries {
		present[entry.path] = true
	}
	selected := map[string]bool{}
	for _, dir := range dirs {
		files, ok := listed[module+"/"+dir]
		if !ok {
			return nil, fmt.Errorf("go list does not list %s", dir)
		}
		for _, f := range files {
			if !present[f] {
				return nil, fmt.Errorf("go list names %s, which the worktree digest does not hold", f)
			}
			selected[f] = true
		}
		if !declared {
			continue
		}
		for p := range present {
			if inTestReadScope(dir, readScopes[dir], p) {
				selected[p] = true
			}
		}
	}
	return selected, nil
}

// readTestScopes decodes the read-scope declaration exactly: the profile, no
// unknown members, and canonical entries (internal/gateselect parseReadScopes).
func readTestScopes(name string) (map[string][]string, error) {
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var file struct {
		Profile  string              `json:"profile"`
		Packages map[string][]string `json:"packages"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%s: %w", readScopesFile, err)
	}
	if file.Profile != readScopesProfile || file.Packages == nil {
		return nil, fmt.Errorf("%s: profile must be %s", readScopesFile, readScopesProfile)
	}
	for dir, entries := range file.Packages {
		if !canonicalPath(dir) || entries == nil {
			return nil, fmt.Errorf("%s: package %q", readScopesFile, dir)
		}
		for _, entry := range entries {
			trimmed := strings.TrimSuffix(entry, "/")
			if !canonicalPath(trimmed) || trimmed == ".git" || strings.HasPrefix(trimmed, ".git/") {
				return nil, fmt.Errorf("%s: package %q entry %q", readScopesFile, dir, entry)
			}
		}
	}
	return file.Packages, nil
}

// inTestReadScope mirrors internal/gateselect inReadScope: the declaration
// itself, a path in the package's subtree, a declared entry, a path in a
// declared subtree, or an ancestor of an entry.
func inTestReadScope(dir string, entries []string, p string) bool {
	if p == readScopesFile || p == dir || strings.HasPrefix(p, dir+"/") {
		return true
	}
	for _, entry := range entries {
		target := strings.TrimSuffix(entry, "/")
		if p == target || strings.HasPrefix(target, p+"/") || (target != entry && strings.HasPrefix(p, entry)) {
			return true
		}
	}
	return false
}

// canonicalPath is a relative slash path with no empty, "." or ".." component.
func canonicalPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\\x00") {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}
