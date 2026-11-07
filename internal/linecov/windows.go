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
	"sort"
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
			}{pos, fmt.Sprintf("\nif d := corvintLCOS.Getenv(\"CORVINT_LINECOV_WINDOWS\"); d != \"\" {\n if _, err := corvintLCOS.Stat(corvintLCPath.Join(d, \"order\")); corvintLCOS.IsNotExist(err) { p := corvintLCPath.Join(d, \"setup\"); if err := corvintLCOS.MkdirAll(p,0700); err != nil { panic(err) }; if err := corvintLCCov.WriteMetaDir(p); err != nil { panic(err) }; if err := corvintLCCov.WriteCountersDir(p); err != nil { panic(err) } }; if f, err := corvintLCOS.OpenFile(corvintLCPath.Join(d, \"order\"), corvintLCOS.O_CREATE|corvintLCOS.O_WRONLY|corvintLCOS.O_APPEND, 0600); err == nil { _, _ = f.WriteString(\"%s\\n\"); _ = f.Close() } else { panic(err) }; p := corvintLCPath.Join(d, \"before-%s\"); if err := corvintLCOS.MkdirAll(p,0700); err != nil { panic(err) }; if err := corvintLCCov.WriteMetaDir(p); err != nil { panic(err) }; if err := corvintLCCov.WriteCountersDir(p); err != nil { panic(err) }; %s.Cleanup(func(){\n p := corvintLCPath.Join(d, %q); if err := corvintLCOS.MkdirAll(p, 0700); err != nil { panic(err) }; if err := corvintLCCov.WriteMetaDir(p); err != nil { panic(err) }; if err := corvintLCCov.WriteCountersDir(p); err != nil { panic(err) }\n })\n}\n", name, name, param.Names[0].Name, name)})
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

// attributeWindows uses cumulative snapshots, never resets. The ordinary exit
// profile remains an independent suite-wide union. Before/after differences
// attribute test bodies; reach between snapshots has unknown ownership and is
// retained by every test, even when another isolated test reaches it too.
func attributeWindows(ctxRun func(string) ([]byte, error), outcomes []Outcome, windows map[string]string) []Outcome {
	isolated := map[string]map[string]map[string]bool{}
	for _, o := range outcomes {
		if o.Record == nil || o.Name == InOrder {
			continue
		}
		if isolated[o.Package] == nil {
			isolated[o.Package] = map[string]map[string]bool{}
		}
		isolated[o.Package][o.Name] = map[string]bool{}
		counts, _ := profileCounts(o.Record.Profile)
		for k, n := range counts {
			if n > 0 {
				isolated[o.Package][o.Name][k] = true
			}
		}
	}
	for i := range outcomes {
		o := &outcomes[i]
		if o.Name != InOrder || o.Record == nil || windows[o.Package] == "" {
			continue
		}
		dir := windows[o.Package]
		// A failed snapshot cannot justify attribution. Keep the reset-free suite
		// reach on every test so an isolated overlap cannot hide a residual owner.
		keepAll := func(data []byte) {
			for j := range outcomes {
				if outcomes[j].Package == o.Package && outcomes[j].Name != InOrder && outcomes[j].Record != nil {
					outcomes[j].Record.Profile = append(outcomes[j].Record.Profile, data...)
				}
			}
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			o.WindowFallback = "window-dump-unavailable"
			keepAll(o.Record.Profile)
			continue
		}
		dumps := map[string]map[string]uint64{}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			data, err := ctxRun(filepath.Join(dir, e.Name()))
			if err == nil {
				dumps[e.Name()], err = profileCounts(data)
			}
			if err != nil {
				o.WindowFallback = "window-dump-invalid"
				break
			}
		}
		if o.WindowFallback != "" {
			keepAll(o.Record.Profile)
			continue
		}
		order, err := os.ReadFile(filepath.Join(dir, "order"))
		if err != nil {
			o.WindowFallback = "window-order-unavailable"
			keepAll(o.Record.Profile)
			continue
		}
		previous := map[string]uint64{}
		var prior []*Record
		for _, name := range strings.Fields(string(order)) {
			var target *Record
			for j := range outcomes {
				if outcomes[j].Package == o.Package && outcomes[j].Name == name {
					target = outcomes[j].Record
					break
				}
			}
			before, startOK := dumps["before-"+name]
			after, endOK := dumps[name]
			if !startOK || !endOK || target == nil {
				o.WindowFallback = "incomplete-test-window"
				break
			}
			gap, err := counterDelta(before, previous)
			if err != nil {
				o.WindowFallback = "window-dump-invalid"
				break
			}
			keepAll(gap)
			delta, err := counterDelta(after, before)
			if err != nil {
				o.WindowFallback = "window-dump-invalid"
				break
			}
			counts, _ := profileCounts(delta)
			var only strings.Builder
			only.WriteString("mode: atomic\n")
			for _, line := range strings.Split(string(delta), "\n") {
				// Keys include the statement count; profileCounts uses the full prefix.
				prefix := line
				if n := strings.LastIndexByte(line, ' '); n >= 0 {
					prefix = line[:n]
				}
				if !strings.HasPrefix(line, "mode:") && counts[prefix] > 0 && !isolated[o.Package][name][prefix] {
					only.WriteString(line + "\n")
				}
			}
			for _, r := range prior {
				r.Profile = append(r.Profile, only.String()...)
			}
			target.Profile = append(target.Profile, delta...)
			prior = append(prior, target)
			previous = after
		}
		if o.WindowFallback != "" {
			keepAll(o.Record.Profile)
			continue
		}
		total, err := profileCounts(o.Record.Profile)
		if err == nil {
			var tail []byte
			tail, err = counterDelta(total, previous)
			if err == nil {
				keepAll(tail)
			}
		}
		if err != nil {
			o.WindowFallback = "window-dump-invalid"
			keepAll(o.Record.Profile)
		}
	}
	return outcomes
}

func profileCounts(data []byte) (map[string]uint64, error) {
	counts := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		n := strings.LastIndexByte(line, ' ')
		if n < 0 {
			return nil, fmt.Errorf("invalid coverage counter")
		}
		count, err := strconv.ParseUint(line[n+1:], 10, 64)
		if err != nil {
			return nil, err
		}
		counts[line[:n]] += count
	}
	return counts, nil
}

func counterDelta(after, before map[string]uint64) ([]byte, error) {
	for key, n := range before {
		if after[key] < n {
			return nil, fmt.Errorf("coverage counters decreased")
		}
	}
	keys := make([]string, 0, len(after))
	for key := range after {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("mode: atomic\n")
	for _, key := range keys {
		if n := after[key] - before[key]; n > 0 {
			fmt.Fprintf(&b, "%s %d\n", key, n)
		}
	}
	return []byte(b.String()), nil
}
