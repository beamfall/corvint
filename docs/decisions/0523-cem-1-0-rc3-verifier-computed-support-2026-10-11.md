# Decision 0523: CEM 1.0 in rc.3 is verifier-computed support, not author-declared support

Requirement ID note (2026-10-12): the draft IDs `CEM-V1-060..070` in this decision collided with
IDs accepted on `main` by beamfall/corvint#817 (`CEM-V1-051..064`, `070..079`), so every ID below is
shifted by +20 (`CEM-V1-080..090`). The P0 slice landed as `CEM-V1-080..082`.

Date: 2026-10-11. Status: accepted as the rc.3 plan (owner instruction in chat 2026-10-11: "I want
this as part of rc.3"). Amends decision 0521 §1 and §2 within the limits 0521 §2 already sets.
Every requirement ID below is proposed; this decision accepts none of them.

## Context

Decision 0521 moved the CEM 1.0 epic into rc.3. A five-lane expert panel on 2026-10-11 reviewed
the epic against `origin/main` fb11b2274 (build-log entry `2026-10-11-cem-1-0-expert-panel.md`).
Its shared finding: a `cem/1.0` map as specified is a reference-integrity wrapper around the
unchanged 0.2/0.3 hunk vocabulary, and nothing in the spec, the verifier or the dogfood producer
distinguishes a careful map from a rubber stamp.

- `supported` means only "at least one structurally valid reference" (`internal/cem/wire/map.go`,
  `docs/CHANGE-EVIDENCE-MAP.md` evidence model). No verifier reads `relation`.
- An all-empty criterion/receipt/link graph is a coherent `cem/1.0` document
  (`internal/cem/wire/stable.go`, `reference-coherence`). Coverage and discrimination witnesses
  are `WIRE_VALIDATED_ONLY` (`internal/cem/verify/stable.go`). A discrimination witness claiming
  1,000,000 mutants under `maxMutants: 1` is accepted today: `discriminationCounts` checks only
  `killed+survived <= mutants` and never compares the counts with the bounds.
- The dogfood producer cannot emit honest uncertainty. `DCW-V0-019`
  (`docs/specs/daily-change-evidence-workflow-v0.md`, `docs/DOGFOOD.md` §4) refuses the whole
  citation plan if any non-bootstrap hunk is left `unknown`. The 99.8% `supported` rate over the
  591 committed maps is therefore a workflow artifact, and the kill condition ">30% unknown" in
  `docs/CHANGE-EVIDENCE-MAP.md` can never fire on dogfood maps.
- Measured over the 591 committed `cem/0.2` maps: 0 of 8,008 supported non-test Go code hunks
  cite a `_test.go` file (one lane counted 2; both counts round to none). 100 of the newest 200
  maps give every supported hunk the identical basis set. 169 of 591 cite one span for every
  supported hunk. The hunk-to-test link that the CEM-V1-006 runner breadth would carry does not
  exist in practice, so the runners have nothing to serve.
- The signing and anchoring path cannot carry `cem/1.0`: `CEMStatementV1` and `cem anchor` both
  call `wire.ParseMap`, which refuses the stable spec (`CEM-V1-018`). `cem anchor --commit REV`
  attaches HEAD's map to any REV and `pointerState` still reads back `verified`, because it
  checks path, blob and digest, never scope. Receipt `authentication` is a single-valued constant
  (`NOT_OBSERVED`); freezing it in V1-0595 makes an authenticated receipt inexpressible in 1.0.
- The only outcome evidence is the 5-change reviewer pilot whose treatment arm was an all-`unknown`
  `cem/0.1` map; `tools/cem-trial/score.go` scores recall against gold paths only, so citing every
  file reaches miss 0 at no cost.

The stable packet is still `"status": "EXPERIMENTAL"` (`protocol/cem-1.0/stable/manifest.json`).
After the V1-0595 freeze, `CEM-CB-025` means each change below needs a new profile ID. The
window to decide what "supported" means in 1.0 is open now and closes at that freeze.

## Decision

### 1. The claim CEM 1.0 makes

A `cem/1.0` consumer computes every hunk's support class from Git bytes and its own re-execution.
Nothing a producer writes can raise that class. The headline for Go changes is: "revert this hunk
alone and the named test fails; re-run it yourself with git and `go test`." Diff coverage shows a
line executed, not load-bearing; SLSA/in-toto shows who built a change, not why a hunk exists; AI
review bots require trusting a model. No shipped format gives a reviewer a per-hunk claim they can
re-check without trusting the author or a model. That is the novelty, and it is only novel if the
verifier, not the author, assigns it.

### 2. Moves, in order of earliest proof

Each move names its proposed requirement IDs, its falsifying fixture, and its rc.3 fit:
**fits**, **recorded limit** (ships with the limit named in the release notes, never reported as
qualified) or **post-rc.3**.

**P0. Consumer-computed support tiers and specificity report (no wire change, day-1 proof).**
`cem verify`/`status`/`report` and the portable consumer derive, per hunk, exactly one tier:
`held` (P2 below), `mechanical-proved` (existing structural re-proof), `anchored`, `cited`
(reference integrity only) or `unknown`, plus an `annotation` flag when the hunk's added lines
are only ID tokens, punctuation or stop terms. `anchored` is derived by the verifier from the
base blob, target blob and canonical patch alone, with two kinds in rc.3: `id` (an ASCII
requirement ID per the `lexical-relevance-floor-v0` grammar occurs in the hunk's added bytes and
the cited span defines it with the ``- `ID`:`` prefix) and `path` (the span names the hunk's
post-image path or a basename of at least 6 bytes). The Go-only `symbol` kind is a recorded
limit. The closed result also gains an integer-exact `citation` block (`hunksByClass`,
`distinctEvidence`, ordered `evidenceFanOut`, `maxFanOut`, `samePathCitations`,
`concentratedHunks`) and a map-level assignment null: sort supported hunks by ID, shift the basis
sets cyclically by one, recompute the anchor count, and report `invariant` (all basis sets equal)
and `specificity` (anchored minus anchored-after-shift). No value in this block is pass/fail.
- Proposed IDs: `CEM-V1-080` (tiers are consumer-derived; no producer field can raise one),
  `CEM-V1-081` (anchor kinds and their byte-level rules), `CEM-V1-082` (`citation` block and the
  shift null are byte-identical across native and portable consumers and carry no verdict).
- Falsifying fixtures: (a) a rubber-stamp map (every hunk cites one `AGENTS.md` span) and a
  careful map of the same 24-hunk patch both ACCEPT, and their `citation` integers differ;
  (b) `AGENTS.md` split into 20 disjoint spans with one per hunk must still report
  `specificity: 0`, which kills any fan-out or umbrella-path implementation; (c) a hunk adding
  ``x // `CEM-SM-007` `` against a span that mentions `CEM-SM-0070` and defines nothing must be
  `cited`, not `anchored`; (d) a hunk adding only that comment line is `annotation`.
- Measured basis: verifier-derived anchors hit 5.7% of real (hunk, basis) pairs against 0.45%
  under evidence drawn from other maps (12.9x lift); 97 of 200 maps have zero anchored hunks and
  45 have positive specificity. Lexical any-term overlap (63.7% real against 55.3% shuffled)
  does not discriminate and is not used.
- Earliest proof: run the new report over the 591 historical maps. The expected baseline is
  roughly 0 `held`, about half `invariant`. That report is the first deliverable of the epic.
- rc.3: **fits**.

**P1. Wire changes that must land before the V1-0595 freeze.**
- `CEM-V1-083` Criterion is provider-neutral: a criterion is any committed Git blob span holding
  acceptance text; the Tasks digests (`claimTicketSha256`, `captureSha256`, `snapshotHead*`)
  become an optional extension. A `cem/1.0` map MUST verify with no Corvint Tasks store present.
- `CEM-V1-084` Receipt `authentication` becomes a verifiable slot: `{"method":"none"}` or
  `{"method":"dsse-ed25519","keyid":K,"envelopeSha256":E}` where E is a `receipt-attestation`
  artifact holding an in-toto Statement whose subjects are the receipt digest, the runner-plan
  digest and the target `gitTree`. The axis rises above `NOT_OBSERVED` only when a consumer-
  supplied `--trusted-keys` file names K; a key the map carries itself stays `UNTRUSTED_KEY`; a
  valid signature never raises `executionAtCommit`.
- `CEM-V1-085` `cem/v2` in-toto statement: subjects are the stable map and every `artifacts[]`
  entry; the predicate adds `target {gitCommit, gitTree}` beside `base` and `patch`. `ParseMap`
  routes the stable spec to `ParseStable` for attestation and anchoring. Verification refuses an
  artifact or `--target` that is not a subject of the signed statement.
- `CEM-V1-086` Verdict/environment split: the result separates `verdict` from `environment`
  (`runtime.goVersion`, repository envelope) and publishes `verdictSha256` over canonical
  `verdict`. Two conformant consumers given identical inputs MUST produce identical
  `verdictSha256`. Later re-verification appends a record keyed by (`mapSha256`, target) and
  rewrites nothing; hunk IDs carry line ranges and are not chain keys.
- `CEM-V1-087` Anchor pointer `corvint-cem-anchor/1` adds `base`, `patchSha256` and
  `targetTree`; `pointerState` recomputes base..REV and returns `scope-mismatch` when the map does
  not describe REV; `cem provenance --notes-tip OID` reports `rollback` when the pinned tip is not
  an ancestor of the live `refs/notes/corvint`. `/0` stays readable. Anchoring stays an explicit
  operator step; no default-on anchoring.
- Falsifying fixtures: a map that includes its own public key as an artifact stays below
  VERIFIED; a valid signature over a different `gitTree` is refused; swapping one receipt file for
  another valid receipt under the same path with a re-signed map and the old envelope is refused;
  anchoring map(A) onto a sibling commit with the same parent yields `scope-mismatch`; resetting
  the notes ref to its parent yields `rollback`; all 54 public stable cases give identical
  `verdictSha256` on both consumers, and one case with drift records out of order changes it.
- rc.3: wire slots, refusals, statement, split and pointer **fit** (Ed25519, sha256 and git only,
  in both modules). Signed production stays an operator step and is a **recorded limit**
  (`FPK-V0-036`).

**P2. Hunk-revert falsifier, `go test` only (the headline).**
A supported basis may name test IDs. The consumer reverse-applies that one hunk at target and runs
only those tests under the fixed `go test` runner profile. `held` requires a classified test
failure the consumer itself observed; a compile, skip or infrastructure failure is `not-run`. A
producer-written witness never counts. The dogfood producer gains a "cite the guarding test"
step so the tier can fire; `TCQ-V0-057` selects only hunks that already cite `_test.go` as
`test-claim`, which today is none.
- Proposed IDs: `CEM-V1-088` (`held` only after consumer re-execution), `CEM-V1-089` (two cases
  are `not-run`, never `held`: a hunk whose revert breaks compilation, and a hunk whose named test
  also fails without the revert), `CEM-V1-090` (discrimination witnesses fail `invalid-field`
  when `mutants` exceeds `bounds.maxMutants` or survivors are inconsistent with the counts).
- rc.3: `go test` **fits**; every other runner is a **recorded limit**. Frozen mutant-operator
  enumeration and pinning the coverprofile as a Git blob are **post-rc.3** (they need cases in
  the still-proposed 0.3 packet). Pre-mutation registration digests for `discriminating` links
  are a **recorded limit** for Go only.

**P3. The producer may say "I don't know".**
`DCW-V0-019` is amended so `dogfood-change` accepts a plan that leaves a hunk `unknown` with a
one-line reason, and `dogfood-check` reports the unknown share instead of refusing. Dogfood then
emits the stable profile instead of `cem/0.2`. This is what lets the ">30% unknown" kill
condition mean anything.
- rc.3: **fits**. Recorded as a defect, not a feature.

**P4. Evaluation that can refute the claim.**
`tools/cem-trial/score.go` scores precision as well as recall (cited-not-gold rate at path and
span level; the recorded secondary span gold is scored). A registration file committed before
selection fixes hypotheses, primary metric, α, n, scorer sha256, model, effort and partition. The
treatment arm is the shipped profile's artifact, never the all-`unknown` `cem/0.1` worklist. A
"cite every suggestion" script agent must lose on precision. The run itself happens only after P2
has a baseline (kill criterion K1 below) and only if the owner commissions it.
- rc.3: scorer repair and registration **fit**; the run is **post-rc.3** and stays `NOT_RUN`.
  No rc.3 text may say CEM "helps reviewers" or is "groundbreaking" until a first-run report
  whose registration commit predates its first lane reads "met" under `CRT-V0-008`.

### 3. Scope amendments to decision 0521 §1

Under 0521 §2, items that cannot close a blocker ship as recorded limits. Applying that rule with
the measurements above, the following ship in rc.3 as recorded limits and receive no further rc.3
capacity: V1-0592, V1-0594, V1-0612, V1-0620, V1-0622, and V1-0593 except the `go test` runner.
The CEM-V1-006 "all main runners" matrix is decoupled from the V1-0595 wire freeze: receipts are
opaque digests on the wire, so runner breadth changes no byte and is a qualification limit, not a
freeze input. V1-0632 is reduced to `verify`/`status`/`report` plus the portable consumer, keeping
the S3 refusals. V1-0596 promotion stays owner-blocked. The freed capacity goes to P0..P4 under
V1-0595 and V1-0614. The 0521 §2 blockers (`CEM-V1-008`, owner promotion, device tuples) are
unchanged and none of P0..P4 closes one.

### 4. Rejected alternatives

- A fourth wire disposition `asserted`: a producer-chosen strength label is another claim, cannot
  express one falsified and one unfalsified basis on the same hunk, and would make about 25 Go
  files that key on `"supported"` skew. Strength is verifier output (P0).
- Published numeric thresholds in the profile (overlap, fan-out, umbrella share): under
  `CEM-CB-025` a frozen threshold needs a new profile to revise; on measured data almost every
  current map fails on day one; fan-out and umbrella share are gamed by span splitting; umbrella
  share is bimodal (0 in 181 maps, 1.0 in 228) so no cutoff separates anything. Integers go on
  the wire, policy stays outside it.
- Author-written anchor offsets on the wire: not needed for the measured lift and would reopen a
  producer-written strength channel; revisit post-rc.3 if P0 precision against human labels
  warrants it.
- Default-on git-notes anchoring plus attestation: git plus sha256 gives integrity, not
  authenticity; anyone with push access can write the note; neither path accepts `cem/1.0` today;
  an anchor can mis-scope its target. Replaced by P1.
- Running the reviewer trial first: the pilot treatment carried no evidence, so a trial before P2
  measures a rubber stamp. Replaced by P4's ordering.
- Treating `interop/cem01-go` as the independent consumer: same author; `CEM-V1-008` and
  `PRS-V1-007` already bar it.

### 5. Kill criteria

- K1: fewer than 25% of non-mechanical Go code hunks are `held` across the next 30 dogfood
  changes after P2 lands. Then the headline is dropped from rc.3 text and P0 ships alone.
- K2: the rubber-stamp and careful maps of the same patch produce the same `citation` block or
  the same tiers. Then P0 is not done.
- K3: more than 5% of 50 audited `held` witnesses are not behavioural test failures.
- K4: a stranger cannot produce and verify a `cem/1.0` map on a non-Corvint repository within one
  engineer-day with no Tasks store. Then the adoption claim is withdrawn.
- K5: median dogfood change latency rises by more than 10% with P0 and P2 on.
- K6: native and portable `verdictSha256` differ on any public case.

### 6. What a consumer still MUST NOT conclude from an ACCEPT

That any test ran, ran at this commit or ran on these inputs; that Corvint or the change author
produced the map; that receipt outcomes are genuine (a forged PASS receipt hashed into the map
verifies); that an anchor `verified` row covers its commit; or that a valid signature proves
execution rather than who made the claim. P1 narrows the last two; none of P0..P4 removes the
first three.

## Rollback

Revert this file and its README row. Decision 0521 §1 then governs the rc.3 scope again with
its original item list. Tickets filed under this decision stay in the queue with this decision
named as their source; archive them if the decision is reverted. Nothing is tagged, published,
frozen or written to the ticket store by this decision itself.
