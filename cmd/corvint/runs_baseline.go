package main

import (
	"context"
	"flag"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// runs baseline records the failures a full run and an isolated re-run share at one revision, and
// gates later runs on it (PWR-V0-041..045, GitHub #1033). Both subcommands are reads: record
// prints the baseline document for the caller to retain, and diff re-reads the evidence it names.
func runRunsBaseline(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	switch args[0] {
	case "record":
		return runRunsBaselineRecord(ctx, root, args[1:], out, diagnostic)
	case "diff":
		return runRunsBaselineDiff(ctx, root, args[1:], out, diagnostic)
	}
	return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
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

func runsNamed(option string, names []string, diagnostic io.Writer) ([]playwrightruns.NamedRun, bool) {
	runs, ok := runsReadAll(option, names, diagnostic)
	if !ok {
		return nil, false
	}
	named := make([]playwrightruns.NamedRun, len(runs))
	for i, run := range runs {
		named[i] = playwrightruns.NamedRun{Path: names[i], Run: run}
	}
	return named, true
}

func runRunsBaselineRecord(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs baseline record", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	full, isolated := []string{}, []string{}
	rev := f.String("rev", "", "the revision both runs tested")
	f.Func("full", "full-run Playwright JSON reports", runsFiles(root, &full))
	f.Func("isolated", "isolated re-run Playwright JSON reports", runsFiles(root, &isolated))
	if f.Parse(args) != nil || f.NArg() != 0 || *rev == "" || len(full) == 0 || len(isolated) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	revision, ok := runsResolveCommit(ctx, root, *rev)
	if !ok {
		return runsRefuse(diagnostic, "runs-revision-unresolved", map[string]any{"option": "rev"})
	}
	fullRuns, ok := runsNamed("full", full, diagnostic)
	if !ok {
		return 2
	}
	isoRuns, ok := runsNamed("isolated", isolated, diagnostic)
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
	if len(b.Known)+len(b.Flaky) > playwrightruns.MaxBaselineEntries {
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
	if f.Parse(args) != nil || f.NArg() != 0 || len(baseline) != 1 || len(candidate) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	b, sum, err := playwrightruns.ReadBaseline(baseline[0])
	if err != nil {
		return runsRefuse(diagnostic, "runs-baseline-unavailable", map[string]any{"option": "baseline", "reason": err.Error()})
	}
	runs, ok := runsNamed("runs", candidate, diagnostic)
	if !ok {
		return 2
	}
	roots := []string{""}
	for _, nr := range runs {
		if r := nr.Run.TestRootPath; r != "" && r != "." && locatableTestFile(r) {
			roots = append(roots, r)
		}
	}
	isOwn := func(file string) bool {
		for _, r := range roots {
			p := path.Join(r, file)
			for _, o := range own {
				if p == o || strings.HasPrefix(p, o+"/") {
					return true
				}
			}
		}
		return false
	}
	load := func(name string) (*playwrightruns.Run, error) {
		if root != "" && !filepath.IsAbs(name) {
			name = filepath.Join(root, name)
		}
		return playwrightruns.ReadFile(name)
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
	point, err := sourceGitText(ctx, root, 8192, "merge-base", "HEAD", baseOID)
	if err != nil || point == "" {
		s.Reason = "BRANCH_POINT_UNRESOLVED"
		return s
	}
	s.BranchPoint = point
	if _, err := sourceGitText(ctx, root, 8192, "merge-base", "--is-ancestor", revision, point); err != nil {
		s.Status, s.Reason = "STALE", "BASELINE_NOT_ANCESTOR"
		return s
	}
	count, err := sourceGitText(ctx, root, 8192, "rev-list", "--count", revision+".."+point)
	n, cerr := strconv.Atoi(count)
	if err != nil || cerr != nil {
		s.Reason = "COUNT_UNAVAILABLE"
		return s
	}
	s.Commits = &n
	s.Status = "FRESH"
	if n > maxAge {
		s.Status, s.Reason = "STALE", "OLDER_THAN_MAX_AGE"
	}
	return s
}
