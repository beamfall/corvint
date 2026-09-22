# CEM pilot kit

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: accepted
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/CHANGE-EVIDENCE-MAP.md`, independent product,
onboarding, security, and wire-contract reviews recorded in `docs/BUILD-LOG.md`

## Agent digest
- Claim: The CEM pilot kit freezes a safe first-run, reviewer-report, and outcome-trial contract for experimental CEM 0.1 use.
- Status: accepted/experimental
- Exists: `internal/cem`, reviewer reporting, `experiments/cem-first-run`, and frozen contracts.
- Blocked on: independent external outcome evidence.
- Read next: `cem-external-interop-v0.md` and `cem-0.2-canonical-binding.md`.

This spec freezes the integration contract for work that began as an experimental prototype. It
does not claim the implementation was originally spec-first. No part of the slice may be promoted
from experimental until it conforms to this contract.

## User and job

An agent or reviewer entering an existing Git repository must be able to create or resume a CEM for
an exact committed change, resolve every hunk without copying opaque identifiers, see readiness and
drift, produce a safe reviewer summary, and run a frozen value trial. Median first use must fit
within 15 minutes.

## Verified starting state

- `context_corvint_cem.py` implements strict `cem/0.1` parsing and verification.
- `corvint cem begin|cite|mark|verify` implements the original low-level workflow.
- `conformance/cem-0.1` is a Corvint-owned conformance seed, not independent interoperability.
- An offensive review found six verifier defects; repairs and focused regressions passed review.
- No external producer or consumer has passed the contract and no real 30×30 outcome trial has run.

## Requirements

### First-run workflow

- `CEM-PILOT-001`: `cem prepare --base REV --target REV` MUST derive the exact committed patch with
  the documented CI diff profile, exclude only the map path, create private atomic outputs, and
  return an ordered hunk worklist.
- `CEM-PILOT-002`: prepare MUST resume a valid map for identical base and patch bytes, refuse an
  outdated or invalid map by default, and replace it only with explicit `--replace`. Only a
  file-system not-exist result at the map path makes the map absent; an existing map that cannot be
  read (oversize or non-regular) is refused the same way, whatever text its read error carries.
- `CEM-PILOT-003`: `cem cite` and `cem mark` MUST accept either the full hunk ID or its canonical
  one-based worklist ordinal. Non-canonical (`01`, `+1`) or out-of-range decimal selectors MUST
  fail. `cite --bytes START:END` MUST refuse a negative START as `invalid-span` for every caller,
  not only the CLI parser.
- `CEM-PILOT-004`: `cem status` MUST report `ready-for-ci`, `incomplete`, or `invalid`, every hunk,
  disposition counts, the next action, verifier results, and optional unknown/mechanical policy
  failures. Unknowns are not green when a configured policy rejects them.
- `CEM-PILOT-005`: `cem verify` MUST apply the same optional policy caps as status, accept an exact
  independently supplied expected base, and retain the existing permissive cap behavior when caps
  are omitted.

### Reviewer report and CI

- `CEM-PILOT-006`: the reviewer model MUST render every parsed patch hunk exactly once with ordinal,
  path, ranges, disposition, reason, basis selectors, and drift status, including unmapped hunks
  from invalid maps.
- `CEM-PILOT-007`: JSON and Markdown reports MUST contain no source body, diff body, prompt, commit
  message, ticket text, or command output. They MUST warn that paths and digests remain sensitive.
- `CEM-PILOT-008`: Markdown MUST render all repository-controlled strings as inert text. Control
  characters, backticks, links, HTML, and line breaks MUST NOT create headings, links, HTML, or new
  list items.
- `CEM-PILOT-009`: the CI verifier example MUST derive base and target revisions itself, require the
  map's base to equal that independently derived base, use the same patch profile as prepare, and
  apply explicit policy caps. Separately, the local CLI MUST generate a reviewer-readable report.
  The default CI example MUST NOT publish that sensitive report; hosted exposure is opt-in
  repository policy.

### Trial and interoperability

- `CEM-PILOT-010`: the trial harness MUST enforce 30 treatment and 30 control lanes, deterministic
  assignment, hash-bound task inputs, arm-neutral reviewer packets, packet-bound blind labels,
  operator observations, strict validation, and preregistered formulas and gates. Synthetic
  fixtures MUST remain visibly unbound and report `NOT_RUN`, never success; only a separately
  frozen human manifest with bound inputs can produce `PASS` or `FAIL`.
- `CEM-PILOT-011`: the SDK-neutral interop kit MUST contain raw, digest-pinned inputs and expected
  outcomes for valid, invalid, producer, and all five drift states, plus exact algorithms that do
  not require importing or invoking Corvint.
- `CEM-PILOT-012`: the implementation matrix MUST distinguish Corvint-authored reference attestation
  from independently authored producers and consumers. Empty independence cells MUST remain empty.

### Operability

- `CEM-PILOT-013`: all map, patch, and report reads and writes MUST be bounded; writes MUST be
  atomic, private, durable (file synced before rename, directory synced after), and reject symlink
  or non-file targets. Where the platform exposes FIFOs, a bounded read's final open MUST be
  nonblocking and no-follow so a regular file replaced after inspection cannot block the command.
  Corvint implements that opener on Darwin and Linux; Windows filesystem inputs do not expose FIFOs,
  and other GOOS targets are unsupported rather than allowed to fall back to a blocking open.
  Every worktree-rooted CEM output MUST refuse any path segment equal to `.git` under case folding;
  a separately verified Git-directory root may still publish its fixed internal artifact paths.
  `cite`, `mark`, and `prepare` MUST serialize their map read-modify-write
  under an exclusive lock on the sibling `<effective-output-map>.lock` file, removed on release
  only while that path still names the held lock file; a non-empty file already at the lock path is
  someone's data, never a lock, and MUST refuse `publish-failed` without being removed;
  for `cite` and `mark`, omitted `--output` makes the input map the effective output. A lock wait
  that outlives its hang detector MUST refuse with `map-locked` rather than lose an update. `prepare`
  MUST restore the single `<map>.bak-<hex>` left by an interrupted map-and-cache publication before
  treating the map as missing, and MUST refuse with `map-unavailable` when several remain. A
  `prepare --patch` cache selecting `<map>.lock` or a `<map>.bak-` name under case folding MUST
  refuse `invalid-arguments` before any write, so a cache is never removed as a lock or restored as
  the map. An explicit `cite`, `mark`, or `report` output selecting its input map's lock or backup
  name under case folding MUST make the same refusal. A pair publication MUST directory-sync the
  set-aside rename, both new final entries before removing the prior-file backup, the backup
  removal, and any rollback. After `prepare` validates a present final map, it MUST durably remove
  one recognized backup left by a crash after that final landed; several backups
  remain ambiguous and MUST refuse `map-unavailable`. An abandoned empty lock grants no age-based
  takeover: only a process that acquires its advisory lock and confirms its inode may remove it on
  release. A map or patch read whose path has an existing ancestor that is a symlink or not a
  directory MUST refuse with that read's `map-unavailable` or `patch-unavailable`, not
  `publish-failed`.
  Platforms without advisory file locks MUST refuse these updates. A `report --output` equal to the
  input map path under case folding, or naming the same existing file as the input map (a
  normalization or locale alias the volume applies), MUST refuse `invalid-arguments` before any write.
- `CEM-PILOT-014`: a clean local install plus prepare, one disposition update, status, report, and
  verify MUST be demonstrated in under 15 minutes with the synchronized commit recorded. The
  POSIX-only harness MUST first invoke and record a sanitized offline inventory for its trusted
  local builder (Python >=3.9, setuptools >=61, and pip and wheel present). It MUST build exactly one
  bounded regular non-symlink wheel from the exact private Corvint clone with
  `python -m pip --isolated --disable-pip-version-check wheel --no-index --no-deps
  --no-build-isolation --no-cache-dir --wheel-dir`, record its filename, bytes, and SHA-256, install
  that exact wheel into a fresh environment with offline/no-dependency/no-cache pip flags, prove the
  installed `corvint_cli` resolves inside that environment, run its `corvint --version`, and reject a
  wheel whose digest changes. Private execution MUST use private `HOME` and `TMPDIR` directories.
- `CEM-PILOT-015`: all CLI operations MUST emit deterministic JSON envelopes and stable non-zero
  exit codes for invalid input or failed policy.
- `CEM-PILOT-016`: Corvint repository changes MUST use the public CEM workflow itself and retain a
  checked-in sidecar for substantive implementation diffs. Self-use MUST NOT count as independent
  interoperability or external value evidence.
- `CEM-PILOT-017`: every Git subprocess MUST ignore ambient repository/config redirection. Patch
  producers MUST stop at a hard byte ceiling, and machine-readable next actions MUST be argv arrays
  rather than executable shell strings.
- `CEM-PILOT-018`: when prepare refuses an existing map under `CEM-PILOT-002`, the untyped
  `cannot read CEM map` refusal MUST append exactly one bounded, source-content-free recovery line
  after `: `: `the existing map records a different base or patch; pass --replace to regenerate` for
  a base or patch mismatch, or `the existing map is not a valid CEM document; pass --replace to
  regenerate` for an unparseable map, never the parser detail. An absent or unreadable map MUST keep
  the fixed `cannot read CEM map` text byte-for-byte. Exit status and error code are unchanged
  (decision 0092).
- `CEM-PILOT-019`: `cem verify` same-path drift MUST use only the five frozen statuses. A target entry
  whose blob OID equals the evidence `blobOid` MUST be `stable` whatever its mode. A target entry with
  a different OID that is not a regular-file blob (mode `100644` or `100755`), such as a symlink,
  directory, or gitlink, MUST be `deleted` with no target blob OID and no target span, and MUST NOT
  be searched. `deleted` therefore means that no regular file with different content exists at the
  literal path, not that the path was removed; no `type-changed` status or reason member is added
  (decision 0098).

## Non-goals

- proving that cited evidence semantically supports or caused an edit;
- automatic evidence retrieval or citation generation;
- IDE, MCP, dashboard, hosted service, database, or repository upload;
- Jira, E2E, signature, attestation, or multi-repository extensions to `cem/0.1`;
- claiming interoperability from Corvint-authored tests or fixtures;
- running or fabricating the real 30×30 trial without human operators and blind reviewers.

## Failure and trust model

Repository bytes, paths, revisions, maps, and patches are untrusted. Parsing and Git operations are
bounded. Structural ambiguity, unsupported patch forms, missing objects, duplicate mappings,
outdated maps, unsafe drift, and policy violations fail closed. Semantic adequacy remains a reviewer
judgment and is never upgraded by structural validity.

The first-run builder is trusted local input, not a hermetic or reproducible build environment. Pip
index, dependency, cache, proxy, and configuration access is disabled and recorded inputs are pinned,
but the harness is not an operating-system network sandbox and does not download or vendor a build
toolchain. The builder may use its user-site build tools; the separate fresh destination remains
Python-isolated and receives no user base. The sanitized builder environment receives only the
invoking process's explicit trusted user base, not caller-supplied Python injection state. A missing
or old prerequisite fails preflight with a distinct error before source build.

Whitespace outside an exact evidence span may relocate it only when the exact bytes have one
non-overlapping match. Whitespace inside the cited span makes it stale. Multiple overlapping or
non-overlapping exact matches make it ambiguous. `relocated` does not assert movement: `stable`
requires an identical target blob, so an unmoved span in an edited file is `relocated` with a
`targetSpan` equal to the recorded span of the same `evidenceId` (decision 0094). `deleted` does not assert
removal: a same-path entry that became a symlink, directory, or gitlink with different content is
`deleted`, and a reader who needs the cause inspects the target tree entry (`CEM-PILOT-019`).

## Acceptance matrix

The `src/context_corvint*` implementation cited below is a historical reference, not live authority:
decision 0012 R0 (`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python
implementation non-authoritative and slated for separate removal.

| Requirement | Implementation | Evidence |
|---|---|---|
| CEM-PILOT-001..005 | `src/context_corvint_cem_workflow.py`, `src/corvint_cli.py` | `tests/test_context_corvint_cem_workflow.py`, `tests/test_cli.py` |
| CEM-PILOT-006..008 | `src/context_corvint_cem_report.py` | deterministic, content-omission, and hostile-rendering tests in `tests/test_context_corvint_cem_report.py` |
| CEM-PILOT-009 | `examples/cem/verify-pr.sh`, `docs/CEM-CI.md` | `script/cem-verify-pr_test.sh` (`make cem-verify-pr-test`, a `make gate` member) runs the example on a fixture repository: a cited cem/0.2 map passes, the unknown-hunk cap fails over and passes at the cap, a map whose base differs from the supplied base fails `base-revision-mismatch`, and no work directory or checkout change remains. The local report is `cem report` under CEM-PILOT-006..008 |
| CEM-PILOT-010 | `experiments/cem-30x30` | synthetic-provenance, bound-packet, blinding, and formula tests in `tests/test_cem_experiment.py`; real result remains `NOT_RUN` |
| CEM-PILOT-011..012 | `interop/cem-0.1` | `tests/test_cem_interop.py`; independent matrix remains unclaimed |
| CEM-PILOT-013 | workflow and CLI output helpers; local dogfood citation staging through public `cem cite --output` | adversarial path, size, mode, atomicity, and symlink tests; `TestReadBoundedFileFIFOReplacementIsBounded`, `TestPairRollbackSyncsRestoredDirectory`, `TestPairSuccessSyncsFinalsBeforeRemovingBackup`, `TestPairFinalSyncFailureRetainsBackup`, `TestConcurrentMarksAllSurvive`, `TestMutationsLockTheirEffectiveOutput`, `TestMutationLockNeverRemovesAnInputMap`, `TestCEMOutputsRefuseGitMetadata`, `TestCEMCLIRefusesGitMetadataOutput`, `TestCEMOutputsRefuseMapSidecars`, `TestPrepareCacheAliasingRefused`, `TestMapReadThroughSymlinkedParentIsUnavailable`, `TestPrepareRecoversInterruptedPairBackup`, and `TestPrepareRemovesValidatedStalePairBackup`; `script/dogfood-change_test.sh` checks bounded whole-plan validation, unchanged prepared-map bytes on failure, and owned staging/child cleanup |
| CEM-PILOT-014 | `experiments/cem-first-run/run.py`, retained closed `/0` and `/0.1` result schemas | synthetic orchestration, builder-preflight, exact-wheel build/install, private-path, wheel-count/symlink/digest, private-clone isolation, porcelain-status identity, interruption, bounded-output, private-write, and non-promotion tests in `tests/test_cem_first_run.py`; one experienced-operator Beamfall run completed in 15.715100875 seconds with all 8 checks passing and 4/4 hunks explicitly unknown; adoption remains `NOT_RUN` |
| CEM-PILOT-015 | `src/corvint_cli.py` | CLI success, invalid-input, and policy-failure tests |
| CEM-PILOT-016 | `docs/DOGFOOD.md`, `script/dogfood-change.sh`, `.corvint/change.cem.json` | synchronized self-change report and local outcome trace; caller-selected plans remain bounded local orchestration, with no new CEM wire or interoperability claim |
| CEM-PILOT-017 | Git environment and `experiments/_bounded_process.py` | hostile-environment, output-ceiling, process-group cleanup, and argv-shape regressions shared by first-run and dogfood measurement tests |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `CEM-PILOT-002` | `internal/cem/workflow/commands.go` `resumeCandidate` | `TestUnsafeExistingMapNotOverwritten`, `TestPrepareRefusesOversizeMapUnderMissingNamedRoot` (oversize map under a repository path containing `missing`) |
| `CEM-PILOT-003` | `internal/cem/workflow/workflow.go` `resolveHunk`, `internal/cem/workflow/commands.go` `evidenceSpan` | `TestMarkRejectsNonCanonicalOrdinal` (`01`, `+1`), `TestCiteRefusesNegativeByteSpan` (library `Session.Cite` with `-1:3`) |
| `CEM-PILOT-011` | `internal/cem/patch/patch.go` `validateSimilarity`, `parseHunkBody`, `checkGroupShape`, `parseHunkHeader`; `internal/cem/sim/sim.go` `groupOldBytes` | kit erratum 2 (decision 0192): `TestParserRejections` rows `similarity-leading-zero`, `body-record-without-lf`, `marker-without-lf`; `TestHunkHeaderSuffixUninterpreted` (`@@x`); `TestCreateAndDeleteAcceptContiguousHunks`; `TestMultiHunkCreateAndDeleteSimulate`; `TestDeleteModeBoundToBaseMode` |
| `CEM-PILOT-013` | `internal/cem/workflow/read.go` `renderReport` | `TestReportRefusesCaseVariantOfMapPath` (case-insensitive volume; skips on a case-sensitive one), `TestReportRefusesNormalizationVariantOfMapPath` (normalization-insensitive volume; skips otherwise) |
| `CEM-PILOT-018` | `internal/cem/cli/cli.go` `emitCEMError`, `internal/cem/workflow/commands.go` `resumeCandidate` | `TestCEMPrepareKeepsInheritedMapReplaceGuidance` (mismatch line, invalid-map line, absent-map fixed text) |
| `CEM-PILOT-019` | `internal/cem/verify/verify.go` `driftItem` | `TestDriftSymlinkSameOIDIsStable` (identical-OID symlink), `TestDriftNonRegularTargetEntryIsDeleted` (changed symlink, directory, gitlink); `TestInteropDriftCases` over kit erratum-1 vector `deleted-symlink-changed` (decision 0098 amendment) |

## Rollout and rollback

Ship only as experimental documentation and local tooling. Existing `begin|cite|mark|verify`
contracts remain compatible. New convenience commands may be removed without a data migration;
maps remain plain `cem/0.1`. If the 15-minute journey fails, simplify the workflow before adding
features. If external implementations cannot consume the kit within one engineer-day or the 30×30
trial misses its value gates, redesign or kill CEM 0.1 rather than expanding it.

## Open evidence

- hostile Markdown rendering and local reviewer-report CLI are implemented and focused tests pass;
- hosted-CI report publication is deliberately disabled because paths and digests are sensitive;
  any opt-in publication needs an explicit repository policy;
- the original `cem-first-run-result/0` Beamfall operator run failed during source installation
  because the fresh destination environment lacked the wheel build command; that immutable failure
  remains evidence and is not promoted by the repaired `cem-first-run-result/0.1` contract;
- the bounded timed first-run harness and versioned closed result contracts are implemented; the
  original `/0` Beamfall failure and repaired `/0.1` pass are preserved under `benchmarks/results/`;
  the repaired experienced-operator run took 15.715100875 seconds and demonstrates the under-15-
  minute requirement once, but a first-use median and external adoption remain `NOT_RUN`;
- no external independent implementation exists;
- the real 30-treatment/30-control trial has not run.
