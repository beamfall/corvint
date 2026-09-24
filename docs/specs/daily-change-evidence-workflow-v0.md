# Daily Change-Evidence Workflow V0

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `DCW-V0`
Intent status: accepted scope (decision 0332); DCW-V0-018/019 accepted (decision 0376); DCW-V0-020..023 owner-directed (V1-0236, 2026-09-24); evaluation protocol awaits separate freeze
Delivery status: experimental; milestone NOT_QUALIFIED
Authoritative inputs: decision 0332, `docs/DOGFOOD.md`, `public-release-v0.md`,
`use-case-conformance-v0.md`, `local-completion-policy-v0.md`

## Agent digest
- Claim: 0.6 requires verified task orientation, change consequence and evidence-carrying local completion.
- Status: accepted scope (decision 0332); DCW-V0-018/019 accepted (decision 0376); DCW-V0-020..023 owner-directed (V1-0236, 2026-09-24); evaluation protocol awaits separate freeze; experimental; milestone NOT_QUALIFIED.
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
  `dogfood-report-drift`; and an uncited hunk leaves `cem-status` `not-ready`, so the check fails
  `dogfood-report-drift`. `dogfood-change` MUST run the local-outcome recorder after it prepares
  and cites the sidecar, so a pass whose sidecar is untracked or modified reports `local-outcome`
  `record-index-failed` and is never complete (V1-0175). A passing check or a seal MUST be described as structural closure, never
  as correctness, test adequacy or a passing project gate.
- `DCW-V0-016`: (proposed 2026-09-23, V1-0200, not accepted) the `corvint-dogfood-change/0`
  report that `dogfood-change` writes, and `corvint dogfood finish` reads, MUST carry one
  `packetCoverage` line listing the `prechange-query` and `prechange-impact` steps in that order.
  A step that produced its packet copies the packet's `coverage.packet_bytes`, `budget_bytes`,
  `within_budget`, `included_results` and `omitted_results` under those names with
  `"status": "PRODUCED"`. A step that compiled no packet is `"status": "NOT_PRODUCED"` with reason
  `packet-not-compiled`, and output without exactly one well-formed occurrence of each field is
  `NOT_PRODUCED` with reason `packet-coverage-unreadable`; neither carries numbers. The line does
  not change `complete` or any step row. `corvint dogfood begin` compiles no packet and records no
  coverage. The member is additive: the profile stays `/0`, and every reader MUST accept a report
  written before it existed.
- `DCW-V0-017`: The `dogfood-check` verifier set and its `outputsAgree` result are author-only
  evidence: they need the author's private report, local-outcome record and OCM maps, and no
  committed artifact binds the report's digest, so a reviewer MUST NOT be asked to accept a
  handed-off report as their own observation. `docs/DOGFOOD.md` step 11 MUST say so and name what
  the reviewer verifies instead from a fresh clone of the bind commit: the committed CEM with their
  own binary (`corvint cem verify` and strict `cem status`), the seal as one exact rename, and the
  report's semantics (section 6). When `HEAD` tracks `.corvint/change.cem.json`, the
  `dogfood-report-missing` failure MUST also print a `review:` line stating this and naming the
  `cem verify` command for that base and `HEAD`; its reason code and exit status MUST NOT change.
- `DCW-V0-018`: `dogfood-change` MUST link an OCM obligation only from a row of the optional,
  author-written `DOGFOOD_OCM_LINKS` plan, applied through the verified `corvint ocm link` after that
  intent's map is prepared on every pass. It MUST NOT infer a link from names, paths or history.
  Without the input it MUST run no link and report no link row, so every requirement stays
  `unassessed`. A supplied plan that is missing, malformed or empty MUST report `ocm-links`
  NOT_PRODUCED; otherwise the report MUST record the plan's sha256 and row count. Every row for a
  prepared map MUST be attempted and reported as `ocm-link-NNN`, NNN being its plan row, and a
  refused row MUST be NOT_PRODUCED with the refusal code. Each NOT_PRODUCED row MUST be followed by
  a `fix:` line and MUST block `"complete": true`; none withholds the OCM aggregate.
- `DCW-V0-019`: Before any cite, `dogfood-change` MUST bind a nonempty `DOGFOOD_CITATIONS` plan to
  the map prepared in the same run and refuse the whole plan as `cem-cite`
  `citation-plan-map-mismatch`, citing nothing, when a row names an ordinal above the map's hunk
  count, a numeric selector is not a canonical ordinal, or a hunk the map records as `unknown` is
  named by neither ordinal nor full hunk ID. The permitted omissions are a hunk whose path is an
  intent path absent at BASE (the bootstrap unknown of `docs/DOGFOOD.md` section 2), and any unknown
  hunk while more of them remain than one 256-row plan can name, so split plans stay usable. The plan format does not change: its first field already accepts a
  full hunk ID, which is derived from the hunk's content, and `cem cite` refuses an ID the map lacks
  as `unknown-hunk-id`. An empty plan remains a zero-citation no-op, and a hunk already cited or
  marked in a resumed map needs no row. Citations add to a resumed map and never replace its
  earlier ones, so when a plan is cited onto a map that already carried a citation and the pass is
  not complete, `dogfood-change` MUST print the `cem-cite` line naming the correction: delete
  `.corvint/change.cem.json` and rerun (`docs/DOGFOOD.md` step 4, V1-0174).
- `DCW-V0-020`: (V1-0236, owner decision 2026-09-24) The daily change, check and seal MUST run as
  the subverbs `corvint dogfood change BASE`, `corvint dogfood check BASE` and `corvint dogfood seal
  BASE` of the existing Core verb, implemented in Go inside the binary, with no new root verb. They
  read the same `DOGFOOD_*` inputs and write the same stdout, stderr, report, receipts, CEM and exit
  statuses as the former scripts, including 129, 130 and 143 for a hang-up, interrupt or
  termination, and depend on no shell, `rg`, `python3` or `jq`. A subverb MUST NOT build,
  resolve on `PATH`, or version-check an executable, and MUST NOT read `CORVINT_BIN` or `VERSION`.
  Every step and verifier role is the running executable unless `--corvint-bin` (change) or
  `--base-verifier`, `--tree-verifier` and `--override-verifier` (check, seal) names another. A
  named verifier that cannot be read refuses `verifier-unavailable`; a missing or extra positional
  argument refuses `dogfood-base-required`, and an unresolvable running executable refuses
  `dogfood-executable-unavailable`. `dogfood seal` runs the check, then makes the same single
  rename commit as `script/dogfood-seal.sh`.
- `DCW-V0-021`: The subverbs MUST run at the worktree top level: the root is `--root` or the working
  directory, and a root with a nonempty `git rev-parse --show-prefix` refuses
  `not-repository-root` with exit 2, before any write.
- `DCW-V0-022`: Corvint-only identity checks MUST stay in Corvint's wrappers. `script/dogfood-change.sh`
  and `script/dogfood-check.sh` keep the `VERSION` read, the `CORVINT_BIN` version match and the
  current-tree (and, for the check, base-tree) builds with their refusal strings, then exec the
  subverb with the executables they built or selected, so a set `CORVINT_BIN` runs the whole flow and
  must carry `dogfood change`; a same-`VERSION` build without it (installed `Corvint 0.8.0 (build
  65)` while `VERSION` is 0.8.0) refuses `invalid-local-completion-action`. The change wrapper makes
  a relative `DOGFOOD_INTENTS_FILE`, `DOGFOOD_CITATIONS`, `DOGFOOD_VERIFY_FILE` or
  `DOGFOOD_OCM_LINKS` absolute against the caller's working directory before it moves to the
  repository root. `script/dogfood-seal.sh` stays the script
  that runs `script/dogfood-check.sh` and seals. The make targets and their `DOGFOOD_*` inputs are
  unchanged. Because the wrappers build before the subverb runs, a refusal the subverb makes now
  follows the builds.
- `DCW-V0-023`: `docs/DOGFOOD.md` "Daily adopter path" MUST give the installed-binary form for any
  Git repository and the ignore entries it needs: the private `.corvint` outputs
  (`dogfood-report.json`, `change.ocm-intents`, `change.ocm-status.json`, `change.ocm.*.json`,
  `self-observations.jsonl`) and the local trace store `.context-corvint/`. Without them the
  worktree is dirty or the recorder refuses `repository-identity-changed`; the subverbs do not
  write ignore rules.

## Non-goals and baseline

This slice creates no daemon, dispatcher, general autonomous authority, new language rewrite,
universal proof of correctness or automatic host promotion. Existing native commands are the
baseline; add runtime machinery only for an observed missing behavior. Keep original evidence and
human authority instead of converting a version label or synthetic packet into product proof.
The daily subverbs do not install, build or upgrade Corvint, pin a version for a foreign
repository, edit its ignore rules, or change any report, receipt or CEM format.

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
| `DCW-V0-016` (proposed) | `internal/dogfoodflow/change.go` `packetCoverage` (formerly `script/dogfood-change.sh` `packet_coverage_entry`); `script/dogfood-change_test.sh` run by `TestGoOnlyContextAbstentionRemainsClosed`; reader: `TestConsoleDogfoodPacketCoverage`; real run recorded in the V1-0200 build-log entry | implemented; not accepted |
| `DCW-V0-013..015` | `docs/DOGFOOD.md` "Daily adopter path"; `script/dogfood-change_test.sh` run by `TestGoOnlyContextAbstentionRemainsClosed` (the repo phase's uncommitted-sidecar pass and `assert_cited_uncommitted`); `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` (real recorder refuses the untracked sidecar of a first pass with every input); scratch reproductions recorded in the V1-0010 build-log entry | implemented; the SIGINT interrupt and reviewer leg NOT_OBSERVED |
| `DCW-V0-017` | `docs/DOGFOOD.md` step 11; the reviewer case in `script/dogfood-change_test.sh`; the V1-0182 build-log entry | implemented; reviewer-side verifier agreement is out of scope by design |
| `DCW-V0-018` | `script/dogfood-change_test.sh` link phase: no plan, exact argv and order, plan digest, partial refusal across rows and intents, and unlisted-intent, CRLF, field-count, empty-item, over-256-row, empty and absent plans; the replay of the sealed LAC-V0-032 change recorded in the V1-0142 build-log entry | implemented; a link on a change delivered through this loop NOT_OBSERVED |
| `DCW-V0-019` | `internal/dogfoodflow/change.go` `citationPlanMatchesMap` (formerly `script/dogfood-change.sh` `citation_plan_matches_map`); `script/dogfood-change_test.sh` cases `stale-nine-of-ten`, `stale-ten-of-nine`, `bootstrap-omitted`, `other-omitted`, `noncanonical-ordinal` and `split-over-row-limit`; `TestDogfoodReasonAdmitsCitationPlanMapMismatch`; `TestDogfoodChangeNamesDeleteWhenACorrectedPlanJoinsEarlierCitations` (built binary: a corrected plan joins the earlier citation and prints the delete line; after the delete only the corrected citation remains) | implemented; observed live on a 22-hunk map at base 34e798b: a 1-row plan refused, a 22-row plan cited all 22 |
| `DCW-V0-020..021`, `DCW-V0-023` | `internal/dogfoodflow`; `cmd/corvint/dogfood_flow.go`; `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` (built binary only, in a Go repository with no `script/`, `VERSION` or `cmd/corvint`, with `PATH` resolving `corvint` to a failing impostor and `CORVINT_BIN` naming a missing file: change, bind, change, check, nested-root refusal, seal) | implemented; foreign-repository portability shown by that fixture only; a real non-Corvint repository NOT_OBSERVED; output parity with the former scripts is shown only for the strings and statuses `script/dogfood-change_test.sh` and `script/dogfood-bind-range_test.sh` assert, and exits 129, 130 and 143 are NOT_OBSERVED by a test |
| `DCW-V0-022` | `script/dogfood-change_test.sh` (through the wrappers over a built driver) and `script/dogfood-bind-range_test.sh`, run in Corvint's tree | implemented; in-tree evidence only |

## Compatibility and rollback

Retain historical `/0` use-case admission and use explicit `/1` for the three added jobs. The
current alpha candidate assembler requires companions, and every candidate requires alpha version
tokens; the reader and installer admit a Core-only profile (`public-release-v0.md`, V1-0125), but
Core-only 0.6 packaging still needs an explicit compatible contract before release. This spec does not change that
wire by implication. If any gate fails, preserve its evidence and keep the affected jobs UNPROVEN.
Retain the previous working installed binary and public release. Do not rewrite historical receipts.
Roll back `DCW-V0-018` by unsetting `DOGFOOD_OCM_LINKS`: no link runs and every requirement
returns to `unassessed` on the next pass.
Roll back `DCW-V0-020..023` by restoring `script/dogfood-change.sh` and `script/dogfood-check.sh`
from `e667812`; the make targets, inputs and artifacts are the same on both sides, so evidence
from either side stays readable. The subverbs may remain unused.
Any runtime behavior, test, selected check, fixture, owning normative requirement or
`T` proof-map change requires a new candidate and invalidates affected evidence.
Preserve failed and incomplete `T` and `E` records; neither local PATH activation nor
evidence-only publication replaces the immutable retained candidate.

## Remaining decisions

The exact sealed evaluation protocol, native platform tuples and candidate artifact schema must
be frozen before execution. Missing cost telemetry or an unavailable independent corpus blocks its
claim, not honest development work. Discontinue promotion on any false complete-evidence result.
