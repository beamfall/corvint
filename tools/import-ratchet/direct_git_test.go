//corvint:read-kinds .go go.mod

package importratchet

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const allowlistPath = "tools/import-ratchet/direct_git_allowlist.txt"

// gitrunPackage is the one package whose Git spawns IRT-V0-020 does not list. It is compared
// against scanned package paths and names no read of its own, so it carries the fixture marker.
const gitrunPackage = "internal/gitrun" //corvint:fixture-path

// gitstatusPackage holds Executable, the resolved Git path some spawns pass as their program. It is
// compared against import paths and names no read of its own.
const gitstatusPackage = "internal/gitstatus" //corvint:fixture-path

//go:embed direct_git_allowlist.txt
var directGitAllowlist []byte

// directGitSites returns one PATH:FUNC key per scanned Git spawn (IRT-V0-020) in the non-test Go
// files of the packages in graph, outside gitrunPackage, sorted. PATH is module-relative and FUNC
// is the enclosing top-level function or TYPE.METHOD ("-" at package level).
func directGitSites(root, module string, graph map[string][]string) ([]string, error) {
	var sites []string
	for pkg := range graph {
		if pkg == gitrunPackage || strings.HasPrefix(pkg, gitrunPackage+"/") {
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			name := e.Name()
			if !e.Type().IsRegular() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}
			rel := name
			if pkg != "." {
				rel = pkg + "/" + name
			}
			for _, fn := range fileGitSpawns(file, module) {
				sites = append(sites, rel+":"+fn)
			}
		}
	}
	sort.Strings(sites)
	return sites, nil
}

// fileGitSpawns returns the enclosing function name of each scanned Git spawn in file: an
// exec.Command or exec.CommandContext call whose program argument is the literal "git", an
// identifier or selector whose last name contains "git" in any case, or a call of
// gitstatus.Executable() (IRT-V0-020).
func fileGitSpawns(file *ast.File, module string) []string {
	local, gitstatus := "", ""
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		}
		switch path {
		case "os/exec":
			local = "exec"
			if name != "" {
				local = name
			}
		case module + "/" + gitstatusPackage:
			gitstatus = "gitstatus"
			if name != "" {
				gitstatus = name
			}
		}
	}
	if local == "" || local == "_" || local == "." {
		return nil
	}
	var out []string
	for _, decl := range file.Decls {
		name := "-"
		if fn, ok := decl.(*ast.FuncDecl); ok {
			name = fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				name = receiverType(fn.Recv.List[0].Type) + "." + name
			}
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != local {
				return true
			}
			program := -1
			switch sel.Sel.Name {
			case "Command":
				program = 0
			case "CommandContext":
				program = 1
			}
			if program < 0 || len(call.Args) <= program {
				return true
			}
			if gitProgram(call.Args[program], gitstatus) {
				out = append(out, name)
			}
			return true
		})
	}
	return out
}

// gitProgram reports whether a program argument names Git: the literal "git", an identifier or
// selector whose last name contains "git" in any case, or a call of gitstatus.Executable() where
// gitstatus is the file's name for that package.
func gitProgram(arg ast.Expr, gitstatus string) bool {
	switch a := arg.(type) {
	case *ast.BasicLit:
		value, err := strconv.Unquote(a.Value)
		return a.Kind == token.STRING && err == nil && value == "git"
	case *ast.Ident:
		return strings.Contains(strings.ToLower(a.Name), "git")
	case *ast.SelectorExpr:
		return strings.Contains(strings.ToLower(a.Sel.Name), "git")
	case *ast.CallExpr:
		sel, ok := a.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Executable" || gitstatus == "" || gitstatus == "_" {
			return false
		}
		x, ok := sel.X.(*ast.Ident)
		return ok && x.Name == gitstatus
	}
	return false
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.IndexExpr:
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

// checkDirectGit compares the scanned spawn sites with the allowlist and returns sorted problems.
// A key listed N times admits N spawns in that function.
func checkDirectGit(sites []string, allowlist []byte) []string {
	var problems []string
	listed := map[string]int{}
	previous := ""
	text := string(allowlist)
	if text != "" && !strings.HasSuffix(text, "\n") {
		problems = append(problems, fmt.Sprintf("ALLOWLIST_FORMAT %s does not end with a newline", allowlistPath))
	}
	for i, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if text == "" {
			break
		}
		path, rest, ok1 := strings.Cut(line, ":")
		fn, reason, ok2 := strings.Cut(rest, ": ")
		if !ok1 || !ok2 || path == "" || fn == "" || strings.ContainsAny(path+fn, " \t") || strings.TrimSpace(reason) == "" {
			problems = append(problems, fmt.Sprintf("ALLOWLIST_FORMAT %s line %d %q: want PATH:FUNC: REASON with a non-blank reason", allowlistPath, i+1, line))
			continue
		}
		if line < previous {
			problems = append(problems, fmt.Sprintf("ALLOWLIST_FORMAT %s line %d %q is out of order: keep lines sorted", allowlistPath, i+1, line))
		}
		previous = line
		listed[path+":"+fn]++
	}
	found := map[string]int{}
	for _, s := range sites {
		found[s]++
	}
	for key, n := range found {
		if n > listed[key] {
			problems = append(problems, fmt.Sprintf("UNLISTED_DIRECT_GIT %s spawns git directly %d time(s), %d listed: route it through %s when the result is unchanged, else add one %s line per spawn with the reason (IRT-V0-020)",
				key, n, listed[key], gitrunPackage, allowlistPath))
		}
	}
	for key, n := range listed {
		if n > found[key] {
			problems = append(problems, fmt.Sprintf("STALE_DIRECT_GIT %s is listed %d time(s) but spawns git directly %d time(s): delete the extra line from %s (IRT-V0-020)",
				key, n, found[key], allowlistPath))
		}
	}
	sort.Strings(problems)
	return problems
}

// TestIRTV0020DirectGitSpawnsAreAllowlisted holds IRT-V0-020 on this checkout and proves its
// controls on a synthetic module: an unlisted spawn of each scanned program form (literal,
// identifier, selector, gitstatus.Executable()), a goBin program that is not reported, a stale
// entry, a gitrun spawn that needs no entry, and malformed lines.
func TestIRTV0020DirectGitSpawnsAreAllowlisted(t *testing.T) {
	m, _ := loadEmbedded(t)
	t.Run("IRT-V0-020 every checkout spawn outside gitrun is listed", func(t *testing.T) {
		root := filepath.Join("..", "..")
		graph, err := scanModule(root, m.Module)
		if err != nil {
			t.Fatal(err)
		}
		sites, err := directGitSites(root, m.Module, graph)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range checkDirectGit(sites, directGitAllowlist) {
			t.Error(p)
		}
		t.Logf("%d scanned Git spawns outside %s, all listed", len(sites), gitrunPackage)
	})
	t.Run("IRT-V0-020 control names unlisted, stale and malformed entries", func(t *testing.T) {
		root := t.TempDir()
		const module = "example.com/m"
		files := map[string]string{
			"go.mod":                   "module " + module + "\n",
			gitrunPackage + "/run.go":  "package gitrun\n\nimport \"os/exec\"\n\nfunc Run() { _ = exec.Command(\"git\") }\n",
			"a/a.go":                   "package a\n\nimport (\n\t\"context\"\n\tx \"os/exec\"\n)\n\ntype T struct{}\n\nfunc (*T) Read(ctx context.Context) { _ = x.CommandContext(ctx, \"git\", \"status\"); _ = x.Command(\"git\") }\n\nfunc Other() { _ = x.Command(\"go\", \"git\") }\n",
			"a/a_test.go":              "package a\n\nimport \"os/exec\"\n\nfunc helper() { _ = exec.Command(\"git\") }\n",
			"b/b.go":                   "package b\n\nimport \"os/exec\"\n\nvar _ = exec.Command(\"git\", \"version\")\n",
			gitstatusPackage + "/x.go": "package gitstatus\n\nfunc Executable() string { return \"git\" }\n",
			"d/d.go": "package d\n\nimport (\n\t\"context\"\n\t\"os/exec\"\n\n\tgs \"" + module + "/" + gitstatusPackage + "\"\n)\n\n" +
				"type R struct{ GitPath string }\n\n" +
				"func Ident(git string) { _ = exec.Command(git, \"status\") }\n\n" +
				"func (r R) Field(ctx context.Context) { _ = exec.CommandContext(ctx, r.GitPath) }\n\n" +
				"func Resolved() { _ = exec.Command(gs.Executable(), \"version\") }\n\n" +
				"func NotGit(goBin string) { _ = exec.Command(goBin, \"git\") }\n",
		}
		for name, body := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		graph, err := scanModule(root, module)
		if err != nil {
			t.Fatal(err)
		}
		sites, err := directGitSites(root, module, graph)
		if err != nil {
			t.Fatal(err)
		}
		// d/d.go:NotGit passes goBin, a name without "git", so it is not a site.
		want := []string{"a/a.go:T.Read", "a/a.go:T.Read", "b/b.go:-", "d/d.go:Ident", "d/d.go:R.Field", "d/d.go:Resolved"}
		if strings.Join(sites, ",") != strings.Join(want, ",") {
			t.Fatalf("SPY_INEFFECTIVE sites %q, want %q", sites, want)
		}
		for _, site := range sites {
			if site == "d/d.go:NotGit" {
				t.Errorf("SPY_INEFFECTIVE a goBin program was reported as a Git spawn")
			}
		}
		const newForms = "d/d.go:Ident: identifier\nd/d.go:R.Field: selector\nd/d.go:Resolved: gitstatus.Executable\n"
		if got := checkDirectGit(sites, []byte("a/a.go:T.Read: one\na/a.go:T.Read: two\nb/b.go:-: package var\n"+newForms)); len(got) != 0 {
			t.Errorf("complete allowlist refused: %q", got)
		}
		const listed = "a/a.go:T.Read: one\na/a.go:T.Read: two\nb/b.go:-: package var\n"
		cases := map[string]struct {
			allowlist string
			want      []string
		}{
			"unlisted second spawn":           {"a/a.go:T.Read: one\nb/b.go:-: package var\n" + newForms, []string{"UNLISTED_DIRECT_GIT a/a.go:T.Read "}},
			"unlisted identifier program":     {listed + "d/d.go:R.Field: selector\nd/d.go:Resolved: gitstatus.Executable\n", []string{"UNLISTED_DIRECT_GIT d/d.go:Ident "}},
			"unlisted selector program":       {listed + "d/d.go:Ident: identifier\nd/d.go:Resolved: gitstatus.Executable\n", []string{"UNLISTED_DIRECT_GIT d/d.go:R.Field "}},
			"unlisted gitstatus.Executable()": {listed + "d/d.go:Ident: identifier\nd/d.go:R.Field: selector\n", []string{"UNLISTED_DIRECT_GIT d/d.go:Resolved "}},
			"stale entry":                     {listed + "c/c.go:Gone: was here\n" + newForms, []string{"STALE_DIRECT_GIT c/c.go:Gone "}},
			"blank reason":                    {"a/a.go:T.Read: one\na/a.go:T.Read:  \nb/b.go:-: package var\n" + newForms, []string{"ALLOWLIST_FORMAT ", "UNLISTED_DIRECT_GIT a/a.go:T.Read "}},
			"unsorted":                        {"b/b.go:-: package var\na/a.go:T.Read: one\na/a.go:T.Read: two\n" + newForms, []string{"ALLOWLIST_FORMAT "}},
			"no final newline":                {listed + strings.TrimSuffix(newForms, "\n"), []string{"ALLOWLIST_FORMAT "}},
		}
		for name, c := range cases {
			got := checkDirectGit(sites, []byte(c.allowlist))
			ok := len(got) == len(c.want)
			for i := 0; ok && i < len(got); i++ {
				ok = strings.HasPrefix(got[i], c.want[i])
			}
			if !ok {
				t.Errorf("SPY_INEFFECTIVE %s: got %q, want problems starting %q", name, got, c.want)
			}
		}
	})
}
