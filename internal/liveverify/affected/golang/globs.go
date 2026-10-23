package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// globName is the selector a literal glob reader calls (AFP-V0-401): the
// path/filepath package's Glob. The prefilter looks for the name alone, since
// a space, a line break or a comment may separate it from its parenthesis.
var globName = []byte("Glob")

// literalGlobs returns the patterns of the filepath.Glob calls in one Go file
// whose argument is literal (AFP-V0-401), in the affected.Unit.PathGlobs form:
// a pattern made only of string literals resolves against the file's package
// directory, where its tests run, and is anchored at the repository root with a
// leading "/"; a filepath.Join whose first argument is any other expression (a
// root located at run time) and whose remaining arguments are all string
// literals keeps the joined literals unanchored when they name a directory
// (two or more components): one component after such a root (`"*.json"` in a
// temporary directory) places no path and would match nearly every file name.
// Any other argument is not recorded, so a path only it could match stays
// unknown as before, and so is a literal that is not valid UTF-8, which no
// repository path can be. A file that does not parse records nothing; the
// imports-only parse already raised its frontier.
func literalGlobs(directory string, body []byte, imports []*ast.ImportSpec) []string {
	if !bytes.Contains(body, globName) {
		return nil
	}
	name := ""
	for _, spec := range imports {
		if value, err := strconv.Unquote(spec.Path.Value); err != nil || value != "path/filepath" {
			continue
		}
		name = "filepath"
		if spec.Name != nil {
			name = spec.Name.Name
		}
	}
	if name == "" || name == "_" || name == "." {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), "", body, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var globs []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !qualified(call.Fun, name, "Glob") || len(call.Args) != 1 {
			return true
		}
		if glob, ok := globPattern(directory, call.Args[0], name); ok {
			globs = append(globs, glob)
		}
		return true
	})
	return globs
}

// globPattern evaluates one Glob argument: a string literal or a
// filepath.Join of string literals after at most one leading non-literal.
func globPattern(directory string, argument ast.Expr, name string) (string, bool) {
	arguments := []ast.Expr{argument}
	if call, ok := argument.(*ast.CallExpr); ok && qualified(call.Fun, name, "Join") && call.Ellipsis == token.NoPos {
		arguments = call.Args
	}
	anchored := true
	literals := make([]string, 0, len(arguments))
	for index, expression := range arguments {
		value, ok := stringLiteral(expression)
		if !ok && index == 0 && len(arguments) > 1 {
			anchored = false
			continue
		}
		if !ok || strings.Contains(value, "\\") || !utf8.ValidString(value) {
			return "", false
		}
		literals = append(literals, value)
	}
	pattern := path.Join(literals...)
	if pattern == "" || pattern == "." || strings.HasPrefix(pattern, "/") {
		return "", false
	}
	if anchored {
		pattern = path.Join(directory, pattern)
		if pattern == "." || pattern == ".." || strings.HasPrefix(pattern, "../") {
			return "", false
		}
		pattern = "/" + pattern
	} else if pattern == ".." || strings.HasPrefix(pattern, "../") || !strings.Contains(pattern, "/") {
		return "", false
	}
	if _, err := path.Match(pattern, ""); err != nil {
		return "", false
	}
	return pattern, true
}

func qualified(expression ast.Expr, name, selector string) bool {
	call, ok := expression.(*ast.SelectorExpr)
	if !ok || call.Sel.Name != selector {
		return false
	}
	qualifier, ok := call.X.(*ast.Ident)
	return ok && qualifier.Name == name
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}
