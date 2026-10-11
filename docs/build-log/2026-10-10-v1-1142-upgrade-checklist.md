# V1-1142: experimental offline upgrade checklist

Date: 2026-10-10. Ticket: V1-1142 / GitHub #784. Intent remains proposed, pending owner
acceptance; no decision or release promotion is made. Builder start was
`ee3ea806eec44bd238f5b0ceb0bd470802f64c0f`, preserving the existing task commits above
`origin/main` at `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`.

## Contract and evidence

The ticket-specific UCL1142 prefix avoids LPCV allocation overlap; no definition was found in
origin/main or the starting HEAD. The proposed local-build contract is `docs/specs/upgrade-checklist-v0.md:26-30@43be9517`.
The additive command/help is `cmd/corvint/changes.go:14-19@f9821b20`. Local first-parent merges and their introduced commits
produce bounded PR/issue evidence, with Refs retained as unknown: `internal/upgradechanges/changes.go:187-193@b056d865`.
Adopter files are explicit regular files, opened by descriptor-relative nofollow traversal:
`internal/upgradechanges/refs_unix.go:15-20@52ea0590`. Closure evidence is advisory; hints retain the file, line and carrying PR.

The end-to-end temp-Git fixture checks range boundaries, carried commits, closing versus Refs
keywords, unchanged repository bytes, deterministic JSON, secret screening and exact whitespace
in paths: `internal/upgradechanges/changes_test.go:67-68@7f82dba0`. Configuration isolation has a signed-commit fixture and a positive control
proving the configured verifier executes without suppression: `internal/upgradechanges/changes_test.go:208-209@7e6c6c23`.
Other focused cases cover absent/reversed builds, equal endpoints, byte/entry/history bounds,
symlinks, directories, FIFO refusal, deadline cancellation, and CLI dispatch/help/refusals.

## Independent findings and verification

Native independent review identified signature-verifier/configuration effects, reference-scan
cost and filename whitespace normalization. Repairs disable signature verification and notes,
fix UTF-8 output, index closures with bounded reference occurrences and cancellation checks,
and screen filenames without normalizing whitespace. Initial tests also caught a promoted
bytes.Buffer ReadFrom bypass, in-root symlink following and root-help budget overflow; these
were repaired. A second filesystem probe exposed macOS descriptor-path traversal limitations;
the implementation now uses the repository's existing openat pattern. The signed-commit fixture
initially failed its own canonical-header precondition; its header was corrected before claiming
coverage. Independent review and the summary-field delta re-review passed with no remaining
findings. Final focused checks passed for internal/upgradechanges (66.414s) and cmd/corvint
(1.205s); these are run observations, not performance claims. The live CLI equal-build smoke
returned the canonical empty-range JSON and explicit unknowns.

```sh
go test -count=1 -timeout 30m ./internal/upgradechanges ./cmd/corvint -run 'TestCompile|TestCappedOutput$|TestReadRefsRejectsFIFO$|TestChangesCLI$|TestRootHelp|TestHelp|TestHFlagAliasesHelpFlag$|TestInvalidChoiceNamesEveryDispatchedTopLevelVerb$|TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp$'
```

Tests used GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp and GOMAXPROCS=3.
All requested spec-requirements, requirement-definitions, traceability-tests, decision-numbers,
line-citations, error-code-ownership, unbounded-readers and use-case-receipts checks passed in the
isolated staging snapshot. Traceability reported six pre-existing planned tests, none belonging
to this slice. The separate use-case-receipts-test passed five cases, and the repin --check found
all 18 receipts current. Existing source citations shifted by dispatch/help insertion were moved
to byte-identical content with their original hashes preserved. git diff --check passed.

## Self-use and retained limits

Corvint query returned an unrelated Tasks lease spec and reported omitted results; bounded
repository source/help/spec inspection supplied the missing context. Affected planning retained
language-frontier unknowns; the owner's explicit focused-only instruction governs this builder.
The one requested index refresh and initial dogfood-change failed on sandbox writes to shared
Git metadata. Final CEM/OCM/seal is NOT_PRODUCED: this builder must not commit. The real Git index
is also outside the writable sandbox, so staging/generation/checks use an isolated index and
object directory under /private/tmp; the actual worktree index remains unstaged. Apple's mktemp
ignored TMPDIR without an explicit template, so a scratch shim supplies a template in the
allowed temporary directory. Receipt fixture tests initially inherited and overwrote the scratch
index; it was rebuilt from HEAD, all task paths were restaged, generation and index gates were
rerun, and the fixture test passed separately without that environment. The real index and
worktree were unaffected. No repository tooling was changed for these environment limits.

Installed Core is 1.0.0-rc.3 build 407; Tasks is 0.0.0-tcp01-unverified+build.407. Official updater
checks for both components failed with descendant snapshot unavailable; available versions and
freshness remain UNKNOWN, and no update verification was bypassed. Account usage, billed tokens,
cache effects and complete cost are NOT_OBSERVED. Inherited model/effort was retained for the
bounded implementation and independent review. No speed or savings claim is made.

GitHub API issue state, reopen/revert semantics, pre-range closures, glob expansion, release
metadata and per-PR flag/command diff are NOT_RUN/not delivered. Source absence or missing commit
objects refuse offline. File scanning is macOS/Linux only; Linux runtime qualification is NOT_RUN.
The repository-wide gate, publication and ticket completion write are NOT_RUN by explicit scope.
No commit, push, decision file or .taskman mutation was made. Any environment/tooling friction
filing remains for the coordinator because this builder is explicitly forbidden to write tickets.
