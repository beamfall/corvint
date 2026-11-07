// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"
)

// sourceObservingImports are standard packages whose import alone lets a
// file observe package metadata or source text beyond the import graph: the
// go/ parsers and importers, build information, embedded files, process and
// system calls, and executable readers (AFP-V0-9010).
var sourceObservingImports = map[string]bool{
	"go/build": true, "go/doc": true, "go/importer": true, "go/parser": true, "go/scanner": true,
	"go/packages": true, "runtime/debug": true, "debug/buildinfo": true, "debug/dwarf": true,
	"debug/elf": true, "debug/gosym": true, "debug/macho": true, "debug/pe": true,
	"debug/plan9obj": true, "embed": true, "plugin": true, "syscall": true,
}

// fileReaders are, by standard package, the members that read a file or a
// directory, start a process or serve files. A call whose first argument is
// a literal path inside the package's own testdata directory is allowed.
var fileReaders = map[string]map[string]bool{
	"os": {
		"ReadFile": true, "Open": true, "OpenFile": true, "ReadDir": true, "DirFS": true, "OpenRoot": true,
		"OpenInRoot": true, "Stat": true, "Lstat": true, "Readlink": true, "StartProcess": true,
	},
	"io/ioutil":     {"ReadFile": true, "ReadDir": true},
	"path/filepath": {"Walk": true, "WalkDir": true, "Glob": true, "EvalSymlinks": true},
	"net/http": {
		"Dir": true, "FileServer": true, "FileServerFS": true, "ServeFile": true, "ServeFileFS": true,
		"NewFileTransport": true, "NewFileTransportFS": true,
	},
	"os/exec":       {"Cmd": true},
	"text/template": {"ParseFiles": true, "ParseGlob": true},
	"html/template": {"ParseFiles": true, "ParseGlob": true},
}

// testdataReaders are the fileReaders members a literal own-testdata path
// keeps proven.
var testdataReaders = map[string]bool{"ReadFile": true, "Open": true, "ReadDir": true, "Stat": true, "Lstat": true}

// observesSource reports a file of an import-graph reader that may observe
// package metadata or source text by a channel other than its proven
// `go list` calls (AFP-V0-9010), so its unit is not proven to observe the
// import graph alone and is selected by any dirty .go path. Unknown is
// observing: a file that does not parse; an import outside the standard
// library, whose code may read files; a source-observing import; a dot or
// shadowed import of a file-reading package; a reference to a file reader
// other than a call that reads a literal path below the package's own
// testdata directory; a `//go:embed` line; a ParseFiles or ParseGlob method;
// and an os/exec call that is not a `go list` call.
func observesSource(relative string, body []byte) bool {
	file, err := parser.ParseFile(token.NewFileSet(), relative, body, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return true
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:embed") {
				return true
			}
		}
	}
	names := unshadowedImports(file)
	readers := map[string]map[string]bool{}
	exec := ""
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return true
		}
		if first, _, _ := strings.Cut(value, "/"); strings.Contains(first, ".") || sourceObservingImports[value] {
			return true
		}
		members, reads := fileReaders[value]
		if !reads {
			continue
		}
		local := path.Base(value)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if local == "_" {
			continue
		}
		if local == "." || names[local] != value {
			return true
		}
		readers[local] = members
		if value == "os/exec" {
			exec = local
		}
	}
	var functions []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if node.Body != nil {
				functions = append(functions, node.Body)
			}
		case *ast.FuncLit:
			functions = append(functions, node.Body)
		}
		return true
	})
	allowed := map[*ast.SelectorExpr]bool{}
	observes := false
	ast.Inspect(file, func(node ast.Node) bool {
		if observes {
			return false
		}
		switch node := node.(type) {
		case *ast.CallExpr:
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch {
			case qualifier.Name == exec && (selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext"):
				if !goListCall(node, selector.Sel.Name, functions) {
					observes = true
				}
			case readers[qualifier.Name] != nil && testdataReaders[selector.Sel.Name] && len(node.Args) != 0 && ownTestdata(node.Args[0], names):
				allowed[selector] = true
			}
		case *ast.SelectorExpr:
			if node.Sel.Name == "ParseFiles" || node.Sel.Name == "ParseGlob" {
				observes = true
				return false
			}
			qualifier, ok := node.X.(*ast.Ident)
			if ok && readers[qualifier.Name][node.Sel.Name] && !allowed[node] {
				observes = true
			}
		}
		return true
	})
	return observes
}

// goListCall reports an os/exec Command or CommandContext call whose command
// is the literal "go" and whose first argument is the literal "list", given
// directly or as the first element of a local []string variable that
// sliceElements proves; goListScanFile then classifies the call itself.
func goListCall(call *ast.CallExpr, function string, functions []ast.Node) bool {
	arguments := call.Args
	if function == "CommandContext" {
		if len(arguments) == 0 {
			return false
		}
		arguments = arguments[1:]
	}
	if len(arguments) < 2 || !literalIs(arguments[0], "go") {
		return false
	}
	elements := arguments[1:]
	if call.Ellipsis.IsValid() {
		ident, ok := elements[0].(*ast.Ident)
		if len(elements) != 1 || !ok {
			return false
		}
		proven := false
		for _, body := range enclosingBodies(functions, call) {
			if elements, proven = sliceElements(body, ident.Name, call); proven {
				break
			}
		}
		if !proven {
			return false
		}
	}
	return len(elements) != 0 && literalIs(elements[0], "list")
}

// ownTestdata reports a path argument that is a literal, or a path/filepath
// Join of literals, naming the package's own testdata directory or a path
// below it: a test runs in its package directory.
func ownTestdata(argument ast.Expr, names map[string]string) bool {
	var parts []string
	if value, ok := literalString(argument); ok {
		parts = []string{value}
	} else {
		call, ok := argument.(*ast.CallExpr)
		if !ok || call.Ellipsis.IsValid() {
			return false
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Join" {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || names[qualifier.Name] != "path/filepath" && names[qualifier.Name] != "path" {
			return false
		}
		for _, element := range call.Args {
			value, ok := literalString(element)
			if !ok {
				return false
			}
			parts = append(parts, value)
		}
	}
	joined := path.Clean(strings.ReplaceAll(path.Join(parts...), `\`, "/"))
	return joined == "testdata" || strings.HasPrefix(joined, "testdata/")
}
