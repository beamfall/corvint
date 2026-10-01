package criterionexperiment

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
)

type sourceFile struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Sha256 string `json:"sha256"`
}

func blob(ctx context.Context, r *gitauth.Repository, commit, p string) ([]byte, string, error) {
	if _, e := r.CommitTree(ctx, commit); e != nil {
		return nil, "", e
	}
	entry, ok, e := r.LookupTreeEntry(ctx, commit, p)
	if e != nil {
		return nil, "", e
	}
	if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, "", fmt.Errorf("source is not regular immutable blob")
	}
	b, e := r.BlobBytes(ctx, entry.OID)
	return b, entry.Mode, e
}
func inventory(ctx context.Context, repo *gitauth.Repository, r Request, c Criterion, commit string) (map[string][]byte, map[string]string, string, error) {
	if _, e := repo.CommitTree(ctx, commit); e != nil {
		return nil, nil, "", e
	}
	paths, e := repo.TreePaths(ctx, commit, r.Module)
	if e != nil {
		return nil, nil, "", e
	}
	if len(paths) == 0 || len(paths) > 256 {
		return nil, nil, "", fmt.Errorf("source inventory exceeds profile")
	}
	files := map[string][]byte{}
	modes := map[string]string{}
	total := 0
	for _, p := range paths {
		entry, ok, e := repo.LookupTreeEntry(ctx, commit, p)
		if e != nil {
			return nil, nil, "", e
		}
		if !ok {
			return nil, nil, "", fmt.Errorf("source disappeared")
		}
		if entry.Type == "tree" {
			continue
		}
		b, mode, e := blob(ctx, repo, commit, p)
		if e != nil {
			return nil, nil, "", e
		}
		rel := strings.TrimPrefix(p, r.Module+"/")
		if rel == p || !literal(rel) {
			return nil, nil, "", fmt.Errorf("invalid source path")
		}
		total += len(b)
		if total > 16<<20 {
			return nil, nil, "", fmt.Errorf("source byte bound")
		}
		files[rel] = b
		modes[rel] = mode
	}
	oracle, mode, e := blob(ctx, repo, c.Oracle.Commit, c.Oracle.Path)
	if e != nil {
		return nil, nil, "", e
	}
	rel := strings.TrimPrefix(c.Oracle.Path, r.Module+"/")
	if path.Dir(rel) != c.Package {
		return nil, nil, "", fmt.Errorf("oracle must reside in selected package")
	}
	parsed, parseErr := parser.ParseFile(token.NewFileSet(), rel, oracle, 0)
	if parseErr != nil {
		return nil, nil, "", fmt.Errorf("invalid oracle")
	}
	found := false
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == c.Test {
			found = true
		}
	}
	if !found {
		return nil, nil, "", fmt.Errorf("named test must be declared in pinned oracle")
	}
	files[rel] = oracle
	modes[rel] = mode
	anchor, _, e := blob(ctx, repo, c.Authority.AnchorCommit, c.Authority.AnchorPath)
	if e != nil {
		return nil, nil, "", e
	}
	if Digest(anchor) != c.Authority.AnchorSha256 {
		return nil, nil, "", fmt.Errorf("oracle authority anchor mismatch")
	}
	// The reviewer attests relevance; byte equality never establishes adequacy.
	if len(files) > 256 {
		return nil, nil, "", fmt.Errorf("overlay file bound")
	}
	total = 0
	for _, b := range files {
		total += len(b)
	}
	if total > 16<<20 {
		return nil, nil, "", fmt.Errorf("overlay byte bound")
	}
	if e = profile(files); e != nil {
		return nil, nil, "", e
	}
	keys := make([]string, 0, len(files))
	for p := range files {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	rows := make([]sourceFile, 0, len(keys))
	for _, p := range keys {
		rows = append(rows, sourceFile{p, modes[p], Digest(files[p])})
	}
	return files, modes, CanonicalDigest(rows), nil
}
func profile(files map[string][]byte) error {
	mod, ok := files["go.mod"]
	if !ok {
		return fmt.Errorf("module missing")
	}
	lines := strings.Split(string(mod), "\n")
	module := ""
	goVersion := false
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			return fmt.Errorf("unsupported module directive")
		}
		switch f[0] {
		case "module":
			if module != "" || !literal(f[1]) {
				return fmt.Errorf("invalid module")
			}
			module = f[1]
		case "go":
			if goVersion {
				return fmt.Errorf("duplicate go")
			}
			goVersion = true
		default:
			return fmt.Errorf("only module/go directives supported")
		}
	}
	if module == "" || !goVersion {
		return fmt.Errorf("module/go required")
	}
	for p, b := range files {
		if p != "go.mod" && (path.Base(p) == "go.mod" || path.Base(p) == "go.work" || path.Base(p) == "go.sum") {
			return fmt.Errorf("nested module/workspace/dependencies unsupported")
		}
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), p, b, parser.ImportsOnly)
		if e != nil {
			return fmt.Errorf("invalid Go source")
		}
		for _, imp := range f.Imports {
			v, e := strconv.Unquote(imp.Path.Value)
			if e != nil {
				return e
			}
			if v == "C" || strings.Contains(strings.Split(v, "/")[0], ".") || strings.HasPrefix(v, ".") || strings.HasPrefix(v, "/") {
				return fmt.Errorf("only standard library imports supported")
			}
		}
	}
	return nil
}

func packageName(files map[string][]byte, c Criterion) string {
	for _, line := range strings.Split(string(files["go.mod"]), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "module" {
			if c.Package == "." {
				return f[1]
			}
			return f[1] + "/" + c.Package
		}
	}
	return ""
}
