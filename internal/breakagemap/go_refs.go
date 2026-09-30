package breakagemap

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Probe only the bounded ancestor go.mod metadata of the explicitly selected
// API. An unlisted nested module must not acquire the root module's identity.
func moduleBoundary(ctx context.Context, root string, c captured, sources map[string]captured) bool {
	args := []string{"ls-tree", "-z", c.repo.Commit, "--"}
	for dir := path.Dir(c.source.Path); ; dir = path.Dir(dir) {
		args = append(args, path.Join(dir, "go.mod"))
		if dir == "." {
			break
		}
	}
	raw, err := git(ctx, root, args...)
	if err != nil {
		return false
	}
	found := false
	for _, row := range strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00") {
		pair := strings.SplitN(row, "\t", 2)
		if len(pair) != 2 {
			return false
		}
		s, ok := sources[sourceKey(c.repo.ID, pair[1])]
		fields := strings.Fields(pair[0])
		if !ok || len(fields) != 3 || fields[1] != "blob" || s.source.Blob != fields[2] {
			return false
		}
		found = true
	}
	return found
}

func declarationBytes(b []byte, name string) (string, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "source.go", b, 0)
	if err != nil {
		return "", false
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return string(b[fset.PositionFor(fn.Pos(), false).Offset:fset.PositionFor(fn.End(), false).Offset]), true
		}
	}
	return "", false
}
func buildConditional(p string, b []byte) bool {
	if strings.Contains(string(b), "//go:build") || strings.Contains(string(b), "// +build") {
		return true
	}
	stem := strings.TrimSuffix(strings.TrimSuffix(path.Base(p), ".go"), "_test")
	for _, part := range strings.Split(stem, "_")[1:] {
		if strings.Contains(" aix android darwin dragonfly freebsd illumos ios js linux netbsd openbsd plan9 solaris wasip1 windows 386 amd64 arm arm64 loong64 mips mipsle mips64 mips64le ppc64 ppc64le riscv64 s390x wasm ", " "+part+" ") {
			return true
		}
	}
	return false
}
func modulePath(sources map[string]captured, c captured) (string, bool) {
	dir := path.Dir(c.source.Path)
	for {
		p := path.Join(dir, "go.mod")
		m, ok := sources[sourceKey(c.repo.ID, p)]
		if ok {
			for _, line := range strings.Split(string(m.text), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 2 && fields[0] == "module" {
					name := fields[1]
					if strings.HasPrefix(name, "\"") {
						var err error
						name, err = strconv.Unquote(name)
						if err != nil {
							return "", false
						}
					}
					if !strings.Contains(name, "/") || strings.ContainsAny(name, " \\\t\r\n") {
						return "", false
					}
					suffix := strings.TrimPrefix(path.Dir(c.source.Path), dir)
					suffix = strings.TrimPrefix(suffix, "/")
					if dir == "." {
						suffix = path.Dir(c.source.Path)
						if suffix == "." {
							suffix = ""
						}
					}
					return path.Join(name, suffix), true
				}
			}
			return "", false
		}
		if dir == "." {
			return "", false
		}
		dir = path.Dir(dir)
	}
}
func analyzeGo(r *Report, sources map[string]captured, selected captured, name string) {
	key := sourceKey(selected.repo.ID, selected.source.Path)
	if !strings.HasSuffix(selected.source.Path, ".go") {
		r.unknown(key, "unsupported-language: selected API has declaration-only relationships")
		return
	}
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, selected.source.Path, selected.text, 0)
	if err != nil {
		r.unknown(key, "malformed Go API source")
		return
	}
	if buildConditional(selected.source.Path, selected.text) {
		r.unknown(key, "API build constraints unresolved")
		return
	}
	var api *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			if fn.Recv != nil {
				r.unknown(key, "receiver method dispatch unresolved")
				continue
			}
			if api != nil {
				r.unknown(key, "ambiguous API declaration")
				return
			}
			api = fn
		}
	}
	if api == nil {
		r.unknown(key, "function declaration unresolved; types and methods unsupported")
		return
	}
	// //line directives describe logical compiler positions, not immutable blob
	// spans. Every evidence anchor must use physical source positions.
	start, end := fs.PositionFor(api.Pos(), false).Line, fs.PositionFor(api.End(), false).Line
	anchor, valid := selected.anchor(start, end)
	if !valid {
		r.unknown(key, "API declaration outside supplied span")
		return
	}
	pkg, ok := modulePath(sources, selected)
	if !ok {
		r.unknown(key, "pinned module identity unavailable")
		return
	}
	r.unknown(key, "imports are syntax matched against pinned module/package; dependency versions, replacements and type resolution unverified")
	r.unknown(key, "references within the selected API file are not enumerated")
	keys := make([]string, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(r.Edges) >= MaxEdges {
			r.Truncated = true
			return
		}
		c := sources[k]
		if k == key || path.Base(c.source.Path) == "go.mod" {
			continue
		}
		if !strings.HasSuffix(c.source.Path, ".go") {
			r.unknown(k, "unsupported-language: only declared provider relations available")
			continue
		}
		if buildConditional(c.source.Path, c.text) {
			r.unknown(k, "build constraints unresolved")
			continue
		}
		cfs := token.NewFileSet()
		cf, e := parser.ParseFile(cfs, c.source.Path, c.text, 0)
		if e != nil {
			r.unknown(k, "malformed Go source")
			continue
		}
		aliases := map[string]bool{}
		imported := false
		for _, imp := range cf.Imports {
			importPath, e := strconv.Unquote(imp.Path.Value)
			if e != nil {
				continue
			}
			if importPath != pkg {
				continue
			}
			imported = true
			alias := f.Name.Name
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "." {
				r.unknown(k, "dot import resolution unsupported")
				continue
			}
			if alias != "_" {
				aliases[alias] = true
			}
		}
		ownPkg, hasModule := modulePath(sources, c)
		same := hasModule && c.repo.ID == selected.repo.ID && ownPkg == pkg && cf.Name.Name == f.Name.Name
		if hasModule && ownPkg == pkg && c.repo.ID != selected.repo.ID {
			r.unknown(k, "same module/package in another repository is not the selected API")
		}
		calls := map[token.Pos]bool{}
		ast.Inspect(cf, func(n ast.Node) bool {
			if len(r.Edges) >= MaxEdges {
				r.Truncated = true
				return false
			}
			if call, ok := n.(*ast.CallExpr); ok {
				calls[call.Fun.Pos()] = true
			}
			return true
		})
		refs := 0
		ast.Inspect(cf, func(n ast.Node) bool {
			if len(r.Edges) >= MaxEdges {
				r.Truncated = true
				return false
			}
			var pos, end token.Pos
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if x.Sel.Name != name {
					return true
				}
				id, ok := x.X.(*ast.Ident)
				if !ok || !aliases[id.Name] {
					r.unknown(k, "same-named selector has unresolved receiver/package identity")
					return false
				}
				if id.Obj != nil {
					r.unknown(k, "package qualifier shadowed by local identifier")
					return false
				}
				if !ast.IsExported(name) {
					r.unknown(k, "unexported cross-package name")
					return false
				}
				pos, end = x.Pos(), x.End()
			case *ast.Ident:
				if !same || x.Name != name || x.Obj != nil {
					return true
				}
				if !calls[x.Pos()] {
					r.unknown(k, "unqualified non-call identifier has unresolved declaration identity")
					return true
				}
				pos, end = x.Pos(), x.End()
			default:
				return true
			}
			line, last := cfs.PositionFor(pos, false).Line, cfs.PositionFor(end, false).Line
			a, valid := c.anchor(line, last)
			if !valid {
				r.unknown(k, "reference outside supplied physical span")
				return false
			}
			kind := "syntax-reference"
			if calls[pos] {
				kind = "syntax-call"
			}
			if strings.HasSuffix(c.source.Path, "_test.go") {
				kind = "test-" + kind
			}
			r.add(Edge{Kind: kind, Confidence: "Go-syntax", Reason: "pinned module/package and identifier syntax; execution and behavior unverified", From: &a, To: &anchor, ByteState: "verified"})
			refs++
			return false
		})
		if imported && refs == 0 {
			a, valid := c.anchor(c.source.Start, c.source.End)
			if !valid {
				r.unknown(k, "import candidate outside supplied physical span")
				continue
			}
			r.add(Edge{Kind: "import-only", Confidence: "Go-syntax", Reason: "imports selected package; no resolved selected-symbol reference in admitted span", From: &a, To: &anchor, ByteState: "verified"})
		}
	}
}
