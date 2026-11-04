// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// windowOverlay wraps top-level tests in scratch sources. It never edits the
// checkout. Parallel tests and unsupported entry points retain the suite union
// (LPCV-V0-962); overlapping counter windows cannot support attribution.
func windowOverlay(dir, work string) (string, string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return "", "", err
	}
	sources, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	hasSource := false
	for _, p := range sources {
		if !strings.HasSuffix(p, "_test.go") {
			hasSource = true
		}
	}
	if !hasSource {
		return "", "no-instrumentable-source", nil
	}
	replace := map[string]string{}
	for i, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return "", "", err
		}
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, p, src, parser.ParseComments)
		if err != nil {
			return "", "", err
		}
		unsafe := ""
		ast.Inspect(f, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "Parallel" {
				unsafe = "parallel-test-windows"
			}
			if fn, ok := n.(*ast.FuncDecl); ok && (strings.HasPrefix(fn.Name.Name, "Example") || strings.HasPrefix(fn.Name.Name, "Fuzz")) {
				unsafe = "unsupported-test-entrypoint"
			}
			return true
		})
		if unsafe != "" {
			return "", unsafe, nil
		}
		var inserts []struct {
			at   int
			text string
		}
		used := false
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name, ok := topLevelTest(fn)
			if !ok || !strings.HasPrefix(name, "Test") || fn.Type.Params.NumFields() != 1 {
				continue
			}
			param := fn.Type.Params.List[0]
			if len(param.Names) != 1 || param.Names[0].Name == "_" {
				return "", "unnamed-test-parameter", nil
			}
			pos := fs.Position(fn.Body.Lbrace).Offset + 1
			inserts = append(inserts, struct {
				at   int
				text string
			}{pos, fmt.Sprintf("\nif d := corvintLCOS.Getenv(\"CORVINT_LINECOV_WINDOWS\"); d != \"\" {\n if _, err := corvintLCOS.Stat(corvintLCPath.Join(d, \"order\")); corvintLCOS.IsNotExist(err) { p := corvintLCPath.Join(d, \"setup\"); if err := corvintLCOS.MkdirAll(p,0700); err != nil { panic(err) }; if err := corvintLCCov.WriteMetaDir(p); err != nil { panic(err) }; if err := corvintLCCov.WriteCountersDir(p); err != nil { panic(err) } }; if f, err := corvintLCOS.OpenFile(corvintLCPath.Join(d, \"order\"), corvintLCOS.O_CREATE|corvintLCOS.O_WRONLY|corvintLCOS.O_APPEND, 0600); err == nil { _, _ = f.WriteString(\"%s\\n\"); _ = f.Close() } else { panic(err) }; if err := corvintLCCov.ClearCounters(); err != nil { panic(err) }; %s.Cleanup(func(){\n p := corvintLCPath.Join(d, %q); if err := corvintLCOS.MkdirAll(p, 0700); err != nil { panic(err) }; if err := corvintLCCov.WriteMetaDir(p); err != nil { panic(err) }; if err := corvintLCCov.WriteCountersDir(p); err != nil { panic(err) }\n })\n}\n", name, param.Names[0].Name, name)})
			used = true
		}
		if !used {
			continue
		}
		for j := len(inserts) - 1; j >= 0; j-- {
			x := inserts[j]
			src = append(append(append([]byte{}, src[:x.at]...), []byte(x.text)...), src[x.at:]...)
		}
		// Add imports with the parser so both grouped and single imports work.
		fs = token.NewFileSet()
		f, err = parser.ParseFile(fs, p, src, parser.ParseComments)
		if err != nil {
			return "", "", err
		}
		decl := &ast.GenDecl{Tok: token.IMPORT, Lparen: 1}
		for _, im := range [][2]string{{"corvintLCCov", "runtime/coverage"}, {"corvintLCOS", "os"}, {"corvintLCPath", "path/filepath"}} {
			decl.Specs = append(decl.Specs, &ast.ImportSpec{Name: ast.NewIdent(im[0]), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(im[1])}})
		}
		f.Decls = append([]ast.Decl{decl}, f.Decls...)
		var b bytes.Buffer
		if err = format.Node(&b, fs, f); err != nil {
			return "", "", err
		}
		dst := filepath.Join(work, fmt.Sprintf("window-%d.go", i))
		if err = os.WriteFile(dst, b.Bytes(), 0600); err != nil {
			return "", "", err
		}
		replace[p] = dst
	}
	if len(replace) == 0 {
		return "", "no-test-windows", nil
	}
	data, _ := json.Marshal(struct{ Replace map[string]string }{replace})
	p := filepath.Join(work, "overlay.json")
	err = os.WriteFile(p, data, 0600)
	return p, "", err
}

// attributeWindows unions each test's isolated and in-order profiles. An
// order-only block also retains every preceding test: those windows may have
// established the state needed to reach it. The suite union remains as a
// residual owner for setup, teardown, child and unclosed-window coverage.
func attributeWindows(ctxRun func(string) ([]byte, error), outcomes []Outcome, windows map[string]string) []Outcome {
	isolated := map[string]map[string]bool{}
	for _, o := range outcomes {
		if o.Record == nil || o.Name == InOrder {
			continue
		}
		if isolated[o.Package] == nil {
			isolated[o.Package] = map[string]bool{}
		}
		for _, l := range strings.Split(string(o.Record.Profile), "\n") {
			k, n, ok := strings.Cut(l, " ")
			if ok && !strings.HasSuffix(n, " 0") {
				isolated[o.Package][k] = true
			}
		}
	}
	for i := range outcomes {
		o := &outcomes[i]
		if o.Name != InOrder || o.Record == nil {
			continue
		}
		dir := windows[o.Package]
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			o.WindowFallback = "window-dump-unavailable"
			continue
		}
		type win struct {
			name string
			data []byte
		}
		var ws []win
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(dir, e.Name())
			d, err := ctxRun(p)
			if err != nil {
				o.WindowFallback = "window-dump-invalid"
				ws = nil
				break
			}
			ws = append(ws, win{e.Name(), d})
			o.Record.Profile = append(o.Record.Profile, d...)
		}
		// Directory iteration is lexical, but dependencies follow the actual start
		// order, recorded by the marker written before each window opens.
		order, err := os.ReadFile(filepath.Join(dir, "order"))
		if err != nil {
			o.WindowFallback = "window-order-unavailable"
			continue
		}
		byName := map[string][]byte{}
		for _, w := range ws {
			byName[w.name] = w.data
		}
		var prior []*Record
		for _, n := range strings.Fields(string(order)) {
			var target *Record
			for j := range outcomes {
				if outcomes[j].Package == o.Package && outcomes[j].Name == n {
					target = outcomes[j].Record
					break
				}
			}
			d, ok := byName[n]
			if !ok || target == nil {
				o.WindowFallback = "incomplete-test-window"
				continue
			}
			var only strings.Builder
			only.WriteString("mode: atomic\n")
			for _, l := range strings.Split(string(d), "\n") {
				k, _, ok := strings.Cut(l, " ")
				if ok && !strings.HasPrefix(l, "mode:") && !isolated[o.Package][k] {
					only.WriteString(l + "\n")
				}
			}
			for _, r := range prior {
				r.Profile = append(r.Profile, []byte(only.String())...)
			}
			target.Profile = append(target.Profile, d...)
			prior = append(prior, target)
		}
	}
	return outcomes
}
