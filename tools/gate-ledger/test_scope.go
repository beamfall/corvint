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
	"path/filepath"
	"strings"
)

// testScopePrefix marks a scope entry `@test:DIR` (GL-V0-775): the step is a
// `go test -run PATTERN ./DIR` whose inputs are the files `go list -deps -test`
// compiles or embeds into DIR's test binary, plus every path the package's
// .corvint/test-read-scopes.json declaration admits (its subtree, its declared
// entries and the declaration itself), plus the gate tooling.
const testScopePrefix = "@test:"

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
	var dirs, rest []string
	for _, s := range scope {
		if dir, ok := strings.CutPrefix(s, testScopePrefix); ok {
			dirs = append(dirs, dir)
		} else {
			rest = append(rest, s)
		}
	}
	if len(dirs) == 0 {
		return l.inputs(scope)
	}
	selected, err := l.testClosure(dirs)
	if err != nil {
		fmt.Fprintf(l.stdout, "%sSCOPE %s unavailable: %s; keyed on the whole tree\n", prefixOut, strings.Join(scope, " "), err)
		return l.inputs([]string{treeScope})
	}
	h := sha256.New()
	for _, entry := range l.entries {
		if selected[entry.path] || inScope(entry.path, rest) || inScope(entry.path, tooling) {
			fmt.Fprintf(h, "%s\n", entry.line)
		}
	}
	return "test-closure " + hex.EncodeToString(h.Sum(nil))
}

// testClosure is the set of worktree paths the `@test:` packages in dirs read.
func (l *ledger) testClosure(dirs []string) (map[string]bool, error) {
	if l.root == "" {
		return nil, errors.New("no repository root")
	}
	module, err := modulePath(filepath.Join(l.root, "go.mod"))
	if err != nil {
		return nil, err
	}
	declared, err := readTestScopes(filepath.Join(l.root, filepath.FromSlash(readScopesFile)))
	if err != nil {
		return nil, err
	}
	patterns := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if _, ok := declared[dir]; !ok {
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
		for p := range present {
			if inTestReadScope(dir, declared[dir], p) {
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
