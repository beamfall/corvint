package main

import (
	"context"
	"flag"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/evidencereach"
	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// reasonTestRootUnresolved is the FRESHNESS_UNRESOLVED detail when a test file cannot be located in
// the repository (PWR-V0-036).
const reasonTestRootUnresolved = "TEST_ROOT_UNRESOLVED"

// runsReadAll reads every named report or refuses, naming the option and 1-based position.
// ok is false after a refusal was written.
// ident, when not nil, binds identity sidecars (PWR-V0-065).
func runsReadAll(option string, names []string, ident *runsIdentityOpts, diagnostic io.Writer) (runs []*playwrightruns.Run, ok bool) {
	runs = []*playwrightruns.Run{}
	for i, name := range names {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"option": option, "reason": err.Error(), "run": i + 1})
			return nil, false
		}
		if !ident.apply(option, i+1, name, run, diagnostic) {
			return nil, false
		}
		runs = append(runs, run)
	}
	return runs, true
}

func runRunsReusable(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs reusable", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	priors, selection := []string{}, []string{}
	f.Func("prior", "prior Playwright JSON reports", runsFiles(root, &priors))
	f.Func("selection", "the selected tests as one Playwright JSON report", runsFiles(root, &selection))
	fresh := f.Bool("fresh", false, "admit FRESH_PASS through evidence freshness")
	summary := f.Bool("summary", false, "compact form")
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		if len(selection) > 1 {
			return []runsNeed{{"selection", runsArgValueInvalid}}
		}
		return []runsNeed{runsRequired("selection", len(selection) == 0), ident.need(selection, priors)}
	}) {
		return 2
	}
	selected, ok := runsReadAll("selection", selection, ident, diagnostic)
	if !ok {
		return 2
	}
	prior, ok := runsReadAll("prior", priors, ident, diagnostic)
	if !ok {
		return 2
	}
	var check playwrightruns.FreshnessCheck
	if *fresh {
		check = testFileFreshness(ctx, root, selected[0])
	}
	report := playwrightruns.Reusable(selected[0], prior, check)
	if *summary {
		report = report.Compact()
	}
	return runsEmit(report, out, diagnostic)
}

// testFileFreshness checks a test file's evidence through the repository's declared reach. The
// file is rootDir-relative; it is located by the selection's declared testRootPath, else by its
// config.rootDir under the repository top level, else the check is unresolved (PWR-V0-036).
func testFileFreshness(ctx context.Context, root string, selection *playwrightruns.Run) playwrightruns.FreshnessCheck {
	reader := evidencereach.NewReader(root)
	prefix, located := runTestRoot(ctx, root, selection)
	return func(oldRevision, newRevision, testFile string) playwrightruns.FreshVerdict {
		cited := path.Join(prefix, testFile)
		if !located || !locatableTestFile(testFile) || !evidencereach.ValidPath(cited) {
			return playwrightruns.FreshVerdict{Verdict: evidencereach.Unresolved, Reason: reasonTestRootUnresolved}
		}
		res := reader.Check(ctx, oldRevision, newRevision, []string{cited})
		paths := make([]string, 0, len(res.Deciding))
		for _, d := range res.Deciding {
			paths = append(paths, d.Path)
		}
		return playwrightruns.FreshVerdict{Verdict: res.Verdict, Reason: res.Reason, Paths: paths, Truncated: res.DecidingTruncated}
	}
}

// runTestRoot is the repository-relative directory a report's rootDir-relative test files are
// under: its declared testRootPath, else its config.rootDir under the repository top level.
// located is false when neither resolves inside the repository.
func runTestRoot(ctx context.Context, root string, run *playwrightruns.Run) (prefix string, located bool) {
	switch rel := run.TestRootPath; {
	case rel == ".":
		return "", true
	case rel != "":
		return rel, evidencereach.ValidPath(rel)
	case run.RootDir != "":
		toplevel, err := sourceGitText(ctx, root, 8192, "rev-parse", "--show-toplevel")
		if err == nil {
			return repositoryRelative(toplevel, run.RootDir)
		}
	}
	return "", false
}

// locatableTestFile reports whether a report's rootDir-relative test file name may be joined to
// the test root. It is checked before the join, so an absolute, escaping or volume-qualified name
// (backslash forms arrive slash-normalized by the reader) is never laundered into the root.
func locatableTestFile(name string) bool {
	if len(name) >= 2 && name[1] == ':' {
		return false
	}
	return evidencereach.ValidPath(name)
}

// repositoryRelative is dir as a slash path relative to the repository top level, "" for the top
// level itself; ok is false when either is unknown or dir lies outside the repository.
func repositoryRelative(toplevel, dir string) (string, bool) {
	if toplevel == "" || !filepath.IsAbs(filepath.FromSlash(dir)) {
		return "", false
	}
	top, err := filepath.EvalSymlinks(toplevel)
	if err != nil {
		return "", false
	}
	target := filepath.FromSlash(dir)
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	rel, err := filepath.Rel(top, target)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "", true
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

func runRunsRedundancy(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs redundancy", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	files := []string{}
	var since *time.Time
	f.Func("runs", "Playwright JSON reports", runsFiles(root, &files))
	f.Func("since", "RFC 3339 window start", func(v string) error {
		at, err := time.Parse(time.RFC3339Nano, v)
		if err != nil || since != nil {
			return errRunsArgument
		}
		since = &at
		return nil
	})
	summary := f.Bool("summary", false, "compact form")
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("runs", len(files) == 0), ident.need(files)}
	}) {
		return 2
	}
	runs, ok := runsReadAll("runs", files, ident, diagnostic)
	if !ok {
		return 2
	}
	report := playwrightruns.Redundancy(runs, since)
	if *summary {
		report = report.Compact()
	}
	return runsEmit(report, out, diagnostic)
}

func runRunsFresh(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs fresh", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	oldRev, newRev, cited := "", "", []string{}
	single := func(dst *string) func(string) error {
		return func(v string) error {
			if v == "" || *dst != "" {
				return errRunsArgument
			}
			*dst = v
			return nil
		}
	}
	f.Func("old", "revision the evidence was produced at", single(&oldRev))
	f.Func("new", "revision the evidence is read at", single(&newRev))
	f.Func("cite", "repository-relative cited paths", func(v string) error {
		for _, p := range strings.Split(v, ",") {
			if !evidencereach.ValidPath(p) || len(cited) >= evidencereach.MaxCited {
				return errRunsArgument
			}
			cited = append(cited, p)
		}
		return nil
	})
	summary := f.Bool("summary", false, "compact form")
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("old", oldRev == ""), runsRequired("new", newRev == "")}
	}) {
		return 2
	}
	res := evidencereach.NewReader(root).Check(ctx, oldRev, newRev, cited)
	if *summary {
		res = res.Summary()
	}
	return runsEmit(res, out, diagnostic)
}
