package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

const runsHelp = `Read Playwright JSON reports across runs (experimental, PWR-V0).

Usage:
  corvint [--root PATH] runs stability --runs FILE[,FILE...] [--runs ...]
    [--min-runs N]
  corvint [--root PATH] runs compare --baseline FILE[,FILE...]
    --candidate FILE[,FILE...]
  corvint [--root PATH] runs reusable --selection FILE
    [--prior FILE[,FILE...]] [--fresh] [--summary]
  corvint [--root PATH] runs redundancy --runs FILE[,FILE...] [--since TIME]
    [--summary]
  corvint [--root PATH] runs fresh --old REV --new REV [--cite PATH[,PATH...]]
    [--summary]
  corvint [--root PATH] runs baseline record --rev REV --full FILE[,FILE...]
    --isolated FILE[,FILE...]
  corvint [--root PATH] runs baseline diff --baseline FILE --runs FILE[,FILE...]
    [--own PATH[,PATH...]] [--base REV] [--max-age N]
  corvint [--root PATH] runs response-wait --log FILE --url-contains S
    [--method M] --wait-start RFC3339 --timeout-ms N

stability pools each test (project, file, title path) across runs whose
identity inputs match: Playwright version plus the applicationRevision,
configDigest and testFileBlobs[file] a run declares under
config.metadata.corvintRunIdentity. Per pool: DETERMINISTIC (same outcome in
at least --min-runs runs, default 3, minimum 2), INTERMITTENT (outcomes differ,
or a run was flaky; differing first-error lines listed) or INSUFFICIENT with a
reason (BELOW_THRESHOLD, IDENTITY_INPUT_MISSING, IDENTITY_AMBIGUOUS,
RUN_INTERRUPTED, NOT_RUN for no result or an unexpected skip). An
intermittent test.fail() test is flagged EXPECTED_FAIL_NOT_DETERMINISTIC with
the alternatives.

Repeats and shards: each repeat of a project run with --repeat-each N
(config.projects[].repeatEach) is one observation, numbered in report order
(repeats[] in stability, k/N slots in compare); a test seen once is one plain
observation, and one seen more than once is IDENTITY_AMBIGUOUS unless it is
seen exactly N times at one reported spec location. Reports
declaring config.shard group into one logical run by shard total, Playwright
version, applicationRevision, configDigest, testFileBlobs and repeatEach (shard
total and repeatEach at most 1024), and are admitted only when shards 1..total
are each present once; otherwise the set is refused with
SHARD_INPUTS_NOT_DECLARED, SHARD_SET_INCOMPLETE or SHARD_DUPLICATE (shardSets[]
names the missing and duplicate indexes): its tests are INSUFFICIENT and
compare is INCOMPARABLE. A test in two shards of one set is ambiguous unless
repeatEach explains it (Playwright shards repeat instances).
merge-reports output is one unsharded run.

compare matches tests by the same identity across the baseline and candidate
report sets (N repeats per side give k/N stability). Each test is SAME,
CHANGED (outcome, retries, or failed assertions raised in the test file or
without a location), MISSING, NEW or AMBIGUOUS (withheld); failed assertions
raised in other files (page objects, helpers, fixtures) are listed separately
as a support difference and do not change the verdict. The overall verdict is
PARITY, DIVERGED or INCOMPARABLE with reasons (REPORT_UNREADABLE,
SHARD_INPUTS_NOT_DECLARED, SHARD_SET_INCOMPLETE, SHARD_DUPLICATE,
PLAYWRIGHT_VERSION_DIFFERS, PROJECTS_DIFFER, CONFIG_DIFFERS,
INPUTS_DIFFER_WITHIN_SIDE, REPORT_HAS_ERRORS, NO_TESTS, IDENTITY_AMBIGUOUS,
RUN_INTERRUPTED). Per report it lists worker indexes and result durations;
fixture durations and cleanup are NOT_OBSERVED in the JSON reporter.

reusable lists each test of the --selection report (for example a
--list run) as REUSABLE or MUST_RUN with a reason. A retained result's
fingerprint is the Playwright version, the project, and the declared
applicationRevision, testRevision, testFileBlobs[file] and configDigest.
EXACT_PASS: a prior run passed the test at the selection's fingerprint, and
every prior result there is a qualified pass (expected, passed, not flaky,
from an admitted run without report errors). MUST_RUN reasons:
IDENTITY_INPUT_MISSING (fields named), IDENTITY_AMBIGUOUS, NO_PRIOR_RESULT,
FINGERPRINT_CHANGED (fields named), PRIOR_NOT_PASSED, PRIOR_NOT_QUALIFIED,
and with --fresh STALE (paths named) or FRESHNESS_UNRESOLVED. --fresh adds
FRESH_PASS: a qualified pass whose fingerprint differs only in revisions,
where every differing revision pair is FRESH for the test file (located by
corvintRunIdentity.testRootPath, else config.rootDir under the repository).

redundancy tallies test minutes per run category
(config.metadata.corvintRunCategory, else UNDECLARED), ordering runs by
stats.startTime: exact re-runs (the same test at a fingerprint that passed
strictly earlier), same-file-content re-runs (same version, test-file blob,
config and project; a revision differs) and their shares. Runs before
--since (RFC 3339) are history: not counted, but their passes count as
earlier. A run without a start time, or a refused shard set, is excluded.

fresh reads Git for evidence produced at --old and read at --new: FRESH
when no changed path is in reach, STALE naming each changed path that is
cited (--cite), matches a shared pattern, or is the reach declaration
itself, else UNRESOLVED with a reason (REVISION_UNRESOLVED,
REACH_UNDECLARED, REACH_INVALID, REACH_UNREADABLE, DIFF_FAILED,
DIFF_BOUND_EXCEEDED); UNRESOLVED is never FRESH. Shared patterns come from
.corvint/evidence-reach.json at --new: {"profile":
"corvint-evidence-reach/0","shared":[...]}, repository-relative globs where
** spans directories and a trailing / covers a directory.

baseline record prints a corvint-run-baseline/0 document for the caller to
retain: the tests that failed both in the --full run and in the --isolated
re-run at --rev are known, each bound to both reports' paths and sha256; a
test that failed or was flaky in the full run only is a suspected flake.
baseline diff gates --runs on it: every unambiguous failing test is new
(NOT_IN_BASELINE, IDENTITY_AMBIGUOUS), known or flaky (SUSPECTED_FLAKE,
never excused). A known failure is excused (KNOWN_VERIFIED) only when both
recorded reports re-read with their digests and still fail it, and its file
is not under --own; otherwise EVIDENCE_MISSING, EVIDENCE_CHANGED,
EVIDENCE_PASSING or OWN_FILE. The receipt binds the baseline revision and
sha256 and lists new, known, flaky, fixed and unobserved sets with counts;
staleness is FRESH, STALE (more than --max-age commits, default 50, from the
baseline revision to the branch point of HEAD and --base, or not an
ancestor) or NOT_OBSERVED. Exit 1 when the verdict is BLOCKED.

--summary prints the compact form: counts without per-test, per-run or path
rows. Reads only the named regular files (relative to --root when given; at
most 64 per option, 64 MiB each; a baseline 16 MiB) and, for fresh, --fresh
and baseline, bounded Git reads of the repository at --root; writes nothing and uses no network. Exit 0
with JSON on stdout. Refusals: runs-invalid-arguments,
runs-report-unavailable (stability, reusable, redundancy, baseline),
runs-baseline-unavailable, runs-revision-unresolved, runs-output-unavailable.
response-wait reads a HAR or JSON-lines server request log for a wait that
timed out: REQUEST_NEVER_SENT (with up to 5 earlier 4xx/5xx requests on the
same page), SLOW_RESPONSE (duration, lateness), RESPONSE_IN_WAIT, or UNKNOWN
when the log is missing or does not cover the wait
(docs/specs/test-source-analysis-v0.md, V1-1939).
Contract: docs/specs/playwright-run-evidence-v0.md (proposed; V1-1230,
V1-1229, V1-1578, V1-1590).
`

var errRunsArgument = errors.New("runs-invalid-arguments")

func runsInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "runs" {
		return "", nil, false
	}
	return root, args[1:], true
}

func runsRefuse(diagnostic io.Writer, code string, fields map[string]any) int {
	body := map[string]any{"code": code, "ok": false}
	for k, v := range fields {
		body[k] = v
	}
	_ = json.NewEncoder(diagnostic).Encode(body)
	return 2
}

// runsFiles appends a comma-separated list of report paths, resolving relative ones against root.
func runsFiles(root string, dst *[]string) func(string) error {
	return func(v string) error {
		for _, name := range strings.Split(v, ",") {
			if name == "" || len(*dst) >= playwrightruns.MaxRuns {
				return errRunsArgument
			}
			if root != "" && !filepath.IsAbs(name) {
				name = filepath.Join(root, name)
			}
			*dst = append(*dst, name)
		}
		return nil
	}
}

func runRuns(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	switch args[0] {
	case "stability":
		return runRunsStability(root, args[1:], out, diagnostic)
	case "compare":
		return runRunsCompare(root, args[1:], out, diagnostic)
	case "reusable":
		return runRunsReusable(ctx, root, args[1:], out, diagnostic)
	case "redundancy":
		return runRunsRedundancy(root, args[1:], out, diagnostic)
	case "fresh":
		return runRunsFresh(ctx, root, args[1:], out, diagnostic)
	case "baseline":
		return runRunsBaseline(ctx, root, args[1:], out, diagnostic)
	case "response-wait":
		return runRunsResponseWait(root, args[1:], out, diagnostic)
	}
	return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
}

func runRunsStability(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs stability", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	files := []string{}
	minRuns, minSet := playwrightruns.DefaultMinRuns, false
	f.Func("runs", "Playwright JSON reports", runsFiles(root, &files))
	f.Func("min-runs", "pool size for DETERMINISTIC", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || minSet || n < playwrightruns.MinMinRuns || n > playwrightruns.MaxRuns {
			return errRunsArgument
		}
		minRuns, minSet = n, true
		return nil
	})
	if f.Parse(args) != nil || f.NArg() != 0 || len(files) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	runs := []*playwrightruns.Run{}
	for i, name := range files {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			return runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"reason": err.Error(), "run": i + 1})
		}
		runs = append(runs, run)
	}
	return runsEmit(playwrightruns.Stability(runs, minRuns), out, diagnostic)
}

func runRunsCompare(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs compare", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	baseline, candidate := []string{}, []string{}
	f.Func("baseline", "baseline Playwright JSON reports", runsFiles(root, &baseline))
	f.Func("candidate", "candidate Playwright JSON reports", runsFiles(root, &candidate))
	if f.Parse(args) != nil || f.NArg() != 0 || len(baseline) == 0 || len(candidate) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	load := func(names []string) []playwrightruns.Loaded {
		out := []playwrightruns.Loaded{}
		for _, name := range names {
			run, err := playwrightruns.ReadFile(name)
			out = append(out, playwrightruns.Loaded{Run: run, Err: err})
		}
		return out
	}
	return runsEmit(playwrightruns.Compare(load(baseline), load(candidate)), out, diagnostic)
}

func runsEmit(value any, out, diagnostic io.Writer) int {
	raw, err := json.Marshal(value)
	if err == nil {
		_, err = out.Write(append(raw, '\n'))
	}
	if err != nil {
		return runsRefuse(diagnostic, "runs-output-unavailable", nil)
	}
	return 0
}
