// SPDX-License-Identifier: Apache-2.0
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
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
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
	sandboxesPath      = ".corvint/test-nested-sandboxes.json"
	sandboxesProfile   = "corvint-test-nested-sandboxes/0"
	unboundedPath      = ".corvint/unbounded-readers.json"
)

// readSandboxes reads the closed {profile, packages} record of the package
// directories whose tests launch a sandbox of their own (AFP-V0-615), mapped to
// the sandbox and why it cannot nest. The confined CI shard runs a declared
// package under the AFP-V0-023 Landlock wrapper, which refuses that nested
// sandbox (bubblewrap under internal/liveverify/mutate, FPK-V0-142, #1937), so
// such a package is never declared. An absent record is empty.
func readSandboxes(root string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sandboxesPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var record struct {
		Profile  string            `json:"profile"`
		Packages map[string]string `json:"packages"`
	}
	// json/v2 matches member names exactly and refuses a duplicate member or trailing data.
	bad := len(raw) > maxLiterals || json.Unmarshal(raw, &record, json.RejectUnknownMembers(true)) != nil ||
		record.Profile != sandboxesProfile || record.Packages == nil
	for directory, reason := range record.Packages {
		bad = bad || directory == "" || path.Clean(directory) != directory || path.IsAbs(directory) ||
			directory == "." || directory == ".." || strings.HasPrefix(directory, "../") || strings.TrimSpace(reason) == ""
	}
	if bad {
		return nil, fmt.Errorf("%s: want exactly {\"profile\":%q,\"packages\":{PACKAGE:REASON}} with a clean relative package directory and a non-empty reason for each", sandboxesPath, sandboxesProfile)
	}
	return record.Packages, nil
}

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
	// listed marks a dependency directory a `go list -deps` of the test loads
	// (V1-2037), rather than a literal the test names.
	listed bool
	// sandbox is the reason a declared package's tests launch a nested
	// sandbox (AFP-V0-615); file and path are then empty.
	sandbox string
	// rootedBy names the test-file literal that makes the package's tokens
	// root-relative, when path is reached only from the repository root
	// (V1-3234); it is empty when the package directory resolves it.
	rootedBy string
}

// rootLocator names the first test-file literal of a package that locates the
// repository root, as file:line and the quoted literal (V1-3234). A token the
// AFP-V0-021 lexicon read outside a string literal is named by the line of its
// first occurrence.
func rootLocator(files []string, bodies [][]byte, tokens [][]string) string {
	scan := tempRootScan{tokens: map[string][]string{}}
	for index, name := range files {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, name, bodies[index], parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		found := ""
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING || found != "" {
				return found == ""
			}
			values, err := scan.literalTokens(literal.Value)
			if err != nil {
				return true
			}
			if locatesRoot([][]string{values}) {
				found = fmt.Sprintf("%s:%d holds %s", name, fileSet.Position(literal.Pos()).Line, literal.Value)
			}
			return found == ""
		})
		if found != "" {
			return found
		}
	}
	for index, name := range files {
		for _, value := range tokens[index] {
			if locatesRoot([][]string{{value}}) {
				line := 1 + bytes.Count(bodies[index][:max(0, bytes.Index(bodies[index], []byte(value)))], []byte("\n"))
				return fmt.Sprintf("%s:%d holds %q", name, line, value)
			}
		}
	}
	return ""
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

// joinRuns returns, for every filepath.Join or path.Join call of a test file,
// each run of two or more adjacent string-literal arguments joined with "/"
// (V1-2117). filepath.Join(filepath.Dir(file), "..", "..", "protocol", "x")
// yields "../../protocol/x", which named resolves against the package directory
// (never from the root); a single literal is already a FilePathTokens token.
func joinRuns(name string, body []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, body, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var runs []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Join" {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "filepath" && pkg.Name != "path" {
			return true
		}
		var run []string
		flush := func() {
			if len(run) > 1 {
				runs = append(runs, strings.Join(run, "/"))
			}
			run = nil
		}
		for _, argument := range call.Args {
			literal, ok := argument.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				flush()
				continue
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				flush()
				continue
			}
			run = append(run, value)
		}
		flush()
		return true
	})
	return runs, nil
}

// goListTargets returns the package patterns of every `go list` command a test
// file builds with exec.Command or exec.CommandContext (V1-2037): the
// string-literal arguments after "list" that are "." or start with "./" or
// "../". A listing with no such argument lists the package itself.
func goListTargets(name string, body []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, body, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var targets []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext" {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "exec" {
			return true
		}
		listing, found := false, false
		for _, argument := range call.Args {
			literal, ok := argument.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				continue
			}
			switch {
			case !listing:
				listing = value == "list"
			case value == "." || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../"):
				targets, found = append(targets, value), true
			}
		}
		if listing && !found {
			targets = append(targets, ".")
		}
		return true
	})
	return targets, nil
}

// dependencyDirectories lists, relative to root, the directory of every
// non-standard package that `go list -deps` of targets loads from inside root.
// Those directories are what a confined `go list` must read. A target resolves
// against the package directory, where `go test` runs, or, when no directory
// is there, against the root, where a test that sets the command's Dir to the
// root runs it; a target that is neither is an error, not a silent pass.
func dependencyDirectories(root, directory string, targets []string) ([]string, error) {
	var resolved []string
	for _, target := range targets {
		var found string
		for _, base := range []string{directory, "."} {
			candidate := path.Join(base, target)
			if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil && info.IsDir() {
				found = candidate
				break
			}
		}
		if found == "" || found == ".." || strings.HasPrefix(found, "../") {
			return nil, fmt.Errorf("%s: a test runs go list of %s, which names no repository directory", directory, target)
		}
		resolved = append(resolved, "./"+found)
	}
	command := exec.Command("go", append([]string{"list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}"}, resolved...)...)
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: go list -deps %s: %w: %s", directory, strings.Join(resolved, " "), err, strings.TrimSpace(stderr.String()))
	}
	// One directory per line: a directory may contain spaces, so splitting at
	// whitespace would drop its fragments as outside the root.
	var directories []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		relative, err := filepath.Rel(root, line)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		directories = append(directories, filepath.ToSlash(relative))
	}
	return directories, nil
}

// dependencyCovered reports whether a confined `go list` run by a test of
// directory may read the source files of dependency: it is the package
// directory or below it, or lies under a declared directory entry. Unlike
// covered, a directory above an entry is not enough: go list reads the
// directory's own files.
func dependencyCovered(directory, dependency string, entries []string) bool {
	if dependency == directory || strings.HasPrefix(dependency, directory+"/") {
		return true
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry, "/") && strings.HasPrefix(dependency+"/", entry) {
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
	sandboxes, err := readSandboxes(root)
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
		if reason, ok := sandboxes[directory]; ok {
			findings = append(findings, finding{pkg: directory, sandbox: reason})
			continue
		}
		tokens, runs := make([][]string, len(unit.Tests)), make([][]string, len(unit.Tests))
		bodies := make([][]byte, len(unit.Tests))
		for index, file := range unit.Tests {
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				return nil, nil, err
			}
			bodies[index] = body
			if tokens[index], err = golang.FilePathTokens(file, body, ""); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", file, err)
			}
			if runs[index], err = joinRuns(file, body); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", file, err)
			}
			targets, err := goListTargets(file, body)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", file, err)
			}
			if len(targets) > 0 {
				dependencies, err := dependencyDirectories(root, directory, targets)
				if err != nil {
					return nil, nil, err
				}
				for _, dependency := range dependencies {
					if !dependencyCovered(directory, dependency, unit.ReadScope) {
						findings = append(findings, finding{pkg: directory, file: file, path: dependency, listed: true})
					}
				}
			}
		}
		fromRoot := locatesRoot(tokens)
		rootedBy := ""
		if fromRoot {
			rootedBy = rootLocator(unit.Tests, bodies, tokens)
		}
		exempt, err := tempRootOnly(unit.Tests, bodies)
		if err != nil {
			return nil, nil, err
		}
		for index, file := range unit.Tests {
			seen := map[string]bool{}
			values := append(slices.Clone(tokens[index]), runs[index]...)
			for position, value := range values {
				// A join run resolves against the package directory only: fixture
				// repositories join the same segments onto temporary roots, so a
				// run is no evidence of a repository-root read (V1-2117). Nor is a
				// token used only by temp-root-only functions (AFP-V0-828).
				rooted := fromRoot && position < len(tokens[index]) && !exempt[index][value]
				for _, target := range named(paths, directory, value, rooted) {
					if seen[target] || covered(directory, target, unit.ReadScope) {
						continue
					}
					seen[target] = true
					if _, ok := recorded[directory][target]; ok {
						used[directory+" "+target] = true
						continue
					}
					found := finding{pkg: directory, file: file, path: target}
					if target != path.Join(directory, value) {
						found.rootedBy = rootedBy
					}
					findings = append(findings, found)
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
		if found.sandbox != "" {
			fmt.Fprintf(stderr, "test-read-scopes-check: package %s is declared in %s, but %s records that its tests launch a nested sandbox (%s), which the confined CI shard refuses; remove its entry and add it to units of %s with a reason (AFP-V0-615)\n",
				found.pkg, affected.ReadScopesPath, sandboxesPath, found.sandbox, unboundedPath)
			continue
		}
		if found.listed {
			fmt.Fprintf(stderr, "test-read-scopes-check: package %s: %s runs go list -deps, which loads %s, outside its entries in %s; declare the directory there (AFP-V0-023)\n",
				found.pkg, found.file, found.path, affected.ReadScopesPath)
			continue
		}
		rooted := ""
		if found.rootedBy != "" {
			// V1-3234: name the literal and the rule that resolve the token from the root.
			rooted = fmt.Sprintf("; it is read from the repository root because %s, which locates the root (AFP-V0-021: such a literal in any test file makes the package's tokens root-relative; only a token whose every literal lies in a temp-root-only test function is exempt, AFP-V0-828)", found.rootedBy)
		}
		fmt.Fprintf(stderr, "test-read-scopes-check: package %s: %s names %s, outside its entries in %s; declare it there (AFP-V0-023), keep the read inside the package, or record in %s why it is no read%s\n",
			found.pkg, found.file, found.path, affected.ReadScopesPath, literalsPath, rooted)
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
