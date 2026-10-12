# Daily Change-Evidence Workflow V0

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `DCW-V0`
Intent status: accepted scope (decision 0332); DCW-V0-018/019 accepted (decision 0376); DCW-V0-020..023 owner-directed (V1-0236, 2026-09-24); DCW-V0-024 owner-approved (V1-0259, 2026-09-25); DCW-V0-025 accepted (decision 0388, V1-0264, 2026-09-25); DCW-V0-026 accepted (decision 0395, panel M1, 2026-09-25); DCW-V0-033 accepted (decision 0441; V1-0940); evaluation protocol awaits separate freeze; DCW-V0-034 accepted, decision 0505; DCW-V0-039..041 accepted (decision 0519; V1-1151, V1-1171); DCW-V0-042..045 accepted (decision 0520; V1-1219, V1-1198, V1-1209, V1-1207); DCW-V0-019 amended and DCW-V0-046 accepted under the owner's standing auto-accept (2026-10-11) (decision 0523 P3; V1-1258); DCW-V0-047 accepted under the owner's standing auto-accept (2026-10-11) (V1-1279); DCW-V0-048/049 accepted under the owner's standing auto-accept (2026-10-11) (V1-1281); DCW-V0-050 accepted under the owner's standing auto-accept (2026-10-11) (V1-1277)
Delivery status: experimental; milestone NOT_QUALIFIED
Authoritative inputs: decision 0332, `docs/DOGFOOD.md`, `public-release-v0.md`,
`use-case-conformance-v0.md`, `local-completion-policy-v0.md`

## Agent digest
- Claim: 0.6 requires verified task orientation, change consequence and evidence-carrying local completion.
- Status: accepted scope (decision 0332); DCW-V0-018/019 accepted (decision 0376); DCW-V0-020..023 owner-directed (V1-0236, 2026-09-24); DCW-V0-024 owner-approved (V1-0259, 2026-09-25); DCW-V0-025 accepted (decision 0388, V1-0264, 2026-09-25); DCW-V0-026 accepted (decision 0395, panel M1, 2026-09-25); DCW-V0-033 accepted (decision 0441; V1-0940); evaluation protocol awaits separate freeze; DCW-V0-034 accepted, decision 0505; DCW-V0-039..041 accepted (decision 0519; V1-1151, V1-1171); DCW-V0-042..045 accepted (decision 0520; V1-1219, V1-1198, V1-1209, V1-1207); DCW-V0-019 amended and DCW-V0-046 accepted under the owner's standing auto-accept (2026-10-11) (decision 0523 P3; V1-1258); DCW-V0-047 accepted under the owner's standing auto-accept (2026-10-11) (V1-1279); DCW-V0-048/049 accepted under the owner's standing auto-accept (2026-10-11) (V1-1281); DCW-V0-050 accepted under the owner's standing auto-accept (2026-10-11) (V1-1277); experimental; milestone NOT_QUALIFIED.
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
  MUST be followed by a `fix:` line naming the correction; for a `cem-cite` `cite-span-not-stable`
  row that line MUST name the failing citation-plan row number. The `dogfood-check` failures
  `dogfood-report-missing`, `dogfood-report-drift` and `intent-scope-drift` MUST each print a
  `fix:` line; for `dogfood-report-drift` that line MUST distinguish a report bound
  to another base or target from an incomplete report. A refusal caused by a local trace recorded
  at a commit that is no longer an ancestor of `HEAD` MUST name that cause. Reason codes MUST NOT
  change.
- `DCW-V0-015`: Dirty, stale, interrupted, unsupported and unknown evidence MUST each end the loop
  without `"complete": true` or `dogfood-check: PASS`: a dirty or untracked worktree refuses
  `dirty-worktree`; a report older than `HEAD` fails `dogfood-report-drift`; an interrupted run
  writes no report, so the check fails `dogfood-report-missing` or, with an older report,
  `dogfood-report-drift`; and an uncited hunk leaves `cem-status` `not-ready`, so the check fails
  `dogfood-report-drift`, except a hunk kept unknown with a recorded reason under `DCW-V0-046`,
  which is admitted and counted in the check's `unknownShare`. `dogfood-change` MUST run the local-outcome recorder after it prepares
  and cites the sidecar, so a pass whose sidecar is untracked or modified reports `local-outcome`
  `record-index-failed` and is never complete (V1-0175). A passing check or a seal MUST be described as structural closure, never
  as correctness, test adequacy or a passing project gate. Amendment (V1-0362): the loop's Git
  reads run under Core's sanitized Git environment with an empty credential helper and no
  transport, so a replace ref, a lazy fetch or the caller's global or system configuration
  (including its hooks and excludes file) cannot change what they read; only the seal's `git mv`
  and `git commit` keep the caller's environment, for its author identity, signing and hooks.
- `DCW-V0-016`: (proposed 2026-09-23, V1-0200, not accepted) the `corvint-dogfood-change/0`
  report that `dogfood-change` writes, and `corvint dogfood finish` reads, MUST carry one
  `packetCoverage` line listing the `coordination-time-query` and `coordination-time-impact` steps
  (named by `DCW-V0-026`) in that order.
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
  hunk while more of them remain than one 256-row plan can name, so split plans stay usable. (Amended
  2026-10-11 by `DCW-V0-046`, V1-1258; accepted under the owner's standing auto-accept (2026-10-11).) A row of the form `HUNK<TAB>unknown<TAB>REASON<TAB>NOTE` names
  its hunk as a citation row does and keeps it `unknown` with a recorded reason instead of citing it;
  only an unreasoned unknown hunk is refused. The plan format does not change: its first field already accepts a
  full hunk ID, which is derived from the hunk's content, and `cem cite` refuses an ID the map lacks
  as `unknown-hunk-id`. An empty plan remains a zero-citation no-op only on a map that owes no hunk
(`DCW-V0-032`, accepted), and a hunk already cited or
  marked in a resumed map needs no row. (proposed 2026-09-25, V1-0228, not accepted) A map hunk
  path is compared with an intent path after JSON unescaping, and when `cem-prepare` is
  NOT_PRODUCED no map was prepared in the run, so `cem-cite` MUST be NOT_PRODUCED
  `cem-map-not-produced` and cite nothing. Citations add to a resumed map and never replace its
  earlier ones, so when a plan is cited onto a map that already carried a citation and the pass is
  not complete, `dogfood-change` MUST print the `cem-cite` line naming the correction: delete
  `.corvint/change.cem.json` and rerun (`docs/DOGFOOD.md` step 4, V1-0174). The CEM contract fixes
  strict JSON, not a byte layout, so a cite pass that leaves the map's JSON value unchanged MUST
  leave its bytes unchanged: a rerun whose plan a committed map in any valid encoding already
  carries keeps that map byte-identical and the worktree clean (V1-0386).
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
- `DCW-V0-024`: (V1-0259, owner decision 2026-09-25) A change that no `## Requirements` spec
  governs, such as one whose intent lives in ADRs or roadmap tickets, MUST be able to complete the
  daily path, but only by explicit declaration: a `DOGFOOD_INTENTS_FILE` whose whole content is the
  one line `#no-intent-declared`. An unset variable, an empty file, or any other content that is
  not a valid manifest still refuses `ocm-aggregate` `missing-intent-scope`, so an accidentally
  missing input never passes; a `#` line is never an intent path, so the declaration cannot be
  read as a manifest. Under the declaration `dogfood change` runs no OCM command, reports
  `ocm-prepare`, `ocm-status` and `ocm-aggregate` as `NOT_PRODUCED` `no-intent-declared`, writes
  `"ocmStatus": {"state": "NOT_ASSESSED", "reason": "no-intent-declared"}`, counts no bootstrap
  unknown and publishes the declaration as `.corvint/change.ocm-intents`. Those rows do not block
  `"complete": true`. A supplied `DOGFOOD_OCM_LINKS` refuses `ocm-links` `invalid-ocm-link-plan`,
  because no intent can own a row. `dogfood check` and `seal` MUST accept such a report only when
  the published declaration and the report's `ocmStatus` line agree, and otherwise fail
  `dogfood-report-drift`, so a swapped snapshot cannot skip OCM verification. They run no OCM
  verifier and print `dogfood-check: NOTE intent-linkage NOT_ASSESSED no-intent-declared` in place
  of the OCM status line before `PASS`. Intent linkage is then unassessed, never covered; the CEM
  citations are the only governing evidence the change carries. A manifest of intent paths behaves
  as before.
- `DCW-V0-025`: (accepted 2026-09-25, decision 0388, V1-0264) The `coordination-time-impact` step (named by `DCW-V0-026`) MUST keep
  a refusal of `corvint impact` as a typed abstention, not a failed step, when the command exits 2
  with empty stdout and stderr of exactly one line `{"code": "CODE", "error": "MESSAGE", "ok":
  false}` whose CODE is `unsupported-impact-range`, `unsupported-impact-repository` (the repository
  has no slash-qualified Go module, or its native Go index exceeds the aggregate bound) or
  `unsupported-impact-path` (a changed Go path is at the module root). The row is `NOT_PRODUCED`
  with that exact CODE as its reason, so the three stay distinguishable; the digest-bound
  context-abstention artifact records the same reason under the unchanged
  `corvint-dogfood-context-abstention/0` profile; the row does not block `"complete": true`; and
  `dogfood check` accepts it only under the same bindings and verifier rerun as before, with the
  rerun stderr carrying the same CODE. Any other code, exit status, stdout or stderr shape stays a
  blocking row, and one of these codes in any other shape reports `context-abstention-invalid`.
  For each of the three codes, `dogfood change` and a passing `dogfood check` each print one stderr
  line `dogfood-<subverb>: NOTE coordination-time-impact NOT_PRODUCED CODE`.
  Reason: impact is a Go-native profile, so its refusal to analyse a repository without a Go module
  or a module-root change is a scope limit that must stay visible, not a failed step; as blocking
  rows these two codes kept a non-Go adopter from ever reaching `"complete": true`.
  Acceptance: decision 0388 accepts this requirement with the `GOC-V0-009` and `ERI-V0-006`
  amendments that admit the same three codes.
- `DCW-V0-026`: (accepted 2026-09-25 by decision 0395, panel finding M1) `dogfood change` MUST NOT
  create, truncate or rewrite `<git-dir>/corvint/prechange-query.json` or `prechange-impact.json`,
  the receipts an agent writes before the change (`docs/DOGFOOD.md` section 1). Its own query and
  impact runs happen after the change, so they are the `coordination-time-query` and
  `coordination-time-impact` steps: their output, stderr, argv and context-abstention files carry
  those names, and so do the report rows, the `packetCoverage` entries and the NOTE lines. No
  report row or packet-coverage entry names a coordinator run as pre-change. `dogfood check` and
  `dogfood finish` read the renamed files; reports and self-observations written before this
  change keep their `prechange-*` rows, and `dogfood-observe` admits both spellings.
  Reason: the coordinator reran query and impact against `HEAD` into the agent's receipt paths,
  truncating the base-tree receipts, and reported its own post-change output as
  `prechange-* PRODUCED`, a false provenance claim.
  Not covered: the coordinator does not bind the tree of an agent receipt; `DCW-V0-031` (proposed)
  makes a missing or stale agent receipt visible (V1-0316).
  Acceptance: decision 0395 accepts this requirement and the rewording it makes to `DCW-V0-016`,
  `DCW-V0-025` and `LCP-V0-005`.
- `DCW-V0-027`: (proposed 2026-09-25, V1-0137, not accepted) `dogfood seal` and
  `script/dogfood-seal.sh` MUST refuse `unarchived-base-cem`, committing nothing, when `BASE`
  tracks `.corvint/change.cem.json`, the bind commit tracks a different blob there, and no
  `.corvint/changes/` file in the bind tree has `BASE`'s blob. The fix line names the commit that
  bound `BASE`'s CEM.
  Reason: a change started on a base that still tracked an unsealed CEM replaced it with
  `--replace`, and the seal then moved the replacement away, so the earlier CEM left the tree
  unarchived; the 6243-line 0.6.0 integration CEM was lost this way.
- `DCW-V0-028`: (proposed 2026-09-25, V1-0227, not accepted) When an intent's `ocm-prepare-NNN`
  row is NOT_PRODUCED, `dogfood change` MUST report every `DOGFOOD_OCM_LINKS` row naming that intent
  as `ocm-link-NNN` NOT_PRODUCED `ocm-map-not-prepared`, NNN being its plan row, with a `fix:` line
  naming the prepare row, and MUST NOT run `ocm link` for it.
  Reason: those rows were skipped with no row, so the report did not say they were never linked.
- `DCW-V0-029`: (proposed 2026-09-25, V1-0239, not accepted) Each time `dogfood change` accepts a
  nonempty plan under `DCW-V0-019`, it MUST record privately in the evidence directory the plan's
  SHA-256 and the prepared map's hunk IDs in map order. A later run with the same plan bytes MUST
  refuse `citation-plan-map-mismatch`, citing nothing, when any row's canonical ordinal named a
  recorded hunk ID that the new map holds at a different ordinal. A recorded ID absent from the new
  map, or a plan with different bytes, is not refused by this rule.
  Reason: a plan kept after a rebase swapped two hunks still named every hunk, so its rows cited
  the wrong hunks without a refusal.
- `DCW-V0-030`: (proposed 2026-09-25, V1-0145, not accepted; not implemented) `dogfood check`
  MUST refuse a report bound to another BASE or HEAD with `dogfood-report-stale` and a report that
  is not `"complete": true` with `dogfood-report-incomplete`, each with its existing `fix:` line,
  instead of `dogfood-report-drift` for both. `dogfood-report-drift` then stays only for a report
  whose anchor, context-abstention binding, intent declaration or dogfood policy line disagrees.
  Reason: one code covers two cases an operator fixes differently, and only the `fix:` line tells
  them apart. Acceptance needs an owner decision because it amends the accepted `DCW-V0-014` rule
  that reason codes MUST NOT change, and because `local-completion-policy-v0.md`,
  `dogfood-observe` and recorded reports read `dogfood-report-drift`.
- `DCW-V0-031`: (proposed 2026-09-25, V1-0316, not accepted) After its steps run, `dogfood change`
  MUST print, for each of `<git-dir>/corvint/prechange-query.json` and `prechange-impact.json`,
  `dogfood-change: NOTE NAME NOT_OBSERVED agent-receipt-absent` when the receipt is not a regular
  file, `dogfood-change: NOTE NAME NOT_OBSERVED agent-receipt-tree-unknown` when it has no
  `context.revision`, and `dogfood-change: NOTE NAME STALE agent-receipt-not-base-tree tree=TREE
  base-tree=BASE_TREE` when that revision is not BASE's tree, NAME being the file name without
  `.json`. A receipt at BASE's tree prints nothing. These lines MUST NOT change the report, its
  rows, `"complete"` or the exit status: `DOGFOOD-002` (`docs/DOGFOOD.md`) does not require a receipt, so an absent or
  stale one stays non-blocking unless an owner decision makes it blocking.
  Reason: after `DCW-V0-026` the coordinator no longer overwrote the agent receipts, but a missing
  receipt or one written against another tree was no more visible than before.
- `DCW-V0-032`: (accepted 2026-10-04, decision 0430; V1-0743) An empty `DOGFOOD_CITATIONS` file
  on a map that still records an `unknown` hunk outside the `DCW-V0-019` bootstrap omission MUST
  report `cem-cite` NOT_PRODUCED `empty-citation-plan` and run no `cem cite`, as an empty
  `DOGFOOD_OCM_LINKS` plan reports `empty-ocm-link-plan` under `DCW-V0-018`; the 256-row split-plan
  exemption does not apply to an empty plan. On a map that owes no hunk the empty plan stays the
  `DCW-V0-019` zero-citation no-op, which `dogfood finish` relies on after strict CEM status
  (`local-completion-policy-v0.md`). `dogfood change` MUST read each of its file inputs through at
  most its bound plus one sentinel byte: `DOGFOOD_INTENTS_FILE` 8208 bytes (16 paths of 512 bytes
  with their LF), over which `ocm-aggregate` is NOT_PRODUCED `intent-manifest-over-bound`;
  `DOGFOOD_VERIFY_FILE` 64 KiB, over which `local-outcome` is NOT_PRODUCED
  `verify-file-over-bound`; and each `DCW-V0-031` agent receipt 4 MiB. Under `DCW-V0-031` a receipt
  over that bound MUST print `NOT_OBSERVED agent-receipt-over-bound`, one that cannot be opened
  `NOT_OBSERVED agent-receipt-unreadable`, and one that does not decode as a receipt object
  `NOT_OBSERVED agent-receipt-malformed`; `agent-receipt-tree-unknown` stays for a receipt that
  decodes but names no `context.revision`. A plan refused because the map cannot be read stays
  `citation-plan-map-mismatch`. Each new refusal MUST have its own `fix:` line, and the receipt
  notes stay non-blocking. The rule binds `dogfood change` only; the bash range loop
  (`script/dogfood-bind-range.sh`) keeps its empty-plan no-op.
  Reason: an empty plan on an uncited map was a silent zero-citation pass that only `cem-status`
  caught later, these
  three inputs were read whole before any bound was checked, and a malformed receipt was
  indistinguishable from one that names no tree.
- `DCW-V0-033`: (accepted by decision 0441; V1-0940) This amends the stdout
  parity of `DCW-V0-020` for a passing check only. On PASS, `dogfood check` and the check inside
  `dogfood seal` MUST print one line `dogfood-check: SUMMARY cem=STATE hunks=N supported=N
  unknown=N mechanical=N ocm=STATE [requirements=N linked=N unlinked=N]
  report=.corvint/dogfood-report.json detail=<git-dir>/corvint/dogfood-check.stdout` (amended by
  `DCW-V0-046`, which adds `unknownShare=U/M reasonedUnknown=N` after `mechanical=N`) in place of
  the CEM and OCM status JSON, then any `DCW-V0-024` note and the unchanged terminator
  `dogfood-check: PASS`. The detail file MUST hold the exact JSON lines the former output printed,
  written owner-only after removing whatever was at that path. A state that is not 1 to 64 letters,
  digits, `-` or `_`, or a field the JSON does not carry, prints `NOT_OBSERVED`, so the line cannot
  invent a value or be split; `ocm=NOT_ASSESSED` marks a declared no-intent change.
  `DOGFOOD_VERBOSE=1`, or a detail file that cannot be written, MUST print the former output
  byte for byte. A failing CEM policy MUST still print its full status JSON before the `FAIL`
  line, because that JSON is the repair detail; every other FAIL, REFUSE and NOTE line is unchanged.
  Non-goals: no change to the report, the CEM, the OCM, the aggregate capture or any exit status;
  the seal line is unchanged.
  Failure modes: a parser that read the status JSON from check stdout gets the summary instead
  and must read the detail file or set `DOGFOOD_VERBOSE=1`; the in-repository consumers read only
  the `dogfood-check: PASS` suffix (`internal/localcompletion` aggregate capture, the daily-loop
  harnesses) or the report.
  Reason: a passing check printed about 2.1 KiB of JSON that the agent running it rarely reads;
  measurement is in `docs/build-log/2026-10-07-hook-context-and-dogfood-summary.md`.

- `DCW-V0-034`: (accepted, decision 0505) Before invoking
  the local-outcome recorder, `dogfood change` MUST validate every verification line
  with the recorder's shared command validator, including its 512-character and
  50-unique-command bounds, on both pre-commit and post-commit binding passes.
  Unsupported syntax MUST produce `local-outcome: unsupported-verify-syntax` and name
  the offending command and accepted syntax, applying the existing DCW-V0-014
  remediation without changing that accepted requirement. Clean verification files
  MUST retain existing recorder behavior. Non-goal: accepting shell syntax or executing
  these commands. Rollback: revert early validation; recorder admission remains intact.
- `DCW-V0-035`: (accepted, decision 0510) The coordinator
  MUST refuse more than 256 LF-terminated citation rows as `citation-plan-row-limit-exceeded`
  and more than 256 OCM-link rows as `ocm-link-plan-row-limit-exceeded`, separately from
  malformed-plan refusals. Each refusal's fix text MUST name the bound, limit and observed
  row count; exactly 256 valid rows MUST remain admissible. No bound is raised.
- `DCW-V0-036`: (accepted, decision 0510) Trace recording, changed-path admission and
  stored-path validation MUST refuse more than 200 distinct normalized paths per path list
  with `trace-path-limit-exceeded`, the path-list name, limit 200 and observed count.
  The `record` and `dogfood-record` JSON refusal envelopes MUST expose
  `trace-path-limit-exceeded` as their public `code`, including stored-path overflow.
  The internal `admitted-path-limit` classification MUST remain available to changed-path
  admission and aggregate fallback consumers; it MUST NOT replace the public code.
  Exactly 200 valid paths MUST remain admissible; no truncation may hide the excess.
- `DCW-V0-037`: (accepted, decision 0510) `docs/DOGFOOD.md`
  MUST give exact staged coordinator steps for one commit with more than 256 cited hunks:
  retain one base/patch and CEM, apply explicit plans of at most 256 rows, name every remaining
  unknown hunk in the last plan, commit only the fully cited sidecar, and rerun/check the same
  base. A 279-hunk fixture MUST finish with supported=279 and unknown=0. Retroactive range
  tooling MUST NOT be advertised as staging one oversized commit. Trace and OCM-link bounds
  remain independent; missing intent or evidence MUST stay unknown.

- `DCW-V0-038`: accepted, decision 0510. Every build in Corvint's
  dogfood wrappers MUST select the first nonempty cache from `CORVINT_GOCACHE`, caller
  `GOCACHE`, then `/tmp/corvint-go-build-cache`: change's current-tree build and check's
  current-tree and archived-base verifier builds, including check invoked by seal.
  The Make targets MUST pass a selected `CORVINT_GOCACHE` (including a command-line value)
  and preserve caller `GOCACHE` as the fallback. Cache selection MUST NOT alter verifier
  source/version identity, strict checks or refusal behavior, or clean unrelated caches.

- `DCW-V0-039`: (accepted, decision 0519 (V1-1151)) `corvint dogfood
  batch-check [--verifier PATH] BASE` (and `make dogfood-batch-check BASE=<sha>`) MUST check a
  merge batch of changes that were each bound, checked and sealed by the daily loop on their own
  branches, without rebinding the batch as one change and without raising any limit of the single
  change loop. It MUST be read-only: it writes no repository object, ref, index, worktree file,
  trace or ledger, and takes no operation lock; its Git reads disable fsmonitor and the untracked
  cache, its cleanliness probe is Core's hardened status, which refuses a configured clean or
  process filter rather than running it, and merge re-derivation runs `git merge-tree
  --write-tree` in a private temporary bare repository, removed on exit, that reads the
  repository's objects as an alternate and applies none of its configuration or attributes, nor
  system or global attributes, so no configured merge driver, filter or hook runs. A temporary
  directory that resolves inside the worktree or Git directory is refused before any scratch file
  exists. It MUST
  refuse (exit 2) an unavailable or unanchored `BASE` (the base-anchoring codes), a `BASE` that is
  not an ancestor of `HEAD`, a dirty worktree, an empty range, a range over 1,024 commits and more
  than 256 sealed changes, and MUST fail (exit 1) unless all of the following hold:
  (a) the constituents are exactly the `.corvint/changes/<bind>.cem.json` files `BASE..HEAD` adds;
  no sealed file is modified or deleted, every added file under `.corvint/changes/` has that form,
  and `HEAD` tracks `.corvint/change.cem.json` only if `BASE` tracks the same blob;
  (b) each constituent's bind commit is in `HEAD`, its sealed blob equals that bind commit's
  `.corvint/change.cem.json` and is a canonical `cem/0.2` or `cem/0.3` map or a `cem/1.0` map
  (a `cem/0.1` map verifies a cached patch, not the committed change), its `baseRevision` is an
  ancestor of the bind commit, and the verifier's `cem status --map <sealed path> --expected-base
  <its base> --target <bind> --max-mechanical 0` exits 0 and reports `ok`, `ready-for-ci`, a valid
  `canonical` verification and exactly that base and bind as base and target revisions, or, for a
  `cem/1.0` map run with `--artifacts <absolute repository root>`, `ok`, `accepted` and a stable
  `ACCEPT` with exactly that base and bind as `expectedBase` and `targetRevision`; the map's
  declared spec selects the form, so neither form proves the other's map (`CEM-V1-148`; an exit
  status or unparseable output alone is never evidence; counts it does not report print
  `NOT_OBSERVED`), with `--max-unknown` equal to its bootstrap unknowns: whole-file
  `no-evidence` unknown hunks on paths absent at its base, each printed, plus its reasoned unknown
  hunks (`DCW-V0-046`);
  (c) every commit of `BASE..HEAD` is in some constituent's own range (bind commit not reachable
  from its base or `BASE`), is that constituent's rename-only seal commit, or is a two-parent merge
  whose tree equals its first parent's tree or the tree `git merge-tree` derives from its parents
  with the default, `-Xours` or `-Xtheirs` strategy, each derivation printed per merge; a merge
  that relied on a configured merge driver or merge attribute is therefore not derivable.
  It MUST print every failure with its reason, count, the listed commits, paths or binds, and one
  `fix:` recovery line, and on success MUST print, besides `PASS`, that OCM is not reverified,
  the composition of merged changes is not observed, the count of merges that dropped a conflict
  side, that no local outcome is produced, and that one verifier ran.
  Non-goals: semantic proof that merged changes compose; reverifying OCM or the local outcome of a
  constituent; retroactive `dogfood-bind-range` bindings, whose maps exist only in the binding
  commit while the verifier reads a map from a worktree path the read-only check cannot write; manual merge resolutions; octopus merges; a second, independent verifier.
  Failure modes: an orchestrator edit committed straight to the batch branch, or a hand-resolved
  merge, fails with its recovery rather than passing; a constituent sealed against a base the
  verifier cannot reach fails `constituent-cem-policy`; a map reused on a bind commit with an
  extra committed edit fails canonical verification; a verifier that exits 0 without canonical
  `ready-for-ci` output fails `constituent-unverified`; a root commit of an unrelated history fails
  `unbound-commits`; a batch over a bound refuses with a
  split recovery instead of truncating.
  Reason: the w4 batch of 2026-10-11 (PR #789, over 200 paths and about 900 hunks) could not be
  bound as one change: `git-budget-exceeded`, `ocm-prepare NOT_PRODUCED`, `intent-scope-drift` and
  `admitted-path-limit`, then `dogfood-report-drift`; its lanes had each been sealed already.
- `DCW-V0-040`: (accepted, decision 0519 (V1-1151)) When `dogfood change`
  reports `git-budget-exceeded` on any step or `admitted-path-limit` on `local-outcome`, and when it
  refuses a change that adds a sealed CEM, it MUST print a `fix:` line naming the bounded recovery:
  bind and seal the work as smaller changes on their own branches, merge them, and run
  `dogfood batch-check` (DCW-V0-039). `ocm-aggregate: intent-scope-drift` keeps its existing `fix:`
  line, which names the `ocm-prepare` or `ocm-status` row above it; on an oversized change that row
  carries `git-budget-exceeded` and so this recovery. It MUST NOT raise the 1,024-operation
  verifier budget, the 200-path learning limit or the intent scope.
- `DCW-V0-041`: (accepted, decision 0519 (V1-1171)) A citation plan row MAY
  cite any regular file present at `BASE`, including source and test files, with any registered
  relation (`call-site`, `decision`, `dependency`, `implementation`, `incident`, `specification`,
  `test-claim`), and one hunk MAY carry several rows. `docs/DOGFOOD.md` MUST say so. When
  `cem cite` refuses a row with `invalid-arguments` or `missing-evidence`, `dogfood change` MUST
  name the row and the accepted relations or the at-`BASE` requirement in a `fix:` line; a hunk
  left uncited stays an explicit `unknown` with reason `no-evidence`, never support.
- `DCW-V0-042`: (accepted 2026-10-11, decision 0520; V1-1219) `dogfood change` MUST
  admit a `BASE..HEAD` range that adds sealed CEMs when each added file under `.corvint/changes/`
  is an unmodified, verifiable sealed change the branch merged or stacked on: it has the
  `.corvint/changes/<bind>.cem.json` form, `BASE..HEAD` adds it (status `A`), and it passes the
  same re-verification `DCW-V0-039` (b) applies to a batch constituent, with the change's step
  runner as verifier. It MUST print one `NOTE sealed-cem-merged` line per admitted file naming the
  bind commit, its base and its bootstrap unknowns, and MUST bind the whole range against the
  unmoved `BASE`, so the merged hunks and each admitted sealed file are hunks the plan cites like
  any other. It MUST still refuse `sealed-cem-in-change` (exit 2, before any step runs or any
  file is written) when any file under `.corvint/changes/` is modified, deleted, misnamed, not
  merged, drifted from its bind commit's map, not canonical, or not proved `ready-for-ci`,
  printing one line per refused path with its `DCW-V0-039` reason and the `DCW-V0-040` `fix:`
  line; when more than 256 files under `.corvint/changes/` change it refuses, verifying none,
  with the one line `sealed CEMs=<n> exceed 256` in place of the per-path lines. This amends the
  decision 0319 consequence that any sealed CEM in the range refuses; a sealed branch reworked in
  place now rebinds as a stacked change.
  Non-goals: reverifying the merged changes' OCM or local outcome; proving that merged changes
  compose; raising any limit of the single change loop.
  Failure modes: a merged sealed file that was edited after the merge fails `sealed-cem-drift`; a
  verifier that cannot reach a lane's base fails `constituent-cem-policy`; a batch too large for
  one change still fails on its own budget and names `batch-check`.
  Reason: lanes that merged another lane's sealed CEM could bind only by moving `BASE` to the
  merge commit (V1-1219, w6 lane report of 2026-10-11).
- `DCW-V0-043`: (accepted 2026-10-11, decision 0520; V1-1198) The inherited archive of
  a `BASE` is `.corvint/changes/<writer>.cem.json` holding exactly the blob `BASE` tracks at
  `.corvint/change.cem.json`, `<writer>` being the last commit that wrote that path. In a
  repository that seals (`BASE` tracks at least one file under `.corvint/changes/`), when `BASE`
  tracks `.corvint/change.cem.json` and no `.corvint/changes/` file at `HEAD` has its blob,
  `dogfood change` MUST refuse `unarchived-base-cem` (exit 2, before any step runs or any file is
  written), naming `<writer>` and, as its `fix:` line, the exact command that commits the
  inherited archive and nothing else; `dogfood seal`'s `DCW-V0-027` refusal MUST print the same
  command. `dogfood change` MUST admit an added inherited archive without reverifying it and print
  `NOTE inherited-cem-archived NOT_REVERIFIED path=<path>`. `dogfood batch-check` MUST treat it as
  no constituent, MUST accept `HEAD` dropping `.corvint/change.cem.json` when `HEAD` keeps the
  inherited archive (this amends `DCW-V0-039` (a)), MUST count as covered a single-parent commit
  whose only change adds the inherited archive or renames `.corvint/change.cem.json` to it (this
  amends `DCW-V0-039` (c)), and MUST print the same note. A batch whose `HEAD` tracks a
  `.corvint/change.cem.json` that `BASE` does not track with the same blob still fails
  `unsealed-cem-at-head` before it is merged.
  Non-goals: preventing a plain `git merge` outside the loop; sealing or reverifying the inherited
  CEM, which the loop never checked; changing the rename-only seal commit.
  Failure modes: an archive with another blob or name is not the inherited archive and is refused
  as a sealed CEM (`DCW-V0-042`); a lane that deletes the inherited CEM without archiving it fails
  `unsealed-cem-at-head`; a repository that never sealed keeps rebinding its tracked CEM in place.
  Reason: the w4 batch merge (#789) left an unsealed `.corvint/change.cem.json` tracked on `main`,
  and every later `dogfood-seal` refused `unarchived-base-cem` only after the change was bound and
  checked (V1-1198).
- `DCW-V0-044`: (accepted 2026-10-11, decision 0520; V1-1209) The daily loop's local
  trace store (`.context-corvint/traces/`), its evidence directory (`<git-dir>/corvint/`) and its
  operation lock MUST belong to one worktree, so `dogfood change`, `dogfood check` and
  `dogfood seal` run in two `git worktree add` worktrees of one repository at the same time without
  either refusing and without writing the primary worktree's store. When a recorded trace names a
  commit `HEAD` cannot reach, the `local trace store:` line MUST also name this recovery: run each
  lane in its own worktree or clone rather than switching branches in one. `docs/DOGFOOD.md` MUST
  say that lanes sharing a repository each use their own worktree or clone.
  Non-goals: one trace store shared or keyed per branch within a worktree, which would change the
  local trace contract (`LTPM-V0`); sharing traces across worktrees.
  Failure modes: a second lane run in the first lane's worktree still refuses
  `unsupported-query-trace-state`, now naming the per-lane worktree.
  Reason: lanes that took turns in one working directory found the second lane's check refused by
  the first lane's traces (V1-1209, w6 lane report of 2026-10-11).
- `DCW-V0-045`: (accepted 2026-10-11, decision 0520; V1-1207) `dogfood check`'s
  cleanliness probe MUST be Core's hardened status (`gokernel.ProbeRepositoryContext`), as
  `dogfood batch-check`'s is, so a repository-configured clean or process filter never runs: it
  MUST refuse `worktree-status-unavailable` (exit 2) with the probe's reason and a `fix:` line
  naming the configuration or a plain clone. Otherwise it MUST keep `dirty-worktree`,
  `uncommitted-change-not-in-base-target` with its `required order:` line, and
  `PASS no-change-clean-worktree`, counting tracked and untracked dirty paths as before.
  Non-goals: hardening the other Git reads of the check, which read commits and trees rather than
  the worktree; checking a repository that needs its filter.
  Reason: `requireCleanChange` ran `git status`, which applies a configured clean filter to a
  re-stated file, so the read-only check could run repository-owned code (V1-1207).
- `DCW-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11), decision 0523 P3; V1-1258) A `DOGFOOD_CITATIONS` row
  whose second field is literally `unknown` and whose third field contains no `:` is an unknown
  row, `HUNK<TAB>unknown<TAB>REASON<TAB>NOTE`; a citation row's third field is always a
  `START:END` span, so the two forms cannot be confused. REASON MUST be `insufficient-evidence` or
  `conflicting-evidence`, registered CEM unknown reasons other than the prepared `no-evidence`,
  and NOTE MUST contain at least one letter or digit; any other unknown row MUST refuse the whole
  plan as `invalid-citation-plan` and run no cite. A hunk a resumed map already records as
  `unknown` with one of those reasons was marked by an earlier staged plan and needs no row. An
  accepted unknown row names its hunk for `DCW-V0-019` and runs `cem mark --disposition unknown --reason REASON` on it in plan order,
  staged as citations are, so the map, and the sealed map after it, records the reason. The
  accepted plan's unknown rows, notes included, MUST be kept owner-only in
  `<git-dir>/corvint/cem-unknown-reasons.tsv` as `HUNK_ID<TAB>REASON<TAB>NOTE` lines in map
  order. A line an earlier staged plan recorded is kept while its hunk is still `unknown` with
  the same reason; the current plan's rows apply in order, an unknown row replacing that hunk's
  line and a citation dropping it. The lines are prepared before any cite:
  a directory at that path, or an earlier file or a result over 16 MiB, refuses (never truncates),
  and a non-regular file there is not read. The file is replaced, owner-only, only after the plan
  is cited, so a refused or failed stage leaves the earlier notes as they were, and it is removed
  when no line remains; when preparing or replacing it fails, `cem-cite` MUST be NOT_PRODUCED
  `unknown-reasons-not-recorded`, and a preparation failure cites nothing. The CEM unknown reason vocabulary is closed,
  so the note is not in the map. `dogfood change`, `dogfood check`
  and `dogfood batch-check` MUST run `cem status` with `--max-unknown` equal to the bootstrap
  allowance plus the count of map hunks that are `unknown` with one of those two reasons, not
  counting a bootstrap hunk twice; an unknown hunk with `no-evidence` or no reason is never
  admitted. On PASS, the `DCW-V0-033` SUMMARY line MUST carry `unknownShare=U/M
  reasonedUnknown=R` after `mechanical=N`, where R counts every reasoned unknown hunk, a bootstrap
  one included, U is the map's unknown hunks, M its material
  (non-mechanical) hunks, and `unknownShare=NOT_OBSERVED` when a count is absent or inconsistent;
  each `dogfood batch-check` CONSTITUENT line MUST carry `reasonedUnknown=N` after
  `bootstrapUnknown=N`. The report's `maximumUnknownAfterBootstrap: 0` keeps its wire value and
  counts unreasoned unknown hunks after the bootstrap and reasoned allowances.
  Non-goals: free-text unknown reasons in the CEM wire; emitting the stable CEM profile; a share
  threshold enforced by the check, since the `docs/CHANGE-EVIDENCE-MAP.md` kill condition is an
  evaluation outcome rather than a per-change gate.
  Failure modes: an author can still keep a hunk unknown that a citation would have supported;
  the recorded reason and note make that choice visible to the reviewer instead of hiding it.
  The note survives only in the private evidence directory, not in the sealed map.
  Reason: refusing every non-bootstrap unknown made the supported share a workflow artifact, so
  the `>30%` unknown kill condition of `docs/CHANGE-EVIDENCE-MAP.md` could never fire (decision
  0523 P3, `docs/build-log/2026-10-11-cem-1-0-expert-panel.md`).
- `DCW-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1279) A change
  whose canonical `cem status` output differs from the base verifier's by construction MUST
  declare it as a declared output change: `dogfood check`, and `dogfood seal` through it, read
  `DOGFOOD_OUTPUT_CHANGE=REQUIREMENT-ID`. Unset or empty, the check is byte-for-byte unchanged and
  every verifier comparison keeps the byte-identical stdout, stderr and status rule. Set, it is
  admitted, after the report is read and before any verifier runs, only when the ID is a line's
  first field in `docs/specs/REQUIREMENTS.tsv` at `HEAD` (read from the commit, at most 16 MiB)
  whose second field is a `docs/specs/*.md` path that `BASE..HEAD` modifies; an absent, malformed
  or unreadable ID MUST refuse `output-change-requirement-unknown`, an unmodified spec
  `output-change-not-in-diff`, and an enrolled aggregate check, whose closed report schema has no
  member for it, `output-change-aggregate-unsupported`, each with exit 2 and one explanatory line.
  An empty `BASE..HEAD` range never admits it: the declaration is checked before that range's
  `no-change-clean-worktree` PASS, so an unknown ID refuses `output-change-requirement-unknown` and
  a known one `output-change-not-in-diff`. When admitted, the `cem status` comparison MUST run the tree verifier in both roles, still run
  the base verifier, and require the tree verifier to agree with itself and with any override; it
  records `base-independence=NOT_PROVED base=sha256:B tree=sha256:T declared=ID`, B and T being the
  digests of the base and tree verifiers' `cem status` stdout, on the `DCW-V0-033` SUMMARY line
  before `report=` (as a `dogfood-check: NOTE` line under `DOGFOOD_VERBOSE=1`), and in the report's
  `dogfoodCheck` member as `"baseIndependence": {"state": "NOT_PROVED", "declared": ID,
  "baseStatusStdoutSha256": B, "treeStatusStdoutSha256": T}` after `outputsAgree`, which then means
  the tree verifier's self-agreement, as in `LCP-V0-014` where `dogfood finish` runs one binary in
  every role and proves no base independence. `dogfood seal`, and `script/dogfood-seal.sh` behind
  `make dogfood-seal` (copied from the check's SUMMARY or NOTE line), MUST keep the same fields on
  its PASS line, after `sealed=`. When admitted, the `dogfood-ocm status` comparison MUST follow the
  same tree-in-both-roles rule (amended by V1-1276: the first change that prepares a `cem/1.0` map
  makes OCM read a map its base verifier refuses `unsupported-spec`); its stdout digests are not
  recorded and the `base-independence=NOT_PROVED` fields cover it. The `impact` abstention
  comparison (`verifyAbstention`) keeps the byte-identical rule unchanged.
  Non-goals: weakening the byte-identical rule for an ordinary change; inferring that an output
  change is additive; a declaration in `dogfood finish` (`LCP-V0-014`) or `dogfood batch-check`.
  Failure modes: a declaration admits any `cem status` difference, not only the intended member,
  so the reviewer MUST compare the two recorded digests' outputs against the declared requirement;
  an admitted declaration records NOT_PROVED even when the outputs happen to agree.
  Reason: a change that adds a member to the canonical verification object can never agree with
  the base verifier, so it either failed `verifier-disagreement` or gated its output behind an
  opt-in flag (V1-1279; V1-1265 landed under that workaround).
- `DCW-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1281) A change
  is a chore when `git diff --no-renames --name-only BASE HEAD` names at least one path and every
  path is exactly `.corvint/change.cem.json` or `.corvint/changes/NAME.cem.json`, NAME being one
  nonempty path segment; a path Git quotes never matches, and any other change, an empty one
  included, is not a chore. Before any cite, after the `DCW-V0-019` binding passes, `dogfood
  change` MUST refuse a uniform plan as `cem-cite` `citation-plan-uniform`, citing and marking
  nothing, unless the change is a chore. A plan is uniform when the map prepared in the run has
  at least two hunks, the plan has no unknown row (`DCW-V0-046`), every citation row carries the
  same evidence PATH and `START:END` span, compared as the decimal integers `cem cite` parses (so
  `01:02` is `1:2`), whatever its RELATION, and every map hunk is named by
  ordinal or full hunk ID. Its `fix:` line MUST name the hunk count and the span as
  `hunks=N span=PATH:START:END` and direct a per-hunk citation of the BASE lines each hunk
  depends on or implements. A plan that keeps a hunk unknown or leaves one unnamed, a
  single-hunk map, and a resumed staged plan naming only some hunks are not uniform; one span
  MAY serve several hunks.
  Non-goals: judging whether a cited span supports its hunk; refusing a plan that cites two
  spans for many hunks; refusing an earlier committed map, which stays valid CEM.
  Failure modes: a plan can still pass with two near-identical spans, or by keeping one hunk
  unknown; the measured citation shape (`CEM-V1-080..082`) stays the evidence of support, and
  this refusal only removes the one-span-for-everything shape that measured invariant.
  Reason: 171 of the 180 invariant committed maps of the V1-1262 citation baseline cite a single
  evidence span, 161 of them over more than one hunk, because the bind loop accepted one row
  per hunk all naming the same span
  (`docs/build-log/2026-10-12-cem-citation-baseline-593-maps.md`).
- `DCW-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1281) When the
  map `cem prepare` writes in a `dogfood change` run declares `cem/1.0`, and the change is a
  chore by `DCW-V0-048`, the run MUST write the stable map member `role` before any cite:
  `seal-chore` when any `.corvint/changes/` path is touched, otherwise `bind-chore`
  (`CEM-V1-095`). It rewrites only a regular, non-symlink map whose wire and reference graph
  parse, through the stable encoder, keeping the file mode; a map that already carries that role
  stays byte-identical, and a map it cannot rewrite is treated as not produced in that run. A map
  for any other change carries no role, which reads as `change`. A Core `cem/0.x` map MUST
  never receive a role: its wire has no such member. Since the stable producer cutover (V1-1276,
  `CEM-V1-140`, `CEM-V1-147`) `cem prepare` writes `cem/1.0` by default, so the write is live;
  an explicit `--spec cem/0.2` prepare still receives no role. `cem status --citation` on a `cem/1.0` map reports the class
  beside the result (`CEM-V1-106`): `change` for a map with no role or role `change`, `chore`
  for `bind-chore` or `seal-chore`. The maintained tool `tools/cem-citation-baseline` reproduces
  the V1-1262 baseline rows and summary over a map directory with each row's class, and the
  summary for all maps, class `change` and class `chore`; a `cem/0.x` map, which has no role, is
  classed by its `BASE..TARGET` paths under the `DCW-V0-048` rule.
  Non-goals: adding `role` to the `cem/0.x` wire; a role for a change map; letting the role
  change any verdict, tier, issue or result member; a rerun of the baseline before V1-1276.
  Failure modes: before V1-1276 no committed map carries a role, so the class of every
  committed map comes from its paths; none of the 593 committed maps is a chore by that rule,
  so the chore class is empty today, and the self-referential invariant maps are change maps
  bound in chore-subject commits, which `DCW-V0-048` addresses instead.
  Reason: the V1-1262 baseline mixed self-referential bind and seal maps with change maps; a
  distinct class keeps a chore's one-span citation out of the change-map support measure.
- `DCW-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) Each
  migration commit, which rewrites committed sealed CEMs to `cem/1.0` in place (`CEM-V1-230`), is
  the seal baseline from that commit on. The migration commits are V1-1277's commit and any
  follow-up commit that converts maps a pre-cutover branch landed later; they are listed in
  `cemMigrationCommits` in `internal/dogfoodflow/migration.go`. When `BASE..HEAD` contains a
  migration commit and `BASE` does not, `dogfood change`'s sealed-CEM admission and
  `dogfood batch-check` MUST admit a modified `.corvint/changes/<bind>.cem.json` path whose `BASE`
  blob is exactly that commit's preimage and whose `HEAD` blob is exactly its rewrite: the path is
  not `sealed-cem-modified`, it is not a batch constituent, it does not count toward the
  256-constituent limit, and each command prints one
  `NOTE sealed-cem-migrated maps=N commit=<migration commits in the range>` line. Every path such
  a migration commit rewrote that `HEAD` holds with bytes the migration did not write, including a
  merge that keeps the pre-migration tree and so hides the rewrite from the net `BASE..HEAD` diff,
  MUST fail `sealed-cem-migration-reverted`. Any other modification of such a path, before or
  after, still fails `sealed-cem-modified`. `batch-check` MUST count a listed migration commit
  itself as bound only when it modifies sealed CEM paths and nothing else. A constituent that
  `BASE..HEAD` adds before a migration commit, whose `HEAD` blob is the migration's rewrite of its
  bind commit's map, MUST fail `constituent-migrated` with a `fix:` line that splits the batch at
  the migration commit, never pass and never fail `sealed-cem-drift`: its bind commit's sidecar
  still holds the source bytes, so the stable verifier cannot verify the rewrite there
  (`CEM-V1-104`). While the list is empty every path keeps its former result.
  Non-goals: re-binding any map or bind commit; a general allow-list of rewrites; admitting a
  migrated constituent in a batch that spans the migration commit.
  Failure modes: a rebase or squash that changes a migration commit's ID disables its admission,
  so the migrated paths fail `sealed-cem-modified` closed until the list names the landed commit;
  a follow-up migration commit not yet listed fails the same way; a later edit of a migrated map
  fails `sealed-cem-modified`; a restored pre-migration map fails
  `sealed-cem-migration-reverted`; a sealed map that a branch adds after a migration commit is
  checked as before.
  Reason: decision 0525 §3 step 6 rewrites the sealed corpus once; without a baseline every
  later range that spans the rewrite would fail closed on 614 sealed paths.

## Code vocabulary

The daily subverbs (`DCW-V0-020`) emit these codes. A change-step code marks that step
`NOT_PRODUCED`; a check code fails `dogfood check`; a window code is reported `NOT_OBSERVED`.

Base anchoring, refused by every subverb:

- `anchor-ref-unavailable`: the configured anchor ref, or its merge base with `HEAD`, cannot be read.
- `multiple-anchor-refs`: more than one anchor ref is configured.
- `anchor-merge-base-ambiguous`: `HEAD` and the anchor ref have no single merge base.
- `base-not-anchored`: `BASE` is not that merge base.

`dogfood change` steps:

- `context-abstention-evidence-failed`, `context-abstention-invalid`: the coordination-time `impact`
  abstention did not run, or did not refuse with exactly one well-formed `unsupported-impact-range`,
  `unsupported-impact-repository` or `unsupported-impact-path` envelope (`DCW-V0-025`).
- `citation-plan-not-provided`, `citation-plan-unavailable`, `invalid-citation-plan`: no citation
  plan was given, its path is not a regular file, or its rows are malformed.
- `citation-plan-row-limit-exceeded`: the citation plan has more than 256 rows; the fix text names
  the limit and observed count (`DCW-V0-035`, accepted, decision 0510).
- `empty-citation-plan`, `intent-manifest-over-bound`, `verify-file-over-bound`: the citation plan
  is empty while the map still owes a hunk, or the intents or verify file exceeds its bound
  (`DCW-V0-032`, accepted).
- `citation-stage-exists`, `citation-stage-cleanup-failed`: a staged citation file from an earlier
  pass is still present, or could not be removed.
- `unknown-reasons-not-recorded`: the plan's unknown rows could not be kept in
  `<git-dir>/corvint/cem-unknown-reasons.tsv`. When preparing the notes fails nothing is cited;
  when replacing the file fails after the cite, the map has changed, the earlier notes are kept,
  and rerunning the same plan replays it (`DCW-V0-046`, accepted).
- `citation-plan-uniform`: the plan cites one span for every hunk of a non-chore change whose
  map has at least two hunks; nothing is cited and the fix text names the hunk count and span
  (`DCW-V0-048`, accepted).
- `cem-map-not-produced`: `cem-prepare` failed in this run or left no `.corvint/change.cem.json`,
  so nothing is cited; a map left by an earlier run is never cited (proposed 2026-09-25, V1-0228,
  not accepted).
- `cem-map-stable-unsupported`: retired by V1-1276 (`CEM-V1-147`): the bind loop cites, checks
  and seals a `cem/1.0` map like a `cem/0.x` map, so no run reports it; the observation ledger
  still admits the reason so earlier entries replay.
- `ocm-link-plan-unavailable`, `empty-ocm-link-plan`, `invalid-ocm-link-plan`: the
  `DOGFOOD_OCM_LINKS` plan is missing, empty, or malformed.
- `ocm-link-plan-row-limit-exceeded`: the `DOGFOOD_OCM_LINKS` plan has more than 256 rows; the fix
  text names the limit and observed count (`DCW-V0-035`, accepted, decision 0510).
- `ocm-map-not-prepared`: the `ocm-link-NNN` row's intent map did not prepare, so the row was not
  run (`DCW-V0-028`, proposed).
- `no-intent-declared`: the change declared no intent (`DCW-V0-024`); the `ocm-prepare`,
  `ocm-status` and `ocm-aggregate` rows carry it and, unlike every other code here, it does not
  block `"complete": true`.

`dogfood check`:

- `cem-map-missing`: `.corvint/change.cem.json` is not present to check.
- `cem-policy`, `ocm-policy`: the CEM or OCM policy check exited non-zero; `ocm-policy` is used
  when the OCM check names no code of its own.
- `context-abstention-evidence-drift`: the report's abstention row, the retained artifact and its
  recorded digest disagree.
- `dogfood-report-write`: the report could not be written.
- `worktree-status-unavailable` (exit 2): Core's hardened status refused the worktree, for example
  a configured clean or process filter; its reason is printed (`DCW-V0-045`, accepted).
- `output-change-requirement-unknown`, `output-change-not-in-diff`,
  `output-change-aggregate-unsupported` (exit 2): `DOGFOOD_OUTPUT_CHANGE` names no requirement
  `REQUIREMENTS.tsv` at `HEAD` maps to a spec, names one whose spec `BASE..HEAD` does not modify, or
  is set for an enrolled aggregate check (`DCW-V0-047`, accepted).
- `previous-cem-absent`, `previous-cem-base-unavailable`, `window-unavailable`,
  `window-exceeds-256-commits`, `window-cem-base-unavailable`: the window of commits since the
  previous CEM cannot be observed.

`dogfood change` refusals, exit 2 before any step runs:

- `sealed-cem-in-change`: `BASE..HEAD` adds, modifies or deletes a file under `.corvint/changes/`
  that is not an unmodified sealed change re-verified against its own base and bind commit; each
  refused path is printed with its `DCW-V0-039` reason (`DCW-V0-042`, accepted).
- `unarchived-base-cem`: in a repository that seals, `BASE` tracks an unsealed
  `.corvint/change.cem.json` that no `.corvint/changes/` file at `HEAD` keeps; the `fix:` line is
  the exact command that commits its inherited archive (`DCW-V0-043`, accepted).

`dogfood seal`:

- `nothing-to-seal`: the bind commit at `HEAD` does not track `.corvint/change.cem.json`.
- `unarchived-base-cem`: sealing would drop a CEM `BASE` tracks that no `.corvint/changes/` file
  keeps (`DCW-V0-027`, proposed); the `fix:` line is the inherited-archive command (`DCW-V0-043`,
  accepted).

`dogfood batch-check` (`DCW-V0-039`): refusals `base-unavailable`,
`base-not-ancestor-of-head`, `dirty-worktree`, `empty-range`, `range-exceeds-1024-commits`,
`constituents-exceed-256`, `worktree-status-unavailable` (Core's hardened status refused, for
example a configured filter; its reason is printed), `scratch-inside-repository` and
`scratch-repository-unavailable`, plus the base-anchoring codes;
failures, each with one `fix:` line:

- `sealed-cem-modified`, `sealed-cem-drift`, `unrecognized-sealed-cem`: a sealed file was modified
  or deleted, differs from its bind commit's map, or is not named for a 40-hex bind commit.
- `unsealed-cem-at-head`: `HEAD` tracks a `.corvint/change.cem.json` that `BASE` does not.
- `constituent-not-merged`, `constituent-cem-invalid`, `constituent-cem-policy`,
  `constituent-unverified`: a sealed change's bind commit is not in `HEAD`, its map is unreadable
  or not canonical, its verifier exits non-zero within its bootstrap allowance, or the verifier
  exits 0 without canonical `ready-for-ci` output (`cem/0.x`) or a stable `ACCEPT` (`cem/1.0`,
  `CEM-V1-148`) for exactly that base and bind.
- `constituent-cem-stable-unsupported`: retired by V1-1276 (`CEM-V1-148`): a sealed `cem/1.0`
  constituent is verified by the stable status, not refused.
- `unbound-commits`: a non-merge commit, including a root commit, belongs to no sealed change and
  is no seal commit.
- `constituent-migrated` (`DCW-V0-050`): a constituent the range adds before a migration commit
  holds that commit's `cem/1.0` rewrite; split the batch at the migration commit.
- `sealed-cem-migration-reverted` (`DCW-V0-050`): `HEAD` holds a sealed CEM that a migration commit
  in the range rewrote, with bytes that commit did not write; restore the migration's bytes.
- `merge-resolution-unbound`, `octopus-merge-unbound`: a merge tree is not derivable from its
  parents, listing at most 20 differing paths, or the merge has more than two parents.

## Non-goals and baseline

This slice creates no daemon, dispatcher, general autonomous authority, new language rewrite,
universal proof of correctness or automatic host promotion. Existing native commands are the
baseline; add runtime machinery only for an observed missing behavior. Keep original evidence and
human authority instead of converting a version label or synthetic packet into product proof.
The daily subverbs do not install, build or upgrade Corvint, pin a version for a foreign
repository, edit its ignore rules, or change any report, receipt or CEM format beyond the
`DCW-V0-024` no-intent rows and `ocmStatus` value, and the `DCW-V0-025` abstention reasons.

## Proposed query abstention extension

Proposed amendment (2026-09-30, V1-0523) to the completion behavior constrained by
`DCW-V0-005`, `DCW-V0-015` and `DCW-V0-025`: the exact intentional authority-start trace
refusal may remain visible as a discovery abstention during structural closure only under
`dogfood-query-abstention-v0.md` QAT-V0-001..005. Only the exact QAT-V0-001 refusal, with
the QAT-V0-002..004 retained binding and independent replay, is exempt from blocking. Every other
missing, dirty, stale, unsupported or failed outcome/check/CEM/OCM input retains the completion
refusals. The distinct reason is `authority-start-trace-state-abstention` and its profile is
`corvint-dogfood-query-abstention/0`. This does not extend the three-code impact allowlist,
change its accepted profile, claim prior acceptance of this proposed technical contract, or
produce complete query evidence or query support. The accepted requirement clauses and their
historical use-case contract receipts remain unchanged.

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
| `DCW-V0-013..015` | `docs/DOGFOOD.md` "Daily adopter path"; `script/dogfood-change_test.sh` run by `TestGoOnlyContextAbstentionRemainsClosed` (the repo phase's uncommitted-sidecar pass and `assert_cited_uncommitted`); `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` (real recorder refuses the untracked sidecar of a first pass with every input); `internal/dogfoodflow/change.go` `fixHints`; `TestDogfoodChangeNamesVerifySyntaxRemediation` (V1-0354: a verification line with `^` and `$` is refused `unsupported-verify-syntax` with a `fix:` line naming the admitted syntax); `TestDogfoodChangeNamesVerifyLengthRemediation` (V1-0435: a verification line over 512 characters is refused `record-failed` with a `fix:` line naming `local-outcome.stderr` and both recorder bounds); `TestDogfoodDCWLinksSurviveRebind` (V1-0147: built-binary producer links all three obligations to test claims, refuses stale evidence after a content commit, and restores links by replaying the saved plan after CEM rebinding; structural linkage only, not full clause adequacy); scratch reproductions recorded in the V1-0010 build-log entry; `TestFlowGitIgnoresAmbientConfigurationAndReplaceRefs` (V1-0362) | implemented; the SIGINT interrupt and reviewer leg NOT_OBSERVED |
| `DCW-V0-017` | `docs/DOGFOOD.md` step 11; the reviewer case in `script/dogfood-change_test.sh`; the V1-0182 build-log entry | implemented; reviewer-side verifier agreement is out of scope by design |
| `DCW-V0-018` | `script/dogfood-change_test.sh` link phase: no plan, exact argv and order, plan digest, partial refusal across rows and intents, and unlisted-intent, CRLF, field-count, empty-item, over-256-row, empty and absent plans; the replay of the sealed LAC-V0-032 change recorded in the V1-0142 build-log entry | implemented; a link on a change delivered through this loop NOT_OBSERVED |
| `DCW-V0-019` | `internal/dogfoodflow/change.go` `citationPlanMatchesMap` (formerly `script/dogfood-change.sh` `citation_plan_matches_map`); `script/dogfood-change_test.sh` cases `stale-nine-of-ten`, `stale-ten-of-nine`, `bootstrap-omitted`, `other-omitted`, `noncanonical-ordinal`, `split-over-row-limit` and `bootstrap-escaped`, and the transient-prepare case asserting `cem-map-not-produced`; `script/dogfood-bind-range_test.sh` (bind-range mismatch, V1-0228); `TestDogfoodReasonAdmitsCitationPlanMapMismatch`; `TestDogfoodReasonAdmitsCitationStageRefusals`; `TestMapHunksReadsAnyJSONLayout` (a compact map binds like the indent-2 one, V1-0383); `TestDogfoodChangeNamesDeleteWhenACorrectedPlanJoinsEarlierCitations` (built binary: a corrected plan joins the earlier citation and prints the delete line; after the delete only the corrected citation remains); `keepMapEncoding` and `TestDogfoodChangeKeepsTheEncodingOfACommittedMapItCitesNothingInto` (built binary: a committed compact map survives a rerun byte-identical, the worktree stays clean and check passes; the rerun exits 1 without the fix, V1-0386) | implemented; observed live on a 22-hunk map at base 34e798b: a 1-row plan refused, a 22-row plan cited all 22 |
| `DCW-V0-020..021`, `DCW-V0-023` | `internal/dogfoodflow`; `cmd/corvint/dogfood_flow.go`; `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` (built binary only, in a Go repository with no `script/`, `VERSION` or `cmd/corvint`, with `PATH` resolving `corvint` to a failing impostor and `CORVINT_BIN` naming a missing file: change, bind, change, check, nested-root refusal, seal) | implemented; foreign-repository portability shown by that fixture only; a real non-Corvint repository NOT_OBSERVED; output parity with the former scripts is shown only for the strings and statuses `script/dogfood-change_test.sh` and `script/dogfood-bind-range_test.sh` assert, and exits 129, 130 and 143 are NOT_OBSERVED by a test |
| `DCW-V0-022` | `script/dogfood-change_test.sh` (through the wrappers over a built driver) and `script/dogfood-bind-range_test.sh`, run in Corvint's tree | implemented; in-tree evidence only |
| `DCW-V0-038` (accepted, decision 0510) | `script/dogfood-change.sh`, `script/dogfood-check.sh`, `Makefile`; `script/dogfood-cache_test.sh` tests every build role through direct wrappers and Make, precedence/empty fallback, archived-base source and trimpath parity, override version refusal and build failures | local stubbed build-environment evidence; real cache writes and prior containment NOT_OBSERVED |
| `DCW-V0-025` | `internal/dogfoodflow/change.go` `prechangeImpact` and `failing`, `internal/dogfoodflow/check.go` `checkContextAbstention`, `internal/dogfoodflow/flow.go` `impactAbstentions`; `TestDogfoodDailyPathCompletesWhenImpactRefusesTheRepositoryOrModuleRoot` (built binary, real refusals: a repository with no Go module and a module-root Go change each complete with their own reason, check and seal); the DCW-V0-025 cases of `script/dogfood-change_test.sh` (both codes complete and check through the wrappers; each invalid shape reports its exact reason, `context-abstention-invalid`, `exit-7` or `unsupported-impact-path-suffix`; change and check print the NOTE line for all three codes) | implemented; accepted with the `GOC-V0-009` and `ERI-V0-006` amendments (decision 0388); a real Beamfall change NOT_OBSERVED |
| `DCW-V0-026` | `internal/dogfoodflow/change.go` `run` and `prechangeImpact`, `internal/dogfoodflow/check.go` `checkContextAbstention`, `internal/localcompletion/finish.go` `terminalPaths`, `internal/observations/observations.go` `validDogfoodStep`; `TestChangeKeepsAgentPrechangeReceipts` | implemented; owner acceptance pending |
| `DCW-V0-027` (proposed) | `internal/dogfoodflow/check.go` `unarchivedBaseCEM`, `script/dogfood-seal.sh`; `TestSealRefusesToDropAnUnarchivedBaseCEM`; the V1-0137 case of `script/dogfood-change_test.sh` | implemented; not accepted |
| `DCW-V0-028` (proposed) | `internal/dogfoodflow/change.go` `skipOCMLinks`; the V1-0227 case of the link phase of `script/dogfood-change_test.sh` | implemented; not accepted |
| `DCW-V0-029` (proposed) | `internal/dogfoodflow/change.go` `recordCitationBinding` and `ordinalsMoved`; `TestChangeRefusesAPlanWhoseOrdinalsMoved` and the V1-0239 `swapped-ordinals` case of `script/dogfood-change_test.sh` | implemented; not accepted |
| `DCW-V0-030` (proposed) | none; `internal/dogfoodflow/check.go` `checkReport` still emits `dogfood-report-drift` for both cases | not implemented; needs an owner decision |
| `DCW-V0-031` (proposed) | `internal/dogfoodflow/change.go` `noteAgentReceipts`; `TestChangeNotesAbsentOrStaleAgentReceipts`; the absent-receipt lines asserted by `TestDogfoodDailyPath*` and the DCW-V0-025 case of `script/dogfood-change_test.sh` | implemented; not accepted |
| `DCW-V0-032` (accepted, decision 0430) | `internal/dogfoodflow/change.go` `citeStep`, `citationPlanMismatch`, `validateIntentManifest`, `localOutcome` and `noteAgentReceipt`; `TestChangeRefusesEmptyPlanAndOverBoundInputs` (including the empty plan admitted on a map that owes no hunk, and inputs of exactly their bound), `TestDogfoodReasonAdmitsInputRefusals` (the new step reasons reach the self-observation ledger), `TestDogfoodFinishRunsFromBinaryInForeignRepository` (finish still completes with its empty plan), the `empty` citation case of `script/dogfood-change_test.sh`, and the malformed and over-bound case of `TestChangeNotesAbsentOrStaleAgentReceipts` | implemented; not accepted |
| `DCW-V0-033` (accepted by decision 0441; V1-0940) | `internal/dogfoodflow/check.go` `verifyBinding`, `checkSummary`, `summaryString`; `cmd/corvint/dogfood_flow.go` (`DOGFOOD_VERBOSE`); `TestDCWV0033CheckSummaryAbstainsOnUnobservedFields` (missing, malformed and forged fields print `NOT_OBSERVED`), `TestDogfoodDailyPathRunsFromBinaryInForeignRepository` (exact two-line PASS, detail file holds both status lines, `DOGFOOD_VERBOSE=1` restores the former bytes, seal prints the summary, terminator and seal line), `TestDogfoodDailyPathCompletesWithDeclaredNoIntent` (`ocm=NOT_ASSESSED` then the note) | implemented; not accepted |
| `DCW-V0-034` (accepted, decision 0505) | `internal/dogfoodflow/change.go` `localOutcome`; `internal/trace/record.go` `ValidateVerificationCommands`; `TestDogfoodChangeDCWV0034VerifySyntaxBeforeBinding`, `TestDogfoodChangeDCWV0034CleanVerifyFile`, `TestDCWV0034VerificationCommandValidationBounds` | focused validation; accepted, decision 0505 |
| `DCW-V0-039` (accepted, decision 0519; V1-1151) | `internal/dogfoodflow/batch.go` `BatchCheck`; `cmd/corvint/dogfood_flow.go`; `script/dogfood-batch-check.sh` (its build uses the `DCW-V0-038` cache selection; `script/dogfood-cache_test.sh` covers it); `TestBatchCheckDCWV0039PassesSealedConstituents` (object store and worktree unchanged), `TestBatchCheckDCWV0039DerivesOneSidedResolution`, `TestBatchCheckDCWV0039RefusesDriftedEvidence`, `TestBatchCheckDCWV0039BootstrapAllowanceIsVisible`, `TestBatchCheckDCWV0039Refusals`, `TestBatchCheckDCWV0039RequiresCanonicalVerification` (cem/0.1 map; empty, malformed, incomplete, structural, wrong-target and count-less verifier output), `TestBatchCheckDCWV0039IgnoresRepositoryDriversAndHooks` (configured merge driver and fsmonitor hook never run; driver-reproducible manual merge fails), `TestBatchCheckDCWV0039ReportsUnrelatedRootCommit`, `TestBatchCheckDCWV0039RefusesRepositoryFilters` (clean filter never runs), `TestBatchCheckDCWV0039RefusesScratchInsideRepository`; `TestDogfoodBatchCheckDCWV0039SealedLanesWithCodeCitations` (built binary, real verifier: two sealed lanes merge and pass, an unbound fix-up fails, a sealed map reused on a bind commit with an omitted edit fails); live runs on the w4 batch recorded in `docs/build-log/2026-10-10-dogfood-batch-check.md` | implemented and accepted; retroactive bindings and composition NOT_OBSERVED |
| `DCW-V0-040` (accepted, decision 0519; V1-1151) | `internal/dogfoodflow/change.go` `fixHints` and the sealed-CEM refusal; `TestChangeDCWV0040NamesBatchRecoveryForOversizedChanges` (each hint matched by step and reason); `TestDogfoodBatchCheckDCWV0039SealedLanesWithCodeCitations` (built binary: rebinding the merged batch refuses `sealed-cem-in-change` and names batch-check, worktree clean); `TestBatchCheckDCWV0040NamesUnboundContentAndRecovery` (batch-check's own `fix:` lines) | implemented and accepted; a real over-budget or over-200-path refusal is not reproduced by a test |
| `DCW-V0-041` (accepted, decision 0519; V1-1171) | `internal/dogfoodflow/change.go` `fixHints`; `docs/DOGFOOD.md` (the `DOGFOOD_CITATIONS` input row and the citation-plan paragraph of section 4); `TestChangeDCWV0041NamesCitationRowFix`; `TestDogfoodBatchCheckDCWV0039SealedLanesWithCodeCitations` (a code and a spec citation on one hunk, an unregistered relation and a path absent at `BASE` each name their fix) | implemented and accepted |
| `DCW-V0-042` (accepted, decision 0520; V1-1219) | `internal/dogfoodflow/change.go` `admitSealedCEMs`, `internal/dogfoodflow/batch.go` `verifySealedCEM` (shared with batch-check); `TestChangeDCWV0042AdmitsOnlyVerifiableSealedCEMs` (two merged seals admitted with one verifier run each; an unverified seal, a modified seal and a misnamed file each refuse with their path and reason and the batch-check `fix:` line; 257 added files refuse by count with no verifier run); `TestDogfoodBatchCheckDCWV0039SealedLanesWithCodeCitations` (built binary, real verifier: a drifted merged seal refuses with a clean worktree, and the merged two-lane batch binds, checks and seals against the unmoved `BASE`, citing both sealed files' hunks) | implemented; not accepted; reproduced before the fix by the same fixture, whose rebind of the valid merged batch refused `sealed-cem-in-change` at `6c38c6c4a` |
| `DCW-V0-043` (accepted, decision 0520; V1-1198) | `internal/dogfoodflow/flow.go` `inheritedCEM`, `internal/dogfoodflow/change.go` `requireArchivedBaseCEM` and `admitSealedCEMs`, `internal/dogfoodflow/check.go` `seal`, `script/dogfood-seal.sh`, `internal/dogfoodflow/batch.go` `sealedConstituents`, `verifyCoverage` and `archivesInherited`; `TestChangeDCWV0043RefusesUnarchivedInheritedCEM` (the refusal's exact command, run through `sh`, commits only the archive, after which the change is admitted with the note and no verifier run), `TestChangeDCWV0043RefusalKeepsPreviousEvidence` (the full command refuses before it clears the previous run's step evidence), `TestChangeDCWV0043IgnoresRepositoriesThatNeverSealed`, `TestSealRefusesToDropAnUnarchivedBaseCEM` and the V1-0137 case of `script/dogfood-change_test.sh` (the seal `fix:` line), `TestBatchCheckDCWV0043AcceptsTheInheritedArchive` (an archive inside one lane's range and a standalone rename archive commit before another lane's bind both pass; a lane that drops the CEM unarchived fails `unsealed-cem-at-head`), `TestBatchCheckDCWV0043AcceptsAnArchiveAlreadyAtBase` (a sealed lane on a `BASE` that already keeps the archive passes), `TestBatchCheckDCWV0039RefusesDriftedEvidence` (an unsealed sidecar on a batch fails before merge) | implemented; not accepted; reproduced before the fix: at `a2f0fef4f` the change was bound and checked and only `dogfood-seal` refused, naming no command |
| `DCW-V0-044` (accepted, decision 0520; V1-1209) | `internal/trace/store.go` (store under the worktree root), `internal/dogfoodflow/change.go`, `check.go` and `flow.go` (`--absolute-git-dir` evidence and lock), `internal/dogfoodflow/change.go` stranded-trace hint; `docs/DOGFOOD.md` step 5 and the batch steps; `TestDogfoodDCWV0044WorktreeLanesRunConcurrently` (built binary: two worktrees start each of change, bind, check and seal together and all pass with their own stores and clean worktrees, the primary worktree's store never written; lane B's work run in lane A's worktree refuses with the per-lane hint) | implemented; not accepted; the concurrent passes already held at `696dbdbf6`, where the shared-worktree refusal lacked the hint, so the defect was the undocumented one-worktree-per-lane rule |
| `DCW-V0-045` (accepted, decision 0520; V1-1207) | `internal/dogfoodflow/check.go` `requireCleanChange`; `TestCheckDCWV0045CleanProbeNeverRunsRepositoryFilters` (a configured clean filter on a re-stated tracked file never runs and the check refuses with the reason and `fix:` line; without it a clean change passes, an untracked file refuses `dirty-worktree`, a change outside `BASE..HEAD` refuses `uncommitted-change-not-in-base-target`, and an empty range passes `no-change-clean-worktree`) | implemented; not accepted; reproduced before the fix: at `36718cd0d` the same test's filter marker was written |
| `DCW-V0-046` (accepted under the owner's standing auto-accept (2026-10-11), decision 0523 P3; V1-1258) | `internal/dogfoodflow/change.go` `unknownRow`, `reasonedUnknownRow`, `prepareUnknownReasons`, `publishUnknownReasons` and `reasonedUnknowns`, `internal/dogfoodflow/check.go` `verifyBinding`, `checkSummary` and `unknownShare`, `internal/dogfoodflow/batch.go` `sealedVerification`; `TestChangeDCWV0046AdmitsReasonedUnknownRefusesUnreasoned` (a plan with one reasoned unknown row is PRODUCED with one cite, two marks and `--max-unknown 2`; an unnamed unknown hunk, a `no-evidence` reason, an unregistered reason and a note with no letter or digit each refuse with no cite or mark; a world-readable evidence file is replaced owner-only; a resumed map whose hunks an earlier staged plan marked owes them no row and keeps their notes; a stage whose citation fails leaves the earlier notes; notes that cannot be replaced after the cite report `unknown-reasons-not-recorded`, keep the earlier notes, and a replay of the same plan records the new ones; a map with no reasoned unknown removes the file; an occupied evidence path refuses `unknown-reasons-not-recorded`); `TestPrepareUnknownReasonsDCWV0046AppliesRowsInPlanOrder` (a later citation drops a hunk's note; the last row for a hunk decides); `TestPrepareUnknownReasonsDCWV0046RefusesOverflow` (notes at the bound are kept, over it refuse without truncation, and nothing is written before a cite); `TestPrepareUnknownReasonsDCWV0046SkipsANonRegularFile` (a FIFO at the path is not opened and is replaced by an owner-only file); `TestReasonedUnknownsDCWV0046CountsBootstrapOnceInTheAllowance`; `TestDogfoodReasonAdmitsCitationStageRefusals` (the new code is observable); `TestBatchCheckDCWV0046ReasonedUnknownAllowance` (reasoned unknowns only: ready-for-ci; one more unreasoned unknown: `constituent-cem-policy`); `TestDCWV0033CheckSummaryAbstainsOnUnobservedFields` (unknownShare cases); `TestDogfoodDCWV0046ReasonedUnknownChecksAndSeals` (built binary: a reasoned unknown change checks with `unknownShare=1/1 reasonedUnknown=1`, passes and seals with the reason in the sealed map, and the real `dogfood batch-check` then reports `reasonedUnknown=1 state=ready-for-ci`) | implemented; accepted |
| `DCW-V0-047` (accepted under the owner's standing auto-accept (2026-10-11); V1-1279) | `internal/dogfoodflow/check.go` `admitOutputChange` (also before the empty-range PASS in `requireCleanChange`), `requirementSpec`, `verifyStatus`, `verifyDeclared`, `verifyOCM`, `baseIndependence`, `checkSummary`, `record` and `seal`; `cmd/corvint/dogfood_flow.go` (`DOGFOOD_OUTPUT_CHANGE`); `TestCheckDCWV0047DeclaredOutputChange` (a tree verifier whose `cem status` stdout adds one member: undeclared fails `verifier-disagreement` with `outputsAgree: false`; declared by a requirement the change adds to a spec it modifies, the base verifier runs once and the tree verifier twice, the SUMMARY line carries `base-independence=NOT_PROVED` with both digests, the report records `outputsAgree: true` and the `baseIndependence` member, and the seal PASS line keeps the fields; a tree verifier that disagrees with itself fails `verifier-disagreement`; a requirement whose spec is not in the diff refuses `output-change-not-in-diff`, an absent or malformed ID `output-change-requirement-unknown`, with no verifier run; on an empty range an unknown ID refuses `output-change-requirement-unknown` and a known one `output-change-not-in-diff` while no declaration still passes `no-change-clean-worktree`; agreeing verifiers without a declaration print no new field); `TestCheckDCWV0047DeclaredOCMStatus` (V1-1276: a base verifier refusing the OCM read `unsupported-spec` fails `verifier-disagreement` undeclared; declared, the base runs once and the tree twice and its line is returned; a tree verifier that disagrees with itself still fails); `script/dogfood-seal.sh` with `script/dogfood-change_test.sh` (a stub check whose SUMMARY carries the fields: the wrapper streams the check output and its PASS line keeps the fields after `sealed=`) | implemented; accepted; a real output-changing lane NOT_OBSERVED |
| `DCW-V0-048` (accepted under the owner's standing auto-accept (2026-10-11); V1-1281) | `internal/dogfoodflow/chore.go` `ChoreRole`, `uniformCitation`; `internal/dogfoodflow/change.go` `citeStep`, `uniformPlan` and its `fix:` hint; `internal/observations/observations.go` `validDogfoodReason`; `TestChoreRoleDCWV0048ClassifiesOnlyCEMPaths` (exact CEM paths only; a quoted path, a nested or empty NAME, another suffix, any other path and an empty diff are not chores; seal-chore when any `.corvint/changes/` path is touched); `TestChangeDCWV0048RefusesUniformPlan` (a three-hunk one-span plan, by ordinal or full ID, is NOT_PRODUCED with no cite and the `fix:` line `hunks=3 span=a.txt:1:2`; distinct spans, another span of the same path, an unnamed hunk and a single hunk are cited; a bind-chore change is exempt); `TestDogfoodReasonAdmitsCitationPlanUniform` | implemented; accepted; the committed maps are not re-bound |
| `DCW-V0-049` (accepted under the owner's standing auto-accept (2026-10-11); V1-1281) | `internal/dogfoodflow/chore.go` `markChoreRole`, `markStableChoreMap` (called after `cem-prepare` in `run`); `internal/cem/workflow/stable_read.go` `StableCitationClass`; `tools/cem-citation-baseline`; `TestMarkChoreRoleDCWV0049WritesStableRoleOnly` (a stable map gains each chore role through the encoder, re-parses and keeps its mode, a second write leaves it byte-identical, a Core 0.x map is refused unchanged and no temporary file remains); `TestChangeDCWV0049MarksPreparedStableChoreMap` (a prepared stable map gets `bind-chore` only for a chore range and none for a change range); `TestCEMV1106StatusCitationClassGroupsChoreMaps`; `TestBaselineDCWV0049SplitsRowsByCitationClass` and `TestBaselineRowKeepsThePythonEncoding` | implemented; accepted; live since V1-1276 (`cem prepare` writes `cem/1.0` by default; `TestChangeDCWV0049MarksPreparedStableChoreMap` now also requires that the loop never refuses the stable map); the class-split baseline rerun is NOT_PRODUCED (no committed map carries a role yet) |
| `DCW-V0-050` (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) | `internal/dogfoodflow/migration.go` `cemMigrationCommits`, `migratedSeals`, `migrationRewrites`, `migratedConstituent`, `migrationsInRange`, `isMigrationCommit`; `internal/dogfoodflow/batch.go` `sealedConstituents`, `verifySealedCEM`, `verifyCoverage`; `internal/dogfoodflow/change.go` `admitSealedCEMs`; `TestBatchCheckDCWV0050MigrationCommitIsTheSealBaseline` (a range from before the migration passes with zero constituents, one commit and the NOTE, running no verifier; admission prints only the note; with the list empty the same range fails `sealed-cem-modified`; a range that adds the constituents before the migration fails `constituent-migrated count=2` with the split `fix:`; a later edit of a migrated map fails `sealed-cem-modified` in both commands); `TestBatchCheckDCWV0050RevertedMigrationFails` (a `-s ours` merge of the migration onto its parent fails `sealed-cem-migration-reverted` in both commands; removing the check fails the test); `TestBatchCheckDCWV0050FollowUpMigration` (a second listed migration commit is admitted with its own NOTE; a map it rewrote that the range adds fails `constituent-migrated`) | implemented; live `batch-check 75bc22605` at the migration commit PASS `constituents=0 commits=1`; a range over the landed merge NOT_OBSERVED until merge |
| `DCW-V0-024` | `internal/dogfoodflow/change.go` `declareNoIntent`, `internal/dogfoodflow/check.go` `verifyBinding`; `TestDogfoodDailyPathCompletesWithDeclaredNoIntent` (built binary, foreign repository: unset and empty intents refuse, a link plan refuses, the declared pass completes with the three rows and `NOT_ASSESSED` status, a swapped snapshot fails `dogfood-report-drift`, check prints the note, seal passes); the DCW-V0-024 case of `script/dogfood-change_test.sh` (through the wrapper: no OCM command runs, check prints the note); live run in a scratch repository with no spec recorded in the V1-0259 build-log entry | implemented; a real Beamfall change NOT_OBSERVED |

| Requirement | Focused evidence | Status |
|---|---|---|
| `DCW-V0-035` | `TestPlanRowLimits` in `internal/dogfoodflow/plan_limits_test.go`; `TestDogfoodPlanRowLimitObservationsPersist` in `internal/observations/observations_test.go`: 256/257 rows, distinct codes and fix counts | proposed |
| `DCW-V0-036` | `TestTracePathLimitDiagnostic` in `internal/trace/record_test.go`: typed current/stored lists at 200/201; `TestRecordTracePathLimitPublicCode`, `TestRecordStoredTracePathLimitPublicCode`, `TestDogfoodRecordNativeLegacyPathLimit`: public JSON code; `TestTypedStoredTracePathLimitCode`: stored schema-v2/v3 cause; `TestLocalOutcomeTracePathLimitKeepsAdmissionReason`, `TestAggregateLegacyAdmissionAcceptsPublicTracePathLimit`: internal fallback classification | proposed |
| `DCW-V0-037` | `TestDogfoodSingleCommitStagedCitationPlans` in `cmd/corvint/dogfood_staged_test.go`: real binary, 279 hunks, staged plans and final check | accepted, decision 0510 |

## Compatibility and rollback

Retain historical `/0` use-case admission and use explicit `/1` for the three added jobs. The
current alpha candidate assembler requires companions, and every candidate requires a
`PUB-V0-023` version token (decision 0384); the reader and installer admit a Core-only profile (`public-release-v0.md`, V1-0125), but
Core-only 0.6 packaging still needs an explicit compatible contract before release. This spec does not change that
wire by implication. If any gate fails, preserve its evidence and keep the affected jobs UNPROVEN.
Retain the previous working installed binary and public release. Do not rewrite historical receipts.
Roll back `DCW-V0-018` by unsetting `DOGFOOD_OCM_LINKS`: no link runs and every requirement
returns to `unassessed` on the next pass.
Roll back `DCW-V0-024` by reverting its change: the declaration then refuses
`missing-intent-scope` as any other invalid manifest did, and a report written under it fails
`dogfood check`, so no no-intent change can pass silently on either side.
Roll back `DCW-V0-025` by reverting its change: the two codes then block `"complete": true` again,
and a report written under it fails `dogfood check` with `context-abstention-evidence-drift`.
Roll back `DCW-V0-032` by reverting its change: an empty plan is again a zero-citation no-op on any map,
the three inputs are read whole, and the receipt notes return to the `DCW-V0-031` codes; no report,
map or receipt format changes, so evidence from either side stays readable.
Roll back `DCW-V0-033` by reverting its change, or per run with `DOGFOOD_VERBOSE=1`: check stdout
returns to the status JSON lines; the detail file is then stale and unused, and no report, map or
capture format changes.
Roll back `DCW-V0-042` by reverting its change: a range that adds any sealed CEM refuses
`sealed-cem-in-change` again; a CEM bound under it stays verifiable, since it is an ordinary map
of the whole range.
Roll back `DCW-V0-043` by reverting its change: `dogfood change` again binds over an unarchived
`BASE` CEM and only `dogfood seal` refuses, and batch-check again counts the inherited archive as
an unverifiable constituent; no report, map or sealed CEM format changes.
Roll back `DCW-V0-044` by reverting its change: the stranded-trace line loses its worktree
clause and the docs again say only "own clone"; no store, report or map format changes.
Roll back `DCW-V0-045` by reverting its change: the check again reads `git status` and runs any
configured filter; no report or map format changes.
Roll back `DCW-V0-046` by reverting its change: an unknown row is again an invalid citation
(`cem cite` refuses `unknown` as an evidence path) and every non-bootstrap unknown hunk fails
`cem-status`, the SUMMARY and CONSTITUENT lines lose their added fields, and the evidence file is
unused. A map or sealed CEM that records a reasoned unknown stays a valid CEM but no longer
passes `dogfood check` or `batch-check`; no report or map format changes.
Roll back `DCW-V0-047` by reverting its change: `DOGFOOD_OUTPUT_CHANGE` is ignored and an
output-changing lane fails `verifier-disagreement` again, so it returns to gating its output behind
an opt-in flag. A report or seal line written under it keeps its extra member or fields, which no
reader requires; no map or sealed CEM format changes.
Roll back `DCW-V0-050` with the V1-1277 migration by reverting its merge commit: the sealed maps
return to `cem/0.2` and the list disappears. Reverting only the list keeps the migrated maps,
and every range that spans the migration commit then fails `sealed-cem-modified` closed.
Roll back `DCW-V0-048` and `DCW-V0-049` by reverting their change: a uniform plan is cited
again, a prepared stable chore map keeps no role, `cem status --citation` loses `citationClass`
and the baseline tool disappears. No committed map carries a role before V1-1276, so nothing
needs migrating; a later stable map with a role stays valid under `CEM-V1-095`.
Roll back `DCW-V0-039..041` by reverting their change: `batch-check`, its wrapper and make
target disappear and the `fix:` lines return to their former text; no report, map, receipt or
sealed CEM format changes, and batch-check writes nothing, so no evidence needs migrating.
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

## Proposed aggregate outcome extension

[Aggregate Local Outcome V0](aggregate-local-outcome-v0.md) owns the experimental, explicit
aggregate selector, nonlearning receipt, `/1` report and additive pending state discriminator.
Its source candidate is **NOT_QUALIFIED**. The default `/0` workflow and 200-path learning limit
remain governed by this existing contract. Aggregate acceptance requires the extension's native
qualification, independent review and frozen terminal gates; a source patch or focused test pass
does not satisfy those requirements. The held recovery entrypoint is separately pending.
