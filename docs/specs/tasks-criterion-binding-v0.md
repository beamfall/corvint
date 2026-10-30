# Tasks criterion binding V0

Owner: Russell Lewis
Date: 2026-10-01
Intent status: proposed; TCB-V0-015..023 accepted under the owner's standing auto-accept (2026-10-11) (V1-1283, V1-1356); TCB-V0-030..031 accepted under it (V1-1358); TCB-V0-035..039 accepted under it (V1-1359); TCB-V0-040..044 accepted under it (V1-1377); TCB-V0-045 accepted under it (V1-1761); TCB-V0-047 accepted (decision 0548, reaffirmed 2026-10-24; V1-1895)
Delivery status: experimental
Authoritative inputs: decision 0397, `docs/specs/cem-criterion-experiments-v0.md`, native V1-0575;
for TCB-V0-007..023, decision 0525 §1 step 8, `docs/specs/cem-stable-v1.md` and native V1-1283 and V1-1356.

## Agent digest
- Claim: Native Tasks captures and verifies coherent claim-era criterion bindings; Core consumes only the Tasks wire contract.
- Status: proposed; TCB-V0-015..023 accepted under the owner's standing auto-accept (2026-10-11) (V1-1283, V1-1356); TCB-V0-030..031 accepted under it (V1-1358); TCB-V0-035..039 accepted under it (V1-1359); TCB-V0-040..044 accepted under it (V1-1377); TCB-V0-045 accepted under it (V1-1761); TCB-V0-047 accepted (decision 0548, reaffirmed 2026-10-24; V1-1895)/experimental delivery; owner release intent remains accepted separately.
- Exists: `internal/tasks/criterionbinding`, compiled CLI read verbs and wire-owned projections; bounded independent repair review passed. Experimental sealed cem/1.0 verdict gate: library `internal/taskman/cem_verdict_gate.go` with a portable cross-check, `corvint cem gate-verdict`, Tasks argv placeholders and a policy entry (TCB-V0-007..019). A merged-forward lane is gated and completed at its seal commit, and an exact touch key naming a directory refuses (TCB-V0-020..023); the portable leg runs on Darwin and Linux amd64/arm64 (TCB-V0-030..031). An optional `effects.verdictPaths`, a subset of `touchPaths`, is the gate's predicate domain through `{claim.verdictPaths}` in repository policy v10, falling back to `touchPaths` (TCB-V0-035..039). The document pins the consumer's stage and code, an unsupported portable route refuses `portable-route-unsupported` by name, and the consumer resolves the gate's own Git, which fixed the hosted macos-15 gate tests (TCB-V0-040..044).
- Blocked on: full CEM1.0 live receipt/source join, integration and release qualification; installing the gate's two executables and registering its policy entry in the live store; installing a `corvint-tasks` with TCB-V0-037 before the store adopts policy v10.
- Read next: Requirements; Acceptance and rollback; Sealed cem/1.0 verdict gate; Merged-forward lanes; Verdict predicate domain.

## Requirements

- `TCB-V0-001`: Core MUST import only Tasks wire. Full ticket/attempt/policy validation stays Tasks-side;
  existing TestImportDirection remains unchanged. No copied partial decoder or whitelist may substitute.
- `TCB-V0-002`: `criterion-binding capture --ticket ID --attempt ID` MUST be a bounded read with no
  repository/trace/Tasks mutation. Exact canonical ticket, queue, attempt and policy bytes MUST share
  audited head sequence/receipt, intent tree and primary identity, with no redo/barriers.
- `TCB-V0-003`: `criterion-binding verify` MUST consume a closed bounded capture on stdin with no
  repository resolution or mutation. Embedded native command identities, canonical records, coherent
  snapshots, acceptance/attempt/runtime/lease and raw/content policy digests MUST be revalidated.
- `TCB-V0-004`: Verify outer current snapshot MUST be null; carried snapshot MUST be explicitly
  HISTORICAL. Capture producer and current verifier identities MUST be distinct. Version/build labels
  cannot substitute for executable identity. All input bytes and results remain digest-bound.
- `TCB-V0-005`: The CEM companion MUST use only independently configured, pinned Tasks executable
  capture/verify calls. Plans bind executable digest and reported identities, never supplied executable
  path or argv. Historical verification runs only that trusted read verifier, never an experiment/test
  or plan-supplied command. Unsupported installed Tasks versions refuse visibly.
- `TCB-V0-006`: Unknown/repeated arguments, malformed/trailing/oversize payloads, nonregular executable,
  contradictory captures and missing authority MUST refuse. Calls cap stdout at 4 MiB, stderr at
  64 KiB and 30 seconds; cancellation/overflow retire the owned verifier process group. Admission
  and post-call checks bind the same executable. Same-user hostile mutation is not authenticated.
- `TCB-V0-007`: The sealed-verdict gate (`taskman-cem-verdict-gate/0`) MUST take only native facts:
  an absolute clean repository root, the attempt's full native base and candidate commit IDs (equal-length
  lowercase 40 or 64 hex), the claim-era `effects.touchPaths` keys and one predicate. Malformed input
  refuses `CEM_VERDICT_INPUT_INVALID` with no pinned inputs; an absent commit refuses it pinned.
- `TCB-V0-008`: The gate MUST read the sealed map from Git objects at the candidate commit, never from
  the worktree, an attachment or a Tasks store: exactly one `.corvint/changes/<target>.cem.json` added
  between base and candidate. None refuses `CEM_VERDICT_MAP_MISSING`; several refuse
  `CEM_VERDICT_MAP_AMBIGUOUS` and pin every sealed name.
- `TCB-V0-009`: Producer-written fields MUST agree with native recomputation or refuse
  `CEM_VERDICT_PRODUCER_MISMATCH`: the sealed name's target is a commit; the candidate tree is that
  target's tree with only `.corvint/change.cem.json` renamed byte for byte to the sealed path; the
  map's `baseRevision` equals the attempt base.
- `TCB-V0-010`: The gate MUST recompute the verdict in process through the stable consumer with the
  repository and artifact root at the root, the attempt base expected and the sealed target, and refuse
  `CEM_VERDICT_UNVERIFIABLE` unless the outcome is ACCEPT, the sidecar is EXACT and a citation block
  exists. An unreadable repository, Git read failures and hosts other than Darwin and Linux refuse the
  same code. Declared artifacts are read under the root and must match digests the sealed map pins.
- `TCB-V0-011`: Predicates are closed: `touched-anchored` governs hunks under a touch key (a `dir/` key
  is a prefix, any other key an exact path) and `all-anchored` governs every hunk. Tiers come only from
  the recomputed citation; `anchored`, `mechanical-proved` and `held` satisfy and nothing raises a tier.
  No governed hunk, or one below anchored, refuses `CEM_VERDICT_PREDICATE_FAILED`. Amended
  2026-10-24 (proposed; V1-2577): a `cited` hunk is also satisfied on the runner-bound bases of
  `CEM-V1-606` and `CEM-V1-607` in `docs/specs/cem-stable-v1.md`; its tier is still reported
  unchanged, and an unsatisfied hunk names a closed reason (`CEM-V1-608`).
- `TCB-V0-012`: Every outcome MUST emit one canonical, deterministic, host-path-free document pinning
  the inputs, candidate tree, sealed names, map path/target/blob/`mapSha256`, consumer
  outcome/stage/code/issues/sidecar/`verdictSha256`, the recomputed citation and per-hunk governed rows.
  Only a fully satisfied predicate GRANTs.
- `TCB-V0-013`: Re-verification MUST recompute from native inputs and GRANT only when the recomputed
  document equals the pinned bytes exactly; other pinned bytes refuse `CEM_VERDICT_PRODUCER_MISMATCH`.
  Any consumer can recompute from the document's own inputs with no Tasks store present.
- `TCB-V0-014`: The gate MUST NOT mutate the repository, refs, objects, traces or a Tasks store. A GRANT
  is gate evidence only: it never completes, satisfies or edits a ticket, attempt, policy or tier, and
  native completion authority and TCB-V0-001..006 stay unchanged.
- `TCB-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) A Tasks COMMAND
  gate MUST receive attempt facts only through three closed argv placeholders, replaced just before the
  gate runs when an argv entry equals one exactly: `{attempt.baseCommit}` is the attempt base,
  `{attempt.candidateCommit}` the worktree HEAD commit whose tree was just proven to be the candidate,
  and `{claim.touchPaths}` the claim-era ticket's `effects.touchPaths` as one canonical JSON string
  array. A ticket record whose digest differs from the claim's refuses `STALE_TICKET`; a
  multi-repository candidate refuses `UNSUPPORTED`. A substituted argv joins the result's inputs
  digest. Other entries, preflight and batch runs keep the declared argv.
- `TCB-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283)
  `corvint [--root PATH] cem gate-verdict --base OID --candidate OID --predicate PREDICATE
  --touch-paths JSON --portable-consumer PATH` MUST evaluate the verdict gate from a closed argv: each
  of the five flags exactly once with a value not starting `--`, `--touch-paths` a JSON string array,
  and nothing before `cem` except `--root` with an absolute clean path. Without `--root` the root is
  the working directory with symbolic links resolved. It prints the gate document and exits 0 on GRANT
  and 1 on REFUSE; an unreadable argv prints `cem-gate-verdict-invalid-arguments` to stderr, no
  stdout, and exits 2.
- `TCB-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) The gate MUST
  run the independent portable consumer at the absolute operator-supplied path (an `interop/cem01-go`
  build) in the gate's process group on the same sealed map bytes through `verify-stable-canonical`,
  and pin its route, outcome, claimed `verdictSha256` and citation digest as `portable`. The portable
  outcome, claimed digest, recomputed digest and canonical citation block MUST each equal the native
  ones; any difference refuses `CEM_VERDICT_VERIFIER_DISAGREEMENT` (`verifier-disagreement`). A
  consumer that cannot run, emits no readable result, ends on a signal, or exits other than 0 on
  ACCEPT and 1 or 2 otherwise refuses `CEM_VERDICT_UNVERIFIABLE` (`portable-consumer-failed`).
- `TCB-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) The document MUST
  carry `paths`: each governed path once, in byte order, with the minimum recomputed citation tier of
  its governed hunks (`held` over `mechanical-proved` over `anchored` over `cited` over `unknown`) and
  whether it satisfies. A tier a producer writes into the sealed map is never admitted: such a map is
  not accepted by the stable consumers and refuses before any predicate runs.
- `TCB-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1283) The gate MUST be
  registered as the `cem-verdict` policy gate below: kind `COMMAND`, cwd `WORKTREE`, env `PATH`, a
  1800-second timeout, expected exit 0, `output` evidence, not reusable and `required: false`, so only a
  ticket that names it in `requiredGates` must pass it. A Tasks PASS records the gate document's digest
  as its output evidence; a FAIL records exit 1 and the refusal document, or exit 2.
- `TCB-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1356) For a ticket
  that requires `cem-verdict`, the gate's candidate MUST be the lane's seal commit: the commit
  `make dogfood-seal` made, whose tree is the bind commit's tree with only the sidecar renamed. The
  attempt is claimed at the lane base that the sealed map's `baseRevision` names, not at a merge's
  first parent; it submits the seal commit's tree, and the gate runs in a worktree whose HEAD is the
  seal commit, so `{attempt.candidateCommit}` is the seal commit. The gate and completion bind trees,
  not commit IDs: a later commit whose tree equals the seal commit's (an empty commit, or a change
  reverted to the sealed tree) presents the same sealed content and is treated as the seal commit.
- `TCB-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1356) Native
  completion of that attempt MUST pin the seal commit itself (`complete --commit <seal>`) under the
  unchanged CAL-V0-017 rule: the commit's tree equals the candidate tree, and the queue's intent
  branch (`main`) or its remote-tracking upstream contains the commit. A merge-commit integration
  (AFP-V0-016) contains it through the merge's second parent, however far `main` moved and whatever
  post-seal commits (such as a merge of `main` into the lane) follow it on the lane branch. The
  merge commit and any post-seal commit whose tree differs refuse `STALE_TREE`; a seal commit `main` does not contain
  refuses `STALE_TREE` with `COMMIT_NOT_INTEGRATED`. No quiescent `main` is required.
- `TCB-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1356)
  `sealed-tree-mismatch` MUST stay the refusal for every candidate whose tree is not the sealed
  target's tree plus the renamed sidecar, including a merge commit over a moved `main` and any
  post-seal commit that changes the tree; the gate never reconstructs or credits a merge. A GRANT proves the sealed lane
  change against the lane base, not the merge result: the merge's integration effects and the
  content of any post-seal commit are proven only by the merge-queue CI run on the merge commit.
- `TCB-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1356) Under
  `touched-anchored`, a touch key without a trailing `/` that names a directory in the base or the
  candidate commit's tree MUST refuse `CEM_VERDICT_INPUT_INVALID` (`touch-key-names-directory`)
  with the inputs and candidate tree pinned, before any map is read; the key is never widened to a
  prefix. A directory listing that cannot be read refuses `CEM_VERDICT_UNVERIFIABLE`
  (`git-read-failed`). A key naming a file or nothing keeps the exact-path rule, and `all-anchored`
  does not check keys.
- `TCB-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1761) When the
  legs disagree under TCB-V0-017, the gate MUST read the Go build information that this verifier
  and the `--portable-consumer` executable record, without running the consumer, and refuse
  `CEM_VERDICT_VERIFIER_DISAGREEMENT` / `verifier-build-mismatch` instead of
  `verifier-disagreement` when both recorded a `vcs.revision` and the revisions differ or either
  records `vcs.modified=true`: the legs are not known to share their source, so a stale or
  locally modified consumer is named before a verifier defect is suspected. A build that records
  no revision (a test binary, a non-Git build, a wrapper script) proves nothing and keeps
  `verifier-disagreement`. The builds are never read for agreeing legs, a build difference never
  refuses by itself, and the document stays free of revisions and paths (TCB-V0-040). The
  recovery is to rebuild the consumer from the verifier's checkout,
  `(cd interop/cem01-go && GOTOOLCHAIN=local go build -o "$CONSUMER" .)`, and rerun the gate.
  `corvint-update` installs release components only and does not install `cem01-go`; the
  documented route that rebuilds and installs both from one checkout is
  `script/install-cem-verifiers.sh` (TCB-V0-046).
  `corvint help cem gate-verdict` names `verifier-build-mismatch` and this rebuild command
  (amendment accepted under the same standing auto-accept, V1-2324).
- `TCB-V0-046`: (proposed; V1-1761) `script/install-cem-verifiers.sh [--bin-dir DIR] [--keep N]`
  is the documented install route for the gate's two verifiers. Run from a Corvint checkout, it
  MUST refuse (exit 2, nothing built or installed) a checkout with any modified, staged or
  untracked file, since Go then records `vcs.modified=true`. It MUST build `./cmd/corvint` and
  `interop/cem01-go` from that HEAD into a temporary directory with `GOTOOLCHAIN=local` and
  `GOOS`/`GOARCH` set to `go env GOHOSTOS GOHOSTARCH`, whatever the environment sets, and MUST
  refuse (exit 2, nothing installed) unless both builds record that host target, `vcs.revision`
  equal to HEAD and `vcs.modified=false`. It then installs each over `DIR/corvint` and `DIR/cem01-go` (default
  `~/.local/bin`) through `script/install-local-bin.sh` (ASD-V0-025, one
  `<tool>.corvint-prev-<n>` rollback each, more with `--keep N`) and re-reads the installed pair,
  printing `MATCH <revision>`. `--check` builds and writes nothing: it reads the build
  information both installed tools record and prints `MATCH <revision>` (exit 0), or `MISMATCH`
  (revisions differ or either is modified, the TCB-V0-045 `verifier-build-mismatch` condition) or
  `UNRECORDED` (a tool records no revision) with each tool's build (exit 1); a missing tool or bad
  arguments exit 2. Non-goals: `corvint-update` is unchanged and still installs release archives
  only; the script installs no other component and never runs the gate. Failure modes: a failed
  second install leaves a mismatched pair, which `--check` and the gate's TCB-V0-045 refusal both
  name. Rollback: `script/install-local-bin.sh --rollback DIR/<tool>` for each tool, or revert the
  script.
- `TCB-V0-047`: (accepted, decision 0548, reaffirmed 2026-10-24
  (owner-held decisions, PR #1721); V1-1895) `complete` MUST keep CAL-V0-017's
  tree equality unchanged: a merge commit whose tree was never submitted and gated is never accepted.
  Both `STALE_TREE` refusals of the completion check MUST name the post-merge sequence, for a change
  that landed as a new merge commit (a merge queue): submit that merge commit's tree
  (`submit --tree $(git rev-parse <merge>^{tree})`, since `submit` takes a full tree OID), run each
  required gate on it (`gate run`), then `complete --commit $(git rev-parse <merge>)`. This holds
  for the tree mismatch and for a commit that neither the intent branch nor its upstream contains,
  with or without a configured upstream. The fetch or integrate route the unreachable refusal
  already names stays first, since a merge-commit
  integration contains the seal commit (TCB-V0-021). V1-1895's first acceptance criterion is
  amended from "the single post-merge command" to "one named sequence".

  Non-goals: no pending completion bound to a PR head, no completion of a merge commit whose
  first-parent diff equals the submitted change without a gate rerun (both rejected by decision
  0548), and no change to codes, to the extra-repository refusal (CAL-V0-087) or to any wire or
  store format. Failure mode: a lane following only the fetch route after a merge queue rewrote its
  tree keeps refusing `STALE_TREE`; the named sequence is its exit. Rollback: revert
  `postMergeSequence` and its three uses in `internal/tasks/transaction/lease_gate.go` and
  `internal/tasks/transaction/lease_gate_post_merge_test.go`.

## Acceptance and rollback

Prove canonical capture, tamper/command/policy/snapshot negatives, verify with a missing repository,
read-only filesystem behavior, duplicate-flag refusal and current versus historical identity. Existing
criterion behavior, focused tests/vet, actual compiled nonfixture native Tasks lifecycle and unchanged
import-direction test MUST pass. Rollback removes optional verbs/adapter and preserves prior receipts;
existing Tasks schemas/policy and frozen CEM bytes do not change. TCB-V0-001..023 have experimental
implementation and scoped acceptance evidence below. TCB-V0-015..023 are accepted under the owner's
standing auto-accept; the rest of this proposed technical contract is not thereby accepted or
promoted. No stable release or execution cutover is implied. TCB-V0-035..039 (Verdict predicate domain)
have the same experimental status and are accepted under the same standing auto-accept (V1-1359).

Capture includes canonical claim-era ticket bytes whose digest equals the attempt claim record, and compares the ordered criteria with the current ticket at the same acceptance revision. Body-only refinements remain admissible. Attempt phase and lease sequence numbers may not exceed the retained snapshot head. Historical audit provides bounded structural journal coherence; semantic authority and authentication remain NOT_OBSERVED. Earlier proposed captures without claim-era bytes remain unsupported by this verifier rather than being silently upgraded.

## Traceability and retained evidence

| Requirement | Source and focused tests | Observed result / limit |
| --- | --- | --- |
| TCB-V0-001 | `internal/tasks/wire/criterion_binding.go`, `internal/tasks/criterionbinding/verify.go`; unchanged `internal/tasks/boundary_test.go` `TestImportDirection` | Core consumes wire, while native Tasks validates full records; no import whitelist |
| TCB-V0-002 | `internal/tasks/cli/criterion_binding.go`; `TestCriterionCaptureRetainsClaimAcrossNativeBodyEdit`, `TestClaimedAcceptanceAndSequenceBindings` | Claim afterimage and current criteria coherence retained; body-only refinement admitted, future attempt/lease sequences refused |
| TCB-V0-003..004 | `TestNativeCaptureVerification`, CLI `TestCriterionBindingOfflineVerify` | Closed native capture, offline historical verification and producer/verifier identities; no current-authority inference |
| TCB-V0-005..006 | `internal/criterionexperiment/tasks_unix.go`; `TestTasksExecutableIndependentPin`, `TestTasksTransportBoundsAndCancellation`, `TestTasksCancellationRetiresDescendant` | Independently pinned bounded read transport and ordinary descendant retirement; same-user races remain outside authentication |
| TCB-V0-007 | `internal/taskman/cem_verdict_gate.go`; `TestTCBV0007GateInputsAreNativeAndClosed` | Malformed roots, IDs, predicates and touch keys refuse unpinned; touch keys pin sorted and unique |
| TCB-V0-008 | `TestTCBV0008SealedMapIsReadFromCandidateGit` | Change and bind candidates refuse missing, a worktree-only copy is ignored, two sealed maps refuse ambiguous |
| TCB-V0-009 | `TestTCBV0009ProducerFieldsDisagreeingRefuse` | Base mismatch, a post-seal edit, edited sealed bytes and a non-commit target each refuse before the consumer |
| TCB-V0-010 | `TestTCBV0010UnverifiableMapRefuses` | A non-repository root refuses as a read failure; a wrong producer `patchSha256` sealed byte for byte refuses with the consumer's non-ACCEPT verdict pinned |
| TCB-V0-011 | `TestTCBV0011PredicateIsClosedAndNeverRaisesATier` | A cited hunk stays cited and fails; governed tiers equal the recomputed citation; exact and prefix keys |
| TCB-V0-012 | `TestTCBV0012GrantPinsVerdictAndCitation` | GRANT pins the same `verdictSha256` and citation as an independent stable consumer run; deterministic bytes |
| TCB-V0-013 | `TestTCBV0013TamperedEvidenceRefusesOnReverify` | Store-less re-verification grants; edited digest, raised tier, dropped citation and other inputs refuse |
| TCB-V0-014 | `TestTCBV0014GateIsReadOnly` | Status, refs, HEAD and object counts are unchanged across GRANT and REFUSE evaluations |
| TCB-V0-015 | `internal/tasks/store/lease_gate.go`; `TestTCBV0015_GateRunSubstitutesAttemptFacts` | Base, candidate HEAD and claim-era touch paths replace only whole entries; substituted argv binds the inputs digest; an edited ticket refuses `STALE_TICKET` |
| TCB-V0-016 | `cmd/corvint/cem_gate_verdict.go`; `TestTCBV0016GateVerdictArgvIsClosed`, `TestTCBV0016GateVerdictRunsBothConsumers` | Twelve unreadable argv shapes exit 2 with one stderr code; on a sealed fixture the command, run with a built `interop/cem01-go`, prints the library's bytes, grants, and refuses a patched portable result |
| TCB-V0-017 | `TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict`, `TestTCBV0017PortableConsumerSharesTheGateProcessGroup` | Agreement pins equal digests; a changed claim, raised tier or outcome disagrees; a missing, unreadable, other-profile or digest-less result, or an ACCEPT that exits 1 or 2 or is killed, is unverifiable; the consumer runs in the gate's process group |
| TCB-V0-018 | `TestTCBV0018PathsCarryTheMinimumRecomputedTier`, `TestTCBV0018ProducerEditedTierIsNeverAdmitted` | A path takes its lowest hunk tier; a sealed map with producer-written `held` tiers refuses `stable-not-accepted` with no predicate rows |
| TCB-V0-019 | `TestTCBV0019_CEMVerdictPolicyEntryRunsAsDeclared` | The entry registers in a fixture policy; stand-in commands with its argv record PASSED and FAILED with the substituted argv as output evidence |
| TCB-V0-020, TCB-V0-022 | `internal/taskman/cem_verdict_merge_forward_unix_test.go` `TestTCBV0020GateGrantsOnlyAtTheSealCommitOfAMergedForwardLane` | Over a moved `main`, the merge commit and a post-seal merge of `main` into the lane each refuse `sealed-tree-mismatch` naming the lane's sealed map, with no consumer run; the seal commit, the merge's second-parent ancestor, grants, and so does a tree-identical empty commit after it |
| TCB-V0-021 | `internal/tasks/store/lease_gate_merge_forward_test.go` `TestTCBV0021_CompletePinsTheSealCommitOfAMergedForwardLane`; live V1-1348 completion (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-v1-1283-native-completion-through-cem-verdict.md`) | The merge and post-seal commits refuse `STALE_TREE` alone and write nothing; the seal commit refuses `COMMIT_NOT_INTEGRATED` until `main` contains it, then completes with no Tasks code change |
| TCB-V0-023 | `TestTCBV0023ExactKeyNamingADirectoryRefuses`, `TestTCBV0023DirectoryListingFramingFailsClosed`; amended `TestTCBV0011PredicateIsClosedAndNeverRaisesATier` | Directory keys at the base only, the candidate only or both refuse pinned before the map; file and absent keys grant; an unlistable candidate or a misframed listing refuses `git-read-failed`; `all-anchored` ignores keys |
| TCB-V0-047 | `internal/tasks/transaction/lease_gate_post_merge_test.go` `TestTCBV0047_TreeMismatchNamesThePostMergeSequence`, `TestTCBV0047_NotIntegratedNamesThePostMergeSequence` | Each refusal branch keeps its codes and names submit of the merge tree, the gate run and `complete --commit $(git rev-parse <merge>)`, each resolved to a full OID; the extra-repository refusal is unchanged |

Independent repair review `/private/tmp/cem10-build/tasks-rereview.json` passed the original
claim-digest and future-sequence counterexamples plus positive native lifecycle tests. The compiled
nonfixture disposable lifecycle in `live-demo-repair1-evidence.json` retained eight scenarios,
three killed controls, native completion, a consistent final audit and historical replay after an
acceptance change. Its qualification admission input was explicitly synthetic feasibility input,
not deployment qualification. Source/binary/result digests remain in that manifest. This proves the
specific Tasks/CEX boundary; it does not transfer historical satisfaction, source Git authority or
completion authority to opaque `cem/1.0-experimental.1` references.

## Sealed cem/1.0 verdict gate

Decision 0525 §1 step 8 asks a Tasks gate to consume the sealed cem/1.0 verdict. The gate reads the
committed sealed map from Git at the attempt's candidate commit and recomputes the verdict; an
attached-evidence receipt was rejected because it carries a producer-written digest bound to the ticket
rather than the candidate. The decision and its rejected alternative are recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-13-v1-1283-tasks-gate-sealed-verdict.md`.

Non-goals: Tasks never raises a tier, never trusts a producer-written field, never reads the map from
the worktree, an attachment or the store, and never completes a ticket from a GRANT. Artifacts a map
declares are the one worktree input: the stable consumer reads them under the root and refuses unless
their bytes match the digests the sealed map pins, so a re-verifier needs those artifact bytes. The `held`
count predicate waits for V1-1266. Verifying a chain of stacked seals is follow-up work. The gate never
takes the portable consumer's path from the map or the store, and a portable result never raises
or replaces a native tier.

Failure modes. Every refusal pins one closed code and one closed reason:

| Code | Reason | Cause |
| --- | --- | --- |
| `CEM_VERDICT_INPUT_INVALID` | `invalid-input` | Malformed root, commit ID, predicate, touch key or portable consumer path |
| `CEM_VERDICT_INPUT_INVALID` | `commit-unavailable` | The base or candidate commit is not in the repository |
| `CEM_VERDICT_INPUT_INVALID` | `touch-key-names-directory` | Under `touched-anchored`, a touch key without a trailing `/` names a directory at the base or candidate |
| `CEM_VERDICT_MAP_MISSING` | none | No sealed map is added between base and candidate |
| `CEM_VERDICT_MAP_AMBIGUOUS` | none | More than one sealed map is added |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-target-unavailable` | The sealed name's target is not a commit |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-tree-mismatch` | The candidate is not the target plus only the renamed sidecar |
| `CEM_VERDICT_PRODUCER_MISMATCH` | `sealed-base-mismatch` | The map's `baseRevision` is not the attempt base |
| `CEM_VERDICT_PRODUCER_MISMATCH` | none | Re-verified pinned bytes differ from the recomputation |
| `CEM_VERDICT_UNVERIFIABLE` | `unsupported-platform` | The host is not Darwin or Linux |
| `CEM_VERDICT_UNVERIFIABLE` | `git-read-failed` | The root is no readable repository or a bounded Git read failed |
| `CEM_VERDICT_UNVERIFIABLE` | `portable-consumer-failed` | The portable consumer did not run, exceeded its bound, emitted no readable `cem-stable-verification/1` result, ended on a signal or exited with a status its outcome does not explain |
| `CEM_VERDICT_UNVERIFIABLE` | `stable-not-accepted` | The stable consumer did not ACCEPT with an EXACT sidecar and citation |
| `CEM_VERDICT_VERIFIER_DISAGREEMENT` | `verifier-disagreement` | The portable outcome, claimed or recomputed `verdictSha256`, or citation block differs from the native one |
| `CEM_VERDICT_VERIFIER_DISAGREEMENT` | `verifier-build-mismatch` | The same difference, when both executables record a `vcs.revision` and they differ or either tree was modified (TCB-V0-045) |
| `CEM_VERDICT_PREDICATE_FAILED` | `no-governed-hunk` | No hunk lies under the touch keys |
| `CEM_VERDICT_PREDICATE_FAILED` | `tier-below-anchored` | A governed hunk is `cited` or `unknown` |

The command adds one closed stderr code, `cem-gate-verdict-invalid-arguments` (exit 2), for an argv it
cannot read. Every other outcome is a gate document.

Tasks gate wiring. TCB-V0-015..019 let a native Tasks COMMAND gate run this library. The decisions, the rejected
alternative (a gate that reads the attempt from a pinned Tasks read) and the rehearsals are recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-13-v1-1283-gate-wiring.md`. The runner substitutes the attempt facts, so the
command takes them as argv. The working directory is the attempt worktree, so the command needs no
`--root`. The registration, with this example's two executable paths replaced by the installed
absolute ones, is:

```json
{"gateId":"cem-verdict","kind":"COMMAND","argv":["/usr/local/bin/corvint","cem","gate-verdict","--base","{attempt.baseCommit}","--candidate","{attempt.candidateCommit}","--predicate","touched-anchored","--touch-paths","{claim.touchPaths}","--portable-consumer","/usr/local/libexec/cem01-go"],"cwd":"WORKTREE","env":["PATH"],"timeoutSeconds":"1800","expected":{"exitCode":"0","reducer":null},"evidence":["output"],"inputs":[],"sharedResource":null,"reusable":false,"required":false}
```

Both executables must be built from a commit that contains TCB-V0-015..019. `PATH` must be in the
policy's `environment.allowedEnvKeys`. The portable consumer runs with `PATH=/usr/bin:/bin` whatever
the gate's environment, and needs `git` there. The gate must run on the seal commit itself
(`{attempt.candidateCommit}` is the worktree HEAD), because a later commit fails `sealed-tree-mismatch`;
a lane that merged `main` forward follows TCB-V0-020 and TCB-V0-021 (see Merged-forward lanes).
Preflight and batch runs keep placeholders literal, and the command then refuses
`CEM_VERDICT_INPUT_INVALID`.

Acceptance evidence: the TCB-V0-007..019 tests above, run on a dogfood-shaped fixture (base, change,
bind and seal commits produced by the CEM workflow), plus the unchanged Tasks import-direction test.
The TCB-V0-016 test runs a freshly built `interop/cem01-go` on darwin, the only host where that
consumer's process containment is qualified; elsewhere it skips `NOT_RUN`. The `internal/taskman`
tests use their own test binary as a stand-in portable consumer that answers with the in-process
stable verifier, because those tests may not read the interop module; they run on darwin and linux.

Rollback: delete `cmd/corvint/cem_gate_verdict.go` and its test, the `cem gate-verdict` dispatch and
help, the placeholder substitution in `internal/tasks/store/lease_gate.go` and its tests, and
`internal/taskman/cem_verdict_gate.go` with its tests, then remove any `cem-verdict` policy entry from
a live store. No Tasks schema, wire contract or CEM byte changed. A gate result recorded with a
substituted argv keeps its inputs digest; a declared argv's digest is unchanged.

## Merged-forward lanes

The requirements TCB-V0-020..023 amend this contract for a lane whose `main` moved before
integration. The decision, the rejected alternatives and the live evidence are recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-v1-1356-cem-verdict-merge-forward.md`.

Decision. The candidate is the lane's seal commit. The lane claims at the base its sealed map names,
submits the seal commit's tree, runs `cem-verdict` with the seal commit as the worktree HEAD, and
completes with `complete --commit <seal>` once `main` contains the seal commit. Under the merge
queue's merge-commit method (AFP-V0-016) the seal commit stays reachable from `main` through the
merge's second parent, so CAL-V0-017 accepts it unchanged. `sealed-tree-mismatch` stays the refusal
for every candidate whose tree is not the target tree plus the renamed sidecar, which covers the
merge commit and any later commit that changes the tree. Both checks bind trees, not commit IDs, so a
tree-identical later commit is the same candidate.

Limit. A GRANT proves the sealed lane change, not the merge result. Integration effects, including a
post-seal commit or a merge resolution that alters lane content, are not seen by the gate; they
belong to merge-queue CI (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-merge-queue-enabled.md`).

Rejected alternatives:

- A merge-aware rule that accepts a merge commit by reconstructing the sealed tree from its parents.
  It widens the trusted surface to merge reconstruction and conflict resolution, which the sealed
  map does not bind.
- A quiescent-`main` requirement that refuses unless `main` is still the lane base. It is unworkable
  under the merge queue: every lane integrated on 2026-10-13 saw a moved `main`.

Live evidence. V1-1348 completed natively through this rule (recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-v1-1283-native-completion-through-cem-verdict.md`): a gate at the merge
commit refused `sealed-tree-mismatch`, the gate at the seal commit granted, `complete --commit <seal>`
returned OK and `receipt audit` reported the store CONSISTENT.

Directory keys. TCB-V0-023 refuses rather than normalizes a key without a trailing `/` that names a
directory. Normalizing would give the gate a key meaning that Tasks does not share: under CAL-V0-021
such a key governs only that exact path, for claims and resource collisions alike. CAL-V0-211's
`EXACT_KEY_NAMES_DIRECTORY` warning at set-effects and claim time stays advisory; the gate is where
the key's mismatch with the sealed change becomes a refusal.

Interaction. V1-1359 (separating the predicate domain from claim scope) may change which keys the
predicate reads. This amendment does not depend on it; TCB-V0-023 applies to whatever keys reach
`--touch-paths`.

Rollback: delete `exactKeyNamesDirectory` and its call in `internal/taskman/cem_verdict_gate.go`,
`internal/taskman/cem_verdict_merge_forward_unix_test.go`,
`internal/tasks/store/lease_gate_merge_forward_test.go`, and the `touch-key-names-directory` case in
`TestTCBV0011PredicateIsClosedAndNeverRaisesATier`. No Tasks code, schema, wire contract or CEM byte
changed.

## Portable leg qualified hosts (V1-1358, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1358.
The portable consumer's process containment is now qualified on Darwin and Linux amd64/arm64
(`docs/specs/cem-stable-v1.md` `CEM-V1-180..184`). This section supersedes the acceptance-evidence
statement above that the TCB-V0-016 test runs only on Darwin and otherwise skips `NOT_RUN`.

- `TCB-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) The
  qualified hosts for the gate's portable leg (TCB-V0-017) MUST be Darwin and Linux amd64/arm64,
  the hosts where `interop/cem01-go` qualifies process containment. On those hosts
  `TestTCBV0016GateVerdictRunsBothConsumers` MUST run a freshly built `interop/cem01-go` and
  pass, with the portable outcome and `verdictSha256` equal to the native ones, and it MUST log
  both digests. It MAY skip `NOT_RUN` only on another Linux architecture or, as every stable
  verifier test does, on an operating system other than Darwin and Linux.
- `TCB-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) On any
  other host the portable consumer still answers `UNSUPPORTED` with
  `unsupported-process-containment`, and the gate MUST keep refusing
  `CEM_VERDICT_VERIFIER_DISAGREEMENT`. It MUST NOT substitute a stand-in consumer or the native
  verifier for the portable leg.

Rollback: revert the V1-1358 commit. The TCB-V0-016 real-consumer test returns to skipping
`NOT_RUN` off Darwin, and the gate on Linux refuses `verifier-disagreement` again. No Tasks schema,
wire contract, policy entry or gate digest changes.

### Portable leg qualified hosts traceability

| Requirement | Source and focused tests | Observed result / limit |
| --- | --- | --- |
| TCB-V0-030 | `cmd/corvint/cem_gate_verdict_test.go` `TestTCBV0016GateVerdictRunsBothConsumers` | darwin/arm64 and linux/arm64 (go1.27.2, uid 1000, `-race`): native and portable `verdictSha256` both `14ac383d61741ab12c6a422d086701302b230295ff97babcb443aead1cb40476`, portable `ACCEPT`; linux/amd64 is the CI `go-product-shard` run |
| TCB-V0-031 | `interop/cem01-go/stable_process_other.go`; `TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict` | Other hosts refuse before admission (cross-`go vet` for linux/386, linux/riscv64, windows/amd64, freebsd/amd64); a differing portable outcome refuses `verifier-disagreement` |

## Verdict predicate domain

Decision (V1-1359). The `cem-verdict` gate's predicate domain is a distinct, closed input from the claim
scope. Under policy v9 `touched-anchored` governed `{claim.touchPaths}`, the ticket's `effects.touchPaths`,
which is also the Tasks resource reservation. One field had two consumers and the narrower one won: an
ordinary cited hunk never reaches `anchored`, so a ticket that declared its real write set could not pass,
and V1-1348 had to declare only its generated spec projections and widen after the claim. A ticket now
declares its full write set as `touchPaths` and, optionally, the narrower governed list as
`effects.verdictPaths`, which the gate receives through the new `{claim.verdictPaths}` placeholder.
Absent, the placeholder falls back to `touchPaths`, so every existing ticket, record byte and completion
keeps its meaning. The detailed evidence is in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-v1-1359-verdict-domain.md`.

Rejected alternatives: a per-gate path list in the policy (the domain is a property of each ticket's
change, not of the gate, and one policy list would govern every ticket the same way); deriving the
domain from the CEM's own anchored set (circular: the evidence would choose what it is judged on, so
every map would pass); and changing `touchPaths` semantics in place (it is the reservation that keeps
concurrent lanes apart, so narrowing it breaks collision detection and widening it breaks the gate).
Making `{claim.touchPaths}` prefer `verdictPaths` was also rejected: a placeholder would no longer name
the fact it carries, and a gate that needs the claim scope could not get it.

- `TCB-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1359) A ticket's
  `effects` MAY carry `verdictPaths`, the verdict predicate domain, in the `touchPaths` shape (a
  sorted, duplicate-free set of at most 256 repository Paths). `touchPaths` stays the claim scope and
  reservation, unchanged. A record or payload that omits the key keeps its earlier bytes; a present key
  is encoded in canonical key order. The native codec, the mutation payload reader and Core's read-only
  planner MUST admit exactly the shared lists `TicketEffectsKeys` and `TicketEffectsOptionalKeys`; the
  planner reserves and plans on `touchPaths` alone.
- `TCB-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1359) A present
  `verdictPaths` MUST be nonempty, and each path MUST be covered by a `touchPaths` key under the CAL-V0-021
  PATH key rule (equal bytes, or a key ending in `/` that prefixes it). CREATE and SET_EFFECTS refuse an
  empty list `MALFORMED` (`VERDICT_PATHS_EMPTY`) and an uncovered path `OUT_OF_SCOPE`
  (`VERDICT_PATH_OUTSIDE_TOUCH_PATHS`) and write nothing; both ticket readers refuse such a record.
  SET_EFFECTS replaces the whole `effects` object, so omitting the key restores the fallback. An exact
  verdict key that names a directory draws the CAL-V0-211 warning a touch key draws.
- `TCB-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1359) A fourth closed
  argv placeholder, `{claim.verdictPaths}`, MUST be replaced with the claim-era ticket's
  `effects.verdictPaths` as one canonical JSON string array, or its `effects.touchPaths` when it declares
  none, under the TCB-V0-015 rules: whole entries only, `STALE_TICKET` for a ticket changed since the
  claim, `UNSUPPORTED` for a multi-repository candidate, and the substituted argv joins the inputs
  digest. `{claim.touchPaths}` keeps its meaning.
- `TCB-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1359) Given the
  substituted verdict domain, `touched-anchored` MUST govern only the hunks under it: a ticket whose
  `touchPaths` hold its full write set and whose `verdictPaths` hold the anchored subset is GRANTed,
  while the same change judged on the write set alone keeps the earlier `tier-below-anchored` refusal.
- `TCB-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1359) The repository
  policy `.taskman/policy.json` version 10 MUST register `cem-verdict` with `--touch-paths
  {claim.verdictPaths}` and is otherwise the TCB-V0-019 entry of version 9. A store MUST adopt version 10
  only after its installed `corvint-tasks` contains TCB-V0-037. An older runner passes the placeholder
  literally, which `corvint cem gate-verdict` refuses as an unreadable argv (exit 2), so the gate fails
  closed rather than governing another domain.

Empty list. An empty `verdictPaths` is refused rather than read as absent: `touched-anchored` over no
key is invalid gate input (TCB-V0-007), and one meaning with two encodings would break the canonical
record. A ticket that wants the fallback omits the key.

Directory keys. `verdictPaths` reuse the rule `touchPaths` use at BASE (`ticket.PathCovers`, now
`wire.PathKeyCovers`, shared by both readers). V1-1356 (TCB-V0-020..023, PR #874) makes the gate refuse
an exact touch key that names a directory. Verdict paths reach the gate as its `--touch-paths`
argument, so that refusal applies to them unchanged; at create time the store can only warn
(CAL-V0-211), because no tree is bound to a ticket.

Non-goals: no new wire code, no change to claim, reservation, collision or scope checks, no Core-side
choice between the two lists (the Tasks runner substitutes one array), and no re-run of a completed
ticket. V1-1348 declared no `verdictPaths`, so its completion keeps the fallback meaning.

| Failure | Refusal |
| --- | --- |
| Empty `verdictPaths` in a payload or record | `MALFORMED`, `VERDICT_PATHS_EMPTY` |
| A verdict path no touch key covers | `OUT_OF_SCOPE`, `VERDICT_PATH_OUTSIDE_TOUCH_PATHS` |
| Null, unsorted, duplicate or invalid paths, or an unknown `effects` member | `MALFORMED` |
| Ticket edited since the claim when the placeholder is substituted | `STALE_TICKET`, before the gate runs |
| Policy version 10 run by a runner without TCB-V0-037 | gate exit 2, `cem-gate-verdict-invalid-arguments`; the gate fails |

| Requirement | Source and focused tests | Observed result / limit |
| --- | --- | --- |
| TCB-V0-035 | `internal/tasks/wire/ticketkeys.go`, `internal/tasks/ticket/verdict_paths.go`, `internal/taskman/decode.go`; `TestTCBV0035_RecordVerdictPathsKey`, `TestTCBV0035_ReaderAdmitsVerdictPaths` | A legacy record keeps its bytes; a carrier round-trips byte-identically and deterministically; Core decodes the codec's own fixture with and without the key |
| TCB-V0-036 | `internal/tasks/mutation/mutation.go`, `internal/tasks/store/directory_keys.go`; `TestTCBV0036_RecordVerdictPathsRefusals`, `TestTCBV0036_ReaderRefusesMalformedVerdictPaths`, `TestTCBV0036_MutationsRefuseUncoveredOrEmptyVerdictPaths`, `TestTCBV0036_VerdictPathKeysWarnLikeTouchPaths` | Both readers refuse empty, uncovered (beneath an exact key, or a prefix without `/`), null and unsorted lists, and Core also duplicate and over-bound ones; CREATE refuses uncovered, empty, unsorted and duplicate lists and SET_EFFECTS uncovered and empty ones, each with no store change; SET_EFFECTS without the key drops it |
| TCB-V0-037 | `internal/tasks/store/lease_gate.go`; `TestTCBV0037_VerdictPathsPlaceholder` | Declared: `["src/"]` beside `{claim.touchPaths}` `["lib/","src/"]`; absent: both substitute `["lib/","src/"]`; an edited ticket refuses `STALE_TICKET` |
| TCB-V0-038 | `TestTCBV0038_VerdictDomainNarrowsTheGovernedSet` | On the dogfood-shaped fixture the write set refuses `tier-below-anchored` over two hunks and the verdict domain grants over the anchored hunk. Composed proof: the import boundary keeps the Tasks runner and the Core gate in separate tests joined by the same argv bytes |
| TCB-V0-039 | `.taskman/policy.json` | Version 10 decodes with the Tasks policy decoder; `TestTCBV0016GateVerdictArgvIsClosed` ("touch not json") covers the literal placeholder's exit 2 |

Rollback: restore `.taskman/policy.json` version 9 (a store that adopted version 10 takes a version 11
with `{claim.touchPaths}`), then delete the `{claim.verdictPaths}` substitution, `verdict_paths.go`, the
optional effects key lists and the Core admission, and these tests. A record that carries
`verdictPaths` must first drop it by SET_EFFECTS, because a reader without the key refuses it.

## Portable leg Git resolution and unsupported-route refusal (V1-1377, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1377.
Release-gates nightly run 38559681509 (job 115837380242, hosted `macos-15`) failed the TCB-V0-010..013,
016, 017 and 018 tests with `CEM_VERDICT_VERIFIER_DISAGREEMENT`: the native leg accepted and the portable
leg answered `UNSUPPORTED`, and the gate document said nothing more. The retained diagnostic output of
the V1-1377 dispatch (run 38650914390, same job matrix, commit 22fca01e6) names the cause: under the
consumer's fixed `PATH=/usr/bin:/bin` the hosted runner resolves Apple Git 2.39.5, which rejects the
`--attr-source` option (Git 2.41 or later) that the stable repository diff passes, so the consumer
answers `UNSUPPORTED` at stage `repository` with `git-diff-failed` (`docs/specs/cem-stable-v1.md`
`CEM-V1-185`), while the gate's own reads and the native verifier resolve the runner's newer Git
through the gate process's PATH. The two legs were running two different Gits. This section pins
the consumer's stage and code in the document, refuses an unsupported portable route by name, and
makes both legs resolve one Git. It supersedes the sentence above that the portable consumer "runs
with `PATH=/usr/bin:/bin` whatever the gate's environment", and the TCB-V0-031 clause that an
unsupported portable route keeps refusing `CEM_VERDICT_VERIFIER_DISAGREEMENT`: the gate still
refuses, and still never substitutes a consumer, but the refusal names the unsupported route.

- `TCB-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The gate
  document's `portable` pin MUST carry the consumer's `stage` and `code` exactly as the
  `cem-stable-verification/1` result states them: `stage` a member of the closed `stage`
  enumeration and `code` either `null` or a member of the closed `code` enumeration of
  `protocol/cem-1.0/stable/RESULT.schema.json`, which the gate holds as two sets that a test pins
  to the schema. A result whose `stage` or `code` is anything else is unreadable and refuses
  `CEM_VERDICT_UNVERIFIABLE` / `portable-consumer-failed` with no portable pin, as any other
  unreadable result does. An agreeing `ACCEPT` pins `stage` `complete` and `code` `null`. No host
  path, version string or free text enters the document.
- `TCB-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) When the
  portable outcome is `UNSUPPORTED` and the native outcome is not, the gate MUST refuse
  `CEM_VERDICT_UNVERIFIABLE` / `portable-route-unsupported` with the native verdict and the
  portable `stage` and `code` pinned, a `null` citation and no governed hunks or paths. It MUST
  NOT grant, and `CEM_VERDICT_VERIFIER_DISAGREEMENT` remains the refusal for every other
  outcome, digest or citation mismatch, including both legs answering `UNSUPPORTED` with
  different digests. The named refusal MUST pin only a result whose `verdictSha256` claim is the
  digest recomputed from its own bytes; an `UNSUPPORTED` whose claim its result does not
  recompute to is a disagreement under TCB-V0-017, so the document never carries a false claim
  under the route's name. The consumer's exit status still has to be one its outcome explains
  under TCB-V0-017 (non-zero for `UNSUPPORTED`); a `0` under `UNSUPPORTED` is
  `portable-consumer-failed`.
- `TCB-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) A test
  MUST run the real `interop/cem01-go` through a wrapper that fixes `PATH=/usr/bin:/bin` and
  assert that the gate's answer is the one that path's Git explains: a bounded probe of
  `git --version` and of `--attr-source` under that same PATH decides between `GRANT` with an
  agreeing portable `ACCEPT` and `CEM_VERDICT_UNVERIFIABLE` / `portable-route-unsupported` with
  portable `repository` / `git-diff-failed`. The TCB-V0-016 test MUST retain that probe's output
  in its failure message, so a hosted failure names its cause rather than a disagreement.
- `TCB-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The
  portable consumer MUST run with `PATH` set to the directory of the `git` the gate process
  itself resolves, followed by `/usr/bin:/bin`, so that both legs of the cross-check run one Git.
  When the gate resolves no `git`, resolves a relative path, or resolves one already under
  `/usr/bin` or `/bin`, the consumer's PATH is `/usr/bin:/bin` alone. The resolved directory is
  an operator fact that never enters the document. Native reads (`runRead`) keep the fixed PATH.
- `TCB-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The hosted
  GitHub `macos-15` runner is a qualified host for the gate's portable leg under TCB-V0-030, and
  the TCB-V0-010..018 and TCB-V0-040..043 tests MUST pass there without a skip, a stand-in
  consumer or a relaxed refusal. The evidence is a retained `release-gates.yml` dispatch at the
  delivered commit (see the traceability table), with the preceding dispatch that retained the
  diagnostic cause.

Non-goals: no change to the consumer's closed stage and code set or to `protocol/cem-1.0`; no
`git --version` probe in the gate or the consumer (a version is not a capability proof, the
diff's own failure is); no process-containment change, which V1-1358 qualified and which is not
the cause; no search for Git beyond the gate's own resolution and the fixed path.

Failure modes: a host whose only Git lacks `--attr-source` on both legs refuses through the
native leg's `git-diff-failed` or, if the native leg accepts for any other reason while the
portable leg cannot, `portable-route-unsupported`; a gate started with no `git` on its PATH
refuses as before; a consumer that mis-states its stage or code as anything but a closed token is
unreadable. Nothing in this section turns a refusal into a grant.

Rollback: revert the V1-1377 commits. The document's `portable` pin loses `stage` and `code`,
an unsupported portable route refuses `verifier-disagreement` again, and the consumer runs under
the fixed PATH, which fails the hosted `macos-15` gate tests again. No Tasks schema, wire
contract, policy entry or gate digest changes.

### Portable leg Git resolution traceability

| Requirement | Source and focused tests | Observed result / limit |
| --- | --- | --- |
| TCB-V0-040 | `internal/taskman/cem_verdict_gate.go` `cemPortablePin`, `readPortable`, `portableStages`, `portableCodes`; `TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict`, `TestTCBV0040PortableUnsupportedRefusesNamingTheRoute`, `TestTCBV0040PortablePinSetsMatchTheResultSchema` | Agreement pins `complete` / `null`; an open, unknown, empty, numeric or missing `code` or `stage` is `portable-consumer-failed` with no pin; the two sets equal the schema's enumerations |
| TCB-V0-041 | `internal/taskman/cem_verdict_gate.go` `crossCheck`; `TestTCBV0040PortableUnsupportedRefusesNamingTheRoute`, `TestTCBV0016GateVerdictRunsBothConsumers` (patched `UNSUPPORTED` case) | A consumer whose PATH resolves a git shim that rejects `--attr-source` refuses `portable-route-unsupported` with `repository` / `git-diff-failed` and its own recomputed digest pinned; exit 0 refuses `portable-consumer-failed`; an `UNSUPPORTED` carrying the native digest as its claim refuses `verifier-disagreement`; the CLI prints the same document |
| TCB-V0-042 | `cmd/corvint/cem_gate_verdict_test.go` `TestTCBV0042PortableConsumerUnderBareHostPath`, `bareHostGitDiagnostic` | darwin/arm64 (Apple Git 2.54.0 at `/usr/bin/git`, `--attr-source` accepted): `GRANT`, portable `ACCEPT`; hosted `macos-15` run 38650914390 (Apple Git 2.39.5, `--attr-source` rejected): `portable-route-unsupported`, `repository` / `git-diff-failed` |
| TCB-V0-043 | `internal/taskman/process_unix.go` `portablePath`; `TestTCBV0043PortableConsumerResolvesTheGateGit` | A shim `git` directory on the gate's PATH is what the consumer resolves first; no, relative, `/usr/bin` or `/bin` resolution keeps `/usr/bin:/bin` |
| TCB-V0-044 | `release-gates.yml` dispatches 38650914390 (cause, commit 22fca01e6) and 38654510601 (fix, commit 52ad86a36) | Run 1 macos-15 full-gate `failure` with the cause retained (Apple Git 2.39.5 rejects `--attr-source`; portable `repository` / `git-diff-failed`); run 2 macos-15 full-gate `success` (cmd/corvint and internal/taskman `ok`), see the lane build log |
| TCB-V0-045 | `internal/taskman/cem_verdict_gate.go` `crossCheck`, `verifierBuilds`, `verifierBuildsDiffer`; `TestTCBV0045DisagreementNamesAVerifierBuildMismatch`, `TestTCBV0045RecordedBuildReadsVCSSettings`; `cmd/corvint/cem_gate_verdict.go` `cemGateVerdictHelp`, `TestTCBV0045GateVerdictHelpNamesBuildMismatchRecovery` (V1-2324: the help names the refusal and the rebuild command; fails without the help text) | Over a real fixture with a disagreeing consumer, differing revisions and a modified tree on either side refuse `verifier-build-mismatch`, while equal or unrecorded builds keep `verifier-disagreement`; agreeing legs grant without reading builds; the test binary and `/bin/sh` read as unrecorded. The test fails with the build check disabled. The installed-binary rehearsal of V1-1761 (a stale `~/.local/bin/cem01-go`) is NOT_RUN. |
| TCB-V0-046 | `script/install-cem-verifiers.sh`; the TCB-V0-046 group of `script/lane-scripts_test.sh` (`make lane-scripts-test`, stub `GO`) | Install from a clean fixture HEAD prints `MATCH`, builds from the repository root and `interop/cem01-go`, and keeps one backup per tool; `--check` reports a stale, modified or unrecorded consumer as `MISMATCH`/`UNRECORDED` (exit 1) and a missing tool exit 2; a `GOOS`/`GOARCH` environment still builds for the host; a dirty checkout, an unstamped, modified or foreign-target build and bad arguments are refused with the bin dir unchanged. The real-build rehearsal is in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-v1-2107-result-schema-opaque-widen.md`. |
