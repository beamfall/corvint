# V1-0408: experimental review hunk order

This entry records the original run. Later P2a/P2b findings and their repair evidence are
retained in the appended repair section below.

Intent: proposed, pending owner acceptance (V1-0408). FPK-V0-053..056 extend
`docs/specs/falsifiable-packet-v0.md`; no owner acceptance or decision is recorded.

The smallest useful slice is `internal/revieworder.Rank`, a pure projection over
already-read parsed hunks, original Git AI note bytes, a witness report and a
caller-verified CEM. Its three cited factors rank lexicographically (yes, unknown,
no), then path, new/old ranges, ID and input ordinal. Authorship remains
repository-history. Missing Git AI notes remain unknown. Known AI added lines qualify the hunk even when
other lines are unresolved; their citation retains that remaining uncertainty.
Witness path-obligation closure and CEM exercising coverage remain separate:
caller test claims cannot close an obligation, and coverage of human added lines
cannot hide an uncovered AI added line. No CLI or existing wire changes.

Limits: the caller must pin/read Git inputs and supply CEM verification for the
same base, target and patch; this library performs no Git object verification,
test execution or authority promotion. Only authorship/3.0.0 prompt/session and
explicit human attestations are recognized; absent, unsupported, conflicting and
malformed inputs remain unknown. A path-level witness does not prove line-level
test execution. Rollback removes the new internal package; no persistent state
or existing command behavior changes.

Evidence: pre-change focused tests passed for `internal/gitnotes`,
`internal/witness`, `internal/cem/patch` and `internal/cem/wire` with Go 1.27.1.
Initial projection tests passed. `TestCommitWithoutGitAINoteReportsUnknownAuthorship`
uses a real committed fixture with no Git AI note, confirms absence of the
provenance row and unknown projection authorship, and checks all repository files
(including Git state and trace/observation sentinels) remain unchanged.
Final focused tests passed for `internal/revieworder`, `internal/gitnotes`,
`internal/witness`, `internal/cem/patch` and `internal/cem/wire`. The new package
was rerun after the protocol and partial-note corrections and passed. Independent
read-only native review passed, including the final existential-authorship delta,
against SHA-256 frozen inputs; no material findings remain. The upstream standard
is pinned in the spec at git-ai-project/git-ai commit
`0670e7ef27590af0e8ff5409267f3f4b09b8fcb4`; the existing provenance fixture's
synthetic prompt key was insufficient qualification and is not reused as protocol
proof. The first doc runs failed the digest's fixed field set and then missing
ownership of two factor-detail reasons; both failures led to specific doc repairs.
The final requested doc suite is retained in
`/private/tmp/claude-501/v1-0408-doc-gates.log`; the direct receipt check passed
(all 18 current). The final full requested doc command exited zero: the
spec-requirements/index, requirement definitions, traceability tests, decision
numbers, line citations, error-code ownership, unbounded readers and use-case
receipt checks passed; the receipt regression fixture passed all five cases.
`sh script/repin-use-case-receipts.sh --check` passed directly in the task worktree.
The final evidence-note update changes only this build-log file, which is outside
these checks' semantic input sets; source/spec/test inputs remain frozen.

Sandbox verification boundary: the worktree's actual Git index is outside the
writable sandbox, so staging there failed without changing it. All task-owned
files were copied into a local shared clone of the same HEAD at
`/private/tmp/claude-501/v1-0408-doc-check`, staged there, and the generated TSV
copied back here. A scratch PATH wrapper supplies an explicit TMPDIR template for
Darwin's no-template `mktemp`, which otherwise ignored TMPDIR and tried a protected
per-user directory. The full suite uses the clone's real index and temporary
fixture repositories; no product scripts or protected paths are altered.

Corvint context: installed Core 1.0.0-rc.3 build 407; Tasks
0.0.0-tcp01-unverified+build.407. Official update checks for both components failed
`process-after-start-failed: descendant snapshot unavailable`; freshness/qualification remain UNKNOWN, and no update/verification bypass was
attempted. A separate current GitHub releases read observed published Core
`v1.0.0-rc.2` and standalone Tasks `tasks-dev-20260929.2` as the latest returned
component releases. The installed Core label is newer; neither that label nor the
unverified Tasks build establishes release qualification. No downgrade was made.
The one requested index refresh failed writing the shared Git snapshot outside
the sandbox. Query succeeded but omitted four ranked results and 38 test-path
symbol candidates; its tree identity
`84f10e86806fb395912e136b4a2e1321f7cdda78` matches the captured base tree. Its top
result is the index-snapshot spec, not the owning review-order evidence, so the
bounded ticket-named original sources supplied implementation context. `affected` on the empty diff selected no packages
and retained language-frontier unknowns; the ticket's named owning packages and
the new package determine focused checks. `make dogfood-change` at change start
failed creating its private directory in the shared Git worktree metadata outside
the sandbox. Final post-commit CEM/OCM binding and sealing are NOT_PRODUCED: the
owner explicitly forbids commits for this slice. `make gate` and repository-wide
`./...` checks are NOT_RUN by explicit owner scope. No ticket-store writes,
publication, plugin manifest edits or decision files. Native ticket closeout and
owner acceptance remain unresolved outside this patch.

Routing: one builder and one bounded independent native reviewer under the
repository's default policy; inherited session model/effort, with review focused
on uncertain source joins and evidence bindings. Runtime tuning, account usage,
billed tokens and cache telemetry are NOT_OBSERVED in the available tools; no
savings or spend-enforcement claim is made. Temporary command receipts/logs are
under `/private/tmp/claude-501/v1-0408-*`, not product state.
# V1-0408: P2a/P2b review repairs

Base: `1f5fedd18aa4c9785b5d75aea76498ab988c4dba`. Intent remains proposed and
delivery experimental under FPK-V0-053..056; these repairs do not promote the slice.
The original run is retained in `2026-10-10-review-hunk-order.md`.

P2a: the original parser accepted unquoted whitespace paths followed by valid
human attestations as definite human authorship. `TestAuthorshipPathQuoting`
reproduced `AIAuthored=no` for unquoted spaces, tabs and carriage returns before
the repair. Such paths now invalidate the note and report unknown. Quoted paths
with spaces or escaped tabs/newlines still support explicit human attribution;
malformed quotes and empty quoted paths remain unknown. Requiring an attestation
for every path also rejects an unquoted newline split into two path lines before
a human record, following the same upstream standard already pinned in the spec.
Independent review found that other leading whitespace could still yield definite
human authorship. The repair now rejects unquoted leading Unicode whitespace;
vertical-tab, form-feed and nonbreaking-space fixtures cover that finding. The
same reviewer inspected this delta and reported no remaining material findings.

P2b: the earlier ranking fixtures did not isolate Unwitnessed. The new
`TestRankUnwitnessedIsOnlyDiscriminatingFactor` fixes authorship and coverage at
yes for all three hunks, with UNPROVEN/NOT_RUN/hunk-witnessed CLOSED obligations
yielding yes/unknown/no. Path and input order oppose that priority. Temporarily
omitting Unwitnessed from both comparator tuples failed the intended order
assertion: `[a-closed.go b-not-run.go c-unproven.go]`, values `[no unknown yes]`.
The production comparator was restored byte for byte before final checks.
The mutation receipt is `/private/tmp/v1-0408-repair-unwitnessed-mutant.log`;
the pre-repair parser failures are `/private/tmp/v1-0408-repair-regression-before.log`.

Verification: the pre-change `go test ./internal/revieworder` passed, and both
new focused tests passed after the repair. Final `go test ./internal/revieworder`
passed after the review repair; its receipt is
`/private/tmp/v1-0408-repair-package-test.log`. The final doc suite exited zero:
`spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
`decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
`unbounded-readers-check`, `use-case-receipts-check` and `use-case-receipts-test`.
All 18 receipts were current and all five receipt regression cases passed. The
final receipt is `/private/tmp/v1-0408-repair-doc-gates-final.log`; the earlier
passing run before the review delta remains at `/private/tmp/v1-0408-repair-doc-gates.log`.
`git diff --check` passed. Rollback reverts the parser changes and new fixtures;
there is no wire/CLI or persistent-state change.

The doc gates ran on a shared, sparse scratch checkout at the exact base,
`/private/tmp/v1-0408-repair-doc-check`, with `.taskman` excluded from checkout and
only the task-owned source/spec/log inputs staged. This preserves the original
worktree's protected Git index and store. The prior Darwin `mktemp` PATH wrapper
was reused without changing product scripts. Requirement locators were regenerated
there and copied back. Source/spec/test/locator hashes are frozen in
`/private/tmp/v1-0408-repair-verified-inputs.json`; this final evidence-note update
changes no source/spec/test input or doc-gate semantic input.

Corvint routes used: authority-start query and pre-change affected receipt retained
at `/private/tmp/v1-0408-repair-{context,affected-before}.json`. The query's first
result did not own this slice and omitted four ranked results; the supplied paths
and owning spec provided bounded fallback. Empty-diff affected advice selected no
tests and retained language-frontier uncertainty. Core remains 1.0.0-rc.3 build
407; Tasks remains 0.0.0-tcp01-unverified+build.407. Both official updater checks
failed `process-after-start-failed: descendant snapshot unavailable`, so freshness
is UNKNOWN. Index refresh failed writing the shared Git metadata outside this
sandbox; no verification bypass or downgrade was attempted.
Post-change affected advice includes `internal/revieworder` and retains broad
language/build-variant and unowned-documentation frontiers; it is retained at
`/private/tmp/v1-0408-repair-affected-after.json`, not claimed as coverage proof.

Routing: one builder and one bounded independent native reviewer, using inherited
session model/effort for this local correctness repair. Minions tools were not
available. No nested delegation or runtime tuning was needed.

Post-commit CEM/OCM/sealing remains NOT_PRODUCED because the owner forbids commits.
The repository-wide gate remains NOT_RUN under the explicit focused scope.
No `.taskman` writes, commit, push or publication is authorized for this repair.
Runtime tuning, account usage, billed tokens and cache telemetry remain NOT_OBSERVED.
