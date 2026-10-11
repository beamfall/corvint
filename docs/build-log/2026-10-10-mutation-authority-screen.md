# V1-0658: authority-screen mutation witnesses

## Scope and contract

Base: `686fb1ac9c067046c1b7b3dc6776ac41e5c61ee6`, equal to this checkout's
`origin/main` at task start. No remote freshness claim is made. The worktree was clean.
The owner requested independent reproduction of the instruction-route screening and
reserved-order demotion survivors, regression witnesses, isolated guard deletions,
and restoration. This is test-only product work under existing experimental
TCP-V0-055/056; no requirement, accepted intent, production behavior, wire contract,
or decision is added or amended. The spec changes only evidence and traceability.
Owner acceptance of the experimental contract remains absent.

Plan: establish the original baseline, remove one guard at a time, add committed-fixture
packet witnesses, repeat each isolated mutation, restore production bytes, then run
focused package tests, independent review, and the requested documentation checks.
The inherited session model/effort is retained for this bounded test change and one
read-only native reviewer; provider/account usage telemetry is NOT_OBSERVED.
Rollback is removal of these test/evidence additions. No commit, publication, ticket
mutation, or plugin manifest change is authorized for this task.

## Findings and mutation evidence

The original poisoned-governing test does not reach routed-target screening: a
screened-out governing row is skipped by `instructionRoutedRows`. Moreover its
passage said “Formatting of any ledger” while the task says “format the ledger
output”. Only `ledger` matches; TCP-V0-047 requires two shared terms. Deleting only
the downgraded-governor routing skip still passed that original test. Adding `output`
to its passage repairs that separate admission defect.

The original spec-mentioned test checks demotion by calling `demoteScreened` directly.
It cannot detect deletion of the call from `reservedRows`. The added tests instead:

- `TestTaskContextScreensContentRoutedByCleanInstructions`: an unchanged clean
  `AGENTS.md` routes `docs/STYLE.md`, whose clean and poisoned variants exercise the
  same passage. The test first proves clean governing authority and exact routed
  admission, then checks authority, trust, confidence, score, warning, action, refusal,
  and critical selectors. Fixture bytes are committed before indexing, separating
  hidden Unicode from dirty-authority detection.
- `TestTaskContextOrdersScreenedReservationsEndToEnd`: the packet contains two clean
  specs and two poisoned reservations (governing and spec). It proves admission and
  screening before checking the exact stable partition, at limits 4 and 20, including
  ordinary source evidence after the reserved block in the larger packet.

All mutation commands use `GOTOOLCHAIN=local`,
`TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`, `-count=1`, and `-timeout 30m`.
The installed toolchain reports `go1.27.1`. Each variant changes only the named guard
relative to the original `taskcontext.go`; guards are never removed together.
Go `-overlay` binds the isolated source/test bytes in the scratch mutation runs.

| Isolated mutation | Original authority-screen tests | New witness | Actual observation |
|---|---|---|---|
| In `reservedRows`, replace `compiler.screenAuthority(compiler.instructionRoutedRows(rows))` with `compiler.instructionRoutedRows(rows)` | PASS: survivor reproduced | `TestTaskContextScreensContentRoutedByCleanInstructions/poisoned` FAIL | Poisoned target retains score 850, `instruction-reference`, `project-authority`, medium confidence and follow-the-route action. |
| In `reservedRows`, remove only the outer `demoteScreened` call | PASS: survivor reproduced | `TestTaskContextOrdersScreenedReservationsEndToEnd` FAIL | At limit 4 packet order is governing, alpha, beta, gamma instead of alpha, gamma, governing, beta. |
| In `instructionRoutedRows`, remove only `\|\| instruction.downgrade != ""` | Original poisoned-governing test PASS: routing-threshold defect reproduced | Repaired poisoned-governing fixture FAIL: `a downgraded governing file routed [docs/STYLE.md ...]` | This is an additional control of the reported original assertion, separate from the two requested guard mutations. |

The original-test selector is the anchored union of every `Test*` function in the
base `authority_screen_test.go`, retained in `/private/tmp/v1-0658-original-selector.txt`.
Each original selector run uses the same baseline or one-guard mutant source and
passes (exit 0). The two new witnesses pass together on unmutated production source;
the table's failures are exit 1 assertion failures, not compiler or fixture failures.
An ordinary pass is not counted as mutation adequacy. These results establish
only the named local deletion witnesses, not exhaustive mutation adequacy or an
external security qualification.

Original and new focused outputs: `/private/tmp/v1-0658-original-focused-*.log`,
`/private/tmp/v1-0658-new-*.log`; supplemental repaired-fixture output:
`/private/tmp/v1-0658-repaired-governing-route-control.log`.
Scratch overlay source and JSON bindings: `/private/tmp/v1-0658-overlay/`.
Final worktree focused verification passed (exit 0):

```sh
GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3 \
  go test -count=1 -timeout 30m ./internal/contextindex \
  -run 'Test(TaskContext.*(Rout|Governing|Screen|SpecMentioned|Reserv)|HiddenUnicode|Screen|CEMGoverning)'
```

Output: `/private/tmp/v1-0658-final-tests.log`. Final test bytes match the independently
reviewed candidate. `taskcontext.go` is byte-for-byte equal to the base; all guards
are restored. An optional unfiltered package baseline sweep was interrupted after
producing no result, and is **NOT_COMPLETED**, not a pass or mutant survivor. Its
queued unfiltered mutant runs never started. The scratch driver initially waited
reentrantly in its interrupt handler; a second interrupt exited it, and the known
driver/Go/test PIDs were verified absent. The scratch handler was corrected to
avoid waiting inside a signal handler; it is not a delivered product change.
Only the completed focused runs above support the mutation claims.

## Context, review, and verification limits

Corvint routes used: index refresh attempted once after the stale-snapshot fallback;
query and tracked-path impact receipts retained in `/private/tmp/v1-0658-context.json`
and `/private/tmp/v1-0658-impact.json`. The narrow query selected Tasks governance
rather than the owning test/spec, with omissions; original evidence was then read
from the owner's explicit paths. Pre-change `affected` selected no Go units on the
empty diff (`EMPTY_SELECTION`, scope `UNKNOWN`, language-frontier unknowns retained
in `/private/tmp/v1-0658-affected-before.json`), so the ticket's owning package supplies
the focused test scope. Post-change `affected` names `internal/contextindex` by
`DIRECT_TEST_CHANGE`, plus broad path-literal/unbounded readers; its language and
unowned-document frontiers remain open in `/private/tmp/v1-0658-affected-final.json`.
The requested touched-package scope and doc gates were used; other advisory packages
were not run. Its generic full-gate advice does not override the explicit
owner prohibition on `make gate` and repository-wide `./...` runs.

Core is `1.0.0-rc.3 (build 407)`; Tasks is
`0.0.0-tcp01-unverified+build.407`. Official update checks for both components failed
with `process-after-start-failed: descendant snapshot unavailable`; available
versions/freshness remain UNKNOWN, and no update was installed or verification
bypassed. The initial no-network check also reported freshness UNKNOWN.

`make dogfood-change BASE=686fb1ac9c067046c1b7b3dc6776ac41e5c61ee6` failed to create
Git-private state outside the sandbox (`Operation not permitted`); output is retained
in `/private/tmp/v1-0658-dogfood.log`. Post-commit CEM/OCM binding, dogfood checking/sealing,
publication, and native ticket completion are NOT_PRODUCED under the owner's explicit
no-commit/no-store-write scope. No retrieval behavior changed, so a frozen retrieval
promotion campaign is not applicable. Repository-wide tests and `make gate` are
NOT_RUN as requested. No ticket-store filing was attempted because store writes
are explicitly prohibited.

Real-index staging was denied by the sandbox. An isolated scratch Git index and
object directory at `/private/tmp/v1-0658-doc-index/` stages the candidate spec and
tests for the index-based generators/checks; the actual worktree index remains
untouched. `make -s spec-requirements` regenerated `REQUIREMENTS.tsv` with no content
change. The system `mktemp` chose an unwritable OS temp directory even with `TMPDIR`;
the check environment selects installed GNU `gmktemp` through a scratch PATH shim,
without altering repository scripts.

All requested documentation targets passed (exit 0): `spec-requirements-check`,
`requirement-definitions-check`, `traceability-tests-check`, `decision-numbers-check`,
`line-citations-check`, `error-code-ownership-check`, `unbounded-readers-check`,
`use-case-receipts-check`, and `use-case-receipts-test`. The separate
`sh script/repin-use-case-receipts.sh --check` passed (exit 0); `cmd/corvint/main.go`
was not changed. Final outputs: `/private/tmp/v1-0658-doc-gates-final.log` and
`/private/tmp/v1-0658-receipts-check-final.log`. The generator and checks used the isolated
index containing the exact candidate spec/test bytes. `REQUIREMENTS.tsv` is unchanged.
A first final rerun failed because the receipt fixture tests had inherited the
scratch `GIT_INDEX_FILE` and replaced its contents with their fixture index. This
was a scratch verification-environment failure, not product evidence. Rebuilding
that index from the untouched real index and clearing `GIT_INDEX_FILE`,
`GIT_OBJECT_DIRECTORY`, and `GIT_ALTERNATE_OBJECT_DIRECTORIES` for the receipt fixture
tests fixed the check setup; all final checks then returned exit 0. No real-index
or requirement-file corruption occurred.

Independent read-only native review inspected the original code, final candidate
tests, both single-guard overlay diffs, original passing outputs, new mutation
assertion failures, and the supplemental routing control. Result: no findings.
The reviewer specifically confirmed that the route witness does not depend on
ordering demotion, and the ordering witness admits no routed target and therefore
does not depend on routed-target screening. The final three changed tests pass
together against restored production source in the overlay (exit 0;
`/private/tmp/v1-0658-final-focused-baseline.log`).

Final SHA-256 of `internal/contextindex/authority_screen_test.go`: `b4e6a04286eaa883685bed3901e11bd2e2b4642c14305a3a354be2c7491db2e0`.
