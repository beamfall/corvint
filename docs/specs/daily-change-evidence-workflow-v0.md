# Daily Change-Evidence Workflow V0

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `DCW-V0`
Intent status: accepted scope (decision 0332); evaluation protocol awaits separate freeze
Delivery status: experimental; milestone NOT_QUALIFIED
Authoritative inputs: decision 0332, `docs/DOGFOOD.md`, `public-release-v0.md`,
`use-case-conformance-v0.md`, `local-completion-policy-v0.md`

## Agent digest
- Claim: 0.6 requires verified task orientation, change consequence and evidence-carrying local completion.
- Status: accepted scope (decision 0332); evaluation protocol awaits separate freeze; experimental; milestone NOT_QUALIFIED.
- Exists: native commands and local completion primitives; three governed ledger identities.
- Blocked on: contract/lifecycle qualification, real dual-repository workflow, sealed correctness/cost evidence and candidate gates.
- Read next: Requirements; Acceptance and evidence; Compatibility and rollback.

## User and job

A developer and a fresh reviewer need to complete a real change with original governing evidence,
explicit affected scope, checked proof maps and a retained outcome. A successful command or a
structurally valid map alone does not establish correctness, complete coverage or host authority.
The published starting point is 0.5.0a3; choosing a candidate version does not qualify 0.6.

## Requirements

- `DCW-V0-001`: Core MUST remain one native Go binary with no required account, network, daemon,
  mutable database service or companion. The daily path MUST connect task context, impact,
  CEM, OCM, frontier, selected checks, independent verification/review and retained outcome.
- `DCW-V0-002`: The three jobs MUST use exactly `UC-TASK-ORIENTATION`, `UC-CHANGE-CONSEQUENCE`,
  and `UC-EVIDENCE-CARRYING-COMPLETION`. All existing jobs and claims MUST remain unchanged.
  Their closed ledger migration MUST satisfy `UCV0-013`.
- `DCW-V0-003`: Task orientation MUST locate governing intent and original revision-pinned evidence,
  or preserve explicit omissions/abstention. Inferred context MUST NOT become accepted authority.
- `DCW-V0-004`: Change consequence MUST identify supported affected scope for a concrete change,
  preserving unsupported languages, unexamined scope, stale evidence and incomplete frontiers.
  A selected test list MUST NOT imply full behavioral coverage.
- `DCW-V0-005`: Evidence-carrying completion MUST bind a real committed change to its immutable
  base/target, CEM, owning OCMs, actual check observations, inspected reports/frontier and outcome.
  Missing, stale, dirty, interrupted, unsupported or unknown evidence MUST NOT produce a false
  complete-evidence verdict. Local policy satisfaction MUST NOT assert universal correctness or
  formal FULL/Frontier authority.
- `DCW-V0-006`: Qualification MUST include genuine Corvint and Beamfall changes completed by a fresh
  agent and reviewer using extracted public artifacts. Real dogfood, hostile tests and sealed
  benchmarks MUST remain distinct evidence classes under `UCV0-003`.
- `DCW-V0-007`: Before a sealed run, the exact three jobs, immutable corpus/labels, strongest
  adequate fixed baseline, correctness thresholds, complete cost accounting, exclusions and
  invalidation rules MUST be preregistered. The treatment MUST have zero treatment-only critical
  misses and zero false complete-evidence verdicts on designated missing-evidence cases.
- `DCW-V0-008`: Measurement MUST retain setup, agent/reviewer work, checks, failures, retries and
  latency for the complete task. Unobserved token or cost dimensions MUST remain NOT_OBSERVED;
  bytes MUST NOT be reported as tokens. Savings require a separately accepted threshold and
  passing paired measurement; this contract makes no savings claim.
- `DCW-V0-009`: Core command/profile compatibility and deterministic migration MUST be explicit.
  Fresh init/adopt, cold/incremental index parity, corruption, staleness, dirty/unsupported input,
  rollback, bounded reads and hostile paths MUST have candidate-specific evidence on each claimed
  native Darwin/Linux target. Cross-builds MUST NOT stand in for native qualification.
- `DCW-V0-010`: Codex and Claude MUST resolve the intended installed binary and current plugin
  artifacts. Exact host versions, enabled skill/hook discovery, real lifecycle operation, visible
  degradation and rollback MUST be retained. Formal FULL authority and other hosts are separate
  and nonblocking for this milestone; no installation may silently promote them.
- `DCW-V0-011`: Console, Tasks, MCP, editor, providers, automatic docs and protected authority MUST
  remain separately qualified optional companions/experimental profiles. Other shipped profiles
  MUST remain experimental unless separately qualified; deferred surfaces MUST remain deferred.
- `DCW-V0-012`: Final version/build, source and proof maps MUST freeze before exact-candidate
  evaluation and release gates. The required full gate, independent review and artifact/installed
  qualification MUST pass before milestone readiness. Evidence from a changed target MUST NOT be
  silently reused. Tagging, pushing, publishing and formal promotion require exact-packet owner approval.
- `DCW-V0-013`: The daily adopter path MUST be one ordered sequence in `docs/DOGFOOD.md` from the
  pre-change receipts to the seal. It MUST give every coordinator input's exact format with an
  example, the expected state after each step including the refusals that are expected before the
  CEM sidecar is committed, and the reason string each fail-closed class produces. A class that was
  not reproduced MUST be labelled NOT_OBSERVED, not described.
- `DCW-V0-014`: A `dogfood-change` not-complete row caused by a malformed or missing input, or by
  uncommitted worktree changes such as the prepared sidecar (including every `ocm-status-NNN` row),
  MUST be followed by a `fix:` line naming the correction. The
  `dogfood-check` failures `dogfood-report-missing`, `dogfood-report-drift` and `intent-scope-drift`
  MUST each print a `fix:` line; for `dogfood-report-drift` that line MUST distinguish a report bound
  to another base or target from an incomplete report. A refusal caused by a local trace recorded
  at a commit that is no longer an ancestor of `HEAD` MUST name that cause. Reason codes MUST NOT
  change.
- `DCW-V0-015`: Dirty, stale, interrupted, unsupported and unknown evidence MUST each end the loop
  without `"complete": true` or `dogfood-check: PASS`: a dirty or untracked worktree refuses
  `dirty-worktree`; a report older than `HEAD` fails `dogfood-report-drift`; an interrupted run
  writes no report, so the check fails `dogfood-report-missing` or, with an older report,
  `dogfood-report-drift`; a host without `rg` refuses `unsupported-environment-missing-rg`; and an
  uncited hunk leaves `cem-status` `not-ready`, so the check fails `dogfood-report-drift`. A passing
  check or a seal MUST be described as structural closure, never as correctness, test adequacy or a
  passing project gate.
- `DCW-V0-018`: `dogfood-change` MUST link an OCM obligation only from a row of the optional,
  author-written `DOGFOOD_OCM_LINKS` plan, applied through the verified `corvint ocm link` after that
  intent's map is prepared on every pass. It MUST NOT infer a link from names, paths or history.
  Without the input it MUST run no link and report no link row, so every requirement stays
  `unassessed`. A missing or malformed plan MUST report `ocm-links` NOT_PRODUCED, and a refused
  link MUST report `ocm-link-NNN` NOT_PRODUCED with the refusal code, each followed by a `fix:`
  line; neither refusal withholds the OCM aggregate.

## Non-goals and baseline

This slice creates no daemon, dispatcher, general autonomous authority, new language rewrite,
universal proof of correctness or automatic host promotion. Existing native commands are the
baseline; add runtime machinery only for an observed missing behavior. Keep original evidence and
human authority instead of converting a version label or synthetic packet into product proof.

## Acceptance and evidence

The native planning store's V1-0001/2/7/8/9 and V1-0010/11/12 acceptance criteria retain their
obligations. Fixture planning records are not execution evidence. Each new row requires contract,
implementation, hostile-tests, corvint-dogfood, beamfall-dogfood and sealed-benchmark receipts.
A fresh independent reviewer examines semantic adequacy, not merely digest validity.
The exact product candidate `T` is frozen before judged runs; its binary, source archive,
version/build, commit/tree and proof maps identify what the six evidence classes and
native/full/artifact/installed-host gates assess. Later evidence-only snapshot `E`
may retain reports, receipts and ledger changes bound to `T`, with its own scoped
CEM/OCM, integrity/docs checks and independent semantic review. These checks prove
`E`'s integrity, not an `E`-built binary. A Beamfall receipt retains the actual
Beamfall `repositoryRevision` and separately binds Corvint `T` artifact hashes.
`T`'s archived `UNPROVEN` claims remain unchanged; only owner-accepted `E` evidence
may qualify the explicitly named `T`. No such acceptance is recorded here.

| Requirements | Implementation/evidence | Current result |
|---|---|---|
| `DCW-V0-002` | `conformance/use-cases-v0`; `TestUCV0ProfileMigration` | migration implemented; qualification pending |
| `DCW-V0-001`, `DCW-V0-003..006` | existing CLI, CEM/OCM and local completion; real workflow receipts required | NOT_QUALIFIED |
| `DCW-V0-007..008` | independently sealed complete-task evaluation required | NOT_RUN |
| `DCW-V0-009..010` | native platform and installed exact-host evidence required | NOT_QUALIFIED |
| `DCW-V0-011..012` | portfolio, gate and candidate evidence required | NOT_QUALIFIED |
| `DCW-V0-013..015` | `docs/DOGFOOD.md` "Daily adopter path"; `script/dogfood-change_test.sh` run by `TestGoOnlyContextAbstentionRemainsClosed`; scratch reproductions recorded in the V1-0010 build-log entry | implemented; the SIGINT interrupt and reviewer leg NOT_OBSERVED |
| `DCW-V0-018` | `script/dogfood-change_test.sh` link phase; the replay of the sealed LAC-V0-032 change recorded in the V1-0142 build-log entry | implemented; a link on a change delivered through this loop NOT_OBSERVED |

## Compatibility and rollback

Retain historical `/0` use-case admission and use explicit `/1` for the three added jobs. The
current alpha candidate/installer requires companions and alpha version tokens; Core-only 0.6
packaging needs an explicit compatible contract before release. This spec does not change that
wire by implication. If any gate fails, preserve its evidence and keep the affected jobs UNPROVEN.
Retain the previous working installed binary and public release. Do not rewrite historical receipts.
Any runtime behavior, test, selected check, fixture, owning normative requirement or
`T` proof-map change requires a new candidate and invalidates affected evidence.
Preserve failed and incomplete `T` and `E` records; neither local PATH activation nor
evidence-only publication replaces the immutable retained candidate.

## Remaining decisions

The exact sealed evaluation protocol, native platform tuples and candidate artifact schema must
be frozen before execution. Missing cost telemetry or an unavailable independent corpus blocks its
claim, not honest development work. Discontinue promotion on any false complete-evidence result.
