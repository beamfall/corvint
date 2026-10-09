# 2026-10-08: native per-ticket obligation ledger (V1-1022)

## Intent

Issue 680 (native ticket V1-1022) asks for a bounded per-ticket ledger of named obligations that a
Playwright json report witnesses step by step, so queue and dispatcher progress counts proof
rather than sessions. `docs/specs/corvint-tasks-obligation-ledger-v0.md` (TOL-V0-001..021) is the
contract. The owner accepted it on 2026-10-08 (decision 0456, recorded on the acceptance branch,
which also owns the spec's intent-status lines) with eight resolutions:

1. `complete` and gates do not require core obligations to be witnessed.
2. Only retry 0 credits.
3. OWNER holds the verbs by default; OPERATOR only through an explicit policy row; a WORKER report
   witness only through the opt-in `obligations.workerWitness` policy key.
4. Bounds: 256 obligations, 1,024 events, 64 KiB events, 64 MiB reports.
5. HELD tickets admit ledger writes.
6. `captureGitInfo` is not required; the caller names the commit.
7. The TOL-V0-021 stall restart stays.
8. The work-state `State` stays in the fingerprint.

## Change

- Record and wire: the optional `obligations` reference `{prefix, revision, head, counts,
  highWater, lastRaise}` joins the ticket record codec and `wire.TicketRecordOptionalKeys`. Core's
  read-only ticket reader (`internal/taskman/obligations.go`) admits it with the same bounds. A
  record without the member keeps its legacy bytes.
- Ledger: each `OBLIGATIONS_SEED|SET|WITNESS` write derives one canonical
  `taskman-obligation-event/0` that retains the whole request, chained by `previous`, in the existing
  MUTATE derived-event slot. The ledger is the fold of that chain (`ticket.FoldObligationChain`).
  An event over 65,536 bytes refuses `LIMIT_EXCEEDED OBLIGATION_EVENT_TOO_LARGE:`.
- Writer (`internal/tasks/mutation/obligations.go`): revision-only writes under CAS; the
  TOL-V0-015 role limits; attempt-generation provenance for a WORKER witness, with a stale
  generation `FENCED`; prefix uniqueness; the secret screen; recomputation of report credits
  (`OBLIGATION_CREDIT_MISMATCH:`); and the high-water and `lastRaise` marks.
- Report reader (`internal/tasks/obligation/report.go`): pure Go with no Node dependency. It reads
  the closed json reporter shape, credits retry 0 only by each step's own error, and reports
  conflicting matches, unknown ids, unmatched tests and three unbound reasons (outside the
  repository, absent at the commit, id not in the source). It refuses any `config.version` outside
  the qualified list, which is empty.
- CLI: `ticket obligations seed|set|witness|show|plan`. `show` and `plan` are read-only.
- Audit: `transaction.ObligationReceiptAudit` replays every ledger change from retained bytes
  through `mutation.ReplayObligation` and re-checks source presence at the event's commit. It is
  wired into receipt audit and redo through `store.FoldReceiptBindings`.
- Dispatcher:
  - The fingerprint hashes the witnessed high water rather than ledger churn (TOL-V0-017).
  - A WORKER `lastRaise` ends a no-progress run (TOL-V0-018).
  - A raise restarts the stall clock, with dispatch-state `/3` adopted (TOL-V0-021).
  - `queue status` and its summary carry the `obligations` sums only when a ledger exists
    (TOL-V0-019).

## Verification

- Focused tests: `GOTOOLCHAIN=local GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m
  ./internal/tasks/... ./internal/taskman/...`. The result is in the V1-1022 handoff.
- Every TOL requirement except the TOL-V0-019 plain-language line traces to a named test in the
  spec's traceability table. TOL tests run on synthetic json reports built in the test.
- `TestTOLV0013_TamperedWorkerGenerationInconsistent` rehashes a WORKER witness so that every byte
  is self-consistent and only the attempt generation is untrue. Receipt audit refuses it with
  `JOURNAL_FORKED` ("the write does not replay"). The control, with the true generation, stays
  consistent.
- Doc gates and `go test ./internal/specindex` ran before commit. Independent review used Codex
  read-only.

## Review

Codex (read-only, `gpt-6-astra`) reviewed `origin/main..HEAD` and reported one P1 and five P2
findings. All six were fixed, each with a test:

- **P1:** a corrupt chain whose event pointed back at its own storage key looped while the writer
  lock was held. `ObligationChainEvents` now stops at a revisit and bounds reads, not distinct keys.
  A case in `TestTOLV0002_EventCanonicalAndChained` covers it.
- **P2, witness replay:** a retried witness recomputed its credits against the ledger it had
  already changed, so it returned `written: false` instead of replaying. Credits now come from the
  ledger before the event that holds the same request ID (`ticket.ObligationLedgerBeforeRequest`).
  Covered by `TestTOLV0014_WitnessReplay`.
- **P2, declared commit:** a `DECLARED` witness now has its commit checked at audit. Covered by
  `TestTOLV0013_DeclaredCommitAudited`.
- **P2, stall rebase:** a new acceptance revision observed mid-session now rebases the stall
  baseline, so a later raise inside that revision restarts the count. Covered by a new subtest of
  `TestTOLV0021_StallRestartsOnRaise`.
- **P2, match bound:** the per-id 16-match bound now applies only to ids inside `--ids`. Covered by
  `TestTOLV0009_SubsetIgnoresExcludedMatchBound`.
- **P2, deferred ids:** a `DEFERRED` id in a declared batch is skipped rather than failing the
  batch. Covered by `TestTOLV0008_DeclaredWitness`.

The focused run also caught three test-side regressions. The shared issue502 fixtures now carry
the new optional `obligations` member. The obligation event and plan profiles are rows in the
CAL-V0-131 newer-version table, and the plan decoder now refuses a newer profile with
`UNSUPPORTED_VERSION`. The ON-V0-006 derived-event slot test now counts the obligation operations
among the declaring operations.

The re-review (one round) found one more P2: a reused request id whose new bytes derive no credit
from the ledger before the recorded request returned `written:false` instead of a conflict. A
recorded witness always carries credits, so the command now refuses that case with
`REQUEST_ID_CONFLICT`. `TestTOLV0014_WitnessReplay` covers it, and it fails with the guard
disabled.

## NOT_RUN and limits

- **Per-id match bound (retained limit).** One credited id holds at most 16 distinct matches in an
  event (`ticket.MaxObligationMatches`), which keeps a 256-credit witness bounded. An id with more
  distinct passing matches, such as one test run across more than 16 projects, refuses
  `LIMIT_EXCEEDED OBLIGATION_EVENT_TOO_LARGE:`, and `--ids` cannot split a single id.

- **Live Playwright 1.63 fixture (TOL-V0-009..013): `NOT_RUN`.** Playwright is not installed on this
  host, and it was not downloaded. The qualified version list stays empty, so any real report
  refuses `UNSUPPORTED_VERSION` (`OBLIGATION_REPORT_VERSION_UNQUALIFIED:`) until a live fixture
  passes and the version is added.
- **TOL-V0-019 plain-language line: `NOT_RUN`.** `queue status` has no plain output mode, so only
  the JSON members exist.
- **Integrated items: `NOT_RUN`.** These are the native archive round trip, the two-process CAS,
  interrupted-commit redo and a live dispatcher run.
- Delivery status is `experimental`. The intent status stays with the acceptance branch.
