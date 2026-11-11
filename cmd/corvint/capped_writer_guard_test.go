// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// cappedWriterScanSkip are directories that hold no Go the product or its
// tests compile: fixtures and vendored trees. Dot directories (Git and
// state stores) other than .github are skipped too.
var cappedWriterScanSkip = map[string]bool{"node_modules": true, "testdata": true, "vendor": true}

// cappedWriterFile is what one Go file contributes to the guard: the struct
// types that embed bytes.Buffer and the types that declare a Write method.
type cappedWriterFile struct {
	pkg      string
	embeds   map[string]token.Position
	writers  map[string]bool
	parseErr error
}

// scanCappedWriterFile parses one Go file. Embedding is resolved through the
// file's own import name for "bytes", so an aliased import is still seen.
func scanCappedWriterFile(rel string, source []byte) cappedWriterFile {
	result := cappedWriterFile{embeds: map[string]token.Position{}, writers: map[string]bool{}}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, rel, source, parser.SkipObjectResolution)
	if err != nil {
		result.parseErr = err
		return result
	}
	result.pkg = file.Name.Name
	bytesName := ""
	for _, spec := range file.Imports {
		if value, _ := strconv.Unquote(spec.Path.Value); value == "bytes" {
			bytesName = "bytes"
			if spec.Name != nil {
				bytesName = spec.Name.Name
			}
		}
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Recv != nil && declaration.Name.Name == "Write" && len(declaration.Recv.List) == 1 {
				if name := receiverTypeName(declaration.Recv.List[0].Type); name != "" {
					result.writers[name] = true
				}
			}
		case *ast.GenDecl:
			if bytesName == "" || bytesName == "_" || declaration.Tok != token.TYPE {
				continue
			}
			for _, spec := range declaration.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					if len(field.Names) == 0 && isBytesBuffer(field.Type, bytesName) {
						result.embeds[typeSpec.Name.Name] = fileSet.Position(field.Pos())
					}
				}
			}
		}
	}
	return result
}

// receiverTypeName is the base type name of a method receiver: T, *T, T[P]
// and *T[P] all yield T.
func receiverTypeName(expression ast.Expr) string {
	for {
		switch typed := expression.(type) {
		case *ast.StarExpr:
			expression = typed.X
		case *ast.IndexExpr:
			expression = typed.X
		case *ast.IndexListExpr:
			expression = typed.X
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}

// isBytesBuffer reports whether an embedded field is bytes.Buffer or
// *bytes.Buffer under the file's import name for "bytes". A dot import
// embeds the bare name Buffer.
func isBytesBuffer(expression ast.Expr, bytesName string) bool {
	if star, ok := expression.(*ast.StarExpr); ok {
		expression = star.X
	}
	if bytesName == "." {
		identifier, ok := expression.(*ast.Ident)
		return ok && identifier.Name == "Buffer"
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Buffer" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == bytesName
}

// cappedWriterViolations groups scanned files by directory and package and
// returns one "path:line Type" site for every type that both embeds
// bytes.Buffer and declares Write in that package, sorted.
func cappedWriterViolations(files map[string][]byte) ([]string, error) {
	type packageKey struct{ dir, pkg string }
	embeds := map[packageKey]map[string]string{}
	writers := map[packageKey]map[string]bool{}
	for rel, source := range files {
		scanned := scanCappedWriterFile(rel, source)
		if scanned.parseErr != nil {
			return nil, scanned.parseErr
		}
		key := packageKey{path.Dir(rel), scanned.pkg}
		for name, position := range scanned.embeds {
			if embeds[key] == nil {
				embeds[key] = map[string]string{}
			}
			embeds[key][name] = rel + ":" + strconv.Itoa(position.Line)
		}
		for name := range scanned.writers {
			if writers[key] == nil {
				writers[key] = map[string]bool{}
			}
			writers[key][name] = true
		}
	}
	var sites []string
	for key, types := range embeds {
		for name, site := range types {
			if writers[key][name] {
				sites = append(sites, site+" "+name)
			}
		}
	}
	slices.Sort(sites)
	return sites, nil
}

// TestCappedWritersEmbedNoBuffer_GPKV09930 guards V1-5122 (GPK-V0-9930): a
// type that declares Write must not embed bytes.Buffer, because the embedded
// buffer promotes ReadFrom, WriteString, WriteByte and WriteRune, and io.Copy
// (os/exec's child-output copy) and io.WriteString then skip the override and
// its cap. It scans every Go file in the repository, tests included, outside
// fixture and vendored trees.
func TestCappedWritersEmbedNoBuffer_GPKV09930(t *testing.T) {
	t.Parallel()
	// The spy must see each shape that promotes the buffer's write methods.
	for name, sources := range map[string]map[string]string{
		"value embed":      {"a/a.go": "package a\nimport \"bytes\"\ntype capped struct {\n\tbytes.Buffer\n\tlimit int\n}\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"pointer embed":    {"a/a.go": "package a\nimport \"bytes\"\ntype capped struct{ *bytes.Buffer }\nfunc (c capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"aliased import":   {"a/a.go": "package a\nimport b \"bytes\"\ntype capped struct{ b.Buffer }\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"dot import":       {"a/a.go": "package a\nimport . \"bytes\"\ntype capped struct{ Buffer }\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"generic receiver": {"a/a.go": "package a\nimport \"bytes\"\ntype capped[T any] struct{ bytes.Buffer; v T }\nfunc (c *capped[T]) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"write in another file": {
			"a/a.go":       "package a\nimport \"bytes\"\ntype capped struct{ bytes.Buffer }\n",
			"a/a_write.go": "package a\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n",
		},
	} {
		sites, err := cappedWriterViolations(stringFiles(sources))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(sites) != 1 || !strings.HasSuffix(sites[0], " capped") || !strings.HasPrefix(sites[0], "a/a.go:") {
			t.Fatalf("SPY_INEFFECTIVE: %s yields %v", name, sites)
		}
	}
	for name, sources := range map[string]map[string]string{
		"named field":                        {"a/a.go": "package a\nimport \"bytes\"\ntype capped struct{ buffer bytes.Buffer }\nfunc (c *capped) Write(p []byte) (int, error) { return c.buffer.Write(p) }\n"},
		"embed without Write":                {"a/a.go": "package a\nimport \"bytes\"\ntype encoder struct{ bytes.Buffer }\nfunc (e *encoder) raw(v string) { _, _ = e.WriteString(v) }\n"},
		"Write on another type":              {"a/a.go": "package a\nimport \"bytes\"\ntype held struct{ bytes.Buffer }\ntype other struct{}\nfunc (o other) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"Write in another package":           {"a/a.go": "package a\nimport \"bytes\"\ntype capped struct{ bytes.Buffer }\n", "b/b.go": "package b\ntype capped struct{}\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"Write in the external test package": {"a/a.go": "package a\nimport \"bytes\"\ntype capped struct{ bytes.Buffer }\n", "a/a_test.go": "package a_test\ntype capped struct{}\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
		"another package's Buffer":           {"a/a.go": "package a\nimport (\n\t\"bytes\"\n\t\"other\"\n)\nvar _ bytes.Reader\ntype capped struct{ other.Buffer }\nfunc (c *capped) Write(p []byte) (int, error) { return len(p), nil }\n"},
	} {
		sites, err := cappedWriterViolations(stringFiles(sources))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(sites) != 0 {
			t.Fatalf("SPY_FALSE_POSITIVE: %s yields %v", name, sites)
		}
	}

	root := moduleRoot(t)
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name != root && (cappedWriterScanSkip[entry.Name()] || (strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".github")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || filepath.Ext(name) != ".go" {
			return nil
		}
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = source
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("the scan read no Go file")
	}
	sites, err := cappedWriterViolations(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("a type that declares Write embeds bytes.Buffer, so its promoted ReadFrom and WriteString skip the cap; hold the buffer as a named field (GPK-V0-9930):\n%s", strings.Join(sites, "\n"))
	}
}

func stringFiles(sources map[string]string) map[string][]byte {
	files := make(map[string][]byte, len(sources))
	for name, source := range sources {
		files[name] = []byte(source)
	}
	return files
}

// onlyReader hides every method but Read, as os/exec's copy of a child's
// pipe reaches the destination: io.Copy then prefers dst.ReadFrom.
type onlyReader struct{ io.Reader }

// TestNegateOutputCapHoldsOnEveryWritePath_GPKV09930 drives negateOutput
// past its limit through io.Copy and io.WriteString (V1-5122, GPK-V0-9930,
// LPCV-V0-941): the overrun signal is set and the buffer keeps no byte past
// the cap.
func TestNegateOutputCapHoldsOnEveryWritePath_GPKV09930(t *testing.T) {
	t.Parallel()
	const limit = 16
	over := strings.Repeat("x", 4*limit)
	if _, ok := any(&negateOutput{}).(io.ReaderFrom); ok {
		t.Fatal("negateOutput exposes ReadFrom, which io.Copy prefers over the capped Write")
	}
	if _, ok := any(&negateOutput{}).(io.StringWriter); ok {
		t.Fatal("negateOutput exposes WriteString, which io.WriteString prefers over the capped Write")
	}
	for name, write := range map[string]func(io.Writer) error{
		"io.Copy from a bare reader": func(w io.Writer) error {
			_, err := io.Copy(w, onlyReader{strings.NewReader(over)})
			return err
		},
		"io.Copy from a WriterTo": func(w io.Writer) error {
			_, err := io.Copy(w, strings.NewReader(over))
			return err
		},
		"io.WriteString": func(w io.Writer) error {
			_, err := io.WriteString(w, over)
			return err
		},
	} {
		output := &negateOutput{limit: limit}
		if err := write(output); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !output.overrun || output.buffer.Len() > limit {
			t.Fatalf("%s: overrun=%v retained=%d bytes past the %d-byte cap", name, output.overrun, output.buffer.Len(), limit)
		}
	}
}
