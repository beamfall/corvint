# CEM stable 1.0

Intent status: owner scope accepted; technical contract accepted (decision 0546, 2026-10-20)
Delivery status: implemented and produced on main (decision 0525, CEM-V1-140..152, V1-1276); not promoted; qualification bounded

## Agent digest
- Claim: Owner requires portable task-linked execution evidence and all main runners for Corvint1.0; contract accepted (decision 0546), promotion open.
- Status: owner scope accepted; technical contract accepted (decision 0546, 2026-10-20); implemented and produced on main (decision 0525, CEM-V1-140..152, V1-1276); not promoted; qualification bounded.
- Exists: experimental runner profiles, native Tasks criterion binding and portable candidate proofs; hidden Core status/verify stable read (accepted by decision 0546 `CEM-V1-017..019`), hidden Core report/MCP preview (accepted by decision 0546 `CEM-V1-034..036`), hidden Core stable bundle export (accepted `CEM-V1-060..064`), explicit Core mutator refusals of `cem/1.0` (accepted by decision 0546 `CEM-V1-030..033`, narrowed to `cem/0.x` producers), the Core `cem/1.0` producer and dogfood loop cutover (accepted `CEM-V1-140..152`, V1-1276), the internal OCM/LRF stable universe admission (accepted `CEM-V1-051..059`, no stable LRF evaluation) and library-level stable witness, collapse, documentation and console projections (accepted `CEM-V1-070..079`); a provider-neutral plain criterion form verified by both consumers without a Tasks store (accepted by decision 0546 `CEM-V1-083`); DSSE Ed25519 runner receipt authentication that rises to VERIFIED only under a consumer `--trusted-keys` file (accepted by decision 0546 `CEM-V1-084`); optional stable map `provenance` and `role` members, a closed producer-only member list emitted empty, and the 0.1/0.3 algorithms re-homed as 1.0-normative `ALGORITHMS.md` text (accepted by decision 0546 `CEM-V1-093..095`); a deterministic cem/0.x to cem/1.0 map converter with provenance and full-corpus verdict-parity evidence (accepted by decision 0546 `CEM-V1-096..105`, satisfied by the frozen V1-1277 evidence; the converter no longer ships); the one-commit migration of every committed map to `cem/1.0` (accepted `CEM-V1-230..232`, V1-1277); the core 0.x retirement: a stable reader refusal of `cem/0.1..0.3` naming the merge-forward fix, a committed-corpus check, a `cem/1.0`-only sealed-constituent admission with a no-coverage rule for a retired seal, and the converter's removal (accepted by decision 0546 `CEM-V1-200..204`, V1-1278 S1); local-completion sidecar-only reuse of a committed `cem/1.0` sidecar on a stable verifier `ACCEPT` over its exact bytes with the absolute repository root and full object IDs, with closed refusal reasons and `CEM-V1-150`'s fail-closed default otherwise (accepted by decision 0546 `CEM-V1-205..209`, V1-1303); every consumer outside `internal/cem` reads `cem/1.0` only and refuses a retired map with `unsupported-spec`, with the frontier-v0, ocm-v0 and cli-parity-v0 freezes superseded under decision 0525 §6 (accepted by decision 0546 `CEM-V1-210..214`, V1-1278 S2a); the 0.x wire, verifier, producers and `cem` CLI routes, the `frontier/0`, `tcq/0`, `lrf/0` and `external-frontier-obligations/0` routes and the attest `cem/0`/`cem/v1` predicates are deleted, leaving one wire (accepted by decision 0546 `CEM-V1-215..219`, V1-1278 S2b); the `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`/`cem-0.2` packets, `examples/cem`, `tools/cem-interop-runner` and the portable consumer's 0.x routes are deleted, the inherited text moves into the self-contained `protocol/cem-1.0` packet and the 0.x specs are retired, completing decision 0525 §1 step 7 (accepted by decision 0546 `CEM-V1-220..225`, V1-1278 S3); one shared `cem/1.0` tamper matrix in the packet that the native and portable consumers both run with the same pinned verdicts (accepted by decision 0546 `CEM-V1-226..229`, V1-1340); `corvint-cem-anchor/1` anchor pointers that read `verified` only when the anchored map describes the commit's canonical patch, `unscoped` legacy `/0` pointers and a `--notes-tip` rollback read (accepted `CEM-V1-067`, V1-1260); the portable consumer's process containment qualified on Darwin and Linux amd64/arm64 through a pidfd/epoll keeper and a `/proc` quiet proof, with every other host still refusing `unsupported-process-containment` (accepted `CEM-V1-180..184`, V1-1358); truthful packet status, the citation rules, stop list, result serialization and diff recipe restated in the packet, and native-generated citation vectors both consumers run (accepted `CEM-V1-107..112`, V1-1333); a discrimination witness whose `mutants` exceeds `bounds.maxMutants` refused `invalid-field` by both consumers (accepted `CEM-V1-090`, V1-1259); a normative nested result member order with pinned whole result lines both consumers reproduce (accepted `CEM-V1-113`, V1-1369); the consumer's Git capability contract (`--attr-source`, Git 2.41 or later, else `repository` / `git-diff-failed`), its PATH-only Git resolution and the hosted `macos-15` qualification through the Tasks gate's own Git (accepted `CEM-V1-185..189`, V1-1377); `cem cite` refusing a governing instruction file the map's own patch changes with `cite-self-modified-authority` (accepted `CEM-V1-114`, V1-0414); `cem status` reporting each governing instruction file the patch changes as `selfModifiedAuthority`, advisory and verdict-neutral (accepted `CEM-V1-270`, V1-0414); `UNSUPPORTED` argument refusals with a stderr or envelope absolute-path fix and a pinned reference precedence, so both consumers return the same outcome, stage and code for the same input (accepted `CEM-V1-115..117`, `CEM-V1-176..177`, V1-1291/V1-1306/V1-1289); an opt-in consumer hunk-revert falsifier, `cem verify --hunk-revert`, that reports `held` beside the unchanged result only from its own `go test` re-execution (accepted `CEM-V1-088..089`, V1-1266; its disposable export refuses a repository-local `TMPDIR`, accepted `CEM-V1-240`, V1-1452); an opt-in, append-only verdict ledger under `refs/notes/corvint-verdicts` that `cem anchor --artifacts` extends with the native `verdictSha256`, refusing a differing verdict for the same map and commit with `verdict-ledger-conflict`, and a read-only `cem provenance --verdicts` listing (accepted under the owner's standing auto-accept (2026-10-11) `CEM-V1-271..279`, V1-1292); opt-in runner receipt admission in the plain producer and an end-to-end fixture chain from a native Tasks attempt through the runner, the producer and the native and portable verdict gate (`CEM-V1-280..287`, accepted under the owner's standing auto-accept (2026-10-11), V1-0632); an unadmitted plain runner receipt labelled `opaque/0`, read beside the legacy `corvint-test-runner-receipt/0` label by both consumers without rewriting any committed map (`CEM-V1-310..313`, accepted under the owner's standing auto-accept (2026-10-11), V1-2096); `cem status` and `cem verify` reading an absolute `--map` outside the repository, with maps inside it or behind a symlink refused and the map-path rules in help (accepted under the owner's standing auto-accept (2026-10-11), `CEM-V1-315..317`, V1-2097); a positive `cem/1.0` read for the dashboard's separate `cem-stable` document and the local-authority capsule's stable closure, each with stale and incomplete binding controls (accepted under the owner's standing auto-accept (2026-10-11), `CEM-V1-290..299`, V1-0632); a plain `assemble-stable` result whose `nextActions` names the `cem verify-stable` route over the bundle it wrote (accepted under the owner's standing auto-accept (2026-10-11), `CEM-V1-350..351`, V1-2105); native and portable qualification of the canonical repository envelope over real ordinary, origin-config, linked and SHA-256 checkouts and twelve adversarial shapes, with one native code divergence pending V1-2133 (accepted under the owner's standing auto-accept (2026-10-11), `CEM-V1-360..367`, V1-0595); a hunk-less binary file group bound by blob identity, reported by `cem prepare` as excluded rows and refused as a citation target (experimental, `CEM-V1-980..986`, V1-2752), now admitted by the portable consumer's stable routes under stable packet revision r2 with a shared native/portable parity case (experimental, `CEM-V1-934..936`, V1-3443); a native-only mode change beside textual hunks, base-mode checked and bound by the canonical-diff proof, with the portable consumer still refusing it (experimental, `CEM-V1-930..933`, V1-3404); qualification remains bounded.
- Blocked on: stable promotion (owner). Runner-tuple gaps and independent interoperability/outcome evidence (`CEM-V1-008`) ship in 1.0 as recorded limits (decision 0546).
- Read next: Owner context; Requirements; Acceptance evidence.

## Owner context

- Owner intent: accepted scope, 2026-10-01 conversation; CEM1.0 belongs in the main Corvint1.0 release and supports actual main test runners across supported languages, linked to native Tasks criteria.
- Technical contract: accepted by decision 0546 (2026-10-20). No version alias, consumer promotion or release qualification is established by this document; promotion stays with the owner.
- Native work: V1-0591; runner lanes V1-0592..0594; portable wire V1-0595; qualification V1-0596. Existing experimental CEX work remains V1-0575.
- Original evidence: `cem-criterion-experiments-v0.md`, `tasks-criterion-binding-v0.md`, `test-runner-execution-v0.md`; immutable Git CEM binding is governed by this contract and the inherited text under `protocol/cem-1.0/stable/inherited/`; `cem-0.2-canonical-binding.md` and `cem-0.3-structural-mechanical.md` are retired by decision 0525 (`CEM-V1-225`).

## Intended result

An agent and an independent reviewer can follow a changed Git hunk to its governing criterion, the exact test inventory executed, native attempts and report bytes, and any unresolved qualification. The evidence says what was observed and what remains unknown. A test failure does not alone demonstrate criterion discrimination, and receipt integrity does not establish semantic adequacy.

## Requirements

- `CEM-V1-001`: (accepted by decision 0546, 2026-10-20) Preserve canonical Git base/target/hunk binding and closed versioned decoding. Amended by decision 0525 §6 (V1-0595): `cem/0.1`, `cem/0.2` and `cem/0.3` documents are no longer readable; every consumer refuses them with `unsupported-spec` and the `CEM-V1-200` recovery line (`CEM-V1-200`, `CEM-V1-210`, `CEM-V1-215`), and Git history keeps them. Any other unknown spec stays an explicit unsupported result.
- `CEM-V1-002`: (accepted by decision 0546, 2026-10-20) Bind criterion references to canonical native ticket identity, acceptance revision, ordered acceptance digest and immutable claim-era record. Current applicability is distinct from historical validity.
- `CEM-V1-003`: (accepted by decision 0546, 2026-10-20) Reference bounded runner receipts by digest, with exact runner/profile, admitted source inventory, primary and auxiliary executable pins, native report hashes, expected inventory and observed attempts. Declare dependency closure and execution authority limits.
- `CEM-V1-004`: (accepted by decision 0546, 2026-10-20) Preserve FAILED, SKIPPED, FLAKY, interruption, timeout, build, collection and infrastructure outcomes. Unknown states, report contradiction, incomplete inventory or changed inputs produce uncertainty. Aggregate reports must not invent individual retries.
- `CEM-V1-005`: (accepted by decision 0546, 2026-10-20) Distinguish selected tests, executed tests, coverage and criterion discrimination. A mutant is killed only with an admitted criterion oracle and a classified, reproducible test failure; arbitrary nonzero exit or infrastructure failure cannot satisfy it.
- `CEM-V1-006`: (accepted by decision 0546, 2026-10-20) Integrate all main runners supported by Corvint's languages, including concrete browser, JVM, Apple and Android profiles. An import parser, static selector or compiler-only domain probe cannot satisfy execution qualification.
- `CEM-V1-007`: (accepted by decision 0546, 2026-10-20) Migrate Core producers/validators/reviewer reports, OCM/frontier/obligation consumers, companion consumers, portable Go consumer and public conformance packet as one evidenced compatibility contract. Inventory every unchanged or unsupported consumer explicitly.
- `CEM-V1-008`: (accepted by decision 0546, 2026-10-20) Retain independent external consumer conformance and matched external outcome evaluation separately from Corvint-authored fixtures and self-use. No self-authored consumer establishes independent adoption.
- `CEM-V1-009`: (accepted by decision 0546, 2026-10-20) Promote only with the native release's compatibility, focused documentation, integration, interoperability and required terminal release gates. Runtime/platform/toolchain gaps remain blockers in native Tasks and release readiness.
- `CEM-V1-010`: (accepted by decision 0546, 2026-10-20) Rollback disables new optional profiles and avoids rewriting native task history. Release promotion is an explicit operation. Amended by decision 0525 §6 (V1-0595): no rollback to a `cem/0.x` producer default or reader exists; the 0.x packets stay in Git history only, and reverting decision 0525's lanes by their merge commits is the only way back.
- `CEM-V1-344`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2109)
  `corvint help cem ACTION` MUST print the `CEM-V1-340` help of that registered action and exit 0.
  `verify-stable` and `gate-verdict` are registered actions: `corvint cem verify-stable --help` and
  `corvint cem gate-verdict --help` (or `-h`, after `--root PATH`, and after other options under the
  argparse scan of `GPK-V0-062`) MUST print that action's help and exit 0 before their closed argv
  parsers run. `gate-verdict` help is the Tasks gate section of `corvint cem --help`. A `--help`
  consumed as an option value is not a help request, so the closed parser still refuses it.
  This amends the `CEM-V1-340` non-goals.
- `CEM-V1-345`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2024)
  `cem provenance --commit REV --verdicts --verdicts-tip OID` MUST compare the pinned
  `refs/notes/corvint-verdicts` commit OID, which the caller recorded earlier, with the live ledger
  tip exactly as `--notes-tip` does for `refs/notes/corvint` (`CEM-V1-067`). The `verdicts` member
  gains `tip` (`ref`, `pinned`, `live`, `state`). The state is `current` when OID is the live tip
  or one of its ancestors and `rollback` otherwise, including when the live ref is absent. Under
  `rollback` the member's `state` MUST be `rollback` and `records` MUST be empty, whatever the live
  note holds. The ledger MUST be read from the live tip commit captured for the state, by object
  ID, never again through the mutable ref, so a ref moved away and back during the read cannot
  supply other content. A ledger ref that moves during the read MUST be refused with
  `provenance-notes-moved`. `--verdicts-tip` without `--verdicts`, or an OID that is not a full
  lowercase hexadecimal commit object ID, MUST be refused with `invalid-arguments`. The read
  writes nothing (`CEM-V1-277`). This amends `CEM-V1-279`: the provenance usage becomes
  `cem provenance --commit REV [--notes-tip OID] [--verdicts [--verdicts-tip OID]]`.
- `CEM-V1-346`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2024) Two
  separate processes anchoring the same map with `--artifacts` against the same ledger MUST leave
  exactly one writer's record. A writer whose validated ledger tip moved before its publish MUST
  be refused with `verdict-ledger-conflict`, MUST overwrite nothing and MUST leave no staging ref
  (`CEM-V1-275`, across processes).
- `CEM-V1-347`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1471) When the
  write, sync or close of a file the plain assembly creates (`CEM-V1-240`) fails, the assembly
  MUST remove that file before returning the failure, so no partial `stable.json` or artifact
  remains in the bundle.
- `CEM-V1-348`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1471) Named
  limit, bundle-parent check/open window: both the plain (`cem-stable-plain-assembly/1`) and the
  Tasks-bound (`cem-stable-assembly/1`) assembly check the bundle parent by pathname
  (`EvalSymlinks`, outside the repository) and then open it. A process that can write an ancestor
  of the bundle directory can swap a component between the check and the open, or rename the
  bundle after verification, so the files land somewhere other than the named path. Threat model:
  the caller owns the bundle parent; a hostile writer to that parent is out of scope. The bundle's
  identity is carried by the map's own digests, not its path, so a consumer verifying the bundle
  is not misled about its content. Closing the window (descriptor-relative opens of every
  component) is not done.
- `CEM-V1-349`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2034) Amends the
  dashboard refusal requirement CEM-V1-293. Every dashboard cem-stable refusal (`corvint-dashboard-cem-stable-refusal/0`)
  MUST carry `verifierStage`, one of `NOT_RUN`, `refused` or `accepted`. `NOT_RUN` means the
  refusal happened before the stable verifier ran and MUST carry `verificationReason`
  `refused-before-verification` with no `verification`. `refused` means the verifier refused and
  carries its result. `accepted` means the verifier accepted but a later admission step refused
  (an inexact excluded-artifact sidecar, an OCM/LRF phase failure, or a changed re-verification);
  the refusal MUST carry the verifier's ACCEPT result, and the refusal code stays the admission
  code. `lrfrepo.VerifyStableUniverse` reports these as `StableAdmissionRefusal`, which unwraps to
  the original error so `CodeOf` and every existing caller are unchanged.
- `CEM-V1-352`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1472) The
  portable legacy `cem01-go verify-stable` route keeps its declared narrower operational envelope
  on purpose: each base or target snapshot reads at most 4096 tree entries, 4 MiB of tree bytes,
  depth 128, a 1 MiB commit, and the whole
  verification at most 1024 Git operations; it refuses `repository` /
  `unsupported-resource-limit` above any of them, never a partial read. The two portable routes
  do not share one bound: repository-scale stable verification uses `verify-stable-canonical`
  under `canonical-repository-bounded/1`. The legacy route may be cited as an independent
  verifier only for repositories whose snapshots fit its envelope; for a larger repository, such
  as this one, its `UNSUPPORTED` answer is a bound, not a disagreement with the canonical
  route's verdict.
- `CEM-V1-242`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2136) The native
  names of the sealed repository-envelope recipe MUST exist in `internal/cem/verify`:
  `TestStableRepositoryEnvelopeOrdinary` runs the case array named by
  `CEM_STABLE_REPOSITORY_CASES` (closed fields `ID`, `Repository`, `Map`, `ExpectedBase`, `Target`,
  `ArtifactRoot`, `Expected`, `Exit`, `Env`; absolute paths; unique IDs) and compares each complete
  result and exit with the frozen expectation through an admitted runtime tuple. Unset, it
  materializes the packet's ordinary, origin, linked and relative cells itself.
  `TestStableRepositoryEnvelopeCancelled`, `...DescendantCleanup` and `...RootIdentity` run the
  matching public cases (at least 11, 3 and 1). The sealed document is not rewritten.
- `CEM-V1-243`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1329) The native
  and the portable tamper-matrix tests MUST carry the same table of one pinned `verdictSha256` per
  sealed `CEM-V1-226` case, MUST fail when the table and the sealed case list differ in size, and
  MUST fail when either consumer's recomputed digest differs from the pin. The sealed
  `EXPECTED.json` is unchanged.
- `CEM-V1-244`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2106) After the
  outside-repository check, an out-of-tree map read and an absolute-path report publish MUST open
  the checked parent directory as a held handle whose identity equals the one observed at the
  check, and MUST perform every member operation through that handle: the map read refuses a
  non-regular or oversized member and opens without following a final symlink; the report is
  written to an exclusive temporary member, synced, renamed and the directory synced. A changed
  parent identity MUST refuse (`map-unavailable` for the map, `publish-failed` for the report)
  with nothing read from or written to the swapped target.
- `CEM-V1-245`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1470) The
  portable op-timeout descendant test MUST retry with longer timeouts (500 ms, 2 s, 8 s, 30 s)
  while the timeout fires before the shim records its descendant, MUST still require
  `git-timeout` and no hold on every attempt, and MUST fail when no attempt records a descendant.
- `CEM-V1-820`: (experimental under the standing lane auto-accept; V1-1470 criterion 2) The portable
  consumer's direct Git backend (`git03` in `interop/cem01-go/cem.go`) MUST wait up to 5 s after its
  Git child exits for the output copy to finish (`exec.Cmd.WaitDelay`), not 250 ms. On a host at
  load about 60 on 12 CPUs, the copy goroutines did not finish inside 250 ms after a successful
  `git cat-file`, so `cmd.Run` returned `exec: WaitDelay expired before I/O complete` and the
  verifier answered `git-read-failed` (UNSUPPORTED) for a valid map; three citation tests failed
  that way under load and passed alone. The Git child itself stays bounded by the 10 s operation
  context and cancellation is unchanged; a descendant that keeps the output open longer than 5 s
  still fails `git-read-failed`. Falsifier: a Git child whose own descendant holds its stdout for
  1 s after a complete, successful output fails. Evidence: the `pipe-held-briefly` case of
  `Test03GitDirectChildControls`, and the three citation tests passing under that load. Rollback:
  restore 250 ms.
- `CEM-V1-568`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2133) When Git
  exits unsuccessfully deriving the canonical patch, the native reader MUST walk the change set for
  a missing object, and answer `repository-object-unavailable`, only in a promisor (partial-clone)
  repository, identified without running Git by a `*.promisor` pack marker in the common object
  store. In any other repository the failure MUST keep `git-diff-failed`, the sealed packet's cause
  provenance, so `verify.Stable` returns the packet's code and `verdictSha256` on
  `missing-target-blob` and `corrupt-target-blob`. A promisor repository still refuses without
  fetching (V1-0349, CEM-CB-019).

- `CEM-V1-037`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0301, decision
  0548) When the native parser (`internal/cem/patch`), and so `cem prepare`, refuses a
  `Binary files` or `GIT binary patch` marker, it MUST keep the code `binary-patch` and MUST name
  the repository path, taken from the unquoted symmetric `diff --git a/PATH b/PATH` line, else the
  `rename to` path, else the new side of an unquoted `Binary files a/OLD and b/NEW differ` marker
  (the old side when the new side is `/dev/null`). A marker that declares no valid path keeps the
  earlier generic message. No path is invented and no map is published. Amended by `CEM-V1-980`
  (V1-2752): the canonical full-index `Binary files` section is a binary file group bound by blob
  identity, and this refusal covers every other binary shape.
- `CEM-V1-038`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2018) In the
  `CEM-V1-286` chain, the ticket MUST require the `cem-verdict` gate, and a `GRANT` recorded for one
  submitted tree MUST NOT be treated as current once another tree is submitted:
  - Tasks refuses completion at the superseded seal commit with `STALE_TREE`.
  - Tasks refuses completion at the new candidate with `GATE_STALE`, and after a failed verdict
    there with `GATE_FAILED`.
  - After the sealed tree is restored and resubmitted, completion is still refused with
    `GATE_STALE` until the gate runs again at that candidate. Its fresh `GRANT` then completes the
    ticket.
- `CEM-V1-039`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2018) Runner
  repairs MUST compose only on one bound source. When two isolated repairs of one draft each have a
  real passing `corvint-test-runner` receipt over a source that the composed target does not
  contain, the plain producer with admission MUST refuse each receipt with `runner-receipt-stale`
  and write no bundle. It MUST admit the one receipt whose plan pins the composed target's inputs
  and selects both repaired tests. That receipt observes both tests `PASSED`, and the bundle binds
  the runner's exact bytes.
- `CEM-V1-368`: The `cem-stable-review/0` projection MUST copy, from the verifier's own `citation`,
  each hunk row's `tier` and `anchors`, and a top-level `citationTiers` with the per-tier counts, the
  hunk count and `samePathCitations` (`NOT_PRODUCED` when the result has no citation). Each evidence
  row MUST carry a `location` with the drift entry's `path` and `targetSpan` (byte offsets, or null),
  and each hunk row a `selfOnly` flag that is true when every cited evidence path equals the hunk's
  path. Rows MUST be ordered weakest support first: `unknown` or `NOT_PRODUCED` tier, then
  self-only, then `cited` with no anchor, then the rest, map order breaking ties, each row keeping its
  map `ordinal`. The Markdown report renders a `Citation tiers` line and, per row, the tier, the
  anchors (or `none`), `self-only citation`, and `path bytes start-end` (or `bytes NOT_LOCATED`).

- `CEM-V1-369`: (accepted by the owner in chat on 2026-10-21, decision 0551; V1-2327) The structural
  predicates MUST be qualified once, on the qualified tuple `go1.27.1`, by the structural corpus
  sealed under `protocol/cem-1.0/structural-corpus/`:
  - `CORPUS.json` names the corpus: every tracked `.go` file of the Corvint tree at one pinned
    commit and its `internal/cem/verify/testdata/structural` fixtures, by path-list digest and
    count, plus the version-sensitive `cases/`. A zero-byte case is declared there and observed as
    zero bytes instead of committed, because an empty-file creation is not representable
    (`CEM-V1-300`). It also names the five derived perturbation kinds (`formatter-only`,
    `import-reorder`, `move`, `rename`, `partial-rename`) and the rule that a `*.crlf.go.txt` case
    is observed with every LF widened to CRLF.
  - `CASES.observations.jsonl` holds the portable consumer's observations of `cases/` on
    `go1.27.1`: per file the source, scanner, parser, printer and formatter digests and format
    idempotence, and per pair its image segments and the four predicates' verdicts in both
    directions.
  - `QUALIFICATION.json` records, per host and toolchain, the observation digest of both sets, the
    native/portable parity result, the parser-closure digests and what was not run.
  - `manifest.json` binds every file by SHA-256.

  Qualification holds only when the portable observations of both sets on `go1.27.1` are
  byte-identical on darwin/arm64 and linux/arm64, and the native verifier reaches the portable
  verdicts on every recorded pair on each host where it ran. `TestStructuralCorpusCases`
  (portable) and `TestStructuralCorpusCasesParity` (native) MUST re-check the committed cases on
  every admitted tuple. A rejected pair is an observation, not a failure; parity and host identity
  are what qualify. A `go1.24.13` run is recorded as the comparison that a release with another
  closure is detected; it is not admitted. Windows stays NOT_RUN until a Windows host runs the
  corpus. Rollback: revert the packet and `CEM-V1-371`; the label returns to
  `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`.
- `CEM-V1-370`: (accepted by the owner in chat on 2026-10-21, decision 0551; V1-2327) A Go release
  MAY join `structuralTuples` without a corpus run only when its parser-closure digest
  (`tools/cem-structural-closure`, profile `cem-structural-parser-closure/1`) equals the qualified
  tuple's. The closure is the union of `go list -deps` over `go/ast`, `go/format`, `go/parser`,
  `go/printer`, `go/scanner` and `go/token` for darwin/arm64, linux/amd64, linux/arm64 and
  windows/amd64, minus the runtime, scheduler, system-call, file-system and GODEBUG packages the
  tool lists. Their behaviour is outside what the predicates observe, so a difference there is not a
  qualification input. The digest covers every regular non-test file directly in each remaining
  package directory. The release's tuple packet MUST record the digest, and
  `TestRunningToolchainClosure` MUST fail when an admitted toolchain's live closure, or any tuple
  packet, differs from the qualified digest. A release with any other digest MUST requalify on the
  full corpus first. Rollback removes the release from both lists and its packet.
- `CEM-V1-371`: (accepted by the owner in chat on 2026-10-21, decision 0551; V1-2327) Both stable
  verifiers MUST set `runtime.structuralQualification` to `QUALIFIED_EXACT_TUPLE` on an admitted
  tuple and keep `UNSUPPORTED` and `NOT_REQUIRED` unchanged. Every `expectedResult.runtime` of the
  frozen packet and its envelope cells MUST be re-emitted through `make cem-stable-regen-replace`
  (`CEM-V1-091`), keeping `goVersion` `go1.27.1`. The frozen `cancelledContextExpectedResult`
  snapshot of the cancel-before-content cell is not a regenerated value and keeps the old label;
  no consumer reads it. Rollback is a Git revert of the re-emission and the three emission sites.
- `CEM-V1-569`: (proposed; V1-2323) `companionrelease.ValidateStableReleaseEvidence` MUST read
  its `Result` bytes as one exact `cem-stable-verification/1` result. It requires the strict CEM
  JSON profile with signed integers (`wire.ParseSigned`, since `citation.specificity` is a signed
  integer in `RESULT.schema.json`), the 23 members of `RESULT.schema.json` in their normative order
  (`CEM-V1-113`), no unknown member in any typed nested record, closed values for `outcome`,
  `stage`, `sidecar`, `assurance`, `runtime.structuralQualification` and the five integrity axes,
  exactly the 18 named axes, the four reference members each an array, and a `verdictSha256` that
  the other members recompute (`CEM-V1-066`). The verdict is recomputed from the parsed bytes, not
  from the typed decoding, and the typed decoding MUST re-encode to the same canonical value, so a
  `null` or a value of the wrong type that typed decoding would coerce is refused rather than
  normalized. Anything else is refused with class `stable-result-schema`. A
  result whose `profile`, `spec` or `verificationMode` is not `cem-stable-verification/1`,
  `cem/1.0` and `canonical-and-reference-integrity` is refused with class `stable-result-mode`.
- `CEM-V1-570`: (proposed; V1-2323) The reader MUST refuse with class `stable-result-authority`
  any authority, history, source, execution, receipt-semantics, dependency, adequacy,
  discrimination or external-interoperability axis other than `NOT_OBSERVED`. `authentication` may
  also be `UNTRUSTED_KEY`, `VERIFIED` or `FAILED` (`CEM-V1-084`). It MUST refuse with class
  `stable-result-unsupported-admission` an `UNSUPPORTED` outcome, a structural qualification other
  than `NOT_REQUIRED` or `QUALIFIED_EXACT_TUPLE`, a `QUALIFIED_EXACT_TUPLE` whose
  `runtime.goVersion` is not an admitted structural tuple (`CEM-V1-050`), a repository envelope
  other than `canonical-repository-bounded/1`, and `structuralProof` `UNSUPPORTED_RUNTIME`. It MUST
  refuse with class `stable-result-not-accepted` anything but an `ACCEPT` with `accept` true, stage
  `complete`, a null code, no issue codes, non-null assurance, `changeIntegrity` `VERIFIED`,
  authentication not `FAILED`, `referenceIntegrity` `REFERENCE_INTEGRITY_ONLY` or
  `EMPTY_REFERENCE_SET`, `sidecar` `ABSENT` or `EXACT`, and `structuralProof` `NOT_REQUIRED` or
  `PROVEN_DECLARED_FILE_PREDICATES`, which `QUALIFIED_EXACT_TUPLE` requires. A result that
  contradicts what a verifier can emit on an accept is refused even when its verdict recomputes.
  These checks run before the release outcomes, and an admission refusal
  holds even when reference integrity passes. Neither `accept` true nor the binary version
  establishes release support.
- `CEM-V1-571`: (proposed; V1-2323) Runner, platform and external outcomes are release evidence
  that the stable result never carries, so the caller supplies each class separately as
  `StableReleaseOutcomes{Required, Observed}`. The reader MUST refuse with class
  `release-runner-incomplete`, `release-platform-incomplete` or `release-external-incomplete`
  when a class declares no required rows, repeats or leaves empty a required name, or has a
  required row that is unobserved or observed as anything other than `PASSED` (for example
  `FAILED`, `SKIPPED` or `NOT_OBSERVED`). This holds even when the stable result is an admitted
  `ACCEPT` with reference integrity. Every refusal is a `*StableReleaseRefusal` whose text starts
  with its class.
- `CEM-V1-572`: (proposed; V1-2323) `make cem-release-suites CORVINT_RELEASE_BIN=... CEM01_RELEASE_BIN=...`
  (`tools/cem-release-suites`) MUST refuse unless both binaries carry Go build information with
  the expected main package (`cmd/corvint`, `interop/cem01-go`), a Git `vcs.revision`,
  `vcs.modified=false`, and the same revision, and the checkout is at that revision. Build stamps
  are unauthenticated, so it MUST report that revision as reported by the binaries, never as
  verified provenance, and pin each binary by the SHA-256 of its bytes. Git runs without any
  inherited `GIT_*` variable. Every suite input is written from the revision's committed tree
  objects into a private directory, so worktree edits, `skip-worktree` and `assume-unchanged`
  entries cannot reach a suite; a tree entry that is not a regular file is refused. It then runs
  the tamper matrix, citation vectors, neutral criterion and receipt authentication cases through
  `corvint cem verify-stable` and `cem01-go verify-stable-canonical`. Each result is checked
  against its sealed expectation, including the sealed `results/accept.json` line for the neutral
  accept, and the two binaries' result lines must be byte-identical apart from
  `runtime.goVersion`; `runtime.structuralQualification` is compared. Each `runtime` must be
  exactly `goVersion` and `structuralQualification`, and a qualification must agree with
  `structuralProof`. The repository-envelope cells are prose S0E recipes, so the tool runs them at
  source level in the same committed tree, through the native and portable S0E, envelope-cell and
  adversarial-shape tests, and reports them as not run against the binaries.
- `CEM-V1-600`: (proposed; V1-2323) `companionrelease.ReadStableReleaseDeclaration` MUST read a
  release-readiness declaration as one exact `cem-release-readiness/1` document: at most 64 KiB of
  UTF-8 strict CEM JSON (duplicate keys refused) holding exactly `profile`, `runner`, `platform`
  and `external`, with these exact names in this order, one value and nothing after it. A
  differently cased name is refused rather than merged. Each class MUST declare 1 to 64 distinct row
  names of 1 to 128 bytes from `[A-Za-z0-9._/:-]`. Anything else is refused with class
  `release-declaration`. The committed declaration `tools/cem-release-suites/READINESS.json`
  requires the five sealed suites on both release binaries, the source-level envelope run,
  `darwin/arm64` and `linux/amd64`, and one independent external consumer. The declaration names
  what must be observed; it observes nothing.
- `CEM-V1-601`: (proposed; V1-2323) `companionrelease.StableReleaseReadiness` MUST refuse with
  class `stable-result-not-accepted` a release run that produced no accepted stable result. It
  MUST read every accepted result through the `CEM-V1-569` and `CEM-V1-570` reader before any
  outcome, so a malformed later result is never hidden behind a missing row. It then applies
  `CEM-V1-571` to the declared runner, platform and external rows.
- `CEM-V1-602`: (proposed; V1-2323) `tools/cem-release-suites --readiness PATH` MUST collect
  every result line either binary emitted with `accept` true, and observe as `PASSED` only rows it
  actually ran: each binary suite on each binary after its sealed expectation held, and
  `source:repository-envelope` after the source-level envelope tests passed. The binary-level
  `corvint:repository-envelope` and `cem01-go:repository-envelope` rows MUST be reported
  `NOT_RUN`. The platform row is the `GOOS/GOARCH` both binaries' build information reports, and
  no platform is observed when they disagree or omit it. No external row is ever observed by this
  tool.
- `CEM-V1-603`: (proposed; V1-2323) The `--readiness` path MUST be repository-relative and is
  read from the committed tree at the binaries' revision (`CEM-V1-572`), not the worktree; a bad
  path exits 2 and a bad declaration exits 1. The tool MUST print one
  `cem-release-suites: readiness: <class> <row> <outcome>` line per declared row, `NOT_OBSERVED`
  for an unobserved row, then either `readiness: READY (...)` with exit 0 or
  `readiness: REFUSED (...): <refusal>` with exit 3. Exit 3 after every suite passed is the honest
  verdict while any declared row is unobserved (`CEM-V1-008`), and READY never implies promotion.
- `CEM-V1-604`: (proposed; V1-2323) `MIGRATION.json` addition A02 MUST leave `OPEN` with a status
  starting `IMPLEMENTED_EXPERIMENTAL: `, naming `make cem-release-gate` and ending
  `not promoted.`, and the sealed stable manifest MUST pin the new `MIGRATION.json` bytes
  (`make cem-stable-regen-check`). No other migration row, sealed case or verifier changes.
- `CEM-V1-605`: (proposed; V1-2323) `make cem-release-gate` MUST refuse a checkout with any
  change or untracked file (Go stamps `vcs.modified` from untracked files too), run the
  independent Node check `tools/cem-release-suites/readiness.test.mjs` (declaration shape and
  suite coverage, A02 out of `OPEN`, and the manifest pin recomputed with `node:crypto`), build
  `corvint` and `cem01-go` with `-trimpath` from that HEAD into a private directory that is
  removed on exit, and run `tools/cem-release-suites` over them with the committed declaration.
  It is opt-in and is not a `make gate` step; wiring it into a release workflow is an owner
  `.github/` change.

- `CEM-V1-606`: (proposed; V1-2577) The sealed verdict gate (`corvint cem gate-verdict`, TCB-V0-011)
  MUST satisfy a governed `cited` hunk on the `runner-input` basis only when all of these hold:
  the candidate commit has a regular-file post-image at the hunk path; a `criterionLinks` row lists
  the hunk and names at least one runner receipt; its criterion is the plain form and that
  criterion's `evidenceId` is in the hunk's basis; every receipt the link names is a
  `runnerReceipts` row with profile `corvint-test-runner-receipt/0` whose `runner-plan` and
  `runner-receipt` artifacts are read from the candidate commit's objects with exactly the digests
  the row binds, decode as closed runner documents carrying the plan profile
  `corvint-test-runner-plan/0` and the receipt profile `corvint-test-runner-receipt/0`, bind the
  receipt to that plan by plan identity,
  and record a complete run with no problem, no error and at least one test, every one `PASSED`;
  and one of those plans lists the hunk path in `inputFiles` with the SHA-256 of the candidate
  blob. The hunk's recomputed tier is reported unchanged; `anchor` names the basis.
- `CEM-V1-607`: (proposed; V1-2577) The gate MUST satisfy a governed `cited` hunk on the
  `runner-artifact` basis only when its path is a map `artifacts` row of kind `runner-plan` or
  `runner-receipt`, the candidate blob at the path has that row's SHA-256, and at least one
  `runnerReceipts` row that binds that digest (`sha256` for a receipt, `planSha256` for a plan)
  has a run satisfying the run conditions of CEM-V1-606. A failed run bound by one row never hides
  a passing run of the same plan bound by another.
- `CEM-V1-608`: (proposed; V1-2577) An unsatisfied governed hunk MUST name one closed `reason`:
  `uncited` (tier below `cited`), `no-post-image` (no regular file at the path in the candidate),
  `no-runner-binding`, `runner-not-passed`, `runner-artifact-mismatch` or `runner-input-mismatch`,
  the most specific observed; the gate still refuses `CEM_VERDICT_PREDICATE_FAILED` /
  `tier-below-anchored`. A path row is satisfied only when every governed hunk at that path is. The
  `anchor` and `reason` members are omitted when empty, so a hunk satisfied by its tier encodes
  as before. Runner evidence is read only when a governed hunk is below `anchored`; a failed Git
  read refuses `CEM_VERDICT_UNVERIFIABLE` / `git-read-failed` with no governed rows. The citation
  block, the verdict digest and the portable cross-check are unchanged.

- `CEM-V1-609`: (proposed; V1-2578) `make cem-release-gate` MUST run the 29 public
  repository-envelope cells once per built binary: the portable consumer's
  `TestStableCanonicalEnvelopeCells` recipes in the committed private tree, with
  `CEM01_ENVELOPE_BINARY` naming that binary's absolute path and stable verification command
  (`corvint cem verify-stable`, `cem01-go verify-stable-canonical`) as each cell's verifier, and
  `CEM01_S0E_CONFORMANCE_OUT` naming a report in the run's private directory. Inherited Git and
  envelope selection variables are dropped, from the source-level suites as well. A malformed `CEM01_ENVELOPE_BINARY` fails the test and
  never falls back to the test binary; unset, the cells keep running the test binary in `RUN_REAL`
  mode.
- `CEM-V1-610`: (proposed; V1-2578) The `corvint:repository-envelope` and
  `cem01-go:repository-envelope` rows MUST be `PASSED` only when that binary's report is
  `cem01-go-s0e-portable-conformance/1` with 29 distinct cells, every one `RUN_BINARY` `PASS`;
  `FAILED` when the test run failed, the report is unreadable or foreign, or any recorded cell did
  not pass; and otherwise `NOT_RUN` (cells this host could not build). A `FAILED` row refuses the
  run with exit 1, whatever the declaration requires.
- `CEM-V1-611`: (proposed; V1-2579) `--record PATH` MUST write the run's `cem-release-run/1`
  record, the canonical `encoding/json` form plus LF, with members `profile`, `revision`,
  `platform`, `declaration` (`path`, `sha256` of the judged declaration bytes), `binaries`
  (exactly `corvint` then `cem01-go`, each with its main package and SHA-256), `runner` and
  `platforms` (the rows observed) and `accepted` (the accepted stable result lines). The record is
  unauthenticated run output; it never proves who ran it or how the binaries were built. A reader
  MUST refuse unknown members, a trailing value and any non-canonical spelling. An existing
  output path is refused before the build and never overwritten.
- `CEM-V1-612`: (proposed; V1-2579) `--aggregate --readiness PATH [--root DIR] [--outcome OUT]
  RECORD...` MUST read the declaration from the first record's revision's committed objects at
  `--root`, never from a record, and MUST refuse the record set with exit 1, writing no outcome,
  unless every record has the run profile, that revision and exactly that declaration path and
  digest; names exactly the `corvint` and `cem01-go` binaries with digests; observes exactly its
  own platform `PASSED`; declares the same runner rows; and carries at least one accepted result.
  No platform and no binary digest may appear in two records. The merged runner row is `PASSED`
  only when every record observed it `PASSED`, and otherwise the first other outcome in record
  order; platforms are the union and accepted lines are concatenated; readiness is then judged as
  in CEM-V1-601 with every external row `NOT_OBSERVED`. The CI-scope `--ci-unobservable` flag of
  AFP-V0-432 changes only that exit, never the judgment or the outcome; without it the aggregate
  is strict.
- `CEM-V1-613`: (proposed; V1-2579, V1-2581) `--outcome PATH` MUST write one
  `cem-release-gate-outcome/1` document when readiness was judged, in either mode:
  `profile`, `revision` (a full 40- or 64-hex object name), `declarationSha256`, `records` (the
  distinct digests of the records the run wrote or aggregated, `[]` when it wrote none), `verdict` and `reason`, in
  that order, LF terminated. `READY` carries an empty reason and `REFUSED` a non-empty one.
  `companionrelease.ReadStableGateOutcome` MUST admit only that canonical encoding and refuse
  everything else with class `release-gate-outcome`. A run refused before readiness is judged
  writes no outcome, so a missing outcome never reads as a verdict.
- `CEM-V1-614`: (proposed; V1-2581) The stable readiness record (SRR-V1-005) MUST end its
  catalogue with the row `gate/cem-release-gate` for every version except `1.0.0-rc.1` and
  `1.0.0-rc.2`, whose records stay verifiable unchanged. The row MUST be `PASS`, and its evidence
  file MUST read as a gate outcome whose `revision` is the candidate commit and whose verdict is
  `READY`, at build and again at verification. A missing outcome, a `REFUSED` one, one for another
  commit, a malformed one or a non-PASS status refuses the candidate.

- `CEM-V1-573`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2322) A
  `cem-stable-plain-assembly/1` request MAY name `runners`, an array of 1 to 64
  `{runnerPlan, runnerReceipt}` pairs of pinned inputs, in place of the top-level `runnerPlan` and
  `runnerReceipt`. In that form every criterion MUST also name `runnerReceiptSha256s`, the 1 to 32
  (the reader's link array bound) receipt digests of the pairs that cover it, and the request MUST carry the `runnerAdmission` of
  `CEM-V1-280`, which the producer applies to each pair in order. The map carries one runner
  receipt record per pair, in pair order, labelled `corvint-test-runner-receipt/0`; pair `i`
  (0-based) writes `artifacts/runner-plan-<i>.json` and `artifacts/runner-receipt-<i>.json`; and
  each criterion link names exactly that criterion's `runnerReceiptSha256s`, in request order.
  Every other rule of `CEM-V1-240` and `CEM-V1-280..285` applies to each pair unchanged.
- `CEM-V1-574`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2322) A request
  in the single form MUST yield the same map, artifacts and result bytes as before `CEM-V1-573`,
  admitted or not.
- `CEM-V1-575`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2322) Before
  any input is read, the producer MUST refuse, writing no bundle:
  `runner-set-form` for a request that names members of both forms, a single-form criterion
  naming `runnerReceiptSha256s`, or a `runners` criterion without it, where a member present in
  the request JSON counts as named even when it is `null` or an empty pin; `runner-admission-required` for
  `runners` without `runnerAdmission`; `runner-set-bound` for an empty `runners` or one over 64
  pairs, and for a criterion list that is empty or over 32; `runner-set-duplicate` for a plan or
  receipt digest or path named twice across the pairs, or a digest repeated in one criterion list;
  `runner-set-unresolved` for a criterion digest that is not a pair's receipt; and
  `runner-set-unused` for a pair receipt that covers no criterion.
- `CEM-V1-576`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2322) After
  every pair is admitted and before the bundle is created, the producer MUST refuse with
  `runner-profile-conflict` a criterion covered by two receipts of the same runner profile.
  Receipts of different profiles MAY cover one criterion.
- `CEM-V1-577`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2322) The
  native (`internal/cem/verify`) and portable (`interop/cem01-go`) readers MUST ACCEPT the
  two-profile case `interop/cem01-go/testdata/multi-runner/` (a `go-test` and a `node-test`
  receipt), which the producer MUST reproduce byte for byte from its recipe repository. Both MUST
  REJECT its six tampered variants with the same stage and code: a dropped receipt artifact or an
  unpinned plan (`references` / `artifact-role-closure`), swapped receipt artifacts (`artifacts` /
  `artifact-digest-mismatch`), a dropped receipt (`references` / `unresolved-reference`), a
  duplicated receipt (`references` / `duplicate-reference`), and an unlinked receipt
  (`references` / `reference-coherence`).
- `CEM-V1-578`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2322) The
  Tasks-bound `cem-stable-assembly/1` request is unchanged. It writes no stable map (its source
  check refuses every source), so it has no runner set to widen, and its closed decoder MUST keep
  refusing a `runners` member as an unknown field.
- `CEM-V1-620`: (proposed; V1-2107) A string `const` or `enum` node of both `RESULT.schema.json`
  copies MAY be widened only by `make cem-stable-regen-widen RECORD=<file>` (`cem-stable-regen
  --widen --record`). The closed record names a reason, the RFC 6901 pointer of the node (object
  members only), the SHA-256 of the node's compact JSON and the values to add. The tool MUST refuse
  before any write: an unknown record member, an empty reason, a root or array-crossing pointer, a
  missing member, a node that is not exactly `{"const": STRING}` or `{"enum": [STRING, ...]}`, a
  predecessor digest that differs, no value, and an empty, repeated or already admitted value. It
  MUST rewrite the node as an `enum` of the old values in order followed by the added values,
  change no other schema byte, case, cell, exit or key count, and re-bind the same chain as
  `CEM-V1-091` with packet status `SCHEMA_ENUM_WIDENED`, a once-only README pointer and a
  `CHANGES.md` section with the reason, the record digest, the old and new admitted values and the
  predecessor and new node digests. `make cem-stable-regen-check` MUST pass before and after.
  Failure modes: a widening that also needs a structural change (a new member, a new `oneOf`
  branch) is out of scope and refused at the node-shape check; a new `anyOf` branch uses
  `CEM-V1-880` instead. Rollback is a Git revert of the regenerated files.
- `CEM-V1-621`: (proposed; V1-2107) Both `RESULT.schema.json` copies MUST admit exactly the two
  `CEM-V1-311` labels at `$defs/runnerReceipt/properties/profile`, `corvint-test-runner-receipt/0`
  and `opaque/0`, produced by one `CEM-V1-620` run from the committed record
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-2107/widen-record.json`. The legacy label stays admitted, so no
  sealed map, public case result or verdict digest changes. Rollback is a Git revert of that run.
- `CEM-V1-622`: (proposed; V1-2107) A test MUST validate the complete native verify result of the
  public `neutral-criterion` case, once with its runner label and once relabelled `opaque/0` (the
  label of an unadmitted map), against both committed schema copies with a closed draft 2020-12
  validator that refuses every keyword it does not implement. The label `opaque/1` and the
  predecessor legacy `const` MUST refuse at `references.runnerReceipts[0].profile`. (Amended
  2026-10-31 by V1-2615: the open limit is closed. Each result MUST validate against the
  committed copies as they are, with no overlaid definition, because `CEM-V1-881` admits the
  `CEM-V1-083` plain criterion binding these results carry.)
- `CEM-V1-880`: (proposed; V1-2615) An object-schema node of both `RESULT.schema.json` copies MAY
  gain one alternative only by `make cem-stable-regen-any-of RECORD=<file>` (`cem-stable-regen
  --any-of --record`). The closed record names a reason, the RFC 6901 pointer of the node (object
  members only), the SHA-256 of the node's compact JSON and the branch, a JSON object schema. The
  tool MUST refuse before any write: an unknown record member, an empty reason, a root or
  array-crossing pointer, a missing member, a pointer whose last member is a map of schemas
  (`$defs`, `definitions`, `properties`, `patternProperties` or `dependentSchemas`, since wrapping
  that map breaks the references into it), a node that is not an object, an `anyOf` node with any
  other keyword, a predecessor digest that differs, a branch that is not a non-empty object, and a
  branch whose compact JSON equals an existing alternative. It MUST rewrite a node that is not yet
  exactly `{"anyOf": [...]}` as `{"anyOf": [OLD, BRANCH]}` with the old bytes kept and only
  re-indented, append the branch to an existing `anyOf`, keep the record's member order in the
  branch, change no other schema byte, case, cell, exit or key count, and re-bind the same chain as
  `CEM-V1-091` with packet status `SCHEMA_ANYOF_BRANCH_ADDED`, a once-only README pointer and a
  `CHANGES.md` section with the reason, the record digest and the predecessor, branch and new
  node digests. `make cem-stable-regen-check` MUST pass before and after. Failure modes: an
  alternative only widens what the schema admits, so no frozen case or verdict changes; a branch
  that should replace or narrow the node is out of scope. Rollback is a Git revert of the
  regenerated files.
- `CEM-V1-881`: (proposed; V1-2615) Both `RESULT.schema.json` copies MUST admit, at
  `$defs/criterionBinding`, `anyOf` the sealed Tasks form unchanged and the `CEM-V1-083` plain form:
  an object with exactly `id` (`$defs/criterionId`), `criterionIndex` (integer 0..255),
  `criterionSha256` (`$defs/sha256`) and `evidenceId` (`^evidence:sha256:[0-9a-f]{64}$`), all
  required and no other member. They are produced by one `CEM-V1-880` run from the committed record
  `tools/cem-stable-regen/testdata/v1-2615-any-of-record.json`. The Tasks form stays admitted,
  so no sealed map, public case result or verdict digest changes. The external form
  (`CEM-V1-330`) is not added here. Rollback is a Git revert of that run.
- `CEM-V1-633`: (proposed; V1-2599) Neither stable reader's verdict MAY depend on a runner
  receipt's recorded test states or on a runner plan's selection. Both readers MUST ACCEPT, with
  `verificationMode` `canonical-and-reference-integrity`, `referenceIntegrity`
  `REFERENCE_INTEGRITY_ONLY` and `runnerReceiptSemantics`, `runnerExecution`, `executionAtCommit`,
  `sourceInventoryCompleteness`, `criterionAdequacy` and `criterionDiscrimination` `NOT_OBSERVED`:
  a map citing a receipt that records a `FAILED` test, under either receipt label; a map whose
  plan selects fewer tests than the target defines; and a map from which a run was left out whole
  (its receipt row, artifact and link digest), whose bytes equal a map with no such run. A partial
  omission MUST refuse at stage `references`: a receipt row a criterion still names with
  `unresolved-reference`, a link digest left out with `reference-coherence`, a receipt artifact
  entry left out with `artifact-role-closure`. Without `runnerAdmission` the plain producer keeps
  a receipt that records a `FAILED` test as an `opaque/0` reference; with admission it refuses
  `runner-receipt-failed` (`CEM-V1-284`, `CEM-V1-286`). This is a documented limit: outcome,
  selection and completeness are not judged by a stable reader (stable `ALGORITHMS.md`,
  `CEM-V1-240`, `CEM-V1-570`).
- `CEM-V1-634`: (accepted by the owner (2026-10-25; V1-2658 sign-off); V1-2599, V1-2658) No
  `cem/1.0` consumer compares a runner plan's selection with `corvint affected` or with the
  target's test inventory, and a map records no affected plan. The sealed verdict gate's
  runner-input basis (`CEM-V1-606`) MUST refuse a hunk whose linked receipt records a `FAILED`
  selected test, or whose link names a failed rerun, with `runner-not-passed`. It still satisfies
  the same hunk once the failing test is left out of the plan's `expectedTests`, or the failed run
  is left out of the map whole (its receipt row, artifact and link digest); no refusal is added
  for a plan narrower than the governed package's tests. Whole omission cannot be detected: the
  map bytes equal those of a map that never had the run. So whenever a governed hunk is satisfied
  on `runner-input`, whatever the outcome, the gate document MUST carry the top-level member
  `uncertainty`, exactly `{"runnerSelectionCompleteness":"NOT_OBSERVED",
  "sourceInventoryCompleteness":"NOT_OBSERVED"}`: the plan's selection and the map's run
  inventory were not observed to be complete. That one statement covers the narrowed plan and
  the whole omission (`CEM-V1-004`, `CEM-V1-005`, AGENTS.md invariant 2). GRANT and REFUSE
  outcomes, codes, reasons and governed rows are unchanged.
- `CEM-V1-635`: (accepted by the owner (2026-10-25; V1-2658 sign-off); V1-2658) The gate MUST
  omit `uncertainty` when no governed hunk is satisfied on `runner-input`, so a document whose
  hunks are satisfied only by tier or on `runner-artifact` encodes byte for byte as before; the
  profile stays `taskman-cem-verdict-gate/0` under the omitted-when-empty rule `CEM-V1-608` set
  for `anchor` and `reason`, and TCB-V0-012 is amended by cross-reference to admit the member.
  The other bases make no completeness claim, because the gap does not apply to them: a tier is
  recomputed from citation spans and reads no runner evidence, and `runner-artifact` claims only
  that the governed bytes are a bound runner plan or receipt with a passing run. Those bytes are
  the selection itself, so a narrower selection is a different artifact, and a run left out
  whole leaves no artifact path to govern. Under TCB-V0-013 a pinned document with a
  `runner-input` satisfaction from before this rule lacks the member, so it no longer equals the
  recomputation and refuses `CEM_VERDICT_PRODUCER_MISMATCH`; this is the intended refusal. Every
  other pinned document still reverifies.
- `CEM-V1-636`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798)
  A `cem status` or `cem report` read with `--max-unknown` or `--max-mechanical` MUST add the
  envelope member `policyState`, `within-ceilings` or `ceiling-exceeded`, beside `state`. `state`
  stays the verifier's structural verdict (`accepted`, `rejected`, `unsupported`; `CEM-V1-017`),
  so `state: accepted` with `ok: false` reads as accepted by the verifier and refused by policy.
  A read without ceilings carries no `policyState`. Exit codes are unchanged.
- `CEM-V1-637`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798)
  `cem report` over a `cem/1.0` map MUST admit `--max-unknown` and `--max-mechanical` in every
  format and apply them exactly as `cem status` does (`CEM-V1-145`): counts, `policyIssues`,
  `policyState`, `ok: false` and exit 1 when a ceiling is exceeded, with `stableVerification`
  unmodified. A Markdown report still publishes the full review. The ceilings apply before an
  `--ocm` combination, which can only lower `ok`. `cem verify` and the MCP report preview keep
  refusing ceilings with `invalid-arguments`.
- `CEM-V1-638`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798)
  `corvint prove --cem MAP --attest-cem-v2` MUST admit `--max-unknown N` and `--max-mechanical N`
  (non-negative integers, each at most once, only with `--attest-cem-v2`). After the stable
  verifier accepts, an exceeded ceiling MUST refuse `attest-cem-v2-policy-exceeded`, naming each
  exceeded count, with exit 2 and no statement on stdout. Ceilings the map meets leave the
  statement byte for byte as without them. The `cem/v2` predicate does not record ceilings. Any
  other ceiling use is `invalid-arguments`. Help states that a verifier `PASS` row means the
  cited evidence is present at the target; it is not a test execution.
- `CEM-V1-639`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798)
  `corvint obligations` in stable mode MUST accept the valueless opt-in `--allow-absent-sidecar`.
  With it, an accepted map whose sidecar is `ABSENT` at the target (the map is not committed at
  `.corvint/change.cem.json`) produces the sidecar document with `binding.sidecar: "ABSENT"`; the
  map stays bound by `cem_sha256`. Without the flag, the `ABSENT` refusal names both fixes: commit
  the map, or pass the flag. An `EXACT` sidecar's document is byte for byte unchanged with or
  without the flag and carries no `sidecar` member. `MISMATCH` and `UNSUPPORTED_KIND` still
  refuse. The flag outside stable mode, or given twice, is an argument error.
- `CEM-V1-640`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798)
  `cem report` MUST read an absolute `--map` outside the repository under the `CEM-V1-315` and
  `CEM-V1-316` rules, with the same embedded verifier result as the repository-relative read. Its
  review file may sit beside the external map, but an `--output` naming the map itself is refused
  `invalid-arguments` ("report and map paths must differ"), and the map is never written. The help
  map-path block names `report`. The MCP report preview stays repository-relative; `prove --cem`
  reads an absolute map under `CEM-V1-641`.
- `CEM-V1-641`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798,
  GitHub #1895 ask 5) `prove --cem` and `prove --verify-cem-attestation --cem` MUST read an
  absolute map outside the repository under the `CEM-V1-315` and `CEM-V1-316` rules: a parent
  inside the worktree or a Git directory, a symlinked map, a missing map or an oversized map is
  refused before anything is signed, with exit 2 and no stdout. Such a map MUST be named
  `external:<64 lowercase hex SHA-256 of its bytes>` wherever the repository-relative path would
  appear: the in-toto prove statement's map subject and the `cem/v2` statement's first subject.
  The signed bytes therefore carry no host path, and the same map bytes at two locations sign
  byte-identical output. A repository-relative map keeps its path as the subject name and its
  output is byte-identical to the output before this change. The `cem/v2` predicate, its digest
  members and the verifier's checks are unchanged; the locator is a subject name string the
  existing shape already admits. Non-goals: no external map for any other `prove` mode, no host
  path recorded anywhere, and no map lookup by digest (the caller still supplies the bytes).
  Failure mode: a consumer that resolved the subject name as a repository path finds no file for
  an `external:` name and must use the digest it already checks.
- `CEM-V1-642`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2798,
  GitHub #1895 ask 4, decided by the owner 2026-10-27) `corvint obligations` in stable mode MUST
  accept `--intent-overlay FILE`, a reviewer-supplied Markdown intent outside the target tree. FILE
  is read under the `CEM-V1-641` map-path rule: it must be absolute, its parent outside the
  worktree and the Git directories, and a symlinked, missing or oversized file refuses; each
  refusal, a file that is not an intent under the `OCM-V0-001` Requirements rule, and the flag
  outside stable mode exit 2 with no stdout. The document then gains one member,
  `reviewer_overlay`, with `authority: "reviewer-supplied"`, `authoritative: false`, the file's
  `intent_sha256`, its Requirements `span` (`start`, `end`, `sha256`), one `obligations` row per
  requirement ID with `state: "UNKNOWN"`, and a fixed `note`. The overlay is added after the
  document's `id` is computed and is never an input to the state, hunks, providers, criteria,
  `id` or exit status, so removing the member gives the document of the same run without the
  flag. It is not project authority: project-owned authority outranks it (product invariant 3),
  and it can never upgrade an `UNKNOWN`. Non-goals: no change to OCM `intentScope`, `ocm/0.x` or any frozen shape, no target
  blob binding, no satisfaction claim, and no overlay outside `obligations`. Failure mode: a reader
  that skips the `authoritative` member could mistake the overlay's IDs for committed
  obligations; the member, the `note` and the `UNKNOWN` states say otherwise.
- `CEM-V1-643`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2809)
  A change MAY record that it defers intent scopes in one tracked file it creates,
  `.corvint/deferred-intents/<sha256>.json`, where `<sha256>` is the lowercase SHA-256 of the
  file's bytes. The bytes MUST be exactly
  `{"deferredIntents":[ENTRY...],"profile":"cem-deferred-intents/0"}` plus one newline, encoded
  without HTML escaping, with 1 to 4 entries strictly sorted by `path`, each exactly
  `{"baseBlobOid":OID|null,"obligationCount":N,"path":P,"reason":R}`: `baseBlobOid` is the
  intent's blob at the map's base or `null` when absent there, `obligationCount` is 1..65536,
  `path` is a clean repository-relative path outside `.corvint/deferred-intents/` and not
  `.corvint/change.cem.json`, and `reason` is 1..200 bytes of one line with no
  control character, U+2028 or U+2029. The record is at most 4096 bytes. A parser MUST refuse
  unknown members, duplicates and any bytes that do not re-encode identically. The record is not a
  `cem/1.0` member: the map's `patchSha256` and hunks bind it as they bind every other changed
  file, so the seal covers it and the review diff shows it.
- `CEM-V1-644`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2809)
  `corvint cem defer --map MAP --intent PATH --reason TEXT` MUST read a `cem/1.0` map, look up
  PATH at the map's base revision (a non-file there refuses), count PATH's worktree obligations
  with the host's intent counter (the `OCM-V0-001` Requirements rule in `corvint`, without the
  per-map OCM obligation bound, so an intent too large to declare can still be deferred), and merge the
  entry into the worktree's one record absent at base, replacing an entry for the same path. It
  publishes the new content-named record, removes the record it replaced and prints
  `{"ok":true,"mutates":true,"tool":"cem-defer","profile",...,"record","deferredIntents","count"}`.
  It never edits the map. A missing counter, a non-intent, a count outside 1..65536, a fifth
  entry, two created records, an invalid or misnamed created record, and an invalid path or reason
  exit 2 as `invalid-arguments`.
- `CEM-V1-645`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2809)
  `cem status` and `cem report` MUST check the records the map's hunks name under
  `.corvint/deferred-intents/` beside, never inside, the verifier result, and accept
  `--max-deferred-intents N` (default 0); `cem verify` and the MCP report preview refuse the flag.
  The check runs only on an `ACCEPT`; otherwise the member's `state` is `not-checked`. A record the
  change removes is skipped. Each remaining record MUST be absent at base, a regular blob, named
  by its content and valid under `CEM-V1-643`, and at most one may remain. Each entry's base blob
  MUST match the base tree, its path MUST be a blob at the target, and with the counter installed
  its count MUST equal the target blob's (`countState` `VERIFIED`, else `MISMATCH`; no counter is
  `NOT_CHECKED`). More entries than the bound is `deferred-intents-exceed-bound`. Any issue sets
  `ok` false and exit 1 with `state`, `policyState` and `stableVerification` unchanged. Status
  carries `deferredIntents: {profile, record, count, max, entries, issues, state}` (`state`
  `within-bound`, `refused` or `not-checked`) only when a record is named or the flag is given.
- `CEM-V1-646`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2809)
  `cem report` MUST carry the same member as `review.deferredIntents` in JSON and render a
  `## Deferred intent scopes` Markdown section before the non-claims, naming the state, the bound,
  the record and every entry's path, count, count state, base blob, base state and reason, and
  every issue, with each repository-derived string a code span. The member is added after every
  `recordSetSha256` computation, so that digest never covers it.
- `CEM-V1-647`: (experimental under the standing lane auto-accept (GitHub #1942)) `cem cite`,
  `cem mark`, `cem cover` and `cem discriminate` MUST read an absolute `--map` outside the
  repository under the `CEM-V1-316` rules when a repository-relative `--output` is given, and MUST
  write the result only to that output under the repository's lock. An absolute `--map` without
  `--output` MUST be refused `invalid-arguments` with the reason, never written in place, and an
  absolute `--output` stays refused.
- `CEM-V1-648`: (experimental under the standing lane auto-accept (GitHub #1941)) Every `cem`
  action parsed by the shared `cem` argument parser MUST refuse `--range`, `--range-root` and
  `--paired-from` (spaced or `=` form) with `invalid-arguments` naming the flag and the reason,
  before its required-argument stage. `prove --cem` and `prove --verify-cem-attestation ... --cem`
  read the same `cem/1.0` maps, so they MUST refuse these flags with the same `invalid-arguments`
  text, never the bare unrecognized-arguments line, before their own required-argument checks
  (GitHub #1941, V1-2936). The closed `verify-stable`, `bundle` and `gate-verdict` parsers MUST give
  the same reason on stderr, each keeping its own closed refusal code (V1-2943). A
  `cem/1.0` map binds one repository's base and target, so no `cem` verb accepts a paired change.
- `CEM-V1-655`: (experimental under the standing lane auto-accept (V1-2964, GitHub #1950)) When the
  canonical patch of `cem prepare --base B --target H` refuses `unsupported-tree-mode`, the refusal
  MUST keep that code and name every changed path whose base or target tree entry is not a
  regular-file blob, as `PATH (base MODE, target MODE)` with `absent` for a missing side, up to
  eight and then a count, with a recovery line. Only tree entries are read: a symbolic link's text
  is never read for the message and a link is never followed. A link unchanged since the base is
  outside the patch and maps as before. The opt-in `cem-non-regular/1` profile maps such a range
  instead, recording each non-regular path as an explicit unknown row (`CAX-V0-020..024`).
- `CEM-V1-690`: (experimental under the standing lane auto-accept (V1-2989)) both verifiers MUST bound the bytes they retain for authenticated evidence spans by
  the distinct blob bytes the existing aggregate blob budget already charges, not by the summed
  span lengths. The native `checkEvidence` (`internal/cem/verify`) and the portable consumer
  (`interop/cem01-go`, both the stable session and the `cem/0.1`..`0.3` paths) keep every span as
  a capacity-clipped window into one retained copy of its blob, so 64 overlapping near-full spans
  of one 4 MiB blob retain 4 MiB, not 256 MiB, and both verifiers accept them. Every blob read and
  its ledger row stay as before. Failure mode: a window keeps its whole blob alive until the
  verification ends; that is the same blob the aggregate budget (128 MiB native,
  `stableBlobAggregate` portable) already admitted.
- `CEM-V1-691`: (experimental under the standing lane auto-accept (V1-2996)) native drift classification MUST stop searching at the second overlapping match,
  which is all `stale`/`relocated`/`ambiguous` distinguishes, and MUST check its context before
  each search; a cancelled drift row refuses `git-cancelled`. `SpanOccurrences`, which names the
  exact count in a refusal message, MUST be linear in the haystack and needle (one
  Knuth-Morris-Pratt pass with one `int32` per needle byte). A 1 MiB repeated-byte needle in a
  4 MiB repeated-byte target classifies and counts in under 2 s. The portable consumer's
  `firstTwoMatches` already had this shape.
- `CEM-V1-692`: (experimental under the standing lane auto-accept (V1-2991)) `--hunk-revert` and `--guarding-tests` MUST NOT run a test in the pristine export.
  Each baseline run and each reverted run gets a fresh snapshot of the export (`<run>/tree` with an
  empty `<run>/tmp` for `TMPDIR`), with only the reverted file changed for a reverted run, and the
  snapshot is removed after the run. A file one run's tests create or change therefore never
  reaches another run: a test that writes a sentinel and fails when it already exists no longer
  makes an unrelated hunk `held`. A cached target baseline stays valid because it ran on its own
  snapshot. Recorded limit: each run copies the export once, so cost grows with tree size times
  runs, inside the unchanged per-run and total bounds; a Go cache variable redirected into the
  workspace (`CEM-V1-088`) is per run and so starts cold.
- `CEM-V1-693`: (experimental under the standing lane auto-accept (V1-2995)) a reverted run is `survived` only when every eligible test (one that passed at the
  target) reports a terminal `pass`. A run where some eligible test is skipped is `not-run:
  skipped`, and one where some eligible test reports no terminal result is `not-run:
  test-not-observed`; `revert-breaks-build` stays the package failure with no eligible test
  passing. A row is `survived` only when every package with eligible tests survived; a package
  with none (all its named tests fail at the target) does not block it. Failure mode: a row that
  previously read `survived` on a partial pass now reads `not-run` with the reason.
- `CEM-V1-694`: (experimental under the standing lane auto-accept (V1-3168)) `cem cite --replace
  --expected-hunk-digest DIGEST` (both or neither) replaces one hunk's citations: after every check
  an ordinary cite runs (evidence at the base, span, stable span, self-modified authority, limits),
  it MUST refuse `hunk-digest-mismatch` unless DIGEST equals the hunk's current
  `cem-hunk-state/1` digest, then drop that hunk's basis, attach the one citation, prune evidence no
  hunk still cites, and publish the map in one atomic write. The digest is the lowercase SHA-256 of
  the LF-terminated lines `cem-hunk-state/1`, the map's `baseRevision`, its `patchSha256`, the hunk
  ID, its disposition and its reason, followed by one `evidenceId<TAB>relation` line per basis entry
  in order (`wire.HunkDigest`). Because evidence IDs are content identities, the digest changes
  when any earlier citation of the hunk, or the base or patch it was cited against, changes. Every
  other hunk, and the hunk's coverage and discrimination witnesses (bound to the hunk and the
  patch, not to its basis), are kept unchanged. Failure modes: a stale old digest, stale base or
  patch, invalid span, policy refusal or failed publication writes nothing, so the previous map is
  intact; `--replace` without the digest, a digest that is not 64 lowercase hex digits, or either
  flag with `--paired-output` refuses `invalid-arguments`.
- `CEM-V1-695`: (experimental under the standing lane auto-accept (V1-3168)) `cem mark
  --expected-hunk-digest DIGEST` refuses `hunk-digest-mismatch`, writing nothing, unless DIGEST is
  the hunk's current `CEM-V1-694` digest; without the option `cem mark` is unchanged. A mark
  already replaces the hunk's basis, so the option only guards it.
- `CEM-V1-960`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2519) A
  repository root, administrative directory or common directory whose path has a symbolic-link
  component MUST be refused by the stable admission itself, before any boundary is retained, with
  `repository-object-unavailable` and a detail naming the link and asking for the resolved path. The
  retained boundary's `Validate` MUST NOT be the first to observe the link, so `repository boundary
  changed during verification` is reported only when an admitted directory or descriptor changed
  identity. The same repository opened by its resolved path is admitted and validates unchanged.
  Failure mode: a macOS `/var` temporary path is admitted and then refused as a change that never
  happened. Falsifier: a symlinked path that OpenStable admits, or a refusal of it whose detail does
  not name the link. Rollback: drop the symbolic-link detail branch in `stableAdmission.directory`;
  the generic `not reached through ordinary directories` refusal with the same code remains.
- `CEM-V1-980`: (experimental under the standing lane auto-accept (V1-2752)) Amends `CEM-V1-037`.
  The native parser (`internal/cem/patch`) MUST accept Git's canonical full-index binary section
  as a hunk-less binary file group that records its path, kind, mode and both object IDs. Every
  other binary shape keeps the `binary-patch` refusal and the `CEM-V1-037` named path. That
  includes a `GIT binary patch` body, rename, copy or similarity metadata (refused when the line is
  present, whatever its value, so an empty `rename from ` line also refuses), `old mode`/`new mode` lines,
  abbreviated object IDs, a marker whose labels disagree with the `diff --git` line, and a quoted
  path. A textual group parses exactly as before. The accepted section is exactly these lines,
  ending at EOF or at the next `diff --git` line:
  - `diff --git a/PATH b/PATH`, with an unquoted, symmetric, valid repository path;
  - at most one `new file mode M` (a creation) or `deleted file mode M` (a deletion) line, with
    `M` either `100644` or `100755`;
  - one `index OLD..NEW` line. `OLD` and `NEW` MUST be full object IDs of the same length (40 or
    64 lowercase hex). `OLD` is all zeros exactly for a creation, and `NEW` exactly for a
    deletion. A modification's two IDs differ, and only a modification carries the ` M` mode
    suffix;
  - `Binary files a/PATH and b/PATH differ`, with `/dev/null` for the absent side.
- `CEM-V1-981`: (experimental under the standing lane auto-accept (V1-2752)) A binary group adds
  no hunk, hunk ID, wire member or schema change. It is bound by the map's `patchSha256`, because
  the canonical patch is accepted only after its binary section's path, modes and both object IDs
  are proved against the self-hashed base and target trees (the `CEM-V1-110` diff recipe in the
  packet's `ALGORITHMS.md`). The blob size is a function of the object ID and is reported, not wired
  (`CEM-V1-983`). Textual hunks, their IDs and their disposition rules are unchanged. A patch
  whose every group is binary MUST still refuse `binary-patch`, because a `cem/1.0` map needs at
  least one textual hunk, and the message MUST name the first binary path.
- `CEM-V1-982`: (experimental under the standing lane auto-accept (V1-2752)) Base simulation
  (`internal/cem/sim`) of a binary group applies no hunk. For a modification or a deletion, the
  base path MUST exist as a regular blob whose Git object ID equals `OLD`, computed with SHA-1 for
  a 40-hex ID and SHA-256 for a 64-hex ID; otherwise the simulation refuses `base-mismatch`. The
  base mode MUST equal the group's mode; otherwise it refuses `diff-metadata-mismatch`. For a
  creation, the destination MUST be absent from the base tree, exactly as for a textual creation.
  The target side has no bytes in the patch, so it is bound only by the canonical-diff proof that
  `cem verify` and `verify-stable` rerun for the map's target.
- `CEM-V1-983`: (experimental under the standing lane auto-accept (V1-2752)) When the prepared
  patch has a binary group, the `cem prepare` result MUST carry `binary`. The member is absent
  when the patch has no binary group, so every earlier prepare result is byte-identical. A blob
  whose header cannot be read refuses the prepare with the Git read's code before any output is
  written. `binary` is a patch-ordered array of `{path, change, mode, baseBlobOid, targetBlobOid,
  baseSize, targetSize, disposition}` rows:
  - `change` is `create`, `modify` or `delete`;
  - the absent side's object ID and size are `null`;
  - each size is the byte size Git's object header gives for that object ID;
  - `disposition` is always `excluded`: the file is bound by its identity, and no citation,
    mark or textual check reads its bytes.
- `CEM-V1-984`: (experimental under the standing lane auto-accept (V1-2752)) `cem cite` MUST
  refuse `cite-span-not-stable` for an evidence span in a path that the map's patch changes as a
  binary group. The message names the path and states that binary content has no line
  classification. Without this rule, an empty hunk list would classify the span as stable. A span
  in any other path is cited as before.
- `CEM-V1-985`: (experimental under the standing lane auto-accept (V1-2752)) The binary
  exclusion is bounded by the existing limits, and no new bound is added. An over-bound binary
  file refuses with the existing code of the bound that it exceeds, never by silently omitting the
  file. Binary bodies are never read, so `cem prepare`, `cem verify` and `verify-stable` MUST each
  read the Git object header of every object ID a binary group names, base and target side, and
  refuse `repository-object-unavailable` naming the path when one exceeds `gitauth.MaxBlobBytes`,
  before base simulation. The limits are:
  - the canonical patch is at most `patch.MaxPatchBytes` (8 MiB);
  - the canonical-diff proof reads at most `gitauth.MaxTotalBlobBytes` (128 MiB) of blobs in all,
    and each blob at most `gitauth.MaxBlobBytes` (64 MiB);
  - base simulation reads the base blob through the same bounded source as a textual group.
- `CEM-V1-986`: (experimental under the standing lane auto-accept (V1-2752); amended by
  `CEM-V1-934..936`, V1-3443, which retire this divergence for the portable stable routes) The frozen
  `protocol/cem-1.0/stable/` packet and the portable consumer (`interop/cem01-go`) are unchanged.
  The portable consumer keeps refusing any patch with a binary marker line (its `patch` refusal),
  so it rejects a map that the native reader now accepts. The divergence fails closed, and it is
  recorded here as a native-only extension until a later packet revision. The shared tamper
  matrix, citation vectors and repository-envelope cells contain no binary section, so every
  pinned native and portable verdict is unchanged.
- `CEM-V1-930`: (experimental under the standing lane auto-accept (V1-3404)) Amends `CEM-V1-301`
  for the native parser (`internal/cem/patch`). A textual modification or rename group MAY carry
  one `old mode M1` line and one `new mode M2` line, with `M1` and `M2` each `100644` or `100755`
  and different. The group records both modes. Git then omits the `index` mode suffix, so a mode
  suffix beside these lines refuses `diff-metadata-mismatch`, as do these lines on a creation or a
  deletion. A lone, repeated or equal pair, or a mode other than `100644` or `100755`, refuses
  `malformed-patch` as before (`CEM-V1-301`). A binary group with these lines keeps the `CEM-V1-980` `binary-patch`
  refusal, and a mode-only group keeps the `CEM-V1-300` refusal.
- `CEM-V1-931`: (experimental under the standing lane auto-accept (V1-3404)) Base simulation
  (`internal/cem/sim`) of a group with a mode change MUST refuse `diff-metadata-mismatch` when the
  base path's tree mode differs from `M1`. The hunks apply exactly as for a group without a mode
  change. The target mode is bound only by the canonical-diff proof (`internal/cem/gitauth`), which
  already renders these lines from the base and target trees. No hunk, hunk ID, wire member or
  schema changes.
- `CEM-V1-932`: (experimental under the standing lane auto-accept (V1-3404)) The frozen
  `protocol/cem-1.0/stable/` packet (`ALGORITHMS.md` still rejects a mode change) and the portable
  consumer (`interop/cem01-go`, `unsupported-tree-mode`) are unchanged. The portable consumer
  therefore refuses a map that the native reader accepts. This divergence fails closed, and it is
  recorded here as a native-only extension until a later packet revision. The `mode-and-text-change`
  vector in `interop/cem01-go/testdata/metadata-only/cases.json` pins it, with an empty
  `nativeCode` (accepted) and an unchanged `portableCode`.
- `CEM-V1-933`: (experimental under the standing lane auto-accept (V1-3404)) A commit that changes
  a tracked file's mode and edits its text MUST be bindable: `cem prepare` accepts its canonical
  patch, and the native stable verifier accepts the prepared map. `corvint dogfood
  propose-citations` reads the same parser. A regression test MUST build such a commit in a real
  repository.
- `CEM-V1-934`: (experimental under the standing lane auto-accept (V1-3443)) Amends `CEM-V1-986`.
  The portable consumer (`interop/cem01-go`) MUST accept, on its stable routes
  (`verify-stable` and `verify-stable-canonical`), Git's canonical full-index binary section in a
  patch with at least one textual hunk, with the `CEM-V1-980` grammar: a symmetric `diff --git`
  line; no rename, similarity or `old mode`/`new mode` record; `new file mode` for a creation or
  `deleted file mode` for a deletion; one `index` line whose full lowercase object IDs have one
  length (40 or 64), all zeros on the absent side, with a mode suffix only for a modification of two
  distinct nonzero IDs; and the `Binary files a/P and b/P differ` marker as the group's last record,
  with `/dev/null` on the absent side. A binary-only patch, `GIT binary patch` data, any other binary
  shape and any binary section on the candidate route keep the `patch` refusal.
- `CEM-V1-935`: (experimental under the standing lane auto-accept (V1-3443)) The portable consumer
  MUST NOT simulate a binary group or read its blob bodies. Each present base and target entry MUST
  be a regular file (otherwise `unsupported-tree-mode`), in the group's mode (otherwise
  `diff-metadata-mismatch`), whose object ID equals the one the index line names (otherwise
  `invalid-field`). The path still counts toward the changed-path inventory. The canonical patch is
  derived from Git and bound by `patchSha256`, as for a textual group.
- `CEM-V1-936`: (experimental under the standing lane auto-accept (V1-3443)) The stable packet is
  revised in place to revision r2 under its own rule: the r1 `ALGORITHMS.md` and `manifest.json`
  bytes are preserved unchanged under `protocol/cem-1.0/stable/revisions/r1/`; `ALGORITHMS.md`
  items 6 and 7 admit the binary section; and `manifest.json` re-hashes every file, including the
  preserved ones. The shared case `interop/cem01-go/testdata/binary-section/` (a binary creation,
  modification and deletion beside one textual hunk) MUST yield one ACCEPT with one patch digest
  from the native reader and from both portable stable routes, and a REJECT once the digest is
  altered. The `old mode`/`new mode` divergence of `CEM-V1-932` is unchanged.
- `CEM-V1-955`: (accepted under the owner's standing auto-accept (2026-11-01); V1-3531) `corvint lrf --cem`, `--ocm` and `--patch` read only inside the repository root; an absolute path outside it MUST refuse `invalid-arguments` with a message that states the rule, the resolved absolute path and the root, and names `corvint cem verify --map` as the stable reader that accepts a map outside the repository (`CEM-V1-315`). Unlike `cem status`/`verify`, `lrf` keeps the inside-root rule: it is the frozen `lrf/0` admission. Test: `TestLRFOutsideRootRefusalNamesRulePathAndRoot`. Rollback: restore the fixed message.
- `CEM-V1-961`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3756)
  The tree-entry, tree-byte, depth, commit-size and Git-operation bounds of `CEM-V1-352`, and the
  legacy configuration profile of `CEM-V1-962`, MUST bind the legacy `verify-stable` route only
  (`verify-candidate` keeps its own candidate-proof bounds). `verify-stable-canonical` MUST verify a map whose snapshots hold
  more than 4096 entries, in an ordinary checkout whose `.git/config` carries `[remote]` and
  `[branch]` sections, with the same outcome it gives at any smaller size. An independent
  verification or Go/native agreement claim over a repository of this repository's size MUST name
  the canonical route; a legacy-route `UNSUPPORTED` answer is a bound, not a verdict.
- `CEM-V1-962`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3756)
  The legacy `verify-stable` route's repository profile, which `verify-candidate` shares, is
  declared, not incidental: `.git` MUST be a directory
  (a linked worktree's `.git` file is outside it), `.git/config` MUST be a regular file of at most
  64 KiB holding only `[core]` with `repositoryformatversion`, `filemode`, `bare`,
  `logallrefupdates`, `ignorecase` or `precomposeunicode`, and `[extensions]` with
  `objectformat`. Any other section or key (an ordinary clone's `[remote "origin"]` and
  `[branch]` sections included) is refused `UNSUPPORTED` / `repository` /
  `unsupported-repository-envelope` before any object is read. A linked worktree is refused
  `unsupported-object-alternates`, because the alternates probe fails first under a `.git` file;
  that code names the probe, not an alternates file.
- `CEM-V1-963`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3758)
  Every tracked `.corvint/changes/NAME.cem.json` MUST have a `baseRevision` that a retained ref
  (default `refs/remotes/origin/main`) reaches, or a `.corvint/cem-lineage.tsv` row
  (`corvint-cem-lineage/1`: target, base, label, retained ref) labelling that exact base
  `archive-lineage`, `rebased-lane-base` or `base-unavailable`. `make cem-base-retention-check`
  MUST refuse any other map, a register row whose map is not tracked or whose base differs, and an
  unknown label, naming each; with no resolvable retained ref it MUST print that it was skipped.
  A label is a disclosure and changes no verdict.
- `CEM-V1-750`: (experimental under the standing lane auto-accept (V1-3771)) Amends `CEM-V1-228`.
  A map's `hunks` array MUST list the canonical patch's hunks in patch order: position `i` holds
  the hunk whose `@@` header is the `i`-th in the canonical patch. Both the native verifier
  (`internal/cem/verify`, on every route that checks hunk coverage) and the portable consumer's
  stable routes (`verify-stable`, `verify-stable-canonical`) MUST refuse a map that names the
  right set of hunks in another order, at the verification stage with the existing closed issue
  code `invalid-field`, the code a hunk that disagrees with its derived identity already gets. No
  new closed code is added. The matrix case `hunk-reorder` (class `HunkReorder`) is that REJECT.
  The citation block is unchanged, because it sorts hunks by `id`.
- `CEM-V1-751`: (experimental under the standing lane auto-accept (V1-3771)) A span edit whose
  `spanSha256` and evidence ID are recomputed (matrix case `span-hash-recomputed`, class
  `SpanRangeEdit`) and an evidence-ID swap between hunks (`evidence-ids-swapped`, `CEM-V1-229`) are
  admissible re-productions, not tampers, and MUST ACCEPT. Each result is a different
  self-consistent map whose every record is still a real, digest-bound span of the base. The
  verifier binds bytes and identities, not which span a producer chose to cite. That choice cannot
  be checked without an authority outside the map. The support tier reports it instead
  (`CEM-V1-080`). No signature, citations plan or producer identity is read to tell the two maps
  apart. A map edited without recomputing (`span-end-edited`, `span-hash-mismatch`) keeps its
  REJECT.
- `CEM-V1-752`: (experimental under the standing lane auto-accept (V1-3771)) The stable packet is
  revised in place to revision r3 under its own rule. The r2 `ALGORITHMS.md` and `manifest.json`
  bytes are kept unchanged under `protocol/cem-1.0/stable/revisions/r2/`. `ALGORITHMS.md` item 5
  states the patch-order rule. The matrix `EXPECTED.json` pins `hunk-reorder` as exit 1, `REJECT`,
  stage `verification`, issue code `invalid-field` and a null citation. The matrix README states
  `CEM-V1-751`. `manifest.json` re-hashes every changed or added file. Every committed map under
  `.corvint/changes/` MUST already list its hunks in patch order, which a scan over the committed
  corpus shows before the rule lands. The producer (`internal/cem/workflow`) emits hunks in parse
  order, which is patch order.
- `CEM-V1-753`: (experimental under the standing lane auto-accept (V1-3800)) The portable legacy
  `verify-stable` route keeps its `primary-clean-config-bounded/1` envelope, which refuses linked
  worktrees (`ALGORITHMS.md`). It MUST name that refusal correctly. A repository whose `.git` is not a
  directory (a linked worktree's `.git` file) is `UNSUPPORTED` at stage `repository` with
  `unsupported-repository-envelope`, not `unsupported-object-alternates`. The same linked worktree
  with no alternates file MUST ACCEPT on `verify-stable-canonical` (`canonical-repository-bounded/1`),
  the route that admits it. A primary clone backed by `objects/info/alternates` keeps
  `unsupported-object-alternates` on `verify-stable`.
- `CEM-V1-754`: (experimental under the standing lane auto-accept (V1-2023)) The portable consumer
  MUST read the verdict ledger with `cem01-go read-verdict-ledger --repository ABSOLUTE_PATH
  --commit REV`. This narrows the `CEM-V1-271..279` non-goal to the writer. Its exit 0 line MUST
  carry the members of the native `cem provenance --verdicts` `verdicts` member (`ref`, `trust`,
  `authority`, `state`, `records`) with the same values: the `CEM-V1-276` state, in the same
  precedence, and the `CEM-V1-272` records only when the state is `verified`. A refusal MUST exit 2
  with `{"ok":false,"code":CODE}`, where CODE is the native reader's. A commit that does not resolve
  is `repository-object-unavailable`. Missing, unknown or repeated flags, or a relative repository
  path, are `invalid-arguments`. The repository MUST be admitted as the native reader admits it: a
  path that is not a worktree root (a subdirectory or a bare repository) is
  `repository-object-unavailable`, object alternates are `unsupported-object-alternates`, effective
  `info/attributes` lines are `unsupported-repository-attributes`, and a reciprocal linked worktree
  is read. Like the native reader, a `git notes list` output over 4 MiB or a note over 1 MiB MUST
  refuse `git-output-exceeded`, and each bound admits its limit. The read MUST write no ref, note, object, index, config or worktree
  file. Like the native ledger, it MUST carry no authority (`CEM-V1-278`): no verify route reads
  it. Non-goals: no `--verdicts-tip` rollback read and no portable writer.

## Non-goals and failure modes

No daemon, network account, broad language rewrite or new specification language is required. Tests remain trusted local executable code; the optional executor does not provide an operating-system sandbox. A declared executable directory does not attest every dependency in that directory. Missing browsers, simulator/device/app identities, unavailable frameworks, unsupported custom harnesses, stale claims and absent independent consumers are retained failure modes, not passes.

## Acceptance evidence

Stable acceptance remains OPEN until the native tickets retain complete qualification, integration and completion evidence. The current implementation has 55 concrete experimental runner profiles: 21 dynamic, 15 native, 16 platform, two SQL and one Appium Android. Six additional listed IDs explicitly refuse execution; the registry listing is not a support claim. Native Tasks capture/verification and the first portable candidate have passed bounded independent review. A same-target disposable proof also passed genuine focused native Tasks qualification, killed all three CEX controls, observed three generic Go passes and verified candidate references with both consumers. Neither this count nor candidate conformance establishes all-supported execution, a stable CEM1.0 wire or release readiness.

| Requirements | Implemented evidence | Remaining acceptance |
| --- | --- | --- |
| CEM-V1-001, 011..013 | `internal/cem/wire/candidate.go`, `internal/cem/verify/candidate.go`; `TestCandidatePortablePacket`; independent `interop/cem01-go` `TestCandidateNormativePacket`; 31 public cases in `protocol/cem-1.0/manifest.json` | The candidate stays an experimental reference profile with the 0.2 hunk vocabulary only (`CEM-V1-013`); the stable `cem/1.0` profile is implemented and produced on main (decision 0525, `CEM-V1-140..152`, V1-1276), its technical contract proposed and not promoted, qualification bounded |
| CEM-V1-002 | Native `internal/tasks/criterionbinding`, CLI read verbs and `internal/criterionexperiment/tasks_unix.go`; TCB-V0 traceability below its owning spec | A coherent same-target native Tasks/CEX/generic Go/candidate join is retained in `/private/tmp/cem10-build/coherent-join/summary.json`; immutable execution-at-commit attestation and integrated candidate production remain open |
| CEM-V1-003..004, 006 | `internal/testrunner`, registry and `cmd/corvint-test-runner`; TRE-V0 traceability and adapter provenance | Full negative/retry/lifecycle/platform matrices and missing application/device/domain tuples remain open |
| CEM-V1-005 | Existing experimental criterion lifecycle retains three killed controls under its own reviewed oracle contract | Cross-language criterion discrimination is not established by runner failures or candidate links |
| CEM-V1-007, 014..016 | Additive native and portable candidate verification plus the optional `internal/cemcandidate` producer and `cmd/corvint-cem-candidate`; byte-compatible runner envelope extraction; actual same-target assembly and independent `/private/tmp/cem10-build/cem-next/review1.json` PASS | Existing producer/report/OCM/frontier/companion consumer migrations and their compatibility inventory remain open |
| CEM-V1-008..010 | Optional-profile rollback defined; the 0.x defaults and packets are removed under decision 0525 §6 (`CEM-V1-215..225`) and kept in Git history only | Independent external consumer, matched external outcomes and terminal release/promotion evidence remain open |
| CEM-V1-051..059 | `lrfrepo.VerifyStableUniverse`, `StableUniverse`, `StableRefusal`, `lrf.ValidateStableRequest`; explicit `cem/1.0` refusals in `VerifyUniverse`, `readOCMBytes`, `PrepareOCM`; tests in the OCM/LRF stable admission traceability table | Stable LRF evaluation needs an accepted stable LRF profile; no CLI or MCP route. The OCM writers bind `cem/1.0` (`CEM-V1-149`, V1-1276) and the 0.x OCM/LRF routes are retired (`CEM-V1-210`, `CEM-V1-215`); C06..C09 and C17 landed under `CEM-V1-120..139`, `CEM-V1-147..150`, `CEM-V1-168` and `CEM-V1-290..299` (`MIGRATION.json`, V1-0595) |
| CEM-V1-050 | `structuralRuntimeAdmitted` in `internal/cem/verify/stable.go` and `interop/cem01-go/cem.go`; sealed `protocol/cem-1.0/structural-tuples/go1.27.2/` and `go1.27.3/`; `TestCEMV1050StructuralTupleAdmission` (native and portable); S0E, linked-control, ledger and cell tests bound through the tuple packet | The full structural corpus ran under V1-2327 (`CEM-V1-369`); Windows and linux/amd64 hosts remain NOT_RUN |
| CEM-V1-369 | `protocol/cem-1.0/structural-corpus/`; `TestStructuralCorpus`, `TestStructuralCorpusCases` (`interop/cem01-go`); `TestStructuralCorpusParity`, `TestStructuralCorpusCasesParity` (`internal/cem/verify`); runs in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-v1-2327-structural-label-lift.md` | Windows and linux/amd64 hosts NOT_RUN; linux/arm64 ran in a colima VM |
| CEM-V1-370 | `tools/cem-structural-closure`; `TestClosureDigestBoundary`, `TestExcludedTreesMatchWholeSegments`, `TestRunningToolchainClosure`; closure digests in `structural-corpus/QUALIFICATION.json` and each tuple's `TUPLE.json` | A future release with another closure needs a corpus run |
| CEM-V1-371 | `internal/cem/verify/stable.go`, `interop/cem01-go/cem.go`, `interop/cem01-go/stable_repository.go`; the `runtime` re-emission in the packet's `CHANGES.md`; `TestReplaceRuntimeKeepsFrozenTuple` (`tools/cem-stable-regen`); S0E and envelope-cell tests on the re-emitted packet | The frozen `cancelledContextExpectedResult` snapshot keeps the old label (no reader) |
| CEM-V1-080..082 | `internal/cem/verify/support.go`, `interop/cem01-go/citation.go` and `testdata/citation/`; `TestCEMV1080*`, `TestCEMV1081*`, `TestCEMV1082*` in `internal/cem/verify`, `internal/cem/workflow` and `interop/cem01-go`, including `TestCEMV1082MeasureVectorsNative` and `TestCEMV1082MeasureVectorsPortable` over `protocol/cem-1.0/stable/citation-vectors/measures.json` (V1-1331, V1-1335, V1-1332 under decision 0530; mutations in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-v1-1331-1335-1332-citation-measures.md`) | Recorded limits: `held` never produced, no `symbol` anchors, no threshold or cut-off, stable-result emission is CEM-V1-092; historical 591-map report in the build log, computed before the decision 0530 amendments |
| CEM-V1-083 | Plain criterion form in `internal/cem/wire/stable.go` and `interop/cem01-go/cem.go`; public case `protocol/cem-1.0/stable/neutral-criterion/`; `TestCEMV1083*` in `internal/cem/wire`, `internal/cem/verify` and `interop/cem01-go` | The plain form is produced by the experimental `cem-stable-plain-assembly/1` request of `corvint-cem-candidate assemble-stable` (`CEM-V1-240`, V1-1290); no default `corvint cem` verb emits it; stranger time NOT_OBSERVED |
| CEM-V1-240 | `AssemblePlain` in `internal/cemcandidate/plain.go` and the profile dispatch in `cmd/corvint-cem-candidate/stable.go`; `TestCEMV1240*` in `internal/cemcandidate` and `cmd/corvint-cem-candidate`; timed run in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-cem-plain-criterion-producer.md` | Experimental companion, not promoted; the runner plan and receipt are opaque inputs checked only by SHA-256, so receipt authority and execution at the target stay `NOT_OBSERVED`; one runner receipt per map in the single form, up to 64 admitted receipts in the `runners` form (`CEM-V1-573..578`); stranger time NOT_OBSERVED |
| CEM-V1-084 | `internal/cem/wire/stable_auth.go`, `internal/cem/verify/stable_auth.go`, `interop/cem01-go/stable_auth.go`; `--trusted-keys` in `internal/cem/cli/stable.go` and both portable commands; public case `protocol/cem-1.0/stable/receipt-authentication/`; `TestCEMV1084*` in `internal/cem/wire`, `internal/cem/verify`, `interop/cem01-go`, `internal/console` and `cmd/corvint` | No producer signs receipts (operator step, FPK-V0-036); `RESULT.schema.json` and the sealed S0E packet still name only `NOT_OBSERVED` for `authentication` and lack the four attestation issue codes (V1-1265); keys are TEST-ONLY; stranger time NOT_OBSERVED |
| CEM-V1-091 | `tools/cem-stable-regen` and the `CEM_STABLE_REGEN_CAPTURE` hook in `internal/cem/verify/stable_repository_test.go`; `TestCheckPassesOnRepositoryPacket`, `TestCheckCoversEveryStableManifestEntry`, `TestRegenerateRealPacketCopy`, `TestRegenerateRefusals`, `TestRegenerateSyntheticPacket`, `TestReplaceRealPacketCopy`, `TestReplaceRefusals`, `TestWidenRealPacketCopy`, `TestWidenRefusals`; proof runs in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-cem-stable-regen-tooling.md` and `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-v1-1331-1335-1332-citation-measures.md` | V1-1265 added `citation` (CEM-V1-092) and `verdictSha256` (CEM-V1-066) through the tool; since V1-1287 `--check` verifies every stable manifest entry (see "Stable manifest entry repair"); V1-1331 re-emitted `citation` and `verdictSha256` of 3 cases and 11 envelope cells with `--replace` under decision 0530 |
| CEM-V1-092 | `citation` member of `StableResult` (`internal/cem/verify/stable.go`) and `stableResult` (`interop/cem01-go/cem.go`, `stable_repository.go`), regenerated into the sealed packet and the public `neutral-criterion` and `receipt-authentication` expectations; `TestCEMV1092CitationOnlyOnAccept`, `TestCEMV1092SealedCasesCarryCitationOnlyOnAccept`, `TestCEMV1092RefuseAndRejectClearCitation` (native); `TestCEMV1092PortableCitationOnlyOnAccept`, `TestCEMV1082StableResultCarriesTheBlock`, `TestStableCanonicalFullResults` (portable) | The go1.24.13 cases 21 and 22 were not run on this host: their values come from the override file (case 21 derived natively on go1.27.2 with every other non-runtime key matching); ten envelope cells take `--cell-values` overrides |
| CEM-V1-066 | `verdictSha256` member and `VerdictDigest` of `StableResult` (`internal/cem/verify/stable.go`, `stable_verdict.go`) and `stableResult` (`interop/cem01-go/cem.go`, `verdict.go`), regenerated into the sealed packet and the public `neutral-criterion` and `receipt-authentication` expectations; `TestCEMV1066SealedVerdictDigests`, `TestCEMV1066VerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066EveryOutcomeCarriesVerdictDigest` (native); `TestCEMV1066PortableSealedVerdictDigests`, `TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest`, `TestStableCanonicalFullResults` (portable) | The go1.24.13 cases 21 and 22 were not run on this host: their values come from the override file, computed from their sealed expectations; ten envelope cells take `--cell-values` overrides; the experimental Tasks verdict gate pins it and requires the portable consumer to recompute it (`docs/specs/tasks-criterion-binding-v0.md` TCB-V0-017; decision 0525 step 8, V1-1283) |
| CEM-V1-093..095 | `internal/cem/wire/stable_provenance.go`, `interop/cem01-go/stable_provenance.go`; `provenance`, `role`, `sha1Oid` in `protocol/cem-1.0/stable/stable.schema.json`; re-homed 0.1/0.3 text in `protocol/cem-1.0/stable/ALGORITHMS.md`; shared vectors `interop/cem01-go/testdata/stable-map-members/cases.json`; `TestCEMV1093*`, `TestCEMV1094*`, `TestCEMV1095*` in `internal/cem/wire` and `interop/cem01-go` | The V1-1275 converter (CEM-V1-096..105) emits `provenance` but never `role`; `cem status --citation` role grouping is `CEM-V1-106` (V1-1281); provenance OIDs are SHA-1 only; the sealed S0E packet copy of `stable.schema.json` is unchanged; stranger time NOT_OBSERVED |
| CEM-V1-096..105 | Removed by V1-1278 (`CEM-V1-204`); satisfied by the frozen V1-1277 evidence; the converter no longer ships. Historically `internal/cem/convert`, `internal/cem/cli/convert.go` (`corvint cem convert`), `tools/cem-convert-parity`; `TestCEMV1096*`..`TestCEMV1104*` in `internal/cem/convert`, `cmd/corvint`, `tools/cem-convert-parity` and `interop/cem01-go`; full-corpus parity `https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1275/` | Committed maps converted by V1-1277 (`CEM-V1-230`); direct verification at a bind commit is `REJECT excluded-artifact-mismatch` until §5 sealing; cem/0.1 out-of-band patch parity limit (kept, decision 0532); the f0fda542 partial drift difference on the resource-limit abort was a native drift charge, fixed by `CEM-V1-240` (V1-1295); removal with 0.x is V1-1278 |
| CEM-V1-230..232 | Migration commit `0b6f1bc245b44a8620c5e64f91e87f6d08da08d0`; since V1-1278 the committed corpus is checked by `TestCEMV1201*` (`internal/cem/wire` and `internal/cem/corpusbinding`, `CEM-V1-201`); historically `script/cem-migrate-maps.sh`, `tools/cem-convert-parity --migrated`, `TestCEMV1100ConvertCorpusIdempotent`, `TestCEMV1230MigratedSourceChecks`; evidence `https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1277/` | Direct verification of a migrated map at its bind commit stays `REJECT excluded-artifact-mismatch`; checks run at T'; the in-tree citation tool has no T' target; a `cem/0.x` map landing later needs a follow-up migration commit; the binding test is NOT_RUN where Git objects are absent outside CI and fails under `CI=true` (V1-1334) |
| CEM-V1-200..204 | `RetiredSpec`/`RetiredSpecFix` and the `ParseStable` refusal in `internal/cem/wire/stable.go`; `internal/cem/wire/corpus_test.go`, `internal/cem/corpusbinding/binding_test.go`; `verifySealedCEM`, `proves` and the retired-seal branch in `internal/dogfoodflow/batch.go` and `change.go`; the `verify-stable`-only dispatch in `internal/cem/cli/stable.go`; `TestCEMV1200*`, `TestCEMV1201*`, `TestCEMV1204*`, `TestBatchCheckCEMV1203*`, `TestBatchCheckDCWV0039RequiresStableVerification` | The `cem/0.x` wire, verifier, producers and their goldens remain until their consumers move (V1-1278 S2/S3); `cli-parity` `cem-begin-map` still pins `cem/0.1` |
| CEM-V1-205..209 | `stableSidecarReuse` in `internal/localcompletion/sidecar_reuse.go`, `checkQualification` in `lifecycle.go` and the `reuseRefused` member of `CheckObservation`; `TestCEMV1205*`..`TestCEMV1208*` in `internal/localcompletion` | One verifier run per reusing check per evaluation; SHA-256 object-format repositories never reuse; `LCP-V0-004` text unchanged; skipped outside darwin and linux |
| CEM-V1-210..214 | Consumer refusals in `internal/lrfrepo`, `internal/frontier*`, `internal/extevidence`, `cmd/corvint/prove.go`, `internal/attest`, `internal/gitnotes`, `internal/console`, `internal/cem/workflow/read.go`, `internal/receiptbundle`, `internal/witness*`, `internal/doccorpus`, `internal/criterionexperiment`, `internal/cemcandidate`, `internal/localcompletion`, `tools/local-authority`, `tools/cem-citation-baseline`; conformance supersessions in `conformance/ocm-v0`, `conformance/frontier-v0` and `conformance/cli-parity-v0`; tests in the consumer cem/1.0 admission traceability table | S2b deletes the 0.x wire, verifier, `patch`, producers and `cem` CLI routes; S3 the protocol and interop 0.x packets |
| CEM-V1-215..219 | Deleted 0.x wire, verifier and producers in `internal/cem/wire`, `internal/cem/verify`, `internal/cem/workflow` and `internal/cem/cli`; retired `frontier/0`, `tcq/0`, `lrf/0` and `external-frontier-obligations/0` routes; `internal/attest/cem.go` retired-predicate refusal; regenerated `cmd/corvint/testdata/core-freeze` and `conformance/cli-parity-v0` declarations; tests listed under "Core 0.x removal traceability"; evidence `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1278-s2b-core-removal.md` | The `protocol/`, `interop/`, `examples/cem` and `conformance/cem-0.1` 0.x packets and their consumers remain for S3; the lrf-v0 corpus and frontier-v0 tier are retired whole, not ported |
| CEM-V1-220..225 | Deleted `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`, `conformance/cem-0.2`, `examples/cem` (except the 0.x-free `experiments/` companion demo), `docs/CEM-CI.md`, `tools/cem-interop-runner` and `script/cem-recipes_test.sh`; `protocol/cem-1.0/stable/inherited/cem-0.3-algorithms.md`; `interop/cem01-go/testdata/inherited-vectors/`; the retired route in `interop/cem01-go/main.go`; `stableStatusArguments` in `internal/dogfoodflow/change.go`; tests listed under "Packet retirement traceability"; evidence `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-13-v1-1278-s3-retire-packets.md` | The 0.x packets stay readable in Git history only; the portable consumer verifies `cem/1.0` only; the retired specs keep their bodies and requirement IDs as history |
| CEM-V1-170 | `internal/cem/workflow/stable_read.go` `StableCitationClass`, `stableRecordPaths`; `TestCEMV1170StatusCitationClassRecord` in `internal/cem/cli` | Committed record maps, which carry no role, are classified from their hunk paths retroactively; the portable consumer has no status envelope and is unchanged |
| CEM-V1-106 | `internal/cem/workflow/stable_read.go` `StableCitationClass` and the `status` branch of `readStable`; `TestCEMV1106StatusCitationClassGroupsChoreMaps` in `internal/cem/cli` | No producer writes `role` before the V1-1276 cutover, so every committed map reads class `change`; the portable consumer has no status envelope and is unchanged |
| CEM-V1-065 | `CEMStatementV2` and the v2 verifier in `internal/attest/cem_v2.go`; `prove --attest-cem-v2` and `--verify-cem-attestation --artifacts/--target` in `cmd/corvint/prove_attest_cem.go`; stable routing in `internal/gitnotes/anchor.go` (`wire.ParseStable`; the `wire.ParseRouted` 0.x router in `internal/cem/wire/route.go` was deleted by V1-1278 S2b, `CEM-V1-215`); `TestCEMV1065*` in `internal/attest`, `cmd/corvint` and `internal/gitnotes` | The verdict ledger is `CEM-V1-271..279` (V1-1292), kept beside the anchor pointer and outside `cem/v2`; no signing producer or key management; v2 emission needs full-OID `--expected-base` and `--target` and an ACCEPT result; no independent consumer of `cem/v2` |
| CEM-V1-067 | `corvint-cem-anchor/1` pointers, `describes` and `tree` in `internal/gitnotes/anchor.go`, the scope states and `ProvenanceWithNotesTip` in `internal/gitnotes/provenance.go`, `--notes-tip` in `internal/cem/cli/cli.go` and `cmd/corvint/cem_anchor.go`; tests under "Anchor pointer scope traceability" | No migration of existing `/0` notes; rollback detection needs an operator-retained tip; no independent consumer of the pointer profile |
| CEM-V1-115..117, 176..177 | `Refuse(..., true)` for both argument stages and `RelativePathFix` in `internal/cem/verify/stable.go`, the stderr line in `internal/cem/cli/stable.go`, the envelope `fix` in `internal/cem/workflow/stable_read.go`; the per-link uniqueness pass in `internal/cem/wire/stable.go`; `validateStableReferences` and `stableRelativePathFix` in `interop/cem01-go/cem.go`; public case `protocol/cem-1.0/stable/reference-precedence/`; `TestCEMV1115*`, `TestCEMV1117*`, `TestCEMV1177*` in `internal/cem/cli`, `internal/cem/verify` and `interop/cem01-go` | V1-1295 (partial drift on a resource-limit result) is resolved by `CEM-V1-240`; V1-1297 reports a missing pinned object as `repository-object-unavailable` (decision 0535, superseding the decision 0532 hold; see `CEM-V1-103`); technical contract acceptance and promotion |

### 1.0 recorded limits (decision 0546)

The owner accepted these as recorded limits for Corvint 1.0, not release blockers (decision 0546,
2026-10-20). An unqualified tuple still fails closed.

- Runner tuples: Linux and Windows tuples for the 18 runner profiles that lack them, Detox
  devices, the local TestCafe browser and Storybook T/R stay unqualified (as decision 0523 §3 did
  for rc.3).
- External consumer (`CEM-V1-008`): only the in-repo portable consumer `interop/cem01-go` exists. 1.0 claims no
  independent consumer and no external outcome evidence.
- Metadata-only diffs (an empty file created or deleted, or a mode-only change) are refused with
  a typed code in 1.0 (V1-0609). Representing them in `cem/1.0` is post-1.0.

## First portable candidate build contract

The first explicit candidate is `cem/1.0-experimental.1`. Historical ParseMap, Core producer defaults and portable verify/ci continue to refuse it; a separate candidate parser/verifier and portable `verify-candidate` operation provide the earliest interoperability proof. The candidate retains canonical independent base/target patch and exact target-sidecar binding, then adds closed criterion/runner/link reference records. It emits REFERENCE_INTEGRITY_ONLY with native Tasks authority, current applicability, source Git binding, authentication and discrimination NOT_OBSERVED. The existing execution receipt cannot acquire immutable source authority from a CEM target or a supplied file hash.

- `CEM-V1-011`: (accepted by decision 0546, 2026-10-20) Reject missing, duplicate, unknown or escaping reference/artifact identities. Retain original bounded artifact bytes and exact hashes, including the reviewed plan, runner receipt and native capture references. Unknown native verification is explicit; CEM seams do not import Tasks or runner implementations.
- `CEM-V1-012`: (accepted by decision 0546, 2026-10-20) Publish the candidate schema and algorithms before the standalone consumer. Native and standalone consumers derive the canonical patch from Git and independently supplied base/target, compare exact committed sidecar bytes and immutable evidence spans, and share positive/adversarial vectors without importing each other's implementation. Corvint-authored consumer conformance remains distinct from an independent external consumer.
- `CEM-V1-013`: (accepted by decision 0546, 2026-10-20) Never translate candidate spec labels to earlier0.x profiles to bypass consumer migration. The first candidate supports only the portable0.2 hunk vocabulary; missing0.3 structural/coverage/discrimination portability remains a stable1.0 release blocker.
- `CEM-V1-014`: (accepted by decision 0546, 2026-10-20) Inventory every existing consumer's profile and migration disposition. An unsupported candidate consumer retains explicit refusal; no candidate reference link satisfies a criterion, native gate, mutation kill, local completion or stable release requirement.

Gate A reviewed the bounded candidate plan with no remaining HIGH under these restrictions. The implemented candidate passed independent repair-cycle-1 review: `/private/tmp/cem10-build/cem1-wire-rereview1.json` binds the 53-file freeze in `cem1-wire-repair1.json`. The 31-case packet includes the original forged-null criterion identity and mixed-case `.GiT` artifact attacks, now refused by both consumers; valid boundary cases remain admitted. Native and standalone consumers derive immutable Git changes, compare original target-sidecar bytes when present, validate evidence drift and hash opaque artifacts before and after verification. No supplied patch or rewritten profile label substitutes for those checks.

The portable module now requires Go 1.24 or later for rooted reads, including builds of its historical commands. This is an accepted, documented source-build floor change; the historical process ABI and frozen predecessor vectors remain intact. The official task-local Go1.24.13 compiler passed all 38 top-level portable historical/candidate tests (147 including subtests) and vet on Darwin ARM64; Linux/Windows AMD64 builds, test compilation and vet also passed. `/private/tmp/cem10-build/portable-qualification/qualification.json` retains verified archive provenance and unchanged 53-file hashes. Native Linux/Windows execution and original Go1.24.0 remain NOT_RUN; this is minimum-minor compatibility evidence, not ongoing security support. All eight candidate assurance limits remain NOT_OBSERVED: native authority, historical validity, current applicability, source Git binding, authentication, dependency closure, criterion discrimination and external interoperability. Raw artifact integrity does not decode or authenticate a native receipt.

## Optional candidate assembly contract

The optional `corvint-cem-candidate` companion is an experimental producer/consumer slice under the same reference-only candidate. Its technical contract remains proposed. It does not admit this profile to the default Core producer, OCM, frontier or native completion.

- `CEM-V1-015`: (accepted by decision 0546, 2026-10-20) Candidate assembly MUST require independently supplied immutable base and target, an exactly verified canonical `cem/0.2` source map, closed bounded declaration documents and independently declared raw artifact digests. It MUST preserve original native Tasks and generic runner artifact bytes, distinguish raw plan SHA256 from the native struct-serialization plan identity, check declared capture/base/optional candidate-tree and ordered criterion references for coherence, and refuse divergent or missing references. Only Tasks wire contracts may be imported. Native capture semantic verification, live applicability, authentication and criterion adequacy remain NOT_OBSERVED; a retained verification envelope is not an independent native authority check.
- `CEM-V1-016`: (accepted by decision 0546, 2026-10-20) Assembly MUST compare each declared runner source input with its self-hashed immutable target Git blob under a literal declared source prefix, rejecting missing entries, links, unsupported modes and changed bytes. The result is DECLARED_INPUT_BYTES_MATCH_TARGET only; source inventory completeness and execution-at-commit remain NOT_OBSERVED. It MUST refuse unsupported `cem/0.3` source vocabulary and a target-sidecar conflict rather than dropping witnesses, rewriting target commits or translating profiles. Output goes only to a new confined standalone bundle with exact artifact-role closure and must pass the native candidate verifier before success. A failed or incomplete runner observation remains retained and cannot be turned into a criterion pass.

The shared runner envelope types may be exported without changing field order, tags, bytes or native plan identity. Tests must demonstrate byte-for-byte compatibility. Rollback removes the optional companion and restores the original aliases without altering historical wire bytes, retained packets, stores or default commands. The coherent actual Go demonstration is self-use, not independent external adoption.

## Candidate artifact refusal codes

These codes describe bounded reference-integrity checks; they do not establish native receipt authority.

| Code | Emitting check | Source |
| --- | --- | --- |
| `candidate-artifact-unavailable` | Cancellation, unavailable paths, symlinks or unsupported path kinds, path lstat/open/read/close failures prevent reading the declared artifact. | `internal/cem/verify/candidate.go:93@8c882b4b` |
| `candidate-artifact-bound` | An opened artifact cannot be statted, is not a regular file, exceeds the per-artifact byte bound, or makes the total artifact inventory exceed its byte bound. | `internal/cem/verify/candidate.go:112@77e4c1f4` |
| `candidate-artifact-digest` | SHA256 of the artifact bytes differs from the declared digest. | `internal/cem/verify/candidate.go:128@21c0f6f2` |

## Optional stable assembly prototype

Technical contract: proposed experimental R1 producer; no default or release
admission. `corvint-cem-candidate assemble-stable --experimental --request ABS
--out-dir NEW_ABS` is a separate optional operation. Existing candidate assembly,
verification commands and their emitted bytes retain their contracts.

The native Core entrypoint `corvint cem verify-stable` is likewise explicit-only and
experimental: it is hidden from help, is matched only in command position (after
optional leading global options), and carries no default or promotion.

The closed `cem-stable-assembly/1` request independently names immutable base and
target, ticket/attempt identity, explicit criterion links, source prefix, and six
absolute SHA256-pinned inputs: canonical source map, native capture, retained
historical verification, snapshot-head artifact, runner plan and runner receipt.
The source is parsed and canonically verified as its actual `cem/0.2` or `cem/0.3`
profile. A distinct typed `StableMap` is constructed and encoded through
`EncodeStable`; complete coverage/discrimination witnesses survive. No source
profile substitution bypasses a parser, validator or consumer migration.

The producer retains all six raw artifact roles, including the exact claimed-ticket
string and exact snapshot-head receipt bytes. `snapshotHeadReceiptSha256` remains
the native binding's identity; `snapshotHeadArtifactSha256` is independently
computed from supplied original bytes. Full criterion identity includes both
fields, and links use the new complete identity. Their incidental equality in one
historical proof is not an identity rule. The generic input reader continues to
reject `.git` paths; an operator must separately pin and stage a selected original
receipt outside Git metadata before supplying it.

Only pure Tasks wire decoding is used by the assembler. An independently configured
native Tasks executable can verify historical capture semantics in a separate
bounded read, with original stdin/stdout/stderr, command and pre/post executable
pins retained. That read does not authenticate the selected receipt's journal
linkage. Without that separate linkage evidence, receipt authority remains
`NOT_OBSERVED`. The stable assembly result contains the actual closed stable
verification envelope; it does not reuse the candidate result or upgrade any of
its thirteen authority/semantic axes.

Each declared runner input must match its immutable target blob. This establishes
`DECLARED_INPUT_BYTES_MATCH_TARGET` only. Raw plan identity, report hashes,
FAILED/SKIPPED/FLAKY/interruption/build/infrastructure outcomes and reported attempts
remain intact; no retry, execution-at-commit, inventory completeness, criterion
adequacy or discrimination claim is inferred. Output must be a new confined bundle,
the target must have no committed CEM sidecar in this producer slice, and native
stable verification must succeed before `stable.json` and a successful result are
published. Changed, absent or unsupported inputs refuse; incomplete intermediate
artifacts are not a successful bundle.

The bounded historical proof retains base
`474f75da42ffab0f6b04425f26c8b655602d2ed8` and target
`a14f9d0123a923fa11ec411ec293a643856603e5`, actual native capture verification and
original generic Go runner evidence. It compares the producer's embedded result
with Core and portable results recursively across all closed fields. Historical
coverage/discrimination witnesses remain absent; separate 0.3 witness tests are
manufactured, non-executed fixtures. All evidence is experimental self-use.

| Existing requirements | Bounded producer evidence | Remaining qualification |
| --- | --- | --- |
| CEM-V1-001, 011..013 | Typed stable construction, exact six-role graph, full witness retention, native/portable result equality | Complete public corpus, supported repository/runtime envelopes and all consumer migration |
| CEM-V1-002 | Pure native wire binding plus separately pinned historical Tasks verification | Current applicability, receipt journal authority and semantic adequacy |
| CEM-V1-003..005 | Exact actual runner/source/report bytes and native outcome preservation | Inventory completeness, execution-at-commit and criterion discrimination |
| CEM-V1-007, 014 | Optional companion producer, explicit stable verification, the hidden Core status/verify read (`CEM-V1-017..019`) the Core report/MCP preview (`CEM-V1-034..036`) and the Core stable bundle export (`CEM-V1-060..064`) | Other Core workflow, OCM/frontier/obligation, companion and completion consumers |
| CEM-V1-008..010 | Original defaults/packets unchanged and optional operation removable | External adoption/outcomes and release/promotion gates |

The current stable verifier admits the experimental
`canonical-repository-bounded/1` envelope of
`protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md`, including its expanded admission
amendment: fixed-order metadata admission without a configuration pre-read, at most
1,024 logical Git operations, one 30-minute wall deadline with a 10-second
per-operation deadline and a single 10-second emergency allowance, and owned
process-group retirement whose unobserved cleanup is `repository` /
`unsupported-process-containment`. The independently reviewed
`primary-clean-config-bounded/1` portable envelope remains preserved as history.
Darwin keeps the quiet-first retirement sequence: after an owned successful group
signal, signal 0 is polled while the leader is unreaped until the group is no
longer signalable, then the leader is reaped and post-reap absence is observed.
The Core runner repeats the group SIGKILL on each pre-reap live observation while
the leader is unreaped, because a Darwin group signal can miss a member forked
alongside it (`PGO-V0-017`).
Linux uses `ReapAfterSuccessfulSignal`: after the owned successful group signal
and observed leader exit, the leader is reaped exactly once and only signal-0
absence polling follows. Linux preserves all `KillGroup` errors, including
`EPERM`, `ESRCH` and early `Stop` failures; no real numeric group signal may occur
after reaping begins. The same original retirement allowance is checked before
and after owner primitives and before accepting post-reap `ESRCH`. Post-reap
polling is bounded by the intersection of that original allowance and the fixed
two-second post-reap cap. Both are checked synchronously before each probe and
immediately before accepting absence; expiration wins simultaneous interval,
exit or reap readiness, including expiration inside an absence-returning probe.
An already expired allowance admits no new signal, probe or reap, even when exit
is observed or its deadline notification has not arrived. A pending
asynchronous reap can be retained only as sticky `HOLD`, never as a later upgrade
or retry. This is linked to `CEM-V1-007` consumer migration because stable Core
verification depends on supported owned-runner containment. At source
`1a355ea551c2745d3fe9f229a36acc348cb205d8`, actual native Linux arm64 with Go 1.27.1
passes the five scoped packages and all 54 applicable public full-result cases,
with exact recursive comparison of all 21 frozen fields. The two historical
Go 1.24.13 cases separately pass the original portable comparator on Darwin arm64;
they are not Linux results. The repair build log retains these distinct runtime
observations and the initial failed qualification harness runs. On 2026-10-16
(V1-0650) cases 21 and 22 pass again on current source under the official Go 1.24.13
toolchain on Darwin arm64, and the hosted ubuntu-24.04 amd64 `go-interop` job passes the
portable package at package level (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-16-s0e-go1.24.13-and-linux-lifecycle.md`).
Per-test Linux amd64 results stay NOT_OBSERVED; cross-device topology and escaped-session
behaviour remain NOT_RUN; all three are excluded from 1.0; external consumers, default promotion and release readiness are
not claimed. Legacy Wait/Run is separate from the new Owner qualification. Canonical ordered fixtures do not qualify ordinary
checkouts; real ordinary, origin-config, linked and SHA-256 checkouts are qualified separately by `CEM-V1-360..367`. Rollback disables the new optional operation and preserves earlier
command defaults, candidate assembly, native task history and historical packet
bytes. Independent producer implementation review and root source admission remain
separate steps.

## Structural runtime tuples

The structural mechanical proof (`cem-0.3-structural-mechanical.md`) depends on the exact Go
release's `go/parser`, `go/scanner`, `go/ast`, `go/token` and `go/format` (`go/printer`) behaviour, so
the stable verifier admits it only on an exact qualified tuple and otherwise refuses with
`unsupported-structural-runtime`.

- `CEM-V1-050`: (accepted by decision 0546, 2026-10-20) The native (`internal/cem/verify`) and portable (`interop/cem01-go`) stable
  verifiers MUST admit structural mechanical proof only on an exact listed Go release: `go1.27.1`,
  the tuple the frozen S0E packet records, `go1.27.2` (decision 0522), and `go1.27.3`, its own
  sealed tuple and the sole toolchain pin under decision 0557. Every other release,
  including later patch releases, MUST keep refusing with `unsupported-structural-runtime` and no
  weakened fallback. A tuple admitted after `go1.27.1` MUST carry its own sealed packet under
  `protocol/cem-1.0/structural-tuples/<version>/` that binds the frozen
  `repository-envelope-packet` manifest and result files by SHA-256, records the stdlib source and
  behavioural byte-identity comparison against its predecessor, records the parser-closure digest
  that admits it (`CEM-V1-370`), and admits exactly one
  substitution: `expectedResult.runtime.goVersion` from the predecessor to the new tuple. The
  frozen packet is never rewritten except by an additive member regeneration or a recorded value
  re-emission of an existing member (`CEM-V1-091`). Tests run the S0E cases, linked control, ledgers and envelope
  cells on an admitted tuple only through that binding and report NOT_RUN otherwise.
  On an admitted tuple both verifiers report `runtime.structuralQualification`
  `QUALIFIED_EXACT_TUPLE` (`CEM-V1-371`), qualified by the structural corpus (`CEM-V1-369`).
  (Amended by V1-2327 under decision 0551, owner approval of 2026-10-21: the label was
  `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS` until the full corpus ran.) Rollback removes the tuple
  from both lists and its packet; the frozen packet and `go1.27.1` behaviour are unchanged.

### Additive member regeneration

An additive stable result member (for example `citation`, V1-1265) changes the closed key set of
every frozen S0E expectation, so it cannot land by hand edits to sealed case files.

- `CEM-V1-091`: (accepted by decision 0546, 2026-10-20) A new top-level member of `cem-stable-verification/1` MUST be added only by
  `tools/cem-stable-regen` (`make cem-stable-regen-capture`, then `make cem-stable-regen`). The
  values MUST come from the native S0E harness capture (`CEM_STABLE_REGEN_CAPTURE`, one
  `{id, tuple, exit, result}` line per case, comparing only the frozen keys) or from an explicit
  per-case override file for cases the host cannot run; a case with neither, a case with both, a
  captured exit or frozen key that differs (`runtime.goVersion` only through an admitted tuple
  substitution), an unknown capture key or an existing member MUST be refused before any write. The
  tool MUST insert the member after `--after` in both `RESULT.schema.json` copies (`properties` and
  `required`) and in every `expectedResult`, including each cell of the packet's
  `inherited/s0e/REPOSITORY-ENVELOPE.cells.json` and its stable copy. A cell takes the value of the
  regenerated cases whose exit and frozen keys equal its own, and those cases MUST agree; a cell no
  case matches MUST take an explicit `--cell-values` override, and a matched cell MUST NOT have one.
  The tool MUST leave every other byte unchanged, keep the case count,
  bump the closed key count, record each replaced file's SHA-256 (`predecessorSHA256`,
  `predecessorPublicManifestSHA256`, the tuple's `inherited.predecessorManifestSha256`, and a
  `CHANGES.md` section with old and new digests, value provenance, the fragment digest and the
  predecessor digest of every re-bound file outside the packet), and re-bind the packet manifest,
  the stable manifest entries for the schema, cells and prose, `publicAmendmentManifestSHA256`,
  the stable `REPOSITORY-ENVELOPE.cells.json` amendment index, `REPOSITORY-ENVELOPE.md` and every
  `structural-tuples/<version>/` `TUPLE.json` and manifest. `make cem-stable-regen-check` MUST
  verify that whole chain, including that each manifest lists its mandatory files, and exit nonzero
  on any mismatch. Native and portable consumers then raise
  their closed key count in the same change; a native/portable value disagreement is a consumer
  defect, never a reason to edit captured values. `INPUTS.json` repair-era pins stay historical.
  An existing member MAY instead receive a recorded value re-emission, only through
  `make cem-stable-regen-replace` (`--replace`), when an accepted amendment changes that member's
  values: the values come from the capture or override files as above (an override object names
  only the re-emitted members, and a member it omits keeps its value; a cell no case matches takes
  a `--cell-values` entry, which may be empty), every other key, exit, case, the schema and the key
  count stay unchanged, and a `--record` file gives the reason and, for every case and cell value
  that changes, the SHA-256 of the compact JSON of the value it replaces. A changed value the record
  does not name, a recorded value that does not change or a predecessor digest that differs MUST be
  refused before any write. The re-emission re-binds the same chain, sets the packet status to
  `MEMBER_VALUE_REEMITTED` and appends a `CHANGES.md` section with the reason, the record digest,
  the value provenance and a predecessor and new digest per value. (Amended by V1-1331 under
  decision 0530, owner approval of 2026-10-15: the frozen packet was additive-only, so no accepted
  correction to an existing member's values could reach it.) A re-emission MAY change only cell
  values: when no case value changes, `FULL-RESULT-CASES.json` and its `predecessorSHA256` stay
  byte-identical and the run is refused only when no case or cell value changes. A cell whose
  matching cases disagree on a re-emitted member is not derivable and MUST take a `--cell-values`
  entry; without one the re-emission is refused. (Amended by V1-1297 under decision 0535, owner
  delegated to the orchestrator 2026-10-16: re-emitting `code` of the missing-object cells needs
  both, because their other keys equal cases with other codes and no case value changes.)
  A re-emitted `runtime` member keeps the frozen `goVersion`: a captured `runtime.goVersion` that
  differs from the frozen value MUST map back through an admitted tuple substitution, and any other
  value is refused before any write. (Amended by V1-2327 under decision 0551: the capture runs on
  an admitted tuple, while every substitution starts from the frozen `go1.27.1`.)
  A string `const` or `enum` node of the schema MAY instead be widened, only through
  `make cem-stable-regen-widen` (`--widen`, `CEM-V1-620`), which keeps every case, cell and key
  count. (Amended by V1-2107: the result schema pinned a runner label that `CEM-V1-311` widened.)
  Rollback is a Git revert of the regenerated files and the consumer change.

## Consumer-computed support tiers (V1-1262, decision 0523 P0)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. Decision 0523 P0. The verifiers derive, from the base blob, target blob,
canonical patch and cited spans alone, how strongly each hunk is supported. No map field is added
or changed. Nothing here is a verdict, threshold or published cut-off.

- `CEM-V1-080`: (accepted by decision 0546, 2026-10-20) Support tiers are consumer-derived. The native (`internal/cem/verify`, `support.go`)
  and portable (`interop/cem01-go`, `citation.go`) verifiers MUST assign each hunk of a verified
  `cem/1.0` map exactly one tier, by precedence `held`, `mechanical-proved`, `anchored`, `cited`,
  `unknown`, plus an `annotation` flag. A supported hunk is one with at least one basis;
  `mechanical-proved` is the `mechanical` disposition after re-proof; `unknown` is a non-mechanical
  hunk with no basis. `held` is reserved and never produced until the go-test hunk-revert falsifier
  lands. No producer-written field can raise a tier. The tier MUST NOT change the verdict, the
  codes or drift. A retired `cem/0.1`, `cem/0.2` or `cem/0.3` map is refused `unsupported-spec`
  (`CEM-V1-210`) and never carries a tier. (Amended by V1-1332 under decision 0530: the text named
  `cem/0.2` and `cem/0.3` maps after the `cem/1.0` cutover.)
- `CEM-V1-081`: (accepted by decision 0546, 2026-10-20) Anchor kinds and byte-level rules. `id`: an ASCII requirement ID
  (`[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`, token boundaries where the adjacent bytes are not in
  `[A-Za-z0-9_-]`) occurs in the hunk's added bytes and the cited span contains a defining line
  (optional leading spaces or tabs, then ``- `ID`:``). The hunk path is the post-image path of the
  hunk's canonical patch file section, or its pre-image (base) path when the section has none (a
  whole-file deletion); every rule below that names a hunk's path uses it. `path`: the span
  contains the hunk path or its basename when the basename is at least 6 bytes. `annotation`: the
  hunk has added lines and, after every boundary-valid requirement ID in the added bytes is masked
  to spaces, every remaining token (maximal `[A-Za-z0-9_]` run) is shorter than 3 bytes or is, after
  ASCII lowercasing, one of the 39 annotation stop terms listed in
  `protocol/cem-1.0/stable/ALGORITHMS.md`: the `lexical-relevance-floor-v0` stop list without the
  21 terms that are a keyword, reserved word, predeclared identifier or builtin in a common
  programming language (`error`, `false`, `from`, `include`, `into`, `package`, `print`, `return`,
  `self`, `source`, `string`, `test`, `then`, `this`, `true`, `unknown`, `value`, `when`, `where`,
  `while`, `with`). A hunk adding only `return value - 1` is not an annotation. (Amended by V1-1335
  and V1-1332 under decision 0530: annotation used the full `lexical-relevance-floor-v0` list, and the
  hunk path was stated only for the `path` anchor.) A hunk's anchors are the union over
  its distinct cited spans. A `symbol` anchor is a recorded limit (Go-only, deferred) and is never
  produced.
- `CEM-V1-082`: (accepted by decision 0546, 2026-10-20) The `citation` block and the shift null MUST be byte-identical across the native and
  portable consumers and carry no verdict. Shape:

```json
"citation": {
  "profile": "cem-citation-support/0",
  "hunks": [ {"hunkId": "...", "tier": "...", "anchors": ["id","path"], "annotation": false} ],
  "hunksByClass": {"held": 0, "mechanical-proved": 0, "anchored": 0, "cited": 0, "unknown": 0},
  "annotationHunks": 0, "distinctEvidence": 0,
  "evidenceFanOut": [ ["<evidenceId>", 3] ], "maxFanOut": 0,
  "samePathCitations": 0, "concentratedHunks": 0,
  "invariant": true, "specificity": 0
}
```

  `hunks` is sorted by `hunkId` bytewise and each `anchors` list is sorted. `distinctEvidence` is the
  count of distinct cited evidence IDs. `evidenceFanOut` lists `[id, distinct citing hunks]` sorted
  by count descending then ID ascending; `maxFanOut` is the largest count or 0. `samePathCitations`
  counts basis entries whose evidence path equals the hunk path of `CEM-V1-081`, so a deletion hunk
  compares against its base path (the same evidence under two relations counts twice).
  `concentratedHunks` is 0 when `maxFanOut` is below 2, and otherwise counts supported hunks whose
  every basis cites an evidence ID at `maxFanOut`: concentration needs an evidence item two hunks
  share. (Amended by V1-1332 and V1-1331 under decision 0530: `samePathCitations` named the
  post-image path, which a pure deletion lacks, and `concentratedHunks` equalled the supported-hunk
  count whenever no evidence was shared.) The shift null sorts supported hunks by `hunkId`, gives hunk
  i the basis set of hunk i-1 (hunk 0 takes the last) and recomputes `anchored`. `invariant` is true
  when all supported hunks' evidence-ID sets are equal (true below two supported hunks).
  `specificity` is `anchored` minus `anchored` after the shift (0 below two supported hunks).

  Placement in this slice: the native 0.x canonical verification object carries key `citation` on
  success and on drift rejection only when `--citation` is given to `verify` or `status`; default
  output stays byte-identical to the frozen 0.x goldens (decision 0357) and to the base verifier,
  which `dogfood-check` requires. The stable (`cem/1.0`) result carries it as the member
  `citation` after `artifactChecks` (CEM-V1-092), added to the sealed `cem-stable-verification/1`
  shape (`protocol/cem-1.0/stable/RESULT.schema.json`, the S0E repository-envelope packet and the
  go1.27.2 structural tuples) by `tools/cem-stable-regen` (CEM-V1-091). The portable consumer
  computes the block as a library result pinned by `testdata/citation/`, emits it in its stable
  result, and emits nothing on the 0.x wire because the frozen `protocol/cem-0.3` vectors pin its
  0.x result. Amended by V1-1278 S3 (`CEM-V1-220`, `CEM-V1-222`): those vectors and the portable
  0.x route are deleted, so the portable consumer has no 0.x result.
- `CEM-V1-092`: (accepted by decision 0546, 2026-10-20) The `cem-stable-verification/1` member `citation` MUST be the `CEM-V1-082` block of
  the verified map, computed from the canonical patch and the bound span bytes, and MUST be non-null
  only when `outcome` is `ACCEPT`. On every other outcome (`REJECT`, `UNSUPPORTED`, a cancellation
  or cleanup override after a provisional ACCEPT, and an evidence-drift refusal or rejection) it
  MUST be `null`. Both consumers MUST follow this rule: the native verifier clears the member in
  every refusal and rejection path and sets it only at ACCEPT; the portable consumer clears it in
  its single failure path and sets it only at its ACCEPT points. A native/portable disagreement on
  the member is a consumer defect.

Non-goals: no pass/fail threshold, no published cut-off, no `symbol` kind, no change to map wire
fields, no producer-side tier, no stable `citation` on any outcome other than ACCEPT.

Failure modes: the block is derived only from span bytes the canonical verification already read
and bound, so a span that cannot be read fails verification before any tier exists; a consumer never
emits a partial or estimated block, and an absent block is not a verdict either way. A stable
result that carries `citation` on a non-ACCEPT outcome, or `null` on ACCEPT, violates CEM-V1-092
and fails the sealed full-result comparison.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-080 | `TestCEMV1080RubberStampAndCarefulMapsDiffer`, `TestCEMV1080CitationIsCanonicalOnly`, `TestCEMStatusCitationIsOptIn` (native); `TestCEMV1080RubberStampAndCarefulBothAccept` (portable) | A rubber-stamp map (every hunk cites one `AGENTS.md` span) and a careful map of the same 24-hunk patch both ACCEPT and their `citation` integers differ; tiers never change `valid`, `issues` or `drift`; `cem/0.1` carries no block |
| CEM-V1-081 | `TestCEMV1081IDBoundaryAndAnnotation`, `TestCEMV1081IDAnchorRequiresBoundedTokenAndDefinition`, `TestCEMV1081PathAnchorUsesPostImageAndBasenameFloor`, `TestCEMV1081Annotation` (native); `TestCEMV1081IDBoundaryAndAnnotation`, `TestCEMV1081RequirementIDGrammar` (portable) | A hunk adding ``x // `CEM-SM-007` `` against a span that mentions `CEM-SM-0070` and defines nothing is `cited`, not `anchored`; a hunk adding only that comment is `annotation: true`; the post-image path and the 6-byte basename floor decide `path` |
| CEM-V1-082 | `TestCEMV1082DisjointUmbrellaSpansHaveNoSpecificity`, `TestCEMV1082CitationIsDeterministic`, `TestCEMV1082BlockShapeAndCounts`, `TestCEMV1082ShiftNull`, `TestCEMV1082HunkOrderDoesNotChangeTheBlock` (native); `TestCEMV1082DisjointUmbrellaSpansHaveZeroSpecificity`, `TestCEMV1082CitationIsOrderIndependent`, `TestCEMV1082CountingRules`, `TestCEMV1082StableResultCarriesTheBlock` (portable); `interop/cem01-go/testdata/citation/*/citation.json` byte-matched against the native binary | `AGENTS.md` split into 20 disjoint spans, one per hunk, reports `specificity: 0`; native and portable blocks are byte-identical on four fixtures; the accepted stable result carries the same block as `citation` |
| CEM-V1-092 | `TestCEMV1092CitationOnlyOnAccept`, `TestCEMV1092SealedCasesCarryCitationOnlyOnAccept`, `TestCEMV1092RefuseAndRejectClearCitation` (native); `TestCEMV1092PortableCitationOnlyOnAccept`, `TestStableCanonicalFullResults` (portable); `TestStableS0EPublicCases` | The 56 sealed cases carry `citation` non-null on exactly their three ACCEPT cases and null on every refusal, including both drift-stage cases; both public no-Tasks cases and every public mutation of them obey the rule; a portable REJECT, evidence-drift REJECT and argument refusal carry `null` |

Rollback: remove `internal/cem/verify/support.go`, `interop/cem01-go/citation.go` with
`testdata/citation/`, and the `citation` key in `successVerification`; remove the `--citation`
flag and the `Citation` read option. No map, packet or protocol vector changes. CEM-V1-092 rolls
back by a Git revert of its regeneration run (the packet, cells, schema, manifests, tuple and the
two public case expectations) together with the consumer change.

## Provider-neutral criterion (V1-1263, decision 0523 P1)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. Decision 0523 P1. A stable criterion no longer requires a Corvint Tasks store.

- `CEM-V1-083`: (accepted by decision 0546, 2026-10-20) A `cem/1.0` criterion binding MUST take exactly one of two closed forms. The plain
  form is `{id, criterionIndex, criterionSha256, evidenceId}`: `evidenceId` MUST name an evidence
  record of the same map (else `unresolved-reference`), `criterionSha256` MUST equal that record's
  `spanSha256` (else `reference-coherence`), no two plain criteria may name the same evidence
  (`duplicate-reference`), and `id` is `criterion:sha256:` over the canonical JSON of the other
  three keys. The Tasks form is the existing eleven keys, unchanged in encoding, identity and
  artifact roles. Any of the eight Tasks keys present selects the whole Tasks form; a record is
  then checked for missing keys (`missing-field`), unknown keys (`unknown-field`) and invalid values
  (`invalid-field`) in that order, so plain keys beside a partial Tasks extension are `missing-field`
  and `evidenceId` beside the full Tasks form is `unknown-field`. A plain criterion contributes no
  Tasks artifact role, and a map with only plain criteria MUST verify on a repository with no
  `.taskman/`. The native and portable consumers MUST return the same result for the same inputs.

Non-goals: no new producer verb (a plain map is hand-assembled until a producer lands), no new
result key, no change to the S0E repository-envelope packet, no change to `cem/0.x`. Runner
receipts, criterion links and runner artifacts stay required by the existing coherence rules.

Failure modes: a mixed or partial record is refused at the wire stage before any repository read;
a plain criterion whose span digest or evidence reference is wrong is refused at the references
stage; artifacts are checked only by SHA-256, so an opaque artifact never upgrades authority.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-083 | `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083TasksFormUnchanged` (`internal/cem/verify`); `TestCEMV1083PlainCriterionWire`, `TestCEMV1083PlainCriterionRefusals` (`internal/cem/wire`); `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083PortablePlainDuplicateEvidence`, `TestCEMV1083PortableTasksFormUnchanged` (`interop/cem01-go`) | Public case `protocol/cem-1.0/stable/neutral-criterion/` ACCEPTs with the same non-runtime result on both consumers on a repository with no `.taskman/`; seven public refusals match on both; every S0E Tasks criterion re-encodes to its original eleven keys; walkthrough in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-neutral-criterion-walkthrough.md` |

Rollback: revert the plain branch in `internal/cem/wire/stable.go` and `interop/cem01-go/cem.go`
(restore the eleven required keys), the `criterionBinding` `oneOf` in `stable.schema.json`, the
CEM-V1-083 text in `ALGORITHMS.md`, the `neutral-criterion/` case and its manifest entries. Tasks
maps need no migration.

## Provider-neutral producer (V1-1290, decision 0523 P1)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; `CEM-V1-240` accepted under the owner's standing auto-accept
(2026-10-11); V1-1290. An experimental companion request, not promoted. It adds a producer for the
plain criterion form of `CEM-V1-083` and leaves that requirement's text unchanged.

- `CEM-V1-240`: (accepted by decision 0546, 2026-10-20) `corvint-cem-candidate assemble-stable --experimental --request R --out-dir D`
  MUST route a request whose `profile` is `cem-stable-plain-assembly/1` to a closed decoder of
  `{profile, repository, expectedBase, target, sourceMap, runnerPlan, runnerReceipt, criteria}`.
  Each pinned input is `{path, sha256}` with an absolute clean path and the raw SHA-256. Each of 1
  to 256 criteria is `{criterionEvidenceId, hunkIds, evidenceIds}`, with 1 to 32 IDs in each list.
  The request names no ticket, attempt, capture, snapshot or source prefix, and the producer reads
  no Corvint Tasks store. Every other profile keeps the Tasks-bound `cem-stable-assembly/1` path.
  The source map MUST be a `cem/1.0` producer map, as written by `corvint cem prepare` and
  `corvint cem cite`. It has no criterion binding, runner receipt, criterion link, artifact or
  provenance, and its change members and `role` are copied unchanged. Request criterion `i` becomes
  the plain criterion of `CEM-V1-083` with `criterionIndex` `i`. Its `evidenceId` is
  `criterionEvidenceId`, which MUST name an evidence record of the source map. Its
  `criterionSha256` is that record's `spanSha256`. It gets one criterion link
  `{criterionId, hunkIds, evidenceIds, runnerReceiptSha256s}` naming the one receipt.
  The map carries one runner receipt record with `profile` `opaque/0` (amended by `CEM-V1-310`;
  `corvint-test-runner-receipt/0` only under the admission of `CEM-V1-280`), the
  receipt and plan digests, `executionAuthority` `CALLER_OBSERVED`, and `sourceGitBinding`,
  `dependencyClosure` and `authentication` `NOT_OBSERVED`. It also carries the artifacts
  `runner-plan` and `runner-receipt`, at `artifacts/runner-plan.json` and
  `artifacts/runner-receipt.json`. The bundle D MUST be a new absolute directory outside the
  repository whose parent has no symlink alias. The producer writes the two artifacts into D and
  verifies the encoded map with the native stable verifier against D. It writes `D/stable.json`
  only after an ACCEPT, and only if every pinned input still matches its digest.
  From the recipe repository of `protocol/cem-1.0/stable/neutral-criterion/`, after `cem prepare`,
  one `cem cite` per hunk, and with that case's two runner artifacts, it MUST re-produce the case's
  `stable.json` and artifacts byte for byte, except that the produced runner receipt record is
  labelled `opaque/0` where the public case keeps its legacy label (`CEM-V1-310`).

Non-goals:
- no new or changed `corvint cem` verb, CLI frozen surface or MCP tool
- no change to the wire, the verifier, `CEM-V1-083` or the Tasks-bound request (the runner receipt
  label set is later widened by `CEM-V1-311`)
- no runner execution: the plan and receipt are opaque caller inputs, checked only by SHA-256 as
  the verifier checks them, unless the request opts into the receipt admission of `CEM-V1-280`
- no receipt signing (`FPK-V0-036`)
- no more than one runner receipt per map in the single `runnerPlan`/`runnerReceipt` form (amended
  by `CEM-V1-573`: the admitted `runners` form carries up to 64 receipts, one per pair)

The result `cem-stable-plain-assembly-result/1` reports `receiptAuthority` and `executionAtCommit`
as `NOT_OBSERVED`. An ACCEPT says nothing more than `CEM-V1-083` and the stable verifier already
say; decision 0523 §6 lists what it does not establish.

Failure modes:
- request shape, profile or pin form: refused before any read
- changed pinned input: refused, before the build and again before the map is written
- a source that is not a producer map: refused
- an unknown criterion evidence ID: refused
- duplicate criterion evidence: refused by the stable encoder (`duplicate-reference`)
- an unresolved hunk or evidence ID: refused by the stable encoder (`unresolved-reference`)
- a bundle inside the repository or already present: refused
- any verifier REFUSE or REJECT: refused, naming its stage and code

No refusal writes `stable.json`. A refusal after the bundle was created leaves at most the two
runner artifacts in D.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-240 | `TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase`, `TestCEMV1240PlainAssemblyRefusals`, `TestCEMV1240PlainAssemblyRechecksPinsAfterVerify` (`internal/cemcandidate`); `TestCEMV1240PlainProfileSelectsPlainDecoder` (`cmd/corvint-cem-candidate`) | The produced map, plan and receipt equal the public `neutral-criterion` case byte for byte (the map with its runner label replaced by `opaque/0`, `CEM-V1-310`), on a recipe repository with no `.taskman/`. The native verifier ACCEPTs it inside the producer. Each pinned input changed after that ACCEPT is refused with no `stable.json`. The public case's `TestCEMV1083NeutralCriterionPublicCase` ACCEPTs the same bytes on both consumers. The timed producer and both-consumer run is in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-cem-plain-criterion-producer.md`. |

Rollback: remove `internal/cemcandidate/plain.go`, the profile dispatch in
`cmd/corvint-cem-candidate/stable.go`, and their tests. Plain maps already produced stay valid
`cem/1.0` maps under `CEM-V1-083`.

## Runner receipt authentication (V1-1264, decision 0523 P1)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. Decision 0523 P1. A runner receipt can carry a signed attestation, and only the
consumer decides which keys it trusts.

- `CEM-V1-084`: (accepted by decision 0546, 2026-10-20) A runner receipt's `authentication` MUST take one of three closed forms: the sealed
  legacy string `"NOT_OBSERVED"` (read as method none and echoed unchanged), `{"method":"none"}`,
  or `{"method":"dsse-ed25519","keyid":K,"envelopeSha256":E}`, where K is the lowercase hex SHA-256
  of the raw 32-byte Ed25519 public key and E is the SHA-256 of the exact envelope bytes. The method
  selects the key set, so a member is refused `missing-field`, then `unknown-field`, then
  `invalid-field`. E MUST resolve to a `receipt-attestation` artifact (`artifact-role-closure`),
  no two receipts may name the same E (`duplicate-reference`), and a `receipt-attestation` artifact
  no receipt names is unused (`artifact-role-closure`). E's bytes MUST be one closed DSSE envelope
  (`payloadType` `application/vnd.in-toto+json`, exactly one signature) over a closed in-toto
  Statement v1 with `predicateType` `https://corvint-context.dev/attestation/cem-receipt/v1`, an
  empty predicate and exactly the subjects `runner-receipt` (sha256), `runner-plan` (sha256) and
  `target-tree` (gitTree), in that order. After both artifact passes a consumer MUST refuse, at
  stage `artifacts` with exit 1 and `authentication` FAILED, `attestation-malformed`,
  `attestation-key-mismatch` (signature keyid differs from K), `attestation-subject-mismatch`
  (receipt, plan or target tree differs, checked with or without trust) and, for a trusted K only,
  `attestation-signature-invalid`. Trust comes only from `--trusted-keys FILE`: an absolute regular
  file of at most 64 KiB holding `{"profile":"corvint-trusted-keys/1","keys":[{"keyid":K,
  "ed25519PublicKey":B64}]}` with 1..64 unique keys, each keyid the digest of its key; any other
  file is `invalid-arguments` before the map is read. On ACCEPT the axis is NOT_OBSERVED without
  that file, UNTRUSTED_KEY when any signed receipt names an untrusted keyid, NOT_OBSERVED when any
  receipt is unsigned, and VERIFIED only when every receipt is signed by a trusted key. A key the
  map or its artifacts carry is never trusted, and a signature never raises `executionAtCommit`.
  The native and portable consumers MUST return the same result for the same inputs.

Non-goals: no signing producer and no key management (signed production is an operator step,
FPK-V0-036); no network, transparency log, certificate or key discovery; no change to
`RESULT.schema.json`, the sealed S0E packet or `cem/0.x`; no change to the candidate profile.

Failure modes: a malformed member is refused at the wire stage and an unresolved or repeated
envelope at the references stage, before any artifact read; an envelope over another receipt,
plan or tree is refused even without a trusted-keys file; a bad signature under an untrusted key
is not checked and leaves UNTRUSTED_KEY, never VERIFIED; a bad trusted-keys file refuses the whole
run rather than silently lowering the axis.

Decision (V1-1291, 2026-10-14): that `invalid-arguments` refusal, like every `arguments` and
`authority-arguments` refusal, is `UNSUPPORTED` with `accept` `null` and exit 2 in both consumers
(CEM-V1-115, CEM-V1-116). A relative `--trusted-keys` path also prints the absolute-path fix on
stderr (CEM-V1-117).

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-084 | `TestCEMV1084ReceiptAuthenticationFixtures`, `TestCEMV1084ReceiptAuthenticationPublicCase` (`internal/cem/verify`); `TestCEMV1084AuthenticationForms`, `TestCEMV1084AuthenticationRefusals`, `TestCEMV1084ReceiptAttestationEnvelope`, `TestCEMV1084TrustedKeysFile` (`internal/cem/wire`); `TestCEMV1084PortableReceiptAuthenticationPublicCase`, `TestCEMV1084PortableTrustedKeysArgument` (`interop/cem01-go`); `TestCEMV1084VerifyStableTrustedKeysFlag`, `TestCEMV1084HelpNamesTrustedKeys` (`cmd/corvint`); `TestCEMV1084ConsoleReceiptAuthentication` (`internal/console`) | Public case `protocol/cem-1.0/stable/receipt-authentication/` ACCEPTs VERIFIED with the same non-runtime result on both consumers; five axis cases and thirteen refusals match on the native verifier and both portable commands; a self-carried key and a signature over the base tree are refused; walkthrough in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-receipt-authentication.md` |

Rollback: revert `stable_auth.go` in `internal/cem/wire`, `internal/cem/verify` and
`interop/cem01-go`, the `--trusted-keys` flag, the `authentication` `oneOf` and the
`receipt-attestation` kind in `stable.schema.json`, the CEM-V1-084 text in `ALGORITHMS.md`, the
`receipt-authentication/` case and its manifest entries. Maps with the legacy string need no
migration; a map with an object form is refused again after rollback.

## Verdict digest (V1-1265, decision 0523 P1)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. Decision 0523 P1. A stable result separates the verdict from the environment that
produced it, so a later consumer can bind the verdict alone.

- `CEM-V1-066`: (accepted by decision 0546, 2026-10-20) Every `cem-stable-verification/1` result MUST carry `verdictSha256` after
  `repositoryEnvelope`: the lowercase hex SHA-256 of the canonical JSON value (sorted keys, minimal
  separators, decimal integers, no terminal LF) of the result object without `runtime`,
  `repositoryEnvelope` and `verdictSha256`. It MUST be computed on every outcome, ACCEPT, REJECT and
  UNSUPPORTED, and is never null. The excluded members are environment: re-verification of the same
  inputs on another admitted Go tuple, or under another repository-envelope label, MUST yield the
  same digest, while a change to any other member MUST change it. The native and portable
  consumers MUST emit the same digest for every public case (S0E cases, envelope cells,
  `neutral-criterion` and `receipt-authentication`); a disagreement is a consumer defect, never a
  reason to edit the sealed value. Later consumers (decision 0525 step 8 and the Tasks gate,
  V1-1283) consume `verdictSha256` rather than hashing the whole result.

Non-goals: no signature or attestation over the digest, no change to the verdict members themselves,
no digest in `cem/0.x` results, and no consumer of the digest in this change.

Failure modes: an emitted digest that differs from the recomputed digest of the emitted result, a
digest that moves with `runtime` or `repositoryEnvelope`, or a non-integer number in the result
(refused rather than rounded) is a consumer defect; a value held in the result struct is never
emitted, because both consumers recompute the digest at encoding.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-066 | `TestCEMV1066SealedVerdictDigests`, `TestCEMV1066VerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066EveryOutcomeCarriesVerdictDigest` (`internal/cem/verify`); `TestCEMV1066PortableSealedVerdictDigests`, `TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest`, `TestStableCanonicalFullResults` (`interop/cem01-go`) | The native capture of 54 S0E cases equals an independent recomputation from the sealed expectations; every sealed case, cell and public ACCEPT digest recomputes by hand on both consumers; each verdict member moves the digest and the three excluded members do not; the public refusals, axis cases and a REJECT, drift and UNSUPPORTED fixture each carry their own digest |

Rollback: revert the `verdictSha256` regeneration (the packet files, both schema copies, the
cells, the tuple packet and their manifests), the public `EXPECTED.json` members and stable
manifest entries, `stable_verdict.go`, `interop/cem01-go/verdict.go` and the key-count changes in
both consumers. No map changes; the Tasks verdict gate reads the digest (TCB-V0-012, TCB-V0-017).

## Stable map provenance, producer-only members and role (V1-1265, decision 0525)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. Decision 0525 §3 step 3 and §4. A converted map can say where it came from and
why its change exists, and a producer without task or runner evidence has a closed list of members
it emits empty.

- `CEM-V1-093`: (accepted by decision 0546, 2026-10-20) A stable map MAY carry `provenance`, a closed object with exactly the required
  members `sourceSpec` (`cem/0.1`, `cem/0.2` or `cem/0.3`), `sourceBlobOid` (40 lowercase hex),
  `sourceSha256` (64 lowercase hex), `bindCommit` (40 lowercase hex) and `converter` (matching
  `^corvint-cem-convert/[0-9]+(\.[0-9]+)*$`). A native map omits it. A non-object value is
  `invalid-field`; inside the object a missing member is `missing-field`, then an unknown member
  `unknown-field`, then an invalid value `invalid-field`, in that member order. Root optional
  members are validated after every required member and before the reference graph, `provenance`
  before `role`. Provenance MUST NOT be read for authority, ranking, any result axis or verdict, and
  the result MUST NOT echo it; it affects only the map bytes and so `mapSha256` and, through it, `verdictSha256`. The native and
  portable consumers MUST refuse and accept the same documents with the same codes.
- `CEM-V1-094`: (accepted by decision 0546, 2026-10-20) The producer-only stable map members are exactly this closed list:
  1. `criterionBindings`;
  2. `runnerReceipts` (including a receipt's `authentication`);
  3. `criterionLinks`;
  4. `artifacts` (including `receipt-attestation` records).

  A producer with no task or runner evidence, including the 0.x converter, MUST emit each as an
  empty array, and both consumers MUST ACCEPT such a map with `referenceIntegrity`
  `EMPTY_REFERENCE_SET` and every non-core axis `NOT_OBSERVED`. There is no "intent and scope"
  member and no "structural witness" member: intent stays in project specs and tickets, and the
  structural reasons are recomputed from Git. Hunk `coverage` and `discriminates` are not
  producer-only; they are optional source witnesses carried unchanged when present, omitted when
  absent and never synthesized.
- `CEM-V1-095`: (accepted by decision 0546, 2026-10-20) A stable map MAY carry `role`, a closed string enum of `change` (the meaning when
  absent), `bind-chore` and `seal-chore`. Any other value is `invalid-field`. Role MUST NOT change a
  verdict, tier or result axis, and the result MUST NOT echo it.

Re-homing: `protocol/cem-1.0/stable/ALGORITHMS.md` now restates, in cem/1.0 vocabulary, the 0.1
algorithms (strict JSON, paths, canonical identity JSON, evidence and hunk IDs, the patch profile
and parser, base simulation, evidence and disposition checks, text mechanical reasons, limits and
drift) and the 0.3 hunk vocabulary, structural reasons and witnesses, citing CEM-SM-001..010 in
`docs/specs/cem-0.3-structural-mechanical.md` as references. It names the 13 `$defs` the stable
schema restates from `inherited/cem-0.3.schema.json`. The `inherited/` copies are retained unchanged
as historical provenance; `ALGORITHMS.md` is the 1.0-normative text and governs where they differ.
The 0.1/0.2/0.3 requirement IDs keep their definitions in their own specs.

Non-goals: no converter here (V1-1275 adds it as CEM-V1-096..105 below); reader grouping by role is `CEM-V1-106` (`cem status --citation`,
V1-1281); no "intent and scope" or "structural witness" member; no change to `RESULT.schema.json`,
the stable result struct, the sealed S0E packet or any `inherited/` file; no SHA-256 repository
form of the provenance OIDs.

Failure modes: a malformed `provenance` or `role` is refused at the wire stage before any reference
or Git check, so a map cannot reach a verdict through an ill-formed optional member; a provenance
that names a source the repository does not hold is not checked, because provenance is never an
input to authority; an all-empty map is accepted but reports `EMPTY_REFERENCE_SET`, never task or
runner evidence it does not carry.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-093 | `TestCEMV1093StableMemberVectors`, `TestCEMV1093ProvenanceDecoded`, `TestCEMV1093SchemaMembers` (`internal/cem/wire`); `TestCEMV1093PortableStableMemberVectors` (`interop/cem01-go`) | 50 shared vectors in `interop/cem01-go/testdata/stable-map-members/cases.json` (10 accepts, 40 refusals including member and stage ordering) give the same code on both consumers; accepted maps round-trip losslessly through `EncodeStable`; schema members match the Go constants; build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-stable-map-provenance.md` |
| CEM-V1-094 | `TestCEMV1094ProducerOnlyEmpty` (`internal/cem/wire`); `TestCEMV1094ProducerOnlyEmptyVerifyAccept` (`internal/cem/wire`, through `internal/cem/verify`); `TestCEMV1094PortableProducerOnlyEmptyAccept` (`interop/cem01-go`) | An all-empty map over the neutral-criterion repository ACCEPTs on both consumers with `EMPTY_REFERENCE_SET`, change integrity VERIFIED and every non-core axis NOT_OBSERVED; adding `provenance` and `role` changes only `mapSha256` and, through it, `verdictSha256` |
| CEM-V1-095 | `TestCEMV1095Role` (`internal/cem/wire`); `TestCEMV1095PortableRoleAndProvenanceDecoded` (`interop/cem01-go`) | Each role decodes and re-encodes unchanged on both consumers and an absent role is effective `change`; six role refusals are `invalid-field` on both consumers |

Rollback: revert `stable_provenance.go` in `internal/cem/wire` and `interop/cem01-go`, the root
key change in both decoders and `EncodeStable`, the `provenance`, `role` and `sha1Oid` definitions
in `stable.schema.json`, the CEM-V1-093..095 and re-homed sections of `ALGORITHMS.md`, the README
bullets and their manifest entries. A map without the optional members needs no migration; a map
carrying either is refused `unknown-field` again after rollback.

## cem/0.x -> cem/1.0 converter (V1-1275, decision 0525 §4)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; satisfied by the frozen V1-1277 evidence and removed by V1-1278 (`CEM-V1-204`),
so the converter no longer ships and the tests named below are retired with it; their runs are
retained in `https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1275/` and `https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1277/`. Decision 0525
§4 "Converter contract" and §5 "Sealed-map rule". The converter
turns one committed cem/0.1, cem/0.2 or cem/0.3 map into the cem/1.0 map CEM-V1-093..095 shape,
as a pure function of the source bytes and the bind commit, so that every committed map can move
to cem/1.0 (V1-1276/V1-1277) without a verdict changing. It lives in `internal/cem/convert` and
behind `corvint cem convert`.

- `CEM-V1-096`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The converter input is the exact bytes of one map. A map that declares
  `cem/1.0` is refused `convert-source-is-stable` before any other check. Otherwise the bytes are
  read by the existing 0.x reader (`wire.ParseMap`), which accepts `cem/0.1`, `cem/0.2` and
  `cem/0.3`; a wire refusal keeps that reader's own code (for example `invalid-json`,
  `unknown-field`, `orphan-evidence`). A source with zero hunks is refused `convert-empty-hunks`,
  because a stable map cannot carry an empty `hunks` array.
- `CEM-V1-097`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The output is exactly the `wire.EncodeStable` bytes of this stable map, which
  `ParseStable` accepts:
  1. `spec` is `cem/1.0`;
  2. `baseRevision`, `patchSha256`, `evidence` and `hunks` are the source values, carried member
     for member. Hunk `disposition`, `reason`, `basis`, ranges and IDs are unchanged, and hunk
     `coverage` and `discriminates` (cem/0.3) are carried unchanged when present and omitted when
     absent, never synthesized;
  3. `excludedPath` is the protocol literal `.corvint/change.cem.json` (a cem/0.1 source has no
     such member; 0.2 and 0.3 sources already carry the same literal);
  4. the CEM-V1-094 producer-only members `criterionBindings`, `runnerReceipts`, `criterionLinks`
     and `artifacts` are empty arrays;
  5. `provenance` is the CEM-V1-098 object;
  6. `role` is omitted (effective `change`); grouping by role is V1-1281.

  No tier, axis, reason or witness is computed: tiers stay consumer-computed. Nothing in the
  output is read from Git.
- `CEM-V1-098`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) `provenance.sourceSpec` is the source `spec`; `sourceBlobOid` is the Git SHA-1
  blob object name of the exact source bytes; `sourceSha256` is the lowercase hex SHA-256 of the
  same bytes; `bindCommit` is the commit the source file name pins (`<40 lowercase
  hex>.cem.json`), or the explicit `--bind-commit` value, which takes precedence; `converter` is
  `corvint-cem-convert/1`. A missing or malformed bind commit is refused
  `convert-bind-commit-unavailable`; the converter never guesses one.
- `CEM-V1-099`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) When a repository is given, the converter reads one Git object path: the blob the
  bind commit holds at `.corvint/change.cem.json`. The report states the source binding as `EXACT`
  (a regular file whose blob OID equals `sourceBlobOid`), `ABSENT` (the commit resolves but holds
  no regular file there), `UNAVAILABLE` (the commit or its trees cannot be read), `MISMATCH` (a
  different blob) or `NOT_CHECKED` (no repository). Only `MISMATCH` refuses
  (`convert-source-binding-mismatch`); the output bytes are the same for every other state.
- `CEM-V1-100`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) Conversion is deterministic and idempotent at the byte level: converting the same
  bytes with the same bind commit twice gives identical output, on any host and with or without a
  repository. Converting the output again is refused `convert-source-is-stable`, never rewritten.
- `CEM-V1-101`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The converter's own refusal codes are exactly `convert-source-is-stable`,
  `convert-empty-hunks`, `convert-bind-commit-unavailable`, `convert-source-binding-mismatch` and
  `convert-encode-refused` (the encoder refused the assembled map; not reachable from an accepted
  source). A refusal emits no map bytes and no provenance.
- `CEM-V1-102`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) `corvint cem convert --map MAP (--output MAP | --in-place) [--bind-commit REV]
  [--repository PATH]` takes absolute clean paths, converts one map and prints one closed
  `corvint-cem-convert-report/1` JSON object: `profile`, `status` (`CONVERTED` or `REFUSED`),
  `code`, `converter`, `sourceSpec`, `sourceBinding`, `provenance`, `hunkCount`, `evidenceCount`
  and `outputSha256`. `--output` never replaces an existing file; `--in-place` replaces the source
  through a sibling temporary file and one rename. Exit 0 is converted, 1 is a CEM-V1-096 or
  CEM-V1-101 refusal, 2 is `invalid-arguments` (including a root option before `cem convert`),
  `map-unavailable` (the map is missing, not a regular file or over the map size limit) or
  `publish-failed` (the output could not be written).
- `CEM-V1-103`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) A map whose pinned objects are unavailable still converts from its bytes alone
  (binding `UNAVAILABLE`, output identical to a reachable conversion). Verifying it gives an
  explicit `UNSUPPORTED` result at the `repository` stage on both consumers, never a verdict; the
  observed code was `git-read-failed`, the same code the 0.2 verifier gives the source. Amended by
  V1-1297 (decision 0535, owner delegated to the orchestrator 2026-10-16, superseding the decision
  0532 hold): a pinned object that is absent locally MUST be `UNSUPPORTED` `repository`
  `repository-object-unavailable` on both consumers. When Git cannot resolve a pinned full object
  ID, each consumer probes `git cat-file -e <oid>`: only a probe that exits normally with status 1
  is `repository-object-unavailable`; a present but unreadable or non-commit object, a native name
  that is not a full object ID (the portable consumer resolves only wire-validated object IDs), a
  probe that exits with any other status or by a signal (128 for a repository Git cannot open or a
  name it cannot parse), and a probe that itself fails at the owner, deadline, parser or launch stay
  `git-read-failed` or the probe's own code. The portable consumer charges the probe as an
  `object-existence-probe` ledger operation. The sealed S0E cells `missing-base`,
  `shallow-missing` and `missing-promisor` were re-emitted with that code and its verdict digest
  (`CEM-V1-091`); `corrupt-base` keeps `git-read-failed`.
- `CEM-V1-104`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) Verdict parity is measured per committed map at the substituted target T', the
  bind commit's tree with only `.corvint/change.cem.json` replaced by the converted bytes, written
  only into a scratch clone. The 0.x native verifier runs on the source at the bind commit; the
  native stable verifier and `interop/cem01-go verify-stable-canonical` run on the converted map
  at T' with the source `baseRevision` as the expected base. Compared: outcome, stage, the reason
  or issue code (0.x repository codes mapped to their stable names), `changeIntegrity`, the
  sorted drift list and the full per-hunk array (disposition, reason, basis, ranges, IDs and
  witnesses). Excluded: `mapSha256`, `verdictSha256` and `citation`, which bind the map bytes and
  so differ by construction; the reference, criterion, runner and artifact axes, which a 0.x
  result does not have (CEM-V1-094 makes them `EMPTY_REFERENCE_SET` and `NOT_OBSERVED`). Each row
  is classed `identical`, `empty-member-explained` (only a references/artifacts stage explains the
  difference), `unavailable-objects`, `pre-existing-refusal` (both consumers repeat the source's
  `UNSUPPORTED` code, stage and hunks), `NOT_OBSERVED` (the bind commit is absent from the clone)
  or `different`. Recorded limits: direct verification at the bind commit itself is `REJECT
  excluded-artifact-mismatch` on both consumers for every map, because the bind commit's sidecar
  holds the 0.x bytes (§5 sealing replaces the sidecar, not the bind commit); and a cem/0.1 source
  carries an out-of-band patch digest that the canonical diff does not reproduce, so its
  conversion verifies as `REJECT` `patch-digest-mismatch`, the same result `verify.Exact` gives the
  source with canonical-diff bytes. cem/1.0 has no out-of-band patch member; the committed corpus
  holds no cem/0.1 map. Decision 0532 (V1-1299) keeps this recorded limit and adds no `CEM-V1-101`
  refusal code: the converter no longer ships (`CEM-V1-204`), so no producer can reach the case.
  Decision 0535 closes V1-1299 as obsolete on that basis.
- `CEM-V1-105`: (accepted by decision 0546, 2026-10-20) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The converter exists only to migrate cem/0.x maps. It is removed with the 0.x
  readers by V1-1278; after that removal `corvint cem convert` is an unknown action and no
  cem/1.0 producer depends on it. V1-1278 S1 removes the converter ahead of the 0.x readers
  (`CEM-V1-204`).

Non-goals: no producer-only evidence (criterion bindings, runner receipts, links, artifacts) is
synthesized; no `role` is assigned; no tier, structural reason or witness is recomputed into the
map; no rewrite of committed maps or bind commits (V1-1276/V1-1277 own the corpus migration and
the §5 sealing commit); no cem/1.0 to 0.x converter; no SHA-256 repository form of the
provenance OIDs; no change to either stable consumer.

Failure modes: a 1.0 or malformed source, an empty hunk list or a missing bind commit is refused
with a bounded code and no output; a sidecar that is a different blob at the bind commit refuses
instead of attributing a map to the wrong commit; an unreadable repository never changes the
output and never becomes a verdict; an existing `--output` file is never overwritten; a
consumer-only divergence on a resource-limit abort (the partial drift list) is retained as a
`nativePortableDiffer` row, not hidden by the class. V1-1295 (2026-10-14) kept that partial drift
list in the result: the sealed S0E full-result cases `ledger-337-evidence` and
`partial-drift-one-item-cancel` pin a partial list on `UNSUPPORTED` results (335 items on a
resource limit, one on a timeout), so the row stays a recorded difference
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-batch4-cem-parity.md`). V1-1295 (2026-10-15) traced the difference to
the native drift stage re-reading each changed record's base blob; `CEM-V1-260` removes the re-read
and both consumers now write the same 102 rows and `verdictSha256` (decision 0532).

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-096 | `TestCEMV1096ConvertAcceptsEachZeroDialect` (`internal/cem/convert`) | cem/0.1, cem/0.2 and cem/0.3 fixtures convert to their goldens under `internal/cem/convert/testdata/` |
| CEM-V1-097 | `TestCEMV1097ConvertMemberMapping` (`internal/cem/convert`) | Carried members equal the source; producer-only members empty; `role` absent; 0.3 witnesses carried, absent witnesses not synthesized |
| CEM-V1-098 | `TestCEMV1098ConvertProvenanceValues` (`internal/cem/convert`) | Each provenance member equals its derived value in the map and the report; the empty input gives Git's empty-blob OID; a file name pins only 40 lowercase hex |
| CEM-V1-099 | `TestCEMV1099ConvertSourceBinding`, `TestCEMV1099ConvertGitReaderStates` (`internal/cem/convert`) | Five binding states over a fake reader and real repositories; only MISMATCH refuses; bytes equal across states |
| CEM-V1-100 | `TestCEMV1100ConvertCorpusIdempotent`, `TestCEMV1100ConvertF0fda542` (`internal/cem/convert`) | All 603 committed maps converted twice to identical bytes and reconversion refuses `convert-source-is-stable`; the 877-hunk f0fda542 map converts. Since the V1-1277 migration the committed maps are `cem/1.0`, and the same test reconverts each migrated map's source blob to its committed bytes (`CEM-V1-230`) |
| CEM-V1-101 | `TestCEMV1101ConvertRefusalCodes` (`internal/cem/convert`) | Each reachable converter code from its trigger with no output or provenance; malformed JSON keeps the reader's `invalid-json` |
| CEM-V1-102 | `TestCEMV1102CEMConvertCommand` (`cmd/corvint`) | Golden output, exits 0/1/2, no overwrite of an existing output, `--in-place` with `--repository`, unnamed bind refused, argument and missing-map refusals |
| CEM-V1-103 | `TestCEMV1103ConvertUnavailableObjects` (`internal/cem/convert`); `TestCEMV1104PortableConvertedFixtures` (`interop/cem01-go`) | Empty repository: binding UNAVAILABLE, output equals golden, both consumers UNSUPPORTED `git-read-failed` as the 0.2 source (frozen V1-1277 evidence). V1-1297 (decision 0535): `TestResolveMissingPinnedObjectUnavailable` and `TestResolveProbeOnlyExitOneIsAbsent` (`internal/cem/gitauth`), `TestStableResolveMissingPinnedObjectUnavailable` (including a probe exit 128) and `TestStableCanonicalEnvelopeCells` (`interop/cem01-go`) on the re-emitted cells `missing-base`, `shallow-missing` and `missing-promisor`; `TestCEMV1066SealedVerdictDigests` re-derives their verdict digests |
| CEM-V1-104 | `TestCEMV1104ConvertParityCEM03`, `TestCEMV1104ConvertParityCEM01` (`internal/cem/convert`); `TestCEMV1104PortableConvertedFixtures` (`interop/cem01-go`); `TestCEMV1104ConvertParitySample`, `TestCEMV1104ConvertParityClassify` (`tools/cem-convert-parity`) | Full corpus run `https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1275/` (603 rows: 602 `identical`, 1 `pre-existing-refusal`, 0 `different`, 0 `NOT_OBSERVED`); build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-convert.md`. Since V1-1277 the sample test runs with `--migrated` over the committed `cem/1.0` maps (`CEM-V1-230`) |
| CEM-V1-105 | `TestCEMV1204StableDispatchNoLongerCapturesConvert` (`internal/cem/cli`) | V1-1278 S1 removed `internal/cem/convert`, `tools/cem-convert-parity`, `script/cem-migrate-maps.sh` and the `cem convert` action (`CEM-V1-204`) |

Rollback: revert `internal/cem/convert`, `internal/cem/cli/convert.go`, the `convert` dispatch in
`internal/cem/cli/stable.go`, the help lines in `cmd/corvint/help.go`,
`tools/cem-convert-parity`, `interop/cem01-go/convert_fixture_test.go` and this section. No
committed map, schema, consumer or sealed packet changes, so nothing needs migration back; a map
already converted stays a valid cem/1.0 map, and its source bytes are recoverable from Git by
`provenance.sourceBlobOid`.

## Status citation class (V1-1281)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1281. The dogfood bind loop
marks a self-referential bind or seal map with its role (`DCW-V0-049`), and a reader of the citation
measure needs to keep those maps apart from change maps without reading the role into the result.

- `CEM-V1-106`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1281) `cem status
  --citation` on a `cem/1.0` map whose wire parses MUST add the envelope member `citationClass`
  beside, never inside, `stableVerification`: an object with exactly `class` and `role`, where
  `role` is the map's effective role (`CEM-V1-095`) and `class` is `change` for role `change` and
  `chore` for `bind-chore` or `seal-chore`. Status without `--citation`, a map whose wire does not
  parse, and every other read keep their envelope unchanged. `stableVerification` stays exactly
  what `cem verify-stable` reports for the same arguments; the class is not a result member, so
  "the result MUST NOT echo it" in `CEM-V1-095` still holds, and the role changes no outcome,
  issue, tier or citation block. Amended 2026-10-13 (V1-1344): for role `change`, `class` is
  `record` when `CEM-V1-170` holds, otherwise `change`.
- `CEM-V1-170`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1344) For a map
  whose effective role is `change`, `citationClass.class` MUST be `record` when its hunk paths name
  at least one path under `docs/build-log/` or `.agent-evidence/` (nonempty remainder) and every
  other hunk path is `.corvint/change.cem.json` or `.corvint/changes/NAME.cem.json`, NAME one
  nonempty segment (`DCW-V0-057`); `role` stays `change`. A bind-chore or seal-chore map stays
  `chore` whatever its paths, and a map with no hunks is `change`.

Non-goals: no `role` or class in `RESULT.schema.json`, the stable result struct, the 0.x
verification object or the portable consumer; no class for a `cem/0.x` map, which has no role; no
`record` role value.

Failure modes: a map that fails verification still reports its class when its wire parses, so a
rejected chore map stays out of the change-map measure; a map whose wire does not parse has no
class, and a reader counts it as an error row, never as either class.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-170 | `TestCEMV1170StatusCitationClassRecord` (`internal/cem/cli`); `TestDogfoodDCWV0058RecordChangeBindsChecksAndSeals` (`cmd/corvint`) | Rewritten hunk paths under `docs/build-log/` or `.agent-evidence/`, alone or with a sidecar path, give class `record` with role `change`; a code path or only a sidecar path gives `change`; a bind-chore role gives `chore`; a dogfood-sealed record map reads `record`; build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-13-v1-1344-record-citation-class.md` |
| CEM-V1-106 | `TestCEMV1106StatusCitationClassGroupsChoreMaps` (`internal/cem/cli`) | Status without `--citation` has no `citationClass`; with it, a map with no role is class `change`, and re-encoded roles `change`, `bind-chore` and `seal-chore` give their class; each `stableVerification` equals `cem verify-stable` with no `role` key, and the outcome, issue codes and citation block match across roles; build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-dogfood-per-hunk-citations.md` |

Rollback: revert the `--citation` branch of the stable `status` read and `StableCitationClass` in
`internal/cem/workflow/stable_read.go`; the envelope loses `citationClass`, and no map, result or
sealed artifact changes. Roll back `CEM-V1-170` alone by reverting `stableRecordPaths`: record maps
read class `change` again.

## Committed map migration (V1-1277, decision 0525 §3 step 6)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1277. Every committed
`.corvint/changes/<bind>.cem.json` map moves to `cem/1.0` in a migration commit, so the sealed
corpus is read by the stable consumers only and the 0.x readers can be removed (V1-1278).

- `CEM-V1-230`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships, so no follow-up migration commit can be written; a later `cem/0.x` seal follows `CEM-V1-203`, and the committed corpus is checked by `CEM-V1-201`) A migration commit MUST rewrite every committed
  `.corvint/changes/<bind>.cem.json` map that declares `cem/0.x` to the exact bytes `corvint cem
  convert --in-place` writes for it (`CEM-V1-096..102`), keeping the file name, and MUST leave
  every map that already declares `cem/1.0` byte-identical; the commit changes no other path.
  `script/cem-migrate-maps.sh` performs the rewrite. It refuses a repository that is not the top
  level of a Git work tree or that holds no map, stops on the first refusal with the converter's
  report, and prints `converted=N unchanged=N refused=N`. It converts only non-1.0 maps, so it is
  rerun after a pre-cutover branch lands further `cem/0.x` maps; those rewrites form a follow-up
  migration commit (`DCW-V0-050`). The migration check is reproducible from the committed tree:
  for each map that carries converter provenance, `TestCEMV1100ConvertCorpusIdempotent` and
  `cem-convert-parity --migrated` read the source blob named by `provenance.sourceBlobOid`,
  require `bindCommit` to equal the file name and the blob's SHA-1 and SHA-256 to equal the
  recorded digests, reconvert it, and require byte equality with the committed file. The test
  reports `NOT_RUN` (skip) when a Git object it reads is absent from the checkout's object store,
  never a pass.
- `CEM-V1-231`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships; `CEM-V1-201` keeps the native-map byte check) A committed `cem/1.0` map without converter provenance is accepted as
  natively produced only when it is byte-identical to the `.corvint/change.cem.json` blob its bind
  commit tracks, the immutable evidence that it was bound with those bytes. Otherwise
  `cem-convert-parity --migrated` fails the row `native-bytes-not-bound` and
  `TestCEMV1100ConvertCorpusIdempotent` fails, so removing the provenance of a migrated map cannot
  bypass the source-blob equality check. A native map that passes is verified at its bind commit
  on both stable consumers and must refuse reconversion.
- `CEM-V1-232`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) The migration is accepted only when, over every committed map, both stable
  consumers re-verify the migrated bytes with no `different` parity row (`CEM-V1-104`, at T'),
  and the per-map citation rows measured at T' after the migration equal the rows measured before
  it in every member except `spec`. The one admitted exception is bounded: a row that is invalid
  on both sides and has no citation block may differ in its `issues` member only when the
  stable issue code is the `CEM-V1-104` mapping of the 0.x code (`refusalCode`,
  `internal/cem/verify/stable.go`), with the message following the code. Any other difference
  stops the migration.

Non-goals: no re-binding of any bind commit or sidecar; no `role` on migrated maps; no change to
either consumer, the converter or the schemas; no change to `tools/cem-citation-baseline` in this
lane (its `cem/1.0` `--artifacts` root is `CEM-V1-146`/`CEM-V1-147`, V1-1281).

Failure modes: a refused conversion stops the run and leaves no migration commit; a migrated map
verified directly at its bind commit is `REJECT excluded-artifact-mismatch` (`CEM-V1-104`), so
every check runs at T', and a citation measurement that verifies at the bind commit reads all
migrated maps as invalid; a Git object missing from a shallow clone is `NOT_RUN`, never a pass;
a rebase that rewrites a migration commit invalidates the `DCW-V0-050` seal baseline, which then
fails closed.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-230 | `TestCEMV1100ConvertCorpusIdempotent` (`internal/cem/convert`); `TestCEMV1104ConvertParitySample`, `TestCEMV1230MigratedSourceChecks` (`tools/cem-convert-parity`) | Migration commit `0b6f1bc245b44a8620c5e64f91e87f6d08da08d0`: `converted=614 unchanged=1 refused=0`; a rerun over the migrated tree prints `converted=0 unchanged=615 refused=0`; an empty inventory exits 1 and a non-repository exits 2; the corpus test reconverts 614 migrated maps from their source blobs to the committed bytes, and a one-byte edit of a committed map fails it; `--migrated` run `https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1277/` (`committed`: 614 `migrated`, 1 `stable-native`; `committedEqualsConverted` 614) |
| CEM-V1-231 | `TestCEMV1230MigratedSourceChecks` (`tools/cem-convert-parity`); `TestCEMV1100ConvertCorpusIdempotent` (`internal/cem/convert`) | A stable map without provenance whose bind commit tracks other bytes fails `native-bytes-not-bound`; 860f59ea equals its bind commit's sidecar blob `495159656e689a037e73448d49f72d97a0a76e94` |
| CEM-V1-232 | Full-corpus runs (`tools/cem-convert-parity --migrated`; citation rows at T') | 615 parity rows: 613 `identical`, 1 `native-stable`, 1 `pre-existing-refusal` (f0fda542, as in V1-1275), 0 `different`, 0 `NOT_OBSERVED`. Citation rows: the pre-migration rows equal the V1-1281 reference (`https://github.com/beamfall/corvint-history/tree/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1281/`) in every member for all 615 maps; the T' rows equal them except `spec` for 614 maps, and f0fda542 differs only by the admitted `git-budget-exceeded` to `unsupported-resource-limit` mapping. The T' rows were measured with the `--targets` tool of commit `ffc6c194b`, reverted in favour of V1-1281's tool; that tool verifies at the bind commit and reads the 614 migrated maps as `excluded-artifact-mismatch` (`citation-post-1281tool.jsonl`); build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1277-cem-migration.md` |

Rollback: revert the V1-1277 merge commit. The maps return to their `cem/0.2` bytes, which the
0.x readers still verify until V1-1278 removes them; after V1-1278 S1 such a revert also needs the
`CEM-V1-201` corpus check reverted; the `DCW-V0-050` migration-commit list goes with it.

## Core cem/0.x retirement (V1-1278 S1, decision 0525 §3 step 7 and §6)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; V1-1278 stage S1. Decision 0525 removes `cem/0.x` once the migration (step 6,
V1-1277) has passed. Every committed map is `cem/1.0` since migration commit
`0b6f1bc245b44a8620c5e64f91e87f6d08da08d0`, so this stage retires the converter, makes the
daily loop's sealed-change admission `cem/1.0`-only and pins the committed corpus with a check
that needs no converter. Decision 0357's `cem/0.2` freeze and its 0.1 N-1 reader are superseded
by decision 0525 §6. The `cem/0.x` wire (`wire.ParseMap`), the 0.x verifier, the
`--spec cem/0.2` and `cem begin` 0.x producers and the 0.x goldens stay in this stage, because
the consumers listed in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1278-s1-remove-0x-core.md` still read or
write them; V1-1278 S2/S3 remove them after those consumers move.

- `CEM-V1-200`: (accepted by decision 0546, 2026-10-20) `wire.ParseStable`, and so every stable read (`cem verify-stable`, a stable
  `cem status`, the corpus check), MUST refuse bytes whose `spec` is `cem/0.1`, `cem/0.2` or
  `cem/0.3` with `unsupported-spec` and the message `<spec> is retired; cem/1.0 is the only
  stable wire; fix: <RetiredSpecFix>`, where the fix line says that committed 0.x maps were
  migrated in place and names the forward recovery: merge `origin/main` into the branch, remove
  the branch's own 0.x seal from `.corvint/changes/` (`CEM-V1-203`), then rerun `dogfood-change`,
  `dogfood-check` and `dogfood-seal` with `BASE=<the origin/main commit merged>`. The fix
  MUST NOT name a rebase or a force-push. Any other non-`cem/1.0` spec keeps the existing
  "stable profile required" refusal.
- `CEM-V1-201`: (accepted by decision 0546, 2026-10-20) Every committed `.corvint/changes/<bind>.cem.json` MUST be accepted by
  `wire.ParseStable`, and a map that carries `provenance` MUST name its own file as
  `bindCommit`. A migrated map (one with `provenance`) MUST name a retired `sourceSpec`
  (`CEM-V1-200`) and a source blob that the repository holds under `sourceBlobOid` with the
  recorded `sourceSha256`, and MUST be byte-identical to the blob one of the `DCW-V0-050`
  migration commits (`cemMigrationCommits`) wrote at its path, so an edit after migration fails.
  A native map (no `provenance`) MUST be byte-identical to the
  `.corvint/change.cem.json` blob its bind commit tracks (`CEM-V1-231`). A Git object the
  checkout lacks makes the binding test report `NOT_RUN` (skip), never a pass. This replaces the
  converter reconversion of `CEM-V1-230`; byte equality between each source and its migrated map
  is the frozen V1-1277 evidence.
  Under decision 0587, the eleven historical maps retained by public root
  `9477995d776ddf88d7f6a82790dd9c22d51da765` use a committed, pinned sparse Git-object
  fixture for these same source, migration and bind-sidecar comparisons. The historical
  names and bytes must match that root and the pinned archive; missing or changed entries
  refuse. The fixture is imported into an isolated temporary repository and proves only
  the selected commit-path and blob relationships, not ancestry, complete history or
  new-root coverage. New/current maps use only the checkout's Git objects, with no fixture
  fallback. Missing proof fails under `CI=true` (V1-1334); corruption never becomes a skip.
- `CEM-V1-202`: (accepted by decision 0546, 2026-10-20) `dogfood batch-check` and `dogfood change`'s sealed-CEM admission (`DCW-V0-039`
  (b), `DCW-V0-042`) MUST verify a sealed constituent only as a `cem/1.0` map: the verifier's
  `cem status` always runs with `--artifacts <absolute repository root>`, and only `ok`,
  `accepted` and a stable `ACCEPT` with exactly that base and bind proves it (`CEM-V1-148`). The
  0.x `ready-for-ci` proof form is retired: a 0.x status line proves no map. A sealed map that is
  unreadable, not a stable map, or declares a spec that is neither `cem/1.0` nor retired
  (`CEM-V1-200`) fails `constituent-cem-invalid`.
- `CEM-V1-203`: (accepted by decision 0546, 2026-10-20) A sealed `cem/0.1`, `cem/0.2` or `cem/0.3` map that the range adds and whose
  blob still equals its bind commit's `.corvint/change.cem.json` (a pre-cutover branch's own
  seal) is a retired seal. It is not verified and grants no coverage. `batch-check` prints
  `NOTE sealed-cem-retired bind=<bind> spec=<spec> NOT_REVERIFIED coverage=none` with a
  `fix:` line, does not count it as a constituent, and does not count its range or its seal
  commit as bound, so those commits fail `unbound-commits` unless a verified `cem/1.0`
  constituent's own range covers them. The recovery is to merge `origin/main`, remove the
  retired seal file with a forward commit (its bind commit and sidecar stay in history), and rebind
  the branch as one `cem/1.0` change whose `BASE` precedes the retired bind, for example the
  `origin/main` commit merged. The removal is required because `CEM-V1-201` refuses any `cem/0.x`
  map in the committed corpus; since the seal is added and removed inside `BASE..HEAD`, the net
  diff that both `dogfood change` and `batch-check` read holds no sealed-CEM change for it, and
  the new map's range covers the retired bind and seal commits. `dogfood change` admits a retired seal with
  `NOTE sealed-cem-retired ... NOT_REVERIFIED bound-by-this-change`, because that change binds
  and cites the whole range itself. No rebase, history rewrite or allow-list is needed. A seal
  modified after its bind commit keeps `sealed-cem-drift` and `DCW-V0-050` keeps its migration
  results.
- `CEM-V1-204`: (accepted by decision 0546, 2026-10-20) The converter (`internal/cem/convert`), `corvint cem convert`
  (`internal/cem/cli/convert.go` and its dispatch), `tools/cem-convert-parity`,
  `script/cem-migrate-maps.sh` and their tests are removed. The stable dispatch handles only
  `verify-stable`, so `corvint cem convert ...` falls through to the generic `cem` parser and
  exits 2 with `invalid-arguments` naming the invalid choice; the help names no `convert`
  action.

Non-goals: no removal of `wire.ParseMap`, the 0.x verifier, the `--spec cem/0.2` or `cem begin`
0.x producers, the MCP, frontier, conformance or attest/anchor 0.x paths (decision 0525 §2 keeps
attest and anchor with the wire owner), the 0.x goldens or the frozen `cli-parity` case
`cem-begin-map` (V1-1278 S2/S3); no rewrite of any committed map or bind commit; no new
migration commit.

Failure modes: a pre-cutover branch that sealed a `cem/0.x` map and merges without rebinding
fails `unbound-commits` with the retired-seal NOTE and its fix, never passes on an unverified
map; a migrated corpus map edited after its migration commit fails `CEM-V1-201` by byte comparison with the blob that commit wrote, and a native map by byte comparison with its bind commit's sidecar; a
shallow checkout without a source blob or bind commit reports `NOT_RUN`; a verifier build that
prints a 0.x `ready-for-ci` line for a stable map fails `constituent-unverified`.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-200 | `TestCEMV1200RetiredSpecRefusedWithMergeFix` (`internal/cem/wire`) | Each retired spec gives `unsupported-spec` with "is retired" and the exact fix, and no "rebase", "force-push" or `--force`; `cem/0.4` keeps the plain refusal; a late 0.x seal in a copied corpus is caught by the corpus scan |
| CEM-V1-201 | `TestCEMV1201CommittedCorpusIsStable` (`internal/cem/wire`), `TestCEMV1201CommittedCorpusBinding` (`internal/cem/corpusbinding`, undeclared so full CI runs it unconfined with Git; it fails instead of skipping under `CI=true`, V1-1334) | At `945867cf` (V1-1334): 726 committed maps parse stable; 615 migrated maps match their source blob digests and retired source specs and equal the blobs their migration commits wrote (614 at `0b6f1bc2`, 1 at `3b86121f`), and 111 native maps equal their bind commits' sidecars; the native count grows with each seal; a late 0.x seal and its removal are checked together |
| CEM-V1-202 | `TestBatchCheckDCWV0039RequiresStableVerification`, `TestBatchCheckDCWV0039PassesSealedConstituents`, `TestBatchCheckAdmitsStableConstituent` (`internal/dogfoodflow`) | A stable ACCEPT admits with `--artifacts`; empty, malformed, incomplete, REJECT, other-target, count-less and 0.x `ready-for-ci` output fail `constituent-unverified` |
| CEM-V1-203 | `TestBatchCheckCEMV1203RetiredSealHasNoCoverage` (`internal/dogfoodflow`) | For each retired spec: batch-check exits 1 with the NOTE, its fix and `unbound-commits count=3` and runs no verifier; `dogfood change` admits it with the bound-by-this-change NOTE; after a `cem/1.0` rebind whose `BASE` precedes the retired bind the batch passes with one constituent; a `cem/0.4` seal fails `constituent-cem-invalid` |
| CEM-V1-204 | `TestCEMV1204StableDispatchNoLongerCapturesConvert` (`internal/cem/cli`) | `cem convert` is not captured by the stable dispatch and is refused by the generic parser with exit 2 |

Rollback: revert the V1-1278 S1 change. The converter, `cem convert`, the parity tool, the
migration script and the 0.x `ready-for-ci` proof form return; `ParseStable` refuses a 0.x map
with the plain "stable profile required" message again; and a retired seal again verifies
through the 0.x reader. No committed map, schema, packet or bind commit changes.

## Local-completion stable sidecar reuse (V1-1303, decision 0525)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; V1-1303. `CEM-V1-150` left local-completion sidecar-only reuse
(`AllowCemSidecarOnlyReuse`) with no `cem/1.0` grant path, so a check observed before the CEM
commit always ran again. This section supersedes `CEM-V1-150` for the grant path only: a
passing observation made on an earlier target may stand for the current target when the only
difference is a committed `cem/1.0` sidecar that the stable verifier accepts in process. Every
other case keeps `CEM-V1-150`'s fail-closed default.

- `CEM-V1-205`: (accepted by decision 0546, 2026-10-20) A check whose plan sets `AllowCemSidecarOnlyReuse` and whose latest passing,
  clean observation was made on another target MUST qualify on the current target only when
  (a) the observation's content digest of the tree excluding `.corvint/change.cem.json` equals
  the current one; (b) the plan's base and the current target are both full 40-hex object IDs;
  (c) `verify.Stable` returns exit 0, outcome `ACCEPT` and `accept: true` for the sidecar with
  `Repository` and `ArtifactRoot` both the absolute repository root, `ExpectedBase` the plan's
  base and `Target` the current target; and (d) the result reports `sidecar: EXACT`,
  `expectedBase` equal to the plan's base and `targetRevision` equal to the current target.
- `CEM-V1-206`: (accepted by decision 0546, 2026-10-20) The verifier MUST receive the exact bytes read from
  `<root>/.corvint/change.cem.json` (bounded by `wire.MaxMapBytes`), never a re-encoding, and
  the result's `mapSha256` MUST equal the SHA-256 of those bytes. A working-tree sidecar that
  differs from the sidecar committed at the target is refused by the verifier or is not `EXACT`,
  and grants nothing; a
  committed sidecar is identified by its raw digest, so a committed whitespace-only re-encoding
  that still verifies is those bytes and may grant.
- `CEM-V1-207`: (accepted by decision 0546, 2026-10-20) A refused reuse MUST leave the check unqualified (`selected-check-unverified`,
  with the existing `dogfood verify` rerun action) and record one closed reason in the check
  observation's additive `reuseRefused` member: `sidecar-content-changed` (a),
  `sidecar-ids-not-full` (b), `sidecar-unavailable` (the sidecar cannot be read),
  `sidecar-rejected` (verifier `REJECT`, for example a stale map, an edited digest or a wrong
  base), `sidecar-unverifiable` (verifier `UNSUPPORTED` or refusal, for example a missing
  artifact under the root) or `sidecar-binding-mismatch` (an accepted result that does not name
  these exact bytes, this base and this target as `EXACT`). `reuseRefused` is omitted when the
  plan does not allow reuse, when the check qualifies, or when no observation applies.
- `CEM-V1-208`: (accepted by decision 0546, 2026-10-20) Without `AllowCemSidecarOnlyReuse`, a check observed on another target MUST stay
  `selected-check-unverified` and no verifier runs (`CEM-V1-150`). No reuse path may report an
  internal error or grant silently: every verifier outcome other than the `CEM-V1-205` grant maps
  to a `CEM-V1-207` reason.
- `CEM-V1-209`: (accepted by decision 0546, 2026-10-20) Each of `CEM-V1-205..208` MUST be traced to a named test in
  `internal/localcompletion` that drives a real `cem prepare` stable map through the in-process
  verifier.

Non-goals: no change to the stable verifier, the wire, `cem prepare`, the dogfood bind loop or
the plan format; no reuse for a repository whose object IDs are not 40-hex (SHA-256 object
format); no caching of verifier results across evaluations; no change to `LCP-V0-004` text.

Failure modes: a sidecar edited after the observation's target, a map from an earlier target, a
wrong base or a missing artifact refuses with its closed reason and the check reruns; an
abbreviated base refuses `sidecar-ids-not-full` before any verifier run; a source change after
the observation refuses `sidecar-content-changed`; a host where structural hunks are outside the
frozen runtime tuples gets `UNSUPPORTED` and refuses `sidecar-unverifiable`. Each reusing check
costs one in-process verifier run per evaluation.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-205 | `TestCEMV1205StableSidecarAcceptGrantsReuse`, `TestCEMV1205ArtifactRootIsRepositoryRoot` (`internal/localcompletion`) | A committed `cem prepare` stable map grants reuse with no rerun; a map with runner artifacts under the root grants, and the same map with one artifact removed refuses `sidecar-unverifiable` |
| CEM-V1-206 | `TestCEMV1206EditedSidecarRefusesReuse` (`internal/localcompletion`) | A committed `patchSha256` edit refuses `sidecar-rejected` (`patch-digest-mismatch`); an uncommitted re-spacing of the committed sidecar refuses `sidecar-rejected` (`excluded-artifact-mismatch`) |
| CEM-V1-207 | `TestCEMV1207StaleSidecarRefusesReuse`, `TestCEMV1207WrongBaseSidecarRefusesReuse`, `TestCEMV1207UnboundSidecarRefusesReuse` (`internal/localcompletion`) | A map from the earlier target refuses `sidecar-rejected` (`patch-digest-mismatch`); a later source change refuses `sidecar-content-changed`; a wrong base refuses `sidecar-rejected` (`base-revision-mismatch`); no sidecar refuses `sidecar-unavailable`, an uncommitted sidecar `sidecar-binding-mismatch` and an abbreviated base `sidecar-ids-not-full`; each refusal names the `dogfood verify` rerun |
| CEM-V1-208 | `TestCEMV1208SidecarReuseFailsClosedByDefault`, `TestCEMV1150StableSidecarGrantsNoReuse` (`internal/localcompletion`) | Without the plan flag a verifying committed sidecar leaves the check `selected-check-unverified` with no `reuseRefused` |
| CEM-V1-209 | This table | Every test above passes on darwin; the suite skips on hosts other than darwin and linux |

Rollback: revert the V1-1303 change. Sidecar-only reuse again grants nothing (`CEM-V1-150`),
the `reuseRefused` member disappears and every check observed before the CEM commit reruns. No
wire, schema, packet, plan or committed map changes.

## `cem/v2` statement (V1-1265, decision 0523 P1)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. Decision 0523 P1. A `cem/v1` statement signs only the map's bytes, its base and
its patch, so a stable map's artifacts and the commit it was accepted at are not signed. `cem/v2`
signs all three for a `cem/1.0` stable map. `cem/0` and `cem/v1` are unchanged.

- `CEM-V1-065`: (accepted by decision 0546, 2026-10-20) `internal/attest.CEMStatementV2(name, map, target)` MUST accept only bytes that
  declare `cem/1.0` and parse as a stable map, and otherwise MUST return a canonical in-toto
  Statement v1. Its `predicateType` is `https://corvint-context.dev/attestation/cem/v2`. Its
  `subject` list is the map first (`{"digest":{"sha256":HEX},"name":NAME}` over the exact map
  bytes), then one `{"digest":{"sha256":S},"name":P}` for each declared artifact, in map order,
  where P and S are the artifact's declared `path` and `sha256`; subject names are unique. Its
  `predicate` is exactly `{"baseRevision":B,"patchSha256":H,"target":{"gitCommit":C,"gitTree":T}}`,
  with B and H the map's `change.baseRevision` and `change.patchSha256`, and C and T the full
  lowercase commit and tree OIDs of the accepted target. A verifier MUST refuse a predicate with an
  unknown, repeated or case-variant member, a missing target, or a malformed digest.
  `corvint prove --cem MAP --expected-base OID --target OID [--artifacts ABS] --attest-cem-v2`
  MUST refuse a `cem/0.x` map with `attest-cem-v2-requires-stable-map`. It runs the stable verifier
  over the map, and MUST refuse any result that is not ACCEPT with a target revision, with
  `attest-cem-v2-not-accepted`. Otherwise it prints the unchanged proof statement or envelope,
  then the `cem/v2` statement or its DSSE envelope over the accepted target. `--attest-cem`,
  `--attest-cem-v1` and `--attest-cem-v2` are mutually exclusive, and `--artifacts` requires
  `--attest-cem-v2` (`invalid-arguments`). Amended by V1-1278 S2b (`CEM-V1-218`): `--attest-cem`
  and `--attest-cem-v1` are retired and refuse as unrecognized arguments (`invalid-arguments`), so
  `--attest-cem-v2` is the only CEM predicate flag. `prove --verify-cem-attestation ENVELOPE
  --attest-public-key PEM --cem MAP [--artifacts ABS] [--target REV]` MUST refuse:
  - map bytes that differ from the signed map subject, with `attest-cem-mismatch`;
  - an artifact file whose digest is not its signed subject, or an artifact set that differs from
    the signed artifact subjects, with `attest-cem-subject-mismatch`;
  - a `--target` whose commit or tree is not the signed target, with `attest-cem-target-mismatch`.

  The same two codes refuse `--artifacts` or `--target` given against a `cem/0` or `cem/v1`
  envelope, which signs neither. Amended by V1-1278 S2b (`CEM-V1-218`): a signed `cem/0` or
  `cem/v1` envelope is no longer read as a claim; it refuses with `attest-verification-failed`,
  whether or not a map is supplied, and the library error names `cem/v2` as the only CEM predicate. A check that is not supplied is reported NOT_RUN in
  `status`, `artifactsStatus` or `targetStatus`, never VERIFIED. `cem anchor` and `cem provenance`
  MUST route a committed map that declares `cem/1.0` through the stable parser. The pointer records
  `map_spec` `cem/1.0` and the map's sha256, the provenance read reports it verified, and the read
  mutates nothing. A `cem/0.x` map's pointer bytes are unchanged (superseded by `CEM-V1-210`: a
  committed `cem/0.x` map refuses with `unsupported-spec` and writes no pointer).

Non-goals: `cem/v2` carries no verdict ledger and no stored verification result (the opt-in ledger beside the anchor pointer is `CEM-V1-271..279`, V1-1292); `cem/v1` is not widened to stable
maps; no signing producer, key management, transparency log or network step; no change to the
stable map schema, `RESULT.schema.json` or the verifier; the target tree is not re-derived from the
artifacts.

Failure modes: a map re-issued over another receipt attestation has new bytes, so the old envelope
refuses it with `attest-cem-mismatch`; the old map with a swapped artifact file is refused with
`attest-cem-subject-mismatch`; a missing or unreadable artifact file is refused with the same
code, never skipped; a rejected, unsupported or drifted stable verification emits no statement;
a `--target` that does not resolve to a commit exits 2 with `unsupported-prove-revision`; a stable
map that fails to parse is not anchored.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-065 | `TestCEMV1065StatementBytesAndSubjectOrder`, `TestCEMV1065VerifiesMapArtifactsAndTarget`, `TestCEMV1065RefusesSwappedReceiptArtifact`, `TestCEMV1065RefusesTargetThatIsNotTheSubject`, `TestCEMV1065PredicateIsClosed`, `TestCEMV1065V1UnchangedForStableMaps` (`internal/attest`) | Statement bytes and subject order over the public `receipt-authentication/` map; the receipt swap is refused under the old envelope both ways; a closed predicate; `cem/v1` still refuses a stable map |
| CEM-V1-065 | `TestCEMV1065ProveEmitsAndVerifiesTheV2Statement`, `TestCEMV1065RefusesAReceiptSwapUnderTheOldEnvelope`, `TestCEMV1065RefusesATargetThatIsNotTheSubject`, `TestCEMV1065RefusesALegacyMap`, `TestCEMV1065RefusesConflictingFlags` (`cmd/corvint`) | `prove --attest-cem-v2` over the materialized public case emits four subjects and the accepted target, verifies VERIFIED on all three axes without mutation, and refuses the swap, a foreign target, a `cem/0.x` map and every flag conflict |
| CEM-V1-065 | `TestCEMV1065AnchorStableMap`, `TestCEMV1210AnchorRefusesARetiredMap` (`internal/gitnotes`) | A committed stable map anchors with `cem/1.0` and its sha256, and provenance reads it back verified without mutation; a committed retired `cem/0.x` map refuses `unsupported-spec` with the fix line and writes no note (`CEM-V1-210`); walkthrough in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-cem-v2-statement.md` |

Rollback: revert `internal/attest/cem_v2.go`, the `V2` field and the v2 parser entry in
`internal/attest`, the stable anchor read in `internal/gitnotes/anchor.go` (the original
`internal/cem/wire/route.go` router was deleted by V1-1278 S2b), the
`--attest-cem-v2` and `--artifacts` branches in `cmd/corvint/prove.go` and
`cmd/corvint/prove_attest_cem.go`, the help lines and this section. No stored state was written in
the new shape: `cem/0` and `cem/v1` bytes are unchanged. A stable-map anchor note already written
stays readable as a plain note, and after rollback `cem provenance` reports its map as
unsupported again.

## Stable verification result codes

The optional `cem verify-stable` operation emits the closed result codes of
`protocol/cem-1.0/stable/OPERATION.json` and `RESULT.schema.json`; those public files stay the
normative mapping. This table names the codes first emitted by the stable verifier, so each has
one owning specification. A code reports a bounded check and never upgrades authority.

| Stage | Codes | Outcome |
| --- | --- | --- |
| references | `invalid-criterion-identity`, `duplicate-reference`, `unresolved-reference`, `reference-coherence`, `artifact-role-closure` | Exit 2; the decoded reference graph is rejected without filesystem or native authority inference |
| authority-arguments | `artifacts-required` | Exit 2 (`UNSUPPORTED`, CEM-V1-116); the explicit artifact root is required even for an empty reference set |
| runtime | `unsupported-structural-runtime`, `unsupported-process-containment` | Exit 2; an unqualified runtime or containment tuple is explicit and has no weakened fallback |
| repository | `unsupported-resource-limit`, `unsupported-repository-envelope`, `verification-timeout` | Exit 2; operational limits stay distinct from invalid evidence |
| repository | `unsupported-patch-inventory` | Exit 2 (`UNSUPPORTED`); a create destination lacks an authenticated base-entry proof, so absence is not inferred |
| artifacts | `artifact-unavailable`, `artifact-resource-limit` | Exit 2 (`UNSUPPORTED`); declared artifact bytes could not be obtained within bounds, which asserts no mismatch |
| artifacts | `artifact-changed-during-verification`, `artifact-digest-mismatch` | Exit 1 (`REJECT` issue codes); the bytes were read and changed between the two passes or differ from the declared digest |
| artifacts | `attestation-malformed`, `attestation-key-mismatch`, `attestation-subject-mismatch`, `attestation-signature-invalid` | Exit 1 (`REJECT` issue codes, CEM-V1-084); a declared receipt attestation does not bind the receipt, plan, target tree or trusted key |

## Core stable read (V1-0632 S1)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. This is the first consumer-migration slice of `CEM-V1-007`. It
adds no producer, no default, no help entry and no stable promotion; the read stays
hidden like `cem verify-stable` until the owner decides help visibility.

- `CEM-V1-017`: (accepted by decision 0546, 2026-10-20) Core `cem status` and `cem verify` over an exact `cem/1.0` map MUST run the stable verifier over the repository root byte for byte as the caller named it (the `--root` value, even empty, with no tilde expansion, absolutization, cleaning or symlink resolution; the working directory only when `--root` is omitted), judged exactly as `cem verify-stable --repository` judges the same string: a relative or empty root is refused at stage `arguments` with `invalid-arguments` before the map is verified, and a symlinked or lexically unclean absolute root at stage `repository`. A root beginning with `--` (for example `--root=--repo`) is refused as `cem verify-stable` refuses that value named first as `--repository`: its parse stops there, so the result keeps no `expectedBase` or `targetRevision`. It runs with the independently supplied `--expected-base`, `--target` and an explicit absolute `--artifacts` root. The envelope MUST embed the complete, unmodified closed result as `stableVerification` and the command MUST exit with the verifier's own code (0 accept, 1 reject, 2 refusal). It MUST NOT synthesize a 0.x `verification`, counts, worklist, policy issues or `ready-for-ci`; status names the outcome `accepted`, `rejected` or `unsupported`. Amended by V1-1276: the one exception is the `cem status` policy ceilings of `CEM-V1-145`, which after an `ACCEPT` add the counts and `policyIssues` and exit 1 when a ceiling fails, with `stableVerification` still unmodified. Amended by V1-2798 (GitHub #1895): `cem report` admits the same ceilings (`CEM-V1-637`), and a read with ceilings names their verdict as `policyState` beside `state`, which stays the verifier's verdict (`CEM-V1-636`). Amended by V1-2962 (GitHub #1948; `CCP-V0-010`, experimental under the standing lane auto-accept, 2026-10-28): `cem status` and `cem report` over a map the stable parser admits always add the per-disposition `counts`, so a read without ceilings adds `counts` alone, with no `policyIssues`, no `policyState` and no exit change; `cem verify` and the MCP report preview still add none. Core MUST NOT open or pre-read the repository before the verifier's own metadata admission.
- `CEM-V1-018`: (accepted by decision 0546, 2026-10-20) A stable read MUST refuse `--patch` with `invalid-arguments`, and `cem verify` MUST refuse policy ceilings with `invalid-arguments` (`cem status` admits them since V1-1276, `CEM-V1-145`, and `cem report` since V1-2798, `CEM-V1-637`), and every other Core workflow read of a `cem/1.0` map with `unsupported-spec` naming the stable reads, until its own slice lands; `cem report` and the report preview are read since S2 (`CEM-V1-034..036`). Each such refusal is marked as a stable refusal, which MCP names `cem-map-stable-refused` (`MCPV0-037`). A 0.x map MUST refuse `--artifacts` with `invalid-arguments` after its map parses. Every 0.x parser refusal of an exact `cem/1.0` root MUST name the stable read rather than report malformed 0.x input. Amended by V1-1278 S2b (`CEM-V1-215`): no 0.x map is read and no 0.x parser remains, so the 0.x `--artifacts` refusal and the parser's naming of the stable read are retired; every root that is not exactly `cem/1.0` refuses with `unsupported-spec`, a retired `cem/0.x` root with the `wire.RetiredSpecFix` line, with or without `--artifacts`.
- `CEM-V1-019`: (accepted by decision 0546, 2026-10-20) (Superseded by decision 0525 §6, V1-0595: the amendment at the end governs.) 0.x reads MUST keep their arguments, precedence, symlink-resolved root, output bytes and exit codes. The stable route MUST be chosen only after the 0.x parser refuses a bounded JSON object whose `spec` is exactly `cem/1.0`; the experimental candidate and every other spec keep their existing code and message, and malformed JSON keeps its earlier failure. Amended by V1-1278 S2b (`CEM-V1-215`): the 0.x reads are retired. `cem status`, `cem verify` and `cem report` take the stable route for an exact `cem/1.0` root and refuse every other spec value, the experimental candidate included, with `unsupported-spec` before any Git read.

Failure modes: an omitted `--artifacts` is the verifier's `authority-arguments` /
`artifacts-required` refusal, never a Core default. A runtime or repository the
verifier does not support is its explicit `UNSUPPORTED` result. The only 0.x change
is that `--artifacts` on a 0.x read is now a stage-3 `invalid-arguments` refusal
after the map read instead of an argument-parser refusal; both exit 2 with no
output. Rollback removes the route and the flag; no 0.x byte, map or packet changes.

### Core stable read traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-017 | `TestCoreStableReadMatchesVerifyStable` | Core verify and status equal `cem verify-stable` (all fields, exit); the public sealed positive is ACCEPT/EXACT on Go 1.27.1 |
| CEM-V1-017 | `TestCoreStableReadKeepsVerifierExit` | Missing `--artifacts` exits 2 (`artifacts-required`); an altered artifact exits 1 (`artifact-digest-mismatch`) |
| CEM-V1-017 | `TestCEMStableReadKeepsCallerRoot` | Through the real argv entry, status and verify with `--root R` equal `cem verify-stable --repository R` (result and exit) for the real root, the symlinked alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias`, `--root ""`, `--root=`, `--root=--repo`, `--root=-x` and `--root=--map`; an omitted `--root` equals `--repository <working directory>`; only the real root and the omitted root pass admission |
| CEM-V1-017 | `TestCEMStableReadDefaultsToSessionRoot` | A workflow session whose entry point never sets a stable repository admits its own unresolved root: a session opened on the symlinked alias equals `cem verify-stable --repository <alias>` (stage `repository`, result and exit) |
| CEM-V1-018 | `TestCoreStableReadExplicitRefusals` | Status/verify `--patch`, verify policy ceilings and report-preview `--patch` refuse before verification (CLI exit 2); status ceilings are `CEM-V1-145` |
| CEM-V1-018, CEM-V1-019 | `TestCEMV1215ReadsRefuseEveryNonStableSpec` | Since V1-1278 S2b a retired 0.x map refuses `unsupported-spec` with the fix line, with or without `--artifacts`, and candidate, other, padded and numeric specs refuse `unsupported-spec`; the 0.x `--artifacts` and parser-text tests were deleted with the 0.x parser |
| CEM-V1-019 | `TestDeclaresStable` | Route selection is exact, bounded and object-rooted |

## Core stable mutator refusals (V1-0632 S3)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. This slice keeps the Core mutators `cem/0.x` producers: the
optional candidate assembler stays the only stable producer until the stable wire
is frozen (`CEM-V1-007`). It adds no producer, default, help entry or promotion.
The requirement IDs start at 030 so that slices proposed in parallel do not collide;
integration may renumber them.

Amended by V1-1276 (`CEM-V1-140..148`): `CEM-V1-030` and `CEM-V1-031` now bind the `cem/0.x` producers (`cem prepare --spec cem/0.2`, `cem begin` without `--spec cem/1.0`, and a mutation of a `cem/0.x` input); a `cem/1.0` producer map is written and mutated by `CEM-V1-141..144`, and `CEM-V1-032` is superseded by `CEM-V1-147` and `CEM-V1-148`.

- `CEM-V1-030`: (accepted by decision 0546, 2026-10-20) A `cem/0.x` Core `cem prepare` MUST refuse an existing map at the fixed sidecar path whose bytes declare exactly `cem/1.0` with `unsupported-spec`, with or without `--replace`. It MUST leave the map, the patch cache and any backup unchanged, and MUST NOT resume, downgrade or overwrite a stable sidecar. It MUST refuse in the same way, before it takes the sidecar lock or recovers a backup, when a recognized interrupted-publication backup (`<map>.bak-` plus sixteen lowercase hex digits) declares `cem/1.0`, and MUST refuse a recognized regular backup it cannot read with `map-unavailable`, because unread bytes may be stable; it re-checks backups under the lock, and neither recovery nor resume cleanup restores or removes a refused backup. With `--replace`, it MUST refuse an existing regular sidecar it cannot read with `map-unavailable` instead of regenerating it, because unread bytes may be stable.
- `CEM-V1-031`: (accepted by decision 0546, 2026-10-20) Core `cem cite`, `cem mark`, `cem cover` and `cem discriminate` MUST refuse a `cem/1.0` input map that is not a producer map (`CEM-V1-143`) with `not-a-producer-map` (amended from `unsupported-spec` by `CEM-V1-342`) naming the command and the stable read, in place or with `--output`, and write nothing. Every Core map writer (`begin`, `cite`, `mark`, `cover`, `discriminate`) writing `cem/0.x` MUST refuse an output destination whose existing bytes declare exactly `cem/1.0`, leaving it unchanged; a writer writing `cem/1.0` refuses such a destination unless it is a producer map. Each writer first runs a non-mutating preflight that refuses a readable `cem/1.0` input or destination before any lock file, directory or backup recovery touches the file system. It then holds the output lock across a strict probe and publication. Only an absent destination, one that is not a regular file (publication replaces the directory entry, never a link target), or one over the map bound (which `wire.DeclaresStable` never selects) proceeds unread. A regular destination whose bytes cannot be read, or that changes while it is read, MUST be refused with `map-unavailable` naming the command and `cem/1.0`, leaving it unchanged.
- `CEM-V1-032`: (accepted by decision 0546, 2026-10-20) (superseded by `CEM-V1-147` and `CEM-V1-148`, V1-1276) The daily bind loop MUST read no hunks from a map declaring `cem/1.0`, so such a map never counts as cited and never binds a citation plan. Its cite step MUST refuse such a map as `cem-cite` `NOT_PRODUCED` `cem-map-stable-unsupported` before it reads the plan, writing no plan binding, cite log or map, and never treat it as a map that owes no hunk. `dogfood batch-check` MUST refuse a sealed `cem/1.0` constituent with `constituent-cem-stable-unsupported` before any verifier run, never as `constituent-cem-invalid`.
- `CEM-V1-033`: (accepted by decision 0546, 2026-10-20) Selection is by the exact declared spec only (`wire.DeclaresStable`). Every `cem/0.x` input, the experimental candidate and every other spec MUST keep the earlier arguments, precedence, output bytes, refusal codes and messages, including the `prepare` `map-unavailable` refusal and an explicit `--replace` regeneration of a non-stable existing map.

Failure modes: a stable sidecar the 0.x workflow used to report as "not a valid
CEM document; pass --replace" now refuses as `unsupported-spec`, so `--replace` is
no longer suggested for it. An unreadable existing regular destination of a 0.x
writer, which publication used to replace, is now refused, because its spec cannot
be resolved: it may be a valid stable map. A destination over the map bound, or not
a regular file, is still left to publication and its existing refusals. The output
lock is advisory and binds only Core writers. Nothing guards against an
uncooperative process that replaces the destination after the strict probe and
before the publication rename: the probe's identity-checked read refuses a
replacement during the read, but no re-check or exclusive create closes the
window that follows. Rollback removes the checks; no 0.x byte, map or packet
changes.

### Core stable mutator refusal traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-030 | `TestPrepareRefusesStableSidecar` | The public sealed positive map at the sidecar path is refused with and without `--replace`; its bytes are unchanged and no patch cache is published |
| CEM-V1-030 | `TestPrepareReplaceRefusesUnreadableSidecar` | A mode-000 stable sidecar keeps the earlier refusal without `--replace` and is refused, not regenerated, with it; no patch cache is published |
| CEM-V1-030 | `TestPrepareRefusesStableBackupBeforeRecovery` | A stable backup with no sidecar is refused before lock and recovery, leaving it in place with no sidecar or lock file; a non-stable backup is still restored |
| CEM-V1-030 | `TestPrepareRefusesUnreadableBackup` | A mode-000 stable backup is refused with `map-unavailable` beside a resumable 0.x sidecar, before resume cleanup would remove it, and with no sidecar, before lock and recovery; the backup and the sidecar stay unchanged and no lock file is created |
| CEM-V1-031 | `TestMutatorsRefuseStableInput` | cite, mark, cover and discriminate refuse a `cem/1.0` input in place and with `--output`; no output is written |
| CEM-V1-031 | `TestMutatorsRefuseStableDestination` | Each mutator, and `cem begin`, refuses a stable `--output` destination; before this slice, cite overwrote it with `cem/0.2` |
| CEM-V1-031 | `TestMutatorsRefuseUnreadableDestination` | Each mutator, and `cem begin`, refuses a mode-000 stable destination with `map-unavailable`; before the fix, each one replaced it |
| CEM-V1-031 | `TestStrictProbeRefusesDestinationReplacedWhileRead` | `begin` and `mark` refuse a destination replaced by a stable map during the locked probe's read |
| CEM-V1-031 | `TestBeginProbesUnderOutputLock` | `begin` waits for a held output lock, publishes nothing meanwhile, then refuses the stable map the holder wrote |
| CEM-V1-030 | `TestStableRefusalPrecedesLock` | `prepare --replace` refuses a stable sidecar before locking; an occupied lock path, which `Lock` refuses as `publish-failed`, stays unchanged |
| CEM-V1-031 | `TestStableRefusalPrecedesLock` | Each writer refuses a stable input or destination before locking, leaving an occupied lock path unchanged |
| CEM-V1-032 | Superseded (V1-1276) | Its tests became `TestMapHunksReadsStableMap`, `TestCiteStepAdmitsStableMap` and `TestBatchCheckAdmitsStableConstituent` (`CEM-V1-147`, `CEM-V1-148`) |
| CEM-V1-033 | `TestPrepareNonStableExistingMapUnchanged` | Candidate, padded and other specs keep the `map-unavailable` refusal and guidance; `--replace` still regenerates them |
| CEM-V1-033 | `TestMutatorsWriteNonStableDestination` | Candidate, truncated and non-JSON destinations are still replaced by the 0.x map |
| CEM-V1-033 | `TestMapHunksReadsStableMap` | A map that does not decode still has no hunks; every other spec keeps the layout-agnostic decode |

## Core stable report (V1-0632 S2)

Status: accepted by decision 0546 (2026-10-20); formerly proposed. This slice moves `cem report`, the report preview and the MCP
`corvint.cem.report` tool from refusing a `cem/1.0` map to reading it through the
stable verifier. It adds no producer, no default, no help entry and no stable
promotion; the read stays hidden like the S1 read.

- `CEM-V1-034`: (accepted by decision 0546, 2026-10-20) Core `cem report --format json` over an exact `cem/1.0` map MUST run the stable verifier exactly as `CEM-V1-017` does, admitting the root exactly as the caller named it (the MCP server's start-time `--root`, not its resolved root), embed its complete, unmodified closed result as `stableVerification`, and exit with the verifier's own code. It MUST add a `review` projection with profile `cem-stable-review/0` that is a pure function of that result: its outcome, stage and code, the map and patch digests, the two revisions, a `stableResultSha256` over the key-sorted result, one row per hunk with its evidence drift state (or `NOT_PRODUCED`), coverage and discrimination states copied from the verifier, `testExecution: NOT_RUN`, an explicit `ocm: NOT_PRODUCED`, the non-claims, and a `recordSetSha256` over the projection. It MUST NOT synthesize a 0.x `verification`, counts, policy issues, worklist or `ready-for-ci`. Amended by V1-2962 (`CCP-V0-010`): the CLI report envelope carries the per-disposition `counts` of a map the stable parser admits; the `review` projection, the MCP preview and every other 0.x member are unchanged.
- `CEM-V1-035`: (accepted by decision 0546, 2026-10-20) Core `cem report` (markdown) over a `cem/1.0` map MUST publish the review rendered from that projection for every verifier outcome, with each repository-derived value in a code span, under the same output resolution, map-aliasing and size bounds as the 0.x report. It MUST locate the worktree's Git directories for publication only after the verifier returns and without the 0.x object admission (alternates, attributes), so a result the verifier refused at stage `repository` still publishes; when the Git directories cannot be located (no valid primary or reciprocal worktree marker), no output path can be checked and publication MUST fail closed with `repository-object-unavailable` and nothing written. The report preview MUST return the same Markdown and `recordSetSha256` without writing, and JSON, preview and published Markdown MUST agree on `recordSetSha256`.
- `CEM-V1-036`: (amended by V1-1278 S1 for V1-1305 under the owner's standing auto-accept of lane requirements (2026-10-11)) A stable report MUST refuse every supplied `--patch`, an empty policy ceiling (a valued ceiling is admitted since V1-2798, `CEM-V1-637`; the MCP report preview still refuses every ceiling), and an empty `--ocm`, including an empty value, with `invalid-arguments` before the verifier runs and before any file is written; these refusals and any unsupported stable action are marked as stable refusals. A non-empty `--ocm` on a stable report MUST produce the OCM review over the `cem/1.0` map through the installed stable OCM reader (`lrfrepo.VerifyStableUniverse`), bound to the OCM bytes Core read; an OCM the stable admission refuses is reported invalid with its closed code and exit 1, and a caller without a stable reader (library or MCP) reports the OCM `NOT_PRODUCED` with reason `ocm-verifier-unavailable`, never ok. A 0.x report MUST refuse `--artifacts` with `invalid-arguments` after its map parses. 0.x report arguments, bytes, goldens and refusals are otherwise unchanged. Amended by V1-1278 S2b (`CEM-V1-215`): the 0.x report and its review projection are retired, so a 0.x report map refuses with `unsupported-spec` and the `wire.RetiredSpecFix` line, with or without `--artifacts`.

Failure modes: an empty `--ocm` refuses; a non-empty `--ocm` (since V1-1305) is read within
the session root, an absent file is `map-unavailable`, a path outside the root is
`invalid-arguments` (`path-traversal`), and an OCM the stable admission refuses is reported
invalid, never bound to the 0.x obligations of another map. A runtime the verifier does not
support yields an `UNSUPPORTED` review that still publishes and exits 2. Rollback
removes the stable report route, the `artifacts` MCP argument and the
`cem-map-stable-refused` code; no 0.x byte, map or packet changes.

### Core stable report traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-034 | `TestCoreStableReportJSONEmbedsVerifierResult` | JSON `stableVerification` equals `cem verify-stable` (all fields, exit); the projection digest recomputes; no 0.x keys; ACCEPT and `stable` drift asserted on Go 1.27.1 |
| CEM-V1-034 | `TestCEMReportToolPreviewsStableMap` | The MCP receipt carries the verifier's complete result for the same inputs, with no 0.x fields |
| CEM-V1-034 | `TestCEMStableReportKeepsCallerRoot`, `TestCEMReportToolAdmitsTheCallerNamedRoot` | For each `--root` spelling (real, alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias`, `--root ""`, `--root=`, omitted, and the option-shaped `--repo`, `-x`, `--map`) the CLI report equals `cem verify-stable --repository <same string>` (complete result, exit); the MCP server accepts only the real root and the alias, each equal to `verify-stable` on that string, and refuses the other spellings at start as `invalid-root` |
| CEM-V1-035 | `TestCoreStableReportPublishesMarkdown` | Published for every outcome; preview equals the published file; `recordSetSha256` agrees across JSON, Markdown and preview |
| CEM-V1-035 | `TestCoreStableReportPublishesRepositoryRefusals` | With `objects/info/alternates` the `UNSUPPORTED` / `repository` / `unsupported-object-alternates` review publishes to the default and a safe external output, equal to `verify-stable`; with no locatable Git directory JSON keeps the result while Markdown fails closed with nothing written |
| CEM-V1-036 | `TestCoreStableReportRefusals` | An empty `--ocm`, an absent or out-of-root OCM, `--patch` (also empty), an empty ceiling and an output naming the map exit 2 with nothing published; `--ocm=` and `--patch=` are argument-parser refusals; ceiling refusals are stable refusals, `cem/9` and 0.x are not |
| CEM-V1-036 | `TestV11305StableReportOCM` (`cmd/corvint`), `TestV11305StableReportOCMWithoutVerifier` (`internal/cem/cli`) | A legacy-bound OCM is invalid `cem-map-digest-mismatch`; a rebound OCM produces coverage, worklist rows and the hunk join; without a reader the OCM is `NOT_PRODUCED` `ocm-verifier-unavailable` |
| CEM-V1-036 | `TestCEMV1215ReadsRefuseEveryNonStableSpec` | Since V1-1278 S2b a 0.x report map refuses `unsupported-spec` with the fix line, with or without `--artifacts` |
| CEM-V1-036 | `TestCEMReportToolNamesStableRefusals` | MCP maps only stable refusals to `cem-map-stable-refused` |

## Portable stable routing (V1-0632 S4)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; retired by V1-1278 S3 (`CEM-V1-222`, `CEM-V1-223`). The portable consumer's
`cem/0.1` default, its `ci` report and `examples/cem` are deleted, so `CEM-V1-PSR-001..004` no
longer bind anything; their tests are kept at commit `5a056d0a`. The text below is history.
Native work: V1-0632, migration rows C04 and C19 of
`protocol/cem-1.0/stable/MIGRATION.json`. The independent portable consumer `interop/cem01-go`
keeps its `cem/0.1` default and its `ci` report schema. It now recognizes an exact `cem/1.0` map
and names the stable command, and an example CI branch runs that command only on explicit opt-in.

- `CEM-V1-PSR-001`: (retired by V1-1278 S3) The default `verify` path refuses a strictly parsed JSON object whose exact
  `spec` member is the string `cem/1.0` with the invalid code `unsupported-spec` (exit 1). The
  refusal comes before the patch is read and before any `cem/0.1` structural check, so a stable map
  is never reported as a malformed `cem/0.1` map.
- `CEM-V1-PSR-002`: (retired by V1-1278 S3) `ci` reports such a map as verdict `unsupported-profile`, exit 4, with `code`
  `use-verify-stable-canonical`. The 14 fixed `cem-ci-report/0` members, the `profile` value
  `cem/0.1` and the verdict-to-exit map (CEM-PILOT-021/022) are unchanged.
- `CEM-V1-PSR-003`: (retired by V1-1278 S3) Only that exact case changes. A duplicate, non-string, differently cased,
  padded or candidate `spec`, malformed JSON, invalid UTF-8, an unpaired surrogate, trailing data
  and every `cem/0.x` map keep their earlier code, bytes and goldens. Frozen protocol packets are
  not edited.
- `CEM-V1-PSR-004`: (retired by V1-1278 S3) `examples/cem/verify-portable.sh` runs `verify-stable-canonical` only when
  `CEM_PROFILE=cem/1.0` is set with an absolute existing `CEM_ARTIFACTS` directory. It reads the
  committed map as Git object data only when the head tree's single entry at exactly
  `CEM_MAP_PATH` is a regular blob, and propagates the verifier's stdout and exit unchanged. Bad
  input, a directory selector or tree entry, an unknown profile or a digest mismatch exits 2 before
  the verifier runs, an absent map exits 3 and an absent head commit exits 5. On the qualified
  runtime tuple the sealed S0e change is accepted and a tampered artifact is rejected at the
  artifact stage; on any other runtime the verifier's own runtime refusal passes through. Acceptance is integrity only and never replaces the
  native completion gate. Nothing switches an existing default workflow to this branch.

Failure modes: a host whose Go toolchain differs from the frozen stable runtime tuple gets
`UNSUPPORTED` `unsupported-structural-runtime` (exit 2) from the stable branch, and the example
passes it through. A map path with characters outside the portable path set is refused before Git
is read. The case-insensitive `cem/0.1` header decoding is unchanged, so `{"SPEC":"cem/1.0"}` keeps
its earlier `version-shape` result.

Rollback: revert the peek in `interop/cem01-go/cem.go`, the route code in `ci.go`, and the
`CEM_PROFILE` branch and recipe hint in `examples/cem/**`. No packet, golden, history or native
state changes.

| Requirement | Implementation | Tests |
| --- | --- | --- |
| CEM-V1-PSR-001 | Retired by V1-1278 S3 (`CEM-V1-222`); `decodeMap` and `declaresStableSpec` are deleted | The test is kept at commit `5a056d0a` |
| CEM-V1-PSR-002 | Retired by V1-1278 S3 (`CEM-V1-222`); `interop/cem01-go/ci.go` is deleted | The test is kept at commit `5a056d0a` |
| CEM-V1-PSR-003 | Retired by V1-1278 S3 (`CEM-V1-222`); the 0.x default path is deleted | The tests are kept at commit `5a056d0a` |
| CEM-V1-PSR-004 | Retired by V1-1278 S3 (`CEM-V1-223`); the example recipe tree and its script test are deleted | The tests are kept at commit `5a056d0a` |

---
## OCM/LRF stable admission (V1-0632 S5a)

Status: accepted under the owner's standing auto-accept (2026-10-11), after independent review
(Codex round 2 PASS). Native work: V1-0632, migration row C05 of
`protocol/cem-1.0/stable/MIGRATION.json`. This slice adds the owning internal
admission that the stable TCQ/frontier, obligation and dashboard consumers (C06..C09, C17)
build on. It adds no CLI route, producer, default, help entry or promotion, and it does
not relabel the frozen `lrf/0` profile.

- `CEM-V1-051`: (accepted by decision 0546, 2026-10-20) `lrfrepo.VerifyStableUniverse` MUST admit a map whose bytes declare exactly
  `cem/1.0` only after the complete stable verifier (`verify.Stable`) accepts those exact original
  bytes. It returns a `StableUniverse` whose `Spec` is `cem/1.0`, whose `MapSHA256` is the
  SHA-256 of the exact original bytes, and whose `Verification` is the complete accepted verifier
  result, unmodified. The verifier MUST report the exact committed sidecar (`EXACT`); an `ABSENT`
  sidecar refuses with `excluded-artifact-mismatch`. The admission MUST re-derive the canonical patch
  at the accepted base and target, and require its digest to equal both the verifier's and the
  map's `patchSha256`. The OCM phase runs inside one retained stable repository boundary. At the
  end, the admission MUST revalidate that boundary and run the verifier again, and MUST require a
  result byte-identical to the first.
- `CEM-V1-052`: (accepted by decision 0546, 2026-10-20) The repository root, expected base, target and artifact root are explicit, with no
  default and no Git-only fallback. A missing or non-absolute argument is the verifier's own
  `authority-arguments` refusal (`expected-base-required`, `target-required`,
  `artifacts-required`, `invalid-arguments`). The artifact references are authenticated under the
  named root, and the authority axes (native authority, authentication, runner execution,
  execution at commit, criterion adequacy) stay `NOT_OBSERVED`.
- `CEM-V1-053`: (accepted by decision 0546, 2026-10-20) The OCM MUST bind the exact original stable map digest, the accepted target and
  patch, the pinned intent scope and its claims, under the existing OCM checks. The universe retains
  every OCM obligation in OCM order. An unsatisfied (`unknown`) obligation keeps its disposition and
  reason, and is never dropped, closed or counted as linked.
- `CEM-V1-054`: (accepted by decision 0546, 2026-10-20) The projection is read-only. It never exposes a legacy `*wire.Map` or any 0.x
  serialization. `OriginalBytes` returns a copy of the authenticated bytes, and `Document`
  re-parses them into a fresh full `wire.StableMap` (criteria, receipts, links and artifacts), so a
  consumer that edits its copy cannot alter the universe or its digest.
- `CEM-V1-055`: (accepted by decision 0546, 2026-10-20) `lrf/0` stays frozen. `lrf.Evaluate` MUST keep refusing a `cem/1.0` context with
  `unsupported-lrf-context` (amended by V1-2665: it refuses every context, because `internal/lrf` carries no `cem/0.1` or
  `cem/0.2` context row, no default lrf/0 admission and no evaluation path; see `CEM-V1-215`). `lrf.ValidateStableRequest` is the owning stable request admission. It
  applies every frozen request check and admits only a `cem/1.0` context with canonical-derived
  patch identity, a target and a complete OCM binding. Any other spec, including a relabeled
  context, and a context without the OCM binding refuse with `unsupported-lrf-context`. It
  evaluates nothing.
- `CEM-V1-056`: (accepted by decision 0546, 2026-10-20) A tampered artifact is the verifier's REJECT (`artifact-digest-mismatch`, exit 1),
  and a missing one its UNSUPPORTED refusal (`artifact-unavailable`). A stable map other than the
  committed exact sidecar, a stale target or base, an OCM bound to another map digest or target, and
  an OCM missing an obligation also refuse. So does an artifact or repository-root change made after
  the first verification: the second verification refuses or differs, or the boundary is no longer
  valid. Every refusal returns no universe, so no consumer can
  derive a closed or empty frontier from it. A verifier refusal is returned as `StableRefusal`, which
  retains the full result and exit; `lrfrepo.CodeOf` reports its code.
- `CEM-V1-057`: (accepted by decision 0546, 2026-10-20) Profile swaps refuse in both directions. A `cem/0.x` or relabeled map refuses in
  `VerifyStableUniverse` with `unsupported-spec`. A `cem/1.0` map refuses in the legacy
  `VerifyUniverse` with `unsupported-spec`, naming the stable universe verification.
- `CEM-V1-058`: (accepted by decision 0546, 2026-10-20) The old OCM/LRF admission is unchanged for `cem/0.x`: `VerifyUniverse`,
  `lrf.Evaluate`, `ocm read`, `ocm prepare`, `ocm link` and `corvint lrf` keep their arguments,
  bytes and codes. `VerifyUniverse`, `lrf.Evaluate` and `corvint lrf` refuse a `cem/1.0` map
  explicitly with `unsupported-spec`, instead of reducing it or reporting it as malformed 0.x
  input. Amended by V1-1276: `ocm read`, `ocm prepare` and `ocm link` admit a `cem/1.0` map only
  through `CEM-V1-149`; `ocm prepare` still refuses before it writes or replaces anything.
- `CEM-V1-059`: (accepted by decision 0546, 2026-10-20) A stable universe verification is a read. It MUST NOT change the repository HEAD,
  index or worktree status, the artifact files or the caller's input bytes.

Failure modes: when the change has structural mechanical hunks, a host whose Go toolchain is
outside the frozen structural runtime tuples gets the verifier's `UNSUPPORTED`
`unsupported-structural-runtime` refusal; the admission's own fixture has none, so this is not
exercised here. A map whose target commits a different sidecar is refused by the verifier's sidecar
binding. A target that commits no sidecar can still be accepted by the verifier (`ABSENT`); the
admission itself then refuses with `excluded-artifact-mismatch`. There is no stable LRF
result: evaluating the projected request needs an accepted stable LRF profile.

Rollback: remove `VerifyStableUniverse`, `StableUniverse`, `StableRefusal` and
`lrf.ValidateStableRequest`, and the explicit `cem/1.0` refusals in `VerifyUniverse`
(the `readOCMBytes` and `PrepareOCM` refusals became the `CEM-V1-149` admission in V1-1276). No `lrf/0` or OCM profile, default, packet or stored evidence
changes.

### OCM/LRF stable admission traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-051 | `TestCEMV1051StableUniverseAccepts` | A real `cem/1.0` change, with its exact sidecar committed at the target and six artifacts, is ACCEPT/EXACT; the universe keeps the exact bytes, digest, patch, target and LRF context |
| CEM-V1-052 | `TestCEMV1052StableUniverseAuthorityArguments` | Six artifact checks; `NOT_OBSERVED` authority axes; a missing artifact root, base, target or root is the verifier's `authority-arguments` refusal (exit 2) |
| CEM-V1-053 | `TestCEMV1053StableUniverseRetainsUnsatisfiedObligations` | `OCM-TEST-001` linked to the supported hunk; `OCM-TEST-002` stays `unknown`/`unassessed` |
| CEM-V1-054 | `TestCEMV1054StableUniverseReadOnlyDocument` | `Document` keeps criteria, receipts, links and artifacts; edits to returned copies never reach the universe |
| CEM-V1-055 | `TestCEMV1055StableLRFRequestAdmission` | `lrf.Evaluate` refuses the stable request; the stable admission refuses a `cem/0.2`, relabeled, target-less or OCM-unbound request |
| CEM-V1-056 | `TestCEMV1056StableUniverseRefusesStaleOrTamperedInput` | Tampered artifact REJECT (exit 1, result retained); missing artifact `artifact-unavailable`; swapped map `excluded-artifact-mismatch`; OCM `cem-map-digest-mismatch`/`target-mismatch`; an absent sidecar with a rebound OCM and a stale caller target `excluded-artifact-mismatch`; stale base; truncated OCM |
| CEM-V1-056 | `TestCEMV1056StableUniverseRevalidatesAfterOCMPhase` | An artifact changed after the OCM phase is the second verification's REJECT; a root replaced by a symlink is `repository-object-unavailable`; restored inputs admit again |
| CEM-V1-057 | `TestCEMV1057StableUniverseProfileSwapsRefuse` | `cem/0.2` and relabeled maps refuse in the stable route; `cem/1.0` refuses in `VerifyUniverse` |
| CEM-V1-058 | `TestCEMV1058LegacyOCMLRFAdmissionUnchanged` | The `cem/0.2` universe and `lrf.Evaluate` still accept; `Evaluate` refuses `cem/1.0` with `unsupported-spec`; a bound `cem/1.0` map whose artifacts are outside the repository root fails closed (`ReadOCM` state `invalid` with cause `artifact-unavailable`, `PrepareOCM` `artifact-unavailable` and `LinkOCM` `invalid-cem`, map unchanged, `CEM-V1-149`); existing `internal/lrfrepo`, `conformance/ocm-v0`, frontier, TCQ and CLI LRF/OCM suites pass |
| CEM-V1-059 | `TestCEMV1059StableUniverseReadDoesNotMutate` | HEAD, porcelain status, artifact bytes and input bytes are unchanged after an accepted read |

## Core stable bundle export (V1-0632 S5b)

Status: accepted under the owner's standing auto-accept (2026-10-11), after independent review
(Codex round 8 PASS). Native work: V1-0632, migration row C10 of
`protocol/cem-1.0/stable/MIGRATION.json`. `cem export` over an exact `cem/1.0` map now writes a
relocatable stable bundle instead of refusing; the `corvint-receipt-bundle/0` export of a 0.x map
keeps its layout and bytes. The route stays hidden like the S1 read: help is unchanged.

- `CEM-V1-060`: (accepted by decision 0546, 2026-10-20) Core `cem export` over a map whose bytes declare exactly `cem/1.0` MUST first run
  the stable verifier over the caller-named root byte for byte, as `CEM-V1-017` does, with
  `--artifacts` as its artifact root. Unless that result is ACCEPT (exit 0) it MUST return the
  verifier's complete result and exit as `stableVerification` with `ok: false` and write nothing.
- `CEM-V1-061`: (accepted by decision 0546, 2026-10-20) An accepted export MUST write a new directory outside the worktree (the 0.x output
  rule) with profile `corvint-cem-stable-bundle/1`: `cem.json` holding the exact map bytes,
  `artifacts/<declared path>` holding the exact bytes of every artifact the map declares (all six
  roles, including the snapshot-head receipt), and a canonical one-line `manifest.json` written last
  that records the profile, spec, base, target and each file's path, digest and byte length. Every
  artifact is re-read and must match its declared digest; the map is never rewritten, relabelled
  or reduced.
- `CEM-V1-062`: (accepted by decision 0546, 2026-10-20) The bundle MUST verify from any root: `ReadStable` / `VerifyStable`, and `cem
  verify-stable --map <dir>/cem.json --artifacts <dir>/artifacts`, over a moved bundle MUST
  reproduce the source's complete closed result and exit. Export MUST verify the written bundle
  the same way and remove it, with `publish-failed`, unless that result equals the source result.
- `CEM-V1-063`: (accepted by decision 0546, 2026-10-20) `ReadStable` MUST refuse a bundle with a missing declared file
  (`stable-bundle-entry-missing`), a file or directory nothing declares
  (`stable-bundle-entry-unused`), a symlink or other non-regular entry anywhere, including the
  artifacts directory (`stable-bundle-entry-escaping`), bytes that differ from the declared digest
  or length (`stable-bundle-entry-digest`), or a manifest that is not exact or disagrees with the
  map (`stable-bundle-manifest`). `VerifyStable` MUST also refuse a bundle that records another base
  or target than the caller supplies (`stable-bundle-revision`). `WriteStable` MUST refuse a bundle
  that is not exactly its map's declared closure (base, order, role, path, bytes) before creating
  anything, and MUST write only inside the new bundle directory: it MUST refuse, before writing, a
  created directory whose name no longer holds that same real directory, whose opened root is
  another directory, or that is not empty; its cleanup MUST remove, without recursion and through
  the opened directory, only the files and directories the bundle declares, and then the name only
  while it still holds that directory. The writer's threat model excludes a concurrent process with
  write access to the output parent directory: against such a process it guarantees only that it
  never writes outside the directory it bound and never follows a swapped symlink (see the remaining
  limits below). Readers MUST open entries without following a final symlink
  or blocking on a substituted FIFO, MUST recheck each name, re-read every declared file against
  the digest it was admitted with, then walk the whole tree again comparing names, kinds and
  directory identities, and each file's identity, size, modification time and inode change time
  with the state its re-read opened (a host without a change time refuses), and check that the caller's path still names the opened directory, so a
  bundle changed during the read (including the bundle or an ancestor directory swapped for a
  symlink) is refused, and MUST read directories in
  bounded batches so a tree over the entry bound is refused before it is held in memory. The Go API (`CollectStable`,
  `WriteStable`, `ReadStable`, `VerifyStable`) is the reusable read/write surface for later
  attestation, Git-notes and example-CI consumers. The export route probe that selects this path
  MUST open the map the same nonblocking way.
- `CEM-V1-064`: (accepted by decision 0546, 2026-10-20) A stable export MUST refuse `--witness` with `invalid-arguments` before
  verification, and an output inside the worktree with `bundle-output-refused` after verification;
  neither writes anything. A 0.x export MUST refuse `--artifacts` with `invalid-arguments`. 0.x
  export arguments, manifest, receipts and bytes are otherwise unchanged.

Failure modes: a stale base, tampered or omitted artifact, missing `--artifacts` or unsupported
runtime is the verifier's own REJECT or UNSUPPORTED result, carried unchanged with nothing
written. Closure checks over a received bundle run through the Go API or through `cem
verify-stable` (which checks every declared artifact but not unused entries); no CLI import verb is
added. Witness, dogfood and gate receipts are not carried in a stable bundle. The only 0.x change
is that `--artifacts` on a 0.x export is an `invalid-arguments` refusal instead of an
argument-parser refusal; both exit 2 with nothing written.

Remaining writer limits (out of scope under `CEM-V1-063`'s threat model; evidence: Codex review round 6,
recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-v1-0632-s5-bundle-export.md`): a concurrent process with write access to the output
parent can (1) move an empty directory of its own into the output name between `Mkdir` and the
writer's identity capture, which the writer then adopts and fills, and removes on a later failed
write; and (2) create a file at a declared path inside the bound directory after it is opened, so
the exclusive write fails and cleanup, which removes every declared path, removes that file.

Rollback: remove the stable branch in `receiptbundle.Export`, `stable_bundle.go`, the new
`Options` fields and the `--artifacts` export flag. No 0.x byte, map, packet or stored bundle
changes.

### Core stable bundle export traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-060 | `TestStableExportRelocatesWithSameVerification` | The export envelope's result equals `verify.Stable` over the source (ACCEPT/EXACT on Go 1.27.1, 1.27.2 and 1.27.3) |
| CEM-V1-060 | `TestStableExportPublishesNothingForARefusedSource` | Stale base and tampered artifact (REJECT, exit 1), omitted artifact (UNSUPPORTED, exit 2), no artifact root and relative root (exit 2) return the verifier result with no bundle written; each absolute-root result equals `verify.Stable` over the same inputs |
| CEM-V1-061 | `TestStableExportRelocatesWithSameVerification` | Map bytes equal the sealed bytes; all six roles, including the snapshot-head receipt, are exact copies; the manifest digest is reported |
| CEM-V1-062 | `TestStableExportRelocatesWithSameVerification`, `TestCEMExportStableBundleRelocates` | After `os.Rename` to another parent, `VerifyStable` and `cem verify-stable` over the moved bundle equal the source result and exit |
| CEM-V1-062 | `TestStableBundleKeepsNestedPathsAcrossRoots` | Nested declared artifact paths keep their paths and digests across a move |
| CEM-V1-063 | `TestStableBundleRefusesIncompleteOrTamperedEntries` | Tampered snapshot-head or map byte, omitted artifact or map, extra file, extra directory, symlinked artifact or artifacts directory, inexact or extended manifest, and another target each refuse with their code (exit 2) |
| CEM-V1-063 | `TestWriteStableRefusesAnAlteredClosure` | An escaping path, a duplicated path (each with the artifact count kept), an added artifact, a relabelled or omitted role, altered bytes and another base refuse before anything is created; the output parent stays empty |
| CEM-V1-063 | `TestReadStableRefusesAnOversizedTree`, `TestStableEntryOpenDoesNotBlockOnAFIFO` | A tree over 4,096 entries refuses; opening a FIFO entry returns at once and the read refuses it |
| CEM-V1-063 | `TestReadStableRefusesABundleChangedDuringTheRead` | Between an entry's check and its open, swapping it for an in-root symlink to an undeclared sibling with the same bytes, rewriting an already-read `cem.json`, adding a file, or, during the re-reads, moving `artifacts` or the bundle itself aside and replacing it with a symlink to it, refuses as `stable-bundle-entry-escaping` |
| CEM-V1-063 | `TestReadStableRefusesAFileRewrittenAfterItsReread` | A declared file rewritten in place after its own re-read, while another file is re-read, refuses as `stable-bundle-entry-escaping`, both with its modification time moved and with the same length and its original modification time restored |
| CEM-V1-063 | `TestWriteStableRefusesASwappedBundleDirectory` | A created bundle directory replaced before its identity check by a non-empty sibling, or before its open by a symlink to that sibling or by another real directory, refuses as `publish-failed` with no file written or removed; a write failing after `cem.json` removes the bundle it started |
| CEM-V1-060 | `TestCEMExportStableKeepsCallerRoot` | For the alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias` and `--repo`, `cem export --root` equals `cem verify-stable --repository` on the same string (complete result, exit) and publishes nothing |
| CEM-V1-064 | `TestStableExportArgumentRefusals` | `--witness` and an in-worktree output refuse with nothing written; a 0.x export refuses `--artifacts` |

## Stable projections (V1-0632 S5c)

Status: accepted under the owner's standing auto-accept (2026-10-11) after independent review
(Codex round 3 PASS); delivery stays library-level and unpromoted. Native work: V1-0632, migration rows C13, C14 and C16 of
`protocol/cem-1.0/stable/MIGRATION.json`. Each family gains a read-only, library-level stable
route beside its unchanged 0.x route. Every route admits an exact `cem/1.0` map only through the
complete stable verifier (or, for the console, the strict stable wire reader with the verifier
named as NOT_RUN). No route reduces, re-encodes or relabels a stable map, writes a map, or treats a
reference as task completion or criterion satisfaction. No CLI flag, default or help entry changes.

- `CEM-V1-070`: (accepted by decision 0546, 2026-10-20) The witness compiler's CEM source (`internal/witness/stable.go`) MUST select the stable reader by `wire.DeclaresStable` before the 0.x parser, run `verify.Stable` with the index root, the caller's expected base, the index commit and an explicit artifact root, and bind the source (`BOUND`) only on an `ACCEPT` result whose `mapSha256` equals the digest of the exact bytes read and whose base equals the expected base. The source keeps the complete verifier result under profile `corvint-witness-stable-binding/1` with `criterionSatisfaction: NOT_OBSERVED`.
- `CEM-V1-071`: (accepted by decision 0546, 2026-10-20) A stable witness source without an artifact root MUST be `UNSUPPORTED` with no verifier result; a verifier refusal at the binding stage MUST be `UNBOUND` and any other refusal `INVALID`. A refused source MUST keep the full result and, whenever the stable reader returns a document (also beside a graph error), the map's hunk and basis counts and MUST apply no citation, so the report keeps its no-bound-evidence-map precondition.
- `CEM-V1-072`: (accepted by decision 0546, 2026-10-20) A report built over a stable source MUST take profile `corvint-witness-stable/0`. Stable citations MUST stay non-closing, and a 0.x source MUST encode no stable field.
- `CEM-V1-073`: (accepted by decision 0546, 2026-10-20) `witnesscollapse.CompileStable` MUST admit a stable map only through an `ACCEPT` verifier result over the complete input binding (repository, expected base, target, artifact root) bound to the exact map digest, then collapse over the private mechanics view under profile `corvint-correlated-witness-collapse/experimental-stable-v0` with `cem.spec` `cem/1.0`. The report MUST carry the complete verifier result (every original hunk with its caller coverage and discrimination states, every reference, the artifact checks and the `NOT_OBSERVED` axes) and `criterion_satisfaction: NOT_OBSERVED`. `VerifyStable` MUST recompile from the same bytes and inputs and refuse any difference in the key-sorted JSON value of the report with `report-mismatch`, so a serialized and decoded report verifies and an edited one does not.
- `CEM-V1-074`: (accepted by decision 0546, 2026-10-20) The 0.x `Compile` MUST keep refusing a stable map. `CompileStable` MUST refuse a non-stable map with `not-stable`, and any verifier refusal or digest mismatch with `stable-verification-refused`, returning a report that holds only the full verifier result and no hunk, group or cause. Evidence or artifact drift after compilation MUST make `VerifyStable` refuse rather than yield an equivalent report. A nil mechanics view fails with `cem-unavailable`.
- `CEM-V1-075`: (accepted by decision 0546, 2026-10-20) `doccorpus.StableCEMProjection` MUST project documentation claims through a stable map only after the stable verifier accepts the exact bytes. Its receipt (schema `corvint-corpus-cem-stable/1`) MUST carry the full map digest and spec, the unmodified verifier result, reference counts marked `REFERENCE_INTEGRITY_ONLY`, `native_authority: NOT_OBSERVED`, and each claim's original immutable span at the verified base bound by evidence identity. It MUST offer no writer argv and MUST NOT encode a reduced stable map. The 0.x `CEMProjection` keeps refusing a stable map.
- `CEM-V1-076`: (accepted by decision 0546, 2026-10-20) A wrong base, edited map bytes, a tampered artifact, a missing artifact root and a non-stable map (which still goes through the verifier and is marked `cem_spec: UNSUPPORTED`) MUST refuse with `corpus-stable-refused`, an outcome `REFUSED`, no claim row and the full verifier result kept; a claim whose span differs at the verified base MUST refuse instead of citing.
- `CEM-V1-077`: (accepted by decision 0546, 2026-10-20) The console chain pane MUST admit `cem/1.0` in its closed profile registry only when the strict stable wire reader accepts the whole sealed document. It MUST draw the verification envelope as an `unverified` gap naming the stable verifier as NOT_RUN, the map digest pin, the sidecar state `NOT_CHECKED`, the integrity axes `NOT_CHECKED` and the native authority, criterion adequacy, criterion discrimination, runner execution and authentication axes `NOT_OBSERVED`.
- `CEM-V1-078`: (accepted by decision 0546, 2026-10-20) Each criterion binding, runner receipt (with its execution authority, source binding, dependency closure and authentication states), criterion link and artifact MUST render as a linked reference row stating that a valid reference is not task completion and not criterion satisfaction, followed by a `task completion NOT_OBSERVED` gap. The pane MUST NOT open or execute artifact content. The caller's survived discrimination state and a stale seal stay visible.
- `CEM-V1-079`: (accepted by decision 0546, 2026-10-20) The registry stays closed: an unsupported profile, a near-stable spec, an unknown member, a duplicate or case-variant `spec` member that names `cem/1.0` beside any other value, and a stable map the strict reader refuses MUST draw no chain; 0.x maps keep their reading (superseded by `CEM-V1-210`: a retired 0.x map is refused with `unsupported-spec` and the recovery line).

Failure modes: a host whose Go toolchain is not an admitted structural tuple gets the verifier's
`UNSUPPORTED` runtime refusal, which every witness, collapse and documentation route keeps as a
refusal. The console never claims verification; its stable rows are display only.

Rollback: remove `internal/witness/stable.go`, `internal/witnesscollapse/stable.go`,
`internal/doccorpus/cem_stable.go` and `internal/console/chain_stable.go` with the stable
branches that call them; restore `cem/1.0` to unsupported in the console registry. No 0.x
profile, default, packet or stored evidence changes.

### Stable projection traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-070 | `TestStableWitnessSourceAdmitsVerifiedMap` | The public sealed positive binds `BOUND` with the complete ACCEPT result, EXACT sidecar, map digest and axes |
| CEM-V1-071 | `TestStableWitnessSourceRefusals` | No artifact root is `UNSUPPORTED`; a wrong base and a tampered artifact refuse with counts and the result kept |
| CEM-V1-072 | `TestStableWitnessSourceAdmitsVerifiedMap`, `TestStableWitnessReportProfile` | Stable report profile, non-closing citations, WITNESS-P3 kept for an unadmitted source, no stable field on 0.x |
| CEM-V1-073 | `TestStableCollapseRetainsFullMap` | Canonical report keeps the survived witness, killed/survived counts, runner receipts and `NOT_OBSERVED`; the in-memory and the serialized-then-decoded report verify; an edited report and a decoded report claiming satisfaction mismatch |
| CEM-V1-074 | `TestStableCollapseRefusals` | 0.x refusal; wrong base, no artifacts, edited map and tampered artifact refuse; unknown witness after admission; drift refuses `VerifyStable` |
| CEM-V1-075 | `TestStableCEMProjectionCitesVerifiedSpan` | The policy span cites its evidence id at the verified base; no writer argv; 0.x projection refuses |
| CEM-V1-076 | `TestStableCEMProjectionRefusals` | Wrong base, no artifacts, edited map, 0.x map, case-variant spec, edited claim span and tampered artifact refuse, each non-span refusal with a `REFUSED` receipt and the full verifier result |
| CEM-V1-077 | `TestConsoleChainStableDisplay` | Envelope, digest pin, sidecar and axis rows render for the sealed public positive |
| CEM-V1-078 | `TestConsoleChainStableDisplay`, `TestConsoleChainStableStale` | Reference rows say not task completion; artifact content not opened; survived witness and stale seal shown |
| CEM-V1-079 | `TestConsoleChainStableRegistry` | `cem/2.0`, `cem/1.0.0`, an unknown member, duplicate and case-variant `spec` members and an invalid stable map refuse; since `CEM-V1-210` each retired 0.x map refuses with `unsupported-spec` and the recovery line |

## Core cem/1.0 producer and dogfood loop (V1-1276, decision 0525 step 5)

Status: accepted under the owner's standing auto-accept (2026-10-11), after independent review
(Codex round 6 PASS); V1-1276. Decision 0525 makes `cem/1.0` the only map the RC3 loop produces;
this slice switches the Core producers and the daily bind loop to it while the `cem/0.x` readers
stay until V1-1278. It amends `CEM-V1-018`, `CEM-V1-030..033` and `CEM-V1-058` in place and the
`DCW-V0` text that named the refusals. A producer map is a `cem/1.0` map with no criteria,
receipts, links, artifacts or provenance; it may carry a `role` (`CEM-V1-095`).

- `CEM-V1-140`: (accepted by decision 0546, 2026-10-20) `corvint cem prepare` MUST write `cem/1.0` by default. `--spec cem/0.2` keeps the
  `cem/0.2` producer as an explicit, fail-closed escape until V1-1278 removes it with the 0.x
  readers. `corvint cem begin` keeps `cem/0.1` by default, because the frozen cli-parity case
  `cem-begin-map` pins it, and writes `cem/1.0` with `--spec cem/1.0`. Any other `--spec` value,
  including `cem/0.1`, `cem/0.3` or the experimental candidate for prepare, MUST be refused with
  `invalid-arguments` before anything is written. The library zero value keeps the legacy
  producer, so MCP, frontier, conformance and every other library caller is unchanged.
- `CEM-V1-141`: (accepted by decision 0546, 2026-10-20) A `cem/1.0` prepare MUST write the canonical stable encoding of a producer map
  whose hunks are the canonical diff of base..target, and MUST resume a same-base, same-patch
  stable map byte for byte. An existing `cem/0.x` sidecar MUST be refused with `map-unavailable`
  "records a different spec; pass --replace to regenerate", and a stable map of another base or
  patch with the existing "different base or patch" refusal. With `--replace` it MUST regenerate
  a `cem/0.x` map, a stable producer map, or bytes that declare `cem/1.0` but do not parse, and
  MUST refuse, unchanged, a valid stable map that is not a producer map (`unsupported-spec`) and a
  regular sidecar it cannot read (`map-unavailable`). Without `--replace`, sidecar bytes that are
  neither a valid `cem/0.x` nor a `cem/1.0` map keep the `map-unavailable` "is not a valid CEM
  document; pass --replace to regenerate" refusal. Before the sidecar lock and again under it,
  before backup recovery or resume cleanup restores or removes a recognized interrupted-publication
  backup, a `cem/1.0` prepare MUST refuse, unchanged, a backup that is a valid stable map but not a
  producer map (`unsupported-spec`) or that it cannot read (`map-unavailable`); a stable producer
  or `cem/0.x` backup is restored or removed as before. A `cem/0.x` prepare keeps `CEM-V1-030`.
- `CEM-V1-142`: (accepted by decision 0546, 2026-10-20) `cem begin --spec cem/1.0` MUST write the canonical stable producer map of the
  supplied patch at its `--output`, may rewrite a stable producer map there, and keeps the
  `CEM-V1-031` destination guards; a `cem/0.x` begin never replaces a stable map.
- `CEM-V1-143`: (accepted by decision 0546, 2026-10-20) `cem cite`, `cem mark`, `cem cover` and `cem discriminate` over a stable producer
  map MUST write a canonical `cem/1.0` producer map, in place or at `--output`, keeping its `role`;
  they MUST NOT upgrade it to `cem/0.3`. A stable map that is not a producer map stays refused
  with `not-a-producer-map` (amended from `unsupported-spec` by `CEM-V1-342`) naming the
  command, as `CEM-V1-031` requires.
- `CEM-V1-144`: (accepted by decision 0546, 2026-10-20) A structural `cem mark` on a stable producer map writes the claimed reason; the
  stable verifier, not the producer, proves it, so an import reorder verifies `ACCEPT` and a
  wrong-class claim on the same hunk is `REJECT` `unproven-mechanical` from `cem status`.
- `CEM-V1-145`: (accepted by decision 0546, 2026-10-20) `cem status` over a `cem/1.0` map MUST admit `--max-unknown` and
  `--max-mechanical`, applied to the verifier's hunk dispositions after an `ACCEPT`: within the
  ceilings the envelope reports `ok: true`, `state: accepted`, the counts and empty
  `policyIssues`; an exceeded ceiling reports `ok: false` with its policy issue and exits 1. The
  embedded `stableVerification` stays exactly the `cem verify-stable` result. `cem verify` keeps
  refusing ceilings (`CEM-V1-018`). Amended by V1-2798 (GitHub #1895): the envelope also names `policyState`
  (`CEM-V1-636`), and `cem report` applies the same ceilings (`CEM-V1-637`).
- `CEM-V1-146`: (accepted by decision 0546, 2026-10-20) A stable prepare's `nextActions` MUST be the single `cem status` argv with the
  literal base and target, `--artifacts` set to the absolute repository root, and zero
  ceilings. A producer map names no artifact, so that root is never opened.
- `CEM-V1-147`: (accepted by decision 0546, 2026-10-20) The daily bind loop MUST cite, count, check and seal a `cem/1.0` map like a
  `cem/0.x` map: `mapHunks` and `citedHunks` read its hunks, the cite step binds the plan to them,
  and the loop's and `dogfood check`'s `cem status` add `--artifacts <absolute repository root>`.
  The first stable pass over a sidecar left by a `cem/0.x` run treats the different-spec refusal
  as an outdated map and regenerates it with `--replace`. `cem-map-stable-unsupported` is no
  longer reported; every other `NOT_PRODUCED` reason stays visible.
- `CEM-V1-148`: (accepted by decision 0546, 2026-10-20) `dogfood batch-check` MUST admit a sealed `cem/1.0` constituent when the verifier,
  run with `--artifacts <absolute repository root>`, reports `ok`, `accepted` and a stable
  `ACCEPT` with exactly its base and bind. The map's declared spec selects the proof form: a 0.x
  `ready-for-ci` line never proves a stable map, and a stable line never proves a 0.x map.
  `constituent-cem-stable-unsupported` is retired.
- `CEM-V1-149`: (accepted by decision 0546, 2026-10-20) `ocm prepare`, `ocm read` (status) and `ocm link` MUST admit a `cem/1.0` map only
  when the stable verifier, with the repository root as artifact root, returns `ACCEPT` for the
  expected base and target, and the canonical diff of that base and target has the map's patch
  digest. Every other result fails closed with the verifier's code (prepare, read cause) or the
  existing `invalid-cem` mapping (link); nothing is written. The sidecar need not be committed,
  because the first bind pass runs before the CEM commit.
- `CEM-V1-150`: (accepted by decision 0546, 2026-10-20) Local-completion sidecar-only reuse (`AllowCemSidecarOnlyReuse`) MUST keep
  binding only a canonical `cem/0.x` sidecar. A `cem/1.0` sidecar grants no reuse: the check stays
  `selected-check-unverified` until it runs again on the current target. This fails closed and
  costs a rerun; stable reuse needs its own requirement (superseded for the grant path by
  `CEM-V1-205`, V1-1303).
- `CEM-V1-151`: (accepted by decision 0546, 2026-10-20) `dogfood finish` MUST read a `cem/1.0` change map: its `cem status` (and the
  status it names in `nextActions`) adds `--artifacts <absolute repository root>` and keeps the
  ceilings (`CEM-V1-145`), and its `cem report` adds the same root and passes no ceilings, which
  the preceding status already enforced (a stable report admits them since V1-2798, `CEM-V1-637`). A `cem/0.x` map's arguments
  are unchanged.
- `CEM-V1-152`: (accepted by decision 0546, 2026-10-20) The aggregate verifier worker's closed argv grammar (`ALO-V0-018`) MUST admit the
  strict check's `cem status` read of a `cem/1.0` change map with a trailing
  `--artifacts <absolute path>` only when that path names the worker's own repository root; any
  other artifact root, a relative one, or `--artifacts` on another read is refused
  `aggregate-worker-invalid` before a verifier runs.

Non-goals: no removal of the `cem/0.x` readers, converter or `--spec cem/0.2` (V1-1278); no
change to `cem begin`'s default, the frozen cli-parity cases, MCP, frontier or conformance
producers; no stable LRF evaluation; no local-completion reuse of a `cem/1.0` CEM (`CEM-V1-150`; V1-1303
adds the grant path, `CEM-V1-205`).

Failure modes: `internal/localcompletion` reads only canonical `cem/0.x` CEMs, so it reports no
reuse for a `cem/1.0` change and fails closed (`CEM-V1-150`; since V1-1303 the stable grant path is
`CEM-V1-205..208`). A structural hunk on a host outside the frozen
structural runtime tuples is the verifier's `UNSUPPORTED`. The change alters the `cem status`
output the loop records, so its own dogfood check declares `DOGFOOD_OUTPUT_CHANGE=DCW-V0-047`
(V1-1279). A stable map with artifacts outside the repository root is not admitted by OCM.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-140 | `TestCEMV1140ProducerSpecSelection` (`internal/cem/workflow`) | Library defaults stay `cem/0.2` and `cem/0.1`; invalid prepare and begin specs refuse `invalid-arguments` and write nothing |
| CEM-V1-141 | `TestCEMV1141StablePrepareWritesResumesAndReplaces`, `TestCEMV1141StablePrepareGuardsBackups` (`internal/cem/workflow`) | Writes and verifies `ACCEPT`, resumes byte-identical, refuses a 0.x sidecar as different spec and invalid bytes as not a valid CEM document, regenerates the 0.x sidecar with `--replace`; a bound stable map is refused by stable and 0.x prepare with `--replace`; a producer backup is removed on resume or restored, a bound or unreadable backup is refused and kept |
| CEM-V1-142 | `TestCEMV1142StableBeginWritesProducerMap` (`internal/cem/workflow`) | Two stable begins write the same verifying producer map; a 0.x begin is refused on it |
| CEM-V1-143 | `TestCEMV1143StableProducerMutationsStayStable` (`internal/cem/workflow`); `TestMutatorsRefuseStableInput`, `TestMutatorsRefuseStableDestination` | cite, cover and discriminate keep `cem/1.0` and role `bind-chore` and the result verifies `ACCEPT`; every mutator refuses the bound public map |
| CEM-V1-144 | `TestCEMV1144StableStructuralMark` (`internal/cem/workflow`) | `import-reorder` stays `cem/1.0` and verifies `ACCEPT` within ceilings; `rename` on the same hunk is `REJECT` `unproven-mechanical`, `ok: false` |
| CEM-V1-145 | `TestCEMV1145StatusCeilings` (`internal/cem/cli`) | Within ceilings exit 0, `accepted`, counts and empty policy issues; `--max-mechanical 0` exits 1 with one issue; verify ceilings exit 2 |
| CEM-V1-146 | `TestCEMV1141StablePrepareWritesResumesAndReplaces`; `TestStableStatusArgumentsAddArtifactRoot` (`internal/dogfoodflow`) | `nextActions` names the absolute root; status argv adds it only for a stable map |
| CEM-V1-147 | `TestMapHunksReadsStableMap`, `TestCiteStepAdmitsStableMap`, `TestStableStatusArgumentsAddArtifactRoot`, `TestChangeDCWV0049MarksPreparedStableChoreMap` (`internal/dogfoodflow`) | Hunks and cited count match the parsed map; missing plans keep their reasons; the chore map is marked and never refused; live round trip in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1276-dogfood-producer-cem-1-0.md` |
| CEM-V1-148 | `TestBatchCheckAdmitsStableConstituent` (`internal/dogfoodflow`) | Stable ACCEPT admits with `--artifacts`; a 0.x line, a REJECT and another bind fail `constituent-unverified`; a stable line never proves a 0.x map |
| CEM-V1-149 | `TestCEMV1149OCMPrepareAdmitsStableProducerMap`, `TestCEMV1149OCMLinkAdmitsStableProducerMap`, `TestCEMV1149OCMPrepareRefusesUnverifiedStableMap`, `TestCEMV1058LegacyOCMLRFAdmissionUnchanged` (`internal/lrfrepo`) | Prepare binds target and patch digest, status is not invalid, resume works; link records a cited stable hunk and the linked map reads valid; a wrong base and a tampered map write nothing; artifacts outside the root fail closed |
| CEM-V1-150 | `TestCEMV1150StableSidecarGrantsNoReuse` (`internal/localcompletion`) | A committed default (`cem/1.0`) sidecar leaves the check `selected-check-unverified`; sidecar-only reuse bound only a retired `cem/0.x` map, so under `CEM-V1-210` no sidecar grants reuse and its 0.x reuse test is retired; since V1-1303 the test runs without the plan flag and the stable grant path is `CEM-V1-205` |
| CEM-V1-151 | `TestDogfoodFinishRunsFromBinaryInForeignRepository`, `TestTraceGitignoreFinishThenChangeCheckSeal` (`cmd/corvint`) | The default prepare writes `cem/1.0`; finish reports, is reviewed and is satisfied, and the next change checks and seals |
| CEM-V1-152 | `TestAggregateVerifierStableArtifactRoot` (`internal/dogfoodflow`); `TestAggregateFinishNativeForeignRepository`, `TestTransportAdaptedRecoveryNative` (`cmd/corvint`) | The worker root, or another spelling of it, is admitted; another, relative or NUL-bearing root, a misspelled flag, an extra flag and `--artifacts` on an OCM read are refused; the aggregate finish over a default `cem/1.0` map passes its final check |

Rollback: revert the V1-1276 change. `cem prepare` again writes `cem/0.2` and refuses a stable
sidecar, the loop and batch-check again refuse `cem/1.0`, and OCM again refuses it. A `cem/1.0`
map already sealed stays valid for `cem status`/`cem verify-stable`, but the reverted
`batch-check` refuses it, so a batch over it needs this change or a rebind under `--spec
cem/0.2`. No wire, schema, packet or frozen case changes.
## Closure-chain stable consumers (V1-0632 C06..C08)

Status: accepted under the owner's standing auto-accept (2026-10-11) after independent review
(Codex round 2 PASS); delivery stays library-level plus two explicit CLI flags, unpromoted. Native work: V1-0632, migration rows C06 (TCQ/frontier), C07 (obligation
sidecar) and C08 (criterion experiment companion) of `protocol/cem-1.0/stable/MIGRATION.json`,
built on the C05 admission (`CEM-V1-051..059`). Each family gains a stable route beside its
unchanged 0.x route. Every stable route admits a `cem/1.0` map only through the shipped stable
verifier's `ACCEPT` over the exact bytes, with an explicit repository, expected base, target and
artifact root, and with the exact committed sidecar (`EXACT`). Every emitted document carries a
distinct stable profile; no 0.x profile, identity domain, default or emitted byte changes, and no
0.x document is relabeled. `MIGRATION.json` is digest-pinned by
`protocol/cem-1.0/stable/manifest.json` and is not edited here; its rows stay
`OPEN_NOT_IMPLEMENTED_BY_THIS_PROPOSAL` until that packet is regenerated. (V1-0595 later set
every row's `currentStatus` to its implemented or retired state and re-bound the manifest entry.)

- `CEM-V1-120`: (accepted by decision 0546, 2026-10-20) `tcq.EvaluateStable` MUST emit a distinct profile `tcq-stable/0`, whose identity
  has the prefix `tcq-stable:sha256:` and is hashed in the domain `corvint-tcq-stable/0`. `tcq/0`
  stays frozen: it keeps admitting `cem/0.2` only, with its identity domain unchanged.
- `CEM-V1-121`: (accepted by decision 0546, 2026-10-20) The stable TCQ stage 3 MUST admit only a bounded object declaring exactly
  `cem/1.0`. Its upstream stage MUST be a `StableUpstreamVerifier`, whose refusal passes through
  unchanged, with no result. `tcq.Evaluate` and the `frontier/0` upstream MUST keep refusing a
  `cem/1.0` map, and the stable route MUST refuse `cem/0.2`.
- `CEM-V1-122`: (accepted by decision 0546, 2026-10-20) The stable result MUST carry:
  - the stable profile;
  - `cem/1.0`;
  - the accepted map and patch digests;
  - an identity that differs from the `tcq/0` identity of the same computation.

  Claims, units, issues and relations MUST come from the unchanged `tcq/0` machinery. A
  supplied cached result MUST verify byte for byte, and an edited cached result refuses with a
  typed TCQ code.
- `CEM-V1-123`: (accepted by decision 0546, 2026-10-20) The verifier binding MUST name the SHA-256 of the exact input map bytes, and the
  OCM MUST bind that map and the accepted patch digest; anything else refuses. The
  `frontierrepo` upstream seam MUST return its memoized accepted binding only for the exact
  verified CEM and OCM bytes, by digest, and the accepted revisions. Any other input runs a real
  stable verification.
- `CEM-V1-124`: (accepted by decision 0546, 2026-10-20) The 0.x TCQ and frontier routes are unchanged. `parseArtifacts` keeps the
  `tcq/0` stage-3 check through `parseArtifactsWith`, and the existing `internal/tcq`, `internal/frontier` and
  `internal/frontierrepo` suites pass unchanged.
- `CEM-V1-125`: (accepted by decision 0546, 2026-10-20) `frontierrepo.RecomputeStable` MUST:
  1. admit the map and OCM only through `lrfrepo.VerifyStableUniverse`;
  2. open the target under the stable repository boundary;
  3. recompute TCQ under `tcq-stable/0` against the same accepted map;
  4. revalidate the boundary.

  It returns the stable universe, its obligations carried field for field, the stable TCQ result
  and its per-edge claims. A declared criterion link or a caller discrimination witness MUST NOT
  match a TCQ claim or grant adequacy.
- `CEM-V1-126`: (accepted by decision 0546, 2026-10-20) No stable frontier document is produced. `frontier/0` binds an `lrf/0` digest, and
  no stable LRF or frontier profile is accepted, so the fail-closed choice is
  `StableFrontierState` `NOT_PRODUCED`. That state is never `EMPTY` or closed. Unknown obligations
  cross with their reasons, and nothing closes them.
- `CEM-V1-127`: (accepted by decision 0546, 2026-10-20) Each of the following MUST refuse, keeping the stable verifier's full result and
  returning no upstream:
  - a wrong base;
  - edited map bytes;
  - a tampered artifact;
  - a missing artifact root.
- `CEM-V1-130`: (accepted by decision 0546, 2026-10-20) `extevidence.StableObligations` MUST emit `external-frontier-obligations-stable/0`,
  with an identity prefix of `obligations-stable:sha256:`. Its `binding` names:
  - `cem/1.0`;
  - the exact map SHA-256, the accepted patch digest, base and target, and the excluded path;
  - the verifier outcome;
  - the impact receipt digest and tool.

  `external-frontier-obligations/0` stays frozen and admits `cem/0.2` only.
- `CEM-V1-131`: (accepted by decision 0546, 2026-10-20) Hunk associations MUST be computed exactly as in the frozen sidecar, with the same
  join, bound, freshness and state rules. Every unknown association, including
  `no-external-section`, is kept.
- `CEM-V1-132`: (accepted by decision 0546, 2026-10-20) A verifier refusal MUST return a `StableObligationsRefusal` that holds the
  complete result. It unwraps to a kernel error carrying the verifier code and produces no
  document. A sidecar other than `EXACT`, or an accepted result that does not bind the exact map
  digest, base and patch, MUST refuse with `invalid-obligations-input`. Amended by V1-2798 (GitHub #1895): an `ABSENT`
  sidecar is admitted only under the explicit opt-in of `CEM-V1-639`; `MISMATCH` and
  `UNSUPPORTED_KIND` still refuse.
- `CEM-V1-133`: (accepted by decision 0546, 2026-10-20) The map's declared criterion links MUST be listed in a separate `criteria` member,
  each giving its criterion, ticket, hunks, evidence and runner receipts with satisfaction
  `NOT_OBSERVED`. They are never associations and close no obligation.
- `CEM-V1-134`: (accepted by decision 0546, 2026-10-20) The frozen sidecar MUST keep refusing `cem/1.0`, and the stable sidecar MUST refuse
  a `cem/0.x` map before verification, both with `invalid-obligations-input`.
- `CEM-V1-135`: (accepted by decision 0546, 2026-10-20) `corvint obligations` MUST select the stable sidecar only when all four of these
  flags are given:
  - `--repository`
  - `--expected-base`
  - `--target`
  - `--stable-artifact-root`

  A partial set is an argument error, with exit 2. A stable refusal is an error envelope with
  exit 2 and no stdout. Without the flags, the verb is the unchanged frozen sidecar, which reads
  no repository. Amended by V1-2798 (GitHub #1895): the optional valueless `--allow-absent-sidecar` (`CEM-V1-639`)
  requires the four flags.
- `CEM-V1-136`: (accepted by decision 0546, 2026-10-20) The criterion experiment companion MUST add the explicit input mode
  `cem-criterion-experiments-stable/0`. A request with that schema yields a plan, receipt and
  summary with the same schema, and a plan never changes mode between phases. A
  `cem-criterion-experiments/0` request keeps its exact frozen behavior and output bytes.
- `CEM-V1-137`: (accepted by decision 0546, 2026-10-20) Stable admission MUST:
  - require bytes declaring `cem/1.0`;
  - run `verify.Stable` with the repository, the request base and target, and the explicit
    artifact root;
  - require `ACCEPT`, an `EXACT` sidecar, and the exact map digest, base, target and patch;
  - require every criterion hunk to be a hunk of the accepted change.

  It MUST NOT read the map's criterion links. Criterion authority stays the native Tasks capture
  and verifier, as in the frozen mode.
- `CEM-V1-138`: (accepted by decision 0546, 2026-10-20) Plan, run and verify MUST each re-admit the plan's map through the stable verifier
  with the invocation's artifact root, which is never plan data. The frozen phases MUST refuse a
  stable plan. A missing artifact root or a tampered artifact MUST refuse verification with
  `StableRefusal`. That refusal keeps the complete result and names the verifier code, else its
  first issue code, else `stable-verification-failed`.
- `CEM-V1-139`: (accepted by decision 0546, 2026-10-20) The two modes never cross:
  - the frozen mode refuses `cem/1.0` and any artifact root;
  - the stable mode refuses `cem/0.2`;
  - a stable request without an artifact root refuses.

  `corvint-cem-experiments --stable-artifact-root DIR` selects the stable phases and prints no
  payload on refusal.

Failure modes:
- A host outside the frozen structural runtime tuples gets the verifier's `UNSUPPORTED` refusal,
  which every route keeps as a refusal.
- A target that commits no sidecar can be `ACCEPT`/`ABSENT` at the verifier, but every stable
  route here refuses it, which is fail-closed.
- The stable TCQ route does not apply the frontier adapter's Python grammar-failure reason rewrite
  (TCQ-V0-047). Such a claim stays abstained with the generic `unsupported-anchor-profile` reason.
- No route promotes, sets a default, or has an MCP surface.

Rollback:
- Remove `internal/tcq/stable.go`, `internal/frontierrepo/stable.go`,
  `internal/extevidence/obligations_stable.go`, `internal/criterionexperiment/stable.go` and the
  test-only `internal/cem/stablefixture`.
- Restore `parseArtifacts`, the companion's single-schema checks and the
  `--stable-artifact-root` flag, and remove the four `obligations` stable flags with their help
  text.

No 0.x profile, default, packet or stored evidence changes.

### Closure-chain stable consumer traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-120 | `TestCEMV1122StableEnvelope` | `tcq-stable/0` profile and `tcq-stable:sha256:` identity in the stable domain |
| CEM-V1-121 | `TestCEMV1121StableAdmissionRefusals`, `TestCEMV1121LegacyRoutesRefuseStable` | `cem/0.2` never enters the stable route; verifier refusal passes through; `tcq/0` and `frontier/0` refuse `cem/1.0` |
| CEM-V1-122 | `TestCEMV1122StableEnvelope`, `TestCEMV1125StableUpstreamAccepts` | Map and patch digests bound; identity differs from `tcq/0`; cached result verifies and an edited one refuses |
| CEM-V1-123 | `TestCEMV1121StableAdmissionRefusals`, `TestCEMV1123StableTCQMemoNeverBypasses` | Binding or OCM digest not the exact map refuses; memo answers only the exact verified bytes |
| CEM-V1-124 | `TestCEMV1121LegacyRoutesRefuseStable` | Frozen routes unchanged; existing `internal/tcq`, `internal/frontier`, `internal/frontierrepo` suites pass |
| CEM-V1-125 | `TestCEMV1125StableUpstreamAccepts` | Verified change yields `tcq-stable/0` bound to map and patch; caller claim stays unmatched |
| CEM-V1-126 | `TestCEMV1125StableUpstreamAccepts`, `TestCEMV1126StableUpstreamKeepsUnknownObligation` | `NOT_PRODUCED` frontier; unknown obligation keeps its reason |
| CEM-V1-127 | `TestCEMV1127StableUpstreamRefusals` | Wrong base, edited bytes, tampered artifact and missing root refuse with the result kept |
| CEM-V1-130 | `TestCEMV1130StableObligationsAccepts` | Stable schema, identity prefix and exact binding; no frozen schema bytes |
| CEM-V1-131 | `TestCEMV1130StableObligationsAccepts`, `TestCEMV1131StableObligationsKeepUnknowns` | Same association kinds as the frozen join; `no-external-section` kept |
| CEM-V1-132 | `TestCEMV1132StableObligationsRefusals` | Wrong base, edited bytes, missing root and tampered artifact refuse with the result kept and a kernel code |
| CEM-V1-133 | `TestCEMV1130StableObligationsAccepts` | Criterion link listed apart with `NOT_OBSERVED`; no association carries a criterion |
| CEM-V1-134 | `TestCEMV1134ObligationRoutesStayDisjoint` | Frozen refuses `cem/1.0`; stable refuses `cem/0.2`; both `invalid-obligations-input` |
| CEM-V1-135 | `TestCEMV1135ObligationsStableMode` | Accept over the four flags; wrong base exit 2 with no stdout; partial flags exit 2; frozen form refuses `cem/1.0` |
| CEM-V1-136 | `TestCEMV1136StableExperimentRunsAndVerifies` | The planning core (`planWithCaptures`, after stable admission, over verified native captures) writes a stable plan and marker; receipt and summary carry the stable schema; `VERIFIED_SATISFIED` over the stable map digest. The live `capture` call is not exercised |
| CEM-V1-137 | `TestCEMV1137StableExperimentAdmission` | Accepts the verified map; wrong base, edited bytes, missing root, tampered artifact and unknown hunk refuse |
| CEM-V1-138 | `TestCEMV1136StableExperimentRunsAndVerifies` | Frozen run and verify refuse the stable plan; missing root and tampered artifact refuse stable verification |
| CEM-V1-139 | `TestCEMV1137StableExperimentAdmission`, `TestCEMV1139StableFlagRefusesWithoutPayload` | Frozen with root, frozen over `cem/1.0`, stable over `cem/0.2` and stable without root refuse; CLI flag prints no payload on refusal |

## Stable projection routes (V1-0632 C13..C17)

Status: accepted under the owner's standing auto-accept (2026-10-11) after independent review
(Codex round 2 PASS); delivery is explicit flags and one explicit corpus operation, unpromoted.
Native work: V1-0632, migration rows C13 (documentation corpus), C14
(witness and collapse), C15 (capsule closure), C16 (console chain pane) and C17 (dashboard) of
`protocol/cem-1.0/stable/MIGRATION.json`, built on the library projections of S5c
(`CEM-V1-070..079`). Each route admits a `cem/1.0` map only on the shipped stable verifier's
`ACCEPT` over the exact bytes, against an explicit expected base, target and artifact root. A wrong
base, edited map bytes, a tampered artifact or a missing artifact root refuses with no projection.
Every 0.x route, profile and default is unchanged and stays until V1-1278; no 0.x document is
relabeled. `MIGRATION.json` is digest-pinned and is not edited here.

- `CEM-V1-160`: (accepted by decision 0546, 2026-10-20) `corvint witness --stable-artifact-root DIR` MUST pass the absolute artifact root to
  the witness compiler, which verifies the head tree's `cem/1.0` map with the repository, `--base` and
  the checked-out HEAD as target. The command MUST print a report only when the source is `BOUND`
  under `corvint-witness-stable/0`, carrying the complete verifier result.
- `CEM-V1-161`: (accepted by decision 0546, 2026-10-20) The witness route MUST fail closed with exit 2, no stdout and the code
  `stable-witness-refused` when:
  - a `cem/1.0` source is anything but `BOUND` (a wrong base, edited map bytes, a tampered artifact,
    or a missing or absent artifact root);
  - `--stable-artifact-root` is given over a source that is not `cem/1.0`, including an absent map.

  Without the flag a 0.x or absent map keeps the `corvint-witness/0` report unchanged.
- `CEM-V1-162`: (accepted by decision 0546, 2026-10-20) `corvint docs corpus cem-stable` MUST require `--artifact`, `--cem`,
  `--expected-base`, `--target` and `--stable-artifact-root`, with an optional `--id`. It verifies
  with `--root` as the repository and prints `corvint-corpus-cem-stable/1` only when the verifier
  accepts the exact map bytes. The artifact root is resolved as an absolute directory and is never a
  `--root`-relative local input.
- `CEM-V1-163`: (accepted by decision 0546, 2026-10-20) A `cem-stable` refusal MUST exit 2 with no stdout and the code
  `corpus-stable-refused`. A partial binding MUST be an argument error. The 0.x `cem` operation MUST
  keep refusing a `cem/1.0` map.
- `CEM-V1-164`: (accepted by decision 0546, 2026-10-20) `corvint-console -stable-artifact-root DIR` MUST select the chain pane's verified
  mode. The pane runs the stable verifier over the specs repository, with the request's `base` as
  expected base, the sealed map's bind commit as target and the operator's artifact root. It MUST
  draw a `cem/1.0` chain only on an `ACCEPT` whose map digest is the SHA-256 of the sealed bytes. The
  accepted chain shows:
  - the verifier outcome on its envelope;
  - the verifier's sidecar state;
  - the verifier's change- and reference-integrity axes.

  The NOT_OBSERVED authority, criterion, runner and authentication axes stay gaps.
- `CEM-V1-165`: (accepted by decision 0546, 2026-10-20) In verified mode, the pane MUST replace the chain with a refusal, drawing no chain
  row, when any of these holds:
  - the `base` is missing or malformed: the refusal is an argument refusal, raised before the
    verifier runs, that asks for `base=<full commit>`;
  - the verifier returns anything but `ACCEPT`, including for a wrong base, a missing artifact
    root, a tampered artifact or edited sealed bytes: the refusal names the verifier outcome, stage
    and code;
  - the verifier does not finish within the console's `-timeout`: the refusal names the timeout.

  An accepted chain's hunk links MUST carry the accepted `base`, so following them stays in
  verified mode. Its reference rows MUST show the verifier's `referenceIntegrity` value in place of
  the display-only "not checked" text, and keep the not-completion qualification.
- `CEM-V1-166`: (accepted by decision 0546, 2026-10-20) Without `-stable-artifact-root`, the display-only NOT_RUN pane of `CEM-V1-077` MUST be
  unchanged. Verified mode MUST NOT change how a 0.x sealed map is drawn.
- `CEM-V1-167`: (accepted by decision 0546, 2026-10-20) The protected local-authority consumer holds a Git object capsule and no
  repository, so it cannot run the stable verifier. It MUST keep refusing an enrollment whose
  `cem.json` declares `cem/1.0`, with `capsule CEM base`, before requesting any object.
  Amended by V1-0632 (`CEM-V1-295..299`): the capsule now admits an exact `cem/1.0` map whose base
  is the enrolled base, derives its object demand from it and leaves verification to the stable
  verifier over the published evidence view; `capsule CEM base` remains the stale-base refusal.
- `CEM-V1-168`: (accepted by decision 0546, 2026-10-20) The dashboard's hash-pinned adapter registry declares no `cem/1.0` profile. So
  `corvint-dashboard-snapshot --cem-ocm-profile` naming `cem/1.0` MUST be
  `DASHBOARD_INVALID_ARGUMENT`, with no stdout, before any path opens. The `cem-ocm-bundle-v0`
  registration MUST keep only its 0.x profiles.
  Amended by V1-1278 (decision 0525 §1, `CEM-V1-217`): the `cem-ocm-bundle-v0` registration and
  its five bundle flags are retired. Every bundle flag and profile, `cem/1.0` and 0.x alike, is
  `DASHBOARD_INVALID_ARGUMENT` with no stdout before any path opens, and the hash-pinned registry
  has no row that names a `cem/` profile.

Failure modes:
- A host outside the frozen structural runtime tuples gets the verifier's `UNSUPPORTED` refusal.
  Every route keeps it as a refusal.
- The console's verified mode needs the operator to add `base=<full commit>` to the chain URL. The
  change listing does not carry it, so a click without it refuses.
- One console artifact root serves every sealed change. A change whose artifacts are elsewhere
  refuses.
- Superseded by V1-0632 (`CEM-V1-290..299`): the dashboard has a separate `cem-stable` read and
  the capsule a stable closure; the bundle registry stays retired. The original text follows.
  The capsule verifier and the dashboard bundle stay without a stable path (`CEM-V1-167`,
  `CEM-V1-168`). Each needs its own accepted contract:
  - the capsule needs an enrollment format carrying the stable artifact roles and a capsule-native
    stable verifier;
  - the dashboard needs a new registry version or registration, verification over the C05 stable
    universe, snapshot fields for the axes and bindings, and new conformance vectors.
- `witnesscollapse.CompileStable` has no command. The collapse contract lists a CLI as a non-goal,
  so it stays library-level. No witness, corpus-CEM or console MCP tool exists, so none is added.

Rollback:
- Remove the `--stable-artifact-root` witness flag and `admitStableWitness`.
- Remove the `cem-stable` corpus operation and its three flags.
- Remove `Options.StableArtifacts`, `VerifyStableChain` and the console flag.
- Remove the five tests named below.

No 0.x profile, default, packet or stored evidence changes.

### Stable projection route traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-160 | `TestCEMV1160WitnessStableRoute` | Accepted map prints `corvint-witness-stable/0` with `BOUND`, `ACCEPT` and the exact map digest |
| CEM-V1-161 | `TestCEMV1160WitnessStableRoute` | Wrong base, missing and absent root, tampered artifact, edited map and flag without a stable map exit 2 with no stdout; 0.x report unchanged without the flag |
| CEM-V1-162 | `TestCEMV1162CorpusStableOperation` | Accepted map prints `corvint-corpus-cem-stable/1` with `PROJECTED`, `ACCEPT`, the map digest and claim rows |
| CEM-V1-163 | `TestCEMV1162CorpusStableOperation` | Wrong base, edited map, missing root and tampered artifact exit 2 with no stdout; partial binding refused; 0.x `cem` refuses `cem/1.0` |
| CEM-V1-164 | `TestCEMV1164ConsoleVerifiedStableChain` | Verified chain shows the `ACCEPT` envelope, `changeIntegrity VERIFIED`, `sidecar EXACT` and keeps NOT_OBSERVED axes |
| CEM-V1-165 | `TestCEMV1164ConsoleVerifiedStableChain` | Missing or malformed base, wrong base, missing root, tampered artifact, edited sealed bytes and a verifier timeout draw only a refusal; a followed hunk link stays verified; reference rows show the verifier value |
| CEM-V1-166 | `TestCEMV1164ConsoleVerifiedStableChain` | Without the root the NOT_RUN envelope renders unchanged |
| CEM-V1-167 | `TestCEMV1167CapsuleRefusesStableEnrollment` | As amended by `CEM-V1-295`: a matching-base `cem/1.0` enrollment reaches the object demand, which refuses an empty capsule; a 0.x map refuses `unsupported-spec` with the fix line |
| CEM-V1-168 | `TestCEMV1168DashboardRefusesStableBundle` | Every retired bundle flag and every profile, `cem/1.0` and 0.x, refuses with `DASHBOARD_INVALID_ARGUMENT` and no stdout; no registry row is `cem-ocm-bundle-v0` or names a `cem/` profile |

## Consumers read cem/1.0 only (V1-1278 S2a, decision 0525 §1 and §6, 2026-10-12)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; V1-1278 stage S2a. S1 (`CEM-V1-200..204`) retired the converter and made the
sealed-change admission `cem/1.0`-only, but left the `cem/0.x` wire in place because the consumers
listed in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1278-s1-remove-0x-core.md` still read or wrote it. This
stage moves every one of those consumers to `cem/1.0`, so that no code outside `internal/cem`
calls `wire.Spec01..03`, `wire.ParseMap`, `wire.Canonical`, `verify.Exact` or `verify.Canonical`,
and S2b can delete them. The `internal/cem` 0.x code, the `cem begin`/`--spec cem/0.2` producers
and the 0.x `cem` CLI routes stay in this stage.

- `CEM-V1-210`: (accepted by decision 0546, 2026-10-20) Every consumer outside `internal/cem` (LRF, OCM, the stable universe, frontier
  and its `frontierrepo`/`frontiernextrepo` adapters, external frontier obligations, `prove`,
  attest predicates, anchor, the console chain pane, the MCP report preview, the receipt bundle,
  witness and witness collapse, the documentation corpus, the criterion experiment, the CEM
  candidate assembler, local completion, `tools/local-authority` and
  `tools/cem-citation-baseline`) MUST admit a map only as `cem/1.0`. A map whose `spec` is
  `cem/0.1`, `cem/0.2` or `cem/0.3` MUST refuse with `unsupported-spec` and, where the consumer's
  error shape carries text, the `CEM-V1-200` recovery line (`wire.RetiredSpecFix`). It MUST
  never be an internal error, a silent fallback to another reader, or a pass. Three routes keep
  the code an accepted requirement already assigns: `prove --attest-cem-v2` refuses with
  `attest-cem-v2-requires-stable-map` (`CEM-V1-065`) and `docs corpus cem-stable` with
  `corpus-stable-refused` (`CEM-V1-076`), each naming the recovery line in its message, and the
  MCP report preview maps `unsupported-spec` into its closed, text-free failure set as
  `cem-map-invalid`. A signed `cem/0` or `cem/v1` attestation still verifies as a claim without
  map bytes (`NOT_RUN`), and refuses every supplied map with `unsupported-spec`. (Superseded since
  V1-1278 S2b by `CEM-V1-218`, under which a `cem/0` or `cem/v1` envelope is a retired predicate and
  never verifies.) A frozen 0.x
  profile whose result was defined over a 0.x map (`lrf/0`, `frontier/0`, `tcq/0`,
  `external-frontier-obligations/0`, the attest v0/v1 predicates) now admits no map: a 0.x map
  refuses as above, and a `cem/1.0` map refuses with `unsupported-spec` or takes that profile's
  existing stable route. This supersedes the "0.x maps keep their reading" clause of
  `CEM-V1-079`, and every other clause in this file or a consumer spec that keeps a 0.x reading
  for a consumer.
- `CEM-V1-211`: (accepted by decision 0546, 2026-10-20) The frontier, LRF/OCM, dashboard, console and MCP projections MUST read the
  stable map through the existing stable routes (`wire.ParseStable`, `verify.VerifyStable` and the
  stable read in `internal/cem/workflow`) and MUST NOT project a 0.x map. The MCP report preview
  admits a map only when the stable reader does; a retired map refuses before any blob is read.
- `CEM-V1-212`: (accepted by decision 0546, 2026-10-20) A frozen conformance suite whose expectations depend on a 0.x map changes only
  through its own contract, and the superseded freeze is recorded under decision 0525 §6:
  `conformance/ocm-v0` regenerates its structural vectors from the real `cem/1.0` producer (only
  `mapSha256` changes); `conformance/frontier-v0` keeps its hand-authored expectations and skips
  every case that needs a 0.x map to verify through the declared capability
  `verified-retired-cem`, while `TestRetiredCEMUniverseRefusesThroughTheEntryPoint` drives the
  refusal for real; `conformance/cli-parity-v0` retires `lrf-evaluate-supported` with a pinned
  `retired` declaration under decision 0525 (`retiredCaseDecisions`), so replay prints `RETIRED
  ... decision=0525` and `SUMMARY parity=103 retired=30`; the `internal/lrf` conformance corpus
  keeps its frozen bytes, runs its validator cases unchanged and skips, as superseded, the four
  cases that read a 0.x map through the retired repository route (`ocm.no-links`,
  `context.01-explicit`, `context.01-default`, `failure.structural`; recorded in
  `docs/specs/lexical-relevance-floor-v0.md`), adding no 0.x consumer path; the core freeze
  retires its open `frontier --json` golden, because no admitted map produces `frontier/0`
  (`CEM-V1-210`), and keeps its 0.x `cem status`/`cem verify` goldens until the 0.x route goes. No
  suite regenerates an expectation from candidate output outside its own regeneration contract.
  Amended by V1-1278 S2b (`CEM-V1-217`, `CEM-V1-219`): with the `frontier/0` engine and the
  `lrf/0` repository route gone, the frontier-v0 fixture tier and the lrf-v0 corpus are retired
  whole rather than skipped, cli-parity retires `cem-begin-map` too (`SUMMARY parity=102
  retired=31`), and the core freeze's `cem status`/`cem verify` goldens are regenerated from a
  `cem/1.0` map.
- `CEM-V1-213`: (accepted by decision 0546, 2026-10-20) The daily loop's admission (`CEM-V1-202`) stays `cem/1.0`-only on every route a
  consumer reaches: the bind loop gives a chore role only to a map that declares `cem/1.0`
  (`markChoreRole` refuses a 0.x map and leaves it byte-identical), the verifier's `cem status`
  gets `--artifacts` only for a stable map, and a sealed 0.x constituent is a retired seal
  (`CEM-V1-203`), never a verified one.
- `CEM-V1-214`: (accepted by decision 0546, 2026-10-20) A test that guards behavior `cem/1.0` keeps MUST be ported to a `cem/1.0` map
  built by the stable producer (`workflow.PrepareOptions{Spec: wire.StableSpec}` plus `Cite`) or
  by `internal/cem/stablefixture`, and keep its assertion. A test that only guarded 0.x behavior
  MUST be deleted, and a test that fed a 0.x map to a consumer becomes a refusal test under the
  admission rule `CEM-V1-210`. A test MUST NOT relabel a 0.x map as `cem/1.0` to reach a stable
  route.

Non-goals: no removal of `wire.Spec01..03`, `wire.ParseMap`, `verify.Exact`, `verify.Canonical`,
`patch`, the 0.x producers in `internal/cem/workflow` or the 0.x `cem` CLI routes and their
goldens (S2b); no change to `protocol/`, `interop/` or the frozen `cem-0.1` packet (S3); no
rewrite of a committed map; no new stable LRF or frontier profile.

Failure modes: a consumer that still decodes a 0.x map fails the acceptance search recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1278-s2a-consumers-cem-1-0.md`; a frozen profile that cannot be
produced from a `cem/1.0` map refuses rather than emitting a partial document; a frontier-v0
runner that verifies a 0.x universe again fails the retired-universe entry-point test (itself
retired with the fixture tier by `CEM-V1-217`; the manifest retirement record now guards it).

Rollback: revert the V1-1278 S2a commits. The 0.x wire and verifier are still in `internal/cem`,
so the reverted consumers compile and read 0.x maps again; no committed map, seal or task record
changes.

### Consumer cem/1.0 admission traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-210 | `TestCEMV1210LRFRefusesEveryMapBeforeAnyInput`, `TestCEMV1210OCMRefusesRetiredMap`, `TestCEMV1210FrontierRefusesARetiredMap`, `TestCEMV1210ProveRefusesEveryMapOutsideTheStableRoute`, `TestCEMV1210FrozenObligationsAdmitNoMap`, `TestCEMV1210AnchorRefusesARetiredMap`, `TestCEMV1218RetiredPredicatesRefuse`, `TestCEMV1210FixtureRecomputeRefusesEveryMap`, `TestAssembleRefusesRetiredAndStableSources`, `TestExportRefusesRetiredMaps`, `TestReadCEMSourceRefusesRetiredMap`, `TestConsoleChainStableRegistry`, `TestStableCollapseRefusals`, `TestStableCEMProjectionRefusals`, `TestBaselineDCWV0049SplitsRowsByCitationClass`, `TestCEMV1150StableSidecarGrantsNoReuse`, `TestCEMV1137StableExperimentAdmission`, `TestCEMV1065RefusesALegacyMap`, `TestCEMReportToolNamesStableRefusals` | Each retired spec refuses with `unsupported-spec` (and the recovery line where the error carries text); no consumer reads a 0.x map |
| CEM-V1-211 | `TestCEMReportToolPreviewsCLIReportWithoutPublishing`, `TestCEMReportToolNamesStableRefusals`, `TestContextAndCEMReportAreBoundReadOnlyAndFramed`, `TestCEMV1058LegacyOCMLRFAdmissionUnchanged`, `TestConsoleChainPanelsAgreeOnObligationHunkEdges` | The MCP preview, OCM/LRF universe and console read the stable map; a retired map refuses |
| CEM-V1-212 | `TestFrozenVectorsMatchTheRealProducer`, `TestManifestPinsRetirementAndArtifacts`, `TestCEMV1210FrontierRefusesARetiredMap`, `TestDecision0308DeclarationsAreClosed`, `TestGPKV0005ManifestWorkerDefault`, `TestCoreVerbsEmitTheFrozenProfiles` | ocm-v0 vectors refrozen from the `cem/1.0` producer; cli-parity retires its cases under decision 0525; the core freeze retires its open-frontier golden. Since V1-1278 S2b (`CEM-V1-217`, `CEM-V1-219`) the frontier-v0 fixture tier and the lrf-v0 corpus are retired whole, so their skip and corpus tests are deleted and the retirement manifest is pinned instead |
| CEM-V1-213 | `TestMarkChoreRoleDCWV0049WritesStableRoleOnly`, `TestStableStatusArgumentsAddArtifactRoot`, `TestBatchCheckCEMV1203RetiredSealHasNoCoverage` | A 0.x map gets no role and no `--artifacts`; a retired seal is not reverified |
| CEM-V1-214 | `TestProveCEMAcceptedMapProvesSupportedHunksAndWritesNothing`, `TestAnchorWritesAVerifiedPointerAndReadsItBack`, `TestHistoricalVerifyArtifactsAndSurvivors`, `TestFrozenVectorsMatchTheRealProducer` | Ported tests build their map with the stable producer or `stablefixture` and keep their assertions |

## Core 0.x removal (V1-1278 S2b, decision 0525 §1 and §6, 2026-10-12)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; V1-1278 stage S2b. S2a (`CEM-V1-210..214`) moved every consumer outside
`internal/cem` to `cem/1.0`. This stage deletes what that left: the `cem/0.1`..`cem/0.3` wire
(`wire.Spec01..03`, `wire.ParseMap`, `wire.ParseMapSpec`, `wire.Canonical`), the 0.x verifier
(`verify.Exact`, `verify.Canonical`), the 0.x producers in `internal/cem/workflow`, the 0.x `cem`
CLI routes and their goldens, the frozen 0.x projection routes and the attest `cem/0`/`cem/v1`
predicates. The mechanics `cem/1.0` inherits (`wire.Map`, structural and mechanical reasons, the
coverage and discrimination witnesses, `internal/cem/patch` and the canonical change derivation in
`internal/cem/verify/verify.go`) stay, because the stable reader, producer and verifier use them.
Owner standing auto-accept of lane requirements (2026-10-11) applies; the requirements stay
proposed until recorded under that acceptance.

- `CEM-V1-215`: (accepted by decision 0546, 2026-10-20) The product MUST ship one CEM wire, `cem/1.0`. No Go package outside the frozen
  S3 packets (`protocol/`, `interop/`, `examples/cem`, `conformance/cem-0.1`) may parse,
  canonicalize, verify or produce a `cem/0.x` map. Every Core read (`cem status`, `cem verify`,
  `cem report`, `cem mark`, the daily loop) MUST refuse a root whose `spec` is not exactly
  `cem/1.0` with `unsupported-spec` before any Git read, and a retired `cem/0.x` root MUST carry
  the `wire.RetiredSpecFix` recovery line (`CEM-V1-200`). The inherited mechanics keep their
  `cem/0.3` meaning without a spec argument: structural reasons are always admitted.
  Amended by V1-1278 S3 (`CEM-V1-220`): the S3 packets are deleted, so no Go package in the
  repository parses a `cem/0.x` map; the portable consumer refuses one (`CEM-V1-222`).
  Amended by V1-2665: `internal/lrf` no longer defines the `cem/0.1` and `cem/0.2` context rows or the
  default lrf/0 context admission, and its evaluation code is deleted; `lrf.Evaluate` refuses every
  context with `unsupported-lrf-context` and `lrf.ValidateStableRequest` is the only admission
  (`internal/lrf/retired_test.go`, `CEM-V1-055`).
- `CEM-V1-216`: (accepted by decision 0546, 2026-10-20) `cem begin` and `cem prepare` MUST accept only `--spec cem/1.0` (the default). A
  retired spec label (`cem/0.1`, `cem/0.2`, `cem/0.3`) MUST refuse with `unsupported-spec` and
  the recovery line; any other value, the experimental candidate included, MUST refuse with
  `invalid-arguments`. A refused begin MUST leave the existing map byte-identical.
- `CEM-V1-217`: (accepted by decision 0546, 2026-10-20) The frozen 0.x projection routes MUST be retired, each with its stable successor
  named in its own contract: `frontier/0` by `frontier-stable-view/0` (`CEM-V1-125`, V1-1304);
  `tcq/0` by `tcq-stable/0` (`CEM-V1-120..124`); `external-frontier-obligations/0` by
  `external-frontier-obligations-stable/0` (`CEM-V1-130`); and `lrf/0` evaluation, which has no
  stable successor, by the stable universe admission only (`CEM-V1-051..059`), so `lrf evaluate`
  refuses every map. A retired route MUST refuse with `unsupported-spec` in its existing closed
  error shape; it MUST NOT emit a partial document. The frozen suites that only exercised a
  retired route (the frontier-v0 fixture tier and the lrf-v0 corpus) are retired whole with a
  pinned retirement record rather than skipped case by case.
- `CEM-V1-218`: (accepted by decision 0546, 2026-10-20) Only the `cem/v2` attestation predicate (`CEMStatementV2`, `CEM-V1-065`)
  remains. A `cem/0` or `cem/v1` envelope MUST fail verification with an error naming
  `cem/v2` as the only CEM predicate (CLI `attest-verification-failed`), whether or not a map is
  supplied. The `prove --attest-cem` and `--attest-cem-v1` flags are removed and MUST be
  refused as unrecognized arguments (`invalid-arguments`). This supersedes the `NOT_RUN`
  clause of `CEM-V1-210` and the v0/v1 clauses of `FPK-V0-030..035` and `FPK-V0-050`.
- `CEM-V1-219`: (accepted by decision 0546, 2026-10-20) The core compatibility freeze and cli-parity-v0 MUST change only under decision
  0525 §6, which supersedes decision 0357 for these cases: the core freeze's `cem status` and
  `cem verify` goldens are regenerated from a `cem/1.0` map read with `--artifacts`
  (`stableVerification.spec` = `cem/1.0`), and cli-parity retires `cem-begin-map` with a pinned
  `retired` declaration (`SUMMARY parity=102 retired=31`). No other frozen expectation changes.

Non-goals: no change to `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`,
`interop/cem01-go`, `conformance/cem-0.1`, `examples/cem`, `tools/cem-interop-runner` or the 0.x
mode of `tools/cem-citation-baseline` (S3); no rewrite of a committed map, seal or task record;
no new stable LRF evaluation; no new attestation predicate. Amended by V1-1278 S3
(`CEM-V1-220..224`): those S3 paths are now deleted or `cem/1.0`-only.

Failure modes: a reintroduced 0.x parser or verifier call fails the acceptance search recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-v1-1278-s2b-core-removal.md`; a 0.x map handed to any Core read or
producer refuses with `unsupported-spec` and the recovery line instead of an internal error; a
retired attestation envelope fails closed; a retired frozen suite that is edited without its
retirement record fails its manifest test.

Rollback: revert the V1-1278 S2b commit. The S2a consumers already read `cem/1.0` only, so the
revert restores the 0.x code without changing any committed map, seal, golden outside this change
or task record.

### Core 0.x removal traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-215 | `TestCEMV1200RetiredSpecRefusedWithMergeFix`, `TestCEMV1215ReadsRefuseEveryNonStableSpec`, `TestCoreStableReadExplicitRefusals`, `TestMarkRefusesARetiredMapBeforeMutation`, `TestInheritedStructuralReasons`, `TestInheritedExcludedPath`, `TestInheritedCoverageWitness`, `TestInheritedDiscriminationWitness`, `TestMapArmsCarryAPreparedCEM10Map`, `TestSidecarBaseRevisionReadsAnyLayout` | Core reads refuse every non-stable root before a Git read; the inherited mechanics keep their behavior without a spec; the trial tool's map arms no longer read cem/0.x and build a cem/1.0 map instead (CRT-V0-005, 2026-10-24); the dogfood check reads a stable sidecar's base revision (V1-1326) |
| CEM-V1-216 | `TestCEMV1140ProducerSpecSelection`, `TestCEMV1142StableBeginWritesProducerMap`, `TestCEMV1216CommandSpecRefusals` | Retired labels refuse with the fix line, other values with `invalid-arguments`; a refused begin leaves the map unchanged; the command parser leaves the judgement to the producer |
| CEM-V1-217 | `TestCEMV1210FrontierRefusesARetiredMap`, `TestCEMV1210FrozenObligationsAdmitNoMap`, `TestObligationsCommandRefusesRetiredMap`, `TestUpstreamProfileRefusals`, `TestStableSectionsMatchOracle`, `TestCacheVerificationAcceptsComputedBytes`, `TestCEMV1210LRFRefusesEveryMapBeforeAnyInput`, `TestManifestPinsRetirementAndArtifacts`, `TestArtifactDigestDriftFails`, `TestCEMV1168DashboardRefusesStableBundle` | Retired routes refuse in their closed shapes; the TCQ oracle is ported to `tcq-stable/0`; the frontier-v0 tier is retired by its manifest record |
| CEM-V1-218 | `TestCEMV1218RetiredPredicatesRefuse`, `TestCEMV1218RetiredAttestationFlagsAreUnrecognized`, `TestProveCEMAttestationVerifyRefusesAnEnvelopeThatDoesNotVerify`, `TestProveCEMAttestationVerifyNamesTheRetiredPredicate`, `TestCEMV1065RefusesALegacyMap` | v0/v1 envelopes fail closed and the CLI names cem/v2; the removed flags are unrecognized; v2 still refuses a 0.x map |
| CEM-V1-219 | `TestCoreVerbsEmitTheFrozenProfiles`, `TestCEMEndToEndAndGoldenEnvelope`, `TestDecision0308DeclarationsAreClosed`, `TestGPKV0005ManifestWorkerDefault` | Core-freeze goldens regenerated from `cem/1.0`; cli-parity replays `parity=102 retired=31` |

## Packet retirement (V1-1278 S3, decision 0525 §1 step 7, 2026-10-13)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; V1-1278 stage S3, the last step of decision 0525 §1. S2b (`CEM-V1-215..219`)
left the product with one wire, `cem/1.0`, but kept the frozen `cem/0.x` packets, examples,
portable routes and specs. This stage retires them. Git history keeps every deleted byte.
Owner standing auto-accept of lane requirements (2026-10-11) applies; the requirements stay
proposed until recorded under that acceptance.

- `CEM-V1-220`: (accepted by decision 0546, 2026-10-20) The `cem/0.x` protocol and conformance packets MUST be deleted:
  `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`,
  `conformance/cem-0.2` and `docs/CEM-CI.md`. No tracked file outside history, frozen evidence,
  generated indexes and retired spec bodies may depend on those paths, and the acceptance search
  in the build-log entry classifies every remaining `cem/0.x` mention.
- `CEM-V1-221`: (accepted by decision 0546, 2026-10-20) `protocol/cem-1.0` MUST be self-contained. The 0.x normative text that `cem/1.0`
  inherits is kept under `protocol/cem-1.0/stable/inherited/` (`cem-0.1-algorithms.md`,
  `cem-0.3-algorithms.md`, `cem-0.3.schema.json`) and listed in the packet `manifest.json` with
  its SHA-256 digest and byte count; no other packet file changes. The patch, simulation and
  repository vectors the inherited mechanics are tested against move unchanged to
  `interop/cem01-go/testdata/inherited-vectors/`, and the EOL policy exemption moves with them.
- `CEM-V1-222`: (accepted by decision 0546, 2026-10-20) `interop/cem01-go` MUST verify `cem/1.0` only. The `cem/0.1` default `verify`
  route, `verify-03` and `ci` are retired: an invocation that names a map whose strictly parsed
  `spec` is a `cem/0.x` string MUST exit 1 with `unsupported-spec` and the fix line naming
  `verify-stable-canonical`, and every other invocation outside the `cem/1.0` commands MUST exit 2
  with `invocation`. The `ci` report, the in-toto envelope and the 0.3 packet tests are deleted.
  This retires `CEM-V1-PSR-001..003`, `FPK-V0-035` and `CEM-PILOT-020..023`.
- `CEM-V1-223`: (accepted by decision 0546, 2026-10-20) `examples/cem` except the 0.x-free `examples/cem/experiments/` companion demo, its recipe script test and gate member (`cem-recipes-test`), and
  `tools/cem-interop-runner` MUST be deleted, with their Makefile, gate-ledger, CI shard and
  unbounded-reader entries. This retires `CEM-V1-PSR-004` and `CEM-PILOT-024..027`.
  `tools/cem-citation-baseline` has had no 0.x mode since S2a and is unchanged.
- `CEM-V1-224`: (accepted by decision 0546, 2026-10-20) The Core remnants MUST name no retired wire: the dogfood loop passes
  `--artifacts` to every `cem status` it runs (`stableStatusArguments`), the experimental
  `frontiernextrepo` fixture recompute refuses every map and returns an empty universe, and the
  patch grammar errors name "the CEM patch grammar" instead of `cem/0.1`. Their codes are
  unchanged.
- `CEM-V1-225`: (accepted by decision 0546, 2026-10-20) The `cem/0.2`, `cem/0.3`, 0.1 Go portability probe and 0.1 external interop specs
  MUST be marked retired by decision 0525 and superseded by this contract, keeping their bodies
  and the requirement IDs `REQUIREMENTS.tsv` traces. Live specs and guides whose 0.x sentences
  describe current behavior (the LRF, change-frontier profile, witness, pilot kit, dogfood, install,
  automation and architecture documents, `README.md` and `ROADMAP.md`) MUST be amended, and
  `LICENSING.md` MUST list exactly the surviving Apache-2.0 paths without widening the boundary.

Non-goals: no change to a `cem/1.0` packet file other than the inherited text and its manifest
entry; no rewrite of a committed map, seal, task record, decision or build-log entry; no edit of
frozen benchmark or performance outputs; no removal of the `docs/cem-0.1.schema.json` and
`docs/cem-0.2.schema.json` files that `protocol/cem-1.0/ALGORITHMS.md` cites; no new 1.0
behavior.

Failure modes: a link or citation into a deleted packet fails `line-citations-check` and is
rewritten as prose pinned to commit `5a056d0a`; a traceability row naming a deleted test fails
`traceability-tests-check` and is rewritten as a retirement note; a 0.x map given to the
portable consumer is refused rather than verified; a manifest entry that drifts from the inherited
file fails the packet digest check.

Rollback: revert the V1-1278 S3 commit. The packets, examples, runner and portable 0.x routes
return unchanged; no committed map, seal, golden or task record depends on them.

### Packet retirement traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-220 | `TestRetiredRouteRefusesEveryZeroXMap` | The acceptance searches in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-13-v1-1278-s3-retire-packets.md` |
| CEM-V1-221 | `TestGoldenHunkIdentities`, `TestFrozenInvalidPatches`, `TestFrozenValidPatchesSimulate`, `TestNormativePatchFixtures` | The inherited vectors read from their new path; `script/check-eol-policy_test.sh`; the manifest digest equals the file's SHA-256 |
| CEM-V1-222 | `TestRetiredRouteRefusesEveryZeroXMap`, `TestStableArtifactBoundary` | Every 0.x map refuses on `verify`, `verify-03` and `ci`; the stable route is unchanged |
| CEM-V1-223 | None; a deletion has no behavior to test | `make test-commands-check` reads the literal `GATE_STEPS` list without the removed member; the acceptance search finds no `examples/cem` path other than the kept `experiments/` demo, and no runner path, outside history |
| CEM-V1-224 | `TestStableStatusArgumentsAddArtifactRoot`, `TestCEMV1210FixtureRecomputeRefusesEveryMap`, `TestFrozenInvalidPatches` | Codes unchanged; no 0.x name in the messages |
| CEM-V1-225 | `TestIndexCoversSpecsAndHeaders` | The retired specs, `INDEX.json` supersession and `README.md` rows agree; `license-headers-check` passes |

## Shared stable tamper matrix (V1-1340, 2026-10-13)

Status: accepted by decision 0546 (2026-10-20); formerly proposed; V1-1340. No native test on the stable path covered the classic tamper classes:
the native tamper suites went through the retired `cem/0.1` path, and the portable consumer had its
own cases. Running one shared matrix showed five tampers on which the consumers disagreed, and the
matrix now pins the inherited normative disposition for each. Owner standing auto-accept of lane
requirements (2026-10-11) applies; the requirements stay proposed until recorded under that
acceptance.

- `CEM-V1-226`: (accepted by decision 0546, 2026-10-20) The packet MUST carry one shared tamper matrix at
  `protocol/cem-1.0/stable/tamper-matrix/`: a repository recipe, one clean map (`stable.json`),
  tampered copies of it under `cases/`, and `EXPECTED.json` (profile `cem-stable-tamper-matrix/1`),
  every file listed in the stable `manifest.json` with its SHA-256 digest and byte count. It MUST
  cover at least these tamper classes: `PatchDigestMismatch`, `FabricatedEvidenceID`, `UncitedHunk`,
  `InvalidSpan`, `SpanOutOfRange`, `EvidenceIDSwap` (between hunks), `EvidenceDrop`, `HunkReorder` and
  `SpanRangeEdit`, plus the clean `control`.
- `CEM-V1-227`: (accepted by decision 0546, 2026-10-20) The native verifier (`internal/cem/verify`, `Stable`) and the portable consumer
  (`interop/cem01-go`, `verify-stable-canonical`) MUST each run every case of the matrix from the
  packet files and return the pinned verdict. A case that only one consumer runs, or a disagreement
  between them, is a failure.
- `CEM-V1-228`: (accepted by decision 0546, 2026-10-20) Each `EXPECTED.json` case MUST carry `name`, `tamper`, `description`, `map`,
  `exit`, `outcome`, `stage`, `code`, `issueCodes` and `citation`. A REJECT pins the exact stage and
  closed code (`code` for the wire and references stages, `issueCodes` for verification) and a null
  `citation`. An ACCEPT pins the whole `cem-citation-support/0` block. The pinned codes follow the
  inherited normative text: a duplicate evidence ID, a basis naming no evidence record, a basis
  pair repeated within one hunk, and an evidence record no basis cites are map-local wire checks
  refused `invalid-field`. An empty or
  reversed span is admitted by the wire and refused at verification as `evidence-unavailable`, as
  is a span past the end of its blob. A map whose hunk count differs from the canonical patch is
  the verification issue `patch-hunk-count`, the stable closed name for the inherited `uncited-hunk`.
- `CEM-V1-229`: (accepted by decision 0546, 2026-10-20) Swapping evidence IDs between hunks, when every record stays a valid digest-bound
  base span, MUST ACCEPT. A support tier never changes the verdict (`CEM-V1-080`). The matrix pins the
  degraded tier instead: both hunks go from `anchored` to `cited`, `hunksByClass.anchored` from 2
  to 0, and `specificity` from 2 to -2.

Non-goals: no change to an existing packet file other than the stable `manifest.json` entries; no
new closed code; no tier threshold or verdict that depends on a tier; no change to native verifier
semantics.

Failure modes: a case file that drifts from its manifest entry fails both consumers' manifest
check. A recipe that no longer builds the pinned commits fails before any case runs. A consumer
whose stage or code differs from the pinned value fails that case by name. A matrix that drops a
required tamper class fails the coverage check in both tests.

Rollback: revert the V1-1340 commit. The matrix directory and its manifest entries go away, and
the portable consumer returns to `invalid-field` for a hunk-count mismatch, to `unresolved-reference`
or verification `invalid-field` for an unknown basis, uncited evidence or a repeated basis pair,
and to a wire
`invalid-field` for an empty or reversed span. No committed map, seal or golden depends on those
codes.

### Shared tamper matrix traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-226 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Both check every matrix file against the stable manifest and the required tamper classes |
| CEM-V1-227 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Each runs every case listed in `EXPECTED.json` as a named subtest (the case count is that file's, not restated here) |
| CEM-V1-228 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Exit, outcome, stage, code, issue codes and the full citation block are compared per case |
| CEM-V1-229 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Subtest `evidence-ids-swapped` pins ACCEPT with anchored 0, cited 2 and specificity -2 |
| CEM-V1-750 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable`, `TestCEMV1750HunkOrder` | Subtest `hunk-reorder` pins REJECT `invalid-field` on both consumers; the native unit test refuses a swapped pair and accepts patch order |
| CEM-V1-751 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Subtests `span-hash-recomputed` and `evidence-ids-swapped` pin ACCEPT; `span-end-edited` and `span-hash-mismatch` pin REJECT |
| CEM-V1-752 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Both check every matrix file and the preserved `revisions/r2/` files against the stable manifest; build log 2026-11-01-cemint-u10 retains the committed-corpus order scan |
| CEM-V1-753 | `TestCEMV1753LegacyEnvelopeNamesLinkedWorktree` (`interop/cem01-go`) | A linked worktree refuses `unsupported-repository-envelope` on `verify-stable` and ACCEPTs on `verify-stable-canonical`; a `git clone --shared` clone refuses `unsupported-object-alternates` |
| CEM-V1-754 | `TestCEMV1754PortableVerdictLedgerMatchesNative` (`internal/gitnotes`, which builds `interop/cem01-go`); `readVerdictLedger` in `interop/cem01-go/verdict_ledger.go` | Fourteen ledger cases (absent, two verified, six `malformed`, two `chain-broken`, two `scope-mismatch`, one `conflict`) give the native listing's exact members, and the repository is unchanged; an unresolvable commit refuses `repository-object-unavailable` on both; a relative repository refuses `invalid-arguments`; `TestCEMV1754PortableLedgerAdmissionAndBoundsMatchNative`: a subdirectory, a bare clone, an alternates-backed clone and effective `info/attributes` refuse with the native code, a linked worktree and a repository below an ancestor directory with search but no read permission read the native listing, a note of 1 MiB is `malformed` and one byte more refuses `git-output-exceeded`, and a notes listing of 51150 lines is read while 51151 lines refuse `git-output-exceeded`, on both readers |

## Anchor pointer scope and notes-ref rollback (V1-1260, decision 0523 P1)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1260. Decision 0523
proposed this as `CEM-V1-087`; the V1-1260 ticket and lane charter number it `CEM-V1-067`, and
`CEM-V1-087` stays unassigned. The 1.0 scope is `PRS-V1-018` (1.0 scope per the V1-0020
dependency). Before this change a `corvint-cem-anchor/0` pointer bound any commit: `cem anchor
--commit REV` attached HEAD's committed map to REV, and `cem provenance` checked only the map
path, blob and digest, so a sibling commit with the same parent read `verified`. A notes ref
reset to its parent read as a missing anchor. The `wire.ParseMap` part of the ticket was already
fixed by V1-1278 (`CEM-V1-065`, `CEM-V1-210..214`): anchoring and provenance parse the map with
`wire.ParseStable`. Decision 0355 is not rewritten; the build log carries the superseding note
for it and for the `/0` pointer text of `falsifiable-packet-v0.md` FPK-V0-037/038.

- `CEM-V1-067`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1260) `cem
  anchor` MUST write the pointer profile `corvint-cem-anchor/1`, which adds the members `base`
  and `patch_sha256` (the map's `change.baseRevision` and `change.patchSha256`) and
  `target_tree` (`REV^{tree}`) to the `/0` members. Anchor MUST refuse with
  `anchor-scope-mismatch`, writing no note, when the canonical `base..REV` patch (the
  `gitauth.CanonicalDiff` recipe every CEM consumer uses, which excludes the map sidecar) does
  not hash to the map's `patchSha256`. `cem provenance` MUST report the anchor row `verified`
  only when the note's map blob, path and digest check as before, the map parses as `cem/1.0`
  with `baseRevision` and `patchSha256` equal to the pointer's, `REV^{tree}` equals
  `target_tree`, and the canonical `base..REV` patch hashes to `patch_sha256`; otherwise it
  reports `scope-mismatch`, or `scope-unavailable` when the patch cannot be derived (for example,
  an absent base object). A `/0` pointer MUST stay readable and is reported `unscoped`, never
  `verified`; a `/0` pointer naming any scope member (even empty or null), a `/1` pointer
  missing one, or a note with bytes after its one pointer object, is `malformed`. `cem
  provenance --commit REV --notes-tip OID` MUST add a `notes_tip` member
  (`ref`, `pinned`, `live`, `state`) and report `state` `rollback`, with an anchor row in state
  `rollback`, when OID is not an ancestor of the live `refs/notes/corvint` tip or the ref is
  absent; otherwise `state` is `current`. Without `--notes-tip` the result has no `notes_tip`
  member. OID MUST be a full lowercase hexadecimal object ID, because a revision name would
  resolve to the live tip it is meant to check; anything else is refused `invalid-arguments`,
  and an OID that names no commit is refused `repository-object-unavailable`. When
  `refs/notes/corvint` moves between the tip comparison and the end of the read, provenance
  MUST refuse with `provenance-notes-moved` rather than report rows from a different tip.
  Under `--notes-tip` the anchor note MUST be read from the live tip commit captured for the
  state, by object ID, never again through the mutable ref, so a ref moved away and back during
  the read cannot supply other content (amended 2026-10-21 under the owner's standing auto-accept,
  V1-2157, matching `CEM-V1-345`).
  Provenance stays read-only: it writes no ref, note, object, index, config or worktree file.

Wire and schema: the pointer profile changes from `/0` to `/1` for new notes and the
`cem provenance` result gains the optional `notes_tip` member and the anchor states `unscoped`,
`scope-mismatch`, `scope-unavailable` and `rollback`; the stable map, `cem-stable-verification/1`
and every other provenance row are unchanged. Readers of `/0` notes keep working, so the change
is additive for readers. It is not additive for a consumer that treated a `/0` `verified` row as
trusted: that row is now `unscoped`.

Non-goals: no default-on or automatic anchoring; anchoring stays an explicit operator step. No
read command writes anything. `verified` does not imply that the map's citations or criteria
verify, who wrote the change, or (without `--notes-tip`) that the notes ref was not rolled back.
No automatic migration of existing `/0` notes.

Failure modes: an existing `/0` note on REV blocks re-anchoring with `anchor-note-conflict`; the
operator removes it deliberately (`git notes --ref=refs/notes/corvint remove REV`) and anchors
again. A map whose base object is absent from the repository is refused by anchor and reads
`scope-unavailable` in provenance. A `--notes-tip` the operator did not retain from a trusted
source proves nothing about rollback; the pin is only as good as where it came from.

Rollback: revert the V1-1260 commit. New anchors are written as `/0` again, provenance reports
`/1` notes `malformed`, and `--notes-tip` becomes an unrecognized argument. No committed map,
seal or golden depends on the pointer profile.

### Anchor pointer scope traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-067 | `TestCEMV1067SiblingCommitIsScopeMismatch`, `TestCEMV1067VerifiedImpliesTheAnchoredPatchIsBaseToREV`, `TestCEMV1067ResetNotesRefIsRollback`, `TestCEMV1067LegacyPointerIsReadableAndUnscoped`, `TestCEMV1067PinnedNotesTipReadIgnoresARefMovedAwayAndBack` (`internal/gitnotes`); `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `TestAnchorActionsParseLikeTheirSiblings` (`internal/cem/cli`) | A sibling commit is refused by anchor and every forged `/1` note on it reads `scope-mismatch`; each `verified` row's `patch_sha256` equals the recomputed canonical `base..REV` digest and `target_tree` equals `REV^{tree}`; a reset or deleted notes ref reads `rollback` under `--notes-tip`, and a ref moved A->B->A during that read reports only A's note; a `/0` note reads `unscoped`; no ref moves on any read |

## Portable Linux process containment (V1-1358, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1358.
The portable consumer's owner/keeper process containment was qualified only on Darwin, so
`verify-stable-canonical` refused every Linux run with `runtime` /
`unsupported-process-containment`, and the experimental Tasks verdict gate (TCB-V0-016/017) could
grant only on Darwin. This section qualifies an explicitly bounded Linux route in
`interop/cem01-go` and supersedes, for that consumer on Linux amd64 and arm64 only, the earlier
statement that portable standalone Linux remains pending. The Linux route was written from this
specification and `protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md`; it copies no code from the
AGPL native reaper, and the module stays Apache-2.0 under `LICENSING.md`.

- `CEM-V1-180`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) The
  portable consumer's process containment MUST be qualified exactly on Darwin and on Linux
  amd64 and arm64, chosen by build constraint. Every other operating system and architecture
  MUST keep refusing with `runtime` / `unsupported-process-containment` before admission or any
  spawn. Both qualified routes share one owner/keeper implementation: one Git child in the
  keeper's private process group, retirement proven by absence, and no keeper or Git process
  outliving its owner.
- `CEM-V1-181`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) On Linux
  the consumer MUST probe its primitives before admission and refuse with
  `unsupported-process-containment` unless all of these hold: the owner is not PID 1,
  `pidfd_open(2)` succeeds on the owner itself (Linux 5.3 or later), `epoll_create1(2)` succeeds,
  and `/proc/self/stat` is readable and agrees with the owner's parent and process group.
- `CEM-V1-182`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) The Linux
  keeper MUST watch the owner PID that the owner names in the keeper's argument vector at spawn,
  never whichever process is its parent when it runs. It MUST refuse a named PID that is not a
  canonical decimal greater than 1 or is not its parent, then open a pidfd on that PID and
  re-check after opening that its parent is still that PID. It MUST wait for control input and for its Git child (through a pidfd on
  the unreaped child) in one close-on-exec epoll set together with the owner's pidfd. Owner exit,
  a failed open or any epoll failure MUST end in the keeper killing its own group and exiting, the
  same fail-closed path as the Darwin kqueue watch.
- `CEM-V1-183`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) Linux
  answers signal 0 successfully for a group that holds only zombies, so on Linux step 1 of the
  two-step retirement MUST NOT end on signal 0. After the owned group SIGKILL, with the keeper
  still unreaped, the group is quiet only when `/proc` shows the keeper as a zombie (`Z` or `X`)
  whose parent is the owner and whose process group is its own PID, one complete `/proc` listing
  shows every process in that group as a zombie with at most one thread and includes the keeper,
  and the keeper still qualifies after the listing. The observation sends no signal. Each `stat`
  read is bounded at 4,096 bytes, each `status` read at 16,384 bytes and the listing at 2^22
  entries. Any read failure, parse failure or exceeded bound counts as not quiet, which ends in
  `HOLD` at the retirement bound. Step 2 is unchanged on both systems: reap the keeper once, then
  only signal-0 `ESRCH` within the two-second post-reap cap proves absence.
- `CEM-V1-184`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) On a
  qualified Linux host `verify-stable-canonical` MUST return the same outcome and `verdictSha256`
  as the native verifier for the same sealed map, and the shared tamper matrix and every
  applicable public full-result case MUST pass there as on Darwin. The qualified hosts for the
  portable leg of the Tasks verdict gate are Darwin and Linux amd64/arm64 (TCB-V0-030).

Non-goals: no change to the native verifier, the packet under `protocol/cem-1.0`, any closed code,
result field or retirement bound; no Linux architecture other than amd64 and arm64; no cgroup,
namespace or `PR_SET_CHILD_SUBREAPER` containment; no claim about escaped sessions or
cross-device topology.

Failure modes: a kernel older than 5.3, a missing or unreadable `/proc`, a seccomp policy that
denies `pidfd_open`, or an owner running as PID 1 refuses before admission. A member that
survives the group SIGKILL, a zombie leader with a live thread, or an unreadable member keeps the
group not quiet and ends in `HOLD` (`unsupported-process-containment`), never a silent pass. A
process created in the group after the listing cannot exist, because every member has a pending
fatal signal and Linux refuses `fork` to such a task. On Linux the PID 1 refusal also keeps the
keeper's owner watch meaningful.

Rollback: revert the V1-1358 commit. `stable_process_unix.go` returns to
`stable_process_darwin.go`, the Linux files are deleted, Linux refuses
`unsupported-process-containment` again, and `TestTCBV0016GateVerdictRunsBothConsumers` returns
to skipping `NOT_RUN` off Darwin. No packet, committed map, seal or result byte depends on the
Linux route.

### Portable Linux process containment traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-180 | `TestStableLifecycleLingeringDescendantRetired`, `TestStableLifecycleHostileDescendantsRetired`, `TestStableLifecycleCancelRetiresDescendants` (Darwin and Linux); build constraints of `stable_process_{unix,darwin,linux,other}.go` | `go vet` of the module for darwin/arm64, linux/amd64, linux/arm64, linux/386, linux/riscv64, windows/amd64 and freebsd/amd64 (build log) |
| CEM-V1-181 | `TestStableLinuxOwnerSupported` | Linux arm64 container run as uid 1000 (build log) |
| CEM-V1-182 | `TestStableLifecycleKeeperOwnerExitMidFrame`, `TestStableLifecycleKeeperOwnerExitWhileHolding`, `TestStableLifecycleKeeperOwnerExitWhileRunning`, `TestStableLifecycleKeeperRefusesForeignOwner` | Pass on Darwin and on Linux arm64 with `-race` |
| CEM-V1-183 | `TestStableLinuxZombieOnlyGroupRetires`, `TestStableLinuxLiveMemberBehindZombieLeaderHolds`, `TestStableLinuxQuietRequiresOwnZombieLeader`, `TestStableLinuxProcStatParsesHostileCommandName`, `TestStableLifecycleRetireTwoStep`, `TestStableLifecycleRetireBoundHold`, `TestStableLifecycleRetirePostReapBoundIsInsideOuterBound` | Signal 0 still succeeds on the zombie-only group while the `/proc` proof retires it |
| CEM-V1-184 | `TestTCBV0016GateVerdictRunsBothConsumers`; `TestStableCanonicalFullResults`, `TestStableCanonicalLedgers`, `TestCEMV1226TamperMatrixPortable` | Native and portable `verdictSha256` both `14ac383d61741ab12c6a422d086701302b230295ff97babcb443aead1cb40476` on linux/arm64 and darwin/arm64 (build log); linux/amd64 is the CI `go-product-shard` run |

## Runner receipt admission and the Tasks chain (V1-0632, 2026-10-20)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-0632. The plain producer
(`CEM-V1-240`) treats its runner plan and receipt as opaque files, so it would bind a failed,
skipped, stale or mismatched receipt as readily as a passing one. Nothing showed the whole chain
from a native Tasks attempt through the runner, the producer and both verifiers. This section adds
one opt-in producer check and one end-to-end fixture chain. It changes no wire member, verifier
rule, map byte or `corvint cem` verb.

- `CEM-V1-280`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) A
  `cem-stable-plain-assembly/1` request MAY carry one more optional member, `runnerAdmission`.
  Its only accepted value is `corvint-test-runner/0`; any other value, including the empty string
  and `null`, is refused before any read. Only an absent member keeps the runner files opaque,
  exactly as `CEM-V1-240` states. When it is present
  the producer applies `CEM-V1-281..285` in that order, after the source-map check and before it
  creates the bundle, and a refusal writes no bundle at all. An admitted receipt yields the same
  map, artifacts and result as the opaque path except the receipt record's `profile`, which is
  `corvint-test-runner-receipt/0` only when admission ran (amended by `CEM-V1-310`), and
  the receipt record keeps `executionAuthority` `CALLER_OBSERVED`.
- `CEM-V1-281`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The plan
  and receipt MUST decode as closed `corvint-test-runner-plan/0` and
  `corvint-test-runner-receipt/0` documents, and every plan input path MUST be a literal
  repository path (checked just before the `CEM-V1-283` read); otherwise the request is refused
  with `runner-document-invalid`.
- `CEM-V1-282`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  receipt MUST bind the plan: its `planSha256` is the plan's identity, its execution
  `inputSha256` is the identity of the plan's input files, and its execution runner is the plan's
  runner. Otherwise the request is refused with `runner-receipt-tampered`.
- `CEM-V1-283`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The plan
  MUST pin 1 to 4096 input files, and each MUST be a blob of the request's `target` whose SHA-256
  equals the pin, read by one bounded `git cat-file --batch`. An empty or oversized set, an absent
  path, a non-blob or a different digest is refused with `runner-receipt-stale`.
- `CEM-V1-284`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Every
  observed test MUST be `PASSED` or `SKIPPED`, the observation MUST be complete, and the receipt
  MUST carry no execution error; otherwise the request is refused with `runner-receipt-failed`.
- `CEM-V1-285`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) At least
  one observed test MUST be `PASSED`; a receipt with only skipped tests or none is refused with
  `runner-receipt-skipped`.
- `CEM-V1-286`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) One
  fixture test MUST drive, on a private disposable native Tasks store and one fixture checkout:
  a native ticket claim; real `corvint-test-runner` plans and receipts built from repository
  sources; the plain producer with admission; a bind and seal; and the store's `cem-verdict` gate
  running the built `corvint cem gate-verdict` with the portable consumer built from
  `interop/cem01-go`. The gate MUST pass with outcome `GRANT`, native `ACCEPT` with sidecar
  `EXACT`, portable `ACCEPT`, equal native and portable `verdictSha256`, and every governed path
  satisfied. Before that, the producer MUST refuse the runner's real failed, skipped, stale and
  tampered receipts with `runner-receipt-failed`, `runner-receipt-skipped`,
  `runner-receipt-stale` and `runner-receipt-tampered`, writing no bundle, and MUST accept the
  failed receipt when admission is absent.
- `CEM-V1-287`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) In the
  same chain, a runner artifact edited after production MUST be refused three ways. Tasks refuses
  the dirty worktree with `DIRTY_WORKTREE`. The gate's native and portable recomputations both
  `REJECT` at stage `artifacts`, the native issue being `artifact-digest-mismatch`, with equal
  verdict digests, so the gate answers `CEM_VERDICT_UNVERIFIABLE`. A committed edit fails the
  Tasks gate with `CEM_VERDICT_PRODUCER_MISMATCH` / `sealed-tree-mismatch`.

Non-goals:
- no runner execution by the producer; admission reads the two documents and the target's blobs
- no receipt signing or authentication (`FPK-V0-036`, V1-1264); a caller can still forge a
  self-consistent passing receipt
- no verifier-side admission: a portable consumer still checks the runner files by SHA-256 only
- no new public case, capsule or dashboard (V1-0632 gap 1)

Failure modes: each refusal above names its code and leaves no bundle; a Git read failure is
refused without a code; an unknown `runnerAdmission` value is refused before any read.

### Runner receipt admission and Tasks chain traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-280 | `TestCEMV1280RunnerAdmissionIsOptIn` and `TestCEMV1280PresentAdmissionMemberIsNeverOpaque` (`internal/cemcandidate`); the opaque control in `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-0632-cem10-tasks-chain.md` |
| CEM-V1-281 | `TestCEMV1281To1285RunnerAdmissionRefusals` (opaque public-case files, unknown receipt member, input path escapes) | Fails with the admission call disabled (build log) |
| CEM-V1-282 | `TestCEMV1281To1285RunnerAdmissionRefusals` (another plan, other inputs, another runner); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-283 | `TestCEMV1281To1285RunnerAdmissionRefusals` (earlier source, absent path, no inputs); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-284 | `TestCEMV1281To1285RunnerAdmissionRefusals` (failed test, incomplete observation, execution error); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-285 | `TestCEMV1281To1285RunnerAdmissionRefusals` (only skipped, no tests); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-286 | `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | Darwin arm64 run (build log); Linux is CI |
| CEM-V1-287 | `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | Darwin arm64 run (build log) |

Rollback: remove `internal/cemcandidate/plain_runner.go`, the `runnerAdmission` member and its
call in `internal/cemcandidate/plain.go`, `internal/cemchain/`, and their tests. Requests without
the member are unaffected; no committed map depends on admission.

## Multi-profile runner receipts in the plain producer (V1-2322, 2026-10-24)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11) and the
CEM-V1 auto-accept (2026-10-20); V1-2322 part 1. The plain producer (`CEM-V1-240`) took exactly one
runner plan and receipt, so a change tested by two runners (for example `go-test` and
`node-test`) needed two maps, although the reader already allows 64 `runnerReceipts`. This section
adds one backward-compatible request form. It changes no wire member, reader rule, sealed packet,
public case or `corvint cem` verb.

The requirements are `CEM-V1-573..578` in `## Requirements`.

Non-goals:
- no reader-side admission and no wire change: a reader checks each runner file by SHA-256 and
  artifact kind only, so it cannot tell a plan swapped between two pairs; the producer's
  admission refuses that swap as `runner-receipt-tampered`
- no opaque multi-receipt form: `runners` always requires admission
- no change to the single form, its fixed artifact paths or its labels
- no receipt merging or aggregation across runners, and no receipt signing (`FPK-V0-036`)
- no new public case under `protocol/cem-1.0/` and no sealed packet or manifest repin
- no Tasks lifecycle for multi-runner maps (`cem-verdict` closeout and the drift follow-up are
  later V1-2322 work)

Failure modes: each request-shape refusal of `CEM-V1-575` happens before any read and writes no
bundle. An admission refusal of any pair (`CEM-V1-281..285`) or `runner-profile-conflict` happens
before the bundle is created. Any verifier REFUSE or REJECT is refused as in `CEM-V1-240`, leaving
at most the runner artifacts in D. A reader that predates this section reads the same `cem/1.0`
members and needs no change.

### Multi-profile runner receipts traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-573 | `TestCEMV1573RunnersFormReproducesTwoProfileCase` (`internal/cemcandidate`) | The produced map and four artifacts equal `interop/cem01-go/testdata/multi-runner/` byte for byte; each pinned input changed after the ACCEPT is refused with no `stable.json`; `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-multi-runner-producer.md` |
| CEM-V1-574 | `TestCEMV1574SingleFormOutputIsUnchanged`, `TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase` (`internal/cemcandidate`) | The admitted single-form map digest recorded on base `025120e1a` and the opaque `neutral-criterion` bytes are unchanged |
| CEM-V1-575 | `TestCEMV1575RunnerSetRefusals`, `TestCEMV1575NamedMembersOfTheOtherFormAreRefused` (`internal/cemcandidate`) | Each code refused with no bundle, including `null` and empty-pin members decoded from request JSON and 33 receipts on one criterion; a plan swapped between pairs is refused as `runner-receipt-tampered` |
| CEM-V1-576 | `TestCEMV1575RunnerSetRefusals` (`internal/cemcandidate`) | Two `go-test` receipts on one criterion refused; `go-test` plus `node-test` on one criterion ACCEPTed |
| CEM-V1-577 | `TestCEMV1577NativeReaderChecksTwoProfileMap` (`internal/cem/verify`); `TestCEMV1577PortableReaderChecksTwoProfileMap` (`interop/cem01-go`) | `EXPECTED.json` of the case: ACCEPT and six REJECTs, equal on both readers |
| CEM-V1-578 | `TestCEMV1578TasksBoundRequestHasNoRunnersForm` (`internal/cemcandidate`) | `runners` refused as an unknown field |
| CEM-V1-620 | `TestWidenRealPacketCopy`, `TestWidenRefusals` (`tools/cem-stable-regen`); `make cem-stable-regen-check` | Both schema copies change only at the node; cases and cells byte-identical; repeat refused |
| CEM-V1-621 | `TestWidenCommittedProfile` (`tools/cem-stable-regen`); the run in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-v1-2107-result-schema-opaque-widen.md` | Exactly the two labels; `CHANGES.md` names the legacy node digest |
| CEM-V1-622 | `TestCEMV1622OpaqueReceiptResultValidates`, `TestStableSchemaValidatorRefusesUnknownKeywords` (`internal/cem/verify`) | `opaque/1` and the legacy const refused; both results validate against the committed copies with no overlay (V1-2615) |
| CEM-V1-880 | `TestAnyOfRealPacketCopy`, `TestAnyOfRefusals` (`tools/cem-stable-regen`); `make cem-stable-regen-check` | Both schema copies change only at the node; old bytes kept; cases and cells byte-identical; repeat refused |
| CEM-V1-881 | `TestAnyOfCommittedCriterionBinding` (`tools/cem-stable-regen`); `TestCEMV1622OpaqueReceiptResultValidates` (`internal/cem/verify`); the run in `docs/build-log/2026-10-31-bugs-w3-bugs.md` | Tasks form plus plain form; `CHANGES.md` names the predecessor node digest |

Rollback: remove the `runners` and `runnerReceiptSha256s` members, the two `UnmarshalJSON`
methods, `validateRunnerSet`, `checkRunnerProfiles` and `artifactPaths` from
`internal/cemcandidate/plain.go`, the `omitzero` optional-field rule in
`internal/cemcandidate/schema.go`, restore the
single-pair `admitRunnerReceipt` signature, and delete the tests and
`interop/cem01-go/testdata/multi-runner/`. Single-form requests are unaffected. A multi-receipt map
already produced stays a valid `cem/1.0` map for both readers.

## Opaque runner receipt label (V1-2096, #1328, 2026-10-20)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-2096. Without
`runnerAdmission`, the plain producer (`CEM-V1-240`) pins any file as the runner plan and receipt
by SHA-256 alone, yet labelled the receipt record `corvint-test-runner-receipt/0`. A trial bound a
framework JSON report and an unrelated provenance document that way, and both consumers ACCEPTed
the map, so a reader could take an arbitrary file for a Corvint runner receipt. This section gives
an unadmitted receipt its own label. Every committed map, sealed or not, keeps its bytes and stays
valid: the reader accepts the legacy label unchanged, so no sealed CEM is rewritten.

- `CEM-V1-310`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2096) The plain
  producer MUST label its one runner receipt record `opaque/0` when the request has no
  `runnerAdmission` member, and `corvint-test-runner-receipt/0` only when the admission of
  `CEM-V1-280` ran and passed. Every other byte of the map, the artifacts and the result is as
  before; an admitted request's output is byte-identical to its output before this requirement.
- `CEM-V1-311`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2096) A
  `cem/1.0` stable reader MUST accept exactly two `runnerReceipts[].profile` values,
  `corvint-test-runner-receipt/0` and `opaque/0`, with the same fixed `sourceGitBinding`,
  `executionAuthority`, `dependencyClosure` and `authentication` rules for both, and MUST refuse
  any other value at the existing runner-limitation check. `stable.schema.json` and the stable
  `ALGORITHMS.md` state the same set; the candidate shape of `protocol/cem-1.0/ALGORITHMS.md` is
  unchanged.
- `CEM-V1-312`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2096) The native
  and portable consumers MUST ACCEPT the public `neutral-criterion` map with its runner label
  replaced by `opaque/0`, with the same result axes as the legacy-labelled case. Neither label is
  verified by a reader and neither raises any result axis; the label is the producer's statement
  of whether it admitted the bytes.
- `CEM-V1-313`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2096) Maps
  written before `opaque/0` existed, including every sealed map under `.corvint/changes/` and the
  public cases under `protocol/cem-1.0/`, MUST keep their bytes and verdicts; the public
  `neutral-criterion` case keeps its legacy label as the legacy-label control. A legacy label on
  an old map says only that the producer of that time labelled it, not that the bytes were
  admitted.

Non-goals:
- no rewrite of a committed or sealed map, public case result or verdict digest
- no reader-side admission: a reader still checks the runner files by SHA-256 only
- no change to the Tasks-bound `cem-stable-assembly/1` path or the candidate shape
- no new runner admission profile and no new result axis

Failure modes: an unknown runner label is refused by both readers at the existing
runner-limitation check, with the same stage and code as before; a third-party `cem/1.0` reader that still accepts only the legacy label refuses
a newly produced unadmitted map instead of misreading it, which is the safe direction.

Closed limit (V1-2107): both `RESULT.schema.json` copies now admit `opaque/0` beside the legacy
label at `references.runnerReceipts[].profile` (`CEM-V1-621`), through the recorded `--widen`
mode of `cem-stable-regen` (`CEM-V1-620`). Closed limit (V1-2615): the result of a plain map
now also validates at `references.criterionBindings[]`, because both copies admit the
`CEM-V1-083` plain form beside the Tasks form (`CEM-V1-881`), through the recorded `--any-of`
mode (`CEM-V1-880`).

### Opaque runner receipt label traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-310 | `TestCEMV1310UnadmittedReceiptIsLabelledOpaque` and `TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase` (`internal/cemcandidate`) | A framework report and a provenance document pinned without admission yield `opaque/0`; the admitted fixture yields `corvint-test-runner-receipt/0` |
| CEM-V1-311 | `TestCEMV1311StableReceiptLabels` (`internal/cem/wire`); the unknown-label control in `TestCEMV1312PortableOpaqueReceiptLabel` (`interop/cem01-go`) | `protocol/cem-1.0/stable/stable.schema.json` enum and `ALGORITHMS.md` receipt labels paragraph, re-pinned in the stable manifest |
| CEM-V1-312 | `TestCEMV1312OpaqueReceiptLabelVerifies` (`internal/cem/verify`); `TestCEMV1312PortableOpaqueReceiptLabel` (`interop/cem01-go`) | Both consumers ACCEPT with the public axes |
| CEM-V1-313 | `TestCEMV1083NeutralCriterionPublicCase` (native and portable, unchanged bytes); `TestCEMV1311StableReceiptLabels` (legacy label) | No file under `.corvint/changes/` or `protocol/cem-1.0/stable/neutral-criterion/` changes in this change |

Rollback: restore the single legacy label in the producer, both readers, the schema and the stable
`ALGORITHMS.md`, and re-pin the manifest. A map already produced with `opaque/0` would then be
refused as `invalid-field`, so reproduce it before rolling back.

## Packet truthfulness and citation vectors (V1-1333, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1333,
recorded by V1-1370. The Apache-2.0 packet still described stable `cem/1.0` as an unimplemented
proposal, carried the citation rules, the stop list, the result serialization and the diff recipe
only by reference to this AGPL spec, Go code or a retired 0.3 packet, and had no vector that pinned
an anchored hunk on a deletion, a nonzero specificity together with same-path citations, or the
exact citation bytes from the native verifier. Owner standing auto-accept of lane requirements
(2026-10-11) applies and V1-1370 recorded each requirement under it. The
scope is the 1.0 packet scope of `PRS-V1-018`.

- `CEM-V1-107`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) Every status statement in `protocol/cem-1.0/README.md` and
  `protocol/cem-1.0/stable/` (the README files, `ALGORITHMS.md`, the `stable.schema.json`
  description, the `status` members of `OPERATION.json` and `manifest.json`) MUST say that the stable `cem/1.0` wire is implemented and
  produced on main, that its technical contract is proposed and not promoted, and that its
  qualification is bounded. None may call the technical contract accepted. `R1-EXPECTED.json` MUST
  be described as a declarative expectation set that no test executes; its bytes stay unchanged.
- `CEM-V1-108`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) `protocol/cem-1.0/stable/ALGORITHMS.md` MUST state, as new Apache-2.0 prose, every
  rule a consumer needs to reproduce the `cem-citation-support/0` block byte for byte: hunk order,
  hunk path (base path for a deletion), the added-bytes join that keeps each line's LF, the
  requirement ID scan, span-relative defining lines, the `id` and `path` anchors, tiers, annotation
  with the full stop list, fan-out, `samePathCitations` over basis entries, concentrated hunks and
  the shift null behind `invariant` and `specificity`.
- `CEM-V1-109`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) The same file MUST state the member order of the citation block and of the
  top-level result, the compact encoding with its escapes, and the canonical form behind
  `verdictSha256` (sorted names, separators, string escapes, numbers, and the three removed
  members). The member order inside the nested result objects is fixed by `CEM-V1-113`. (Amended in
  place by V1-1369 while still proposed: the earlier clause said that the nested order was not fixed
  and that results compare by digest, citation bytes or parsed value.)
- `CEM-V1-110`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) The same file MUST carry the canonical diff recipe in full: Git options, `-c`
  settings, environment, the attribute source, the path exclusion and the provenance proof the
  output must pass. It MUST name Git's own diff output as the one part defined by reference.
- `CEM-V1-111`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) The packet MUST carry `protocol/cem-1.0/stable/citation-vectors/`: a repository
  recipe, a clean map, refused copies and `EXPECTED.json` (profile `cem-stable-citation-vectors/1`),
  every file listed in the stable `manifest.json`. The expected values MUST come from the native
  verifier. The clean case MUST have anchored hunks, nonzero `specificity` and nonzero
  `samePathCitations`, including a cited deletion hunk anchored through its base path. The refused
  cases MUST include a same-path deletion drift, a stale-span drift, a base mismatch and wire classes
  not in the tamper matrix. The native verifier and the portable consumer MUST each run every case
  and match exit, outcome, stage, codes, drift, `verdictSha256`, top-level member order and the
  citation block bytes.
- `CEM-V1-112`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) For each listed misreading (an LF-stripped join, citation or top-level member order,
  digest separators or escapes, an absolute instead of span-relative defining line, a stop-list
  omission or case-sensitive stop terms, a specificity miscount, a deletion hunk without its path, a
  same-path count over distinct evidence, a deleted cited file not treated as drift), a one-line
  mutation of the native verifier MUST fail at least one citation vector. The packet README MUST
  list each misreading with the cases that fail it.

Non-goals: no change to verifier or producer behaviour; no change to `R1-EXPECTED.json`, the
inherited copies or any frozen digest; no restatement of Git's diff algorithm; no technical
acceptance or promotion of this contract. (V1-1369 removed the non-goal "no fixed member order
for nested result objects"; `CEM-V1-113` now fixes that order.)

Failure modes: a vector file that drifts from its manifest entry fails both consumers' manifest
check. A recipe that no longer builds the pinned commits fails before any case runs. A consumer
whose citation bytes, digest or member order differ fails that case by name. A packet edit that
again calls the contract accepted, or a rule restated wrongly, is caught only by review and by the
vectors that exercise the rule.

Rollback: revert the V1-1333 commit. The vectors and their manifest entries go away and the packet
prose returns to its previous text. No committed map, seal or golden depends on them.

### Packet truthfulness traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-107 | none (prose) | Readback of the packet status statements; build log `2026-10-14-v1-1333-cem-packet-truthful.md` |
| CEM-V1-108 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | The clean case pins the citation block bytes the restated rules produce |
| CEM-V1-109 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | Each case pins `verdictSha256` and the top-level member order; the nested order is traced under `CEM-V1-113` |
| CEM-V1-110 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | Both consumers derive the same canonical patch from the recipe repository |
| CEM-V1-111 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | All 8 cases run as named subtests; both also check the clean case's anchored, specificity and same-path counts and its path-anchored whole-file deletion hunk |
| CEM-V1-112 | `TestCEMV1111CitationVectorsNative` | Scratch mutation run of 13 mutants, all killed; commands and table in the build log |

## Discrimination witness mutant bound (V1-1259, decision 0523 P2, 2026-10-14)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1259. A hunk
`discriminates` witness claiming 1,000,000 mutants under `bounds.maxMutants: 1` was admitted by
both consumers: the count rule checked only `killed + survived <= mutants`. Decision 0523 P2
proposed this ID; it is the native obligation the inherited `ALGORITHMS.md` text waits for before
a consumer may require `mutants <= maxMutants`.

- `CEM-V1-090`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1259) A
  `cem/1.0` hunk `discriminates` witness whose `mutants` exceeds its `bounds.maxMutants` MUST be
  refused at the wire stage with `invalid-field` by the native reader (`internal/cem/wire`,
  `validateDiscrimination`) and the portable consumer (`interop/cem01-go`,
  `stableDiscrimination`); `mutants` equal to `maxMutants` is admitted. A witness whose
  `survived` differs from the number of `survivors` entries, or whose survivor `line` lies
  outside the hunk's `newRange`, stays refused `invalid-field` by both, as the inherited count
  rules already required. Survivors need not be distinct. The shared tamper matrix carries the
  cases `discrimination-mutants-over-bound` and `discrimination-survivors-inconsistent` (wire
  `invalid-field`) and the boundary control `discrimination-at-bound` (ACCEPT), each pinned for
  both consumers under `CEM-V1-227`.

Non-goals: no new closed code; no check of `maxHunks` or `wallTimeSeconds` against anything; no
requirement that `killed + survived = mutants`; no duplicate-survivor rule; no change to the
witness encoding, the producer (`internal/cemdiscriminate` already stops at `maxMutants`) or
`discriminationValidation`, which stays `WIRE_VALIDATED_ONLY`.

Failure modes: a map from an external producer that over-reports mutants now fails the wire stage
instead of verifying. No committed `.corvint/changes` map carries `discriminates`, so no committed
map changes verdict. A consumer that skipped the bound fails the matrix case by name.

Rollback: revert the V1-1259 change commit. Both consumers return to admitting any `mutants`
count, and the three matrix cases and their manifest entries go away.

### Discrimination mutant bound traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-090 | `TestInheritedDiscriminationWitness` (`internal/cem/wire`); `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Subtests `discrimination-mutants-over-bound` and `discrimination-survivors-inconsistent` pin wire `invalid-field` and `discrimination-at-bound` pins ACCEPT on both consumers; without the bound check both consumers fail `discrimination-mutants-over-bound`; the matrix runs the 23 cases of `EXPECTED.json` (the CEM-V1-227 row no longer restates a count) |

## Consumer hunk-revert falsifier (V1-1266, decision 0523 P2)

Status: accepted; `CEM-V1-088..089` under the owner's standing auto-accept (2026-10-11), V1-1266;
`CEM-V1-250` under the same standing auto-accept, V1-1452. Decision 0523 P2 makes `held` the one support tier that a consumer
observes rather than reads: the consumer reverse-applies one hunk and runs the tests the map names
for it. The verifier result and its `verdictSha256` (`CEM-V1-066`) stay exactly as they are; the
falsifier reports beside them, opt-in, under the fixed `go test` runner profile.

- `CEM-V1-088`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) `held`
  MUST come only from the consumer's own re-execution. `cem verify --hunk-revert` adds the envelope
  member `hunkRevert` (profile `cem-hunk-revert/0`, runner `go-test`) beside the unchanged
  `stableVerification` result, and only after an `ACCEPT`. A candidate is a `supported` hunk of
  non-test Go source whose basis cites a `_test.go` span as `test-claim`; its named tests are the
  top-level `func TestX(t *testing.T)` declarations whose headers lie inside the cited span of the
  parsed base file (`TCQ-V0-061`), run in
  the cited file's package directory. The consumer exports the target tree from Git objects into a
  disposable directory under the system temporary directory, reverse-applies exactly that one hunk
  there, and runs only the named tests with `go test -json -count=1 -run '^(names)$' .` under the
  native go-test pins (`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`) plus `GOWORK=off` and
  `GOENV=off`. A row is `held` only when a named test passed at the target and, with the hunk
  reverted, the consumer decoded a test-level `fail` event for it in a package whose own result is
  `fail`; a test-level `fail` in a package that passed is printed output, not a failure, and is
  `not-run: infrastructure`. A row is `survived` when the eligible tests all pass reverted:
  every eligible test reports a terminal `pass` (`CEM-V1-693`). Each run uses its own fresh
  snapshot of the export (`CEM-V1-692`).
  A producer-written witness, including a `discriminates` member, never counts. The caller's
  worktree, index, refs and objects are only read (`git ls-tree`, `git cat-file --batch`, no
  optional locks), and the copy is removed on every exit path. The copy's reverted file is replaced
  only through real directories, never through a symlink, and a Go cache variable that points into
  the caller's repository is moved into the disposable workspace. The report's `hunksByClass` moves
  each held hunk from `anchored` or `cited` to `held`; the in-result `citation` block keeps
  `held` 0. Without the flag the envelope and every output byte are unchanged; `status` and
  `report` refuse the flag `invalid-arguments`.
- `CEM-V1-089`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) A row that
  is not `held` or `survived` MUST be `not-run` with one closed reason, never `held`:
  `revert-breaks-build` (the reverted package does not compile, or the package fails with no
  eligible test result), `fails-without-revert` (no named test passes at the target, including a
  package that does not build there), `skipped`, `test-not-observed`, `timeout` (a per-run bound,
  default 300 seconds, or the test binary's own timeout panic), `total-limit` (the whole falsifier
  bound, default 1200 seconds), `infrastructure` (no `go` binary, an export or decode failure, an
  output bound), `no-named-test` (the cited span declares no top-level test), `not-a-modification`
  (an added or deleted file) and `revert-mismatch` (the target bytes at the hunk's new range are
  not its new side). The report records both bounds and its elapsed seconds.
- `CEM-V1-250`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1452) The
  disposable export of `--hunk-revert`, and of `cem verify --guarding-tests` (`TCQ-V0-062`), MUST
  be created by one shared helper under the system temporary directory, and that helper MUST
  refuse a temporary directory that is, or lies below, the repository's worktree or Git common
  directory, decided by file identity of the directory and each ancestor so that a symbolic link,
  a case alias on a case-insensitive volume or any other spelling counts: nothing is exported or run, and every row that would
  have run is `not-run` with reason `infrastructure`. A repository below the temporary directory,
  the ordinary layout, is unaffected.

Non-goals: no change to `StableResult`, the citation block, `verdictSha256`, the wire, the public
cases or the portable consumer, which does not re-execute; no runner other than `go test`; no
promotion of `held` into the verifier result; no mutation testing (`TCQ-V0-056`); no change to
`cem status`, `report` or any dogfood command.

Failure modes: a hunk whose revert leaves the named test passing is `survived`, so a weak test is
visible rather than credited. A test that fails at the target, a broken build, a skip, a timeout or
a host without Go is `not-run` with its reason. A tree with a symlink, a submodule or a path
escaping the copy is refused as `infrastructure` rather than copied partially. A run whose output
exceeds 32 MiB is `infrastructure`.

Recorded limits: only `go test` re-executes; every other runner is a recorded rc.3 limit, as is the
portable consumer (`interop/cem01-go`), which reports no falsifier. The run environment passes the
host `HOME`, `PATH`, `GOROOT`, `GOPATH`, `GOMODCACHE` and `GOCACHE` through so cached modules and
builds are reused, which the native executor's minimal environment does not; dependencies absent
from the module cache fail offline as `fails-without-revert` or `revert-breaks-build`. Tests are
named only from the cited span; a subtest is never named on its own. The flag executes the target's
test code with the caller's privileges and no filesystem confinement, exactly as `go test` would:
Corvint itself only reads the caller's repository, but code under test can write anywhere the caller
can, and code under test that deliberately fails when its source is reverted makes a row `held`.
Run the flag only on a change whose tests you would run. `CEM-V1-250` keeps Corvint's own export
out of the repository; it does not confine what the tests themselves write.

Rollback: revert the V1-1266 code commit. The flag, the `hunkRevert` member and the falsifier code
go away; no committed map, seal, golden or public case depends on them. `CEM-V1-250` rolls back with
the V1-1452 code commit: both flags again create the export wherever `TMPDIR` points.

### Hunk-revert falsifier traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-088 | `TestCEMV1088HeldOnlyFromConsumerReexecution`, `TestCEMV1088ForgedWitnessNeverCounts`, `TestCEMV1088FlagIsVerifyOnly` (`internal/cem/workflow`); `TestCEMV1088VerifyHunkRevertIsOptIn` (`internal/cem/cli`); `TestCEMV1088JudgeRevertsOnlyInsideTheCopy`, `TestCEMV1088ReverseHunkAppliesExactlyOneHunk`, `TestCEMV1088GoTestProfileMatchesNative`, `TestCEMV1088ExportTreeReadsOnlyObjects`, `TestCEMV1088CopyWritesNeverFollowSymlinks`, `TestCEMV1088CountMovesOnlyAcceptedTiers` (`internal/cem/verify`) | Held only from a consumer-decoded failure; a forged `discriminates` witness on a weak test stays `survived`; the caller's tree including `.git` hashes identically before and after; the stable result is byte-identical with and without the flag |
| CEM-V1-089 | `TestCEMV1088HeldOnlyFromConsumerReexecution` (`internal/cem/workflow`); `TestCEMV1089ClassifyRevert`, `TestCEMV1089DecodeGoTestJSON`, `TestCEMV1088JudgeRevertsOnlyInsideTheCopy` (`internal/cem/verify`) | Real `go test` runs: a revert that breaks compilation and a named test that fails without the revert are both `not-run`; a printed test failure in a passing package (the recorded go1.27.2 stream) is never `held`; timeout, total limit, mismatch, skip and infrastructure reasons pinned |
| CEM-V1-250 | `TestCEMV1250HunkRevertRefusesRepositoryTMPDIR`, `TestCEMV1250GuardingTestsRefusesRepositoryTMPDIR` (`internal/cem/workflow`); `TestCEMV1250WorkspaceRefusesEverySpellingOfTheRepository` (`internal/cem/verify`: worktree, subdirectory, Git directory and, where the volume is case-insensitive, a case alias are refused; a directory outside is used) | With `TMPDIR` set to an existing directory inside the worktree, every row of both flags is `not-run: infrastructure`, no plan row prints, and nothing is written there; without the refusal the same runs export into it and judge the rows |

## Normative nested result member order (V1-1369, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1369.
The packet left the member order inside the nested objects of `drift`, `hunks` and
`references` unfixed, because the two reference consumers wrote it differently. A consumer could
therefore not reproduce a whole result line from the packet alone. Measured at
`b8a2e0336cb88269378f87432104bfc33fc4e89b`, both consumers are deterministic, the native order
follows from Go's sorted map keys and fixed struct order, and the digest was already independent of
nested order. Aligning the portable consumer to the native order needs no change to `internal/cem`
output bytes, so the native order becomes the contract. The scope is the 1.0 packet scope of the
product requirement `PRS-V1-018`.

- `CEM-V1-113`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1369) Every
  `verify-stable` result line MUST be the bytes `protocol/cem-1.0/stable/ALGORITHMS.md` defines,
  including the member order inside every nested object. A `drift` entry has `evidenceId`, `path`,
  `baseBlobOid`, `status`, `targetBlobOid`, `targetSpan`, and a `targetSpan` object has `end`, then
  `start`. Within every object inside a `hunks` entry, at any depth, members are sorted by name.
  `references` has `artifacts`, `criterionBindings`, `criterionLinks`, `runnerReceipts`, and each
  of their entries uses the order and omission rule the packet states. `axes` is sorted by name. The
  packet MUST state this order as Apache-2.0 prose. It MUST pin the whole result lines of every
  `citation-vectors` case and of the `neutral-criterion` ACCEPT as `results/*.json` files listed in
  the stable manifest. The native verifier and the portable consumer MUST each produce those bytes
  exactly. The only exception is `runtime.goVersion`, which names the running toolchain.

Non-goals: no change to the native verifier's output bytes, the verdict digest, the top-level
member order or the citation block; no change to map encoding; no witness-bearing (`coverage` or
`discriminates`) result line is pinned (the public cases have none, so the sorted-hunk rule covers
those members only by statement); no technical acceptance or promotion of this contract.

Failure modes: a consumer that writes any nested member in another order fails the pinned line for
that case by name, in both consumers' tests. A pinned file that drifts from its manifest entry fails
both manifest checks. A toolchain other than go1.27.2 changes only `runtime.goVersion`, which the
tests substitute before comparing.

Rollback: revert the V1-1369 commit. The portable consumer then writes its previous nested order,
the pinned lines and their manifest entries are removed, and `CEM-V1-109` returns to saying that
nested order is not fixed. No committed map, seal or digest depends on nested order.

### Nested result order traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-113 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable`, `TestCEMV1083NeutralCriterionPublicCase` (both consumers) | Nine pinned whole result lines under `citation-vectors/results/` and `neutral-criterion/results/`; the build log records the order measured at base and the mutation check |

## Portable consumer Git capability and the Tasks gate's PATH (V1-1377, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1377.
On the hosted GitHub `macos-15` runner the experimental Tasks verdict gate refused every fixture with
`CEM_VERDICT_VERIFIER_DISAGREEMENT` because `verify-stable-canonical`, run under the gate's fixed
`PATH=/usr/bin:/bin`, answered `UNSUPPORTED` while the native verifier accepted. The retained
diagnostic output of release-gates dispatch 38650914390 names the cause: that PATH resolves Apple
Git 2.39.5, which rejects the `--attr-source` option the stable repository diff passes
(`interop/cem01-go/stable_repository.go` `stableOpDiff`; the native `internal/cem/gitauth` diff
passes the same option), so the consumer's canonical diff child exits non-zero and it refuses at stage `repository`
with the packet's existing `git-diff-failed` code. Process containment (CEM-V1-180..184) is not involved. This
section records the consumer's Git capability contract and the gate-side fix
(`docs/specs/tasks-criterion-binding-v0.md` TCB-V0-040..044). The consumer's code, the packet and
every closed code and result field are unchanged.

- `CEM-V1-185`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The stable
  repository diff requires a Git that accepts `--attr-source` (Git 2.41 or later), because the
  envelope pins attributes to the empty tree through that option. On a Git that rejects it the
  consumer MUST answer `UNSUPPORTED` at stage `repository` with `git-diff-failed`, the code its
  repository envelope already assigns to a canonical diff child that exits non-zero, with
  `accept` `null` and no fabricated patch, verdict or citation. No new code is introduced for
  this and the consumer MUST NOT probe `git --version` in its place: the diff's own failure is
  the proof.
- `CEM-V1-186`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The
  consumer resolves `git` only through the `PATH` its caller gives it, which `stableGitEnv`
  passes to the Git child with the pinned `GIT_*` isolation, and MUST NOT search any other
  location. A caller that needs the consumer and its own verifier to judge the same bytes with
  one Git MUST put that Git's directory first in the consumer's `PATH`; the Tasks gate does so
  with the directory of the Git it resolves itself (TCB-V0-043).
- `CEM-V1-187`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) An
  `UNSUPPORTED` result's only diagnostic is its `stage` and `code`, both members of the closed
  `stage` and `code` enumerations of `protocol/cem-1.0/stable/RESULT.schema.json` and never a
  host path, version string or free text. A gate that pins them (TCB-V0-040) therefore records
  the cause of an unsupported route without recording the host.
- `CEM-V1-188`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The hosted
  GitHub `macos-15` runner is a qualified host for the consumer under a caller-supplied `PATH`
  whose first Git accepts `--attr-source`: there `verify-stable-canonical` MUST return the same
  outcome and `verdictSha256` as the native verifier for the same sealed map, as on Darwin
  (local) and Linux amd64/arm64 (CEM-V1-184). Under `/usr/bin:/bin` alone that runner answers
  `repository` / `git-diff-failed`, which is the correct answer, not a defect.
- `CEM-V1-189`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) A test in
  the Tasks gate (TCB-V0-042) MUST run the real consumer under `PATH=/usr/bin:/bin` and accept
  only the answer that path's Git explains, so a host whose fixed-path Git loses `--attr-source`
  again, or gains it, is observed rather than assumed.

Non-goals: no consumer change, no `git --version` floor enforced by the consumer, no packet
change, no new stage or code, no containment change, no Windows statement.

Failure modes: a caller whose own Git is older than 2.41 sees `repository` / `git-diff-failed`
from the consumer and, in the Tasks gate, the native leg's own `git-diff-failed` (`stable-not-accepted`) or
`portable-route-unsupported`; neither is a grant. A caller that puts an untrusted directory first
in the consumer's `PATH` runs that directory's Git; the gate only ever puts first the directory of
the Git it already trusted for its native reads.

Rollback: revert the V1-1377 commits. The consumer is unchanged by them; the Tasks gate returns to
the fixed PATH and the hosted `macos-15` gate tests fail again with `verifier-disagreement`.

### Portable consumer Git capability traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-185 | `TestTCBV0042PortableConsumerUnderBareHostPath` (real consumer, `PATH=/usr/bin:/bin`); `TestStableCanonicalFullResults` (unchanged) | Hosted `macos-15` run 38650914390: Apple Git 2.39.5 rejects `--attr-source`, consumer `repository` / `git-diff-failed`, native `ACCEPT`; darwin/arm64 Apple Git 2.54.0 accepts and the consumer `ACCEPT`s |
| CEM-V1-186 | `interop/cem01-go/stable_repository.go` `stableGitEnv`; `TestTCBV0043PortableConsumerResolvesTheGateGit` | The consumer's recorded `PATH` and `command -v git` are the gate's Git directory first, then `/usr/bin:/bin` |
| CEM-V1-187 | `TestTCBV0040PortableUnsupportedRefusesNamingTheRoute`, `TestTCBV0016GateVerdictRunsBothConsumers` | The pinned `stage` and `code` are closed tokens; any other shape is unreadable |
| CEM-V1-188 | `TestTCBV0016GateVerdictRunsBothConsumers`, `TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict` on hosted `macos-15` | Release-gates dispatch 38654510601 at 52ad86a36: macos-15 full-gate `success` (`ok internal/taskman`, `ok cmd/corvint`); dispatch 38650914390 at 22fca01e6 retained the bare-PATH answer `repository` / `git-diff-failed` under Apple Git 2.39.5 |
| CEM-V1-189 | `TestTCBV0042PortableConsumerUnderBareHostPath` | Passes on darwin/arm64 (`GRANT`) and on hosted `macos-15` (`portable-route-unsupported`) with the probe output logged |

## Stable consumer argument and reference parity (V1-1291, V1-1306, V1-1289, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1291,
V1-1306, V1-1289. Measured at `ee27168739ec1b65dd5a653d720cf811338c37ae`, the native verifier and
the portable consumer gave different results for the same input in two places. An `arguments` or
`authority-arguments` refusal was `REJECT` (`accept` `false`) natively and `UNSUPPORTED` (`accept`
`null`) in the portable consumer. Four reference-graph mutations got a different `references` code
from each consumer. `OPERATION.json` fixes the stage and exit of both argument stages but not their
outcome, and `ALGORITHMS.md` states the graph precedence only coarsely. This section fixes both
answers so the two consumers agree. The scope is the 1.0 packet scope of the product requirement
`PRS-V1-018` (1.0 scope per V1-0020 dependency).

- `CEM-V1-115`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1291) A
  `verify-stable` refusal at stage `arguments` MUST be `UNSUPPORTED` with `accept` `null`,
  `assurance` `null` and exit 2 in every stable entry point: the native `cem verify-stable`, the
  Core stable read and export, the internal stable universe admission and both portable commands.
  A bad invocation proves nothing about the change, so it is never a rejection of the evidence.
- `CEM-V1-116`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1291) A
  `verify-stable` refusal at stage `authority-arguments` (`expected-base-required`,
  `target-required`, `artifacts-required`, `invalid-arguments`) MUST likewise be `UNSUPPORTED` with
  `accept` `null`, `assurance` `null` and exit 2 in both consumers. Its stage, code and the members
  already decoded (hunks, references) are unchanged.
- `CEM-V1-117`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1306) When an
  `invalid-arguments` refusal follows a relative path option (`--root`, `--repository`, `--map`,
  `--artifacts` or `--trusted-keys`), the consumer MUST keep refusing it (CEM-V1-017: a relative path
  is refused, never resolved) and MUST name the fix beside the result, never inside it
  (CEM-V1-187): one stderr line `<command>: invalid-arguments: relative path option <options>;
  fix: pass each as an absolute path such as "$PWD/<path>"; a relative path is refused, never
  resolved` for the native and portable commands, and the same text in a `fix` member of the Core
  stable read envelope. The absolute form of the same invocation MUST pass both argument stages and
  print no fix line. A repeated option is still refused, and the fix names a relative value in any
  of its occurrences. The closed result bytes are unchanged.
- `CEM-V1-176`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1289) Both
  consumers MUST apply the reference precedence the CEM-V1-176 paragraph of
  `protocol/cem-1.0/stable/ALGORITHMS.md` states. A hunk basis that names no evidence record is
  `wire` / `invalid-field`. A repeated hunk ID is `duplicate-reference`, after runner receipt
  uniqueness and before any link is resolved. A repeated ID inside one link array is
  `duplicate-reference` before any ID of that link is resolved. A declared criterion or runner receipt that no link names is
  `reference-coherence`, after link resolution and basis membership and before artifact role
  closure. A repeated artifact path or digest is `duplicate-reference`, before that record's role
  check. The other cases are `REJECT` at stage `references` with exit 2.
- `CEM-V1-177`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1289) The packet
  MUST carry one public case per CEM-V1-176 rule under
  `protocol/cem-1.0/stable/reference-precedence/`, added by the manifest extension rule (new
  directory and new `manifest.json` entries; no existing vector byte or digest changes), with the
  whole result line each consumer must write. Both consumers MUST run every case and match its line
  byte for byte, `runtime.goVersion` excepted.

Non-goals: no new stage or code; no change to an `ACCEPT` or a Git-reading result; no change to any
sealed S0E cell, full-result case or committed map result; the fix line is not a machine contract
beyond its prefix and is never part of `verdictSha256`; a relative path is still never resolved.
V1-1295 (partial drift on an `UNSUPPORTED` resource-limit result) and V1-1297 (a separate code for
a missing pinned object) are not decided here; the build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-batch4-cem-parity.md` records why both stayed open. Decision 0532 later
resolves V1-1295 with `CEM-V1-260` and kept `git-read-failed` for V1-1297; decision 0535 then
amends `CEM-V1-103` to `repository-object-unavailable`.

Failure modes: a caller that branched on `accept` `false` for a bad invocation now sees `null`;
every in-repository caller (receipt bundle export, stable universe, Tasks gate verdict) already
treated exit 2 as not accepted. A consumer that checks a link's IDs one at a time, or the role
before the artifact repeat, fails the pinned `reference-precedence` line for that case by name in
both consumers' tests. A stderr reader that expected silence on a refusal sees one line.

Rollback: revert the V1-1291 commit. The native verifier then returns `REJECT` for argument
refusals again, the reference order and the fix line are removed from both consumers, and the
`reference-precedence/` directory and its manifest entries are deleted. No committed map, seal or
digest depends on these answers.

### Stable consumer argument and reference parity traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-115 | `TestCEMV1115ArgumentRefusalsNative` (`internal/cem/cli`); `TestCEMV1115ArgumentRefusalsPortable` (`interop/cem01-go`); `TestStableExportPublishesNothingForARefusedSource` (`internal/receiptbundle`) | The same argv gives `UNSUPPORTED` `arguments` / `invalid-arguments`, `accept` `null`, exit 2 on the native command and both portable commands; at base the native command answered `REJECT` |
| CEM-V1-116 | `TestCEMV1115ArgumentRefusalsNative`, `TestCEMV1115ArgumentRefusalsPortable`; `TestCEMV1052StableUniverseAuthorityArguments` (`internal/lrfrepo`) | `artifacts-required` and a relative `--artifacts` are `UNSUPPORTED` `authority-arguments` in both consumers |
| CEM-V1-117 | `TestCEMV1115ArgumentRefusalsNative`, `TestCEMV1117CoreReadNamesRelativePathFix` (`internal/cem/cli`); `TestCEMV1115ArgumentRefusalsPortable`; `TestCEMStableReadKeepsCallerRoot`, `TestCEMV1084VerifyStableTrustedKeysFlag` (`cmd/corvint`) | Relative `--root`, `--repository`, `--artifacts` and `--trusted-keys` print the fix line (envelope `fix` in the Core read), also when the relative value is the second of a repeated option; each absolute form passes both argument stages with empty stderr |
| CEM-V1-176 | `TestCEMV1177ReferencePrecedenceNative` (`internal/cem/verify`); `TestCEMV1177ReferencePrecedencePortable` (`interop/cem01-go`) | At base five of the six cases differed: portable `REJECT` `verification` (issue `patch-hunk-count`, exit 1) against native `references` / `duplicate-reference` for a repeated hunk ID, native `unresolved-reference` against portable `duplicate-reference` for a repeated unknown link ID, portable `unresolved-reference` against native `reference-coherence` for an unused criterion or runner receipt, and portable `artifact-role-closure` against native `duplicate-reference` for a repeated artifact |
| CEM-V1-177 | `TestCEMV1177ReferencePrecedenceNative`, `TestCEMV1177ReferencePrecedencePortable`; `TestStableS0EPublicCases`, `TestStableCanonicalFullResults` (unchanged) | Six cases and pinned lines under `protocol/cem-1.0/stable/reference-precedence/`, listed in the stable manifest; existing vectors byte-identical |

## Self-modified governing authority is never cited (V1-0414, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0414.
Ticket V1-0414 requires that a governing file changed in the diff under review is not cited as
authority for hunks in the same change. At `495eda08cf3dc4d8122b169a4cc16d9c4e599b19` the packet
downgraded such a file (TCP-V0-057) and `cem status` reported a citation of it
(`selfModifiedAuthority`, CEM-CB-026), but `cem cite` still wrote the citation whenever the cited
span was stable: an agent could edit `AGENTS.md` and cite its unchanged lines as the
`specification` basis of that same edit. The threat and its two external references are recorded
in `task-context-packet-v0.md` ("Authority screen for reserved rows"). The scope is the 1.0 scope
per the V1-0020 dependency.

- `CEM-V1-114`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0414) `cem cite`
  MUST refuse, with the producer-only closed code `cite-self-modified-authority`, any evidence path
  that is a governing instruction file under `workflow.GoverningInstructionPath` (the CEM-CB-026
  rule, equal to TCP-V0-008's `documentKind`) and is the path of a hunk of the same map, whatever
  the span, the relation or the hunk selected. The refusal precedes the span-stability check and
  writes no map bytes. A stable span of a changed non-governing file, and any span of a governing
  file the patch does not change, are cited as before. The verifier, `cem status` and the wire are
  unchanged: a map written by another route that carries such a citation still verifies and is
  reported by `selfModifiedAuthority`.

Non-goals: no change to verification, the verdict digest or any committed map; no refusal in
`verify-stable` or `cem status`; no screen of a governing file's old path in a rename (the hunk
path is the new path, as in CEM-CB-026), of governing names `documentKind` does not classify, or of
a non-governing spec the change edits.

Failure modes: a change that edits a governing file and cites its unchanged lines now fails at
`cem cite`; its author cites another file (the spec or decision the edit implements) or keeps the
hunk `unknown` with a recorded reason. The dogfood plan runner reports the code as the `cem-cite`
reason with its own fix hint, and the self-observation ledger admits the code (both added by V1-1385
after this section was written).

Rollback: revert the V1-0414 change commit. `cem cite` then writes such citations again and
`cem status` keeps reporting them.

### Self-modified governing authority traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-114 | `TestCiteRefusesSelfModifiedGoverningAuthority`; `TestStatusReportsSelfModifiedAuthority` (hand-written citation still reported) | Disabling the refusal makes the first test fail on the `AGENTS.md` hunk; build log `2026-10-14-v1-0414-closeout-evidence.md` |

## Self-modified governing authority in `cem status` (V1-0414, 2026-10-16)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0414.
Ticket V1-0414 requires that a governing file changed in the diff under review is reported as
self-modified authority in `cem status`. The report shipped with PR #467 and was specified only as
`CEM-CB-026`, an experimental requirement of `cem-0.2-canonical-binding.md`, which decision 0525
retired (`CEM-V1-225`). The `cem/1.0` status read (`internal/cem/workflow/stable_read.go`) still
emits the member, so this section restates the contract for `cem/1.0` without changing it. The
threat and its two external references are recorded in `task-context-packet-v0.md` ("Authority
screen for reserved rows") and `docs/SECURITY.md`.

- `CEM-V1-270`: (accepted under the owner's standing auto-accept (2026-10-11), 2026-10-16; V1-0414)
  For a `cem/1.0` map, `cem status` MUST report every governing instruction file the map's own
  patch changes, under `workflow.GoverningInstructionPath` (the rule of TCP-V0-008's
  `documentKind`: basename `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` or `copilot-instructions.md`,
  case-insensitive, or a `.github/instructions/*.instructions.md` file). The envelope then carries
  `selfModifiedAuthority`, an array sorted by path of `{path, hunks, citedBy, reason}`: `hunks`
  lists the IDs of the hunks changing that path, and `citedBy` lists, in map order, every
  `{hunk, evidenceId, relation}` basis whose evidence row cites that path (empty when none does).
  The member is absent when the patch changes no governing file, so such an envelope is
  byte-identical to one written before the report. The report never changes `state`, `ok`, the
  verifier result, the map or any other command's output, and status refuses nothing: refusal of a
  new citation is `CEM-V1-114`'s, at `cem cite`. It supersedes `CEM-CB-026` for `cem/1.0`, which
  keeps its ID for traceability only.

Non-goals: no new member, code or state; no screen of a renamed governing file's old path (the hunk
path is the new path), of names `documentKind` does not classify, or of non-governing specs the
change edits; no hidden-Unicode screen of map evidence (the packet screens reserved rows,
TCP-V0-055).

Failure modes: a consumer that ignores the member treats a self-modifying change as ordinary; the
member is advisory beside an unchanged verdict, so CI that must block such a change reads it
explicitly.

Rollback: delete this section and its traceability row; the code and the wire are unchanged by it.

### Self-modified authority status traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-270 | `TestStatusReportsSelfModifiedAuthority`, `TestStatusOmitsSelfModifiedAuthorityWithoutAGoverningChange`, `TestSelfModifiedAuthorityListsAnUncitedGoverningChange` (`internal/cem/workflow`); `TestCEMGoverningInstructionRuleMatchesThePacket` (`internal/contextindex`) | build log `2026-10-16-v1-0414-governing-authority-acceptance.md` |

## Stable manifest entry repair (V1-1287, 2026-10-14)

The packet seal (fc5588948) wrote `protocol/cem-1.0/stable/manifest.json` before the final bytes of
two files, so two entries never matched their files: `REPOSITORY-ENVELOPE.cells.json` recorded
sha256 `c3eed8b8371322ffb6d03a7bd3065fbf956c1c6d312c92826699a406c396d257` / 302453 bytes and
`REPOSITORY-ENVELOPE.md` recorded `9c35b7a5e681c80fb148cd85d972f3200d67c838782242aa41116590a4f3dbab`
/ 9705 bytes. The first regeneration under `CEM-V1-091` (af8f466dd, the V1-1265 `citation` member)
re-bound both entries to the bytes it wrote (`ecbc86f8aab33a9548ae9cf9cb4f6e02d3198ff13d9d01a92814a78ccb1609c5`
/ 316109 and `89385b7ae4a910a4f2588264547f75353b0e30f144b7ae0851e118678b111a25` / 10003), and later
regenerations keep them current. Until then `tools/cem-stable-regen --check` deliberately checked
only the `RESULT.schema.json` entry of this manifest; it now checks every entry, as it already did
for the packet and tuple manifests, and passes on the repository packet.
`TestCheckCoversEveryStableManifestEntry` appends one byte to two stable files no other check reads
(`PACKING.md`, `tamper-matrix/README.md`) and requires `--check` to name each drifted entry; it fails
against the schema-only check. The stable manifest also lists the
`repository-envelope-packet/` files, so a regeneration now re-binds each listed packet file it
rewrites and the packet `manifest.json` there too (`TestRegenerateRealPacketCopy` checks the
result); without that a regenerated copy fails its own widened check. No sealed byte changed in
this repair. Rollback: restore the schema-only filter in `check()` and drop the packet-entry
re-binding from the stable manifest step; the manifest needs no change.

## Drift Git accounting parity (V1-1295, decision 0532, 2026-10-15)

Status: `CEM-V1-260` accepted under the owner's standing auto-accept (2026-10-11); V1-1295.

Intent: two stable consumers that read the same map at the same commits under the same 1,024
logical Git operation limit write the same result, including a result the limit cuts short in the
drift stage. V1-1295 measured the f0fda542 map ending `UNSUPPORTED` `repository`
`unsupported-resource-limit` with 68 drift rows on the native verifier and 102 on the portable
consumer, so their `verdictSha256` differed. V1-1295 (2026-10-14) had already kept the partial drift
list in an `UNSUPPORTED` result, because the sealed S0E full-result cases pin one; what remained
unpinned was the drift stage's own per-record charge. The native verifier read each changed
record's base blob a second time (three operations per record); the portable consumer searched the
span bytes the evidence stage had already authenticated (two). With the native re-read removed both
consumers write 102 rows and verdict
`92747a603414f0d4a08dd4ce10df19afeec2d00bbec18e7a2d6eda16a4db2d10`, byte-identical results.

- `CEM-V1-260`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1295) Drift
  (`protocol/cem-1.0/stable/ALGORITHMS.md` item 12) MUST charge, for each evidence record in
  evidence-ID order, one target path lookup, and one target blob read only when the target entry is
  a 100644 or 100755 blob whose OID differs from the record's `blobOid`. The span bytes it searches
  MUST be the bytes the evidence stage authenticated; drift MUST NOT read a base blob. When the
  operation limit refuses an operation during drift, the result is `UNSUPPORTED` `repository`
  `unsupported-resource-limit` and keeps exactly the rows completed before the refused operation, so
  every conforming consumer writes the same `drift` list and `verdictSha256`. The packet MUST carry
  the public case `protocol/cem-1.0/stable/drift-resource-limit/`, added by the manifest extension
  rule (a new directory and new `manifest.json` entries; no existing vector byte or digest changes
  other than `ALGORITHMS.md`, whose item 12 states the charge), with the whole result line. The case
  declares a lowered operation limit (`operations`, 113) that each consumer applies only through its
  in-package test seam (`stableSeam.operations` natively, `stableHooks.callLimit` in
  `interop/cem01-go`); production always uses 1,024. Both consumers MUST run it and match the line
  byte for byte, `runtime.goVersion` excepted.

Non-goals: no change to the limit, to any other stage's charge, to the drift statuses or to an
`ACCEPT`, `REJECT` or completed drift result; no change to any sealed S0E cell or full-result case
(their partial lists hold only `stable` rows, which read no blob, so both charges agree on them); no
dropping of the partial drift list from an `UNSUPPORTED` result.

Failure modes: a consumer that re-reads a base blob or skips a record's target lookup stops at a
different row on this case and fails it. Every record of the case cites a changed blob, so a
consumer that also read the target blob of an unchanged record (identical OID) is not caught by
it. A record whose authenticated span length is not its declared span length fails closed
`evidence-unavailable` rather than searching other bytes. A case that spent the full 1,024
operations was too slow to run (14 minutes natively; the portable consumer passed the 30-minute
verification deadline at load 65 to 76), hence the lowered seam limit; a charge difference that
shows only past operation 113 is caught by the f0fda542 re-run above, not by the case. The case is
skipped only where stable process containment is not qualified.

Acceptance evidence: on the case (limit 113) the fixed native verifier and the portable consumer
write byte-identical lines, 13 drift rows and verdict
`a5611d4da50659d8f87988a89f7515413b6ed12df21abc801760b866f17d33d6`; restoring the base re-read
makes the native verifier write 8 rows and fail its case test. At the full 1,024 limit on a 340-record
form of the same map the BASE native verifier (253d6341f) wrote 103 rows and the fixed one 154.
Build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-cem-stable-consumer-residuals.md`.

Rollback: revert the change commit. The native verifier then re-reads base blobs in drift again, the
case and its manifest entries go, and V1-1295 reopens as a recorded difference.

### Drift Git accounting traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-260 | `TestCEMV1260DriftResourceLimitNative` (`internal/cem/verify`); `TestCEMV1260DriftResourceLimitPortable` (`interop/cem01-go`); `checkDrift`, `driftItem` in `internal/cem/verify/verify.go` | f0fda542: native 68 to 102 rows, byte-identical to portable; the public case: native and portable 13 rows, byte-identical; base re-read mutation 8 rows; build log `2026-10-15-cem-stable-consumer-residuals.md` |

## Protocol pointer gate (V1-1296, decision 0532, 2026-10-15)

Status: `CEM-V1-261` accepted under the owner's standing auto-accept (2026-10-11); V1-1296.

Intent: a digest-pinned file pointer in the `protocol/cem-1.0` packet either resolves or is a
recorded, explained exception. V1-1296 found `stable/MIGRATION.json` naming a missing
`../migration-plan.json`; the scan also found the same member in the packet's inherited S0E copy and
`repository-envelope-packet/INPUTS.json` naming a missing `../REVIEW-of-repair5.json`. All three sit
in sealed packet bytes, pinned by the stable manifest and the packet's input list, and are marked
informative there. Removing them rewrites sealed bytes, so decision 0532 records them and leaves
their removal to the owner. Decision 0535 (owner delegated to the orchestrator, 2026-10-16)
declines the removal: sealed evidence stays immutable, and the three pointers stay recorded
exceptions that `--check` reports.

- `CEM-V1-261`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1296)
  `tools/cem-stable-regen --check` MUST refuse a dangling protocol pointer. A pointer is a string
  member `K` of a JSON object under `protocol/cem-1.0` whose object also has a `K` + `SHA256` or
  `K` + `Sha256` string, or a `path` member starting `../` beside a `sha256` string; a document
  whose top-level `spec` starts `cem/` is wire data and is not scanned, and a file that is not JSON
  carries no pointer. Each pointer MUST resolve, relative to its file's directory and inside the
  repository with no symbolic link on the way, to a regular file whose SHA-256 is the pinned one. The only exceptions are the recorded
  dangling pointers, each named by file and JSON pointer with its exact text; a recorded pointer
  that resolves, changes text or disappears from a file that still exists is itself refused, so the
  record cannot outlive its reason.

Non-goals: no change to any sealed packet byte, manifest or digest; no pointer kinds outside the
two shapes above; no scan outside `protocol/cem-1.0`.

Failure modes: a pointer written in another shape is not checked; the recorded exceptions stay
dangling by decision (decision 0535 declined their removal).

Acceptance evidence: `--check` passes on the repository with the three recorded pointers and
reports exactly three problems without the record. Removing the gate call from `check` makes
`TestCEMV1261CheckRunsPointerGate` fail, and reading the target without the symbolic-link check
makes `TestCEMV1261PointerSymlinkRefused` fail. Build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-cem-stable-consumer-residuals.md`.

Rollback: revert the change commit; `--check` then ignores protocol pointers.

### Protocol pointer gate traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-261 | `TestCEMV1261PointerProblems`, `TestCEMV1261RepositoryPointersRecorded`, `TestCEMV1261PointerSymlinkRefused`, `TestCEMV1261CheckRunsPointerGate` (`tools/cem-stable-regen`); `pointerProblems` in `tools/cem-stable-regen/pointers.go` | Three recorded sealed pointers; build log `2026-10-15-cem-stable-consumer-residuals.md` |

## Verdict ledger (V1-1292, decision 0523 K6)

Status: `CEM-V1-271..279` accepted under the owner's standing auto-accept (2026-10-11); V1-1292.

Intent: an operator who anchors a stable map can also keep, beside the pointer, the verification
verdict the native consumer reached for that map on that commit, so a later disagreement (another
artifact set, another verifier release) is detected instead of silently replacing the earlier
result. Before this change the `CEM-V1-065` row and the `cem/v2` non-goals said "no verdict
ledger"; the anchor pointer (`CEM-V1-067`) records which map describes a commit, never what a
verifier concluded. The precedent the ledger stands on is decision 0523 K6: native and portable
consumers agree on `verdictSha256` (`CEM-V1-066`) for every public stable case before any record is
written. That precondition already holds through the `CEM-V1-066` tests; this change does not
re-qualify it.

- `CEM-V1-271`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) The
  verdict ledger MUST be kept as Git notes under `refs/notes/corvint-verdicts`, a sibling of the
  anchor ref `refs/notes/corvint` in the Corvint notes namespace (a ref below
  `refs/notes/corvint/` cannot coexist with `refs/notes/corvint`, and the anchor note stays one
  pointer object). Each target commit has at most one ledger note: a sequence of LF-terminated
  lines, each exactly one canonical `corvint-cem-verdict/1` record, at most 256 records. The
  recorded digest is the `CEM-V1-066` `verdictSha256`. The `refs/notes/corvint` pointer note and
  its `CEM-V1-067` reads are unchanged.
- `CEM-V1-272`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) A record
  MUST be the closed object with exactly these members in this order and no others: `schema`
  (`corvint-cem-verdict/1`), `mapSha256` (the anchored map's SHA-256), `target` (`gitCommit`, the
  annotated commit, and `gitTree`, its tree, both full lowercase object IDs), `outcome` (`ACCEPT`
  or `REJECT`), `verdictSha256` (64 lowercase hexadecimal), `consumer` (`native`), `runtime`
  (`goVersion`, the verifier's Go release, printable ASCII of 1..256 bytes), `recordedAt` (RFC
  3339 UTC with second precision) and `previousRecordSha256` (null for the first line of the note,
  otherwise the SHA-256 of the previous line's exact bytes). Its line MUST be the compact JSON
  encoding of those members (Go `encoding/json`, no insignificant whitespace); a line whose bytes
  differ from the re-encoding of its decoded record does not verify.
- `CEM-V1-273`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) `cem
  anchor --map MAP [--commit REV] --artifacts DIR` MUST, after every `CEM-V1-067` check, run the
  native stable verifier over the exact committed map bytes with the map's `baseRevision` as the
  expected base, REV as the target and DIR as the artifact root, as `cem verify` does, and append
  one record per (`mapSha256`, target commit). Its result gains a `verdict` member (`ref`,
  `written`, `record`, `state`). Without `--artifacts`, anchor runs no verifier, writes no ledger
  note and its result has no `verdict` member.
- `CEM-V1-274`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) Only an
  `ACCEPT` or `REJECT` result MUST be recorded. Recording again a `verdictSha256` equal to the
  one already recorded for the same key MUST write nothing (`written` false, the existing record
  returned). A record for a new key MUST be appended after the existing lines, whose bytes are
  kept, with `previousRecordSha256` naming the last of them.
- `CEM-V1-275`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) Anchor
  with `--artifacts` MUST refuse, writing neither the pointer note nor the ledger note, with:
  `verdict-ledger-conflict` when the target's ledger already records a different
  `verdictSha256` for the map (a recorded verdict is never replaced);
  `verdict-ledger-malformed` when the target's existing ledger note does not read `verified`;
  `verdict-ledger-unsupported-result` when the verifier reaches no verdict (`UNSUPPORTED`, for
  example a relative or missing artifact root); and `verdict-ledger-full` when the note already
  holds 256 records. Every refusal of the pointer or of the verdict MUST be decided before any
  note is written. Anchor MUST build each new notes commit on the ref tip it validated and publish
  the pointer ref and the ledger ref in one atomic `git update-ref --stdin` transaction whose old
  values are those tips (the zero object ID for an absent ref), so a lock, a failure or a lost race
  leaves both refs unchanged. A lost race MUST be refused after re-reading the tips, with
  `verdict-ledger-conflict` when the ledger ref moved and `anchor-note-conflict` when only the
  pointer ref moved; nothing is overwritten.
- `CEM-V1-276`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) A ledger
  read MUST report exactly one state: `absent` (no note), `verified`, `malformed` (empty note, no
  final LF, more than 256 lines, a line that is not one canonical well-formed record),
  `chain-broken` (a `previousRecordSha256` link that is not null on the first line or not the
  previous line's digest), `scope-mismatch` (a record naming another commit or tree than the
  annotated commit) or `conflict` (a repeated `mapSha256`), in that precedence.
- `CEM-V1-277`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) `cem
  provenance --commit REV --verdicts` MUST add a `verdicts` member (`ref`, `trust`
  `repository-history`, `authority` `git-history`, `state`, `records`) listing REV's records
  only when the state is `verified`, and an empty list otherwise. Without `--verdicts` the result
  has no `verdicts` member. The listing MUST write no ref, note, object, index, config or
  worktree file.
- `CEM-V1-278`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) The
  ledger MUST carry no authority: a record changes no anchor or provenance row state, no `cem
  verify`, `cem status` or export result, tier, ranking or learning input. Only an explicit `cem
  anchor --artifacts` writes it; no read command writes it (product invariant 4).
- `CEM-V1-279`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) The CLI
  surface MUST be the two flags, with no new `cem` action: `cem anchor --map MAP [--commit REV]
  [--artifacts DIR]` and `cem provenance --commit REV [--notes-tip OID] [--verdicts]`, both shown
  in `corvint cem --help`.

Wire and schema: new note ref and record profile `corvint-cem-verdict/1`; the anchor result gains
the optional `verdict` member and the provenance result the optional `verdicts` member. The stable
map, `cem-stable-verification/1`, the anchor pointer profile and every provenance row are
unchanged, so the change is additive.

Non-goals: no portable writer of the ledger (the portable consumer agrees on `verdictSha256` by
`CEM-V1-066`; only the native consumer writes; the portable reader is `CEM-V1-754`, V1-2023); no retry of a lost race; no
`--notes-tip` rollback check for the ledger ref; no signing; no default-on recording; no automatic removal or migration of a conflicting record.

Failure modes: a differing verdict for an anchored map blocks recording with
`verdict-ledger-conflict`; the operator finds the cause (artifacts, verifier release) and removes
the note deliberately (`git notes --ref=refs/notes/corvint-verdicts remove REV`) before recording
again. An edited, reordered or truncated ledger note reads `malformed`, `chain-broken`,
`scope-mismatch` or `conflict`, lists no records, and is never extended. A deleted or reset ledger
ref reads `absent`; the chain detects edits inside one note, not removal of a whole note. Two
concurrent anchors of the same commit cannot both publish: the second compare-and-swap fails, and
that anchor is refused `verdict-ledger-conflict` with nothing written; rerunning it reads the
winner's ledger. A crash between building a notes commit and publishing it leaves at most a
transient `refs/notes/corvint-staging/` ref and unreachable objects, never a moved public ref.

Acceptance evidence: the tests below, each shown failing under a mutation of its guard; build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-1292-cem-verdict-ledger.md`.

Rollback: revert the V1-1292 commit. `--artifacts` and `--verdicts` become unrecognized
arguments; existing `refs/notes/corvint-verdicts` notes stay in the repository, unread, and can be
removed with `git update-ref -d refs/notes/corvint-verdicts`. No committed map, seal or golden
depends on the ledger.

### Verdict ledger traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-271 | `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently` (`internal/gitnotes`); `VerdictRef`, `readLedger` in `internal/gitnotes/verdict.go` | The one-line note is written under `refs/notes/corvint-verdicts` and the anchor receipt stays `ok` (its pointer reads back `verified`) |
| CEM-V1-272 | `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` (`internal/gitnotes`); `VerdictRecord`, `wellFormedRecord` in `internal/gitnotes/verdict.go` | An unknown member, non-canonical spacing, a missing final LF and an unknown outcome each read `malformed` |
| CEM-V1-273 | `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently` (`internal/gitnotes`); `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `AnchorWithVerdict` in `internal/gitnotes/anchor.go` | The record's `verdictSha256` equals the digest of an independent `verify.Stable` run over the same inputs |
| CEM-V1-274 | `TestCEMV1274AppendLinksToThePreviousRecord`, `TestCEMV1274UnsupportedResultWritesNothing`, `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently` (`internal/gitnotes`); `planAppend` | A repeat leaves every ref, note and object unchanged and returns the first `recordedAt`; a second key keeps the first line's bytes and links to its digest; a relative artifact root writes no note |
| CEM-V1-275 | `TestCEMV1275DifferingVerdictIsRefused`, `TestCEMV1275FullLedgerIsRefused`, `TestCEMV1275LostRaceIsRefusedWithoutOverwrite`, `TestCEMV1275LockedLedgerRefLeavesThePointerUnwritten`, `TestCEMV1274UnsupportedResultWritesNothing`, `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` (`internal/gitnotes`); `planAppend` | `verdict-ledger-conflict` creates no `refs/notes/corvint` pointer; each refusal leaves every ref, note and object unchanged; a 256-record ledger plans no body; a writer interleaved between validation and publish wins and the other is refused with the winner's ledger intact; a locked ledger ref leaves the pointer ref unwritten |
| CEM-V1-276 | `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` (`internal/gitnotes`); `readLedger` | Eight tamper cases read `malformed`, `chain-broken`, `scope-mismatch` or `conflict` |
| CEM-V1-277 | `TestCEMV1277VerdictListingIsReadOnly` (`internal/gitnotes`); `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `ProvenanceWithVerdicts` in `internal/gitnotes/provenance.go` | Refs, notes and objects are unchanged across the listing; no `verdicts` member without the flag |
| CEM-V1-278 | `TestCEMV1277VerdictListingIsReadOnly` (`internal/gitnotes`); `readLedger` is called only by `AnchorWithVerdict`, `appendLedger` and `verdictsMember` in `internal/gitnotes` | No verifier, status, export, tier or ranking path reads `refs/notes/corvint-verdicts`; the listing reports `mutates` false and changes nothing |
| CEM-V1-279 | `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `TestAnchorActionsParseLikeTheirSiblings` (`internal/cem/cli`) | The help lists both flags and the action list is unchanged |
## Capsule and dashboard stable positive cases (V1-0632, 2026-10-20)

Status: `CEM-V1-290..299` accepted under the owner's standing auto-accept (2026-10-11); V1-0632
audit gap 1.

Intent: the dashboard (C17) and the local-authority capsule (C15) each read an actual `cem/1.0`
change map, not only refuse one. Each keeps the exact raw stable graph, leaves artifacts opaque and
retains the verifier's whole result and its unknown authority axes. A stale or incomplete binding
refuses without losing evidence. This amends `CEM-V1-167` and the failure mode that left both
families without a stable path. The frozen `corvint-dashboard-snapshot/0` schema, its hash-pinned
registry and the retired bundle flags (`CEM-V1-168`, `CEM-V1-217`) are unchanged.

- `CEM-V1-290`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632)
  `corvint-dashboard-snapshot cem-stable --root R --cem C --ocm O --expected-base B --target T
  --artifact-root A` MUST admit the map through `lrfrepo.VerifyStableUniverse` and, on success,
  write one `corvint-dashboard-cem-stable/0` document to stdout with exit 0. It carries the input
  paths and SHA-256 digests, spec, map digest, base and target revisions, object format, patch and
  OCM digests, intent scope, obligations, the verifier's whole `verify.StableResult` as
  `verification`, the exact original map bytes as standard base64 in `mapBase64`, whose SHA-256
  MUST equal `mapSha256` (final LF and formatting included), and `taskCompletion` and
  `criterionSatisfaction` as `NOT_OBSERVED`. A hunk's coverage witness, present or absent, MUST
  survive in `mapBase64` unchanged.
- `CEM-V1-291`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  `cem-stable` subcommand MUST NOT change the snapshot command: its schema, adapter registry and
  hash, error profile, and the `DASHBOARD_INVALID_ARGUMENT` refusal of every bundle flag and profile
  stay as `CEM-V1-168` and `CEM-V1-217` state.
- `CEM-V1-292`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Each of
  the six flags MUST appear exactly once; `--root` and `--artifact-root` MUST be absolute and
  clean, and `--cem` and `--ocm` relative and clean. Any other form MUST be
  `DASHBOARD_INVALID_ARGUMENT` with no stdout before any path opens. A map or OCM that is missing,
  not a regular file or larger than the stable map bound MUST be `DASHBOARD_REPOSITORY_UNAVAILABLE`
  with no stdout. The read MUST NOT follow a symlink in any path component under `--root`: each
  directory opens through its parent's descriptor and must be the directory `Lstat` saw, and the
  file opens non-blocking and its descriptor must be the regular file `Lstat` saw, so a symlinked
  parent or final component, a FIFO, or a component swapped after `Lstat` refuses without
  blocking.
- `CEM-V1-293`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) A refused
  admission MUST write no stdout, exit 2, and write one `corvint-dashboard-cem-stable-refusal/0`
  document to stderr with the refusal code, the input paths and digests, and `verification` set
  to the verifier's whole result when the verifier ran, else `null`. A stale base, a stale target,
  an edited map, a tampered or missing artifact, an OCM bound to another map and a retired `cem/0.x`
  map each refuse.
- `CEM-V1-294`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  subcommand MUST NOT write any repository, map, OCM or artifact byte, and MUST NOT derive task
  completion, criterion satisfaction or any authority axis that the verifier leaves unknown.
- `CEM-V1-295`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Amending
  `CEM-V1-167`, the local-authority capsule MUST parse the enrolled `cem.json` with the stable
  decoder. A retired `cem/0.x` map keeps `unsupported-spec` with the fix line (`CEM-V1-210`). A
  `cem/1.0` map whose base is not the enrolled base MUST refuse `capsule CEM base`. A matching map
  proceeds to the object demand.
- `CEM-V1-296`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  capsule's stable closure MUST account one artifact for each of `tasks-capture`,
  `tasks-verification`, `tasks-claimed-ticket`, `tasks-snapshot-head-receipt`, `runner-plan` and
  `runner-receipt`, in that order, and the SHA-256 of the exact enrolled map bytes. A missing role
  MUST refuse `capsule stable role missing`. The capsule MUST NOT read artifact bytes.
- `CEM-V1-297`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The object
  demand MUST require the base and target trees. It requires each evidence path at the base with
  the map's blob OID (`capsule CEM blob binding`), the OCM intent path at base and target, each OCM
  claim path at the target, each check's driver at base and target and its subject at the target.
  It requires the committed `.corvint/change.cem.json` at the target, whose blob MUST equal the
  enrolled `cem.json` bytes (`capsule CEM sidecar binding`). An absent path refuses
  `required capsule path missing`, and an absent blob `changed blob missing`.
- `CEM-V1-298`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  capsule closure confers no authority. Acceptance comes only from the stable verifier run over the
  published minimal evidence view, with the artifact root supplied separately. Over that view, the
  verifier MUST return the same map digest, all six artifact checks, an `EXACT` sidecar, every
  result key, and the unknown axes as `NOT_OBSERVED`. A tampered artifact the closure still admits
  MUST be rejected `artifact-digest-mismatch` by that verifier.
- `CEM-V1-299`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Every
  capsule refusal in `CEM-V1-295..297` MUST happen before evidence publication and leave the
  enrolled bytes unchanged.

Non-goals:
- no new snapshot schema version or registry row, and no bundle route;
- no capsule-native stable verifier;
- no enrollment format that carries artifact bytes;
- no native Tasks authority, authentication or runner execution claim;
- no C15 stale-live-gate control: the capsule holds no live Tasks gate to go stale.

Failure modes:
- When the committed sidecar is not exact, `VerifyStableUniverse` returns a plain
  `excluded-artifact-mismatch` without a result. The dashboard refusal then keeps the input digests
  and code, with `verification: null`. The same holds for an OCM failure and a retired map.
- The protected enrollment command still accepts only a codec-package target tree
  (`onlyCodecPackage`), and it needs the root-owned authority store, so a full protected enrollment
  of a stable change was not run.

Acceptance evidence: the tests below pass, and each fails without its fix. With the dispatch
removed, the dashboard tests see `DASHBOARD_INVALID_ARGUMENT`. With the base capsule closure and a
stub, the capsule tests see `capsule CEM base`. With the first reader, a symlinked parent is
admitted (exit 0), a swapped-in FIFO blocks the open, and the emitted map is compacted without its
final LF. Without the check demand the driver control admits; without the subject demand the
subject control admits; without the OID comparison the mismatch reaches `capsule CEM sidecar
binding`. Build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-0632-capsule-dashboard-positive.md`.

Rollback: revert the change commit. The dashboard then has no `cem-stable` subcommand, and the
capsule refuses every `cem/1.0` enrollment with `capsule CEM base` again.

### Capsule and dashboard stable positive traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-290 | `TestCEMV1290DashboardCEMStableReadsRealUniverse` (`cmd/corvint-dashboard-snapshot`) | Strict key set; decoded `mapBase64` equals the raw on-disk stable map, final LF included, and its SHA-256 is `mapSha256`, with and without a witness; `verification` equals an independent `verify.Stable` marshal with 23 keys, 6 artifact checks, `EXACT` sidecar |
| CEM-V1-291 | `TestCEMV1168DashboardRefusesStableBundle`, `TestCEMV1290DashboardCEMStableReadsRealUniverse` | Bundle flags still refuse; snapshot package suite and `conformance/dashboard-snapshot-v0` pass unchanged |
| CEM-V1-292 | `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings`, `TestCEMV1292DashboardCEMStableInputStaysUnderRoot`, `TestCEMV1292DashboardCEMStableRefusesSwappedFIFO` | Four bad-argument forms are `DASHBOARD_INVALID_ARGUMENT` with no stdout; a symlinked parent (map or OCM), a symlinked final component and a FIFO are `DASHBOARD_REPOSITORY_UNAVAILABLE` within 10 s; a FIFO swapped in after `Lstat` refuses without blocking |
| CEM-V1-293 | `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` | Seven controls refuse with code, input digests, and the verifier result when it ran |
| CEM-V1-294 | `TestCEMV1290DashboardCEMStableReadsRealUniverse`, `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` | `NOT_OBSERVED` completion and axes; inputs byte-identical after every refusal |
| CEM-V1-295 | `TestCEMV1167CapsuleRefusesStableEnrollment`, `TestCEMV1297CapsuleRefusesStableClosureControls` (`tools/local-authority`) | 0.x `unsupported-spec` with fix line; stale base `capsule CEM base` |
| CEM-V1-296 | `TestCEMV1295CapsuleAdmitsStableClosure`, `TestCEMV1297CapsuleRefusesStableClosureControls` | Six roles in order, map digest of exact bytes; producer map `capsule stable role missing` |
| CEM-V1-297 | `TestCEMV1295CapsuleAdmitsStableClosure`, `TestCEMV1297CapsuleRefusesStableClosureControls` | Minimal capsule admits with and without two checks; other sidecar, missing sidecar and missing evidence blob refuse; an evidence OID naming another carried blob is `capsule CEM blob binding`; a missing driver blob and a missing subject blob of an unchanged, uncited path are each `changed blob missing`; an absent driver path is `required capsule path missing` |
| CEM-V1-298 | `TestCEMV1295CapsuleAdmitsStableClosure` | Published view `ACCEPT` with the same keys and axes as the direct run; tampered artifact `REJECT` `artifact-digest-mismatch` |
| CEM-V1-299 | `TestCEMV1297CapsuleRefusesStableClosureControls` | Every refusal from the closure step, which precedes publication; enrolled map unchanged |

## Metadata-only change typed refusal (V1-0609, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0609.
Whether `cem/1.0` should represent metadata-only changes is a wire change. That decision is still
the owner's to make. Until then, these requirements only make the refusal explicit.

Git emits a headerless group for three kinds of change: an empty-file creation, an empty-file
deletion, and a mode-only change. Each group is a `diff --git` line followed by exactly two
metadata lines, with no `---`/`+++` header and no hunk. The two lines are:

- `new file mode M` and `index O..N` for a creation;
- `deleted file mode M` and `index O..N` for a deletion;
- `old mode A` and `new mode B` for a mode-only change.

The group ends at EOF or at the next `diff --git` line. Before this change both consumers refused
these groups with misleading grammar codes. Native gave `malformed-patch` ("expected --- file
header"). Portable gave `metadata`, `file-header` or `unsupported-tree-mode`.

- `CEM-V1-300`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0609) The native
  parser (`internal/cem/patch`) and the portable consumer (`interop/cem01-go`) MUST refuse such a
  group with the same code, `metadata-only-change-unsupported`.
  - The refusal names the change kind (`empty-file-creation`, `empty-file-deletion` or `mode-only`)
    and the repository path.
  - It applies whether the group stands alone or sits before or after a text group in the same
    patch.
  - It is an operational refusal of a documented limit, not a protocol-invalid patch.
  - It never invents a text hunk, and it publishes no map.
  - Recognition requires these exact conditions: both modes are `100644` or `100755`, a mode-only
    change has two different modes, the `index` mode is absent or equal to the file mode, and the
    unquoted `diff --git a/PATH b/PATH` names one valid path on both sides.
- `CEM-V1-301`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0609) Any group
  outside the `CEM-V1-300` shape MUST keep its pre-V1-0609 error unchanged (for the native parser,
  `CEM-V1-930` amends the mode-plus-text case). Examples:
  - a mode change together with a text change;
  - a creation without an `index` line;
  - an unchanged or non-regular mode;
  - an asymmetric `diff --git` path.
- `CEM-V1-302`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0609) The frozen
  stable profile MUST stay byte-identical. Its closed `code` and `issueCodes` enums
  (`protocol/cem-1.0/stable/RESULT.schema.json`) have no metadata-only code. Therefore:
  - The native stable verifier reports `malformed-patch` for these groups, as before.
  - The portable stable verifier reports its legacy code: `metadata` when another group follows,
    `file-header` at EOF, and `unsupported-tree-mode` for a mode-only change.
  - No file under `protocol/cem-1.0/stable/` changes.

Non-goals:
- no wire representation, metadata identity or binding for empty files or modes in `cem/1.0`; that
  is deferred to an owner decision;
- no change to `ALGORITHMS.md`, the inherited algorithms, the stable schema or the manifest;
- no reconciliation of the existing native/portable stable issue-code divergence on these inputs.

Failure modes:
- A canonical change that contains an empty file or a chmod still cannot be bound. It now refuses
  with a code that names the path and kind instead of a grammar error. The work-around is the one
  the fixture precedent already uses (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-cem03-untracked-empty-vector.md`):
  keep zero-byte artifacts out of the bound change.
- The portable `verify-candidate` result carries only the code. The kind and path are kept in the
  error value, and native messages print them.

Acceptance evidence: the tests below pass, and each fails without the parser change, where the
vectors give `malformed-patch` natively and `metadata`/`file-header`/`unsupported-tree-mode`
portably. The vectors in `interop/cem01-go/testdata/metadata-only/` are canonical-diff output from
Git 2.54, plus hand-built near misses. Build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-0609-metadata-only-typed-refusal.md`.

Rollback: revert the change commit. Both consumers then return the earlier grammar codes, and the
stable results are unchanged either way.

### Metadata-only change typed refusal traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-300 | `TestMetadataOnlyChangeRefusalVectors` (`internal/cem/patch`, `interop/cem01-go`), `TestPrepareRefusesMetadataOnlyChanges` (`internal/cem/workflow`) | Five shared vectors (creation, deletion, mode-only, and both mixed orders) give the identical code, kind and path natively and portably; `cem prepare` on real commits refuses each kind and the mixed case |
| CEM-V1-301 | `TestMetadataOnlyChangeRefusalVectors` (`internal/cem/patch`, `interop/cem01-go`) | Mode-and-text, asymmetric-path, no-index, symlink-mode and unchanged-mode vectors keep `malformed-patch` natively and their legacy portable code |
| CEM-V1-302 | `TestStablePatchRefusalKeepsFrozenVocabulary` (`internal/cem/verify`), `TestMetadataOnlyChangeRefusalVectors` (`interop/cem01-go`) | Native stable maps the refusal to `malformed-patch` with the same message; the portable stable code equals the manifest's `portableStableIssue`; `make cem-stable-regen-check` passes with no stable byte change |

## Out-of-tree map reads for `cem status` and `cem verify` (V1-2097, #1329, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2097.
`cem export` and bundle assembly write a map outside the repository, but `cem status` and
`cem verify` refused an absolute `--map` with `invalid-arguments` ("map path must be
repository-relative"), and the verb help did not say so. The adopter had to copy the map into the
working tree. A `cem/1.0` map binds the repository by object IDs, so its location carries no
authority; `cem verify-stable` already reads a map at any path.

- `CEM-V1-315`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2097) `cem status`
  and `cem verify` MUST accept an absolute `--map` whose resolved parent directory lies outside the
  worktree, its Git directory and the common Git directory.
  - The map is read once, bounded by the map size limit, and nothing is written in the repository,
    its Git directories or beside the map (product invariant 4).
  - The stable verification result and exit are exactly what `cem verify-stable` reports for the
    same map bytes, repository, expected base, target and `--artifacts`.
  - `cem status` reports the resolved absolute path it read as `map`.
- `CEM-V1-316`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2097) An absolute
  `--map` MUST NOT reach a map inside the repository by any route.
  - An absolute path whose parent directory is, or is inside, the worktree or a Git directory is
    refused `invalid-arguments` with a message to name it repository-relative. The comparison uses
    file identity after resolving the parent's symlinks, so a symlinked parent directory and case or
    normalization aliases are refused too.
  - A final component that is a symlink, is not a regular file, is missing or exceeds the bound is
    refused `map-unavailable`; a symlink is never followed.
  - `cem report`, every producer and mutator, `cem anchor`, `cem export`, `prove --cem` and the MCP
    report preview keep repository-relative maps only, with their existing refusals. Amended by V1-2798 (GitHub #1895):
    `cem report` reads an absolute out-of-tree map under these rules (`CEM-V1-640`), and so do
    `prove --cem` and its attestation verifier (`CEM-V1-641`). Amended by GitHub #1942: the
    mutators read one into a repository-relative `--output` only (`CEM-V1-647`).
- `CEM-V1-317`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2097) The `cem` help,
  which every `cem` verb's `--help` prints, MUST document the map-path rules: the repository-relative
  form, the absolute out-of-tree form for status and verify (and report, `CEM-V1-640`, and
  `prove --cem`, `CEM-V1-641`) and its refusals, `verify-stable`'s
  unrestricted `--map`, and that `cem anchor` and its `--artifacts` verdict ledger read only the map
  committed at HEAD.

Non-goals:
- no out-of-tree mode for `cem anchor` or the `refs/notes/corvint-verdicts` ledger. The
  `corvint-cem-anchor/1` pointer names the map's commit, path and blob, and `cem provenance`
  verifies them; a pointer to a map held in a separate evidence repository is a new pointer profile
  and read-back contract. It is deferred to a follow-up under #1329;
- no change to the stable verifier, its result profile or the map wire;
- no absolute map for `cem report`; its output aliasing checks assume a repository-relative map
  (superseded by `CEM-V1-640`, V1-2798).

Failure modes:
- An adopter who keeps maps in a separate evidence repository can verify them in place but must
  still commit a map in the product repository to anchor it or record a verdict.
- A hard link inside the repository named through an outside directory is read like any other
  outside file. It is read only, and the verifier still judges its bytes.
- When the Git directories cannot be located, only the worktree is protected and the stable
  verifier refuses the repository itself.

Acceptance evidence: the tests below pass. `TestCEMV1315StatusVerifyReadOutOfTreeMap` fails without
the change, because the absolute map is refused `invalid-arguments`. A live run of the lane binary
over the committed map of `830ff9027` copied outside the repository returned `ACCEPT`, exit 0, for
both `cem status` and `cem verify`, identical to `cem verify-stable`, with the worktree unchanged.

Rollback: revert the change commit. `cem status` and `cem verify` then refuse an absolute map again,
and no stored state changes.

### Out-of-tree map read traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-315 | `TestCEMV1315StatusVerifyReadOutOfTreeMap` (`internal/cem/cli`) | status and verify over an outside copy of the sealed map equal `verify-stable` over the same path; status names the path; the repository and map directory listings are unchanged |
| CEM-V1-316 | `TestCEMV1316OutOfTreeMapRefusals` (`internal/cem/cli`) | absolute maps in the worktree, in `.git` and through a symlinked parent refuse `invalid-arguments`; a symlinked or missing map refuses `map-unavailable`; `cem mark` keeps its repository-relative refusal (report reads one since `CEM-V1-640`) |
| CEM-V1-317 | `TestCEMV1317HelpDocumentsMapPaths` (`cmd/corvint`) | `cem status --help`, `cem verify --help`, `cem report --help` and `cem anchor --help` print the map-path rules and the anchor limit |

## Refusal and report reasons for agents (V1-2098, V1-2099, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2098
(beamfall/corvint#1330) and V1-2099 (beamfall/corvint#1331). Every refusal code below is unchanged.
Only the hunk-revert report gains a member, and only some refusal messages gain recovery text.

- `CEM-V1-320`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2098) The
  `cem-hunk-revert/0` report of `cem verify --hunk-revert` MUST carry a trailing member
  `notAttempted`. It is an array with one `{hunkId, path, reason}` row, in hunk ID order as `hunks` is, for every map
  hunk that the falsifier did not attempt. `reason` is one closed value, chosen by the first test
  that matches in this order:
  - `test-only-hunk`: the hunk changes a `_test.go` file;
  - `unsupported-language`: the hunk changes a file that is not Go source, so no runner re-executes it;
  - `not-supported-disposition`: the hunk's disposition is not `supported`;
  - `not-in-patch`: the hunk is not in the derived canonical patch;
  - `no-test-claim`: no `test-claim` basis cites a `_test.go` span.
  The member is present, possibly empty, whenever the report is. A map that has no candidate (for
  example a Ruby-only change) therefore names each hunk with its reason, instead of reporting only
  an empty `hunks` array.
- `CEM-V1-321`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2098) The change
  is additive. `hunks`, `held`, `survived`, `notRun`, the bounds, `elapsedSeconds`, their member
  order and the `CEM-V1-088`/`CEM-V1-089` row semantics stay unchanged. A hunk is in either `hunks` or
  `notAttempted`, never both. Every map hunk is in one of the two. The stable result, its
  `verdictSha256` and the envelope without the flag are byte-identical.
- `CEM-V1-322`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2099) The
  `cite-span-not-stable` refusal of `cem cite --bytes` or `--lines` MUST keep its code and its
  existing message as a prefix. It MUST add the rule: a cited base span must survive the change,
  so its exact text must occur exactly once in the changed file, and the citation should use base
  lines that no hunk removes. When it can do so cheaply, it MUST also name the nearest stable spans:
  - It looks for at most one maximal unedited base-line run above the span and one below it, each
    clipped to the citation byte bound.
  - It names only the runs that the stable classifier admits, as `--lines A:B` arguments.
  - When no such run is stable, it says so.
  - For a deleted or renamed file, it says that no base line of the file is citable.
- `CEM-V1-323`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2099) The
  `unsupported-object-alternates` refusal MUST keep its code and MUST name a working fix.
  - For a repository `objects/info/alternates` (a `git clone --shared` or `--reference` clone),
    the fix is: run `git repack -a -d`, then remove the alternates files under the Git common
    directory, and rewrite or remove commit-graph chains. The alternative is to clone again
    without `--shared` or `--reference`.
  - For an ambient `GIT_ALTERNATE_OBJECT_DIRECTORIES`, the fix is to unset it and rerun.

Non-goals: no new refusal code for these cases; no change to which hunks are falsifier candidates;
no automatic repack, re-clone or citation; no change to the stable wire or the portable consumer.

Failure modes:
- A consumer that rejects unknown members of `cem-hunk-revert/0` sees the new `notAttempted`
  member. The profile is opt-in and experimental, and no golden, seal or public case pins it.
- The nearest-span guidance is advisory. It covers only the two runs next to the span, so a
  stable span further away is not named. It never claims a span is stable without classifying it.
- The repack fix makes the clone self-contained only after the alternates file is removed.
  Running the repack step alone still refuses.

Acceptance evidence: the tests below pass, and each fails without its code change.

Rollback: revert the change commit. The member and the added message text go away, and every code
stays the same.

### Refusal and report reasons traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-320 | `TestCEMV1320NotAttemptedHunksCarryAClosedReason` (`internal/cem/verify`) | Six hunks give one candidate and five rows, one per reason, in hunk ID order. A Ruby-only map runs no `go test` and names its hunk `unsupported-language` |
| CEM-V1-321 | `TestCEMV1320NotAttemptedHunksCarryAClosedReason`, `TestCEMV1088CountMovesOnlyAcceptedTiers`, `TestCEMV1089ClassifyRevert` (`internal/cem/verify`); `TestCEMV1088HeldOnlyFromConsumerReexecution` (`internal/cem/workflow`) | Candidate rows and counts are unchanged. A Ruby-only report has empty `hunks` and `notRun` 0 |
| CEM-V1-322 | `TestCEMV1322RemovedSpanRefusalNamesRuleAndNearestStableSpans` (`internal/cem/workflow`) | A span over removed lines refuses `cite-span-not-stable` with the rule and `--lines 1:2, --lines 5:6`. Both named spans then cite successfully |
| CEM-V1-323 | `TestCEMV1323SharedCloneRefusalNamesAWorkingFix` (`internal/cem/gitauth`) | A real `git clone --shared` refuses with the fix. After `git repack -a -d` and removing the alternates file, the open succeeds. An ambient alternates variable names its own fix |

## Per-action help, assembly `ok` and the producer-map refusal (V1-2102, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2102
(beamfall/corvint#1335).

- `CEM-V1-340`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2102)
  `corvint cem ACTION --help` (and `-h`) for a registered action MUST print help for that action
  only. The help is projected from `corvint cem --help` so that the two cannot drift:
  - the action's usage lines;
  - its Actions paragraph (`status`, `verify` and `report` share one);
  - the Verification options, for `status`, `verify` and `report` only;
  - the closing wire note;
  - a pointer to the full `corvint cem --help`.
  The registered actions now include `cover` and `discriminate`. This amends `GPK-V0-062` for
  `cem` actions only: their help is no longer byte-identical to `corvint cem --help`, which is
  unchanged. `ocm`, `harness` and `corvint help cem` are unchanged.
- `CEM-V1-341`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2102)
  `corvint-cem-candidate assemble-stable` MUST print its assembly result (`StableAssemblyResult` or
  `PlainAssemblyResult`) with one leading top-level member `"ok": true`. Every other byte is the
  result's unchanged encoding, including its member order and trailing newline. Refusals keep
  exit 1, the stderr line and empty stdout.
- `CEM-V1-342`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2102) Core
  mutators MUST refuse a `cem/1.0` map that is not a producer map with `not-a-producer-map`, not
  `unsupported-spec`, because `cem/1.0` is supported. The mutators are the `cite`, `mark`, `cover`
  and `discriminate` inputs, the destination guards of every writer, and the backup guards of
  `prepare`. An assembled stable map is the usual case. The message is unchanged: it names the
  command and the stable read. This amends the refusal code of `CEM-V1-030`, `CEM-V1-031`
  and `CEM-V1-143`. Retired `cem/0.x` maps and unknown specs still refuse `unsupported-spec`.
- `CEM-V1-343`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2102) The code
  `not-a-producer-map` MUST be registered in `internal/cem/cemcode` and named here, so that the
  error-code ownership check owns it. It MUST also be admitted as a dogfood `NOT_PRODUCED` reason
  (`internal/observations`).

Non-goals: no per-action help for `verify-stable` or `gate-verdict`, which parse their own argv
before the help router; no `corvint help cem ACTION` topic; no change to the assembly result
structs, the request decoders or receipt labelling; no change to the map write guards themselves.

Failure modes:
- A script that compared `cem ACTION --help` to `cem --help` byte for byte now differs. The full
  text is still `corvint cem --help`.
- A consumer that matched `unsupported-spec` for an assembled map now sees `not-a-producer-map`.
- A strict decoder of the assembly result that refuses unknown members must admit `ok`.

Acceptance evidence: the tests below pass. The help and `ok` tests fail without their code change.

Rollback: revert the change commit. Action help returns to the full cem help, the `ok` member goes
away and the mutators refuse `unsupported-spec` again.

### Per-action help, assembly `ok` and producer-map refusal traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-340 | `TestCEMV1340CEMActionHelpNamesOnlyThatAction`, `TestHelpInvocationsAreDeterministicAndDoNotInspectRootOrStdin`, `TestHelpFlagAnywhereFollowsArgparse`, `TestHFlagAliasesHelpFlag` (`cmd/corvint`) | Each registered action's help has its usage, no other action's usage, and Verification options only for the verifying actions. `cem prepare --help` and `cem begin --patch p -h` print that action's help, and `cem --help` is unchanged |
| CEM-V1-341 | `TestCEMV1341AssemblyResultCarriesTopLevelOK` (`cmd/corvint-cem-candidate`) | For both result profiles, the output is `{"ok":true,` followed by the `json.Encoder` bytes after the opening brace |
| CEM-V1-342 | `TestMutatorsRefuseStableInput`, `TestMutatorsRefuseStableDestination` and the other `requireStableRefusal` callers (`internal/cem/workflow`) | Every stable-map mutator refusal carries `not-a-producer-map` and writes nothing |
| CEM-V1-343 | `TestDogfoodReasonAdmitsEveryRegisteredCEMCode` (`internal/observations`); `script/check-error-code-ownership.sh` | The code is admitted as a dogfood reason and owned by this spec |

## External criterion (V1-2100, GitHub #1332, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2100.

The plain criterion form (`CEM-V1-083`) can bind only an evidence span in the verified repository at
the base revision. Requirements kept in a separate repository had to be copied in before they could
be bound. These requirements add a third closed criterion form that binds a blob span in a separately
supplied repository. The addition is strictly additive to the frozen `cem/1.0` wire: every existing
map, schema member, criterion identity and result keeps its bytes, and the result profile
`cem-stable-verification/1` is unchanged. The normative algorithm text is the `CEM-V1-330..336`
section of `protocol/cem-1.0/stable/ALGORITHMS.md`.

- `CEM-V1-330`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) The external
  criterion form MUST have exactly the members `id`, `criterionIndex`, `criterionSha256`,
  `externalRepository`, `externalBlobOid` and `externalSpan`.
  - `externalRepository` is a consumer-side name matching `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`. It is
    never a path or URL.
  - `externalBlobOid` is a lowercase Git object ID of 40 or 64 hex digits.
  - `externalSpan` is the closed object `{start, end}` of wire integers with `start < end`, a
    zero-based, end-exclusive byte range of that blob.
  - `criterionSha256` is the SHA-256 of those bytes.
  - `protocol/cem-1.0/stable/stable.schema.json` gains the matching `externalCriterionBinding`
    branch; the plain and Tasks branches are unchanged.
- `CEM-V1-331`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) Form
  selection MUST be by key presence before any value check, with the closed error order unchanged:
  any Tasks member selects the Tasks form; otherwise any of `externalRepository`, `externalBlobOid`
  or `externalSpan` selects the external form, where `evidenceId` is an unknown member; otherwise the
  plain form applies.
  - A plain or Tasks map parses, encodes and identifies exactly as before.
  - A reader that predates this form selects the plain form for an external record and refuses the
    map at stage `wire` with `missing-field` (`evidenceId`). It never accepts.
- `CEM-V1-332`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) The
  criterion ID MUST be `criterion:sha256:` + SHA-256 of the canonical JSON of the five non-`id`
  members, with `externalSpan` written as `{"end":N,"start":M}`; a different ID is
  `invalid-criterion-identity` at stage `references`.
  - External and plain criteria share the unnamed criterion group, so `criterionIndex` is unique
    across both.
  - The tuple (`externalRepository`, `externalBlobOid`, `start`, `end`) occurs at most once
    (`duplicate-reference`).
  - An external criterion needs no evidence record and no Tasks artifact role. It is linked like any
    other criterion.
- `CEM-V1-333`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) The native
  `corvint cem verify-stable` and both portable commands (`cem01-go verify-stable` and
  `verify-stable-canonical`) MUST accept a repeatable `--external-repository NAME=PATH` option, one
  per distinct name.
  - `PATH` is an absolute, clean repository root.
  - A missing value, an invalid name, a repeated name or an unclean `PATH` is `invalid-arguments` at
    stage `arguments`.
  - A relative `PATH` is also `invalid-arguments`, and the `CEM-V1-117` absolute-path fix line names
    `--external-repository`; the path is refused, never resolved.
- `CEM-V1-334`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) After the
  declared artifacts and receipt authentication, each external criterion MUST be checked in map
  order.
  - A repository the consumer did not supply, or a blob it cannot read, is `UNSUPPORTED` with
    `accept: null`, stage `artifacts`, code `artifact-unavailable` and exit 2. The criterion stays
    NOT_OBSERVED; the verification never reaches `ACCEPT`.
  - A span that ends past the blob, or whose digest differs from `criterionSha256`, is `REJECT` with
    exit 1 and `issueCodes` `["artifact-digest-mismatch"]` at stage `artifacts`, and
    `referenceIntegrity` is `FAILED`.
  - Only when every external span is read from its supplied repository and matches can the
    verification `ACCEPT`. The criterion is echoed in `references` as declared.
- `CEM-V1-335`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) Each
  supplied root MUST be admitted under the same repository envelope as the verified repository and
  read in its own bounded object session, which is revalidated and closed before the next.
  - Every external read charges the same verification-wide Git-operation and distinct-blob budgets
    as the verified repository; a read that would exceed either abstains `artifact-unavailable`.
  - Any admission or read failure is `artifact-unavailable`.
  - Cancellation is `verification-timeout`.
  - An unobserved process cleanup keeps its containment code (`unsupported-process-containment`
    natively) at stage `repository`.
- `CEM-V1-336`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) After an
  `artifact-unavailable` abstention at stage `artifacts`, each command MUST print one stderr line that
  names, sorted, every external repository the map binds but the caller did not supply, and the fix
  `--external-repository NAME=/absolute/path`. The line never enters the closed result, and no line
  is printed when every name is supplied.
- `CEM-V1-337`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2100) The public
  case `protocol/cem-1.0/stable/external-criterion/` MUST record the map, both repository recipes,
  the exact ACCEPT result without `runtime` when the repository is supplied, and nine cases (not
  supplied, absent blob, digest mismatch, span past the blob, four wire refusals and a stale
  identity). Both consumers MUST return the recorded ACCEPT and the recorded exit, outcome, stage,
  code and issue codes for every case, and the stable manifest lists every new file.

Non-goals:
- no wire-version bump, no new result member, no new result code and no change to
  `RESULT.schema.json`;
- no content-digest-plus-locator form: the repository-name, blob-ID and span form was chosen because
  it is verifiable from Git objects alone;
- no producer support: `corvint cem prepare`, `cem cite` and `corvint-cem-candidate` do not emit the
  external form;
- no external repository on the Core read paths (`corvint cem verify`, `status`, `report`, and the
  console), which never supply one, so an external map abstains there;
- no network fetch, URL or remote name: the consumer always names a local root.

Failure modes:
- The caller omits `--external-repository`. The verification is `UNSUPPORTED` `artifact-unavailable`
  and stderr names the missing repository; it is never `ACCEPT`.
- The caller supplies the wrong repository, or one without the blob. The result is the same
  abstention, because the bytes were not observed.
- The caller supplies a repository whose blob has different bytes at the span. The verification
  rejects with `artifact-digest-mismatch`.
- An older consumer receives an external map. It refuses `missing-field` at stage `wire`.
- The result does not mark the individual criterion NOT_OBSERVED: the profile has no per-criterion
  observation member, and adding one would change the frozen result profile (`CEM-V1-091`). The
  whole verification abstains instead. A per-criterion member is recorded as an owner fork.
- `RESULT.schema.json` describes `references.criterionBindings` with the Tasks form only, while both
  consumers already echo plain criteria there; external criteria are echoed the same way. This
  pre-existing schema gap is recorded, not changed.

Acceptance evidence: the tests below pass on Darwin. Before the change, the base-revision portable
reader (`cem01-go verify-stable-canonical`) and the base-revision native `wire.ParseStable` both
refused `external-criterion/stable.json` with `REJECT` stage `wire` `missing-field` (`evidenceId`),
recorded as manual evidence in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-2100-external-criterion.md`.

Rollback: revert the change commit. The external form, option, case and schema branch disappear;
maps that use the external form then fail closed at stage `wire` again, and no other map, identity
or result changes.

### External criterion traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-330 | `TestCEMV1330ExternalCriterionWire`, `TestCEMV1331ExternalCriterionRefusals` (`internal/cem/wire`) | The public map parses as six members with a valid span; invalid names, OIDs and spans refuse `invalid-field` |
| CEM-V1-331 | `TestCEMV1331ExternalCriterionRefusals`, `TestCEMV1331PlainAndTasksIdentityUnchanged` (`internal/cem/wire`), `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083TasksFormUnchanged` | Selection order and unknown `evidenceId`; plain and Tasks results unchanged; base-revision readers refuse `missing-field` (build log) |
| CEM-V1-332 | `TestCEMV1330ExternalCriterionWire`, `TestCEMV1331ExternalCriterionRefusals` (`duplicate-span`), `TestCEMV1334ExternalCriterionPublicCase` (`external-stale-identity`) | Identity recomputes, duplicates refuse, a stale ID is `invalid-criterion-identity` in both consumers |
| CEM-V1-333 | `TestCEMV1333ExternalRepositoryOption` (`internal/cem/cli`), `TestCEMV1333ExternalRepositoryOptionPortable` (`interop/cem01-go`) | Six bad values refuse at `arguments` in all three commands; relative path names the fix |
| CEM-V1-334 | `TestCEMV1334ExternalCriterionPublicCase` (`internal/cem/verify`), `TestCEMV1334ExternalCriterionPublicCasePortable` (`interop/cem01-go`) | Exact ACCEPT when supplied; not-supplied and absent blob abstain; mismatch and span past the blob reject |
| CEM-V1-335 | `TestCEMV1334ExternalCriterionPublicCase` (non-repository root), `TestCEMV1335ChargeBlobsToSharesTheBudget` (`internal/cem/gitauth`), `TestCEMV1335ExternalReadsChargePrimaryBudgets` (`interop/cem01-go`) | A supplied root that is not a repository is `artifact-unavailable`; an exhausted primary budget abstains in both portable readers and the native blob charge is carried back; containment and timeout mapping by code review |
| CEM-V1-336 | `TestCEMV1336ExternalFixLine` (`internal/cem/verify`), `TestCEMV1333ExternalRepositoryOption`, `TestCEMV1333ExternalRepositoryOptionPortable` | The exact stderr line when unsupplied and none when supplied; nothing in the result |
| CEM-V1-337 | `TestCEMV1334ExternalCriterionPublicCase`, `TestCEMV1334ExternalCriterionPublicCasePortable` (manifest check via `checkStableCaseManifest` and `stableCaseFiles`) | Both consumers match `EXPECTED.json`; every case file is in `manifest.json` |

## Plain assembly verify route (V1-2105, GitHub #1329, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2105.

After plain `corvint-cem-candidate assemble-stable` the result named no verify route. `cem prepare`'s
next action points `--artifacts` at the repository root and assumes the producer map is committed
there, so an adopter copied the assembled map over the producer map path in a scratch clone to verify
it. The bundle already verifies in place: the assembler's own stable verification reads its
`stable.json` with the bundle directory as the artifact root. `cem verify-stable` takes that map at
any path and names the repository itself, so it needs no working directory or `--root`; `cem verify`
can also read the out-of-tree map since `CEM-V1-315`, but only from a resolved Core root.

- `CEM-V1-350`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2105) A successful
  plain `assemble-stable` (`cem-stable-plain-assembly/1`) result MUST end with one `nextActions`
  member holding exactly one argv array:
  `["corvint","cem","verify-stable","--repository",R,"--map",S,"--expected-base",B,"--target",T,"--artifacts",D]`.
  - `R`, `B` and `T` are the request's `repository`, `expectedBase` and `target` strings, byte for
    byte; `S` is the result's `stablePath`; `D` is the `--out-dir` bundle directory.
  - These are exactly the options of the assembler's own stable verification.
  - This amends `CEM-V1-341` for the plain profile only: `"ok": true` still leads, `nextActions`
    trails, and every byte between them is the result's unchanged encoding. The Tasks-bound
    `cem-stable-assembly/1` result and every refusal are unchanged.
- `CEM-V1-351`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2105) Run unchanged
  on the bundle it names, with no copy of the map into any repository, the emitted action MUST
  `ACCEPT` with exit 0. Its result MUST equal the assembly result's `verification`, and the
  repository's working-tree status MUST be unchanged.

Non-goals:
- no next action for the Tasks-bound `cem-stable-assembly/1` result, which has no Tasks-store-free
  fixture in this package; its bundle uses the same layout and route;
- no `cem verify` or `cem status` form of the action, and no policy ceilings in it: those need a Core
  root, and `cem verify-stable` is the route that reads the bundle from any working directory;
- no out-of-tree `cem anchor` or verdict-ledger mode (still deferred under `CEM-V1-315..317`);
- no change to the assembly structs in `internal/cemcandidate`, the stable verifier or the map wire.

Failure modes:
- The bundle is moved. The action names the original absolute paths, so it must be rewritten to the
  new `--map` and `--artifacts`; the stable verifier then refuses or rejects on missing or changed
  bytes, never accepts silently.
- A consumer that compared the whole plain result to `CEM-V1-341`'s bytes now sees one more trailing
  member.
- `corvint` is resolved from the caller's PATH, so an older Core binary can return its own verdict.

Acceptance evidence: the test below passes on Darwin. Without the change the result has no
`nextActions` and the test fails at its first assertion.

Rollback: revert the change commit. The plain result loses `nextActions` and returns to the
`CEM-V1-341` bytes; bundles and their verification are unchanged.

### Plain assembly verify route traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-350 | `TestCEMV1350PlainAssemblyNextActionVerifiesBundle`, `TestCEMV1341AssemblyResultCarriesTopLevelOK` (`cmd/corvint-cem-candidate`) | The neutral-case plain result ends with the exact verify-stable argv; with no next action the output keeps the `CEM-V1-341` bytes |
| CEM-V1-351 | `TestCEMV1350PlainAssemblyNextActionVerifiesBundle` (`cmd/corvint-cem-candidate`) | The emitted action, run through `cli.RunStableInvocation` (the Core `verify-stable` entry), exits 0 with `ACCEPT`, equals the assembly's verification and leaves `git status` unchanged |

## Repository envelope over real checkouts (V1-0595 U2, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0595.

The `canonical-repository-bounded/1` envelope reads no Git configuration before admission and takes
the object format only from `git rev-parse --show-object-format`, so ordinary checkouts with an
origin configuration, linked worktrees and SHA-256 repositories need no special parsing. Until this
change only the portable consumer ran the 29 public cells of
`protocol/cem-1.0/stable/repository-envelope-packet/inherited/s0e/REPOSITORY-ENVELOPE.cells.json`
over real Git repositories (`TestStableCanonicalEnvelopeCells`). The native verifier was qualified
only on hand-built layouts and the canonical ordered fixtures, which do not qualify ordinary
checkouts. These requirements qualify both consumers on real checkouts and freeze the answer for
adversarial shapes the cells do not name. Every shape is either the same repository as a public
positive cell, with the identical full result, or a typed refusal at stage `repository`.

- `CEM-V1-360`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) The native
  stable verifier MUST return each public cell's complete 23-key result and exit code for a fresh
  `git init` repository, and for one created with `git init --object-format=sha256`, both with the
  configuration Git wrote and both with the inert `remote.origin.url`, `user.name` and
  `branch.main.remote` settings of the origin cells (`ordinary-*`, `origin-*`). The settings never
  select or widen the envelope.
- `CEM-V1-361`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) The native
  stable verifier MUST return each linked cell's complete result for a worktree made by
  `git worktree add --detach`, with an absolute marker and with a `--relative-paths` marker, in both
  object formats (`linked-*`, `relative-*`). A wrong back-pointer, an absolute `commondir` and
  per-worktree attributes MUST refuse exactly as their cells state. Building a `relative-*` cell
  needs `git worktree add --relative-paths`, which Git added in 2.48. That is above the floor of
  Git 2.41 set by `CEM-V1-185`. Both tests read the option from the host Git's own usage text, and on a host
  without it they skip only the two relative cells, reporting them as `NOT_RUN`.
- `CEM-V1-362`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) Every other
  public cell MUST give the native verifier its complete published result over the same real-Git
  recipe the portable consumer runs: attributes, alternates, no-follow links, a symlinked root,
  shallow clones, missing or corrupt base objects, hostile configuration and environment (with no
  helper executed), ambient alternates, a promisor configuration and a missing target tree. Since
  V1-2133 there is no exception (`CEM-V1-367`, resolved).
- `CEM-V1-363`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) The envelope
  does not require the administrative directory to sit inside the repository: the packet excludes
  root containment. A worktree whose marker names an administrative directory outside the common
  directory MUST be admitted when the path to it is a chain of ordinary directories, its back-pointer
  reciprocally identifies the worktree, and its `commondir` is relative. That case
  (`gitdir-outside-reciprocal`) returns `linked-sha1`'s complete result in both consumers. Each of
  the following malformed cases MUST refuse with `repository-object-unavailable` in both consumers:
  - the same accepted outside directory with its back-pointer removed;
  - another repository's real worktree administrative directory, whose back-pointer names that other
    worktree;
  - a relative marker that reaches otherwise valid, reciprocal outside metadata through a
    symbolic-link ancestor.

  Each refusal first proves that its fixture is accepted, and only then introduces its one defect.
- `CEM-V1-364`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) A `commondir`
  that names a symbolic link (a self-loop, or a link to the real common directory), or a `commondir`
  file that is itself a symbolic link, MUST refuse with `repository-object-unavailable` in both
  consumers. No link is followed and no loop is walked.
- `CEM-V1-365`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) Configuration
  syntax is Git's to interpret, never the envelope's. Both consumers MUST return a positive cell's
  complete result in these two cases:
  - a SHA-256 configuration rewritten with `[Extensions]` before a mixed-case `[CoRe]` section and
    mixed-case keys returns `ordinary-sha256`'s result;
  - a version 0 repository naming an unknown extension, which Git ignores below version 1, returns
    `ordinary-sha1`'s result.
- `CEM-V1-366`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595) A configuration
  that Git itself refuses MUST refuse with `git-read-failed` in both consumers, with no guessed object
  format. This covers:
  - an unknown extension in a version 1 repository;
  - a SHA-1 object store whose configuration claims `objectFormat = sha256`;
  - a SHA-256 object store whose configuration drops the extension.
- `CEM-V1-367`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0595; closed as
  resolved by V1-2133 on 2026-10-21, see `CEM-V1-568`) The divergence on `missing-target-blob` and
  `corrupt-target-blob` MUST stay visible until follow-up V1-2133 lands. On 2026-10-20 the owner
  decided to keep the sealed packet. Under V1-2133 the native verifier answers `git-diff-failed` in
  ordinary repositories and keeps `repository-object-unavailable` only for promisor and
  partial-clone repositories. Before V1-2133, on these cells:
  - the packet (`PROPOSED-RULES.md` section 4: cause provenance, a changed blob missing during Git
    diff is `git-diff-failed`) and the portable consumer answer `git-diff-failed`;
  - the native verifier walks the change set after the failed diff and answers
    `repository-object-unavailable`, the V1-0349 partial-clone contract that
    `TestMapCoreVerbsRefuseAPromisorObjectWithoutFetching` pins for `cem status` and `cem verify`;
  - both answer `UNSUPPORTED` at stage `repository` with exit 2 and the same 21 other keys; only
    `code` and `verdictSha256` differ;
  - the native test pins the native code and compares every other key exactly. Its expected
    `verdictSha256` is recomputed by hand from that expectation, so V1-2133 removes the table
    entries deliberately.

Non-goals:
- no change to the admission order, the stable profile, the packet, the cells or any file under
  `protocol/cem-1.0/stable/`;
- no resolution of `CEM-V1-367` here; the native change is V1-2133 (`CEM-V1-568`);
- no change to `primary-clean-config-bounded/1` or the candidate profile's case-sensitive config
  grammar in `interop/cem01-go/cem.go`;
- no Windows qualification; both tests run on Darwin and Linux only.

Failure modes:
- An outside administrative directory is admitted only by proof: ordinary directories along the
  way, a reciprocal back-pointer and a relative `commondir`. Anything less refuses, even where Git
  itself would still use the directory.
- On a host Git older than 2.48 the two `relative-*` cells are `NOT_RUN` on both consumers, never
  passed.
- A Git release that starts honouring unknown extensions in version 0 repositories, or that stops
  refusing an object-format mismatch, changes the answer in `CEM-V1-365`/`CEM-V1-366`. The tests then
  fail, and the change is reviewed rather than absorbed.
- While `CEM-V1-367` stands, the two consumers' `verdictSha256` values differ on a repository that
  lacks, or holds corrupt, a changed blob. A gate that requires both consumers to recompute the same
  verdict refuses such a repository, which is fail-closed.

Acceptance evidence: on Darwin arm64 with Go 1.27.3 and Apple Git 2.54.0, the native test runs the
29 cells: 27 match exactly, and 2 match on every key except the retained `CEM-V1-367` code and its
digest. Both consumers give the identical answer on the 12 adversarial shapes, and the portable
consumer still passes all 29 cells. Build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-0595-repository-envelope-real-checkouts.md`.

Rollback: delete the two test files. No product code, wire byte or packet byte changed.

### Repository envelope over real checkouts traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-360 | `TestStableCanonicalEnvelopeCellsNative` (`internal/cem/verify`) | `ordinary-sha1`, `ordinary-sha256`, `origin-sha1` and `origin-sha256` give the published 23-key result with exit 0 |
| CEM-V1-361 | `TestStableCanonicalEnvelopeCellsNative` (`internal/cem/verify`) | `linked-*` and `relative-*` in both formats accept; `linked-wrong-backpointer`, `linked-missing-common` and `linked-attributes` refuse as published |
| CEM-V1-362 | `TestStableCanonicalEnvelopeCellsNative` (`internal/cem/verify`), `TestStableCanonicalEnvelopeCells` (`interop/cem01-go`) | The remaining public cells match natively; no sentinel executes; the portable consumer passes all 29 |
| CEM-V1-363 | `TestStableEnvelopeAdversarialShapes` (`internal/cem/verify`, `interop/cem01-go`) | `gitdir-outside-reciprocal` gives `linked-sha1`'s full result; `gitdir-outside-no-backpointer` and `gitdir-relative-symlinked-ancestor` first prove the same accepted fixture, then refuse with `repository-object-unavailable`, as `gitdir-outside-foreign-worktree` does, in both consumers |
| CEM-V1-364 | `TestStableEnvelopeAdversarialShapes` (`internal/cem/verify`, `interop/cem01-go`) | `commondir-symlink-loop`, `commondir-symlink` and `commondir-file-symlink` refuse with `repository-object-unavailable` in both consumers |
| CEM-V1-365 | `TestStableEnvelopeAdversarialShapes` (`internal/cem/verify`, `interop/cem01-go`) | `sha256-extensions-first-mixed-case` and `unknown-extension-v0` give the full result of `ordinary-sha256` and `ordinary-sha1` |
| CEM-V1-366 | `TestStableEnvelopeAdversarialShapes` (`internal/cem/verify`, `interop/cem01-go`) | `unknown-extension-v1`, `sha1-store-claims-sha256` and `sha256-store-claims-sha1` refuse with `git-read-failed` in both consumers |
| CEM-V1-367 | `TestStableCanonicalEnvelopeCellsNative` (`internal/cem/verify`), `TestStableCanonicalEnvelopeCells` (`interop/cem01-go`), `TestMapCoreVerbsRefuseAPromisorObjectWithoutFetching` (`cmd/corvint`) | Resolved by V1-2133 (`CEM-V1-568`): both consumers give `git-diff-failed` and the packet's `verdictSha256` on the two blob cells; the native test asserts 29/29 exact cells |
| CEM-V1-960 | `TestOpenStableSymlinkedPathIsRefusedUpFront_V12519`, `TestStableBoundaryResolvedPathValidates_V12519` (`internal/cem/gitauth`) | Ancestor link, root link and a gitfile naming a linked path refuse up front with `repository-object-unavailable` naming the link; the resolved path validates |

### Stable review rows carry citation tier, anchors and evidence location (V1-1339, 2026-10-21)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1339.
Amends the `CEM-V1-034` row content additively; the projection stays a pure function of the result.

The requirement is `CEM-V1-368` in `## Requirements`.

Non-goals: line numbers and a quoted span. Both need repository reads, which `CEM-V1-034` forbids in
the projection; they remain a residual needing a contract decision. No verifier, wire or packet byte
changes, and the portable consumer is untouched.

Failure modes: a result refused before citation renders `NOT_PRODUCED` tiers and sorts every row
first; a missing drift entry renders `NOT_PRODUCED` for its location. Record-set digests of every
stable review change once, which is intended.

Acceptance evidence: `TestCEMV1368StableRowsCarryTierAnchorsAndLocation` renders a self-cited
two-hunk map (the 03a8ca10 shape) and an anchored map to different tier lines, rows and record-set
digests, and fails without the change. Build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-cem-1021d-review-tiers-and-check-warnings.md`.

Rollback: revert the projection and renderer change; no stored state depends on it.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-368 | `TestCEMV1368StableRowsCarryTierAnchorsAndLocation` (`internal/cem/workflow`) | Exact tier line, tier/anchor row, `path bytes start-end` row, weakest-first order with the map ordinal kept, and `NOT_PRODUCED` without a citation |

## Per-action help for `help cem`, `verify-stable` and `gate-verdict` (V1-2109, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2109.

Non-goals: no change to the argv, output or exit codes of a non-help `verify-stable` or
`gate-verdict` invocation; no change to `corvint cem --help`; no other `help cem` topics.

Failure modes:
- A script that expected `cem verify-stable --help` to print an `invalid-arguments` result now gets
  help text and exit 0.

Acceptance evidence: `TestCEMV1340HelpCEMActionAndOwnedArgvActions` and
`TestCEMV1340CEMActionHelpNamesOnlyThatAction` (`cmd/corvint`) pass; disabling the help check in
`runCEMInvocation` makes the first fail.

Rollback: revert the change commit. `help cem ACTION` refuses as an unknown topic again and the two
actions refuse `--help` as an invalid argument.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-344 | `TestCEMV1340HelpCEMActionAndOwnedArgvActions`, `TestCEMV1340CEMActionHelpNamesOnlyThatAction` (`cmd/corvint`) | Every registered action's `help cem ACTION` output equals its action help; `verify-stable` and `gate-verdict` print theirs on `--help`/`-h` with and without `--root`; `verify-stable --map --help` still refuses with exit 2 |

## Verdict ledger rollback read and two-process anchor race (V1-2024, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2024.

Non-goals: no rollback witness kept by Corvint (the pinned OID is the caller's record); reflog
inspection; any change to anchor writes, the ledger record or the `--notes-tip` read.

Failure modes:
- A caller that pins no tip still cannot detect a rewound ledger; a rewind to a valid earlier
  prefix reads `verified` without `--verdicts-tip`.

Acceptance evidence: `TestCEMV1345RewoundVerdictTipReadsRollback` and
`TestCEMV1346TwoProcessAnchorsOneWins` (`internal/gitnotes`) pass. Dropping the rollback override
makes the first fail, and dropping the `update-ref` old value makes the second fail.

Rollback: revert the change commit. `--verdicts-tip` becomes an unrecognized argument again, and no
stored ledger, note or map depends on it.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-345 | `TestCEMV1345RewoundVerdictTipReadsRollback`, `TestCEMV1345PinnedTipReadIgnoresARefMovedAwayAndBack` (`internal/gitnotes`); `TestCEMV1345VerdictsTipThroughTheCLI` (`cmd/corvint`); `ProvenanceWithVerdictsTip`, `refTipState` in `internal/gitnotes/provenance.go` | Equal and ancestor pins read `current` with the verified record; a ref reset to an older commit and a deleted ref read `rollback` with no records; the two argument refusals; repository state unchanged across every read; a ref moved A to B to A mid-read still lists only A's verified record |
| CEM-V1-346 | `TestCEMV1346TwoProcessAnchorsOneWins` (`internal/gitnotes`), which re-executes the test binary as two anchor processes; `publishNotes` in `internal/gitnotes/anchor.go` | The held process validates and stages, the free process wins, the held process is refused `verdict-ledger-conflict`, and the ledger holds only the winner's line with no staging ref |

## Plain assembly write-failure cleanup and bundle-parent limit (V1-1471, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1471.

Non-goals: a temporary sibling with an atomic rename; any change to the bundle layout, the
refusals or the Tasks-bound producer.

Failure modes:
- A process killed between create and cleanup can still leave a partial file; the next assembly
  into a new directory is unaffected, because the bundle directory must be new.

Acceptance evidence: `TestCEMV1347WriteNewFailureRemovesThePartialFile` (`internal/cemcandidate`)
injects a sync failure through the `syncNew` seam and passes. Dropping the cleanup `Remove` makes
it fail.

Rollback: revert the change commit. The seam is a test-only package variable and no stored
artifact depends on it.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-347 | `TestCEMV1347WriteNewFailureRemovesThePartialFile` (`internal/cemcandidate`); `writeNew`, `syncNew` in `internal/cemcandidate/plain.go` | The injected failure is returned, no `stable.json` remains, and the name is reusable |
| CEM-V1-348 | Recorded limit; `newBundle` in `internal/cemcandidate/plain.go`; finding 2 in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-cem-plain-criterion-producer.md` | Not closed; the threat model above |

## Dashboard cem-stable refusal names the verifier step (V1-2034, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2034.

Decision (conservative fork): keep the accepted result with an explicit stage instead of dropping
it, so a reader never mistakes an ACCEPT result for the refusal's verdict; the top-level refusal
still decides.

Non-goals: no change to refusal codes, to the stable verifier, or to the OCM or frontier callers'
outputs.

Failure modes:
- An admission step added later without the wrapper reads `NOT_RUN` although the verifier ran;
  the stage then under-reports, it never claims a verdict that did not happen.

Acceptance evidence: `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` asserts the
stage for all seven controls (stale target and unbound OCM read `accepted`; legacy map reads
`NOT_RUN` with the reason), that the result is ACCEPT exactly when the stage is `accepted`, and
that the reason is present exactly when the stage is `NOT_RUN`. Returning the bare phase error
makes the unbound-OCM case fail.

Rollback: revert the change commit; the two fields are additive on a draft (`/0`) refusal.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-349 | `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` (`cmd/corvint-dashboard-snapshot`); `StableAdmissionRefusal` in `internal/lrfrepo/adapter.go` | Stage, reason and result agree for every control |

## Portable legacy verify-stable tree-entry bound (V1-1472, 2026-10-20)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1472.

Decision (conservative fork): record the existing bound as intentional instead of raising it, so
no verifier reads more than it has declared and the canonical route stays the only
repository-scale portable verifier.

Non-goals: no change to either route's bounds, codes or results.

Failure modes:
- A reader that treats the legacy `UNSUPPORTED` as a REJECT misreads it; the outcome is
  `UNSUPPORTED`, not `REJECT`.

Acceptance evidence: `TestCEMV1352LegacySnapshotTreeEntryBound` (`interop/cem01-go`) builds
flat trees of 4096 and 4097 entries: 4096 reads every entry, 4097 refuses
`unsupported-resource-limit`. Raising the bound to 4097 makes the test fail.

Rollback: revert the change commit; it adds a test and this record only.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-352 | `TestCEMV1352LegacySnapshotTreeEntryBound` (`interop/cem01-go`); `snapshot03` in `interop/cem01-go/cem.go` | 4096 entries read; 4097 refuse `unsupported-resource-limit` |

## Lane cem sweep (V1-2136, V1-1329, V1-2106, V1-1470, 2026-10-21)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2136,
V1-1329, V1-2106, V1-1470.

Four independent gaps:
- The sealed `REPOSITORY-ENVELOPE.md` recipe names `CEM_STABLE_REPOSITORY_CASES` and four native
  tests in `internal/cem/verify` that did not exist, so the recipe could not be run (V1-2136).
- The `CEM-V1-226` tamper matrix compared each case's verdict, stage and citation in both consumers,
  but not its `verdictSha256`, so a divergent verdict digest between the native and the portable
  consumer passed (V1-1329).
- `cem status`/`cem verify` with an out-of-tree map (`CEM-V1-315`) and `cem verify-stable` with an
  absolute `--output` checked the parent directory by path and then read or wrote by path again. A
  parent swapped for a symlink in that window redirected the read, or the report write, into the
  repository's Git directory (V1-2106).
- `TestStableLifecycleOpTimeoutRetiresDescendants` armed a 500 ms operation timer before the shim
  launched. Under host load the timer fired before the shim recorded its descendant, and the test
  failed with "descendant pid not recorded" (V1-1470).

The requirements `CEM-V1-242..245` are defined in the Requirements section above.

Non-goals:
- no change to any sealed member, committed map, the stable verifier's verdicts or the map wire;
- `CEM-V1-244` closes the external map and report window of `CEM-V1-315..316` only. The
  plain-assembly bundle-parent window stays the named open limit `CEM-V1-348`;
- no full `interop/cem01-go` runtime bound under load (V1-1470 criterion 2 stays open);
- no rerun of V1-1329's original reproduction target, which is not in this clone.

Failure modes:
- The retry spends up to about 40 s on a host so loaded that every short timeout fires first; the
  final 30 s attempt fails the test rather than passing silently.
- On a platform without `O_NOFOLLOW`/`os.Root` support the held read refuses rather than falling
  back to a path read.
- A legitimate replacement of the output directory between the check and the publish is refused;
  the caller reruns.

Acceptance evidence: the tests below pass on Darwin. Without the `CEM-V1-244` change the map test
reads the redirected file and the report test writes into `.git`. A mutated pinned digest fails both
tamper-matrix tests. With a simulated delayed shim the previous op-timeout test fails and the new
one passes on its 2 s retry.

Rollback: revert the lane commits. Out-of-tree maps and absolute reports return to path reads and
writes, the digest pins and envelope names disappear, and no stored state changes.

### Lane cem sweep traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-242 | `TestStableRepositoryEnvelopeOrdinary`, `TestStableRepositoryEnvelopeCancelled`, `TestStableRepositoryEnvelopeDescendantCleanup`, `TestStableRepositoryEnvelopeRootIdentity` (`internal/cem/verify`) | The materialized array runs through the env contract; the public cancellation, cleanup and root-identity cases pass |
| CEM-V1-243 | `TestCEMV1226TamperMatrixNative` (`internal/cem/verify`), `TestCEMV1226TamperMatrixPortable` (`interop/cem01-go`) | All 23 sealed cases match the shared pinned digests; a mutated pin fails |
| CEM-V1-244 | `TestCEMV1244ExternalMapParentSwapRefused`, `TestCEMV1244ExternalReportParentSwapRefused`, `TestCEMV1244ExternalReportPublishes` (`internal/cem/workflow`) | A swapped map parent refuses `map-unavailable`; a swapped report parent refuses `publish-failed` with nothing written in `.git`; an unswapped report publishes at 0600 with no residue |
| CEM-V1-245 | `TestStableLifecycleOpTimeoutRetiresDescendants` (`interop/cem01-go`) | Passes in 0.56 s; with a delayed shim it passes on the 2 s retry where the previous test failed |

## Ordinary-repository diff failure keeps cause provenance (V1-2133, 2026-10-21)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2133.
Resolves `CEM-V1-367`.

The requirement is `CEM-V1-568` in `## Requirements`.

Non-goals: no packet, cell, portable-consumer or wire change; no change to cancellation, timeout or
budget classification during the walk.

Failure modes: a partial clone whose promisor packs were all removed is treated as ordinary and a
missing promised blob answers `git-diff-failed`, still a refusal at stage `repository` with exit 2.

Acceptance evidence: `TestStableCanonicalEnvelopeCellsNative` logs `exact=29 notRun=0 of 29` and
fails unless every cell is exact or `NOT_RUN`; `TestCanonicalDiffOrdinaryRepositoryMissingBlobIsDiffFailed`
pins both answers over one repository; `TestPromisorObjectNeverFetchedByAGitThatDropsTheLazyFetchGuard`
and `TestMapCoreVerbsRefuseAPromisorObjectWithoutFetching` still pass.

Rollback: revert the `isPromisor` guard in `internal/cem/gitauth/diff.go` and restore the two-entry
divergence table in the native envelope test; `CEM-V1-367` then reopens.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-568 | `TestStableCanonicalEnvelopeCellsNative` (`internal/cem/verify`), `TestCanonicalDiffOrdinaryRepositoryMissingBlobIsDiffFailed` (`internal/cem/gitauth`), `TestMapCoreVerbsRefuseAPromisorObjectWithoutFetching` (`cmd/corvint`) | 29/29 exact native cells; ordinary repository `git-diff-failed`, promisor-marked repository `repository-object-unavailable` |

## Binary-content refusal names the path (V1-0301, decision 0548, 2026-10-21)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0301,
narrowed by decision 0548. `cem/1.0` does not represent binary content, and this section only makes
that limit name the file that hit it. Amended by `CEM-V1-980..986` (V1-2752): the canonical
full-index binary section is now a hunk-less group bound by blob identity. Mode-only and empty-file changes are already covered
by `CEM-V1-300`. Representing binary or mode changes in a map (CEM-PILOT-001) moves to a post-1.0
profile.

The requirement is `CEM-V1-037` in `## Requirements`.

Non-goals: no new refusal code; no wire, result or portable-consumer change (the portable
`verify-candidate` result already carries only the code); no binary or mode representation.

Failure modes: a quoted or traversing path is not named, so the message stays generic. A patch
with several binary groups names only the first one the parser reaches.

Acceptance evidence: the tests below pass. Each named-path case fails without the parser change,
where every marker gave the generic message. Build log
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-cem-authority-1021t-binary-path-runner-authority.md`.

Rollback: revert `binaryRefusal` and `binaryMarkerPath` in `internal/cem/patch/patch.go`. The
refusal code and every stable result are unchanged either way.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-037 | `TestBinaryRefusalNamesPath` (`internal/cem/patch`), `TestPrepareBinaryRefusalNamesPath` (`internal/cem/workflow`) | Modify, `GIT binary patch`, rename and bare creation/deletion markers name the path; bare and traversing markers keep the generic message; `cem prepare` on real binary-only creation, modification and deletion commits refuses `binary-patch` naming the path; a mixed text+binary commit prepares under `CEM-V1-980` |

## Current versus historical Tasks authority and composed runner repairs (V1-2018, 2026-10-21)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-2018.
V1-0632 left two gaps in the `CEM-V1-286` chain. It did not show that a `GRANT` for a superseded
tree has no authority, and it did not show how separate runner repairs combine into one admitted
receipt. This section closes both gaps with fixture tests. It changes no product code, wire member,
verifier rule or Tasks rule. The tests pin the existing Tasks gate-currency rules (`gateVerdict`,
`completionBlocker`) and the producer's `CEM-V1-283` stale check in this chain.

The requirements are `CEM-V1-038` and `CEM-V1-039` in `## Requirements`.

Non-goals:
- no new refusal code;
- no change to how Tasks stores or reuses gate results;
- no receipt merging: a composed receipt is a fresh runner execution over the composed source,
  never a union of earlier receipts;
- no receipt signing (`FPK-V0-036`).

Failure modes: if Tasks treated a gate result from another tree as current, the historical `GRANT`
would complete the ticket and `CEM-V1-038` fails. If the producer stopped comparing input digests
with the target, an isolated repair's receipt would be admitted and `CEM-V1-039` fails.

Acceptance evidence: both tests pass. With the tree comparison in `gateVerdict` disabled, the chain
test fails because the historical `GRANT` completes the ticket. With the `CEM-V1-283` digest
comparison disabled, the composition test fails because an isolated repair receipt is admitted.
Build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-cem-authority-1021t-binary-path-runner-authority.md`.

Rollback: revert the `internal/cemchain/chain_test.go` changes and these rows. No product code
changed.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-038 | `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | Superseded seal `STALE_TREE`, historical `GRANT` `GATE_STALE`, failed verdict `GATE_FAILED`, restored tree `GATE_STALE` until re-gated, then `COMPLETED`; fails with the `gateVerdict` tree check disabled |
| CEM-V1-039 | `TestCEMV1039RunnerRepairsComposeOnOneBoundSource` (`internal/cemchain`) | Both isolated repair receipts are refused as `runner-receipt-stale` with no bundle; the composed receipt is admitted, observing `TestAdd` and `TestMul` `PASSED`; fails with the input digest comparison disabled |

## Release readiness over the stable result (V1-2323, MIGRATION A02, 2026-10-23)

Status: proposed (V1-2323); owner acceptance and promotion open. `MIGRATION.json` addition A02
said that the companion release Core evidence readers (`internal/companionrelease`) used
`wire.Parse` only as a strict JSON parser. They never recognized the exact `cem/1.0` stable result,
its mode, or its authority, runtime and envelope limits. No gate ran the sealed suites against the
built release binaries either. This section adds a library reader and a binary-level suite runner.
Neither is called by a release workflow yet. The workflow wiring is an owner step under
`.github/`, described in the V1-2323 pull request.

The requirements are `CEM-V1-569` to `CEM-V1-572` and `CEM-V1-600` to `CEM-V1-605` in
`## Requirements`. The second block (V1-2323, 2026-10-24) integrates the reader: a committed
readiness declaration, a readiness verdict over both built release binaries, and the opt-in
`make cem-release-gate` target, after which A02 leaves `OPEN`.

Non-goals:
- no release workflow, CI job or `.github/` change; no promotion; no Tasks completion;
- no change to the stable wire, either verifier, `RESULT.schema.json` or any sealed case;
- no JSON Schema validation of the nested `hunks`, `references` and `drift` records beyond typed
  decoding and the verdict binding;
- the committed declaration is the source of truth for which runner, platform and external rows
  a release requires; an empty declaration is refused rather than read as complete;
- no observation of external interoperability, of the repository-envelope cells against the
  built binaries, or of a platform other than the one the binaries were built for: those rows
  stay unobserved and the gate stays refused (recorded limits, never passes).

Failure modes: a third-party or retired-profile result, a reordered or extended result, a forged
verdict, or a `null` that typed decoding would turn into zero is refused as schema or mode. A result that claims native authority is refused as
authority. A result from an experimental or unsupported tuple or envelope is refused as admission,
even though its `verdictSha256` stays valid (the verdict excludes `runtime` and
`repositoryEnvelope`). Release evidence that is missing, partial, failed or skipped is refused per
class. A sidecar, structural proof or tuple that contradicts an accept is refused as not accepted
or admission. The suite runner refuses binaries with skewed or dirty builds before running
anything, reads only committed objects, and reports the first case where the two consumers
disagree. Build stamps can be forged, so the runner reports them as unauthenticated and pins the
binary bytes; it does not prove how the binaries were built.

Acceptance evidence: `TestCEMV1569StableReleaseReadiness` (`internal/companionrelease`) runs one
accepted case and refusals for every class. The accepted case is the public
`neutral-criterion/results/accept.json` line, and the admission, authority, runner, platform and
external refusals keep its passing reference integrity. `TestCEMV1572ReleaseBuildsMustMatch`
(`tools/cem-release-suites`) covers the provenance refusals. Locally, `make cem-release-suites`
over clean same-commit builds passed 23 tamper-matrix, 8 citation-vector, 8 neutral-criterion and
19 receipt-authentication cases on both binaries, plus the source-level envelope tests. A
`cem01-go` built from a different commit was refused with `builds differ`. The review repair
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-cem-release-readiness-review-repair.md`) records the rerun.
The 2026-10-24 integration adds `TestCEMV1600ReadinessDeclaration`,
`TestCEMV1601ReadinessReadsEveryResultFirst` (`internal/companionrelease`),
`TestCEMV1602ObservedRowsNeverPassUnrunRows`, `TestCEMV1602BuildPlatform`,
`TestCEMV1603JudgeReadiness`, `TestCEMV1603ReadinessPathIsRepositoryRelative`
(`tools/cem-release-suites`) and the Node check `tools/cem-release-suites/readiness.test.mjs`.
`MIGRATION.json` A02 is `IMPLEMENTED_EXPERIMENTAL`, pinned by
`TestCEMV1604MigrationA02Integrated`. The local `make cem-release-gate` run and the real
V1-2322 lifecycle over this change are recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-1-0-real-lifecycle.md`.
Its sealed map is [`.corvint/changes/fafdef4db349da6aedf6098067801d9a1e978a53.cem.json`](https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/.corvint/changes/fafdef4db349da6aedf6098067801d9a1e978a53.cem.json) (map
SHA-256 `f392f540dce0a8ef1f0fc66791b15cdb38c81100fed250c731f89d3aac5a4abc`, target
`b3a82e86695a58d02d81c5eb0e43c1e030b4554e`); this sentence is its follow-up drift commit.

Rollback: revert `internal/companionrelease/core_stable_result.go` and its test,
`wire.ParseSigned` and its test, `verify.StructuralTupleAdmitted`, `tools/cem-release-suites`, the
`cem-release-suites` Makefile target and these rows. To roll back only the 2026-10-24 integration,
revert its commit: that removes `core_stable_readiness.go`, `READINESS.json`, the Node check, the
`--readiness` flag and the `cem-release-gate` target, and restores the A02 status `OPEN`, the
earlier `statusNote` and the earlier manifest pin of `MIGRATION.json` together. No caller, wire or
sealed case depends on them.

### Release readiness over the stable result traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-569 | `TestCEMV1569StableReleaseReadiness` (`internal/companionrelease`): unknown member, missing member, member order, stale verdict, unknown axis, wrong profile, retired spec, other mode, null specificity under the original and a resealed verdict, scalar reference value; negative specificity accepted. `TestParseSignedAdmitsNegativeIntegers` (`internal/cem/wire`) | The public neutral-criterion ACCEPT line is accepted unchanged |
| CEM-V1-570 | `TestCEMV1569StableReleaseReadiness`: raised native authority and runner execution, experimental and unsupported tuples, a qualified tuple outside the allowlist or without a structural proof, failed structural proof, mismatched or unchecked sidecar, unadmitted envelope, `UNSUPPORTED` outcome, `REJECT` | Admission and authority cases keep `REFERENCE_INTEGRITY_ONLY` |
| CEM-V1-571 | `TestCEMV1569StableReleaseReadiness`: runner `FAILED`, `SKIPPED` and missing rows, platform `NOT_OBSERVED`, external undeclared | Each refusal follows an admitted ACCEPT with reference integrity |
| CEM-V1-572 | `TestCEMV1572ReleaseBuildsMustMatch`, `TestCEMV1572GoVersionIsTheOnlyMaskedValue`, `TestCEMV1572RuntimeConsistent`, `TestCEMV1572ProvenanceIsReportedNotProven`, `TestCEMV1572SuitesReadCommittedObjects`, `TestCEMV1572MaterializeRefusesNonRegularEntries`, `TestCEMV1572EnvelopeSuitesRunInTheCommittedTree` (`tools/cem-release-suites`) | Local `make cem-release-suites` runs recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-cem-release-readiness-a02.md` and `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-cem-release-readiness-review-repair.md` |
| CEM-V1-600 | `TestCEMV1600ReadinessDeclaration` (`internal/companionrelease`): committed declaration 11/2/1 rows; 18 malformed variants, including case aliases and reordering | Node check `readiness.test.mjs` reads the same bytes |
| CEM-V1-601 | `TestCEMV1601ReadinessReadsEveryResultFirst`: ready, no result, later wrong-mode result before outcomes, runner `NOT_RUN`, platform and external unobserved | Every refusal is a `*StableReleaseRefusal` |
| CEM-V1-602 | `TestCEMV1602ObservedRowsNeverPassUnrunRows`, `TestCEMV1602BuildPlatform` (`tools/cem-release-suites`) | Binary-level envelope rows `NOT_RUN`; no external row |
| CEM-V1-603 | `TestCEMV1603JudgeReadiness`, `TestCEMV1603ReadinessPathIsRepositoryRelative` | Committed declaration exits 3 with `release-runner-incomplete` |
| CEM-V1-604 | `TestCEMV1604MigrationA02Integrated` (`internal/companionrelease`); `make cem-stable-regen-check` | Node check recomputes the manifest pin |
| CEM-V1-605 | `make cem-release-gate` run recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-1-0-real-lifecycle.md`; `node --test tools/cem-release-suites/readiness.test.mjs` | Exit 3 refused; not a gate step |

## Runner-bound verdict anchors (V1-2577, 2026-10-24)

Status: proposed (V1-2577); experimental until owner acceptance. The first real cem/1.0
lifecycle (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-1-0-real-lifecycle.md`) refused the sealed verdict
gate over its full touched scope with `tier-below-anchored`: 13 of 20 paths were new files or
committed runner artifacts that no BASE span can name (CEM-V1-081), so they stayed `cited`. The
map did carry evidence for them: passed runner receipts whose plans pin those files' bytes, and
artifact digests the receipts bind. This section makes the gate read that evidence. It is a
gate rule, not a citation rule: `verify.Citation` is the portable cross-consumer block
(TCB-V0-017), so no tier changes and `interop/cem01-go` is unaffected. TCB-V0-011 is amended by
cross-reference to admit these bases.

The requirements are `CEM-V1-606` to `CEM-V1-608` in `## Requirements`.

Non-goals: no tier is raised; no exemption by path, kind or file age; opaque (`opaque/0`)
receipts bind nothing; no working-tree read; no change to the citation, verdict or portable
cross-check. The acceptance criterion's wording "a spec span in the same change" is not used:
the gate has no parsed patch to read added spec bytes from, and the runner binding is the
mechanical evidence the map already pins.

Failure modes: a stale plan digest, a failed or incomplete run, a receipt not bound to its plan,
missing or edited artifact bytes, an unlinked hunk, a deletion and an uncited hunk each refuse
with the named reason; a Git read failure is unverifiable, never satisfied.

Acceptance evidence: the tests below, and the rerun of the gate over #1703's full touched scope
(20 paths), base `0949876e`, candidate `adda562c`, which now GRANTs with verdict
`5058b4ccfe2d6d7b1371e708ba8ed853b69e1c0f4b3dab20356a31ab8da9f605` (18 hunks `runner-input`,
6 `runner-artifact`, 12 `anchored`), recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-verdict-runner-anchors.md`.

Rollback: revert the change; `cem_verdict_runner.go` and the `anchor`/`reason` members go away
and the gate refuses those paths again. Pinned GRANT documents whose hunks were all satisfied by
tier are byte-identical under both versions; a pinned runner-bound GRANT stops reverifying
(TCB-V0-013) after a rollback, which is the intended refusal.

### Runner-bound verdict anchors traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-606 | `TestCEMV1606RunnerInputAnchorsNewAndEditedFiles` (`internal/taskman`): new and edited files GRANT on `runner-input`, tiers equal the citation, pinned bytes reverify | #1703 rerun: 18 `runner-input` hunks |
| CEM-V1-607 | `TestCEMV1607RunnerArtifactAnchorsCommittedRunnerArtifacts`: plan and receipt GRANT on `runner-artifact`; a passing receipt after a failed one for the same plan anchors the plan; a failed run anchors neither | #1703 rerun: 6 `runner-artifact` hunks |
| CEM-V1-608 | `TestCEMV1608UnanchorablePathsRefuseWithANamedReason` (deletion, input absent from the plan, stale input digest, failed run, opaque receipt, foreign plan or receipt profile, unlinked hunk; `all-anchored` refuses), `TestCEMV1608RunnerAnchorReasonsAndReadFailure`, `TestCEMV1608AnchoredGrantEncodesWithoutRunnerMembers` | Earlier tier-only GRANT bytes unchanged |

## Release-gate binary envelopes, run aggregation and readiness binding (V1-2578, V1-2579, V1-2581, 2026-10-24)

Status: proposed (V1-2578, V1-2579, V1-2581); experimental until owner acceptance. The first real
lifecycle (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-1-0-real-lifecycle.md`) left `make cem-release-gate`
refused for three reasons this section addresses: the binary-level repository-envelope rows were
`NOT_RUN`, one run observes one platform so `linux/amd64` stayed `NOT_OBSERVED`, and the release
candidate readiness record did not bind the gate's verdict. SRR-V1-005 is amended by
cross-reference to append the gate row (CEM-V1-614).

The requirements are `CEM-V1-609` to `CEM-V1-614` in `## Requirements`.

Non-goals: no signing or authentication of run records or outcomes; no network transfer between
hosts (the operator copies records); no external-consumer observation, which stays
`NOT_OBSERVED` under V1-0014; no CI wiring (V1-2580 is an owner `.github/` change); no change to
the sealed cases, the stable verifier or the declaration format.

Failure modes: a failing binary envelope run, a foreign or partial report, a record of another
revision, declaration or binary set, a reused binary digest, a repeated or foreign platform, a
non-canonical record or outcome and an existing output path each refuse as named above. A record
is unauthenticated: an operator can fabricate one, so an aggregated `READY` proves only what the
copied records claim; the binary digests let a reviewer rebuild and compare.

Acceptance evidence: the tests below, and the runs at `27e9d9807214029a1535a5135f3401b6c6d45bec`
recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-cem-release-run-aggregation.md`: a darwin/arm64 run and a
WSL2 linux/amd64 run each observed both binary envelope rows `PASSED` (29 of 29), and their
aggregation observed every runner row and both platform rows `PASSED`, refused only at
`independent-external-consumer`.

Rollback: revert the change. The `--record`, `--aggregate` and `--outcome` flags and the gate row
go away; written records and outcomes are evidence only and no reader takes them as ranking,
authority or promotion input. A readiness record carrying `gate/cem-release-gate` stops verifying
after a rollback (its catalogue is not closed), which is the intended refusal; rc.1 and rc.2
records verify under both versions.

### Release-gate aggregation traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-609 | `TestCEMV1609BinaryEnvelopeSuiteNamesTheBinary` (`tools/cem-release-suites`): binary and source-level suites drop inherited selection | darwin and linux runs: 29 of 29 cells per binary |
| CEM-V1-610 | `TestCEMV1610EnvelopeOutcome`, `TestCEMV1610FailingBinaryEnvelopeRunIsRefused` | A stub binary fails every cell and the row is `FAILED` |
| CEM-V1-611 | `TestCEMV1611RunRecordIsCanonical` | darwin and linux records by digest in the build log |
| CEM-V1-612 | `TestCEMV1612AggregationRefusesMismatchedRuns`, `TestCEMV1612BothPlatformRowsFromTwoRuns` | Real aggregation: both platform rows `PASSED`, exit 3 at the external row |
| CEM-V1-613 | `TestCEMV1613GateOutcomeIsExact` (`internal/companionrelease`) | Outcomes of both runs and the aggregation read back |
| CEM-V1-614 | `TestCEMV1614ReadinessBindsTheCEMGateOutcome` (`internal/releasecandidate`) | rc.1 and rc.2 fixtures unchanged (`TestSRRV1005MissingEvidenceIsNotRun` and the other SRR-V1 tests) |

## Omission soundness limits (V1-2599, 2026-10-25)

Status: `CEM-V1-633` proposed (V1-2599), experimental until owner acceptance; `CEM-V1-634` and
`CEM-V1-635` accepted by the owner on 2026-10-25 (V1-2658 sign-off: state the uncertainty, add no
narrower-plan refusal). V1-2599 asked whether a producer
can leave a failure out of a map and still get a clean verdict: (a) the receipt's expected
inventory narrowed to exclude a failing test, (b) a receipt or a FAILED rerun left out, and
(c) a selection narrower than `corvint affected` would choose. Each case was tried on the native
reader, the portable reader and the sealed verdict gate. This section pins what they do. V1-2658
adds one gate document member and no reader or verdict behaviour.

The requirements are `CEM-V1-633` to `CEM-V1-635` in `## Requirements`.

Results: both readers ACCEPT (a), whole omission (b) and (c) with reference integrity only and the
receipt semantics, inventory and adequacy axes `NOT_OBSERVED`, which is the stated limit of a
stable reader. They refuse every partial omission. The gate refuses a recorded failure and still
GRANTs a narrowed plan or a whole omission; since V1-2658 that GRANT states
`runnerSelectionCompleteness` and `sourceInventoryCompleteness` `NOT_OBSERVED` in `uncertainty`.

Non-goals: no change to either reader or the producer; no gate outcome, refusal or reason change;
no refusal of a plan narrower than the package's tests; no reader-side runner admission; no
comparison with `corvint affected` in any consumer; no new reader result axis; no profile bump.
The portable consumer `interop/cem01-go` never reads a gate document and is unchanged. Consumers
of the gate document read only the members they name (the `cem-verdict` policy gate records its
digest as output evidence, `TCB-V0-019`), and no committed testdata pins a gate document.

Failure modes: if a reader starts judging receipt outcome or selection, `CEM-V1-633` fails and
its tests name the case; if a runner-input satisfaction loses the member, the member's value
changes, the narrowed plan starts refusing, or a tier-only or runner-artifact document gains the
member, the `CEM-V1-634` or `CEM-V1-635` test fails. A pinned runner-input GRANT from before
V1-2658 refuses `CEM_VERDICT_PRODUCER_MISMATCH` on reverification (TCB-V0-013); the remedy is to
rerun the gate and pin the recomputed document.

Acceptance evidence: the tests below over `interop/cem01-go/testdata/omission/`; the CLI runs of
`corvint cem verify-stable` (Corvint 1.0.0-rc.3 build 754) and `cem01-go verify-stable-canonical`
over the same maps; and `corvint affected` over the recipe, all recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-cem-omission-soundness.md`; the V1-2658 tests, recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-cem-gate-runner-uncertainty.md`.

Rollback: revert V1-2658 to drop the `uncertainty` member; documents pinned with it then refuse on
reverification, and every other document is unchanged. Delete the three rows, this section, the
tests, the `brokenTest` fixture option and `interop/cem01-go/testdata/omission/` to remove the
rest. No map, sealed CEM or verdict changes.

Completeness profile (V1-2947, 2026-10-28): cases (a) to (c) are decided by the separately
versioned `cem-completeness/1` profile against an independently supplied run inventory
(`docs/specs/cem-completeness-profile-v0.md`, `CCP-V0-*`). It leaves both readers, the gate and
this section's requirements unchanged.

### Omission soundness traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-633 | `TestCEMV1633NativeReaderVerdictIgnoresReceiptOutcomeAndSelection` (`internal/cem/verify`); `TestCEMV1633PortableReaderVerdictIgnoresReceiptOutcomeAndSelection` (`interop/cem01-go`); `TestCEMV1633ProducerKeepsAFailedReceiptOnlyWithoutAdmission` (`internal/cemcandidate`) | `EXPECTED.json` of the omission case: four ACCEPTs and three REJECTs, equal on both readers and on both CLIs |
| CEM-V1-634 | `TestCEMV1634RunnerInputGrantStatesSelectionAndInventoryNotObserved` (`internal/taskman`) | Failing selected test and linked failed rerun refuse `runner-not-passed` with no member; narrowed plan (also the whole-omission map) GRANTs with `uncertainty` both `NOT_OBSERVED`; it reverifies, and the same bytes without the member refuse `CEM_VERDICT_PRODUCER_MISMATCH` |
| CEM-V1-635 | `TestCEMV1635OnlyRunnerInputSatisfactionStatesUncertainty` (`internal/taskman`) | A refusal with a runner-input satisfaction states the member; runner-artifact-only and tier-only GRANTs keep the earlier member set |

## Adopter CEM chain gaps (V1-2798, GitHub #1895, 2026-10-27)

Status: `CEM-V1-636` to `CEM-V1-642` are experimental under the owner's standing lane auto-accept
of 2026-10-11 (V1-2798); the owner decided the ask 4, 6 and 7 forks on 2026-10-27. An adopter running the whole CEM chain on a non-Go repository reported
eight gaps (#1895). This section takes the ones that fit the frozen contracts as additive fields,
exit-status fixes and clearer refusals. The `cem/1.0` wire, the closed verifier result, the
`cem/v2` predicate and every conformance vector are unchanged.

The requirements are `CEM-V1-636` to `CEM-V1-642` in `## Requirements`. Ruby mutation
discrimination (#1895 ask 7) is `TCQ-V0-066`, the language-neutral runner contract is
`TCQ-V0-067..070`, and the same-change test witness (#1895 ask 6) is `TCQ-V0-071`, all in
`docs/specs/test-claim-qualification-v0.md`.

Non-goals:
- no OCM intent outside the target tree as project authority (#1895 ask 4). The OCM wire binds
  intent by target-tree path and blob OID, and no `ocm/0.2` is added; an out-of-tree intent is
  only the advisory `reviewer_overlay` of `CEM-V1-642`. See "Out-of-tree OCM intent: reviewer
  overlay" in `docs/specs/ocm-v0-dogfood.md`.
- no citation of a test the same change adds (#1895 ask 6). Evidence stays anchored at the base
  revision in the wire and both verifiers; no `cem/1.1` and no sidecar. Such a test can witness
  its hunk only by execution (`TCQ-V0-071`); see "Same-change test witness" below.
- no non-Go execution or hunk-revert witness, and no discrimination arm beyond Go and Ruby
  (#1895 ask 7). Both arms follow the accepted runner contract (`TCQ-V0-067..070`); a further
  language needs its own compiled-in runner and planner.
- no change to the `git clone --shared` refusal (#1895 ask 8), already answered by `CEM-V1-323`.
- no `policyState` in the Markdown review body; the envelope carries it.

Failure modes:
- A consumer that keys on `state` alone still sees `accepted` for a policy-refused map; `ok`, the
  exit code and now `policyState` carry the refusal.
- `--allow-absent-sidecar` binds an uncommitted map by digest only. Nothing in the target names
  it, so the caller must keep the map bytes to reproduce the binding.
- A report with ceilings that exits 1 still publishes the review file, as a rejected report does.
- An `external:` subject name cannot be resolved to a file; a consumer must hold the map bytes and
  match them by the digest it already checks.

Acceptance evidence: the tests below, run through the WSL heavy runner on Go 1.27.3 for this
change.

Rollback: revert the V1-2798 change commit. Status and report lose `policyState`, report and
`prove --attest-cem-v2` refuse ceilings again, obligations refuses an absent sidecar
unconditionally, report and `prove --cem` refuse an absolute map, and obligations refuses
`--intent-overlay`. The runner contract and `--same-change-test` roll back as
`TCQ-V0-067..071` say. No stored map, seal, golden or vector
depends on the change except the `cem-status-canonical` golden's added `policyState` member.

### Adopter CEM chain gaps traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-636 | `TestCEMV1636StatusNamesThePolicyVerdict` (`internal/cem/cli`); `TestCEMEndToEndAndGoldenEnvelope` (`cmd/corvint`) | No ceilings: no member; within: exit 0 `within-ceilings`; `--max-mechanical 0`: exit 1, `ok` false, `state` accepted, `ceiling-exceeded`; the status golden carries the member |
| CEM-V1-637 | `TestCEMV1637ReportAppliesCeilings` (`internal/cem/cli`); `TestCoreStableReportRefusals` | JSON within and exceeded, Markdown exceeded: counts, issues and exit match status, `stableVerification` equals the plain report, the review is published; verify and the MCP preview still refuse ceilings |
| CEM-V1-638 | `TestCEMV1638ProveAttestCEMV2PolicyCeilings` (`cmd/corvint`) | Met ceilings: identical statement; an unknown hunk with `--max-unknown 0`: exit 2 `attest-cem-v2-policy-exceeded`, `unknown 1 > 0`, empty stdout; no `--attest-cem-v2`, a malformed value or a repeat: `invalid-arguments` |
| CEM-V1-639 | `TestCEMV1639ObligationsAllowAbsentSidecar` (`cmd/corvint`) | Exact sidecar unchanged by the flag; absent sidecar refuses naming the flag, and with it binds `sidecar: ABSENT` and the map digest; misuse is an argument error |
| CEM-V1-640 | `TestCEMV1640ReportReadsOutOfTreeMap`, `TestCEMV1316OutOfTreeMapRefusals` (`internal/cem/cli`); `TestCEMV1317HelpDocumentsMapPaths` (`cmd/corvint`) | Absolute-map JSON report equals the relative read; a review beside the map is written; `--output` naming the map refuses; the map bytes are unchanged; report help prints the map-path block |
| CEM-V1-642 | `TestCEMV1642ObligationsReviewerOverlay` (`cmd/corvint`) | With the overlay the document gains only `reviewer_overlay` (`reviewer-supplied`, `authoritative` false, two `UNKNOWN` rows); every other member, the `id` and the `unknown` state equal the run without it; relative, in-tree, symlinked, missing and non-intent files and the flag outside stable mode exit 2 with no stdout |
| CEM-V1-641 | `TestCEMV1641ProveCEMSignsAnOutOfTreeMapByContent`, `TestCEMV1317HelpDocumentsMapPaths` (`cmd/corvint`) | The in-tree `--attest-cem-v2` stdout keeps its pinned SHA-256; the same map in two temporary directories signs identical bytes whose subjects are `external:<sha256>` and contain no host path; the verifier over the absolute map reads VERIFIED; an in-repository absolute path, a symlink and a missing map exit 2 with no stdout; help names the rule |

### Same-change test witness (GitHub #1895 ask 6)

Status: decided by the owner on 2026-10-27 (V1-2798) as option 3 below and built as `TCQ-V0-071`
in `docs/specs/test-claim-qualification-v0.md`, experimental under the standing lane auto-accept
of 2026-10-11. Design entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-cem-adopter-chain-design.md`.

Problem: a hunk whose guarding test is added by the same change cannot cite that test. `cem/1.0`
evidence is `{id, blobOid, path, span, spanSha256}` anchored at `baseRevision`, its keys are
closed, and both verifiers resolve every evidence blob at the base. A spec file that does not exist
at the base is `missing-evidence`. Adding a member such as `anchor: "target"` to an evidence entry
is a closed-key violation that every frozen `cem/1.0` verifier refuses, and changing what the
existing members mean would silently widen the frozen contract, so no additive `cem/1.0` route
exists.

Options considered:
1. Rejected: a `cem/1.1` minor revision whose evidence entries may carry `anchor: "target"`. It
   needs new conformance vectors, a reader refusal and a freeze amendment.
2. Rejected: an experimental sidecar beside the map listing target-side test claims. A reviewer
   would have to trust a second file.
3. Chosen: no wire change. A same-change test is checked only by execution:
   `cem discriminate --same-change-test HUNK=PATH` runs it against its hunk, and the hunk is
   `discriminates` only when the observed run passes the test at the target and a mutant of the
   hunk fails it. The witness keeps the frozen `TCQ-V0-056` member, `selectionSha256` covers the
   test, and the `detail` names it as `same-change test (execution only)`. A test that never ran
   stays `not-run`. A `cem cover` witness (`TCQ-V0-065`) remains the coverage-only route.

A target-side test never satisfies a base-anchored rule, is never cited, and changes no basis,
disposition, support tier, count or exit status, so a same-change witness never reads as stronger
evidence than a base-anchored one.

## Recorded intent-scope deferral (V1-2809, 2026-10-27)

Status: `CEM-V1-643` to `CEM-V1-646` are experimental under the owner's standing lane auto-accept
of 2026-10-11 (V1-2809). A dogfood change that deferred an intent scope recorded it only in private
side files: the dogfood report, the intents manifest, OCM status and the map's side files. A
reviewer sees only the sealed CEM, so the deferral was neither durable nor visible.

Mechanism: the deferral is a tracked, content-named file the change creates
(`CEM-V1-643`), not a map member. The frozen `cem/1.0` map already binds every changed file
except the map itself through `patchSha256` and the hunks, so the seal covers the record and the
PR diff shows it. `cem defer` writes it (`CEM-V1-644`), `cem status` and `cem report` check it
against the base and target trees and a bound (`CEM-V1-645`), and the review names it
(`CEM-V1-646`). The `cem/1.0` wire, its canonical encoding, the closed verifier result, the
`cem/v2` predicate and every conformance vector are unchanged.

Non-goals:
- no `deferredIntents` member in the map, the verifier result or any frozen shape;
- no check that a deferred path is not also a kept intent, and no `deferred<TAB>PATH<TAB>REASON`
  plan row: both belong to the dogfood flow (`internal/dogfoodflow`), which reads the record
  through the exported `workflow` encoder and parser;
- no change to `state`, `policyState`, `stableVerification` or the record-set digest.

Failure modes:
- A record the map's hunks do not name is invisible to status: a map that omits a changed file
  is already refused by the verifier, so an accepted map names every record the change creates.
- The record binds the intent's obligation count, not its requirement IDs; an intent edited to
  keep its count reads `VERIFIED`. The base blob OID still binds the base content.
- A host without the counter (any binary other than `corvint`) refuses `cem defer` and reports
  `countState` `NOT_CHECKED`.
- A consumer that keys on `state` alone still reads `accepted` for a refused deferral; `ok`, the
  exit code and the member carry the refusal.

Acceptance evidence: the tests below, run through the WSL heavy runner on Go 1.27.3.

Rollback: revert the V1-2809 change commit. `cem defer` and `--max-deferred-intents` disappear,
status and report lose the member and section, and the DR-0040 candidate returns to twelve actions.
A committed record then reads as an ordinary changed file; no map, seal, golden or vector depends
on the change.

### Recorded intent-scope deferral traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-643 | `TestCEMV1643DeferredIntentsRecordEncoding` (`internal/cem/workflow`) | Exact canonical bytes, round trip and content name; pretty, unterminated, unknown-member, missing-member, wrong-profile and empty records, and unsorted, duplicate, five-entry, multi-line or over-long reason, zero-count, reserved-path and unclean-path entries refuse |
| CEM-V1-644 | `TestCEMV1644To646DeferRecordsAndStatusChecks` (`internal/cem/workflow`); `TestAnchorActionsParseLikeTheirSiblings` (`internal/cem/cli`); `TestCEMV1644CounterAdmitsAnIntentOverTheOCMBound` (`cmd/corvint`) | `defer` publishes the content-named record, replaces it on a second call, and refuses with no counter; the CLI parses `defer` and its required flags; the installed counter counts 257 obligations, one over the OCM bound, and still refuses a non-intent |
| CEM-V1-645 | `TestCEMV1644To646DeferRecordsAndStatusChecks`, `TestCEMV1645StatusRefusesMisnamedAndMismatchedRecords` (`internal/cem/workflow`) | Default bound exits 1 `exceed-bound`; `--max-deferred-intents 1` is `within-bound`; no counter is `NOT_CHECKED`; a miscounted record, a misnamed record and a wrong base blob refuse; verify refuses the flag; a change without a record has no member |
| CEM-V1-646 | `TestCEMV1644To646DeferRecordsAndStatusChecks` (`internal/cem/workflow`) | JSON `review.deferredIntents` within bound; `recordSetSha256` excludes the member; the preview Markdown renders the section, the entry, the reason and the bound issue |

## Mutator out-of-tree maps and paired changes (GitHub #1942, #1941)

Status: `CEM-V1-647` and `CEM-V1-648` are experimental under the standing lane auto-accept of
2026-10-11. Build entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-cem-witness-paired-range.md`.

The case for `CEM-V1-647` is that `cem discriminate` refused an absolute `--map` that `cem status`, `cem report` and
`prove --cem` accept. The mutators now share the read rules, but write only where the caller says
and only inside the repository: Core publishes a map under the repository's lock, so an in-place
write outside the repository has no lock to take.

The case for `CEM-V1-648` is that a change whose application and tests live in two repositories has no `cem` verb. The
frozen `cem/1.0` map binds one repository through `expectedBase`, `target`, `patchSha256` and its
hunks, and the verifier resolves every citation in that one repository; a cross-range citation needs
a new wire version. This slice refuses the paired flags with the reason. What remains: a paired map
profile naming both pins, citation into the tests range at its head, a verifier that resolves the
second repository from an explicit root, and a `cem/v2` predicate binding both pins. Until that
profile lands, `prove --cem` refuses the paired flags with the same reason (V1-2936); the design
the profile is expected to take is recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-paired-range-prove-refusal.md`, and it stays unbuilt.

Non-goals:
- no in-place write of an out-of-tree map, no absolute `--output`, and no change to the producers
  `begin` and `prepare`, which write the repository's own map;
- no paired map, no change to the `cem/1.0` wire, the verifier result, `cem/v2` or any vector.

Failure modes:
- An out-of-tree map that is symlinked, missing, oversized or inside the worktree or a Git
  directory is refused by the `CEM-V1-316` checks before any write.
- A caller that relied on the unrecognized-arguments text for `--range` now reads the paired
  refusal; the code stays `invalid-arguments`.

Rollback: revert the change commits. The mutators return to repository-relative maps only, and the
paired flags return to the generic unrecognized-arguments refusal; no map, seal, golden or vector
depends on either change.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-647 | `TestCEMV1647MutatorsReadAnOutOfTreeMapIntoARelativeOutput` (`internal/cem/workflow`); `TestCEMV1316OutOfTreeMapRefusals` (`internal/cem/cli`); `TestCEMV1317HelpDocumentsMapPaths` (`cmd/corvint`) | mark over an absolute out-of-tree map writes the relative output; in-place mark and discriminate refuse naming `--output`; an in-repository absolute path refuses; the external bytes are unchanged; help names the rule |
| CEM-V1-648 | `TestCEMV1648CEMRefusesAPairedChange` (`internal/cem/cli`); `TestCEMV1648ProveCEMRefusesAPairedChange` and `TestCEMV1317HelpDocumentsMapPaths` (`cmd/corvint`) | prepare, begin, status and report refuse each paired flag in both forms with the reason; `prove --cem` (with and without `--attest-cem-v2`) and `prove --verify-cem-attestation --cem` give the same text as `cem prepare`; `TestCEMV1648MapReadingSurfacesRefuseAPairedChange` shows `verify-stable` (fix line beside its `invalid-arguments` result), `bundle` (refusal error) and `gate-verdict` (after its code) give it too; a prefix-sharing flag stays unrecognized; help names the refusal |

## Non-regular paths in a prepared range (GitHub #1950)

Status: `CEM-V1-655` is experimental under the standing lane auto-accept of 2026-10-11 (V1-2964).
Build entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-cem-witness-edges.md`.

`CEM-V1-655` is defined in the Requirements section above.

The case is that a range adding `web/escape -> ../../../outside` refused with only `tree mode
"120000"`. Mapping the rest of the range needs the frozen `cem/1.0` patch grammar to admit mode
`120000`, or an unknown-hunk member: the `protocol/cem-1.0` packet admits only regular files, and its
`near-miss-mode-symlink` vector pins `unsupported-tree-mode` for both consumers. Either is a
protocol change for the owner, so this slice names the paths and leaves the range refused.

Non-goals: admitting mode `120000` or `160000` hunks; excluding the paths from the canonical
patch, which would make the native and portable consumers derive different patches; any wire,
vector or verifier change.

Failure modes: a refusal whose change set cannot be listed keeps the parser's message; the range
still produces no map until it is split around the link.

Rollback: drop `treeModeRefusal` and `UnsupportedModeChanges`; the parser's refusal is unchanged.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-655 | `TestPrepareNamesEachNonRegularPath` (`internal/cem/workflow`) | an added escaping link refuses `unsupported-tree-mode` naming `web/escape (base absent, target 120000)` and not its text; a link unchanged since the base maps |

## Verifier span and drift bounds, hunk-revert isolation (V1-2989, V1-2996, V1-2991, V1-2995, 2026-10-28)

Status: `CEM-V1-690..693` are experimental under the owner's standing lane auto-accept of
2026-10-11. Build entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-cem-verifier-bounds-and-revert-isolation.md`.
None of them changes the `cem/1.0` wire, a verdict digest, a public case or a pinned operation
ledger: both verifiers make exactly the same Git reads as before.

The requirements are `CEM-V1-690` to `CEM-V1-693` in the Requirements section: bounded span
retention, drift that stops at the second match, a fresh snapshot per hunk-revert run, and
`survived` only on a terminal pass for every eligible test.

Non-goals: no new wire member, reason, profile version, runner or verifier result field; no change
to `CEM-V1-089`'s closed reasons, to which runs execute, or to the portable consumer's drift.

Rollback: revert the V1-2989/V1-2996/V1-2991/V1-2995 code commit. Spans return to per-record
copies, drift to an uncapped count, runs to the shared mutated export, and `survived` to any pass.
No map, seal, golden, vector or ledger depends on the change.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-690 | `TestCEMV1690OverlappingSpansShareOneBlob` (`internal/cem/verify`); `TestCEMV1690RetainedSpansShareOneBlob` (`interop/cem01-go`) | 64 overlapping near-full spans of a 4 MiB blob are accepted and each is a clipped window at its offset into one blob copy, in both verifiers; the portable conformance ledgers (`TestCEMV1260DriftResourceLimitPortable`) are unchanged |
| CEM-V1-691 | `TestCEMV1691DriftStopsAtTheSecondMatch`, `TestClassifySpanCountsAMatchEndingAtTheEnd` (`internal/cem/verify`) | A 1 MiB repeated-byte needle in a 4 MiB repeated-byte target is ambiguous and counted exactly (3 MiB + 1) in under 2 s; a cancelled context errors; small overlapping counts are exact |
| CEM-V1-692 | `TestCEMV1692EachRunGetsAFreshSnapshot`, `TestCEMV1088JudgeRevertsOnlyInsideTheCopy` (`internal/cem/verify`) | A sentinel-writing test leaves an unrelated hunk `survived`, runs see two distinct snapshots and never the export, the export never sees the sentinel, and no run directory remains |
| CEM-V1-693 | `TestCEMV1089ClassifyRevert`, `TestCEMV1693SurvivedNeedsEveryEligiblePass` (`internal/cem/verify`) | A skipped eligible test is `skipped`, a missing one `test-not-observed`, a partial pass in a failed package `test-not-observed`; a row survives only when every package with eligible tests survived |

### Guarded one-hunk citation replacement (V1-3168)

Status: `CEM-V1-694` and `CEM-V1-695` are experimental under the owner's standing lane
auto-accept of 2026-10-11. Build entry `docs/build-log/2026-10-30-guarded-citation-replacement.md`.
`cem cite` only ever added evidence, so correcting one cited hunk forced deleting the whole map;
the guarded replacement changes that one hunk and nothing else. It adds no `cem/1.0` wire member,
verdict, reason or profile: a replaced map is an ordinary producer map, and every verifier reads it
as before. `dogfood change` drives it from a plan (`DCW-V0-390..392`).

Non-goals: no unguarded replacement, no removal without a replacing citation (use the guarded
`cem mark`), no multi-hunk transaction in Core (the dogfood flow stages its rows and publishes the
tracked map once), and no digest in the cite or mark result envelope.

Rollback: revert the V1-3168 code commit. `cem cite` and `cem mark` lose the two options and a
correction again needs a fresh map; no map, seal, golden, vector or ledger depends on the change.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-694 | `TestCiteReplaceCEMV1694` (`internal/cem/workflow`); `TestCEMV1694ReplaceFlagsAreGuarded` (`internal/cem/cli`) | Positive first: hunk 1's two citations are replaced by one, hunk 2's digest is unchanged, and the two dropped spans are pruned while the shared span stays; then a stale old digest, a stale base, a stale patch, an out-of-range line span, evidence absent at the base and a failed publication each refuse with their code and leave the map's bytes identical; the flag pairing, digest shape and paired refusal are checked at the command line |
| CEM-V1-695 | `TestCiteReplaceCEMV1694` (`internal/cem/workflow`); `TestCEMV1694ReplaceFlagsAreGuarded` (`internal/cem/cli`) | A stale-digest mark refuses with the map unchanged; the current digest marks hunk 1 unknown and leaves hunk 2's digest unchanged |

## Binary file groups bound by blob identity (V1-2752, 2026-10-31)

Status: `CEM-V1-980..986` are experimental under the owner's standing lane auto-accept of
2026-10-11. Build entry `docs/build-log/2026-10-31-cembin-w1-bugs.md`. They amend `CEM-V1-037`,
which until now refused every change that committed a binary file. Such a change could not be
prepared or dogfood-bound at all. One binary test fixture in a batch left the whole batch
`NOT_PRODUCED` (V1-2752, observed on beamfall/corvint#1868).

The requirements are `CEM-V1-980` to `CEM-V1-986` in the Requirements section. They cover the
binary group grammar, binding through `patchSha256` with no wire change, base simulation by object
ID, the prepare `binary` rows, the cite refusal, the existing bounds, and the recorded portable
divergence.

Design: Git's canonical patch already carries each binary file as a section that names the path,
modes and full object IDs, and the `CEM-V1-110` proof already pins that section to the verified
trees. So the native reader accepts that one exact form as a hunk-less group. A new profile flag
would have had to reach every dogfood, OCM, cite, status and verify call. The file's bytes are
never read as text, so the change is bound by identity and explicitly excluded from textual
evidence, not reviewed.

Non-goals:

- no `GIT binary patch` (`--binary`) body;
- no binary rename, mode change or similarity metadata;
- no binary hunk, hunk ID, disposition or wire member;
- no evidence span inside a binary file;
- no change to the frozen packet or the portable consumer;
- no binary-only map: a map still needs a textual hunk.

Failure modes:

- the portable consumer rejects a map that carries a binary section, which fails closed;
- a reviewer sees only the path, object IDs and sizes, never the content, so the file's adequacy
  stays unknown;
- a binary file over a blob bound refuses the whole prepare, as before.

Rollback: revert the V1-2752 implementation commit. Every binary section again refuses
`binary-patch` naming its path (`CEM-V1-037`). A map that was prepared with a binary group then no
longer verifies natively and has to be re-prepared without that change. No committed map, seal,
golden, vector or ledger depends on this change.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-980 | `TestCEMV1980BinaryGroupGrammar`, `TestBinaryRefusalNamesPath` (`internal/cem/patch`) | Canonical creation, modification and deletion sections, with SHA-1 and SHA-256 IDs, parse as hunk-less binary groups beside a textual group whose hunk ID is unchanged; abbreviated IDs, mismatched lengths, a nonzero creation ID, a mode suffix on a creation, a missing index, a disagreeing marker, rename metadata and `GIT binary patch` refuse `binary-patch` naming the path; empty-valued `rename from `/`rename to ` lines, similarity and copy metadata refuse |
| CEM-V1-981 | `TestCEMV1980BinaryGroupGrammar` (`internal/cem/patch`), `TestPrepareBinaryRefusalNamesPath` (`internal/cem/workflow`) | A binary-only patch refuses `binary-patch` naming the path; real binary-only creation, modification and deletion commits refuse `cem prepare` naming the path |
| CEM-V1-982 | `TestCEMV1982BinaryGroupBaseSimulation` (`internal/cem/sim`) | A matching SHA-1 or SHA-256 base passes; a changed base byte, an absent base, a present creation destination and a disagreeing mode refuse with the named codes |
| CEM-V1-983 | `TestCEMV1983PrepareBindsBinaryFixture` (`internal/cem/workflow`) | A real commit with one binary fixture and a text change prepares; the result's `binary` rows name the path, change, both blob IDs, both sizes and `excluded`; the map has only the textual hunk and native `verify` reads it without a parser refusal; a text-only prepare has no `binary` member |
| CEM-V1-984 | `TestCEMV1984CiteRefusesBinaryPath` (`internal/cem/workflow`) | A cite into the changed binary path refuses `cite-span-not-stable` naming it, while a cite into the text file succeeds |
| CEM-V1-985 | `TestCEMV1985OverBoundBinaryRefuses` (`internal/cem/workflow`), `TestCEMV1980BinaryGroupGrammar` (`internal/cem/patch`) | A binary creation one byte over `gitauth.MaxBlobBytes` beside a text change refuses `cem prepare` with `repository-object-unavailable` naming the path and writes no map; a map begun over the same canonical patch is refused with that code by the candidate and stable verifiers; the parser's oversize and NUL refusals are unchanged |
| CEM-V1-986 | `TestCEMV1934PortableBinarySectionGrammar` (`interop/cem01-go`) | Superseded for the stable routes by `CEM-V1-934..936`; a binary-only patch, `GIT binary patch` data and every non-canonical binary shape keep the `patch` refusal |

## Mode change beside textual hunks (V1-3404, 2026-10-31)

Status: `CEM-V1-930..933` are experimental under the owner's standing lane auto-accept of
2026-10-11. Build entry `docs/build-log/2026-10-31-cmdcem-mac.md`. Until now the native parser
refused every `old mode`/`new mode` line with `malformed-patch`. A commit that ran `chmod +x` on a
tracked script and edited it could not be prepared or dogfood-bound (V1-3404, observed by lane
linux-w2).

Design: the canonical-diff proof already renders `old mode`/`new mode` from the verified trees, so
the target mode is bound through `patchSha256`. The parser records both modes, and base simulation
checks the base side. This follows the `CEM-V1-980` precedent: native-only, with the portable
divergence failing closed.

Non-goals: no mode-only map (`CEM-V1-300` stands), no binary or symlink mode change, no wire
member, and no change to the frozen packet or the portable consumer.

Failure modes: the portable consumer refuses such a map with `unsupported-tree-mode`, which fails
closed. A base whose mode differs from `old mode` refuses `diff-metadata-mismatch`.

Rollback: revert the V1-3404 implementation commit. Every mode change again refuses
`malformed-patch`, and a map prepared over one must be re-prepared without that change. No
committed map, seal, golden or ledger depends on it.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-930 | `TestCEMV1930ModeChangeGrammar` (`internal/cem/patch`) | A modification and a rename with a mode change parse and record both modes; a creation, a deletion, an index mode suffix, a lone, repeated or equal pair, and a non-regular mode refuse with the named codes |
| CEM-V1-931 | `TestCEMV1931ModeChangeBaseSimulation` (`internal/cem/sim`) | A base in `old mode` passes; a base in the other mode refuses `diff-metadata-mismatch` |
| CEM-V1-932 | `TestMetadataOnlyChangeRefusalVectors` (`internal/cem/patch` and `interop/cem01-go`) | `mode-and-text-change` parses natively and still refuses `unsupported-tree-mode` portably |
| CEM-V1-933 | `TestCEMV1933PrepareBindsModeChange` (`internal/cem/workflow`) | A real commit that runs `chmod +x` on a tracked script and edits one line prepares, and the native stable verifier accepts the map |

## Portable binary section and stable packet r2 (V1-3443, 2026-10-31)

Status: `CEM-V1-934..936` are experimental under the owner's standing lane auto-accept of
2026-10-11. Build entry `docs/build-log/2026-10-31-cmdcem-mac.md`. `CEM-V1-986` left the portable
consumer refusing every binary marker, so a map that the native reader accepts over a binary file
change failed portably.

Design: the portable parser admits only the exact section that the native parser admits
(`CEM-V1-980`). A binary group carries no hunk and is checked against the verified trees by mode and
object ID. Both stable routes derive the canonical patch from Git and compare its digest, so the
section's object IDs are bound through `patchSha256`. The packet keeps one directory: its r1 bytes
are preserved under `revisions/r1/` before items 6 and 7 change, as its README requires.

Non-goals: no binary content diff or citation of binary bytes, no binary section on the portable
candidate route, no `old mode`/`new mode` support in the portable consumer (`CEM-V1-932` stands),
and no wire or schema change.

Failure modes: a non-canonical binary shape, a binary-only patch and `GIT binary patch` data keep the
`patch` refusal. A tree entry that disagrees with the section refuses with the `CEM-V1-935` codes.

Rollback: revert the V1-3443 implementation commit. The portable consumer again refuses every binary
marker, and `protocol/cem-1.0/stable/` returns to the r1 bytes preserved in `revisions/r1/`. No
committed map, seal, golden or ledger depends on the revision.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-934 | `TestCEMV1934PortableBinarySectionGrammar` (`interop/cem01-go`) | A binary creation, modification and deletion (SHA-1 and SHA-256 lengths) beside a textual hunk parse; a binary-only patch, abbreviated or mixed-length IDs, a missing or misplaced mode suffix, equal or zero IDs on a modification, a wrong label, rename or mode-pair metadata, a trailing record and `GIT binary patch` data refuse `patch` |
| CEM-V1-935 | `TestCEMV1935PortableBinaryEntries` (`interop/cem01-go`) | Matching entries pass; a symlink, a mode mismatch and an object ID mismatch on either side refuse with the named codes |
| CEM-V1-936 | `TestCEMV1936PortableReaderAgreesOnBinarySectionVector` (`interop/cem01-go`), `TestCEMV1936NativeReaderAgreesOnBinarySectionVector` (`internal/cem/verify`) | The shared case ACCEPTs at `complete` with patch `f551fd0a…` on the native reader and on both portable stable routes, maps only `text.txt`, and REJECTs once the digest is altered |

## Portable route envelopes over the committed corpus (V1-3756, 2026-11-01)

Status: experimental under the owner's standing lane auto-accept (2026-10-11); V1-3756.

Decision: the 2026-11-01 CEM format audit read the legacy `cem01-go verify-stable` route's
4096-entry refusal as the independent consumer refusing every real CEM. The repository-scale
portable route is `verify-stable-canonical` (`CEM-V1-352`), which has no such bound, so the fork
taken is to measure that route over the whole committed corpus and state each route's envelope,
not to raise the legacy bound.

Requirements: `CEM-V1-961` (the legacy bounds bind the legacy route only; canonical verifies past them) and
`CEM-V1-962` (the legacy repository profile), both in the Requirements section.

Non-goals: no change to either route's bounds, codes or results; no claim about maps outside
`.corvint/changes/`; no ancestry repair of the maps whose objects a clone lacks (V1-3758).

Failure modes:
- A clone that lacks a map's base or target object answers `UNSUPPORTED` /
  `repository-object-unavailable` on both consumers. That is missing evidence, reported as such,
  never a REJECT.
- The corpus measurement is a dated observation, not a gate: a later map is covered only by a
  rerun.

Acceptance evidence: `TestCEMV1961CanonicalRouteHasNoLegacyTreeOrConfigBound`
(`interop/cem01-go`) rebuilds the public `neutral-criterion` case with 5000 extra unchanged
entries and an ordinary remote/branch config: the canonical route ACCEPTs, the legacy snapshot of
the same target refuses `unsupported-resource-limit`, and the legacy repository check refuses
`unsupported-repository-envelope`. Corpus measurement (2026-11-01, base
`8daec98ce4349c1583186692726d2c3a782ac647`, Linux amd64): for all 111 maps under
`.corvint/changes/`, `corvint cem verify-stable` and `cem01-go verify-stable-canonical` gave the
same outcome, stage, code and `verdictSha256` in a linked worktree (108 ACCEPT, 1 REJECT
`binding` for `8668f77b`, 2 UNSUPPORTED `repository-object-unavailable`) and in an ordinary
`git clone --no-local` checkout (104 ACCEPT, 1 REJECT, 6 UNSUPPORTED
`repository-object-unavailable`, the 4 extra being objects only local refs retain). Each
canonical run took under one second; see `docs/build-log/2026-11-01-searchcem-w8.md`.

Rollback: revert the change commit; it adds a test, a test helper parameter and this record only.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-961 | `TestCEMV1961CanonicalRouteHasNoLegacyTreeOrConfigBound` (`interop/cem01-go`) | canonical ACCEPT over 5000+ entries and an ordinary config; 111/111 corpus parity |
| CEM-V1-962 | `TestCEMV1961CanonicalRouteHasNoLegacyTreeOrConfigBound` (`interop/cem01-go`); `candidateRepository` and `repository03` in `interop/cem01-go/cem.go` | ordinary config refused `unsupported-repository-envelope` on the legacy route |

## Committed map base retention (V1-3758, 2026-11-01)

Status: experimental under the owner's standing lane auto-accept (2026-10-11); V1-3758; decision
0588.

Requirement: `CEM-V1-963` (every committed map base retained or labelled), in the Requirements section.

Non-goals: no rewrite of a sealed map, no new ref, no verification of the archive repository.

Failure modes: a shallow or remote-less clone skips the check, saying so; a label cannot make an
absent object verify, so a `base-unavailable` map stays `UNSUPPORTED`.

Acceptance evidence: `script/check-cem-base-retained_test.sh` (retained base passes; a base only a
deleted branch held and a missing base are refused with the fix; a label admits exactly its base;
stale, mismatched and unknown-label rows are refused; no retained ref skips). The committed corpus
passes with 111 maps and 17 labelled rows (decision 0588).

Rollback: delete the register, the scripts and their Makefile targets; no map changes.

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-963 | `script/check-cem-base-retained_test.sh` (`make cem-base-retention-test`) | `make cem-base-retention-check` over the committed corpus |
