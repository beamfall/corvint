# Dogfood wrapper cache selection (V1-0693)

Intent: `DCW-V0-038` in `docs/specs/daily-change-evidence-workflow-v0.md` is
proposed, pending owner acceptance (V1-0693). No owner acceptance or decision is recorded.
Base: `29e1f653486fca13216606683d3a5c2b05cacd36`.

The wrappers now select the first nonempty `CORVINT_GOCACHE`, caller `GOCACHE`,
then the existing `/tmp/corvint-go-build-cache` default. Make captures the incoming
choices before its general cache default and forwards them only in dogfood targets.
Change's current-tree build and check's tree/base builds use the same selection;
seal inherits it through check. Verifier argv, source/archive selection, version
checks and strict verification are unchanged. Rollback is to revert these wrapper,
Make and documentation edits; no cache removal or migration is required.

Focused evidence: `script/dogfood-cache_test.sh` (`DCW-V0-038`) records build
cache/toolchain/source/argv through a fake Go executable in a disposable Git fixture.
It covers direct and Make change/check/seal, each build role, spaces, empty values,
precedence, Make command-line input, archived-base source and trimpath parity,
version mismatch, valid override and both build-failure refusals. No actual Go
cache writes are exercised by this test. Verification results are recorded below.

The prior scope gap remains open: earlier private-cache declarations did not
contain these wrapper builds because they forced the shared cache. This patch
makes no retroactive containment claim, does not establish which prior runs wrote
which caches, and does not qualify unrelated build paths. No existing cache was
cleaned. Real cache writes and prior containment remain NOT_OBSERVED.

Independent native review identified a global Make-default scope regression;
selection was narrowed to dogfood targets before final verification. The reviewer
otherwise found no wrapper/cache-test correctness issue. The initial local test
also exposed Bash 3.2's empty-array nounset behavior; optional Make argv now uses
the existing wrappers' guarded expansion convention. macOS `wc -l` padding (`       2`) also requires a numeric count comparison.
The negative control then exposed Bash 3.2 continuing after failed `[[ ]] loop
assertions despite errexit; a minimal shell reproducer confirmed it. Every test
assertion now explicitly exits on failure. Earlier shell PASS observations are
invalidated; final positive/negative runs retain explicit exit observations.

Corvint routes used: query and baseline/dirty affected receipts are retained in
`/private/tmp/v10693-context.txt`, `v10693-affected-baseline.json` and
`v10693-affected.json`. Query selected an unrelated spec with omissions, so the
owning spec was located from the named wrapper path. Affected advice retains its
UNKNOWN frontier and cannot establish complete shell test coverage. The host event
reported `corvint-event-rejected:dogfood-event-deadline`. CEM/OCM binding, frozen
post-commit dogfood checks and seal are NOT_RUN because this builder is explicitly
forbidden to commit. Broad gates and `./...` were explicitly excluded by the owner.
No completion, publication or native ticket-store write is claimed.

Tool check: installed Core `1.0.0-rc.3 (build 407)` and Tasks
`0.0.0-tcp01-unverified+build.407` (`verification: NOT_RUN`). Official update
check with network failed `version smoke failed: process-after-start-failed:
descendant snapshot unavailable`; available releases/freshness remain UNKNOWN,
and no update or verification bypass was attempted. Account usage telemetry was
not available in this session; cost is NOT_OBSERVED.

The managed filesystem denies writes to this worktree's Git metadata in the
owning checkout. A private copied index and private object directory under
`/private/tmp/v10693-*` stage the spec before `make -s spec-requirements`, then
stage the generated TSV and check inputs; the real index is untouched. The macOS
no-argument `mktemp` used by doc gates selected a denied system temporary path;
a scratch PATH shim gives that invocation an explicit template in the supplied
TMPDIR. This changes only the local gate environment, not repository tooling.

Verification: Go reports `go1.27.1 darwin/arm64`. Shell syntax and diff whitespace
checks pass. All requested document gates pass: `spec-requirements-check`,
`requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
`line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
`use-case-receipts-check`, `use-case-receipts-test`. The receipt gate reports all
18 receipts current and its five test cases passed. Full output is retained at
`/private/tmp/v10693-doc-gates.log`. An initial citation check found shifted anchors;
helper/text placement was adjusted to preserve those original anchors, and the
citation check then passed without repinning unrelated evidence.

The repaired negative control against wrappers extracted from the immutable base
exits 1 at `direct/caller/change`: expected the fixture's private caller cache,
observed `/tmp/corvint-go-build-cache`. Its diagnostic and exit are retained in
`/private/tmp/v10693-cache-baseline.log` and `v10693-cache-baseline.exit`.
Independent review of the explicit assertion repairs found no remaining issue.
`sh script/repin-use-case-receipts.sh --check` passes: all 18 receipts current
(`/private/tmp/v10693-repin.log`). No receipt repinning was needed; `cmd/corvint/main.go`
was unchanged. No Go process implementation was changed; the focused regression
is a shell fixture, so there is no Go race-test invocation for this patch.

Final focused run: `bash script/dogfood-cache_test.sh` exits 0 and prints
`DCW-V0-038 dogfood cache selection: PASS`; output and exit are retained at
`/private/tmp/v10693-cache-test-final.log` and `v10693-cache-test-final.exit`.
This is the repaired harness, with explicit assertion failures; prior shell PASS
observations are not reused. The positive and negative controls both completed
and retired their disposable fixture directories. Independent review is clean.
Normal staging in the real index remains blocked by the managed filesystem;
all eight task files remain available in the worktree for the coordinator to stage.

The receipt-test fixtures inherited the alternate-index environment and replaced
that scratch index. The real index was untouched. Final readback rebuilds a fresh
private index from the original, stages exactly the eight task files and repeats
the affected index-based documentation checks; fixture mutations are not retained
as a repository diff.
