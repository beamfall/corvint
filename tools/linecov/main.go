// Command linecov collects a per-test Go coverage index at one commit and
// selects, from it, the tests a change can affect (LPCV-V0-950..963).
//
//	linecov collect -root R -out INDEX [-packages a,b] [-parallel N] [-timeout D] [-suite-timeout D] [-tests RE] [-inorder run|only|skip] [-prior INDEX] [-misses FILE]
//	linecov select  -root R -index INDEX -base SHA [-static a,b]
//	linecov qualify -root R -index INDEX -packages a,b [-suite c,d] [-mutants N | -start K -count N] [-controls N] [-seed S] [-report FILE]
//	linecov qualify -summarize [-root R -record FILE] REPORT...
//	linecov qualified -root R -rev REV [-record FILE]
//	linecov redundancy -root R -index INDEX [-packages a,b] [-low-unique N]
//	linecov merge -out INDEX PART...
//	linecov ci -root R -index INDEX -base SHA -packages FILE -slices FILE -out DIR
//	linecov ci -selection FILE -packages FILE -slices FILE -out DIR
//	linecov miss -failed FAILED.json... -run DIR...
//
// collect needs a clean checkout; the index is bound to its HEAD and written
// as linecov-index/4 (GLS-V0-001). -prior carries the previous index's
// divergent shared-step classes, and -misses (a JSON list of linecov.Miss)
// adds new ones, each kept while its reach blobs are unchanged (MCJ-V0-018).
// select reads
// the change base..HEAD of a checkout at the change head and prints one JSON
// plan: a non-empty reason means every static package runs whole. redundancy
// reads a checkout at the index commit and prints the tests whose coverage
// other tests repeat, as candidates for review (LPCV-V0-964). merge unions
// indexes a sharded collection wrote. ci narrows one CI shard's whole packages
// to the tests the change can reach and leaves its slices as they are.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/watch"
)

func main() {
	if len(os.Args) < 2 {
		_ = help(os.Stderr, nil)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "help", "--help", "-h":
		err = help(os.Stdout, os.Args[2:])
	case "watch":
		err = watchCommand(os.Args[2:])
	case "ruby-collect", "js-collect":
		err = languageCollect(strings.TrimSuffix(os.Args[1], "-collect"), os.Args[2:])
	case "ruby-select", "js-select":
		err = languageSelect(strings.TrimSuffix(os.Args[1], "-select"), os.Args[2:])
	case "collect":
		err = collect(os.Args[2:])
	case "select":
		err = selectTests(os.Args[2:])
	case "qualify":
		err = qualify(os.Args[2:])
	case "qualified":
		err = qualified(os.Args[2:])
	case "redundancy":
		err = redundancy(os.Args[2:])
	case "merge":
		err = merge(os.Args[2:])
	case "ci":
		err = ci(os.Args[2:])
	case "miss":
		err = miss(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if errors.Is(err, flag.ErrHelp) {
		return
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

// runner is the index runner identity of root's environment under the test
// runtime of the go test -timeout timeout, and its effective GOFLAGS
// (linecov.RunnerID).
func runner(root string, timeout time.Duration, wrapper string) (id, goflags string, err error) {
	return linecov.RunnerID(context.Background(), root, nil, timeout, wrapper)
}

// runnerLike is the current runner identity of root under the test runtime
// the identity want names, for a check of an index or a record made under
// want that compares everything else.
func runnerLike(root, want, wrapper string) (string, error) {
	timeout, ok := linecov.RunnerTestTimeout(want)
	if !ok {
		return "", fmt.Errorf("runner %q names no test runtime; recollect the index", want)
	}
	id, _, err := runner(root, timeout, wrapper)
	return id, err
}

// clean refuses a checkout with tracked changes, or with an untracked Go file
// in any of dirs or any module package they or their tests depend on: go test
// compiles untracked files too, so either makes what runs differ from HEAD.
func clean(root string, dirs []string) error {
	dirty, err := output(root, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if dirty != "" {
		return errors.New("checkout has uncommitted changes")
	}
	if len(dirs) == 0 {
		return nil
	}
	module, err := output(root, "go", "list", "-m")
	if err != nil {
		return err
	}
	list := []string{"list", "-deps", "-test", "-f", "{{if .Module}}{{if .Module.Main}}{{.ImportPath}}{{end}}{{end}}"}
	for _, d := range dirs {
		list = append(list, "./"+d)
	}
	deps, err := output(root, "go", list...)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	specs := []string{"ls-files", "--others", "--"}
	for _, p := range strings.Fields(deps) {
		// -test also lists the test variants "p [p.test]" and "p.test".
		if strings.HasSuffix(p, ".test") || strings.HasPrefix(p, "[") {
			continue
		}
		d := "."
		if p != module {
			d = strings.TrimPrefix(p, module+"/")
		}
		if !seen[d] {
			seen[d] = true
			specs = append(specs, ":(glob)"+path.Join(d, "*.go"))
		}
	}
	untracked, err := output(root, "git", specs...)
	if err != nil {
		return err
	}
	if untracked != "" {
		return fmt.Errorf("untracked Go files: %s", strings.Join(strings.Fields(untracked), ", "))
	}
	return nil
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
func modulePath(root string) (string, error) { return linecov.ModulePath(root) }

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
	timeout := fs.Duration("timeout", 10*time.Minute, "per isolated test process")
	suite := fs.Duration("suite-timeout", time.Hour, "per package in-order suite run")
	testTimeout := fs.Duration("test-timeout", linecov.DefaultTestTimeout, "go test -timeout of the runs the index selects for, which every test process gets as its deadline")
	execWrapper := fs.String("exec", "", "go test -exec wrapper every test process runs under, as the selected runs do")
	scratch := fs.String("work", "", "scratch directory parent (default the system temp directory)")
	testsRE := fs.String("tests", "", "collect only the isolated tests whose names match this regexp")
	inOrder := fs.String("inorder", "run", "the in-order suite run: run, only (no isolated test) or skip")
	priorFile := fs.String("prior", "", "previous index whose divergent classes carry forward")
	missFile := fs.String("misses", "", "JSON list of shadow misses to mark divergent")
	fs.Parse(args)
	if *out == "" {
		return errors.New("collect: -out is required")
	}
	var filter *regexp.Regexp
	if *testsRE != "" {
		re, err := regexp.Compile(*testsRE)
		if err != nil {
			return fmt.Errorf("collect: -tests: %w", err)
		}
		filter = re
	}
	mode := map[string]string{"run": linecov.InOrderRun, "only": linecov.InOrderOnly, "skip": linecov.InOrderSkip}
	inOrderMode, ok := mode[*inOrder]
	if !ok {
		return fmt.Errorf("collect: -inorder must be run, only or skip, got %q", *inOrder)
	}
	prior, misses, err := divergenceInputs(*priorFile, *missFile)
	if err != nil {
		return fmt.Errorf("collect: %w", err)
	}
	commit, err := output(*root, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	run, goflags, err := runner(*root, *testTimeout, *execWrapper)
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
	if err := clean(*root, dirs); err != nil {
		return fmt.Errorf("collect: checkout must be clean: %w", err)
	}
	work, err := os.MkdirTemp(*scratch, "linecov-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	ctx := context.Background()
	start := time.Now()
	outcomes, err := linecov.Collect(ctx, *root, linecov.CollectOptions{Module: module, Packages: dirs, Work: work, Parallel: *parallel, Timeout: *timeout, SuiteTimeout: *suite, TestTimeout: *testTimeout, GOFLAGS: goflags, Tests: filter, InOrder: inOrderMode, Exec: *execWrapper})
	if err != nil {
		return err
	}
	var records []linecov.Record
	var unordered, ran []string
	divergent := 0
	for _, o := range outcomes {
		if o.Record != nil && o.Name == linecov.InOrder {
			ran = append(ran, o.Package)
		}
		if o.Record == nil {
			divergent++
			fmt.Fprintf(os.Stderr, "linecov: isolation-divergent %s.%s: %s\n", o.Package, o.Name, o.Divergent)
			if o.Name == linecov.InOrder {
				unordered = append(unordered, o.Package)
			}
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
	ix.TestTimeout, ix.SuiteTimeout = *timeout, *suite
	ix.WindowFallback = map[string]string{}
	for _, o := range outcomes {
		if o.WindowFallback != "" {
			ix.WindowFallback[o.Package] = o.WindowFallback
		}
	}
	sort.Strings(unordered)
	ix.Unordered = unordered
	sort.Strings(ran)
	ix.InOrderRan = ran
	if inOrderMode == linecov.InOrderSkip {
		// Every collected package: Merge marks it unordered unless another
		// part ran its in-order suite.
		ix.InOrderSkipped = append([]string(nil), dirs...)
		sort.Strings(ix.InOrderSkipped)
	}
	if err := linecov.CarryDivergent(ix, prior, misses); err != nil {
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

// divergenceInputs reads the prior index's divergent classes and the new
// misses; either file may be unnamed.
func divergenceInputs(priorFile, missFile string) ([]linecov.Divergence, []linecov.Miss, error) {
	var prior []linecov.Divergence
	if priorFile != "" {
		f, err := os.Open(priorFile)
		if err != nil {
			return nil, nil, err
		}
		ix, err := linecov.Decode(f)
		f.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("prior %s: %w", priorFile, err)
		}
		prior = ix.Divergent
	}
	var misses []linecov.Miss
	if missFile != "" {
		data, err := os.ReadFile(missFile)
		if err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(data, &misses); err != nil {
			return nil, nil, fmt.Errorf("misses %s: %w", missFile, err)
		}
	}
	return prior, misses, nil
}

// Plan is the select output: one decision per static package and, for each
// package that runs, its `go test` arguments.
type Plan = watch.Plan

// Invocation is one `go test` package argument and its -run filter.
type Invocation = watch.Invocation

func selectTests(args []string) error {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	root := fs.String("root", ".", "checkout at the change head")
	index := fs.String("index", "", "index file")
	base := fs.String("base", "", "change base commit")
	static := fs.String("static", "", "comma-separated statically selected directories (default: every package)")
	testTimeout := fs.Duration("test-timeout", linecov.DefaultTestTimeout, "go test -timeout the selected tests run under")
	execWrapper := fs.String("exec", "", "go test -exec wrapper the selected tests run under")
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
	p, err := decide(ctx, *root, ix, *base, split(*static), *testTimeout, *execWrapper)
	if err != nil {
		return err
	}
	p.ElapsedMS = time.Since(start).Milliseconds()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

// decide plans base..HEAD of the checkout at root for the static directories,
// every package when static is empty, for tests run under the go test
// -timeout timeout and the -exec wrapper exec ("" for none).
func decide(ctx context.Context, root string, ix *linecov.Index, base string, static []string, timeout time.Duration, wrapper string) (Plan, error) {
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
	run, _, err := runner(root, timeout, wrapper)
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
	// The tests run under this process's environment (GLS-V0-018).
	p.Packages, reason = ix.Narrow(os.Environ(), dirs, heads, drift, change)
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

// Report is the redundancy output.
type Report struct {
	Index    string
	Summary  Summary
	Packages []linecov.PackageReport
}

// Summary totals the report. Elapsed figures are zero, and ElapsedObserved
// false, for an index collected before run times were recorded.
type Summary struct {
	Packages, Tests, Candidates, Unindexed, InOrderExcluded int
	ByKind                                                  map[linecov.Kind]int
	ElapsedObserved                                         bool
	TotalElapsed, CandidateElapsed                          time.Duration
	TopPackages                                             []PackageTime // by candidate time, then count
}

// PackageTime is one package's candidate count and isolated run time.
type PackageTime struct {
	Package    string
	Candidates int
	Elapsed    time.Duration
}

func redundancy(args []string) error {
	fs := flag.NewFlagSet("redundancy", flag.ExitOnError)
	root := fs.String("root", ".", "clean checkout at the index commit")
	index := fs.String("index", "", "index file")
	only := fs.String("packages", "", "comma-separated module-relative directories (default: every indexed package)")
	low := fs.Int("low-unique", 1, "list a test with at most this many blocks no other test of its package executes")
	top := fs.Int("top", 20, "packages listed in Summary.TopPackages")
	reqs := fs.String("requirements", "docs/specs/REQUIREMENTS.tsv", "root-relative table whose first column lists the known requirement IDs; when present, a cited ID it lacks is dropped")
	fs.Parse(args)
	if *index == "" {
		return errors.New("redundancy: -index is required")
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
	// Declarations are read from the tree, so it must be the indexed one.
	head, err := output(*root, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != ix.Commit {
		return fmt.Errorf("redundancy: checkout is at %s, index at %s", head, ix.Commit)
	}
	dirs := split(*only)
	if dirs == nil {
		seen := map[string]bool{}
		for _, t := range ix.Tests {
			if !seen[t.Package] {
				seen[t.Package] = true
				dirs = append(dirs, t.Package)
			}
		}
	}
	if err := clean(*root, dirs); err != nil {
		return fmt.Errorf("redundancy: checkout must be clean: %w", err)
	}
	declared, err := linecov.HeadTests(*root, dirs)
	if err != nil {
		return err
	}
	sources := map[string]map[string]linecov.Source{}
	for _, d := range dirs {
		if sources[d], err = linecov.TestSources(*root, d); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if known, err := knownRequirements(filepath.Join(*root, filepath.FromSlash(*reqs))); err != nil {
		return err
	} else if known != nil {
		for _, srcs := range sources {
			for name, src := range srcs {
				kept := src.Requirements[:0:0]
				for _, id := range src.Requirements {
					if known[id] {
						kept = append(kept, id)
					}
				}
				src.Requirements = kept
				srcs[name] = src
			}
		}
	}
	want := map[string]bool{}
	for _, d := range dirs {
		want[d] = true
	}
	r := Report{Index: ix.Commit, Summary: Summary{ByKind: map[linecov.Kind]int{}}}
	for _, t := range ix.Tests {
		if want[t.Package] && t.Name != linecov.InOrder {
			r.Summary.Tests++
			r.Summary.TotalElapsed += t.Elapsed
			if t.Elapsed > 0 {
				r.Summary.ElapsedObserved = true
			}
		}
	}
	var times []PackageTime
	for _, p := range ix.Redundancy(declared, sources, *low) {
		if !want[p.Package] {
			continue
		}
		r.Packages = append(r.Packages, p)
		pt := PackageTime{Package: p.Package, Candidates: len(p.Candidates)}
		for _, c := range p.Candidates {
			r.Summary.ByKind[c.Kind]++
			pt.Elapsed += c.Elapsed
		}
		r.Summary.Candidates += pt.Candidates
		r.Summary.CandidateElapsed += pt.Elapsed
		r.Summary.Unindexed += len(p.Unindexed)
		if p.InOrder {
			r.Summary.InOrderExcluded++
		}
		if pt.Candidates > 0 {
			times = append(times, pt)
		}
	}
	r.Summary.Packages = len(dirs)
	sort.Slice(times, func(i, j int) bool {
		a, b := times[i], times[j]
		if a.Elapsed != b.Elapsed {
			return a.Elapsed > b.Elapsed
		}
		if a.Candidates != b.Candidates {
			return a.Candidates > b.Candidates
		}
		return a.Package < b.Package
	})
	if len(times) > *top {
		times = times[:*top]
	}
	r.Summary.TopPackages = times
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// knownRequirements reads the first column of a requirements table; a missing
// table is nil, so every cited ID is kept.
func knownRequirements(file string) (map[string]bool, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if id, _, _ := strings.Cut(line, "\t"); id != "" {
			known[id] = true
		}
	}
	return known, nil
}

func readIndex(file string) (*linecov.Index, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return linecov.Decode(f)
}

func writeIndex(file string, ix *linecov.Index) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if err = ix.Encode(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// merge unions the indexes a sharded collection wrote at one commit.
func merge(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	out := fs.String("out", "", "index file to write")
	fs.Parse(args)
	if *out == "" || fs.NArg() == 0 {
		return errors.New("merge: -out and at least one part are required")
	}
	var parts []*linecov.Index
	for _, file := range fs.Args() {
		ix, err := readIndex(file)
		if err != nil {
			return fmt.Errorf("merge: %s: %w", file, err)
		}
		parts = append(parts, ix)
	}
	ix, err := linecov.Merge(parts...)
	if err != nil {
		return err
	}
	return writeIndex(*out, ix)
}

// ci narrows one CI shard. packages lists the import paths it runs whole and
// slices its "FLAG PATTERN IMPORTPATH" invocations of split packages. Each
// whole package becomes whole, a -run slice of the tests the change can
// reach, a "-run ^$" compile-only slice, or nothing; slices pass through, so
// a split package keeps its slice. A plan with a reason writes both lists
// unchanged. out receives packages, slices and plan.json.
func ci(args []string) error {
	fs := flag.NewFlagSet("ci", flag.ExitOnError)
	root := fs.String("root", ".", "checkout at the tested commit")
	index := fs.String("index", "", "index file")
	base := fs.String("base", "", "change base commit")
	pkgFile := fs.String("packages", "", "file of whole-package import paths")
	sliceFile := fs.String("slices", "", "file of FLAG PATTERN IMPORTPATH slices")
	out := fs.String("out", "", "output directory")
	selection := fs.String("selection", "", "corvint affected --line-index receipt (or its lineSelection) to narrow by, instead of -index and -base")
	testTimeout := fs.Duration("test-timeout", linecov.DefaultTestTimeout, "go test -timeout the selected tests run under")
	execWrapper := fs.String("exec", "", "go test -exec wrapper the selected tests run under")
	fs.Parse(args)
	if *selection != "" {
		if *pkgFile == "" || *sliceFile == "" || *out == "" {
			return errors.New("ci: -selection needs -packages, -slices and -out")
		}
		whole, err := lines(*pkgFile)
		if err != nil {
			return err
		}
		slices, err := lines(*sliceFile)
		if err != nil {
			return err
		}
		return ciSelection(*selection, whole, slices, *out)
	}
	if *index == "" || *base == "" || *pkgFile == "" || *sliceFile == "" || *out == "" {
		return errors.New("ci: -index, -base, -packages, -slices and -out are required")
	}
	whole, err := lines(*pkgFile)
	if err != nil {
		return err
	}
	slices, err := lines(*sliceFile)
	if err != nil {
		return err
	}
	start := time.Now()
	ix, err := readIndex(*index)
	if err != nil {
		return err
	}
	module, err := modulePath(*root)
	if err != nil {
		return err
	}
	split := map[string]bool{}
	for _, s := range slices {
		if f := strings.Fields(s); len(f) == 3 {
			split[f[2]] = true
		} else {
			return fmt.Errorf("ci: malformed slice %q", s)
		}
	}
	var dirs []string
	for _, p := range whole {
		switch {
		case split[p]:
		case p == module:
			dirs = append(dirs, ".")
		case strings.HasPrefix(p, module+"/"):
			dirs = append(dirs, strings.TrimPrefix(p, module+"/"))
		}
	}
	if dirs == nil {
		dirs = []string{} // an empty static plan, not every package
	}
	p, err := decide(context.Background(), *root, ix, *base, dirs, *testTimeout, *execWrapper)
	if err != nil {
		return err
	}
	p.ElapsedMS = time.Since(start).Milliseconds()
	keepWhole, keepSlices := whole, slices
	if p.Reason == "" {
		keepWhole, keepSlices = nil, nil
		for _, pkg := range whole {
			d := strings.TrimPrefix(strings.TrimPrefix(pkg, module), "/")
			if d == "" {
				d = "."
			}
			pp, ok := p.Packages[d]
			switch {
			case split[pkg] || !ok || pp.Mode == linecov.Whole:
				keepWhole = append(keepWhole, pkg)
			case pp.Mode == linecov.Tests:
				keepSlices = append(keepSlices, "-run ^("+strings.Join(pp.Tests, "|")+")$ "+pkg)
			case pp.Mode == linecov.Compile:
				keepSlices = append(keepSlices, "-run ^$ "+pkg)
			}
		}
		keepSlices = append(keepSlices, slices...)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	plan, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	for name, body := range map[string]string{
		"packages":  joinLines(keepWhole),
		"slices":    joinLines(keepSlices),
		"plan.json": string(plan) + "\n",
	} {
		if err := os.WriteFile(filepath.Join(*out, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// lines reads file's non-empty lines.
func lines(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

func joinLines(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return strings.Join(list, "\n") + "\n"
}
