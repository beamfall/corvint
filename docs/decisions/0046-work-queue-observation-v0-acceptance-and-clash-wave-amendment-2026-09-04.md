# Decision 0046 — Work Queue Observation V0 is accepted, with Corvint-derived clash detection and a maximal wave

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction
"Work Queue Observation V0 should be moved to accepted. I think corvint should be able to know if 2
pieces of work are likely to clash on the same repo and be able to fire off the most tasks at once."
(2026-09-04).

## What is decided

1. `docs/specs/work-queue-observation-v0.md` moves from `proposed` to `accepted`. Its delivery
   status stays `experimental (implementation not started)`; acceptance of intent is not a delivery
   or authority claim, and every promotion gate in §9 remains `NOT_RUN`.
2. The contract gains §5.6, `WQO-V0-041..045`: tickets declare `touchPaths`; Corvint expands them at
   the snapshot commit through its own dependency index into a collision closure; two tickets clash
   when their closures share a path; derived groups are additive to adapter groups; the proposal
   reports the complete `collisionClosure` and selects the largest collision-free set of eligible
   tickets (exact branch and bound up to 64 candidates, deterministic greedy above), reported in
   `waveOptimality`.
3. The closed wire changed before any implementation exists: `TicketSummary.touchPaths`,
   `ProposalEntry.collisionGroupIds`, a `CollisionGroup` record, `Path` primitive,
   proposal `collisionClosure` and `waveOptimality`, and the unknown code
   `COLLISION_CLOSURE_INCOMPLETE`. Under `WQO-V0-040` this is a new identity; nothing was minted
   under the old one.
4. The Corvint self-dogfood authority IDs are `repo:corvint`, `queue:corvint:worklist`,
   `scope:corvint:worklist`, and `access:corvint:local`. The self-dogfood adapter is a tracked script
   that exports a snapshot from a tracked ticket file; its exact file shape is the implementation's
   call within the contract.

## Reading of the owner's second sentence

"Know if 2 pieces of work are likely to clash" is read as path-overlap after dependency expansion at
one commit, which is what Corvint can compute from immutable content and explain (invariant 1). It is
not read as semantic conflict prediction, model judgement, or merge simulation, none of which V0 can
pin to evidence. "Fire off the most tasks at once" is read as computing the largest set that can
start together; firing them stays with the orchestrator, because V0 is non-operative and read
commands do not mutate (invariant 4, `WQO-V0-045`). If the owner wants Corvint itself to start work,
that is a new operative spec under §9's promotion rule, not an amendment here.

## What stays open

- Beamfall's authority IDs, its read-only adapter (`WQO-V0-029`), and the independent recorder
  (`WQO-V0-036`) are owner inputs; without them the 500-cycle count cannot start
  (`docs/agent-memory/questions.md`, 2026-09-04 entry).
- Delivery status advances only with the implementation and its conformance package under §8.

## Rollback

Revert the header, digest, §1 amendment paragraph, §4 wire additions, `WQO-V0-015` code,
`WQO-V0-020`/`WQO-V0-021` wording, §5.6, the §6, §8, and §12 rows, and the §12 closing paragraph of
the spec; restore `proposed` in `docs/specs/README.md` and `docs/specs/INDEX.json`; regenerate
`docs/specs/REQUIREMENTS.tsv`; remove this record from `docs/decisions/README.md`. No implementation
or data depends on the new identity yet.
