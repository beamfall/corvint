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
	"github.com/Beamfall/corvint/internal/runidentity"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const runsHelp = `Read Playwright JSON reports across runs (experimental, PWR-V0).

Usage:
  corvint [--root PATH] runs stability --runs FILE[,FILE...] [--runs ...]
    [--min-runs N] [--last N [--window all|completed]] [--pool-by inputs|closure]
    [--identity REPORT=FILE...] [--identity-sidecars]
  corvint [--root PATH] runs compare --baseline FILE[,FILE...]
    --candidate FILE[,FILE...]
    [--identity REPORT=FILE...] [--identity-sidecars]
  corvint [--root PATH] runs reusable --selection FILE
    [--prior FILE[,FILE...]] [--fresh] [--summary]
    [--identity REPORT=FILE...] [--identity-sidecars]
  corvint [--root PATH] runs redundancy --runs FILE[,FILE...] [--since TIME]
    [--summary]
    [--identity REPORT=FILE...] [--identity-sidecars]
  corvint [--root PATH] runs fresh --old REV --new REV [--cite PATH[,PATH...]]
    [--summary]
  corvint [--root PATH] runs baseline record --rev REV --full FILE[,FILE...]
    --isolated FILE[,FILE...] [--allow-unobserved]
    [--identity REPORT=FILE...] [--identity-sidecars]
  corvint [--root PATH] runs baseline diff --baseline FILE --runs FILE[,FILE...]
    [--own PATH[,PATH...]] [--base REV] [--max-age N]
    [--identity REPORT=FILE...] [--identity-sidecars]
  corvint [--root PATH] runs response-wait --log FILE --url-contains S
    [--method M] --wait-start RFC3339 --timeout-ms N
  corvint [--root PATH] runs server-errors --runs FILE[,FILE...]
    --server-log FILE [--requests FILE] [--support-path P[,P...]]
    [--window-ms N]
  corvint [--root PATH] runs identity --config FILE [--test-dir DIR]
    [--app-root DIR] [--test-match REGEXP] [--project-test-dir DIR[:REGEXP]...]
    [--artifact-digest VALUE] [--closure-digests FILE] [--compute-closures]

stability pools each test (project, file, title path) across runs whose
identity inputs match: Playwright version plus the applicationRevision,
configDigest, testFileBlobs[file] and servedArtifactDigest a run declares under
config.metadata.corvintRunIdentity. Each runs[] entry states servedDigest:
DECLARED, UNKNOWN (served digest unknown: the revision is trusted unverified;
counted in summary.servedDigestUnknownRuns) or DISAGREES (another run at the
same applicationRevision served a different build, so one of them was not
deployed; such runs never pool together). Per pool: DETERMINISTIC (same outcome in
at least --min-runs runs, default 3, minimum 2), INTERMITTENT (outcomes differ,
or a run was flaky; differing first-error lines listed) or INSUFFICIENT with a
reason (BELOW_THRESHOLD, IDENTITY_INPUT_MISSING, IDENTITY_AMBIGUOUS,
RUN_INTERRUPTED, NOT_RUN for no result or an unexpected skip);
IDENTITY_INPUT_MISSING names missingInputs (applicationRevision,
configDigest, testFileBlobs[file]; a ../ file usually needs
runs identity --project-test-dir). An
intermittent test.fail() test is flagged EXPECTED_FAIL_NOT_DETERMINISTIC with
the alternatives. A report with no corvintRunIdentity at all is named once in
runs[] (identityInputs NOT_DECLARED, unpooledTests, remedy) and counted in
summary.identityNotDeclaredRuns; its tests are not listed one by one.
--pool-by closure keys pools on Playwright version, applicationRevision,
servedArtifactDigest, testClosureRule and testClosureDigests[file] in place of
configDigest and testFileBlobs[file], so an unrelated tracked edit no longer
splits pools; runs[].closureSource reads DECLARED, COMPUTED (rule
corvint-test-closure/0 from runs identity --compute-closures) or NOT_DECLARED
(summary.closureNotDeclaredRuns), and a test without a closure digest is
IDENTITY_INPUT_MISSING. A DECLARED and a COMPUTED digest never pool together. Two reports declaring one closure digest for a file
over different test-file blobs refuse runs-report-unavailable
(CLOSURE_DIGEST_CONTRADICTED, option pool-by).
A test keeps one row: an interrupted or not-run observation at declared
inputs where the test also pooled is listed under that entry's excluded[]
(run, repeat, reason) and not counted; runs[].interrupted marks a report
cut short. --last N pools only the last N logical runs by stats.startTime
(others are runs[].outsideWindow); the report's window names the rule:
all (default) counts an interrupted run toward N and lists such runs in
windowInterruptedRuns, completed (--window, needs --last) skips interrupted
runs and marks them outsideWindow; a report without a start time refuses
runs-report-unavailable (START_TIME_MISSING).

identity prints a corvint-run-identity/0 document whose corvintRunIdentity
member is the declaration to embed under config.metadata (for example
metadata: { corvintRunIdentity: JSON.parse(execFileSync("corvint", ["runs",
"identity", "--config", __filename])).corvintRunIdentity }). Rules: test
files are the files under --test-dir (the config's top-level testDir, which
is config.rootDir; default the config's directory; never a project testDir)
whose testDir-relative path matches --test-match (default Playwright's
testMatch, node_modules excluded), plus, per repeatable --project-test-dir
DIR[:REGEXP] (a project's own testDir; REGEXP defaults to --test-match), the
files under DIR whose DIR-relative path matches; testFileBlobs maps each such
path, spelled as the report's file (relative to --test-dir, so a project
testDir beside it gives ../setup/x.setup.ts), to its working-tree Git blob
id. projectTestDirs[] states each project testDir: RESOLVED with its file
count, or UNRESOLVED with limit PROJECT_TEST_DIR_UNRESOLVED (not a directory)
or PROJECT_TEST_DIR_NO_TEST_FILES; one outside the repository refuses.
--artifact-digest VALUE (1-256 bytes of letters, digits and ._:+/=@-), the
digest of the build the browser loads as the deploy step reports it (an
image digest, or sha256 of the served entry bundles computed by the
caller), is declared as servedArtifactDigest and servedDigest reads
DECLARED; without it servedDigest reads UNKNOWN. Corvint fetches nothing.
--closure-digests FILE ({"<test file>": "sha256:<64 hex>", ...}, each key a
testFileBlobs path) declares caller-computed per-test closures as
testClosureDigests with testClosureRule DECLARED; Corvint does not verify
what a closure covers. A malformed file or an unknown key refuses
(CLOSURE_DECLARATION_INVALID). --compute-closures computes each test's
closure from the config's static TypeScript graph (testClosureRule
corvint-test-closure/0): the configDigest lines plus the test-file lines,
less the files the test provably cannot load (TypeScript files only another
test's imports reach; a modified file is never left out). A test whose
closure is not static (dynamic import, unresolved config, an untracked
closure file) gets no digest and is listed under closure.unknown (limit
TEST_CLOSURE_UNKNOWN; a reason shared by several tests is also stated once in closure.sharedCauses); a frontier outside the suite leaves nothing out
(closure.widened, limit TEST_CLOSURE_EXCLUSION_WIDENED). --closure-digests
wins over it (closure.source DECLARED).
configDigest is sha256 over "mode blob<TAB>path" of every tracked non-test
file of the config's repository at working-tree content, plus the config
file and its path; untracked files are outside it (limit UNTRACKED_FILES_OUTSIDE_DIGEST).
applicationRevision is the HEAD commit of --app-root (default the config's
repository), suffixed +worktree.sha256:<diff digest> when it has tracked
uncommitted changes (limit APPLICATION_WORKTREE_MODIFIED). testRevision and
testRootPath are the config repository's HEAD and testDir path. Refusal:
runs-identity-unavailable with a reason (CONFIG_UNREADABLE, NOT_A_REPOSITORY,
TEST_DIR_OUTSIDE_REPOSITORY, TEST_MATCH_INVALID, TEST_REVISION_UNRESOLVED,
APPLICATION_REVISION_UNRESOLVED, UNMERGED_PATHS, SUBMODULE_MODIFIED,
INDEX_FLAGS_SET, FILTER_CONFIGURED, PATH_UNREPRESENTABLE, NO_TEST_FILES,
GIT_READ_FAILED, PROJECT_TEST_DIR_OUTSIDE_REPOSITORY); both checkouts are validated first.

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
PARITY, DIVERGED, UNDEPLOYED or INCOMPARABLE with reasons (REPORT_UNREADABLE,
SHARD_INPUTS_NOT_DECLARED, SHARD_SET_INCOMPLETE, SHARD_DUPLICATE,
PLAYWRIGHT_VERSION_DIFFERS, PROJECTS_DIFFER, CONFIG_DIFFERS,
INPUTS_DIFFER_WITHIN_SIDE, REPORT_HAS_ERRORS, NO_TESTS, IDENTITY_AMBIGUOUS,
RUN_INTERRUPTED). UNDEPLOYED (after INCOMPARABLE, before DIVERGED; test rows
still listed) means every report declares one servedArtifactDigest per side
and either SERVED_DIGEST_UNCHANGED (the candidate served the baseline's build
at a different applicationRevision) or SERVED_DIGEST_DIFFERS_AT_REVISION
(same revision, different builds). A report without one adds the unknown
SERVED_DIGEST_UNKNOWN. Per report it lists worker indexes and result durations;
fixture durations and cleanup are NOT_OBSERVED in the JSON reporter.

reusable lists each test of the --selection report (for example a
--list run) as REUSABLE or MUST_RUN with a reason. A retained result's
fingerprint is the Playwright version, the project, and the declared
applicationRevision, testRevision, testFileBlobs[file], configDigest and
servedArtifactDigest (when either side declares one).
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
An interrupted or not-run occurrence is never failure evidence. A full run
leaving a test unobserved (every occurrence interrupted or not run) refuses
runs-baseline-unavailable (FULL_RUN_INCOMPLETE, unobserved count) unless
--allow-unobserved records it under unobserved[] with its reason; it is
never recorded as passing, and a candidate failure of it is new
(UNOBSERVED_AT_BASELINE, listed in unobservedAtBaseline).
baseline diff gates --runs on it: every unambiguous failing test is new
(NOT_IN_BASELINE, IDENTITY_AMBIGUOUS), known or flaky (SUSPECTED_FLAKE,
never excused). A known failure is excused (KNOWN_VERIFIED) only when both
recorded reports re-read with their digests and still fail it, and its file
is not under --own; otherwise EVIDENCE_MISSING, EVIDENCE_CHANGED,
EVIDENCE_PASSING or OWN_FILE. The receipt binds the baseline revision and
sha256 and lists new, known, flaky, fixed and unobserved sets with counts;
staleness is FRESH, STALE (more than --max-age commits, default 50, from the
baseline revision to the branch point of HEAD and --base, or not an
ancestor: BASELINE_NOT_ANCESTOR adds mergeBase, baselineSideCommits and
branchSideCommits) or NOT_OBSERVED. Exit 1 when the verdict is BLOCKED.

Identity sidecars (stability, compare, reusable, redundancy, baseline record
and diff): --identity REPORT=FILE (repeatable; REPORT must be one of the
command's reports) or --identity-sidecars (REPORT.identity.json beside each
report, skipped when absent) binds {"profile":
"corvint-run-identity-sidecar/0","reportSha256":"<report sha256>",
"corvintRunIdentity":{...}} to a report that does not embed a declaration.
A sidecar never overrides an embedded one; runs[].identitySource reads
EMBEDDED, SIDECAR or NOT_DECLARED. Refusal runs-report-unavailable
(IDENTITY_SIDECAR_UNREADABLE, IDENTITY_SIDECAR_BOUND_EXCEEDED,
IDENTITY_SIDECAR_INVALID, IDENTITY_SIDECAR_MISMATCH, IDENTITY_SIDECAR_CONFLICT).

--summary prints the compact form: counts without per-test, per-run or path
rows. Reads only the named regular files (relative to --root when given; at
most 64 per option, 64 MiB each; a baseline 16 MiB) and, for fresh, --fresh
and baseline, bounded Git reads of the repository at --root (identity: of
the named checkouts); writes nothing and uses no network. Exit 0
with JSON on stdout. Refusals: runs-invalid-arguments (with reason
SUBCOMMAND_MISSING, SUBCOMMAND_UNKNOWN, OPTION_UNKNOWN (with hint OPTION_VALUE_JOINED when an option and its value arrived as one argument), VALUE_MISSING,
VALUE_INVALID, ARGUMENT_UNEXPECTED or OPTION_REQUIRED, and the option when
known; values are never echoed),
runs-report-unavailable (stability, reusable, redundancy, baseline),
runs-baseline-unavailable, runs-revision-unresolved, runs-output-unavailable,
runs-identity-unavailable.
response-wait reads a HAR or JSON-lines server request log for a wait that
timed out: REQUEST_NEVER_SENT (with up to 5 earlier 4xx/5xx requests on the
same page), SLOW_RESPONSE (duration, lateness), RESPONSE_IN_WAIT, or UNKNOWN
when the log is missing or does not cover the wait
(docs/specs/test-source-analysis-v0.md, V1-1939).
server-errors attaches, to each failed attempt whose error names an HTTP 5xx,
the server-error-log/0 record matching its method, path, status and time
window (plus request id when both carry one): exception class, screened
message and first application frame. A frame under --support-path reads
environment/fixture; no match reads "server error not found in log"; several
distinct matches or a missing startTime read UNKNOWN. Repeats group across
runs. Refusals add runs-server-log-unavailable, runs-request-log-unavailable
(docs/specs/observed-ui-evidence-v0.md).
Contract: docs/specs/playwright-run-evidence-v0.md (proposed; V1-1230,
V1-1229, V1-1578, V1-1590).
`

var errRunsArgument = errors.New("runs-invalid-arguments")

// runs-invalid-arguments reasons (PWR-V0-059, GitHub #1896).
const (
	runsArgSubcommandMissing = "SUBCOMMAND_MISSING"
	runsArgSubcommandUnknown = "SUBCOMMAND_UNKNOWN"
	runsArgOptionUnknown     = "OPTION_UNKNOWN"
	runsArgValueMissing      = "VALUE_MISSING"
	runsArgValueInvalid      = "VALUE_INVALID"
	runsArgUnexpected        = "ARGUMENT_UNEXPECTED"
	runsArgOptionRequired    = "OPTION_REQUIRED"
)

// runsNeed is one post-parse requirement: when reason is not "", option fails it.
type runsNeed struct{ option, reason string }

// runsRequired is the requirement that option was given.
func runsRequired(option string, missing bool) runsNeed {
	if missing {
		return runsNeed{option, runsArgOptionRequired}
	}
	return runsNeed{}
}

// runsInvalid refuses runs-invalid-arguments naming the reason and, when known, the option
// (PWR-V0-059). The value given is never echoed.
func runsInvalid(diagnostic io.Writer, option, reason string) int {
	fields := map[string]any{"reason": reason}
	if option != "" {
		fields["option"] = option
	}
	return runsRefuse(diagnostic, "runs-invalid-arguments", fields)
}

// runsParse parses args and checks needs in order; on the first failure it writes the
// runs-invalid-arguments refusal with its option and reason and returns false (PWR-V0-059).
func runsParse(f *flag.FlagSet, args []string, diagnostic io.Writer, needs func() []runsNeed) bool {
	if err := f.Parse(args); err != nil {
		if option := runsJoinedOption(f, err); option != "" {
			runsRefuse(diagnostic, "runs-invalid-arguments", map[string]any{"reason": runsArgOptionUnknown, "option": option, "hint": runsHintValueJoined})
			return false
		}
		option, reason := runsFlagError(f, err)
		runsInvalid(diagnostic, option, reason)
		return false
	}
	if f.NArg() != 0 {
		runsInvalid(diagnostic, "", runsArgUnexpected)
		return false
	}
	if needs != nil {
		for _, n := range needs() {
			if n.reason != "" {
				runsInvalid(diagnostic, n.option, n.reason)
				return false
			}
		}
	}
	return true
}

// runsHintValueJoined marks an unknown argument that starts with a known option name, a space and
// a value: the option and its value were passed as one argv element (PWR-V0-176, GitHub #1918).
const runsHintValueJoined = "OPTION_VALUE_JOINED"

// runsJoinedOption is the defined option an unknown argument starts with when the rest follows
// whitespace, else "". Only the option name is returned; the value is never echoed.
func runsJoinedOption(f *flag.FlagSet, err error) string {
	name, ok := strings.CutPrefix(err.Error(), "flag provided but not defined: ")
	if !ok {
		return ""
	}
	name = strings.TrimLeft(name, "-")
	if i := strings.IndexAny(name, " \t\n"); i > 0 && f.Lookup(name[:i]) != nil {
		return name[:i]
	}
	return ""
}

// runsFlagError maps a flag package parse error to the option it names and a reason. The option
// is reported only when it is a defined flag, or an unknown one spelled as a plain option name.
func runsFlagError(f *flag.FlagSet, err error) (string, string) {
	msg := err.Error()
	named := func(prefix string) string {
		name := strings.TrimLeft(strings.TrimPrefix(msg, prefix), "-")
		if f.Lookup(name) != nil || runsPlainOption(name) {
			return name
		}
		return ""
	}
	switch {
	case strings.HasPrefix(msg, "flag provided but not defined: "):
		return named("flag provided but not defined: "), runsArgOptionUnknown
	case strings.HasPrefix(msg, "flag needs an argument: "):
		return named("flag needs an argument: "), runsArgValueMissing
	case strings.HasPrefix(msg, "invalid value ") || strings.HasPrefix(msg, "invalid boolean value "):
		// "invalid value %q for flag -NAME: ERR" or "invalid boolean value %q for -NAME: ERR";
		// the value is quoted, so the last marker is the real one.
		for _, marker := range []string{" for flag -", " for -"} {
			if i := strings.LastIndex(msg, marker); i >= 0 {
				rest := msg[i+len(marker):]
				if j := strings.Index(rest, ": "); j >= 0 && f.Lookup(rest[:j]) != nil {
					return rest[:j], runsArgValueInvalid
				}
			}
		}
		return "", runsArgValueInvalid
	}
	return "", runsArgOptionUnknown
}

// runsPlainOption reports whether an undefined option name may be echoed: a short plain name
// that the secret screen does not match.
func runsPlainOption(name string) bool {
	if name == "" || len(name) > 40 || secretscreen.MatchString(name) {
		return false
	}
	for _, c := range name {
		if !(c == '-' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')) {
			return false
		}
	}
	return true
}

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
		return runsInvalid(diagnostic, "", runsArgSubcommandMissing)
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
	case "server-errors":
		return runRunsServerErrors(root, args[1:], out, diagnostic)
	case "identity":
		return runRunsIdentity(ctx, root, args[1:], out, diagnostic)
	}
	return runsInvalid(diagnostic, "", runsArgSubcommandUnknown)
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
	last := 0
	f.Func("last", "pool only the last N logical runs by stats.startTime", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || last != 0 || n < 1 || n > playwrightruns.MaxRuns || strconv.Itoa(n) != v {
			return errRunsArgument
		}
		last = n
		return nil
	})
	window := ""
	f.Func("window", "all (default) or completed: --last N skips interrupted runs (PWR-V0-175)", func(v string) error {
		if window != "" || (v != playwrightruns.WindowAll && v != playwrightruns.WindowCompleted) {
			return errRunsArgument
		}
		window = v
		return nil
	})
	poolBy := ""
	f.Func("pool-by", "inputs (default) or closure: pool on declared per-test closure digests (PWR-V0-069)", func(v string) error {
		if poolBy != "" || (v != playwrightruns.PoolByInputs && v != playwrightruns.PoolByClosure) {
			return errRunsArgument
		}
		poolBy = v
		return nil
	})
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("runs", len(files) == 0), runsRequired("last", window != "" && last == 0), ident.need(files)}
	}) {
		return 2
	}
	runs := []*playwrightruns.Run{}
	for i, name := range files {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			return runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"reason": err.Error(), "run": i + 1})
		}
		if !ident.apply("runs", i+1, name, run, diagnostic) {
			return 2
		}
		// A window is ordered by start time, so a report without one cannot be placed (PWR-V0-063).
		if last > 0 && !run.HasStartTime {
			return runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"option": "last", "reason": "START_TIME_MISSING", "run": i + 1})
		}
		runs = append(runs, run)
	}
	if poolBy == playwrightruns.PoolByClosure {
		// A closure contains its test file, so the same closure over two test-file blobs is a false
		// claim (PWR-V0-070).
		if run, earlier, contradicted := playwrightruns.ClosureContradiction(runs); contradicted {
			return runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"option": "pool-by", "reason": "CLOSURE_DIGEST_CONTRADICTED", "run": run, "conflictsWith": earlier})
		}
	}
	if poolBy == "" {
		poolBy = playwrightruns.PoolByInputs
	}
	return runsEmit(playwrightruns.StabilityWindowed(runs, minRuns, last, poolBy, window), out, diagnostic)
}

func runRunsCompare(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs compare", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	baseline, candidate := []string{}, []string{}
	f.Func("baseline", "baseline Playwright JSON reports", runsFiles(root, &baseline))
	f.Func("candidate", "candidate Playwright JSON reports", runsFiles(root, &candidate))
	ident := runsIdentityFlags(f, root)
	if !runsParse(f, args, diagnostic, func() []runsNeed {
		return []runsNeed{runsRequired("baseline", len(baseline) == 0), runsRequired("candidate", len(candidate) == 0), ident.need(baseline, candidate)}
	}) {
		return 2
	}
	load := func(option string, names []string) ([]playwrightruns.Loaded, bool) {
		out := []playwrightruns.Loaded{}
		for i, name := range names {
			run, err := playwrightruns.ReadFile(name)
			if err == nil && !ident.apply(option, i+1, name, run, diagnostic) {
				return nil, false
			}
			out = append(out, playwrightruns.Loaded{Run: run, Err: err})
		}
		return out, true
	}
	base, ok := load("baseline", baseline)
	if !ok {
		return 2
	}
	cand, ok := load("candidate", candidate)
	if !ok {
		return 2
	}
	return runsEmit(playwrightruns.Compare(base, cand), out, diagnostic)
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

// runRunsIdentity prints the corvintRunIdentity declaration for a Playwright config
// (PWR-V0-046..PWR-V0-050).
func runRunsIdentity(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs identity", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	opts := runidentity.Options{}
	closureFile := ""
	once := func(dst *string, resolve bool) func(string) error {
		return func(v string) error {
			if v == "" || *dst != "" {
				return errRunsArgument
			}
			if resolve && root != "" && !filepath.IsAbs(v) {
				v = filepath.Join(root, v)
			}
			*dst = v
			return nil
		}
	}
	f.Func("config", "Playwright config file", once(&opts.Config, true))
	f.Func("test-dir", "Playwright top-level testDir (config.rootDir)", once(&opts.TestDir, true))
	f.Func("app-root", "application checkout", once(&opts.AppRoot, true))
	f.Func("test-match", "test file regular expression", once(&opts.TestMatch, false))
	f.Func("artifact-digest", "digest of the served build (PWR-V0-054)", func(v string) error {
		if opts.ArtifactDigest != "" || !runidentity.ValidArtifactDigest(v) {
			return errRunsArgument
		}
		opts.ArtifactDigest = v
		return nil
	})
	f.Func("closure-digests", "JSON file of caller-declared per-test closure digests (PWR-V0-071)", func(v string) error {
		if v == "" || opts.ClosureDigests != nil {
			return errRunsArgument
		}
		if root != "" && !filepath.IsAbs(v) {
			v = filepath.Join(root, v)
		}
		closureFile = v
		opts.ClosureDigests = map[string]string{}
		return nil
	})
	f.BoolVar(&opts.ComputeClosures, "compute-closures", false, "compute per-test closure digests (PWR-V0-072)")
	f.Func("project-test-dir", "project testDir DIR[:REGEXP], repeatable", func(v string) error {
		dir, match, ok := splitProjectTestDir(v)
		if !ok {
			return errRunsArgument
		}
		if root != "" && !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		opts.ProjectTestDirs = append(opts.ProjectTestDirs, runidentity.ProjectTestDir{Dir: dir, TestMatch: match})
		return nil
	})
	if !runsParse(f, args, diagnostic, func() []runsNeed { return []runsNeed{runsRequired("config", opts.Config == "")} }) {
		return 2
	}
	if closureFile != "" {
		// The file is read bounded and regular-only, as a sidecar is (PWR-V0-071).
		raw, err := playwrightruns.ReadSidecarFile(closureFile)
		if err == nil {
			opts.ClosureDigests, err = runidentity.ParseClosureDigests(raw)
		}
		if err != nil {
			return runsRefuse(diagnostic, "runs-identity-unavailable", map[string]any{"reason": runidentity.ReasonClosureInvalid, "option": "closure-digests"})
		}
	}
	doc, err := runidentity.Produce(ctx, opts)
	if err != nil {
		var coded *runidentity.Error
		if errors.As(err, &coded) {
			return runsRefuse(diagnostic, "runs-identity-unavailable", map[string]any{"reason": coded.Reason, "detail": coded.Detail})
		}
		return runsRefuse(diagnostic, "runs-identity-unavailable", map[string]any{"reason": runidentity.ReasonGitReadFailed})
	}
	return runsEmit(doc, out, diagnostic)
}

// splitProjectTestDir splits DIR[:REGEXP] at the first colon after a Windows drive prefix; an
// empty DIR or an empty REGEXP after a colon is invalid (PWR-V0-052).
func splitProjectTestDir(v string) (dir, match string, ok bool) {
	skip := 0
	if len(v) >= 3 && v[1] == ':' && (v[2] == '\\' || v[2] == '/') && ('a' <= v[0]|0x20 && v[0]|0x20 <= 'z') {
		skip = 2
	}
	i := strings.IndexByte(v[skip:], ':')
	if i < 0 {
		return v, "", v != ""
	}
	dir, match = v[:skip+i], v[skip+i+1:]
	return dir, match, dir != "" && match != ""
}
