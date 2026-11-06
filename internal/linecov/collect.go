package linecov

import (
	"bytes"
	"context"
	"fmt"
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
	// GOFLAGS is the effective `go env GOFLAGS` the index's runner names, so
	// flags saved with `go env -w` apply to collection as they do to selection.
	GOFLAGS string
}

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
// the same GOCOVERDIR (LPCV-V0-954).
func Collect(ctx context.Context, root string, o CollectOptions) ([]Outcome, error) {
	if o.Parallel < 1 {
		o.Parallel = 1
	}
	// The runner's GOFLAGS (build tags among them) still apply.
	flags := strings.TrimSpace(o.GOFLAGS + " -cover -covermode=atomic -coverpkg=" + o.Module + "/...")
	// Clipped, so each worker's append below copies instead of sharing storage.
	env := slices.Clip(append(os.Environ(), "GOFLAGS="+flags))
	type job struct{ pkg, name, bin, dir, flags string }
	var jobs []job
	windows := map[string]string{}
	fallback := map[string]string{}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for i, pkg := range o.Packages {
		bin := filepath.Join(o.Work, fmt.Sprintf("p%d.test", i))
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		overlayWork := filepath.Join(o.Work, fmt.Sprintf("overlay%d", i))
		if err := os.MkdirAll(overlayWork, 0700); err != nil {
			return nil, err
		}
		overlay, why, err := windowOverlay(dir, overlayWork)
		if err != nil {
			return nil, err
		}
		fallback[pkg] = why
		overlay, err = collectorOverlay(overlayWork, overlay)
		if err != nil {
			return nil, err
		}
		buildArgs := []string{"test", "-c", "-o", bin}
		if overlay != "" {
			buildArgs = append(buildArgs, "-overlay="+overlay)
			if why == "" {
				windows[pkg] = filepath.Join(o.Work, fmt.Sprintf("windows%d", i))
			}
			if why == "" {
				if err := os.MkdirAll(windows[pkg], 0700); err != nil {
					return nil, err
				}
			}
		}
		build := exec.CommandContext(ctx, "go", append(buildArgs, "./"+pkg)...)
		// The overlay keys are paths under root as given. With an explicit Env,
		// os/exec leaves the inherited PWD stale, so the go command would fall
		// back to getcwd's symlink-resolved path (macOS /var -> /private/var) and
		// silently skip every window wrapper. PWD keeps both spellings equal.
		build.Dir, build.Env = root, append(env, "PWD="+absRoot)
		if out, err := build.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("linecov: build %s: %v: %s", pkg, err, out)
		}
		if _, err := os.Stat(bin); err != nil {
			continue // no test files
		}
		list := exec.CommandContext(ctx, bin, "-test.list", ".*")
		list.Dir, list.Env = dir, env
		out, err := list.Output()
		if err != nil {
			return nil, fmt.Errorf("linecov: list %s: %v", pkg, err)
		}
		listed := false
		for _, name := range strings.Fields(string(out)) {
			if listedTest.MatchString(name) {
				jobs = append(jobs, job{pkg, name, bin, dir, flags + " -overlay=" + overlay})
				listed = true
			}
		}
		if listed {
			jobs = append(jobs, job{pkg, InOrder, bin, dir, flags + " -overlay=" + overlay})
		}
	}
	outcomes := make([]Outcome, len(jobs))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < o.Parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				j := jobs[i]
				jobEnv := append(slices.Clone(env), "GOFLAGS="+j.flags)
				if j.name == InOrder && windows[j.pkg] != "" {
					jobEnv = append(jobEnv, "CORVINT_LINECOV_WINDOWS="+windows[j.pkg])
				}
				outcomes[i] = runOne(ctx, o, jobEnv, j.pkg, j.name, j.bin, j.dir, filepath.Join(o.Work, fmt.Sprintf("c%d", i)))
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range outcomes {
		if outcomes[i].Name == InOrder {
			outcomes[i].WindowFallback = fallback[outcomes[i].Package]
		}
	}
	outcomes = attributeWindows(func(dir string) ([]byte, error) {
		p := filepath.Join(dir, "profile.txt")
		cmd := exec.CommandContext(ctx, "go", "tool", "covdata", "textfmt", "-i="+dir, "-o="+p)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("covdata: %v: %s", err, out)
		}
		return os.ReadFile(p)
	}, outcomes, windows)
	sort.Slice(outcomes, func(i, j int) bool {
		a, b := outcomes[i], outcomes[j]
		return a.Package < b.Package || a.Package == b.Package && a.Name < b.Name
	})
	return outcomes, nil
}

func runOne(ctx context.Context, o CollectOptions, env []string, pkg, name, bin, dir, cov string) Outcome {
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
	args := []string{"-test.count=1", "-test.gocoverdir=" + cov, "-test.coverprofile=" + profile}
	if name != InOrder {
		args = append(args, "-test.run", "^"+name+"$")
	}
	args = append(args, o.TestArgs...)
	cmd := exec.CommandContext(runCtx, bin, args...)
	audit := filepath.Join(cov, "exec-audit")
	cmd.Dir, cmd.Env = dir, append(env, "GOCOVERDIR="+children, "CORVINT_LINECOV_AUDIT="+audit)
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	began := time.Now()
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			out.Divergent = "timeout"
		} else {
			out.Divergent = fmt.Sprintf("%v: %s", err, log.String())
		}
		return out
	}
	auditData, _ := os.ReadFile(audit)
	if bytes.Contains(auditData, []byte("uninstrumented-child\t")) {
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
	if len(counterDirs) > 0 {
		merged := filepath.Join(cov, "children.txt")
		conv := exec.CommandContext(ctx, "go", "tool", "covdata", "textfmt", "-i="+strings.Join(counterDirs, ","), "-o="+merged)
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
	out.Record = &Record{Package: pkg, Name: name, Profile: data, Elapsed: time.Since(began)}
	return out
}
