package golang

import (
	"fmt"
	"go/ast"
	"path"
	"strconv"
	"strings"
)

// This file mirrors AFP-V0-012 rule (d) of tools/gate-affected-select
// (rootLocatorImports, scanSource, escapesPackage, markUnresolved): a package
// whose reads no literal bounds. The gate tool is a standard-library-only
// package main the trusted PR driver builds, so the two cannot share code.

// AFP-V0-810 (V1-2758): runtime frame APIs return metadata, not file
// contents. They never witness an unbounded read, regardless of the package's
// imports; the gate tool excludes them the same way.

// rootLocatorFuncs names calls that locate the working directory at run time.
var rootLocatorFuncs = map[string]map[string]bool{
	"os": {"Getwd": true},
}

// rootLocatorToken is the Git option with the same effect, split so that this
// package's own source does not carry it.
var rootLocatorToken = "--show-" + "toplevel"

// unboundedReads accumulates one package's rule (d) state: the first reason,
// replaced by the first non-test one, and whether a non-test file locates the
// root, which makes every dependent unbounded too.
type unboundedReads struct {
	reason      string
	locatesRoot bool
}

func (reads *unboundedReads) mark(reason string, test bool) {
	if reads.reason == "" || (!test && !reads.locatesRoot) {
		reads.reason = reason
	}
	reads.locatesRoot = reads.locatesRoot || !test
}

// rootLocatorImports maps a file's local qualifier (an import's default name
// or its alias) to each root-locating import path, and marks the ones pulled
// in with a dot import, whose exported names appear unqualified.
func rootLocatorImports(specs []*ast.ImportSpec) (aliases map[string]string, dotImported map[string]bool) {
	return qualifiedImports(specs, func(importPath string) bool {
		_, locator := rootLocatorFuncs[importPath]
		return locator
	})
}

// qualifiedImports maps each local qualifier of an import that wanted accepts
// (its default name, the last import path element, or its alias) to the
// import path, and marks the ones pulled in with a dot import.
func qualifiedImports(specs []*ast.ImportSpec, wanted func(string) bool) (aliases map[string]string, dotImported map[string]bool) {
	aliases = map[string]string{}
	dotImported = map[string]bool{}
	for _, spec := range specs {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil || !wanted(value) {
			continue
		}
		switch {
		case spec.Name == nil:
			aliases[path.Base(value)] = value
		case spec.Name.Name == ".":
			dotImported[value] = true
		case spec.Name.Name != "_":
			aliases[spec.Name.Name] = value
		}
	}
	return aliases, dotImported
}

// rootLocatorCall names the root-locating call an identifier completes, given
// the two tokens before it: `qualifier . Getwd` or a dot-imported `Getwd`.
func rootLocatorCall(previous [2]string, ident string, aliases map[string]string, dotImported map[string]bool) []string {
	if previous[1] == "." {
		importPath, ok := aliases[strings.TrimPrefix(previous[0], "IDENT")]
		if ok && strings.HasPrefix(previous[0], "IDENT") && rootLocatorFuncs[importPath][ident] {
			return []string{importPath + "." + ident}
		}
		return nil
	}
	var calls []string
	for importPath := range dotImported {
		if rootLocatorFuncs[importPath][ident] {
			calls = append(calls, importPath+"."+ident+" (dot import)")
		}
	}
	return calls
}

// escapesPackage names why a token read from directory's files cannot be
// bounded by its components. Any file: it names the root lookup. A test file
// (run with its package directory as the working directory): it is a package
// pattern, it is made only of `..` components, or it resolves to the
// repository root or above. A non-test file: it climbs with `..` to a named
// component, which resolves against whichever package's test runs it.
func escapesPackage(directory, value string, test bool) string {
	if value == rootLocatorToken || (test && strings.HasSuffix(value, "/...")) {
		return fmt.Sprintf("carries the literal %q", value)
	}
	climbs, parents := false, true
	for _, component := range strings.Split(value, "/") {
		climbs = climbs || component == ".."
		parents = parents && (component == ".." || component == "." || component == "")
	}
	if strings.HasPrefix(value, "/") || !climbs || (parents && !test) {
		return ""
	}
	if !test {
		return fmt.Sprintf("carries the literal %q, which climbs from the working directory", value)
	}
	resolved := path.Join(directory, value)
	if parents || resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return fmt.Sprintf("carries the literal %q, which reaches the repository root", value)
	}
	return ""
}
