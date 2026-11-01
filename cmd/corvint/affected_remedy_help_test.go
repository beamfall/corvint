package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// affectedRemedy matches a remediation phrase that ends where a declaration
// file is named: "declare it in ", "record the path in ", and so on.
var affectedRemedy = regexp.MustCompile(`(?i)\b(declare|declaring|record|list|add)\b[^.;:]{0,60}\bin $`)

// affectedInlineRemedy matches a remediation that names the file in the same
// literal: "declare it in .corvint/x.json".
var affectedInlineRemedy = regexp.MustCompile(`(?i)\b(?:declare|declaring|record|list|add)\b[^.;:]{0,60}\bin (\.corvint/[a-z0-9-]+\.json)`)

// TestAffectedRemedyDeclarationsAreDocumented_AFPV0874 pins V1-3934: every
// .corvint declaration file that an affected planner remediation message tells
// the reader to declare a path in is documented in `corvint help affected`
// (AFP-V0-874), so no plan names a file the help never explains.
func TestAffectedRemedyDeclarationsAreDocumented_AFPV0874(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	named := map[string][]string{}
	for _, dir := range []string{"internal/liveverify/affected", "cmd/corvint"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			if dir == "cmd/corvint" && !strings.HasPrefix(entry.Name(), "affected") {
				return nil
			}
			for _, file := range affectedRemedyFiles(t, path) {
				named[file] = append(named[file], filepath.ToSlash(path))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(named) == 0 || named[".corvint/playwright-unreached-paths.json"] == nil {
		t.Fatalf("found no remediation naming .corvint/playwright-unreached-paths.json: %v", named)
	}
	var help, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"help", "affected"}, strings.NewReader(""), &help, &stderr); code != 0 {
		t.Fatalf("help affected exit %d: %s", code, stderr.String())
	}
	files := make([]string, 0, len(named))
	for file := range named {
		files = append(files, file)
	}
	slices.Sort(files)
	for _, file := range files {
		if !strings.Contains(help.String(), file) {
			t.Errorf("%s names %s in a remediation, but `corvint help affected` never documents it", named[file][0], file)
		}
	}
}

// affectedRemedyFiles returns the .corvint/*.json files one Go source names
// right after a remediation phrase, resolving package string constants.
func affectedRemedyFiles(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	constants := affectedPackageConstants(t, filepath.Dir(path))
	value := func(expr ast.Expr) (string, bool) {
		switch e := expr.(type) {
		case *ast.BasicLit:
			if e.Kind == token.STRING {
				s, err := strconv.Unquote(e.Value)
				return s, err == nil
			}
		case *ast.Ident:
			s, ok := constants[e.Name]
			return s, ok
		case *ast.SelectorExpr:
			s, ok := constants[e.Sel.Name]
			return s, ok
		}
		return "", false
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.BasicLit); ok {
			if text, ok := value(literal); ok {
				for _, match := range affectedInlineRemedy.FindAllStringSubmatch(text, -1) {
					found = append(found, match[1])
				}
			}
			return true
		}
		binary, ok := node.(*ast.BinaryExpr)
		if !ok || binary.Op != token.ADD {
			return true
		}
		var operands []ast.Expr
		var flatten func(ast.Expr)
		flatten = func(expr ast.Expr) {
			if b, ok := expr.(*ast.BinaryExpr); ok && b.Op == token.ADD {
				flatten(b.X)
				flatten(b.Y)
				return
			}
			operands = append(operands, expr)
		}
		flatten(binary)
		for _, operand := range operands {
			ast.Inspect(operand, func(inner ast.Node) bool {
				if literal, ok := inner.(*ast.BasicLit); ok {
					if text, ok := value(literal); ok {
						for _, match := range affectedInlineRemedy.FindAllStringSubmatch(text, -1) {
							found = append(found, match[1])
						}
					}
				}
				return true
			})
		}
		for index := 0; index+1 < len(operands); index++ {
			text, ok := value(operands[index])
			if !ok || !affectedRemedy.MatchString(text) {
				continue
			}
			if next, ok := value(operands[index+1]); ok && strings.HasPrefix(next, ".corvint/") {
				found = append(found, strings.Fields(next)[0])
			}
		}
		return false
	})
	return found
}

// affectedPackageConstants maps the string constants of the non-test files in
// dir, and of the affected package every adapter imports, by name.
func affectedPackageConstants(t *testing.T, dir string) map[string]string {
	t.Helper()
	constants := map[string]string{}
	for _, directory := range []string{dir, filepath.Join("..", "..", "internal", "liveverify", "affected")} {
		fset := token.NewFileSet()
		packages, err := parser.ParseDir(fset, directory, func(info fs.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range packages {
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					general, ok := decl.(*ast.GenDecl)
					if !ok || general.Tok != token.CONST {
						continue
					}
					for _, spec := range general.Specs {
						values := spec.(*ast.ValueSpec)
						for index, name := range values.Names {
							if index < len(values.Values) {
								if literal, ok := values.Values[index].(*ast.BasicLit); ok && literal.Kind == token.STRING {
									if value := constant.MakeFromLiteral(literal.Value, token.STRING, 0); value.Kind() == constant.String {
										constants[name.Name] = constant.StringVal(value)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return constants
}
