# pk-dogfood: evidence workflow, task closeout and context repairs

Owner intent: deliver the nineteen tickets from the supplied authoritative exports in the
pk-dogfood lane, based on `6c33df33bc5670131482f5263b410cb980580657`. Requirements use the assigned
7400–7499 block, with cross-branch `git log --all -S ID` collision checks. New clauses carry the
owner's standing acceptance, not an owner read-back. Task-store writes, publication and sealing
are excluded by the lane charter. Independent review belongs to the orchestrator.

## Decisions and contract changes

- `CEM-V1-7400` (V1-3863): verify accepts an absolute in-worktree map through the checked parent-directory
  reader. A lexical-only normalization was replaced after an alias counterexample failed:
  parent identity, held-directory reads and metadata aliases must retain the old safeguards. Verify-stable anchors relative maps to its explicit repository.
  Git metadata, final symlinks, bounds and existing out-of-tree checks keep their refusals.
- `DCW-V0-7401` (V1-3977): fail a receipt-less no-diff change start with a named refusal instead
  of silently succeeding with instructions. Existing TASK/PATHS recording is reused. Supply a
  discoverable authoring template in the existing spec-development guide, facets decoder route
  and help-tree regeneration hint. A standalone template required a documentation declaration
  edit that itself became unowned and raised 79 language frontiers. Embedding the template in
  the existing route removes those new inputs; no planner refusal or gate is suppressed.
  Bare single-caret and quoted expressions share typed argv; diagnostics retain the line text.
  Citation drafting emits only admissible supplied VERIFY commands, separately from the TSV.
  Exact unstaged-spec `git add` guidance and single-read journal-aware readiness already existed.
- `DCW-V0-7402` (V1-4569): the suspected disagreement is a population and witness mismatch,
  not an arithmetic defect. A test uses 0/115 OCM links and 12/12 change traces. Explain both
  populations; do not invent explicit OCM links from test/table traceability.
- `DCW-V0-7403` (V1-4169): choose an early named refusal for typed argv when exact path
  admission requires the string-only aggregate receipt. Do not silently revise its closed wire format or
  claim that a quoted test regex can bind through that receipt. Inputs-check explains the
  actual incompatibility before prepare, with a plain-command remedy. An end-to-end preflight
  caught an overbroad first check: wrappers enable aggregate fallback even for small changes.
  A read-only shared recorder admission now checks the actual 200-source-path boundary;
  enabling fallback alone is compatible with typed argv. Tests cover 200/201 admitted paths,
  201 filtered non-source paths and zero trace-store/append-recovery entry calls.
- `TCL-V0-7404` (V1-4288, V1-4289): explicit seal overrides take precedence over a delivering
  lane PR's exported head. Validate containment as before. Reuse the claim retry admission
  function for restored widen paths, and pass admitted keys to the actual claim. Bind step
  request IDs to base, candidate, scope source and scope keys, preserving the readable ticket
  and attempt prefix. Multi-lane and submitted-widen regressions exercise the real CLI.
  Retry scope comes only from the latest generation, excluding declared paths; the 256-key
  explicit request limit does not replace the larger native declared-resource limit. A
  submitted-widen regression includes a declaration of 257 resources (64 PATH resources plus 193 touch paths).
- `TCL-V0-7405` (V1-3536, V1-3539, V1-3598): remove dangling local guide references from help;
  required manual-completion flags have a shared invocation in help and missing-flag errors;
  unbounded-effect warnings report effective coverage, including UNKNOWN and INCOMPLETE.
- `TCP-V0-7406` (V1-3546): explicit task evidence reuses governance's pinned one-hop symlink
  admission. Bind both link and target identities; retain the same typed refusal for unsafe
  links. No working-tree symlink is followed and no ranking mechanism changes.
- `MCPV0-7407` (V1-3916): the two public task-review constructors return ordinary errors,
  with plain nil success. Explicit adapters preserve the transport's typed error boundary;
  the context worker uses separate constructor/call errors to avoid reintroducing typed nil.
- `TCQ-V0-7408` (V1-1451): imported selectors do not carry same-package declaration closure;
  local shadows still can. The first narrowing lost a direct-importer guard; the existing
  TCQ-V0-761 regression caught it. Qualified references are retained separately for importer
  search. A second regression confirms that importer edges need the exact import path, not
  just the exported selector spelling; otherwise unrelated packages reintroduce the broad
  closure. A third failing regression confirms field names were also conflated with package
  declarations. Field declarations and receiver selectors now stay separate; selectors retain
  the old conservative edges when a package method has that spelling, and importer receiver
  selectors stay conservative. This is syntax-based candidate naming, not full Go dataflow or
  whole-suite adequacy. External test packages remain conservative because import paths
  alone do not prove the package identity. The new regression failed before this fix, naming
  four candidates instead of the two real package references. Timing promotion remains
  contingent on the recorded-pair measurements below.

Rollback is to revert the associated logical change and its new clause together; no task store,
receipt profile, authority ordering or ranking weights migrate. Existing CEMs and receipts remain
readable. Request IDs intentionally differ when candidate or scope differs.

## Existing deliveries confirmed at the supplied base

- V1-3922: `ASD-V0-840`, `TestDCWV0882InputsCheckRefusesUnsortedIntentsFirst`, including swapped
  complete lines, first out-of-order pair and the exact `LC_ALL=C sort -u -o` recovery.
- V1-3296 audit: `readCanonicalPatch` is the only dogfoodflow reader of a patch consumed by CEM.
  It uses `patch.MaxPatchBytes` and `git-output-exceeded`; `TestProposeCitationsDCWV0802NamesWhyThePatchIsUnavailable`
  covers the oversized patch. The four default-limit sites are `flow.go` (boundedGit metadata/name lists), `propose.go`
  (gitContents, checked again at the 8 MiB patch boundary), `check_anchor_grep.go` (grep results)
  and `propose_blobs.go` (individual BASE blobs). The changed-line coverage diff is also not
  a CEM input patch; no other patch reader uses the 16 MiB limit as its admission ceiling.
- V1-2522: `CEM-PILOT-033`, `publish.LockRefusal`, the non-advisory-platform early refusal and
  testplatform skip matching already exist. Root cause: the earlier wrapper discarded the
  platform-lock refusal. Native Windows rerun remains blocked: invoking PowerShell from this
  sandbox fails `UtilBindVsockAnyPort: socket failed 1`; this is not Windows execution evidence.
- V1-3533: `TestTMV0008_AS08_ExportOverLimitIsAnError` confirms an ERROR, nonzero exit,
  LIMIT_EXCEEDED and the maximum limit instead of an empty successful page.
- V1-3544: the suspected silence is the explicit AHI-046 governance-only rule. The resolution
  receipt names the missing actionable anchor; native prompt lifecycle qualification exercises
  the Claude-code route independently of a Codex SessionStart. No change to packet triggering.
- V1-3988: strict wire framing already refuses missing LF with `missing the single trailing LF`.
  `TestWireMissingNewlineDiagnostic` now checks both accepted LF and the exact diagnostic.
- V1-4004: `Project` already decodes/validates the body before Patch/Blobs calls. The malformed-body
  regression checks seven tamper shapes and asserts zero source calls; no side effect precedes it.

## Verification and limits

All selected Go tests run through `script/heavy.sh`, one package per command, after Corvint affected
selection against the fixed base. Focused package logs and exact commands are retained in the lane
handback. The new path test caught a metadata admission error during implementation; the final
negative and positive path tests pass. The submitted-widen test passes through precheck and actual
gated completion. New and existing diagnostics, symlink refusals, bridge interfaces, native host
projection and malformed-body tests are selected; repository-wide tests and gate are NOT_RUN by
charter. The known BASE failure in `script/batch-precheck_test.sh` is left untouched.

The first 120-second-per-run guarding measurement was cancelled before producing a complete row:
it did not match the normal producer's 300-second per-run ceiling. Corrected measurement uses
`DefaultHunkRevertLimits` (300 seconds per run, 1200 seconds total), with three repetitions of each
recorded pair. No timing improvement or guard-preservation claim follows from that cancelled run.
The first after-source trial completed PR #934 at a median 929.480 seconds against BASE's
1013.055 seconds, with the same guard, but was interrupted after the namespace regression
was confirmed. An exact-import namespace trial then retained one PR #934 run at 989.142 seconds, but was
stopped for the field-namespace correction before completing its repetitions. Only the
final field-aware source is used for promotion; earlier numbers remain trial evidence.

Self-development routes used: change-start context/query/impact receipts, affected test selection,
CEM binding and reviewer-report inspection. Mutation, ranking learning and unrelated context
experiments are excluded; no ranking or external-outcome benefit is claimed. Frozen ranking and
external-repository gates are NOT_RUN because this slice does not promote those mechanisms.

The OCM finding message changes canonical aggregate output. Binding/checking therefore declares
`DOGFOOD_OUTPUT_CHANGE=DCW-V0-7402`; the checker retains `base-independence=NOT_PROVED` rather than
claiming old/new canonical output agreement. This is the explicit output-change route, not an
independent verifier result.

CEM binding/checking follows the committed measurement evidence and stops at a passing check;
the handback retains the actual receipt. Sealing and independent review are reserved to the
orchestrator. Native ticket completion writes are explicitly forbidden in this
lane, so tickets remain open in the authoritative store regardless of implementation disposition.


## Qualification design fork

The large command/CEM and Tasks merge controls remain at their time ceilings. A roughly
one-second median shift at a 1200-second ceiling is not treated as meaningful speedup.
One command/CEM after repetition at load 44.04 also misses the old guards under timeout and
total-limit; their candidate identities remain present and another repetition observes them.
The stricter every-repetition diagnostic is retained as failed, so repeatability stays
NOT_ESTABLISHED. Median-run and aggregate observed identities are reported separately.

Keep both large controls and every raw row. Add two recorded, bounded changes (the VERIFY
anchor grammar change `25164d1ac` and canonical wire string-run optimization `3fb719f93`)
to distinguish candidate narrowing from broad dispatcher/fixture reach. These are additional
qualification observations, not replacements for the limiting controls. No speedup is
promoted merely from smaller candidate lists or the censored large-control median.

The VERIFY-anchor pair completed with a 7.94% median reduction (139.679 to 128.593
seconds). The wire pair is a flat control: 10.413 to 10.693 seconds despite 108 to 99
candidate edges. Retain all six wire rows; smaller candidate counts alone are not timing
qualification. Add the recorded documented-VERIFY-forms change `ed4c714ee`, whose
quoted-run diagnostics exercise the same dogfood package namespace, as another bounded
observation. This selection follows the failed wire qualification and is recorded explicitly;
no wire row is removed and no algorithm changes between these observations.
