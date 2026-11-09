package linecov

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// CollectOptions configures one collection run (LPCV-V0-950, LPCV-V0-954).
type CollectOptions struct {
	Module   string
	Packages []string // module-relative directories
	Work     string   // scratch directory for binaries and coverage data
	Parallel int
	Timeout  time.Duration // per isolated test process
	// SuiteTimeout bounds each package's in-order run of its whole suite.
	SuiteTimeout time.Duration
	TestArgs     []string // extra -test.* flags, matching how CI runs the suite
	// TestTimeout is the go test -timeout of the runs the index selects for;
	// every test process gets its TestRuntime arguments (GLS-V0-020).
	TestTimeout time.Duration
	// GOFLAGS is the effective `go env GOFLAGS` the index's runner names, so
	// flags saved with `go env -w` apply to collection as they do to selection.
	GOFLAGS string
	// Exec is the `go test -exec` wrapper every test process runs under, as
	// `Exec BINARY ARGS...` from the package directory, "" for none; the
	// runner identity names it (GLS-V0-009).
	Exec string
	// Tests, when set, keeps only the isolated tests whose names it matches,
	// and InOrder is InOrderRun (the default), InOrderOnly or InOrderSkip, so a
	// package too slow for one bounded run is collected in parts that Merge
	// joins (GLS-V0-008).
	Tests   *regexp.Regexp
	InOrder string
}

// The CollectOptions.InOrder modes.
const (
	InOrderRun  = ""
	InOrderOnly = "only"
	InOrderSkip = "skip"
)

// Outcome is one collected test: a Record when the isolated run passed,
// otherwise the reason it yields none (LPCV-V0-962).
type Outcome struct {
	Package, Name  string
	Record         *Record
	WindowFallback string // named reason the in-order union is retained
	Divergent      string // nonzero exit or timeout when run alone
}

var listedTest = regexp.MustCompile(`^(Test|Example|Fuzz)\w*$`)

// Collect runs every top-level test of each package alone, in its own
// process, with coverage over the whole module, and each package's whole
// suite once in ordinary order as its InOrder record. GOFLAGS carries the coverage
// flags, so a binary a test builds and executes also writes its blocks into
// the same GOCOVERDIR (LPCV-V0-954). At most four packages are in flight,
// each built, listed and run, its test processes bounded together by
// Parallel, and its binary deleted once its runs end, so scratch holds at
// most four binaries (GLS-V0-008).
func Collect(ctx context.Context, root string, o CollectOptions) ([]Outcome, error) {
	if err := checkGoFlags(o.GOFLAGS); err != nil {
		return nil, err
	}
	if o.Parallel < 1 {
		o.Parallel = 1
	}
	// Overlay keys and the go command's working directory must use the same
	// absolute physical path (macOS /var is /private/var). Go also preserves
	// a valid symlinked PWD, so pin the build's PWD to this path below.
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	// The runner's GOFLAGS (build tags among them) still apply.
	flags := strings.TrimSpace(o.GOFLAGS + " -cover -covermode=atomic -coverpkg=" + o.Module + "/...")
	// Clipped, so each worker's append below copies instead of sharing storage.
	env := slices.Clip(append(os.Environ(), "GOFLAGS="+flags))
	slots := make(chan struct{}, o.Parallel) // test processes in flight
	results := make([][]Outcome, len(o.Packages))
	errs := make([]error, len(o.Packages))
	var seq int64 // scratch name of each run
	var seqMu sync.Mutex
	nextScratch := func() string {
		seqMu.Lock()
		defer seqMu.Unlock()
		seq++
		return filepath.Join(o.Work, fmt.Sprintf("c%d", seq))
	}
	covdata := func(dir string) ([]byte, error) {
		p := filepath.Join(dir, "profile.txt")
		cmd := exec.CommandContext(ctx, "go", "tool", "covdata", "textfmt", "-i="+dir, "-o="+p)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("covdata: %v: %s", err, out)
		}
		return os.ReadFile(p)
	}
	runPackage := func(i int) ([]Outcome, error) {
		pkg := o.Packages[i]
		bin := filepath.Join(o.Work, fmt.Sprintf("p%d.test", i))
		defer os.Remove(bin)
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		// The window overlay wraps each top-level test so the in-order run
		// attributes its blocks per test (LPCV-V0-7600); the collector overlay
		// audits every process a test starts (LPCV-V0-954). Both are scratch
		// copies; the checkout is never edited.
		overlayWork := filepath.Join(o.Work, fmt.Sprintf("overlay%d", i))
		defer os.RemoveAll(overlayWork)
		if err := os.MkdirAll(overlayWork, 0o700); err != nil {
			return nil, err
		}
		overlay, why, err := windowOverlay(dir, overlayWork)
		if err != nil {
			return nil, err
		}
		overlay, err = collectorOverlay(overlayWork, overlay, o.Module)
		if err != nil {
			return nil, err
		}
		buildArgs := []string{"test", "-c", "-o", bin}
		windowsDir := ""
		if overlay != "" {
			buildArgs = append(buildArgs, "-overlay="+overlay)
			if why == "" {
				windowsDir = filepath.Join(o.Work, fmt.Sprintf("windows%d", i))
				defer os.RemoveAll(windowsDir)
				if err := os.MkdirAll(windowsDir, 0o700); err != nil {
					return nil, err
				}
			}
		}
		build := exec.CommandContext(ctx, "go", append(buildArgs, "./"+pkg)...)
		build.Dir, build.Env = root, append(slices.Clone(env), "PWD="+root)
		if out, err := build.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("linecov: build %s: %v: %s", pkg, err, out)
		}
		if _, err := os.Stat(bin); err != nil {
			return nil, nil // no test files
		}
		list := exec.CommandContext(ctx, bin, "-test.list", ".*")
		list.Dir, list.Env = dir, env
		out, err := list.Output()
		if err != nil {
			return nil, fmt.Errorf("linecov: list %s: %v", pkg, err)
		}
		var names []string
		declares := false
		for _, name := range strings.Fields(string(out)) {
			if !listedTest.MatchString(name) {
				continue
			}
			declares = true
			if o.InOrder != InOrderOnly && (o.Tests == nil || o.Tests.MatchString(name)) {
				names = append(names, name)
			}
		}
		// Every test binary gets a suite record, also one whose package
		// declares no named test: its TestMain, init and package variables
		// still run, and without a record of them a change they reach could
		// exclude the package (GLS-V0-008).
		if o.InOrder != InOrderSkip {
			names = append(names, InOrder)
		}
		// A test that builds and runs a binary builds it under the same
		// overlay, so the audit sees every child it starts.
		jobEnv := slices.Clip(append(slices.Clone(env), "GOFLAGS="+flags+" -overlay="+overlay))
		outcomes := make([]Outcome, len(names))
		var wg sync.WaitGroup
		var fullMu sync.Mutex
		seenBlocks := make(map[string]bool)
		for j, name := range names {
			slots <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() { <-slots; wg.Done() }()
				runEnv := jobEnv
				if name == InOrder && windowsDir != "" {
					runEnv = append(slices.Clone(jobEnv), "CORVINT_LINECOV_WINDOWS="+windowsDir)
				}
				out := runOne(ctx, o, runEnv, pkg, name, bin, dir, nextScratch(), !declares)
				// Retain each instrumented block once, plus every hit. A
				// different test may start additional instrumented binaries.
				if out.Record != nil {
					fullMu.Lock()
					out.Record.Profile = compactProfile(out.Record.Profile, seenBlocks)
					fullMu.Unlock()
				}
				outcomes[j] = out
			}()
		}
		wg.Wait()
		for j := range outcomes {
			if outcomes[j].Name == InOrder {
				outcomes[j].WindowFallback = why
			}
		}
		if windowsDir != "" {
			outcomes = attributeWindows(covdata, outcomes, map[string]string{pkg: windowsDir})
		}
		return outcomes, nil
	}
	var pwg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < min(o.Parallel, 4, len(o.Packages)); w++ {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			for i := range next {
				results[i], errs[i] = runPackage(i)
			}
		}()
	}
	for i := range o.Packages {
		next <- i
	}
	close(next)
	pwg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var outcomes []Outcome
	for i := range o.Packages {
		if errs[i] != nil {
			return nil, errs[i]
		}
		outcomes = append(outcomes, results[i]...)
	}
	sort.Slice(outcomes, func(i, j int) bool {
		a, b := outcomes[i], outcomes[j]
		return a.Package < b.Package || a.Package == b.Package && a.Name < b.Name
	})
	return outcomes, nil
}

func runOne(ctx context.Context, o CollectOptions, env []string, pkg, name, bin, dir, cov string, suiteOnly bool) Outcome {
	out := Outcome{Package: pkg, Name: name}
	// Each process's counters and meta-data are read into the Record and
	// dropped, so scratch space stays bounded by Parallel, not by test count.
	defer os.RemoveAll(cov)
	if err := os.MkdirAll(cov, 0o755); err != nil {
		out.Divergent = err.Error()
		return out
	}
	runCtx, limit := ctx, o.Timeout
	if name == InOrder {
		limit = o.SuiteTimeout
	}
	if limit > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, limit)
		defer cancel()
	}
	// The test binary writes its own profile at exit but ignores counters of
	// binaries it executed, so those go to children, converted only when one
	// wrote any: most tests then cost no covdata process.
	profile, children := filepath.Join(cov, "profile.txt"), filepath.Join(cov, "children")
	if err := os.MkdirAll(children, 0o755); err != nil {
		out.Divergent = err.Error()
		return out
	}
	args := append(TestRuntime(o.TestTimeout), "-test.count=1", "-test.gocoverdir="+cov, "-test.coverprofile="+profile)
	if name != InOrder {
		args = append(args, "-test.run", "^"+name+"$")
	}
	args = append(args, o.TestArgs...)
	cmd := exec.CommandContext(runCtx, bin, args...)
	if o.Exec != "" {
		cmd = exec.CommandContext(runCtx, o.Exec, append([]string{bin}, args...)...)
	}
	audit, envAudit := filepath.Join(cov, "exec-audit"), filepath.Join(cov, "env-audit")
	cmd.Dir, cmd.Env = dir, append(env, "GOCOVERDIR="+children, "CORVINT_LINECOV_AUDIT="+audit, "CORVINT_LINECOV_ENVAUDIT="+envAudit)
	isolate(cmd)
	cmd.WaitDelay = time.Second
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	began := time.Now()
	runErr := cmd.Run()
	unfinished := cleanup(cmd)
	if err := runErr; err != nil {
		if runCtx.Err() != nil {
			out.Divergent = "timeout"
		} else {
			out.Divergent = fmt.Sprintf("%v: %s", err, quoteOutput(log.String()))
		}
		return out
	}
	if unfinished {
		out.Divergent = "incomplete-process-tree"
		return out
	}
	auditData, _ := os.ReadFile(audit)
	if bytes.Contains(auditData, []byte("uninstrumented-child\t")) {
		// A child without coverage metadata (git, a shell, node, a Go binary
		// built without -cover) executed code this record cannot show, so no
		// parent-only record of the test supports exclusion (LPCV-V0-7601).
		// Ordered-only launches have no isolated owner; reject the suite too.
		out.Divergent = "uninstrumented-child: " + strings.TrimSpace(string(auditData))
		return out
	}
	data, err := os.ReadFile(profile)
	if err != nil {
		out.Divergent = err.Error()
		return out
	}
	var counterDirs []string
	_ = filepath.WalkDir(children, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "covcounters.") {
			counterDirs = append(counterDirs, filepath.Dir(p))
		}
		return nil
	})
	for _, line := range strings.Split(string(auditData), "\n") {
		if p, ok := strings.CutPrefix(line, "covered-child\t"); ok {
			counters, _ := filepath.Glob(filepath.Join(p, "covcounters.*"))
			if len(counters) == 0 {
				out.Divergent = "missing-child-counters: " + p
				return out
			}
		}
	}
	sort.Strings(counterDirs)
	counterDirs = slices.Compact(counterDirs)
	// A test that starts many instrumented children leaves one counter
	// directory per launch; covdata takes them in batches, so the argument
	// list stays below the system limit whatever the count.
	for batch := 0; batch < len(counterDirs); batch += covdataBatch {
		dirs := counterDirs[batch:min(batch+covdataBatch, len(counterDirs))]
		merged := filepath.Join(cov, fmt.Sprintf("children-%d.txt", batch))
		conv := exec.CommandContext(ctx, "go", "tool", "covdata", "textfmt", "-i="+strings.Join(dirs, ","), "-o="+merged)
		if msg, err := conv.CombinedOutput(); err != nil {
			out.Divergent = fmt.Sprintf("covdata: %v: %s", err, msg)
			return out
		}
		more, err := os.ReadFile(merged)
		if err != nil {
			out.Divergent = err.Error()
			return out
		}
		data = append(data, more...)
	}
	// The inherited environment the run read, parent and instrumented
	// children together (GLS-V0-017); an unreadable or malformed audit yields
	// no record. A run that read nothing leaves an empty audit, or none.
	envData, err := os.ReadFile(envAudit)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		out.Divergent = "env-audit: " + err.Error()
		return out
	}
	bindings, err := parseEnvAudit(envData)
	if err != nil {
		out.Divergent = err.Error()
		return out
	}
	// The in-order record owns everything it executed (Record.Union): the
	// windows widen the test records, they do not narrow this one.
	out.Record = &Record{Package: pkg, Name: name, Profile: data, Elapsed: time.Since(began), Union: name == InOrder, Env: bindings, EnvObserved: true}
	return out
}

// covdataBatch is the most child counter directories one covdata invocation
// names: a scratch path is some hundred bytes, so a batch stays well below
// the argument-list limit.
const covdataBatch = 64

// quoteOutput indents a test process's output so that, printed inside
// another go test run (the tools' own tests collect fixture modules), its
// `--- FAIL:` and `=== RUN` lines are not read by test2json as results of
// that run.
func quoteOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for i, l := range lines {
		// test2json starts a frame at the \x16 marker wherever it falls
		// in a line, so the indent alone does not hide a framed line.
		lines[i] = "  | " + strings.ReplaceAll(l, "\x16", "")
	}
	return "\n" + strings.Join(lines, "\n")
}

// compactProfile keeps every executed block and the first occurrence of
// every instrumented block, including blocks introduced by child binaries.
// Malformed lines are retained for Build to reject rather than hidden.
func compactProfile(profile []byte, seen map[string]bool) []byte {
	out := make([]byte, 0, 4096)
	for len(profile) > 0 {
		line := profile
		if i := bytes.IndexByte(profile, '\n'); i >= 0 {
			line, profile = profile[:i+1], profile[i+1:]
		} else {
			profile = nil
		}
		trimmed := bytes.TrimRight(line, "\n")
		keep := true
		if i := bytes.LastIndexByte(trimmed, ' '); i >= 0 && !bytes.HasPrefix(trimmed, []byte("mode:")) {
			key := string(trimmed[:i])
			keep = !seen[key] || !bytes.Equal(trimmed[i+1:], []byte("0"))
			seen[key] = true
		}
		if keep {
			out = append(out, line...)
		}
	}
	return out
}
