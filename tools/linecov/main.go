// Command linecov collects a per-test Go coverage index at one commit and
// selects, from it, the tests a change can affect (LPCV-V0-950..963).
//
//	linecov collect -root R -out INDEX [-packages a,b] [-parallel N] [-timeout D]
//	linecov select  -root R -index INDEX -base SHA [-static a,b]
//	linecov qualify -root R -index INDEX -packages a,b [-mutants N] [-seed S]
//
// collect needs a clean checkout; the index is bound to its HEAD. select reads
// the change base..HEAD of a checkout at the change head and prints one JSON
// plan: a non-empty reason means every static package runs whole.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: linecov collect|select [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "collect":
		err = collect(os.Args[2:])
	case "select":
		err = selectTests(os.Args[2:])
	case "qualify":
		err = qualify(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "linecov:", err)
		os.Exit(1)
	}
}

func output(root string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// runner names everything that must match between collection and selection:
// the toolchain, platform and coverage mode.
func runner(root string) (string, error) {
	v, err := output(root, "go", "env", "GOVERSION", "GOOS", "GOARCH")
	if err != nil {
		return "", err
	}
	return strings.Join(strings.Fields(v), " ") + " atomic", nil
}

// packages lists the module-relative directories of the module's packages.
func packages(root string) (module string, dirs []string, err error) {
	if module, err = output(root, "go", "list", "-m"); err != nil {
		return "", nil, err
	}
	out, err := output(root, "go", "list", "./...")
	if err != nil {
		return "", nil, err
	}
	for _, p := range strings.Fields(out) {
		switch {
		case p == module:
			dirs = append(dirs, ".")
		case strings.HasPrefix(p, module+"/"):
			dirs = append(dirs, strings.TrimPrefix(p, module+"/"))
		}
	}
	return module, dirs, nil
}

// modulePath reads the module directive of root's go.mod.
func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", errors.New("go.mod has no module directive")
}

func split(list string) []string {
	if list == "" {
		return nil
	}
	return strings.Split(list, ",")
}

func collect(args []string) error {
	fs := flag.NewFlagSet("collect", flag.ExitOnError)
	root := fs.String("root", ".", "clean checkout to collect at")
	out := fs.String("out", "", "index file to write")
	only := fs.String("packages", "", "comma-separated module-relative directories (default: every package)")
	parallel := fs.Int("parallel", runtime.NumCPU(), "concurrent test processes")
	timeout := fs.Duration("timeout", 10*time.Minute, "per test process")
	scratch := fs.String("work", "", "scratch directory parent (default the system temp directory)")
	fs.Parse(args)
	if *out == "" {
		return errors.New("collect: -out is required")
	}
	if dirty, err := output(*root, "git", "status", "--porcelain", "--untracked-files=no"); err != nil || dirty != "" {
		return fmt.Errorf("collect: checkout must be clean (%v)", err)
	}
	commit, err := output(*root, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	run, err := runner(*root)
	if err != nil {
		return err
	}
	module, dirs, err := packages(*root)
	if err != nil {
		return err
	}
	if *only != "" {
		dirs = split(*only)
	}
	work, err := os.MkdirTemp(*scratch, "linecov-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	ctx := context.Background()
	start := time.Now()
	outcomes, err := linecov.Collect(ctx, *root, linecov.CollectOptions{Module: module, Packages: dirs, Work: work, Parallel: *parallel, Timeout: *timeout})
	if err != nil {
		return err
	}
	var records []linecov.Record
	divergent := 0
	for _, o := range outcomes {
		if o.Record == nil {
			divergent++
			fmt.Fprintf(os.Stderr, "linecov: isolation-divergent %s.%s: %s\n", o.Package, o.Name, o.Divergent)
			continue
		}
		records = append(records, *o.Record)
	}
	oids, err := linecov.OIDs(ctx, *root, commit)
	if err != nil {
		return err
	}
	ix, err := linecov.Build(module, commit, run, oids, records)
	if err != nil {
		return err
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err = ix.Encode(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "linecov: %d records (%d divergent), %d files, %s at %s in %s\n", len(records), divergent, len(ix.Files), run, commit, time.Since(start).Round(time.Second))
	return nil
}

// Plan is the select output: one decision per static package and, for each
// package that runs, its `go test` arguments.
type Plan struct {
	Index, Base, Head string
	Reason            string `json:",omitempty"`
	Packages          map[string]linecov.PackagePlan
	Run               []Invocation
	Excluded          []string
	ElapsedMS         int64
}

// Invocation is one `go test` package argument and its -run filter.
type Invocation struct {
	Package string
	Run     string `json:",omitempty"`
}

func selectTests(args []string) error {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	root := fs.String("root", ".", "checkout at the change head")
	index := fs.String("index", "", "index file")
	base := fs.String("base", "", "change base commit")
	static := fs.String("static", "", "comma-separated statically selected directories (default: every package)")
	fs.Parse(args)
	if *index == "" || *base == "" {
		return errors.New("select: -index and -base are required")
	}
	start := time.Now()
	ctx := context.Background()
	f, err := os.Open(*index)
	if err != nil {
		return err
	}
	ix, err := linecov.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	p, err := decide(ctx, *root, ix, *base, split(*static))
	if err != nil {
		return err
	}
	p.ElapsedMS = time.Since(start).Milliseconds()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

// decide plans base..HEAD of the checkout at root for the static directories,
// every package when static is empty.
func decide(ctx context.Context, root string, ix *linecov.Index, base string, static []string) (Plan, error) {
	head, err := output(root, "git", "rev-parse", "HEAD")
	if err != nil {
		return Plan{}, err
	}
	// Listing every package costs most of a select; a caller passing the
	// static plan needs only the module path.
	module, dirs := "", static
	if static == nil {
		module, dirs, err = packages(root)
	} else {
		module, err = modulePath(root)
	}
	if err != nil {
		return Plan{}, err
	}
	p := Plan{Index: ix.Commit, Base: base, Head: head}
	run, err := runner(root)
	switch {
	case err != nil:
		p.Reason = "runner unavailable: " + err.Error()
	case run != ix.Runner:
		p.Reason = fmt.Sprintf("index runner %q, current %q", ix.Runner, run)
	case module != ix.Module:
		p.Reason = fmt.Sprintf("index module %q, current %q", ix.Module, module)
	}
	if dirty, err := output(root, "git", "status", "--porcelain", "--untracked-files=no"); err != nil || dirty != "" {
		p.Reason = "checkout is not clean at its HEAD"
	}
	var drift, change linecov.Effect
	if p.Reason == "" {
		drift, change, err = effects(ctx, root, ix, base, head)
		if err != nil {
			p.Reason = err.Error()
		}
	}
	if p.Reason != "" {
		drift.Fallback = p.Reason
	}
	heads, err := linecov.HeadTests(root, dirs)
	if err != nil {
		return Plan{}, err
	}
	var reason string
	p.Packages, reason = ix.Narrow(dirs, heads, drift, change)
	if p.Reason == "" {
		p.Reason = reason
	}
	names := make([]string, 0, len(p.Packages))
	for d := range p.Packages {
		names = append(names, d)
	}
	sort.Strings(names)
	for _, d := range names {
		pkg := "./" + filepath.ToSlash(d)
		switch pp := p.Packages[d]; pp.Mode {
		case linecov.Whole:
			p.Run = append(p.Run, Invocation{Package: pkg})
		case linecov.Tests:
			p.Run = append(p.Run, Invocation{Package: pkg, Run: "^(" + strings.Join(pp.Tests, "|") + ")$"})
		case linecov.Compile:
			p.Run = append(p.Run, Invocation{Package: pkg, Run: "^$"})
		default:
			p.Excluded = append(p.Excluded, pkg)
		}
	}
	return p, nil
}

// effects analyses the history from the index commit to base (drift) and the
// change from base to head (LPCV-V0-951, LPCV-V0-952).
func effects(ctx context.Context, root string, ix *linecov.Index, base, head string) (drift, change linecov.Effect, err error) {
	if _, err = output(root, "git", "merge-base", "--is-ancestor", ix.Commit, base); err != nil {
		return drift, change, errors.New("index commit is not an ancestor of the change base")
	}
	d, err := linecov.Diff(ctx, root, ix.Commit, base)
	if err != nil {
		return drift, change, err
	}
	drift = ix.Analyze(d, linecov.Reader(ctx, root, ix.Commit))
	c, err := linecov.Diff(ctx, root, base, head)
	if err != nil {
		return drift, change, err
	}
	// A path the drift changed no longer has the index's blob at base, so
	// Analyze falls back for it rather than mapping stale lines.
	change = ix.Analyze(c, linecov.Reader(ctx, root, base))
	return drift, change, nil
}

// qualify is the LPCV-V0-963 fault injection on a real tree. root must be a
// clean checkout at the index commit; each mutant inserts a panic as the first
// statement of one multi-line coverage block of the collected packages,
// commits it, selects, runs every collected package whole, and requires every
// failing test to be kept. It restores root to the index commit after each.
func qualify(args []string) error {
	fs := flag.NewFlagSet("qualify", flag.ExitOnError)
	root := fs.String("root", ".", "clean disposable checkout at the index commit")
	index := fs.String("index", "", "index file")
	only := fs.String("packages", "", "comma-separated collected directories")
	mutants := fs.Int("mutants", 20, "mutants to try")
	seed := fs.Int64("seed", 1, "mutant site seed")
	fs.Parse(args)
	f, err := os.Open(*index)
	if err != nil {
		return err
	}
	ix, err := linecov.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	dirs := split(*only)
	if len(dirs) == 0 {
		return errors.New("qualify: -packages is required")
	}
	if head, err := output(*root, "git", "rev-parse", "HEAD"); err != nil || head != ix.Commit {
		return fmt.Errorf("qualify: root must be checked out at the index commit %s", ix.Commit)
	}
	type site struct {
		path string
		line int
	}
	in := map[string]bool{}
	for _, d := range dirs {
		in[d] = true
	}
	var sites []site
	for _, file := range ix.Files {
		if !in[filepath.ToSlash(filepath.Dir(file.Path))] {
			continue
		}
		src, err := os.ReadFile(filepath.Join(*root, filepath.FromSlash(file.Path)))
		if err != nil {
			return err
		}
		// A site is the line opening a statement list that continues on
		// later lines, so the inserted statement always compiles.
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file.Path, src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		clauses := map[*ast.BlockStmt]bool{} // switch and select bodies hold clauses, not statements
		ast.Inspect(parsed, func(n ast.Node) bool {
			open, close := token.NoPos, token.NoPos
			switch n := n.(type) {
			case *ast.SwitchStmt:
				clauses[n.Body] = true
			case *ast.TypeSwitchStmt:
				clauses[n.Body] = true
			case *ast.SelectStmt:
				clauses[n.Body] = true
			case *ast.BlockStmt:
				if clauses[n] {
					break
				}
				open, close = n.Lbrace, n.Rbrace
			case *ast.CaseClause:
				open, close = n.Colon, n.End()
			case *ast.CommClause:
				open, close = n.Colon, n.End()
			}
			if open.IsValid() && fset.Position(close).Line > fset.Position(open).Line+1 {
				sites = append(sites, site{file.Path, fset.Position(open).Line})
			}
			return true
		})
	}
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].path < sites[j].path || sites[i].path == sites[j].path && sites[i].line < sites[j].line
	})
	rand.New(rand.NewSource(*seed)).Shuffle(len(sites), func(i, j int) { sites[i], sites[j] = sites[j], sites[i] })
	if len(sites) > *mutants {
		sites = sites[:*mutants]
	}
	ctx := context.Background()
	git := func(args ...string) error {
		_, err := output(*root, "git", append([]string{"-c", "user.name=linecov", "-c", "user.email=linecov@invalid", "-c", "commit.gpgsign=false"}, args...)...)
		return err
	}
	defer git("reset", "-q", "--hard", ix.Commit)
	total, misses, failing, invalid, keptSum := 0, 0, 0, 0, 0
	for _, m := range sites {
		if err := git("reset", "-q", "--hard", ix.Commit); err != nil {
			return err
		}
		full := filepath.Join(*root, filepath.FromSlash(m.path))
		src, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		lines := strings.SplitAfter(string(src), "\n")
		mutated := strings.Join(lines[:m.line], "") + "\tpanic(\"linecov-mutant\")\n" + strings.Join(lines[m.line:], "")
		if err := os.WriteFile(full, []byte(mutated), 0o644); err != nil {
			return err
		}
		if err := git("commit", "-qam", "mutant"); err != nil {
			return err
		}
		start := time.Now()
		p, err := decide(ctx, *root, ix, ix.Commit, dirs)
		if err != nil {
			return err
		}
		elapsed := time.Since(start)
		kept := map[string]bool{}
		heads, err := linecov.HeadTests(*root, dirs)
		if err != nil {
			return err
		}
		all := 0
		for d, pp := range p.Packages {
			all += len(heads[d])
			names := pp.Tests
			if pp.Mode == linecov.Whole {
				names = heads[d]
			}
			for _, n := range names {
				kept[d+"."+n] = true
			}
		}
		failed, buildFail := failingTests(*root, ix.Module, dirs)
		total++
		var missed []string
		for t := range failed {
			if !kept[t] {
				missed = append(missed, t)
			}
		}
		sort.Strings(missed)
		if len(failed) > 0 {
			failing++
		} else if buildFail {
			invalid++ // the mutant did not build: proves nothing
		}
		if len(missed) > 0 {
			misses++
		}
		keptSum += len(kept)
		fmt.Printf("%s:%d fallback=%q kept=%d/%d failed=%d build-fail=%v select=%s MISS=%v\n", m.path, m.line, p.Reason, len(kept), all, len(failed), buildFail, elapsed.Round(time.Millisecond), missed)
	}
	fmt.Printf("mutants=%d failing=%d invalid=%d misses=%d mean-kept=%.1f\n", total, failing, invalid, misses, float64(keptSum)/float64(max(total, 1)))
	if misses > 0 {
		return fmt.Errorf("%d mutants missed a failing test", misses)
	}
	if invalid > 0 {
		return fmt.Errorf("%d mutants did not build", invalid)
	}
	return nil
}

// failingTests runs the directories whole and returns failing top-level
// tests as dir.Name, and whether any package failed outside a test.
func failingTests(root, module string, dirs []string) (map[string]bool, bool) {
	args := []string{"test", "-count=1", "-json"}
	for _, d := range dirs {
		args = append(args, "./"+d)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	out, _ := cmd.Output()
	failed := map[string]bool{}
	pkgFail := false
	for _, line := range bytes.Split(out, []byte("\n")) {
		var ev struct{ Action, Package, Test string }
		if json.Unmarshal(line, &ev) != nil || ev.Action != "fail" || strings.Contains(ev.Test, "/") {
			continue
		}
		dir := strings.TrimPrefix(strings.TrimPrefix(ev.Package, module), "/")
		if dir == "" {
			dir = "."
		}
		if ev.Test == "" {
			pkgFail = true
			continue
		}
		failed[dir+"."+ev.Test] = true
	}
	return failed, pkgFail
}
