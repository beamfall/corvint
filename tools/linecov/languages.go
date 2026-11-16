// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/rubylinecov"
)

// languageCollect exposes existing collectors through the same immutable index
// writer. Commands are explicit argv JSON, never an interpolated shell string.
func languageCollect(language string, args []string) error {
	fs := flag.NewFlagSet(language+"-collect", flag.ContinueOnError)
	root := fs.String("root", ".", "clean repository root (required)")
	out := fs.String("out", "", "output index (required)")
	command := fs.String("command", "", "Ruby suite argv as a JSON array (required for ruby)")
	parallel := fs.Int("parallel", 1, "parallel isolated runs (JavaScript and Ruby)")
	state := fs.String("state", "", "Ruby: persistent collection directory a rerun resumes from (LPCV-V0-7609)")
	budget := fs.Duration("budget", 0, "Ruby: stop launching isolated runs after this; exit 3 with the retained count")
	family := fs.String("runner", "node-test", "JavaScript runner family")
	exe := fs.String("executable", "", "absolute JavaScript runner executable (required for js)")
	files := fs.String("files", "", "comma-separated JavaScript test files (required for js)")
	config := fs.String("config", "", "JavaScript runner configuration path")
	timeout := fs.Duration("timeout", 2*time.Minute, "isolated process deadline")
	suite := fs.Duration("suite-timeout", 10*time.Minute, "in-order suite deadline")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: linecov %s-collect -root DIR -out INDEX [flags]\nExample: linecov ruby-collect -root . -out /tmp/ruby.idx -command '[\"ruby\",\"-Itest\",\"test/all.rb\"]'\nExample: linecov ruby-collect -root . -out /tmp/ruby.idx -state /tmp/ruby-state -parallel 4 -budget 8m -command '[\"bundle\",\"exec\",\"rspec\"]'\nExample: linecov js-collect -root . -out /tmp/js.idx -runner node-test -executable /usr/bin/node -files test/a.test.js\n", language)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required")
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	*root = abs
	if status, err := output(*root, "git", "status", "--porcelain", "--untracked-files=no"); err != nil || status != "" {
		return errors.New("collect: checkout must be clean")
	}
	commit, err := output(*root, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	ctx := context.Background()
	oids, err := linecov.OIDs(ctx, *root, commit)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "linecov-language-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	var ix *linecov.Index
	var argv []string
	switch language {
	case "ruby":
		if err = json.Unmarshal([]byte(*command), &argv); err != nil || len(argv) == 0 {
			return errors.New("ruby-collect: -command must be a nonempty JSON argv array")
		}
		run, err := rubylinecov.Collect(ctx, rubylinecov.Options{Root: *root, Work: work, Argv: argv, Timeout: *suite, IsolatedTimeout: *timeout, State: *state, Parallel: *parallel, Budget: *budget})
		if err != nil {
			return err
		}
		ix, err = rubylinecov.Index(".", commit, run, oids)
		if err != nil {
			return err
		}
	case "js":
		if *exe == "" || *files == "" {
			return errors.New("js-collect: -executable and -files required")
		}
		version, err := output(*root, *exe, "--version")
		if err != nil {
			return err
		}
		outcomes, err := jslinecov.Collect(ctx, *root, jslinecov.Options{Module: ".", Runner: *family, Executable: *exe, Files: split(*files), Config: *config, Work: work, Parallel: *parallel, Timeout: *timeout, SuiteTimeout: *suite, Keep: func(p string) bool { return oids[p] != "" }})
		if err != nil {
			return err
		}
		var records []linecov.Record
		var unordered []string
		for _, o := range outcomes {
			if o.Record != nil {
				records = append(records, *o.Record)
			} else {
				fmt.Fprintf(os.Stderr, "%s.%s: %s\n", o.Package, o.Name, o.Divergent)
				unordered = append(unordered, o.Package)
			}
		}
		ix, err = linecov.Build(".", commit, jslinecov.Identity(*family, version), oids, records)
		if err != nil {
			return err
		}
		sort.Strings(unordered)
		ix.Unordered = unordered
	default:
		return fmt.Errorf("unknown language %s", language)
	}
	ix.Language, ix.Command, ix.Executable, ix.RunnerFamily, ix.Config = language, argv, *exe, *family, *config
	if language == "ruby" {
		ix.Executable, ix.RunnerFamily, ix.Config = "", "", ""
	}
	ix.TestTimeout, ix.SuiteTimeout = *timeout, *suite
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err = ix.Encode(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// LanguagePlan retains both the candidate and effective decisions. Until the
// runner's fault qualification is admitted, candidate exclusions are shadow
// observations and effective execution keeps the static files; an admitted
// Ruby family executes its line-level candidate (LPCV-V0-7605, LPCV-V0-7608).
type LanguagePlan struct {
	RecordedTests               map[string][]string
	Index, Base, Head, Language string
	Runner                      string // the index runner identity admission checks
	Scope                       string
	GateEligible                bool
	Reason                      string
	// Admission is the Ruby admission outcome: "admitted" or the refusal.
	// Fallback is the candidate's static-fallback reason, if any.
	Admission, Fallback                        string `json:",omitempty"`
	Candidate, Effective                       map[string]linecov.PackagePlan
	CoverageSelected, StaticFallback, Excluded int
	ElapsedMS                                  int64
}

func languageSelect(language string, args []string) error {
	fs := flag.NewFlagSet(language+"-select", flag.ContinueOnError)
	root := fs.String("root", ".", "repository at change head")
	index := fs.String("index", "", "collected index (required)")
	base := fs.String("base", "", "full change base commit (required)")
	files := fs.String("files", "", "comma-separated static test files (default tracked test files)")
	var opts selectOptions
	fs.StringVar(&opts.selectorRoot, "selector-root", "", "Ruby: Corvint checkout holding this executable's revision and its Ruby qualification record (LPCV-V0-7608)")
	fs.StringVar(&opts.ruby, "ruby", "ruby", "Ruby: interpreter of the index's Ruby version for the syntax check of changed files")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: linecov %s-select -root DIR -index INDEX -base FULL_SHA [-files a,b] [-selector-root CORVINT_DIR] [-ruby PATH]\nExample: linecov %s-select -root . -index /tmp/tests.idx -base FULL_SHA\nExample: linecov ruby-select -root . -index /tmp/ruby.idx -base FULL_SHA -selector-root ~/corvint\nOutput: user-scoped candidate and effective plan; the effective plan stays the static suite unless a passing Ruby qualification record admits the family.\n", language, language)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *index == "" || *base == "" {
		return errors.New("-index and -base required")
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
	p, err := languageDecide(context.Background(), *root, ix, language, *base, split(*files), opts)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

// selectOptions configures Ruby admission and its syntax check.
type selectOptions struct {
	selectorRoot, ruby string
}

// rubyAdmit is the LPCV-V0-7608 admission of this executable; tests replace it.
var rubyAdmit = func(ctx context.Context, selectorRoot, runner string) error {
	info, _ := debug.ReadBuildInfo()
	return linecov.CheckRubyQualification(ctx, selectorRoot, runner, info)
}

// rubySyntax returns the check that a changed source parses under the
// index's Ruby (`ruby -c`), or nil without an interpreter. An interpreter of
// another version than the index runner's fails every check.
func rubySyntax(ctx context.Context, ruby, runner string) func(string, []byte) error {
	if ruby == "" {
		return nil
	}
	var once sync.Once
	var versionErr error
	env := withoutRubyCollector(os.Environ())
	return func(path string, src []byte) error {
		once.Do(func() {
			cmd := exec.CommandContext(ctx, ruby, "-e", "print RUBY_VERSION")
			cmd.Env = env
			out, err := cmd.Output()
			want := strings.Fields(runner)
			switch {
			case err != nil:
				versionErr = fmt.Errorf("ruby %s unavailable: %v", ruby, err)
			case len(want) < 2 || want[0] != "ruby" || want[1] != string(out):
				versionErr = fmt.Errorf("syntax checker runs Ruby %s, the index runner is %q", out, runner)
			}
		})
		if versionErr != nil {
			return versionErr
		}
		cmd := exec.CommandContext(ctx, ruby, "-c", "-")
		cmd.Env = env
		cmd.Stdin = bytes.NewReader(src)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s", firstLine(strings.TrimSpace(string(out))))
		}
		return nil
	}
}

func withoutRubyCollector(env []string) []string {
	var out []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, "RUBYOPT=") && !strings.HasPrefix(kv, "CORVINT_LINECOV_") {
			out = append(out, kv)
		}
	}
	return out
}

func languageDecide(ctx context.Context, root string, ix *linecov.Index, language, base string, static []string, opts selectOptions) (LanguagePlan, error) {
	start := time.Now()
	head, err := output(root, "git", "rev-parse", "HEAD")
	if err != nil {
		return LanguagePlan{}, err
	}
	p := LanguagePlan{Index: ix.Commit, Base: base, Head: head, Language: language, Runner: ix.Runner, Scope: "user", Reason: "coverage-collector-unqualified", Effective: map[string]linecov.PackagePlan{}}
	if static == nil {
		tracked, err := output(root, "git", "ls-files")
		if err != nil {
			return p, err
		}
		for _, f := range strings.Split(tracked, "\n") {
			if language == "ruby" && rubylinecov.IsTestFile(f) || language == "js" && jslinecov.IsTestFile(f) {
				static = append(static, f)
			}
		}
	}
	if ix.Language != language {
		return p, errors.New("index language mismatch")
	}
	if _, err := output(root, "git", "merge-base", "--is-ancestor", ix.Commit, base); err != nil {
		return p, errors.New("index commit is not an ancestor of base")
	}
	driftDiff, err := linecov.Diff(ctx, root, ix.Commit, base)
	if err != nil {
		return p, err
	}
	diff, err := linecov.Diff(ctx, root, base, head)
	if err != nil {
		return p, err
	}
	var drift, change linecov.Effect
	switch language {
	case "ruby":
		check := rubySyntax(ctx, opts.ruby, ix.Runner)
		drift = rubylinecov.AnalyzeLines(ix, driftDiff, rubylinecov.Sources{Old: linecov.Reader(ctx, root, ix.Commit), New: linecov.Reader(ctx, root, base), Check: check})
		change = rubylinecov.AnalyzeLines(ix, diff, rubylinecov.Sources{Old: linecov.Reader(ctx, root, base), New: linecov.Reader(ctx, root, head), Check: check})
	case "js":
		drift = jslinecov.Analyze(ix, driftDiff, linecov.Reader(ctx, root, ix.Commit))
		change = jslinecov.Analyze(ix, diff, linecov.Reader(ctx, root, base))
	}
	if change.Whole == nil {
		change.Whole = map[string]bool{}
	}
	heads := map[string][]string{}
	for _, t := range ix.Tests {
		if t.Package != rubylinecov.Load && t.Name != linecov.InOrder {
			heads[t.Package] = append(heads[t.Package], t.Name)
		}
	}
	// A changed/added test file is always whole; unseen files remain whole too.
	for _, d := range diff {
		if language == "ruby" && rubylinecov.IsTestFile(d.Path) || language == "js" && jslinecov.IsTestFile(d.Path) {
			change.Whole[d.Path] = true
		}
	}
	if status, err := output(root, "git", "status", "--porcelain", "--untracked-files=no"); err != nil || status != "" {
		change.Fallback = "checkout is not clean"
	}
	p.RecordedTests = heads
	p.Candidate, p.Fallback = ix.Narrow(nil, static, heads, drift, change) // a candidate: no environment binding
	admitted := false
	if language == "ruby" {
		// The family is admitted only by a passing record for this executable
		// and the index's runner; any refusal keeps the static plan.
		if err := rubyAdmit(ctx, opts.selectorRoot, ix.Runner); err != nil {
			p.Admission = err.Error()
		} else {
			admitted, p.Admission = true, "admitted"
			p.Reason, p.GateEligible = "coverage-qualified", true
		}
	}
	for _, f := range static {
		pp := p.Candidate[f]
		if len(heads[f]) == 0 {
			pp = linecov.PackagePlan{Mode: linecov.Whole}
			p.Candidate[f] = pp
		}
		countDecision(&p, pp, heads[f])
		p.Effective[f] = linecov.PackagePlan{Mode: linecov.Whole}
		if admitted {
			p.Effective[f] = pp
		}
	}
	p.ElapsedMS = time.Since(start).Milliseconds()
	return p, nil
}

func languageWorkingPlan(ctx context.Context, root string, ix *linecov.Index, static []string, diffs []linecov.FileDiff) (LanguagePlan, error) {
	p := LanguagePlan{Index: ix.Commit, Base: ix.Commit, Head: "working-tree", Language: ix.Language, Scope: "user", Reason: "coverage-collector-unqualified", Effective: map[string]linecov.PackagePlan{}}
	if static == nil {
		for _, t := range ix.Tests {
			if t.Package != rubylinecov.Load {
				static = append(static, t.Package)
			}
		}
		for _, d := range diffs {
			if rubylinecov.IsTestFile(d.Path) || jslinecov.IsTestFile(d.Path) {
				static = append(static, d.Path)
			}
		}
		sort.Strings(static)
		static = slices.Compact(static)
	}
	var e linecov.Effect
	if ix.Language == "ruby" {
		e = rubylinecov.Analyze(ix, diffs)
	} else {
		e = jslinecov.Analyze(ix, diffs, linecov.Reader(ctx, root, ix.Commit))
	}
	heads := map[string][]string{}
	for _, t := range ix.Tests {
		if t.Name != linecov.InOrder && t.Package != rubylinecov.Load {
			heads[t.Package] = append(heads[t.Package], t.Name)
		}
	}
	p.RecordedTests = heads
	p.Candidate, _ = ix.Narrow(nil, static, heads, linecov.Effect{}, e) // a candidate: no environment binding
	for _, f := range static {
		pp := p.Candidate[f]
		if len(heads[f]) == 0 {
			pp = linecov.PackagePlan{Mode: linecov.Whole}
			p.Candidate[f] = pp
		}
		countDecision(&p, pp, heads[f])
		p.Effective[f] = linecov.PackagePlan{Mode: linecov.Whole}
	}
	return p, nil
}

func countDecision(p *LanguagePlan, pp linecov.PackagePlan, names []string) {
	if pp.Mode == linecov.Exclude {
		p.Excluded += len(names)
		return
	}
	if pp.Mode == linecov.Whole && len(pp.Why) == 0 {
		p.StaticFallback += len(names)
		return
	}
	for _, name := range names {
		switch pp.Why[name] {
		case "selected":
			p.CoverageSelected++
		case "":
			p.Excluded++
		default:
			p.StaticFallback++
		}
	}
}
