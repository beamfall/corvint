# CEM pilot kit

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: accepted
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/CHANGE-EVIDENCE-MAP.md`, independent product,
onboarding, security, and wire-contract reviews recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md`

## Agent digest
- Claim: The CEM pilot kit freezes a safe first-run, reviewer-report, and outcome-trial contract for experimental CEM 0.1 use.
- Status: accepted/experimental
- Exists: `internal/cem`, reviewer reporting, the experimental read-only hunk JSON projection (CEM-PILOT-028..031), `experiments/cem-first-run` and frozen contracts; CEM-PILOT-009, 011..012 and 020..027 are retired with `cem/0.x` (decision 0525).
- Blocked on: independent external outcome evidence.
- Read next: `cem-stable-v1.md` and `../CHANGE-EVIDENCE-MAP.md`.

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
- `conformance/cem-0.1` was a Corvint-owned conformance seed, not independent interoperability;
  V1-1278 S3 deleted it (`CEM-V1-220`).
- An offensive review found six verifier defects; repairs and focused regressions passed review.
- No external producer or consumer has passed the contract and no real 30×30 outcome trial has run.

## Requirements

### First-run workflow

- `CEM-PILOT-001`: `cem prepare --base REV --target REV` MUST derive the exact committed patch with
  the documented CI diff profile, exclude only the map path, create private atomic outputs, and
  return an ordered hunk worklist. (proposed 2026-09-25, V1-0301, not accepted) Today a patch with
  a binary file refuses the whole prepare with `binary-patch`, and one with an `old mode`/`new mode`
  change refuses it with `malformed-patch`, because the frozen `cem/0.1` grammar
  (now `protocol/cem-1.0/stable/inherited/cem-0.1-algorithms.md`) and the `cem/0.2` hunk wire and hunk IDs have no such hunk. A
  new versioned profile MAY bind each binary or mode-only file change as its own hunk kind whose
  disposition is always `unknown`: no citation or mark can make it ready, so a map that contains
  one never reaches `ready-for-ci` without an explicit policy exception. Neither frozen profile
  changes its bytes for this.
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
  repository policy. Amended 2026-10-12 (V1-1278, CEM-V1-215): the 0.x example `verify-pr.sh` has
  no producer left and its fixture test is deleted; the stable CI path is `verify-portable.sh` with
  `CEM_PROFILE=cem/1.0`.

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
  (accepted, decision 0512) For in-place updates, including an explicit
  output equal to the input path, the lock MUST precede the first bounded map
  read: concurrent valid atomic replacements MUST serialize with all twelve marks succeeding
  and surviving. Safe-read identity, size, type, and no-follow checks MUST remain intact;
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
  (proposed 2026-09-25, V1-0141, not accepted) An absolute `report --output` whose existing parent
  directory lies outside the worktree and its Git directories MUST be published there; one whose
  parent lies inside them, by file identity, MUST refuse `invalid-arguments` before any write.
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
- `CEM-PILOT-018`: when prepare refuses an existing map under `CEM-PILOT-002`, the
  `cannot read CEM map` refusal MUST append exactly one bounded, source-content-free recovery line
  after `: `: `the existing map records a different base or patch; pass --replace to regenerate` for
  a base or patch mismatch, or `the existing map is not a valid CEM document; pass --replace to
  regenerate` for an unparseable map, never the parser detail. An absent or unreadable map MUST keep
  the fixed `cannot read CEM map` text byte-for-byte. Exit status and error code are unchanged
  (decision 0092). (accepted 2026-09-26, decision 0424; from decision 0398) The refusal also carries
  `code` `map-unavailable` (CCF-V1-004).
- `CEM-PILOT-019`: `cem verify` same-path drift MUST use only the five frozen statuses. A target entry
  whose blob OID equals the evidence `blobOid` MUST be `stable` whatever its mode. A target entry with
  a different OID that is not a regular-file blob (mode `100644` or `100755`), such as a symlink,
  directory, or gitlink, MUST be `deleted` with no target blob OID and no target span, and MUST NOT
  be searched. `deleted` therefore means that no regular file with different content exists at the
  literal path, not that the path was removed; no `type-changed` status or reason member is added
  (decision 0098).
- `CEM-PILOT-033`: (proposed 2026-10-24, V1-2522, not accepted) On a platform without advisory
  file locks (every GOOS but darwin and linux, so windows/amd64, which the 1.0 release spec lists
  `UNSUPPORTED`), the CEM update lock that `begin`, `prepare`, `cite`, `mark`, `cover` and
  `discriminate` take MUST refuse `publish-failed` before it creates any directory or lock file (a
  command whose map read fails first keeps that read's refusal), and the refusal message MUST end
  with `: serialized CEM map updates are unsupported on this platform`, the exact platform refusal
  that `internal/testplatform` matches. Before V1-2522 the lock created `.corvint/`, then dropped
  that cause and reported only `update lock for ".corvint/change.cem.json" cannot be opened`.
  Non-goals: taking a lock on Windows (V1-2485 owns the Windows 1.0 scope decision), and a new wire
  code. Failure mode: a Windows `cem prepare` exits with `publish-failed` naming the platform; no map,
  cache, `.corvint/` directory or lock file is written. Rollback: revert the commit; the refusal
  keeps its code and loses only its cause and its up-front position.

### Hunk review projection (V1-0501)

The owner requested a clickable review of immutable evidence, obligations, retained test
witnesses and remaining unknowns on 2026-09-29. This extension remains experimental and
composes existing CEM/OCM verification; it supplies no semantic proof or test execution.

- `CEM-PILOT-028`: `cem report --format json` MUST emit a read-only `cem-review/0` projection
  under the command's `review` member, with one record per derived patch hunk in patch order,
  including hunks absent from an invalid map. Each record MUST carry its content digest, identifier,
  path, both ranges, line revision/range (base for deletion, target otherwise), map validity,
  disposition, exact reason, and cited evidence identities, spans, inclusion relations and available
  drift states. Invalid maps MUST retain the failed verifier verdict; surplus mapped hunks MUST be
  listed separately. A JSON report MUST reject `--output`, write no report and retain `mutates:false`.
- `CEM-PILOT-029`: An explicitly supplied `--ocm MAP` MUST be verified by the owning OCM reader
  against the exact retained CEM bytes and independently supplied base/target. Only verified
  `hunkIds` relations may join obligations to hunk records. Unknown obligations MUST remain in
  `unmappedObligations`, with their exact reason, rather than being assigned to every hunk. A failed
  OCM MUST preserve its verdict and contribute no joins; absent OCM input MUST state
  `NOT_PRODUCED` / `ocm-not-supplied`. A requested but unavailable owning verifier MUST state
  `NOT_PRODUCED` / `ocm-verifier-unavailable`, retain all hunk records and return a non-success
  command verdict. The optional reader MUST be supplied per invocation, with no global registration.
- `CEM-PILOT-030`: JSON and the published Markdown report MUST bind the same `recordSetSha256`,
  computed as SHA-256 of Go `encoding/json.Marshal` bytes for the complete `review` object before
  adding `recordSetSha256`; this is a deterministic projection digest, not a CEM identity or an
  authority receipt. The object MUST include the map/derived-patch/declared-patch digests, immutable
  endpoints, validity, optional OCM projection, all hunks, surplus map rows and unassigned obligations.
  Every repository string interpolated into Markdown MUST use inert code spans. JSON MUST escape
  strings and include no source/diff bodies or command output. Equivalent invocations with the same
  retained inputs MUST produce identical digests.
- `CEM-PILOT-031`: A missing coverage witness MUST state `NOT_PRODUCED` /
  `no-coverage-witness`; existing coverage and discrimination witnesses MUST retain their state,
  identity and exact unrun reason. Every hunk MUST state test execution `NOT_RUN` /
  `review-does-not-execute-tests`. Structural test claims, retained caller witnesses and operator
  outcomes MUST NOT be presented as a current passing test or semantic support.
- `CEM-PILOT-032`: (proposed 2026-10-08, V1-0513, not accepted) When a supplied OCM is invalid,
  the published Markdown report MUST state the OCM validity, the native OCM state and every
  verification issue code and message as inert code spans, alongside the same `recordSetSha256` as
  the JSON projection, and MUST NOT render a ready state, a valid verdict or any obligation join.
  An OCM that is not a JSON object MUST be refused without publishing a report.

### Portable CI verifier

Retired by V1-1278 S3 (`CEM-V1-222`, `CEM-V1-223`): `cem01-go ci` and the `examples/cem` portable
workflow read `cem/0.1` maps only and are deleted; the four requirements below are kept as history.

- `CEM-PILOT-020`: (proposed; accepted with decision 0356) the portable CI example
  (`examples/cem/github-actions-portable.yml`, `examples/cem/verify-portable.sh`) MUST pin the
  `interop/cem01-go` verifier by module pseudo-version and executable SHA-256, MUST build it only in
  a fetch step with `-trimpath` and `CGO_ENABLED=0`, MUST check the executable digest before
  executing it and exit 2 without executing on a mismatch, and MUST run verification inside a
  network-denied sandbox. Verification uses no LLM, index, retriever, or network (decision 0356).
- `CEM-PILOT-021`: (proposed; accepted with decision 0356) `cem01-go ci` MUST write exactly one JSON
  line with schema `cem-ci-report/0` and the fixed members `schema`, `verdict`, `exit`, `code`,
  `profile`, `base`, `head`, `mapPath`, `mapSha256`, `patchSha256`, `hunks`, `evidence`, `drift`,
  and `limits`. The line MUST be byte-deterministic for the same inputs, at most 6 MiB, and MUST
  hold only digests, paths, spans, counts, and verdicts, never source or diff text. An invalid
  invocation MUST echo no argument.
- `CEM-PILOT-022`: (proposed; accepted with decision 0356) `cem01-go ci` MUST derive the patch with
  the `docs/CEM-CI.md` profile and exit 0 `accepted`, 1 `rejected`, 2 `operational`, 3
  `missing-evidence` (map absent, or unknown hunks), 4 `unsupported-profile` (a map `spec` other
  than `cem/0.1`), or 5 `repository-mismatch` (a declared commit absent, or map `baseRevision` not
  the declared base), with the verdict and a bounded `code` in the report. For a structurally valid
  map, unsafe drift ranks before unknown hunks. (proposed 2026-09-25, V1-0133, not accepted) The
  profile is read only from a strictly parsed map, and a `cem/0.1` map MUST pass the `verify`
  structural checks, exiting 1 when it fails them, before its `baseRevision` is compared. The `verify` mode and its adapter ABI (`CEM-GO-002`)
  are unchanged.
- `CEM-PILOT-023`: (proposed; accepted with decision 0356) `examples/cem/README.md` MUST give the
  exact pinned install, digest computation, and invocation, the exit taxonomy, the report members,
  and the verifier's limits.

Walkthrough pointer (V1-1263, `CEM-V1-083`, informative): the timed run from `cem prepare` on a
non-Corvint repository with no `.taskman/` to a provider-neutral `cem/1.0` map accepted by both
`corvint cem verify-stable` and `cem01-go verify-stable-canonical` is recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-neutral-criterion-walkthrough.md`, with every refusal met on the
way; its map was hand-assembled. Since V1-1290 (`CEM-V1-240`) the walkthrough to follow is
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-cem-plain-criterion-producer.md`: after `cem prepare` and `cem cite`,
`corvint-cem-candidate assemble-stable --experimental` with a `cem-stable-plain-assembly/1` request
emits the plain map with no Tasks store and no hand-assembly step. The public case is
`protocol/cem-1.0/stable/neutral-criterion/`, which the producer re-produces byte for byte.

### Workflow recipes

Retired by V1-1278 S3 (`CEM-V1-223`): `examples/cem/recipes/` and its gate member are deleted; the
four requirements below are kept as history.

Owner-requested for 0.7.0 on 2026-09-22 (ticket V1-0026); delivery is experimental.

- `CEM-PILOT-024`: `examples/cem/recipes/` MUST ship three ordinary Bash scripts, each composing
  only existing commands: `understand-change.sh` (`impact --base`, `affected --base`, `context`),
  `review-change.sh` (`cem prepare`, strict `cem status` with unknown/mechanical caps, a committed-map
  check, `cem report`, `review --base`), and `ci-verify.sh` (`examples/cem/verify-portable.sh`). They
  add no Corvint verb, workflow language, scheduler, service, or LLM call, and `ci-verify.sh` needs
  no hosted Corvint service or network.
- `CEM-PILOT-025`: `examples/cem/recipes/README.md` MUST state for each recipe its supported inputs,
  prerequisites, exact invocation, expected output, completion boundary, experimental status and
  owning contract, and one missing, stale, and unsupported evidence case with its recovery.
- `CEM-PILOT-026`: a recipe MUST NOT convert a refusal or uncertainty into success. Understand and
  review exit 0 only when every step exits 0 (and impact is `READY`, the map is committed at HEAD),
  3 when a step refuses or evidence is incomplete, and 2 for invalid recipe input or a timeout; the CI
  recipe exits with the verifier's 0..5, and 2 when the verifier returns no verdict. Every step's
  stdout and stderr stay in `RECIPE_OUT`, which must be new or empty, and no recipe replaces an
  outdated or invalid map.
- `CEM-PILOT-027`: each recipe step MUST run in its own process group bounded by `RECIPE_TIMEOUT`
  seconds (default 600), and the recipe MUST kill the running step's group on its own exit, `HUP`,
  `INT`, or `TERM`: `TERM` first, then `KILL` after a 2-second grace, so a step that ignores `TERM`
  still ends within `RECIPE_TIMEOUT` plus the grace.

## Non-goals

- proving that cited evidence semantically supports or caused an edit;
- automatic evidence retrieval or citation generation;
- IDE, MCP, dashboard, hosted service, database, or repository upload;
- Jira, E2E, signature, attestation, or multi-repository extensions to `cem/0.1`;
- claiming interoperability from Corvint-authored tests or fixtures;
- running or fabricating the real 30×30 trial without human operators and blind reviewers.
- a workflow language, scheduler, or hosted runner for the recipes (`CEM-PILOT-024`).

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
| CEM-PILOT-009 | `examples/cem/verify-pr.sh`, `docs/CEM-CI.md` | retired by V1-1278 (`CEM-V1-215`, `CEM-V1-223`): the example read cem/0.1 and cem/0.2 only, so S2b deleted `script/cem-verify-pr_test.sh` and its gate member and S3 deleted `examples/cem` and `docs/CEM-CI.md`; they are kept in Git history. The local report is `cem report` under CEM-PILOT-006..008 |
| CEM-PILOT-010 | `experiments/cem-30x30` | synthetic-provenance, bound-packet, blinding, and formula tests in `tests/test_cem_experiment.py`; real result remains `NOT_RUN` |
| CEM-PILOT-011..012 | `interop/cem-0.1` | retired by V1-1278 S3 (`CEM-V1-220`): `interop/cem-0.1` is deleted; the historical evidence was `tests/test_cem_interop.py`, and the independent matrix was never claimed |
| CEM-PILOT-013 | workflow and CLI output helpers; local dogfood citation staging through public `cem cite --output` | adversarial path, size, mode, atomicity, and symlink tests; `TestReadBoundedFileFIFOReplacementIsBounded`, `TestPairRollbackSyncsRestoredDirectory`, `TestPairSuccessSyncsFinalsBeforeRemovingBackup`, `TestPairFinalSyncFailureRetainsBackup`, `TestConcurrentMarksAllSurvive`, `TestMarkReplacementAtFirstRead`, `TestMarkHostileInputPrecedence`, `TestMutationsLockTheirEffectiveOutput`, `TestMutationLockNeverRemovesAnInputMap`, `TestCEMOutputsRefuseGitMetadata`, `TestCEMCLIRefusesGitMetadataOutput`, `TestCEMOutputsRefuseMapSidecars`, `TestPrepareCacheAliasingRefused`, `TestMapReadThroughSymlinkedParentIsUnavailable`, `TestPrepareRecoversInterruptedPairBackup`, and `TestPrepareRemovesValidatedStalePairBackup`; `script/dogfood-change_test.sh` checks bounded whole-plan validation, unchanged prepared-map bytes on failure, and owned staging/child cleanup |
| CEM-PILOT-014 | `experiments/cem-first-run/run.py`, retained closed `/0` and `/0.1` result schemas | synthetic orchestration, builder-preflight, exact-wheel build/install, private-path, wheel-count/symlink/digest, private-clone isolation, porcelain-status identity, interruption, bounded-output, private-write, and non-promotion tests in `tests/test_cem_first_run.py`; one experienced-operator Beamfall run completed in 15.715100875 seconds with all 8 checks passing and 4/4 hunks explicitly unknown; adoption remains `NOT_RUN` |
| CEM-PILOT-015 | `src/corvint_cli.py` | CLI success, invalid-input, and policy-failure tests |
| CEM-PILOT-016 | `docs/DOGFOOD.md`, `script/dogfood-change.sh`, `.corvint/change.cem.json` | synchronized self-change report and local outcome trace; caller-selected plans remain bounded local orchestration, with no new CEM wire or interoperability claim |
| CEM-PILOT-017 | Git environment and `experiments/_bounded_process.py` | hostile-environment, output-ceiling, process-group cleanup, and argv-shape regressions shared by first-run and dogfood measurement tests |
| CEM-PILOT-020..023 | `interop/cem01-go/ci.go`, `examples/cem/verify-portable.sh`, `examples/cem/github-actions-portable.yml`, `examples/cem/README.md` | retired by V1-1278 S3 (`CEM-V1-222`, `CEM-V1-223`): `cem01-go ci` read `cem/0.1` maps only, so `interop/cem01-go/ci.go`, its tests and the `examples/cem` portable workflow are deleted and kept at commit `5a056d0a`; `cem01-go verify-stable-canonical` is the portable `cem/1.0` check |
| CEM-PILOT-024..027 | `examples/cem/recipes/understand-change.sh`, `review-change.sh`, `ci-verify.sh`, `bounded.sh`, `README.md` | retired by V1-1278 S3 (`CEM-V1-223`): `examples/cem/recipes/`, `script/cem-recipes_test.sh` and the `cem-recipes-test` gate member are deleted and kept at commit `5a056d0a` |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `CEM-PILOT-002` | `internal/cem/workflow/commands.go` `resumeCandidate` | `TestUnsafeExistingMapNotOverwritten`, `TestPrepareRefusesOversizeMapUnderMissingNamedRoot` (oversize map under a repository path containing `missing`) |
| `CEM-PILOT-003` | `internal/cem/workflow/workflow.go` `resolveHunk`, `internal/cem/workflow/commands.go` `evidenceSpan` | `TestMarkRejectsNonCanonicalOrdinal` (`01`, `+1`), `TestCiteRefusesNegativeByteSpan` (library `Session.Cite` with `-1:3`) |
| `CEM-PILOT-011` | `internal/cem/patch/patch.go` `validateSimilarity`, `parseHunkBody`, `checkGroupShape`, `parseHunkHeader`; `internal/cem/sim/sim.go` `groupOldBytes` | kit erratum 2 (decision 0192): `TestParserRejections` rows `similarity-leading-zero`, `body-record-without-lf`, `marker-without-lf`; `TestHunkHeaderSuffixUninterpreted` (`@@x`); `TestCreateAndDeleteAcceptContiguousHunks`; `TestMultiHunkCreateAndDeleteSimulate`; `TestDeleteModeBoundToBaseMode` |
| `CEM-PILOT-013` | `internal/cem/workflow/read.go` `renderReport` | `TestReportRefusesCaseVariantOfMapPath` (case-insensitive volume; skips on a case-sensitive one), `TestReportRefusesNormalizationVariantOfMapPath` (normalization-insensitive volume; skips otherwise) |
| `CEM-PILOT-018` | `internal/cem/cli/cli.go` `emitCEMError`, `internal/cem/workflow/commands.go` `resumeCandidate` | `TestCEMPrepareKeepsInheritedMapReplaceGuidance` (mismatch line, invalid-map line, absent-map fixed text) |
| `CEM-PILOT-019` | `internal/cem/verify/verify.go` `driftItem` | `TestDriftSymlinkSameOIDIsStable` (identical-OID symlink), `TestDriftNonRegularTargetEntryIsDeleted` (changed symlink, directory, gitlink); the kit erratum-1 vector `deleted-symlink-changed` (decision 0098 amendment) ran through the 0.x verifier, whose interop corpus test was retired by V1-1278 S2b (`CEM-V1-215`) |
| `CEM-PILOT-033` | `internal/cem/publish/lock.go` `Lock`, `lock_other.go` and `lock_unix.go` `LockRefusal` | `TestLockRefusalIsNilWithAdvisoryLocks` (darwin, linux), `TestLockRefusesUpFrontWithoutAdvisoryLocks` (other GOOS; type-checked by `make windows-vet`, run only on a Windows host), `TestCEMLockAndTasksRefusalsSkipOnlyOnWindows` (`internal/testplatform`); the Windows suites of `internal/cem/workflow`, `internal/lrfrepo`, `internal/frontiernextrepo` and `conformance/ocm-v0` skip on this refusal through `testplatform.SkipWindowsRefusal` (Windows rerun `NEEDS-HOST`) |

## Rollout and rollback

Ship only as experimental documentation and local tooling. Existing `begin|cite|mark|verify`
contracts remain compatible. New convenience commands may be removed without a data migration;
maps are `cem/1.0` since decision 0525. V1-1278 S3 removed the recipes under `examples/cem/recipes/`
and the `cem-recipes-test` target; restoring them is a revert of that commit, and no command, map, or
ledger depends on them. If the 15-minute journey fails, simplify the workflow before adding
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
- (historical; retired by V1-1278 S3) the portable verifier workflow has not run on a GitHub runner, and no pseudo-version or
  executable digest of a published revision is recorded yet (`CEM-PILOT-020`).
- (historical; retired by V1-1278 S3) the recipes (`CEM-PILOT-024`..`027`) were exercised only on Corvint-authored fixtures; time to a
  first valid receipt, review, or CI verification by a reader unfamiliar with them is
  `NOT_OBSERVED`, and the CI recipe's verifier comes from a checkout build, not a published pin.

## Hunk projection acceptance and rollback (V1-0501)

The simpler baseline remains Markdown `cem report` and the existing console chain. JSON adds
machine consumption without a new map profile, datastore, source body, semantic classifier or
execution provider. With `--format json`, `--output` is refused; default/`markdown` report publication
keeps the existing bounded local path rules. The optional owning OCM reader is composed by the native
command boundary; a standalone CEM consumer without it reports unavailable explicitly.

| Requirement | Implementation | Focused acceptance evidence |
|---|---|---|
| CEM-PILOT-028 | `internal/cem/workflow/review_projection.go`, `read.go`; `internal/cem/cli/cli.go` | `TestReviewProjectionParityAndReadOnly`, `TestReviewProjectionInvalidMapRetainsUnmappedHunks`, `TestReviewProjectionArguments` |
| CEM-PILOT-029 | per-call `ReviewOCMReader`; `internal/lrfrepo/ocm_read.go` `ReadOCMReview` | `TestReviewProjectionOCMJoinAndInvalidRefusal`, `TestReviewProjectionPerCallResolver`; the native 0.x adapter test is retired by CEM-V1-210 (the OCM reader admits cem/1.0 only); the invalid stable OCM stays visible in Markdown: `TestV11305StableReportOCM` |
| CEM-PILOT-030 | retained-input projection digest and inert Markdown rendering | `TestReviewProjectionParityAndReadOnly`, `TestReviewProjectionHostileText` |
| CEM-PILOT-031 | retained coverage/discrimination fields and explicit execution boundary | `TestReviewProjectionParityAndReadOnly`; existing coverage/discrimination report regressions |
| CEM-PILOT-032 | `renderReviewProjection` OCM validity/state/issue lines (proposed, V1-0513) | 0.x native rows retired by CEM-V1-210 (the OCM reader admits cem/1.0 only); the cem/1.0 report OCM lines: `TestV11305StableReportOCM` (an invalid OCM keeps validity `false`, state `invalid`, the escaped issue code and the JSON record-set digest in Markdown and joins no obligation), `TestV11305StableReportOCMWithoutVerifier` |

The deterministic qualification is a real producer CEM/OCM fixture, JSON/Markdown digest parity,
all derived hunks on an invalid-map fixture, explicit callback OCM joins and native unknown reasons, hostile
string rendering, and invocation-local reader isolation. An actual final dogfood report must be
inspected through this projection before delivery; the integrated receipt and worked example are
owned by the combined change's evidence record. External usefulness, review-time reduction and
independent interoperability remain `NOT_OBSERVED`; no exhaustive gate equivalence is claimed.

Native linked-adapter live qualification is `NOT_PRODUCED`: the proposed synthetic case anchor
was outside the owning extractor's admitted worklist after two construction repairs. Those failed
runs remain retained; no extractor rule was changed. The native fixture verifies unknown-only OCM
producer bytes, retained CEM inputs and wrong-base rejection. Exact native linked claims must be
qualified separately using the final integrated requirement-bearing tests and naturally valid OCM.

Rollback removes `--format`/`--ocm`, `RunWithOCM`, the report projection and its optional adapter,
and removes the projection appendix from Markdown. Existing maps and verifier semantics require
no migration. Failure to derive a bounded patch or read an explicitly requested OCM remains an
operational error; a semantic invalid verdict remains false with no qualified obligation joins.

## Named diagnostics

| Code | Meaning |
|---|---|
| `hunk-not-mapped` | A parsed patch hunk has no matching map entry; keep the hunk in the denominator with unknown disposition. |
| `map-hunk-not-in-derived-patch` | A map entry has no matching hunk in the original derived patch; expose it as surplus invalid map evidence. |
