package main

import (
	"context"
	"flag"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gitancestry"
	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// runs baseline records the failures a full run and an isolated re-run share at one revision, and
// gates later runs on it (PWR-V0-041..045, GitHub #1033). Both subcommands are reads: record
// prints the baseline document for the caller to retain, and diff re-reads the evidence it names.
func runRunsBaseline(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return runsInvalid(diagnostic, "", runsArgSubcommandMissing)
	}
	switch args[0] {
	case "record":
		return runRunsBaselineRecord(ctx, root, args[1:], out, diagnostic)
	case "diff":
		return runRunsBaselineDiff(ctx, root, args[1:], out, diagnostic)
	}
	return runsInvalid(diagnostic, "", runsArgSubcommandUnknown)
}

// runsResolveCommit resolves rev to a full commit object id with one bounded Git read.
func runsResolveCommit(ctx context.Context, root, rev string) (string, bool) {
	if rev == "" || strings.HasPrefix(rev, "-") || strings.ContainsAny(rev, "\x00\n\r") {
		return "", false
	}
	oid, err := sourceGitText(ctx, root, 8192, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil || oid == "" {
		return "", false
	}
	return oid, true
}

func runsNamed(option string, names []string, ident *runsIdentityOpts, diagnostic io.Writer) ([]playwrightruns.NamedRun, bool) {
	runs, ok := runsReadAll(option, names, ident, diagnostic)
	if !ok {
		return nil, false
	}
	named := make([]playwrightruns.NamedRun, len(runs))
	for i, run := range runs {
		named[i] = playwrightruns.NamedRun{Path: names[i], Run: run}
	}
	return named, true
}

// runsNamedUnder reads names resolved against root but names each run by the path as given.
func runsNamedUnder(root, option string, names []string, ident *runsIdentityOpts, diagnostic io.Writer) ([]playwrightruns.NamedRun, bool) {
	runs, ok := runsNamed(option, runsUnder(root, names), ident, diagnostic)
	for i := range runs {
		runs[i].Path = names[i]
	}
	return runs, ok
}

// runsUnder resolves each relative name against root.
func runsUnder(root string, names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		if root != "" && !filepath.IsAbs(name) {
			name = filepath.Join(root, name)
		}
		out[i] = name
	}
	return out
}

func runRunsBaselineRecord(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs baseline record", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	// Evidence paths are recorded as given, relative to --root, so a diff with the same --root
	// re-reads the same files.
	full, isolated := []string{}, []string{}
	rev := f.String("rev", "", "the revision both runs tested")
	f.Func("full", "full-run Playwright JSON reports", runsFiles("", &full))
	f.Func("isolated", "isolated re-run Playwright JSON reports", runsFiles("", &isolated))
	allowUnobserved := f.Bool("allow-unobserved", false, "record tests the full run never observed completing as unobserved")
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("rev", *rev == ""), runsRequired("full", len(full) == 0), runsRequired("isolated", len(isolated) == 0),
			ident.need(runsUnder(root, full), runsUnder(root, isolated))}
	}) {
		return 2
	}
	revision, ok := runsResolveCommit(ctx, root, *rev)
	if !ok {
		return runsRefuse(diagnostic, "runs-revision-unresolved", map[string]any{"option": "rev"})
	}
	fullRuns, ok := runsNamedUnder(root, "full", full, ident, diagnostic)
	if !ok {
		return 2
	}
	isoRuns, ok := runsNamedUnder(root, "isolated", isolated, ident, diagnostic)
	if !ok {
		return 2
	}
	// A report that declares the revision it tested must have tested this one.
	for option, runs := range map[string][]playwrightruns.NamedRun{"full": fullRuns, "isolated": isoRuns} {
		for i, nr := range runs {
			if declared := nr.Run.Inputs.ApplicationRevision; declared != "" && declared != revision {
				return runsRefuse(diagnostic, "runs-baseline-unavailable", map[string]any{"option": option, "run": i + 1, "reason": "APPLICATION_REVISION_DIFFERS"})
			}
		}
	}
	b := playwrightruns.RecordBaseline(revision, fullRuns, isoRuns)
	// A full run that left tests unobserved (interrupted or not run) is not a clean baseline: refuse
	// unless the caller accepts recording them as unobserved (PWR-V0-060).
	if len(b.Unobserved) > 0 && !*allowUnobserved {
		return runsRefuse(diagnostic, "runs-baseline-unavailable", map[string]any{"option": "full", "reason": "FULL_RUN_INCOMPLETE", "unobserved": len(b.Unobserved)})
	}
	if len(b.Known)+len(b.Flaky)+len(b.Unobserved) > playwrightruns.MaxBaselineEntries {
		return runsRefuse(diagnostic, "runs-baseline-unavailable", map[string]any{"reason": "BASELINE_BOUND_EXCEEDED"})
	}
	return runsEmit(b, out, diagnostic)
}

func runRunsBaselineDiff(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs baseline diff", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	baseline, candidate, own := []string{}, []string{}, []string{}
	f.Func("baseline", "the baseline document", runsFiles(root, &baseline))
	f.Func("runs", "candidate Playwright JSON reports", runsFiles(root, &candidate))
	f.Func("own", "the change's own test files", func(v string) error {
		for _, p := range strings.Split(v, ",") {
			if !locatableTestFile(p) || len(own) >= playwrightruns.MaxRuns {
				return errRunsArgument
			}
			own = append(own, path.Clean(p))
		}
		return nil
	})
	base := f.String("base", "", "the branch's base revision for the staleness check")
	maxAge, maxAgeSet := playwrightruns.DefaultBaselineMaxAge, false
	f.Func("max-age", "staleness bound in commits", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || maxAgeSet || n < 0 || n > 1000000 || strconv.Itoa(n) != v {
			return errRunsArgument
		}
		maxAge, maxAgeSet = n, true
		return nil
	})
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		if len(baseline) > 1 {
			return []runsNeed{{"baseline", runsArgValueInvalid}}
		}
		return []runsNeed{runsRequired("baseline", len(baseline) == 0), runsRequired("runs", len(candidate) == 0), ident.need(candidate)}
	}) {
		return 2
	}
	b, sum, err := playwrightruns.ReadBaseline(baseline[0])
	if err != nil {
		return runsRefuse(diagnostic, "runs-baseline-unavailable", map[string]any{"option": "baseline", "reason": err.Error()})
	}
	runs, ok := runsNamed("runs", candidate, ident, diagnostic)
	if !ok {
		return 2
	}
	// A file is own when, under any candidate's test root, it is an --own path or lies under one.
	// With --own given, a candidate whose test root does not resolve fails closed: a known failure
	// that matches no own path blocks as TEST_ROOT_UNRESOLVED, since it may be an own file.
	roots, unresolved := []string{""}, false
	for _, nr := range runs {
		if prefix, located := runTestRoot(ctx, root, nr.Run); !located {
			unresolved = true
		} else if prefix != "" {
			roots = append(roots, prefix)
		}
	}
	isOwn := func(file string) string {
		if len(own) == 0 {
			return ""
		}
		for _, r := range roots {
			p := path.Join(r, file)
			for _, o := range own {
				if p == o || strings.HasPrefix(p, o+"/") {
					return playwrightruns.ReasonOwnFile
				}
			}
		}
		if unresolved {
			return reasonTestRootUnresolved
		}
		return ""
	}
	// Recorded evidence paths are relative to --root (runs baseline record keeps them as given).
	load := func(name string) (*playwrightruns.Run, error) {
		return playwrightruns.ReadFile(runsUnder(root, []string{name})[0])
	}
	d := playwrightruns.DiffBaseline(b, sum, runs, isOwn, load, baselineStaleness(ctx, root, b.Revision, *base, maxAge))
	if code := runsEmit(d, out, diagnostic); code != 0 {
		return code
	}
	if d.Verdict != playwrightruns.VerdictBaselinePass {
		return 1
	}
	return 0
}

// baselineStaleness counts the commits from the baseline revision to the branch point (the merge
// base of HEAD and base). It is reported, never blocking, and NOT_OBSERVED when base is not given
// or a Git read fails (PWR-V0-045).
func baselineStaleness(ctx context.Context, root, revision, base string, maxAge int) playwrightruns.Staleness {
	s := playwrightruns.Staleness{Status: "NOT_OBSERVED", MaxAge: maxAge}
	if base == "" {
		s.Reason = "BASE_NOT_GIVEN"
		return s
	}
	baseOID, ok := runsResolveCommit(ctx, root, base)
	if !ok {
		s.Reason = "REVISION_UNRESOLVED"
		return s
	}
	// A grafted or shallow checkout rewrites every ancestry read below, so the branch point is
	// unresolved rather than a STALE or FRESH answer from rewritten parents (EEP-V0-770).
	if gitancestry.Rewritten(ctx, func(ctx context.Context, args ...string) ([]byte, error) {
		return sourceGit(ctx, root, 8192, args...)
	}) != "" {
		s.Reason = "BRANCH_POINT_UNRESOLVED"
		return s
	}
	point, err := sourceGitText(ctx, root, 8192, "merge-base", "HEAD", baseOID)
	if err != nil || point == "" {
		s.Reason = "BRANCH_POINT_UNRESOLVED"
		return s
	}
	s.BranchPoint = point
	if _, err := sourceGitText(ctx, root, 8192, "merge-base", "--is-ancestor", revision, point); err != nil {
		s.Status, s.Reason = "STALE", "BASELINE_NOT_ANCESTOR"
		// Name where the two lines meet and how far each side is from it (PWR-V0-062).
		if mb, err := sourceGitText(ctx, root, 8192, "merge-base", revision, point); err == nil && mb != "" {
			s.MergeBase = mb
			s.BaselineSideCommits = runsCommitCount(ctx, root, mb, revision)
			s.BranchSideCommits = runsCommitCount(ctx, root, mb, point)
		}
		return s
	}
	n := runsCommitCount(ctx, root, revision, point)
	if n == nil {
		s.Reason = "COUNT_UNAVAILABLE"
		return s
	}
	s.Commits = n
	s.Status = "FRESH"
	if *n > maxAge {
		s.Status, s.Reason = "STALE", "OLDER_THAN_MAX_AGE"
	}
	return s
}

// runsCommitCount is the number of commits in from..to, nil when Git cannot count them.
func runsCommitCount(ctx context.Context, root, from, to string) *int {
	count, err := sourceGitText(ctx, root, 8192, "rev-list", "--count", from+".."+to)
	if err != nil {
		return nil
	}
	n, err := strconv.Atoi(count)
	if err != nil {
		return nil
	}
	return &n
}
