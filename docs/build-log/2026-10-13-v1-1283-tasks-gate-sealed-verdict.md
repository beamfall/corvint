# 2026-10-13 Tasks gate consumes the sealed cem/1.0 verdict (V1-1283)

Branch `claude/v1-1283-tasks-gate-sealed-verdict` from `origin/main` 94a7d3dc5, in a plain clone.
Ticket V1-1283, decision 0525 §1 step 8. Requirements `TCB-V0-007..014` in
`docs/specs/tasks-criterion-binding-v0.md`, added under that spec's existing proposed intent status;
this entry does not accept or promote them.

## Intent

After rc.3 a ticket's native completion and the change's sealed cem/1.0 verdict are two unconnected
proofs of the same change. A Tasks gate should be able to require the sealed verdict as evidence,
for example "every hunk under the ticket's `effects.touchPaths` is at least anchored", with the
verdict pinned in the gate result and re-verifiable by any consumer with no Tasks store present.
Tasks never raises a tier, never trusts a producer-written field, and completion authority stays
with the native store.

## Decision: the gate reads the committed sealed map at the attempt's candidate commit

Chosen. The gate (`internal/taskman/cem_verdict_gate.go`, profile `taskman-cem-verdict-gate/0`)
takes only native attempt facts: the repository root, the attempt's base and candidate commits, the
claim-era touch keys and one closed predicate. It reads the map, the patch and the cited evidence
from Git objects, never from the worktree:

1. It finds exactly one `.corvint/changes/<target>.cem.json` added between base and candidate.
2. It checks every producer-written field against Git. The target must be a commit. The candidate
   tree must be the target tree with only `.corvint/change.cem.json` renamed byte for byte to the
   sealed path. The map's `baseRevision` must equal the attempt base.
3. It recomputes the verdict in process with `verify.Stable`. The repository and artifact root is
   the root, the expected base is the attempt base and the target is the sealed target. It requires
   ACCEPT, an EXACT sidecar and a citation block. Artifacts a map declares are the one worktree
   input. They must match the digests the sealed map pins, so they can make a verdict refuse but
   cannot change what is granted.
4. It evaluates the closed predicate on the recomputed citation tiers only.
5. It emits one canonical, host-path-free document pinning the inputs, the candidate tree, the map
   blob and `mapSha256`, the consumer verdict with `verdictSha256`, the citation block and the
   per-hunk governed rows.

A Tasks COMMAND gate pins that output by sha256. Any consumer recomputes the same bytes from the
document's own inputs and a clone. `ReverifyCEMVerdictGate` grants only on byte equality.

Rejected: receiving the verdict as an attached-evidence receipt (`corvint-tasks-evidence-attachments-v0.md`).
It lost for four reasons:

- The attachment's digest and citation would be producer-written, so the gate would have to
  recompute them anyway.
- Evidence attachments are gate-neutral by contract.
- An attachment carries a digest rather than the map bytes, so a consumer without the producer's
  files could not re-verify.
- It binds to the ticket rather than to the attempt's candidate commit, so a verdict sealed for an
  earlier change could satisfy a later attempt.

Reading the committed map binds the evidence to the exact candidate tree that native Tasks already
pins for COMMAND gates. It also needs no new Tasks wire or schema.

Other decisions:

- The predicate vocabulary is two words: `touched-anchored` and `all-anchored`. Touch keys follow
  Tasks PATH semantics: a trailing `/` makes a prefix, and any other key is an exact path.
  `anchored`, `mechanical-proved` and `held` satisfy. A `held >= N` predicate waits for V1-1266.
- A producer disagreement is its own code (`CEM_VERDICT_PRODUCER_MISMATCH`), distinct from an
  unverifiable map, so a tampered seal is not reported as a CEM verification failure.
- The gate is a Core library in `internal/taskman`. Decision 0397 keeps `cmd/corvint-tasks` and
  `internal/tasks/**` free of Core imports, so Tasks cannot link the stable verifier itself.

## Failure modes

Closed codes, each with a closed reason (see the spec's failure-mode table):

- `CEM_VERDICT_INPUT_INVALID` has two reasons: `invalid-input` and `commit-unavailable`.
- `CEM_VERDICT_MAP_MISSING` and `CEM_VERDICT_MAP_AMBIGUOUS`.
- `CEM_VERDICT_PRODUCER_MISMATCH` has three reasons: `sealed-target-unavailable`,
  `sealed-tree-mismatch` and `sealed-base-mismatch`. It is also returned when re-verified bytes differ.
- `CEM_VERDICT_UNVERIFIABLE` has three reasons: `unsupported-platform`, `git-read-failed` (which
  includes a root that is no readable repository) and `stable-not-accepted`.
- `CEM_VERDICT_PREDICATE_FAILED` has two reasons: `no-governed-hunk` and `tier-below-anchored`.

## Acceptance evidence

The tests run on a dogfood-shaped fixture: base, change, bind and seal commits produced by the CEM
workflow. One hunk is anchored and one is cited.
`GOTOOLCHAIN=local go test -count=1 -run TestTCBV00 ./internal/taskman/` passed all eight tests
(19.3s on a quiet host after the review fix; 118.7s before it at load average 43):

- `TestTCBV0007GateInputsAreNativeAndClosed`
- `TestTCBV0008SealedMapIsReadFromCandidateGit`
- `TestTCBV0009ProducerFieldsDisagreeingRefuse`, which includes the sealed-bytes tamper.
- `TestTCBV0010UnverifiableMapRefuses`, which includes a root that is no repository.
- `TestTCBV0011PredicateIsClosedAndNeverRaisesATier`
- `TestTCBV0012GrantPinsVerdictAndCitation`, which compares against an independent `verify.Stable`
  run.
- `TestTCBV0013TamperedEvidenceRefusesOnReverify`, which covers an edited `verdictSha256`, a raised
  tier and a dropped citation, plus store-less re-verification.
- `TestTCBV0014GateIsReadOnly`

## Retained limits and NOT_PRODUCED

- No native gate consumes the library yet (NOT_PRODUCED). Three pieces are outside this lane's scope:
  - a Core command a static-argv COMMAND gate can run (`cmd/corvint`);
  - delivery of the attempt's base, candidate and claim-era touch keys to that command;
  - a `.taskman/policy.json` gate registration.
- The installed `corvint-tasks` binary is not rebuilt.
- Stacked seals and candidates with more than one added sealed map refuse `CEM_VERDICT_MAP_AMBIGUOUS`.
  Verifying a seal chain is follow-up work.
- A candidate with commits after the seal, such as a merge of `main`, refuses `sealed-tree-mismatch`.
  The gate must run on the seal commit itself.
- The `anchored` tier can be reached by lexical coincidence (decision 0527, V1-1336). The gate
  inherits the verifier's tier and does not judge citation quality.
- Only Darwin and Linux are supported, as for the stable verifier. Same-user hostile mutation of the
  repository during a read is not authenticated.
- The tests add about 20 seconds to `internal/taskman` on a quiet host, and about two minutes at
  load average 43, because each one builds a CEM workflow fixture. The CI shard cost table
  (`.github/cishards`) is outside this lane and was not changed.
- A map that declares artifacts depends on those artifact files being present under the root. The
  fixture declares none, matching the dogfood maps at BASE, so the artifact path is exercised only by
  the stable verifier's own tests.

## Independent review

One read-only Codex round over 94a7d3dc5..HEAD returned CONCERNS with three findings:

1. Fixed. An unreadable repository and an absent commit both reported `commit-unavailable`. A
   repository probe now runs first, and the commit lookup uses a quiet `--verify`, so the first case
   refuses `CEM_VERDICT_UNVERIFIABLE` (`git-read-failed`). The test covers a root that is no
   repository.
2. Confirmed and retained as a limit. With the artifact root at the worktree, which the lane charter
   requires, declared artifacts are a worktree input. The spec and this entry now say so, along with
   why the inputs cannot change a grant.
3. Confirmed and out of scope. No native gate consumes the library yet. The three missing pieces
   are listed under retained limits and reported as lane blockers.

## Rollback

Delete `internal/taskman/cem_verdict_gate.go` and its test, and revert the spec section and the
docs-tail rows. No Tasks schema, policy, store, wire contract or CEM byte changed.
