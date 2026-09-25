# Main Use-Case Conformance V0

Owner: Russell Lewis
Frozen: 2026-08-23
Intent status: proposed overall; accepted daily-workflow scope and UCV0-013 (decision 0332)
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/DOGFOOD.md`,
`docs/specs/applied-intelligence-breakthroughs-v0.md`,
`docs/specs/agent-harness-integration-v0.md`

## Agent digest
- Claim: Headline Corvint jobs remain UNPROVEN until each has complete content-addressed promotion evidence.
- Status: proposed overall; accepted daily-workflow scope and UCV0-013 (decision 0332); experimental
- Exists: `conformance/use-cases-v0/ledger.json` and deterministic validator with every job `UNPROVEN`.
- Blocked on: complete promotion receipts, Corvint and Beamfall dogfood, and sealed outcome benchmarks.
- Read next: Verified current state; Status and claim model; Acceptance matrix.

## User and job

Corvint maintainers and adopters need one mechanically checked answer to a deceptively simple
question: which headline jobs are merely described, which have executable experiments, and which
have survived the complete product-evidence gate? A feature list, passing unit suite, or compelling
demo must never silently become a claim that Corvint supports a job in production.

The governed jobs are AI-assisted coding, debugging, and engineering research; Pi and DeepSeek
Harness discovery/update lifecycles; MkDocs and other human documentation; automatic E2E testing;
PR maintenance and policy-safe merge; change breakage; missing-test discovery and generation;
minimum-test execution; onboarding; ticket/team routing; code review; code-to-spec extraction;
removal and migration planning; shipped-change narration; incident orientation; and Live
Proof-Carrying Verification. Decision 0332 adds the narrower daily-workflow jobs
`UC-TASK-ORIENTATION`, `UC-CHANGE-CONSEQUENCE`, and `UC-EVIDENCE-CARRYING-COMPLETION` under
`daily-change-evidence-workflow-v0.md`; it does not narrow or promote any existing job.

## Verified current state

The canonical ledger is `conformance/use-cases-v0/ledger.json`; its deterministic validator is
`conformance/use-cases-v0/main.go`. At this freeze every governed job is `specified` and `UNPROVEN`.
Some Corvint primitives and harness adapters are executable, but foundations do not establish any
complete job. No row currently has the six promotion receipts, Corvint plus Beamfall dogfood, or a
sealed outcome benchmark required for `VERIFIED`.

On 2026-09-23 (decision 0373 item 16) the three daily-workflow rows moved to `experimental` with
`contract` and `implementation` receipts under `conformance/use-cases-v0/receipts/`; every claim
stays `UNPROVEN`. Their sealed-benchmark receipts from run-001 record `FAIL` for orientation and
consequence and `PASS` for completion; sealed run-002 (preregistration amendment 1, ticket V1-0202)
records `PASS` for all three, and its receipts are bound to the rows. Ticket V1-0188 added a
`hostile-tests` receipt per row, pinned to its entrypoint test file
`cmd/corvint/usecase_hostile_*_test.go`. Ticket V1-0207 added a `corvint-dogfood` receipt per
row, pinned to bind commit 8668f77b73eaf7b203abbb922aa1fe9cff8fd6c8 of the V1-0188 review-repair
increment, which went through the `docs/DOGFOOD.md` daily path; each receipt's subjects are that
commit's sealed CEM and a byte-identical copy of the retained daily-path artifact for the row
(`prechange-query`, `prechange-impact`, `dogfood-report`) under
`receipts/<useCaseId>/corvint-dogfood/`. Ticket V1-0184 bound a `beamfall-dogfood` receipt for
`UC-CHANGE-CONSEQUENCE` and `UC-EVIDENCE-CARRYING-COMPLETION`, from the agent-run daily path on
Beamfall's LCRES-15 change (bind commit 86fde0eb21d2e0fc41bfc06a4c06c9c3aef63e59, published on
Beamfall/core branch `claude/corvint-dogfood-LCRES-15`, PR beamfall/core#31). Each receipt's
subjects are that commit's sealed CEM and a byte-identical copy of the retained daily-path
artifact for the row under `receipts/<useCaseId>/beamfall-dogfood/`; the consequence row's
artifact is path impact over the intended files, named `prechange-impact` by convention, because
the pre-edit range impact is empty by construction (ticket V1-0262). The owner accepted the
Beamfall intent spec these receipts attest against (`docs/plugins/trust-roots.md`,
`PTR-V0-001..004`) on 2026-09-25 in that same Beamfall PR #31 (commit
6a95ae32116718d55687da97efd9108b5174fe5e, after the bind). `UC-TASK-ORIENTATION`'s
`beamfall-dogfood` receipt comes from a later Beamfall change that adds tests for that spec (bind commit
1c9fa18da1a3cd715b56f73b267c4ee5e8d3c76e, base 6a95ae32…, Beamfall/core branch
`claude/corvint-dogfood-PTR-tests`, PR beamfall/core#32). Its subject is the retained
`prechange-query` packet, which is `READY` with no abstention after `GPK-V0-066` (decision 0387),
plus that commit's sealed CEM. The earlier LCRES-15 query had abstained with zero results
(ticket V1-0260). All three Core rows hold one receipt from every evidence class. Ticket V1-0011 promoted them
to `verified`/`VERIFIED` together. The other nineteen rows remain `UNPROVEN`. The orientation
row's Beamfall run used a build that includes `GPK-V0-066`, which the published 0.8.1 archive does not
contain (0.8.1 abstains on that query, per V1-0260). Under `UCV0-012`, the `VERIFIED` claim
therefore applies to releases that include decision 0387, not to 0.8.1.

## Status and claim model

- `specified`: a stable job and promotion boundary are recorded. It does not imply an executable
  product path.
- `experimental`: an executable product path exists, but one or more promotion obligations remain.
- `verified`: the complete promotion evidence set is present, content-addressed, and accepted by
  the conformance runner.
- `UNPROVEN`: the mandatory claim for every `specified` or `experimental` row.
- `VERIFIED`: permitted only for a `verified` row whose complete evidence set validates.

`UNPROVEN` is the default. Unknown, missing, stale, malformed, mismatched, or inaccessible evidence
cannot be interpreted as success.

## Requirements

- `UCV0-001`: The canonical ledger MUST contain exactly one row for every governed job named above,
  with a stable ID, non-empty title and job, one status, one claim, and a closed evidence object.
- `UCV0-002`: Status MUST be exactly `specified`, `experimental`, or `verified`. Claim MUST be
  exactly `UNPROVEN` or `VERIFIED`. `specified` and `experimental` MUST be `UNPROVEN`; `verified`
  MUST be `VERIFIED`.
- `UCV0-003`: The promotion evidence classes are exactly `contract`, `implementation`,
  `hostile-tests`, `corvint-dogfood`, `beamfall-dogfood`, and `sealed-benchmark`. A `verified` row
  MUST have one receipt from every class. An `experimental` row MUST have at least `contract` and
  `implementation` receipts; other partial evidence remains non-promoting.
- `UCV0-004`: Every evidence reference MUST contain only a repository-relative receipt path and the
  lowercase SHA-256 of its exact bytes. Absolute paths, traversal, symlinks, non-files, digest
  mismatch, duplicate receipt reuse, and paths outside the repository MUST fail closed.
- `UCV0-005`: Every referenced receipt MUST be closed JSON with profile
  `corvint-use-case-evidence/0`; the same use-case ID and evidence class as its ledger edge; `PASS`;
  one immutable 40- or 64-hex repository revision; and at least one independently hashed subject.
  Subject paths and bytes obey the same containment and SHA-256 rules as receipts.
- `UCV0-006`: A `contract` receipt MUST cite at least one stable requirement ID. An
  `implementation` receipt MUST cite at least one executable entry point. A `hostile-tests` receipt
  MUST attest distinct `negative`, `hostile`, and `abstention` cases. Dogfood receipts MUST identify
  exactly `corvint` or `beamfall` as appropriate and attest a `PASS` outcome.
- `UCV0-007`: A `sealed-benchmark` receipt MUST attest `PASS` and bind three distinct SHA-256
  identities: a preregistration, a corpus, and a result. Equality, omission, or malformed identities
  fail closed. Mechanical completeness does not replace independent review of benchmark quality.
- `UCV0-008`: The runner MUST validate populated evidence even on an `UNPROVEN` row, so partial or
  staged evidence cannot rot invisibly. It MUST produce deterministic canonical JSON and a non-zero
  exit for any violation.
- `UCV0-009`: The runner MUST NOT execute receipts, subjects, product code, test commands, hooks, or
  network requests. It reads bounded local files only; successful conformance proves ledger and
  evidence-envelope integrity, not the truth of arbitrary prose inside a subject.
- `UCV0-010`: Promotion requires both Corvint self-dogfood and Beamfall dogfood. Evidence from one
  repository, a synthetic fixture, unit conformance, or a benchmark alone cannot substitute for the
  other evidence classes.
- `UCV0-011`: Live Proof-Carrying Verification is a separately governed product job: on each edit it
  must derive affected obligations, select witnessed checks, execute incrementally, expose runtime
  evidence and explicit unknown scope, and bind the result to the exact worktree. A generic test
  runner or static impact list does not satisfy it.
- `UCV0-012`: No row may be advertised as supported or delivered from this ledger unless its claim
  is `VERIFIED`. Consumers MUST display the literal claim rather than infer support from status,
  receipt count, implementation presence, or neighboring rows.

- `UCV0-013`: The canonical ledger profile MUST be `corvint-use-case-conformance/1`, containing
  the historical nineteen IDs plus exactly `UC-TASK-ORIENTATION`, `UC-CHANGE-CONSEQUENCE`, and
  `UC-EVIDENCE-CARRYING-COMPLETION`. The reader MUST retain the historical closed nineteen-ID
  `corvint-use-case-conformance/0` profile. New IDs in `/0`, missing IDs in `/1`, and unknown
  profiles MUST fail visibly. Migration preserves every historical row and its evidence bytes;
  it adds the new rows as `specified`/`UNPROVEN` with no evidence. Historical readers reject `/1`;
  archives retain `/0` bytes instead of pretending forward compatibility. Evidence `/0` and
  result `/0` schemas, the six evidence classes, and promotion requirements remain unchanged.

## Trust boundary and failure behavior

The ledger and receipt envelopes are caller-controlled local bytes. The runner treats all paths and
JSON values as hostile, imposes a 1 MiB limit per JSON or subject file, rejects duplicate JSON keys,
accepts no extra fields, follows no symlink, and performs no Git or subprocess operation. Invalid
input emits no partial promotion result and exits non-zero. A valid ledger with nineteen
`UNPROVEN` rows exits zero because honest incompleteness is a valid state.

The runner proves only that the declared promotion packets are complete, internally bound, and
content-addressed. Human acceptance, product value, benchmark independence, and external-market
truth remain outside this mechanical boundary.

## Non-goals and baseline

This version does not implement any governed job, schedule work, choose commercial licensing,
publish documentation, install plugins, execute tests, mutate PRs, or merge code. It does not allow
receipt quantity to vote a row into `VERIFIED`.

The simpler baseline is a Markdown feature matrix maintained by convention. It is rejected because
status can drift, links can rot, partial tests can be mistaken for complete evidence, and no
mechanism prevents an unsupported `verified` label.

## Acceptance matrix

The conformance tests MUST cover the honest all-unproven ledger, exact job coverage, canonical
repeatability, a complete synthetic promotion packet, missing evidence, receipt and subject
tampering, class/subject mismatch, duplicate reuse, traversal/symlink escape, missing hostile cases,
and an incomplete or self-colliding benchmark seal.

Promotion of this conformance mechanism from experimental requires an independent review of the
closed schemas plus successful Corvint and Beamfall use in a real capability promotion. Roll back by
removing the runner from promotion policy while retaining the ledger and receipts as historical
evidence; never rewrite a failed receipt.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| `UCV0-001..003`, `UCV0-012` | `conformance/use-cases-v0/ledger.json`, `main.go` | `conformance/use-cases-v0/main_test.go` |
| `UCV0-004..010` | `conformance/use-cases-v0/main.go` | hostile and complete-packet tests |
| `UCV0-013` | `conformance/use-cases-v0/main.go`, `ledger.json` | `TestUCV0ProfileMigration`; historical reader refusal |
| `UCV0-011` | governed ledger row only | `UNPROVEN`; implementation and outcome evidence absent |

## Unresolved decisions and kill criteria

Before any real promotion, owner review must decide who may issue each evidence class and whether
cryptographic signatures supplement local content addressing. If the mechanism permits one false
`VERIFIED`, silently accepts tampering, or allows one evidence class to substitute for another,
disable promotion immediately. Retain it as a read-only gap ledger until repaired.
