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
lane. Native disposition remains with the orchestrator; this lane writes no completion evidence.


## Qualification design fork

The large command/CEM and Tasks merge controls remain limited by timeouts or the total budget. A roughly
one-second median shift at a 1200-second ceiling is not treated as meaningful speedup.
One command/CEM after repetition at load 44.04 also misses the old guards under timeout and
total-limit; their candidate identities remain present and another repetition observes them.
The stricter every-repetition diagnostic is retained as failed, so repeatability stays
NOT_ESTABLISHED. Median-run and aggregate observed identities are reported separately.

Keep both large controls and every raw row. Add two recorded, bounded changes (the VERIFY
anchor grammar change `25164d1ac` and canonical wire string-run optimization `3fb719f93`)
to distinguish candidate narrowing from broad dispatcher/fixture reach. These are additional
qualification observations, not replacements for the limiting controls. No speedup is
promoted merely from smaller candidate lists or a large control whose median is limited by the budget.

The VERIFY-anchor pair completed with a 7.94% median reduction (139.679 to 128.593
seconds). The wire pair is a flat control: 10.413 to 10.693 seconds despite 108 to 99
candidate edges. Retain all six wire rows; smaller candidate counts alone are not timing
qualification. Add the recorded documented-VERIFY-forms change `ed4c714ee`, whose
quoted-run diagnostics exercise the same dogfood package namespace, as another bounded
observation. This selection follows the failed wire qualification and is recorded explicitly;
no wire row is removed and no algorithm changes between these observations.

The documented-VERIFY-forms pair retains its old guard in all repetitions, but medians
238.789 to 236.842 seconds (0.82%) miss the preselected 1% measurement threshold. Treat
it as another control, not a qualifying improvement; keep the 320/322-second repetitions.
Add the separate recorded quoting-diagnostic change `eb08313ca`, a single existing-function
edit with an existing guard. Its intent is to test candidate-selection savings without
adding a new helper or a broader change entry point. The three-repeat, 300/1200-second
limits and 1% threshold stay fixed. All seven observed targets remain in the final record.

## Recorded-pair guarding qualification

`TCQ-V0-7408` / V1-1451: the three qualifying recorded-pair medians decreased by at least 1%, and every
before `(hunkId, guard)` result of those qualifying pairs is retained in each after run.
Each guard’s test remains a candidate in every after run. The four controls for broad
dispatch, wire parsing and documented forms remain in the tables, including their limiting observations. Every-repetition repeatability is a
separate diagnostic: timeout/total-limit rows remain unobserved, never converted to passes.
This measures the actual guarding producer step (export, naming and revert/test work),
excluding the preceding Stable CLI validation. It uses normal limits of 300 seconds per
Go invocation and 1200 seconds total, with the same driver for each before/after pair and immutable historical revisions.
The historical Git objects must be available locally to rerun the driver.

This is a shared-host observation, not an isolated causal benchmark. Host loads differ and
cache warmth is NOT_OBSERVED. Candidate counts are repeated hunk/test edges, not unique tests
or execution counts. Timeouts and other NOT_RUN reasons remain in the raw reports; no
whole-suite adequacy, external outcome or ranking benefit is claimed.

| Role / target | Before median (s) | After median (s) | Reduction | Before / after candidate edges | Before guards retained in median after run |
|---|---:|---:|---:|---:|---:|
| qualification / `95622955d` | 1013.055 | 885.065 | 12.63% | 4485 / 4361 | 1 / 1 |
| limiting-control / `3465dd98c` | 1204.315 | 1203.285 | 0.09% | 4626 / 4470 | 5 / 5 |
| limiting-control / `6e0ce750c` | 607.650 | 611.731 | -0.67% | 3918 / 3750 | 0 / 0 |
| qualification / `25164d1ac` | 139.679 | 128.593 | 7.94% | 396 / 314 | 0 / 0 |
| limiting-control / `3fb719f93` | 10.413 | 10.693 | -2.69% | 108 / 99 | 0 / 0 |
| limiting-control / `ed4c714ee` | 238.789 | 236.842 | 0.82% | 508 / 395 | 1 / 1 |
| qualification / `eb08313ca` | 138.925 | 122.489 | 11.83% | 202 / 166 | 2 / 2 |

All repetitions, with Linux load averages (1 / 5 / 15 minutes) sampled at step start:

| Target / run | Before seconds | Before load | After seconds | After load | Before G/U/N | After G/U/N |
|---|---:|---|---:|---|---|---|
| `95622955d` / 0 | 1115.168 | 60.50 / 57.52 / 40.23 | 885.065 | 6.19 / 11.18 / 19.93 | 1/8/33 | 1/8/33 |
| `95622955d` / 1 | 1013.055 | 62.03 / 60.42 / 55.41 | 885.739 | 5.01 / 7.64 / 12.85 | 1/8/33 | 1/8/33 |
| `95622955d` / 2 | 932.995 | 20.36 / 25.68 / 36.78 | 843.546 | 6.06 / 8.40 / 10.81 | 1/8/33 | 1/8/33 |
| `3465dd98c` / 0 | 1204.315 | 10.10 / 11.82 / 21.74 | 1200.711 | 4.95 / 6.76 / 8.60 | 2/5/27 | 2/6/26 |
| `3465dd98c` / 1 | 1204.651 | 32.04 / 21.15 / 21.59 | 1239.049 | 44.04 / 20.13 / 13.85 | 2/5/27 | 0/1/33 |
| `3465dd98c` / 2 | 1203.804 | 26.64 / 29.60 / 30.73 | 1203.285 | 46.82 / 54.01 / 44.04 | 2/5/27 | 2/5/27 |
| `6e0ce750c` / 0 | 607.503 | 7.72 / 12.70 / 21.16 | 606.979 | 5.11 / 12.42 / 25.35 | 0/0/8 | 0/0/8 |
| `6e0ce750c` / 1 | 609.832 | 8.71 / 9.09 / 15.31 | 611.731 | 11.93 / 9.99 / 17.45 | 0/0/8 | 0/0/8 |
| `6e0ce750c` / 2 | 607.650 | 16.91 / 10.99 / 12.84 | 615.568 | 36.19 / 24.45 / 19.69 | 0/0/8 | 0/0/8 |
| `25164d1ac` / 0 | 135.820 | 8.09 / 9.02 / 20.51 | 123.833 | 6.16 / 8.69 / 20.65 | 0/2/1 | 0/2/1 |
| `25164d1ac` / 1 | 140.419 | 7.99 / 9.01 / 18.98 | 128.593 | 8.27 / 9.17 / 19.41 | 0/2/1 | 0/2/1 |
| `25164d1ac` / 2 | 139.679 | 9.64 / 9.38 / 17.72 | 132.957 | 9.62 / 9.36 / 18.13 | 0/2/1 | 0/2/1 |
| `3fb719f93` / 0 | 10.643 | 11.20 / 10.29 / 16.90 | 10.603 | 12.26 / 10.29 / 17.27 | 0/2/2 | 0/2/2 |
| `3fb719f93` / 1 | 10.413 | 10.91 / 10.26 / 16.82 | 10.710 | 11.38 / 10.17 / 17.15 | 0/2/2 | 0/2/2 |
| `3fb719f93` / 2 | 10.031 | 10.63 / 10.22 / 16.73 | 10.693 | 10.87 / 10.10 / 17.06 | 0/2/2 | 0/2/2 |
| `ed4c714ee` / 0 | 227.322 | 14.69 / 10.68 / 15.71 | 199.651 | 12.62 / 10.09 / 15.61 | 1/1/0 | 1/1/0 |
| `ed4c714ee` / 1 | 320.197 | 33.88 / 17.26 / 16.95 | 322.116 | 18.83 / 13.19 / 15.71 | 1/1/0 | 1/1/0 |
| `ed4c714ee` / 2 | 238.789 | 26.00 / 26.42 / 21.51 | 236.842 | 30.26 / 26.93 / 21.44 | 1/1/0 | 1/1/0 |
| `eb08313ca` / 0 | 178.630 | 39.69 / 32.28 / 25.04 | 122.489 | 32.39 / 24.04 / 21.64 | 1/0/0 | 1/0/0 |
| `eb08313ca` / 1 | 88.945 | 81.59 / 59.61 / 36.96 | 170.852 | 33.07 / 31.33 / 24.96 | 1/0/0 | 1/0/0 |
| `eb08313ca` / 2 | 138.925 | 29.64 / 48.15 / 35.00 | 81.524 | 60.82 / 56.54 / 36.55 | 1/0/0 | 1/0/0 |

Immutable pair and guard identities:

- `85ba6321a811146f5e150ef10852c5f2963aa476` → `95622955d2dc0933c32f8bca08b6b255920f9d3a`. Guards: `TestSelectPackagesHonorsDeclaredReadScopes` on `hunk:sha256:ad27ae7457b4ee595ee889dcd72fe882dc22d2d56d964a340812bc37accf64b6`; no before guard lost in the median after run.
- `103a8b580b507798552182b65a9d669169515105` → `3465dd98cd1c8ed1ab4410616b367906af139ce9`. Guards: `TestCAPV0019CommittedCatalogMatchesTheBinary` on `hunk:sha256:03542cbc7cd86d5203024d1cb6fbe68834232688661584a12b7085b5ea1b7db3`, `TestCAPV0045CatalogLinksCurrentTreeSnapshots` on `hunk:sha256:03542cbc7cd86d5203024d1cb6fbe68834232688661584a12b7085b5ea1b7db3`, `TestMCPV0100HelpToolMatchesTheCLI` on `hunk:sha256:03542cbc7cd86d5203024d1cb6fbe68834232688661584a12b7085b5ea1b7db3`, `TestMCPV0105CompanionHelpMatchesTheCLI` on `hunk:sha256:03542cbc7cd86d5203024d1cb6fbe68834232688661584a12b7085b5ea1b7db3`, `TestMetadataOnlyChangeRefusalVectors` on `hunk:sha256:e31ed49194a161b2b9920328b05553ccd5f9e0efc856fe109f0cf1f31575de10`; no before guard lost in the median after run.
- `783855b48fcde1a7f7c1472806eaea67ebd3e6c4` → `6e0ce750ccff967960f3a85d6667e4a8cab766a0`. Guards: none observed before; no before guard lost in the median after run.
- `671c346bdfa2418740602aa81bcad6864e7d4367` → `25164d1ac044f6184510c289c58272c02a3aabe4`. Guards: none observed before; no before guard lost in the median after run.
- `35475f860d97bfde9bbaa6cccf1f2ba8f78ea44b` → `3fb719f93a5972a39744ee98d80c11091bd72e9a`. Guards: none observed before; no before guard lost in the median after run.
- `fa10dffe6a3462fd8efdf7160f8e7f50f46e83c7` → `ed4c714ee62c089288ee53c1170fc87739b78042`. Guards: `TestDCWV0817RecordIntentHintNamesAcceptedForms` on `hunk:sha256:f385281f1bc1c2b229403c0f805142215ad6e2545b4b4d4777e420212f43ff02`; no before guard lost in the median after run.
- `ec411c9675f4a1aa36f0469ab80392e3eba224c3` → `eb08313caac553d8bd0eb62d47d771acfc08846f`. Guards: `TestDCWV0403UnquotedRunAlternationRefusesInOneLine` on `hunk:sha256:35f528291fc832169c862a35f56f8829ff80fac5a1b776db86ce317911626c68`, `TestFinishLaneDCWV0092RefusesVerifySyntaxBeforePrepare` on `hunk:sha256:35f528291fc832169c862a35f56f8829ff80fac5a1b776db86ce317911626c68`; no before guard lost in the median after run.

Algorithm and execution bindings (build stamps were dirty during the controlled builds; exact source blobs and executable bytes are retained separately):

- before: algorithm revision `6c33df33bc5670131482f5263b410cb980580657`; executable SHA-256 `0612f37f7133927365d2fd5c59b2dad6bd3734de382a404c7436b9b44bad05d2`.
  `internal/cem/verify/guards.go` Git blob `9999c1faa062d55cfa34a62bdb5cb5f156a38644`.
  `internal/cem/verify/guard_importers.go` Git blob `45ed5a9dbc5edf23b9ff67f4bcad2cbbfeec96ea`.
- after: algorithm revision `58293582e47eaee1a90b39b89b381668aef9838e`; executable SHA-256 `e34ec5155148c0d1be4d10e393fe8a170585cac3cf105ce16fb70527e915bfc1`.
  `internal/cem/verify/guards.go` Git blob `0b9ed807afa3fee28d9152524ee397d33875731e`.
  `internal/cem/verify/guard_importers.go` Git blob `3ecfce1ee4c167b80894aa10248172ed02e720db`.
- Driver Git blob `b6049bb0a6f864af8387dbbf2092db522ef27461`; additional-pair driver Git blob `929b76339344db741fbdd1179ba34a500fe1d613` (`internal/cem/verify/pk_guard_pairs_test.go`).
- Additional before executable SHA-256 `16e9d103de166d433676bd50116a85aab5e00fa63da49c8f574aef2ddf1650ba`; same algorithm blobs.
- Additional after executable SHA-256 `541e5ce34b7e29951b5666b4368df953c6a23e83adbde0088ae91b1c8178ff8d`; same algorithm blobs.
- Third-pair driver Git blob `cf4c40cb73139c80de05445dcc2737843b29e893` at `62e0fa084ebd89077a4c66ae182beef23cb706c6`.
- Third-pair before executable SHA-256 `1053e82c47eaffc9ffeefa020a2fb1c9ebb2fc29ac8efd0ec40b69bb159d6c3a`; same algorithm blobs.
- Third-pair after executable SHA-256 `b82cfe1bea8c0fc3e50ee184ba839d5147f1afed3482667ae6d6817e75e9236f`; same algorithm blobs.
- Fourth-pair driver Git blob `85be83f149dfc3015e4cba697feb6f5ef77ae0bb` at `696c3cf7a104763f7dfa2d6aecf1966fafc86c58`.
- Fourth-pair before executable SHA-256 `2db3d4963bd83871a32401a707d15a524794cec23f93cea40f4c4d95311bf0c2`; same algorithm blobs.
- Fourth-pair after executable SHA-256 `c9e6a65c8aeb751a83f40bf8c924459032f63e93df5f4d20f127fa96f9ff848b`; same algorithm blobs.
- Complete before log `guard-default-before.log` SHA-256 `a3e564b90f1e236faeffee1d6eab2fdb7a2ef439e9ba3a6106441f24e797cfb5`; retained with the lane handback evidence.
- Complete before log `guard-additional-before.log` SHA-256 `584056504fda77527e86824b41c83a9279af73a632aa95645e9f0e38d17f9ac5`; retained with the lane handback evidence.
- Complete before log `guard-third-before.log` SHA-256 `ed40d3088f00af898a7e7d469c2cf2305957534d3c5a4cae41c72dea4a5b516a`; retained with the lane handback evidence.
- Complete before log `guard-fourth-before.log` SHA-256 `b9388db23d0103a7e2ec91b3689bfbfb27801e9001124c0b97bdb00ac1ac2d4b`; retained with the lane handback evidence.
- Complete after log `guard-final-after.log` SHA-256 `55b0709e57fc84795a72283967f24951be079475ac0905ed82c7840d57083a40`; retained with the lane handback evidence.
- Complete after log `guard-additional-after.log` SHA-256 `55664dea9d93d4d57b08c3c64f042e422509f0e010fdde2a59405b81060dfd5c`; retained with the lane handback evidence.
- Complete after log `guard-third-after.log` SHA-256 `bf70586110e7ab2ccc7e5d55f3a697ad33f8ee93e7b904eef3fb1de261a2fda4`; retained with the lane handback evidence.
- Complete after log `guard-fourth-after.log` SHA-256 `76ddb5508fd1469f78184f2f6fe1afa40bf4bddb314b20f4926e38bb2269e507`; retained with the lane handback evidence.

Retained method and result integrity bindings:

- `compare-guards.py` SHA-256 `5edf40f084b1e67c85878cc9729032242f4d8b0e376d527b8836dbf7edd867d6`; exact method, results or source binding, retained with the handback.
- `guard-comparison.json` SHA-256 `2d8d5f4e488cb1a0ffba2fb479a885881d408ede7fbd134076ff76f7c6a0f9b9`; exact method, results or source binding, retained with the handback.
- `compare-guards-every-run.py` SHA-256 `769d8dd11f0a4161c845d775198c5b45aa8487645a35300b6b38e4f2fd992f77`; exact method, results or source binding, retained with the handback.
- `guard-every-run-comparison.json` SHA-256 `1f54501c0a452c838f3b04c2a628d747159f108e39d2df4639ebf457a3506752`; exact method, results or source binding, retained with the handback.
- `guard-source-manifest.json` SHA-256 `3c18a5b33a1af7e54cbacad1c8712c7a0df9929dbb5420814fdcfa0ab7055218`; exact method, results or source binding, retained with the handback.
- `record-performance.py` SHA-256 `3a7e6b462e7c21ef748b1f1e585e95b1dd690d78c03f6d760d07e5295298b592`; exact method, results or source binding, retained with the handback.

The original three-pair diagnostic required every median to fall and every before guard
to appear in every after repetition. It fails: at least one after repetition misses earlier
observations under timeout/total-limit. All candidate names remain present, and the median
run plus aggregate observed set retain the earlier guards. The requested median comparison
is reported separately from repeatability, which is NOT_ESTABLISHED. The Tasks control also
rises from 607.650 to 611.731 seconds. The complete failed diagnostic is retained, not
discarded or relabelled passed.

This lane uses the charter's `make dogfood-change` / `make dogfood-check` finish path.
Keyed `begin` / `verify` / report-set acknowledgment receipts are NOT_PRODUCED: no keyed
plan was frozen at change start, and actual foreground selected-check logs are not
reconstructed as keyed observations. Independent review remains with the orchestrator.

The lane handback retains the complete raw reports, comparison program/JSON, exact commands,
build metadata, earlier cancelled trials and every NOT_PRODUCED reason. The final CEM/check
receipt follows this committed measurement record; no lane seal is performed.


## Native final-check publication blocker

The first final check at `37db224dfc6f1853451db4ebe0ff735bd8e84ee3` failed
`dogfood-report-write` despite writable report paths. The checker staged under the
worktree’s private Git directory, then renamed into the worktree. A bounded probe
confirms `EXDEV` (errno 18): these writable sandbox roots are different mounts.
The generic permissions recovery hid the actual publication error. The original report
was intact; the failed check and probe remain retained.

`DCW-V0-7409` reuses the existing owner-only same-directory atomic publisher instead
of assuming the Git and worktree mounts are the same. Its temporary `.claims-*.tmp`
family is ignored and documented; normal failures remove it and preserve the old
artifact. The check retains its existing failure code and stderr text under the finish charter’s freeze.
A portable regression makes Git staging unavailable and requires successful publication;
a failure case verifies preserved destination contents and temporary-file cleanup.
The final native check qualifies the actual split mounts. No verifier gate is weakened.
The guarding algorithm blobs are unchanged; the recorded-pair measurements remain bound
to those exact blobs, and are not rerun for this report-publication repair.


## Finish-only disposition

The finish round keeps the already-started destination-mount publication repair and
its preservation/cleanup regressions. It drops the unfinished extra stderr cause line
because this lane freezes diagnostic text. No new feature or guarding measurement is
started. Existing committed pair measurements remain the qualification evidence;
whole-package tests and a bounded CEM bind/check finish this round. Independent review
and native ticket completion remain with the orchestrator. Native Windows execution
remains externally blocked as recorded above.
