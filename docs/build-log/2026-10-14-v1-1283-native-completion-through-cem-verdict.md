# 2026-10-14 A real ticket completed through the native cem-verdict gate on a native map (V1-1283 criterion 4, migrated-map clause unmet)

Ticket V1-1283 criterion 4, verbatim: "One real ticket in this queue is completed through the gate on
a migrated map and its receipt audit is retained in the build log." This entry retains the
completion and the audit for a ticket whose map is a natively sealed cem/1.0 map. The "on a migrated
map" clause is not met, and the gate as specified cannot meet it (see Retained limits); the
disposition of that clause is the owner's. Gate wiring: PR #863 (merge
b9b10b42d, entry `2026-10-13-v1-1283-gate-wiring.md`, `TCB-V0-015..019`); policy version 9 registers
`cem-verdict` (kind COMMAND, `corvint cem gate-verdict --base {attempt.baseCommit} --candidate
{attempt.candidateCommit} --predicate touched-anchored --touch-paths {claim.touchPaths}
--portable-consumer <cem01-go>`, cwd WORKTREE, required false, reusable false).

## The completion

Ticket V1-1348 (dogfood `finish-lane`, PR #868, merge 5163d1ed4) declares `requiredGates:
["cem-verdict"]` and `effects.touchPaths: ["docs/specs/README.md","docs/specs/REQUIREMENTS.tsv"]`.
Its lane sealed a native cem/1.0 map: change fe807433d, bind adc5dce2e, seal 460fc31ae
(`.corvint/changes/adc5dce2e7145fa4b8d0ef85908daa92cfaf9d44.cem.json`, blob 277f01f830, mapSha256
317d3c4c14913fe15ad247d7c01b219003bcaaccd5877e98a21aec0a067f9ef0). The orchestrator ran, from the
store's primary checkout with the installed `corvint` built from b9b10b42d and `cem01-go` as the
portable consumer:

| Step | Request | Result |
| --- | --- | --- |
| claim V1-1348, base d97f00b64, scope = 14 diff paths to the seal | attempt `attempt:corvint:main:a96c4df574a7319ee05202910455b9b6`, generation 393 | OK (scope clipped to the two declared paths; `widen` restored the 14) |
| submit tree 518bd7816aa14eb586c8e55f8dd048d4c390f99f (the seal commit's tree) | | OK |
| gate run `focused-docs` | receipt 000000005367 | PASSED, exit 0 |
| gate run `cem-verdict` on a worktree detached at 460fc31ae | receipt 000000005368, gate result 34b109619d384b6bf2bcd3cbf7a57a4a3bb196fea571bb88661ac92ee645bf0a, evidence `output` dda1fc440448fbc7a5514e123131e876783d6e53dc196e9516511e4b579873a0 | PASSED, exit 0, 2026-10-14T00:11:08Z to 00:11:11Z |
| complete `--commit 460fc31ae` | | OK; `ticket show` reads COMPLETED, completion VERIFIED at 2026-10-14T00:11:15Z |

The recorded `taskman-cem-verdict-gate/0` document:

- `outcome: GRANT`, inputs base d97f00b64e36ed097d975594c67cc3c05c63a105, candidate
  460fc31ae6d0134fa27f7a66336b99db88fc91c6, predicate `touched-anchored`, the two declared touch paths.
- Native verdict: `ACCEPT`, stage `complete`, sidecar `EXACT`,
  `verdictSha256 8c0d6b0c315003535d17828ded7adffe14c6df746a9892cab47b445882a59bf9`.
- Portable consumer (`interop/cem01-go`, route `verify-stable-canonical`): `ACCEPT`, the same
  `verdictSha256`, citation sha256 9fea891c1a8f96c3c0df409f9b71e20c872186ff92959e8d07e2db6ed4e2e532.
  No `verifier-disagreement`.
- Governed paths, both `satisfied: true` at tier `anchored`: `docs/specs/README.md`
  (hunk:sha256:8667329a…) and `docs/specs/REQUIREMENTS.tsv` (hunk:sha256:8ba24ccb…).

## Receipt audit after the completion

`corvint-tasks receipt audit` (head sequence 5369, last receipt sha256
570950703d7ba339e24598341984551a4b8d5df9ba081d60c926ee5bc36d6582): `structuralConsistency
CONSISTENT`, `projectionAgreement AGREES`, `stagingPresent false`, `semanticCoverage UNKNOWN`, and
`actorAuthentication`, `historicalAcceptance`, `liveness`, `runtimeQualification` all
`NOT_OBSERVED` (decision 0003: the actor binding is a recorded local-operator claim, not an
authentication).

## The refused first run (same ticket, receipt 000000005362)

The first attempt (generation 392) was claimed at the merge's first parent 142f0e3e7 with the merge
commit 5163d1ed4 as the worktree HEAD, the shape every earlier native closeout used. The gate ran
(exit 1, FAILED, gate result 3f2e6487cf…, evidence 991b6c6344…) and refused:
`outcome REFUSE`, `code CEM_VERDICT_PRODUCER_MISMATCH`, `reason sealed-tree-mismatch`, with
`verdict null` and no governed paths. The merge commit's tree 728e6e3f99 carries PR #867's files as
well, so it is not the tree the map was sealed against. `complete` then refused with `GATE_FAILED`,
as specified: a refusal is never a pass. This is ticket V1-1356 (the gate's candidate must be the
seal commit while native completion pins the merge commit); the closeout recipe for a
`cem-verdict` ticket is therefore: claim at the lane base, widen to the lane's diff paths, submit
the seal commit's tree, run the gate with the worktree detached at the seal commit, complete at the
seal commit (an ancestor of `origin/main` through the merge's second parent). The clipped claim
scope is V1-1359.

## Retained limits

- The completed map is a natively sealed cem/1.0 map, not a migrated one. No ticket was completed
  through the gate on a migrated map, and none can be: the gate-wiring entry records that its
  rehearsal on the migrated map `01cdbcb09…` (provenance cem/0.2, rewritten by 0b6f1bc24 for
  V1-1277) failed at once with `CEM_VERDICT_UNVERIFIABLE` (`stable-not-accepted`, both consumers
  REJECT at stage `wire` with `unsupported-spec`), because the seal commit still holds the cem/0.2
  bytes. Admitting migrated maps needs a provenance-aware mode with its own contract. The
  migrated-map clause of criterion 4 therefore stays unmet; V1-1283's native completion records
  this as a deviation for the owner to accept or refuse.
- The portable leg ran on darwin (V1-1358: `verify-stable-canonical` is darwin-only).
- The gate's `{attempt.candidateCommit}` is the worktree HEAD, so the orchestrator, not the policy,
  guarantees that the worktree is at the seal commit.
