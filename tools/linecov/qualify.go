package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"path"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// QualificationProfile names the qualify report (GLS-V0-033) and
// QualifiedProfile the qualification record (GLS-V0-034).
const (
	QualificationProfile = "linecov-qualification/1"
	QualifiedProfile     = "linecov-qualified/1"
)

// qualifiedSources are the selector and collector files a qualification
// binds by blob OID (GLS-V0-034): any change to one needs a new one.
var qualifiedSources = linecov.QualifiedSources

// The site kinds, in the order the sample takes them (GLS-V0-030).
var siteKinds = []string{"case", "else-if", "loop", "closure", "statement"}

// Site is one mutation: a byte span of a file and its replacement.
type Site struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Kind     string `json:"kind"`
	Operator string `json:"operator"`
	start    int
	end      int
	repl     string
}

// MutantResult is one mutant's outcome.
type MutantResult struct {
	Index     int      `json:"index"`
	Site      Site     `json:"site"`
	State     string   `json:"state"` // killed, survived or invalid
	Detail    string   `json:"detail,omitempty"`
	Kept      []string `json:"kept,omitempty"` // unit=mode[:run]
	KeptCount int      `json:"keptCount"`
	// KeptByCoverage counts the kept tests whose record reached the change
	// (selected, direct or stale), and KeptOfPackage the kept tests of the
	// mutated package; a control passes with both zero (GLS-V0-032).
	KeptByCoverage int      `json:"keptByCoverage"`
	KeptOfPackage  int      `json:"keptOfPackage"`
	Failing        []string `json:"failing,omitempty"`
	Missed         []string `json:"missed,omitempty"`
	SelectMillis   float64  `json:"selectMillis"`
}

// BaselineResult retains a package outcome before any mutation.
type BaselineResult struct {
	Package string   `json:"package"`
	State   string   `json:"state"`
	Failing []string `json:"failing,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}

// QualReport is one qualify run (GLS-V0-033).
type QualReport struct {
	Sources   map[string]string `json:"sources,omitempty"`
	Baseline  []BaselineResult  `json:"baseline,omitempty"`
	Profile   string            `json:"profile"`
	Seed      int64             `json:"seed"`
	Index     ReportIndex       `json:"index"`
	Packages  []string          `json:"packages"`
	Suite     []string          `json:"suite"`
	Start     int               `json:"start"`
	Sites     int               `json:"sites"` // sites of the seeded order this run consumed
	Order     int               `json:"order"` // sites in the seeded order
	Mutants   []MutantResult    `json:"mutants"`
	Controls  []MutantResult    `json:"controls"`
	Counts    ReportCounts      `json:"counts"`
	ElapsedMS int64             `json:"elapsedMillis"`
}

// ReportIndex identifies the index a report qualified against.
type ReportIndex struct {
	Commit string `json:"commit"`
	Digest string `json:"digest"`
	Runner string `json:"runner"`
}

// ReportCounts totals a report.
type ReportCounts struct {
	Mutants         int            `json:"mutants"` // valid mutants
	Killed          int            `json:"killed"`
	Invalid         int            `json:"invalid"`
	Misses          int            `json:"misses"`
	KeptMean        float64        `json:"keptMean"`
	KeptMax         int            `json:"keptMax"`
	Controls        int            `json:"controls"`
	ControlsFailed  int            `json:"controlsFailed"`
	ByKind          map[string]int `json:"byKind"`
	ByOperator      map[string]int `json:"byOperator"`
	KilledByKind    map[string]int `json:"killedByKind"`
	SuiteTests      int            `json:"suiteTests"`
	SelectMillisMax float64        `json:"selectMillisMax"`
}

// qualify is the GLS-V0-030..033 fault-injection run on a disposable clean
// checkout at the index commit.
func qualify(args []string) error {
	fs := flag.NewFlagSet("qualify", flag.ExitOnError)
	root := fs.String("root", ".", "clean disposable checkout at the index commit")
	index := fs.String("index", "", "index file")
	only := fs.String("packages", "", "comma-separated directories whose files are mutated")
	suiteList := fs.String("suite", "", "comma-separated directories whose tests run per mutant (default -packages)")
	mutants := fs.Int("mutants", 20, "valid mutants to run from -start, when -count is not given")
	controls := fs.Int("controls", 0, "negative controls: deletions in bodies no record executed")
	seed := fs.Int64("seed", 1, "sample seed")
	start := fs.Int("start", 0, "first site of the seeded order to run")
	count := fs.Int("count", 0, "sites to run from -start (0: until -mutants valid mutants)")
	report := fs.String("report", "", "write the JSON report here")
	testTimeout := fs.Duration("test-timeout", 0, "go test -timeout of each suite run (default: the deadline the index's runner identity names)")
	execWrapper := fs.String("exec", "", "go test -exec wrapper the suite runs under, as the index's runner names it")
	summarize := fs.Bool("summarize", false, "fold the REPORT arguments into one summary")
	record := fs.String("record", "", "with -summarize: write the qualification record here")
	fs.Parse(args)
	if *summarize {
		return summarizeReports(*root, *record, fs.Args())
	}
	f, err := os.Open(*index)
	if err != nil {
		return err
	}
	ix, err := linecov.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	// Mutants run under the deadline the index was collected for, so a test
	// that reads t.Deadline() executes as recorded (GLS-V0-009); an index
	// naming none, or a zero one, cannot qualify (LPCV-V0-7603). An explicit
	// -test-timeout must name that same deadline: the report keeps the index's
	// runner identity, so runs under another deadline would qualify a runner
	// they did not reproduce.
	recorded, ok := linecov.RunnerTestTimeout(ix.Runner)
	if !ok || recorded <= 0 {
		return errors.New("qualify: missing-recorded-test-deadline")
	}
	if *testTimeout > 0 && *testTimeout != recorded {
		return fmt.Errorf("qualify: test-deadline-mismatch: -test-timeout %s, index runner records %s", *testTimeout, recorded)
	}
	*testTimeout = recorded
	dirs := split(*only)
	if len(dirs) == 0 {
		return errors.New("qualify: -packages is required")
	}
	suite := split(*suiteList)
	if len(suite) == 0 {
		suite = dirs
	}
	if head, err := output(*root, "git", "rev-parse", "HEAD"); err != nil || head != ix.Commit {
		return fmt.Errorf("qualify: root must be checked out at the index commit %s", ix.Commit)
	}
	run, err := runnerLike(*root, ix.Runner, *execWrapper)
	if err != nil {
		return err
	}
	if run != ix.Runner {
		return fmt.Errorf("qualify: index runner %q, current %q", ix.Runner, run)
	}
	// Every mutant resets the checkout hard, so local work would be lost.
	if err := clean(*root, suite); err != nil {
		return fmt.Errorf("qualify: checkout must be clean: %w", err)
	}
	controlOK, excludedControls, err := controlEligibility(*root, ix, suite)
	if err != nil {
		return err
	}
	for _, pkg := range excludedControls {
		fmt.Fprintf(os.Stderr, "linecov qualify: no controls in %s: an unrecorded test of it or of a suite importer could execute any body (GLS-V0-032)\n", pkg)
	}
	sites, controlSites, err := mutationSites(*root, ix, dirs, *seed, controlOK)
	if err != nil {
		return err
	}
	began := time.Now()
	rep := QualReport{Sources: executableSources(*root), Profile: QualificationProfile, Seed: *seed, Index: ReportIndex{Commit: ix.Commit, Digest: ix.Digest(), Runner: ix.Runner},
		Packages: dirs, Suite: suite, Start: *start, Order: len(sites), Mutants: []MutantResult{}, Controls: []MutantResult{}}
	baselineHeads, err := linecov.HeadTestNames(*root, suite)
	if err != nil {
		return err
	}
	// A clean full-suite baseline distinguishes regressions from ambient failures.
	for _, pkg := range suite {
		failed, broken, err := failingTests(*root, ix.Module, []string{pkg}, *testTimeout, *execWrapper)
		failed = declaredFailures(failed, baselineHeads)
		result := BaselineResult{Package: pkg, State: "passed", Failing: sortedKeys(failed)}
		if err != nil || len(failed) > 0 || len(broken.build) > 0 || len(broken.crash) > 0 {
			result.State = "failed"
			result.Detail = fmt.Sprintf("failed=%v build=%v crash=%v error=%v", sortedKeys(failed), sortedKeys(broken.build), sortedKeys(broken.crash), err)
			rep.Baseline = append(rep.Baseline, result)
			rep.ElapsedMS = time.Since(began).Milliseconds()
			if writeErr := writeJSON(*report, rep); writeErr != nil {
				return writeErr
			}
			return fmt.Errorf("qualify: baseline failed for %s: %s; no mutants run", pkg, result.Detail)
		}
		rep.Baseline = append(rep.Baseline, result)
	}
	git := func(args ...string) error {
		_, err := output(*root, "git", append([]string{"-c", "user.name=Russell Lewis", "-c", "user.email=me@russe11.com", "-c", "commit.gpgsign=false"}, args...)...)
		return err
	}
	defer git("reset", "-q", "--hard", ix.Commit)
	q := qualifier{root: *root, ix: ix, suite: suite, git: git, timeout: *testTimeout, exec: *execWrapper}
	heads, err := linecov.HeadTestNames(*root, suite)
	if err != nil {
		return err
	}
	for _, names := range heads {
		rep.Counts.SuiteTests += len(names)
	}
	valid := 0
	for i := *start; i < len(sites); i++ {
		if *count > 0 && i >= *start+*count || *count == 0 && valid >= *mutants {
			break
		}
		res, err := q.mutant(sites[i])
		if err != nil {
			return err
		}
		res.Index = i
		rep.Sites++
		if res.State != "invalid" {
			valid++
		}
		rep.Mutants = append(rep.Mutants, res)
		fmt.Fprintf(os.Stderr, "linecov qualify: %d %s:%d %s/%s %s kept=%d failing=%d missed=%v select=%.1fms %s\n",
			i, res.Site.Path, res.Site.Line, res.Site.Kind, res.Site.Operator, res.State, res.KeptCount, len(res.Failing), res.Missed, res.SelectMillis, res.Detail)
	}
	for i := 0; i < len(controlSites) && len(rep.Controls) < *controls; i++ {
		res, err := q.mutant(controlSites[i])
		if err != nil {
			return err
		}
		res.Index = i
		if res.State == "invalid" {
			continue // a control that does not build proves nothing; take the next
		}
		// Unrecorded tests of other packages are kept whatever the change;
		// a control fails on a test of its package, a test kept by coverage
		// anywhere, or a failure (GLS-V0-032).
		if res.KeptOfPackage > 0 || res.KeptByCoverage > 0 || len(res.Failing) > 0 {
			res.State, res.Detail = "control-failed", "a mutant on an unexecuted line kept a test of its package or by coverage, or failed a test"
		} else {
			res.State = "control-passed"
		}
		rep.Controls = append(rep.Controls, res)
		fmt.Fprintf(os.Stderr, "linecov qualify: control %s:%d %s kept=%d of-package=%d by-coverage=%d failing=%d\n", res.Site.Path, res.Site.Line, res.State, res.KeptCount, res.KeptOfPackage, res.KeptByCoverage, len(res.Failing))
	}
	rep.Counts = foldCounts(rep.Mutants, rep.Controls, rep.Counts.SuiteTests)
	rep.ElapsedMS = time.Since(began).Milliseconds()
	if err := writeJSON(*report, rep); err != nil {
		return err
	}
	fmt.Printf("sites=%d mutants=%d killed=%d invalid=%d misses=%d controls=%d controls-failed=%d mean-kept=%.1f\n",
		rep.Sites, rep.Counts.Mutants, rep.Counts.Killed, rep.Counts.Invalid, rep.Counts.Misses, rep.Counts.Controls, rep.Counts.ControlsFailed, rep.Counts.KeptMean)
	if rep.Counts.Misses > 0 || rep.Counts.ControlsFailed > 0 {
		return fmt.Errorf("%d misses, %d failed controls", rep.Counts.Misses, rep.Counts.ControlsFailed)
	}
	return nil
}

// qualifier runs one mutant at a time on the checkout.
type qualifier struct {
	root    string
	ix      *linecov.Index
	suite   []string
	git     func(args ...string) error
	timeout time.Duration
	exec    string // the go test -exec wrapper, "" for none
}

// mutant applies one site, commits it, selects for that one-line change
// and runs the whole suite (GLS-V0-031).
func (q qualifier) mutant(s Site) (MutantResult, error) {
	res := MutantResult{Site: s}
	if err := q.git("reset", "-q", "--hard", q.ix.Commit); err != nil {
		return res, err
	}
	full := filepath.Join(q.root, filepath.FromSlash(s.Path))
	src, err := os.ReadFile(full)
	if err != nil {
		return res, err
	}
	mutated := append(append(append([]byte{}, src[:s.start]...), s.repl...), src[s.end:]...)
	if err := os.WriteFile(full, mutated, 0o644); err != nil {
		return res, err
	}
	build := exec.Command("go", "build", "-o", os.DevNull, "./"+path.Dir(s.Path))
	build.Dir = q.root
	if out, err := build.CombinedOutput(); err != nil {
		res.State, res.Detail = "invalid", "does not build: "+firstLine(string(out))
		return res, nil
	}
	if err := q.git("commit", "-qam", "mutant"); err != nil {
		return res, err
	}
	head, err := output(q.root, "git", "rev-parse", "HEAD")
	if err != nil {
		return res, err
	}
	ctx := context.Background()
	diffs, err := linecov.Diff(ctx, q.root, q.ix.Commit, head)
	if err != nil {
		return res, err
	}
	read, err := linecov.BlobReader(ctx, q.root, diffs)
	if err != nil {
		return res, err
	}
	var units []linecov.Unit
	for _, d := range q.suite {
		units = append(units, linecov.Unit{ID: "go:" + d, Dir: d, Witness: s.Path})
	}
	selected := time.Now()
	sel := q.ix.Plan(linecov.PlanInput{
		Base: q.ix.Commit, Head: head, Change: diffs, ReadDrift: read, ReadChange: read, Units: units,
		Heads: func(dirs []string) (map[string][]string, error) { return linecov.HeadTestNames(q.root, dirs) },
		// Without the affected graph, a fallback path keeps every suite unit.
		StaticOf: func([]string) (bool, map[string]string, error) {
			reach := map[string]string{}
			for _, d := range q.suite {
				reach[d] = s.Path
			}
			return false, reach, nil
		},
		// The suite runs under this process's environment (GLS-V0-018).
		Environ: os.Environ(),
	})
	res.SelectMillis = linecov.Millis(time.Since(selected))
	if sel.State != linecov.StateNarrowed {
		return res, fmt.Errorf("qualify: selection %s %s: %s", sel.State, sel.Reason, sel.Detail)
	}
	heads, err := linecov.HeadTestNames(q.root, q.suite)
	if err != nil {
		return res, err
	}
	kept := map[string]bool{}
	whole := map[string]bool{}
	for _, u := range sel.Units {
		switch u.Mode {
		case linecov.Whole:
			whole[unitDir(u)] = true
			for _, n := range heads[unitDir(u)] {
				kept[unitDir(u)+"."+n] = true
			}
			res.Kept = append(res.Kept, fmt.Sprintf("%s=whole(%d)", unitDir(u), len(heads[unitDir(u)])))
			// A whole unit the change reaches (its witness is the mutated
			// path) is kept by coverage or by order; a whole unit of another
			// cause (unordered, environment) is not the mutant's doing.
			if u.Category != linecov.CategoryUnordered && u.Category != linecov.CategoryEnvironment && u.Category != linecov.CategoryStaticRead {
				res.KeptByCoverage += len(heads[unitDir(u)])
			}
		case linecov.Tests:
			for _, k := range u.Kept {
				kept[unitDir(u)+"."+k.Name] = true
				if k.Why == linecov.WhySelected || k.Why == linecov.WhyDirect || k.Why == linecov.WhyStale {
					res.KeptByCoverage++
				}
			}
			res.Kept = append(res.Kept, unitDir(u)+"=tests:"+u.Run)
		}
		if unitDir(u) == path.Dir(s.Path) {
			for _, n := range heads[unitDir(u)] {
				if kept[unitDir(u)+"."+n] {
					res.KeptOfPackage++
				}
			}
		}
	}
	res.KeptCount = len(kept)
	failed, broken, err := failingTests(q.root, q.ix.Module, q.suite, q.timeout, q.exec)
	if err != nil {
		res.State, res.Detail = "invalid", err.Error()
		return res, nil
	}
	failed = declaredFailures(failed, heads)
	if len(broken.build) > 0 {
		res.State, res.Detail = "invalid", "package build failed: "+strings.Join(sortedKeys(broken.build), ", ")
		return res, nil
	}
	// A package that failed outside every test may have failed in any of
	// them: every test of it counts as failing.
	for pkg := range broken.crash {
		for _, n := range heads[pkg] {
			failed[pkg+"."+n] = true
		}
	}
	res.Failing = sortedKeys(failed)
	for _, f := range res.Failing {
		if !kept[f] {
			res.Missed = append(res.Missed, f)
		}
	}
	res.State = "survived"
	if len(res.Failing) > 0 {
		res.State = "killed"
	}
	return res, nil
}

// controlEligibility reports, per package, whether a body no record executed
// is known unexecuted: every declared test of the package, and of every suite
// package that depends on it, has a record. An unrecorded test (divergent,
// timed out, a starter of an uninstrumented child) could execute any body, so
// a control there proves nothing (GLS-V0-032). It also lists the ineligible
// suite packages.
func controlEligibility(root string, ix *linecov.Index, suite []string) (func(pkg string) bool, []string, error) {
	heads, err := linecov.HeadTestNames(root, suite)
	if err != nil {
		return nil, nil, err
	}
	recorded := map[string]bool{}
	for _, t := range ix.Tests {
		recorded[t.Package+"\x00"+t.Name] = true
	}
	unrecorded := map[string]bool{}
	for pkg, names := range heads {
		for _, n := range names {
			if !recorded[pkg+"\x00"+n] {
				unrecorded[pkg] = true
			}
		}
	}
	args := []string{"list", "-f", "{{.ImportPath}} {{join .Deps \" \"}} {{join .TestImports \" \"}} {{join .XTestImports \" \"}}"}
	for _, d := range suite {
		args = append(args, "./"+d)
	}
	out, err := output(root, "go", args...)
	if err != nil {
		return nil, nil, fmt.Errorf("qualify: go list: %w", err)
	}
	dependsOn := map[string]map[string]bool{} // suite package -> module packages it depends on
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		pkg := strings.TrimPrefix(strings.TrimPrefix(f[0], ix.Module), "/")
		if pkg == "" {
			pkg = "."
		}
		dependsOn[pkg] = map[string]bool{}
		for _, dep := range f[1:] {
			if strings.HasPrefix(dep, ix.Module) {
				d := strings.TrimPrefix(strings.TrimPrefix(dep, ix.Module), "/")
				if d == "" {
					d = "."
				}
				dependsOn[pkg][d] = true
			}
		}
	}
	eligible := map[string]bool{}
	var excluded []string
	for _, pkg := range suite {
		ok := !unrecorded[pkg]
		for other := range dependsOn {
			if dependsOn[other][pkg] && unrecorded[other] {
				ok = false
			}
		}
		eligible[pkg] = ok
		if !ok {
			excluded = append(excluded, pkg)
		}
	}
	sort.Strings(excluded)
	return func(pkg string) bool { return eligible[pkg] }, excluded, nil
}

// mutationSites returns the seeded order of mutation sites in executed
// bodies of dirs' indexed files, kinds taken in turn, and the shuffled
// control sites in bodies no record executed of the packages controlOK
// admits.
func mutationSites(root string, ix *linecov.Index, dirs []string, seed int64, controlOK func(pkg string) bool) ([]Site, []Site, error) {
	in := map[string]bool{}
	for _, d := range dirs {
		in[d] = true
	}
	byKind := map[string][]Site{}
	var controls []Site
	for i := range ix.Files {
		file := ix.Files[i]
		if !in[path.Dir(file.Path)] || !strings.HasSuffix(file.Path, ".go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, nil, err
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file.Path, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		for _, d := range parsed.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			first, last := int32(fset.Position(fn.Pos()).Line), int32(fset.Position(fn.End()).Line)
			executed, instrumented := false, false
			for _, b := range file.Blocks {
				if b.Start >= first && b.End <= last {
					instrumented = true
					executed = executed || len(b.Tests) > 0
				}
			}
			if !instrumented {
				continue
			}
			found := funcSites(fset, src, file.Path, fn)
			if !executed {
				if controlOK != nil && !controlOK(path.Dir(file.Path)) {
					continue
				}
				for _, s := range found {
					if s.Operator == "delete" {
						controls = append(controls, s)
					}
				}
				continue
			}
			for _, s := range found {
				byKind[s.Kind] = append(byKind[s.Kind], s)
			}
		}
	}
	r := rand.New(rand.NewSource(seed))
	for _, k := range siteKinds {
		sortSites(byKind[k])
		r.Shuffle(len(byKind[k]), func(i, j int) { byKind[k][i], byKind[k][j] = byKind[k][j], byKind[k][i] })
	}
	var order []Site
	for {
		took := false
		for _, k := range siteKinds {
			if len(byKind[k]) > 0 {
				order = append(order, byKind[k][0])
				byKind[k] = byKind[k][1:]
				took = true
			}
		}
		if !took {
			break
		}
	}
	sortSites(controls)
	r.Shuffle(len(controls), func(i, j int) { controls[i], controls[j] = controls[j], controls[i] })
	return order, controls, nil
}

func sortSites(s []Site) {
	sort.Slice(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.start != b.start {
			return a.start < b.start
		}
		return a.Operator < b.Operator
	})
}

// funcSites lists the mutation sites of one function body (GLS-V0-030).
func funcSites(fset *token.FileSet, src []byte, file string, fn *ast.FuncDecl) []Site {
	var out []Site
	add := func(n ast.Node, kind, op, repl string) {
		p := fset.Position(n.Pos())
		out = append(out, Site{Path: file, Line: p.Line, Col: p.Column, Kind: kind, Operator: op,
			start: fset.Position(n.Pos()).Offset, end: fset.Position(n.End()).Offset, repl: repl})
	}
	text := func(n ast.Node) string {
		return string(src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
	}
	var stack []ast.Node
	inside := func(match func(ast.Node) bool) bool {
		for i := len(stack) - 1; i >= 0; i-- {
			if match(stack[i]) {
				return true
			}
		}
		return false
	}
	closure := func() bool {
		return inside(func(n ast.Node) bool { _, ok := n.(*ast.FuncLit); return ok })
	}
	// contextKind is the kind of a literal: in a for or case header, in a
	// closure, or a plain statement.
	contextKind := func(n ast.Node) string {
		for i := len(stack) - 1; i >= 0; i-- {
			switch p := stack[i].(type) {
			case *ast.ForStmt:
				if p.Body != nil && n.Pos() < p.Body.Lbrace {
					return "loop"
				}
				return contextOuter(stack[:i])
			case *ast.CaseClause:
				if p.Colon.IsValid() && n.Pos() < p.Colon {
					return "case"
				}
				return contextOuter(stack[:i])
			case *ast.FuncLit:
				return "closure"
			}
		}
		return "statement"
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		parent := ast.Node(nil)
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		switch x := n.(type) {
		case *ast.IfStmt:
			kind := "statement"
			if p, ok := parent.(*ast.IfStmt); ok && p.Else == x {
				kind = "else-if"
			} else if closure() {
				kind = "closure"
			}
			add(x.Cond, kind, "negate", "!("+text(x.Cond)+")")
		case *ast.ForStmt:
			if x.Cond != nil {
				add(x.Cond, "loop", "negate", "!("+text(x.Cond)+")")
			}
		case *ast.SwitchStmt:
			if x.Tag == nil {
				for _, c := range x.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						add(e, "case", "negate", "!("+text(e)+")")
					}
				}
			}
		case *ast.ExprStmt, *ast.IncDecStmt, *ast.SendStmt, *ast.GoStmt, *ast.DeferStmt:
			kind := "statement"
			if closure() {
				kind = "closure"
			}
			add(x, kind, "delete", "if false { "+text(x)+" }")
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				kind := "statement"
				if closure() {
					kind = "closure"
				}
				add(x, kind, "delete", "if false { "+text(x)+" }")
			}
		case *ast.Field:
			// A struct tag is no expression: a field with one is left alone.
			if x.Tag != nil {
				return false
			}
		case *ast.BasicLit:
			switch x.Kind {
			case token.INT, token.FLOAT:
				add(x, contextKind(x), "constant", "("+x.Value+" + 1)")
			case token.STRING:
				add(x, contextKind(x), "constant", "("+x.Value+` + "x")`)
			}
		case *ast.Ident:
			if (x.Name == "true" || x.Name == "false") && !isSelectorSel(parent, x) {
				flip := "false"
				if x.Name == "false" {
					flip = "true"
				}
				add(x, contextKind(x), "constant", flip)
			}
		}
		stack = append(stack, n)
		return true
	})
	return out
}

// contextOuter is the kind of the nodes enclosing a header the node is not
// in: a closure, or a plain statement.
func contextOuter(stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if _, ok := stack[i].(*ast.FuncLit); ok {
			return "closure"
		}
	}
	return "statement"
}

func isSelectorSel(parent ast.Node, x *ast.Ident) bool {
	s, ok := parent.(*ast.SelectorExpr)
	return ok && s.Sel == x
}

// broken holds the packages a suite run failed outside every test: build
// is a build or setup failure, crash any other.
type broken struct{ build, crash map[string]bool }

// failingTests runs the directories whole and returns failing top-level
// tests as dir.Name, and the packages that failed outside every test. A
// directory with no package result means go test did not run it, which
// proves nothing, so it is an error.
func failingTests(root, module string, dirs []string, timeout time.Duration, wrapper ...string) (map[string]bool, broken, error) {
	if timeout <= 0 {
		return nil, broken{}, errors.New("qualify: missing-recorded-test-deadline")
	}
	args := []string{"test", "-count=1", "-json", "-timeout=" + timeout.String()}
	if len(wrapper) == 1 && wrapper[0] != "" {
		args = append(args, "-exec", wrapper[0])
	}
	var out []byte
	var runErr error
	for _, d := range dirs {
		pkgArgs := append(append([]string{}, args...), "./"+d)
		cmd := exec.Command("go", pkgArgs...)
		cmd.Dir = root
		data, err := cmd.CombinedOutput()
		out = append(out, data...)
		out = append(out, '\n')
		if err != nil {
			runErr = err
		}
	}
	failed, done := map[string]bool{}, map[string]bool{}
	testFailed := map[string]bool{}
	b := broken{build: map[string]bool{}, crash: map[string]bool{}}
	buildOutput := map[string]bool{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		var ev struct{ Action, Package, Test, Output string }
		if json.Unmarshal(line, &ev) != nil || strings.Contains(ev.Test, "/") {
			continue
		}
		dir := strings.TrimPrefix(strings.TrimPrefix(ev.Package, module), "/")
		if dir == "" {
			dir = "."
		}
		if ev.Action == "build-fail" || ev.Test == "" && ev.Action == "output" && (strings.Contains(ev.Output, "[build failed]") || strings.Contains(ev.Output, "[setup failed]")) {
			buildOutput[dir] = true
		}
		if ev.Test == "" && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip") {
			done[dir] = true
		}
		if ev.Action != "fail" {
			continue
		}
		if ev.Test == "" {
			if buildOutput[dir] {
				b.build[dir] = true
			} else if !testFailed[dir] {
				b.crash[dir] = true
			}
			continue
		}
		failed[dir+"."+ev.Test] = true
		testFailed[dir] = true
	}
	for _, d := range dirs {
		if !done[d] {
			return nil, b, fmt.Errorf("qualify: go test reported no result for %s (%v)", d, runErr)
		}
	}
	return failed, b, nil
}

// declaredFailures keeps the failures of tests their package declares at
// HEAD. go test -json frames results from the test binary's output lines, so
// a test that prints another run's `--- FAIL: Name` line (the tools' own
// tests collect fixture modules) yields a result for a test the package has
// not got; only a declared test can fail, so the rest is output, not a
// failure. A failure of a declared test is never dropped.
func declaredFailures(failed map[string]bool, heads map[string][]string) map[string]bool {
	out := make(map[string]bool, len(failed))
	for key := range failed {
		if i := strings.LastIndex(key, "."); i >= 0 && slices.Contains(heads[key[:i]], key[i+1:]) {
			out[key] = true
		}
	}
	return out
}

// foldCounts totals mutants and controls.
func foldCounts(mutants, controls []MutantResult, suiteTests int) ReportCounts {
	c := ReportCounts{ByKind: map[string]int{}, ByOperator: map[string]int{}, KilledByKind: map[string]int{}, SuiteTests: suiteTests}
	kept := 0
	for _, m := range mutants {
		if m.State == "invalid" {
			c.Invalid++
			continue
		}
		c.Mutants++
		c.ByKind[m.Site.Kind]++
		c.ByOperator[m.Site.Operator]++
		kept += m.KeptCount
		c.KeptMax = max(c.KeptMax, m.KeptCount)
		c.SelectMillisMax = max(c.SelectMillisMax, m.SelectMillis)
		if m.State == "killed" {
			c.Killed++
			c.KilledByKind[m.Site.Kind]++
		}
		if len(m.Missed) > 0 {
			c.Misses++
		}
	}
	if c.Mutants > 0 {
		c.KeptMean = float64(kept) / float64(c.Mutants)
	}
	for _, r := range controls {
		c.Controls++
		if r.State != "control-passed" {
			c.ControlsFailed++
		}
	}
	return c
}

// summarizeReports folds chunked reports of one seeded order and, with
// record, writes the GLS-V0-034 qualification record of the selector and
// collector files committed at root's HEAD.
func summarizeReports(root, record string, files []string) error {
	if len(files) == 0 {
		return errors.New("qualify -summarize: name at least one report")
	}
	var all QualReport
	type span struct{ start, end int }
	var spans []span
	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var r QualReport
		if err := json.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if r.Profile != QualificationProfile {
			return fmt.Errorf("%s: profile %q", file, r.Profile)
		}
		if i == 0 {
			all = QualReport{Profile: QualificationProfile, Seed: r.Seed, Index: r.Index, Packages: r.Packages, Suite: r.Suite, Order: r.Order, Sources: r.Sources}
		} else if !reflect.DeepEqual(r.Sources, all.Sources) || r.Seed != all.Seed || r.Index != all.Index || strings.Join(r.Packages, ",") != strings.Join(all.Packages, ",") || strings.Join(r.Suite, ",") != strings.Join(all.Suite, ",") {
			return fmt.Errorf("%s: another implementation, index, seed, package set or suite than %s", file, files[0])
		}
		for _, b := range r.Baseline {
			if b.State != "passed" {
				return fmt.Errorf("%s: baseline did not pass: %s", file, b.Package)
			}
		}
		spans = append(spans, span{r.Start, r.Start + r.Sites})
		all.Sites += r.Sites
		all.Mutants = append(all.Mutants, r.Mutants...)
		all.Controls = append(all.Controls, r.Controls...)
		all.ElapsedMS += r.ElapsedMS
		all.Counts.SuiteTests = r.Counts.SuiteTests
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			return fmt.Errorf("qualify -summarize: site ranges [%d,%d) and [%d,%d) overlap", spans[i-1].start, spans[i-1].end, spans[i].start, spans[i].end)
		}
	}
	all.Counts = foldCounts(all.Mutants, all.Controls, all.Counts.SuiteTests)
	summary, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(summary))
	c := all.Counts
	if c.Misses > 0 || c.ControlsFailed > 0 {
		return fmt.Errorf("%d misses, %d failed controls", c.Misses, c.ControlsFailed)
	}
	if record == "" {
		return nil
	}
	if c.Mutants == 0 || c.Killed == 0 || c.Controls == 0 {
		return errors.New("qualify -summarize: a record needs at least one killed mutant and one passing control")
	}
	sum := sha256.Sum256(summary)
	sources := map[string]string{}
	for _, p := range qualifiedSources {
		oid, err := output(root, "git", "rev-parse", "HEAD:"+p)
		if err != nil {
			return fmt.Errorf("qualify -summarize: %s is not committed at HEAD: %w", p, err)
		}
		if dirty, _ := output(root, "git", "status", "--porcelain", "--", p); dirty != "" {
			return fmt.Errorf("qualify -summarize: %s has uncommitted changes", p)
		}
		if all.Sources[p] != oid {
			return fmt.Errorf("qualify -summarize: %s was qualified at blob %q, HEAD holds %s; rerun qualification with a clean VCS-stamped executable", p, all.Sources[p], oid)
		}
		sources[p] = oid
	}
	return writeJSON(record, Qualified{
		Profile: QualifiedProfile, Family: "go", Runner: all.Index.Runner, IndexCommit: all.Index.Commit,
		Report: QualifiedReport{Digest: "sha256:" + hex.EncodeToString(sum[:]), Seed: all.Seed, Sites: all.Sites, Mutants: c.Mutants,
			Killed: c.Killed, Invalid: c.Invalid, Controls: c.Controls, Misses: c.Misses, Packages: all.Packages, Suite: all.Suite},
		Sources: sources,
	})
}

// Qualified is the qualification record (GLS-V0-034).
type Qualified = linecov.Qualified
type QualifiedReport = linecov.QualifiedReport

// qualified checks that a qualification record holds for the selector and
// collector files at rev (GLS-V0-034, GLS-V0-043): exit 0 only when every
// qualified source has the recorded blob at rev and no source is missing.
func qualified(args []string) error {
	fs := flag.NewFlagSet("qualified", flag.ExitOnError)
	root := fs.String("root", ".", "repository")
	execWrapper := fs.String("exec", "", "go test -exec wrapper the runs the record is checked for use")
	file := fs.String("record", ".github/linecov/qualification.json", "qualification record, relative to -root unless absolute")
	rev := fs.String("rev", "HEAD", "commit whose sources must match")
	fs.Parse(args)
	name := *file
	if !filepath.IsAbs(name) {
		name = filepath.Join(*root, name)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("qualified: no qualification record: %w", err)
	}
	var q Qualified
	if err := json.Unmarshal(data, &q); err != nil {
		return fmt.Errorf("qualified: %w", err)
	}
	if q.Profile != QualifiedProfile || q.Family != "go" || q.Report.Misses != 0 || q.Report.Mutants == 0 || q.Report.Killed == 0 || q.Report.Controls == 0 {
		return errors.New("qualified: the record is not a passing go qualification")
	}
	current, err := runnerLike(*root, q.Runner, *execWrapper)
	if err != nil || current != q.Runner {
		return fmt.Errorf("qualified: runner mismatch: record %q, current %q (%v)", q.Runner, current, err)
	}
	for _, p := range qualifiedSources {
		oid, err := output(*root, "git", "rev-parse", *rev+":"+p)
		if err != nil {
			return fmt.Errorf("qualified: %s absent at %s", p, *rev)
		}
		if q.Sources[p] != oid {
			return fmt.Errorf("qualified: %s at %s is blob %s, the qualification qualified %q", p, *rev, oid, q.Sources[p])
		}
	}
	fmt.Printf("qualified: %d sources match the record (report %s, %d mutants, %d killed, 0 misses)\n", len(qualifiedSources), q.Report.Digest, q.Report.Mutants, q.Report.Killed)
	return nil
}

func writeJSON(file string, v any) error {
	if file == "" {
		return nil
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// unitDir is the module-relative directory of a qualify unit, whose id is
// "go:" and the directory.
func unitDir(u linecov.UnitSelection) string { return strings.TrimPrefix(u.Unit, "go:") }

// executableSources binds the report to the clean revision Go stamped into
// the executable, rather than whichever sources are present at summarization.
// Tests and builds without VCS provenance can still run experiments; their
// reports cannot be promoted to a qualification record.
func executableSources(root string) map[string]string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	rev, clean := "", false
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			rev = s.Value
		}
		if s.Key == "vcs.modified" {
			clean = s.Value == "false"
		}
	}
	if rev == "" || !clean {
		return nil
	}
	sources := map[string]string{}
	for _, p := range qualifiedSources {
		oid, err := output(root, "git", "rev-parse", rev+":"+p)
		if err != nil {
			return nil
		}
		sources[p] = oid
	}
	return sources
}
