# V1-0665: serialize in-place CEM reads before publication

Intent: CEM-PILOT-013 amendment is proposed, pending owner acceptance (V1-0665).
The existing numbered requirement owns bounded safe reads and serialized map updates;
origin/main and the open beamfall/corvint PR list were checked before amending it. No
new requirement ID or decision file is introduced.

The pre-lock map read could capture the old inode and then open a concurrent mark's
atomic replacement. Its safe-reader refusal was correct, but a valid in-place mark
failed before reaching serialization. In-place updates now lock before their first
read, including an explicit output equal to the input path. Separate outputs retain
input-validation precedence and lock their effective destination. On a lock refusal,
input errors still take precedence. Identity, size, type, no-follow, and bounded reads
remain enforced. Rollback is reverting the lock-order change and its read-only test seam.

`TestMarkReplacementAtFirstRead` initializes and validates a canonical two-hunk map,
then coordinates a real second `Mark` between the observer's metadata capture and
safe open under the old ordering. The regression failed for omitted and explicit same-path output (and a forbidden dot-path fixture)
with `map-unavailable: ... changed while being read` before the fix. The fixed ordering
lets that writer complete before metadata capture; both distinct marks survive.
`TestConcurrentMarksAllSurvive` retains the twelve-writer outcome. The per-root
`BeforeReadOpen` callback does not replace the opener or bypass any read validation.

Independent read-only review confirmed the real replacement and preserved checks,
and requested mutation-specific hostile-input precedence coverage. That coverage was
added in `TestMarkHostileInputPrecedence`: a symlinked parent, malformed map with an
occupied lock, and valid map with an occupied lock, including unchanged input/lock bytes.

Verification: all requested make targets passed in the disposable staged clone:
`spec-requirements-check requirement-definitions-check traceability-tests-check
 decision-numbers-check line-citations-check error-code-ownership-check
 unbounded-readers-check use-case-receipts-check use-case-receipts-test`.
`sh script/repin-use-case-receipts.sh --check` passed: all eighteen receipts are current.
`git diff --check` passed. The first repeated race run failed on a mistaken success fixture with an explicit
`.corvint/./change.cem.json` output: the existing path contract forbids dot segments.
That failure is retained in `/private/tmp/v10665-focused-final.log`; it is not a
safe-read failure after the fix. The success matrix and proposed amendment were
narrowed to valid omitted and explicit same-path output. Corrected focused race tests passed with `-race -count=3 -timeout 30m`
for `TestMarkReplacementAtFirstRead`, `TestMarkHostileInputPrecedence`,
`TestConcurrentMarksAllSurvive`, `TestMutationsLockTheirEffectiveOutput`, and
`TestMutationLockNeverRemovesAnInputMap` (146.366s). The failed retry command
in `/private/tmp/v10665-corrected.log` was an unquoted PATH containing spaces;
the successful invocation is retained in `/private/tmp/v10665-corrected-final.log`.
Builder/reviewer used the inherited model and effort for the bounded concurrency
repair; live controls and billed usage remain NOT_OBSERVED.

Corvint use: pre-change query and affected receipts retained in `/private/tmp/v10665-query.json`
and `/private/tmp/v10665-affected.json`. Query retained omitted results and withheld test
candidates; affected scope remains UNKNOWN with non-Go/source/graph exclusions. The
requested focused tests and checks govern this patch; `make gate` and repository-wide
Go tests are NOT_RUN by explicit owner instruction. Installed Core is 1.0.0-rc.3 build 407;
Tasks is 0.0.0-tcp01-unverified+build.407. Official update checks failed their version
smoke (`descendant snapshot unavailable`), so available versions/freshness are UNKNOWN.
No update verification was bypassed. Account usage telemetry was NOT_OBSERVED.

Sandbox limits: the shared worktree Git index and derived index directory are outside
writable roots. Original `git add`, index refresh, and dogfood-change failed with
operation-not-permitted. Index-based verification uses a disposable clone of the same
HEAD with the exact changed files staged; its generated REQUIREMENTS.tsv is copied back.
The local Apple Git launcher polluted fixture revision output with denied xcrun cache
writes; using `/Library/Developer/CommandLineTools/usr/bin/git` directly resolved that
fixture-only blocker. A temporary mktemp wrapper directs default temporary files to
the requested TMPDIR. No repository tooling was changed for these environment limits.

Final immutable CEM binding/sealing and native ticket completion are NOT_PRODUCED:
the owner explicitly forbids commits and ticket-store writes in this task. No push,
PR, decision file, or plugin manifest change is made.

Final publication and CLI regressions passed with `-race -count=1 -timeout 30m`
for `./internal/cem/publish` (1.826s) and `./internal/cem/cli` (5.257s).
The earlier broader three-package `-race -count=5` invocation was interrupted
after the corrected focused result superseded its invalid fixture; it is not
claimed as a passing run. Its owned exec session completed with exit 1.

After the fixture correction, the complete requested make check sequence passed
again against the matching staged source/spec snapshot. The final receipt repin
check passed with all eighteen receipts current. Original and verification-clone
bytes matched for both spec files and all three changed Go files. The subsequent
log-only result addendum receives direct readback and diff validation.
